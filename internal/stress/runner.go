package stress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"sync"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type VerificationExecutor interface {
	PlanBatch([]verification.Input) (verification.BatchPlan, error)
	ExecuteBatch(context.Context, verification.BatchPlan) (verification.BatchResult, error)
}

type ReplayRunRequest struct {
	Relation    Relation
	Admission   ConstructAdmission
	Original    verification.Input
	Transformed verification.Input
}

type ReplaySide struct {
	PlanFingerprint string                  `json:"plan_fingerprint"`
	Result          verification.Result     `json:"result"`
	Observations    []mode.ScoreObservation `json:"observations"`
	StageTrace      StageTrace              `json:"stage_trace"`
}

type ReplayExecution struct {
	SchemaVersion       string                      `json:"schema_version"`
	CanonicalPolicy     string                      `json:"canonical_policy"`
	RelationDigest      string                      `json:"relation_digest"`
	CaseID              string                      `json:"case_id"`
	Entrypoint          string                      `json:"entrypoint"`
	EvidencePolicy      verification.EvidencePolicy `json:"evidence_policy"`
	ReplaySource        provider.ExactReplaySource  `json:"replay_source"`
	BatchRunFingerprint string                      `json:"batch_run_fingerprint"`
	Original            ReplaySide                  `json:"original"`
	Transformed         ReplaySide                  `json:"transformed"`
	StageComparison     StageComparison             `json:"stage_comparison"`
	Digest              string                      `json:"digest"`
}

type ReplayFirstRunner struct {
	executor VerificationExecutor
}

func NewReplayFirstRunner(executor VerificationExecutor) (*ReplayFirstRunner, error) {
	if executor == nil {
		return nil, errors.New("stress replay runner requires a verification executor")
	}
	return &ReplayFirstRunner{executor: executor}, nil
}

func (runner *ReplayFirstRunner) RunMutation(ctx context.Context, request ReplayRunRequest) (ReplayExecution, error) {
	if ctx == nil {
		return ReplayExecution{}, errors.New("stress replay runner requires a context")
	}
	validated, err := snapshotReplayRunRequest(request)
	if err != nil {
		return ReplayExecution{}, err
	}
	if err := validateReplayRunRequest(validated); err != nil {
		return ReplayExecution{}, err
	}
	pair, err := runner.RunPairEvidence(ctx, ReplayPairRequest{
		Original: validated.Original, Transformed: validated.Transformed,
	})
	if err != nil {
		return ReplayExecution{}, err
	}
	return buildReplayExecution(validated, pair)
}

func snapshotReplayRunRequest(value ReplayRunRequest) (ReplayRunRequest, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ReplayRunRequest{}, fmt.Errorf("snapshot stress replay request: %w", err)
	}
	var snapshot ReplayRunRequest
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return ReplayRunRequest{}, fmt.Errorf("snapshot stress replay request: %w", err)
	}
	return snapshot, nil
}

func validateReplayRunRequest(request ReplayRunRequest) error {
	if err := request.Relation.Validate(); err != nil {
		return fmt.Errorf("validate stress replay relation: %w", err)
	}
	if request.Relation.Transform.Kind != TransformMutation || request.Relation.Transform.Version != mutation.MutationProgramVersionV3 {
		return errors.New("stress mutation replay requires one registered v3 mutation relation")
	}
	if err := request.Admission.Validate(); err != nil {
		return fmt.Errorf("validate stress replay admission: %w", err)
	}
	if request.Admission.Status == AdmissionHumanContradicted ||
		request.Relation.StatisticalFamily.Estimand == EstimandPrimaryCore && !request.Admission.PrimaryEligible ||
		request.Relation.StatisticalFamily.Estimand != EstimandPrimaryCore && !request.Admission.SensitivityEligible {
		return errors.New("stress replay admission is ineligible for the registered estimand")
	}
	if err := validateReplaySideInput(request.Relation, request.Admission, request.Original, "original"); err != nil {
		return err
	}
	if err := validateReplaySideInput(request.Relation, request.Admission, request.Transformed, "transformed"); err != nil {
		return err
	}
	if request.Original.Mode != request.Transformed.Mode || request.Original.Task != request.Transformed.Task ||
		!reflect.DeepEqual(request.Original.Criteria, request.Transformed.Criteria) || !reflect.DeepEqual(request.Original.Policy, request.Transformed.Policy) ||
		request.Original.Entrypoint != request.Transformed.Entrypoint {
		return errors.New("stress replay sides changed verification mode, task, criteria, policy, or entrypoint")
	}
	if reflect.DeepEqual(request.Original.Trajectories, request.Transformed.Trajectories) {
		return errors.New("stress replay transform did not change the trajectory inputs")
	}
	return nil
}

