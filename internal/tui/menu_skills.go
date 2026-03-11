package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/models"
)

// skillsMenuModel is the stub BubbleTea model for the skills screen.
type skillsMenuModel struct {
	db     *db.Database
	skills []*models.Skill
	cursor int
	err    error
}

// newSkillsMenu constructs a skillsMenuModel and loads skills from the DB.
func newSkillsMenu(d *db.Database) skillsMenuModel {
	m := skillsMenuModel{db: d}
	skills, err := models.ListAllSkills(d)
	if err != nil {
		m.err = err
	} else {
		m.skills = skills
	}
	return m
}

// update handles keyboard navigation within the skills list.
func (m skillsMenuModel) update(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.skills)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

// view renders the skills list.
func (m skillsMenuModel) view() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Skills"))
	sb.WriteString("\n\n")

	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error loading skills: %v", m.err)))
		sb.WriteString("\n")
	} else if len(m.skills) == 0 {
		sb.WriteString(subtleStyle.Render("No skills found."))
		sb.WriteString("\n")
	} else {
		for i, sk := range m.skills {
			line := fmt.Sprintf("  %s", sk.Name)
			if sk.Description != "" {
				line += " — " + sk.Description
			}
			if i == m.cursor {
				sb.WriteString(selectedStyle.Render("> " + line[2:]))
			} else {
				sb.WriteString(normalStyle.Render(line))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("↑/↓ navigate • esc back"))

	return sb.String()
}
