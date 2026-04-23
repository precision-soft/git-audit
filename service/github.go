package service

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strconv"
    "strings"
    "sync"
    "time"
)

const githubApiBase = "https://api.github.com"
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

type GithubRelease struct {
    ID              int64  `json:"id"`
    TagName         string `json:"tag_name"`
    Name            string `json:"name"`
    Body            string `json:"body"`
    Draft           bool   `json:"draft"`
    Prerelease      bool   `json:"prerelease"`
    TargetCommitish string `json:"target_commitish"`
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

type GithubClient struct {
    token        string
    httpClient   *http.Client
    rateLimit    RateLimitInfo
    rateLimitMux sync.Mutex
}

func NewGithubClient(token string) *GithubClient {
    return &GithubClient{
        token:      token,
        httpClient: newHttpClient(),
    }
}

func (client *GithubClient) RateLimit() RateLimitInfo {
    client.rateLimitMux.Lock()
    defer client.rateLimitMux.Unlock()
    return client.rateLimit
}

/**
 * recordRateLimit tracks the minimum Remaining seen across all responses
 * (= peak usage). Prior last-wins behavior overstated headroom.
 */
func (client *GithubClient) recordRateLimit(response *http.Response) {
    limit := response.Header.Get("X-RateLimit-Limit")
    remaining := response.Header.Get("X-RateLimit-Remaining")
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
    if parsed, parseErr := strconv.Atoi(response.Header.Get("X-RateLimit-Used")); nil == parseErr {
        info.Used = parsed
    }
    if parsed, parseErr := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); nil == parseErr {
        info.Reset = time.Unix(parsed, 0)
    }
    info.Resource = response.Header.Get("X-RateLimit-Resource")

    client.rateLimitMux.Lock()
    defer client.rateLimitMux.Unlock()

    if false == client.rateLimit.HasData || info.Remaining < client.rateLimit.Remaining {
        client.rateLimit = info
    }
}

type githubTagApiResponse struct {
    Name   string `json:"name"`
    Commit struct {
        SHA string `json:"sha"`
    } `json:"commit"`
}

func (client *GithubClient) GetTags(organization, repository string) ([]GithubTag, error) {
    var all []GithubTag

    for page := 1; ; page++ {
        url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=%d&page=%d", githubApiBase, organization, repository, perPage, page)

        var batch []githubTagApiResponse
        if getErr := client.get(url, &batch); nil != getErr {
            return nil, fmt.Errorf("page %d: %w", page, getErr)
        }

        for _, responseItem := range batch {
            all = append(all, GithubTag{
                Name:      responseItem.Name,
                CommitSHA: responseItem.Commit.SHA,
            })
        }

        if len(batch) < perPage {
            break
        }
    }

    return all, nil
}

func (client *GithubClient) GetReleases(organization, repository string) ([]GithubRelease, error) {
    var all []GithubRelease

    for page := 1; ; page++ {
        url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d&page=%d", githubApiBase, organization, repository, perPage, page)

        var batch []GithubRelease
        if getErr := client.get(url, &batch); nil != getErr {
            return nil, fmt.Errorf("page %d: %w", page, getErr)
        }

        all = append(all, batch...)

        if len(batch) < perPage {
            break
        }
    }

    return all, nil
}

func (client *GithubClient) GetFileContentAtRef(organization, repository, path, ref string) (string, error) {
    endpoint := fmt.Sprintf(
        "https://raw.githubusercontent.com/%s/%s/%s/%s",
        organization,
        repository,
        ref,
        path,
    )

    request, requestErr := http.NewRequest(http.MethodGet, endpoint, nil)
    if nil != requestErr {
        return "", requestErr
    }

    if "" != strings.TrimSpace(client.token) {
        request.Header.Set("Authorization", "Bearer "+client.token)
    }

    response, doErr := doWithRetry(client.httpClient, request)
    if nil != doErr {
        return "", doErr
    }
    defer response.Body.Close()

    if http.StatusOK != response.StatusCode {
        responseBody, _ := io.ReadAll(response.Body)
        return "", fmt.Errorf("http %d: %s", response.StatusCode, string(responseBody))
    }

    bodyBytes, readErr := io.ReadAll(response.Body)
    if nil != readErr {
        return "", readErr
    }

    return string(bodyBytes), nil
}

