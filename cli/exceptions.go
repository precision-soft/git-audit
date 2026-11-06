package cli

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "time"

    "github.com/precision-soft/git-audit/types"

    "github.com/precision-soft/melody/v3/cli/output"
    "golang.org/x/term"
)

const exceptionDateLayout = "2006-01-02"

type ExceptionEntry struct {
    Issue         string
    ReviewedUntil time.Time
}

type Exceptions map[string]map[string]map[string][]ExceptionEntry

type pendingWarning struct {
    repository string
    version    string
    level      string
    issue      string
    url        string
}

func (instance *ExceptionEntry) UnmarshalJSON(data []byte) error {
    var asString string
    if stringErr := json.Unmarshal(data, &asString); nil == stringErr {
        instance.Issue = asString
        return nil
    }
    var rawObject struct {
        Issue         string `json:"issue"`
        ReviewedUntil string `json:"reviewed_until,omitempty"`
    }
    if objectErr := json.Unmarshal(data, &rawObject); nil != objectErr {
        return objectErr
    }
    instance.Issue = rawObject.Issue
    if "" != strings.TrimSpace(rawObject.ReviewedUntil) {
        parsed, parseErr := time.Parse(exceptionDateLayout, rawObject.ReviewedUntil)
        if nil != parseErr {
            return fmt.Errorf("invalid reviewed_until %q: %w", rawObject.ReviewedUntil, parseErr)
        }
        instance.ReviewedUntil = parsed
    }
    return nil
}

func (instance ExceptionEntry) MarshalJSON() ([]byte, error) {
    if true == instance.ReviewedUntil.IsZero() {
        return json.Marshal(instance.Issue)
    }
    return json.Marshal(struct {
        Issue         string `json:"issue"`
        ReviewedUntil string `json:"reviewed_until"`
    }{
        Issue:         instance.Issue,
        ReviewedUntil: instance.ReviewedUntil.Format(exceptionDateLayout),
    })
}

func (instance ExceptionEntry) Active(now time.Time) bool {
    if true == instance.ReviewedUntil.IsZero() {
        return true
    }
    return now.Before(instance.ReviewedUntil.AddDate(0, 0, 1))
}

func loadExceptions(filePath string) (Exceptions, error) {
    data, readErr := os.ReadFile(filePath)
    if true == os.IsNotExist(readErr) {
        return make(Exceptions), nil
    }
    if nil != readErr {
        return nil, fmt.Errorf("read exceptions: %w", readErr)
    }

    var exceptions Exceptions
    if unmarshalErr := json.Unmarshal(data, &exceptions); nil != unmarshalErr {
        return nil, fmt.Errorf("parse exceptions: %w", unmarshalErr)
    }
    if nil == exceptions {
        return make(Exceptions), nil
    }

    return exceptions, nil
}

func saveExceptions(filePath string, exceptions Exceptions) error {
    data, marshalErr := json.MarshalIndent(exceptions, "", "  ")
    if nil != marshalErr {
        return marshalErr
    }

    temporary, createErr := os.CreateTemp(filepath.Dir(filePath), ".exceptions-*")
    if nil != createErr {
        return createErr
    }
    temporaryPath := temporary.Name()
    defer os.Remove(temporaryPath)

    if chmodErr := temporary.Chmod(0o644); nil != chmodErr {
        temporary.Close()
        return chmodErr
    }
    if _, writeErr := temporary.Write(data); nil != writeErr {
        temporary.Close()
        return writeErr
    }
    if syncErr := temporary.Sync(); nil != syncErr {
        temporary.Close()
        return syncErr
    }
    if closeErr := temporary.Close(); nil != closeErr {
        return closeErr
    }

    return os.Rename(temporaryPath, filePath)
}

func (instance Exceptions) has(repository, version, level, issue string, now time.Time) bool {
    versionMap, hasRepository := instance[repository]
    if false == hasRepository {
        return false
    }
    levelMap, hasVersion := versionMap[version]
    if false == hasVersion {
        return false
    }
    for _, entry := range levelMap[level] {
        if entry.Issue == issue && true == entry.Active(now) {
            return true
        }
    }
    return false
}

