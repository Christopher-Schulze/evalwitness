package reliance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/claim"
)

var relianceCapsuleFixtureCache struct {
	once   sync.Once
	base   capsule.ReferencePackage
	child  capsule.ReferencePackage
	mapOut EvidenceRelianceMap
}

type reliancePublicArtifact struct {
	path string
	raw  []byte
}

type relianceArtifactDigestMapping struct {
	leftToRight map[string]string
	rightToLeft map[string]string
}

func TestEvidenceRelianceCapsuleExtendsVerifiedTask050Parents(t *testing.T) {
	base, child, value := cachedRelianceCapsuleFixture(t)
	report, err := capsule.VerifyPackageFamily(context.Background(), referencePackageAsCapsule(child),
		[]capsule.Package{referencePackageAsCapsule(base)},
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || !report.Offline || len(child.Manifest.Components) != 1 ||
		child.Manifest.Components[0].TypeID != EvidenceRelianceMapSchemaVersion ||
		child.Manifest.Components[0].Payload.Digest == "" || child.Manifest.CapsuleID == base.Manifest.CapsuleID {
		t.Fatalf("evidence reliance capsule identity = %+v / %+v", child.Manifest, report)
	}
	if len(child.Manifest.Components[0].Parents) != 2 || value.Digest == "" {
		t.Fatalf("evidence reliance capsule parents or map digest = %+v / %q", child.Manifest.Components[0].Parents, value.Digest)
	}
}

func TestEvidenceRelianceCapsuleRejectsEmpiricalPromotionAndForeignFamily(t *testing.T) {
	base, child, value := cachedRelianceCapsuleFixture(t)
	promoted := cloneRelianceMapTestValue(t, value)
	promoted.Scope.Empirical = true
	resealRelianceMapTestValue(t, &promoted)
	if _, err := BuildEvidenceRelianceCapsule(base, promoted); err == nil {
		t.Fatal("evidence reliance capsule accepted empirical promotion")
	}
	foreign := base
	foreign.Manifest.CapsuleID = strings.Repeat("f", 64)
	if _, err := capsule.VerifyPackageFamily(context.Background(), referencePackageAsCapsule(child),
		[]capsule.Package{referencePackageAsCapsule(foreign)},
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}); err == nil {
		t.Fatal("evidence reliance capsule accepted a foreign parent family")
	}
}

func TestEvidenceRelianceLedgerVerifiesCapsuleDerivedClaims(t *testing.T) {
	base, child, _ := cachedRelianceCapsuleFixture(t)
	ledger, err := BuildEvidenceRelianceLedger(context.Background(), base, child)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Claims) != 11 || ledger.Claims[0].ClaimID != "CLM-035" || ledger.Claims[10].ClaimID != "CLM-045" {
		t.Fatalf("evidence reliance ledger claims = %+v", ledger.Claims)
	}
	report, err := claim.VerifyLedger(context.Background(), child.Registry, child.Manifest, child.Payloads, ledger)
	if err != nil || !report.Valid || report.StatusCounts[string(claim.StatusSupported)] != 8 ||
		report.StatusCounts[string(claim.StatusUnsupported)] != 3 {
		t.Fatalf("evidence reliance ledger verification = %+v / %v", report, err)
	}
	challenges, err := claim.BuildChallengePack(context.Background(), child.Registry, child.Manifest, child.Payloads, ledger)
	if err != nil || challenges.Validate() != nil || len(challenges.Receipts) == 0 {
		t.Fatalf("evidence reliance claim challenges = %+v / %v", challenges, err)
	}
}

