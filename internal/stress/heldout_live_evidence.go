package stress

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type HeldOutLiveBatchExecutor interface {
	ExecuteBatch(context.Context, verification.BatchPlan) (verification.BatchResult, error)
}

type HeldOutLiveBatchExecution struct {
	Evidence HeldOutLiveBatchEvidence
	Result   verification.BatchResult
}

const (
	HeldOutLiveReplayVerificationSchemaVersion = "evalwitness.stress-held-out-live-replay-verification.v1"
	heldOutLiveReplayVerificationStatus        = "independently_executed_exact_replay_verified"
)

type HeldOutLiveReplayCellVerification struct {
	CellID                       string `json:"cell_id"`
	RelationID                   string `json:"relation_id"`
	CaseID                       string `json:"case_id"`
	LiveResponseDigest           string `json:"live_response_digest"`
	ExactReplayResponseDigest    string `json:"exact_replay_response_digest"`
	LiveObservationDigest        string `json:"live_observation_digest"`
	ExactReplayObservationDigest string `json:"exact_replay_observation_digest"`
	LiveDecisionDigest           string `json:"live_decision_digest"`
	ExactReplayDecisionDigest    string `json:"exact_replay_decision_digest"`
	ReplayEvidenceDigest         string `json:"replay_evidence_digest"`
	LogicalCalls                 int    `json:"logical_calls"`
}

type HeldOutLiveReplayVerification struct {
	SchemaVersion             string                              `json:"schema_version"`
	CanonicalPolicy           string                              `json:"canonical_policy"`
	PlanDigest                string                              `json:"plan_digest"`
	AdmissionPlanDigest       string                              `json:"admission_plan_digest"`
	LiveBatchEvidenceDigest   string                              `json:"live_batch_evidence_digest"`
	ArmID                     string                              `json:"arm_id"`
	RouteID                   string                              `json:"route_id"`
	ReplaySource              provider.ExactReplaySource          `json:"replay_source"`
	EligibleCellSetDigest     string                              `json:"eligible_cell_set_digest"`
	Cells                     []HeldOutLiveReplayCellVerification `json:"cells"`
	EligibleCells             int                                 `json:"eligible_cells"`
	ExactReplayCells          int                                 `json:"exact_replay_cells"`
	LiveProviderCalls         int                                 `json:"live_provider_calls"`
	ExactReplayLogicalCalls   int                                 `json:"exact_replay_logical_calls"`
	ResponseEvidenceMatches   int                                 `json:"response_evidence_matches"`
	ReplayEvidenceSetDigest   string                              `json:"replay_evidence_set_digest"`
	ResponseEvidenceSetDigest string                              `json:"response_evidence_set_digest"`
	Status                    string                              `json:"status"`
	ClaimBoundary             HeldOutCampaignClaimBoundary        `json:"claim_boundary"`
	Digest                    string                              `json:"digest"`
}

func BuildHeldOutLiveReplayVerification(
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	live HeldOutLiveBatchEvidence,
	exactReplay []ArmReplayEvidence,
) (HeldOutLiveReplayVerification, error) {
	value, err := buildHeldOutLiveReplayVerification(plan, registry, replayed, admission, live, exactReplay)
	if err != nil {
		return HeldOutLiveReplayVerification{}, err
	}
	value.Digest, err = heldOutLiveReplayVerificationDigest(value)
	if err != nil {
		return HeldOutLiveReplayVerification{}, err
	}
	if err := value.ValidateAgainst(plan, registry, replayed, admission, live, exactReplay); err != nil {
		return HeldOutLiveReplayVerification{}, err
	}
	return value, nil
}

func (value HeldOutLiveReplayVerification) Validate() error {
	if value.SchemaVersion != HeldOutLiveReplayVerificationSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.PlanDigest) || !validDigest(value.AdmissionPlanDigest) || !validDigest(value.LiveBatchEvidenceDigest) ||
		strings.TrimSpace(value.ArmID) == "" || !validHeldOutRouteID(value.RouteID) || value.ReplaySource.RouteID != value.RouteID ||
		!validDigest(value.EligibleCellSetDigest) ||
		value.EligibleCells <= 0 || value.ExactReplayCells != value.EligibleCells || len(value.Cells) != value.EligibleCells ||
		value.LiveProviderCalls <= 0 || value.ExactReplayLogicalCalls != value.LiveProviderCalls ||
		value.ResponseEvidenceMatches != value.LiveProviderCalls || !validDigest(value.ReplayEvidenceSetDigest) ||
		!validDigest(value.ResponseEvidenceSetDigest) || value.Status != heldOutLiveReplayVerificationStatus ||
		value.ClaimBoundary.SupportedClaim != heldOutLiveReplayVerificationSupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutLiveReplayVerificationUnsupportedClaims) {
		return errors.New("stress held-out live replay verification identity, totals, status, or claim boundary is invalid")
	}
	if err := value.ReplaySource.Validate(); err != nil {
		return err
	}
	if value.ReplaySource.Records < value.ExactReplayLogicalCalls {
		return errors.New("stress held-out exact replay capture has fewer records than reproduced logical calls")
	}
	cellIDs := make([]string, len(value.Cells))
	replayDigests := make([]string, len(value.Cells))
	responseDigests := make([]string, len(value.Cells))
	logicalCalls := 0
	previous := ""
	for index, cell := range value.Cells {
		if cell.CellID <= previous || !identifierPattern.MatchString(cell.CellID) || !identifierPattern.MatchString(cell.RelationID) ||
			!identifierPattern.MatchString(cell.CaseID) || !validDigest(cell.LiveResponseDigest) ||
			cell.LiveResponseDigest != cell.ExactReplayResponseDigest || !validDigest(cell.LiveObservationDigest) ||
			cell.LiveObservationDigest != cell.ExactReplayObservationDigest || !validDigest(cell.LiveDecisionDigest) ||
			cell.LiveDecisionDigest != cell.ExactReplayDecisionDigest || !validDigest(cell.ReplayEvidenceDigest) || cell.LogicalCalls <= 0 {
			return errors.New("stress held-out live replay cell does not prove exact response, observation, and decision reproduction")
		}
		previous = cell.CellID
		cellIDs[index] = cell.CellID
		replayDigests[index] = cell.ReplayEvidenceDigest
		responseDigests[index] = cell.LiveResponseDigest
		logicalCalls += cell.LogicalCalls
	}
	cellSetDigest, err := digestDocument(cellIDs)
	if err != nil || cellSetDigest != value.EligibleCellSetDigest || logicalCalls != value.ExactReplayLogicalCalls {
		return errors.New("stress held-out live replay cells do not reproduce their exact cell set or call total")
	}
	sort.Strings(replayDigests)
	sort.Strings(responseDigests)
	replaySetDigest, replayErr := digestDocument(replayDigests)
	responseSetDigest, responseErr := digestDocument(responseDigests)
	if replayErr != nil || responseErr != nil || replaySetDigest != value.ReplayEvidenceSetDigest ||
		responseSetDigest != value.ResponseEvidenceSetDigest {
		return errors.New("stress held-out live replay aggregate evidence digest is invalid")
	}
	expected, err := heldOutLiveReplayVerificationDigest(value)
	if err != nil || expected != value.Digest {
		return errors.New("stress held-out live replay verification digest is invalid")
	}
	return nil
}

