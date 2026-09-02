//go:build e2e

/*
End-to-end coverage for the built binary: these compile the command, run it out of a scratch
directory and assert on what an operator sees — stdout, the exit code, the files left behind. The
build tag keeps them out of `go test ./...`, which stays the fast offline gate; run them with
`go test -tags=e2e ./...`. A prerequisite this module cannot provide (git, a GitHub token) skips
instead of failing.
*/
package main

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "os/exec"
    "path/filepath"
    "reflect"
    "regexp"
    "strings"
    "sync"
    "testing"
)

/* every line melody writes around a command starts with an ANSI escape; nothing a command
   renders does. */
var melodyBannerLine = regexp.MustCompile("^\x1b\\[")

/*
commandDocument returns the part of stdout that belongs to the command, with melody's framing
removed. melody wraps every command's stdout in a coloured started/finished banner in every format
— cli/command.go consults neither the format nor a flag — so no melody CLI is pipeable as-is and
this module cannot change that. What it can guarantee, and what these tests assert, is that
everything between the banners is exactly one document.
*/
func commandDocument(stdout string) string {
    var kept []string
    for _, line := range strings.Split(stdout, "\n") {
        if true == melodyBannerLine.MatchString(line) {
            continue
        }

        kept = append(kept, line)
    }

    return strings.TrimSpace(strings.Join(kept, "\n"))
}

/* searched for verbatim in the output, so it must not look like anything else the tool prints */
const canaryToken = "ghp_e2eCANARY0123456789"

var binaryPath string

func TestMain(m *testing.M) {
    directory, tempErr := os.MkdirTemp("", "git-audit-e2e-")
    if nil != tempErr {
        panic(tempErr)
    }

    binaryPath = filepath.Join(directory, "git-audit")

    build := exec.Command("go", "build", "-o", binaryPath, ".")
    build.Stderr = os.Stderr
    if buildErr := build.Run(); nil != buildErr {
        os.RemoveAll(directory)
        panic(buildErr)
    }

    /* melody derives the project directory from the executable location, so these have to sit next
       to the binary rather than in the working directory */
    environment := "MELODY_ENV=dev\nMELODY_DEFAULT_MODE=cli\nMELODY_CLI_NAME=git-audit\nMELODY_LOG_LEVEL=error\n"
    if writeErr := os.WriteFile(filepath.Join(directory, ".env"), []byte(environment), 0o644); nil != writeErr {
        os.RemoveAll(directory)
        panic(writeErr)
    }
    if writeErr := os.WriteFile(filepath.Join(directory, ".env.local"), []byte("GITHUB_TOKEN="+canaryToken+"\n"), 0o600); nil != writeErr {
        os.RemoveAll(directory)
        panic(writeErr)
    }

    code := m.Run()

    os.RemoveAll(directory)
    os.Exit(code)
}

type commandResult struct {
    stdout   string
    stderr   string
    exitCode int
}

func runBinary(t *testing.T, workingDirectory string, arguments ...string) commandResult {
    t.Helper()

    command := exec.Command(binaryPath, arguments...)
    command.Dir = workingDirectory

    var stdout, stderr strings.Builder
    command.Stdout = &stdout
    command.Stderr = &stderr

    exitCode := 0
    if runErr := command.Run(); nil != runErr {
        exitError, isExitError := runErr.(*exec.ExitError)
        if false == isExitError {
            t.Fatalf("running %v: %v", arguments, runErr)
        }
        exitCode = exitError.ExitCode()
    }

    return commandResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode}
}

func TestBinaryListsItsCommands(t *testing.T) {
    result := runBinary(t, t.TempDir(), "--help")

    for _, command := range []string{"audit", "sync", "exceptions", "debug:parameters"} {
        if false == strings.Contains(result.stdout, command) {
            t.Errorf("expected %q in the help output, got:\n%s", command, result.stdout)
        }
    }
}

