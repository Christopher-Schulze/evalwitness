package explorer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestBuildReportBindsCurrentReferenceEvidence(t *testing.T) {
	report, pack, ledger, autopsy, repositoryRoot := referenceReport(t)
	if report.Capsule.CapsuleID != pack.Manifest.CapsuleID ||
		report.Capsule.LedgerDigest != ledger.Digest || report.Capsule.AutopsyDigest != autopsy.Digest {
		t.Fatal("report identity is detached from its verified capsule sidecars")
	}
	if report.Claim.ClaimID != "claim-fixed-fixture-exits-zero" || !report.Claim.Accepted ||
		report.Scope.Evaluator != "@evalwitness" || report.Scope.Route != nil ||
		report.Scope.RouteAvailability != AvailabilityNotApplicable ||
		report.Scope.DevelopmentFixtures != 2 || report.Scope.EmpiricalTaskGroups != 0 ||
		report.Scope.ResearchAdmittedSources != 0 || report.Scope.ProviderCalls != 0 ||
		report.Scope.Empirical || report.Scope.ProviderRanking {
		t.Fatal("report scope or claim boundary differs from the current development evidence")
	}
	if got := generationStates(report.Method.Generations); !slices.Equal(got, []string{"falsified", "superseded", "admitted_development"}) {
		t.Fatalf("method states = %v", got)
	}
	v3 := report.Method.Generations[2]
	for name, want := range map[string]int{
		"natural_total_attempts": 939,
		"scarcity_admitted":      3,
		"scarcity_target":        40,
		"scarcity_test_role":     0,
	} {
		if got, found := countValue(v3.FrozenDenominators, name); !found || got != want {
			t.Fatalf("v3 denominator %q = %d, found %t, want %d", name, got, found, want)
		}
	}
	if report.Transport.SelectedPath != "non-failable-counterexample" ||
		report.Transport.SelectedLayer != "retained_bundle" || len(report.Transport.Paths) != 2 {
		t.Fatal("transport does not select the canonical first-loss layer")
	}
	for _, layer := range report.Transport.Paths[0].Layers {
		if layer.ObjectDigest == nil || layer.ObjectDigestAvailability != AvailabilityAvailable ||
			layer.DetailAvailability != AvailabilityAvailable || len(layer.RequiredFields) != 4 ||
			len(layer.DecisiveChannels) != 3 {
			t.Fatalf("accepted layer %q lacks its object digest", layer.Layer)
		}
	}
	for index, layer := range report.Transport.Paths[1].Layers {
		want := AvailabilityUnsupported
		if index >= 3 {
			want = AvailabilityNotApplicable
		}
		if layer.ObjectDigest != nil || layer.ObjectDigestAvailability != want ||
			layer.DetailAvailability != want || len(layer.RequiredFields) != 0 || len(layer.DecisiveChannels) != 0 {
			t.Fatalf("counterexample layer %q availability = %q, want %q", layer.Layer, layer.ObjectDigestAvailability, want)
		}
	}
	if report.BOM.BOMID != "offline-bom-stdout-success" || report.BOM.CandidateID != "offline-candidate-stdout-success" ||
		report.BOM.Freshness.State != "current" || report.BOM.Freshness.EvaluatedAt == "" ||
		report.BOM.Freshness.ValidUntil != nil || len(report.BOM.TruncatedRequiredFields) != 0 {
		t.Fatal("accepted BOM projection is incomplete")
	}
	if report.Loss.CertificateID != "task_069-same-path-loss-v1" ||
		report.Loss.FailabilityReason != "self_comparison" || report.Loss.Disposition != "terminal_ineligible" ||
		!report.Loss.PublicSafe || report.Loss.RestrictedContent || len(report.Loss.UnsupportedClaims) != 4 {
		t.Fatal("loss-certificate projection is incomplete")
	}
	if report.Release.Files != 20 || report.Release.FilesVerified != 20 ||
		len(report.Release.Manifest) != 20 ||
		report.Reproduction.Command != "scripts/tests/run-claimcheck.sh" ||
		report.Reproduction.NetworkRequired || report.Reproduction.ProviderCalls != 0 {
		t.Fatal("release verification or reproduction boundary is incorrect")
	}
	if report.OwnerInspection.Assessments.Required != 66 || report.OwnerInspection.Assessments.Completed != 66 ||
		len(report.OwnerInspection.Dimensions) != 16 || report.OwnerInspection.Assessments.ScarcityTestCases != 0 ||
		report.OwnerInspection.Outcomes.CoreStatus != "passed" ||
		report.OwnerInspection.Outcomes.ScarcityStatus != "revision_required" ||
		report.OwnerInspection.Outcomes.OverallStatus != "revision_required" ||
		report.OwnerInspection.HumanStudyStatus != "not_run" ||
		report.OwnerInspection.ExternalActionStatus != "not_authorized" ||
		!report.OwnerInspection.Disclosure.PrivateChainVerified ||
		report.OwnerInspection.Disclosure.PrivateJournalIdentitiesDisclosed ||
		report.OwnerInspection.Disclosure.RestrictedEvidenceDisclosed ||
		len(report.OwnerInspection.Claims) != 10 ||
		report.OwnerInspection.Source.PayloadSHA256 != "7304efbce27d68746f75180b4296c7b76fddfef0f9a4c53a9522db36d5d13fe8" ||
		report.OwnerInspection.Digest != "fd2c364fee2d575120ae4fc29e07788fe5f4107c63f2828da5add916dc7e2a84" {
		t.Fatal("owner-inspection projection differs from the verified public attestation")
	}
	if len(report.Limitations) != 7 || report.Limitations[0].ID != "capsule_integration" ||
		report.Limitations[0].ArtifactStatus != "not_implemented" ||
		report.Limitations[0].CurrentAvailability != AvailabilityAvailable ||
		report.Limitations[0].ResolvedBy == nil ||
		report.Limitations[0].ResolvedBy.ArtifactDigest != autopsy.Digest {
		t.Fatal("historical capsule limitation is not preserved with its current resolution evidence")
	}
	if len(report.Extensions) != 5 {
		t.Fatalf("extension registry contains %d entries, want 5", len(report.Extensions))
	}
	if report.SchemaVersion != "evalwitness.evidence-explorer-report.v2" ||
		report.Stress.CaseStudyID != "first-listed-candidate-order-one-minimal" ||
		report.Stress.Outcome != "violated" || report.Stress.OriginalLineUnits != 32 ||
		report.Stress.FinalLineUnits != 2 || report.Stress.ReductionAttempts != 53 ||
		report.Stress.AcceptedReductions != 30 || report.Stress.RejectedReductions != 23 ||
		report.Stress.ReductionBasisPoints != 9375 || report.Stress.Minimality != "one_minimal" ||
		len(report.Stress.FinalWitness) != 2 || report.Stress.FinalWitness[0].Content != "5" ||
		report.Stress.FinalWitness[1].Content != "-1" || report.Stress.EmpiricalUnits != 0 ||
		report.Stress.ProviderCalls != 0 || report.Stress.NetworkRequired ||
		report.Stress.Source.PayloadSHA256 != "5efdb994d9a608c0f65697013f9870042fd2337595f3813241c93e4ee6f16e44" ||
		report.Stress.Digest != "b4a7adb23505a88ee0ab79c73957bce5fbd15539a0e5e687590719d3ec4ee84b" {
		t.Fatal("stress development case differs from the repository-validated witness")
	}
	for _, extension := range report.Extensions {
		if extension.ExtensionID == "task-056-verifier-relation" && extension.Availability == AvailabilityAvailable {
			t.Fatal("development stress mechanism promoted the unavailable empirical TASK 056 extension")
		}
	}
	for _, extension := range report.Extensions {
		if extension.Availability == AvailabilityAvailable || len(extension.MissingTypes) == 0 {
			t.Fatalf("future extension %q is incorrectly presented as available", extension.ExtensionID)
		}
	}
	if report.Challenge.ClaimID != "CLM-011" || report.Challenge.SelectedReceipts != 7 ||
		report.Challenge.TotalReceipts != 189 || len(report.Challenge.Classes) != 8 {
		t.Fatal("challenge view does not expose the deterministic maximum-coverage claim")
	}
	if report.Challenge.Classes[5].Availability != AvailabilityNotApplicable ||
		report.Challenge.Classes[5].Receipt != nil {
		t.Fatal("stale-attestation challenge was fabricated for an attestation-free claim")
	}
	assertExplorerRenderContract(t, report)

	raw, err := EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, report) {
		t.Fatal("report does not survive its canonical codec")
	}
	second, err := BuildReport(context.Background(), repositoryRoot, pack, ledger, autopsy)
	if err != nil {
		t.Fatal(err)
	}
	secondRaw, err := EncodeReport(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, secondRaw) || report.Digest != second.Digest {
		t.Fatal("identical verified inputs produced different reports")
	}
}

