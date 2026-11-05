package platform

import (
	"path/filepath"
	"runtime"
	"strings"
)

type Info struct {
	OS   string
	Arch string
}

func Current() Info {
	return Info{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func PathListSeparator() string {
	return string(filepath.ListSeparator)
}

func SplitList(raw string) []string {
	return filepath.SplitList(raw)
}

func NormalizePath(path string) string {
	return filepath.Clean(path)
}

func IsBatchFile(path string) bool {
	if !IsWindows() {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".cmd" || ext == ".bat"
}

func IsWindowsAppAlias(path string) bool {
	if !IsWindows() {
		return false
	}
	return strings.Contains(strings.ToLower(path), `\microsoft\windowsapps\`)
}

func ClampWidth(width int) int {
	if width < 40 {
		return 40
	}
	if width > 120 {
		return 120
	}
	return width
}
