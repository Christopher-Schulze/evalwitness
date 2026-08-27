package mutation

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

const (
	CorpusSpecSchemaVersion    = "evalwitness.corruption-corpus-spec.v1"
	CorpusReleaseSchemaVersion = "evalwitness.corruption-corpus-release.v1"
	CorpusSplitAlgorithm       = "sha256-lineage-component-balanced-60-20-20.v1"
	relationCriticalSuccesses  = 29
	relationActualNullTail     = 0.0032132880478457347
	relationPowerAtAlternative = 0.9124947640780025
	relationDesignTolerance    = 1e-12
)

type CorpusSpec struct {
	SchemaVersion          string           `json:"schema_version"`
	CorpusVersion          string           `json:"corpus_version"`
	Seed                   string           `json:"seed"`
	SourceTasks            int              `json:"source_tasks"`
	TerminalTasks          int              `json:"terminal_tasks"`
	SWETasks               int              `json:"swe_tasks"`
	TrajectoriesPerTask    int              `json:"trajectories_per_task"`
	CasesPerFamily         int              `json:"cases_per_family"`
	PrimaryFamilies        []Family         `json:"primary_families"`
	MutationProgramDigest  string           `json:"mutation_program_digest"`
	DevelopmentAuditDigest string           `json:"development_audit_digest"`
	DesignEvidenceDigest   string           `json:"design_evidence_digest"`
	MutatorsFrozen         bool             `json:"mutators_frozen"`
	Design                 RelationDesign   `json:"design"`
	DevelopmentAudit       DevelopmentAudit `json:"development_audit"`
}

type DevelopmentAudit struct {
	Method                   string   `json:"method"`
	AuditedAt                string   `json:"audited_at"`
	AuditedReleaseDigest     string   `json:"audited_release_digest"`
	ObservedSources          int      `json:"observed_sources"`
	ObservedSourceTasks      int      `json:"observed_source_tasks"`
	ObservedCases            int      `json:"observed_cases"`
	ObservedPositiveControls int      `json:"observed_positive_controls"`
	AttemptedFamilies        []Family `json:"attempted_families"`
	ExcludedFamilies         []Family `json:"excluded_families"`
	Findings                 []string `json:"findings"`
	MutationProgramDigest    string   `json:"mutation_program_digest,omitempty"`
	ConstructAuditDigest     string   `json:"construct_audit_digest,omitempty"`
}

type RelationDesign struct {
	Method                 string  `json:"method"`
	PrimaryUnit            string  `json:"primary_unit"`
	NominalAlpha           float64 `json:"nominal_alpha"`
	FamilySize             int     `json:"family_size"`
	AdjustedAlpha          float64 `json:"adjusted_alpha"`
	NullAccuracy           float64 `json:"null_accuracy"`
	AlternativeAccuracy    float64 `json:"alternative_accuracy"`
	TargetPower            float64 `json:"target_power"`
	CriticalSuccesses      int     `json:"critical_successes"`
	ActualNullTail         float64 `json:"actual_null_tail"`
	PowerAtAlternative     float64 `json:"power_at_alternative"`
	UniqueTaskWithinFamily bool    `json:"unique_task_within_family"`
	CrossFamilyClusterUnit string  `json:"cross_family_cluster_unit"`
}

type CorpusSource struct {
	ID               string                  `json:"id"`
	TaskID           string                  `json:"task_id"`
	RepositoryID     string                  `json:"repository_id"`
	SourceFamily     string                  `json:"source_family"`
	SourceFormat     preprocess.SourceFormat `json:"source_format"`
	SourceLocation   string                  `json:"source_location"`
	SourceRevision   string                  `json:"source_revision"`
	SourceDigest     string                  `json:"source_digest"`
	TrajectoryDigest string                  `json:"trajectory_digest"`
	PatchDigest      string                  `json:"patch_digest,omitempty"`
	SplitGroupID     string                  `json:"split_group_id"`
	NearDuplicateID  string                  `json:"near_duplicate_id"`
	LineageClusterID string                  `json:"lineage_cluster_id"`
	Split            study.DataRole          `json:"split"`
	Outcome          SourceOutcome           `json:"outcome"`
	License          LicenseMetadata         `json:"license"`
	Privacy          PrivacyMetadata         `json:"privacy"`
}

type CorpusCase struct {
	ID                string                   `json:"id"`
	SourceIDs         []string                 `json:"source_ids"`
	Family            Family                   `json:"family"`
	Split             study.DataRole           `json:"split"`
	Control           string                   `json:"control"`
	Manifest          Manifest                 `json:"manifest"`
	BlindPacket       BlindReviewPacket        `json:"blind_review_packet"`
	Reduction         *ReductionWitness        `json:"reduction,omitempty"`
	RegenerationKey   string                   `json:"regeneration_key"`
	ConstructFirewall *ConstructFirewallReport `json:"construct_firewall,omitempty"`
}

