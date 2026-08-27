package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestResponseBundleBuildVerifyAndReplay(t *testing.T) {
	capturePath := buildResponseBundleCapture(t)
	sources := map[string]string{"primary": capturePath}
	policy, evidence := responseBundlePolicyForTest(t, sources)
	redistributionEvidence := responseBundleRedistributionEvidenceForTest()
	first, err := BuildResponseBundle(policy, evidence, redistributionEvidence, sources, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildResponseBundle(policy, evidence, redistributionEvidence, sources, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Package.Manifest.CapsuleID != second.Package.Manifest.CapsuleID ||
		first.Package.Manifest.ManifestDigest != second.Package.Manifest.ManifestDigest ||
		first.Index.Digest != second.Index.Digest {
		t.Fatal("identical captures did not produce one deterministic response bundle identity")
	}
	if first.Index.TotalEntries != 2 || first.Index.Captures[0].LogProbabilityEntries != 2 ||
		first.Index.Captures[0].JudgeEntries != 0 || first.PublicScan.Files != 9 {
		t.Fatalf("response bundle census = %+v scan=%+v", first.Index, first.PublicScan)
	}

	bundlePath := filepath.Join(t.TempDir(), "bundle")
	if err := capsule.WriteDirectory(
		context.Background(), bundlePath, first.Package.Registry, first.Package.Manifest, first.Package.Payloads,
	); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyResponseBundleDirectory(context.Background(), bundlePath, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || !report.ExactReplay || report.NetworkRequired || report.ProviderCalls != 0 ||
		report.TotalEntries != 2 || len(report.Captures) != 1 || report.PublicScan.Findings == nil ||
		report.LineagePolicy != ResponseBundleLineageExactFixture ||
		report.EvidenceCeiling != ResponseBundleEvidenceMechanismConformance {
		t.Fatalf("response bundle verification report = %+v", report)
	}
	if report.RedistributionEvidenceSHA256 != responseBundleRedistributionEvidenceDigestForTest(t) {
		t.Fatalf("redistribution evidence digest = %q", report.RedistributionEvidenceSHA256)
	}
	if report.DatasetID != policy.Dataset.ID || report.RequestCorpusDigest != policy.Dataset.Digest {
		t.Fatalf("verified request corpus = %q/%q", report.DatasetID, report.RequestCorpusDigest)
	}
	if report.StudyID != policy.StudyID || report.CellID != policy.CellID ||
		report.CaptureSetDigest != first.Index.CaptureSetDigest ||
		report.Captures[0].ComponentID != first.Index.Captures[0].ComponentID {
		t.Fatalf("verified response-bundle identities = %+v", report)
	}

	replayPath := filepath.Join(bundlePath, filepath.FromSlash(report.Captures[0].PayloadPath))
	providerReplay, err := LoadReplay(replayPath, "inner-provider", "bundle-model", provider.Capabilities{})
	if err != nil {
		t.Fatal(err)
	}
	request := exactRequest(t, "inner-provider", "bundle-model", "bundle request one")
	if _, err := providerReplay.Score(context.Background(), request); err != nil {
		t.Fatalf("verified bundle did not replay its exact response: %v", err)
	}
}

func TestResponseBundleRejectsLegacyNonCanonicalAndUnsafeCaptures(t *testing.T) {
	validCapture := buildResponseBundleCapture(t)
	policy, evidence := responseBundlePolicyForTest(t, map[string]string{"primary": validCapture})
	redistributionEvidence := responseBundleRedistributionEvidenceForTest()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "legacy", content: `{"schema_version":1}` + "\n", want: "legacy or unsupported schema"},
		{name: "non-canonical", content: strings.TrimSuffix(readCaptureForTest(t, buildResponseBundleCapture(t)), "\n") + " \n", want: "deterministic JSON form"},
		{name: "private-path", content: readCaptureForTest(t, buildResponseBundleCaptureWithPrompt(t, "/Users/alice/private/task")), want: "public scan"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "capture.jsonl")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			testPolicy := policy
			testEvidence := evidence
			if test.name == "private-path" {
				testPolicy, testEvidence = responseBundlePolicyForTest(t, map[string]string{"primary": path})
			}
			_, err := BuildResponseBundle(testPolicy, testEvidence, redistributionEvidence, map[string]string{"primary": path}, nil, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("bundle error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestResponseBundleRejectsPolicyDriftAndPayloadCorruption(t *testing.T) {
	capturePath := buildResponseBundleCapture(t)
	sources := map[string]string{"primary": capturePath}
	policy, evidence := responseBundlePolicyForTest(t, sources)
	redistributionEvidence := responseBundleRedistributionEvidenceForTest()
	if _, err := BuildResponseBundle(policy, evidence, redistributionEvidence, map[string]string{"different": capturePath}, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "differ from the policy capture set") {
		t.Fatalf("capture-set mismatch error = %v", err)
	}
	driftedPolicy := policy
	driftedPolicy.Dataset.Digest = strings.Repeat("f", 64)
	driftedPolicy, err := SealResponseBundlePolicy(driftedPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildResponseBundle(driftedPolicy, evidence, redistributionEvidence, sources, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "exact request corpus differs") {
		t.Fatalf("request-corpus mismatch error = %v", err)
	}
	wrongRedistributionEvidence := filepath.Join(t.TempDir(), "wrong-license.txt")
	if err := os.WriteFile(wrongRedistributionEvidence, []byte("not the sealed license\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildResponseBundle(policy, evidence, wrongRedistributionEvidence, sources, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "redistribution evidence differs") {
		t.Fatalf("redistribution-evidence mismatch error = %v", err)
	}
	build, err := BuildResponseBundle(policy, evidence, redistributionEvidence, map[string]string{"primary": capturePath}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := make(map[string][]byte, len(build.Package.Payloads))
	for digest, payload := range build.Package.Payloads {
		corrupted[digest] = slices.Clone(payload)
	}
	captureDigest := build.Index.Captures[0].PayloadSHA256
	corrupted[captureDigest][0] ^= 1
	if _, err := capsule.VerifyPackage(
		context.Background(), build.Package.Registry, build.Package.Manifest, corrupted,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
	); err == nil {
		t.Fatal("response bundle accepted a byte-corrupted exact capture")
	}
}

func TestResponseBundleResearchLineagePolicy(t *testing.T) {
	cleanEvidence := responseBundleCleanProducerEvidenceForTest(t)
	incompleteCapture := buildResponseBundleCapture(t)
	incompleteSources := map[string]string{"primary": incompleteCapture}
	incompletePolicy, _ := responseBundlePolicyForTest(t, incompleteSources)
	incompletePolicy.LineagePolicy = ResponseBundleLineageCompleteResearch
	incompletePolicy.Producer = cleanEvidence.Producer()
	incompletePolicy, err := SealResponseBundlePolicy(incompletePolicy)
	if err != nil {
		t.Fatal(err)
	}
	redistributionEvidence := responseBundleRedistributionEvidenceForTest()
	if _, err := BuildResponseBundle(incompletePolicy, cleanEvidence, redistributionEvidence, incompleteSources, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "complete research evidence is required for every entry") {
		t.Fatalf("incomplete research-lineage error = %v", err)
	}

	completeCapture := buildResponseBundleResearchCapture(t)
	completeSources := map[string]string{"primary": completeCapture}
	completePolicy, dirtyEvidence := responseBundlePolicyForTest(t, completeSources)
	completePolicy.LineagePolicy = ResponseBundleLineageCompleteResearch
	completePolicy.Producer = cleanEvidence.Producer()
	completePolicy, err = SealResponseBundlePolicy(completePolicy)
	if err != nil {
		t.Fatal(err)
	}
	build, err := BuildResponseBundle(completePolicy, cleanEvidence, redistributionEvidence, completeSources, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	inspection := build.Index.Captures[0].CaptureInspection
	if inspection.CompleteResearchEntries != inspection.Entries ||
		!slices.Equal(inspection.StudyCellIDs, []string{"exact-replay"}) {
		t.Fatalf("complete research-lineage inspection = %+v", inspection)
	}
	bundlePath := filepath.Join(t.TempDir(), "complete-research-bundle")
	if err := capsule.WriteDirectory(
		context.Background(), bundlePath, build.Package.Registry, build.Package.Manifest, build.Package.Payloads,
	); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyResponseBundleDirectory(context.Background(), bundlePath, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.EvidenceCeiling != ResponseBundleEvidenceExternalUnresolved {
		t.Fatalf("complete research evidence ceiling = %q", report.EvidenceCeiling)
	}
	dirtyPolicy := completePolicy
	dirtyPolicy.Producer = dirtyEvidence.Producer()
	dirtyPolicy, err = SealResponseBundlePolicy(dirtyPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildResponseBundle(dirtyPolicy, dirtyEvidence, redistributionEvidence, completeSources, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "clean committed source tree") {
		t.Fatalf("dirty research-producer error = %v", err)
	}
}

func TestResponseBundleRejectsManifestAndCaptureGovernanceSubstitution(t *testing.T) {
	capturePath := buildResponseBundleCapture(t)
	sources := map[string]string{"primary": capturePath}
	policy, evidence := responseBundlePolicyForTest(t, sources)
	build, err := BuildResponseBundle(policy, evidence, responseBundleRedistributionEvidenceForTest(), sources, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	mismatchedManifest, err := capsule.BuildManifest(build.Package.Registry, capsule.ManifestInput{
		StudyID: build.Package.Manifest.StudyID, CellID: "substituted-cell",
		ParentCapsules:    build.Package.Manifest.ParentCapsules,
		ScientificRoots:   build.Package.Manifest.ScientificRoots,
		PresentationRoots: build.Package.Manifest.PresentationRoots,
		Components:        build.Package.Manifest.Components,
	})
	if err != nil {
		t.Fatal(err)
	}
	mismatchedPath := filepath.Join(t.TempDir(), "mismatched-manifest")
	if err := capsule.WriteDirectory(
		context.Background(), mismatchedPath, build.Package.Registry, mismatchedManifest, build.Package.Payloads,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyResponseBundleDirectory(context.Background(), mismatchedPath, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "manifest study or cell differs") {
		t.Fatalf("manifest substitution error = %v", err)
	}

	indexContext := responseBundleBindingContextForTest(t, build, ResponseBundleIndexSchemaVersion)
	for parentIndex := range indexContext.Parents {
		if indexContext.Parents[parentIndex].Record.TypeID == ResponseCaptureComponentSchemaVersion {
			indexContext.Parents[parentIndex].Record.Parents[0].ComponentID = strings.Repeat("f", 64)
		}
	}
	if err := validateResponseBundleIndexBindings(indexContext); err == nil ||
		!strings.Contains(err.Error(), "not governed by its index policy") {
		t.Fatalf("capture governance substitution error = %v", err)
	}
}

func TestResponseBundleRequiresOneSingularComponentPerType(t *testing.T) {
	records := []capsule.ComponentRecord{{TypeID: ResponseBundlePolicySchemaVersion}, {TypeID: ResponseBundlePolicySchemaVersion}}
	if _, err := responseComponentByType(records, ResponseBundlePolicySchemaVersion); err == nil ||
		!strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("duplicate singular-component error = %v", err)
	}
}

func TestResponseBundleRejectsCrossCaptureRequestDuplication(t *testing.T) {
	capturePath := buildResponseBundleCapture(t)
	inspected, err := inspectResponseBundleSources(
		[]string{"first", "second"},
		map[string]string{"first": capturePath, "second": capturePath},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := responseBundleRequestCorpusDigest(inspected); err == nil ||
		!strings.Contains(err.Error(), "repeat request and sampling slot") {
		t.Fatalf("cross-capture duplicate error = %v", err)
	}
}

func TestResponseBundlePolicyDraftSealsExactlyOnce(t *testing.T) {
	sources := map[string]string{"primary": buildResponseBundleCapture(t)}
	policy, evidence := responseBundlePolicyForTest(t, sources)
	redistributionEvidence := responseBundleRedistributionEvidenceForTest()
	policy.Digest = ""
	policy.Producer = ResponseBundleProducer{}
	policy.Dataset.Digest = ""
	draft, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := SealResponseBundlePolicyDraft(draft, evidence, redistributionEvidence, sources)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeResponseBundlePolicy(sealed)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeResponseBundlePolicy(append(encoded, '\n'))
	if err != nil || decoded.Digest != sealed.Digest {
		t.Fatalf("sealed policy did not survive canonical decode: %+v, %v", decoded, err)
	}
	if _, err := SealResponseBundlePolicyDraft(encoded, evidence, redistributionEvidence, sources); err == nil || !strings.Contains(err.Error(), "already has sealed producer, request-corpus, or policy identity") {
		t.Fatalf("sealed policy was accepted as an unsealed draft: %v", err)
	}
	if bytes.Contains(encoded, []byte("\n")) {
		t.Fatal("canonical sealed policy unexpectedly contains formatting whitespace")
	}
}

func buildResponseBundleCapture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capture.jsonl")
	inner := &scriptedInner{}
	capture, err := WrapCapture(inner, "bundle-model", path, false)
	if err != nil {
		t.Fatal(err)
	}
	first := exactRequest(t, inner.Name(), "bundle-model", "bundle request one")
	second := exactRequest(t, inner.Name(), "bundle-model", "bundle request two")
	second.Lineage.SamplingSlot = "criterion@r1"
	if _, err := capture.Score(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Score(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func buildResponseBundleCaptureWithPrompt(t *testing.T, prompt string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capture.jsonl")
	inner := &scriptedInner{}
	capture, err := WrapCapture(inner, "bundle-model", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Score(context.Background(), exactRequest(t, inner.Name(), "bundle-model", prompt)); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func buildResponseBundleResearchCapture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capture.jsonl")
	inner := &scriptedInner{}
	capture, err := WrapCapture(inner, "bundle-model", path, false)
	if err != nil {
		t.Fatal(err)
	}
	request := exactRequest(t, inner.Name(), "bundle-model", "research bundle request")
	request.EvidenceBindings = []provider.EvidenceBinding{{
		InputSlot: "trajectory_0", SourceDigest: strings.Repeat("d", 64),
		CanonicalDigest: strings.Repeat("e", 64), IngestionDigest: strings.Repeat("f", 64),
		TraceEnvelopeDigest: strings.Repeat("1", 64), MappingReportDigest: strings.Repeat("2", 64),
		MappingPolicyVersion: "evalwitness.trace-mapping.v1",
	}}
	request.Lineage = provider.RequestLineage{
		CriterionID: "criterion", SamplingSlot: "criterion@r0", Entrypoint: "verify",
		AuditCaseID: "case-001", SourceTraceHash: strings.Repeat("a", 64),
		TraceMapHash: strings.Repeat("b", 64), MutationID: "unmodified",
		StudyCellID: "exact-replay", PolicyHash: strings.Repeat("c", 64),
	}
	if err := request.Lineage.ValidateResearch(); err != nil {
		t.Fatal(err)
	}
	inner.reply = func(request provider.RequestEnvelope) (provider.ResponseRecord, error) {
		response, err := exactResponse(request)
		if err != nil {
			return provider.ResponseRecord{}, err
		}
		response.CapabilityAttestationID = "att-" + strings.Repeat("3", 64)
		response.CheckpointAssertion = "served-alias-only"
		return provider.FinalizeResponse(request, response)
	}
	if _, err := capture.Score(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func responseBundlePolicyForTest(t *testing.T, sources map[string]string) (ResponseBundlePolicy, ResponseBundleProducerEvidence) {
	t.Helper()
	evidence := responseBundleProducerEvidenceForTest(t)
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	slices.Sort(names)
	inspected, err := inspectResponseBundleSources(names, sources)
	if err != nil {
		t.Fatal(err)
	}
	requestCorpusDigest, err := responseBundleRequestCorpusDigest(inspected)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := SealResponseBundlePolicy(ResponseBundlePolicy{
		SchemaVersion: ResponseBundlePolicySchemaVersion,
		StudyID:       "response-bundle-test", CellID: "exact-replay",
		LineagePolicy: ResponseBundleLineageExactFixture,
		Producer:      evidence.Producer(),
		Dataset: ResponseBundleDataset{
			ID: "synthetic-test", Digest: requestCorpusDigest, LicenseExpression: "CC0-1.0",
		},
		ResponseLicenseExpression: "CC0-1.0", RedistributionStatus: "authorized",
		RedistributionBasis: "synthetic-test-owner", RedistributionEvidenceDigest: responseBundleRedistributionEvidenceDigestForTest(t),
		AllowedCaptureNames: slices.Clone(names), OmittedClasses: slices.Clone(responseBundleOmittedClasses),
		ExactCaptureOnly: true, PublicArtifactScanRequired: true, ProviderCalls: 0, NetworkRequired: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return policy, evidence
}

func responseBundleRedistributionEvidenceForTest() string {
	return filepath.Join("..", "..", "LICENSE")
}

func responseBundleRedistributionEvidenceDigestForTest(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(responseBundleRedistributionEvidenceForTest())
	if err != nil {
		t.Fatal(err)
	}
	return protocol.DigestBytes(raw)
}

var (
	responseBundleEvidenceOnce sync.Once
	responseBundleEvidence     ResponseBundleProducerEvidence
	responseBundleEvidenceErr  error
)

func responseBundleProducerEvidenceForTest(t *testing.T) ResponseBundleProducerEvidence {
	t.Helper()
	responseBundleEvidenceOnce.Do(func() {
		executable, err := os.Executable()
		if err != nil {
			responseBundleEvidenceErr = err
			return
		}
		responseBundleEvidence, responseBundleEvidenceErr = CollectResponseBundleProducerEvidence(
			context.Background(), filepath.Join("..", ".."), executable,
		)
	})
	if responseBundleEvidenceErr != nil {
		t.Fatal(responseBundleEvidenceErr)
	}
	return responseBundleEvidence
}

func responseBundleCleanProducerEvidenceForTest(t *testing.T) ResponseBundleProducerEvidence {
	t.Helper()
	ctx := context.Background()
	repository := t.TempDir()
	for path, content := range map[string]string{
		"go.mod":  "module example.com/responsebundleproducer\n\ngo 1.23\n",
		"main.go": "package main\n\nfunc main() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(repository, path), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range [][]string{
		{"git", "init", "--quiet"},
		{"git", "config", "user.name", "EvalWitness Test"},
		{"git", "config", "user.email", "evalwitness-test@example.invalid"},
		{"git", "add", "go.mod", "main.go"},
		{"git", "commit", "--quiet", "-m", "fixture"},
	} {
		responseBundleRunCommandForTest(t, ctx, repository, command[0], command[1:]...)
	}
	binary := filepath.Join(t.TempDir(), "producer")
	responseBundleRunCommandForTest(t, ctx, repository, "go", "build", "-o", binary, ".")
	evidence, err := CollectResponseBundleProducerEvidence(ctx, repository, binary)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.SourceTree.Dirty || evidence.Build.Dirty || evidence.Build.SourceMatchStatus != "matched" {
		t.Fatalf("clean producer evidence = %+v / %+v", evidence.SourceTree, evidence.Build)
	}
	return evidence
}

func responseBundleRunCommandForTest(t *testing.T, ctx context.Context, directory, name string, args ...string) {
	t.Helper()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = directory
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run %s %v: %v: %s", name, args, err, output)
	}
}

func responseBundleBindingContextForTest(t *testing.T, build ResponseBundleBuild, typeID string) capsule.BindingContext {
	t.Helper()
	record, err := responseComponentByType(build.Package.Manifest.Components, typeID)
	if err != nil {
		t.Fatal(err)
	}
	records := make(map[string]capsule.ComponentRecord, len(build.Package.Manifest.Components))
	for _, component := range build.Package.Manifest.Components {
		records[component.ComponentID] = component
	}
	parents := make([]capsule.BoundParent, 0, len(record.Parents))
	for _, reference := range record.Parents {
		parent, found := records[reference.ComponentID]
		if !found {
			t.Fatalf("binding parent %q is absent", reference.ComponentID)
		}
		parents = append(parents, capsule.BoundParent{
			Reference: reference, Record: parent, Payload: build.Package.Payloads[parent.Payload.Digest],
		})
	}
	return capsule.BindingContext{
		Component: record, Payload: build.Package.Payloads[record.Payload.Digest], Parents: parents,
	}
}

func readCaptureForTest(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
