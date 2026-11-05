package scanner

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/codee-vinay/kankyo/internal/diagnostics"
	"github.com/codee-vinay/kankyo/internal/pathcheck"
	"github.com/codee-vinay/kankyo/internal/platform"
	"github.com/codee-vinay/kankyo/internal/tools"
)

func goSpec() tools.Spec {
	spec, _ := tools.FindSpec("go")
	return spec
}

func candidate(path string) pathcheck.Candidate {
	return pathcheck.Candidate{ToolID: "go", Command: "go", Path: path, EntryIndex: 0, Executable: true}
}

func testOptions(path string, run func(context.Context, string, []string, bool, time.Duration) platform.RunResult) Options {
	return Options{
		Goos:    "windows",
		Timeout: time.Second,
		LookPath: func(string) (string, error) {
			return path, nil
		},
		Run: run,
	}
}

func TestCheckRelativeCandidateRejected(t *testing.T) {
	relative := filepath.Join("relative", "go.exe")
	lookedUp := false
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(relative)}},
		Options{
			Goos:    "windows",
			Timeout: time.Second,
			LookPath: func(string) (string, error) {
				lookedUp = true
				return "", errors.New("relative candidate must not be looked up")
			},
			Run: func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
				t.Fatal("Run must not be called for a relative candidate")
				return platform.RunResult{}
			},
		})
	if lookedUp {
		t.Fatal("LookPath was called for a relative candidate")
	}
	if results[0].Status != ToolSkipped || results[0].Found || results[0].Path != "" {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_CWD_RESOLUTION") {
		t.Fatalf("missing relative-candidate finding: %+v", findings)
	}
}

func TestCheckLookPathRelativeResultRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.exe")
	lookedUp := false
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(path)}},
		Options{
			Goos:    "windows",
			Timeout: time.Second,
			LookPath: func(string) (string, error) {
				lookedUp = true
				return filepath.Join("relative", "go.exe"), nil
			},
			Run: func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
				t.Fatal("Run must not be called for a non-absolute LookPath result")
				return platform.RunResult{}
			},
		})
	if !lookedUp {
		t.Fatal("LookPath was not called")
	}
	if results[0].Status != ToolSkipped || results[0].Found || results[0].Path != "" {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_CWD_RESOLUTION") {
		t.Fatalf("missing relative-lookup finding: %+v", findings)
	}
}

func TestCheckNoVerifiedCandidate(t *testing.T) {
	first := filepath.Join(t.TempDir(), "go.exe")
	second := filepath.Join(t.TempDir(), "go.exe")
	other := filepath.Join(t.TempDir(), "other.exe")
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(first), candidate(second)}},
		Options{
			Goos:    "windows",
			Timeout: time.Second,
			LookPath: func(string) (string, error) {
				return other, nil
			},
			Run: func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
				t.Fatal("Run must not be called when no candidate matches LookPath")
				return platform.RunResult{}
			},
		})
	if results[0].Status != ToolErrorStatus || results[0].Error == nil || results[0].Error.Code != "lookup_mismatch" {
		t.Fatalf("result = %+v", results[0])
	}
	if results[0].Path != "" || results[0].Found {
		t.Fatalf("unexpected candidate selection: %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_EXECUTION_UNSUPPORTED") {
		t.Fatalf("missing no-verified-candidate finding: %+v", findings)
	}
}

func TestCheckLookPathDifferentPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.exe")
	other := filepath.Join(t.TempDir(), "other.exe")
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(path)}},
		Options{
			Goos:    "windows",
			Timeout: time.Second,
			LookPath: func(string) (string, error) {
				return other, nil
			},
			Run: func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
				t.Fatal("Run must not be called for a mismatched candidate")
				return platform.RunResult{}
			},
		})
	if results[0].Status != ToolErrorStatus || results[0].Error == nil || results[0].Error.Code != "lookup_mismatch" {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_EXECUTION_UNSUPPORTED") {
		t.Fatalf("missing mismatch finding: %+v", findings)
	}
}

func TestCheckLookPathUnexpectedError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.exe")
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(path)}},
		Options{
			Goos:    "windows",
			Timeout: time.Second,
			LookPath: func(string) (string, error) {
				return "", errors.New("lookup failed")
			},
			Run: func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
				t.Fatal("Run must not be called after an unexpected lookup error")
				return platform.RunResult{}
			},
		})
	if results[0].Status != ToolErrorStatus || results[0].Error == nil || results[0].Error.Code != "lookup_error" {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_EXECUTION_UNSUPPORTED") {
		t.Fatalf("missing lookup-error finding: %+v", findings)
	}
}

