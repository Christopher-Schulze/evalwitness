package registry

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const EmpiricalIndexSchemaVersion = "evalwitness.registry-empirical-index.v1"

type EmpiricalValidityIndex struct {
	SchemaVersion          string   `json:"schema_version"`
	OutcomeStatus          string   `json:"outcome_status"`
	OutcomeLedgerPresent   bool     `json:"outcome_ledger_present"`
	RelationValidityStatus string   `json:"relation_validity_status"`
	PackageInventoryDigest string   `json:"package_inventory_digest,omitempty"`
	HumanStudyStatus       string   `json:"human_study_status"`
	Empirical              bool     `json:"empirical"`
	Rankable               bool     `json:"rankable"`
	Limitations            []string `json:"limitations"`
	Digest                 string   `json:"digest"`
}

func IndexEmpiricalValidity(ownerInspectionPath string) (EmpiricalValidityIndex, error) {
	if ownerInspectionPath == "" {
		return EmpiricalValidityIndex{}, fmt.Errorf("empirical index: owner-inspection attestation path is required")
	}
	owner, err := IndexOwnerInspectionAttestation(ownerInspectionPath)
	if err != nil {
		return EmpiricalValidityIndex{}, err
	}
	index := EmpiricalValidityIndex{
		SchemaVersion:          EmpiricalIndexSchemaVersion,
		OutcomeStatus:          "not_run",
		OutcomeLedgerPresent:   false,
		RelationValidityStatus: owner.OverallStatus,
		PackageInventoryDigest: owner.PackageInventoryDigest,
		HumanStudyStatus:       owner.HumanStudyStatus,
		Empirical:              false,
		Rankable:               false,
		Limitations: []string{
			"TASK 057 outcome ledger is absent; status stays not_run",
			"TASK 068 relation validity is the public owner-inspection aggregate only",
			"empirical-not-run: no dual labels, no held-out outcome, no ranking",
		},
	}
	digest, err := protocol.Digest(unsignedEmpiricalIndex(index))
	if err != nil {
		return EmpiricalValidityIndex{}, err
	}
	index.Digest = digest
	return index, nil
}

func unsignedEmpiricalIndex(index EmpiricalValidityIndex) EmpiricalValidityIndex {
	index.Digest = ""
	return index
}
