package preview

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/feedtypes"
)

// ViewMode represents the current view mode
type ViewMode int

// View modes for the preview TUI
const (
	ListViewMode ViewMode = iota
	DetailViewMode
	XMLViewMode
)

// Model represents the Bubble Tea model for the preview TUI
type Model struct {
	items         []feedtypes.FeedItem
	cursor        int
	viewMode      ViewMode
	providerName  string
	templateName  string
	feedConfig    feed.Config
	width         int
	height        int
	selectedIndex int // Index of the item currently being viewed in detail
}

// NewModel creates a new preview model
func NewModel(items []feedtypes.FeedItem, providerName, templateName string, feedConfig feed.Config) Model {
	return Model{
		items:         items,
		cursor:        0,
		viewMode:      ListViewMode,
		providerName:  providerName,
		templateName:  templateName,
		feedConfig:    feedConfig,
		selectedIndex: -1,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.viewMode {
		case ListViewMode:
			return m.updateListView(msg)
		case DetailViewMode, XMLViewMode:
			return m.updateDetailView(msg)
		}
	}

	return m, nil
}

// updateListView handles key presses in list view mode
func (m Model) updateListView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}

	case "enter":
		m.selectedIndex = m.cursor
		m.viewMode = DetailViewMode

	case "x":
		m.selectedIndex = m.cursor
		m.viewMode = XMLViewMode
	}

	return m, nil
}

// updateDetailView handles key presses in detail/XML view modes
func (m Model) updateDetailView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.viewMode = ListViewMode

	case "x":
		// Toggle between detail and XML views
		if m.viewMode == DetailViewMode {
			m.viewMode = XMLViewMode
		} else {
			m.viewMode = DetailViewMode
		}
	}

	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	switch m.viewMode {
	case ListViewMode:
		return m.renderListView()
	case DetailViewMode:
		return m.renderDetailView()
	case XMLViewMode:
		return m.renderXMLView()
	}
	return ""
}

// renderListView renders the list view
func (m Model) renderListView() string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	header := fmt.Sprintf("Feed Preview - %s (%d items)", m.providerName, len(m.items))
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n\n")

	// Items list
	visibleStart := 0
	visibleEnd := len(m.items)

	// Calculate visible range if height is set
	if m.height > 0 {
		maxVisible := m.height - 6 // Account for header, footer, and padding
		if maxVisible < len(m.items) {
			// Keep cursor in the middle of the screen when possible
			visibleStart = m.cursor - maxVisible/2
			if visibleStart < 0 {
				visibleStart = 0
			}
			visibleEnd = visibleStart + maxVisible
			if visibleEnd > len(m.items) {
				visibleEnd = len(m.items)
				visibleStart = visibleEnd - maxVisible
				if visibleStart < 0 {
					visibleStart = 0
				}
			}
		}
	}

	for i := visibleStart; i < visibleEnd; i++ {
		item := m.items[i]
		line := FormatCompactListItem(i, item)

		if i == m.cursor {
			// Highlight selected item
			selectedStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("12")).
				Bold(true)
			b.WriteString(selectedStyle.Render("→ " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	footer := "↑/↓ or j/k: navigate • enter: view details • x: XML view • q: quit"
	b.WriteString(footerStyle.Render(footer))

	return b.String()
}

// renderDetailView renders the detail view
func (m Model) renderDetailView() string {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.items) {
		return "No item selected"
	}

	item := m.items[m.selectedIndex]
	content := FormatDetailedItem(item)

	var b strings.Builder
	b.WriteString(content)
	b.WriteString("\n")

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	footer := "esc: back to list • x: toggle XML view • q: quit"
	b.WriteString(footerStyle.Render(footer))

	return b.String()
}

// renderXMLView renders the XML view
func (m Model) renderXMLView() string {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.items) {
		return "No item selected"
	}

	item := m.items[m.selectedIndex]
	content := FormatXMLItem(item, m.templateName, m.feedConfig)

	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	b.WriteString(headerStyle.Render("XML Entry Preview"))
	b.WriteString("\n\n")
	b.WriteString(content)
	b.WriteString("\n")

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	footer := "esc: back to list • x: toggle detail view • q: quit"
	b.WriteString(footerStyle.Render(footer))

	return b.String()
}

// Run starts the Bubble Tea program
func Run(items []feedtypes.FeedItem, providerName, templateName string, feedConfig feed.Config) error {
	if len(items) == 0 {
		fmt.Println("No items to preview")
		return nil
	}

	p := tea.NewProgram(NewModel(items, providerName, templateName, feedConfig), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
