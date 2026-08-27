package capsule

import (
	"errors"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/product"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	SourceTreeProvenanceSchemaVersion = "evalwitness.provenance.source-tree.v1"
	BuildProvenanceSchemaVersionV1    = "evalwitness.provenance.build.v1"
	BuildProvenanceSchemaVersion      = "evalwitness.provenance.build.v2"
	DatasetProvenanceSchemaVersion    = "evalwitness.provenance.dataset.v1"
	RouteProvenanceSchemaVersion      = "evalwitness.provenance.route.v1"
	StudyProvenanceSchemaVersion      = "evalwitness.provenance.study.v1"
	AnalysisProvenanceSchemaVersion   = "evalwitness.provenance.analysis.v1"
	ClockProvenanceSchemaVersion      = "evalwitness.provenance.clocks.v1"
	RenderReceiptSchemaVersion        = "evalwitness.provenance.render-receipt.v1"
	SourceTreeAlgorithm               = "evalwitness.git-visible-source-tree.v1"
)

type EvidenceAvailability string

const (
	AvailabilityAvailable               EvidenceAvailability = "available"
	AvailabilityFixtureOnly             EvidenceAvailability = "fixture_only"
	AvailabilityNotRun                  EvidenceAvailability = "not_run"
	AvailabilityUnavailablePublicSource EvidenceAvailability = "unavailable_public_source"
	AvailabilityOmittedPrivate          EvidenceAvailability = "omitted_private"
)

func (availability EvidenceAvailability) Valid() bool {
	switch availability {
	case AvailabilityAvailable, AvailabilityFixtureOnly, AvailabilityNotRun,
		AvailabilityUnavailablePublicSource, AvailabilityOmittedPrivate:
		return true
	default:
		return false
	}
}

type SourceTreeEntry struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	GitMode    string `json:"git_mode"`
	State      string `json:"state"`
	Bytes      int64  `json:"bytes"`
	SHA256     string `json:"sha256,omitempty"`
	LinkTarget string `json:"link_target,omitempty"`
}

type SourceTreeProvenance struct {
	SchemaVersion string            `json:"schema_version"`
	Algorithm     string            `json:"algorithm"`
	VCS           string            `json:"vcs"`
	Commit        string            `json:"commit"`
	Dirty         bool              `json:"dirty"`
	StatusDigest  string            `json:"status_digest"`
	Files         int               `json:"files"`
	Bytes         int64             `json:"bytes"`
	Entries       []SourceTreeEntry `json:"entries"`
	Digest        string            `json:"digest"`
}

func (source SourceTreeProvenance) Validate() error {
	if source.SchemaVersion != SourceTreeProvenanceSchemaVersion || source.Algorithm != SourceTreeAlgorithm ||
		source.VCS != "git" || !validVCSRevision(source.Commit) || !validDigest(source.StatusDigest) ||
		!validDigest(source.Digest) || source.Files < 1 || source.Bytes < 1 || len(source.Entries) != source.Files {
		return errors.New("source-tree provenance identity is invalid")
	}
	previous := ""
	var total int64
	for _, entry := range source.Entries {
		if !validSourcePath(entry.Path) || entry.Path <= previous || entry.Bytes < 0 {
			return errors.New("source-tree entries are invalid, duplicated, or unsorted")
		}
		switch entry.State {
		case "present", "untracked":
			if entry.Kind != "file" && entry.Kind != "symlink" || !validDigest(entry.SHA256) {
				return errors.New("source-tree entry content identity is invalid")
			}
			if entry.Kind == "file" && entry.LinkTarget != "" || entry.Kind == "symlink" && entry.LinkTarget == "" {
				return errors.New("source-tree symlink metadata is invalid")
			}
		case "deleted":
			if entry.SHA256 != "" || entry.LinkTarget != "" || entry.Bytes != 0 {
				return errors.New("deleted source-tree entry contains content")
			}
		default:
			return errors.New("source-tree entry state is invalid")
		}
		if entry.GitMode != "100644" && entry.GitMode != "100755" && entry.GitMode != "120000" {
			return errors.New("source-tree entry mode is invalid")
		}
		total += entry.Bytes
		previous = entry.Path
	}
	if total != source.Bytes {
		return errors.New("source-tree byte total is invalid")
	}
	digest, err := sourceTreeProvenanceDigest(source)
	if err != nil || digest != source.Digest {
		return errors.New("source-tree provenance digest is invalid")
	}
	return nil
}

type BuildSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type BuildDependency struct {
	Path           string `json:"path"`
	Version        string `json:"version"`
	Sum            string `json:"sum,omitempty"`
	ReplacedByPath string `json:"replaced_by_path,omitempty"`
	ReplacedBy     string `json:"replaced_by_version,omitempty"`
	ReplacedBySum  string `json:"replaced_by_sum,omitempty"`
}

type BuildProvenance struct {
	SchemaVersion            string            `json:"schema_version"`
	ProductVersion           string            `json:"product_version,omitempty"`
	ArtifactKind             string            `json:"artifact_kind"`
	BinaryDigest             string            `json:"binary_digest"`
	BinaryBytes              int64             `json:"binary_bytes"`
	BinaryIncluded           bool              `json:"binary_included"`
	Commit                   string            `json:"commit"`
	Dirty                    bool              `json:"dirty"`
	SourceTreeDigest         string            `json:"source_tree_digest"`
	GoVersion                string            `json:"go_version"`
	MainPackage              string            `json:"main_package"`
	TargetGOOS               string            `json:"target_goos"`
	TargetGOARCH             string            `json:"target_goarch"`
	BuildFlags               []BuildSetting    `json:"build_flags"`
	Dependencies             []BuildDependency `json:"dependencies"`
	DependencyManifestDigest string            `json:"dependency_manifest_digest"`
	EmbeddedVCSRevision      string            `json:"embedded_vcs_revision,omitempty"`
	EmbeddedVCSModified      string            `json:"embedded_vcs_modified"`
	SourceMatchStatus        string            `json:"source_match_status"`
	Reproducibility          string            `json:"reproducibility"`
	Digest                   string            `json:"digest"`
}

func (build BuildProvenance) Validate() error {
	if build.SchemaVersion != BuildProvenanceSchemaVersionV1 && build.SchemaVersion != BuildProvenanceSchemaVersion ||
		!validIdentifier(build.ArtifactKind) ||
		!validDigest(build.BinaryDigest) || build.BinaryBytes < 1 || build.BinaryIncluded ||
		!validVCSRevision(build.Commit) || !validDigest(build.SourceTreeDigest) ||
		!strings.HasPrefix(build.GoVersion, "go") || strings.TrimSpace(build.MainPackage) == "" ||
		strings.TrimSpace(build.TargetGOOS) == "" || strings.TrimSpace(build.TargetGOARCH) == "" ||
		!validDigest(build.DependencyManifestDigest) || !validDigest(build.Digest) ||
		build.Reproducibility != "digest-bound-external-binary" {
		return errors.New("build provenance identity is invalid")
	}
	if build.SchemaVersion == BuildProvenanceSchemaVersionV1 && build.ProductVersion != "" ||
		build.SchemaVersion == BuildProvenanceSchemaVersion && !product.ValidVersion(build.ProductVersion) {
		return errors.New("build product version is invalid for its schema")
	}
	if build.EmbeddedVCSRevision != "" && !validVCSRevision(build.EmbeddedVCSRevision) {
		return errors.New("build embedded VCS revision is invalid")
	}
	if build.EmbeddedVCSModified != "true" && build.EmbeddedVCSModified != "false" && build.EmbeddedVCSModified != "unavailable" {
		return errors.New("build embedded VCS modified state is invalid")
	}
	if build.SourceMatchStatus != "matched" && build.SourceMatchStatus != "embedded-vcs-unavailable" {
		return errors.New("build source match status is invalid")
	}
	if build.SourceMatchStatus == "matched" && build.EmbeddedVCSRevision == "" ||
		build.SourceMatchStatus == "embedded-vcs-unavailable" && build.EmbeddedVCSRevision != "" {
		return errors.New("build source match status disagrees with embedded VCS provenance")
	}
	if build.EmbeddedVCSModified != "unavailable" && (build.EmbeddedVCSModified == "true") != build.Dirty {
		return errors.New("build embedded VCS modified state disagrees with source provenance")
	}
	if err := validateBuildSettings(build.BuildFlags); err != nil {
		return err
	}
	if err := validateBuildDependencies(build.Dependencies); err != nil {
		return err
	}
	dependencyDigest, err := protocol.Digest(build.Dependencies)
	if err != nil || dependencyDigest != build.DependencyManifestDigest {
		return errors.New("build dependency manifest digest is invalid")
	}
	digest, err := buildProvenanceDigest(build)
	if err != nil || digest != build.Digest {
		return errors.New("build provenance digest is invalid")
	}
	return nil
}