func validateReplaySideInput(relation Relation, admission ConstructAdmission, input verification.Input, side string) error {
	if !input.DisableCache || input.AuthorizationDigest != "" || input.BudgetStatePath != "" {
		return fmt.Errorf("stress replay %s side must disable cache and omit live authorization and persistent budget state", side)
	}
	if relation.Repeat.Kind == RepeatFixed && (input.Policy.UseSPRT || input.Policy.NReps != relation.Repeat.MaximumRepetitions) {
		return fmt.Errorf("stress replay %s side differs from the registered fixed repetition policy", side)
	}
	if relation.Repeat.Kind == RepeatRegisteredAdaptive && (!input.Policy.UseSPRT || input.Policy.SPRTParams.MinReps != relation.Repeat.MinimumRepetitions || input.Policy.SPRTParams.MaxReps != relation.Repeat.MaximumRepetitions) {
		return fmt.Errorf("stress replay %s side differs from the registered adaptive repetition policy", side)
	}
	if input.Lineage.AuditCaseID != admission.CaseID || input.Lineage.TransformationID != relation.ID {
		return fmt.Errorf("stress replay %s lineage does not bind the admitted case and relation", side)
	}
	count := len(input.Trajectories)
	if count < relation.Applicability.MinimumTrajectories || count > relation.Applicability.MaximumTrajectories {
		return fmt.Errorf("stress replay %s trajectory count %d is outside relation applicability", side, count)
	}
	return nil
}

func buildReplayExecution(request ReplayRunRequest, pair ReplayPairEvidence) (ReplayExecution, error) {
	execution := ReplayExecution{
		SchemaVersion: ReplayExecutionSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RelationDigest: request.Relation.Digest, CaseID: request.Admission.CaseID,
		Entrypoint: pair.Entrypoint, EvidencePolicy: pair.EvidencePolicy, ReplaySource: pair.ReplaySource,
		BatchRunFingerprint: pair.BatchRunFingerprint, Original: pair.Original, Transformed: pair.Transformed,
	}
	if err := pair.ValidateRequest(ReplayPairRequest{Original: request.Original, Transformed: request.Transformed}); err != nil {
		return execution, err
	}
	comparison, err := CompareStageTraces(request.Relation, pair.Original.StageTrace, pair.Transformed.StageTrace)
	if err != nil {
		return execution, err
	}
	execution.StageComparison = comparison
	execution.Digest, err = replayExecutionDigest(execution)
	if err != nil {
		return execution, err
	}
	return execution, nil
}

func validateReplayBatchEvidence(inputs []verification.Input, batch verification.BatchPlan, result verification.BatchResult) error {
	count := len(inputs)
	if batch.SchemaVersion != verification.BatchSchemaVersion || result.SchemaVersion != verification.BatchSchemaVersion ||
		!validDigest(batch.RunFingerprint) || result.RunFingerprint != batch.RunFingerprint || count == 0 ||
		len(batch.Plans) != count || len(result.Results) != count || batch.Authorization != nil || batch.AuthorizationDigest != "" {
		return errors.New("stress replay batch identity, cardinality, or offline authorization boundary is invalid")
	}
	if !completeLifecycle(result.Lifecycle) {
		return errors.New("stress replay batch lifecycle is incomplete")
	}
	seenFingerprints := make(map[string]struct{}, len(batch.Plans))
	for index, plan := range batch.Plans {
		item := result.Results[index]
		if plan.SchemaVersion != verification.RunSchemaVersion || !validDigest(plan.RunFingerprint) || plan.Authorization != nil ||
			len(plan.PreparedTextDigests) != len(plan.Input.Trajectories) || len(plan.TrajectoryEvidence) != len(plan.Input.Trajectories) || len(plan.Requests.Requests) == 0 ||
			item.SchemaVersion != verification.RunSchemaVersion || item.RunFingerprint != plan.RunFingerprint || item.Mode != plan.Input.Mode || !completeLifecycle(item.Lifecycle) {
			return fmt.Errorf("stress replay side %d plan, result, or lifecycle binding is invalid", index)
		}
		if _, exists := seenFingerprints[plan.RunFingerprint]; exists {
			return errors.New("stress replay sides collapsed to one verification plan fingerprint")
		}
		seenFingerprints[plan.RunFingerprint] = struct{}{}
		if !replayPlanBindsInput(plan.Input, inputs[index]) {
			return fmt.Errorf("stress replay side %d plan does not bind its requested input", index)
		}
	}
	return nil
}

