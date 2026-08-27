package explorer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/stress"
)

const (
	stressDevelopmentCaseStudyPath = "eval/results/stress-development-case-study-v1.json"
	stressReproductionCommand      = "scripts/audits/run-stress-lab.sh"
	maximumStressSidecarBytes      = int64(16 << 20)
)

type stressCaseLoader func(string) (stress.DevelopmentCaseStudy, []byte, error)

func loadStressDevelopmentCaseStudy(repositoryRoot string) (stress.DevelopmentCaseStudy, []byte, error) {
	rootPath, err := resolvedRepositoryRoot(repositoryRoot)
	if err != nil {
		return stress.DevelopmentCaseStudy{}, nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return stress.DevelopmentCaseStudy{}, nil, err
	}
	relative := filepath.FromSlash(stressDevelopmentCaseStudyPath)
	walked, err := inspectReleasePath(root, relative)
	if err != nil {
		return stress.DevelopmentCaseStudy{}, nil, errors.Join(err, root.Close())
	}
	raw, readErr := readBoundedStressSidecar(root, relative, walked)
	closeErr := root.Close()
	if readErr != nil || closeErr != nil {
		return stress.DevelopmentCaseStudy{}, nil, errors.Join(readErr, closeErr)
	}
	value, err := stress.DecodeDevelopmentCaseStudy(bytes.NewReader(raw))
	if err != nil {
		return stress.DevelopmentCaseStudy{}, nil, err
	}
	if err := value.ValidateAgainstRepository(rootPath); err != nil {
		return stress.DevelopmentCaseStudy{}, nil, err
	}
	return value, raw, nil
}

func readBoundedStressSidecar(root *os.Root, relative string, walked os.FileInfo) ([]byte, error) {
	opened, err := root.Open(relative)
	if err != nil {
		return nil, err
	}
	openedInfo, err := opened.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || openedInfo.Size() < 1 ||
		openedInfo.Size() > maximumStressSidecarBytes || !os.SameFile(walked, openedInfo) {
		return nil, errors.Join(err, opened.Close(), errors.New("stress sidecar changed while it was opened"))
	}
	raw, readErr := io.ReadAll(io.LimitReader(opened, maximumStressSidecarBytes+1))
	finalInfo, statErr := opened.Stat()
	pathInfo, pathErr := root.Lstat(relative)
	closeErr := opened.Close()
	if readErr != nil || statErr != nil || pathErr != nil || closeErr != nil ||
		len(raw) == 0 || int64(len(raw)) > maximumStressSidecarBytes || int64(len(raw)) != openedInfo.Size() ||
		!pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(openedInfo, finalInfo) || !os.SameFile(finalInfo, pathInfo) ||
		openedInfo.Size() != finalInfo.Size() || !openedInfo.ModTime().Equal(finalInfo.ModTime()) {
		return nil, errors.Join(readErr, statErr, pathErr, closeErr, errors.New("stress sidecar changed while it was read"))
	}
	return raw, nil
}

func buildStressView(value stress.DevelopmentCaseStudy, raw []byte) (StressView, error) {
	canonical, err := stress.EncodeIndented(value)
	if err != nil || !bytes.Equal(canonical, raw) {
		return StressView{}, errors.New("evidence explorer stress sidecar is not the deterministic repository projection")
	}
	if len(value.Trajectories) != len(value.Observation.OriginalOrder) {
		return StressView{}, errors.New("evidence explorer stress trajectories are detached from candidate order")
	}
	trajectoryByPath := make(map[string]string, len(value.Trajectories))
	for index, trajectory := range value.Trajectories {
		trajectoryByPath[trajectory.Path] = value.Observation.OriginalOrder[index]
	}
	steps := make([]StressReductionStepView, len(value.Counterexample.Steps))
	rejected := 0
	for index, step := range value.Counterexample.Steps {
		if step.Decision == stress.ReductionRejected {
			rejected++
		}
		steps[index] = StressReductionStepView{
			Index: step.Index, UnitID: step.UnitID, Decision: string(step.Decision),
			RelationRevalidated: step.Observation.RelationRevalidated,
			PrivacyRevalidated:  step.Observation.PrivacyRevalidated,
			ViolationPreserved:  step.Observation.ViolationPreserved,
			BeforeDigest:        step.BeforeDigest, CandidateDigest: step.CandidateDigest,
			AfterDigest: step.AfterDigest, ObservationDigest: step.Observation.Digest,
		}
	}
	witness := make([]StressWitnessLineView, len(value.FinalWitness))
	for index, line := range value.FinalWitness {
		trajectoryID, found := trajectoryByPath[line.FixturePath]
		if !found {
			return StressView{}, fmt.Errorf("evidence explorer stress witness references unknown fixture %q", line.FixturePath)
		}
		witness[index] = StressWitnessLineView{
			TrajectoryID: trajectoryID, UnitID: line.UnitID, FixturePath: line.FixturePath,
			Line: line.Line, Content: line.Content,
		}
	}
	view := StressView{
		CaseStudyID: value.CaseStudyID, DataRole: value.DataRole, Status: value.Status,
		TaskRequirement: value.TaskRequirement, RelationID: value.Relation.ID,
		RelationKind: string(value.Relation.Kind), RelationDigest: value.Relation.Digest,
		ControlArmID:        value.Observation.ControlArmID,
		OriginalOrder:       slices.Clone(value.Observation.OriginalOrder),
		TransformedOrder:    slices.Clone(value.Observation.TransformedOrder),
		OriginalSelected:    value.Observation.OriginalSelected,
		TransformedSelected: value.Observation.TransformedSelected,
		Outcome:             string(value.Observation.Outcome),
		OriginalInputDigest: value.Counterexample.OriginalInputDigest,
		ReducedInputDigest:  value.Counterexample.ReducedInputDigest,
		OriginalLineUnits:   value.OriginalLineUnits, FinalLineUnits: value.FinalLineUnits,
		ReductionAttempts: len(steps), AcceptedReductions: value.AcceptedReductions,
		RejectedReductions: rejected, ReductionBasisPoints: int(value.ReductionPercent * 100),
		Minimality: string(value.Counterexample.Minimality), Steps: steps, FinalWitness: witness,
		EmpiricalUnits: value.EmpiricalUnits, ProviderCalls: value.ProviderCalls,
		NetworkRequired: value.NetworkRequired, ClaimBoundary: value.ClaimBoundary,
		AllowedClaims: slices.Clone(value.AllowedClaims), ForbiddenClaims: slices.Clone(value.ForbiddenClaims),
		ReproductionCommand: stressReproductionCommand, Digest: value.Digest,
		Source: sidecarArtifactRef(
			"stress-development-case-study-v1", stress.DevelopmentCaseStudySchemaVersion,
			raw, value.Digest,
		),
	}
	if err := validateStressView(view); err != nil {
		return StressView{}, err
	}
	return view, nil
}

