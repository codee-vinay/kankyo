package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/codee-vinay/kankyo/internal/diagnostics"
	"github.com/codee-vinay/kankyo/internal/pathcheck"
	"github.com/codee-vinay/kankyo/internal/platform"
	"github.com/codee-vinay/kankyo/internal/redact"
	"github.com/codee-vinay/kankyo/internal/render"
	"github.com/codee-vinay/kankyo/internal/report"
	"github.com/codee-vinay/kankyo/internal/scanner"
	"github.com/codee-vinay/kankyo/internal/tools"
	"github.com/codee-vinay/kankyo/internal/version"
)

const (
	globalTimeout = 8 * time.Second
	toolTimeout   = 3 * time.Second
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		welcome(stdout)
		return 0
	}

	switch args[0] {
	case "scan":
		return runScan(args[1:], stdout, stderr)
	case "path":
		return runPath(args[1:], stdout, stderr)
	case "tools":
		return runTools(args[1:], stdout, stderr)
	case "report":
		return runReport(args[1:], stdout, stderr)
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "help":
		return runHelp(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "kankyo: unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

type commonFlags struct {
	verbose bool
	redact  bool
}

func parseCommon(name string, args []string, stderr io.Writer) (commonFlags, int) {
	var out commonFlags
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { commandUsage(stderr, name) }
	fs.BoolVar(&out.verbose, "verbose", false, "show evidence and per-entry details")
	fs.BoolVar(&out.redact, "redact", false, "redact home directory and username in paths")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return out, 0
		}
		return out, 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "kankyo %s: unexpected argument %q\n\n", name, fs.Arg(0))
		commandUsage(stderr, name)
		return out, 2
	}
	return out, -1
}

func runScan(args []string, stdout, stderr io.Writer) int {
	flags, code := parseCommon("scan", args, stderr)
	if code >= 0 {
		return code
	}
	data, findings, err := collect(true)
	if err != nil {
		fmt.Fprintf(stderr, "kankyo scan: %v\n", err)
		return 3
	}
	rep := buildReport(data, findings, flags.redact)
	if err := render.Report(stdout, rep, render.Options{
		Color:     platform.ColorEnabled(stdout),
		Width:     platform.TerminalWidth(stdout),
		Verbose:   flags.verbose,
		ShowTools: true,
		ShowPath:  true,
	}); err != nil {
		fmt.Fprintf(stderr, "kankyo scan: %v\n", err)
		return 3
	}
	return exitFor(findings)
}

func runPath(args []string, stdout, stderr io.Writer) int {
	flags, code := parseCommon("path", args, stderr)
	if code >= 0 {
		return code
	}
	analysis := analyzePath()
	rep := buildReport(collection{analysis: analysis, findings: analysis.Findings}, analysis.Findings, flags.redact)
	rep.Tools = []scanner.ToolResult{}
	rep.Summary.ToolsChecked = 0
	rep.Summary.ToolsDetected = 0
	rep.Summary.Skipped = 0
	if err := render.Report(stdout, rep, render.Options{
		Color:     platform.ColorEnabled(stdout),
		Width:     platform.TerminalWidth(stdout),
		Verbose:   flags.verbose,
		ShowTools: false,
		ShowPath:  true,
	}); err != nil {
		fmt.Fprintf(stderr, "kankyo path: %v\n", err)
		return 3
	}
	return exitFor(analysis.Findings)
}

func runTools(args []string, stdout, stderr io.Writer) int {
	flags, code := parseCommon("tools", args, stderr)
	if code >= 0 {
		return code
	}
	data, findings, err := collect(true)
	if err != nil {
		fmt.Fprintf(stderr, "kankyo tools: %v\n", err)
		return 3
	}
	findings = toolFocusedFindings(findings)
	rep := buildReport(collection{analysis: data.analysis, tools: data.tools, findings: findings}, findings, flags.redact)
	if err := render.Report(stdout, rep, render.Options{
		Color:     platform.ColorEnabled(stdout),
		Width:     platform.TerminalWidth(stdout),
		Verbose:   flags.verbose,
		ShowTools: true,
		ShowPath:  false,
	}); err != nil {
		fmt.Fprintf(stderr, "kankyo tools: %v\n", err)
		return 3
	}
	return exitFor(findings)
}

func runReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { commandUsage(stderr, "report") }
	var output string
	var doRedact bool
	fs.StringVar(&output, "output", "", "write JSON report to this path, or - for stdout")
	fs.BoolVar(&doRedact, "redact", false, "redact home directory and username in the report")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "kankyo report: unexpected argument %q\n\n", fs.Arg(0))
		commandUsage(stderr, "report")
		return 2
	}
	if strings.TrimSpace(output) == "" {
		fmt.Fprintln(stderr, "kankyo report: --output is required")
		fmt.Fprintln(stderr)
		commandUsage(stderr, "report")
		return 2
	}

	data, findings, err := collect(true)
	if err != nil {
		fmt.Fprintf(stderr, "kankyo report: %v\n", err)
		return 3
	}
	rep := buildReport(data, findings, doRedact)
	encoded, err := report.Encode(rep)
	if err != nil {
		fmt.Fprintf(stderr, "kankyo report: %v\n", err)
		return 3
	}
	if output == "-" {
		if _, err := stdout.Write(encoded); err != nil {
			fmt.Fprintf(stderr, "kankyo report: %v\n", err)
			return 3
		}
	} else if err := report.WriteFile(output, encoded); err != nil {
		fmt.Fprintf(stderr, "kankyo report: %v\n", err)
		return 3
	}
	return exitFor(findings)
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "kankyo version: no arguments are accepted")
		return 2
	}
	fmt.Fprintf(stdout, "kankyo %s\n", version.Version)
	fmt.Fprintf(stdout, "commit:      %s\n", version.Commit)
	fmt.Fprintf(stdout, "built:       %s\n", version.BuildDate)
	fmt.Fprintf(stdout, "go:          %s\n", runtime.Version())
	fmt.Fprintf(stdout, "platform:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return 0
}

func runHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stdout)
		return 0
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "kankyo help: at most one command name is accepted")
		return 2
	}
	switch args[0] {
	case "scan", "path", "tools", "report", "version", "help":
		commandUsage(stdout, args[0])
		return 0
	default:
		fmt.Fprintf(stderr, "kankyo help: unknown command %q\n", args[0])
		return 2
	}
}

type collection struct {
	analysis pathcheck.Analysis
	tools    []scanner.ToolResult
	findings []diagnostics.Finding
}

func collect(withTools bool) (collection, []diagnostics.Finding, error) {
	analysis := analyzePath()
	if !withTools {
		return collection{analysis: analysis, findings: analysis.Findings}, analysis.Findings, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), globalTimeout)
	defer cancel()
	results, toolFindings := scanner.Check(ctx, tools.Specs, analysis, scanner.Options{
		Goos:    runtime.GOOS,
		Timeout: toolTimeout,
	})
	findings := append(append([]diagnostics.Finding{}, analysis.Findings...), toolFindings...)
	findings = diagnostics.Sort(diagnostics.Dedupe(findings))
	return collection{analysis: analysis, tools: results, findings: findings}, findings, nil
}

func analyzePath() pathcheck.Analysis {
	raw, present := os.LookupEnv("PATH")
	return pathcheck.Analyze(raw, present, tools.Specs, runtime.GOOS)
}

func buildReport(data collection, findings []diagnostics.Finding, doRedact bool) report.Report {
	rep := report.Build(time.Now(), version.Version, platform.Current(), data.tools, data.analysis, findings)
	if doRedact {
		report.ApplyRedaction(&rep, newRedactor())
	}
	return rep
}

func newRedactor() *redact.Redactor {
	home, _ := os.UserHomeDir()
	username := os.Getenv("USERNAME")
	if runtime.GOOS != "windows" {
		username = os.Getenv("USER")
		if username == "" && home != "" {
			username = filepath.Base(home)
		}
	}
	return redact.New(home, username, runtime.GOOS)
}

func exitFor(findings []diagnostics.Finding) int {
	if diagnostics.HasErrors(findings) {
		return 1
	}
	return 0
}

func toolFocusedFindings(findings []diagnostics.Finding) []diagnostics.Finding {
	out := make([]diagnostics.Finding, 0, len(findings))
	for _, finding := range findings {
		if strings.HasPrefix(finding.ID, "TOOL_") ||
			strings.HasPrefix(finding.ID, "PLATFORM_") ||
			finding.ID == "PATH_MULTIPLE_CANDIDATES" {
			out = append(out, finding)
		}
	}
	return diagnostics.Sort(diagnostics.Dedupe(out))
}

func welcome(w io.Writer) {
	fmt.Fprintln(w, "KANKYŌ — Environment Diagnostics")
	fmt.Fprintln(w, "Understand your environment.")
	fmt.Fprintf(w, "kankyo %s\n", version.Version)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  kankyo <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  scan      Run a complete environment scan")
	fmt.Fprintln(w, "  path      Inspect the process PATH")
	fmt.Fprintln(w, "  tools     Check supported developer tools")
	fmt.Fprintln(w, "  report    Write a JSON diagnostic report")
	fmt.Fprintln(w, "  version   Show version and build information")
	fmt.Fprintln(w, "  help      Show help for a command")
	fmt.Fprintln(w)
	fmt.Fprintln(w, `Run "kankyo scan" to inspect your development environment.`)
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "KANKYŌ — Environment Diagnostics")
	fmt.Fprintln(w, "Understand your environment.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  kankyo <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  scan      Run a complete environment scan")
	fmt.Fprintln(w, "  path      Inspect the process PATH")
	fmt.Fprintln(w, "  tools     Check supported developer tools")
	fmt.Fprintln(w, "  report    Write a JSON diagnostic report")
	fmt.Fprintln(w, "  version   Show version and build information")
	fmt.Fprintln(w, "  help      Show help for a command")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit codes:")
	fmt.Fprintln(w, "  0  success")
	fmt.Fprintln(w, "  1  completed with ERROR findings")
	fmt.Fprintln(w, "  2  usage error")
	fmt.Fprintln(w, "  3  operational failure")
}

func commandUsage(w io.Writer, name string) {
	switch name {
	case "scan":
		fmt.Fprintln(w, "Usage: kankyo scan [--verbose] [--redact]")
		fmt.Fprintln(w, "Run a complete environment scan.")
	case "path":
		fmt.Fprintln(w, "Usage: kankyo path [--verbose] [--redact]")
		fmt.Fprintln(w, "Inspect the current process PATH only.")
	case "tools":
		fmt.Fprintln(w, "Usage: kankyo tools [--verbose] [--redact]")
		fmt.Fprintln(w, "Check the supported developer tools.")
	case "report":
		fmt.Fprintln(w, "Usage: kankyo report --output <path|-> [--redact]")
		fmt.Fprintln(w, "Write the diagnostic report as JSON.")
	case "version":
		fmt.Fprintln(w, "Usage: kankyo version")
		fmt.Fprintln(w, "Show version and build information.")
	case "help":
		fmt.Fprintln(w, "Usage: kankyo help [command]")
		fmt.Fprintln(w, "Show general or command-specific help.")
	default:
		usage(w)
	}
}
