package app

import "testing"

// The launch rules are the whole contract of a release binary: what a double
// click does, and — just as importantly — that a mistyped flag never opens a
// window instead of reporting the mistake.

func TestGUIIntentNoArgumentsOpensGUI(t *testing.T) {
	if got := guiIntentFor(nil); got != guiImplicit {
		t.Fatalf("no arguments should request the GUI implicitly, got %v", got)
	}
	if got := guiIntentFor([]string{}); got != guiImplicit {
		t.Fatalf("empty argument list should request the GUI implicitly, got %v", got)
	}
}

func TestGUIIntentExplicitFlag(t *testing.T) {
	for _, args := range [][]string{
		{"--gui"},
		{"-g"},
		{"--gui=true"},
		{"--gui=1"},
		{"--gui="},
	} {
		if got := guiIntentFor(args); got != guiExplicit {
			t.Fatalf("%v should request the GUI explicitly, got %v", args, got)
		}
	}
}

// --gui=false is an opt-out and must beat the flag, otherwise there is no way to
// disable the GUI from a shortcut that already passes --gui.
func TestGUIIntentExplicitOptOut(t *testing.T) {
	for _, args := range [][]string{
		{"--gui=false"},
		{"--gui=0"},
		{"--gui=no"},
	} {
		if got := guiIntentFor(args); got != guiNone {
			t.Fatalf("%v should disable the GUI, got %v", args, got)
		}
	}
}

// A mistaken flag must print usage. If it opened the GUI the error would be
// hidden behind a window that gives no hint about what was wrong.
func TestGUIIntentUnknownFlagStaysCLI(t *testing.T) {
	for _, args := range [][]string{
		{"--guy"},
		{"-x"},
		{"--gui2"},
		{"--gui --nope"},
	} {
		if got := guiIntentFor(args); got != guiNone {
			t.Fatalf("%v should stay on the CLI path, got %v", args, got)
		}
	}
}

func TestGUIIntentHelpStaysCLI(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"-h"},
	} {
		if got := guiIntentFor(args); got != guiNone {
			t.Fatalf("%v should print usage, got %v", args, got)
		}
	}
}

func TestGUIIntentURLStaysCLI(t *testing.T) {
	urls := []string{
		"https://music.apple.com/cn/album/123",
		"https://music.apple.com/cn/song/456",
	}
	if got := guiIntentFor(urls); got != guiNone {
		t.Fatalf("URLs should download on the CLI path, got %v", got)
	}
}

// Real operations must keep working from a terminal, GUI or not.
func TestGUIIntentRealFlagsStayCLI(t *testing.T) {
	for _, args := range [][]string{
		{"--search", "album", "query"},
		{"--update"},
		{"--check-update"},
		{"--version"},
		{"-y", "--update"},
	} {
		if got := guiIntentFor(args); got != guiNone {
			t.Fatalf("%v should stay on the CLI path, got %v", args, got)
		}
	}
}

// --gui combined with a non-decisive flag is still an explicit GUI request: the
// user asked for the window, so a second flag must not cancel it.
func TestGUIIntentExplicitWinsOverOtherFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--gui", "--debug"},
		{"--debug", "-g"},
		{"-g", "--yes"},
	} {
		if got := guiIntentFor(args); got != guiExplicit {
			t.Fatalf("%v should still request the GUI, got %v", args, got)
		}
	}
}

// Help and URLs outrank an explicit --gui: both are requests for behaviour the
// GUI cannot provide, so they must not be swallowed by a stray flag.
func TestGUIIntentHelpAndURLBeatExplicitGUI(t *testing.T) {
	for _, args := range [][]string{
		{"--gui", "--help"},
		{"-h", "-g"},
		{"--gui", "https://music.apple.com/cn/album/1"},
	} {
		if got := guiIntentFor(args); got != guiNone {
			t.Fatalf("%v should stay on the CLI path, got %v", args, got)
		}
	}
}

func TestGUIIntentResultingFlagValue(t *testing.T) {
	// guiIntentFor must agree with the old boolean helper on every case that
	// reached this point. guiFlagValue reads os.Args, so the classifier itself is
	// exercised here rather than mutating process state.
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"--gui"}, true},
		{[]string{"-g"}, true},
		{[]string{"--gui=false"}, false},
		{[]string{"--help"}, false},
		{[]string{"https://music.apple.com/cn/album/1"}, false},
	}
	for _, c := range cases {
		got := guiIntentFor(c.args) != guiNone
		if got != c.want {
			t.Fatalf("guiIntentFor(%v) != guiNone is %v, want %v", c.args, got, c.want)
		}
	}
}
