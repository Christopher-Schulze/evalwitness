package stress

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type reductionInputFixture struct {
	units []ReductionUnit
}

func (fixture reductionInputFixture) CanonicalBytes() ([]byte, error) {
	return json.Marshal(fixture.units)
}

func (fixture reductionInputFixture) Units() []ReductionUnit {
	return append([]ReductionUnit(nil), fixture.units...)
}

func (fixture reductionInputFixture) Remove(removed ReductionUnit) (ReducibleInput, error) {
	result := make([]ReductionUnit, 0, len(fixture.units)-1)
	found := false
	for _, unit := range fixture.units {
		if unit == removed {
			found = true
			continue
		}
		result = append(result, unit)
	}
	if !found {
		return nil, errors.New("fixture unit not found")
	}
	return reductionInputFixture{units: result}, nil
}

type reductionOracleFixture struct {
	required            ReductionUnit
	relationDigest      string
	privacyPolicyDigest string
	replayResultDigest  string
}

type mutableReductionInputFixture struct {
	units []ReductionUnit
}

func (fixture *mutableReductionInputFixture) CanonicalBytes() ([]byte, error) {
	return json.Marshal(fixture.units)
}

func (fixture *mutableReductionInputFixture) Units() []ReductionUnit {
	return append([]ReductionUnit(nil), fixture.units...)
}

func (fixture *mutableReductionInputFixture) Remove(removed ReductionUnit) (ReducibleInput, error) {
	value, err := reductionInputFixture{units: fixture.units}.Remove(removed)
	if err != nil {
		return nil, err
	}
	return &mutableReductionInputFixture{units: value.(reductionInputFixture).units}, nil
}

type mutatingReductionOracleFixture struct {
	reductionOracleFixture
}

func (fixture mutatingReductionOracleFixture) Evaluate(ctx context.Context, input ReducibleInput) (ReductionObservation, error) {
	observation, err := fixture.reductionOracleFixture.Evaluate(ctx, input)
	mutable := input.(*mutableReductionInputFixture)
	mutable.units = append(mutable.units, ReductionUnit{Kind: "event", ID: "oracle-injected"})
	return observation, err
}

func (fixture reductionOracleFixture) Evaluate(_ context.Context, input ReducibleInput) (ReductionObservation, error) {
	preservesViolation := false
	for _, unit := range input.Units() {
		if unit == fixture.required {
			preservesViolation = true
			break
		}
	}
	replayResultDigest := fixture.replayResultDigest
	if replayResultDigest == "" {
		replayResultDigest = digestText("replay-result")
	}
	return SealReductionObservation(ReductionObservation{
		RelationDigest: fixture.relationDigest, PrivacyPolicyDigest: fixture.privacyPolicyDigest,
		RelationRevalidated: true, PrivacyRevalidated: true, ViolationPreserved: preservesViolation,
		RelationProofDigest: digestText("relation-proof"), PrivacyProofDigest: digestText("privacy-proof"),
		ReplayResultDigest: replayResultDigest,
	})
}

func TestReducerRestartsAfterAcceptanceAndProvesOneMinimality(t *testing.T) {
	units := []ReductionUnit{{Kind: "event", ID: "a"}, {Kind: "event", ID: "b"}, {Kind: "event", ID: "c"}}
	relationDigest, privacyDigest := digestText("relation"), digestText("privacy")
	value, err := ReduceCounterexample(context.Background(), ReductionRequest{
		RelationDigest: relationDigest, SourceResultDigest: digestText("result"), CaseID: "mutation-example",
		PrivacyPolicyDigest: privacyDigest, PublicReleaseAllowed: true,
		Input: reductionInputFixture{units: units}, Oracle: reductionOracleFixture{required: units[2], relationDigest: relationDigest, privacyPolicyDigest: privacyDigest}, MaximumEvaluations: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.Minimality != ReductionOneMinimal || value.AcceptedReductions != 2 || len(value.FinalUnits) != 1 || value.FinalUnits[0] != units[2] {
		t.Fatalf("unexpected reduced witness: %+v", value)
	}
	if len(value.Steps) != 3 || value.Steps[0].Decision != ReductionAccepted || value.Steps[1].Decision != ReductionAccepted || value.Steps[2].Decision != ReductionRejected {
		t.Fatalf("unexpected deterministic reduction trace: %+v", value.Steps)
	}
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestReducerRejectsBudgetThatCannotCompleteItsMinimalityPass(t *testing.T) {
	units := []ReductionUnit{{Kind: "event", ID: "a"}, {Kind: "event", ID: "b"}, {Kind: "event", ID: "c"}}
	relationDigest, privacyDigest := digestText("relation"), digestText("privacy")
	_, err := ReduceCounterexample(context.Background(), ReductionRequest{
		RelationDigest: relationDigest, SourceResultDigest: digestText("result"), CaseID: "mutation-example",
		PrivacyPolicyDigest: privacyDigest, Input: reductionInputFixture{units: units},
		Oracle: reductionOracleFixture{required: units[2], relationDigest: relationDigest, privacyPolicyDigest: privacyDigest}, MaximumEvaluations: 5,
	})
	if err == nil {
		t.Fatal("reducer accepted an evaluation budget that could not prove one-minimality")
	}
}

func TestReducerRejectsOracleInputMutation(t *testing.T) {
	units := []ReductionUnit{{Kind: "event", ID: "a"}, {Kind: "event", ID: "b"}}
	relationDigest, privacyDigest := digestText("relation"), digestText("privacy")
	_, err := ReduceCounterexample(context.Background(), ReductionRequest{
		RelationDigest: relationDigest, SourceResultDigest: digestText("result"), CaseID: "mutation-example",
		PrivacyPolicyDigest: privacyDigest, Input: &mutableReductionInputFixture{units: append([]ReductionUnit(nil), units...)},
		Oracle: mutatingReductionOracleFixture{reductionOracleFixture{required: units[1], relationDigest: relationDigest, privacyPolicyDigest: privacyDigest}}, MaximumEvaluations: 3,
	})
	if err == nil {
		t.Fatal("reducer accepted an oracle that mutated its input")
	}
}

func TestReducerRejectsTypedNilInput(t *testing.T) {
	var input *mutableReductionInputFixture
	relationDigest, privacyDigest := digestText("relation"), digestText("privacy")
	_, err := ReduceCounterexample(context.Background(), ReductionRequest{
		RelationDigest: relationDigest, SourceResultDigest: digestText("result"), CaseID: "mutation-example",
		PrivacyPolicyDigest: privacyDigest, Input: input,
		Oracle: reductionOracleFixture{required: ReductionUnit{Kind: "event", ID: "a"}, relationDigest: relationDigest, privacyPolicyDigest: privacyDigest}, MaximumEvaluations: 1,
	})
	if err == nil {
		t.Fatal("reducer accepted a typed nil input")
	}
}
