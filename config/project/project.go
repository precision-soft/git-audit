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
        Name:         "Doctrine Type",
        GithubUrl:    "https://github.com/precision-soft/doctrine-type",
        PackagistUrl: "https://packagist.org/packages/precision-soft/doctrine-type",
    },
    {
        Name:         "Doctrine Utility",
        GithubUrl:    "https://github.com/precision-soft/doctrine-utility",
        PackagistUrl: "https://packagist.org/packages/precision-soft/doctrine-utility",
    },
    {
        Name:         "Symfony Console",
        GithubUrl:    "https://github.com/precision-soft/symfony-console",
        PackagistUrl: "https://packagist.org/packages/precision-soft/symfony-console",
    },
    {
        Name:         "Symfony Doctrine Audit",
        GithubUrl:    "https://github.com/precision-soft/symfony-doctrine-audit",
        PackagistUrl: "https://packagist.org/packages/precision-soft/symfony-doctrine-audit",
    },
    {
        Name:         "Symfony Doctrine Encrypt",
        GithubUrl:    "https://github.com/precision-soft/symfony-doctrine-encrypt",
        PackagistUrl: "https://packagist.org/packages/precision-soft/symfony-doctrine-encrypt",
    },
    {
        Name:         "Symfony JSON Form",
        GithubUrl:    "https://github.com/precision-soft/symfony-json-form",
        PackagistUrl: "https://packagist.org/packages/precision-soft/symfony-json-form",
    },
    {
        Name:         "Symfony PHPUnit",
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
}
