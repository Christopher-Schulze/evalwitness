package relation

import (
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	RelationResolutionSchemaVersionV1      = "evalwitness.relation-resolution.v1"
	RelationResolutionSchemaVersionV2      = "evalwitness.relation-resolution.v2"
	RelationResolutionSchemaVersionV3      = "evalwitness.relation-resolution.v3"
	RelationResolutionSchemaVersion        = RelationResolutionSchemaVersionV1
	FormalHumanComparisonSchemaVersionV1   = "evalwitness.relation-formal-human-comparison.v1"
	FormalHumanComparisonSchemaVersionV2   = "evalwitness.relation-formal-human-comparison.v2"
	FormalHumanComparisonSchemaVersionV3   = "evalwitness.relation-formal-human-comparison.v3"
	FormalHumanComparisonSchemaVersion     = FormalHumanComparisonSchemaVersionV1
	TerminalRelationLedgerSchemaVersionV1  = "evalwitness.relation-terminal-ledger.v1"
	TerminalRelationLedgerSchemaVersionV2  = "evalwitness.relation-terminal-ledger.v2"
	TerminalRelationLedgerSchemaVersionV3  = "evalwitness.relation-terminal-ledger.v3"
	TerminalRelationLedgerSchemaVersion    = TerminalRelationLedgerSchemaVersionV1
	RelationResolutionRule                 = "information-insufficiency-veto_then_primary-agreement_then_tie-majority_else-indeterminate.v1"
	TaskClusterAggregationRule             = "any-contradiction_else-any-unresolved_else-all-support.v1"
	RelationAdmissibilityHumanSupported    = "human_supported"
	RelationAdmissibilityHumanContradicted = "human_contradicted"
	RelationAdmissibilityHumanUnresolved   = "human_unresolved"
	VerifierRelationNotConsulted           = "not_consulted"
)

type ReviewerJudgmentReference struct {
	ReviewerSlot   int    `json:"reviewer_slot"`
	JudgmentDigest string `json:"judgment_digest"`
}

type AxisResolution struct {
	Axis             Axis             `json:"axis"`
	SlotOneRating    Rating           `json:"slot_one_rating"`
	SlotTwoRating    Rating           `json:"slot_two_rating"`
	TieBreakRating   Rating           `json:"tie_break_rating,omitempty"`
	ResolutionRule   string           `json:"resolution_rule"`
	ResolvedRating   Rating           `json:"resolved_rating"`
	NormalizedRating NormalizedRating `json:"normalized_rating"`
}

type RelationResolution struct {
	SchemaVersion          string                      `json:"schema_version"`
	CanonicalPolicy        string                      `json:"canonical_policy"`
	ProtocolVersion        string                      `json:"protocol_version"`
	Objective              ReviewObjective             `json:"review_objective"`
	PlanDigest             string                      `json:"plan_digest"`
	BundleDigest           string                      `json:"bundle_digest"`
	MappingRevealDigest    string                      `json:"mapping_reveal_digest"`
	MappingDigest          string                      `json:"mapping_digest"`
	PacketID               string                      `json:"packet_id"`
	PacketDigest           string                      `json:"packet_digest"`
	CaseID                 string                      `json:"case_id"`
	TaskGroupID            string                      `json:"task_group_id"`
	Family                 mutation.Family             `json:"family"`
	InterventionClass      mutation.InterventionClass  `json:"intervention_class"`
	ExpectedRelation       mutation.Relation           `json:"expected_relation"`
	Unit                   UnitType                    `json:"unit"`
	SplitRole              string                      `json:"split_role"`
	Control                string                      `json:"control"`
	FormalWitnessDigest    string                      `json:"formal_witness_digest"`
	LeftLogicalSide        LogicalSide                 `json:"left_logical_side"`
	PrimaryJudgments       []ReviewerJudgmentReference `json:"primary_judgments"`
	TieBreakJudgmentDigest string                      `json:"tie_break_judgment_digest,omitempty"`
	ConstructCaveats       []ReasonCode                `json:"construct_caveats"`
	AxisResolutions        []AxisResolution            `json:"axis_resolutions"`
	TranslationContract    FamilyContract              `json:"translation_contract"`
	Translation            TranslationResult           `json:"translation"`
	AdmissibilityStatus    string                      `json:"admissibility_status"`
	ResolutionRule         string                      `json:"resolution_rule"`
	VerifierRelationStatus string                      `json:"verifier_relation_status"`
	ResolvedAt             string                      `json:"resolved_at"`
	ExternalActionStatus   ExternalActionStatus        `json:"external_action_status"`
	Digest                 string                      `json:"digest"`
}

type StateTally struct {
	Denominator int `json:"denominator"`
	Supports    int `json:"supports"`
	Contradicts int `json:"contradicts"`
	Unresolved  int `json:"unresolved"`
}

type StratumTally struct {
	ID          string `json:"id"`
	Denominator int    `json:"denominator"`
	Supports    int    `json:"supports"`
	Contradicts int    `json:"contradicts"`
	Unresolved  int    `json:"unresolved"`
}

type ProbeGuessTally struct {
	Correct   int `json:"correct"`
	Incorrect int `json:"incorrect"`
	Unknown   int `json:"unknown"`
}

type ReviewerProbeSummary struct {
	ReviewerSlot    int             `json:"reviewer_slot"`
	Packets         int             `json:"packets"`
	FamilyGuess     ProbeGuessTally `json:"family_guess"`
	DirectionGuess  ProbeGuessTally `json:"direction_guess"`
	SourceCondition ProbeGuessTally `json:"source_condition_guess"`
	RecognizedTasks int             `json:"recognized_tasks"`
}

type TaskClusterSummary struct {
	AggregationRule       string       `json:"aggregation_rule"`
	TaskGroups            int          `json:"task_groups"`
	Supports              int          `json:"supports"`
	Contradicts           int          `json:"contradicts"`
	Unresolved            int          `json:"unresolved"`
	SupportInterval       RateInterval `json:"support_interval"`
	ContradictionInterval RateInterval `json:"contradiction_interval"`
	UnresolvedInterval    RateInterval `json:"unresolved_interval"`
}

type UnresolvedSensitivity struct {
	Denominator                int     `json:"denominator"`
	StrictSupported            int     `json:"strict_supported"`
	StrictSupportRate          float64 `json:"strict_support_rate"`
	BestCaseSupported          int     `json:"best_case_supported"`
	BestCaseSupportRate        float64 `json:"best_case_support_rate"`
	ObservedContradicted       int     `json:"observed_contradicted"`
	ObservedContradictionRate  float64 `json:"observed_contradiction_rate"`
	WorstCaseContradicted      int     `json:"worst_case_contradicted"`
	WorstCaseContradictionRate float64 `json:"worst_case_contradiction_rate"`
	UnresolvedRetained         int     `json:"unresolved_retained"`
}

type EvidenceLayerBoundary struct {
	FormalRelationSource   string `json:"formal_relation_source"`
	HumanConstructSource   string `json:"human_construct_source"`
	VerifierRelationStatus string `json:"verifier_relation_status"`
}

