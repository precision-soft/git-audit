package cli

import (
    "os"

    "golang.org/x/term"
)

/*
Table width policy: minimum floor, no maximum cap.

Melody's TableMaxWidth defaults to 120 when unset, which forced wrapping on
wide terminals and looked cramped when output was piped to a file. Instead,
we auto-size from the controlling terminal (never below autoTableMinWidth)
and fall back to effectively-unlimited when stdout is redirected so that the
data — not a hardcoded constant — drives the width.
*/

const (
    autoTableMinWidth       = 120
    autoTableUnlimitedWidth = 100000
)

func autoTableMaxWidth() int {
    fd := int(os.Stdout.Fd())
    if false == term.IsTerminal(fd) {
        return autoTableUnlimitedWidth
    }

    width, _, err := term.GetSize(fd)
    if nil != err || 1 > width {
        return autoTableUnlimitedWidth
    }

    if width < autoTableMinWidth {
        return autoTableMinWidth
    }
    return width
}
