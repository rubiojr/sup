package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v3"
	"go.mau.fi/whatsmeow/types"

	"github.com/rubiojr/sup/internal/client"
	"github.com/rubiojr/sup/internal/config"
)

// Styles
var (
	alTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	alStatusBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	alSelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	alHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("24"))

	alNormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	alDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	alCheckStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	alTabActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57")).
				Padding(0, 2)

	alTabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Padding(0, 2)

	alBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	alDialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(1, 2).
			Align(lipgloss.Center)

	alDialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205"))

	alDialogButtonStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Margin(0, 1)

	alDialogButtonActiveStyle = lipgloss.NewStyle().
					Padding(0, 2).
					Margin(0, 1).
					Background(lipgloss.Color("57")).
					Foreground(lipgloss.Color("229"))
)

type alTab int

const (
	tabGroups alTab = iota
	tabUsers
)

// allowListItem represents a group or user for the TUI
type allowListItem struct {
	JID         string
	Name        string
	Phone       string // users only
	IsAllowed   bool
	MemberCount int // groups only
}

// allowListModel is the bubbletea model for the allow-list TUI
type allowListModel struct {
	configPath   string
	cfg          *config.Config
	groups       []allowListItem
	users        []allowListItem
	filtered     []allowListItem
	tab          alTab
	cursor       int
	highlighted  map[int]bool
	anchorIdx    int
	searchInput  textinput.Model
	searching    bool
	confirming   bool
	dialogChoice int // 0=Save, 1=Discard, 2=Cancel
	width        int
	height       int
	statusMsg    string
	dirty        bool
}

func botAllowListCommand(_ context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	c, err := client.GetClient()
	if err != nil {
		return err
	}
	defer c.Disconnect()

	groups, err := c.GetJoinedGroups()
	if err != nil {
		return fmt.Errorf("fetching groups: %w", err)
	}

	contacts, err := c.GetAllContacts()
	if err != nil {
		return fmt.Errorf("fetching contacts: %w", err)
	}

	m := newAllowListModel(configPath, cfg, groups, contacts)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

func newAllowListModel(configPath string, cfg *config.Config, groups []*types.GroupInfo, contacts map[types.JID]types.ContactInfo) *allowListModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 100
	ti.Width = 50

	allowedGroups := make(map[string]bool, len(cfg.Allow.Groups))
	for _, g := range cfg.Allow.Groups {
		allowedGroups[g.JID] = true
	}
	allowedUsers := make(map[string]bool, len(cfg.Allow.Users))
	for _, u := range cfg.Allow.Users {
		allowedUsers[u.JID] = true
	}

	// Build group items
	groupItems := make([]allowListItem, 0, len(groups))
	for _, g := range groups {
		groupItems = append(groupItems, allowListItem{
			JID:         g.JID.String(),
			Name:        g.Name,
			IsAllowed:   allowedGroups[g.JID.String()],
			MemberCount: len(g.Participants),
		})
	}
	sort.Slice(groupItems, func(i, j int) bool {
		if groupItems[i].IsAllowed != groupItems[j].IsAllowed {
			return groupItems[i].IsAllowed
		}
		return groupItems[i].Name < groupItems[j].Name
	})

	// Build user items
	userItems := make([]allowListItem, 0, len(contacts))
	for jid, contact := range contacts {
		if jid.Server != types.DefaultUserServer {
			continue
		}
		name := contact.FullName
		if name == "" {
			name = contact.BusinessName
		}
		if name == "" {
			name = jid.User
		}
		userItems = append(userItems, allowListItem{
			JID:       jid.String(),
			Name:      name,
			Phone:     jid.User,
			IsAllowed: allowedUsers[jid.String()],
		})
	}
	sort.Slice(userItems, func(i, j int) bool {
		if userItems[i].IsAllowed != userItems[j].IsAllowed {
			return userItems[i].IsAllowed
		}
		return userItems[i].Name < userItems[j].Name
	})

	m := &allowListModel{
		configPath:  configPath,
		cfg:         cfg,
		groups:      groupItems,
		users:       userItems,
		tab:         tabGroups,
		searchInput: ti,
		highlighted: make(map[int]bool),
		anchorIdx:   -1,
	}
	m.rebuildFiltered()

	return m
}

func (m *allowListModel) Init() tea.Cmd {
	return nil
}

func (m *allowListModel) currentItems() *[]allowListItem {
	if m.tab == tabGroups {
		return &m.groups
	}
	return &m.users
}

