package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Bigsy/mcpmu/internal/config"
	"github.com/Bigsy/mcpmu/internal/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// TemplateFormResult is sent when the user completes or cancels the template form.
type TemplateFormResult struct {
	Name         string
	OriginalName string
	Template     config.TemplateConfig
	Submitted    bool
	IsEdit       bool
}

// TemplateFormModel is a form for adding/editing templates.
type TemplateFormModel struct {
	theme   theme.Theme
	visible bool
	isEdit  bool
	width   int
	height  int

	form           *huh.Form
	originalName   string
	originalConfig *config.TemplateConfig

	// Form field values
	name           string
	description    string
	commandOrURL   string
	args           string
	testArgs       string
	cwd            string
	env            string
	bearerEnv      string
	oauthClientID  string
	oauthCBPort    string
	oauthScopes    string
	startupTimeout string
	toolTimeout    string
	denyByDefault  bool

	// Initial values for dirty checking
	initialName           string
	initialDescription    string
	initialCommandOrURL   string
	initialArgs           string
	initialTestArgs       string
	initialCwd            string
	initialEnv            string
	initialBearerEnv      string
	initialOAuthClientID  string
	initialOAuthCBPort    string
	initialOAuthScopes    string
	initialStartupTimeout string
	initialToolTimeout    string
	initialDenyByDefault  bool

	showConfirmDiscard bool
	escKey             key.Binding
}

