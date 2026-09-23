package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/meiumo/gitadd/internal/gitlab"
)

// Report prints the applied outcomes after the form exits, so the result
// survives the alternate screen buffer being torn down.
func Report(w io.Writer, outcomes []gitlab.Outcome, dryRun bool) {
	if len(outcomes) == 0 {
		fmt.Fprintln(w, "nothing applied")
		return
	}

	userW := 0
	targetW := 0
	for _, o := range outcomes {
		if n := len(o.User); n > userW {
			userW = n
		}
		if n := len(o.Target); n > targetW {
			targetW = n
		}
	}

	failed := 0
	for _, o := range outcomes {
		style := okStyle
		if o.Failed() {
			style = badStyle
			failed++
		}
		fmt.Fprintf(w, "  %s  %s  %s\n",
			pad(o.User, userW),
			pad(o.Target, targetW),
			style.Render(o.Result))
	}

	total := len(outcomes)
	summary := fmt.Sprintf("%d/%d ok", total-failed, total)
	if dryRun {
		summary += " (dry run)"
	}
	style := okStyle
	if failed > 0 {
		style = warnStyle
	}
	fmt.Fprintf(w, "\n%s\n", style.Render(summary))
}

// PlanReport prints resolution results for the non-interactive path.
func PlanReport(w io.Writer, plan *gitlab.Plan) int {
	problems := 0
	for _, u := range plan.Users {
		if u.Resolved() {
			fmt.Fprintf(w, "user   %s  %s\n", boldish(u.Label()),
				faintStyle.Render(fmt.Sprintf("%s (id %d)", u.User.Name, u.User.ID)))
			continue
		}
		problems++
		fmt.Fprintf(w, "user   %s  %s\n", boldish(u.Raw), badStyle.Render(u.Status))
	}

	for _, t := range plan.Targets {
		if t.Resolved() {
			fmt.Fprintf(w, "target %-7s %s  %s\n", t.Kind(), t.Label(),
				warnStyle.Render(t.Role.Name))
			continue
		}
		problems++
		fmt.Fprintf(w, "target %s  %s\n", boldish(t.Raw), badStyle.Render(t.Status))
		if t.Target != nil {
			for i, c := range t.Target.Candidates {
				if i == 10 {
					break
				}
				fmt.Fprintf(w, "         %-8s %s\n", c.Kind, c.FullPath)
			}
		}
	}
	return problems
}

func boldish(s string) string {
	return strings.TrimSpace(s)
}
