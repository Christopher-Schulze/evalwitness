package registry

import (
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const ReviewChecklistSchemaVersion = "evalwitness.registry-review-checklist.v1"

type ReviewChecklistItem struct {
	ItemID   string `json:"item_id"`
	Required bool   `json:"required"`
	Command  string `json:"command"`
	Text     string `json:"text"`
}

type ReviewChecklist struct {
	SchemaVersion string                `json:"schema_version"`
	Items         []ReviewChecklistItem `json:"items"`
	Limitations   []string              `json:"limitations"`
	Digest        string                `json:"digest"`
}

func MaintainerReviewChecklist() (ReviewChecklist, error) {
	checklist := ReviewChecklist{
		SchemaVersion: ReviewChecklistSchemaVersion,
		Items: []ReviewChecklistItem{
			{ItemID: "template", Required: true, Command: "evalwitness registry template", Text: "start from the non-submittable skeleton; do not submit the template"},
			{ItemID: "preflight", Required: true, Command: "evalwitness registry preflight --entry FILE [--catalog FILE]", Text: "reject expiry, replayed challenge_nonce, and catalog duplicates"},
			{ItemID: "refresh", Required: true, Command: "evalwitness registry refresh --catalog FILE", Text: "classify the whole catalog offline; no live provider call"},
			{ItemID: "matrix", Required: true, Command: "evalwitness registry render-matrix --catalog FILE [--history FILE]", Text: "group compatible contracts without ranking; history is append-only"},
			{ItemID: "no-live", Required: true, Command: "", Text: "CI and review never read a provider key or issue a network inference request"},
			{ItemID: "no-rank", Required: true, Command: "", Text: "do not rank providers or promote owner-pass to independently_reproduced"},
			{ItemID: "status", Required: true, Command: "", Text: "intake status stays format_verified; community_validated stays false"},
			{ItemID: "seed", Required: true, Command: "scripts/audits/run-registry-seed-catalog.sh", Text: "refresh the contrasting development seed catalog; it is not community-validated"},
			{ItemID: "reliance", Required: true, Command: "evalwitness registry render-reliance --catalog FILE", Text: "group TASK 065 parents only when all six version digests match; never rank or pool"},
			{ItemID: "scarcity", Required: true, Command: "evalwitness registry index-scarcity --evidence FILE", Text: "index the committed scarcity JSON as a non-rankable availability record"},
			{ItemID: "owner-inspection", Required: true, Command: "evalwitness registry index-owner-inspection --attestation FILE", Text: "index the public owner-inspection attestation as readiness only; never promote owner-pass"},
			{ItemID: "method-lineage", Required: true, Command: "evalwitness registry render-method-lineage --autopsy FILE", Text: "render the two-break method ladder without ranking or pooling"},
			{ItemID: "empirical", Required: true, Command: "evalwitness registry index-empirical --attestation FILE", Text: "keep TASK 057 outcome not_run; do not invent an outcome ledger"},
			{ItemID: "inventory", Required: true, Command: "evalwitness registry inventory", Text: "treat configured presets as configured only; never as independently reproduced"},
			{ItemID: "derivative", Required: true, Command: "evalwitness registry public-derivative --entry FILE", Text: "publish only the mechanism_conformance derivative; omit credentials and local paths"},
		},
		Limitations: []string{
			"local maintainer checklist only; not community admission or signed CI",
		},
	}
	digest, err := protocol.Digest(unsignedReviewChecklist(checklist))
	if err != nil {
		return ReviewChecklist{}, err
	}
	checklist.Digest = digest
	return checklist, nil
}

func unsignedReviewChecklist(checklist ReviewChecklist) ReviewChecklist {
	checklist.Digest = ""
	return checklist
}
