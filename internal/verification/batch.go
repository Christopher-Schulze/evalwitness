package verification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"golang.org/x/sync/errgroup"
)

const BatchSchemaVersion = "evalwitness.verification-batch.v1"

type BatchPlan struct {
	SchemaVersion       string                  `json:"schema_version"`
	RunFingerprint      string                  `json:"run_fingerprint"`
	Plans               []Plan                  `json:"plans"`
	Requests            RequestPlan             `json:"requests"`
	Authorization       *mode.AuthorizationPlan `json:"authorization,omitempty"`
	AuthorizationDigest string                  `json:"authorization_digest,omitempty"`
}

type BatchResult struct {
	SchemaVersion  string              `json:"schema_version"`
	RunFingerprint string              `json:"run_fingerprint"`
	Results        []Result            `json:"results"`
	Budget         mode.BudgetSnapshot `json:"budget"`
	ServedModel    string              `json:"served_model,omitempty"`
	Lifecycle      Lifecycle           `json:"lifecycle"`
}

func (service *Service) PlanBatch(inputs []Input) (BatchPlan, error) {
	if len(inputs) == 0 {
		return BatchPlan{}, errors.New("verification batch requires at least one input")
	}
	requestedLimits := inputs[0].Limits
	entrypoint := strings.TrimSpace(inputs[0].Entrypoint)
	evidence := inputs[0].Policy.Evidence
	minDispatchIntervalSeconds := inputs[0].Policy.MinDispatchIntervalSeconds
	budgetStatePath := inputs[0].BudgetStatePath
	disableCache := inputs[0].DisableCache
	studyManifestDigest := strings.TrimSpace(inputs[0].StudyManifestDigest)
	authorizationDigest := strings.TrimSpace(inputs[0].AuthorizationDigest)
	plans := make([]Plan, len(inputs))
	for index, input := range inputs {
		if !reflect.DeepEqual(input.Limits, requestedLimits) {
			return BatchPlan{}, fmt.Errorf("verification batch input %d has different budget limits", index)
		}
		if strings.TrimSpace(input.Entrypoint) != entrypoint || input.Policy.Evidence != evidence ||
			input.Policy.MinDispatchIntervalSeconds != minDispatchIntervalSeconds ||
			input.BudgetStatePath != budgetStatePath || input.DisableCache != disableCache {
			return BatchPlan{}, fmt.Errorf("verification batch input %d has incompatible runtime settings", index)
		}
		if strings.TrimSpace(input.AuthorizationDigest) != authorizationDigest {
			return BatchPlan{}, fmt.Errorf("verification batch input %d has a different authorization digest", index)
		}
		if strings.TrimSpace(input.StudyManifestDigest) != studyManifestDigest {
			return BatchPlan{}, fmt.Errorf("verification batch input %d has a different study manifest digest", index)
		}
		plan, err := service.Plan(input)
		if err != nil {
			return BatchPlan{}, fmt.Errorf("plan verification batch input %d: %w", index, err)
		}
		plans[index] = plan
	}
	requests, err := aggregateRequestPlans(plans)
	if err != nil {
		return BatchPlan{}, err
	}
	limits := service.resolveLimits(requestedLimits, requests)
	maxWorkers := 1
	for index := range plans {
		plans[index].Input.Limits = limits
		if plans[index].Input.Policy.MaxWorkers > maxWorkers {
			maxWorkers = plans[index].Input.Policy.MaxWorkers
		}
		fingerprint, fingerprintErr := planFingerprint(
			plans[index].Input, plans[index].PreparedTextDigests, plans[index].TrajectoryEvidence, plans[index].Requests,
		)
		if fingerprintErr != nil {
			return BatchPlan{}, fingerprintErr
		}
		plans[index].RunFingerprint = fingerprint
		if !service.offline {
			authorization, authErr := service.authorizationFor(entrypoint, plans[index].Input.Policy.MaxWorkers, minDispatchIntervalSeconds, limits, plans[index].Requests, studyManifestDigest)
			if authErr != nil {
				return BatchPlan{}, authErr
			}
			plans[index].Authorization = &authorization
		}
	}
	batch := BatchPlan{
		SchemaVersion: BatchSchemaVersion, Plans: plans, Requests: requests,
		AuthorizationDigest: authorizationDigest,
	}
	if !service.offline {
		authorization, authErr := service.authorizationFor(entrypoint, maxWorkers, minDispatchIntervalSeconds, limits, requests, studyManifestDigest)
		if authErr != nil {
			return BatchPlan{}, authErr
		}
		batch.Authorization = &authorization
	}
	batch.RunFingerprint, err = batchFingerprint(batch)
	if err != nil {
		return BatchPlan{}, err
	}
	return batch, nil
}

