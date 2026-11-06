package service

import (
    "os"
    "os/exec"
    "path/filepath"
    "slices"
    "strings"
    "testing"
)

func TestEnsureCloneResetRejectsInvalidInput(t *testing.T) {
    cases := []struct {
        name          string
        cloneName     string
        repositoryUrl string
        wantErr       string
    }{
        {"empty name", "", "https://example.com/repo.git", "clone name is required"},
        {"forward slash", "a/b", "https://example.com/repo.git", "invalid clone name"},
        {"backslash", "a\\b", "https://example.com/repo.git", "invalid clone name"},
        {"parent traversal", "..", "https://example.com/repo.git", "invalid clone name"},
        {"escaping path", "../escape", "https://example.com/repo.git", "invalid clone name"},
        {"empty url", "valid", "", "repo url is required"},
        {"option-like url", "valid", "--upload-pack=touch", "invalid repo url"},
        {"ext transport url", "valid", "-ext::sh -c touch", "invalid repo url"},
    }

    for _, testCase := range cases {
        t.Run(testCase.name, func(t *testing.T) {
            path, err := EnsureCloneReset(testCase.cloneName, testCase.repositoryUrl)
            if nil == err {
                t.Fatalf("expected error, got path %q", path)
            }
            if false == strings.Contains(err.Error(), testCase.wantErr) {
                t.Errorf("error %q does not contain %q", err.Error(), testCase.wantErr)
            }
        })
    }
}

func TestEnsureCloneResetRefusesAnEmbeddedCredential(t *testing.T) {
    _, err := EnsureCloneReset("valid", "https://user:ghp_secret@github.com/acme/widget")
    if nil == err {
        t.Fatal("expected a url carrying a credential to be refused")
    }
    if true == strings.Contains(err.Error(), "ghp_secret") {
        t.Errorf("the credential must not reach the message, got %q", err.Error())
    }
}

func TestEnsureCloneResetRejectsTheCloneRootItself(t *testing.T) {
    if _, err := EnsureCloneReset(".", "https://github.com/acme/widget"); nil == err {
        t.Fatal("expected the name . to be refused")
    }
}

func TestEnsureCloneResetLeavesAnEnclosingRepositoryAlone(t *testing.T) {
    enclosing := t.TempDir()
    createCommittedRepository(t, enclosing)
    if output, remoteErr := exec.Command("git", "-C", enclosing, "remote", "add", "origin", "file://"+enclosing).CombinedOutput(); nil != remoteErr {
        t.Fatalf("git remote add: %v: %s", remoteErr, output)
    }
    t.Chdir(enclosing)

    work := filepath.Join(enclosing, "README")
    if writeErr := os.WriteFile(work, []byte("uncommitted"), 0o644); nil != writeErr {
        t.Fatal(writeErr)
    }
    target := filepath.Join(cloneRootDirectory, "widget")
    if mkdirErr := os.MkdirAll(target, 0o755); nil != mkdirErr {
        t.Fatal(mkdirErr)
    }
    if writeErr := os.WriteFile(filepath.Join(target, "leftover"), []byte("x"), 0o644); nil != writeErr {
        t.Fatal(writeErr)
    }

    _, err := EnsureCloneReset("widget", "file://"+enclosing)
    if nil == err {
        t.Fatal("expected a directory that is not a clone to be refused")
    }

    content, readErr := os.ReadFile(work)
    if nil != readErr || "uncommitted" != string(content) {
        t.Fatalf("the enclosing repository's work must survive, got %q / %v", content, readErr)
    }
}

func TestEnsureCloneResetClonesIntoAnEmptyDirectory(t *testing.T) {
    origin := t.TempDir()
    createCommittedRepository(t, origin)
    t.Chdir(t.TempDir())

    if mkdirErr := os.MkdirAll(filepath.Join(cloneRootDirectory, "widget"), 0o755); nil != mkdirErr {
        t.Fatal(mkdirErr)
    }

    target, err := EnsureCloneReset("widget", "file://"+origin)
    if nil != err {
        t.Fatal(err)
    }
    if _, statErr := os.Stat(filepath.Join(target, "README")); nil != statErr {
        t.Fatalf("expected the clone's content: %v", statErr)
    }
}

func TestEnsureCloneResetRefusesACloneOfAnotherRepository(t *testing.T) {
    firstOrigin := t.TempDir()
    createCommittedRepository(t, firstOrigin)
    secondOrigin := t.TempDir()
    createCommittedRepository(t, secondOrigin)
    t.Chdir(t.TempDir())

    if _, err := EnsureCloneReset("widget", "file://"+firstOrigin); nil != err {
        t.Fatal(err)
    }
    if _, err := EnsureCloneReset("widget", "file://"+firstOrigin); nil != err {
        t.Fatalf("a second run on the same origin must reset, got %v", err)
    }

    _, err := EnsureCloneReset("widget", "file://"+secondOrigin)
    if nil == err || false == strings.Contains(err.Error(), "is a clone of") {
        t.Fatalf("expected the other origin to be refused, got %v", err)
    }
}

func TestNormalizeRepositoryUrlUnifiesTheSpellingsOfOneRepository(t *testing.T) {
    expected := "github.com/acme/widget"
    for _, spelling := range []string{
        "https://github.com/acme/widget",
        "https://github.com/acme/widget.git",
        "https://github.com/Acme/Widget/",
        "git@github.com:acme/widget.git",
        "ssh://git@github.com/acme/widget.git",
        "https://github.com/acme/widget\n",
    } {
        if normalized := NormalizeRepositoryUrl(spelling); expected != normalized {
            t.Errorf("%q: expected %q, got %q", spelling, expected, normalized)
        }
    }
    if expected == NormalizeRepositoryUrl("https://github.com/other/widget") {
        t.Error("another owner must stay another repository")
    }
}

func TestGitEnvironmentForbidsPromptsAndKeepsTheCallersSshCommand(t *testing.T) {
    defaults := gitEnvironment([]string{"PATH=/bin"})
    if false == slices.Contains(defaults, "GIT_TERMINAL_PROMPT=0") || false == slices.Contains(defaults, "GIT_SSH_COMMAND=ssh -o BatchMode=yes") {
        t.Errorf("expected both prompts disabled, got %v", defaults)
    }

    custom := gitEnvironment([]string{"GIT_SSH_COMMAND=ssh -i key"})
    if true == slices.Contains(custom, "GIT_SSH_COMMAND=ssh -o BatchMode=yes") {
        t.Errorf("the caller's ssh command must stay, got %v", custom)
    }
}

func createCommittedRepository(t *testing.T, directory string) {
    t.Helper()

    if writeErr := os.WriteFile(filepath.Join(directory, "README"), []byte("readme"), 0o644); nil != writeErr {
        t.Fatal(writeErr)
    }
    for _, arguments := range [][]string{
        {"init", "--initial-branch", "main"},
        {"add", "README"},
        {"-c", "user.name=git-audit test", "-c", "user.email=test@example.com", "commit", "-m", "initial"},
    } {
        command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
        if output, runErr := command.CombinedOutput(); nil != runErr {
            t.Fatalf("git %v: %v: %s", arguments, runErr, output)
        }
    }
}
