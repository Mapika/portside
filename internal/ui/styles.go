package ui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	dirStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cursorStyle    = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("231"))
	statusStyle    = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("250"))
	statusErrStyle = lipgloss.NewStyle().Background(lipgloss.Color("52")).Foreground(lipgloss.Color("231"))
	changedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)
