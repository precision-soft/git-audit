package cli

import (
    "fmt"
    "io"
    "strings"

    "github.com/precision-soft/git-audit/types"
)

func renderGithubAnnotations(writer io.Writer, audits []types.ProjectAudit) {
    for _, audit := range audits {
        repository := audit.OrganizationName + "/" + audit.RepositoryName
        if "" != audit.FetchError {
            fmt.Fprintf(writer, "::error title=%s::%s\n", escapeWorkflowCommand(repository), escapeWorkflowCommand(audit.FetchError))
        }
        for releaseIndex := range audit.Releases {
            release := &audit.Releases[releaseIndex]
            for level, result := range releaseLevels(release) {
                annotation := "warning"
                if types.LevelFailed == result.Status {
                    annotation = "error"
                }
                for _, issue := range result.Issues {
                    fmt.Fprintf(
                        writer,
                        "::%s title=%s %s %s::%s\n",
                        annotation,
                        escapeWorkflowCommand(repository),
                        escapeWorkflowCommand(release.TagName),
                        escapeWorkflowCommand(level),
                        escapeWorkflowCommand(issue),
                    )
                }
            }
        }
    }
}

func escapeWorkflowCommand(value string) string {
    replacer := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
    return replacer.Replace(value)
}
