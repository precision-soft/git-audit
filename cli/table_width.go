package cli

import (
    "os"

    "golang.org/x/term"
)

/* melody's default of 120 wraps on a wide terminal and on a redirected stdout alike */

const (
    autoTableMinWidth       = 120
    autoTableUnlimitedWidth = 100000
)

func autoTableMaxWidth() int {
    fileDescriptor := int(os.Stdout.Fd())
    if false == term.IsTerminal(fileDescriptor) {
        return autoTableUnlimitedWidth
    }

    width, _, err := term.GetSize(fileDescriptor)
    if nil != err || 1 > width {
        return autoTableUnlimitedWidth
    }

    if autoTableMinWidth > width {
        return autoTableMinWidth
    }
    return width
}
