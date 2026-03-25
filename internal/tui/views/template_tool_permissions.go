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
	TemplateName  string
	DisabledTools []string
	Submitted     bool
}

// templateToolItem represents a tool in the template tool permission editor.
type templateToolItem struct {
	toolName    string
	description string
	enabled     bool // true = tool is enabled (not in disabled list)
}

func (i templateToolItem) Title() string       { return i.toolName }
func (i templateToolItem) Description() string { return i.description }
func (i templateToolItem) FilterValue() string { return i.toolName }

// TemplateToolPermissionsModel is a modal for editing template disabled tools.
type TemplateToolPermissionsModel struct {
	theme        theme.Theme
	visible      bool
	list         list.Model
	width        int
	height       int
	templateName string
	discovering  bool

	// Track enabled state per tool
	toolStates map[string]bool // toolName -> enabled

	escKey   key.Binding
	enterKey key.Binding
	spaceKey key.Binding
}

// NewTemplateToolPermissions creates a new template tool permissions editor.
func NewTemplateToolPermissions(th theme.Theme) TemplateToolPermissionsModel {
	delegate := newTemplateToolDelegate(th, make(map[string]bool))
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Template Tool Filter"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = th.Title
	l.FilterInput.PromptStyle = th.Primary
	l.FilterInput.Cursor.Style = th.Primary

	return TemplateToolPermissionsModel{
		theme:      th,
		list:       l,
		toolStates: make(map[string]bool),
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

// Show displays the editor with discovered tools.
func (m *TemplateToolPermissionsModel) Show(templateName string, tools []events.McpTool, disabledTools []string) {
	m.visible = true
	m.discovering = false
	m.templateName = templateName
	m.toolStates = make(map[string]bool)

	// Build disabled tool lookup
	disabledSet := make(map[string]bool)
	for _, name := range disabledTools {
		disabledSet[name] = true
	}

	var items []list.Item
	for _, tool := range tools {
		enabled := !disabledSet[tool.Name]
		m.toolStates[tool.Name] = enabled
		items = append(items, templateToolItem{
			toolName:    tool.Name,
			description: tool.Description,
			enabled:     enabled,
		})
	}

	m.list.SetItems(items)
	m.list.SetDelegate(newTemplateToolDelegate(m.theme, m.toolStates))
}

// ShowDiscovering shows the discovering tools state.
func (m *TemplateToolPermissionsModel) ShowDiscovering(templateName string) {
	m.visible = true
	m.discovering = true
	m.templateName = templateName
	m.toolStates = make(map[string]bool)
	m.list.SetItems([]list.Item{})
}

// FinishDiscovery transitions from discovery to editing mode.
func (m *TemplateToolPermissionsModel) FinishDiscovery(tools []events.McpTool, disabledTools []string) {
	m.discovering = false

	disabledSet := make(map[string]bool)
	for _, name := range disabledTools {
		disabledSet[name] = true
	}

	var items []list.Item
	for _, tool := range tools {
		enabled := !disabledSet[tool.Name]
		m.toolStates[tool.Name] = enabled
		items = append(items, templateToolItem{
			toolName:    tool.Name,
			description: tool.Description,
			enabled:     enabled,
		})
	}

	m.list.SetItems(items)
	m.list.SetDelegate(newTemplateToolDelegate(m.theme, m.toolStates))
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
			// Build disabled tools list from current state
			var disabled []string
			for name, enabled := range m.toolStates {
				if !enabled {
					disabled = append(disabled, name)
				}
			}
			return func() tea.Msg {
				return TemplateToolPermissionsResult{
					TemplateName:  m.templateName,
					DisabledTools: disabled,
					Submitted:     true,
				}
			}

		case key.Matches(msg, m.spaceKey):
			if item := m.list.SelectedItem(); item != nil {
				ti := item.(templateToolItem)
				current := m.toolStates[ti.toolName]
				m.toolStates[ti.toolName] = !current
				m.list.SetDelegate(newTemplateToolDelegate(m.theme, m.toolStates))
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
		enabledCount := 0
		disabledCount := 0
		for _, enabled := range m.toolStates {
			if enabled {
				enabledCount++
			} else {
				disabledCount++
			}
		}
		header := fmt.Sprintf("Template Tool Filter — %d enabled, %d disabled", enabledCount, disabledCount)
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
	theme      theme.Theme
	toolStates map[string]bool
}

func newTemplateToolDelegate(th theme.Theme, states map[string]bool) templateToolDelegate {
	return templateToolDelegate{theme: th, toolStates: states}
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
	enabled := d.toolStates[item.toolName]

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

	cursor := "  "
	if isSelected {
		cursor = d.theme.Primary.Render("▸ ")
	}

	line1 := cursor + icon + " " + nameStyle.Render(item.toolName)

	desc := item.description
	if len(desc) > 55 {
		desc = desc[:52] + "..."
	}
	line2 := "      " + d.theme.Faint.Render(desc)

	fmt.Fprintf(w, "%s\n%s", line1, line2)
}
