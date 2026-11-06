package main

import (
    "context"
    "io/fs"

    "github.com/precision-soft/git-audit/config"

    melodyapplication "github.com/precision-soft/melody/v3/application"
)

var embeddedPublicFiles fs.FS = nil

func main() {
    application := melodyapplication.NewApplication(
        context.Background(),
        embeddedEnvFiles,
        embeddedPublicFiles,
    )

    application.RegisterModule(config.NewGithubAuditModule())

    application.Run()
}
