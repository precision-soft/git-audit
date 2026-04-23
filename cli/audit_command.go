package cli

import (
    "fmt"
    "io"
    "regexp"
    "sort"
    "strings"
    "sync"
    "time"
    "unicode"

    "github.com/precision-soft/git-audit/config/project"
    "github.com/precision-soft/git-audit/service"
    "github.com/precision-soft/git-audit/types"

    clicontract "github.com/precision-soft/melody/v3/cli/contract"
    "github.com/precision-soft/melody/v3/cli/output"
    runtimecontract "github.com/precision-soft/melody/v3/runtime/contract"
)

const (
    flagToken         = "token"
    flagRepo          = "repo"
    flagFailOnWarning = "fail-on-warning"
    flagExceptions    = "exceptions"

    auditParallelism = 4
)

var (
    versionRegex                = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
    titlePartsRegex             = regexp.MustCompile(`^(.+?)\s+(v\d+\.\d+\.\d+)\s+-\s+(.+)$`)
    changelogDatedTitledHeading = regexp.MustCompile(`(?m)^##\s+\[(v\d+\.\d+\.\d+)\]\s+-\s+(\d{4}-\d{2}-\d{2})\s+-\s+(.+?)\s*$`)
    changelogDatedHeading       = regexp.MustCompile(`(?m)^##\s+\[(v\d+\.\d+\.\d+)\]\s+-\s+(\d{4}-\d{2}-\d{2})\s*$`)
    changelogCompareLink        = regexp.MustCompile(`(?m)^\[(v\d+\.\d+\.\d+)\]:\s*https?://\S+/compare/\S+`)
    shaHexRegex                 = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

    standardSections = map[string]bool{
        "## Fixed":            true,
        "## Changed":          true,
        "## Added":            true,
        "## Notes":            true,
        "## Breaking Changes": true,
        "## Upgrade Notes":    true,
        "## Bug Fixes":        true,
        "## Deprecated":       true,
    }
)

type PackagistAuditInfo struct {
    Versions map[string]service.PackagistVersion
    Count    int
    Err      error
}

type ChangelogAuditResult struct {
    Status         types.LevelStatus
    Issues         []string
    Path           string
    HasVersion     bool
    EntryBody      string
    NormalizedBody string
    HeadingTitle   string
}

type ChangelogSource struct {
    Content string
    Err     error
}

type DiffAuditResult struct {
    Status       types.LevelStatus
    Issues       []string
    PreviousTag  string
    CommitCount  int
    ChangedFiles []string
}

type AuditCommand struct{}

func (command *AuditCommand) Name() string {
    return "audit"
}

func (command *AuditCommand) Description() string {
    return "Audit GitHub releases: tag/release 1:1, changelog, code diff, Packagist sync"
}

func (command *AuditCommand) Flags() []clicontract.Flag {
    return output.MergeFlags(
        output.DebugFlags(),
        []clicontract.Flag{
            &clicontract.StringFlag{
                Name:  flagToken,
                Usage: "GitHub token (overrides GITHUB_TOKEN env)",
                Value: "",
            },
            &clicontract.StringFlag{
                Name:  flagRepo,
                Usage: "filter by repo name; comma-separated for multiple (e.g. doctrine-utility or doctrine-type,doctrine-utility)",
                Value: "",
            },
            &clicontract.StringFlag{
                Name:  flagRepoUrl,
                Usage: "github URL for a repo not in the built-in project list (required when --repo is unknown)",
                Value: "",
            },
            &clicontract.BoolFlag{
                Name:  flagFailOnWarning,
                Usage: "exit with code 1 on warning, not only on failed",
                Value: false,
            },
            &clicontract.StringFlag{
                Name:  flagExceptions,
                Usage: "path to exceptions JSON file",
                Value: "exceptions.json",
            },
        },
    )
}

