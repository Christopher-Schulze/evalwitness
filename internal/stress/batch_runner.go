package stress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

type ReplayBatchItemRequest struct {
	Label string
	Input verification.Input
}

type ReplayBatchRequest struct {
	Items []ReplayBatchItemRequest
}

type ReplayBatchItemEvidence struct {
	Label        string     `json:"label"`
	StudyVariant string     `json:"study_variant,omitempty"`
	StudyCellID  string     `json:"study_cell_id,omitempty"`
	InputDigest  string     `json:"input_digest"`
	Replay       ReplaySide `json:"replay"`
}

type ReplayBatchEvidence struct {
	SchemaVersion       string                      `json:"schema_version"`
	CanonicalPolicy     string                      `json:"canonical_policy"`
	Entrypoint          string                      `json:"entrypoint"`
	EvidencePolicy      verification.EvidencePolicy `json:"evidence_policy"`
	ReplaySource        provider.ExactReplaySource  `json:"replay_source"`
	BatchRunFingerprint string                      `json:"batch_run_fingerprint"`
	Items               []ReplayBatchItemEvidence   `json:"items"`
	Digest              string                      `json:"digest"`
}

type replayBatchRun struct {
	request      ReplayBatchRequest
	batch        verification.BatchPlan
	result       verification.BatchResult
	observations []mode.ScoreObservation
	executionErr error
}

func (runner *ReplayFirstRunner) RunBatchEvidence(ctx context.Context, request ReplayBatchRequest) (ReplayBatchEvidence, error) {
	if ctx == nil {
		return ReplayBatchEvidence{}, errors.New("stress replay batch requires a context")
	}
	run, err := runner.executeReplayBatch(ctx, request)
	if err != nil {
		return ReplayBatchEvidence{}, err
	}
	evidence, evidenceErr := buildReplayBatchEvidence(run)
	if run.executionErr != nil || evidenceErr != nil {
		return evidence, errors.Join(run.executionErr, evidenceErr)
	}
	return evidence, nil
}

func (runner *ReplayFirstRunner) executeReplayBatch(ctx context.Context, request ReplayBatchRequest) (replayBatchRun, error) {
	validated, err := snapshotReplayBatchRequest(request)
	if err != nil {
		return replayBatchRun{}, err
	}
	if err := validateReplayBatchRequest(validated); err != nil {
		return replayBatchRun{}, err
	}
	batch, err := runner.executor.PlanBatch(replayBatchInputs(validated))
	if err != nil {
		return replayBatchRun{}, fmt.Errorf("plan stress replay batch: %w", err)
	}
	if batch.Authorization != nil {
		return replayBatchRun{}, errors.New("stress replay-first runner rejected a live-authorized verification plan")
	}
	collector := newScoreObservationCollector()
	result, executionErr := runner.executor.ExecuteBatch(mode.ContextWithScoreObserver(ctx, collector.Observe), batch)
	return replayBatchRun{validated, batch, result, collector.Snapshot(), executionErr}, nil
}

func snapshotReplayBatchRequest(value ReplayBatchRequest) (ReplayBatchRequest, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ReplayBatchRequest{}, fmt.Errorf("snapshot stress replay batch: %w", err)
	}
	var snapshot ReplayBatchRequest
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return ReplayBatchRequest{}, fmt.Errorf("snapshot stress replay batch: %w", err)
	}
	return snapshot, nil
}

func validateReplayBatchRequest(request ReplayBatchRequest) error {
	if len(request.Items) == 0 {
		return errors.New("stress replay batch requires at least one labeled input")
	}
	seenLabels := make(map[string]struct{}, len(request.Items))
	for _, item := range request.Items {
		if !identifierPattern.MatchString(item.Label) {
			return fmt.Errorf("stress replay batch label %q is invalid", item.Label)
		}
		if _, duplicate := seenLabels[item.Label]; duplicate {
			return fmt.Errorf("stress replay batch label %q is duplicated", item.Label)
		}
		seenLabels[item.Label] = struct{}{}
		if err := validateReplayBatchInput(item.Input, item.Label); err != nil {
			return err
		}
	}
	return validateSharedReplayBatchControls(request.Items)
}

func validateReplayBatchInput(input verification.Input, label string) error {
	if err := validateReplayPairSide(input, label); err != nil {
		return err
	}
	hasStudy := input.StudyManifestDigest != ""
	if hasStudy != (input.StudyVariant != "" && input.Lineage.StudyCellID != "") {
		return fmt.Errorf("stress replay batch item %q has an incomplete study identity", label)
	}
	return nil
}

func validateSharedReplayBatchControls(items []ReplayBatchItemRequest) error {
	want := replayBatchControlInput(items[0].Input)
	seenInputs := make(map[string]struct{}, len(items))
	for _, item := range items {
		if !reflect.DeepEqual(replayBatchControlInput(item.Input), want) {
			return errors.New("stress replay batch inputs differ outside trajectory, study variant, or study cell")
		}
		digest, err := replayInputDigest(item.Input)
		if err != nil {
			return err
		}
		if _, duplicate := seenInputs[digest]; duplicate {
			return errors.New("stress replay batch repeats one exact verification input")
		}
		seenInputs[digest] = struct{}{}
	}
	return nil
}

func replayBatchControlInput(value verification.Input) verification.Input {
	value.Trajectories = nil
	value.StudyVariant = ""
	value.Lineage.StudyCellID = ""
	return value
}

func replayBatchInputs(request ReplayBatchRequest) []verification.Input {
	result := make([]verification.Input, len(request.Items))
	for index, item := range request.Items {
		result[index] = item.Input
	}
	return result
}

