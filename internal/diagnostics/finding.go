package diagnostics

import (
	"fmt"
	"sort"
	"strings"
)

type Severity string

const (
	Info    Severity = "INFO"
	Warning Severity = "WARNING"
	Error   Severity = "ERROR"
)

type Evidence struct {
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
}

type Finding struct {
	ID             string     `json:"id"`
	Severity       Severity   `json:"severity"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Evidence       []Evidence `json:"evidence"`
	Impact         string     `json:"impact"`
	Recommendation string     `json:"recommendation"`
}

type Meta struct {
	Severity       Severity
	Title          string
	Description    string
	Impact         string
	Recommendation string
}

var registry = map[string]Meta{
	"PATH_UNAVAILABLE": {
		Severity:       Error,
		Title:          "PATH is unavailable",
		Description:    "Kankyo could not find a process PATH value to inspect.",
		Impact:         "Supported tools cannot be resolved, and PATH diagnostics cannot be performed.",
		Recommendation: "Set PATH in the current shell or session; Kankyo will not modify it.",
	},
	"PATH_DUPLICATE_ENTRY": {
		Severity:       Warning,
		Title:          "Duplicate PATH entry detected",
		Description:    "The same directory appears more than once in PATH.",
		Impact:         "May create unnecessary PATH complexity and ambiguous command resolution.",
		Recommendation: "Review the duplicate entry and remove it manually if appropriate.",
	},
	"PATH_EMPTY_ENTRY": {
		Severity:       Warning,
		Title:          "Empty PATH entry detected",
		Description:    "PATH contains an empty entry.",
		Impact:         "On Linux an empty entry means the current directory; Windows tools may ignore it or treat it differently.",
		Recommendation: "Remove the empty entry or replace it with an explicit directory path.",
	},
	"PATH_DIRECTORY_NOT_FOUND": {
		Severity:       Warning,
		Title:          "PATH directory does not exist",
		Description:    "A directory listed in PATH could not be found.",
		Impact:         "Commands in that directory cannot be found through PATH.",
		Recommendation: "Correct the path or remove the nonexistent entry manually.",
	},
	"PATH_DIRECTORY_INACCESSIBLE": {
		Severity:       Warning,
		Title:          "PATH directory is inaccessible",
		Description:    "A directory listed in PATH returned a permission error when inspected.",
		Impact:         "Commands in that directory may not be discoverable by the current user.",
		Recommendation: "Review the directory permissions or remove the entry if it is no longer needed.",
	},
	"PATH_DIRECTORY_NOT_SEARCHABLE": {
		Severity:       Warning,
		Title:          "PATH directory may not be searchable",
		Description:    "A Linux PATH directory lacks execute/search permission bits.",
		Impact:         "Commands in this directory may not be discoverable, depending on ACLs and the current user.",
		Recommendation: "Review the directory permissions; Kankyo does not change them.",
	},
	"PATH_ENTRY_NOT_DIRECTORY": {
		Severity:       Warning,
		Title:          "PATH entry is not a directory",
		Description:    "A PATH entry exists but is not a directory.",
		Impact:         "The entry cannot contribute executable candidates to PATH resolution.",
		Recommendation: "Replace the entry with its containing directory or remove it manually.",
	},
	"PATH_RELATIVE_ENTRY": {
		Severity:       Warning,
		Title:          "Relative PATH entry detected",
		Description:    "A non-empty PATH entry is relative rather than absolute.",
		Impact:         "Resolution depends on the current working directory and can be unpredictable or unsafe.",
		Recommendation: "Use an absolute directory path instead.",
	},
	"PATH_ENV_VAR_NOT_EXPANDED": {
		Severity:       Warning,
		Title:          "PATH entry contains an unexpanded variable reference",
		Description:    "A PATH entry contains a literal environment-variable reference.",
		Impact:         "Kankyo does not expand it, and some tools may not resolve the entry as intended.",
		Recommendation: "Use an expanded absolute path in PATH.",
	},
	"PATH_MULTIPLE_CANDIDATES": {
		Severity:       Warning,
		Title:          "Multiple executable candidates detected",
		Description:    "More than one executable candidate for a supported command was found in PATH.",
		Impact:         "The first PATH match normally wins; a different installation may be used than expected.",
		Recommendation: "Review PATH order and the detected installations; Kankyo does not modify PATH.",
	},
	"TOOL_NOT_FOUND": {
		Severity:       Info,
		Title:          "Supported tool not found",
		Description:    "No executable candidate for this supported tool was found in the process PATH.",
		Impact:         "The tool cannot be checked or used through PATH until it is installed or configured.",
		Recommendation: "Install or configure the tool only if it is required for your work.",
	},
	"TOOL_NOT_EXECUTABLE": {
		Severity:       Error,
		Title:          "Tool candidate is not executable",
		Description:    "A file matching the tool name exists but cannot be executed.",
		Impact:         "The tool cannot run until its executable permissions are corrected.",
		Recommendation: "Review the file permissions or reinstall the tool if appropriate.",
	},
	"TOOL_PERMISSION_DENIED": {
		Severity:       Error,
		Title:          "Tool execution permission denied",
		Description:    "The operating system denied execution of a supported tool.",
		Impact:         "The tool's installation or environment cannot be verified.",
		Recommendation: "Review permissions and security policy; Kankyo does not change them.",
	},
	"TOOL_EXECUTION_FAILED": {
		Severity:       Error,
		Title:          "Tool version check failed",
		Description:    "The supported tool was found but its version command did not complete successfully.",
		Impact:         "The tool may be broken, misconfigured, or blocked by the environment.",
		Recommendation: "Run the tool manually to investigate; reinstall only if appropriate.",
	},
	"TOOL_TIMEOUT": {
		Severity:       Error,
		Title:          "Tool version check timed out",
		Description:    "The supported tool did not finish its version command within the allowed time.",
		Impact:         "The tool's version and functionality could not be verified.",
		Recommendation: "Run the tool manually and check whether a wrapper or startup script is hanging.",
	},
	"TOOL_VERSION_UNPARSED": {
		Severity:       Warning,
		Title:          "Tool version could not be parsed",
		Description:    "The version command completed but its output did not match the expected format.",
		Impact:         "The tool may still work, but Kankyo could not report a reliable version.",
		Recommendation: "Run the tool's version command manually and inspect its output.",
	},
	"TOOL_CWD_RESOLUTION": {
		Severity:       Warning,
		Title:          "Tool resolved relative to the current directory",
		Description:    "Go's executable lookup found a candidate relative to the current working directory.",
		Impact:         "Kankyo refuses to execute this candidate to avoid running an unintended program.",
		Recommendation: "Remove current-directory resolution from PATH and use an absolute installation path.",
	},
	"TOOL_EXECUTION_SKIPPED": {
		Severity:       Info,
		Title:          "Tool execution intentionally skipped",
		Description:    "Kankyo found the tool but skipped execution for safety reasons.",
		Impact:         "The tool's version was not verified.",
		Recommendation: "Run the tool manually if you need to verify it.",
	},
	"TOOL_EXECUTION_UNSUPPORTED": {
		Severity:       Error,
		Title:          "Tool execution mode is unsupported",
		Description:    "Kankyo could not safely construct the required execution path for this tool.",
		Impact:         "The tool was not run and its version was not verified.",
		Recommendation: "Inspect the installation and the relevant environment settings.",
	},
	"PLATFORM_COMSPEC_MISMATCH": {
		Severity:       Warning,
		Title:          "ComSpec differs from the system command interpreter",
		Description:    "The ComSpec environment variable does not match the validated system cmd.exe path.",
		Impact:         "Some shell wrappers may use a different interpreter than Kankyo uses for batch checks.",
		Recommendation: "Review ComSpec; Kankyo intentionally uses the system cmd.exe from GetSystemDirectory.",
	},
}

// Make returns a finding using the registry metadata. An explicit severity can
// override the registry default when a condition has platform-specific impact.
func Make(id string, severity Severity, evidence ...Evidence) Finding {
	meta, ok := registry[id]
	if !ok {
		meta = Meta{
			Severity:       Info,
			Title:          id,
			Description:    "Diagnostic metadata is missing.",
			Impact:         "The meaning of this finding is not documented.",
			Recommendation: "Report this as a Kankyo defect.",
		}
	}
	if severity == "" {
		severity = meta.Severity
	}
	if evidence == nil {
		evidence = []Evidence{}
	}
	return Finding{
		ID:             id,
		Severity:       severity,
		Title:          meta.Title,
		Description:    meta.Description,
		Evidence:       evidence,
		Impact:         meta.Impact,
		Recommendation: meta.Recommendation,
	}
}

func SeverityRank(s Severity) int {
	switch s {
	case Error:
		return 3
	case Warning:
		return 2
	case Info:
		return 1
	default:
		return 0
	}
}

func Sort(findings []Finding) []Finding {
	out := append([]Finding(nil), findings...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := SeverityRank(out[i].Severity), SeverityRank(out[j].Severity)
		if ri != rj {
			return ri > rj
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return evidenceKey(out[i].Evidence) < evidenceKey(out[j].Evidence)
	})
	return out
}

func Dedupe(findings []Finding) []Finding {
	seen := make(map[string]bool, len(findings))
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		key := string(f.Severity) + "|" + f.ID + "|" + evidenceKey(f.Evidence)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}

func HasErrors(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == Error {
			return true
		}
	}
	return false
}

func evidenceKey(evidence []Evidence) string {
	var b strings.Builder
	for _, e := range evidence {
		fmt.Fprintf(&b, "%s\x00%s\x00%s\x00", e.Kind, e.Value, e.Detail)
	}
	return b.String()
}
