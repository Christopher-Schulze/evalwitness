package claim

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func EncodeLedger(ledger Ledger) ([]byte, error) {
	if err := ledger.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(ledger)
}

func DecodeLedger(raw []byte) (Ledger, error) {
	var ledger Ledger
	if err := protocol.DecodeStrict(raw, &ledger); err != nil {
		return Ledger{}, fmt.Errorf("decode claim ledger: %w", err)
	}
	canonical, err := protocol.CanonicalMarshal(ledger)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Ledger{}, errors.New("claim ledger is not canonical JSON")
	}
	if err := ledger.Validate(); err != nil {
		return Ledger{}, err
	}
	return ledger, nil
}

func EncodeProjection(projection Projection) ([]byte, error) {
	if err := projection.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(projection)
}

func DecodeProjection(raw []byte) (Projection, error) {
	var projection Projection
	if err := protocol.DecodeStrict(raw, &projection); err != nil {
		return Projection{}, fmt.Errorf("decode claim projection: %w", err)
	}
	canonical, err := protocol.CanonicalMarshal(projection)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Projection{}, errors.New("claim projection is not canonical JSON")
	}
	if err := projection.Validate(); err != nil {
		return Projection{}, err
	}
	return projection, nil
}

func EncodeAutopsy(autopsy Autopsy) ([]byte, error) {
	if err := autopsy.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(autopsy)
}

func DecodeAutopsy(raw []byte) (Autopsy, error) {
	var autopsy Autopsy
	if err := protocol.DecodeStrict(raw, &autopsy); err != nil {
		return Autopsy{}, fmt.Errorf("decode claim autopsy: %w", err)
	}
	canonical, err := protocol.CanonicalMarshal(autopsy)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Autopsy{}, errors.New("claim autopsy is not canonical JSON")
	}
	if err := autopsy.Validate(); err != nil {
		return Autopsy{}, err
	}
	return autopsy, nil
}

func EncodeChallengeReceipt(receipt ChallengeReceipt) ([]byte, error) {
	if err := receipt.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(receipt)
}