type CorpusControl struct {
	ID                  string        `json:"id"`
	Kind                string        `json:"kind"`
	Validator           ValidatorSpec `json:"validator"`
	OriginalArtifact    string        `json:"original_artifact"`
	MutatedArtifact     string        `json:"mutated_artifact"`
	OriginalDigest      string        `json:"original_digest"`
	MutatedDigest       string        `json:"mutated_digest"`
	OutcomeProof        OutcomeProof  `json:"outcome_proof"`
	RegenerationCommand []string      `json:"regeneration_command"`
	Digest              string        `json:"digest"`
}

type CorpusCount struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

type CorpusRelease struct {
	SchemaVersion         string                    `json:"schema_version"`
	CanonicalPolicy       string                    `json:"canonical_policy"`
	CorpusVersion         string                    `json:"corpus_version"`
	Spec                  CorpusSpec                `json:"spec"`
	SpecDigest            string                    `json:"spec_digest"`
	MutationProgramDigest string                    `json:"mutation_program_digest"`
	SplitAlgorithm        string                    `json:"split_algorithm"`
	Sources               []CorpusSource            `json:"sources"`
	Cases                 []CorpusCase              `json:"cases"`
	ConstructRejections   []ConstructFirewallReport `json:"construct_rejections,omitempty"`
	Controls              []CorpusControl           `json:"controls"`
	SourceFamilyCounts    []CorpusCount             `json:"source_family_counts"`
	MutationFamilyCounts  []CorpusCount             `json:"mutation_family_counts"`
	SplitCounts           []CorpusCount             `json:"split_counts"`
	TaskCount             int                       `json:"task_count"`
	PositiveControls      int                       `json:"positive_controls"`
	NegativeControls      int                       `json:"negative_controls"`
	DecoyControls         int                       `json:"decoy_controls"`
	AmbiguousCases        int                       `json:"ambiguous_cases"`
	Digest                string                    `json:"digest"`
}

type SourceCandidate struct {
	Source     CorpusSource
	Trajectory preprocess.Trajectory
}

func mutationProgramDigest(programVersion string) string {
	switch programVersion {
	case MutationProgramVersionV1:
		return digestText(MutationProgramVersionV1 + "\x00" + RelationContractVersionV1 + "\x00" + EvidenceBoundaryVersionV1)
	case MutationProgramVersionV2:
		return digestText(MutationProgramVersionV2 + "\x00" + RelationContractVersionV2 + "\x00" + EvidenceBoundaryVersionV2 + "\x00" + ConstructFirewallSchemaVersion)
	case MutationProgramVersionV3:
		return digestText(MutationProgramVersionV3 + "\x00" + RelationContractVersionV3 + "\x00" + EvidenceBoundaryVersionV3 + "\x00" + ConstructFirewallSchemaVersionV2 + "\x00" + InvocationParserVersion + "\x00" + PresentationClassifierVersion)
	default:
		return ""
	}
}

func mutationProgramVersion(programDigest string) (string, bool) {
	for _, version := range []string{MutationProgramVersionV1, MutationProgramVersionV2} {
		if programDigest == mutationProgramDigest(version) {
			return version, true
		}
	}
	return "", false
}