func replayPlanBindsInput(planned, requested verification.Input) bool {
	return planned.Entrypoint == requested.Entrypoint && planned.Mode == requested.Mode && planned.Task == requested.Task &&
		reflect.DeepEqual(planned.Trajectories, requested.Trajectories) && reflect.DeepEqual(planned.Criteria, requested.Criteria) &&
		reflect.DeepEqual(planned.Policy, requested.Policy) && planned.AuthorizationDigest == "" && planned.BudgetStatePath == requested.BudgetStatePath &&
		planned.DisableCache && requested.DisableCache && reflect.DeepEqual(planned.Lineage, requested.Lineage)
}

func completeLifecycle(value verification.Lifecycle) bool {
	return value.RuntimeOpen == verification.LifecycleComplete && value.Execution == verification.LifecycleComplete &&
		value.Cleanup == verification.LifecycleComplete && value.Audit == verification.LifecycleComplete && value.Error == ""
}

func buildReplaySide(side string, plan verification.Plan, result verification.Result, observations []mode.ScoreObservation) (ReplaySide, error) {
	value := ReplaySide{PlanFingerprint: plan.RunFingerprint, Result: result, Observations: cloneScoreObservations(observations)}
	if len(observations) == 0 {
		return value, fmt.Errorf("stress replay %s side produced no score-call observations", side)
	}
	for _, observation := range observations {
		if observation.Validate() != nil || observation.Scope != plan.RunFingerprint ||
			observation.Entrypoint != plan.Input.Entrypoint || observation.ReplayStatus != provider.ReplayStatusExact ||
			observation.ExtractionStatus != mode.ExtractionObservationComplete {
			return value, fmt.Errorf("stress replay %s side contains non-exact or rejected score evidence", side)
		}
		if observation.ReplaySource == nil {
			return value, fmt.Errorf("stress replay %s side lacks validated capture provenance", side)
		}
	}
	trace, err := stageTraceFromReplay(side, plan, result, observations)
	if err != nil {
		return value, err
	}
	value.StageTrace = trace
	return value, nil
}

func sharedReplaySource(batch verification.BatchPlan, sides ...ReplaySide) (provider.ExactReplaySource, error) {
	var source provider.ExactReplaySource
	for _, side := range sides {
		for _, observation := range side.Observations {
			if observation.ReplaySource == nil || observation.ReplaySource.Validate() != nil {
				return provider.ExactReplaySource{}, errors.New("stress exact replay lacks validated capture provenance")
			}
			if source.SchemaVersion == "" {
				source = *observation.ReplaySource
				continue
			}
			if !reflect.DeepEqual(source, *observation.ReplaySource) {
				return provider.ExactReplaySource{}, errors.New("stress exact replay combines more than one capture source")
			}
		}
	}
	if err := source.Validate(); err != nil {
		return provider.ExactReplaySource{}, err
	}
	requestKeys := make(map[string]struct{})
	for _, side := range sides {
		for _, observation := range side.Observations {
			requestKeys[string(observation.RequestFingerprint)+"\x00"+observation.SamplingSlot] = struct{}{}
		}
	}
	if source.Records < len(requestKeys) {
		return provider.ExactReplaySource{}, errors.New("stress exact replay capture has fewer records than observed request and sampling-slot identities")
	}
	for _, plan := range batch.Plans {
		for _, request := range plan.Requests.Requests {
			if request.ProviderID != source.ProviderID || request.RouteID() != source.RouteID || request.RequestedModel != source.RequestedModel {
				return provider.ExactReplaySource{}, errors.New("stress exact replay capture source differs from the planned provider route or model")
			}
		}
	}
	return source, nil
}

type ingestionStageMaterial struct {
	PreparedTextDigests []string                       `json:"prepared_text_digests"`
	TrajectoryEvidence  []preprocess.AccountingSummary `json:"trajectory_evidence"`
}

type requestStageMaterial struct {
	Fingerprints   []string `json:"fingerprints"`
	SetFingerprint string   `json:"set_fingerprint"`
	ContractDigest string   `json:"contract_digest"`
}

