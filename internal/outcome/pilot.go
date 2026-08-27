package outcome

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	PilotSampleSchemaVersion           = "evalwitness.outcome-pilot-sample.v1"
	PilotReadinessSchemaVersion        = "evalwitness.outcome-pilot-readiness.v1"
	OutcomePilotSampleSchemaVersion    = "evalwitness.outcome-pilot-sample.v2"
	OutcomePilotReadinessSchemaVersion = "evalwitness.outcome-pilot-readiness.v3"

	PilotSelectionRule           = "one development review-required mutation per prespecified family, choosing the lexicographically first eligible case without mutation task-group reuse; plus rank one from every nonempty prespecified natural stratum"
	OutcomePilotSelectionRule    = "rank one from every nonempty prespecified natural stratum; no mutation, pair, pair-order, or expected-relation unit is eligible"
	PilotQualificationMode       = "every reviewer must pass the governed public qualification set before assignment"
	PilotRequiredExternalAction  = "obtain explicit owner authorization before reviewer recruitment, contact, assignment, or packet sharing"
	OutcomePilotExpectedRelation = "single_trajectory_outcome"
)

type ReviewObjective string

const ReviewObjectiveOutcome ReviewObjective = "single_trajectory_outcome"

type PilotSource string

const (
	PilotSourceMutation PilotSource = "mutation"
	PilotSourceNatural  PilotSource = "natural"
)

type PilotTechnicalStatus string

const PilotTechnicalReady PilotTechnicalStatus = "ready_for_authorization"

type PilotExternalActionStatus string

const PilotExternalActionNotAuthorized PilotExternalActionStatus = "not_authorized"

type PilotCaseReference struct {
	Source                  PilotSource `json:"source"`
	Dimension               string      `json:"dimension"`
	TaskGroupID             string      `json:"task_group_id"`
	SourceCaseDigest        string      `json:"source_case_digest"`
	SelectionEvidenceDigest string      `json:"selection_evidence_digest"`
}

type PilotSampleCommitment struct {
	SchemaVersion             string               `json:"schema_version"`
	CanonicalPolicy           string               `json:"canonical_policy"`
	PlanDigest                string               `json:"plan_digest"`
	MutationSampleDigest      string               `json:"mutation_sample_digest"`
	MutationReleaseDigest     string               `json:"mutation_release_digest"`
	NaturalInventoryDigest    string               `json:"natural_inventory_digest"`
	DataRole                  ReviewDataRole       `json:"data_role"`
	SelectionRule             string               `json:"selection_rule"`
	RequiredMutationFamilies  []string             `json:"required_mutation_families"`
	SelectedNaturalStrata     []string             `json:"selected_natural_strata"`
	UnavailableNaturalStrata  []string             `json:"unavailable_natural_strata"`
	SelectedCases             int                  `json:"selected_cases"`
	RequiredPrimaryReviewers  int                  `json:"required_primary_reviewers"`
	RequiredTieBreakReviewers int                  `json:"required_tie_break_reviewers"`
	RequiredPrimaryLabels     int                  `json:"required_primary_labels"`
	MaximumTieBreakLabels     int                  `json:"maximum_tie_break_labels"`
	RequiredSourceProbes      int                  `json:"required_source_probes"`
	Cases                     []PilotCaseReference `json:"cases"`
	SelectionDigest           string               `json:"selection_digest"`
	Limitations               []string             `json:"limitations"`
	Digest                    string               `json:"digest"`
}

type PilotReadiness struct {
	SchemaVersion             string                    `json:"schema_version"`
	CanonicalPolicy           string                    `json:"canonical_policy"`
	PlanDigest                string                    `json:"plan_digest"`
	PilotSampleDigest         string                    `json:"pilot_sample_digest"`
	QualificationSetDigest    string                    `json:"qualification_set_digest"`
	HandbookDigest            string                    `json:"handbook_digest"`
	BundleDigest              string                    `json:"bundle_digest"`
	BlindingProtocolDigest    string                    `json:"blinding_protocol_digest"`
	DataRole                  ReviewDataRole            `json:"data_role"`
	Visibility                ReviewVisibility          `json:"visibility"`
	BundleCreatedAt           string                    `json:"bundle_created_at"`
	ProtocolCreatedAt         string                    `json:"protocol_created_at"`
	Packets                   int                       `json:"packets"`
	MappingReferences         []MappingReference        `json:"mapping_references"`
	MappingCommitmentDigest   string                    `json:"mapping_commitment_digest"`
	ConditionCandidates       []string                  `json:"condition_candidates"`
	RequiredPrimaryReviewers  int                       `json:"required_primary_reviewers"`
	RequiredTieBreakReviewers int                       `json:"required_tie_break_reviewers"`
	RequiredPrimaryLabels     int                       `json:"required_primary_labels"`
	MaximumTieBreakLabels     int                       `json:"maximum_tie_break_labels"`
	RequiredSourceProbes      int                       `json:"required_source_probes"`
	QualificationMode         string                    `json:"qualification_mode"`
	TechnicalStatus           PilotTechnicalStatus      `json:"technical_status"`
	ExternalActionStatus      PilotExternalActionStatus `json:"external_action_status"`
	RequiredExternalAction    string                    `json:"required_external_action"`
	PreparedAt                string                    `json:"prepared_at"`
	Limitations               []string                  `json:"limitations"`
	Digest                    string                    `json:"digest"`
}

type OutcomePilotCaseReference struct {
	Objective               ReviewObjective `json:"objective"`
	Stratum                 string          `json:"stratum"`
	TaskGroupID             string          `json:"task_group_id"`
	SourceCaseDigest        string          `json:"source_case_digest"`
	SelectionEvidenceDigest string          `json:"selection_evidence_digest"`
}

