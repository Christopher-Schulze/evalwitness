package stress

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/baseline"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type ArmKind string

const (
	ArmScoreTokenVerifier ArmKind = "score_token_verifier"
	ArmExplicitTextJudge  ArmKind = "explicit_text_judge"
	ArmZeroCostControl    ArmKind = "zero_cost_control"
	ArmProtocolAdapter    ArmKind = "protocol_adapter"
)

type ArmSupport string

const (
	ArmSupported   ArmSupport = "supported"
	ArmUnsupported ArmSupport = "unsupported"
)

type ArmDefinition struct {
	ID                                 string                      `json:"id"`
	Kind                               ArmKind                     `json:"kind"`
	Entrypoint                         string                      `json:"entrypoint,omitempty"`
	EvidencePolicy                     verification.EvidencePolicy `json:"evidence_policy,omitempty"`
	BaselineName                       string                      `json:"baseline_name,omitempty"`
	BaselineFeature                    string                      `json:"baseline_feature,omitempty"`
	BaselineOrder                      string                      `json:"baseline_order,omitempty"`
	ProtocolVersion                    string                      `json:"protocol_version,omitempty"`
	ProtocolInvocationSchema           string                      `json:"protocol_invocation_schema,omitempty"`
	ProtocolResultSchema               string                      `json:"protocol_result_schema,omitempty"`
	ExternalProcessConformanceRequired bool                        `json:"external_process_conformance_required"`
	ProviderDependent                  bool                        `json:"provider_dependent"`
}

type ArmComparisonCell struct {
	CellID         string     `json:"cell_id"`
	ArmID          string     `json:"arm_id"`
	RelationID     string     `json:"relation_id"`
	RelationDigest string     `json:"relation_digest"`
	CaseID         string     `json:"case_id"`
	TaskGroupID    string     `json:"task_group_id"`
	ManifestDigest string     `json:"manifest_digest"`
	Support        ArmSupport `json:"support"`
	Reason         string     `json:"reason,omitempty"`
}

type ArmComparisonPlan struct {
	SchemaVersion    string              `json:"schema_version"`
	CanonicalPolicy  string              `json:"canonical_policy"`
	RegistryDigest   string              `json:"registry_digest"`
	ReleaseDigest    string              `json:"release_digest"`
	Arms             []ArmDefinition     `json:"arms"`
	Cells            []ArmComparisonCell `json:"cells"`
	RelationCases    int                 `json:"relation_cases"`
	SupportedCells   int                 `json:"supported_cells"`
	UnsupportedCells int                 `json:"unsupported_cells"`
	GlobalScore      bool                `json:"global_score"`
	Digest           string              `json:"digest"`
}

func BuildArmComparisonPlan(registry RelationRegistry, replayed []ReplayedRelationCaseV3) (ArmComparisonPlan, error) {
	value, err := buildArmComparisonPlanUnchecked(registry, replayed)
	if err != nil {
		return ArmComparisonPlan{}, err
	}
	value.Digest, err = armComparisonPlanDigest(value)
	if err != nil {
		return ArmComparisonPlan{}, err
	}
	if err := value.ValidateAgainst(registry, replayed); err != nil {
		return ArmComparisonPlan{}, err
	}
	return value, nil
}

func (value ArmComparisonPlan) ValidateAgainst(registry RelationRegistry, replayed []ReplayedRelationCaseV3) error {
	if value.SchemaVersion != ArmComparisonPlanSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.RegistryDigest != registry.Digest || value.ReleaseDigest != registry.ReleaseDigest || value.GlobalScore ||
		!validDigest(value.Digest) {
		return errors.New("stress arm-comparison plan identity or global-score boundary is invalid")
	}
	expected, err := buildArmComparisonPlanUnchecked(registry, replayed)
	if err != nil {
		return err
	}
	expected.Digest, err = armComparisonPlanDigest(expected)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("stress arm-comparison plan differs from the locked relation corpus and canonical arms")
	}
	return nil
}

