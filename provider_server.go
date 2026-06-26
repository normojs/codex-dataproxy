package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

type upstreamProviderConfig struct {
	ID           string              `yaml:"id" json:"id"`
	Name         string              `yaml:"name" json:"name"`
	Active       bool                `yaml:"active" json:"active"`
	Enabled      bool                `yaml:"enabled" json:"enabled"`
	Sort         int                 `yaml:"sort" json:"sort"`
	BaseURL      string              `yaml:"base_url" json:"base_url"`
	DefaultModel string              `yaml:"default_model" json:"default_model"`
	Keys         []upstreamKeyConfig `yaml:"keys" json:"keys"`
	path         string
}

type upstreamKeyConfig struct {
	ID      string `yaml:"id" json:"id"`
	Name    string `yaml:"name" json:"name"`
	APIKey  string `yaml:"api_key" json:"api_key"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Sort    int    `yaml:"sort" json:"sort"`
	Default bool   `yaml:"default" json:"default"`
	Models  string `yaml:"models" json:"models"`
}

type modelRoute struct {
	ProviderID string `json:"provider_id"`
	KeyID      string `json:"key_id"`
	Manual     bool   `json:"manual"`
}

type providerStore struct {
	mu         sync.RWMutex
	dir        string
	providers  []upstreamProviderConfig
	autoModels map[string][]string
	models     []string
	routes     map[string]modelRoute
}

type localHTTPServer struct {
	server    *http.Server
	listener  net.Listener
	store     *providerStore
	codexHome string
	appServer *localAppServer
	baseURL   string
}

func newProviderStore(configDir string) (*providerStore, error) {
	store := &providerStore{
		dir:        configDir,
		autoModels: map[string][]string{},
		routes:     map[string]modelRoute{},
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, err
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	if len(store.providers) == 0 {
		if err := store.createDefaultProvider(); err != nil {
			return nil, err
		}
		if err := store.load(); err != nil {
			return nil, err
		}
	}
	store.normalize()
	return store, nil
}

func (s *providerStore) load() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}

	providers := []upstreamProviderConfig{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
			continue
		}
		path := filepath.Join(s.dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var provider upstreamProviderConfig
		if err := yaml.Unmarshal(raw, &provider); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		provider.path = path
		providers = append(providers, provider)
	}
	sortProviders(providers)
	s.mu.Lock()
	s.providers = providers
	s.rebuildRoutesLocked()
	s.mu.Unlock()
	return nil
}

func (s *providerStore) createDefaultProvider() error {
	provider := upstreamProviderConfig{
		ID:           "dataproxy",
		Name:         "DataProxy",
		Active:       true,
		Enabled:      true,
		Sort:         10,
		BaseURL:      "https://dp.app.mbu.ltd/v1",
		DefaultModel: "",
		Keys: []upstreamKeyConfig{
			{
				ID:      "key-1",
				Name:    "",
				APIKey:  "",
				Enabled: true,
				Sort:    10,
				Default: false,
				Models:  "",
			},
		},
	}
	return s.writeProvider(provider)
}

func (s *providerStore) normalize() {
	s.mu.Lock()
	defer s.mu.Unlock()

	sortProviders(s.providers)
	activeSeen := false
	for i := range s.providers {
		s.providers[i].ID = sanitizeID(fallback(s.providers[i].ID, s.providers[i].Name))
		if s.providers[i].ID == "" {
			s.providers[i].ID = fmt.Sprintf("provider-%d", i+1)
		}
		s.providers[i].Name = fallback(strings.TrimSpace(s.providers[i].Name), s.providers[i].ID)
		s.providers[i].BaseURL = strings.TrimSpace(s.providers[i].BaseURL)
		s.providers[i].DefaultModel = strings.TrimSpace(s.providers[i].DefaultModel)
		if s.providers[i].Active && !activeSeen {
			activeSeen = true
		} else {
			s.providers[i].Active = false
		}
		sortKeys(s.providers[i].Keys)
		defaultKeySeen := false
		for j := range s.providers[i].Keys {
			key := &s.providers[i].Keys[j]
			key.ID = sanitizeID(fallback(key.ID, key.Name))
			if key.ID == "" {
				key.ID = fmt.Sprintf("key-%d", j+1)
			}
			key.Name = strings.TrimSpace(key.Name)
			key.APIKey = strings.TrimSpace(key.APIKey)
			key.Models = strings.TrimSpace(key.Models)
			if key.Default && !defaultKeySeen {
				defaultKeySeen = true
			} else {
				key.Default = false
			}
		}
	}
	if !activeSeen && len(s.providers) > 0 {
		s.providers[0].Active = true
	}
	s.rebuildRoutesLocked()
}

func (s *providerStore) writeProvider(provider upstreamProviderConfig) error {
	provider.ID = sanitizeID(fallback(provider.ID, provider.Name))
	if provider.ID == "" {
		provider.ID = "provider"
	}
	path := filepath.Join(s.dir, provider.ID+".yaml")
	provider.path = ""
	raw, err := yaml.Marshal(provider)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func (s *providerStore) upsertProvider(provider upstreamProviderConfig) error {
	s.normalizeProvider(&provider)

	s.mu.Lock()
	replaced := false
	for i := range s.providers {
		if s.providers[i].ID == provider.ID {
			if provider.Active {
				for j := range s.providers {
					s.providers[j].Active = false
				}
			}
			s.providers[i] = provider
			replaced = true
			break
		}
	}
	if !replaced {
		if provider.Active {
			for j := range s.providers {
				s.providers[j].Active = false
			}
		}
		s.providers = append(s.providers, provider)
	}
	sortProviders(s.providers)
	if !hasActiveProvider(s.providers) && len(s.providers) > 0 {
		s.providers[0].Active = true
	}
	providers := append([]upstreamProviderConfig{}, s.providers...)
	s.rebuildRoutesLocked()
	s.mu.Unlock()

	for _, item := range providers {
		if err := s.writeProvider(item); err != nil {
			return err
		}
	}
	return s.load()
}

func (s *providerStore) activateProvider(id string) error {
	s.mu.Lock()
	found := false
	for i := range s.providers {
		active := s.providers[i].ID == id
		s.providers[i].Active = active
		if active {
			found = true
		}
	}
	if !found {
		s.mu.Unlock()
		return fmt.Errorf("provider not found: %s", id)
	}
	providers := append([]upstreamProviderConfig{}, s.providers...)
	s.rebuildRoutesLocked()
	s.mu.Unlock()

	for _, item := range providers {
		if err := s.writeProvider(item); err != nil {
			return err
		}
	}
	return s.load()
}

func (s *providerStore) normalizeProvider(provider *upstreamProviderConfig) {
	provider.ID = sanitizeID(fallback(provider.ID, provider.Name))
	if provider.ID == "" {
		provider.ID = "provider"
	}
	provider.Name = fallback(strings.TrimSpace(provider.Name), provider.ID)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.DefaultModel = strings.TrimSpace(provider.DefaultModel)
	if provider.Sort == 0 {
		provider.Sort = 10
	}
	if len(provider.Keys) == 0 {
		provider.Keys = []upstreamKeyConfig{{ID: "default", Name: "默认 Key", Enabled: true, Sort: 10, Default: true}}
	}
	sortKeys(provider.Keys)
	defaultSeen := false
	for i := range provider.Keys {
		key := &provider.Keys[i]
		key.ID = sanitizeID(fallback(key.ID, key.Name))
		if key.ID == "" {
			key.ID = fmt.Sprintf("key-%d", i+1)
		}
		key.Name = strings.TrimSpace(key.Name)
		key.APIKey = strings.TrimSpace(key.APIKey)
		key.Models = strings.TrimSpace(key.Models)
		if key.Sort == 0 {
			key.Sort = (i + 1) * 10
		}
		if key.Default && !defaultSeen {
			defaultSeen = true
		} else {
			key.Default = false
		}
	}
}

func hasActiveProvider(providers []upstreamProviderConfig) bool {
	for _, provider := range providers {
		if provider.Active {
			return true
		}
	}
	return false
}

func sortProviders(providers []upstreamProviderConfig) {
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Sort == providers[j].Sort {
			return providers[i].Name < providers[j].Name
		}
		return providers[i].Sort < providers[j].Sort
	})
}

func sortKeys(keys []upstreamKeyConfig) {
	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].Sort == keys[j].Sort {
			return keys[i].Name < keys[j].Name
		}
		return keys[i].Sort < keys[j].Sort
	})
}

func sanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func (s *providerStore) activeProvider() (upstreamProviderConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, provider := range s.providers {
		if provider.Active && provider.Enabled {
			return provider, true
		}
	}
	return upstreamProviderConfig{}, false
}

func (s *providerStore) setupProblems() []string {
	provider, ok := s.activeProvider()
	if !ok {
		return []string{"还没有启用的当前中转站"}
	}
	var problems []string
	if strings.TrimSpace(provider.BaseURL) == "" {
		problems = append(problems, "当前中转站 base_url 为空")
	}
	validKey := false
	for _, key := range provider.Keys {
		if !key.Enabled {
			continue
		}
		if key.APIKey != "" && !strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
			validKey = true
			break
		}
	}
	if !validKey {
		problems = append(problems, "当前中转站还没有可用 API Key")
	}
	return problems
}

func (s *providerStore) defaultModel() string {
	provider, ok := s.activeProvider()
	if !ok {
		return ""
	}
	if model := strings.TrimSpace(provider.DefaultModel); model != "" {
		return model
	}
	models := s.modelIDs()
	if len(models) > 0 {
		return models[0]
	}
	return ""
}

func (s *providerStore) ensureActiveDefaultModel() error {
	s.mu.Lock()
	changed := false
	for i := range s.providers {
		if !s.providers[i].Active || !s.providers[i].Enabled {
			continue
		}
		if strings.TrimSpace(s.providers[i].DefaultModel) == "" && len(s.models) > 0 {
			s.providers[i].DefaultModel = s.models[0]
			changed = true
		}
		break
	}
	providers := append([]upstreamProviderConfig{}, s.providers...)
	if changed {
		s.rebuildRoutesLocked()
	}
	s.mu.Unlock()

	if !changed {
		return nil
	}
	for _, item := range providers {
		if err := s.writeProvider(item); err != nil {
			return err
		}
	}
	return s.load()
}

func (s *providerStore) modelIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.models...)
}

func (s *providerStore) rebuildRoutesLocked() {
	routes := map[string]modelRoute{}
	models := []string{}
	index := map[string]int{}

	var active *upstreamProviderConfig
	for i := range s.providers {
		if s.providers[i].Active && s.providers[i].Enabled {
			active = &s.providers[i]
			break
		}
	}
	if active == nil {
		s.routes = routes
		s.models = models
		return
	}

	keys := append([]upstreamKeyConfig{}, active.Keys...)
	sortKeys(keys)
	for _, key := range keys {
		if !key.Enabled {
			continue
		}
		manual := splitModelIDs(key.Models)
		keyModels := manual
		if len(keyModels) == 0 {
			keyModels = s.autoModels[routeKey(active.ID, key.ID)]
		}
		for _, model := range keyModels {
			if model == "" {
				continue
			}
			if pos, ok := index[model]; ok {
				models = append(models[:pos], models[pos+1:]...)
				index = map[string]int{}
				for i, value := range models {
					index[value] = i
				}
			}
			index[model] = len(models)
			models = append(models, model)
			routes[model] = modelRoute{ProviderID: active.ID, KeyID: key.ID, Manual: len(manual) > 0}
		}
	}
	s.routes = routes
	s.models = models
}

func routeKey(providerID string, keyID string) string {
	return providerID + "/" + keyID
}

func (s *providerStore) refreshActiveModels(ctx context.Context) error {
	provider, ok := s.activeProvider()
	if !ok {
		return errors.New("active provider is not configured")
	}
	endpoint, err := modelsEndpoint(provider.BaseURL)
	if err != nil {
		return err
	}

	autoModels := map[string][]string{}
	var firstErr error
	for _, key := range provider.Keys {
		if !key.Enabled || len(splitModelIDs(key.Models)) > 0 {
			continue
		}
		if key.APIKey == "" || strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
			continue
		}
		models, err := fetchModelIDsWithKey(ctx, endpoint, key.APIKey)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		autoModels[routeKey(provider.ID, key.ID)] = models
	}

	s.mu.Lock()
	for key, value := range autoModels {
		s.autoModels[key] = value
	}
	s.rebuildRoutesLocked()
	s.mu.Unlock()
	return firstErr
}

func fetchModelIDsWithKey(ctx context.Context, endpoint string, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

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

func (s *providerStore) keyForModel(model string) (upstreamProviderConfig, upstreamKeyConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var active *upstreamProviderConfig
	for i := range s.providers {
		if s.providers[i].Active && s.providers[i].Enabled {
			active = &s.providers[i]
			break
		}
	}
	if active == nil {
		return upstreamProviderConfig{}, upstreamKeyConfig{}, false
	}

	if model != "" {
		if route, ok := s.routes[model]; ok {
			for _, key := range active.Keys {
				if key.ID == route.KeyID && key.Enabled && key.APIKey != "" && !strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
					return *active, key, true
				}
			}
		}
	}

	keys := append([]upstreamKeyConfig{}, active.Keys...)
	sortKeys(keys)
	for _, key := range keys {
		if key.Default && key.Enabled && key.APIKey != "" && !strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
			return *active, key, true
		}
	}
	for _, key := range keys {
		if key.Enabled && key.APIKey != "" && !strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
			return *active, key, true
		}
	}
	return upstreamProviderConfig{}, upstreamKeyConfig{}, false
}

func startLocalHTTPServer(store *providerStore, codexHome string, appServer *localAppServer) (*localHTTPServer, error) {
	var listener net.Listener
	var err error
	for port := 16666; port < 16720; port++ {
		listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			break
		}
	}
	if listener == nil {
		return nil, fmt.Errorf("cannot listen on local settings port: %w", err)
	}

	local := &localHTTPServer{
		listener:  listener,
		store:     store,
		codexHome: codexHome,
		appServer: appServer,
		baseURL:   "http://" + listener.Addr().String(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/settings", local.handleSettings)
	mux.HandleFunc("/api/providers", local.handleProviders)
	mux.HandleFunc("/api/providers/activate", local.handleActivateProvider)
	mux.HandleFunc("/api/models/refresh", local.handleRefreshModels)
	mux.HandleFunc("/v1/", local.handleProxy)
	local.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		_ = local.server.Serve(listener)
	}()
	return local, nil
}

func (s *localHTTPServer) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
}

func (s *localHTTPServer) proxyBaseURL() string {
	return s.baseURL + "/v1"
}

func (s *localHTTPServer) settingsURL() string {
	return s.baseURL + "/settings"
}

func (s *localHTTPServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.ReplaceAll(settingsPageHTML, "{{BASE_URL}}", s.baseURL)
	_, _ = io.WriteString(w, page)
}

func (s *localHTTPServer) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		var provider upstreamProviderConfig
		if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&provider); err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := s.store.upsertProvider(provider); err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		_ = s.store.refreshActiveModels(ctx)
		cancel()
		_ = s.store.ensureActiveDefaultModel()
		_ = syncDynamicModelList(s.codexHome)
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.store.mu.RLock()
	providers := append([]upstreamProviderConfig{}, s.store.providers...)
	models := append([]string{}, s.store.models...)
	routes := make(map[string]modelRoute, len(s.store.routes))
	for key, value := range s.store.routes {
		routes[key] = value
	}
	s.store.mu.RUnlock()
	writeJSON(w, map[string]any{
		"providers":  providers,
		"models":     models,
		"routes":     routes,
		"proxy_url":  s.proxyBaseURL(),
		"app_server": s.appServerStatus(),
	})
}

func (s *localHTTPServer) appServerStatus() appServerStatus {
	if s == nil || s.appServer == nil {
		return appServerStatus{Enabled: false, Status: appServerStatusDisabled}
	}
	return s.appServer.snapshot()
}

func (s *localHTTPServer) handleActivateProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.activateProvider(strings.TrimSpace(payload.ID)); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	_ = s.store.refreshActiveModels(ctx)
	cancel()
	_ = s.store.ensureActiveDefaultModel()
	_ = syncDynamicModelList(s.codexHome)
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *localHTTPServer) handleRefreshModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	err := s.store.refreshActiveModels(ctx)
	_ = s.store.ensureActiveDefaultModel()
	result := syncDynamicModelList(s.codexHome)
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "models": result.Models})
		return
	}
	writeJSON(w, map[string]any{"models": result.Models})
}

func (s *localHTTPServer) handleProxy(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v1/models" && r.Method == http.MethodGet {
		s.handleMergedModels(w)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = r.Body.Close()
	model := requestModel(body)
	provider, key, ok := s.store.keyForModel(model)
	if !ok {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{
			"error": "no available API key for model " + fallback(model, "(default)"),
		})
		return
	}

	target, err := proxyTargetURL(provider.BaseURL, r.URL)
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	copyProxyHeaders(req.Header, r.Header)
	req.Header.Set("Authorization", "Bearer "+key.APIKey)

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (s *localHTTPServer) handleMergedModels(w http.ResponseWriter) {
	models := s.store.modelIDs()
	data := make([]map[string]string, 0, len(models))
	for _, model := range models {
		data = append(data, map[string]string{"id": model, "object": "model"})
	}
	writeJSON(w, map[string]any{"data": data})
}

func requestModel(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Model)
}

func proxyTargetURL(baseURL string, requestURL *url.URL) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base_url is not absolute: %s", baseURL)
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	suffix := strings.TrimPrefix(requestURL.Path, "/v1")
	parsed.Path = basePath + suffix
	parsed.RawQuery = requestURL.RawQuery
	return parsed.String(), nil
}

func copyProxyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if isHopByHopHeader(key) || strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "Host") {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func copyResponseHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	writeJSONStatus(w, http.StatusOK, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