func TestRelianceProfileAndPaperProjectionsRemainCapsuleDerived(t *testing.T) {
	base, child, _ := cachedRelianceCapsuleFixture(t)
	ledger, err := BuildEvidenceRelianceLedger(context.Background(), base, child)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := BuildRelianceProfileProjection(context.Background(), base, child, ledger)
	if err != nil {
		t.Fatal(err)
	}
	paper, err := BuildReliancePaperProjection(context.Background(), base, child, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.Validate(context.Background(), base, child, ledger); err != nil {
		t.Fatal(err)
	}
	if err := paper.Validate(context.Background(), base, child, ledger); err != nil {
		t.Fatal(err)
	}
	if len(profile.Dimensions) != 98 || !profile.GlobalScoreProhibited ||
		len(paper.Rows) != 98 || len(paper.CurrentClaimIDs) != 8 || len(paper.UnsupportedClaimIDs) != 3 ||
		profile.ProviderCalls != 0 || profile.NetworkRequired || paper.ProviderCalls != 0 || paper.NetworkRequired {
		t.Fatalf("reliance profile or paper projection = %+v / %+v", profile, paper)
	}
	assertRelianceProjectionTamperRejection(t, base, child, ledger, profile, paper)
}

func TestRelianceExplorerProjectionRemainsCapsuleDerivedAndPrivateSafe(t *testing.T) {
	base, child, _ := cachedRelianceCapsuleFixture(t)
	ledger, err := BuildEvidenceRelianceLedger(context.Background(), base, child)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := BuildRelianceExplorerProjection(context.Background(), base, child, ledger)
	if err != nil || projection.Validate() != nil {
		t.Fatalf("build reliance explorer projection: %v", err)
	}
	if len(projection.Outcomes) != 98 || len(projection.Selectors) != len(relianceExplorerSelectorTerms()) ||
		len(projection.ArmContrasts) != 5 || len(projection.Witnesses) != 1 ||
		projection.Witnesses[0].RawTrajectoryContentShown || projection.ProviderCalls != 0 ||
		projection.NetworkRequired || !projection.GlobalScoreProhibited {
		t.Fatalf("reliance explorer projection boundary = %+v", projection)
	}
	candidate := cloneRelianceExplorerProjectionTestValue(t, projection)
	candidate.Outcomes[0].Denominator--
	candidate.Digest, err = relianceExplorerProjectionDigest(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := candidate.Validate(); err == nil {
		t.Fatal("reliance explorer projection accepted a resealed denominator change")
	}
}

func TestEvidenceReliancePublicArtifactsMatchCanonicalProjection(t *testing.T) {
	artifacts := buildReliancePublicArtifacts(t)
	if os.Getenv("EVALWITNESS_WRITE_RELIANCE_CANDIDATES") == "1" {
		for _, artifact := range artifacts {
			writeRelianceCandidate(t, artifact)
		}
	}
	digestMapping := relianceArtifactDigestMapping{
		leftToRight: make(map[string]string), rightToLeft: make(map[string]string),
	}
	for _, artifact := range artifacts {
		want, err := os.ReadFile(artifact.path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(want, artifact.raw) {
			continue
		}
		if err := compareRelianceArtifactJSON(want, artifact.raw, &digestMapping); err != nil {
			t.Fatalf("public reliance artifact %q differs from canonical projection: %v", artifact.path, err)
		}
	}
}

func compareRelianceArtifactJSON(want, got []byte, mapping *relianceArtifactDigestMapping) error {
	wantValue, err := decodeRelianceArtifactJSON(want)
	if err != nil {
		return fmt.Errorf("decode committed artifact: %w", err)
	}
	gotValue, err := decodeRelianceArtifactJSON(got)
	if err != nil {
		return fmt.Errorf("decode canonical projection: %w", err)
	}
	return compareRelianceJSONValues(wantValue, gotValue, "$", mapping)
}

func decodeRelianceArtifactJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("trailing JSON value")
		}
		return nil, err
	}
	return value, nil
}

