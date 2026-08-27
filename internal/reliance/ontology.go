package reliance

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	PathCommandExitCode         preprocess.FieldPath = "/command/exit_code"
	PathErrorClass              preprocess.FieldPath = "/error/class"
	PathErrorSafeMessage        preprocess.FieldPath = "/error/safe_message"
	PathEvaluationErrorType     preprocess.FieldPath = "/evaluation/error_type"
	PathEvaluationExplanation   preprocess.FieldPath = "/evaluation/explanation"
	PathEvaluationScoreLabel    preprocess.FieldPath = "/evaluation/score_label"
	PathEvaluationScoreValue    preprocess.FieldPath = "/evaluation/score_value"
	PathFileChangeContentDigest preprocess.FieldPath = "/file_change/content_digest"
	PathFileChangeDiff          preprocess.FieldPath = "/file_change/diff"
	PathFileChangeDiffDigest    preprocess.FieldPath = "/file_change/diff_digest"
	PathFileChangeOperation     preprocess.FieldPath = "/file_change/operation"
	PathFileChangePathAlias     preprocess.FieldPath = "/file_change/path_alias"
	PathFileChangeSizeBytes     preprocess.FieldPath = "/file_change/size_bytes"
	PathMessageParts            preprocess.FieldPath = "/message/parts"
	PathMessagePhase            preprocess.FieldPath = "/message/phase"
	PathMetadataName            preprocess.FieldPath = "/metadata/name"
	PathMetadataPresent         preprocess.FieldPath = "/metadata/present"
	PathMetadataValue           preprocess.FieldPath = "/metadata/value"
	PathMetadataValueDigest     preprocess.FieldPath = "/metadata/value_digest"
	PathOutputStatus            preprocess.FieldPath = "/output/status"
	PathOutputStream            preprocess.FieldPath = "/output/stream"
	PathOutputText              preprocess.FieldPath = "/output/text"
	PathToolCallArguments       preprocess.FieldPath = "/tool_call/arguments"
	PathToolResultError         preprocess.FieldPath = "/tool_result/error"
	PathToolResultExitCode      preprocess.FieldPath = "/tool_result/exit_code"
	PathToolResultOutput        preprocess.FieldPath = "/tool_result/output"
	PathToolResultStderr        preprocess.FieldPath = "/tool_result/stderr"
	PathToolResultStdout        preprocess.FieldPath = "/tool_result/stdout"
	PathToolResultStatus        preprocess.FieldPath = "/tool_result/status"
)

func FrozenOntology() (FactorOntology, error) {
	value := FactorOntology{
		SchemaVersion: FactorOntologySchemaVersion, CanonicalPolicy: CanonicalPolicy,
		OntologyVersion: FactorOntologyVersion, Factors: canonicalFactors(), Operators: canonicalOperators(),
		AmbiguityRules:            canonicalAmbiguityRules(),
		IdentificationAssumptions: canonicalIdentificationAssumptions(),
	}
	digest, err := factorOntologyDigest(value)
	if err != nil {
		return FactorOntology{}, err
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		return FactorOntology{}, err
	}
	return value, nil
}

func (value FactorOntology) Validate() error {
	if value.SchemaVersion != FactorOntologySchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.OntologyVersion != FactorOntologyVersion {
		return errors.New("reliance factor ontology identity is invalid")
	}
	if !reflect.DeepEqual(value.Factors, canonicalFactors()) || !reflect.DeepEqual(value.Operators, canonicalOperators()) ||
		!reflect.DeepEqual(value.AmbiguityRules, canonicalAmbiguityRules()) ||
		!reflect.DeepEqual(value.IdentificationAssumptions, canonicalIdentificationAssumptions()) {
		return errors.New("reliance factor ontology differs from the frozen v1 catalog")
	}
	digest, err := factorOntologyDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance factor ontology digest is invalid")
	}
	return nil
}

func (value FactorOntology) Factor(factorID FactorID) (EvidenceFactor, bool) {
	for _, factor := range value.Factors {
		if factor.FactorID == factorID {
			factor.AllowedTargets = slices.Clone(factor.AllowedTargets)
			return factor, true
		}
	}
	return EvidenceFactor{}, false
}

func (value FactorOntology) Operator(operator InterventionOperator) (OperatorDefinition, bool) {
	for _, definition := range value.Operators {
		if definition.Operator == operator {
			definition.AllowedTargets = slices.Clone(definition.AllowedTargets)
			return definition, true
		}
	}
	return OperatorDefinition{}, false
}

func (factor EvidenceFactor) Allows(target FieldTarget) bool {
	return slices.Contains(factor.AllowedTargets, target)
}

