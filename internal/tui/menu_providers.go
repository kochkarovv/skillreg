package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/linker"
	"github.com/vladyslav/skillreg/internal/models"
	"github.com/vladyslav/skillreg/internal/tui/components"
)

// providersView enumerates the sub-views within the providers menu.
type providersView int

const (
	providersViewList           providersView = iota
	providersViewInstanceDetail                       // detail for selected instance
	providersViewAddInstance                           // text inputs for new instance
	providersViewScanHome                             // auto-discovered instances
	providersViewConfirmRemove                        // confirm instance removal
)

// providerNode groups a provider with its instances for display purposes.
type providerNode struct {
	provider  *models.Provider
	instances []*models.Instance
}

// flatRow is one selectable row in the providers tree view.
type flatRow struct {
	kind       string // "provider" (header, not selectable), "instance", "add"
	provider   *models.Provider
	instance   *models.Instance
}

// discoveredInstance is a potential instance found during home scan.
type discoveredInstance struct {
	provider *models.Provider
	name     string
	path     string
}

// providersMenuModel is the BubbleTea model for the providers screen.
type providersMenuModel struct {
	db     *db.Database
	nodes  []providerNode
	rows   []flatRow // flattened for navigation
	cursor int
	err    error
	status string

	currentView providersView

	// Instance detail
	selectedInstance *models.Instance
	selectedProvider *models.Provider
	instSkills       []*installDetailRow // installed skills for the instance
	detailCursor     int
	detailActions    []string

	// Add instance
	addProvider    *models.Provider
	nameInput      textinput.Model
	pathInput      textinput.Model
	addStep        int // 0 = name, 1 = path
	suggestedPath  string

	// Scan home
	discovered      []discoveredInstance
	scanSel         map[int]bool
	scanCursor      int

	// Confirm remove
	confirm components.ConfirmModel

	// Auto-scan flag: triggers home scan on first list view render
	needsAutoScan bool
}

// installDetailRow is a display row for an installed skill on an instance.
type installDetailRow struct {
	installation *models.Installation
	skillName    string
}

// newProvidersMenu constructs a providersMenuModel and loads data from the DB.
func newProvidersMenu(d *db.Database) providersMenuModel {
	nameTI := textinput.New()
	nameTI.Placeholder = "Instance name..."
	nameTI.CharLimit = 256

	pathTI := textinput.New()
	pathTI.Placeholder = "Path to global skills directory..."
	pathTI.CharLimit = 512

	m := providersMenuModel{
		db:            d,
		nameInput:     nameTI,
		pathInput:     pathTI,
		scanSel:       make(map[int]bool),
		needsAutoScan: true,
	}
	m.loadData()
	return m
}

func (m *providersMenuModel) loadData() {
	providers, err := models.ListProviders(m.db)
	if err != nil {
		m.err = err
		return
	}

	nodes := make([]providerNode, 0, len(providers))
	for _, p := range providers {
		instances, _ := models.ListInstancesByProvider(m.db, p.ID)
		nodes = append(nodes, providerNode{provider: p, instances: instances})
	}
	m.nodes = nodes
	m.err = nil
	m.buildFlatRows()
}

func (m *providersMenuModel) buildFlatRows() {
	var rows []flatRow
	for _, node := range m.nodes {
		// Provider header row
		rows = append(rows, flatRow{kind: "provider", provider: node.provider})
		// Instance rows
		for _, inst := range node.instances {
			rows = append(rows, flatRow{kind: "instance", provider: node.provider, instance: inst})
		}
		// [Add instance] row per provider
		rows = append(rows, flatRow{kind: "add", provider: node.provider})
	}
	m.rows = rows
}

// selectableRow returns true if the row at index i can be selected.
func (m providersMenuModel) selectableRow(i int) bool {
	if i < 0 || i >= len(m.rows) {
		return false
	}
	return m.rows[i].kind != "provider"
}

func (m providersMenuModel) update(msg tea.Msg) (providersMenuModel, tea.Cmd) {
	switch m.currentView {
	case providersViewList:
		return m.updateList(msg)
	case providersViewInstanceDetail:
		return m.updateInstanceDetail(msg)
	case providersViewAddInstance:
		return m.updateAddInstance(msg)
	case providersViewScanHome:
		return m.updateScanHome(msg)
	case providersViewConfirmRemove:
		return m.updateConfirmRemove(msg)
	}
	return m, nil
}

// --- List view ---

