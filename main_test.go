package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestModelsEndpoint(t *testing.T) {
	tests := map[string]string{
		"https://dp.app.mbu.ltd":          "https://dp.app.mbu.ltd/v1/models",
		"https://dp.app.mbu.ltd/v1":       "https://dp.app.mbu.ltd/v1/models",
		"https://example.test/openai/v1/": "https://example.test/openai/v1/models",
		"https://example.test/v1/models":  "https://example.test/v1/models",
	}

	for input, want := range tests {
		got, err := modelsEndpoint(input)
		if err != nil {
			t.Fatalf("modelsEndpoint(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("modelsEndpoint(%q) = %q, want %q", input, got, want)
		}
	}
}

func withDefaultTestConfig(t *testing.T) {
	t.Helper()
	old := cfg
	cfg = defaultCodexConfig()
	t.Cleanup(func() { cfg = old })
}

func TestFetchProviderModelIDs(t *testing.T) {
	withDefaultTestConfig(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-ai/DeepSeek-V4-Flash"},{"id":"qwen/qwen3-coder"}]}`))
	}))
	defer server.Close()

	provider := cfg.Providers["dataproxy"]
	provider.BaseURL = server.URL + "/v1"
	provider.APIKey = "sk-test"
	cfg.Providers["dataproxy"] = provider

	got, err := fetchProviderModelIDs()
	if err != nil {
		t.Fatalf("fetchProviderModelIDs returned error: %v", err)
	}
	want := []string{"deepseek-ai/DeepSeek-V4-Flash", "qwen/qwen3-coder"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fetchProviderModelIDs() = %#v, want %#v", got, want)
	}
}

