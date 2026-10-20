package app

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/pflag"

	ampapi "amdl/internal/amp-api"
	"amdl/internal/config"
	"amdl/internal/download"
)

// RunGUI starts the graphical front end. It loads the same config.yaml as the
// CLI, reuses the same download engine, and serves the UI on loopback for a
// WebView2 window (falling back to the default browser).
func RunGUI() error {
	// pflag must know about the flag or it will reject it as unknown.
	pflag.BoolP("gui", "g", false, "Launch the graphical interface")
	var configFile string
	pflag.StringVarP(&configFile, "config", "c", "", "Path to custom config file (default: config.yaml)")
	pflag.CommandLine.ParseErrorsWhitelist.UnknownFlags = true
	pflag.Parse()

	// A double-click sets the working directory to whatever the shortcut or the
	// file manager happened to choose, so the default config is resolved from
	// the executable's directory first rather than from the cwd alone. An
	// explicit -c always wins.
	if configFile == "" {
		configFile = resolveImplicitConfig()
	}

	r := NewRunner(config.ConfigSet{})
	if err := r.loadConfig(config.LoadOptions{ConfigFile: configFile}); err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	if r.Config.General.LiteServer == "" {
		return fmt.Errorf("config.yaml 中未配置 lite-server")
	}
	if err := download.Init(r.Config.General.Proxy); err != nil {
		return fmt.Errorf("代理配置错误: %w", err)
	}

	// The GUI talks to the Apple API directly, so it needs a bearer token just
	// like the CLI does.
	token, err := fetchTokenWithRetry()
	if err != nil {
		if r.Config.General.AuthorizationToken != "" && r.Config.General.AuthorizationToken != "your-authorization-token" {
			token = strings.Replace(r.Config.General.AuthorizationToken, "Bearer ", "", -1)
		} else {
			return fmt.Errorf("无法获取 Apple Music token: %w", err)
		}
	}
	r.Token = token
	// A GUI window must never block on stdin.
	r.NonInteractive = true

	srv, err := NewGUIServer(r)
	if err != nil {
		return err
	}

	go func() {
		if err := srv.Serve(); err != nil {
			fmt.Fprintln(os.Stderr, "GUI server stopped:", err)
			os.Exit(1)
		}
	}()

	// Give the listener a moment, then verify it answers before opening a window.
	if err := waitForServer(srv.URL(), 5*time.Second); err != nil {
		return err
	}

	fmt.Println("界面已在 ", srv.URL(), " 启动")
	fmt.Printf("该地址固定（端口 %d），重启程序后依然可用，书签或已打开的标签页无需更换。\n", srv.Port)
	fmt.Println("如界面提示“与本地服务断开连接”，重新启动程序即可自动恢复。关闭窗口或按 Ctrl+C 停止服务。")

	started := time.Now()
	win, err := openGUIWindow(srv.URL())
	if err != nil {
		fmt.Println("无法打开独立窗口，已改用默认浏览器：", err)
		if berr := openInDefaultBrowser(srv.URL()); berr != nil {
			return fmt.Errorf("无法打开界面: %w", berr)
		}
	}

	// Ctrl+C (or a terminal close) must also stop the server, so the process
	// never lingers in the background holding the port.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		fmt.Println("\n正在停止界面服务…")
		_ = srv.Shutdown()
		os.Exit(0)
	}()

	if win != nil {
		// Block until the UI window exits, then fall through and shut down so
		// the server lifetime matches the window lifetime.
		werr := win.Wait()
		if werr != nil {
			fmt.Fprintln(os.Stderr, "窗口已关闭：", werr)
		}
		// A browser that hands the window to an already-running instance exits
		// immediately while the window stays open. Treating that as a close
		// would kill the server out from under the user, so a very short
		// lifetime with a still-serving UI is treated as a hand-off instead.
		if time.Since(started) < 3*time.Second && httpGetOK(srv.URL()) {
			fmt.Println("窗口已交由现有浏览器进程托管；按 Ctrl+C 停止服务。")
			select {}
		}
	} else {
		// No window process to track (browser fallback): wait for Ctrl+C only.
		select {}
	}

	fmt.Println("窗口已关闭，正在停止界面服务…")
	return srv.Shutdown()
}

// waitForServer polls the URL until it responds or the timeout elapses.
func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if httpGetOK(url) {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return fmt.Errorf("界面服务未能在 %s 内启动", timeout)
}

