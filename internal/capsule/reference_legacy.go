package capsule

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	cachekit "github.com/Christopher-Schulze/evalwitness/internal/cache"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	LegacyDerivedResultSchemaVersion = "evalwitness.legacy-derived-result.v1"
	LegacyEvidenceIndexSchemaVersion = "evalwitness.legacy-evidence-index.v1"

	legacyDerivedResultValidatorID = "evalwitness.validator.legacy-derived-result.v1"
	legacyEvidenceIndexValidatorID = "evalwitness.validator.legacy-evidence-index.v1"
	legacyEvidenceIndexBindingID   = "evalwitness.validator.legacy-evidence-index-bindings.v1"
	legacyCacheCensusValidatorID   = "evalwitness.validator.legacy-cache-census.v1"
	legacyEvidenceCeiling          = "E1"
	legacyProvenanceStatus         = "incomplete_legacy"
	legacyCacheCensusPath          = "eval/governance/legacy-response-custody-v1.json"
)

var legacyMissingProvenance = cachekit.LegacyCacheMissingIdentities()

type legacyResultDefinition struct {
	Path      string
	Name      string
	Benchmark string
	Mode      string
}

type LegacyEvidenceEntry struct {
	SourcePath        string   `json:"source_path"`
	ComponentName     string   `json:"component_name"`
	ComponentID       string   `json:"component_id"`
	PayloadSHA256     string   `json:"payload_sha256"`
	Bytes             int64    `json:"bytes"`
	Benchmark         string   `json:"benchmark"`
	Mode              string   `json:"mode"`
	EvidenceCeiling   string   `json:"evidence_ceiling"`
	ProvenanceStatus  string   `json:"provenance_status"`
	MissingProvenance []string `json:"missing_provenance"`
}

type LegacyEvidenceIndex struct {
	SchemaVersion string                `json:"schema_version"`
	Entries       []LegacyEvidenceEntry `json:"entries"`
	Digest        string                `json:"digest"`
}

func referenceLegacyTypes() []ComponentType {
	return []ComponentType{
		{
			TypeID: cachekit.LegacyCacheCensusSchemaVersion, SchemaID: cachekit.LegacyCacheCensusSchemaVersion,
			Role: RoleCommitment, AllowedVisibilities: []Visibility{VisibilityPublic},
			MediaType: "application/json", PayloadProfile: PayloadExactBytes,
			ValidatorID: legacyCacheCensusValidatorID, ParentRules: []ParentRule{},
		},
		{
			TypeID: LegacyDerivedResultSchemaVersion, SchemaID: LegacyDerivedResultSchemaVersion,
			Role: RoleDerivation, AllowedVisibilities: []Visibility{VisibilityPublic},
			MediaType: "application/json", PayloadProfile: PayloadExactBytes,
			ValidatorID: legacyDerivedResultValidatorID, ParentRules: []ParentRule{},
		},
		{
			TypeID: LegacyEvidenceIndexSchemaVersion, SchemaID: LegacyEvidenceIndexSchemaVersion,
			Role: RoleDerivation, AllowedVisibilities: []Visibility{VisibilityPublic},
			MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON,
			ValidatorID: legacyEvidenceIndexValidatorID, BindingValidatorID: legacyEvidenceIndexBindingID,
			ParentRules: []ParentRule{
				parentRule(EdgeDerivedFrom, cachekit.LegacyCacheCensusSchemaVersion, 1, 1),
				parentRule(EdgeDerivedFrom, LegacyDerivedResultSchemaVersion, len(legacyResultDefinitions()), len(legacyResultDefinitions())),
			},
		},
	}
}

func referenceLegacyPayloadValidators() map[string]PayloadValidator {
	return map[string]PayloadValidator{
		legacyCacheCensusValidatorID: func(payload []byte) error {
			_, err := cachekit.DecodeLegacyCacheCensus(payload)
			return err
		},
		legacyDerivedResultValidatorID: validateLegacyDerivedResult,
		legacyEvidenceIndexValidatorID: func(payload []byte) error {
			var index LegacyEvidenceIndex
			if err := decodeReferenceJSON(payload, &index); err != nil {
				return err
			}
			return index.Validate()
		},
	}
}

