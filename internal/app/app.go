package app

import (
	"amdl/internal/config"
	"os"
	"regexp"
)

var (
	forbiddenNames = regexp.MustCompile(`[/\\<>:"|?*]`)
)

type Flags struct {
	Atmos          bool
	AAC            bool
	Select         bool
	ArtistSelect   bool
	Debug          bool
	PrintJSON      bool
	SaveM3U8       bool
	LiteServerFlag string
	Version        bool
	Update         bool
	CheckUpdate    bool
	Yes            bool
	ForceUpdate    bool
	MigrateConfig  bool
}

type State struct {
	Counter     Counter
	OKDict      map[string][]int
	AddedTracks []AddedTrack
}

type Runner struct {
	Config config.ConfigSet
	Flags  Flags
	State  State

	// Token is the Apple Music bearer token fetched once at startup and reused
	// by every download, including GUI-initiated ones.
	Token string

	// Emit receives structured progress events. It is nil for CLI runs, which
	// print to stdout instead.
	Emit func(ProgressEvent)

	// OnProgress receives a live counter/file snapshot as each track finishes,
	// so a front end can advance a progress bar during the run. It is called
	// only from the download goroutine.
	OnProgress func(Counter, []AddedTrack)

	// NonInteractive disables the "press Enter to retry" loop so an automated
	// front end never blocks waiting on stdin.
	NonInteractive bool
}

func NewRunner(cfg config.ConfigSet) *Runner {
	return &Runner{Config: cfg, State: State{OKDict: make(map[string][]int)}}
}

// emit sends a progress event when a listener is attached.
func (r *Runner) emit(ev ProgressEvent) {
	if r.Emit != nil {
		r.Emit(ev)
	}
}

// notifyProgress publishes a live snapshot to OnProgress. It is called from the
// download goroutine wherever a track finishes, which keeps the GUI bar moving
// instead of jumping from 0% to 100% at the end.
func (r *Runner) notifyProgress() {
	if r.OnProgress == nil {
		return
	}
	r.OnProgress(r.State.Counter, r.State.AddedTracks)
}

// GetStateInto copies the live counters and downloaded-file list out of the
// runner. It is called from the download goroutine itself, which is the only
// writer of State, so no locking is required here.
func (r *Runner) GetStateInto(counter *Counter, tracks *[]AddedTrack) {
	*counter = r.State.Counter
	*tracks = append([]AddedTrack(nil), r.State.AddedTracks...)
}

type AddedTrack struct {
	Path     string `json:"path"`
	Artist   string `json:"artist"`
	ArtistID string `json:"artist_id"`
	Album    string `json:"album"`
	Song     string `json:"song"`
}

// topLevelKeys returns the set of top-level YAML keys in data.

// contains reports whether item is present in slice.
func contains[T comparable](slice []T, item T) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func (r *Runner) LimitString(s string) string {
	if len([]rune(s)) > r.Config.Paths.LimitMax {
		return string([]rune(s)[:r.Config.Paths.LimitMax])
	}
	return s
}

func fileExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		return !f.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