func (definition OperatorDefinition) Allows(target FieldTarget) bool {
	return slices.Contains(definition.AllowedTargets, target)
}

func factorOntologyDigest(value FactorOntology) (string, error) {
	value.Digest = ""
	return protocolkit.Digest(value)
}

func canonicalFactors() []EvidenceFactor {
	factors := []EvidenceFactor{
		{
			FactorID: FactorCommandExit, Family: FamilyExecutableOutcome,
			Description: "Typed command and tool-result exit evidence.",
			AllowedTargets: targets(
				target(preprocess.EventCommand, PathCommandExitCode, ValueInteger, true),
				target(preprocess.EventToolResult, PathToolResultExitCode, ValueInteger, true),
			),
		},
		{
			FactorID: FactorErrorOutput, Family: FamilyErrorToolOutput,
			Description: "Typed errors, failure status, and diagnostic output.",
			AllowedTargets: targets(
				target(preprocess.EventError, PathErrorClass, ValueText, false),
				target(preprocess.EventError, PathErrorSafeMessage, ValueText, true),
				target(preprocess.EventOutput, PathOutputStatus, ValueText, true),
				target(preprocess.EventOutput, PathOutputText, ValueText, true),
				target(preprocess.EventToolResult, PathToolResultError, ValueBoolean, false),
				target(preprocess.EventToolResult, PathToolResultStderr, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStatus, ValueText, false),
			),
		},
		{
			FactorID: FactorExecutableOutcome, Family: FamilyExecutableOutcome,
			Description: "Observable evidence that an executable task succeeded, failed, or remained unresolved.",
			AllowedTargets: targets(
				target(preprocess.EventCommand, PathCommandExitCode, ValueInteger, true),
				target(preprocess.EventEvaluation, PathEvaluationErrorType, ValueText, true),
				target(preprocess.EventEvaluation, PathEvaluationScoreLabel, ValueText, true),
				target(preprocess.EventEvaluation, PathEvaluationScoreValue, ValueText, true),
				target(preprocess.EventOutput, PathOutputStatus, ValueText, true),
				target(preprocess.EventOutput, PathOutputText, ValueText, true),
				target(preprocess.EventToolResult, PathToolResultError, ValueBoolean, false),
				target(preprocess.EventToolResult, PathToolResultExitCode, ValueInteger, true),
				target(preprocess.EventToolResult, PathToolResultOutput, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStderr, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStdout, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStatus, ValueText, false),
			),
		},
		{
			FactorID: FactorIrrelevantVerbosity, Family: FamilyVerbosity,
			Description: "Redundant narrative or output content that adds no registered task evidence.",
			AllowedTargets: targets(
				target(preprocess.EventMessage, PathMessageParts, ValueContentParts, false),
				target(preprocess.EventOutput, PathOutputText, ValueText, true),
				target(preprocess.EventToolResult, PathToolResultOutput, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStderr, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStdout, ValueContentParts, true),
			),
		},
		{
			FactorID: FactorMetadata, Family: FamilyMetadataFormat,
			Description: "Canonical metadata and presentation fields without task-result content.",
			AllowedTargets: targets(
				target(preprocess.EventMessage, PathMessagePhase, ValueText, true),
				target(preprocess.EventMetadata, PathMetadataName, ValueText, false),
				target(preprocess.EventMetadata, PathMetadataPresent, ValueBoolean, false),
				target(preprocess.EventMetadata, PathMetadataValue, ValueText, true),
				target(preprocess.EventMetadata, PathMetadataValueDigest, ValueText, true),
				target(preprocess.EventOutput, PathOutputStream, ValueText, true),
			),
		},
		{
			FactorID: FactorPatchEdit, Family: FamilyPatchEdit,
			Description: "Typed file operations, paths, diffs, and content identities.",
			AllowedTargets: targets(
				target(preprocess.EventFileChange, PathFileChangeContentDigest, ValueText, true),
				target(preprocess.EventFileChange, PathFileChangeDiff, ValueText, true),
				target(preprocess.EventFileChange, PathFileChangeDiffDigest, ValueText, true),
				target(preprocess.EventFileChange, PathFileChangeOperation, ValueText, false),
				target(preprocess.EventFileChange, PathFileChangePathAlias, ValueText, true),
				target(preprocess.EventFileChange, PathFileChangeSizeBytes, ValueInteger, false),
			),
		},
		{
			FactorID: FactorPromptInjection, Family: FamilyAdversarial,
			Description: "Untrusted instructions that attempt to alter evaluator behavior or control-plane state.",
			AllowedTargets: targets(
				target(preprocess.EventMessage, PathMessageParts, ValueContentParts, false),
				target(preprocess.EventOutput, PathOutputText, ValueText, true),
				target(preprocess.EventToolCall, PathToolCallArguments, ValueText, true),
				target(preprocess.EventToolResult, PathToolResultOutput, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStderr, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStdout, ValueContentParts, true),
			),
		},
		{
			FactorID: FactorSuccessFailureProse, Family: FamilyNarrative,
			Description: "Natural-language claims that a task succeeded or failed.",
			AllowedTargets: targets(
				target(preprocess.EventEvaluation, PathEvaluationExplanation, ValueText, true),
				target(preprocess.EventMessage, PathMessageParts, ValueContentParts, false),
				target(preprocess.EventOutput, PathOutputText, ValueText, true),
			),
		},
		{
			FactorID: FactorTestResult, Family: FamilyExecutableOutcome,
			Description: "Test-run output, typed status, scores, and failure evidence.",
			AllowedTargets: targets(
				target(preprocess.EventEvaluation, PathEvaluationErrorType, ValueText, true),
				target(preprocess.EventEvaluation, PathEvaluationExplanation, ValueText, true),
				target(preprocess.EventEvaluation, PathEvaluationScoreLabel, ValueText, true),
				target(preprocess.EventEvaluation, PathEvaluationScoreValue, ValueText, true),
				target(preprocess.EventOutput, PathOutputStatus, ValueText, true),
				target(preprocess.EventOutput, PathOutputText, ValueText, true),
				target(preprocess.EventToolResult, PathToolResultError, ValueBoolean, false),
				target(preprocess.EventToolResult, PathToolResultExitCode, ValueInteger, true),
				target(preprocess.EventToolResult, PathToolResultOutput, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStderr, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStdout, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStatus, ValueText, false),
			),
		},
		{
			FactorID: FactorToolOutput, Family: FamilyErrorToolOutput,
			Description: "Typed tool-result and terminal-output content.",
			AllowedTargets: targets(
				target(preprocess.EventOutput, PathOutputStatus, ValueText, true),
				target(preprocess.EventOutput, PathOutputText, ValueText, true),
				target(preprocess.EventToolResult, PathToolResultError, ValueBoolean, false),
				target(preprocess.EventToolResult, PathToolResultExitCode, ValueInteger, true),
				target(preprocess.EventToolResult, PathToolResultOutput, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStderr, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStdout, ValueContentParts, true),
				target(preprocess.EventToolResult, PathToolResultStatus, ValueText, false),
			),
		},
	}
	slices.SortFunc(factors, func(left, right EvidenceFactor) int {
		return strings.Compare(string(left.FactorID), string(right.FactorID))
	})
	return factors
}

