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
