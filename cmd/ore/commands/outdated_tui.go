package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	outdatedTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Bold(true).
				Padding(0, 1)

	outdatedStatusBar = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Background(lipgloss.Color("235"))

	checkboxSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")). // Green
				Bold(true)

	checkboxUnselectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	// Update type colors
	majorUpdateStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")). // Red
				Bold(true)

	minorUpdateStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("11")). // Yellow
				Bold(true)

	patchUpdateStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")). // Green
				Bold(true)

	versionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246"))

	constraintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
)

// tableRow represents a row in the table
type tableRow struct {
	gemIndex int // -1 for headers
	isHeader bool
}

type outdatedModel struct {
	table           table.Model
	gems            []OutdatedGem
	rows            []tableRow // Map table rows to gem indices
	width           int
	height          int
	showPreview     bool
	quitting        bool
	filterGroup     string   // Empty = all groups
	availableGroups []string // All groups present in gems
}

type outdatedKeyMap struct {
	Toggle      key.Binding
	SelectPatch key.Binding
	SelectMinor key.Binding
	SelectMajor key.Binding
	SelectAll   key.Binding
	SelectNone  key.Binding
	CycleGroup  key.Binding
	Update      key.Binding
	Quit        key.Binding
}

var outdatedKeys = outdatedKeyMap{
	Toggle: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle"),
	),
	SelectPatch: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "select patches"),
	),
	SelectMinor: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "select minors"),
	),
	SelectMajor: key.NewBinding(
		key.WithKeys("M"),
		key.WithHelp("M", "select majors"),
	),
	SelectAll: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "select all"),
	),
	SelectNone: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "select none"),
	),
	CycleGroup: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "cycle groups"),
	),
	Update: key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "update selected"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c", "esc"),
		key.WithHelp("q", "quit"),
	),
}

// buildTableRows creates table rows from gems with inline group display
func buildTableRows(gems []OutdatedGem) ([]table.Row, []tableRow) {
	var rows []table.Row
	var rowMapping []tableRow

	for i, gem := range gems {
		// Checkbox
		var checkbox string
		if gem.Selected {
			checkbox = checkboxSelectedStyle.Render("[✓]")
		} else {
			checkbox = checkboxUnselectedStyle.Render("[ ]")
		}

		// Update type with color
		var updateTypeStr string
		switch gem.UpdateType {
		case UpdateMajor:
			updateTypeStr = majorUpdateStyle.Render("MAJOR")
		case UpdateMinor:
			updateTypeStr = minorUpdateStyle.Render("MINOR")
		case UpdatePatch:
			updateTypeStr = patchUpdateStyle.Render("PATCH")
		default:
			updateTypeStr = "?"
		}

		// Version change
		versionChange := versionStyle.Render(fmt.Sprintf("%s → %s", gem.CurrentVersion, gem.LatestVersion))

		// Constraint
		constraint := ""
		if gem.Constraint != "" {
			constraint = constraintStyle.Render(gem.Constraint)
		}

		// Groups (show primary or first few)
		groupsStr := strings.Join(gem.Groups, ", ")
		if groupsStr == "" {
			groupsStr = "default"
		}
		groupsDisplay := lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Render(groupsStr)

		rows = append(rows, table.Row{
			checkbox,
			gem.Name,
			versionChange,
			updateTypeStr,
			constraint,
			groupsDisplay,
		})
		rowMapping = append(rowMapping, tableRow{gemIndex: i, isHeader: false})
	}

	return rows, rowMapping
}

// collectAvailableGroups extracts all unique groups from gems
func collectAvailableGroups(gems []OutdatedGem) []string {
	groupSet := make(map[string]bool)
	for _, gem := range gems {
		for _, group := range gem.Groups {
			groupSet[group] = true
		}
	}

	// Convert to sorted slice, with "default" first
	var groups []string
	if groupSet["default"] {
		groups = append(groups, "default")
		delete(groupSet, "default")
	}
	for group := range groupSet {
		groups = append(groups, group)
	}
	if len(groups) > 1 {
		sort.Strings(groups[1:]) // Sort all except "default" which is first
	}

	return groups
}

func initialOutdatedModel(gems []OutdatedGem) outdatedModel {
	// Build table rows
	rows, rowMapping := buildTableRows(gems)

	// Create table columns - will be adjusted on first WindowSizeMsg
	columns := []table.Column{
		{Title: "", Width: 4},            // Checkbox
		{Title: "Gem", Width: 25},        // Name
		{Title: "Version", Width: 22},    // Current → Latest
		{Title: "Type", Width: 8},        // MAJOR/MINOR/PATCH
		{Title: "Constraint", Width: 12}, // Version constraint
		{Title: "Groups", Width: 15},     // Gem groups
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(25), // Will be adjusted by WindowSizeMsg
	)

	// Custom table styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("86")).
		Background(lipgloss.Color("235")).
		Bold(false)
	t.SetStyles(s)

	return outdatedModel{
		table:           t,
		gems:            gems,
		rows:            rowMapping,
		availableGroups: collectAvailableGroups(gems),
	}
}

