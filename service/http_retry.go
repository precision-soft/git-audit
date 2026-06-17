package service

import (
    "net/http"
    "time"

    "github.com/precision-soft/melody/v3/httpclient"
    httpclientcontract "github.com/precision-soft/melody/v3/httpclient/contract"
)

const (
    httpTimeout    = 30 * time.Second
    maxAttempts    = 3
    initialBackoff = 500 * time.Millisecond
    /*
     * maxResponseBodyBytes is an effectively-unbounded ceiling that preserves the
     * prior unbounded io.ReadAll behavior while still guarding against OOM. melody's
     * httpclient defaults to 10 MiB, which a large `compare` diff can exceed.
     */
    maxResponseBodyBytes = 256 * 1024 * 1024
)

func newHttpClient() httpclientcontract.Client {
    return httpclient.NewHttpClient(
        httpclient.NewHttpClientConfig("", httpTimeout, nil),
    )
}

/**
 * requestWithRetry executes a request through melody's httpclient with retries on
 * 5xx/429 using exponential backoff. melody re-derives the request body from the
 * options on every attempt, so no manual body rewinding is needed.
 */
func requestWithRetry(
    client httpclientcontract.Client,
    method string,
    url string,
    options ...httpclientcontract.RequestOption,
) (httpclientcontract.Response, error) {
    var lastResponse httpclientcontract.Response
    var lastErr error

    /*
     * Default the body cap to effectively-unbounded; a caller may still override it
     * by passing its own WithMaxResponseBodyBytes after this.
     */
    options = append(
        []httpclientcontract.RequestOption{httpclient.WithMaxResponseBodyBytes(maxResponseBodyBytes)},
        options...,
    )

    backoff := initialBackoff
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        response, requestErr := client.Request(method, url, options...)
        if nil == requestErr && false == shouldRetryStatus(response.StatusCode()) {
            return response, nil
        }

        if nil != response {
            lastResponse = response
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
