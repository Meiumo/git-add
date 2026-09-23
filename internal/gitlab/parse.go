package gitlab

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	splitRe    = regexp.MustCompile(`[,\n;]+`)
	roleTailRe = regexp.MustCompile(`[:=]\s*([A-Za-z]+|\d{2})\s*$`)
	webSuffix  = regexp.MustCompile(`/-/(tree|blob|merge_requests|issues|pipelines|settings|members).*$`)
)

// SplitList breaks argv entries on commas, newlines and semicolons.
func SplitList(chunks []string) []string {
	var out []string
	for _, chunk := range chunks {
		for _, part := range splitRe.Split(chunk, -1) {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// URLToPath reduces a browser or clone URL to "group/sub/repo".
func URLToPath(value string) string {
	v := strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(v, "git@"):
		if i := strings.Index(v, ":"); i >= 0 {
			v = v[i+1:]
		}
	case strings.HasPrefix(v, "http://"), strings.HasPrefix(v, "https://"):
		if u, err := url.Parse(v); err == nil {
			v = u.Path
		}
	}
	v = strings.Trim(v, "/")
	v = strings.TrimSuffix(v, ".git")
	v = webSuffix.ReplaceAllString(v, "")
	return strings.Trim(v, "/")
}

// Kind narrows what a bare name is allowed to match. Granting on a group
// cascades to every project inside it, so the distinction is not cosmetic and
// the caller is given a way to state it up front.
type Kind string

const (
	KindAny     Kind = ""
	KindProject Kind = "project"
	KindGroup   Kind = "group"
)

// kindPrefixRe matches an explicit "group:" or "project:" qualifier. The
// short forms g: and p: are accepted because they are quicker to type and
// cannot collide with a role suffix, which only ever appears at the end.
var kindPrefixRe = regexp.MustCompile(`(?i)^(group|groups|g|project|proj|p|repo)\s*:\s*`)

// ParseTarget splits a raw entry into a path, an optional kind qualifier and
// an optional role.
//
// A trailing segment is only eaten as a role when it is a known role token,
// so "dso/sub/group" keeps all three segments while "dso/repo/m" does not.
func ParseTarget(raw string) (string, Role, bool) {
	path, _, role, ok := ParseTargetKind(raw)
	return path, role, ok
}

// ParseTargetKind is ParseTarget plus the explicit kind qualifier.
func ParseTargetKind(raw string) (string, Kind, Role, bool) {
	s := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), ","))
	if s == "" {
		return "", KindAny, Role{}, false
	}

	kind := KindAny
	// Only strip a qualifier when it is not part of a URL scheme.
	if !strings.HasPrefix(strings.ToLower(s), "http://") &&
		!strings.HasPrefix(strings.ToLower(s), "https://") {
		if m := kindPrefixRe.FindStringSubmatch(s); m != nil {
			switch strings.ToLower(m[1]) {
			case "group", "groups", "g":
				kind = KindGroup
			default:
				kind = KindProject
			}
			s = strings.TrimSpace(s[len(m[0]):])
		}
	}

	var role Role
	var haveRole bool

	if m := roleTailRe.FindStringSubmatchIndex(s); m != nil {
		if r, ok := ParseRole(s[m[2]:m[3]]); ok {
			role, haveRole = r, true
			s = strings.TrimSpace(s[:m[0]])
		}
	}

	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git@") || strings.Contains(s, "/-/") {
		s = URLToPath(s)
	}

	if !haveRole {
		if i := strings.LastIndex(s, "/"); i > 0 {
			if r, ok := ParseRole(s[i+1:]); ok {
				role, haveRole = r, true
				s = s[:i]
			}
		}
	}

	return strings.Trim(s, "/"), kind, role, haveRole
}

// ParseUser normalises a user entry.
func ParseUser(raw string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "@")), ","))
}
