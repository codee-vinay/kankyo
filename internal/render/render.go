package render

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/codee-vinay/kankyo/internal/diagnostics"
	"github.com/codee-vinay/kankyo/internal/report"
	"github.com/codee-vinay/kankyo/internal/scanner"
)

type Options struct {
	Color     bool
	Width     int
	Verbose   bool
	ShowTools bool
	ShowPath  bool
}

type renderer struct {
	w    *bufio.Writer
	opts Options
}

func Report(w io.Writer, rep report.Report, opts Options) error {
	if opts.Width <= 0 {
		opts.Width = 80
	}
	r := &renderer{w: bufio.NewWriter(w), opts: opts}

	r.header()
	r.section("Platform")
	r.kv("OS", displayOS(rep.Platform.OS))
	r.kv("Arch", rep.Platform.Arch)

	r.section("Environment")
	r.kv("PATH source", "Process environment")
	r.kv("PATH entries", fmt.Sprintf("%d", rep.Path.EntryCount))
	if opts.ShowTools {
		r.kv("Tools checked", fmt.Sprintf("%d", rep.Summary.ToolsChecked))
	}

	if opts.ShowTools {
		r.section("Developer Tools")
		if len(rep.Tools) == 0 {
			r.line("  [INFO]  No tool results available.")
		}
		for _, tool := range rep.Tools {
			r.toolLine(tool)
		}
	}

	if opts.ShowPath {
		r.section("PATH Analysis")
		r.pathSection(rep)
	}

	r.section("Detected Issues")
	if len(rep.Findings) == 0 {
		r.line("  [OK]    No issues detected by the checks performed.")
	} else {
		limit := len(rep.Findings)
		if !opts.Verbose && limit > 20 {
			limit = 20
		}
		for _, finding := range rep.Findings[:limit] {
			r.finding(finding)
		}
		if limit < len(rep.Findings) {
			r.line(fmt.Sprintf("  [INFO]  %d additional findings omitted; use --verbose to show them.", len(rep.Findings)-limit))
		}
	}

	if recs := recommendations(rep.Findings); len(recs) > 0 {
		r.section("Recommendations")
		for _, rec := range recs {
			r.wrapped("  - ", rec, "    ")
		}
	}

	r.section("Summary")
	r.kv("Tools detected", fmt.Sprintf("%d", rep.Summary.ToolsDetected))
	r.kv("Warnings", fmt.Sprintf("%d", rep.Summary.Warnings))
	r.kv("Errors", fmt.Sprintf("%d", rep.Summary.Errors))
	if rep.Summary.Skipped > 0 {
		r.kv("Skipped", fmt.Sprintf("%d", rep.Summary.Skipped))
	}
	return r.w.Flush()
}

func (r *renderer) header() {
	r.line("KANKYŌ — Environment Diagnostics")
}

func (r *renderer) section(name string) {
	r.line("")
	r.line(name)
}

func (r *renderer) kv(label, value string) {
	r.line(fmt.Sprintf("  %-14s %s", label+":", value))
}

func (r *renderer) toolLine(tool scanner.ToolResult) {
	label := toolStatusLabel(tool)
	detail := toolDetail(tool)
	r.line(fmt.Sprintf("  %-8s %-12s %s", r.color(label, toolColor(tool.Status)), tool.Name, detail))
}

func (r *renderer) pathSection(rep report.Report) {
	if rep.Path.EntryCount == 0 {
		r.line("  [INFO]  No PATH entries available.")
		return
	}
	r.line(fmt.Sprintf("  %-8s PATH entries inspected (%d)", r.color("[OK]", 32), rep.Path.EntryCount))
	if rep.Path.Analysis.Duplicates > 0 {
		r.line(fmt.Sprintf("  %-8s Duplicate entries: %d", r.color("[WARN]", 33), rep.Path.Analysis.Duplicates))
	}
	if rep.Path.Analysis.Missing > 0 {
		r.line(fmt.Sprintf("  %-8s Missing directories: %d", r.color("[WARN]", 33), rep.Path.Analysis.Missing))
	}
	if rep.Path.Analysis.Inaccessible > 0 {
		r.line(fmt.Sprintf("  %-8s Inaccessible directories: %d", r.color("[WARN]", 33), rep.Path.Analysis.Inaccessible))
	}
	if rep.Path.Analysis.Empty > 0 {
		r.line(fmt.Sprintf("  %-8s Empty entries: %d", r.color("[WARN]", 33), rep.Path.Analysis.Empty))
	}
	if rep.Path.Analysis.Relative > 0 {
		r.line(fmt.Sprintf("  %-8s Relative entries: %d", r.color("[WARN]", 33), rep.Path.Analysis.Relative))
	}

	if r.opts.Verbose {
		for _, entry := range rep.Path.Entries {
			status := "[OK]"
			color := 32
			switch {
			case entry.Empty:
				status, color = "[WARN]", 33
			case !entry.Exists:
				status, color = "[WARN]", 33
			case !entry.IsDir:
				status, color = "[WARN]", 33
			case !entry.Accessible:
				status, color = "[WARN]", 33
			case entry.Searchable != nil && !*entry.Searchable:
				status, color = "[WARN]", 33
			}
			detail := entry.Raw
			if entry.Empty {
				detail = "(empty entry)"
			}
			r.line(fmt.Sprintf("  %-8s %s", r.color(status, color), detail))
			if entry.DuplicateOf != nil {
				r.line(fmt.Sprintf("           duplicate of index %d", *entry.DuplicateOf))
			}
			if entry.Error != "" {
				r.line("           " + entry.Error)
			}
		}
	}
}

