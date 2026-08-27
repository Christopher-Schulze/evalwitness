package capsule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	referenceTask049DesignType       = "evalwitness.task-049-controlled-corruption-design.v1"
	referenceProvenanceBindingID     = "evalwitness.validator.reference-provenance-bindings.v1"
	referenceProvenanceRendererID    = "evalwitness.renderer.relation-scarcity-public-brief.v1"
	referenceProvenanceArtifactKind  = "reference-capsule-builder"
	referenceProvenancePayloadPrefix = "evalwitness.validator.reference-provenance."
	referenceSyntheticFixtureLicense = "MIT"
)

func referenceProvenanceTypes() []ComponentType {
	public := []Visibility{VisibilityPublic}
	canonical := func(typeID string, role Role, rules ...ParentRule) ComponentType {
		return ComponentType{
			TypeID: typeID, SchemaID: typeID, Role: role, AllowedVisibilities: public,
			MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON,
			ValidatorID: referenceProvenanceValidatorID(typeID), BindingValidatorID: referenceProvenanceBindingID,
			ParentRules: rules,
		}
	}
	return []ComponentType{
		{
			TypeID: referenceTask049DesignType, SchemaID: referenceTask049DesignType, Role: RoleGovernance,
			AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: PayloadExactBytes,
			ValidatorID: referenceProvenanceValidatorID(referenceTask049DesignType),
		},
		canonical(SourceTreeProvenanceSchemaVersion, RoleObservation),
		canonical(BuildProvenanceSchemaVersion, RoleAttestation,
			parentRule(EdgeAttests, SourceTreeProvenanceSchemaVersion, 1, 1)),
		canonical(DatasetProvenanceSchemaVersion, RoleAttestation,
			parentRule(EdgeAttests, referenceTraceSourceType, 1, 1),
			parentRule(EdgeAttests, preprocess.CanonicalTrajectorySchema, 1, 1),
			parentRule(EdgeAttests, lineage.DatasetCardSchemaVersion, 1, 1),
			parentRule(EdgeGovernedBy, lineage.SourceSpecificationRegistryVersion, 1, 1)),
		canonical(RouteProvenanceSchemaVersion, RoleDerivation,
			parentRule(EdgeDerivedFrom, referenceRequestCorpusType, 1, 1),
			parentRule(EdgeDerivedFrom, protocolkit.RunSchema, 1, 1)),
		canonical(StudyProvenanceSchemaVersion, RoleGovernance,
			parentRule(EdgeDerivedFrom, referenceTask049DesignType, 1, 1),
			parentRule(EdgeDerivedFrom, protocolkit.VectorCorpusSchema, 1, 1),
			parentRule(EdgeDerivedFrom, relation.PlanSchemaVersionV3, 1, 1),
			parentRule(EdgeDerivedFrom, lineage.PlanSchemaVersion, 1, 1)),
		canonical(AnalysisProvenanceSchemaVersion, RoleAttestation,
			parentRule(EdgeAttests, protocolkit.RunSchema, 1, 1),
			parentRule(EdgeAttests, relation.ScarcityPublicEvidenceSchemaVersion, 1, 1),
			parentRule(EdgeAttests, lineage.ReleaseSchemaVersion, 1, 1)),
		canonical(ClockProvenanceSchemaVersion, RoleDerivation,
			parentRule(EdgeDerivedFrom, AnalysisProvenanceSchemaVersion, 1, 1),
			parentRule(EdgeDerivedFrom, protocolkit.RunSchema, 1, 1),
			parentRule(EdgeDerivedFrom, preprocess.CanonicalTrajectorySchema, 1, 1)),
		canonical(RenderReceiptSchemaVersion, RolePresentation,
			parentRule(EdgeRenders, relation.ScarcityPublicEvidenceSchemaVersion, 1, 1)),
	}
}

