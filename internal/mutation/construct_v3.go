package mutation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
)

const (
	InvocationParserVersion       = "evalwitness.shell-invocation.v1"
	PresentationClassifierVersion = "evalwitness.presentation-content-kind.v1"
)

func (report ConstructFirewallReportV2) Validate() error {
	if report.SchemaVersion != ConstructFirewallSchemaVersionV2 || report.CanonicalPolicy != CanonicalPolicy || report.ProgramVersion != MutationProgramVersionV3 {
		return errors.New("construct firewall v2 schema, canonical policy, or program version is unsupported")
	}
	definition, exists := DefinitionFor(report.Family)
	if !exists || !validDigest(report.SourceTrajectoryDigest) {
		return errors.New("construct firewall v2 family or source trajectory is invalid")
	}
	if !slices.Contains([]ConstructStatus{ConstructApplied, ConstructRejected}, report.Status) {
		return errors.New("construct firewall v2 status is invalid")
	}
	if err := validateConstructRoleV3(report); err != nil {
		return err
	}
	if err := validateUniqueSortedStrings("construct firewall v2 target event IDs", report.TargetEventIDs); err != nil {
		return err
	}
	if err := validateUniqueSortedStrings("construct firewall v2 proof event IDs", report.ProofEventIDs); err != nil {
		return err
	}
	if len(report.Checks) == 0 {
		return errors.New("construct firewall v2 requires at least one check")
	}
	seenChecks := make(map[string]struct{}, len(report.Checks))
	allPassed := true
	for index, check := range report.Checks {
		if missing(check.Name, check.Expected, check.Observed) {
			return fmt.Errorf("construct firewall v2 check %d is incomplete", index)
		}
		if _, exists := seenChecks[check.Name]; exists {
			return fmt.Errorf("duplicate construct firewall v2 check %q", check.Name)
		}
		seenChecks[check.Name] = struct{}{}
		allPassed = allPassed && check.Passed
	}
	if err := validateConstructRejectionReasons(report.RejectionReasons); err != nil {
		return err
	}
	if err := validateV3TypedProof(report); err != nil {
		return err
	}
	switch report.Status {
	case ConstructApplied:
		if !validDigest(report.MutatedTrajectoryDigest) || report.MutatedTrajectoryDigest == report.SourceTrajectoryDigest || (!definition.PairLevel && (len(report.TargetEventIDs) == 0 || len(report.ProofEventIDs) == 0)) || len(report.RejectionReasons) != 0 || !allPassed {
			return errors.New("applied construct firewall v2 report is not fully proven")
		}
		for _, eventID := range report.TargetEventIDs {
			if !slices.Contains(report.ProofEventIDs, eventID) {
				return fmt.Errorf("construct firewall v2 target event %q is absent from its proof lineage", eventID)
			}
		}
	case ConstructRejected:
		if report.MutatedTrajectoryDigest != "" || len(report.RejectionReasons) == 0 || allPassed {
			return errors.New("rejected construct firewall v2 report lacks a closed failed reason")
		}
	}
	expected, err := report.digest()
	if err != nil {
		return err
	}
	if report.Digest != expected {
		return errors.New("construct firewall v2 digest is invalid")
	}
	return nil
}

func validateConstructRoleV3(report ConstructFirewallReportV2) error {
	if report.Status == ConstructRejected && report.SemanticRole == "" {
		return nil
	}
	allowed := []string{"registered_operator"}
	switch report.Family {
	case FamilyTestEvidenceOmitted:
		allowed = []string{"test", "check", "build", "outcome_probe"}
	case FamilyNeutralFormatting:
		allowed = []string{"assistant_prose"}
	case FamilyCausalIndependentReorder:
		allowed = []string{"transaction_independent_events"}
	case FamilyCandidateOrderReversal:
		allowed = []string{"candidate_pair_presentation"}
	}
	if !slices.Contains(allowed, report.SemanticRole) {
		return fmt.Errorf("construct firewall v2 semantic role %q is invalid for family %q", report.SemanticRole, report.Family)
	}
	return nil
}

