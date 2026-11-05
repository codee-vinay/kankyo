package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/codee-vinay/kankyo/internal/diagnostics"
	"github.com/codee-vinay/kankyo/internal/pathcheck"
	"github.com/codee-vinay/kankyo/internal/platform"
	"github.com/codee-vinay/kankyo/internal/report"
	"github.com/codee-vinay/kankyo/internal/scanner"
)

func sample() report.Report {
	ok := true
	tools := []scanner.ToolResult{{
		ID: "go", Name: "Go", Command: "go", Status: scanner.ToolOK,
		Found: true, Path: `C:\Go\bin\go.exe`, Version: "1.27.3",
		ExecutionOK: &ok, Candidates: []string{`C:\Go\bin\go.exe`},
	}}
	analysis := pathcheck.Analysis{
		Entries: []pathcheck.Entry{{Index: 0, Raw: `C:\Go\bin`, Exists: true, IsDir: true, Accessible: true}},
	}
	findings := []diagnostics.Finding{diagnostics.Make(
		"PATH_DUPLICATE_ENTRY", "",
		diagnostics.Evidence{Kind: "path_entry", Value: `C:\Go\bin`, Detail: "index 0"},
	)}
	return report.Build(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "0.1.0", platform.Info{OS: "windows", Arch: "amd64"}, tools, analysis, findings)
}

func TestRenderSectionsAndMonochrome(t *testing.T) {
	var buf bytes.Buffer
	err := Report(&buf, sample(), Options{Width: 80, ShowTools: true, ShowPath: true})
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"KANKYŌ", "Platform", "Developer Tools", "PATH Analysis", "Detected Issues", "Recommendations", "Summary", "[OK]", "[WARN]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatal("monochrome output contains ANSI escapes")
	}
}

func TestRenderColorAndVerbose(t *testing.T) {
	var buf bytes.Buffer
	err := Report(&buf, sample(), Options{Width: 80, Color: true, Verbose: true, ShowTools: true, ShowPath: true})
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "\x1b[") {
		t.Fatal("color output missing ANSI escapes")
	}
	if !strings.Contains(out, "Evidence:") {
		t.Fatalf("verbose output missing evidence:\n%s", out)
	}
}

func TestRenderHidesUnselectedSections(t *testing.T) {
	var buf bytes.Buffer
	if err := Report(&buf, sample(), Options{Width: 80, ShowTools: true, ShowPath: false}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "PATH Analysis") {
		t.Fatalf("PATH section shown when disabled:\n%s", out)
	}
}

func TestWrap(t *testing.T) {
	lines := wrap("one two three four", 9)
	if len(lines) < 2 {
		t.Fatalf("wrap did not split: %v", lines)
	}
}