type OutcomePilotSampleCommitment struct {
	SchemaVersion             string                      `json:"schema_version"`
	CanonicalPolicy           string                      `json:"canonical_policy"`
	PlanDigest                string                      `json:"plan_digest"`
	NaturalInventoryDigest    string                      `json:"natural_inventory_digest"`
	Objective                 ReviewObjective             `json:"objective"`
	DataRole                  ReviewDataRole              `json:"data_role"`
	SelectionRule             string                      `json:"selection_rule"`
	SelectedNaturalStrata     []string                    `json:"selected_natural_strata"`
	UnavailableNaturalStrata  []string                    `json:"unavailable_natural_strata"`
	SelectedCases             int                         `json:"selected_cases"`
	RequiredPrimaryReviewers  int                         `json:"required_primary_reviewers"`
	RequiredTieBreakReviewers int                         `json:"required_tie_break_reviewers"`
	RequiredPrimaryLabels     int                         `json:"required_primary_labels"`
	MaximumTieBreakLabels     int                         `json:"maximum_tie_break_labels"`
	RequiredSourceProbes      int                         `json:"required_source_probes"`
	Cases                     []OutcomePilotCaseReference `json:"cases"`
	SelectionDigest           string                      `json:"selection_digest"`
	Limitations               []string                    `json:"limitations"`
	Digest                    string                      `json:"digest"`
}

type OutcomePilotReadiness struct {
	SchemaVersion             string                    `json:"schema_version"`
	CanonicalPolicy           string                    `json:"canonical_policy"`
	PlanDigest                string                    `json:"plan_digest"`
	PilotSampleDigest         string                    `json:"pilot_sample_digest"`
	QualificationSetDigest    string                    `json:"qualification_set_digest"`
	HandbookDigest            string                    `json:"handbook_digest"`
	BundleDigest              string                    `json:"bundle_digest"`
	BlindingProtocolDigest    string                    `json:"blinding_protocol_digest"`
	InspectionDigest          string                    `json:"inspection_digest"`
	Objective                 ReviewObjective           `json:"objective"`
	DataRole                  ReviewDataRole            `json:"data_role"`
	Visibility                ReviewVisibility          `json:"visibility"`
	BundleCreatedAt           string                    `json:"bundle_created_at"`
	ProtocolCreatedAt         string                    `json:"protocol_created_at"`
	Packets                   int                       `json:"packets"`
	MappingReferences         []MappingReference        `json:"mapping_references"`
	MappingCommitmentDigest   string                    `json:"mapping_commitment_digest"`
	ConditionCandidates       []string                  `json:"condition_candidates"`
	RequiredPrimaryReviewers  int                       `json:"required_primary_reviewers"`
	RequiredTieBreakReviewers int                       `json:"required_tie_break_reviewers"`
	RequiredPrimaryLabels     int                       `json:"required_primary_labels"`
	MaximumTieBreakLabels     int                       `json:"maximum_tie_break_labels"`
	RequiredSourceProbes      int                       `json:"required_source_probes"`
	QualificationMode         string                    `json:"qualification_mode"`
	TechnicalStatus           PilotTechnicalStatus      `json:"technical_status"`
	ExternalActionStatus      PilotExternalActionStatus `json:"external_action_status"`
	RequiredExternalAction    string                    `json:"required_external_action"`
	PreparedAt                string                    `json:"prepared_at"`
	Limitations               []string                  `json:"limitations"`
	Digest                    string                    `json:"digest"`
}

func BuildOutcomePilotSampleCommitment(plan Plan, inventory NaturalInventory) (OutcomePilotSampleCommitment, error) {
	if err := plan.Validate(); err != nil {
		return OutcomePilotSampleCommitment{}, err
	}
	if err := inventory.Validate(); err != nil {
		return OutcomePilotSampleCommitment{}, err
	}
	if inventory.PlanDigest != plan.Digest {
		return OutcomePilotSampleCommitment{}, errors.New("outcome pilot inventory does not bind the governed outcome plan")
	}
	cases, selectedStrata, unavailableStrata := selectOutcomePilotCases(plan, inventory.Selections)
	if len(cases) == 0 {
		return OutcomePilotSampleCommitment{}, errors.New("outcome pilot has no populated natural stratum")
	}
	return SealOutcomePilotSampleCommitment(OutcomePilotSampleCommitment{
		PlanDigest: plan.Digest, NaturalInventoryDigest: inventory.Digest, Objective: ReviewObjectiveOutcome,
		DataRole: ReviewDataDevelopment, SelectionRule: OutcomePilotSelectionRule,
		SelectedNaturalStrata: selectedStrata, UnavailableNaturalStrata: unavailableStrata, SelectedCases: len(cases),
		RequiredPrimaryReviewers: plan.PrimaryAdjudicators, RequiredTieBreakReviewers: plan.TieBreakAdjudicators,
		RequiredPrimaryLabels: len(cases) * plan.PrimaryAdjudicators, MaximumTieBreakLabels: len(cases) * plan.TieBreakAdjudicators,
		RequiredSourceProbes: len(cases) * plan.PrimaryAdjudicators, Cases: cases,
		SelectionDigest: outcomePilotSelectionDigest(cases), Limitations: outcomePilotSampleLimitations(),
	})
}

func SealOutcomePilotSampleCommitment(commitment OutcomePilotSampleCommitment) (OutcomePilotSampleCommitment, error) {
	commitment.SchemaVersion, commitment.CanonicalPolicy, commitment.Digest = OutcomePilotSampleSchemaVersion, CanonicalPolicy, ""
	digest, err := outcomePilotSampleDigest(commitment)
	if err != nil {
		return OutcomePilotSampleCommitment{}, err
	}
	commitment.Digest = digest
	return commitment, commitment.Validate()
}

