package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Release-friendly startup.
//
// A released .exe is normally started by double-clicking it, which passes no
// arguments at all. Until now that printed the CLI usage and exited — correct
// for a tool that lives in a terminal, useless for one shipped with a GUI. The
// rules here are deliberately narrow so command-line users see no change:
//
//   - No arguments                 -> open the GUI.
//   - --help / -h, or an unknown flag -> print usage (never an accidental GUI).
//   - A URL or any real operation (--search, --update, --version, ...) -> CLI.

// guiIntent records how the GUI was requested, which changes how failures are
// reported. A failure in a double-clicked window can only be shown in a message
// box, while a failure from a terminal should be printed and left on screen.
type guiIntent int

const (
	// guiNone means the CLI path should run.
	guiNone guiIntent = iota
	// guiExplicit means --gui / -g was passed on the command line.
	guiExplicit
	// guiImplicit means the process was started with no arguments at all.
	guiImplicit
)

// guiFlagValue keeps its original contract for callers that only care whether
// the GUI was asked for, and delegates to the shared parser below.
func guiFlagValue() bool { return guiIntentFor(os.Args[1:]) != guiNone }

// guiIntentFor classifies the raw arguments. It scans os.Args directly rather
// than using pflag because this decision must be made *before* pflag is
// configured: pflag's own Parse would reject --gui as an unknown flag.
//
// The rules are ordered, and the order is the contract:
//
//  1. --help / -h anywhere              -> CLI help
//  2. a bare argument (a URL) anywhere  -> CLI download
//  3. --gui=false / --gui=0 anywhere    -> CLI
//  4. --gui / -g anywhere               -> explicit GUI
//  5. no arguments                      -> implicit GUI (if no console)
//  6. anything else                     -> CLI
//
// Rules 1 and 2 outrank an explicit --gui because both are requests for output
// the GUI cannot provide: help is text, and a URL is a download the user expects
// to happen. Rule 6 is what makes a mistyped flag print usage instead of
// silently opening a window that hides the mistake.
func guiIntentFor(args []string) guiIntent {
	if len(args) == 0 {
		return guiImplicit
	}

	sawGUI := false
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			// Help is a request for text the GUI cannot show, so it always wins
			// and is handled by pflag further down.
			return guiNone
		case strings.HasPrefix(a, "--gui="):
			// An explicit opt-out must beat the flag, otherwise a shortcut that
			// already passes --gui could never be configured to use the CLI.
			v := strings.TrimPrefix(a, "--gui=")
			if v != "" && v != "true" && v != "1" {
				return guiNone
			}
			sawGUI = true
		case a == "--gui" || a == "-g":
			sawGUI = true
		case strings.HasPrefix(a, "-"):
			// Another flag is not decisive on its own: --gui may still appear,
			// or may already have been seen. A *mistyped* flag ends up here and
			// falls through to guiNone, so it prints usage instead of silently
			// opening a window that hides the mistake.
		default:
			// A bare argument is a URL, which is the CLI download path.
			return guiNone
		}
	}
	if sawGUI {
		return guiExplicit
	}
	return guiNone
}

// isDoubleClicked reports whether the process was launched without a console,
// which is what happens when a user double-clicks a .exe in Explorer. The
// platform-specific check lives in gui_launch_windows.go / gui_launch_unix.go.
func isDoubleClicked() bool { return !hasConsole() }

// reportUIFailure surfaces a startup failure in whichever way the user can
// actually see it. A double-clicked window has no console, so anything printed
// is discarded and the program would otherwise look like it did nothing at all.
func reportUIFailure(err error) {
	if err == nil {
		return
	}
	if hasConsole() {
		fmt.Fprintln(os.Stderr, "GUI 启动失败:", err)
		return
	}
	showMessageBox("启动失败", "Apple Music 下载器无法启动。\n\n"+err.Error())
}

// userFriendlyGUIError turns internal errors into advice with a next step. A
// missing config is by far the most common first-run failure, and the raw error
// ("open config.yaml: no such file") tells the user nothing about what to do.
func userFriendlyGUIError(err error, exePath string) error {
	if err == nil {
		return nil
	}
	low := strings.ToLower(err.Error())
	missingConfig := strings.Contains(low, "config") &&
		(strings.Contains(low, "no such file") ||
			strings.Contains(low, "not exist") ||
			strings.Contains(low, "cannot find") ||
			strings.Contains(low, "找不到"))
	if missingConfig {
		return fmt.Errorf(
			"没有找到配置文件 config.yaml。\n\n"+
				"请把 config.yaml 放在程序所在目录：\n%s\n\n"+
				"如果还没有该文件，可以从发行包中的 config.yaml.example 复制一份，\n"+
				"重命名为 config.yaml 后按注释填写 lite-server 等选项。",
			filepath.Dir(exePath))
	}
	if strings.Contains(low, "lite-server") {
		return fmt.Errorf(
			"%v\n\n"+
				"请先启动并登录 wrapper-lite，然后在 config.yaml 中确认：\n"+
				"general.lite-server: \"http://127.0.0.1:12340\"",
			err)
	}
	return err
}

// configLookupPaths lists the directories searched for config.yaml when the GUI
// is started by double-click. The executable's own directory comes first so a
// portable layout (the exe and its config side by side) always wins.
func configLookupPaths() []string {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, filepath.Join(d, "config.yaml"))
	}
	return out
}

// resolveImplicitConfig returns the first existing config.yaml from the lookup
// paths, or an empty string to let the normal loader report the failure.
func resolveImplicitConfig() string {
	for _, p := range configLookupPaths() {
		if pathExists(p) {
			return p
		}
	}
	return ""
}

// executablePath returns the running binary's path, falling back to its bare
// argument when the OS cannot report it. Callers only use it to name a
// directory in a message, so a best effort is enough.
func executablePath() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return os.Args[0]
}
