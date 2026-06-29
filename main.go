package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/portapps/portapps/v3/pkg/log"
	"go.yaml.in/yaml/v3"
)

type userConfig struct {
	BaseURL *string      `yaml:"base_url"`
	APIKey  *string      `yaml:"api_key"`
	Model   *string      `yaml:"model"`
	Models  *modelIDList `yaml:"models"`
}

type modelIDList []string

type codexConfig struct {
	Executable            string                       `yaml:"executable" mapstructure:"executable"`
	ForceUserprofileHome  bool                         `yaml:"force_userprofile_codex_home" mapstructure:"force_userprofile_codex_home"`
	SelectedModel         string                       `yaml:"selected_model" mapstructure:"selected_model"`
	SelectedProvider      string                       `yaml:"selected_provider" mapstructure:"selected_provider"`
	AppServer             appServerConfig              `yaml:"app_server" mapstructure:"app_server"`
	AdditionalEnvironment map[string]string            `yaml:"additional_environment" mapstructure:"additional_environment"`
	Providers             map[string]providerConfig    `yaml:"providers" mapstructure:"providers"`
	ModelPresets          map[string]modelPresetConfig `yaml:"model_presets" mapstructure:"model_presets"`
	Models                []modelEntryConfig           `yaml:"models" mapstructure:"models"`
}

type appServerConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Host    string `yaml:"host" mapstructure:"host"`
	Port    int    `yaml:"port" mapstructure:"port"`
}

type providerConfig struct {
	Name                  string            `yaml:"name" mapstructure:"name"`
	BaseURL               string            `yaml:"base_url" mapstructure:"base_url"`
	APIKey                string            `yaml:"api_key" mapstructure:"api_key"`
	AuthMode              string            `yaml:"auth_mode" mapstructure:"auth_mode"`
	EnvKey                string            `yaml:"env_key" mapstructure:"env_key"`
	WireAPI               string            `yaml:"wire_api" mapstructure:"wire_api"`
	FetchModels           bool              `yaml:"fetch_models" mapstructure:"fetch_models"`
	FetchedModelPreset    string            `yaml:"fetched_model_preset" mapstructure:"fetched_model_preset"`
	AdditionalEnvironment map[string]string `yaml:"additional_environment" mapstructure:"additional_environment"`
	Codex                 codexNativeConfig `yaml:"codex" mapstructure:"codex"`
	Catalog               catalogConfig     `yaml:"catalog" mapstructure:"catalog"`
}

type modelPresetConfig struct {
	Codex     codexNativeConfig `yaml:"codex" mapstructure:"codex"`
	Catalog   catalogConfig     `yaml:"catalog" mapstructure:"catalog"`
	DataProxy map[string]any    `yaml:"dataproxy" mapstructure:"dataproxy"`
}

type modelEntryConfig struct {
	ID          string            `yaml:"id" mapstructure:"id"`
	Provider    string            `yaml:"provider" mapstructure:"provider"`
	Preset      string            `yaml:"preset" mapstructure:"preset"`
	DisplayName string            `yaml:"display_name" mapstructure:"display_name"`
	Description string            `yaml:"description" mapstructure:"description"`
	Enabled     *bool             `yaml:"enabled" mapstructure:"enabled"`
	Codex       codexNativeConfig `yaml:"codex" mapstructure:"codex"`
	Catalog     catalogConfig     `yaml:"catalog" mapstructure:"catalog"`
	DataProxy   map[string]any    `yaml:"dataproxy" mapstructure:"dataproxy"`
}

type codexNativeConfig struct {
	ModelReasoningEffort       string `yaml:"model_reasoning_effort" mapstructure:"model_reasoning_effort"`
	PlanModeReasoningEffort    string `yaml:"plan_mode_reasoning_effort" mapstructure:"plan_mode_reasoning_effort"`
	ModelReasoningSummary      string `yaml:"model_reasoning_summary" mapstructure:"model_reasoning_summary"`
	ModelVerbosity             string `yaml:"model_verbosity" mapstructure:"model_verbosity"`
	ModelContextWindow         int64  `yaml:"model_context_window" mapstructure:"model_context_window"`
	ModelAutoCompactTokenLimit int64  `yaml:"model_auto_compact_token_limit" mapstructure:"model_auto_compact_token_limit"`
}

type catalogConfig struct {
	DefaultReasoningEffort    string   `yaml:"default_reasoning_effort" mapstructure:"default_reasoning_effort"`
	SupportedReasoningEfforts []string `yaml:"supported_reasoning_efforts" mapstructure:"supported_reasoning_efforts"`
}

type effectiveModelConfig struct {
	ID          string
	Provider    string
	DisplayName string
	Description string
	Codex       codexNativeConfig
	Catalog     catalogConfig
	Configured  bool
}

var (
	app               *runtimeApp
	cfg               *codexConfig
	runtimeStore      *providerStore
	runtimeHTTPServer *localHTTPServer
	runtimeAppServer  *localAppServer
)

const (
	appID                = "codex-dataproxy"
	appName              = "Codex DataProxy"
	configFileName       = appID + ".yml"
	apiKeyPlaceholder    = "sk-xx"
	defaultModel         = "gpt-5.5"
	defaultLanguageArg   = "--lang=zh-CN"
	unelevatedRetryArg   = "--codex-dataproxy-unelevated-retry"
	defaultWireAPI       = "responses"
	localProxyAPIKey     = "codex-dataproxy-local"
	authModeCodexAPIKey  = "codex_api_key"
	authModeProviderEnv  = "provider_env"
	authFileName         = "auth.json"
	modelsFileName       = "dataproxy-models.json"
	appServerTokenName   = "dataproxy-app-server.token"
	deviceIDFileName     = "dataproxy-device-id"
	defaultAppServerHost = "127.0.0.1"
	defaultAppServerPort = 17666
	emptyModelsLabel     = "获取模型为空"
	persistedAtomState   = "electron-persisted-atom-state"
	permissionVisibility = "composer-permission-mode-visibility"
	windowsSandboxMode   = "unelevated"
	elevatedSandboxMode  = "elevated"
)

