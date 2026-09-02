package cli

import "testing"

func TestParseSupplyChainChecksAll(t *testing.T) {
    checks, parseErr := parseSupplyChainChecks("all")
    if nil != parseErr {
        t.Fatal(parseErr)
    }
    for _, name := range []string{"signed-tags", "checksums", "sbom", "attestations"} {
        if false == checks[name] {
            t.Errorf("expected %s to be enabled", name)
        }
    }
}

func TestParseSupplyChainChecksRejectsUnknown(t *testing.T) {
    if _, parseErr := parseSupplyChainChecks("unknown"); nil == parseErr {
        t.Fatal("expected an error")
    }
}

func TestContainsAssetMarkerIsCaseNormalizedByCaller(t *testing.T) {
    if false == containsAssetMarker([]string{"project.spdx.json"}, []string{"spdx"}) {
        t.Fatal("expected SPDX asset to match")
    }
}
