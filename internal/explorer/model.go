package explorer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/internal/reliance"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	ReportSchemaVersion = "evalwitness.evidence-explorer-report.v2"
	CanonicalPolicy     = "evalwitness.evidence-explorer-canonical-json.v1"
)

type Availability string

const (
	AvailabilityAvailable             Availability = "available"
	AvailabilityNotMeasured           Availability = "not_measured"
	AvailabilityNotApplicable         Availability = "not_applicable"
	AvailabilityUnsupported           Availability = "unsupported"
	AvailabilityNotAuthorized         Availability = "not_authorized"
	AvailabilityNotRun                Availability = "not_run"
	AvailabilityWithheldPrivateParent Availability = "withheld_private_parent"
)

func (availability Availability) Valid() bool {
	switch availability {
	case AvailabilityAvailable, AvailabilityNotMeasured, AvailabilityNotApplicable,
		AvailabilityUnsupported, AvailabilityNotAuthorized, AvailabilityNotRun,
		AvailabilityWithheldPrivateParent:
		return true
	default:
		return false
	}
}

type ArtifactKind string

const (
	ArtifactCapsuleComponent ArtifactKind = "capsule_component"
	ArtifactReleaseFile      ArtifactKind = "release_file"
	ArtifactSidecar          ArtifactKind = "sidecar"
)

func (kind ArtifactKind) Valid() bool {
	return kind == ArtifactCapsuleComponent || kind == ArtifactReleaseFile || kind == ArtifactSidecar
}

type ArtifactRef struct {
	Kind           ArtifactKind `json:"kind"`
	ID             string       `json:"id"`
	SchemaVersion  string       `json:"schema_version"`
	PayloadSHA256  string       `json:"payload_sha256"`
	ArtifactDigest string       `json:"artifact_digest"`
}

type CapsuleBinding struct {
	CapsuleID      string      `json:"capsule_id"`
	ManifestDigest string      `json:"manifest_digest"`
	LedgerDigest   string      `json:"ledger_digest"`
	AutopsyDigest  string      `json:"autopsy_digest"`
	LedgerSource   ArtifactRef `json:"ledger_source"`
	AutopsySource  ArtifactRef `json:"autopsy_source"`
}

type CountView struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type ScopeView struct {
	Evaluator                  string       `json:"evaluator"`
	Route                      *string      `json:"route"`
	RouteAvailability          Availability `json:"route_availability"`
	EvidenceRole               string       `json:"evidence_role"`
	ReleaseVersion             string       `json:"release_version"`
	DevelopmentFixtures        int          `json:"development_fixtures"`
	EmpiricalTaskGroups        int          `json:"empirical_task_groups"`
	ResearchAdmittedSources    int          `json:"research_admitted_sources"`
	ProviderCalls              int          `json:"provider_calls"`
	Empirical                  bool         `json:"empirical"`
	ProviderRanking            bool         `json:"provider_ranking"`
	RestrictedMaterialExcluded bool         `json:"restricted_material_excluded"`
}

type ClaimView struct {
	ClaimID                    string      `json:"claim_id"`
	FailableProperty           string      `json:"failable_property"`
	ObservableFailureCondition string      `json:"observable_failure_condition"`
	Accepted                   bool        `json:"accepted"`
	ClaimBoundary              []string    `json:"claim_boundary"`
	KnownLimitations           []string    `json:"known_limitations"`
	OutOfScopeUses             []string    `json:"out_of_scope_uses"`
	Source                     ArtifactRef `json:"source"`
}

type FreshnessView struct {
	State               string  `json:"state"`
	ObservedStateDigest string  `json:"observed_state_digest"`
	ValidFrom           string  `json:"valid_from"`
	ValidUntil          *string `json:"valid_until"`
	EvaluatedAt         string  `json:"evaluated_at"`
	InvalidationEdges   int     `json:"invalidation_edges"`
}

type BOMView struct {
	BOMID                   string        `json:"bom_id"`
	CandidateID             string        `json:"candidate_id"`
	Executable              string        `json:"executable"`
	Operands                []string      `json:"operands"`
	RequiredFields          []string      `json:"required_fields"`
	DecisiveChannels        []string      `json:"decisive_channels"`
	SurvivingChannels       []string      `json:"surviving_channels"`
	TruncatedRequiredFields []string      `json:"truncated_required_fields"`
	RepositoryStateDigest   string        `json:"repository_state_digest"`
	Freshness               FreshnessView `json:"freshness"`
	Source                  ArtifactRef   `json:"source"`
}