func referenceProvenancePayloadValidators() map[string]PayloadValidator {
	validators := map[string]PayloadValidator{
		referenceProvenanceValidatorID(referenceTask049DesignType): func(payload []byte) error {
			var design mutation.RelationDesign
			if err := decodeReferenceJSON(payload, &design); err != nil {
				return err
			}
			return design.Validate(40, 8)
		},
	}
	validators[referenceProvenanceValidatorID(SourceTreeProvenanceSchemaVersion)] = provenanceValidator(func(value SourceTreeProvenance) error { return value.Validate() })
	validators[referenceProvenanceValidatorID(BuildProvenanceSchemaVersion)] = provenanceValidator(func(value BuildProvenance) error { return value.Validate() })
	validators[referenceProvenanceValidatorID(DatasetProvenanceSchemaVersion)] = provenanceValidator(func(value DatasetProvenance) error { return value.Validate() })
	validators[referenceProvenanceValidatorID(RouteProvenanceSchemaVersion)] = provenanceValidator(func(value RouteProvenance) error { return value.Validate() })
	validators[referenceProvenanceValidatorID(StudyProvenanceSchemaVersion)] = provenanceValidator(func(value StudyProvenance) error { return value.Validate() })
	validators[referenceProvenanceValidatorID(AnalysisProvenanceSchemaVersion)] = provenanceValidator(func(value AnalysisProvenance) error { return value.Validate() })
	validators[referenceProvenanceValidatorID(ClockProvenanceSchemaVersion)] = provenanceValidator(func(value ClockProvenance) error { return value.Validate() })
	validators[referenceProvenanceValidatorID(RenderReceiptSchemaVersion)] = provenanceValidator(func(value RenderReceipt) error { return value.Validate() })
	return validators
}

func provenanceValidator[T any](validate func(T) error) PayloadValidator {
	return func(payload []byte) error {
		var value T
		if err := decodeReferenceJSON(payload, &value); err != nil {
			return err
		}
		return validate(value)
	}
}

func referenceProvenanceBindingValidators() map[string]BindingValidator {
	return map[string]BindingValidator{referenceProvenanceBindingID: validateReferenceProvenanceBindings}
}

