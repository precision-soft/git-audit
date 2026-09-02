package service

import (
    "bytes"
    "os"
    "testing"
)

func TestHttpCacheRoundTripUsesAtomicPrivateFile(t *testing.T) {
    directory := t.TempDir()
    body := []byte(`[{"name":"v1.0.0"}]`)
    if writeErr := writeCachedResponse(directory, "https://example.test/releases", `"etag-1"`, body); nil != writeErr {
        t.Fatal(writeErr)
    }
    cached, readErr := readCachedResponse(directory, "https://example.test/releases")
    if nil != readErr || nil == cached {
        t.Fatalf("read cache: %v %#v", readErr, cached)
    }
    if `"etag-1"` != cached.EntityTag || false == bytes.Equal(body, cached.Body) {
        t.Fatalf("unexpected cache: %#v", cached)
    }
    info, statErr := os.Stat(cachePath(directory, "https://example.test/releases"))
    if nil != statErr || 0o600 != info.Mode().Perm() {
        t.Fatalf("expected mode 0600: %v %v", info.Mode().Perm(), statErr)
    }
}
