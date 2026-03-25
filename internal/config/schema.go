// Package config provides configuration schema and persistence for mcpmu.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SchemaVersion is the current config schema version.
const SchemaVersion = 1

// ServerKind represents the transport type for an MCP server.
type ServerKind string

const (
	ServerKindStdio          ServerKind = "stdio"
	ServerKindStreamableHTTP ServerKind = "streamable_http"
)

// OAuthConfig holds per-server OAuth configuration.
type OAuthConfig struct {
	ClientID     string   `json:"client_id,omitempty"`
	ClientSecret string   `json:"client_secret,omitempty"`
	CallbackPort *int     `json:"callback_port,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
}

// ServerConfig represents an MCP server configuration.
// Field names are compatible with mcpServers format (Claude Desktop, Cursor, etc).
// The server name/identifier is the map key, not stored in this struct.
type ServerConfig struct {
	Kind      ServerKind        `json:"kind,omitempty"`      // optional, inferred from command vs url
	Enabled   *bool             `json:"enabled,omitempty"`   // nil treated as true (enabled by default)
	Autostart bool              `json:"autostart,omitempty"` // start server automatically on app launch
	Command   string            `json:"command,omitempty"`   // stdio only
	Args      []string          `json:"args,omitempty"`      // stdio only
	Cwd       string            `json:"cwd,omitempty"`
	Env       map[string]string `json:"env,omitempty"`

	// Streamable HTTP fields (mutually exclusive with Command)
	URL               string            `json:"url,omitempty"`                  // Server URL for HTTP transport
	BearerTokenEnvVar string            `json:"bearer_token_env_var,omitempty"` // Env var containing bearer token
	HTTPHeaders       map[string]string `json:"http_headers,omitempty"`         // Static HTTP headers
	EnvHTTPHeaders    map[string]string `json:"env_http_headers,omitempty"`     // HTTP headers from env vars (key=header name, value=env var name)
	OAuth             *OAuthConfig      `json:"oauth,omitempty"`                // OAuth configuration (HTTP only)

	// Timeouts (seconds)
	StartupTimeoutSec int `json:"startup_timeout_sec,omitempty"` // Default 10
	ToolTimeoutSec    int `json:"tool_timeout_sec,omitempty"`    // Default 60

	// Template reference: if set, this server inherits from a template.
	// The server's Args are appended to the template's Args; Env is merged
	// (server values override template values). Other fields from the template
	// are used as defaults and can be overridden by the server config.
	Template string `json:"template,omitempty"`
}

// TemplateConfig defines a reusable server configuration template.
// Templates allow defining the base MCP server configuration once and reusing
// it across multiple server definitions. They also support per-tool permissions.
type TemplateConfig struct {
	// Description is a human-readable description of the template.
	Description string `json:"description,omitempty"`

	// Base server configuration fields (same as ServerConfig).
	Kind    ServerKind        `json:"kind,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	URL               string            `json:"url,omitempty"`
	BearerTokenEnvVar string            `json:"bearer_token_env_var,omitempty"`
	HTTPHeaders       map[string]string `json:"http_headers,omitempty"`
	EnvHTTPHeaders    map[string]string `json:"env_http_headers,omitempty"`
	OAuth             *OAuthConfig      `json:"oauth,omitempty"`

	StartupTimeoutSec int `json:"startup_timeout_sec,omitempty"`
	ToolTimeoutSec    int `json:"tool_timeout_sec,omitempty"`

	// DenyByDefault when true means tools without an explicit permission
	// entry are denied. When false (default), unlisted tools are allowed.
	DenyByDefault bool `json:"denyByDefault,omitempty"`

	// ToolPermissions stores per-tool allow/deny decisions.
	// Key is the tool name, value is true (allow) or false (deny).
	// Tools not present in this map fall back to DenyByDefault.
	ToolPermissions map[string]bool `json:"toolPermissions,omitempty"`

	// TestArgs are extra arguments appended when starting the template as a
	// test server for tool discovery. This is useful for servers that require
	// connection-specific parameters (e.g. ABAP system details) to start.
	TestArgs []string `json:"testArgs,omitempty"`
}

// TemplateEntry pairs a template name with its configuration.
type TemplateEntry struct {
	Name   string
	Config TemplateConfig
}

