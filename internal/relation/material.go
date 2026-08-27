package relation

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const (
	RelationEvidenceSelectorVersion = "evalwitness.relation-paired-evidence-selector.v1"
	RelationEvidenceBudgetTokens    = 16_000
	restrictedReferenceVisibility   = "restricted_reference_only"
)

var (
	relationSourceReferencePattern = regexp.MustCompile(`\bsource=([^\s|]+)`)
	relationCallReferencePattern   = regexp.MustCompile(`\bcall_id=([^\s\]]+)`)
)

func MaterializeCase(root string, plan Plan, release mutation.CorpusRelease, caseID string, evidenceBudgetTokens int) (CaseMaterial, error) {
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return CaseMaterial{}, err
	}
	if err := release.Validate(); err != nil {
		return CaseMaterial{}, err
	}
	if release.Digest != plan.SourceCorpusDigest || evidenceBudgetTokens != RelationEvidenceBudgetTokens {
		return CaseMaterial{}, errors.New("relation materialization requires the governed corpus and frozen 16000-token budget")
	}
	if plan.ProtocolVersion == ProtocolVersionV2 && (release.SpecDigest != plan.SourceCorpusSpecDigest ||
		release.MutationProgramDigest != plan.SourceMutationProgramDigest || release.Spec.DevelopmentAudit.ConstructAuditDigest != plan.SourceConstructAuditDigest) {
		return CaseMaterial{}, errors.New("v2 relation materialization corpus, mutation program, or construct audit binding disagrees with the plan")
	}
	replayed, err := ReplayCase(root, release, caseID)
	if err != nil {
		return CaseMaterial{}, err
	}
	if replayed.Receipt.ProtocolVersion != plan.ProtocolVersion {
		return CaseMaterial{}, errors.New("relation replay receipt protocol does not match the governed plan")
	}
	caseRecord, sources, err := materialCaseSources(release, caseID)
	if err != nil {
		return CaseMaterial{}, err
	}
	privateValues := relationPrivateValues(sources)
	requirement, err := commonTaskRequirement(replayed.Original)
	if err != nil {
		return CaseMaterial{}, err
	}
	requirement = redactRelationIdentities(requirement, privateValues)

	original, transformed, alignmentDigest, err := materializeAlignedEvidence(caseRecord, sources, replayed, evidenceBudgetTokens, privateValues)
	if err != nil {
		return CaseMaterial{}, err
	}
	material := CaseMaterial{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest, SourceCorpusDigest: release.Digest,
		CaseID: caseRecord.ID, Family: caseRecord.Family, Unit: replayed.Receipt.Unit, TaskRequirement: requirement,
		TaskRequirementDigest: digestText(requirement), Original: original, Transformed: transformed, AlignmentDigest: alignmentDigest,
		ReplayReceiptDigest: replayed.Receipt.Digest,
		Limitations: []string{
			"Deterministic paired slicing may omit judgment-relevant evidence; omitted event counts remain explicit.",
			"Materialization proves reproducibility, alignment, redaction, and accounting, not human semantic sufficiency.",
			"Restricted reference-only excerpts are not public-release artifacts.",
		},
		ExternalActionStatus: ExternalActionNotAuthorized,
	}
	if plan.ProtocolVersion == ProtocolVersionV2 {
		if caseRecord.ConstructFirewall == nil {
			return CaseMaterial{}, errors.New("v2 relation materialization requires the selected case construct firewall")
		}
		material.SourceCorpusSpecDigest = plan.SourceCorpusSpecDigest
		material.SourceMutationProgramDigest = plan.SourceMutationProgramDigest
		material.SourceConstructAuditDigest = plan.SourceConstructAuditDigest
		material.RelationContractVersion = mutation.RelationContractVersionV2
		material.EvidenceBoundaryVersion = mutation.EvidenceBoundaryVersionV2
		material.ConstructFirewallDigest = caseRecord.ConstructFirewall.Digest
	}
	return SealCaseMaterial(material)
}

