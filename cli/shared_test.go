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
