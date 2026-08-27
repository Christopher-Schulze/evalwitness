package capsule

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	LegacyClaimFactsSchemaVersion = "evalwitness.legacy-claim-facts.v1"
	legacyClaimFactsValidatorID   = "evalwitness.validator.legacy-claim-facts.v1"
	legacyClaimFactsBindingID     = "evalwitness.validator.legacy-claim-facts-bindings.v1"
)

type LegacyClaimFacts struct {
	SchemaVersion                  string `json:"schema_version"`
	LegacyResultCount              int    `json:"legacy_result_count"`
	BenchmarkTasksTotal            int    `json:"benchmark_tasks_total"`
	BenchmarkTrialsTotal           int    `json:"benchmark_trials_total"`
	VerifierCallsTotal             int    `json:"verifier_calls_total"`
	VerifierScoresTotal            string `json:"verifier_scores_total"`
	RandomExpectedScoresTotal      string `json:"random_expected_scores_total"`
	PairDecisionsTotal             int    `json:"pair_decisions_total"`
	PairTiesTotal                  int    `json:"pair_ties_total"`
	UnextractedScoresTotal         int    `json:"unextracted_scores_total"`
	PaperParityCallsTotal          int    `json:"paper_parity_calls_total"`
	PaperParityScoresTotal         string `json:"paper_parity_scores_total"`
	BoundedScoresTotal             string `json:"bounded_scores_total"`
	ThreeCallPairsTotal            int    `json:"three_call_pairs_total"`
	TerminalVerifierScore          string `json:"terminal_verifier_score"`
	TerminalVerifierCalls          int    `json:"terminal_verifier_calls"`
	TerminalEvidence16KScore       string `json:"terminal_evidence_16k_score"`
	TerminalEvidence16KCalls       int    `json:"terminal_evidence_16k_calls"`
	TerminalAbsoluteScore          string `json:"terminal_absolute_score"`
	TerminalAbsoluteCalls          int    `json:"terminal_absolute_calls"`
	SWEVerifierScore               string `json:"swe_verifier_score"`
	SWESizeAgnosticScore           string `json:"swe_size_agnostic_score"`
	SWEHunkBaselineDecidableSolved int    `json:"swe_hunk_baseline_decidable_solved"`
	SWEVerifierDecidableSolved     int    `json:"swe_verifier_decidable_solved"`
	CalibrationDecisionsTotal      int    `json:"calibration_decisions_total"`
	CalibrationMonotone            bool   `json:"calibration_monotone"`
	VerifierRoutesEqual            bool   `json:"verifier_routes_equal"`
	Digest                         string `json:"digest"`
}

type legacyBenchmarkResult struct {
	Tasks         int         `json:"tasks"`
	Trials        int         `json:"trials"`
	VerifierScore json.Number `json:"verifier_score"`
	PassAt1Score  json.Number `json:"pass_at_1_score"`
	Usage         struct {
		Calls             int `json:"calls"`
		UnextractedScores int `json:"unextracted_scores"`
	} `json:"usage"`
	Escalation struct {
		ThreeCallPairs int `json:"three_call_pairs"`
	} `json:"escalation"`
	Calibration struct {
		Count    int  `json:"count"`
		Monotone bool `json:"monotone"`
	} `json:"calibration"`
	Route struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	} `json:"route"`
	Details []struct {
		Selection *struct {
			PairDecisions []struct {
				WinProbability json.Number `json:"win_probability"`
			} `json:"pair_decisions"`
		} `json:"selection,omitempty"`
	} `json:"details"`
	Baselines []struct {
		Name            string `json:"name"`
		DecidableSolved int    `json:"decidable_solved"`
		BothCorrect     int    `json:"both_correct"`
		SubjectOnly     int    `json:"subject_only"`
	} `json:"baselines"`
}

