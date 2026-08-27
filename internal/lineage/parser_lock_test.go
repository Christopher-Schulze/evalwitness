package lineage

import (
	"strings"
	"testing"
)

func TestParserLockBindsCurrentSourceMappingsAndExecutedDevelopmentVectors(t *testing.T) {
	lock, err := BuildVerificationLineageParserLock("../..")
	if err != nil {
		t.Fatal(err)
	}
	if lock.DevelopmentVectors != 63 || lock.ConformanceChecks != 504 || lock.ConformanceFailures != 0 || len(lock.SourceBindings) != 24 || len(lock.ArtifactBindings) != 6 {
		t.Fatalf("unexpected parser lock surface: %#v", lock)
	}
	if err := VerifyVerificationLineageParserLock("../..", lock); err != nil {
		t.Fatal(err)
	}
}

func TestParserLockVerificationRejectsResealedSourceSubstitution(t *testing.T) {
	lock, err := BuildVerificationLineageParserLock("../..")
	if err != nil {
		t.Fatal(err)
	}
	lock.SourceBindings = append([]ParserLockSourceBinding(nil), lock.SourceBindings...)
	lock.SourceBindings[0].SHA256 = strings.Repeat("9", 64)
	lock.Digest, err = parserLockDigest(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Validate(); err != nil {
		t.Fatal("resealed lock should reach live source verification:", err)
	}
	if err := VerifyVerificationLineageParserLock("../..", lock); err == nil {
		t.Fatal("resealed source substitution matched the live parser lock")
	}
}

func TestParserLockRejectsCalibrationInspectionAndPostLockRuleWeakening(t *testing.T) {
	lock, err := BuildVerificationLineageParserLock("../..")
	if err != nil {
		t.Fatal(err)
	}
	lock.CalibrationResultsInspected = true
	lock.Digest, err = parserLockDigest(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Validate(); err == nil {
		t.Fatal("post-calibration parser lock was accepted as pre-calibration")
	}
}