func MaterializeCaseV3(root string, reviewPlan Plan, corpusPlan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3, caseID string, evidenceBudgetTokens int) (CaseMaterial, error) {
	if err := validateVersionedPlanConsumer(reviewPlan); err != nil {
		return CaseMaterial{}, err
	}
	if reviewPlan.ProtocolVersion != ProtocolVersionV3 || reviewPlan.SchemaVersion != PlanSchemaVersionV3Adapter {
		return CaseMaterial{}, errors.New("v3 relation materialization requires the governed v3 review-plan adapter")
	}
	if err := release.Validate(corpusPlan, audit); err != nil {
		return CaseMaterial{}, err
	}
	if release.Digest != reviewPlan.SourceCorpusDigest || release.PlanDigest != reviewPlan.SourceCorpusPlanDigest ||
		release.MutationProgramDigest != reviewPlan.SourceMutationProgramDigest || audit.Digest != reviewPlan.SourceConstructAuditDigest ||
		evidenceBudgetTokens != RelationEvidenceBudgetTokens {
		return CaseMaterial{}, errors.New("v3 relation materialization requires the exact governed corpus chain and frozen 16000-token budget")
	}
	replayed, err := ReplayCaseV3(root, corpusPlan, audit, release, caseID)
	if err != nil {
		return CaseMaterial{}, err
	}
	caseRecord, sources, err := materialCaseSourcesV3(release, caseID)
	if err != nil {
		return CaseMaterial{}, err
	}
	legacyCase := legacyCorpusCaseV3(caseRecord)
	privateValues := relationPrivateValues(sources)
	requirement, err := commonTaskRequirement(replayed.Original)
	if err != nil {
		return CaseMaterial{}, err
	}
	requirement = redactRelationIdentities(requirement, privateValues)
	original, transformed, alignmentDigest, err := materializeAlignedEvidence(legacyCase, sources, replayed, evidenceBudgetTokens, privateValues)
	if err != nil {
		return CaseMaterial{}, err
	}
	material := CaseMaterial{
		ProtocolVersion: ProtocolVersionV3, Objective: ReviewObjectiveControlledRelation, PlanDigest: reviewPlan.Digest,
		SourceCorpusDigest: release.Digest, SourceCorpusPlanDigest: corpusPlan.Digest,
		SourceMutationProgramDigest: release.MutationProgramDigest, SourceConstructAuditDigest: audit.Digest,
		RelationContractVersion: mutation.RelationContractVersionV3, EvidenceBoundaryVersion: mutation.EvidenceBoundaryVersionV3,
		ConstructFirewallDigest: caseRecord.ConstructFirewall.Digest, CaseID: caseRecord.ID, Family: caseRecord.Family,
		Unit: replayed.Receipt.Unit, TaskRequirement: requirement, TaskRequirementDigest: digestText(requirement),
		Original: original, Transformed: transformed, AlignmentDigest: alignmentDigest, ReplayReceiptDigest: replayed.Receipt.Digest,
		Limitations: []string{
			"Deterministic paired slicing may omit judgment-relevant evidence; omitted event counts remain explicit.",
			"Materialization proves reproducibility, alignment, redaction, and accounting, not human semantic sufficiency.",
			"Restricted reference-only excerpts are not public-release artifacts.",
		},
		ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealCaseMaterial(material)
}

func SealCaseMaterial(material CaseMaterial) (CaseMaterial, error) {
	schemaVersion, err := schemaVersionForProtocol(material.ProtocolVersion, CaseMaterialSchemaVersionV1, CaseMaterialSchemaVersionV2, CaseMaterialSchemaVersionV3)
	if err != nil {
		return CaseMaterial{}, err
	}
	material.SchemaVersion, material.CanonicalPolicy, material.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := caseMaterialDigest(material)
	if err != nil {
		return CaseMaterial{}, err
	}
	material.Digest = digest
	return material, material.Validate()
}

func (material CaseMaterial) Validate() error {
	definition, familyExists := mutation.DefinitionFor(material.Family)
	expectedArity := 1
	expectedUnit := UnitTrajectoryPair
	if familyExists && definition.PairLevel {
		expectedArity = 2
		expectedUnit = UnitCandidatePairOrders
	}
	if !validVersionedIdentity(material.SchemaVersion, material.ProtocolVersion, CaseMaterialSchemaVersionV1, CaseMaterialSchemaVersionV2, CaseMaterialSchemaVersionV3) || material.CanonicalPolicy != CanonicalPolicy ||
		material.Objective != ReviewObjectiveControlledRelation || !validDigest(material.PlanDigest) || !validDigest(material.SourceCorpusDigest) ||
		!familyExists || material.Unit != expectedUnit || strings.TrimSpace(material.CaseID) == "" || strings.TrimSpace(material.TaskRequirement) == "" || material.TaskRequirementDigest != digestText(material.TaskRequirement) ||
		len(material.Original) != expectedArity || len(material.Transformed) != expectedArity || !validDigest(material.AlignmentDigest) ||
		!validDigest(material.ReplayReceiptDigest) || len(material.Limitations) != 3 || !slices.IsSorted(material.Limitations) ||
		material.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation case material identity, unit, requirement, alignment, limitation, or authorization boundary is invalid")
	}
	if material.ProtocolVersion == ProtocolVersionV1 {
		if material.SourceCorpusSpecDigest != "" || material.SourceCorpusPlanDigest != "" || material.SourceMutationProgramDigest != "" || material.SourceConstructAuditDigest != "" ||
			material.RelationContractVersion != "" || material.EvidenceBoundaryVersion != "" || material.ConstructFirewallDigest != "" {
			return errors.New("v1 relation case material contains v2-only source or construct bindings")
		}
	} else if material.ProtocolVersion == ProtocolVersionV2 && (!validDigest(material.SourceCorpusSpecDigest) || material.SourceCorpusPlanDigest != "" || !validDigest(material.SourceMutationProgramDigest) || !validDigest(material.SourceConstructAuditDigest) ||
		material.RelationContractVersion != mutation.RelationContractVersionV2 || material.EvidenceBoundaryVersion != mutation.EvidenceBoundaryVersionV2 ||
		!validDigest(material.ConstructFirewallDigest)) {
		return errors.New("v2 relation case material lacks its corpus, program, contract, evidence-boundary, or construct-firewall binding")
	} else if material.ProtocolVersion == ProtocolVersionV3 && (material.SourceCorpusSpecDigest != "" || !validDigest(material.SourceCorpusPlanDigest) ||
		!validDigest(material.SourceMutationProgramDigest) || !validDigest(material.SourceConstructAuditDigest) || material.RelationContractVersion != mutation.RelationContractVersionV3 ||
		material.EvidenceBoundaryVersion != mutation.EvidenceBoundaryVersionV3 || !validDigest(material.ConstructFirewallDigest)) {
		return errors.New("v3 relation case material lacks its corpus plan, program, contract, evidence-boundary, or typed construct-firewall binding")
	}
	for _, excerpt := range append(append([]EvidenceExcerpt(nil), material.Original...), material.Transformed...) {
		if err := validateEvidenceExcerpt(excerpt); err != nil {
			return err
		}
	}
	if material.Unit == UnitCandidatePairOrders {
		if material.Original[0].ContentDigest != material.Transformed[1].ContentDigest || material.Original[1].ContentDigest != material.Transformed[0].ContentDigest {
			return errors.New("relation candidate-order material is not an exact excerpt reversal")
		}
	}
	expected, err := caseMaterialDigest(material)
	if err != nil || expected != material.Digest {
		return errors.New("relation case material digest is invalid")
	}
	return nil
}