func (commitment OutcomePilotSampleCommitment) Validate() error {
	if commitment.SchemaVersion != OutcomePilotSampleSchemaVersion || commitment.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(commitment.PlanDigest) || !validDigest(commitment.NaturalInventoryDigest) || commitment.Objective != ReviewObjectiveOutcome ||
		commitment.DataRole != ReviewDataDevelopment || commitment.SelectionRule != OutcomePilotSelectionRule || commitment.SelectedCases != len(commitment.Cases) || commitment.SelectedCases < 1 ||
		commitment.RequiredPrimaryReviewers != 2 || commitment.RequiredTieBreakReviewers != 1 ||
		commitment.RequiredPrimaryLabels != commitment.SelectedCases*commitment.RequiredPrimaryReviewers ||
		commitment.MaximumTieBreakLabels != commitment.SelectedCases*commitment.RequiredTieBreakReviewers ||
		commitment.RequiredSourceProbes != commitment.SelectedCases*commitment.RequiredPrimaryReviewers {
		return errors.New("outcome pilot sample identity, objective, bindings, role, selection, or workload is invalid")
	}
	for name, values := range map[string][]string{
		"outcome pilot selected natural strata":    commitment.SelectedNaturalStrata,
		"outcome pilot unavailable natural strata": commitment.UnavailableNaturalStrata,
		"outcome pilot limitations":                commitment.Limitations,
	} {
		if err := uniqueSorted(name, values); err != nil {
			return err
		}
	}
	if len(commitment.SelectedNaturalStrata) != len(commitment.Cases) || !equalStrings(commitment.Limitations, outcomePilotSampleLimitations()) {
		return errors.New("outcome pilot must contain exactly one natural outcome unit per populated stratum")
	}
	selected := make(map[string]struct{}, len(commitment.SelectedNaturalStrata))
	for _, stratum := range commitment.SelectedNaturalStrata {
		selected[stratum] = struct{}{}
	}
	for _, stratum := range commitment.UnavailableNaturalStrata {
		if _, overlap := selected[stratum]; overlap {
			return errors.New("outcome pilot natural strata cannot be both selected and unavailable")
		}
	}
	seenGroups, seenCases := make(map[string]struct{}, len(commitment.Cases)), make(map[string]struct{}, len(commitment.Cases))
	for index, item := range commitment.Cases {
		_, expectedStratum := selected[item.Stratum]
		if item.Objective != ReviewObjectiveOutcome || !expectedStratum || !validTaskGroupID(item.TaskGroupID) || !validDigest(item.SourceCaseDigest) ||
			!validDigest(item.SelectionEvidenceDigest) || item.SourceCaseDigest != item.SelectionEvidenceDigest ||
			index > 0 && outcomePilotCaseIdentity(commitment.Cases[index-1]) >= outcomePilotCaseIdentity(item) {
			return errors.New("outcome pilot case references must be natural outcome units, complete, unique, and canonically sorted")
		}
		if _, duplicate := seenGroups[item.TaskGroupID]; duplicate {
			return errors.New("outcome pilot sample reuses a task group")
		}
		if _, duplicate := seenCases[item.SourceCaseDigest]; duplicate {
			return errors.New("outcome pilot sample reuses a source case")
		}
		seenGroups[item.TaskGroupID], seenCases[item.SourceCaseDigest] = struct{}{}, struct{}{}
	}
	if commitment.SelectionDigest != outcomePilotSelectionDigest(commitment.Cases) {
		return errors.New("outcome pilot selection digest is invalid")
	}
	expected, err := outcomePilotSampleDigest(commitment)
	if err != nil || commitment.Digest != expected {
		return errors.New("outcome pilot sample digest is invalid")
	}
	return nil
}

func BuildPilotSampleCommitment(plan Plan, sample SampleCommitment, release mutation.CorpusRelease, inventory NaturalInventory) (PilotSampleCommitment, error) {
	if err := plan.Validate(); err != nil {
		return PilotSampleCommitment{}, err
	}
	if err := sample.Validate(); err != nil {
		return PilotSampleCommitment{}, err
	}
	if err := release.Validate(); err != nil {
		return PilotSampleCommitment{}, err
	}
	if err := inventory.Validate(); err != nil {
		return PilotSampleCommitment{}, err
	}
	generatedSample, err := BuildMutationSampleCommitment(plan, release)
	if err != nil {
		return PilotSampleCommitment{}, err
	}
	if generatedSample.Digest != sample.Digest || sample.PlanDigest != plan.Digest || inventory.PlanDigest != plan.Digest {
		return PilotSampleCommitment{}, errors.New("pilot inputs do not reproduce one governed outcome plan and mutation sample")
	}
	cases, selectedStrata, unavailableStrata, err := selectPilotCases(plan, release.Cases, inventory.Selections)
	if err != nil {
		return PilotSampleCommitment{}, err
	}
	return SealPilotSampleCommitment(PilotSampleCommitment{
		PlanDigest: plan.Digest, MutationSampleDigest: sample.Digest, MutationReleaseDigest: release.Digest, NaturalInventoryDigest: inventory.Digest,
		DataRole: ReviewDataDevelopment, SelectionRule: PilotSelectionRule,
		RequiredMutationFamilies: append([]string(nil), plan.MutationFamilies...), SelectedNaturalStrata: selectedStrata, UnavailableNaturalStrata: unavailableStrata,
		SelectedCases: len(cases), RequiredPrimaryReviewers: plan.PrimaryAdjudicators, RequiredTieBreakReviewers: plan.TieBreakAdjudicators,
		RequiredPrimaryLabels: len(cases) * plan.PrimaryAdjudicators, MaximumTieBreakLabels: len(cases) * plan.TieBreakAdjudicators,
		RequiredSourceProbes: len(cases) * plan.PrimaryAdjudicators, Cases: cases, SelectionDigest: pilotSelectionDigest(cases), Limitations: pilotSampleLimitations(),
	})
}

