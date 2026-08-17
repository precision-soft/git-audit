package config

import (
    applicationcontract "github.com/precision-soft/melody/v3/application/contract"
)

const (
    ParameterGithubToken = "github.token"

    EnvironmentGithubToken = "GITHUB_TOKEN"
)

/*
RegisterParameters declares the GitHub token as a credential, so melody's auto-registered
`debug:parameters` renders it redacted instead of in clear text. Both spellings have to be covered
and they are secret independently: ParameterGithubToken is this module's own, EnvironmentGithubToken
is the one melody registers itself from the .env artifacts, and each resolves to the same token.
*/
func (instance *GithubAuditModule) RegisterParameters(registrar applicationcontract.ParameterRegistrar) {
    registrar.RegisterSecretParameter(ParameterGithubToken, "%env("+EnvironmentGithubToken+")%")
    registrar.MarkParameterSecret(EnvironmentGithubToken)
}

var _ applicationcontract.ParameterModule = (*GithubAuditModule)(nil)
