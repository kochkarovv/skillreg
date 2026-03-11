package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/linker"
	"github.com/vladyslav/skillreg/internal/models"
	"github.com/vladyslav/skillreg/internal/tui/components"
)

// skillsView enumerates the sub-views within the skills menu.
type skillsView int

const (
	skillsViewList            skillsView = iota
	skillsViewInstallSelect                     // pick a skill to install
	skillsViewInstallTargets                    // multi-select instances
	skillsViewUninstallSelect                   // multi-select installed skills to remove
	skillsViewCollision                         // collision resolution
	skillsViewConfirmUninstall                  // confirm uninstall
)

// Top-level action items in the skills list view.
const (
	skillsActionBrowse    = 0
	skillsActionInstall   = 1
	skillsActionUninstall = 2
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
	db     *db.Database
	skills []*models.Skill
	cursor int
	err    error
	status string

	currentView skillsView

	// Lookup maps built on load
	sourceNames map[int64]string // sourceID → source name

	// Action row selection in list view
	actionCursor int
	actions      []string
	inActions    bool // true when cursor is on action row area

	// Install flow
	installSkills  []*models.Skill  // all available skills
	installCursor  int
	selectedSkill  *models.Skill    // chosen skill
	instances      []*models.Instance
	instanceSel    map[int]bool     // selected instance indices
	instanceCursor int

	// Collision handling
	collisionTarget    string            // target path that collided
	collisionSource    string            // skill original path
	collisionInstance  *models.Instance
	collisionCursor    int
	collisionOptions   []string
	renameInput        textinput.Model
	renameActive       bool
	pendingInstalls    []pendingInstall  // remaining installs after collision

	// Uninstall flow
	installations      []*installationRow
	uninstallSel       map[int]bool
	uninstallCursor    int

	// Confirm uninstall
	confirm components.ConfirmModel
}

// pendingInstall represents one install that still needs to happen.
type pendingInstall struct {
	skill    *models.Skill
	instance *models.Instance
}

// installationRow is a display row for installations.
type installationRow struct {
	installation *models.Installation
	skillName    string
	instanceName string
}

// newSkillsMenu constructs a skillsMenuModel and loads skills from the DB.
func newSkillsMenu(d *db.Database) skillsMenuModel {
	ti := textinput.New()
	ti.Placeholder = "New name..."
	ti.CharLimit = 256

	m := skillsMenuModel{
		db:          d,
		actions:     []string{"Browse all", "Install skill", "Uninstall skill"},
		instanceSel: make(map[int]bool),
		uninstallSel: make(map[int]bool),
		renameInput: ti,
		sourceNames: make(map[int64]string),
	}
	m.loadData()
	return m
}

func (m *skillsMenuModel) loadData() {
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
}

func (m skillsMenuModel) skillDisplayName(sk *models.Skill) string {
	srcName := m.sourceNames[sk.SourceID]
	if srcName != "" {
		return fmt.Sprintf("%s (%s)", sk.Name, srcName)
	}
	return sk.Name
}

// totalListRows: actions + skills
func (m skillsMenuModel) totalListRows() int {
	return len(m.actions) + len(m.skills)
}

func (m skillsMenuModel) update(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch m.currentView {
	case skillsViewList:
		return m.updateList(msg)
	case skillsViewInstallSelect:
		return m.updateInstallSelect(msg)
	case skillsViewInstallTargets:
		return m.updateInstallTargets(msg)
	case skillsViewUninstallSelect:
		return m.updateUninstallSelect(msg)
	case skillsViewCollision:
		return m.updateCollision(msg)
	case skillsViewConfirmUninstall:
		return m.updateConfirmUninstall(msg)
	}
	return m, nil
}

// --- List view ---

func (m skillsMenuModel) updateList(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < m.totalListRows()-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor < len(m.actions) {
				// Action selected
				switch m.cursor {
				case skillsActionBrowse:
					// Already showing all skills, do nothing special
				case skillsActionInstall:
					m.installSkills = m.skills
					m.installCursor = 0
					m.currentView = skillsViewInstallSelect
					m.status = ""
				case skillsActionUninstall:
					m.loadInstallations()
					m.uninstallCursor = 0
					m.uninstallSel = make(map[int]bool)
					m.currentView = skillsViewUninstallSelect
					m.status = ""
				}
			}
		case "esc", "q":
			return m, navigate(viewMain)
		}
	}
	return m, nil
}