type MethodGenerationView struct {
	Generation         string        `json:"generation"`
	State              string        `json:"state"`
	Evidence           []ArtifactRef `json:"evidence"`
	FrozenDenominators []CountView   `json:"frozen_denominators"`
	Observations       []string      `json:"observations"`
	NonClaims          []string      `json:"non_claims"`
	GuardingTests      []string      `json:"guarding_tests"`
	DeepLink           string        `json:"deep_link"`
}

type MethodTransitionView struct {
	From           string        `json:"from"`
	To             string        `json:"to"`
	Reason         string        `json:"reason"`
	Evidence       []ArtifactRef `json:"evidence"`
	GuardingTest   string        `json:"guarding_test"`
	FromDeepLink   string        `json:"from_deep_link"`
	TargetDeepLink string        `json:"target_deep_link"`
}

type MethodView struct {
	Current     string                 `json:"current"`
	Boundary    string                 `json:"boundary"`
	Generations []MethodGenerationView `json:"generations"`
	Transitions []MethodTransitionView `json:"transitions"`
}

type TransportLayerView struct {
	Layer                    string       `json:"layer"`
	Status                   string       `json:"status"`
	ObjectDigest             *string      `json:"object_digest"`
	ObjectDigestAvailability Availability `json:"object_digest_availability"`
	RequiredFields           []string     `json:"required_fields"`
	DecisiveChannels         []string     `json:"decisive_channels"`
	DetailAvailability       Availability `json:"detail_availability"`
	DeepLink                 string       `json:"deep_link"`
}

type TransportPathView struct {
	PathID        string               `json:"path_id"`
	Label         string               `json:"label"`
	TerminalState string               `json:"terminal_state"`
	Evidence      ArtifactRef          `json:"evidence"`
	Layers        []TransportLayerView `json:"layers"`
	DeepLink      string               `json:"deep_link"`
}

type TransportView struct {
	ClaimID       string              `json:"claim_id"`
	Accepted      bool                `json:"accepted"`
	SelectedPath  string              `json:"selected_path"`
	SelectedLayer string              `json:"selected_layer"`
	Paths         []TransportPathView `json:"paths"`
}

type LossCertificateView struct {
	CertificateID         string      `json:"certificate_id"`
	PathID                string      `json:"path_id"`
	FirstLossState        string      `json:"first_loss_state"`
	Disposition           string      `json:"disposition"`
	Precedence            int         `json:"precedence"`
	FailabilityResolution string      `json:"failability_resolution"`
	FailabilityReason     string      `json:"failability_reason"`
	PublicSafe            bool        `json:"public_safe"`
	RestrictedContent     bool        `json:"restricted_content"`
	SupportedClaim        string      `json:"supported_claim"`
	UnsupportedClaims     []string    `json:"unsupported_claims"`
	Source                ArtifactRef `json:"source"`
}

type StressReductionStepView struct {
	Index               int    `json:"index"`
	UnitID              string `json:"unit_id"`
	Decision            string `json:"decision"`
	RelationRevalidated bool   `json:"relation_revalidated"`
	PrivacyRevalidated  bool   `json:"privacy_revalidated"`
	ViolationPreserved  bool   `json:"violation_preserved"`
	BeforeDigest        string `json:"before_digest"`
	CandidateDigest     string `json:"candidate_digest"`
	AfterDigest         string `json:"after_digest"`
	ObservationDigest   string `json:"observation_digest"`
}

type StressWitnessLineView struct {
	TrajectoryID string `json:"trajectory_id"`
	UnitID       string `json:"unit_id"`
	FixturePath  string `json:"fixture_path"`
	Line         int    `json:"line"`
	Content      string `json:"content"`
}