func init() {
	var err error
	cfg = defaultCodexConfig()
	if isTestBinary() {
		return
	}

	app, err = newRuntimeApp(appID, appName, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot initialize Codex DataProxy")
	}
}

func defaultCodexConfig() *codexConfig {
	return &codexConfig{
		Executable:           "Codex.exe",
		ForceUserprofileHome: false,
		SelectedModel:        defaultModel,
		SelectedProvider:     "dataproxy",
		AppServer: appServerConfig{
			Enabled: true,
			Host:    defaultAppServerHost,
			Port:    defaultAppServerPort,
		},
		AdditionalEnvironment: map[string]string{},
		Providers: map[string]providerConfig{
			"dataproxy": {
				Name:               "DataProxy",
				BaseURL:            "https://dp.app.mbu.ltd/v1",
				APIKey:             apiKeyPlaceholder,
				AuthMode:           authModeCodexAPIKey,
				WireAPI:            defaultWireAPI,
				FetchModels:        true,
				FetchedModelPreset: "default_gpt",
			},
		},
		ModelPresets: map[string]modelPresetConfig{
			"default_gpt": {
				Codex: codexNativeConfig{
					ModelReasoningEffort:       "medium",
					PlanModeReasoningEffort:    "medium",
					ModelReasoningSummary:      "auto",
					ModelVerbosity:             "medium",
					ModelContextWindow:         128000,
					ModelAutoCompactTokenLimit: 96000,
				},
				Catalog: catalogConfig{
					DefaultReasoningEffort:    "medium",
					SupportedReasoningEfforts: []string{"minimal", "low", "medium", "high"},
				},
			},
			"gpt_reasoning_1m": {
				Codex: codexNativeConfig{
					ModelReasoningEffort:       "xhigh",
					PlanModeReasoningEffort:    "high",
					ModelReasoningSummary:      "auto",
					ModelVerbosity:             "medium",
					ModelContextWindow:         1048576,
					ModelAutoCompactTokenLimit: 900000,
				},
				Catalog: catalogConfig{
					DefaultReasoningEffort:    "xhigh",
					SupportedReasoningEfforts: []string{"minimal", "low", "medium", "high", "xhigh"},
				},
			},
			"chat_128k_think": {
				Codex: codexNativeConfig{
					ModelContextWindow:         128000,
					ModelAutoCompactTokenLimit: 96000,
				},
				Catalog: catalogConfig{
					DefaultReasoningEffort:    "medium",
					SupportedReasoningEfforts: []string{"low", "medium", "high"},
				},
				DataProxy: map[string]any{"think": true},
			},
		},
		Models: []modelEntryConfig{
			{
				ID:          defaultModel,
				Provider:    "dataproxy",
				Preset:      "gpt_reasoning_1m",
				DisplayName: "GPT-5.5",
				Enabled:     boolPtr(true),
			},
		},
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func (list *modelIDList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*list = splitModelIDs(value.Value)
	case yaml.SequenceNode:
		values := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			values = append(values, splitModelIDs(item.Value)...)
		}
		*list = values
	default:
		*list = nil
	}
	return nil
}

func splitModelIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func isTestBinary() bool {
	name := strings.ToLower(filepath.Base(os.Args[0]))
	return strings.HasSuffix(name, ".test") || strings.HasSuffix(name, ".test.exe")
}

func main() {
	configureConsoleOutput()
	removeGeneratedSampleConfigs()
	if relaunchUnelevatedIfNeeded() {
		return
	}
	printStartupBanner()
	defer app.Close()

	portableCodexHome := filepath.Join(app.DataPath, ".codex")
	portableAppData := filepath.Join(app.DataPath, "appdata")
	portableLocalAppData := filepath.Join(app.DataPath, "localappdata")

	mustCreateDir(portableCodexHome)
	mustCreateDir(portableAppData)
	mustCreateDir(portableLocalAppData)
	cleanupCodexRuntimeState(portableCodexHome)
	runtimeAppServer = newLocalAppServer(portableCodexHome)

	var err error
	runtimeStore, err = newProviderStore(filepath.Join(app.RootPath, "config"))
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load provider config")
	}
	seedCachedDynamicModels(runtimeStore, portableCodexHome)
	runtimeHTTPServer, err = startLocalHTTPServer(runtimeStore, portableCodexHome, runtimeAppServer)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot start local settings server")
	}
	defer runtimeHTTPServer.close()
	printSettingsServerReady(runtimeHTTPServer.settingsURL(), runtimeHTTPServer.proxyBaseURL())

	if reasons := runtimeStore.setupProblems(); len(reasons) > 0 {
		printProviderSetupRequired(reasons, runtimeHTTPServer.settingsURL())
		openSettingsPage(runtimeHTTPServer.settingsURL())
		waitForProviderSetup(runtimeHTTPServer.settingsURL())
	}

	syncCodexAuth(portableCodexHome)
	modelList := syncDynamicModelList(portableCodexHome)
	syncCodexConfig(portableCodexHome, modelList.DefaultModel)
	startBackgroundModelRefresh(portableCodexHome)

	app.Process = platformCodexProcessPath(app.AppPath, cfg.Executable)
	app.WorkingDir = app.AppPath

	configurePortableEnvironment(portableCodexHome, portableAppData, portableLocalAppData)

	for key, value := range cfg.AdditionalEnvironment {
		if value != "" {
			os.Setenv(key, value)
		}
	}

	if cfg.ForceUserprofileHome {
		cleanup, err := forcePortableCodexHome(portableCodexHome)
		if err != nil {
			log.Fatal().Err(err).Msg("Cannot activate force_userprofile_codex_home")
		}
		defer cleanup()
	}

	launchAndWait(userLaunchArgs())
	cleanupCodexRuntimeState(portableCodexHome)
}

