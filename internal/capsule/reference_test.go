package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestReferencePackageBindsCompleteEvidenceAndProvenanceDAG(t *testing.T) {
	pack := buildReferencePackage(t)
	requiredTypes := []string{
		ReferenceIndexSchemaVersion,
		LegacyDerivedResultSchemaVersion, LegacyEvidenceIndexSchemaVersion, LegacyClaimFactsSchemaVersion,
		SourceTreeProvenanceSchemaVersion, BuildProvenanceSchemaVersion, DatasetProvenanceSchemaVersion,
		RouteProvenanceSchemaVersion, StudyProvenanceSchemaVersion, AnalysisProvenanceSchemaVersion,
		ClockProvenanceSchemaVersion, RenderReceiptSchemaVersion,
		protocol.RunSchema, protocol.VectorCorpusSchema, preprocess.CanonicalTrajectorySchema,
		mutation.ConstructRepairEvidenceSchemaVersion, mutation.ConstructChallengeSchemaVersion,
		relation.ScarcityPublicEvidenceSchemaVersion, relation.OwnerInspectionPublicAttestationSchemaVersion,
		lineage.PlanSchemaVersion, lineage.SourceSchemaVersion, lineage.WitnessSchemaVersion,
		lineage.CandidateSchemaVersion, lineage.AssessmentSchemaVersion, lineage.CapabilitySchemaVersion,
		lineage.AuditSchemaVersion, lineage.BOMSchemaVersion, lineage.DatasetCardSchemaVersion, lineage.ReleaseSchemaVersion,
	}
	document := pack.Registry.Document()
	if len(document.Types) != 51 {
		t.Fatalf("reference registry type count = %d, want 51", len(document.Types))
	}
	for _, typeID := range requiredTypes {
		if _, found := pack.Registry.Lookup(typeID); !found {
			t.Fatalf("reference registry omits required type %q", typeID)
		}
	}
	if len(pack.Manifest.Components) != 71 || len(pack.Payloads) != 71 {
		t.Fatalf("reference package has %d components and %d payloads, want 71 and 71", len(pack.Manifest.Components), len(pack.Payloads))
	}
	if err := pack.Manifest.Validate(pack.Registry); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "reference-capsule")
	if err := WriteDirectory(context.Background(), root, pack.Registry, pack.Manifest, pack.Payloads); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyDirectory(context.Background(), root, pack.Registry, VerificationOptions{MaximumVisibility: VisibilityPublic})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || !report.Offline || report.Components != 71 || report.ScientificComponents != 69 || report.PresentationComponents != 2 {
		t.Fatalf("reference verification report = %+v", report)
	}
	if report.CapsuleID != pack.Manifest.CapsuleID || report.ManifestDigest != pack.Manifest.ManifestDigest || report.RegistryDigest != pack.Registry.Digest() {
		t.Fatal("reference verification report differs from the sealed package")
	}
	second, err := BuildReferencePackage(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if second.Manifest.CapsuleID != pack.Manifest.CapsuleID || second.Manifest.ManifestDigest != pack.Manifest.ManifestDigest || second.Registry.Digest() != pack.Registry.Digest() {
		t.Fatal("reference package identity changed without a source, binary, evidence, or presentation change")
	}
}

func TestReferenceProvenanceRecordsHonestUnavailableBoundaries(t *testing.T) {
	pack := buildReferencePackage(t)
	_, buildRaw := referenceComponent(t, pack, BuildProvenanceSchemaVersion)
	var build BuildProvenance
	if err := json.Unmarshal(buildRaw, &build); err != nil {
		t.Fatal(err)
	}
	if !validDigest(build.BinaryDigest) || !validDigest(build.SourceTreeDigest) || build.BinaryIncluded {
		t.Fatalf("reference build provenance = %+v", build)
	}

	_, datasetRaw := referenceComponent(t, pack, DatasetProvenanceSchemaVersion)
	var dataset DatasetProvenance
	if err := json.Unmarshal(datasetRaw, &dataset); err != nil {
		t.Fatal(err)
	}
	if dataset.AcquisitionChecksum == dataset.CanonicalSourceDigest || dataset.LabelStatus != AvailabilityNotRun || len(dataset.LabelDigests) != 0 {
		t.Fatalf("reference dataset provenance collapsed exact bytes or invented labels: %+v", dataset)
	}

	_, routeRaw := referenceComponent(t, pack, RouteProvenanceSchemaVersion)
	var route RouteProvenance
	if err := json.Unmarshal(routeRaw, &route); err != nil {
		t.Fatal(err)
	}
	if route.RequestStatus != AvailabilityFixtureOnly || route.ResponseStatus != AvailabilityUnavailablePublicSource ||
		route.AttestationStatus != AvailabilityUnavailablePublicSource || route.ProviderCallsStatus != AvailabilityNotRun ||
		len(route.ServedIdentities) != 0 || len(route.CheckpointAssertions) != 0 {
		t.Fatalf("reference route provenance promoted unavailable observations: %+v", route)
	}

	_, clocksRaw := referenceComponent(t, pack, ClockProvenanceSchemaVersion)
	var clocks ClockProvenance
	if err := json.Unmarshal(clocksRaw, &clocks); err != nil {
		t.Fatal(err)
	}
	if clocks.Events[1].Kind != "collection" || clocks.Events[1].Status != "not_run" ||
		clocks.Events[2].Kind != "replay" || clocks.Events[2].Status != "completed_time_unavailable" {
		t.Fatalf("reference clocks collapsed collection and replay: %+v", clocks.Events)
	}
}

