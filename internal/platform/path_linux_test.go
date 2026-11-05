package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeScript(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestFindCandidatesLinux(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go")
	writeScript(t, path, "#!/bin/sh\nexit 0\n", 0o644)
	if got := FindCandidates(dir, "go"); len(got) != 1 || got[0].Executable {
		t.Fatalf("non-executable candidate = %+v", got)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindCandidates(dir, "go"); len(got) != 1 || !got[0].Executable || got[0].Path != path {
		t.Fatalf("candidate = %+v", got)
	}
}

func TestSamePathLinux(t *testing.T) {
	if SamePath("/tmp/Go", "/tmp/go") {
		t.Fatal("Linux path comparison must be case-sensitive")
	}
}

func TestInspectEntrySearchable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can search directories without execute bits")
	}
	dir := t.TempDir()
	if info := InspectEntry(dir); info.Searchable == nil || !*info.Searchable {
		t.Fatalf("expected searchable dir: %+v", info)
	}
	if err := os.Chmod(dir, 0o644); err != nil {
		t.Fatal(err)
	}
	if info := InspectEntry(dir); info.Searchable == nil || *info.Searchable {
		t.Fatalf("expected non-searchable dir: %+v", info)
	}
}

func TestSplitListPreservesEmpty(t *testing.T) {
	entries := SplitList("a::b")
	if len(entries) != 3 || entries[0] != "a" || entries[1] != "" || entries[2] != "b" {
		t.Fatalf("SplitList = %#v", entries)
	}
}

func TestRunScriptAndTimeout(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.sh")
	writeScript(t, ok, "#!/bin/sh\nprintf 'go version go1.99.0 linux/amd64\\n'\n", 0o755)
	res := Run(context.Background(), ok, []string{"version"}, false, 3*time.Second)
	if res.ErrorCode != "" || !strings.Contains(res.Stdout, "go1.99.0") {
		t.Fatalf("script result = %+v", res)
	}

	slow := filepath.Join(dir, "slow.sh")
	writeScript(t, slow, "#!/bin/sh\nsleep 10\n", 0o755)
	res = Run(context.Background(), slow, nil, false, time.Second)
	if !res.TimedOut || res.ErrorCode != "timeout" {
		t.Fatalf("timeout result = %+v", res)
	}
}

func TestRunBoundedOutputLinux(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big.sh")
	writeScript(t, big, "#!/bin/sh\ni=0\nwhile [ $i -lt 4000 ]; do echo xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx; i=$((i+1)); done\n", 0o755)
	res := Run(context.Background(), big, nil, false, 10*time.Second)
	t.Logf("bounded: exit=%d timedout=%v code=%q stdout_bytes=%d", res.ExitCode, res.TimedOut, res.ErrorCode, len(res.Stdout))
	if res.TimedOut {
		t.Fatal("output fixture timed out")
	}
	if len(res.Stdout) > StdoutLimit {
		t.Fatalf("stdout retention %d exceeds %d", len(res.Stdout), StdoutLimit)
	}
	if len(res.Stdout) < 1000 {
		t.Fatalf("expected retained output, got %d", len(res.Stdout))
	}
}
