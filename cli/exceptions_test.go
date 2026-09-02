package cli

import (
    "fmt"
    "reflect"
    "testing"
    "time"

    "github.com/precision-soft/git-audit/config/project"
    "github.com/precision-soft/git-audit/types"
)

func auditWithLevelResult(level string, result types.LevelResult) []types.ProjectAudit {
    release := types.ReleaseAudit{TagName: "v1.0.0"}
    *releaseLevels(&release)[level] = result

    audit := types.ProjectAudit{
        OrganizationName: "precision-soft",
        RepositoryName:   "doctrine-type",
        Releases:         []types.ReleaseAudit{release},
    }
    audit.Releases[0].Status = computeReleaseStatus(audit.Releases[0])

    return []types.ProjectAudit{audit}
}

func exceptionsFor(level string, issue string) Exceptions {
    exceptions := make(Exceptions)
    exceptions.add("precision-soft/doctrine-type", "v1.0.0", level, issue)

    return exceptions
}

func TestApplyExceptionsClearsAnAcceptedWarning(t *testing.T) {
    const issue = `title summary "lowercase" must start with uppercase`

    audits := auditWithLevelResult("presentation", types.LevelResult{
        Status: types.LevelWarning,
        Issues: []string{issue},
    })

    applyExceptions(audits, exceptionsFor("presentation", issue))

    result := releaseLevels(&audits[0].Releases[0])["presentation"]
    if types.LevelOk != result.Status {
        t.Errorf("expected the accepted warning to leave the level ok, got %q", result.Status)
    }
    if 0 != len(result.Issues) {
        t.Errorf("expected the accepted warning to be dropped, got %v", result.Issues)
    }
    if types.StatusOk != audits[0].Status {
        t.Errorf("expected the project to be ok, got %q", audits[0].Status)
    }
}

func TestApplyExceptionsLeavesAFailedLevelIntact(t *testing.T) {
    const issue = "tag has no matching release"

    audits := auditWithLevelResult("integrity", types.LevelResult{
        Status: types.LevelFailed,
        Issues: []string{issue},
    })

    applyExceptions(audits, exceptionsFor("integrity", issue))

    result := releaseLevels(&audits[0].Releases[0])["integrity"]
    if types.LevelFailed != result.Status {
        t.Errorf("expected the failed level to stay failed, got %q", result.Status)
    }
    if 1 != len(result.Issues) || issue != result.Issues[0] {
        t.Fatalf("expected the failure to keep its issue, got %v", result.Issues)
    }
}

func TestApplyExceptionsKeepsTheWarningsItDoesNotMatch(t *testing.T) {
    const accepted = "non-standard section: ## Notes"
    const kept = `title summary "lowercase" must start with uppercase`

    audits := auditWithLevelResult("presentation", types.LevelResult{
        Status: types.LevelWarning,
        Issues: []string{accepted, kept},
    })

    applyExceptions(audits, exceptionsFor("presentation", accepted))

    result := releaseLevels(&audits[0].Releases[0])["presentation"]
    if types.LevelWarning != result.Status {
        t.Errorf("expected the level to stay a warning, got %q", result.Status)
    }
    if 1 != len(result.Issues) || kept != result.Issues[0] {
        t.Fatalf("expected only the unmatched warning to remain, got %v", result.Issues)
    }
}

func TestExceptionEntryActiveHonoursTheReviewDate(t *testing.T) {
    reviewedUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
    entry := ExceptionEntry{Issue: "something", ReviewedUntil: reviewedUntil}

    cases := []struct {
        name string
        now  time.Time
        want bool
    }{
        {"before", reviewedUntil.AddDate(0, 0, -1), true},
        {"on the day", reviewedUntil, true},
        {"after", reviewedUntil.AddDate(0, 0, 1), false},
    }

    for _, testCase := range cases {
        if got := entry.Active(testCase.now); got != testCase.want {
            t.Errorf("%s: Active(%s) = %t, want %t", testCase.name, testCase.now.Format(exceptionDateLayout), got, testCase.want)
        }
    }

    if false == (ExceptionEntry{Issue: "something"}).Active(time.Now()) {
        t.Error("an entry without a review date must never expire")
    }
}

func TestParseSelection(t *testing.T) {
    cases := []struct {
        name  string
        input string
        max   int
        want  []int
    }{
        {"single", "2", 5, []int{1}},
        {"several", "1 3 5", 5, []int{0, 2, 4}},
        {"range", "2-4", 5, []int{1, 2, 3}},
        {"all", "all", 3, []int{0, 1, 2}},
        {"deduplicated", "2 2 2-3", 5, []int{1, 2}},
        {"out of range dropped", "0 6 3", 5, []int{2}},
        {"garbage dropped", "abc 2", 5, []int{1}},
    }

    for _, testCase := range cases {
        got := parseSelection(testCase.input, testCase.max)
        if len(got) != len(testCase.want) {
            t.Errorf("%s: parseSelection(%q) = %v, want %v", testCase.name, testCase.input, got, testCase.want)
            continue
        }
        for index := range got {
            if got[index] != testCase.want[index] {
                t.Errorf("%s: parseSelection(%q) = %v, want %v", testCase.name, testCase.input, got, testCase.want)
                break
            }
        }
    }
}

func TestRecomputeProjectAggregatesKeepsAFetchErrorAudit(t *testing.T) {
    audit := buildFetchErrorAudit(project.ProjectConfig{GithubUrl: "https://github.com/acme/widget"}, fmt.Errorf("get tags: http 500"))
    expected := audit

    recomputeProjectAggregates(&audit)

    if false == reflect.DeepEqual(expected, audit) {
        t.Fatalf("a fetch-error audit has nothing to aggregate and must stay as built, got %#v", audit)
    }
}
