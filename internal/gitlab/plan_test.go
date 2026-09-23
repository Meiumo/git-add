package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeGitLab serves the handful of endpoints the planner touches, so the
// tests exercise the real client including its caching and grant logic.
func fakeGitLab(t *testing.T, grants *[]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v4/users", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("username")
		if name == "" {
			name = r.URL.Query().Get("search")
		}
		known := map[string]int{"ivanov": 1, "petrov": 2}
		id, ok := known[strings.ToLower(name)]
		if !ok {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": id, "username": strings.ToLower(name), "name": strings.ToUpper(name)},
		})
	})

	mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v4/projects/")
		if strings.Contains(path, "/members") {
			parts := strings.SplitN(path, "/members", 2)
			if r.Method == http.MethodPost {
				body, _ := json.Marshal(map[string]any{})
				var payload map[string]any
				_ = json.NewDecoder(r.Body).Decode(&payload)
				*grants = append(*grants, fmt.Sprintf("%s:%v:%v", parts[0], payload["user_id"], payload["access_level"]))
				w.WriteHeader(http.StatusCreated)
				w.Write(body)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// ServeMux hands over a decoded path, so the %2F the client sent is
		// already back to a plain slash here.
		switch path {
		case "dso/a", "dso/b":
			id := 10
			if path == "dso/b" {
				id = 11
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": id, "path_with_namespace": path,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	mux.HandleFunc("/api/v4/groups/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	return httptest.NewServer(mux)
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return New(Config{URL: srv.URL, Token: "test"})
}

func TestPlanCartesian(t *testing.T) {
	var grants []string
	srv := fakeGitLab(t, &grants)
	defer srv.Close()

	plan := BuildPlan([]string{"ivanov", "petrov"}, []string{"dso/a", "dso/b/m"}, DefaultRole)
	outcomes := plan.Apply(newTestClient(t, srv), false, nil)

	if len(outcomes) != 4 {
		t.Fatalf("expected 4 outcomes, got %d", len(outcomes))
	}
	if len(grants) != 4 {
		t.Fatalf("expected 4 grants, got %v", grants)
	}
	want := map[string]bool{
		"10:1:30": true, "11:1:40": true,
		"10:2:30": true, "11:2:40": true,
	}
	for _, g := range grants {
		if !want[g] {
			t.Errorf("unexpected grant %s", g)
		}
	}
}

func TestPlanDryRun(t *testing.T) {
	var grants []string
	srv := fakeGitLab(t, &grants)
	defer srv.Close()

	plan := BuildPlan([]string{"ivanov"}, []string{"dso/a"}, DefaultRole)
	outcomes := plan.Apply(newTestClient(t, srv), true, nil)

	if len(grants) != 0 {
		t.Fatalf("dry run wrote %v", grants)
	}
	if !strings.HasPrefix(outcomes[0].Result, "DRY") {
		t.Fatalf("got %q", outcomes[0].Result)
	}
}

func TestPlanSkipsUnknownUser(t *testing.T) {
	var grants []string
	srv := fakeGitLab(t, &grants)
	defer srv.Close()

	plan := BuildPlan([]string{"ivanov", "ghost"}, []string{"dso/a"}, DefaultRole)
	outcomes := plan.Apply(newTestClient(t, srv), false, nil)

	var ghost, good Outcome
	for _, o := range outcomes {
		if o.User == "ghost" {
			ghost = o
		} else {
			good = o
		}
	}
	if !strings.HasPrefix(ghost.Result, "SKIP user") {
		t.Fatalf("ghost result = %q", ghost.Result)
	}
	if good.Failed() {
		t.Fatalf("good result = %q", good.Result)
	}
	if len(grants) != 1 {
		t.Fatalf("expected 1 grant, got %v", grants)
	}
}

func TestPlanSkipsUnknownTarget(t *testing.T) {
	var grants []string
	srv := fakeGitLab(t, &grants)
	defer srv.Close()

	plan := BuildPlan([]string{"ivanov"}, []string{"dso/a", "dso/nope"}, DefaultRole)
	plan.Apply(newTestClient(t, srv), false, nil)

	if plan.Failures() != 1 {
		t.Fatalf("expected 1 failure, got %d", plan.Failures())
	}
}

func TestDedupe(t *testing.T) {
	plan := BuildPlan([]string{"ivanov", "IVANOV", "petrov"}, []string{"dso/x", "dso/x/m"}, DefaultRole)
	if len(plan.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(plan.Users))
	}
	if len(plan.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(plan.Targets))
	}
	if plan.Targets[0].Role.Key != "m" {
		t.Fatalf("last role should win, got %s", plan.Targets[0].Role.Key)
	}
	if plan.PairCount() != 2 {
		t.Fatalf("expected 2 pairs, got %d", plan.PairCount())
	}
}

func TestApplyDedupesEditedRows(t *testing.T) {
	var grants []string
	srv := fakeGitLab(t, &grants)
	defer srv.Close()

	// Simulates a form where the same row was typed twice by hand.
	plan := &Plan{}
	plan.AddUser("ivanov")
	plan.AddUser("ivanov")
	plan.AddTarget("dso/a", DefaultRole)

	outcomes := plan.Apply(newTestClient(t, srv), false, nil)
	if len(outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(outcomes))
	}
}

func TestExpiryIsSent(t *testing.T) {
	var seen string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/users", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "username": "ivanov", "name": "I"}})
	})
	mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/members") {
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if v, ok := payload["expires_at"].(string); ok {
				seen = v
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("{}"))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 10, "path_with_namespace": "dso/a"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	plan := BuildPlan([]string{"ivanov"}, []string{"dso/a"}, DefaultRole)
	plan.ExpiresAt = "2026-12-31"
	plan.Apply(New(Config{URL: srv.URL, Token: "t"}), false, nil)

	if seen != "2026-12-31" {
		t.Fatalf("expires_at = %q", seen)
	}
}

func TestGrantUpdatesExistingMember(t *testing.T) {
	mux := http.NewServeMux()
	puts := 0
	mux.HandleFunc("/api/v4/projects/10/members", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message":"Member already exists"}`))
	})
	mux.HandleFunc("/api/v4/projects/10/members/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_level": 20})
			return
		}
		puts++
		w.Write([]byte("{}"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "t"})
	target := &Target{Kind: "project", ID: 10, FullPath: "dso/a", Status: "ok"}
	got := c.Grant(target, 1, 40, "")

	if puts != 1 {
		t.Fatalf("expected a PUT, got %d", puts)
	}
	if got != "updated reporter -> maintainer" {
		t.Fatalf("got %q", got)
	}
}

func TestGrantNoopWhenLevelMatches(t *testing.T) {
	mux := http.NewServeMux()
	puts := 0
	mux.HandleFunc("/api/v4/groups/5/members", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message":"Member already exists"}`))
	})
	mux.HandleFunc("/api/v4/groups/5/members/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_level": 40})
			return
		}
		puts++
		w.Write([]byte("{}"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "t"})
	target := &Target{Kind: "group", ID: 5, FullPath: "dso", Status: "ok"}
	got := c.Grant(target, 1, 40, "")

	if puts != 0 {
		t.Fatalf("should not write when the level already matches")
	}
	if got != "already maintainer" {
		t.Fatalf("got %q", got)
	}
}
