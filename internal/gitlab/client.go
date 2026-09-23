package gitlab

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// APIError carries the GitLab status code alongside the decoded message.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Status == 0 {
		return e.Message
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Message)
}

// Config is everything needed to talk to one GitLab instance.
type Config struct {
	URL      string
	Token    string
	Insecure bool
	CAFile   string
	CAPool   *x509.CertPool
}

func (c Config) Host() string {
	if u, err := url.Parse(c.URL); err == nil && u.Host != "" {
		return u.Host
	}
	return c.URL
}

// Client is a thin REST wrapper with caches, so a batch of users does not
// re-resolve the same projects over and over.
type Client struct {
	cfg  Config
	http *http.Client

	mu      sync.Mutex
	users   map[string]*User
	targets map[string]*Target
}

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	State    string `json:"state"`
}

// Target is a project or group that membership can be granted on.
type Target struct {
	Raw        string
	Kind       string // "project" or "group"
	ID         int
	FullPath   string
	Status     string
	Candidates []Candidate
}

type Candidate struct {
	Kind     string
	ID       int
	FullPath string
	Name     string
}

func (t *Target) Resolved() bool { return t != nil && t.Status == "ok" && t.ID != 0 }

func (t *Target) Label() string {
	if t == nil {
		return ""
	}
	if t.FullPath != "" {
		return t.FullPath
	}
	return t.Raw
}

func New(cfg Config) *Client {
	return &Client{
		cfg:     cfg,
		http:    buildHTTPClient(cfg),
		users:   map[string]*User{},
		targets: map[string]*Target{},
	}
}

func buildHTTPClient(cfg Config) *http.Client {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.Insecure {
		tlsCfg.InsecureSkipVerify = true
	} else if cfg.CAPool != nil {
		tlsCfg.RootCAs = cfg.CAPool
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg, Proxy: http.ProxyFromEnvironment},
	}
}

func (c *Client) Config() Config { return c.cfg }

// SetTrust swaps in a rebuilt CA pool after a verification failure.
func (c *Client) SetTrust(pool *x509.CertPool) {
	c.cfg.CAPool = pool
	c.cfg.Insecure = false
	c.http = buildHTTPClient(c.cfg)
}

func (c *Client) do(method, path string, query url.Values, body any) ([]byte, error) {
	endpoint := strings.TrimRight(c.cfg.URL, "/") + "/api/v4" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.cfg.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "git-add")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &APIError{Status: 0, Message: unwrapTransport(err)}
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Message: decodeMessage(raw, resp.Status)}
	}
	return raw, nil
}

func (c *Client) getJSON(path string, query url.Values, out any) error {
	raw, err := c.do(http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func decodeMessage(raw []byte, fallback string) string {
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err == nil {
		for _, key := range []string{"message", "error"} {
			v, ok := probe[key]
			if !ok {
				continue
			}
			switch typed := v.(type) {
			case string:
				return truncate(typed)
			case map[string]any:
				var parts []string
				for k, val := range typed {
					parts = append(parts, fmt.Sprintf("%s: %v", k, val))
				}
				return truncate(strings.Join(parts, "; "))
			}
		}
	}
	if s := strings.TrimSpace(string(raw)); s != "" {
		return truncate(s)
	}
	return fallback
}

func truncate(s string) string {
	if len(s) > 300 {
		return s[:300]
	}
	return s
}

// unwrapTransport keeps the certificate wording intact so the caller can spot
// a corporate-root problem and rebuild the trust store.
func unwrapTransport(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); i > 0 && strings.Contains(msg, "x509") {
		return msg[i+2:]
	}
	return msg
}

// IsCertError reports whether the failure was TLS trust, not credentials.
func IsCertError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "x509") ||
		strings.Contains(msg, "certificate signed by unknown authority") ||
		strings.Contains(msg, "certificate verify failed")
}

