package claim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
)

const relianceClaimGeneration = "reliance-local-mechanism-v1"

type RelianceLedgerSource struct {
	ComponentName string
	TypeID        string
}

type relianceClaimProjection struct {
	SchemaVersion           string `json:"schema_version"`
	PublicationPolicy       string `json:"publication_policy"`
	RegisteredCells         int    `json:"registered_cells"`
	ProjectionProviderCalls int    `json:"projection_provider_calls"`
	NetworkRequired         bool   `json:"network_required"`
	Scope                   struct {
		Domain         string `json:"domain"`
		Entrypoint     string `json:"entrypoint"`
		RouteID        string `json:"route_id"`
		RequestedModel string `json:"requested_model"`
		Empirical      bool   `json:"empirical"`
	} `json:"scope"`
	Terms             []json.RawMessage `json:"terms"`
	ArmComparisons    []json.RawMessage `json:"arm_comparisons"`
	Witnesses         []json.RawMessage `json:"witnesses"`
	ProfileDimensions []json.RawMessage `json:"profile_dimensions"`
	PaperRows         []json.RawMessage `json:"paper_rows"`
	ForbiddenClaims   []string          `json:"forbidden_claims"`
}

func BuildRelianceLedger(
	ctx context.Context,
	registry *capsule.Registry,
	manifest capsule.Manifest,
	payloads map[string][]byte,
	source RelianceLedgerSource,
) (Ledger, error) {
	if ctx == nil || registry == nil {
		return Ledger{}, errors.New("reliance claim ledger requires context and capsule registry")
	}
	if _, err := capsule.VerifyPackage(ctx, registry, manifest, payloads,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}); err != nil {
		return Ledger{}, err
	}
	record, err := relianceMapComponent(manifest, source)
	if err != nil {
		return Ledger{}, err
	}
	value, err := decodeRelianceClaimProjection(payloads[record.Payload.Digest], source.TypeID)
	if err != nil {
		return Ledger{}, err
	}
	return SealLedger(manifest, relianceClaims(value, source))
}

func relianceMapComponent(manifest capsule.Manifest, source RelianceLedgerSource) (capsule.ComponentRecord, error) {
	if !validToken(source.ComponentName) || !validToken(source.TypeID) {
		return capsule.ComponentRecord{}, errors.New("reliance claim source identity is invalid")
	}
	var result capsule.ComponentRecord
	found := 0
	for _, record := range manifest.Components {
		if record.Name == source.ComponentName {
			result, found = record, found+1
		}
	}
	if found != 1 || result.TypeID != source.TypeID {
		return capsule.ComponentRecord{}, errors.New("reliance claim ledger requires exactly one typed reliance map component")
	}
	return result, nil
}

func decodeRelianceClaimProjection(payload []byte, typeID string) (relianceClaimProjection, error) {
	var value relianceClaimProjection
	if err := json.Unmarshal(payload, &value); err != nil {
		return relianceClaimProjection{}, fmt.Errorf("decode reliance claim projection: %w", err)
	}
	if value.SchemaVersion != typeID || value.PublicationPolicy == "" || value.RegisteredCells <= 0 ||
		value.ProjectionProviderCalls != 0 || value.NetworkRequired || value.Scope.Empirical ||
		value.Scope.Domain == "" || value.Scope.Entrypoint == "" || value.Scope.RouteID == "" || value.Scope.RequestedModel == "" ||
		len(value.Terms) == 0 || len(value.ArmComparisons) == 0 || len(value.Witnesses) == 0 ||
		len(value.ProfileDimensions) == 0 || len(value.PaperRows) == 0 || len(value.ForbiddenClaims) < 5 ||
		value.ForbiddenClaims[0] != "agent-step or environment causality" ||
		value.ForbiddenClaims[4] != "selector nondetection as zero effect or equivalence" {
		return relianceClaimProjection{}, errors.New("reliance claim projection identity, denominator, or claim boundary is invalid")
	}
	return value, nil
}

