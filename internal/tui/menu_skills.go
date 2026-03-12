package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/linker"
	"github.com/vladyslav/skillreg/internal/models"
	"github.com/vladyslav/skillreg/internal/tui/components"
)

// skillsView enumerates the sub-views within the skills menu.
type skillsView int

const (
	skillsViewList           skillsView = iota
	skillsViewPopup                            // install/uninstall popup for a skill
	skillsViewInstallTargets                   // multi-select instances to install to
	skillsViewCollision                        // collision resolution
	skillsViewUninstallConfirm                 // confirm uninstall
)

// collisionChoice enumerates the user's options when a collision occurs.
type collisionChoice int

const (
	collisionBackupReplace collisionChoice = iota
	collisionReplace
	collisionRename
	collisionSkip
)

// skillsMenuModel is the BubbleTea model for the skills screen.
type skillsMenuModel struct {
	db           *db.Database
	skills       []*models.Skill
	cursor       int
	err          error
	status       string
	height       int // terminal height
	scrollOffset int // first visible item index

	currentView skillsView

	// Lookup maps
	sourceNames map[int64]string // sourceID → source name
	instMap     map[int64][]string // skillID → list of instance names where installed

	// Tabs
	tabs           []string // ["All", "source1", "source2", ...]
	tabSourceIDs   []int64  // [0, sourceID1, sourceID2, ...] (0 = All)
	activeTab      int
	filteredSkills []*models.Skill

	// Search
	searchInput  textinput.Model
	searching    bool
	searchQuery  string

	// Popup
	popupSkill   *models.Skill
	popupOptions []string
	popupCursor  int

	// Install flow
	selectedSkill  *models.Skill
	instances      []*models.Instance
	instanceSel    map[int]bool
	instanceCursor int

	// Collision handling
	collisionTarget   string
	collisionSource   string
	collisionInstance *models.Instance
	collisionCursor   int
	collisionOptions  []string
	renameInput       textinput.Model
	renameActive      bool
	pendingInstalls   []pendingInstall

	// Uninstall flow
	skillInstallations []*models.Installation
	confirm            components.ConfirmModel
}

// pendingInstall represents one install that still needs to happen.
type pendingInstall struct {
	skill    *models.Skill
	instance *models.Instance
}

// newSkillsMenu constructs a skillsMenuModel and loads skills from the DB.
func newSkillsMenu(d *db.Database) skillsMenuModel {
	ti := textinput.New()
	ti.Placeholder = "New name..."
	ti.CharLimit = 256

	si := textinput.New()
	si.Placeholder = "Search skills..."
	si.CharLimit = 256

	m := skillsMenuModel{
		db:          d,
		instanceSel: make(map[int]bool),
		renameInput: ti,
		searchInput: si,
		sourceNames: make(map[int64]string),
	}
	m.loadData()
	return m
}

func (m *skillsMenuModel) loadData() {
	// Sync all sources to pick up moved/added/removed skills
	_ = models.SyncAllSources(m.db)

	skills, err := models.ListAllSkills(m.db)
	if err != nil {
		m.err = err
		return
	}
	m.skills = skills
	m.err = nil

	// Build source name lookup
	m.sourceNames = make(map[int64]string)
	sources, _ := models.ListSources(m.db)
	for _, s := range sources {
		m.sourceNames[s.ID] = s.Name
	}

	// Build installation map
	m.instMap = m.buildInstallationMap()

	// Build tabs
	m.tabs = []string{"All"}
	m.tabSourceIDs = []int64{0}
	for _, s := range sources {
		m.tabs = append(m.tabs, s.Name)
		m.tabSourceIDs = append(m.tabSourceIDs, s.ID)
	}

	m.applyTabFilter()
}

