//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/portapps/portapps/v3"
	"github.com/portapps/portapps/v3/pkg/log"
	"github.com/portapps/portapps/v3/pkg/utl"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	shell32                = syscall.NewLazyDLL("shell32.dll")
	procSetConsoleCP       = kernel32.NewProc("SetConsoleCP")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
	procGetConsoleMode     = kernel32.NewProc("GetConsoleMode")
	procWriteConsoleW      = kernel32.NewProc("WriteConsoleW")
	procCreateJobObjectW   = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJob  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob = kernel32.NewProc("AssignProcessToJobObject")
	procOpenProcess        = kernel32.NewProc("OpenProcess")
	procCloseHandle        = kernel32.NewProc("CloseHandle")
	procIsUserAnAdmin      = shell32.NewProc("IsUserAnAdmin")
)

const (
	jobObjectExtendedLimitInformationClass = 9
	jobObjectLimitKillOnJobClose           = 0x00002000
	processTerminate                       = 0x0001
	processSetQuota                        = 0x0100
)

func newRuntimeApp(appID string, appName string, cfg *codexConfig) (*runtimeApp, error) {
	portable, err := portapps.NewWithCfg(appID, appName, cfg)
	if err != nil {
		return nil, err
	}
	return &runtimeApp{
		ID:         portable.ID,
		RootPath:   portable.RootPath,
		DataPath:   portable.DataPath,
		AppPath:    portable.AppPath,
		Args:       portable.Args,
		CommonArgs: append([]string{}, portable.Config().Common.Args...),
		DisableLog: portable.Config().Common.DisableLog,
		close:      portable.Close,
	}, nil
}

func configureConsoleOutput() {
	_, _, _ = procSetConsoleCP.Call(65001)
	_, _, _ = procSetConsoleOutputCP.Call(65001)
}

func relaunchUnelevatedIfNeeded() bool {
	if !isRunningAsAdmin() {
		return false
	}
	if hasUnelevatedRetryArg() {
		printUnelevatedRetryFailedNotice()
		waitForEnterToExit()
		return true
	}

	printElevatedNotice()
	if err := relaunchThroughExplorer(); err != nil {
		printConsoleLine("")
		printConsoleLine("自动切换到普通权限失败：" + err.Error())
		printConsoleLine("请关闭此窗口后，直接双击 codex-dataproxy.exe 启动。")
		printConsoleLine("按 Enter 键退出...")
		waitForEnterToExit()
		return true
	}

	printConsoleLine("已尝试以普通用户权限重新打开 Codex DataProxy。")
	printConsoleLine("如果新窗口没有出现，请关闭此窗口后直接双击 codex-dataproxy.exe。")
	printConsoleLine("当前管理员窗口将在 5 秒后关闭...")
	time.Sleep(5 * time.Second)
	return true
}

func isRunningAsAdmin() bool {
	ok, _, _ := procIsUserAnAdmin.Call()
	return ok != 0
}

func relaunchThroughExplorer() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	args := append([]string{exePath, unelevatedRetryArg}, userLaunchArgs()...)
	cmd := exec.Command("explorer.exe", args...)
	cmd.Dir = filepath.Dir(exePath)
	return cmd.Start()
}

func platformCodexProcessPath(appPath string, executable string) string {
	return filepath.Join(appPath, executable)
}

func configurePortableEnvironment(portableCodexHome string, portableAppData string, portableLocalAppData string) {
	_ = os.Setenv("CODEX_HOME", portableCodexHome)
	_ = os.Setenv("APPDATA", portableAppData)
	_ = os.Setenv("LOCALAPPDATA", portableLocalAppData)
}

