package mutation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func ApplyV3(parent preprocess.Trajectory, request ApplyRequest) (ApplyV3Outcome, error) {
	if err := parent.Validate(); err != nil {
		return ApplyV3Outcome{}, fmt.Errorf("validate mutation source: %w", err)
	}
	definition, exists := DefinitionFor(request.Family)
	if !exists {
		return ApplyV3Outcome{}, fmt.Errorf("unsupported mutation family %q", request.Family)
	}
	if definition.PairLevel {
		return ApplyV3Outcome{}, errors.New("candidate-order reversal requires its pair-level v3 operator")
	}
	if missing(request.CorpusVersion, request.TaskID, request.RepositoryID, request.SourceFamily, request.SourceLocation,
		request.SourceRevision, request.SplitGroupID, request.Seed) {
		return ApplyV3Outcome{}, errors.New("mutation request source, corpus, split, and seed are required")
	}
	events, err := cloneEvents(parent.Events)
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	change, err := applyTrajectoryMutationV3(parent, events, request, definition)
	if err != nil {
		return rejectedV3Outcome(parent, request, err)
	}
	child, err := preprocess.DeriveTrajectory(parent, change.events, change.links, preprocess.DerivationSpec{
		Relation: string(definition.Relation), Validator: request.Validator.ID,
		ChangedEventIDs: change.eventIDs, ChangedFieldPaths: change.fieldPaths,
	})
	if err != nil {
		return ApplyV3Outcome{}, fmt.Errorf("derive v3 mutated trajectory: %w", err)
	}
	preservation, preservationChecks, err := buildPreservationV2(parent, child, definition)
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	preservation.BoundaryVersion = EvidenceBoundaryVersionV3
	checks := append(append([]Check{}, change.checks...), preservationChecks...)
	label := LabelProven
	relation := definition.Relation
	if relation == RelationAmbiguous {
		label = LabelAmbiguous
		preservation.AmbiguityReasons = append(preservation.AmbiguityReasons, "prespecified_ambiguity")
		sort.Strings(preservation.AmbiguityReasons)
	}
	for _, check := range checks {
		if !check.Passed {
			return rejectedV3Outcome(parent, request, constructRejection{
				reasons: []ConstructRejectionReason{RejectionPreservationFailure}, eventIDs: change.eventIDs,
				role: constructRole(change), checks: checks, invocation: change.invocation, presentation: change.presentation,
			})
		}
	}
	firewall, err := sealConstructFirewallV3(ConstructFirewallReportV2{
		Family: request.Family, Status: ConstructApplied, SourceTrajectoryDigest: parent.Digest,
		MutatedTrajectoryDigest: child.Digest, TargetEventIDs: change.eventIDs,
		ProofEventIDs: constructProofEventIDs(change), SemanticRole: constructRole(change),
		Invocation: change.invocation, Presentation: change.presentation,
		Checks: checks, RejectionReasons: []ConstructRejectionReason{},
	})
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	witness, err := SealWitness(Witness{
		ValidatorID: request.Validator.ID, ValidatorVersion: request.Validator.Version,
		Relation: relation, LabelState: label, Checks: checks,
	})
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	packet, err := SealBlindReviewPacket(BlindReviewPacket{
		MutationMaterialDigest: digestText(parent.Digest + "\x00" + child.Digest + "\x00" + string(request.Family) + "\x00" + firewall.Digest),
		TaskAlias:              "task-" + digestText(request.TaskID)[:16], SourceFormat: parent.SourceFormat,
		OriginalDigest: parent.Digest, MutatedDigest: child.Digest, AffectedEventCount: len(change.eventIDs),
		ReviewQuestions: []string{
			"Did the transformation alter task-level semantic quality?",
			"Does the typed construct-firewall proof match the visible change?",
			"Is either trajectory ambiguous without hidden source information?",
		},
	})
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	parameters := append([]NamedValue(nil), change.parameters...)
	sort.Slice(parameters, func(left, right int) bool { return parameters[left].Name < parameters[right].Name })
	manifest, err := SealManifestV3(Manifest{
		CorpusVersion: request.CorpusVersion,
		Source: SourceRef{
			TaskID: request.TaskID, RepositoryID: request.RepositoryID, SourceFamily: request.SourceFamily,
			SourceFormat: parent.SourceFormat, SourceLocation: request.SourceLocation, SourceRevision: request.SourceRevision,
			SourceDigest: parent.SourceDigest, TrajectoryDigest: parent.Digest, Outcome: request.Outcome,
		},
		Program: Program{
			Family: request.Family, Seed: request.Seed, Operator: operatorForProgram(MutationProgramVersionV3, definition),
			Parameters: parameters,
		},
		Class: definition.Class, ExpectedRelation: relation,
		Affected: AffectedSurface{
			EventIDs: sortedStrings(change.eventIDs), FieldPaths: sortedFieldPaths(change.fieldPaths), FileAliases: sortedStrings(change.fileAliases),
		},
		Validator: request.Validator, Preservation: preservation, Witness: witness,
		OutcomeProof: request.OutcomeProof,
		License:      request.License, Privacy: request.Privacy, SplitGroupID: request.SplitGroupID,
		OriginalTrajectoryDigest: parent.Digest, MutatedTrajectoryDigest: child.Digest,
		Review: ReviewState{
			Required: label == LabelAmbiguous || request.ReviewSampled, SamplingStratum: request.ReviewSamplingStratum, BlindPacketDigest: packet.Digest,
		},
		ConstructFirewallDigest: firewall.Digest,
	})
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	result := ApplyResult{Manifest: manifest, Mutated: child, Packet: packet}
	return ApplyV3Outcome{Status: ConstructApplied, Applied: &result, Firewall: firewall}, nil
}

