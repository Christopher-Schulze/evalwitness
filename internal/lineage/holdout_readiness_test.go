package lineage

import "testing"

func TestHoldoutReadinessRefusesRetrospectiveTransferClaim(t *testing.T) {
	audit, err := BuildVerificationLineageHoldoutReadinessAudit("../..")
	if err != nil {
		t.Fatal(err)
	}
	if audit.ClaimBoundary.HoldoutExecuted || audit.ClaimBoundary.TransferClaim || audit.Findings[0].SelectedID != "claude_code_jsonl" {
		t.Fatalf("holdout readiness promoted contaminated development evidence: %#v", audit)
	}
}

func TestHoldoutReadinessRejectsResealedOutcomePromotion(t *testing.T) {
	audit, err := BuildVerificationLineageHoldoutReadinessAudit("../..")
	if err != nil {
		t.Fatal(err)
	}
	audit.Findings[0].Predictions = 1
	audit.Findings[0].LabeledOutcomes = 1
	audit.ClaimBoundary.HoldoutExecuted = true
	audit.ClaimBoundary.TransferClaim = true
	audit.Digest, err = holdoutReadinessAuditDigest(audit)
	if err != nil {
		t.Fatal(err)
	}
	if err := audit.Validate("../.."); err == nil {
		t.Fatal("resealed retrospective holdout claim was accepted")
	}
}

func TestHoldoutReadinessRejectsSelectionSubstitution(t *testing.T) {
	audit, err := BuildVerificationLineageHoldoutReadinessAudit("../..")
	if err != nil {
		t.Fatal(err)
	}
	audit.Findings[0].SelectedID = "opencode_export_json"
	audit.Findings[0].Candidates[0].Selected = false
	audit.Findings[0].Candidates[2].Selected = true
	audit.Digest, err = holdoutReadinessAuditDigest(audit)
	if err != nil {
		t.Fatal(err)
	}
	if err := audit.Validate("../.."); err == nil {
		t.Fatal("resealed format-holdout selection substitution was accepted")
	}
}
