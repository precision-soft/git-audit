package cli

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
    "time"

    "github.com/precision-soft/git-audit/service"

    clicontract "github.com/precision-soft/melody/v3/cli/contract"
    "github.com/precision-soft/melody/v3/cli/output"
    melodyconfig "github.com/precision-soft/melody/v3/config"
    runtimecontract "github.com/precision-soft/melody/v3/runtime/contract"
)

const (
    flagApply = "apply"
    flagTag   = "tag"
)

var changelogVersionListRegex = regexp.MustCompile(`(?m)^##\s+\[?(v\d+\.\d+\.\d+)\]?`)

type SyncCommand struct{}

type syncResult struct {
    Repository  string `json:"repository"`
    Tag         string `json:"tag"`
    Status      string `json:"status"`
    Message     string `json:"message,omitempty"`
    Diff        string `json:"diff,omitempty"`
    TitleChange string `json:"titleChange,omitempty"`
}

type syncDiff struct {
    repository  string
    tag         string
    diff        string
    titleChange string
}

func (command *SyncCommand) Name() string {
    return "sync"
}

func (command *SyncCommand) Description() string {
    return "sync github release bodies from local changelogs (changelog is the source of truth)"
}

func (command *SyncCommand) Flags() []clicontract.Flag {
    return output.MergeFlags(
        output.DebugFlags(),
        []clicontract.Flag{
            &clicontract.StringFlag{
                Name:  flagToken,
                Usage: "github token",
            },
            &clicontract.StringFlag{
                Name:  flagRepo,
                Usage: "restrict to a single repository (by repo name, e.g. doctrine-type)",
            },
            &clicontract.StringFlag{
                Name:  flagRepoUrl,
                Usage: "github URL for a repo not in the built-in project list (required when --repo is unknown)",
            },
            &clicontract.StringFlag{
                Name:  flagTag,
                Usage: "restrict to a single tag (e.g. v3.4.1) — useful for testing sync on one release before rolling across all tags",
            },
            &clicontract.BoolFlag{
                Name:  flagApply,
                Usage: "actually patch release bodies (default is dry-run: print per-tag unified diff, no API writes)",
                Value: false,
            },
        },
    )
}

