package study

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DevelopmentInventorySchema = "evalwitness.development-data-inventory.v1"

type DevelopmentInventory struct {
	SchemaVersion string               `json:"schema_version"`
	Policy        string               `json:"policy"`
	Datasets      []DevelopmentDataset `json:"datasets"`
}

type DevelopmentDataset struct {
	ID                    string              `json:"id"`
	TaskCount             int                 `json:"task_count"`
	TaskIDsDigest         string              `json:"task_ids_digest"`
	ArtifactSetDigest     string              `json:"artifact_set_digest"`
	Role                  DataRole            `json:"role"`
	ConfirmationPermitted bool                `json:"confirmation_permitted"`
	Artifacts             []InventoryArtifact `json:"artifacts"`
}

type InventoryArtifact struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

func VerifyDevelopmentInventory(root string, inventory DevelopmentInventory) error {
	_, err := DevelopmentTaskIDs(root, inventory)
	return err
}

func DevelopmentTaskIDs(root string, inventory DevelopmentInventory) (map[string]struct{}, error) {
	if inventory.SchemaVersion != DevelopmentInventorySchema || strings.TrimSpace(inventory.Policy) == "" || len(inventory.Datasets) == 0 {
		return nil, errors.New("development inventory schema, policy, or datasets are invalid")
	}
	seenDatasetIDs := make(map[string]struct{}, len(inventory.Datasets))
	for _, dataset := range inventory.Datasets {
		if _, duplicate := seenDatasetIDs[dataset.ID]; duplicate {
			return nil, fmt.Errorf("duplicate inventory dataset ID %q", dataset.ID)
		}
		seenDatasetIDs[dataset.ID] = struct{}{}
	}
	taskIDs := make(map[string]struct{})
	for index, dataset := range inventory.Datasets {
		ids, err := inspectDevelopmentDataset(root, dataset)
		if err != nil {
			return nil, fmt.Errorf("inventory dataset %d: %w", index, err)
		}
		for id := range ids {
			taskIDs[id] = struct{}{}
		}
	}
	return taskIDs, nil
}

func inspectDevelopmentDataset(root string, dataset DevelopmentDataset) (map[string]struct{}, error) {
	if strings.TrimSpace(dataset.ID) == "" || dataset.TaskCount < 1 || !validDigest(dataset.TaskIDsDigest) || !validDigest(dataset.ArtifactSetDigest) || len(dataset.Artifacts) == 0 {
		return nil, errors.New("dataset identity, counts, digests, or artifacts are incomplete")
	}
	if dataset.Role != RoleDevelopment || dataset.ConfirmationPermitted {
		return nil, errors.New("previously accessed inventory data must remain development-only")
	}
	taskIDs := make(map[string]struct{})
	artifactRows := make([]string, 0, len(dataset.Artifacts))
	seenPaths := make(map[string]struct{}, len(dataset.Artifacts))
	for _, artifact := range dataset.Artifacts {
		if filepath.IsAbs(artifact.Path) || filepath.Clean(artifact.Path) != artifact.Path || strings.HasPrefix(artifact.Path, "..") || !validDigest(artifact.Digest) {
			return nil, fmt.Errorf("artifact %q path or digest is invalid", artifact.Path)
		}
		if _, duplicate := seenPaths[artifact.Path]; duplicate {
			return nil, fmt.Errorf("duplicate artifact path %q", artifact.Path)
		}
		seenPaths[artifact.Path] = struct{}{}
		raw, err := readInventoryArtifact(root, artifact.Path)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(raw)
		actualDigest := hex.EncodeToString(digest[:])
		if actualDigest != artifact.Digest {
			return nil, fmt.Errorf("artifact %q digest changed", artifact.Path)
		}
		var document struct {
			Details []struct {
				InstanceID string `json:"instance_id"`
				TaskName   string `json:"task_name"`
			} `json:"details"`
		}
		if err := json.Unmarshal(raw, &document); err != nil {
			return nil, fmt.Errorf("decode %q: %w", artifact.Path, err)
		}
		if len(document.Details) == 0 {
			return nil, fmt.Errorf("artifact %q has no task details", artifact.Path)
		}
		for _, detail := range document.Details {
			id := detail.InstanceID
			if id == "" {
				id = detail.TaskName
			}
			if id == "" {
				return nil, fmt.Errorf("artifact %q has a detail without task identity", artifact.Path)
			}
			taskIDs[id] = struct{}{}
		}
		artifactRows = append(artifactRows, artifact.Path+"\x00"+actualDigest)
	}
	if len(taskIDs) != dataset.TaskCount {
		return nil, fmt.Errorf("observed %d unique tasks, locked count is %d", len(taskIDs), dataset.TaskCount)
	}
	ids := make([]string, 0, len(taskIDs))
	for id := range taskIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	idsDigest := sha256.Sum256([]byte(strings.Join(ids, "\n") + "\n"))
	if hex.EncodeToString(idsDigest[:]) != dataset.TaskIDsDigest {
		return nil, errors.New("task ID inventory digest changed")
	}
	sort.Strings(artifactRows)
	artifactsDigest := sha256.Sum256([]byte(strings.Join(artifactRows, "\n") + "\n"))
	if hex.EncodeToString(artifactsDigest[:]) != dataset.ArtifactSetDigest {
		return nil, errors.New("artifact set inventory digest changed")
	}
	return taskIDs, nil
}

func readInventoryArtifact(root, relativePath string) ([]byte, error) {
	current := root
	for _, component := range strings.Split(relativePath, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("artifact path %q contains a symbolic link", relativePath)
		}
	}
	info, err := os.Lstat(current)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("artifact path %q is not a regular file", relativePath)
	}
	return os.ReadFile(current)
}
