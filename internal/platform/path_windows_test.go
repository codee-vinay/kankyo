package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeBatch(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join(lines, "\r\n") + "\r\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSystemCmdPath(t *testing.T) {
	path, err := SystemCmdPath()
	if err != nil {
		t.Fatalf("SystemCmdPath: %v", err)
	}
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Base(path), "cmd.exe") {
		t.Fatalf("invalid system cmd path: %q", path)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("invalid cmd file: info=%v err=%v", info, err)
	}
	t.Logf("system_cmd=%q", path)
}

func TestPathExtAndSamePath(t *testing.T) {
	t.Setenv("PATHEXT", ".CMD;.EXE")
	got := PathExt()
	if len(got) != 2 || got[0] != ".cmd" || got[1] != ".exe" {
		t.Fatalf("PathExt = %v", got)
	}
	if !SamePath(`C:\Windows\System32`, `c:\windows\system32`) {
		t.Fatal("SamePath should be case-insensitive")
	}
}

func TestFindCandidates(t *testing.T) {
	t.Setenv("PATHEXT", ".EXE;.CMD")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.exe"), []byte("exe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.cmd"), []byte("cmd"), 0o600); err != nil {
		t.Fatal(err)
	}
	candidates := FindCandidates(dir, "go")
	if len(candidates) != 2 {
		t.Fatalf("candidates = %+v", candidates)
	}
	if !strings.HasSuffix(strings.ToLower(candidates[0].Path), ".exe") {
		t.Fatalf("PATHEXT order not respected: %+v", candidates)
	}
}

func TestBuildBatchCommandLine(t *testing.T) {
	systemCmd, err := SystemCmdPath()
	if err != nil {
		t.Fatal(err)
	}
	batchPath := filepath.Join(t.TempDir(), "dir with spaces (x86)", "npm.cmd")
	line, err := BuildBatchCommandLine(systemCmd, batchPath, []string{"--version"})
	if err != nil {
		t.Fatalf("BuildBatchCommandLine: %v", err)
	}
	if !strings.Contains(line, `"`+batchPath+`"`) {
		t.Fatalf("quoted path missing: %q", line)
	}
	if !strings.Contains(line, "/d /s /v:off /c ") {
		t.Fatalf("switches missing: %q", line)
	}
	t.Logf("command_line=%q", line)
}

func TestValidateBatchPath(t *testing.T) {
	valid := []string{
		filepath.Join(t.TempDir(), "dir with spaces", "x.cmd"),
		filepath.Join(t.TempDir(), "dir (x86)", "x.bat"),
	}
	for _, path := range valid {
		if err := ValidateBatchPath(path); err != nil {
			t.Errorf("valid path rejected %q: %v", path, err)
		}
	}
	invalid := []string{
		filepath.Join(t.TempDir(), "bad&dir", "x.cmd"),
		filepath.Join(t.TempDir(), "bad%dir", "x.cmd"),
		filepath.Join(t.TempDir(), "bad!dir", "x.cmd"),
		filepath.Join(t.TempDir(), "bad^dir", "x.cmd"),
		filepath.Join(t.TempDir(), "bad|dir", "x.cmd"),
		filepath.Join(t.TempDir(), "x.txt"),
	}
	for _, path := range invalid {
		if err := ValidateBatchPath(path); err == nil {
			t.Errorf("invalid path accepted %q", path)
		}
	}
}

