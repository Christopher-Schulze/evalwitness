package stress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

type ReplayPairRequest struct {
	Original    verification.Input
	Transformed verification.Input
}

type ReplayPairEvidence struct {
	SchemaVersion          string                      `json:"schema_version"`
	CanonicalPolicy        string                      `json:"canonical_policy"`
	Entrypoint             string                      `json:"entrypoint"`
	EvidencePolicy         verification.EvidencePolicy `json:"evidence_policy"`
	OriginalInputDigest    string                      `json:"original_input_digest"`
	TransformedInputDigest string                      `json:"transformed_input_digest"`
	ReplaySource           provider.ExactReplaySource  `json:"replay_source"`
	BatchRunFingerprint    string                      `json:"batch_run_fingerprint"`
	Original               ReplaySide                  `json:"original"`
	Transformed            ReplaySide                  `json:"transformed"`
	Digest                 string                      `json:"digest"`
}

func (runner *ReplayFirstRunner) RunPairEvidence(ctx context.Context, request ReplayPairRequest) (ReplayPairEvidence, error) {
	if ctx == nil {
		return ReplayPairEvidence{}, errors.New("stress replay pair requires a context")
	}
	validated, err := snapshotReplayPairRequest(request)
	if err != nil {
		return ReplayPairEvidence{}, err
	}
	if err := validateReplayPairRequest(validated); err != nil {
		return ReplayPairEvidence{}, err
	}
	batch, err := runner.RunBatchEvidence(ctx, replayPairBatchRequest(validated))
	evidence, buildErr := replayPairEvidenceFromBatch(batch)
	return evidence, errors.Join(err, buildErr)
}

func snapshotReplayPairRequest(value ReplayPairRequest) (ReplayPairRequest, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ReplayPairRequest{}, fmt.Errorf("snapshot stress replay pair: %w", err)
	}
	var snapshot ReplayPairRequest
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return ReplayPairRequest{}, fmt.Errorf("snapshot stress replay pair: %w", err)
	}
	return snapshot, nil
}

func validateReplayPairRequest(request ReplayPairRequest) error {
	if err := validateReplayPairSide(request.Original, "original"); err != nil {
		return err
	}
	if err := validateReplayPairSide(request.Transformed, "transformed"); err != nil {
		return err
	}
	left, right := request.Original, request.Transformed
	left.Trajectories, right.Trajectories = nil, nil
	left.StudyVariant, right.StudyVariant = "", ""
	if !reflect.DeepEqual(left, right) {
		return errors.New("stress replay pair sides differ outside trajectories and study variant")
	}
	leftVariant := strings.TrimSpace(request.Original.StudyVariant)
	rightVariant := strings.TrimSpace(request.Transformed.StudyVariant)
	if (leftVariant == "") != (rightVariant == "") || leftVariant != "" && leftVariant == rightVariant {
		return errors.New("stress replay pair study variants must both be absent or both be distinct")
	}
	if leftVariant == "" && reflect.DeepEqual(request.Original.Trajectories, request.Transformed.Trajectories) {
		return errors.New("stress replay pair without study variants did not change trajectory inputs")
	}
	return nil
}

func validateReplayPairSide(input verification.Input, side string) error {
	if !input.DisableCache || input.AuthorizationDigest != "" || input.BudgetStatePath != "" {
		return fmt.Errorf("stress replay %s side must disable cache and omit live authorization and persistent budget state", side)
	}
	if input.StudyVariant != strings.TrimSpace(input.StudyVariant) {
		return fmt.Errorf("stress replay %s side study variant is not canonical", side)
	}
	return nil
}

func replayPairBatchRequest(request ReplayPairRequest) ReplayBatchRequest {
	return ReplayBatchRequest{Items: []ReplayBatchItemRequest{
		{Label: "original", Input: request.Original},
		{Label: "transformed", Input: request.Transformed},
	}}
}

func replayPairEvidenceFromBatch(batch ReplayBatchEvidence) (ReplayPairEvidence, error) {
	value := ReplayPairEvidence{
		SchemaVersion: ReplayPairEvidenceSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		Entrypoint: batch.Entrypoint, EvidencePolicy: batch.EvidencePolicy,
		ReplaySource: batch.ReplaySource, BatchRunFingerprint: batch.BatchRunFingerprint,
	}
	if err := batch.Validate(); err != nil {
		return value, err
	}
	if len(batch.Items) != 2 || batch.Items[0].Label != "original" || batch.Items[1].Label != "transformed" {
		return value, errors.New("stress replay pair batch does not contain the canonical two sides")
	}
	value.OriginalInputDigest, value.TransformedInputDigest = batch.Items[0].InputDigest, batch.Items[1].InputDigest
	value.Original, value.Transformed = batch.Items[0].Replay, batch.Items[1].Replay
	var err error
	value.Digest, err = replayPairEvidenceDigest(value)
	if err != nil {
		return value, err
	}
	return value, value.Validate()
}

func (value ReplayPairEvidence) Validate() error {
	if value.SchemaVersion != ReplayPairEvidenceSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validReplayEvidenceIdentity(value.Entrypoint, value.EvidencePolicy, value.BatchRunFingerprint) ||
		!validDigest(value.OriginalInputDigest) || !validDigest(value.TransformedInputDigest) ||
		value.OriginalInputDigest == value.TransformedInputDigest ||
		value.Original.PlanFingerprint == value.Transformed.PlanFingerprint {
		return errors.New("stress replay pair evidence identity is invalid")
	}
	if err := value.ReplaySource.Validate(); err != nil {
		return err
	}
	if err := validateReplayExecutionSide(value.Original, "original", value.Entrypoint, value.EvidencePolicy); err != nil {
		return err
	}
	if err := validateReplayExecutionSide(value.Transformed, "transformed", value.Entrypoint, value.EvidencePolicy); err != nil {
		return err
	}
	if !replaySidesBindSource(value.Original, value.Transformed, value.ReplaySource) {
		return errors.New("stress replay pair evidence does not bind one exact capture source")
	}
	digest, err := replayPairEvidenceDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("stress replay pair evidence digest is invalid")
	}
	return nil
}

func (value ReplayPairEvidence) ValidateRequest(request ReplayPairRequest) error {
	if err := value.Validate(); err != nil {
		return err
	}
	snapshot, err := snapshotReplayPairRequest(request)
	if err != nil {
		return err
	}
	if err := validateReplayPairRequest(snapshot); err != nil {
		return err
	}
	originalDigest, err := replayInputDigest(snapshot.Original)
	if err != nil {
		return err
	}
	transformedDigest, err := replayInputDigest(snapshot.Transformed)
	if err != nil || originalDigest != value.OriginalInputDigest || transformedDigest != value.TransformedInputDigest {
		return errors.New("stress replay pair evidence does not bind its requested input")
	}
	return nil
}

func replayInputDigest(value verification.Input) (string, error) {
	maxDuration := value.Limits.MaxDuration
	value.Limits.MaxDuration = 0
	return digestDocument(struct {
		Input                  verification.Input `json:"input"`
		MaxDurationNanoseconds int64              `json:"max_duration_nanoseconds"`
	}{value, int64(maxDuration)})
}

func replayPairEvidenceDigest(value ReplayPairEvidence) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
