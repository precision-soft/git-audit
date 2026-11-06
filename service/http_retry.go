package service

import (
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/precision-soft/melody/v3/httpclient"
    httpclientcontract "github.com/precision-soft/melody/v3/httpclient/contract"
)

const (
    httpTimeout    = 30 * time.Second
    maxAttempts    = 3
    initialBackoff = 500 * time.Millisecond
    maxRetryAfter  = 30 * time.Second
    /* melody's default of 10 MiB is exceeded by a large compare diff */
    maxResponseBodyBytes = 256 * 1024 * 1024
)

func newHttpClient() httpclientcontract.Client {
    return httpclient.NewHttpClient(
        httpclient.NewHttpClientConfig("", httpTimeout, nil),
    )
}

func requestWithRetry(
    client httpclientcontract.Client,
    method string,
    url string,
    options ...httpclientcontract.RequestOption,
) (httpclientcontract.Response, error) {
    var lastResponse httpclientcontract.Response
    var lastErr error

    /* prepended, not appended, so a caller's own WithMaxResponseBodyBytes still wins */
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

        delay := backoff
        if nil != response {
            if retryAfter := retryAfterDelay(response.Headers().Get("Retry-After")); 0 < retryAfter {
                delay = retryAfter
            }
        }
        time.Sleep(delay)
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
    return 500 <= statusCode && 600 > statusCode
}

func retryAfterDelay(header string) time.Duration {
    seconds, parseErr := strconv.Atoi(strings.TrimSpace(header))
    if nil != parseErr || 0 >= seconds {
        return 0
    }

    delay := time.Duration(seconds) * time.Second
    if maxRetryAfter < delay {
        return maxRetryAfter
    }

    return delay
}