type DatasetProvenance struct {
	SchemaVersion              string               `json:"schema_version"`
	DatasetID                  string               `json:"dataset_id"`
	Source                     string               `json:"source"`
	License                    string               `json:"license"`
	SourceSpecificationLicense string               `json:"source_specification_license"`
	AcquisitionStatus          EvidenceAvailability `json:"acquisition_status"`
	AcquisitionChecksum        string               `json:"acquisition_checksum"`
	CanonicalSourceDigest      string               `json:"canonical_source_digest"`
	TaskIDs                    []string             `json:"task_ids"`
	TaskIDsDigest              string               `json:"task_ids_digest"`
	Split                      string               `json:"split"`
	TrajectoryDigests          []string             `json:"trajectory_digests"`
	LabelStatus                EvidenceAvailability `json:"label_status"`
	LabelDigests               []string             `json:"label_digests"`
	Exclusions                 []string             `json:"exclusions"`
	SourceComponentID          string               `json:"source_component_id"`
	TrajectoryComponentID      string               `json:"trajectory_component_id"`
	DatasetCardComponentID     string               `json:"dataset_card_component_id"`
	Digest                     string               `json:"digest"`
}

func (dataset DatasetProvenance) Validate() error {
	if dataset.SchemaVersion != DatasetProvenanceSchemaVersion || !validIdentifier(dataset.DatasetID) ||
		strings.TrimSpace(dataset.Source) == "" || strings.TrimSpace(dataset.License) == "" ||
		strings.TrimSpace(dataset.SourceSpecificationLicense) == "" || !dataset.AcquisitionStatus.Valid() ||
		!validDigest(dataset.AcquisitionChecksum) || !validDigest(dataset.CanonicalSourceDigest) || !validDigest(dataset.TaskIDsDigest) ||
		strings.TrimSpace(dataset.Split) == "" || !dataset.LabelStatus.Valid() || !validDigest(dataset.Digest) ||
		!validDigest(dataset.SourceComponentID) || !validDigest(dataset.TrajectoryComponentID) || !validDigest(dataset.DatasetCardComponentID) {
		return errors.New("dataset provenance identity is invalid")
	}
	if err := validateSortedUniqueText("dataset task IDs", dataset.TaskIDs, 1); err != nil {
		return err
	}
	if err := validateDigestList("dataset trajectory digests", dataset.TrajectoryDigests, 1); err != nil {
		return err
	}
	if err := validateDigestList("dataset label digests", dataset.LabelDigests, 0); err != nil {
		return err
	}
	if err := validateSortedUniqueText("dataset exclusions", dataset.Exclusions, 1); err != nil {
		return err
	}
	if dataset.LabelStatus == AvailabilityNotRun && len(dataset.LabelDigests) != 0 {
		return errors.New("not-run dataset labels contain digests")
	}
	taskDigest, err := protocol.Digest(dataset.TaskIDs)
	if err != nil || taskDigest != dataset.TaskIDsDigest {
		return errors.New("dataset task ID digest is invalid")
	}
	digest, err := datasetProvenanceDigest(dataset)
	if err != nil || digest != dataset.Digest {
		return errors.New("dataset provenance digest is invalid")
	}
	return nil
}

type RouteRequestProvenance struct {
	CaseID         string `json:"case_id"`
	Fingerprint    string `json:"fingerprint"`
	ProviderID     string `json:"provider_id"`
	RouteID        string `json:"route_id"`
	RequestedModel string `json:"requested_model"`
}

type RouteProvenance struct {
	SchemaVersion        string                   `json:"schema_version"`
	Requests             []RouteRequestProvenance `json:"requests"`
	RequestStatus        EvidenceAvailability     `json:"request_status"`
	ResponseStatus       EvidenceAvailability     `json:"response_status"`
	AttestationStatus    EvidenceAvailability     `json:"attestation_status"`
	ProviderCallsStatus  EvidenceAvailability     `json:"provider_calls_status"`
	ExternalActionStatus string                   `json:"external_action_status"`
	ServedIdentities     []string                 `json:"served_identities"`
	CheckpointAssertions []string                 `json:"checkpoint_assertions"`
	ObservedLimitations  []string                 `json:"observed_limitations"`
	RequestComponentID   string                   `json:"request_component_id"`
	ProtocolRunID        string                   `json:"protocol_run_component_id"`
	Digest               string                   `json:"digest"`
}

