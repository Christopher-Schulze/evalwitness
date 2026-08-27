package stress

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
)

type AdmissionError struct {
	State  InvalidState
	Reason string
}

func (err *AdmissionError) Error() string {
	return fmt.Sprintf("stress construct admission rejected (%s): %s", err.State, err.Reason)
}

func AdmitMutationCase(spec Relation, item mutation.CorpusCaseV3, owner relationevidence.OwnerInspectionPublicAttestation, expectedPackageDigest string, ledger *relationevidence.TerminalRelationLedger) (ConstructAdmission, error) {
	if err := spec.Validate(); err != nil {
		return ConstructAdmission{}, &AdmissionError{State: InvalidCrossVersion, Reason: err.Error()}
	}
	if spec.Transform.Kind != TransformMutation {
		return ConstructAdmission{}, &AdmissionError{State: InvalidNotApplicable, Reason: "construct admission requires a registered mutation relation"}
	}
	if err := validateMutationCaseBinding(spec, item); err != nil {
		return ConstructAdmission{}, err
	}
	if err := validateOwnerCustody(owner, expectedPackageDigest); err != nil {
		return ConstructAdmission{}, err
	}
	admission := ConstructAdmission{
		SchemaVersion: AdmissionSchemaVersion, CanonicalPolicy: CanonicalPolicy, CaseID: item.ID,
		FormalWitnessDigest: item.Manifest.Witness.Digest, ConstructFirewallDigest: item.ConstructFirewall.Digest,
		OwnerAttestationDigest: owner.Digest, Status: AdmissionFormalOnly, SensitivityEligible: true,
		Reason: "formal v3 witness and passed owner custody are verified; no blinded formal-human terminal ledger was supplied",
	}
	if ledger != nil {
		entry, err := verifiedLedgerEntry(*ledger, item)
		if err != nil {
			return ConstructAdmission{}, err
		}
		admission.TerminalLedgerDigest = ledger.Digest
		admission.HumanResolutionDigest = entry.HumanResolutionDigest
		switch entry.AdmissibilityStatus {
		case relationevidence.RelationAdmissibilityHumanSupported:
			admission.Status, admission.PrimaryEligible, admission.SensitivityEligible = AdmissionHumanSupported, true, true
			admission.Reason = "blinded formal-human terminal ledger supports the frozen formal relation"
		case relationevidence.RelationAdmissibilityHumanContradicted:
			admission.Status, admission.PrimaryEligible, admission.SensitivityEligible = AdmissionHumanContradicted, false, false
			admission.Reason = "blinded formal-human terminal ledger contradicts the frozen formal relation"
		case relationevidence.RelationAdmissibilityHumanUnresolved:
			admission.Status, admission.PrimaryEligible, admission.SensitivityEligible = AdmissionHumanUnresolved, false, true
			admission.Reason = "blinded formal-human terminal ledger leaves the frozen formal relation unresolved"
		default:
			return ConstructAdmission{}, &AdmissionError{State: InvalidCustody, Reason: "terminal ledger entry has an unsupported admissibility status"}
		}
	}
	digest, err := constructAdmissionDigest(admission)
	if err != nil {
		return ConstructAdmission{}, err
	}
	admission.Digest = digest
	if err := admission.Validate(); err != nil {
		return ConstructAdmission{}, err
	}
	return admission, nil
}

func (value ConstructAdmission) Validate() error {
	if value.SchemaVersion != AdmissionSchemaVersion || value.CanonicalPolicy != CanonicalPolicy || !identifierPattern.MatchString(value.CaseID) ||
		!validDigest(value.FormalWitnessDigest) || !validDigest(value.ConstructFirewallDigest) || !validDigest(value.OwnerAttestationDigest) ||
		!slices.Contains([]AdmissionStatus{AdmissionFormalOnly, AdmissionHumanSupported, AdmissionHumanContradicted, AdmissionHumanUnresolved}, value.Status) || value.Reason == "" {
		return errors.New("stress construct admission identity or evidence is invalid")
	}
	hasLedger := value.TerminalLedgerDigest != "" || value.HumanResolutionDigest != ""
	if hasLedger && (!validDigest(value.TerminalLedgerDigest) || !validDigest(value.HumanResolutionDigest)) {
		return errors.New("stress construct admission has a partial or invalid terminal-ledger binding")
	}
	switch value.Status {
	case AdmissionFormalOnly:
		if hasLedger || value.PrimaryEligible || !value.SensitivityEligible {
			return errors.New("formal-only construct admission has an invalid denominator role")
		}
	case AdmissionHumanSupported:
		if !hasLedger || !value.PrimaryEligible || !value.SensitivityEligible {
			return errors.New("human-supported construct admission has an invalid denominator role")
		}
	case AdmissionHumanContradicted:
		if !hasLedger || value.PrimaryEligible || value.SensitivityEligible {
			return errors.New("human-contradicted construct admission entered an execution denominator")
		}
	case AdmissionHumanUnresolved:
		if !hasLedger || value.PrimaryEligible || !value.SensitivityEligible {
			return errors.New("human-unresolved construct admission has an invalid denominator role")
		}
	}
	expected, err := constructAdmissionDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress construct admission digest is invalid")
	}
	return nil
}

