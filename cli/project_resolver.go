package cli

import (
    "fmt"

    "github.com/precision-soft/git-audit/config/project"
)

const flagRepoUrl = "repo-url"

/**
 * resolveTargetProjects applies the --repo / --repo-url flag pair against the
 * built-in project list:
 *
 *   - empty filter, empty url        → return all known projects
 *   - empty filter, url set          → error (--repo-url requires --repo)
 *   - known filter, empty url        → return that project
 *   - known filter, matching url     → return that project
 *   - known filter, mismatching url  → error (cannot override known URL)
 *   - unknown filter, empty url      → error (pass --repo-url)
 *   - unknown filter, url set        → return ad-hoc ProjectConfig with that URL
 */
func resolveTargetProjects(repositoryFilter, repositoryUrl string) ([]project.ProjectConfig, error) {
    if "" == repositoryFilter {
        if "" != repositoryUrl {
            return nil, fmt.Errorf("--repo-url requires --repo")
        }
        return project.Projects, nil
    }

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