func (m *allowListModel) rebuildFiltered() {
	items := *m.currentItems()
	query := strings.ToLower(m.searchInput.Value())

	if query == "" {
		m.filtered = make([]allowListItem, len(items))
		copy(m.filtered, items)
	} else {
		m.filtered = make([]allowListItem, 0)
		for _, item := range items {
			target := strings.ToLower(item.Name + " " + item.JID + " " + item.Phone)
			if strings.Contains(target, query) {
				m.filtered = append(m.filtered, item)
			}
		}
	}

	if m.cursor >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		} else {
			m.cursor = 0
		}
	}
}

func (m *allowListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	if m.searching {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.rebuildFiltered()
		return m, cmd
	}

	return m, nil
}

func (m *allowListModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Dialog keys
	if m.confirming {
		switch msg.String() {
		case "left", "h":
			if m.dialogChoice > 0 {
				m.dialogChoice--
			}
		case "right", "l":
			if m.dialogChoice < 2 {
				m.dialogChoice++
			}
		case "tab":
			m.dialogChoice = (m.dialogChoice + 1) % 3
		case "shift+tab":
			m.dialogChoice = (m.dialogChoice + 2) % 3
		case "enter", " ":
			switch m.dialogChoice {
			case 0: // Save
				m.saveConfig()
				return m, tea.Quit
			case 1: // Discard
				return m, tea.Quit
			case 2: // Cancel
				m.confirming = false
				m.dialogChoice = 0
			}
		case "esc", "q":
			m.confirming = false
			m.dialogChoice = 0
		case "s":
			m.saveConfig()
			return m, tea.Quit
		case "d":
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "q":
		if m.searching {
			break
		}
		if m.dirty {
			m.confirming = true
			m.dialogChoice = 0
			return m, nil
		}
		return m, tea.Quit

	case "esc":
		if m.searching {
			m.searching = false
			m.searchInput.Blur()
			m.searchInput.SetValue("")
			m.rebuildFiltered()
			m.clearHighlight()
			return m, nil
		}
		if len(m.highlighted) > 0 {
			m.clearHighlight()
			return m, nil
		}
		if m.dirty {
			m.confirming = true
			m.dialogChoice = 0
			return m, nil
		}
		return m, tea.Quit

	case "tab":
		if !m.searching && !m.confirming {
			if m.tab == tabGroups {
				m.tab = tabUsers
			} else {
				m.tab = tabGroups
			}
			m.cursor = 0
			m.searchInput.SetValue("")
			m.rebuildFiltered()
			m.clearHighlight()
			return m, nil
		}

	case "/":
		if !m.searching && !m.confirming {
			m.searching = true
			m.searchInput.Focus()
			m.clearHighlight()
			return m, textinput.Blink
		}

	case "enter":
		if m.searching {
			m.searching = false
			m.searchInput.Blur()
			return m, nil
		}
		m.saveConfig()
		return m, tea.Quit

	case " ":
		if !m.searching && !m.confirming && len(m.filtered) > 0 {
			m.toggleSelection()
			return m, nil
		}

	case "shift+up", "K":
		if !m.searching && !m.confirming && len(m.filtered) > 0 {
			m.extendHighlight(-1)
		}

	case "shift+down", "J":
		if !m.searching && !m.confirming && len(m.filtered) > 0 {
			m.extendHighlight(1)
		}

	case "up", "k":
		if !m.searching && !m.confirming && m.cursor > 0 {
			m.cursor--
			m.clearHighlight()
		}

	case "down", "j":
		if !m.searching && !m.confirming && m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.clearHighlight()
		}

	case "g":
		if !m.searching && !m.confirming {
			m.cursor = 0
			m.clearHighlight()
		}

	case "G":
		if !m.searching && !m.confirming && len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
			m.clearHighlight()
		}

	case "a":
		if !m.searching && !m.confirming {
			m.selectAll()
			return m, nil
		}

	case "n":
		if !m.searching && !m.confirming {
			m.selectNone()
			return m, nil
		}
	}

	if m.searching {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.rebuildFiltered()
		return m, cmd
	}

	return m, nil
}

func (m *allowListModel) clearHighlight() {
	m.highlighted = make(map[int]bool)
	m.anchorIdx = -1
}

