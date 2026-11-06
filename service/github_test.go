package service

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestSetEndpointsKeepsThePublicHostsForBlankValues(t *testing.T) {
    client := NewGithubClient("token")
    client.SetEndpoints(" ", "")

    if githubApiBase != client.apiBase || githubRawBase != client.rawBase {
        t.Fatalf("blank endpoints must keep the public hosts, got %q and %q", client.apiBase, client.rawBase)
    }

    client.SetEndpoints("http://127.0.0.1:1/api/", "http://127.0.0.1:1/raw/")
    if "http://127.0.0.1:1/api" != client.apiBase || "http://127.0.0.1:1/raw" != client.rawBase {
        t.Fatalf("endpoints must be stored without a trailing slash, got %q and %q", client.apiBase, client.rawBase)
    }
}

func TestNewGithubReleaseServiceHandsTheEndpointsToEveryClient(t *testing.T) {
    releaseService := NewGithubReleaseService("token", "http://api.test", "http://raw.test")

    for name, client := range map[string]*GithubClient{
        "service client": releaseService.Client(),
        "token client":   releaseService.NewClientWithToken("other"),
    } {
        if "http://api.test" != client.apiBase || "http://raw.test" != client.rawBase {
            t.Errorf("%s must carry the configured endpoints, got %q and %q", name, client.apiBase, client.rawBase)
        }
    }
}

func TestGetTagsRequestsTheConfiguredApiBase(t *testing.T) {
    var requestedPath string
    server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        requestedPath = request.URL.Path
        writer.Header().Set("Content-Type", "application/json")
        writer.Write([]byte(`[{"name":"v1.0.0","commit":{"sha":"abc"}}]`))
    }))
    defer server.Close()

    client := NewGithubClient("")
    client.SetEndpoints(server.URL, server.URL)

    tags, getErr := client.GetTags("acme", "widget")
    if nil != getErr {
        t.Fatal(getErr)
    }
    if "/repos/acme/widget/tags" != requestedPath {
        t.Fatalf("expected the tags path on the configured base, got %q", requestedPath)
    }
    if 1 != len(tags) || "v1.0.0" != tags[0].Name || "abc" != tags[0].CommitSha {
        t.Fatalf("unexpected tags: %#v", tags)
    }
}

func TestGetFileContentAtRefRequestsTheConfiguredRawBase(t *testing.T) {
    var requestedPath string
    server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        requestedPath = request.URL.Path
        writer.Write([]byte("# Changelog\n"))
    }))
    defer server.Close()

    client := NewGithubClient("")
    client.SetEndpoints(server.URL, server.URL+"/raw")

    content, getErr := client.GetFileContentAtRef("acme", "widget", "CHANGELOG.md", "v1.0.0")
    if nil != getErr {
        t.Fatal(getErr)
    }
    if "/raw/acme/widget/v1.0.0/CHANGELOG.md" != requestedPath {
        t.Fatalf("expected the raw path on the configured base, got %q", requestedPath)
    }
    if "# Changelog\n" != content {
        t.Fatalf("unexpected content: %q", content)
    }
}

func TestGetReturnsTheStatusOfAFailedRequest(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        writer.WriteHeader(http.StatusNotFound)
        writer.Write([]byte(`{"message":"Not Found"}`))
    }))
    defer server.Close()

    client := NewGithubClient("")
    client.SetEndpoints(server.URL, server.URL)

    var destination json.RawMessage
    getErr := client.get(server.URL+"/missing", &destination)
    if nil == getErr || "http 404: {\"message\":\"Not Found\"}" != getErr.Error() {
        t.Fatalf("expected the http status in the error, got %v", getErr)
    }
}

func TestCompareTagsEscapesTagsAndReplaysTheCache(t *testing.T) {
    var requested []string
    var revalidated []string
    server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        requested = append(requested, request.URL.EscapedPath())
        if `"compare"` == request.Header.Get("If-None-Match") {
            revalidated = append(revalidated, request.URL.EscapedPath())
            writer.WriteHeader(http.StatusNotModified)
            return
        }
        writer.Header().Set("ETag", `"compare"`)
        writer.Write([]byte(`{"status":"ahead","total_commits":2}`))
    }))
    defer server.Close()

    client := NewGithubClient("token")
    client.SetEndpoints(server.URL, server.URL)
    client.SetCacheDir(t.TempDir())

    for attempt := 1; 2 >= attempt; attempt++ {
        response, compareErr := client.CompareTags("acme", "widget", "v1.0.0", "v1.0.0#1")
        if nil != compareErr {
            t.Fatal(compareErr)
        }
        if 2 != response.TotalCommits {
            t.Fatalf("attempt %d: expected 2 commits, got %d", attempt, response.TotalCommits)
        }
    }

    if "/repos/acme/widget/compare/v1.0.0...v1.0.0%231" != requested[0] {
        t.Errorf("the tag must be escaped, got %s", requested[0])
    }
    if 1 != len(revalidated) {
        t.Errorf("the second compare must revalidate the cached response, got %d", len(revalidated))
    }
}

func TestGetFileContentAtRefEscapesThePathAndKeepsItsSeparators(t *testing.T) {
    var requested string
    server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        requested = request.URL.EscapedPath()
        writer.Write([]byte("content"))
    }))
    defer server.Close()

    client := NewGithubClient("")
    client.SetEndpoints(server.URL, server.URL)

    if _, fetchErr := client.GetFileContentAtRef("acme", "widget", "docs/change#log.md", "HEAD"); nil != fetchErr {
        t.Fatal(fetchErr)
    }
    if "/acme/widget/HEAD/docs/change%23log.md" != requested {
        t.Errorf("unexpected path %s", requested)
    }
}