func canonicalOperators() []OperatorDefinition {
	all := allCanonicalTargets()
	optional := slices.DeleteFunc(slices.Clone(all), func(value FieldTarget) bool { return !value.Optional })
	maskable := maskableCanonicalTargets(all)
	operators := []OperatorDefinition{
		{Operator: OperatorControlledReplacement, ChangesEvidence: true, RequiresReplacement: true, AllowedTargets: slices.Clone(all)},
		{Operator: OperatorRemove, ChangesEvidence: true, AllowedTargets: optional},
		{Operator: OperatorRetain, ChangesEvidence: false, AllowedTargets: slices.Clone(all)},
		{Operator: OperatorTypedMask, ChangesEvidence: true, AllowedTargets: maskable},
	}
	slices.SortFunc(operators, func(left, right OperatorDefinition) int {
		return strings.Compare(string(left.Operator), string(right.Operator))
	})
	return operators
}

func allCanonicalTargets() []FieldTarget {
	all := []FieldTarget{
		target(preprocess.EventCommand, PathCommandExitCode, ValueInteger, true),
		target(preprocess.EventError, PathErrorClass, ValueText, false),
		target(preprocess.EventError, PathErrorSafeMessage, ValueText, true),
		target(preprocess.EventEvaluation, PathEvaluationErrorType, ValueText, true),
		target(preprocess.EventEvaluation, PathEvaluationExplanation, ValueText, true),
		target(preprocess.EventEvaluation, PathEvaluationScoreLabel, ValueText, true),
		target(preprocess.EventEvaluation, PathEvaluationScoreValue, ValueText, true),
		target(preprocess.EventFileChange, PathFileChangeContentDigest, ValueText, true),
		target(preprocess.EventFileChange, PathFileChangeDiff, ValueText, true),
		target(preprocess.EventFileChange, PathFileChangeDiffDigest, ValueText, true),
		target(preprocess.EventFileChange, PathFileChangeOperation, ValueText, false),
		target(preprocess.EventFileChange, PathFileChangePathAlias, ValueText, true),
		target(preprocess.EventFileChange, PathFileChangeSizeBytes, ValueInteger, false),
		target(preprocess.EventMessage, PathMessageParts, ValueContentParts, false),
		target(preprocess.EventMessage, PathMessagePhase, ValueText, true),
		target(preprocess.EventMetadata, PathMetadataName, ValueText, false),
		target(preprocess.EventMetadata, PathMetadataPresent, ValueBoolean, false),
		target(preprocess.EventMetadata, PathMetadataValue, ValueText, true),
		target(preprocess.EventMetadata, PathMetadataValueDigest, ValueText, true),
		target(preprocess.EventOutput, PathOutputStatus, ValueText, true),
		target(preprocess.EventOutput, PathOutputStream, ValueText, true),
		target(preprocess.EventOutput, PathOutputText, ValueText, true),
		target(preprocess.EventToolCall, PathToolCallArguments, ValueText, true),
		target(preprocess.EventToolResult, PathToolResultError, ValueBoolean, false),
		target(preprocess.EventToolResult, PathToolResultExitCode, ValueInteger, true),
		target(preprocess.EventToolResult, PathToolResultOutput, ValueContentParts, true),
		target(preprocess.EventToolResult, PathToolResultStderr, ValueContentParts, true),
		target(preprocess.EventToolResult, PathToolResultStdout, ValueContentParts, true),
		target(preprocess.EventToolResult, PathToolResultStatus, ValueText, false),
	}
	return targets(all...)
}