func materializeAlignedEvidence(caseRecord mutation.CorpusCase, sources []mutation.CorpusSource, replayed ReplayedMaterial, budget int, privateValues []string) ([]EvidenceExcerpt, []EvidenceExcerpt, string, error) {
	if replayed.Receipt.Unit == UnitCandidatePairOrders {
		excerpts := make([]EvidenceExcerpt, len(replayed.Original))
		lineages := make([]string, len(replayed.Original))
		for index, trajectory := range replayed.Original {
			anchor, err := relationDecisionAnchor(trajectory)
			if err != nil {
				return nil, nil, "", err
			}
			excerpts[index], err = buildEvidenceExcerpt(trajectory, []string{anchor.ID}, sources[index], budget, privateValues)
			if err != nil {
				return nil, nil, "", err
			}
			lineages[index] = excerpts[index].RetainedLineageDigest
		}
		alignment, err := digestJSON(lineages)
		if err != nil {
			return nil, nil, "", err
		}
		return excerpts, []EvidenceExcerpt{excerpts[1], excerpts[0]}, alignment, nil
	}
	originalRequired := append([]string(nil), caseRecord.Manifest.Affected.EventIDs...)
	anchor, err := relationDecisionAnchor(replayed.Original[0])
	if err != nil {
		return nil, nil, "", err
	}
	originalRequired = append(originalRequired, anchor.ID)
	originalRequired = uniqueSorted(originalRequired)
	transformedRequired, err := alignedEventIDs(replayed.Original[0], replayed.Transformed[0], originalRequired)
	if err != nil {
		return nil, nil, "", err
	}
	originalRetained, transformedRetained, err := preprocess.ApplyPairedEvidenceBudgetWithRequiredEvents(
		replayed.Original[0], replayed.Transformed[0], budget, originalRequired, transformedRequired,
	)
	if err != nil {
		return nil, nil, "", err
	}
	original, err := buildEvidenceExcerptFromSelection(replayed.Original[0], originalRetained, originalRequired, sources[0], budget, privateValues)
	if err != nil {
		return nil, nil, "", err
	}
	transformed, err := buildEvidenceExcerptFromSelection(replayed.Transformed[0], transformedRetained, transformedRequired, sources[0], budget, privateValues)
	if err != nil {
		return nil, nil, "", err
	}
	if original.RetainedLineageDigest != transformed.RetainedLineageDigest {
		return nil, nil, "", errors.New("relation paired evidence selector retained different event lineages across the transformation")
	}
	return []EvidenceExcerpt{original}, []EvidenceExcerpt{transformed}, original.RetainedLineageDigest, nil
}

