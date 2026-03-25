package views

import (
	"fmt"
	"io"

	"github.com/Bigsy/mcpmu/internal/events"
	"github.com/Bigsy/mcpmu/internal/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TemplateToolPermissionsResult is sent when the user finishes editing template tool permissions.
type TemplateToolPermissionsResult struct {
	TemplateName string
	// Changes contains permission changes (toolName -> enabled).
	Changes map[string]bool
	// Deletions contains tool names whose explicit permission should be removed
	// (revert to template default).
	Deletions []string
	Submitted bool
}

// templateToolItem represents a tool in the template tool permission editor.
type templateToolItem struct {
	toolName    string
	description string
	enabled     bool // resolved enabled state for display
}

func (i templateToolItem) Title() string       { return i.toolName }
func (i templateToolItem) Description() string { return i.description }
func (i templateToolItem) FilterValue() string { return i.toolName }

// TemplateToolPermissionsModel is a modal for editing template tool permissions.
type TemplateToolPermissionsModel struct {
	theme        theme.Theme
	visible      bool
	list         list.Model
	width        int
	height       int
	templateName string
	discovering  bool

	// Permission state (mirrors the namespace pattern)
	originalPerms map[string]bool // toolName -> enabled (explicit permissions at open time)
	currentPerms  map[string]bool // toolName -> enabled (current working copy)
	denyByDefault bool            // template-level default

	escKey   key.Binding
	enterKey key.Binding
	spaceKey key.Binding
}

// NewTemplateToolPermissions creates a new template tool permissions editor.
func NewTemplateToolPermissions(th theme.Theme) TemplateToolPermissionsModel {
	delegate := newTemplateToolDelegate(th, make(map[string]bool), false)
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Template Tool Permissions"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = th.Title
	l.FilterInput.PromptStyle = th.Primary
	l.FilterInput.Cursor.Style = th.Primary

	return TemplateToolPermissionsModel{
		theme:         th,
		list:          l,
		originalPerms: make(map[string]bool),
		currentPerms:  make(map[string]bool),
		escKey: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		enterKey: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "save"),
		),
		spaceKey: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
	}
}

// defaultAllowed returns whether tools are allowed by default in this template.
func (m *TemplateToolPermissionsModel) defaultAllowed() bool {
	return !m.denyByDefault
}

// Show displays the editor with discovered tools.
func (m *TemplateToolPermissionsModel) Show(templateName string, tools []events.McpTool, permissions map[string]bool, denyByDefault bool) {
	m.visible = true
	m.discovering = false
	m.templateName = templateName
	m.denyByDefault = denyByDefault
	m.originalPerms = make(map[string]bool)
	m.currentPerms = make(map[string]bool)

	// Copy explicit permissions
	for name, enabled := range permissions {
		m.originalPerms[name] = enabled
		m.currentPerms[name] = enabled
	}

	m.populateList(tools)
}

// ShowDiscovering shows the discovering tools state.
func (m *TemplateToolPermissionsModel) ShowDiscovering(templateName string) {
	m.visible = true
	m.discovering = true
	m.templateName = templateName
	m.originalPerms = make(map[string]bool)
	m.currentPerms = make(map[string]bool)
	m.list.SetItems([]list.Item{})
}

// FinishDiscovery transitions from discovery to editing mode.
func (m *TemplateToolPermissionsModel) FinishDiscovery(tools []events.McpTool, permissions map[string]bool, denyByDefault bool) {
	m.discovering = false
	m.denyByDefault = denyByDefault

	// Copy explicit permissions
	for name, enabled := range permissions {
		m.originalPerms[name] = enabled
		m.currentPerms[name] = enabled
	}

	m.populateList(tools)
}

func (m *TemplateToolPermissionsModel) populateList(tools []events.McpTool) {
	var items []list.Item
	for _, tool := range tools {
		enabled, hasExplicit := m.currentPerms[tool.Name]
		if !hasExplicit {
			enabled = m.defaultAllowed()
		}
		items = append(items, templateToolItem{
			toolName:    tool.Name,
			description: tool.Description,
			enabled:     enabled,
		})
	}

	m.list.SetItems(items)
	m.list.SetDelegate(newTemplateToolDelegate(m.theme, m.currentPerms, m.denyByDefault))
}

// Hide hides the editor.
func (m *TemplateToolPermissionsModel) Hide() {
	m.visible = false
	m.discovering = false
}

// IsVisible returns whether the editor is visible.
func (m TemplateToolPermissionsModel) IsVisible() bool {
	return m.visible
}

// IsDiscovering returns whether the editor is in discovery mode.
func (m TemplateToolPermissionsModel) IsDiscovering() bool {
	return m.discovering
}

// GetTemplateName returns the template name being edited.
func (m TemplateToolPermissionsModel) GetTemplateName() string {
	return m.templateName
}

// SetSize sets the available size.
func (m *TemplateToolPermissionsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	editorWidth := 70
	if width < 80 {
		editorWidth = width - 10
	}
	editorHeight := 25
	if height < 30 {
		editorHeight = height - 5
	}
	m.list.SetSize(editorWidth-6, editorHeight-6)
}

