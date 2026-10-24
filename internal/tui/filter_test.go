package tui

import "testing"

func TestFilterRowsPinnedEmptyQuery(t *testing.T) {
	rows := []Row{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta", Pinned: true},
		{ID: "c", Label: "Gamma"},
	}
	got := FilterRows(rows, "")
	if got[0].ID != "b" || got[1].ID != "a" || got[2].ID != "c" {
		t.Fatalf("%#v", got)
	}
}

func TestFilterRowsPinnedTie(t *testing.T) {
	rows := []Row{
		{ID: "other", Label: "Todo App"},
		{ID: "cwd", Label: "Todo App", Pinned: true},
	}
	got := FilterRows(rows, "Todo App")
	if len(got) != 2 {
		t.Fatal(len(got))
	}
	if got[0].ID != "cwd" {
		t.Fatalf("pinned tie should win, got %#v", got)
	}
}

func TestFilterRowsNoMatch(t *testing.T) {
	got := FilterRows([]Row{{ID: "a", Label: "Alpha"}}, "zzz")
	if len(got) != 0 {
		t.Fatalf("%#v", got)
	}
}