func (m *skillsMenuModel) applyTabFilter() {
	var base []*models.Skill
	if m.activeTab == 0 || m.activeTab >= len(m.tabSourceIDs) {
		base = m.skills
	} else {
		srcID := m.tabSourceIDs[m.activeTab]
		for _, sk := range m.skills {
			if sk.SourceID == srcID {
				base = append(base, sk)
			}
		}
	}

	// Apply search filter
	if m.searchQuery != "" {
		q := strings.ToLower(m.searchQuery)
		m.filteredSkills = nil
		for _, sk := range base {
			if strings.Contains(strings.ToLower(sk.Name), q) ||
				strings.Contains(strings.ToLower(sk.Description), q) {
				m.filteredSkills = append(m.filteredSkills, sk)
			}
		}
	} else {
		m.filteredSkills = base
	}

	if m.cursor >= len(m.filteredSkills) {
		m.cursor = 0
	}
	m.scrollOffset = 0
}

// visibleCount returns how many skill items fit in the viewport.
// Each skill takes 2 lines (name + description), minus header/footer chrome.
func (m skillsMenuModel) visibleCount() int {
	// Header: title (1) + blank (1) + tabs (1) + blank (1) = 4 lines
	// Footer: blank (1) + status (1) + help (1) = 3 lines
	chrome := 7
	if len(m.tabs) <= 1 {
		chrome -= 2 // no tab bar
	}
	available := m.height - chrome
	if available < 2 {
		available = 2
	}
	// Each skill = 2 lines (name row + description row)
	return available / 2
}

func (m *skillsMenuModel) adjustScroll() {
	visible := m.visibleCount()
	if visible <= 0 {
		return
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
}

func (m skillsMenuModel) skillDisplayName(sk *models.Skill) string {
	srcName := m.sourceNames[sk.SourceID]
	if srcName != "" {
		return fmt.Sprintf("%s (%s)", sk.Name, srcName)
	}
	return sk.Name
}

func (m skillsMenuModel) isInstalled(sk *models.Skill) bool {
	names, ok := m.instMap[sk.ID]
	return ok && len(names) > 0
}

func (m skillsMenuModel) buildInstallationMap() map[int64][]string {
	instMap := make(map[int64][]string)
	installations, _ := models.ListAllInstallations(m.db)
	instances, _ := models.ListAllInstances(m.db)

	instNames := make(map[int64]string)
	for _, inst := range instances {
		instNames[inst.ID] = inst.Name
	}

	for _, installation := range installations {
		name := instNames[installation.InstanceID]
		if name == "" {
			name = "unknown"
		}
		instMap[installation.SkillID] = append(instMap[installation.SkillID], name)
	}
	return instMap
}

func (m skillsMenuModel) update(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch m.currentView {
	case skillsViewList:
		return m.updateList(msg)
	case skillsViewPopup:
		return m.updatePopup(msg)
	case skillsViewInstallTargets:
		return m.updateInstallTargets(msg)
	case skillsViewCollision:
		return m.updateCollision(msg)
	case skillsViewUninstallConfirm:
		return m.updateUninstallConfirm(msg)
	}
	return m, nil
}

// --- List view ---

func (m skillsMenuModel) updateList(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Search mode: delegate most keys to the text input
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.searchInput.Blur()
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.applyTabFilter()
				return m, nil
			case "enter":
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			case "up", "down":
				// Allow navigation while search is active
				m.searching = false
				m.searchInput.Blur()
				// Fall through to normal key handling below
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.searchQuery = m.searchInput.Value()
				m.applyTabFilter()
				return m, cmd
			}
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}
		case "down", "j":
			if m.cursor < len(m.filteredSkills)-1 {
				m.cursor++
				m.adjustScroll()
			}
		case "left", "h":
			if len(m.tabs) > 1 {
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				m.applyTabFilter()
			}
		case "right", "l":
			if len(m.tabs) > 1 {
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
				m.applyTabFilter()
			}
		case "tab":
			if len(m.tabs) > 1 {
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
				m.applyTabFilter()
			}
		case "shift+tab":
			if len(m.tabs) > 1 {
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				m.applyTabFilter()
			}
		case "/":
			m.searching = true
			m.searchInput.Focus()
			return m, nil
		case "enter":
			if len(m.filteredSkills) > 0 && m.cursor < len(m.filteredSkills) {
				sk := m.filteredSkills[m.cursor]
				m.popupSkill = sk
				m.popupCursor = 0
				installed := m.isInstalled(sk)
				if installed {
					m.popupOptions = []string{"Install to more...", "Uninstall"}
				} else {
					m.popupOptions = []string{"Install"}
				}
				m.currentView = skillsViewPopup
				m.status = ""
			}
		case "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.applyTabFilter()
				return m, nil
			}
			return m, navigate(viewMain)
		case "q":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.applyTabFilter()
				return m, nil
			}
			return m, navigate(viewMain)
		}
	}
	return m, nil
}