func TestReferencePublicPackageExcludesPrivateRelationIdentityAndPaths(t *testing.T) {
	pack := buildReferencePackage(t)
	for _, typeID := range []string{
		PrivateRelationPackageFileSchemaVersion,
		PrivateRelationPackageReceiptSchemaVersion,
		PrivateRelationEventChainSchemaVersion,
		PrivateRelationSourceCommitmentSchemaVersion,
		PrivateRelationProofSchemaVersion,
		relation.PilotInspectionSessionSchemaVersion,
		relation.PilotInspectionCompletionSchemaVersion,
	} {
		if _, found := pack.Registry.Lookup(typeID); found {
			t.Fatalf("public registry exposes private relation type %q", typeID)
		}
	}
	for _, record := range pack.Manifest.Components {
		if record.Visibility != VisibilityPublic {
			t.Fatalf("public reference component %q has visibility %q", record.Name, record.Visibility)
		}
		raw := pack.Payloads[record.Payload.Digest]
		for _, forbidden := range [][]byte{
			[]byte("/Users/"), []byte("eval/results/private"), []byte("pilot-inspections"),
			[]byte(`"inspector_alias"`), []byte(`"session_digest"`),
			[]byte(`"inspection_record_digest"`), []byte(`"completion_digest"`),
		} {
			if bytes.Contains(raw, forbidden) {
				t.Fatalf("public component %q contains forbidden private fragment %q", record.Name, forbidden)
			}
		}
	}
	_, attestationRaw := referenceComponent(t, pack, relation.OwnerInspectionPublicAttestationSchemaVersion)
	attestation, err := relation.DecodeOwnerInspectionPublicAttestation(bytes.NewReader(attestationRaw))
	if err != nil {
		t.Fatal(err)
	}
	if !attestation.Disclosure.PrivateChainVerified || attestation.Disclosure.PrivateJournalIdentitiesDisclosed ||
		attestation.Disclosure.RestrictedEvidenceDisclosed || attestation.HumanStudyStatus != "not_run" ||
		attestation.ExternalActionStatus != relation.ExternalActionNotAuthorized {
		t.Fatalf("public owner-inspection disclosure boundary = %+v", attestation)
	}
}

func TestReferenceLegacyEvidenceCannotMasqueradeAsCompleteProvenance(t *testing.T) {
	pack := buildReferencePackage(t)
	_, raw := referenceComponent(t, pack, LegacyEvidenceIndexSchemaVersion)
	var index LegacyEvidenceIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	if len(index.Entries) != 11 {
		t.Fatalf("legacy evidence count = %d, want 11", len(index.Entries))
	}
	for _, entry := range index.Entries {
		if entry.EvidenceCeiling != "E1" || entry.ProvenanceStatus != "incomplete_legacy" || !slices.Equal(entry.MissingProvenance, legacyMissingProvenance) {
			t.Fatalf("legacy entry %q promoted incomplete evidence: %+v", entry.SourcePath, entry)
		}
	}
	_, factsRaw := referenceComponent(t, pack, LegacyClaimFactsSchemaVersion)
	var facts LegacyClaimFacts
	if err := json.Unmarshal(factsRaw, &facts); err != nil {
		t.Fatal(err)
	}
	if facts.PairDecisionsTotal != 240 || facts.PairTiesTotal != 0 || facts.VerifierCallsTotal != 303 ||
		facts.VerifierScoresTotal != "463" || facts.RandomExpectedScoresTotal != "906266666666667/2000000000000" {
		t.Fatalf("legacy claim facts = %+v", facts)
	}
}