type FormalHumanComparison struct {
	SchemaVersion           string                        `json:"schema_version"`
	CanonicalPolicy         string                        `json:"canonical_policy"`
	ProtocolVersion         string                        `json:"protocol_version"`
	Objective               ReviewObjective               `json:"review_objective"`
	PlanDigest              string                        `json:"plan_digest"`
	BundleDigest            string                        `json:"bundle_digest"`
	SampleDigest            string                        `json:"sample_digest"`
	DataRole                ReviewDataRole                `json:"data_role"`
	MappingRevealDigest     string                        `json:"mapping_reveal_digest"`
	AmbiguityAnalysisDigest string                        `json:"ambiguity_analysis_digest"`
	PrimaryCommitments      []JudgmentCommitmentReference `json:"primary_commitments"`
	TieBreakBatchDigest     string                        `json:"tie_break_batch_digest,omitempty"`
	ProbeBatchDigests       []string                      `json:"probe_batch_digests"`
	ResolutionRule          string                        `json:"resolution_rule"`
	ResolutionDigests       []string                      `json:"resolution_digests"`
	PacketStates            StateTally                    `json:"packet_states"`
	FamilyDenominators      []StratumTally                `json:"family_denominators"`
	ClassDenominators       []StratumTally                `json:"class_denominators"`
	SplitDenominators       []StratumTally                `json:"split_denominators"`
	ControlDenominators     []StratumTally                `json:"control_denominators"`
	TaskGroupDenominators   []StratumTally                `json:"task_group_denominators"`
	ReviewerProbeSummaries  []ReviewerProbeSummary        `json:"reviewer_probe_summaries"`
	TaskClusters            TaskClusterSummary            `json:"task_clusters"`
	Sensitivity             UnresolvedSensitivity         `json:"unresolved_sensitivity"`
	EvidenceLayers          EvidenceLayerBoundary         `json:"evidence_layers"`
	CompletedAt             string                        `json:"completed_at"`
	ExternalActionStatus    ExternalActionStatus          `json:"external_action_status"`
	Digest                  string                        `json:"digest"`
}

type FormalHumanResult struct {
	Comparison  FormalHumanComparison `json:"comparison"`
	Resolutions []RelationResolution  `json:"resolutions"`
}

type RelationLedgerEntry struct {
	PacketID               string            `json:"packet_id"`
	PacketDigest           string            `json:"packet_digest"`
	CaseID                 string            `json:"case_id"`
	Family                 mutation.Family   `json:"family"`
	ExpectedRelation       mutation.Relation `json:"expected_relation"`
	FormalWitnessDigest    string            `json:"formal_witness_digest"`
	HumanResolutionDigest  string            `json:"human_resolution_digest"`
	HumanState             TranslationState  `json:"human_state"`
	AdmissibilityStatus    string            `json:"admissibility_status"`
	VerifierRelationStatus string            `json:"verifier_relation_status"`
}

type TerminalRelationLedger struct {
	SchemaVersion               string                        `json:"schema_version"`
	CanonicalPolicy             string                        `json:"canonical_policy"`
	ProtocolVersion             string                        `json:"protocol_version"`
	Objective                   ReviewObjective               `json:"review_objective"`
	PlanDigest                  string                        `json:"plan_digest"`
	BundleDigest                string                        `json:"bundle_digest"`
	SampleDigest                string                        `json:"sample_digest"`
	DataRole                    ReviewDataRole                `json:"data_role"`
	MappingRevealDigest         string                        `json:"mapping_reveal_digest"`
	AmbiguityAnalysisDigest     string                        `json:"ambiguity_analysis_digest"`
	FormalHumanComparisonDigest string                        `json:"formal_human_comparison_digest"`
	PrimaryCommitments          []JudgmentCommitmentReference `json:"primary_commitments"`
	TieBreakAssignmentDigest    string                        `json:"tie_break_assignment_digest,omitempty"`
	TieBreakBatchDigest         string                        `json:"tie_break_batch_digest,omitempty"`
	ProbeBatchDigests           []string                      `json:"probe_batch_digests"`
	Entries                     []RelationLedgerEntry         `json:"entries"`
	PacketStates                StateTally                    `json:"packet_states"`
	Status                      string                        `json:"status"`
	EvidenceLayers              EvidenceLayerBoundary         `json:"evidence_layers"`
	CompletedAt                 string                        `json:"completed_at"`
	ExternalActionStatus        ExternalActionStatus          `json:"external_action_status"`
	Digest                      string                        `json:"digest"`
}

func BuildFormalHumanComparison(plan Plan, bundle ReviewBundle, mappings []PrivateMapping, reveal MappingReveal, ambiguity RelationAmbiguityAnalysis,
	leftAssignment ReviewAssignment, leftBatch JudgmentBatch, leftProbes ConditionProbeBatch,
	rightAssignment ReviewAssignment, rightBatch JudgmentBatch, rightProbes ConditionProbeBatch,
	tieAssignment *ReviewAssignment, tieBatch *JudgmentBatch, completedAt string) (FormalHumanResult, error) {
	leftAssignment, leftBatch, rightAssignment, rightBatch, err := validatePrimaryJudgmentPair(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch)
	if err != nil {
		return FormalHumanResult{}, err
	}
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return FormalHumanResult{}, err
	}
	if bundle.ProtocolVersion != plan.ProtocolVersion || bundle.PlanDigest != plan.Digest {
		return FormalHumanResult{}, errors.New("relation formal-human comparison plan and bundle disagree")
	}
	leftProbes, rightProbes, err = probesBySlot(leftProbes, rightProbes)
	if err != nil {
		return FormalHumanResult{}, err
	}
	if err := verifyMappingRevealForComparison(bundle, mappings, reveal, ambiguity, leftAssignment, leftBatch, leftProbes, rightAssignment, rightBatch, rightProbes, tieAssignment, tieBatch); err != nil {
		return FormalHumanResult{}, err
	}
	if err := requireRelationTimeAfter("relation formal-human comparison", completedAt, reveal.RevealedAt); err != nil {
		return FormalHumanResult{}, err
	}
	mappingByPacket := privateMappingsByPacket(mappings)
	leftJudgments, rightJudgments := judgmentsByPacket(leftBatch.Judgments), judgmentsByPacket(rightBatch.Judgments)
	tieJudgments := map[string]PairJudgment{}
	if tieBatch != nil {
		tieJudgments = judgmentsByPacket(tieBatch.Judgments)
	}
	ambiguityByPacket := make(map[string]RelationAmbiguityItem, len(ambiguity.Items))
	for _, item := range ambiguity.Items {
		ambiguityByPacket[item.PacketID] = item
	}
	resolutions := make([]RelationResolution, len(bundle.Packets))
	for index, packet := range bundle.Packets {
		resolution, buildErr := buildRelationResolution(plan, bundle, reveal, packet, mappingByPacket[packet.PacketID], ambiguityByPacket[packet.PacketID], leftJudgments[packet.PacketID], rightJudgments[packet.PacketID], tieJudgments[packet.PacketID], completedAt)
		if buildErr != nil {
			return FormalHumanResult{}, buildErr
		}
		resolutions[index] = resolution
	}
	sort.Slice(resolutions, func(left, right int) bool { return resolutions[left].PacketID < resolutions[right].PacketID })
	comparison := summarizeFormalHumanComparison(bundle, reveal, ambiguity, leftProbes, rightProbes, mappingByPacket, resolutions, completedAt)
	sealed, err := SealFormalHumanComparison(comparison)
	if err != nil {
		return FormalHumanResult{}, err
	}
	return FormalHumanResult{Comparison: sealed, Resolutions: resolutions}, nil
}