func (m outdatedModel) Init() tea.Cmd {
	return nil
}

func (m outdatedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Leave room for title, status bar, and padding
		tableHeight := msg.Height - 8
		if tableHeight < 5 {
			tableHeight = 5
		}
		m.table.SetHeight(tableHeight)
		m.table.SetWidth(msg.Width - 4)
		return m, nil

	case tea.KeyMsg:
		// Handle preview modal
		if m.showPreview {
			switch {
			case key.Matches(msg, outdatedKeys.Quit):
				m.showPreview = false
				return m, nil
			case key.Matches(msg, outdatedKeys.Update):
				// TODO: Actually perform update
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		// Normal mode key handling
		switch {
		case key.Matches(msg, outdatedKeys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, outdatedKeys.Toggle):
			// Toggle current selection
			cursor := m.table.Cursor()
			if cursor < len(m.rows) && !m.rows[cursor].isHeader {
				gemIdx := m.rows[cursor].gemIndex
				m.gems[gemIdx].Selected = !m.gems[gemIdx].Selected
				m.refreshTable()
			}
			return m, nil

		case key.Matches(msg, outdatedKeys.SelectPatch):
			m.selectByType(UpdatePatch)
			m.refreshTable()
			return m, nil

		case key.Matches(msg, outdatedKeys.SelectMinor):
			m.selectByType(UpdateMinor)
			m.refreshTable()
			return m, nil

		case key.Matches(msg, outdatedKeys.SelectMajor):
			m.selectByType(UpdateMajor)
			m.refreshTable()
			return m, nil

		case key.Matches(msg, outdatedKeys.SelectAll):
			for i := range m.gems {
				m.gems[i].Selected = true
			}
			m.refreshTable()
			return m, nil

		case key.Matches(msg, outdatedKeys.SelectNone):
			for i := range m.gems {
				m.gems[i].Selected = false
			}
			m.refreshTable()
			return m, nil

		case key.Matches(msg, outdatedKeys.CycleGroup):
			// Cycle through group filters: "" -> "default" -> "development" -> "test" -> ""
			m.cycleGroupFilter()
			m.refreshTable()
			return m, nil

		case key.Matches(msg, outdatedKeys.Update):
			// Show preview if any gems selected
			if m.countSelected() > 0 {
				m.showPreview = true
				return m, nil
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// selectByType selects all gems of a specific update type
func (m *outdatedModel) selectByType(updateType UpdateType) {
	for i := range m.gems {
		if m.gems[i].UpdateType == updateType {
			m.gems[i].Selected = true
		}
	}
}

// cycleGroupFilter cycles through available group filters
func (m *outdatedModel) cycleGroupFilter() {
	if len(m.availableGroups) == 0 {
		return
	}

	if m.filterGroup == "" {
		// Start with first available group
		m.filterGroup = m.availableGroups[0]
		return
	}

	// Find current position and move to next
	for i, group := range m.availableGroups {
		if group == m.filterGroup {
			if i+1 < len(m.availableGroups) {
				m.filterGroup = m.availableGroups[i+1]
			} else {
				// Wrap back to "all groups"
				m.filterGroup = ""
			}
			return
		}
	}

	// If current filter not found in available groups, reset
	m.filterGroup = ""
}

// refreshTable rebuilds the table with current selection state
func (m *outdatedModel) refreshTable() {
	// Filter gems by group if needed
	var visibleGems []OutdatedGem
	var gemIndexMap map[int]int // Map from visible index to original index

	if m.filterGroup == "" {
		visibleGems = m.gems
	} else {
		gemIndexMap = make(map[int]int)
		for i, gem := range m.gems {
			for _, group := range gem.Groups {
				if group == m.filterGroup {
					gemIndexMap[len(visibleGems)] = i
					visibleGems = append(visibleGems, gem)
					break
				}
			}
		}
	}

	// Rebuild rows
	rows, rowMapping := buildTableRows(visibleGems)

	// If we filtered, update row mapping to use original indices
	if gemIndexMap != nil {
		for i := range rowMapping {
			if !rowMapping[i].isHeader {
				rowMapping[i].gemIndex = gemIndexMap[rowMapping[i].gemIndex]
			}
		}
	}

	m.rows = rowMapping
	m.table.SetRows(rows)
}

// countSelected returns number of selected gems
func (m *outdatedModel) countSelected() int {
	count := 0
	for _, gem := range m.gems {
		if gem.Selected {
			count++
		}
	}
	return count
}

func (m outdatedModel) View() string {
	if m.quitting {
		return ""
	}

	var view strings.Builder

	// Title
	title := outdatedTitleStyle.Render("Outdated Gems")
	if m.filterGroup != "" {
		title = outdatedTitleStyle.Render(fmt.Sprintf("Outdated Gems (group: %s)", m.filterGroup))
	}
	view.WriteString(title)
	view.WriteString("\n\n")

	// Table
	view.WriteString(m.table.View())
	view.WriteString("\n\n")

	// Status bar
	statusBar := m.renderStatusBar()
	view.WriteString(statusBar)

	baseView := lipgloss.NewStyle().Padding(1, 2).Render(view.String())

	// Preview modal overlay
	if m.showPreview {
		overlay := m.renderPreviewModal()
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
	}

	return baseView
}

func (m outdatedModel) renderStatusBar() string {
	selectedCount := m.countSelected()

	var helpText string
	if m.showPreview {
		helpText = " U update • Esc cancel "
	} else {
		helpText = " Space toggle • p patch • m minor • M major • a all • n none • g group • U update • q quit "
	}

	width := m.width
	if width == 0 {
		width = 80
	}

	info := fmt.Sprintf(" %d outdated • %d selected ", len(m.gems), selectedCount)

	bar := outdatedStatusBar.
		Width(width).
		Render(info + strings.Repeat(" ", max(0, width-len(info)-len(helpText))) + helpText)

	return bar
}

func (m outdatedModel) renderPreviewModal() string {
	// Get selected gems
	var selected []OutdatedGem
	for _, gem := range m.gems {
		if gem.Selected {
			selected = append(selected, gem)
		}
	}

	// Sort by update type (major first, then minor, then patch, unknown last)
	updateTypeRank := map[UpdateType]int{
		UpdateMajor:   0,
		UpdateMinor:   1,
		UpdatePatch:   2,
		UpdateUnknown: 3,
	}
	sort.Slice(selected, func(i, j int) bool {
		rankI := updateTypeRank[selected[i].UpdateType]
		rankJ := updateTypeRank[selected[j].UpdateType]
		if rankI != rankJ {
			return rankI < rankJ
		}
		return selected[i].Name < selected[j].Name
	})

	// Styles
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("12")).
		Padding(1, 2).
		Width(min(80, m.width-4)).
		MaxWidth(80)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render(fmt.Sprintf("Update %d gems?", len(selected))))
	content.WriteString("\n\n")

	// List gems by type
	var majors, minors, patches []string
	for _, gem := range selected {
		line := fmt.Sprintf("  • %s: %s → %s", gem.Name, gem.CurrentVersion, gem.LatestVersion)
		switch gem.UpdateType {
		case UpdateMajor:
			majors = append(majors, majorUpdateStyle.Render(line))
		case UpdateMinor:
			minors = append(minors, minorUpdateStyle.Render(line))
		case UpdatePatch:
			patches = append(patches, patchUpdateStyle.Render(line))
		}
	}

	if len(majors) > 0 {
		content.WriteString(majorUpdateStyle.Render("MAJOR updates:"))
		content.WriteString("\n")
		content.WriteString(strings.Join(majors, "\n"))
		content.WriteString("\n\n")
	}

	if len(minors) > 0 {
		content.WriteString(minorUpdateStyle.Render("MINOR updates:"))
		content.WriteString("\n")
		content.WriteString(strings.Join(minors, "\n"))
		content.WriteString("\n\n")
	}

	if len(patches) > 0 {
		content.WriteString(patchUpdateStyle.Render("PATCH updates:"))
		content.WriteString("\n")
		content.WriteString(strings.Join(patches, "\n"))
		content.WriteString("\n\n")
	}

	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Press U to confirm, Esc to cancel"))

	return boxStyle.Render(content.String())
}

// RunOutdatedTUI starts the interactive TUI for viewing outdated gems
func RunOutdatedTUI(gemfilePath string) error {
	// Load outdated gems
	gems, err := LoadOutdatedGems(gemfilePath)
	if err != nil {
		return err
	}

	if len(gems) == 0 {
		fmt.Println("✨ All gems are up to date!")
		return nil
	}

	// Start TUI
	p := tea.NewProgram(initialOutdatedModel(gems), tea.WithAltScreen())
	_, err = p.Run()
	return err
}