func TestBinaryRedactsTheGithubToken(t *testing.T) {
    for _, format := range []string{"table", "json"} {
        result := runBinary(t, t.TempDir(), "debug:parameters", "--format", format)

        if 0 != result.exitCode {
            t.Fatalf("debug:parameters --format %s exited %d, stderr:\n%s", format, result.exitCode, result.stderr)
        }
        if true == strings.Contains(result.stdout, canaryToken) {
            t.Errorf("the github token reached stdout in the %s format", format)
        }
        if true == strings.Contains(result.stderr, canaryToken) {
            t.Errorf("the github token reached stderr in the %s format", format)
        }
        if false == strings.Contains(result.stdout, "********") {
            t.Errorf("expected a redacted value in the %s format, got:\n%s", format, result.stdout)
        }
    }
}

func TestBinaryEmitsParseableJson(t *testing.T) {
    result := runBinary(t, t.TempDir(), "debug:parameters", "--format", "json")

    document := commandDocument(result.stdout)

    var decoded any
    if unmarshalErr := json.Unmarshal([]byte(document), &decoded); nil != unmarshalErr {
        t.Fatalf("the rendered document is not parseable json (%v):\n%s", unmarshalErr, document)
    }
}

func TestBinaryRendersExceptionsFromAFile(t *testing.T) {
    workingDirectory := t.TempDir()
    exceptionsPath := filepath.Join(workingDirectory, "exceptions.json")

    const issue = `title summary "lowercase" must start with uppercase`
    content := `{"precision-soft/doctrine-type":{"v1.0.0":{"presentation":[` + jsonString(issue) + `]}}}`
    if writeErr := os.WriteFile(exceptionsPath, []byte(content), 0o644); nil != writeErr {
        t.Fatalf("writing the exceptions file: %v", writeErr)
    }

    result := runBinary(t, workingDirectory, "exceptions", "--exceptions", exceptionsPath, "--format", "json")

    if 0 != result.exitCode {
        t.Fatalf("exceptions exited %d, stderr:\n%s", result.exitCode, result.stderr)
    }

    document := commandDocument(result.stdout)

    var decoded map[string]any
    if unmarshalErr := json.Unmarshal([]byte(document), &decoded); nil != unmarshalErr {
        t.Fatalf("the rendered document is not parseable json (%v):\n%s", unmarshalErr, document)
    }
    if false == strings.Contains(result.stdout, "must start with uppercase") {
        t.Errorf("expected the exception to be listed, got:\n%s", result.stdout)
    }
}

func TestBinaryRejectsAnUnknownRepository(t *testing.T) {
    result := runBinary(t, t.TempDir(), "audit", "--token", canaryToken, "--repo", "not-a-precision-soft-project")

    if 0 == result.exitCode {
        t.Fatalf("expected a non-zero exit for an unknown repo, stdout:\n%s", result.stdout)
    }

    combined := result.stdout + result.stderr
    if false == strings.Contains(combined, "unknown repo") {
        t.Errorf("expected the unknown-repo message, got:\n%s", combined)
    }
}

/* the GitHub call that follows the clone needs a real token, so only the clone half is driven */
func TestBinaryClonesAnAdHocRepositoryForSync(t *testing.T) {
    if _, lookErr := exec.LookPath("git"); nil != lookErr {
        t.Skip("git is not on PATH")
    }

    origin := filepath.Join(t.TempDir(), "scratch")
    createScratchRepository(t, origin)

    workingDirectory := t.TempDir()
    result := runBinary(
        t,
        workingDirectory,
        "sync",
        "--token", canaryToken,
        "--repo", "scratch",
        "--repo-url", "file://"+origin,
        "--format", "json",
    )

    combined := result.stdout + result.stderr
    if true == strings.Contains(combined, "clone:") {
        t.Fatalf("the clone stage failed, output:\n%s", combined)
    }

    clonedChangelog := filepath.Join(workingDirectory, ".dev-data", "clones", "scratch", "CHANGELOG.md")
    if _, statErr := os.Stat(clonedChangelog); nil != statErr {
        t.Fatalf("expected the changelog in the clone at %s: %v", clonedChangelog, statErr)
    }

    document := commandDocument(result.stdout)

    var decoded any
    if unmarshalErr := json.Unmarshal([]byte(document), &decoded); nil != unmarshalErr {
        t.Errorf("the rendered sync document is not parseable json (%v):\n%s", unmarshalErr, document)
    }
}

