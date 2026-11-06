package service

import (
    "context"
    "fmt"
    "net/url"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

const (
    cloneRootDirectory = ".dev-data/clones"
    gitCommandTimeout  = 10 * time.Minute
    gitWaitDelay       = 5 * time.Second
)

func EnsureCloneReset(name, repositoryUrl string) (string, error) {
    if "" == name {
        return "", fmt.Errorf("clone name is required")
    }
    if "." == name || true == strings.ContainsAny(name, "/\\") || true == strings.Contains(name, "..") {
        return "", fmt.Errorf("invalid clone name %q: must not be %q or contain path separators or %q", name, ".", "..")
    }
    if "" == repositoryUrl {
        return "", fmt.Errorf("repo url is required for %q", name)
    }
    if true == strings.HasPrefix(repositoryUrl, "-") {
        return "", fmt.Errorf("invalid repo url %q: must not start with %q", repositoryUrl, "-")
    }
    if true == urlCarriesCredentials(repositoryUrl) {
        return "", fmt.Errorf("invalid repo url for %q: credentials in the url are not accepted, use ssh or a git credential helper", name)
    }

    target := filepath.Join(cloneRootDirectory, name)

    info, statErr := os.Stat(target)
    if nil != statErr && false == os.IsNotExist(statErr) {
        return "", fmt.Errorf("stat %q: %w", target, statErr)
    }

    if nil != statErr {
        parent := filepath.Dir(target)
        if mkdirErr := os.MkdirAll(parent, 0o755); nil != mkdirErr {
            return "", fmt.Errorf("mkdir %q: %w", parent, mkdirErr)
        }
        return cloneInto(target, repositoryUrl)
    }
    if false == info.IsDir() {
        return "", fmt.Errorf("%q exists and is not a directory", target)
    }

    entries, readErr := os.ReadDir(target)
    if nil != readErr {
        return "", fmt.Errorf("read %q: %w", target, readErr)
    }
    if 0 == len(entries) {
        return cloneInto(target, repositoryUrl)
    }

    /* git -C walks up to any enclosing repository, and the reset below would destroy its work */
    if verifyErr := verifyOwnClone(target); nil != verifyErr {
        return "", fmt.Errorf("%q is not a usable clone, remove it to clone again: %w", target, verifyErr)
    }

    origin, originErr := runGitOutput(target, "remote", "get-url", "origin")
    if nil != originErr {
        return "", fmt.Errorf("read origin of %q: %w", target, originErr)
    }
    if NormalizeRepositoryUrl(origin) != NormalizeRepositoryUrl(repositoryUrl) {
        return "", fmt.Errorf(
            "%q is a clone of %s, not of %s; remove it to clone again",
            target,
            NormalizeRepositoryUrl(origin),
            NormalizeRepositoryUrl(repositoryUrl),
        )
    }

    if fetchErr := runGit(target, "fetch", "--tags", "--prune", "--force", "origin"); nil != fetchErr {
        return "", fmt.Errorf("fetch %q: %w", target, fetchErr)
    }

    defaultBranch, branchErr := resolveDefaultBranch(target)
    if nil != branchErr {
        return "", fmt.Errorf("resolve default branch for %q: %w", target, branchErr)
    }

    if checkoutErr := runGit(target, "checkout", "--quiet", defaultBranch); nil != checkoutErr {
        return "", fmt.Errorf("checkout %s in %q: %w", defaultBranch, target, checkoutErr)
    }
    if resetErr := runGit(target, "reset", "--hard", "origin/"+defaultBranch); nil != resetErr {
        return "", fmt.Errorf("reset %q to origin/%s: %w", target, defaultBranch, resetErr)
    }
    if cleanErr := runGit(target, "clean", "-fdx"); nil != cleanErr {
        return "", fmt.Errorf("clean %q: %w", target, cleanErr)
    }

    return target, nil
}

func NormalizeRepositoryUrl(repositoryUrl string) string {
    normalized := strings.ToLower(strings.TrimSpace(repositoryUrl))
    normalized = strings.TrimSuffix(strings.TrimRight(normalized, "/"), ".git")
    for _, scheme := range []string{"https://", "http://", "ssh://", "git://"} {
        normalized = strings.TrimPrefix(normalized, scheme)
    }

    atIndex := strings.IndexByte(normalized, '@')
    slashIndex := strings.IndexByte(normalized, '/')
    if 0 <= atIndex && (-1 == slashIndex || slashIndex > atIndex) {
        normalized = normalized[atIndex+1:]
    }

    return strings.Replace(normalized, ":", "/", 1)
}

func urlCarriesCredentials(repositoryUrl string) bool {
    parsed, parseErr := url.Parse(repositoryUrl)
    if nil != parseErr {
        return false
    }

    return ("https" == parsed.Scheme || "http" == parsed.Scheme) && nil != parsed.User
}

func cloneInto(target, repositoryUrl string) (string, error) {
    if cloneErr := runGit("", "clone", "--", repositoryUrl, target); nil != cloneErr {
        return "", fmt.Errorf("clone %s into %q: %w", repositoryUrl, target, cloneErr)
    }

    return target, nil
}

func verifyOwnClone(target string) error {
    topLevel, topLevelErr := runGitOutput(target, "rev-parse", "--show-toplevel")
    if nil != topLevelErr {
        return topLevelErr
    }

    resolvedTarget, targetErr := resolvePath(target)
    if nil != targetErr {
        return targetErr
    }
    resolvedTopLevel, topLevelPathErr := resolvePath(strings.TrimSpace(topLevel))
    if nil != topLevelPathErr {
        return topLevelPathErr
    }
    if resolvedTarget != resolvedTopLevel {
        return fmt.Errorf("it sits inside the repository at %q", resolvedTopLevel)
    }

    return nil
}

func resolvePath(path string) (string, error) {
    absolute, absoluteErr := filepath.Abs(path)
    if nil != absoluteErr {
        return "", absoluteErr
    }

    return filepath.EvalSymlinks(absolute)
}

func resolveDefaultBranch(directory string) (string, error) {
    head, refErr := runGitOutput(directory, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
    if nil != refErr {
        if setHeadErr := runGit(directory, "remote", "set-head", "origin", "--auto"); nil != setHeadErr {
            return "", fmt.Errorf("remote set-head: %w", setHeadErr)
        }
        head, refErr = runGitOutput(directory, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
        if nil != refErr {
            return "", refErr
        }
    }
    return strings.TrimPrefix(strings.TrimSpace(head), "origin/"), nil
}

func gitEnvironment(environment []string) []string {
    result := append(append([]string(nil), environment...), "GIT_TERMINAL_PROMPT=0")
    for _, variable := range environment {
        if true == strings.HasPrefix(variable, "GIT_SSH_COMMAND=") {
            return result
        }
    }

    return append(result, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
}

func runGit(directory string, arguments ...string) error {
    _, runErr := runGitOutput(directory, arguments...)
    return runErr
}

func runGitOutput(directory string, arguments ...string) (string, error) {
    timeoutContext, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
    defer cancel()

    gitArguments := arguments
    if "" != directory {
        gitArguments = append([]string{"-C", directory}, arguments...)
    }

    command := exec.CommandContext(timeoutContext, "git", gitArguments...)
    command.Env = gitEnvironment(os.Environ())
    /* a killed git can leave its transport child holding the output pipe open */
    command.WaitDelay = gitWaitDelay

    output, runErr := command.CombinedOutput()
    if nil != runErr {
        return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), runErr, strings.TrimSpace(string(output)))
    }
    return string(output), nil
}