type StressView struct {
	CaseStudyID          string                    `json:"case_study_id"`
	DataRole             string                    `json:"data_role"`
	Status               string                    `json:"status"`
	TaskRequirement      string                    `json:"task_requirement"`
	RelationID           string                    `json:"relation_id"`
	RelationKind         string                    `json:"relation_kind"`
	RelationDigest       string                    `json:"relation_digest"`
	ControlArmID         string                    `json:"control_arm_id"`
	OriginalOrder        []string                  `json:"original_order"`
	TransformedOrder     []string                  `json:"transformed_order"`
	OriginalSelected     string                    `json:"original_selected"`
	TransformedSelected  string                    `json:"transformed_selected"`
	Outcome              string                    `json:"outcome"`
	OriginalInputDigest  string                    `json:"original_input_digest"`
	ReducedInputDigest   string                    `json:"reduced_input_digest"`
	OriginalLineUnits    int                       `json:"original_line_units"`
	FinalLineUnits       int                       `json:"final_line_units"`
	ReductionAttempts    int                       `json:"reduction_attempts"`
	AcceptedReductions   int                       `json:"accepted_reductions"`
	RejectedReductions   int                       `json:"rejected_reductions"`
	ReductionBasisPoints int                       `json:"reduction_basis_points"`
	Minimality           string                    `json:"minimality"`
	Steps                []StressReductionStepView `json:"steps"`
	FinalWitness         []StressWitnessLineView   `json:"final_witness"`
	EmpiricalUnits       int                       `json:"empirical_units"`
	ProviderCalls        int                       `json:"provider_calls"`
	NetworkRequired      bool                      `json:"network_required"`
	ClaimBoundary        string                    `json:"claim_boundary"`
	AllowedClaims        []string                  `json:"allowed_claims"`
	ForbiddenClaims      []string                  `json:"forbidden_claims"`
	ReproductionCommand  string                    `json:"reproduction_command"`
	Digest               string                    `json:"digest"`
	Source               ArtifactRef               `json:"source"`
}

type DatasetView struct {
	DatasetID        string      `json:"dataset_id"`
	Title            string      `json:"title"`
	EvidenceRole     string      `json:"evidence_role"`
	Counts           []CountView `json:"counts"`
	KnownLimitations []string    `json:"known_limitations"`
	OutOfScopeUses   []string    `json:"out_of_scope_uses"`
	Source           ArtifactRef `json:"source"`
}

type LimitationView struct {
	ID                  string       `json:"id"`
	ArtifactStatus      string       `json:"artifact_status"`
	CurrentAvailability Availability `json:"current_availability"`
	Boundary            string       `json:"boundary"`
	RequiredResolution  string       `json:"required_resolution"`
	Source              ArtifactRef  `json:"source"`
	ResolvedBy          *ArtifactRef `json:"resolved_by"`
}

type ReleaseView struct {
	ReleaseID                  string            `json:"release_id"`
	Version                    string            `json:"version"`
	Files                      int               `json:"files"`
	FilesVerified              int               `json:"files_verified"`
	AllFilesVerified           bool              `json:"all_files_verified"`
	PublicProjection           bool              `json:"public_projection"`
	RestrictedMaterialExcluded bool              `json:"restricted_material_excluded"`
	Manifest                   []ReleaseFileView `json:"manifest"`
	Source                     ArtifactRef       `json:"source"`
}

type ReleaseFileView struct {
	Path          string `json:"path"`
	Role          string `json:"role"`
	Bytes         int64  `json:"bytes"`
	PayloadSHA256 string `json:"payload_sha256"`
}

type ExtensionView struct {
	ExtensionID   string                       `json:"extension_id"`
	OwnerTask     string                       `json:"owner_task"`
	RequiredTypes []string                     `json:"required_types"`
	Components    []ExtensionComponentIdentity `json:"components"`
	MissingTypes  []string                     `json:"missing_types"`
	Availability  Availability                 `json:"availability"`
}

type ExtensionComponentIdentity struct {
	TypeID      string `json:"type_id"`
	ComponentID string `json:"component_id"`
}

type ReproductionView struct {
	Command         string      `json:"command"`
	NetworkRequired bool        `json:"network_required"`
	ProviderCalls   int         `json:"provider_calls"`
	Source          ArtifactRef `json:"source"`
}

