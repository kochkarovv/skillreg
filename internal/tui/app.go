package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
)

// view enumerates the possible screen states.
type view int

const (
	viewMain      view = iota
	viewSkills
	viewSources
	viewProviders
	viewTools
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

// App is the root BubbleTea model. It owns a stack of sub-menus and
// delegates Init / Update / View to whichever sub-menu is currently active.
type App struct {
	db       *db.Database
	current  view
	main     mainMenuModel
	skills   skillsMenuModel
	sources  sourcesMenuModel
	providers providersMenuModel
	tools    toolsMenuModel
	width    int
	height   int
}

// NewApp constructs an App with the given database connection.
func NewApp(d *db.Database) App {
	return App{
		db:        d,
		current:   viewMain,
		main:      newMainMenu(d),
		skills:    newSkillsMenu(d),
		sources:   newSourcesMenu(d),
		providers: newProvidersMenu(d),
		tools:    newToolsMenu(d),
	}
}

// Init kicks off the main-menu initialisation (which triggers background fetches).
func (a App) Init() tea.Cmd {
	return a.main.Init()
}

// Update handles global messages and delegates the rest to the active sub-menu.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
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
	switch a.current {
	case viewSkills:
		return a.skills.view()
	case viewSources:
		return a.sources.view()
	case viewProviders:
		return a.providers.view()
	case viewTools:
		return a.tools.view()
	default:
		return a.main.view()
	}
}
