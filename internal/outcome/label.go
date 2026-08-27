package outcome

import (
	"fmt"
	"io"
)

func SealLabelDraft(draft LabelDraft) (Label, error) {
	return SealLabel(Label{
		PacketID: draft.PacketID, AdjudicatorAlias: draft.AdjudicatorAlias, ReviewerSlot: draft.ReviewerSlot,
		PrimaryOutcome: draft.PrimaryOutcome, TaskSatisfaction: draft.TaskSatisfaction, TechnicalCorrectness: draft.TechnicalCorrectness,
		VerificationQuality: draft.VerificationQuality, HarmfulSideEffects: draft.HarmfulSideEffects, EvidenceSufficiency: draft.EvidenceSufficiency,
		ReasonCodes: append([]ReasonCode(nil), draft.ReasonCodes...), SubmittedAt: draft.SubmittedAt, RubricVersion: draft.RubricVersion,
		QualificationDigest: draft.QualificationDigest, ConflictsOfInterest: append([]string(nil), draft.ConflictsOfInterest...),
	})
}

func DecodeLabelDraft(reader io.Reader) (LabelDraft, error) {
	var value LabelDraft
	if err := decodeStrict(reader, &value); err != nil {
		return LabelDraft{}, fmt.Errorf("decode outcome label draft: %w", err)
	}
	label, err := SealLabelDraft(value)
	if err != nil {
		return LabelDraft{}, err
	}
	return LabelDraft{
		PacketID: label.PacketID, AdjudicatorAlias: label.AdjudicatorAlias, ReviewerSlot: label.ReviewerSlot,
		PrimaryOutcome: label.PrimaryOutcome, TaskSatisfaction: label.TaskSatisfaction, TechnicalCorrectness: label.TechnicalCorrectness,
		VerificationQuality: label.VerificationQuality, HarmfulSideEffects: label.HarmfulSideEffects, EvidenceSufficiency: label.EvidenceSufficiency,
		ReasonCodes: append([]ReasonCode(nil), label.ReasonCodes...), SubmittedAt: label.SubmittedAt, RubricVersion: label.RubricVersion,
		QualificationDigest: label.QualificationDigest, ConflictsOfInterest: append([]string(nil), label.ConflictsOfInterest...),
	}, nil
}