type Report struct {
	SchemaVersion     string                               `json:"schema_version"`
	CanonicalPolicy   string                               `json:"canonical_policy"`
	Capsule           CapsuleBinding                       `json:"capsule"`
	Scope             ScopeView                            `json:"scope"`
	Claim             ClaimView                            `json:"claim"`
	BOM               BOMView                              `json:"bom"`
	Method            MethodView                           `json:"method"`
	OwnerInspection   OwnerInspectionView                  `json:"owner_inspection"`
	Transport         TransportView                        `json:"transport"`
	Loss              LossCertificateView                  `json:"loss"`
	Stress            StressView                           `json:"stress"`
	Reliance          *reliance.RelianceExplorerProjection `json:"reliance,omitempty"`
	Profile           *ProfileExplorerView                 `json:"profile,omitempty"`
	IdenticalResponse *IdenticalResponseView               `json:"identical_response,omitempty"`
	Dataset           DatasetView                          `json:"dataset"`
	Limitations       []LimitationView                     `json:"limitations"`
	Release           ReleaseView                          `json:"release"`
	Challenge         ChallengeView                        `json:"challenge"`
	Extensions        []ExtensionView                      `json:"extensions"`
	Reproduction      ReproductionView                     `json:"reproduction"`
	Digest            string                               `json:"digest"`
}

func (report Report) Validate() error {
	if err := report.validateIdentity(); err != nil {
		return err
	}
	for _, validation := range []func() error{
		report.validateScope, report.validateClaim, report.validateBOM, report.validateMethod,
		report.validateOwnerInspection, report.validateTransport, report.validateLoss, report.validateDataset, report.validateLimitations,
		report.validateStress, report.validateReliance, report.validateIdenticalResponse, report.validateRelease, report.validateChallenge, report.validateExtensions, report.validateReproduction,
	} {
		if err := validation(); err != nil {
			return err
		}
	}
	digest, err := reportDigest(report)
	if err != nil || digest != report.Digest {
		return errors.New("evidence explorer report digest is invalid")
	}
	return nil
}

func EncodeReport(report Report) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(report)
}

func DecodeReport(raw []byte) (Report, error) {
	var report Report
	if err := protocol.DecodeStrict(raw, &report); err != nil {
		return Report{}, fmt.Errorf("decode evidence explorer report: %w", err)
	}
	canonical, err := protocol.CanonicalMarshal(report)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Report{}, errors.New("evidence explorer report is not canonical JSON")
	}
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	return report, nil
}

