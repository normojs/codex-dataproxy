package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalAppServerBuildsLoopbackCommand(t *testing.T) {
	withDefaultTestConfig(t)
	oldApp := app
	t.Cleanup(func() { app = oldApp })

	root := t.TempDir()
	appPath := filepath.Join(root, "app")
	resourcesPath := filepath.Join(appPath, "resources")
	if err := os.MkdirAll(resourcesPath, 0o755); err != nil {
		t.Fatalf("cannot create resources path: %v", err)
	}
	cliName := "codex"
	if runtime.GOOS == "windows" {
		cliName = "codex.exe"
	}
	cliPath := filepath.Join(resourcesPath, cliName)
	if err := os.WriteFile(cliPath, []byte("fake"), 0o755); err != nil {
		t.Fatalf("cannot create fake codex CLI: %v", err)
	}

	app = &runtimeApp{AppPath: appPath}
	cfg.AppServer.Enabled = true
	cfg.AppServer.Host = "0.0.0.0"
	cfg.AppServer.Port = 0

	server := newLocalAppServer(filepath.Join(root, "data", ".codex"))
	if server.cliPath != cliPath {
		t.Fatalf("cliPath = %q, want %q", server.cliPath, cliPath)
	}
	if err := server.prepareEndpoint(); err != nil {
		t.Fatalf("prepareEndpoint returned error: %v", err)
	}
	if err := server.ensureTokenFile(); err != nil {
		t.Fatalf("ensureTokenFile returned error: %v", err)
	}

	cmd := server.command()
	args := strings.Join(cmd.Args, "\n")
	for _, want := range []string{
		"app-server",
		"--listen",
		"ws://127.0.0.1:",
		"--ws-auth",
		"capability-token",
		"--ws-token-file",
		server.tokenFile,
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("command args missing %q in %#v", want, cmd.Args)
		}
	}
}

func TestLocalAppServerSnapshotDoesNotExposeToken(t *testing.T) {
	withDefaultTestConfig(t)
	oldApp := app
	t.Cleanup(func() { app = oldApp })

	root := t.TempDir()
	app = &runtimeApp{AppPath: filepath.Join(root, "app")}
	server := newLocalAppServer(filepath.Join(root, "data", ".codex"))
	if err := server.ensureTokenFile(); err != nil {
		t.Fatalf("ensureTokenFile returned error: %v", err)
	}
	tokenBytes, err := os.ReadFile(server.tokenFile)
	if err != nil {
		t.Fatalf("cannot read token file: %v", err)
	}

	status := server.snapshot()
	if status.TokenFile != server.tokenFile {
		t.Fatalf("TokenFile = %q, want %q", status.TokenFile, server.tokenFile)
	}
	if strings.Contains(status.URL, string(tokenBytes)) || strings.Contains(status.Error, string(tokenBytes)) {
		t.Fatalf("snapshot should not expose token contents: %#v", status)
	}
}
