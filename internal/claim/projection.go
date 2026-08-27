package claim

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

type ProjectionClaim struct {
	ClaimID       string        `json:"claim_id"`
	Text          string        `json:"text"`
	Status        Status        `json:"status"`
	EvidenceLevel EvidenceLevel `json:"evidence_level"`
	Scope         Scope         `json:"scope"`
	Value         ExactValue    `json:"value"`
	Caveats       []string      `json:"caveats"`
	CapsuleIDs    []string      `json:"capsule_ids"`
	LedgerDigest  string        `json:"ledger_digest"`
}

type Projection struct {
	SchemaVersion    string            `json:"schema_version"`
	CapsuleID        string            `json:"capsule_id"`
	ManifestDigest   string            `json:"manifest_digest"`
	LedgerDigest     string            `json:"ledger_digest"`
	CurrentClaims    []ProjectionClaim `json:"current_claims"`
	HistoricalClaims []ProjectionClaim `json:"historical_claims"`
	Digest           string            `json:"digest"`
}

func BuildProjection(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) (Projection, error) {
	report, err := VerifyLedger(ctx, registry, manifest, payloads, ledger)
	if err != nil {
		return Projection{}, err
	}
	values := make(map[string]ExactValue, len(report.Claims))
	for _, verification := range report.Claims {
		values[verification.ClaimID] = verification.ExpressionValue
	}
	projection := Projection{
		SchemaVersion: ProjectionSchemaVersion, CapsuleID: manifest.CapsuleID,
		ManifestDigest: manifest.ManifestDigest, LedgerDigest: ledger.Digest,
		CurrentClaims: []ProjectionClaim{}, HistoricalClaims: []ProjectionClaim{},
	}
	for _, item := range ledger.Claims {
		projected := ProjectionClaim{
			ClaimID: item.ClaimID, Text: item.TextTemplate, Status: item.Status,
			EvidenceLevel: item.EvidenceLevel, Scope: cloneScope(item.Scope), Value: values[item.ClaimID],
			Caveats: slices.Clone(item.Caveats), CapsuleIDs: slices.Clone(item.CapsuleIDs), LedgerDigest: ledger.Digest,
		}
		if item.Status.Assertable() {
			projection.CurrentClaims = append(projection.CurrentClaims, projected)
		} else {
			projection.HistoricalClaims = append(projection.HistoricalClaims, projected)
		}
	}
	projection.Digest, err = projectionDigest(projection)
	if err != nil {
		return Projection{}, err
	}
	if err := projection.Validate(); err != nil {
		return Projection{}, err
	}
	return projection, nil
}

func (projection Projection) Validate() error {
	if projection.SchemaVersion != ProjectionSchemaVersion || !validDigest(projection.CapsuleID) ||
		!validDigest(projection.ManifestDigest) || !validDigest(projection.LedgerDigest) ||
		projection.CurrentClaims == nil || projection.HistoricalClaims == nil || !validDigest(projection.Digest) {
		return errors.New("claim projection identity is invalid")
	}
	seen := make(map[string]struct{}, len(projection.CurrentClaims)+len(projection.HistoricalClaims))
	if err := validateProjectionClaims(projection.CurrentClaims, projection, seen, true); err != nil {
		return err
	}
	if err := validateProjectionClaims(projection.HistoricalClaims, projection, seen, false); err != nil {
		return err
	}
	digest, err := projectionDigest(projection)
	if err != nil || digest != projection.Digest {
		return errors.New("claim projection digest is invalid")
	}
	return nil
}

func validateProjectionClaims(claims []ProjectionClaim, projection Projection, seen map[string]struct{}, current bool) error {
	previous := ""
	for _, item := range claims {
		if !validClaimID(item.ClaimID) || item.ClaimID <= previous || !validText(item.Text) || !item.Status.Valid() ||
			item.Status.Assertable() != current || !item.EvidenceLevel.Valid() || item.Caveats == nil ||
			!validSortedText(item.Caveats) || !slices.Equal(item.CapsuleIDs, []string{projection.CapsuleID}) ||
			item.LedgerDigest != projection.LedgerDigest {
			return fmt.Errorf("projected claim %q is invalid or in the wrong lifecycle lane", item.ClaimID)
		}
		if err := item.Scope.Validate(); err != nil {
			return err
		}
		if err := item.Value.Validate(); err != nil {
			return err
		}
		if _, duplicate := seen[item.ClaimID]; duplicate {
			return fmt.Errorf("projected claim %q is duplicated", item.ClaimID)
		}
		seen[item.ClaimID] = struct{}{}
		previous = item.ClaimID
	}
	return nil
}

func projectionDigest(projection Projection) (string, error) {
	projection.Digest = ""
	return protocol.Digest(projection)
}

func cloneScope(scope Scope) Scope {
	return Scope{
		Routes: slices.Clone(scope.Routes), Models: slices.Clone(scope.Models), Domains: slices.Clone(scope.Domains),
		Tasks: slices.Clone(scope.Tasks), Policies: slices.Clone(scope.Policies),
		TimeBounds: slices.Clone(scope.TimeBounds), Entrypoints: slices.Clone(scope.Entrypoints),
	}
}
