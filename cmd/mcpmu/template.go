package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Bigsy/mcpmu/internal/config"
	"github.com/spf13/cobra"
)

// ============================================================================
// template (parent command)
// ============================================================================

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage pre-defined server templates",
	Long: `Templates allow defining a base MCP server configuration once and reusing
it across multiple server definitions.  Servers that reference a template
inherit the template's command, args, environment, timeouts, etc.

Server-level args are appended to template args, server-level env is merged
on top of template env, and other non-zero server fields override the
template defaults.

Templates can also carry a disabled-tools list: any tool on that list is
never exposed to MCP clients.`,
}

func init() {
	rootCmd.AddCommand(templateCmd)
}

// ============================================================================
// template add
// ============================================================================

var (
	tmplAddEnvFlags       []string
	tmplAddCwd            string
	tmplAddURL            string
	tmplAddBearerEnv      string
	tmplAddScopes         []string
	tmplAddOAuthClientID  string
	tmplAddOAuthCBPort    int
	tmplAddStartupTimeout int
	tmplAddToolTimeout    int
)

var templateAddCmd = &cobra.Command{
	Use:   "add <name> [<url> | -- <command> [args...]]",
	Short: "Add a new server template",
	Long: `Add a pre-defined server template.

Examples:
  # Stdio template
  mcpmu template add abap -- npx -y @sap/abap-mcp --system

  # HTTP template
  mcpmu template add figma-base https://mcp.figma.com/mcp --bearer-env FIGMA_TOKEN`,
	RunE: runTemplateAdd,
}

func init() {
	templateAddCmd.Flags().StringArrayVarP(&tmplAddEnvFlags, "env", "e", nil, "Environment variable (KEY=VALUE)")
	templateAddCmd.Flags().StringVar(&tmplAddCwd, "cwd", "", "Working directory")
	templateAddCmd.Flags().StringVar(&tmplAddURL, "url", "", "Server URL for HTTP transport")
	templateAddCmd.Flags().StringVar(&tmplAddBearerEnv, "bearer-env", "", "Env var containing bearer token")
	templateAddCmd.Flags().StringSliceVar(&tmplAddScopes, "scopes", nil, "OAuth scopes")
	templateAddCmd.Flags().StringVar(&tmplAddOAuthClientID, "oauth-client-id", "", "Pre-registered OAuth client ID")
	templateAddCmd.Flags().IntVar(&tmplAddOAuthCBPort, "oauth-callback-port", 0, "OAuth callback port")
	templateAddCmd.Flags().IntVar(&tmplAddStartupTimeout, "startup-timeout", 0, "Startup timeout in seconds")
	templateAddCmd.Flags().IntVar(&tmplAddToolTimeout, "tool-timeout", 0, "Tool call timeout in seconds")

	templateCmd.AddCommand(templateAddCmd)
}

func runTemplateAdd(cmd *cobra.Command, args []string) error {
	// Determine HTTP vs stdio
	if tmplAddURL != "" {
		return runTemplateAddHTTP(cmd, args)
	}
	dashIdx := cmd.ArgsLenAtDash()
	if dashIdx == -1 && len(args) >= 2 && isURL(args[1]) {
		tmplAddURL = args[1]
		return runTemplateAddHTTP(cmd, args[:1])
	}
	return runTemplateAddStdio(cmd, args)
}

func runTemplateAddStdio(cmd *cobra.Command, args []string) error {
	if tmplAddBearerEnv != "" || tmplAddOAuthClientID != "" || len(tmplAddScopes) > 0 {
		return fmt.Errorf("OAuth/bearer flags are only valid for HTTP templates")
	}

	dashIdx := cmd.ArgsLenAtDash()
	if dashIdx == -1 {
		return fmt.Errorf("missing -- separator\n\nUsage: mcpmu template add <name> -- <command> [args...]")
	}
	if dashIdx < 1 {
		return fmt.Errorf("missing template name")
	}
	name := args[0]
	cmdArgs := args[dashIdx:]
	if len(cmdArgs) < 1 {
		return fmt.Errorf("missing command after --")
	}

	env, err := parseEnvFlags(tmplAddEnvFlags)
	if err != nil {
		return err
	}

	cfg, err := loadCfg()
	if err != nil {
		return err
	}

	tmpl := config.TemplateConfig{
		Command:           cmdArgs[0],
		Args:              cmdArgs[1:],
		Cwd:               tmplAddCwd,
		Env:               env,
		StartupTimeoutSec: tmplAddStartupTimeout,
		ToolTimeoutSec:    tmplAddToolTimeout,
	}

	if err := cfg.AddTemplate(name, tmpl); err != nil {
		return err
	}
	if err := saveCfg(cfg); err != nil {
		return err
	}
	fmt.Printf("Added template %q\n", name)
	return nil
}

