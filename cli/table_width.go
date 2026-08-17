package cli

import (
    "os"

    "golang.org/x/term"
)

/*
A minimum floor and no maximum cap: melody's TableMaxWidth defaults to 120 when unset, which wraps
on a wide terminal and on a redirected stdout alike. The width is auto-sized from the controlling
terminal instead, and effectively unlimited when stdout is not one, so the data drives it.
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