func DefaultCorpusSpec() (CorpusSpec, error) {
	design, err := stats.ExactBinomialUpperDesign(40, 0.5, 0.8, 0.05/8)
	if err != nil {
		return CorpusSpec{}, err
	}
	if design.CriticalSuccesses != relationCriticalSuccesses ||
		!relationDesignMetricEqual(design.ActualNullTail, relationActualNullTail) ||
		!relationDesignMetricEqual(design.Power, relationPowerAtAlternative) {
		return CorpusSpec{}, errors.New("controlled-relation executable design differs from the frozen evidence")
	}
	designRecord := RelationDesign{
		Method: "exact_binomial_upper_tail", PrimaryUnit: "source_task", NominalAlpha: 0.05, FamilySize: 8,
		AdjustedAlpha: 0.05 / 8, NullAccuracy: 0.5, AlternativeAccuracy: 0.8, TargetPower: 0.8,
		CriticalSuccesses: relationCriticalSuccesses, ActualNullTail: relationActualNullTail, PowerAtAlternative: relationPowerAtAlternative,
		UniqueTaskWithinFamily: true, CrossFamilyClusterUnit: "source_task",
	}
	audit := DevelopmentAudit{
		Method: "provider_free_full_corpus_build_and_relation_validation", AuditedAt: "2026-08-09",
		AuditedReleaseDigest: "8c5489a8e37afbb8d42b9e5b318e5bcf9c66a2cdf252b83adb41bb2a062dc3f4",
		ObservedSources:      200, ObservedSourceTasks: 100, ObservedCases: 320, ObservedPositiveControls: 1,
		AttemptedFamilies: []Family{
			FamilyAmbiguousSemanticEdit, FamilyCandidateOrderReversal, FamilyCausalIndependentReorder,
			FamilyCommandFailureHidden, FamilyTestEvidenceFalsified, FamilyToolOutputIncomplete,
			FamilyIrrelevantVerbosity, FamilyFailingChangeReintroduced, FamilyPatchHunkRemoval,
			FamilyNeutralFormatting, FamilyTestEvidenceOmitted, FamilyStablePathAlias, FamilyUntrustedScoreInjection,
		},
		ExcludedFamilies: []Family{
			FamilyAmbiguousSemanticEdit, FamilyCommandFailureHidden, FamilyFailingChangeReintroduced,
			FamilyPatchHunkRemoval, FamilyStablePathAlias,
		},
		Findings: []string{
			"ambiguous edits remain outside primary gold-label analysis",
			"command failure hiding lacks independently verified exit status in the public upstream traces",
			"factorial eight-factor simulation was rejected because it required 3180 cells for a 320-case corpus",
			"semantic-quality mutations remain excluded without an independently executable upstream task environment",
			"stable path aliasing is implemented but unavailable in benchmark adapters that intentionally omit raw paths",
		},
	}
	designDigest, err := digestJSON(designRecord)
	if err != nil {
		return CorpusSpec{}, err
	}
	auditDigest, err := digestJSON(audit)
	if err != nil {
		return CorpusSpec{}, err
	}
	return CorpusSpec{
		SchemaVersion: CorpusSpecSchemaVersion,
		CorpusVersion: "evalwitness-controlled-corruption.v1",
		Seed:          "evalwitness-controlled-corruption-v1-frozen-seed",
		SourceTasks:   100, TerminalTasks: 60, SWETasks: 40, TrajectoriesPerTask: 2,
		CasesPerFamily: 40,
		PrimaryFamilies: []Family{
			FamilyCandidateOrderReversal,
			FamilyCausalIndependentReorder,
			FamilyTestEvidenceFalsified,
			FamilyToolOutputIncomplete,
			FamilyIrrelevantVerbosity,
			FamilyNeutralFormatting,
			FamilyTestEvidenceOmitted,
			FamilyUntrustedScoreInjection,
		},
		MutationProgramDigest:  mutationProgramDigest(MutationProgramVersionV1),
		DevelopmentAuditDigest: auditDigest,
		DesignEvidenceDigest:   designDigest,
		MutatorsFrozen:         true,
		Design:                 designRecord, DevelopmentAudit: audit,
	}, nil
}

func (spec CorpusSpec) Validate() error {
	if spec.SchemaVersion != CorpusSpecSchemaVersion || missing(spec.CorpusVersion, spec.Seed) ||
		spec.SourceTasks < 40 || spec.TerminalTasks < 1 || spec.SWETasks < 1 || spec.TerminalTasks+spec.SWETasks != spec.SourceTasks ||
		spec.TrajectoriesPerTask != 2 || spec.CasesPerFamily < 1 || len(spec.PrimaryFamilies) < 8 || !spec.MutatorsFrozen ||
		!validDigest(spec.MutationProgramDigest) || !validDigest(spec.DevelopmentAuditDigest) || !validDigest(spec.DesignEvidenceDigest) {
		return errors.New("corruption corpus size, frozen-program, audit, or design specification is invalid")
	}
	if !slices.IsSorted(spec.PrimaryFamilies) {
		return errors.New("corruption corpus mutation families must be sorted")
	}
	if err := spec.Design.Validate(spec.CasesPerFamily, len(spec.PrimaryFamilies)); err != nil {
		return err
	}
	designDigest, err := digestJSON(spec.Design)
	if err != nil || designDigest != spec.DesignEvidenceDigest {
		return errors.New("controlled-relation design digest does not bind the exact design evidence")
	}
	if err := spec.DevelopmentAudit.Validate(spec); err != nil {
		return err
	}
	auditDigest, err := digestJSON(spec.DevelopmentAudit)
	if err != nil || auditDigest != spec.DevelopmentAuditDigest {
		return errors.New("controlled-relation development audit digest does not bind the audit record")
	}
	programVersion, supportedProgram := mutationProgramVersion(spec.MutationProgramDigest)
	if !supportedProgram {
		return errors.New("controlled-relation mutation program digest is stale")
	}
	if programVersion == MutationProgramVersionV2 && spec.DevelopmentAudit.MutationProgramDigest != spec.MutationProgramDigest {
		return errors.New("v2 controlled-relation audit does not bind the mutation program")
	}
	if programVersion == MutationProgramVersionV1 && spec.DevelopmentAudit.MutationProgramDigest != "" {
		return errors.New("v1 controlled-relation audit contains an unsupported program binding")
	}
	seen := make(map[Family]struct{}, len(spec.PrimaryFamilies))
	for _, family := range spec.PrimaryFamilies {
		definition, exists := DefinitionFor(family)
		if !exists || definition.RequiresGoldProof || family == FamilyAmbiguousSemanticEdit {
			return fmt.Errorf("primary corpus family %q is unavailable without independent executable gold proof", family)
		}
		if _, duplicate := seen[family]; duplicate {
			return fmt.Errorf("duplicate primary corpus family %q", family)
		}
		seen[family] = struct{}{}
	}
	return nil
}

