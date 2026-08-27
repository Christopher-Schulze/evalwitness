package mutation

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

type TrustedValidator struct {
	Spec            ValidatorSpec
	Executable      string
	Arguments       []string
	Environment     []string
	PassingExitCode int
}

type TaskEnvironment struct {
	ID              string
	Revision        string
	Root            string
	Disposable      bool
	NetworkDisabled bool
}

type TaskEnvironmentPair struct {
	Original TaskEnvironment
	Mutated  TaskEnvironment
}

type HermeticFailure string

const (
	HermeticFailureTimeout     HermeticFailure = "timeout"
	HermeticFailureOutputLimit HermeticFailure = "output_limit"
	HermeticFailureExecution   HermeticFailure = "execution_error"
	HermeticFailureCleanup     HermeticFailure = "cleanup_error"
)

type HermeticExecution struct {
	ExitCode            int             `json:"exit_code"`
	OutputDigest        string          `json:"output_digest"`
	OutputBytes         int             `json:"output_bytes"`
	TimedOut            bool            `json:"timed_out"`
	Passed              bool            `json:"passed"`
	Failure             HermeticFailure `json:"failure,omitempty"`
	FailureDetailDigest string          `json:"failure_detail_digest,omitempty"`
}

type HermeticResult struct {
	ValidatorID string            `json:"validator_id"`
	Original    HermeticExecution `json:"original"`
	Mutated     HermeticExecution `json:"mutated"`
	Passed      bool              `json:"passed"`
}

type HermeticRegistry struct {
	validators map[string]TrustedValidator
}

func TrustedValidatorContractDigest(validator TrustedValidator) (string, error) {
	material := struct {
		ID              string
		Version         string
		Kind            ValidationKind
		Executable      string
		Arguments       []string
		Environment     []string
		PassingExitCode int
	}{
		ID: validator.Spec.ID, Version: validator.Spec.Version, Kind: validator.Spec.Kind,
		Executable: validator.Executable, Arguments: validator.Arguments,
		Environment: validator.Environment, PassingExitCode: validator.PassingExitCode,
	}
	return digestJSON(material)
}

func NewHermeticRegistry(validators []TrustedValidator) (*HermeticRegistry, error) {
	registry := &HermeticRegistry{validators: make(map[string]TrustedValidator, len(validators))}
	for _, validator := range validators {
		if validator.Spec.Kind != ValidationHermetic || missing(validator.Spec.ID, validator.Spec.Version, validator.Executable) || !filepath.IsAbs(validator.Executable) {
			return nil, errors.New("trusted hermetic validator requires an absolute executable and hermetic specification")
		}
		if err := validateValidator(validator.Spec, Definition{}); err != nil {
			return nil, err
		}
		if !sort.StringsAreSorted(validator.Environment) {
			return nil, errors.New("trusted validator environment must be sorted")
		}
		contractDigest, err := TrustedValidatorContractDigest(validator)
		if err != nil {
			return nil, err
		}
		if contractDigest != validator.Spec.ContractDigest {
			return nil, errors.New("trusted validator contract digest does not bind its executable configuration")
		}
		key := validator.Spec.ID + "\x00" + validator.Spec.Version
		if _, duplicate := registry.validators[key]; duplicate {
			return nil, fmt.Errorf("duplicate trusted validator %q", validator.Spec.ID)
		}
		registry.validators[key] = validator
	}
	return registry, nil
}

func ValidateFormalRelation(manifest Manifest, original, mutated preprocess.Trajectory) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	definition, _ := DefinitionFor(manifest.Program.Family)
	if definition.PairLevel {
		return errors.New("pair-level relation requires pair-order validation")
	}
	if err := original.Validate(); err != nil {
		return err
	}
	if err := mutated.Validate(); err != nil {
		return err
	}
	if original.Digest != manifest.OriginalTrajectoryDigest || mutated.Digest != manifest.MutatedTrajectoryDigest || mutated.Derivation == nil || mutated.Derivation.ParentDigest != original.Digest {
		return errors.New("formal relation inputs do not match manifest lineage")
	}
	var preservation PreservationRecord
	var checks []Check
	var err error
	if manifest.Program.Version == MutationProgramVersionV2 || manifest.Program.Version == MutationProgramVersionV3 {
		preservation, checks, err = buildPreservationV2(original, mutated, definition)
		if manifest.Program.Version == MutationProgramVersionV3 {
			preservation.BoundaryVersion = EvidenceBoundaryVersionV3
		}
	} else {
		preservation, checks, err = buildPreservation(original, mutated, definition)
	}
	if err != nil {
		return err
	}
	if manifest.ExpectedRelation == RelationAmbiguous {
		preservation.AmbiguityReasons = append([]string(nil), manifest.Preservation.AmbiguityReasons...)
	}
	preservationDigest, err := digestJSON(preservation)
	if err != nil {
		return err
	}
	manifestPreservationDigest, err := digestJSON(manifest.Preservation)
	if err != nil {
		return err
	}
	if preservationDigest != manifestPreservationDigest {
		return errors.New("formal relation preservation record does not reproduce")
	}
	for _, check := range checks {
		if !check.Passed && manifest.Witness.LabelState == LabelProven {
			return fmt.Errorf("formal relation check %q failed for a proven label", check.Name)
		}
	}
	return nil
}