func buildRelationResolution(plan Plan, bundle ReviewBundle, reveal MappingReveal, packet BlindPacket, mapping PrivateMapping, ambiguity RelationAmbiguityItem,
	left, right, tie PairJudgment, resolvedAt string) (RelationResolution, error) {
	if err := mapping.Validate(); err != nil {
		return RelationResolution{}, err
	}
	if mapping.ProtocolVersion != plan.ProtocolVersion || packet.ProtocolVersion != plan.ProtocolVersion || reveal.ProtocolVersion != plan.ProtocolVersion ||
		left.ProtocolVersion != plan.ProtocolVersion || right.ProtocolVersion != plan.ProtocolVersion ||
		mapping.PacketID != packet.PacketID || mapping.PacketDigest != packet.Digest || ambiguity.PacketID != packet.PacketID || left.PacketID != packet.PacketID || right.PacketID != packet.PacketID {
		return RelationResolution{}, errors.New("relation resolution packet, mapping, ambiguity, and judgments disagree")
	}
	hasTie := tie.Digest != ""
	if hasTie && tie.ProtocolVersion != plan.ProtocolVersion {
		return RelationResolution{}, errors.New("relation resolution tie-break protocol differs from the plan")
	}
	if ambiguity.TieBreakRequired != hasTie {
		return RelationResolution{}, errors.New("relation resolution tie-break presence contradicts prereveal ambiguity")
	}
	leftLogical, err := leftLogicalSide(mapping)
	if err != nil {
		return RelationResolution{}, err
	}
	caveats := constructCaveats(left, right, tie)
	axisResolutions := make([]AxisResolution, len(defaultAxes()))
	for index, definition := range defaultAxes() {
		leftObservation := left.Observations[index]
		rightObservation := right.Observations[index]
		if leftObservation.Axis != definition.ID || rightObservation.Axis != definition.ID {
			return RelationResolution{}, errors.New("relation resolution judgment axes are not aligned")
		}
		tieRating := Rating("")
		if hasTie {
			if tie.Observations[index].Axis != definition.ID {
				return RelationResolution{}, errors.New("relation resolution tie-break axes are not aligned")
			}
			tieRating = tie.Observations[index].Rating
		}
		resolved, rule := resolveVisibleRatingWithCaveats(definition.ID, leftObservation.Rating, rightObservation.Rating, tieRating, caveats)
		normalized, normalizeErr := normalizeVisibleRating(resolved, leftLogical)
		if normalizeErr != nil {
			return RelationResolution{}, normalizeErr
		}
		axisResolutions[index] = AxisResolution{definition.ID, leftObservation.Rating, rightObservation.Rating, tieRating, rule, resolved, normalized}
	}
	contract, exists := familyContract(plan, mapping.Family)
	if !exists {
		return RelationResolution{}, errors.New("relation resolution family is not governed")
	}
	normalized := make([]AxisObservation, len(contract.RequiredAxes))
	for index, axis := range contract.RequiredAxes {
		axisIndex := slices.IndexFunc(axisResolutions, func(value AxisResolution) bool { return value.Axis == axis })
		normalized[index] = AxisObservation{Axis: axis, Rating: axisResolutions[axisIndex].NormalizedRating}
	}
	translation, err := Translate(plan, mapping.Family, normalized)
	if err != nil {
		return RelationResolution{}, err
	}
	definition, _ := mutation.DefinitionFor(mapping.Family)
	resolution := RelationResolution{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest, BundleDigest: bundle.Digest,
		MappingRevealDigest: reveal.Digest, MappingDigest: mapping.Digest, PacketID: packet.PacketID, PacketDigest: packet.Digest,
		CaseID: mapping.CaseID, TaskGroupID: mapping.SourceTaskGroupID, Family: mapping.Family, InterventionClass: definition.Class,
		ExpectedRelation: mapping.ExpectedRelation, Unit: mapping.Unit, SplitRole: mapping.SplitRole, Control: mapping.Control,
		FormalWitnessDigest: mapping.WitnessDigest, LeftLogicalSide: leftLogical,
		PrimaryJudgments: []ReviewerJudgmentReference{{1, left.Digest}, {2, right.Digest}}, ConstructCaveats: caveats, AxisResolutions: axisResolutions,
		TranslationContract: contract, Translation: translation, AdmissibilityStatus: admissibilityForState(translation.State), ResolutionRule: RelationResolutionRule,
		VerifierRelationStatus: VerifierRelationNotConsulted, ResolvedAt: resolvedAt, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	if hasTie {
		resolution.TieBreakJudgmentDigest = tie.Digest
	}
	return SealRelationResolution(resolution)
}

func resolveVisibleRating(axis Axis, left, right, tie Rating) (Rating, string) {
	return resolveVisibleRatingWithCaveats(axis, left, right, tie, nil)
}

func resolveVisibleRatingWithCaveats(axis Axis, left, right, tie Rating, caveats []ReasonCode) (Rating, string) {
	if axis == AxisInformation {
		if slices.Contains(caveats, ReasonInsufficientInformation) {
			return RatingInsufficient, "construct_caveat_veto"
		}
		if len(caveats) > 0 {
			return RatingIndeterminate, "construct_caveat_veto"
		}
		values := []Rating{left, right}
		if tie != "" {
			values = append(values, tie)
		}
		if slices.Contains(values, RatingInsufficient) {
			return RatingInsufficient, "information_insufficiency_veto"
		}
		if slices.Contains(values, RatingIndeterminate) {
			return RatingIndeterminate, "information_indeterminate_veto"
		}
	}
	if left == right {
		return left, "primary_agreement"
	}
	if tie != "" && (tie == left || tie == right) {
		return tie, "tie_break_majority"
	}
	if axis == AxisInformation {
		return RatingIndeterminate, "unresolved_disagreement"
	}
	return RatingIndeterminate, "unresolved_disagreement"
}

func normalizeVisibleRating(value Rating, leftLogical LogicalSide) (NormalizedRating, error) {
	switch value {
	case RatingLeft:
		if leftLogical == LogicalOriginal {
			return NormalizedOriginal, nil
		}
		return NormalizedTransformed, nil
	case RatingRight:
		if leftLogical == LogicalOriginal {
			return NormalizedTransformed, nil
		}
		return NormalizedOriginal, nil
	case RatingControlEffect:
		return NormalizedControlEffect, nil
	case RatingEqual:
		return NormalizedEqual, nil
	case RatingIndeterminate:
		return NormalizedIndeterminate, nil
	case RatingInsufficient:
		return NormalizedInsufficient, nil
	case RatingNoControl:
		return NormalizedNoControl, nil
	case RatingNotApplicable:
		return NormalizedNotApplicable, nil
	case RatingSufficient:
		return NormalizedSufficient, nil
	default:
		return "", fmt.Errorf("relation visible rating %q cannot be normalized", value)
	}
}

func leftLogicalSide(mapping PrivateMapping) (LogicalSide, error) {
	byPosition := make(map[VisiblePosition]LogicalSide, 2)
	for _, evidence := range mapping.EvidenceMappings {
		if existing, exists := byPosition[evidence.VisiblePosition]; exists && existing != evidence.LogicalSide {
			return "", errors.New("relation mapping assigns one visible position to multiple logical sides")
		}
		byPosition[evidence.VisiblePosition] = evidence.LogicalSide
	}
	if len(byPosition) != 2 || byPosition[PositionLeft] == byPosition[PositionRight] || !slices.Contains([]LogicalSide{LogicalOriginal, LogicalTransformed}, byPosition[PositionLeft]) {
		return "", errors.New("relation mapping does not establish a bijective visible-to-logical side mapping")
	}
	return byPosition[PositionLeft], nil
}

func SealRelationResolution(resolution RelationResolution) (RelationResolution, error) {
	schemaVersion, err := schemaVersionForProtocol(resolution.ProtocolVersion, RelationResolutionSchemaVersionV1, RelationResolutionSchemaVersionV2, RelationResolutionSchemaVersionV3)
	if err != nil {
		return RelationResolution{}, err
	}
	resolution.SchemaVersion, resolution.CanonicalPolicy, resolution.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := relationResolutionDigest(resolution)
	if err != nil {
		return RelationResolution{}, err
	}
	resolution.Digest = digest
	return resolution, resolution.Validate()
}

func (resolution RelationResolution) Validate() error {
	definition, familyExists := mutation.DefinitionFor(resolution.Family)
	if !validVersionedIdentity(resolution.SchemaVersion, resolution.ProtocolVersion, RelationResolutionSchemaVersionV1, RelationResolutionSchemaVersionV2, RelationResolutionSchemaVersionV3) || resolution.CanonicalPolicy != CanonicalPolicy ||
		resolution.Objective != ReviewObjectiveControlledRelation || !validDigest(resolution.PlanDigest) || !validDigest(resolution.BundleDigest) || !validDigest(resolution.MappingRevealDigest) ||
		!validDigest(resolution.MappingDigest) || !validOpaqueID(resolution.PacketID, "relation-packet-") || !validDigest(resolution.PacketDigest) || strings.TrimSpace(resolution.CaseID) == "" ||
		strings.TrimSpace(resolution.TaskGroupID) == "" || !familyExists || resolution.InterventionClass != definition.Class || resolution.ExpectedRelation != definition.Relation ||
		resolution.Unit != unitForDefinition(definition) || strings.TrimSpace(resolution.SplitRole) == "" || strings.TrimSpace(resolution.Control) == "" || !validDigest(resolution.FormalWitnessDigest) ||
		!slices.Contains([]LogicalSide{LogicalOriginal, LogicalTransformed}, resolution.LeftLogicalSide) || resolution.ResolutionRule != RelationResolutionRule ||
		resolution.VerifierRelationStatus != VerifierRelationNotConsulted || resolution.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation resolution identity, formal relation, normalization, rule, or evidence-layer boundary is invalid")
	}
	if len(resolution.PrimaryJudgments) != 2 || resolution.PrimaryJudgments[0].ReviewerSlot != 1 || resolution.PrimaryJudgments[1].ReviewerSlot != 2 ||
		!validDigest(resolution.PrimaryJudgments[0].JudgmentDigest) || !validDigest(resolution.PrimaryJudgments[1].JudgmentDigest) || resolution.PrimaryJudgments[0].JudgmentDigest == resolution.PrimaryJudgments[1].JudgmentDigest {
		return errors.New("relation resolution primary judgments are invalid")
	}
	if resolution.TieBreakJudgmentDigest != "" && !validDigest(resolution.TieBreakJudgmentDigest) {
		return errors.New("relation resolution tie-break judgment is invalid")
	}
	if len(resolution.AxisResolutions) != len(defaultAxes()) {
		return errors.New("relation resolution must preserve all seven axes")
	}
	visible := make([]VisibleAxisObservation, len(resolution.AxisResolutions))
	slotOne := make([]VisibleAxisObservation, len(resolution.AxisResolutions))
	slotTwo := make([]VisibleAxisObservation, len(resolution.AxisResolutions))
	tieBreak := make([]VisibleAxisObservation, len(resolution.AxisResolutions))
	for index, axis := range resolution.AxisResolutions {
		if axis.Axis != defaultAxes()[index].ID || !slices.Contains([]string{"construct_caveat_veto", "information_insufficiency_veto", "information_indeterminate_veto", "primary_agreement", "tie_break_majority", "unresolved_disagreement"}, axis.ResolutionRule) {
			return errors.New("relation resolution axes are incomplete, unsorted, or use an unknown rule")
		}
		visible[index] = VisibleAxisObservation{Axis: axis.Axis, Rating: axis.ResolvedRating}
		slotOne[index] = VisibleAxisObservation{Axis: axis.Axis, Rating: axis.SlotOneRating}
		slotTwo[index] = VisibleAxisObservation{Axis: axis.Axis, Rating: axis.SlotTwoRating}
		if resolution.TieBreakJudgmentDigest != "" {
			tieBreak[index] = VisibleAxisObservation{Axis: axis.Axis, Rating: axis.TieBreakRating}
		} else if axis.TieBreakRating != "" {
			return errors.New("relation resolution axis has tie-break evidence without a tie-break judgment")
		}
		expectedRating, expectedRule := resolveVisibleRatingWithCaveats(axis.Axis, axis.SlotOneRating, axis.SlotTwoRating, axis.TieBreakRating, resolution.ConstructCaveats)
		if axis.ResolvedRating != expectedRating || axis.ResolutionRule != expectedRule {
			return errors.New("relation resolution axis does not reproduce the frozen resolution rule")
		}
		normalized, err := normalizeVisibleRating(axis.ResolvedRating, resolution.LeftLogicalSide)
		if err != nil || normalized != axis.NormalizedRating {
			return errors.New("relation resolution normalized axis does not reproduce its visible rating")
		}
	}
	if err := validateVisibleObservations(visible); err != nil {
		return err
	}
	if err := validateConstructCaveats(resolution.ConstructCaveats); err != nil {
		return err
	}
	if err := validateVisibleObservations(slotOne); err != nil {
		return err
	}
	if err := validateVisibleObservations(slotTwo); err != nil {
		return err
	}
	if resolution.TieBreakJudgmentDigest != "" {
		if err := validateVisibleObservations(tieBreak); err != nil {
			return err
		}
	}
	if err := resolution.Translation.Validate(); err != nil {
		return err
	}
	if resolution.Translation.ProtocolVersion != resolution.ProtocolVersion || resolution.TranslationContract.Family != resolution.Family || resolution.TranslationContract.Unit != resolution.Unit || resolution.TranslationContract.ExpectedRelation != resolution.ExpectedRelation ||
		resolution.Translation.PlanDigest != resolution.PlanDigest || resolution.Translation.Family != resolution.Family || resolution.Translation.ExpectedRelation != resolution.ExpectedRelation ||
		resolution.AdmissibilityStatus != admissibilityForState(resolution.Translation.State) {
		return errors.New("relation resolution translation or admissibility binding is invalid")
	}
	if err := validateResolutionContract(resolution.TranslationContract); err != nil {
		return err
	}
	want := make([]AxisObservation, len(resolution.TranslationContract.RequiredAxes))
	for index, axis := range resolution.TranslationContract.RequiredAxes {
		resolvedIndex := slices.IndexFunc(resolution.AxisResolutions, func(value AxisResolution) bool { return value.Axis == axis })
		want[index] = AxisObservation{Axis: axis, Rating: resolution.AxisResolutions[resolvedIndex].NormalizedRating}
	}
	if !reflect.DeepEqual(want, resolution.Translation.Observations) {
		return errors.New("relation resolution translation observations do not reproduce resolved axes")
	}
	recomputedTranslation, err := translateWithContract(resolution.ProtocolVersion, resolution.PlanDigest, resolution.TranslationContract, want)
	if err != nil || !reflect.DeepEqual(recomputedTranslation, resolution.Translation) {
		return errors.New("relation resolution translation does not reproduce its frozen contract")
	}
	if _, err := time.Parse(time.RFC3339, resolution.ResolvedAt); err != nil {
		return errors.New("relation resolution time must be RFC3339")
	}
	expected, err := relationResolutionDigest(resolution)
	if err != nil || resolution.Digest != expected {
		return errors.New("relation resolution digest is invalid")
	}
	return nil
}