func reportDigest(report Report) (string, error) {
	report.Digest = ""
	raw, err := protocol.CanonicalMarshal(report)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func (report Report) validateIdentity() error {
	if report.SchemaVersion != ReportSchemaVersion || report.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(report.Capsule.CapsuleID) || !validDigest(report.Capsule.ManifestDigest) ||
		!validDigest(report.Capsule.LedgerDigest) || !validDigest(report.Capsule.AutopsyDigest) ||
		!validDigest(report.Digest) || validateArtifactRef(report.Capsule.LedgerSource) != nil ||
		validateArtifactRef(report.Capsule.AutopsySource) != nil ||
		report.Capsule.LedgerSource.ArtifactDigest != report.Capsule.LedgerDigest ||
		report.Capsule.AutopsySource.ArtifactDigest != report.Capsule.AutopsyDigest {
		return errors.New("evidence explorer report identity is invalid")
	}
	if report.Capsule.LedgerSource.Kind != ArtifactSidecar || report.Capsule.LedgerSource.ID != "claim-ledger" ||
		report.Capsule.LedgerSource.SchemaVersion != claimledger.LedgerSchemaVersion ||
		report.Capsule.AutopsySource.Kind != ArtifactSidecar || report.Capsule.AutopsySource.ID != "claim-autopsy" ||
		report.Capsule.AutopsySource.SchemaVersion != claimledger.AutopsySchemaVersion {
		return errors.New("evidence explorer report sidecar identity is invalid")
	}
	return nil
}

func (report Report) validateScope() error {
	scope := report.Scope
	if scope.Evaluator == "" || !scope.RouteAvailability.Valid() ||
		(scope.Route != nil) != (scope.RouteAvailability == AvailabilityAvailable) ||
		scope.EvidenceRole == "" || scope.ReleaseVersion == "" || scope.DevelopmentFixtures < 1 ||
		scope.EmpiricalTaskGroups != 0 || scope.ResearchAdmittedSources != 0 || scope.ProviderCalls != 0 ||
		scope.Empirical || scope.ProviderRanking || !scope.RestrictedMaterialExcluded {
		return errors.New("evidence explorer scope is incomplete")
	}
	return nil
}

func (report Report) validateClaim() error {
	claim := report.Claim
	if !validIdentifier(claim.ClaimID) || claim.FailableProperty == "" || claim.ObservableFailureCondition == "" ||
		!claim.Accepted || len(claim.ClaimBoundary) == 0 || len(claim.KnownLimitations) == 0 ||
		len(claim.OutOfScopeUses) == 0 || validateArtifactRef(claim.Source) != nil ||
		claim.Source.Kind != ArtifactCapsuleComponent || claim.Source.SchemaVersion != lineage.BOMSchemaVersion {
		return errors.New("evidence explorer claim view is incomplete")
	}
	for name, values := range map[string][]string{
		"claim boundary":          claim.ClaimBoundary,
		"claim known limitations": claim.KnownLimitations,
		"claim out-of-scope uses": claim.OutOfScopeUses,
	} {
		if err := validateSortedUnique(name, values); err != nil {
			return err
		}
	}
	return nil
}

func (report Report) validateBOM() error {
	bom := report.BOM
	if bom.BOMID == "" || bom.CandidateID == "" || bom.Executable == "" || len(bom.Operands) == 0 ||
		len(bom.RequiredFields) == 0 || len(bom.DecisiveChannels) == 0 || len(bom.SurvivingChannels) == 0 ||
		!validDigest(bom.RepositoryStateDigest) || validateArtifactRef(bom.Source) != nil ||
		bom.Source != report.Claim.Source {
		return errors.New("evidence explorer BOM view is incomplete")
	}
	if err := validateUniqueNonEmpty("BOM operands", bom.Operands); err != nil {
		return err
	}
	for name, values := range map[string][]string{
		"BOM required fields": bom.RequiredFields, "BOM decisive channels": bom.DecisiveChannels,
		"BOM surviving channels": bom.SurvivingChannels,
	} {
		if err := validateSortedUnique(name, values); err != nil {
			return err
		}
	}
	if err := validateSortedUniqueAllowEmpty("BOM truncated required fields", bom.TruncatedRequiredFields); err != nil {
		return err
	}
	if len(bom.TruncatedRequiredFields) != 0 || bom.Freshness.ObservedStateDigest != bom.RepositoryStateDigest {
		return errors.New("evidence explorer accepted BOM is truncated or stale")
	}
	for _, channel := range bom.DecisiveChannels {
		if !slices.Contains(bom.SurvivingChannels, channel) {
			return errors.New("evidence explorer BOM lost a decisive channel")
		}
	}
	return validateFreshness(bom.Freshness)
}

func (report Report) validateMethod() error {
	method := report.Method
	if method.Current != "v3" || method.Boundary == "" || len(method.Generations) != 3 || len(method.Transitions) != 2 {
		return errors.New("evidence explorer method view is incomplete")
	}
	wantStates := []string{"falsified", "superseded", "admitted_development"}
	for index, generation := range method.Generations {
		if generation.State != wantStates[index] {
			return errors.New("evidence explorer method state is invalid")
		}
		if err := validateMethodGeneration(generation, index); err != nil {
			return err
		}
	}
	for index, transition := range method.Transitions {
		if err := validateMethodTransition(transition, index); err != nil {
			return err
		}
	}
	return nil
}

func (report Report) validateTransport() error {
	transport := report.Transport
	if transport.ClaimID != report.Claim.ClaimID || !transport.Accepted || len(transport.Paths) != 2 ||
		transport.SelectedPath != "non-failable-counterexample" || transport.SelectedLayer != "retained_bundle" ||
		transport.Paths[0].Evidence != report.Claim.Source {
		return errors.New("evidence explorer transport selection is invalid")
	}
	for index, pathView := range transport.Paths {
		if err := validateTransportPath(pathView, transport.ClaimID, index); err != nil {
			return err
		}
	}
	return nil
}

func (report Report) validateLoss() error {
	loss := report.Loss
	if loss.CertificateID == "" || loss.PathID != report.Transport.SelectedPath ||
		loss.FirstLossState == "" || loss.Disposition == "" || loss.Precedence < 1 ||
		loss.FailabilityResolution == "" || loss.FailabilityReason == "" || !loss.PublicSafe ||
		loss.RestrictedContent || loss.SupportedClaim == "" || len(loss.UnsupportedClaims) == 0 ||
		validateArtifactRef(loss.Source) != nil || loss.Source.Kind != ArtifactReleaseFile ||
		loss.Source.ID != lossCertificatePath || loss.Source.SchemaVersion != lineage.LossCertificateVersion {
		return errors.New("evidence explorer loss certificate view is incomplete")
	}
	if err := validateSortedUnique("loss unsupported claims", loss.UnsupportedClaims); err != nil {
		return err
	}
	for _, pathView := range report.Transport.Paths {
		if pathView.PathID == loss.PathID && pathView.Evidence == loss.Source &&
			pathView.TerminalState == loss.FirstLossState {
			return nil
		}
	}
	return errors.New("evidence explorer loss certificate is detached from its transport path")
}

func (report Report) validateDataset() error {
	dataset := report.Dataset
	if dataset.DatasetID == "" || dataset.Title == "" || dataset.EvidenceRole != report.Scope.EvidenceRole ||
		len(dataset.Counts) == 0 || len(dataset.KnownLimitations) == 0 || len(dataset.OutOfScopeUses) == 0 ||
		validateArtifactRef(dataset.Source) != nil || dataset.Source.Kind != ArtifactCapsuleComponent ||
		dataset.Source.SchemaVersion != lineage.DatasetCardSchemaVersion {
		return errors.New("evidence explorer dataset view is incomplete")
	}
	if err := validateCounts(dataset.Counts); err != nil {
		return err
	}
	for name, want := range map[string]int{
		"development_fixtures":      report.Scope.DevelopmentFixtures,
		"empirical_task_groups":     report.Scope.EmpiricalTaskGroups,
		"research_admitted_sources": report.Scope.ResearchAdmittedSources,
	} {
		if got, found := countByName(dataset.Counts, name); !found || got != want {
			return errors.New("evidence explorer dataset count differs from report scope")
		}
	}
	if err := validateSortedUnique("dataset known limitations", dataset.KnownLimitations); err != nil {
		return err
	}
	return validateSortedUnique("dataset out-of-scope uses", dataset.OutOfScopeUses)
}

func (report Report) validateLimitations() error {
	if len(report.Limitations) == 0 {
		return errors.New("evidence explorer limitations are absent")
	}
	previous := ""
	for _, limitation := range report.Limitations {
		if !validIdentifier(limitation.ID) || limitation.ID <= previous || limitation.ArtifactStatus == "" ||
			!limitation.CurrentAvailability.Valid() || limitation.Boundary == "" || limitation.RequiredResolution == "" ||
			validateArtifactRef(limitation.Source) != nil || limitation.Source.Kind != ArtifactReleaseFile ||
			limitation.Source.ID != limitationsPath || limitation.Source.SchemaVersion != lineage.LimitationsLedgerVersion {
			return errors.New("evidence explorer limitation is incomplete or unordered")
		}
		if limitation.ResolvedBy != nil && validateArtifactRef(*limitation.ResolvedBy) != nil {
			return errors.New("evidence explorer limitation resolution is invalid")
		}
		if (limitation.ResolvedBy != nil) != (limitation.CurrentAvailability == AvailabilityAvailable) {
			return errors.New("evidence explorer limitation availability differs from its resolution evidence")
		}
		if limitation.ID == "capsule_integration" &&
			(limitation.ResolvedBy == nil || *limitation.ResolvedBy != report.Capsule.AutopsySource) {
			return errors.New("evidence explorer capsule-integration resolution is detached from Claim Autopsy")
		}
		previous = limitation.ID
	}
	return nil
}

func (report Report) validateRelease() error {
	release := report.Release
	if release.ReleaseID == "" || release.Version != report.Scope.ReleaseVersion || release.Files < 1 ||
		release.FilesVerified != release.Files || !release.AllFilesVerified || !release.PublicProjection ||
		!release.RestrictedMaterialExcluded || len(release.Manifest) != release.Files ||
		validateArtifactRef(release.Source) != nil || release.Source.Kind != ArtifactCapsuleComponent ||
		release.Source.SchemaVersion != lineage.ReleaseSchemaVersion {
		return errors.New("evidence explorer release view is incomplete")
	}
	previous := ""
	for _, file := range release.Manifest {
		if !validReleasePath(file.Path) || file.Path <= previous || file.Role == "" || file.Bytes < 1 ||
			!validDigest(file.PayloadSHA256) {
			return errors.New("evidence explorer release manifest is incomplete or unordered")
		}
		previous = file.Path
	}
	return validateRequiredReleaseManifest(release.Manifest)
}

func (report Report) validateExtensions() error {
	return validateExtensionViews(report.Extensions)
}

func (report Report) validateReproduction() error {
	reproduction := report.Reproduction
	if reproduction.Command != "scripts/tests/run-claimcheck.sh" || reproduction.NetworkRequired || reproduction.ProviderCalls != 0 ||
		validateArtifactRef(reproduction.Source) != nil || reproduction.Source != report.Release.Source {
		return errors.New("evidence explorer reproduction contract is invalid")
	}
	return nil
}

func validateMethodGeneration(generation MethodGenerationView, index int) error {
	wantGeneration := []string{"v1", "v2", "v3"}
	if generation.Generation != wantGeneration[index] || generation.State == "" || len(generation.Evidence) == 0 ||
		len(generation.FrozenDenominators) == 0 || len(generation.Observations) == 0 ||
		len(generation.NonClaims) == 0 || len(generation.GuardingTests) == 0 ||
		generation.DeepLink != "#autopsy/method/"+generation.Generation {
		return errors.New("evidence explorer method generation is incomplete")
	}
	for _, reference := range generation.Evidence {
		if err := validateArtifactRef(reference); err != nil {
			return err
		}
	}
	if err := validateCounts(generation.FrozenDenominators); err != nil {
		return err
	}
	for name, values := range map[string][]string{
		"method observations":   generation.Observations,
		"method non-claims":     generation.NonClaims,
		"method guarding tests": generation.GuardingTests,
	} {
		if err := validateUniqueNonEmpty(name, values); err != nil {
			return err
		}
	}
	return nil
}

func validateMethodTransition(transition MethodTransitionView, index int) error {
	want := [][2]string{{"v1", "v2"}, {"v2", "v3"}}
	if transition.From != want[index][0] || transition.To != want[index][1] || transition.Reason == "" ||
		len(transition.Evidence) == 0 || transition.GuardingTest == "" ||
		transition.FromDeepLink != "#autopsy/method/"+transition.From ||
		transition.TargetDeepLink != "#autopsy/method/"+transition.To {
		return errors.New("evidence explorer method transition is invalid")
	}
	for _, reference := range transition.Evidence {
		if err := validateArtifactRef(reference); err != nil {
			return err
		}
	}
	return nil
}

func validateTransportPath(pathView TransportPathView, claimID string, index int) error {
	wantIDs := []string{"accepted-positive", "non-failable-counterexample"}
	if pathView.PathID != wantIDs[index] || pathView.Label == "" || pathView.TerminalState == "" ||
		len(pathView.Layers) != 5 || validateArtifactRef(pathView.Evidence) != nil ||
		pathView.DeepLink != "#autopsy/transport/"+claimID+"/"+pathView.PathID {
		return errors.New("evidence explorer transport path is invalid")
	}
	wantLayers := []string{"runtime_witness", "native_export", "canonical_graph", "retained_bundle", "verifier_request"}
	wantStatuses := [][]string{
		{"survived", "survived", "survived", "survived", "survived"},
		{"survived", "survived", "survived", "first_loss", "not_reached"},
	}
	for layerIndex, layer := range pathView.Layers {
		if err := validateTransportLayer(layer, pathView.DeepLink, wantLayers[layerIndex], wantStatuses[index][layerIndex], index, layerIndex); err != nil {
			return err
		}
	}
	return nil
}

func validateTransportLayer(
	layer TransportLayerView,
	pathDeepLink string,
	wantLayer string,
	wantStatus string,
	pathIndex int,
	layerIndex int,
) error {
	if layer.Layer != wantLayer || layer.Status != wantStatus || !layer.ObjectDigestAvailability.Valid() ||
		!layer.DetailAvailability.Valid() || layer.DeepLink != pathDeepLink+"/"+layer.Layer {
		return errors.New("evidence explorer transport layer is invalid")
	}
	if (layer.ObjectDigest != nil) != (layer.ObjectDigestAvailability == AvailabilityAvailable) {
		return errors.New("evidence explorer transport object availability is inconsistent")
	}
	if layer.ObjectDigest != nil && !validDigest(*layer.ObjectDigest) {
		return errors.New("evidence explorer transport object digest is invalid")
	}
	detailAvailable := layer.DetailAvailability == AvailabilityAvailable
	if (len(layer.RequiredFields) > 0 || len(layer.DecisiveChannels) > 0) != detailAvailable {
		return errors.New("evidence explorer transport detail availability is inconsistent")
	}
	if detailAvailable {
		if err := validateSortedUnique("transport required fields", layer.RequiredFields); err != nil {
			return err
		}
		if err := validateSortedUnique("transport decisive channels", layer.DecisiveChannels); err != nil {
			return err
		}
	}
	wantAvailability := transportAvailability(pathIndex, layerIndex)
	if layer.ObjectDigestAvailability != wantAvailability || layer.DetailAvailability != wantAvailability {
		return errors.New("evidence explorer transport layer availability is invalid")
	}
	return nil
}

func transportAvailability(pathIndex int, layerIndex int) Availability {
	if pathIndex == 0 {
		return AvailabilityAvailable
	}
	if layerIndex < 3 {
		return AvailabilityUnsupported
	}
	return AvailabilityNotApplicable
}

func validateRequiredReleaseManifest(manifest []ReleaseFileView) error {
	required := map[string]string{
		acceptedBOMPath: "accepted_bom_example", datasetCardPath: "dataset_card",
		limitationsPath: "limitations", lineageGraphPath: "canonical_graph",
		lineageGraphSVGPath: "graph_projection", lossCertificatePath: "loss_certificate",
	}
	for _, file := range manifest {
		if role, found := required[file.Path]; found {
			if file.Role != role {
				return errors.New("evidence explorer required release role is invalid")
			}
			delete(required, file.Path)
		}
	}
	if len(required) != 0 {
		return errors.New("evidence explorer required release files are absent")
	}
	return nil
}

func validateFreshness(freshness FreshnessView) error {
	if freshness.State != "current" || !validDigest(freshness.ObservedStateDigest) ||
		freshness.ValidUntil != nil || freshness.InvalidationEdges != 0 {
		return errors.New("evidence explorer freshness view is incomplete")
	}
	validFrom, err := time.Parse(time.RFC3339Nano, freshness.ValidFrom)
	if err != nil {
		return errors.New("evidence explorer freshness start is invalid")
	}
	evaluatedAt, err := time.Parse(time.RFC3339Nano, freshness.EvaluatedAt)
	if err != nil || evaluatedAt.Before(validFrom) {
		return errors.New("evidence explorer freshness evaluation is invalid")
	}
	return nil
}

func validateArtifactRef(reference ArtifactRef) error {
	if !reference.Kind.Valid() || reference.ID == "" || reference.SchemaVersion == "" ||
		!validDigest(reference.PayloadSHA256) || !validDigest(reference.ArtifactDigest) {
		return errors.New("evidence explorer artifact reference is invalid")
	}
	if reference.Kind == ArtifactCapsuleComponent && !validDigest(reference.ID) {
		return errors.New("evidence explorer capsule component identity is invalid")
	}
	if reference.Kind == ArtifactReleaseFile && !validReleasePath(reference.ID) {
		return errors.New("evidence explorer release file identity is invalid")
	}
	if reference.Kind == ArtifactSidecar && !validIdentifier(reference.ID) {
		return errors.New("evidence explorer sidecar identity is invalid")
	}
	return nil
}

func validateCounts(counts []CountView) error {
	previous := ""
	for _, count := range counts {
		if !validIdentifier(count.Name) || count.Name <= previous || count.Value < 0 {
			return errors.New("evidence explorer counts must be non-negative, unique, and sorted")
		}
		previous = count.Name
	}
	return nil
}

func countByName(counts []CountView, name string) (int, bool) {
	for _, count := range counts {
		if count.Name == name {
			return count.Value, true
		}
	}
	return 0, false
}

func validateSortedUnique(name string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must not be empty", name)
	}
	return validateSortedUniqueAllowEmpty(name, values)
}

func validateSortedUniqueAllowEmpty(name string, values []string) error {
	if !slices.IsSorted(values) || len(slices.Compact(slices.Clone(values))) != len(values) {
		return fmt.Errorf("%s must be sorted and unique", name)
	}
	for _, value := range values {
		if value == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
	}
	return nil
}

func validateUniqueNonEmpty(name string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s contains duplicate value %q", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validIdentifier(value string) bool {
	if value == "" || value != strings.ToLower(value) {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func validReleasePath(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && !strings.Contains(value, "\\") &&
		!strings.HasPrefix(value, "/") && path.Clean(value) == value && value != "." &&
		value != ".." && !strings.HasPrefix(value, "../")
}