func (command *SyncCommand) Run(
    runtimeInstance runtimecontract.Runtime,
    commandContext *clicontract.CommandContext,
) error {
    startedAt := time.Now()

    token := strings.TrimSpace(commandContext.String(flagToken))
    if "" == token {
        configuration := melodyconfig.ConfigMustFromContainer(runtimeInstance.Container())
        token = configuration.Get("github.token").String()
    }
    if "" == token {
        return fmt.Errorf("github token required (--token or github_token env)")
    }

    repositoryFilter := strings.TrimSpace(commandContext.String(flagRepo))
    repositoryUrl := strings.TrimSpace(commandContext.String(flagRepoUrl))
    tagFilter := strings.TrimSpace(commandContext.String(flagTag))
    dryRun := false == commandContext.Bool(flagApply)

    filteredProjects, resolveErr := resolveTargetProjects(repositoryFilter, repositoryUrl)
    if nil != resolveErr {
        return resolveErr
    }

    githubClient := service.NewGithubClient(token)
    option := output.NormalizeOption(output.ParseOptionFromCommand(commandContext))
    meta := output.NewMeta(
        command.Name(),
        commandContext.Args().Slice(),
        option,
        startedAt,
        time.Since(startedAt),
        output.Version{},
    )
    envelope := output.NewEnvelope(meta)

    var results []syncResult
    var diffs []syncDiff
    counts := map[string]int{"updated": 0, "would-update": 0, "up-to-date": 0, "missing": 0, "skipped": 0, "error": 0}

    for _, projectConfig := range filteredProjects {
        organization, repository := parseGithubUrl(projectConfig.GithubUrl)
        if "" == organization || "" == repository {
            continue
        }

        cloneDirectory, cloneErr := service.EnsureCloneReset(repository, projectConfig.GithubUrl)
        if nil != cloneErr {
            results = append(results, syncResult{
                Repository: repository,
                Status:     "error",
                Message:    fmt.Sprintf("clone: %s", cloneErr),
            })
            counts["error"]++
            continue
        }

        releases, listErr := githubClient.GetReleases(organization, repository)
        if nil != listErr {
            results = append(results, syncResult{
                Repository: repository,
                Status:     "error",
                Message:    fmt.Sprintf("list releases: %s", listErr),
            })
            counts["error"]++
            continue
        }

        releaseByTag := make(map[string]*service.GithubRelease, len(releases))
        for index := range releases {
            releaseByTag[releases[index].TagName] = &releases[index]
        }

        for _, relativePath := range resolveChangelogPaths(projectConfig) {
            absolutePath := filepath.Join(cloneDirectory, relativePath)
            contentBytes, readErr := os.ReadFile(absolutePath)
            if nil != readErr {
                results = append(results, syncResult{
                    Repository: repository,
                    Tag:        relativePath,
                    Status:     "error",
                    Message:    fmt.Sprintf("read changelog: %s", readErr),
                })
                counts["error"]++
                continue
            }
            content := string(contentBytes)

            versions := listChangelogVersions(content)
            sort.SliceStable(versions, func(leftIndex, rightIndex int) bool {
                return compareSemver(versions[leftIndex], versions[rightIndex]) < 0
            })

            for _, version := range versions {
                if "" != tagFilter && version != tagFilter {
                    continue
                }

                entryBody, found := extractChangelogEntry(content, version)
                if false == found {
                    continue
                }

                foldedBody := foldChangelogBody(entryBody)
                if "" == foldedBody {
                    results = append(results, syncResult{
                        Repository: repository,
                        Tag:        version,
                        Status:     "skipped",
                        Message:    "empty body after normalization",
                    })
                    counts["skipped"]++
                    continue
                }

                release, releaseFound := releaseByTag[version]
                if false == releaseFound {
                    results = append(results, syncResult{
                        Repository: repository,
                        Tag:        version,
                        Status:     "missing",
                        Message:    "no github release for tag",
                    })
                    counts["missing"]++
                    continue
                }

                desiredName := buildReleaseName(projectConfig.Name, version, content)
                bodyMatches := compareReleaseBody(release.Body, foldedBody)
                nameMatches := "" == desiredName || release.Name == desiredName

                if true == bodyMatches && true == nameMatches {
                    results = append(results, syncResult{
                        Repository: repository,
                        Tag:        version,
                        Status:     "up-to-date",
                    })
                    counts["up-to-date"]++
                    continue
                }

                titleChange := ""
                if false == nameMatches {
                    titleChange = fmt.Sprintf("%q → %q", release.Name, desiredName)
                }

                if true == dryRun {
                    bodyDiff := ""
                    if false == bodyMatches {
                        bodyDiff = unifiedDiff(canonicalReleaseBody(release.Body), canonicalReleaseBody(foldedBody))
                    }
                    results = append(results, syncResult{
                        Repository:  repository,
                        Tag:         version,
                        Status:      "would-update",
                        Diff:        bodyDiff,
                        TitleChange: titleChange,
                    })
                    diffs = append(diffs, syncDiff{
                        repository:  repository,
                        tag:         version,
                        diff:        bodyDiff,
                        titleChange: titleChange,
                    })
                    counts["would-update"]++
                    continue
                }

                updateErr := githubClient.UpdateRelease(organization, repository, release.ID, foldedBody, desiredName)
                if nil != updateErr {
                    results = append(results, syncResult{
                        Repository: repository,
                        Tag:        version,
                        Status:     "error",
                        Message:    updateErr.Error(),
                    })
                    counts["error"]++
                    continue
                }
                results = append(results, syncResult{
                    Repository: repository,
                    Tag:        version,
                    Status:     "updated",
                })
                counts["updated"]++
            }
        }
    }

    if option.Format == output.FormatTable {
        builder := output.NewTableBuilder()
        mode := "apply"
        if true == dryRun {
            mode = "dry-run"
        }
        builder.AddSummaryLine(fmt.Sprintf(
            "mode: %s | updated: %d | would-update: %d | up-to-date: %d | missing: %d | skipped: %d | errors: %d",
            mode, counts["updated"], counts["would-update"], counts["up-to-date"], counts["missing"], counts["skipped"], counts["error"],
        ))
        if rateLimitLine := formatRateLimitLine(githubClient.RateLimit()); "" != rateLimitLine {
            builder.AddSummaryLine(rateLimitLine)
        }

        byRepository := make(map[string][]syncResult)
        for _, row := range results {
            byRepository[row.Repository] = append(byRepository[row.Repository], row)
        }
        repositories := make([]string, 0, len(byRepository))
        for repositoryKey := range byRepository {
            repositories = append(repositories, repositoryKey)
        }
        sort.Strings(repositories)
        for _, repositoryKey := range repositories {
            block := builder.AddBlock(repositoryKey, []string{"tag", "status", "message"})
            for _, row := range byRepository[repositoryKey] {
                block.AddRow(row.Tag, row.Status, row.Message)
            }
        }
        envelope.Table = builder.Build()
    } else {
        envelope.Data = results
    }

    envelope.Meta.DurationMilliseconds = time.Since(startedAt).Milliseconds()

    if renderErr := output.Render(commandContext.Writer, envelope, option); nil != renderErr {
        return renderErr
    }

    if option.Format == output.FormatTable && true == dryRun {
        printSyncDiffs(commandContext.Writer, diffs)
    }

    if counts["error"] > 0 {
        return fmt.Errorf("sync completed with %d errors", counts["error"])
    }
    return nil
}

