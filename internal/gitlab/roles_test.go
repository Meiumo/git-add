package gitlab

import "testing"

// Package-level vars initialise before init(), so a map-based DefaultRole
// would silently be the zero Role and grant access level 0.
func TestDefaultRoleIsPopulated(t *testing.T) {
	if DefaultRole.Level == 0 || DefaultRole.Key != "d" || DefaultRole.Name != "developer" {
		t.Fatalf("DefaultRole = %+v, want developer/30", DefaultRole)
	}
}

func TestEveryRoleHasNonZeroLevel(t *testing.T) {
	for _, r := range Roles {
		if r.Level == 0 || r.Key == "" || r.Name == "" {
			t.Fatalf("incomplete role %+v", r)
		}
	}
}

func TestLevelName(t *testing.T) {
	if got := LevelName(40); got != "maintainer" {
		t.Fatalf("got %q", got)
	}
	if got := LevelName(0); got != "?" {
		t.Fatalf("got %q", got)
	}
	// A future GitLab level must not render as an empty column.
	if got := LevelName(45); got != "45" {
		t.Fatalf("got %q", got)
	}
}

func TestRoleIndex(t *testing.T) {
	if got := RoleIndex(mustRole("o")); got != 4 {
		t.Fatalf("got %d", got)
	}
}