func (service *Service) ExecuteBatch(ctx context.Context, batch BatchPlan) (result BatchResult, err error) {
	if err := service.validateBatchPlan(batch); err != nil {
		return BatchResult{}, err
	}
	if batch.Authorization != nil {
		if batch.AuthorizationDigest == "" {
			return BatchResult{}, &AuthorizationRequiredError{Plan: *batch.Authorization}
		}
		if err := batch.Authorization.Verify(batch.AuthorizationDigest); err != nil {
			return BatchResult{}, &AuthorizationRequiredError{Plan: *batch.Authorization, Cause: err}
		}
	}
	result = BatchResult{
		SchemaVersion: BatchSchemaVersion, RunFingerprint: batch.RunFingerprint,
		Results: make([]Result, len(batch.Plans)),
		Lifecycle: Lifecycle{
			RuntimeOpen: LifecycleFailed, Execution: LifecycleFailed, Cleanup: LifecycleFailed, Audit: LifecycleFailed,
		},
	}
	runtimePlan := batch.Plans[0]
	runtimePlan.RunFingerprint = batch.RunFingerprint
	runtimePlan.Requests = batch.Requests
	runtimePlan.Authorization = batch.Authorization
	runtime, err := service.openRuntime(ctx, runtimePlan)
	if err != nil {
		result.Lifecycle.Error = err.Error()
		return result, fmt.Errorf("open verification batch runtime: %w", err)
	}
	if runtime.Runner == nil {
		return result, errors.Join(
			errors.New("verification batch runtime returned no runner"),
			closeRuntimeResources(runtime, &result.Lifecycle),
			closeRuntimeAudit(runtime, &result.Lifecycle),
		)
	}
	result.Lifecycle.RuntimeOpen = LifecycleComplete
	group, groupCtx := errgroup.WithContext(ctx)
	maxWorkers := batch.Plans[0].Input.Policy.MaxWorkers
	for _, plan := range batch.Plans[1:] {
		if plan.Input.Policy.MaxWorkers > maxWorkers {
			maxWorkers = plan.Input.Policy.MaxWorkers
		}
	}
	group.SetLimit(maxWorkers)
	indexedErrors := make([]error, len(batch.Plans))
	for index, plan := range batch.Plans {
		index, plan := index, plan
		group.Go(func() error {
			item := Result{
				SchemaVersion: RunSchemaVersion, RunFingerprint: plan.RunFingerprint, Mode: plan.Input.Mode,
				Lifecycle: Lifecycle{
					RuntimeOpen: LifecycleComplete, Execution: LifecycleFailed, Cleanup: LifecycleFailed, Audit: LifecycleFailed,
				},
				CalibrationPolicy: DefaultCalibrationPolicy(),
				Fallback:          DefaultFallbackAccount(),
			}
			if cancellationErr := groupCtx.Err(); cancellationErr != nil {
				item.Lifecycle.Error = cancellationErr.Error()
				result.Results[index] = item
				indexedErrors[index] = fmt.Errorf("verification batch input %d: %w", index, cancellationErr)
				return cancellationErr
			}
			dispatchErr := service.dispatch(groupCtx, runtime.Runner, plan, &item)
			if dispatchErr == nil {
				item.Lifecycle.Execution = LifecycleComplete
			} else {
				item.Lifecycle.Error = dispatchErr.Error()
				indexedErrors[index] = fmt.Errorf("verification batch input %d: %w", index, dispatchErr)
			}
			if dispatchErr == nil && plan.Input.Policy.MinDispatchIntervalSeconds > 0 && index < len(batch.Plans)-1 {
				if waitErr := waitBatchDispatchInterval(groupCtx, time.Duration(plan.Input.Policy.MinDispatchIntervalSeconds)*time.Second); waitErr != nil {
					item.Lifecycle.Execution = LifecycleFailed
					item.Lifecycle.Error = waitErr.Error()
					result.Results[index] = item
					indexedErrors[index] = fmt.Errorf("verification batch input %d pacing: %w", index, waitErr)
					return waitErr
				}
			}
			result.Results[index] = item
			return dispatchErr
		})
	}
	dispatchErr := group.Wait()
	result.Budget = runtime.Runner.Budget.Snapshot()
	result.ServedModel = runtime.Runner.ServedModel()
	cleanupErr := closeRuntimeResources(runtime, &result.Lifecycle)
	for index := range result.Results {
		result.Results[index].Lifecycle.Cleanup = result.Lifecycle.Cleanup
	}
	var auditWriteErr error
	for index, plan := range batch.Plans {
		result.Results[index].Budget = result.Budget
		if auditErr := writeRunAudit(runtime.Runner, plan, result.Results[index], indexedErrors[index], cleanupErr); auditErr != nil {
			auditWriteErr = errors.Join(auditWriteErr, fmt.Errorf("verification batch input %d: %w", index, auditErr))
			result.Results[index].Lifecycle.Error = errors.Join(indexedErrors[index], auditErr).Error()
		} else {
			result.Results[index].Lifecycle.Audit = LifecycleComplete
		}
	}
	if auditWriteErr == nil {
		result.Lifecycle.Audit = LifecycleComplete
	}
	auditCloseErr := closeRuntimeAudit(runtime, &result.Lifecycle)
	if auditCloseErr != nil {
		for index := range result.Results {
			result.Results[index].Lifecycle.Audit = LifecycleFailed
		}
	}
	if dispatchErr == nil {
		result.Lifecycle.Execution = LifecycleComplete
	}
	err = errors.Join(errors.Join(indexedErrors...), cleanupErr, auditWriteErr, auditCloseErr)
	if err != nil {
		result.Lifecycle.Error = err.Error()
	}
	return result, err
}

func waitBatchDispatchInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (service *Service) validateBatchPlan(batch BatchPlan) error {
	if batch.SchemaVersion != BatchSchemaVersion || len(batch.Plans) == 0 {
		return errors.New("verification batch plan schema or inputs are invalid")
	}
	for index, plan := range batch.Plans {
		if err := service.validatePlan(plan); err != nil {
			return fmt.Errorf("verification batch plan input %d: %w", index, err)
		}
	}
	requests, err := aggregateRequestPlans(batch.Plans)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(requests, batch.Requests) {
		return errors.New("verification batch request set changed")
	}
	maxWorkers := 1
	for _, plan := range batch.Plans {
		if !reflect.DeepEqual(plan.Input.Limits, batch.Plans[0].Input.Limits) ||
			plan.Input.Entrypoint != batch.Plans[0].Input.Entrypoint || plan.Input.Policy.Evidence != batch.Plans[0].Input.Policy.Evidence ||
			plan.Input.Policy.MinDispatchIntervalSeconds != batch.Plans[0].Input.Policy.MinDispatchIntervalSeconds ||
			plan.Input.BudgetStatePath != batch.Plans[0].Input.BudgetStatePath || plan.Input.DisableCache != batch.Plans[0].Input.DisableCache ||
			plan.Input.StudyManifestDigest != batch.Plans[0].Input.StudyManifestDigest {
			return errors.New("verification batch runtime settings changed")
		}
		if strings.TrimSpace(plan.Input.AuthorizationDigest) != batch.AuthorizationDigest {
			return errors.New("verification batch authorization digest changed")
		}
		if plan.Input.Policy.MaxWorkers > maxWorkers {
			maxWorkers = plan.Input.Policy.MaxWorkers
		}
	}
	if service.offline {
		if batch.Authorization != nil {
			return errors.New("offline verification batch carries live authorization")
		}
	} else {
		expected, authErr := service.authorizationFor(
			batch.Plans[0].Input.Entrypoint, maxWorkers, batch.Plans[0].Input.Policy.MinDispatchIntervalSeconds,
			batch.Plans[0].Input.Limits, requests, batch.Plans[0].Input.StudyManifestDigest,
		)
		if authErr != nil {
			return authErr
		}
		if batch.Authorization == nil || !reflect.DeepEqual(expected, *batch.Authorization) {
			return errors.New("verification batch authorization changed")
		}
	}
	fingerprint, err := batchFingerprint(batch)
	if err != nil {
		return err
	}
	if fingerprint != batch.RunFingerprint {
		return errors.New("verification batch fingerprint changed")
	}
	return nil
}