func TestFetchProviderModelIDsAllowsEmptyList(t *testing.T) {
	withDefaultTestConfig(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	provider := cfg.Providers["dataproxy"]
	provider.BaseURL = server.URL + "/v1"
	provider.APIKey = "sk-test"
	cfg.Providers["dataproxy"] = provider

	got, err := fetchProviderModelIDs()
	if err != nil {
		t.Fatalf("fetchProviderModelIDs returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("fetchProviderModelIDs() = %#v, want empty list", got)
	}
}

func TestConfiguredModelsUsePresets(t *testing.T) {
	withDefaultTestConfig(t)

	models := configuredModelsForActiveProvider()
	if len(models) != 1 {
		t.Fatalf("configuredModelsForActiveProvider() returned %d models, want 1", len(models))
	}
	model := models[0]
	if model.ID != "gpt-5.5" {
		t.Fatalf("configured model ID = %q, want gpt-5.5", model.ID)
	}
	if model.Codex.ModelReasoningEffort != "xhigh" {
		t.Fatalf("model reasoning effort = %q, want xhigh", model.Codex.ModelReasoningEffort)
	}
	if model.Codex.ModelContextWindow != 1048576 {
		t.Fatalf("model context window = %d, want 1048576", model.Codex.ModelContextWindow)
	}
}

func TestApplyUserConfigUsesSimplifiedFields(t *testing.T) {
	withDefaultTestConfig(t)

	raw := []byte(`
base_url: "https://example.test/v1"
api_key: "sk-test"
model: "deepseek-ai/DeepSeek-V4-Flash"
models: "gpt-5.5,deepseek-ai/DeepSeek-V4-Flash"
`)
	var user userConfig
	if err := yaml.Unmarshal(raw, &user); err != nil {
		t.Fatalf("simplified config should parse: %v", err)
	}

	applyUserConfig(user)

	if cfg.SelectedModel != "deepseek-ai/DeepSeek-V4-Flash" {
		t.Fatalf("SelectedModel = %q", cfg.SelectedModel)
	}
	provider := cfg.Providers["dataproxy"]
	if provider.BaseURL != "https://example.test/v1" || provider.APIKey != "sk-test" {
		t.Fatalf("provider = %#v", provider)
	}
	if provider.AuthMode != authModeCodexAPIKey || provider.WireAPI != defaultWireAPI || !provider.FetchModels {
		t.Fatalf("provider defaults were not preserved: %#v", provider)
	}
	if len(cfg.Models) != 2 {
		t.Fatalf("configured models = %#v, want 2", cfg.Models)
	}
	if cfg.Models[0].Preset != "gpt_reasoning_1m" {
		t.Fatalf("gpt-5.5 preset = %q, want gpt_reasoning_1m", cfg.Models[0].Preset)
	}
	if cfg.Models[1].Preset != "default_gpt" {
		t.Fatalf("second model preset = %q, want default_gpt", cfg.Models[1].Preset)
	}
}

func TestModelIDListSupportsSequenceAndCommaSeparatedValues(t *testing.T) {
	raw := []byte(`
models:
  - "gpt-5.5,deepseek-ai/DeepSeek-V4-Flash"
  - "qwen/qwen3-coder"
`)
	var user userConfig
	if err := yaml.Unmarshal(raw, &user); err != nil {
		t.Fatalf("models should parse: %v", err)
	}
	got := []string(*user.Models)
	want := []string{"gpt-5.5", "deepseek-ai/DeepSeek-V4-Flash", "qwen/qwen3-coder"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %#v, want %#v", got, want)
	}
}

func TestUserLaunchArgsFiltersInternalRetryArg(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"codex-dataproxy.exe", unelevatedRetryArg, "--foo", "bar"}
	t.Cleanup(func() { os.Args = oldArgs })

	got := userLaunchArgs()
	want := []string{"--foo", "bar"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("userLaunchArgs() = %#v, want %#v", got, want)
	}
	if !hasUnelevatedRetryArg() {
		t.Fatalf("hasUnelevatedRetryArg() should be true")
	}
}

func TestProviderStoreModelRoutesPreferManualAndLaterKeys(t *testing.T) {
	store := &providerStore{
		autoModels: map[string][]string{
			"dataproxy/auto": {"auto-only", "shared"},
		},
		routes: map[string]modelRoute{},
		providers: []upstreamProviderConfig{
			{
				ID:           "dataproxy",
				Name:         "DataProxy",
				Active:       true,
				Enabled:      true,
				BaseURL:      "https://dp.app.mbu.ltd/v1",
				DefaultModel: "manual-only",
				Keys: []upstreamKeyConfig{
					{ID: "auto", Name: "Auto", APIKey: "sk-auto", Enabled: true, Sort: 10},
					{ID: "manual", Name: "Manual", APIKey: "sk-manual", Enabled: true, Sort: 20, Models: "manual-only,shared"},
				},
			},
		},
	}
	store.rebuildRoutesLocked()

	if got := store.modelIDs(); !reflect.DeepEqual(got, []string{"auto-only", "manual-only", "shared"}) {
		t.Fatalf("modelIDs() = %#v", got)
	}
	_, key, ok := store.keyForModel("shared")
	if !ok {
		t.Fatalf("shared model should have a route")
	}
	if key.ID != "manual" {
		t.Fatalf("shared model key = %q, want manual", key.ID)
	}
}

func TestMergeEffectiveModelsKeepsConfiguredModelsFirst(t *testing.T) {
	withDefaultTestConfig(t)

	configured := []effectiveModelConfig{
		{ID: "configured-model", DisplayName: "Configured"},
		{ID: "manual-model", DisplayName: "Manual"},
	}
	got := effectiveModelIDs(mergeEffectiveModels(configured, []string{"other-model", "configured-model", "other-model"}))
	want := []string{"configured-model", "manual-model", "other-model"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeEffectiveModels() = %#v, want %#v", got, want)
	}
}

func TestBuildCodexModelsUsesCatalogReasoningEfforts(t *testing.T) {
	withDefaultTestConfig(t)

	model := configuredModelsForActiveProvider()[0]
	got := buildCodexModels([]effectiveModelConfig{model}, model.ID)
	if len(got) != 1 {
		t.Fatalf("buildCodexModels() returned %d models, want 1", len(got))
	}
	if got[0].DefaultReasoningEffort != "xhigh" {
		t.Fatalf("DefaultReasoningEffort = %q, want xhigh", got[0].DefaultReasoningEffort)
	}
	if got[0].SupportedReasoningEfforts[len(got[0].SupportedReasoningEfforts)-1].ReasoningEffort != "xhigh" {
		t.Fatalf("last reasoning effort = %#v, want xhigh", got[0].SupportedReasoningEfforts)
	}
	if !got[0].IsDefault {
		t.Fatalf("configured model should be marked default")
	}
}

func TestRenderCodexConfigIncludesModelPresetSettings(t *testing.T) {
	withDefaultTestConfig(t)

	provider := cfg.Providers["dataproxy"]
	provider.BaseURL = "https://dp.app.mbu.ltd/v1"
	provider.APIKey = "sk-test"
	cfg.Providers["dataproxy"] = provider

	got := renderCodexConfig("gpt-5.5")
	for _, want := range []string{
		`model = "gpt-5.5"`,
		`model_provider = "dataproxy-local"`,
		`model_reasoning_effort = "xhigh"`,
		`plan_mode_reasoning_effort = "high"`,
		`model_reasoning_summary = "auto"`,
		`model_verbosity = "medium"`,
		`model_context_window = 1048576`,
		`model_auto_compact_token_limit = 900000`,
		`[model_providers.dataproxy-local]`,
		`base_url = "http://127.0.0.1:16666/v1"`,
		`requires_openai_auth = true`,
		`wire_api = "responses"`,
		`[windows]`,
		`sandbox = "unelevated"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderCodexConfig() missing %q in:\n%s", want, got)
		}
	}

	for _, unwanted := range []string{`approval_policy`, `sandbox_mode`, `env_key`} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("renderCodexConfig() should not include %q in:\n%s", unwanted, got)
		}
	}
}

func TestRenderCodexConfigAlwaysUsesLocalProxyAuth(t *testing.T) {
	withDefaultTestConfig(t)

	provider := cfg.Providers["dataproxy"]
	provider.BaseURL = "https://dp.app.mbu.ltd/v1"
	provider.APIKey = "sk-test"
	provider.AuthMode = authModeProviderEnv
	provider.EnvKey = "DATAPROXY_API_KEY"
	cfg.Providers["dataproxy"] = provider

	got := renderCodexConfig("gpt-5.5")
	for _, want := range []string{
		`model_provider = "dataproxy-local"`,
		`requires_openai_auth = true`,
		`wire_api = "responses"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderCodexConfig() missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, `env_key`) {
		t.Fatalf("renderCodexConfig() should not include env_key when using local proxy:\n%s", got)
	}
}

func TestSyncCodexAuthWritesLocalProxyKey(t *testing.T) {
	withDefaultTestConfig(t)

	provider := cfg.Providers["dataproxy"]
	provider.APIKey = "sk-test"
	cfg.Providers["dataproxy"] = provider

	dir := t.TempDir()
	syncCodexAuth(dir)

	contents, err := os.ReadFile(filepath.Join(dir, authFileName))
	if err != nil {
		t.Fatalf("auth.json was not written: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(contents, &got); err != nil {
		t.Fatalf("auth.json is not valid JSON: %v", err)
	}
	if got["OPENAI_API_KEY"] != "codex-dataproxy-local" {
		t.Fatalf("OPENAI_API_KEY = %q, want local proxy key", got["OPENAI_API_KEY"])
	}
}

func TestRestorePermissionModeVisibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".codex-global-state.json")
	initial := []byte(`{"electron-persisted-atom-state":{"composer-permission-mode-visibility":{"guardian-approvals":false,"full-access":false},"agent-mode-by-host-id":{"local":"read-only"}},"other":true}`)
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatalf("cannot write test state: %v", err)
	}

	if err := restorePermissionModeVisibility(path); err != nil {
		t.Fatalf("restorePermissionModeVisibility returned error: %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read restored state: %v", err)
	}

	var state map[string]any
	if err := json.Unmarshal(contents, &state); err != nil {
		t.Fatalf("restored state is not valid JSON: %v", err)
	}
	persisted, ok := state[persistedAtomState].(map[string]any)
	if !ok {
		t.Fatalf("persisted atom state missing or wrong type: %#v", state[persistedAtomState])
	}
	visibility, ok := persisted[permissionVisibility].(map[string]any)
	if !ok {
		t.Fatalf("visibility state missing or wrong type: %#v", persisted[permissionVisibility])
	}
	if visibility["guardian-approvals"] != true || visibility["full-access"] != true {
		t.Fatalf("visibility state = %#v, want both true", visibility)
	}
	agentModes, ok := persisted["agent-mode-by-host-id"].(map[string]any)
	if !ok {
		t.Fatalf("agent-mode-by-host-id missing or wrong type: %#v", persisted["agent-mode-by-host-id"])
	}
	if agentModes["local"] != "auto" {
		t.Fatalf("local agent mode = %#v, want auto", agentModes["local"])
	}
	if _, exists := state[permissionVisibility]; exists {
		t.Fatalf("top-level visibility state should not be left behind: %#v", state)
	}
	if state["other"] != true {
		t.Fatalf("unrelated state should be preserved: %#v", state)
	}
}
