package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GUIServer exposes the local HTTP API the browser/WebView UI talks to. It
// binds to the loopback interface only and requires a per-run token, so no
// other process or page can drive it.
type GUIServer struct {
	Runner   *Runner
	Sessions *SessionManager
	Logins   *LoginManager
	// History is the persisted record of finished jobs. Sessions is memory-only
	// and dies with the process; History is what makes the history view survive
	// a restart.
	History *HistoryStore
	Token   string
	Addr    string
	// Port is the loopback port actually bound, which the UI prints so a user
	// can revisit the same address after a restart.
	Port int

	mu         sync.Mutex
	httpServer *http.Server
}

// DefaultGUIPort is the stable loopback port the UI prefers. Using one fixed
// port keeps an already-open browser tab (and any bookmark) working across
// restarts; a random port per launch invalidated every tab and was the direct
// cause of the repeated "Failed to fetch" reports.
const DefaultGUIPort = 8722

// NewGUIServer builds the server on the stable port, falling back to an
// ephemeral one only when that port is genuinely taken by someone else. The
// token is persisted too, so a reloading tab keeps authenticating.
func NewGUIServer(r *Runner) (*GUIServer, error) {
	st := loadGUIState()

	ln, port, err := listenStable(st.Port)
	if err != nil {
		return nil, err
	}
	addr := ln.Addr().String()
	ln.Close()

	tok := stableToken(st)
	if tok == "" {
		return nil, fmt.Errorf("无法生成界面访问令牌")
	}

	// Persist whatever we actually settled on, so the next run reuses it.
	saveGUIState(guiState{Port: port, Token: tok})

	return &GUIServer{
		Runner:   r,
		Sessions: NewSessionManager(),
		Logins:   NewLoginManager(),
		History:  loadHistory(),
		Token:    tok,
		Addr:     addr,
		Port:     port,
	}, nil
}

// listenStable binds a fixed loopback port, preferring the requested one and
// stepping through a few well-known alternatives before giving up and asking
// the OS for anything free. It returns the listener so the caller can learn the
// bound address before handing it to Serve.
func listenStable(preferred int) (net.Listener, int, error) {
	if preferred == 0 {
		preferred = DefaultGUIPort
	}
	// The preferred port first, then a few neighbours, then an ephemeral one.
	candidates := []int{preferred, DefaultGUIPort, DefaultGUIPort + 1, DefaultGUIPort + 2}
	seen := map[int]bool{}
	for _, p := range candidates {
		if p == 0 || seen[p] {
			continue
		}
		seen[p] = true
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return ln, p, nil
		}
	}
	// Everything was taken; fall back to an OS-assigned port so the GUI still
	// starts rather than refusing to run.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("cannot bind loopback port: %w", err)
	}
	return ln, ln.Addr().(*net.TCPAddr).Port, nil
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// Handler wires the API routes plus the embedded UI assets.
func (s *GUIServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/app.css", s.handleCSS)
	mux.HandleFunc("/app.js", s.handleJS)
	mux.HandleFunc("/api/probe", s.auth(s.handleProbe))
	mux.HandleFunc("/api/search", s.auth(s.handleSearch))
	mux.HandleFunc("/api/login/status", s.auth(s.handleLoginStatus))
	mux.HandleFunc("/api/login", s.auth(s.handleLogin))
	mux.HandleFunc("/api/login/progress", s.auth(s.handleLoginProgress))
	mux.HandleFunc("/api/preview", s.authMedia(s.handlePreview))
	mux.HandleFunc("/api/ping", s.auth(s.handlePing))
	mux.HandleFunc("/api/download", s.auth(s.handleDownload))
	mux.HandleFunc("/api/session", s.auth(s.handleSession))
	mux.HandleFunc("/api/tasks", s.auth(s.handleTasks))
	mux.HandleFunc("/api/history/clear", s.auth(s.handleHistoryClear))
	mux.HandleFunc("/api/history/delete", s.auth(s.handleHistoryDelete))
	mux.HandleFunc("/api/about", s.auth(s.handleAbout))
	mux.HandleFunc("/api/reveal", s.auth(s.handleReveal))
	mux.HandleFunc("/api/settings", s.auth(s.handleSettings))
	return mux
}

// auth requires the per-run token on every API call. The token is injected into
// the page, so same-origin UI calls carry it; a foreign page cannot read it.
func (s *GUIServer) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-AmDL-Token") != s.Token {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid token"})
			return
		}
		next(w, r)
	}
}

// authMedia authorises a request that is consumed by an <audio>/<video>
// element. Such elements cannot attach custom headers, so the token may also
// arrive as the `t` query parameter. The endpoint stays closed to foreign pages
// because the token is still required — it is simply carried in the URL, which
// never leaves this machine because the server is loopback-only.
func (s *GUIServer) authMedia(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("X-AmDL-Token")
		if tok == "" {
			tok = r.URL.Query().Get("t")
		}
		if tok != s.Token {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid token"})
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// jsonDecode reads a JSON body with a size cap, so a malformed or oversized
// request cannot exhaust memory.
func jsonDecode(r *http.Request, v any) error {
	const maxBody = 64 << 10
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	return dec.Decode(v)
}

func (s *GUIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.ReplaceAll(indexHTML, "__TOKEN__", s.Token)
	_, _ = w.Write([]byte(page))
}

func (s *GUIServer) handleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	// The assets are rebuilt with the binary, so a stale browser cache would
	// silently serve an older UI against a newer server.
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	_, _ = w.Write([]byte(appCSS))
}

func (s *GUIServer) handleJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	_, _ = w.Write([]byte(appJS))
}

