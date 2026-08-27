package outcome

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"
)

func SealPrivateMapping(mapping PrivateMapping) (PrivateMapping, error) {
	mapping.SchemaVersion = MappingSchemaVersion
	mapping.CanonicalPolicy = CanonicalPolicy
	mapping.Digest = ""
	digest, err := privateMappingDigest(mapping)
	if err != nil {
		return PrivateMapping{}, err
	}
	mapping.Digest = digest
	return mapping, mapping.Validate()
}

func (mapping PrivateMapping) Validate() error {
	if mapping.SchemaVersion != MappingSchemaVersion || mapping.CanonicalPolicy != CanonicalPolicy ||
		missing(mapping.PacketID, mapping.SourceTaskAlias, mapping.Condition, mapping.ExpectedRelation, mapping.BlindingKeyID) ||
		!validOpaquePacketID(mapping.PacketID) || !validDigest(mapping.SourceCaseDigest) || len(mapping.SlotMappings) == 0 || !validDigest(mapping.Digest) {
		return errors.New("private outcome mapping identity, condition, relation, key, or source is invalid")
	}
	for index, slot := range mapping.SlotMappings {
		if !validOpaqueSlot(slot.Slot) || !validDigest(slot.SourceDigest) || index > 0 && mapping.SlotMappings[index-1].Slot >= slot.Slot {
			return errors.New("private outcome mapping slots must be valid, unique, and sorted")
		}
	}
	expected, err := privateMappingDigest(mapping)
	if err != nil || mapping.Digest != expected {
		return errors.New("private outcome mapping digest is invalid")
	}
	return nil
}

func ResolveLabels(primary []Label, tieBreak *Label, resolvedAt, rule string) (Resolution, error) {
	if len(primary) != 2 || missing(resolvedAt, rule) {
		return Resolution{}, errors.New("outcome resolution requires two primary labels, time, and rule")
	}
	reviewerSlots := make(map[int]struct{}, len(primary))
	for index, label := range primary {
		if err := label.Validate(); err != nil {
			return Resolution{}, fmt.Errorf("primary outcome label %d: %w", index, err)
		}
		if label.ReviewerSlot > 2 {
			return Resolution{}, errors.New("primary outcome labels must use reviewer slots one or two")
		}
		reviewerSlots[label.ReviewerSlot] = struct{}{}
	}
	if len(reviewerSlots) != 2 || primary[0].PacketID != primary[1].PacketID || primary[0].AdjudicatorAlias == primary[1].AdjudicatorAlias {
		return Resolution{}, errors.New("primary outcome labels must be independent labels for one packet")
	}
	digests := []string{primary[0].Digest, primary[1].Digest}
	sort.Strings(digests)
	resolution := Resolution{
		PacketID: primary[0].PacketID, PrimaryLabelDigests: digests,
		ResolvedAt: resolvedAt, Rule: rule,
	}
	if primary[0].PrimaryOutcome == primary[1].PrimaryOutcome {
		if tieBreak != nil {
			return Resolution{}, errors.New("agreeing primary outcome labels cannot use a tie-break label")
		}
		resolution.State = primary[0].PrimaryOutcome
		resolution.AgreementState = "primary_agreement"
	} else if tieBreak == nil {
		resolution.State = StateIndeterminate
		resolution.AgreementState = "unresolved_disagreement"
	} else {
		if err := tieBreak.Validate(); err != nil {
			return Resolution{}, err
		}
		if tieBreak.PacketID != resolution.PacketID || tieBreak.ReviewerSlot != 3 ||
			tieBreak.AdjudicatorAlias == primary[0].AdjudicatorAlias || tieBreak.AdjudicatorAlias == primary[1].AdjudicatorAlias {
			return Resolution{}, errors.New("tie-break outcome label is not an independent round-three label for the packet")
		}
		resolution.State = tieBreak.PrimaryOutcome
		resolution.AgreementState = "third_adjudicator_resolution"
		resolution.TieBreakLabelDigest = tieBreak.Digest
	}
	return SealResolution(resolution)
}

func ResolveQualifiedLabels(primary []Label, qualifications []QualificationReport, tieBreak *Label, tieQualification *QualificationReport, resolvedAt, rule string) (Resolution, error) {
	if len(primary) != 2 || len(qualifications) != 2 {
		return Resolution{}, errors.New("qualified outcome resolution requires two primary labels and two qualification reports")
	}
	for index := range primary {
		if err := ValidateReviewerQualification(primary[index], qualifications[index]); err != nil {
			return Resolution{}, fmt.Errorf("primary reviewer %d qualification: %w", index+1, err)
		}
	}
	if (tieBreak == nil) != (tieQualification == nil) {
		return Resolution{}, errors.New("tie-break label and qualification report must be supplied together")
	}
	if tieBreak != nil {
		if err := ValidateReviewerQualification(*tieBreak, *tieQualification); err != nil {
			return Resolution{}, fmt.Errorf("tie-break reviewer qualification: %w", err)
		}
	}
	return ResolveLabels(primary, tieBreak, resolvedAt, rule)
}

func SealResolution(resolution Resolution) (Resolution, error) {
	resolution.SchemaVersion = ResolutionSchemaVersion
	resolution.CanonicalPolicy = CanonicalPolicy
	resolution.ResolutionID = ""
	resolution.Digest = ""
	digest, err := resolutionDigest(resolution)
	if err != nil {
		return Resolution{}, err
	}
	resolution.Digest = digest
	resolution.ResolutionID = "resolution-" + digest
	return resolution, resolution.Validate()
}