// --- Install select view ---

func (m skillsMenuModel) updateInstallSelect(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.installCursor > 0 {
				m.installCursor--
			}
		case "down", "j":
			if m.installCursor < len(m.installSkills)-1 {
				m.installCursor++
			}
		case "enter":
			if len(m.installSkills) > 0 {
				m.selectedSkill = m.installSkills[m.installCursor]
				// Load instances
				instances, _ := models.ListAllInstances(m.db)
				m.instances = instances
				m.instanceSel = make(map[int]bool)
				m.instanceCursor = 0
				m.currentView = skillsViewInstallTargets
			}
		case "esc", "q":
			m.currentView = skillsViewList
			m.status = ""
		}
	}
	return m, nil
}

// --- Install targets view (multi-select instances) ---

func (m skillsMenuModel) updateInstallTargets(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	// Extra rows: "Select all" at end
	totalRows := len(m.instances) + 1

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
			if m.instanceCursor < len(m.instances) {
				m.instanceSel[m.instanceCursor] = !m.instanceSel[m.instanceCursor]
			} else {
				// Select all toggle
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
			}
		case "enter":
			return m.executeInstall()
		case "esc", "q":
			m.currentView = skillsViewInstallSelect
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
			// Collision detected
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

			// Put this install back as the first pending (we'll process it after resolution)
			m.pendingInstalls = append([]pendingInstall{{skill: m.selectedSkill, instance: p.instance}}, m.pendingInstalls...)
			m.currentView = skillsViewCollision
			return m, nil
		}

		// No collision — create symlink and DB record
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
			// Skip this collision entirely
			if len(m.pendingInstalls) > 0 {
				m.pendingInstalls = m.pendingInstalls[1:] // remove the colliding install
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
			// Remove the colliding pending install and install with new name
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

// --- Uninstall select view ---

func (m *skillsMenuModel) loadInstallations() {
	allInstalls, _ := models.ListAllInstallations(m.db)
	allInstances, _ := models.ListAllInstances(m.db)

	// Build instance name lookup
	instNames := make(map[int64]string)
	for _, inst := range allInstances {
		instNames[inst.ID] = inst.Name
	}

	// Build skill name lookup
	skillNames := make(map[int64]string)
	for _, sk := range m.skills {
		skillNames[sk.ID] = m.skillDisplayName(sk)
	}

	rows := make([]*installationRow, 0, len(allInstalls))
	for _, inst := range allInstalls {
		rows = append(rows, &installationRow{
			installation: inst,
			skillName:    skillNames[inst.SkillID],
			instanceName: instNames[inst.InstanceID],
		})
	}
	m.installations = rows
}

func (m skillsMenuModel) updateUninstallSelect(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.uninstallCursor > 0 {
				m.uninstallCursor--
			}
		case "down", "j":
			if m.uninstallCursor < len(m.installations)-1 {
				m.uninstallCursor++
			}
		case " ":
			if m.uninstallCursor < len(m.installations) {
				m.uninstallSel[m.uninstallCursor] = !m.uninstallSel[m.uninstallCursor]
			}
		case "enter":
			count := 0
			for _, sel := range m.uninstallSel {
				if sel {
					count++
				}
			}
			if count == 0 {
				m.status = "No installations selected"
				return m, nil
			}
			m.confirm = components.NewConfirm(
				fmt.Sprintf("Remove %d installation(s)?", count),
			)
			m.currentView = skillsViewConfirmUninstall
		case "esc", "q":
			m.currentView = skillsViewList
			m.status = ""
		}
	}
	return m, nil
}

// --- Confirm uninstall ---

