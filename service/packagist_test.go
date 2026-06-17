package service

import (
    "strings"
    "testing"
)

func TestGetPackagistPackageVersionsParsesAndFallsBackToDist(t *testing.T) {
    body := `{"packages":{"acme/widget":[
        {"version":"v1.0.0","source":{"reference":"abc123"}},
        {"version":"v1.1.0","source":{"reference":""},"dist":{"reference":"def456"}},
        {"version":"   ","source":{"reference":"ignored"}}
    ]}}`

    original := packagistClient
    packagistClient = &fakeHttpClient{response: &fakeResponse{statusCode: 200, body: []byte(body)}}
    defer func() { packagistClient = original }()

    versions, err := GetPackagistPackageVersions("acme/widget")
    if nil != err {
        t.Fatalf("unexpected error: %v", err)
    }
    if 2 != len(versions) {
        t.Fatalf("expected 2 versions (the blank-version entry is skipped), got %d", len(versions))
    }
    if "v1.0.0" != versions[0].Version || "abc123" != versions[0].Reference {
        t.Errorf("unexpected first version: %+v", versions[0])
    }
    if "v1.1.0" != versions[1].Version || "def456" != versions[1].Reference {
        t.Errorf("expected fallback to dist reference, got %+v", versions[1])
    }
}

func TestGetPackagistPackageVersionsErrorsWhenPackageMissing(t *testing.T) {
    original := packagistClient
    packagistClient = &fakeHttpClient{response: &fakeResponse{statusCode: 200, body: []byte(`{"packages":{}}`)}}
    defer func() { packagistClient = original }()

    if _, err := GetPackagistPackageVersions("acme/widget"); nil == err {
        t.Fatal("expected error for package missing from payload")
    } else if false == strings.Contains(err.Error(), "not found") {
        t.Errorf("error %q does not mention the package was not found", err.Error())
    }
}

func TestGetPackagistPackageVersionsErrorsOnNon2xx(t *testing.T) {
    original := packagistClient
    packagistClient = &fakeHttpClient{response: &fakeResponse{statusCode: 404, body: []byte("nope")}}
    defer func() { packagistClient = original }()

    if _, err := GetPackagistPackageVersions("acme/widget"); nil == err {
        t.Fatal("expected error for non-2xx response")
    } else if false == strings.Contains(err.Error(), "404") {
        t.Errorf("error %q does not mention the status code", err.Error())
    }
}
