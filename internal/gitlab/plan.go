package gitlab

import "strings"

// UserRow is one person in the plan.
type UserRow struct {
	Raw    string
	User   *User
	Status string
}

func (u *UserRow) Resolved() bool { return u.User != nil }

func (u *UserRow) Label() string {
	if u.User != nil {
		return u.User.Username
	}
	if u.Raw == "" {
		return "<empty>"
	}
	return u.Raw
}

func (u *UserRow) Detail() string {
	if u.User != nil {
		return u.User.Name
	}
	return u.Status
}

// TargetRow is one project or group with the role to grant on it.
type TargetRow struct {
	Raw    string
	Role   Role
	Target *Target
	Status string
}

func (t *TargetRow) Resolved() bool { return t.Target.Resolved() }

func (t *TargetRow) Label() string {
	if t.Target != nil && t.Target.FullPath != "" {
		return t.Target.FullPath
	}
	if t.Raw == "" {
		return "<empty>"
	}
	return t.Raw
}

func (t *TargetRow) Kind() string {
	if t.Target != nil {
		return t.Target.Kind
	}
	return ""
}

// Outcome is the result of one (user, target) pair.
type Outcome struct {
	User   string
	Target string
	Role   Role
	Result string
}

func (o Outcome) Failed() bool {
	return strings.HasPrefix(o.Result, "FAIL") || strings.HasPrefix(o.Result, "SKIP")
}

// Plan is the cartesian product of users and targets.
type Plan struct {
	Users     []*UserRow
	Targets   []*TargetRow
	Outcomes  []Outcome
	ExpiresAt string
}

// BuildPlan folds duplicates: a name repeated in the input is one grant, and
// for a repeated target the last role wins rather than issuing two
// conflicting calls.
func BuildPlan(userEntries, targetEntries []string, def Role) *Plan {
	p := &Plan{}

	seen := map[string]bool{}
	for _, raw := range userEntries {
		name := ParseUser(raw)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		p.Users = append(p.Users, &UserRow{Raw: name})
	}

	index := map[string]*TargetRow{}
	for _, raw := range targetEntries {
		path, role, ok := ParseTarget(raw)
		if path == "" {
			continue
		}
		key := strings.ToLower(path)
		if existing, dup := index[key]; dup {
			if ok {
				existing.Role = role
			}
			continue
		}
		r := def
		if ok {
			r = role
		}
		row := &TargetRow{Raw: path, Role: r}
		index[key] = row
		p.Targets = append(p.Targets, row)
	}
	return p
}

func (p *Plan) AddUser(raw string) *UserRow {
	row := &UserRow{Raw: ParseUser(raw)}
	p.Users = append(p.Users, row)
	return row
}

func (p *Plan) AddTarget(raw string, def Role) *TargetRow {
	path, role, ok := ParseTarget(raw)
	r := def
	if ok {
		r = role
	}
	row := &TargetRow{Raw: path, Role: r}
	p.Targets = append(p.Targets, row)
	return row
}

// PairCount is the number of distinct grants Apply will issue.
func (p *Plan) PairCount() int {
	users := map[string]bool{}
	for _, u := range p.Users {
		if u.Raw != "" {
			users[strings.ToLower(u.Raw)] = true
		}
	}
	targets := map[string]bool{}
	for _, t := range p.Targets {
		if t.Raw != "" {
			targets[strings.ToLower(t.Raw)] = true
		}
	}
	return len(users) * len(targets)
}

func (p *Plan) ResolveUser(c *Client, row *UserRow) {
	row.User, row.Status = nil, ""
	if row.Raw == "" {
		row.Status = "empty"
		return
	}
	user, err := c.ResolveUser(row.Raw)
	if err != nil {
		row.Status = err.Error()
		return
	}
	row.User, row.Status = user, "ok"
}

func (p *Plan) ResolveTarget(c *Client, row *TargetRow) {
	row.Target, row.Status = nil, ""
	if row.Raw == "" {
		row.Status = "empty"
		return
	}
	t := c.ResolveTarget(row.Raw)
	row.Target, row.Status = t, t.Status
	if t.Resolved() && t.FullPath != "" {
		row.Raw = t.FullPath
	}
}

// ResolveAll fills in every row; progress may be nil.
func (p *Plan) ResolveAll(c *Client, progress func(string)) {
	for _, row := range p.Users {
		if progress != nil {
			progress("user " + row.Raw)
		}
		p.ResolveUser(c, row)
	}
	for _, row := range p.Targets {
		if progress != nil {
			progress("target " + row.Raw)
		}
		p.ResolveTarget(c, row)
	}
}

// Apply runs every distinct pair. A failure on one pair never stops the rest.
func (p *Plan) Apply(c *Client, dryRun bool, progress func(string)) []Outcome {
	p.Outcomes = nil
	done := map[string]bool{}

	for _, user := range p.Users {
		if user.Raw == "" {
			continue
		}
		if !user.Resolved() {
			p.ResolveUser(c, user)
		}
		for _, target := range p.Targets {
			if target.Raw == "" {
				continue
			}
			if !target.Resolved() {
				p.ResolveTarget(c, target)
			}

			key := strings.ToLower(user.Raw) + "\x00" + strings.ToLower(target.Raw)
			if done[key] {
				continue
			}
			done[key] = true

			out := Outcome{User: user.Label(), Target: target.Label(), Role: target.Role}
			switch {
			case !user.Resolved():
				out.Result = "SKIP user: " + user.Status
			case !target.Resolved():
				out.Result = "SKIP target: " + target.Status
			case dryRun:
				out.Result = "DRY would set " + target.Role.Name
			default:
				if progress != nil {
					progress(user.Label() + " -> " + target.Label())
				}
				out.Result = c.Grant(target.Target, user.User.ID, target.Role.Level, p.ExpiresAt)
			}
			p.Outcomes = append(p.Outcomes, out)
		}
	}
	return p.Outcomes
}

func (p *Plan) Failures() int {
	n := 0
	for _, o := range p.Outcomes {
		if o.Failed() {
			n++
		}
	}
	return n
}
