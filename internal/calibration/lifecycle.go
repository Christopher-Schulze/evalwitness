package calibration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// SplitRole is TASK 049 role.
type SplitRole string

const (
	RoleDevelopment SplitRole = "development"
	RoleCalibration SplitRole = "calibration"
	RoleTest        SplitRole = "test"
)

// Lifecycle binds fit/select/evaluate to TASK 049 splits without leakage.
type Lifecycle struct {
	FeatureSchema FeatureSchema `json:"feature_schema"`
	SplitDigest   string        `json:"split_digest"`
	StudyDigest   string        `json:"study_digest"`
}

// NewLifecycle creates a versioned lifecycle. Digests are hex SHA256 of split/study manifests.
func NewLifecycle(schema FeatureSchema, splitDigest, studyDigest string) (Lifecycle, error) {
	if len(schema.Keys) == 0 {
		return Lifecycle{}, fmt.Errorf("calibration: feature schema empty")
	}
	if len(splitDigest) != 64 || len(studyDigest) != 64 {
		return Lifecycle{}, fmt.Errorf("calibration: digests must be hex SHA256")
	}
	return Lifecycle{FeatureSchema: schema, SplitDigest: splitDigest, StudyDigest: studyDigest}, nil
}

// Digest computes hex SHA256 of canonical JSON. Marshal cannot fail for this struct; error is checked per repo rule.
func (l Lifecycle) Digest() string {
	b, err := json.Marshal(l)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// NewLifecycleFromFile loads a Lifecycle from an explicit path containing a Study record JSON.
// The file must contain hex SHA256 digests for split and study; schema is spec-pinned study-manifest.v1/locked-study/record/split.
// Test fixtures should live next to the test file, not under eval/governance.
func NewLifecycleFromFile(path string, schema FeatureSchema) (Lifecycle, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Lifecycle{}, fmt.Errorf("calibration: read lifecycle file: %w", err)
	}
	var raw struct {
		SplitDigest string `json:"split_digest"`
		StudyDigest string `json:"study_digest"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Lifecycle{}, fmt.Errorf("calibration: decode lifecycle file: %w", err)
	}
	return NewLifecycle(schema, raw.SplitDigest, raw.StudyDigest)
}

// ValidateSplit enforces that every observation's SplitRole matches expectedRole and TaskID is present.
// This prevents calibration fit on test data or test evaluation on calibration data.
func ValidateSplit(observations []Observation, expectedRole SplitRole) error {
	if len(observations) == 0 {
		return fmt.Errorf("calibration: empty %s split", expectedRole)
	}
	for i, o := range observations {
		if o.TaskID == "" {
			return fmt.Errorf("calibration: missing task_id at index %d", i)
		}
		if o.SplitRole != expectedRole {
			return fmt.Errorf("calibration: observation %q has role %q, expected %q (TASK 049 leakage)", o.ID, o.SplitRole, expectedRole)
		}
	}
	return nil
}
