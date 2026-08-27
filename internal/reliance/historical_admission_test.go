package reliance

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestSealedHistoricalConstructDefectsCannotProduceRelianceAdmission(t *testing.T) {
	file, err := os.Open("../../eval/governance/construct-repair-evidence-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	evidence, err := mutation.DecodeConstructRepairEvidence(file)
	if err != nil {
		t.Fatal(err)
	}
	rejections, err := stress.HistoricalConstructRejections(evidence)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{
		"generic_completion_evidence_role",
		"pathological_executable_text_presentation",
		"shared_tool_transaction_reorder",
	}
	gotIDs := make([]string, len(rejections))
	for index, rejection := range rejections {
		gotIDs[index] = rejection.CaseID
		assertHistoricalConstructAdmissionRejected(t, evidence, rejection)
	}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("historical construct IDs = %v, want %v", gotIDs, wantIDs)
	}
}

func assertHistoricalConstructAdmissionRejected(
	t *testing.T,
	evidence mutation.ConstructRepairEvidence,
	rejection stress.HistoricalConstructRejection,
) {
	t.Helper()
	spec := relationAdmissionStressRelationForFamily(t, stress.EstimandSensitivity, rejection.Family)
	_, err := stress.AdmitHistoricalConstructCase(spec, evidence, rejection.CaseID)
	var admissionErr *stress.AdmissionError
	if !errors.As(err, &admissionErr) || admissionErr.State != stress.InvalidCrossVersion {
		t.Fatalf("historical construct %q admission error = %v", rejection.CaseID, err)
	}
}

func TestRelationBackedAdmissionRejectsCrossVersionAndMissingFirewallReplay(t *testing.T) {
	base := relationAdmissionFixture(t, EstimandEvidenceOnly, true)
	valid := relationInputsWithStatus(t, base, stress.EstimandSensitivity, stress.AdmissionHumanSupported)
	tests := []struct {
		name   string
		mutate func(*stress.ReplayedRelationCaseV3)
	}{
		{"v1 program", func(value *stress.ReplayedRelationCaseV3) {
			value.MutationProgramVersion = mutation.MutationProgramVersionV1
		}},
		{"v2 relation", func(value *stress.ReplayedRelationCaseV3) {
			value.RelationContractVersion = mutation.RelationContractVersionV2
		}},
		{"missing firewall", func(value *stress.ReplayedRelationCaseV3) { value.ConstructFirewallDigest = "" }},
		{"foreign witness", func(value *stress.ReplayedRelationCaseV3) {
			value.FormalWitnessDigest = protocolkit.DigestBytes([]byte("foreign-witness"))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputs := valid
			test.mutate(&inputs.ReplayedRelationCase)
			_, err := BindRelationBackedIntervention(inputs)
			assertInterventionErrorCode(t, err, FailureRelationEvidenceInvalid)
		})
	}
}
