package redact

import (
	"regexp"
	"strings"
)

type Redactor struct {
	home     string
	username string
	windows  bool
	userRe   *regexp.Regexp
}

func New(home, username, goos string) *Redactor {
	r := &Redactor{
		home:     strings.TrimRight(home, `\/`),
		username: username,
		windows:  goos == "windows",
	}
	if username != "" {
		pattern := `(^|[\\/])` + regexp.QuoteMeta(username) + `([\\/]|$)`
		if r.windows {
			pattern = `(?i)` + pattern
		}
		r.userRe = regexp.MustCompile(pattern)
	}
	return r
}

func (r *Redactor) String(s string) string {
	if r == nil || s == "" {
		return s
	}
	if r.home != "" {
		s = replaceHomePrefix(s, r.home, r.windows)
	}
	if r.userRe != nil {
		s = r.userRe.ReplaceAllString(s, `${1}<USER>${2}`)
	}
	return s
}

func replaceHomePrefix(s, home string, caseInsensitive bool) string {
	if len(home) == 0 || len(s) < len(home) {
		return s
	}
	prefix := s[:len(home)]
	match := prefix == home
	if caseInsensitive {
		match = strings.EqualFold(prefix, home)
	}
	if !match {
		return s
	}
	if len(s) == len(home) {
		return "<HOME>"
	}
	next := s[len(home)]
	if next != '\\' && next != '/' {
		return s
	}
	return "<HOME>" + s[len(home):]
}
