package types

type LevelResult struct {
    Status LevelStatus
    Issues []string
}

type ReleaseAudit struct {
    TagName      string      `json:"tagName"`
    PreviousTag  string      `json:"previousTag"`
    ReleaseTitle string      `json:"releaseTitle"`
    ReleaseBody  string      `json:"releaseBody"`
    CommitCount  int         `json:"commitCount"`
    ChangedFiles []string    `json:"changedFiles"`
    Integrity    LevelResult `json:"integrity"`
    Distribution LevelResult `json:"distribution"`
    Changelog    LevelResult `json:"changelog"`
    Diff         LevelResult `json:"diff"`
    Presentation LevelResult `json:"presentation"`
    Status       Status      `json:"status"`
}

type ProjectAudit struct {
    OrganizationName    string         `json:"organizationName"`
    RepositoryName      string         `json:"repositoryName"`
    ProjectName         string         `json:"projectName"`
    PackagistPackage    string         `json:"packagistPackage"`
    TagCount            int            `json:"tagCount"`
    ReleaseCount        int            `json:"releaseCount"`
    PackagistCount      int            `json:"packagistCount"`
    SubmoduleTagCount   int            `json:"submoduleTagCount"`
    Releases            []ReleaseAudit `json:"releases"`
    IntegrityStatus     LevelStatus    `json:"integrityStatus"`
    DistributionStatus  LevelStatus    `json:"distributionStatus"`
    ChangelogStatus     LevelStatus    `json:"changelogStatus"`
    ChangelogDisplay    string         `json:"changelogDisplay"`
    DiffStatus          LevelStatus    `json:"diffStatus"`
    DiffDisplay         string         `json:"diffDisplay"`
    PresentationStatus  LevelStatus    `json:"presentationStatus"`
    PresentationDisplay string         `json:"presentationDisplay"`
    Status              Status         `json:"status"`
    FetchError          string         `json:"fetchError,omitempty"`
}