func addReferenceProvenanceEvidence(ctx context.Context, repositoryRoot string, registry *Registry, records *[]ComponentRecord, payloads map[string][]byte) ([]ComponentRecord, ComponentRecord, error) {
	if ctx == nil {
		return nil, ComponentRecord{}, errors.New("reference provenance requires a context")
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, ComponentRecord{}, fmt.Errorf("resolve reference builder executable: %w", err)
	}
	sourceValue, buildValue, err := CollectBuildProvenance(ctx, repositoryRoot, executable, referenceProvenanceArtifactKind)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	source, err := addCanonicalReferenceValue(registry, "provenance.source-tree", SourceTreeProvenanceSchemaVersion, sourceValue, nil, records, payloads)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	build, err := addCanonicalReferenceValue(registry, "provenance.build", BuildProvenanceSchemaVersion, buildValue,
		[]ParentRef{internalParentRef(EdgeAttests, source)}, records, payloads)
	if err != nil {
		return nil, ComponentRecord{}, err
	}

	traceSource, err := componentByType(*records, referenceTraceSourceType)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	trajectory, err := componentByType(*records, preprocess.CanonicalTrajectorySchema)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	datasetCard, err := componentByType(*records, lineage.DatasetCardSchemaVersion)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	sourceSpecifications, err := componentByType(*records, lineage.SourceSpecificationRegistryVersion)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	datasetValue, err := expectedReferenceDataset(
		boundReferenceRecord(traceSource, payloads), boundReferenceRecord(trajectory, payloads),
		boundReferenceRecord(datasetCard, payloads), boundReferenceRecord(sourceSpecifications, payloads),
	)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	dataset, err := addCanonicalReferenceValue(registry, "provenance.dataset", DatasetProvenanceSchemaVersion, datasetValue, []ParentRef{
		internalParentRef(EdgeAttests, traceSource), internalParentRef(EdgeAttests, trajectory),
		internalParentRef(EdgeAttests, datasetCard), internalParentRef(EdgeGovernedBy, sourceSpecifications),
	}, records, payloads)
	if err != nil {
		return nil, ComponentRecord{}, err
	}

	requestCorpus, err := componentByType(*records, referenceRequestCorpusType)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	protocolRun, err := componentByType(*records, protocolkit.RunSchema)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	routeValue, err := expectedReferenceRoute(boundReferenceRecord(requestCorpus, payloads), boundReferenceRecord(protocolRun, payloads))
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	route, err := addCanonicalReferenceValue(registry, "provenance.route", RouteProvenanceSchemaVersion, routeValue, []ParentRef{
		internalParentRef(EdgeDerivedFrom, requestCorpus), internalParentRef(EdgeDerivedFrom, protocolRun),
	}, records, payloads)
	if err != nil {
		return nil, ComponentRecord{}, err
	}

	task049, err := addReferenceFile(repositoryRoot, registry, "provenance.task-049-design", referenceTask049DesignType,
		VisibilityPublic, "eval/governance/controlled-corruption-design.json", nil, records, payloads)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	protocolVectors, err := componentByType(*records, protocolkit.VectorCorpusSchema)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	relationPlan, err := componentByType(*records, relation.PlanSchemaVersionV3)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	lineagePlan, err := componentByType(*records, lineage.PlanSchemaVersion)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	studyValue, err := expectedReferenceStudy(task049, protocolVectors, relationPlan, lineagePlan)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	study, err := addCanonicalReferenceValue(registry, "provenance.study", StudyProvenanceSchemaVersion, studyValue, []ParentRef{
		internalParentRef(EdgeDerivedFrom, task049), internalParentRef(EdgeDerivedFrom, protocolVectors),
		internalParentRef(EdgeDerivedFrom, relationPlan), internalParentRef(EdgeDerivedFrom, lineagePlan),
	}, records, payloads)
	if err != nil {
		return nil, ComponentRecord{}, err
	}

	scarcity, err := componentByType(*records, relation.ScarcityPublicEvidenceSchemaVersion)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	lineageRelease, err := componentByType(*records, lineage.ReleaseSchemaVersion)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	analysisValue, err := expectedReferenceAnalysis(protocolRun, scarcity, lineageRelease)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	analysis, err := addCanonicalReferenceValue(registry, "provenance.analysis", AnalysisProvenanceSchemaVersion, analysisValue, []ParentRef{
		internalParentRef(EdgeAttests, protocolRun), internalParentRef(EdgeAttests, scarcity), internalParentRef(EdgeAttests, lineageRelease),
	}, records, payloads)
	if err != nil {
		return nil, ComponentRecord{}, err
	}

	clockValue, err := expectedReferenceClocks(analysis, protocolRun, trajectory)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	clocks, err := addCanonicalReferenceValue(registry, "provenance.clocks", ClockProvenanceSchemaVersion, clockValue, []ParentRef{
		internalParentRef(EdgeDerivedFrom, analysis), internalParentRef(EdgeDerivedFrom, protocolRun), internalParentRef(EdgeDerivedFrom, trajectory),
	}, records, payloads)
	if err != nil {
		return nil, ComponentRecord{}, err
	}

	renderValue, err := expectedReferenceRenderReceipt(boundReferenceRecord(scarcity, payloads))
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	renderReceipt, err := addCanonicalReferenceValue(registry, "provenance.render-receipt", RenderReceiptSchemaVersion, renderValue,
		[]ParentRef{internalParentRef(EdgeRenders, scarcity)}, records, payloads)
	if err != nil {
		return nil, ComponentRecord{}, err
	}
	return []ComponentRecord{build, dataset, route, study, analysis, clocks}, renderReceipt, nil
}

func validateReferenceProvenanceBindings(context BindingContext) error {
	switch context.Component.TypeID {
	case SourceTreeProvenanceSchemaVersion:
		return nil
	case BuildProvenanceSchemaVersion:
		return validateReferenceBuildBinding(context)
	case DatasetProvenanceSchemaVersion:
		expected, err := expectedReferenceDatasetByContext(context)
		return compareReferenceProvenance(context.Payload, expected, err)
	case RouteProvenanceSchemaVersion:
		expected, err := expectedReferenceRouteByContext(context)
		return compareReferenceProvenance(context.Payload, expected, err)
	case StudyProvenanceSchemaVersion:
		expected, err := expectedReferenceStudyByContext(context)
		return compareReferenceProvenance(context.Payload, expected, err)
	case AnalysisProvenanceSchemaVersion:
		expected, err := expectedReferenceAnalysisByContext(context)
		return compareReferenceProvenance(context.Payload, expected, err)
	case ClockProvenanceSchemaVersion:
		expected, err := expectedReferenceClocksByContext(context)
		return compareReferenceProvenance(context.Payload, expected, err)
	case RenderReceiptSchemaVersion:
		expected, err := expectedReferenceRenderReceiptByContext(context)
		return compareReferenceProvenance(context.Payload, expected, err)
	default:
		return fmt.Errorf("unsupported reference provenance binding type %q", context.Component.TypeID)
	}
}

