package service

import (
    "testing"
    "time"
)

func TestShouldRetryStatus(t *testing.T) {
    cases := []struct {
        statusCode int
        want       bool
    }{
        {200, false},
        {301, false},
        {400, false},
        {404, false},
        {429, true},
        {499, false},
        {500, true},
        {503, true},
        {599, true},
        {600, false},
    }

    for _, testCase := range cases {
        if got := shouldRetryStatus(testCase.statusCode); testCase.want != got {
            t.Errorf("shouldRetryStatus(%d) = %v, want %v", testCase.statusCode, got, testCase.want)
        }
    }
}

func TestRetryAfterDelayHonoursTheHeaderUpToTheCeiling(t *testing.T) {
    cases := map[string]time.Duration{
        "":                              0,
        "soon":                          0,
        "0":                             0,
        "2":                             2 * time.Second,
        "3600":                          maxRetryAfter,
        "Wed, 21 Oct 2026 07:28:00 GMT": 0,
    }
    for header, want := range cases {
        if got := retryAfterDelay(header); want != got {
            t.Errorf("%q: expected %s, got %s", header, want, got)
        }
    }
}
