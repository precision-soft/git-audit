# Git Audit

[![Go >= 1.24](https://img.shields.io/badge/go-%3E%3D1.24-00ADD8)](https://go.dev/)
[![License MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Go CLI that **audits GitHub releases** and **syncs release bodies from each repo's `CHANGELOG.md`** (changelog is the source of truth). For every tag it produces per-level status (integrity, distribution, changelog, diff, presentation), supports accepted exceptions with expiry, and manages local clones automatically.

Works with **any GitHub repo** — public or private, yours or someone else's. Ships with a default project list tuned for `precision-soft/*` libraries (see [Internal — precision-soft defaults](#internal--precision-soft-defaults)); everything else is driven by `--repo-url`.

**MIT licensed. Fork and modify it as you wish.** Suggestions and PRs are welcomed.

## Features

- **Per-release audit** with five independent checks (see [Audit rules](#audit-rules)).
- **Changelog → release body sync** with dry-run diffs; only writes when you pass `--apply`.
- **Automatic local clones** under `.dev-data/clones/<repo>/` — clones on first use, hard-resets to origin on every subsequent run (scratch space, not a workspace).
- **SSH and HTTPS** clone URLs both work — private repos use whatever your `git` + SSH agent are configured to use.
- **Ad-hoc mode**: audit/sync any repo without adding it to the config via `--repo-url`.
- **Exception list** with `reviewed_until` expiry so accepted warnings stay silent until they lapse.
- **Parallel audits** (4 repos at a time), shared HTTP client with retry + rate-limit tracking.

## Requirements

- Go 1.24+
- `git` on `$PATH`
- GitHub token with `Contents: Read & write` on target repos (via `--token`, config file, or `GITHUB_TOKEN` env)
- For private repos: SSH key configured in your git agent (same setup `git clone git@github.com:...` uses)

## Installation

```bash
go install github.com/precision-soft/git-audit@latest
```

Or build from source:

```bash
git clone https://github.com/precision-soft/git-audit
cd git-audit
go build -o git-audit ./...
```

## Configuration

Two things can come from env vars or an `.env` / `.env.local` file next to the binary:

```bash
# .env.local (gitignored)
GITHUB_TOKEN=ghp_xxx
```

Precedence for the token: `--token` flag > `github.token` in the melody config > `GITHUB_TOKEN` env.

## Commands

| Command      | Purpose                                                                                       |
|--------------|-----------------------------------------------------------------------------------------------|
| `audit`      | Cross-check tags, GitHub releases, Packagist versions, changelog entries, commit diffs.       |
| `sync`       | Push local `CHANGELOG.md` sections into the matching GitHub release body. Dry-run by default. |
| `exceptions` | List accepted warnings from `exceptions.json`, grouped by project, with expiry status.        |

### audit

```bash
./git-audit audit [--token TOKEN] [--repo NAME] [--repo-url URL] [--fail-on-warning] [--exceptions PATH]
```

| Flag                | Meaning                                                                                                                                                                                                                                                                                              |
|---------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--token TOKEN`     | GitHub token; overrides melody `github.token` / `GITHUB_TOKEN` env.                                                                                                                                                                                                                                  |
| `--repo NAME`       | Filter to a single repo by name (e.g. `doctrine-type`).                                                                                                                                                                                                                                              |
| `--repo-url URL`    | GitHub URL for an ad-hoc repo not in the built-in project list (required when `--repo` doesn't match a known project). Accepts HTTPS (`https://github.com/org/repo`) or SSH (`git@github.com:org/repo.git`). Ad-hoc repos default to no Packagist, a single `CHANGELOG.md`, and `GoSubmodule=false`. |
| `--fail-on-warning` | Exit 1 on warnings, not only on failures.                                                                                                                                                                                                                                                            |
| `--exceptions PATH` | JSON file of accepted issues (default `exceptions.json`). TTY prompts to add unacknowledged warnings.                                                                                                                                                                                                |

#### Audit rules

| Level          | Rule                                                                                                                                                                                                       |
|----------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `integrity`    | Tag ↔ release 1:1; release is not draft/prerelease; `target_commitish` matches tag commit.                                                                                                                 |
| `distribution` | Packagist has a version with a matching commit reference (skipped when no Packagist).                                                                                                                      |
| `changelog`    | Entry exists; heading matches `## [vX.Y.Z] - YYYY-MM-DD - <Title>`; `[vX.Y.Z]: .../compare/...` link; heading title matches the release title `<Summary>`; release body overlap ≥60% with changelog entry. |
| `diff`         | Previous-tag compare has ≥1 commit; non-trivial diffs must have release notes.                                                                                                                             |
| `presentation` | Title matches `<Name> vX.Y.Z - <Summary>`; only whitelisted `## ` sections.                                                                                                                                |

Projects are audited in parallel (4 at a time). The summary block aggregates per-level status; per-repo issue blocks follow.

### sync

```bash
./git-audit sync [--token TOKEN] [--repo NAME] [--repo-url URL] [--apply]
```

**Default is dry-run** — reports `would-update` per tag and prints a unified diff (`+ added / - removed`) below the summary table. Pass `--apply` to actually PATCH release bodies.

| Flag             | Meaning                                                                                   |
|------------------|-------------------------------------------------------------------------------------------|
| `--token TOKEN`  | GitHub token (required to list releases; also to PATCH when `--apply`).                   |
| `--repo NAME`    | Restrict to one project.                                                                  |
| `--repo-url URL` | GitHub URL for an ad-hoc repo. Same format as `audit` — HTTPS or SSH.                     |
| `--apply`        | Actually PATCH release bodies. Without this flag, `sync` runs as dry-run (no API writes). |

**Local clones are managed automatically.** Before reading the changelog for each repo, `sync` ensures `.dev-data/clones/<repo>/` exists and is in sync with origin:

- If missing → `git clone <GithubUrl> .dev-data/clones/<repo>`.
- If present → `git fetch --tags --prune && git checkout <default-branch> && git reset --hard origin/<default-branch> && git clean -fdx`. **Any local changes in that clone are discarded** — these clones are treated as scratch space.

For unknown repos, the URL comes from `--repo-url`. For known repos, it comes from `config/project/project.go`. Ad-hoc (unknown) repos default to a single `CHANGELOG.md` at the root.

For each `## [vX.Y.Z]` section in the changelog:

1. Extracts the entry body; normalizes `### Security` → `## Fixed`, `### Removed` → `## Changed`, promotes other `### ` headings to `## `.
2. Compares (CRLF-normalized, blank-run-collapsed, trailing-whitespace-stripped) against the current GitHub release body.
3. If different and not dry-run, `PATCH /repos/:owner/:repo/releases/:id` with the folded body.

### exceptions

```bash
./git-audit exceptions [--exceptions PATH]
```

Lists entries grouped by repo — `version`, `level`, `issue`, `reviewed_until` (with `(expired)` past due). Expired entries are ignored by `audit` so the warnings resurface.

`exceptions.json` entries accept two shapes:

```json
{
    "precision-soft/doctrine-type": {
        "v1.0.0": {
            "presentation": [
                "title project name \"DoctrineType\" does not match expected \"Doctrine Type\"",
                {
                    "issue": "non-standard section: ## Notes",
                    "reviewed_until": "2026-07-01"
                }
            ]
        }
    }
}
```

## Working with any repo

Two supported modes for pointing git-audit at a repo:

### 1. Ad-hoc (no config change)

```bash
# Public repo — HTTPS
./git-audit audit --repo my-lib --repo-url https://github.com/my-org/my-lib
./git-audit sync  --repo my-lib --repo-url https://github.com/my-org/my-lib --apply

# Private repo — SSH
./git-audit audit --repo secret-svc --repo-url git@github.com:my-org/secret-svc.git
./git-audit sync  --repo secret-svc --repo-url git@github.com:my-org/secret-svc.git --apply
```

Ad-hoc repos default to: no Packagist check, a single `CHANGELOG.md` at the repo root, and no Go submodule filtering.

### 2. Add to the default list (persistent)

Fork, then edit `config/project/project.go` and add an entry:

```go
{
Name:         "My Service",
GithubUrl:    "git@github.com:my-org/my-service.git", // or HTTPS
PackagistUrl: "", // optional
GoSubmodule:  false,
ChangelogPaths: []string{"CHANGELOG.md"}, // omit for the default
},
```

The `Name` field is used in presentation checks (title must match `<Name> vX.Y.Z - <Summary>`); if omitted, git-audit derives it from the repo slug.

## Local clones

Both `sync` and the bulk-clone script keep clones under `.dev-data/clones/<repo>/` (gitignored). Each command that needs a local clone:

- clones the repo from its GitHub URL if the folder is missing;
- otherwise fetches and **hard-resets the working tree to origin** (default branch), then `git clean -fdx`. Local edits in those clones are discarded — they're scratch space, not a workspace.

SSH URLs work the same as HTTPS — `git` handles the auth. Make sure your SSH agent has the key loaded (`ssh-add -l`) before running.

Bulk-clone all known projects up front (optional, for warm cache):

```bash
.dev/clone-repos.sh
```

## Output formats

Inherits melody's standard flags: `--format=json`, `--format=table` (default), `--limit`, `--offset`, debug flags. When available, rate-limit info is printed as a summary line on tables.

## Resilience

`service/http_retry.go` centralizes HTTP behavior: 30s timeout, 3 attempts with exponential backoff (500ms initial) on 5xx/429, bodies rewound via `GetBody`. Both GitHub and Packagist use this client. Rate-limit tracking keeps **minimum** `Remaining` seen across calls (= peak usage), not last-wins.

## Layout

```
cli/                 # melody commands: audit, sync, exceptions + resolver
config/              # melody config wiring
config/project/      # centralized project list (GithubUrl, PackagistUrl, etc.)
service/             # GitHub + Packagist clients, shared HTTP layer, local-clone helper
types/               # Status, LevelStatus, ReleaseAudit, ProjectAudit
.dev/                # dev-container wiring (Dockerfile, entrypoint, clone-repos.sh)
.dev-data/clones/    # auto-managed local clones (gitignored, scratch space)
main.go              # melody runtime bootstrap
```

## Tests

```bash
go test ./...
```

Covers semver comparison, overlap ratio, changelog entry extraction, title regex, `sync` folding/canonicalization, GitHub URL parsing (HTTPS + SSH).

## Code style

Go code here follows a yoda-comparison house style (`nil == err`, `false == flag`, `"" == token`), descriptive variable names (`configuration`, `repository`, `command`), `/** */` doc comments. Keep new code consistent.

---

## Internal — precision-soft defaults

The built-in project list in `config/project/project.go` is tuned for the `precision-soft/*` open-source libraries. If you're working inside that org (or forking and swapping the list for your own), these are the copy-paste commands:

```bash
# All projects
./git-audit audit [--token TOKEN] [--fail-on-warning] [--exceptions PATH]
./git-audit sync  [--token TOKEN] [--apply]

# doctrine-type
./git-audit audit --repo doctrine-type [--token TOKEN] [--fail-on-warning]
./git-audit sync  --repo doctrine-type [--token TOKEN] [--apply]

# doctrine-utility
./git-audit audit --repo doctrine-utility [--token TOKEN] [--fail-on-warning]
./git-audit sync  --repo doctrine-utility [--token TOKEN] [--apply]

# symfony-console
./git-audit audit --repo symfony-console [--token TOKEN] [--fail-on-warning]
./git-audit sync  --repo symfony-console [--token TOKEN] [--apply]

# symfony-doctrine-audit
./git-audit audit --repo symfony-doctrine-audit [--token TOKEN] [--fail-on-warning]
./git-audit sync  --repo symfony-doctrine-audit [--token TOKEN] [--apply]

# symfony-doctrine-encrypt
./git-audit audit --repo symfony-doctrine-encrypt [--token TOKEN] [--fail-on-warning]
./git-audit sync  --repo symfony-doctrine-encrypt [--token TOKEN] [--apply]

# symfony-json-form
./git-audit audit --repo symfony-json-form [--token TOKEN] [--fail-on-warning]
./git-audit sync  --repo symfony-json-form [--token TOKEN] [--apply]

# symfony-phpunit
./git-audit audit --repo symfony-phpunit [--token TOKEN] [--fail-on-warning]
./git-audit sync  --repo symfony-phpunit [--token TOKEN] [--apply]

# melody (resolves CHANGELOG.md + v2/CHANGELOG.md + v3/CHANGELOG.md)
./git-audit audit --repo melody [--token TOKEN] [--fail-on-warning]
./git-audit sync  --repo melody [--token TOKEN] [--apply]

# git-audit (self-audit)
./git-audit audit --repo git-audit [--token TOKEN] [--fail-on-warning]
./git-audit sync  --repo git-audit [--token TOKEN] [--apply]
```

Special-case config for melody (three changelogs, one per major version) and `GoSubmodule=true` (filters `integrations/*/vX.Y.Z` tags from core auditing) lives in `config/project/project.go` — use it as a template if your own project has a similar layout.
