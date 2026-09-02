package service

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/precision-soft/melody/v3/httpclient"
    httpclientcontract "github.com/precision-soft/melody/v3/httpclient/contract"
)

const githubApiBase = "https://api.github.com"
const githubRawBase = "https://raw.githubusercontent.com"
const perPage = 100

type RateLimitInfo struct {
    Limit     int
    Remaining int
    Used      int
    Reset     time.Time
    Resource  string
    HasData   bool
}

type GithubTag struct {
    Name      string `json:"name"`
    CommitSHA string `json:"commitSha"`
}

type GithubAsset struct {
    Name   string `json:"name"`
    Digest string `json:"digest"`
}

type GithubRelease struct {
    ID              int64         `json:"id"`
    TagName         string        `json:"tag_name"`
    Name            string        `json:"name"`
    Body            string        `json:"body"`
    Draft           bool          `json:"draft"`
    Prerelease      bool          `json:"prerelease"`
    TargetCommitish string        `json:"target_commitish"`
    Assets          []GithubAsset `json:"assets"`
}

type CompareFile struct {
    Filename string `json:"filename"`
    Status   string `json:"status"`
}

type CompareCommit struct {
    SHA string `json:"sha"`
}

type CompareResponse struct {
    Status       string          `json:"status"`
    AheadBy      int             `json:"ahead_by"`
    BehindBy     int             `json:"behind_by"`
    TotalCommits int             `json:"total_commits"`
    Commits      []CompareCommit `json:"commits"`
    Files        []CompareFile   `json:"files"`
}

func newHttpStatusError(response httpclientcontract.Response) *HttpStatusError {
    return &HttpStatusError{
        StatusCode: response.StatusCode(),
        Body:       string(response.Body()),
    }
}

type HttpStatusError struct {
    StatusCode int
    Body       string
}

func (instance *HttpStatusError) Error() string {
    return fmt.Sprintf("http %d: %s", instance.StatusCode, instance.Body)
}

type GithubClient struct {
    token        string
    httpClient   httpclientcontract.Client
    rateLimit    RateLimitInfo
    rateLimitMux sync.Mutex
    cacheDir     string
    apiBase      string
    rawBase      string
}

