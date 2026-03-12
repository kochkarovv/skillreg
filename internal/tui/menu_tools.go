package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/linker"
	"github.com/vladyslav/skillreg/internal/models"
	"github.com/vladyslav/skillreg/internal/tui/components"
)

// toolsView enumerates the sub-views within the tools menu.
type toolsView int

const (
	toolsViewList    toolsView = iota
	toolsViewCleanup                   // cleanup results with multi-select
	toolsViewConfirm                   // confirm deletion
)

// toolItem is a selectable tool in the tools list.
type toolItem struct {
	label       string
	description string
}

// staleEntry represents a non-symlink directory found in an instance's skills directory.
type staleEntry struct {
	instanceName string
	path         string // absolute path
	name         string // directory name
}

// staleGroup groups stale entries by instance for tree view rendering.
type staleGroup struct {
	instanceName string
	entries      []int // indices into staleEntries
}

// toolsMenuModel is the BubbleTea model for the tools screen.
type toolsMenuModel struct {
	db     *db.Database
	cursor int
	status string
	tools  []toolItem

	currentView toolsView

	// Cleanup
	staleEntries  []staleEntry
	staleGroups   []staleGroup   // groups for tree view
	staleFlatRows []int          // flat row index → -1 for header, or entry index
	staleFlatKind []string       // "header" or "entry"
	staleSel      map[int]bool   // keyed by entry index
	staleCursor   int

	// Confirm
	confirm components.ConfirmModel
}

func newToolsMenu(d *db.Database) toolsMenuModel {
	return toolsMenuModel{
		db: d,
		tools: []toolItem{
			{label: "Cleanup", description: "Find and remove non-symlink skills from instance directories"},
		},
		staleSel: make(map[int]bool),
	}
}

func (m *toolsMenuModel) loadData() {
	// Nothing to preload for tools list
}

func (m toolsMenuModel) update(msg tea.Msg) (toolsMenuModel, tea.Cmd) {
	switch m.currentView {
	case toolsViewList:
		return m.updateList(msg)
	case toolsViewCleanup:
		return m.updateCleanup(msg)
	case toolsViewConfirm:
		return m.updateConfirm(msg)
	}
	return m, nil
}

// --- List view ---

func (m toolsMenuModel) updateList(msg tea.Msg) (toolsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.tools)-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor == 0 {
				// Cleanup tool
				m.runCleanupScan()
				m.staleCursor = 0
				m.staleSel = make(map[int]bool)
				// Pre-select all
				for i := range m.staleEntries {
					m.staleSel[i] = true
				}
				m.currentView = toolsViewCleanup
			}
		case "esc", "q":
			return m, navigate(viewMain)
		}
	}
	return m, nil
}

// runCleanupScan scans all instance skill directories for non-symlink directories.
func (m *toolsMenuModel) runCleanupScan() {
	instances, _ := models.ListAllInstances(m.db)

	var entries []staleEntry
	var groups []staleGroup

	for _, inst := range instances {
		dir := inst.GlobalSkillsPath
		dirEntries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var groupIndices []int
		for _, e := range dirEntries {
			if !e.IsDir() {
				continue
			}
			fullPath := filepath.Join(dir, e.Name())
			// Skip symlinks — those are managed by us
			if linker.IsSymlink(fullPath) {
				continue
			}
			idx := len(entries)
			entries = append(entries, staleEntry{
				instanceName: inst.Name,
				path:         fullPath,
				name:         e.Name(),
			})
			groupIndices = append(groupIndices, idx)
		}
		if len(groupIndices) > 0 {
			groups = append(groups, staleGroup{
				instanceName: inst.Name,
				entries:      groupIndices,
			})
		}
	}
	m.staleEntries = entries
	m.staleGroups = groups
	m.buildFlatRows()
}

// buildFlatRows creates a flat navigation list from the tree structure.
// Row 0 is always "Select all", then groups with headers and entries.
func (m *toolsMenuModel) buildFlatRows() {
	m.staleFlatRows = nil
	m.staleFlatKind = nil

	// Row 0: Select all
	m.staleFlatRows = append(m.staleFlatRows, -1)
	m.staleFlatKind = append(m.staleFlatKind, "selectall")

	for _, g := range m.staleGroups {
		// Header row (not selectable for toggle, but navigable)
		m.staleFlatRows = append(m.staleFlatRows, -1)
		m.staleFlatKind = append(m.staleFlatKind, "header")
		// Entry rows
		for _, idx := range g.entries {
			m.staleFlatRows = append(m.staleFlatRows, idx)
			m.staleFlatKind = append(m.staleFlatKind, "entry")
		}
	}
}

// --- Cleanup view ---

func (m toolsMenuModel) selectableRow(i int) bool {
	if i < 0 || i >= len(m.staleFlatKind) {
		return false
	}
	return m.staleFlatKind[i] != "header"
}

