# Changelog

All notable changes to this project will be documented in this file.

## 2026-09-02

### Fixed

- `computeReleaseStatus()` — a supply-chain finding failed the **project** but not the **release** it was found on: the release status was computed from five levels while the project aggregate read six, so `--supply-chain-fail` produced `"status": "ok"` on a release whose `supplyChain` level said `failed`. The sixth level is part of the release status now, and the table's ISSUES block lists supply-chain issues, which it silently skipped
- `buildProjectAudit()` and `recomputeProjectAggregates()` — a check that never ran reported `ok`. Without `--supply-chain` the SUMMARY column was blank and the json carried `"supplyChainStatus": ""`, and the moment an exception filtered any warning the re-aggregation turned it into `ok`. The level is `n/a` on the project and on every release unless it was requested, the way `distribution` is without Packagist. The same re-aggregation reset a fetch-error audit — every `-` level to `ok`, the status to `ok`, the `fetchError` still attached — whenever it was reached; it now leaves those audits alone
- `applySupplyChainAudits()` — one failing GitHub call aborted the whole command before any output, including the audits of every other project, and a project already recorded with a fetch error was fetched again. A release's checks are recorded on its `supply-chain` level now: a missing tag reference (a lightweight or deleted tag answers 404) is `tag signature is not verified (tag reference not found)`, any other failure is `supply-chain check unavailable: …`, and the command continues. The `checksums` / `sbom` issues came out of a map, so their order — in the json, in the annotations and in the interactive prompt's numbering — changed from run to run; they are an ordered list
- `renderGithubAnnotations()` — the levels were iterated from a map, so two issues on different levels rendered in a different order on every run; it walks `levelNames` like `applyExceptions()` does. The message was escaped with the **property** escaper, so every `:` and `,` in an issue reached the Checks UI as a literal `%3A` / `%2C` — actions/toolkit encodes those two only in properties, which is what `escapeWorkflowProperty()` and `escapeWorkflowData()` now do separately
- `--cache-dir` — it defaulted to `.dev-data/cache`, relative to the caller's working directory and on for every invocation, so any `audit` run left a directory behind wherever it was started; and a cache that could not be read, decoded or written failed a request whose HTTP call had already succeeded. The cache is off unless the flag is set, and every cache failure is a miss
- `loadProjectManifest()` — a misspelled key (`github_url`, `changelogPath`) was silently ignored and the built-in list audited as if the override had applied; the decoder rejects unknown fields. `https://github.com/` and `https://github.com/owner` passed validation and produced `/repos//owner/…` requests that failed with `http 404`; an owner and a repository are required
- `GetTagVerification()` — the tag name was interpolated unescaped into the ref URL, so a `#` truncated the request at the fragment and **the wrong tag was verified**, a `%` was decoded server-side; every segment is escaped and the `/` of a go submodule tag is kept
- `--concurrency` — the 1..32 rule lived in two places that disagreed about who owned it (`Run()` refused out-of-range values, `auditProjectsParallel()` clamped only the low end); one `validateConcurrency()` over two named bounds, and the fan-out clamps both ends
- `.dev/validate/all.sh` — the plain `go test` leg ran without `-count=1`, so a cached pass certified a tree that was never built, while `CONTRIBUTING.md` claimed every leg had it

### Changed

- **`types.LevelResult` is serialised as `status` / `issues`** — the struct carried no json tags, so every per-release level (`integrity`, `distribution`, `changelog`, `diff`, `presentation`, `supplyChain`) rendered its two fields capitalised inside otherwise lowerCamel documents. A consumer that reads `Status` or `Issues` from the `audit --format json` output has to switch to the lowercase keys
- `service.GithubClient` — the API and raw hosts are fields (`SetEndpoints()`), fed by two new optional parameters `github.api_base` / `github.raw_base` (`GITHUB_API_BASE` / `GITHUB_RAW_BASE` in the `.env` files); unset, the public hosts are used. `GithubReleaseService` hands them to every client it builds, the `--token` client included. Non-2xx answers are a typed `HttpStatusError`, so a caller can tell a 404 from a transport failure
- `.dev/docker/Dockerfile` — `golang:1.25-alpine` (a branch at its end of life) → `golang:1.26-alpine`, staticcheck `v0.7.0` → `v0.8.1`, and `gcc` + `musl-dev` so `go test -race` can build; `go.mod` moves to `go 1.26.0` and the README badge and `CONTRIBUTING.md` follow. `.github/workflows/ci.yml` follows with `go-version: 1.26.x` and a `test ( race )` step; `all.sh --e2e` runs the race detector over `cli/` and `service/`
- `cli/audit_command.go`, `cli/exceptions.go`, `cli/project_resolver.go`, `service/github.go` — the seven explanatory comments the feature commit had deleted are back; each records a decision a reader would otherwise "fix" (the section whitelist, the semver ordering, one document per invocation, warnings-only exceptions, peak rate-limit usage)