func validateResolutionContract(contract FamilyContract) error {
	axes := make(map[Axis]struct{}, len(defaultAxes()))
	for _, definition := range defaultAxes() {
		axes[definition.ID] = struct{}{}
	}
	if len(contract.RequiredAxes) == 0 || !slices.IsSorted(contract.RequiredAxes) || hasDuplicate(contract.RequiredAxes) || len(contract.SupportAll) == 0 || len(contract.ContradictAny) == 0 {
		return errors.New("relation resolution translation contract is incomplete")
	}
	for _, axis := range contract.RequiredAxes {
		if _, exists := axes[axis]; !exists {
			return errors.New("relation resolution translation contract references an unknown axis")
		}
	}
	for _, conditions := range [][]TranslationCondition{contract.SupportAll, contract.ContradictAny} {
		if err := validateConditions(contract, conditions, axes); err != nil {
			return err
		}
	}
	return nil
}

func constructCaveats(judgments ...PairJudgment) []ReasonCode {
	result := make([]ReasonCode, 0, 4)
	for _, judgment := range judgments {
		for _, reason := range judgment.ReasonCodes {
			if slices.Contains([]ReasonCode{ReasonAmbiguousTask, ReasonHiddenContextRequired, ReasonInsufficientInformation, ReasonMultiFactorChange}, reason) {
				result = append(result, reason)
			}
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func validateConstructCaveats(caveats []ReasonCode) error {
	if !slices.IsSorted(caveats) || hasDuplicate(caveats) {
		return errors.New("relation resolution construct caveats must be unique and sorted")
	}
	for _, caveat := range caveats {
		if !slices.Contains([]ReasonCode{ReasonAmbiguousTask, ReasonHiddenContextRequired, ReasonInsufficientInformation, ReasonMultiFactorChange}, caveat) {
			return errors.New("relation resolution contains a non-caveat reason")
		}
	}
	return nil
}

func summarizeFormalHumanComparison(bundle ReviewBundle, reveal MappingReveal, ambiguity RelationAmbiguityAnalysis, leftProbes, rightProbes ConditionProbeBatch,
	mappings map[string]PrivateMapping, resolutions []RelationResolution, completedAt string) FormalHumanComparison {
	comparison := FormalHumanComparison{
		ProtocolVersion: bundle.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: bundle.PlanDigest, BundleDigest: bundle.Digest,
		SampleDigest: bundle.SampleDigest, DataRole: bundle.DataRole, MappingRevealDigest: reveal.Digest, AmbiguityAnalysisDigest: ambiguity.Digest,
		PrimaryCommitments: append([]JudgmentCommitmentReference(nil), reveal.PrimaryCommitments...), TieBreakBatchDigest: reveal.TieBreakBatchDigest,
		ProbeBatchDigests: append([]string(nil), reveal.ProbeBatchDigests...), ResolutionRule: RelationResolutionRule,
		EvidenceLayers: relationEvidenceLayers(), CompletedAt: completedAt, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	comparison.ResolutionDigests = make([]string, len(resolutions))
	for index, resolution := range resolutions {
		comparison.ResolutionDigests[index] = resolution.Digest
		addState(&comparison.PacketStates, resolution.Translation.State)
	}
	sort.Strings(comparison.ResolutionDigests)
	comparison.FamilyDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return string(value.Family) })
	comparison.ClassDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return string(value.InterventionClass) })
	comparison.SplitDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return value.SplitRole })
	comparison.ControlDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return value.Control })
	comparison.TaskGroupDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return value.TaskGroupID })
	comparison.ReviewerProbeSummaries = []ReviewerProbeSummary{
		summarizeReviewerProbes(leftProbes, mappings), summarizeReviewerProbes(rightProbes, mappings),
	}
	comparison.TaskClusters = summarizeTaskClusters(comparison.TaskGroupDenominators)
	comparison.Sensitivity = summarizeUnresolvedSensitivity(comparison.PacketStates)
	return comparison
}

