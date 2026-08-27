package registry

import (
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const ScarcityIndexSchemaVersion = "evalwitness.registry-scarcity-index.v1"

const committedScarcityEvidenceDigest = "f401b84549b013f20deb9c78366f28b609fa7ff83cad330ac7e01617cd466d4f"

var committedScarcityParents = []struct {
	ID     string
	Digest string
}{
	{"corpus_development_plan", "5a7f4f55aafabd03ef9c802a9c48cf198a778228b40ade35e8f074df8d060c85"},
	{"natural_corpus_audit", "af0c0fd56fb498586096a8776e0d40794ee93acf5afda67cc000e576bfcef4d2"},
	{"controlled_corpus_release", "9b4999dafe2d37ea04c298b80a7aba0a1769755fdfd650cd01bf3a9cc31a2e42"},
	{"relation_governance_plan", "6eac462cae0a5b626561d5cbea274a5c3a72c78b6cb9d50a8952be6ccbb6fa8c"},
	{"relation_primary_sample", "6b721bcf0fb10e47923b46d92f3c14691cbd0dd98949ab0bf1dc016d8e1c1e43"},
	{"relation_scarcity_sentinel", "ec720a56394249a47eb4c0f7ef618471ce14c69ff8c2c13e1e851350c23e71fb"},
}

type ScarcityRegistryRecord struct {
	SchemaVersion  string   `json:"schema_version"`
	EvidenceSchema string   `json:"evidence_schema"`
	EvidenceDigest string   `json:"evidence_digest"`
	Attempted      int      `json:"attempted"`
	Admitted       int      `json:"admitted"`
	Rejected       int      `json:"rejected"`
	Development    int      `json:"development"`
	Calibration    int      `json:"calibration"`
	Test           int      `json:"test"`
	PackageFormat  string   `json:"package_format_commitment"`
	ParentDigests  []string `json:"parent_digests"`
	Rankable       bool     `json:"rankable"`
	VerifierScore  bool     `json:"verifier_score"`
	Limitations    []string `json:"limitations"`
	Digest         string   `json:"digest"`
}

func IndexScarcityNegativeEvidence(path string) (record ScarcityRegistryRecord, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return ScarcityRegistryRecord{}, fmt.Errorf("scarcity index: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("scarcity index: close source: %w", closeErr)
		}
	}()
	evidence, err := relation.DecodeScarcityPublicEvidence(file)
	if err != nil {
		return ScarcityRegistryRecord{}, fmt.Errorf("scarcity index: %w", err)
	}
	if err := evidence.Validate(); err != nil {
		return ScarcityRegistryRecord{}, fmt.Errorf("scarcity index: %w", err)
	}
	if evidence.Availability.Attempted != 198 || evidence.Availability.Admitted != 3 || evidence.Availability.Rejected != 195 {
		return ScarcityRegistryRecord{}, fmt.Errorf("scarcity index: committed funnel must be 198/3/195")
	}
	if evidence.StudyRoles.Development != 2 || evidence.StudyRoles.Calibration != 1 || evidence.StudyRoles.Test != 0 {
		return ScarcityRegistryRecord{}, fmt.Errorf("scarcity index: committed roles must be 2/1/0")
	}
	if evidence.Digest != committedScarcityEvidenceDigest {
		return ScarcityRegistryRecord{}, fmt.Errorf("scarcity index: evidence digest is not the committed public artifact")
	}
	parentDigests, err := verifyCommittedScarcityParents(evidence.Parents)
	if err != nil {
		return ScarcityRegistryRecord{}, err
	}
	record = ScarcityRegistryRecord{
		SchemaVersion:  ScarcityIndexSchemaVersion,
		EvidenceSchema: evidence.SchemaVersion,
		EvidenceDigest: evidence.Digest,
		Attempted:      evidence.Availability.Attempted,
		Admitted:       evidence.Availability.Admitted,
		Rejected:       evidence.Availability.Rejected,
		Development:    evidence.StudyRoles.Development,
		Calibration:    evidence.StudyRoles.Calibration,
		Test:           evidence.StudyRoles.Test,
		PackageFormat:  relation.PilotPackageFormatV5,
		ParentDigests:  parentDigests,
		Rankable:       false,
		VerifierScore:  false,
		Limitations: []string{
			"availability record only; never a verifier score, failed provider cell, or eighth relation family",
			"Markdown is a renderer-bound human view; registry values come from JSON",
			"held-out omitted-evidence validity stays unsupported",
		},
	}
	digest, err := protocol.Digest(unsignedScarcityRecord(record))
	if err != nil {
		return ScarcityRegistryRecord{}, err
	}
	record.Digest = digest
	return record, nil
}

func verifyCommittedScarcityParents(parents []relation.ScarcityPublicParent) ([]string, error) {
	if len(parents) != len(committedScarcityParents) {
		return nil, fmt.Errorf("scarcity index: public parent chain length %d", len(parents))
	}
	digests := make([]string, len(parents))
	for i, parent := range parents {
		expected := committedScarcityParents[i]
		if parent.ID != expected.ID || parent.Digest != expected.Digest {
			return nil, fmt.Errorf("scarcity index: parent %s is not the committed package-format-v5 chain", expected.ID)
		}
		digests[i] = parent.Digest
	}
	return digests, nil
}

func unsignedScarcityRecord(record ScarcityRegistryRecord) ScarcityRegistryRecord {
	record.Digest = ""
	return record
}