func (command *AuditCommand) Run(
    runtimeInstance runtimecontract.Runtime,
    commandContext *clicontract.CommandContext,
) error {
    startedAt := time.Now()

    repositoryFilter := strings.TrimSpace(commandContext.String(flagRepo))
    repositoryUrl := strings.TrimSpace(commandContext.String(flagRepoUrl))
    failOnWarning := commandContext.Bool(flagFailOnWarning)
    exceptionsFile := strings.TrimSpace(commandContext.String(flagExceptions))

    exceptions, loadErr := loadExceptions(exceptionsFile)
    if nil != loadErr {
        return fmt.Errorf("load exceptions: %w", loadErr)
    }

    githubClient, resolveErr := resolveGithubClient(runtimeInstance, commandContext)
    if nil != resolveErr {
        return resolveErr
    }
    projects, resolveErr := resolveTargetProjects(repositoryFilter, repositoryUrl)
    if nil != resolveErr {
        return resolveErr
    }
    option := output.NormalizeOption(output.ParseOptionFromCommand(commandContext))
    if 1 > option.TableMaxWidth {
        option.TableMaxWidth = autoTableMaxWidth()
    }

    audits := auditProjectsParallel(githubClient, projects, auditParallelism)

    applyExceptions(audits, exceptions)

    hasFailure := false
    hasWarning := false
    for _, audit := range audits {
        switch audit.Status {
        case types.StatusFailed:
            hasFailure = true
        case types.StatusWarning:
            hasWarning = true
        }
    }

    globalStatus := types.StatusOk
    if true == hasFailure {
        globalStatus = types.StatusFailed
    } else if true == hasWarning {
        globalStatus = types.StatusWarning
    }

    meta := output.NewMeta(
        command.Name(),
        commandContext.Args().Slice(),
        option,
        startedAt,
        time.Since(startedAt),
        output.Version{},
    )

    envelope := output.NewEnvelope(meta)

    if option.Format == output.FormatTable {
        builder := output.NewTableBuilder()
        builder.AddSummaryLine(fmt.Sprintf("projects: %d | status: %s", len(audits), globalStatus))

        if rateLimitLine := formatRateLimitLine(githubClient.RateLimit()); "" != rateLimitLine {
            builder.AddSummaryLine(rateLimitLine)
        }

        summaryBlock := builder.AddBlock(
            "SUMMARY",
            []string{"repo", "tags", "submod", "releases", "packagist", "integrity", "distribution", "changelog", "diff", "presentation", "status"},
        )

        for _, audit := range audits {
            packagistColumn := "-"
            if audit.PackagistCount >= 0 {
                packagistColumn = fmt.Sprintf("%d", audit.PackagistCount)
            } else if "" == audit.PackagistPackage {
                packagistColumn = "n/a"
            }

            submoduleColumn := "-"
            if audit.SubmoduleTagCount > 0 {
                submoduleColumn = fmt.Sprintf("%d", audit.SubmoduleTagCount)
            }

            changelogStatus := "-"
            if "" != audit.ChangelogDisplay {
                changelogStatus = audit.ChangelogDisplay
            }

            diffStatus := "-"
            if "" != audit.DiffDisplay {
                diffStatus = audit.DiffDisplay
            }

            summaryBlock.AddRow(
                audit.OrganizationName+"/"+audit.RepositoryName,
                fmt.Sprintf("%d", audit.TagCount),
                submoduleColumn,
                fmt.Sprintf("%d", audit.ReleaseCount),
                packagistColumn,
                string(audit.IntegrityStatus),
                string(audit.DistributionStatus),
                changelogStatus,
                diffStatus,
                audit.PresentationDisplay,
                string(audit.Status),
            )
        }

        var fetchErrorAudits []types.ProjectAudit
        for _, audit := range audits {
            if "" != audit.FetchError {
                fetchErrorAudits = append(fetchErrorAudits, audit)
            }
        }

        if len(fetchErrorAudits) > 0 {
            fetchBlock := builder.AddBlock("FETCH ERRORS", []string{"repo", "error"})
            for _, audit := range fetchErrorAudits {
                fetchBlock.AddRow(
                    audit.OrganizationName+"/"+audit.RepositoryName,
                    audit.FetchError,
                )
            }
        }

        type issueRow struct {
            version  string
            level    string
            issue    string
            priority int
        }

        repositoryIssueMap := make(map[string][]issueRow)
        for _, audit := range audits {
            repositoryKey := audit.OrganizationName + "/" + audit.RepositoryName
            for _, release := range audit.Releases {
                for _, issue := range release.Integrity.Issues {
                    repositoryIssueMap[repositoryKey] = append(repositoryIssueMap[repositoryKey], issueRow{release.TagName, "integrity", issue, 0})
                }
                for _, issue := range release.Distribution.Issues {
                    repositoryIssueMap[repositoryKey] = append(repositoryIssueMap[repositoryKey], issueRow{release.TagName, "distribution", issue, 1})
                }
                for _, issue := range release.Changelog.Issues {
                    repositoryIssueMap[repositoryKey] = append(repositoryIssueMap[repositoryKey], issueRow{release.TagName, "changelog", issue, 2})
                }
                for _, issue := range release.Diff.Issues {
                    repositoryIssueMap[repositoryKey] = append(repositoryIssueMap[repositoryKey], issueRow{release.TagName, "diff", issue, 3})
                }
                for _, issue := range release.Presentation.Issues {
                    repositoryIssueMap[repositoryKey] = append(repositoryIssueMap[repositoryKey], issueRow{release.TagName, "presentation", issue, 4})
                }
            }
        }

        seen := make(map[string]bool)
        for _, audit := range audits {
            repositoryKey := audit.OrganizationName + "/" + audit.RepositoryName
            rows, hasIssues := repositoryIssueMap[repositoryKey]
            if false == hasIssues || true == seen[repositoryKey] {
                continue
            }
            seen[repositoryKey] = true

            sort.SliceStable(rows, func(leftIndex, rightIndex int) bool {
                if rows[leftIndex].priority != rows[rightIndex].priority {
                    return rows[leftIndex].priority < rows[rightIndex].priority
                }
                return rows[leftIndex].version < rows[rightIndex].version
            })

            block := builder.AddBlock(repositoryKey, []string{"version", "level", "issue"})
            for _, row := range rows {
                block.AddRow(row.version, row.level, row.issue)
            }
        }

        envelope.Table = builder.Build()
    } else {
        envelope.Data = output.NewListPayload(audits, len(audits), option.Limit, option.Offset)
    }

    envelope.Meta.DurationMilliseconds = time.Since(startedAt).Milliseconds()

    if renderErr := output.Render(commandContext.Writer, envelope, option); nil != renderErr {
        return renderErr
    }

    printTitleFixes(commandContext.Writer, audits)

    pending := collectPendingWarnings(audits)
    modified, reviewErr := reviewWarningsInteractively(pending, exceptions)
    if nil != reviewErr {
        return fmt.Errorf("review warnings: %w", reviewErr)
    }
    if true == modified {
        if saveErr := saveExceptions(exceptionsFile, exceptions); nil != saveErr {
            return fmt.Errorf("save exceptions: %w", saveErr)
        }
    }

    if true == hasFailure {
        return fmt.Errorf("audit completed with failures")
    }

    if true == failOnWarning && true == hasWarning {
        return fmt.Errorf("audit completed with warnings")
    }

    return nil
}