func stratifyResolutions(resolutions []RelationResolution, key func(RelationResolution) string) []StratumTally {
	byID := make(map[string]*StratumTally)
	for _, resolution := range resolutions {
		id := key(resolution)
		if byID[id] == nil {
			byID[id] = &StratumTally{ID: id}
		}
		addStratumState(byID[id], resolution.Translation.State)
	}
	result := make([]StratumTally, 0, len(byID))
	for _, value := range byID {
		result = append(result, *value)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result
}

func addState(tally *StateTally, state TranslationState) {
	tally.Denominator++
	switch state {
	case TranslationSupports:
		tally.Supports++
	case TranslationContradicts:
		tally.Contradicts++
	case TranslationUnresolved:
		tally.Unresolved++
	}
}

func addStratumState(tally *StratumTally, state TranslationState) {
	tally.Denominator++
	switch state {
	case TranslationSupports:
		tally.Supports++
	case TranslationContradicts:
		tally.Contradicts++
	case TranslationUnresolved:
		tally.Unresolved++
	}
}

func summarizeReviewerProbes(batch ConditionProbeBatch, mappings map[string]PrivateMapping) ReviewerProbeSummary {
	result := ReviewerProbeSummary{ReviewerSlot: batch.ReviewerSlot, Packets: len(batch.Probes)}
	for _, probe := range batch.Probes {
		mapping := mappings[probe.PacketID]
		accumulateProbeGuess(&result.FamilyGuess, probe.FamilyGuess, string(mapping.Family), UnknownProbeValue)
		direction := DirectionRightOriginal
		if logical, _ := leftLogicalSide(mapping); logical == LogicalOriginal {
			direction = DirectionLeftOriginal
		}
		accumulateProbeGuess(&result.DirectionGuess, string(probe.DirectionGuess), string(direction), string(DirectionUnknown))
		accumulateProbeGuess(&result.SourceCondition, probe.SourceConditionGuess, mapping.Control, UnknownProbeValue)
		if probe.RecognizedTask {
			result.RecognizedTasks++
		}
	}
	return result
}

func accumulateProbeGuess(tally *ProbeGuessTally, guess, actual, unknown string) {
	switch guess {
	case unknown:
		tally.Unknown++
	case actual:
		tally.Correct++
	default:
		tally.Incorrect++
	}
}

func summarizeTaskClusters(strata []StratumTally) TaskClusterSummary {
	result := TaskClusterSummary{AggregationRule: TaskClusterAggregationRule, TaskGroups: len(strata)}
	for _, stratum := range strata {
		switch {
		case stratum.Contradicts > 0:
			result.Contradicts++
		case stratum.Unresolved > 0:
			result.Unresolved++
		default:
			result.Supports++
		}
	}
	result.SupportInterval = relationWilsonInterval(result.Supports, result.TaskGroups)
	result.ContradictionInterval = relationWilsonInterval(result.Contradicts, result.TaskGroups)
	result.UnresolvedInterval = relationWilsonInterval(result.Unresolved, result.TaskGroups)
	return result
}

func summarizeUnresolvedSensitivity(states StateTally) UnresolvedSensitivity {
	return UnresolvedSensitivity{
		Denominator: states.Denominator, StrictSupported: states.Supports, StrictSupportRate: relationRatio(states.Supports, states.Denominator),
		BestCaseSupported: states.Supports + states.Unresolved, BestCaseSupportRate: relationRatio(states.Supports+states.Unresolved, states.Denominator),
		ObservedContradicted: states.Contradicts, ObservedContradictionRate: relationRatio(states.Contradicts, states.Denominator),
		WorstCaseContradicted: states.Contradicts + states.Unresolved, WorstCaseContradictionRate: relationRatio(states.Contradicts+states.Unresolved, states.Denominator),
		UnresolvedRetained: states.Unresolved,
	}
}

func SealFormalHumanComparison(comparison FormalHumanComparison) (FormalHumanComparison, error) {
	schemaVersion, err := schemaVersionForProtocol(comparison.ProtocolVersion, FormalHumanComparisonSchemaVersionV1, FormalHumanComparisonSchemaVersionV2, FormalHumanComparisonSchemaVersionV3)
	if err != nil {
		return FormalHumanComparison{}, err
	}
	comparison.SchemaVersion, comparison.CanonicalPolicy, comparison.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := formalHumanComparisonDigest(comparison)
	if err != nil {
		return FormalHumanComparison{}, err
	}
	comparison.Digest = digest
	return comparison, comparison.Validate()
}

func (comparison FormalHumanComparison) Validate() error {
	if !validVersionedIdentity(comparison.SchemaVersion, comparison.ProtocolVersion, FormalHumanComparisonSchemaVersionV1, FormalHumanComparisonSchemaVersionV2, FormalHumanComparisonSchemaVersionV3) || comparison.CanonicalPolicy != CanonicalPolicy ||
		comparison.Objective != ReviewObjectiveControlledRelation || !validDigest(comparison.PlanDigest) || !validDigest(comparison.BundleDigest) || !validDigest(comparison.SampleDigest) ||
		!slices.Contains([]ReviewDataRole{ReviewDataDevelopmentPilot, ReviewDataPrimaryAudit}, comparison.DataRole) || !validDigest(comparison.MappingRevealDigest) || !validDigest(comparison.AmbiguityAnalysisDigest) ||
		comparison.ResolutionRule != RelationResolutionRule || comparison.PacketStates.Denominator < 1 || comparison.PacketStates.Denominator != len(comparison.ResolutionDigests) ||
		comparison.ExternalActionStatus != ExternalActionNotAuthorized || comparison.EvidenceLayers != relationEvidenceLayers() {
		return errors.New("relation formal-human comparison identity, denominators, rule, or evidence-layer boundary is invalid")
	}
	if err := validateJudgmentCommitments(comparison.PrimaryCommitments); err != nil {
		return err
	}
	if len(comparison.ProbeBatchDigests) != 2 || !slices.IsSorted(comparison.ProbeBatchDigests) || hasDuplicate(comparison.ProbeBatchDigests) ||
		len(comparison.ResolutionDigests) == 0 || !slices.IsSorted(comparison.ResolutionDigests) || hasDuplicate(comparison.ResolutionDigests) {
		return errors.New("relation formal-human comparison commitments are incomplete or unsorted")
	}
	if comparison.TieBreakBatchDigest != "" && !validDigest(comparison.TieBreakBatchDigest) {
		return errors.New("relation formal-human comparison tie-break commitment is invalid")
	}
	if err := validateStateTally("relation packet states", comparison.PacketStates); err != nil {
		return err
	}
	for name, strata := range map[string][]StratumTally{"family": comparison.FamilyDenominators, "class": comparison.ClassDenominators, "split": comparison.SplitDenominators, "control": comparison.ControlDenominators, "task group": comparison.TaskGroupDenominators} {
		if err := validateStrata(name, strata, comparison.PacketStates); err != nil {
			return err
		}
	}
	if len(comparison.ReviewerProbeSummaries) != 2 || comparison.ReviewerProbeSummaries[0].ReviewerSlot != 1 || comparison.ReviewerProbeSummaries[1].ReviewerSlot != 2 {
		return errors.New("relation formal-human comparison requires probe summaries for primary slots one and two")
	}
	for _, summary := range comparison.ReviewerProbeSummaries {
		if summary.Packets != comparison.PacketStates.Denominator || !validProbeGuessTally(summary.FamilyGuess, summary.Packets) || !validProbeGuessTally(summary.DirectionGuess, summary.Packets) ||
			!validProbeGuessTally(summary.SourceCondition, summary.Packets) || summary.RecognizedTasks < 0 || summary.RecognizedTasks > summary.Packets {
			return errors.New("relation formal-human comparison probe denominators are invalid")
		}
	}
	if comparison.TaskClusters != summarizeTaskClusters(comparison.TaskGroupDenominators) || comparison.Sensitivity != summarizeUnresolvedSensitivity(comparison.PacketStates) {
		return errors.New("relation formal-human comparison cluster or sensitivity statistics do not reproduce denominators")
	}
	if _, err := time.Parse(time.RFC3339, comparison.CompletedAt); err != nil {
		return errors.New("relation formal-human comparison time must be RFC3339")
	}
	expected, err := formalHumanComparisonDigest(comparison)
	if err != nil || comparison.Digest != expected {
		return errors.New("relation formal-human comparison digest is invalid")
	}
	return nil
}

func VerifyFormalHumanComparison(comparison FormalHumanComparison, resolutions []RelationResolution) error {
	if err := comparison.Validate(); err != nil {
		return err
	}
	if len(resolutions) != comparison.PacketStates.Denominator {
		return errors.New("relation formal-human comparison resolution coverage differs from its denominator")
	}
	digests := make([]string, len(resolutions))
	packetIDs := make([]string, len(resolutions))
	for index, resolution := range resolutions {
		if err := resolution.Validate(); err != nil {
			return err
		}
		if resolution.ProtocolVersion != comparison.ProtocolVersion || resolution.PlanDigest != comparison.PlanDigest || resolution.BundleDigest != comparison.BundleDigest || resolution.MappingRevealDigest != comparison.MappingRevealDigest {
			return errors.New("relation formal-human comparison contains a cross-bound resolution")
		}
		digests[index] = resolution.Digest
		packetIDs[index] = resolution.PacketID
	}
	sort.Strings(digests)
	sort.Strings(packetIDs)
	if hasDuplicate(packetIDs) {
		return errors.New("relation formal-human comparison contains duplicate packet resolutions")
	}
	if !slices.Equal(digests, comparison.ResolutionDigests) {
		return errors.New("relation formal-human comparison resolution commitments do not reproduce supplied resolutions")
	}
	recomputed := summarizeComparisonFromResolutions(resolutions)
	if comparison.PacketStates != recomputed.PacketStates || !reflect.DeepEqual(comparison.FamilyDenominators, recomputed.FamilyDenominators) ||
		!reflect.DeepEqual(comparison.ClassDenominators, recomputed.ClassDenominators) || !reflect.DeepEqual(comparison.SplitDenominators, recomputed.SplitDenominators) ||
		!reflect.DeepEqual(comparison.ControlDenominators, recomputed.ControlDenominators) || !reflect.DeepEqual(comparison.TaskGroupDenominators, recomputed.TaskGroupDenominators) {
		return errors.New("relation formal-human comparison denominators do not reproduce resolutions")
	}
	return nil
}

func summarizeComparisonFromResolutions(resolutions []RelationResolution) FormalHumanComparison {
	result := FormalHumanComparison{}
	for _, resolution := range resolutions {
		addState(&result.PacketStates, resolution.Translation.State)
	}
	result.FamilyDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return string(value.Family) })
	result.ClassDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return string(value.InterventionClass) })
	result.SplitDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return value.SplitRole })
	result.ControlDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return value.Control })
	result.TaskGroupDenominators = stratifyResolutions(resolutions, func(value RelationResolution) string { return value.TaskGroupID })
	return result
}

