// Command git-add grants GitLab users access to projects and groups.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/meiumo/gitadd/internal/config"
	"github.com/meiumo/gitadd/internal/gitlab"
	"github.com/meiumo/gitadd/internal/trust"
	"github.com/meiumo/gitadd/internal/ui"
)

const version = "0.3.0"

const usage = `git-add - grant GitLab users access to projects and groups

usage:
  git-add [flags] [users] [targets...]

examples:
  git-add ivanov dso/cicd-supply/m
  git-add ivanov,petrov "dso/cicd-supply/m, k8s-values/d"
  git-add -u ivanov -u petrov -t dso/cicd-supply -t dso -r m
  git-add -n ivanov dso                  dry run, changes nothing
  git-add                                interactive form

targets may be URLs, group/sub/repo paths, or bare names resolved by search.
roles: g guest, r reporter, d developer, m maintainer, o owner.
every user gets every target, so 2 users x 3 targets is 6 grants.

flags:
`

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	os.Exit(run())
}

func run() int {
	var users, targets stringList
	fs := flag.NewFlagSet("git-add", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fs.PrintDefaults()
	}
	fs.Var(&users, "u", "add a user (repeatable)")
	fs.Var(&targets, "t", "add a target (repeatable)")
	roleFlag := fs.String("r", "d", "default role: g|r|d|m|o")
	expires := fs.String("e", "", "membership expiry date (YYYY-MM-DD)")
	interactive := fs.Bool("i", false, "force the interactive form")
	dryRun := fs.Bool("n", false, "resolve everything, change nothing")
	quiet := fs.Bool("q", false, "only report failures")
	doSetup := fs.Bool("setup", false, "configure URL and token")
	doWhoami := fs.Bool("whoami", false, "verify the token")
	doFixCA := fs.Bool("fix-ca", false, "rebuild the CA bundle from the OS trust store")
	doLogout := fs.Bool("logout", false, "remove the stored token")
	showVersion := fs.Bool("version", false, "print the version")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Printf("git-add %s\n", version)
		return 0
	}

	role, ok := gitlab.ParseRole(*roleFlag)
	if !ok {
		fmt.Fprintf(os.Stderr, "git-add: unknown role %q; use g|r|d|m|o\n", *roleFlag)
		return 2
	}

	switch {
	case *doSetup:
		return runSetup()
	case *doLogout:
		return runLogout()
	case *doFixCA:
		return runFixCA()
	}

	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotConfigured) && isTTY() {
			if code := runSetup(); code != 0 {
				return code
			}
			if cfg, err = config.Load(); err != nil {
				fmt.Fprintf(os.Stderr, "git-add: %v\n", err)
				return 1
			}
		} else {
			fmt.Fprintf(os.Stderr, "git-add: %v\n", err)
			return 1
		}
	}

	client := gitlab.New(cfg)

	if *doWhoami {
		user, err := client.Whoami()
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %v\n", err)
			if hint := config.Explain(err); hint != "" {
				fmt.Fprintln(os.Stderr, hint)
			}
			return 1
		}
		fmt.Printf("%s (id %d)\n", user.Username, user.ID)
		return 0
	}

	userEntries := gitlab.SplitList(append([]string{}, users...))
	targetEntries := gitlab.SplitList(append([]string{}, targets...))

	// Positional form: first argument is the user list, the rest are targets.
	if rest := fs.Args(); len(rest) > 0 {
		userEntries = append(userEntries, gitlab.SplitList(rest[:1])...)
		if len(rest) > 1 {
			targetEntries = append(targetEntries, gitlab.SplitList(rest[1:])...)
		}
	}

	plan := gitlab.BuildPlan(userEntries, targetEntries, role)
	plan.ExpiresAt = *expires

	if *interactive || len(plan.Users) == 0 || len(plan.Targets) == 0 {
		if !isTTY() {
			fmt.Fprintln(os.Stderr, "git-add: no users or targets given and stdin is not a terminal")
			return 2
		}
		return runForm(client, plan, *dryRun)
	}
	return runCLI(client, plan, *dryRun, *quiet)
}

func runForm(client *gitlab.Client, plan *gitlab.Plan, dryRun bool) int {
	model := ui.New(client, plan, dryRun)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git-add: %v\n", err)
		return 1
	}
	outcomes := final.(ui.Model).Outcomes()
	ui.Report(os.Stdout, outcomes, dryRun)
	for _, o := range outcomes {
		if o.Failed() {
			return 1
		}
	}
	return 0
}

