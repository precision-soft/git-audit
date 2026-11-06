# Git Audit

[![ci](https://github.com/precision-soft/git-audit/actions/workflows/ci.yml/badge.svg)](https://github.com/precision-soft/git-audit/actions/workflows/ci.yml)
[![Go >= 1.26](https://img.shields.io/badge/go-%3E%3D1.26-00ADD8)](https://go.dev/)
[![License MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Go CLI that **audits GitHub releases** and **syncs release bodies from each repo's `CHANGELOG.md`** (changelog is the source of truth). For every tag it produces per-level status (integrity, distribution, changelog, diff, presentation), supports accepted exceptions with expiry, and manages local clones automatically.

Works with **any GitHub repo** — public or private, yours or someone else's. Ships with a default project list tuned for `precision-soft/*` libraries (see [Internal — precision-soft defaults](#internal--precision-soft-defaults)); everything else is driven by `--repo-url`.

**MIT licensed. Fork and modify it as you wish.** Suggestions and PRs are welcomed.

## Features

- **Per-release audit** with six independent checks (see [Audit rules](#audit-rules)).
- **Changelog → release body sync** with dry-run diffs; only writes when you pass `--apply`.
- **Automatic local clones** under `.dev-data/clones/<repo>/` — clones on first use, hard-resets to origin on every subsequent run (scratch space, not a workspace).
- **SSH and HTTPS** clone URLs both work — private repos use whatever your `git` + SSH agent are configured to use.
- **Ad-hoc mode**: audit/sync any repo without adding it to the config via `--repo-url`.
- **Exception list** with `reviewed_until` expiry so accepted warnings stay silent until they lapse.
- **Parallel audits** (4 repos at a time), shared HTTP client with retry + rate-limit tracking.

## Requirements

- Go 1.26+
- `git` on `$PATH`
- GitHub token with `Contents: Read & write` on target repos (via `--token`, or `GITHUB_TOKEN` in `.env.local`)
- For private repos: SSH key configured in your git agent (same setup `git clone git@github.com:...` uses)

## Installation

```bash
go install github.com/precision-soft/git-audit@latest
```

Or build from source:

```bash
git clone https://github.com/precision-soft/git-audit
cd git-audit
go build -o git-audit .
```

After `go install`, the Go binary directory must be on your `PATH`. The binary reads its configuration from the directory it sits in, and `go install` copies only the binary, so put both the repository's [`.env`](./.env) and the `.env.local` below next to it; that directory must also be writable, because the logs go to `var/` there. Without the `.env` the binary starts as an http server instead of a command line tool:

```bash
cd "$(dirname "$(command -v git-audit)")"
curl -fsSL https://raw.githubusercontent.com/precision-soft/git-audit/main/.env -o .env
```

After a source build, run `./git-audit` from the checkout instead. The dev container runs as root, so once it has run, `var/` and `.dev-data/clones/` in the checkout belong to root: keep using `go run .` inside the container, or take them back with `sudo chown -R "$(id -u):$(id -g)" var .dev-data/clones` before running on the host.

## Configuration

Parameters come from the `.env` / `.env.local` files next to the binary (melody reads no process environment):

```bash
# .env.local (gitignored)
GITHUB_TOKEN=ghp_xxx
# optional: point the client at a proxy or a test double instead of api.github.com / raw.githubusercontent.com
GITHUB_API_BASE=https://api.github.com
GITHUB_RAW_BASE=https://raw.githubusercontent.com
```

To create the token, either:

- generate a [fine-grained personal access token](https://github.com/settings/personal-access-tokens/new) limited to the target repositories, with `Contents: Read & write`, and paste it in `.env.local`, or
- reuse the GitHub CLI login, whose token reaches everything that account can reach. From the directory that holds the `.env` files:

  ```bash
  (umask 077; token="$(gh auth token)" && printf 'GITHUB_TOKEN=%s\n' "${token}" >> .env.local)
  ```

Precedence for the token: `--token` flag > `GITHUB_TOKEN` from `.env.local` / `.env`. The token is optional at boot: `exceptions` runs without one, while `audit` and `sync` stop with `github token required` when neither source provides it.

The token is registered as a **secret** parameter, so `debug:parameters` renders it as `********` instead of in clear text — under both the `github.token` name and the `GITHUB_TOKEN` key melody registers from the `.env` artifacts.

> **Do not build with `-tags melody_env_embedded` from a checkout whose `.env.local` holds a real token.** That tag embeds `.env*` into the binary, `.env.local` included, so the credential ships inside the executable. The tag exists for environments that have no `.env` file next to the binary; give it an `.env.local` written for that purpose.

## Commands

| Command      | Purpose                                                                                       |
|--------------|-----------------------------------------------------------------------------------------------|
| `audit`      | Cross-check tags, GitHub releases, Packagist versions, changelog entries, commit diffs.       |
| `sync`       | Push local `CHANGELOG.md` sections into the matching GitHub release body. Dry-run by default. |
| `exceptions` | List accepted warnings from `exceptions.json`, grouped by project, with expiry status.        |

### audit

```bash
git-audit audit [--token TOKEN] [--repo NAME] [--repo-url URL] [--fail-on-warning] [--exceptions PATH] [--config PATH] [--concurrency N] [--github-annotations] [--cache-dir PATH] [--supply-chain LIST] [--supply-chain-fail]
```

| Flag                   | Meaning                                                                                                                                                                                                                                                                                                                                       |
|------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--token TOKEN`        | GitHub token; overrides `GITHUB_TOKEN` from `.env.local` / `.env`.                                                                                                                                                                                                                                                                            |
| `--repo NAME`          | Filter by repo name; pass a comma-separated list for multiple repos (e.g. `doctrine-type` or `doctrine-type,doctrine-utility`).                                                                                                                                                                                                               |
| `--repo-url URL`       | GitHub URL for an ad-hoc repo not in the built-in project list (required when `--repo` doesn't match a known project; only valid with a single `--repo` value). Accepts HTTPS (`https://github.com/org/repo`) or SSH (`git@github.com:org/repo.git`). Ad-hoc repos default to no Packagist, a single `CHANGELOG.md`, and `GoSubmodule=false`. |
| `--fail-on-warning`    | Exit 1 on warnings, not only on failures.                                                                                                                                                                                                                                                                                                     |
| `--exceptions PATH`    | JSON file of accepted issues (default `exceptions.json`). TTY prompts to add unacknowledged warnings.                                                                                                                                                                                                                                         |
| `--config PATH`        | JSON project manifest `{ "mode": "merge\|replace", "projects": [...] }` (see [External project manifests and CI output](#external-project-manifests-and-ci-output)). |
| `--concurrency N`      | Projects audited at a time, 1 to 32 (default 4).                                                                                                                                                                                                                                                                                              |
| `--github-annotations` | Write one `::warning` / `::error` workflow command per issue to stderr; stdout stays the plain document, so `--format json` remains parseable in a GitHub Actions step.                                                                                                                                                                       |
| `--cache-dir PATH`     | Cache GitHub responses by `ETag` under `PATH` (off unless set; nothing is written to the working directory otherwise). A cache that cannot be read or written is a miss, never a failure.                                                                                                                                                     |
| `--supply-chain LIST`  | Opt-in checks per release: `signed-tags`, `checksums`, `sbom`, `attestations`, or `all`. Findings are warnings on the `supply-chain` level; a project whose calls fail carries the failure on that level and the audit continues.                                                                                                             |
| `--supply-chain-fail`  | Report supply-chain findings as failures instead of warnings.                                                                                                                                                                                                                                                                                 |

#### Audit rules

| Level          | Rule                                                                                                                                                                                                       |
|----------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `integrity`    | Tag ↔ release 1:1; release is not draft/prerelease; `target_commitish` matches tag commit.                                                                                                                 |
| `distribution` | Packagist has a version with a matching commit reference (skipped when no Packagist).                                                                                                                      |
| `changelog`    | Entry exists; heading matches `## [vX.Y.Z] - YYYY-MM-DD - <Title>`; `[vX.Y.Z]: .../compare/...` link; heading title matches the release title `<Summary>`; release body overlap ≥60% with changelog entry. |
| `diff`         | Previous-tag compare has ≥1 commit; non-trivial diffs must have release notes.                                                                                                                             |
| `presentation` | Title matches `<Name> vX.Y.Z - <Summary>`; only whitelisted `## ` sections.                                                                                                                                |

Projects are audited in parallel (`--concurrency`, 4 by default). The summary block aggregates per-level status, `supply-chain` included (`n/a` unless requested); per-repo issue blocks follow.

### sync

```bash
git-audit sync [--token TOKEN] [--repo NAME] [--repo-url URL] [--tag TAG] [--apply]
```

**Default is dry-run** — reports `would-update` per tag and prints a unified diff (`+ added / - removed`) below the summary table. Pass `--apply` to actually PATCH release bodies.

| Flag             | Meaning                                                                                                              |
|------------------|----------------------------------------------------------------------------------------------------------------------|
| `--token TOKEN`  | GitHub token (required to list releases; also to PATCH when `--apply`).                                              |
| `--repo NAME`    | Restrict to one repo, or a comma-separated list of repos (e.g. `doctrine-type` or `doctrine-type,doctrine-utility`). |
| `--repo-url URL` | GitHub URL for an ad-hoc repo. Same format as `audit` — HTTPS or SSH; only valid with a single `--repo` value. With `--apply` it must point at `github.com`, unless `GITHUB_API_BASE` names another host. A URL carrying credentials is refused. |
| `--tag TAG`      | Restrict to one tag (e.g. `v3.4.1`), to try `sync` on a single release first.                                         |
| `--apply`        | Actually PATCH release bodies. Without this flag, `sync` runs as dry-run (no API writes).                            |

**Local clones are managed automatically.** Before reading the changelog for each repo, `sync` ensures `.dev-data/clones/<repo>/`, relative to the working directory, exists and is in sync with origin. A directory there that is not a clone of its own, or a clone of another repository, stops that repo with an error instead of being reset:

- If missing → `git clone <GithubUrl> .dev-data/clones/<repo>`.
- If present → `git fetch --tags --prune && git checkout <default-branch> && git reset --hard origin/<default-branch> && git clean -fdx`. **Any local changes in that clone are discarded** — these clones are treated as scratch space.

For unknown repos, the URL comes from `--repo-url`. For known repos, it comes from `config/project/project.go`. Ad-hoc (unknown) repos default to a single `CHANGELOG.md` at the root.

For each `## [vX.Y.Z]` section in the changelog:

1. Extracts the entry body and promotes its `### ` headings to `## `, keeping the section names.
2. Compares (CRLF-normalized, blank-run-collapsed, trailing-whitespace-stripped) against the current GitHub release body.
3. If different and not dry-run, `PATCH /repos/:owner/:repo/releases/:id` with the folded body.

### exceptions

```bash
git-audit exceptions [--exceptions PATH]
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
git-audit audit --repo my-lib --repo-url https://github.com/my-org/my-lib
git-audit sync  --repo my-lib --repo-url https://github.com/my-org/my-lib --apply

# Private repo — SSH
git-audit audit --repo secret-svc --repo-url git@github.com:my-org/secret-svc.git
git-audit sync  --repo secret-svc --repo-url git@github.com:my-org/secret-svc.git --apply
```

Ad-hoc repos default to: no Packagist check, a single `CHANGELOG.md` at the repo root, and no Go submodule filtering.

### 2. Add to the default list (persistent)

Fork, then edit `config/project/project.go` and add an entry:

```go
{
    Name:           "My Service",
    GithubUrl:      "git@github.com:my-org/my-service.git",
    PackagistUrl:   "",
    GoSubmodule:    false,
    ChangelogPaths: []string{"CHANGELOG.md"},
},
```

`GithubUrl` takes HTTPS or SSH; `PackagistUrl` is optional, and `ChangelogPaths` defaults to `CHANGELOG.md`.

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

Inherits melody's standard flags: `--format=json`, `--format=table` (default), debug flags. `--limit` and `--offset` window the projects in `audit --format=json`; tables and `sync` always return everything. When available, rate-limit info is printed as a summary line on tables.

## Resilience

`service/http_retry.go` centralizes HTTP behavior: 30s timeout, 3 attempts with exponential backoff (500ms initial) on 5xx/429, waiting for `Retry-After` when the response sends one (30s at most). Both GitHub and Packagist use this client. Rate-limit tracking keeps **minimum** `Remaining` seen across calls (= peak usage), not last-wins.

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

The default gate is fast and offline. It covers semver comparison, overlap ratio, changelog entry extraction, title regex, `sync` folding/canonicalization, GitHub URL parsing (HTTPS + SSH), exception handling, and markdown heading/code-fence scanning.

### End-to-end

```bash
go test -tags=e2e ./...
go test -count=1 -race ./cli/ ./service/
```

`main_test.go` compiles the binary and runs it out of a scratch directory, asserting on stdout, the exit code, and the files it leaves behind — including that the GitHub token is redacted in `debug:parameters` and that `--repo-url` clones a scratch repository built with git plumbing. Anything needing a prerequisite the module cannot provide (git, a real GitHub token) skips rather than fails. The build tag keeps all of it out of the default gate.

`.dev/validate/all.sh --e2e` runs it as a section, and the CI workflow runs it on every push. It stays behind a flag locally because it compiles the binary, which is more than a pre-commit hook should cost — the hook and CI enforce the same set except for this one lane, deliberately.

### Fuzzing

```bash
go test -run=NONE -fuzz=FuzzCompareSemver -fuzztime=30s ./cli
```

Targets exist for the parsers that have produced real bugs: `FuzzCompareSemver`, `FuzzSemverParts`, `FuzzParseGithubUrl`, `FuzzExtractChangelogEntry` and `FuzzFoldChangelogBody`. They assert properties, not just the absence of a crash — the ordering is reflexive, antisymmetric and transitive; the fold is idempotent and leaves no section heading at the changelog's nesting level. Crashers are kept as seeds under `cli/testdata/fuzz/` and run as ordinary tests from then on.

### Static analysis

```bash
staticcheck ./... && staticcheck -tags=e2e ./...
```

Configured by `staticcheck.conf`, which disables only the two checks that contradict the house style on purpose (`S1002` bool comparisons, `ST1017` Yoda conditions). staticcheck does not check formatting, so the four-space indentation is untouched. It is baked into the dev image and runs as a section of `.dev/validate/all.sh`.

## Code style

Go code here follows a yoda-comparison house style (`nil == err`, `false == flag`, `"" == token`), descriptive variable names (`configuration`, `repository`, `command`), four-space indentation and `/* */` comments only — and few of them: a comment earns its place when the code is tangled or when something deliberate would otherwise be "fixed". Do not run `gofmt`; keep new code consistent.

---

## Internal — precision-soft defaults

The built-in project list in `config/project/project.go` is tuned for the `precision-soft/*` open-source libraries. If you're working inside that org (or forking and swapping the list for your own), these are the copy-paste commands:

```bash
# All projects
go run . audit [--token TOKEN] [--fail-on-warning] [--exceptions PATH]
go run . sync  [--token TOKEN] [--apply]

# doctrine-type
go run . audit --repo doctrine-type [--token TOKEN] [--fail-on-warning]
go run . sync  --repo doctrine-type [--token TOKEN] [--apply]

# doctrine-utility
go run . audit --repo doctrine-utility [--token TOKEN] [--fail-on-warning]
go run . sync  --repo doctrine-utility [--token TOKEN] [--apply]

# symfony-console
go run . audit --repo symfony-console [--token TOKEN] [--fail-on-warning]
go run . sync  --repo symfony-console [--token TOKEN] [--apply]

# symfony-doctrine-audit
go run . audit --repo symfony-doctrine-audit [--token TOKEN] [--fail-on-warning]
go run . sync  --repo symfony-doctrine-audit [--token TOKEN] [--apply]

# symfony-doctrine-encrypt
go run . audit --repo symfony-doctrine-encrypt [--token TOKEN] [--fail-on-warning]
go run . sync  --repo symfony-doctrine-encrypt [--token TOKEN] [--apply]

# symfony-phpunit
go run . audit --repo symfony-phpunit [--token TOKEN] [--fail-on-warning]
go run . sync  --repo symfony-phpunit [--token TOKEN] [--apply]

# melody (resolves CHANGELOG.md + v2/CHANGELOG.md + v3/CHANGELOG.md)
go run . audit --repo melody [--token TOKEN] [--fail-on-warning]
go run . sync  --repo melody [--token TOKEN] [--apply]

# symfony-json-form
go run . audit --repo symfony-json-form [--token TOKEN] [--fail-on-warning]
go run . sync  --repo symfony-json-form [--token TOKEN] [--apply]
```

Special-case config for melody (three changelogs, one per major version) and `GoSubmodule=true` (filters `integrations/*/vX.Y.Z` tags from core auditing) lives in `config/project/project.go` — use it as a template if your own project has a similar layout.

#### All precision-soft repos in one shot

`--repo` accepts a comma-separated list, so you can target the full precision-soft set (or any subset) in a single invocation:

```bash
# audit all precision-soft projects
go run . audit --repo doctrine-type,doctrine-utility,symfony-console,symfony-doctrine-audit,symfony-doctrine-encrypt,symfony-json-form,symfony-phpunit,melody [--token TOKEN] [--fail-on-warning]

# sync all precision-soft projects (dry-run by default; add --apply to PATCH)
go run . sync  --repo doctrine-type,doctrine-utility,symfony-console,symfony-doctrine-audit,symfony-doctrine-encrypt,symfony-json-form,symfony-phpunit,melody [--token TOKEN] [--apply]
```

Omitting `--repo` entirely is equivalent — it targets every project in `config/project/project.go`. Use the comma-separated form when you want a specific subset (e.g. `--repo doctrine-type,doctrine-utility`). `--repo-url` is only valid with a single `--repo` value.

## External project manifests and CI output

`audit --config projects.json` loads `{ "mode": "merge|replace", "projects": [...] }` — the same fields as `config/project/project.go` (`name`, `githubUrl`, `packagistUrl`, `goSubmodule`, `changelogPaths`). `merge` keeps the built-ins and overrides them by name; `replace` audits only the manifest. A misspelled key, a URL without an owner and a repository, a duplicate URL or a changelog path that escapes the repository is refused before anything is fetched. `--repo-url` is deliberately not validated the same way: `sync` clones from any git URL, `file://` included.

`--github-annotations` writes one workflow command per issue to stderr (`::warning title=<org>/<repo> <tag> <level>::<issue>`, `::error` for a failed level), in a fixed level order, with only the title escaped as a property; stdout stays the document, so `--format json` is still parseable in the same step. melody's boot log also lands on stderr as json lines, which the runner ignores.

`--cache-dir PATH` keeps GitHub API responses by `ETag` and revalidates them with `If-None-Match`; entries are written atomically with mode `0600`. It is off unless set, and a cache that cannot be read or written costs nothing but the cache. The key is the URL alone: an entity tag is only ever offered, and GitHub answers a caller that may not see a resource with 404, never with 304.

Supply-chain checks are opt-in through `--supply-chain signed-tags,checksums,sbom,attestations` or `all`. They run after the main pass, project by project: a release without a signed tag, a checksum asset, an SBOM asset or a GitHub artifact attestation carries one issue per missing item on its `supply-chain` level, and a project whose GitHub calls fail carries that failure on the same level instead of aborting the audit. Findings are warnings unless `--supply-chain-fail` is set, in which case they fail the release and the project like any other level.