func (registry *HermeticRegistry) Observe(ctx context.Context, validatorSpec ValidatorSpec, environments TaskEnvironmentPair) (OutcomeProof, HermeticResult, error) {
	if registry == nil {
		return OutcomeProof{}, HermeticResult{}, errors.New("hermetic validator registry is nil")
	}
	if validatorSpec.Kind != ValidationHermetic {
		return OutcomeProof{}, HermeticResult{}, errors.New("outcome observation does not declare a hermetic validator")
	}
	if err := validateTaskEnvironment(environments.Original); err != nil {
		return OutcomeProof{}, HermeticResult{}, fmt.Errorf("original task environment: %w", err)
	}
	if err := validateTaskEnvironment(environments.Mutated); err != nil {
		return OutcomeProof{}, HermeticResult{}, fmt.Errorf("mutated task environment: %w", err)
	}
	if environments.Original.Root == environments.Mutated.Root || environments.Original.ID != environments.Mutated.ID || environments.Original.Revision != environments.Mutated.Revision {
		return OutcomeProof{}, HermeticResult{}, errors.New("outcome observation requires distinct roots for one pinned task and revision")
	}
	key := validatorSpec.ID + "\x00" + validatorSpec.Version
	validator, exists := registry.validators[key]
	if !exists || validator.Spec != validatorSpec {
		return OutcomeProof{}, HermeticResult{}, errors.New("mutation validator is not pinned in the trusted registry")
	}
	original, err := registry.Execute(ctx, validatorSpec, environments.Original)
	if err != nil {
		return OutcomeProof{}, HermeticResult{}, fmt.Errorf("original task outcome: %w", err)
	}
	mutated, err := registry.Execute(ctx, validatorSpec, environments.Mutated)
	if err != nil {
		return OutcomeProof{}, HermeticResult{}, fmt.Errorf("mutated task outcome: %w", err)
	}
	result := HermeticResult{ValidatorID: validator.Spec.ID, Original: original, Mutated: mutated}
	result.Passed = original.Passed && !mutated.Passed
	resultDigest, err := digestJSON(result)
	if err != nil {
		return OutcomeProof{}, HermeticResult{}, err
	}
	proof := OutcomeProof{
		Mechanism: ValidationHermetic, ContractDigest: validator.Spec.ContractDigest,
		OriginalPassed: original.Passed, MutatedPassed: mutated.Passed,
		IndependentOfTrace: true, WitnessDigest: resultDigest,
	}
	if !result.Passed {
		return proof, result, errors.New("trusted validator did not observe a pass-to-fail outcome change")
	}
	return proof, result, nil
}

func (registry *HermeticRegistry) Execute(ctx context.Context, validatorSpec ValidatorSpec, environment TaskEnvironment) (HermeticExecution, error) {
	if registry == nil {
		return HermeticExecution{}, errors.New("hermetic validator registry is nil")
	}
	if validatorSpec.Kind != ValidationHermetic {
		return HermeticExecution{}, errors.New("outcome execution does not declare a hermetic validator")
	}
	if err := validateTaskEnvironment(environment); err != nil {
		return HermeticExecution{}, err
	}
	key := validatorSpec.ID + "\x00" + validatorSpec.Version
	validator, exists := registry.validators[key]
	if !exists || validator.Spec != validatorSpec {
		return HermeticExecution{}, errors.New("outcome validator is not pinned in the trusted registry")
	}
	return runTrustedValidator(ctx, validator, environment)
}

