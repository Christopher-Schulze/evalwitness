package registry

import (
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/audit"
	"github.com/Christopher-Schulze/evalwitness/internal/profile"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const MethodLineageSchemaVersion = "evalwitness.registry-method-lineage.v1"

type MethodLineageStage struct {
	StageID         string   `json:"stage_id"`
	From            string   `json:"from"`
	To              string   `json:"to"`
	Reason          string   `json:"reason"`
	Denominator     int      `json:"denominator"`
	DenominatorName string   `json:"denominator_name"`
	NonClaims       []string `json:"non_claims"`
}

type MethodLineageRecord struct {
	SchemaVersion string               `json:"schema_version"`
	Current       string               `json:"current"`
	Stages        []MethodLineageStage `json:"stages"`
	Rankable      bool                 `json:"rankable"`
	Pooled        bool                 `json:"pooled"`
	Limitations   []string             `json:"limitations"`
	Digest        string               `json:"digest"`
}

func RenderMethodLineage(view profile.AutopsyView) (MethodLineageRecord, error) {
	steps, err := profile.StepsFromAutopsyView(view)
	if err != nil {
		return MethodLineageRecord{}, fmt.Errorf("method lineage: %w", err)
	}
	auditSteps := make([]audit.MethodLineageStep, len(steps))
	stages := make([]MethodLineageStage, len(steps))
	for i, step := range steps {
		auditSteps[i] = audit.MethodLineageStep{
			From: step.From, To: step.To, Reason: step.Reason,
			Denominator: step.Denominator, NonClaims: append([]string(nil), step.NonClaims...),
		}
		stages[i] = MethodLineageStage{
			StageID:         fmt.Sprintf("%s-%s", step.From, step.To),
			From:            step.From,
			To:              step.To,
			Reason:          step.Reason,
			Denominator:     step.Denominator,
			DenominatorName: step.DenominatorName,
			NonClaims:       append([]string(nil), step.NonClaims...),
		}
	}
	if fails := audit.CheckMethodLineage(auditSteps, audit.MethodLineageRequirement{Current: view.MethodIntegrity.Current}); len(fails) > 0 {
		return MethodLineageRecord{}, fmt.Errorf("method lineage: %s", fails[0])
	}
	record := MethodLineageRecord{
		SchemaVersion: MethodLineageSchemaVersion,
		Current:       view.MethodIntegrity.Current,
		Stages:        stages,
		Rankable:      false,
		Pooled:        false,
		Limitations: []string{
			"two-break method history only; later passes never overwrite earlier failures",
			"incompatible generations are not pooled or inherited",
			"not a community ranking or admission upgrade",
		},
	}
	digest, err := protocol.Digest(unsignedMethodLineage(record))
	if err != nil {
		return MethodLineageRecord{}, err
	}
	record.Digest = digest
	return record, nil
}

func LoadMethodLineageView(path string) (profile.AutopsyView, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return profile.AutopsyView{}, fmt.Errorf("method lineage: %w", err)
	}
	var view profile.AutopsyView
	if err := protocol.DecodeStrict(raw, &view); err != nil {
		return profile.AutopsyView{}, fmt.Errorf("method lineage: %w", err)
	}
	return view, nil
}

func unsignedMethodLineage(record MethodLineageRecord) MethodLineageRecord {
	record.Digest = ""
	return record
}
