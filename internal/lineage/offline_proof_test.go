package lineage

import "testing"

func TestOfflineProofTraversesPositiveAndRejectsCounterexample(t *testing.T) {
	proof, err := BuildVerificationLineageOfflineProof("../..")
	if err != nil {
		t.Fatal(err)
	}
	if proof.Positive.TerminalState != StateDirectVerificationInvocation || proof.Counterexample.Classification.TerminalState != StateNonFailableVerification {
		t.Fatalf("offline proof lost its positive or counterexample: %#v", proof)
	}
}

func TestOfflineProofRejectsResealedResearchPromotion(t *testing.T) {
	proof, err := BuildVerificationLineageOfflineProof("../..")
	if err != nil {
		t.Fatal(err)
	}
	proof.ClaimBoundary.EmpiricalAuditRun = true
	proof.ClaimBoundary.ResearchRelease = true
	proof.Digest, err = offlineProofDigest(proof)
	if err != nil {
		t.Fatal(err)
	}
	if err := proof.Validate("../.."); err == nil {
		t.Fatal("resealed development proof was promoted to research evidence")
	}
}
