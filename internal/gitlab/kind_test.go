package gitlab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseTargetKind(t *testing.T) {
	cases := []struct {
		in       string
		wantPath string
		wantKind Kind
		wantRole string
	}{
		{"group:infra", "infra", KindGroup, ""},
		{"g:infra", "infra", KindGroup, ""},
		{"project:infra", "infra", KindProject, ""},
		{"p:infra", "infra", KindProject, ""},
		{"repo:infra", "infra", KindProject, ""},
		{"group:infra/m", "infra", KindGroup, "maintainer"},
		{"group: dso/platform", "dso/platform", KindGroup, ""},
		{"GROUP:infra", "infra", KindGroup, ""},
		{"infra", "infra", KindAny, ""},
		// A URL keeps its scheme: https: must not read as a qualifier.
		{"https://gl.corp/dso/repo", "dso/repo", KindAny, ""},
		{"git@gl.corp:dso/repo.git", "dso/repo", KindAny, ""},
	}

	for _, c := range cases {
		path, kind, role, ok := ParseTargetKind(c.in)
		if path != c.wantPath {
			t.Errorf("ParseTargetKind(%q) path = %q, want %q", c.in, path, c.wantPath)
		}
		if kind != c.wantKind {
			t.Errorf("ParseTargetKind(%q) kind = %q, want %q", c.in, kind, c.wantKind)
		}
		if c.wantRole == "" && ok {
			t.Errorf("ParseTargetKind(%q) unexpected role %s", c.in, role.Name)
		}
		if c.wantRole != "" && (!ok || role.Name != c.wantRole) {
			t.Errorf("ParseTargetKind(%q) role = %v, want %s", c.in, role, c.wantRole)
		}
	}
}

// searchServer answers both search endpoints so kind filtering can be tested.
func searchServer(t *testing.T, projectHits, groupHits []string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		var out []map[string]any
		for i, p := range projectHits {
			out = append(out, map[string]any{
				"id": 100 + i, "path": lastSegment(p), "path_with_namespace": p,
			})
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/api/v4/groups", func(w http.ResponseWriter, r *http.Request) {
		var out []map[string]any
		for i, g := range groupHits {
			out = append(out, map[string]any{
				"id": 200 + i, "path": lastSegment(g), "full_path": g,
			})
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	// Group project counts for the scope hint.
	mux.HandleFunc("/api/v4/groups/200/projects", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 1}, {"id": 2}, {"id": 3}})
	})

	return httptest.NewServer(mux)
}

func lastSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func TestKindFilterNarrowsSearch(t *testing.T) {
	srv := searchServer(t, []string{"dso/k8s-values", "dso/k8s-templates"}, []string{"dso/k8s-platform"})
	defer srv.Close()
	c := New(Config{URL: srv.URL, Token: "t"})

	any := c.ResolveTargetKind("k8s", KindAny)
	if len(any.Candidates) != 3 {
		t.Fatalf("KindAny: expected 3 candidates, got %d", len(any.Candidates))
	}

	groups := c.ResolveTargetKind("k8s", KindGroup)
	if !groups.Resolved() || groups.Kind != "group" {
		t.Fatalf("KindGroup should resolve to the single group, got %+v", groups)
	}

	projects := c.ResolveTargetKind("k8s", KindProject)
	if len(projects.Candidates) != 2 {
		t.Fatalf("KindProject: expected 2 candidates, got %d", len(projects.Candidates))
	}
	for _, cand := range projects.Candidates {
		if cand.Kind != "project" {
			t.Fatalf("KindProject returned a %s", cand.Kind)
		}
	}
}

func TestCandidateOrderPutsProjectsFirst(t *testing.T) {
	srv := searchServer(t, []string{"dso/deep/nested/thing"}, []string{"dso/thing"})
	defer srv.Close()
	c := New(Config{URL: srv.URL, Token: "t"})

	got := c.ResolveTargetKind("thing", KindAny)
	if len(got.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(got.Candidates))
	}
	// Both names match exactly, so the narrower grant (project) wins.
	if got.Candidates[0].Kind != "project" {
		t.Fatalf("expected a project first, got %s", got.Candidates[0].Kind)
	}
}

func TestGroupCandidateCarriesScope(t *testing.T) {
	srv := searchServer(t, []string{"dso/k8s-values", "dso/k8s-extra"}, []string{"dso/k8s-platform"})
	defer srv.Close()
	c := New(Config{URL: srv.URL, Token: "t"})

	got := c.ResolveTargetKind("k8s", KindAny)
	var group *Candidate
	for i := range got.Candidates {
		if got.Candidates[i].Kind == "group" {
			group = &got.Candidates[i]
		}
	}
	if group == nil {
		t.Fatal("no group candidate")
	}
	if group.Projects != 3 {
		t.Fatalf("group project count = %d, want 3", group.Projects)
	}
	if scope := group.Scope(); scope != "3 projects" {
		t.Fatalf("scope = %q", scope)
	}
}

func TestKindIsPartOfRowIdentity(t *testing.T) {
	// "group:infra" and "project:infra" are two grants, not a duplicate.
	plan := BuildPlan([]string{"a"}, []string{"group:infra", "project:infra"}, DefaultRole)
	if len(plan.Targets) != 2 {
		t.Fatalf("expected 2 target rows, got %d", len(plan.Targets))
	}
	if plan.Targets[0].Want != KindGroup || plan.Targets[1].Want != KindProject {
		t.Fatalf("kinds not carried: %+v", plan.Targets)
	}
}

func TestChooseFixesRow(t *testing.T) {
	row := &TargetRow{Raw: "k8s", Role: DefaultRole}
	row.Target = &Target{Raw: "k8s", Status: "ambiguous (2)", Candidates: []Candidate{
		{Kind: "group", ID: 7, FullPath: "dso/platform", Name: "platform"},
	}}
	if !row.Ambiguous() {
		t.Fatal("row should be ambiguous")
	}
	row.Choose(row.Target.Candidates[0])
	if !row.Resolved() || row.Raw != "dso/platform" || row.Target.ID != 7 {
		t.Fatalf("Choose did not fix the row: %+v", row)
	}
	if row.Ambiguous() {
		t.Fatal("row should no longer be ambiguous")
	}
}
