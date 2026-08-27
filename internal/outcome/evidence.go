package outcome

import (
	"fmt"
	"io"
)

func SealEvidenceDraft(draft EvidenceDraft) (Evidence, error) {
	return SealEvidence(Evidence{
		ID: draft.ID, Kind: draft.Kind, State: draft.State, ArtifactDigest: draft.ArtifactDigest,
		ValidatorID: draft.ValidatorID, ObservedAt: draft.ObservedAt, Independent: draft.Independent,
		Public: draft.Public, Limitation: draft.Limitation, ParentDigests: append([]string(nil), draft.ParentDigests...),
	})
}

func DecodeEvidenceDraft(reader io.Reader) (EvidenceDraft, error) {
	var value EvidenceDraft
	if err := decodeStrict(reader, &value); err != nil {
		return EvidenceDraft{}, fmt.Errorf("decode outcome evidence draft: %w", err)
	}
	evidence, err := SealEvidenceDraft(value)
	if err != nil {
		return EvidenceDraft{}, err
	}
	return EvidenceDraft{
		ID: evidence.ID, Kind: evidence.Kind, State: evidence.State, ArtifactDigest: evidence.ArtifactDigest,
		ValidatorID: evidence.ValidatorID, ObservedAt: evidence.ObservedAt, Independent: evidence.Independent,
		Public: evidence.Public, Limitation: evidence.Limitation, ParentDigests: append([]string(nil), evidence.ParentDigests...),
	}, nil
}
