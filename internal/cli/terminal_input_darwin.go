//go:build darwin

package cli

import (
	"os"

	"golang.org/x/sys/unix"
)

func flushTerminalInput() {
	_ = unix.IoctlSetInt(int(os.Stdin.Fd()), unix.TIOCFLUSH, unix.TCIFLUSH)
}