func buildArmComparisonPlanUnchecked(registry RelationRegistry, replayed []ReplayedRelationCaseV3) (ArmComparisonPlan, error) {
	if err := validateReplayedRelationCoverage(registry, replayed); err != nil {
		return ArmComparisonPlan{}, err
	}
	arms := canonicalArmDefinitions()
	value := ArmComparisonPlan{
		SchemaVersion: ArmComparisonPlanSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RegistryDigest: registry.Digest, ReleaseDigest: registry.ReleaseDigest, Arms: arms, GlobalScore: false,
	}
	relations := make(map[string]Relation, len(registry.Relations))
	for _, relation := range registry.Relations {
		relations[relation.ID] = relation
	}
	for _, item := range replayed {
		for _, relationID := range item.RelationIDs {
			relation := relations[relationID]
			value.RelationCases++
			for _, arm := range arms {
				support, reason := armSupportForRelation(arm, relation)
				cell := ArmComparisonCell{
					ArmID: arm.ID, RelationID: relation.ID, RelationDigest: relation.Digest,
					CaseID: item.CaseID, TaskGroupID: item.TaskGroupID, ManifestDigest: item.ManifestDigest,
					Support: support, Reason: reason,
				}
				cellID, err := comparisonCellID(cell.ArmID, cell.RelationID, cell.CaseID)
				if err != nil {
					return ArmComparisonPlan{}, err
				}
				cell.CellID = cellID
				value.Cells = append(value.Cells, cell)
				if support == ArmSupported {
					value.SupportedCells++
				} else {
					value.UnsupportedCells++
				}
			}
		}
	}
	sort.Slice(value.Cells, func(left, right int) bool { return value.Cells[left].CellID < value.Cells[right].CellID })
	return value, nil
}

