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

func (instance *GithubReleaseService) Token() string {
    return instance.token
}

func (instance *GithubReleaseService) Client() *GithubClient {
    return instance.client
}

func GithubReleaseServiceMustFromRuntime(runtimeInstance runtimecontract.Runtime) *GithubReleaseService {
    return melodyruntime.MustFromRuntime[*GithubReleaseService](runtimeInstance, ServiceGithubRelease)
}
