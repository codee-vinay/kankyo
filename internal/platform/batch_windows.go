package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// SystemCmdPath returns the validated system cmd.exe path. It does not consult
// ComSpec and does not fall back to an arbitrary interpreter.
func SystemCmdPath() (string, error) {
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return "", fmt.Errorf("GetSystemDirectory: %w", err)
	}
	systemDir = filepath.Clean(systemDir)
	if systemDir == "" || !filepath.IsAbs(systemDir) {
		return "", fmt.Errorf("invalid system directory %q", systemDir)
	}

	cmdPath := filepath.Join(systemDir, "cmd.exe")
	if !filepath.IsAbs(cmdPath) {
		return "", fmt.Errorf("constructed cmd.exe path is not absolute: %q", cmdPath)
	}
	if !strings.EqualFold(filepath.Base(cmdPath), "cmd.exe") {
		return "", fmt.Errorf("constructed path basename is not cmd.exe: %q", cmdPath)
	}
	if !strings.EqualFold(filepath.Clean(filepath.Dir(cmdPath)), systemDir) {
		return "", fmt.Errorf("constructed cmd.exe is not directly under the system directory: %q", cmdPath)
	}

	info, err := os.Lstat(cmdPath)
	if err != nil {
		return "", fmt.Errorf("Lstat system cmd.exe: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("system cmd.exe is not a regular file: %v", info.Mode())
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("system cmd.exe is a symlink")
	}

	cmdPtr, err := windows.UTF16PtrFromString(cmdPath)
	if err != nil {
		return "", fmt.Errorf("UTF16PtrFromString: %w", err)
	}
	attrs, err := windows.GetFileAttributes(cmdPtr)
	if err != nil {
		return "", fmt.Errorf("GetFileAttributes: %w", err)
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "", fmt.Errorf("system cmd.exe has reparse-point attribute: 0x%x", attrs)
	}
	return cmdPath, nil
}

// ComSpecMismatch compares ComSpec with the validated system interpreter. The
// returned actual value is informational; it is never executed.
func ComSpecMismatch() (mismatch bool, expected, actual string) {
	expected, err := SystemCmdPath()
	if err != nil {
		return false, "", os.Getenv("ComSpec")
	}
	actual, ok := os.LookupEnv("ComSpec")
	if !ok || actual == "" {
		return false, expected, ""
	}
	return !strings.EqualFold(filepath.Clean(actual), expected), expected, actual
}

func ValidateBatchPath(batchPath string) error {
	if !filepath.IsAbs(batchPath) {
		return fmt.Errorf("batch path is not absolute: %q", batchPath)
	}
	ext := strings.ToLower(filepath.Ext(batchPath))
	if ext != ".cmd" && ext != ".bat" {
		return fmt.Errorf("batch path does not end in .cmd/.bat: %q", batchPath)
	}
	if strings.ContainsAny(batchPath, "\"%!^&|<>"+"\r\n\x00") {
		return fmt.Errorf("batch path contains a rejected metacharacter: %q", batchPath)
	}
	return nil
}

func BuildBatchCommandLine(systemCmd, batchPath string, args []string) (string, error) {
	if !filepath.IsAbs(systemCmd) {
		return "", fmt.Errorf("system cmd.exe path is not absolute: %q", systemCmd)
	}
	if !strings.EqualFold(filepath.Base(systemCmd), "cmd.exe") {
		return "", fmt.Errorf("system interpreter basename is not cmd.exe: %q", systemCmd)
	}
	if err := ValidateBatchPath(batchPath); err != nil {
		return "", err
	}
	for _, arg := range args {
		if arg == "" {
			return "", fmt.Errorf("empty batch argument")
		}
		if strings.ContainsAny(arg, "\"%!^&|<> \t\r\n") {
			return "", fmt.Errorf("batch argument contains a rejected character: %q", arg)
		}
	}

	inner := `"` + batchPath + `"`
	if len(args) > 0 {
		inner += " " + strings.Join(args, " ")
	}
	return `"` + systemCmd + `" /d /s /v:off /c "` + inner + `"`, nil
}
