package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fakeTool(t *testing.T, dir, name, output string, exitCode int) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".cmd")
		body := "@echo off\r\n"
		if output != "" {
			body += "echo " + output + "\r\n"
		}
		body += "exit /b " + itoa(exitCode) + "\r\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, name)
	body := "#!/bin/sh\n"
	if output != "" {
		body += "printf '%s\\n' " + shellQuote(output) + "\n"
	}
	body += "exit " + itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func setupEnv(t *testing.T, toolDir, home string) {
	t.Helper()
	t.Setenv("PATH", toolDir)
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".CMD;.EXE")
		t.Setenv("USERPROFILE", home)
		t.Setenv("USERNAME", "alice")
	} else {
		t.Setenv("HOME", home)
		t.Setenv("USER", "alice")
	}
}

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func TestVersionHelpAndUnknown(t *testing.T) {
	out, _, code := runCLI(t, "version")
	if code != 0 || !strings.Contains(out, "kankyo") || !strings.Contains(out, "go:") {
		t.Fatalf("version code=%d out=%q", code, out)
	}
	out, _, code = runCLI(t, "help", "scan")
	if code != 0 || !strings.Contains(out, "kankyo scan") {
		t.Fatalf("help scan code=%d out=%q", code, out)
	}
	_, errOut, code := runCLI(t, "does-not-exist")
	if code != 2 || !strings.Contains(errOut, "unknown command") {
		t.Fatalf("unknown code=%d err=%q", code, errOut)
	}
}

func TestScanWithFakeTool(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	fakeTool(t, bin, "go", "go version go1.99.0 windows/amd64", 0)
	setupEnv(t, bin, home)

	out, errOut, code := runCLI(t, "scan")
	if code != 0 {
		t.Fatalf("scan code=%d err=%q", code, errOut)
	}
	for _, want := range []string{"Go", "1.99.0", "PATH Analysis", "Summary"} {
		if !strings.Contains(out, want) {
			t.Errorf("scan output missing %q:\n%s", want, out)
		}
	}
}

func TestScanFailingToolExitOne(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	fakeTool(t, bin, "go", "go version go1.99.0 windows/amd64", 7)
	setupEnv(t, bin, home)

	out, _, code := runCLI(t, "scan")
	if code != 1 {
		t.Fatalf("scan code=%d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "TOOL_EXECUTION_FAILED") {
		t.Fatalf("missing failure finding:\n%s", out)
	}
}

func TestScanNoTools(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	setupEnv(t, bin, home)
	out, errOut, code := runCLI(t, "scan")
	if code != 0 {
		t.Fatalf("scan code=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, "Not found") {
		t.Fatalf("expected missing tool output:\n%s", out)
	}
}

func TestPathCommand(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	fakeTool(t, bin, "go", "go version go1.99.0 windows/amd64", 0)
	setupEnv(t, bin+string(os.PathListSeparator)+bin, home)
	out, _, code := runCLI(t, "path")
	if code != 0 {
		t.Fatalf("path code=%d", code)
	}
	if !strings.Contains(out, "Duplicate entries") {
		t.Fatalf("expected duplicate finding:\n%s", out)
	}
}

func TestToolsCommandFiltersPathNoise(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	fakeTool(t, bin, "go", "go version go1.99.0 windows/amd64", 0)
	setupEnv(t, bin, home)
	out, _, code := runCLI(t, "tools")
	if code != 0 {
		t.Fatalf("tools code=%d", code)
	}
	if strings.Contains(out, "PATH_DIRECTORY_NOT_FOUND") {
		t.Fatalf("tools output contains path noise:\n%s", out)
	}
	if !strings.Contains(out, "Go") {
		t.Fatalf("tools output missing Go:\n%s", out)
	}
}

func TestReportJSONAndRedaction(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	fakeTool(t, bin, "go", "go version go1.99.0 windows/amd64", 0)
	setupEnv(t, bin, home)

	outPath := filepath.Join(t.TempDir(), "report.json")
	_, errOut, code := runCLI(t, "report", "--output", outPath)
	if code != 0 {
		t.Fatalf("report code=%d err=%q", code, errOut)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, raw)
	}
	if decoded["schema_version"] != "1.0" {
		t.Fatalf("schema = %v", decoded["schema_version"])
	}

	redPath := filepath.Join(t.TempDir(), "redacted.json")
	_, _, code = runCLI(t, "report", "--output", redPath, "--redact")
	if code != 0 {
		t.Fatalf("redacted report code=%d", code)
	}
	red, err := os.ReadFile(redPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(red), "alice") {
		t.Fatalf("redacted report contains username:\n%s", red)
	}
	if !strings.Contains(string(red), "HOME") {
		t.Fatalf("redacted report missing HOME marker:\n%s", red)
	}
}

func TestReportStdoutAndFailures(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	fakeTool(t, bin, "go", "go version go1.99.0 windows/amd64", 0)
	setupEnv(t, bin, home)

	out, errOut, code := runCLI(t, "report", "--output", "-")
	if code != 0 {
		t.Fatalf("stdout report code=%d err=%q", code, errOut)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("stdout is not JSON:\n%s", out)
	}

	_, errOut, code = runCLI(t, "report")
	if code != 2 || !strings.Contains(errOut, "--output is required") {
		t.Fatalf("missing output code=%d err=%q", code, errOut)
	}

	dirOut := t.TempDir()
	_, _, code = runCLI(t, "report", "--output", dirOut)
	if code != 3 {
		t.Fatalf("directory output code=%d, want 3", code)
	}
}

func TestWelcomeAndUsage(t *testing.T) {
	out, _, code := runCLI(t)
	if code != 0 || !strings.Contains(out, "Commands:") || !strings.Contains(out, "kankyo scan") {
		t.Fatalf("welcome code=%d out=%q", code, out)
	}
	out, _, code = runCLI(t, "help")
	if code != 0 || !strings.Contains(out, "Exit codes:") {
		t.Fatalf("help code=%d out=%q", code, out)
	}
}
