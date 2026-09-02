package service

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

type cachedResponse struct {
    EntityTag string          `json:"etag"`
    Body      json.RawMessage `json:"body"`
}

/* every failure is a miss: the cache only ever saves a request, it never gets to fail one */
func readCachedResponse(directory, urlString string) *cachedResponse {
    if "" == directory {
        return nil
    }
    cachedData, readErr := os.ReadFile(cachePath(directory, urlString))
    if nil != readErr {
        return nil
    }
    var cached cachedResponse
    if decodeErr := json.Unmarshal(cachedData, &cached); nil != decodeErr {
        return nil
    }
    return &cached
}

func writeCachedResponse(directory, urlString, entityTag string, body []byte) error {
    if "" == directory || "" == entityTag {
        return nil
    }
    if mkdirErr := os.MkdirAll(directory, 0o700); nil != mkdirErr {
        return mkdirErr
    }
    payload, marshalErr := json.Marshal(cachedResponse{EntityTag: entityTag, Body: append(json.RawMessage(nil), body...)})
    if nil != marshalErr {
        return marshalErr
    }
    temporary, createErr := os.CreateTemp(directory, ".etag-*")
    if nil != createErr {
        return createErr
    }
    temporaryPath := temporary.Name()
    defer os.Remove(temporaryPath)
    if chmodErr := temporary.Chmod(0o600); nil != chmodErr {
        temporary.Close()
        return chmodErr
    }
    if _, writeErr := temporary.Write(payload); nil != writeErr {
        temporary.Close()
        return writeErr
    }
    if syncErr := temporary.Sync(); nil != syncErr {
        temporary.Close()
        return syncErr
    }
    if closeErr := temporary.Close(); nil != closeErr {
        return closeErr
    }
    return os.Rename(temporaryPath, cachePath(directory, urlString))
}

func cachePath(directory, urlString string) string {
    digest := sha256.Sum256([]byte(urlString))
    return filepath.Join(directory, fmt.Sprintf("%s.json", hex.EncodeToString(digest[:])))
}
