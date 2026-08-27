package relation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func DecodePlan(reader io.Reader) (Plan, error) {
	var value Plan
	if err := decodeStrict(reader, &value); err != nil {
		return Plan{}, fmt.Errorf("decode relation audit plan: %w", err)
	}
	return value, value.Validate()
}

func DecodePlanV3(reader io.Reader) (RelationPlanV3, error) {
	var value RelationPlanV3
	if err := decodeStrict(reader, &value); err != nil {
		return RelationPlanV3{}, fmt.Errorf("decode v3 relation audit plan: %w", err)
	}
	return value, value.Validate()
}

// DecodeReviewPlan accepts a public v1/v2 plan or the public v3 governance plan
// and returns the internal plan view consumed by reviewer-workflow commands.
func DecodeReviewPlan(reader io.Reader) (Plan, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, MaximumDocumentSize+1))
	if err != nil {
		return Plan{}, fmt.Errorf("read relation review plan: %w", err)
	}
	if len(raw) > MaximumDocumentSize {
		return Plan{}, errors.New("relation review plan exceeds the maximum document size")
	}
	var identity struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil {
		return Plan{}, fmt.Errorf("decode relation review plan identity: %w", err)
	}
	if identity.SchemaVersion == PlanSchemaVersionV3 {
		governed, decodeErr := DecodePlanV3(bytes.NewReader(raw))
		if decodeErr != nil {
			return Plan{}, decodeErr
		}
		return ReviewPlanV3(governed)
	}
	return DecodePlan(bytes.NewReader(raw))
}

func DecodePrimarySampleV3(reader io.Reader, plan RelationPlanV3) (PrimarySampleV3, error) {
	var value PrimarySampleV3
	if err := decodeStrict(reader, &value); err != nil {
		return PrimarySampleV3{}, fmt.Errorf("decode v3 relation primary sample: %w", err)
	}
	return value, value.Validate(plan)
}

func DecodeScarcitySentinelV3(reader io.Reader, plan RelationPlanV3, primary PrimarySampleV3) (ScarcitySentinelV3, error) {
	var value ScarcitySentinelV3
	if err := decodeStrict(reader, &value); err != nil {
		return ScarcitySentinelV3{}, fmt.Errorf("decode v3 relation scarcity sentinel: %w", err)
	}
	return value, value.Validate(plan, primary)
}

func DecodeScarcityPublicEvidence(reader io.Reader) (ScarcityPublicEvidence, error) {
	var value ScarcityPublicEvidence
	if err := decodeStrict(reader, &value); err != nil {
		return ScarcityPublicEvidence{}, fmt.Errorf("decode relation scarcity public evidence: %w", err)
	}
	return value, value.Validate()
}

func DecodeOwnerInspectionPublicAttestation(reader io.Reader) (OwnerInspectionPublicAttestation, error) {
	var value OwnerInspectionPublicAttestation
	if err := decodeStrict(reader, &value); err != nil {
		return OwnerInspectionPublicAttestation{}, fmt.Errorf("decode relation owner-inspection public attestation: %w", err)
	}
	return value, value.Validate()
}

func DecodePilotSampleV3(reader io.Reader, plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3) (PilotSampleV3, error) {
	var value PilotSampleV3
	if err := decodeStrict(reader, &value); err != nil {
		return PilotSampleV3{}, fmt.Errorf("decode v3 relation pilot sample: %w", err)
	}
	return value, value.Validate(plan, primary, sentinel)
}

func DecodeStudyAmendmentV3(reader io.Reader, plan RelationPlanV3, pilot PilotSampleV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3) (StudyAmendmentV3, error) {
	var value StudyAmendmentV3
	if err := decodeStrict(reader, &value); err != nil {
		return StudyAmendmentV3{}, fmt.Errorf("decode v3 relation study amendment: %w", err)
	}
	return value, value.Validate(plan, pilot, primary, sentinel)
}

func DecodePrimarySample(reader io.Reader) (PrimarySample, error) {
	var value PrimarySample
	if err := decodeStrict(reader, &value); err != nil {
		return PrimarySample{}, fmt.Errorf("decode relation primary sample: %w", err)
	}
	return value, value.Validate()
}