func (value HeldOutLiveReplayVerification) ValidateAgainst(
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	live HeldOutLiveBatchEvidence,
	exactReplay []ArmReplayEvidence,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	want, err := buildHeldOutLiveReplayVerification(plan, registry, replayed, admission, live, exactReplay)
	if err != nil {
		return err
	}
	want.Digest, err = heldOutLiveReplayVerificationDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out live replay verification differs from its live capture or independently executed exact replay parents")
	}
	return nil
}

func buildHeldOutLiveReplayVerification(
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	live HeldOutLiveBatchEvidence,
	exactReplay []ArmReplayEvidence,
) (HeldOutLiveReplayVerification, error) {
	if err := live.Validate(); err != nil {
		return HeldOutLiveReplayVerification{}, err
	}
	arm, exists := canonicalArmByID(live.ArmID)
	if !exists || arm.Kind == ArmZeroCostControl || arm.Kind == ArmProtocolAdapter || arm.Entrypoint == "" || arm.EvidencePolicy == "" {
		return HeldOutLiveReplayVerification{}, errors.New("stress held-out live replay verification requires one canonical live provider arm")
	}
	if err := plan.ValidateAgainst(registry, replayed); err != nil {
		return HeldOutLiveReplayVerification{}, err
	}
	if err := admission.Validate(); err != nil {
		return HeldOutLiveReplayVerification{}, err
	}
	relations := make(map[string]Relation, len(registry.Relations))
	for _, relation := range registry.Relations {
		relations[relation.ID] = relation
	}
	cases := make(map[string]ReplayedRelationCaseV3, len(replayed))
	for _, item := range replayed {
		cases[item.CaseID] = item
	}
	admissions := make(map[string]ConstructAdmission, len(admission.Entries))
	for _, entry := range admission.Entries {
		if entry.ExecutionEligible {
			admissions[heldOutRelationCaseKey(entry.RelationID, entry.CaseID)] = entry.Admission
		}
	}
	replayByCell := make(map[string]ArmReplayEvidence, len(live.Cells))
	for _, evidence := range exactReplay {
		if evidence.ArmID != live.ArmID {
			continue
		}
		if _, duplicate := replayByCell[evidence.CellID]; duplicate {
			return HeldOutLiveReplayVerification{}, fmt.Errorf("stress held-out exact replay repeats cell %q", evidence.CellID)
		}
		replayByCell[evidence.CellID] = evidence
	}
	if len(replayByCell) != len(live.Cells) {
		return HeldOutLiveReplayVerification{}, errors.New("stress held-out live replay verification lacks one exact replay per live cell")
	}
	value := HeldOutLiveReplayVerification{
		SchemaVersion: HeldOutLiveReplayVerificationSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PlanDigest: plan.Digest, AdmissionPlanDigest: admission.Digest, LiveBatchEvidenceDigest: live.Digest,
		ArmID: live.ArmID, RouteID: live.RouteID, EligibleCellSetDigest: live.EligibleCellSetDigest, EligibleCells: live.EligibleCells,
		LiveProviderCalls: live.ProviderCalls, Status: heldOutLiveReplayVerificationStatus,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim:    heldOutLiveReplayVerificationSupportedClaim,
			UnsupportedClaims: slices.Clone(heldOutLiveReplayVerificationUnsupportedClaims),
		},
	}
	for _, cell := range live.Cells {
		relation, relationExists := relations[cell.RelationID]
		item, caseExists := cases[cell.CaseID]
		constructAdmission, admissionExists := admissions[heldOutRelationCaseKey(cell.RelationID, cell.CaseID)]
		if !relationExists || !caseExists || !admissionExists {
			return HeldOutLiveReplayVerification{}, fmt.Errorf("stress held-out live cell %q lacks relation, case, or admission parents", cell.CellID)
		}
		evidence, replayExists := replayByCell[cell.CellID]
		if !replayExists || evidence.Execution.RelationDigest != relation.Digest || evidence.Execution.CaseID != item.CaseID {
			return HeldOutLiveReplayVerification{}, fmt.Errorf("stress held-out live cell %q lacks its independently executed exact replay", cell.CellID)
		}
		if err := evidence.ValidateAgainst(plan, relation, constructAdmission, item, nil); err != nil {
			return HeldOutLiveReplayVerification{}, fmt.Errorf("validate stress held-out exact replay cell %q: %w", cell.CellID, err)
		}
		if value.ReplaySource.SchemaVersion == "" {
			value.ReplaySource = evidence.Execution.ReplaySource
		} else if !reflect.DeepEqual(value.ReplaySource, evidence.Execution.ReplaySource) {
			return HeldOutLiveReplayVerification{}, errors.New("stress held-out exact replay combines more than one capture source")
		}
		original, err := compareHeldOutLiveReplaySide(cell.Original, evidence.Execution.Original, "original", arm.Entrypoint, arm.EvidencePolicy)
		if err != nil {
			return HeldOutLiveReplayVerification{}, fmt.Errorf("compare stress held-out exact replay cell %q original side: %w", cell.CellID, err)
		}
		transformed, err := compareHeldOutLiveReplaySide(cell.Transformed, evidence.Execution.Transformed, "transformed", arm.Entrypoint, arm.EvidencePolicy)
		if err != nil {
			return HeldOutLiveReplayVerification{}, fmt.Errorf("compare stress held-out exact replay cell %q transformed side: %w", cell.CellID, err)
		}
		liveResponses := append(slices.Clone(cell.Original.ResponseEvidenceDigests), cell.Transformed.ResponseEvidenceDigests...)
		replayResponses := make([]string, 0, len(evidence.Execution.Original.Observations)+len(evidence.Execution.Transformed.Observations))
		for _, observation := range append(cloneScoreObservations(evidence.Execution.Original.Observations), evidence.Execution.Transformed.Observations...) {
			replayResponses = append(replayResponses, observation.ResponseEvidenceDigest)
		}
		sort.Strings(liveResponses)
		sort.Strings(replayResponses)
		liveResponseDigest, liveResponseErr := digestDocument(liveResponses)
		replayResponseDigest, replayResponseErr := digestDocument(replayResponses)
		liveObservationDigest, liveObservationErr := digestDocument([]string{original.liveObservationDigest, transformed.liveObservationDigest})
		replayObservationDigest, replayObservationErr := digestDocument([]string{original.replayObservationDigest, transformed.replayObservationDigest})
		liveDecisionDigest, liveDecisionErr := digestDocument([]string{original.liveDecisionDigest, transformed.liveDecisionDigest})
		replayDecisionDigest, replayDecisionErr := digestDocument([]string{original.replayDecisionDigest, transformed.replayDecisionDigest})
		if errors.Join(liveResponseErr, replayResponseErr, liveObservationErr, replayObservationErr, liveDecisionErr, replayDecisionErr) != nil ||
			liveResponseDigest != cell.ResponseDigest || liveResponseDigest != replayResponseDigest ||
			liveObservationDigest != replayObservationDigest || liveDecisionDigest != replayDecisionDigest ||
			evidence.Result.ProviderCalls != cell.ProviderCalls {
			return HeldOutLiveReplayVerification{}, fmt.Errorf("stress held-out exact replay cell %q differs from its live response, observation, decision, or call evidence", cell.CellID)
		}
		value.Cells = append(value.Cells, HeldOutLiveReplayCellVerification{
			CellID: cell.CellID, RelationID: cell.RelationID, CaseID: cell.CaseID,
			LiveResponseDigest: liveResponseDigest, ExactReplayResponseDigest: replayResponseDigest,
			LiveObservationDigest: liveObservationDigest, ExactReplayObservationDigest: replayObservationDigest,
			LiveDecisionDigest: liveDecisionDigest, ExactReplayDecisionDigest: replayDecisionDigest,
			ReplayEvidenceDigest: evidence.Digest, LogicalCalls: cell.ProviderCalls,
		})
		value.ExactReplayLogicalCalls += cell.ProviderCalls
		value.ResponseEvidenceMatches += cell.ProviderCalls
	}
	value.ExactReplayCells = len(value.Cells)
	if err := value.ReplaySource.Validate(); err != nil {
		return HeldOutLiveReplayVerification{}, err
	}
	if value.ReplaySource.RouteID != live.RouteID {
		return HeldOutLiveReplayVerification{}, errors.New("stress held-out exact replay capture differs from the live route")
	}
	if value.ReplaySource.Records < value.ExactReplayLogicalCalls {
		return HeldOutLiveReplayVerification{}, errors.New("stress held-out exact replay capture cannot contain every reproduced logical call")
	}
	replayDigests := make([]string, len(value.Cells))
	responseDigests := make([]string, len(value.Cells))
	for index, cell := range value.Cells {
		replayDigests[index] = cell.ReplayEvidenceDigest
		responseDigests[index] = cell.LiveResponseDigest
	}
	sort.Strings(replayDigests)
	sort.Strings(responseDigests)
	var err error
	value.ReplayEvidenceSetDigest, err = digestDocument(replayDigests)
	if err != nil {
		return HeldOutLiveReplayVerification{}, err
	}
	value.ResponseEvidenceSetDigest, err = digestDocument(responseDigests)
	if err != nil {
		return HeldOutLiveReplayVerification{}, err
	}
	return value, nil
}

