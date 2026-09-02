package service

import (
    melodyruntime "github.com/precision-soft/melody/v3/runtime"
    runtimecontract "github.com/precision-soft/melody/v3/runtime/contract"
)

const ServiceGithubRelease = "git-audit.github-release"

type GithubReleaseService struct {
    token   string
    apiBase string
    rawBase string
    client  *GithubClient
}

func NewGithubReleaseService(token, apiBase, rawBase string) *GithubReleaseService {
    instance := &GithubReleaseService{
        token:   token,
        apiBase: apiBase,
        rawBase: rawBase,
    }
    instance.client = instance.NewClientWithToken(token)

    return instance
}

func (instance *GithubReleaseService) Token() string {
    return instance.token
}

func (instance *GithubReleaseService) Client() *GithubClient {
    return instance.client
}

func (instance *GithubReleaseService) NewClientWithToken(token string) *GithubClient {
    client := NewGithubClient(token)
    client.SetEndpoints(instance.apiBase, instance.rawBase)

    return client
}

func GithubReleaseServiceMustFromRuntime(runtimeInstance runtimecontract.Runtime) *GithubReleaseService {
    return melodyruntime.MustFromRuntime[*GithubReleaseService](runtimeInstance, ServiceGithubRelease)
}
