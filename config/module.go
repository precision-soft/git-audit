package config

import (
    applicationcontract "github.com/precision-soft/melody/v3/application/contract"
)

func NewGithubAuditModule() *GithubAuditModule {
    return &GithubAuditModule{}
}

type GithubAuditModule struct{}

func (instance *GithubAuditModule) Name() string {
    return "git-audit"
}

func (instance *GithubAuditModule) Description() string {
    return "github release audit tool"
}

var _ applicationcontract.Module = (*GithubAuditModule)(nil)
