# Changelog

All notable changes to this project will be documented in this file.

## 2026-06-17

### Fixed

- `semverParts()` — a pre-release or build-metadata suffix (e.g. `v1.2.3-rc1`, `v1.2.3+build`) is now stripped before parsing the patch segment; previously `strconv.Atoi("3-rc1")` failed and silently zeroed the segment, so `v1.2.3-rc1` compared as `1.2.0`
- `stripTrailingLinkReferences()` — the trailing-link matcher now requires an `http(s)://` URL, so a section ending in a non-URL markdown reference definition (e.g. `[ticket]: ABC-123`) is no longer stripped from the extracted release body
- `EnsureCloneReset()` — clone names containing path separators or `..` are now rejected, preventing a malformed project URL from writing outside `.dev-data/clones`
- `EnsureCloneReset()` — repo URLs starting with `-` are now rejected and `git clone` is invoked with a `--` separator, preventing a malicious project URL from being parsed as a git option (e.g. `--upload-pack=…`, `-ext::sh …` argument injection)
- `CompareTags()` — the JSON decode error is now wrapped with the org/repo and compared refs for context
- `compareSemver()` — when numeric versions are equal a pre-release tag now ranks below its final release (`v1.0.0-rc1 < v1.0.0`) per semver precedence; the previous raw string fallback ranked the longer `-rc1` string as greater, skewing "latest version" selection when pre-release tags exist
- `comparePrerelease()` — equal pre-release tags are now compared per semver §11: identifiers are split on `.`, numeric identifiers compare numerically (so `v1.0.0-rc.2 < v1.0.0-rc.10`), numeric identifiers rank below alphanumeric ones, and a larger identifier set outranks a shorter prefix (`v1.0.0-rc < v1.0.0-rc.1`); the previous fallback compared the whole pre-release string lexically, ordering `rc.10` before `rc.2`

### Added

- `TestSemverPartsStripsPreReleaseAndBuild`, `TestCompareSemverIsNumericNotLexicographic`, `TestCompareSemverPreReleaseRanksBelowFinal`, `TestStripTrailingLinkReferencesDropsCompareLinks`, `TestStripTrailingLinkReferencesKeepsNonUrlReference` — unit coverage for the parsing fixes
- `TestCompareSemverPreReleaseDottedIdentifiers` — coverage for dotted pre-release precedence (numeric vs alphanumeric identifiers, identifier-count tiebreak)
- `TestEnsureCloneResetRejectsInvalidInput` (clone-name/url validation, incl. the path-traversal guard and the option-injection guard for `--upload-pack`/`-ext::sh` URLs), `TestShouldRetryStatus` (retry status policy), `TestAutoTableMaxWidthUnlimitedWhenNotTerminal` (non-terminal width), and `TestGetPackagistPackageVersions*` (packagist payload parsing, dist-reference fallback, missing-package and non-2xx errors) — the last via a dependency-free `fakeHttpClient`/`fakeResponse` test double

## 2026-04-23

### Fixed

- `classifyDiff()` — false positive "release has no code changes" on monorepo multi-version tags. When the semver-predecessor tag is chronologically newer than the tag under audit (e.g. `v1.12.0 → v2.0.0` where `v2.0.0` was committed before `v1.12.0`), `git diff v1.12.0...v2.0.0` returns empty because `v2.0.0` is an ancestor of `v1.12.0`. The GitHub compare API returns `status="behind"` in this case; the diff check now returns `LevelNotApplicable` instead of failing. Lock-step releases (`status="identical"`) continue to be flagged as expected.

### Changed

- `service.CompareResponse` — added `Status string`, `AheadBy int`, `BehindBy int` fields from the GitHub compare API response so callers can distinguish direction from emptiness
- `auditDiff()` — classification logic extracted into `classifyDiff()` pure helper for testability

### Added

- `TestClassifyDiffBehindIsNotApplicable`, `TestClassifyDiffIdenticalIsNoCodeChanges`, `TestClassifyDiffNormalAheadIsOk` — unit coverage for the three comparison-direction branches

## 2026-04-20