func (report Report) validateStress() error {
	return validateStressView(report.Stress)
}

func validateStressView(view StressView) error {
	if view.CaseStudyID != "first-listed-candidate-order-one-minimal" || view.DataRole != "adapter_development" ||
		view.Status != "mechanism_demonstration" || strings.TrimSpace(view.TaskRequirement) == "" ||
		view.RelationID != "v3-sensitivity-candidate_order_reversal" || view.RelationKind != "invariance" ||
		!validDigest(view.RelationDigest) || view.ControlArmID != "zero-cost-first-listed" ||
		!slices.Equal(view.OriginalOrder, []string{"trajectory-a", "trajectory-b"}) ||
		!slices.Equal(view.TransformedOrder, []string{"trajectory-b", "trajectory-a"}) ||
		view.OriginalSelected != "trajectory-a" || view.TransformedSelected != "trajectory-b" ||
		view.Outcome != "violated" || !validDigest(view.OriginalInputDigest) || !validDigest(view.ReducedInputDigest) ||
		view.OriginalLineUnits != 32 || view.FinalLineUnits != 2 || view.ReductionAttempts != 53 ||
		view.AcceptedReductions != 30 || view.RejectedReductions != 23 || view.ReductionBasisPoints != 9375 ||
		view.Minimality != "one_minimal" || len(view.Steps) != view.ReductionAttempts ||
		len(view.FinalWitness) != view.FinalLineUnits || view.EmpiricalUnits != 0 || view.ProviderCalls != 0 ||
		view.NetworkRequired || strings.TrimSpace(view.ClaimBoundary) == "" ||
		view.ReproductionCommand != stressReproductionCommand || !validDigest(view.Digest) ||
		validateArtifactRef(view.Source) != nil || view.Source.Kind != ArtifactSidecar ||
		view.Source.ID != "stress-development-case-study-v1" ||
		view.Source.SchemaVersion != stress.DevelopmentCaseStudySchemaVersion || view.Source.ArtifactDigest != view.Digest {
		return errors.New("evidence explorer stress view identity or claim boundary is invalid")
	}
	if err := validateSortedUnique("stress allowed claims", view.AllowedClaims); err != nil {
		return err
	}
	if err := validateSortedUnique("stress forbidden claims", view.ForbiddenClaims); err != nil {
		return err
	}
	current := view.OriginalInputDigest
	accepted := 0
	rejected := 0
	for index, step := range view.Steps {
		if step.Index != index || strings.TrimSpace(step.UnitID) == "" || step.BeforeDigest != current ||
			!validDigest(step.CandidateDigest) || step.CandidateDigest == step.BeforeDigest ||
			!validDigest(step.AfterDigest) || !validDigest(step.ObservationDigest) {
			return errors.New("evidence explorer stress reduction trace is invalid")
		}
		switch step.Decision {
		case "accepted":
			if !step.RelationRevalidated || !step.PrivacyRevalidated || !step.ViolationPreserved ||
				step.AfterDigest != step.CandidateDigest {
				return errors.New("evidence explorer accepted stress reduction lacks its proofs")
			}
			accepted++
		case "rejected":
			if step.RelationRevalidated && step.PrivacyRevalidated && step.ViolationPreserved ||
				step.AfterDigest != step.BeforeDigest {
				return errors.New("evidence explorer rejected stress reduction changed the witness")
			}
			rejected++
		default:
			return errors.New("evidence explorer stress reduction decision is invalid")
		}
		current = step.AfterDigest
	}
	if current != view.ReducedInputDigest || accepted != view.AcceptedReductions || rejected != view.RejectedReductions {
		return errors.New("evidence explorer stress reduction summary differs from its trace")
	}
	seenTrajectories := make(map[string]struct{}, len(view.FinalWitness))
	for _, line := range view.FinalWitness {
		if !slices.Contains(view.OriginalOrder, line.TrajectoryID) || strings.TrimSpace(line.UnitID) == "" ||
			!validReleasePath(line.FixturePath) || line.Line < 1 || strings.TrimSpace(line.Content) == "" {
			return errors.New("evidence explorer stress final witness is invalid")
		}
		seenTrajectories[line.TrajectoryID] = struct{}{}
	}
	if len(seenTrajectories) != len(view.OriginalOrder) {
		return errors.New("evidence explorer stress final witness omits a candidate")
	}
	return nil
}