// tokenFetchAttempts and tokenFetchBackoff control the retry around Apple's
// token bootstrap.
//
// Fetching the token means two sequential HTTPS requests to music.apple.com.
// Through a local proxy or VPN that drops the occasional connection, that
// bootstrap can fail transiently while everything else works — and because a
// failure here aborts startup entirely, the window would never open. Retrying
// a few times turns a flaky network into a slower start instead of no start.
const (
	tokenFetchAttempts = 3
	tokenFetchBackoff  = 1500 * time.Millisecond
)

// fetchTokenWithRetry calls ampapi.GetToken, retrying transient failures.
func fetchTokenWithRetry() (string, error) {
	var lastErr error
	for attempt := 1; attempt <= tokenFetchAttempts; attempt++ {
		token, err := ampapi.GetToken()
		if err == nil {
			return token, nil
		}
		lastErr = err
		if attempt < tokenFetchAttempts {
			time.Sleep(tokenFetchBackoff)
		}
	}
	return "", lastErr
}

// openGUIWindow opens a dedicated application window and returns the handle so
// the caller can stop the server when the user closes it. On Windows the Edge
// WebView2 runtime renders it as a chromeless app window; elsewhere, a browser
// window in app mode is used. A nil handle means a fallback browser was opened
// instead, whose lifetime cannot be tracked.
func openGUIWindow(url string) (*exec.Cmd, error) {
	// A per-run profile directory is required, not cosmetic: browsers file-lock
	// the profile, so reusing one directory lets a second launch hand the window
	// to the first process and exit immediately, which would make the launcher
	// think the window was closed the moment it opened.
	profile := filepath.Join(os.TempDir(), "amdl-gui-profile-"+randomTokenOrFallback())

	switch {
	case webView2Available():
		edge := edgeExecutable()
		if edge == "" {
			return nil, fmt.Errorf("WebView2 已安装但未找到 Edge 可执行文件")
		}
		return startWindow(edge, "--app="+url, "--window-size=980,900", "--user-data-dir="+profile)
	case msedgeAvailable():
		return startWindow(edgeExecutable(), "--app="+url, "--window-size=980,900", "--user-data-dir="+profile)
	case chromeAvailable():
		return startWindow(chromeExecutable(), "--app="+url, "--window-size=980,900", "--user-data-dir="+profile)
	default:
		return nil, fmt.Errorf("未找到可用的 WebView2 / Edge / Chrome 运行时")
	}
}

// randomTokenOrFallback returns a short unique suffix for the window profile.
func randomTokenOrFallback() string {
	tok, err := randomToken()
	if err != nil || len(tok) < 12 {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return tok[:12]
}

// startWindow launches the browser window and returns its command handle.
func startWindow(name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// webView2Available reports whether the Edge WebView2 runtime is installed.
func webView2Available() bool {
	roots := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "EdgeWebView", "Application"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "EdgeWebView", "Application"),
	}
	for _, root := range roots {
		if root == "" || root == "\\" {
			continue
		}
		if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
			return true
		}
	}
	return false
}

// edgeExecutable returns the first existing msedge.exe path.
func edgeExecutable() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		candidates = append(candidates, filepath.Join(local, "Microsoft", "Edge", "Application", "msedge.exe"))
	}
	for _, p := range candidates {
		if p != "" && pathExists(p) {
			return p
		}
	}
	return ""
}

func msedgeAvailable() bool { return edgeExecutable() != "" }

// chromeExecutable returns the first existing chrome.exe path.
func chromeExecutable() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		candidates = append(candidates, filepath.Join(local, "Google", "Chrome", "Application", "chrome.exe"))
	}
	for _, p := range candidates {
		if p != "" && pathExists(p) {
			return p
		}
	}
	return ""
}

func chromeAvailable() bool { return chromeExecutable() != "" }

// openInDefaultBrowser opens url with the platform's default handler.
func openInDefaultBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return startCmd("cmd", "/c", "start", "", url)
	case "darwin":
		return startCmd("open", url)
	default:
		return startCmd("xdg-open", url)
	}
}

// httpClientWithTimeout returns a client suitable for the readiness probe.
func httpClientWithTimeout(d time.Duration) *http.Client {
	return &http.Client{Timeout: d}
}

// httpGetOK performs a short GET and reports whether it succeeded.
func httpGetOK(url string) bool {
	resp, err := httpClientWithTimeout(2 * time.Second).Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}
