package reliance

import (
	"errors"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type relationEventGraph struct {
	SchemaVersion    string                     `json:"schema_version"`
	TrajectorySchema string                     `json:"trajectory_schema"`
	SourceFormat     preprocess.SourceFormat    `json:"source_format"`
	SourceDigest     string                     `json:"source_digest"`
	Events           []preprocess.Event         `json:"events"`
	Links            []preprocess.Link          `json:"links"`
	Report           preprocess.IngestionReport `json:"report"`
}

func BindRelationBackedIntervention(inputs RelationAdmissionInputs) (RelationBackedInterventionAdmission, error) {
	value, err := buildRelationBackedAdmission(inputs)
	if err != nil {
		return RelationBackedInterventionAdmission{}, err
	}
	digest, err := relationBackedAdmissionDigest(value)
	if err != nil {
		return RelationBackedInterventionAdmission{}, err
	}
	value.Digest = digest
	if err := value.Validate(inputs); err != nil {
		return RelationBackedInterventionAdmission{}, err
	}
	return value, nil
}

func buildRelationBackedAdmission(inputs RelationAdmissionInputs) (RelationBackedInterventionAdmission, error) {
	if err := validateRelationAdmissionParents(inputs); err != nil {
		return RelationBackedInterventionAdmission{}, err
	}
	originalDigest, transformedDigest, err := validateRelationReplayBinding(inputs)
	if err != nil {
		return RelationBackedInterventionAdmission{}, err
	}
	status, reasons, primary, sensitivity := assessRelationBackedAdmissibility(inputs)
	intervention := inputs.Intervention.Intervention
	construct := inputs.ConstructAdmission
	return RelationBackedInterventionAdmission{
		SchemaVersion: RelationBackedAdmissionSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		InterventionDigest: intervention.Digest, AssignmentSetDigest: intervention.AssignmentSetDigest,
		FactorID: intervention.FactorID, Operator: intervention.Operator, EstimandFamily: intervention.EstimandFamily,
		InterventionAdmissibility:        intervention.Admissibility,
		InterventionAdmissibilityReasons: slices.Clone(intervention.AdmissibilityReasons),
		RelationID:                       inputs.Relation.ID, RelationDigest: inputs.Relation.Digest,
		RelationCaseID: inputs.ReplayedRelationCase.CaseID, RelationManifestDigest: inputs.ReplayedRelationCase.ManifestDigest,
		ConstructAdmissionDigest: construct.Digest, FormalWitnessDigest: construct.FormalWitnessDigest,
		ConstructFirewallDigest: construct.ConstructFirewallDigest, OwnerAttestationDigest: construct.OwnerAttestationDigest,
		TerminalLedgerDigest: construct.TerminalLedgerDigest, HumanResolutionDigest: construct.HumanResolutionDigest,
		ConstructStatus: construct.Status, OriginalEventGraphDigest: originalDigest,
		TransformedEventGraphDigest: transformedDigest, Admissibility: status, AdmissibilityReasons: reasons,
		PrimaryEligible: primary, SensitivityEligible: sensitivity,
		AssignmentFrozenBeforeOutput: intervention.AssignmentFrozenBeforeOutput,
		ProviderCalls:                0, NetworkRequired: false,
	}, nil
}

func validateRelationAdmissionParents(inputs RelationAdmissionInputs) error {
	if err := inputs.Intervention.Validate(inputs.Ontology, inputs.Estimands, inputs.Assignments, inputs.Parent); err != nil {
		return relationAdmissionFailure(err)
	}
	if err := inputs.Relation.Validate(); err != nil {
		return relationAdmissionFailure(err)
	}
	if err := inputs.ConstructAdmission.Validate(); err != nil {
		return relationAdmissionFailure(err)
	}
	if inputs.Relation.Transform.Kind != stress.TransformMutation || inputs.Relation.Applicability.Unit != stress.UnitTrajectory {
		return relationAdmissionFailure(errors.New("relation-backed intervention requires one trajectory-level v3 mutation relation"))
	}
	return validateRelationEstimandStatus(inputs.Relation, inputs.ConstructAdmission)
}

func validateRelationEstimandStatus(spec stress.Relation, admission stress.ConstructAdmission) error {
	switch spec.StatisticalFamily.Estimand {
	case stress.EstimandPrimaryCore:
		if admission.Status == stress.AdmissionFormalOnly {
			return relationAdmissionFailure(errors.New("primary-core relation cannot use formal-only admission"))
		}
	case stress.EstimandSensitivity, stress.EstimandScarcitySentinel:
	default:
		return relationAdmissionFailure(errors.New("relation-backed intervention uses an unsupported stress estimand"))
	}
	return nil
}

func validateRelationReplayBinding(inputs RelationAdmissionInputs) (string, string, error) {
	replayed := inputs.ReplayedRelationCase
	if err := replayed.ValidateConstructBinding(inputs.Relation, inputs.ConstructAdmission); err != nil {
		return "", "", relationAdmissionFailure(err)
	}
	if len(replayed.Original) != 1 || len(replayed.Transformed) != 1 {
		return "", "", relationAdmissionFailure(errors.New("relation replay is not one canonical trajectory pair"))
	}
	if err := replayed.Original[0].Validate(); err != nil {
		return "", "", relationAdmissionFailure(err)
	}
	if err := replayed.Transformed[0].Validate(); err != nil {
		return "", "", relationAdmissionFailure(err)
	}
	if replayed.Original[0].Digest == replayed.Transformed[0].Digest {
		return "", "", relationAdmissionFailure(errors.New("relation replay does not contain a transformed trajectory"))
	}
	return validateRelationEventGraphs(inputs, replayed.Original[0], replayed.Transformed[0])
}

func validateRelationEventGraphs(
	inputs RelationAdmissionInputs,
	original, transformed preprocess.Trajectory,
) (string, string, error) {
	if !reflect.DeepEqual(inputs.Parent, original) {
		return "", "", relationAdmissionFailure(errors.New("intervention parent differs from the replayed relation source"))
	}
	originalDigest, err := relationEventGraphDigest(original)
	if err != nil {
		return "", "", relationAdmissionFailure(err)
	}
	transformedDigest, err := relationEventGraphDigest(transformed)
	if err != nil {
		return "", "", relationAdmissionFailure(err)
	}
	interventionDigest, err := relationEventGraphDigest(inputs.Intervention.Trajectory)
	if err != nil || interventionDigest != transformedDigest {
		return "", "", relationAdmissionFailure(errors.New("intervention event graph differs from the replayed relation transform"))
	}
	if originalDigest == transformedDigest {
		return "", "", relationAdmissionFailure(errors.New("relation replay event graph did not change"))
	}
	return originalDigest, transformedDigest, nil
}

func assessRelationBackedAdmissibility(
	inputs RelationAdmissionInputs,
) (InterventionAdmissibility, []InterventionFailureCode, bool, bool) {
	intervention := inputs.Intervention.Intervention
	baseReady := intervention.Admissibility == InterventionAdmissible || relationCanResolveQualityChange(intervention)
	if !baseReady {
		return intervention.Admissibility, slices.Clone(intervention.AdmissibilityReasons), false, false
	}
	construct := inputs.ConstructAdmission
	switch construct.Status {
	case stress.AdmissionHumanSupported:
		primary, sensitivity := relationDenominatorEligibility(inputs.Relation, construct)
		return InterventionAdmissible, []InterventionFailureCode{}, primary, sensitivity
	case stress.AdmissionFormalOnly:
		_, sensitivity := relationDenominatorEligibility(inputs.Relation, construct)
		return InterventionUnresolved, []InterventionFailureCode{FailureRelationFormalOnly}, false, sensitivity
	case stress.AdmissionHumanUnresolved:
		_, sensitivity := relationDenominatorEligibility(inputs.Relation, construct)
		return InterventionUnresolved, []InterventionFailureCode{FailureRelationHumanUnresolved}, false, sensitivity
	case stress.AdmissionHumanContradicted:
		return InterventionInadmissible, []InterventionFailureCode{FailureRelationHumanContradicted}, false, false
	default:
		return InterventionInadmissible, []InterventionFailureCode{FailureRelationEvidenceInvalid}, false, false
	}
}

func relationCanResolveQualityChange(value EvidenceIntervention) bool {
	return value.EstimandFamily == EstimandQualityChanging && value.Admissibility == InterventionUnresolved &&
		slices.Equal(value.AdmissibilityReasons, []InterventionFailureCode{FailureRelationAdmissionRequired})
}

func relationDenominatorEligibility(spec stress.Relation, admission stress.ConstructAdmission) (bool, bool) {
	primary := spec.StatisticalFamily.Estimand == stress.EstimandPrimaryCore && admission.PrimaryEligible
	sensitivity := (spec.StatisticalFamily.Estimand == stress.EstimandSensitivity ||
		spec.StatisticalFamily.Estimand == stress.EstimandScarcitySentinel) && admission.SensitivityEligible
	return primary, sensitivity
}

func (value RelationBackedInterventionAdmission) Validate(inputs RelationAdmissionInputs) error {
	expected, err := buildRelationBackedAdmission(inputs)
	if err != nil {
		return err
	}
	expected.Digest, err = relationBackedAdmissionDigest(expected)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("relation-backed intervention admission does not reproduce from its frozen parents")
	}
	return nil
}

func relationEventGraphDigest(trajectory preprocess.Trajectory) (string, error) {
	return protocolkit.Digest(relationEventGraph{
		SchemaVersion: RelationEventGraphSchemaVersion, TrajectorySchema: trajectory.SchemaVersion,
		SourceFormat: trajectory.SourceFormat, SourceDigest: trajectory.SourceDigest,
		Events: trajectory.Events, Links: trajectory.Links, Report: trajectory.Report,
	})
}

func relationBackedAdmissionDigest(value RelationBackedInterventionAdmission) (string, error) {
	value.Digest = ""
	return protocolkit.Digest(value)
}

func relationAdmissionFailure(err error) error {
	return interventionFailure(FailureRelationEvidenceInvalid, err)
}
