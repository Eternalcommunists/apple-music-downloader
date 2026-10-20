package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// newTestHistory returns a store backed by a temp file so no test touches the
// real user config directory.
func newTestHistory(t *testing.T) *HistoryStore {
	t.Helper()
	return loadHistoryAt(filepath.Join(t.TempDir(), "history.json"))
}

// snapshotFixture builds a finished-job snapshot, so tests exercise the same
// summarize() projection that the server uses in production.
func snapshotFixture(id, started string, status string) SessionSnapshot {
	return SessionSnapshot{
		ID:        id,
		Status:    status,
		Name:      "Album " + id,
		Artist:    "Artist",
		Quality:   "alac",
		StartedAt: started,
		Total:     10,
		Success:   10,
	}
}

// summaryFixture builds a stored record directly, for the paths that take
// summaries rather than snapshots.
func summaryFixture(id, started string, status string) SessionSummary {
	return SessionSummary{
		ID:        id,
		Status:    status,
		Name:      "Album " + id,
		Artist:    "Artist",
		Quality:   "alac",
		StartedAt: started,
	}
}

func TestHistoryAddPersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")

	st := loadHistoryAt(path)
	st.add(snapshotFixture("a", "2026-10-19T10:00:00+08:00", SessionCompleted))

	// A fresh store on the same path simulates a program restart.
	reloaded := loadHistoryAt(path)
	got := reloaded.list()
	if len(got) != 1 {
		t.Fatalf("expected 1 persisted entry, got %d", len(got))
	}
	if got[0].ID != "a" || got[0].Name != "Album a" || got[0].Status != SessionCompleted {
		t.Fatalf("persisted entry lost fields: %+v", got[0])
	}
}

func TestHistoryNewestFirst(t *testing.T) {
	st := newTestHistory(t)
	st.add(snapshotFixture("old", "2026-10-18T10:00:00+08:00", SessionCompleted))
	st.add(snapshotFixture("new", "2026-10-19T10:00:00+08:00", SessionCompleted))

	got := st.list()
	if len(got) != 2 || got[0].ID != "new" || got[1].ID != "old" {
		t.Fatalf("expected newest first, got %+v", got)
	}
}

// Unfinished records must not survive a restart: their goroutine is gone, so
// they would render as permanently running rows.
func TestHistoryDropsUnfinishedOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	file := historyFile{
		Version: historyVersion,
		Entries: []SessionSummary{
			summaryFixture("done", "2026-10-19T10:00:00+08:00", SessionCompleted),
			summaryFixture("running", "2026-10-19T11:00:00+08:00", SessionRunning),
			summaryFixture("queued", "2026-10-19T12:00:00+08:00", SessionQueued),
		},
	}
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got := loadHistoryAt(path).list()
	if len(got) != 1 || got[0].ID != "done" {
		t.Fatalf("expected only the finished entry, got %+v", got)
	}

	// The pruned result must be written back, so the stale record does not sit
	// in the file until the next download happens to overwrite it.
	var onDisk historyFile
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal back: %v", err)
	}
	if len(onDisk.Entries) != 1 || onDisk.Entries[0].ID != "done" {
		t.Fatalf("expected the pruned list to be persisted, got %+v", onDisk.Entries)
	}
}

func TestHistoryCorruptFileIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt fixture: %v", err)
	}

	st := loadHistoryAt(path)
	if got := st.list(); len(got) != 0 {
		t.Fatalf("expected empty store from corrupt file, got %+v", got)
	}
	// The store must still be usable so the GUI recovers on the next job.
	st.add(snapshotFixture("a", "2026-10-19T10:00:00+08:00", SessionCompleted))
	if got := loadHistoryAt(path).list(); len(got) != 1 {
		t.Fatalf("expected store to be rewritten after corruption, got %+v", got)
	}
}

func TestHistoryMissingFileIsEmpty(t *testing.T) {
	st := loadHistoryAt(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if got := st.list(); len(got) != 0 {
		t.Fatalf("expected empty store, got %+v", got)
	}
}

func TestHistoryCapsEntriesKeepingNewest(t *testing.T) {
	st := newTestHistory(t)
	total := maxHistoryEntries + 5
	for i := 0; i < total; i++ {
		// Increasing timestamps make "newest" unambiguous, so the evicted
		// records can be identified exactly.
		ts := fmt.Sprintf("2026-01-01T00:00:%02d+08:00", i%60)
		st.add(snapshotFixture(fmt.Sprintf("id-%04d", i), ts, SessionCompleted))
	}

	got := st.list()
	if len(got) != maxHistoryEntries {
		t.Fatalf("expected cap %d, got %d", maxHistoryEntries, len(got))
	}
	// The most recent add must survive and the very first must be gone.
	if got[0].ID != fmt.Sprintf("id-%04d", total-1) {
		t.Fatalf("expected newest record to be kept, got %s", got[0].ID)
	}
	for _, e := range got {
		if e.ID == "id-0000" {
			t.Fatal("expected the oldest record to be evicted")
		}
	}
}

func TestPruneHistoryOrdersAndFilters(t *testing.T) {
	got := pruneHistory([]SessionSummary{
		summaryFixture("b", "2026-10-19T11:00:00+08:00", SessionCompleted),
		summaryFixture("a", "2026-10-19T10:00:00+08:00", SessionFailed),
		summaryFixture("running", "2026-10-19T12:00:00+08:00", SessionRunning),
		{ID: "   ", StartedAt: "2026-10-19T13:00:00+08:00", Status: SessionCompleted},
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 usable records, got %+v", got)
	}
	if got[0].ID != "b" || got[1].ID != "a" {
		t.Fatalf("expected newest first, got %+v", got)
	}
}

// mustTime parses a fixture timestamp, failing the test on a bad literal.
func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad test timestamp %q: %v", s, err)
	}
	return v
}

func TestHistoryRemoveAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	st := loadHistoryAt(path)
	st.add(snapshotFixture("a", "2026-10-19T10:00:00+08:00", SessionCompleted))
	st.add(snapshotFixture("b", "2026-10-19T11:00:00+08:00", SessionCompleted))

	if !st.remove("a") {
		t.Fatal("remove(a) should report success")
	}
	if st.remove("a") {
		t.Fatal("remove(a) twice should report failure")
	}
	if got := loadHistoryAt(path).list(); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("removal was not persisted, got %+v", got)
	}

	if n := st.clear(); n != 1 {
		t.Fatalf("clear should report 1 removed, got %d", n)
	}
	if got := loadHistoryAt(path).list(); len(got) != 0 {
		t.Fatalf("clear was not persisted, got %+v", got)
	}
	if n := st.clear(); n != 0 {
		t.Fatalf("clear on empty store should report 0, got %d", n)
	}
}

func TestHistoryFilePermissionsAndNoTempLeftBehind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	st := loadHistoryAt(path)
	st.add(snapshotFixture("a", "2026-10-19T10:00:00+08:00", SessionCompleted))

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file should have been renamed away, stat err=%v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("history file missing: %v", err)
	}
	// The file holds only local download metadata, so it must not be readable by
	// other accounts. Windows has no POSIX mode bits — Go always reports 0666
	// there — and the file sits under the per-user config directory, which the
	// OS already ACLs to that user.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("expected 0600 history file, got %o", perm)
		}
	}
	if info.Size() == 0 {
		t.Fatal("history file should not be empty after a save")
	}
}

// A job that finished while the window is open exists in both places. The live
// view must win so the row is not duplicated or stale.
func TestMergedJobsPrefersLiveSession(t *testing.T) {
	srv := &GUIServer{
		Sessions: NewSessionManager(),
		History:  newTestHistory(t),
	}

	stored := snapshotFixture("dup", "2026-10-19T10:00:00+08:00", SessionCompleted)
	stored.Percent = 100
	srv.History.add(stored)

	live := &DownloadSession{ID: "dup", done: make(chan struct{})}
	live.status = SessionRunning
	live.startedAt = mustTime(t, "2026-10-19T10:00:00+08:00")
	live.counter = Counter{Success: 3, Total: 10}
	srv.Sessions.add(live)

	got := srv.mergedJobs()
	if len(got) != 1 {
		t.Fatalf("expected the duplicate to collapse into one row, got %d", len(got))
	}
	if got[0].Status != SessionRunning || got[0].Processed != 3 {
		t.Fatalf("expected the live state to win, got %+v", got[0])
	}

	// A job only present on disk must still be listed after a restart.
	srv.History.add(snapshotFixture("old", "2026-10-18T10:00:00+08:00", SessionCompleted))
	got = srv.mergedJobs()
	if len(got) != 2 || got[0].ID != "dup" || got[1].ID != "old" {
		t.Fatalf("expected merged newest-first list, got %+v", got)
	}
}

func TestSessionDetailFallsBackToHistory(t *testing.T) {
	srv := &GUIServer{
		Sessions: NewSessionManager(),
		History:  newTestHistory(t),
	}
	srv.History.add(snapshotFixture("old", "2026-10-19T10:00:00+08:00", SessionCompleted))

	detail, ok := srv.sessionDetail("old")
	if !ok {
		t.Fatal("expected a stored record to resolve")
	}
	if detail.Name != "Album old" {
		t.Fatalf("unexpected record: %+v", detail)
	}
	// The file list is intentionally not persisted, so the UI must cope with it
	// being absent for a record from a previous run.
	if detail.AddedTracks != nil {
		t.Fatalf("expected no persisted file list, got %+v", detail.AddedTracks)
	}

	if _, ok := srv.sessionDetail("missing"); ok {
		t.Fatal("expected an unknown id to fail")
	}
}
