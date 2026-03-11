package components

import "github.com/charmbracelet/lipgloss"

// StatusBar renders a styled bottom bar with the given text, spanning the given width.
func StatusBar(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#333333")).
		Width(width).
		Padding(0, 1)
	return style.Render(text)
}
