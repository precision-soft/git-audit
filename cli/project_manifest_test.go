package cli

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/precision-soft/git-audit/config/project"
)

func TestProjectManifestMergeOverridesByNameAndAddsProjects(t *testing.T) {
    path := writeManifest(t, `{"mode":"merge","projects":[{"name":"Existing","githubUrl":"https://github.com/acme/replaced"},{"name":"New","githubUrl":"https://github.com/acme/new","changelogPaths":["docs/CHANGELOG.md"]}]}`)
    projects, loadErr := loadProjectManifest(path, []project.ProjectConfig{{Name: "Existing", GithubUrl: "https://github.com/acme/old"}})
    if nil != loadErr {
        t.Fatal(loadErr)
    }
    if 2 != len(projects) || "https://github.com/acme/replaced" != projects[0].GithubUrl {
        t.Fatalf("unexpected merged projects: %#v", projects)
    }
}

func TestProjectManifestRejectsUnsafeChangelogPath(t *testing.T) {
    _, loadErr := loadProjectManifest(writeManifest(t, `{"mode":"replace","projects":[{"name":"Bad","githubUrl":"https://github.com/acme/bad","changelogPaths":["../secret"]}]}`), nil)
    if nil == loadErr || false == strings.Contains(loadErr.Error(), "unsafe changelog") {
        t.Fatalf("expected unsafe path error, got %v", loadErr)
    }
}

func writeManifest(t *testing.T, body string) string {
    t.Helper()
    path := filepath.Join(t.TempDir(), "projects.json")
    if writeErr := os.WriteFile(path, []byte(body), 0o600); nil != writeErr {
        t.Fatal(writeErr)
    }
    return path
}

func TestProjectManifestRejectsUnknownFields(t *testing.T) {
    _, loadErr := loadProjectManifest(writeManifest(t, `{"mode":"replace","projects":[{"name":"Typo","github_url":"https://github.com/acme/typo"}]}`), nil)
    if nil == loadErr || false == strings.Contains(loadErr.Error(), "unknown field") {
        t.Fatalf("a misspelled key must be rejected, not ignored, got %v", loadErr)
    }
}

func TestValidateManifestProjectRequiresOwnerAndRepository(t *testing.T) {
    for _, githubUrl := range []string{"https://github.com/", "https://github.com/owner", "https://github.com/owner/"} {
        validateErr := validateManifestProject(project.ProjectConfig{Name: "Short", GithubUrl: githubUrl})
        if nil == validateErr || false == strings.Contains(validateErr.Error(), "invalid github url") {
            t.Errorf("%q must be rejected, got %v", githubUrl, validateErr)
        }
    }
    if validateErr := validateManifestProject(project.ProjectConfig{Name: "Full", GithubUrl: "https://github.com/owner/repository"}); nil != validateErr {
        t.Errorf("an owner/repository url must pass, got %v", validateErr)
    }
}
