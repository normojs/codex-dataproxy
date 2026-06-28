package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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
	ModelAliases map[string]string   `yaml:"model_aliases,omitempty" json:"model_aliases,omitempty"`
	Keys         []upstreamKeyConfig `yaml:"keys" json:"keys"`
	path         string
}

type upstreamKeyConfig struct {
	ID        string `yaml:"id" json:"id"`
	Name      string `yaml:"name" json:"name"`
	APIKey    string `yaml:"api_key" json:"api_key"`
	APIKeySet bool   `yaml:"-" json:"api_key_set,omitempty"`
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Sort      int    `yaml:"sort" json:"sort"`
	Default   bool   `yaml:"default" json:"default"`
	Models    string `yaml:"models" json:"models"`
}

type modelRoute struct {
	ProviderID string `json:"provider_id"`
	KeyID      string `json:"key_id"`
	Manual     bool   `json:"manual"`
}

type keyModelStatus struct {
	ProviderID    string `json:"provider_id"`
	KeyID         string `json:"key_id"`
	Status        string `json:"status"`
	HTTPStatus    string `json:"http_status,omitempty"`
	ModelCount    int    `json:"model_count"`
	LatencyMS     int64  `json:"latency_ms,omitempty"`
	LastRefreshAt string `json:"last_refresh_at,omitempty"`
	Error         string `json:"error,omitempty"`
}

type providerTestResult struct {
	ProviderID string           `json:"provider_id"`
	OK         bool             `json:"ok"`
	Results    []keyModelStatus `json:"results"`
	Error      string           `json:"error,omitempty"`
}

type modelPatternRoute struct {
	Pattern string
	Route   modelRoute
}

const defaultDataProxyServerURL = "https://dp.app.mbu.ltd"
const maxProxyRequestBodyBytes int64 = 256 << 20

var upstreamRequestTimeout = 20 * time.Second

type dataProxyDeviceStartRequest struct {
	ServerURL  string `json:"server_url"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Locale     string `json:"locale"`
}

type dataProxyDeviceStartResponse struct {
	DeviceID                string `json:"device_id"`
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	ServerURL               string `json:"server_url"`
}

type dataProxyDevicePollRequest struct {
	ServerURL  string `json:"server_url"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	DeviceCode string `json:"device_code"`
	Group      string `json:"group"`
}

type dataProxyDevicePollResponse struct {
	Status                   string         `json:"status"`
	Interval                 int            `json:"interval,omitempty"`
	ManagementToken          string         `json:"management_token,omitempty"`
	ManagementTokenExpiresAt int64          `json:"management_token_expires_at,omitempty"`
	ServerURL                string         `json:"server_url,omitempty"`
	BaseURL                  string         `json:"base_url,omitempty"`
	User                     map[string]any `json:"user,omitempty"`
	Capabilities             map[string]any `json:"capabilities,omitempty"`
	Selected                 bool           `json:"selected,omitempty"`
	ProviderID               string         `json:"provider_id,omitempty"`
	KeyID                    string         `json:"key_id,omitempty"`
	Error                    string         `json:"error,omitempty"`
}

type dataProxyEnsureTokenResponse struct {
	Selected   bool   `json:"selected"`
	Created    bool   `json:"created"`
	Rotated    bool   `json:"rotated"`
	APIKeyOnce bool   `json:"api_key_once"`
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Token      struct {
		ID        any    `json:"id"`
		Name      string `json:"name"`
		MaskedKey string `json:"masked_key"`
		Group     string `json:"group"`
	} `json:"token"`
}

type providerStore struct {
	mu              sync.RWMutex
	dir             string
	providers       []upstreamProviderConfig
	autoModels      map[string][]string
	models          []string
	routes          map[string]modelRoute
	duplicateRoutes map[string][]modelRoute
	patternRoutes   []modelPatternRoute
	keyStatus       map[string]keyModelStatus
}

type localHTTPServer struct {
	server    *http.Server
	listener  net.Listener
	store     *providerStore
	codexHome string
	appServer *localAppServer
	baseURL   string
	csrfToken string
}

