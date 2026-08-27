package stress

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

type HistoricalConstructRejection struct {
	EvidenceDigest    string
	CaseID            string
	Family            mutation.Family
	LegacyProgram     string
	CorrectedProgram  string
	SourceDigest      string
	CorrectedFirewall string
	ClosedReasons     []mutation.ConstructRejectionReason
}

func HistoricalConstructRejections(evidence mutation.ConstructRepairEvidence) ([]HistoricalConstructRejection, error) {
	if err := evidence.Validate(); err != nil {
		return nil, fmt.Errorf("validate historical construct-repair evidence: %w", err)
	}
	result := make([]HistoricalConstructRejection, len(evidence.Cases))
	for index, item := range evidence.Cases {
		if item.LegacyManifest.Program.Version != mutation.MutationProgramVersionV1 ||
			item.CorrectedFirewall.ProgramVersion != mutation.MutationProgramVersionV2 ||
			item.CorrectedFirewall.Status != mutation.ConstructRejected {
			return nil, fmt.Errorf("historical construct-repair case %q is not one v1 acceptance and v2 closed rejection", item.ID)
		}
		result[index] = HistoricalConstructRejection{
			EvidenceDigest: evidence.Digest, CaseID: item.ID, Family: item.Family,
			LegacyProgram: item.LegacyManifest.Program.Version, CorrectedProgram: item.CorrectedFirewall.ProgramVersion,
			SourceDigest: item.SourceTrajectoryDigest, CorrectedFirewall: item.CorrectedFirewall.Digest,
			ClosedReasons: slices.Clone(item.ExpectedRejectionReasons),
		}
	}
	return result, nil
}

func AdmitHistoricalConstructCase(spec Relation, evidence mutation.ConstructRepairEvidence, caseID string) (ConstructAdmission, error) {
	if err := spec.Validate(); err != nil {
		return ConstructAdmission{}, &AdmissionError{State: InvalidCrossVersion, Reason: err.Error()}
	}
	rejections, err := HistoricalConstructRejections(evidence)
	if err != nil {
		return ConstructAdmission{}, &AdmissionError{State: InvalidFormalWitness, Reason: err.Error()}
	}
	for _, rejection := range rejections {
		if rejection.CaseID != caseID {
			continue
		}
		if spec.Transform.Kind != TransformMutation || spec.Transform.MutationFamily != rejection.Family {
			return ConstructAdmission{}, &AdmissionError{State: InvalidCrossVersion, Reason: "historical fixture family differs from the registered relation"}
		}
		return ConstructAdmission{}, &AdmissionError{
			State:  InvalidCrossVersion,
			Reason: fmt.Sprintf("sealed historical fixture %q was accepted only by %s and rejected by %s under %v; it cannot enter the v3 stress corpus", rejection.CaseID, rejection.LegacyProgram, rejection.CorrectedProgram, rejection.ClosedReasons),
		}
	}
	return ConstructAdmission{}, &AdmissionError{State: InvalidNotApplicable, Reason: fmt.Sprintf("historical construct-repair evidence has no case %q", caseID)}
}

func VerifyV3ConstructChallenge(evidence mutation.ConstructChallengeEvidence) error {
	if err := mutation.VerifyConstructChallengeEvidence(evidence); err != nil {
		return fmt.Errorf("reproduce v3 construct challenge: %w", err)
	}
	if evidence.Summary.V2FalseAcceptances == 0 || evidence.Summary.V3RepairedNegatives != evidence.Summary.V2FalseAcceptances {
		return errors.New("v3 construct challenge does not close every recorded v2 false acceptance")
	}
	return nil
}