func (registry *HermeticRegistry) Run(ctx context.Context, manifest Manifest, original, mutated preprocess.Trajectory, environments TaskEnvironmentPair) (HermeticResult, error) {
	if err := ValidateFormalRelation(manifest, original, mutated); err != nil {
		return HermeticResult{}, err
	}
	if manifest.OutcomeProof == nil {
		return HermeticResult{}, errors.New("hermetic mutation has no outcome proof")
	}
	proof, result, err := registry.Observe(ctx, manifest.Validator, environments)
	if err != nil {
		return result, err
	}
	if proof != *manifest.OutcomeProof {
		return result, errors.New("observed hermetic outcome does not reproduce the mutation proof")
	}
	return result, nil
}

func runTrustedValidator(ctx context.Context, validator TrustedValidator, environment TaskEnvironment) (HermeticExecution, error) {
	boundedContext, cancel := context.WithTimeout(ctx, time.Duration(validator.Spec.TimeoutMillis)*time.Millisecond)
	defer cancel()
	output := &boundedOutput{maximum: validator.Spec.MaximumOutputBytes}
	command := exec.CommandContext(boundedContext, validator.Executable, validator.Arguments...)
	command.Dir = environment.Root
	command.Env = append([]string(nil), validator.Environment...)
	command.Stdout = output
	command.Stderr = output
	configureValidatorProcess(command)
	runErr := command.Run()
	result := HermeticExecution{ExitCode: 0, OutputDigest: digestText(string(output.bytes)), OutputBytes: len(output.bytes)}
	if runErr != nil {
		var exitError *exec.ExitError
		if errors.As(runErr, &exitError) {
			result.ExitCode = exitError.ExitCode()
		} else if boundedContext.Err() == nil && !errors.Is(runErr, errOutputLimit) {
			result.ExitCode = -1
			result.Failure = HermeticFailureExecution
			result.FailureDetailDigest = digestText(runErr.Error())
		}
	}
	if cleanupErr := terminateValidatorProcess(command); cleanupErr != nil {
		result.Failure = HermeticFailureCleanup
		result.FailureDetailDigest = digestText(cleanupErr.Error())
		return result, fmt.Errorf("terminate trusted validator process tree: %w", cleanupErr)
	}
	result.TimedOut = errors.Is(boundedContext.Err(), context.DeadlineExceeded)
	if result.TimedOut {
		result.Failure = HermeticFailureTimeout
		result.FailureDetailDigest = digestText(context.DeadlineExceeded.Error())
		return result, errors.New("trusted validator exceeded its timeout")
	}
	if output.exceeded {
		result.Failure = HermeticFailureOutputLimit
		result.FailureDetailDigest = digestText(errOutputLimit.Error())
		return result, errors.New("trusted validator exceeded its output limit")
	}
	if boundedContext.Err() != nil {
		result.ExitCode = -1
		result.Failure = HermeticFailureExecution
		result.FailureDetailDigest = digestText(boundedContext.Err().Error())
		return result, fmt.Errorf("trusted validator context ended: %w", boundedContext.Err())
	}
	if result.Failure != "" {
		return result, fmt.Errorf("run trusted validator: %w", runErr)
	}
	result.Passed = result.ExitCode == validator.PassingExitCode
	return result, nil
}

func validateTaskEnvironment(environment TaskEnvironment) error {
	if missing(environment.ID, environment.Revision, environment.Root) || !environment.Disposable || !environment.NetworkDisabled {
		return errors.New("hermetic validation requires a pinned disposable network-disabled task environment")
	}
	if !filepath.IsAbs(environment.Root) {
		return errors.New("hermetic task environment root must be absolute")
	}
	info, err := os.Lstat(environment.Root)
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("hermetic task environment root must be a real directory")
	}
	return nil
}

var errOutputLimit = errors.New("validator output limit exceeded")

type boundedOutput struct {
	bytes    []byte
	maximum  int
	exceeded bool
}

func (output *boundedOutput) Write(data []byte) (int, error) {
	remaining := output.maximum - len(output.bytes)
	if remaining <= 0 {
		output.exceeded = true
		return 0, errOutputLimit
	}
	if len(data) > remaining {
		output.bytes = append(output.bytes, data[:remaining]...)
		output.exceeded = true
		return remaining, errOutputLimit
	}
	output.bytes = append(output.bytes, data...)
	return len(data), nil
}

func SortedEnvironment(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for name, value := range values {
		name = strings.TrimSpace(name)
		if name == "" || strings.ContainsAny(name, "=\x00") || strings.ContainsRune(value, '\x00') {
			continue
		}
		result = append(result, name+"="+value)
	}
	sort.Strings(result)
	return result
}
