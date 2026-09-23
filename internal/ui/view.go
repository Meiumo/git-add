package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/meiumo/gitadd/internal/gitlab"
)

const (
	minPathWidth = 22
	maxPathWidth = 48
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.mode == modePick {
		return m.viewPicker()
	}

	var b strings.Builder
	b.WriteString(m.viewHeader())
	b.WriteString("\n")
	b.WriteString(m.viewUsers())
	b.WriteString("\n")
	b.WriteString(m.viewTargets())
	if m.applied {
		b.WriteString("\n")
		b.WriteString(m.viewSummary())
	}
	b.WriteString("\n")
	b.WriteString(m.viewStatus())
	b.WriteString("\n")
	b.WriteString(m.viewHelp())
	return b.String()
}

func (m Model) pathWidth() int {
	w := m.width - 46
	if w < minPathWidth {
		w = minPathWidth
	}
	if w > maxPathWidth {
		w = maxPathWidth
	}
	return w
}

func (m Model) viewHeader() string {
	parts := []string{
		titleStyle.Render("git-add"),
		hostStyle.Render(m.client.Config().Host()),
		faintStyle.Render(fmt.Sprintf("%d grant(s)", m.plan.PairCount())),
	}
	if m.dryRun {
		parts = append(parts, badgeStyle.Render("DRY RUN"))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, spaced(parts)...)
}

func spaced(parts []string) []string {
	out := make([]string, 0, len(parts)*2)
	for i, p := range parts {
		if i > 0 {
			out = append(out, "  ")
		}
		out = append(out, p)
	}
	return out
}

func (m Model) viewUsers() string {
	pathW := m.pathWidth()
	var rows []string
	rows = append(rows, headerRowStyle.Render(pad("user", pathW)+"  status"))

	for i, u := range m.plan.Users {
		selected := m.section == sectionUsers && i == m.userIdx
		cursor := "  "
		if selected {
			cursor = cursorStyle.Render("> ")
		}

		label := u.Raw
		if label == "" {
			label = faintStyle.Render("<empty>")
			label = padRendered(label, "<empty>", pathW)
		} else {
			label = pad(ellipsize(label, pathW), pathW)
		}

		if m.mode == modeEdit && selected {
			line := cursor + editStyle.Render(m.input.View())
			rows = append(rows, line)
			continue
		}

		detail := u.Detail()
		style := faintStyle
		if u.Resolved() {
			style = okStyle
			detail = fmt.Sprintf("%s (id %d)", u.User.Name, u.User.ID)
		} else if u.Status != "" && u.Status != "empty" {
			style = badStyle
		}

		line := cursor + label + "  " + style.Render(truncate(detail, m.width-pathW-8))
		if selected {
			line = selectedRowStyle.Render(line)
		}
		rows = append(rows, line)
	}

	content := strings.Join(rows, "\n")
	style := panelStyle
	if m.section == sectionUsers {
		style = activePanelStyle
	}
	return sectionStyle.Render("users") + "\n" + style.Width(m.panelWidth()).Render(content)
}

func (m Model) viewTargets() string {
	pathW := m.pathWidth()
	var rows []string

	var head strings.Builder
	head.WriteString(pad("target", pathW))
	head.WriteString(" ")
	for _, r := range gitlab.Roles {
		head.WriteString(" " + r.Key + " ")
	}
	head.WriteString("  status")
	rows = append(rows, headerRowStyle.Render(head.String()))

	for i, t := range m.plan.Targets {
		selected := m.section == sectionTargets && i == m.targetIdx
		cursor := "  "
		if selected {
			cursor = cursorStyle.Render("> ")
		}

		if m.mode == modeEdit && selected {
			rows = append(rows, cursor+editStyle.Render(m.input.View()))
			continue
		}

		label := t.Raw
		if label == "" {
			label = padRendered(faintStyle.Render("<empty>"), "<empty>", pathW)
		} else {
			label = pad(ellipsize(label, pathW), pathW)
		}

		var boxes strings.Builder
		boxes.WriteString(" ")
		for ci, r := range gitlab.Roles {
			on := t.Role.Key == r.Key
			glyph := " "
			if on {
				glyph = r.Key
			}
			box := "[" + glyph + "]"
			switch {
			case on:
				boxes.WriteString(roleOnStyle.Render(box))
			case selected && ci == m.roleCol:
				boxes.WriteString(roleCurStyle.Render(box))
			default:
				boxes.WriteString(roleOffStyle.Render(box))
			}
		}

		status := t.Status
		if t.Kind() != "" {
			status = t.Kind() + "  " + status
		}
		line := cursor + label + boxes.String() + "  " + m.statusStyle(t.Status).Render(truncate(status, 28))
		if selected {
			line = selectedRowStyle.Render(line)
		}
		rows = append(rows, line)
	}

	content := strings.Join(rows, "\n")
	style := panelStyle
	if m.section == sectionTargets {
		style = activePanelStyle
	}
	return sectionStyle.Render("targets") + "\n" + style.Width(m.panelWidth()).Render(content)
}

