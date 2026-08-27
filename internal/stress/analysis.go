package stress

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

const (
	StressAnalysisDesignSchemaVersion = "evalwitness.stress-analysis-design.v1"
	StressAnalysisReportSchemaVersion = "evalwitness.stress-analysis-report.v1"
	stressNominalAlpha                = 0.05
	stressIntervalMethod              = "source_task_cluster_wilson_score"
	stressMultiplicityMethod          = "bonferroni_within_estimand_across_supported_arm_relation_endpoints"
	stressContrastReferenceArm        = "score-token-verifier"
	stressClusterAggregation          = "any_relation_violation_and_any_non_satisfied_outcome_within_source_task_cluster"
	stressMissingnessPolicy           = "retain_every_supported_cell; any_not_run_cell_blocks_inference_without_leaving_the_denominator"
)

type AnalysisSplit string

const (
	AnalysisDevelopment AnalysisSplit = "development"
	AnalysisCalibration AnalysisSplit = "calibration"
	AnalysisTest        AnalysisSplit = "test"
)

type AnalysisStatus string

const (
	AnalysisAdjustedComplete AnalysisStatus = "adjusted_complete"
	AnalysisDescriptive      AnalysisStatus = "descriptive_complete"
	AnalysisIncomplete       AnalysisStatus = "incomplete"
	AnalysisNotRun           AnalysisStatus = "not_run"
	AnalysisUnsupported      AnalysisStatus = "unsupported"
)

type WitnessBindingStatus string

const (
	WitnessBoundPrivate                    WitnessBindingStatus = "bound_private"
	WitnessBoundPublic                     WitnessBindingStatus = "bound_public"
	WitnessMissingCapsule                  WitnessBindingStatus = "missing_capsule"
	WitnessMissingCounterexample           WitnessBindingStatus = "missing_counterexample"
	WitnessMissingCapsuleAndCounterexample WitnessBindingStatus = "missing_capsule_and_counterexample"
)

type StressAnalysisDesign struct {
	SchemaVersion                 string  `json:"schema_version"`
	CanonicalPolicy               string  `json:"canonical_policy"`
	RegistryDigest                string  `json:"registry_digest"`
	ArmPlanDigest                 string  `json:"arm_plan_digest"`
	PrimaryDataRole               string  `json:"primary_data_role"`
	ClusterUnit                   string  `json:"cluster_unit"`
	ClusterAggregation            string  `json:"cluster_aggregation"`
	MissingnessPolicy             string  `json:"missingness_policy"`
	IntervalMethod                string  `json:"interval_method"`
	NominalAlpha                  float64 `json:"nominal_alpha"`
	MultiplicityMethod            string  `json:"multiplicity_method"`
	PrimaryRateFamilySize         int     `json:"primary_rate_family_size"`
	SensitivityRateFamilySize     int     `json:"sensitivity_rate_family_size"`
	PrimaryContrastFamilySize     int     `json:"primary_contrast_family_size"`
	SensitivityContrastFamilySize int     `json:"sensitivity_contrast_family_size"`
	ContrastReferenceArm          string  `json:"contrast_reference_arm"`
	GlobalScore                   bool    `json:"global_score"`
	PopulationGeneralization      bool    `json:"population_generalization"`
	Digest                        string  `json:"digest"`
}

type AnalysisInterval struct {
	Estimate   float64 `json:"estimate"`
	Lower      float64 `json:"lower"`
	Upper      float64 `json:"upper"`
	Confidence float64 `json:"confidence"`
	Method     string  `json:"method"`
	Clusters   int     `json:"clusters"`
}

type AnalysisOutcomeCounts struct {
	Satisfied      int `json:"satisfied"`
	Violated       int `json:"violated"`
	Abstained      int `json:"abstained"`
	Invalid        int `json:"invalid"`
	Unsupported    int `json:"unsupported"`
	ProviderFailed int `json:"provider_failed"`
	Inconclusive   int `json:"inconclusive"`
	NotRun         int `json:"not_run"`
}

type AnalysisAdmissionCounts struct {
	FormalOnly      int `json:"formal_only"`
	HumanSupported  int `json:"human_supported"`
	HumanUnresolved int `json:"human_unresolved"`
}

