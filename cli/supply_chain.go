package cli

import (
    "errors"
    "fmt"
    "net/http"
    "path/filepath"
    "strings"

    "github.com/precision-soft/git-audit/service"
    "github.com/precision-soft/git-audit/types"
)

type assetRequirement struct {
    check   string
    markers []string
}

const supplyChainUnavailable = "supply-chain check unavailable: "

var assetRequirements = []assetRequirement{
    {check: "checksums", markers: []string{"sha256", "sha512", "checksums"}},
    {check: "sbom", markers: []string{"sbom", "spdx", "cyclonedx"}},
}

/*
applySupplyChainAudits runs the opt-in checks after the main pass and never aborts the command:
a project whose GitHub calls fail keeps the audit it already has and carries the failure on its
supply-chain level, the same way the main pass records a fetch error instead of exiting.
*/
func applySupplyChainAudits(githubClient *service.GithubClient, audits []types.ProjectAudit, requestedChecks string, failOnFinding bool) error {
    checks, parseErr := parseSupplyChainChecks(requestedChecks)
    if nil != parseErr || 0 == len(checks) {
        return parseErr
    }
    issueStatus := types.LevelWarning
    if true == failOnFinding {
        issueStatus = types.LevelFailed
    }
    for auditIndex := range audits {
        audit := &audits[auditIndex]
        if "" != audit.FetchError {
            continue
        }
        releases, fetchErr := githubClient.GetReleases(audit.OrganizationName, audit.RepositoryName)
        byTag := make(map[string]service.GithubRelease, len(releases))
        for _, release := range releases {
            byTag[release.TagName] = release
        }
        for releaseIndex := range audit.Releases {
            releaseAudit := &audit.Releases[releaseIndex]
            var result types.LevelResult
            if nil != fetchErr {
                result = types.LevelResult{Status: issueStatus, Issues: []string{supplyChainUnavailable + fetchErr.Error()}}
            } else {
                result = auditReleaseSupplyChain(githubClient, audit, releaseAudit.TagName, byTag[releaseAudit.TagName], checks, issueStatus)
            }
            releaseAudit.SupplyChain = result
            releaseAudit.Status = computeReleaseStatus(*releaseAudit)
        }
        recomputeProjectAggregates(audit)
    }
    return nil
}

func auditReleaseSupplyChain(
    githubClient *service.GithubClient,
    audit *types.ProjectAudit,
    tag string,
    release service.GithubRelease,
    checks map[string]bool,
    issueStatus types.LevelStatus,
) types.LevelResult {
    result := types.LevelResult{Status: types.LevelOk}
    addIssue := func(issue string) {
        result.Status = issueStatus
        result.Issues = append(result.Issues, issue)
    }
    if true == checks["signed-tags"] {
        verified, reason, verifyErr := githubClient.GetTagVerification(audit.OrganizationName, audit.RepositoryName, tag)
        switch {
        case true == isHttpStatus(verifyErr, http.StatusNotFound):
            addIssue("tag signature is not verified (tag reference not found)")
        case nil != verifyErr:
            addIssue(supplyChainUnavailable + verifyErr.Error())
        case false == verified:
            addIssue("tag signature is not verified (" + reason + ")")
        }
    }
    assetNames := make([]string, 0, len(release.Assets))
    for _, asset := range release.Assets {
        assetNames = append(assetNames, strings.ToLower(filepath.Base(asset.Name)))
    }
    for _, requirement := range assetRequirements {
        if false == checks[requirement.check] || true == containsAssetMarker(assetNames, requirement.markers) {
            continue
        }
        addIssue("release has no " + requirement.check + " artifact")
    }
    if true == checks["attestations"] {
        hasAttestation := false
        var attestationErr error
        for _, asset := range release.Assets {
            var verified bool
            verified, attestationErr = githubClient.HasArtifactAttestation(audit.OrganizationName, audit.RepositoryName, asset.Digest)
            if nil != attestationErr || true == verified {
                hasAttestation = verified
                break
            }
        }
        switch {
        case nil != attestationErr:
            addIssue(supplyChainUnavailable + attestationErr.Error())
        case false == hasAttestation:
            addIssue("release has no verified github artifact attestation")
        }
    }
    return result
}

func isHttpStatus(err error, statusCode int) bool {
    var statusErr *service.HttpStatusError
    return true == errors.As(err, &statusErr) && statusCode == statusErr.StatusCode
}

func parseSupplyChainChecks(value string) (map[string]bool, error) {
    checks := make(map[string]bool)
    for _, item := range strings.Split(value, ",") {
        item = strings.TrimSpace(item)
        if "" == item {
            continue
        }
        if "all" == item {
            for _, name := range []string{"signed-tags", "checksums", "sbom", "attestations"} {
                checks[name] = true
            }
            continue
        }
        switch item {
        case "signed-tags", "checksums", "sbom", "attestations":
            checks[item] = true
        default:
            return nil, fmt.Errorf("unknown supply-chain check %q", item)
        }
    }
    return checks, nil
}

func containsAssetMarker(names, markers []string) bool {
    for _, name := range names {
        for _, marker := range markers {
            if true == strings.Contains(name, marker) {
                return true
            }
        }
    }
    return false
}
