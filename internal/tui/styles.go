package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 2)
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CC00"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)
	normalStyle   = lipgloss.NewStyle()
	boxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#7D56F4")).Padding(1, 2)
)