### Fixed

- `extractChangelogEntry()` — the remainder of the heading line (` - YYYY-MM-DD - <Title>` for the titled form) previously leaked into the extracted entry body, so `sync` pushed a stray `- 2026-04-20 - Title` bullet as the first line of every release notes body. Extraction now advances past the newline at the end of the heading line so the body begins cleanly at the first `### Section`. The same fix also cleans up the trivial V1 (`## vX.Y.Z`) case

### Changed

- `changelog` level now requires the heading format `## [vX.Y.Z] - YYYY-MM-DD - <Title>`. Dated headings without a title still parse but emit a warning. The heading title is cross-checked against the GitHub release title summary; a mismatch is reported as a warning so the CHANGELOG stays the single source of truth for the release title
- `audit` CLI — first-tagged release is no longer reported as missing a compare link; the rule only fires on non-first tags
- `sync` command now updates the GitHub release **title** in addition to the body: the desired title is built as `<Project Name> <vX.Y.Z> - <Title>` from the CHANGELOG titled heading, using the `Name` field configured in `config/project/project.go`. Dry-run output shows a `title: "current" → "desired"` line; `--apply` sends a single `PATCH` with both `body` and `name`. Entries without a titled heading — or projects without a configured `Name` — leave the existing release title untouched (the `name` field is omitted from the payload), so sync never regresses manually curated titles
- `service/github.go` — `UpdateReleaseBody(organization, repository, releaseId, body)` renamed to `UpdateRelease(organization, repository, releaseId, body, name)`; `name` is omitted from the JSON payload when empty so legacy dated-only entries do not clobber existing release titles
- `config/project/project.go` — `Name` field filled for every built-in project (`Doctrine Type`, `Doctrine Utility`, `Symfony Console`, `Symfony Doctrine Audit`, `Symfony Doctrine Encrypt`, `Symfony JSON Form`, `Symfony PHPUnit`) so audit `titlePartsRegex` and sync title composition agree on the canonical, human-readable project label
- `config/project/project.go` — removed `git-audit` from its own default project list. git-audit is a standalone CLI tool (no tagged releases, no Packagist), so auditing itself would always fail the integrity level; `CHANGELOG.md` reformatted to date-based sections (`## YYYY-MM-DD`) instead of version-tagged headings

### Added

- `sync --tag vX.Y.Z` — restrict sync to a single tag across the filtered projects. Useful for rehearsing a sync on one release before rolling across all tags
- `cli/audit_command_test.go` — unit tests covering titled heading parsing, dated-only-heading warning, first-tag skip, non-first-tag compare-link enforcement, heading-tail stripping in `extractChangelogEntry()` for both V1 and V2 formats
- `cli/sync_command_test.go` — unit tests for `extractChangelogTitle()` (titled heading, dated-only, unknown version) and `buildReleaseName()` (format composition, empty when title missing, empty when project name missing)

## 2026-04-19

### Added

- `audit` command — cross-checks tags, GitHub releases, Packagist versions, changelog entries, and commit diffs for every configured project. Per-level reporting: `integrity`, `distribution`, `changelog`, `diff`, `presentation`. Projects audited in parallel (4 at a time)
- `sync` command — pushes local `CHANGELOG.md` sections into the matching GitHub release body (changelog is the source of truth). Default is dry-run with per-tag unified diff in a `DIFFS:` block; `--apply` actually `PATCH`es release bodies
- `exceptions` command — lists accepted warnings from `exceptions.json`, grouped by project, with `reviewed_until` expiry
- Automatic local-clone management — `sync` maintains `.dev-data/clones/<repo>/` (clone if missing, hard-reset to origin if present); `.dev/clone-repos.sh` bulk-clones all configured projects
- `--repo-url URL` flag on both `audit` and `sync` — opt-in support for repositories not in the built-in project list
- Centralized HTTP behavior in `service/http_retry.go`: 30s timeout, 3 attempts with exponential backoff on 5xx/429, rate-limit tracking (peak-usage `Remaining`)
- Default project list in `config/project/project.go` for `precision-soft/*` open-source repositories