func (resolution Resolution) Validate() error {
	if resolution.SchemaVersion != ResolutionSchemaVersion || resolution.CanonicalPolicy != CanonicalPolicy ||
		missing(resolution.ResolutionID, resolution.AgreementState, resolution.ResolvedAt, resolution.Rule) || !validOpaquePacketID(resolution.PacketID) ||
		!adjudicatableState(resolution.State) || len(resolution.PrimaryLabelDigests) != 2 {
		return errors.New("outcome resolution identity, state, labels, time, or rule is invalid")
	}
	if err := uniqueSortedDigests("outcome resolution primary labels", resolution.PrimaryLabelDigests); err != nil {
		return err
	}
	if resolution.AgreementState == "third_adjudicator_resolution" && !validDigest(resolution.TieBreakLabelDigest) ||
		resolution.AgreementState != "third_adjudicator_resolution" && resolution.TieBreakLabelDigest != "" {
		return errors.New("outcome resolution tie-break evidence does not match its agreement state")
	}
	if !slices.Contains([]string{"primary_agreement", "unresolved_disagreement", "third_adjudicator_resolution"}, resolution.AgreementState) {
		return errors.New("outcome resolution agreement state is invalid")
	}
	if _, err := time.Parse(time.RFC3339, resolution.ResolvedAt); err != nil {
		return errors.New("outcome resolution timestamp must be RFC3339")
	}
	expected, err := resolutionDigest(resolution)
	if err != nil || resolution.Digest != expected || resolution.ResolutionID != "resolution-"+expected {
		return errors.New("outcome resolution identity is invalid")
	}
	return nil
}

func BuildRevision(parent Record, evidence []Evidence, resolution State, basis, limitations []string, authorID, reason string) (Record, error) {
	if err := parent.Validate(); err != nil {
		return Record{}, err
	}
	evidence = append([]Evidence(nil), evidence...)
	basis = append([]string(nil), basis...)
	limitations = append([]string(nil), limitations...)
	sort.Slice(evidence, func(left, right int) bool { return evidence[left].ID < evidence[right].ID })
	sort.Strings(basis)
	sort.Strings(limitations)
	return SealRecord(Record{
		TaskAlias: parent.TaskAlias, Revision: parent.Revision + 1, ParentDigest: parent.Digest,
		Evidence: evidence, Resolution: resolution, ResolutionBasis: basis, Limitations: limitations,
		AuthorID: authorID, RevisionReason: reason,
	})
}

func SealPreservation(preservation Preservation) (Preservation, error) {
	preservation.SchemaVersion = PreservationSchemaVersion
	preservation.CanonicalPolicy = CanonicalPolicy
	preservation.Digest = ""
	digest, err := preservationDigest(preservation)
	if err != nil {
		return Preservation{}, err
	}
	preservation.Digest = digest
	return preservation, preservation.Validate()
}

func EvaluatePreservation(source, intervened Record, mechanism string) (Preservation, error) {
	if err := source.Validate(); err != nil {
		return Preservation{}, err
	}
	if err := intervened.Validate(); err != nil {
		return Preservation{}, err
	}
	preservation := Preservation{
		SourceOutcomeDigest: source.Digest, IntervenedOutcomeDigest: intervened.Digest,
		SourceState: source.Resolution, IntervenedState: intervened.Resolution,
		Mechanism: mechanism, EvaluatorBlind: true,
	}
	if source.TaskAlias != intervened.TaskAlias {
		preservation.InadmissibilityReasons = append(preservation.InadmissibilityReasons, "task_lineage_mismatch")
	}
	if source.Resolution != intervened.Resolution {
		preservation.InadmissibilityReasons = append(preservation.InadmissibilityReasons, "outcome_state_changed")
	}
	if !slices.Contains([]State{StateSolved, StateUnsolved}, source.Resolution) || !slices.Contains([]State{StateSolved, StateUnsolved}, intervened.Resolution) {
		preservation.InadmissibilityReasons = append(preservation.InadmissibilityReasons, "outcome_state_not_decisive")
	}
	sort.Strings(preservation.InadmissibilityReasons)
	preservation.Admissible = len(preservation.InadmissibilityReasons) == 0
	return SealPreservation(preservation)
}

func (preservation Preservation) Validate() error {
	if preservation.SchemaVersion != PreservationSchemaVersion || preservation.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(preservation.SourceOutcomeDigest) || !validDigest(preservation.IntervenedOutcomeDigest) ||
		!validState(preservation.SourceState) || !validState(preservation.IntervenedState) || missing(preservation.Mechanism) || !preservation.EvaluatorBlind {
		return errors.New("outcome preservation identity, state, mechanism, or blinding is invalid")
	}
	if err := uniqueSorted("outcome preservation inadmissibility reasons", preservation.InadmissibilityReasons); err != nil {
		return err
	}
	if preservation.Admissible != (len(preservation.InadmissibilityReasons) == 0) || preservation.Admissible && preservation.SourceState != preservation.IntervenedState {
		return errors.New("outcome preservation admissibility contradicts its states or reasons")
	}
	expected, err := preservationDigest(preservation)
	if err != nil || preservation.Digest != expected {
		return errors.New("outcome preservation digest is invalid")
	}
	return nil
}

func privateMappingDigest(value PrivateMapping) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
func resolutionDigest(value Resolution) (string, error) {
	value.ResolutionID, value.Digest = "", ""
	return digestJSON(value)
}
func preservationDigest(value Preservation) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