// handleSettings reports the paths the UI shows, so the user can see where
// files will land before confirming.
func (s *GUIServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	c := s.Runner.Config
	writeJSON(w, http.StatusOK, map[string]any{
		"alac":        c.Paths.Alac,
		"atmos":       c.Paths.Atmos,
		"aac":         c.Paths.Aac,
		"mv":          c.Paths.MV,
		"liteServer":  c.General.LiteServer,
		"storefront":  c.General.Storefront,
		"historyFile": historyPath(),
		"stateFile":   guiStatePath(),
	})
}

// handleProbe resolves a pasted URL into displayable metadata plus the
// qualities that are actually available.
func (s *GUIServer) handleProbe(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	res, err := s.Runner.probeURL(rawURL, s.Runner.Token)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleSearch runs one page of catalog search for the GUI search box. The
// returned hits carry URLs that the normal probe flow accepts, so selecting a
// result reuses the exact same download path as a pasted link.
func (s *GUIServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("type")
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	res, err := s.Runner.searchAppleMusic(query, kind, offset)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type downloadRequest struct {
	URL     string `json:"url"`
	Quality string `json:"quality"`
}

// handleDownload starts a download job. An album URL that carries ?i=<song>
// downloads just that song; otherwise the whole album is downloaded.
func (s *GUIServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req downloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "URL 不能为空"})
		return
	}

	probe, err := s.Runner.probeURL(req.URL, s.Runner.Token)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	job := NewRunner(s.Runner.Config)
	job.Token = s.Runner.Token
	job.Flags = s.Runner.Flags
	job.NonInteractive = true
	if err := job.applyQualityFlavor(req.Quality); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	id, err := randomToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot create session"})
		return
	}
	sess := &DownloadSession{ID: id, done: make(chan struct{})}
	s.Sessions.add(sess)
	sess.startDownload(job, []string{req.URL}, probe, req.Quality)

	// Persist the outcome once the job settles. This is the only place history
	// is written, so the file is touched once per download rather than on every
	// poll.
	go func() {
		<-sess.done
		s.History.add(sess.snapshot())
	}()

	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

// allowedPreviewHost restricts the preview proxy to Apple's own clip CDNs, so
// the endpoint can never be turned into a general-purpose SSRF primitive.
func allowedPreviewHost(host string) bool {
	h := strings.ToLower(host)
	switch {
	case h == "audio-ssl.itunes.apple.com":
		return true
	case strings.HasSuffix(h, ".mzstatic.com"):
		return true
	case strings.HasSuffix(h, ".itunes.apple.com"):
		return true
	}
	return false
}

// handlePreview streams an Apple preview clip back to the UI with a corrected
// Content-Type. Apple serves these files as audio/x-m4p (the FairPlay MP4
// type) even though the bytes are a plain, unencrypted AAC-in-MP4 track, and
// browsers refuse to decode anything labelled x-m4p. Proxying also removes the
// cross-origin dependency, so playback works the same in Edge, Chrome and the
// embedded WebView.
func (s *GUIServer) handlePreview(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("u")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !allowedPreviewHost(u.Host) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "不允许的预览地址"})
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// Forward the range request so the browser can seek within the clip.
	if rg := r.Header.Get("Range"); rg != "" {
		req.Header.Set("Range", rg)
	}

	resp, err := previewHTTPClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "获取预览失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// Only the media type is rewritten; every other upstream header (length and
	// range info) is passed through so seeking keeps working.
	h := w.Header()
	h.Set("Content-Type", "audio/mp4")
	h.Set("Accept-Ranges", "bytes")
	h.Set("Cache-Control", "public, max-age=3600")
	for _, k := range []string{"Content-Length", "Content-Range"} {
		if v := resp.Header.Get(k); v != "" {
			h.Set(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// previewHTTPClient is shared by the preview proxy.
var previewHTTPClient = &http.Client{Timeout: 60 * time.Second}

// handlePing is the cheapest possible liveness check. The UI calls it while it
// believes the server is down, so it must not touch the Apple API, the
// lite-server, or any session state.
func (s *GUIServer) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "port": s.Port})
}

func (s *GUIServer) handleSession(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	detail, ok := s.sessionDetail(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

type revealRequest struct {
	Path string `json:"path"`
}

// handleReveal opens the OS file manager at the downloaded file or folder. The
// path is only ever used as an argument to a fixed system command, and it must
// exist, so the endpoint cannot be abused to run arbitrary programs.
func (s *GUIServer) handleReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req revealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := openInFileManager(req.Path); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// openInFileManager reveals path in the platform's file browser, selecting the
// file when the platform supports it.
func openInFileManager(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("路径为空")
	}
	if !pathExists(path) {
		return fmt.Errorf("路径不存在: %s", path)
	}
	switch runtime.GOOS {
	case "windows":
		if strings.HasPrefix(path, `\\`) || isDir(path) {
			return startCmd("explorer.exe", path)
		}
		// /select,<path> highlights the file inside its folder.
		return startCmd("explorer.exe", "/select,"+path)
	case "darwin":
		if isDir(path) {
			return startCmd("open", path)
		}
		return startCmd("open", "-R", path)
	default:
		return startCmd("xdg-open", path)
	}
}

// startCmd launches a helper without inheriting stdio or blocking, so a UI
// request returns immediately.
func startCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// pathExists reports whether path exists as a file or directory.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isDir reports whether path is a directory.
func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// Serve runs the HTTP server until the process exits.
func (s *GUIServer) Serve() error {
	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.httpServer = srv
	return srv.ListenAndServe()
}

// Shutdown stops the HTTP server gracefully. It is called when the UI window is
// closed so the process does not linger with nothing to serve.
func (s *GUIServer) Shutdown() error {
	s.mu.Lock()
	srv := s.httpServer
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// URL is the address the UI is reachable at.
func (s *GUIServer) URL() string {
	return "http://" + s.Addr + "/"
}