func buildEvidenceExcerpt(source preprocess.Trajectory, required []string, license mutation.CorpusSource, budget int, privateValues []string) (EvidenceExcerpt, error) {
	retained, err := preprocess.ApplyEvidenceBudgetWithRequiredEvents(source, budget, required)
	if err != nil {
		return EvidenceExcerpt{}, err
	}
	return buildEvidenceExcerptFromSelection(source, retained, required, license, budget, privateValues)
}

func buildEvidenceExcerptFromSelection(source, retained preprocess.Trajectory, required []string, license mutation.CorpusSource, budget int, privateValues []string) (EvidenceExcerpt, error) {
	content := redactRelationIdentities(preprocess.RenderTrajectory(retained), privateValues)
	content = pseudonymizeRelationRouteTokens(content)
	lineage, err := retainedLineageDigest(retained)
	if err != nil {
		return EvidenceExcerpt{}, err
	}
	return EvidenceExcerpt{
		SourceTrajectoryDigest: source.Digest, RetainedTrajectoryDigest: retained.Digest,
		SourceEvents: len(source.Events), RetainedEvents: len(retained.Events), OmittedEvents: len(source.Events) - len(retained.Events),
		RedactionHits: source.Report.RedactionHits, EvidenceBudgetTokens: budget, EvidenceSelector: RelationEvidenceSelectorVersion,
		RequiredEventIDs: append([]string(nil), required...), RetainedLineageDigest: lineage, Content: content, ContentDigest: digestText(content),
		LicenseSPDX: license.License.SPDX, SourceURL: license.License.SourceURL, SourceRevision: license.License.SourceRevision,
		Redistribution: license.License.Redistribution, Visibility: restrictedReferenceVisibility, PublicReleasable: false,
	}, nil
}