func (route RouteProvenance) Validate() error {
	if route.SchemaVersion != RouteProvenanceSchemaVersion || !route.RequestStatus.Valid() ||
		!route.ResponseStatus.Valid() || !route.AttestationStatus.Valid() || !route.ProviderCallsStatus.Valid() ||
		route.ExternalActionStatus != "not_authorized" || !validDigest(route.RequestComponentID) ||
		!validDigest(route.ProtocolRunID) || !validDigest(route.Digest) || len(route.Requests) == 0 {
		return errors.New("route provenance identity is invalid")
	}
	previous := ""
	for _, request := range route.Requests {
		if !validIdentifier(request.CaseID) || request.CaseID <= previous || !validDigest(request.Fingerprint) ||
			strings.TrimSpace(request.ProviderID) == "" || !strings.HasPrefix(request.RouteID, "route-") ||
			!validDigest(strings.TrimPrefix(request.RouteID, "route-")) || strings.TrimSpace(request.RequestedModel) == "" {
			return errors.New("route request provenance is invalid, duplicated, or unsorted")
		}
		previous = request.CaseID
	}
	for name, values := range map[string][]string{
		"route served identities": route.ServedIdentities, "route checkpoint assertions": route.CheckpointAssertions,
		"route limitations": route.ObservedLimitations,
	} {
		minimum := 0
		if name == "route limitations" {
			minimum = 1
		}
		if err := validateSortedUniqueText(name, values, minimum); err != nil {
			return err
		}
	}
	if route.ResponseStatus != AvailabilityAvailable && (len(route.ServedIdentities) != 0 || len(route.CheckpointAssertions) != 0) {
		return errors.New("unavailable route responses expose served or checkpoint identities")
	}
	digest, err := routeProvenanceDigest(route)
	if err != nil || digest != route.Digest {
		return errors.New("route provenance digest is invalid")
	}
	return nil
}

type StudyPlanBinding struct {
	TaskID      string `json:"task_id"`
	Name        string `json:"name"`
	ComponentID string `json:"component_id"`
}

type StudyProvenance struct {
	SchemaVersion      string               `json:"schema_version"`
	StudyID            string               `json:"study_id"`
	EvidenceRole       string               `json:"evidence_role"`
	ProviderStudy      EvidenceAvailability `json:"provider_study_status"`
	HumanStudy         EvidenceAvailability `json:"human_study_status"`
	ExternalAction     string               `json:"external_action_status"`
	ConfirmationStatus string               `json:"confirmation_status"`
	ClaimCeiling       string               `json:"claim_ceiling"`
	Plans              []StudyPlanBinding   `json:"plans"`
	Digest             string               `json:"digest"`
}

func (study StudyProvenance) Validate() error {
	if study.SchemaVersion != StudyProvenanceSchemaVersion || !validIdentifier(study.StudyID) ||
		study.EvidenceRole != "development_reference" || !study.ProviderStudy.Valid() || !study.HumanStudy.Valid() ||
		study.ExternalAction != "not_authorized" || study.ConfirmationStatus != "not_admitted" ||
		strings.TrimSpace(study.ClaimCeiling) == "" || !validDigest(study.Digest) || len(study.Plans) < 3 {
		return errors.New("study provenance identity is invalid")
	}
	previous := ""
	for _, plan := range study.Plans {
		key := plan.TaskID + "\x00" + plan.Name
		if !validTaskID(plan.TaskID) || !validIdentifier(plan.Name) || !validDigest(plan.ComponentID) || key <= previous {
			return errors.New("study plan bindings are invalid, duplicated, or unsorted")
		}
		previous = key
	}
	digest, err := studyProvenanceDigest(study)
	if err != nil || digest != study.Digest {
		return errors.New("study provenance digest is invalid")
	}
	return nil
}