func githubApiHeaders() map[string]string {
    return map[string]string{
        "Accept":               "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }
}

func NewGithubClient(token string) *GithubClient {
    return &GithubClient{
        token:      token,
        httpClient: newHttpClient(),
        apiBase:    githubApiBase,
        rawBase:    githubRawBase,
    }
}

func (instance *GithubClient) SetCacheDir(directory string) {
    instance.cacheDir = strings.TrimSpace(directory)
}

/* an empty base keeps the public host, so a blank parameter can never redirect a request to "/repos/..." */
func (instance *GithubClient) SetEndpoints(apiBase, rawBase string) {
    instance.apiBase = strings.TrimRight(strings.TrimSpace(apiBase), "/")
    if "" == instance.apiBase {
        instance.apiBase = githubApiBase
    }
    instance.rawBase = strings.TrimRight(strings.TrimSpace(rawBase), "/")
    if "" == instance.rawBase {
        instance.rawBase = githubRawBase
    }
}

func (instance *GithubClient) RateLimit() RateLimitInfo {
    instance.rateLimitMux.Lock()
    defer instance.rateLimitMux.Unlock()
    return instance.rateLimit
}

/*
recordRateLimit keeps the minimum Remaining seen across all responses, which is peak usage: a
last-wins value overstates the headroom left.
*/
func (instance *GithubClient) recordRateLimit(response httpclientcontract.Response) {
    headers := response.Headers()
    limit := headers.Get("X-RateLimit-Limit")
    remaining := headers.Get("X-RateLimit-Remaining")
    if "" == limit && "" == remaining {
        return
    }

    info := RateLimitInfo{HasData: true}
    if parsed, parseErr := strconv.Atoi(limit); nil == parseErr {
        info.Limit = parsed
    }
    if parsed, parseErr := strconv.Atoi(remaining); nil == parseErr {
        info.Remaining = parsed
    }
    if parsed, parseErr := strconv.Atoi(headers.Get("X-RateLimit-Used")); nil == parseErr {
        info.Used = parsed
    }
    if parsed, parseErr := strconv.ParseInt(headers.Get("X-RateLimit-Reset"), 10, 64); nil == parseErr {
        info.Reset = time.Unix(parsed, 0)
    }
    info.Resource = headers.Get("X-RateLimit-Resource")

    instance.rateLimitMux.Lock()
    defer instance.rateLimitMux.Unlock()

    if false == instance.rateLimit.HasData || info.Remaining < instance.rateLimit.Remaining {
        instance.rateLimit = info
    }
}

type githubTagApiResponse struct {
    Name   string `json:"name"`
    Commit struct {
        SHA string `json:"sha"`
    } `json:"commit"`
}

func (instance *GithubClient) GetTags(organization, repository string) ([]GithubTag, error) {
    var all []GithubTag

    for page := 1; ; page++ {
        urlString := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=%d&page=%d", instance.apiBase, organization, repository, perPage, page)

        var batch []githubTagApiResponse
        if getErr := instance.get(urlString, &batch); nil != getErr {
            return nil, fmt.Errorf("page %d: %w", page, getErr)
        }

        for _, responseItem := range batch {
            all = append(all, GithubTag{
                Name:      responseItem.Name,
                CommitSHA: responseItem.Commit.SHA,
            })
        }

        if perPage > len(batch) {
            break
        }
    }

    return all, nil
}

func (instance *GithubClient) GetReleases(organization, repository string) ([]GithubRelease, error) {
    var all []GithubRelease

    for page := 1; ; page++ {
        urlString := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d&page=%d", instance.apiBase, organization, repository, perPage, page)

        var batch []GithubRelease
        if getErr := instance.get(urlString, &batch); nil != getErr {
            return nil, fmt.Errorf("page %d: %w", page, getErr)
        }

        all = append(all, batch...)

        if perPage > len(batch) {
            break
        }
    }

    return all, nil
}

func (instance *GithubClient) GetFileContentAtRef(organization, repository, path, ref string) (string, error) {
    endpoint := fmt.Sprintf(
        "%s/%s/%s/%s/%s",
        instance.rawBase,
        organization,
        repository,
        ref,
        path,
    )

    var options []httpclientcontract.RequestOption
    if "" != strings.TrimSpace(instance.token) {
        options = append(options, httpclient.WithBearerToken(instance.token))
    }

    response, requestErr := requestWithRetry(instance.httpClient, http.MethodGet, endpoint, options...)
    if nil != requestErr {
        return "", requestErr
    }

    if http.StatusOK != response.StatusCode() {
        return "", newHttpStatusError(response)
    }

    return string(response.Body()), nil
}

func (instance *GithubClient) CompareTags(organization, repository, base, head string) (*CompareResponse, error) {
    endpoint := fmt.Sprintf(
        "%s/repos/%s/%s/compare/%s...%s",
        instance.apiBase,
        organization,
        repository,
        base,
        head,
    )

    options := []httpclientcontract.RequestOption{
        httpclient.WithHeaders(githubApiHeaders()),
    }
    if "" != strings.TrimSpace(instance.token) {
        options = append(options, httpclient.WithBearerToken(instance.token))
    }

    response, requestErr := requestWithRetry(instance.httpClient, http.MethodGet, endpoint, options...)
    if nil != requestErr {
        return nil, requestErr
    }

    instance.recordRateLimit(response)

    if http.StatusOK != response.StatusCode() {
        return nil, newHttpStatusError(response)
    }

    var compareResponse CompareResponse
    if decodeErr := json.Unmarshal(response.Body(), &compareResponse); nil != decodeErr {
        return nil, fmt.Errorf("parse compare response for %s/%s %s...%s: %w", organization, repository, base, head, decodeErr)
    }

    return &compareResponse, nil
}

func (instance *GithubClient) UpdateRelease(organization, repository string, releaseId int64, body, name string) error {
    if "" == strings.TrimSpace(instance.token) {
        return fmt.Errorf("github token required to update release")
    }

    endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/%d", instance.apiBase, organization, repository, releaseId)

    payload := struct {
        Body string `json:"body"`
        Name string `json:"name,omitempty"`
    }{Body: body, Name: name}

    options := []httpclientcontract.RequestOption{
        httpclient.WithJson(payload),
        httpclient.WithHeaders(map[string]string{
            "Accept":               "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        }),
        httpclient.WithBearerToken(instance.token),
    }

    response, requestErr := requestWithRetry(instance.httpClient, http.MethodPatch, endpoint, options...)
    if nil != requestErr {
        return requestErr
    }

    instance.recordRateLimit(response)

    if 200 > response.StatusCode() || 300 <= response.StatusCode() {
        return newHttpStatusError(response)
    }

    return nil
}

func (instance *GithubClient) get(urlString string, destination any) error {
    headers := githubApiHeaders()
    cached := readCachedResponse(instance.cacheDir, urlString)
    if nil != cached && "" != cached.EntityTag {
        headers["If-None-Match"] = cached.EntityTag
    }
    options := []httpclientcontract.RequestOption{
        httpclient.WithHeaders(headers),
    }
    if "" != instance.token {
        options = append(options, httpclient.WithBearerToken(instance.token))
    }

    response, requestErr := requestWithRetry(instance.httpClient, http.MethodGet, urlString, options...)
    if nil != requestErr {
        return requestErr
    }

    instance.recordRateLimit(response)

    if http.StatusNotModified == response.StatusCode() {
        if nil == cached {
            return fmt.Errorf("http 304 without a cached response")
        }
        return json.Unmarshal(cached.Body, destination)
    }

    if 200 > response.StatusCode() || 300 <= response.StatusCode() {
        return newHttpStatusError(response)
    }

    /* a cache that cannot be written is no cache; the response in hand is still the answer */
    writeCachedResponse(instance.cacheDir, urlString, response.Headers().Get("ETag"), response.Body())

    return json.Unmarshal(response.Body(), destination)
}
