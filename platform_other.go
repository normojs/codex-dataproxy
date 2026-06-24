//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/portapps/portapps/v3/pkg/log"
)

func newRuntimeApp(appID string, appName string, cfg *codexConfig) (*runtimeApp, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	rootPath := filepath.Dir(executable)
	return &runtimeApp{
		ID:       appID,
		RootPath: rootPath,
		DataPath: filepath.Join(rootPath, "data"),
		AppPath:  filepath.Join(rootPath, "app"),
		Args:     append([]string{}, os.Args[1:]...),
	}, nil
}

func configureConsoleOutput() {
}

func relaunchUnelevatedIfNeeded() bool {
	return false
}

func platformCodexProcessPath(appPath string, executable string) string {
	if executable != "" {
		if filepath.IsAbs(executable) {
			return executable
		}
		candidate := filepath.Join(appPath, executable)
		if fileExists(candidate) {
			return candidate
		}
	}
	if runtime.GOOS == "darwin" {
		candidate := filepath.Join(appPath, "Codex.app", "Contents", "MacOS", "Codex")
		if fileExists(candidate) {
			return candidate
		}
	}
	return filepath.Join(appPath, executable)
}

func configurePortableEnvironment(portableCodexHome string, portableAppData string, portableLocalAppData string) {
	_ = os.Setenv("CODEX_HOME", portableCodexHome)
}

func forcePortableCodexHome(portableCodexHome string) (func(), error) {
	return func() {}, nil
}

func openSettingsPage(settingsURL string) {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	cmd := exec.Command(command, settingsURL)
	if err := cmd.Start(); err != nil {
		log.Warn().Err(err).Msg("Cannot open settings page")
		return
	}
	go func() {
		_ = cmd.Wait()
	}()
}

func writeConsoleLine(line string) bool {
	return false
}

func writePlatformCodexConfig(b *strings.Builder) {
}

func normalizePlatformSandboxFromFile(path string) error {
	return nil
}

func launchAndWait(args []string) {
	if !fileExists(app.Process) {
		log.Fatal().Msgf("Application not found: %s", app.Process)
	}

	launchArgs := []string{defaultLanguageArg}
	launchArgs = append(launchArgs, app.CommonArgs...)
	launchArgs = append(launchArgs, args...)
	launchArgs = append(launchArgs, app.Args...)

	cmd := exec.Command(app.Process, launchArgs...)
	cmd.Dir = app.WorkingDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if !app.DisableLog {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		log.Fatal().Err(err).Msg("Cannot start Codex Desktop")
	}
	defer func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}()

	if err := cmd.Wait(); err != nil {
		log.Fatal().Err(err).Msg("Codex Desktop exited with an error")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
