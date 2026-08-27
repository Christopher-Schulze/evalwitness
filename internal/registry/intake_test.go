package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func validEntry(id string) IntakeEntry {
	now := time.Now()
	capSum := sha256.Sum256([]byte("capsule-" + id))
	profSum := sha256.Sum256([]byte("profile-" + id))
	capDigest := hex.EncodeToString(capSum[:])
	profDigest := hex.EncodeToString(profSum[:])
	nonceSum := sha256.Sum256([]byte("nonce-" + id))
	return IntakeEntry{
		EntryID:            id,
		Submitter:          "test-contributor",
		CapsuleID:          "capsule-" + id,
		CapsuleDigest:      capDigest,
		ProfileDigest:      profDigest,
		ChallengeNonce:     hex.EncodeToString(nonceSum[:]),
		RequestContract:    "evalwitness.verifier.prompt.v1",
		ScorePolicy:        "evalwitness.strict-score-policy.v1",
		TraceMapping:       "evalwitness.trace-mapping.v1",
		SchemaVersion:      2,
		EndpointKind:       "chat_completions",
		ThinkingMode:       "disabled",
		Temperature:        1.0,
		MaxOutputTokens:    4096,
		TopLogprobs:        20,
		ScoreAlphabet:      "ABCDEFGHIJKLMNOPQRST",
		EvidenceLevel:      "E1",
		ObservedAt:         now.Add(-24 * time.Hour),
		ExpiresAt:          now.Add(24 * time.Hour),
		ServedModel:        "deepseek-v4-flash",
		Status:             IntakeStatusFormatVerified,
		License:            "Apache-2.0",
		PrivacyClass:       "public",
		PublicReleaseOK:    true,
		ContaminationFree:  true,
		CommunityValidated: false,
	}
}

func TestValidateIntakeAcceptsValidEntry(t *testing.T) {
	e := validEntry("test-1")
	if err := ValidateIntake(e); err != nil {
		t.Fatalf("valid entry rejected: %v", err)
	}
}

func TestValidateIntakeRejectsEmptyID(t *testing.T) {
	e := validEntry("")
	if err := ValidateIntake(e); err == nil {
		t.Fatal("empty entry_id accepted")
	}
}

func TestValidateIntakeRejectsShortDigest(t *testing.T) {
	e := validEntry("test-1")
	e.CapsuleDigest = "short"
	if err := ValidateIntake(e); err == nil {
		t.Fatal("short capsule_digest accepted")
	}
}

func TestValidateIntakeRejectsLowTopK(t *testing.T) {
	e := validEntry("test-1")
	e.TopLogprobs = 5
	if err := ValidateIntake(e); err == nil {
		t.Fatal("top_logprobs < 20 accepted")
	}
}

func TestValidateIntakeRejectsExpired(t *testing.T) {
	e := validEntry("test-1")
	e.ExpiresAt = e.ObservedAt.Add(-time.Hour)
	if err := ValidateIntake(e); err == nil {
		t.Fatal("expires_at before observed_at accepted")
	}
}

func TestValidateIntakeRejectsNoRelease(t *testing.T) {
	e := validEntry("test-1")
	e.PublicReleaseOK = false
	if err := ValidateIntake(e); err == nil {
		t.Fatal("public_release_allowed=false accepted")
	}
}

func TestValidateIntakeRejectsContaminated(t *testing.T) {
	e := validEntry("test-1")
	e.ContaminationFree = false
	if err := ValidateIntake(e); err == nil {
		t.Fatal("contamination_free=false accepted")
	}
}

func TestIntakeValidatorAddAndCount(t *testing.T) {
	v := NewIntakeValidator()
	e := validEntry("test-1")
	if err := v.Add(e); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if v.Count() != 1 {
		t.Fatalf("count = %d, want 1", v.Count())
	}
	// Duplicate entry_id
	if err := v.Add(e); err == nil {
		t.Fatal("duplicate entry_id accepted")
	}
}

