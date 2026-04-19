package service

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

const cloneRootDirectory = ".dev-data/clones"

/**
 * EnsureCloneReset clones repositoryUrl into .dev-data/clones/<name>/ when the
 * target directory is missing. If it already exists, the working tree is
 * hard-reset to origin (fetch → checkout default branch → reset --hard
 * origin/<branch> → clean -fdx), discarding any local changes. Returns the
 * relative path of the local clone.
 */
func EnsureCloneReset(name, repositoryUrl string) (string, error) {
    if "" == name {
        return "", fmt.Errorf("clone name is required")
    }
    if "" == repositoryUrl {
        return "", fmt.Errorf("repo url is required for %q", name)
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
        if cloneErr := runGit("", "clone", repositoryUrl, target); nil != cloneErr {
            return "", fmt.Errorf("clone %s into %q: %w", repositoryUrl, target, cloneErr)
        }
        return target, nil
    }
    if false == info.IsDir() {
        return "", fmt.Errorf("%q exists and is not a directory", target)
    }

    if fetchErr := runGit(target, "fetch", "--tags", "--prune", "origin"); nil != fetchErr {
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

func runGit(directory string, arguments ...string) error {
    _, runErr := runGitOutput(directory, arguments...)
    return runErr
}

func runGitOutput(directory string, arguments ...string) (string, error) {
    var command *exec.Cmd
    if "" == directory {
        command = exec.Command("git", arguments...)
    } else {
        command = exec.Command("git", append([]string{"-C", directory}, arguments...)...)
    }
    output, runErr := command.CombinedOutput()
    if nil != runErr {
        return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), runErr, strings.TrimSpace(string(output)))
    }
    return string(output), nil
}
