package service

import (
    melodyruntime "github.com/precision-soft/melody/v3/runtime"
    runtimecontract "github.com/precision-soft/melody/v3/runtime/contract"
)

const ServiceGithubRelease = "git-audit.github-release"

type GithubReleaseService struct {
    token  string
    client *GithubClient
}

func NewGithubReleaseService(token string) *GithubReleaseService {
    return &GithubReleaseService{
        token:  token,
        client: NewGithubClient(token),
    }
}

func (s *GithubReleaseService) Token() string {
    return s.token
}

func (s *GithubReleaseService) Client() *GithubClient {
    return s.client
}

func GithubReleaseServiceMustFromRuntime(runtimeInstance runtimecontract.Runtime) *GithubReleaseService {
    return melodyruntime.MustFromRuntime[*GithubReleaseService](runtimeInstance, ServiceGithubRelease)
}