type RelationArmAnalysis struct {
	SummaryID                  string                  `json:"summary_id"`
	ArmID                      string                  `json:"arm_id"`
	RelationID                 string                  `json:"relation_id"`
	RelationDigest             string                  `json:"relation_digest"`
	MutationFamily             string                  `json:"mutation_family"`
	Estimand                   Estimand                `json:"estimand"`
	Split                      AnalysisSplit           `json:"split"`
	DenominatorPolicy          DenominatorPolicy       `json:"denominator_policy"`
	MultiplicityMethod         string                  `json:"multiplicity_method"`
	PlannedCells               int                     `json:"planned_cells"`
	SupportedCells             int                     `json:"supported_cells"`
	StructuralUnsupportedCells int                     `json:"structural_unsupported_cells"`
	CompletedCells             int                     `json:"completed_cells"`
	NotRunCells                int                     `json:"not_run_cells"`
	SourceTaskClusters         int                     `json:"source_task_clusters"`
	OutcomeCounts              AnalysisOutcomeCounts   `json:"outcome_counts"`
	AdmissionCounts            AnalysisAdmissionCounts `json:"admission_counts"`
	ViolatedClusters           int                     `json:"violated_clusters"`
	FailedClusters             int                     `json:"failed_clusters"`
	ViolationRate              *float64                `json:"violation_rate,omitempty"`
	FailureRate                *float64                `json:"failure_rate,omitempty"`
	ViolationInterval          *AnalysisInterval       `json:"violation_interval,omitempty"`
	FailureInterval            *AnalysisInterval       `json:"failure_interval,omitempty"`
	AdjustedAlpha              *float64                `json:"adjusted_alpha,omitempty"`
	CapsuleBoundCells          int                     `json:"capsule_bound_cells"`
	CapsuleMissingCells        int                     `json:"capsule_missing_cells"`
	Status                     AnalysisStatus          `json:"status"`
}

type ArmContrast struct {
	ContrastID            string                `json:"contrast_id"`
	ReferenceArmID        string                `json:"reference_arm_id"`
	ComparatorArmID       string                `json:"comparator_arm_id"`
	RelationID            string                `json:"relation_id"`
	RelationDigest        string                `json:"relation_digest"`
	Estimand              Estimand              `json:"estimand"`
	Split                 AnalysisSplit         `json:"split"`
	PairedClusters        int                   `json:"paired_clusters"`
	BothFailed            int                   `json:"both_failed"`
	ComparatorOnlyFailed  int                   `json:"comparator_only_failed"`
	ReferenceOnlyFailed   int                   `json:"reference_only_failed"`
	NeitherFailed         int                   `json:"neither_failed"`
	FailureRiskDifference *float64              `json:"failure_risk_difference,omitempty"`
	Interval              *stats.PairedInterval `json:"interval,omitempty"`
	RawPValue             *float64              `json:"raw_p_value,omitempty"`
	AdjustedAlpha         *float64              `json:"adjusted_alpha,omitempty"`
	DifferenceDetected    bool                  `json:"difference_detected"`
	Status                AnalysisStatus        `json:"status"`
}

type MinimalWitnessBinding struct {
	CellID               string               `json:"cell_id"`
	ResultDigest         string               `json:"result_digest"`
	RelationDigest       string               `json:"relation_digest"`
	CaseID               string               `json:"case_id"`
	CapsuleDigest        string               `json:"capsule_digest,omitempty"`
	CounterexampleDigest string               `json:"counterexample_digest,omitempty"`
	PublicReleaseAllowed bool                 `json:"public_release_allowed"`
	Status               WitnessBindingStatus `json:"status"`
}

type StressAnalysisTotals struct {
	PlannedCells          int `json:"planned_cells"`
	SupportedCells        int `json:"supported_cells"`
	StructuralUnsupported int `json:"structural_unsupported_cells"`
	CompletedCells        int `json:"completed_cells"`
	NotRunCells           int `json:"not_run_cells"`
	ViolatedCells         int `json:"violated_cells"`
	WitnessesRequired     int `json:"witnesses_required"`
	WitnessesBound        int `json:"witnesses_bound"`
	PublicWitnessesBound  int `json:"public_witnesses_bound"`
	WitnessesMissing      int `json:"witnesses_missing"`
}

type StressAnalysisReport struct {
	SchemaVersion    string                  `json:"schema_version"`
	CanonicalPolicy  string                  `json:"canonical_policy"`
	DesignDigest     string                  `json:"design_digest"`
	ArmReportDigest  string                  `json:"arm_report_digest"`
	Summaries        []RelationArmAnalysis   `json:"summaries"`
	Contrasts        []ArmContrast           `json:"contrasts"`
	MinimalWitnesses []MinimalWitnessBinding `json:"minimal_witnesses"`
	Totals           StressAnalysisTotals    `json:"totals"`
	GlobalScore      bool                    `json:"global_score"`
	Digest           string                  `json:"digest"`
}