func (m providersMenuModel) updateList(msg tea.Msg) (providersMenuModel, tea.Cmd) {
	// Auto-scan on first entry to detect unregistered instances
	if m.needsAutoScan {
		m.needsAutoScan = false
		m.runHomeScan()
		if len(m.discovered) > 0 {
			m.scanCursor = 0
			m.scanSel = make(map[int]bool)
			// Pre-select all discovered instances
			for i := range m.discovered {
				m.scanSel[i] = true
			}
			m.currentView = providersViewScanHome
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			for i := m.cursor - 1; i >= 0; i-- {
				if m.selectableRow(i) {
					m.cursor = i
					break
				}
			}
		case "down", "j":
			for i := m.cursor + 1; i < len(m.rows); i++ {
				if m.selectableRow(i) {
					m.cursor = i
					break
				}
			}
		case "enter":
			if m.cursor < len(m.rows) {
				row := m.rows[m.cursor]
				switch row.kind {
				case "instance":
					m.selectedInstance = row.instance
					m.selectedProvider = row.provider
					m.loadInstanceDetail()
					m.detailCursor = 0
					m.currentView = providersViewInstanceDetail
					m.status = ""

				case "add":
					m.addProvider = row.provider
					// Check if there are undiscovered instances for this provider
					m.runHomeScan()
					var providerDiscovered []discoveredInstance
					for _, d := range m.discovered {
						if d.provider.ID == row.provider.ID {
							providerDiscovered = append(providerDiscovered, d)
						}
					}
					if len(providerDiscovered) > 0 {
						m.discovered = providerDiscovered
						m.scanCursor = 0
						m.scanSel = make(map[int]bool)
						for i := range m.discovered {
							m.scanSel[i] = true
						}
						m.currentView = providersViewScanHome
						return m, nil
					}
					m.prepareAddInstance(row.provider)
					m.currentView = providersViewAddInstance
					return m, m.nameInput.Cursor.BlinkCmd()
				}
			}
		case "esc", "q":
			return m, navigate(viewMain)
		}
	}
	return m, nil
}

func (m *providersMenuModel) loadInstanceDetail() {
	installs, _ := models.ListInstallationsByInstance(m.db, m.selectedInstance.ID)
	skills, _ := models.ListAllSkills(m.db)

	skillNames := make(map[int64]string)
	for _, sk := range skills {
		skillNames[sk.ID] = sk.Name
	}

	rows := make([]*installDetailRow, 0, len(installs))
	for _, inst := range installs {
		rows = append(rows, &installDetailRow{
			installation: inst,
			skillName:    skillNames[inst.SkillID],
		})
	}
	m.instSkills = rows
	m.detailActions = []string{"Remove instance"}
}

// --- Instance detail view ---

func (m providersMenuModel) updateInstanceDetail(msg tea.Msg) (providersMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
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
			if m.detailCursor == 0 {
				// Remove instance
				m.confirm = components.NewConfirm(
					fmt.Sprintf("Remove instance %q? This will also remove all its installations.", m.selectedInstance.Name),
				)
				m.currentView = providersViewConfirmRemove
			}
		case "esc", "q":
			m.currentView = providersViewList
			m.status = ""
		}
	}
	return m, nil
}

// --- Confirm remove ---

func (m providersMenuModel) updateConfirmRemove(msg tea.Msg) (providersMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case components.ConfirmResultMsg:
		if msg.Confirmed {
			inst := m.selectedInstance
			// Remove all installations for this instance
			installs, _ := models.ListInstallationsByInstance(m.db, inst.ID)
			for _, installation := range installs {
				_ = linker.RemoveSymlink(installation.SymlinkPath)
				_ = models.DeleteInstallation(m.db, installation.ID)
			}
			// Delete instance
			if err := models.DeleteInstance(m.db, inst.ID); err != nil {
				m.status = fmt.Sprintf("Error: %v", err)
			} else {
				m.status = fmt.Sprintf("Removed instance %q", inst.Name)
			}
			m.loadData()
			m.cursor = 0
			// Ensure cursor lands on a selectable row
			for i := range m.rows {
				if m.selectableRow(i) {
					m.cursor = i
					break
				}
			}
		} else {
			m.status = ""
		}
		m.currentView = providersViewList
		return m, nil
	}

	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)
	return m, cmd
}

// --- Add instance view ---

func (m *providersMenuModel) prepareAddInstance(provider *models.Provider) {
	m.addProvider = provider
	m.addStep = 0
	m.nameInput.SetValue("")
	m.nameInput.Focus()
	m.pathInput.SetValue("")

	// Suggest a path based on provider prefix
	home, err := os.UserHomeDir()
	if err == nil && provider.ConfigDirPrefix != "" {
		prefix := provider.ConfigDirPrefix
		if !strings.HasPrefix(prefix, ".") {
			prefix = "." + prefix
		}
		m.suggestedPath = filepath.Join(home, prefix, "skills")
	} else {
		m.suggestedPath = ""
	}
}

