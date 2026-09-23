package ui

import "github.com/charmbracelet/lipgloss"

// Border set: heavy rule under the title bar, rounded panels, so the eye
// reads the header as a separate plane from the editable lists.
var (
	titleBar = lipgloss.NewStyle().
			Bold(true).
			Foreground(inverse).
			Background(violet).
			Padding(0, 2)

	titleChip = lipgloss.NewStyle().
			Foreground(textBright).
			Background(violetDeep).
			Padding(0, 1)

	dryChip = lipgloss.NewStyle().
		Bold(true).
		Foreground(inverse).
		Background(amber).
		Padding(0, 1)

	busyChip = lipgloss.NewStyle().
			Foreground(inverse).
			Background(violetDim).
			Padding(0, 1)

	hostText = lipgloss.NewStyle().Foreground(textDim)

	// Panels. The active section gets a brighter border and a title tag
	// rendered into the top rule, which is cheaper on vertical space than
	// a separate heading line.
	panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lineColor).
		Padding(0, 1)

	panelActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(violet).
			Padding(0, 1)

	panelTag = lipgloss.NewStyle().
			Bold(true).
			Foreground(violet)

	panelTagIdle = lipgloss.NewStyle().Foreground(textFaint)

	columnHead = lipgloss.NewStyle().
			Foreground(textFaint).
			Underline(true)

	cursorMark = lipgloss.NewStyle().Foreground(violet).Bold(true)

	rowSelected = lipgloss.NewStyle().Background(violetDeep).Foreground(textBright)

	// Semantic text.
	okText    = lipgloss.NewStyle().Foreground(teal)
	okFaint   = lipgloss.NewStyle().Foreground(tealDim)
	warnText  = lipgloss.NewStyle().Foreground(amber)
	warnFaint = lipgloss.NewStyle().Foreground(amberDim)
	badText   = lipgloss.NewStyle().Foreground(rose)
	badFaint  = lipgloss.NewStyle().Foreground(roseDim)
	dimText   = lipgloss.NewStyle().Foreground(textFaint)
	bodyText  = lipgloss.NewStyle().Foreground(textBright)

	// Role checkboxes.
	roleOn      = lipgloss.NewStyle().Foreground(teal).Bold(true)
	roleOff     = lipgloss.NewStyle().Foreground(textFaint)
	roleFocused = lipgloss.NewStyle().Foreground(amber).Bold(true)

	kindTag = lipgloss.NewStyle().Foreground(violetDim)

	editPrompt = lipgloss.NewStyle().Foreground(amber).Bold(true)

	helpKey  = lipgloss.NewStyle().Foreground(textDim).Bold(true)
	helpText = lipgloss.NewStyle().Foreground(textFaint)

	statusLine = lipgloss.NewStyle().Foreground(textDim)
)

// Glyphs. Box-drawing and geometric shapes only, no emoji and nothing outside
// the ranges a Nerd Font is not required for.
const (
	glyphCursor   = "▸"
	glyphOK       = "●"
	glyphPending  = "○"
	glyphFail     = "✕"
	glyphWarn     = "▲"
	glyphArrow    = "→"
	glyphSpinner0 = "⠋"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
