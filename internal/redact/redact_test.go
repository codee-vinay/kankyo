package redact

import "testing"

func TestRedactWindows(t *testing.T) {
	r := New(`C:\Users\alice`, "alice", "windows")
	cases := map[string]string{
		`C:\Users\alice\bin\go.exe`: `<HOME>\bin\go.exe`,
		`C:\Users\ALICE\bin`:        `<HOME>\bin`,
		`D:\Users\alice\other`:      `D:\Users\<USER>\other`,
		`C:\Users\alice2\bin`:       `C:\Users\alice2\bin`,
	}
	for in, want := range cases {
		if got := r.String(in); got != want {
			t.Errorf("String(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactLinux(t *testing.T) {
	r := New("/home/alice", "alice", "linux")
	if got := r.String("/home/alice/bin/go"); got != "<HOME>/bin/go" {
		t.Fatalf("home redaction = %q", got)
	}
	if got := r.String("/opt/alice/bin"); got != "/opt/<USER>/bin" {
		t.Fatalf("username redaction = %q", got)
	}
	if got := r.String("/home/alice2/bin"); got != "/home/alice2/bin" {
		t.Fatalf("boundary redaction = %q", got)
	}
}
