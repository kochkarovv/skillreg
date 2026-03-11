package components

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmResultMsg is sent when the user makes a choice in the confirm dialog.
type ConfirmResultMsg struct {
	Confirmed bool
}

// ConfirmModel is a simple yes/no confirmation dialog.
type ConfirmModel struct {
	Question string
	Focused  int // 0 = yes, 1 = no
}

// NewConfirm creates a new confirmation dialog with the given question.
func NewConfirm(question string) ConfirmModel {
	return ConfirmModel{
		Question: question,
		Focused:  1, // default to "no" for safety
	}
}

// Update handles key input for the confirm dialog.
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			m.Focused = 0
		case "right", "l":
			m.Focused = 1
		case "y":
			return m, confirmResult(true)
		case "n":
			return m, confirmResult(false)
		case "enter":
			return m, confirmResult(m.Focused == 0)
		}
	}
	return m, nil
}

// View renders the confirm dialog.
func (m ConfirmModel) View() string {
	yesStyle := lipgloss.NewStyle().Padding(0, 2)
	noStyle := lipgloss.NewStyle().Padding(0, 2)

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 2)

	yes := yesStyle.Render("Yes")
	no := noStyle.Render("No")
	if m.Focused == 0 {
		yes = activeStyle.Render("Yes")
	} else {
		no = activeStyle.Render("No")
	}

	return fmt.Sprintf("\n  %s\n\n  %s  %s\n", m.Question, yes, no)
}

func confirmResult(confirmed bool) tea.Cmd {
	return func() tea.Msg {
		return ConfirmResultMsg{Confirmed: confirmed}
	}
}
