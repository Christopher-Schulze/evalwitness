package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func digestOf(label string) string {
	sum := sha256.Sum256([]byte(label))
	return hex.EncodeToString(sum[:])
}

func withReliance(entry IntakeEntry, suffix string) IntakeEntry {
	entry.RelianceOntologyDigest = digestOf("ontology-" + suffix)
	entry.ReliancePanelDigest = digestOf("panel-" + suffix)
	entry.RelianceEstimatorDigest = digestOf("estimator-" + suffix)
	entry.RelianceInterventionDigest = digestOf("intervention-" + suffix)
	entry.RelianceOutcomeDigest = digestOf("outcome-" + suffix)
	entry.RelianceProfileDigest = digestOf("profile-" + suffix)
	return entry
}

func TestValidateIntakeRejectsPartialRelianceParents(t *testing.T) {
	entry := validEntry("partial")
	entry.RelianceOntologyDigest = digestOf("only-ontology")
	if err := ValidateIntake(entry); err == nil {
		t.Fatal("partial reliance parents accepted")
	}
}

func TestRenderRelianceIndexKeepsIncompatibleCellsSeparate(t *testing.T) {
	sameA := withReliance(validEntry("same-a"), "v1")
	sameB := withReliance(validEntry("same-b"), "v1")
	other := withReliance(validEntry("other"), "v2")
	bare := validEntry("bare")
	index, err := RenderRelianceIndex([]IntakeEntry{sameA, other, sameB, bare})
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Cells) != 2 {
		t.Fatalf("cells = %d, want 2", len(index.Cells))
	}
	if len(index.Omitted) != 1 || index.Omitted[0] != "bare" {
		t.Fatalf("omitted = %#v", index.Omitted)
	}
	if len(index.Cells[0].Entries)+len(index.Cells[1].Entries) != 3 {
		t.Fatalf("indexed members = %#v", index.Cells)
	}
	for _, limitation := range index.Limitations {
		if limitation == "" {
			t.Fatal("empty limitation")
		}
	}
}
