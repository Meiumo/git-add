package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/meiumo/gitadd/internal/gitlab"
)

// ErrAborted is returned when the operator declines to choose.
type ErrAborted struct{ Target string }

func (e ErrAborted) Error() string { return "no choice made for " + e.Target }

// ResolveAmbiguous asks the operator to pick between matches, one prompt per
// ambiguous row. This is the non-interactive path's answer to what the form
// does with its picker: a bare name that matches several places is a question,
// not a dead end.
//
// Returns the number of rows left unresolved.
func ResolveAmbiguous(in io.Reader, out io.Writer, plan *gitlab.Plan) int {
	rows := plan.AmbiguousRows()
	if len(rows) == 0 {
		return 0
	}

	reader := bufio.NewReader(in)
	unresolved := 0

	for _, row := range rows {
		cands := row.Target.Candidates
		fmt.Fprintf(out, "\n  %s %s %s\n",
			warnText.Render(glyphWarn),
			bodyText.Render(row.Raw),
			dimText.Render(fmt.Sprintf("matches %d places", len(cands))))

		printCandidates(out, cands)
		fmt.Fprintf(out, "      %s\n", dimText.Render(
			"narrow it next time with group:name or project:name"))

		choice := promptChoice(reader, out, len(cands))
		if choice < 0 {
			unresolved++
			fmt.Fprintf(out, "  %s %s\n", dimText.Render(glyphPending),
				dimText.Render("skipped "+row.Raw))
			continue
		}
		row.Choose(cands[choice])
		fmt.Fprintf(out, "  %s %s %s\n", okText.Render(glyphOK),
			bodyText.Render(row.Raw),
			dimText.Render("("+cands[choice].Kind+")"))
	}
	return unresolved
}

func printCandidates(out io.Writer, cands []gitlab.Candidate) {
	width := 0
	for _, c := range cands {
		if n := len(c.FullPath); n > width {
			width = n
		}
	}
	if width > 54 {
		width = 54
	}

	for i, c := range cands {
		style := kindTag
		if c.Kind == "group" {
			style = warnText
		}
		scope := c.Scope()
		if scope != "" {
			scope = dimText.Render("  " + scope)
		}
		fmt.Fprintf(out, "   %s %s %s%s\n",
			dimText.Render(fmt.Sprintf("%2d)", i+1)),
			style.Render(pad(c.Kind, 8)),
			pad(ellipsize(c.FullPath, width), width),
			scope)
	}
}

// promptChoice reads a 1-based index, or blank/q to skip. Returns -1 to skip.
func promptChoice(reader *bufio.Reader, out io.Writer, n int) int {
	for {
		fmt.Fprintf(out, "  %s ", helpKey.Render(fmt.Sprintf("pick 1-%d, or enter to skip:", n)))
		line, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			return -1
		}
		answer := strings.TrimSpace(line)
		if answer == "" || strings.EqualFold(answer, "q") || strings.EqualFold(answer, "s") {
			return -1
		}
		idx, convErr := strconv.Atoi(answer)
		if convErr != nil || idx < 1 || idx > n {
			fmt.Fprintf(out, "  %s\n", badFaint.Render(fmt.Sprintf("enter a number between 1 and %d", n)))
			continue
		}
		return idx - 1
	}
}

// CanPrompt reports whether an interactive choice is possible.
func CanPrompt() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