### Added

- `audit --config PATH` — an external project manifest, `{ "mode": "merge|replace", "projects": [...] }`, validated before anything is fetched (`loadProjectManifest()`, `validateManifestProject()`); `merge` keeps and overrides the built-ins by name, `replace` audits the manifest alone
- `audit --github-annotations`, `--concurrency N`, `--cache-dir PATH` — workflow commands on stderr for GitHub Actions with the json document intact on stdout; the fan-out width; an opt-in `ETag` cache with atomic `0600` entries revalidated through `If-None-Match`
- `audit --supply-chain LIST` and `--supply-chain-fail` — opt-in per-release checks (`signed-tags`, `checksums`, `sbom`, `attestations`, `all`) on a sixth level, `supply-chain`, in the summary, the json and the annotations; findings are warnings unless the fail flag is set
- `main_test.go` — an in-process fake of the GitHub API and raw host that the built binary is pointed at through `.env.local`, and six end-to-end runs over it: the two manifest modes, a misspelled manifest key, annotations on stderr with parseable stdout, a second `--cache-dir` run revalidating with `If-None-Match` (and nothing written without the flag), the supply-chain fields agreeing across release, level and project under `--supply-chain-fail`, and `--concurrency` out of range. `testdata/manifest-*.json` carry the manifests
- `cli/github_annotations_test.go`, `service/github_test.go` — one test file per source, as the house rule asks; the annotation test that lived in `project_manifest_test.go` moved, and `service/github.go` has its first tests (endpoint configuration, the tags and raw paths, a non-2xx answer). `fake_http_test.go` records the URLs it is asked for, which is how the tag escaping is asserted

## 2026-08-16

### Fixed

- `.dev/docker/Dockerfile` — **the built binary carried five known standard-library vulnerabilities.** The image had settled on `go1.25.12`; `GO-2026-6218` (quadratic `resolvePath` in `net/url`), `GO-2026-6090` (unbounded post-handshake messages in `crypto/tls`), `GO-2026-6089` (`ReadHeaderTimeout` not applied on the unencrypted HTTP/2 check in `net/http`), `GO-2026-5972` (asn1 recursion depth) and `GO-2026-5026` (`x/net/idna` punycode labels) are all fixed in `go1.25.13`, and govulncheck traced every one of them to a **reachable** call path — four of them through `service.requestWithRetry`, the function that talks to GitHub. Rebuilding against a freshly pulled `golang:1.25-alpine` clears all five. Nothing in `go.mod` was touched and nothing could have been: `go list -m -u all` reports every module current, because the standard library is shipped by the toolchain and is not a module

### Added

- `govulncheck` in the dev image, pinned by `GOVULNCHECK_VERSION` in the Dockerfile alongside `STATICCHECK_VERSION`, and a `govulncheck` section of `.dev/validate/all.sh` behind a new `--audit` flag — the only section that needs the network, which is why it is opt-in rather than part of the default gate. It runs on both build configurations and, like the staticcheck section, reports and moves on when the image predates it. Note the deliberate asymmetry the comment in the Dockerfile records: the **scanner** is pinned, the **toolchain** is not. `golang:1.25-alpine` stays a floating patch tag precisely because every finding so far has been in the standard library, so pinning it would freeze the vulnerability rather than the findings
- `.dev/validate/all.sh` — flags are parsed as a set instead of being matched against `$1`, so `--audit` is reachable behind `--all` or `--staged` rather than being silently ignored
- `config/parameter_test.go` — the token redaction is now pinned in the **fast** gate. It was proved end to end against the built binary with a canary token, but that test sits behind the `e2e` build tag, so `go test ./...` — what the pre-commit hook runs — covered none of it, and the `config` package had **no test at all** (0% of statements). Both halves are asserted: `github.token` goes through `RegisterSecretParameter` rather than the plain `RegisterParameter`, and the `GITHUB_TOKEN` melody registers by itself from the `.env` artifacts is marked secret — marking only one of the two spellings leaves the other rendering the same token in clear text through `debug:parameters`. Verified by regressing the call to `RegisterParameter`, which now fails the fast gate

## 2026-08-15

### Fixed

