package stress

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
)

type RelationRegistry struct {
	SchemaVersion              string            `json:"schema_version"`
	CanonicalPolicy            string            `json:"canonical_policy"`
	CatalogVersion             string            `json:"catalog_version"`
	CorpusVersion              string            `json:"corpus_version"`
	ReplayCorpusIdentityDigest string            `json:"replay_corpus_identity_digest"`
	PlanDigest                 string            `json:"plan_digest"`
	PlanDesignEvidenceDigest   string            `json:"plan_design_evidence_digest"`
	AuditDigest                string            `json:"audit_digest"`
	ReleaseDigest              string            `json:"release_digest"`
	MutationProgramDigest      string            `json:"mutation_program_digest"`
	ReleasePolicyVersion       string            `json:"release_policy_version"`
	CoreFamilies               []mutation.Family `json:"core_families"`
	CoreCasesPerFamily         int               `json:"core_cases_per_family"`
	CoreCases                  int               `json:"core_cases"`
	SentinelFamily             mutation.Family   `json:"sentinel_family"`
	SentinelCases              int               `json:"sentinel_cases"`
	PrimaryRelations           int               `json:"primary_relations"`
	SensitivityRelations       int               `json:"sensitivity_relations"`
	SentinelRelations          int               `json:"sentinel_relations"`
	Relations                  []Relation        `json:"relations"`
	Digest                     string            `json:"digest"`
}

func SealRelationRegistry(plan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3, relations []Relation) (RelationRegistry, error) {
	if err := release.Validate(plan, audit); err != nil {
		return RelationRegistry{}, fmt.Errorf("validate v3 corpus before relation registration: %w", err)
	}
	canonicalRelations := make([]Relation, len(relations))
	for index, relation := range relations {
		sealed, err := SealRelation(relation)
		if err != nil {
			return RelationRegistry{}, fmt.Errorf("seal registered relation %d: %w", index, err)
		}
		canonicalRelations[index] = sealed
	}
	sort.Slice(canonicalRelations, func(left, right int) bool { return canonicalRelations[left].ID < canonicalRelations[right].ID })
	replayCorpusIdentityDigest, err := releaseReplayCorpusIdentityDigest(release)
	if err != nil {
		return RelationRegistry{}, err
	}
	value := RelationRegistry{
		SchemaVersion: RelationRegistrySchemaVersion, CanonicalPolicy: CanonicalPolicy, CatalogVersion: RelationCatalogVersionV3,
		CorpusVersion: release.CorpusVersion, ReplayCorpusIdentityDigest: replayCorpusIdentityDigest,
		PlanDigest: plan.Digest, PlanDesignEvidenceDigest: plan.DesignEvidenceDigest,
		AuditDigest: audit.Digest, ReleaseDigest: release.Digest, MutationProgramDigest: release.MutationProgramDigest,
		ReleasePolicyVersion: release.Policy.Version, CoreFamilies: slices.Clone(release.Policy.InferentialCoreFamilies),
		CoreCasesPerFamily: release.Policy.CoreCasesPerFamily, CoreCases: release.Policy.CoreCases,
		SentinelFamily: release.Policy.ScarcitySentinelFamily, SentinelCases: release.Policy.ScarcitySentinelCases,
		Relations: canonicalRelations,
	}
	value.PrimaryRelations, value.SensitivityRelations, value.SentinelRelations = relationCatalogCounts(canonicalRelations)
	digest, err := relationRegistryDigest(value)
	if err != nil {
		return RelationRegistry{}, err
	}
	value.Digest = digest
	if err := value.ValidateAgainst(plan, audit, release); err != nil {
		return RelationRegistry{}, err
	}
	return value, nil
}