func BuildTerminalRelationLedger(bundle ReviewBundle, reveal MappingReveal, ambiguity RelationAmbiguityAnalysis, comparison FormalHumanComparison, resolutions []RelationResolution, completedAt string) (TerminalRelationLedger, error) {
	if err := bundle.Validate(); err != nil {
		return TerminalRelationLedger{}, err
	}
	if err := reveal.Validate(); err != nil {
		return TerminalRelationLedger{}, err
	}
	if err := ambiguity.Validate(); err != nil {
		return TerminalRelationLedger{}, err
	}
	if err := VerifyFormalHumanComparison(comparison, resolutions); err != nil {
		return TerminalRelationLedger{}, err
	}
	if reveal.ProtocolVersion != bundle.ProtocolVersion || ambiguity.ProtocolVersion != bundle.ProtocolVersion || comparison.ProtocolVersion != bundle.ProtocolVersion ||
		reveal.BundleDigest != bundle.Digest || ambiguity.BundleDigest != bundle.Digest || comparison.BundleDigest != bundle.Digest || reveal.AmbiguityAnalysisDigest != ambiguity.Digest ||
		comparison.AmbiguityAnalysisDigest != ambiguity.Digest || comparison.MappingRevealDigest != reveal.Digest ||
		!slices.Equal(comparison.PrimaryCommitments, reveal.PrimaryCommitments) || !slices.Equal(comparison.ProbeBatchDigests, reveal.ProbeBatchDigests) || comparison.TieBreakBatchDigest != reveal.TieBreakBatchDigest {
		return TerminalRelationLedger{}, errors.New("relation terminal ledger parents disagree")
	}
	wantPackets := make([]string, len(bundle.Packets))
	gotPackets := make([]string, len(resolutions))
	for index, packet := range bundle.Packets {
		wantPackets[index] = packet.PacketID
	}
	for index, resolution := range resolutions {
		gotPackets[index] = resolution.PacketID
	}
	sort.Strings(wantPackets)
	sort.Strings(gotPackets)
	if !slices.Equal(wantPackets, gotPackets) {
		return TerminalRelationLedger{}, errors.New("relation terminal ledger resolution coverage differs from the bundle")
	}
	if err := requireRelationTimeAfter("relation terminal ledger", completedAt, comparison.CompletedAt); err != nil {
		return TerminalRelationLedger{}, err
	}
	entries := make([]RelationLedgerEntry, len(resolutions))
	for index, resolution := range resolutions {
		entries[index] = RelationLedgerEntry{
			PacketID: resolution.PacketID, PacketDigest: resolution.PacketDigest, CaseID: resolution.CaseID, Family: resolution.Family,
			ExpectedRelation: resolution.ExpectedRelation, FormalWitnessDigest: resolution.FormalWitnessDigest, HumanResolutionDigest: resolution.Digest,
			HumanState: resolution.Translation.State, AdmissibilityStatus: resolution.AdmissibilityStatus, VerifierRelationStatus: VerifierRelationNotConsulted,
		}
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].PacketID < entries[right].PacketID })
	status := "complete"
	if comparison.PacketStates.Unresolved > 0 {
		status = "complete_with_unresolved"
	}
	ledger := TerminalRelationLedger{
		ProtocolVersion: bundle.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: bundle.PlanDigest, BundleDigest: bundle.Digest,
		SampleDigest: bundle.SampleDigest, DataRole: bundle.DataRole, MappingRevealDigest: reveal.Digest, AmbiguityAnalysisDigest: ambiguity.Digest,
		FormalHumanComparisonDigest: comparison.Digest, PrimaryCommitments: append([]JudgmentCommitmentReference(nil), reveal.PrimaryCommitments...),
		TieBreakAssignmentDigest: reveal.TieBreakAssignmentDigest, TieBreakBatchDigest: reveal.TieBreakBatchDigest,
		ProbeBatchDigests: append([]string(nil), reveal.ProbeBatchDigests...), Entries: entries, PacketStates: comparison.PacketStates,
		Status: status, EvidenceLayers: relationEvidenceLayers(), CompletedAt: completedAt, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealTerminalRelationLedger(ledger)
}