func hasUnelevatedRetryArg() bool {
	for _, arg := range os.Args[1:] {
		if arg == unelevatedRetryArg {
			return true
		}
	}
	return false
}

func userLaunchArgs() []string {
	args := make([]string, 0, len(os.Args)-1)
	for _, arg := range os.Args[1:] {
		if arg != unelevatedRetryArg {
			args = append(args, arg)
		}
	}
	return args
}

func printElevatedNotice() {
	lines := []string{
		"============================================================",
		"检测到当前正在以 Windows 管理员身份运行。",
		"",
		"Codex 的“完全访问”是应用内权限选项，不等于 Windows 管理员身份。",
		"为了让 Codex 自带的三档权限选择正常工作，请使用普通用户权限启动。",
		"",
		"正在尝试自动切换为普通用户权限重新打开...",
		"============================================================",
	}
	for _, line := range lines {
		printConsoleLine(line)
	}
}

func printUnelevatedRetryFailedNotice() {
	lines := []string{
		"============================================================",
		"仍然检测到当前是 Windows 管理员身份。",
		"",
		"启动器已经尝试自动切换为普通用户权限，但本机环境没有完成降权。",
		"请关闭此窗口，不要右键选择“以管理员身份运行”，直接双击 codex-dp.exe 启动。",
		"",
		"按 Enter 键退出...",
		"============================================================",
	}
	for _, line := range lines {
		printConsoleLine(line)
	}
}

func printStartupBanner() {
	lines := []string{
		"============================================================",
		"Codex DataProxy 正在启动，请稍候...",
		"首次启动或电脑较慢时可能需要几十秒；请保持此窗口打开。",
		"",
		"默认内置 DataProxy AI 中转站：https://dp.app.mbu.ltd",
		"价格通常低于官方，支持 GPT，并会随中转站能力扩展更多模型。",
		"首次使用请先在网站注册账号、创建 API 密钥，并在本地 settings 页面填写。",
		"国产大模型是否可用取决于中转站当前接口和模型适配；",
		"可进群咨询或自行测试，等待中转站升级后即可支持更多国产模型。",
		"也可以改用其他 OpenAI 兼容中转站、模型和密钥。",
		"",
		"配置交流与问题反馈：QQ 交流群 891855578",
		"============================================================",
		"",
	}
	for _, line := range lines {
		printConsoleLine(line)
	}
}

func printSettingsServerReady(settingsURL string, proxyURL string) {
	lines := []string{
		"本地设置页面已启动：",
		"  " + settingsURL,
		"",
		"Codex Desktop 将通过本地代理访问中转站：",
		"  " + proxyURL,
		"",
	}
	for _, line := range lines {
		printConsoleLine(line)
	}
}

func printProviderSetupRequired(reasons []string, settingsURL string) {
	lines := []string{
		"中转站配置未完成，Codex Desktop 会先等待你完成设置。",
		"",
		"请先打开 settings 页面完成配置：",
		"  " + settingsURL,
		"",
		"需要处理：",
	}
	for _, reason := range reasons {
		lines = append(lines, "  - "+reason)
	}
	lines = append(lines,
		"",
		"保存配置后，本窗口会自动继续启动 Codex Desktop，不需要重新启动程序。",
		"正在等待 settings 页面保存有效配置...",
		"",
	)
	for _, line := range lines {
		printConsoleLine(line)
	}
}

func waitForProviderSetup(settingsURL string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	lastNotice := time.Now()
	for range ticker.C {
		if runtimeStore == nil {
			return
		}
		if reasons := runtimeStore.setupProblems(); len(reasons) == 0 {
			printConsoleLine("")
			printConsoleLine("配置已保存，正在启动 Codex Desktop...")
			printConsoleLine("")
			return
		}
		if time.Since(lastNotice) >= 30*time.Second {
			printConsoleLine("仍在等待有效配置保存。settings 页面：" + settingsURL)
			lastNotice = time.Now()
		}
	}
}

func loadUserConfig() error {
	configPath := filepath.Join(app.RootPath, configFileName)
	raw, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var user userConfig
	if err := yaml.Unmarshal(raw, &user); err != nil {
		return err
	}
	applyUserConfig(user)
	return nil
}

func applyUserConfig(user userConfig) {
	provider := cfg.Providers["dataproxy"]
	provider.Name = "DataProxy"
	provider.AuthMode = authModeCodexAPIKey
	provider.WireAPI = defaultWireAPI
	provider.FetchModels = true
	provider.FetchedModelPreset = "default_gpt"

	if user.BaseURL != nil {
		provider.BaseURL = strings.TrimSpace(*user.BaseURL)
	}
	if user.APIKey != nil {
		provider.APIKey = strings.TrimSpace(*user.APIKey)
	}
	if user.Model != nil {
		cfg.SelectedModel = strings.TrimSpace(*user.Model)
	}
	if user.Models != nil {
		cfg.Models = modelEntriesFromIDs([]string(*user.Models))
	}

	cfg.SelectedProvider = "dataproxy"
	cfg.Providers = map[string]providerConfig{"dataproxy": provider}
}

func modelEntriesFromIDs(ids []string) []modelEntryConfig {
	seen := map[string]bool{}
	models := make([]modelEntryConfig, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, modelEntryConfig{
			ID:          id,
			Provider:    "dataproxy",
			Preset:      presetForModelID(id),
			DisplayName: displayNameForModelID(id),
			Enabled:     boolPtr(true),
		})
	}
	return models
}

func presetForModelID(id string) string {
	if id == defaultModel {
		return "gpt_reasoning_1m"
	}
	return "default_gpt"
}

