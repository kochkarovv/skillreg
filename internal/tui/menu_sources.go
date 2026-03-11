package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/git"
	"github.com/vladyslav/skillreg/internal/models"
	"github.com/vladyslav/skillreg/internal/scanner"
	"github.com/vladyslav/skillreg/internal/tui/components"
)

// sourcesView enumerates the sub-views within the sources menu.
type sourcesView int

const (
	sourcesViewList       sourcesView = iota
	sourcesViewDetail                         // selected source: actions
	sourcesViewAddPath                        // text input for repo path
	sourcesViewAddConfirm                     // show discovered skills, confirm
	sourcesViewPullOptions                    // options when pull fails
	sourcesViewConfirmRemove                  // confirm removal
)

// Detail actions for a selected source.
const (
	sourceActionPull       = 0
	sourceActionRescan     = 1
	sourceActionAutoUpdate = 2
	sourceActionRemove     = 3
)

// Pull failure options.
const (
	pullOptionStashAndPull = 0
	pullOptionForceReset   = 1
	pullOptionOpenTerminal = 2
	pullOptionSkip         = 3
)

// shellDoneMsg is returned when the spawned shell exits.
type shellDoneMsg struct{ err error }

// sourceScanResult carries async scan results.
type sourceScanResult struct {
	skills []scanner.DiscoveredSkill
	err    error
}

// sourcePullResult carries async pull results.
type sourcePullResult struct {
	err   error
	dirty bool // true if failed because working tree is dirty
}

// sourcesMenuModel is the BubbleTea model for the sources screen.
type sourcesMenuModel struct {
	db      *db.Database
	sources []*models.Source
	cursor  int
	err     error
	status  string // status message shown at bottom

	currentView sourcesView

	// Detail view
	selectedSource *models.Source
	detailCursor   int
	detailActions  []string

	// Add path view
	pathInput       textinput.Model
	discoveredSkills []scanner.DiscoveredSkill
	addRepoPath     string // validated absolute path
	addRepoRemote   string

	// Pull options view
	pullCursor  int
	pullOptions []string
	pullDirty   bool // whether failure was due to dirty tree

	// Confirm remove
	confirm components.ConfirmModel
}

// newSourcesMenu constructs a sourcesMenuModel and loads sources from the DB.
func newSourcesMenu(d *db.Database) sourcesMenuModel {
	ti := textinput.New()
	ti.Placeholder = "Enter path to git repository..."
	ti.CharLimit = 512

	m := sourcesMenuModel{
		db:        d,
		pathInput: ti,
	}
	m.loadSources()
	return m
}

func (m *sourcesMenuModel) loadSources() {
	sources, err := models.ListSources(m.db)
	if err != nil {
		m.err = err
	} else {
		m.sources = sources
		m.err = nil
	}
}

// totalRows returns len(sources) + 1 for the [Add source] row.
func (m sourcesMenuModel) totalRows() int {
	return len(m.sources) + 1
}

func (m sourcesMenuModel) buildDetailActions(src *models.Source) []string {
	autoLabel := "Enable auto-update"
	if src.AutoUpdate {
		autoLabel = "Disable auto-update"
	}
	return []string{
		"Pull latest changes",
		"Rescan for skills",
		autoLabel,
		"Remove source",
	}
}

// update handles keyboard navigation and state transitions.
func (m sourcesMenuModel) update(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	switch m.currentView {
	case sourcesViewList:
		return m.updateList(msg)
	case sourcesViewDetail:
		return m.updateDetail(msg)
	case sourcesViewAddPath:
		return m.updateAddPath(msg)
	case sourcesViewAddConfirm:
		return m.updateAddConfirm(msg)
	case sourcesViewPullOptions:
		return m.updatePullOptions(msg)
	case sourcesViewConfirmRemove:
		return m.updateConfirmRemove(msg)
	}
	return m, nil
}

// --- List view ---

func (m sourcesMenuModel) updateList(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < m.totalRows()-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor < len(m.sources) {
				// Select a source → detail view
				src := m.sources[m.cursor]
				m.selectedSource = src
				m.detailActions = m.buildDetailActions(src)
				m.detailCursor = 0
				m.currentView = sourcesViewDetail
			} else {
				// [Add source]
				m.pathInput.SetValue("")
				m.pathInput.Focus()
				m.status = ""
				m.currentView = sourcesViewAddPath
				return m, m.pathInput.Cursor.BlinkCmd()
			}
		case "esc", "q":
			return m, navigate(viewMain)
		}
	}
	return m, nil
}