func auditProjectsParallel(client *service.GithubClient, projects []project.ProjectConfig, parallelism int) []types.ProjectAudit {
    audits := make([]types.ProjectAudit, len(projects))

    if parallelism < 1 {
        parallelism = 1
    }
    semaphore := make(chan struct{}, parallelism)
    var waitGroup sync.WaitGroup

    for index, projectConfig := range projects {
        waitGroup.Add(1)
        semaphore <- struct{}{}
        go func(slot int, configuration project.ProjectConfig) {
            defer waitGroup.Done()
            defer func() { <-semaphore }()

            audit, auditErr := auditProject(client, configuration)
            if nil != auditErr {
                audits[slot] = buildFetchErrorAudit(configuration, auditErr)
                return
            }
            audits[slot] = *audit
        }(index, projectConfig)
    }

    waitGroup.Wait()
    return audits
}

func filterProjects(projects []project.ProjectConfig, repositoryFilter string) []project.ProjectConfig {
    if "" == repositoryFilter {
        return projects
    }

    var filtered []project.ProjectConfig
    for _, projectConfig := range projects {
        _, repository := parseGithubUrl(projectConfig.GithubUrl)
        if repository == repositoryFilter {
            filtered = append(filtered, projectConfig)
        }
    }

    return filtered
}

func auditProject(client *service.GithubClient, projectConfig project.ProjectConfig) (*types.ProjectAudit, error) {
    organization, repository := parseGithubUrl(projectConfig.GithubUrl)
    packagistPackage := parsePackagistUrl(projectConfig.PackagistUrl)

    projectName := projectConfig.Name
    if "" == projectName {
        projectName = deriveProjectName(repository)
    }

    tags, tagsErr := client.GetTags(organization, repository)
    if nil != tagsErr {
        return nil, fmt.Errorf("get tags: %w", tagsErr)
    }

    releases, releasesErr := client.GetReleases(organization, repository)
    if nil != releasesErr {
        return nil, fmt.Errorf("get releases: %w", releasesErr)
    }

    var regularTags []service.GithubTag
    submoduleTagCount := 0
    for _, tag := range tags {
        if true == projectConfig.GoSubmodule && true == isGoSubmoduleTag(tag.Name) {
            submoduleTagCount++
        } else {
            regularTags = append(regularTags, tag)
        }
    }

    sort.SliceStable(regularTags, func(leftIndex, rightIndex int) bool {
        return compareSemver(regularTags[leftIndex].Name, regularTags[rightIndex].Name) < 0
    })

    releaseMap := make(map[string]*service.GithubRelease, len(releases))
    for releaseIndex := range releases {
        releaseMap[releases[releaseIndex].TagName] = &releases[releaseIndex]
    }

    tagSet := make(map[string]bool, len(regularTags))
    for _, tag := range regularTags {
        tagSet[tag.Name] = true
    }

    packagistInfo := fetchPackagistVersions(packagistPackage)

    changelogPaths := resolveChangelogPaths(projectConfig)
    changelogSources := fetchChangelogSources(client, organization, repository, changelogPaths)

    var releaseAudits []types.ReleaseAudit

    for index, tag := range regularTags {
        previousTagName := ""
        if index > 0 {
            previousTagName = regularTags[index-1].Name
        }

        releaseAudit := auditRelease(
            client,
            organization,
            repository,
            tag,
            previousTagName,
            releaseMap[tag.Name],
            projectName,
            packagistPackage,
            packagistInfo,
            changelogPaths,
            changelogSources,
        )

        releaseAudits = append(releaseAudits, releaseAudit)
    }

    for tagName := range releaseMap {
        if false == tagSet[tagName] {
            releaseAudits = append(releaseAudits, types.ReleaseAudit{
                TagName: tagName,
                Integrity: types.LevelResult{
                    Status: types.LevelFailed,
                    Issues: []string{"release has no matching tag"},
                },
                Distribution: types.LevelResult{Status: types.LevelSkipped},
                Changelog:    types.LevelResult{Status: types.LevelSkipped},
                Diff:         types.LevelResult{Status: types.LevelSkipped},
                Presentation: types.LevelResult{Status: types.LevelSkipped},
                Status:       types.StatusFailed,
            })
        }
    }

    sort.SliceStable(releaseAudits, func(leftIndex, rightIndex int) bool {
        return releaseAudits[leftIndex].TagName < releaseAudits[rightIndex].TagName
    })

    return buildProjectAudit(
        organization,
        repository,
        projectName,
        packagistPackage,
        packagistInfo.Count,
        len(regularTags),
        len(releases),
        submoduleTagCount,
        releaseAudits,
    ), nil
}