func newProviderStore(configDir string) (*providerStore, error) {
	store := &providerStore{
		dir:             configDir,
		autoModels:      map[string][]string{},
		routes:          map[string]modelRoute{},
		duplicateRoutes: map[string][]modelRoute{},
		keyStatus:       map[string]keyModelStatus{},
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
		s.providers[i].ModelAliases = normalizeModelAliases(s.providers[i].ModelAliases)
		if s.providers[i].Active && !activeSeen {
			activeSeen = true
		} else {
			s.providers[i].Active = false
		}
		sortKeys(s.providers[i].Keys)
		defaultKeySeen := false
		seenKeyIDs := map[string]bool{}
		for j := range s.providers[i].Keys {
			key := &s.providers[i].Keys[j]
			key.ID = uniqueKeyID(key.ID, key.Name, j+1, seenKeyIDs)
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
	if strings.TrimSpace(provider.BaseURL) != "" {
		if err := validateAPIBaseURL(provider.BaseURL); err != nil {
			return err
		}
	}

	s.mu.Lock()
	replaced := false
	for i := range s.providers {
		if s.providers[i].ID == provider.ID {
			preserveProviderSecrets(&provider, s.providers[i])
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

func (s *providerStore) deleteProvider(id string) error {
	id = sanitizeID(id)
	if id == "" {
		return errors.New("provider id is required")
	}

	s.mu.Lock()
	index := -1
	var removed upstreamProviderConfig
	for i := range s.providers {
		if s.providers[i].ID == id {
			index = i
			removed = s.providers[i]
			break
		}
	}
	if index < 0 {
		s.mu.Unlock()
		return fmt.Errorf("provider not found: %s", id)
	}

	wasActive := s.providers[index].Active
	s.providers = append(s.providers[:index], s.providers[index+1:]...)
	if wasActive && len(s.providers) > 0 {
		sortProviders(s.providers)
		for i := range s.providers {
			s.providers[i].Active = i == 0
		}
	}
	providers := append([]upstreamProviderConfig{}, s.providers...)
	for key := range s.autoModels {
		if strings.HasPrefix(key, id+"/") {
			delete(s.autoModels, key)
		}
	}
	for key := range s.keyStatus {
		if strings.HasPrefix(key, id+"/") {
			delete(s.keyStatus, key)
		}
	}
	s.rebuildRoutesLocked()
	s.mu.Unlock()

	path := removed.path
	if path == "" {
		path = filepath.Join(s.dir, id+".yaml")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
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

func (s *providerStore) reorderProviders(ids []string) error {
	if len(ids) == 0 {
		return errors.New("provider order is empty")
	}
	order := map[string]int{}
	for i, id := range ids {
		clean := sanitizeID(id)
		if clean != "" {
			order[clean] = (i + 1) * 10
		}
	}

	s.mu.Lock()
	changed := false
	tailSort := (len(order) + 1) * 10
	for i := range s.providers {
		if sortValue, ok := order[s.providers[i].ID]; ok {
			if s.providers[i].Sort != sortValue {
				changed = true
			}
			s.providers[i].Sort = sortValue
			continue
		}
		s.providers[i].Sort = tailSort
		tailSort += 10
		changed = true
	}
	sortProviders(s.providers)
	providers := append([]upstreamProviderConfig{}, s.providers...)
	if changed {
		s.rebuildRoutesLocked()
	}
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
	provider.ModelAliases = normalizeModelAliases(provider.ModelAliases)
	if provider.Sort == 0 {
		provider.Sort = 10
	}
	if len(provider.Keys) == 0 {
		provider.Keys = []upstreamKeyConfig{{ID: "default", Name: "默认 Key", Enabled: true, Sort: 10, Default: true}}
	}
	sortKeys(provider.Keys)
	defaultSeen := false
	seenKeyIDs := map[string]bool{}
	for i := range provider.Keys {
		key := &provider.Keys[i]
		key.ID = uniqueKeyID(key.ID, key.Name, i+1, seenKeyIDs)
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

func uniqueKeyID(id string, name string, index int, seen map[string]bool) string {
	base := sanitizeID(fallback(id, name))
	if base == "" {
		base = fmt.Sprintf("key-%d", index)
	}
	candidate := base
	for suffix := 2; seen[candidate]; suffix++ {
		candidate = fmt.Sprintf("%s-%d", base, suffix)
	}
	seen[candidate] = true
	return candidate
}

func normalizeModelAliases(aliases map[string]string) map[string]string {
	if len(aliases) == 0 {
		return nil
	}
	normalized := map[string]string{}
	for source, target := range aliases {
		source = strings.TrimSpace(source)
		target = strings.TrimSpace(target)
		if source != "" && target != "" {
			normalized[source] = target
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func preserveProviderSecrets(next *upstreamProviderConfig, existing upstreamProviderConfig) {
	existingKeys := map[string]upstreamKeyConfig{}
	for _, key := range existing.Keys {
		existingKeys[key.ID] = key
	}
	for i := range next.Keys {
		if next.Keys[i].APIKey == "" || isMaskedAPIKey(next.Keys[i].APIKey) {
			if existing, ok := existingKeys[next.Keys[i].ID]; ok {
				next.Keys[i].APIKey = existing.APIKey
			}
		}
	}
}

func redactProviders(providers []upstreamProviderConfig) []upstreamProviderConfig {
	redacted := make([]upstreamProviderConfig, len(providers))
	for i, provider := range providers {
		redacted[i] = provider
		redacted[i].path = ""
		redacted[i].Keys = make([]upstreamKeyConfig, len(provider.Keys))
		for j, key := range provider.Keys {
			redacted[i].Keys[j] = key
			redacted[i].Keys[j].APIKeySet = key.APIKey != "" && !strings.EqualFold(key.APIKey, apiKeyPlaceholder)
			redacted[i].Keys[j].APIKey = maskAPIKey(key.APIKey)
		}
	}
	return redacted
}

func maskAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, apiKeyPlaceholder) {
		return ""
	}
	if len(value) <= 10 {
		return "sk-***"
	}
	return value[:3] + "***" + value[len(value)-4:]
}

func isMaskedAPIKey(value string) bool {
	return strings.Contains(strings.TrimSpace(value), "***")
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

func (s *providerStore) providerByID(id string) (upstreamProviderConfig, bool) {
	id = sanitizeID(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, provider := range s.providers {
		if provider.ID == id {
			return provider, true
		}
	}
	return upstreamProviderConfig{}, false
}

func (s *providerStore) providerForTest(id string, candidate upstreamProviderConfig) (upstreamProviderConfig, error) {
	if candidate.ID == "" && strings.TrimSpace(candidate.Name) == "" && strings.TrimSpace(candidate.BaseURL) == "" {
		provider, ok := s.providerByID(id)
		if !ok {
			return upstreamProviderConfig{}, fmt.Errorf("provider not found: %s", id)
		}
		return provider, nil
	}
	s.normalizeProvider(&candidate)
	if existing, ok := s.providerByID(candidate.ID); ok {
		preserveProviderSecrets(&candidate, existing)
	}
	return candidate, nil
}

func (s *providerStore) testProvider(ctx context.Context, provider upstreamProviderConfig, keyID string) providerTestResult {
	result := providerTestResult{ProviderID: provider.ID}
	endpoint, err := modelsEndpoint(provider.BaseURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	statuses := []keyModelStatus{}
	for _, key := range provider.Keys {
		if keyID != "" && key.ID != keyID {
			continue
		}
		statusKey := routeKey(provider.ID, key.ID)
		status := keyModelStatus{
			ProviderID:    provider.ID,
			KeyID:         key.ID,
			Status:        "skipped",
			LastRefreshAt: time.Now().Format(time.RFC3339),
		}
		switch {
		case !key.Enabled:
			status.Error = "key is disabled"
		case key.APIKey == "" || strings.EqualFold(key.APIKey, apiKeyPlaceholder) || isMaskedAPIKey(key.APIKey):
			status.Error = "api key is empty"
		default:
			_, status = fetchModelIDsWithKeyStatus(ctx, endpoint, provider.ID, key.ID, key.APIKey)
		}
		statuses = append(statuses, status)
		s.mu.Lock()
		s.keyStatus[statusKey] = status
		s.mu.Unlock()
	}
	result.Results = statuses
	for _, status := range statuses {
		if status.Status == "ok" {
			result.OK = true
			return result
		}
		if result.Error == "" && status.Error != "" {
			result.Error = status.Error
		}
	}
	if result.Error == "" {
		result.Error = "no key was tested"
	}
	return result
}

func (s *providerStore) applyDataProxyConnectedToken(baseURL string, apiKey string) (string, string, error) {
	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" {
		return "", "", errors.New("base_url is required")
	}
	if err := validateAPIBaseURL(baseURL); err != nil {
		return "", "", err
	}
	if apiKey == "" {
		return "", "", errors.New("api key is required")
	}

	provider := upstreamProviderConfig{
		ID:      "dataproxy",
		Name:    "DataProxy",
		Active:  true,
		Enabled: true,
		Sort:    10,
		BaseURL: baseURL,
		Keys: []upstreamKeyConfig{
			{
				ID:      "connected-app",
				Name:    "DataProxy Connected App",
				APIKey:  apiKey,
				Enabled: true,
				Sort:    10,
				Default: true,
			},
		},
	}
	if existing, ok := s.providerByID(provider.ID); ok {
		provider.Name = fallback(strings.TrimSpace(existing.Name), provider.Name)
		provider.DefaultModel = existing.DefaultModel
		provider.ModelAliases = existing.ModelAliases
		provider.Sort = existing.Sort
		provider.Active = true
		keys := []upstreamKeyConfig{provider.Keys[0]}
		for _, key := range existing.Keys {
			if key.ID == "connected-app" {
				continue
			}
			keys = append(keys, key)
		}
		provider.Keys = keys
	}
	if err := s.upsertProvider(provider); err != nil {
		return "", "", err
	}
	return provider.ID, "connected-app", nil
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
	duplicates := map[string][]modelRoute{}
	models := []string{}
	index := map[string]int{}
	patterns := []modelPatternRoute{}

	var active *upstreamProviderConfig
	for i := range s.providers {
		if s.providers[i].Active && s.providers[i].Enabled {
			active = &s.providers[i]
			break
		}
	}
	if active == nil {
		s.routes = routes
		s.duplicateRoutes = duplicates
		s.models = models
		s.patternRoutes = patterns
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
			route := modelRoute{ProviderID: active.ID, KeyID: key.ID, Manual: len(manual) > 0}
			if strings.Contains(model, "*") {
				patterns = append(patterns, modelPatternRoute{Pattern: model, Route: route})
				continue
			}
			if pos, ok := index[model]; ok {
				if len(duplicates[model]) == 0 {
					duplicates[model] = append(duplicates[model], routes[model])
				}
				models = append(models[:pos], models[pos+1:]...)
				index = map[string]int{}
				for i, value := range models {
					index[value] = i
				}
			}
			index[model] = len(models)
			models = append(models, model)
			routes[model] = route
			if len(duplicates[model]) > 0 {
				duplicates[model] = append(duplicates[model], route)
			}
		}
	}
	s.routes = routes
	s.duplicateRoutes = duplicates
	s.models = models
	s.patternRoutes = patterns
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
	statuses := map[string]keyModelStatus{}
	var firstErr error
	for _, key := range provider.Keys {
		statusKey := routeKey(provider.ID, key.ID)
		manualModels := splitModelIDs(key.Models)
		if !key.Enabled {
			statuses[statusKey] = keyModelStatus{
				ProviderID:    provider.ID,
				KeyID:         key.ID,
				Status:        "skipped",
				LastRefreshAt: time.Now().Format(time.RFC3339),
				Error:         "key is disabled",
			}
			continue
		}
		if len(manualModels) > 0 {
			statuses[statusKey] = keyModelStatus{
				ProviderID:    provider.ID,
				KeyID:         key.ID,
				Status:        "manual",
				ModelCount:    len(manualModels),
				LastRefreshAt: time.Now().Format(time.RFC3339),
			}
			continue
		}
		if key.APIKey == "" || strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
			statuses[statusKey] = keyModelStatus{
				ProviderID:    provider.ID,
				KeyID:         key.ID,
				Status:        "skipped",
				LastRefreshAt: time.Now().Format(time.RFC3339),
				Error:         "api key is empty",
			}
			continue
		}
		models, status := fetchModelIDsWithKeyStatus(ctx, endpoint, provider.ID, key.ID, key.APIKey)
		statuses[statusKey] = status
		if status.Status != "ok" {
			err := errors.New(status.Error)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		autoModels[routeKey(provider.ID, key.ID)] = models
	}

	s.mu.Lock()
	providerPrefix := provider.ID + "/"
	for key := range s.autoModels {
		if strings.HasPrefix(key, providerPrefix) {
			delete(s.autoModels, key)
		}
	}
	for key := range s.keyStatus {
		if strings.HasPrefix(key, providerPrefix) {
			delete(s.keyStatus, key)
		}
	}
	for key, value := range autoModels {
		s.autoModels[key] = value
	}
	now := time.Now().Format(time.RFC3339)
	for key, value := range statuses {
		if value.LastRefreshAt == "" {
			value.LastRefreshAt = now
		}
		s.keyStatus[key] = value
	}
	s.rebuildRoutesLocked()
	s.mu.Unlock()
	return firstErr
}

func fetchModelIDsWithKey(ctx context.Context, endpoint string, apiKey string) ([]string, error) {
	models, status := fetchModelIDsWithKeyStatus(ctx, endpoint, "", "", apiKey)
	if status.Status != "ok" {
		return nil, errors.New(status.Error)
	}
	return models, nil
}

func fetchModelIDsWithKeyStatus(ctx context.Context, endpoint string, providerID string, keyID string, apiKey string) ([]string, keyModelStatus) {
	start := time.Now()
	status := keyModelStatus{
		ProviderID:    providerID,
		KeyID:         keyID,
		Status:        "failed",
		LastRefreshAt: time.Now().Format(time.RFC3339),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		status.Error = err.Error()
		status.LatencyMS = time.Since(start).Milliseconds()
		return nil, status
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		status.Error = err.Error()
		status.LatencyMS = time.Since(start).Milliseconds()
		return nil, status
	}
	defer resp.Body.Close()
	status.HTTPStatus = resp.Status

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		status.Error = err.Error()
		status.LatencyMS = time.Since(start).Milliseconds()
		return nil, status
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status.Error = fmt.Sprintf("GET %s returned %s", endpoint, resp.Status)
		status.LatencyMS = time.Since(start).Milliseconds()
		return nil, status
	}

	var parsed openAIModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		status.Error = err.Error()
		status.LatencyMS = time.Since(start).Milliseconds()
		return nil, status
	}
	models := make([]string, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if id := modelIDFromRawJSON(item); id != "" {
			models = append(models, id)
		}
	}
	status.Status = "ok"
	status.ModelCount = len(models)
	status.LatencyMS = time.Since(start).Milliseconds()
	return models, status
}

func (s *providerStore) keyForModel(model string) (upstreamProviderConfig, upstreamKeyConfig, bool) {
	provider, key, _, ok := s.keyForModelWithTarget(model)
	return provider, key, ok
}

func (s *providerStore) keyForModelWithTarget(model string) (upstreamProviderConfig, upstreamKeyConfig, string, bool) {
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
		return upstreamProviderConfig{}, upstreamKeyConfig{}, "", false
	}

	targetModel := strings.TrimSpace(model)
	if targetModel != "" {
		if alias, ok := active.ModelAliases[targetModel]; ok && strings.TrimSpace(alias) != "" {
			targetModel = strings.TrimSpace(alias)
		}
	}

	if model != "" {
		if route, ok := s.routes[targetModel]; ok {
			for _, key := range active.Keys {
				if key.ID == route.KeyID && key.Enabled && key.APIKey != "" && !strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
					return *active, key, targetModel, true
				}
			}
		}
		for _, pattern := range s.patternRoutes {
			if modelPatternMatches(pattern.Pattern, targetModel) {
				for _, key := range active.Keys {
					if key.ID == pattern.Route.KeyID && key.Enabled && key.APIKey != "" && !strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
						return *active, key, targetModel, true
					}
				}
			}
		}
	}

	keys := append([]upstreamKeyConfig{}, active.Keys...)
	sortKeys(keys)
	for _, key := range keys {
		if key.Default && key.Enabled && key.APIKey != "" && !strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
			return *active, key, targetModel, true
		}
	}
	for _, key := range keys {
		if key.Enabled && key.APIKey != "" && !strings.EqualFold(key.APIKey, apiKeyPlaceholder) {
			return *active, key, targetModel, true
		}
	}
	return upstreamProviderConfig{}, upstreamKeyConfig{}, "", false
}

func modelPatternMatches(pattern string, model string) bool {
	pattern = strings.TrimSpace(pattern)
	model = strings.TrimSpace(model)
	if pattern == "" || model == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == model
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		return strings.HasPrefix(model, parts[0]) && strings.HasSuffix(model, parts[1])
	}
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		found := strings.Index(model[pos:], part)
		if found < 0 {
			return false
		}
		if i == 0 && found != 0 {
			return false
		}
		pos += found + len(part)
	}
	last := parts[len(parts)-1]
	return last == "" || strings.HasSuffix(model, last)
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
		csrfToken: newCSRFToken(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/settings", local.handleSettings)
	mux.HandleFunc("/api/providers", local.handleProviders)
	mux.HandleFunc("/api/providers/activate", local.handleActivateProvider)
	mux.HandleFunc("/api/providers/delete", local.handleDeleteProvider)
	mux.HandleFunc("/api/providers/reorder", local.handleReorderProviders)
	mux.HandleFunc("/api/providers/test", local.handleTestProvider)
	mux.HandleFunc("/api/models/refresh", local.handleRefreshModels)
	mux.HandleFunc("/api/dataproxy/device/start", local.handleDataProxyDeviceStart)
	mux.HandleFunc("/api/dataproxy/device/poll", local.handleDataProxyDevicePoll)
	mux.HandleFunc("/v1/", local.handleProxy)
	local.server = &http.Server{Handler: local.guard(mux), ReadHeaderTimeout: 10 * time.Second}

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

func newCSRFToken() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (s *localHTTPServer) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !s.validSameOrigin(r) {
				writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "cross-origin request rejected"})
				return
			}
			if r.Header.Get("X-CSRF-Token") != s.csrfToken {
				writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "invalid csrf token"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *localHTTPServer) validSameOrigin(r *http.Request) bool {
	for _, header := range []string{"Origin", "Referer"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil {
			return false
		}
		origin := parsed.Scheme + "://" + parsed.Host
		if origin != s.baseURL {
			return false
		}
	}
	return true
}

func (s *localHTTPServer) ensureDeviceID(candidate string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	path := filepath.Join(s.codexHome, deviceIDFileName)
	if candidate != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(candidate), 0o600); err != nil {
			return "", err
		}
		return candidate, nil
	}
	if raw, err := os.ReadFile(path); err == nil {
		if value := strings.TrimSpace(string(raw)); value != "" {
			return value, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	deviceID := "codex-dp-" + base64.RawURLEncoding.EncodeToString(raw)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(deviceID), 0o600); err != nil {
		return "", err
	}
	return deviceID, nil
}

func normalizeDataProxyServerURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultDataProxyServerURL
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return defaultDataProxyServerURL
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(parsed.Path, "/v1") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/v1")
	}
	return strings.TrimRight(parsed.String(), "/")
}