// Validate checks that the TemplateConfig is in a valid state.
func (t TemplateConfig) Validate() error {
	hasCommand := t.Command != ""
	hasURL := t.URL != ""

	if hasCommand && hasURL {
		return errors.New("cannot set both command and url: stdio and http are mutually exclusive")
	}
	if !hasCommand && !hasURL {
		return errors.New("must set either command (for stdio) or url (for http)")
	}

	if hasCommand {
		if t.BearerTokenEnvVar != "" {
			return errors.New("bearer_token_env_var is only valid for http servers")
		}
		if len(t.HTTPHeaders) > 0 {
			return errors.New("http_headers is only valid for http servers")
		}
		if len(t.EnvHTTPHeaders) > 0 {
			return errors.New("env_http_headers is only valid for http servers")
		}
		if t.OAuth != nil {
			return errors.New("oauth is only valid for http servers")
		}
	}

	if hasURL {
		if len(t.Args) > 0 {
			return errors.New("args is only valid for stdio servers")
		}
		if t.BearerTokenEnvVar != "" && t.OAuth != nil {
			return errors.New("bearer_token_env_var and oauth are mutually exclusive")
		}
		if t.OAuth != nil && t.OAuth.CallbackPort != nil {
			port := *t.OAuth.CallbackPort
			if port < 1 || port > 65535 {
				return fmt.Errorf("oauth callback_port must be 1-65535, got %d", port)
			}
		}
	}

	return nil
}

// IsHTTP returns true if this template is for an HTTP server.
func (t TemplateConfig) IsHTTP() bool {
	return t.URL != ""
}

// IsToolAllowed returns whether the given tool is allowed by this template's
// permission configuration. It checks explicit ToolPermissions first, then
// falls back to the DenyByDefault setting.
func (t TemplateConfig) IsToolAllowed(toolName string) bool {
	if allowed, ok := t.ToolPermissions[toolName]; ok {
		return allowed
	}
	return !t.DenyByDefault
}

// ToServerConfig converts the template to a base ServerConfig.
// TestArgs are NOT included — use ToTestServerConfig for tool discovery.
func (t TemplateConfig) ToServerConfig() ServerConfig {
	return ServerConfig{
		Kind:              t.Kind,
		Command:           t.Command,
		Args:              append([]string{}, t.Args...),
		Cwd:               t.Cwd,
		Env:               copyStringMap(t.Env),
		URL:               t.URL,
		BearerTokenEnvVar: t.BearerTokenEnvVar,
		HTTPHeaders:       copyStringMap(t.HTTPHeaders),
		EnvHTTPHeaders:    copyStringMap(t.EnvHTTPHeaders),
		OAuth:             t.OAuth,
		StartupTimeoutSec: t.StartupTimeoutSec,
		ToolTimeoutSec:    t.ToolTimeoutSec,
	}
}