// NewTemplateForm creates a new template form.
func NewTemplateForm(th theme.Theme) TemplateFormModel {
	return TemplateFormModel{
		theme: th,
		escKey: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// ShowAdd displays the form for adding a new template.
func (m *TemplateFormModel) ShowAdd() tea.Cmd {
	m.visible = true
	m.isEdit = false
	m.showConfirmDiscard = false
	m.originalName = ""
	m.originalConfig = nil
	m.name = ""
	m.description = ""
	m.commandOrURL = ""
	m.args = ""
	m.testArgs = ""
	m.cwd = ""
	m.env = ""
	m.bearerEnv = ""
	m.oauthClientID = ""
	m.oauthCBPort = ""
	m.oauthScopes = ""
	m.startupTimeout = ""
	m.toolTimeout = ""
	m.denyByDefault = false
	m.saveInitialValues()
	m.buildForm()
	return m.form.Init()
}

// ShowEdit displays the form for editing an existing template.
func (m *TemplateFormModel) ShowEdit(name string, tmpl config.TemplateConfig) tea.Cmd {
	m.visible = true
	m.isEdit = true
	m.showConfirmDiscard = false
	m.originalName = name
	m.originalConfig = &tmpl
	m.name = name
	m.description = tmpl.Description

	if tmpl.URL != "" {
		m.commandOrURL = tmpl.URL
		m.args = ""
		m.bearerEnv = tmpl.BearerTokenEnvVar
		if tmpl.OAuth != nil {
			m.oauthClientID = tmpl.OAuth.ClientID
			if tmpl.OAuth.CallbackPort != nil {
				m.oauthCBPort = fmt.Sprintf("%d", *tmpl.OAuth.CallbackPort)
			}
			m.oauthScopes = strings.Join(tmpl.OAuth.Scopes, ", ")
		}
	} else {
		m.commandOrURL = tmpl.Command
		m.args = formatArgs(tmpl.Args)
		m.bearerEnv = ""
		m.oauthClientID = ""
		m.oauthCBPort = ""
		m.oauthScopes = ""
	}

	m.testArgs = formatArgs(tmpl.TestArgs)
	m.cwd = tmpl.Cwd
	m.env = formatEnvVars(tmpl.Env)
	if tmpl.StartupTimeoutSec > 0 {
		m.startupTimeout = strconv.Itoa(tmpl.StartupTimeoutSec)
	} else {
		m.startupTimeout = ""
	}
	if tmpl.ToolTimeoutSec > 0 {
		m.toolTimeout = strconv.Itoa(tmpl.ToolTimeoutSec)
	} else {
		m.toolTimeout = ""
	}
	m.denyByDefault = tmpl.DenyByDefault

	m.saveInitialValues()
	m.buildForm()
	return m.form.Init()
}

func (m *TemplateFormModel) saveInitialValues() {
	m.initialName = m.name
	m.initialDescription = m.description
	m.initialCommandOrURL = m.commandOrURL
	m.initialArgs = m.args
	m.initialTestArgs = m.testArgs
	m.initialCwd = m.cwd
	m.initialEnv = m.env
	m.initialBearerEnv = m.bearerEnv
	m.initialOAuthClientID = m.oauthClientID
	m.initialOAuthCBPort = m.oauthCBPort
	m.initialOAuthScopes = m.oauthScopes
	m.initialStartupTimeout = m.startupTimeout
	m.initialToolTimeout = m.toolTimeout
	m.initialDenyByDefault = m.denyByDefault
}

func (m *TemplateFormModel) isDirty() bool {
	return m.name != m.initialName ||
		m.description != m.initialDescription ||
		m.commandOrURL != m.initialCommandOrURL ||
		m.args != m.initialArgs ||
		m.testArgs != m.initialTestArgs ||
		m.cwd != m.initialCwd ||
		m.env != m.initialEnv ||
		m.bearerEnv != m.initialBearerEnv ||
		m.oauthClientID != m.initialOAuthClientID ||
		m.oauthCBPort != m.initialOAuthCBPort ||
		m.oauthScopes != m.initialOAuthScopes ||
		m.startupTimeout != m.initialStartupTimeout ||
		m.toolTimeout != m.initialToolTimeout ||
		m.denyByDefault != m.initialDenyByDefault
}

func (m *TemplateFormModel) buildForm() {
	keymap := huh.NewDefaultKeyMap()
	keymap.Input.Prev.SetKeys("up", "shift+tab")
	keymap.Input.Next.SetKeys("down", "tab")
	keymap.Text.Prev.SetKeys("up", "shift+tab")
	keymap.Text.Next.SetKeys("down", "tab")
	keymap.Confirm.Prev.SetKeys("up", "shift+tab")
	keymap.Confirm.Next.SetKeys("down", "tab")

	formTheme := huh.ThemeBase16()
	orange := lipgloss.AdaptiveColor{Light: "#EA580C", Dark: "#FB923C"}
	formTheme.Focused.Title = formTheme.Focused.Title.Foreground(orange)
	formTheme.Blurred.Title = formTheme.Blurred.Title.Foreground(orange)

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name").
				Description("Template name").
				Value(&m.name).
				Validate(huh.ValidateNotEmpty()),
			huh.NewInput().
				Title("Description").
				Description("What this template is for").
				Value(&m.description),
			huh.NewInput().
				Title("Command or URL").
				Description("Command to run, or https:// URL for HTTP server").
				Value(&m.commandOrURL).
				Validate(huh.ValidateNotEmpty()),
			huh.NewInput().
				Title("Arguments").
				Description("Space-separated base args").
				Value(&m.args),
			huh.NewInput().
				Title("Test Arguments").
				Description("Extra args for tool discovery (e.g. connection params)").
				Value(&m.testArgs),
			huh.NewInput().
				Title("Working Directory").
				Value(&m.cwd),
			huh.NewText().
				Title("Environment Variables").
				Description("One per line: KEY=value").
				Value(&m.env).
				CharLimit(1000).
				Lines(2),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Bearer Token Env Var").
				Description("HTTP only").
				Value(&m.bearerEnv),
			huh.NewInput().
				Title("OAuth Client ID").
				Description("HTTP only").
				Value(&m.oauthClientID),
			huh.NewInput().
				Title("OAuth Callback Port").
				Value(&m.oauthCBPort),
			huh.NewInput().
				Title("OAuth Scopes").
				Description("Comma-separated").
				Value(&m.oauthScopes),
			huh.NewInput().
				Title("Startup Timeout (sec)").
				Placeholder("10").
				Value(&m.startupTimeout).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return nil
					}
					n, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if n < 1 {
						return fmt.Errorf("must be at least 1")
					}
					return nil
				}),
			huh.NewInput().
				Title("Tool Timeout (sec)").
				Placeholder("60").
				Value(&m.toolTimeout).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return nil
					}
					n, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if n < 1 {
						return fmt.Errorf("must be at least 1")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Deny by Default").
				Description("Deny tools unless explicitly allowed").
				Value(&m.denyByDefault),
		).Title("Advanced"),
	).WithTheme(formTheme).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true).
		WithKeyMap(keymap)
}

