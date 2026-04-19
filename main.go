package main

import (
    "context"
    "io/fs"

    "github.com/precision-soft/git-audit/config"

    "github.com/precision-soft/melody/v3/application"
)

var embeddedPublicFiles fs.FS = nil

func main() {
    ctx := context.Background()

    app := application.NewApplication(
        ctx,
        embeddedEnvFiles,
        embeddedPublicFiles,
    )

    app.RegisterModule(config.NewGithubAuditModule())

    app.Run()
}