func validateEvidenceExcerpt(excerpt EvidenceExcerpt) error {
	if !validDigest(excerpt.SourceTrajectoryDigest) || !validDigest(excerpt.RetainedTrajectoryDigest) || excerpt.SourceEvents < 1 ||
		excerpt.RetainedEvents < 1 || excerpt.RetainedEvents > excerpt.SourceEvents || excerpt.OmittedEvents != excerpt.SourceEvents-excerpt.RetainedEvents ||
		excerpt.RedactionHits < 0 || excerpt.EvidenceBudgetTokens != RelationEvidenceBudgetTokens || excerpt.EvidenceSelector != RelationEvidenceSelectorVersion ||
		len(excerpt.RequiredEventIDs) < 1 || !slices.IsSorted(excerpt.RequiredEventIDs) || hasDuplicate(excerpt.RequiredEventIDs) || !validDigest(excerpt.RetainedLineageDigest) ||
		strings.TrimSpace(excerpt.Content) == "" || excerpt.ContentDigest != digestText(excerpt.Content) || strings.TrimSpace(excerpt.LicenseSPDX) == "" ||
		strings.TrimSpace(excerpt.SourceURL) == "" || strings.TrimSpace(excerpt.SourceRevision) == "" || excerpt.Redistribution != "reference_only" ||
		excerpt.Visibility != restrictedReferenceVisibility || excerpt.PublicReleasable {
		return errors.New("relation evidence excerpt identity, accounting, license, visibility, or content is invalid")
	}
	return nil
}

func materialCaseSources(release mutation.CorpusRelease, caseID string) (mutation.CorpusCase, []mutation.CorpusSource, error) {
	caseByID := make(map[string]mutation.CorpusCase, len(release.Cases))
	for _, item := range release.Cases {
		caseByID[item.ID] = item
	}
	item, exists := caseByID[caseID]
	if !exists {
		return mutation.CorpusCase{}, nil, fmt.Errorf("relation material case %q is absent", caseID)
	}
	sourceByID := make(map[string]mutation.CorpusSource, len(release.Sources))
	for _, source := range release.Sources {
		sourceByID[source.ID] = source
	}
	sources := make([]mutation.CorpusSource, 0, len(item.SourceIDs))
	for _, sourceID := range item.SourceIDs {
		source, exists := sourceByID[sourceID]
		if !exists {
			return mutation.CorpusCase{}, nil, fmt.Errorf("relation material source %q is absent", sourceID)
		}
		sources = append(sources, source)
	}
	return item, sources, nil
}

func materialCaseSourcesV3(release mutation.CorpusReleaseV3, caseID string) (mutation.CorpusCaseV3, []mutation.CorpusSource, error) {
	var item *mutation.CorpusCaseV3
	for index := range release.Cases {
		if release.Cases[index].ID == caseID {
			item = &release.Cases[index]
			break
		}
	}
	if item == nil {
		return mutation.CorpusCaseV3{}, nil, fmt.Errorf("v3 relation material case %q is absent", caseID)
	}
	sourceByID := make(map[string]mutation.CorpusSource, len(release.Sources))
	for _, source := range release.Sources {
		sourceByID[source.ID] = source
	}
	sources := make([]mutation.CorpusSource, 0, len(item.SourceIDs))
	for _, sourceID := range item.SourceIDs {
		source, exists := sourceByID[sourceID]
		if !exists {
			return mutation.CorpusCaseV3{}, nil, fmt.Errorf("v3 relation material source %q is absent", sourceID)
		}
		sources = append(sources, source)
	}
	return *item, sources, nil
}

func legacyCorpusCaseV3(item mutation.CorpusCaseV3) mutation.CorpusCase {
	return mutation.CorpusCase{
		ID: item.ID, SourceIDs: append([]string(nil), item.SourceIDs...), Family: item.Family, Split: item.Split, Control: item.Control,
		Manifest: item.Manifest, BlindPacket: item.BlindPacket, Reduction: item.Reduction, RegenerationKey: item.RegenerationKey,
	}
}

func commonTaskRequirement(trajectories []preprocess.Trajectory) (string, error) {
	var requirement string
	for _, trajectory := range trajectories {
		current, err := taskRequirement(trajectory)
		if err != nil {
			return "", err
		}
		if requirement == "" {
			requirement = current
			continue
		}
		if strings.Join(strings.Fields(requirement), " ") != strings.Join(strings.Fields(current), " ") {
			return "", errors.New("relation pair sources do not share one coherent task requirement")
		}
	}
	return requirement, nil
}