type AnalysisBinding struct {
	AnalysisID       string   `json:"analysis_id"`
	Version          string   `json:"version"`
	Command          []string `json:"command"`
	InputComponents  []string `json:"input_components"`
	OutputComponents []string `json:"output_components"`
	Status           string   `json:"status"`
}

type AnalysisProvenance struct {
	SchemaVersion         string            `json:"schema_version"`
	ProviderCallsRequired int               `json:"provider_calls_required"`
	Analyses              []AnalysisBinding `json:"analyses"`
	Exclusions            []string          `json:"exclusions"`
	Digest                string            `json:"digest"`
}

func (analysis AnalysisProvenance) Validate() error {
	if analysis.SchemaVersion != AnalysisProvenanceSchemaVersion || analysis.ProviderCallsRequired != 0 ||
		!validDigest(analysis.Digest) || len(analysis.Analyses) == 0 {
		return errors.New("analysis provenance identity is invalid")
	}
	previous := ""
	for _, binding := range analysis.Analyses {
		if !validIdentifier(binding.AnalysisID) || binding.AnalysisID <= previous || strings.TrimSpace(binding.Version) == "" ||
			binding.Status != "completed" || len(binding.Command) == 0 {
			return errors.New("analysis bindings are invalid, duplicated, or unsorted")
		}
		for _, argument := range binding.Command {
			if strings.TrimSpace(argument) == "" {
				return errors.New("analysis command contains an empty argument")
			}
		}
		if err := validateDigestList("analysis input components", binding.InputComponents, 1); err != nil {
			return err
		}
		if err := validateDigestList("analysis output components", binding.OutputComponents, 1); err != nil {
			return err
		}
		previous = binding.AnalysisID
	}
	if err := validateSortedUniqueText("analysis exclusions", analysis.Exclusions, 1); err != nil {
		return err
	}
	digest, err := analysisProvenanceDigest(analysis)
	if err != nil || digest != analysis.Digest {
		return errors.New("analysis provenance digest is invalid")
	}
	return nil
}

type ClockEvent struct {
	Kind               string   `json:"kind"`
	Status             string   `json:"status"`
	OccurredAt         string   `json:"occurred_at,omitempty"`
	SourceComponentIDs []string `json:"source_component_ids"`
	Boundary           string   `json:"boundary"`
}

type ClockProvenance struct {
	SchemaVersion string       `json:"schema_version"`
	Events        []ClockEvent `json:"events"`
	Digest        string       `json:"digest"`
}

func (clocks ClockProvenance) Validate() error {
	if clocks.SchemaVersion != ClockProvenanceSchemaVersion || !validDigest(clocks.Digest) || len(clocks.Events) != 3 {
		return errors.New("clock provenance identity is invalid")
	}
	wanted := []string{"analysis", "collection", "replay"}
	for index, event := range clocks.Events {
		if event.Kind != wanted[index] || strings.TrimSpace(event.Boundary) == "" {
			return errors.New("clock events are incomplete or unsorted")
		}
		switch event.Status {
		case "observed":
			if _, err := time.Parse(time.RFC3339Nano, event.OccurredAt); err != nil {
				return errors.New("observed clock event has an invalid timestamp")
			}
		case "completed_time_unavailable":
			if event.OccurredAt != "" || len(event.SourceComponentIDs) == 0 {
				return errors.New("time-unavailable clock event has invalid evidence")
			}
		case "not_run":
			if event.OccurredAt != "" || len(event.SourceComponentIDs) != 0 {
				return errors.New("not-run clock event contains observation evidence")
			}
		default:
			return errors.New("clock event status is invalid")
		}
		if err := validateDigestList("clock source components", event.SourceComponentIDs, 0); err != nil {
			return err
		}
	}
	digest, err := clockProvenanceDigest(clocks)
	if err != nil || digest != clocks.Digest {
		return errors.New("clock provenance digest is invalid")
	}
	return nil
}

type RenderReceipt struct {
	SchemaVersion     string `json:"schema_version"`
	RendererID        string `json:"renderer_id"`
	SourceComponentID string `json:"source_component_id"`
	OutputDigest      string `json:"output_digest"`
	TimeStatus        string `json:"time_status"`
	RenderedAt        string `json:"rendered_at,omitempty"`
	Digest            string `json:"digest"`
}