type decisionStageMaterial struct {
	Policy    verification.Policy    `json:"policy"`
	Mode      verification.Mode      `json:"mode"`
	State     verifier.DecisionState `json:"state"`
	Delta     *mode.Verdict          `json:"delta,omitempty"`
	Absolute  *mode.Score            `json:"absolute,omitempty"`
	Selection *mode.Selection        `json:"selection,omitempty"`
}

func stageTraceFromReplay(side string, plan verification.Plan, result verification.Result, observations []mode.ScoreObservation) (StageTrace, error) {
	ingestion := ingestionStageMaterial{
		append([]string(nil), plan.PreparedTextDigests...), append([]preprocess.AccountingSummary(nil), plan.TrajectoryEvidence...),
	}
	requestMaterial := requestStageMaterial{
		append([]string(nil), plan.Requests.Fingerprints...), plan.Requests.SetFingerprint, plan.Requests.ContractDigest,
	}
	providerMaterial := providerObservationMaterial(observations)
	extractionMaterial := extractionObservationMaterial(observations)
	decisionMaterial := decisionStageMaterial{
		plan.Input.Policy, result.Mode, result.State, result.Delta, result.Absolute, result.Selection,
	}
	renderingMaterial := stableRenderingMaterial(result)
	records := make([]StageRecord, 0, len(orderedStages()))
	var err error
	if records, err = appendStageRecord(records, StageIngestion, "canonical-ingestion", ingestion); err != nil {
		return StageTrace{}, err
	}
	if records, err = appendStageRecord(records, StageRequestConstruction, "canonical-request-set", requestMaterial); err != nil {
		return StageTrace{}, err
	}
	if records, err = appendStageRecord(records, StageProviderResponse, "response-digest-set", providerMaterial); err != nil {
		return StageTrace{}, err
	}
	if records, err = appendStageRecord(records, StageScoreExtraction, "score-evidence-set", extractionMaterial); err != nil {
		return StageTrace{}, err
	}
	if records, err = appendStageRecord(records, StageDecisionPolicy, "verification-decision", decisionMaterial); err != nil {
		return StageTrace{}, err
	}
	if records, err = appendStageRecord(records, StageRendering, "verification-result", renderingMaterial); err != nil {
		return StageTrace{}, err
	}
	return SealStageTrace(side, records)
}

type stableBudgetSnapshot struct {
	Calls                int     `json:"calls"`
	Attempts             int     `json:"attempts"`
	EstimatedInputTokens int     `json:"estimated_input_tokens"`
	ReservedOutputTokens int     `json:"reserved_output_tokens"`
	PeakConcurrent       int     `json:"peak_concurrent"`
	EstimatedCostUSD     float64 `json:"estimated_cost_usd"`
}

type stableVerificationResult struct {
	SchemaVersion  string                 `json:"schema_version"`
	RunFingerprint string                 `json:"run_fingerprint"`
	Mode           verification.Mode      `json:"mode"`
	State          verifier.DecisionState `json:"state"`
	Delta          *mode.Verdict          `json:"delta,omitempty"`
	Absolute       *mode.Score            `json:"absolute,omitempty"`
	Selection      *mode.Selection        `json:"selection,omitempty"`
	Budget         stableBudgetSnapshot   `json:"budget"`
	Lifecycle      verification.Lifecycle `json:"lifecycle"`
}

func stableRenderingMaterial(result verification.Result) stableVerificationResult {
	return stableVerificationResult{
		SchemaVersion: result.SchemaVersion, RunFingerprint: result.RunFingerprint, Mode: result.Mode, State: result.State,
		Delta: result.Delta, Absolute: result.Absolute, Selection: result.Selection,
		Budget: stableBudgetSnapshot{
			Calls: result.Budget.Calls, Attempts: result.Budget.Attempts, EstimatedInputTokens: result.Budget.EstimatedInputTokens,
			ReservedOutputTokens: result.Budget.ReservedOutputTokens, PeakConcurrent: result.Budget.PeakConcurrent, EstimatedCostUSD: result.Budget.EstimatedCostUSD,
		},
		Lifecycle: result.Lifecycle,
	}
}

func appendStageRecord[T any](records []StageRecord, stage Stage, unit string, value T) ([]StageRecord, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode stress %s stage: %w", stage, err)
	}
	record, err := NewStageRecord(stage, unit, encoded)
	if err != nil {
		return nil, err
	}
	return append(records, record), nil
}

