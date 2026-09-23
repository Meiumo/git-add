# git-add

Grant GitLab users access to projects and groups from the terminal. One command for a batch of people across a batch of repositories, or an interactive form when the list is not yet in your head.

A single static binary. No runtime, no interpreter, no `node_modules`: `CGO_ENABLED=0` builds for macOS, Linux and Windows out of the same source.

## Install

```
make build && cp git-add ~/bin/
```

Or fetch a binary from `make release`, which writes `dist/git-add-<os>-<arch>` for all five supported targets.

First run configures the instance:

```
git-add --setup
```

The token goes into the OS secret store: macOS keychain, Windows Credential Manager, or libsecret on Linux. The config file keeps only the URL and the CA bundle path, so a dotfiles repo never carries a credential. A personal access token with the `api` scope is required; `read_api` is not enough because membership is a write.

## Usage

```
git-add ivanov dso/cicd-supply/m
git-add ivanov,petrov "dso/cicd-supply/m, k8s-values/d"
git-add -u ivanov -u petrov -t dso/cicd-supply -t dso -r m
git-add -n ivanov dso                     dry run, changes nothing
git-add -e 2026-12-31 contractor dso/x    membership expires on that date
git-add                                   interactive form
```

Every user in the list gets every target in the list, so two users and three repositories is six grants. Duplicates are folded, and when the same target appears twice with different roles the last one wins.

### Targets

A target is a project or a group, written any way you happen to have it:

| Input | Resolves to |
| --- | --- |
| `https://gitlab.corp/dso/cicd-supply` | `dso/cicd-supply` |
| `https://gitlab.corp/dso/cicd-supply/-/tree/main/x` | `dso/cicd-supply` |
| `git@gitlab.corp:dso/cicd-supply.git` | `dso/cicd-supply` |
| `dso/cicd-supply` | exact path |
| `cicd-supply` | searched by name, both projects and groups |
| `dso` | the group, if that is what matches |

A bare name that matches more than one thing is never guessed: the CLI prints the candidates and skips the row, the form opens a picker.

### Roles

Appended to the target as `/r`, or written explicitly as `:maintainer`.

| Key | Role | Level |
| --- | --- | --- |
| `g` | guest | 10 |
| `r` | reporter | 20 |
| `d` | developer | 30 |
| `m` | maintainer | 40 |
| `o` | owner | 50 |

Without a suffix the default from `-r` applies, which is `developer` unless you change it.

## Interactive form

Run `git-add` with no arguments, or `-i` to open it pre-filled.

```
 git-add   gitlab.corp   4 grant(s)   DRY RUN

users
╭──────────────────────────────────────────────────────────────╮
│ user                    status                               │
│ > ivanov                Ivan Ivanov (id 12)                  │
│   petrov                Petr Petrov (id 34)                  │
╰──────────────────────────────────────────────────────────────╯

targets
╭──────────────────────────────────────────────────────────────╮
│ target                  g  r  d  m  o   status               │
│   dso/cicd-supply      [ ][ ][ ][m][ ]  project  ok          │
│ > dso                  [ ][ ][d][ ][ ]  group    ok          │
╰──────────────────────────────────────────────────────────────╯
```

| Key | Action |
| --- | --- |
| `enter` | edit the current row |
| `a` | add a row below and start editing |
| `D` | delete the row |
| `tab` | switch between users and targets |
| arrows | move; left/right picks the role column |
| `g` `r` `d` `m` `o` | set the role directly |
| `R` | resolve everything against the API |
| `ctrl+a` | apply |
| `ctrl+d` | toggle dry run |
| `q` | quit |

The form is a Bubbletea program: state in, view out, network calls dispatched as commands so typing never blocks on GitLab.

## Behaviour worth knowing

Re-granting is safe. An existing member is detected, the current level compared, and a `PUT` issued instead of a failing `POST`, reported as `updated developer -> maintainer`. An unchanged level reports `already maintainer` and writes nothing.

Membership inherited from a parent group cannot be lowered on a single project; GitLab rejects it and the tool says so, pointing at the group.

A failure on one pair never stops the rest. Unresolvable users and targets are skipped with a reason, and the exit code is non-zero when anything failed, which makes the tool usable from a script.

## Corporate TLS

Go reads the OS trust store on every platform, so a corporate root installed by MDM normally works with no setup at all. When it does not, usually a root delivered by group policy on Windows or an explicit PEM handed over by IT:

```
git-add --fix-ca
```

This exports the OS store into a PEM bundle next to the config and pins it. Verification stays on. `--setup` runs the same repair automatically when it meets an unknown issuer.

As a last resort `"insecure": true` in the config disables verification. Reach for it only when the root genuinely cannot be obtained.

## Configuration

`~/.config/git-add/config.json`, or `%APPDATA%\git-add\config.json` on Windows:

```json
{
  "url": "https://gitlab.corp.tld",
  "ca_file": "/Users/you/.config/git-add/ca-bundle.pem"
}
```

Environment variables override the file: `GITLAB_URL`, `GITLAB_TOKEN`, `GITLAB_CA_FILE`, `GITLAB_INSECURE=1`. This is what CI should use, so nothing is stored on disk.

`git-add --logout` removes the stored token, `git-add --whoami` verifies it.

## Development

```
make test      # unit tests against an httptest GitLab
make vet
make release   # static binaries for five platforms
```

The test suite runs a fake GitLab over `httptest`, so it covers the real client, caching and grant logic without network access or credentials.

## Layout

| Path | Contents |
| --- | --- |
| `cmd/git-add` | flag parsing, setup wizard, non-interactive output |
| `internal/gitlab` | REST client, resolution, roles, the grant plan |
| `internal/ui` | Bubbletea form, Lipgloss styles, result report |
| `internal/config` | config file, keyring, environment precedence |
| `internal/trust` | OS certificate store export, per-platform |
