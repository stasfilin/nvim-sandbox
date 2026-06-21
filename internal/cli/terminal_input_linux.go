//go:build linux

package cli

import (
	"os"

	"golang.org/x/sys/unix"
)

func flushTerminalInput() {
	_ = unix.IoctlSetInt(int(os.Stdin.Fd()), unix.TCFLSH, unix.TCIFLUSH)
}
