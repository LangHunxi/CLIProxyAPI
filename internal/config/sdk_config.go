// Package config provides configuration management for the CLI Proxy API server.
// It handles loading and parsing YAML configuration files, and provides structured
// access to application settings including server port, authentication directory,
// debug settings, proxy configuration, and API keys.
package config

import (
	"strings"
	"sync/atomic"

	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
)

// ApiKeyEntry represents an API key with extended metadata for management.
type ApiKeyEntry struct {
	// ID is a stable unique identifier for this key entry (UUID format).
	ID string `yaml:"id,omitempty" json:"id,omitempty"`

	// Key is the actual API key value.
	Key string `yaml:"api-key" json:"api-key"`

	// Name is an optional human-readable name for the key.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// IsActive indicates whether the key is currently enabled.
	IsActive bool `yaml:"is-active" json:"is-active"`

	// UsageCount tracks how many times this key has been used.
	// Use atomic operations for thread-safe updates.
	UsageCount int64 `yaml:"usage-count,omitempty" json:"usage-count,omitempty"`

	// InputTokens tracks total input tokens consumed by this key.
	InputTokens int64 `yaml:"input-tokens,omitempty" json:"input-tokens,omitempty"`

	// OutputTokens tracks total output tokens consumed by this key.
	OutputTokens int64 `yaml:"output-tokens,omitempty" json:"output-tokens,omitempty"`

	// LastUsedAt is the ISO 8601 timestamp of the last usage.
	LastUsedAt string `yaml:"last-used-at,omitempty" json:"last-used-at,omitempty"`

	// CreatedAt is the ISO 8601 timestamp when this key was created.
	CreatedAt string `yaml:"created-at,omitempty" json:"created-at,omitempty"`
}

// IncrementUsage atomically increments the usage count.
// Note: LastUsedAt is updated by the caller separately (not thread-safe for that field).
func (e *ApiKeyEntry) IncrementUsage(timestamp string) {
	atomic.AddInt64(&e.UsageCount, 1)
	// LastUsedAt update is best-effort; in high concurrency the last writer wins
	e.LastUsedAt = timestamp
}

// IncrementTokens atomically increments input and output token counts.
func (e *ApiKeyEntry) IncrementTokens(inputTokens, outputTokens int64) {
	if inputTokens > 0 {
		atomic.AddInt64(&e.InputTokens, inputTokens)
	}
	if outputTokens > 0 {
		atomic.AddInt64(&e.OutputTokens, outputTokens)
	}
}

// GetUsageCount returns the current usage count atomically.
func (e *ApiKeyEntry) GetUsageCount() int64 {
	return atomic.LoadInt64(&e.UsageCount)
}

// GetInputTokens returns the current input token count atomically.
func (e *ApiKeyEntry) GetInputTokens() int64 {
	return atomic.LoadInt64(&e.InputTokens)
}

// GetOutputTokens returns the current output token count atomically.
func (e *ApiKeyEntry) GetOutputTokens() int64 {
	return atomic.LoadInt64(&e.OutputTokens)
}

// UnmarshalYAML implements custom YAML unmarshaling for ApiKeyEntry.
// This allows backward compatibility with simple string format API keys.
// Supports both:
//   - Simple string format: "your-api-key"
//   - Extended format: { api-key: "your-api-key", is-active: true, ... }
func (e *ApiKeyEntry) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First, try to unmarshal as a simple string (legacy format)
	var simpleKey string
	if err := unmarshal(&simpleKey); err == nil && simpleKey != "" {
		e.Key = simpleKey
		e.IsActive = true // Default to active for legacy format
		return nil
	}

	// Otherwise, unmarshal as the full struct format
	// Use an alias type to avoid infinite recursion
	type apiKeyEntryAlias ApiKeyEntry
	var alias apiKeyEntryAlias
	if err := unmarshal(&alias); err != nil {
		return err
	}

	*e = ApiKeyEntry(alias)
	return nil
}

// MarshalYAML emits scalar string form when metadata fields are empty/default,
// so config can be persisted as:
// api-keys:
//   - key1
//   - key2
func (e ApiKeyEntry) MarshalYAML() (interface{}, error) {
	key := strings.TrimSpace(e.Key)
	if key == "" {
		return "", nil
	}
	if e.ID == "" && e.Name == "" && !e.IsActive && e.UsageCount == 0 && e.InputTokens == 0 && e.OutputTokens == 0 && e.LastUsedAt == "" && e.CreatedAt == "" {
		return key, nil
	}
	type alias ApiKeyEntry
	copy := alias(e)
	copy.Key = key
	return copy, nil
}

// SDKConfig represents the application's configuration, loaded from a YAML file.
type SDKConfig struct {
	// ProxyURL is the URL of an optional proxy server to use for outbound requests.
	ProxyURL string `yaml:"proxy-url" json:"proxy-url"`

	// EnableGeminiCLIEndpoint controls whether Gemini CLI internal endpoints (/v1internal:*) are enabled.
	// Default is false for safety; when false, /v1internal:* requests are rejected.
	EnableGeminiCLIEndpoint bool `yaml:"enable-gemini-cli-endpoint" json:"enable-gemini-cli-endpoint"`

	// ForceModelPrefix requires explicit model prefixes (e.g., "teamA/gemini-3-pro-preview")
	// to target prefixed credentials. When false, unprefixed model requests may use prefixed
	// credentials as well.
	ForceModelPrefix bool `yaml:"force-model-prefix" json:"force-model-prefix"`

	// RequestLog enables or disables detailed request logging functionality.
	RequestLog bool `yaml:"request-log" json:"request-log"`

	Access sdkaccess.AccessConfig `yaml:"access,omitempty" json:"access,omitempty"`

	// APIKeys is a list of keys for authenticating clients to this proxy server.
	// Supports both simple string format (for backward compatibility) and extended ApiKeyEntry format.
	APIKeys []ApiKeyEntry `yaml:"api-keys" json:"api-keys"`

	// PassthroughHeaders controls whether upstream response headers are forwarded to downstream clients.
	// Default is false (disabled).
	PassthroughHeaders bool `yaml:"passthrough-headers" json:"passthrough-headers"`

	// Streaming configures server-side streaming behavior (keep-alives and safe bootstrap retries).
	Streaming StreamingConfig `yaml:"streaming" json:"streaming"`

	// NonStreamKeepAliveInterval controls how often blank lines are emitted for non-streaming responses.
	// <= 0 disables keep-alives. Value is in seconds.
	NonStreamKeepAliveInterval int `yaml:"nonstream-keepalive-interval,omitempty" json:"nonstream-keepalive-interval,omitempty"`
}

// StreamingConfig holds server streaming behavior configuration.
type StreamingConfig struct {
	// KeepAliveSeconds controls how often the server emits SSE heartbeats (": keep-alive\n\n").
	// <= 0 disables keep-alives. Default is 0.
	KeepAliveSeconds int `yaml:"keepalive-seconds,omitempty" json:"keepalive-seconds,omitempty"`

	// BootstrapRetries controls how many times the server may retry a streaming request before any bytes are sent,
	// to allow auth rotation / transient recovery.
	// <= 0 disables bootstrap retries. Default is 0.
	BootstrapRetries int `yaml:"bootstrap-retries,omitempty" json:"bootstrap-retries,omitempty"`
}
