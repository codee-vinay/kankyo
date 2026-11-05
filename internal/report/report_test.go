package report

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codee-vinay/kankyo/internal/diagnostics"
	"github.com/codee-vinay/kankyo/internal/pathcheck"
	"github.com/codee-vinay/kankyo/internal/platform"
	"github.com/codee-vinay/kankyo/internal/redact"
	"github.com/codee-vinay/kankyo/internal/scanner"
)

func sampleReport() Report {
	ok := true
	tools := []scanner.ToolResult{{
		ID:          "go",
		Name:        "Go",
		Command:     "go",
		Status:      scanner.ToolOK,
		Found:       true,
		Path:        `C:\Users\alice\bin\go.exe`,
		Version:     "1.27.3",
		ExecutionOK: &ok,
		Candidates:  []string{`C:\Users\alice\bin\go.exe`},
	}}
	analysis := pathcheck.Analysis{
		Entries: []pathcheck.Entry{{
			Index: 0, Raw: `C:\Users\alice\bin`, Normalized: `C:\Users\alice\bin`,
			Exists: true, IsDir: true, Accessible: true,
		}},
		Counts: pathcheck.Counts{},
	}
	findings := []diagnostics.Finding{
		diagnostics.Make("PATH_DUPLICATE_ENTRY", "", diagnostics.Evidence{Kind: "path_entry", Value: `C:\Users\alice\bin`}),
	}
	return Build(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC), "0.1.0", platform.Info{OS: "windows", Arch: "amd64"}, tools, analysis, findings)
}

func TestBuildAndEncode(t *testing.T) {
	rep := sampleReport()
	if rep.SchemaVersion != "1.0" || rep.GeneratedAt != "2026-11-04T12:00:00Z" {
		t.Fatalf("metadata: %+v", rep)
	}
	if rep.Summary.ToolsChecked != 1 || rep.Summary.ToolsDetected != 1 || rep.Summary.Warnings != 1 {
		t.Fatalf("summary: %+v", rep.Summary)
	}
	data, err := Encode(rep)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"schema_version", "kankyo_version", "generated_at", "platform", "tools", "path", "findings", "summary"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}
}

func TestApplyRedaction(t *testing.T) {
	rep := sampleReport()
	ApplyRedaction(&rep, redact.New(`C:\Users\alice`, "alice", "windows"))
	data, err := Encode(rep)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "alice") || strings.Contains(text, `C:\Users`) {
		t.Fatalf("redaction failed: %s", text)
	}
	if !rep.Redacted {
		t.Fatal("Redacted flag not set")
	}
}

func TestApplyRedactionVersion(t *testing.T) {
	rep := sampleReport()
	rep.Tools[0].Version = `C:\Users\alice\sdk\1.27.3`
	ApplyRedaction(&rep, redact.New(`C:\Users\alice`, "alice", "windows"))
	data, err := Encode(rep)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if text := string(data); strings.Contains(text, "alice") || strings.Contains(text, `C:\Users`) {
		t.Fatalf("version redaction failed: %s", text)
	}
	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got, want := decoded.Tools[0].Version, `<HOME>\sdk\1.27.3`; got != want {
		t.Fatalf("redacted version = %q, want %q", got, want)
	}

	plain := sampleReport()
	plain.Tools[0].Version = "1.27.3"
	ApplyRedaction(&plain, redact.New(`C:\Users\alice`, "alice", "windows"))
	if plain.Tools[0].Version != "1.27.3" {
		t.Fatalf("ordinary version changed: %q", plain.Tools[0].Version)
	}
}

func TestWriteFileSuccessAndDirectoryRejection(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "report.json")
	data := []byte(`{"ok":true}`)
	if err := WriteFile(dest, data); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(data) {
		t.Fatalf("read back: %q err=%v", got, err)
	}
	if err := WriteFile(dir, data); err == nil {
		t.Fatal("directory destination accepted")
	}
}

func TestWriteFileInjectedRenameFailure(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "report.json")
	original := []byte("original")
	if err := os.WriteFile(dest, original, 0o600); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected")
	err := writeFileAtomic(dest, []byte("new"), func(_, _ string) error { return injected })
	if err == nil || !strings.Contains(err.Error(), injected.Error()) {
		t.Fatalf("expected injected failure, got %v", err)
	}
	got, readErr := os.ReadFile(dest)
	if readErr != nil || string(got) != string(original) {
		t.Fatalf("original not preserved: %q err=%v", got, readErr)
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".kankyo-report-*.tmp"))
	if len(leftovers) != 0 {
		t.Fatalf("temp leftovers: %v", leftovers)
	}
}

func TestWriteFileSymlinkRejection(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := WriteFile(link, []byte("new")); err == nil {
		t.Fatal("symlink destination accepted")
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "target" {
		t.Fatalf("target changed: %q err=%v", got, err)
	}
}
