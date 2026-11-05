package pathcheck

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLinuxSearchabilityAndInaccessible(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux permission semantics")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can bypass permission checks")
	}

	base := t.TempDir()
	restricted := filepath.Join(base, "restricted")
	if err := os.Mkdir(restricted, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(restricted, "bin")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(restricted, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(restricted, 0o755) }()

	analysis := Analyze(pathValue(child), true, testSpecs(), runtime.GOOS)
	ids := findingIDs(analysis.Findings)
	if !ids["PATH_DIRECTORY_INACCESSIBLE"] {
		t.Fatalf("missing inaccessible finding: %+v", analysis.Findings)
	}

	unsearchable := filepath.Join(base, "unsearchable")
	if err := os.Mkdir(unsearchable, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unsearchable, 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(unsearchable, 0o755) }()
	analysis = Analyze(pathValue(unsearchable), true, testSpecs(), runtime.GOOS)
	ids = findingIDs(analysis.Findings)
	if !ids["PATH_DIRECTORY_NOT_SEARCHABLE"] {
		t.Fatalf("missing not-searchable finding: %+v", analysis.Findings)
	}
}
