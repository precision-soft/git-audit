package cli

import (
    "fmt"
    "strings"

    "github.com/precision-soft/git-audit/config/project"
)

const flagRepoUrl = "repo-url"

/*
resolveTargetProjects applies the --repo / --repo-url flag pair against the
built-in project list. --repo accepts a comma-separated list of repo names
(e.g. `doctrine-type,doctrine-utility`); --repo-url applies only when exactly
one --repo value is provided.

  - empty filter, empty url         → return all known projects
  - empty filter, url set           → error (--repo-url requires --repo)
  - single filter, empty url        → resolve that repo against the project list
  - single filter, matching url     → return that project
  - single filter, mismatching url  → error (cannot override known URL)
  - single unknown filter, url set  → return ad-hoc ProjectConfig with that URL
  - multiple filters, empty url     → resolve each (all must be known); dedupe
  - multiple filters, url set       → error (--repo-url is single-valued)
*/
func resolveTargetProjects(repositoryFilter, repositoryUrl string) ([]project.ProjectConfig, error) {
    filters := splitRepoFilters(repositoryFilter)

    if 0 == len(filters) {
        if "" != repositoryUrl {
            return nil, fmt.Errorf("--repo-url requires --repo")
        }
        return project.Projects, nil
    }

    if len(filters) > 1 && "" != repositoryUrl {
        return nil, fmt.Errorf("--repo-url cannot be combined with multiple --repo values")
    }

    var resolved []project.ProjectConfig
    seen := make(map[string]bool)
    for _, singleFilter := range filters {
        projects, resolveErr := resolveSingleRepoFilter(singleFilter, repositoryUrl)
        if nil != resolveErr {
            return nil, resolveErr
        }
        for _, projectConfig := range projects {
            if true == seen[projectConfig.GithubUrl] {
                continue
            }
            seen[projectConfig.GithubUrl] = true
            resolved = append(resolved, projectConfig)
        }
    }
    return resolved, nil
}

func resolveSingleRepoFilter(repositoryFilter, repositoryUrl string) ([]project.ProjectConfig, error) {
    filtered := filterProjects(project.Projects, repositoryFilter)
    if len(filtered) > 1 {
        return nil, fmt.Errorf("ambiguous --repo %q matched %d projects", repositoryFilter, len(filtered))
    }
    if 1 == len(filtered) {
        if "" != repositoryUrl && repositoryUrl != filtered[0].GithubUrl {
            return nil, fmt.Errorf(
                "--repo %q is a known project (%s); remove --repo-url or pick a different --repo name",
                repositoryFilter, filtered[0].GithubUrl,
            )
        }
        return filtered, nil
    }

    if "" == repositoryUrl {
        return nil, fmt.Errorf(
            "unknown repo %q; pass --repo-url https://github.com/<org>/%s to clone it on the fly",
            repositoryFilter, repositoryFilter,
        )
    }
    return []project.ProjectConfig{{GithubUrl: repositoryUrl}}, nil
}

func splitRepoFilters(repositoryFilter string) []string {
    if "" == repositoryFilter {
        return nil
    }
    var filters []string
    for _, part := range strings.Split(repositoryFilter, ",") {
        trimmed := strings.TrimSpace(part)
        if "" == trimmed {
            continue
        }
        filters = append(filters, trimmed)
    }
    return filters
}