func createScratchRepository(t *testing.T, directory string) {
    t.Helper()

    if mkdirErr := os.MkdirAll(directory, 0o755); nil != mkdirErr {
        t.Fatalf("creating the scratch repository: %v", mkdirErr)
    }

    changelog := "# Changelog\n\n" +
        "## [v1.0.0] - 2026-01-01 - Initial release\n\n" +
        "### Added\n\n- the first thing\n\n" +
        "[v1.0.0]: https://example.com/compare/v0.9.0...v1.0.0\n"
    if writeErr := os.WriteFile(filepath.Join(directory, "CHANGELOG.md"), []byte(changelog), 0o644); nil != writeErr {
        t.Fatalf("writing the scratch changelog: %v", writeErr)
    }

    commands := [][]string{
        {"init", "--initial-branch", "main"},
        {"add", "CHANGELOG.md"},
        {"-c", "user.name=git-audit e2e", "-c", "user.email=e2e@example.com", "commit", "-m", "initial"},
        {"tag", "v1.0.0"},
    }
    for _, arguments := range commands {
        command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
        if output, runErr := command.CombinedOutput(); nil != runErr {
            t.Fatalf("git %v in the scratch repository: %v: %s", arguments, runErr, output)
        }
    }
}

func jsonString(value string) string {
    encoded, marshalErr := json.Marshal(value)
    if nil != marshalErr {
        panic(marshalErr)
    }

    return string(encoded)
}

/* one repository, one tag with its release, and a changelog on the raw host: enough for every level to run */
const fakeCommitSha = "0123456789abcdef0123456789abcdef01234567"

var fakeGithubRoutes = map[string]string{
    "/repos/acme/widget/tags":     `[{"name":"v1.0.0","commit":{"sha":"` + fakeCommitSha + `"}}]`,
    "/repos/acme/widget/releases": `[{"id":1,"tag_name":"v1.0.0","name":"Widget v1.0.0 - First release","body":"## Added\n\n- the first thing\n","draft":false,"prerelease":false,"target_commitish":"` + fakeCommitSha + `","assets":[]}]`,
    "/raw/acme/widget/HEAD/CHANGELOG.md": "# Changelog\n\n" +
        "## [v1.0.0] - 2026-01-01 - First release\n\n" +
        "### Added\n\n- the first thing\n\n" +
        "[v1.0.0]: https://github.com/acme/widget/compare/v0.9.0...v1.0.0\n",
}

type fakeGithub struct {
    server         *httptest.Server
    mutex          sync.Mutex
    conditionalGet map[string]int
}

func (instance *fakeGithub) conditionalRequests(path string) int {
    instance.mutex.Lock()
    defer instance.mutex.Unlock()

    return instance.conditionalGet[path]
}

/*
startFakeGithub serves the api and the raw host from one server and answers 304 to a matching
If-None-Match, so the binary's cache can be observed from outside
*/
func startFakeGithub(t *testing.T) *fakeGithub {
    t.Helper()

    fake := &fakeGithub{conditionalGet: make(map[string]int)}
    fake.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        body, routed := fakeGithubRoutes[request.URL.Path]
        if false == routed {
            writer.WriteHeader(http.StatusNotFound)
            writer.Write([]byte(`{"message":"Not Found"}`))
            return
        }

        entityTag := `"` + strings.ReplaceAll(strings.Trim(request.URL.Path, "/"), "/", "-") + `"`
        if entityTag == request.Header.Get("If-None-Match") {
            fake.mutex.Lock()
            fake.conditionalGet[request.URL.Path]++
            fake.mutex.Unlock()
            writer.WriteHeader(http.StatusNotModified)
            return
        }
        writer.Header().Set("ETag", entityTag)
        writer.Write([]byte(body))
    }))
    t.Cleanup(fake.server.Close)

    return fake
}

/* melody reads the parameters from the .env files next to the binary, never from the process environment */
func pointBinaryAt(t *testing.T, fake *fakeGithub) {
    t.Helper()

    environmentPath := filepath.Join(filepath.Dir(binaryPath), ".env.local")
    environment := "GITHUB_TOKEN=" + canaryToken + "\n" +
        "GITHUB_API_BASE=" + fake.server.URL + "\n" +
        "GITHUB_RAW_BASE=" + fake.server.URL + "/raw\n"
    if writeErr := os.WriteFile(environmentPath, []byte(environment), 0o600); nil != writeErr {
        t.Fatalf("pointing the binary at the fake server: %v", writeErr)
    }
    t.Cleanup(func() {
        os.WriteFile(environmentPath, []byte("GITHUB_TOKEN="+canaryToken+"\n"), 0o600)
    })
}

