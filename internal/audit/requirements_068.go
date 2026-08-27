package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

// NegativeEvidenceRequirement is the TASK 068 integrity requirement: it pins
// the frozen availability funnel (198 attempted / 3 applied / 195 rejected),
// zero test-role cases, and the canonical scarcity evidence source. It fails
// when any value drifts or when a downstream artifact promotes scarcity into
// held-out, primary, human, or provider evidence — scarcity itself is never
// reported as a CI defect.
type NegativeEvidenceRequirement struct {
	AttemptedTarget int    `json:"attempted_target"`
	AppliedTarget   int    `json:"applied_target"`
	RejectedTarget  int    `json:"rejected_target"`
	EvidencePath    string `json:"evidence_path"`
}

// CheckNegativeEvidence loads the canonical typed scarcity evidence through
// profile.LoadNegativeEvidence (which reuses relation.DecodeScarcityPublicEvidence:
// strict decode, schema/canonical-policy/parent/digest validation) and compares
// every pinned field.
func CheckNegativeEvidence(path string, req NegativeEvidenceRequirement) []string {
	if req.EvidencePath == "" {
		req.EvidencePath = path
	}
	projection, err := profile.LoadNegativeEvidence(req.EvidencePath)
	if err != nil {
		return []string{fmt.Sprintf("negative-evidence integrity: %v", err)}
	}
	var fails []string
	if projection.Attempts != req.AttemptedTarget {
		fails = append(fails, fmt.Sprintf("negative-evidence: attempted %d != frozen target %d", projection.Attempts, req.AttemptedTarget))
	}
	if projection.Applied != req.AppliedTarget {
		fails = append(fails, fmt.Sprintf("negative-evidence: applied %d != frozen target %d", projection.Applied, req.AppliedTarget))
	}
	if projection.Rejected != req.RejectedTarget {
		fails = append(fails, fmt.Sprintf("negative-evidence: rejected %d != frozen target %d", projection.Rejected, req.RejectedTarget))
	}
	dims, err := profile.LoadRelationScarcity(req.EvidencePath)
	if err != nil {
		return append(fails, fmt.Sprintf("negative-evidence scarcity: %v", err))
	}
	// Belt-and-braces: relation.validateScarcityRoles already rejects
	// StudyRoles.Test != 0 at decode time (a sealed nonzero-test file is
	// unconstructible), so this loop only guards against a future loader
	// regression silently re-enabling promotion.
	for _, dim := range dims {
		if !dim.TestZero {
			fails = append(fails, "negative-evidence: test-role sentinel cases are nonzero; held-out validity promotion detected")
		}
	}
	return fails
}

// OwnerInspectionRequirement is the TASK 068 claim-safe owner-inspection
// readiness requirement. It accepts only the closed public attestation states
// and forbids translating owner pass into human-supported construct validity.
type OwnerInspectionRequirement struct {
	AttestationPath string `json:"attestation_path"`
	RequirePassed   bool   `json:"require_passed"`
}

// ownerAttestationStatus extracts the closed public attestation state from
// its schema_version + status fields with DisallowUnknownFields. The full
// relation.DecodeOwnerInspectionPublicAttestation validation runs in the
// relation command surface; this audit view reads only public status fields.
func ownerAttestationStatus(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var probe struct {
		SchemaVersion string `json:"schema_version"`
		Status        string `json:"status"`
	}
	if err := decodeStrictBytes(raw, &probe); err != nil {
		return "", err
	}
	return probe.Status, nil
}

// CheckOwnerInspectionRequirement reads only the closed public attestation's
// status fields — never private journal bytes.
func CheckOwnerInspectionRequirement(req OwnerInspectionRequirement) (string, []string) {
	status, err := ownerAttestationStatus(req.AttestationPath)
	if err != nil {
		return "", []string{fmt.Sprintf("owner-inspection: %v", err)}
	}
	switch {
	case status == "":
		return "", []string{"owner-inspection: attestation status missing"}
	case status == "passed" && !req.RequirePassed:
		// Passed is only a local pre-review custody gate; not required here but valid.
		return status, nil
	case status == "revision_required":
		return status, []string{"owner-inspection: revision required by owner inspection"}
	case status == "unresolved":
		return status, []string{"owner-inspection: owner inspection unresolved"}
	case req.RequirePassed && status != "passed":
		return status, []string{fmt.Sprintf("owner-inspection: policy requires passed readiness, got %s", status)}
	}
	return status, nil
}

// MethodLineageRequirement pins both falsification transitions and the current
// admitted generation via profile.StepsFromAutopsyView.
type MethodLineageRequirement struct {
	Current string `json:"current"`
}

// CheckMethodLineage validates the ladder shape: three generations, two
// transitions with reasons and denominators, current edge to v3, non-empty
// scientific non-claims per generation. Historical v1/v2 failures remain
// passing preserved evidence; they are not defects.
func CheckMethodLineage(steps []MethodLineageStep, req MethodLineageRequirement) []string {
	var fails []string
	if len(steps) != 3 {
		fails = append(fails, fmt.Sprintf("method-lineage: expected 3 ladder steps (v1->v2, v2->v3, v3->frozen), got %d", len(steps)))
		return fails
	}
	if steps[0].From != "v1" || steps[0].To != "v2" || steps[1].From != "v2" || steps[1].To != "v3" || steps[2].From != "v3" || steps[2].To != "frozen" {
		fails = append(fails, "method-lineage: ladder must be v1->v2, v2->v3, v3->frozen")
	}
	for i, step := range steps {
		if step.Reason == "" {
			fails = append(fails, fmt.Sprintf("method-lineage: transition %d supersession reason empty", i))
		}
		if step.Denominator <= 0 {
			fails = append(fails, fmt.Sprintf("method-lineage: transition %d denominator missing", i))
		}
		if len(step.NonClaims) == 0 {
			fails = append(fails, fmt.Sprintf("method-lineage: transition %d carries no scientific non-claims", i))
		}
	}
	if req.Current != "" {
		// Current names the admitted generation (v3); the ladder must end at
		// its frozen state.
		if req.Current != "v3" || steps[1].To != "v3" || steps[2].To != "frozen" {
			fails = append(fails, fmt.Sprintf("method-lineage: current edge must end at the frozen %s state", req.Current))
		}
	}
	return fails
}

// MethodLineageStep mirrors profile.SelfFalsificationStep without importing
// internal/profile into call sites that already hold the view types.
type MethodLineageStep struct {
	From        string
	To          string
	Reason      string
	Denominator int
	NonClaims   []string
}

func decodeStrictBytes(raw []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}
