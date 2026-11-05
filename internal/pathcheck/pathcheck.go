package pathcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/codee-vinay/kankyo/internal/diagnostics"
	"github.com/codee-vinay/kankyo/internal/platform"
	"github.com/codee-vinay/kankyo/internal/tools"
)

type Entry struct {
	Index       int    `json:"index"`
	Raw         string `json:"raw"`
	Normalized  string `json:"normalized,omitempty"`
	Empty       bool   `json:"empty"`
	Exists      bool   `json:"exists"`
	IsDir       bool   `json:"is_dir"`
	Accessible  bool   `json:"accessible"`
	Searchable  *bool  `json:"searchable"`
	DuplicateOf *int   `json:"duplicate_of"`
	Relative    bool   `json:"relative"`
	Symlink     bool   `json:"symlink"`
	Error       string `json:"error,omitempty"`
}

type Counts struct {
	Duplicates   int `json:"duplicates"`
	Empty        int `json:"empty"`
	Missing      int `json:"missing"`
	Inaccessible int `json:"inaccessible"`
	Relative     int `json:"relative"`
}

type Candidate struct {
	ToolID     string
	Command    string
	Path       string
	EntryIndex int
	Executable bool
}

type Analysis struct {
	Entries    []Entry
	Candidates []Candidate
	Findings   []diagnostics.Finding
	Counts     Counts
}

var windowsEnvRef = regexp.MustCompile(`%[A-Za-z_][A-Za-z0-9_]*%`)
var unixEnvRef = regexp.MustCompile(`\$(\{[A-Za-z_][A-Za-z0-9_]*\}|[A-Za-z_][A-Za-z0-9_]*)`)

func Analyze(raw string, present bool, specs []tools.Spec, goos string) Analysis {
	var result Analysis
	if !present || raw == "" {
		result.Findings = append(result.Findings, diagnostics.Make(
			"PATH_UNAVAILABLE",
			"",
			diagnostics.Evidence{Kind: "environment", Value: "PATH"},
		))
		return result
	}

	entriesRaw := platform.SplitList(raw)
	result.Entries = make([]Entry, 0, len(entriesRaw))
	for i, rawEntry := range entriesRaw {
		entry := Entry{Index: i, Raw: rawEntry}
		if rawEntry == "" {
			entry.Empty = true
			result.Counts.Empty++
			severity := diagnostics.Warning
			if goos == "windows" {
				severity = diagnostics.Info
			}
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_EMPTY_ENTRY",
				severity,
				diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d", i)},
			))
			result.Entries = append(result.Entries, entry)
			continue
		}

		entry.Normalized = platform.NormalizePath(rawEntry)
		entry.Relative = !filepath.IsAbs(rawEntry)
		if entry.Relative {
			result.Counts.Relative++
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_RELATIVE_ENTRY",
				"",
				diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d", i)},
			))
		}

		if hasEnvReference(rawEntry, goos) {
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_ENV_VAR_NOT_EXPANDED",
				"",
				diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d", i)},
			))
		}

		// Duplicate detection uses documented lexical normalization. Symlink,
		// junction and 8.3 aliases are intentionally outside this check.
		for j := 0; j < i; j++ {
			prev := result.Entries[j]
			if prev.Empty || prev.Normalized == "" {
				continue
			}
			if platform.SamePath(prev.Normalized, entry.Normalized) {
				idx := prev.Index
				entry.DuplicateOf = &idx
				result.Counts.Duplicates++
				result.Findings = append(result.Findings, diagnostics.Make(
					"PATH_DUPLICATE_ENTRY",
					"",
					diagnostics.Evidence{Kind: "path_entry", Value: prev.Raw, Detail: fmt.Sprintf("index %d", prev.Index)},
					diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d", i)},
				))
				break
			}
		}

		info := platform.InspectEntry(rawEntry)
		entry.Exists = info.Exists
		entry.IsDir = info.IsDir
		entry.Accessible = info.Accessible
		entry.Searchable = info.Searchable
		entry.Symlink = info.Symlink
		entry.Error = info.Error

		switch {
		case info.PermissionDenied:
			result.Counts.Inaccessible++
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_DIRECTORY_INACCESSIBLE",
				"",
				diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d: %s", i, info.Error)},
			))
		case info.Error != "" && !info.Exists:
			result.Counts.Missing++
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_DIRECTORY_NOT_FOUND",
				"",
				diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d: %s", i, info.Error)},
			))
		case !info.Exists:
			result.Counts.Missing++
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_DIRECTORY_NOT_FOUND",
				"",
				diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d", i)},
			))
		case !info.IsDir:
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_ENTRY_NOT_DIRECTORY",
				"",
				diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d", i)},
			))
		case !info.Accessible:
			result.Counts.Inaccessible++
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_DIRECTORY_INACCESSIBLE",
				"",
				diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d", i)},
			))
		case info.Searchable != nil && !*info.Searchable:
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_DIRECTORY_NOT_SEARCHABLE",
				"",
				diagnostics.Evidence{Kind: "path_entry", Value: rawEntry, Detail: fmt.Sprintf("index %d", i)},
			))
		}

		result.Entries = append(result.Entries, entry)
	}

	result.Candidates = discoverCandidates(result.Entries, specs, goos)
	for _, spec := range specs {
		var candidates []Candidate
		for _, c := range result.Candidates {
			if c.ToolID == spec.ID {
				candidates = append(candidates, c)
			}
		}
		if len(candidates) > 1 {
			executableCount := 0
			for _, c := range candidates {
				if c.Executable {
					executableCount++
				}
			}
			if executableCount < 2 {
				continue
			}
			evidence := make([]diagnostics.Evidence, 0, len(candidates))
			for _, c := range candidates {
				evidence = append(evidence, diagnostics.Evidence{
					Kind:   "file",
					Value:  c.Path,
					Detail: fmt.Sprintf("%s; PATH index %d", spec.Name, c.EntryIndex),
				})
			}
			result.Findings = append(result.Findings, diagnostics.Make(
				"PATH_MULTIPLE_CANDIDATES",
				"",
				evidence...,
			))
		}
	}

	result.Findings = diagnostics.Sort(diagnostics.Dedupe(result.Findings))
	result.Findings = aggregatePathFindings(result.Findings)
	return result
}

