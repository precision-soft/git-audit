package cli

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/precision-soft/git-audit/config/project"
    "github.com/precision-soft/git-audit/types"
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

func TestGithubAnnotationsEscapeCommands(t *testing.T) {
    audits := auditWithLevelResult("integrity", types.LevelResult{Status: types.LevelFailed, Issues: []string{"bad: value\nnext"}})
    var output strings.Builder
    renderGithubAnnotations(&output, audits)
    rendered := output.String()
    if false == strings.Contains(rendered, "::error") || false == strings.Contains(rendered, "bad%3A value%0Anext") {
        t.Fatalf("unexpected annotation: %s", rendered)
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
