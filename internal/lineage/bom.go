package lineage

import (
	"errors"
	"slices"
)

const TransformationTargetBindingVersion = "evalwitness.transformation-target-binding.v1"

type TransformationTargetBinding struct {
	Version              string   `json:"version"`
	TargetID             string   `json:"target_id"`
	TaskGroupID          string   `json:"task_group_id"`
	SourceObjectDigest   string   `json:"source_object_digest"`
	RetainedObjectDigest string   `json:"retained_object_digest"`
	RequiredFields       []string `json:"required_fields"`
	DecisiveChannels     []string `json:"decisive_channels"`
	Digest               string   `json:"digest"`
}

func BuildTransformationTargetBinding(targetID, taskGroupID, sourceObjectDigest, retainedObjectDigest string, requiredFields, decisiveChannels []string) (TransformationTargetBinding, error) {
	binding := TransformationTargetBinding{
		Version: TransformationTargetBindingVersion, TargetID: targetID, TaskGroupID: taskGroupID,
		SourceObjectDigest: sourceObjectDigest, RetainedObjectDigest: retainedObjectDigest,
		RequiredFields: append([]string(nil), requiredFields...), DecisiveChannels: append([]string(nil), decisiveChannels...),
	}
	var err error
	binding.Digest, err = digestJSON(binding)
	if err != nil {
		return TransformationTargetBinding{}, err
	}
	return binding, binding.Validate()
}

func (binding TransformationTargetBinding) Validate() error {
	if binding.Version != TransformationTargetBindingVersion || missing(binding.TargetID, binding.TaskGroupID) ||
		!validDigest(binding.SourceObjectDigest) || !validDigest(binding.RetainedObjectDigest) {
		return errors.New("transformation-target binding identity is invalid")
	}
	if err := validateSortedUnique("transformation-target required fields", binding.RequiredFields, 1); err != nil {
		return err
	}
	if err := validateSortedUnique("transformation-target decisive channels", binding.DecisiveChannels, 1); err != nil {
		return err
	}
	copy := binding
	copy.Digest = ""
	expected, err := digestJSON(copy)
	if err != nil {
		return err
	}
	if binding.Digest != expected {
		return errors.New("transformation-target binding digest is invalid")
	}
	return nil
}

func BuildVerificationEvidenceBOM(
	bomID string,
	source VerificationLineageSource,
	witness ExecutionWitness,
	pairing WitnessNativePairing,
	candidate LineageCandidate,
	assessment LineageAssessment,
	audit LineageAudit,
	failability FailabilityAnalysis,
	transformationTarget TransformationTargetBinding,
	requiredFields []string,
	decisiveChannels []string,
) (VerificationEvidenceBOM, error) {
	if missing(bomID) {
		return VerificationEvidenceBOM{}, errors.New("verification-evidence BOM construction identity is invalid")
	}
	fields := append([]string(nil), requiredFields...)
	channels := append([]string(nil), decisiveChannels...)
	if err := validateSortedUnique("BOM construction required fields", fields, 1); err != nil {
		return VerificationEvidenceBOM{}, err
	}
	if err := validateSortedUnique("BOM construction decisive channels", channels, 1); err != nil {
		return VerificationEvidenceBOM{}, err
	}
	if err := validateBOMInputs(source, witness, pairing, candidate, assessment, audit, failability, transformationTarget, fields, channels); err != nil {
		return VerificationEvidenceBOM{}, err
	}
	retentionAnalysis, err := AnalyzeRetention(candidate, fields, channels)
	if err != nil {
		return VerificationEvidenceBOM{}, err
	}
	if !retentionAnalysis.Complete {
		return VerificationEvidenceBOM{}, errors.New("verification-evidence BOM cannot be built from a lossy retention analysis")
	}
	parents := []ParentRef{
		parentFromArtifact("source", source.Header), parentFromArtifact("witness", witness.Header),
		parentFromArtifact("candidate", candidate.Header), parentFromArtifact("assessment", assessment.Header),
		parentFromArtifact("audit", audit.Header),
	}
	bom := VerificationEvidenceBOM{
		Header: ArtifactHeader{
			SchemaVersion: BOMSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: bomID, TaskID: candidate.Header.TaskID, TaskGroupID: candidate.Header.TaskGroupID,
			DataRole: candidate.Header.DataRole, PlanDigest: LockedPlanDigest, Parents: parents,
		},
		BOMID: bomID, CandidateID: candidate.CandidateID, SourceDigest: source.Header.Digest,
		ExecutionWitnessDigest: witness.Header.Digest, CandidateDigest: candidate.Header.Digest,
		AssessmentDigest: assessment.Header.Digest, AuditDigest: audit.Header.Digest,
		Evidence: EvidenceBinding{
			ClaimID: candidate.ClaimID, FailableProperty: failability.Finding.Property, Executable: failability.Executable,
			Operands: append([]string(nil), failability.Argv[1:]...), ObservableFailureCondition: failability.Finding.FailureCondition,
			CallIDs: []string{pairing.InvocationID}, ResultIDs: []string{pairing.ToolResultEventID},
			RequiredFields: fields, DecisiveChannels: channels,
		},
		Retention: RetentionBinding{
			NativeRecordDigest: candidate.Layers[1].ObjectDigest, CanonicalEventDigest: candidate.Layers[2].ObjectDigest,
			TransformationTargetDigest: transformationTarget.Digest, RetainedBundleDigest: candidate.Layers[3].ObjectDigest,
			RequestFingerprint: candidate.RequestFingerprint, CanonicalRequestLineageDigest: candidate.CanonicalRequestLineageDigest,
			SurvivingChannels:       append([]string(nil), retentionAnalysis.SurvivingChannels...),
			TruncatedRequiredFields: append([]string(nil), retentionAnalysis.TruncatedRequiredFields...),
		},
		RepositoryStateDigest: witness.RepositoryStateDigest, ValidityInterval: assessment.Freshness, Accepted: true,
	}
	bom.Header.Digest, err = artifactDigest(bom)
	if err != nil {
		return VerificationEvidenceBOM{}, err
	}
	if err := ValidateBOMLineage(bom, source, witness, pairing, candidate, assessment, audit, failability, transformationTarget); err != nil {
		return VerificationEvidenceBOM{}, err
	}
	return bom, nil
}

