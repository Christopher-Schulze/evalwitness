package calibration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

const FrozenSplitSchemaVersion = "evalwitness.identical-response-frozen-split.v1"

type FrozenSplitIndex struct {
	SplitDigest    string               `json:"split_digest"`
	StudyDigest    string               `json:"study_digest"`
	Tasks          map[string]SplitRole `json:"-"`
	PermittedRoles []SplitRole          `json:"permitted_roles"`
	HasTestRole    bool                 `json:"has_test_role"`
}

func fileDigest(path string) (string, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), raw, nil
}

const Bind049SchemaVersion = "evalwitness.calibration-049-bind.v1"

type Bind049Report struct {
	SchemaVersion  string      `json:"schema_version"`
	Lifecycle      Lifecycle   `json:"lifecycle"`
	PermittedRoles []SplitRole `json:"permitted_roles"`
	HasTestRole    bool        `json:"has_test_role"`
	TaskCount      int         `json:"task_count"`
	Limitations    []string    `json:"limitations"`
	Digest         string      `json:"digest"`
}

func Bind049ReportFromFiles(splitPath, studyPath string, schema FeatureSchema) (Bind049Report, error) {
	lifecycle, index, err := Bind049Lifecycle(splitPath, studyPath, schema)
	if err != nil {
		return Bind049Report{}, err
	}
	report := Bind049Report{
		SchemaVersion:  Bind049SchemaVersion,
		Lifecycle:      lifecycle,
		PermittedRoles: index.PermittedRoles,
		HasTestRole:    index.HasTestRole,
		TaskCount:      len(index.Tasks),
		Limitations: []string{
			"binds committed 049 split/study bytes only",
			"frozen identical-response split has no confirmatory test role",
			"not a held-out deployable calibration policy",
		},
	}
	encoded, err := json.Marshal(unsignedBind049Report(report))
	if err != nil {
		return Bind049Report{}, err
	}
	sum := sha256.Sum256(encoded)
	report.Digest = hex.EncodeToString(sum[:])
	return report, nil
}

func unsignedBind049Report(report Bind049Report) Bind049Report {
	report.Digest = ""
	return report
}

func Bind049Lifecycle(splitPath, studyPath string, schema FeatureSchema) (Lifecycle, FrozenSplitIndex, error) {
	splitDigest, splitRaw, err := fileDigest(splitPath)
	if err != nil {
		return Lifecycle{}, FrozenSplitIndex{}, fmt.Errorf("calibration: read frozen split: %w", err)
	}
	studyDigest, _, err := fileDigest(studyPath)
	if err != nil {
		return Lifecycle{}, FrozenSplitIndex{}, fmt.Errorf("calibration: read study record: %w", err)
	}
	index, err := decodeFrozenSplit(splitRaw, splitDigest, studyDigest)
	if err != nil {
		return Lifecycle{}, FrozenSplitIndex{}, err
	}
	lifecycle, err := NewLifecycle(schema, index.SplitDigest, index.StudyDigest)
	if err != nil {
		return Lifecycle{}, FrozenSplitIndex{}, err
	}
	return lifecycle, index, nil
}

func decodeFrozenSplit(raw []byte, splitDigest, studyDigest string) (FrozenSplitIndex, error) {
	var document struct {
		SchemaVersion string `json:"schema_version"`
		Assignments   []struct {
			Split   string   `json:"split"`
			TaskIDs []string `json:"task_ids"`
		} `json:"assignments"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return FrozenSplitIndex{}, fmt.Errorf("calibration: decode frozen split: %w", err)
	}
	if document.SchemaVersion != FrozenSplitSchemaVersion {
		return FrozenSplitIndex{}, fmt.Errorf("calibration: frozen split schema %q", document.SchemaVersion)
	}
	index := FrozenSplitIndex{
		SplitDigest: splitDigest,
		StudyDigest: studyDigest,
		Tasks:       map[string]SplitRole{},
	}
	roles := map[SplitRole]bool{}
	for _, assignment := range document.Assignments {
		role := SplitRole(assignment.Split)
		switch role {
		case RoleDevelopment, RoleCalibration, RoleTest:
		default:
			return FrozenSplitIndex{}, fmt.Errorf("calibration: unknown split role %q", assignment.Split)
		}
		roles[role] = true
		if role == RoleTest {
			index.HasTestRole = true
		}
		for _, taskID := range assignment.TaskIDs {
			if existing, ok := index.Tasks[taskID]; ok && existing != role {
				return FrozenSplitIndex{}, fmt.Errorf("calibration: task %q crosses split roles", taskID)
			}
			index.Tasks[taskID] = role
		}
	}
	for _, role := range []SplitRole{RoleDevelopment, RoleCalibration, RoleTest} {
		if roles[role] {
			index.PermittedRoles = append(index.PermittedRoles, role)
		}
	}
	if len(index.Tasks) == 0 {
		return FrozenSplitIndex{}, fmt.Errorf("calibration: frozen split has no tasks")
	}
	return index, nil
}

func (index FrozenSplitIndex) ValidateObservations(observations []Observation, expectedRole SplitRole) error {
	if err := ValidateSplit(observations, expectedRole); err != nil {
		return err
	}
	if expectedRole == RoleTest && !index.HasTestRole {
		return fmt.Errorf("calibration: frozen 049 split has no confirmatory test role")
	}
	for _, observation := range observations {
		role, ok := index.Tasks[observation.TaskID]
		if !ok {
			return fmt.Errorf("calibration: task %q is not in the frozen 049 split", observation.TaskID)
		}
		if role != expectedRole {
			return fmt.Errorf("calibration: task %q is %s in the frozen split, expected %s", observation.TaskID, role, expectedRole)
		}
	}
	return nil
}