func manifestPath(t *testing.T, name string) string {
    t.Helper()

    absolutePath, absErr := filepath.Abs(filepath.Join("testdata", name))
    if nil != absErr {
        t.Fatal(absErr)
    }

    return absolutePath
}

type auditDocument struct {
    Data struct {
        Items []struct {
            OrganizationName  string `json:"organizationName"`
            RepositoryName    string `json:"repositoryName"`
            SupplyChainStatus string `json:"supplyChainStatus"`
            Status            string `json:"status"`
            FetchError        string `json:"fetchError"`
            Releases          []struct {
                TagName     string `json:"tagName"`
                Status      string `json:"status"`
                SupplyChain struct {
                    Status string   `json:"status"`
                    Issues []string `json:"issues"`
                } `json:"supplyChain"`
            } `json:"releases"`
        } `json:"items"`
    } `json:"data"`
}

func decodeAuditDocument(t *testing.T, result commandResult) auditDocument {
    t.Helper()

    document := commandDocument(result.stdout)
    var decoded auditDocument
    if unmarshalErr := json.Unmarshal([]byte(document), &decoded); nil != unmarshalErr {
        t.Fatalf("the rendered audit is not parseable json (%v):\n%s\nstderr:\n%s", unmarshalErr, document, result.stderr)
    }

    return decoded
}

func TestBinaryAuditsProjectsFromAManifest(t *testing.T) {
    pointBinaryAt(t, startFakeGithub(t))

    for name, arguments := range map[string][]string{
        "replace": {"--config", manifestPath(t, "manifest-replace.json")},
        "merge":   {"--config", manifestPath(t, "manifest-merge.json"), "--repo", "widget"},
    } {
        result := runBinary(t, t.TempDir(), append([]string{"audit", "--format", "json"}, arguments...)...)
        decoded := decodeAuditDocument(t, result)

        if 1 != len(decoded.Data.Items) || "acme" != decoded.Data.Items[0].OrganizationName || "widget" != decoded.Data.Items[0].RepositoryName {
            t.Fatalf("%s manifest: expected exactly the manifest project, got %#v", name, decoded.Data.Items)
        }
        if "" != decoded.Data.Items[0].FetchError {
            t.Fatalf("%s manifest: the fake server must be reached, got fetch error %q", name, decoded.Data.Items[0].FetchError)
        }
        if 1 != len(decoded.Data.Items[0].Releases) || "v1.0.0" != decoded.Data.Items[0].Releases[0].TagName {
            t.Fatalf("%s manifest: expected the v1.0.0 release, got %#v", name, decoded.Data.Items[0].Releases)
        }
    }
}

func TestBinaryRejectsAManifestWithUnknownKeys(t *testing.T) {
    pointBinaryAt(t, startFakeGithub(t))
    workingDirectory := t.TempDir()
    manifest := filepath.Join(workingDirectory, "projects.json")
    if writeErr := os.WriteFile(manifest, []byte(`{"mode":"replace","projects":[{"name":"Widget","github_url":"https://github.com/acme/widget"}]}`), 0o600); nil != writeErr {
        t.Fatal(writeErr)
    }

    result := runBinary(t, workingDirectory, "audit", "--config", manifest, "--format", "json")

    if 0 == result.exitCode || false == strings.Contains(result.stdout+result.stderr, "unknown field") {
        t.Fatalf("a misspelled manifest key must be refused, exit %d:\n%s%s", result.exitCode, result.stdout, result.stderr)
    }
}

func TestBinaryEmitsGithubAnnotationsOnStderr(t *testing.T) {
    pointBinaryAt(t, startFakeGithub(t))

    result := runBinary(
        t,
        t.TempDir(),
        "audit",
        "--config", manifestPath(t, "manifest-replace.json"),
        "--github-annotations",
        "--supply-chain", "all",
        "--format", "json",
    )

    decodeAuditDocument(t, result)

    /* melody logs its boot as json to stderr before it reads the log level; the runner ignores every line that is not a workflow command */
    annotations := 0
    for _, line := range strings.Split(result.stderr, "\n") {
        if "" == strings.TrimSpace(line) || true == melodyBannerLine.MatchString(line) || true == strings.HasPrefix(line, "{") {
            continue
        }
        if false == strings.HasPrefix(line, "::warning ") && false == strings.HasPrefix(line, "::error ") {
            t.Fatalf("every stderr line must be a workflow command, got %q", line)
        }
        annotations++
    }
    if 0 == annotations {
        t.Fatalf("expected at least one annotation on stderr, got:\n%s", result.stderr)
    }
}