func aggregateRequestPlans(plans []Plan) (RequestPlan, error) {
	aggregate := RequestPlan{}
	fingerprints := make(map[string]struct{})
	contracts := make(map[string]struct{})
	var routeID string
	for _, plan := range plans {
		for _, request := range plan.Requests.Requests {
			currentRouteID := request.RouteID()
			if routeID == "" {
				routeID = currentRouteID
			} else if currentRouteID != routeID {
				return RequestPlan{}, errors.New("verification batch spans multiple provider routes")
			}
			aggregate.Requests = append(aggregate.Requests, request)
		}
		for _, fingerprint := range plan.Requests.Fingerprints {
			fingerprints[fingerprint] = struct{}{}
		}
		contracts[plan.Requests.ContractDigest] = struct{}{}
		aggregate.WorstLogicalCalls += plan.Requests.WorstLogicalCalls
		if plan.Requests.MaximumInputTokens > aggregate.MaximumInputTokens {
			aggregate.MaximumInputTokens = plan.Requests.MaximumInputTokens
		}
		if plan.Requests.MaximumOutputTokens > aggregate.MaximumOutputTokens {
			aggregate.MaximumOutputTokens = plan.Requests.MaximumOutputTokens
		}
	}
	if len(aggregate.Requests) == 0 {
		return RequestPlan{}, errors.New("verification batch request set is empty")
	}
	aggregate.Fingerprints = sortedKeys(fingerprints)
	aggregate.SetFingerprint = digestOrderedStrings(aggregate.Fingerprints)
	aggregate.ContractDigest = digestOrderedStrings(sortedKeys(contracts))
	return aggregate, nil
}

func batchFingerprint(batch BatchPlan) (string, error) {
	planFingerprints := make([]string, len(batch.Plans))
	for index, plan := range batch.Plans {
		planFingerprints[index] = plan.RunFingerprint
	}
	type budgetMaterial struct {
		MaxCalls                int     `json:"max_calls"`
		MaxAttempts             int     `json:"max_attempts"`
		MaxEstimatedInputTokens int     `json:"max_estimated_input_tokens"`
		MaxReservedOutputTokens int     `json:"max_reserved_output_tokens"`
		MaxConcurrent           int     `json:"max_concurrent"`
		MaxCostUSD              float64 `json:"max_cost_usd"`
		MaxDurationNanoseconds  int64   `json:"max_duration_nanoseconds"`
	}
	limits := batch.Plans[0].Input.Limits
	material := struct {
		SchemaVersion    string         `json:"schema_version"`
		PlanFingerprints []string       `json:"plan_fingerprints"`
		RequestSet       string         `json:"request_set_fingerprint"`
		Contract         string         `json:"request_contract_digest"`
		Limits           budgetMaterial `json:"limits"`
	}{BatchSchemaVersion, planFingerprints, batch.Requests.SetFingerprint, batch.Requests.ContractDigest, budgetMaterial{
		limits.MaxCalls, limits.MaxAttempts, limits.MaxEstimatedInputTokens, limits.MaxReservedOutputTokens,
		limits.MaxConcurrent, limits.MaxCostUSD, int64(limits.MaxDuration),
	}}
	encoded, err := json.Marshal(material)
	if err != nil {
		return "", fmt.Errorf("encode verification batch plan: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
