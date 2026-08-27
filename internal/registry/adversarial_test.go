package registry

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
)

func TestRejectArchivePayload(t *testing.T) {
	if err := RejectArchivePayload("capsule.tar.gz", []byte("not-an-archive")); err == nil {
		t.Fatal("tar.gz name accepted")
	}
	if err := RejectArchivePayload("entry.json", append([]byte{'P', 'K', 0x03, 0x04}, []byte("zip")...)); err == nil {
		t.Fatal("zip magic accepted")
	}
	if err := RejectArchivePayload("entry.json", []byte(`{"entry_id":"ok"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestRejectUnboundedAllocation(t *testing.T) {
	if err := RejectUnboundedAllocation(MaxIntakePayloadBytes + 1); err == nil {
		t.Fatal("oversized payload accepted")
	}
	if err := RejectUnboundedAllocation(32); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyDetachedEd25519RejectsWrongKey(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("intake-entry")
	signature := ed25519.Sign(private, message)
	if err := VerifyDetachedEd25519(public, message, signature); err != nil {
		t.Fatal(err)
	}
	wrongPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyDetachedEd25519(wrongPublic, message, signature); err == nil {
		t.Fatal("wrong key accepted")
	}
}

func TestSchemaMigrationAndPathTraversalStayClosed(t *testing.T) {
	if err := ValidateSchemaMigration(SchemaMigration{FromSchema: 2, ToSchema: 1}); err == nil {
		t.Fatal("downgrade rewrite accepted")
	}
	if err := RejectArchivePayload("../secret.zip", nil); err == nil {
		t.Fatal("path-traversal archive name accepted")
	}
	if !strings.Contains(RejectArchivePayload("x.zip", nil).Error(), "archive") {
		t.Fatal("expected archive refusal")
	}
}
