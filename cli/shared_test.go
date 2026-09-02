package cli

import (
    "strings"
    "testing"
    "time"

    "github.com/precision-soft/git-audit/config/project"
    "github.com/precision-soft/git-audit/service"
)

func TestIsGoSubmoduleTag(t *testing.T) {
    cases := []struct {
        tag  string
        want bool
    }{
        {"v1.0.0", false},
        {"integrations/bunorm/v1.0.0", true},
        {"sub/v2.3.4", true},
        {"integrations/bunorm/v1.0.0-beta", false},
        {"", false},
    }

    for _, testCase := range cases {
        got := isGoSubmoduleTag(testCase.tag)
        if got != testCase.want {
            t.Errorf("isGoSubmoduleTag(%q) = %v, want %v", testCase.tag, got, testCase.want)
        }
    }
}

func TestFormatRateLimitLineNoData(t *testing.T) {
    got := formatRateLimitLine(service.RateLimitInfo{HasData: false})
    if "" != got {
        t.Errorf("expected empty string when HasData=false, got %q", got)
    }
}

func TestFormatRateLimitLineBasic(t *testing.T) {
    info := service.RateLimitInfo{
        HasData:   true,
        Limit:     5000,
        Remaining: 4200,
    }

    got := formatRateLimitLine(info)

    if false == strings.Contains(got, "4200/5000") {
        t.Errorf("expected remaining/limit in output, got %q", got)
    }
}

func TestFormatRateLimitLineWithResource(t *testing.T) {
    info := service.RateLimitInfo{
        HasData:   true,
        Limit:     60,
        Remaining: 0,
        Resource:  "core",
    }

    got := formatRateLimitLine(info)

    if false == strings.Contains(got, "resource: core") {
        t.Errorf("expected resource in output, got %q", got)
    }
}

func TestFormatRateLimitLineWithResetTime(t *testing.T) {
    info := service.RateLimitInfo{
        HasData:   true,
        Limit:     5000,
        Remaining: 1000,
        Reset:     time.Now().Add(10 * time.Minute),
    }

    got := formatRateLimitLine(info)

    if false == strings.Contains(got, "resets at") {
        t.Errorf("expected reset time in output, got %q", got)
    }
}

func TestFormatRateLimitLineSeparatedByPipe(t *testing.T) {
    info := service.RateLimitInfo{
        HasData:   true,
        Limit:     5000,
        Remaining: 3000,
        Resource:  "search",
        Reset:     time.Now().Add(5 * time.Minute),
    }

    got := formatRateLimitLine(info)

    parts := strings.Split(got, " | ")
    if 3 != len(parts) {
        t.Errorf("expected 3 pipe-separated parts (remaining, reset, resource), got %d: %q", len(parts), got)
    }
}

func TestResolveChangelogPathsDefault(t *testing.T) {
    got := resolveChangelogPaths(project.ProjectConfig{})

    if 1 != len(got) || "CHANGELOG.md" != got[0] {
        t.Errorf("expected [CHANGELOG.md], got %v", got)
    }
}

func TestResolveChangelogPathsCustom(t *testing.T) {
    custom := []string{"packages/foo/CHANGELOG.md", "packages/bar/CHANGELOG.md"}
    got := resolveChangelogPaths(project.ProjectConfig{ChangelogPaths: custom})

    if len(custom) != len(got) {
        t.Fatalf("expected %d paths, got %v", len(custom), got)
    }
    for index, path := range custom {
        if got[index] != path {
            t.Errorf("got[%d] = %q, want %q", index, got[index], path)
        }
    }
}

func TestResolveChangelogPathsEmptySliceUsesDefault(t *testing.T) {
    got := resolveChangelogPaths(project.ProjectConfig{ChangelogPaths: []string{}})

    if 1 != len(got) || "CHANGELOG.md" != got[0] {
        t.Errorf("empty ChangelogPaths should fall back to default, got %v", got)
    }
}

func TestSemverPartsStripsPreReleaseAndBuild(t *testing.T) {
    cases := []struct {
        tag  string
        want [3]int
    }{
        {"v1.2.3", [3]int{1, 2, 3}},
        {"1.2.3", [3]int{1, 2, 3}},
        {"v1.2.3-rc1", [3]int{1, 2, 3}},
        {"v1.2.3+build.5", [3]int{1, 2, 3}},
        {"sub/v2.3.4", [3]int{2, 3, 4}},
    }

    for _, testCase := range cases {
        got := semverParts(testCase.tag)
        if got != testCase.want {
            t.Errorf("semverParts(%q) = %v, want %v", testCase.tag, got, testCase.want)
        }
    }
}

