package diagnostics

import "testing"

func TestMakeKnownAndUnknown(t *testing.T) {
	f := Make("PATH_DUPLICATE_ENTRY", "", Evidence{Kind: "path_entry", Value: `C:\a`})
	if f.Severity != Warning || f.Title == "" || f.Description == "" || f.Impact == "" || f.Recommendation == "" {
		t.Fatalf("incomplete finding: %+v", f)
	}
	unknown := Make("NOT_A_REAL_ID", "")
	if unknown.Severity != Info || unknown.Title != "NOT_A_REAL_ID" {
		t.Fatalf("unexpected unknown fallback: %+v", unknown)
	}
}

func TestSortDedupeAndHasErrors(t *testing.T) {
	info := Make("TOOL_NOT_FOUND", "", Evidence{Kind: "command", Value: "go"})
	warn := Make("PATH_EMPTY_ENTRY", "", Evidence{Kind: "path_entry", Value: "x", Detail: "index 0"})
	err := Make("TOOL_TIMEOUT", "", Evidence{Kind: "command", Value: "go"})
	sorted := Sort(Dedupe([]Finding{info, warn, err, warn}))
	if len(sorted) != 3 {
		t.Fatalf("dedupe failed: %d", len(sorted))
	}
	if sorted[0].Severity != Error || sorted[1].Severity != Warning || sorted[2].Severity != Info {
		t.Fatalf("sort order wrong: %+v", sorted)
	}
	if !HasErrors(sorted) {
		t.Fatal("HasErrors returned false")
	}
}