func displayNameForModelID(id string) string {
	if id == defaultModel {
		return "GPT-5.5"
	}
	return ""
}

func validateRequiredConfig() []string {
	var reasons []string

	cfg.SelectedModel = strings.TrimSpace(cfg.SelectedModel)
	cfg.SelectedProvider = strings.TrimSpace(cfg.SelectedProvider)

	if cfg.SelectedModel == "" {
		reasons = append(reasons, "model 还没有配置")
	}

	if cfg.SelectedProvider == "" {
		reasons = append(reasons, "内部 provider 没有配置")
		return reasons
	}

	_, provider, ok := activeProvider()
	if !ok {
		reasons = append(reasons, "内部 provider 没有配置")
		return reasons
	}

	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.APIKey = strings.TrimSpace(provider.APIKey)
	provider.AuthMode = strings.TrimSpace(provider.AuthMode)

	if provider.BaseURL == "" {
		reasons = append(reasons, "base_url 还没有配置")
	} else if err := validateAPIBaseURL(provider.BaseURL); err != nil {
		reasons = append(reasons, err.Error())
	}

	if provider.APIKey == "" {
		reasons = append(reasons, "api_key 还没有配置")
	} else if strings.EqualFold(provider.APIKey, apiKeyPlaceholder) {
		reasons = append(reasons, "api_key 仍然是默认占位值 sk-xx")
	}

	if !isSupportedAuthMode(provider.AuthMode) {
		reasons = append(reasons, fmt.Sprintf("内部 auth_mode 只支持 %s 或 %s", authModeCodexAPIKey, authModeProviderEnv))
	}

	return reasons
}

func activeProviderID() string {
	return strings.TrimSpace(fallback(cfg.SelectedProvider, "dataproxy"))
}

func activeProvider() (string, providerConfig, bool) {
	id := activeProviderID()
	if id == "" || cfg.Providers == nil {
		return id, providerConfig{}, false
	}
	provider, ok := cfg.Providers[id]
	if !ok {
		return id, providerConfig{}, false
	}
	provider.Name = strings.TrimSpace(provider.Name)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.APIKey = strings.TrimSpace(provider.APIKey)
	provider.AuthMode = strings.TrimSpace(provider.AuthMode)
	provider.EnvKey = strings.TrimSpace(provider.EnvKey)
	provider.WireAPI = strings.TrimSpace(provider.WireAPI)
	provider.FetchedModelPreset = strings.TrimSpace(provider.FetchedModelPreset)
	return id, provider, true
}

func authModeForProvider(provider providerConfig) string {
	mode := strings.ToLower(strings.TrimSpace(provider.AuthMode))
	if mode == "" {
		return authModeCodexAPIKey
	}
	return mode
}

func isSupportedAuthMode(mode string) bool {
	switch authModeForProvider(providerConfig{AuthMode: mode}) {
	case authModeCodexAPIKey, authModeProviderEnv:
		return true
	default:
		return false
	}
}

func envKeyForProvider(provider providerConfig) string {
	return fallback(strings.TrimSpace(provider.EnvKey), "DATAPROXY_API_KEY")
}

func printConfigLoadError(err error) {
	lines := []string{
		"配置文件读取失败，Codex Desktop 暂不启动。",
		"",
		"请检查当前目录下的 codex-dataproxy.yml 格式是否正确。",
		"推荐格式：",
		`  base_url: "https://dp.app.mbu.ltd/v1"`,
		`  api_key: "sk-xx"`,
		`  model: "gpt-5.5"`,
		`  models: "gpt-5.5"`,
		"",
		"错误信息：" + err.Error(),
		"",
		"按 Enter 键退出...",
	}
	for _, line := range lines {
		printConsoleLine(line)
	}
}

func printConfigurationRequired(reasons []string) {
	lines := []string{
		"配置未完成，Codex Desktop 暂不启动。",
		"",
		"请先完成以下配置：",
	}

	for _, reason := range reasons {
		lines = append(lines, "  - "+reason)
	}

	lines = append(lines,
		"",
		"请打开当前目录下的 codex-dataproxy.yml，确认以下配置：",
		"  - base_url",
		"  - api_key",
		"  - model",
		"",
		"默认内置 DataProxy 地址：https://dp.app.mbu.ltd/v1",
		"首次使用请先在 https://dp.app.mbu.ltd 注册账号、创建 API 密钥，",
		"然后把 api_key 从 sk-xx 替换为你的真实密钥。",
		"默认模型为 gpt-5.5。",
		"国产大模型是否可用取决于中转站当前接口和模型适配；",
		"可进群咨询或自行测试，等待中转站升级后即可支持更多国产模型。",
		"",
		"也支持改用其他 OpenAI 兼容中转站、模型和密钥。",
		"配置交流与问题反馈：QQ 交流群 891855578",
		"",
		"按 Enter 键退出...",
	)

	for _, line := range lines {
		printConsoleLine(line)
	}
}

func waitForEnterToExit() {
	_, _ = fmt.Fscanln(os.Stdin)
}

func printConsoleLine(line string) {
	if writeConsoleLine(line) {
		return
	}
	fmt.Println(line)
}

func removeGeneratedSampleConfigs() {
	ids := []string{app.ID}
	if app.ID != "codex-portable" {
		ids = append(ids, "codex-portable")
	}

	for _, id := range ids {
		samplePath := filepath.Join(app.RootPath, fmt.Sprintf("%s.sample.yml", id))
		if err := os.Remove(samplePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warn().Err(err).Msgf("Cannot remove generated sample config: %s", samplePath)
		}
	}
}

func mustCreateDir(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		log.Fatal().Err(err).Msgf("Cannot create %s", path)
	}
}