func canonicalAmbiguityRules() []string {
	return []string{
		"assignment_must_be_precommitted",
		"duplicate_target_rejected",
		"factor_path_mismatch_rejected",
		"multiple_factor_assignment_rejected",
		"operator_path_mismatch_rejected",
		"unassigned_target_rejected",
		"unknown_event_rejected",
		"unknown_factor_rejected",
		"unknown_field_rejected",
	}
}

func canonicalIdentificationAssumptions() []string {
	return []string{
		"assignment_commitment_precedes_verifier_output",
		"construct_admission_independent_of_verifier_output",
		"exact_replay_or_declared_live_cell",
		"executable_quality_preserved_for_evidence_only",
		"frozen_verifier_contract",
		"no_cross_case_interference",
		"no_unrecorded_field_changes",
		"outcome_disagreement_not_resolved_by_verifier",
		"task_identity_preserved",
		"within_task_intervention_randomized",
	}
}

func maskableCanonicalTargets(all []FieldTarget) []FieldTarget {
	allowed := map[preprocess.FieldPath]struct{}{
		PathErrorSafeMessage: {}, PathEvaluationExplanation: {}, PathFileChangeDiff: {},
		PathMessageParts: {}, PathMetadataValue: {}, PathOutputText: {}, PathToolCallArguments: {},
		PathToolResultOutput: {}, PathToolResultStderr: {}, PathToolResultStdout: {},
	}
	return slices.DeleteFunc(slices.Clone(all), func(value FieldTarget) bool {
		_, found := allowed[value.FieldPath]
		return !found
	})
}

func target(kind preprocess.EventKind, path preprocess.FieldPath, valueKind FieldValueKind, optional bool) FieldTarget {
	return FieldTarget{EventKind: kind, FieldPath: path, ValueKind: valueKind, Optional: optional}
}

func targets(values ...FieldTarget) []FieldTarget {
	result := slices.Clone(values)
	slices.SortFunc(result, compareTargets)
	return slices.Compact(result)
}

func compareTargets(left, right FieldTarget) int {
	leftKey := string(left.EventKind) + "\x00" + string(left.FieldPath) + "\x00" + string(left.ValueKind)
	rightKey := string(right.EventKind) + "\x00" + string(right.FieldPath) + "\x00" + string(right.ValueKind)
	if value := strings.Compare(leftKey, rightKey); value != 0 {
		return value
	}
	return compareBooleans(left.Optional, right.Optional)
}

func compareBooleans(left, right bool) int {
	switch {
	case left == right:
		return 0
	case !left:
		return -1
	default:
		return 1
	}
}

func validateFactorTarget(ontology FactorOntology, factorID FactorID, target FieldTarget) error {
	factor, found := ontology.Factor(factorID)
	if !found {
		return fmt.Errorf("unknown reliance factor %q", factorID)
	}
	if !factor.Allows(target) {
		return fmt.Errorf("reliance factor %q does not allow %s%s", factorID, target.EventKind, target.FieldPath)
	}
	return nil
}
