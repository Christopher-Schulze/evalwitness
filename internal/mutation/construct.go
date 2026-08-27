package mutation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

type constructRejection struct {
	reasons      []ConstructRejectionReason
	eventIDs     []string
	role         string
	checks       []Check
	invocation   *InvocationProof
	presentation *PresentationProof
}

func (rejection constructRejection) Error() string {
	values := make([]string, len(rejection.reasons))
	for index, reason := range rejection.reasons {
		values[index] = string(reason)
	}
	return "construct firewall rejected mutation: " + strings.Join(values, ",")
}

func rejectConstruct(reason ConstructRejectionReason, eventIDs []string, role string, checks ...Check) error {
	return constructRejection{reasons: []ConstructRejectionReason{reason}, eventIDs: sortedStrings(eventIDs), role: role, checks: checks}
}

func (report ConstructFirewallReport) Validate() error {
	if report.SchemaVersion != ConstructFirewallSchemaVersion || report.CanonicalPolicy != CanonicalPolicy || report.ProgramVersion != MutationProgramVersionV2 {
		return errors.New("construct firewall schema, canonical policy, or program version is unsupported")
	}
	definition, exists := DefinitionFor(report.Family)
	if !exists || !validDigest(report.SourceTrajectoryDigest) {
		return errors.New("construct firewall family or source trajectory is invalid")
	}
	if !slices.Contains([]ConstructStatus{ConstructApplied, ConstructRejected}, report.Status) {
		return errors.New("construct firewall status is invalid")
	}
	if err := validateConstructRole(report); err != nil {
		return err
	}
	if err := validateUniqueSortedStrings("construct firewall target event IDs", report.TargetEventIDs); err != nil {
		return err
	}
	if err := validateUniqueSortedStrings("construct firewall proof event IDs", report.ProofEventIDs); err != nil {
		return err
	}
	if len(report.Checks) == 0 {
		return errors.New("construct firewall requires at least one check")
	}
	seenChecks := make(map[string]struct{}, len(report.Checks))
	allPassed := true
	for index, check := range report.Checks {
		if missing(check.Name, check.Expected, check.Observed) {
			return fmt.Errorf("construct firewall check %d is incomplete", index)
		}
		if _, exists := seenChecks[check.Name]; exists {
			return fmt.Errorf("duplicate construct firewall check %q", check.Name)
		}
		seenChecks[check.Name] = struct{}{}
		allPassed = allPassed && check.Passed
	}
	if err := validateConstructRejectionReasons(report.RejectionReasons); err != nil {
		return err
	}
	switch report.Status {
	case ConstructApplied:
		if !validDigest(report.MutatedTrajectoryDigest) || report.MutatedTrajectoryDigest == report.SourceTrajectoryDigest || (!definition.PairLevel && (len(report.TargetEventIDs) == 0 || len(report.ProofEventIDs) == 0)) || len(report.RejectionReasons) != 0 || !allPassed {
			return errors.New("applied construct firewall report is not fully proven")
		}
		for _, eventID := range report.TargetEventIDs {
			if !slices.Contains(report.ProofEventIDs, eventID) {
				return fmt.Errorf("construct firewall target event %q is absent from its proof lineage", eventID)
			}
		}
	case ConstructRejected:
		if report.MutatedTrajectoryDigest != "" || len(report.RejectionReasons) == 0 || allPassed {
			return errors.New("rejected construct firewall report lacks a closed failed reason")
		}
	}
	expected, err := report.digest()
	if err != nil {
		return err
	}
	if report.Digest != expected {
		return errors.New("construct firewall digest is invalid")
	}
	return nil
}

func validateConstructRole(report ConstructFirewallReport) error {
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
		return fmt.Errorf("construct firewall semantic role %q is invalid for family %q", report.SemanticRole, report.Family)
	}
	return nil
}

func validateConstructRejectionReasons(reasons []ConstructRejectionReason) error {
	if !slices.IsSorted(reasons) {
		return errors.New("construct firewall rejection reasons must be sorted")
	}
	allowed := []ConstructRejectionReason{
		RejectionNoApplicableTarget,
		RejectionPreservationFailure,
		RejectionTemporalDependency,
		RejectionTokenSequenceChanged,
		RejectionTransactionDependency,
		RejectionUnnaturalFormatting,
		RejectionUnverifiedEvidenceRole,
	}
	seen := make(map[ConstructRejectionReason]struct{}, len(reasons))
	for _, reason := range reasons {
		if !slices.Contains(allowed, reason) {
			return fmt.Errorf("unsupported construct firewall rejection reason %q", reason)
		}
		if _, exists := seen[reason]; exists {
			return fmt.Errorf("duplicate construct firewall rejection reason %q", reason)
		}
		seen[reason] = struct{}{}
	}
	return nil
}

func sealConstructFirewall(report ConstructFirewallReport) (ConstructFirewallReport, error) {
	report.SchemaVersion = ConstructFirewallSchemaVersion
	report.CanonicalPolicy = CanonicalPolicy
	report.ProgramVersion = MutationProgramVersionV2
	report.TargetEventIDs = sortedStrings(report.TargetEventIDs)
	report.ProofEventIDs = sortedStrings(report.ProofEventIDs)
	sort.Slice(report.RejectionReasons, func(left, right int) bool {
		return report.RejectionReasons[left] < report.RejectionReasons[right]
	})
	report.Digest = ""
	digest, err := report.digest()
	if err != nil {
		return ConstructFirewallReport{}, err
	}
	report.Digest = digest
	if err := report.Validate(); err != nil {
		return ConstructFirewallReport{}, err
	}
	return report, nil
}

func (report ConstructFirewallReport) digest() (string, error) {
	report.Digest = ""
	return digestJSON(report)
}