func (audit DevelopmentAudit) Validate(spec CorpusSpec) error {
	programVersion, supportedProgram := mutationProgramVersion(spec.MutationProgramDigest)
	if !supportedProgram || audit.ObservedSources < 100 || audit.ObservedSourceTasks < 40 || audit.ObservedCases < len(spec.PrimaryFamilies) ||
		len(audit.AttemptedFamilies) != len(definitions) || len(audit.ExcludedFamilies) == 0 || len(audit.Findings) == 0 {
		return errors.New("controlled-relation development audit is incomplete")
	}
	if programVersion == MutationProgramVersionV1 {
		if audit.Method != "provider_free_full_corpus_build_and_relation_validation" || audit.AuditedAt != "2026-08-09" ||
			!validDigest(audit.AuditedReleaseDigest) || audit.ObservedPositiveControls < 1 || audit.ConstructAuditDigest != "" {
			return errors.New("v1 controlled-relation development audit identity is not frozen")
		}
	}
	if programVersion == MutationProgramVersionV2 {
		if audit.Method != "provider_free_full_corpus_build_construct_firewall_audit.v2" || audit.AuditedReleaseDigest != "" ||
			audit.ObservedSources != spec.SourceTasks*spec.TrajectoriesPerTask || audit.ObservedSourceTasks != spec.SourceTasks ||
			audit.ObservedCases != spec.CasesPerFamily*len(spec.PrimaryFamilies) || audit.ObservedPositiveControls != 0 ||
			audit.MutationProgramDigest != spec.MutationProgramDigest || !validDigest(audit.ConstructAuditDigest) {
			return errors.New("v2 controlled-relation development audit method is invalid")
		}
		if _, err := time.Parse(time.DateOnly, audit.AuditedAt); err != nil {
			return errors.New("v2 controlled-relation development audit date is invalid")
		}
	}
	if !slices.IsSorted(audit.AttemptedFamilies) || !slices.IsSorted(audit.ExcludedFamilies) || !sort.StringsAreSorted(audit.Findings) {
		return errors.New("controlled-relation development audit collections must be sorted")
	}
	for _, family := range spec.PrimaryFamilies {
		if slices.Contains(audit.ExcludedFamilies, family) {
			return fmt.Errorf("primary mutation family %q was excluded by the development audit", family)
		}
	}
	return nil
}

func (design RelationDesign) Validate(casesPerFamily, familyCount int) error {
	if design.Method != "exact_binomial_upper_tail" || design.PrimaryUnit != "source_task" || design.CrossFamilyClusterUnit != "source_task" ||
		design.FamilySize != familyCount || design.CriticalSuccesses < 1 || !design.UniqueTaskWithinFamily ||
		design.NominalAlpha <= 0 || design.NominalAlpha >= 1 || design.AdjustedAlpha != design.NominalAlpha/float64(design.FamilySize) ||
		design.TargetPower <= 0 || design.TargetPower >= 1 {
		return errors.New("controlled-relation design method, cluster unit, multiplicity, or target is invalid")
	}
	exact, err := stats.ExactBinomialUpperDesign(casesPerFamily, design.NullAccuracy, design.AlternativeAccuracy, design.AdjustedAlpha)
	if err != nil {
		return err
	}
	if exact.CriticalSuccesses != design.CriticalSuccesses ||
		!relationDesignMetricEqual(exact.ActualNullTail, design.ActualNullTail) ||
		!relationDesignMetricEqual(exact.Power, design.PowerAtAlternative) || design.PowerAtAlternative < design.TargetPower {
		return errors.New("controlled-relation exact design evidence does not reproduce or reach target power")
	}
	return nil
}

func relationDesignMetricEqual(left, right float64) bool {
	return math.Abs(left-right) <= relationDesignTolerance
}

func SealCorpusRelease(release CorpusRelease) (CorpusRelease, error) {
	release.SchemaVersion = CorpusReleaseSchemaVersion
	release.CanonicalPolicy = CanonicalPolicy
	release.SplitAlgorithm = CorpusSplitAlgorithm
	release.Digest = ""
	digest, err := corpusReleaseDigest(release)
	if err != nil {
		return CorpusRelease{}, err
	}
	release.Digest = digest
	if err := release.Validate(); err != nil {
		return CorpusRelease{}, err
	}
	return release, nil
}

