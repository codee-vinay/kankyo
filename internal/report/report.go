package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codee-vinay/kankyo/internal/diagnostics"
	"github.com/codee-vinay/kankyo/internal/pathcheck"
	"github.com/codee-vinay/kankyo/internal/platform"
	"github.com/codee-vinay/kankyo/internal/redact"
	"github.com/codee-vinay/kankyo/internal/scanner"
)

const SchemaVersion = "1.0"

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type PathReport struct {
	Source     string            `json:"source"`
	Separator  string            `json:"separator"`
	EntryCount int               `json:"entry_count"`
	Entries    []pathcheck.Entry `json:"entries"`
	Analysis   pathcheck.Counts  `json:"analysis"`
}

type Summary struct {
	ToolsChecked  int `json:"tools_checked"`
	ToolsDetected int `json:"tools_detected"`
	Info          int `json:"info"`
	Warnings      int `json:"warnings"`
	Errors        int `json:"errors"`
	Skipped       int `json:"skipped"`
}

type Report struct {
	SchemaVersion string                `json:"schema_version"`
	KankyoVersion string                `json:"kankyo_version"`
	GeneratedAt   string                `json:"generated_at"`
	Redacted      bool                  `json:"redacted"`
	Platform      Platform              `json:"platform"`
	Tools         []scanner.ToolResult  `json:"tools"`
	Path          PathReport            `json:"path"`
	Findings      []diagnostics.Finding `json:"findings"`
	Summary       Summary               `json:"summary"`
}

func Build(now time.Time, kankyoVersion string, info platform.Info, tools []scanner.ToolResult, analysis pathcheck.Analysis, findings []diagnostics.Finding) Report {
	if tools == nil {
		tools = []scanner.ToolResult{}
	}
	entries := analysis.Entries
	if entries == nil {
		entries = []pathcheck.Entry{}
	}
	findings = diagnostics.Sort(diagnostics.Dedupe(findings))
	if findings == nil {
		findings = []diagnostics.Finding{}
	}

	summary := Summary{ToolsChecked: len(tools)}
	for _, tool := range tools {
		if tool.Found {
			summary.ToolsDetected++
		}
		if tool.Status == scanner.ToolSkipped {
			summary.Skipped++
		}
	}
	for _, finding := range findings {
		switch finding.Severity {
		case diagnostics.Error:
			summary.Errors++
		case diagnostics.Warning:
			summary.Warnings++
		case diagnostics.Info:
			summary.Info++
		}
	}

	return Report{
		SchemaVersion: SchemaVersion,
		KankyoVersion: kankyoVersion,
		GeneratedAt:   now.UTC().Format(time.RFC3339Nano),
		Platform:      Platform{OS: info.OS, Arch: info.Arch},
		Tools:         tools,
		Path: PathReport{
			Source:     "process",
			Separator:  platform.PathListSeparator(),
			EntryCount: len(entries),
			Entries:    entries,
			Analysis:   analysis.Counts,
		},
		Findings: findings,
		Summary:  summary,
	}
}

func ApplyRedaction(r *Report, redactor *redact.Redactor) {
	if redactor == nil {
		return
	}
	for i := range r.Path.Entries {
		r.Path.Entries[i].Raw = redactor.String(r.Path.Entries[i].Raw)
		r.Path.Entries[i].Normalized = redactor.String(r.Path.Entries[i].Normalized)
		r.Path.Entries[i].Error = redactor.String(r.Path.Entries[i].Error)
	}
	for i := range r.Tools {
		r.Tools[i].Path = redactor.String(r.Tools[i].Path)
		r.Tools[i].Version = redactor.String(r.Tools[i].Version)
		for j := range r.Tools[i].Candidates {
			r.Tools[i].Candidates[j] = redactor.String(r.Tools[i].Candidates[j])
		}
		if r.Tools[i].Error != nil {
			r.Tools[i].Error.Message = redactor.String(r.Tools[i].Error.Message)
		}
	}
	for i := range r.Findings {
		for j := range r.Findings[i].Evidence {
			r.Findings[i].Evidence[j].Value = redactor.String(r.Findings[i].Evidence[j].Value)
			r.Findings[i].Evidence[j].Detail = redactor.String(r.Findings[i].Evidence[j].Detail)
		}
	}
	r.Redacted = true
}

func Encode(r Report) ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}
	return append(data, '\n'), nil
}

func WriteFile(dest string, data []byte) error {
	return writeFileAtomic(dest, data, os.Rename)
}

func writeFileAtomic(dest string, data []byte, rename func(string, string) error) error {
	if rename == nil {
		rename = os.Rename
	}
	dir := filepath.Dir(dest)
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination is a symlink: %s", dest)
		}
		if info.IsDir() {
			return fmt.Errorf("destination is a directory: %s", dest)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("destination is not a regular file: %s", dest)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect destination: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".kankyo-report-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary report: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temporary report: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary report: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary report: %w", err)
	}
	if err := rename(tmpName, dest); err != nil {
		return fmt.Errorf("replace report: %w", err)
	}
	cleanup = false
	return nil
}
