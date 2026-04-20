package cli

import (
    "strings"
    "testing"

    "github.com/precision-soft/git-audit/types"
)

func TestCompareSemver(t *testing.T) {
    cases := []struct {
        left, right string
        want        int
    }{
        {"v1.0.0", "v1.0.0", 0},
        {"v1.0.0", "v1.0.1", -1},
        {"v1.0.1", "v1.0.0", 1},
        {"v1.9.0", "v1.10.0", -1},
        {"v1.10.0", "v1.9.0", 1},
        {"v1.10.0", "v1.10.1", -1},
        {"v2.0.0", "v1.99.99", 1},
        {"v0.0.0", "v0.0.1", -1},
        {"v1.2.3", "v1.2.10", -1},
        {"integrations/bunorm/v1.0.0", "integrations/bunorm/v1.2.0", -1},
    }

    for _, testCase := range cases {
        got := compareSemver(testCase.left, testCase.right)
        gotSign := 0
        if got < 0 {
            gotSign = -1
        } else if got > 0 {
            gotSign = 1
        }
        if gotSign != testCase.want {
            t.Errorf("compareSemver(%q, %q) = %d, want sign %d", testCase.left, testCase.right, got, testCase.want)
        }
    }
}

func TestOverlapRatio(t *testing.T) {
    cases := []struct {
        name        string
        left, right string
        wantLow     float64
        wantHigh    float64
    }{
        {"identical", "fixed tinyint range", "fixed tinyint range", 1.0, 1.0},
        {"disjoint", "alpha beta", "gamma delta", 0.0, 0.0},
        {"empty", "", "nonempty", 0.0, 0.0},
        {"mostlyOverlap", "fixed tinyint range validation", "fixed tinyint range", 0.60, 0.80},
    }

    for _, testCase := range cases {
        got := overlapRatio(testCase.left, testCase.right)
        if got < testCase.wantLow || got > testCase.wantHigh {
            t.Errorf("%s: overlapRatio(%q, %q) = %v, want in [%v, %v]", testCase.name, testCase.left, testCase.right, got, testCase.wantLow, testCase.wantHigh)
        }
    }
}

func TestExtractChangelogEntry(t *testing.T) {
    content := "# Changelog\n\n" +
        "## [v2.0.0] - 2026-01-15\n\n" +
        "### Changed\n\n- Big change\n\n" +
        "## [v1.0.1] - 2025-12-20\n\n" +
        "### Fixed\n\n- Small fix\n\n" +
        "## [v1.0.0] - 2025-12-01\n\n" +
        "### Added\n\n- Initial release\n\n" +
        "[v2.0.0]: https://example.com/compare/v1.0.1...v2.0.0\n"

    body, found := extractChangelogEntry(content, "v1.0.1")
    if false == found {
        t.Fatalf("expected to find v1.0.1 entry")
    }
    if false == strings.Contains(body, "Small fix") {
        t.Errorf("expected body to contain 'Small fix', got %q", body)
    }
    if true == strings.Contains(body, "Big change") || true == strings.Contains(body, "Initial release") {
        t.Errorf("body leaked into adjacent versions: %q", body)
    }

    if _, foundMissing := extractChangelogEntry(content, "v9.9.9"); true == foundMissing {
        t.Errorf("unexpectedly found v9.9.9 entry")
    }
}

func TestTitlePartsRegex(t *testing.T) {
    cases := []struct {
        title string
        want  bool
    }{
        {"Doctrine Type v1.0.0 - Initial Release", true},
        {"Symfony Console v2.3.4 - Fix description", true},
        {"v1.0.0", false},
        {"bad format", false},
        {"no version - here", false},
    }

    for _, testCase := range cases {
        got := titlePartsRegex.MatchString(testCase.title)
        if got != testCase.want {
            t.Errorf("titlePartsRegex(%q) = %v, want %v", testCase.title, got, testCase.want)
        }
    }
}

func TestShortSha(t *testing.T) {
    if got := shortSha("abcdef1234567890"); "abcdef1" != got {
        t.Errorf("shortSha long input = %q, want abcdef1", got)
    }
    if got := shortSha("abc"); "abc" != got {
        t.Errorf("shortSha short input = %q, want abc", got)
    }
}

func TestApplyChangelogDateAndLinkChecks_FirstTagSkipsCompareLink(t *testing.T) {
    content := "# Changelog\n\n" +
        "## [v1.0.0] - 2026-01-15 - Initial release\n\n" +
        "### Added\n\n- Initial release\n\n"

    result := &ChangelogAuditResult{Status: types.LevelOk}
    applyChangelogDateAndLinkChecks("v1.0.0", "", content, "CHANGELOG.md", result)

    if types.LevelOk != result.Status {
        t.Errorf("first tag should not produce warnings, got status=%v issues=%v", result.Status, result.Issues)
    }
    for _, issue := range result.Issues {
        if true == strings.Contains(issue, "compare link") {
            t.Errorf("first tag should not emit compare-link issue, got %q", issue)
        }
    }
    if "Initial release" != result.HeadingTitle {
        t.Errorf("expected HeadingTitle=%q, got %q", "Initial release", result.HeadingTitle)
    }
}