func (release CorpusRelease) Validate() error {
	if release.SchemaVersion != CorpusReleaseSchemaVersion || release.CanonicalPolicy != CanonicalPolicy || release.SplitAlgorithm != CorpusSplitAlgorithm ||
		missing(release.CorpusVersion) || !validDigest(release.SpecDigest) || !validDigest(release.MutationProgramDigest) {
		return errors.New("corruption corpus release identity, source count, task count, or cases are invalid")
	}
	if err := release.Spec.Validate(); err != nil {
		return fmt.Errorf("corruption corpus release specification: %w", err)
	}
	expectedSpecDigest, err := corpusSpecDigest(release.Spec)
	if err != nil {
		return err
	}
	if release.SpecDigest != expectedSpecDigest || release.CorpusVersion != release.Spec.CorpusVersion || release.MutationProgramDigest != release.Spec.MutationProgramDigest ||
		len(release.Sources) != release.Spec.SourceTasks*release.Spec.TrajectoriesPerTask || release.TaskCount != release.Spec.SourceTasks ||
		len(release.Cases) != release.Spec.CasesPerFamily*len(release.Spec.PrimaryFamilies) {
		return errors.New("corruption corpus release does not reproduce its governed specification")
	}
	if err := validateCorpusSources(release.Sources, release.Spec.Seed); err != nil {
		return err
	}
	if err := validateCorpusCases(release.Cases, release.Sources, release.CorpusVersion, release.MutationProgramDigest); err != nil {
		return err
	}
	programVersion, _ := mutationProgramVersion(release.MutationProgramDigest)
	if err := validateConstructRejections(release.ConstructRejections, programVersion, release.Sources); err != nil {
		return err
	}
	if err := validateCorpusControls(release.Controls); err != nil {
		return err
	}
	if err := validateCorpusCounts(release); err != nil {
		return err
	}
	expected, err := corpusReleaseDigest(release)
	if err != nil {
		return err
	}
	if release.Digest != expected {
		return errors.New("corruption corpus release digest is invalid")
	}
	return nil
}

func validateCorpusSources(sources []CorpusSource, seed string) error {
	ids := make(map[string]struct{}, len(sources))
	lineageSplits := make(map[string]study.DataRole)
	for index, source := range sources {
		if missing(source.ID, source.TaskID, source.RepositoryID, source.SourceFamily, source.SourceLocation, source.SourceRevision,
			source.SplitGroupID, source.NearDuplicateID, source.LineageClusterID) ||
			!validSourceFormat(source.SourceFormat) || !validDigest(source.SourceDigest) || !validDigest(source.TrajectoryDigest) ||
			!slices.Contains([]study.DataRole{study.RoleDevelopment, study.RoleCalibration, study.RoleTest}, source.Split) || !source.Privacy.PublicReleaseAllowed {
			return fmt.Errorf("corruption source %d is incomplete or non-releasable", index)
		}
		if source.ID != "source-"+source.TrajectoryDigest {
			return fmt.Errorf("corruption source %d identity does not bind its trajectory", index)
		}
		if index > 0 && sources[index-1].ID >= source.ID {
			return errors.New("corruption sources must be unique and sorted by identity")
		}
		if missing(source.Outcome.Kind, source.Outcome.Value) || !validDigest(source.Outcome.WitnessDigest) {
			return fmt.Errorf("corruption source %q outcome is invalid", source.ID)
		}
		if err := validateLicense(source.License, source.Privacy); err != nil {
			return fmt.Errorf("corruption source %q: %w", source.ID, err)
		}
		if unsafePublicLocation(source.SourceLocation) {
			return fmt.Errorf("corruption source %q location is unsafe for public release", source.ID)
		}
		if _, duplicate := ids[source.ID]; duplicate {
			return fmt.Errorf("duplicate corruption source %q", source.ID)
		}
		ids[source.ID] = struct{}{}
		keys := []string{
			"cluster:" + source.LineageClusterID,
			"group:" + source.SplitGroupID,
			"near_duplicate:" + source.NearDuplicateID,
			"repository:" + source.RepositoryID,
			"task:" + source.RepositoryID + "\x00" + source.TaskID,
			"trajectory:" + source.TrajectoryDigest,
		}
		if source.PatchDigest != "" {
			if !validDigest(source.PatchDigest) {
				return fmt.Errorf("corruption source %q patch digest is invalid", source.ID)
			}
			keys = append(keys, "patch:"+source.PatchDigest)
		}
		for _, key := range keys {
			if assigned, exists := lineageSplits[key]; exists && assigned != source.Split {
				return fmt.Errorf("corruption lineage %q crosses %s and %s splits", key, assigned, source.Split)
			}
			lineageSplits[key] = source.Split
		}
	}
	plan := corpusLineagePlan(sources, seed)
	for _, source := range sources {
		expected := plan[source.ID]
		if source.LineageClusterID != expected.ClusterID || source.Split != expected.Split {
			return fmt.Errorf("corruption source %q does not reproduce its lineage-component split", source.ID)
		}
	}
	return nil
}

