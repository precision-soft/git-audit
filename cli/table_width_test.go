package cli

import "testing"

func TestAutoTableMaxWidthUnlimitedWhenNotTerminal(t *testing.T) {
    /** under `go test` stdout is a pipe, not a terminal, so the width is effectively unbounded */
    if got := autoTableMaxWidth(); autoTableUnlimitedWidth != got {
        t.Errorf("expected unlimited width %d when stdout is not a terminal, got %d", autoTableUnlimitedWidth, got)
    }
}
