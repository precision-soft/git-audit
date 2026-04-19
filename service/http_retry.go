package service

import (
    "bytes"
    "fmt"
    "io"
    "net/http"
    "time"
)

const (
    httpTimeout    = 30 * time.Second
    maxAttempts    = 3
    initialBackoff = 500 * time.Millisecond
)

func newHttpClient() *http.Client {
    return &http.Client{Timeout: httpTimeout}
}

/**
 * doWithRetry executes request with retries on 5xx/429 using exponential backoff.
 * The request body (if any) must be rewindable via http.Request.GetBody.
 */
func doWithRetry(httpClient *http.Client, request *http.Request) (*http.Response, error) {
    var lastResponse *http.Response
    var lastErr error

    backoff := initialBackoff
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        if attempt > 1 && nil != request.GetBody {
            replayBody, bodyErr := request.GetBody()
            if nil != bodyErr {
                return nil, fmt.Errorf("rewind body: %w", bodyErr)
            }
            request.Body = replayBody
        }

        response, requestErr := httpClient.Do(request)
        if nil == requestErr && false == shouldRetryStatus(response.StatusCode) {
            return response, nil
        }

        if nil != response {
            lastResponse = response
            if attempt < maxAttempts {
                _, _ = io.Copy(io.Discard, response.Body)
                _ = response.Body.Close()
            }
        }
        lastErr = requestErr

        if attempt == maxAttempts {
            break
        }
        time.Sleep(backoff)
        backoff *= 2
    }

    if nil != lastResponse {
        return lastResponse, nil
    }
    return nil, lastErr
}

func shouldRetryStatus(statusCode int) bool {
    if http.StatusTooManyRequests == statusCode {
        return true
    }
    return statusCode >= 500 && statusCode < 600
}

/**
 * bodyReader returns a reader + GetBody factory so requests with a body can be retried.
 */
func bodyReader(payload []byte) (io.Reader, func() (io.ReadCloser, error)) {
    if 0 == len(payload) {
        return nil, nil
    }
    reader := bytes.NewReader(payload)
    getBody := func() (io.ReadCloser, error) {
        return io.NopCloser(bytes.NewReader(payload)), nil
    }
    return reader, getBody
}