func SealTerminalRelationLedger(ledger TerminalRelationLedger) (TerminalRelationLedger, error) {
	schemaVersion, err := schemaVersionForProtocol(ledger.ProtocolVersion, TerminalRelationLedgerSchemaVersionV1, TerminalRelationLedgerSchemaVersionV2, TerminalRelationLedgerSchemaVersionV3)
	if err != nil {
		return TerminalRelationLedger{}, err
	}
	ledger.SchemaVersion, ledger.CanonicalPolicy, ledger.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := terminalRelationLedgerDigest(ledger)
	if err != nil {
		return TerminalRelationLedger{}, err
	}
	ledger.Digest = digest
	return ledger, ledger.Validate()
}

func (ledger TerminalRelationLedger) Validate() error {
	if !validVersionedIdentity(ledger.SchemaVersion, ledger.ProtocolVersion, TerminalRelationLedgerSchemaVersionV1, TerminalRelationLedgerSchemaVersionV2, TerminalRelationLedgerSchemaVersionV3) || ledger.CanonicalPolicy != CanonicalPolicy ||
		ledger.Objective != ReviewObjectiveControlledRelation || !validDigest(ledger.PlanDigest) || !validDigest(ledger.BundleDigest) || !validDigest(ledger.SampleDigest) ||
		!slices.Contains([]ReviewDataRole{ReviewDataDevelopmentPilot, ReviewDataPrimaryAudit}, ledger.DataRole) || !validDigest(ledger.MappingRevealDigest) ||
		!validDigest(ledger.AmbiguityAnalysisDigest) || !validDigest(ledger.FormalHumanComparisonDigest) || len(ledger.Entries) == 0 || len(ledger.Entries) != ledger.PacketStates.Denominator ||
		ledger.ExternalActionStatus != ExternalActionNotAuthorized || ledger.EvidenceLayers != relationEvidenceLayers() {
		return errors.New("relation terminal ledger identity, evidence, or layer boundary is invalid")
	}
	if err := validateJudgmentCommitments(ledger.PrimaryCommitments); err != nil {
		return err
	}
	if (ledger.TieBreakAssignmentDigest == "") != (ledger.TieBreakBatchDigest == "") || ledger.TieBreakAssignmentDigest != "" && (!validDigest(ledger.TieBreakAssignmentDigest) || !validDigest(ledger.TieBreakBatchDigest)) ||
		len(ledger.ProbeBatchDigests) != 2 || !slices.IsSorted(ledger.ProbeBatchDigests) || hasDuplicate(ledger.ProbeBatchDigests) {
		return errors.New("relation terminal ledger reveal commitments are incomplete")
	}
	states := StateTally{}
	for index, entry := range ledger.Entries {
		if !validOpaqueID(entry.PacketID, "relation-packet-") || !validDigest(entry.PacketDigest) || strings.TrimSpace(entry.CaseID) == "" || !validDigest(entry.FormalWitnessDigest) ||
			!validDigest(entry.HumanResolutionDigest) || entry.AdmissibilityStatus != admissibilityForState(entry.HumanState) || entry.VerifierRelationStatus != VerifierRelationNotConsulted ||
			index > 0 && ledger.Entries[index-1].PacketID >= entry.PacketID {
			return errors.New("relation terminal ledger entry identity, state, or evidence-layer boundary is invalid")
		}
		definition, exists := mutation.DefinitionFor(entry.Family)
		if !exists || definition.Relation != entry.ExpectedRelation {
			return errors.New("relation terminal ledger entry formal relation is invalid")
		}
		addState(&states, entry.HumanState)
	}
	if states != ledger.PacketStates || validateStateTally("relation terminal ledger states", ledger.PacketStates) != nil ||
		ledger.Status == "complete" && ledger.PacketStates.Unresolved != 0 || ledger.Status == "complete_with_unresolved" && ledger.PacketStates.Unresolved == 0 ||
		!slices.Contains([]string{"complete", "complete_with_unresolved"}, ledger.Status) {
		return errors.New("relation terminal ledger status contradicts complete entry states")
	}
	if _, err := time.Parse(time.RFC3339, ledger.CompletedAt); err != nil {
		return errors.New("relation terminal ledger time must be RFC3339")
	}
	expected, err := terminalRelationLedgerDigest(ledger)
	if err != nil || ledger.Digest != expected {
		return errors.New("relation terminal ledger digest is invalid")
	}
	return nil
}

