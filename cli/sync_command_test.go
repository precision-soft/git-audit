package cli

import (
    "strings"
    "testing"
)

func TestListChangelogVersions(t *testing.T) {
    content := "# Changelog\n\n" +
        "## [v2.0.0] - 2026-01-01\n\n- X\n\n" +
        "## [v1.1.0] - 2025-06-15\n\n- Y\n\n" +
        "## v1.0.0\n\n- Z\n"

    versions := listChangelogVersions(content)
    want := []string{"v2.0.0", "v1.1.0", "v1.0.0"}
    if len(versions) != len(want) {
        t.Fatalf("got %d versions, want %d: %v", len(versions), len(want), versions)
    }
    for index, version := range want {
        if versions[index] != version {
            t.Errorf("versions[%d] = %q, want %q", index, versions[index], version)
        }
    }
}

func TestFoldChangelogBody(t *testing.T) {
    body := "### Security\n\n- fixed vuln\n\n### Removed\n\n- old thing\n\n### Added\n\n- new thing\n"

    folded := foldChangelogBody(body)

    for _, section := range []string{"## Security", "## Removed", "## Added"} {
        if false == strings.Contains(folded, section) {
            t.Errorf("expected %q in folded: %q", section, folded)
        }
    }

    if true == strings.Contains(folded, "### ") {
        t.Errorf("expected no '### ' headings after fold: %q", folded)
    }

    for _, invented := range []string{"## Fixed", "## Changed"} {
        if true == strings.Contains(folded, invented) {
            t.Errorf("nothing in the entry was a fix or a change, so %q was invented by renaming a section: %q", invented, folded)
        }
    }
}

func TestFoldChangelogBodyKeepsSectionsDistinct(t *testing.T) {
    body := "### Fixed\n\n- a bug\n\n### Security\n\n- a vuln\n"

    folded := foldChangelogBody(body)

    if 1 != strings.Count(folded, "## Fixed") {
        t.Errorf("expected exactly one '## Fixed' section, got %d: %q", strings.Count(folded, "## Fixed"), folded)
    }

    if 1 != strings.Count(folded, "## Security") {
        t.Errorf("expected the security section to survive under its own name: %q", folded)
    }
}

func TestFoldChangelogBodyPreservesBullets(t *testing.T) {
    body := "### Added\n\n- `Foo::bar()` helper\n- `Baz` class\n"
    folded := foldChangelogBody(body)
    if false == strings.Contains(folded, "- `Foo::bar()` helper") {
        t.Errorf("bullet content lost: %q", folded)
    }
}

func TestCanonicalReleaseBodyCrlf(t *testing.T) {
    crlf := "## Added\r\n\r\n- one\r\n"
    lf := "## Added\n\n- one\n"
    if canonicalReleaseBody(crlf) != canonicalReleaseBody(lf) {
        t.Errorf("CRLF and LF should canonicalize identically:\n  crlf=%q\n  lf=%q",
            canonicalReleaseBody(crlf), canonicalReleaseBody(lf))
    }
}

func TestCanonicalReleaseBodyTrailingWhitespace(t *testing.T) {
    padded := "## Added\n\n- one   \n"
    clean := "## Added\n\n- one\n"
    if canonicalReleaseBody(padded) != canonicalReleaseBody(clean) {
        t.Errorf("trailing whitespace should be stripped:\n  padded=%q\n  clean=%q",
            canonicalReleaseBody(padded), canonicalReleaseBody(clean))
    }
}

func TestCanonicalReleaseBodyCollapsesBlankRuns(t *testing.T) {
    manyBlanks := "## Added\n\n\n\n- one\n"
    oneBlank := "## Added\n\n- one\n"
    if canonicalReleaseBody(manyBlanks) != canonicalReleaseBody(oneBlank) {
        t.Errorf("runs of blank lines should collapse:\n  many=%q\n  one=%q",
            canonicalReleaseBody(manyBlanks), canonicalReleaseBody(oneBlank))
    }
}

