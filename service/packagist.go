package service

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
)

type PackagistVersion struct {
    Version   string
    Reference string
}

type packagistPackageVersion struct {
    Version string `json:"version"`
    Source  struct {
        Reference string `json:"reference"`
    } `json:"source"`
    Dist struct {
        Reference string `json:"reference"`
    } `json:"dist"`
}

type packagistResponse struct {
    Packages map[string][]packagistPackageVersion `json:"packages"`
}

var packagistClient = newHttpClient()

func GetPackagistPackageVersions(packageName string) ([]PackagistVersion, error) {
    url := fmt.Sprintf("https://repo.packagist.org/p2/%s.json", packageName)

    request, requestErr := http.NewRequest(http.MethodGet, url, nil)
    if nil != requestErr {
        return nil, requestErr
    }

    response, doErr := doWithRetry(packagistClient, request)
    if nil != doErr {
        return nil, doErr
    }
    defer response.Body.Close()

    body, readErr := io.ReadAll(response.Body)
    if nil != readErr {
        return nil, readErr
    }

    if response.StatusCode < 200 || response.StatusCode >= 300 {
        return nil, fmt.Errorf("http %d: %s", response.StatusCode, string(body))
    }

    var result packagistResponse
    if unmarshalErr := json.Unmarshal(body, &result); nil != unmarshalErr {
        return nil, unmarshalErr
    }

    versionList, hasPackage := result.Packages[packageName]
    if false == hasPackage {
        return nil, fmt.Errorf("package %s not found in packagist payload", packageName)
    }

    versions := make([]PackagistVersion, 0, len(versionList))
    for _, versionEntry := range versionList {
        version := strings.TrimSpace(versionEntry.Version)
        if "" == version {
            continue
        }

        reference := strings.TrimSpace(versionEntry.Source.Reference)
        if "" == reference {
            reference = strings.TrimSpace(versionEntry.Dist.Reference)
        }

        versions = append(versions, PackagistVersion{
            Version:   version,
            Reference: reference,
        })
    }

    return versions, nil
}