func TestBuildReportRejectsTamperedReleaseBytes(t *testing.T) {
	_, pack, ledger, autopsy, repositoryRoot := referenceReport(t)
	loader := func(root string, file lineage.ReleaseFile) ([]byte, error) {
		raw, err := loadReleaseFile(root, file)
		if err != nil {
			return nil, err
		}
		if file.Path == acceptedBOMPath {
			raw = slices.Clone(raw)
			raw[len(raw)/2] ^= 1
		}
		return raw, nil
	}
	_, err := buildReport(context.Background(), repositoryRoot, pack, ledger, autopsy, loader, loadStressDevelopmentCaseStudy)
	if err == nil || !strings.Contains(err.Error(), "differs from its manifest") {
		t.Fatalf("tampered release bytes error = %v", err)
	}
}

func TestBuildReportRejectsTamperedStressSidecarBytes(t *testing.T) {
	_, pack, ledger, autopsy, repositoryRoot := referenceReport(t)
	loader := func(root string) (stress.DevelopmentCaseStudy, []byte, error) {
		value, raw, err := loadStressDevelopmentCaseStudy(root)
		if err != nil {
			return stress.DevelopmentCaseStudy{}, nil, err
		}
		raw = slices.Clone(raw)
		raw[len(raw)-2] ^= 1
		return value, raw, nil
	}
	_, err := buildReport(context.Background(), repositoryRoot, pack, ledger, autopsy, loadReleaseFile, loader)
	if err == nil || !strings.Contains(err.Error(), "not the deterministic repository projection") {
		t.Fatalf("tampered stress sidecar error = %v", err)
	}
}

