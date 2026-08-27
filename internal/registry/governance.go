package registry

import (
	"fmt"
	"strings"
)

const (
	EvidenceStatusFormatVerified          = "format_verified"
	EvidenceStatusCapsuleVerified         = "capsule_verified"
	EvidenceStatusIndependentlyReproduced = "independently_reproduced"
	EvidenceStatusExpired                 = "expired"
	EvidenceStatusWithdrawn               = "withdrawn"
	EvidenceStatusDisputed                = "disputed"
	GovernanceActionDispute               = "dispute"
	GovernanceActionCorrection            = "correction"
	GovernanceActionWithdrawal            = "withdrawal"
	ContributorModeNamed                  = "named"
	ContributorModeAnonymous              = "anonymous"
)

type EvidenceStatus string

type FreshnessClass string

const (
	FreshnessCurrent FreshnessClass = "current"
	FreshnessExpired FreshnessClass = "expired"
	FreshnessReplay  FreshnessClass = "replay"
)

type ContributorRecord struct {
	ContributorID string `json:"contributor_id"`
	Mode          string `json:"mode"`
	Credit        string `json:"credit"`
}

type DisputeRecord struct {
	EntryID string `json:"entry_id"`
	Actor   string `json:"actor"`
	Reason  string `json:"reason"`
}

type CorrectionRecord struct {
	PriorEntryID string `json:"prior_entry_id"`
	NewEntryID   string `json:"new_entry_id"`
	Actor        string `json:"actor"`
	Reason       string `json:"reason"`
}

type WithdrawalRecord struct {
	EntryID string `json:"entry_id"`
	Actor   string `json:"actor"`
	Reason  string `json:"reason"`
}

type SchemaMigration struct {
	FromSchema int    `json:"from_schema"`
	ToSchema   int    `json:"to_schema"`
	Rewrites   bool   `json:"rewrites_signed_history"`
	Reason     string `json:"reason"`
}

type GovernanceAction struct {
	Action         string `json:"action"`
	EntryID        string `json:"entry_id"`
	Actor          string `json:"actor"`
	Reason         string `json:"reason"`
	PriorStatus    string `json:"prior_status"`
	NextStatus     string `json:"next_status"`
	RewriteHistory bool   `json:"rewrite_history"`
}

func ValidateContributor(record ContributorRecord) error {
	if strings.TrimSpace(record.ContributorID) == "" {
		return fmt.Errorf("governance: contributor_id required")
	}
	if record.Mode != ContributorModeNamed && record.Mode != ContributorModeAnonymous {
		return fmt.Errorf("governance: contributor mode must be named or anonymous")
	}
	return rejectUnsafeIntakeText("credit", record.Credit)
}

func ValidateGovernanceAction(action GovernanceAction) error {
	if action.RewriteHistory {
		return fmt.Errorf("governance: signed history cannot be rewritten")
	}
	if strings.TrimSpace(action.EntryID) == "" || strings.TrimSpace(action.Actor) == "" || strings.TrimSpace(action.Reason) == "" {
		return fmt.Errorf("governance: entry_id, actor, and reason are required")
	}
	if err := rejectUnsafeIntakeText("reason", action.Reason); err != nil {
		return err
	}
	switch action.Action {
	case GovernanceActionDispute:
		if action.NextStatus != EvidenceStatusDisputed {
			return fmt.Errorf("governance: dispute next_status must be disputed")
		}
	case GovernanceActionWithdrawal:
		if action.NextStatus != EvidenceStatusWithdrawn {
			return fmt.Errorf("governance: withdrawal next_status must be withdrawn")
		}
	case GovernanceActionCorrection:
		if action.NextStatus != EvidenceStatusFormatVerified {
			return fmt.Errorf("governance: correction must emit a new format_verified entry")
		}
	default:
		return fmt.Errorf("governance: unknown action %q", action.Action)
	}
	return nil
}

func ValidateSchemaMigration(migration SchemaMigration) error {
	if migration.Rewrites {
		return fmt.Errorf("governance: schema migration cannot rewrite signed history")
	}
	if migration.FromSchema < 1 || migration.ToSchema < migration.FromSchema {
		return fmt.Errorf("governance: schema migration range is invalid")
	}
	return nil
}

func IntakeWritableStatus(status string) bool {
	return status == EvidenceStatusFormatVerified
}