// --- Popup view ---

func (m skillsMenuModel) updatePopup(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.popupCursor > 0 {
				m.popupCursor--
			}
		case "down", "j":
			if m.popupCursor < len(m.popupOptions)-1 {
				m.popupCursor++
			}
		case "enter":
			selected := m.popupOptions[m.popupCursor]
			switch selected {
			case "Install", "Install to more...":
				m.selectedSkill = m.popupSkill
				instances, _ := models.ListAllInstances(m.db)
				m.instances = instances
				m.instanceSel = make(map[int]bool)
				// Pre-select all for fresh install, none for "install to more"
				if selected == "Install" {
					for i := range instances {
						m.instanceSel[i] = true
					}
				}
				m.instanceCursor = 0
				m.currentView = skillsViewInstallTargets
			case "Uninstall":
				installs, _ := models.ListInstallationsBySkill(m.db, m.popupSkill.ID)
				m.skillInstallations = installs
				if len(installs) == 0 {
					m.status = "No installations found"
					m.currentView = skillsViewList
					return m, nil
				}
				instanceNames := make(map[int64]string)
				allInstances, _ := models.ListAllInstances(m.db)
				for _, inst := range allInstances {
					instanceNames[inst.ID] = inst.Name
				}
				var names []string
				for _, inst := range installs {
					name := instanceNames[inst.InstanceID]
					if name == "" {
						name = "unknown"
					}
					names = append(names, name)
				}
				m.confirm = components.NewConfirm(
					fmt.Sprintf("Uninstall %q from %s?", m.popupSkill.Name, strings.Join(names, ", ")),
				)
				m.currentView = skillsViewUninstallConfirm
			}
		case "esc", "q":
			m.currentView = skillsViewList
		}
	}
	return m, nil
}

// --- Install targets view (multi-select instances) ---

func (m skillsMenuModel) updateInstallTargets(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	totalRows := len(m.instances) + 1 // +1 for "Select all" at top

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.instanceCursor > 0 {
				m.instanceCursor--
			}
		case "down", "j":
			if m.instanceCursor < totalRows-1 {
				m.instanceCursor++
			}
		case " ":
			if m.instanceCursor == 0 {
				// Toggle all
				allSelected := true
				for i := range m.instances {
					if !m.instanceSel[i] {
						allSelected = false
						break
					}
				}
				for i := range m.instances {
					m.instanceSel[i] = !allSelected
				}
			} else {
				idx := m.instanceCursor - 1
				m.instanceSel[idx] = !m.instanceSel[idx]
			}
		case "enter":
			return m.executeInstall()
		case "esc", "q":
			m.currentView = skillsViewPopup
			m.status = ""
		}
	}
	return m, nil
}

func (m skillsMenuModel) executeInstall() (skillsMenuModel, tea.Cmd) {
	sk := m.selectedSkill
	var pending []pendingInstall

	for i, inst := range m.instances {
		if !m.instanceSel[i] {
			continue
		}
		pending = append(pending, pendingInstall{skill: sk, instance: inst})
	}

	if len(pending) == 0 {
		m.status = "No instances selected"
		return m, nil
	}

	m.pendingInstalls = pending
	return m.processNextInstall()
}

