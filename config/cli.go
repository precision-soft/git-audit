package config

import (
    "github.com/precision-soft/git-audit/cli"

    applicationcontract "github.com/precision-soft/melody/v3/application/contract"
    clicontract "github.com/precision-soft/melody/v3/cli/contract"
    kernelcontract "github.com/precision-soft/melody/v3/kernel/contract"
)

func (instance *GithubAuditModule) RegisterCliCommands(kernelInstance kernelcontract.Kernel) []clicontract.Command {
    return []clicontract.Command{
        &cli.AuditCommand{},
        &cli.ExceptionsCommand{},
        &cli.SyncCommand{},
    }
}

var _ applicationcontract.CliModule = (*GithubAuditModule)(nil)
