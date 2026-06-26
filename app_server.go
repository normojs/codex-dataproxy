package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/portapps/portapps/v3/pkg/log"
)

const (
	appServerStatusDisabled    = "disabled"
	appServerStatusUnavailable = "unavailable"
	appServerStatusPending     = "pending"
	appServerStatusStarting    = "starting"
	appServerStatusRunning     = "running"
	appServerStatusStopping    = "stopping"
	appServerStatusStopped     = "stopped"
	appServerStatusFailed      = "failed"
)

type appServerStatus struct {
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"`
	URL       string `json:"url"`
	TokenFile string `json:"token_file"`
	PID       int    `json:"pid"`
	Error     string `json:"error,omitempty"`
}

type localAppServer struct {
	mu        sync.RWMutex
	enabled   bool
	host      string
	port      int
	url       string
	tokenFile string
	cliPath   string
	status    string
	lastError string
	cmd       *exec.Cmd
	pid       int
	stopping  bool
}

func newLocalAppServer(portableCodexHome string) *localAppServer {
	config := cfg.AppServer
	host := normalizeAppServerHost(config.Host)
	port := config.Port
	if port <= 0 {
		port = defaultAppServerPort
	}

	tokenFile, err := filepath.Abs(filepath.Join(portableCodexHome, appServerTokenName))
	if err != nil {
		tokenFile = filepath.Join(portableCodexHome, appServerTokenName)
	}

	status := appServerStatusPending
	if !config.Enabled {
		status = appServerStatusDisabled
	}

	server := &localAppServer{
		enabled:   config.Enabled,
		host:      host,
		port:      port,
		url:       appServerURL(host, port),
		tokenFile: tokenFile,
		cliPath:   localCodexCLIPath(),
		status:    status,
	}
	if server.enabled && server.cliPath == "" {
		server.status = appServerStatusUnavailable
		server.lastError = "cannot find bundled codex CLI"
	}
	return server
}

func normalizeAppServerHost(host string) string {
	host = strings.TrimSpace(host)
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return defaultAppServerHost
	default:
		return defaultAppServerHost
	}
}

func localCodexCLIPath() string {
	if app == nil || app.AppPath == "" {
		return ""
	}

	candidates := []string{}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, filepath.Join(app.AppPath, "Codex.app", "Contents", "Resources", "codex"))
	}
	candidates = append(candidates,
		filepath.Join(app.AppPath, "resources", "codex.exe"),
		filepath.Join(app.AppPath, "resources", "codex"),
	)
	if runtime.GOOS == "windows" {
		candidates = append([]string{filepath.Join(app.AppPath, "resources", "codex.exe")}, candidates...)
	}

	for _, candidate := range candidates {
		if regularFileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *localAppServer) start(assign func(int) error) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	if !s.enabled {
		s.status = appServerStatusDisabled
		s.mu.Unlock()
		return nil
	}
	if s.cmd != nil && s.pid != 0 {
		s.mu.Unlock()
		return nil
	}
	if s.cliPath == "" {
		s.status = appServerStatusUnavailable
		s.lastError = "cannot find bundled codex CLI"
		err := errors.New(s.lastError)
		s.mu.Unlock()
		return err
	}
	s.status = appServerStatusStarting
	s.lastError = ""
	s.mu.Unlock()

	if err := s.prepareEndpoint(); err != nil {
		s.markFailed(err)
		return err
	}
	if err := s.ensureTokenFile(); err != nil {
		s.markFailed(err)
		return err
	}

	cmd := s.command()
	if err := cmd.Start(); err != nil {
		s.markFailed(err)
		return err
	}

	if assign != nil {
		if err := assign(cmd.Process.Pid); err != nil {
			log.Warn().Err(err).Msg("Cannot attach app-server process to cleanup job")
		}
	}

	s.mu.Lock()
	s.cmd = cmd
	s.pid = cmd.Process.Pid
	s.status = appServerStatusRunning
	s.stopping = false
	s.mu.Unlock()

	go s.wait(cmd)
	return nil
}

func (s *localAppServer) prepareEndpoint() error {
	s.mu.RLock()
	host := s.host
	port := s.port
	s.mu.RUnlock()

	availablePort, err := chooseAvailablePort(host, port)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.port = availablePort
	s.url = appServerURL(host, availablePort)
	s.mu.Unlock()
	return nil
}

