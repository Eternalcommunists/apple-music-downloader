package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SessionStatus is the lifecycle state of a GUI download.
const (
	SessionQueued    = "queued"
	SessionRunning   = "running"
	SessionCompleted = "completed"
	SessionFailed    = "failed"
)

// maxLogLines caps the log ring buffer the UI polls.
const maxLogLines = 400

// ProgressEvent is a structured update emitted by the download engine and
// surfaced to the GUI.
type ProgressEvent struct {
	Type    string `json:"type"`    // track | collection | counter | note
	Message string `json:"message"` // human readable line
}

// SessionSnapshot is everything the UI needs for one poll, serialized as JSON.
type SessionSnapshot struct {
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
	CurrentItem string `json:"currentItem"`
	// ReportedQuality is the codec/sample-rate the engine actually negotiated,
	// e.g. "ALAC 24-bit / 96000 Hz". It is captured from the engine's own output
	// because a .m4a container says nothing about whether the audio is lossless.
	ReportedQuality string       `json:"reportedQuality"`
	Log             []string     `json:"log"`
	AddedTracks     []AddedTrack `json:"addedTracks"`
	OutputDir       string       `json:"outputDir"`
	ErrorText       string       `json:"errorText"`
}

// DownloadSession tracks one GUI-initiated download. It owns its own copy of
// the progress state: the download goroutine publishes into it and the HTTP
// poller only reads, so no state is shared unsynchronized.
type DownloadSession struct {
	ID      string
	Quality string
	Kind    string
	Name    string
	Artist  string

	mu          sync.Mutex
	status      string
	startedAt   time.Time
	finishedAt  time.Time
	currentItem string
	// lastProgressLine dedupes repeated progress-bar redraws so the log only
	// records each distinct state once.
	lastProgressLine string
	// reportedQuality is the last codec/sample-rate line seen in engine output.
	reportedQuality string
	counter         Counter
	log             []string
	addedTracks     []AddedTrack
	outputDir       string
	err             error
	done            chan struct{}
}

// SessionManager keeps the active sessions, capped so a long-lived window does
// not accumulate them.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*DownloadSession
	order    []string
}

const maxSessions = 20

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]*DownloadSession)}
}

func (m *SessionManager) add(s *DownloadSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	m.order = append(m.order, s.ID)
	for len(m.order) > maxSessions {
		oldest := m.order[0]
		m.order = m.order[1:]
		delete(m.sessions, oldest)
	}
}

func (m *SessionManager) get(id string) (*DownloadSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// qualityRe matches the codec/sample-rate line the engine prints after it has
// negotiated a stream, e.g. "ALAC 24-bit / 96000 Hz" or "AAC | 2 Channel".
var qualityRe = regexp.MustCompile(`(?i)^\s*(ALAC\s+\d+-bit\s*/\s*\d+\s*Hz|AAC[^\n]{0,40}|E-AC-3[^\n]{0,40}|AC-3[^\n]{0,40})\s*$`)

// appendLog adds a line to the ring buffer.
func (s *DownloadSession) appendLog(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log = append(s.log, line)
	if len(s.log) > maxLogLines {
		s.log = s.log[len(s.log)-maxLogLines:]
	}
	// Remember the negotiated quality so the UI can show what was really
	// written, which the .m4a extension alone cannot convey.
	if m := qualityRe.FindStringSubmatch(line); m != nil {
		s.reportedQuality = strings.TrimSpace(m[1])
	}
}

// appendCarriageReturnFrames folds a progress-bar line into its segments.
// Tools like the progress bar library redraw the same line with \r, so the raw
// text would otherwise arrive as one enormous line and swamp the log pane. The
// final segment carries the settled state, so it becomes the current-item text
// and is only appended to the log when it actually changed.
func (s *DownloadSession) appendCarriageReturnFrames(raw string) {
	segments := strings.Split(raw, "\r")
	last := ""
	for _, seg := range segments {
		if t := strings.TrimSpace(seg); t != "" {
			last = t
		}
	}
	if last == "" {
		return
	}

	s.mu.Lock()
	s.currentItem = last
	changed := last != s.lastProgressLine
	if changed {
		s.lastProgressLine = last
	}
	s.mu.Unlock()

	if changed {
		s.appendLog(last)
	}
}

// note records current-item text and mirrors it into the log.
func (s *DownloadSession) note(msg string) {
	// Progress-bar redraws use \r; fold them instead of logging them raw.
	if strings.ContainsRune(msg, '\r') {
		s.appendCarriageReturnFrames(msg)
		return
	}
	s.mu.Lock()
	s.currentItem = msg
	s.mu.Unlock()
	s.appendLog(msg)
}

func (s *DownloadSession) setStatus(status string) {
	s.mu.Lock()
	s.status = status
	if status == SessionCompleted || status == SessionFailed {
		s.finishedAt = time.Now()
	}
	s.mu.Unlock()
}

func (s *DownloadSession) fail(err error) {
	s.mu.Lock()
	s.err = err
	s.status = SessionFailed
	s.finishedAt = time.Now()
	s.mu.Unlock()
}

// snapshot returns a consistent view for one UI poll.
func (s *DownloadSession) snapshot() SessionSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	processed := s.counter.Success + s.counter.Error + s.counter.Unavailable + s.counter.NotSong
	percent := 0
	if s.counter.Total > 0 {
		percent = processed * 100 / s.counter.Total
		if percent > 100 {
			percent = 100
		}
	}
	finished := ""
	if !s.finishedAt.IsZero() {
		finished = s.finishedAt.Format(time.RFC3339)
	}
	errText := ""
	if s.err != nil {
		errText = s.err.Error()
	}

	logCopy := make([]string, len(s.log))
	copy(logCopy, s.log)
	// Paths are absolutized for the UI: config paths are relative to the
	// working directory, but "open file location" needs a real absolute path.
	tracksCopy := make([]AddedTrack, len(s.addedTracks))
	for i, t := range s.addedTracks {
		t.Path = absPath(t.Path)
		tracksCopy[i] = t
	}

	return SessionSnapshot{
		ID:              s.ID,
		Status:          s.status,
		Quality:         s.Quality,
		Kind:            s.Kind,
		Name:            s.Name,
		Artist:          s.Artist,
		StartedAt:       s.startedAt.Format(time.RFC3339),
		FinishedAt:      finished,
		Success:         s.counter.Success,
		Error:           s.counter.Error,
		Unavailable:     s.counter.Unavailable,
		Total:           s.counter.Total,
		Processed:       processed,
		Percent:         percent,
		CurrentItem:     s.currentItem,
		ReportedQuality: s.reportedQuality,
		Log:             logCopy,
		AddedTracks:     tracksCopy,
		OutputDir:       s.outputDir,
		ErrorText:       errText,
	}
}

