package service

import "testing"

func TestHasArtifactAttestation(t *testing.T) {
    tests := []struct {
        name       string
        statusCode int
        body       string
        expected   bool
    }{
        {name: "verified", statusCode: 200, body: `{"attestations":[{}]}`, expected: true},
        {name: "empty", statusCode: 200, body: `{"attestations":[]}`, expected: false},
        {name: "not found", statusCode: 404, body: `{}`, expected: false},
    }

    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            client := &GithubClient{httpClient: &fakeHttpClient{response: &fakeResponse{
                statusCode: test.statusCode,
                body:       []byte(test.body),
            }}}
            actual, checkErr := client.HasArtifactAttestation("precision-soft", "git-audit", "sha256:abc")
            if nil != checkErr {
                t.Fatalf("unexpected error: %v", checkErr)
            }
            if test.expected != actual {
                t.Fatalf("expected %t, got %t", test.expected, actual)
            }
        })
    }
}

func TestHasArtifactAttestationWithoutDigest(t *testing.T) {
    client := &GithubClient{}
    actual, checkErr := client.HasArtifactAttestation("precision-soft", "git-audit", " ")
    if nil != checkErr || true == actual {
        t.Fatalf("expected false without error, got %t and %v", actual, checkErr)
    }
}