func printSyncDiffs(writer io.Writer, diffs []syncDiff) {
    if 0 == len(diffs) {
        return
    }

    byRepository := make(map[string][]syncDiff)
    var repositories []string
    for _, entry := range diffs {
        if _, exists := byRepository[entry.repository]; false == exists {
            repositories = append(repositories, entry.repository)
        }
        byRepository[entry.repository] = append(byRepository[entry.repository], entry)
    }
    sort.Strings(repositories)

    fmt.Fprintln(writer)
    fmt.Fprintln(writer, "DIFFS:")
    for _, repositoryKey := range repositories {
        fmt.Fprintf(writer, "  %s\n", repositoryKey)
        for _, entry := range byRepository[repositoryKey] {
            fmt.Fprintf(writer, "    %s\n", entry.tag)
            if "" != entry.titleChange {
                fmt.Fprintf(writer, "      title: %s\n", entry.titleChange)
            }
            if "" == entry.diff {
                if "" == entry.titleChange {
                    fmt.Fprintln(writer, "      (no textual changes after normalization)")
                }
                continue
            }
            for _, line := range strings.Split(entry.diff, "\n") {
                fmt.Fprintf(writer, "      %s\n", line)
            }
        }
    }
}

/**
 * unifiedDiff returns a line-oriented diff between current and desired,
 * prefixed with "- " (removed), "+ " (added), "  " (context). LCS-based,
 * sufficient for small release bodies.
 */
func unifiedDiff(current, desired string) string {
    currentLines := splitLinesForDiff(current)
    desiredLines := splitLinesForDiff(desired)

    rows, columns := len(currentLines), len(desiredLines)
    lcs := make([][]int, rows+1)
    for index := range lcs {
        lcs[index] = make([]int, columns+1)
    }
    for row := 1; row <= rows; row++ {
        for column := 1; column <= columns; column++ {
            if currentLines[row-1] == desiredLines[column-1] {
                lcs[row][column] = lcs[row-1][column-1] + 1
            } else if lcs[row-1][column] >= lcs[row][column-1] {
                lcs[row][column] = lcs[row-1][column]
            } else {
                lcs[row][column] = lcs[row][column-1]
            }
        }
    }

    var lines []string
    row, column := rows, columns
    for row > 0 || column > 0 {
        switch {
        case row > 0 && column > 0 && currentLines[row-1] == desiredLines[column-1]:
            lines = append(lines, "  "+currentLines[row-1])
            row--
            column--
        case column > 0 && (0 == row || lcs[row][column-1] >= lcs[row-1][column]):
            lines = append(lines, "+ "+desiredLines[column-1])
            column--
        case row > 0:
            lines = append(lines, "- "+currentLines[row-1])
            row--
        }
    }

    for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
        lines[left], lines[right] = lines[right], lines[left]
    }

    return strings.Join(lines, "\n")
}

func splitLinesForDiff(text string) []string {
    if "" == text {
        return nil
    }
    return strings.Split(text, "\n")
}

func extractChangelogTitle(content, version string) (string, bool) {
    matches := changelogDatedTitledHeading.FindAllStringSubmatch(content, -1)
    for _, match := range matches {
        if match[1] == version {
            return strings.TrimSpace(match[3]), true
        }
    }
    return "", false
}

func buildReleaseName(projectName, version, content string) string {
    if "" == projectName {
        return ""
    }
    title, found := extractChangelogTitle(content, version)
    if false == found {
        return ""
    }
    return fmt.Sprintf("%s %s - %s", projectName, version, title)
}

func listChangelogVersions(content string) []string {
    matches := changelogVersionListRegex.FindAllStringSubmatch(content, -1)
    seen := make(map[string]bool, len(matches))
    versions := make([]string, 0, len(matches))
    for _, match := range matches {
        if len(match) < 2 {
            continue
        }
        if true == seen[match[1]] {
            continue
        }
        seen[match[1]] = true
        versions = append(versions, match[1])
    }
    return versions
}

func foldChangelogBody(body string) string {
    normalized := strings.ReplaceAll(body, "\r\n", "\n")

    var folded []string
    for _, line := range strings.Split(normalized, "\n") {
        trimmed := strings.TrimRight(line, " \t")
        switch {
        case "### Security" == trimmed:
            folded = append(folded, "## Fixed")
        case "### Removed" == trimmed:
            folded = append(folded, "## Changed")
        case strings.HasPrefix(trimmed, "### "):
            folded = append(folded, "## "+strings.TrimPrefix(trimmed, "### "))
        default:
            folded = append(folded, trimmed)
        }
    }

    return strings.TrimSpace(strings.Join(folded, "\n"))
}

func compareReleaseBody(current, desired string) bool {
    return canonicalReleaseBody(current) == canonicalReleaseBody(desired)
}

func canonicalReleaseBody(body string) string {
    normalized := strings.ReplaceAll(body, "\r\n", "\n")
    lines := strings.Split(normalized, "\n")
    var cleaned []string
    for _, line := range lines {
        trimmed := strings.TrimRight(line, " \t")
        if "" == trimmed && len(cleaned) > 0 && "" == cleaned[len(cleaned)-1] {
            continue
        }
        cleaned = append(cleaned, trimmed)
    }
    return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

var _ clicontract.Command = (*SyncCommand)(nil)