func TestCompareReleaseBodyDetectsContentDifference(t *testing.T) {
    if true == compareReleaseBody("## Added\n\n- one\n", "## Added\n\n- two\n") {
        t.Errorf("expected compareReleaseBody to detect content difference")
    }
}

func TestCompareReleaseBodyEqualsOnFormattingOnly(t *testing.T) {
    withTrailing := "## Added\n\n- one   \n"
    clean := "## Added\n\n- one\n"
    if false == compareReleaseBody(withTrailing, clean) {
        t.Errorf("expected compareReleaseBody to ignore trailing whitespace")
    }
}

func TestUnifiedDiffBasicReplacement(t *testing.T) {
    current := "## Added\n\n- old item"
    desired := "## Added\n\n- new item"
    diff := unifiedDiff(current, desired)

    if false == strings.Contains(diff, "- - old item") {
        t.Errorf("expected '- - old item' in diff:\n%s", diff)
    }
    if false == strings.Contains(diff, "+ - new item") {
        t.Errorf("expected '+ - new item' in diff:\n%s", diff)
    }
    if false == strings.Contains(diff, "  ## Added") {
        t.Errorf("expected unchanged '  ## Added' context:\n%s", diff)
    }
}

func TestUnifiedDiffEmptyCurrent(t *testing.T) {
    diff := unifiedDiff("", "## Added\n\n- first line")
    if false == strings.Contains(diff, "+ ## Added") {
        t.Errorf("expected '+ ## Added' when current is empty:\n%s", diff)
    }
    for _, line := range strings.Split(diff, "\n") {
        if true == strings.HasPrefix(line, "- ") {
            t.Errorf("unexpected removed line %q when current is empty:\n%s", line, diff)
        }
    }
}

func TestUnifiedDiffIdenticalInputsProduceNoChanges(t *testing.T) {
    body := "## Added\n\n- one\n- two"
    diff := unifiedDiff(body, body)
    for _, line := range strings.Split(diff, "\n") {
        if true == strings.HasPrefix(line, "+ ") || true == strings.HasPrefix(line, "- ") {
            t.Errorf("unexpected change line %q in identical diff:\n%s", line, diff)
        }
    }
}

func TestExtractChangelogTitleFromTitledHeading(t *testing.T) {
    content := "## [v1.2.3] - 2026-04-20 - Some Release Title\n\n- change\n"
    title, found := extractChangelogTitle(content, "v1.2.3")
    if false == found {
        t.Fatalf("expected to find title for v1.2.3")
    }
    if "Some Release Title" != title {
        t.Errorf("got title %q, want %q", title, "Some Release Title")
    }
}

func TestExtractChangelogTitleReturnsFalseForDatedOnly(t *testing.T) {
    content := "## [v1.2.3] - 2026-04-20\n\n### Added\n\n- change\n"
    _, found := extractChangelogTitle(content, "v1.2.3")
    if true == found {
        t.Errorf("expected no title for dated-only heading")
    }
}

func TestExtractChangelogTitleReturnsFalseForUnknownVersion(t *testing.T) {
    content := "## [v1.2.3] - 2026-04-20 - Title\n"
    _, found := extractChangelogTitle(content, "v9.9.9")
    if true == found {
        t.Errorf("expected no title for missing version")
    }
}

func TestBuildReleaseNameComposesTitleFormat(t *testing.T) {
    content := "## [v2.0.0] - 2026-04-20 - Big Refactor\n\n- bullet\n"
    name := buildReleaseName("Symfony PHPUnit", "v2.0.0", content)
    if "Symfony PHPUnit v2.0.0 - Big Refactor" != name {
        t.Errorf("got %q, want %q", name, "Symfony PHPUnit v2.0.0 - Big Refactor")
    }
}

func TestBuildReleaseNameReturnsEmptyWhenTitleMissing(t *testing.T) {
    content := "## [v2.0.0] - 2026-04-20\n\n### Added\n\n- bullet\n"
    name := buildReleaseName("Symfony PHPUnit", "v2.0.0", content)
    if "" != name {
        t.Errorf("expected empty name for dated-only heading, got %q", name)
    }
}

