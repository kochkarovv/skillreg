package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/git"
	"github.com/vladyslav/skillreg/internal/models"
)

// menuItem represents a single entry in the main menu.
type menuItem struct {
	label string
	count int
	view  view
}

// fetchResult holds the outcome of a background git fetch for one source.
type fetchResult struct {
	sourceID   int64
	sourceName string
	behind     int
	err        error
	pulled     bool
}

// fetchResultsMsg carries all fetch results back to the Update loop.
type fetchResultsMsg struct {
	results []fetchResult
}

// mainMenuModel is the BubbleTea model for the main menu screen.
type mainMenuModel struct {
	db           *db.Database
	cursor       int
	items        []menuItem
	fetchResults []fetchResult
	fetching     bool
}

// newMainMenu creates and populates the main menu model.
func newMainMenu(d *db.Database) mainMenuModel {
	m := mainMenuModel{
		db:      d,
		cursor:  0,
		fetching: true,
	}
	m.items = m.buildItems()
	return m
}

// buildItems queries the DB for current counts and returns the menu rows.
func (m mainMenuModel) buildItems() []menuItem {
	skillCount := 0
	if skills, err := models.ListAllSkills(m.db); err == nil {
		skillCount = len(skills)
	}

	sourceCount := 0
	if sources, err := models.ListSources(m.db); err == nil {
		sourceCount = len(sources)
	}

	providerCount := 0
	if providers, err := models.ListProviders(m.db); err == nil {
		providerCount = len(providers)
	}

	return []menuItem{
		{label: "Skills", count: skillCount, view: viewSkills},
		{label: "Sources", count: sourceCount, view: viewSources},
		{label: "Providers", count: providerCount, view: viewProviders},
	}
}

// Init dispatches background goroutines to git-fetch all sources.
func (m mainMenuModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchAllSources(),
	)
}

// fetchAllSources spawns a goroutine per source and collects results.
func (m mainMenuModel) fetchAllSources() tea.Cmd {
	return func() tea.Msg {
		sources, err := models.ListSources(m.db)
		if err != nil || len(sources) == 0 {
			return fetchResultsMsg{}
		}

		results := make([]fetchResult, 0, len(sources))
		for _, src := range sources {
			r := fetchResult{sourceID: src.ID, sourceName: src.Name}

			// Fetch from remote (ignore errors for repos with no remote)
			_ = git.Fetch(src.Path)

			// Count how far behind we are
			behind, err := git.CommitsBehind(src.Path)
			if err != nil {
				r.err = err
				results = append(results, r)
				continue
			}
			r.behind = behind

			// Auto-pull if configured and behind
			if src.AutoUpdate && behind > 0 {
				if pullErr := git.PullFF(src.Path); pullErr == nil {
					r.pulled = true
					r.behind = 0
					_ = models.UpdateSourceLastChecked(m.db, src.ID)
				}
			}

			results = append(results, r)
		}
		return fetchResultsMsg{results: results}
	}
}

// update handles keyboard navigation and incoming messages.
func (m mainMenuModel) update(msg tea.Msg) (mainMenuModel, tea.Cmd) {
	switch msg := msg.(type) {

	case fetchResultsMsg:
		m.fetchResults = msg.results
		m.fetching = false
		// Refresh counts after potential auto-pull
		m.items = m.buildItems()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter", " ":
			if m.cursor < len(m.items) {
				return m, navigate(m.items[m.cursor].view)
			}
		}
	}
	return m, nil
}

// view renders the main menu.
func (m mainMenuModel) view() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("SkillRegistry"))
	sb.WriteString("\n\n")

	// Update banner
	if m.fetching {
		sb.WriteString(subtleStyle.Render("Checking for updates..."))
		sb.WriteString("\n\n")
	} else {
		// Show any sources that are behind
		for _, r := range m.fetchResults {
			if r.err != nil {
				sb.WriteString(errorStyle.Render(fmt.Sprintf("  Error checking %s: %v", r.sourceName, r.err)))
				sb.WriteString("\n")
			} else if r.pulled {
				sb.WriteString(successStyle.Render(fmt.Sprintf("  %s auto-updated successfully", r.sourceName)))
				sb.WriteString("\n")
			} else if r.behind > 0 {
				sb.WriteString(warningStyle.Render(fmt.Sprintf("  %s is %d commit(s) behind origin", r.sourceName, r.behind)))
				sb.WriteString("\n")
			}
		}
		if len(m.fetchResults) > 0 {
			sb.WriteString("\n")
		}
	}

	// Menu items
	for i, item := range m.items {
		line := fmt.Sprintf("  %s (%d)", item.label, item.count)
		if i == m.cursor {
			sb.WriteString(selectedStyle.Render("> " + line[2:]))
		} else {
			sb.WriteString(normalStyle.Render(line))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(subtleStyle.Render("↑/↓ navigate • enter select • q quit"))

	return sb.String()
}