func validateV3TypedProof(report ConstructFirewallReportV2) error {
	switch report.Family {
	case FamilyTestEvidenceOmitted:
		if report.Status == ConstructRejected && slices.Contains(report.RejectionReasons, RejectionNoApplicableTarget) && report.Invocation == nil && report.Presentation == nil {
			return nil
		}
		if report.Invocation == nil || report.Presentation != nil {
			return errors.New("v3 omitted-test-evidence firewall requires only an invocation proof")
		}
		proof := report.Invocation
		if proof.ParserVersion != InvocationParserVersion || missing(proof.EvidenceEventID, proof.Executable, proof.Decision) || proof.SegmentIndex < 0 || !slices.Contains([]InvocationParseStatus{InvocationParsed, InvocationRejected}, proof.ParseStatus) {
			return errors.New("v3 invocation proof identity or parser result is invalid")
		}
		for _, eventID := range []string{proof.ToolCallEventID, proof.CommandEventID, proof.EvidenceEventID} {
			if eventID != "" && !slices.Contains(report.ProofEventIDs, eventID) {
				return fmt.Errorf("v3 invocation proof event %q is absent from firewall lineage", eventID)
			}
		}
		if report.Status == ConstructApplied {
			if missing(proof.ToolCallEventID, proof.CommandEventID) || proof.ParseStatus != InvocationParsed || !proof.DirectInvocation || proof.SemanticRole != report.SemanticRole {
				return errors.New("applied v3 omitted-test-evidence proof is not a direct verification invocation")
			}
		} else if proof.DirectInvocation || proof.SemanticRole != "" {
			return errors.New("rejected v3 omitted-test-evidence proof claims a verification invocation")
		}
	case FamilyNeutralFormatting:
		if report.Status == ConstructRejected && slices.Contains(report.RejectionReasons, RejectionNoApplicableTarget) && report.Presentation == nil && report.Invocation == nil {
			return nil
		}
		if report.Presentation == nil || report.Invocation != nil {
			return errors.New("v3 neutral-formatting firewall requires only a presentation proof")
		}
		proof := report.Presentation
		if proof.ClassifierVersion != PresentationClassifierVersion || missing(proof.EventID, proof.MessageRole, proof.Decision) || proof.TextPartCount < 0 || proof.TokenCount < 0 || proof.LineCount < 1 || proof.MaximumLineBytes < 0 || !slices.Contains(report.ProofEventIDs, proof.EventID) {
			return errors.New("v3 presentation proof identity or metrics are invalid")
		}
		if report.Status == ConstructApplied && (proof.MessageRole != "assistant" || proof.ContentKind != PresentationAssistantProse || proof.Decision != "assistant_prose_wrap_eligible" || report.SemanticRole != string(PresentationAssistantProse)) {
			return errors.New("applied v3 neutral-formatting proof is not assistant prose")
		}
		if report.Status == ConstructRejected && proof.Decision == "assistant_prose_wrap_eligible" {
			return errors.New("rejected v3 neutral-formatting proof claims wrap eligibility")
		}
	default:
		if report.Invocation != nil || report.Presentation != nil {
			return errors.New("v3 typed construct proof is not valid for this family")
		}
	}
	return nil
}

func sealConstructFirewallV3(report ConstructFirewallReportV2) (ConstructFirewallReportV2, error) {
	report.SchemaVersion = ConstructFirewallSchemaVersionV2
	report.CanonicalPolicy = CanonicalPolicy
	report.ProgramVersion = MutationProgramVersionV3
	report.TargetEventIDs = sortedStrings(report.TargetEventIDs)
	report.ProofEventIDs = sortedStrings(report.ProofEventIDs)
	sort.Slice(report.RejectionReasons, func(left, right int) bool {
		return report.RejectionReasons[left] < report.RejectionReasons[right]
	})
	report.Digest = ""
	digest, err := report.digest()
	if err != nil {
		return ConstructFirewallReportV2{}, err
	}
	report.Digest = digest
	if err := report.Validate(); err != nil {
		return ConstructFirewallReportV2{}, err
	}
	return report, nil
}

func (report ConstructFirewallReportV2) digest() (string, error) {
	report.Digest = ""
	return digestJSON(report)
}