func (m skillsMenuModel) updateConfirmUninstall(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case components.ConfirmResultMsg:
		if msg.Confirmed {
			removed := 0
			for i, row := range m.installations {
				if !m.uninstallSel[i] {
					continue
				}
				_ = linker.RemoveSymlink(row.installation.SymlinkPath)
				_ = models.DeleteInstallation(m.db, row.installation.ID)
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
	case skillsViewInstallSelect:
		return m.viewInstallSelect()
	case skillsViewInstallTargets:
		return m.viewInstallTargets()
	case skillsViewUninstallSelect:
		return m.viewUninstallSelect()
	case skillsViewCollision:
		return m.viewCollision()
	case skillsViewConfirmUninstall:
		return m.viewConfirmUninstall()
	default:
		return m.viewList()
	}
}

func (m skillsMenuModel) viewList() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Skills"))
	sb.WriteString("\n\n")

	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error loading skills: %v", m.err)))
		sb.WriteString("\n")
	} else {
		// Action items
		for i, action := range m.actions {
			if i == m.cursor {
				sb.WriteString(selectedStyle.Render("  > " + action))
			} else {
				sb.WriteString(subtleStyle.Render("    " + action))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")

		// Skills list
		if len(m.skills) == 0 {
			sb.WriteString(subtleStyle.Render("  No skills found."))
			sb.WriteString("\n")
		} else {
			// Build installation lookup: skillID → list of instance names
			instMap := m.buildInstallationMap()

			for i, sk := range m.skills {
				rowIdx := len(m.actions) + i
				name := m.skillDisplayName(sk)
				desc := ""
				if sk.Description != "" {
					desc = " — " + sk.Description
				}

				// Show where installed
				installedTo := ""
				if names, ok := instMap[sk.ID]; ok && len(names) > 0 {
					installedTo = successStyle.Render(" [" + strings.Join(names, ", ") + "]")
				}

				line := name + desc + installedTo
				if rowIdx == m.cursor {
					sb.WriteString(selectedStyle.Render("  > "+name+desc) + installedTo)
				} else {
					sb.WriteString(normalStyle.Render("    " + line))
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n")
	if m.status != "" {
		sb.WriteString(components.StatusBar(m.status, 60))
		sb.WriteString("\n")
	}
	sb.WriteString(subtleStyle.Render("↑/↓ navigate • enter select • esc back"))

	return sb.String()
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

func (m skillsMenuModel) viewInstallSelect() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Install Skill"))
	sb.WriteString("\n\n")
	sb.WriteString("  Select a skill to install:\n\n")

	if len(m.installSkills) == 0 {
		sb.WriteString(subtleStyle.Render("  No skills available."))
		sb.WriteString("\n")
	} else {
		for i, sk := range m.installSkills {
			name := m.skillDisplayName(sk)
			desc := ""
			if sk.Description != "" {
				desc = subtleStyle.Render(" — " + sk.Description)
			}
			if i == m.installCursor {
				sb.WriteString(selectedStyle.Render("  > " + name) + desc)
			} else {
				sb.WriteString(normalStyle.Render("    " + name) + desc)
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  ↑/↓ navigate • enter select • esc back"))

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
		for i, inst := range m.instances {
			check := "[ ]"
			if m.instanceSel[i] {
				check = "[x]"
			}
			line := fmt.Sprintf("%s %s  %s", check, inst.Name, subtleStyle.Render(inst.GlobalSkillsPath))
			if i == m.instanceCursor {
				sb.WriteString(selectedStyle.Render("  > "+check+" "+inst.Name) + "  " + subtleStyle.Render(inst.GlobalSkillsPath))
			} else {
				sb.WriteString(normalStyle.Render("    " + line))
			}
			sb.WriteString("\n")
		}

		// Select all row
		allIdx := len(m.instances)
		allLabel := "Select all"
		if m.instanceCursor == allIdx {
			sb.WriteString(selectedStyle.Render("  > " + allLabel))
		} else {
			sb.WriteString(subtleStyle.Render("    " + allLabel))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	if m.status != "" {
		sb.WriteString(warningStyle.Render("  " + m.status))
		sb.WriteString("\n")
	}
	sb.WriteString(subtleStyle.Render("  space toggle • enter confirm • esc back"))

	return sb.String()
}

func (m skillsMenuModel) viewUninstallSelect() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Uninstall Skills"))
	sb.WriteString("\n\n")

	if len(m.installations) == 0 {
		sb.WriteString(subtleStyle.Render("  No installations found."))
		sb.WriteString("\n")
	} else {
		sb.WriteString("  Select installation(s) to remove:\n\n")
		for i, row := range m.installations {
			check := "[ ]"
			if m.uninstallSel[i] {
				check = "[x]"
			}
			line := fmt.Sprintf("%s %s → %s", check, row.skillName, row.instanceName)
			if i == m.uninstallCursor {
				sb.WriteString(selectedStyle.Render("  > " + line))
			} else {
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

func (m skillsMenuModel) viewConfirmUninstall() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Confirm Uninstall"))
	sb.WriteString("\n")
	sb.WriteString(m.confirm.View())
	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  ←/→ switch • y/n shortcut • enter confirm"))
	return sb.String()
}
