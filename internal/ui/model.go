// Package ui is the interactive form, built on Bubbletea's Elm architecture:
// immutable model, messages in, view out. Network calls run as commands so the
// form never blocks while GitLab is being queried.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/meiumo/gitadd/internal/gitlab"
)

type section int

const (
	sectionUsers section = iota
	sectionTargets
)

type mode int

const (
	modeBrowse mode = iota
	modeEdit
	modePick
)

// Messages carrying the result of background work.
type (
	userResolvedMsg   struct{ index int }
	targetResolvedMsg struct{ index int }
	appliedMsg        struct{}
	statusMsg         struct{ text string }
)

type Model struct {
	client *gitlab.Client
	plan   *gitlab.Plan

	section   section
	userIdx   int
	targetIdx int
	roleCol   int

	mode     mode
	input    textinput.Model
	pickRow  int
	pickIdx  int
	dryRun   bool
	status   string
	busy     bool
	applied  bool
	quitting bool

	width  int
	height int
}

func New(client *gitlab.Client, plan *gitlab.Plan, dryRun bool) Model {
	if len(plan.Users) == 0 {
		plan.AddUser("")
	}
	if len(plan.Targets) == 0 {
		plan.AddTarget("", gitlab.DefaultRole)
	}

	in := textinput.New()
	in.Prompt = "> "
	in.CharLimit = 200

	return Model{
		client:  client,
		plan:    plan,
		input:   in,
		dryRun:  dryRun,
		roleCol: gitlab.RoleIndex(gitlab.DefaultRole),
		width:   100,
		height:  30,
	}
}

func (m Model) Init() tea.Cmd {
	return m.resolveAllCmd()
}

// ---------------------------------------------------------------- commands

func (m Model) resolveUserCmd(i int) tea.Cmd {
	return func() tea.Msg {
		if i < len(m.plan.Users) {
			m.plan.ResolveUser(m.client, m.plan.Users[i])
		}
		return userResolvedMsg{index: i}
	}
}

func (m Model) resolveTargetCmd(i int) tea.Cmd {
	return func() tea.Msg {
		if i < len(m.plan.Targets) {
			m.plan.ResolveTarget(m.client, m.plan.Targets[i])
		}
		return targetResolvedMsg{index: i}
	}
}

func (m Model) resolveAllCmd() tea.Cmd {
	return func() tea.Msg {
		m.plan.ResolveAll(m.client, nil)
		return statusMsg{text: "resolved"}
	}
}

func (m Model) applyCmd() tea.Cmd {
	plan, client, dry := m.plan, m.client, m.dryRun
	return func() tea.Msg {
		plan.Apply(client, dry, nil)
		return appliedMsg{}
	}
}

// ---------------------------------------------------------------- update

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case userResolvedMsg, targetResolvedMsg:
		m.busy = false
		return m, nil

	case statusMsg:
		m.busy = false
		m.status = msg.text
		return m, nil

	case appliedMsg:
		m.busy = false
		m.applied = true
		failed := m.plan.Failures()
		total := len(m.plan.Outcomes)
		m.status = fmt.Sprintf("done: %d/%d ok", total-failed, total)
		if m.dryRun {
			m.status += " (dry run)"
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeEdit:
			return m.updateEdit(msg)
		case modePick:
			return m.updatePick(msg)
		default:
			return m.updateBrowse(msg)
		}
	}
	return m, nil
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "tab", "shift+tab":
		m.section = 1 - m.section
		return m, nil

	case "up", "k":
		if m.section == sectionUsers {
			if m.userIdx > 0 {
				m.userIdx--
			}
		} else if m.targetIdx > 0 {
			m.targetIdx--
		} else {
			m.section = sectionUsers
			m.userIdx = len(m.plan.Users) - 1
		}
		return m, nil

	case "down", "j":
		if m.section == sectionUsers {
			if m.userIdx < len(m.plan.Users)-1 {
				m.userIdx++
			} else {
				m.section = sectionTargets
				m.targetIdx = 0
			}
		} else if m.targetIdx < len(m.plan.Targets)-1 {
			m.targetIdx++
		}
		return m, nil

	case "left", "h":
		if m.section == sectionTargets && m.roleCol > 0 {
			m.roleCol--
		}
		return m, nil

	case "right", "l":
		if m.section == sectionTargets && m.roleCol < len(gitlab.Roles)-1 {
			m.roleCol++
		}
		return m, nil

	case " ":
		if m.section == sectionTargets && len(m.plan.Targets) > 0 {
			m.plan.Targets[m.targetIdx].Role = gitlab.Roles[m.roleCol]
		}
		return m, nil

	case "enter":
		return m.startEdit()

	case "a":
		if m.section == sectionUsers {
			m.plan.Users = insertUser(m.plan.Users, m.userIdx+1, &gitlab.UserRow{})
			m.userIdx++
		} else {
			role := gitlab.DefaultRole
			if len(m.plan.Targets) > 0 {
				role = m.plan.Targets[m.targetIdx].Role
			}
			m.plan.Targets = insertTarget(m.plan.Targets, m.targetIdx+1, &gitlab.TargetRow{Role: role})
			m.targetIdx++
		}
		return m.startEdit()

	case "D":
		if m.section == sectionUsers && len(m.plan.Users) > 0 {
			m.plan.Users = append(m.plan.Users[:m.userIdx], m.plan.Users[m.userIdx+1:]...)
			if len(m.plan.Users) == 0 {
				m.plan.AddUser("")
			}
			if m.userIdx >= len(m.plan.Users) {
				m.userIdx = len(m.plan.Users) - 1
			}
		} else if m.section == sectionTargets && len(m.plan.Targets) > 0 {
			m.plan.Targets = append(m.plan.Targets[:m.targetIdx], m.plan.Targets[m.targetIdx+1:]...)
			if len(m.plan.Targets) == 0 {
				m.plan.AddTarget("", gitlab.DefaultRole)
			}
			if m.targetIdx >= len(m.plan.Targets) {
				m.targetIdx = len(m.plan.Targets) - 1
			}
		}
		return m, nil

	case "R":
		m.busy = true
		m.status = "resolving..."
		return m, m.resolveAllCmd()

	case "ctrl+a":
		if m.plan.PairCount() == 0 {
			m.status = "nothing to apply: need at least one user and one target"
			return m, nil
		}
		m.busy = true
		m.status = "applying..."
		return m, m.applyCmd()

	case "ctrl+d":
		m.dryRun = !m.dryRun
		if m.dryRun {
			m.status = "dry-run ON"
		} else {
			m.status = "dry-run OFF"
		}
		return m, nil
	}

	// Single-letter role shortcuts.
	if m.section == sectionTargets && len(msg.String()) == 1 {
		if role, ok := gitlab.ParseRole(msg.String()); ok {
			m.plan.Targets[m.targetIdx].Role = role
			m.roleCol = gitlab.RoleIndex(role)
			return m, nil
		}
	}
	return m, nil
}

