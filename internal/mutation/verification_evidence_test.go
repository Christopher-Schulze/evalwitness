package mutation

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestVerificationEvidenceChallengeClosesNaturalFalseTargetsAndPreservesControls(t *testing.T) {
	challenge, err := BuildVerificationEvidenceChallenge()
	if err != nil {
		t.Fatal(err)
	}
	want := VerificationEvidenceChallengeSummary{
		Cases: 9, NaturalNegatives: 2, PositiveControls: 2, SyntheticGuards: 5,
		Eligible: 2, Rejected: 7, ProvenanceFailures: 5, FailabilityFailures: 2, EvidenceLossFailures: 7,
	}
	if challenge.Summary != want {
		t.Fatalf("verification-evidence summary = %#v, want %#v", challenge.Summary, want)
	}
	for index, item := range challenge.Cases {
		definition := verificationEvidenceChallengeDefinitions[index]
		if item.Assessment.Status != definition.expectedStatus || !slices.Equal(item.Assessment.RejectionReasons, definition.expectedReasons) {
			t.Fatalf("challenge case %q = status %q reasons %v, want status %q reasons %v", item.ID, item.Assessment.Status, item.Assessment.RejectionReasons, definition.expectedStatus, definition.expectedReasons)
		}
		if item.Category == VerificationChallengeNaturalNegative && item.SourceBinding == nil {
			t.Fatalf("natural negative %q lacks its frozen source binding", item.ID)
		}
	}
}

func TestVerificationEvidenceChallengeCodecSchemaAndTamperRejection(t *testing.T) {
	challenge, err := BuildVerificationEvidenceChallenge()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(challenge)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeVerificationEvidenceChallenge(strings.NewReader(string(encoded)))
	if err != nil || decoded.Digest != challenge.Digest {
		t.Fatalf("verification-evidence challenge round trip failed: %v", err)
	}
	if err := VerifyVerificationEvidenceChallenge(decoded); err != nil {
		t.Fatalf("verification-evidence challenge did not reproduce: %v", err)
	}
	if _, err := Schema("verification-evidence-assessment"); err != nil {
		t.Fatal(err)
	}
	if _, err := Schema("verification-evidence-challenge"); err != nil {
		t.Fatal(err)
	}

	var tampered map[string]any
	if err := json.Unmarshal(encoded, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered["summary"].(map[string]any)["eligible"] = float64(3)
	raw, err := json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeVerificationEvidenceChallenge(strings.NewReader(string(raw))); err == nil {
		t.Fatal("tampered verification-evidence summary was accepted")
	}
	unknown := strings.Replace(string(encoded), `"digest": "`, `"unknown": true, "digest": "`, 1)
	if _, err := DecodeVerificationEvidenceChallenge(strings.NewReader(unknown)); err == nil {
		t.Fatal("unknown verification-evidence field was accepted")
	}
}

func TestVerificationEvidenceAssessmentDigestRejectsTypedTampering(t *testing.T) {
	trajectory, targetEventID, err := verificationEvidenceFixture("go test ./...", "ok  example/project  0.123s", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	assessment, err := AssessVerificationEvidence(trajectory, targetEventID)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Status != VerificationEvidenceEligible {
		t.Fatalf("positive test output was rejected: %#v", assessment)
	}
	encoded, err := EncodeIndented(assessment)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeVerificationEvidenceAssessment(strings.NewReader(string(encoded))); err != nil {
		t.Fatalf("verification-evidence assessment round trip failed: %v", err)
	}
	tampered := strings.Replace(string(encoded), `"verification_failable": true`, `"verification_failable": false`, 1)
	if _, err := DecodeVerificationEvidenceAssessment(strings.NewReader(tampered)); err == nil {
		t.Fatal("tampered verification-evidence assessment was accepted")
	}
	assessment.Status = VerificationEvidenceRejected
	assessment.RejectionReasons = []VerificationEvidenceRejectionReason{VerificationEvidenceNotWeakened}
	assessment.Checks = verificationEvidenceChecks(assessment)
	assessment.Digest, err = assessment.digest()
	if err != nil {
		t.Fatal(err)
	}
	if err := assessment.Validate(); err == nil {
		t.Fatal("self-consistent but semantically false rejection reasons were accepted")
	}
}

func TestComparisonOperandsRejectTautologiesAndAmbiguousShapes(t *testing.T) {
	tests := []struct {
		name       string
		executable string
		arguments  []string
		want       [2]string
		valid      bool
	}{
		{name: "different cmp operands", executable: "cmp", arguments: []string{"-s", "/tmp/a", "/tmp/b"}, want: [2]string{"/tmp/a", "/tmp/b"}, valid: true},
		{name: "same cmp operands remain parseable", executable: "cmp", arguments: []string{"-s", "/tmp/a", "/tmp/a"}, want: [2]string{"/tmp/a", "/tmp/a"}, valid: true},
		{name: "diff option value", executable: "diff", arguments: []string{"-U", "3", "before", "after"}, want: [2]string{"before", "after"}, valid: true},
		{name: "missing operand", executable: "cmp", arguments: []string{"-s", "/tmp/a"}, valid: false},
		{name: "extra operand", executable: "diff", arguments: []string{"a", "b", "c"}, valid: false},
		{name: "missing option value", executable: "cmp", arguments: []string{"-n"}, valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := comparisonOperands(test.executable, test.arguments)
			if valid != test.valid || got != test.want {
				t.Fatalf("comparison operands = %#v, %t; want %#v, %t", got, valid, test.want, test.valid)
			}
		})
	}
}

func TestVerificationEvidenceGatesFailIndependently(t *testing.T) {
	tests := []struct {
		name    string
		command string
		result  string
		status  string
		exit    *int
		reason  VerificationEvidenceRejectionReason
	}{
		{name: "provenance", command: "go test ./...", result: "I need to inspect this again.", reason: VerificationEvidenceProvenanceUnbound},
		{name: "failability", command: "cmp -s /tmp/a /tmp/a && printf 'equal\\n'", result: "equal\n", reason: VerificationEvidenceNonFailable},
		{name: "exit code survives", command: "go test ./...", result: "ok  example/project  0.123s", exit: intPointer(0), reason: VerificationEvidenceNotWeakened},
		{name: "status survives", command: "go test ./...", result: "ok  example/project  0.123s", status: "success", reason: VerificationEvidenceNotWeakened},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trajectory, targetEventID, err := verificationEvidenceFixture(test.command, test.result, test.status, test.exit)
			if err != nil {
				t.Fatal(err)
			}
			assessment, err := AssessVerificationEvidence(trajectory, targetEventID)
			if err != nil {
				t.Fatal(err)
			}
			if assessment.Status != VerificationEvidenceRejected || !slices.Contains(assessment.RejectionReasons, test.reason) {
				t.Fatalf("gate %q did not fail closed: %#v", test.reason, assessment)
			}
		})
	}
}