func runCLI(client *gitlab.Client, plan *gitlab.Plan, dryRun, quiet bool) int {
	plan.ResolveAll(client, nil)

	out := os.Stdout
	problems := 0
	if !quiet {
		problems = ui.PlanReport(out, plan)
	} else {
		for _, u := range plan.Users {
			if !u.Resolved() {
				problems++
			}
		}
		for _, t := range plan.Targets {
			if !t.Resolved() {
				problems++
			}
		}
	}

	usableUsers, usableTargets := 0, 0
	for _, u := range plan.Users {
		if u.Resolved() {
			usableUsers++
		}
	}
	for _, t := range plan.Targets {
		if t.Resolved() {
			usableTargets++
		}
	}
	if usableUsers == 0 || usableTargets == 0 {
		fmt.Fprintln(os.Stderr, "git-add: nothing to do")
		return 1
	}

	if !quiet {
		fmt.Fprintln(out)
	}
	plan.Apply(client, dryRun, nil)
	ui.Report(out, plan.Outcomes, dryRun)

	if plan.Failures() > 0 || problems > 0 {
		return 1
	}
	return 0
}

func runSetup() int {
	file := config.LoadFile()
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("git-add setup   (secrets: %s)\n", config.KeyringBackend())

	prompt := "GitLab URL (https://gitlab.corp.tld): "
	if file.URL != "" {
		prompt = fmt.Sprintf("GitLab URL [%s]: ", file.URL)
	}
	fmt.Print(prompt)
	line, _ := reader.ReadString('\n')
	url := strings.TrimSpace(line)
	if url == "" {
		url = file.URL
	}
	if url == "" {
		fmt.Fprintln(os.Stderr, "git-add: no URL given")
		return 1
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	url = strings.TrimRight(url, "/")

	token, err := readSecret("Personal access token (scope: api): ")
	if err != nil || token == "" {
		fmt.Fprintln(os.Stderr, "git-add: no token given")
		return 1
	}

	file.URL = url
	file.Token = ""
	if err := config.SetToken(url, token); err != nil {
		file.Token = token
		fmt.Printf("keyring unavailable (%v); token written to the config file\n", err)
	}
	if err := config.SaveFile(file); err != nil {
		fmt.Fprintf(os.Stderr, "git-add: %v\n", err)
		return 1
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git-add: %v\n", err)
		return 1
	}
	client := gitlab.New(cfg)
	user, err := client.Whoami()
	if err != nil && gitlab.IsCertError(err) {
		fmt.Printf("unknown certificate issuer; exporting roots from the %s\n", trust.StoreName())
		if code := runFixCA(); code == 0 {
			return 0
		}
	}
	if err != nil {
		fmt.Printf("warning: token check failed: %v\n", err)
		if hint := config.Explain(err); hint != "" {
			fmt.Println(hint)
		}
		return 0
	}
	fmt.Printf("authenticated as %s (id %d)\n", user.Username, user.ID)
	return 0
}

func runFixCA() int {
	count, err := trust.Export(config.CABundle())
	if err != nil {
		fmt.Fprintf(os.Stderr, "git-add: %v\n", err)
		return 1
	}
	file := config.LoadFile()
	file.CAFile = config.CABundle()
	file.Insecure = false
	if err := config.SaveFile(file); err != nil {
		fmt.Fprintf(os.Stderr, "git-add: %v\n", err)
		return 1
	}
	fmt.Printf("CA bundle written: %s (%d certificates)\n", config.CABundle(), count)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git-add: %v\n", err)
		return 1
	}
	user, err := gitlab.New(cfg).Whoami()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL %v\n", err)
		return 1
	}
	fmt.Printf("authenticated as %s (id %d)\n", user.Username, user.ID)
	return 0
}

func runLogout() int {
	file := config.LoadFile()
	if err := config.DeleteToken(file.URL); err != nil {
		fmt.Printf("keyring: %v\n", err)
	}
	if file.Token != "" {
		file.Token = ""
		_ = config.SaveFile(file)
	}
	fmt.Printf("token removed from the %s\n", config.KeyringBackend())
	return 0
}

func readSecret(prompt string) (string, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		raw, err := term.ReadPassword(fd)
		fmt.Println()
		return strings.TrimSpace(string(raw)), err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line), err
}

func isTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
