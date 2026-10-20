package app

import (
	"net/http"
	"runtime"

	"amdl/internal/version"
)

// SessionSummary is the compact row the task and history views render. Keeping
// it separate from SessionSnapshot avoids shipping the full log for every row.
type SessionSummary struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Quality     string `json:"quality"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Artist      string `json:"artist"`
	StartedAt   string `json:"startedAt"`
	FinishedAt  string `json:"finishedAt"`
	Success     int    `json:"success"`
	Error       int    `json:"error"`
	Unavailable int    `json:"unavailable"`
	Total       int    `json:"total"`
	Processed   int    `json:"processed"`
	Percent     int    `json:"percent"`
	OutputDir   string `json:"outputDir"`
	ErrorText   string `json:"errorText"`
	// ReportedQuality is the codec actually written, e.g. "ALAC 24-bit / 96000 Hz".
	ReportedQuality string `json:"reportedQuality"`
}

// summarize projects a snapshot down to one row.
func summarize(s SessionSnapshot) SessionSummary {
	return SessionSummary{
		ID:              s.ID,
		Status:          s.Status,
		Quality:         s.Quality,
		Kind:            s.Kind,
		Name:            s.Name,
		Artist:          s.Artist,
		StartedAt:       s.StartedAt,
		FinishedAt:      s.FinishedAt,
		Success:         s.Success,
		Error:           s.Error,
		Unavailable:     s.Unavailable,
		Total:           s.Total,
		Processed:       s.Processed,
		Percent:         s.Percent,
		OutputDir:       s.OutputDir,
		ErrorText:       s.ErrorText,
		ReportedQuality: s.ReportedQuality,
	}
}

// summaries returns every tracked session, newest first. The order slice is
// walked backwards so the most recent job appears at the top of the lists.
//
// The manager lock is released before each session's own lock is taken: holding
// both at once (manager then session) while another path takes them in the
// opposite order would deadlock.
func (m *SessionManager) summaries() []SessionSummary {
	m.mu.Lock()
	ids := make([]string, 0, len(m.order))
	for i := len(m.order) - 1; i >= 0; i-- {
		ids = append(ids, m.order[i])
	}
	sessions := make([]*DownloadSession, 0, len(ids))
	for _, id := range ids {
		if s, ok := m.sessions[id]; ok {
			sessions = append(sessions, s)
		}
	}
	m.mu.Unlock()

	out := make([]SessionSummary, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, summarize(s.snapshot()))
	}
	return out
}

// clearFinished drops every session that is no longer running, which is what
// the "clear history" action does. Active jobs are preserved.
func (m *SessionManager) clearFinished() int {
	type entry struct {
		id string
		s  *DownloadSession
	}

	m.mu.Lock()
	entries := make([]entry, 0, len(m.order))
	for _, id := range m.order {
		if s, ok := m.sessions[id]; ok {
			entries = append(entries, entry{id, s})
		}
	}
	m.mu.Unlock()

	// Decide outside the manager lock, then prune in one critical section.
	var drop []string
	for _, e := range entries {
		st := e.s.snapshot().Status
		if st != SessionRunning && st != SessionQueued {
			drop = append(drop, e.id)
		}
	}
	if len(drop) == 0 {
		return 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	dropSet := make(map[string]bool, len(drop))
	for _, id := range drop {
		dropSet[id] = true
		delete(m.sessions, id)
	}
	kept := make([]string, 0, len(m.order))
	for _, id := range m.order {
		if !dropSet[id] {
			kept = append(kept, id)
		}
	}
	m.order = kept
	return len(drop)
}

// remove deletes one session unless it is still running.
func (m *SessionManager) remove(id string) bool {
	m.mu.Lock()
	s, ok := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return false
	}
	st := s.snapshot().Status
	if st == SessionRunning || st == SessionQueued {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	for i, x := range m.order {
		if x == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return true
}

// handleTasks lists every known job, split into active and finished, so the
// task view can show progress and the history view can show the archive from a
// single call.
//
// The list merges this run's live sessions with the records persisted by
// previous runs, so history is populated immediately after a restart instead of
// appearing empty until something new is downloaded.
func (s *GUIServer) handleTasks(w http.ResponseWriter, r *http.Request) {
	all := s.mergedJobs()

	active := make([]SessionSummary, 0, 4)
	done := make([]SessionSummary, 0, len(all))
	for _, x := range all {
		if x.Status == SessionRunning || x.Status == SessionQueued {
			active = append(active, x)
			continue
		}
		done = append(done, x)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"active":  active,
		"history": done,
	})
}

// handleHistoryClear removes finished jobs, keeping anything still running.
//
// The in-memory sessions are pruned first: a live session that was already
// persisted would otherwise reappear in the list on the next poll, making the
// clear button look broken.
func (s *GUIServer) handleHistoryClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	removed := s.Sessions.clearFinished()
	removed += s.History.clear()
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

// handleHistoryDelete removes a single finished job.
func (s *GUIServer) handleHistoryDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 id"})
		return
	}
	// A record lives in one of two places depending on whether the job ran in
	// this process, so both are tried. remove() refuses a job that is still
	// running, which is why its result is checked before touching the store.
	removedLive := s.Sessions.remove(id)
	removedStored := s.History.remove(id)
	if !removedLive && !removedStored {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "任务不存在或仍在进行中"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AboutInfo is the payload for the about view.
type AboutInfo struct {
	Version    string            `json:"version"`
	Commit     string            `json:"commit"`
	BuildDate  string            `json:"buildDate"`
	GoVersion  string            `json:"goVersion"`
	Dev        bool              `json:"dev"`
	LiteServer string            `json:"liteServer"`
	Storefront string            `json:"storefront"`
	Paths      map[string]string `json:"paths"`
	Quality    map[string]any    `json:"quality"`
}

// handleAbout reports build identity plus the effective configuration, so the
// about view can show exactly what the running binary is doing.
func (s *GUIServer) handleAbout(w http.ResponseWriter, r *http.Request) {
	c := s.Runner.Config
	loggedIn, regions := s.Runner.loginStatus()

	info := AboutInfo{
		Version:    version.Version,
		Commit:     version.Commit,
		BuildDate:  version.BuildDate,
		GoVersion:  versionRuntime(),
		Dev:        version.IsDev(),
		LiteServer: c.General.LiteServer,
		Storefront: c.General.Storefront,
		Paths: map[string]string{
			"alac":  c.Paths.Alac,
			"atmos": c.Paths.Atmos,
			"aac":   c.Paths.Aac,
			"mv":    c.Paths.MV,
		},
		Quality: map[string]any{
			"alacMax":      c.Media.AlacMax,
			"atmosMax":     c.Media.AtmosMax,
			"aacType":      c.Media.AacType,
			"losslessOnly": c.Media.LosslessOnly,
			"alacFix":      c.Media.ALACFix,
			"alacVerify":   c.Media.ALACVerify,
			"loggedIn":     loggedIn,
			"regions":      regions,
		},
	}
	writeJSON(w, http.StatusOK, info)
}

// versionRuntime returns the Go runtime version string (e.g. "go1.23.1").
func versionRuntime() string {
	return runtime.Version()
}