func SealPilotSampleCommitment(commitment PilotSampleCommitment) (PilotSampleCommitment, error) {
	commitment.SchemaVersion, commitment.CanonicalPolicy, commitment.Digest = PilotSampleSchemaVersion, CanonicalPolicy, ""
	digest, err := pilotSampleDigest(commitment)
	if err != nil {
		return PilotSampleCommitment{}, err
	}
	commitment.Digest = digest
	return commitment, commitment.Validate()
}

func (commitment PilotSampleCommitment) Validate() error {
	if commitment.SchemaVersion != PilotSampleSchemaVersion || commitment.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(commitment.PlanDigest) || !validDigest(commitment.MutationSampleDigest) || !validDigest(commitment.MutationReleaseDigest) || !validDigest(commitment.NaturalInventoryDigest) ||
		commitment.DataRole != ReviewDataDevelopment || commitment.SelectionRule != PilotSelectionRule || commitment.SelectedCases != len(commitment.Cases) || commitment.SelectedCases < 2 ||
		commitment.RequiredPrimaryReviewers != 2 || commitment.RequiredTieBreakReviewers != 1 ||
		commitment.RequiredPrimaryLabels != commitment.SelectedCases*commitment.RequiredPrimaryReviewers ||
		commitment.MaximumTieBreakLabels != commitment.SelectedCases*commitment.RequiredTieBreakReviewers ||
		commitment.RequiredSourceProbes != commitment.SelectedCases*commitment.RequiredPrimaryReviewers {
		return errors.New("pilot sample identity, bindings, role, selection, or workload is invalid")
	}
	for name, values := range map[string][]string{
		"pilot mutation families":          commitment.RequiredMutationFamilies,
		"pilot selected natural strata":    commitment.SelectedNaturalStrata,
		"pilot unavailable natural strata": commitment.UnavailableNaturalStrata,
		"pilot limitations":                commitment.Limitations,
	} {
		if err := uniqueSorted(name, values); err != nil {
			return err
		}
	}
	if len(commitment.RequiredMutationFamilies) == 0 || len(commitment.SelectedNaturalStrata) == 0 || !equalStrings(commitment.Limitations, pilotSampleLimitations()) {
		return errors.New("pilot sample must contain mutation and natural evidence and preserve its frozen limitations")
	}
	selectedNatural := make(map[string]struct{}, len(commitment.SelectedNaturalStrata))
	for _, stratum := range commitment.SelectedNaturalStrata {
		selectedNatural[stratum] = struct{}{}
	}
	for _, stratum := range commitment.UnavailableNaturalStrata {
		if _, overlap := selectedNatural[stratum]; overlap {
			return errors.New("pilot natural strata cannot be both selected and unavailable")
		}
	}
	if len(commitment.Cases) != len(commitment.RequiredMutationFamilies)+len(commitment.SelectedNaturalStrata) {
		return errors.New("pilot case count does not match one case per selected dimension")
	}
	if err := validatePilotCases(commitment); err != nil {
		return err
	}
	if commitment.SelectionDigest != pilotSelectionDigest(commitment.Cases) {
		return errors.New("pilot selection digest is invalid")
	}
	expected, err := pilotSampleDigest(commitment)
	if err != nil || commitment.Digest != expected {
		return errors.New("pilot sample digest is invalid")
	}
	return nil
}

func BuildPilotReadiness(pilot PilotSampleCommitment, bundle ReviewBundle, qualification QualificationSet, handbook ReviewerHandbook, protocol BlindingProtocol, mappings []PrivateMapping, preparedAt string) (PilotReadiness, error) {
	if err := pilot.Validate(); err != nil {
		return PilotReadiness{}, err
	}
	return PilotReadiness{}, errors.New("mixed pilot v1 is non-launchable because it combines outcome and controlled-relation review units; use outcome pilot v2")
}