func (value RelationRegistry) Validate() error {
	if value.SchemaVersion != RelationRegistrySchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.CatalogVersion != RelationCatalogVersionV3 || value.CorpusVersion == "" || !validDigest(value.ReplayCorpusIdentityDigest) ||
		!validDigest(value.PlanDigest) || !validDigest(value.PlanDesignEvidenceDigest) ||
		!validDigest(value.AuditDigest) || !validDigest(value.ReleaseDigest) || !validDigest(value.MutationProgramDigest) ||
		value.ReleasePolicyVersion != mutation.CorpusReleasePolicyVersionV3 || len(value.CoreFamilies) != 7 ||
		value.CoreCasesPerFamily != 40 || value.CoreCases != 280 || value.SentinelFamily != mutation.FamilyTestEvidenceOmitted ||
		value.SentinelCases != 3 || value.PrimaryRelations != 7 || value.SensitivityRelations != 7 || value.SentinelRelations != 1 || len(value.Relations) != 15 {
		return errors.New("stress relation registry identity, v3 corpus policy, or coverage is invalid")
	}
	for index, family := range value.CoreFamilies {
		if family == value.SentinelFamily || index > 0 && value.CoreFamilies[index-1] >= family {
			return errors.New("stress relation registry core families must be unique, sorted, and exclude the sentinel")
		}
	}
	covered := make(map[mutation.Family]bool, len(value.CoreFamilies)+1)
	roles := make(map[mutation.Family]map[Estimand]int, len(value.CoreFamilies)+1)
	allowed := make(map[mutation.Family]bool, len(value.CoreFamilies)+1)
	for _, family := range value.CoreFamilies {
		allowed[family] = true
	}
	allowed[value.SentinelFamily] = true
	for index, relation := range value.Relations {
		if err := relation.Validate(); err != nil {
			return fmt.Errorf("validate registered relation %d: %w", index, err)
		}
		if index > 0 && value.Relations[index-1].ID >= relation.ID {
			return errors.New("stress relation registry relations must be unique and identity-sorted")
		}
		family := relation.Transform.MutationFamily
		if relation.Transform.Kind != TransformMutation || !allowed[family] {
			return fmt.Errorf("stress relation %q does not belong to the registered v3 release families", relation.ID)
		}
		if err := validateRegistryRelation(relation, value.SentinelFamily); err != nil {
			return fmt.Errorf("stress relation %q: %w", relation.ID, err)
		}
		if err := validateV3CatalogRelation(relation); err != nil {
			return fmt.Errorf("stress relation %q differs from the canonical v3 catalog: %w", relation.ID, err)
		}
		covered[family] = true
		if roles[family] == nil {
			roles[family] = make(map[Estimand]int)
		}
		roles[family][relation.StatisticalFamily.Estimand]++
	}
	for _, family := range value.CoreFamilies {
		if !covered[family] {
			return fmt.Errorf("stress relation registry omits release family %q", family)
		}
		if roles[family][EstimandPrimaryCore] != 1 || roles[family][EstimandSensitivity] != 1 || len(roles[family]) != 2 {
			return fmt.Errorf("stress relation registry family %q must have one primary and one sensitivity relation", family)
		}
	}
	if !covered[value.SentinelFamily] || roles[value.SentinelFamily][EstimandScarcitySentinel] != 1 || len(roles[value.SentinelFamily]) != 1 {
		return errors.New("stress relation registry must have one isolated scarcity-sentinel relation")
	}
	expected, err := relationRegistryDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress relation registry digest is invalid")
	}
	return nil
}