func relianceClaims(value relianceClaimProjection, source RelianceLedgerSource) []Claim {
	component := source.ComponentName
	typeID := source.TypeID
	scope := relianceClaimScope(value)
	localCaveat := "E1 local mechanism fixture only; no external-model or transfer result"
	return []Claim{
		defaultClaim("CLM-035", fmt.Sprintf("The capsule publishes exactly %d frozen evidence-factor and interaction terms", len(value.Terms)), StatusSupported, EvidenceE1, scope, count(component, typeID, "/terms", strconv.Itoa(len(value.Terms))), localCaveat, relianceClaimGeneration, AttestationNotRequired),
		defaultClaim("CLM-036", fmt.Sprintf("The reliance map retains all %d registered factorial cells in its denominator accounting", value.RegisteredCells), StatusSupported, EvidenceE1, scope, pointer(component, typeID, "/registered_cells", NumberValue(strconv.Itoa(value.RegisteredCells))), localCaveat, relianceClaimGeneration, AttestationNotRequired),
		defaultClaim("CLM-037", fmt.Sprintf("The capsule publishes exactly %d profile dimensions derived from the canonical reliance map", len(value.ProfileDimensions)), StatusSupported, EvidenceE1, scope, count(component, typeID, "/profile_dimensions", strconv.Itoa(len(value.ProfileDimensions))), localCaveat, relianceClaimGeneration, AttestationNotRequired),
		defaultClaim("CLM-038", fmt.Sprintf("The capsule publishes exactly %d paper rows derived from the canonical reliance map", len(value.PaperRows)), StatusSupported, EvidenceE1, scope, count(component, typeID, "/paper_rows", strconv.Itoa(len(value.PaperRows))), localCaveat, relianceClaimGeneration, AttestationNotRequired),
		defaultClaim("CLM-039", "Reliance publication projection performed zero provider calls", StatusSupported, EvidenceE1, scope, pointer(component, typeID, "/projection_provider_calls", NumberValue("0")), localCaveat, relianceClaimGeneration, AttestationNotRequired),
		defaultClaim("CLM-040", "Reliance publication projection required no network access", StatusSupported, EvidenceE1, scope, pointer(component, typeID, "/network_required", BooleanValue(false)), localCaveat, relianceClaimGeneration, AttestationNotRequired),
		defaultClaim("CLM-041", fmt.Sprintf("The capsule publishes %d prespecified arm-comparison artifact with all five contrast families explicit", len(value.ArmComparisons)), StatusSupported, EvidenceE1, scope, count(component, typeID, "/arm_comparisons", strconv.Itoa(len(value.ArmComparisons))), localCaveat, relianceClaimGeneration, AttestationNotRequired),
		defaultClaim("CLM-042", fmt.Sprintf("The capsule publishes %d public one-minimal reliance witness", len(value.Witnesses)), StatusSupported, EvidenceE1, scope, count(component, typeID, "/witnesses", strconv.Itoa(len(value.Witnesses))), "One-minimality is over declared reduction units and does not establish a global minimum", relianceClaimGeneration, AttestationNotRequired),
		defaultClaim("CLM-043", "The reliance capsule establishes provider, model-family, route, entrypoint, or population transfer", StatusUnsupported, EvidenceE0, scope, pointer(component, typeID, "/scope/empirical", BooleanValue(false)), "No transfer claim is permitted without separately admitted empirical evidence", relianceClaimGeneration, AttestationUnavailable),
		defaultClaim("CLM-044", "The reliance capsule establishes agent-step causality or model-internal reasoning attribution", StatusUnsupported, EvidenceE0, scope, pointer(component, typeID, "/forbidden_claims/0", StringValue("agent-step or environment causality")), "Observable verifier-output movement does not identify agent causality or model internals", relianceClaimGeneration, AttestationUnavailable),
		defaultClaim("CLM-045", "Selector nondetection in the reliance capsule establishes zero effect or equivalence", StatusUnsupported, EvidenceE0, scope, pointer(component, typeID, "/forbidden_claims/4", StringValue("selector nondetection as zero effect or equivalence")), "Nondetection is not equivalence and cannot justify selector tuning from held-out outcomes", relianceClaimGeneration, AttestationUnavailable),
	}
}

func relianceClaimScope(value relianceClaimProjection) Scope {
	return Scope{
		Routes: []string{value.Scope.RouteID}, Models: []string{value.Scope.RequestedModel},
		Domains: []string{value.Scope.Domain}, Tasks: []string{"TASK-065"},
		Policies: []string{value.PublicationPolicy}, TimeBounds: []string{},
		Entrypoints: []string{value.Scope.Entrypoint},
	}
}