func defaultDeviceName() string {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		return "Codex DataProxy"
	}
	return name
}

func upstreamContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok || upstreamRequestTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, upstreamRequestTimeout)
}

func postJSON(ctx context.Context, endpoint string, bearerToken string, request any, response any) error {
	ctx, cancel := upstreamContext(ctx)
	defer cancel()

	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(bearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s returned %s: %s", endpoint, resp.Status, summarizeHTTPError(raw))
	}
	if response == nil {
		return nil
	}
	if err := json.Unmarshal(raw, response); err != nil {
		return err
	}
	return nil
}

func summarizeHTTPError(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if len(text) > 300 {
		text = text[:300]
	}
	return text
}

func ensureDataProxyConnectedToken(ctx context.Context, serverURL string, managementToken string, request map[string]any) (dataProxyEnsureTokenResponse, error) {
	var token dataProxyEnsureTokenResponse
	if err := postJSON(ctx, serverURL+"/api/connected-apps/codex-dp/tokens/ensure", managementToken, request, &token); err != nil {
		return token, err
	}
	if strings.TrimSpace(token.APIKey) != "" {
		return token, nil
	}

	rotated, err := rotateDataProxyConnectedToken(ctx, serverURL, managementToken, request, token)
	if err != nil {
		return token, err
	}
	if strings.TrimSpace(rotated.APIKey) == "" {
		return rotated, errors.New("token rotation did not return api_key")
	}
	return rotated, nil
}

