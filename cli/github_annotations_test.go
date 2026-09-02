package cli

import (
    "strings"
    "testing"

    "github.com/precision-soft/git-audit/types"
)

func TestGithubAnnotationsEscapeCommands(t *testing.T) {
    audits := auditWithLevelResult("integrity", types.LevelResult{Status: types.LevelFailed, Issues: []string{"bad: value\nnext"}})
    audits[0].Releases[0].TagName = "v1,0"

    var output strings.Builder
    renderGithubAnnotations(&output, audits)

    expected := "::error title=precision-soft/doctrine-type v1%2C0 integrity::bad: value%0Anext\n"
    if expected != output.String() {
        t.Fatalf("expected %q, got %q", expected, output.String())
    }
}

func TestRenderGithubAnnotationsKeepsColonsInTheMessage(t *testing.T) {
    audits := auditWithLevelResult("presentation", types.LevelResult{Status: types.LevelWarning, Issues: []string{"title: a, b"}})

    var output strings.Builder
    renderGithubAnnotations(&output, audits)

    if false == strings.HasSuffix(output.String(), "::title: a, b\n") {
        t.Fatalf("the message part only escapes %%, CR and LF, got %q", output.String())
    }
}

func TestRenderGithubAnnotationsIsDeterministic(t *testing.T) {
    release := types.ReleaseAudit{TagName: "v1.0.0"}
    for _, level := range levelNames {
        *releaseLevels(&release)[level] = types.LevelResult{Status: types.LevelWarning, Issues: []string{level + " issue"}}
    }
    audits := []types.ProjectAudit{{OrganizationName: "acme", RepositoryName: "widget", Releases: []types.ReleaseAudit{release}}}

    var expected strings.Builder
    for _, level := range levelNames {
        expected.WriteString("::warning title=acme/widget v1.0.0 " + level + "::" + level + " issue\n")
    }

    for run := 0; 20 > run; run++ {
        var output strings.Builder
        renderGithubAnnotations(&output, audits)
        if expected.String() != output.String() {
            t.Fatalf("run %d rendered the levels out of order:\n%s", run, output.String())
        }
    }
}