func TestCompareSemverIsNumericNotLexicographic(t *testing.T) {
    if -1 != compareSemver("v1.2.0", "v1.10.0") {
        t.Errorf("expected v1.2.0 < v1.10.0 (numeric), got %d", compareSemver("v1.2.0", "v1.10.0"))
    }
    if 0 != compareSemver("v1.2.3-rc1", "v1.2.3-rc1") {
        t.Errorf("expected identical tags to compare equal")
    }
}

func TestCompareSemverPreReleaseRanksBelowFinal(t *testing.T) {
    testCases := []struct {
        left  string
        right string
        want  int
    }{
        {"v1.0.0", "v1.0.0-rc1", 1},
        {"v1.0.0-rc1", "v1.0.0", -1},
        {"v1.0.0-rc1", "v1.0.0-rc2", -1},
        {"v1.0.0-rc2", "v1.0.0-rc1", 1},
        {"v1.0.0-rc1", "v1.0.0-rc1", 0},
        {"v2.0.0-rc1", "v1.0.0", 1},
    }
    for _, testCase := range testCases {
        if got := compareSemver(testCase.left, testCase.right); got != testCase.want {
            t.Errorf("compareSemver(%q, %q) = %d, want %d", testCase.left, testCase.right, got, testCase.want)
        }
    }
}

func TestCompareSemverPreReleaseDottedIdentifiers(t *testing.T) {
    testCases := []struct {
        left  string
        right string
        want  int
    }{
        {"v1.0.0-rc.2", "v1.0.0-rc.10", -1},
        {"v1.0.0-rc.10", "v1.0.0-rc.2", 1},
        {"v1.0.0-rc.2", "v1.0.0-rc.2", 0},
        {"v1.0.0-rc", "v1.0.0-rc.1", -1},
        {"v1.0.0-alpha", "v1.0.0-beta", -1},
        {"v1.0.0-alpha.1", "v1.0.0-alpha.beta", -1},
    }
    for _, testCase := range testCases {
        if got := compareSemver(testCase.left, testCase.right); got != testCase.want {
            t.Errorf("compareSemver(%q, %q) = %d, want %d", testCase.left, testCase.right, got, testCase.want)
        }
    }
}

func TestStripTrailingLinkReferencesDropsCompareLinks(t *testing.T) {
    body := "### Fixed\n\n- something\n\n[v1.0.0]: https://github.com/org/repo/compare/v0.9.0...v1.0.0"

    got := stripTrailingLinkReferences(body)

    if true == strings.Contains(got, "compare") {
        t.Errorf("expected trailing compare link to be stripped, got %q", got)
    }
    if false == strings.Contains(got, "- something") {
        t.Errorf("expected body content to be preserved, got %q", got)
    }
}

func TestStripTrailingLinkReferencesKeepsNonUrlReference(t *testing.T) {
    body := "### Fixed\n\n- done\n\n[ticket]: ABC-123"

    got := stripTrailingLinkReferences(body)

    if false == strings.Contains(got, "ABC-123") {
        t.Errorf("expected non-URL reference definition to be preserved, got %q", got)
    }
}

func TestCodeFenceScannerTracksFencedRegions(t *testing.T) {
    lines := []string{
        "## Added",
        "",
        "```go",
        "## not a heading",
        "```",
        "## Fixed",
        "~~~",
        "### neither is this",
        "~~~",
        "## Changed",
    }
    want := []bool{false, false, true, true, true, false, true, true, true, false}

    var fence codeFenceScanner
    for index, line := range lines {
        if got := fence.inside(line); got != want[index] {
            t.Errorf("line %d (%q): inside = %t, want %t", index, line, got, want[index])
        }
    }
}

func TestCodeFenceScannerIgnoresAMismatchedCloser(t *testing.T) {
    var fence codeFenceScanner

    fence.inside("```")
    if false == fence.inside("~~~") {
        t.Error("a different fence character must not close an open fence")
    }
    if false == fence.inside("still inside") {
        t.Error("the fence must stay open after a mismatched closer")
    }
    fence.inside("```")
    if true == fence.inside("out") {
        t.Error("a matching closer must end the fence")
    }
}

func FuzzCompareSemver(f *testing.F) {
    seeds := []string{
        "v1.0.0", "v1.0.1", "v1.10.0", "v1.9.0", "v4.1.9", "v4.1.12",
        "v1.0.0-rc1", "v1.0.0-rc.2", "v1.0.0-rc.10", "v1.0.0+build.5",
        "integrations/bunorm/v1.0.0", "integrations/bunorm/mysql/v1.1.5",
        "", "v", "v1", "v1.2", "v1.2.3.4", "v-1.0.0", "vx.y.z",
    }
    for _, left := range seeds {
        for _, right := range seeds {
            f.Add(left, right, "v2.0.0")
        }
    }

    f.Fuzz(func(t *testing.T, left string, right string, third string) {
        if 0 != compareSemver(left, left) {
            t.Fatalf("compareSemver is not reflexive for %q", left)
        }

        forward := compareSign(compareSemver(left, right))
        backward := compareSign(compareSemver(right, left))
        if forward != -backward {
            t.Fatalf("compareSemver is not antisymmetric for %q / %q: %d vs %d", left, right, forward, backward)
        }

        leftMiddle := compareSign(compareSemver(left, right))
        middleRight := compareSign(compareSemver(right, third))
        if leftMiddle == middleRight && 0 != leftMiddle {
            if leftMiddle != compareSign(compareSemver(left, third)) {
                t.Fatalf(
                    "compareSemver is not transitive for %q, %q, %q",
                    left, right, third,
                )
            }
        }
    })
}

