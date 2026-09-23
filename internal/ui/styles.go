package ui

import "github.com/charmbracelet/lipgloss"

// Adaptive colours keep the form readable on both light and dark terminals.
var (
	accent   = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#A78BFA"}
	subtle   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	faint    = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#4B5563"}
	okColor  = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"}
	badColor = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	warn     = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	surface  = lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#312E81"}
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accent).
			Padding(0, 1)

	badgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(warn).
			Padding(0, 1)

	hostStyle = lipgloss.NewStyle().Foreground(subtle)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			MarginTop(1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(faint).
			Padding(0, 1)

	activePanelStyle = panelStyle.
				BorderForeground(accent)

	headerRowStyle = lipgloss.NewStyle().Foreground(subtle)

	cursorStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)

	selectedRowStyle = lipgloss.NewStyle().Background(surface)

	okStyle    = lipgloss.NewStyle().Foreground(okColor)
	badStyle   = lipgloss.NewStyle().Foreground(badColor)
	warnStyle  = lipgloss.NewStyle().Foreground(warn)
	faintStyle = lipgloss.NewStyle().Foreground(faint)

	roleOnStyle  = lipgloss.NewStyle().Foreground(okColor).Bold(true)
	roleOffStyle = lipgloss.NewStyle().Foreground(faint)
	roleCurStyle = lipgloss.NewStyle().Foreground(accent).Bold(true).Underline(true)

	helpStyle = lipgloss.NewStyle().Foreground(faint).MarginTop(1)

	editStyle = lipgloss.NewStyle().Foreground(accent)
)
