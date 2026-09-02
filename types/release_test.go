package types

import (
    "encoding/json"
    "testing"
)

func TestLevelResultSerialisesLowerCamelKeys(t *testing.T) {
    encoded, marshalErr := json.Marshal(LevelResult{Status: LevelWarning, Issues: []string{"release has no sbom artifact"}})
    if nil != marshalErr {
        t.Fatal(marshalErr)
    }
    if `{"status":"warning","issues":["release has no sbom artifact"]}` != string(encoded) {
        t.Fatalf("the level keys must be lowerCamel like every other key in the document, got %s", encoded)
    }
}