func (m *allowListModel) extendHighlight(direction int) {
	if len(m.filtered) == 0 {
		return
	}
	if m.anchorIdx == -1 {
		m.anchorIdx = m.cursor
		m.highlighted[m.cursor] = true
	}

	newCursor := m.cursor + direction
	if newCursor < 0 {
		newCursor = 0
	}
	if newCursor >= len(m.filtered) {
		newCursor = len(m.filtered) - 1
	}
	m.cursor = newCursor

	m.highlighted = make(map[int]bool)
	start, end := m.anchorIdx, m.cursor
	if start > end {
		start, end = end, start
	}
	for i := start; i <= end; i++ {
		m.highlighted[i] = true
	}
}

func (m *allowListModel) toggleSelection() {
	if m.cursor >= len(m.filtered) {
		return
	}

	if len(m.highlighted) > 0 {
		m.toggleHighlighted()
		return
	}

	item := &m.filtered[m.cursor]
	item.IsAllowed = !item.IsAllowed
	m.dirty = true

	// Sync back to source list
	items := m.currentItems()
	for i := range *items {
		if (*items)[i].JID == item.JID {
			(*items)[i].IsAllowed = item.IsAllowed
			break
		}
	}

	if item.IsAllowed {
		m.statusMsg = fmt.Sprintf("✓ Allowed: %s", alTruncate(item.Name, 40))
	} else {
		m.statusMsg = fmt.Sprintf("✗ Removed: %s", alTruncate(item.Name, 40))
	}
}

func (m *allowListModel) toggleHighlighted() {
	if len(m.highlighted) == 0 {
		return
	}

	selectedCount := 0
	for idx := range m.highlighted {
		if m.filtered[idx].IsAllowed {
			selectedCount++
		}
	}
	allowThem := selectedCount < len(m.highlighted)/2+1

	items := m.currentItems()
	for idx := range m.highlighted {
		item := &m.filtered[idx]
		item.IsAllowed = allowThem
		for j := range *items {
			if (*items)[j].JID == item.JID {
				(*items)[j].IsAllowed = allowThem
				break
			}
		}
	}

	m.dirty = true
	if allowThem {
		m.statusMsg = fmt.Sprintf("✓ Allowed %d items", len(m.highlighted))
	} else {
		m.statusMsg = fmt.Sprintf("✗ Removed %d items", len(m.highlighted))
	}
	m.clearHighlight()
}

func (m *allowListModel) selectAll() {
	items := m.currentItems()
	for i := range m.filtered {
		if !m.filtered[i].IsAllowed {
			m.filtered[i].IsAllowed = true
			for j := range *items {
				if (*items)[j].JID == m.filtered[i].JID {
					(*items)[j].IsAllowed = true
					break
				}
			}
		}
	}
	m.dirty = true
	m.statusMsg = fmt.Sprintf("Allowed all %d visible items", len(m.filtered))
}

func (m *allowListModel) selectNone() {
	items := m.currentItems()
	for i := range m.filtered {
		if m.filtered[i].IsAllowed {
			m.filtered[i].IsAllowed = false
			for j := range *items {
				if (*items)[j].JID == m.filtered[i].JID {
					(*items)[j].IsAllowed = false
					break
				}
			}
		}
	}
	m.dirty = true
	m.statusMsg = fmt.Sprintf("Removed all %d visible items", len(m.filtered))
}

func (m *allowListModel) saveConfig() {
	var groups, users []config.AllowEntry
	for _, g := range m.groups {
		if g.IsAllowed {
			groups = append(groups, config.AllowEntry{JID: g.JID, Name: g.Name})
		}
	}
	for _, u := range m.users {
		if u.IsAllowed {
			users = append(users, config.AllowEntry{JID: u.JID, Name: u.Name})
		}
	}

	m.cfg.Allow.Groups = groups
	m.cfg.Allow.Users = users

	if err := config.Save(m.configPath, m.cfg); err != nil {
		m.statusMsg = fmt.Sprintf("Error saving: %v", err)
		return
	}

	m.dirty = false
	m.statusMsg = "Config saved"
}