func (m Model) startEdit() (tea.Model, tea.Cmd) {
	m.mode = modeEdit
	if m.section == sectionUsers {
		m.input.SetValue(m.plan.Users[m.userIdx].Raw)
		m.input.Placeholder = "username, id or email"
	} else {
		m.input.SetValue(m.plan.Targets[m.targetIdx].Raw)
		m.input.Placeholder = "group/repo, URL or name"
	}
	m.input.CursorEnd()
	m.input.Focus()
	return m, textinput.Blink
}

func (m Model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeBrowse
		m.input.Blur()
		return m, nil

	case "enter":
		value := strings.TrimSpace(m.input.Value())
		m.mode = modeBrowse
		m.input.Blur()
		m.busy = true

		if m.section == sectionUsers {
			m.plan.Users[m.userIdx].Raw = gitlab.ParseUser(value)
			return m, m.resolveUserCmd(m.userIdx)
		}
		path, role, ok := gitlab.ParseTarget(value)
		row := m.plan.Targets[m.targetIdx]
		row.Raw = path
		if ok {
			row.Role = role
			m.roleCol = gitlab.RoleIndex(role)
		}
		return m, m.resolveTargetCmd(m.targetIdx)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updatePick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	row := m.plan.Targets[m.pickRow]
	cands := row.Target.Candidates

	switch msg.String() {
	case "esc", "q":
		m.mode = modeBrowse
		return m, nil
	case "up", "k":
		if m.pickIdx > 0 {
			m.pickIdx--
		}
	case "down", "j":
		if m.pickIdx < len(cands)-1 {
			m.pickIdx++
		}
	case "enter":
		c := cands[m.pickIdx]
		row.Target.Kind, row.Target.ID = c.Kind, c.ID
		row.Target.FullPath, row.Target.Status = c.FullPath, "ok"
		row.Target.Candidates = nil
		row.Raw, row.Status = c.FullPath, "ok"
		m.mode = modeBrowse
	}
	return m, nil
}

// OpenPicker switches to the candidate list for an ambiguous target.
func (m *Model) OpenPicker(rowIdx int) {
	m.mode = modePick
	m.pickRow = rowIdx
	m.pickIdx = 0
}

func insertUser(rows []*gitlab.UserRow, at int, row *gitlab.UserRow) []*gitlab.UserRow {
	if at > len(rows) {
		at = len(rows)
	}
	rows = append(rows, nil)
	copy(rows[at+1:], rows[at:])
	rows[at] = row
	return rows
}

func insertTarget(rows []*gitlab.TargetRow, at int, row *gitlab.TargetRow) []*gitlab.TargetRow {
	if at > len(rows) {
		at = len(rows)
	}
	rows = append(rows, nil)
	copy(rows[at+1:], rows[at:])
	rows[at] = row
	return rows
}

// Outcomes exposes the applied results for the final report.
func (m Model) Outcomes() []gitlab.Outcome { return m.plan.Outcomes }

var _ = lipgloss.NewStyle