// --- Detail view ---

func (m sourcesMenuModel) updateDetail(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case sourcePullResult:
		if msg.err == nil {
			m.status = "Pull successful"
			_ = models.UpdateSourceLastChecked(m.db, m.selectedSource.ID)
			m.currentView = sourcesViewDetail
		} else {
			// Pull failed — show options
			m.pullDirty = msg.dirty
			if msg.dirty {
				m.pullOptions = []string{"Stash & pull", "Open terminal", "Skip"}
			} else {
				m.pullOptions = []string{"Force reset to origin", "Open terminal", "Skip"}
			}
			m.pullCursor = 0
			m.currentView = sourcesViewPullOptions
			m.status = fmt.Sprintf("Pull failed: %v", msg.err)
		}
		return m, nil

	case sourceScanResult:
		if msg.err != nil {
			m.status = fmt.Sprintf("Scan error: %v", msg.err)
		} else {
			// Delete old skills, create new ones
			_ = models.DeleteSkillsBySource(m.db, m.selectedSource.ID)
			for _, sk := range msg.skills {
				_, _ = models.CreateSkill(m.db, m.selectedSource.ID, sk.Name, sk.Path, sk.Description)
			}
			m.status = fmt.Sprintf("Found %d skill(s)", len(msg.skills))
		}
		return m, nil

	case shellDoneMsg:
		m.status = "Returned from terminal"
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.detailCursor > 0 {
				m.detailCursor--
			}
		case "down", "j":
			if m.detailCursor < len(m.detailActions)-1 {
				m.detailCursor++
			}
		case "enter":
			return m.executeDetailAction()
		case "esc", "q":
			m.currentView = sourcesViewList
			m.status = ""
		}
	}
	return m, nil
}

func (m sourcesMenuModel) executeDetailAction() (sourcesMenuModel, tea.Cmd) {
	src := m.selectedSource
	switch m.detailCursor {
	case sourceActionPull:
		m.status = "Pulling..."
		return m, m.pullSource(src.Path)

	case sourceActionRescan:
		m.status = "Scanning..."
		return m, m.scanSource(src.Path)

	case sourceActionAutoUpdate:
		newVal := !src.AutoUpdate
		if err := models.SetSourceAutoUpdate(m.db, src.ID, newVal); err != nil {
			m.status = fmt.Sprintf("Error: %v", err)
		} else {
			src.AutoUpdate = newVal
			m.detailActions = m.buildDetailActions(src)
			if newVal {
				m.status = "Auto-update enabled"
			} else {
				m.status = "Auto-update disabled"
			}
		}

	case sourceActionRemove:
		m.confirm = components.NewConfirm(
			fmt.Sprintf("Remove source %q and all its skills?", src.Name),
		)
		m.currentView = sourcesViewConfirmRemove
	}
	return m, nil
}

func (m sourcesMenuModel) pullSource(path string) tea.Cmd {
	return func() tea.Msg {
		// Try fast-forward pull first
		err := git.PullFF(path)
		if err == nil {
			return sourcePullResult{}
		}
		// Check if dirty
		dirty, _ := git.IsDirty(path)
		return sourcePullResult{err: err, dirty: dirty}
	}
}

func (m sourcesMenuModel) scanSource(path string) tea.Cmd {
	return func() tea.Msg {
		skills, err := scanner.ScanRepo(path)
		return sourceScanResult{skills: skills, err: err}
	}
}

// --- Pull options view ---

func (m sourcesMenuModel) updatePullOptions(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case shellDoneMsg:
		m.status = "Returned from terminal"
		m.currentView = sourcesViewDetail
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.pullCursor > 0 {
				m.pullCursor--
			}
		case "down", "j":
			if m.pullCursor < len(m.pullOptions)-1 {
				m.pullCursor++
			}
		case "enter":
			return m.executePullOption()
		case "esc", "q":
			m.currentView = sourcesViewDetail
			m.status = ""
		}
	}
	return m, nil
}

