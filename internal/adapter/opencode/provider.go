package opencode

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// ProviderInfo holds the resolved provider configuration for OpenCode.
type ProviderInfo struct {
	Host             string
	IsOpenCodeHosted bool
}

// ProviderUnresolvableError indicates the OpenCode provider host cannot be
// determined from the available config and auth state.
type ProviderUnresolvableError struct{}

func (e *ProviderUnresolvableError) Error() string {
	return "cannot determine the OpenCode provider host from available config and auth state; " +
		"configure the provider explicitly, use --model provider/model with a known provider, or pass --egress-allow manually"
}

// ModelProviderUnknownError indicates a --model provider prefix that is not
// in the curated known-providers map.
type ModelProviderUnknownError struct {
	Provider string
}

func (e *ModelProviderUnknownError) Error() string {
	return fmt.Sprintf(
		"unknown model provider %q: Tessariq cannot determine the API host; "+
			"use --egress-allow to allowlist the provider's endpoint, or use --egress open",
		e.Provider,
	)
}

// knownProviderHosts maps provider prefixes (as used in "provider/model" format
// by models.dev) to their single-host API endpoints. Providers requiring
// wildcard host patterns (Bedrock, Azure OpenAI, Vertex AI) are excluded.
var knownProviderHosts = map[string]string{
	"anthropic":      "api.anthropic.com",
	"openai":         "api.openai.com",
	"google":         "generativelanguage.googleapis.com",
	"mistral":        "api.mistral.ai",
	"deepseek":       "api.deepseek.com",
	"xai":            "api.x.ai",
	"cohere":         "api.cohere.com",
	"groq":           "api.groq.com",
	"fireworks":      "api.fireworks.ai",
	"together":       "api.together.xyz",
	"cerebras":       "api.cerebras.ai",
	"deepinfra":      "api.deepinfra.com",
	"perplexity":     "api.perplexity.ai",
	"openrouter":     "openrouter.ai",
	"opencode":       "opencode.ai",
	"minimax":        "api.minimax.io",
	"moonshot":       "api.moonshot.ai",
	"zhipu":          "api.z.ai",
	"github-copilot": "api.githubcopilot.com",
	"github-models":  "models.github.ai",
	"nvidia":         "integrate.api.nvidia.com",
	"huggingface":    "router.huggingface.co",
	"llama":          "api.llama.com",
	"morph":          "api.morphllm.com",
	"venice":         "api.venice.ai",
}

// ParseModelProvider extracts the provider prefix before the first "/" in a
// model identifier. Returns "" if model has no "/" or the prefix is empty.
func ParseModelProvider(model string) string {
	i := strings.Index(model, "/")
	if i <= 0 {
		return ""
	}
	return model[:i]
}

// KnownProviderHost returns the API host for a known provider prefix.
// Returns ("", false) when the provider is not in the curated map.
func KnownProviderHost(provider string) (string, bool) {
	host, ok := knownProviderHosts[provider]
	return host, ok
}

// IsOpenCodeHostedHost reports whether host is opencode.ai or a subdomain.
func IsOpenCodeHostedHost(host string) bool {
	return isOpenCodeHosted(host)
}

// ResolveProvider determines the OpenCode provider host from parsed auth and
// config state. authData is required (from auth.json). configData is optional
// (from config.json, may be nil).
func ResolveProvider(authData []byte, configData []byte) (*ProviderInfo, error) {
	// Try config first (takes precedence).
	if configData != nil {
		var cfg map[string]any
		if err := json.Unmarshal(configData, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
		if host := extractHost(cfg, "provider"); host != "" {
			return newProviderInfo(host), nil
		}
	}

	// Fall back to auth data.
	var auth map[string]any
	if err := json.Unmarshal(authData, &auth); err != nil {
		return nil, fmt.Errorf("parse auth: %w", err)
	}

	for _, field := range []string{"provider", "base_url"} {
		if host := extractHost(auth, field); host != "" {
			return newProviderInfo(host), nil
		}
	}

	return nil, &ProviderUnresolvableError{}
}

// ResolveProviderFromPaths reads auth and config files to determine the
// OpenCode provider. authPath is the path to auth.json (required to exist by
// this point). configDir is the path to the config directory (may be empty).
// readFile is an injectable file reader for testability.
func ResolveProviderFromPaths(authPath, configDir string, readFile func(string) ([]byte, error)) (*ProviderInfo, error) {
	authData, err := readFile(authPath)
	if err != nil {
		return nil, fmt.Errorf("read auth file: %w", err)
	}

	var configData []byte
	if configDir != "" {
		configPath := filepath.Join(configDir, "config.json")
		data, readErr := readFile(configPath)
		if readErr == nil {
			configData = data
		}
		// config.json not found is not an error; fall through to auth-only resolution.
	}

	return ResolveProvider(authData, configData)
}

// extractHost reads a string field from the map and extracts the hostname.
// Returns empty string if the field is missing, empty, or not a string.
func extractHost(data map[string]any, field string) string {
	val, ok := data[field]
	if !ok {
		return ""
	}
	s, ok := val.(string)
	if !ok || s == "" {
		return ""
	}

	// Try parsing as a URL first.
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}

	// Treat as bare hostname.
	return s
}

// isOpenCodeHosted returns true when the host is opencode.ai or a subdomain.
func isOpenCodeHosted(host string) bool {
	return host == "opencode.ai" || strings.HasSuffix(host, ".opencode.ai")
}

func newProviderInfo(host string) *ProviderInfo {
	return &ProviderInfo{
		Host:             host,
		IsOpenCodeHosted: isOpenCodeHosted(host),
	}
}
