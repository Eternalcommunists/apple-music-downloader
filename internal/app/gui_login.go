package app

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"amdl/internal/wrapper"
)

// Credential environment variables handed to the configured login command.
// Passing them through the environment keeps the command template free of user
// input, so a password can never be interpreted as shell syntax.
const (
	envLoginUser = "AMDL_USER"
	envLoginPass = "AMDL_PASS"
	envLoginCode = "AMDL_CODE"
)

// loginTimeout caps how long a sign-in attempt may run before it is killed.
// wrapper-lite waits up to 60s for a 2FA code, so this sits above that.
const loginTimeout = 150 * time.Second

// loginMaxOutput caps captured output length, so a chatty command cannot make
// the session state unbounded.
const loginMaxOutput = 8 * 1024

// redactPatterns scrub anything that looks like a credential out of captured
// login output before it is shown or stored.
var redactPatterns = []*regexp.Regexp{
	// Apple IDs are emails or long digit strings (e.g. a phone-number account).
	regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	regexp.MustCompile(`\b\d{10,}\b`),
	// macOS keychain / credential prompts echo the account next to a colon.
	regexp.MustCompile(`(?i)(password|passwd|pass)\s*[:=]\s*\S+`),
	regexp.MustCompile(`<key>password</key>\s*<string>[^<]*</string>`),
}

// LoginSession tracks one sign-in attempt. Like DownloadSession it owns its
// state behind a mutex so the HTTP poller only ever reads a consistent copy.
type LoginSession struct {
	ID string

	mu         sync.Mutex
	status     string // running | completed | failed
	message    string
	detail     string
	output     []string
	startedAt  time.Time
	finishedAt time.Time
	err        error
}

// LoginSnapshot is the poll payload for the sign-in dialog.
type LoginSnapshot struct {
	ID         string   `json:"id"`
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	Detail     string   `json:"detail"`
	Output     []string `json:"output"`
	StartedAt  string   `json:"startedAt"`
	FinishedAt string   `json:"finishedAt"`
	ErrorText  string   `json:"errorText"`
	LoggedIn   bool     `json:"loggedIn"`
	Regions    []string `json:"regions"`
}

// LoginManager keeps the active sign-in attempts. At most one runs at a time,
// because wrapper-lite writes to a single shared token store.
type LoginManager struct {
	mu      sync.Mutex
	current *LoginSession
	busy    bool
}

func NewLoginManager() *LoginManager { return &LoginManager{} }

// start rejects a second concurrent attempt and returns the new session.
func (m *LoginManager) start() (*LoginSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busy {
		return nil, errors.New("已有一个登录正在进行，请等待它结束")
	}
	id, err := randomToken()
	if err != nil {
		return nil, err
	}
	s := &LoginSession{ID: id, status: SessionRunning, startedAt: time.Now()}
	m.current = s
	m.busy = true
	return s, nil
}

func (m *LoginManager) finish() {
	m.mu.Lock()
	m.busy = false
	m.mu.Unlock()
}

func (m *LoginManager) get(id string) (*LoginSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil || m.current.ID != id {
		return nil, false
	}
	return m.current, true
}

func (m *LoginManager) snapshot() *LoginSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

func (s *LoginSession) appendOutput(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(text) == 0 {
		return
	}
	if len(text) > loginMaxOutput {
		text = text[:loginMaxOutput] + "…"
	}
	s.output = append(s.output, text)
	if len(s.output) > 200 {
		s.output = s.output[len(s.output)-200:]
	}
}

func (s *LoginSession) setStatus(status, message, detail string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	s.message = message
	s.detail = detail
	s.err = err
	if status == SessionCompleted || status == SessionFailed {
		s.finishedAt = time.Now()
	}
}

func (s *LoginSession) snapshotPayload(loggedIn bool, regions []string) LoginSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	finished := ""
	if !s.finishedAt.IsZero() {
		finished = s.finishedAt.Format(time.RFC3339)
	}
	errText := ""
	if s.err != nil {
		errText = s.err.Error()
	}
	out := make([]string, len(s.output))
	copy(out, s.output)

	return LoginSnapshot{
		ID:         s.ID,
		Status:     s.status,
		Message:    s.message,
		Detail:     s.detail,
		Output:     out,
		StartedAt:  s.startedAt.Format(time.RFC3339),
		FinishedAt: finished,
		ErrorText:  errText,
		LoggedIn:   loggedIn,
		Regions:    regions,
	}
}

// loginStatus reports whether wrapper-lite currently holds a usable session.
// An empty region list means it is running but not signed in, which is exactly
// what the first-run dialog needs to detect.
func (r *Runner) loginStatus() (bool, []string) {
	if r.Config.General.LiteServer == "" {
		return false, nil
	}
	regions, err := wrapper.GetStatus(r.Config.General.LiteServer)
	if err != nil {
		return false, nil
	}
	return len(regions) > 0, regions
}

