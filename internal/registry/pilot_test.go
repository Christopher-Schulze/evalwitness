package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func TestPilotReadiness(t *testing.T) {
	// Create capsule file and verified entry
	f, err := os.CreateTemp("", "pilot-capsule-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(f.Name()); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove pilot capsule fixture: %v", err)
		}
	}()
	content := []byte("pilot-capsule-content")
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(content)
	d := hex.EncodeToString(h[:])

	e, err := Verify(Entry{Provider: "a", Digest: d}, f.Name())
	if err != nil {
		t.Fatalf("verify %v", err)
	}
	var m Matrix
	m.Add(e)
	p := Pilot{CapsulePath: f.Name(), Verified: true}
	if err := IsReady(p, m); err != nil {
		t.Fatalf("ready %v", err)
	}
	p2 := Pilot{CapsulePath: "", Verified: true}
	if err := IsReady(p2, m); err == nil {
		t.Fatal("expected error on empty path")
	}
	var empty Matrix
	if err := IsReady(p, empty); err == nil {
		t.Fatal("expected error on unverified matrix")
	}
	// Wrong capsule must fail
	f2, err := os.CreateTemp("", "pilot-wrong-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(f2.Name()); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove wrong pilot fixture: %v", err)
		}
	}()
	if _, err := f2.Write([]byte("wrong")); err != nil {
		t.Fatal(err)
	}
	if err := f2.Close(); err != nil {
		t.Fatal(err)
	}
	p3 := Pilot{CapsulePath: f2.Name(), Verified: true}
	if err := IsReady(p3, m); err == nil {
		t.Fatal("expected error on wrong capsule")
	}
}
