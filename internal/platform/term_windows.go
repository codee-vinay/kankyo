package platform

import (
	"os"

	"golang.org/x/sys/windows"
)

func supportsColor(f *os.File) bool {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(f.Fd()), &mode); err != nil {
		return false
	}
	return mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0
}
