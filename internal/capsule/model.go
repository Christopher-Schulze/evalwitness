package capsule

const (
	ManifestSchemaVersion     = "evalwitness.capsule-manifest.v1"
	RegistrySchemaVersion     = "evalwitness.capsule-registry.v1"
	VerificationSchemaVersion = "evalwitness.capsule-verification.v1"
	CanonicalPolicy           = "evalwitness.capsule-canonical-json.v1"
	DigestAlgorithm           = "sha256"
)

type Role string

const (
	RoleObservation  Role = "observation"
	RoleDerivation   Role = "derivation"
	RoleGovernance   Role = "governance"
	RoleAttestation  Role = "attestation"
	RoleCommitment   Role = "commitment"
	RolePresentation Role = "presentation"
)

func (r Role) Valid() bool {
	switch r {
	case RoleObservation, RoleDerivation, RoleGovernance, RoleAttestation, RoleCommitment, RolePresentation:
		return true
	default:
		return false
	}
}

type Visibility string

const (
	VisibilityPublic     Visibility = "public"
	VisibilityRestricted Visibility = "restricted"
	VisibilityPrivate    Visibility = "private"
)

func (v Visibility) Valid() bool {
	switch v {
	case VisibilityPublic, VisibilityRestricted, VisibilityPrivate:
		return true
	default:
		return false
	}
}

func (v Visibility) rank() int {
	switch v {
	case VisibilityPublic:
		return 0
	case VisibilityRestricted:
		return 1
	case VisibilityPrivate:
		return 2
	default:
		return -1
	}
}

type PayloadProfile string

const (
	PayloadExactBytes    PayloadProfile = "evalwitness.payload-exact-bytes.v1"
	PayloadCanonicalJSON PayloadProfile = "evalwitness.payload-canonical-json.v1"
)

func (p PayloadProfile) Valid() bool {
	return p == PayloadExactBytes || p == PayloadCanonicalJSON
}

type ParentResolution string

const (
	ParentInternal ParentResolution = "internal"
	ParentExternal ParentResolution = "external"
	ParentOmitted  ParentResolution = "omitted"
)

func (r ParentResolution) Valid() bool {
	return r == ParentInternal || r == ParentExternal || r == ParentOmitted
}

type EdgeKind string

const (
	EdgeDerivedFrom  EdgeKind = "derived_from"
	EdgeObservedFrom EdgeKind = "observed_from"
	EdgeGovernedBy   EdgeKind = "governed_by"
	EdgeAttests      EdgeKind = "attests"
	EdgeCommitsTo    EdgeKind = "commits_to"
	EdgeRedacts      EdgeKind = "redacts"
	EdgeRenders      EdgeKind = "renders"
	EdgeSupersedes   EdgeKind = "supersedes"
)

func (k EdgeKind) Valid() bool {
	switch k {
	case EdgeDerivedFrom, EdgeObservedFrom, EdgeGovernedBy, EdgeAttests, EdgeCommitsTo, EdgeRedacts, EdgeRenders, EdgeSupersedes:
		return true
	default:
		return false
	}
}

type ParentRule struct {
	Kind        EdgeKind           `json:"kind"`
	ParentType  string             `json:"parent_type"`
	Minimum     int                `json:"minimum"`
	Maximum     int                `json:"maximum"`
	Resolutions []ParentResolution `json:"resolutions"`
}

type ComponentType struct {
	TypeID              string         `json:"type_id"`
	SchemaID            string         `json:"schema_id"`
	Role                Role           `json:"role"`
	AllowedVisibilities []Visibility   `json:"allowed_visibilities"`
	MediaType           string         `json:"media_type"`
	PayloadProfile      PayloadProfile `json:"payload_profile"`
	ValidatorID         string         `json:"validator_id"`
	BindingValidatorID  string         `json:"binding_validator_id,omitempty"`
	ParentRules         []ParentRule   `json:"parent_rules"`
}

