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
	"github.com/vladyslav/skillreg/internal/linker"
	"github.com/vladyslav/skillreg/internal/models"
	"github.com/vladyslav/skillreg/internal/scanner"
	"github.com/vladyslav/skillreg/internal/tui/components"
)

// sourcesView enumerates the sub-views within the sources menu.
type sourcesView int

const (
	sourcesViewList            sourcesView = iota
	sourcesViewDetail                              // selected source: actions
	sourcesViewAddPath                             // text input for repo path
	sourcesViewAddConfirm                          // show discovered skills, confirm
	sourcesViewInstallPick                         // pick skills from new source to install
	sourcesViewInstallTargets                      // pick target instances
	sourcesViewPullOptions                         // options when pull fails
	sourcesViewConfirmRemove                       // confirm removal
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

	// Tab completion
	completions    []string // current set of matching paths
	completionIdx  int      // index into completions for cycling
	completionBase string   // the text that was completed from

	// Post-add install flow
	newSourceSkills []*models.Skill    // skills from newly added source
	skillSel        map[int]bool       // selected skill indices
	skillCursor     int
	installInstances []*models.Instance // all instances for target selection
	installSel       map[int]bool      // selected instance indices
	installCursor    int

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
	case sourcesViewInstallPick:
		return m.updateInstallPick(msg)
	case sourcesViewInstallTargets:
		return m.updateInstallTargets(msg)
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
				// [Add source] — prefill with home directory
				home, _ := os.UserHomeDir()
				if home == "" {
					home = "/"
				}
				m.pathInput.SetValue(home + "/")
				m.pathInput.CursorEnd()
				m.pathInput.Focus()
				m.status = ""
				m.completions = nil
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

		case "tab":
			m.handleTabCompletion()
			return m, nil

		case "esc":
			m.currentView = sourcesViewList
			m.status = ""
			return m, nil
		}
	}

	// Any non-tab key resets completion state
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() != "tab" {
		m.completions = nil
	}

	// Delegate to textinput
	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

// handleTabCompletion implements filesystem path tab-completion.
func (m *sourcesMenuModel) handleTabCompletion() {
	current := m.pathInput.Value()

	// If we already have completions and the base hasn't changed, cycle through them
	if len(m.completions) > 0 && m.completionBase == current {
		m.completionIdx = (m.completionIdx + 1) % len(m.completions)
		m.pathInput.SetValue(m.completions[m.completionIdx])
		m.pathInput.CursorEnd()
		m.completionBase = m.pathInput.Value()
		return
	}

	// Build fresh completions
	expanded := expandPath(current)

	var dir, prefix string
	if strings.HasSuffix(current, "/") {
		// User typed a trailing slash — list that directory
		dir = expanded
		prefix = ""
	} else {
		dir = filepath.Dir(expanded)
		prefix = filepath.Base(expanded)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		m.status = "Cannot read directory"
		return
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}
		fullPath := filepath.Join(dir, name) + "/"
		matches = append(matches, fullPath)
	}

	if len(matches) == 0 {
		m.status = "No matches"
		m.completions = nil
		return
	}

	m.completions = matches
	m.completionIdx = 0
	m.pathInput.SetValue(matches[0])
	m.pathInput.CursorEnd()
	m.completionBase = m.pathInput.Value()
	m.status = fmt.Sprintf("%d match(es) — tab to cycle", len(matches))
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
			var createdSkills []*models.Skill
			for _, sk := range m.discoveredSkills {
				created, err := models.CreateSkill(m.db, src.ID, sk.Name, sk.Path, sk.Description)
				if err == nil {
					createdSkills = append(createdSkills, created)
				}
			}
			m.loadSources()

			// If there are skills and instances, offer to install them
			instances, _ := models.ListAllInstances(m.db)
			if len(createdSkills) > 0 && len(instances) > 0 {
				m.newSourceSkills = createdSkills
				m.skillSel = make(map[int]bool)
				for i := range createdSkills {
					m.skillSel[i] = true // pre-select all
				}
				m.skillCursor = 0
				m.status = fmt.Sprintf("Added source %q — select skills to install:", name)
				m.currentView = sourcesViewInstallPick
				return m, nil
			}

			m.status = fmt.Sprintf("Added source %q with %d skill(s)", name, len(createdSkills))
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
	case sourcesViewInstallPick:
		return m.viewInstallPick()
	case sourcesViewInstallTargets:
		return m.viewInstallTargets()
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

	sb.WriteString(titleStyle.Render("Skill Sources"))
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
	sb.WriteString(subtleStyle.Render("  tab autocomplete • enter confirm • esc cancel"))

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

// --- Post-add install: pick skills ---