func (m providersMenuModel) updateAddInstance(msg tea.Msg) (providersMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.addStep == 0 {
				// Validate name
				name := strings.TrimSpace(m.nameInput.Value())
				if name == "" {
					m.status = "Name cannot be empty"
					return m, nil
				}
				m.addStep = 1
				m.nameInput.Blur()
				if m.suggestedPath != "" {
					m.pathInput.SetValue(m.suggestedPath)
				}
				m.pathInput.Focus()
				m.status = ""
				return m, m.pathInput.Cursor.BlinkCmd()
			}
			// Step 1: validate path and create
			rawPath := strings.TrimSpace(m.pathInput.Value())
			if rawPath == "" {
				m.status = "Path cannot be empty"
				return m, nil
			}
			absPath := expandPath(rawPath)
			name := strings.TrimSpace(m.nameInput.Value())

			_, err := models.CreateInstance(m.db, m.addProvider.ID, name, absPath, false)
			if err != nil {
				// Check for UNIQUE constraint
				if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
					m.status = "Path already in use by another instance"
				} else {
					m.status = fmt.Sprintf("Error: %v", err)
				}
				return m, nil
			}
			m.status = fmt.Sprintf("Added instance %q", name)
			m.loadData()
			m.cursor = 0
			for i := range m.rows {
				if m.selectableRow(i) {
					m.cursor = i
					break
				}
			}
			m.currentView = providersViewList
			return m, nil

		case "esc":
			if m.addStep == 1 {
				m.addStep = 0
				m.pathInput.Blur()
				m.nameInput.Focus()
				m.status = ""
				return m, m.nameInput.Cursor.BlinkCmd()
			}
			m.currentView = providersViewList
			m.status = ""
			return m, nil
		}
	}

	// Delegate to active text input
	var cmd tea.Cmd
	if m.addStep == 0 {
		m.nameInput, cmd = m.nameInput.Update(msg)
	} else {
		m.pathInput, cmd = m.pathInput.Update(msg)
	}
	return m, cmd
}

// --- Scan home view ---

func (m *providersMenuModel) runHomeScan() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// Build a set of already-registered global_skills_path values
	allInst, _ := models.ListAllInstances(m.db)
	registered := make(map[string]bool, len(allInst))
	for _, inst := range allInst {
		registered[inst.GlobalSkillsPath] = true
	}

	var discovered []discoveredInstance
	for _, node := range m.nodes {
		p := node.provider
		if p.ConfigDirPrefix == "" {
			continue
		}
		// ConfigDirPrefix already includes the dot (e.g. ".claude")
		prefix := p.ConfigDirPrefix
		if !strings.HasPrefix(prefix, ".") {
			prefix = "." + prefix
		}
		// If the prefix already contains a glob wildcard, use it as-is;
		// otherwise append * to match variants (e.g. ".claude" → ".claude*")
		pattern := filepath.Join(home, prefix)
		if !strings.Contains(prefix, "*") {
			pattern = filepath.Join(home, prefix+"*")
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			// The skills path is <match>/skills/
			skillsPath := filepath.Join(match, "skills")
			// Skip if already registered
			if registered[skillsPath] || registered[match] {
				continue
			}
			name := filepath.Base(match)
			discovered = append(discovered, discoveredInstance{
				provider: p,
				name:     name,
				path:     skillsPath,
			})
		}
	}
	m.discovered = discovered
}

func (m providersMenuModel) updateScanHome(msg tea.Msg) (providersMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.scanCursor > 0 {
				m.scanCursor--
			}
		case "down", "j":
			if m.scanCursor < len(m.discovered)-1 {
				m.scanCursor++
			}
		case " ":
			m.scanSel[m.scanCursor] = !m.scanSel[m.scanCursor]
		case "enter":
			// Add selected discovered instances
			added := 0
			for i, d := range m.discovered {
				if !m.scanSel[i] {
					continue
				}
				_, err := models.CreateInstance(m.db, d.provider.ID, d.name, d.path, false)
				if err == nil {
					added++
				}
			}
			m.status = fmt.Sprintf("Added %d instance(s)", added)
			m.loadData()
			m.cursor = 0
			for i := range m.rows {
				if m.selectableRow(i) {
					m.cursor = i
					break
				}
			}
			m.currentView = providersViewList
		case "esc", "q":
			// Go to manual add instead
			m.prepareAddInstance(m.addProvider)
			m.currentView = providersViewAddInstance
			return m, m.nameInput.Cursor.BlinkCmd()
		}
	}
	return m, nil
}

// --- Views ---

func (m providersMenuModel) view() string {
	switch m.currentView {
	case providersViewInstanceDetail:
		return m.viewInstanceDetail()
	case providersViewAddInstance:
		return m.viewAddInstance()
	case providersViewScanHome:
		return m.viewScanHome()
	case providersViewConfirmRemove:
		return m.viewConfirmRemove()
	default:
		return m.viewList()
	}
}