func DecodePilotSample(reader io.Reader) (PilotSample, error) {
	var value PilotSample
	if err := decodeStrict(reader, &value); err != nil {
		return PilotSample{}, fmt.Errorf("decode relation pilot sample: %w", err)
	}
	return value, value.Validate()
}

func DecodeRelationPilotReadiness(reader io.Reader) (RelationPilotReadiness, error) {
	var value RelationPilotReadiness
	if err := decodeStrict(reader, &value); err != nil {
		return RelationPilotReadiness{}, fmt.Errorf("decode relation pilot readiness: %w", err)
	}
	return value, value.Validate()
}

func DecodePilotChangeReceipt(reader io.Reader) (PilotChangeReceipt, error) {
	var value PilotChangeReceipt
	if err := decodeStrict(reader, &value); err != nil {
		return PilotChangeReceipt{}, fmt.Errorf("decode relation pilot change receipt: %w", err)
	}
	return value, value.Validate()
}

func DecodePilotInspectionDecisionDrafts(reader io.Reader) ([]PilotInspectionDecisionDraft, error) {
	var value []PilotInspectionDecisionDraft
	if err := decodeStrict(reader, &value); err != nil {
		return nil, fmt.Errorf("decode relation pilot inspection decisions: %w", err)
	}
	return value, nil
}

func DecodePilotInspectionRecord(reader io.Reader) (PilotInspectionRecord, error) {
	var value PilotInspectionRecord
	if err := decodeStrict(reader, &value); err != nil {
		return PilotInspectionRecord{}, fmt.Errorf("decode relation pilot inspection record: %w", err)
	}
	return value, value.Validate()
}

func DecodePilotLaunchDossier(reader io.Reader) (PilotLaunchDossier, error) {
	var value PilotLaunchDossier
	if err := decodeStrict(reader, &value); err != nil {
		return PilotLaunchDossier{}, fmt.Errorf("decode relation pilot launch dossier: %w", err)
	}
	return value, value.Validate()
}

func DecodeObservations(reader io.Reader) ([]AxisObservation, error) {
	var value []AxisObservation
	if err := decodeStrict(reader, &value); err != nil {
		return nil, fmt.Errorf("decode normalized relation observations: %w", err)
	}
	return value, nil
}

func DecodeTranslationResult(reader io.Reader) (TranslationResult, error) {
	var value TranslationResult
	if err := decodeStrict(reader, &value); err != nil {
		return TranslationResult{}, fmt.Errorf("decode relation translation result: %w", err)
	}
	return value, value.Validate()
}

func DecodeReplayReceipt(reader io.Reader) (ReplayReceipt, error) {
	var value ReplayReceipt
	if err := decodeStrict(reader, &value); err != nil {
		return ReplayReceipt{}, fmt.Errorf("decode relation replay receipt: %w", err)
	}
	return value, value.Validate()
}

func DecodeCaseMaterial(reader io.Reader) (CaseMaterial, error) {
	var value CaseMaterial
	if err := decodeStrict(reader, &value); err != nil {
		return CaseMaterial{}, fmt.Errorf("decode relation case material: %w", err)
	}
	return value, value.Validate()
}

func DecodeBlindPacket(reader io.Reader) (BlindPacket, error) {
	var value BlindPacket
	if err := decodeStrict(reader, &value); err != nil {
		return BlindPacket{}, fmt.Errorf("decode relation blind packet: %w", err)
	}
	return value, value.Validate()
}

func DecodePrivateMapping(reader io.Reader) (PrivateMapping, error) {
	var value PrivateMapping
	if err := decodeStrict(reader, &value); err != nil {
		return PrivateMapping{}, fmt.Errorf("decode relation private mapping: %w", err)
	}
	return value, value.Validate()
}

func DecodeReviewerHandbook(reader io.Reader) (ReviewerHandbook, error) {
	var value ReviewerHandbook
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewerHandbook{}, fmt.Errorf("decode relation reviewer handbook: %w", err)
	}
	return value, value.Validate()
}

