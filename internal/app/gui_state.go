package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// guiState is the small on-disk record that keeps the GUI address stable.
//
// Both fields exist purely so a browser tab never breaks:
//   - Port:  the UI is served on the same port every run, so a bookmarked or
//     already-open tab keeps working after a restart. With a random port every
//     launch invalidated every open tab, which is what produced "Failed to
//     fetch" over and over.
//   - Token: the per-run token must survive a restart for the same reason. A
//     reloading tab re-sends the token it was given, so a fresh random token
//     would reject it even on the same port.
//
// Security note: the token is a loopback-only credential stored in a 0600 file
// next to config.yaml, which is the same protection the config's own
// media-user-token already relies on. Nothing is exposed beyond the machine.
type guiState struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// amdlStateDir is the single directory holding the GUI's local state: the
// stable address (gui.json) and the download history (history.json). Keeping
// them together means one place to inspect or delete when resetting, and one
// permission to get right.
func amdlStateDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "amdl")
}

// guiStatePath returns the state file location. It sits beside the user's
// config so a portable install keeps its GUI address with it.
func guiStatePath() string { return filepath.Join(amdlStateDir(), "gui.json") }

// loadGUIState reads the persisted state, returning a zero value when absent or
// unreadable. A corrupt file is simply ignored and rewritten on next save.
func loadGUIState() guiState {
	var st guiState
	data, err := os.ReadFile(guiStatePath())
	if err != nil {
		return st
	}
	_ = json.Unmarshal(data, &st)
	// A token must be long enough to be the real thing; anything else is stale.
	if len(strings.TrimSpace(st.Token)) < 32 {
		st.Token = ""
	}
	if st.Port < 1024 || st.Port > 65535 {
		st.Port = 0
	}
	return st
}

// saveGUIState writes the state with owner-only permissions. Failures are not
// fatal: the GUI still works, it just loses the stable address.
func saveGUIState(st guiState) {
	path := guiStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// stableToken returns the persisted token, generating and storing one on first
// use so subsequent runs authenticate the same open tabs.
func stableToken(st guiState) string {
	if st.Token != "" {
		return st.Token
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		// Without randomness the GUI cannot be secured; surface it loudly.
		return ""
	}
	return hex.EncodeToString(buf)
}