func (m providersMenuModel) viewList() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Providers"))
	sb.WriteString("\n\n")

	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error loading providers: %v", m.err)))
		sb.WriteString("\n")
	} else if len(m.rows) == 0 {
		sb.WriteString(subtleStyle.Render("No providers found."))
		sb.WriteString("\n")
	} else {
		for i, row := range m.rows {
			switch row.kind {
			case "provider":
				// Non-selectable header
				sb.WriteString(normalStyle.Render("  " + row.provider.Name))
				sb.WriteString("\n")

			case "instance":
				inst := row.instance
				label := fmt.Sprintf("%s  %s", inst.Name, subtleStyle.Render(inst.GlobalSkillsPath))
				defaultTag := ""
				if inst.IsDefault {
					defaultTag = successStyle.Render(" [default]")
				}
				if i == m.cursor {
					sb.WriteString(selectedStyle.Render("    > "+inst.Name) + "  " + subtleStyle.Render(inst.GlobalSkillsPath) + defaultTag)
				} else {
					sb.WriteString(normalStyle.Render("      "+label) + defaultTag)
				}
				sb.WriteString("\n")

			case "add":
				label := "[Add instance]"
				if i == m.cursor {
					sb.WriteString(selectedStyle.Render("    > " + label))
				} else {
					sb.WriteString(subtleStyle.Render("      " + label))
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

func (m providersMenuModel) viewInstanceDetail() string {
	var sb strings.Builder
	inst := m.selectedInstance

	sb.WriteString(titleStyle.Render("Instance: " + inst.Name))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  Provider:    %s\n", m.selectedProvider.Name))
	sb.WriteString(fmt.Sprintf("  Skills path: %s\n", inst.GlobalSkillsPath))
	if inst.IsDefault {
		sb.WriteString(successStyle.Render("  Default instance"))
		sb.WriteString("\n")
	}

	sb.WriteString("\n  Installed skills:\n")
	if len(m.instSkills) == 0 {
		sb.WriteString(subtleStyle.Render("    (none)"))
		sb.WriteString("\n")
	} else {
		for _, row := range m.instSkills {
			statusIcon := "  "
			st := linker.CheckSymlink(row.installation.SymlinkPath, "")
			switch st {
			case linker.StatusActive:
				statusIcon = successStyle.Render("●")
			case linker.StatusBroken:
				statusIcon = errorStyle.Render("●")
			case linker.StatusOrphaned:
				statusIcon = warningStyle.Render("○")
			}
			sb.WriteString(fmt.Sprintf("    %s %s\n", statusIcon, row.skillName))
		}
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
	sb.WriteString(subtleStyle.Render("↑/↓ navigate • enter select • esc back"))

	return sb.String()
}

func (m providersMenuModel) viewAddInstance() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Add Instance"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  Provider: %s\n\n", m.addProvider.Name))

	if m.addStep == 0 {
		sb.WriteString("  Instance name:\n\n")
		sb.WriteString("  " + m.nameInput.View())
	} else {
		sb.WriteString(fmt.Sprintf("  Name: %s\n\n", m.nameInput.Value()))
		sb.WriteString("  Global skills path:\n\n")
		sb.WriteString("  " + m.pathInput.View())
	}

	sb.WriteString("\n\n")
	if m.status != "" {
		sb.WriteString(errorStyle.Render("  " + m.status))
		sb.WriteString("\n\n")
	}
	sb.WriteString(subtleStyle.Render("  enter confirm • esc back"))

	return sb.String()
}

func (m providersMenuModel) viewScanHome() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Discovered Instances"))
	sb.WriteString("\n\n")

	if len(m.discovered) == 0 {
		sb.WriteString(subtleStyle.Render("  No instances discovered in home directory."))
		sb.WriteString("\n")
	} else {
		sb.WriteString("  Select instances to add:\n\n")
		for i, d := range m.discovered {
			check := "[ ]"
			if m.scanSel[i] {
				check = "[x]"
			}
			line := fmt.Sprintf("%s %s (%s)  %s", check, d.name, d.provider.Name, subtleStyle.Render(d.path))
			if i == m.scanCursor {
				sb.WriteString(selectedStyle.Render("  > " + line))
			} else {
				sb.WriteString(normalStyle.Render("    " + line))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  space toggle • enter confirm • esc manual add"))

	return sb.String()
}

func (m providersMenuModel) viewConfirmRemove() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Remove Instance"))
	sb.WriteString("\n")
	sb.WriteString(m.confirm.View())
	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  ←/→ switch • y/n shortcut • enter confirm"))
	return sb.String()
}