func (m toolsMenuModel) updateCleanup(msg tea.Msg) (toolsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			for i := m.staleCursor - 1; i >= 0; i-- {
				if m.selectableRow(i) {
					m.staleCursor = i
					break
				}
			}
		case "down", "j":
			for i := m.staleCursor + 1; i < len(m.staleFlatRows); i++ {
				if m.selectableRow(i) {
					m.staleCursor = i
					break
				}
			}
		case " ":
			if m.staleCursor < len(m.staleFlatKind) {
				kind := m.staleFlatKind[m.staleCursor]
				if kind == "selectall" {
					allSelected := true
					for i := range m.staleEntries {
						if !m.staleSel[i] {
							allSelected = false
							break
						}
					}
					for i := range m.staleEntries {
						m.staleSel[i] = !allSelected
					}
				} else if kind == "entry" {
					idx := m.staleFlatRows[m.staleCursor]
					m.staleSel[idx] = !m.staleSel[idx]
				}
			}
		case "enter":
			count := 0
			for _, sel := range m.staleSel {
				if sel {
					count++
				}
			}
			if count == 0 {
				m.status = "Nothing selected"
				return m, nil
			}
			m.confirm = components.NewConfirm(
				fmt.Sprintf("Delete %d item(s)? This cannot be undone.", count),
			)
			m.currentView = toolsViewConfirm
		case "esc", "q":
			m.currentView = toolsViewList
			m.status = ""
		}
	}
	return m, nil
}

// --- Confirm view ---

func (m toolsMenuModel) updateConfirm(msg tea.Msg) (toolsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case components.ConfirmResultMsg:
		if msg.Confirmed {
			deleted := 0
			errors := 0
			for i, entry := range m.staleEntries {
				if !m.staleSel[i] {
					continue
				}
				if err := os.RemoveAll(entry.path); err != nil {
					errors++
				} else {
					deleted++
				}
			}
			if errors > 0 {
				m.status = fmt.Sprintf("Deleted %d, failed %d", deleted, errors)
			} else {
				m.status = fmt.Sprintf("Deleted %d item(s)", deleted)
			}
			// Re-scan to show remaining
			m.runCleanupScan()
			m.staleSel = make(map[int]bool)
			m.staleCursor = 0
			if len(m.staleEntries) == 0 {
				m.currentView = toolsViewCleanup
			} else {
				m.currentView = toolsViewCleanup
			}
		} else {
			m.status = ""
			m.currentView = toolsViewCleanup
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)
	return m, cmd
}

// --- Views ---

func (m toolsMenuModel) view() string {
	switch m.currentView {
	case toolsViewCleanup:
		return m.viewCleanup()
	case toolsViewConfirm:
		return m.viewConfirm()
	default:
		return m.viewList()
	}
}

func (m toolsMenuModel) viewList() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Tools"))
	sb.WriteString("\n\n")

	for i, tool := range m.tools {
		if i == m.cursor {
			sb.WriteString(selectedStyle.Render("  > " + tool.label))
			sb.WriteString(subtleStyle.Render(" — " + tool.description))
		} else {
			sb.WriteString(normalStyle.Render("    " + tool.label))
			sb.WriteString(subtleStyle.Render(" — " + tool.description))
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

func (m toolsMenuModel) viewCleanup() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Cleanup"))
	sb.WriteString("\n\n")

	if len(m.staleEntries) == 0 {
		sb.WriteString(successStyle.Render("  All clean! No non-symlink directories found in skill directories."))
		sb.WriteString("\n")
	} else {
		sb.WriteString(fmt.Sprintf("  Found %d non-symlink director(ies) in skill directories:\n\n", len(m.staleEntries)))

		groupIdx := 0
		for i, kind := range m.staleFlatKind {
			switch kind {
			case "selectall":
				label := "Select all"
				if i == m.staleCursor {
					sb.WriteString(selectedStyle.Render("  > " + label))
				} else {
					sb.WriteString(subtleStyle.Render("    " + label))
				}
				sb.WriteString("\n\n")

			case "header":
				if groupIdx < len(m.staleGroups) {
					sb.WriteString(normalStyle.Render("  " + m.staleGroups[groupIdx].instanceName))
					sb.WriteString("\n")
					groupIdx++
				}

			case "entry":
				idx := m.staleFlatRows[i]
				entry := m.staleEntries[idx]
				check := "[ ]"
				if m.staleSel[idx] {
					check = "[x]"
				}
				label := fmt.Sprintf("%s %s", check, entry.name)
				if i == m.staleCursor {
					sb.WriteString(selectedStyle.Render("    > " + label))
				} else {
					sb.WriteString(normalStyle.Render("      " + label))
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
	if len(m.staleEntries) > 0 {
		sb.WriteString(subtleStyle.Render("  space toggle • enter delete selected • esc back"))
	} else {
		sb.WriteString(subtleStyle.Render("  esc back"))
	}

	return sb.String()
}

func (m toolsMenuModel) viewConfirm() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Confirm Cleanup"))
	sb.WriteString("\n")
	sb.WriteString(m.confirm.View())
	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("  ←/→ switch • y/n shortcut • enter confirm"))
	return sb.String()
}
