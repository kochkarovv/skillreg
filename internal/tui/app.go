package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/updater"
)

// view enumerates the possible screen states.
type view int

const (
	viewMain      view = iota
	viewSkills
	viewSources
	viewProviders
	viewTools
	viewUpdate
)

// navigateMsg is sent to switch the active view.
type navigateMsg struct {
	target view
}

// navigate returns a command that sends a navigateMsg.
func navigate(target view) tea.Cmd {
	return func() tea.Msg {
		return navigateMsg{target: target}
	}
}

// updateAvailableMsg is sent when a newer version is found.
type updateAvailableMsg struct {
	release *updater.Release
}

// updateDoneMsg is sent when the update download/apply finishes.
type updateDoneMsg struct {
	version string
	err     error
}

// App is the root BubbleTea model. It owns a stack of sub-menus and
// delegates Init / Update / View to whichever sub-menu is currently active.
type App struct {
	db        *db.Database
	current   view
	main      mainMenuModel
	skills    skillsMenuModel
	sources   sourcesMenuModel
	providers providersMenuModel
	tools     toolsMenuModel
	width     int
	height    int

	// Update
	version         string
	availableUpdate *updater.Release
	updateStatus    string
	previousView    view
}

// NewApp constructs an App with the given database connection.
func NewApp(d *db.Database, version string) App {
	return App{
		db:        d,
		current:   viewMain,
		main:      newMainMenu(d),
		skills:    newSkillsMenu(d),
		sources:   newSourcesMenu(d),
		providers: newProvidersMenu(d),
		tools:     newToolsMenu(d),
		version:   version,
	}
}

// Init kicks off the main-menu initialisation and background update check.
func (a App) Init() tea.Cmd {
	return tea.Batch(a.main.Init(), a.checkForUpdate())
}

func (a App) checkForUpdate() tea.Cmd {
	return func() tea.Msg {
		rel, _ := updater.CheckLatest(a.version)
		if rel != nil {
			return updateAvailableMsg{release: rel}
		}
		return nil
	}
}

func (a App) applyUpdate() tea.Cmd {
	rel := a.availableUpdate
	return func() tea.Msg {
		err := updater.Apply(rel, "")
		return updateDoneMsg{version: rel.TagName, err: err}
	}
}

// Update handles global messages and delegates the rest to the active sub-menu.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.skills.height = msg.Height
		return a, nil

	case updateAvailableMsg:
		a.availableUpdate = msg.release
		return a, nil

	case updateDoneMsg:
		if msg.err != nil {
			a.updateStatus = fmt.Sprintf("Update failed: %v", msg.err)
		} else {
			a.updateStatus = fmt.Sprintf("Updated to %s! Restart skillreg to use the new version.", msg.version)
		}
		return a, nil

	case navigateMsg:
		a.current = msg.target
		// Refresh data when entering any sub-menu
		switch msg.target {
		case viewSkills:
			a.skills.loadData()
		case viewSources:
			a.sources.loadSources()
		case viewProviders:
			a.providers.needsAutoScan = true
			a.providers.loadData()
		case viewTools:
			a.tools.loadData()
		case viewMain:
			a.main.items = a.main.buildItems()
		}
		return a, nil

	case tea.KeyMsg:
		// Handle esc/q from update view
		if a.current == viewUpdate && (msg.String() == "esc" || msg.String() == "q") {
			a.current = a.previousView
			return a, nil
		}
		// [u] triggers update from anywhere (except update view itself)
		if msg.String() == "u" && a.availableUpdate != nil && a.current != viewUpdate {
			a.previousView = a.current
			a.current = viewUpdate
			a.updateStatus = fmt.Sprintf("Downloading %s...", a.availableUpdate.TagName)
			return a, a.applyUpdate()
		}
		// ctrl+c always quits from anywhere
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		// q quits from the main menu only; in sub-menus it's delegated
		if msg.String() == "q" && a.current == viewMain {
			return a, tea.Quit
		}
		// esc from main menu does nothing; in sub-menus it's delegated
		// (sub-menus return navigate(viewMain) when at their root view)
	}

	// Delegate to the active sub-menu.
	var cmd tea.Cmd
	switch a.current {
	case viewMain:
		var m mainMenuModel
		m, cmd = a.main.update(msg)
		a.main = m
	case viewSkills:
		var m skillsMenuModel
		m, cmd = a.skills.update(msg)
		a.skills = m
	case viewSources:
		var m sourcesMenuModel
		m, cmd = a.sources.update(msg)
		a.sources = m
	case viewProviders:
		var m providersMenuModel
		m, cmd = a.providers.update(msg)
		a.providers = m
	case viewTools:
		var m toolsMenuModel
		m, cmd = a.tools.update(msg)
		a.tools = m
	}
	return a, cmd
}

// View delegates rendering to the active sub-menu.
func (a App) View() string {
	banner := a.updateBanner()
	var content string
	switch a.current {
	case viewUpdate:
		return a.viewUpdate()
	case viewSkills:
		content = a.skills.view()
	case viewSources:
		content = a.sources.view()
	case viewProviders:
		content = a.providers.view()
	case viewTools:
		content = a.tools.view()
	default:
		content = a.main.view()
	}
	return banner + content
}

func (a App) updateBanner() string {
	if a.availableUpdate == nil {
		return ""
	}
	return updateBannerStyle.Render(
		fmt.Sprintf("Update available: %s → %s — press [u] to update",
			a.version, a.availableUpdate.TagName)) + "\n"
}

func (a App) viewUpdate() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Update"))
	sb.WriteString("\n\n")
	sb.WriteString("  " + a.updateStatus)
	sb.WriteString("\n\n")
	sb.WriteString(subtleStyle.Render("esc back"))
	return sb.String()
}