func taskRequirement(trajectory preprocess.Trajectory) (string, error) {
	for _, event := range trajectory.Events {
		if event.Message == nil || !strings.EqualFold(event.Message.Role, "user") {
			continue
		}
		var parts []string
		for _, part := range event.Message.Parts {
			if part.Kind == preprocess.ContentText && strings.TrimSpace(part.Text) != "" {
				parts = append(parts, strings.TrimSpace(part.Text))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n"), nil
		}
	}
	return "", errors.New("relation trajectory has no nonempty user task requirement")
}

func relationDecisionAnchor(trajectory preprocess.Trajectory) (preprocess.Event, error) {
	for index := len(trajectory.Events) - 1; index >= 0; index-- {
		if trajectory.Events[index].FileChange != nil && strings.TrimSpace(trajectory.Events[index].FileChange.Diff) != "" {
			return trajectory.Events[index], nil
		}
	}
	for index := len(trajectory.Events) - 1; index >= 0; index-- {
		event := trajectory.Events[index]
		if event.Message == nil || !strings.EqualFold(event.Message.Role, "assistant") {
			continue
		}
		for _, part := range event.Message.Parts {
			if part.Kind == preprocess.ContentText && strings.TrimSpace(part.Text) != "" {
				return event, nil
			}
		}
	}
	return preprocess.Event{}, errors.New("relation trajectory has no final patch or assistant narrative anchor")
}

func alignedEventIDs(original, transformed preprocess.Trajectory, required []string) ([]string, error) {
	originalByID := make(map[string]preprocess.Event, len(original.Events))
	transformedByID := make(map[string]preprocess.Event, len(transformed.Events))
	transformedByLineage := make(map[string]string, len(transformed.Events))
	for _, event := range original.Events {
		originalByID[event.ID] = event
	}
	for _, event := range transformed.Events {
		transformedByID[event.ID] = event
		transformedByLineage[eventLineageKey(event)] = event.ID
	}
	result := make([]string, 0, len(required))
	for _, eventID := range required {
		if _, exists := transformedByID[eventID]; exists {
			result = append(result, eventID)
			continue
		}
		event, exists := originalByID[eventID]
		if !exists {
			return nil, fmt.Errorf("relation original required event %q is absent", eventID)
		}
		mapped, exists := transformedByLineage[eventLineageKey(event)]
		if !exists {
			return nil, fmt.Errorf("relation transformed counterpart for event %q is absent", eventID)
		}
		result = append(result, mapped)
	}
	return uniqueSorted(result), nil
}

func retainedLineageDigest(trajectory preprocess.Trajectory) (string, error) {
	keys := make([]string, len(trajectory.Events))
	for index, event := range trajectory.Events {
		keys[index] = eventLineageKey(event)
	}
	sort.Strings(keys)
	return digestJSON(keys)
}

func eventLineageKey(event preprocess.Event) string {
	value, _ := digestJSON(struct {
		Kind          preprocess.EventKind      `json:"kind"`
		Source        preprocess.SourceLocation `json:"source"`
		SourceEventID string                    `json:"source_event_id"`
	}{event.Kind, event.Source, event.SourceEventID})
	return value
}

func relationPrivateValues(sources []mutation.CorpusSource) []string {
	var result []string
	for _, source := range sources {
		result = append(result, source.ID, source.TaskID, source.RepositoryID, source.SourceFamily, source.SourceLocation, source.SourceRevision)
	}
	return uniqueSorted(result)
}

func redactRelationIdentities(value string, privateValues []string) string {
	for _, private := range privateValues {
		for {
			index := strings.Index(strings.ToLower(value), strings.ToLower(private))
			if index < 0 {
				break
			}
			value = value[:index] + "[SOURCE_ID_REDACTED]" + value[index+len(private):]
		}
	}
	return value
}

func pseudonymizeRelationRouteTokens(value string) string {
	value = relationSourceReferencePattern.ReplaceAllStringFunc(value, func(match string) string {
		return "source=source-ref-" + digestText(strings.TrimPrefix(match, "source="))[:16]
	})
	return relationCallReferencePattern.ReplaceAllStringFunc(value, func(match string) string {
		return "call_id=call-ref-" + digestText(strings.TrimPrefix(match, "call_id="))[:16]
	})
}

func caseMaterialDigest(material CaseMaterial) (string, error) {
	material.Digest = ""
	return digestJSON(material)
}