func auditRelease(
    client *service.GithubClient,
    organization string,
    repository string,
    tag service.GithubTag,
    previousTagName string,
    release *service.GithubRelease,
    projectName string,
    packagistPackage string,
    packagistInfo PackagistAuditInfo,
    changelogPaths []string,
    changelogSources map[string]ChangelogSource,
) types.ReleaseAudit {
    releaseAudit := types.ReleaseAudit{
        TagName: tag.Name,
    }

    integrity := types.LevelResult{Status: types.LevelOk}

    if false == versionRegex.MatchString(tag.Name) {
        integrity.Status = types.LevelFailed
        integrity.Issues = append(integrity.Issues, "version format invalid (expected vX.Y.Z)")
    }

    if nil == release {
        integrity.Status = types.LevelFailed
        integrity.Issues = append(integrity.Issues, "tag has no matching release")
    } else {
        releaseAudit.ReleaseTitle = release.Name
        releaseAudit.ReleaseBody = release.Body

        if true == release.Draft {
            integrity.Status = types.LevelFailed
            integrity.Issues = append(integrity.Issues, "release is draft")
        }

        if true == release.Prerelease {
            integrity.Status = types.LevelFailed
            integrity.Issues = append(integrity.Issues, "release is prerelease")
        }

        if true == shaHexRegex.MatchString(release.TargetCommitish) && "" != tag.CommitSHA && release.TargetCommitish != tag.CommitSHA {
            if types.LevelFailed != integrity.Status {
                integrity.Status = types.LevelWarning
            }
            integrity.Issues = append(integrity.Issues, fmt.Sprintf(
                "release target_commitish %s does not match tag commit %s",
                shortSha(release.TargetCommitish), shortSha(tag.CommitSHA),
            ))
        }
    }

    releaseAudit.Integrity = integrity
    releaseAudit.Distribution = auditDistribution(tag, packagistPackage, packagistInfo)

    changelogAudit := auditChangelog(
        tag.Name,
        previousTagName,
        release,
        changelogPaths,
        changelogSources,
    )
    releaseAudit.Changelog = types.LevelResult{
        Status: changelogAudit.Status,
        Issues: changelogAudit.Issues,
    }

    diffAudit := auditDiff(
        client,
        organization,
        repository,
        previousTagName,
        tag.Name,
        release,
    )
    releaseAudit.Diff = types.LevelResult{
        Status: diffAudit.Status,
        Issues: diffAudit.Issues,
    }
    releaseAudit.PreviousTag = diffAudit.PreviousTag
    releaseAudit.CommitCount = diffAudit.CommitCount
    releaseAudit.ChangedFiles = diffAudit.ChangedFiles

    if nil == release {
        releaseAudit.Presentation = types.LevelResult{Status: types.LevelSkipped}
    } else {
        releaseAudit.Presentation = auditPresentation(
            projectName,
            tag.Name,
            release.Name,
            release.Body,
        )
    }

    releaseAudit.Status = computeReleaseStatus(releaseAudit)

    return releaseAudit
}

func auditDistribution(
    tag service.GithubTag,
    packagistPackage string,
    packagistInfo PackagistAuditInfo,
) types.LevelResult {
    if "" == packagistPackage {
        return types.LevelResult{Status: types.LevelNotApplicable}
    }

    if nil != packagistInfo.Err {
        return types.LevelResult{
            Status: types.LevelFailed,
            Issues: []string{fmt.Sprintf("could not fetch packagist: %s", packagistInfo.Err.Error())},
        }
    }

    normalizedVersion := strings.TrimPrefix(tag.Name, "v")
    packagistVersion, hasExact := packagistInfo.Versions[tag.Name]
    if false == hasExact {
        packagistVersion, hasExact = packagistInfo.Versions[normalizedVersion]
    }

    if false == hasExact {
        return types.LevelResult{
            Status: types.LevelFailed,
            Issues: []string{fmt.Sprintf("version %s not found in packagist", tag.Name)},
        }
    }

    tagReference := strings.TrimSpace(tag.CommitSHA)
    packagistReference := strings.TrimSpace(packagistVersion.Reference)

    if "" != tagReference && "" != packagistReference && false == strings.HasPrefix(packagistReference, tagReference) && false == strings.HasPrefix(tagReference, packagistReference) {
        return types.LevelResult{
            Status: types.LevelFailed,
            Issues: []string{fmt.Sprintf(
                "packagist reference %s does not match tag commit %s",
                packagistReference,
                tagReference,
            )},
        }
    }

    if "" == packagistReference {
        return types.LevelResult{
            Status: types.LevelWarning,
            Issues: []string{"packagist version exists but reference could not be determined"},
        }
    }

    return types.LevelResult{Status: types.LevelOk}
}