// ToTestServerConfig converts the template to a ServerConfig for test/discovery.
// TestArgs are appended to Args so servers that require connection-specific
// parameters (e.g. ABAP system details) can be started for tool discovery.
func (t TemplateConfig) ToTestServerConfig() ServerConfig {
	srv := t.ToServerConfig()
	srv.Args = append(srv.Args, t.TestArgs...)
	return srv
}

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// UnmarshalJSON implements custom JSON unmarshaling for backward compatibility.
// Migrates old flat fields (scopes, oauth_client_id) into the nested oauth block.
func (s *ServerConfig) UnmarshalJSON(data []byte) error {
	// Alias to avoid recursion
	type Alias ServerConfig

	// Extended type with legacy flat fields
	aux := &struct {
		*Alias
		LegacyScopes        []string `json:"scopes,omitempty"`
		LegacyOAuthClientID string   `json:"oauth_client_id,omitempty"`
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Migrate legacy flat fields into OAuth block (nested block takes precedence)
	if aux.LegacyOAuthClientID != "" || len(aux.LegacyScopes) > 0 {
		if s.OAuth == nil {
			s.OAuth = &OAuthConfig{}
		}
		if s.OAuth.ClientID == "" && aux.LegacyOAuthClientID != "" {
			s.OAuth.ClientID = aux.LegacyOAuthClientID
		}
		if len(s.OAuth.Scopes) == 0 && len(aux.LegacyScopes) > 0 {
			s.OAuth.Scopes = aux.LegacyScopes
		}
	}

	return nil
}

// ServerEntry pairs a server name with its configuration.
// Used for iteration when the name is needed.
type ServerEntry struct {
	Name   string
	Config ServerConfig
}

// NamespaceConfig represents a namespace that groups servers and their tool permissions.
// The namespace name/identifier is the map key, not stored in this struct.
type NamespaceConfig struct {
	Description    string          `json:"description,omitempty"`
	Separator      string          `json:"separator,omitempty"` // Tool name separator (default ".")
	ServerIDs      []string        `json:"serverIds"`
	DenyByDefault  bool            `json:"denyByDefault,omitempty"`  // If true, unconfigured tools are denied
	ServerDefaults map[string]bool `json:"serverDefaults,omitempty"` // Per-server deny-default override (true = deny)
}

// GetSeparator returns the configured separator, defaulting to ".".
func (ns NamespaceConfig) GetSeparator() string {
	if ns.Separator == "" {
		return "."
	}
	return ns.Separator
}

// NamespaceEntry pairs a namespace name with its configuration.
// Used for iteration when the name is needed.
type NamespaceEntry struct {
	Name   string
	Config NamespaceConfig
}

// ToolPermission controls whether a specific tool is enabled in a namespace.
type ToolPermission struct {
	Namespace string `json:"namespace"`
	Server    string `json:"server"`
	ToolName  string `json:"toolName"`
	Enabled   bool   `json:"enabled"`
}

// Config is the root configuration structure.
type Config struct {
	SchemaVersion    int                        `json:"schemaVersion"`
	DefaultNamespace string                     `json:"defaultNamespace,omitempty"`
	Templates        map[string]TemplateConfig  `json:"templates,omitempty"`
	Servers          map[string]ServerConfig    `json:"servers"`
	Namespaces       map[string]NamespaceConfig `json:"namespaces,omitempty"`
	ToolPermissions  []ToolPermission           `json:"toolPermissions,omitempty"`
	LastModified     time.Time                  `json:"lastModified"`

	// OAuth settings (Codex-compatible)
	MCPOAuthCredentialStore string `json:"mcp_oauth_credentials_store,omitempty"` // "auto", "keyring", "file"
	MCPOAuthCallbackPort    *int   `json:"mcp_oauth_callback_port,omitempty"`     // nil = random, 0 invalid
}

// NewConfig creates a new empty configuration with default values.
func NewConfig() *Config {
	return &Config{
		SchemaVersion: SchemaVersion,
		Templates:     make(map[string]TemplateConfig),
		Servers:       make(map[string]ServerConfig),
		Namespaces:    make(map[string]NamespaceConfig),
		LastModified:  time.Now(),
	}
}

// IsEnabled returns whether the server is enabled (nil defaults to true).
func (s ServerConfig) IsEnabled() bool {
	return s.Enabled == nil || *s.Enabled
}

// IsHTTP returns true if this server uses HTTP transport (has URL configured).
func (s ServerConfig) IsHTTP() bool {
	return s.URL != ""
}

// GetKind returns the effective kind based on configuration.
// If URL is set, returns ServerKindStreamableHTTP regardless of Kind field.
func (s ServerConfig) GetKind() ServerKind {
	if s.URL != "" {
		return ServerKindStreamableHTTP
	}
	if s.Kind == "" {
		return ServerKindStdio
	}
	return s.Kind
}

// StartupTimeout returns the startup timeout in seconds, with a default of 10.
func (s ServerConfig) StartupTimeout() int {
	if s.StartupTimeoutSec <= 0 {
		return 10
	}
	return s.StartupTimeoutSec
}

// ToolTimeout returns the tool call timeout in seconds, with a default of 60.
func (s ServerConfig) ToolTimeout() int {
	if s.ToolTimeoutSec <= 0 {
		return 60
	}
	return s.ToolTimeoutSec
}

// SetEnabled sets the enabled state.
func (s *ServerConfig) SetEnabled(enabled bool) {
	s.Enabled = &enabled
}

// Validate checks that the ServerConfig is in a valid state.
// Returns an error if:
// - Both Command and URL are set (mutually exclusive)
// - Neither Command nor URL is set (must have one) — unless Template is set
// - Kind is explicitly set but doesn't match the fields
func (s ServerConfig) Validate() error {
	hasCommand := s.Command != ""
	hasURL := s.URL != ""

	// Servers referencing a template are allowed to have no command/url
	// (they inherit from the template). We still validate mutually exclusive fields.
	if s.Template != "" {
		if hasCommand && hasURL {
			return errors.New("cannot set both command and url: stdio and http are mutually exclusive")
		}
		// Template-based servers only carry args (appended) and env (merged),
		// so skip the "must have command or url" check.
		return nil
	}

	// Must have exactly one of Command or URL
	if hasCommand && hasURL {
		return errors.New("cannot set both command and url: stdio and http are mutually exclusive")
	}
	if !hasCommand && !hasURL {
		return errors.New("must set either command (for stdio) or url (for http)")
	}

	// If Kind is explicitly set, it must match the fields
	if s.Kind != "" {
		if s.Kind == ServerKindStdio && hasURL {
			return fmt.Errorf("kind is %q but url is set", s.Kind)
		}
		if s.Kind == ServerKindStreamableHTTP && hasCommand {
			return fmt.Errorf("kind is %q but command is set", s.Kind)
		}
	}

	// Stdio-specific validation
	if hasCommand {
		// Args without command doesn't make sense, but Args with command is fine
		// URL-related fields shouldn't be set
		if s.BearerTokenEnvVar != "" {
			return errors.New("bearer_token_env_var is only valid for http servers")
		}
		if len(s.HTTPHeaders) > 0 {
			return errors.New("http_headers is only valid for http servers")
		}
		if len(s.EnvHTTPHeaders) > 0 {
			return errors.New("env_http_headers is only valid for http servers")
		}
		if s.OAuth != nil {
			return errors.New("oauth is only valid for http servers")
		}
	}

	// HTTP-specific validation
	if hasURL {
		// Command-related fields shouldn't be set
		if len(s.Args) > 0 {
			return errors.New("args is only valid for stdio servers")
		}

		// bearer_token_env_var and oauth are mutually exclusive
		if s.BearerTokenEnvVar != "" && s.OAuth != nil {
			return errors.New("bearer_token_env_var and oauth are mutually exclusive")
		}

		// Validate OAuth callback port if set
		if s.OAuth != nil && s.OAuth.CallbackPort != nil {
			port := *s.OAuth.CallbackPort
			if port < 1 || port > 65535 {
				return fmt.Errorf("oauth callback_port must be 1-65535, got %d", port)
			}
		}
	}

	return nil
}

// ServerEntries returns the servers as name/config pairs, sorted by name for display.
func (c *Config) ServerEntries() []ServerEntry {
	entries := make([]ServerEntry, 0, len(c.Servers))
	for name, cfg := range c.Servers {
		entries = append(entries, ServerEntry{Name: name, Config: cfg})
	}

	// Keep list ordering stable across runs (maps are randomized).
	sort.SliceStable(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries
}

// NamespaceEntries returns the namespaces as name/config pairs, sorted by name for display.
func (c *Config) NamespaceEntries() []NamespaceEntry {
	entries := make([]NamespaceEntry, 0, len(c.Namespaces))
	for name, cfg := range c.Namespaces {
		entries = append(entries, NamespaceEntry{Name: name, Config: cfg})
	}

	// Keep list ordering stable across runs (maps are randomized).
	sort.SliceStable(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries
}

// GetServer returns a server by name and whether it was found.
func (c *Config) GetServer(name string) (ServerConfig, bool) {
	s, ok := c.Servers[name]
	return s, ok
}

// GetTemplate returns a template by name and whether it was found.
func (c *Config) GetTemplate(name string) (TemplateConfig, bool) {
	if c.Templates == nil {
		return TemplateConfig{}, false
	}
	t, ok := c.Templates[name]
	return t, ok
}

// TemplateEntries returns the templates as name/config pairs, sorted by name for display.
func (c *Config) TemplateEntries() []TemplateEntry {
	entries := make([]TemplateEntry, 0, len(c.Templates))
	for name, cfg := range c.Templates {
		entries = append(entries, TemplateEntry{Name: name, Config: cfg})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}

// ResolveServer returns the effective ServerConfig for a server, merging its
// template if one is referenced. The merge rules are:
//   - Template provides the base config (command, url, args, env, timeouts, etc.)
//   - Server-level Args are appended to template Args
//   - Server-level Env is merged on top of template Env (server wins)
//   - Non-zero server fields override the template (e.g. Cwd, timeouts)
//   - Enabled and Autostart always come from the server config
func (c *Config) ResolveServer(name string) (ServerConfig, bool) {
	srv, ok := c.Servers[name]
	if !ok {
		return ServerConfig{}, false
	}
	if srv.Template == "" {
		return srv, true
	}
	tmpl, ok := c.GetTemplate(srv.Template)
	if !ok {
		// Template not found — return server as-is (will likely fail validation)
		return srv, true
	}

	resolved := tmpl.ToServerConfig()

	// Preserve server identity fields
	resolved.Enabled = srv.Enabled
	resolved.Autostart = srv.Autostart
	resolved.Template = srv.Template

	// Append server args to template args
	if len(srv.Args) > 0 {
		resolved.Args = append(resolved.Args, srv.Args...)
	}

	// Merge env (server values override template values)
	if len(srv.Env) > 0 {
		if resolved.Env == nil {
			resolved.Env = make(map[string]string)
		}
		for k, v := range srv.Env {
			resolved.Env[k] = v
		}
	}

	// Override non-zero fields from server
	if srv.Command != "" {
		resolved.Command = srv.Command
	}
	if srv.URL != "" {
		resolved.URL = srv.URL
	}
	if srv.Cwd != "" {
		resolved.Cwd = srv.Cwd
	}
	if srv.Kind != "" {
		resolved.Kind = srv.Kind
	}
	if srv.StartupTimeoutSec > 0 {
		resolved.StartupTimeoutSec = srv.StartupTimeoutSec
	}
	if srv.ToolTimeoutSec > 0 {
		resolved.ToolTimeoutSec = srv.ToolTimeoutSec
	}
	if srv.BearerTokenEnvVar != "" {
		resolved.BearerTokenEnvVar = srv.BearerTokenEnvVar
	}
	if srv.OAuth != nil {
		resolved.OAuth = srv.OAuth
	}
	if len(srv.HTTPHeaders) > 0 {
		resolved.HTTPHeaders = srv.HTTPHeaders
	}
	if len(srv.EnvHTTPHeaders) > 0 {
		resolved.EnvHTTPHeaders = srv.EnvHTTPHeaders
	}

	return resolved, true
}

// IsToolAllowedByTemplate returns whether a tool is allowed by the server's
// template permission configuration. Returns true if the server has no template.
func (c *Config) IsToolAllowedByTemplate(serverName, toolName string) bool {
	srv, ok := c.Servers[serverName]
	if !ok || srv.Template == "" {
		return true
	}
	tmpl, ok := c.GetTemplate(srv.Template)
	if !ok {
		return true
	}
	return tmpl.IsToolAllowed(toolName)
}

// GetTemplateForServer returns the template config for a server, if any.
func (c *Config) GetTemplateForServer(serverName string) (TemplateConfig, bool) {
	srv, ok := c.Servers[serverName]
	if !ok || srv.Template == "" {
		return TemplateConfig{}, false
	}
	return c.GetTemplate(srv.Template)
}

// GetNamespace returns a namespace by name and whether it was found.
func (c *Config) GetNamespace(name string) (NamespaceConfig, bool) {
	ns, ok := c.Namespaces[name]
	return ns, ok
}

// MarshalJSON implements custom JSON marshaling.
func (c *Config) MarshalJSON() ([]byte, error) {
	type Alias Config
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	})
}

// Validate checks that all templates and servers in the config are valid.
// Returns an error describing the first invalid entry found.
func (c *Config) Validate() error {
	for name, tmpl := range c.Templates {
		if err := tmpl.Validate(); err != nil {
			return fmt.Errorf("template %q: %w", name, err)
		}
	}
	for name, srv := range c.Servers {
		if err := srv.Validate(); err != nil {
			return fmt.Errorf("server %q: %w", name, err)
		}
		// Validate template reference exists
		if srv.Template != "" {
			if _, ok := c.GetTemplate(srv.Template); !ok {
				return fmt.Errorf("server %q references unknown template %q", name, srv.Template)
			}
		}
	}
	return nil
}