func (client *GithubClient) CompareTags(organization, repository, base, head string) (*CompareResponse, error) {
    endpoint := fmt.Sprintf(
        "%s/repos/%s/%s/compare/%s...%s",
        githubApiBase,
        organization,
        repository,
        base,
        head,
    )

    request, requestErr := http.NewRequest(http.MethodGet, endpoint, nil)
    if nil != requestErr {
        return nil, requestErr
    }

    request.Header.Set("Accept", "application/vnd.github+json")
    request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
    request.Header.Set("Cache-Control", "no-cache, no-store")
    request.Header.Set("Pragma", "no-cache")

    if "" != strings.TrimSpace(client.token) {
        request.Header.Set("Authorization", "Bearer "+client.token)
    }

    response, doErr := doWithRetry(client.httpClient, request)
    if nil != doErr {
        return nil, doErr
    }
    defer response.Body.Close()

    client.recordRateLimit(response)

    if http.StatusOK != response.StatusCode {
        responseBody, _ := io.ReadAll(response.Body)
        return nil, fmt.Errorf("http %d: %s", response.StatusCode, string(responseBody))
    }

    var compareResponse CompareResponse
    if decodeErr := json.NewDecoder(response.Body).Decode(&compareResponse); nil != decodeErr {
        return nil, decodeErr
    }

    return &compareResponse, nil
}

func (client *GithubClient) UpdateRelease(organization, repository string, releaseId int64, body, name string) error {
    if "" == strings.TrimSpace(client.token) {
        return fmt.Errorf("github token required to update release")
    }

    endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/%d", githubApiBase, organization, repository, releaseId)

    payload, marshalErr := json.Marshal(struct {
        Body string `json:"body"`
        Name string `json:"name,omitempty"`
    }{Body: body, Name: name})
    if nil != marshalErr {
        return marshalErr
    }

    reader, getBody := bodyReader(payload)
    request, requestErr := http.NewRequest(http.MethodPatch, endpoint, reader)
    if nil != requestErr {
        return requestErr
    }
    request.GetBody = getBody

    request.Header.Set("Accept", "application/vnd.github+json")
    request.Header.Set("Content-Type", "application/json")
    request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
    request.Header.Set("Authorization", "Bearer "+client.token)

    response, doErr := doWithRetry(client.httpClient, request)
    if nil != doErr {
        return doErr
    }
    defer response.Body.Close()

    client.recordRateLimit(response)

    if response.StatusCode < 200 || response.StatusCode >= 300 {
        responseBody, _ := io.ReadAll(response.Body)
        return fmt.Errorf("http %d: %s", response.StatusCode, string(responseBody))
    }

    return nil
}

func (client *GithubClient) get(url string, destination any) error {
    request, requestErr := http.NewRequest(http.MethodGet, url, nil)
    if nil != requestErr {
        return requestErr
    }

    request.Header.Set("Accept", "application/vnd.github+json")
    request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
    request.Header.Set("Cache-Control", "no-cache, no-store")
    request.Header.Set("Pragma", "no-cache")

    if "" != client.token {
        request.Header.Set("Authorization", "Bearer "+client.token)
    }

    response, doErr := doWithRetry(client.httpClient, request)
    if nil != doErr {
        return doErr
    }
    defer response.Body.Close()

    client.recordRateLimit(response)

    body, readErr := io.ReadAll(response.Body)
    if nil != readErr {
        return readErr
    }

    if response.StatusCode < 200 || response.StatusCode >= 300 {
        return fmt.Errorf("http %d: %s", response.StatusCode, string(body))
    }

    return json.Unmarshal(body, destination)
}
