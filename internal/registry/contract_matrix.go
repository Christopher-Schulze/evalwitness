package registry

import (
	"fmt"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const ContractMatrixSchemaVersion = "evalwitness.registry-contract-matrix.v1"

type ContractMatrixMember struct {
	EntryID       string `json:"entry_id"`
	CapsuleDigest string `json:"capsule_digest"`
	ServedModel   string `json:"served_model"`
	EvidenceLevel string `json:"evidence_level"`
}

type ContractMatrixCell struct {
	RequestContract string                 `json:"request_contract"`
	SchemaVersion   int                    `json:"schema_version"`
	EndpointKind    string                 `json:"endpoint_kind"`
	ScorePolicy     string                 `json:"score_policy"`
	TopLogprobs     int                    `json:"top_logprobs"`
	Entries         []ContractMatrixMember `json:"entries"`
}

type ContractMatrix struct {
	SchemaVersion string               `json:"schema_version"`
	ParentDigest  string               `json:"parent_digest,omitempty"`
	Cells         []ContractMatrixCell `json:"cells"`
	Limitations   []string             `json:"limitations"`
	Digest        string               `json:"digest"`
}

func RenderContractMatrix(entries []IntakeEntry) (ContractMatrix, error) {
	validator := NewIntakeValidator()
	for _, entry := range entries {
		if err := validator.Add(entry); err != nil {
			return ContractMatrix{}, err
		}
	}
	groups := map[string]*ContractMatrixCell{}
	for _, entry := range entries {
		key := fmt.Sprintf("%s\x1f%d\x1f%s\x1f%s\x1f%d", entry.RequestContract, entry.SchemaVersion, entry.EndpointKind, entry.ScorePolicy, entry.TopLogprobs)
		cell := groups[key]
		if cell == nil {
			cell = &ContractMatrixCell{
				RequestContract: entry.RequestContract,
				SchemaVersion:   entry.SchemaVersion,
				EndpointKind:    entry.EndpointKind,
				ScorePolicy:     entry.ScorePolicy,
				TopLogprobs:     entry.TopLogprobs,
			}
			groups[key] = cell
		}
		cell.Entries = append(cell.Entries, ContractMatrixMember{
			EntryID:       entry.EntryID,
			CapsuleDigest: entry.CapsuleDigest,
			ServedModel:   entry.ServedModel,
			EvidenceLevel: entry.EvidenceLevel,
		})
	}
	matrix := ContractMatrix{
		SchemaVersion: ContractMatrixSchemaVersion,
		Cells:         make([]ContractMatrixCell, 0, len(groups)),
		Limitations: []string{
			"intake-derived contract grouping only; no ranking or provider score",
			"not a signed community registry or TASK 050/058 capsule verify",
		},
	}
	for _, cell := range groups {
		sort.Slice(cell.Entries, func(i, j int) bool { return cell.Entries[i].EntryID < cell.Entries[j].EntryID })
		matrix.Cells = append(matrix.Cells, *cell)
	}
	sort.Slice(matrix.Cells, func(i, j int) bool {
		left := fmt.Sprintf("%s\x1f%d\x1f%s\x1f%s\x1f%d", matrix.Cells[i].RequestContract, matrix.Cells[i].SchemaVersion, matrix.Cells[i].EndpointKind, matrix.Cells[i].ScorePolicy, matrix.Cells[i].TopLogprobs)
		right := fmt.Sprintf("%s\x1f%d\x1f%s\x1f%s\x1f%d", matrix.Cells[j].RequestContract, matrix.Cells[j].SchemaVersion, matrix.Cells[j].EndpointKind, matrix.Cells[j].ScorePolicy, matrix.Cells[j].TopLogprobs)
		return left < right
	})
	digest, err := protocol.Digest(unsignedContractMatrix(matrix))
	if err != nil {
		return ContractMatrix{}, err
	}
	matrix.Digest = digest
	return matrix, nil
}

func MergeContractMatrix(previous ContractMatrix, entries []IntakeEntry) (ContractMatrix, error) {
	current, err := RenderContractMatrix(entries)
	if err != nil {
		return ContractMatrix{}, err
	}
	if previous.SchemaVersion == "" && len(previous.Cells) == 0 && previous.Digest == "" {
		return current, nil
	}
	if previous.SchemaVersion != ContractMatrixSchemaVersion {
		return ContractMatrix{}, fmt.Errorf("registry: history schema %q", previous.SchemaVersion)
	}
	unsigned := unsignedContractMatrix(previous)
	recomputed, err := protocol.Digest(unsigned)
	if err != nil {
		return ContractMatrix{}, err
	}
	if previous.Digest != recomputed {
		return ContractMatrix{}, fmt.Errorf("registry: history digest mismatch")
	}
	byKey := map[string]*ContractMatrixCell{}
	for _, cell := range current.Cells {
		copied := cell
		byKey[contractCellKey(copied)] = &copied
	}
	for _, cell := range previous.Cells {
		key := contractCellKey(cell)
		existing := byKey[key]
		if existing == nil {
			copied := cell
			byKey[key] = &copied
			continue
		}
		seen := map[string]bool{}
		for _, member := range existing.Entries {
			seen[member.EntryID] = true
		}
		for _, member := range cell.Entries {
			if seen[member.EntryID] {
				continue
			}
			existing.Entries = append(existing.Entries, member)
		}
	}
	merged := ContractMatrix{
		SchemaVersion: ContractMatrixSchemaVersion,
		ParentDigest:  previous.Digest,
		Cells:         make([]ContractMatrixCell, 0, len(byKey)),
		Limitations: []string{
			"intake-derived contract grouping only; no ranking or provider score",
			"not a signed community registry or TASK 050/058 capsule verify",
			"history is append-only; earlier members are never overwritten",
		},
	}
	for _, cell := range byKey {
		sort.Slice(cell.Entries, func(i, j int) bool { return cell.Entries[i].EntryID < cell.Entries[j].EntryID })
		merged.Cells = append(merged.Cells, *cell)
	}
	sort.Slice(merged.Cells, func(i, j int) bool { return contractCellKey(merged.Cells[i]) < contractCellKey(merged.Cells[j]) })
	digest, err := protocol.Digest(unsignedContractMatrix(merged))
	if err != nil {
		return ContractMatrix{}, err
	}
	merged.Digest = digest
	return merged, nil
}

func contractCellKey(cell ContractMatrixCell) string {
	return fmt.Sprintf("%s\x1f%d\x1f%s\x1f%s\x1f%d", cell.RequestContract, cell.SchemaVersion, cell.EndpointKind, cell.ScorePolicy, cell.TopLogprobs)
}

func unsignedContractMatrix(matrix ContractMatrix) ContractMatrix {
	matrix.Digest = ""
	return matrix
}