func auditChangelog(
    tagName string,
    previousTagName string,
    release *service.GithubRelease,
    changelogPaths []string,
    changelogSources map[string]ChangelogSource,
) ChangelogAuditResult {
    if 0 == len(changelogPaths) {
        return ChangelogAuditResult{
            Status: types.LevelNotApplicable,
        }
    }

    hasAnyContent := false
    for _, path := range changelogPaths {
        source := changelogSources[path]
        if nil == source.Err {
            hasAnyContent = true
            break
        }
    }

    if false == hasAnyContent {
        return ChangelogAuditResult{
            Status: types.LevelNotApplicable,
        }
    }

    var matchedResult *ChangelogAuditResult
    var missingPaths []string

    for _, changelogPath := range changelogPaths {
        source := changelogSources[changelogPath]
        if nil != source.Err {
            continue
        }

        entryBody, hasVersion := extractChangelogEntry(source.Content, tagName)
        if false == hasVersion {
            missingPaths = append(missingPaths, changelogPath)
            continue
        }

        result := &ChangelogAuditResult{
            Status:         types.LevelOk,
            Path:           changelogPath,
            HasVersion:     true,
            EntryBody:      entryBody,
            NormalizedBody: normalizeMarkdownBlock(entryBody),
        }

        applyChangelogDateAndLinkChecks(tagName, previousTagName, source.Content, changelogPath, result)

        if nil != release && "" != result.HeadingTitle {
            releaseTitleMatches := titlePartsRegex.FindStringSubmatch(strings.TrimSpace(release.Name))
            if nil != releaseTitleMatches {
                releaseSummary := strings.TrimSpace(releaseTitleMatches[3])
                if false == strings.EqualFold(releaseSummary, result.HeadingTitle) {
                    result.Status = maxLevelStatus(result.Status, types.LevelWarning)
                    result.Issues = append(result.Issues, fmt.Sprintf(
                        "changelog heading title %q does not match release title summary %q (in %s)",
                        result.HeadingTitle, releaseSummary, changelogPath,
                    ))
                }
            }
        }

        if nil == release {
            matchedResult = result
            break
        }

        normalizedReleaseBody := normalizeMarkdownBlock(release.Body)
        if "" == normalizedReleaseBody {
            result.Status = types.LevelWarning
            result.Issues = append(result.Issues, fmt.Sprintf("release body is empty but changelog entry exists in %s", changelogPath))
            matchedResult = result
            break
        }

        similarity := overlapRatio(normalizedReleaseBody, result.NormalizedBody)
        if similarity < 0.60 {
            result.Status = types.LevelWarning
            result.Issues = append(result.Issues, fmt.Sprintf(
                "release body and %s entry for %s differ significantly (overlap %.2f)",
                changelogPath,
                tagName,
                similarity,
            ))
        }

        matchedResult = result
        break
    }

    if nil != matchedResult {
        return *matchedResult
    }

    return ChangelogAuditResult{
        Status: types.LevelFailed,
        Issues: []string{fmt.Sprintf("version %s not found in %s", tagName, strings.Join(missingPaths, ", "))},
    }
}

func fetchChangelogSources(
    client *service.GithubClient,
    organization string,
    repository string,
    changelogPaths []string,
) map[string]ChangelogSource {
    sources := make(map[string]ChangelogSource, len(changelogPaths))

    for _, path := range changelogPaths {
        content, err := client.GetFileContentAtRef(organization, repository, path, "HEAD")
        sources[path] = ChangelogSource{
            Content: content,
            Err:     err,
        }
    }

    return sources
}

func classifyDiff(
    compareResponse *service.CompareResponse,
    previousTagName string,
    release *service.GithubRelease,
) DiffAuditResult {
    changedFiles := make([]string, 0, len(compareResponse.Files))
    for _, file := range compareResponse.Files {
        changedFiles = append(changedFiles, file.Filename)
    }

    result := DiffAuditResult{
        Status:       types.LevelOk,
        PreviousTag:  previousTagName,
        CommitCount:  compareResponse.TotalCommits,
        ChangedFiles: changedFiles,
    }

    if 0 == compareResponse.TotalCommits {
        if "behind" == compareResponse.Status {
            return DiffAuditResult{
                Status:      types.LevelNotApplicable,
                PreviousTag: previousTagName,
            }
        }
        result.Status = types.LevelFailed
        result.Issues = append(result.Issues, "release has no code changes compared to previous tag")
    }

    if nil != release {
        normalizedBody := normalizeMarkdownBlock(release.Body)
        if compareResponse.TotalCommits >= 3 && "" == normalizedBody {
            result.Status = maxLevelStatus(result.Status, types.LevelWarning)
            result.Issues = append(result.Issues, "release notes are empty for a non-trivial diff")
        }
    }

    return result
}

func auditDiff(
    client *service.GithubClient,
    organization string,
    repository string,
    previousTagName string,
    currentTagName string,
    release *service.GithubRelease,
) DiffAuditResult {
    if "" == previousTagName {
        return DiffAuditResult{
            Status:      types.LevelNotApplicable,
            PreviousTag: "",
        }
    }

    compareResponse, err := client.CompareTags(organization, repository, previousTagName, currentTagName)
    if nil != err {
        return DiffAuditResult{
            Status:      types.LevelFailed,
            PreviousTag: previousTagName,
            Issues: []string{
                fmt.Sprintf("could not compare %s...%s: %s", previousTagName, currentTagName, err.Error()),
            },
        }
    }

    return classifyDiff(compareResponse, previousTagName, release)
}

func auditPresentation(
    projectName string,
    tagName string,
    title string,
    body string,
) types.LevelResult {
    result := types.LevelResult{Status: types.LevelOk}

    trimmedTitle := strings.TrimSpace(title)
    matches := titlePartsRegex.FindStringSubmatch(trimmedTitle)
    if nil == matches {
        result.Status = types.LevelWarning
        result.Issues = append(result.Issues, "title format invalid (expected: <Name> vX.Y.Z - <Summary>)")
    } else {
        titleName := matches[1]
        titleVersion := matches[2]
        titleSummary := matches[3]

        if normalizeProjectName(titleName) != normalizeProjectName(projectName) {
            result.Status = types.LevelWarning
            result.Issues = append(result.Issues, fmt.Sprintf(
                "title project name %q does not match expected %q",
                titleName,
                projectName,
            ))
        }

        if titleVersion != tagName {
            result.Status = types.LevelWarning
            result.Issues = append(result.Issues, fmt.Sprintf(
                "title version %q does not match tag %q",
                titleVersion,
                tagName,
            ))
        }

        titleRunes := []rune(titleSummary)
        if len(titleRunes) > 0 && false == unicode.IsUpper(titleRunes[0]) {
            result.Status = types.LevelWarning
            result.Issues = append(result.Issues, fmt.Sprintf(
                "title summary %q must start with uppercase",
                titleSummary,
            ))
        }
    }

    for _, section := range nonStandardSections(body) {
        result.Status = types.LevelWarning
        result.Issues = append(result.Issues, fmt.Sprintf("non-standard section: %s", section))
    }

    if true == hasTrailingChangelogLinkReferences(body) {
        result.Status = maxLevelStatus(result.Status, types.LevelWarning)
        result.Issues = append(result.Issues,
            "release body ends with markdown link reference definitions (e.g. `[vX.Y.Z]: https://.../compare/...`) — "+
                "these belong at the bottom of CHANGELOG.md, not in the release notes",
        )
    }

    return result
}

