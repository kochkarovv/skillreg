package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/models"
)

// sourcesMenuModel is the stub BubbleTea model for the sources screen.
// The last row in the list is a virtual "[Add source]" entry.
type sourcesMenuModel struct {
	db      *db.Database
	sources []*models.Source
	cursor  int
	err     error
}

// newSourcesMenu constructs a sourcesMenuModel and loads sources from the DB.
func newSourcesMenu(d *db.Database) sourcesMenuModel {
	m := sourcesMenuModel{db: d}
	sources, err := models.ListSources(d)
	if err != nil {
		m.err = err
	} else {
		m.sources = sources
	}
	return m
}

// totalRows returns len(sources) + 1 for the [Add source] row.
func (m sourcesMenuModel) totalRows() int {
	return len(m.sources) + 1
}

// update handles keyboard navigation within the sources list.
func (m sourcesMenuModel) update(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
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
		}
	}
	return m, nil
}

// view renders the sources list with an [Add source] option at the bottom.
func (m sourcesMenuModel) view() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Sources"))
	sb.WriteString("\n\n")

	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error loading sources: %v", m.err)))
		sb.WriteString("\n")
	} else {
		for i, src := range m.sources {
			line := fmt.Sprintf("  %s  %s", src.Name, subtleStyle.Render(src.Path))
			if i == m.cursor {
				sb.WriteString(selectedStyle.Render("> " + src.Name + "  "))
				sb.WriteString(subtleStyle.Render(src.Path))
			} else {
				sb.WriteString(normalStyle.Render(line))
			}
			sb.WriteString("\n")
		}

		// [Add source] virtual row — index = len(sources)
		addIdx := len(m.sources)
		addLabel := "[Add source]"
		if m.cursor == addIdx {
			sb.WriteString(selectedStyle.Render("> " + addLabel))
		} else {
			sb.WriteString(subtleStyle.Render("  " + addLabel))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("↑/↓ navigate • enter select • esc back"))

	return sb.String()
}
