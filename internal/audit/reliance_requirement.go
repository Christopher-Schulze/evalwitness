package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// RelianceRequirement is a TASK 065 compatible-panel policy requirement. It
// names a map_term_id (factor), optional term kind, status, and evidence level
// that the frozen, capsule-backed TASK 065 profile projection must satisfy
// exactly. It is valid only against that panel — never a universal claim.
type RelianceRequirement struct {
	MapTermID     string `json:"map_term_id"`
	TermKind      string `json:"term_kind,omitempty"`
	Status        string `json:"status"`
	EvidenceLevel string `json:"evidence_level"`
}

// The following types mirror internal/reliance's publication projection shapes
// exactly (field names, JSON tags, order) so the canonical digest recompute is
// byte-identical. Declared locally because internal/reliance transitively
// imports internal/audit through internal/stress -> internal/mode.

type reliancePublicationScope struct {
	EvidenceRole        string `json:"evidence_role"`
	DataRole            string `json:"data_role"`
	EvidenceLevel       string `json:"evidence_level"`
	Domain              string `json:"domain"`
	StudyManifestDigest string `json:"study_manifest_digest"`
	Entrypoint          string `json:"entrypoint"`
	CriterionID         string `json:"criterion_id"`
	ScoreTag            string `json:"score_tag"`
	EvidencePolicy      string `json:"evidence_policy"`
	ProviderID          string `json:"provider_id"`
	RouteID             string `json:"route_id"`
	RequestedModel      string `json:"requested_model"`
	Empirical           bool   `json:"empirical"`
}

type relianceMapEstimate struct {
	Estimate       float64 `json:"estimate"`
	StandardError  float64 `json:"standard_error"`
	Lower          float64 `json:"lower"`
	Upper          float64 `json:"upper"`
	AdjustedPValue float64 `json:"adjusted_p_value"`
}

type reliancePanelDimension struct {
	DimensionID          string                   `json:"dimension_id"`
	MapTermID            string                   `json:"map_term_id"`
	TermKind             string                   `json:"term_kind"`
	Factors              []string                 `json:"factors"`
	OutcomeID            string                   `json:"outcome_id"`
	Status               string                   `json:"status"`
	EvidenceLevel        string                   `json:"evidence_level"`
	Scope                reliancePublicationScope `json:"scope"`
	RegisteredCells      int                      `json:"registered_cells"`
	EligibleObservations int                      `json:"eligible_observations"`
	ExcludedFromFit      int                      `json:"excluded_from_fit"`
	InvalidCells         int                      `json:"invalid_cells"`
	Estimate             *relianceMapEstimate     `json:"estimate,omitempty"`
	Reason               string                   `json:"reason,omitempty"`
	SourceAnalysisDigest string                   `json:"source_analysis_digest"`
	Policy               string                   `json:"policy"`
	Unit                 string                   `json:"unit"`
	InterventionFamily   string                   `json:"intervention_family"`
	ClaimBoundary        string                   `json:"claim_boundary"`
	CapsuleExpression    string                   `json:"capsule_expression"`
	Caveats              []string                 `json:"caveats"`
}

type reliancePanelStatusCount struct {
	Status     string `json:"status"`
	Dimensions int    `json:"dimensions"`
}

type reliancePanel struct {
	SchemaVersion         string                     `json:"schema_version"`
	CapsuleID             string                     `json:"capsule_id"`
	ManifestDigest        string                     `json:"manifest_digest"`
	MapComponentID        string                     `json:"map_component_id"`
	MapDigest             string                     `json:"map_digest"`
	LedgerDigest          string                     `json:"ledger_digest"`
	Scope                 reliancePublicationScope   `json:"scope"`
	Dimensions            []reliancePanelDimension   `json:"dimensions"`
	StatusCounts          []reliancePanelStatusCount `json:"status_counts"`
	GlobalScoreProhibited bool                       `json:"global_score_prohibited"`
	ProviderCalls         int                        `json:"provider_calls"`
	NetworkRequired       bool                       `json:"network_required"`
	Digest                string                     `json:"digest"`
}

const reliancePanelSchemaVersion = "evalwitness.reliance-profile-projection.v1"

// declaredDigest reads the digest field from the raw bytes before any
// normalization so a tampered file is detected against its own claim.
// A malformed probe surfaces as an error instead of degrading to "".
func declaredDigest(raw []byte) (string, error) {
	var probe struct {
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return "", fmt.Errorf("reliance panel digest probe: %w", err)
	}
	return probe.Digest, nil
}

func sha256Hex(encoded []byte) string {
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// LoadReliancePanel decodes the frozen capsule-backed TASK 065 profile
// projection strictly from its canonical eval/results file and enforces its
// sealed digest: sha256 over canonical JSON of the panel with Digest cleared —
// exactly how the reliance package seals its projections.
func LoadReliancePanel(path string) (reliancePanel, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return reliancePanel{}, fmt.Errorf("reliance panel read: %w", err)
	}
	var panel reliancePanel
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&panel); err != nil {
		return reliancePanel{}, fmt.Errorf("reliance panel decode: %w", err)
	}
	if panel.SchemaVersion != reliancePanelSchemaVersion {
		return reliancePanel{}, fmt.Errorf("reliance panel schema %q", panel.SchemaVersion)
	}
	if len(panel.Dimensions) == 0 || !panel.GlobalScoreProhibited {
		return reliancePanel{}, fmt.Errorf("reliance panel incomplete or global-score violating")
	}
	declared, digestErr := declaredDigest(raw)
	if digestErr != nil {
		return reliancePanel{}, digestErr
	}
	if declared == "" {
		return reliancePanel{}, fmt.Errorf("reliance panel digest missing")
	}
	panel.Digest = ""
	encoded, err := json.Marshal(panel)
	if err != nil {
		return reliancePanel{}, fmt.Errorf("reliance panel digest: %w", err)
	}
	if recomputed := sha256Hex(encoded); recomputed != declared {
		return reliancePanel{}, fmt.Errorf("reliance panel digest mismatch: recomputed %s want %s", recomputed, declared)
	}
	panel.Digest = declared
	return panel, nil
}

// CheckRelianceRequirements evaluates every declared requirement against the
// frozen panel's dimensions. A requirement passes only when at least one
// dimension matches map_term_id (+ term_kind when declared), status, and
// evidence level; failures explain the mismatch in factor/status/evidence
// terms. Panel incompatibility is an explicit failure, never a silent pass.
func CheckRelianceRequirements(panel reliancePanel, requirements []RelianceRequirement) []string {
	var fails []string
	for _, req := range requirements {
		matched := false
		for _, dim := range panel.Dimensions {
			if dim.MapTermID != req.MapTermID {
				continue
			}
			if req.TermKind != "" && dim.TermKind != req.TermKind {
				continue
			}
			if dim.Status == req.Status && dim.EvidenceLevel == req.EvidenceLevel {
				matched = true
				break
			}
		}
		if !matched {
			termKind := req.TermKind
			if termKind == "" {
				termKind = "any term kind"
			}
			fails = append(fails, fmt.Sprintf(
				"reliance %s (term_kind %s): no dimension with status %s at evidence level %s satisfies this requirement in the frozen TASK 065 panel (panel incompatible, unmeasured, or slicing scope mismatch)",
				req.MapTermID, termKind, req.Status, req.EvidenceLevel))
		}
	}
	return fails
}