type heldOutLiveReplaySideComparison struct {
	liveResponseDigest      string
	replayResponseDigest    string
	liveObservationDigest   string
	replayObservationDigest string
	liveDecisionDigest      string
	replayDecisionDigest    string
}

func compareHeldOutLiveReplaySide(
	live HeldOutLiveSideEvidence,
	replay ReplaySide,
	variant, entrypoint string,
	policy verification.EvidencePolicy,
) (heldOutLiveReplaySideComparison, error) {
	if err := validateHeldOutLiveSide(live, variant); err != nil {
		return heldOutLiveReplaySideComparison{}, err
	}
	if err := validateReplayExecutionSide(replay, variant, entrypoint, policy); err != nil {
		return heldOutLiveReplaySideComparison{}, err
	}
	liveResponseDigest, err := digestDocument(live.ResponseEvidenceDigests)
	if err != nil {
		return heldOutLiveReplaySideComparison{}, err
	}
	replayResponseDigests := make([]string, len(replay.Observations))
	for index, observation := range replay.Observations {
		replayResponseDigests[index] = observation.ResponseEvidenceDigest
	}
	sort.Strings(replayResponseDigests)
	replayResponseDigest, err := digestDocument(replayResponseDigests)
	if err != nil {
		return heldOutLiveReplaySideComparison{}, err
	}
	liveObservationDigest, err := heldOutLiveReplayObservationDigest(live.Observations)
	if err != nil {
		return heldOutLiveReplaySideComparison{}, err
	}
	replayObservationDigest, err := heldOutLiveReplayObservationDigest(replay.Observations)
	if err != nil {
		return heldOutLiveReplaySideComparison{}, err
	}
	liveDecisionDigest, err := heldOutLiveReplayDecisionDigest(live.Result)
	if err != nil {
		return heldOutLiveReplaySideComparison{}, err
	}
	replayDecisionDigest, err := heldOutLiveReplayDecisionDigest(replay.Result)
	if err != nil {
		return heldOutLiveReplaySideComparison{}, err
	}
	if liveResponseDigest != replayResponseDigest || liveObservationDigest != replayObservationDigest || liveDecisionDigest != replayDecisionDigest {
		return heldOutLiveReplaySideComparison{}, errors.New("exact replay did not reproduce the live response, observation, and decision evidence")
	}
	return heldOutLiveReplaySideComparison{
		liveResponseDigest: liveResponseDigest, replayResponseDigest: replayResponseDigest,
		liveObservationDigest: liveObservationDigest, replayObservationDigest: replayObservationDigest,
		liveDecisionDigest: liveDecisionDigest, replayDecisionDigest: replayDecisionDigest,
	}, nil
}

type heldOutLiveReplayObservation struct {
	SchemaVersion          string               `json:"schema_version"`
	CriterionID            string               `json:"criterion_id"`
	SamplingSlot           string               `json:"sampling_slot"`
	Entrypoint             string               `json:"entrypoint"`
	RequestFingerprint     provider.Fingerprint `json:"request_fingerprint"`
	ProviderRequestID      string               `json:"provider_request_id"`
	ResponseBodyDigest     string               `json:"response_body_digest"`
	ParsedPayloadDigest    string               `json:"parsed_payload_digest"`
	ResponseEvidenceDigest string               `json:"response_evidence_digest"`
	ScoreEvidenceDigest    string               `json:"score_evidence_digest"`
	ExtractionStatus       string               `json:"extraction_status"`
	ExtractionErrorDigest  string               `json:"extraction_error_digest,omitempty"`
}

func heldOutLiveReplayObservationDigest(values []mode.ScoreObservation) (string, error) {
	observations := cloneScoreObservations(values)
	sortScoreObservations(observations)
	material := make([]heldOutLiveReplayObservation, len(observations))
	for index, observation := range observations {
		material[index] = heldOutLiveReplayObservation{
			SchemaVersion: observation.SchemaVersion, CriterionID: observation.CriterionID,
			SamplingSlot: observation.SamplingSlot, Entrypoint: observation.Entrypoint,
			RequestFingerprint: observation.RequestFingerprint, ProviderRequestID: observation.ProviderRequestID,
			ResponseBodyDigest:  observation.ResponseBodyDigest,
			ParsedPayloadDigest: observation.ParsedPayloadDigest, ResponseEvidenceDigest: observation.ResponseEvidenceDigest,
			ScoreEvidenceDigest: observation.ScoreEvidenceDigest,
			ExtractionStatus:    observation.ExtractionStatus, ExtractionErrorDigest: observation.ExtractionErrorDigest,
		}
	}
	return digestDocument(material)
}