func TestApplyChangelogDateAndLinkChecks_NonFirstTagRequiresCompareLink(t *testing.T) {
    content := "# Changelog\n\n" +
        "## [v1.0.1] - 2026-02-01 - Small fix\n\n" +
        "### Fixed\n\n- Small fix\n\n"

    result := &ChangelogAuditResult{Status: types.LevelOk}
    applyChangelogDateAndLinkChecks("v1.0.1", "v1.0.0", content, "CHANGELOG.md", result)

    if types.LevelWarning != result.Status {
        t.Errorf("non-first tag without compare link should warn, got status=%v", result.Status)
    }
    foundCompareIssue := false
    for _, issue := range result.Issues {
        if true == strings.Contains(issue, "missing compare link") {
            foundCompareIssue = true
            break
        }
    }
    if false == foundCompareIssue {
        t.Errorf("expected compare-link issue, got %v", result.Issues)
    }
}

func TestApplyChangelogDateAndLinkChecks_FirstTagStillValidatesDate(t *testing.T) {
    content := "# Changelog\n\n" +
        "## [v1.0.0]\n\n" +
        "### Added\n\n- Initial release\n\n"

    result := &ChangelogAuditResult{Status: types.LevelOk}
    applyChangelogDateAndLinkChecks("v1.0.0", "", content, "CHANGELOG.md", result)

    if types.LevelWarning != result.Status {
        t.Errorf("missing date/title on first tag should still warn, got status=%v", result.Status)
    }
    foundHeadingIssue := false
    for _, issue := range result.Issues {
        if true == strings.Contains(issue, "YYYY-MM-DD - <Title>") {
            foundHeadingIssue = true
            break
        }
    }
    if false == foundHeadingIssue {
        t.Errorf("expected heading-suffix issue, got %v", result.Issues)
    }
}

func TestApplyChangelogDateAndLinkChecks_DatedWithoutTitleWarns(t *testing.T) {
    content := "# Changelog\n\n" +
        "## [v1.0.0] - 2026-01-15\n\n" +
        "### Added\n\n- Initial release\n\n"

    result := &ChangelogAuditResult{Status: types.LevelOk}
    applyChangelogDateAndLinkChecks("v1.0.0", "", content, "CHANGELOG.md", result)

    if types.LevelWarning != result.Status {
        t.Errorf("dated heading without title should warn, got status=%v", result.Status)
    }
    foundTitleIssue := false
    for _, issue := range result.Issues {
        if true == strings.Contains(issue, "<Title>") && true == strings.Contains(issue, "after the date") {
            foundTitleIssue = true
            break
        }
    }
    if false == foundTitleIssue {
        t.Errorf("expected missing-title issue, got %v", result.Issues)
    }
    if "" != result.HeadingTitle {
        t.Errorf("expected HeadingTitle to be empty for legacy dated heading, got %q", result.HeadingTitle)
    }
}

func TestApplyChangelogDateAndLinkChecks_TitledHeadingExtractsTitleAndSkipsWarning(t *testing.T) {
    content := "# Changelog\n\n" +
        "## [v2.0.0] - 2026-03-01 - Harden HTTP timeouts\n\n" +
        "### Added\n\n- Timeout defaults\n\n" +
        "[v2.0.0]: https://example.com/compare/v1.0.0...v2.0.0\n"

    result := &ChangelogAuditResult{Status: types.LevelOk}
    applyChangelogDateAndLinkChecks("v2.0.0", "v1.0.0", content, "CHANGELOG.md", result)

    if types.LevelOk != result.Status {
        t.Errorf("titled heading with compare link should pass clean, got status=%v issues=%v", result.Status, result.Issues)
    }
    if "Harden HTTP timeouts" != result.HeadingTitle {
        t.Errorf("expected HeadingTitle=%q, got %q", "Harden HTTP timeouts", result.HeadingTitle)
    }
}

func TestParseGithubUrl(t *testing.T) {
    cases := []struct {
        url              string
        wantOrganization string
        wantRepository   string
    }{
        {"https://github.com/precision-soft/doctrine-type", "precision-soft", "doctrine-type"},
        {"https://github.com/precision-soft/doctrine-type/", "precision-soft", "doctrine-type"},
        {"https://github.com/precision-soft/doctrine-type.git", "precision-soft", "doctrine-type"},
        {"http://github.com/precision-soft/doctrine-type", "precision-soft", "doctrine-type"},
        {"git@github.com:precision-soft/doctrine-type.git", "precision-soft", "doctrine-type"},
        {"git@github.com:precision-soft/doctrine-type", "precision-soft", "doctrine-type"},
        {"ssh://git@github.com/precision-soft/doctrine-type.git", "precision-soft", "doctrine-type"},
        {"github.com/precision-soft/doctrine-type", "precision-soft", "doctrine-type"},
        {"precision-soft/doctrine-type", "precision-soft", "doctrine-type"},
    }

    for _, testCase := range cases {
        organization, repository := parseGithubUrl(testCase.url)
        if organization != testCase.wantOrganization || repository != testCase.wantRepository {
            t.Errorf(
                "parseGithubUrl(%q) = (%q, %q), want (%q, %q)",
                testCase.url, organization, repository,
                testCase.wantOrganization, testCase.wantRepository,
            )
        }
    }
}