func BuildStressAnalysisDesign(plan ArmComparisonPlan, registry RelationRegistry, replayed []ReplayedRelationCaseV3) (StressAnalysisDesign, error) {
	value, err := buildStressAnalysisDesignUnchecked(plan, registry, replayed)
	if err != nil {
		return StressAnalysisDesign{}, err
	}
	value.Digest, err = stressAnalysisDesignDigest(value)
	if err != nil {
		return StressAnalysisDesign{}, err
	}
	if err := value.ValidateAgainst(plan, registry, replayed); err != nil {
		return StressAnalysisDesign{}, err
	}
	return value, nil
}

func (value StressAnalysisDesign) ValidateAgainst(plan ArmComparisonPlan, registry RelationRegistry, replayed []ReplayedRelationCaseV3) error {
	if err := value.Validate(); err != nil {
		return err
	}
	want, err := buildStressAnalysisDesignUnchecked(plan, registry, replayed)
	if err != nil {
		return err
	}
	want.Digest, err = stressAnalysisDesignDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress analysis design differs from the locked corpus, arms, or inference contract")
	}
	return nil
}

func (value StressAnalysisDesign) Validate() error {
	if value.SchemaVersion != StressAnalysisDesignSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.RegistryDigest) || !validDigest(value.ArmPlanDigest) || value.PrimaryDataRole != string(AnalysisTest) ||
		value.ClusterUnit != "source_task" || value.ClusterAggregation != stressClusterAggregation ||
		value.MissingnessPolicy != stressMissingnessPolicy || value.IntervalMethod != stressIntervalMethod ||
		value.NominalAlpha != stressNominalAlpha || value.MultiplicityMethod != stressMultiplicityMethod ||
		value.PrimaryRateFamilySize <= 0 || value.SensitivityRateFamilySize <= 0 ||
		value.PrimaryContrastFamilySize <= 0 || value.SensitivityContrastFamilySize <= 0 ||
		value.ContrastReferenceArm != stressContrastReferenceArm || value.GlobalScore || value.PopulationGeneralization {
		return errors.New("stress analysis design identity, inference family, or claim boundary is invalid")
	}
	expectedDigest, err := stressAnalysisDesignDigest(value)
	if err != nil || value.Digest != expectedDigest {
		return errors.New("stress analysis design digest is invalid")
	}
	return nil
}

func buildStressAnalysisDesignUnchecked(plan ArmComparisonPlan, registry RelationRegistry, replayed []ReplayedRelationCaseV3) (StressAnalysisDesign, error) {
	if err := plan.ValidateAgainst(registry, replayed); err != nil {
		return StressAnalysisDesign{}, err
	}
	primaryRates, sensitivityRates, primaryContrasts, sensitivityContrasts := analysisFamilySizes(plan, registry, replayed)
	if primaryRates == 0 || sensitivityRates == 0 || primaryContrasts == 0 || sensitivityContrasts == 0 {
		return StressAnalysisDesign{}, errors.New("stress analysis design has an empty confirmatory or sensitivity family")
	}
	return StressAnalysisDesign{
		SchemaVersion: StressAnalysisDesignSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RegistryDigest: registry.Digest, ArmPlanDigest: plan.Digest, PrimaryDataRole: string(AnalysisTest),
		ClusterUnit: "source_task", ClusterAggregation: stressClusterAggregation, MissingnessPolicy: stressMissingnessPolicy,
		IntervalMethod: stressIntervalMethod, NominalAlpha: stressNominalAlpha, MultiplicityMethod: stressMultiplicityMethod,
		PrimaryRateFamilySize: primaryRates, SensitivityRateFamilySize: sensitivityRates,
		PrimaryContrastFamilySize: primaryContrasts, SensitivityContrastFamilySize: sensitivityContrasts,
		ContrastReferenceArm: stressContrastReferenceArm, GlobalScore: false, PopulationGeneralization: false,
	}, nil
}

func BuildStressAnalysisReport(
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	armReport ArmComparisonReport,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	replayEvidence []ArmReplayEvidence,
	zeroCostEvidence []ZeroCostExecution,
	protocolProof *ProtocolAdapterProof,
	counterexamples []Counterexample,
) (StressAnalysisReport, error) {
	value, err := buildStressAnalysisReportUnchecked(
		design, plan, armReport, registry, replayed, replayEvidence, zeroCostEvidence, protocolProof, counterexamples,
	)
	if err != nil {
		return StressAnalysisReport{}, err
	}
	value.Digest, err = stressAnalysisReportDigest(value)
	if err != nil {
		return StressAnalysisReport{}, err
	}
	if err := value.ValidateAgainst(
		design, plan, armReport, registry, replayed, replayEvidence, zeroCostEvidence, protocolProof, counterexamples,
	); err != nil {
		return StressAnalysisReport{}, err
	}
	return value, nil
}

