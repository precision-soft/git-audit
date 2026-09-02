package cli

import (
    "fmt"
    "strings"

    "github.com/precision-soft/git-audit/config/project"
)

const flagRepoUrl = "repo-url"

func resolveTargetProjects(repositoryFilter, repositoryUrl string) ([]project.ProjectConfig, error) {
    return resolveTargetProjectsFrom(project.Projects, repositoryFilter, repositoryUrl)
}

func resolveTargetProjectsFrom(available []project.ProjectConfig, repositoryFilter, repositoryUrl string) ([]project.ProjectConfig, error) {
    filters := splitRepoFilters(repositoryFilter)

    if 0 == len(filters) {
        if "" != repositoryUrl {
            return nil, fmt.Errorf("--repo-url requires --repo")
        }
        return available, nil
    }

    if 1 < len(filters) && "" != repositoryUrl {
        return nil, fmt.Errorf("--repo-url cannot be combined with multiple --repo values")
    }

    var resolved []project.ProjectConfig
    seen := make(map[string]bool)
    for _, singleFilter := range filters {
        projects, resolveErr := resolveSingleRepoFilterFrom(available, singleFilter, repositoryUrl)
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


func resolveSingleRepoFilterFrom(available []project.ProjectConfig, repositoryFilter, repositoryUrl string) ([]project.ProjectConfig, error) {
    filtered := filterProjects(available, repositoryFilter)
    if 1 < len(filtered) {
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