func (r *renderer) finding(f diagnostics.Finding) {
	label := severityLabel(f.Severity)
	color := severityColor(f.Severity)
	r.line("  " + r.color(label, color) + " " + f.ID + " — " + f.Title)
	r.wrapped("    ", f.Description, "    ")
	if r.opts.Verbose {
		r.wrapped("    Impact: ", f.Impact, "            ")
		for _, evidence := range f.Evidence {
			value := evidence.Value
			if evidence.Detail != "" {
				value += " (" + evidence.Detail + ")"
			}
			r.wrapped("    Evidence: ", value, "              ")
		}
	}
	if f.Recommendation != "" {
		r.wrapped("    Recommendation: ", f.Recommendation, "                    ")
	}
}

func (r *renderer) wrapped(prefix, text, continuation string) {
	width := r.opts.Width - len(prefix)
	if width < 20 {
		width = 20
	}
	for i, line := range wrap(text, width) {
		if i == 0 {
			r.line(prefix + line)
		} else {
			r.line(continuation + line)
		}
	}
}

func (r *renderer) line(s string) {
	fmt.Fprintln(r.w, s)
}

func (r *renderer) color(s string, code int) string {
	if !r.opts.Color {
		return s
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", code, s)
}

func wrap(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func recommendations(findings []diagnostics.Finding) []string {
	seen := make(map[string]bool)
	var out []string
	for _, f := range findings {
		if f.Recommendation == "" || seen[f.Recommendation] {
			continue
		}
		seen[f.Recommendation] = true
		out = append(out, f.Recommendation)
	}
	sort.Strings(out)
	return out
}

func toolStatusLabel(tool scanner.ToolResult) string {
	switch tool.Status {
	case scanner.ToolOK:
		return "[OK]"
	case scanner.ToolWarning:
		return "[WARN]"
	case scanner.ToolErrorStatus:
		return "[ERROR]"
	case scanner.ToolSkipped:
		return "[SKIP]"
	default:
		return "[INFO]"
	}
}

func toolColor(status scanner.ToolStatus) int {
	switch status {
	case scanner.ToolOK:
		return 32
	case scanner.ToolWarning:
		return 33
	case scanner.ToolErrorStatus:
		return 31
	case scanner.ToolSkipped:
		return 0
	default:
		return 36
	}
}

func toolDetail(tool scanner.ToolResult) string {
	if !tool.Found {
		return "Not found"
	}
	if tool.Status == scanner.ToolSkipped {
		return "Skipped"
	}
	if tool.Error != nil {
		switch tool.Error.Code {
		case "not_executable":
			return "Not executable"
		case "timeout":
			return "Timed out"
		case "prepare":
			return "Unsupported"
		case "start":
			return "Failed to start"
		default:
			return "Execution failed"
		}
	}
	if tool.Multiple {
		return "Multiple installations detected"
	}
	if tool.Version != "" {
		return tool.Version
	}
	return "Version unavailable"
}

func severityLabel(severity diagnostics.Severity) string {
	switch severity {
	case diagnostics.Error:
		return "[ERROR]"
	case diagnostics.Warning:
		return "[WARN]"
	default:
		return "[INFO]"
	}
}

func severityColor(severity diagnostics.Severity) int {
	switch severity {
	case diagnostics.Error:
		return 31
	case diagnostics.Warning:
		return 33
	default:
		return 36
	}
}

func displayOS(osName string) string {
	switch osName {
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return osName
	}
}
