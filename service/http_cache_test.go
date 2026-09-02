package service

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"
)

func TestHttpCacheRoundTripUsesAtomicPrivateFile(t *testing.T) {
    directory := t.TempDir()
    body := []byte(`[{"name":"v1.0.0"}]`)
    if writeErr := writeCachedResponse(directory, "https://example.test/releases", `"etag-1"`, body); nil != writeErr {
        t.Fatal(writeErr)
    }
    cached := readCachedResponse(directory, "https://example.test/releases")
    if nil == cached {
        t.Fatal("expected a cache hit")
    }
    if `"etag-1"` != cached.EntityTag || false == bytes.Equal(body, cached.Body) {
        t.Fatalf("unexpected cache: %#v", cached)
    }
    info, statErr := os.Stat(cachePath(directory, "https://example.test/releases"))
    if nil != statErr || 0o600 != info.Mode().Perm() {
        t.Fatalf("expected mode 0600: %v %v", info.Mode().Perm(), statErr)
    }
}

func TestACorruptCacheEntryIsAMiss(t *testing.T) {
    directory := t.TempDir()
    if writeErr := os.WriteFile(cachePath(directory, "https://example.test/releases"), []byte("{truncated"), 0o600); nil != writeErr {
        t.Fatal(writeErr)
    }

    if cached := readCachedResponse(directory, "https://example.test/releases"); nil != cached {
        t.Fatalf("a cache entry that cannot be decoded is a miss, got %#v", cached)
    }
}

func TestAnUnwritableCacheDirectoryDoesNotFailTheRequest(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        writer.Header().Set("ETag", `"etag-1"`)
        writer.Write([]byte(`[{"name":"v1.0.0"}]`))
    }))
    defer server.Close()

    /* a regular file where the directory should be defeats MkdirAll for root as well as for anyone else */
    blocker := filepath.Join(t.TempDir(), "cache")
    if writeErr := os.WriteFile(blocker, []byte("not a directory"), 0o600); nil != writeErr {
        t.Fatal(writeErr)
    }

    client := NewGithubClient("")
    client.SetEndpoints(server.URL, server.URL)
    client.SetCacheDir(blocker)

    var tags []GithubTag
    if getErr := client.get(server.URL+"/repos/acme/widget/tags", &tags); nil != getErr {
        t.Fatalf("a cache that cannot be written must degrade to no cache, got %v", getErr)
    }
    if 1 != len(tags) || "v1.0.0" != tags[0].Name {
        t.Fatalf("unexpected tags: %#v", tags)
    }
}

func TestCachedResponseIsReplayedOnNotModified(t *testing.T) {
    var receivedEntityTag string
    server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        receivedEntityTag = request.Header.Get("If-None-Match")
        if `"etag-1"` == receivedEntityTag {
            writer.WriteHeader(http.StatusNotModified)
            return
        }
        writer.Header().Set("ETag", `"etag-1"`)
        writer.Write([]byte(`[{"name":"v1.0.0"}]`))
    }))
    defer server.Close()

    client := NewGithubClient("")
    client.SetEndpoints(server.URL, server.URL)
    client.SetCacheDir(t.TempDir())

    for run := 0; 2 > run; run++ {
        var tags []GithubTag
        if getErr := client.get(server.URL+"/repos/acme/widget/tags", &tags); nil != getErr {
            t.Fatalf("run %d: %v", run, getErr)
        }
        if 1 != len(tags) || "v1.0.0" != tags[0].Name {
            t.Fatalf("run %d: unexpected tags: %#v", run, tags)
        }
    }
    if `"etag-1"` != receivedEntityTag {
        t.Fatalf("the second request must carry the cached entity tag, got %q", receivedEntityTag)
    }
}

/*
the key deliberately ignores the token: a cached entity tag is only ever offered as If-None-Match,
and GitHub answers a caller that may not see the resource with 404, never with 304, so a body cached
under one token is never replayed to a caller the server would refuse
*/
func TestCacheKeyIgnoresTheToken(t *testing.T) {
    var receivedEntityTag string
    server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        receivedEntityTag = request.Header.Get("If-None-Match")
        writer.Header().Set("ETag", `"etag-1"`)
        writer.Write([]byte(`[]`))
    }))
    defer server.Close()

    directory := t.TempDir()
    for _, token := range []string{"token-a", "token-b"} {
        client := NewGithubClient(token)
        client.SetEndpoints(server.URL, server.URL)
        client.SetCacheDir(directory)
        var tags []GithubTag
        if getErr := client.get(server.URL+"/repos/acme/widget/tags", &tags); nil != getErr {
            t.Fatal(getErr)
        }
    }
    if `"etag-1"` != receivedEntityTag {
        t.Fatalf("the entry written under the first token must be offered under the second, got %q", receivedEntityTag)
    }
}