func validateMutationCaseBinding(spec Relation, item mutation.CorpusCaseV3) error {
	if err := item.Manifest.Validate(); err != nil {
		return &AdmissionError{State: InvalidFormalWitness, Reason: err.Error()}
	}
	if err := item.BlindPacket.Validate(); err != nil {
		return &AdmissionError{State: InvalidFormalWitness, Reason: err.Error()}
	}
	if err := item.ConstructFirewall.Validate(); err != nil {
		return &AdmissionError{State: InvalidConstructRejected, Reason: err.Error()}
	}
	if item.ID != item.Manifest.MutationID || item.Family != item.Manifest.Program.Family || item.Family != spec.Transform.MutationFamily ||
		item.Manifest.Program.Version != mutation.MutationProgramVersionV3 || item.Manifest.RelationContractVersion != mutation.RelationContractVersionV3 ||
		item.Manifest.ExpectedRelation != spec.Transform.ExpectedFormalRelation || item.Manifest.Witness.Relation != item.Manifest.ExpectedRelation ||
		item.Manifest.Witness.Digest == "" || item.Manifest.ConstructFirewallDigest != item.ConstructFirewall.Digest ||
		item.ConstructFirewall.Status != mutation.ConstructApplied || item.ConstructFirewall.Family != item.Family ||
		item.BlindPacket.Digest != item.Manifest.Review.BlindPacketDigest || item.BlindPacket.OriginalDigest != item.Manifest.OriginalTrajectoryDigest ||
		item.BlindPacket.MutatedDigest != item.Manifest.MutatedTrajectoryDigest {
		return &AdmissionError{State: InvalidCrossVersion, Reason: "case, v3 manifest, formal witness, firewall, and blind packet do not form one exact chain"}
	}
	return nil
}

func validateOwnerCustody(owner relationevidence.OwnerInspectionPublicAttestation, expectedPackageDigest string) error {
	if !validDigest(expectedPackageDigest) {
		return &AdmissionError{State: InvalidCustody, Reason: "expected owner package inventory digest is invalid"}
	}
	if err := owner.Validate(); err != nil {
		return &AdmissionError{State: InvalidCustody, Reason: err.Error()}
	}
	if owner.PackageInventoryDigest != expectedPackageDigest || owner.Assessments.Required != relationevidence.PilotInspectionRequiredAssessments ||
		owner.Assessments.Completed != relationevidence.PilotInspectionRequiredAssessments || len(owner.Dimensions) != 16 ||
		owner.Outcomes.OverallStatus != relationevidence.PilotInspectionOverallPassed || !owner.Disclosure.PrivateChainVerified ||
		owner.Disclosure.PrivateJournalIdentitiesDisclosed || owner.Disclosure.RestrictedEvidenceDisclosed {
		return &AdmissionError{State: InvalidCustody, Reason: "owner attestation is not one passed, non-disclosing 66-assessment, 16-dimension package projection"}
	}
	return nil
}

func verifiedLedgerEntry(ledger relationevidence.TerminalRelationLedger, item mutation.CorpusCaseV3) (relationevidence.RelationLedgerEntry, error) {
	if ledger.SchemaVersion != relationevidence.TerminalRelationLedgerSchemaVersionV3 || ledger.ProtocolVersion != relationevidence.ProtocolVersionV3 {
		return relationevidence.RelationLedgerEntry{}, &AdmissionError{State: InvalidCrossVersion, Reason: "v3 mutation admission requires the v3 terminal relation ledger"}
	}
	if err := ledger.Validate(); err != nil {
		return relationevidence.RelationLedgerEntry{}, &AdmissionError{State: InvalidCustody, Reason: err.Error()}
	}
	if ledger.DataRole != relationevidence.ReviewDataPrimaryAudit {
		return relationevidence.RelationLedgerEntry{}, &AdmissionError{State: InvalidCustody, Reason: "terminal ledger is not the blinded primary-audit ledger"}
	}
	var match *relationevidence.RelationLedgerEntry
	for index := range ledger.Entries {
		entry := ledger.Entries[index]
		if entry.CaseID != item.ID {
			continue
		}
		if match != nil {
			return relationevidence.RelationLedgerEntry{}, &AdmissionError{State: InvalidCustody, Reason: "terminal ledger contains duplicate case entries"}
		}
		copy := entry
		match = &copy
	}
	if match == nil || match.Family != item.Family || match.ExpectedRelation != item.Manifest.ExpectedRelation || match.FormalWitnessDigest != item.Manifest.Witness.Digest {
		return relationevidence.RelationLedgerEntry{}, &AdmissionError{State: InvalidCrossVersion, Reason: "terminal ledger does not bind the exact case, family, relation, and formal witness"}
	}
	return *match, nil
}

func constructAdmissionDigest(value ConstructAdmission) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