func compareRelianceJSONValues(
	want, got any,
	path string,
	mapping *relianceArtifactDigestMapping,
) error {
	switch wantValue := want.(type) {
	case nil:
		if got != nil {
			return fmt.Errorf("%s: want null, got %T", path, got)
		}
	case bool:
		if gotValue, ok := got.(bool); !ok || wantValue != gotValue {
			return fmt.Errorf("%s: boolean differs", path)
		}
	case string:
		gotValue, ok := got.(string)
		if !ok {
			return fmt.Errorf("%s: string differs", path)
		}
		if !compareRelianceNumericStrings(path, wantValue, gotValue) && !compareRelianceStrings(wantValue, gotValue, mapping) {
			return fmt.Errorf("%s: string differs", path)
		}
	case json.Number:
		gotValue, ok := got.(json.Number)
		if !ok {
			return fmt.Errorf("%s: number type differs", path)
		}
		wantFloat, wantErr := strconv.ParseFloat(wantValue.String(), 64)
		gotFloat, gotErr := strconv.ParseFloat(gotValue.String(), 64)
		if wantErr != nil || gotErr != nil || !closeRelianceFloat(wantFloat, gotFloat) {
			return fmt.Errorf("%s: numeric values %s and %s differ", path, wantValue, gotValue)
		}
	case []any:
		gotValue, ok := got.([]any)
		if !ok || len(wantValue) != len(gotValue) {
			return fmt.Errorf("%s: array shape differs", path)
		}
		for index := range wantValue {
			if err := compareRelianceJSONValues(wantValue[index], gotValue[index], fmt.Sprintf("%s[%d]", path, index), mapping); err != nil {
				return err
			}
		}
	case map[string]any:
		gotValue, ok := got.(map[string]any)
		if !ok || len(wantValue) != len(gotValue) {
			return fmt.Errorf("%s: object shape differs", path)
		}
		for key, wantItem := range wantValue {
			gotItem, ok := gotValue[key]
			if !ok {
				return fmt.Errorf("%s.%s: key missing", path, key)
			}
			if err := compareRelianceJSONValues(wantItem, gotItem, path+"."+key, mapping); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("%s: unsupported JSON value %T", path, want)
	}
	return nil
}

func compareRelianceStrings(want, got string, mapping *relianceArtifactDigestMapping) bool {
	if want == got {
		return true
	}
	if !relianceDigestString(want) || !relianceDigestString(got) {
		return false
	}
	if mapped, ok := mapping.leftToRight[want]; ok {
		return mapped == got
	}
	if mapped, ok := mapping.rightToLeft[got]; ok {
		return mapped == want
	}
	mapping.leftToRight[want] = got
	mapping.rightToLeft[got] = want
	return true
}

func compareRelianceNumericStrings(path, want, got string) bool {
	if !relianceNumericStringPath(path) {
		return false
	}
	wantFloat, wantErr := strconv.ParseFloat(want, 64)
	gotFloat, gotErr := strconv.ParseFloat(got, 64)
	return wantErr == nil && gotErr == nil && closeRelianceFloat(wantFloat, gotFloat)
}

func relianceNumericStringPath(path string) bool {
	for _, suffix := range []string{
		".estimate.estimate", ".estimate.standard_error", ".estimate.lower", ".estimate.upper", ".estimate.adjusted_p_value",
	} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func relianceDigestString(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func TestRelianceArtifactComparisonRejectsMaterialDrift(t *testing.T) {
	tests := []struct {
		name string
		want string
		got  string
		fail bool
	}{
		{name: "exact", want: `{"value":1}`, got: `{"value":1}`},
		{name: "platform rounding", want: `{"value":1}`, got: `{"value":1.0000000000001}`},
		{name: "platform string rounding", want: `{"estimate":{"estimate":"1"}}`, got: `{"estimate":{"estimate":"1.0000000000001"}}`},
		{name: "material numeric drift", want: `{"value":1}`, got: `{"value":1.000001}`, fail: true},
		{name: "material string numeric drift", want: `{"estimate":{"estimate":"1"}}`, got: `{"estimate":{"estimate":"1.000001"}}`, fail: true},
		{name: "consistent digest substitution", want: `{"digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, got: `{"digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`},
		{name: "inconsistent digest substitution", want: `{"a":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","b":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, got: `{"a":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","b":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`, fail: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapping := relianceArtifactDigestMapping{leftToRight: make(map[string]string), rightToLeft: make(map[string]string)}
			err := compareRelianceArtifactJSON([]byte(test.want), []byte(test.got), &mapping)
			if (err != nil) != test.fail {
				t.Fatalf("comparison error = %v, want failure = %v", err, test.fail)
			}
		})
	}
}

func buildReliancePublicArtifacts(t *testing.T) []reliancePublicArtifact {
	t.Helper()
	base, child, _ := cachedRelianceCapsuleFixture(t)
	ledger, err := BuildEvidenceRelianceLedger(context.Background(), base, child)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := BuildRelianceProfileProjection(context.Background(), base, child, ledger)
	if err != nil {
		t.Fatal(err)
	}
	paper, err := BuildReliancePaperProjection(context.Background(), base, child, ledger)
	if err != nil {
		t.Fatal(err)
	}
	explorer, err := BuildRelianceExplorerProjection(context.Background(), base, child, ledger)
	if err != nil {
		t.Fatal(err)
	}
	ledgerRaw, err := claim.EncodeLedger(ledger)
	if err != nil {
		t.Fatal(err)
	}
	return reliancePublicArtifactSet(t, child, ledgerRaw, profile, paper, explorer)
}

func reliancePublicArtifactSet(
	t *testing.T,
	child capsule.ReferencePackage,
	ledgerRaw []byte,
	profile RelianceProfileProjection,
	paper ReliancePaperProjection,
	explorer RelianceExplorerProjection,
) []reliancePublicArtifact {
	t.Helper()
	record, err := uniqueRelianceCapsuleComponent(child.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..", "eval", "results")
	return []reliancePublicArtifact{
		{filepath.Join(root, "evidence-reliance-map-v1.json"), child.Payloads[record.Payload.Digest]},
		{filepath.Join(root, "evidence-reliance-claims-v1.json"), ledgerRaw},
		{filepath.Join(root, "evidence-reliance-profile-v1.json"), marshalRelianceArtifact(t, profile)},
		{filepath.Join(root, "evidence-reliance-paper-v1.json"), marshalRelianceArtifact(t, paper)},
		{filepath.Join(root, "evidence-reliance-explorer-v1.json"), marshalRelianceArtifact(t, explorer)},
	}
}

func marshalRelianceArtifact(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func writeRelianceCandidate(t *testing.T, artifact reliancePublicArtifact) {
	t.Helper()
	candidate := artifact.path + ".candidate"
	if _, err := os.Lstat(candidate); !os.IsNotExist(err) {
		t.Fatalf("refusing to overwrite reliance candidate %q", candidate)
	}
	if err := os.WriteFile(candidate, artifact.raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func cloneRelianceExplorerProjectionTestValue(
	t *testing.T,
	value RelianceExplorerProjection,
) RelianceExplorerProjection {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result RelianceExplorerProjection
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertRelianceProjectionTamperRejection(
	t *testing.T,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	ledger claim.Ledger,
	profile RelianceProfileProjection,
	paper ReliancePaperProjection,
) {
	t.Helper()
	profile.Dimensions[0].InvalidCells++
	digest, err := relianceProfileProjectionDigest(profile)
	if err != nil {
		t.Fatal(err)
	}
	profile.Digest = digest
	if err := profile.Validate(context.Background(), base, child, ledger); err == nil {
		t.Fatal("resealed reliance profile projection tampering was accepted")
	}
	paper.Rows = paper.Rows[:len(paper.Rows)-1]
	digest, err = reliancePaperProjectionDigest(paper)
	if err != nil {
		t.Fatal(err)
	}
	paper.Digest = digest
	if err := paper.Validate(context.Background(), base, child, ledger); err == nil {
		t.Fatal("resealed reliance paper projection tampering was accepted")
	}
}

func cachedRelianceCapsuleFixture(
	t *testing.T,
) (capsule.ReferencePackage, capsule.ReferencePackage, EvidenceRelianceMap) {
	t.Helper()
	relianceCapsuleFixtureCache.once.Do(func() {
		_, value := cachedReliancePublicationFixture(t)
		base := loadReliancePublicationBase(t)
		child, err := BuildEvidenceRelianceCapsule(base, value)
		if err != nil {
			t.Fatal(err)
		}
		relianceCapsuleFixtureCache.base = base
		relianceCapsuleFixtureCache.child = child
		relianceCapsuleFixtureCache.mapOut = value
	})
	return relianceCapsuleFixtureCache.base, relianceCapsuleFixtureCache.child, relianceCapsuleFixtureCache.mapOut
}

func loadReliancePublicationBase(t *testing.T) capsule.ReferencePackage {
	t.Helper()
	root := filepath.Join("..", "..", "eval", "results", "evidence-reliance-base-capsule-v1")
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		if os.Getenv("EVALWITNESS_WRITE_RELIANCE_BASE_CANDIDATE") != "1" {
			t.Fatal(err)
		}
		return writeRelianceBaseCandidate(t, root)
	} else if err != nil {
		t.Fatal(err)
	}
	registry, err := capsule.ReferenceRegistry()
	if err != nil {
		t.Fatal(err)
	}
	manifest, payloads, err := capsule.LoadDirectory(context.Background(), root, registry,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic})
	if err != nil {
		t.Fatal(err)
	}
	base := capsule.ReferencePackage{Registry: registry, Manifest: manifest, Payloads: payloads}
	verifyRelianceBaseLedger(t, filepath.Dir(root), base)
	return base
}

func writeRelianceBaseCandidate(t *testing.T, root string) capsule.ReferencePackage {
	t.Helper()
	candidate := root + ".candidate"
	if _, err := os.Lstat(candidate); !os.IsNotExist(err) {
		t.Fatalf("refusing to overwrite reliance base candidate %q", candidate)
	}
	base, err := capsule.BuildReferencePackage(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := capsule.WriteDirectory(context.Background(), candidate, base.Registry, base.Manifest, base.Payloads); err != nil {
		t.Fatal(err)
	}
	writeRelianceBaseLedgerCandidate(t, filepath.Dir(root), base)
	return base
}

func writeRelianceBaseLedgerCandidate(t *testing.T, root string, base capsule.ReferencePackage) {
	t.Helper()
	ledger, err := claim.DefaultLedger(base.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := claim.EncodeLedger(ledger)
	if err != nil {
		t.Fatal(err)
	}
	writeRelianceCandidate(t, reliancePublicArtifact{
		path: filepath.Join(root, "evidence-reliance-base-claims-v1.json"), raw: raw,
	})
}

func verifyRelianceBaseLedger(t *testing.T, root string, base capsule.ReferencePackage) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "evidence-reliance-base-claims-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := claim.DecodeLedger(raw)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := claim.DefaultLedger(base.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	expectedRaw, err := claim.EncodeLedger(expected)
	report, verifyErr := claim.VerifyLedger(context.Background(), base.Registry, base.Manifest, base.Payloads, ledger)
	if err != nil || verifyErr != nil || !report.Valid || !bytes.Equal(raw, expectedRaw) || ledger.Digest != expected.Digest {
		t.Fatal("reliance base claim ledger differs from its frozen capsule")
	}
}