func rejectedV3Outcome(parent preprocess.Trajectory, request ApplyRequest, cause error) (ApplyV3Outcome, error) {
	var rejection constructRejection
	if !errors.As(cause, &rejection) {
		if !strings.Contains(cause.Error(), "no applicable target event") && !strings.Contains(cause.Error(), "no causally independent") {
			return ApplyV3Outcome{}, cause
		}
		rejection = constructRejection{
			reasons: []ConstructRejectionReason{RejectionNoApplicableTarget},
			checks:  []Check{{Name: "applicable_target", Expected: "present", Observed: "absent", Passed: false}},
		}
	}
	if len(rejection.checks) == 0 {
		rejection.checks = []Check{{Name: "construct_eligibility", Expected: "eligible", Observed: "rejected", Passed: false}}
	}
	proofEventIDs := rejection.eventIDs
	if rejection.invocation != nil {
		proofEventIDs = invocationProofEventIDs(*rejection.invocation)
	}
	if rejection.presentation != nil {
		if !slices.Contains(proofEventIDs, rejection.presentation.EventID) {
			proofEventIDs = append(proofEventIDs, rejection.presentation.EventID)
		}
		proofEventIDs = sortedStrings(proofEventIDs)
	}
	firewall, err := sealConstructFirewallV3(ConstructFirewallReportV2{
		Family: request.Family, Status: ConstructRejected, SourceTrajectoryDigest: parent.Digest,
		TargetEventIDs: rejection.eventIDs, ProofEventIDs: proofEventIDs, SemanticRole: rejection.role,
		Invocation: rejection.invocation, Presentation: rejection.presentation,
		Checks: rejection.checks, RejectionReasons: rejection.reasons,
	})
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	return ApplyV3Outcome{Status: ConstructRejected, Firewall: firewall}, nil
}

func applyTrajectoryMutationV3(parent preprocess.Trajectory, events []preprocess.Event, request ApplyRequest, definition Definition) (mutationChange, error) {
	switch request.Family {
	case FamilyTestEvidenceOmitted:
		return mutateVerifiedEvidenceV3(parent, events, request)
	case FamilyNeutralFormatting:
		return mutateNaturalFormattingV3(parent, events, request)
	case FamilyCausalIndependentReorder:
		return mutateIndependentOrderV2(parent, events, request)
	default:
		change, err := applyTrajectoryMutation(parent, events, request, definition)
		if err == nil {
			change.parameters = append(change.parameters, NamedValue{Name: "construct_role", Value: "registered_operator"})
			change.checks = append(change.checks, Check{Name: "registered_v3_operator", Expected: "registered", Observed: "registered", Passed: true})
		}
		return change, err
	}
}

func mutateVerifiedEvidenceV3(parent preprocess.Trajectory, events []preprocess.Event, request ApplyRequest) (mutationChange, error) {
	type candidate struct {
		index int
		proof InvocationProof
		role  string
	}
	var candidates []candidate
	var rejected []candidate
	for index, event := range events {
		if request.TargetEventID != "" && event.ID != request.TargetEventID {
			continue
		}
		if eventEvidenceText(event) == "" {
			continue
		}
		proof, role, eligible := verifiedExecutionInvocation(parent, event)
		item := candidate{index: index, proof: proof, role: role}
		if eligible {
			candidates = append(candidates, item)
		} else {
			rejected = append(rejected, item)
		}
	}
	if len(candidates) == 0 {
		if len(rejected) == 0 {
			return mutationChange{}, rejectConstruct(RejectionNoApplicableTarget, nil, "", Check{
				Name: "evidence_target_available", Expected: "present", Observed: "absent", Passed: false,
			})
		}
		indices := make([]int, len(rejected))
		byIndex := make(map[int]candidate, len(rejected))
		for index, item := range rejected {
			indices[index] = item.index
			byIndex[item.index] = item
		}
		selected, err := deterministicIndex(events, indices, request)
		if err != nil {
			return mutationChange{}, err
		}
		item := byIndex[selected]
		return mutationChange{}, constructRejection{
			reasons:  []ConstructRejectionReason{RejectionUnverifiedEvidenceRole},
			eventIDs: []string{events[selected].ID}, invocation: &item.proof,
			checks: []Check{{Name: "direct_verification_invocation", Expected: "present", Observed: describeInvocation(item.proof), Passed: false}},
		}
	}
	indices := make([]int, len(candidates))
	byIndex := make(map[int]candidate, len(candidates))
	for index, item := range candidates {
		indices[index] = item.index
		byIndex[item.index] = item
	}
	selected, err := deterministicIndex(events, indices, request)
	if err != nil {
		return mutationChange{}, err
	}
	item := byIndex[selected]
	originalID := events[selected].ID
	before := eventEvidenceText(events[selected])
	setEventEvidenceText(&events[selected], "")
	change, err := rebuildMutationEvent(parent, events, selected, originalID, "/evidence", []NamedValue{
		{Name: "construct_role", Value: item.role}, {Name: "operation", Value: "omit_direct_verification_evidence"},
		{Name: "invocation_parser", Value: InvocationParserVersion},
	}, []Check{
		{Name: "direct_verification_invocation", Expected: "present", Observed: describeInvocation(item.proof), Passed: true},
		{Name: "evidence_nonempty_before", Expected: "nonempty", Observed: fmt.Sprintf("bytes=%d", len(before)), Passed: before != ""},
		{Name: "evidence_empty_after", Expected: "empty", Observed: fmt.Sprintf("bytes=%d", len(eventEvidenceText(events[selected]))), Passed: eventEvidenceText(events[selected]) == ""},
	})
	change.proofEventIDs = invocationProofEventIDs(item.proof)
	change.invocation = &item.proof
	return change, err
}