func (m sourcesMenuModel) executePullOption() (sourcesMenuModel, tea.Cmd) {
	src := m.selectedSource
	selected := m.pullOptions[m.pullCursor]

	switch selected {
	case "Stash & pull":
		if err := git.StashAndPull(src.Path); err != nil {
			m.status = fmt.Sprintf("Stash & pull failed: %v", err)
		} else {
			m.status = "Stash & pull successful"
			_ = models.UpdateSourceLastChecked(m.db, src.ID)
		}
		m.currentView = sourcesViewDetail

	case "Force reset to origin":
		if err := git.ForceReset(src.Path); err != nil {
			m.status = fmt.Sprintf("Force reset failed: %v", err)
		} else {
			m.status = "Force reset successful"
			_ = models.UpdateSourceLastChecked(m.db, src.ID)
		}
		m.currentView = sourcesViewDetail

	case "Open terminal":
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		c := exec.Command(shell, "-l")
		c.Dir = src.Path
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return shellDoneMsg{err: err}
		})

	case "Skip":
		m.currentView = sourcesViewDetail
		m.status = ""
	}
	return m, nil
}

// --- Add path view ---

func (m sourcesMenuModel) updateAddPath(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			raw := strings.TrimSpace(m.pathInput.Value())
			if raw == "" {
				m.status = "Path cannot be empty"
				return m, nil
			}
			absPath := expandPath(raw)
			if !git.IsGitRepo(absPath) {
				m.status = "Not a valid git repository"
				return m, nil
			}
			// Scan for skills
			skills, err := scanner.ScanRepo(absPath)
			if err != nil {
				m.status = fmt.Sprintf("Scan error: %v", err)
				return m, nil
			}
			m.addRepoPath = absPath
			m.addRepoRemote = git.GetRemoteURL(absPath)
			m.discoveredSkills = skills
			m.currentView = sourcesViewAddConfirm
			m.status = ""
			return m, nil

		case "esc":
			m.currentView = sourcesViewList
			m.status = ""
			return m, nil
		}
	}

	// Delegate to textinput
	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

// --- Add confirm view ---

func (m sourcesMenuModel) updateAddConfirm(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			// Create source and skills
			name := filepath.Base(m.addRepoPath)
			src, err := models.CreateSource(m.db, name, m.addRepoPath, m.addRepoRemote)
			if err != nil {
				m.status = fmt.Sprintf("Error creating source: %v", err)
				m.currentView = sourcesViewList
				return m, nil
			}
			for _, sk := range m.discoveredSkills {
				_, _ = models.CreateSkill(m.db, src.ID, sk.Name, sk.Path, sk.Description)
			}
			m.status = fmt.Sprintf("Added source %q with %d skill(s)", name, len(m.discoveredSkills))
			m.loadSources()
			m.currentView = sourcesViewList
			return m, nil

		case "n", "esc":
			m.currentView = sourcesViewAddPath
			m.status = ""
		}
	}
	return m, nil
}

// --- Confirm remove view ---

func (m sourcesMenuModel) updateConfirmRemove(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case components.ConfirmResultMsg:
		if msg.Confirmed {
			src := m.selectedSource
			_ = models.DeleteSkillsBySource(m.db, src.ID)
			if err := models.DeleteSource(m.db, src.ID); err != nil {
				m.status = fmt.Sprintf("Error removing source: %v", err)
			} else {
				m.status = fmt.Sprintf("Removed source %q", src.Name)
			}
			m.loadSources()
			m.cursor = 0
			m.currentView = sourcesViewList
		} else {
			m.currentView = sourcesViewDetail
			m.status = ""
		}
		return m, nil
	}

	// Delegate to confirm component
	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)
	return m, cmd
}

// --- View ---

func (m sourcesMenuModel) view() string {
	switch m.currentView {
	case sourcesViewDetail:
		return m.viewDetail()
	case sourcesViewAddPath:
		return m.viewAddPath()
	case sourcesViewAddConfirm:
		return m.viewAddConfirm()
	case sourcesViewPullOptions:
		return m.viewPullOptions()
	case sourcesViewConfirmRemove:
		return m.viewConfirmRemove()
	default:
		return m.viewList()
	}
}