// redactCredentials removes anything credential-shaped from login output.
func redactCredentials(s string) string {
	for _, re := range redactPatterns {
		s = re.ReplaceAllString(s, "[已隐藏]")
	}
	return s
}

// loginCommandParts splits the configured template into program and arguments.
// The template is author-controlled config, never assembled from user input.
func loginCommandParts(template string) (string, []string, error) {
	fields := strings.Fields(template)
	if len(fields) == 0 {
		return "", nil, errors.New("未配置登录命令（config.yaml 的 general.lite-login-command）")
	}
	if strings.TrimSpace(fields[0]) == "" {
		return "", nil, errors.New("登录命令的第一个词必须是可执行文件")
	}
	return fields[0], fields[1:], nil
}

// runLogin executes the configured sign-in command with credentials supplied
// through the environment. It blocks until the command exits, then refreshes
// the session so the caller can report the resulting regions.
func (s *LoginSession) runLogin(r *Runner, user, pass, code string) {
	defer func() {
		// Drop the credential values once the attempt is over.
		user, pass, code = "", "", ""
	}()

	prog, args, err := loginCommandParts(r.Config.General.LiteLoginCommand)
	if err != nil {
		s.setStatus(SessionFailed, "无法登录", err.Error(), err)
		return
	}

	cmd := exec.Command(prog, args...)
	cmd.Env = append(cmd.Environ(),
		envLoginUser+"="+user,
		envLoginPass+"="+pass,
	)
	if code != "" {
		cmd.Env = append(cmd.Env, envLoginCode+"="+code)
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		s.setStatus(SessionFailed, "无法启动登录命令",
			fmt.Sprintf("检查 general.lite-login-command 是否指向可执行文件: %v", err), err)
		return
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case werr := <-done:
		s.appendOutput(redactCredentials(buf.String()))
		if werr != nil {
			s.setStatus(SessionFailed, "登录失败", redactCredentials(buf.String()), werr)
			return
		}
	case <-time.After(loginTimeout):
		killProcessGroup(cmd)
		<-done
		s.appendOutput(redactCredentials(buf.String()))
		s.setStatus(SessionFailed, "登录超时",
			fmt.Sprintf("超过 %s 仍未完成，可能是在等待 2FA 验证码", loginTimeout), errors.New("timeout"))
		return
	}

	loggedIn, regions := r.loginStatus()
	if !loggedIn {
		// The command exited cleanly but the service still has no session,
		// which usually means wrong credentials or an incomplete 2FA step.
		s.setStatus(SessionFailed, "登录未生效",
			"命令已结束，但 wrapper-lite 仍未取得会话。请确认账号密码正确，并检查是否需要 2FA 验证码。", nil)
		return
	}
	s.setStatus(SessionCompleted, "登录成功",
		"已取得可用区域: "+strings.Join(regions, ", "), nil)
}

// credentialRequest is the login form payload.
type credentialRequest struct {
	User string `json:"user"`
	Pass string `json:"pass"`
	Code string `json:"code"`
}

// handleLoginStatus reports whether the service is reachable and signed in,
// which is what the first-run dialog checks before showing the form.
func (s *GUIServer) handleLoginStatus(w http.ResponseWriter, r *http.Request) {
	loggedIn, regions := s.Runner.loginStatus()
	configured := strings.TrimSpace(s.Runner.Config.General.LiteLoginCommand) != ""
	writeJSON(w, http.StatusOK, map[string]any{
		"loggedIn":   loggedIn,
		"regions":    regions,
		"configured": configured,
		"liteServer": s.Runner.Config.General.LiteServer,
	})
}

// handleLogin starts a sign-in attempt. Credentials arrive in the POST body
// and are handed to the configured command through the environment.
func (s *GUIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var req credentialRequest
	if err := jsonDecode(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
		return
	}
	if strings.TrimSpace(req.User) == "" || req.Pass == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请输入账号与密码"})
		return
	}
	if strings.TrimSpace(s.Runner.Config.General.LiteLoginCommand) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "未配置登录命令，请在 config.yaml 设置 general.lite-login-command",
		})
		return
	}

	sess, err := s.Logins.start()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	user, pass, code := req.User, req.Pass, req.Code
	go func() {
		defer s.Logins.finish()
		sess.runLogin(s.Runner, user, pass, code)
	}()

	writeJSON(w, http.StatusOK, map[string]string{"id": sess.ID})
}

// handleLoginProgress polls one sign-in attempt.
func (s *GUIServer) handleLoginProgress(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	sess, ok := s.Logins.get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "登录会话不存在"})
		return
	}
	loggedIn, regions := s.Runner.loginStatus()
	writeJSON(w, http.StatusOK, sess.snapshotPayload(loggedIn, regions))
}

// setProcessGroup / killProcessGroup give the login command its own process
// group so a timeout kills the whole tree, not just the direct child.
func setProcessGroup(cmd *exec.Cmd) {
	setSysProcAttr(cmd)
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	killSysProcAttr(cmd)
}