type providerStageObservation struct {
	CriterionID            string                `json:"criterion_id"`
	SamplingSlot           string                `json:"sampling_slot"`
	RequestFingerprint     provider.Fingerprint  `json:"request_fingerprint"`
	ResponseBodyDigest     string                `json:"response_body_digest"`
	ParsedPayloadDigest    string                `json:"parsed_payload_digest"`
	ResponseEvidenceDigest string                `json:"response_evidence_digest"`
	ReplayStatus           provider.ReplayStatus `json:"replay_status"`
	ProviderRequestID      string                `json:"provider_request_id,omitempty"`
}

type extractionStageObservation struct {
	CriterionID           string `json:"criterion_id"`
	SamplingSlot          string `json:"sampling_slot"`
	ScoreEvidenceDigest   string `json:"score_evidence_digest"`
	ExtractionStatus      string `json:"extraction_status"`
	ExtractionErrorDigest string `json:"extraction_error_digest,omitempty"`
}

func providerObservationMaterial(values []mode.ScoreObservation) []providerStageObservation {
	result := make([]providerStageObservation, len(values))
	for index, value := range values {
		result[index] = providerStageObservation{
			value.CriterionID, value.SamplingSlot, value.RequestFingerprint, value.ResponseBodyDigest,
			value.ParsedPayloadDigest, value.ResponseEvidenceDigest, value.ReplayStatus, value.ProviderRequestID,
		}
	}
	return result
}

func extractionObservationMaterial(values []mode.ScoreObservation) []extractionStageObservation {
	result := make([]extractionStageObservation, len(values))
	for index, value := range values {
		result[index] = extractionStageObservation{value.CriterionID, value.SamplingSlot, value.ScoreEvidenceDigest, value.ExtractionStatus, value.ExtractionErrorDigest}
	}
	return result
}

func scoreObservationsByScope(values []mode.ScoreObservation) map[string][]mode.ScoreObservation {
	result := make(map[string][]mode.ScoreObservation)
	for _, value := range values {
		result[value.Scope] = append(result[value.Scope], value)
	}
	for scope := range result {
		sortScoreObservations(result[scope])
	}
	return result
}

func validateObservationScopes(values map[string][]mode.ScoreObservation, plans []verification.Plan) error {
	expected := make(map[string]struct{}, len(plans))
	for _, plan := range plans {
		expected[plan.RunFingerprint] = struct{}{}
	}
	for scope := range values {
		if _, exists := expected[scope]; !exists {
			return fmt.Errorf("stress replay observed an unplanned execution scope %q", scope)
		}
	}
	return nil
}

func sortScoreObservations(values []mode.ScoreObservation) {
	slices.SortFunc(values, func(left, right mode.ScoreObservation) int {
		leftKey := left.CriterionID + "\x00" + left.SamplingSlot + "\x00" + string(left.RequestFingerprint) + "\x00" + left.ResponseEvidenceDigest + "\x00" + left.ScoreEvidenceDigest
		rightKey := right.CriterionID + "\x00" + right.SamplingSlot + "\x00" + string(right.RequestFingerprint) + "\x00" + right.ResponseEvidenceDigest + "\x00" + right.ScoreEvidenceDigest
		return compareStrings(leftKey, rightKey)
	})
}

func compareStrings(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func cloneScoreObservations(values []mode.ScoreObservation) []mode.ScoreObservation {
	result := append([]mode.ScoreObservation(nil), values...)
	for index := range result {
		if values[index].ReplaySource != nil {
			source := *values[index].ReplaySource
			result[index].ReplaySource = &source
		}
	}
	return result
}

type scoreObservationCollector struct {
	mu     sync.Mutex
	values []mode.ScoreObservation
}

func newScoreObservationCollector() *scoreObservationCollector {
	return &scoreObservationCollector{values: []mode.ScoreObservation{}}
}

func (collector *scoreObservationCollector) Observe(value mode.ScoreObservation) error {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	collector.values = append(collector.values, value)
	return nil
}

func (collector *scoreObservationCollector) Snapshot() []mode.ScoreObservation {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	values := cloneScoreObservations(collector.values)
	sort.Slice(values, func(left, right int) bool {
		if values[left].Scope != values[right].Scope {
			return values[left].Scope < values[right].Scope
		}
		leftKey := values[left].CriterionID + "\x00" + values[left].SamplingSlot + "\x00" + string(values[left].RequestFingerprint)
		rightKey := values[right].CriterionID + "\x00" + values[right].SamplingSlot + "\x00" + string(values[right].RequestFingerprint)
		return leftKey < rightKey
	})
	return values
}
