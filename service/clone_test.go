package service

import (
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