// aggregatePathFindings merges repeated occurrences of the same PATH condition
// into one finding with multiple evidence entries. This keeps terminal output
// useful on large PATHs while retaining every occurrence in JSON evidence.
func aggregatePathFindings(findings []diagnostics.Finding) []diagnostics.Finding {
	aggregate := map[string]bool{
		"PATH_DUPLICATE_ENTRY":          true,
		"PATH_EMPTY_ENTRY":              true,
		"PATH_DIRECTORY_NOT_FOUND":      true,
		"PATH_DIRECTORY_INACCESSIBLE":   true,
		"PATH_DIRECTORY_NOT_SEARCHABLE": true,
		"PATH_ENTRY_NOT_DIRECTORY":      true,
		"PATH_RELATIVE_ENTRY":           true,
		"PATH_ENV_VAR_NOT_EXPANDED":     true,
	}
	index := make(map[string]int)
	out := make([]diagnostics.Finding, 0, len(findings))
	for _, finding := range findings {
		if !aggregate[finding.ID] || len(finding.Evidence) == 0 {
			out = append(out, finding)
			continue
		}
		key := finding.ID + "\x00" + finding.Evidence[0].Value
		if at, ok := index[key]; ok {
			out[at].Evidence = mergeEvidence(out[at].Evidence, finding.Evidence)
			continue
		}
		index[key] = len(out)
		out = append(out, finding)
	}
	return diagnostics.Sort(out)
}

func mergeEvidence(existing, added []diagnostics.Evidence) []diagnostics.Evidence {
	seen := make(map[string]bool, len(existing)+len(added))
	out := make([]diagnostics.Evidence, 0, len(existing)+len(added))
	for _, evidence := range append(append([]diagnostics.Evidence{}, existing...), added...) {
		key := evidence.Kind + "\x00" + evidence.Value + "\x00" + evidence.Detail
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, evidence)
	}
	return out
}

func hasEnvReference(entry, goos string) bool {
	if goos == "windows" {
		return windowsEnvRef.MatchString(entry)
	}
	return unixEnvRef.MatchString(entry)
}

func discoverCandidates(entries []Entry, specs []tools.Spec, goos string) []Candidate {
	var all []Candidate
	for _, spec := range specs {
		var byTool []Candidate
		for _, command := range spec.Commands(goos) {
			for _, entry := range entries {
				if entry.Empty || !entry.Exists || !entry.IsDir || !entry.Accessible {
					continue
				}
				if entry.Searchable != nil && !*entry.Searchable {
					continue
				}
				for _, found := range platform.FindCandidates(entry.Raw, command) {
					if !filepath.IsAbs(found.Path) {
						continue
					}
					candidate := Candidate{
						ToolID:     spec.ID,
						Command:    command,
						Path:       found.Path,
						EntryIndex: entry.Index,
						Executable: found.Executable,
					}
					if !hasCandidate(byTool, candidate.Path) {
						byTool = append(byTool, candidate)
					}
				}
			}
		}
		all = append(all, byTool...)
	}
	return all
}

func hasCandidate(candidates []Candidate, path string) bool {
	for _, c := range candidates {
		if platform.SamePath(c.Path, path) {
			return true
		}
		if infoA, errA := os.Stat(c.Path); errA == nil {
			if infoB, errB := os.Stat(path); errB == nil && os.SameFile(infoA, infoB) {
				return true
			}
		}
	}
	return false
}