func referenceLegacyFactTypes() []ComponentType {
	return []ComponentType{{
		TypeID: LegacyClaimFactsSchemaVersion, SchemaID: LegacyClaimFactsSchemaVersion,
		Role: RoleDerivation, AllowedVisibilities: []Visibility{VisibilityPublic},
		MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON,
		ValidatorID: legacyClaimFactsValidatorID, BindingValidatorID: legacyClaimFactsBindingID,
		ParentRules: []ParentRule{
			parentRule(EdgeDerivedFrom, LegacyDerivedResultSchemaVersion, len(legacyResultDefinitions()), len(legacyResultDefinitions())),
			parentRule(EdgeDerivedFrom, LegacyEvidenceIndexSchemaVersion, 1, 1),
		},
	}}
}

func referenceLegacyFactPayloadValidators() map[string]PayloadValidator {
	return map[string]PayloadValidator{legacyClaimFactsValidatorID: func(payload []byte) error {
		var facts LegacyClaimFacts
		if err := decodeReferenceJSON(payload, &facts); err != nil {
			return err
		}
		return facts.Validate()
	}}
}

func referenceLegacyFactBindingValidators() map[string]BindingValidator {
	return map[string]BindingValidator{legacyClaimFactsBindingID: validateLegacyClaimFactBindings}
}

func addReferenceLegacyClaimFacts(registry *Registry, index ComponentRecord, records *[]ComponentRecord, payloads map[string][]byte) (ComponentRecord, error) {
	parents := []ParentRef{internalParentRef(EdgeDerivedFrom, index)}
	bound := make([]BoundParent, 0, len(legacyResultDefinitions()))
	for _, definition := range legacyResultDefinitions() {
		record, err := componentByName(*records, definition.Name)
		if err != nil {
			return ComponentRecord{}, err
		}
		parents = append(parents, internalParentRef(EdgeDerivedFrom, record))
		bound = append(bound, boundReferenceRecord(record, payloads))
	}
	facts, err := deriveLegacyClaimFacts(bound)
	if err != nil {
		return ComponentRecord{}, err
	}
	raw, err := protocol.CanonicalMarshal(facts)
	if err != nil {
		return ComponentRecord{}, err
	}
	return addReferencePayload(registry, ComponentInput{
		Name: "legacy.claim-facts", TypeID: LegacyClaimFactsSchemaVersion,
		Visibility: VisibilityPublic, Payload: raw, Parents: parents,
	}, records, payloads)
}

func (facts LegacyClaimFacts) Validate() error {
	if facts.SchemaVersion != LegacyClaimFactsSchemaVersion || facts.LegacyResultCount != len(legacyResultDefinitions()) ||
		facts.BenchmarkTasksTotal < 1 || facts.BenchmarkTrialsTotal < 1 || facts.VerifierCallsTotal < 1 ||
		!validExactRational(facts.VerifierScoresTotal) || !validExactRational(facts.RandomExpectedScoresTotal) ||
		facts.PairDecisionsTotal < 1 || facts.PairTiesTotal < 0 || facts.UnextractedScoresTotal < 0 ||
		facts.PaperParityCallsTotal < 1 || !validExactRational(facts.PaperParityScoresTotal) ||
		!validExactRational(facts.BoundedScoresTotal) || facts.ThreeCallPairsTotal < 0 ||
		!validExactRational(facts.TerminalVerifierScore) || facts.TerminalVerifierCalls < 1 ||
		!validExactRational(facts.TerminalEvidence16KScore) || facts.TerminalEvidence16KCalls < 1 ||
		!validExactRational(facts.TerminalAbsoluteScore) || facts.TerminalAbsoluteCalls < 1 ||
		!validExactRational(facts.SWEVerifierScore) || !validExactRational(facts.SWESizeAgnosticScore) ||
		facts.SWEHunkBaselineDecidableSolved < 0 || facts.SWEVerifierDecidableSolved < 0 ||
		facts.CalibrationDecisionsTotal < 1 || !validDigest(facts.Digest) {
		return errors.New("legacy claim facts identity is invalid")
	}
	digest, err := legacyClaimFactsDigest(facts)
	if err != nil || digest != facts.Digest {
		return errors.New("legacy claim facts digest is invalid")
	}
	return nil
}

