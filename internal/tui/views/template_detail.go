package views

import (
	"fmt"
	"strings"

	"github.com/Bigsy/mcpmu/internal/config"
	"github.com/Bigsy/mcpmu/internal/tui/theme"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// TemplateDetailModel displays detailed information about a template.
type TemplateDetailModel struct {
	theme        theme.Theme
	templateName string
	template     *config.TemplateConfig
	usedBy       []string
	viewport     viewport.Model
	width        int
	height       int
	topPad       int
	focused      bool
}

// NewTemplateDetail creates a new template detail view.
func NewTemplateDetail(th theme.Theme) TemplateDetailModel {
	vp := viewport.New(0, 0)
	return TemplateDetailModel{
		theme:    th,
		viewport: vp,
	}
}

// SetTemplate sets the template to display.
func (m *TemplateDetailModel) SetTemplate(name string, tmpl *config.TemplateConfig, usedBy []string) {
	m.templateName = name
	m.template = tmpl
	m.usedBy = usedBy
	m.updateContent()
}

// SetSize sets the dimensions.
func (m *TemplateDetailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width - 4
	m.topPad = paneTopPaddingLines(height)
	m.viewport.Height = height - 2 - m.topPad
	if m.viewport.Width < 10 {
		m.viewport.Width = 10
	}
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}
	m.updateContent()
}

// SetFocused sets whether the view is focused.
func (m *TemplateDetailModel) SetFocused(focused bool) {
	m.focused = focused
}

func (m *TemplateDetailModel) updateContent() {
	if m.template == nil {
		m.viewport.SetContent("No template selected")
		return
	}

	var content strings.Builder

	labelStyle := m.theme.Base.Bold(true)
	infoStyle := m.theme.Muted

	content.WriteString(m.theme.Title.Render(m.templateName))
	content.WriteString("\n\n")

	if m.template.Description != "" {
		content.WriteString(infoStyle.Render(m.template.Description))
		content.WriteString("\n\n")
	}

	// Type
	if m.template.IsHTTP() {
		content.WriteString(labelStyle.Render("Type: "))
		content.WriteString(infoStyle.Render("HTTP (Streamable)"))
		content.WriteString("\n")

		content.WriteString(labelStyle.Render("URL: "))
		content.WriteString(infoStyle.Render(m.template.URL))
		content.WriteString("\n")

		if m.template.BearerTokenEnvVar != "" {
			content.WriteString(labelStyle.Render("Auth: "))
			content.WriteString(infoStyle.Render("Bearer ($" + m.template.BearerTokenEnvVar + ")"))
			content.WriteString("\n")
		}
		if m.template.OAuth != nil {
			content.WriteString(labelStyle.Render("Auth: "))
			content.WriteString(infoStyle.Render("OAuth"))
			content.WriteString("\n")
		}
	} else {
		content.WriteString(labelStyle.Render("Type: "))
		content.WriteString(infoStyle.Render("Stdio"))
		content.WriteString("\n")

		cmd := m.template.Command
		if len(m.template.Args) > 0 {
			cmd += " " + strings.Join(m.template.Args, " ")
		}
		content.WriteString(labelStyle.Render("Command: "))
		content.WriteString(infoStyle.Render(cmd))
		content.WriteString("\n")
	}

	if m.template.Cwd != "" {
		content.WriteString(labelStyle.Render("Working Dir: "))
		content.WriteString(infoStyle.Render(m.template.Cwd))
		content.WriteString("\n")
	}

	if len(m.template.TestArgs) > 0 {
		content.WriteString(labelStyle.Render("Test Args: "))
		content.WriteString(infoStyle.Render(strings.Join(m.template.TestArgs, " ")))
		content.WriteString("\n")
	}

	if m.template.StartupTimeoutSec > 0 {
		content.WriteString(labelStyle.Render("Startup Timeout: "))
		content.WriteString(infoStyle.Render(fmt.Sprintf("%ds", m.template.StartupTimeoutSec)))
		content.WriteString("\n")
	}

	if m.template.ToolTimeoutSec > 0 {
		content.WriteString(labelStyle.Render("Tool Timeout: "))
		content.WriteString(infoStyle.Render(fmt.Sprintf("%ds", m.template.ToolTimeoutSec)))
		content.WriteString("\n")
	}

	// Environment
	if len(m.template.Env) > 0 {
		content.WriteString("\n")
		content.WriteString(labelStyle.Render("Environment:\n"))
		for k, v := range m.template.Env {
			content.WriteString("  ")
			content.WriteString(infoStyle.Render(k + "=" + v))
			content.WriteString("\n")
		}
	}

	// Used by servers
	content.WriteString("\n")
	content.WriteString(labelStyle.Render(fmt.Sprintf("Used by %d server(s)", len(m.usedBy))))
	content.WriteString("\n")
	if len(m.usedBy) > 0 {
		for _, name := range m.usedBy {
			content.WriteString("  • ")
			content.WriteString(infoStyle.Render(name))
			content.WriteString("\n")
		}
	}

	// Tool permissions
	content.WriteString("\n")
	defaultLabel := "allow"
	if m.template.DenyByDefault {
		defaultLabel = "deny"
	}
	content.WriteString(m.theme.Title.Render(fmt.Sprintf("Tool Permissions (default: %s)", defaultLabel)))
	content.WriteString("\n")
	if len(m.template.ToolPermissions) == 0 {
		if m.template.DenyByDefault {
			content.WriteString(m.theme.Faint.Render("  All tools denied by default (none explicitly allowed)"))
		} else {
			content.WriteString(m.theme.Faint.Render("  All tools allowed (no explicit overrides)"))
		}
		content.WriteString("\n")
	} else {
		for tool, allowed := range m.template.ToolPermissions {
			content.WriteString("  ")
			if allowed {
				content.WriteString(m.theme.Success.Render("✓ " + tool))
			} else {
				content.WriteString(m.theme.Danger.Render("✗ " + tool))
			}
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(m.theme.Faint.Render("Press 't' to test, 'p' to edit tool permissions, 'e' to edit"))

	m.viewport.SetContent(content.String())
}

// Init implements tea.Model.
func (m TemplateDetailModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m TemplateDetailModel) Update(msg tea.Msg) (TemplateDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m TemplateDetailModel) View() string {
	title := "Template"
	if m.templateName != "" {
		title = m.templateName
	}
	content := strings.TrimSuffix(m.viewport.View(), "\n")
	if m.topPad > 0 {
		content = strings.Repeat("\n", m.topPad) + content
	}
	return m.theme.RenderPane(title, content, m.width, m.focused)
}
