package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/models"
)

// providerNode groups a provider with its instances for display purposes.
type providerNode struct {
	provider  *models.Provider
	instances []*models.Instance
}

// providersMenuModel is the stub BubbleTea model for the providers screen.
type providersMenuModel struct {
	db     *db.Database
	nodes  []providerNode
	cursor int
	err    error
}

// newProvidersMenu constructs a providersMenuModel and loads data from the DB.
func newProvidersMenu(d *db.Database) providersMenuModel {
	m := providersMenuModel{db: d}

	providers, err := models.ListProviders(d)
	if err != nil {
		m.err = err
		return m
	}

	nodes := make([]providerNode, 0, len(providers))
	for _, p := range providers {
		instances, _ := models.ListInstancesByProvider(d, p.ID)
		nodes = append(nodes, providerNode{provider: p, instances: instances})
	}
	m.nodes = nodes
	return m
}

// update handles keyboard navigation within the providers list.
func (m providersMenuModel) update(msg tea.Msg) (providersMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.nodes)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

// view renders the providers list with nested instances.
func (m providersMenuModel) view() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Providers"))
	sb.WriteString("\n\n")

	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error loading providers: %v", m.err)))
		sb.WriteString("\n")
	} else if len(m.nodes) == 0 {
		sb.WriteString(subtleStyle.Render("No providers found."))
		sb.WriteString("\n")
	} else {
		for i, node := range m.nodes {
			name := node.provider.Name
			if i == m.cursor {
				sb.WriteString(selectedStyle.Render("> " + name))
			} else {
				sb.WriteString(normalStyle.Render("  " + name))
			}
			sb.WriteString("\n")

			// Nested instances
			for _, inst := range node.instances {
				instLine := fmt.Sprintf("      • %s  %s", inst.Name, subtleStyle.Render(inst.GlobalSkillsPath))
				sb.WriteString(instLine)
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("↑/↓ navigate • esc back"))

	return sb.String()
}
