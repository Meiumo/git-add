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

// ParseTarget splits a raw entry into a path and an optional role.
//
// A trailing segment is only eaten as a role when it is a known role token,
// so "dso/sub/group" keeps all three segments while "dso/repo/m" does not.
func ParseTarget(raw string) (string, Role, bool) {
	s := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), ","))
	if s == "" {
		return "", Role{}, false
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

	return strings.Trim(s, "/"), role, haveRole
}

// ParseUser normalises a user entry.
func ParseUser(raw string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "@")), ","))
}
