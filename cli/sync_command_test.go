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

    if false == strings.Contains(folded, "## Fixed") {
        t.Errorf("expected '## Fixed' in folded: %q", folded)
    }
    if false == strings.Contains(folded, "## Changed") {
        t.Errorf("expected '## Changed' in folded: %q", folded)
    }
    if false == strings.Contains(folded, "## Added") {
        t.Errorf("expected '## Added' in folded: %q", folded)
    }
    if true == strings.Contains(folded, "### ") {
        t.Errorf("expected no '### ' headings after fold: %q", folded)
    }
    if true == strings.Contains(folded, "Security") || true == strings.Contains(folded, "Removed") {
        t.Errorf("expected Security/Removed to be folded away: %q", folded)
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
