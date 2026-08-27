package mode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

const runBudgetStateSchemaVersion = 2

type BudgetLimits struct {
	MaxCalls                int           `json:"max_calls"`
	MaxAttempts             int           `json:"max_attempts"`
	MaxEstimatedInputTokens int           `json:"max_estimated_input_tokens"`
	MaxReservedOutputTokens int           `json:"max_reserved_output_tokens"`
	MaxConcurrent           int           `json:"max_concurrent"`
	MaxCostUSD              float64       `json:"max_cost_usd,omitempty"`
	MaxDuration             time.Duration `json:"-"`
}

type BudgetSnapshot struct {
	Calls                int     `json:"calls"`
	Attempts             int     `json:"attempts"`
	EstimatedInputTokens int     `json:"estimated_input_tokens"`
	ReservedOutputTokens int     `json:"reserved_output_tokens"`
	PeakConcurrent       int     `json:"peak_concurrent"`
	EstimatedCostUSD     float64 `json:"estimated_cost_usd"`
	ElapsedSeconds       float64 `json:"elapsed_seconds"`
}

type AttemptReservation struct {
	EstimatedInputTokens int
	MaxOutputTokens      int
	EstimatedCostUSD     float64
}

type AttemptActual struct {
	InputTokens   int
	OutputTokens  int
	CostUSD       *float64
	UsageObserved bool
}

type BudgetExceededError struct {
	Metric    string
	Used      float64
	Requested float64
	Limit     float64
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("run budget exceeded for %s: used %.4f + requested %.4f > limit %.4f", e.Metric, e.Used, e.Requested, e.Limit)
}

type RunBudget struct {
	mu        sync.Mutex
	limits    BudgetLimits
	started   time.Time
	deadline  time.Time
	usage     BudgetSnapshot
	statePath string
	attempts  chan struct{}
	active    int
}

type persistedBudgetLimits struct {
	MaxCalls                int     `json:"max_calls"`
	MaxAttempts             int     `json:"max_attempts"`
	MaxEstimatedInputTokens int     `json:"max_estimated_input_tokens"`
	MaxReservedOutputTokens int     `json:"max_reserved_output_tokens"`
	MaxConcurrent           int     `json:"max_concurrent"`
	MaxCostUSD              float64 `json:"max_cost_usd"`
	MaxDurationSeconds      int64   `json:"max_duration_seconds"`
}

type persistedBudgetState struct {
	SchemaVersion int                   `json:"schema_version"`
	StartedAt     time.Time             `json:"started_at"`
	Limits        persistedBudgetLimits `json:"limits"`
	Usage         BudgetSnapshot        `json:"usage"`
}

func NewRunBudget(limits BudgetLimits) *RunBudget {
	started := time.Now()
	budget := &RunBudget{limits: limits, started: started}
	if limits.MaxConcurrent > 0 {
		budget.attempts = make(chan struct{}, limits.MaxConcurrent)
	}
	if limits.MaxDuration > 0 {
		budget.deadline = started.Add(limits.MaxDuration)
	}
	return budget
}

func NewPersistentRunBudget(limits BudgetLimits, statePath string) (*RunBudget, error) {
	if statePath == "" {
		return nil, errors.New("persistent run budget requires a state path")
	}
	budget := NewRunBudget(limits)
	budget.statePath = statePath
	raw, err := os.ReadFile(statePath)
	if errors.Is(err, os.ErrNotExist) {
		if err := budget.persistLocked(); err != nil {
			return nil, err
		}
		return budget, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read persistent run budget: %w", err)
	}
	var state persistedBudgetState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("parse persistent run budget: %w", err)
	}
	if state.SchemaVersion != runBudgetStateSchemaVersion {
		return nil, fmt.Errorf("persistent run budget schema %d is unsupported", state.SchemaVersion)
	}
	if state.StartedAt.IsZero() {
		return nil, errors.New("persistent run budget has no start time")
	}
	if state.Limits != persistedLimits(limits) {
		return nil, fmt.Errorf("persistent run budget limits changed: stored %+v, requested %+v", state.Limits, persistedLimits(limits))
	}
	if err := validateRestoredBudget(state.Usage, limits); err != nil {
		return nil, err
	}
	budget.started = state.StartedAt
	budget.usage = state.Usage
	budget.usage.ElapsedSeconds = 0
	if limits.MaxDuration > 0 {
		budget.deadline = budget.started.Add(limits.MaxDuration)
	}
	if err := os.Chmod(statePath, safety.SensitiveFileMode); err != nil {
		return nil, fmt.Errorf("secure persistent run budget: %w", err)
	}
	return budget, nil
}

