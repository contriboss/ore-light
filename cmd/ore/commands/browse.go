package commands

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("235"))

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

type groupedGem struct {
	name     string
	versions []string
	paths    []string // Paths for each version
}

type item struct {
	gem groupedGem
}

func (i item) Title() string       { return i.gem.name }
func (i item) Description() string { return strings.Join(i.gem.versions, ", ") }
func (i item) FilterValue() string { return i.gem.name }

type model struct {
	list        list.Model
	gems        []GemInfo    // Original ungrouped gems
	groupedGems []groupedGem // Grouped by name
	searchInput textinput.Model
	searchMode  bool
	width       int
	height      int
	message     string
	quitting    bool
	openPath    string // Path to open in editor after quitting
}

type keyMap struct {
	Open   key.Binding
	Info   key.Binding
	Why    key.Binding
	Search key.Binding
	Quit   key.Binding
}

var keys = keyMap{
	Open: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open"),
	),
	Info: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "info"),
	),
	Why: key.NewBinding(
		key.WithKeys("w"),
		key.WithHelp("w", "why"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c", "esc"),
		key.WithHelp("q", "quit"),
	),
}

func initialModel(gems []GemInfo) model {
	// Group gems by name
	grouped := groupGemsByName(gems)

	// Create list items
	items := make([]list.Item, len(grouped))
	for i, gem := range grouped {
		items[i] = item{gem: gem}
	}

	// Create list
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	delegate.Styles.NormalTitle = normalItemStyle

	l := list.New(items, delegate, 0, 0)
	l.Title = "Installed Gems"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	// Create search input
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 50

	return model{
		list:        l,
		gems:        gems,
		groupedGems: grouped,
		searchInput: ti,
		searchMode:  false,
	}
}

// groupGemsByName groups gems by name and collects all versions
func groupGemsByName(gems []GemInfo) []groupedGem {
	// Map of gem name -> grouped gem
	gemMap := make(map[string]*groupedGem)

	for _, gem := range gems {
		if g, exists := gemMap[gem.Name]; exists {
			g.versions = append(g.versions, gem.Version)
			g.paths = append(g.paths, gem.Path)
		} else {
			gemMap[gem.Name] = &groupedGem{
				name:     gem.Name,
				versions: []string{gem.Version},
				paths:    []string{gem.Path},
			}
		}
	}

	// Convert map to sorted slice
	var result []groupedGem
	for _, g := range gemMap {
		result = append(result, *g)
	}

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})

	return result
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v-4)
		return m, nil

	case tea.KeyMsg:
		if m.searchMode {
			return m.handleSearchMode(msg)
		}
		return m.handleNormalMode(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.searchMode = false
		m.searchInput.SetValue("")
		m.filterGems("")
		return m, nil

	case tea.KeyEnter:
		m.searchMode = false
		// Keep current filter active
		return m, nil
	}

	// Update search input
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)

	// Filter in real-time as user types
	query := m.searchInput.Value()
	m.filterGems(query)

	return m, cmd
}

func (m model) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit

	case key.Matches(msg, keys.Search):
		m.searchMode = true
		m.searchInput.Focus()
		return m, textinput.Blink

	case key.Matches(msg, keys.Open):
		if selected := m.getSelectedGem(); selected != nil {
			// Open first version's path
			if len(selected.paths) > 0 {
				m.openPath = selected.paths[0]
				m.quitting = true
				return m, tea.Quit
			}
		}
		return m, nil

	case key.Matches(msg, keys.Info):
		if selected := m.getSelectedGem(); selected != nil {
			versions := strings.Join(selected.versions, ", ")
			m.message = fmt.Sprintf("Info for %s (%s)", selected.name, versions)
		}
		return m, nil

	case key.Matches(msg, keys.Why):
		if selected := m.getSelectedGem(); selected != nil {
			m.message = fmt.Sprintf("Why %s is installed...", selected.name)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) filterGems(query string) {
	if query == "" {
		// Show all gems
		items := make([]list.Item, len(m.groupedGems))
		for i, gem := range m.groupedGems {
			items[i] = item{gem: gem}
		}
		m.list.SetItems(items)
		m.list.Title = "Installed Gems"
		return
	}

	// Filter gems
	query = strings.ToLower(query)
	var filtered []groupedGem
	for _, gem := range m.groupedGems {
		if strings.Contains(strings.ToLower(gem.name), query) {
			filtered = append(filtered, gem)
		}
	}

	items := make([]list.Item, len(filtered))
	for i, gem := range filtered {
		items[i] = item{gem: gem}
	}
	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("Installed Gems (filter: %q)", query)
}

func (m *model) getSelectedGem() *groupedGem {
	if selectedItem := m.list.SelectedItem(); selectedItem != nil {
		if i, ok := selectedItem.(item); ok {
			return &i.gem
		}
	}
	return nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var view strings.Builder

	// Main list
	view.WriteString(m.list.View())
	view.WriteString("\n")

	// Search input (if active)
	if m.searchMode {
		view.WriteString("\n")
		view.WriteString(m.searchInput.View())
		view.WriteString("\n")
	}

	// Status bar
	statusBar := m.renderStatusBar()
	view.WriteString(statusBar)

	// Message (if any)
	if m.message != "" {
		view.WriteString("\n")
		view.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Render(m.message))
	}

	return appStyle.Render(view.String())
}

func (m model) renderStatusBar() string {
	selected := m.getSelectedGem()
	var selectedInfo string
	if selected != nil {
		versions := strings.Join(selected.versions, ", ")
		selectedInfo = fmt.Sprintf(" %s (%s) ", selected.name, versions)
	}

	helpText := " / search • o open • i info • w why • q quit "
	if m.searchMode {
		helpText = " Type to filter • Enter to keep • Esc to clear "
	}

	width := m.width
	if width == 0 {
		width = 80
	}

	bar := statusBarStyle.
		Width(width).
		Render(selectedInfo + strings.Repeat(" ", max(0, width-len(selectedInfo)-len(helpText))) + helpText)

	return bar
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RunBrowse starts the interactive TUI for browsing gems
func RunBrowse() error {
	// Get all installed gems
	gemDir, err := getGemDirectory()
	if err != nil {
		return fmt.Errorf("failed to get gem directory: %w", err)
	}

	gems, err := findInstalledGems(gemDir)
	if err != nil {
		return fmt.Errorf("failed to find installed gems: %w", err)
	}

	if len(gems) == 0 {
		return fmt.Errorf("no gems found")
	}

	// Start TUI
	p := tea.NewProgram(initialModel(gems), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	// Check if user wants to open a gem in editor
	if m, ok := finalModel.(model); ok && m.openPath != "" {
		editor := getEditor()
		if editor == "" {
			return fmt.Errorf("no editor found. Set $EDITOR, $VISUAL, or $BUNDLER_EDITOR")
		}

		cmd := exec.Command(editor, m.openPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return nil
}