func BuildOutcomePilotReadiness(pilot OutcomePilotSampleCommitment, bundle ReviewBundle, qualification QualificationSet, handbook ReviewerHandbook, protocol BlindingProtocol, inspection OutcomePilotInspection, mappings []PrivateMapping, preparedAt string) (OutcomePilotReadiness, error) {
	if err := pilot.Validate(); err != nil {
		return OutcomePilotReadiness{}, err
	}
	if err := bundle.Validate(); err != nil {
		return OutcomePilotReadiness{}, err
	}
	if err := qualification.Validate(); err != nil {
		return OutcomePilotReadiness{}, err
	}
	if err := VerifyReviewerHandbook(handbook, qualification); err != nil {
		return OutcomePilotReadiness{}, err
	}
	if err := protocol.Validate(); err != nil {
		return OutcomePilotReadiness{}, err
	}
	if err := inspection.Validate(); err != nil {
		return OutcomePilotReadiness{}, err
	}
	if bundle.PlanDigest != pilot.PlanDigest || bundle.DataRole != pilot.DataRole || bundle.Visibility != ReviewVisibilityRestricted ||
		bundle.QualificationSetDigest != qualification.Digest || bundle.HandbookDigest != handbook.Digest || bundle.HandbookVersion != handbook.HandbookVersion ||
		protocol.BundleDigest != bundle.Digest || inspection.PilotSampleDigest != pilot.Digest || inspection.BundleDigest != bundle.Digest ||
		inspection.Objective != ReviewObjectiveOutcome || inspection.Packets != pilot.SelectedCases ||
		inspection.ReviewabilityStatus != OutcomePilotStructurallyReady || inspection.SemanticStatus != OutcomePilotRequiresHumanPilot || len(bundle.Items) != pilot.SelectedCases {
		return OutcomePilotReadiness{}, errors.New("outcome pilot readiness inputs do not share the frozen plan, restricted role, governance, protocol, and case count")
	}
	references, err := mappingReferences(bundle, mappings)
	if err != nil {
		return OutcomePilotReadiness{}, errors.New("outcome pilot readiness mappings do not reproduce the review bundle")
	}
	if err := validateOutcomePilotBundleCoverage(pilot, bundle, mappings); err != nil {
		return OutcomePilotReadiness{}, err
	}
	if err := requireStrictlyAfter("outcome pilot readiness", preparedAt, bundle.CreatedAt); err != nil {
		return OutcomePilotReadiness{}, err
	}
	if err := requireStrictlyAfter("outcome pilot readiness", preparedAt, protocol.CreatedAt); err != nil {
		return OutcomePilotReadiness{}, err
	}
	mappingDigest, err := digestJSON(references)
	if err != nil {
		return OutcomePilotReadiness{}, err
	}
	if inspection.MappingCommitmentDigest != mappingDigest {
		return OutcomePilotReadiness{}, errors.New("outcome pilot readiness inspection does not bind the exact owner-only mapping commitment")
	}
	return SealOutcomePilotReadiness(OutcomePilotReadiness{
		PlanDigest: pilot.PlanDigest, PilotSampleDigest: pilot.Digest, QualificationSetDigest: qualification.Digest, HandbookDigest: handbook.Digest,
		BundleDigest: bundle.Digest, BlindingProtocolDigest: protocol.Digest, InspectionDigest: inspection.Digest, Objective: ReviewObjectiveOutcome,
		DataRole: bundle.DataRole, Visibility: bundle.Visibility, BundleCreatedAt: bundle.CreatedAt, ProtocolCreatedAt: protocol.CreatedAt, Packets: len(bundle.Items),
		MappingReferences: references, MappingCommitmentDigest: mappingDigest, ConditionCandidates: append([]string(nil), protocol.ConditionCandidates...),
		RequiredPrimaryReviewers: pilot.RequiredPrimaryReviewers, RequiredTieBreakReviewers: pilot.RequiredTieBreakReviewers,
		RequiredPrimaryLabels: pilot.RequiredPrimaryLabels, MaximumTieBreakLabels: pilot.MaximumTieBreakLabels, RequiredSourceProbes: pilot.RequiredSourceProbes,
		QualificationMode: PilotQualificationMode, TechnicalStatus: PilotTechnicalReady, ExternalActionStatus: PilotExternalActionNotAuthorized,
		RequiredExternalAction: PilotRequiredExternalAction, PreparedAt: preparedAt, Limitations: outcomePilotReadinessLimitations(),
	})
}

func SealOutcomePilotReadiness(readiness OutcomePilotReadiness) (OutcomePilotReadiness, error) {
	readiness.SchemaVersion, readiness.CanonicalPolicy, readiness.Digest = OutcomePilotReadinessSchemaVersion, CanonicalPolicy, ""
	digest, err := outcomePilotReadinessDigest(readiness)
	if err != nil {
		return OutcomePilotReadiness{}, err
	}
	readiness.Digest = digest
	return readiness, readiness.Validate()
}

func (readiness OutcomePilotReadiness) Validate() error {
	if readiness.SchemaVersion != OutcomePilotReadinessSchemaVersion || readiness.CanonicalPolicy != CanonicalPolicy || readiness.Objective != ReviewObjectiveOutcome ||
		!validDigest(readiness.PlanDigest) || !validDigest(readiness.PilotSampleDigest) || !validDigest(readiness.QualificationSetDigest) || !validDigest(readiness.HandbookDigest) ||
		!validDigest(readiness.BundleDigest) || !validDigest(readiness.BlindingProtocolDigest) || !validDigest(readiness.InspectionDigest) || !validDigest(readiness.MappingCommitmentDigest) ||
		readiness.DataRole != ReviewDataDevelopment || readiness.Visibility != ReviewVisibilityRestricted || readiness.Packets < 1 || len(readiness.MappingReferences) != readiness.Packets ||
		readiness.RequiredPrimaryReviewers != 2 || readiness.RequiredTieBreakReviewers != 1 || readiness.RequiredPrimaryLabels != readiness.Packets*2 ||
		readiness.MaximumTieBreakLabels != readiness.Packets || readiness.RequiredSourceProbes != readiness.Packets*2 ||
		readiness.QualificationMode != PilotQualificationMode || readiness.TechnicalStatus != PilotTechnicalReady ||
		readiness.ExternalActionStatus != PilotExternalActionNotAuthorized || readiness.RequiredExternalAction != PilotRequiredExternalAction {
		return errors.New("outcome pilot readiness identity, objective, governance, workload, or authorization boundary is invalid")
	}
	if err := validateMappingReferences(readiness.MappingReferences); err != nil {
		return err
	}
	mappingDigest, err := digestJSON(readiness.MappingReferences)
	if err != nil || readiness.MappingCommitmentDigest != mappingDigest {
		return errors.New("outcome pilot readiness mapping commitment is invalid")
	}
	if err := uniqueSorted("outcome pilot readiness condition candidates", readiness.ConditionCandidates); err != nil {
		return err
	}
	if len(readiness.ConditionCandidates) < 2 || !equalStrings(readiness.Limitations, outcomePilotReadinessLimitations()) {
		return errors.New("outcome pilot readiness candidate universe or frozen limitations are invalid")
	}
	for name, value := range map[string]string{"bundle": readiness.BundleCreatedAt, "protocol": readiness.ProtocolCreatedAt, "readiness": readiness.PreparedAt} {
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return fmt.Errorf("outcome pilot %s timestamp must be RFC3339", name)
		}
	}
	if err := requireStrictlyAfter("outcome pilot readiness", readiness.PreparedAt, readiness.BundleCreatedAt); err != nil {
		return err
	}
	if err := requireStrictlyAfter("outcome pilot readiness", readiness.PreparedAt, readiness.ProtocolCreatedAt); err != nil {
		return err
	}
	if err := requireNotBefore("outcome pilot protocol", readiness.ProtocolCreatedAt, readiness.BundleCreatedAt); err != nil {
		return err
	}
	expected, err := outcomePilotReadinessDigest(readiness)
	if err != nil || readiness.Digest != expected {
		return errors.New("outcome pilot readiness digest is invalid")
	}
	return nil
}