func heldOutLiveReplayDecisionDigest(result verification.Result) (string, error) {
	var delta *heldOutDeltaDecision
	if result.Delta != nil {
		delta = &heldOutDeltaDecision{
			State: result.Delta.State, AbstentionReason: string(result.Delta.AbstentionReason), Winner: result.Delta.Winner,
			Margin: result.Delta.Margin, ScoreA: result.Delta.ScoreA, ScoreB: result.Delta.ScoreB,
			PerCriterion: result.Delta.PerCriterion, Inconsistent: result.Delta.Inconsistent,
			Decision: heldOutPairDecisionSummary(result.Delta.Decision), EvidenceStrength: result.Delta.EvidenceStrength,
		}
	}
	var absolute *heldOutAbsoluteDecision
	if result.Absolute != nil {
		absolute = &heldOutAbsoluteDecision{
			State: result.Absolute.State, Value: result.Absolute.Value, PerCriterion: result.Absolute.PerCriterion,
			EvidenceStrength: result.Absolute.EvidenceStrength,
		}
	}
	var selection *heldOutSelectionDecision
	if result.Selection != nil {
		pairDecisions := make([]heldOutPairDecision, len(result.Selection.PairDecisions))
		for index := range result.Selection.PairDecisions {
			pairDecisions[index] = *heldOutPairDecisionSummary(&result.Selection.PairDecisions[index])
		}
		selection = &heldOutSelectionDecision{
			State: result.Selection.State, AbstentionReason: string(result.Selection.AbstentionReason),
			BestIndex: result.Selection.BestIndex, Confidence: result.Selection.Confidence,
			Scores: result.Selection.Scores, Wins: result.Selection.Wins, EvidenceStrength: result.Selection.EvidenceStrength,
			InconsistentPairs: result.Selection.InconsistentPairs, PairsEvaluated: result.Selection.PairsEvaluated,
			EscalatedPairs: result.Selection.EscalatedPairs, PairDecisions: pairDecisions,
		}
	}
	return digestDocument(struct {
		SchemaVersion string                    `json:"schema_version"`
		Mode          verification.Mode         `json:"mode"`
		State         verifier.DecisionState    `json:"state"`
		Delta         *heldOutDeltaDecision     `json:"delta,omitempty"`
		Absolute      *heldOutAbsoluteDecision  `json:"absolute,omitempty"`
		Selection     *heldOutSelectionDecision `json:"selection,omitempty"`
		Lifecycle     verification.Lifecycle    `json:"lifecycle"`
	}{
		SchemaVersion: result.SchemaVersion, Mode: result.Mode, State: result.State,
		Delta: delta, Absolute: absolute, Selection: selection, Lifecycle: result.Lifecycle,
	})
}

type heldOutDeltaDecision struct {
	State            verifier.DecisionState          `json:"state"`
	AbstentionReason string                          `json:"abstention_reason,omitempty"`
	Winner           string                          `json:"winner"`
	Margin           float64                         `json:"conditional_margin"`
	ScoreA           float64                         `json:"conditional_score_a"`
	ScoreB           float64                         `json:"conditional_score_b"`
	PerCriterion     map[string]mode.CriterionScores `json:"per_criterion_conditional_scores"`
	Inconsistent     bool                            `json:"inconsistent"`
	Decision         *heldOutPairDecision            `json:"decision,omitempty"`
	EvidenceStrength verifier.EvidenceStrength       `json:"evidence_strength"`
}

type heldOutAbsoluteDecision struct {
	State            verifier.DecisionState    `json:"state"`
	Value            float64                   `json:"conditional_score"`
	PerCriterion     map[string]float64        `json:"per_criterion_conditional_scores"`
	EvidenceStrength verifier.EvidenceStrength `json:"evidence_strength"`
}

type heldOutSelectionDecision struct {
	State             verifier.DecisionState    `json:"state"`
	AbstentionReason  string                    `json:"abstention_reason,omitempty"`
	BestIndex         int                       `json:"best_index"`
	Confidence        float64                   `json:"decision_strength"`
	Scores            []float64                 `json:"conditional_scores"`
	Wins              []float64                 `json:"wins"`
	EvidenceStrength  verifier.EvidenceStrength `json:"evidence_strength"`
	InconsistentPairs [][2]int                  `json:"inconsistent_pairs,omitempty"`
	PairsEvaluated    int                       `json:"pairs_evaluated,omitempty"`
	EscalatedPairs    int                       `json:"escalated_pairs,omitempty"`
	PairDecisions     []heldOutPairDecision     `json:"pair_decisions,omitempty"`
}

type heldOutPairDecision struct {
	State            verifier.DecisionState    `json:"state"`
	AbstentionReason string                    `json:"abstention_reason,omitempty"`
	Pair             [2]int                    `json:"pair"`
	OrderPolicy      string                    `json:"order_policy"`
	FirstOrder       string                    `json:"first_order"`
	Calls            int                       `json:"calls"`
	RepeatCount      int                       `json:"repeat_count"`
	MeanDifference   float64                   `json:"conditional_mean_difference"`
	WinProbability   float64                   `json:"model_win_probability"`
	Calibrated       bool                      `json:"calibrated"`
	Confidence       float64                   `json:"decision_strength"`
	Winner           int                       `json:"winner"`
	Inconsistent     bool                      `json:"inconsistent,omitempty"`
	Uncertainty      mode.PairUncertainty      `json:"uncertainty"`
	EvidenceStrength verifier.EvidenceStrength `json:"evidence_strength"`
}

func heldOutPairDecisionSummary(value *mode.PairDecision) *heldOutPairDecision {
	if value == nil {
		return nil
	}
	return &heldOutPairDecision{
		State: value.State, AbstentionReason: string(value.AbstentionReason), Pair: value.Pair,
		OrderPolicy: value.OrderPolicy, FirstOrder: value.FirstOrder, Calls: value.Calls, RepeatCount: value.RepeatCount,
		MeanDifference: value.MeanDifference, WinProbability: value.WinProbability, Calibrated: value.Calibrated,
		Confidence: value.Confidence, Winner: value.Winner, Inconsistent: value.Inconsistent,
		Uncertainty: value.Uncertainty, EvidenceStrength: value.EvidenceStrength,
	}
}

func heldOutLiveReplayVerificationDigest(value HeldOutLiveReplayVerification) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func validateHeldOutLiveResultPolicy(result verification.Result, policy verification.EvidencePolicy) error {
	strength, err := resultEvidenceStrength(result)
	if err != nil {
		return err
	}
	want := verifier.ExtractionModeVerifier
	if policy == verification.EvidenceExplicitJudge {
		want = verifier.ExtractionModeJudge
	}
	if strength.ExtractionMode != want {
		return fmt.Errorf("stress held-out live result evidence mode %q differs from arm policy %q", strength.ExtractionMode, policy)
	}
	return nil
}

func ExecuteHeldOutLiveBatch(
	ctx context.Context,
	executor HeldOutLiveBatchExecutor,
	binding HeldOutExecutionArmBatchBinding,
	preview verification.BatchPlan,
	permit HeldOutExecutionPermit,
	reservation HeldOutExecutionReservation,
) (HeldOutLiveBatchExecution, error) {
	return executeHeldOutLiveBatchAt(ctx, executor, binding, preview, permit, reservation, time.Now)
}

