package config

import (
    applicationcontract "github.com/precision-soft/melody/v3/application/contract"
)

const (
    ParameterGithubToken   = "github.token"
    ParameterGithubApiBase = "github.api_base"
    ParameterGithubRawBase = "github.raw_base"

    EnvironmentGithubToken   = "GITHUB_TOKEN"
    EnvironmentGithubApiBase = "GITHUB_API_BASE"
    EnvironmentGithubRawBase = "GITHUB_RAW_BASE"
)

/* melody's debug:parameters prints resolved values, and each spelling of the token is secret independently */
func (instance *GithubAuditModule) RegisterParameters(registrar applicationcontract.ParameterRegistrar) {
    registrar.RegisterSecretParameter(ParameterGithubToken, "%env(default::"+EnvironmentGithubToken+")%")
    registrar.MarkParameterSecret(EnvironmentGithubToken)
    registrar.RegisterParameter(ParameterGithubApiBase, "%env(default::"+EnvironmentGithubApiBase+")%")
    registrar.RegisterParameter(ParameterGithubRawBase, "%env(default::"+EnvironmentGithubRawBase+")%")
}

var _ applicationcontract.ParameterModule = (*GithubAuditModule)(nil)
