package watch

import "testing"

func TestDecideDrift(t *testing.T) {
	notify, clear := DecideDrift("abc", "abc", "")
	if notify || clear {
		t.Fatalf("in sync: notify=%v clear=%v", notify, clear)
	}
	notify, clear = DecideDrift("abc", "def", "")
	if !notify || clear {
		t.Fatalf("drift: notify=%v clear=%v", notify, clear)
	}
	notify, clear = DecideDrift("abc", "def", "def")
	if notify || clear {
		t.Fatalf("already notified: notify=%v clear=%v", notify, clear)
	}
	notify, clear = DecideDrift("abc", "abc", "def")
	if notify || !clear {
		t.Fatalf("resolved: notify=%v clear=%v", notify, clear)
	}
	notify, clear = DecideDrift("", "abc", "")
	if !notify || clear {
		t.Fatalf("unsaved: notify=%v clear=%v", notify, clear)
	}
}

func TestDecideIgnore(t *testing.T) {
	notify, clear := DecideIgnore("unignored\nx", "", true)
	if !notify || clear {
		t.Fatal(notify, clear)
	}
	notify, clear = DecideIgnore("unignored\nx", "unignored\nx", true)
	if notify || clear {
		t.Fatal(notify, clear)
	}
	notify, clear = DecideIgnore("ignored\nok", "unignored\nx", false)
	if notify || !clear {
		t.Fatal(notify, clear)
	}
}