func executeHeldOutLiveBatchAt(
	ctx context.Context,
	executor HeldOutLiveBatchExecutor,
	binding HeldOutExecutionArmBatchBinding,
	preview verification.BatchPlan,
	permit HeldOutExecutionPermit,
	reservation HeldOutExecutionReservation,
	now func() time.Time,
) (HeldOutLiveBatchExecution, error) {
	if ctx == nil || executor == nil || now == nil {
		return HeldOutLiveBatchExecution{}, errors.New("stress held-out live batch execution requires context and an executor")
	}
	if err := ctx.Err(); err != nil {
		return HeldOutLiveBatchExecution{}, fmt.Errorf("stress held-out live batch execution context is unavailable: %w", err)
	}
	if err := validateHeldOutApprovedBatchPreview(binding, preview, permit); err != nil {
		return HeldOutLiveBatchExecution{}, err
	}
	reservedAt, err := parseHeldOutExecutionPermitTime(reservation.ReservedAt)
	startedAt := now().UTC()
	if err != nil || reservation.ValidateAgainst(permit, reservedAt) != nil || startedAt.Before(reservedAt) || permit.ValidateAt(startedAt) != nil {
		return HeldOutLiveBatchExecution{}, errors.New("stress held-out live batch execution lacks a current permit reservation")
	}
	approved, err := approveHeldOutBatchPreview(preview, binding.Batch.RequiredAuthorizationDigest)
	if err != nil {
		return HeldOutLiveBatchExecution{}, err
	}
	collector := newScoreObservationCollector()
	result, executionErr := executor.ExecuteBatch(mode.ContextWithScoreObserver(ctx, collector.Observe), approved)
	completedAt := now().UTC()
	if completedAt.Before(startedAt) || permit.ValidateAt(completedAt) != nil {
		return HeldOutLiveBatchExecution{Result: result}, errors.Join(executionErr, errors.New("stress held-out live batch did not complete inside its permit window"))
	}
	evidence, evidenceErr := BuildHeldOutLiveBatchEvidence(binding, permit, reservation, result, collector.Snapshot(), completedAt)
	if executionErr != nil || evidenceErr != nil {
		return HeldOutLiveBatchExecution{Evidence: evidence, Result: result}, errors.Join(executionErr, evidenceErr)
	}
	return HeldOutLiveBatchExecution{Evidence: evidence, Result: result}, nil
}

func validateHeldOutApprovedBatchPreview(
	binding HeldOutExecutionArmBatchBinding,
	preview verification.BatchPlan,
	permit HeldOutExecutionPermit,
) error {
	if binding.ExecutionClass != HeldOutExecutionLiveProvider || binding.Batch.Offline || preview.Authorization == nil ||
		preview.AuthorizationDigest != "" || preview.RunFingerprint != binding.Batch.RunFingerprint ||
		len(preview.Plans) != binding.VerificationInputs || preview.Authorization.AuthorizationDigest != binding.Batch.RequiredAuthorizationDigest {
		return errors.New("stress held-out live batch preview differs from its bound unapproved execution plan")
	}
	permitArm, exists := heldOutExecutionPermitArmByID(permit.Arms, binding.ArmID)
	if !exists || permitArm.BatchPlanBindingDigest != binding.Batch.Digest ||
		permitArm.RequiredAuthorizationDigest != preview.Authorization.AuthorizationDigest ||
		permitArm.ProvidedAuthorizationDigest != preview.Authorization.AuthorizationDigest {
		return errors.New("stress held-out live batch preview is outside its exact arm permit")
	}
	for _, plan := range preview.Plans {
		if plan.Authorization == nil || plan.Input.AuthorizationDigest != "" ||
			plan.Authorization.Verify(plan.Authorization.AuthorizationDigest) != nil {
			return errors.New("stress held-out live batch preview contains an inconsistent authorization plan")
		}
	}
	return nil
}

func approveHeldOutBatchPreview(preview verification.BatchPlan, authorizationDigest string) (verification.BatchPlan, error) {
	if preview.Authorization == nil || preview.AuthorizationDigest != "" || preview.Authorization.Verify(authorizationDigest) != nil {
		return verification.BatchPlan{}, errors.New("stress held-out live batch authorization digest does not verify the preview")
	}
	approved := preview
	approved.AuthorizationDigest = authorizationDigest
	approved.Plans = slices.Clone(preview.Plans)
	for index := range approved.Plans {
		approved.Plans[index].Input.AuthorizationDigest = authorizationDigest
	}
	return approved, nil
}

const (
	HeldOutLiveBatchEvidenceSchemaVersion = "evalwitness.stress-held-out-live-batch-evidence.v1"
	heldOutLiveRuntimeEvidenceStatus      = "runtime_recorded_live_responses"
)

type HeldOutLiveSideEvidence struct {
	Variant                 string                  `json:"variant"`
	PlanFingerprint         string                  `json:"plan_fingerprint"`
	Result                  verification.Result     `json:"result"`
	ResultDigest            string                  `json:"result_digest"`
	Observations            []mode.ScoreObservation `json:"observations"`
	ObservationDigest       string                  `json:"observation_digest"`
	ResponseEvidenceDigests []string                `json:"response_evidence_digests"`
	ProviderCalls           int                     `json:"provider_calls"`
}

type HeldOutLiveCellEvidence struct {
	CellID         string                  `json:"cell_id"`
	RelationID     string                  `json:"relation_id"`
	CaseID         string                  `json:"case_id"`
	Original       HeldOutLiveSideEvidence `json:"original"`
	Transformed    HeldOutLiveSideEvidence `json:"transformed"`
	ProviderCalls  int                     `json:"provider_calls"`
	ResponseDigest string                  `json:"response_digest"`
}

type HeldOutLiveBatchEvidence struct {
	SchemaVersion          string                       `json:"schema_version"`
	CanonicalPolicy        string                       `json:"canonical_policy"`
	ArmID                  string                       `json:"arm_id"`
	RouteID                string                       `json:"route_id"`
	BatchPlanBindingDigest string                       `json:"batch_plan_binding_digest"`
	BatchRunFingerprint    string                       `json:"batch_run_fingerprint"`
	PermitDigest           string                       `json:"permit_digest"`
	ReservationDigest      string                       `json:"reservation_digest"`
	ExecutedAt             string                       `json:"executed_at"`
	ServedModel            string                       `json:"served_model"`
	Budget                 mode.BudgetSnapshot          `json:"budget"`
	Lifecycle              verification.Lifecycle       `json:"lifecycle"`
	Cells                  []HeldOutLiveCellEvidence    `json:"cells"`
	EligibleCellSetDigest  string                       `json:"eligible_cell_set_digest"`
	EligibleCells          int                          `json:"eligible_cells"`
	VerificationInputs     int                          `json:"verification_inputs"`
	ProviderCalls          int                          `json:"provider_calls"`
	ProviderAttempts       int                          `json:"provider_attempts"`
	LiveResponses          int                          `json:"live_responses"`
	RuntimeEvidenceStatus  string                       `json:"runtime_evidence_status"`
	ClaimBoundary          HeldOutCampaignClaimBoundary `json:"claim_boundary"`
	Digest                 string                       `json:"digest"`
}

