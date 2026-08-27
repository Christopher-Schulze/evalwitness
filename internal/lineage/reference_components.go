package lineage

import "errors"

type VerificationLineageReferenceComponents struct {
	Plan         VerificationLineagePlan
	Source       VerificationLineageSource
	Witness      ExecutionWitness
	Candidate    LineageCandidate
	Assessment   LineageAssessment
	Capabilities []TraceCapabilityVector
	Audit        LineageAudit
	BOM          VerificationEvidenceBOM
	DatasetCard  VerificationLineageDatasetCard
	Release      VerificationLineageRelease
}

func BuildVerificationLineageReferenceComponents(repositoryRoot string) (VerificationLineageReferenceComponents, error) {
	plan, err := DefaultPlan()
	if err != nil {
		return VerificationLineageReferenceComponents{}, err
	}
	evidence, err := buildOfflinePortfolioEvidence(repositoryRoot)
	if err != nil {
		return VerificationLineageReferenceComponents{}, err
	}
	datasetCard, err := BuildVerificationLineageDevelopmentDatasetCard(repositoryRoot)
	if err != nil {
		return VerificationLineageReferenceComponents{}, err
	}
	release, err := BuildVerificationLineageDevelopmentRelease(repositoryRoot)
	if err != nil {
		return VerificationLineageReferenceComponents{}, err
	}
	components := VerificationLineageReferenceComponents{
		Plan: plan, Source: evidence.Source, Witness: evidence.Witness, Candidate: evidence.Candidate,
		Assessment: evidence.Assessment, Capabilities: append([]TraceCapabilityVector(nil), evidence.Matrix.Vectors...), Audit: evidence.Audit,
		BOM: evidence.BOM, DatasetCard: datasetCard, Release: release,
	}
	if err := components.Validate(); err != nil {
		return VerificationLineageReferenceComponents{}, err
	}
	return components, nil
}

func (c VerificationLineageReferenceComponents) Validate() error {
	for _, validation := range []func() error{
		c.Plan.Validate, c.Source.Validate, c.Witness.Validate, c.Candidate.Validate,
		c.Assessment.Validate, c.Audit.Validate, c.BOM.Validate,
		c.DatasetCard.Validate, c.Release.Validate,
	} {
		if err := validation(); err != nil {
			return err
		}
	}
	if len(c.Capabilities) != 3 {
		return errors.New("verification-lineage reference requires all three capability vectors")
	}
	for _, capability := range c.Capabilities {
		if err := capability.Validate(); err != nil {
			return err
		}
	}
	datasetAuditDigest, found := parentDigest(c.DatasetCard.Header, "audit")
	if !found {
		return errors.New("verification-lineage dataset card has no audit parent")
	}
	if c.Plan.Digest != LockedPlanDigest || c.BOM.SourceDigest != c.Source.Header.Digest ||
		c.BOM.ExecutionWitnessDigest != c.Witness.Header.Digest || c.BOM.CandidateDigest != c.Candidate.Header.Digest ||
		c.BOM.AssessmentDigest != c.Assessment.Header.Digest || c.BOM.AuditDigest != c.Audit.Header.Digest ||
		datasetAuditDigest != c.Audit.Header.Digest || c.Release.AuditDigest != c.Audit.Header.Digest ||
		c.Release.DatasetCardDigest != c.DatasetCard.Header.Digest {
		return errors.New("verification-lineage reference components do not preserve the ten-component parent chain")
	}
	return nil
}