func validateCorpusCases(cases []CorpusCase, sources []CorpusSource, corpusVersion, programDigest string) error {
	sourceByID := make(map[string]CorpusSource, len(sources))
	for _, source := range sources {
		sourceByID[source.ID] = source
	}
	caseIDs := make(map[string]struct{}, len(cases))
	for index, item := range cases {
		if missing(item.ID, item.Control, item.RegenerationKey) || !validDigest(item.RegenerationKey) || len(item.SourceIDs) == 0 || item.ID != item.Manifest.MutationID || item.Family != item.Manifest.Program.Family {
			return fmt.Errorf("corruption case %d identity, control, or regeneration key is invalid", index)
		}
		if _, duplicate := caseIDs[item.ID]; duplicate {
			return fmt.Errorf("duplicate corruption case %q", item.ID)
		}
		if index > 0 && cases[index-1].ID >= item.ID {
			return errors.New("corruption cases must be unique and sorted by identity")
		}
		caseIDs[item.ID] = struct{}{}
		if err := item.Manifest.Validate(); err != nil {
			return fmt.Errorf("corruption case %q manifest: %w", item.ID, err)
		}
		if err := item.BlindPacket.Validate(); err != nil {
			return fmt.Errorf("corruption case %q packet: %w", item.ID, err)
		}
		programVersion, supported := mutationProgramVersion(programDigest)
		if !supported || item.Manifest.CorpusVersion != corpusVersion || item.Manifest.Program.Version != programVersion || item.Manifest.Validator.ContractDigest != programDigest {
			return fmt.Errorf("corruption case %q does not use the frozen mutation program", item.ID)
		}
		if programVersion == MutationProgramVersionV2 {
			if item.ConstructFirewall == nil || item.Manifest.ConstructFirewallDigest != item.ConstructFirewall.Digest || item.ConstructFirewall.Status != ConstructApplied {
				return fmt.Errorf("corruption case %q does not bind an applied construct firewall", item.ID)
			}
			if err := item.ConstructFirewall.Validate(); err != nil {
				return fmt.Errorf("corruption case %q construct firewall: %w", item.ID, err)
			}
		} else if item.ConstructFirewall != nil {
			return fmt.Errorf("v1 corruption case %q cannot contain a construct firewall", item.ID)
		}
		if item.Control != corpusControl(item.Family) || item.BlindPacket.Digest != item.Manifest.Review.BlindPacketDigest ||
			item.BlindPacket.OriginalDigest != item.Manifest.OriginalTrajectoryDigest || item.BlindPacket.MutatedDigest != item.Manifest.MutatedTrajectoryDigest {
			return fmt.Errorf("corruption case %q control or blind packet does not bind its manifest", item.ID)
		}
		linkedSources := make([]CorpusSource, 0, len(item.SourceIDs))
		for _, sourceID := range item.SourceIDs {
			source, exists := sourceByID[sourceID]
			if !exists || source.Split != item.Split || source.SplitGroupID != item.Manifest.SplitGroupID {
				return fmt.Errorf("corruption case %q crosses source lineage or split", item.ID)
			}
			linkedSources = append(linkedSources, source)
		}
		definition, _ := DefinitionFor(item.Family)
		if err := validateCaseSourceBinding(item, linkedSources, definition, programDigest); err != nil {
			return err
		}
		if item.Manifest.Witness.LabelState == LabelAmbiguous && item.Control != "ambiguous" {
			return fmt.Errorf("corruption case %q hides an ambiguous label", item.ID)
		}
		if definition.PairLevel && item.Reduction != nil || !definition.PairLevel && item.Reduction == nil {
			return fmt.Errorf("corruption case %q reduction presence does not match its mutation level", item.ID)
		}
		if item.Reduction != nil {
			if err := item.Reduction.Validate(); err != nil || item.Reduction.MutationDigest != item.Manifest.Digest {
				return fmt.Errorf("corruption case %q reduction is invalid", item.ID)
			}
		}
	}
	return nil
}

func validateConstructRejections(rejections []ConstructFirewallReport, programVersion string, sources []CorpusSource) error {
	if programVersion == MutationProgramVersionV1 {
		if len(rejections) != 0 {
			return errors.New("v1 corruption corpus cannot contain construct-firewall rejections")
		}
		return nil
	}
	sourceDigests := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		sourceDigests[source.TrajectoryDigest] = struct{}{}
	}
	for index, report := range rejections {
		if report.Status != ConstructRejected {
			return fmt.Errorf("construct rejection %d is not rejected", index)
		}
		if index > 0 && rejections[index-1].Digest >= report.Digest {
			return errors.New("construct rejections must be unique and sorted by digest")
		}
		if err := report.Validate(); err != nil {
			return fmt.Errorf("construct rejection %d: %w", index, err)
		}
		if _, exists := sourceDigests[report.SourceTrajectoryDigest]; !exists {
			return fmt.Errorf("construct rejection %d does not bind a governed source trajectory", index)
		}
	}
	return nil
}

