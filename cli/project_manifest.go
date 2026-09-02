package cli

import (
    "encoding/json"
    "fmt"
    "net/url"
    "os"
    "path/filepath"
    "strings"

    "github.com/precision-soft/git-audit/config/project"
)

type projectManifest struct {
    Mode     string                  `json:"mode"`
    Projects []project.ProjectConfig `json:"projects"`
}

func loadProjectManifest(manifestPath string, builtInProjects []project.ProjectConfig) ([]project.ProjectConfig, error) {
    if "" == strings.TrimSpace(manifestPath) {
        return builtInProjects, nil
    }

    manifestData, readErr := os.ReadFile(manifestPath)
    if nil != readErr {
        return nil, fmt.Errorf("read project configuration: %w", readErr)
    }
    var manifest projectManifest
    if decodeErr := json.Unmarshal(manifestData, &manifest); nil != decodeErr {
        return nil, fmt.Errorf("decode project configuration: %w", decodeErr)
    }
    if "" == manifest.Mode {
        manifest.Mode = "merge"
    }
    if "merge" != manifest.Mode && "replace" != manifest.Mode {
        return nil, fmt.Errorf("project configuration mode must be merge or replace")
    }

    projects := append([]project.ProjectConfig(nil), builtInProjects...)
    if "replace" == manifest.Mode {
        projects = nil
    }
    byName := make(map[string]int)
    byUrl := make(map[string]int)
    for index, projectConfig := range projects {
        byName[strings.ToLower(projectConfig.Name)] = index
        byUrl[strings.ToLower(strings.TrimSuffix(projectConfig.GithubUrl, "/"))] = index
    }
    for _, projectConfig := range manifest.Projects {
        if validateErr := validateManifestProject(projectConfig); nil != validateErr {
            return nil, validateErr
        }
        nameKey := strings.ToLower(projectConfig.Name)
        urlKey := strings.ToLower(strings.TrimSuffix(projectConfig.GithubUrl, "/"))
        nameIndex, nameExists := byName[nameKey]
        urlIndex, urlExists := byUrl[urlKey]
        if true == nameExists && true == urlExists && nameIndex != urlIndex {
            return nil, fmt.Errorf("project %q conflicts with an existing name and url", projectConfig.Name)
        }
        if true == nameExists {
            delete(byUrl, strings.ToLower(strings.TrimSuffix(projects[nameIndex].GithubUrl, "/")))
            projects[nameIndex] = projectConfig
            byUrl[urlKey] = nameIndex
            continue
        }
        if true == urlExists {
            return nil, fmt.Errorf("duplicate project url %q", projectConfig.GithubUrl)
        }
        byName[nameKey] = len(projects)
        byUrl[urlKey] = len(projects)
        projects = append(projects, projectConfig)
    }
    return projects, nil
}

func validateManifestProject(projectConfig project.ProjectConfig) error {
    if "" == strings.TrimSpace(projectConfig.Name) {
        return fmt.Errorf("project name is required")
    }
    parsed, parseErr := url.Parse(projectConfig.GithubUrl)
    if nil != parseErr || "https" != parsed.Scheme || "github.com" != strings.ToLower(parsed.Host) {
        return fmt.Errorf("project %q has an invalid github url", projectConfig.Name)
    }
    for _, changelogPath := range projectConfig.ChangelogPaths {
        clean := filepath.Clean(changelogPath)
        if true == filepath.IsAbs(changelogPath) || "." == clean || ".." == clean || true == strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
            return fmt.Errorf("project %q has an unsafe changelog path %q", projectConfig.Name, changelogPath)
        }
    }
    return nil
}
