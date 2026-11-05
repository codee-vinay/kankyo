package platform

import (
	"os"
	"path/filepath"
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
	return filepath.Clean(a) == filepath.Clean(b)
}

func PathExt() []string {
	return nil
}

func FindCandidates(dir, command string) []Candidate {
	if dir == "" {
		return nil
	}
	path := filepath.Join(dir, command)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return []Candidate{{
		Path:       path,
		Executable: info.Mode().Perm()&0o111 != 0,
	}}
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
	if info.IsDir() {
		searchable := info.Mode().Perm()&0o111 != 0
		result.Searchable = &searchable
	}
	if link, linkErr := os.Lstat(path); linkErr == nil && link.Mode()&os.ModeSymlink != 0 {
		result.Symlink = true
	}
	return result
}

func isExecutableFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	return info.Mode().Perm()&0o111 != 0, nil
}
