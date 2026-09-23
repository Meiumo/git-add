package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"

	"github.com/meiumo/gitadd/internal/gitlab"
)

// Report prints applied outcomes after the form exits, so the result survives
// the alternate screen buffer being torn down.
func Report(w io.Writer, outcomes []gitlab.Outcome, dryRun bool) {
	if len(outcomes) == 0 {
		fmt.Fprintln(w, dimText.Render("nothing applied"))
		return
	}

	userW, targetW := 0, 0
	for _, o := range outcomes {
		if n := lipgloss.Width(o.User); n > userW {
			userW = n
		}
		if n := lipgloss.Width(o.Target); n > targetW {
			targetW = n
		}
	}

	failed := 0
	for _, o := range outcomes {
		glyph, style := outcomeStyle(o)
		if o.Failed() {
			failed++
		}
		fmt.Fprintf(w, "  %s %s %s %s  %s\n",
			glyph,
			pad(o.User, userW),
			dimText.Render(glyphArrow),
			pad(o.Target, targetW),
			style.Render(o.Result))
	}

	total := len(outcomes)
	line := okText.Render(fmt.Sprintf("%s %d of %d applied", glyphOK, total-failed, total))
	if failed > 0 {
		line = warnText.Render(fmt.Sprintf("%s %d of %d applied, %d failed",
			glyphWarn, total-failed, total, failed))
	}
	if dryRun {
		line += dimText.Render("  (dry run, nothing written)")
	}
	fmt.Fprintf(w, "\n  %s\n", line)
}

// PlanReport prints resolution results for the non-interactive path, using the
// same glyph and colour language as the form.
func PlanReport(w io.Writer, plan *gitlab.Plan) int {
	problems := 0

	for _, u := range plan.Users {
		if u.Resolved() {
			fmt.Fprintf(w, "  %s %s %s\n",
				okText.Render(glyphOK),
				bodyText.Render(pad(u.Label(), 20)),
				dimText.Render(fmt.Sprintf("%s (id %d)", u.User.Name, u.User.ID)))
			continue
		}
		problems++
		fmt.Fprintf(w, "  %s %s %s\n",
			badText.Render(glyphFail),
			bodyText.Render(pad(u.Raw, 20)),
			badFaint.Render(u.Status))
	}

	for _, t := range plan.Targets {
		if t.Resolved() {
			fmt.Fprintf(w, "  %s %s %s %s\n",
				okText.Render(glyphOK),
				bodyText.Render(pad(t.Label(), 34)),
				kindTag.Render(pad(t.Kind(), 8)),
				warnText.Render(t.Role.Name))
			continue
		}
		problems++
		fmt.Fprintf(w, "  %s %s %s\n",
			badText.Render(glyphFail),
			bodyText.Render(pad(t.Raw, 34)),
			badFaint.Render(t.Status))

		if t.Target != nil {
			for i, c := range t.Target.Candidates {
				if i == 10 {
					fmt.Fprintf(w, "      %s\n",
						dimText.Render(fmt.Sprintf("... %d more", len(t.Target.Candidates)-10)))
					break
				}
				fmt.Fprintf(w, "      %s %s\n",
					kindTag.Render(pad(c.Kind, 8)),
					dimText.Render(c.FullPath))
			}
		}
	}
	return problems
}

// Banner prints the instance being touched, keeping the CLI visually anchored
// to the form.
func Banner(w io.Writer, host string, dryRun bool) {
	chips := titleBar.Render("git-add") + hostText.Render(" "+host)
	if dryRun {
		chips += "  " + dryChip.Render("DRY RUN")
	}
	fmt.Fprintf(w, "\n  %s\n\n", chips)
}
