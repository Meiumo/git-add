package gitlab

import (
	"strconv"
	"strings"
)

// Role is a GitLab access level with the shorthand used on the command line.
type Role struct {
	Key   string
	Name  string
	Level int
}

var Roles = []Role{
	{"g", "guest", 10},
	{"r", "reporter", 20},
	{"d", "developer", 30},
	{"m", "maintainer", 40},
	{"o", "owner", 50},
}

var (
	roleByKey   = map[string]Role{}
	roleByName  = map[string]Role{}
	roleByLevel = map[int]Role{}
)

// DefaultRole is used when an entry carries no role of its own.
//
// Built from the Roles slice rather than a map lookup: package-level variable
// initialisation runs before init(), so reading roleByKey here would yield a
// zero Role with access level 0 and silently grant nothing.
var DefaultRole = mustRole("d")

func mustRole(key string) Role {
	for _, r := range Roles {
		if r.Key == key {
			return r
		}
	}
	panic("gitlab: unknown default role " + key)
}

func init() {
	for _, r := range Roles {
		roleByKey[r.Key] = r
		roleByName[r.Name] = r
		roleByLevel[r.Level] = r
	}
}

// ParseRole accepts a key ("m"), a full name ("maintainer") or a level ("40").
func ParseRole(token string) (Role, bool) {
	t := strings.ToLower(strings.TrimSpace(token))
	if t == "" {
		return Role{}, false
	}
	if r, ok := roleByKey[t]; ok {
		return r, true
	}
	if r, ok := roleByName[t]; ok {
		return r, true
	}
	if n, err := strconv.Atoi(t); err == nil {
		if r, ok := roleByLevel[n]; ok {
			return r, true
		}
	}
	return Role{}, false
}

// LevelName renders an access level for display, tolerating unknown values
// so a future GitLab level does not turn into a blank column.
func LevelName(level int) string {
	if r, ok := roleByLevel[level]; ok {
		return r.Name
	}
	if level == 0 {
		return "?"
	}
	return strconv.Itoa(level)
}

// RoleIndex is the position of a role in Roles, for the checkbox columns.
func RoleIndex(r Role) int {
	for i, x := range Roles {
		if x.Key == r.Key {
			return i
		}
	}
	return 0
}
