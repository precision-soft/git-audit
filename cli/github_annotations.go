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
            fmt.Fprintf(writer, "::error title=%s::%s\n", escapeWorkflowProperty(repository), escapeWorkflowData(audit.FetchError))
        }
        for releaseIndex := range audit.Releases {
            release := &audit.Releases[releaseIndex]
            for _, level := range levelNames {
                result := releaseLevels(release)[level]
                annotation := "warning"
                if types.LevelFailed == result.Status {
                    annotation = "error"
                }
                for _, issue := range result.Issues {
                    fmt.Fprintf(
                        writer,
                        "::%s title=%s %s %s::%s\n",
                        annotation,
                        escapeWorkflowProperty(repository),
                        escapeWorkflowProperty(release.TagName),
                        escapeWorkflowProperty(level),
                        escapeWorkflowData(issue),
                    )
                }
            }
        }
    }
}

/* the two escapers mirror actions/toolkit: only a property needs ":" and "," encoded, the runner decodes nothing else in the message */
func escapeWorkflowData(value string) string {
    return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(value)
}

func escapeWorkflowProperty(value string) string {
    return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(value)
}