// emit records a structured event from the engine. Output is already captured
// from stdout, so this only updates the counters rather than logging again.
func (s *DownloadSession) emit(r *Runner, ev ProgressEvent) {
	switch ev.Type {
	case "counter":
		r.GetStateInto(&s.counter, &s.addedTracks)
	default:
		if ev.Message != "" {
			s.note(ev.Message)
		}
	}
}

// startDownload wires a Runner for GUI use and runs the download in the
// background. The passed runner is assumed to be freshly built for this job.
func (s *DownloadSession) startDownload(r *Runner, urls []string, probe *ProbeResult, quality string) {
	// Every field snapshot() reads is written under the same lock, so a poller
	// can never observe a half-initialised session. startedAt in particular must
	// be set here: leaving it zero made the UI render "1-01-01 08:05".
	s.mu.Lock()
	s.Quality = quality
	s.Kind = probe.Kind
	s.Name = probe.Name
	s.Artist = probe.Artist
	s.status = SessionQueued
	s.startedAt = time.Now()
	s.mu.Unlock()

	r.Emit = func(ev ProgressEvent) { s.emit(r, ev) }
	r.OnProgress = func(c Counter, tracks []AddedTrack) {
		s.mu.Lock()
		s.counter = c
		s.addedTracks = append([]AddedTrack(nil), tracks...)
		s.mu.Unlock()
	}

	go func() {
		defer close(s.done)
		s.setStatus(SessionRunning)
		s.appendLog(fmt.Sprintf("开始下载：%s - %s（%s）", s.Artist, s.Name, s.Quality))

		// Capture the engine's own stdout so the UI log pane shows real
		// progress lines such as "Track 3 of 12: ...".
		restore := captureStdout(func(line string) {
			if strings.TrimSpace(line) != "" {
				s.appendCarriageReturnFrames(line)
			}
		})
		defer restore()

		if err := r.runDownloadURLs(urls); err != nil {
			s.fail(err)
			s.appendLog("下载失败：" + err.Error())
			return
		}

		r.GetStateInto(&s.counter, &s.addedTracks)
		s.mu.Lock()
		s.outputDir = absPath(commonDir(s.addedTracks))
		s.mu.Unlock()

		// A run with zero successes is a failure even without an error return,
		// otherwise the UI would claim success on an empty result.
		if s.counter.Success == 0 {
			s.setStatus(SessionFailed)
			if s.err == nil {
				s.mu.Lock()
				s.err = fmt.Errorf("没有下载到任何内容")
				s.mu.Unlock()
			}
			s.appendLog("下载完成，但没有成功下载任何曲目")
			return
		}

		s.setStatus(SessionCompleted)
		s.appendLog(fmt.Sprintf("下载完成：成功 %d，警告 %d，错误 %d",
			s.counter.Success, s.counter.Unavailable+s.counter.NotSong, s.counter.Error))
	}()
}

// absPath resolves a possibly-relative download path against the working
// directory so the OS file manager can be pointed at it.
func absPath(p string) string {
	if p == "" {
		return ""
	}
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// commonDir returns the deepest directory shared by all downloaded files, which
// is what the "open folder" action reveals.
func commonDir(tracks []AddedTrack) string {
	if len(tracks) == 0 {
		return ""
	}
	dir := filepath.Dir(tracks[0].Path)
	for _, t := range tracks[1:] {
		d := filepath.Dir(t.Path)
		for !strings.HasPrefix(strings.ToLower(d), strings.ToLower(dir)) {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return dir
}

// captureStdout redirects os.Stdout to a pipe and invokes onLine for each line
// until the returned restore function is called. Only one download runs at a
// time in the GUI, so a process-wide swap is safe here; restore is deferred so
// stdout is always put back even on panic.
func captureStdout(onLine func(string)) func() {
	orig := os.Stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		return func() {}
	}
	os.Stdout = pw

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			onLine(sc.Text())
		}
		io.Copy(io.Discard, pr)
	}()

	return func() {
		pw.Close()
		wg.Wait()
		pr.Close()
		os.Stdout = orig
	}
}
