package types

type Status string

const (
    StatusOk      Status = "ok"
    StatusWarning Status = "warning"
    StatusFailed  Status = "failed"
)

type LevelStatus string

const (
    LevelOk            LevelStatus = "ok"
    LevelWarning       LevelStatus = "warning"
    LevelFailed        LevelStatus = "failed"
    LevelNotApplicable LevelStatus = "n/a"
    LevelSkipped       LevelStatus = "-"
)