func (value RelationRegistry) ValidateAgainst(plan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := release.Validate(plan, audit); err != nil {
		return err
	}
	policy := release.Policy
	replayCorpusIdentityDigest, err := releaseReplayCorpusIdentityDigest(release)
	if err != nil {
		return err
	}
	if value.CorpusVersion != release.CorpusVersion || value.ReplayCorpusIdentityDigest != replayCorpusIdentityDigest ||
		value.PlanDigest != plan.Digest || value.PlanDesignEvidenceDigest != plan.DesignEvidenceDigest ||
		value.AuditDigest != audit.Digest || value.ReleaseDigest != release.Digest || value.MutationProgramDigest != release.MutationProgramDigest ||
		value.ReleasePolicyVersion != policy.Version || !slices.Equal(value.CoreFamilies, policy.InferentialCoreFamilies) ||
		value.CoreCasesPerFamily != policy.CoreCasesPerFamily || value.CoreCases != policy.CoreCases ||
		value.SentinelFamily != policy.ScarcitySentinelFamily || value.SentinelCases != policy.ScarcitySentinelCases ||
		policy.SentinelInPrimaryEstimand || !policy.ScarcitySentinelExhaustive || policy.HeldOutSentinelClaimAvailable || policy.BalancedEightFamilyAvailable {
		return errors.New("stress relation registry differs from the exact v3 plan, audit, release, or scarcity boundary")
	}
	requiredFormats := releaseFormatsByFamily(release)
	for _, relation := range value.Relations {
		if relation.StatisticalFamily.ClusterUnit != plan.Design.CrossFamilyClusterUnit {
			return fmt.Errorf("stress relation %q cluster unit differs from the v3 corpus design", relation.ID)
		}
		if !slices.Equal(relation.Applicability.RequiredSourceFormats, requiredFormats[relation.Transform.MutationFamily]) {
			return fmt.Errorf("stress relation %q source formats differ from its exact release coverage", relation.ID)
		}
	}
	return nil
}

func relationCatalogCounts(relations []Relation) (primary, sensitivity, sentinel int) {
	for _, relation := range relations {
		switch relation.StatisticalFamily.Estimand {
		case EstimandPrimaryCore:
			primary++
		case EstimandSensitivity:
			sensitivity++
		case EstimandScarcitySentinel:
			sentinel++
		}
	}
	return primary, sensitivity, sentinel
}

func validateRegistryRelation(relation Relation, sentinel mutation.Family) error {
	requirements := []SourceRequirement{
		{Kind: RequirementV3Manifest, Value: mutation.ManifestSchemaVersion},
		{Kind: RequirementV3ConstructFirewall, Value: mutation.ConstructFirewallSchemaVersionV2},
		{Kind: RequirementFormalWitness, Value: mutation.WitnessSchemaVersion},
		{Kind: RequirementExactReplay, Value: "required"},
		{Kind: RequirementOwnerAttestation, Value: relationevidence.OwnerInspectionPublicAttestationSchemaVersion},
	}
	for _, requirement := range requirements {
		if !hasExactSourceRequirement(relation.Applicability.Requirements, requirement.Kind, requirement.Value) {
			return fmt.Errorf("required source contract %q does not bind %q", requirement.Kind, requirement.Value)
		}
	}
	if relation.Transform.MutationFamily == sentinel {
		if relation.StatisticalFamily.Estimand != EstimandScarcitySentinel {
			return errors.New("scarcity sentinel relation entered a non-sentinel estimand")
		}
		return nil
	}
	if relation.StatisticalFamily.Estimand == EstimandScarcitySentinel {
		return errors.New("inferential-core relation entered the scarcity-sentinel estimand")
	}
	if relation.StatisticalFamily.Estimand == EstimandPrimaryCore &&
		!hasExactSourceRequirement(relation.Applicability.Requirements, RequirementTerminalLedger, relationevidence.TerminalRelationLedgerSchemaVersionV3) {
		return errors.New("primary-core relation lacks the v3 terminal-ledger requirement")
	}
	return nil
}

func hasExactSourceRequirement(values []SourceRequirement, kind SourceRequirementKind, expected string) bool {
	return slices.ContainsFunc(values, func(value SourceRequirement) bool { return value.Kind == kind && value.Value == expected })
}

func releaseFormatsByFamily(release mutation.CorpusReleaseV3) map[mutation.Family][]preprocess.SourceFormat {
	sources := make(map[string]preprocess.SourceFormat, len(release.Sources))
	for _, source := range release.Sources {
		sources[source.ID] = source.SourceFormat
	}
	sets := make(map[mutation.Family]map[preprocess.SourceFormat]bool)
	for _, item := range release.Cases {
		if sets[item.Family] == nil {
			sets[item.Family] = make(map[preprocess.SourceFormat]bool)
		}
		for _, sourceID := range item.SourceIDs {
			sets[item.Family][sources[sourceID]] = true
		}
	}
	result := make(map[mutation.Family][]preprocess.SourceFormat, len(sets))
	for family, formats := range sets {
		for format := range formats {
			result[family] = append(result[family], format)
		}
		slices.Sort(result[family])
	}
	return result
}