type EvidenceBinding struct {
	ClaimID                    string   `json:"claim_id"`
	FailableProperty           string   `json:"failable_property"`
	Executable                 string   `json:"executable"`
	Operands                   []string `json:"operands"`
	ObservableFailureCondition string   `json:"observable_failure_condition"`
	CallIDs                    []string `json:"call_ids"`
	ResultIDs                  []string `json:"result_ids"`
	RequiredFields             []string `json:"required_fields"`
	DecisiveChannels           []string `json:"decisive_channels"`
}

type RetentionBinding struct {
	NativeRecordDigest            string   `json:"native_record_digest"`
	CanonicalEventDigest          string   `json:"canonical_event_digest"`
	TransformationTargetDigest    string   `json:"transformation_target_digest"`
	RetainedBundleDigest          string   `json:"retained_bundle_digest"`
	RequestFingerprint            string   `json:"request_fingerprint"`
	CanonicalRequestLineageDigest string   `json:"canonical_request_lineage_digest"`
	SurvivingChannels             []string `json:"surviving_channels"`
	TruncatedRequiredFields       []string `json:"truncated_required_fields"`
}

type VerificationEvidenceBOM struct {
	Header                 ArtifactHeader    `json:"header"`
	BOMID                  string            `json:"bom_id"`
	CandidateID            string            `json:"candidate_id"`
	SourceDigest           string            `json:"source_digest"`
	ExecutionWitnessDigest string            `json:"execution_witness_digest"`
	CandidateDigest        string            `json:"candidate_digest"`
	AssessmentDigest       string            `json:"assessment_digest"`
	AuditDigest            string            `json:"audit_digest"`
	Evidence               EvidenceBinding   `json:"evidence"`
	Retention              RetentionBinding  `json:"retention"`
	RepositoryStateDigest  string            `json:"repository_state_digest"`
	ValidityInterval       FreshnessInterval `json:"validity_interval"`
	Accepted               bool              `json:"accepted"`
}

