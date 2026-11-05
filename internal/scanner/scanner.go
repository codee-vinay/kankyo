package scanner

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/codee-vinay/kankyo/internal/diagnostics"
	"github.com/codee-vinay/kankyo/internal/pathcheck"
	"github.com/codee-vinay/kankyo/internal/platform"
	"github.com/codee-vinay/kankyo/internal/tools"
)

type ToolStatus string

const (
	ToolOK          ToolStatus = "ok"
	ToolWarning     ToolStatus = "warning"
	ToolErrorStatus ToolStatus = "error"
	ToolInfo        ToolStatus = "info"
	ToolSkipped     ToolStatus = "skipped"
)

type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ToolResult struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Command     string     `json:"command"`
	Status      ToolStatus `json:"status"`
	Found       bool       `json:"found"`
	Path        string     `json:"path,omitempty"`
	Version     string     `json:"version,omitempty"`
	ExecutionOK *bool      `json:"execution_ok"`
	Multiple    bool       `json:"multiple"`
	Candidates  []string   `json:"candidates"`
	Error       *ToolError `json:"error,omitempty"`
}

type Options struct {
	Goos     string
	Timeout  time.Duration
	LookPath func(string) (string, error)
	Run      func(context.Context, string, []string, bool, time.Duration) platform.RunResult
}

func Check(ctx context.Context, specs []tools.Spec, analysis pathcheck.Analysis, opts Options) ([]ToolResult, []diagnostics.Finding) {
	if opts.Goos == "" {
		opts.Goos = platform.Current().OS
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 3 * time.Second
	}
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}
	if opts.Run == nil {
		opts.Run = platform.Run
	}

	byTool := make(map[string][]pathcheck.Candidate)
	for _, c := range analysis.Candidates {
		byTool[c.ToolID] = append(byTool[c.ToolID], c)
	}

	var findings []diagnostics.Finding
	if opts.Goos == "windows" {
		if mismatch, expected, actual := platform.ComSpecMismatch(); mismatch {
			findings = append(findings, diagnostics.Make(
				"PLATFORM_COMSPEC_MISMATCH",
				"",
				diagnostics.Evidence{Kind: "environment", Value: "ComSpec", Detail: "actual=" + actual},
				diagnostics.Evidence{Kind: "file", Value: expected, Detail: "validated system interpreter"},
			))
		}
	}

	results := make([]ToolResult, 0, len(specs))
	for _, spec := range specs {
		result, toolFindings := checkTool(ctx, spec, byTool[spec.ID], opts)
		results = append(results, result)
		findings = append(findings, toolFindings...)
	}

	findings = diagnostics.Sort(diagnostics.Dedupe(findings))
	return results, findings
}

func checkTool(ctx context.Context, spec tools.Spec, candidates []pathcheck.Candidate, opts Options) (ToolResult, []diagnostics.Finding) {
	executable := make([]pathcheck.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Executable {
			executable = append(executable, candidate)
		}
	}
	result := ToolResult{
		ID:         spec.ID,
		Name:       spec.Name,
		Command:    firstCommand(spec, opts.Goos),
		Status:     ToolInfo,
		Found:      false,
		Multiple:   len(executable) > 1,
		Candidates: candidatePaths(candidates),
	}
	if len(candidates) == 0 {
		return result, []diagnostics.Finding{diagnostics.Make(
			"TOOL_NOT_FOUND",
			"",
			diagnostics.Evidence{Kind: "command", Value: firstCommand(spec, opts.Goos)},
		)}
	}
	if len(executable) == 0 {
		result.Found = true
		result.Path = candidates[0].Path
		result.Command = candidates[0].Command
		result.Status = ToolErrorStatus
		result.Error = &ToolError{Code: "not_executable", Message: "candidate has no executable permission"}
		return result, []diagnostics.Finding{diagnostics.Make(
			"TOOL_NOT_EXECUTABLE",
			"",
			diagnostics.Evidence{Kind: "file", Value: result.Path, Detail: "no executable permission"},
		)}
	}

	selected, command, issue, detail := chooseCandidate(executable, opts.LookPath)
	if issue != candidateOK {
		result.Command = command
		var findings []diagnostics.Finding
		switch issue {
		case candidateCWD:
			result.Status = ToolSkipped
			findings = append(findings, diagnostics.Make(
				"TOOL_CWD_RESOLUTION",
				"",
				diagnostics.Evidence{Kind: "command", Value: command, Detail: detail},
			))
		case candidateLookupError:
			result.Status = ToolErrorStatus
			result.Error = &ToolError{Code: "lookup_error", Message: detail}
			findings = append(findings, diagnostics.Make(
				"TOOL_EXECUTION_UNSUPPORTED",
				"",
				diagnostics.Evidence{Kind: "command", Value: command},
				diagnostics.Evidence{Kind: "error", Value: detail},
			))
		case candidateMismatch:
			result.Status = ToolErrorStatus
			result.Error = &ToolError{Code: "lookup_mismatch", Message: detail}
			findings = append(findings, diagnostics.Make(
				"TOOL_EXECUTION_UNSUPPORTED",
				"",
				diagnostics.Evidence{Kind: "command", Value: command, Detail: detail},
			))
		default:
			result.Status = ToolErrorStatus
			result.Error = &ToolError{Code: "no_verified_candidate", Message: detail}
			findings = append(findings, diagnostics.Make(
				"TOOL_EXECUTION_UNSUPPORTED",
				"",
				diagnostics.Evidence{Kind: "command", Value: command, Detail: detail},
			))
		}
		return result, findings
	}

	result.Command = command
	result.Path = selected.Path
	result.Found = true

	var findings []diagnostics.Finding
	if platform.IsWindowsAppAlias(result.Path) {
		result.Status = ToolSkipped
		findings = append(findings, diagnostics.Make(
			"TOOL_EXECUTION_SKIPPED",
			"",
			diagnostics.Evidence{Kind: "file", Value: result.Path, Detail: "Windows app execution alias"},
		))
		return result, findings
	}

	res := opts.Run(ctx, result.Path, spec.VersionArgs, platform.IsBatchFile(result.Path), opts.Timeout)
	executionOK := res.ErrorCode == ""
	result.ExecutionOK = &executionOK

	if res.ErrorCode != "" {
		result.Status = ToolErrorStatus
		result.Error = &ToolError{Code: res.ErrorCode, Message: res.ErrorMessage}
		findings = append(findings, executionFailureFinding(spec, result, res))
		return result, findings
	}

	result.Version = tools.ParseVersion(spec, res.Stdout, res.Stderr)
	switch {
	case result.Version == "":
		result.Status = ToolWarning
		findings = append(findings, diagnostics.Make(
			"TOOL_VERSION_UNPARSED",
			"",
			diagnostics.Evidence{Kind: "command", Value: result.Command, Detail: "no parseable version"},
		))
	case result.Multiple:
		result.Status = ToolWarning
	default:
		result.Status = ToolOK
	}
	return result, findings
}