func syncCodexAuth(portableCodexHome string) {
	authTarget := filepath.Join(portableCodexHome, authFileName)
	payload := map[string]string{
		"OPENAI_API_KEY": localProxyAPIKey,
	}
	contents, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot encode portable auth.json")
	}
	contents = append(contents, '\n')

	if err := os.WriteFile(authTarget, contents, 0o600); err != nil {
		log.Fatal().Err(err).Msg("Cannot write portable auth.json")
	}
}

func syncCodexConfig(portableCodexHome string, selectedModel string) {
	if _, provider, ok := activeProvider(); !ok || provider.BaseURL == "" {
		return
	}

	configTarget := filepath.Join(portableCodexHome, "config.toml")
	configContent := renderCodexConfig(selectedModel)
	if existing, err := os.ReadFile(configTarget); err == nil {
		configContent = mergeCodexConfig(configContent, string(existing))
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatal().Err(err).Msg("Cannot read portable config.toml")
	}
	if err := os.WriteFile(configTarget, []byte(configContent), 0o644); err != nil {
		log.Fatal().Err(err).Msg("Cannot write portable config.toml")
	}

	removeGeneratedRequirements(portableCodexHome)
}

func removeGeneratedRequirements(portableCodexHome string) {
	requirementsTarget := filepath.Join(portableCodexHome, "requirements.toml")
	if err := os.Remove(requirementsTarget); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warn().Err(err).Msg("Cannot remove stale requirements.toml")
	}
}

func cleanupCodexRuntimeState(portableCodexHome string) {
	removeGeneratedRequirements(portableCodexHome)

	configTarget := filepath.Join(portableCodexHome, "config.toml")
	if err := normalizePlatformSandboxFromFile(configTarget); err != nil {
		log.Warn().Err(err).Msg("Cannot normalize platform sandbox setting")
	}

	for _, name := range []string{".codex-global-state.json", ".codex-global-state.json.bak"} {
		if err := restorePermissionModeVisibility(filepath.Join(portableCodexHome, name)); err != nil {
			log.Warn().Err(err).Msgf("Cannot restore permission visibility in %s", name)
		}
	}
}

func restorePermissionModeVisibility(path string) error {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var state map[string]any
	if err := json.Unmarshal(contents, &state); err != nil {
		return err
	}
	persisted, ok := state[persistedAtomState].(map[string]any)
	if !ok {
		persisted = map[string]any{}
		state[persistedAtomState] = persisted
	}
	persisted[permissionVisibility] = map[string]bool{
		"guardian-approvals": true,
		"full-access":        true,
	}
	normalizePersistedAgentMode(persisted, "agent-mode-by-host-id")
	normalizePersistedAgentMode(persisted, "preferred-non-full-access-agent-mode-by-host-id")
	delete(state, permissionVisibility)

	updated, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	updated = append(updated, '\n')
	return os.WriteFile(path, updated, 0o644)
}

func normalizePersistedAgentMode(persisted map[string]any, key string) {
	modes, ok := persisted[key].(map[string]any)
	if !ok {
		modes = map[string]any{}
		persisted[key] = modes
	}
	if mode, _ := modes["local"].(string); mode == "" || mode == "read-only" {
		modes["local"] = "auto"
	}
}

type openAIModelsResponse struct {
	Data []json.RawMessage `json:"data"`
}

type codexModelsResponse struct {
	Data []codexModel `json:"data"`
}

type codexModel struct {
	ID                        string            `json:"id"`
	Model                     string            `json:"model"`
	DisplayName               string            `json:"displayName"`
	Description               string            `json:"description"`
	DefaultReasoningEffort    string            `json:"defaultReasoningEffort"`
	SupportedReasoningEfforts []reasoningEffort `json:"supportedReasoningEfforts"`
	Hidden                    bool              `json:"hidden"`
	IsDefault                 bool              `json:"isDefault"`
}

type reasoningEffort struct {
	ReasoningEffort string `json:"reasoningEffort"`
	Description     string `json:"description"`
}

type modelListResult struct {
	DefaultModel string
	Models       []string
}

func seedCachedDynamicModels(store *providerStore, portableCodexHome string) {
	if store == nil {
		return
	}
	models, defaultModel, err := readCachedDynamicModels(portableCodexHome)
	if err != nil {
		log.Warn().Err(err).Msg("Cannot read cached dynamic model list")
		return
	}
	if store.seedCachedModels(models, defaultModel) {
		printConsoleLine(fmt.Sprintf("Using cached model list: %d models.", len(models)))
	}
}

func readCachedDynamicModels(portableCodexHome string) ([]string, string, error) {
	target := filepath.Join(portableCodexHome, modelsFileName)
	raw, err := os.ReadFile(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}

	var payload codexModelsResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, "", err
	}

	seen := map[string]bool{}
	models := []string{}
	defaultModel := ""
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" || id == emptyModelsLabel || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, id)
		if item.IsDefault && defaultModel == "" {
			defaultModel = id
		}
	}
	return models, defaultModel, nil
}

func syncDynamicModelList(portableCodexHome string) modelListResult {
	modelIDs := []string{}
	defaultModelID := ""
	if runtimeStore != nil {
		modelIDs = runtimeStore.modelIDs()
		defaultModelID = runtimeStore.defaultModel()
	}
	if len(modelIDs) == 0 {
		modelIDs = configuredStartupModelIDs()
		if defaultModelID == "" {
			defaultModelID = strings.TrimSpace(cfg.SelectedModel)
		}
	}
	if defaultModelID != "" {
		cfg.SelectedModel = defaultModelID
	}
	effectiveModels := make([]effectiveModelConfig, 0, len(modelIDs)+1)
	for _, id := range modelIDs {
		effectiveModels = append(effectiveModels, effectiveModelForID(id))
	}
	if defaultModelID != "" && !effectiveModelsContain(effectiveModels, defaultModelID) {
		effectiveModels = append([]effectiveModelConfig{effectiveModelForID(defaultModelID)}, effectiveModels...)
	}
	models := effectiveModelIDs(effectiveModels)
	if len(models) == 0 {
		models = []string{emptyModelsLabel}
		effectiveModels = []effectiveModelConfig{{ID: emptyModelsLabel, DisplayName: emptyModelsLabel}}
	}

	defaultModel := defaultModelForList(effectiveModels)
	target := filepath.Join(portableCodexHome, modelsFileName)
	payload := codexModelsResponse{Data: buildCodexModels(effectiveModels, defaultModel)}
	contents, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot encode dynamic model list")
	}

	if err := os.WriteFile(target, contents, 0o644); err != nil {
		log.Fatal().Err(err).Msg("Cannot write dynamic model list")
	}

	printConsoleLine(fmt.Sprintf("已准备模型列表：%d 个模型。", len(payload.Data)))
	return modelListResult{DefaultModel: defaultModel, Models: models}
}