func (m skillsMenuModel) processNextInstall() (skillsMenuModel, tea.Cmd) {
	for len(m.pendingInstalls) > 0 {
		p := m.pendingInstalls[0]
		m.pendingInstalls = m.pendingInstalls[1:]

		targetDir := p.instance.GlobalSkillsPath
		targetPath := filepath.Join(targetDir, p.skill.Name)

		// Check for collision
		if linker.ExistsAtTarget(targetPath) {
			m.collisionTarget = targetPath
			m.collisionSource = p.skill.OriginalPath
			m.collisionInstance = p.instance
			m.collisionCursor = 0
			m.renameActive = false

			if linker.IsDirectory(targetPath) {
				m.collisionOptions = []string{"Backup & replace", "Skip"}
			} else if linker.IsSymlink(targetPath) {
				m.collisionOptions = []string{"Replace symlink", "Rename and install", "Skip"}
			} else {
				m.collisionOptions = []string{"Backup & replace", "Skip"}
			}

			m.pendingInstalls = append([]pendingInstall{{skill: m.selectedSkill, instance: p.instance}}, m.pendingInstalls...)
			m.currentView = skillsViewCollision
			return m, nil
		}

		if err := linker.CreateSymlink(p.skill.OriginalPath, targetPath); err != nil {
			m.status = fmt.Sprintf("Error creating symlink: %v", err)
			continue
		}
		_, _ = models.CreateInstallation(m.db, p.skill.ID, p.instance.ID, targetPath, p.skill.Name)
	}

	count := 0
	for _, sel := range m.instanceSel {
		if sel {
			count++
		}
	}
	m.status = fmt.Sprintf("Installed %q to %d instance(s)", m.selectedSkill.Name, count)
	m.loadData()
	m.currentView = skillsViewList
	return m, nil
}

// --- Collision view ---

func (m skillsMenuModel) updateCollision(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	if m.renameActive {
		return m.updateCollisionRename(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.collisionCursor > 0 {
				m.collisionCursor--
			}
		case "down", "j":
			if m.collisionCursor < len(m.collisionOptions)-1 {
				m.collisionCursor++
			}
		case "enter":
			return m.executeCollisionChoice()
		case "esc":
			if len(m.pendingInstalls) > 0 {
				m.pendingInstalls = m.pendingInstalls[1:]
			}
			return m.processNextInstall()
		}
	}
	return m, nil
}

func (m skillsMenuModel) updateCollisionRename(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			newName := strings.TrimSpace(m.renameInput.Value())
			if newName == "" {
				m.status = "Name cannot be empty"
				return m, nil
			}
			if len(m.pendingInstalls) > 0 {
				p := m.pendingInstalls[0]
				m.pendingInstalls = m.pendingInstalls[1:]

				targetPath := filepath.Join(p.instance.GlobalSkillsPath, newName)
				if err := linker.CreateSymlink(p.skill.OriginalPath, targetPath); err != nil {
					m.status = fmt.Sprintf("Error: %v", err)
				} else {
					_, _ = models.CreateInstallation(m.db, p.skill.ID, p.instance.ID, targetPath, newName)
				}
			}
			m.renameActive = false
			return m.processNextInstall()
		}
		if msg.String() == "esc" {
			m.renameActive = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m skillsMenuModel) executeCollisionChoice() (skillsMenuModel, tea.Cmd) {
	selected := m.collisionOptions[m.collisionCursor]

	switch selected {
	case "Backup & replace":
		if len(m.pendingInstalls) > 0 {
			p := m.pendingInstalls[0]
			m.pendingInstalls = m.pendingInstalls[1:]

			if err := linker.BackupAndReplace(p.skill.OriginalPath, m.collisionTarget); err != nil {
				m.status = fmt.Sprintf("Backup & replace error: %v", err)
			} else {
				_, _ = models.CreateInstallation(m.db, p.skill.ID, p.instance.ID, m.collisionTarget, p.skill.Name)
			}
		}
		return m.processNextInstall()

	case "Replace symlink":
		if len(m.pendingInstalls) > 0 {
			p := m.pendingInstalls[0]
			m.pendingInstalls = m.pendingInstalls[1:]

			_ = linker.RemoveSymlink(m.collisionTarget)
			if err := linker.CreateSymlink(p.skill.OriginalPath, m.collisionTarget); err != nil {
				m.status = fmt.Sprintf("Replace error: %v", err)
			} else {
				_, _ = models.CreateInstallation(m.db, p.skill.ID, p.instance.ID, m.collisionTarget, p.skill.Name)
			}
		}
		return m.processNextInstall()

	case "Rename and install":
		m.renameInput.SetValue("")
		m.renameInput.Focus()
		m.renameActive = true
		return m, m.renameInput.Cursor.BlinkCmd()

	case "Skip":
		if len(m.pendingInstalls) > 0 {
			m.pendingInstalls = m.pendingInstalls[1:]
		}
		return m.processNextInstall()
	}
	return m, nil
}