func runTemplateAddHTTP(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing template name")
	}
	name := args[0]

	env, err := parseEnvFlags(tmplAddEnvFlags)
	if err != nil {
		return err
	}

	cfg, err := loadCfg()
	if err != nil {
		return err
	}

	tmpl := config.TemplateConfig{
		URL:               tmplAddURL,
		BearerTokenEnvVar: tmplAddBearerEnv,
		Env:               env,
		Cwd:               tmplAddCwd,
		StartupTimeoutSec: tmplAddStartupTimeout,
		ToolTimeoutSec:    tmplAddToolTimeout,
	}
	if tmplAddOAuthClientID != "" || len(tmplAddScopes) > 0 || tmplAddOAuthCBPort > 0 {
		tmpl.OAuth = &config.OAuthConfig{
			ClientID: tmplAddOAuthClientID,
			Scopes:   tmplAddScopes,
		}
		if tmplAddOAuthCBPort > 0 {
			tmpl.OAuth.CallbackPort = &tmplAddOAuthCBPort
		}
	}

	if err := cfg.AddTemplate(name, tmpl); err != nil {
		return err
	}
	if err := saveCfg(cfg); err != nil {
		return err
	}
	fmt.Printf("Added template %q\n", name)
	return nil
}

// ============================================================================
// template list
// ============================================================================

var tmplListJSON bool

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured templates",
	RunE:  runTemplateList,
}

func init() {
	templateListCmd.Flags().BoolVar(&tmplListJSON, "json", false, "Output as JSON")
	templateCmd.AddCommand(templateListCmd)
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	cfg, err := loadCfg()
	if err != nil {
		return err
	}

	entries := cfg.TemplateEntries()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	if tmplListJSON {
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(entries) == 0 {
		fmt.Println("No templates configured")
		return nil
	}

	for _, e := range entries {
		kind := "stdio"
		detail := e.Config.Command
		if e.Config.IsHTTP() {
			kind = "http"
			detail = e.Config.URL
		}
		usedBy := cfg.ServersUsingTemplate(e.Name)
		permCount := len(e.Config.ToolPermissions)
		defaultLabel := "allow"
		if e.Config.DenyByDefault {
			defaultLabel = "deny"
		}
		fmt.Printf("  %-20s  %s  %s  (used by %d servers, %d perms, default: %s)\n",
			e.Name, kind, detail, len(usedBy), permCount, defaultLabel)
	}
	return nil
}

// ============================================================================
// template remove
// ============================================================================

var tmplRemoveYes bool

var templateRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a server template",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateRemove,
}

func init() {
	templateRemoveCmd.Flags().BoolVarP(&tmplRemoveYes, "yes", "y", false, "Skip confirmation")
	templateCmd.AddCommand(templateRemoveCmd)
}

func runTemplateRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := loadCfg()
	if err != nil {
		return err
	}

	if _, ok := cfg.GetTemplate(name); !ok {
		return fmt.Errorf("template %q not found", name)
	}

	if !tmplRemoveYes {
		fmt.Printf("Remove template %q? [y/N] ", name)
		reader := bufio.NewReader(os.Stdin)
		resp, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp != "y" && resp != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if err := cfg.DeleteTemplate(name); err != nil {
		return err
	}
	if err := saveCfg(cfg); err != nil {
		return err
	}
	fmt.Printf("Removed template %q\n", name)
	return nil
}

// ============================================================================
// helpers (shared with add.go)
// ============================================================================

func loadCfg() (*config.Config, error) {
	var cfg *config.Config
	var err error
	if configPath != "" {
		cfg, err = config.LoadFrom(configPath)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

func saveCfg(cfg *config.Config) error {
	if configPath != "" {
		if err := config.SaveTo(cfg, configPath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	} else {
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	return nil
}