func configuredStartupModelIDs() []string {
	models := configuredModelsForActiveProvider()
	ids := make([]string, 0, len(models)+1)
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	add(cfg.SelectedModel)
	for _, model := range models {
		add(model.ID)
	}
	return ids
}

func startBackgroundModelRefresh(portableCodexHome string) {
	if runtimeStore == nil {
		return
	}
	startupModels := runtimeStore.modelIDs()
	startupDefault := runtimeStore.defaultModel()
	printConsoleLine("Refreshing model list in the background; Codex Desktop will open without waiting.")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := runtimeStore.refreshActiveModels(ctx)
		cancel()
		if err != nil {
			log.Warn().Err(err).Msg("Cannot refresh active provider models")
		}
		if len(runtimeStore.modelIDs()) == 0 {
			if len(startupModels) > 0 {
				runtimeStore.seedCachedModels(startupModels, startupDefault)
			} else {
				runtimeStore.seedCachedModels(configuredStartupModelIDs(), strings.TrimSpace(cfg.SelectedModel))
			}
		}
		if err := runtimeStore.ensureActiveDefaultModel(); err != nil {
			log.Warn().Err(err).Msg("Cannot persist active default model")
		}
		modelList := syncDynamicModelList(portableCodexHome)
		syncCodexConfig(portableCodexHome, modelList.DefaultModel)
		printConsoleLine("Background model refresh finished.")
	}()
}

func fetchProviderModelIDs() ([]string, error) {
	_, provider, ok := activeProvider()
	if !ok {
		return nil, errors.New("selected provider is not configured")
	}

	endpoint, err := modelsEndpoint(provider.BaseURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned %s", endpoint, resp.Status)
	}

	var parsed openAIModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if id := modelIDFromRawJSON(item); id != "" {
			models = append(models, id)
		}
	}

	return models, nil
}

func modelsEndpoint(baseURL string) (string, error) {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return "", errors.New("base_url is empty")
	}
	if err := validateAPIBaseURL(raw); err != nil {
		return "", err
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base_url is not an absolute URL: %s", raw)
	}

	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "":
		parsed.Path = "/models"
	default:
		parsed.Path = path + "/models"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validateAPIBaseURL(baseURL string) error {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return errors.New("base_url is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("base_url is not an absolute URL: %s", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("base_url must use http or https: %s", raw)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("base_url should not include query parameters or fragments: %s", raw)
	}
	path := strings.ToLower(strings.TrimRight(parsed.EscapedPath(), "/"))
	for _, endpoint := range []string{"/models", "/responses", "/chat/completions", "/completions", "/messages"} {
		if strings.HasSuffix(path, endpoint) {
			return fmt.Errorf("base_url should be the upstream API root, not a full endpoint: %s", raw)
		}
	}
	return nil
}

func modelIDFromRawJSON(raw json.RawMessage) string {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}

	var asObject struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(raw, &asObject); err != nil {
		return ""
	}

	for _, candidate := range []string{asObject.ID, asObject.Model, asObject.Name} {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			return candidate
		}
	}
	return ""
}

func configuredModelsForActiveProvider() []effectiveModelConfig {
	providerID := activeProviderID()
	models := make([]effectiveModelConfig, 0, len(cfg.Models)+1)

	for _, model := range cfg.Models {
		if !isModelEnabled(model) {
			continue
		}
		model.ID = strings.TrimSpace(model.ID)
		if model.ID == "" {
			continue
		}
		modelProvider := strings.TrimSpace(fallback(model.Provider, providerID))
		if modelProvider != providerID {
			continue
		}
		model.Provider = modelProvider
		models = append(models, effectiveModelFromEntry(model, true))
	}

	selected := strings.TrimSpace(cfg.SelectedModel)
	if selected != "" && !effectiveModelsContain(models, selected) {
		models = append([]effectiveModelConfig{effectiveModelForID(selected)}, models...)
	}
	return models
}

func isModelEnabled(model modelEntryConfig) bool {
	return model.Enabled == nil || *model.Enabled
}

func effectiveModelForID(id string) effectiveModelConfig {
	providerID, provider, _ := activeProvider()
	for _, model := range cfg.Models {
		if !isModelEnabled(model) {
			continue
		}
		model.ID = strings.TrimSpace(model.ID)
		modelProvider := strings.TrimSpace(fallback(model.Provider, providerID))
		if model.ID == id && modelProvider == providerID {
			model.Provider = modelProvider
			return effectiveModelFromEntry(model, true)
		}
	}

	return effectiveModelFromEntry(modelEntryConfig{
		ID:       id,
		Provider: providerID,
		Preset:   provider.FetchedModelPreset,
	}, false)
}

