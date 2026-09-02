package service

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"

    "github.com/precision-soft/melody/v3/httpclient"
    httpclientcontract "github.com/precision-soft/melody/v3/httpclient/contract"
)

type gitReference struct {
    Object struct {
        Type string `json:"type"`
        SHA  string `json:"sha"`
    } `json:"object"`
}

type gitTagObject struct {
    Verification struct {
        Verified bool   `json:"verified"`
        Reason   string `json:"reason"`
    } `json:"verification"`
}

func (instance *GithubClient) GetTagVerification(organization, repository, tag string) (bool, string, error) {
    var reference gitReference
    endpoint := fmt.Sprintf("%s/repos/%s/%s/git/ref/tags/%s", githubApiBase, organization, repository, tag)
    if getErr := instance.get(endpoint, &reference); nil != getErr {
        return false, "", getErr
    }
    if "tag" != reference.Object.Type {
        return false, "lightweight tag", nil
    }
    var tagObject gitTagObject
    endpoint = fmt.Sprintf("%s/repos/%s/%s/git/tags/%s", githubApiBase, organization, repository, reference.Object.SHA)
    if getErr := instance.get(endpoint, &tagObject); nil != getErr {
        return false, "", getErr
    }
    return tagObject.Verification.Verified, tagObject.Verification.Reason, nil
}

func (instance *GithubClient) HasArtifactAttestation(organization, repository, digest string) (bool, error) {
    digest = strings.TrimSpace(digest)
    if "" == digest {
        return false, nil
    }

    endpoint := fmt.Sprintf(
        "%s/repos/%s/%s/attestations/%s",
        githubApiBase,
        organization,
        repository,
        url.PathEscape(digest),
    )
    options := []httpclientcontract.RequestOption{
        httpclient.WithHeaders(githubApiHeaders()),
    }
    if "" != strings.TrimSpace(instance.token) {
        options = append(options, httpclient.WithBearerToken(instance.token))
    }
    response, requestErr := requestWithRetry(instance.httpClient, http.MethodGet, endpoint, options...)
    if nil != requestErr {
        return false, requestErr
    }
    instance.recordRateLimit(response)
    if http.StatusNotFound == response.StatusCode() {
        return false, nil
    }
    if 200 > response.StatusCode() || 300 <= response.StatusCode() {
        return false, fmt.Errorf("http %d: %s", response.StatusCode(), string(response.Body()))
    }

    var result struct {
        Attestations []json.RawMessage `json:"attestations"`
    }
    if decodeErr := json.Unmarshal(response.Body(), &result); nil != decodeErr {
        return false, fmt.Errorf("parse artifact attestations: %w", decodeErr)
    }
    return 0 < len(result.Attestations), nil
}