func (b *RunBudget) Reserve(estimatedInputTokens int, estimatedCostUSD float64) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.reserveCallLocked(); err != nil {
		return err
	}
	reservation := AttemptReservation{EstimatedInputTokens: estimatedInputTokens, EstimatedCostUSD: estimatedCostUSD}
	if err := b.reserveAttemptLocked(reservation); err != nil {
		b.usage.Calls--
		return err
	}
	if err := b.persistLocked(); err != nil {
		b.usage.Calls--
		b.usage.Attempts--
		b.usage.EstimatedInputTokens -= estimatedInputTokens
		b.usage.EstimatedCostUSD -= estimatedCostUSD
		return err
	}
	return nil
}

func (b *RunBudget) ReserveCall() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.reserveCallLocked(); err != nil {
		return err
	}
	if err := b.persistLocked(); err != nil {
		b.usage.Calls--
		return err
	}
	return nil
}

func (b *RunBudget) AcquireAttempt(ctx context.Context, reservation AttemptReservation) (func(AttemptActual) error, error) {
	if b == nil {
		return func(AttemptActual) error { return nil }, nil
	}
	if reservation.EstimatedInputTokens < 0 || reservation.MaxOutputTokens < 0 || reservation.EstimatedCostUSD < 0 {
		return nil, errors.New("attempt reservation values must be non-negative")
	}
	if b.attempts != nil {
		select {
		case b.attempts <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	releaseSlot := func() {
		if b.attempts != nil {
			<-b.attempts
		}
	}
	b.mu.Lock()
	if err := b.reserveAttemptLocked(reservation); err != nil {
		b.mu.Unlock()
		releaseSlot()
		return nil, err
	}
	b.active++
	if b.active > b.usage.PeakConcurrent {
		b.usage.PeakConcurrent = b.active
	}
	if err := b.persistLocked(); err != nil {
		b.active--
		b.rollbackAttemptLocked(reservation)
		b.mu.Unlock()
		releaseSlot()
		return nil, err
	}
	b.mu.Unlock()
	var once sync.Once
	var releaseErr error
	return func(actual AttemptActual) error {
		once.Do(func() {
			b.mu.Lock()
			b.active--
			if actual.UsageObserved {
				releaseErr = b.reconcileAttemptLocked(reservation, actual)
			}
			if persistErr := b.persistLocked(); persistErr != nil {
				releaseErr = errors.Join(releaseErr, persistErr)
			}
			b.mu.Unlock()
			releaseSlot()
		})
		return releaseErr
	}, nil
}

func (b *RunBudget) reserveCallLocked() error {
	if err := b.checkDeadlineLocked(); err != nil {
		return err
	}
	if b.limits.MaxCalls > 0 && b.usage.Calls+1 > b.limits.MaxCalls {
		return &BudgetExceededError{Metric: "calls", Used: float64(b.usage.Calls), Requested: 1, Limit: float64(b.limits.MaxCalls)}
	}
	b.usage.Calls++
	return nil
}

func (b *RunBudget) reserveAttemptLocked(reservation AttemptReservation) error {
	if err := b.checkDeadlineLocked(); err != nil {
		return err
	}
	if b.limits.MaxAttempts > 0 && b.usage.Attempts+1 > b.limits.MaxAttempts {
		return &BudgetExceededError{Metric: "attempts", Used: float64(b.usage.Attempts), Requested: 1, Limit: float64(b.limits.MaxAttempts)}
	}
	if b.limits.MaxEstimatedInputTokens > 0 && b.usage.EstimatedInputTokens+reservation.EstimatedInputTokens > b.limits.MaxEstimatedInputTokens {
		return &BudgetExceededError{Metric: "estimated_input_tokens", Used: float64(b.usage.EstimatedInputTokens), Requested: float64(reservation.EstimatedInputTokens), Limit: float64(b.limits.MaxEstimatedInputTokens)}
	}
	if b.limits.MaxReservedOutputTokens > 0 && b.usage.ReservedOutputTokens+reservation.MaxOutputTokens > b.limits.MaxReservedOutputTokens {
		return &BudgetExceededError{Metric: "reserved_output_tokens", Used: float64(b.usage.ReservedOutputTokens), Requested: float64(reservation.MaxOutputTokens), Limit: float64(b.limits.MaxReservedOutputTokens)}
	}
	if b.limits.MaxCostUSD > 0 && b.usage.EstimatedCostUSD+reservation.EstimatedCostUSD > b.limits.MaxCostUSD {
		return &BudgetExceededError{Metric: "estimated_cost_usd", Used: b.usage.EstimatedCostUSD, Requested: reservation.EstimatedCostUSD, Limit: b.limits.MaxCostUSD}
	}
	b.usage.Attempts++
	b.usage.EstimatedInputTokens += reservation.EstimatedInputTokens
	b.usage.ReservedOutputTokens += reservation.MaxOutputTokens
	b.usage.EstimatedCostUSD += reservation.EstimatedCostUSD
	return nil
}

func (b *RunBudget) rollbackAttemptLocked(reservation AttemptReservation) {
	b.usage.Attempts--
	b.usage.EstimatedInputTokens -= reservation.EstimatedInputTokens
	b.usage.ReservedOutputTokens -= reservation.MaxOutputTokens
	b.usage.EstimatedCostUSD -= reservation.EstimatedCostUSD
}

func (b *RunBudget) reconcileAttemptLocked(reservation AttemptReservation, actual AttemptActual) error {
	if actual.InputTokens < 0 || actual.OutputTokens < 0 || actual.CostUSD != nil && *actual.CostUSD < 0 {
		return errors.New("actual attempt usage must be non-negative")
	}
	var reconciliationErrors []error
	inputBefore := b.usage.EstimatedInputTokens - reservation.EstimatedInputTokens
	b.usage.EstimatedInputTokens = inputBefore + actual.InputTokens
	if b.limits.MaxEstimatedInputTokens > 0 && b.usage.EstimatedInputTokens > b.limits.MaxEstimatedInputTokens {
		reconciliationErrors = append(reconciliationErrors, &BudgetExceededError{
			Metric: "actual_input_tokens", Used: float64(inputBefore), Requested: float64(actual.InputTokens), Limit: float64(b.limits.MaxEstimatedInputTokens),
		})
	}
	outputBefore := b.usage.ReservedOutputTokens - reservation.MaxOutputTokens
	b.usage.ReservedOutputTokens = outputBefore + actual.OutputTokens
	if b.limits.MaxReservedOutputTokens > 0 && b.usage.ReservedOutputTokens > b.limits.MaxReservedOutputTokens {
		reconciliationErrors = append(reconciliationErrors, &BudgetExceededError{
			Metric: "actual_output_tokens", Used: float64(outputBefore), Requested: float64(actual.OutputTokens), Limit: float64(b.limits.MaxReservedOutputTokens),
		})
	}
	if actual.CostUSD != nil {
		costBefore := b.usage.EstimatedCostUSD - reservation.EstimatedCostUSD
		b.usage.EstimatedCostUSD = costBefore + *actual.CostUSD
		if b.limits.MaxCostUSD > 0 && b.usage.EstimatedCostUSD > b.limits.MaxCostUSD {
			reconciliationErrors = append(reconciliationErrors, &BudgetExceededError{
				Metric: "actual_cost_usd", Used: costBefore, Requested: *actual.CostUSD, Limit: b.limits.MaxCostUSD,
			})
		}
	}
	return errors.Join(reconciliationErrors...)
}

func (b *RunBudget) checkDeadlineLocked() error {
	if !b.deadline.IsZero() && !time.Now().Before(b.deadline) {
		return &BudgetExceededError{Metric: "duration_seconds", Used: time.Since(b.started).Seconds(), Limit: b.limits.MaxDuration.Seconds()}
	}
	return nil
}

func (b *RunBudget) Deadline() (time.Time, bool) {
	if b == nil || b.deadline.IsZero() {
		return time.Time{}, false
	}
	return b.deadline, true
}

func (b *RunBudget) Snapshot() BudgetSnapshot {
	if b == nil {
		return BudgetSnapshot{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	snapshot := b.usage
	snapshot.ElapsedSeconds = time.Since(b.started).Seconds()
	return snapshot
}

func persistedLimits(limits BudgetLimits) persistedBudgetLimits {
	return persistedBudgetLimits{
		MaxCalls:                limits.MaxCalls,
		MaxAttempts:             limits.MaxAttempts,
		MaxEstimatedInputTokens: limits.MaxEstimatedInputTokens,
		MaxReservedOutputTokens: limits.MaxReservedOutputTokens,
		MaxConcurrent:           limits.MaxConcurrent,
		MaxCostUSD:              limits.MaxCostUSD,
		MaxDurationSeconds:      int64(limits.MaxDuration / time.Second),
	}
}

func validateRestoredBudget(usage BudgetSnapshot, limits BudgetLimits) error {
	if usage.Calls < 0 || usage.Attempts < 0 || usage.EstimatedInputTokens < 0 || usage.ReservedOutputTokens < 0 || usage.EstimatedCostUSD < 0 {
		return errors.New("persistent run budget contains negative usage")
	}
	if limits.MaxCalls > 0 && usage.Calls > limits.MaxCalls {
		return fmt.Errorf("persistent run budget calls %d exceed limit %d", usage.Calls, limits.MaxCalls)
	}
	if limits.MaxAttempts > 0 && usage.Attempts > limits.MaxAttempts {
		return fmt.Errorf("persistent run budget attempts %d exceed limit %d", usage.Attempts, limits.MaxAttempts)
	}
	if limits.MaxEstimatedInputTokens > 0 && usage.EstimatedInputTokens > limits.MaxEstimatedInputTokens {
		return fmt.Errorf("persistent run budget input %d exceeds limit %d", usage.EstimatedInputTokens, limits.MaxEstimatedInputTokens)
	}
	if limits.MaxReservedOutputTokens > 0 && usage.ReservedOutputTokens > limits.MaxReservedOutputTokens {
		return fmt.Errorf("persistent run budget output %d exceed limit %d", usage.ReservedOutputTokens, limits.MaxReservedOutputTokens)
	}
	if limits.MaxConcurrent > 0 && usage.PeakConcurrent > limits.MaxConcurrent {
		return fmt.Errorf("persistent run budget peak concurrency %d exceeds limit %d", usage.PeakConcurrent, limits.MaxConcurrent)
	}
	if limits.MaxCostUSD > 0 && usage.EstimatedCostUSD > limits.MaxCostUSD {
		return fmt.Errorf("persistent run budget cost %.4f exceeds limit %.4f", usage.EstimatedCostUSD, limits.MaxCostUSD)
	}
	return nil
}

func (b *RunBudget) persistLocked() error {
	if b.statePath == "" {
		return nil
	}
	state := persistedBudgetState{
		SchemaVersion: runBudgetStateSchemaVersion,
		StartedAt:     b.started,
		Limits:        persistedLimits(b.limits),
		Usage:         b.usage,
	}
	state.Usage.ElapsedSeconds = time.Since(b.started).Seconds()
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode persistent run budget: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(b.statePath), safety.SensitiveDirectoryMode); err != nil {
		return fmt.Errorf("create persistent run budget directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(b.statePath), "."+filepath.Base(b.statePath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create persistent run budget candidate: %w", err)
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(safety.SensitiveFileMode); err != nil {
		return fmt.Errorf("secure persistent run budget candidate: %w", err)
	}
	if _, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write persistent run budget candidate: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync persistent run budget candidate: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close persistent run budget candidate: %w", err)
	}
	if err := os.Rename(tempPath, b.statePath); err != nil {
		return fmt.Errorf("commit persistent run budget: %w", err)
	}
	committed = true
	return nil
}
