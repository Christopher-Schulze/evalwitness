package claim

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const SurfaceSchemaVersion = "evalwitness.claim-surface.v1"

const (
	SurfaceDocumentation = "documentation"
	SurfaceFindings      = "findings"
	SurfaceREADME        = "readme"
	SurfaceRelease       = "release"
	SurfaceResult        = "result"
	SurfaceSkill         = "skill"
)

type SurfaceClaim struct {
	ClaimID       string        `json:"claim_id"`
	Status        Status        `json:"status"`
	EvidenceLevel EvidenceLevel `json:"evidence_level"`
	Value         ExactValue    `json:"value"`
	Statement     string        `json:"statement"`
	Caveats       []string      `json:"caveats"`
}

type SurfaceView struct {
	SchemaVersion string         `json:"schema_version"`
	Surface       string         `json:"surface"`
	Claims        []SurfaceClaim `json:"claims"`
	Digest        string         `json:"digest"`
}

func SurfaceIDs() []string {
	return []string{SurfaceDocumentation, SurfaceFindings, SurfaceREADME, SurfaceRelease, SurfaceResult, SurfaceSkill}
}

func BuildSurfaceViews(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) (map[string]SurfaceView, error) {
	report, err := VerifyLedger(ctx, registry, manifest, payloads, ledger)
	if err != nil {
		return nil, err
	}
	claims := make(map[string]Claim, len(ledger.Claims))
	values := make(map[string]ExactValue, len(report.Claims))
	for _, item := range ledger.Claims {
		claims[item.ClaimID] = item
	}
	for _, verification := range report.Claims {
		values[verification.ClaimID] = verification.ExpressionValue
	}
	views := make(map[string]SurfaceView, len(SurfaceIDs()))
	for _, surface := range SurfaceIDs() {
		view := SurfaceView{SchemaVersion: SurfaceSchemaVersion, Surface: surface, Claims: []SurfaceClaim{}}
		for _, claimID := range surfaceClaimIDs(surface) {
			item, found := claims[claimID]
			if !found {
				return nil, fmt.Errorf("claim surface %q references absent claim %q", surface, claimID)
			}
			view.Claims = append(view.Claims, SurfaceClaim{
				ClaimID: item.ClaimID, Status: item.Status, EvidenceLevel: item.EvidenceLevel,
				Value: values[item.ClaimID], Statement: item.TextTemplate, Caveats: slices.Clone(item.Caveats),
			})
		}
		view.Digest, err = surfaceDigest(view)
		if err != nil {
			return nil, err
		}
		if err := view.Validate(); err != nil {
			return nil, err
		}
		views[surface] = view
	}
	return views, nil
}

func (view SurfaceView) Validate() error {
	if view.SchemaVersion != SurfaceSchemaVersion || !slices.Contains(SurfaceIDs(), view.Surface) ||
		!validDigest(view.Digest) || !slices.Equal(surfaceViewClaimIDs(view.Claims), surfaceClaimIDs(view.Surface)) {
		return errors.New("claim surface identity or coverage is invalid")
	}
	for _, item := range view.Claims {
		if !validClaimID(item.ClaimID) || !item.Status.Valid() || !item.EvidenceLevel.Valid() ||
			!validText(item.Statement) || item.Caveats == nil || !validSortedText(item.Caveats) {
			return fmt.Errorf("claim surface entry %q is invalid", item.ClaimID)
		}
		if err := item.Value.Validate(); err != nil {
			return err
		}
	}
	digest, err := surfaceDigest(view)
	if err != nil || digest != view.Digest {
		return errors.New("claim surface digest is invalid")
	}
	return nil
}

func EncodeSurfaceView(view SurfaceView) ([]byte, error) {
	if err := view.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(view)
}

func DecodeSurfaceView(raw []byte) (SurfaceView, error) {
	var view SurfaceView
	if err := protocol.DecodeStrict(raw, &view); err != nil {
		return SurfaceView{}, fmt.Errorf("decode claim surface: %w", err)
	}
	canonical, err := protocol.CanonicalMarshal(view)
	if err != nil || !bytes.Equal(canonical, raw) {
		return SurfaceView{}, errors.New("claim surface is not canonical JSON")
	}
	if err := view.Validate(); err != nil {
		return SurfaceView{}, err
	}
	return view, nil
}

func RenderSurfaceMarkdown(view SurfaceView) ([]byte, error) {
	if err := view.Validate(); err != nil {
		return nil, err
	}
	var output strings.Builder
	fmt.Fprintf(&output, "%s\n", SurfaceBeginMarker(view.Surface))
	output.WriteString("### Machine-verified claim view\n\n")
	output.WriteString("This block is deterministically derived from the canonical claim ledger and verified capsule. Edit the ledger or evidence, never these rows.\n\n")
	output.WriteString("| Claim | Status | Level | Exact value | Governed statement | Required caveat |\n")
	output.WriteString("|---|---|---|---|---|---|\n")
	for _, item := range view.Claims {
		fmt.Fprintf(
			&output, "| %s | %s | %s | %s:%s | %s | %s |\n",
			item.ClaimID, item.Status, item.EvidenceLevel, item.Value.Kind, markdownCell(item.Value.Value),
			markdownCell(item.Statement), markdownCell(strings.Join(item.Caveats, "<br>")),
		)
	}
	fmt.Fprintf(&output, "\nView digest: `%s`\n\n%s\n", view.Digest, SurfaceEndMarker(view.Surface))
	return []byte(output.String()), nil
}

func SurfaceBeginMarker(surface string) string {
	return "<!-- evalwitness:claim-surface:" + surface + ":begin -->"
}

func SurfaceEndMarker(surface string) string {
	return "<!-- evalwitness:claim-surface:" + surface + ":end -->"
}

func surfaceClaimIDs(surface string) []string {
	switch surface {
	case SurfaceDocumentation:
		return claimIDRange(1, 34)
	case SurfaceREADME:
		return []string{"CLM-001", "CLM-002", "CLM-003", "CLM-031", "CLM-032"}
	case SurfaceFindings:
		return append(claimIDRange(9, 25), "CLM-027", "CLM-031", "CLM-032")
	case SurfaceRelease:
		return []string{"CLM-003", "CLM-029", "CLM-030", "CLM-034"}
	case SurfaceResult:
		return claimIDRange(9, 25)
	case SurfaceSkill:
		return append(claimIDRange(1, 8), claimIDRange(26, 34)...)
	default:
		return nil
	}
}

func claimIDRange(first, last int) []string {
	values := make([]string, 0, last-first+1)
	for value := first; value <= last; value++ {
		values = append(values, fmt.Sprintf("CLM-%03d", value))
	}
	return values
}

func surfaceViewClaimIDs(claims []SurfaceClaim) []string {
	values := make([]string, len(claims))
	for index, item := range claims {
		values[index] = item.ClaimID
	}
	return values
}

func surfaceDigest(view SurfaceView) (string, error) {
	view.Digest = ""
	return protocol.Digest(view)
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.ReplaceAll(value, "\n", " ")
}