func (bom VerificationEvidenceBOM) Validate() error {
	if err := validateHeader(bom.Header, BOMSchemaVersion, []ParentRequirement{
		{Relation: "source", SchemaVersions: []string{SourceSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true},
		{Relation: "witness", SchemaVersions: []string{WitnessSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true},
		{Relation: "candidate", SchemaVersions: []string{CandidateSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true},
		{Relation: "assessment", SchemaVersions: []string{AssessmentSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true},
		{Relation: "audit", SchemaVersions: []string{AuditSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true},
	}); err != nil {
		return err
	}
	if missing(bom.BOMID, bom.CandidateID, bom.Evidence.ClaimID, bom.Evidence.FailableProperty,
		bom.Evidence.Executable, bom.Evidence.ObservableFailureCondition) || !bom.Accepted ||
		!validDigest(bom.SourceDigest) || !validDigest(bom.ExecutionWitnessDigest) || !validDigest(bom.CandidateDigest) ||
		!validDigest(bom.AssessmentDigest) || !validDigest(bom.AuditDigest) ||
		!validDigest(bom.RepositoryStateDigest) {
		return errors.New("verification-evidence BOM identity or acceptance is invalid")
	}
	if err := bom.ValidityInterval.Validate(); err != nil {
		return err
	}
	if bom.ValidityInterval.State != FreshnessCurrent {
		return errors.New("accepted verification-evidence BOM requires current state lineage")
	}
	if bom.ValidityInterval.ObservedStateDigest != bom.RepositoryStateDigest {
		return errors.New("verification-evidence BOM freshness state differs from its repository binding")
	}
	bindings := []struct {
		relation string
		digest   string
	}{{"source", bom.SourceDigest}, {"witness", bom.ExecutionWitnessDigest}, {"candidate", bom.CandidateDigest}, {"assessment", bom.AssessmentDigest}, {"audit", bom.AuditDigest}}
	for _, binding := range bindings {
		parent, found := parentReference(bom.Header, binding.relation)
		if !found || parent.Digest != binding.digest {
			return errors.New("verification-evidence BOM digest binding does not match its parent")
		}
		if binding.relation == "candidate" && parent.ObjectID != bom.CandidateID {
			return errors.New("verification-evidence BOM candidate identity does not match its parent")
		}
	}
	if err := validateOrderedStrings("BOM operands", bom.Evidence.Operands); err != nil {
		return err
	}
	for name, values := range map[string][]string{
		"BOM call IDs":          bom.Evidence.CallIDs,
		"BOM result IDs":        bom.Evidence.ResultIDs,
		"BOM required fields":   bom.Evidence.RequiredFields,
		"BOM decisive channels": bom.Evidence.DecisiveChannels,
	} {
		if err := validateSortedUnique(name, values, 1); err != nil {
			return err
		}
	}
	retention := bom.Retention
	if !validDigest(retention.NativeRecordDigest) || !validDigest(retention.CanonicalEventDigest) ||
		!validDigest(retention.TransformationTargetDigest) || !validDigest(retention.RetainedBundleDigest) ||
		!validDigest(retention.RequestFingerprint) || !validDigest(retention.CanonicalRequestLineageDigest) ||
		len(retention.SurvivingChannels) == 0 || len(retention.TruncatedRequiredFields) != 0 {
		return errors.New("verification-evidence BOM retention chain is invalid or weakened")
	}
	if err := validateSortedUnique("BOM surviving channels", retention.SurvivingChannels, 1); err != nil {
		return err
	}
	for _, decisiveChannel := range bom.Evidence.DecisiveChannels {
		if !slices.Contains(retention.SurvivingChannels, decisiveChannel) {
			return errors.New("verification-evidence BOM lost a decisive claim channel")
		}
	}
	copy := bom
	copy.Header.Digest = ""
	return validateArtifactDigest(bom.Header.Digest, copy)
}

func ValidateBOMLineage(
	bom VerificationEvidenceBOM,
	source VerificationLineageSource,
	witness ExecutionWitness,
	pairing WitnessNativePairing,
	candidate LineageCandidate,
	assessment LineageAssessment,
	audit LineageAudit,
	failability FailabilityAnalysis,
	transformationTarget TransformationTargetBinding,
) error {
	if err := bom.Validate(); err != nil {
		return err
	}
	if err := validateBOMInputs(source, witness, pairing, candidate, assessment, audit, failability, transformationTarget, bom.Evidence.RequiredFields, bom.Evidence.DecisiveChannels); err != nil {
		return err
	}
	if err := ValidateBOMCandidateBinding(bom, candidate); err != nil {
		return err
	}
	if bom.BOMID != bom.Header.ObjectID || bom.SourceDigest != source.Header.Digest || bom.ExecutionWitnessDigest != witness.Header.Digest ||
		bom.AssessmentDigest != assessment.Header.Digest || bom.AuditDigest != audit.Header.Digest ||
		bom.RepositoryStateDigest != witness.RepositoryStateDigest || bom.ValidityInterval.State != FreshnessCurrent ||
		bom.ValidityInterval.ObservedStateDigest != witness.RepositoryStateDigest ||
		bom.Evidence.FailableProperty != failability.Finding.Property || bom.Evidence.Executable != failability.Executable ||
		!slices.Equal(bom.Evidence.Operands, failability.Argv[1:]) || bom.Evidence.ObservableFailureCondition != failability.Finding.FailureCondition ||
		!slices.Equal(bom.Evidence.CallIDs, []string{pairing.InvocationID}) || !slices.Equal(bom.Evidence.ResultIDs, []string{pairing.ToolResultEventID}) ||
		bom.Retention.TransformationTargetDigest != transformationTarget.Digest {
		return errors.New("verification-evidence BOM differs from its traversed evidence lineage")
	}
	return nil
}

func ValidateBOMCandidateBinding(bom VerificationEvidenceBOM, candidate LineageCandidate) error {
	if err := bom.Validate(); err != nil {
		return err
	}
	if err := candidate.Validate(); err != nil {
		return err
	}
	if bom.CandidateID != candidate.CandidateID || bom.CandidateDigest != candidate.Header.Digest || bom.Evidence.ClaimID != candidate.ClaimID {
		return errors.New("verification-evidence BOM candidate identity, digest, or claim was substituted")
	}
	if bom.Retention.RequestFingerprint != candidate.RequestFingerprint ||
		bom.Retention.CanonicalRequestLineageDigest != candidate.CanonicalRequestLineageDigest {
		return errors.New("verification-evidence BOM request body or canonical lineage differs from its candidate")
	}
	expectedLayerDigests := []string{
		bom.ExecutionWitnessDigest, bom.Retention.NativeRecordDigest, bom.Retention.CanonicalEventDigest,
		bom.Retention.RetainedBundleDigest, bom.Retention.RequestFingerprint,
	}
	if len(candidate.Layers) != len(expectedLayerDigests) {
		return errors.New("verification-evidence BOM candidate does not contain five layers")
	}
	for index, layer := range candidate.Layers {
		if layer.ObjectDigest != expectedLayerDigests[index] {
			return errors.New("verification-evidence BOM five-layer digest chain differs from its candidate")
		}
		for _, requiredField := range bom.Evidence.RequiredFields {
			if !slices.Contains(layer.RequiredFields, requiredField) {
				return errors.New("verification-evidence BOM required field is absent from a candidate layer")
			}
		}
		for _, decisiveChannel := range bom.Evidence.DecisiveChannels {
			if !slices.Contains(layer.DecisiveChannels, decisiveChannel) {
				return errors.New("verification-evidence BOM decisive channel is absent from a candidate layer")
			}
		}
	}
	return nil
}

func validateBOMInputs(
	source VerificationLineageSource,
	witness ExecutionWitness,
	pairing WitnessNativePairing,
	candidate LineageCandidate,
	assessment LineageAssessment,
	audit LineageAudit,
	failability FailabilityAnalysis,
	transformationTarget TransformationTargetBinding,
	requiredFields []string,
	decisiveChannels []string,
) error {
	for _, validation := range []func() error{source.Validate, witness.Validate, pairing.Validate, candidate.Validate, assessment.Validate, audit.Validate, failability.Validate, transformationTarget.Validate} {
		if err := validation(); err != nil {
			return err
		}
	}
	if failability.Resolution != FailabilityProven || failability.Finding != assessment.Failability ||
		assessment.TerminalState != StateDirectVerificationInvocation || assessment.StateDisposition != DispositionEligible ||
		assessment.EquivalentChannelSurvives || assessment.Freshness.State != FreshnessCurrent ||
		assessment.CandidateID != candidate.CandidateID || candidate.InvocationID != witness.InvocationID ||
		pairing.InvocationID != witness.InvocationID || pairing.SourceDigest != source.Header.Digest || pairing.WitnessDigest != witness.Header.Digest ||
		pairing.CommandOperandsDigest != witness.CommandOperandsDigest || candidate.Alignment.CommandOperandsDigest != witness.CommandOperandsDigest ||
		!candidate.Alignment.CallIDMatched || !candidate.Alignment.ParentEdgesMatched || candidate.Alignment.TimestampOnlyCausality ||
		!slices.Equal(failability.Argv, witness.Argv) || failability.Finding.OperandsDigest != witness.CommandOperandsDigest ||
		witness.RepositoryStateDigest != assessment.Freshness.ObservedStateDigest {
		return errors.New("verification-evidence BOM inputs do not establish one eligible execution lineage")
	}
	if source.RawRecordDigest != pairing.NativeRawDigest || source.CanonicalTrajectoryDigest != pairing.CanonicalTrajectoryDigest ||
		candidate.Layers[0].ObjectDigest != witness.Header.Digest || candidate.Layers[1].ObjectDigest != source.RawRecordDigest ||
		candidate.Layers[2].ObjectDigest != source.CanonicalTrajectoryDigest || candidate.Layers[4].ObjectDigest != candidate.RequestFingerprint {
		return errors.New("verification-evidence BOM inputs disagree on five-layer object identity")
	}
	if transformationTarget.TaskGroupID != candidate.Header.TaskGroupID || transformationTarget.SourceObjectDigest != candidate.Layers[2].ObjectDigest ||
		transformationTarget.RetainedObjectDigest != candidate.Layers[3].ObjectDigest ||
		!slices.Equal(transformationTarget.RequiredFields, requiredFields) || !slices.Equal(transformationTarget.DecisiveChannels, decisiveChannels) {
		return errors.New("verification-evidence transformation target differs from the candidate or claim contract")
	}
	if !parentMatches(candidate.Header, "source", source.Header) || !parentMatches(candidate.Header, "witness", witness.Header) ||
		!parentMatches(assessment.Header, "candidate", candidate.Header) || !parentMatches(assessment.Header, "witness", witness.Header) ||
		!parentMatches(audit.Header, "assessment", assessment.Header) {
		return errors.New("verification-evidence BOM parent traversal is incomplete or substituted")
	}
	for _, proofID := range failability.ProofRecordIDs {
		if !slices.Contains(assessment.ProofRecordIDs, proofID) {
			return errors.New("verification-evidence assessment dropped failability proof lineage")
		}
	}
	if err := validateSortedUnique("BOM required fields", requiredFields, 1); err != nil {
		return err
	}
	if err := validateSortedUnique("BOM decisive channels", decisiveChannels, 1); err != nil {
		return err
	}
	retentionAnalysis, err := AnalyzeRetention(candidate, requiredFields, decisiveChannels)
	if err != nil {
		return err
	}
	if !retentionAnalysis.Complete {
		return errors.New("accepted verification-evidence BOM contains a first-layer retention loss")
	}
	if !slices.Contains(candidate.Layers[0].RecordIDs, witness.InvocationID) ||
		!layerContainsRecords(candidate.Layers[1], pairing.ToolCallEventID, pairing.CommandEventID, pairing.ToolResultEventID) ||
		!layerContainsRecords(candidate.Layers[2], pairing.ToolCallEventID, pairing.CommandEventID, pairing.ToolResultEventID) ||
		!layerContainsRecords(candidate.Layers[3], pairing.ToolCallEventID, pairing.CommandEventID, pairing.ToolResultEventID) ||
		!slices.Contains(candidate.Layers[4].RecordIDs, pairing.InvocationID) {
		return errors.New("verification-evidence record identities did not survive the five-layer chain")
	}
	return nil
}

func parentFromArtifact(relation string, header ArtifactHeader) ParentRef {
	return ParentRef{Relation: relation, SchemaVersion: header.SchemaVersion, ObjectID: header.ObjectID, TaskID: header.TaskID, TaskGroupID: header.TaskGroupID, Digest: header.Digest}
}

func parentMatches(child ArtifactHeader, relation string, expected ArtifactHeader) bool {
	for _, parent := range child.Parents {
		if parent.Relation == relation && parent.ObjectID == expected.ObjectID && parent.SchemaVersion == expected.SchemaVersion &&
			parent.TaskID == expected.TaskID && parent.TaskGroupID == expected.TaskGroupID && parent.Digest == expected.Digest {
			return true
		}
	}
	return false
}

func layerContainsRecords(layer LayerBinding, recordIDs ...string) bool {
	for _, recordID := range recordIDs {
		if !slices.Contains(layer.RecordIDs, recordID) {
			return false
		}
	}
	return true
}

func validateOrderedStrings(name string, values []string) error {
	for _, value := range values {
		if missing(value) {
			return errors.New(name + " contains an empty value")
		}
	}
	return nil
}