// Update handles messages.
func (m *TemplateToolPermissionsModel) Update(msg tea.Msg) tea.Cmd {
	if !m.visible {
		return nil
	}

	if m.discovering {
		if msg, ok := msg.(tea.KeyMsg); ok && key.Matches(msg, m.escKey) {
			m.visible = false
			m.discovering = false
			return func() tea.Msg {
				return TemplateToolPermissionsResult{
					TemplateName: m.templateName,
					Submitted:    false,
				}
			}
		}
		return nil
	}

	// When filtering, let the list handle keys
	if m.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.escKey):
			if m.list.FilterState() == list.FilterApplied {
				m.list.ResetFilter()
				return nil
			}
			m.visible = false
			return func() tea.Msg {
				return TemplateToolPermissionsResult{
					TemplateName: m.templateName,
					Submitted:    false,
				}
			}

		case key.Matches(msg, m.enterKey):
			m.visible = false
			// Calculate changes and deletions (same pattern as namespace permissions)
			changes := make(map[string]bool)
			var deletions []string

			for toolName, enabled := range m.currentPerms {
				orig, hadOrig := m.originalPerms[toolName]
				if !hadOrig || orig != enabled {
					changes[toolName] = enabled
				}
			}

			for toolName := range m.originalPerms {
				if _, stillExists := m.currentPerms[toolName]; !stillExists {
					deletions = append(deletions, toolName)
				}
			}

			return func() tea.Msg {
				return TemplateToolPermissionsResult{
					TemplateName: m.templateName,
					Changes:      changes,
					Deletions:    deletions,
					Submitted:    true,
				}
			}

		case key.Matches(msg, m.spaceKey):
			if item := m.list.SelectedItem(); item != nil {
				ti := item.(templateToolItem)
				current, has := m.currentPerms[ti.toolName]
				if !has {
					current = m.defaultAllowed()
				}
				newValue := !current
				// If new value matches default, remove explicit permission
				if newValue == m.defaultAllowed() {
					delete(m.currentPerms, ti.toolName)
				} else {
					m.currentPerms[ti.toolName] = newValue
				}
				m.list.SetDelegate(newTemplateToolDelegate(m.theme, m.currentPerms, m.denyByDefault))
			}
			return nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return cmd
}

// RenderOverlay renders the editor as an overlay.
func (m TemplateToolPermissionsModel) RenderOverlay(base string, width, height int) string {
	if !m.visible {
		return base
	}

	editorWidth := 70
	if width < 80 {
		editorWidth = width - 10
	}

	var content string
	if m.discovering {
		content = m.theme.Primary.Render("Discovering tools...") +
			"\n\n" + m.theme.Faint.Render("Starting template server to find available tools.\nPress Esc to cancel.")
	} else {
		allowedCount := 0
		deniedCount := 0
		for _, item := range m.list.Items() {
			ti := item.(templateToolItem)
			enabled, has := m.currentPerms[ti.toolName]
			if !has {
				enabled = m.defaultAllowed()
			}
			if enabled {
				allowedCount++
			} else {
				deniedCount++
			}
		}
		defaultLabel := "allow"
		if m.denyByDefault {
			defaultLabel = "deny"
		}
		header := fmt.Sprintf("Template Tool Permissions — %d allowed, %d denied (default: %s)", allowedCount, deniedCount, defaultLabel)
		content = m.theme.Title.Render(header) + "\n" +
			m.theme.Faint.Render("space:toggle  /:filter  enter:save  esc:cancel") +
			"\n\n" + m.list.View()
	}

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary.GetForeground()).
		Padding(1, 2).
		Width(editorWidth).
		Render(content)

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#1F2937"}),
	)
}

// templateToolDelegate renders template tool items.
type templateToolDelegate struct {
	theme         theme.Theme
	currentPerms  map[string]bool
	denyByDefault bool
}

func newTemplateToolDelegate(th theme.Theme, perms map[string]bool, denyByDefault bool) templateToolDelegate {
	return templateToolDelegate{theme: th, currentPerms: perms, denyByDefault: denyByDefault}
}

func (d templateToolDelegate) Height() int                             { return 2 }
func (d templateToolDelegate) Spacing() int                            { return 0 }
func (d templateToolDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d templateToolDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(templateToolItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	// Resolve effective enabled state
	enabled, hasExplicit := d.currentPerms[item.toolName]
	if !hasExplicit {
		enabled = !d.denyByDefault
	}

	// Status icon
	var icon string
	var nameStyle lipgloss.Style
	if enabled {
		icon = d.theme.Success.Render("✓")
		if isSelected {
			nameStyle = d.theme.Primary.Bold(true)
		} else {
			nameStyle = d.theme.Base
		}
	} else {
		icon = d.theme.Danger.Render("✗")
		nameStyle = d.theme.Faint
	}

	// Show marker for explicit vs default
	marker := " "
	if hasExplicit {
		marker = "•"
	}

	cursor := "  "
	if isSelected {
		cursor = d.theme.Primary.Render("▸ ")
	}

	line1 := cursor + icon + marker + nameStyle.Render(item.toolName)

	desc := item.description
	if len(desc) > 55 {
		desc = desc[:52] + "..."
	}
	line2 := "      " + d.theme.Faint.Render(desc)

	fmt.Fprintf(w, "%s\n%s", line1, line2)
}