func effectiveModelFromEntry(model modelEntryConfig, configured bool) effectiveModelConfig {
	providerID := strings.TrimSpace(fallback(model.Provider, activeProviderID()))
	provider, _ := providerByID(providerID)
	preset := cfg.ModelPresets[strings.TrimSpace(model.Preset)]

	return effectiveModelConfig{
		ID:          strings.TrimSpace(model.ID),
		Provider:    providerID,
		DisplayName: strings.TrimSpace(model.DisplayName),
		Description: strings.TrimSpace(model.Description),
		Codex:       mergeCodexNativeConfigs(provider.Codex, preset.Codex, model.Codex),
		Catalog:     mergeCatalogConfigs(provider.Catalog, preset.Catalog, model.Catalog),
		Configured:  configured,
	}
}

func providerByID(id string) (providerConfig, bool) {
	if id == "" || cfg.Providers == nil {
		return providerConfig{}, false
	}
	provider, ok := cfg.Providers[id]
	if !ok {
		return providerConfig{}, false
	}
	provider.Name = strings.TrimSpace(provider.Name)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.APIKey = strings.TrimSpace(provider.APIKey)
	provider.AuthMode = strings.TrimSpace(provider.AuthMode)
	provider.EnvKey = strings.TrimSpace(provider.EnvKey)
	provider.WireAPI = strings.TrimSpace(provider.WireAPI)
	provider.FetchedModelPreset = strings.TrimSpace(provider.FetchedModelPreset)
	return provider, true
}

func mergeEffectiveModels(configuredModels []effectiveModelConfig, fetchedIDs []string) []effectiveModelConfig {
	seen := map[string]bool{}
	models := []effectiveModelConfig{}

	add := func(model effectiveModelConfig) {
		model.ID = strings.TrimSpace(model.ID)
		if model.ID == "" || seen[model.ID] {
			return
		}
		seen[model.ID] = true
		models = append(models, model)
	}

	for _, model := range configuredModels {
		add(model)
	}
	for _, id := range fetchedIDs {
		add(effectiveModelForID(id))
	}
	return models
}

func effectiveModelIDs(models []effectiveModelConfig) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		if model.ID != "" {
			ids = append(ids, model.ID)
		}
	}
	return ids
}

func effectiveModelsContain(models []effectiveModelConfig, id string) bool {
	for _, model := range models {
		if model.ID == id {
			return true
		}
	}
	return false
}

func defaultModelForList(models []effectiveModelConfig) string {
	selected := strings.TrimSpace(cfg.SelectedModel)
	if selected != "" {
		for _, model := range models {
			if model.ID == selected {
				return selected
			}
		}
	}
	if len(models) == 0 {
		return emptyModelsLabel
	}
	return models[0].ID
}

func mergeCodexNativeConfigs(configs ...codexNativeConfig) codexNativeConfig {
	var result codexNativeConfig
	for _, config := range configs {
		if config.ModelReasoningEffort != "" {
			result.ModelReasoningEffort = config.ModelReasoningEffort
		}
		if config.PlanModeReasoningEffort != "" {
			result.PlanModeReasoningEffort = config.PlanModeReasoningEffort
		}
		if config.ModelReasoningSummary != "" {
			result.ModelReasoningSummary = config.ModelReasoningSummary
		}
		if config.ModelVerbosity != "" {
			result.ModelVerbosity = config.ModelVerbosity
		}
		if config.ModelContextWindow != 0 {
			result.ModelContextWindow = config.ModelContextWindow
		}
		if config.ModelAutoCompactTokenLimit != 0 {
			result.ModelAutoCompactTokenLimit = config.ModelAutoCompactTokenLimit
		}
	}
	return result
}

func mergeCatalogConfigs(configs ...catalogConfig) catalogConfig {
	var result catalogConfig
	for _, config := range configs {
		if config.DefaultReasoningEffort != "" {
			result.DefaultReasoningEffort = config.DefaultReasoningEffort
		}
		if len(config.SupportedReasoningEfforts) > 0 {
			result.SupportedReasoningEfforts = append([]string{}, config.SupportedReasoningEfforts...)
		}
	}
	return result
}

func buildCodexModels(effectiveModels []effectiveModelConfig, defaultID string) []codexModel {
	models := make([]codexModel, 0, len(effectiveModels))
	for _, model := range effectiveModels {
		displayName := fallback(model.DisplayName, model.ID)
		description := fallback(model.Description, "DataProxy models")
		defaultEffort := fallback(model.Catalog.DefaultReasoningEffort, "medium")
		models = append(models, codexModel{
			ID:                        model.ID,
			Model:                     model.ID,
			DisplayName:               displayName,
			Description:               description,
			DefaultReasoningEffort:    defaultEffort,
			SupportedReasoningEfforts: reasoningEffortsForCatalog(model.Catalog, defaultEffort),
			Hidden:                    false,
			IsDefault:                 model.ID == defaultID,
		})
	}
	return models
}

