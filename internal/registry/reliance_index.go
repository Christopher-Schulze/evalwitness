package registry

import (
	"fmt"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const RelianceIndexSchemaVersion = "evalwitness.registry-reliance-index.v1"

type RelianceIndexMember struct {
	EntryID       string `json:"entry_id"`
	CapsuleDigest string `json:"capsule_digest"`
	ServedModel   string `json:"served_model"`
	EvidenceLevel string `json:"evidence_level"`
}

type RelianceIndexCell struct {
	OntologyDigest     string                `json:"ontology_digest"`
	PanelDigest        string                `json:"panel_digest"`
	EstimatorDigest    string                `json:"estimator_digest"`
	InterventionDigest string                `json:"intervention_digest"`
	OutcomeDigest      string                `json:"outcome_digest"`
	ProfileDigest      string                `json:"profile_digest"`
	Entries            []RelianceIndexMember `json:"entries"`
}

type RelianceIndex struct {
	SchemaVersion string              `json:"schema_version"`
	Cells         []RelianceIndexCell `json:"cells"`
	Omitted       []string            `json:"omitted_entry_ids"`
	Limitations   []string            `json:"limitations"`
	Digest        string              `json:"digest"`
}

func RenderRelianceIndex(entries []IntakeEntry) (RelianceIndex, error) {
	validator := NewIntakeValidator()
	for _, entry := range entries {
		if err := validator.Add(entry); err != nil {
			return RelianceIndex{}, err
		}
	}
	groups := map[string]*RelianceIndexCell{}
	omitted := []string{}
	for _, entry := range entries {
		if !entry.hasRelianceParents() {
			omitted = append(omitted, entry.EntryID)
			continue
		}
		key := relianceCompatibilityKey(entry)
		cell := groups[key]
		if cell == nil {
			cell = &RelianceIndexCell{
				OntologyDigest:     entry.RelianceOntologyDigest,
				PanelDigest:        entry.ReliancePanelDigest,
				EstimatorDigest:    entry.RelianceEstimatorDigest,
				InterventionDigest: entry.RelianceInterventionDigest,
				OutcomeDigest:      entry.RelianceOutcomeDigest,
				ProfileDigest:      entry.RelianceProfileDigest,
			}
			groups[key] = cell
		}
		cell.Entries = append(cell.Entries, RelianceIndexMember{
			EntryID:       entry.EntryID,
			CapsuleDigest: entry.CapsuleDigest,
			ServedModel:   entry.ServedModel,
			EvidenceLevel: entry.EvidenceLevel,
		})
	}
	sort.Strings(omitted)
	index := RelianceIndex{
		SchemaVersion: RelianceIndexSchemaVersion,
		Cells:         make([]RelianceIndexCell, 0, len(groups)),
		Omitted:       omitted,
		Limitations: []string{
			"TASK 065 parents group only identical ontology/panel/estimator/intervention/outcome/profile versions",
			"incompatible cells stay separate; no ranking, pooling, or provider score",
			"not a signed community registry or held-out reliance result",
		},
	}
	for _, cell := range groups {
		sort.Slice(cell.Entries, func(i, j int) bool { return cell.Entries[i].EntryID < cell.Entries[j].EntryID })
		index.Cells = append(index.Cells, *cell)
	}
	sort.Slice(index.Cells, func(i, j int) bool {
		return relianceCellKey(index.Cells[i]) < relianceCellKey(index.Cells[j])
	})
	digest, err := protocol.Digest(unsignedRelianceIndex(index))
	if err != nil {
		return RelianceIndex{}, err
	}
	index.Digest = digest
	return index, nil
}

func relianceCellKey(cell RelianceIndexCell) string {
	return fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s", cell.OntologyDigest, cell.PanelDigest, cell.EstimatorDigest, cell.InterventionDigest, cell.OutcomeDigest, cell.ProfileDigest)
}

func unsignedRelianceIndex(index RelianceIndex) RelianceIndex {
	index.Digest = ""
	return index
}
