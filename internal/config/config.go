// Package config resolves the instance URL, token and CA bundle.
//
// The token lives in the OS secret store (macOS keychain, Windows Credential
// Manager, libsecret on Linux). The config file holds only the URL and the CA
// path, so a shared dotfiles repo never carries a credential.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/zalando/go-keyring"

	"github.com/meiumo/gitadd/internal/gitlab"
	"github.com/meiumo/gitadd/internal/trust"
)

const KeyringService = "git-add-gitlab"

// ErrNotConfigured is returned when no URL or token can be found.
var ErrNotConfigured = errors.New("not configured")

type File struct {
	URL      string `json:"url,omitempty"`
	Token    string `json:"token,omitempty"`
	CAFile   string `json:"ca_file,omitempty"`
	Insecure bool   `json:"insecure,omitempty"`
}

func Dir() string {
	if runtime.GOOS == "windows" {
		if base := os.Getenv("APPDATA"); base != "" {
			return filepath.Join(base, "git-add")
		}
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "git-add")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".git-add"
	}
	return filepath.Join(home, ".config", "git-add")
}

func Path() string     { return filepath.Join(Dir(), "config.json") }
func CABundle() string { return filepath.Join(Dir(), "ca-bundle.pem") }

func LoadFile() File {
	var f File
	raw, err := os.ReadFile(Path())
	if err != nil {
		return f
	}
	_ = json.Unmarshal(raw, &f)
	return f
}

func SaveFile(f File) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := Path() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, Path())
}

// KeyringBackend names the secret store for display.
func KeyringBackend() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS keychain"
	case "windows":
		return "Windows Credential Manager"
	default:
		return "libsecret"
	}
}

func account(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		return u.Host
	}
	if rawURL != "" {
		return rawURL
	}
	return "default"
}

func GetToken(rawURL string) string {
	if tok, err := keyring.Get(KeyringService, account(rawURL)); err == nil && tok != "" {
		return tok
	}
	if tok, err := keyring.Get(KeyringService, "default"); err == nil {
		return tok
	}
	return ""
}

func SetToken(rawURL, token string) error {
	return keyring.Set(KeyringService, account(rawURL), token)
}

func DeleteToken(rawURL string) error {
	return keyring.Delete(KeyringService, account(rawURL))
}

// Load assembles the config from environment, file and keyring, in that order
// of precedence, so CI can override everything without touching disk.
func Load() (gitlab.Config, error) {
	file := LoadFile()

	cfg := gitlab.Config{
		URL:      firstNonEmpty(os.Getenv("GITLAB_URL"), file.URL),
		Token:    firstNonEmpty(os.Getenv("GITLAB_TOKEN"), file.Token),
		CAFile:   firstNonEmpty(os.Getenv("GITLAB_CA_FILE"), file.CAFile),
		Insecure: file.Insecure || os.Getenv("GITLAB_INSECURE") == "1",
	}
	cfg.URL = strings.TrimRight(strings.TrimSpace(cfg.URL), "/")

	if cfg.CAFile == "" {
		if _, err := os.Stat(CABundle()); err == nil {
			cfg.CAFile = CABundle()
		}
	}
	if cfg.Token == "" && cfg.URL != "" {
		cfg.Token = GetToken(cfg.URL)
	}
	if cfg.URL == "" || cfg.Token == "" {
		return cfg, fmt.Errorf("%w: run `git-add --setup`, or set GITLAB_URL and GITLAB_TOKEN, or write %s",
			ErrNotConfigured, Path())
	}

	pool, err := trust.Pool(cfg.CAFile)
	if err == nil {
		cfg.CAPool = pool
	}
	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// Explain turns a transport failure into an actionable sentence.
func Explain(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "x509") || strings.Contains(msg, "unknown authority"):
		return "the corporate root is not trusted; run `git-add --fix-ca`, or point ca_file at the CA PEM"
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "lookup"):
		return "DNS did not resolve the host; the VPN is probably down"
	case strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized"):
		return "the token was rejected; it may be expired or lack the `api` scope"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "the instance did not answer in time; check the VPN or the host"
	}
	return ""
}