func SealPilotReadiness(readiness PilotReadiness) (PilotReadiness, error) {
	readiness.SchemaVersion, readiness.CanonicalPolicy, readiness.Digest = PilotReadinessSchemaVersion, CanonicalPolicy, ""
	digest, err := pilotReadinessDigest(readiness)
	if err != nil {
		return PilotReadiness{}, err
	}
	readiness.Digest = digest
	return readiness, readiness.Validate()
}

func (readiness PilotReadiness) Validate() error {
	if readiness.SchemaVersion != PilotReadinessSchemaVersion || readiness.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(readiness.PlanDigest) || !validDigest(readiness.PilotSampleDigest) || !validDigest(readiness.QualificationSetDigest) || !validDigest(readiness.HandbookDigest) ||
		!validDigest(readiness.BundleDigest) || !validDigest(readiness.BlindingProtocolDigest) || !validDigest(readiness.MappingCommitmentDigest) ||
		readiness.DataRole != ReviewDataDevelopment || !validReviewVisibility(readiness.Visibility) || readiness.Packets < 2 || len(readiness.MappingReferences) != readiness.Packets ||
		readiness.RequiredPrimaryReviewers != 2 || readiness.RequiredTieBreakReviewers != 1 || readiness.RequiredPrimaryLabels != readiness.Packets*2 ||
		readiness.MaximumTieBreakLabels != readiness.Packets || readiness.RequiredSourceProbes != readiness.Packets*2 ||
		readiness.QualificationMode != PilotQualificationMode || readiness.TechnicalStatus != PilotTechnicalReady ||
		readiness.ExternalActionStatus != PilotExternalActionNotAuthorized || readiness.RequiredExternalAction != PilotRequiredExternalAction {
		return errors.New("pilot readiness identity, governance, workload, or authorization boundary is invalid")
	}
	if err := validateMappingReferences(readiness.MappingReferences); err != nil {
		return err
	}
	mappingDigest, err := digestJSON(readiness.MappingReferences)
	if err != nil || readiness.MappingCommitmentDigest != mappingDigest {
		return errors.New("pilot readiness mapping commitment is invalid")
	}
	if err := uniqueSorted("pilot readiness condition candidates", readiness.ConditionCandidates); err != nil {
		return err
	}
	if len(readiness.ConditionCandidates) < 2 || !equalStrings(readiness.Limitations, pilotReadinessLimitations()) {
		return errors.New("pilot readiness candidate universe or frozen limitations are invalid")
	}
	for name, value := range map[string]string{"bundle": readiness.BundleCreatedAt, "protocol": readiness.ProtocolCreatedAt, "readiness": readiness.PreparedAt} {
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return fmt.Errorf("pilot %s timestamp must be RFC3339", name)
		}
	}
	if err := requireStrictlyAfter("pilot readiness", readiness.PreparedAt, readiness.BundleCreatedAt); err != nil {
		return err
	}
	if err := requireStrictlyAfter("pilot readiness", readiness.PreparedAt, readiness.ProtocolCreatedAt); err != nil {
		return err
	}
	if err := requireNotBefore("pilot protocol", readiness.ProtocolCreatedAt, readiness.BundleCreatedAt); err != nil {
		return err
	}
	expected, err := pilotReadinessDigest(readiness)
	if err != nil || readiness.Digest != expected {
		return errors.New("pilot readiness digest is invalid")
	}
	return nil
}

func DecodePilotSampleCommitment(reader io.Reader) (PilotSampleCommitment, error) {
	var value PilotSampleCommitment
	if err := decodeStrict(reader, &value); err != nil {
		return PilotSampleCommitment{}, fmt.Errorf("decode pilot sample commitment: %w", err)
	}
	return value, value.Validate()
}

func DecodePilotReadiness(reader io.Reader) (PilotReadiness, error) {
	var value PilotReadiness
	if err := decodeStrict(reader, &value); err != nil {
		return PilotReadiness{}, fmt.Errorf("decode pilot readiness: %w", err)
	}
	return value, value.Validate()
}

func DecodeOutcomePilotSampleCommitment(reader io.Reader) (OutcomePilotSampleCommitment, error) {
	var value OutcomePilotSampleCommitment
	if err := decodeStrict(reader, &value); err != nil {
		return OutcomePilotSampleCommitment{}, fmt.Errorf("decode outcome pilot sample commitment: %w", err)
	}
	return value, value.Validate()
}

