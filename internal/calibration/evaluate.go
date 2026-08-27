package calibration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const DeploymentEvaluationSchemaVersion = "evalwitness.calibration-deployment-evaluation.v1"

const (
	DeploymentStatusDeployable  = "deployable"
	DeploymentStatusUnsupported = "unsupported"
)

type DeploymentEvaluation struct {
	SchemaVersion string              `json:"schema_version"`
	Threshold     float64             `json:"threshold"`
	TargetRisk    float64             `json:"target_risk"`
	MinCoverage   float64             `json:"min_coverage"`
	Seed          uint64              `json:"seed"`
	Selective     SelectiveMetrics    `json:"selective"`
	FiniteSample  FiniteSampleControl `json:"finite_sample"`
	Deployable    bool                `json:"deployable"`
	Status        string              `json:"status"`
	Applicability *Applicability      `json:"applicability,omitempty"`
	Digest        string              `json:"digest"`
}

func EvaluateDeployment(observations []Observation, threshold, targetRisk, minCoverage float64, seed uint64) (DeploymentEvaluation, error) {
	if err := ValidateSplit(observations, RoleTest); err != nil {
		return DeploymentEvaluation{}, err
	}
	selective := EvaluateSelectiveWithIntervals(observations, threshold, func(o Observation) float64 {
		return o.Predicted
	}, seed)
	deployable := IsDeployable(selective, targetRisk, minCoverage)
	evaluation := DeploymentEvaluation{
		SchemaVersion: DeploymentEvaluationSchemaVersion,
		Threshold:     threshold,
		TargetRisk:    targetRisk,
		MinCoverage:   minCoverage,
		Seed:          seed,
		Selective:     selective,
		FiniteSample: EvaluateFiniteSample(observations, threshold, func(o Observation) float64 {
			return o.Predicted
		}),
		Deployable: deployable,
	}
	if deployable {
		evaluation.Status = DeploymentStatusDeployable
	} else {
		evaluation.Status = DeploymentStatusUnsupported
	}
	digest, err := deploymentEvaluationDigest(evaluation)
	if err != nil {
		return DeploymentEvaluation{}, err
	}
	evaluation.Digest = digest
	return evaluation, nil
}

func EvaluateDeploymentScoped(observations []Observation, threshold, targetRisk, minCoverage float64, seed uint64, artifact ModelArtifact, route, domain string) (DeploymentEvaluation, error) {
	evaluation, err := EvaluateDeployment(observations, threshold, targetRisk, minCoverage, seed)
	if err != nil {
		return DeploymentEvaluation{}, err
	}
	applicability := MatchApplicability(artifact, route, domain)
	evaluation.Applicability = &applicability
	if !applicability.Applicable {
		evaluation.Deployable = false
		evaluation.Status = DeploymentStatusUnsupported
	}
	digest, err := deploymentEvaluationDigest(evaluation)
	if err != nil {
		return DeploymentEvaluation{}, err
	}
	evaluation.Digest = digest
	return evaluation, nil
}

func deploymentEvaluationDigest(evaluation DeploymentEvaluation) (string, error) {
	evaluation.Digest = ""
	raw, err := json.Marshal(evaluation)
	if err != nil {
		return "", fmt.Errorf("calibration: encode deployment evaluation: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