type RegistryDocument struct {
	SchemaVersion      string          `json:"schema_version"`
	CanonicalPolicy    string          `json:"canonical_policy"`
	RegistryID         string          `json:"registry_id"`
	BaseRegistryDigest string          `json:"base_registry_digest,omitempty"`
	Types              []ComponentType `json:"types"`
	Digest             string          `json:"digest"`
}

type ParentRef struct {
	Kind          EdgeKind         `json:"kind"`
	ComponentID   string           `json:"component_id"`
	TypeID        string           `json:"type_id"`
	Role          Role             `json:"role"`
	Visibility    Visibility       `json:"visibility"`
	Resolution    ParentResolution `json:"resolution"`
	CapsuleID     string           `json:"capsule_id,omitempty"`
	OmissionClass string           `json:"omission_class,omitempty"`
}

type PayloadIdentity struct {
	Algorithm        string         `json:"algorithm"`
	Digest           string         `json:"digest"`
	Bytes            int64          `json:"bytes"`
	Canonicalization PayloadProfile `json:"canonicalization"`
}

type ComponentRecord struct {
	ComponentID string          `json:"component_id"`
	Name        string          `json:"name"`
	TypeID      string          `json:"type_id"`
	SchemaID    string          `json:"schema_id"`
	Role        Role            `json:"role"`
	Visibility  Visibility      `json:"visibility"`
	MediaType   string          `json:"media_type"`
	Payload     PayloadIdentity `json:"payload"`
	Parents     []ParentRef     `json:"parents"`
}

type CapsuleRef struct {
	Relation  string `json:"relation"`
	CapsuleID string `json:"capsule_id"`
}

type Manifest struct {
	SchemaVersion     string            `json:"schema_version"`
	CanonicalPolicy   string            `json:"canonical_policy"`
	CapsuleID         string            `json:"capsule_id"`
	ManifestDigest    string            `json:"manifest_digest"`
	RegistryDigest    string            `json:"registry_digest"`
	StudyID           string            `json:"study_id"`
	CellID            string            `json:"cell_id"`
	ParentCapsules    []CapsuleRef      `json:"parent_capsules"`
	ScientificRoots   []string          `json:"scientific_roots"`
	PresentationRoots []string          `json:"presentation_roots"`
	Components        []ComponentRecord `json:"components"`
}

type VerificationReport struct {
	SchemaVersion          string         `json:"schema_version"`
	CapsuleID              string         `json:"capsule_id"`
	ManifestDigest         string         `json:"manifest_digest"`
	RegistryDigest         string         `json:"registry_digest"`
	Components             int            `json:"components"`
	ScientificComponents   int            `json:"scientific_components"`
	PresentationComponents int            `json:"presentation_components"`
	VisibilityCounts       map[string]int `json:"visibility_counts"`
	Offline                bool           `json:"offline"`
	Valid                  bool           `json:"valid"`
}

type VerificationLimits struct {
	MaxComponents    int
	MaxManifestBytes int64
	MaxRegistryBytes int64
	MaxPayloadBytes  int64
	MaxTotalBytes    int64
}

func DefaultVerificationLimits() VerificationLimits {
	return VerificationLimits{
		MaxComponents: 100_000, MaxManifestBytes: 64 << 20, MaxRegistryBytes: 16 << 20,
		MaxPayloadBytes: 512 << 20, MaxTotalBytes: 8 << 30,
	}
}

func (l VerificationLimits) Valid() bool {
	return l.MaxComponents > 0 && l.MaxManifestBytes > 0 && l.MaxRegistryBytes > 0 &&
		l.MaxPayloadBytes > 0 && l.MaxTotalBytes >= l.MaxPayloadBytes
}

type VerificationOptions struct {
	MaximumVisibility Visibility
	Limits            VerificationLimits
}

type ArchiveReport struct {
	SchemaVersion string `json:"schema_version"`
	CapsuleID     string `json:"capsule_id"`
	ArchiveRoot   string `json:"archive_root"`
	SHA256        string `json:"sha256"`
	Bytes         int64  `json:"bytes"`
	Files         int    `json:"files"`
	Deterministic bool   `json:"deterministic"`
}
