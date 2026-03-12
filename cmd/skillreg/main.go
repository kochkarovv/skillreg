package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/config"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/tui"
	"github.com/vladyslav/skillreg/internal/updater"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("skillreg %s\n", Version)
			os.Exit(0)
		case "update":
			runUpdate()
			return
		}
	}

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
	app := tui.NewApp(database, Version)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: %v\n", err)
		os.Exit(1)
	}
}

func runUpdate() {
	fmt.Printf("Checking for updates (current: %s)...\n", Version)
	rel, err := updater.CheckLatest(Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}
	if rel == nil {
		fmt.Printf("Already up to date (%s)\n", Version)
		return
	}
	fmt.Printf("Downloading %s...\n", rel.TagName)
	if err := updater.Apply(rel, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Updated to %s! Restart skillreg to use the new version.\n", rel.TagName)
}
