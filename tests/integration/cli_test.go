package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildFakeTool(t *testing.T, dir, name, output string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".cmd")
		body := "@echo off\r\necho " + output + "\r\nexit /b 0\r\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	path := filepath.Join(dir, name)
	body := "#!/bin/sh\nprintf '%s\\n' '" + strings.ReplaceAll(output, "'", "'\\''") + "'\nexit 0\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func goBinary(t *testing.T) string {
	t.Helper()
	name := "go"
	if runtime.GOOS == "windows" {
		name = "go.exe"
	}
	candidate := filepath.Join(runtime.GOROOT(), "bin", name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	if found, err := exec.LookPath("go"); err == nil {
		return found
	}
	t.Skip("Go toolchain not found for binary integration test")
	return ""
}

func TestBuiltBinary(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	buildDir := t.TempDir()
	exeName := "kankyo"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exe := filepath.Join(buildDir, exeName)
	build := exec.Command(goBinary(t), "build", "-o", exe, "./cmd/kankyo")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	buildFakeTool(t, bin, "go", "go version go1.99.0 windows/amd64")

	env := append(os.Environ(),
		"PATH="+bin,
		"HOME="+home,
		"USERPROFILE="+home,
		"USER=alice",
		"USERNAME=alice",
	)
	if runtime.GOOS == "windows" {
		env = append(env, "PATHEXT=.CMD;.EXE")
	}

	run := func(args ...string) (string, string, int) {
		cmd := exec.Command(exe, args...)
		cmd.Env = env
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				t.Fatalf("run %v: %v", args, err)
			}
		}
		return stdout.String(), stderr.String(), code
	}

	out, stderr, code := run("version")
	if code != 0 || !strings.Contains(out, "kankyo") {
		t.Fatalf("version code=%d err=%q out=%q", code, stderr, out)
	}

	out, stderr, code = run("report", "--output", "-")
	if code != 0 {
		t.Fatalf("report code=%d err=%q", code, stderr)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("report JSON: %v\n%s", err, out)
	}
	if decoded["schema_version"] != "1.0" {
		t.Fatalf("schema = %v", decoded["schema_version"])
	}

	out, stderr, code = run("scan")
	if code != 0 {
		t.Fatalf("scan code=%d err=%q", code, stderr)
	}
	if !strings.Contains(out, "Go") || !strings.Contains(out, "1.99.0") {
		t.Fatalf("scan output missing fake Go:\n%s", out)
	}
}
