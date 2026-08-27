package calibration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

var forbiddenFeatureKeys = map[string]bool{
	"task_id": true, "won": true, "reward": true, "benchmark_reward": true, "label": true,
}

func SealModelArtifact(artifact ModelArtifact, calibrator any) (ModelArtifact, error) {
	if err := validateFeatureSchema(artifact.FeatureSchema); err != nil {
		return ModelArtifact{}, err
	}
	if strings.TrimSpace(artifact.RouteScope) == "" || strings.TrimSpace(artifact.DomainScope) == "" {
		return ModelArtifact{}, fmt.Errorf("calibration: route_scope and domain_scope required")
	}
	if len(artifact.TrainingDigest) != 64 {
		return ModelArtifact{}, fmt.Errorf("calibration: training_manifest_digest must be SHA-256 hex")
	}
	digest, err := checksumCalibrator(calibrator)
	if err != nil {
		return ModelArtifact{}, err
	}
	artifact.SchemaVersion = SchemaVersion
	artifact.CalibratorDigest = digest
	return artifact, nil
}

func VerifyModelArtifact(artifact ModelArtifact, calibrator any) error {
	sealed, err := SealModelArtifact(ModelArtifact{
		FeatureSchema:  artifact.FeatureSchema,
		TrainingDigest: artifact.TrainingDigest,
		RouteScope:     artifact.RouteScope,
		DomainScope:    artifact.DomainScope,
		EvidencePolicy: artifact.EvidencePolicy,
		PromptVersion:  artifact.PromptVersion,
		TopKContract:   artifact.TopKContract,
		ModelType:      artifact.ModelType,
	}, calibrator)
	if err != nil {
		return err
	}
	if sealed.CalibratorDigest != artifact.CalibratorDigest {
		return fmt.Errorf("calibration: calibrator checksum mismatch")
	}
	return nil
}

func SealModelArtifactBytes(artifact ModelArtifact, calibratorRaw []byte) (ModelArtifact, error) {
	return SealModelArtifact(artifact, calibratorPayload(bytes.TrimSpace(calibratorRaw)))
}

func VerifyModelArtifactBytes(artifact ModelArtifact, calibratorRaw []byte) error {
	return VerifyModelArtifact(artifact, calibratorPayload(bytes.TrimSpace(calibratorRaw)))
}

func MatchApplicability(artifact ModelArtifact, route, domain string) Applicability {
	if route != artifact.RouteScope {
		return Applicability{Applicable: false, Reason: "route_mismatch", RouteScope: artifact.RouteScope, DomainScope: artifact.DomainScope}
	}
	if domain != artifact.DomainScope {
		return Applicability{Applicable: false, Reason: "domain_mismatch", RouteScope: artifact.RouteScope, DomainScope: artifact.DomainScope}
	}
	return Applicability{Applicable: true, RouteScope: artifact.RouteScope, DomainScope: artifact.DomainScope}
}

func validateFeatureSchema(schema FeatureSchema) error {
	if len(schema.Keys) == 0 {
		return fmt.Errorf("calibration: feature schema empty")
	}
	for _, key := range schema.Keys {
		if forbiddenFeatureKeys[key] {
			return fmt.Errorf("calibration: feature %q is leakage", key)
		}
	}
	return nil
}

func checksumCalibrator(value any) (string, error) {
	if typed, ok := value.(calibratorPayload); ok {
		sum := sha256.Sum256(typed)
		return hex.EncodeToString(sum[:]), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("calibration: encode checksum: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

type calibratorPayload []byte