func forcePortableCodexHome(portableCodexHome string) (func(), error) {
	userProfile, ok := os.LookupEnv("USERPROFILE")
	if !ok || userProfile == "" {
		return nil, errors.New("USERPROFILE is not set")
	}

	hostCodexHome := filepath.Join(userProfile, ".codex")
	backupCodexHome := filepath.Join(userProfile, ".codex.portable-backup")

	if samePath(hostCodexHome, portableCodexHome) {
		return func() {}, nil
	}

	if utl.Exists(backupCodexHome) {
		return nil, fmt.Errorf("backup path already exists: %s", backupCodexHome)
	}

	hostExisted := utl.Exists(hostCodexHome)
	if hostExisted {
		if err := os.Rename(hostCodexHome, backupCodexHome); err != nil {
			return nil, err
		}
	}

	if err := os.Symlink(portableCodexHome, hostCodexHome); err != nil {
		if hostExisted {
			_ = os.Rename(backupCodexHome, hostCodexHome)
		}
		return nil, err
	}

	return func() {
		_ = os.Remove(hostCodexHome)
		if hostExisted {
			_ = os.Rename(backupCodexHome, hostCodexHome)
		}
	}, nil
}

func samePath(left string, right string) bool {
	l, errLeft := filepath.Abs(left)
	r, errRight := filepath.Abs(right)
	if errLeft != nil || errRight != nil {
		return false
	}
	return filepath.Clean(l) == filepath.Clean(r)
}

func openSettingsPage(settingsURL string) {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", settingsURL)
	if err := cmd.Start(); err != nil {
		log.Warn().Err(err).Msg("Cannot open settings page")
		return
	}
	go func() {
		_ = cmd.Wait()
	}()
}

func writeConsoleLine(line string) bool {
	var mode uint32
	stdout := os.Stdout.Fd()
	if ok, _, _ := procGetConsoleMode.Call(stdout, uintptr(unsafe.Pointer(&mode))); ok == 0 {
		return false
	}

	text, err := syscall.UTF16FromString(line + "\r\n")
	if err != nil || len(text) == 0 {
		return false
	}

	var written uint32
	ok, _, _ := procWriteConsoleW.Call(
		stdout,
		uintptr(unsafe.Pointer(&text[0])),
		uintptr(len(text)-1),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	return ok != 0
}

func writePlatformCodexConfig(b *strings.Builder) {
	fmt.Fprintf(b, "[windows]\n")
	fmt.Fprintf(b, "sandbox = %s\n", tomlString(windowsSandboxMode))
}

func normalizePlatformSandboxFromFile(path string) error {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	cleaned := normalizeWindowsSandbox(string(contents))
	if cleaned == string(contents) {
		return nil
	}
	return os.WriteFile(path, []byte(cleaned), 0o644)
}

func normalizeWindowsSandbox(contents string) string {
	blocks := splitTomlBlocks(contents)
	seenWindows := false
	var b strings.Builder
	for _, block := range blocks {
		if block.hasHeader && block.name == "windows" {
			seenWindows = true
			block.body = normalizeWindowsSandboxLines(block.body)
			if !hasTomlContent(block.body) {
				continue
			}
		}
		if block.hasHeader {
			b.WriteString(block.header)
		}
		for _, line := range block.body {
			b.WriteString(line)
		}
	}
	if !seenWindows {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		writePlatformCodexConfig(&b)
	}
	return b.String()
}

type tomlBlock struct {
	name      string
	header    string
	body      []string
	hasHeader bool
}

func splitTomlBlocks(contents string) []tomlBlock {
	lines := strings.SplitAfter(contents, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	blocks := []tomlBlock{{}}
	for _, line := range lines {
		if name, ok := tomlSectionName(line); ok {
			blocks = append(blocks, tomlBlock{name: name, header: line, hasHeader: true})
			continue
		}
		blocks[len(blocks)-1].body = append(blocks[len(blocks)-1].body, line)
	}
	return blocks
}

func tomlSectionName(line string) (string, bool) {
	trimmed := strings.TrimSpace(stripSimpleTomlComment(strings.TrimRight(line, "\r\n")))
	if strings.HasPrefix(trimmed, "[[") || !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return "", false
	}
	name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
	return name, name != ""
}

func normalizeWindowsSandboxLines(lines []string) []string {
	cleaned := make([]string, 0, len(lines))
	seenSandbox := false
	for _, line := range lines {
		if isElevatedSandboxLine(line) {
			cleaned = append(cleaned, replaceLineContent(line, "sandbox = "+tomlString(windowsSandboxMode)))
			seenSandbox = true
			continue
		}
		if isWindowsSandboxLine(line) {
			seenSandbox = true
		}
		cleaned = append(cleaned, line)
	}
	if !seenSandbox {
		cleaned = append([]string{"sandbox = " + tomlString(windowsSandboxMode) + "\n"}, cleaned...)
	}
	return cleaned
}

func isElevatedSandboxLine(line string) bool {
	value, ok := windowsSandboxLineValue(line)
	return ok && value == elevatedSandboxMode
}

func isWindowsSandboxLine(line string) bool {
	_, ok := windowsSandboxLineValue(line)
	return ok
}

func windowsSandboxLineValue(line string) (string, bool) {
	body := strings.TrimSpace(stripSimpleTomlComment(strings.TrimRight(line, "\r\n")))
	key, value, ok := strings.Cut(body, "=")
	if !ok {
		return "", false
	}
	if strings.TrimSpace(key) != "sandbox" {
		return "", false
	}
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value, value != ""
}

func replaceLineContent(line string, content string) string {
	switch {
	case strings.HasSuffix(line, "\r\n"):
		return content + "\r\n"
	case strings.HasSuffix(line, "\n"):
		return content + "\n"
	default:
		return content
	}
}

func hasTomlContent(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(stripSimpleTomlComment(strings.TrimRight(line, "\r\n")))
		if trimmed != "" {
			return true
		}
	}
	return false
}

