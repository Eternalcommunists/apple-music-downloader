package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Download history that survives restarts.
//
// SessionManager is deliberately memory-only: it owns live goroutines, progress
// counters and log buffers, none of which mean anything once the process exits.
// What a user actually wants to keep is the *record* of what was downloaded —
// artist, title, quality, and where it landed — so history is stored separately
// as a small JSON file instead of serialising whole sessions.
//
// The file is kept in the kilobyte range on purpose. The engine log and the
// per-track file list are dropped, because "open the folder" only needs
// outputDir, and a log for a job that finished last week is noise.
// maxHistoryEntries then bounds the file even after years of use.

// maxHistoryEntries caps how many records are kept. Oldest are dropped first,
// matching the newest-first order the UI renders.
const maxHistoryEntries = 300

// historyVersion tags the file so a future format change can be migrated rather
// than mistaken for corruption.
const historyVersion = 1

// historyFile is the on-disk envelope. A wrapper object rather than a bare array
// leaves room to add fields later without breaking older readers.
type historyFile struct {
	Version int              `json:"version"`
	Entries []SessionSummary `json:"entries"`
}

// HistoryStore owns the persisted records. All access is serialized: writes
// happen at most once per finished job, so holding the lock across the write is
// cheaper than coordinating concurrent snapshot files.
type HistoryStore struct {
	mu      sync.Mutex
	entries []SessionSummary // newest first
	path    string
}

// historyPath returns the history file location, beside gui.json so both pieces
// of GUI state live in one directory.
func historyPath() string { return filepath.Join(amdlStateDir(), "history.json") }

// loadHistory reads the store from disk. A missing or unreadable file yields an
// empty store: history is a convenience, never a reason to refuse to start.
func loadHistory() *HistoryStore { return loadHistoryAt(historyPath()) }

// loadHistoryAt is the path-injectable form, so tests never touch the real user
// config directory.
func loadHistoryAt(path string) *HistoryStore {
	st := &HistoryStore{path: path}
	data, err := os.ReadFile(st.path)
	if err != nil {
		return st
	}
	var file historyFile
	if err := json.Unmarshal(data, &file); err != nil {
		// A corrupt file is discarded rather than reported: the GUI must still
		// open, and the next finished job rewrites the file cleanly.
		return st
	}
	st.entries = pruneHistory(file.Entries)
	// Pruning can drop records (an unfinished entry left behind by a crash, or
	// a file written by an older version). Write the cleaned list back so the
	// file on disk matches what the UI shows, instead of lurking there until the
	// next successful download.
	if len(st.entries) != len(file.Entries) {
		st.mu.Lock()
		st.saveLocked()
		st.mu.Unlock()
	}
	return st
}

// pruneHistory normalises a record list. Unfinished jobs are dropped because a
// record with no end time would render as a permanently running ghost row after
// a restart — its goroutine no longer exists to finish it.
func pruneHistory(in []SessionSummary) []SessionSummary {
	out := make([]SessionSummary, 0, len(in))
	for _, e := range in {
		if strings.TrimSpace(e.ID) == "" {
			continue
		}
		if e.Status == SessionRunning || e.Status == SessionQueued {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	if len(out) > maxHistoryEntries {
		out = out[:maxHistoryEntries]
	}
	return out
}

// add records one finished job. It is called from the goroutine that waits on
// the session, never from a poll, so the file is written once per job.
func (h *HistoryStore) add(s SessionSnapshot) {
	e := summarize(s)

	h.mu.Lock()
	defer h.mu.Unlock()

	// Replace any existing record for the same id. The same job is otherwise
	// persisted once, but replacing keeps this safe if it is ever called twice.
	for i, old := range h.entries {
		if old.ID == e.ID {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			break
		}
	}
	h.entries = append([]SessionSummary{e}, h.entries...)
	if len(h.entries) > maxHistoryEntries {
		h.entries = h.entries[:maxHistoryEntries]
	}
	h.saveLocked()
}

// list returns a copy of every record, newest first.
func (h *HistoryStore) list() []SessionSummary {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]SessionSummary, len(h.entries))
	copy(out, h.entries)
	return out
}

// get returns one stored record. It is the fallback for a session id that is no
// longer in memory, which happens for any job from a previous run.
func (h *HistoryStore) get(id string) (SessionSummary, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, e := range h.entries {
		if e.ID == id {
			return e, true
		}
	}
	return SessionSummary{}, false
}

// remove deletes one record, reporting whether it existed.
func (h *HistoryStore) remove(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, e := range h.entries {
		if e.ID == id {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			h.saveLocked()
			return true
		}
	}
	return false
}

// clear drops every record and returns how many were removed.
func (h *HistoryStore) clear() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := len(h.entries)
	if n == 0 {
		return 0
	}
	h.entries = nil
	h.saveLocked()
	return n
}

// saveLocked writes the current records. The caller must hold h.mu.
//
// The write goes to a temporary file that is then renamed over the target, so a
// crash mid-write cannot truncate a good history file into garbage.
func (h *HistoryStore) saveLocked() {
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return
	}
	data, err := json.MarshalIndent(historyFile{Version: historyVersion, Entries: h.entries}, "", "  ")
	if err != nil {
		return
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	// Failures here are non-fatal: the in-memory copy stays correct for this
	// run and the next save retries.
	_ = os.Rename(tmp, h.path)
}

// mergedJobs combines persisted history with live sessions for the task and
// history views. A job that is both running and already recorded (it finished
// while the window was open) is represented once, by its live state, so the row
// cannot appear twice or show stale progress.
func (s *GUIServer) mergedJobs() []SessionSummary {
	merged := s.History.list()
	index := make(map[string]int, len(merged))
	for i, e := range merged {
		index[e.ID] = i
	}
	for _, live := range s.Sessions.summaries() {
		if i, ok := index[live.ID]; ok {
			merged[i] = live
			continue
		}
		merged = append(merged, live)
	}
	// Timestamps are RFC3339 in a single local zone, so a string compare orders
	// them correctly and avoids parsing every record on each poll.
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].StartedAt > merged[j].StartedAt })
	return merged
}

// SessionDetail is the payload for a single-session lookup. It is a summary
// plus the per-track file list that only exists while the job is in memory.
//
// The split matters: the history store deliberately drops the file list to stay
// small, so a record loaded from a previous run carries AddedTracks == nil and
// the UI falls back to revealing outputDir.
type SessionDetail struct {
	SessionSummary
	AddedTracks []AddedTrack `json:"addedTracks"`
}

// sessionDetail resolves a session id for the done card and the reveal action.
//
// A live session is preferred because it still carries addedTracks, which the
// done card lists and the "open location" action can fall back to. A job from a
// previous run is served from the store, where only the summary remains.
func (s *GUIServer) sessionDetail(id string) (SessionDetail, bool) {
	if sess, ok := s.Sessions.get(id); ok {
		snap := sess.snapshot()
		return SessionDetail{SessionSummary: summarize(snap), AddedTracks: snap.AddedTracks}, true
	}
	if rec, ok := s.History.get(id); ok {
		return SessionDetail{SessionSummary: rec}, true
	}
	return SessionDetail{}, false
}