func validateLegacyClaimFactBindings(context BindingContext) error {
	var actual LegacyClaimFacts
	if err := decodeReferenceJSON(context.Payload, &actual); err != nil {
		return err
	}
	results := make([]BoundParent, 0, len(legacyResultDefinitions()))
	indexCount := 0
	for _, parent := range context.Parents {
		switch parent.Record.TypeID {
		case LegacyDerivedResultSchemaVersion:
			results = append(results, parent)
		case LegacyEvidenceIndexSchemaVersion:
			indexCount++
		}
	}
	if indexCount != 1 || len(results) != len(legacyResultDefinitions()) {
		return errors.New("legacy claim facts require the exact legacy index and result set")
	}
	expected, err := deriveLegacyClaimFacts(results)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return errors.New("legacy claim facts differ from deterministic recomputation")
	}
	return nil
}

func deriveLegacyClaimFacts(parents []BoundParent) (LegacyClaimFacts, error) {
	results := make(map[string]legacyBenchmarkResult, len(parents))
	for _, parent := range parents {
		result, err := decodeLegacyBenchmarkResult(parent.Payload)
		if err != nil {
			return LegacyClaimFacts{}, fmt.Errorf("derive facts from %q: %w", parent.Record.Name, err)
		}
		results[parent.Record.Name] = result
	}
	wanted := func(name string) (legacyBenchmarkResult, error) {
		result, found := results["legacy.result."+name]
		if !found {
			return legacyBenchmarkResult{}, fmt.Errorf("legacy result %q is absent", name)
		}
		return result, nil
	}
	terminal, err := wanted("terminal-bench-verifier")
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	swe, err := wanted("swebench-verifier")
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	terminalParity, err := wanted("terminal-bench-paper-parity")
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	sweParity, err := wanted("swebench-paper-parity")
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	terminal16K, err := wanted("terminal-bench-evidence-16k")
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	absolute, err := wanted("terminal-bench-absolute")
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	sizeAgnostic, err := wanted("swebench-size-agnostic")
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	verifierScores, err := sumLegacyNumbers(terminal.VerifierScore, swe.VerifierScore)
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	randomScores, err := sumLegacyNumbers(terminal.PassAt1Score, swe.PassAt1Score)
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	parityScores, err := sumLegacyNumbers(terminalParity.VerifierScore, sweParity.VerifierScore)
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	decisions, ties, err := pairDecisionCounts(terminal, swe)
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	hunkBaseline, verifierSolved, err := hunkBaselineFacts(swe)
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	facts := LegacyClaimFacts{
		SchemaVersion: LegacyClaimFactsSchemaVersion, LegacyResultCount: len(results),
		BenchmarkTasksTotal: terminal.Tasks + swe.Tasks, BenchmarkTrialsTotal: terminal.Trials + swe.Trials,
		VerifierCallsTotal:  terminal.Usage.Calls + swe.Usage.Calls,
		VerifierScoresTotal: verifierScores, RandomExpectedScoresTotal: randomScores,
		PairDecisionsTotal: decisions, PairTiesTotal: ties,
		UnextractedScoresTotal: terminal.Usage.UnextractedScores + swe.Usage.UnextractedScores,
		PaperParityCallsTotal:  terminalParity.Usage.Calls + sweParity.Usage.Calls,
		PaperParityScoresTotal: parityScores, BoundedScoresTotal: verifierScores,
		ThreeCallPairsTotal:   terminal.Escalation.ThreeCallPairs + swe.Escalation.ThreeCallPairs,
		TerminalVerifierScore: exactLegacyNumber(terminal.VerifierScore), TerminalVerifierCalls: terminal.Usage.Calls,
		TerminalEvidence16KScore: exactLegacyNumber(terminal16K.VerifierScore), TerminalEvidence16KCalls: terminal16K.Usage.Calls,
		TerminalAbsoluteScore: exactLegacyNumber(absolute.VerifierScore), TerminalAbsoluteCalls: absolute.Usage.Calls,
		SWEVerifierScore: exactLegacyNumber(swe.VerifierScore), SWESizeAgnosticScore: exactLegacyNumber(sizeAgnostic.VerifierScore),
		SWEHunkBaselineDecidableSolved: hunkBaseline, SWEVerifierDecidableSolved: verifierSolved,
		CalibrationDecisionsTotal: terminal.Calibration.Count + swe.Calibration.Count,
		CalibrationMonotone:       terminal.Calibration.Monotone && swe.Calibration.Monotone,
		VerifierRoutesEqual:       terminal.Route.Provider == swe.Route.Provider && terminal.Route.Model == swe.Route.Model,
	}
	facts.Digest, err = legacyClaimFactsDigest(facts)
	if err != nil {
		return LegacyClaimFacts{}, err
	}
	return facts, facts.Validate()
}