func chooseAvailablePort(host string, start int) (int, error) {
	if start <= 0 {
		start = defaultAppServerPort
	}
	var lastErr error
	for port := start; port < start+50; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
		if err == nil {
			_ = listener.Close()
			return port, nil
		}
		lastErr = err
	}
	return 0, fmt.Errorf("cannot find available app-server port near %d: %w", start, lastErr)
}

func appServerURL(host string, port int) string {
	return fmt.Sprintf("ws://%s:%d", host, port)
}

func (s *localAppServer) ensureTokenFile() error {
	if s.tokenFile == "" {
		return errors.New("app-server token file is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.tokenFile), 0o700); err != nil {
		return err
	}
	if contents, err := os.ReadFile(s.tokenFile); err == nil && strings.TrimSpace(string(contents)) != "" {
		return nil
	}

	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return err
	}
	encoded := base64.RawURLEncoding.EncodeToString(token)
	return os.WriteFile(s.tokenFile, []byte(encoded), 0o600)
}

func (s *localAppServer) command() *exec.Cmd {
	s.mu.RLock()
	cliPath := s.cliPath
	url := s.url
	tokenFile := s.tokenFile
	s.mu.RUnlock()

	cmd := exec.Command(cliPath,
		"app-server",
		"--listen", url,
		"--ws-auth", "capability-token",
		"--ws-token-file", tokenFile,
	)
	if app != nil {
		cmd.Dir = app.AppPath
		if !app.DisableLog {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
	}
	configureManagedChildProcess(cmd)
	return cmd
}

func (s *localAppServer) wait(cmd *exec.Cmd) {
	err := cmd.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != cmd {
		return
	}
	s.cmd = nil
	s.pid = 0
	if s.stopping {
		s.status = appServerStatusStopped
		s.lastError = ""
		s.stopping = false
		return
	}
	if err != nil {
		s.status = appServerStatusFailed
		s.lastError = err.Error()
		return
	}
	s.status = appServerStatusStopped
	s.lastError = ""
}

func (s *localAppServer) stop() {
	if s == nil {
		return
	}

	s.mu.Lock()
	cmd := s.cmd
	if cmd == nil || cmd.Process == nil {
		if s.enabled && s.status != appServerStatusUnavailable && s.status != appServerStatusFailed {
			s.status = appServerStatusStopped
		}
		s.mu.Unlock()
		return
	}
	s.stopping = true
	s.status = appServerStatusStopping
	s.mu.Unlock()

	terminateManagedChildProcess(cmd)
}

func (s *localAppServer) markFailed(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = appServerStatusFailed
	if err != nil {
		s.lastError = err.Error()
	}
}

func (s *localAppServer) snapshot() appServerStatus {
	if s == nil {
		return appServerStatus{Enabled: false, Status: appServerStatusDisabled}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return appServerStatus{
		Enabled:   s.enabled,
		Status:    s.status,
		URL:       s.url,
		TokenFile: s.tokenFile,
		PID:       s.pid,
		Error:     s.lastError,
	}
}

func startRuntimeAppServer(assign func(int) error) {
	if runtimeAppServer == nil {
		return
	}
	err := runtimeAppServer.start(assign)
	status := runtimeAppServer.snapshot()
	if err != nil {
		log.Warn().Err(err).Msg("Cannot start local app-server")
		printLocalAppServerFailed(status)
		return
	}
	if status.Status == appServerStatusRunning {
		printLocalAppServerReady(status)
	}
}

func stopRuntimeAppServer() {
	if runtimeAppServer != nil {
		runtimeAppServer.stop()
	}
}

func printLocalAppServerReady(status appServerStatus) {
	lines := []string{
		"本地 Codex app-server 已启动：",
		"  " + status.URL,
		"token 文件：",
		"  " + status.TokenFile,
		"当前仅监听本机；手机代理和公网隧道暂未启用。",
		"",
	}
	for _, line := range lines {
		printConsoleLine(line)
	}
}

func printLocalAppServerFailed(status appServerStatus) {
	reason := status.Error
	if reason == "" {
		reason = status.Status
	}
	lines := []string{
		"本地 Codex app-server 未能启动，不影响 Codex Desktop 使用。",
		"原因：" + reason,
		"",
	}
	for _, line := range lines {
		printConsoleLine(line)
	}
}
