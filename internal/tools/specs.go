package tools

import (
	"regexp"
	"strings"
)

type Spec struct {
	ID              string
	Name            string
	WindowsCommands []string
	UnixCommands    []string
	VersionArgs     []string
	Pattern         *regexp.Regexp
}

func (s Spec) Commands(goos string) []string {
	if goos == "windows" {
		return s.WindowsCommands
	}
	return s.UnixCommands
}

var Specs = []Spec{
	{
		ID:              "go",
		Name:            "Go",
		WindowsCommands: []string{"go"},
		UnixCommands:    []string{"go"},
		VersionArgs:     []string{"version"},
		Pattern:         regexp.MustCompile(`(?m)^go version go([^\s]+)`),
	},
	{
		ID:              "git",
		Name:            "Git",
		WindowsCommands: []string{"git"},
		UnixCommands:    []string{"git"},
		VersionArgs:     []string{"--version"},
		Pattern:         regexp.MustCompile(`(?m)^git version ([^\s]+)`),
	},
	{
		ID:              "node",
		Name:            "Node.js",
		WindowsCommands: []string{"node"},
		UnixCommands:    []string{"node"},
		VersionArgs:     []string{"--version"},
		Pattern:         regexp.MustCompile(`(?m)^v?([0-9]+\.[0-9]+\.[0-9]+[^\s]*)`),
	},
	{
		ID:              "npm",
		Name:            "npm",
		WindowsCommands: []string{"npm"},
		UnixCommands:    []string{"npm"},
		VersionArgs:     []string{"--version"},
		Pattern:         regexp.MustCompile(`(?m)^([0-9]+\.[0-9]+\.[0-9]+[^\s]*)`),
	},
	{
		ID:              "python",
		Name:            "Python",
		WindowsCommands: []string{"python", "py"},
		UnixCommands:    []string{"python3", "python"},
		VersionArgs:     []string{"--version"},
		Pattern:         regexp.MustCompile(`(?im)^Python\s+([0-9][^\s]*)`),
	},
	{
		ID:              "rust",
		Name:            "Rust",
		WindowsCommands: []string{"rustc"},
		UnixCommands:    []string{"rustc"},
		VersionArgs:     []string{"--version"},
		Pattern:         regexp.MustCompile(`(?m)^rustc\s+([0-9][^\s]*)`),
	},
	{
		ID:              "cargo",
		Name:            "Cargo",
		WindowsCommands: []string{"cargo"},
		UnixCommands:    []string{"cargo"},
		VersionArgs:     []string{"--version"},
		Pattern:         regexp.MustCompile(`(?m)^cargo\s+([0-9][^\s]*)`),
	},
	{
		ID:              "java",
		Name:            "Java",
		WindowsCommands: []string{"java"},
		UnixCommands:    []string{"java"},
		VersionArgs:     []string{"-version"},
		Pattern:         regexp.MustCompile(`(?m)version\s+"([^"]+)"`),
	},
}

func FindSpec(id string) (Spec, bool) {
	for _, s := range Specs {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}

// ParseVersion extracts a short version from stdout, then stderr. It never
// returns raw multi-line output.
func ParseVersion(spec Spec, stdout, stderr string) string {
	for _, output := range []string{stdout, stderr} {
		if output == "" {
			continue
		}
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(strings.TrimRight(line, "\r"))
			if line == "" {
				continue
			}
			if spec.Pattern != nil {
				if m := spec.Pattern.FindStringSubmatch(line); len(m) > 1 {
					return strings.TrimSpace(m[1])
				}
			}
		}
	}
	for _, output := range []string{stdout, stderr} {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(strings.TrimRight(line, "\r"))
			if line == "" {
				continue
			}
			fallback := strings.Join(strings.Fields(line), " ")
			if len(fallback) > 80 {
				fallback = fallback[:77] + "..."
			}
			if strings.IndexFunc(fallback, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0 {
				return fallback
			}
		}
	}
	return ""
}
