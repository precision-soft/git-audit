package cli

import (
    "fmt"
    "regexp"
    "strconv"
    "strings"
    "time"

    "github.com/precision-soft/git-audit/config/project"
    "github.com/precision-soft/git-audit/service"

    clicontract "github.com/precision-soft/melody/v3/cli/contract"
    runtimecontract "github.com/precision-soft/melody/v3/runtime/contract"
)

var (
    goSubmoduleTagRegex        = regexp.MustCompile(`^.+/v\d+\.\d+\.\d+$`)
    changelogHeadingV2         = regexp.MustCompile(`(?m)^##\s+\[(v\d+\.\d+\.\d+)\]`)
    changelogHeadingV1         = regexp.MustCompile(`(?m)^##\s+(v\d+\.\d+\.\d+)`)
    changelogLinkReferenceLine = regexp.MustCompile(`^\[[^\]]+\]:\s+\S+`)
)

func resolveGithubClient(
    runtimeInstance runtimecontract.Runtime,
    commandContext *clicontract.CommandContext,
) (*service.GithubClient, error) {
    cliToken := strings.TrimSpace(commandContext.String(flagToken))
    if "" != cliToken {
        return service.NewGithubClient(cliToken), nil
    }
    releaseService := service.GithubReleaseServiceMustFromRuntime(runtimeInstance)
    if "" == releaseService.Token() {
        return nil, fmt.Errorf("github token required (--token or GITHUB_TOKEN env)")
    }
    return releaseService.Client(), nil
}

func isGoSubmoduleTag(tagName string) bool {
    return goSubmoduleTagRegex.MatchString(tagName)
}

func formatRateLimitLine(info service.RateLimitInfo) string {
    if false == info.HasData {
        return ""
    }

    parts := []string{
        fmt.Sprintf("github rate limit: %d/%d remaining", info.Remaining, info.Limit),
    }

    if false == info.Reset.IsZero() {
        resetLocal := info.Reset.Local()
        remainingTime := time.Until(info.Reset).Round(time.Second)
        parts = append(parts, fmt.Sprintf("resets at %s (in %s)", resetLocal.Format("15:04:05"), remainingTime))
    }

    if "" != info.Resource {
        parts = append(parts, fmt.Sprintf("resource: %s", info.Resource))
    }

    return strings.Join(parts, " | ")
}

/*
 * parseGithubUrl extracts the organization and repository from any of the
 * common GitHub URL forms:
 *   - https://github.com/<org>/<repo>[.git][/]
 *   - git@github.com:<org>/<repo>[.git]
 *   - ssh://git@github.com/<org>/<repo>[.git]
 *   - github.com/<org>/<repo>[.git]
 *
 * The SSH forms are required so that forks can drop private-repo SSH URLs into
 * the project list (or pass them via --repo-url) without extra configuration.
 */
func parseGithubUrl(url string) (organization, repository string) {
    trimmed := strings.TrimRight(url, "/")
    trimmed = strings.TrimPrefix(trimmed, "https://")
    trimmed = strings.TrimPrefix(trimmed, "http://")
    trimmed = strings.TrimPrefix(trimmed, "ssh://")
    trimmed = strings.TrimPrefix(trimmed, "git@")
    trimmed = strings.TrimPrefix(trimmed, "github.com:")
    trimmed = strings.TrimPrefix(trimmed, "github.com/")

    trimmed = strings.TrimSuffix(trimmed, ".git")

    parts := strings.Split(trimmed, "/")
    if len(parts) < 2 {
        return "", trimmed
    }

    return parts[len(parts)-2], parts[len(parts)-1]
}

func resolveChangelogPaths(projectConfig project.ProjectConfig) []string {
    if len(projectConfig.ChangelogPaths) > 0 {
        return projectConfig.ChangelogPaths
    }

    return []string{"CHANGELOG.md"}
}

func extractChangelogEntry(content string, version string) (string, bool) {
    matchIndexes := changelogHeadingV2.FindAllStringSubmatchIndex(content, -1)
    if len(matchIndexes) > 0 {
        for matchIndex, match := range matchIndexes {
            currentVersion := content[match[2]:match[3]]
            if currentVersion != version {
                continue
            }

            start := skipRestOfHeadingLine(content, match[1])
            end := len(content)
            if matchIndex+1 < len(matchIndexes) {
                end = matchIndexes[matchIndex+1][0]
            }

            return stripTrailingLinkReferences(strings.TrimSpace(content[start:end])), true
        }
    }

    matchIndexes = changelogHeadingV1.FindAllStringSubmatchIndex(content, -1)
    if len(matchIndexes) > 0 {
        for matchIndex, match := range matchIndexes {
            currentVersion := content[match[2]:match[3]]
            if currentVersion != version {
                continue
            }

            start := skipRestOfHeadingLine(content, match[1])
            end := len(content)
            if matchIndex+1 < len(matchIndexes) {
                end = matchIndexes[matchIndex+1][0]
            }

            return stripTrailingLinkReferences(strings.TrimSpace(content[start:end])), true
        }
    }

    return "", false
}

func skipRestOfHeadingLine(content string, start int) int {
    newlineIndex := strings.IndexByte(content[start:], '\n')
    if -1 == newlineIndex {
        return len(content)
    }
    return start + newlineIndex + 1
}

/**
 * stripTrailingLinkReferences drops trailing markdown link reference
 * definitions (e.g. `[v1.0.0]: https://.../compare/...`) from an extracted
 * changelog body. The last version's section otherwise absorbs the global
 * reference block that typically sits at the bottom of a CHANGELOG.md, since
 * extractChangelogEntry has no next heading to use as a stop boundary.
 */
func stripTrailingLinkReferences(body string) string {
    if "" == body {
        return body
    }

    lines := strings.Split(body, "\n")
    cutoff := len(lines)
    for index := len(lines) - 1; index >= 0; index-- {
        trimmed := strings.TrimSpace(lines[index])
        if "" == trimmed {
            cutoff = index
            continue
        }
        if true == changelogLinkReferenceLine.MatchString(trimmed) {
            cutoff = index
            continue
        }
        break
    }

    return strings.TrimSpace(strings.Join(lines[:cutoff], "\n"))
}

func hasTrailingChangelogLinkReferences(body string) bool {
    trimmed := strings.TrimSpace(body)
    if "" == trimmed {
        return false
    }
    lines := strings.Split(trimmed, "\n")
    for index := len(lines) - 1; index >= 0; index-- {
        line := strings.TrimSpace(lines[index])
        if "" == line {
            continue
        }
        return changelogLinkReferenceLine.MatchString(line)
    }
    return false
}

func compareSemver(left, right string) int {
    leftParts := semverParts(left)
    rightParts := semverParts(right)
    for index := 0; index < 3; index++ {
        if leftParts[index] != rightParts[index] {
            if leftParts[index] < rightParts[index] {
                return -1
            }
            return 1
        }
    }
    if left < right {
        return -1
    }
    if left > right {
        return 1
    }
    return 0
}

func semverParts(tag string) [3]int {
    trimmed := strings.TrimPrefix(tag, "v")
    if slashIndex := strings.LastIndex(trimmed, "/v"); slashIndex >= 0 {
        trimmed = trimmed[slashIndex+2:]
    }
    var parts [3]int
    for index, segment := range strings.SplitN(trimmed, ".", 3) {
        if index >= 3 {
            break
        }
        if value, atoiErr := strconv.Atoi(segment); nil == atoiErr {
            parts[index] = value
        }
    }
    return parts
}