- `config/parameter.go` — **the GitHub token was printed in clear text by `debug:parameters`**. melody registers that command automatically, and it renders the *resolved* value of every parameter it knows; the token was registered with the ordinary `RegisterParameter`, so both `github.token` and the `GITHUB_TOKEN` key melody registers from the `.env` artifacts carried it verbatim into stdout, in the table and the json format alike. Both are now declared secret — `RegisterSecretParameter` for this module's own parameter and `MarkParameterSecret` for melody's — and render as `********`. Reproduced against the built binary with a canary token before the fix and pinned by `TestBinaryRedactsTheGithubToken`
- `AuditCommand.Run()` — the `TITLE FIXES:` block was written after the rendered envelope regardless of the output format, so `audit --format json` produced a stream no parser accepts (`invalid character 'T' after top-level value`). `printSyncDiffs` had guarded the same way since it was written; the audit side had not. Rendering and the follow-up block now live in `renderAuditOutput()`, which emits the block only for the table format
- `reviewWarningsInteractively()` — the interactive prompt was written to `os.Stdout` with `fmt.Printf` rather than to the command's writer, and ran in every output format. It now writes to the command writer and is confined to the table format, for the same reason as above
- `applyExceptions()` — an exception matching an issue on a **failed** level stripped that issue while leaving the failure standing, so the audit reported red with nothing under it to explain why. Exceptions accept a warning, never a failure: a level that is not a warning is now left untouched. The interactive review already offered warnings only, so this closes the gap a hand-edited `exceptions.json` could reach
- `auditProject()` — the audited releases were ordered lexically by tag name while every other tag comparison in the tool uses `compareSemver()`, so `v4.1.12` was listed above `v4.1.9` in every table. Extracted as `sortReleaseAudits()` and ordered by semver
- `nonStandardSections()` and `foldChangelogBody()` — a heading inside a fenced code block was treated as a real heading, so a release body quoting markdown was reported as carrying a non-standard section, and a changelog quoting `### Security` had that sample rewritten to `## Security` on its way into the release body. Both now track fenced regions through the shared `codeFenceScanner`
- `foldChangelogBody()` — a heading indented by one to three columns, still a heading in markdown, was left at `###` in the release body while every sibling rose to `##`; a heading indented by four, which is an indented code block and not a heading at all, was liable to become one. Heading detection moved to the shared `markdownHeading()`, which counts the indent instead of trimming it. Found by `FuzzFoldChangelogBody`
- `foldChangelogBody()` and `canonicalReleaseBody()` — replacing `\r\n` left a lone `\r` behind, and trimming only spaces and tabs then rebuilt a `\r\n` pair for the next pass to replace again. The body never settled, so `sync` would have reported the same release as out of date on every run. The carriage return is now trimmed alongside the space and the tab. Found by `FuzzFoldChangelogBody`
- `foldChangelogBody()` and `canonicalReleaseBody()` — trimming the assembled block as one string took the indentation of the first content line with it, and four leading columns are what separates an indented code block from a heading. Only the blank lines at the edges are dropped now, through `trimBlankEdgeLines()`. Found by `FuzzFoldChangelogBody`

### Changed

- `go.mod` — melody `v3.11.0` → `v3.13.0`, for the secret-parameter API the token fix needs (`RegisterSecretParameter`, `MarkParameterSecret`, `Parameter.IsSecret()` and the redaction in `debug:parameters`). The interfaces this module implements are unchanged across those two minors; only ones it consumes gained methods, so the bump is source-compatible

### Added

- `main_test.go` — end-to-end coverage behind a `//go:build e2e` tag, the first tests in this module that run the built binary. They compile the command, run it out of a scratch directory and assert on stdout, the exit code and the files left behind: the token redaction, the command list, a rendered exceptions file, the unknown-repo refusal, and an ad-hoc `--repo-url` clone of a scratch repository built with git plumbing. Prerequisites the module cannot provide (git, a real token) skip rather than fail, and the tag keeps all of it out of `go test ./...`
- `FuzzCompareSemver`, `FuzzSemverParts`, `FuzzParseGithubUrl`, `FuzzExtractChangelogEntry`, `FuzzFoldChangelogBody` — property-based fuzzing of the parsers that have produced real bugs. The ordering must be reflexive, antisymmetric and transitive; `semverParts()` must never yield a negative segment; neither half of a parsed GitHub URL may carry a separator; an extracted entry must be a piece of the changelog it came from; the fold must be idempotent and leave no section heading at the changelog's nesting level. Three of the four fixes above came out of these runs, and their crashers are kept as seeds under `cli/testdata/fuzz/`
- `cli/exceptions_test.go` — direct coverage for `applyExceptions()`, `ExceptionEntry.Active()` and `parseSelection()`, which had none
- `codeFenceScanner` and `markdownHeading()` in `cli/shared.go` — one markdown-aware heading scanner shared by the presentation audit and the changelog fold, so the two cannot disagree about what a heading is
- `staticcheck` in the dev image, pinned by `STATICCHECK_VERSION` in the Dockerfile, with a `staticcheck.conf` that disables only `S1002` (bool comparisons) and `ST1017` (Yoda conditions) because both contradict the house style deliberately. It runs clean on both build configurations and is a section of `.dev/validate/all.sh`, which reports and moves on when the image predates it
- `README.md` — sections for the end-to-end suite, fuzzing and static analysis, a note that the token now renders redacted, and a warning that `-tags melody_env_embedded` embeds `.env.local` — and with it a real token — into the binary

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