// --- Uninstall confirm ---

func (m skillsMenuModel) updateUninstallConfirm(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case components.ConfirmResultMsg:
		if msg.Confirmed {
			removed := 0
			for _, inst := range m.skillInstallations {
				_ = linker.RemoveSymlink(inst.SymlinkPath)
				_ = models.DeleteInstallation(m.db, inst.ID)
				removed++
			}
			m.status = fmt.Sprintf("Removed %d installation(s)", removed)
			m.loadData()
		} else {
			m.status = ""
		}
		m.currentView = skillsViewList
		return m, nil
	}

	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)
	return m, cmd
}

// --- Views ---

func (m skillsMenuModel) view() string {
	switch m.currentView {
	case skillsViewPopup:
		return m.viewPopup()
	case skillsViewInstallTargets:
		return m.viewInstallTargets()
	case skillsViewCollision:
		return m.viewCollision()
	case skillsViewUninstallConfirm:
		return m.viewUninstallConfirm()
	default:
		return m.viewList()
	}
}

func (m skillsMenuModel) viewList() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Skills"))
	sb.WriteString("\n\n")

	// Tab bar
	if len(m.tabs) > 1 {
		var tabParts []string
		for i, label := range m.tabs {
			if i == m.activeTab {
				tabParts = append(tabParts, activeTabStyle.Render(label))
			} else {
				tabParts = append(tabParts, inactiveTabStyle.Render(label))
			}
		}
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabParts...))
		sb.WriteString("\n\n")
	}

	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error loading skills: %v", m.err)))
		sb.WriteString("\n")
	} else if len(m.filteredSkills) == 0 {
		sb.WriteString(subtleStyle.Render("  No skills found."))
		sb.WriteString("\n")
	} else {
		total := len(m.filteredSkills)
		visible := m.visibleCount()
		end := m.scrollOffset + visible
		if end > total {
			end = total
		}

		if m.scrollOffset > 0 {
			sb.WriteString(subtleStyle.Render("    ↑ more"))
			sb.WriteString("\n")
		}

		for i := m.scrollOffset; i < end; i++ {
			sk := m.filteredSkills[i]
			// Install status icon
			var icon string
			installed := m.isInstalled(sk)
			if installed {
				icon = successStyle.Render("● ")
			} else {
				icon = subtleStyle.Render("○ ")
			}

			name := m.skillDisplayName(sk)

			// Show where installed
			installedTo := ""
			if names, ok := m.instMap[sk.ID]; ok && len(names) > 0 {
				installedTo = successStyle.Render(" [" + strings.Join(names, ", ") + "]")
			}

			// Row 1: name + installations
			if i == m.cursor {
				sb.WriteString(selectedStyle.Render("  > ") + icon + selectedStyle.Render(name) + installedTo)
			} else {
				sb.WriteString(normalStyle.Render("    ") + icon + normalStyle.Render(name) + installedTo)
			}
			sb.WriteString("\n")

			// Row 2: description (indented)
			if sk.Description != "" {
				sb.WriteString(subtleStyle.Render("      " + sk.Description))
				sb.WriteString("\n")
			}
		}

		if end < total {
			sb.WriteString(subtleStyle.Render("    ↓ more"))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	if m.searching {
		sb.WriteString("  " + m.searchInput.View())
		sb.WriteString("\n")
	} else if m.searchQuery != "" {
		sb.WriteString(subtleStyle.Render(fmt.Sprintf("  filter: %q (esc to clear)", m.searchQuery)))
		sb.WriteString("\n")
	}
	if m.status != "" {
		sb.WriteString(components.StatusBar(m.status, 60))
		sb.WriteString("\n")
	}
	sb.WriteString(subtleStyle.Render("←/→ source • ↑/↓ navigate • / search • enter select • esc back"))

	return sb.String()
}

func (m skillsMenuModel) viewPopup() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Skills"))
	sb.WriteString("\n\n")

	sk := m.popupSkill
	name := m.skillDisplayName(sk)

	var popupContent strings.Builder
	popupContent.WriteString(fmt.Sprintf("  %s\n", name))
	if sk.Description != "" {
		popupContent.WriteString(fmt.Sprintf("  %s\n", subtleStyle.Render(sk.Description)))
	}

	// Show where installed
	if names, ok := m.instMap[sk.ID]; ok && len(names) > 0 {
		popupContent.WriteString(fmt.Sprintf("  Installed: %s\n", successStyle.Render(strings.Join(names, ", "))))
	}

	popupContent.WriteString("\n")
	for i, opt := range m.popupOptions {
		if i == m.popupCursor {
			popupContent.WriteString(selectedStyle.Render("  > " + opt))
		} else {
			popupContent.WriteString(normalStyle.Render("    " + opt))
		}
		popupContent.WriteString("\n")
	}

	sb.WriteString(boxStyle.Render(popupContent.String()))

	sb.WriteString("\n\n")
	sb.WriteString(subtleStyle.Render("↑/↓ navigate • enter confirm • esc cancel"))

	return sb.String()
}