func TestBuildReportRejectsTamperedCapsulePayload(t *testing.T) {
	_, pack, ledger, autopsy, repositoryRoot := referenceReport(t)
	_, _, err := componentByType(pack, lineage.BOMSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	tampered := capsule.Package{
		Registry: pack.Registry, Manifest: pack.Manifest,
		Payloads: make(map[string][]byte, len(pack.Payloads)),
	}
	for digest, raw := range pack.Payloads {
		tampered.Payloads[digest] = slices.Clone(raw)
	}
	for _, component := range pack.Manifest.Components {
		if component.TypeID == lineage.BOMSchemaVersion {
			raw := tampered.Payloads[component.Payload.Digest]
			raw[len(raw)/2] ^= 1
			break
		}
	}
	_, err = BuildReport(context.Background(), repositoryRoot, tampered, ledger, autopsy)
	if err == nil || !strings.Contains(err.Error(), "verify evidence explorer sources") {
		t.Fatalf("tampered capsule payload error = %v", err)
	}
}

func TestLoadReleaseFileRejectsSymbolicLink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "eval", "results", "linked.json")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	_, err := loadReleaseFile(root, lineage.ReleaseFile{
		Path: "eval/results/linked.json", Bytes: 2, Digest: protocol.DigestBytes([]byte("{}")),
	})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("symbolic-link release error = %v", err)
	}
}