func TestComSpecMismatch(t *testing.T) {
	systemCmd, err := SystemCmdPath()
	if err != nil {
		t.Fatal(err)
	}
	old, had := os.LookupEnv("ComSpec")
	defer func() {
		if had {
			_ = os.Setenv("ComSpec", old)
		} else {
			_ = os.Unsetenv("ComSpec")
		}
	}()

	_ = os.Unsetenv("ComSpec")
	if mismatch, _, _ := ComSpecMismatch(); mismatch {
		t.Fatal("unset ComSpec reported mismatch")
	}
	_ = os.Setenv("ComSpec", systemCmd)
	if mismatch, _, _ := ComSpecMismatch(); mismatch {
		t.Fatal("matching ComSpec reported mismatch")
	}
	fake := filepath.Join(t.TempDir(), "cmd.exe")
	if err := os.WriteFile(fake, []byte("not an interpreter"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("ComSpec", fake)
	if mismatch, expected, actual := ComSpecMismatch(); !mismatch || expected != systemCmd || actual != fake {
		t.Fatalf("fake ComSpec mismatch=%v expected=%q actual=%q", mismatch, expected, actual)
	}
}

func TestIsWindowsAppAlias(t *testing.T) {
	if !IsWindowsAppAlias(`C:\Users\x\AppData\Local\Microsoft\WindowsApps\python.exe`) {
		t.Fatal("expected WindowsApps alias")
	}
	if IsWindowsAppAlias(`C:\Python313\python.exe`) {
		t.Fatal("unexpected alias")
	}
}

func TestRunBatchFixtures(t *testing.T) {
	base := t.TempDir()

	version := filepath.Join(base, "version.cmd")
	writeBatch(t, version, "@echo off", "echo npm 10.0.0")
	res := Run(context.Background(), version, []string{"--version"}, true, 3*time.Second)
	t.Logf("version: exit=%d timedout=%v code=%q stdout=%q stderr=%q", res.ExitCode, res.TimedOut, res.ErrorCode, res.Stdout, res.Stderr)
	if res.ErrorCode != "" || res.ExitCode != 0 || !strings.Contains(res.Stdout, "npm 10.0.0") {
		t.Fatalf("version fixture failed: %+v", res)
	}

	stderrFixture := filepath.Join(base, "stderr.cmd")
	writeBatch(t, stderrFixture, "@echo off", ">&2 echo npm 10.0.0")
	res = Run(context.Background(), stderrFixture, []string{"--version"}, true, 3*time.Second)
	if res.ErrorCode != "" || res.ExitCode != 0 || !strings.Contains(res.Stderr, "npm 10.0.0") {
		t.Fatalf("stderr fixture failed: %+v", res)
	}

	exit7 := filepath.Join(base, "exit7.cmd")
	writeBatch(t, exit7, "@echo off", "exit /b 7")
	res = Run(context.Background(), exit7, []string{"--version"}, true, 3*time.Second)
	if res.ExitCode != 7 || res.ErrorCode != "exit" {
		t.Fatalf("exit7 fixture failed: %+v", res)
	}

	spaces := filepath.Join(base, "dir with spaces (x86)", "space.cmd")
	writeBatch(t, spaces, "@echo off", "echo spaces ok")
	res = Run(context.Background(), spaces, []string{"--version"}, true, 3*time.Second)
	if res.ErrorCode != "" || res.ExitCode != 0 || !strings.Contains(res.Stdout, "spaces ok") {
		t.Fatalf("spaces fixture failed: %+v", res)
	}

	timeoutFixture := filepath.Join(base, "timeout.cmd")
	writeBatch(t, timeoutFixture, "@echo off", "ping -n 10 127.0.0.1 >nul")
	res = Run(context.Background(), timeoutFixture, []string{"--version"}, true, time.Second)
	if !res.TimedOut || res.ErrorCode != "timeout" {
		t.Fatalf("timeout fixture failed: %+v", res)
	}
	t.Logf("timeout: exit=%d timedout=%v code=%q duration=%s", res.ExitCode, res.TimedOut, res.ErrorCode, res.Duration)
}

func TestRunBoundedOutput(t *testing.T) {
	base := t.TempDir()
	big := filepath.Join(base, "big.cmd")
	writeBatch(t, big,
		"@echo off",
		"for /L %%i in (1,1,3000) do @echo xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	)
	res := Run(context.Background(), big, []string{"--version"}, true, 10*time.Second)
	t.Logf("bounded: exit=%d timedout=%v code=%q stdout_bytes=%d stderr_bytes=%d", res.ExitCode, res.TimedOut, res.ErrorCode, len(res.Stdout), len(res.Stderr))
	if res.TimedOut {
		t.Fatal("output fixture timed out")
	}
	if len(res.Stdout) > StdoutLimit {
		t.Fatalf("stdout retention %d exceeds %d", len(res.Stdout), StdoutLimit)
	}
	if len(res.Stdout) < 1000 {
		t.Fatalf("expected substantial retained output, got %d", len(res.Stdout))
	}
}

func TestRunUsesExactBatchCandidate(t *testing.T) {
	base := t.TempDir()
	aDir := filepath.Join(base, "A")
	bDir := filepath.Join(base, "B")
	aMarker := filepath.Join(aDir, "a_marker.txt")
	bMarker := filepath.Join(bDir, "b_marker.txt")
	aBatch := filepath.Join(aDir, "same.cmd")
	bBatch := filepath.Join(bDir, "same.cmd")
	writeBatch(t, aBatch, "@echo off", `echo A> "%~dp0a_marker.txt"`)
	writeBatch(t, bBatch, "@echo off", `echo B> "%~dp0b_marker.txt"`)

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(bDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	res := Run(context.Background(), aBatch, []string{"--version"}, true, 3*time.Second)
	if res.ErrorCode != "" || res.ExitCode != 0 {
		t.Fatalf("run failed: %+v", res)
	}
	if _, err := os.Stat(aMarker); err != nil {
		t.Fatalf("A marker missing: %v", err)
	}
	if _, err := os.Stat(bMarker); err == nil {
		t.Fatal("B marker exists; wrong batch executed")
	} else if !os.IsNotExist(err) {
		t.Fatalf("B marker stat: %v", err)
	}
}

func TestRunIgnoresFakeComSpec(t *testing.T) {
	systemCmd, err := SystemCmdPath()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	batchPath := filepath.Join(base, "version.cmd")
	writeBatch(t, batchPath, "@echo off", "echo npm 10.0.0")
	fake := filepath.Join(base, "cmd.exe")
	if err := os.WriteFile(fake, []byte("not an executable interpreter"), 0o600); err != nil {
		t.Fatal(err)
	}

	old, had := os.LookupEnv("ComSpec")
	defer func() {
		if had {
			_ = os.Setenv("ComSpec", old)
		} else {
			_ = os.Unsetenv("ComSpec")
		}
	}()
	if err := os.Setenv("ComSpec", fake); err != nil {
		t.Fatal(err)
	}
	mismatch, expected, actual := ComSpecMismatch()
	if !mismatch || expected != systemCmd || actual != fake {
		t.Fatalf("mismatch=%v expected=%q actual=%q", mismatch, expected, actual)
	}
	res := Run(context.Background(), batchPath, []string{"--version"}, true, 3*time.Second)
	if res.ErrorCode != "" || res.ExitCode != 0 || !strings.Contains(res.Stdout, "npm 10.0.0") {
		t.Fatalf("system interpreter was not used: %+v", res)
	}
	t.Logf("fake_comspec_ignored=true system_cmd=%q", res.Interpreter)
}

func TestTimeoutLeavesDescendantRunning(t *testing.T) {
	base := t.TempDir()
	marker := filepath.Join(base, "child_marker.txt")
	worker := filepath.Join(base, "worker.cmd")
	writeBatch(t, worker,
		"@echo off",
		"ping -n 3 127.0.0.1 >nul",
		`echo survived> "%~dp0child_marker.txt"`,
	)
	parent := filepath.Join(base, "parent.cmd")
	writeBatch(t, parent,
		"@echo off",
		`start "" /b "`+worker+`"`,
		"ping -n 10 127.0.0.1 >nul",
	)

	res := Run(context.Background(), parent, []string{"--version"}, true, time.Second)
	if !res.TimedOut {
		t.Fatalf("parent did not time out: %+v", res)
	}
	time.Sleep(5 * time.Second)
	if _, err := os.Stat(marker); err != nil {
		t.Logf("descendant did not survive on this run: %v", err)
	} else {
		t.Logf("descendant_survived=true marker=%q", marker)
	}
}