func TestBinaryReplaysTheEtagOnASecondRun(t *testing.T) {
    fake := startFakeGithub(t)
    pointBinaryAt(t, fake)
    workingDirectory := t.TempDir()
    manifest := manifestPath(t, "manifest-replace.json")

    runBinary(t, workingDirectory, "audit", "--config", manifest, "--format", "json")
    if _, statErr := os.Stat(filepath.Join(workingDirectory, ".dev-data")); nil == statErr {
        t.Fatal("without --cache-dir nothing may be written to the working directory")
    }

    cacheDirectory := filepath.Join(workingDirectory, "cache")
    for run := 0; 2 > run; run++ {
        result := runBinary(t, workingDirectory, "audit", "--config", manifest, "--cache-dir", cacheDirectory, "--format", "json")
        decoded := decodeAuditDocument(t, result)
        if 1 != len(decoded.Data.Items) || "" != decoded.Data.Items[0].FetchError {
            t.Fatalf("run %d: expected a clean audit through the cache, got %#v", run, decoded.Data.Items)
        }
    }
    if 1 != fake.conditionalRequests("/repos/acme/widget/tags") {
        t.Fatalf("the second run must revalidate the tags with If-None-Match, got %d conditional requests", fake.conditionalRequests("/repos/acme/widget/tags"))
    }
}

func TestBinaryReportsSupplyChainFields(t *testing.T) {
    pointBinaryAt(t, startFakeGithub(t))
    manifest := manifestPath(t, "manifest-replace.json")

    plain := decodeAuditDocument(t, runBinary(t, t.TempDir(), "audit", "--config", manifest, "--format", "json"))
    if "n/a" != plain.Data.Items[0].SupplyChainStatus || "n/a" != plain.Data.Items[0].Releases[0].SupplyChain.Status {
        t.Fatalf("a check that was not requested is n/a on the project and on the release, got %q / %q", plain.Data.Items[0].SupplyChainStatus, plain.Data.Items[0].Releases[0].SupplyChain.Status)
    }

    result := runBinary(t, t.TempDir(), "audit", "--config", manifest, "--supply-chain", "all", "--supply-chain-fail", "--format", "json")
    if 0 == result.exitCode {
        t.Fatal("--supply-chain-fail with findings must exit non-zero")
    }
    strict := decodeAuditDocument(t, result)
    release := strict.Data.Items[0].Releases[0]
    if "failed" != release.SupplyChain.Status || "failed" != release.Status || "failed" != strict.Data.Items[0].SupplyChainStatus || "failed" != strict.Data.Items[0].Status {
        t.Fatalf("the release, the level and the project must agree, got release %s / level %s / project level %s / project %s", release.Status, release.SupplyChain.Status, strict.Data.Items[0].SupplyChainStatus, strict.Data.Items[0].Status)
    }
    expectedIssues := []string{
        "tag signature is not verified (tag reference not found)",
        "release has no checksums artifact",
        "release has no sbom artifact",
        "release has no verified github artifact attestation",
    }
    if false == reflect.DeepEqual(expectedIssues, release.SupplyChain.Issues) {
        t.Fatalf("expected the findings in a fixed order, got %v", release.SupplyChain.Issues)
    }
}

func TestBinaryRejectsConcurrencyOutOfRange(t *testing.T) {
    pointBinaryAt(t, startFakeGithub(t))

    for _, value := range []string{"0", "33"} {
        result := runBinary(t, t.TempDir(), "audit", "--config", manifestPath(t, "manifest-replace.json"), "--concurrency", value)
        if 0 == result.exitCode || false == strings.Contains(result.stdout+result.stderr, "--concurrency must be between 1 and 32") {
            t.Errorf("--concurrency=%s must be refused, exit %d:\n%s%s", value, result.exitCode, result.stdout, result.stderr)
        }
    }
}
