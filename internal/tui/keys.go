package tui

import "github.com/charmbracelet/bubbles/key"

type globalKeyMap struct {
	Quit key.Binding
	Back key.Binding
	Help key.Binding
}

var globalKeys = globalKeyMap{
	Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Back: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}