func canonicalArmDefinitions() []ArmDefinition {
	result := []ArmDefinition{
		{
			ID: "explicit-text-judge", Kind: ArmExplicitTextJudge, Entrypoint: "cli.verify",
			EvidencePolicy: verification.EvidenceExplicitJudge, ProviderDependent: true,
		},
		{
			ID: "external-protocol-adapter", Kind: ArmProtocolAdapter, Entrypoint: "protocol.application",
			EvidencePolicy: verification.EvidenceStrictVerifier, ProtocolVersion: protocolkit.CurrentVersion,
			ProtocolInvocationSchema: protocolkit.ApplicationInvocationSchema, ProtocolResultSchema: protocolkit.ApplicationResultSchema,
			ExternalProcessConformanceRequired: true, ProviderDependent: true,
		},
		{
			ID: "score-token-verifier", Kind: ArmScoreTokenVerifier, Entrypoint: "cli.verify",
			EvidencePolicy: verification.EvidenceStrictVerifier, ProviderDependent: true,
		},
	}
	for _, control := range canonicalZeroCostControls() {
		result = append(result, ArmDefinition{
			ID: zeroCostArmID(control), Kind: ArmZeroCostControl,
			BaselineName: control.Name, BaselineFeature: control.Feature, BaselineOrder: string(control.Order),
		})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result
}

func canonicalZeroCostControls() []baseline.Baseline {
	result := make([]baseline.Baseline, 0, 7)
	for _, control := range baseline.Terminal() {
		if slices.Contains([]string{"none", "steps", "trace bytes", "error words"}, control.Feature) {
			result = append(result, control)
		}
	}
	return result
}

func zeroCostArmID(control baseline.Baseline) string {
	switch control.Name {
	case "first listed":
		return "zero-cost-first-listed"
	case "fewest steps":
		return "zero-cost-fewest-steps"
	case "most steps":
		return "zero-cost-most-steps"
	case "fewest trace bytes":
		return "zero-cost-fewest-trace-bytes"
	case "most trace bytes":
		return "zero-cost-most-trace-bytes"
	case "fewest error words":
		return "zero-cost-fewest-error-words"
	case "most error words":
		return "zero-cost-most-error-words"
	default:
		return ""
	}
}

func canonicalArmByID(id string) (ArmDefinition, bool) {
	index := slices.IndexFunc(canonicalArmDefinitions(), func(arm ArmDefinition) bool { return arm.ID == id })
	if index < 0 {
		return ArmDefinition{}, false
	}
	return canonicalArmDefinitions()[index], true
}

func zeroCostControlByArm(arm ArmDefinition) (baseline.Baseline, bool) {
	if arm.Kind != ArmZeroCostControl {
		return baseline.Baseline{}, false
	}
	index := slices.IndexFunc(canonicalZeroCostControls(), func(control baseline.Baseline) bool {
		return zeroCostArmID(control) == arm.ID && control.Name == arm.BaselineName && control.Feature == arm.BaselineFeature && string(control.Order) == arm.BaselineOrder
	})
	if index < 0 {
		return baseline.Baseline{}, false
	}
	return canonicalZeroCostControls()[index], true
}

func armSupportForRelation(arm ArmDefinition, relation Relation) (ArmSupport, string) {
	if arm.Kind != ArmZeroCostControl {
		return ArmSupported, ""
	}
	if relation.Applicability.Unit != UnitCandidatePair || !slices.ContainsFunc(relation.Constraints, func(value ExpectedConstraint) bool {
		return value.Required && value.Metric == MetricDecision
	}) {
		return ArmUnsupported, "zero_cost_control_requires_candidate_pair_decision_identity"
	}
	return ArmSupported, ""
}

func validateReplayedRelationCoverage(registry RelationRegistry, replayed []ReplayedRelationCaseV3) error {
	if err := registry.Validate(); err != nil {
		return err
	}
	if len(replayed) != registry.CoreCases+registry.SentinelCases {
		return fmt.Errorf("stress arm comparison replayed %d cases, want %d", len(replayed), registry.CoreCases+registry.SentinelCases)
	}
	replayIdentityDigest, err := replayedCorpusIdentityDigest(replayed)
	if err != nil {
		return err
	}
	if replayIdentityDigest != registry.ReplayCorpusIdentityDigest {
		return errors.New("stress arm comparison replay corpus differs from the release-committed case, split, task-group, manifest, or trajectory identities")
	}
	relationsByFamily := make(map[mutation.Family][]string, len(registry.CoreFamilies)+1)
	for _, relation := range registry.Relations {
		relationsByFamily[relation.Transform.MutationFamily] = append(relationsByFamily[relation.Transform.MutationFamily], relation.ID)
	}
	for family := range relationsByFamily {
		slices.Sort(relationsByFamily[family])
	}
	seen := make(map[string]struct{}, len(replayed))
	coverage := make(map[string]int, len(registry.Relations))
	for _, item := range replayed {
		if !identifierPattern.MatchString(item.CaseID) || !identifierPattern.MatchString(item.TaskGroupID) || !validDigest(item.ManifestDigest) ||
			len(item.Original) == 0 || len(item.Original) != len(item.Transformed) || !slices.Equal(item.RelationIDs, relationsByFamily[item.Family]) {
			return fmt.Errorf("stress arm comparison replay case %q is invalid or relation-incomplete", item.CaseID)
		}
		if _, exists := seen[item.CaseID]; exists {
			return fmt.Errorf("stress arm comparison replay case %q is duplicated", item.CaseID)
		}
		seen[item.CaseID] = struct{}{}
		for _, relationID := range item.RelationIDs {
			coverage[relationID]++
		}
	}
	for _, relation := range registry.Relations {
		want := registry.CoreCasesPerFamily
		if relation.StatisticalFamily.Estimand == EstimandScarcitySentinel {
			want = registry.SentinelCases
		}
		if coverage[relation.ID] != want {
			return fmt.Errorf("stress arm comparison relation %q covers %d cases, want %d", relation.ID, coverage[relation.ID], want)
		}
	}
	return nil
}

func comparisonCellID(armID, relationID, caseID string) (string, error) {
	digest, err := digestDocument(struct {
		ArmID      string `json:"arm_id"`
		RelationID string `json:"relation_id"`
		CaseID     string `json:"case_id"`
	}{armID, relationID, caseID})
	if err != nil {
		return "", err
	}
	return "cell-" + digest[:24], nil
}

func armComparisonPlanDigest(value ArmComparisonPlan) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