func validateReferenceBuildBinding(context BindingContext) error {
	parent, err := uniqueReferenceParent(context, SourceTreeProvenanceSchemaVersion)
	if err != nil {
		return err
	}
	var source SourceTreeProvenance
	var build BuildProvenance
	if err := decodeReferenceJSON(parent.Payload, &source); err != nil {
		return err
	}
	if err := decodeReferenceJSON(context.Payload, &build); err != nil {
		return err
	}
	if build.SourceTreeDigest != source.Digest || build.Commit != source.Commit || build.Dirty != source.Dirty {
		return errors.New("build provenance differs from its exact source-tree parent")
	}
	return nil
}

func expectedReferenceDataset(source, trajectoryParent, cardParent, specificationParent BoundParent) (DatasetProvenance, error) {
	var trajectory preprocess.Trajectory
	var card lineage.VerificationLineageDatasetCard
	var specifications lineage.TraceSourceSpecificationRegistry
	if err := decodeReferenceJSON(trajectoryParent.Payload, &trajectory); err != nil {
		return DatasetProvenance{}, err
	}
	if err := decodeReferenceJSON(cardParent.Payload, &card); err != nil {
		return DatasetProvenance{}, err
	}
	if err := decodeReferenceJSON(specificationParent.Payload, &specifications); err != nil {
		return DatasetProvenance{}, err
	}
	if err := trajectory.Validate(); err != nil {
		return DatasetProvenance{}, err
	}
	if err := card.Validate(); err != nil {
		return DatasetProvenance{}, err
	}
	if err := specifications.Validate(); err != nil {
		return DatasetProvenance{}, err
	}
	license := ""
	for _, specification := range specifications.Specifications {
		if specification.ID == "codex_rollout_jsonl" {
			license = specification.License
			break
		}
	}
	if license == "" {
		return DatasetProvenance{}, errors.New("reference dataset source or license is not bound to the trace fixture")
	}
	taskIDs := []string{"fixture.codex-rollout"}
	taskDigest, err := protocolkit.Digest(taskIDs)
	if err != nil {
		return DatasetProvenance{}, err
	}
	value := DatasetProvenance{
		SchemaVersion: DatasetProvenanceSchemaVersion, DatasetID: "task-050-reference-trace",
		Source: "synthetic_codex_rollout_golden_fixture", License: referenceSyntheticFixtureLicense,
		SourceSpecificationLicense: license, AcquisitionStatus: AvailabilityFixtureOnly,
		AcquisitionChecksum: source.Record.Payload.Digest, CanonicalSourceDigest: trajectory.SourceDigest,
		TaskIDs: taskIDs, TaskIDsDigest: taskDigest,
		Split: "development", TrajectoryDigests: []string{trajectory.Digest}, LabelStatus: AvailabilityNotRun,
		LabelDigests: []string{}, Exclusions: slices.Clone(card.ExclusionCriteria),
		SourceComponentID: source.Record.ComponentID, TrajectoryComponentID: trajectoryParent.Record.ComponentID,
		DatasetCardComponentID: cardParent.Record.ComponentID,
	}
	value.Digest, err = datasetProvenanceDigest(value)
	if err != nil {
		return DatasetProvenance{}, err
	}
	return value, value.Validate()
}

type referenceRequestCorpus struct {
	CorpusSchema         string `json:"corpus_schema"`
	RequestSchemaVersion int    `json:"request_schema_version"`
	Digest               string `json:"digest"`
	NumericContract      string `json:"numeric_contract"`
	Cases                []struct {
		Name             string          `json:"name"`
		Request          json.RawMessage `json:"request"`
		CanonicalUTF8Hex string          `json:"canonical_utf8_hex"`
		Fingerprint      string          `json:"fingerprint_sha256"`
	} `json:"cases"`
}

