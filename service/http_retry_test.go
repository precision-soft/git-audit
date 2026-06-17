package service

import "testing"

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