type replayCorpusIdentity struct {
	CaseID                       string          `json:"case_id"`
	TaskGroupID                  string          `json:"task_group_id"`
	Family                       mutation.Family `json:"family"`
	Split                        string          `json:"split"`
	ManifestDigest               string          `json:"manifest_digest"`
	OriginalTrajectoryDigests    []string        `json:"original_trajectory_digests"`
	TransformedTrajectoryDigests []string        `json:"transformed_trajectory_digests"`
}

func releaseReplayCorpusIdentityDigest(release mutation.CorpusReleaseV3) (string, error) {
	sources := make(map[string]string, len(release.Sources))
	for _, source := range release.Sources {
		sources[source.ID] = source.TrajectoryDigest
	}
	identities := make([]replayCorpusIdentity, 0, len(release.Cases))
	for _, item := range release.Cases {
		original := make([]string, len(item.SourceIDs))
		for index, sourceID := range item.SourceIDs {
			digest, exists := sources[sourceID]
			if !exists || !validDigest(digest) {
				return "", fmt.Errorf("stress release case %q has an invalid replay source %q", item.ID, sourceID)
			}
			original[index] = digest
		}
		transformed := []string{item.Manifest.MutatedTrajectoryDigest}
		definition, exists := mutation.DefinitionFor(item.Family)
		if !exists {
			return "", fmt.Errorf("stress release case %q has an unknown family", item.ID)
		}
		if definition.PairLevel {
			transformed = slices.Clone(original)
			slices.Reverse(transformed)
		}
		identities = append(identities, replayCorpusIdentity{
			CaseID: item.ID, TaskGroupID: item.Manifest.SplitGroupID, Family: item.Family, Split: string(item.Split),
			ManifestDigest: item.Manifest.Digest, OriginalTrajectoryDigests: original, TransformedTrajectoryDigests: transformed,
		})
	}
	return replayCorpusIdentityDigest(identities)
}

func replayedCorpusIdentityDigest(replayed []ReplayedRelationCaseV3) (string, error) {
	identities := make([]replayCorpusIdentity, 0, len(replayed))
	for _, item := range replayed {
		original := make([]string, len(item.Original))
		for index, trajectory := range item.Original {
			original[index] = trajectory.Digest
		}
		transformed := make([]string, len(item.Transformed))
		for index, trajectory := range item.Transformed {
			transformed[index] = trajectory.Digest
		}
		identities = append(identities, replayCorpusIdentity{
			CaseID: item.CaseID, TaskGroupID: item.TaskGroupID, Family: item.Family, Split: string(item.Split),
			ManifestDigest: item.ManifestDigest, OriginalTrajectoryDigests: original, TransformedTrajectoryDigests: transformed,
		})
	}
	return replayCorpusIdentityDigest(identities)
}

func replayCorpusIdentityDigest(identities []replayCorpusIdentity) (string, error) {
	canonical := append([]replayCorpusIdentity(nil), identities...)
	sort.Slice(canonical, func(left, right int) bool { return canonical[left].CaseID < canonical[right].CaseID })
	for index, identity := range canonical {
		if !identifierPattern.MatchString(identity.CaseID) || !identifierPattern.MatchString(identity.TaskGroupID) ||
			!validDigest(identity.ManifestDigest) || len(identity.OriginalTrajectoryDigests) == 0 ||
			len(identity.OriginalTrajectoryDigests) != len(identity.TransformedTrajectoryDigests) ||
			index > 0 && canonical[index-1].CaseID >= identity.CaseID {
			return "", errors.New("stress replay corpus identity projection is invalid, duplicated, or incomplete")
		}
		for _, digest := range append(slices.Clone(identity.OriginalTrajectoryDigests), identity.TransformedTrajectoryDigests...) {
			if !validDigest(digest) {
				return "", errors.New("stress replay corpus identity projection has an invalid trajectory digest")
			}
		}
	}
	return digestDocument(canonical)
}

func relationRegistryDigest(value RelationRegistry) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