func referenceLegacyBindingValidators() map[string]BindingValidator {
	return map[string]BindingValidator{legacyEvidenceIndexBindingID: validateLegacyEvidenceIndexBindings}
}

func addReferenceLegacyEvidence(repositoryRoot string, registry *Registry, records *[]ComponentRecord, payloads map[string][]byte) (ComponentRecord, error) {
	definitions := legacyResultDefinitions()
	custody, err := addReferenceFile(
		repositoryRoot,
		registry,
		"legacy.response-custody",
		cachekit.LegacyCacheCensusSchemaVersion,
		VisibilityPublic,
		legacyCacheCensusPath,
		nil,
		records,
		payloads,
	)
	if err != nil {
		return ComponentRecord{}, err
	}
	components := make([]ComponentRecord, 0, len(definitions))
	entries := make([]LegacyEvidenceEntry, 0, len(definitions))
	for _, definition := range definitions {
		record, err := addReferenceFile(
			repositoryRoot,
			registry,
			definition.Name,
			LegacyDerivedResultSchemaVersion,
			VisibilityPublic,
			definition.Path,
			nil,
			records,
			payloads,
		)
		if err != nil {
			return ComponentRecord{}, err
		}
		components = append(components, record)
		entries = append(entries, LegacyEvidenceEntry{
			SourcePath: definition.Path, ComponentName: record.Name, ComponentID: record.ComponentID,
			PayloadSHA256: record.Payload.Digest, Bytes: record.Payload.Bytes,
			Benchmark: definition.Benchmark, Mode: definition.Mode,
			EvidenceCeiling: legacyEvidenceCeiling, ProvenanceStatus: legacyProvenanceStatus,
			MissingProvenance: slices.Clone(legacyMissingProvenance),
		})
	}
	index := LegacyEvidenceIndex{SchemaVersion: LegacyEvidenceIndexSchemaVersion, Entries: entries}
	digest, err := legacyEvidenceIndexDigest(index)
	if err != nil {
		return ComponentRecord{}, err
	}
	index.Digest = digest
	if err := index.Validate(); err != nil {
		return ComponentRecord{}, err
	}
	raw, err := protocol.CanonicalMarshal(index)
	if err != nil {
		return ComponentRecord{}, err
	}
	parents := make([]ParentRef, 0, len(components)+1)
	parents = append(parents, internalParentRef(EdgeDerivedFrom, custody))
	for _, component := range components {
		parents = append(parents, internalParentRef(EdgeDerivedFrom, component))
	}
	return addReferencePayload(registry, ComponentInput{
		Name: "legacy.evidence-index", TypeID: LegacyEvidenceIndexSchemaVersion,
		Visibility: VisibilityPublic, Payload: raw, Parents: parents,
	}, records, payloads)
}

func (index LegacyEvidenceIndex) Validate() error {
	definitions := legacyResultDefinitions()
	if index.SchemaVersion != LegacyEvidenceIndexSchemaVersion || len(index.Entries) != len(definitions) || !validDigest(index.Digest) {
		return errors.New("legacy evidence index identity is invalid")
	}
	for entryIndex, entry := range index.Entries {
		definition := definitions[entryIndex]
		if entry.SourcePath != definition.Path || entry.ComponentName != definition.Name ||
			!validDigest(entry.ComponentID) || entry.PayloadSHA256 == "" || !validDigest(entry.PayloadSHA256) || entry.Bytes < 1 ||
			entry.Benchmark != definition.Benchmark || entry.Mode != definition.Mode ||
			entry.EvidenceCeiling != legacyEvidenceCeiling || entry.ProvenanceStatus != legacyProvenanceStatus ||
			!slices.Equal(entry.MissingProvenance, legacyMissingProvenance) {
			return fmt.Errorf("legacy evidence entry %q is invalid", entry.SourcePath)
		}
	}
	digest, err := legacyEvidenceIndexDigest(index)
	if err != nil || digest != index.Digest {
		return errors.New("legacy evidence index digest is invalid")
	}
	return nil
}

