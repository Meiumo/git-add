package ui

import "github.com/charmbracelet/lipgloss"

// The palette is a deliberate xterm-256 ramp rather than ad-hoc hex values.
// Lipgloss downsamples truecolor to the 256 cube on terminals that need it;
// choosing from the cube up front makes that step a no-op instead of a guess,
// so the form looks the same in Terminal.app, Ghostty and a bare tmux.
//
// Hue policy, applied consistently across every widget:
//
//	violet   structure and focus (borders, titles, cursor)
//	teal     resolved, applied, safe
//	amber    pending, ambiguous, dry-run
//	rose     failures and skips
//	slate    labels, help, anything the eye should skip
//
// Each entry carries a light-background twin; terminals report their
// background and lipgloss picks the right side automatically.
func c(dark, light string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Dark: dark, Light: light}
}

var (
	violet     = c("141", "98") // primary accent
	violetDim  = c("104", "61") // inactive border
	violetDeep = c("61", "146") // selected row wash

	teal    = c("79", "29")
	tealDim = c("72", "22")

	amber    = c("221", "136")
	amberDim = c("179", "130")

	rose    = c("210", "160")
	roseDim = c("174", "124")

	textBright = c("253", "235")
	textDim    = c("247", "240")
	textFaint  = c("240", "247")

	lineColor = c("238", "251")
	inverse   = c("232", "231")
)