func validateCaseSourceBinding(item CorpusCase, sources []CorpusSource, definition Definition, programDigest string) error {
	if definition.PairLevel {
		if len(sources) != 2 || sources[0].ID == sources[1].ID || sources[0].TaskID != sources[1].TaskID || sources[0].RepositoryID != sources[1].RepositoryID ||
			sources[0].SplitGroupID != sources[1].SplitGroupID || sources[0].License != sources[1].License || sources[0].Privacy != sources[1].Privacy {
			return fmt.Errorf("corruption case %q pair source lineage is invalid", item.ID)
		}
		trajectoryDigests := []string{sources[0].TrajectoryDigest, sources[1].TrajectoryDigest}
		originalDigest, err := digestJSON(trajectoryDigests)
		if err != nil {
			return err
		}
		mutatedDigest, err := digestJSON([]string{trajectoryDigests[1], trajectoryDigests[0]})
		if err != nil {
			return err
		}
		combinedSourceDigest, err := digestJSON([]string{sources[0].SourceDigest, sources[1].SourceDigest})
		if err != nil {
			return err
		}
		expectedOutcome := SourceOutcome{
			Kind: "paired_benchmark_rewards", Value: sources[0].Outcome.Value + "," + sources[1].Outcome.Value,
			WitnessDigest: digestText(sources[0].Outcome.WitnessDigest + "\x00" + sources[1].Outcome.WitnessDigest),
		}
		expectedLocation := "pair/" + sources[0].SplitGroupID + "/" + digestText(sources[0].ID+"\x00"+sources[1].ID)
		source := item.Manifest.Source
		if source.TaskID != sources[0].TaskID || source.RepositoryID != sources[0].RepositoryID ||
			source.SourceFamily != "paired/"+sources[0].SourceFamily+"+"+sources[1].SourceFamily || source.SourceFormat != sources[0].SourceFormat ||
			source.SourceLocation != expectedLocation || source.SourceRevision != sources[0].SourceRevision+"+"+sources[1].SourceRevision ||
			source.SourceDigest != combinedSourceDigest || source.TrajectoryDigest != originalDigest || !slices.Equal(source.PairedTrajectoryDigests, trajectoryDigests) ||
			source.Outcome != expectedOutcome || item.Manifest.MutatedTrajectoryDigest != mutatedDigest || item.Manifest.License != sources[0].License || item.Manifest.Privacy != sources[0].Privacy {
			return fmt.Errorf("corruption case %q pair manifest does not reproduce its sources", item.ID)
		}
		expectedKey := digestText(programDigest + "\x00" + sources[0].ID + "\x00" + sources[1].ID + "\x00" + string(item.Family))
		if item.RegenerationKey != expectedKey {
			return fmt.Errorf("corruption case %q pair regeneration key is invalid", item.ID)
		}
		return nil
	}
	if len(sources) != 1 {
		return fmt.Errorf("corruption case %q requires exactly one trajectory source", item.ID)
	}
	source := sources[0]
	manifestSource := item.Manifest.Source
	if manifestSource.TaskID != source.TaskID || manifestSource.RepositoryID != source.RepositoryID || manifestSource.SourceFamily != source.SourceFamily ||
		manifestSource.SourceFormat != source.SourceFormat || manifestSource.SourceLocation != source.SourceLocation || manifestSource.SourceRevision != source.SourceRevision ||
		manifestSource.SourceDigest != source.SourceDigest || manifestSource.TrajectoryDigest != source.TrajectoryDigest || manifestSource.Outcome != source.Outcome ||
		item.Manifest.License != source.License || item.Manifest.Privacy != source.Privacy {
		return fmt.Errorf("corruption case %q manifest does not reproduce its source", item.ID)
	}
	expectedKey := digestText(programDigest + "\x00" + source.ID + "\x00" + string(item.Family))
	if item.RegenerationKey != expectedKey {
		return fmt.Errorf("corruption case %q regeneration key is invalid", item.ID)
	}
	return nil
}

func validateCorpusControls(controls []CorpusControl) error {
	if len(controls) == 0 {
		return errors.New("corruption corpus requires at least one independent positive control")
	}
	ids := make(map[string]struct{}, len(controls))
	for index, control := range controls {
		if missing(control.ID, control.Kind, control.OriginalArtifact, control.MutatedArtifact) || control.Kind != "positive" ||
			!validDigest(control.OriginalDigest) || !validDigest(control.MutatedDigest) || len(control.RegenerationCommand) == 0 {
			return fmt.Errorf("corruption control %d is incomplete", index)
		}
		if _, duplicate := ids[control.ID]; duplicate {
			return fmt.Errorf("duplicate corruption control %q", control.ID)
		}
		ids[control.ID] = struct{}{}
		if err := validateValidator(control.Validator, Definition{RequiresGoldProof: true}); err != nil {
			return err
		}
		if err := validateOutcomeProof(&control.OutcomeProof, control.Validator, Definition{RequiresGoldProof: true}, RelationOriginalBetter); err != nil {
			return err
		}
		expected, err := corpusControlDigest(control)
		if err != nil || control.Digest != expected || control.ID != "control-"+expected {
			return fmt.Errorf("corruption control %d identity is invalid", index)
		}
	}
	return nil
}

