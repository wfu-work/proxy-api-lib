package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wfu-work/proxy-api-lib/auth"
	"github.com/wfu-work/proxy-api-lib/compatible"
	"github.com/wfu-work/proxy-api-lib/domains"
)

// Config contains the Codex-style model and provider settings this library consumes.
type Config struct {
	ModelProvider          string
	Model                  string
	ModelReasoningEffort   string
	DisableResponseStorage bool
	PreferredAuthMethod    string
	Providers              map[string]ProviderConfig
	Auth                   map[string]string
}

// ProviderConfig describes one [model_providers.<name>] entry.
type ProviderConfig struct {
	Name     string
	BaseURL  string
	WireAPI  string
	ProxyURL string
}

// Load reads a Codex-style config.toml and auth.json.
func Load(configPath, authPath string) (*Config, error) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{},
		Auth:      map[string]string{},
	}
	if configPath != "" {
		if err := cfg.loadTOML(configPath); err != nil {
			return nil, err
		}
	}
	if authPath != "" {
		if err := cfg.loadAuth(authPath); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// Provider returns an OpenAI-compatible provider config.
func (c *Config) Provider(name string) compatible.Config {
	if name == "" {
		name = c.ModelProvider
	}
	provider := c.Providers[name]
	if provider.Name == "" {
		provider.Name = name
	}
	if provider.WireAPI == "" {
		provider.WireAPI = compatible.WireAPIResponses
	}
	return compatible.Config{
		Name:     provider.Name,
		BaseURL:  provider.BaseURL,
		WireAPI:  provider.WireAPI,
		ProxyURL: provider.ProxyURL,
	}
}

// RequestDefaults maps Codex top-level settings onto a Responses request.
func (c *Config) RequestDefaults() domains.ResponseRequest {
	req := domains.ResponseRequest{
		Model: c.Model,
	}
	if c.ModelReasoningEffort != "" {
		req.Reasoning = &domains.Reasoning{Effort: c.ModelReasoningEffort}
	}
	if c.DisableResponseStorage {
		store := false
		req.Store = &store
	}
	return req
}

// Credential returns a credential based on preferred_auth_method.
func (c *Config) Credential() domains.Credential {
	method := strings.ToLower(c.PreferredAuthMethod)
	value := c.Auth["OPENAI_API_KEY"]
	if value == "" {
		return nil
	}
	if method == "token" || method == "bearer" {
		return auth.BearerToken(value)
	}
	return auth.APIKey(value)
}

// OpenAIAPIKey returns the OPENAI_API_KEY auth value for simple API key flows.
func (c *Config) OpenAIAPIKey() string {
	return c.Auth["OPENAI_API_KEY"]
}

func (c *Config) loadAuth(path string) error {
	data, err := os.ReadFile(expandPath(path))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &c.Auth)
}

func (c *Config) loadTOML(path string) error {
	file, err := os.Open(expandPath(path))
	if err != nil {
		return err
	}
	defer file.Close()

	var currentProvider string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := stripComment(strings.TrimSpace(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			if strings.HasPrefix(section, "model_providers.") {
				currentProvider = strings.TrimPrefix(section, "model_providers.")
				if c.Providers[currentProvider].Name == "" {
					c.Providers[currentProvider] = ProviderConfig{Name: currentProvider}
				}
				continue
			}
			currentProvider = ""
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("codex: invalid config line %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if currentProvider != "" {
			provider := c.Providers[currentProvider]
			setProviderField(&provider, key, parseString(value))
			c.Providers[currentProvider] = provider
			continue
		}
		c.setTopLevel(key, value)
	}
	return scanner.Err()
}

func (c *Config) setTopLevel(key, value string) {
	switch key {
	case "model_provider":
		c.ModelProvider = parseString(value)
	case "model":
		c.Model = parseString(value)
	case "model_reasoning_effort":
		c.ModelReasoningEffort = parseString(value)
	case "disable_response_storage":
		c.DisableResponseStorage = parseBool(value)
	case "preferred_auth_method":
		c.PreferredAuthMethod = parseString(value)
	}
}

func setProviderField(provider *ProviderConfig, key, value string) {
	switch key {
	case "name":
		provider.Name = value
	case "base_url":
		provider.BaseURL = value
	case "wire_api":
		provider.WireAPI = value
	case "proxy_url":
		provider.ProxyURL = value
	}
}

func parseString(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	value = strings.Trim(value, `'`)
	return value
}

func parseBool(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func stripComment(line string) string {
	inQuote := false
	var quote rune
	for i, r := range line {
		if (r == '"' || r == '\'') && (i == 0 || line[i-1] != '\\') {
			if inQuote && r == quote {
				inQuote = false
				continue
			}
			if !inQuote {
				inQuote = true
				quote = r
				continue
			}
		}
		if r == '#' && !inQuote {
			return strings.TrimSpace(line[:i])
		}
	}
	return line
}

func expandPath(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
