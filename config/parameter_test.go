package config

import (
    "testing"
)

type recordingParameterRegistrar struct {
    plain  map[string]any
    secret map[string]any
    marked []string
}

func newRecordingParameterRegistrar() *recordingParameterRegistrar {
    return &recordingParameterRegistrar{
        plain:  map[string]any{},
        secret: map[string]any{},
    }
}

func (instance *recordingParameterRegistrar) RegisterParameter(name string, value any) {
    instance.plain[name] = value
}

func (instance *recordingParameterRegistrar) RegisterSecretParameter(name string, value any) {
    instance.secret[name] = value
}

func (instance *recordingParameterRegistrar) MarkParameterSecret(name string) {
    instance.marked = append(instance.marked, name)
}

func TestRegisterParametersDeclaresTheTokenAsACredential(t *testing.T) {
    registrar := newRecordingParameterRegistrar()

    (&GithubAuditModule{}).RegisterParameters(registrar)

    if 2 != len(registrar.plain) {
        t.Fatalf("only the two base urls may be registered in clear text, got %v", registrar.plain)
    }

    expectedValue := "%env(" + EnvironmentGithubToken + ")%"

    value, registered := registrar.secret[ParameterGithubToken]

    if false == registered {
        t.Fatalf("`%s` must be registered as a secret parameter", ParameterGithubToken)
    }

    if value != expectedValue {
        t.Fatalf("`%s` must resolve to `%s`, got `%v`", ParameterGithubToken, expectedValue, value)
    }
}

func TestRegisterParametersMarksTheEnvironmentVariableSecret(t *testing.T) {
    registrar := newRecordingParameterRegistrar()

    (&GithubAuditModule{}).RegisterParameters(registrar)

    if 1 != len(registrar.marked) || EnvironmentGithubToken != registrar.marked[0] {
        t.Fatalf("`%s` must be marked secret, got %v", EnvironmentGithubToken, registrar.marked)
    }
}

func TestRegisterParametersDeclaresTheBaseUrlsAsOptional(t *testing.T) {
    registrar := newRecordingParameterRegistrar()

    (&GithubAuditModule{}).RegisterParameters(registrar)

    expected := map[string]string{
        ParameterGithubApiBase: "%env(default::" + EnvironmentGithubApiBase + ")%",
        ParameterGithubRawBase: "%env(default::" + EnvironmentGithubRawBase + ")%",
    }
    for name, expectedValue := range expected {
        value, registered := registrar.plain[name]
        if false == registered {
            t.Fatalf("`%s` must be registered as a plain parameter", name)
        }
        if value != expectedValue {
            t.Fatalf("`%s` must resolve to `%s`, got `%v`", name, expectedValue, value)
        }
        if _, secret := registrar.secret[name]; true == secret {
            t.Fatalf("`%s` is a host, not a credential", name)
        }
    }
}