func buildReplayBatchEvidence(run replayBatchRun) (ReplayBatchEvidence, error) {
	first := run.request.Items[0].Input
	value := ReplayBatchEvidence{
		SchemaVersion: ReplayBatchEvidenceSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		Entrypoint: first.Entrypoint, EvidencePolicy: first.Policy.Evidence,
		BatchRunFingerprint: run.batch.RunFingerprint,
	}
	if err := validateReplayBatchEvidence(replayBatchInputs(run.request), run.batch, run.result); err != nil {
		return value, err
	}
	byScope := scoreObservationsByScope(run.observations)
	if err := validateObservationScopes(byScope, run.batch.Plans); err != nil {
		return value, err
	}
	items, sides, err := buildReplayBatchItems(run.request, run.batch, run.result, byScope)
	if err != nil {
		return value, err
	}
	value.Items = items
	value.ReplaySource, err = sharedReplaySource(run.batch, sides...)
	if err != nil {
		return value, err
	}
	value.Digest, err = replayBatchEvidenceDigest(value)
	if err != nil {
		return value, err
	}
	return value, value.Validate()
}

func buildReplayBatchItems(request ReplayBatchRequest, batch verification.BatchPlan, result verification.BatchResult, observations map[string][]mode.ScoreObservation) ([]ReplayBatchItemEvidence, []ReplaySide, error) {
	items := make([]ReplayBatchItemEvidence, len(request.Items))
	sides := make([]ReplaySide, len(request.Items))
	for index, requested := range request.Items {
		plan := batch.Plans[index]
		side, err := buildReplaySide(requested.Label, plan, result.Results[index], observations[plan.RunFingerprint])
		if err != nil {
			return nil, nil, err
		}
		inputDigest, err := replayInputDigest(requested.Input)
		if err != nil {
			return nil, nil, err
		}
		sides[index] = side
		items[index] = ReplayBatchItemEvidence{
			Label: requested.Label, StudyVariant: requested.Input.StudyVariant,
			StudyCellID: requested.Input.Lineage.StudyCellID, InputDigest: inputDigest, Replay: side,
		}
	}
	return items, sides, nil
}

func (value ReplayBatchEvidence) Validate() error {
	if value.SchemaVersion != ReplayBatchEvidenceSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validReplayEvidenceIdentity(value.Entrypoint, value.EvidencePolicy, value.BatchRunFingerprint) || len(value.Items) == 0 {
		return errors.New("stress replay batch evidence identity or items are invalid")
	}
	if err := value.ReplaySource.Validate(); err != nil {
		return err
	}
	if err := validateReplayBatchItems(value); err != nil {
		return err
	}
	digest, err := replayBatchEvidenceDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("stress replay batch evidence digest is invalid")
	}
	return nil
}

func validateReplayBatchItems(value ReplayBatchEvidence) error {
	labels := make(map[string]struct{}, len(value.Items))
	inputs := make(map[string]struct{}, len(value.Items))
	sides := make([]ReplaySide, len(value.Items))
	for index, item := range value.Items {
		if !identifierPattern.MatchString(item.Label) || !validDigest(item.InputDigest) {
			return errors.New("stress replay batch item identity is invalid")
		}
		if _, duplicate := labels[item.Label]; duplicate {
			return errors.New("stress replay batch item label is duplicated")
		}
		if _, duplicate := inputs[item.InputDigest]; duplicate {
			return errors.New("stress replay batch item input is duplicated")
		}
		labels[item.Label], inputs[item.InputDigest], sides[index] = struct{}{}, struct{}{}, item.Replay
		if err := validateReplayExecutionSide(item.Replay, item.Label, value.Entrypoint, value.EvidencePolicy); err != nil {
			return err
		}
	}
	if !allReplaySidesBindSource(sides, value.ReplaySource) {
		return errors.New("stress replay batch evidence does not bind one exact capture source")
	}
	return nil
}

func allReplaySidesBindSource(sides []ReplaySide, source provider.ExactReplaySource) bool {
	for _, side := range sides {
		for _, observation := range side.Observations {
			if observation.ReplaySource == nil || *observation.ReplaySource != source {
				return false
			}
		}
	}
	return true
}

func (value ReplayBatchEvidence) ValidateRequest(request ReplayBatchRequest) error {
	if err := value.Validate(); err != nil {
		return err
	}
	snapshot, err := snapshotReplayBatchRequest(request)
	if err != nil {
		return err
	}
	if err := validateReplayBatchRequest(snapshot); err != nil {
		return err
	}
	if len(snapshot.Items) != len(value.Items) {
		return errors.New("stress replay batch evidence does not bind the requested item count")
	}
	for index, item := range snapshot.Items {
		digest, digestErr := replayInputDigest(item.Input)
		evidence := value.Items[index]
		if digestErr != nil || item.Label != evidence.Label || item.Input.StudyVariant != evidence.StudyVariant ||
			item.Input.Lineage.StudyCellID != evidence.StudyCellID || digest != evidence.InputDigest {
			return errors.New("stress replay batch evidence does not bind the requested inputs")
		}
	}
	return nil
}

func validReplayEvidenceIdentity(entrypoint string, policy verification.EvidencePolicy, fingerprint string) bool {
	return entrypoint != "" && slices.Contains(
		[]verification.EvidencePolicy{verification.EvidenceStrictVerifier, verification.EvidenceExplicitJudge}, policy,
	) && validDigest(fingerprint)
}

func replayBatchEvidenceDigest(value ReplayBatchEvidence) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