func deriveProjectName(repository string) string {
    words := strings.Split(repository, "-")
    for index, word := range words {
        if len(word) > 0 {
            wordRunes := []rune(word)
            wordRunes[0] = unicode.ToUpper(wordRunes[0])
            words[index] = string(wordRunes)
        }
    }

    return strings.Join(words, " ")
}

func normalizeProjectName(name string) string {
    var result strings.Builder
    for _, runeValue := range strings.ToLower(name) {
        if unicode.IsLetter(runeValue) || unicode.IsDigit(runeValue) {
            result.WriteRune(runeValue)
        }
    }

    return result.String()
}

func suggestCorrectedTitle(projectName, tagName, currentTitle string) string {
    matches := titlePartsRegex.FindStringSubmatch(strings.TrimSpace(currentTitle))
    if nil == matches {
        return projectName + " " + tagName + " - <Summary>"
    }

    titleSummary := matches[3]
    summaryRunes := []rune(titleSummary)
    if len(summaryRunes) > 0 {
        summaryRunes[0] = unicode.ToUpper(summaryRunes[0])
        titleSummary = string(summaryRunes)
    }

    return projectName + " " + tagName + " - " + titleSummary
}

func nonStandardSections(body string) []string {
    if "" == strings.TrimSpace(body) {
        return nil
    }

    var found []string
    for _, line := range strings.Split(body, "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "## ") && false == standardSections[line] {
            found = append(found, line)
        }
    }

    return found
}

func computeReleaseStatus(releaseAudit types.ReleaseAudit) types.Status {
    levels := []types.LevelStatus{
        releaseAudit.Integrity.Status,
        releaseAudit.Distribution.Status,
        releaseAudit.Changelog.Status,
        releaseAudit.Diff.Status,
        releaseAudit.Presentation.Status,
    }

    for _, level := range levels {
        if types.LevelFailed == level {
            return types.StatusFailed
        }
    }

    for _, level := range levels {
        if types.LevelWarning == level {
            return types.StatusWarning
        }
    }

    return types.StatusOk
}

func buildProjectAudit(
    organization string,
    repository string,
    projectName string,
    packagistPackage string,
    packagistCount int,
    tagCount int,
    releaseCount int,
    submoduleTagCount int,
    releaseAudits []types.ReleaseAudit,
) *types.ProjectAudit {
    integrityStatus := types.LevelOk
    distributionStatus := types.LevelOk
    changelogStatus := types.LevelOk
    diffStatus := types.LevelOk
    presentationStatus := types.LevelOk

    warningCount := 0
    changelogWarningCount := 0
    diffWarningCount := 0

    if "" == packagistPackage {
        distributionStatus = types.LevelNotApplicable
    }

    for _, releaseAudit := range releaseAudits {
        if types.LevelFailed == releaseAudit.Integrity.Status {
            integrityStatus = types.LevelFailed
        }

        if types.LevelFailed == releaseAudit.Distribution.Status && types.LevelFailed != distributionStatus {
            distributionStatus = types.LevelFailed
        }

        if types.LevelWarning == releaseAudit.Distribution.Status && types.LevelOk == distributionStatus {
            distributionStatus = types.LevelWarning
        }

        if types.LevelFailed == releaseAudit.Changelog.Status && types.LevelFailed != changelogStatus {
            changelogStatus = types.LevelFailed
        }

        if types.LevelWarning == releaseAudit.Changelog.Status {
            changelogWarningCount++
            if types.LevelOk == changelogStatus {
                changelogStatus = types.LevelWarning
            }
        }

        if types.LevelFailed == releaseAudit.Diff.Status && types.LevelFailed != diffStatus {
            diffStatus = types.LevelFailed
        }

        if types.LevelWarning == releaseAudit.Diff.Status {
            diffWarningCount++
            if types.LevelOk == diffStatus {
                diffStatus = types.LevelWarning
            }
        }

        if types.LevelFailed == releaseAudit.Presentation.Status && types.LevelFailed != presentationStatus {
            presentationStatus = types.LevelFailed
        }

        if types.LevelWarning == releaseAudit.Presentation.Status {
            warningCount++
            if types.LevelOk == presentationStatus {
                presentationStatus = types.LevelWarning
            }
        }
    }

    projectStatus := types.StatusOk
    if types.LevelFailed == integrityStatus ||
        types.LevelFailed == distributionStatus ||
        types.LevelFailed == changelogStatus ||
        types.LevelFailed == diffStatus ||
        types.LevelFailed == presentationStatus {
        projectStatus = types.StatusFailed
    } else if types.LevelWarning == integrityStatus ||
        types.LevelWarning == distributionStatus ||
        types.LevelWarning == changelogStatus ||
        types.LevelWarning == diffStatus ||
        types.LevelWarning == presentationStatus {
        projectStatus = types.StatusWarning
    }

    presentationDisplay := string(presentationStatus)
    if types.LevelWarning == presentationStatus && warningCount > 0 {
        presentationDisplay = fmt.Sprintf("warning (%d)", warningCount)
    }

    changelogDisplay := string(changelogStatus)
    if types.LevelWarning == changelogStatus && changelogWarningCount > 0 {
        changelogDisplay = fmt.Sprintf("warning (%d)", changelogWarningCount)
    }

    diffDisplay := string(diffStatus)
    if types.LevelWarning == diffStatus && diffWarningCount > 0 {
        diffDisplay = fmt.Sprintf("warning (%d)", diffWarningCount)
    }

    return &types.ProjectAudit{
        OrganizationName:    organization,
        RepositoryName:      repository,
        ProjectName:         projectName,
        PackagistPackage:    packagistPackage,
        TagCount:            tagCount,
        ReleaseCount:        releaseCount,
        PackagistCount:      packagistCount,
        SubmoduleTagCount:   submoduleTagCount,
        Releases:            releaseAudits,
        IntegrityStatus:     integrityStatus,
        DistributionStatus:  distributionStatus,
        ChangelogStatus:     changelogStatus,
        ChangelogDisplay:    changelogDisplay,
        DiffStatus:          diffStatus,
        DiffDisplay:         diffDisplay,
        PresentationStatus:  presentationStatus,
        PresentationDisplay: presentationDisplay,
        Status:              projectStatus,
    }
}

