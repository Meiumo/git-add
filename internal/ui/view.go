package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/meiumo/gitadd/internal/gitlab"
)

const (
	minPathWidth = 20
	maxPathWidth = 52
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.mode == modePick {
		return m.viewPicker()
	}

	blocks := []string{
		m.viewHeader(),
		m.viewPanel(sectionUsers),
		m.viewPanel(sectionTargets),
	}
	if m.applied && len(m.plan.Outcomes) > 0 {
		blocks = append(blocks, m.viewResult())
	}
	blocks = append(blocks, m.viewStatus(), m.viewHelp())

	// Drop empty blocks so an absent status line does not leave a gap that
	// shifts the help text as rows resolve.
	kept := blocks[:0]
	for _, b := range blocks {
		if strings.TrimSpace(b) != "" {
			kept = append(kept, b)
		}
	}

	return "\n" + strings.Join(kept, "\n") + "\n"
}

// ---------------------------------------------------------------- header

func (m Model) viewHeader() string {
	left := titleBar.Render("git-add")
	host := hostText.Render(" " + m.client.Config().Host())

	chips := []string{titleChip.Render(fmt.Sprintf("%d grant(s)", m.plan.PairCount()))}
	if m.dryRun {
		chips = append(chips, dryChip.Render("DRY RUN"))
	}
	if m.busy {
		chips = append(chips, busyChip.Render(m.spinnerFrame()+" working"))
	}

	head := left + host
	tail := strings.Join(chips, " ")

	gap := m.frameWidth() - lipgloss.Width(head) - lipgloss.Width(tail)
	if gap < 1 {
		gap = 1
	}
	return indent + head + strings.Repeat(" ", gap) + tail
}

func (m Model) spinnerFrame() string {
	if len(spinnerFrames) == 0 {
		return glyphSpinner0
	}
	return spinnerFrames[m.tick%len(spinnerFrames)]
}

// ---------------------------------------------------------------- panels

const indent = "  "

func (m Model) frameWidth() int {
	w := m.width - 4
	if w < 56 {
		w = 56
	}
	if w > 120 {
		w = 120
	}
	return w
}

// panelWidth is the content width inside the border and padding.
//
// lipgloss Width() sets the content box, and the 1-column padding on each
// side is drawn inside it, so a rule spanning the full content must be
// panelWidth-2 or it wraps onto a second line.
func (m Model) panelWidth() int {
	return m.frameWidth() - 4
}

// ruleWidth is the widest a horizontal rule can be without wrapping.
func (m Model) ruleWidth() int {
	w := m.panelWidth() - 2
	if w < 8 {
		w = 8
	}
	return w
}

func (m Model) pathWidth() int {
	w := m.panelWidth() - 40
	if w < minPathWidth {
		w = minPathWidth
	}
	if w > maxPathWidth {
		w = maxPathWidth
	}
	return w
}

// viewPanel renders one section with its tag written into the top border,
// which reads like a labelled fieldset and costs no extra line.
func (m Model) viewPanel(s section) string {
	active := m.section == s

	var title string
	var body string
	if s == sectionUsers {
		title = fmt.Sprintf("users (%d)", countFilled(len(m.plan.Users), func(i int) bool {
			return m.plan.Users[i].Raw != ""
		}))
		body = m.viewUserRows()
	} else {
		title = fmt.Sprintf("targets (%d)", countFilled(len(m.plan.Targets), func(i int) bool {
			return m.plan.Targets[i].Raw != ""
		}))
		body = m.viewTargetRows()
	}

	style := panel
	tag := panelTagIdle.Render(title)
	if active {
		style = panelActive
		tag = panelTag.Render(title)
	}

	rendered := style.Width(m.panelWidth()).Render(body)
	return indentBlock(injectTitle(rendered, tag, active))
}

func countFilled(n int, filled func(int) bool) int {
	c := 0
	for i := 0; i < n; i++ {
		if filled(i) {
			c++
		}
	}
	return c
}