func expectedReferenceRoute(requestParent, runParent BoundParent) (RouteProvenance, error) {
	var corpus referenceRequestCorpus
	if err := decodeReferenceJSON(requestParent.Payload, &corpus); err != nil {
		return RouteProvenance{}, err
	}
	if corpus.CorpusSchema != referenceRequestCorpusType || corpus.RequestSchemaVersion != provider.RequestSchemaVersion ||
		corpus.Digest != DigestAlgorithm || len(corpus.Cases) == 0 {
		return RouteProvenance{}, errors.New("reference route request corpus identity is invalid")
	}
	var run protocolkit.AuditRun
	if err := protocolkit.DecodeStrict(runParent.Payload, &run); err != nil {
		return RouteProvenance{}, err
	}
	requests := make([]RouteRequestProvenance, 0, len(corpus.Cases))
	for _, item := range corpus.Cases {
		var request provider.RequestEnvelope
		if err := json.Unmarshal(item.Request, &request); err != nil {
			return RouteProvenance{}, err
		}
		fingerprint, err := request.Fingerprint()
		if err != nil || string(fingerprint) != item.Fingerprint {
			return RouteProvenance{}, errors.New("reference route request fingerprint differs from the frozen corpus")
		}
		requests = append(requests, RouteRequestProvenance{
			CaseID: "request." + item.Name, Fingerprint: item.Fingerprint, ProviderID: request.ProviderID,
			RouteID: request.RouteID(), RequestedModel: request.RequestedModel,
		})
	}
	slices.SortFunc(requests, func(left, right RouteRequestProvenance) int { return strings.Compare(left.CaseID, right.CaseID) })
	value := RouteProvenance{
		SchemaVersion: RouteProvenanceSchemaVersion, Requests: requests, RequestStatus: AvailabilityFixtureOnly,
		ResponseStatus: AvailabilityUnavailablePublicSource, AttestationStatus: AvailabilityUnavailablePublicSource,
		ProviderCallsStatus: AvailabilityNotRun, ExternalActionStatus: "not_authorized",
		ServedIdentities: []string{}, CheckpointAssertions: []string{},
		ObservedLimitations: []string{
			"capability-attestation-bytes-unavailable", "checkpoint-assertion-unavailable", "no-provider-call-authorized",
			"request-corpus-is-conformance-fixture", "response-record-bytes-unavailable",
		},
		RequestComponentID: requestParent.Record.ComponentID, ProtocolRunID: runParent.Record.ComponentID,
	}
	var err error
	value.Digest, err = routeProvenanceDigest(value)
	if err != nil {
		return RouteProvenance{}, err
	}
	return value, value.Validate()
}

func expectedReferenceStudy(task049, vectors, relationPlan, lineagePlan ComponentRecord) (StudyProvenance, error) {
	plans := []StudyPlanBinding{
		{TaskID: "TASK-049", Name: "controlled-corruption-design", ComponentID: task049.ComponentID},
		{TaskID: "TASK-053", Name: "protocol-normative-corpus", ComponentID: vectors.ComponentID},
		{TaskID: "TASK-068", Name: "relation-audit-plan-v3", ComponentID: relationPlan.ComponentID},
		{TaskID: "TASK-069", Name: "verification-lineage-plan-v1", ComponentID: lineagePlan.ComponentID},
	}
	value := StudyProvenance{
		SchemaVersion: StudyProvenanceSchemaVersion, StudyID: "task-050-reference",
		EvidenceRole: "development_reference", ProviderStudy: AvailabilityNotRun, HumanStudy: AvailabilityNotRun,
		ExternalAction: "not_authorized", ConfirmationStatus: "not_admitted",
		ClaimCeiling: "offline method and artifact integrity only; no provider, human-validity, prevalence, transfer, or performance claim",
		Plans:        plans,
	}
	var err error
	value.Digest, err = studyProvenanceDigest(value)
	if err != nil {
		return StudyProvenance{}, err
	}
	return value, value.Validate()
}