func (m *TemplateFormModel) buildTemplateConfig() config.TemplateConfig {
	var tmpl config.TemplateConfig

	if m.isEdit && m.originalConfig != nil {
		tmpl = *m.originalConfig
	}

	tmpl.Description = strings.TrimSpace(m.description)

	if isHTTPURL(m.commandOrURL) {
		tmpl.Command = ""
		tmpl.Args = nil
		tmpl.URL = strings.TrimSpace(m.commandOrURL)
		tmpl.BearerTokenEnvVar = strings.TrimSpace(m.bearerEnv)

		clientID := strings.TrimSpace(m.oauthClientID)
		scopesStr := strings.TrimSpace(m.oauthScopes)
		cbPort := strings.TrimSpace(m.oauthCBPort)
		if clientID != "" || scopesStr != "" || cbPort != "" {
			tmpl.OAuth = &config.OAuthConfig{ClientID: clientID}
			if scopesStr != "" {
				tmpl.OAuth.Scopes = splitAndTrim(scopesStr)
			}
			if cbPort != "" {
				if p, err := strconv.Atoi(cbPort); err == nil {
					tmpl.OAuth.CallbackPort = &p
				}
			}
		} else {
			tmpl.OAuth = nil
		}
	} else {
		tmpl.URL = ""
		tmpl.BearerTokenEnvVar = ""
		tmpl.OAuth = nil
		tmpl.Command = strings.TrimSpace(m.commandOrURL)
		tmpl.Args = parseArgs(strings.TrimSpace(m.args))
	}

	tmpl.Cwd = strings.TrimSpace(m.cwd)
	tmpl.Env = parseEnvVars(strings.TrimSpace(m.env))
	tmpl.TestArgs = parseArgs(strings.TrimSpace(m.testArgs))
	tmpl.DenyByDefault = m.denyByDefault

	if s := strings.TrimSpace(m.startupTimeout); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			tmpl.StartupTimeoutSec = n
		}
	} else {
		tmpl.StartupTimeoutSec = 0
	}
	if s := strings.TrimSpace(m.toolTimeout); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			tmpl.ToolTimeoutSec = n
		}
	} else {
		tmpl.ToolTimeoutSec = 0
	}

	return tmpl
}

// Hide hides the form.
func (m *TemplateFormModel) Hide() {
	m.visible = false
	m.form = nil
}

// IsVisible returns whether the form is visible.
func (m TemplateFormModel) IsVisible() bool {
	return m.visible
}

// SetSize sets the available size for the form.
func (m *TemplateFormModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages for the form.
func (m *TemplateFormModel) Update(msg tea.Msg) tea.Cmd {
	if !m.visible || m.form == nil {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showConfirmDiscard {
			switch msg.String() {
			case "y", "Y", "enter":
				m.visible = false
				m.showConfirmDiscard = false
				tmpl := m.buildTemplateConfig()
				name := strings.TrimSpace(m.name)
				originalName := m.originalName
				isEdit := m.isEdit
				return func() tea.Msg {
					return TemplateFormResult{
						Name:         name,
						OriginalName: originalName,
						Template:     tmpl,
						Submitted:    true,
						IsEdit:       isEdit,
					}
				}
			case "n", "N":
				m.visible = false
				m.showConfirmDiscard = false
				return func() tea.Msg {
					return TemplateFormResult{Submitted: false}
				}
			case "esc", "c", "C":
				m.showConfirmDiscard = false
				return nil
			}
			return nil
		}

		if key.Matches(msg, m.escKey) {
			if m.isDirty() {
				m.showConfirmDiscard = true
				return nil
			}
			m.visible = false
			return func() tea.Msg {
				return TemplateFormResult{Submitted: false}
			}
		}
	}

	if m.showConfirmDiscard {
		return nil
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		m.visible = false
		tmpl := m.buildTemplateConfig()
		name := strings.TrimSpace(m.name)
		originalName := m.originalName
		isEdit := m.isEdit
		return func() tea.Msg {
			return TemplateFormResult{
				Name:         name,
				OriginalName: originalName,
				Template:     tmpl,
				Submitted:    true,
				IsEdit:       isEdit,
			}
		}
	}

	if m.form.State == huh.StateAborted {
		m.visible = false
		return func() tea.Msg {
			return TemplateFormResult{Submitted: false}
		}
	}

	return cmd
}

// RenderOverlay renders the form as an overlay on top of the base content.
func (m TemplateFormModel) RenderOverlay(base string, width, height int) string {
	if !m.visible || m.form == nil {
		return base
	}

	formView := m.form.View()

	title := "Add Template"
	if m.isEdit {
		title = "Edit Template"
	}

	if m.showConfirmDiscard {
		formView += "\n\n" + m.theme.Warn.Render("Unsaved changes. [y]save [n]discard [esc]cancel")
	}

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary.GetForeground()).
		Padding(1, 2).
		Width(66).
		Render(m.theme.Title.Render(title) + "\n\n" + formView)

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#1F2937"}),
	)
}

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