// injectTitle splices a tag into the top border of an already-rendered panel.
func injectTitle(rendered, tag string, active bool) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}
	top := lines[0]
	plainTag := lipgloss.NewStyle().Render(stripANSI(tag))
	tagW := lipgloss.Width(plainTag)

	runes := []rune(stripANSI(top))
	if len(runes) < tagW+6 {
		return rendered
	}

	borderStyle := lipgloss.NewStyle().Foreground(lineColor)
	if active {
		borderStyle = lipgloss.NewStyle().Foreground(violet)
	}

	head := borderStyle.Render(string(runes[0:2]))
	tail := borderStyle.Render(string(runes[2+tagW+2:]))
	lines[0] = head + " " + tag + " " + tail
	return strings.Join(lines, "\n")
}

func indentBlock(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------- rows

// rowPrefix is the cursor mark plus the status glyph and its space, so the
// column headers line up with the data rows instead of drifting left by two.
const rowPrefix = 4

func (m Model) viewUserRows() string {
	pathW := m.pathWidth()
	// The detail column takes whatever the path column leaves, so a long
	// display name is not clipped to a stub like "Ivan Ivanovich (id…".
	detailW := m.panelWidth() - rowPrefix - pathW - 2
	if detailW < 12 {
		detailW = 12
	}
	rows := []string{
		columnHead.Render(pad("", rowPrefix) + pad("user", pathW) + pad("status", detailW)),
	}

	for i, u := range m.plan.Users {
		selected := m.section == sectionUsers && i == m.userIdx

		if m.mode == modeEdit && selected {
			rows = append(rows, cursorMark.Render(glyphCursor)+" "+editPrompt.Render(m.input.View()))
			continue
		}

		mark := "  "
		if selected {
			mark = cursorMark.Render(glyphCursor) + " "
		}

		glyph, detail, style := userStatus(u)
		line := mark + glyph + " " + pad(ellipsize(labelOf(u.Raw), pathW), pathW) +
			style.Render(truncate(detail, detailW))

		if selected {
			line = rowSelected.Render(padPlain(line, m.panelWidth()))
		}
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n")
}

func userStatus(u *gitlab.UserRow) (string, string, lipgloss.Style) {
	switch {
	case u.Resolved():
		return okText.Render(glyphOK),
			fmt.Sprintf("%s (id %d)", u.User.Name, u.User.ID),
			okFaint
	case u.Raw == "":
		return dimText.Render(glyphPending), "", dimText
	case u.Status == "" || u.Status == "empty":
		return dimText.Render(glyphPending), "unresolved", dimText
	default:
		return badText.Render(glyphFail), u.Status, badFaint
	}
}

func (m Model) viewTargetRows() string {
	pathW := m.pathWidth()

	var head strings.Builder
	head.WriteString(pad("", rowPrefix))
	head.WriteString(pad("target", pathW))
	for _, r := range gitlab.Roles {
		head.WriteString(" " + r.Key + " ")
	}
	head.WriteString("  " + pad("kind", 8) + "status")
	rows := []string{columnHead.Render(head.String())}

	for i, t := range m.plan.Targets {
		selected := m.section == sectionTargets && i == m.targetIdx

		if m.mode == modeEdit && selected {
			rows = append(rows, cursorMark.Render(glyphCursor)+" "+editPrompt.Render(m.input.View()))
			continue
		}

		mark := "  "
		if selected {
			mark = cursorMark.Render(glyphCursor) + " "
		}

		glyph, status, style := targetStatus(t)

		var boxes strings.Builder
		for ci, r := range gitlab.Roles {
			on := t.Role.Key == r.Key
			inner := " "
			if on {
				inner = r.Key
			}
			box := "[" + inner + "]"
			switch {
			case on && selected && ci == m.roleCol:
				boxes.WriteString(roleFocused.Render("[" + strings.ToUpper(r.Key) + "]"))
			case on:
				boxes.WriteString(roleOn.Render(box))
			case selected && ci == m.roleCol:
				boxes.WriteString(roleFocused.Render("[·]"))
			default:
				boxes.WriteString(roleOff.Render(box))
			}
		}

		kind := ""
		if t.Kind() != "" {
			kind = kindTag.Render(pad(t.Kind(), 8))
		} else {
			kind = pad("", 8)
		}

		line := mark + glyph + " " + pad(ellipsize(labelOf(t.Raw), pathW), pathW) +
			boxes.String() + "  " + kind + style.Render(truncate(status, 22))

		if selected {
			line = rowSelected.Render(padPlain(line, m.panelWidth()))
		}
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n")
}

func targetStatus(t *gitlab.TargetRow) (string, string, lipgloss.Style) {
	low := strings.ToLower(t.Status)
	switch {
	case t.Resolved():
		return okText.Render(glyphOK), "ok", okFaint
	case t.Raw == "":
		return dimText.Render(glyphPending), "", dimText
	case strings.Contains(low, "ambiguous"):
		return warnText.Render(glyphWarn), t.Status, warnFaint
	case t.Status == "":
		return dimText.Render(glyphPending), "unresolved", dimText
	default:
		return badText.Render(glyphFail), t.Status, badFaint
	}
}

func labelOf(raw string) string {
	if raw == "" {
		return "<empty>"
	}
	return raw
}

// ---------------------------------------------------------------- result

func (m Model) viewResult() string {
	failed := m.plan.Failures()
	total := len(m.plan.Outcomes)

	userW := 18
	targetW := m.pathWidth()
	resultW := m.panelWidth() - userW - targetW - 8
	if resultW < 18 {
		resultW = 18
		if over := userW + targetW + resultW + 8 - m.panelWidth(); over > 0 {
			targetW -= over
			if targetW < 12 {
				targetW = 12
			}
		}
	}

	var rows []string
	for _, o := range m.plan.Outcomes {
		glyph, style := outcomeStyle(o)
		rows = append(rows, fmt.Sprintf("%s %s %s %s",
			glyph,
			pad(ellipsize(o.User, userW), userW),
			dimText.Render(glyphArrow)+" "+pad(ellipsize(o.Target, targetW), targetW),
			style.Render(truncate(o.Result, resultW))))
	}

	summary := okText.Render(fmt.Sprintf("%s %d of %d applied", glyphOK, total-failed, total))
	if failed > 0 {
		summary = warnText.Render(fmt.Sprintf("%s %d of %d applied, %d failed",
			glyphWarn, total-failed, total, failed))
	}
	if m.dryRun {
		summary += dimText.Render("  (dry run, nothing written)")
	}

	body := summary + "\n" + dimText.Render(strings.Repeat("─", m.ruleWidth())) + "\n" +
		strings.Join(rows, "\n")

	rendered := panel.BorderForeground(lineColor).Width(m.panelWidth()).Render(body)
	return indentBlock(injectTitle(rendered, panelTagIdle.Render("result"), false))
}

func outcomeStyle(o gitlab.Outcome) (string, lipgloss.Style) {
	switch {
	case o.Failed():
		return badText.Render(glyphFail), badText
	case strings.HasPrefix(o.Result, "DRY"):
		return warnText.Render(glyphPending), warnFaint
	case strings.HasPrefix(o.Result, "already"):
		return dimText.Render(glyphOK), dimText
	default:
		return okText.Render(glyphOK), okText
	}
}

// ---------------------------------------------------------------- chrome

func (m Model) viewStatus() string {
	if m.status == "" {
		// No blank placeholder line: an empty status should not push the
		// help text down and make the layout jitter.
		return ""
	}
	prefix := dimText.Render(glyphPending)
	style := statusLine
	switch {
	case m.busy:
		prefix = warnText.Render(m.spinnerFrame())
	case looksLikeProblem(m.status):
		prefix = badText.Render(glyphFail)
		style = badFaint
	}
	return indent + prefix + " " + style.Render(m.status)
}

func looksLikeProblem(s string) bool {
	low := strings.ToLower(s)
	for _, needle := range []string{"not found", "no user", "failed", "forbidden", "unauthorised", "ambiguous"} {
		if strings.Contains(low, needle) {
			return true
		}
	}
	return false
}

type helpEntry struct{ key, action string }

func (m Model) viewHelp() string {
	if m.mode == modeEdit {
		return renderHelp([]helpEntry{
			{"enter", "confirm"}, {"esc", "cancel"},
			{"ctrl+u", "clear"},
		}, m.frameWidth())
	}
	entries := []helpEntry{
		{"enter", "edit"}, {"a", "add"}, {"D", "delete"},
		{"tab", "section"}, {"←→", "role"}, {"grdmo", "set"},
	}
	// Only advertise the picker shortcut when there is something to pick.
	if m.section == sectionTargets && len(m.plan.Targets) > 0 &&
		m.plan.Targets[m.targetIdx].Ambiguous() {
		entries = append(entries, helpEntry{"c", "choose"})
	}
	entries = append(entries,
		helpEntry{"R", "resolve"}, helpEntry{"^A", "apply"},
		helpEntry{"^D", "dry-run"}, helpEntry{"q", "quit"})
	return renderHelp(entries, m.frameWidth())
}

func renderHelp(entries []helpEntry, width int) string {
	var parts []string
	for _, e := range entries {
		parts = append(parts, helpKey.Render(e.key)+helpText.Render(" "+e.action))
	}

	var lines []string
	cur := ""
	for _, p := range parts {
		sep := "   "
		if cur == "" {
			sep = ""
		}
		if lipgloss.Width(cur)+lipgloss.Width(sep)+lipgloss.Width(p) > width {
			lines = append(lines, cur)
			cur = p
			continue
		}
		cur += sep + p
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------- picker

func (m Model) viewPicker() string {
	row := m.plan.Targets[m.pickRow]

	pathW := m.panelWidth() - 26
	if pathW < 20 {
		pathW = 20
	}

	var rows []string
	for i, cand := range row.Target.Candidates {
		kindStyle := kindTag
		if cand.Kind == "group" {
			// A group grant cascades, so it reads as a warning, not a label.
			kindStyle = warnText
		}
		scope := ""
		if s := cand.Scope(); s != "" {
			scope = dimText.Render("  " + s)
		}

		line := kindStyle.Render(pad(cand.Kind, 8)) +
			bodyText.Render(pad(ellipsize(cand.FullPath, pathW), pathW)) + scope

		if i == m.pickIdx {
			mark := cursorMark.Render(glyphCursor) + " "
			rows = append(rows, rowSelected.Render(padPlain(mark+line, m.panelWidth()-2)))
			continue
		}
		rows = append(rows, "  "+line)
	}

	body := warnText.Render(glyphWarn+" "+row.Raw) +
		dimText.Render(fmt.Sprintf("  matches %d places, pick one", len(row.Target.Candidates))) + "\n" +
		dimText.Render(strings.Repeat("─", m.ruleWidth())) + "\n" +
		strings.Join(rows, "\n") + "\n" +
		dimText.Render(strings.Repeat("─", m.ruleWidth())) + "\n" +
		dimText.Render("tip: type group:name or project:name to skip this step")

	rendered := panelActive.Width(m.panelWidth()).Render(body)
	return "\n" + indentBlock(injectTitle(rendered, panelTag.Render("disambiguate"), true)) +
		"\n" + renderHelp([]helpEntry{
		{"enter", "pick"}, {"g/p", "filter kind"}, {"esc", "cancel"},
	}, m.frameWidth())
}

// ---------------------------------------------------------------- text

func pad(s string, width int) string {
	if n := lipgloss.Width(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// padPlain pads a styled string to a visible width, used before applying a
// background so the highlight spans the whole row.
func padPlain(s string, width int) string {
	if n := lipgloss.Width(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

func ellipsize(s string, width int) string {
	r := []rune(s)
	if width <= 0 || len(r) <= width {
		return s
	}
	if width <= 3 {
		return string(r[:width])
	}
	return "…" + string(r[len(r)-(width-1):])
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

// stripANSI removes escape sequences so widths can be measured on raw text.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
				inEsc = false
			}
			continue
		}
		out.WriteByte(ch)
	}
	return out.String()
}