func (instance Exceptions) add(repository, version, level, issue string) {
    if nil == instance[repository] {
        instance[repository] = make(map[string]map[string][]ExceptionEntry)
    }
    if nil == instance[repository][version] {
        instance[repository][version] = make(map[string][]ExceptionEntry)
    }
    entries := instance[repository][version][level]
    for entryIndex := range entries {
        if entries[entryIndex].Issue == issue {
            entries[entryIndex].ReviewedUntil = time.Time{}
            return
        }
    }
    instance[repository][version][level] = append(
        instance[repository][version][level],
        ExceptionEntry{Issue: issue},
    )
}

var levelNames = []string{"integrity", "distribution", "changelog", "diff", "presentation", "supply-chain"}

func releaseLevels(release *types.ReleaseAudit) map[string]*types.LevelResult {
    return map[string]*types.LevelResult{
        "integrity":    &release.Integrity,
        "distribution": &release.Distribution,
        "changelog":    &release.Changelog,
        "diff":         &release.Diff,
        "presentation": &release.Presentation,
        "supply-chain": &release.SupplyChain,
    }
}

func applyExceptions(audits []types.ProjectAudit, exceptions Exceptions) {
    now := time.Now()
    for auditIndex := range audits {
        audit := &audits[auditIndex]
        repositoryKey := audit.OrganizationName + "/" + audit.RepositoryName
        projectChanged := false
        for releaseIndex := range audit.Releases {
            release := &audit.Releases[releaseIndex]
            releaseChanged := false
            for _, level := range levelNames {
                result := releaseLevels(release)[level]

                /* filtering a failed level would leave it red with no line explaining why */
                if types.LevelWarning != result.Status {
                    continue
                }

                var remaining []string
                filtered := false
                for _, issue := range result.Issues {
                    if true == exceptions.has(repositoryKey, release.TagName, level, issue, now) {
                        filtered = true
                    } else {
                        remaining = append(remaining, issue)
                    }
                }
                if false == filtered {
                    continue
                }
                result.Issues = remaining
                if 0 == len(remaining) {
                    result.Status = types.LevelOk
                }
                releaseChanged = true
            }
            if true == releaseChanged {
                release.Status = computeReleaseStatus(*release)
                projectChanged = true
            }
        }
        if true == projectChanged {
            recomputeProjectAggregates(audit)
        }
    }
}

func recomputeProjectAggregates(audit *types.ProjectAudit) {
    if "" != audit.FetchError {
        return
    }

    statuses := make(map[string]types.LevelStatus, len(levelNames))
    warningCounts := make(map[string]int, len(levelNames))

    for _, level := range levelNames {
        statuses[level] = types.LevelOk
    }
    if types.LevelNotApplicable == audit.DistributionStatus {
        statuses["distribution"] = types.LevelNotApplicable
    }
    if false == hasLevelResults(audit, "supply-chain") {
        statuses["supply-chain"] = types.LevelNotApplicable
    }
    if false == hasLevelResults(audit, "changelog") {
        statuses["changelog"] = types.LevelNotApplicable
    }

    for releaseIndex := range audit.Releases {
        for _, level := range levelNames {
            result := releaseLevels(&audit.Releases[releaseIndex])[level]
            if types.LevelFailed == result.Status && types.LevelFailed != statuses[level] {
                statuses[level] = types.LevelFailed
            }
            if types.LevelWarning == result.Status {
                warningCounts[level]++
                if types.LevelOk == statuses[level] {
                    statuses[level] = types.LevelWarning
                }
            }
        }
    }

    audit.IntegrityStatus = statuses["integrity"]
    audit.DistributionStatus = statuses["distribution"]
    audit.ChangelogStatus = statuses["changelog"]
    audit.DiffStatus = statuses["diff"]
    audit.PresentationStatus = statuses["presentation"]
    audit.SupplyChainStatus = statuses["supply-chain"]

    audit.ChangelogDisplay = formatLevelDisplay(statuses["changelog"], warningCounts["changelog"])
    audit.DiffDisplay = formatLevelDisplay(statuses["diff"], warningCounts["diff"])
    audit.PresentationDisplay = formatLevelDisplay(statuses["presentation"], warningCounts["presentation"])

    audit.Status = types.StatusOk
    for _, level := range levelNames {
        if types.LevelFailed == statuses[level] {
            audit.Status = types.StatusFailed
            return
        }
    }
    for _, level := range levelNames {
        if types.LevelWarning == statuses[level] {
            audit.Status = types.StatusWarning
            return
        }
    }
}

