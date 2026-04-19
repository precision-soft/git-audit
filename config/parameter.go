package config

import (
    applicationcontract "github.com/precision-soft/melody/v3/application/contract"
)

const (
    ParameterGithubToken = "github.token"
)

func (instance *GithubAuditModule) RegisterParameters(registrar applicationcontract.ParameterRegistrar) {
    registrar.RegisterParameter(ParameterGithubToken, "%env(GITHUB_TOKEN)%")
}

var _ applicationcontract.ParameterModule = (*GithubAuditModule)(nil)
