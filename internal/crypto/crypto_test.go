package crypto

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	pass := "test-passphrase"
	key, recovery, err := Create(dir, pass, 12)
	if err != nil {
		t.Fatal(err)
	}
	if recovery == "" {
		t.Fatal("expected recovery key")
	}
	got, err := UnlockPassphrase(dir, pass)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, key) {
		t.Fatal("passphrase unlock mismatch")
	}
	got, err = UnlockRecovery(dir, recovery)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, key) {
		t.Fatal("recovery unlock mismatch")
	}
	if _, err := UnlockPassphrase(dir, "wrong-passphrase"); err == nil {
		t.Fatal("expected wrong passphrase to fail")
	}
	plain := []byte("APP_KEY=secret\n")
	blob, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Open(key, blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("got %q", out)
	}
}

func TestShortPassphrase(t *testing.T) {
	if _, _, err := Create(t.TempDir(), "short", 12); err == nil {
		t.Fatal("expected short passphrase to fail")
	}
}
