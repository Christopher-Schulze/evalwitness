package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func TestMatrixNoRanking(t *testing.T) {
	// Create two capsule files with known digests
	f1, err := os.CreateTemp("", "capsule-a-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(f1.Name()); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove capsule-a fixture: %v", err)
		}
	}()
	content1 := []byte("capsule-a-content")
	if _, err := f1.Write(content1); err != nil {
		t.Fatal(err)
	}
	if err := f1.Close(); err != nil {
		t.Fatal(err)
	}
	h1 := sha256.Sum256(content1)
	d1 := hex.EncodeToString(h1[:])

	f2, err := os.CreateTemp("", "capsule-b-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(f2.Name()); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove capsule-b fixture: %v", err)
		}
	}()
	content2 := []byte("capsule-b-content")
	if _, err := f2.Write(content2); err != nil {
		t.Fatal(err)
	}
	if err := f2.Close(); err != nil {
		t.Fatal(err)
	}
	h2 := sha256.Sum256(content2)
	d2 := hex.EncodeToString(h2[:])

	var m Matrix
	e1, err := Verify(Entry{Provider: "a", Digest: d1}, f1.Name())
	if err != nil {
		t.Fatalf("verify1 %v", err)
	}
	e2, err := Verify(Entry{Provider: "b", Digest: d2}, f2.Name())
	if err != nil {
		t.Fatalf("verify2 %v", err)
	}
	m.Add(e2)
	m.Add(e1)
	if len(m.Entries) != 2 {
		t.Fatalf("len %d", len(m.Entries))
	}
	// Sorted by digest
	if m.Entries[0].Digest > m.Entries[1].Digest {
		t.Fatalf("not sorted %v", m.Entries)
	}
	if !m.Entries[0].Verified {
		t.Fatal("not verified")
	}
	// Bad digest must fail
	if _, err := Verify(Entry{Provider: "x", Digest: "bad"}, f1.Name()); err == nil {
		t.Fatal("expected error on bad digest")
	}
	// Wrong file must fail
	if _, err := Verify(Entry{Provider: "a", Digest: d1}, f2.Name()); err == nil {
		t.Fatal("expected digest mismatch")
	}
}