func BuildHeldOutLiveBatchEvidence(
	binding HeldOutExecutionArmBatchBinding,
	permit HeldOutExecutionPermit,
	reservation HeldOutExecutionReservation,
	result verification.BatchResult,
	observations []mode.ScoreObservation,
	executedAt time.Time,
) (HeldOutLiveBatchEvidence, error) {
	value, err := buildHeldOutLiveBatchEvidence(binding, permit, reservation, result, observations, executedAt)
	if err != nil {
		return HeldOutLiveBatchEvidence{}, err
	}
	value.Digest, err = heldOutLiveBatchEvidenceDigest(value)
	if err != nil {
		return HeldOutLiveBatchEvidence{}, err
	}
	if err := value.ValidateAgainst(binding, permit, reservation); err != nil {
		return HeldOutLiveBatchEvidence{}, err
	}
	return value, nil
}

func (value HeldOutLiveBatchEvidence) Validate() error {
	if value.SchemaVersion != HeldOutLiveBatchEvidenceSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		strings.TrimSpace(value.ArmID) == "" || !validHeldOutRouteID(value.RouteID) || !validDigest(value.BatchPlanBindingDigest) ||
		!validDigest(value.BatchRunFingerprint) || !validDigest(value.PermitDigest) || !validDigest(value.ReservationDigest) ||
		value.EligibleCells <= 0 || value.VerificationInputs != value.EligibleCells*heldOutProviderSidesPerCell ||
		value.ProviderCalls <= 0 || value.ProviderAttempts < value.ProviderCalls || value.LiveResponses != value.ProviderCalls ||
		value.RuntimeEvidenceStatus != heldOutLiveRuntimeEvidenceStatus || !completeLifecycle(value.Lifecycle) ||
		value.ClaimBoundary.SupportedClaim != heldOutLiveBatchEvidenceSupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutLiveBatchEvidenceUnsupportedClaims) {
		return errors.New("stress held-out live batch evidence identity, execution, or claim boundary is invalid")
	}
	executedAt, err := parseHeldOutExecutionPermitTime(value.ExecutedAt)
	if err != nil || executedAt.IsZero() {
		return errors.New("stress held-out live batch evidence execution time is invalid")
	}
	if value.Budget.Calls != value.ProviderCalls || value.Budget.Attempts != value.ProviderAttempts ||
		len(value.Cells) != value.EligibleCells || !validDigest(value.EligibleCellSetDigest) {
		return errors.New("stress held-out live batch evidence budget or cell totals are inconsistent")
	}
	cellIDs := make([]string, 0, len(value.Cells))
	calls, responses := 0, 0
	previous := ""
	for _, cell := range value.Cells {
		if cell.CellID <= previous || !identifierPattern.MatchString(cell.CellID) || !identifierPattern.MatchString(cell.RelationID) ||
			!identifierPattern.MatchString(cell.CaseID) || cell.ProviderCalls != cell.Original.ProviderCalls+cell.Transformed.ProviderCalls ||
			cell.ProviderCalls <= 0 || !validDigest(cell.ResponseDigest) {
			return errors.New("stress held-out live cell evidence identity or call accounting is invalid")
		}
		if err := validateHeldOutLiveSide(cell.Original, "original"); err != nil {
			return err
		}
		if err := validateHeldOutLiveSide(cell.Transformed, "transformed"); err != nil {
			return err
		}
		responseDigests := append(slices.Clone(cell.Original.ResponseEvidenceDigests), cell.Transformed.ResponseEvidenceDigests...)
		sort.Strings(responseDigests)
		responseDigest, digestErr := digestDocument(responseDigests)
		if digestErr != nil || responseDigest != cell.ResponseDigest {
			return errors.New("stress held-out live cell response evidence digest is invalid")
		}
		previous = cell.CellID
		cellIDs = append(cellIDs, cell.CellID)
		calls += cell.ProviderCalls
		responses += len(cell.Original.Observations) + len(cell.Transformed.Observations)
	}
	cellSetDigest, err := digestDocument(cellIDs)
	if err != nil || cellSetDigest != value.EligibleCellSetDigest || calls != value.ProviderCalls || responses != value.LiveResponses {
		return errors.New("stress held-out live batch cells do not reproduce their exact cell set or response totals")
	}
	expected, err := heldOutLiveBatchEvidenceDigest(value)
	if err != nil || expected != value.Digest {
		return errors.New("stress held-out live batch evidence digest is invalid")
	}
	return nil
}

func (value HeldOutLiveBatchEvidence) ValidateAgainst(
	binding HeldOutExecutionArmBatchBinding,
	permit HeldOutExecutionPermit,
	reservation HeldOutExecutionReservation,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := binding.Batch.Validate(); err != nil {
		return err
	}
	if binding.ExecutionClass != HeldOutExecutionLiveProvider || binding.ArmID != value.ArmID || binding.Batch.Offline ||
		binding.Batch.RouteID != value.RouteID ||
		binding.Batch.Digest != value.BatchPlanBindingDigest || binding.Batch.RunFingerprint != value.BatchRunFingerprint ||
		binding.EligibleTestCellSetDigest != value.EligibleCellSetDigest || binding.EligibleTestCells != value.EligibleCells ||
		binding.VerificationInputs != value.VerificationInputs {
		return errors.New("stress held-out live batch evidence differs from its exact live execution binding")
	}
	reservedAt, err := parseHeldOutExecutionPermitTime(reservation.ReservedAt)
	if err != nil || reservation.ValidateAgainst(permit, reservedAt) != nil || value.PermitDigest != permit.Digest ||
		value.ReservationDigest != reservation.Digest {
		return errors.New("stress held-out live batch evidence lacks its exact permit and reservation")
	}
	executedAt, err := parseHeldOutExecutionPermitTime(value.ExecutedAt)
	if err != nil || executedAt.Before(reservedAt) || permit.ValidateAt(executedAt) != nil {
		return errors.New("stress held-out live batch execution is outside its reserved permit window")
	}
	permitArm, exists := heldOutExecutionPermitArmByID(permit.Arms, value.ArmID)
	if !exists || permitArm.BatchPlanBindingDigest != binding.Batch.Digest || permitArm.EligibleTestCellSetDigest != binding.EligibleTestCellSetDigest ||
		permitArm.RequiredAuthorizationDigest != binding.Batch.RequiredAuthorizationDigest {
		return errors.New("stress held-out live batch evidence arm is outside the exact execution permit")
	}
	if err := validateHeldOutLiveCellsAgainstBinding(value.Cells, binding.Batch); err != nil {
		return err
	}
	return nil
}