func TestIntakeValidatorDuplicateCapsuleRejected(t *testing.T) {
	v := NewIntakeValidator()
	e1 := validEntry("test-1")
	e2 := validEntry("test-2")
	e2.CapsuleDigest = e1.CapsuleDigest
	e2.ProfileDigest = e1.ProfileDigest
	if err := v.Add(e1); err != nil {
		t.Fatal(err)
	}
	if err := v.Add(e2); err == nil {
		t.Fatal("duplicate capsule+profile pair accepted")
	}
}

func TestIntakeReportDeterministic(t *testing.T) {
	v := NewIntakeValidator()
	for i := range 3 {
		e := validEntry(string(rune('a' + i)))
		if err := v.Add(e); err != nil {
			t.Fatal(err)
		}
	}
	r := v.Report()
	if r.TotalEntries != 3 {
		t.Fatalf("total = %d, want 3", r.TotalEntries)
	}
	if len(r.Submitters) != 1 {
		t.Fatalf("submitters = %d, want 1", len(r.Submitters))
	}
}

func TestIntakeValidatorRejectsExpiredAndReplayedNonce(t *testing.T) {
	v := NewIntakeValidator()
	fixed := time.Date(2026, 8, 24, 15, 0, 0, 0, time.UTC)
	v.nowFunc = func() time.Time { return fixed }
	expired := validEntry("expired")
	expired.ObservedAt = fixed.Add(-48 * time.Hour)
	expired.ExpiresAt = fixed.Add(-time.Hour)
	if err := v.Add(expired); err == nil {
		t.Fatal("expired entry accepted")
	}
	first := validEntry("fresh")
	first.ObservedAt = fixed.Add(-time.Hour)
	first.ExpiresAt = fixed.Add(time.Hour)
	if err := v.Add(first); err != nil {
		t.Fatal(err)
	}
	replay := validEntry("replay")
	replay.ObservedAt = first.ObservedAt
	replay.ExpiresAt = first.ExpiresAt
	replay.ChallengeNonce = first.ChallengeNonce
	if err := v.Add(replay); err == nil {
		t.Fatal("replayed challenge_nonce accepted")
	}
}

func TestValidateIntakeRejectsNonHexDigest(t *testing.T) {
	e := validEntry("test-1")
	e.CapsuleDigest = strings.Repeat("z", 64)
	if err := ValidateIntake(e); err == nil {
		t.Fatal("non-hex capsule_digest accepted")
	}
}

func TestValidateIntakeRejectsMarkupAndTraversal(t *testing.T) {
	script := validEntry("markup")
	script.Submitter = "<script>alert(1)</script>"
	if err := ValidateIntake(script); err == nil {
		t.Fatal("markup submitter accepted")
	}
	traverse := validEntry("traverse")
	traverse.CapsuleID = "../private/capsule"
	if err := ValidateIntake(traverse); err == nil {
		t.Fatal("path-traversal capsule_id accepted")
	}
	uri := validEntry("uri")
	uri.ServedModel = "javascript:alert(1)"
	if err := ValidateIntake(uri); err == nil {
		t.Fatal("javascript URI served_model accepted")
	}
}

func TestValidateIntakeRejectsOversizedText(t *testing.T) {
	entry := validEntry("huge")
	entry.Submitter = strings.Repeat("a", MaxIntakeTextRunes+1)
	if err := ValidateIntake(entry); err == nil {
		t.Fatal("oversized submitter accepted")
	}
}

func TestValidateIntakeRejectsPromotedStatusAndCommunityClaim(t *testing.T) {
	promoted := validEntry("promoted")
	promoted.Status = "independently_reproduced"
	if err := ValidateIntake(promoted); err == nil {
		t.Fatal("independently_reproduced intake accepted")
	}
	claimed := validEntry("claimed")
	claimed.CommunityValidated = true
	if err := ValidateIntake(claimed); err == nil {
		t.Fatal("community_validated=true accepted")
	}
}
