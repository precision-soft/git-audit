package service

import (
    "testing"
)

func TestNewGithubReleaseServiceToken(t *testing.T) {
    svc := NewGithubReleaseService("my-token")

    if "my-token" != svc.Token() {
        t.Errorf("expected token %q, got %q", "my-token", svc.Token())
    }
}

func TestNewGithubReleaseServiceClientNotNil(t *testing.T) {
    svc := NewGithubReleaseService("any-token")

    if nil == svc.Client() {
        t.Errorf("expected non-nil GithubClient")
    }
}

func TestNewGithubReleaseServiceEmptyToken(t *testing.T) {
    svc := NewGithubReleaseService("")

    if "" != svc.Token() {
        t.Errorf("empty token should be preserved, got %q", svc.Token())
    }
    if nil == svc.Client() {
        t.Errorf("expected non-nil GithubClient even with empty token")
    }
}

func TestNewGithubReleaseServiceTokenRoundtrip(t *testing.T) {
    token := "ghp_abc123XYZ"
    svc := NewGithubReleaseService(token)

    if token != svc.Token() {
        t.Errorf("Token() = %q, want %q", svc.Token(), token)
    }
    if nil == svc.Client() {
        t.Errorf("Client() should not be nil")
    }
}
