package calibration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func DecodeObservations(raw []byte) ([]Observation, error) {
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	decoder.DisallowUnknownFields()
	var observations []Observation
	if err := decoder.Decode(&observations); err != nil {
		return nil, fmt.Errorf("calibration: decode observations: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("calibration: observations contain trailing JSON")
	}
	return observations, nil
}

func DecodeModelArtifact(raw []byte) (ModelArtifact, error) {
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	decoder.DisallowUnknownFields()
	var artifact ModelArtifact
	if err := decoder.Decode(&artifact); err != nil {
		return ModelArtifact{}, fmt.Errorf("calibration: decode model artifact: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ModelArtifact{}, fmt.Errorf("calibration: model artifact contains trailing JSON")
	}
	return artifact, nil
}

func EncodeDeploymentEvaluation(evaluation DeploymentEvaluation) ([]byte, error) {
	return encodeCalibrationJSON("deployment evaluation", evaluation)
}

func EncodeModelArtifact(artifact ModelArtifact) ([]byte, error) {
	return encodeCalibrationJSON("model artifact", artifact)
}

func encodeCalibrationJSON(kind string, value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("calibration: encode %s: %w", kind, err)
	}
	return append(raw, '\n'), nil
}
