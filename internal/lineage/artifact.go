package lineage

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
)

const (
	SourceSchemaVersion      = "evalwitness.verification-lineage-source.v1"
	WitnessSchemaVersion     = "evalwitness.execution-witness.v1"
	CandidateSchemaVersion   = "evalwitness.verification-lineage-candidate.v1"
	AssessmentSchemaVersion  = "evalwitness.verification-lineage-assessment.v1"
	CapabilitySchemaVersion  = "evalwitness.trace-capability-vector.v1"
	AuditSchemaVersion       = "evalwitness.verification-lineage-audit.v1"
	BOMSchemaVersion         = "evalwitness.verification-evidence-bom.v1"
	ReleaseSchemaVersion     = "evalwitness.verification-lineage-release.v1"
	DatasetCardSchemaVersion = "evalwitness.verification-lineage-dataset-card.v1"
)

type ArtifactHeader struct {
	SchemaVersion   string      `json:"schema_version"`
	CanonicalPolicy string      `json:"canonical_policy"`
	ProtocolVersion string      `json:"protocol_version"`
	ObjectID        string      `json:"object_id"`
	TaskID          string      `json:"task_id"`
	TaskGroupID     string      `json:"task_group_id"`
	DataRole        DataRole    `json:"data_role"`
	PlanDigest      string      `json:"plan_digest"`
	Parents         []ParentRef `json:"parents"`
	Digest          string      `json:"digest"`
}

type ParentRef struct {
	Relation      string `json:"relation"`
	SchemaVersion string `json:"schema_version"`
	ObjectID      string `json:"object_id"`
	TaskID        string `json:"task_id"`
	TaskGroupID   string `json:"task_group_id"`
	Digest        string `json:"digest"`
}

type ParentRequirement struct {
	Relation       string   `json:"relation"`
	SchemaVersions []string `json:"schema_versions"`
	Minimum        int      `json:"minimum"`
	Maximum        int      `json:"maximum"`
	SameTask       bool     `json:"same_task"`
	SameTaskGroup  bool     `json:"same_task_group"`
}

type DocumentSummary struct {
	SchemaVersion string `json:"schema_version"`
	ObjectID      string `json:"object_id"`
	Valid         bool   `json:"valid"`
	Digest        string `json:"digest"`
}

func validateHeader(header ArtifactHeader, schemaVersion string, requirements []ParentRequirement) error {
	if header.SchemaVersion != schemaVersion || header.CanonicalPolicy != CanonicalPolicy || header.ProtocolVersion != ProtocolVersion {
		return errors.New("lineage artifact protocol identity is invalid")
	}
	if missing(header.ObjectID, header.TaskID, header.TaskGroupID) || header.TaskID != "TASK-069" || header.PlanDigest != LockedPlanDigest || !validDigest(header.Digest) {
		return errors.New("lineage artifact identity or digest is invalid")
	}
	if err := validateRoles([]DataRole{header.DataRole}, false); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(header.Parents))
	for _, parent := range header.Parents {
		key := parent.Relation + "\x00" + parent.ObjectID
		if missing(parent.Relation, parent.SchemaVersion, parent.ObjectID, parent.TaskID, parent.TaskGroupID) || !validDigest(parent.Digest) {
			return errors.New("lineage artifact contains an incomplete parent reference")
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate lineage parent %q", key)
		}
		seen[key] = struct{}{}
	}
	for _, parent := range header.Parents {
		if parent.Relation == "plan" && parent.Digest != LockedPlanDigest {
			return errors.New("lineage artifact references a non-locked plan")
		}
	}
	for _, requirement := range requirements {
		count := 0
		for _, parent := range header.Parents {
			if parent.Relation != requirement.Relation {
				continue
			}
			count++
			if !slices.Contains(requirement.SchemaVersions, parent.SchemaVersion) {
				return fmt.Errorf("parent relation %q has forbidden schema %q", parent.Relation, parent.SchemaVersion)
			}
			if requirement.SameTask && parent.TaskID != header.TaskID {
				return fmt.Errorf("parent relation %q crosses task identity", parent.Relation)
			}
			if requirement.SameTaskGroup && parent.TaskGroupID != header.TaskGroupID {
				return fmt.Errorf("parent relation %q crosses task-group identity", parent.Relation)
			}
		}
		if count < requirement.Minimum || (requirement.Maximum >= 0 && count > requirement.Maximum) {
			return fmt.Errorf("parent relation %q count %d is outside [%d,%d]", requirement.Relation, count, requirement.Minimum, requirement.Maximum)
		}
	}
	for _, parent := range header.Parents {
		if !slices.ContainsFunc(requirements, func(requirement ParentRequirement) bool { return requirement.Relation == parent.Relation }) {
			return fmt.Errorf("unexpected parent relation %q", parent.Relation)
		}
	}
	cursor := 0
	for _, requirement := range requirements {
		previousObjectID := ""
		for cursor < len(header.Parents) && header.Parents[cursor].Relation == requirement.Relation {
			if header.Parents[cursor].ObjectID <= previousObjectID {
				return fmt.Errorf("parent relation %q must be sorted by object ID", requirement.Relation)
			}
			previousObjectID = header.Parents[cursor].ObjectID
			cursor++
		}
	}
	if cursor != len(header.Parents) {
		return errors.New("lineage parents are not in canonical relation order")
	}
	return nil
}

func parentDigest(header ArtifactHeader, relation string) (string, bool) {
	parent, found := parentReference(header, relation)
	return parent.Digest, found
}

func parentReference(header ArtifactHeader, relation string) (ParentRef, bool) {
	for _, parent := range header.Parents {
		if parent.Relation == relation {
			return parent, true
		}
	}
	return ParentRef{}, false
}

func artifactDigest(value any) (string, error) {
	return digestJSON(value)
}

func validateArtifactDigest(actual string, withoutDigest any) error {
	expected, err := artifactDigest(withoutDigest)
	if err != nil {
		return err
	}
	if actual != expected {
		return errors.New("lineage artifact digest is invalid")
	}
	return nil
}

func validateSortedUnique(name string, values []string, minimum int) error {
	if len(values) < minimum {
		return fmt.Errorf("%s requires at least %d values", name, minimum)
	}
	if err := validateUniqueStrings(name, values, true); err != nil {
		return err
	}
	return nil
}

func validReleasePath(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && !strings.Contains(value, "\\") &&
		!strings.HasPrefix(value, "/") && path.Clean(value) == value && value != "." && !strings.HasPrefix(value, "../")
}
