package platform

import (
	"os"
	"path/filepath"
	"strings"
)

type EntryInfo struct {
	Exists           bool
	IsDir            bool
	Accessible       bool
	PermissionDenied bool
	Searchable       *bool
	Symlink          bool
	Error            string
}

type Candidate struct {
	Path       string
	Executable bool
}

func SamePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func PathExt() []string {
	raw := os.Getenv("PATHEXT")
	if raw == "" {
		return []string{".com", ".exe", ".bat", ".cmd"}
	}
	var exts []string
	for _, ext := range strings.Split(strings.ToLower(raw), ";") {
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		exts = append(exts, ext)
	}
	return exts
}

func FindCandidates(dir, command string) []Candidate {
	if dir == "" {
		return nil
	}
	base := filepath.Join(dir, command)
	var out []Candidate
	seen := make(map[string]bool)
	add := func(path string) {
		if seen[strings.ToLower(filepath.Clean(path))] {
			return
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		seen[strings.ToLower(filepath.Clean(path))] = true
		out = append(out, Candidate{Path: path, Executable: true})
	}

	if filepath.Ext(command) != "" {
		add(base)
	}
	for _, ext := range PathExt() {
		add(base + ext)
	}
	return out
}

func InspectEntry(path string) EntryInfo {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsPermission(err) {
			return EntryInfo{PermissionDenied: true, Error: "permission denied"}
		}
		if os.IsNotExist(err) {
			return EntryInfo{}
		}
		return EntryInfo{Error: err.Error()}
	}
	result := EntryInfo{Exists: true, IsDir: info.IsDir(), Accessible: true}
	if link, linkErr := os.Lstat(path); linkErr == nil && link.Mode()&os.ModeSymlink != 0 {
		result.Symlink = true
	}
	return result
}