func (m skillsMenuModel) viewInstallTargets() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Install: " + m.selectedSkill.Name))
	sb.WriteString("\n\n")
	sb.WriteString("  Select target instance(s):\n\n")

	if len(m.instances) == 0 {
		sb.WriteString(subtleStyle.Render("  No instances found. Add one in the Providers menu."))
		sb.WriteString("\n")
	} else {
		// Select all at top
		allLabel := "Select all"
		if m.instanceCursor == 0 {
			sb.WriteString(selectedStyle.Render("  > " + allLabel))
		} else {
			sb.WriteString(subtleStyle.Render("    " + allLabel))
		}
		sb.WriteString("\n")

		for i, inst := range m.instances {
			check := "[ ]"
			if m.instanceSel[i] {
				check = "[x]"
			}
			rowIdx := i + 1
			if rowIdx == m.instanceCursor {
				sb.WriteString(selectedStyle.Render("  > "+check+" "+inst.Name) + "  " + subtleStyle.Render(inst.GlobalSkillsPath))
			} else {
				line := fmt.Sprintf("%s %s  %s", check, inst.Name, subtleStyle.Render(inst.GlobalSkillsPath))
				sb.WriteString(normalStyle.Render("    " + line))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	if m.status != "" {
		sb.WriteString(warningStyle.Render("  " + m.status))
		sb.WriteString("\n")
	}
	sb.WriteString(subtleStyle.Render("  space toggle • enter confirm • esc back"))

	return sb.String()
}

func (m skillsMenuModel) viewCollision() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Collision Detected"))
	sb.WriteString("\n\n")
	sb.WriteString(warningStyle.Render(fmt.Sprintf("  Target already exists: %s", m.collisionTarget)))
	sb.WriteString("\n\n")

	if m.renameActive {
		sb.WriteString("  Enter new name:\n\n")
		sb.WriteString("  " + m.renameInput.View())
		sb.WriteString("\n\n")
		sb.WriteString(subtleStyle.Render("  enter confirm • esc cancel"))
		return sb.String()
	}

	for i, opt := range m.collisionOptions {
		if i == m.collisionCursor {
			sb.WriteString(selectedStyle.Render("  > " + opt))
		} else {
			sb.WriteString(normalStyle.Render("    " + opt))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  ↑/↓ navigate • enter select • esc skip"))

	return sb.String()
}

func (m skillsMenuModel) viewUninstallConfirm() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Confirm Uninstall"))
	sb.WriteString("\n")
	sb.WriteString(m.confirm.View())
	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  ←/→ switch • y/n shortcut • enter confirm"))
	return sb.String()
}
