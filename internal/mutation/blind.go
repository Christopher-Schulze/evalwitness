package mutation

import (
	"errors"
	"fmt"
)

func SealBlindReviewPacket(packet BlindReviewPacket) (BlindReviewPacket, error) {
	packet.SchemaVersion = BlindPacketSchemaVersion
	packet.PacketID = ""
	packet.Digest = ""
	digest, err := packet.digest()
	if err != nil {
		return BlindReviewPacket{}, err
	}
	packet.Digest = digest
	packet.PacketID = "blind-" + digest
	if err := packet.Validate(); err != nil {
		return BlindReviewPacket{}, err
	}
	return packet, nil
}

func (packet BlindReviewPacket) Validate() error {
	if packet.SchemaVersion != BlindPacketSchemaVersion || missing(packet.PacketID, packet.TaskAlias, string(packet.SourceFormat)) ||
		!validSourceFormat(packet.SourceFormat) || !validDigest(packet.MutationMaterialDigest) || !validDigest(packet.OriginalDigest) || !validDigest(packet.MutatedDigest) ||
		packet.AffectedEventCount < 0 || len(packet.ReviewQuestions) == 0 {
		return errors.New("blind-review packet identity, source, digests, or questions are invalid")
	}
	if err := validateUniqueSortedStrings("blind-review questions", packet.ReviewQuestions); err != nil {
		return err
	}
	expected, err := packet.digest()
	if err != nil {
		return err
	}
	if packet.Digest != expected || packet.PacketID != "blind-"+expected {
		return fmt.Errorf("blind-review packet identity does not match canonical content")
	}
	return nil
}

func (packet BlindReviewPacket) digest() (string, error) {
	packet.PacketID = ""
	packet.Digest = ""
	return digestJSON(packet)
}