func (m sourcesMenuModel) viewList() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Sources"))
	sb.WriteString("\n\n")

	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error loading sources: %v", m.err)))
		sb.WriteString("\n")
	} else {
		for i, src := range m.sources {
			name := src.Name
			path := subtleStyle.Render(src.Path)

			var extra string
			if src.AutoUpdate {
				extra = successStyle.Render(" [auto]")
			}

			if i == m.cursor {
				sb.WriteString(selectedStyle.Render("> "+name) + " " + path + extra)
			} else {
				sb.WriteString(normalStyle.Render("  "+name) + " " + path + extra)
			}
			sb.WriteString("\n")
		}

		// [Add source] virtual row
		addLabel := "[Add source]"
		addIdx := len(m.sources)
		if m.cursor == addIdx {
			sb.WriteString(selectedStyle.Render("> " + addLabel))
		} else {
			sb.WriteString(subtleStyle.Render("  " + addLabel))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	if m.status != "" {
		sb.WriteString(components.StatusBar(m.status, 60))
		sb.WriteString("\n")
	}
	sb.WriteString(subtleStyle.Render("↑/↓ navigate • enter select • esc back"))

	return sb.String()
}

func (m sourcesMenuModel) viewDetail() string {
	var sb strings.Builder
	src := m.selectedSource

	sb.WriteString(titleStyle.Render("Source: " + src.Name))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  Path:        %s\n", src.Path))
	if src.RemoteURL != "" {
		sb.WriteString(fmt.Sprintf("  Remote:      %s\n", src.RemoteURL))
	}
	sb.WriteString(fmt.Sprintf("  Auto-update: %v\n", src.AutoUpdate))

	// Show branch info
	if branch, err := git.GetCurrentBranch(src.Path); err == nil {
		sb.WriteString(fmt.Sprintf("  Branch:      %s\n", branch))
	}
	if behind, err := git.CommitsBehind(src.Path); err == nil && behind > 0 {
		sb.WriteString(warningStyle.Render(fmt.Sprintf("  %d commit(s) behind origin", behind)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	for i, action := range m.detailActions {
		if i == m.detailCursor {
			sb.WriteString(selectedStyle.Render("  > " + action))
		} else {
			sb.WriteString(normalStyle.Render("    " + action))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	if m.status != "" {
		sb.WriteString(components.StatusBar(m.status, 60))
		sb.WriteString("\n")
	}
	sb.WriteString(subtleStyle.Render("↑/↓ navigate • enter select • esc back"))

	return sb.String()
}

func (m sourcesMenuModel) viewAddPath() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Add Source"))
	sb.WriteString("\n\n")
	sb.WriteString("  Enter the path to a git repository:\n\n")
	sb.WriteString("  " + m.pathInput.View())
	sb.WriteString("\n\n")

	if m.status != "" {
		sb.WriteString(errorStyle.Render("  " + m.status))
		sb.WriteString("\n\n")
	}
	sb.WriteString(subtleStyle.Render("  enter confirm • esc cancel"))

	return sb.String()
}

func (m sourcesMenuModel) viewAddConfirm() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Confirm Add Source"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  Repository: %s\n", m.addRepoPath))
	if m.addRepoRemote != "" {
		sb.WriteString(fmt.Sprintf("  Remote:     %s\n", m.addRepoRemote))
	}
	sb.WriteString(fmt.Sprintf("\n  Discovered %d skill(s):\n", len(m.discoveredSkills)))
	for _, sk := range m.discoveredSkills {
		desc := ""
		if sk.Description != "" {
			desc = " — " + sk.Description
		}
		sb.WriteString(fmt.Sprintf("    • %s%s\n", sk.Name, desc))
	}

	sb.WriteString("\n  Add this source? (y/n)\n")

	return sb.String()
}

func (m sourcesMenuModel) viewPullOptions() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Pull Failed"))
	sb.WriteString("\n\n")

	if m.status != "" {
		sb.WriteString(warningStyle.Render("  " + m.status))
		sb.WriteString("\n\n")
	}

	sb.WriteString("  Choose an option:\n\n")
	for i, opt := range m.pullOptions {
		if i == m.pullCursor {
			sb.WriteString(selectedStyle.Render("  > " + opt))
		} else {
			sb.WriteString(normalStyle.Render("    " + opt))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  ↑/↓ navigate • enter select • esc back"))

	return sb.String()
}

func (m sourcesMenuModel) viewConfirmRemove() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Remove Source"))
	sb.WriteString("\n")
	sb.WriteString(m.confirm.View())
	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  ←/→ switch • y/n shortcut • enter confirm"))

	return sb.String()
}

// expandPath expands ~ to the user's home directory and resolves the absolute path.
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