func (value StressAnalysisReport) ValidateAgainst(
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	armReport ArmComparisonReport,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	replayEvidence []ArmReplayEvidence,
	zeroCostEvidence []ZeroCostExecution,
	protocolProof *ProtocolAdapterProof,
	counterexamples []Counterexample,
) error {
	if value.SchemaVersion != StressAnalysisReportSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.DesignDigest != design.Digest || value.ArmReportDigest != armReport.Digest || value.GlobalScore || !validDigest(value.Digest) {
		return errors.New("stress analysis report identity or global-score boundary is invalid")
	}
	want, err := buildStressAnalysisReportUnchecked(
		design, plan, armReport, registry, replayed, replayEvidence, zeroCostEvidence, protocolProof, counterexamples,
	)
	if err != nil {
		return err
	}
	want.Digest, err = stressAnalysisReportDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return fmt.Errorf(
			"stress analysis report differs from its design, arm ledger, or witness evidence: summaries=%t contrasts=%t witnesses=%t totals=%t digest=%t",
			reflect.DeepEqual(value.Summaries, want.Summaries), reflect.DeepEqual(value.Contrasts, want.Contrasts),
			reflect.DeepEqual(value.MinimalWitnesses, want.MinimalWitnesses), value.Totals == want.Totals, value.Digest == want.Digest,
		)
	}
	return nil
}