func validateLegacyDerivedResult(payload []byte) error {
	if len(payload) == 0 || len(payload) > maximumReferenceDocumentBytes {
		return errors.New("legacy result is empty or exceeds its size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var document map[string]json.RawMessage
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode legacy result: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("legacy result contains trailing data")
	}
	for _, field := range []string{"tasks", "trials", "verifier_score", "oracle_score", "pass_at_1_score", "route", "details"} {
		if len(document[field]) == 0 {
			return fmt.Errorf("legacy result is missing required field %q", field)
		}
	}
	if len(document["schema_version"]) != 0 {
		return errors.New("legacy result unexpectedly declares a modern schema version")
	}
	return nil
}

func validateLegacyEvidenceIndexBindings(context BindingContext) error {
	var index LegacyEvidenceIndex
	if err := decodeReferenceJSON(context.Payload, &index); err != nil {
		return err
	}
	if err := index.Validate(); err != nil {
		return err
	}
	if len(context.Parents) != len(index.Entries)+1 {
		return errors.New("legacy evidence index and capsule graph have different result counts")
	}
	parents := make(map[string]BoundParent, len(context.Parents))
	var custody cachekit.LegacyCacheCensus
	custodyCount := 0
	for _, parent := range context.Parents {
		if parent.Reference.Resolution != ParentInternal {
			return errors.New("legacy evidence index requires internal parents")
		}
		switch parent.Record.TypeID {
		case cachekit.LegacyCacheCensusSchemaVersion:
			var err error
			custody, err = cachekit.DecodeLegacyCacheCensus(parent.Payload)
			if err != nil {
				return err
			}
			custodyCount++
		case LegacyDerivedResultSchemaVersion:
			parents[parent.Record.Name] = parent
		default:
			return errors.New("legacy evidence index has an unsupported parent type")
		}
	}
	if custodyCount != 1 || custody.ExactAdmissibleEntries != 0 || custody.LegacyOnlyEntries != custody.ResponseFiles ||
		!slices.Equal(custody.MissingIdentities, legacyMissingProvenance) {
		return errors.New("legacy response custody does not preserve the schema-1 evidence ceiling")
	}
	for _, entry := range index.Entries {
		parent, found := parents[entry.ComponentName]
		if !found || entry.ComponentID != parent.Record.ComponentID || entry.PayloadSHA256 != parent.Record.Payload.Digest || entry.Bytes != parent.Record.Payload.Bytes {
			return fmt.Errorf("legacy evidence entry %q differs from its capsule parent", entry.SourcePath)
		}
		result, err := decodeLegacyBenchmarkResult(parent.Payload)
		if err != nil || result.Route.Provider != custody.PublishedNamespace.ProviderID {
			return fmt.Errorf("legacy evidence entry %q is outside the committed published namespace", entry.SourcePath)
		}
	}
	return nil
}

func legacyEvidenceIndexDigest(index LegacyEvidenceIndex) (string, error) {
	index.Digest = ""
	return protocol.Digest(index)
}

func legacyResultDefinitions() []legacyResultDefinition {
	definitions := []legacyResultDefinition{
		legacyDefinition("swebench-all-pairs", "swebench", "all-pairs"),
		legacyDefinition("swebench-judge", "swebench", "judge"),
		legacyDefinition("swebench-paper-parity", "swebench", "paper-parity"),
		legacyDefinition("swebench-size-agnostic", "swebench", "size-agnostic"),
		legacyDefinition("swebench-verifier", "swebench", "verifier"),
		legacyDefinition("terminal-bench-absolute", "terminal-bench", "absolute"),
		legacyDefinition("terminal-bench-all-pairs", "terminal-bench", "all-pairs"),
		legacyDefinition("terminal-bench-evidence-16k", "terminal-bench", "evidence-16k"),
		legacyDefinition("terminal-bench-judge", "terminal-bench", "judge"),
		legacyDefinition("terminal-bench-paper-parity", "terminal-bench", "paper-parity"),
		legacyDefinition("terminal-bench-verifier", "terminal-bench", "verifier"),
	}
	slices.SortFunc(definitions, func(left, right legacyResultDefinition) int { return strings.Compare(left.Path, right.Path) })
	return definitions
}

func legacyDefinition(base, benchmark, mode string) legacyResultDefinition {
	return legacyResultDefinition{
		Path:      filepath.ToSlash(filepath.Join("eval", "results", base+".json")),
		Name:      "legacy.result." + base,
		Benchmark: benchmark,
		Mode:      mode,
	}
}