func (m sourcesMenuModel) updateInstallPick(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	totalRows := len(m.newSourceSkills) + 1 // +1 for "Select all" at top

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.skillCursor > 0 {
				m.skillCursor--
			}
		case "down", "j":
			if m.skillCursor < totalRows-1 {
				m.skillCursor++
			}
		case " ":
			if m.skillCursor == 0 {
				// Toggle all
				allSelected := true
				for i := range m.newSourceSkills {
					if !m.skillSel[i] {
						allSelected = false
						break
					}
				}
				for i := range m.newSourceSkills {
					m.skillSel[i] = !allSelected
				}
			} else {
				idx := m.skillCursor - 1
				m.skillSel[idx] = !m.skillSel[idx]
			}
		case "enter":
			var selected []*models.Skill
			for i, sk := range m.newSourceSkills {
				if m.skillSel[i] {
					selected = append(selected, sk)
				}
			}
			if len(selected) == 0 {
				m.status = "Source added. No skills selected for install."
				m.currentView = sourcesViewList
				return m, nil
			}
			instances, _ := models.ListAllInstances(m.db)
			m.installInstances = instances
			m.installSel = make(map[int]bool)
			for i := range instances {
				m.installSel[i] = true
			}
			m.installCursor = 0
			m.status = ""
			m.currentView = sourcesViewInstallTargets
		case "esc":
			m.status = "Source added. Skipped install."
			m.currentView = sourcesViewList
		}
	}
	return m, nil
}

// --- Post-add install: pick instances ---

func (m sourcesMenuModel) updateInstallTargets(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	totalRows := len(m.installInstances) + 1 // +1 for "Select all" at top

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.installCursor > 0 {
				m.installCursor--
			}
		case "down", "j":
			if m.installCursor < totalRows-1 {
				m.installCursor++
			}
		case " ":
			if m.installCursor == 0 {
				allSelected := true
				for i := range m.installInstances {
					if !m.installSel[i] {
						allSelected = false
						break
					}
				}
				for i := range m.installInstances {
					m.installSel[i] = !allSelected
				}
			} else {
				idx := m.installCursor - 1
				m.installSel[idx] = !m.installSel[idx]
			}
		case "enter":
			installed := 0
			skipped := 0
			for si, sk := range m.newSourceSkills {
				if !m.skillSel[si] {
					continue
				}
				for ii, inst := range m.installInstances {
					if !m.installSel[ii] {
						continue
					}
					targetPath := filepath.Join(inst.GlobalSkillsPath, sk.Name)
					if linker.ExistsAtTarget(targetPath) {
						skipped++
						continue
					}
					if err := linker.CreateSymlink(sk.OriginalPath, targetPath); err != nil {
						skipped++
						continue
					}
					_, _ = models.CreateInstallation(m.db, sk.ID, inst.ID, targetPath, sk.Name)
					installed++
				}
			}
			if skipped > 0 {
				m.status = fmt.Sprintf("Installed %d symlink(s), skipped %d collision(s)", installed, skipped)
			} else {
				m.status = fmt.Sprintf("Installed %d symlink(s)", installed)
			}
			m.currentView = sourcesViewList
		case "esc":
			m.status = "Source added. Skipped install."
			m.currentView = sourcesViewList
		}
	}
	return m, nil
}

// --- Post-add install views ---

func (m sourcesMenuModel) viewInstallPick() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Install Skills"))
	sb.WriteString("\n\n")
	sb.WriteString("  Select skills to install:\n\n")

	// Select all at top
	allLabel := "Select all"
	if m.skillCursor == 0 {
		sb.WriteString(selectedStyle.Render("  > " + allLabel))
	} else {
		sb.WriteString(subtleStyle.Render("    " + allLabel))
	}
	sb.WriteString("\n")

	for i, sk := range m.newSourceSkills {
		check := "[ ]"
		if m.skillSel[i] {
			check = "[x]"
		}
		desc := ""
		if sk.Description != "" {
			desc = subtleStyle.Render(" — " + sk.Description)
		}
		rowIdx := i + 1
		if rowIdx == m.skillCursor {
			sb.WriteString(selectedStyle.Render(fmt.Sprintf("  > %s %s", check, sk.Name)) + desc)
		} else {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("    %s %s", check, sk.Name)) + desc)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	if m.status != "" {
		sb.WriteString(successStyle.Render("  " + m.status))
		sb.WriteString("\n")
	}
	sb.WriteString(subtleStyle.Render("  space toggle • enter continue • esc skip"))

	return sb.String()
}

func (m sourcesMenuModel) viewInstallTargets() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Install To"))
	sb.WriteString("\n\n")
	sb.WriteString("  Select target instance(s):\n\n")

	if len(m.installInstances) == 0 {
		sb.WriteString(subtleStyle.Render("  No instances found. Add one in the Providers menu."))
		sb.WriteString("\n")
	} else {
		// Select all at top
		allLabel := "Select all"
		if m.installCursor == 0 {
			sb.WriteString(selectedStyle.Render("  > " + allLabel))
		} else {
			sb.WriteString(subtleStyle.Render("    " + allLabel))
		}
		sb.WriteString("\n")

		for i, inst := range m.installInstances {
			check := "[ ]"
			if m.installSel[i] {
				check = "[x]"
			}
			rowIdx := i + 1
			if rowIdx == m.installCursor {
				sb.WriteString(selectedStyle.Render(fmt.Sprintf("  > %s %s", check, inst.Name)) + "  " + subtleStyle.Render(inst.GlobalSkillsPath))
			} else {
				sb.WriteString(normalStyle.Render(fmt.Sprintf("    %s %s", check, inst.Name)) + "  " + subtleStyle.Render(inst.GlobalSkillsPath))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  space toggle • enter install • esc skip"))

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
