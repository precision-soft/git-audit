package config

import (
    "strings"

    "github.com/precision-soft/git-audit/service"

    applicationcontract "github.com/precision-soft/melody/v3/application/contract"
    melodyconfig "github.com/precision-soft/melody/v3/config"
    containercontract "github.com/precision-soft/melody/v3/container/contract"
)

func (instance *GithubAuditModule) RegisterServices(registrar applicationcontract.ServiceRegistrar) {
    registrar.RegisterService(
        service.ServiceGithubRelease,
        func(resolver containercontract.Resolver) (*service.GithubReleaseService, error) {
            configuration := melodyconfig.ConfigMustFromResolver(resolver)
            token := strings.TrimSpace(configuration.Get(ParameterGithubToken).String())
            return service.NewGithubReleaseService(token), nil
        },
    )
}

var _ applicationcontract.ServiceModule = (*GithubAuditModule)(nil)