// panelWidth is the inner width of a bordered panel. Lipgloss counts the
// border and padding on top of Width(), so the budget is the terminal minus
// two border columns and two padding columns.
func (m Model) panelWidth() int {
	w := m.width - 6
	if w < 48 {
		w = 48
	}
	if w > 118 {
		w = 118
	}
	return w
}

func (m Model) statusStyle(status string) lipgloss.Style {
	low := strings.ToLower(status)
	switch {
	case strings.HasPrefix(status, "FAIL"), strings.HasPrefix(status, "SKIP"),
		strings.Contains(low, "not found"), strings.Contains(low, "no user"):
		return badStyle
	case status == "ok", strings.HasPrefix(status, "added"), strings.HasPrefix(status, "updated"):
		return okStyle
	case strings.Contains(low, "ambiguous"), strings.HasPrefix(status, "already"),
		strings.HasPrefix(status, "DRY"):
		return warnStyle
	}
	return faintStyle
}

func (m Model) viewSummary() string {
	failed := m.plan.Failures()
	total := len(m.plan.Outcomes)
	if total == 0 {
		return ""
	}
	// Columns are sized against the panel so a long result never wraps onto
	// a second line, which would break the one-row-per-grant reading.
	userW, targetW := 16, m.pathWidth()
	resultW := m.panelWidth() - userW - targetW - 4
	if resultW < 16 {
		resultW = 16
		if over := userW + targetW + resultW + 4 - m.panelWidth(); over > 0 {
			if targetW -= over; targetW < 12 {
				targetW = 12
			}
		}
	}

	var rows []string
	for _, o := range m.plan.Outcomes {
		style := okStyle
		if o.Failed() {
			style = badStyle
		}
		rows = append(rows, fmt.Sprintf("  %s %s %s",
			pad(ellipsize(o.User, userW), userW),
			pad(ellipsize(o.Target, targetW), targetW),
			style.Render(truncate(o.Result, resultW))))
	}
	head := okStyle.Render(fmt.Sprintf("%d/%d ok", total-failed, total))
	if failed > 0 {
		head = warnStyle.Render(fmt.Sprintf("%d/%d ok, %d failed", total-failed, total, failed))
	}
	return sectionStyle.Render("result") + "\n" + panelStyle.Width(m.panelWidth()).
		Render(head+"\n"+strings.Join(rows, "\n"))
}

func (m Model) viewStatus() string {
	if m.busy {
		return warnStyle.Render("  " + m.status)
	}
	if m.status == "" {
		return ""
	}
	return faintStyle.Render("  " + m.status)
}

func (m Model) viewHelp() string {
	if m.mode == modeEdit {
		return helpStyle.Render("  enter confirm   esc cancel")
	}
	lines := []string{
		"enter edit   a add   D delete   tab section   arrows move",
		"g r d m o role   R resolve   ctrl+a apply   ctrl+d dry-run   q quit",
	}
	return helpStyle.Render("  " + strings.Join(lines, "\n  "))
}

func (m Model) viewPicker() string {
	row := m.plan.Targets[m.pickRow]
	var rows []string
	for i, c := range row.Target.Candidates {
		cursor := "  "
		line := fmt.Sprintf("%-8s %s", c.Kind, c.FullPath)
		if i == m.pickIdx {
			cursor = cursorStyle.Render("> ")
			line = selectedRowStyle.Render(line)
		}
		rows = append(rows, cursor+line)
	}
	body := fmt.Sprintf("%s\n\n%s", warnStyle.Render(row.Raw+": several matches"), strings.Join(rows, "\n"))
	return "\n" + activePanelStyle.Width(m.panelWidth()).Render(body) +
		"\n" + helpStyle.Render("  enter pick   esc cancel")
}

// ---------------------------------------------------------------- text

func pad(s string, width int) string {
	n := lipgloss.Width(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// padRendered pads a styled string using the width of its plain source, since
// escape sequences would otherwise be counted as visible columns.
func padRendered(styled, plain string, width int) string {
	if len(plain) >= width {
		return styled
	}
	return styled + strings.Repeat(" ", width-len([]rune(plain)))
}

func ellipsize(s string, width int) string {
	r := []rune(s)
	if width <= 0 || len(r) <= width {
		return s
	}
	if width <= 3 {
		return string(r[:width])
	}
	return "..." + string(r[len(r)-(width-3):])
}

func truncate(s string, width int) string {
	r := []rune(s)
	if width <= 0 || len(r) <= width {
		return s
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "…"
}