func buildFetchErrorAudit(projectConfig project.ProjectConfig, fetchErr error) types.ProjectAudit {
    organization, repository := parseGithubUrl(projectConfig.GithubUrl)
    packagistPackage := parsePackagistUrl(projectConfig.PackagistUrl)

    projectName := projectConfig.Name
    if "" == projectName {
        projectName = deriveProjectName(repository)
    }

    return types.ProjectAudit{
        OrganizationName:    organization,
        RepositoryName:      repository,
        ProjectName:         projectName,
        PackagistPackage:    packagistPackage,
        TagCount:            0,
        ReleaseCount:        0,
        PackagistCount:      -1,
        SubmoduleTagCount:   0,
        Releases:            nil,
        IntegrityStatus:     types.LevelFailed,
        DistributionStatus:  types.LevelSkipped,
        ChangelogStatus:     types.LevelSkipped,
        ChangelogDisplay:    string(types.LevelSkipped),
        DiffStatus:          types.LevelSkipped,
        DiffDisplay:         string(types.LevelSkipped),
        PresentationStatus:  types.LevelSkipped,
        PresentationDisplay: string(types.LevelSkipped),
        Status:              types.StatusFailed,
        FetchError:          fetchErr.Error(),
    }
}

func fetchPackagistVersions(packagistPackage string) PackagistAuditInfo {
    if "" == packagistPackage {
        return PackagistAuditInfo{
            Versions: nil,
            Count:    -1,
            Err:      nil,
        }
    }

    versions, err := service.GetPackagistPackageVersions(packagistPackage)
    if nil != err {
        return PackagistAuditInfo{
            Versions: nil,
            Count:    -1,
            Err:      err,
        }
    }

    versionMap := make(map[string]service.PackagistVersion, len(versions))
    for _, version := range versions {
        versionMap[version.Version] = version
    }

    return PackagistAuditInfo{
        Versions: versionMap,
        Count:    len(versions),
        Err:      nil,
    }
}

func printTitleFixes(writer io.Writer, audits []types.ProjectAudit) {
    type fixEntry struct {
        version string
        url     string
        command string
    }

    type repoFixes struct {
        key   string
        fixes []fixEntry
    }

    var repos []repoFixes
    seen := make(map[string]bool)

    for _, audit := range audits {
        repositoryKey := audit.OrganizationName + "/" + audit.RepositoryName
        if true == seen[repositoryKey] {
            continue
        }

        var entries []fixEntry
        for _, release := range audit.Releases {
            hasTitleIssue := false
            for _, issue := range release.Presentation.Issues {
                if strings.Contains(issue, "title") {
                    hasTitleIssue = true
                    break
                }
            }

            if false == hasTitleIssue {
                continue
            }

            editUrl := fmt.Sprintf(
                "https://github.com/%s/%s/releases/edit/%s",
                audit.OrganizationName,
                audit.RepositoryName,
                release.TagName,
            )

            correctedTitle := suggestCorrectedTitle(audit.ProjectName, release.TagName, release.ReleaseTitle)
            ghCommand := fmt.Sprintf(
                "gh release edit %s --repo %s/%s --title %q",
                release.TagName,
                audit.OrganizationName,
                audit.RepositoryName,
                correctedTitle,
            )

            entries = append(entries, fixEntry{
                version: release.TagName,
                url:     editUrl,
                command: ghCommand,
            })
        }

        if len(entries) > 0 {
            seen[repositoryKey] = true
            repos = append(repos, repoFixes{
                key:   repositoryKey,
                fixes: entries,
            })
        }
    }

    if 0 == len(repos) {
        return
    }

    fmt.Fprintln(writer)
    fmt.Fprintln(writer, "TITLE FIXES:")
    for _, repo := range repos {
        fmt.Fprintf(writer, "  %s\n", repo.key)
        for _, fix := range repo.fixes {
            fmt.Fprintf(writer, "    %s  %s\n", fix.version, fix.url)
            fmt.Fprintf(writer, "    %s  %s\n", fix.version, fix.command)
        }
    }
}