func (m *allowListModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Title
	title := alTitleStyle.Render("🔒 Bot Allow List")

	// Tabs
	groupsTab := alTabInactiveStyle.Render("Groups")
	usersTab := alTabInactiveStyle.Render("Users")
	if m.tab == tabGroups {
		groupsTab = alTabActiveStyle.Render("Groups")
	} else {
		usersTab = alTabActiveStyle.Render("Users")
	}
	tabs := "  " + groupsTab + " " + usersTab + "\n"

	// Search bar
	searchBar := ""
	if m.searching {
		searchBar = "  🔍 " + m.searchInput.View() + "\n"
	} else {
		searchBar = alDimStyle.Render("  Press / to search, tab to switch") + "\n"
	}

	// List height: total - title(1) - tabs(1) - search(1) - status(1) - borders(2) - padding(1)
	listHeight := m.height - 7
	if listHeight < 5 {
		listHeight = 5
	}

	var listBuilder strings.Builder

	if len(m.filtered) == 0 {
		items := m.currentItems()
		if len(*items) == 0 {
			listBuilder.WriteString("\n  Loading...\n")
		} else {
			listBuilder.WriteString("\n  No matching items.\n")
		}
	} else {
		// Viewport
		start := 0
		if m.cursor >= listHeight {
			start = m.cursor - listHeight + 1
		}
		end := start + listHeight
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		maxNameWidth := m.width - 40
		if maxNameWidth < 20 {
			maxNameWidth = 20
		}

		for i := start; i < end; i++ {
			item := m.filtered[i]

			checkbox := "[ ]"
			if item.IsAllowed {
				checkbox = alCheckStyle.Render("[✓]")
			}

			name := alTruncate(item.Name, maxNameWidth)

			var detail string
			if m.tab == tabGroups {
				detail = fmt.Sprintf("%d members", item.MemberCount)
			} else {
				detail = item.Phone
			}

			line := fmt.Sprintf(" %s %-*s  %-15s  %s",
				checkbox, maxNameWidth, name, detail, alDimStyle.Render(item.JID))

			// Truncate full line to prevent wrapping
			maxLineWidth := m.width - 4
			if len(line) > maxLineWidth {
				line = line[:maxLineWidth]
			}

			if i == m.cursor {
				line = alSelectedStyle.Render(line)
			} else if m.highlighted[i] {
				line = alHighlightStyle.Render(line)
			} else {
				line = alNormalStyle.Render(line)
			}

			listBuilder.WriteString(line + "\n")
		}
	}

	// Content with border
	contentHeight := m.height - 4
	if contentHeight < 5 {
		contentHeight = 5
	}
	contentStyle := alBorderStyle.Width(m.width - 2).Height(contentHeight - 2)
	content := searchBar + listBuilder.String()
	contentBox := contentStyle.Render(content)

	// Status bar
	allowedCount := 0
	for _, item := range *m.currentItems() {
		if item.IsAllowed {
			allowedCount++
		}
	}

	leftStatus := m.statusMsg
	if leftStatus == "" {
		if len(m.highlighted) > 0 {
			leftStatus = fmt.Sprintf("%d highlighted | %d allowed", len(m.highlighted), allowedCount)
		} else if len(m.filtered) > 0 {
			leftStatus = fmt.Sprintf("%d/%d items | %d allowed", m.cursor+1, len(m.filtered), allowedCount)
		} else {
			leftStatus = fmt.Sprintf("0 items | %d allowed", allowedCount)
		}
		if m.dirty {
			leftStatus += " *"
		}
	}
	rightStatus := "space:toggle  a:all  n:none  ↵:save  q:quit"

	gap := m.width - len(leftStatus) - len(rightStatus) - 4
	if gap < 0 {
		gap = 0
	}

	statusBar := " " + alStatusBarStyle.Render(leftStatus) +
		strings.Repeat(" ", gap) +
		alDimStyle.Render(rightStatus) + " "

	mainView := lipgloss.JoinVertical(lipgloss.Left,
		title,
		tabs,
		contentBox,
		statusBar,
	)

	if m.confirming {
		return m.renderWithDialog(mainView)
	}

	return mainView
}

func (m *allowListModel) renderWithDialog(mainView string) string {
	buttons := []string{"[S]ave", "[D]iscard", "[C]ancel"}
	var renderedButtons []string
	for i, btn := range buttons {
		if i == m.dialogChoice {
			renderedButtons = append(renderedButtons, alDialogButtonActiveStyle.Render(btn))
		} else {
			renderedButtons = append(renderedButtons, alDialogButtonStyle.Render(btn))
		}
	}

	buttonRow := lipgloss.JoinHorizontal(lipgloss.Center, renderedButtons...)

	dialogContent := lipgloss.JoinVertical(lipgloss.Center,
		alDialogTitleStyle.Render("⚠ Unsaved Changes"),
		"",
		"You have unsaved allow list changes.",
		"What would you like to do?",
		"",
		buttonRow,
		"",
		alDimStyle.Render("←/→ to navigate, Enter to confirm"),
	)

	dialog := alDialogStyle.Render(dialogContent)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
	)
}

func alTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
