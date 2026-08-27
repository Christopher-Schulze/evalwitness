package calibration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

const Bind034SchemaVersion = "evalwitness.calibration-034-bind.v1"

type Bind034Report struct {
	SchemaVersion         string    `json:"schema_version"`
	InventoryDigest       string    `json:"inventory_digest"`
	TaskCount             int       `json:"task_count"`
	ConfirmationPermitted bool      `json:"confirmation_permitted"`
	Role                  SplitRole `json:"role"`
	Limitations           []string  `json:"limitations"`
	Digest                string    `json:"digest"`
	tasks                 map[string]struct{}
}

func Bind034Development(root, inventoryPath string) (Bind034Report, error) {
	inventoryDigest, raw, err := fileDigest(inventoryPath)
	if err != nil {
		return Bind034Report{}, fmt.Errorf("calibration: read 034 inventory: %w", err)
	}
	var inventory study.DevelopmentInventory
	if err := study.DecodeStrict(bytes.NewReader(raw), &inventory); err != nil {
		return Bind034Report{}, fmt.Errorf("calibration: decode 034 inventory: %w", err)
	}
	tasks, err := study.DevelopmentTaskIDs(root, inventory)
	if err != nil {
		return Bind034Report{}, fmt.Errorf("calibration: verify 034 inventory: %w", err)
	}
	report := Bind034Report{
		SchemaVersion:         Bind034SchemaVersion,
		InventoryDigest:       inventoryDigest,
		TaskCount:             len(tasks),
		ConfirmationPermitted: false,
		Role:                  RoleDevelopment,
		Limitations: []string{
			"TASK 034 historical reliability artifacts are development evidence only",
			"confirmation_permitted=false; descendants inherit this role",
			"not a held-out deployable calibration policy",
		},
		tasks: tasks,
	}
	encoded, err := json.Marshal(unsignedBind034Report(report))
	if err != nil {
		return Bind034Report{}, err
	}
	sum := sha256.Sum256(encoded)
	report.Digest = hex.EncodeToString(sum[:])
	return report, nil
}

func (report Bind034Report) RejectConfirmatory(observations []Observation) error {
	if report.ConfirmationPermitted {
		return fmt.Errorf("calibration: 034 inventory cannot permit confirmation")
	}
	for _, observation := range observations {
		if _, known := report.tasks[observation.TaskID]; !known {
			continue
		}
		if observation.SplitRole != RoleDevelopment {
			return fmt.Errorf("calibration: TASK 034 task %q cannot leave development", observation.TaskID)
		}
	}
	return nil
}

func unsignedBind034Report(report Bind034Report) Bind034Report {
	report.Digest = ""
	report.tasks = nil
	return report
}

func Bind034ReportFromFiles(root, inventoryPath string) (Bind034Report, error) {
	if _, err := os.Stat(inventoryPath); err != nil {
		return Bind034Report{}, err
	}
	return Bind034Development(root, inventoryPath)
}

func Guard034Development(root, inventoryPath string, observations []Observation) error {
	if inventoryPath == "" {
		return nil
	}
	report, err := Bind034Development(root, inventoryPath)
	if err != nil {
		return err
	}
	return report.RejectConfirmatory(observations)
}