func parsePackagistUrl(url string) string {
    if "" == url {
        return ""
    }

    const prefix = "/packages/"
    index := strings.Index(url, prefix)
    if -1 == index {
        return ""
    }

    return strings.TrimRight(url[index+len(prefix):], "/")
}

func applyChangelogDateAndLinkChecks(version, previousTagName, content, changelogPath string, result *ChangelogAuditResult) {
    titledMatches := changelogDatedTitledHeading.FindAllStringSubmatch(content, -1)
    titledIndex := -1
    for matchIndex, match := range titledMatches {
        if match[1] == version {
            titledIndex = matchIndex
            break
        }
    }

    if -1 != titledIndex {
        if _, parseErr := time.Parse(exceptionDateLayout, titledMatches[titledIndex][2]); nil != parseErr {
            result.Status = maxLevelStatus(result.Status, types.LevelWarning)
            result.Issues = append(result.Issues, fmt.Sprintf(
                "changelog entry for %s in %s has unparseable date %q",
                version, changelogPath, titledMatches[titledIndex][2],
            ))
        }
        result.HeadingTitle = strings.TrimSpace(titledMatches[titledIndex][3])
    } else {
        datedMatches := changelogDatedHeading.FindAllStringSubmatch(content, -1)
        datedIndex := -1
        for matchIndex, match := range datedMatches {
            if match[1] == version {
                datedIndex = matchIndex
                break
            }
        }
        if -1 == datedIndex {
            result.Status = maxLevelStatus(result.Status, types.LevelWarning)
            result.Issues = append(result.Issues, fmt.Sprintf(
                "changelog entry for %s in %s is missing the ' - YYYY-MM-DD - <Title>' heading suffix",
                version, changelogPath,
            ))
        } else {
            if _, parseErr := time.Parse(exceptionDateLayout, datedMatches[datedIndex][2]); nil != parseErr {
                result.Status = maxLevelStatus(result.Status, types.LevelWarning)
                result.Issues = append(result.Issues, fmt.Sprintf(
                    "changelog entry for %s in %s has unparseable date %q",
                    version, changelogPath, datedMatches[datedIndex][2],
                ))
            }
            result.Status = maxLevelStatus(result.Status, types.LevelWarning)
            result.Issues = append(result.Issues, fmt.Sprintf(
                "changelog entry for %s in %s is missing the ' - <Title>' suffix after the date",
                version, changelogPath,
            ))
        }
    }

    if "" == previousTagName {
        return
    }

    compareLinkMatches := changelogCompareLink.FindAllStringSubmatch(content, -1)
    hasCompareLink := false
    for _, match := range compareLinkMatches {
        if match[1] == version {
            hasCompareLink = true
            break
        }
    }
    if false == hasCompareLink {
        result.Status = maxLevelStatus(result.Status, types.LevelWarning)
        result.Issues = append(result.Issues, fmt.Sprintf(
            "missing compare link `[%s]: .../compare/...` in %s",
            version, changelogPath,
        ))
    }
}

func shortSha(sha string) string {
    if len(sha) >= 7 {
        return sha[:7]
    }
    return sha
}

func normalizeMarkdownBlock(input string) string {
    normalized := strings.ReplaceAll(input, "\r\n", "\n")
    normalized = strings.TrimSpace(normalized)

    var lines []string
    for _, line := range strings.Split(normalized, "\n") {
        line = strings.TrimSpace(line)
        if "" == line {
            continue
        }

        line = strings.TrimPrefix(line, "- ")
        line = strings.TrimPrefix(line, "* ")
        line = strings.Join(strings.Fields(line), " ")
        lines = append(lines, strings.ToLower(line))
    }

    return strings.Join(lines, "\n")
}

func overlapRatio(left string, right string) float64 {
    leftTokens := tokenizeForOverlap(left)
    rightTokens := tokenizeForOverlap(right)

    if 0 == len(leftTokens) || 0 == len(rightTokens) {
        return 0
    }

    intersection := 0
    for token := range leftTokens {
        if true == rightTokens[token] {
            intersection++
        }
    }

    maxCount := len(leftTokens)
    if len(rightTokens) > maxCount {
        maxCount = len(rightTokens)
    }

    if 0 == maxCount {
        return 0
    }

    return float64(intersection) / float64(maxCount)
}

func tokenizeForOverlap(input string) map[string]bool {
    result := make(map[string]bool)
    for _, token := range strings.Fields(strings.ToLower(input)) {
        token = strings.TrimSpace(token)
        if "" == token {
            continue
        }

        result[token] = true
    }

    return result
}

func maxLevelStatus(left types.LevelStatus, right types.LevelStatus) types.LevelStatus {
    if types.LevelFailed == left || types.LevelFailed == right {
        return types.LevelFailed
    }

    if types.LevelWarning == left || types.LevelWarning == right {
        return types.LevelWarning
    }

    if types.LevelNotApplicable == left {
        return right
    }

    if types.LevelNotApplicable == right {
        return left
    }

    return left
}

var _ clicontract.Command = (*AuditCommand)(nil)