func buildStressAnalysisReportUnchecked(
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	armReport ArmComparisonReport,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	replayEvidence []ArmReplayEvidence,
	zeroCostEvidence []ZeroCostExecution,
	protocolProof *ProtocolAdapterProof,
	counterexamples []Counterexample,
) (StressAnalysisReport, error) {
	if err := design.ValidateAgainst(plan, registry, replayed); err != nil {
		return StressAnalysisReport{}, err
	}
	if err := armReport.ValidateAgainst(plan, registry, replayed, replayEvidence, zeroCostEvidence, protocolProof); err != nil {
		return StressAnalysisReport{}, err
	}
	if armReport.PlanDigest != plan.Digest || design.ArmPlanDigest != plan.Digest {
		return StressAnalysisReport{}, errors.New("stress analysis arm report is invalid or belongs to another design")
	}
	relations := make(map[string]Relation, len(registry.Relations))
	for _, relation := range registry.Relations {
		relations[relation.ID] = relation
	}
	cases := make(map[string]ReplayedRelationCaseV3, len(replayed))
	for _, item := range replayed {
		cases[item.CaseID] = item
	}
	groups := make(map[string][]ArmComparisonObservation)
	for _, cell := range armReport.Cells {
		item, exists := cases[cell.CaseID]
		if !exists {
			return StressAnalysisReport{}, fmt.Errorf("stress analysis cell %q has no replayed case", cell.CellID)
		}
		split, err := analysisSplit(item.Split)
		if err != nil {
			return StressAnalysisReport{}, err
		}
		key := analysisGroupKey(cell.ArmID, cell.RelationID, split)
		groups[key] = append(groups[key], cell)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	value := StressAnalysisReport{
		SchemaVersion: StressAnalysisReportSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		DesignDigest: design.Digest, ArmReportDigest: armReport.Digest, GlobalScore: false,
	}
	for _, key := range keys {
		cells := groups[key]
		relation, exists := relations[cells[0].RelationID]
		if !exists {
			return StressAnalysisReport{}, fmt.Errorf("stress analysis relation %q is not registered", cells[0].RelationID)
		}
		split, err := analysisSplit(cases[cells[0].CaseID].Split)
		if err != nil {
			return StressAnalysisReport{}, err
		}
		summary, err := summarizeRelationArm(design, relation, split, cells)
		if err != nil {
			return StressAnalysisReport{}, err
		}
		value.Summaries = append(value.Summaries, summary)
	}
	contrasts, err := buildArmContrasts(design, armReport, registry, cases)
	if err != nil {
		return StressAnalysisReport{}, err
	}
	value.Contrasts = contrasts
	witnesses, err := bindMinimalWitnesses(armReport, counterexamples)
	if err != nil {
		return StressAnalysisReport{}, err
	}
	value.MinimalWitnesses = witnesses
	value.Totals = stressAnalysisTotals(armReport, witnesses)
	return value, nil
}

func analysisFamilySizes(plan ArmComparisonPlan, registry RelationRegistry, replayed []ReplayedRelationCaseV3) (int, int, int, int) {
	relations := make(map[string]Relation, len(registry.Relations))
	for _, relation := range registry.Relations {
		relations[relation.ID] = relation
	}
	testCases := make(map[string]bool, len(replayed))
	for _, item := range replayed {
		testCases[item.CaseID] = string(item.Split) == string(AnalysisTest)
	}
	rateEndpoints := map[Estimand]map[string]struct{}{EstimandPrimaryCore: {}, EstimandSensitivity: {}}
	contrastEndpoints := map[Estimand]map[string]struct{}{EstimandPrimaryCore: {}, EstimandSensitivity: {}}
	referenceSupport := make(map[string]bool)
	for _, cell := range plan.Cells {
		if !testCases[cell.CaseID] || cell.Support != ArmSupported {
			continue
		}
		relation := relations[cell.RelationID]
		if relation.StatisticalFamily.Estimand != EstimandPrimaryCore && relation.StatisticalFamily.Estimand != EstimandSensitivity {
			continue
		}
		endpoint := cell.ArmID + "\x00" + cell.RelationID
		rateEndpoints[relation.StatisticalFamily.Estimand][endpoint] = struct{}{}
		if cell.ArmID == stressContrastReferenceArm {
			referenceSupport[cell.RelationID+"\x00"+cell.CaseID] = true
		}
	}
	for _, cell := range plan.Cells {
		if !testCases[cell.CaseID] || cell.Support != ArmSupported || cell.ArmID == stressContrastReferenceArm ||
			!referenceSupport[cell.RelationID+"\x00"+cell.CaseID] {
			continue
		}
		relation := relations[cell.RelationID]
		if relation.StatisticalFamily.Estimand == EstimandPrimaryCore || relation.StatisticalFamily.Estimand == EstimandSensitivity {
			contrastEndpoints[relation.StatisticalFamily.Estimand][cell.ArmID+"\x00"+cell.RelationID] = struct{}{}
		}
	}
	return len(rateEndpoints[EstimandPrimaryCore]), len(rateEndpoints[EstimandSensitivity]),
		len(contrastEndpoints[EstimandPrimaryCore]), len(contrastEndpoints[EstimandSensitivity])
}

func summarizeRelationArm(design StressAnalysisDesign, relation Relation, split AnalysisSplit, cells []ArmComparisonObservation) (RelationArmAnalysis, error) {
	value := RelationArmAnalysis{
		ArmID: cells[0].ArmID, RelationID: relation.ID, RelationDigest: relation.Digest,
		MutationFamily: string(relation.Transform.MutationFamily), Estimand: relation.StatisticalFamily.Estimand, Split: split,
		DenominatorPolicy: relation.StatisticalFamily.DenominatorPolicy, MultiplicityMethod: relation.StatisticalFamily.MultiplicityMethod,
		PlannedCells: len(cells),
	}
	var err error
	value.SummaryID, err = analysisGroupID(value.ArmID, value.RelationID, split)
	if err != nil {
		return RelationArmAnalysis{}, err
	}
	clusters := make(map[string][]ArmComparisonObservation)
	for _, cell := range cells {
		switch cell.Support {
		case ArmSupported:
			value.SupportedCells++
			clusters[cell.TaskGroupID] = append(clusters[cell.TaskGroupID], cell)
		case ArmUnsupported:
			value.StructuralUnsupportedCells++
		}
		if cell.Status == ArmCellNotRun {
			value.NotRunCells++
			value.OutcomeCounts.NotRun++
			continue
		}
		if cell.Status != ArmCellExecuted {
			continue
		}
		value.CompletedCells++
		incrementAnalysisOutcome(&value.OutcomeCounts, cell.Outcome)
		incrementAnalysisAdmission(&value.AdmissionCounts, cell.AdmissionStatus)
		if cell.CapsuleDigest == "" {
			value.CapsuleMissingCells++
		} else {
			value.CapsuleBoundCells++
		}
	}
	value.SourceTaskClusters = len(clusters)
	if value.SupportedCells == 0 {
		value.Status = AnalysisUnsupported
		return value, nil
	}
	if value.CompletedCells == 0 {
		value.Status = AnalysisNotRun
		return value, nil
	}
	if value.NotRunCells != 0 {
		value.Status = AnalysisIncomplete
		return value, nil
	}
	for _, clusterCells := range clusters {
		violated, failed := false, false
		for _, cell := range clusterCells {
			violated = violated || cell.Outcome == OutcomeViolated
			failed = failed || cell.Outcome != OutcomeSatisfied
		}
		if violated {
			value.ViolatedClusters++
		}
		if failed {
			value.FailedClusters++
		}
	}
	violationRate := float64(value.ViolatedClusters) / float64(value.SourceTaskClusters)
	failureRate := float64(value.FailedClusters) / float64(value.SourceTaskClusters)
	value.ViolationRate, value.FailureRate = &violationRate, &failureRate
	alpha, adjusted := analysisAlpha(design, relation.StatisticalFamily.Estimand, split, false)
	confidence := 1 - alpha
	value.ViolationInterval = taskClusterWilsonInterval(value.ViolatedClusters, value.SourceTaskClusters, confidence, adjusted)
	value.FailureInterval = taskClusterWilsonInterval(value.FailedClusters, value.SourceTaskClusters, confidence, adjusted)
	if adjusted {
		value.AdjustedAlpha = &alpha
		value.Status = AnalysisAdjustedComplete
	} else {
		value.Status = AnalysisDescriptive
	}
	return value, nil
}

func buildArmContrasts(design StressAnalysisDesign, report ArmComparisonReport, registry RelationRegistry, cases map[string]ReplayedRelationCaseV3) ([]ArmContrast, error) {
	relations := make(map[string]Relation, len(registry.Relations))
	for _, relation := range registry.Relations {
		relations[relation.ID] = relation
	}
	byEndpoint := make(map[string]map[string]ArmComparisonObservation)
	for _, cell := range report.Cells {
		split, err := analysisSplit(cases[cell.CaseID].Split)
		if err != nil {
			return nil, err
		}
		key := cell.ArmID + "\x00" + cell.RelationID + "\x00" + string(split)
		if byEndpoint[key] == nil {
			byEndpoint[key] = make(map[string]ArmComparisonObservation)
		}
		byEndpoint[key][cell.CaseID] = cell
	}
	var result []ArmContrast
	for key, comparatorCells := range byEndpoint {
		parts := splitAnalysisKey(key)
		if parts[0] == stressContrastReferenceArm {
			continue
		}
		referenceKey := stressContrastReferenceArm + "\x00" + parts[1] + "\x00" + parts[2]
		referenceCells, exists := byEndpoint[referenceKey]
		if !exists {
			continue
		}
		relation := relations[parts[1]]
		split := AnalysisSplit(parts[2])
		contrast, err := summarizeArmContrast(design, relation, split, parts[0], referenceCells, comparatorCells, cases)
		if err != nil {
			return nil, err
		}
		if contrast.PairedClusters > 0 || contrast.Status == AnalysisUnsupported {
			result = append(result, contrast)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ContrastID < result[right].ContrastID })
	return result, nil
}

func summarizeArmContrast(
	design StressAnalysisDesign,
	relation Relation,
	split AnalysisSplit,
	comparator string,
	referenceCells, comparatorCells map[string]ArmComparisonObservation,
	cases map[string]ReplayedRelationCaseV3,
) (ArmContrast, error) {
	value := ArmContrast{
		ReferenceArmID: stressContrastReferenceArm, ComparatorArmID: comparator,
		RelationID: relation.ID, RelationDigest: relation.Digest, Estimand: relation.StatisticalFamily.Estimand, Split: split,
	}
	contrastID, err := analysisContrastID(comparator, relation.ID, split)
	if err != nil {
		return ArmContrast{}, err
	}
	value.ContrastID = contrastID
	type pair struct{ reference, comparator []ArmComparisonObservation }
	clusters := make(map[string]pair)
	for caseID, comparatorCell := range comparatorCells {
		referenceCell, exists := referenceCells[caseID]
		if !exists || comparatorCell.Support != ArmSupported || referenceCell.Support != ArmSupported {
			continue
		}
		clusterID := cases[caseID].TaskGroupID
		cluster := clusters[clusterID]
		cluster.reference = append(cluster.reference, referenceCell)
		cluster.comparator = append(cluster.comparator, comparatorCell)
		clusters[clusterID] = cluster
	}
	value.PairedClusters = len(clusters)
	if value.PairedClusters == 0 {
		value.Status = AnalysisUnsupported
		return value, nil
	}
	complete := true
	completedCells := 0
	for _, cluster := range clusters {
		for _, cell := range cluster.reference {
			if cell.Status == ArmCellExecuted {
				completedCells++
			}
		}
		for _, cell := range cluster.comparator {
			if cell.Status == ArmCellExecuted {
				completedCells++
			}
		}
		if slices.ContainsFunc(cluster.reference, func(cell ArmComparisonObservation) bool { return cell.Status != ArmCellExecuted }) ||
			slices.ContainsFunc(cluster.comparator, func(cell ArmComparisonObservation) bool { return cell.Status != ArmCellExecuted }) {
			complete = false
		}
	}
	if !complete {
		value.Status = AnalysisIncomplete
		if completedCells == 0 {
			value.Status = AnalysisNotRun
		}
		return value, nil
	}
	for _, cluster := range clusters {
		referenceFailed := slices.ContainsFunc(cluster.reference, func(cell ArmComparisonObservation) bool { return cell.Outcome != OutcomeSatisfied })
		comparatorFailed := slices.ContainsFunc(cluster.comparator, func(cell ArmComparisonObservation) bool { return cell.Outcome != OutcomeSatisfied })
		switch {
		case referenceFailed && comparatorFailed:
			value.BothFailed++
		case comparatorFailed:
			value.ComparatorOnlyFailed++
		case referenceFailed:
			value.ReferenceOnlyFailed++
		default:
			value.NeitherFailed++
		}
	}
	alpha, adjusted := analysisAlpha(design, relation.StatisticalFamily.Estimand, split, true)
	interval, err := stats.PairedBinaryScoreInterval(
		value.BothFailed, value.ComparatorOnlyFailed, value.ReferenceOnlyFailed, value.NeitherFailed, 1-alpha,
	)
	if err != nil {
		value.Status = AnalysisIncomplete
		return value, nil
	}
	effect := interval.Estimate
	value.FailureRiskDifference, value.Interval = &effect, &interval
	if !adjusted {
		value.Status = AnalysisDescriptive
		return value, nil
	}
	pValue := stats.McNemarExact(value.ComparatorOnlyFailed, value.ReferenceOnlyFailed)
	value.RawPValue, value.AdjustedAlpha = &pValue, &alpha
	value.DifferenceDetected = pValue < alpha && (interval.Lower > 0 || interval.Upper < 0)
	value.Status = AnalysisAdjustedComplete
	return value, nil
}

func bindMinimalWitnesses(report ArmComparisonReport, counterexamples []Counterexample) ([]MinimalWitnessBinding, error) {
	byResult := make(map[string]Counterexample, len(counterexamples))
	for _, counterexample := range counterexamples {
		if err := counterexample.Validate(); err != nil {
			return nil, err
		}
		if _, exists := byResult[counterexample.SourceResultDigest]; exists {
			return nil, fmt.Errorf("stress analysis has duplicate counterexamples for result %q", counterexample.SourceResultDigest)
		}
		byResult[counterexample.SourceResultDigest] = counterexample
	}
	used := make(map[string]struct{}, len(counterexamples))
	var result []MinimalWitnessBinding
	for _, cell := range report.Cells {
		if cell.Status != ArmCellExecuted || cell.Outcome != OutcomeViolated {
			continue
		}
		value := MinimalWitnessBinding{
			CellID: cell.CellID, ResultDigest: cell.ResultDigest, RelationDigest: cell.RelationDigest,
			CaseID: cell.CaseID, CapsuleDigest: cell.CapsuleDigest,
		}
		counterexample, hasCounterexample := byResult[cell.ResultDigest]
		if hasCounterexample {
			if counterexample.RelationDigest != cell.RelationDigest || counterexample.CaseID != cell.CaseID {
				return nil, fmt.Errorf("stress counterexample %q crosses its violated result cell", counterexample.Digest)
			}
			value.CounterexampleDigest = counterexample.Digest
			value.PublicReleaseAllowed = counterexample.PublicReleaseAllowed
			used[counterexample.Digest] = struct{}{}
		}
		switch {
		case cell.CapsuleDigest != "" && hasCounterexample && counterexample.PublicReleaseAllowed:
			value.Status = WitnessBoundPublic
		case cell.CapsuleDigest != "" && hasCounterexample:
			value.Status = WitnessBoundPrivate
		case cell.CapsuleDigest == "" && !hasCounterexample:
			value.Status = WitnessMissingCapsuleAndCounterexample
		case cell.CapsuleDigest == "":
			value.Status = WitnessMissingCapsule
		default:
			value.Status = WitnessMissingCounterexample
		}
		result = append(result, value)
	}
	if len(used) != len(counterexamples) {
		return nil, errors.New("stress analysis counterexample does not bind one violated arm result")
	}
	sort.Slice(result, func(left, right int) bool { return result[left].CellID < result[right].CellID })
	return result, nil
}

func stressAnalysisTotals(report ArmComparisonReport, witnesses []MinimalWitnessBinding) StressAnalysisTotals {
	value := StressAnalysisTotals{
		PlannedCells: report.PlannedCells, StructuralUnsupported: report.UnsupportedCells,
		CompletedCells: report.ExecutedCells, NotRunCells: report.NotRunCells,
	}
	value.SupportedCells = value.CompletedCells + value.NotRunCells
	for _, cell := range report.Cells {
		if cell.Status == ArmCellExecuted && cell.Outcome == OutcomeViolated {
			value.ViolatedCells++
		}
	}
	value.WitnessesRequired = len(witnesses)
	for _, witness := range witnesses {
		switch witness.Status {
		case WitnessBoundPrivate:
			value.WitnessesBound++
		case WitnessBoundPublic:
			value.WitnessesBound++
			value.PublicWitnessesBound++
		default:
			value.WitnessesMissing++
		}
	}
	return value
}

func analysisAlpha(design StressAnalysisDesign, estimand Estimand, split AnalysisSplit, contrast bool) (float64, bool) {
	if split != AnalysisTest || estimand != EstimandPrimaryCore && estimand != EstimandSensitivity {
		return design.NominalAlpha, false
	}
	familySize := design.PrimaryRateFamilySize
	if estimand == EstimandSensitivity {
		familySize = design.SensitivityRateFamilySize
	}
	if contrast {
		familySize = design.PrimaryContrastFamilySize
		if estimand == EstimandSensitivity {
			familySize = design.SensitivityContrastFamilySize
		}
	}
	return design.NominalAlpha / float64(familySize), true
}

func taskClusterWilsonInterval(successes, total int, confidence float64, adjusted bool) *AnalysisInterval {
	if total <= 0 {
		return nil
	}
	z := math.Sqrt2 * math.Erfinv(confidence)
	n := float64(total)
	p := float64(successes) / n
	z2 := z * z
	center := (p + z2/(2*n)) / (1 + z2/n)
	half := z * math.Sqrt((p*(1-p)+z2/(4*n))/n) / (1 + z2/n)
	method := stressIntervalMethod + "_descriptive"
	if adjusted {
		method = stressIntervalMethod + "_bonferroni"
	}
	return &AnalysisInterval{
		Estimate: p, Lower: math.Max(0, center-half), Upper: math.Min(1, center+half),
		Confidence: confidence, Method: method, Clusters: total,
	}
}

func incrementAnalysisOutcome(value *AnalysisOutcomeCounts, outcome Outcome) {
	switch outcome {
	case OutcomeSatisfied:
		value.Satisfied++
	case OutcomeViolated:
		value.Violated++
	case OutcomeAbstained:
		value.Abstained++
	case OutcomeInvalid:
		value.Invalid++
	case OutcomeUnsupported:
		value.Unsupported++
	case OutcomeProviderFailed:
		value.ProviderFailed++
	case OutcomeInconclusive:
		value.Inconclusive++
	}
}

func incrementAnalysisAdmission(value *AnalysisAdmissionCounts, status AdmissionStatus) {
	switch status {
	case AdmissionFormalOnly:
		value.FormalOnly++
	case AdmissionHumanSupported:
		value.HumanSupported++
	case AdmissionHumanUnresolved:
		value.HumanUnresolved++
	}
}

func analysisSplit[T ~string](value T) (AnalysisSplit, error) {
	split := AnalysisSplit(value)
	if !slices.Contains([]AnalysisSplit{AnalysisDevelopment, AnalysisCalibration, AnalysisTest}, split) {
		return "", fmt.Errorf("stress analysis data role %q is unsupported", split)
	}
	return split, nil
}

func analysisGroupKey(armID, relationID string, split AnalysisSplit) string {
	return armID + "\x00" + relationID + "\x00" + string(split)
}

func splitAnalysisKey(value string) [3]string {
	var result [3]string
	parts := []byte(value)
	start, index := 0, 0
	for position, char := range parts {
		if char == 0 && index < 2 {
			result[index] = string(parts[start:position])
			start, index = position+1, index+1
		}
	}
	result[index] = string(parts[start:])
	return result
}

func analysisGroupID(armID, relationID string, split AnalysisSplit) (string, error) {
	digest, err := digestDocument(struct {
		ArmID      string        `json:"arm_id"`
		RelationID string        `json:"relation_id"`
		Split      AnalysisSplit `json:"split"`
	}{armID, relationID, split})
	return digest, err
}

func analysisContrastID(comparator, relationID string, split AnalysisSplit) (string, error) {
	digest, err := digestDocument(struct {
		Reference  string        `json:"reference"`
		Comparator string        `json:"comparator"`
		RelationID string        `json:"relation_id"`
		Split      AnalysisSplit `json:"split"`
	}{stressContrastReferenceArm, comparator, relationID, split})
	return digest, err
}

func stressAnalysisDesignDigest(value StressAnalysisDesign) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func stressAnalysisReportDigest(value StressAnalysisReport) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
