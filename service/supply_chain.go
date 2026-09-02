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
    endpoint := fmt.Sprintf("%s/repos/%s/%s/git/ref/tags/%s", instance.apiBase, organization, repository, escapePathSegments(tag))
    if getErr := instance.get(endpoint, &reference); nil != getErr {
        return false, "", getErr
    }
    if "tag" != reference.Object.Type {
        return false, "lightweight tag", nil
    }
    var tagObject gitTagObject
    endpoint = fmt.Sprintf("%s/repos/%s/%s/git/tags/%s", instance.apiBase, organization, repository, reference.Object.SHA)
    if getErr := instance.get(endpoint, &tagObject); nil != getErr {
        return false, "", getErr
    }
    return tagObject.Verification.Verified, tagObject.Verification.Reason, nil
}

/* a tag may carry "/" (go submodule tags do), so each segment is escaped and the separators are kept */
func escapePathSegments(value string) string {
    segments := strings.Split(value, "/")
    for index, segment := range segments {
        segments[index] = url.PathEscape(segment)
    }
    return strings.Join(segments, "/")
}

func (instance *GithubClient) HasArtifactAttestation(organization, repository, digest string) (bool, error) {
    digest = strings.TrimSpace(digest)
    if "" == digest {
        return false, nil
    }

    endpoint := fmt.Sprintf(
        "%s/repos/%s/%s/attestations/%s",
        instance.apiBase,
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
        return false, newHttpStatusError(response)
    }

    var result struct {
        Attestations []json.RawMessage `json:"attestations"`
    }
    if decodeErr := json.Unmarshal(response.Body(), &result); nil != decodeErr {
        return false, fmt.Errorf("parse artifact attestations: %w", decodeErr)
    }
    return 0 < len(result.Attestations), nil
}