func buildHeldOutLiveBatchEvidence(
	binding HeldOutExecutionArmBatchBinding,
	permit HeldOutExecutionPermit,
	reservation HeldOutExecutionReservation,
	result verification.BatchResult,
	observations []mode.ScoreObservation,
	executedAt time.Time,
) (HeldOutLiveBatchEvidence, error) {
	if err := binding.Batch.Validate(); err != nil {
		return HeldOutLiveBatchEvidence{}, err
	}
	if binding.ExecutionClass != HeldOutExecutionLiveProvider || binding.Batch.Offline {
		return HeldOutLiveBatchEvidence{}, errors.New("stress held-out live batch evidence requires one live arm binding")
	}
	if executedAt.IsZero() || result.SchemaVersion != verification.BatchSchemaVersion || result.RunFingerprint != binding.Batch.RunFingerprint ||
		len(result.Results) != binding.Batch.InputCount || !completeLifecycle(result.Lifecycle) {
		return HeldOutLiveBatchEvidence{}, errors.New("stress held-out live batch result identity, cardinality, or lifecycle is invalid")
	}
	byScope := scoreObservationsByScope(observations)
	if len(byScope) != binding.Batch.InputCount {
		return HeldOutLiveBatchEvidence{}, errors.New("stress held-out live batch observations do not cover every bound verification input")
	}
	type cellSides struct {
		relationID  string
		caseID      string
		original    *HeldOutLiveSideEvidence
		transformed *HeldOutLiveSideEvidence
	}
	byCell := make(map[string]*cellSides, binding.EligibleTestCells)
	totalCalls := 0
	for index, item := range result.Results {
		planFingerprint := binding.Batch.PlanFingerprints[index]
		if item.RunFingerprint != planFingerprint || item.SchemaVersion != verification.RunSchemaVersion ||
			item.Mode != binding.Batch.InputModes[index] || !completeLifecycle(item.Lifecycle) {
			return HeldOutLiveBatchEvidence{}, fmt.Errorf("stress held-out live batch result %d differs from its bound verification input", index)
		}
		side, err := buildHeldOutLiveSideEvidence(binding.Batch.EvidencePolicy, binding.Batch.StudyVariants[index], planFingerprint, item, byScope[planFingerprint])
		if err != nil {
			return HeldOutLiveBatchEvidence{}, fmt.Errorf("build stress held-out live side %d: %w", index, err)
		}
		cellID := binding.Batch.StudyCellIDs[index]
		cell := byCell[cellID]
		if cell == nil {
			cell = &cellSides{relationID: binding.Batch.TransformationIDs[index], caseID: binding.Batch.AuditCaseIDs[index]}
			byCell[cellID] = cell
		}
		if cell.relationID != binding.Batch.TransformationIDs[index] || cell.caseID != binding.Batch.AuditCaseIDs[index] {
			return HeldOutLiveBatchEvidence{}, fmt.Errorf("stress held-out live cell %q crosses relation or case lineage", cellID)
		}
		switch side.Variant {
		case "original":
			if cell.original != nil {
				return HeldOutLiveBatchEvidence{}, fmt.Errorf("stress held-out live cell %q repeats original evidence", cellID)
			}
			cell.original = &side
		case "transformed":
			if cell.transformed != nil {
				return HeldOutLiveBatchEvidence{}, fmt.Errorf("stress held-out live cell %q repeats transformed evidence", cellID)
			}
			cell.transformed = &side
		default:
			return HeldOutLiveBatchEvidence{}, fmt.Errorf("stress held-out live input %d has unsupported study variant %q", index, side.Variant)
		}
		totalCalls += side.ProviderCalls
	}
	if totalCalls != result.Budget.Calls || len(observations) != totalCalls {
		return HeldOutLiveBatchEvidence{}, errors.New("stress held-out live observations do not reproduce the completed logical-call budget")
	}
	value := HeldOutLiveBatchEvidence{
		SchemaVersion: HeldOutLiveBatchEvidenceSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		ArmID: binding.ArmID, RouteID: binding.Batch.RouteID,
		BatchPlanBindingDigest: binding.Batch.Digest, BatchRunFingerprint: result.RunFingerprint,
		PermitDigest: permit.Digest, ReservationDigest: reservation.Digest, ExecutedAt: formatHeldOutExecutionPermitTime(executedAt),
		ServedModel: result.ServedModel, Budget: result.Budget, Lifecycle: result.Lifecycle,
		EligibleCellSetDigest: binding.EligibleTestCellSetDigest, EligibleCells: binding.EligibleTestCells,
		VerificationInputs: binding.VerificationInputs, ProviderCalls: result.Budget.Calls, ProviderAttempts: result.Budget.Attempts,
		LiveResponses: len(observations), RuntimeEvidenceStatus: heldOutLiveRuntimeEvidenceStatus,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim:    heldOutLiveBatchEvidenceSupportedClaim,
			UnsupportedClaims: slices.Clone(heldOutLiveBatchEvidenceUnsupportedClaims),
		},
	}
	for cellID, sides := range byCell {
		if sides.original == nil || sides.transformed == nil {
			return HeldOutLiveBatchEvidence{}, fmt.Errorf("stress held-out live cell %q lacks one bound side", cellID)
		}
		responseDigests := append(slices.Clone(sides.original.ResponseEvidenceDigests), sides.transformed.ResponseEvidenceDigests...)
		sort.Strings(responseDigests)
		responseDigest, err := digestDocument(responseDigests)
		if err != nil {
			return HeldOutLiveBatchEvidence{}, err
		}
		value.Cells = append(value.Cells, HeldOutLiveCellEvidence{
			CellID: cellID, RelationID: sides.relationID, CaseID: sides.caseID,
			Original: *sides.original, Transformed: *sides.transformed,
			ProviderCalls: sides.original.ProviderCalls + sides.transformed.ProviderCalls, ResponseDigest: responseDigest,
		})
	}
	sort.Slice(value.Cells, func(left, right int) bool { return value.Cells[left].CellID < value.Cells[right].CellID })
	return value, nil
}

func validHeldOutRouteID(value string) bool {
	return strings.HasPrefix(value, "route-") && validDigest(strings.TrimPrefix(value, "route-"))
}

