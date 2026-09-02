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

/*
RegisterParameters declares the GitHub token as a credential, so melody's auto-registered
`debug:parameters` renders it redacted instead of in clear text. Both spellings have to be covered
and they are secret independently: ParameterGithubToken is this module's own, EnvironmentGithubToken
is the one melody registers itself from the .env artifacts, and each resolves to the same token.
The two base URLs are plain and optional: unset, the client keeps the public GitHub hosts.
*/
func (instance *GithubAuditModule) RegisterParameters(registrar applicationcontract.ParameterRegistrar) {
    registrar.RegisterSecretParameter(ParameterGithubToken, "%env("+EnvironmentGithubToken+")%")
    registrar.MarkParameterSecret(EnvironmentGithubToken)
    registrar.RegisterParameter(ParameterGithubApiBase, "%env(default::"+EnvironmentGithubApiBase+")%")
    registrar.RegisterParameter(ParameterGithubRawBase, "%env(default::"+EnvironmentGithubRawBase+")%")
}

var _ applicationcontract.ParameterModule = (*GithubAuditModule)(nil)
