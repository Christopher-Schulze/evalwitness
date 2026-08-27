package calibration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/reliability"
)

// ReportArtifact is the versioned held-out calibration output.
type ReportArtifact struct {
	SchemaVersion    string               `json:"schema_version"`
	ModelType        string               `json:"model_type"`
	FeatureSchema    FeatureSchema        `json:"feature_schema"`
	Lifecycle        Lifecycle            `json:"lifecycle"`
	Metrics          *reliability.Metrics `json:"metrics,omitempty"`
	Selective        SelectiveMetrics     `json:"selective"`
	ObservationCount int                  `json:"observation_count"`
	TaskCount        int                  `json:"task_count"`
	Digest           string               `json:"digest"`
}

// BuildReport builds a held-out report from test observations using estimate.
// modelType is platt/isotonic/uncalibrated/legacy. lifecycle must be test-bound.
func BuildReport(modelType string, schema FeatureSchema, lifecycle Lifecycle, observations []Observation, threshold float64, estimate func(Observation) float64, seed uint64) (ReportArtifact, error) {
	if err := ValidateSplit(observations, RoleTest); err != nil {
		return ReportArtifact{}, err
	}
	if modelType == "" {
		return ReportArtifact{}, fmt.Errorf("calibration: model_type empty")
	}
	// Build reliability metrics from observations as reliability observations
	var relObs []reliability.Observation
	for _, o := range observations {
		if o.Won == nil {
			continue
		}
		relObs = append(relObs, reliability.Observation{
			ID:        o.ID,
			TaskID:    o.TaskID,
			Predicted: estimate(o),
			Won:       *o.Won,
		})
	}
	report := reliability.Analyze(relObs)
	sel := EvaluateSelectiveWithIntervals(observations, threshold, estimate, seed)

	taskSet := make(map[string]struct{})
	for _, o := range observations {
		taskSet[o.TaskID] = struct{}{}
	}

	art := ReportArtifact{
		SchemaVersion:    SchemaVersion,
		ModelType:        modelType,
		FeatureSchema:    schema,
		Lifecycle:        lifecycle,
		Metrics:          &report.Metrics,
		Selective:        sel,
		ObservationCount: len(observations),
		TaskCount:        len(taskSet),
	}
	// Deterministic digest over canonical fields without Digest itself
	tmp := struct {
		SchemaVersion string           `json:"schema_version"`
		ModelType     string           `json:"model_type"`
		FeatureSchema FeatureSchema    `json:"feature_schema"`
		Lifecycle     Lifecycle        `json:"lifecycle"`
		Selective     SelectiveMetrics `json:"selective"`
		TaskCount     int              `json:"task_count"`
	}{
		SchemaVersion: art.SchemaVersion,
		ModelType:     art.ModelType,
		FeatureSchema: art.FeatureSchema,
		Lifecycle:     art.Lifecycle,
		Selective:     art.Selective,
		TaskCount:     art.TaskCount,
	}
	b, err := json.Marshal(tmp)
	if err != nil {
		return ReportArtifact{}, fmt.Errorf("calibration: marshal report: %w", err)
	}
	h := sha256.Sum256(b)
	art.Digest = hex.EncodeToString(h[:])

	return art, nil
}