func decodeLegacyBenchmarkResult(payload []byte) (legacyBenchmarkResult, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var result legacyBenchmarkResult
	if err := decoder.Decode(&result); err != nil {
		return legacyBenchmarkResult{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return legacyBenchmarkResult{}, errors.New("legacy benchmark result contains trailing JSON")
	}
	if result.Tasks < 1 || result.Trials < 1 || result.VerifierScore == "" || result.PassAt1Score == "" || result.Usage.Calls < 0 {
		return legacyBenchmarkResult{}, errors.New("legacy benchmark result lacks required fact fields")
	}
	return result, nil
}

func pairDecisionCounts(results ...legacyBenchmarkResult) (int, int, error) {
	total := 0
	ties := 0
	half := big.NewRat(1, 2)
	for _, result := range results {
		for _, detail := range result.Details {
			if detail.Selection == nil {
				continue
			}
			for _, decision := range detail.Selection.PairDecisions {
				probability, ok := new(big.Rat).SetString(string(decision.WinProbability))
				if !ok {
					return 0, 0, errors.New("legacy pair decision probability is invalid")
				}
				total++
				if probability.Cmp(half) == 0 {
					ties++
				}
			}
		}
	}
	return total, ties, nil
}

func hunkBaselineFacts(result legacyBenchmarkResult) (int, int, error) {
	for _, baseline := range result.Baselines {
		if baseline.Name == "most patch hunks" {
			return baseline.DecidableSolved, baseline.BothCorrect + baseline.SubjectOnly, nil
		}
	}
	return 0, 0, errors.New("SWE-bench hunk baseline is absent")
}

func sumLegacyNumbers(values ...json.Number) (string, error) {
	total := new(big.Rat)
	for _, value := range values {
		rational, ok := new(big.Rat).SetString(string(value))
		if !ok {
			return "", fmt.Errorf("legacy number %q is invalid", value)
		}
		total.Add(total, rational)
	}
	return total.RatString(), nil
}

func exactLegacyNumber(value json.Number) string {
	rational, ok := new(big.Rat).SetString(string(value))
	if !ok {
		return ""
	}
	return rational.RatString()
}

func validExactRational(value string) bool {
	rational, ok := new(big.Rat).SetString(value)
	return ok && rational.RatString() == value
}

func legacyClaimFactsDigest(facts LegacyClaimFacts) (string, error) {
	facts.Digest = ""
	return protocol.Digest(facts)
}

func componentByName(records []ComponentRecord, name string) (ComponentRecord, error) {
	index := slices.IndexFunc(records, func(record ComponentRecord) bool { return record.Name == name })
	if index < 0 {
		return ComponentRecord{}, fmt.Errorf("reference package has no component named %q", name)
	}
	return records[index], nil
}