func rotateDataProxyConnectedToken(ctx context.Context, serverURL string, managementToken string, request map[string]any, token dataProxyEnsureTokenResponse) (dataProxyEnsureTokenResponse, error) {
	tokenID := dataProxyTokenID(token.Token.ID)
	var rotateErr error
	if tokenID != "" {
		var rotated dataProxyEnsureTokenResponse
		endpoint := serverURL + "/api/connected-apps/codex-dp/tokens/" + url.PathEscape(tokenID) + "/rotate"
		if err := postJSON(ctx, endpoint, managementToken, map[string]any{}, &rotated); err == nil {
			return rotated, nil
		} else {
			rotateErr = err
		}
	}

	rotatingRequest := copyMap(request)
	rotatingRequest["rotate"] = true
	var rotated dataProxyEnsureTokenResponse
	err := postJSON(ctx, serverURL+"/api/connected-apps/codex-dp/tokens/ensure", managementToken, rotatingRequest, &rotated)
	if err != nil && rotateErr != nil {
		return rotated, fmt.Errorf("token rotate failed: %v; ensure rotate failed: %w", rotateErr, err)
	}
	return rotated, err
}

func dataProxyTokenID(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case float32:
		return strings.TrimSpace(strconv.FormatFloat(float64(v), 'f', -1, 32))
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func copyMap(values map[string]any) map[string]any {
	copied := make(map[string]any, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func (s *localHTTPServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.ReplaceAll(settingsPageHTML, "{{BASE_URL}}", s.baseURL)
	page = strings.ReplaceAll(page, "{{CSRF_TOKEN}}", s.csrfToken)
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
	duplicates := make(map[string][]modelRoute, len(s.store.duplicateRoutes))
	for key, value := range s.store.duplicateRoutes {
		duplicates[key] = append([]modelRoute{}, value...)
	}
	keyStatus := make(map[string]keyModelStatus, len(s.store.keyStatus))
	for key, value := range s.store.keyStatus {
		keyStatus[key] = value
	}
	s.store.mu.RUnlock()
	writeJSON(w, map[string]any{
		"providers":        redactProviders(providers),
		"models":           models,
		"routes":           routes,
		"duplicate_routes": duplicates,
		"key_status":       keyStatus,
		"proxy_url":        s.proxyBaseURL(),
		"app_server":       s.appServerStatus(),
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

func (s *localHTTPServer) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
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
	if err := s.store.deleteProvider(payload.ID); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.ensureActiveDefaultModel()
	_ = syncDynamicModelList(s.codexHome)
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *localHTTPServer) handleReorderProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.reorderProviders(payload.IDs); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *localHTTPServer) handleTestProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		ID       string                 `json:"id"`
		KeyID    string                 `json:"key_id"`
		Provider upstreamProviderConfig `json:"provider"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&payload); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	provider, err := s.store.providerForTest(payload.ID, payload.Provider)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result := s.store.testProvider(ctx, provider, strings.TrimSpace(payload.KeyID))
	status := http.StatusOK
	if !result.OK {
		status = http.StatusBadGateway
	}
	writeJSONStatus(w, status, result)
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

func (s *localHTTPServer) handleDataProxyDeviceStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload dataProxyDeviceStartRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	deviceID, err := s.ensureDeviceID(payload.DeviceID)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	serverURL := normalizeDataProxyServerURL(payload.ServerURL)
	deviceName := fallback(strings.TrimSpace(payload.DeviceName), defaultDeviceName())
	request := map[string]any{
		"device_id":   deviceID,
		"device_name": deviceName,
		"platform":    runtime.GOOS,
		"app_version": "0.2.1",
		"client":      "codex-dp",
		"locale":      fallback(strings.TrimSpace(payload.Locale), "zh-CN"),
	}
	var upstream dataProxyDeviceStartResponse
	if err := postJSON(r.Context(), serverURL+"/api/connected-apps/codex-dp/device/start", "", request, &upstream); err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	upstream.DeviceID = deviceID
	upstream.ServerURL = serverURL
	writeJSON(w, upstream)
}

func (s *localHTTPServer) handleDataProxyDevicePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload dataProxyDevicePollRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(payload.DeviceCode) == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "device_code is required"})
		return
	}
	deviceID, err := s.ensureDeviceID(payload.DeviceID)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	serverURL := normalizeDataProxyServerURL(payload.ServerURL)
	var poll dataProxyDevicePollResponse
	if err := postJSON(r.Context(), serverURL+"/api/connected-apps/codex-dp/device/poll", "", map[string]string{"device_code": payload.DeviceCode}, &poll); err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if poll.Status != "authorized" {
		if poll.Interval == 0 {
			poll.Interval = 3
		}
		writeJSON(w, poll)
		return
	}
	if strings.TrimSpace(poll.ManagementToken) == "" {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": "authorized response did not include management_token"})
		return
	}
	if poll.ServerURL != "" {
		serverURL = normalizeDataProxyServerURL(poll.ServerURL)
	}
	baseURL := fallback(strings.TrimSpace(poll.BaseURL), serverURL+"/v1")
	deviceName := fallback(strings.TrimSpace(payload.DeviceName), defaultDeviceName())
	tokenRequest := map[string]any{
		"device_id":   deviceID,
		"device_name": deviceName,
		"platform":    runtime.GOOS,
		"app_version": "0.2.1",
		"group":       strings.TrimSpace(payload.Group),
		"rotate":      false,
	}
	token, err := ensureDataProxyConnectedToken(r.Context(), serverURL, poll.ManagementToken, tokenRequest)
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if token.BaseURL != "" {
		baseURL = strings.TrimSpace(token.BaseURL)
	}
	providerID, keyID, err := s.store.applyDataProxyConnectedToken(baseURL, token.APIKey)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	_ = s.store.refreshActiveModels(ctx)
	cancel()
	_ = s.store.ensureActiveDefaultModel()
	_ = syncDynamicModelList(s.codexHome)
	poll.ManagementToken = ""
	poll.Selected = token.Selected
	poll.BaseURL = baseURL
	poll.ProviderID = providerID
	poll.KeyID = keyID
	writeJSON(w, poll)
}

func (s *localHTTPServer) handleProxy(w http.ResponseWriter, r *http.Request) {
	if !validLocalProxyAuthorization(r) {
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "invalid local proxy authorization"})
		return
	}
	if r.URL.Path == "/v1/models" && r.Method == http.MethodGet {
		s.handleMergedModels(w)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxProxyRequestBodyBytes))
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSONStatus(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body is too large"})
			return
		}
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = r.Body.Close()
	model := requestModel(body)
	provider, key, targetModel, ok := s.store.keyForModelWithTarget(model)
	if !ok {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{
			"error": "no available API key for model " + fallback(model, "(default)"),
		})
		return
	}
	if targetModel != "" && targetModel != model {
		body = rewriteRequestModel(body, targetModel)
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

func validLocalProxyAuthorization(r *http.Request) bool {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	return header == "Bearer "+localProxyAPIKey
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

func rewriteRequestModel(body []byte, model string) []byte {
	if len(body) == 0 || strings.TrimSpace(model) == "" {
		return body
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	if _, ok := payload["model"]; !ok {
		return body
	}
	payload["model"] = strings.TrimSpace(model)
	rewritten, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return rewritten
}

func proxyTargetURL(baseURL string, requestURL *url.URL) (string, error) {
	if err := validateAPIBaseURL(baseURL); err != nil {
		return "", err
	}
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