/* a level nobody ran leaves every release at n/a (or unset, for audits built before the level existed) */
func hasLevelResults(audit *types.ProjectAudit, level string) bool {
    for releaseIndex := range audit.Releases {
        status := releaseLevels(&audit.Releases[releaseIndex])[level].Status
        if "" != status && types.LevelNotApplicable != status {
            return true
        }
    }
    return false
}

func formatLevelDisplay(status types.LevelStatus, warningCount int) string {
    if types.LevelWarning == status && 0 < warningCount {
        return fmt.Sprintf("warning (%d)", warningCount)
    }
    return string(status)
}

func collectPendingWarnings(audits []types.ProjectAudit) []pendingWarning {
    var pending []pendingWarning
    for auditIndex := range audits {
        audit := &audits[auditIndex]
        repositoryKey := audit.OrganizationName + "/" + audit.RepositoryName
        for releaseIndex := range audit.Releases {
            release := &audit.Releases[releaseIndex]
            editUrl := fmt.Sprintf(
                "https://github.com/%s/%s/releases/edit/%s",
                audit.OrganizationName,
                audit.RepositoryName,
                release.TagName,
            )
            for _, level := range levelNames {
                result := releaseLevels(release)[level]
                if types.LevelWarning != result.Status {
                    continue
                }
                for _, issue := range result.Issues {
                    pending = append(pending, pendingWarning{repositoryKey, release.TagName, level, issue, editUrl})
                }
            }
        }
    }
    return pending
}

func reviewWarningsInteractively(
    writer io.Writer,
    option output.Option,
    pending []pendingWarning,
    exceptions Exceptions,
) (bool, error) {
    if 0 == len(pending) || output.FormatTable != option.Format || false == isInteractiveTTY() {
        return false, nil
    }

    fmt.Fprintf(writer, "\nUnacknowledged warnings (%d):\n", len(pending))
    for warningIndex, warning := range pending {
        fmt.Fprintf(writer, "  [%3d] %-42s %-14s %s\n", warningIndex+1, warning.repository, warning.version, warning.issue)
        fmt.Fprintf(writer, "       %s\n", warning.url)
    }
    fmt.Fprintf(writer, "\nAccept as OK (e.g. \"1 2 5\", \"1-10\", \"all\", or Enter to skip): ")

    scanner := bufio.NewScanner(os.Stdin)
    if false == scanner.Scan() {
        return false, nil
    }

    input := strings.TrimSpace(scanner.Text())
    if "" == input {
        return false, nil
    }

    indices := parseSelection(input, len(pending))
    for _, index := range indices {
        warning := pending[index]
        exceptions.add(warning.repository, warning.version, warning.level, warning.issue)
    }

    return 0 < len(indices), nil
}

func parseSelection(input string, limit int) []int {
    if true == strings.EqualFold(input, "all") {
        result := make([]int, limit)
        for index := range result {
            result[index] = index
        }
        return result
    }

    seen := make(map[int]bool)
    var result []int

    for _, part := range strings.Fields(input) {
        if separatorIndex := strings.Index(part, "-"); 0 < separatorIndex {
            from, fromErr := strconv.Atoi(part[:separatorIndex])
            to, toErr := strconv.Atoi(part[separatorIndex+1:])
            if nil == fromErr && nil == toErr {
                for value := from; value <= to; value++ {
                    if 1 <= value && limit >= value && false == seen[value-1] {
                        seen[value-1] = true
                        result = append(result, value-1)
                    }
                }
            }
        } else {
            number, parseErr := strconv.Atoi(part)
            if nil == parseErr && 1 <= number && limit >= number && false == seen[number-1] {
                seen[number-1] = true
                result = append(result, number-1)
            }
        }
    }

    sort.Ints(result)
    return result
}

func isInteractiveTTY() bool {
    return true == term.IsTerminal(int(os.Stdin.Fd()))
}
