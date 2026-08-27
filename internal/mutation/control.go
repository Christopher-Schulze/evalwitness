package mutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const FormalControlSchemaVersion = "evalwitness.formal-arithmetic-control.v1"

type FormalControlProgram struct {
	SchemaVersion string `json:"schema_version"`
	Operator      string `json:"operator"`
	Left          int64  `json:"left"`
	Right         int64  `json:"right"`
	Expected      int64  `json:"expected"`
}

type FormalControlResult struct {
	Value  int64  `json:"value"`
	Passed bool   `json:"passed"`
	Digest string `json:"digest"`
}

func EvaluateFormalControl(program FormalControlProgram) (FormalControlResult, error) {
	if program.SchemaVersion != FormalControlSchemaVersion {
		return FormalControlResult{}, errors.New("formal control schema is unsupported")
	}
	var value int64
	switch program.Operator {
	case "add":
		if program.Right > 0 && program.Left > int64(^uint64(0)>>1)-program.Right || program.Right < 0 && program.Left < -int64(^uint64(0)>>1)-1-program.Right {
			return FormalControlResult{}, errors.New("formal control addition overflows int64")
		}
		value = program.Left + program.Right
	case "subtract":
		if program.Right < 0 && program.Left > int64(^uint64(0)>>1)+program.Right || program.Right > 0 && program.Left < -int64(^uint64(0)>>1)-1+program.Right {
			return FormalControlResult{}, errors.New("formal control subtraction overflows int64")
		}
		value = program.Left - program.Right
	default:
		return FormalControlResult{}, fmt.Errorf("formal control operator %q is unsupported", program.Operator)
	}
	result := FormalControlResult{Value: value, Passed: value == program.Expected}
	digest, err := digestJSON(result)
	if err != nil {
		return FormalControlResult{}, err
	}
	result.Digest = digest
	return result, nil
}

func DecodeFormalControl(reader io.Reader) (FormalControlProgram, error) {
	var program FormalControlProgram
	if err := decodeStrict(reader, &program); err != nil {
		return FormalControlProgram{}, err
	}
	return program, nil
}

func ValidateFormalControlPair(original, mutated FormalControlProgram) (OutcomeProof, error) {
	originalResult, err := EvaluateFormalControl(original)
	if err != nil {
		return OutcomeProof{}, err
	}
	mutatedResult, err := EvaluateFormalControl(mutated)
	if err != nil {
		return OutcomeProof{}, err
	}
	contractDigest, err := formalControlContractDigest()
	if err != nil {
		return OutcomeProof{}, err
	}
	witnessDigest, err := digestJSON([]FormalControlResult{originalResult, mutatedResult})
	if err != nil {
		return OutcomeProof{}, err
	}
	proof := OutcomeProof{
		Mechanism: ValidationFormal, ContractDigest: contractDigest,
		OriginalPassed: originalResult.Passed, MutatedPassed: mutatedResult.Passed,
		IndependentOfTrace: true, WitnessDigest: witnessDigest,
	}
	if !proof.OriginalPassed || proof.MutatedPassed {
		return proof, errors.New("formal positive control does not produce a pass-to-fail outcome")
	}
	return proof, nil
}

func FormalPositiveControl() (CorpusControl, error) {
	original := FormalControlProgram{SchemaVersion: FormalControlSchemaVersion, Operator: "add", Left: 2, Right: 2, Expected: 4}
	mutated := FormalControlProgram{SchemaVersion: FormalControlSchemaVersion, Operator: "subtract", Left: 2, Right: 2, Expected: 4}
	originalDigest, err := digestJSON(original)
	if err != nil {
		return CorpusControl{}, err
	}
	mutatedDigest, err := digestJSON(mutated)
	if err != nil {
		return CorpusControl{}, err
	}
	proof, err := ValidateFormalControlPair(original, mutated)
	if err != nil {
		return CorpusControl{}, err
	}
	contractDigest, err := formalControlContractDigest()
	if err != nil {
		return CorpusControl{}, err
	}
	control := CorpusControl{
		Kind: "positive",
		Validator: ValidatorSpec{
			ID: "evalwitness.formal-arithmetic-control", Version: FormalControlSchemaVersion, Kind: ValidationFormal,
			ContractDigest: contractDigest, TimeoutMillis: 1_000, MaximumOutputBytes: 64 * 1024,
		},
		OriginalArtifact: "eval/fixtures/controlled-corruption/original.json",
		MutatedArtifact:  "eval/fixtures/controlled-corruption/mutated.json",
		OriginalDigest:   originalDigest, MutatedDigest: mutatedDigest, OutcomeProof: proof,
		RegenerationCommand: []string{"./evalwitness", "mutation", "control", "validate", "--original", "@eval/fixtures/controlled-corruption/original.json", "--mutated", "@eval/fixtures/controlled-corruption/mutated.json"},
	}
	digest, err := corpusControlDigest(control)
	if err != nil {
		return CorpusControl{}, err
	}
	control.Digest = digest
	control.ID = "control-" + digest
	return control, nil
}

func formalControlContractDigest() (string, error) {
	contract := struct {
		SchemaVersion string   `json:"schema_version"`
		Operators     []string `json:"operators"`
		Arithmetic    string   `json:"arithmetic"`
	}{SchemaVersion: FormalControlSchemaVersion, Operators: []string{"add", "subtract"}, Arithmetic: "checked_int64_exact_equality"}
	encoded, err := json.Marshal(contract)
	if err != nil {
		return "", err
	}
	return digestText(string(encoded)), nil
}