func reasoningEffortsForCatalog(catalog catalogConfig, defaultEffort string) []reasoningEffort {
	values := append([]string{}, catalog.SupportedReasoningEfforts...)
	if len(values) == 0 {
		values = []string{"low", "medium", "high"}
	}
	if defaultEffort != "" && !stringSliceContains(values, defaultEffort) {
		values = append(values, defaultEffort)
	}

	efforts := make([]reasoningEffort, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		efforts = append(efforts, reasoningEffort{
			ReasoningEffort: value,
			Description:     value + " effort",
		})
	}
	return efforts
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func renderCodexConfig(selectedModel string) string {
	model := fallback(selectedModel, emptyModelsLabel)
	modelProvider := "dataproxy-local"
	providerName := "Codex DataProxy"
	baseURL := "http://127.0.0.1:16666/v1"
	if runtimeHTTPServer != nil {
		baseURL = runtimeHTTPServer.proxyBaseURL()
	}
	modelConfig := effectiveModelForID(model)

	var b strings.Builder
	fmt.Fprintf(&b, "# Generated by Codex DataProxy. Edit providers in config/*.yaml or /settings.\n")
	fmt.Fprintf(&b, "model = %s\n", tomlString(model))
	fmt.Fprintf(&b, "model_provider = %s\n\n", tomlString(modelProvider))
	writeCodexNativeConfig(&b, modelConfig.Codex)
	fmt.Fprintf(&b, "\n[model_providers.%s]\n", modelProvider)
	fmt.Fprintf(&b, "name = %s\n", tomlString(providerName))
	fmt.Fprintf(&b, "base_url = %s\n", tomlString(baseURL))
	fmt.Fprintf(&b, "requires_openai_auth = true\n")
	fmt.Fprintf(&b, "wire_api = %s\n", tomlString(defaultWireAPI))
	fmt.Fprintf(&b, "\n")
	writePlatformCodexConfig(&b)
	return b.String()
}

func mergeCodexConfig(generated string, existing string) string {
	if strings.TrimSpace(existing) == "" {
		return generated
	}

	generatedBlocks := splitConfigTomlBlocks(generated)
	existingBlocks := splitConfigTomlBlocks(existing)
	preservedTopLevel := []string{}
	preservedSections := []configTomlBlock{}
	for _, block := range existingBlocks {
		if !block.hasHeader {
			preservedTopLevel = preserveUnmanagedTopLevelConfig(block.body)
			continue
		}
		if isManagedCodexConfigSection(block.name) || !hasConfigTomlContent(block.body) {
			continue
		}
		preservedSections = append(preservedSections, block)
	}

	var b strings.Builder
	for _, block := range generatedBlocks {
		if !block.hasHeader {
			for _, line := range block.body {
				b.WriteString(line)
			}
			for _, line := range preservedTopLevel {
				b.WriteString(line)
			}
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(block.header)
		for _, line := range block.body {
			b.WriteString(line)
		}
	}

	for _, block := range preservedSections {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(block.header)
		for _, line := range block.body {
			b.WriteString(line)
		}
	}

	b.WriteString("\n")
	return b.String()
}

func preserveUnmanagedTopLevelConfig(lines []string) []string {
	preserved := make([]string, 0, len(lines))
	for _, line := range lines {
		key, ok := configTomlLineKey(line)
		if ok && isManagedTopLevelCodexConfigKey(key) {
			continue
		}
		preserved = append(preserved, line)
	}
	return preserved
}

func isManagedTopLevelCodexConfigKey(key string) bool {
	switch key {
	case "model",
		"model_provider",
		"model_reasoning_effort",
		"plan_mode_reasoning_effort",
		"model_reasoning_summary",
		"model_verbosity",
		"model_context_window",
		"model_auto_compact_token_limit":
		return true
	default:
		return false
	}
}

func isManagedCodexConfigSection(name string) bool {
	return name == "model_providers.dataproxy-local" || name == "windows"
}

type configTomlBlock struct {
	name      string
	header    string
	body      []string
	hasHeader bool
}

func splitConfigTomlBlocks(contents string) []configTomlBlock {
	lines := strings.SplitAfter(contents, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	blocks := []configTomlBlock{{}}
	for _, line := range lines {
		if name, ok := configTomlSectionName(line); ok {
			blocks = append(blocks, configTomlBlock{name: name, header: line, hasHeader: true})
			continue
		}
		blocks[len(blocks)-1].body = append(blocks[len(blocks)-1].body, line)
	}
	return blocks
}

func configTomlSectionName(line string) (string, bool) {
	trimmed := strings.TrimSpace(stripConfigTomlComment(strings.TrimRight(line, "\r\n")))
	switch {
	case strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]"):
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "[["), "]]"))
		return name, name != ""
	case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
		return name, name != ""
	default:
		return "", false
	}
}

func configTomlLineKey(line string) (string, bool) {
	body := strings.TrimSpace(stripConfigTomlComment(strings.TrimRight(line, "\r\n")))
	key, _, ok := strings.Cut(body, "=")
	if !ok {
		return "", false
	}
	key = strings.TrimSpace(key)
	return key, key != ""
}

func hasConfigTomlContent(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(stripConfigTomlComment(strings.TrimRight(line, "\r\n")))
		if trimmed != "" {
			return true
		}
	}
	return false
}

func stripConfigTomlComment(line string) string {
	inBasicString := false
	inLiteralString := false
	escaped := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inBasicString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inBasicString = false
			}
			continue
		}
		if inLiteralString {
			if ch == '\'' {
				inLiteralString = false
			}
			continue
		}
		switch ch {
		case '"':
			inBasicString = true
		case '\'':
			inLiteralString = true
		case '#':
			return line[:i]
		}
	}
	return line
}

func writeCodexNativeConfig(b *strings.Builder, config codexNativeConfig) {
	if config.ModelReasoningEffort != "" {
		fmt.Fprintf(b, "model_reasoning_effort = %s\n", tomlString(config.ModelReasoningEffort))
	}
	if config.PlanModeReasoningEffort != "" {
		fmt.Fprintf(b, "plan_mode_reasoning_effort = %s\n", tomlString(config.PlanModeReasoningEffort))
	}
	if config.ModelReasoningSummary != "" {
		fmt.Fprintf(b, "model_reasoning_summary = %s\n", tomlString(config.ModelReasoningSummary))
	}
	if config.ModelVerbosity != "" {
		fmt.Fprintf(b, "model_verbosity = %s\n", tomlString(config.ModelVerbosity))
	}
	if config.ModelContextWindow != 0 {
		fmt.Fprintf(b, "model_context_window = %d\n", config.ModelContextWindow)
	}
	if config.ModelAutoCompactTokenLimit != 0 {
		fmt.Fprintf(b, "model_auto_compact_token_limit = %d\n", config.ModelAutoCompactTokenLimit)
	}
}

func fallback(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func tomlString(value string) string {
	return strconv.Quote(value)
}
