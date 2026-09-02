package cli

import (
    "fmt"
    "path/filepath"
    "strings"

    "github.com/precision-soft/git-audit/service"
    "github.com/precision-soft/git-audit/types"
)

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
        releases, fetchErr := githubClient.GetReleases(audit.OrganizationName, audit.RepositoryName)
        if nil != fetchErr {
            return fmt.Errorf("supply-chain releases for %s/%s: %w", audit.OrganizationName, audit.RepositoryName, fetchErr)
        }
        byTag := make(map[string]service.GithubRelease)
        for _, release := range releases {
            byTag[release.TagName] = release
        }
        for releaseIndex := range audit.Releases {
            result := types.LevelResult{Status: types.LevelOk}
            releaseAudit := &audit.Releases[releaseIndex]
            release := byTag[releaseAudit.TagName]
            if true == checks["signed-tags"] {
                verified, reason, verifyErr := githubClient.GetTagVerification(audit.OrganizationName, audit.RepositoryName, releaseAudit.TagName)
                if nil != verifyErr {
                    return verifyErr
                }
                if false == verified {
                    result.Status = issueStatus
                    result.Issues = append(result.Issues, "tag signature is not verified ("+reason+")")
                }
            }
            assetNames := make([]string, 0, len(release.Assets))
            for _, asset := range release.Assets {
                assetNames = append(assetNames, strings.ToLower(filepath.Base(asset.Name)))
            }
            requirements := map[string][]string{
                "checksums": {"sha256", "sha512", "checksums"},
                "sbom":      {"sbom", "spdx", "cyclonedx"},
            }
            for check, markers := range requirements {
                if false == checks[check] || true == containsAssetMarker(assetNames, markers) {
                    continue
                }
                result.Status = issueStatus
                result.Issues = append(result.Issues, "release has no "+check+" artifact")
            }
            if true == checks["attestations"] {
                hasAttestation := false
                for _, asset := range release.Assets {
                    verified, attestationErr := githubClient.HasArtifactAttestation(
                        audit.OrganizationName,
                        audit.RepositoryName,
                        asset.Digest,
                    )
                    if nil != attestationErr {
                        return fmt.Errorf("artifact attestation for %s/%s %s: %w", audit.OrganizationName, audit.RepositoryName, releaseAudit.TagName, attestationErr)
                    }
                    if true == verified {
                        hasAttestation = true
                        break
                    }
                }
                if false == hasAttestation {
                    result.Status = issueStatus
                    result.Issues = append(result.Issues, "release has no verified github artifact attestation")
                }
            }
            releaseAudit.SupplyChain = result
            releaseAudit.Status = computeReleaseStatus(*releaseAudit)
        }
        recomputeProjectAggregates(audit)
    }
    return nil
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