func DecodeQualificationSet(reader io.Reader) (QualificationSet, error) {
	var value QualificationSet
	if err := decodeStrict(reader, &value); err != nil {
		return QualificationSet{}, fmt.Errorf("decode relation qualification set: %w", err)
	}
	return value, value.Validate()
}

func DecodeQualificationAnswerKey(reader io.Reader) (QualificationAnswerKey, error) {
	var value QualificationAnswerKey
	if err := decodeStrict(reader, &value); err != nil {
		return QualificationAnswerKey{}, fmt.Errorf("decode relation qualification answer key: %w", err)
	}
	return value, value.Validate()
}

func DecodeQualificationResponses(reader io.Reader) ([]QualificationResponse, error) {
	var value []QualificationResponse
	if err := decodeStrict(reader, &value); err != nil {
		return nil, fmt.Errorf("decode relation qualification responses: %w", err)
	}
	return value, nil
}

func DecodeQualificationReport(reader io.Reader) (QualificationReport, error) {
	var value QualificationReport
	if err := decodeStrict(reader, &value); err != nil {
		return QualificationReport{}, fmt.Errorf("decode relation qualification report: %w", err)
	}
	return value, value.Validate()
}

func DecodeReviewerRecord(reader io.Reader) (ReviewerRecord, error) {
	var value ReviewerRecord
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewerRecord{}, fmt.Errorf("decode relation reviewer record: %w", err)
	}
	return value, value.Validate()
}

func DecodeReviewBundle(reader io.Reader) (ReviewBundle, error) {
	var value ReviewBundle
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewBundle{}, fmt.Errorf("decode relation review bundle: %w", err)
	}
	return value, value.Validate()
}

func DecodeReviewAssignment(reader io.Reader) (ReviewAssignment, error) {
	var value ReviewAssignment
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewAssignment{}, fmt.Errorf("decode relation review assignment: %w", err)
	}
	return value, value.Validate()
}

func DecodeReviewerKit(reader io.Reader) (ReviewerKit, error) {
	var value ReviewerKit
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewerKit{}, fmt.Errorf("decode relation reviewer kit: %w", err)
	}
	return value, value.Validate()
}

func DecodePairJudgmentDraft(reader io.Reader) (PairJudgmentDraft, error) {
	var value PairJudgmentDraft
	if err := decodeStrict(reader, &value); err != nil {
		return PairJudgmentDraft{}, fmt.Errorf("decode relation pair judgment draft: %w", err)
	}
	return value, nil
}

func DecodePairJudgment(reader io.Reader) (PairJudgment, error) {
	var value PairJudgment
	if err := decodeStrict(reader, &value); err != nil {
		return PairJudgment{}, fmt.Errorf("decode relation pair judgment: %w", err)
	}
	return value, value.Validate()
}

func DecodePairJudgments(reader io.Reader) ([]PairJudgment, error) {
	var value []PairJudgment
	if err := decodeStrict(reader, &value); err != nil {
		return nil, fmt.Errorf("decode relation pair judgments: %w", err)
	}
	for index := range value {
		if err := value[index].Validate(); err != nil {
			return nil, fmt.Errorf("decode relation pair judgment %d: %w", index, err)
		}
	}
	return value, nil
}

func DecodeJudgmentBatch(reader io.Reader) (JudgmentBatch, error) {
	var value JudgmentBatch
	if err := decodeStrict(reader, &value); err != nil {
		return JudgmentBatch{}, fmt.Errorf("decode relation judgment batch: %w", err)
	}
	return value, value.Validate()
}

func DecodeRelationAmbiguityAnalysis(reader io.Reader) (RelationAmbiguityAnalysis, error) {
	var value RelationAmbiguityAnalysis
	if err := decodeStrict(reader, &value); err != nil {
		return RelationAmbiguityAnalysis{}, fmt.Errorf("decode relation prereveal ambiguity analysis: %w", err)
	}
	return value, value.Validate()
}

func DecodeConditionProbeDrafts(reader io.Reader) ([]ConditionProbeDraft, error) {
	var value []ConditionProbeDraft
	if err := decodeStrict(reader, &value); err != nil {
		return nil, fmt.Errorf("decode relation condition probe drafts: %w", err)
	}
	return value, nil
}