func TestBuildReleaseNameReturnsEmptyWhenProjectNameMissing(t *testing.T) {
    content := "## [v2.0.0] - 2026-04-20 - Big Refactor\n\n- bullet\n"
    name := buildReleaseName("", "v2.0.0", content)
    if "" != name {
        t.Errorf("expected empty name when projectName is empty, got %q", name)
    }
}

func TestFoldChangelogBodyLeavesFencedCodeBlocksAlone(t *testing.T) {
    body := "### Added\n\n" +
        "- Documented the heading shape\n\n" +
        "```markdown\n" +
        "### Security\n" +
        "```\n"

    folded := foldChangelogBody(body)

    if false == strings.Contains(folded, "## Added") {
        t.Errorf("expected the real heading to be lifted to h2, got:\n%s", folded)
    }
    if false == strings.Contains(folded, "### Security") {
        t.Errorf("expected the fenced sample heading to survive untouched, got:\n%s", folded)
    }
}

/* the two properties: folding twice changes nothing more, and no h3 survives outside a fence */
func FuzzFoldChangelogBody(f *testing.F) {
    for _, seed := range []string{
        "### Added\n\n- one\n",
        "### Security\n\n- two\n\n### Removed\n\n- three\n",
        "### Added\n\n```markdown\n### Security\n```\n",
        "#### Deeper\n\n### Added\n",
        "", "###", "### ", "###  spaced  \r\n", "```\n### inside\n",
    } {
        f.Add(seed)
    }

    f.Fuzz(func(t *testing.T, body string) {
        folded := foldChangelogBody(body)

        if refolded := foldChangelogBody(folded); refolded != folded {
            t.Fatalf("foldChangelogBody is not idempotent for %q: %q then %q", body, folded, refolded)
        }

        var fence codeFenceScanner
        for _, line := range strings.Split(folded, "\n") {
            if true == fence.inside(line) {
                continue
            }
            if level, _ := markdownHeading(line); 3 == level {
                t.Fatalf("foldChangelogBody left %q at the changelog nesting level, from %q", line, body)
            }
        }
    })
}

func TestFoldChangelogBodyLiftsAnIndentedHeading(t *testing.T) {
    folded := foldChangelogBody(" ### Security\n\n- one\n   ### Removed\n\n- two\n")

    if false == strings.Contains(folded, "## Security") || true == strings.Contains(folded, "### Security") {
        t.Errorf("expected the indented h3 to be lifted, got:\n%s", folded)
    }
    if false == strings.Contains(folded, "## Removed") || true == strings.Contains(folded, "### Removed") {
        t.Errorf("expected the three-column indented h3 to be lifted, got:\n%s", folded)
    }
}

func TestFoldChangelogBodyLeavesAnIndentedCodeBlockAlone(t *testing.T) {
    folded := foldChangelogBody("### Added\n\n- one\n\n    ### Security\n")

    if false == strings.Contains(folded, "    ### Security") {
        t.Errorf("a four-column indent is an indented code block and must survive, got:\n%s", folded)
    }
}

func TestFoldChangelogBodyDropsStrayCarriageReturns(t *testing.T) {
    folded := foldChangelogBody("### Added\r\n\r\n- one \r \n- two\r\n")

    if true == strings.Contains(folded, "\r") {
        t.Fatalf("expected no carriage return in the folded body, got %q", folded)
    }
    if refolded := foldChangelogBody(folded); refolded != folded {
        t.Errorf("expected the fold to settle, got %q then %q", folded, refolded)
    }
}

func TestCompareReleaseBodyIgnoresLineEndingNoise(t *testing.T) {
    if false == compareReleaseBody("## Added\r\n\r\n- one \r \n", "## Added\n\n- one\n") {
        t.Error("expected bodies differing only in line endings and trailing whitespace to compare equal")
    }
}
