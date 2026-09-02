package cli

import (
    "bytes"
    "encoding/json"
    "fmt"
    "strings"
    "testing"
    "time"

    "github.com/precision-soft/git-audit/config/project"
    "github.com/precision-soft/git-audit/service"
    "github.com/precision-soft/git-audit/types"

    clicontract "github.com/precision-soft/melody/v3/cli/contract"
    "github.com/precision-soft/melody/v3/cli/output"
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
        if 0 > got {
            gotSign = -1
        } else if 0 < got {
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

func TestExtractChangelogEntryStripsHeadingTail(t *testing.T) {
    content := "# Changelog\n\n" +
        "## [v1.2.3] - 2026-04-20 - Some Release Title\n\n" +
        "### Changed\n\n- actual change\n"

    body, found := extractChangelogEntry(content, "v1.2.3")
    if false == found {
        t.Fatalf("expected to find v1.2.3 entry")
    }
    if true == strings.Contains(body, "2026-04-20") {
        t.Errorf("body should not contain heading date, got %q", body)
    }
    if true == strings.Contains(body, "Some Release Title") {
        t.Errorf("body should not contain heading title, got %q", body)
    }
    if false == strings.HasPrefix(body, "### Changed") {
        t.Errorf("body should start at the first section heading, got %q", body)
    }
}

func TestExtractChangelogEntryStripsTrailingLinkReferences(t *testing.T) {
    content := "# Changelog\n\n" +
        "## [v2.0.0] - 2026-03-01 - Big Refactor\n\n" +
        "### Changed\n\n- Big change\n\n" +
        "## [v1.0.0] - 2025-12-01 - Initial release\n\n" +
        "### Added\n\n- Initial release\n\n" +
        "[Unreleased]: https://github.com/example/repo/compare/v2.0.0...HEAD\n\n" +
        "[v2.0.0]: https://github.com/example/repo/compare/v1.0.0...v2.0.0\n\n" +
        "[v1.0.0]: https://github.com/example/repo/releases/tag/v1.0.0\n"

    body, found := extractChangelogEntry(content, "v1.0.0")
    if false == found {
        t.Fatalf("expected to find v1.0.0 entry")
    }
    if true == strings.Contains(body, "[Unreleased]:") {
        t.Errorf("v1.0.0 body should not absorb [Unreleased] link ref, got %q", body)
    }
    if true == strings.Contains(body, "[v2.0.0]:") {
        t.Errorf("v1.0.0 body should not absorb [v2.0.0] link ref, got %q", body)
    }
    if true == strings.Contains(body, "[v1.0.0]:") {
        t.Errorf("v1.0.0 body should not absorb its own [v1.0.0] link ref, got %q", body)
    }
    if false == strings.Contains(body, "Initial release") {
        t.Errorf("v1.0.0 body should still contain the actual content, got %q", body)
    }
    if false == strings.HasSuffix(strings.TrimSpace(body), "- Initial release") {
        t.Errorf("v1.0.0 body should end at the last content bullet, got %q", body)
    }
}

func TestExtractChangelogEntryHandlesV1HeadingFormat(t *testing.T) {
    content := "# Changelog\n\n" +
        "## v1.0.0\n\n" +
        "### Added\n\n- initial\n"

    body, found := extractChangelogEntry(content, "v1.0.0")
    if false == found {
        t.Fatalf("expected to find v1.0.0 entry")
    }
    if false == strings.HasPrefix(body, "### Added") {
        t.Errorf("body should start at the first section heading, got %q", body)
    }
}

func TestAuditPresentationFlagsTrailingLinkReferences(t *testing.T) {
    body := "## Added\n\n" +
        "- Initial public release\n\n" +
        "[Unreleased]: https://github.com/example/repo/compare/v2.0.0...HEAD\n\n" +
        "[v2.0.0]: https://github.com/example/repo/compare/v1.0.0...v2.0.0\n\n" +
        "[v1.0.0]: https://github.com/example/repo/releases/tag/v1.0.0\n"

    result := auditPresentation("Symfony PHPUnit", "v1.0.0", "Symfony PHPUnit v1.0.0 - Initial release", body)

    if types.LevelWarning != result.Status {
        t.Fatalf("expected warning for trailing link refs, got status=%v issues=%v", result.Status, result.Issues)
    }

    foundLinkRefIssue := false
    for _, issue := range result.Issues {
        if true == strings.Contains(issue, "trailing") || true == strings.Contains(issue, "link reference") {
            foundLinkRefIssue = true
            break
        }
    }
    if false == foundLinkRefIssue {
        t.Errorf("expected a trailing link-reference issue, got %v", result.Issues)
    }
}

func TestAuditPresentationCleanBodyHasNoLinkRefIssue(t *testing.T) {
    body := "## Added\n\n- Initial public release\n"

    result := auditPresentation("Symfony PHPUnit", "v1.0.0", "Symfony PHPUnit v1.0.0 - Initial release", body)

    for _, issue := range result.Issues {
        if true == strings.Contains(issue, "link reference") {
            t.Errorf("clean body should not emit a link-reference issue, got %q", issue)
        }
    }
}

func TestHasTrailingChangelogLinkReferencesSkipsMidBody(t *testing.T) {
    body := "## Added\n\n" +
        "- See [v1.0.0]: https://example.com for details\n" +
        "- another bullet\n"

    if true == hasTrailingChangelogLinkReferences(body) {
        t.Errorf("link-like text inside a bullet should not trigger trailing-refs detection: %q", body)
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

func TestClassifyDiffBehindIsNotApplicable(t *testing.T) {
    response := &service.CompareResponse{
        Status:       "behind",
        TotalCommits: 0,
    }

    result := classifyDiff(response, "v1.12.0", nil)

    if types.LevelNotApplicable != result.Status {
        t.Fatalf("expected LevelNotApplicable for behind comparison, got %v (issues: %v)", result.Status, result.Issues)
    }
    if 0 != len(result.Issues) {
        t.Errorf("expected no issues for behind comparison, got %v", result.Issues)
    }
}

func TestClassifyDiffIdenticalIsNoCodeChanges(t *testing.T) {
    response := &service.CompareResponse{
        Status:       "identical",
        TotalCommits: 0,
    }

    result := classifyDiff(response, "v1.9.0", nil)

    if types.LevelFailed != result.Status {
        t.Fatalf("expected LevelFailed for identical comparison (lock-step), got %v", result.Status)
    }
    if 1 != len(result.Issues) || false == strings.Contains(result.Issues[0], "no code changes") {
        t.Errorf("expected 'no code changes' issue, got %v", result.Issues)
    }
}

func TestClassifyDiffNormalAheadIsOk(t *testing.T) {
    response := &service.CompareResponse{
        Status:       "ahead",
        TotalCommits: 3,
        Files: []service.CompareFile{
            {Filename: "application/application_http.go", Status: "modified"},
        },
    }

    result := classifyDiff(response, "v1.11.0", nil)

    if types.LevelOk != result.Status {
        t.Fatalf("expected LevelOk for normal ahead comparison, got %v (issues: %v)", result.Status, result.Issues)
    }
    if 3 != result.CommitCount {
        t.Errorf("expected CommitCount=3, got %d", result.CommitCount)
    }
    if 1 != len(result.ChangedFiles) {
        t.Errorf("expected 1 changed file, got %v", result.ChangedFiles)
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

func TestNonStandardSectionsAcceptsEveryKeepAChangelogSection(t *testing.T) {
    for _, section := range []string{"Added", "Changed", "Deprecated", "Removed", "Fixed", "Security"} {
        body := "## " + section + "\n\n- Something happened\n"
        if found := nonStandardSections(body); 0 != len(found) {
            t.Fatalf("expected the Keep a Changelog section %q to be accepted, got flagged: %v", section, found)
        }
    }
}

func TestNonStandardSectionsAcceptsTheProjectSections(t *testing.T) {
    for _, section := range []string{"Documentation", "Notes", "Breaking Changes", "Upgrade Notes", "Bug Fixes"} {
        body := "## " + section + "\n\n- Something happened\n"
        if found := nonStandardSections(body); 0 != len(found) {
            t.Fatalf("expected the project section %q to be accepted, got flagged: %v", section, found)
        }
    }
}

func TestNonStandardSectionsFlagsAnUnknownSection(t *testing.T) {
    found := nonStandardSections("## Added\n\n- One\n\n## Miscellaneous\n\n- Two\n")
    if 1 != len(found) || "## Miscellaneous" != found[0] {
        t.Fatalf("expected only the unknown section to be flagged, got %v", found)
    }
}

func auditWithTitleIssue() []types.ProjectAudit {
    return []types.ProjectAudit{
        {
            OrganizationName: "precision-soft",
            RepositoryName:   "doctrine-type",
            ProjectName:      "Doctrine Type",
            Releases: []types.ReleaseAudit{
                {
                    TagName:      "v1.0.0",
                    ReleaseTitle: "doctrine type v1.0.0 - lowercase summary",
                    Presentation: types.LevelResult{
                        Status: types.LevelWarning,
                        Issues: []string{`title summary "lowercase summary" must start with uppercase`},
                    },
                    Status: types.StatusWarning,
                },
            },
            PresentationStatus:  types.LevelWarning,
            PresentationDisplay: "warning (1)",
            Status:              types.StatusWarning,
        },
    }
}

func renderAuditOutputForFormat(t *testing.T, format output.Format, audits []types.ProjectAudit) string {
    t.Helper()

    option := output.NormalizeOption(output.DefaultOption())
    option.Format = format
    option.NoColor = true

    envelope := output.NewEnvelope(
        output.NewMeta("audit", nil, option, time.Now(), 0, output.Version{}),
    )
    envelope.Data = output.NewListPayload(audits, len(audits), option.Limit, option.Offset)
    if output.FormatTable == format {
        envelope.Table = buildAuditTable(audits, types.StatusOk, service.RateLimitInfo{})
    }

    var buffer bytes.Buffer
    if renderErr := renderAuditOutput(&buffer, envelope, option, audits); nil != renderErr {
        t.Fatalf("renderAuditOutput returned an error: %v", renderErr)
    }

    return buffer.String()
}

func TestRenderAuditOutputKeepsJsonParseable(t *testing.T) {
    rendered := renderAuditOutputForFormat(t, output.FormatJson, auditWithTitleIssue())

    var decoded any
    if unmarshalErr := json.Unmarshal([]byte(rendered), &decoded); nil != unmarshalErr {
        t.Fatalf("json output is not parseable (%v), rendered:\n%s", unmarshalErr, rendered)
    }

    if true == strings.Contains(rendered, "TITLE FIXES:") {
        t.Errorf("the title-fix block leaked into the json output:\n%s", rendered)
    }
}

func TestRenderAuditOutputKeepsTitleFixesInTheTableFormat(t *testing.T) {
    rendered := renderAuditOutputForFormat(t, output.FormatTable, auditWithTitleIssue())

    if false == strings.Contains(rendered, "TITLE FIXES:") {
        t.Fatalf("expected the title-fix block in the table output, got:\n%s", rendered)
    }
    if false == strings.Contains(rendered, "Doctrine Type v1.0.0 - Lowercase summary") {
        t.Errorf("expected the corrected title in the table output, got:\n%s", rendered)
    }
}

func TestSortReleaseAuditsOrdersBySemverNotLexically(t *testing.T) {
    releaseAudits := []types.ReleaseAudit{
        {TagName: "v4.1.9"},
        {TagName: "v4.1.12"},
        {TagName: "v4.1.10"},
        {TagName: "v10.0.0"},
        {TagName: "v2.0.0"},
        {TagName: "v2.0.0-rc1"},
    }

    sortReleaseAudits(releaseAudits)

    want := []string{"v2.0.0-rc1", "v2.0.0", "v4.1.9", "v4.1.10", "v4.1.12", "v10.0.0"}
    for index, expected := range want {
        if releaseAudits[index].TagName != expected {
            got := make([]string, 0, len(releaseAudits))
            for _, releaseAudit := range releaseAudits {
                got = append(got, releaseAudit.TagName)
            }
            t.Fatalf("sortReleaseAudits ordered %v, want %v", got, want)
        }
    }
}

func TestNonStandardSectionsIgnoresFencedCodeBlocks(t *testing.T) {
    body := "## Added\n\n" +
        "- Support for a new heading\n\n" +
        "```markdown\n" +
        "## Miscellaneous\n" +
        "```\n\n" +
        "## Fixed\n\n- Something\n"

    if found := nonStandardSections(body); 0 != len(found) {
        t.Fatalf("expected no section to be flagged, got %v", found)
    }
}

/* flagging a mis-cased or decorated heading is deliberate, not an oversight to relax */
func TestNonStandardSectionsFlagsMisCasedAndDecoratedHeadings(t *testing.T) {
    for _, section := range []string{"## fixed", "## FIXED", "## Fixed (3)"} {
        found := nonStandardSections(section + "\n\n- Something\n")
        if 1 != len(found) || section != found[0] {
            t.Errorf("expected %q to be flagged, got %v", section, found)
        }
    }
}

func TestTableIssuesListSupplyChainIssues(t *testing.T) {
    rendered := renderAuditOutputForFormat(t, output.FormatTable, auditWithLevelResult("supply-chain", types.LevelResult{
        Status: types.LevelWarning,
        Issues: []string{"release has no sbom artifact"},
    }))

    if false == strings.Contains(rendered, "release has no sbom artifact") {
        t.Fatalf("expected the supply-chain issue in the table output, got:\n%s", rendered)
    }
}

func TestSupplyChainStatusIsNotApplicableWhenNotRequested(t *testing.T) {
    audit := buildProjectAudit("acme", "widget", "Widget", "", -1, 1, 1, 0, []types.ReleaseAudit{{TagName: "v1.0.0"}})

    if types.LevelNotApplicable != audit.SupplyChainStatus {
        t.Fatalf("a check that never ran is n/a, not %q", audit.SupplyChainStatus)
    }
    if types.LevelNotApplicable != audit.Releases[0].SupplyChain.Status {
        t.Fatalf("the release level of a check that never ran is n/a, not %q", audit.Releases[0].SupplyChain.Status)
    }

    recomputeProjectAggregates(audit)

    if types.LevelNotApplicable != audit.SupplyChainStatus {
        t.Fatalf("re-aggregating must keep n/a like it keeps distribution's, got %q", audit.SupplyChainStatus)
    }
}

func TestCacheIsOffByDefault(t *testing.T) {
    for _, flag := range (&AuditCommand{}).Flags() {
        stringFlag, isStringFlag := flag.(*clicontract.StringFlag)
        if false == isStringFlag || flagCacheDir != stringFlag.Name {
            continue
        }
        if "" != stringFlag.Value {
            t.Fatalf("--cache-dir must be opt-in, got the default %q", stringFlag.Value)
        }
        return
    }
    t.Fatal("the --cache-dir flag is not declared")
}

func TestConcurrencyOutOfRangeIsRejected(t *testing.T) {
    for _, value := range []int{0, -1, 33} {
        if nil == validateConcurrency(value) {
            t.Errorf("--concurrency=%d must be rejected", value)
        }
    }
    for _, value := range []int{1, 4, 32} {
        if validateErr := validateConcurrency(value); nil != validateErr {
            t.Errorf("--concurrency=%d must be accepted, got %v", value, validateErr)
        }
    }
}

func TestAuditProjectsParallelAuditsEveryProject(t *testing.T) {
    routes := make(map[string]string)
    var projects []project.ProjectConfig
    for index := 0; 40 > index; index++ {
        repository := fmt.Sprintf("widget-%d", index)
        routes["/repos/acme/"+repository+"/tags"] = `[]`
        routes["/repos/acme/"+repository+"/releases"] = `[]`
        projects = append(projects, project.ProjectConfig{Name: repository, GithubUrl: "https://github.com/acme/" + repository})
    }
    client, _ := fakeGithubApi(t, routes)

    audits := auditProjectsParallel(client, projects, 32)

    if len(projects) != len(audits) {
        t.Fatalf("expected %d audits, got %d", len(projects), len(audits))
    }
    for index, audit := range audits {
        if projects[index].Name != audit.RepositoryName || "" != audit.FetchError {
            t.Errorf("slot %d must hold %s without a fetch error, got %s / %q", index, projects[index].Name, audit.RepositoryName, audit.FetchError)
        }
    }
}