func validateCorpusCounts(release CorpusRelease) error {
	actualSources := make(map[string]int)
	actualFamilies := make(map[string]int)
	actualSplits := make(map[string]int)
	tasks := make(map[string]struct{})
	tasksBySplit := make(map[string]map[string]struct{})
	controls := map[string]int{}
	for _, source := range release.Sources {
		actualSources[source.SourceFamily]++
		actualSplits[string(source.Split)]++
		tasks[source.SplitGroupID] = struct{}{}
		if tasksBySplit[string(source.Split)] == nil {
			tasksBySplit[string(source.Split)] = make(map[string]struct{})
		}
		tasksBySplit[string(source.Split)][source.SplitGroupID] = struct{}{}
	}
	for _, item := range release.Cases {
		actualFamilies[string(item.Family)]++
		controls[item.Control]++
	}
	for _, control := range release.Controls {
		controls[control.Kind]++
	}
	if !equalCounts(release.SourceFamilyCounts, actualSources) || !equalCounts(release.MutationFamilyCounts, actualFamilies) || !equalCounts(release.SplitCounts, actualSplits) || release.TaskCount != len(tasks) ||
		release.PositiveControls != controls["positive"] || release.NegativeControls != controls["negative"] || release.DecoyControls != controls["decoy"] || release.AmbiguousCases != controls["ambiguous"] {
		return errors.New("corruption corpus declared counts do not match release contents")
	}
	if len(actualSources) < 3 || len(actualFamilies) < 8 || release.PositiveControls == 0 || release.NegativeControls == 0 || release.DecoyControls == 0 {
		return errors.New("corruption corpus lacks required source families, mutation families, or controls")
	}
	expectedTaskSplits := map[string]int{
		string(study.RoleDevelopment): release.TaskCount * 60 / 100,
		string(study.RoleCalibration): release.TaskCount * 20 / 100,
	}
	expectedTaskSplits[string(study.RoleTest)] = release.TaskCount - expectedTaskSplits[string(study.RoleDevelopment)] - expectedTaskSplits[string(study.RoleCalibration)]
	for role, expected := range expectedTaskSplits {
		if len(tasksBySplit[role]) != expected {
			return fmt.Errorf("corruption corpus %s split has %d tasks, expected %d", role, len(tasksBySplit[role]), expected)
		}
	}
	minimum, maximum := -1, -1
	for _, count := range actualFamilies {
		if minimum < 0 || count < minimum {
			minimum = count
		}
		if count > maximum {
			maximum = count
		}
	}
	if maximum-minimum > 1 {
		return errors.New("corruption corpus mutation prevalence is not balanced")
	}
	for _, family := range release.Spec.PrimaryFamilies {
		if actualFamilies[string(family)] != release.Spec.CasesPerFamily {
			return fmt.Errorf("corruption corpus family %q does not match its governed case count", family)
		}
	}
	terminalSources, sweSources := 0, 0
	for family, count := range actualSources {
		switch {
		case strings.HasPrefix(family, "terminal-bench-2/"):
			terminalSources += count
		case strings.HasPrefix(family, "swe-bench/"):
			sweSources += count
		}
	}
	if terminalSources != release.Spec.TerminalTasks*release.Spec.TrajectoriesPerTask || sweSources != release.Spec.SWETasks*release.Spec.TrajectoriesPerTask {
		return errors.New("corruption corpus source composition does not match its governed specification")
	}
	return nil
}

func countRecords(values map[string]int) []CorpusCount {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]CorpusCount, 0, len(keys))
	for _, key := range keys {
		result = append(result, CorpusCount{ID: key, Count: values[key]})
	}
	return result
}

func equalCounts(records []CorpusCount, values map[string]int) bool {
	return slices.Equal(records, countRecords(values))
}

func corpusControl(family Family) string {
	switch family {
	case FamilyNeutralFormatting, FamilyCandidateOrderReversal, FamilyCausalIndependentReorder:
		return "negative"
	case FamilyUntrustedScoreInjection:
		return "decoy"
	case FamilyPatchHunkRemoval, FamilyFailingChangeReintroduced:
		return "positive"
	case FamilyAmbiguousSemanticEdit:
		return "ambiguous"
	default:
		return "relation"
	}
}

func corpusReleaseDigest(release CorpusRelease) (string, error) {
	release.Digest = ""
	return digestJSON(release)
}

func corpusControlDigest(control CorpusControl) (string, error) {
	control.ID = ""
	control.Digest = ""
	return digestJSON(control)
}

func corpusSpecDigest(spec CorpusSpec) (string, error) {
	return digestJSON(spec)
}
