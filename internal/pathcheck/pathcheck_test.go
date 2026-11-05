package pathcheck

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/codee-vinay/kankyo/internal/diagnostics"
	"github.com/codee-vinay/kankyo/internal/tools"
)

func testSpecs() []tools.Spec {
	return []tools.Spec{{
		ID:              "go",
		Name:            "Go",
		WindowsCommands: []string{"go"},
		UnixCommands:    []string{"go"},
	}}
}

func makeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".exe")
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func pathValue(entries ...string) string {
	return strings.Join(entries, string(os.PathListSeparator))
}

func TestAnalyzeDuplicateEmptyMissingRelative(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".EXE")
	}
	base := t.TempDir()
	dir1 := filepath.Join(base, "bin1")
	dir2 := filepath.Join(base, "bin2")
	makeExecutable(t, dir1, "go")
	makeExecutable(t, dir2, "go")
	missing := filepath.Join(base, "missing")
	raw := pathValue(dir1, dir1, missing, "relative", "", dir2)

	analysis := Analyze(raw, true, testSpecs(), runtime.GOOS)
	if analysis.Counts.Duplicates < 1 || analysis.Counts.Missing < 1 || analysis.Counts.Empty != 1 || analysis.Counts.Relative != 1 {
		t.Fatalf("counts = %+v", analysis.Counts)
	}
	ids := findingIDs(analysis.Findings)
	for _, want := range []string{"PATH_DUPLICATE_ENTRY", "PATH_DIRECTORY_NOT_FOUND", "PATH_RELATIVE_ENTRY", "PATH_EMPTY_ENTRY"} {
		if !ids[want] {
			t.Errorf("missing finding %s in %v", want, ids)
		}
	}
	if len(analysis.Candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(analysis.Candidates))
	}
	if !ids["PATH_MULTIPLE_CANDIDATES"] {
		t.Errorf("missing multiple-candidates finding: %v", ids)
	}
}

func TestDuplicateFindingAggregatesEvidence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".EXE")
	}
	base := t.TempDir()
	dir := filepath.Join(base, "bin")
	makeExecutable(t, dir, "go")
	analysis := Analyze(pathValue(dir, dir, dir), true, testSpecs(), runtime.GOOS)
	var duplicates int
	for _, finding := range analysis.Findings {
		if finding.ID == "PATH_DUPLICATE_ENTRY" {
			duplicates++
			if len(finding.Evidence) < 2 {
				t.Fatalf("duplicate finding evidence = %d", len(finding.Evidence))
			}
		}
	}
	if duplicates != 1 {
		t.Fatalf("duplicate findings = %d, want 1", duplicates)
	}
}

func TestPathUnavailable(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		present bool
	}{
		{"", false},
		{"", true},
	} {
		analysis := Analyze(tc.raw, tc.present, testSpecs(), runtime.GOOS)
		if len(analysis.Entries) != 0 || len(analysis.Findings) != 1 || analysis.Findings[0].ID != "PATH_UNAVAILABLE" {
			t.Fatalf("unavailable result: %+v", analysis)
		}
	}
}

func TestEntryNotDirectoryAndEnvReference(t *testing.T) {
	base := t.TempDir()
	file := filepath.Join(base, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	envRef := "$HOME/bin"
	if runtime.GOOS == "windows" {
		envRef = "%USERPROFILE%\\bin"
	}
	analysis := Analyze(pathValue(file, envRef), true, testSpecs(), runtime.GOOS)
	ids := findingIDs(analysis.Findings)
	if !ids["PATH_ENTRY_NOT_DIRECTORY"] {
		t.Errorf("missing not-directory finding: %v", ids)
	}
	if !ids["PATH_ENV_VAR_NOT_EXPANDED"] {
		t.Errorf("missing env-reference finding: %v", ids)
	}
}

func findingIDs(findings []diagnostics.Finding) map[string]bool {
	out := make(map[string]bool)
	for _, f := range findings {
		out[f.ID] = true
	}
	return out
}