func verifyMappingRevealForComparison(bundle ReviewBundle, mappings []PrivateMapping, reveal MappingReveal, ambiguity RelationAmbiguityAnalysis,
	leftAssignment ReviewAssignment, leftBatch JudgmentBatch, leftProbes ConditionProbeBatch,
	rightAssignment ReviewAssignment, rightBatch JudgmentBatch, rightProbes ConditionProbeBatch,
	tieAssignment *ReviewAssignment, tieBatch *JudgmentBatch) error {
	if err := reveal.Validate(); err != nil {
		return err
	}
	if err := verifyRelationAmbiguityAgainstPair(ambiguity, bundle, leftAssignment, leftBatch, rightAssignment, rightBatch); err != nil {
		return err
	}
	if validateConditionProbeBatchAgainst(leftProbes, leftAssignment, leftBatch) != nil || validateConditionProbeBatchAgainst(rightProbes, rightAssignment, rightBatch) != nil {
		return errors.New("relation formal-human comparison probe batches do not bind primary commitments")
	}
	if err := validateTieBreakForReveal(bundle, ambiguity, tieAssignment, tieBatch); err != nil {
		return err
	}
	references, mappingByPacket, err := relationMappingReferences(bundle, mappings)
	if err != nil {
		return err
	}
	probeDigests := []string{leftProbes.Digest, rightProbes.Digest}
	sort.Strings(probeDigests)
	if reveal.ProtocolVersion != bundle.ProtocolVersion || ambiguity.ProtocolVersion != bundle.ProtocolVersion ||
		leftProbes.ProtocolVersion != bundle.ProtocolVersion || rightProbes.ProtocolVersion != bundle.ProtocolVersion ||
		reveal.PlanDigest != bundle.PlanDigest || reveal.BundleDigest != bundle.Digest || reveal.AmbiguityAnalysisDigest != ambiguity.Digest ||
		!slices.Equal(reveal.PrimaryCommitments, primaryJudgmentCommitments(leftAssignment, leftBatch, rightAssignment, rightBatch)) || !slices.Equal(reveal.ProbeBatchDigests, probeDigests) ||
		!slices.Equal(reveal.Mappings, references) {
		return errors.New("relation formal-human comparison reveal does not bind exact prereveal evidence")
	}
	if (tieAssignment == nil) != (reveal.TieBreakAssignmentDigest == "") || tieAssignment != nil && (reveal.TieBreakAssignmentDigest != tieAssignment.Digest || reveal.TieBreakBatchDigest != tieBatch.Digest) {
		return errors.New("relation formal-human comparison reveal tie-break commitment differs")
	}
	assignmentByDigest := map[string]ReviewAssignment{leftAssignment.Digest: leftAssignment, rightAssignment.Digest: rightAssignment}
	if tieAssignment != nil {
		assignmentByDigest[tieAssignment.Digest] = *tieAssignment
	}
	if len(reveal.OrderingSeeds) != len(assignmentByDigest) {
		return errors.New("relation formal-human comparison reveal assignment seed coverage differs")
	}
	for _, seedReveal := range reveal.OrderingSeeds {
		assignment, exists := assignmentByDigest[seedReveal.AssignmentDigest]
		seed, decodeErr := hex.DecodeString(seedReveal.Seed)
		if !exists || decodeErr != nil || !assignmentOrderMatches(bundle, mappingByPacket, assignment, seed) {
			return errors.New("relation formal-human comparison reveal seed does not reproduce an assignment")
		}
	}
	return nil
}

func privateMappingsByPacket(mappings []PrivateMapping) map[string]PrivateMapping {
	result := make(map[string]PrivateMapping, len(mappings))
	for _, mapping := range mappings {
		result[mapping.PacketID] = mapping
	}
	return result
}

func validateStateTally(name string, tally StateTally) error {
	if tally.Denominator < 1 || tally.Supports < 0 || tally.Contradicts < 0 || tally.Unresolved < 0 || tally.Supports+tally.Contradicts+tally.Unresolved != tally.Denominator {
		return fmt.Errorf("%s denominator does not retain all states", name)
	}
	return nil
}

func validateStrata(name string, strata []StratumTally, total StateTally) error {
	if len(strata) == 0 {
		return fmt.Errorf("relation %s denominators are empty", name)
	}
	combined := StateTally{}
	for index, stratum := range strata {
		if strings.TrimSpace(stratum.ID) == "" || index > 0 && strata[index-1].ID >= stratum.ID {
			return fmt.Errorf("relation %s denominators must be identified, unique, and sorted", name)
		}
		if err := validateStateTally("relation "+name+" stratum", StateTally{stratum.Denominator, stratum.Supports, stratum.Contradicts, stratum.Unresolved}); err != nil {
			return err
		}
		combined.Denominator += stratum.Denominator
		combined.Supports += stratum.Supports
		combined.Contradicts += stratum.Contradicts
		combined.Unresolved += stratum.Unresolved
	}
	if combined != total {
		return fmt.Errorf("relation %s denominators delete or duplicate packets", name)
	}
	return nil
}

func validProbeGuessTally(tally ProbeGuessTally, denominator int) bool {
	return tally.Correct >= 0 && tally.Incorrect >= 0 && tally.Unknown >= 0 && tally.Correct+tally.Incorrect+tally.Unknown == denominator
}

func admissibilityForState(state TranslationState) string {
	switch state {
	case TranslationSupports:
		return RelationAdmissibilityHumanSupported
	case TranslationContradicts:
		return RelationAdmissibilityHumanContradicted
	default:
		return RelationAdmissibilityHumanUnresolved
	}
}

func relationEvidenceLayers() EvidenceLayerBoundary {
	return EvidenceLayerBoundary{
		FormalRelationSource:   "frozen_mutation_manifest_and_witness",
		HumanConstructSource:   "post_reveal_resolved_blinded_pair_judgments",
		VerifierRelationStatus: VerifierRelationNotConsulted,
	}
}

func relationResolutionDigest(value RelationResolution) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func formalHumanComparisonDigest(value FormalHumanComparison) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func terminalRelationLedgerDigest(value TerminalRelationLedger) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