func buildHeldOutLiveSideEvidence(
	policy verification.EvidencePolicy,
	variant string,
	planFingerprint string,
	result verification.Result,
	observations []mode.ScoreObservation,
) (HeldOutLiveSideEvidence, error) {
	if len(observations) == 0 {
		return HeldOutLiveSideEvidence{}, errors.New("live side produced no score observations")
	}
	usage, err := verificationResultUsage(result)
	if err != nil {
		return HeldOutLiveSideEvidence{}, err
	}
	if usage.Calls != len(observations) || usage.Calls <= 0 {
		return HeldOutLiveSideEvidence{}, errors.New("live side observations differ from its result call accounting")
	}
	values := cloneScoreObservations(observations)
	sortScoreObservations(values)
	for _, observation := range values {
		if err := observation.Validate(); err != nil {
			return HeldOutLiveSideEvidence{}, err
		}
		if observation.Scope != planFingerprint || observation.ReplayStatus != provider.ReplayStatusLive ||
			strings.TrimSpace(observation.ProviderRequestID) == "" || observation.ExtractionStatus != mode.ExtractionObservationComplete {
			return HeldOutLiveSideEvidence{}, errors.New("live side contains replayed, rejected, or cross-scope response evidence")
		}
	}
	if err := validateHeldOutLiveResultPolicy(result, policy); err != nil {
		return HeldOutLiveSideEvidence{}, err
	}
	resultDigest, err := digestDocument(result)
	if err != nil {
		return HeldOutLiveSideEvidence{}, err
	}
	observationDigest, err := digestDocument(values)
	if err != nil {
		return HeldOutLiveSideEvidence{}, err
	}
	responseDigests := make([]string, len(values))
	for index, observation := range values {
		responseDigests[index] = observation.ResponseEvidenceDigest
	}
	sort.Strings(responseDigests)
	return HeldOutLiveSideEvidence{
		Variant: variant, PlanFingerprint: planFingerprint, Result: result, ResultDigest: resultDigest,
		Observations: values, ObservationDigest: observationDigest, ResponseEvidenceDigests: responseDigests,
		ProviderCalls: usage.Calls,
	}, nil
}

func validateHeldOutLiveSide(value HeldOutLiveSideEvidence, variant string) error {
	if value.Variant != variant || !validDigest(value.PlanFingerprint) || value.Result.SchemaVersion != verification.RunSchemaVersion ||
		value.Result.RunFingerprint != value.PlanFingerprint || !completeLifecycle(value.Result.Lifecycle) ||
		!validDigest(value.ResultDigest) || !validDigest(value.ObservationDigest) || value.ProviderCalls <= 0 ||
		len(value.Observations) != value.ProviderCalls || len(value.ResponseEvidenceDigests) != value.ProviderCalls {
		return fmt.Errorf("stress held-out live %s side identity or execution evidence is invalid", variant)
	}
	resultDigest, err := digestDocument(value.Result)
	if err != nil || resultDigest != value.ResultDigest {
		return fmt.Errorf("stress held-out live %s result digest is invalid", variant)
	}
	if !slices.IsSorted(value.ResponseEvidenceDigests) {
		return fmt.Errorf("stress held-out live %s response digests are not sorted", variant)
	}
	usage, err := verificationResultUsage(value.Result)
	if err != nil || usage.Calls != value.ProviderCalls {
		return fmt.Errorf("stress held-out live %s result call accounting is invalid", variant)
	}
	for _, observation := range value.Observations {
		if err := observation.Validate(); err != nil {
			return err
		}
		if observation.Scope != value.PlanFingerprint || observation.ReplayStatus != provider.ReplayStatusLive ||
			strings.TrimSpace(observation.ProviderRequestID) == "" || observation.ExtractionStatus != mode.ExtractionObservationComplete {
			return fmt.Errorf("stress held-out live %s observation is not completed live evidence", variant)
		}
	}
	observations := cloneScoreObservations(value.Observations)
	sortScoreObservations(observations)
	if !reflect.DeepEqual(observations, value.Observations) {
		return fmt.Errorf("stress held-out live %s observations are not canonically ordered", variant)
	}
	observationDigest, err := digestDocument(value.Observations)
	if err != nil || observationDigest != value.ObservationDigest {
		return fmt.Errorf("stress held-out live %s observation digest is invalid", variant)
	}
	responseDigests := make([]string, len(value.Observations))
	for index, observation := range value.Observations {
		responseDigests[index] = observation.ResponseEvidenceDigest
	}
	sort.Strings(responseDigests)
	if !slices.Equal(responseDigests, value.ResponseEvidenceDigests) {
		return fmt.Errorf("stress held-out live %s response digest list differs from its observations", variant)
	}
	return nil
}

func validateHeldOutLiveCellsAgainstBinding(cells []HeldOutLiveCellEvidence, binding verification.BatchPlanBinding) error {
	expected := make(map[string]map[string]string, len(cells))
	for index, cellID := range binding.StudyCellIDs {
		if expected[cellID] == nil {
			expected[cellID] = make(map[string]string, heldOutProviderSidesPerCell)
		}
		variant := binding.StudyVariants[index]
		if _, duplicate := expected[cellID][variant]; duplicate {
			return fmt.Errorf("stress held-out live binding repeats cell %q variant %q", cellID, variant)
		}
		expected[cellID][variant] = binding.PlanFingerprints[index]
	}
	if len(expected) != len(cells) {
		return errors.New("stress held-out live cells differ from the bound cell cardinality")
	}
	for _, cell := range cells {
		variants, exists := expected[cell.CellID]
		if !exists || variants["original"] != cell.Original.PlanFingerprint || variants["transformed"] != cell.Transformed.PlanFingerprint {
			return fmt.Errorf("stress held-out live cell %q differs from its bound plan fingerprints", cell.CellID)
		}
		if validateHeldOutLiveResultPolicy(cell.Original.Result, binding.EvidencePolicy) != nil ||
			validateHeldOutLiveResultPolicy(cell.Transformed.Result, binding.EvidencePolicy) != nil {
			return fmt.Errorf("stress held-out live cell %q differs from its bound evidence policy", cell.CellID)
		}
	}
	return nil
}

func verificationResultUsage(result verification.Result) (mode.UsageSummary, error) {
	switch {
	case result.Absolute != nil:
		return result.Absolute.Usage, nil
	case result.Delta != nil:
		return result.Delta.Usage, nil
	case result.Selection != nil:
		return result.Selection.Usage, nil
	default:
		return mode.UsageSummary{}, errors.New("verification result has no usage-bearing payload")
	}
}

func heldOutExecutionPermitArmByID(values []HeldOutExecutionPermitArm, armID string) (HeldOutExecutionPermitArm, bool) {
	index := slices.IndexFunc(values, func(value HeldOutExecutionPermitArm) bool { return value.ArmID == armID })
	if index < 0 {
		return HeldOutExecutionPermitArm{}, false
	}
	return values[index], true
}

func heldOutLiveBatchEvidenceDigest(value HeldOutLiveBatchEvidence) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutLiveBatchEvidenceSupportedClaim = "one permit-bound live arm batch completed with runtime-recorded live response and score-evidence digests for every admission-eligible original and transformed input"

var heldOutLiveBatchEvidenceUnsupportedClaims = []string{
	"independent packet-level proof that network transport occurred",
	"provider identity beyond the bound route and returned response records",
	"exact replay until the response capture is separately sealed and verified",
	"empirical verifier reliability without the bound arm report and analysis",
	"exactly-once provider execution",
}

const heldOutLiveReplayVerificationSupportedClaim = "an independently executed exact replay reproduced every live response-evidence digest, score observation, and verifier decision for one admission-eligible live arm"

var heldOutLiveReplayVerificationUnsupportedClaims = []string{
	"packet-level proof that network transport occurred",
	"provider identity beyond the bound route and returned response records",
	"independent capture-byte validation without retaining and re-inspecting the exact capture parent",
	"replay correctness beyond the exact captured response set",
	"empirical verifier reliability without the bound arm report and analysis",
	"exactly-once provider execution",
}