func TestLoadReleaseFilesRejectsPathEscapeBeforeRead(t *testing.T) {
	called := false
	loader := func(string, lineage.ReleaseFile) ([]byte, error) {
		called = true
		return nil, errors.New("must not be called")
	}
	_, err := loadReleaseFiles(".", lineage.VerificationLineageRelease{Files: []lineage.ReleaseFile{
		{Path: "../outside.json", Role: "escape", Bytes: 1, Digest: strings.Repeat("a", 64)},
	}}, loader)
	if err == nil || called {
		t.Fatalf("unsafe release path error = %v, loader called = %t", err, called)
	}
}

func TestDecodeReportRejectsNonCanonicalAndUnknownFields(t *testing.T) {
	report, _, _, _, _ := referenceReport(t)
	raw, err := EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, raw, "", "  "); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeReport(indented.Bytes()); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("non-canonical report error = %v", err)
	}
	unknown := append([]byte(`{"unknown":true,`), raw[1:]...)
	if _, err := DecodeReport(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown-field report error = %v", err)
	}
}

func TestReportValidationRejectsClosedContractDrift(t *testing.T) {
	report, _, _, _, _ := referenceReport(t)
	tests := []struct {
		name   string
		mutate func(*Report)
	}{
		{name: "missing extension", mutate: func(candidate *Report) {
			candidate.Extensions = candidate.Extensions[:len(candidate.Extensions)-1]
		}},
		{name: "invented extension evidence", mutate: func(candidate *Report) {
			candidate.Extensions[0].MissingTypes = []string{}
			candidate.Extensions[0].Availability = AvailabilityAvailable
		}},
		{name: "challenge source mutation", mutate: func(candidate *Report) {
			candidate.Challenge.Classes = cloneChallengeClasses(report.Challenge.Classes)
			candidate.Challenge.Classes[0].Receipt.AfterSealedSourceDigest = strings.Repeat("0", 64)
		}},
		{name: "false freshness end", mutate: func(candidate *Report) {
			value := "0001-01-01T00:00:00Z"
			candidate.BOM.Freshness.ValidUntil = &value
		}},
		{name: "method state promotion", mutate: func(candidate *Report) {
			candidate.Method.Generations[1].State = "admitted_development"
		}},
		{name: "transport loss suppression", mutate: func(candidate *Report) {
			candidate.Transport.Paths[1].Layers[3].Status = "survived"
		}},
		{name: "dataset scope drift", mutate: func(candidate *Report) {
			candidate.Scope.EmpiricalTaskGroups = 1
		}},
		{name: "private journal source promotion", mutate: func(candidate *Report) {
			candidate.OwnerInspection.Source.SchemaVersion = capsule.PrivateRelationEventChainSchemaVersion
		}},
		{name: "private identifier disclosure", mutate: func(candidate *Report) {
			candidate.OwnerInspection.Disclosure.PrivateJournalIdentitiesDisclosed = true
		}},
		{name: "partial inspection promotion", mutate: func(candidate *Report) {
			candidate.OwnerInspection.Assessments.Completed--
		}},
		{name: "owner inspection promoted to human study", mutate: func(candidate *Report) {
			candidate.OwnerInspection.HumanStudyStatus = "completed"
		}},
		{name: "stress reduction count promoted", mutate: func(candidate *Report) {
			candidate.Stress.AcceptedReductions++
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := report
			candidate.Extensions = slices.Clone(report.Extensions)
			candidate.Method.Generations = slices.Clone(report.Method.Generations)
			candidate.Transport.Paths = cloneTransportPaths(report.Transport.Paths)
			test.mutate(&candidate)
			var err error
			candidate.Digest, err = reportDigest(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if err := candidate.Validate(); err == nil {
				t.Fatal("self-consistent contract drift was accepted")
			}
		})
	}
}

type referenceReportFixture struct {
	reportRaw      []byte
	pack           capsule.Package
	ledgerRaw      []byte
	autopsyRaw     []byte
	repositoryRoot string
}

var (
	referenceReportOnce     sync.Once
	referenceReportCached   referenceReportFixture
	referenceReportBuildErr error
)

func referenceReport(t *testing.T) (Report, capsule.Package, claimledger.Ledger, claimledger.Autopsy, string) {
	t.Helper()
	referenceReportOnce.Do(func() {
		referenceReportCached, referenceReportBuildErr = buildReferenceReportFixture()
	})
	if referenceReportBuildErr != nil {
		t.Fatal(referenceReportBuildErr)
	}
	report, err := DecodeReport(referenceReportCached.reportRaw)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := claimledger.DecodeLedger(referenceReportCached.ledgerRaw)
	if err != nil {
		t.Fatal(err)
	}
	autopsy, err := claimledger.DecodeAutopsy(referenceReportCached.autopsyRaw)
	if err != nil {
		t.Fatal(err)
	}
	return report, cloneExplorerPackage(referenceReportCached.pack), ledger, autopsy, referenceReportCached.repositoryRoot
}

func buildReferenceReportFixture() (referenceReportFixture, error) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return referenceReportFixture{}, fmt.Errorf("resolve repository root: %w", err)
	}
	repositoryRoot, err = filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		return referenceReportFixture{}, fmt.Errorf("resolve repository root links: %w", err)
	}
	reference, err := capsule.BuildReferencePackage(repositoryRoot)
	if err != nil {
		return referenceReportFixture{}, fmt.Errorf("build reference package: %w", err)
	}
	pack := capsule.Package(reference)
	ledger, err := claimledger.DefaultLedger(pack.Manifest)
	if err != nil {
		return referenceReportFixture{}, fmt.Errorf("build claim ledger: %w", err)
	}
	autopsy, err := claimledger.BuildAutopsy(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		return referenceReportFixture{}, fmt.Errorf("build claim autopsy: %w", err)
	}
	report, err := BuildReport(context.Background(), repositoryRoot, pack, ledger, autopsy)
	if err != nil {
		return referenceReportFixture{}, fmt.Errorf("build explorer report: %w", err)
	}
	reportRaw, err := EncodeReport(report)
	if err != nil {
		return referenceReportFixture{}, fmt.Errorf("encode explorer report: %w", err)
	}
	ledgerRaw, err := claimledger.EncodeLedger(ledger)
	if err != nil {
		return referenceReportFixture{}, fmt.Errorf("encode claim ledger: %w", err)
	}
	autopsyRaw, err := claimledger.EncodeAutopsy(autopsy)
	if err != nil {
		return referenceReportFixture{}, fmt.Errorf("encode claim autopsy: %w", err)
	}
	return referenceReportFixture{
		reportRaw: reportRaw, pack: pack, ledgerRaw: ledgerRaw, autopsyRaw: autopsyRaw,
		repositoryRoot: repositoryRoot,
	}, nil
}

func cloneExplorerPackage(source capsule.Package) capsule.Package {
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

func generationStates(generations []MethodGenerationView) []string {
	states := make([]string, len(generations))
	for index, generation := range generations {
		states[index] = generation.State
	}
	return states
}

func cloneChallengeClasses(classes []ChallengeClassView) []ChallengeClassView {
	cloned := slices.Clone(classes)
	for index := range cloned {
		if cloned[index].Receipt != nil {
			receipt := *cloned[index].Receipt
			cloned[index].Receipt = &receipt
		}
	}
	return cloned
}

func countValue(counts []CountView, name string) (int, bool) {
	for _, count := range counts {
		if count.Name == name {
			return count.Value, true
		}
	}
	return 0, false
}

func cloneTransportPaths(paths []TransportPathView) []TransportPathView {
	cloned := slices.Clone(paths)
	for index := range cloned {
		cloned[index].Layers = slices.Clone(paths[index].Layers)
	}
	return cloned
}