func mutateNaturalFormattingV3(parent preprocess.Trajectory, events []preprocess.Event, request ApplyRequest) (mutationChange, error) {
	afterByEventID := make(map[string]string)
	proofByEventID := make(map[string]PresentationProof)
	indices := candidateIndices(events, request.TargetEventID, func(event preprocess.Event) bool {
		after, proof, eligible := naturalFormattingV3(event)
		proofByEventID[event.ID] = proof
		if eligible {
			afterByEventID[event.ID] = after
		}
		return eligible
	})
	selected, err := deterministicIndex(events, indices, request)
	if err != nil {
		var rejectedIndices []int
		for index, event := range events {
			if request.TargetEventID != "" && event.ID != request.TargetEventID {
				continue
			}
			if event.Message != nil {
				rejectedIndices = append(rejectedIndices, index)
				if _, exists := proofByEventID[event.ID]; !exists {
					proofByEventID[event.ID] = presentationProof(event)
				}
			}
		}
		if len(rejectedIndices) == 0 {
			return mutationChange{}, rejectConstruct(RejectionNoApplicableTarget, nil, "", Check{
				Name: "message_target_available", Expected: "present", Observed: "absent", Passed: false,
			})
		}
		selected, selectErr := deterministicIndex(events, rejectedIndices, request)
		if selectErr != nil {
			return mutationChange{}, selectErr
		}
		proof := proofByEventID[events[selected].ID]
		return mutationChange{}, constructRejection{
			reasons: []ConstructRejectionReason{RejectionUnnaturalFormatting}, eventIDs: []string{events[selected].ID},
			presentation: &proof,
			checks:       []Check{{Name: "assistant_prose_content_kind", Expected: string(PresentationAssistantProse), Observed: string(proof.ContentKind), Passed: false}},
		}
	}
	originalID := events[selected].ID
	before := eventText(events[selected])
	after := afterByEventID[events[selected].ID]
	proof := proofByEventID[events[selected].ID]
	beforeTokens := strings.Fields(before)
	afterTokens := strings.Fields(after)
	if strings.Join(beforeTokens, "\x00") != strings.Join(afterTokens, "\x00") {
		return mutationChange{}, constructRejection{
			reasons: []ConstructRejectionReason{RejectionTokenSequenceChanged}, eventIDs: []string{originalID}, role: string(PresentationAssistantProse),
			presentation: &proof,
			checks:       []Check{{Name: "token_sequence_preserved", Expected: "equal", Observed: "different", Passed: false}},
		}
	}
	setEventText(&events[selected], after)
	change, err := rebuildMutationEvent(parent, events, selected, originalID, "/text", []NamedValue{
		{Name: "construct_role", Value: string(PresentationAssistantProse)}, {Name: "operation", Value: "natural_line_wrap"},
		{Name: "presentation_classifier", Value: PresentationClassifierVersion}, {Name: "wrap_width", Value: "72"},
	}, []Check{
		{Name: "assistant_prose_content_kind", Expected: string(PresentationAssistantProse), Observed: string(proof.ContentKind), Passed: true},
		{Name: "token_sequence_preserved", Expected: "equal", Observed: "equal", Passed: true},
		{Name: "single_token_paragraphs_absent", Expected: "absent", Observed: fmt.Sprintf("present=%t", hasSingleTokenLine(after)), Passed: !hasSingleTokenLine(after)},
		{Name: "formatting_changed", Expected: "changed", Observed: fmt.Sprintf("changed=%t", before != after), Passed: before != after},
	})
	change.proofEventIDs = []string{originalID}
	change.presentation = &proof
	return change, err
}
