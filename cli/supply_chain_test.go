package cli

import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "reflect"
    "strings"
    "sync"
    "testing"

    "github.com/precision-soft/git-audit/config/project"
    "github.com/precision-soft/git-audit/service"
    "github.com/precision-soft/git-audit/types"
)

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

func TestSupplyChainFailurePropagatesToTheReleaseStatus(t *testing.T) {
    for expected, level := range map[types.Status]types.LevelStatus{
        types.StatusFailed:  types.LevelFailed,
        types.StatusWarning: types.LevelWarning,
    } {
        release := types.ReleaseAudit{
            TagName:      "v1.0.0",
            Integrity:    types.LevelResult{Status: types.LevelOk},
            Distribution: types.LevelResult{Status: types.LevelOk},
            Changelog:    types.LevelResult{Status: types.LevelOk},
            Diff:         types.LevelResult{Status: types.LevelOk},
            Presentation: types.LevelResult{Status: types.LevelOk},
            SupplyChain:  types.LevelResult{Status: level, Issues: []string{"release has no sbom artifact"}},
        }
        if actual := computeReleaseStatus(release); expected != actual {
            t.Errorf("a %s supply-chain level must make the release %s, got %s", level, expected, actual)
        }
    }
}

type recordedRequests struct {
    mutex sync.Mutex
    paths []string
}

func (instance *recordedRequests) add(path string) {
    instance.mutex.Lock()
    defer instance.mutex.Unlock()
    instance.paths = append(instance.paths, path)
}

func (instance *recordedRequests) all() []string {
    instance.mutex.Lock()
    defer instance.mutex.Unlock()
    return append([]string(nil), instance.paths...)
}

/* routes map a request path to a 200 body; everything else answers 404 like api.github.com does */
func fakeGithubApi(t *testing.T, routes map[string]string) (*service.GithubClient, *recordedRequests) {
    t.Helper()
    requests := &recordedRequests{}
    server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        requests.add(request.URL.Path)
        body, routed := routes[request.URL.Path]
        if false == routed {
            writer.WriteHeader(http.StatusNotFound)
            writer.Write([]byte(`{"message":"Not Found"}`))
            return
        }
        writer.Header().Set("Content-Type", "application/json")
        writer.Write([]byte(body))
    }))
    t.Cleanup(server.Close)

    client := service.NewGithubClient("")
    client.SetEndpoints(server.URL, server.URL)

    return client, requests
}

func projectAuditWithRelease(repository, tag string) types.ProjectAudit {
    release := types.ReleaseAudit{TagName: tag}
    release.Status = computeReleaseStatus(release)

    return types.ProjectAudit{
        OrganizationName: "acme",
        RepositoryName:   repository,
        Releases:         []types.ReleaseAudit{release},
        Status:           types.StatusOk,
    }
}

func TestApplySupplyChainAuditsRecordsPerProjectFailures(t *testing.T) {
    client, requests := fakeGithubApi(t, map[string]string{
        "/repos/acme/fine/releases": `[{"tag_name":"v1.0.0","assets":[]}]`,
    })
    fetchErrorAudit := buildFetchErrorAudit(project.ProjectConfig{GithubUrl: "https://github.com/acme/unreachable"}, fmt.Errorf("get tags: http 500"))
    audits := []types.ProjectAudit{
        projectAuditWithRelease("broken", "v1.0.0"),
        projectAuditWithRelease("fine", "v1.0.0"),
        fetchErrorAudit,
    }

    if applyErr := applySupplyChainAudits(client, audits, "signed-tags", false); nil != applyErr {
        t.Fatalf("one failing project must not abort the command, got %v", applyErr)
    }

    broken := audits[0].Releases[0].SupplyChain
    if types.LevelWarning != broken.Status || 1 != len(broken.Issues) || false == strings.HasPrefix(broken.Issues[0], "supply-chain check unavailable: ") {
        t.Errorf("the failing project must carry the failure on its supply-chain level, got %#v", broken)
    }
    fine := audits[1].Releases[0].SupplyChain
    if types.LevelWarning != fine.Status || 1 != len(fine.Issues) || "tag signature is not verified (tag reference not found)" != fine.Issues[0] {
        t.Errorf("a missing tag reference is a finding, not an abort, got %#v", fine)
    }
    if fetchErrorAudit.Status != audits[2].Status || fetchErrorAudit.IntegrityStatus != audits[2].IntegrityStatus || fetchErrorAudit.FetchError != audits[2].FetchError {
        t.Errorf("a fetch-error audit must be left exactly as it was, got %#v", audits[2])
    }
    for _, path := range requests.all() {
        if true == strings.Contains(path, "/unreachable/") {
            t.Errorf("a fetch-error project must not be fetched again, got request %s", path)
        }
    }
}

func TestSupplyChainIssueOrderIsStable(t *testing.T) {
    client, _ := fakeGithubApi(t, map[string]string{
        "/repos/acme/widget/releases": `[{"tag_name":"v1.0.0","assets":[]}]`,
    })
    expected := []string{"release has no checksums artifact", "release has no sbom artifact"}

    for run := 0; 50 > run; run++ {
        audits := []types.ProjectAudit{projectAuditWithRelease("widget", "v1.0.0")}
        if applyErr := applySupplyChainAudits(client, audits, "checksums,sbom", true); nil != applyErr {
            t.Fatal(applyErr)
        }
        actual := audits[0].Releases[0].SupplyChain.Issues
        if false == reflect.DeepEqual(expected, actual) {
            t.Fatalf("run %d produced the issues in the order %v, want %v", run, actual, expected)
        }
        if types.StatusFailed != audits[0].Releases[0].Status || types.LevelFailed != audits[0].SupplyChainStatus {
            t.Fatalf("--supply-chain-fail must fail the release and the project, got %s / %s", audits[0].Releases[0].Status, audits[0].SupplyChainStatus)
        }
    }
}
