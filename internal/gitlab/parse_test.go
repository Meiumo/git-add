package gitlab

import "testing"

func TestSplitList(t *testing.T) {
	got := SplitList([]string{"a,", "b/m,", "c"})
	want := []string{"a", "b/m", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestSplitListSeparators(t *testing.T) {
	if got := SplitList([]string{"a\nb;c, d"}); len(got) != 4 {
		t.Fatalf("expected 4 entries, got %v", got)
	}
	if got := SplitList([]string{"", "  ", ","}); len(got) != 0 {
		t.Fatalf("expected none, got %v", got)
	}
}

func TestParseRole(t *testing.T) {
	cases := map[string]string{"m": "maintainer", "maintainer": "maintainer", "40": "maintainer", "o": "owner"}
	for in, want := range cases {
		r, ok := ParseRole(in)
		if !ok || r.Name != want {
			t.Fatalf("ParseRole(%q) = %v, %v; want %s", in, r, ok, want)
		}
	}
	for _, bad := range []string{"admin", "", "99"} {
		if _, ok := ParseRole(bad); ok {
			t.Fatalf("ParseRole(%q) should fail", bad)
		}
	}
}

func TestParseTarget(t *testing.T) {
	cases := []struct {
		in       string
		wantPath string
		wantRole string
	}{
		{"dso/cicd-supply", "dso/cicd-supply", ""},
		{"dso/cicd-supply/m", "dso/cicd-supply", "maintainer"},
		{"dso/cicd-supply:maintainer", "dso/cicd-supply", "maintainer"},
		{"https://gl.corp/dso/cicd-supply.git", "dso/cicd-supply", ""},
		{"https://gl.corp/dso/cicd-supply/-/tree/main/x", "dso/cicd-supply", ""},
		{"git@gl.corp:dso/k8s-values.git", "dso/k8s-values", ""},
		{"https://gl.corp/dso/msop/r", "dso/msop", "reporter"},
		{"dso/sub/group", "dso/sub/group", ""},
		{"cicd-supply", "cicd-supply", ""},
		{"m", "m", ""},
		{"  k8s-values/d  ", "k8s-values", "developer"},
	}
	for _, c := range cases {
		path, role, ok := ParseTarget(c.in)
		if path != c.wantPath {
			t.Errorf("ParseTarget(%q) path = %q, want %q", c.in, path, c.wantPath)
		}
		if c.wantRole == "" && ok {
			t.Errorf("ParseTarget(%q) unexpectedly found role %s", c.in, role.Name)
		}
		if c.wantRole != "" && (!ok || role.Name != c.wantRole) {
			t.Errorf("ParseTarget(%q) role = %v %v, want %s", c.in, role, ok, c.wantRole)
		}
	}
}

func TestParseUser(t *testing.T) {
	if got := ParseUser("@ivanov"); got != "ivanov" {
		t.Fatalf("got %q", got)
	}
	if got := ParseUser("ivanov,"); got != "ivanov" {
		t.Fatalf("got %q", got)
	}
}