// Whoami verifies the token.
func (c *Client) Whoami() (*User, error) {
	var u User
	if err := c.getJSON("/user", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ResolveUser accepts a numeric id, an exact username or a search term.
func (c *Client) ResolveUser(ident string) (*User, error) {
	key := strings.ToLower(ParseUser(ident))
	if key == "" {
		return nil, &APIError{Status: 400, Message: "empty user"}
	}

	c.mu.Lock()
	if cached, ok := c.users[key]; ok {
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	user, err := c.resolveUserUncached(ParseUser(ident))
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.users[key] = user
	c.mu.Unlock()
	return user, nil
}

func (c *Client) resolveUserUncached(ident string) (*User, error) {
	if isDigits(ident) {
		var u User
		if err := c.getJSON("/users/"+ident, nil, &u); err != nil {
			return nil, err
		}
		return &u, nil
	}

	var byName []User
	if err := c.getJSON("/users", url.Values{"username": {ident}}, &byName); err != nil {
		return nil, err
	}
	if len(byName) == 1 {
		return &byName[0], nil
	}

	var hits []User
	if err := c.getJSON("/users", url.Values{"search": {ident}, "per_page": {"20"}}, &hits); err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, &APIError{Status: 404, Message: fmt.Sprintf("no user matches %q", ident)}
	}

	var exact []User
	for _, h := range hits {
		if strings.EqualFold(h.Username, ident) || strings.EqualFold(h.Email, ident) {
			exact = append(exact, h)
		}
	}
	if len(exact) == 1 {
		return &exact[0], nil
	}
	if len(hits) == 1 {
		return &hits[0], nil
	}

	var names []string
	for i, h := range hits {
		if i == 8 {
			break
		}
		names = append(names, fmt.Sprintf("%s (%s)", h.Username, h.Name))
	}
	return nil, &APIError{Status: 300, Message: fmt.Sprintf("%q is ambiguous: %s", ident, strings.Join(names, ", "))}
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ResolveTarget finds a project or group by path, URL or bare name.
func (c *Client) ResolveTarget(path string) *Target {
	clean := strings.Trim(strings.TrimSpace(path), "/")
	key := strings.ToLower(clean)

	c.mu.Lock()
	if cached, ok := c.targets[key]; ok {
		c.mu.Unlock()
		return cached
	}
	c.mu.Unlock()

	t := c.resolveTargetUncached(clean)
	c.mu.Lock()
	c.targets[key] = t
	c.mu.Unlock()
	return t
}

func (c *Client) resolveTargetUncached(path string) *Target {
	t := &Target{Raw: path}
	if path == "" {
		t.Status = "empty"
		return t
	}

	if strings.Contains(path, "/") {
		for _, probe := range []struct{ kind, endpoint string }{
			{"project", "/projects/"},
			{"group", "/groups/"},
		} {
			var obj struct {
				ID                int    `json:"id"`
				PathWithNamespace string `json:"path_with_namespace"`
				FullPath          string `json:"full_path"`
			}
			err := c.getJSON(probe.endpoint+url.PathEscape(path), nil, &obj)
			if err != nil {
				var apiErr *APIError
				if ok := asAPIError(err, &apiErr); ok && (apiErr.Status == 404 || apiErr.Status == 403) {
					continue
				}
				t.Status = err.Error()
				return t
			}
			t.Kind, t.ID, t.Status = probe.kind, obj.ID, "ok"
			t.FullPath = obj.PathWithNamespace
			if t.FullPath == "" {
				t.FullPath = obj.FullPath
			}
			return t
		}
		t.Status = "not found"
		return t
	}

	var cands []Candidate

	var projects []struct {
		ID                int    `json:"id"`
		Path              string `json:"path"`
		PathWithNamespace string `json:"path_with_namespace"`
	}
	if err := c.getJSON("/projects", url.Values{
		"search": {path}, "simple": {"true"}, "per_page": {"20"},
	}, &projects); err == nil {
		for _, p := range projects {
			cands = append(cands, Candidate{"project", p.ID, p.PathWithNamespace, p.Path})
		}
	}

	var groups []struct {
		ID       int    `json:"id"`
		Path     string `json:"path"`
		FullPath string `json:"full_path"`
	}
	if err := c.getJSON("/groups", url.Values{"search": {path}, "per_page": {"20"}}, &groups); err == nil {
		for _, g := range groups {
			cands = append(cands, Candidate{"group", g.ID, g.FullPath, g.Path})
		}
	}

	var exact []Candidate
	for _, c := range cands {
		if strings.EqualFold(c.Name, path) {
			exact = append(exact, c)
		}
	}
	pool := exact
	if len(pool) == 0 {
		pool = cands
	}

	switch len(pool) {
	case 0:
		t.Status = "not found"
	case 1:
		t.Kind, t.ID, t.FullPath, t.Status = pool[0].Kind, pool[0].ID, pool[0].FullPath, "ok"
	default:
		t.Candidates = pool
		t.Status = fmt.Sprintf("ambiguous (%d)", len(pool))
	}
	return t
}

func asAPIError(err error, out **APIError) bool {
	if e, ok := err.(*APIError); ok {
		*out = e
		return true
	}
	return false
}

// Grant adds or updates a membership. Re-granting an existing member is not an
// error: the level is compared and a PUT issued when it differs.
func (c *Client) Grant(t *Target, userID, level int, expiresAt string) string {
	base := "/groups/"
	if t.Kind == "project" {
		base = "/projects/"
	}
	base = fmt.Sprintf("%s%d/members", base, t.ID)

	body := map[string]any{"user_id": userID, "access_level": level}
	if expiresAt != "" {
		body["expires_at"] = expiresAt
	}

	_, err := c.do(http.MethodPost, base, nil, body)
	if err == nil {
		return "added as " + LevelName(level)
	}

	var apiErr *APIError
	exists := false
	if asAPIError(err, &apiErr) {
		exists = apiErr.Status == 409 || strings.Contains(strings.ToLower(apiErr.Message), "already exists")
	}
	if !exists {
		return "FAIL " + err.Error()
	}

	current := 0
	var member struct {
		AccessLevel int `json:"access_level"`
	}
	if err := c.getJSON(fmt.Sprintf("%s/%d", base, userID), nil, &member); err == nil {
		current = member.AccessLevel
	}
	if current == level && expiresAt == "" {
		return "already " + LevelName(level)
	}

	if _, err := c.do(http.MethodPut, fmt.Sprintf("%s/%d", base, userID), nil, body); err != nil {
		if asAPIError(err, &apiErr) &&
			(apiErr.Status == 403 || strings.Contains(strings.ToLower(apiErr.Message), "inherited")) {
			return "FAIL " + apiErr.Message + " (inherited from a parent group; grant there)"
		}
		return "FAIL " + err.Error()
	}
	if current == level {
		return LevelName(level) + ", expiry updated"
	}
	return fmt.Sprintf("updated %s -> %s", LevelName(current), LevelName(level))
}