func DecodeOutcomePilotReadiness(reader io.Reader) (OutcomePilotReadiness, error) {
	var value OutcomePilotReadiness
	if err := decodeStrict(reader, &value); err != nil {
		return OutcomePilotReadiness{}, fmt.Errorf("decode outcome pilot readiness: %w", err)
	}
	return value, value.Validate()
}

func selectOutcomePilotCases(plan Plan, naturalSelections []NaturalSelection) ([]OutcomePilotCaseReference, []string, []string) {
	naturalByStratum := make(map[string]NaturalSelection, len(plan.RequiredStrata))
	for _, selection := range naturalSelections {
		if selection.Rank == 1 {
			naturalByStratum[selection.Stratum] = selection
		}
	}
	var selectedStrata, unavailableStrata []string
	var result []OutcomePilotCaseReference
	for _, stratum := range plan.RequiredStrata {
		selection, exists := naturalByStratum[stratum]
		if !exists {
			unavailableStrata = append(unavailableStrata, stratum)
			continue
		}
		selectedStrata = append(selectedStrata, stratum)
		result = append(result, OutcomePilotCaseReference{
			Objective: ReviewObjectiveOutcome, Stratum: stratum, TaskGroupID: "group-" + selection.TaskGroupDigest,
			SourceCaseDigest: selection.CaseDigest, SelectionEvidenceDigest: selection.CaseDigest,
		})
	}
	sort.Strings(selectedStrata)
	sort.Strings(unavailableStrata)
	sort.Slice(result, func(left, right int) bool {
		return outcomePilotCaseIdentity(result[left]) < outcomePilotCaseIdentity(result[right])
	})
	return result, selectedStrata, unavailableStrata
}

func selectPilotCases(plan Plan, releaseCases []mutation.CorpusCase, naturalSelections []NaturalSelection) ([]PilotCaseReference, []string, []string, error) {
	mutationByFamily := make(map[string][]mutation.CorpusCase, len(plan.MutationFamilies))
	for _, item := range releaseCases {
		if item.Manifest.Review.Required && string(item.Split) == string(ReviewDataDevelopment) {
			mutationByFamily[string(item.Family)] = append(mutationByFamily[string(item.Family)], item)
		}
	}
	for family := range mutationByFamily {
		sort.Slice(mutationByFamily[family], func(left, right int) bool {
			return mutationByFamily[family][left].ID < mutationByFamily[family][right].ID
		})
	}
	usedGroups := make(map[string]struct{})
	result := make([]PilotCaseReference, 0, len(plan.MutationFamilies)+len(plan.RequiredStrata))
	for _, family := range plan.MutationFamilies {
		selected := false
		for _, item := range mutationByFamily[family] {
			if _, duplicate := usedGroups[item.Manifest.SplitGroupID]; duplicate {
				continue
			}
			usedGroups[item.Manifest.SplitGroupID] = struct{}{}
			result = append(result, PilotCaseReference{
				Source: PilotSourceMutation, Dimension: family, TaskGroupID: item.Manifest.SplitGroupID,
				SourceCaseDigest: item.Manifest.Digest, SelectionEvidenceDigest: item.BlindPacket.Digest,
			})
			selected = true
			break
		}
		if !selected {
			return nil, nil, nil, fmt.Errorf("pilot cannot select a unique development mutation task group for family %q", family)
		}
	}
	naturalByStratum := make(map[string]NaturalSelection, len(plan.RequiredStrata))
	for _, selection := range naturalSelections {
		if selection.Rank == 1 {
			naturalByStratum[selection.Stratum] = selection
		}
	}
	var selectedStrata, unavailableStrata []string
	for _, stratum := range plan.RequiredStrata {
		selection, exists := naturalByStratum[stratum]
		if !exists {
			unavailableStrata = append(unavailableStrata, stratum)
			continue
		}
		selectedStrata = append(selectedStrata, stratum)
		result = append(result, PilotCaseReference{
			Source: PilotSourceNatural, Dimension: stratum, TaskGroupID: "group-" + selection.TaskGroupDigest,
			SourceCaseDigest: selection.CaseDigest, SelectionEvidenceDigest: selection.CaseDigest,
		})
	}
	sort.Slice(result, func(left, right int) bool { return pilotCaseIdentity(result[left]) < pilotCaseIdentity(result[right]) })
	return result, selectedStrata, unavailableStrata, nil
}

func validatePilotCases(commitment PilotSampleCommitment) error {
	mutationFamilies := make(map[string]struct{}, len(commitment.RequiredMutationFamilies))
	for _, family := range commitment.RequiredMutationFamilies {
		mutationFamilies[family] = struct{}{}
	}
	naturalStrata := make(map[string]struct{}, len(commitment.SelectedNaturalStrata))
	for _, stratum := range commitment.SelectedNaturalStrata {
		naturalStrata[stratum] = struct{}{}
	}
	seenDimensions := make(map[string]struct{}, len(commitment.Cases))
	seenGroups := make(map[string]struct{}, len(commitment.Cases))
	seenCases := make(map[string]struct{}, len(commitment.Cases))
	for index, item := range commitment.Cases {
		if missing(item.Dimension) || !validTaskGroupID(item.TaskGroupID) || !validDigest(item.SourceCaseDigest) || !validDigest(item.SelectionEvidenceDigest) ||
			index > 0 && pilotCaseIdentity(commitment.Cases[index-1]) >= pilotCaseIdentity(item) {
			return errors.New("pilot case references must be complete, unique, and canonically sorted")
		}
		validDimension := false
		switch item.Source {
		case PilotSourceMutation:
			_, validDimension = mutationFamilies[item.Dimension]
		case PilotSourceNatural:
			_, validDimension = naturalStrata[item.Dimension]
		default:
			return errors.New("pilot case source is invalid")
		}
		if !validDimension {
			return errors.New("pilot case dimension is outside its committed source set")
		}
		dimensionKey := string(item.Source) + "\x00" + item.Dimension
		if _, duplicate := seenDimensions[dimensionKey]; duplicate {
			return errors.New("pilot sample contains more than one case for a committed dimension")
		}
		if _, duplicate := seenGroups[item.TaskGroupID]; duplicate {
			return errors.New("pilot sample reuses a task group")
		}
		if _, duplicate := seenCases[item.SourceCaseDigest]; duplicate {
			return errors.New("pilot sample reuses a source case")
		}
		seenDimensions[dimensionKey], seenGroups[item.TaskGroupID], seenCases[item.SourceCaseDigest] = struct{}{}, struct{}{}, struct{}{}
	}
	return nil
}