type candidateIssue string

const (
	candidateOK          candidateIssue = ""
	candidateCWD         candidateIssue = "cwd"
	candidateMismatch    candidateIssue = "mismatch"
	candidateLookupError candidateIssue = "lookup_error"
	candidateNoVerified  candidateIssue = "no_verified_candidate"
)

// chooseCandidate accepts a candidate only when LookPath resolves to that
// exact absolute path. It never falls back to an unverified candidate.
func chooseCandidate(candidates []pathcheck.Candidate, lookPath func(string) (string, error)) (pathcheck.Candidate, string, candidateIssue, string) {
	command := firstCandidateCommand(candidates)
	cwdIssue := false
	mismatch := false
	var lookupErr error

	for _, c := range candidates {
		if !filepath.IsAbs(c.Path) {
			cwdIssue = true
			continue
		}
		if command == "" {
			command = c.Command
		}
		path, err := lookPath(c.Command)
		if err != nil {
			if errors.Is(err, exec.ErrDot) {
				cwdIssue = true
			} else {
				lookupErr = err
			}
			continue
		}
		if !filepath.IsAbs(path) {
			cwdIssue = true
			continue
		}
		if platform.SamePath(path, c.Path) {
			return c, c.Command, candidateOK, ""
		}
		mismatch = true
	}

	switch {
	case cwdIssue:
		return pathcheck.Candidate{}, command, candidateCWD, "resolution is relative to the current directory"
	case mismatch:
		return pathcheck.Candidate{}, command, candidateMismatch, "LookPath resolved to a different executable path"
	case lookupErr != nil:
		return pathcheck.Candidate{}, command, candidateLookupError, lookupErr.Error()
	default:
		return pathcheck.Candidate{}, command, candidateNoVerified, "no candidate matched the resolved executable path"
	}
}

func executionFailureFinding(spec tools.Spec, result ToolResult, res platform.RunResult) diagnostics.Finding {
	evidence := []diagnostics.Evidence{
		{Kind: "command", Value: result.Command},
		{Kind: "file", Value: result.Path},
	}
	if res.ErrorMessage != "" {
		evidence = append(evidence, diagnostics.Evidence{Kind: "error", Value: res.ErrorMessage})
	}
	switch {
	case res.ErrorCode == "timeout" || res.ErrorCode == "output_drain_timeout":
		return diagnostics.Make("TOOL_TIMEOUT", "", evidence...)
	case res.ErrorCode == "exit":
		return diagnostics.Make("TOOL_EXECUTION_FAILED", "", evidence...)
	case res.ErrorCode == "prepare":
		return diagnostics.Make("TOOL_EXECUTION_UNSUPPORTED", "", evidence...)
	case strings.Contains(strings.ToLower(res.ErrorMessage), "permission"):
		return diagnostics.Make("TOOL_PERMISSION_DENIED", "", evidence...)
	default:
		return diagnostics.Make("TOOL_EXECUTION_FAILED", "", evidence...)
	}
}

func candidatePaths(candidates []pathcheck.Candidate) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.Path)
	}
	return out
}

func firstCommand(spec tools.Spec, goos string) string {
	commands := spec.Commands(goos)
	if len(commands) == 0 {
		return spec.ID
	}
	return commands[0]
}

func firstCandidateCommand(candidates []pathcheck.Candidate) string {
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0].Command
}
