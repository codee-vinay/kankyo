package platform

import "os"

func supportsColor(_ *os.File) bool {
	return true
}
