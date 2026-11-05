package platform

import (
	"io"
	"os"
	"strconv"

	"golang.org/x/term"
)

func ColorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	if !term.IsTerminal(int(f.Fd())) {
		return false
	}
	return supportsColor(f)
}

func TerminalWidth(w io.Writer) int {
	if f, ok := w.(*os.File); ok {
		if width, _, err := term.GetSize(int(f.Fd())); err == nil && width > 0 {
			return ClampWidth(width)
		}
	}
	if raw := os.Getenv("COLUMNS"); raw != "" {
		if width, err := strconv.Atoi(raw); err == nil && width > 0 {
			return ClampWidth(width)
		}
	}
	return 80
}