func TestReferenceRegistryRejectsLineageWeakeningAndSubstitution(t *testing.T) {
	pack := buildReferencePackage(t)
	bomRecord, bomPayload := referenceComponent(t, pack, lineage.BOMSchemaVersion)
	var bom lineage.VerificationEvidenceBOM
	if err := json.Unmarshal(bomPayload, &bom); err != nil {
		t.Fatal(err)
	}
	if bom.Retention.RequestFingerprint == bom.Retention.CanonicalRequestLineageDigest {
		t.Fatal("reference BOM collapsed request bytes and study-lineage identities")
	}
	bom.Retention.TruncatedRequiredFields = []string{"exit_status"}
	bom.Header.Digest = lineageTestDigest(t, bom)
	mutatedBOM, err := lineage.EncodeIndented(bom)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := BuildComponent(pack.Registry, ComponentInput{
		Name: bomRecord.Name, TypeID: bomRecord.TypeID, Visibility: bomRecord.Visibility,
		Payload: mutatedBOM, Parents: bomRecord.Parents,
	}); err == nil {
		t.Fatal("reference registry accepted decisive-field truncation with a recomputed artifact digest")
	}

	misboundBOM := bom
	misboundBOM.Retention.TruncatedRequiredFields = nil
	misboundBOM.SourceDigest = strings.Repeat("1", 64)
	for index := range misboundBOM.Header.Parents {
		if misboundBOM.Header.Parents[index].Relation == "source" {
			misboundBOM.Header.Parents[index].Digest = misboundBOM.SourceDigest
		}
	}
	misboundBOM.Header.Digest = lineageTestDigest(t, misboundBOM)
	misboundPayload, err := lineage.EncodeIndented(misboundBOM)
	if err != nil {
		t.Fatal(err)
	}
	misboundRecord, normalized, err := BuildComponent(pack.Registry, ComponentInput{
		Name: bomRecord.Name, TypeID: bomRecord.TypeID, Visibility: bomRecord.Visibility,
		Payload: misboundPayload, Parents: bomRecord.Parents,
	})
	if err != nil {
		t.Fatalf("self-consistent substitution fixture did not reach the capsule binding guard: %v", err)
	}
	if err := pack.Registry.ValidateBindings(BindingContext{
		Component: misboundRecord, Payload: normalized, Parents: referenceBoundParents(t, pack, bomRecord),
	}); err == nil {
		t.Fatal("reference registry accepted a self-consistent lineage payload bound to the wrong capsule parent")
	}

	sourceRecord, sourcePayload := referenceComponent(t, pack, lineage.SourceSchemaVersion)
	var source lineage.VerificationLineageSource
	if err := json.Unmarshal(sourcePayload, &source); err != nil {
		t.Fatal(err)
	}
	source.Header.TaskID = "TASK-999"
	source.Header.Digest = lineageTestDigest(t, source)
	mutatedSource, err := lineage.EncodeIndented(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := BuildComponent(pack.Registry, ComponentInput{
		Name: sourceRecord.Name, TypeID: sourceRecord.TypeID, Visibility: sourceRecord.Visibility,
		Payload: mutatedSource, Parents: sourceRecord.Parents,
	}); err == nil {
		t.Fatal("reference registry accepted cross-task lineage substitution with a recomputed artifact digest")
	}
}

func referenceBoundParents(t *testing.T, pack ReferencePackage, child ComponentRecord) []BoundParent {
	t.Helper()
	byID := make(map[string]ComponentRecord, len(pack.Manifest.Components))
	for _, record := range pack.Manifest.Components {
		byID[record.ComponentID] = record
	}
	parents := make([]BoundParent, 0, len(child.Parents))
	for _, reference := range child.Parents {
		record, found := byID[reference.ComponentID]
		if !found {
			t.Fatalf("reference parent %q is missing", reference.ComponentID)
		}
		parents = append(parents, BoundParent{
			Reference: reference, Record: record, Payload: pack.Payloads[record.Payload.Digest],
		})
	}
	return parents
}

var (
	referencePackageOnce sync.Once
	referencePackage     ReferencePackage
	referencePackageErr  error
)

func buildReferencePackage(t *testing.T) ReferencePackage {
	t.Helper()
	referencePackageOnce.Do(func() {
		referencePackage, referencePackageErr = BuildReferencePackage(filepath.Join("..", ".."))
	})
	if referencePackageErr != nil {
		t.Fatal(referencePackageErr)
	}
	return cloneReferencePackage(referencePackage)
}

func cloneReferencePackage(source ReferencePackage) ReferencePackage {
	cloned := source
	cloned.Manifest.ParentCapsules = slices.Clone(source.Manifest.ParentCapsules)
	cloned.Manifest.ScientificRoots = slices.Clone(source.Manifest.ScientificRoots)
	cloned.Manifest.PresentationRoots = slices.Clone(source.Manifest.PresentationRoots)
	cloned.Manifest.Components = slices.Clone(source.Manifest.Components)
	for index := range cloned.Manifest.Components {
		cloned.Manifest.Components[index].Parents = slices.Clone(source.Manifest.Components[index].Parents)
	}
	cloned.Payloads = make(map[string][]byte, len(source.Payloads))
	for digest, raw := range source.Payloads {
		cloned.Payloads[digest] = slices.Clone(raw)
	}
	return cloned
}

func referenceComponent(t *testing.T, pack ReferencePackage, typeID string) (ComponentRecord, []byte) {
	t.Helper()
	for _, record := range pack.Manifest.Components {
		if record.TypeID == typeID {
			return record, pack.Payloads[record.Payload.Digest]
		}
	}
	t.Fatalf("reference package has no component of type %q", typeID)
	return ComponentRecord{}, nil
}

func lineageTestDigest(t *testing.T, value any) string {
	t.Helper()
	switch typed := value.(type) {
	case lineage.VerificationEvidenceBOM:
		typed.Header.Digest = ""
		value = typed
	case lineage.VerificationLineageSource:
		typed.Header.Digest = ""
		value = typed
	default:
		t.Fatalf("unsupported lineage test artifact %T", value)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