func expectedReferenceAnalysis(protocolRun, scarcity, release ComponentRecord) (AnalysisProvenance, error) {
	analyses := []AnalysisBinding{
		{
			AnalysisID: "protocol-reference-conformance", Version: protocolkit.CurrentVersion,
			Command:         []string{"scripts/audits/run-protocol-conformance.sh", "--format", "json"},
			InputComponents: []string{protocolRun.ComponentID}, OutputComponents: []string{protocolRun.ComponentID}, Status: "completed",
		},
		{
			AnalysisID: "relation-scarcity-analysis", Version: relation.ScarcityPublicEvidenceSchemaVersion,
			Command: []string{"scripts/tests/run-claimcheck.sh"}, InputComponents: []string{scarcity.ComponentID},
			OutputComponents: []string{scarcity.ComponentID}, Status: "completed",
		},
		{
			AnalysisID: "verification-lineage-development", Version: lineage.ReleaseSchemaVersion,
			Command: []string{"scripts/tests/run-claimcheck.sh"}, InputComponents: []string{release.ComponentID},
			OutputComponents: []string{release.ComponentID}, Status: "completed",
		},
	}
	value := AnalysisProvenance{
		SchemaVersion: AnalysisProvenanceSchemaVersion, ProviderCallsRequired: 0, Analyses: analyses,
		Exclusions: []string{"human-study-not-run", "no-live-provider-observation", "private-owner-chain-omitted", "restricted-v2-corpus-parents-omitted"},
	}
	var err error
	value.Digest, err = analysisProvenanceDigest(value)
	if err != nil {
		return AnalysisProvenance{}, err
	}
	return value, value.Validate()
}

func expectedReferenceClocks(analysis, protocolRun, trajectory ComponentRecord) (ClockProvenance, error) {
	replaySources := []string{protocolRun.ComponentID, trajectory.ComponentID}
	slices.Sort(replaySources)
	value := ClockProvenance{
		SchemaVersion: ClockProvenanceSchemaVersion,
		Events: []ClockEvent{
			{Kind: "analysis", Status: "completed_time_unavailable", SourceComponentIDs: []string{analysis.ComponentID}, Boundary: "offline derived analyses completed; source artifacts carry identity but no trustworthy wall-clock field"},
			{Kind: "collection", Status: "not_run", SourceComponentIDs: []string{}, Boundary: "no provider or human collection was authorized for the TASK-050 reference capsule"},
			{Kind: "replay", Status: "completed_time_unavailable", SourceComponentIDs: replaySources, Boundary: "offline protocol and trace replay completed without minting a provider observation"},
		},
	}
	var err error
	value.Digest, err = clockProvenanceDigest(value)
	if err != nil {
		return ClockProvenance{}, err
	}
	return value, value.Validate()
}

func expectedReferenceRenderReceipt(scarcity BoundParent) (RenderReceipt, error) {
	evidence, err := relation.DecodeScarcityPublicEvidence(strings.NewReader(string(scarcity.Payload)))
	if err != nil {
		return RenderReceipt{}, err
	}
	rendered, err := relation.RenderScarcityPublicBriefMarkdown(evidence)
	if err != nil {
		return RenderReceipt{}, err
	}
	value := RenderReceipt{
		SchemaVersion: RenderReceiptSchemaVersion, RendererID: referenceProvenanceRendererID,
		SourceComponentID: scarcity.Record.ComponentID, OutputDigest: protocolkit.DigestBytes([]byte(rendered)),
		TimeStatus: "time_unavailable",
	}
	value.Digest, err = renderReceiptDigest(value)
	if err != nil {
		return RenderReceipt{}, err
	}
	return value, value.Validate()
}

func expectedReferenceDatasetByContext(context BindingContext) (DatasetProvenance, error) {
	source, err := uniqueReferenceParent(context, referenceTraceSourceType)
	if err != nil {
		return DatasetProvenance{}, err
	}
	trajectory, err := uniqueReferenceParent(context, preprocess.CanonicalTrajectorySchema)
	if err != nil {
		return DatasetProvenance{}, err
	}
	card, err := uniqueReferenceParent(context, lineage.DatasetCardSchemaVersion)
	if err != nil {
		return DatasetProvenance{}, err
	}
	specifications, err := uniqueReferenceParent(context, lineage.SourceSpecificationRegistryVersion)
	if err != nil {
		return DatasetProvenance{}, err
	}
	return expectedReferenceDataset(source, trajectory, card, specifications)
}