func (receipt RenderReceipt) Validate() error {
	if receipt.SchemaVersion != RenderReceiptSchemaVersion || !validIdentifier(receipt.RendererID) ||
		!validDigest(receipt.SourceComponentID) || !validDigest(receipt.OutputDigest) || !validDigest(receipt.Digest) {
		return errors.New("render receipt identity is invalid")
	}
	switch receipt.TimeStatus {
	case "recorded":
		if _, err := time.Parse(time.RFC3339Nano, receipt.RenderedAt); err != nil {
			return errors.New("render receipt timestamp is invalid")
		}
	case "time_unavailable":
		if receipt.RenderedAt != "" {
			return errors.New("time-unavailable render receipt contains a timestamp")
		}
	default:
		return errors.New("render receipt time status is invalid")
	}
	digest, err := renderReceiptDigest(receipt)
	if err != nil || digest != receipt.Digest {
		return errors.New("render receipt digest is invalid")
	}
	return nil
}

func sourceTreeProvenanceDigest(value SourceTreeProvenance) (string, error) {
	value.Digest = ""
	return protocol.Digest(value)
}

func buildProvenanceDigest(value BuildProvenance) (string, error) {
	value.Digest = ""
	return protocol.Digest(value)
}

func datasetProvenanceDigest(value DatasetProvenance) (string, error) {
	value.Digest = ""
	return protocol.Digest(value)
}

func routeProvenanceDigest(value RouteProvenance) (string, error) {
	value.Digest = ""
	return protocol.Digest(value)
}

func studyProvenanceDigest(value StudyProvenance) (string, error) {
	value.Digest = ""
	return protocol.Digest(value)
}

func analysisProvenanceDigest(value AnalysisProvenance) (string, error) {
	value.Digest = ""
	return protocol.Digest(value)
}

func clockProvenanceDigest(value ClockProvenance) (string, error) {
	value.Digest = ""
	return protocol.Digest(value)
}

func renderReceiptDigest(value RenderReceipt) (string, error) {
	value.Digest = ""
	return protocol.Digest(value)
}

func validateBuildSettings(settings []BuildSetting) error {
	if len(settings) == 0 {
		return errors.New("build flags are empty")
	}
	previous := ""
	for _, setting := range settings {
		if setting.Key == "" || strings.ContainsAny(setting.Key, "= \t\r\n") ||
			strings.ContainsAny(setting.Value, "\r\n") || setting.Key <= previous {
			return errors.New("build flags are invalid, duplicated, or unsorted")
		}
		previous = setting.Key
	}
	return nil
}

func validateBuildDependencies(dependencies []BuildDependency) error {
	previous := ""
	for _, dependency := range dependencies {
		key := dependency.Path + "\x00" + dependency.Version
		if strings.TrimSpace(dependency.Path) == "" || strings.TrimSpace(dependency.Version) == "" || key <= previous ||
			strings.ContainsAny(dependency.Path+dependency.Version+dependency.Sum+dependency.ReplacedByPath+dependency.ReplacedBy+dependency.ReplacedBySum, "\r\n") {
			return errors.New("build dependencies are invalid, duplicated, or unsorted")
		}
		previous = key
	}
	return nil
}

func validateSortedUniqueText(name string, values []string, minimum int) error {
	if len(values) < minimum || values == nil {
		return errors.New(name + " are missing")
	}
	previous := ""
	for _, value := range values {
		if strings.TrimSpace(value) == "" || value <= previous {
			return errors.New(name + " are invalid, duplicated, or unsorted")
		}
		previous = value
	}
	return nil
}

func validateDigestList(name string, values []string, minimum int) error {
	if len(values) < minimum || values == nil {
		return errors.New(name + " are missing")
	}
	if !slices.IsSorted(values) {
		return errors.New(name + " are unsorted")
	}
	previous := ""
	for _, value := range values {
		if !validDigest(value) || value <= previous {
			return errors.New(name + " are invalid or duplicated")
		}
		previous = value
	}
	return nil
}

func validSourcePath(value string) bool {
	return value != "" && !strings.HasPrefix(value, "/") && path.Clean(value) == value &&
		value != "." && value != ".." && !strings.HasPrefix(value, "../") && !strings.ContainsRune(value, '\x00')
}

func validVCSRevision(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func validTaskID(value string) bool {
	if !strings.HasPrefix(value, "TASK-") || len(value) != 8 {
		return false
	}
	for _, character := range value[5:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
