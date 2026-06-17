package cli

import (
    "fmt"
    "sort"
    "strings"
    "time"

    clicontract "github.com/precision-soft/melody/v3/cli/contract"
    "github.com/precision-soft/melody/v3/cli/output"
    runtimecontract "github.com/precision-soft/melody/v3/runtime/contract"
)

type ExceptionsCommand struct{}

func (instance *ExceptionsCommand) Name() string {
    return "exceptions"
}

func (instance *ExceptionsCommand) Description() string {
    return "List exceptions from exceptions file, grouped by project"
}

func (instance *ExceptionsCommand) Flags() []clicontract.Flag {
    return output.MergeFlags(
        output.DebugFlags(),
        []clicontract.Flag{
            &clicontract.StringFlag{
                Name:  flagExceptions,
                Usage: "path to exceptions JSON file",
                Value: "exceptions.json",
            },
        },
    )
}

func (instance *ExceptionsCommand) Run(
    _ runtimecontract.Runtime,
    commandContext *clicontract.CommandContext,
) error {
    startedAt := time.Now()

    exceptionsFile := strings.TrimSpace(commandContext.String(flagExceptions))

    exceptions, loadErr := loadExceptions(exceptionsFile)
    if nil != loadErr {
        return fmt.Errorf("load exceptions: %w", loadErr)
    }

    option := output.NormalizeOption(output.ParseOptionFromCommand(commandContext))

    meta := output.NewMeta(
        instance.Name(),
        commandContext.Args().Slice(),
        option,
        startedAt,
        time.Since(startedAt),
        output.Version{},
    )

    envelope := output.NewEnvelope(meta)

    if option.Format == output.FormatTable {
        builder := output.NewTableBuilder()

        totalCount := 0
        expiredCount := 0
        now := time.Now()
        for _, versionMap := range exceptions {
            for _, levelMap := range versionMap {
                for _, entries := range levelMap {
                    for _, entry := range entries {
                        totalCount++
                        if false == entry.Active(now) {
                            expiredCount++
                        }
                    }
                }
            }
        }
        builder.AddSummaryLine(fmt.Sprintf("repos: %d | exceptions: %d | expired: %d", len(exceptions), totalCount, expiredCount))

        repositories := make([]string, 0, len(exceptions))
        for repository := range exceptions {
            repositories = append(repositories, repository)
        }
        sort.Strings(repositories)

        for _, repository := range repositories {
            versionMap := exceptions[repository]
            versions := make([]string, 0, len(versionMap))
            for version := range versionMap {
                versions = append(versions, version)
            }
            sort.Strings(versions)

            block := builder.AddBlock(repository, []string{"version", "level", "issue", "reviewed_until"})
            for _, version := range versions {
                levelMap := versionMap[version]
                levels := make([]string, 0, len(levelMap))
                for level := range levelMap {
                    levels = append(levels, level)
                }
                sort.Strings(levels)
                for _, level := range levels {
                    for _, entry := range levelMap[level] {
                        reviewedUntil := "-"
                        if false == entry.ReviewedUntil.IsZero() {
                            reviewedUntil = entry.ReviewedUntil.Format(exceptionDateLayout)
                            if false == entry.Active(now) {
                                reviewedUntil += " (expired)"
                            }
                        }
                        block.AddRow(version, level, entry.Issue, reviewedUntil)
                    }
                }
            }
        }

        envelope.Table = builder.Build()
    } else {
        envelope.Data = exceptions
    }

    envelope.Meta.DurationMilliseconds = time.Since(startedAt).Milliseconds()

    return output.Render(commandContext.Writer, envelope, option)
}

var _ clicontract.Command = (*ExceptionsCommand)(nil)