func expectedReferenceRouteByContext(context BindingContext) (RouteProvenance, error) {
	requests, err := uniqueReferenceParent(context, referenceRequestCorpusType)
	if err != nil {
		return RouteProvenance{}, err
	}
	run, err := uniqueReferenceParent(context, protocolkit.RunSchema)
	if err != nil {
		return RouteProvenance{}, err
	}
	return expectedReferenceRoute(requests, run)
}

func expectedReferenceStudyByContext(context BindingContext) (StudyProvenance, error) {
	task049, err := uniqueReferenceParent(context, referenceTask049DesignType)
	if err != nil {
		return StudyProvenance{}, err
	}
	vectors, err := uniqueReferenceParent(context, protocolkit.VectorCorpusSchema)
	if err != nil {
		return StudyProvenance{}, err
	}
	relationPlan, err := uniqueReferenceParent(context, relation.PlanSchemaVersionV3)
	if err != nil {
		return StudyProvenance{}, err
	}
	lineagePlan, err := uniqueReferenceParent(context, lineage.PlanSchemaVersion)
	if err != nil {
		return StudyProvenance{}, err
	}
	return expectedReferenceStudy(task049.Record, vectors.Record, relationPlan.Record, lineagePlan.Record)
}

func expectedReferenceAnalysisByContext(context BindingContext) (AnalysisProvenance, error) {
	run, err := uniqueReferenceParent(context, protocolkit.RunSchema)
	if err != nil {
		return AnalysisProvenance{}, err
	}
	scarcity, err := uniqueReferenceParent(context, relation.ScarcityPublicEvidenceSchemaVersion)
	if err != nil {
		return AnalysisProvenance{}, err
	}
	release, err := uniqueReferenceParent(context, lineage.ReleaseSchemaVersion)
	if err != nil {
		return AnalysisProvenance{}, err
	}
	return expectedReferenceAnalysis(run.Record, scarcity.Record, release.Record)
}

func expectedReferenceClocksByContext(context BindingContext) (ClockProvenance, error) {
	analysis, err := uniqueReferenceParent(context, AnalysisProvenanceSchemaVersion)
	if err != nil {
		return ClockProvenance{}, err
	}
	run, err := uniqueReferenceParent(context, protocolkit.RunSchema)
	if err != nil {
		return ClockProvenance{}, err
	}
	trajectory, err := uniqueReferenceParent(context, preprocess.CanonicalTrajectorySchema)
	if err != nil {
		return ClockProvenance{}, err
	}
	return expectedReferenceClocks(analysis.Record, run.Record, trajectory.Record)
}

func expectedReferenceRenderReceiptByContext(context BindingContext) (RenderReceipt, error) {
	scarcity, err := uniqueReferenceParent(context, relation.ScarcityPublicEvidenceSchemaVersion)
	if err != nil {
		return RenderReceipt{}, err
	}
	return expectedReferenceRenderReceipt(scarcity)
}

func compareReferenceProvenance[T any](payload []byte, expected T, expectedErr error) error {
	if expectedErr != nil {
		return expectedErr
	}
	var actual T
	if err := decodeReferenceJSON(payload, &actual); err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return errors.New("reference provenance differs from its exact capsule parents")
	}
	return nil
}

func addCanonicalReferenceValue(registry *Registry, name, typeID string, value any, parents []ParentRef, records *[]ComponentRecord, payloads map[string][]byte) (ComponentRecord, error) {
	raw, err := protocolkit.CanonicalMarshal(value)
	if err != nil {
		return ComponentRecord{}, err
	}
	return addReferencePayload(registry, ComponentInput{
		Name: name, TypeID: typeID, Visibility: VisibilityPublic, Payload: raw, Parents: parents,
	}, records, payloads)
}

func boundReferenceRecord(record ComponentRecord, payloads map[string][]byte) BoundParent {
	return BoundParent{Record: record, Payload: payloads[record.Payload.Digest]}
}

func referenceProvenanceValidatorID(typeID string) string {
	return referenceProvenancePayloadPrefix + strings.TrimPrefix(typeID, "evalwitness.")
}
