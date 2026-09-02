# CONTRIBUTING

This document describes local development, testing, and contribution rules for Git Audit.

## Development setup

Prerequisites:

- Go 1.26+
- Docker (the repository ships a containerized development shell under [`.dev/`](./.dev/))

```bash
./dc build
./dc up -d dev
./dcsh dev
```

The container mounts the repository at the working directory and keeps the Go build and module caches under
`.dev-data/`, so a rebuild does not re-download the module graph.

## Verification

The gate lives in one place — [`.dev/validate/all.sh`](./.dev/validate/all.sh) — so a section is added once and every caller inherits it. Run it from the host, before opening a pull request:

```bash
.dev/validate/all.sh            # build, vet, test, staticcheck — on both build configurations
.dev/validate/all.sh --e2e      # also the end-to-end suite (builds the binary) and the race detector
.dev/validate/all.sh --audit    # also scan for known vulnerabilities (needs the network)
```

`--audit` is deliberately outside the default set: it is the only section that needs the network.

Every test leg passes **`-count=1`**. Without it `go test` returns a cached result and prints `ok … (cached)`, which certifies a binary that was never built from the current tree. Count `--- PASS` lines (`-v`) rather than trusting `ok`.

The end-to-end tests sit behind the **`e2e` build tag**, so `go test ./...` does not see them. The gate runs both configurations — `go vet`, `go vet -tags=e2e`, `go test -count=1`, `go test -tags=e2e -count=1` — because a test that only exists behind a tag is a test the default gate cannot regress. `--e2e` also runs `go test -race` over `cli/` and `service/`: the parallel audit shares one client across up to 32 goroutines, and the detector is the only thing that reads that.

The git `pre-commit` hook ([`.dev/git-hooks/pre-commit`](./.dev/git-hooks/pre-commit)) is a deliberately thin caller of the same script (`--staged`, which does nothing unless the index carries a `.go` change). It checks and never fixes. It adds one guard CI cannot: it reads the index and rejects a force-staged `.dev-data/` path or `.dev/docker/.env.local`, both of which are gitignored and, by the time a push reaches CI, would already be in the history.

### Development toolchain

The dev image ([`.dev/docker/Dockerfile`](./.dev/docker/Dockerfile)) pins the two analysers, and the asymmetry with the base image is deliberate:

- **staticcheck** and **govulncheck** are pinned, so the same source produces the same findings everywhere. They land in `${GOPATH}/bin`, which the module-cache volume does not cover, so they have to be baked into the image to survive a restart.
- **The toolchain is not pinned.** `golang:1.26-alpine` stays a floating patch tag. Every vulnerability reported here so far has lived in the Go standard library, which the toolchain ships and `go.mod` does not express — so `./dc build --pull` against a newer base image is the fix, and `go list -m -u all` cannot see the problem at all. Pinning the toolchain would freeze the vulnerability rather than the findings.

The staticcheck section reports and moves on when the image predates the binary: a missing linter must not block a commit. Rebuild the image to switch it back on.

`.profile` is copied late in the Dockerfile, after `apk add` and both `go install` steps, so an edit to it rebuilds in well under a second instead of re-running the expensive layers. `ENTRYPOINT` uses the exec form — the shell form wraps the process in `/bin/sh -c` and the process tree then lies about PID 1 — and signal delivery is handled by `init: true` in the compose file.

### Continuous integration

[`.github/workflows/ci.yml`](./.github/workflows/ci.yml) cannot call `.dev/validate/all.sh` — the script needs Docker and a compose project — so it runs the same commands natively instead: `build`, `vet`, `vet ( e2e )`, `test`, `test ( e2e )`, `staticcheck`, `govulncheck` and a `fuzz smoke` leg.

**CI reads the pinned linter versions out of the dev image's Dockerfile** rather than carrying its own copies, so a pin moves in exactly one place and the two can never disagree.

The `fuzz smoke` leg runs each target briefly. It is a smoke test, not a campaign: the five targets (`FuzzCompareSemver`, `FuzzSemverParts`, `FuzzParseGithubUrl`, `FuzzExtractChangelogEntry`,
`FuzzFoldChangelogBody`) each carry a seed corpus taken from the portfolio's real tags and changelogs, and every crash they have found so far was a genuine defect that is now a committed seed.

## Development workflow

This tool is **not released**. It carries no tags and no GitHub releases; the dated sections in
[`CHANGELOG.md`](./CHANGELOG.md) are its entire version history. Add a dated section for anything a user of the tool would notice, and nothing for internal cleanups.

## Code style

- Indent Go code with four spaces. Do not run `gofmt` or `go fmt`; both replace the repository's indentation and create unrelated churn.
- Use singular package, file, and type names, descriptive camel-case identifiers, and camel-case acronyms such as `urlString`, `httpClient`, and `userId`.
- Name every method receiver `instance`.
- Put constants on the left of comparisons, including ordered comparisons, and express boolean conditions explicitly without `!`.
- Use single-star `/* ... */` comments. Line comments are reserved for build, embed, generated-code, and linter directives that require them.
- Keep exported fields before unexported fields and prefer one major type per file.
- No raw `panic` — use typed or sentinel errors, and wrap with `%w` where the caller may want to unwrap.
- Plain `err` is fine when it is the only error in scope; with several in one scope, name them (`validationErr`, `dispatchErr`).
- Constructors go immediately before the type they construct.
- Multiline calls: one argument per line, closing `)` on its own line.
- One major type per file; no god files. No import cycles.
- For each source file `foo.go`, at most one test file `foo_test.go` — never split one source's tests across files.

### Comments and messages

The default is **no comment**. Write for a reader fluent in Go and in this domain. A comment is justified in exactly two cases: the code is genuinely tangled, or something looks wrong and is deliberate, so a reader would otherwise "fix" it. Nothing else — no history (git has it), no restating the line below, no narrating the reasoning that produced the code. The long "why" belongs here and in [`README.md`](./README.md).

When a comment is warranted it is **one short line**. If it wants to be a block, try a better name or an extracted function first. In a test, the test's **name** is the explanation.

Shell scripts under [`.dev/`](./.dev/) carry exactly one `#`: the shebang.

Error and log messages are fully lowercase.

## Reporting bugs

When submitting a bug report, include:

- The exact commit.
- Go version and operating system.
- Clear reproduction steps (minimal example if possible).
- The observed behavior and the expected behavior.
- Relevant output (redact tokens — the tool handles a GitHub token and redacts it in its own output).

## Submitting pull requests

- Use a topic branch based on `main`.
- Keep the PR focused: one logical change-set per PR.
- Add or update tests for behavioral changes, and put the test in the default build unless it genuinely needs the `e2e` tag.
- Update [`CHANGELOG.md`](./CHANGELOG.md) under a dated section.
- Update [`README.md`](./README.md) when user-facing behavior changes.

## Security and support

- For security issues, do not open a public issue: report privately through GitHub's private vulnerability reporting, with a minimal reproduction and an impact assessment.
- For non-security questions, use the issue tracker and include context (commit, steps, output).