func compareSign(value int) int {
    if 0 > value {
        return -1
    }
    if 0 < value {
        return 1
    }

    return 0
}

/* semverParts zeroes a segment it cannot parse, so non-negative is the whole guarantee */
func FuzzSemverParts(f *testing.F) {
    for _, seed := range []string{"v1.2.3", "v1.2.3-rc1", "v1.2.3+build", "", "v...", "v1.2.3.4.5", "-1.-2.-3", "v99999999999999999999.0.0"} {
        f.Add(seed)
    }

    f.Fuzz(func(t *testing.T, tag string) {
        parts := semverParts(tag)
        for index, part := range parts {
            if 0 > part {
                t.Fatalf("semverParts(%q)[%d] = %d, want a non-negative segment", tag, index, part)
            }
        }
    })
}

/* both halves are interpolated into a github API path, so neither may carry a separator */
func FuzzParseGithubUrl(f *testing.F) {
    for _, seed := range []string{
        "https://github.com/precision-soft/doctrine-type",
        "https://github.com/precision-soft/doctrine-type.git",
        "git@github.com:precision-soft/doctrine-type.git",
        "ssh://git@github.com/precision-soft/doctrine-type",
        "github.com/precision-soft/doctrine-type/",
        "", "/", "//", "https://", "git@", "a/b/c/d/e",
    } {
        f.Add(seed)
    }

    f.Fuzz(func(t *testing.T, url string) {
        organization, repository := parseGithubUrl(url)

        if true == strings.Contains(organization, "/") {
            t.Fatalf("parseGithubUrl(%q) organization %q carries a separator", url, organization)
        }
        if true == strings.Contains(repository, "/") {
            t.Fatalf("parseGithubUrl(%q) repository %q carries a separator", url, repository)
        }
    })
}

/* the entry is pushed verbatim into a release body, so it must stay a piece of its own changelog */
func FuzzExtractChangelogEntry(f *testing.F) {
    f.Add("# Changelog\n\n## [v1.0.0] - 2026-01-01 - Title\n\n### Added\n\n- one\n", "v1.0.0")
    f.Add("## v1.0.0\n\n### Added\n\n- one\n", "v1.0.0")
    f.Add("## [v2.0.0] - 2026-01-01\n\n- a\n\n## [v1.0.0] - 2025-01-01\n\n- b\n\n[v1.0.0]: https://x/compare/a...b\n", "v1.0.0")
    f.Add("", "v1.0.0")

    f.Fuzz(func(t *testing.T, content string, version string) {
        body, found := extractChangelogEntry(content, version)
        if false == found {
            return
        }

        if false == strings.Contains(content, body) {
            t.Fatalf("extractChangelogEntry(%q) returned %q, which is not part of the content", version, body)
        }
    })
}

func TestMarkdownHeading(t *testing.T) {
    cases := []struct {
        name      string
        line      string
        wantLevel int
        wantText  string
    }{
        {"plain h2", "## Fixed", 2, "Fixed"},
        {"plain h3", "### Security", 3, "Security"},
        {"indented by one", " ### Security", 3, "Security"},
        {"indented by three", "   ### Security", 3, "Security"},
        {"indented by four is code", "    ### Security", 0, ""},
        {"tab indent is code", "\t### Security", 0, ""},
        {"extra inner spacing", "###   Security  ", 3, "Security"},
        {"no space after hashes", "###Security", 0, ""},
        {"no text", "###", 0, ""},
        {"only spaces after hashes", "###   ", 0, ""},
        {"seven hashes", "####### Deep", 0, ""},
        {"not a heading", "- a bullet", 0, ""},
        {"empty", "", 0, ""},
    }

    for _, testCase := range cases {
        level, text := markdownHeading(testCase.line)
        if level != testCase.wantLevel || text != testCase.wantText {
            t.Errorf(
                "%s: markdownHeading(%q) = (%d, %q), want (%d, %q)",
                testCase.name, testCase.line, level, text, testCase.wantLevel, testCase.wantText,
            )
        }
    }
}
