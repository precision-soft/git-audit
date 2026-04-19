# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.0] - 2026-04-19

### Added

- `audit` command — cross-checks tags, GitHub releases, Packagist versions, changelog entries, and commit diffs for every configured project. Per-level reporting: `integrity`, `distribution`, `changelog`, `diff`, `presentation`. Projects audited in parallel (4 at a time)
- `sync` command — pushes local `CHANGELOG.md` sections into the matching GitHub release body (changelog is the source of truth). Default is dry-run with per-tag unified diff in a `DIFFS:` block; `--apply` actually `PATCH`es release bodies
- `exceptions` command — lists accepted warnings from `exceptions.json`, grouped by project, with `reviewed_until` expiry
- Automatic local-clone management — `sync` maintains `.dev-data/clones/<repo>/` (clone if missing, hard-reset to origin if present); `.dev/clone-repos.sh` bulk-clones all configured projects
- `--repo-url URL` flag on both `audit` and `sync` — opt-in support for repositories not in the built-in project list
- Centralized HTTP behavior in `service/http_retry.go`: 30s timeout, 3 attempts with exponential backoff on 5xx/429, rate-limit tracking (peak-usage `Remaining`)
- Default project list in `config/project/project.go` for `precision-soft/*` open-source repositories

[v0.1.0]: https://github.com/precision-soft/git-audit/releases/tag/v0.1.0