func DecodeConditionProbe(reader io.Reader) (ConditionProbe, error) {
	var value ConditionProbe
	if err := decodeStrict(reader, &value); err != nil {
		return ConditionProbe{}, fmt.Errorf("decode relation condition probe: %w", err)
	}
	return value, value.Validate()
}

func DecodeConditionProbeBatch(reader io.Reader) (ConditionProbeBatch, error) {
	var value ConditionProbeBatch
	if err := decodeStrict(reader, &value); err != nil {
		return ConditionProbeBatch{}, fmt.Errorf("decode relation condition probe batch: %w", err)
	}
	return value, value.Validate()
}

func DecodeMappingReveal(reader io.Reader) (MappingReveal, error) {
	var value MappingReveal
	if err := decodeStrict(reader, &value); err != nil {
		return MappingReveal{}, fmt.Errorf("decode relation mapping reveal: %w", err)
	}
	return value, value.Validate()
}

func DecodeRelationResolution(reader io.Reader) (RelationResolution, error) {
	var value RelationResolution
	if err := decodeStrict(reader, &value); err != nil {
		return RelationResolution{}, fmt.Errorf("decode relation resolution: %w", err)
	}
	return value, value.Validate()
}

func DecodeRelationResolutions(reader io.Reader) ([]RelationResolution, error) {
	var value []RelationResolution
	if err := decodeStrict(reader, &value); err != nil {
		return nil, fmt.Errorf("decode relation resolutions: %w", err)
	}
	for index := range value {
		if err := value[index].Validate(); err != nil {
			return nil, fmt.Errorf("decode relation resolution %d: %w", index, err)
		}
	}
	return value, nil
}

func DecodeFormalHumanComparison(reader io.Reader) (FormalHumanComparison, error) {
	var value FormalHumanComparison
	if err := decodeStrict(reader, &value); err != nil {
		return FormalHumanComparison{}, fmt.Errorf("decode relation formal-human comparison: %w", err)
	}
	return value, value.Validate()
}

func DecodeTerminalRelationLedger(reader io.Reader) (TerminalRelationLedger, error) {
	var value TerminalRelationLedger
	if err := decodeStrict(reader, &value); err != nil {
		return TerminalRelationLedger{}, fmt.Errorf("decode relation terminal ledger: %w", err)
	}
	return value, value.Validate()
}

func DecodeBlindPackets(reader io.Reader) ([]BlindPacket, error) {
	var value []BlindPacket
	if err := decodeStrict(reader, &value); err != nil {
		return nil, fmt.Errorf("decode relation blind packets: %w", err)
	}
	for index := range value {
		if err := value[index].Validate(); err != nil {
			return nil, fmt.Errorf("decode relation blind packet %d: %w", index, err)
		}
	}
	return value, nil
}

func DecodePrivateMappings(reader io.Reader) ([]PrivateMapping, error) {
	var value []PrivateMapping
	if err := decodeStrict(reader, &value); err != nil {
		return nil, fmt.Errorf("decode relation private mappings: %w", err)
	}
	for index := range value {
		if err := value[index].Validate(); err != nil {
			return nil, fmt.Errorf("decode relation private mapping %d: %w", index, err)
		}
	}
	return value, nil
}

func DecodeStudyAmendment(reader io.Reader) (StudyAmendment, error) {
	var value StudyAmendment
	if err := decodeStrict(reader, &value); err != nil {
		return StudyAmendment{}, fmt.Errorf("decode relation study amendment: %w", err)
	}
	return value, value.Validate()
}

func EncodeIndented(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func decodeStrict(reader io.Reader, target any) error {
	raw, err := io.ReadAll(io.LimitReader(reader, MaximumDocumentSize+1))
	if err != nil {
		return err
	}
	if len(raw) > MaximumDocumentSize {
		return errors.New("relation document exceeds 16 MiB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("relation document contains more than one JSON value")
		}
		return err
	}
	return nil
}

func digestJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func digestText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
