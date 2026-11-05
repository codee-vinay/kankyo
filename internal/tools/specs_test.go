package tools

import "testing"

func TestVersionParsers(t *testing.T) {
	cases := []struct {
		id     string
		stdout string
		stderr string
		want   string
	}{
		{"go", "go version go1.27.3 windows/amd64\n", "", "1.27.3"},
		{"git", "git version 2.55.0.windows.3\n", "", "2.55.0.windows.3"},
		{"node", "v26.13.0\n", "", "26.13.0"},
		{"npm", "11.20.0\n", "", "11.20.0"},
		{"python", "Python 3.13.0\n", "", "3.13.0"},
		{"rust", "rustc 1.90.0 (abc 2026-01-01)\n", "", "1.90.0"},
		{"cargo", "cargo 1.90.0 (abc 2026-01-01)\n", "", "1.90.0"},
		{"java", "", `openjdk version "22.0.2" 2026-01-01` + "\n", "22.0.2"},
	}
	for _, tc := range cases {
		spec, ok := FindSpec(tc.id)
		if !ok {
			t.Fatalf("missing spec %s", tc.id)
		}
		if got := ParseVersion(spec, tc.stdout, tc.stderr); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.id, got, tc.want)
		}
	}
}

func TestVersionParserFallbackAndEmpty(t *testing.T) {
	spec, _ := FindSpec("go")
	if got := ParseVersion(spec, "custom build 2026.1\n", ""); got != "custom build 2026.1" {
		t.Fatalf("fallback = %q", got)
	}
	if got := ParseVersion(spec, "no digits here\n", ""); got != "" {
		t.Fatalf("unexpected fallback %q", got)
	}
	if got := ParseVersion(spec, "", ""); got != "" {
		t.Fatalf("empty output returned %q", got)
	}
}

func TestSpecCommands(t *testing.T) {
	spec, _ := FindSpec("python")
	if got := spec.Commands("windows"); len(got) != 2 || got[0] != "python" {
		t.Fatalf("windows python commands = %v", got)
	}
	if got := spec.Commands("linux"); len(got) != 2 || got[0] != "python3" {
		t.Fatalf("linux python commands = %v", got)
	}
}
