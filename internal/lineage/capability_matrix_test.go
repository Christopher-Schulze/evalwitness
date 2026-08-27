package lineage

import "testing"

func TestCapabilityMatrixSeparatesSpecificationFromObservation(t *testing.T) {
	matrix, err := BuildVerificationLineageCapabilityMatrix()
	if err != nil {
		t.Fatal(err)
	}
	claude := matrix.Vectors[0]
	if claude.Format != "claude_code_jsonl" {
		t.Fatalf("unexpected first format %q", claude.Format)
	}
	for _, field := range claude.Fields {
		if field.RepresentableByFormat != CapabilityUnspecified {
			t.Fatalf("Claude development observation promoted format capability for %s", field.Field)
		}
	}
	if matrix.ClaimBoundary.FormatWideClaim || matrix.ClaimBoundary.EmpiricalPrevalence {
		t.Fatal("development capability matrix promoted a research claim")
	}
}

func TestCapabilityMatrixRejectsResealedFormatWidePromotion(t *testing.T) {
	matrix, err := BuildVerificationLineageCapabilityMatrix()
	if err != nil {
		t.Fatal(err)
	}
	matrix.ClaimBoundary.FormatWideClaim = true
	matrix.Digest, err = capabilityMatrixDigest(matrix)
	if err != nil {
		t.Fatal(err)
	}
	if err := matrix.Validate(); err == nil {
		t.Fatal("resealed format-wide capability promotion was accepted")
	}
}

func TestCapabilityMatrixRejectsCrossFormatVectorSubstitution(t *testing.T) {
	matrix, err := BuildVerificationLineageCapabilityMatrix()
	if err != nil {
		t.Fatal(err)
	}
	matrix.Vectors[0], matrix.Vectors[1] = matrix.Vectors[1], matrix.Vectors[0]
	matrix.Digest, err = capabilityMatrixDigest(matrix)
	if err != nil {
		t.Fatal(err)
	}
	if err := matrix.Validate(); err == nil {
		t.Fatal("cross-format capability-vector substitution was accepted")
	}
}
