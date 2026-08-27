package claim

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
)

const (
	testRelianceMapType = "evalwitness.evidence-reliance-map.v1"
	testRelianceMapName = "reliance.evidence-map"
)

func TestRelianceLedgerVerifiesClosedClaimsAndChallengePack(t *testing.T) {
	pack := buildRelianceClaimPackage(t)
	ledger, err := BuildRelianceLedger(context.Background(), pack.Registry, pack.Manifest, pack.Payloads,
		RelianceLedgerSource{ComponentName: testRelianceMapName, TypeID: testRelianceMapType})
	if err != nil {
		t.Fatal(err)
	}
	report, err := VerifyLedger(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || !report.Offline || len(report.Claims) != 11 ||
		report.StatusCounts[string(StatusSupported)] != 8 || report.StatusCounts[string(StatusUnsupported)] != 3 {
		t.Fatalf("reliance claim verification = %+v", report)
	}
	challengePack, err := BuildChallengePack(context.Background(), pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if challengePack.Validate() != nil || len(challengePack.Receipts) == 0 {
		t.Fatalf("reliance claim challenge pack = %+v", challengePack)
	}
}

func TestRelianceLedgerRejectsMapPayloadSubstitution(t *testing.T) {
	pack := buildRelianceClaimPackage(t)
	record := pack.Manifest.Components[0]
	tampered := make(map[string][]byte, len(pack.Payloads))
	for digest, payload := range pack.Payloads {
		tampered[digest] = append([]byte(nil), payload...)
	}
	tampered[record.Payload.Digest][0] ^= 1
	if _, err := BuildRelianceLedger(context.Background(), pack.Registry, pack.Manifest, tampered,
		RelianceLedgerSource{ComponentName: testRelianceMapName, TypeID: testRelianceMapType}); err == nil {
		t.Fatal("reliance claim ledger accepted substituted map payload bytes")
	}
}

func buildRelianceClaimPackage(t *testing.T) capsule.ReferencePackage {
	t.Helper()
	document, err := capsule.SealRegistry("evalwitness.test-reliance-claims.v1", "", []capsule.ComponentType{{
		TypeID: testRelianceMapType, SchemaID: testRelianceMapType, Role: capsule.RoleDerivation,
		AllowedVisibilities: []capsule.Visibility{capsule.VisibilityPublic}, MediaType: "application/json",
		PayloadProfile: capsule.PayloadExactBytes, ValidatorID: "evalwitness.test-reliance-claims-validator.v1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := capsule.NewRegistry(document, map[string]capsule.PayloadValidator{
		"evalwitness.test-reliance-claims-validator.v1": func(payload []byte) error {
			var value relianceClaimProjection
			return json.Unmarshal(payload, &value)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(relianceClaimTestProjection())
	if err != nil {
		t.Fatal(err)
	}
	record, normalized, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: testRelianceMapName, TypeID: testRelianceMapType, Visibility: capsule.VisibilityPublic, Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := capsule.BuildManifest(registry, capsule.ManifestInput{
		StudyID: "task-065-claim-test", CellID: "local-mechanism", ScientificRoots: []string{record.ComponentID},
		Components: []capsule.ComponentRecord{record},
	})
	if err != nil {
		t.Fatal(err)
	}
	return capsule.ReferencePackage{Registry: registry, Manifest: manifest, Payloads: map[string][]byte{record.Payload.Digest: normalized}}
}

func relianceClaimTestProjection() relianceClaimProjection {
	value := relianceClaimProjection{
		SchemaVersion: testRelianceMapType, PublicationPolicy: "evalwitness.reliance-publication-policy.v1",
		RegisteredCells: 1_536, Terms: make([]json.RawMessage, 14),
		ArmComparisons: make([]json.RawMessage, 1), Witnesses: make([]json.RawMessage, 1),
		ProfileDimensions: make([]json.RawMessage, 98), PaperRows: make([]json.RawMessage, 98),
		ForbiddenClaims: []string{
			"agent-step or environment causality", "global-minimum witness",
			"human-interpretable or model-internal reasoning attribution",
			"provider, model-family, route, entrypoint, or population transfer without separately admitted empirical evidence",
			"selector nondetection as zero effect or equivalence", "universal verifier faithfulness or trust",
		},
	}
	value.Scope.Domain = "coding_agent_trajectory"
	value.Scope.Entrypoint = "test-entrypoint"
	value.Scope.RouteID = "route-test"
	value.Scope.RequestedModel = "test-model"
	return value
}