func validateOutcomePilotBundleCoverage(pilot OutcomePilotSampleCommitment, bundle ReviewBundle, mappings []PrivateMapping) error {
	pilotBySource := make(map[string]OutcomePilotCaseReference, len(pilot.Cases))
	for _, item := range pilot.Cases {
		pilotBySource[item.SourceCaseDigest] = item
	}
	itemByPacket := make(map[string]ReviewItem, len(bundle.Items))
	for _, item := range bundle.Items {
		if item.Packet.PublicReleasable || item.Packet.PrivacyClass != "restricted_reference_only" || len(item.Packet.Evidence) != 2 {
			return errors.New("outcome pilot bundle must contain restricted reference-only packets with exactly task and trajectory evidence")
		}
		kinds := []string{item.Packet.Evidence[0].Kind, item.Packet.Evidence[1].Kind}
		sort.Strings(kinds)
		if !equalStrings(kinds, []string{"task_requirement", "trajectory_evidence"}) {
			return errors.New("outcome pilot packet contains relation or non-outcome evidence")
		}
		itemByPacket[item.Packet.PacketID] = item
	}
	seenSources := make(map[string]struct{}, len(mappings))
	for _, mapping := range mappings {
		pilotCase, exists := pilotBySource[mapping.SourceCaseDigest]
		bundleItem, packetExists := itemByPacket[mapping.PacketID]
		expectedCondition := "natural." + pilotCase.Stratum
		if !exists || !packetExists || bundleItem.TaskGroupID != pilotCase.TaskGroupID || mapping.ExpectedRelation != OutcomePilotExpectedRelation || mapping.Condition != expectedCondition {
			return errors.New("outcome pilot bundle mapping does not bind the committed natural outcome case, stratum, objective, and task group")
		}
		if _, duplicate := seenSources[mapping.SourceCaseDigest]; duplicate {
			return errors.New("outcome pilot bundle maps one source case more than once")
		}
		seenSources[mapping.SourceCaseDigest] = struct{}{}
	}
	if len(seenSources) != len(pilot.Cases) {
		return errors.New("outcome pilot bundle does not cover every committed natural source case exactly once")
	}
	return nil
}

func pilotCaseIdentity(value PilotCaseReference) string {
	return string(value.Source) + "\x00" + value.Dimension + "\x00" + value.TaskGroupID + "\x00" + value.SourceCaseDigest + "\x00" + value.SelectionEvidenceDigest
}

func outcomePilotCaseIdentity(value OutcomePilotCaseReference) string {
	return string(value.Objective) + "\x00" + value.Stratum + "\x00" + value.TaskGroupID + "\x00" + value.SourceCaseDigest + "\x00" + value.SelectionEvidenceDigest
}

func pilotSelectionDigest(cases []PilotCaseReference) string {
	identities := make([]string, len(cases))
	for index, item := range cases {
		identities[index] = pilotCaseIdentity(item)
	}
	return digestText(joinWithNUL(identities))
}

func outcomePilotSelectionDigest(cases []OutcomePilotCaseReference) string {
	identities := make([]string, len(cases))
	for index, item := range cases {
		identities[index] = outcomePilotCaseIdentity(item)
	}
	return digestText(joinWithNUL(identities))
}

func pilotSampleLimitations() []string {
	return []string{
		"cross-source task overlap cannot be disproved because mutation and legacy natural inventories use different historical identity namespaces",
		"natural abstention and provider_failure strata remain unavailable in the governed development inventory and are not synthetically replaced",
	}
}

func pilotReadinessLimitations() []string {
	return []string{
		"readiness does not record reviewer recruitment, consent, qualification, assignment, labeling, adjudication, or external packet delivery",
		"readiness verifies identity and protocol bindings, not the semantic faithfulness of redaction or excerpt selection",
		"technical readiness does not authorize external contact or data sharing",
	}
}

func outcomePilotSampleLimitations() []string {
	return []string{
		"natural abstention and provider_failure strata remain unavailable in the governed development inventory and are not synthetically replaced",
		"the development pilot estimates rubric ambiguity and workflow feasibility; it is not a confirmatory verifier-performance evaluation",
	}
}

func outcomePilotReadinessLimitations() []string {
	return []string{
		"readiness does not record reviewer recruitment, consent, qualification, assignment, labeling, adjudication, or external packet delivery",
		"reference-only source material remains restricted even after deterministic secret redaction and evidence slicing",
		"reviewability inspection proves structural coverage and lineage but semantic sufficiency still requires the independent human development pilot",
		"technical readiness does not authorize external contact or data sharing",
	}
}

func pilotSampleDigest(value PilotSampleCommitment) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func pilotReadinessDigest(value PilotReadiness) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func outcomePilotSampleDigest(value OutcomePilotSampleCommitment) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func outcomePilotReadinessDigest(value OutcomePilotReadiness) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
