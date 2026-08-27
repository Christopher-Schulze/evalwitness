package mutation

import (
	"errors"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func ApplyCandidateOrderReversal(left, right preprocess.Trajectory, request ApplyRequest) (ApplyResult, error) {
	if request.Family != FamilyCandidateOrderReversal {
		return ApplyResult{}, errors.New("pair-order mutation requires candidate_order_reversal family")
	}
	if err := left.Validate(); err != nil {
		return ApplyResult{}, err
	}
	if err := right.Validate(); err != nil {
		return ApplyResult{}, err
	}
	if left.Digest == right.Digest || left.SourceFormat != right.SourceFormat {
		return ApplyResult{}, errors.New("pair-order mutation requires distinct trajectories in one source format")
	}
	if missing(request.CorpusVersion, request.TaskID, request.RepositoryID, request.SourceFamily, request.SourceLocation,
		request.SourceRevision, request.SplitGroupID, request.Seed, request.ReviewSamplingStratum) {
		return ApplyResult{}, errors.New("pair-order mutation request is incomplete")
	}
	definition, _ := DefinitionFor(request.Family)
	originalPairDigest, err := digestJSON([]string{left.Digest, right.Digest})
	if err != nil {
		return ApplyResult{}, err
	}
	mutatedPairDigest, err := digestJSON([]string{right.Digest, left.Digest})
	if err != nil {
		return ApplyResult{}, err
	}
	qualityDigests, evidenceDigests, causalDigests := make([]string, 2), make([]string, 2), make([]string, 2)
	for index, trajectory := range []preprocess.Trajectory{left, right} {
		qualityDigests[index], err = projectionDigest(trajectory, definition, false)
		if err != nil {
			return ApplyResult{}, err
		}
		evidenceDigests[index], err = projectionDigest(trajectory, definition, true)
		if err != nil {
			return ApplyResult{}, err
		}
		causalDigests[index], err = causalGraphDigest(trajectory)
		if err != nil {
			return ApplyResult{}, err
		}
	}
	sort.Strings(qualityDigests)
	sort.Strings(evidenceDigests)
	sort.Strings(causalDigests)
	qualitySet, _ := digestJSON(qualityDigests)
	evidenceSet, _ := digestJSON(evidenceDigests)
	causalSet, _ := digestJSON(causalDigests)
	preservation := PreservationRecord{
		BoundaryVersion:         EvidenceBoundaryVersion,
		QualityProjectionBefore: qualitySet, QualityProjectionAfter: qualitySet,
		EvidenceProjectionBefore: evidenceSet, EvidenceProjectionAfter: evidenceSet,
		CausalGraphBefore: causalSet, CausalGraphAfter: causalSet,
		ChangedGroups: []string{"presentation"}, PreservedGroups: []string{"causal_graph", "evidence_semantics", "semantic_quality"}, AmbiguityReasons: []string{},
	}
	witness, err := SealWitness(Witness{
		ValidatorID: request.Validator.ID, ValidatorVersion: request.Validator.Version,
		Relation: definition.Relation, LabelState: LabelProven,
		Checks: []Check{
			{Name: "candidate_multiset_preserved", Expected: "equal", Observed: "equal", Passed: true},
			{Name: "candidate_order_reversed", Expected: right.Digest + "," + left.Digest, Observed: right.Digest + "," + left.Digest, Passed: true},
		},
	})
	if err != nil {
		return ApplyResult{}, err
	}
	packet, err := SealBlindReviewPacket(BlindReviewPacket{
		MutationMaterialDigest: digestText(originalPairDigest + "\x00" + mutatedPairDigest),
		TaskAlias:              "task-" + digestText(request.TaskID)[:16], SourceFormat: left.SourceFormat,
		OriginalDigest: originalPairDigest, MutatedDigest: mutatedPairDigest,
		ReviewQuestions: []string{
			"Did candidate order change any judgment-relevant content?",
			"Is the pair semantically identical aside from presentation order?",
		},
	})
	if err != nil {
		return ApplyResult{}, err
	}
	combinedSourceDigest, _ := digestJSON([]string{left.SourceDigest, right.SourceDigest})
	manifest, err := SealManifest(Manifest{
		CorpusVersion: request.CorpusVersion,
		Source: SourceRef{
			TaskID: request.TaskID, RepositoryID: request.RepositoryID, SourceFamily: request.SourceFamily,
			SourceFormat: left.SourceFormat, SourceLocation: request.SourceLocation, SourceRevision: request.SourceRevision,
			SourceDigest: combinedSourceDigest, TrajectoryDigest: originalPairDigest,
			PairedTrajectoryDigests: []string{left.Digest, right.Digest}, Outcome: request.Outcome,
		},
		Program: Program{
			Version: MutationProgramVersion, Family: request.Family, Seed: request.Seed, Operator: definition.Operator,
			Parameters: []NamedValue{{Name: "left_digest", Value: left.Digest}, {Name: "right_digest", Value: right.Digest}},
		},
		Class: definition.Class, ExpectedRelation: definition.Relation, Affected: AffectedSurface{EventIDs: []string{}, FieldPaths: []preprocess.FieldPath{}, FileAliases: []string{}},
		Validator: request.Validator, Preservation: preservation, Witness: witness,
		License: request.License, Privacy: request.Privacy, SplitGroupID: request.SplitGroupID,
		OriginalTrajectoryDigest: originalPairDigest, MutatedTrajectoryDigest: mutatedPairDigest,
		Review: ReviewState{Required: request.ReviewSampled, SamplingStratum: request.ReviewSamplingStratum, BlindPacketDigest: packet.Digest},
	})
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{Manifest: manifest, Packet: packet, PairOrder: [2]string{right.Digest, left.Digest}}, nil
}
