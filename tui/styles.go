package tui

import "github.com/charmbracelet/lipgloss"

var (
	purple      = lipgloss.Color("#7C3AED")
	lightPurple = lipgloss.Color("#A78BFA")
	green       = lipgloss.Color("#10B981")
	red         = lipgloss.Color("#EF4444")
	yellow      = lipgloss.Color("#F59E0B")
	gray        = lipgloss.Color("#6B7280")
	white       = lipgloss.Color("#F9FAFB")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(purple).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lightPurple).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(white).
			MarginRight(1)

	btnStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(purple).
			Padding(0, 3).
			Bold(true)

	btnDimStyle = lipgloss.NewStyle().
			Foreground(gray).
			Background(lipgloss.Color("#374151")).
			Padding(0, 3).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(yellow)

	dimStyle = lipgloss.NewStyle().
			Foreground(gray)

	statusDotConnected = lipgloss.NewStyle().
				Foreground(green).
				SetString("●")

	statusDotDisconnected = lipgloss.NewStyle().
				Foreground(red).
				SetString("●")

	statusDotConnecting = lipgloss.NewStyle().
				Foreground(yellow).
				SetString("●")

	logLineStyle = lipgloss.NewStyle().
			Foreground(gray)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(purple).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(white)

	appStyle = lipgloss.NewStyle().
			Padding(1, 2)

	infoStyle = lipgloss.NewStyle().
			Foreground(lightPurple).
			Italic(true)

	separatorStyle = lipgloss.NewStyle().
			Foreground(gray).
			SetString("─").
			Width(1).
			Italic(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(purple)
)
