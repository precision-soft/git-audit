package project

type ProjectConfig struct {
    Name           string   `json:"name"`
    GithubUrl      string   `json:"githubUrl"`
    PackagistUrl   string   `json:"packagistUrl"`
    GoSubmodule    bool     `json:"goSubmodule"`
    ChangelogPaths []string `json:"changelogPaths"`
}

var Projects = []ProjectConfig{
    {
        GithubUrl:    "https://github.com/precision-soft/doctrine-type",
        PackagistUrl: "https://packagist.org/packages/precision-soft/doctrine-type",
    },
    {
        GithubUrl:    "https://github.com/precision-soft/doctrine-utility",
        PackagistUrl: "https://packagist.org/packages/precision-soft/doctrine-utility",
    },
    {
        GithubUrl:    "https://github.com/precision-soft/symfony-console",
        PackagistUrl: "https://packagist.org/packages/precision-soft/symfony-console",
    },
    {
        GithubUrl:    "https://github.com/precision-soft/symfony-doctrine-audit",
        PackagistUrl: "https://packagist.org/packages/precision-soft/symfony-doctrine-audit",
    },
    {
        GithubUrl:    "https://github.com/precision-soft/symfony-doctrine-encrypt",
        PackagistUrl: "https://packagist.org/packages/precision-soft/symfony-doctrine-encrypt",
    },
    {
        GithubUrl:    "https://github.com/precision-soft/symfony-json-form",
        PackagistUrl: "https://packagist.org/packages/precision-soft/symfony-json-form",
    },
    {
        GithubUrl:    "https://github.com/precision-soft/symfony-phpunit",
        PackagistUrl: "https://packagist.org/packages/precision-soft/symfony-phpunit",
    },
    {
        Name:        "Melody",
        GithubUrl:   "https://github.com/precision-soft/melody",
        GoSubmodule: true,
        ChangelogPaths: []string{
            "CHANGELOG.md",
            "v2/CHANGELOG.md",
            "v3/CHANGELOG.md",
        },
    },
    {
        Name:      "Git Audit",
        GithubUrl: "https://github.com/precision-soft/git-audit",
    },
}
