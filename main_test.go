package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestModelsEndpoint(t *testing.T) {
	tests := map[string]string{
		"https://dp.app.mbu.ltd":          "https://dp.app.mbu.ltd/models",
		"https://dp.app.mbu.ltd/v1":       "https://dp.app.mbu.ltd/v1/models",
		"https://example.test/openai/v1/": "https://example.test/openai/v1/models",
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

	for _, input := range []string{
		"https://example.test/v1/models",
		"https://example.test/v1/responses",
		"https://example.test/v1/chat/completions",
		"https://example.test/v1/messages",
		"https://example.test/v1?target=responses",
		"https://example.test/v1#responses",
		"ftp://example.test/v1",
	} {
		if _, err := modelsEndpoint(input); err == nil {
			t.Fatalf("modelsEndpoint(%q) should reject full endpoint base_url", input)
		}
	}
}

func TestProxyTargetURLUsesConfiguredBasePath(t *testing.T) {
	requestURL, err := url.Parse("http://127.0.0.1:16666/v1/responses?stream=true")
	if err != nil {
		t.Fatalf("cannot parse request URL: %v", err)
	}
	tests := map[string]string{
		"https://dp.app.mbu.ltd":          "https://dp.app.mbu.ltd/responses?stream=true",
		"https://dp.app.mbu.ltd/v1":       "https://dp.app.mbu.ltd/v1/responses?stream=true",
		"https://example.test/openai/v1/": "https://example.test/openai/v1/responses?stream=true",
	}

	for input, want := range tests {
		got, err := proxyTargetURL(input, requestURL)
		if err != nil {
			t.Fatalf("proxyTargetURL(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("proxyTargetURL(%q) = %q, want %q", input, got, want)
		}
	}

	if _, err := proxyTargetURL("https://example.test/v1/chat/completions", requestURL); err == nil {
		t.Fatalf("proxyTargetURL should reject full endpoint base_url")
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

func TestProviderStoreRefreshTracksPerKeyStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer sk-good":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "good-model"}},
			})
		default:
			http.Error(w, "nope", http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	store := &providerStore{
		dir:             t.TempDir(),
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
		providers: []upstreamProviderConfig{
			{
				ID:      "dataproxy",
				Name:    "DataProxy",
				Active:  true,
				Enabled: true,
				BaseURL: server.URL + "/v1",
				Keys: []upstreamKeyConfig{
					{ID: "good", APIKey: "sk-good", Enabled: true, Sort: 10},
					{ID: "bad", APIKey: "sk-bad", Enabled: true, Sort: 20},
					{ID: "manual", APIKey: "sk-manual", Enabled: true, Sort: 30, Models: "manual-model"},
				},
			},
		},
	}

	if err := store.refreshActiveModels(context.Background()); err == nil {
		t.Fatalf("refreshActiveModels should report the first failing key")
	}
	if got := store.modelIDs(); !reflect.DeepEqual(got, []string{"good-model", "manual-model"}) {
		t.Fatalf("modelIDs() = %#v", got)
	}
	if status := store.keyStatus["dataproxy/good"]; status.Status != "ok" || status.ModelCount != 1 {
		t.Fatalf("good status = %#v, want ok with 1 model", status)
	}
	if status := store.keyStatus["dataproxy/bad"]; status.Status != "failed" || status.Error == "" {
		t.Fatalf("bad status = %#v, want failed with error", status)
	}
	if status := store.keyStatus["dataproxy/manual"]; status.Status != "manual" || status.ModelCount != 1 {
		t.Fatalf("manual status = %#v, want manual with 1 model", status)
	}
}

func TestProviderStoreRefreshClearsStaleAutoModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "expired", http.StatusUnauthorized)
	}))
	defer server.Close()

	store := &providerStore{
		dir: t.TempDir(),
		autoModels: map[string][]string{
			"dataproxy/bad": {"stale-model"},
		},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus: map[string]keyModelStatus{
			"dataproxy/old": {ProviderID: "dataproxy", KeyID: "old", Status: "ok"},
		},
		providers: []upstreamProviderConfig{
			{
				ID:      "dataproxy",
				Name:    "DataProxy",
				Active:  true,
				Enabled: true,
				BaseURL: server.URL + "/v1",
				Keys: []upstreamKeyConfig{
					{ID: "bad", APIKey: "sk-bad", Enabled: true, Sort: 10},
				},
			},
		},
	}
	store.rebuildRoutesLocked()
	if got := store.modelIDs(); !reflect.DeepEqual(got, []string{"stale-model"}) {
		t.Fatalf("initial modelIDs() = %#v, want stale model", got)
	}

	if err := store.refreshActiveModels(context.Background()); err == nil {
		t.Fatalf("refreshActiveModels should fail against unreachable test upstream")
	}
	if got := store.modelIDs(); len(got) != 0 {
		t.Fatalf("modelIDs() after failed refresh = %#v, want stale models cleared", got)
	}
	if _, ok := store.keyStatus["dataproxy/old"]; ok {
		t.Fatalf("stale key status should be cleared: %#v", store.keyStatus)
	}
	if status := store.keyStatus["dataproxy/bad"]; status.Status != "failed" || status.Error == "" {
		t.Fatalf("bad status = %#v, want failed with error", status)
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
	os.Args = []string{"codex-dp.exe", unelevatedRetryArg, "--foo", "bar"}
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
		autoModels:      map[string][]string{"dataproxy/auto": {"auto-only", "shared"}},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
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

func TestProviderStoreModelDiagnosticsAliasesAndPatterns(t *testing.T) {
	store := &providerStore{
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
		providers: []upstreamProviderConfig{
			{
				ID:           "dataproxy",
				Name:         "DataProxy",
				Active:       true,
				Enabled:      true,
				BaseURL:      "https://dp.app.mbu.ltd/v1",
				ModelAliases: map[string]string{"gpt-5.5": "shared"},
				Keys: []upstreamKeyConfig{
					{ID: "first", Name: "First", APIKey: "sk-first", Enabled: true, Sort: 10, Models: "shared"},
					{ID: "second", Name: "Second", APIKey: "sk-second", Enabled: true, Sort: 20, Models: "shared,qwen/*"},
				},
			},
		},
	}
	store.rebuildRoutesLocked()

	if got := len(store.duplicateRoutes["shared"]); got != 2 {
		t.Fatalf("duplicate route count = %d, want 2: %#v", got, store.duplicateRoutes["shared"])
	}
	_, key, target, ok := store.keyForModelWithTarget("gpt-5.5")
	if !ok || key.ID != "second" || target != "shared" {
		t.Fatalf("alias route = key:%#v target:%q ok:%v, want second/shared/true", key, target, ok)
	}
	_, key, target, ok = store.keyForModelWithTarget("qwen/qwen3-coder")
	if !ok || key.ID != "second" || target != "qwen/qwen3-coder" {
		t.Fatalf("pattern route = key:%#v target:%q ok:%v, want second/qwen/qwen3-coder/true", key, target, ok)
	}
}

func TestProviderStoreNormalizesDuplicateKeyIDs(t *testing.T) {
	store := &providerStore{
		dir:             t.TempDir(),
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
	}
	provider := upstreamProviderConfig{
		ID:      "dataproxy",
		Name:    "DataProxy",
		Active:  true,
		Enabled: true,
		BaseURL: "https://example.test/v1",
		Keys: []upstreamKeyConfig{
			{ID: "main", APIKey: "sk-one", Enabled: true, Sort: 10},
			{ID: "main", APIKey: "sk-two", Enabled: true, Sort: 20},
			{ID: "", Name: "main", APIKey: "sk-three", Enabled: true, Sort: 30},
		},
	}
	if err := store.upsertProvider(provider); err != nil {
		t.Fatalf("upsertProvider returned error: %v", err)
	}
	got, ok := store.providerByID("dataproxy")
	if !ok {
		t.Fatalf("provider missing after upsert")
	}
	ids := []string{got.Keys[0].ID, got.Keys[1].ID, got.Keys[2].ID}
	if !reflect.DeepEqual(ids, []string{"main", "main-2", "main-3"}) {
		t.Fatalf("key ids = %#v, want main/main-2/main-3", ids)
	}
}

func TestLocalProxyRewritesAliasedModelAndStreamsResponse(t *testing.T) {
	var upstreamBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-route" {
			t.Fatalf("authorization = %q, want route key", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("cannot read upstream body: %v", err)
		}
		upstreamBody = string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("data: one\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: two\n\n"))
	}))
	defer server.Close()

	store := &providerStore{
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
		providers: []upstreamProviderConfig{
			{
				ID:           "dataproxy",
				Name:         "DataProxy",
				Active:       true,
				Enabled:      true,
				BaseURL:      server.URL + "/v1",
				ModelAliases: map[string]string{"codex-model": "upstream-model"},
				Keys: []upstreamKeyConfig{
					{ID: "route", APIKey: "sk-route", Enabled: true, Sort: 10, Models: "upstream-model"},
				},
			},
		},
	}
	store.rebuildRoutesLocked()
	local := &localHTTPServer{store: store}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"codex-model","input":"hello"}`))
	req.Header.Set("Authorization", "Bearer "+localProxyAPIKey)
	rec := httptest.NewRecorder()
	local.handleProxy(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("proxy status = %d, want %d; body=%s", resp.StatusCode, http.StatusAccepted, rec.Body.String())
	}
	if !strings.Contains(upstreamBody, `"model":"upstream-model"`) {
		t.Fatalf("upstream body was not rewritten: %s", upstreamBody)
	}
	if got := rec.Body.String(); got != "data: one\n\ndata: two\n\n" {
		t.Fatalf("stream body = %q", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"codex-model"}`))
	rec = httptest.NewRecorder()
	local.handleProxy(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized proxy status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec = httptest.NewRecorder()
	local.handleProxy(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized model list status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+localProxyAPIKey)
	rec = httptest.NewRecorder()
	local.handleProxy(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authorized model list status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestSettingsAPIRequiresCSRFAndSupportsReorderDelete(t *testing.T) {
	withDefaultTestConfig(t)

	store := &providerStore{
		dir:             t.TempDir(),
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
		providers: []upstreamProviderConfig{
			{
				ID:      "alpha",
				Name:    "Alpha",
				Active:  true,
				Enabled: true,
				Sort:    10,
				BaseURL: "https://alpha.example/v1",
				Keys:    []upstreamKeyConfig{{ID: "key-1", APIKey: "sk-alpha", Enabled: true, Sort: 10}},
			},
			{
				ID:      "beta",
				Name:    "Beta",
				Enabled: true,
				Sort:    20,
				BaseURL: "https://beta.example/v1",
				Keys:    []upstreamKeyConfig{{ID: "key-1", APIKey: "sk-beta", Enabled: true, Sort: 10}},
			},
		},
	}
	for _, provider := range store.providers {
		if err := store.writeProvider(provider); err != nil {
			t.Fatalf("writeProvider returned error: %v", err)
		}
	}
	codexHome := t.TempDir()
	local := &localHTTPServer{
		store:     store,
		codexHome: codexHome,
		baseURL:   "http://127.0.0.1:16666",
		csrfToken: "test-token",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/providers/reorder", local.handleReorderProviders)
	mux.HandleFunc("/api/providers/delete", local.handleDeleteProvider)
	handler := local.guard(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/providers/reorder", bytes.NewBufferString(`{"ids":["beta","alpha"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", local.baseURL)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reorder without csrf status = %d, want 403", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/providers/reorder", bytes.NewBufferString(`{"ids":["beta","alpha"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", local.baseURL)
	req.Header.Set("X-CSRF-Token", "test-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reorder status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := []string{store.providers[0].ID, store.providers[1].ID}; !reflect.DeepEqual(got, []string{"beta", "alpha"}) {
		t.Fatalf("provider order = %#v", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/providers/delete", bytes.NewBufferString(`{"id":"beta"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", local.baseURL)
	req.Header.Set("X-CSRF-Token", "test-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.providers) != 1 || store.providers[0].ID != "alpha" {
		t.Fatalf("providers after delete = %#v", store.providers)
	}
	if _, err := os.Stat(filepath.Join(store.dir, "beta.yaml")); !os.IsNotExist(err) {
		t.Fatalf("beta.yaml should be removed, stat err=%v", err)
	}
}

func TestSettingsAPIRedactsPreservesAndTestsAPIKeys(t *testing.T) {
	withDefaultTestConfig(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-secret-value" {
			t.Fatalf("authorization = %q, want saved secret", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "tested-model"}},
		})
	}))
	defer upstream.Close()

	store := &providerStore{
		dir:             t.TempDir(),
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
		providers: []upstreamProviderConfig{
			{
				ID:      "dataproxy",
				Name:    "DataProxy",
				Active:  true,
				Enabled: true,
				BaseURL: upstream.URL + "/v1",
				Keys:    []upstreamKeyConfig{{ID: "main", APIKey: "sk-secret-value", Enabled: true, Sort: 10}},
			},
		},
	}
	if err := store.writeProvider(store.providers[0]); err != nil {
		t.Fatalf("writeProvider returned error: %v", err)
	}
	local := &localHTTPServer{
		store:     store,
		codexHome: t.TempDir(),
		baseURL:   "http://127.0.0.1:16666",
		csrfToken: "test-token",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/providers", local.handleProviders)
	mux.HandleFunc("/api/providers/test", local.handleTestProvider)
	handler := local.guard(mux)

	getReq := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET providers status = %d, body=%s", getRec.Code, getRec.Body.String())
	}
	var list struct {
		Providers []upstreamProviderConfig `json:"providers"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("cannot decode providers: %v", err)
	}
	if got := list.Providers[0].Keys[0].APIKey; got == "sk-secret-value" || !strings.Contains(got, "***") {
		t.Fatalf("redacted api key = %q", got)
	}
	if !list.Providers[0].Keys[0].APIKeySet {
		t.Fatalf("api_key_set should be true")
	}

	update := list.Providers[0]
	update.Name = "Updated"
	body, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("cannot marshal provider update: %v", err)
	}
	putReq := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Origin", local.baseURL)
	putReq.Header.Set("X-CSRF-Token", "test-token")
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT providers status = %d, body=%s", putRec.Code, putRec.Body.String())
	}
	provider, ok := store.providerByID("dataproxy")
	if !ok || provider.Keys[0].APIKey != "sk-secret-value" {
		t.Fatalf("stored provider after masked update = %#v", provider)
	}

	testReq := httptest.NewRequest(http.MethodPost, "/api/providers/test", bytes.NewBufferString(`{"id":"dataproxy","key_id":"main"}`))
	testReq.Header.Set("Content-Type", "application/json")
	testReq.Header.Set("Origin", local.baseURL)
	testReq.Header.Set("X-CSRF-Token", "test-token")
	testRec := httptest.NewRecorder()
	handler.ServeHTTP(testRec, testReq)
	if testRec.Code != http.StatusOK {
		t.Fatalf("provider test status = %d, body=%s", testRec.Code, testRec.Body.String())
	}
	var result providerTestResult
	if err := json.Unmarshal(testRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("cannot decode test result: %v", err)
	}
	if !result.OK || len(result.Results) != 1 || result.Results[0].Status != "ok" || result.Results[0].ModelCount != 1 {
		t.Fatalf("test result = %#v", result)
	}
}

func TestPostJSONUsesDefaultTimeout(t *testing.T) {
	oldTimeout := upstreamRequestTimeout
	upstreamRequestTimeout = 25 * time.Millisecond
	t.Cleanup(func() { upstreamRequestTimeout = oldTimeout })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	start := time.Now()
	err := postJSON(context.Background(), server.URL, "", map[string]bool{"ok": true}, nil)
	if err == nil {
		t.Fatalf("postJSON should time out without an explicit caller deadline")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("postJSON timeout took too long: %s", elapsed)
	}
}

func TestDataProxyDeviceFlowStoresConnectedToken(t *testing.T) {
	withDefaultTestConfig(t)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/connected-apps/codex-dp/device/start":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("cannot decode device/start: %v", err)
			}
			if payload["client"] != "codex-dp" || payload["device_id"] == "" {
				t.Fatalf("unexpected device/start payload: %#v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dev-test",
				"user_code":                 "CDP-TEST",
				"verification_uri_complete": server.URL + "/activate?user_code=CDP-TEST",
				"expires_in":                600,
				"interval":                  1,
			})
		case "/api/connected-apps/codex-dp/device/poll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":           "authorized",
				"management_token": "cdpat-test",
				"server_url":       server.URL,
				"base_url":         server.URL + "/v1",
			})
		case "/api/connected-apps/codex-dp/tokens/ensure":
			if got := r.Header.Get("Authorization"); got != "Bearer cdpat-test" {
				t.Fatalf("tokens/ensure auth = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"selected": true,
				"api_key":  "sk-connected",
				"base_url": server.URL + "/v1",
				"token": map[string]any{
					"id":         123,
					"masked_key": "sk-conn****cted",
				},
			})
		case "/v1/models":
			if got := r.Header.Get("Authorization"); got != "Bearer sk-connected" {
				t.Fatalf("models auth = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "connected-model"}},
			})
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	store := &providerStore{
		dir:             t.TempDir(),
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
	}
	oldRuntimeStore := runtimeStore
	runtimeStore = store
	t.Cleanup(func() { runtimeStore = oldRuntimeStore })

	local := &localHTTPServer{
		store:     store,
		codexHome: t.TempDir(),
		baseURL:   "http://127.0.0.1:16666",
		csrfToken: "test-token",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/dataproxy/device/start", local.handleDataProxyDeviceStart)
	mux.HandleFunc("/api/dataproxy/device/poll", local.handleDataProxyDevicePoll)
	handler := local.guard(mux)

	startReq := httptest.NewRequest(http.MethodPost, "/api/dataproxy/device/start", bytes.NewBufferString(`{"server_url":"`+server.URL+`"}`))
	startReq.Header.Set("Content-Type", "application/json")
	startReq.Header.Set("Origin", local.baseURL)
	startReq.Header.Set("X-CSRF-Token", "test-token")
	startRec := httptest.NewRecorder()
	handler.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("device/start status = %d, body=%s", startRec.Code, startRec.Body.String())
	}
	var start dataProxyDeviceStartResponse
	if err := json.Unmarshal(startRec.Body.Bytes(), &start); err != nil {
		t.Fatalf("cannot decode start response: %v", err)
	}
	if start.DeviceCode != "dev-test" || start.DeviceID == "" {
		t.Fatalf("start response = %#v", start)
	}

	pollBody := fmt.Sprintf(`{"server_url":%q,"device_id":%q,"device_code":"dev-test"}`, server.URL, start.DeviceID)
	pollReq := httptest.NewRequest(http.MethodPost, "/api/dataproxy/device/poll", bytes.NewBufferString(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollReq.Header.Set("Origin", local.baseURL)
	pollReq.Header.Set("X-CSRF-Token", "test-token")
	pollRec := httptest.NewRecorder()
	handler.ServeHTTP(pollRec, pollReq)
	if pollRec.Code != http.StatusOK {
		t.Fatalf("device/poll status = %d, body=%s", pollRec.Code, pollRec.Body.String())
	}
	var poll dataProxyDevicePollResponse
	if err := json.Unmarshal(pollRec.Body.Bytes(), &poll); err != nil {
		t.Fatalf("cannot decode poll response: %v", err)
	}
	if poll.Status != "authorized" || poll.ProviderID != "dataproxy" || poll.KeyID != "connected-app" || poll.ManagementToken != "" {
		t.Fatalf("poll response = %#v", poll)
	}
	provider, ok := store.providerByID("dataproxy")
	if !ok {
		t.Fatalf("dataproxy provider was not stored")
	}
	if !provider.Active || provider.BaseURL != server.URL+"/v1" || len(provider.Keys) == 0 || provider.Keys[0].APIKey != "sk-connected" {
		t.Fatalf("stored provider = %#v", provider)
	}
	if got := store.modelIDs(); !reflect.DeepEqual(got, []string{"connected-model"}) {
		t.Fatalf("modelIDs() = %#v", got)
	}
}

func TestDataProxyDeviceFlowRotatesWhenEnsureOmitsAPIKey(t *testing.T) {
	withDefaultTestConfig(t)

	rotateCalled := false
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/connected-apps/codex-dp/device/poll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":           "authorized",
				"management_token": "cdpat-test",
				"server_url":       server.URL,
				"base_url":         server.URL + "/v1",
			})
		case "/api/connected-apps/codex-dp/tokens/ensure":
			if got := r.Header.Get("Authorization"); got != "Bearer cdpat-test" {
				t.Fatalf("tokens/ensure auth = %q", got)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("cannot decode tokens/ensure: %v", err)
			}
			if payload["rotate"] == true {
				t.Fatalf("first ensure call should not request rotate: %#v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"selected": true,
				"base_url": server.URL + "/v1",
				"token": map[string]any{
					"id":         123,
					"masked_key": "sk-conn****cted",
				},
			})
		case "/api/connected-apps/codex-dp/tokens/123/rotate":
			rotateCalled = true
			if got := r.Header.Get("Authorization"); got != "Bearer cdpat-test" {
				t.Fatalf("tokens/rotate auth = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"selected": true,
				"api_key":  "sk-rotated",
				"base_url": server.URL + "/v1",
				"token": map[string]any{
					"id":         123,
					"masked_key": "sk-rota****ated",
				},
			})
		case "/v1/models":
			if got := r.Header.Get("Authorization"); got != "Bearer sk-rotated" {
				t.Fatalf("models auth = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "rotated-model"}},
			})
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	store := &providerStore{
		dir:             t.TempDir(),
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
	}
	oldRuntimeStore := runtimeStore
	runtimeStore = store
	t.Cleanup(func() { runtimeStore = oldRuntimeStore })

	local := &localHTTPServer{
		store:     store,
		codexHome: t.TempDir(),
		baseURL:   "http://127.0.0.1:16666",
		csrfToken: "test-token",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/dataproxy/device/poll", local.handleDataProxyDevicePoll)
	handler := local.guard(mux)

	pollBody := fmt.Sprintf(`{"server_url":%q,"device_code":"dev-test"}`, server.URL)
	pollReq := httptest.NewRequest(http.MethodPost, "/api/dataproxy/device/poll", bytes.NewBufferString(pollBody))
	pollReq.Header.Set("Content-Type", "application/json")
	pollReq.Header.Set("Origin", local.baseURL)
	pollReq.Header.Set("X-CSRF-Token", "test-token")
	pollRec := httptest.NewRecorder()
	handler.ServeHTTP(pollRec, pollReq)
	if pollRec.Code != http.StatusOK {
		t.Fatalf("device/poll status = %d, body=%s", pollRec.Code, pollRec.Body.String())
	}
	if !rotateCalled {
		t.Fatalf("device flow should rotate when ensure omits api_key")
	}
	provider, ok := store.providerByID("dataproxy")
	if !ok {
		t.Fatalf("dataproxy provider was not stored")
	}
	if len(provider.Keys) == 0 || provider.Keys[0].APIKey != "sk-rotated" {
		t.Fatalf("stored provider = %#v", provider)
	}
	if got := store.modelIDs(); !reflect.DeepEqual(got, []string{"rotated-model"}) {
		t.Fatalf("modelIDs() = %#v", got)
	}
}

func TestProviderStoreEnsuresActiveDefaultModel(t *testing.T) {
	store := &providerStore{
		dir:             t.TempDir(),
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
		models:          []string{"gpt-5.5", "gpt-5.4"},
		providers: []upstreamProviderConfig{
			{
				ID:      "dataproxy",
				Name:    "DataProxy",
				Active:  true,
				Enabled: true,
				BaseURL: "https://dp.app.mbu.ltd/v1",
				Keys: []upstreamKeyConfig{
					{ID: "main", Name: "Main", APIKey: "sk-test", Enabled: true, Sort: 10},
				},
			},
		},
	}

	if err := store.ensureActiveDefaultModel(); err != nil {
		t.Fatalf("ensureActiveDefaultModel returned error: %v", err)
	}
	provider, ok := store.activeProvider()
	if !ok {
		t.Fatalf("active provider missing after ensureActiveDefaultModel")
	}
	if provider.DefaultModel != "gpt-5.5" {
		t.Fatalf("DefaultModel = %q, want gpt-5.5", provider.DefaultModel)
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

func TestMergeCodexConfigPreservesPluginSettings(t *testing.T) {
	generated := strings.Join([]string{
		`# Generated by Codex DataProxy. Edit providers in config/*.yaml or /settings.`,
		`model = "gpt-5.5"`,
		`model_provider = "dataproxy-local"`,
		``,
		`model_reasoning_effort = "xhigh"`,
		``,
		`[model_providers.dataproxy-local]`,
		`name = "Codex DataProxy"`,
		`base_url = "http://127.0.0.1:16666/v1"`,
		`requires_openai_auth = true`,
		`wire_api = "responses"`,
		``,
		`[windows]`,
		`sandbox = "unelevated"`,
		``,
	}, "\n")
	existing := strings.Join([]string{
		`model_provider = "custom"`,
		`model = "old-model"`,
		`disable_response_storage = true`,
		`sandbox_mode = "danger-full-access"`,
		``,
		`[model_providers.custom]`,
		`name = "custom"`,
		`base_url = "http://127.0.0.1:15721/v1"`,
		``,
		`[marketplaces.role-specific-plugins]`,
		`last_updated = "2026-06-27T07:48:03Z"`,
		`source_type = "git"`,
		`source = "https://github.com/openai/role-specific-plugins.git"`,
		``,
		`[plugins."product-design@role-specific-plugins"]`,
		`enabled = true`,
		``,
		`[[mcp_servers]]`,
		`name = "docs#server"`,
		`command = "node"`,
		`args = ["https://example.test/schema#latest"]`,
		``,
		`[windows]`,
		`sandbox = "elevated"`,
		``,
	}, "\n")

	got := mergeCodexConfig(generated, existing)
	for _, want := range []string{
		`model_provider = "dataproxy-local"`,
		`model = "gpt-5.5"`,
		`disable_response_storage = true`,
		`sandbox_mode = "danger-full-access"`,
		`[model_providers.custom]`,
		`base_url = "http://127.0.0.1:15721/v1"`,
		`[marketplaces.role-specific-plugins]`,
		`source = "https://github.com/openai/role-specific-plugins.git"`,
		`[plugins."product-design@role-specific-plugins"]`,
		`enabled = true`,
		`[[mcp_servers]]`,
		`name = "docs#server"`,
		`args = ["https://example.test/schema#latest"]`,
		`[windows]`,
		`sandbox = "unelevated"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("mergeCodexConfig() missing %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		`model_provider = "custom"`,
		`model = "old-model"`,
		`sandbox = "elevated"`,
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("mergeCodexConfig() should not include %q in:\n%s", unwanted, got)
		}
	}
}

func TestConfigTomlParsingIgnoresCommentsOutsideStrings(t *testing.T) {
	if got := stripConfigTomlComment(`source = "https://example.test/path#fragment" # comment`); got != `source = "https://example.test/path#fragment" ` {
		t.Fatalf("stripConfigTomlComment basic string = %q", got)
	}
	if got := stripConfigTomlComment(`source = 'literal#value' # comment`); got != `source = 'literal#value' ` {
		t.Fatalf("stripConfigTomlComment literal string = %q", got)
	}
	if name, ok := configTomlSectionName(`[[mcp_servers]] # keep table`); !ok || name != "mcp_servers" {
		t.Fatalf("array table section = %q/%v, want mcp_servers/true", name, ok)
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
	if got["OPENAI_API_KEY"] != localProxyAPIKey {
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
