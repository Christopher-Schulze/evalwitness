package mutation

import (
	"errors"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func ApplyCandidateOrderReversalV3(left, right preprocess.Trajectory, request ApplyRequest) (ApplyV3Outcome, error) {
	if request.Family != FamilyCandidateOrderReversal {
		return ApplyV3Outcome{}, errors.New("pair-order v3 mutation requires candidate_order_reversal family")
	}
	legacy, err := ApplyCandidateOrderReversal(left, right, request)
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	checks := append([]Check(nil), legacy.Manifest.Witness.Checks...)
	firewall, err := sealConstructFirewallV3(ConstructFirewallReportV2{
		Family: request.Family, Status: ConstructApplied,
		SourceTrajectoryDigest:  legacy.Manifest.OriginalTrajectoryDigest,
		MutatedTrajectoryDigest: legacy.Manifest.MutatedTrajectoryDigest,
		TargetEventIDs:          []string{}, ProofEventIDs: []string{}, SemanticRole: "candidate_pair_presentation",
		Checks: checks, RejectionReasons: []ConstructRejectionReason{},
	})
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	packet, err := SealBlindReviewPacket(BlindReviewPacket{
		MutationMaterialDigest: digestText(legacy.Manifest.OriginalTrajectoryDigest + "\x00" + legacy.Manifest.MutatedTrajectoryDigest + "\x00" + firewall.Digest),
		TaskAlias:              "task-" + digestText(request.TaskID)[:16], SourceFormat: left.SourceFormat,
		OriginalDigest: legacy.Manifest.OriginalTrajectoryDigest, MutatedDigest: legacy.Manifest.MutatedTrajectoryDigest,
		ReviewQuestions: []string{
			"Did candidate order change any judgment-relevant content?",
			"Does the construct-firewall proof bind the same unordered pair?",
			"Is the pair semantically identical aside from presentation order?",
		},
	})
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	manifest := legacy.Manifest
	definition, _ := DefinitionFor(request.Family)
	manifest.Program.Operator = operatorForProgram(MutationProgramVersionV3, definition)
	manifest.Preservation.BoundaryVersion = EvidenceBoundaryVersionV3
	manifest.Review.BlindPacketDigest = packet.Digest
	manifest.ConstructFirewallDigest = firewall.Digest
	manifest, err = SealManifestV3(manifest)
	if err != nil {
		return ApplyV3Outcome{}, err
	}
	result := ApplyResult{Manifest: manifest, Packet: packet, PairOrder: legacy.PairOrder}
	return ApplyV3Outcome{Status: ConstructApplied, Applied: &result, Firewall: firewall}, nil
}