func TestCheckMultipleCandidatesUsesResolvedCandidate(t *testing.T) {
	first := filepath.Join(t.TempDir(), "go.exe")
	second := filepath.Join(t.TempDir(), "go.exe")
	var ran string
	results, _ := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(first), candidate(second)}},
		Options{
			Goos:    "windows",
			Timeout: time.Second,
			LookPath: func(string) (string, error) {
				return second, nil
			},
			Run: func(_ context.Context, path string, _ []string, _ bool, _ time.Duration) platform.RunResult {
				ran = path
				return platform.RunResult{Stdout: "go version go1.27.3 windows/amd64\n"}
			},
		})
	if ran != second {
		t.Fatalf("ran %q, want exact resolved candidate %q", ran, second)
	}
	if results[0].Path != second || !results[0].Multiple || results[0].Status != ToolWarning {
		t.Fatalf("result = %+v", results[0])
	}
}

func TestCheckSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.exe")
	var ran string
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(path)}},
		testOptions(path, func(_ context.Context, gotPath string, _ []string, _ bool, _ time.Duration) platform.RunResult {
			ran = gotPath
			return platform.RunResult{Stdout: "go version go1.27.3 windows/amd64\n"}
		}))
	if ran != path {
		t.Fatalf("ran %q, want exact candidate %q", ran, path)
	}
	if len(results) != 1 || results[0].Status != ToolOK || results[0].Version != "1.27.3" {
		t.Fatalf("result = %+v", results)
	}
	if findings != nil && len(findings) != 0 {
		t.Fatalf("unexpected findings: %+v", findings)
	}
}

func TestCheckNotExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go")
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{{
			ToolID: "go", Command: "go", Path: path, EntryIndex: 0, Executable: false,
		}}},
		Options{Goos: "linux"})
	if results[0].Status != ToolErrorStatus || results[0].Error == nil {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_NOT_EXECUTABLE") {
		t.Fatalf("missing not-executable finding: %+v", findings)
	}
}

func TestCheckMissing(t *testing.T) {
	results, findings := Check(context.Background(), []tools.Spec{goSpec()}, pathcheck.Analysis{}, Options{Goos: "windows"})
	if len(results) != 1 || results[0].Found || results[0].Status != ToolInfo {
		t.Fatalf("result = %+v", results)
	}
	if !hasFinding(findings, "TOOL_NOT_FOUND") {
		t.Fatalf("missing TOOL_NOT_FOUND: %+v", findings)
	}
}

func TestCheckExecutionFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.exe")
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(path)}},
		testOptions(path, func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
			return platform.RunResult{ExitCode: 7, ErrorCode: "exit", ErrorMessage: "exit status 7"}
		}))
	if results[0].Status != ToolErrorStatus || results[0].Error == nil {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_EXECUTION_FAILED") {
		t.Fatalf("missing failure finding: %+v", findings)
	}
}

func TestCheckTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.exe")
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(path)}},
		testOptions(path, func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
			return platform.RunResult{ExitCode: -1, TimedOut: true, ErrorCode: "timeout", ErrorMessage: "timeout"}
		}))
	if results[0].Status != ToolErrorStatus {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_TIMEOUT") {
		t.Fatalf("missing timeout finding: %+v", findings)
	}
}

func TestCheckUnparsedAndMultiple(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.exe")
	second := filepath.Join(t.TempDir(), "go.exe")
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(path), candidate(second)}},
		testOptions(path, func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
			return platform.RunResult{Stdout: "nonsense\n"}
		}))
	if results[0].Status != ToolWarning || !results[0].Multiple {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_VERSION_UNPARSED") {
		t.Fatalf("missing unparsed finding: %+v", findings)
	}
}

func TestCheckCwdResolution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.exe")
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(path)}},
		Options{
			Goos:    "windows",
			Timeout: time.Second,
			LookPath: func(string) (string, error) {
				return "", exec.ErrDot
			},
			Run: func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
				t.Fatal("Run should not be called for ErrDot")
				return platform.RunResult{}
			},
		})
	if results[0].Status != ToolSkipped {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_CWD_RESOLUTION") {
		t.Fatalf("missing cwd finding: %+v", findings)
	}
}

func TestCheckStartError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.exe")
	results, findings := Check(context.Background(), []tools.Spec{goSpec()},
		pathcheck.Analysis{Candidates: []pathcheck.Candidate{candidate(path)}},
		testOptions(path, func(context.Context, string, []string, bool, time.Duration) platform.RunResult {
			return platform.RunResult{ExitCode: -1, ErrorCode: "start", ErrorMessage: "permission denied"}
		}))
	if results[0].Status != ToolErrorStatus {
		t.Fatalf("result = %+v", results[0])
	}
	if !hasFinding(findings, "TOOL_PERMISSION_DENIED") {
		t.Fatalf("missing permission finding: %+v", findings)
	}
}

func hasFinding(findings []diagnostics.Finding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}
