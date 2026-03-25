package views

import (
	"fmt"
	"io"
	"strings"

	"github.com/Bigsy/mcpmu/internal/config"
	"github.com/Bigsy/mcpmu/internal/tui/theme"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TemplateItem represents a template in the list.
type TemplateItem struct {
	Name          string
	Config        config.TemplateConfig
	UsedByServers []string
	DisabledCount int
}

func (i TemplateItem) Title() string       { return i.Name }
func (i TemplateItem) Description() string { return i.Config.Description }
func (i TemplateItem) FilterValue() string { return i.Name }

// TemplateListModel is the template list view component.
type TemplateListModel struct {
	list    list.Model
	theme   theme.Theme
	width   int
	height  int
	topPad  int
	focused bool
}

// NewTemplateList creates a new template list view.
func NewTemplateList(th theme.Theme) TemplateListModel {
	delegate := newTemplateDelegate(th)
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetStatusBarItemName("template", "templates")

	return TemplateListModel{
		list:    l,
		theme:   th,
		focused: true,
	}
}

// SetItems updates the template list items.
func (m *TemplateListModel) SetItems(items []TemplateItem) {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}
	m.list.SetItems(listItems)
}

// SetSize sets the dimensions of the list.
func (m *TemplateListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	listWidth := width - 4
	m.topPad = paneTopPaddingLines(height)
	listHeight := height - 2 - m.topPad
	if listWidth < 10 {
		listWidth = 10
	}
	if listHeight < 3 {
		m.topPad = max(height-2-3, 0)
		listHeight = 3
	}
	if listHeight < 3 {
		listHeight = 3
	}
	m.list.SetSize(listWidth, listHeight)
}

// SetFocused sets whether the list is focused.
func (m *TemplateListModel) SetFocused(focused bool) {
	m.focused = focused
}

// SelectedItem returns the currently selected template.
func (m *TemplateListModel) SelectedItem() *TemplateItem {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}
	ti := item.(TemplateItem)
	return &ti
}

// Init implements tea.Model.
func (m TemplateListModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m TemplateListModel) Update(msg tea.Msg) (TemplateListModel, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m TemplateListModel) View() string {
	content := m.list.View()
	if len(m.list.Items()) == 0 {
		content = m.theme.Faint.Render("    ○\n  No templates configured\n\n  Press 'a' to add your first template")
	}
	if m.topPad > 0 {
		content = strings.Repeat("\n", m.topPad) + content
	}
	return m.theme.RenderPane("Templates", content, m.width, m.focused)
}

// templateDelegate renders template list items.
type templateDelegate struct {
	theme theme.Theme
}

func newTemplateDelegate(th theme.Theme) templateDelegate {
	return templateDelegate{theme: th}
}

func (d templateDelegate) Height() int                             { return 2 }
func (d templateDelegate) Spacing() int                            { return 0 }
func (d templateDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d templateDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(TemplateItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	// Name
	name := item.Name
	if isSelected {
		name = d.theme.Primary.Bold(true).Render(name)
	} else {
		name = d.theme.Base.Render(name)
	}

	// Kind
	kind := "stdio"
	if item.Config.IsHTTP() {
		kind = "http"
	}
	kindBadge := d.theme.Faint.Render(fmt.Sprintf("[%s]", kind))

	// Usage count
	usageBadge := ""
	if len(item.UsedByServers) > 0 {
		usageBadge = d.theme.Muted.Render(fmt.Sprintf(" (%d servers)", len(item.UsedByServers)))
	}

	// Disabled tools count
	disabledBadge := ""
	if item.DisabledCount > 0 {
		disabledBadge = d.theme.Warn.Render(fmt.Sprintf(" %d disabled", item.DisabledCount))
	}

	// Detail line
	detail := item.Config.Command
	if item.Config.IsHTTP() {
		detail = item.Config.URL
	}
	if len(detail) > 50 {
		detail = detail[:47] + "..."
	}

	cursor := "  "
	if isSelected {
		cursor = d.theme.Primary.Render("▸ ")
	}

	line1 := cursor + name + " " + kindBadge + usageBadge + disabledBadge
	line2 := "    " + d.theme.Faint.Render(detail)

	fmt.Fprintf(w, "%s\n%s", line1, line2)
}

// lipgloss is imported but used via theme — keep it accessible.
var _ = lipgloss.NewStyle
