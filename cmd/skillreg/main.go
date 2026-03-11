package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/config"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/tui"
)

func main() {
	// Open (or create) the database.
	database, err := db.Open(config.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Seed built-in providers (idempotent).
	if err := database.SeedProviders(); err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: failed to seed providers: %v\n", err)
		os.Exit(1)
	}

	// Build and run the TUI.
	app := tui.NewApp(database)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: %v\n", err)
		os.Exit(1)
	}
}
