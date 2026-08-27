package outcome

import (
	"fmt"
	"io"
)

func SealRecordDraft(draft RecordDraft) (Record, error) {
	return SealRecord(Record{
		TaskAlias: draft.TaskAlias, Revision: draft.Revision, ParentDigest: draft.ParentDigest,
		Evidence: append([]Evidence(nil), draft.Evidence...), Resolution: draft.Resolution,
		ResolutionBasis: append([]string(nil), draft.ResolutionBasis...), Limitations: append([]string(nil), draft.Limitations...),
		AuthorID: draft.AuthorID, RevisionReason: draft.RevisionReason,
	})
}

func DecodeRecordDraft(reader io.Reader) (RecordDraft, error) {
	var value RecordDraft
	if err := decodeStrict(reader, &value); err != nil {
		return RecordDraft{}, fmt.Errorf("decode outcome record draft: %w", err)
	}
	record, err := SealRecordDraft(value)
	if err != nil {
		return RecordDraft{}, err
	}
	return RecordDraft{
		TaskAlias: record.TaskAlias, Revision: record.Revision, ParentDigest: record.ParentDigest,
		Evidence: append([]Evidence(nil), record.Evidence...), Resolution: record.Resolution,
		ResolutionBasis: append([]string(nil), record.ResolutionBasis...), Limitations: append([]string(nil), record.Limitations...),
		AuthorID: record.AuthorID, RevisionReason: record.RevisionReason,
	}, nil
}