func stripSimpleTomlComment(line string) string {
	if before, _, ok := strings.Cut(line, "#"); ok {
		return before
	}
	return line
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type processJob struct {
	handle syscall.Handle
}

func newKillOnCloseJob() (*processJob, error) {
	handle, _, err := procCreateJobObjectW.Call(0, 0)
	if handle == 0 {
		return nil, windowsCallError(err, "CreateJobObjectW failed")
	}

	info := jobObjectExtendedLimitInformation{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose

	ok, _, err := procSetInformationJob.Call(
		handle,
		jobObjectExtendedLimitInformationClass,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	if ok == 0 {
		closeHandle(syscall.Handle(handle))
		return nil, windowsCallError(err, "SetInformationJobObject failed")
	}

	return &processJob{handle: syscall.Handle(handle)}, nil
}

func (job *processJob) assign(pid int) error {
	if job == nil || job.handle == 0 {
		return nil
	}

	access := uintptr(processTerminate | processSetQuota)
	processHandle, _, err := procOpenProcess.Call(access, 0, uintptr(uint32(pid)))
	if processHandle == 0 {
		return windowsCallError(err, "OpenProcess failed")
	}
	defer closeHandle(syscall.Handle(processHandle))

	ok, _, err := procAssignProcessToJob.Call(uintptr(job.handle), processHandle)
	if ok == 0 {
		return windowsCallError(err, "AssignProcessToJobObject failed")
	}
	return nil
}

func (job *processJob) close() {
	if job == nil || job.handle == 0 {
		return
	}
	closeHandle(job.handle)
	job.handle = 0
}

func closeHandle(handle syscall.Handle) {
	if handle != 0 {
		_, _, _ = procCloseHandle.Call(uintptr(handle))
	}
}

func windowsCallError(err error, fallback string) error {
	if errno, ok := err.(syscall.Errno); ok && errno == 0 {
		return errors.New(fallback)
	}
	return err
}

func launchAndWait(args []string) {
	if !utl.Exists(app.Process) {
		log.Fatal().Msgf("Application not found: %s", app.Process)
	}

	job, err := newKillOnCloseJob()
	if err != nil {
		log.Warn().Err(err).Msg("Cannot create process cleanup job")
	}
	defer job.close()

	launchArgs := []string{defaultLanguageArg}
	launchArgs = append(launchArgs, app.CommonArgs...)
	launchArgs = append(launchArgs, args...)
	launchArgs = append(launchArgs, app.Args...)

	cmd := exec.Command(app.Process, launchArgs...)
	cmd.Dir = app.WorkingDir
	if !app.DisableLog {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		log.Fatal().Err(err).Msg("Cannot start Codex Desktop")
	}

	if err := job.assign(cmd.Process.Pid); err != nil {
		log.Warn().Err(err).Msg("Cannot attach Codex process to cleanup job")
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal().Err(err).Msg("Codex Desktop exited with an error")
	}
}
