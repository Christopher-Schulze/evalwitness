package outcome

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
)

const maximumBlindingKeyFileBytes = 256

func DecodeBlindingKey(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumBlindingKeyFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read blinding key: %w", err)
	}
	if len(raw) > maximumBlindingKeyFileBytes {
		return nil, errors.New("blinding key file exceeds 256-byte limit")
	}
	encoded := bytes.TrimSpace(raw)
	if len(encoded) != 64 || !bytes.Equal(encoded, bytes.ToLower(encoded)) {
		return nil, errors.New("blinding key must be exactly 64 lowercase hexadecimal characters")
	}
	key := make([]byte, 32)
	if _, err := hex.Decode(key, encoded); err != nil {
		return nil, errors.New("blinding key must be exactly 64 lowercase hexadecimal characters")
	}
	return key, nil
}

func BuildBlindedPacketFromRequest(request BlindBuildRequest, key []byte) (BlindPacket, PrivateMapping, error) {
	if err := request.Validate(); err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	packet, mapping, err := BuildBlindedPacket(BlindPacket{
		PlanDigest: request.PlanDigest, TaskAlias: request.TaskAlias, Evidence: request.Evidence,
		RubricQuestions: request.RubricQuestions, PrivacyClass: request.PrivacyClass, PublicReleasable: request.PublicReleasable,
	}, request.SourceCaseDigest, request.Condition, request.ExpectedRelation, request.BlindingKeyID, key, request.SlotMappings)
	if err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	forbidden := append([]string(nil), request.ForbiddenValues...)
	forbidden = append(forbidden, request.TaskAlias, request.Condition, request.ExpectedRelation, request.BlindingKeyID, request.SourceCaseDigest)
	if err := ValidatePacketLeakage(packet, forbidden); err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	return packet, mapping, nil
}

func (request BlindBuildRequest) Validate() error {
	if request.SchemaVersion != BlindBuildSchemaVersion || !validDigest(request.PlanDigest) || !validDigest(request.SourceCaseDigest) ||
		missing(request.TaskAlias, request.PrivacyClass, request.Condition, request.ExpectedRelation, request.BlindingKeyID) ||
		len(request.Evidence) == 0 || len(request.RubricQuestions) == 0 || len(request.SlotMappings) == 0 || len(request.ForbiddenValues) == 0 {
		return errors.New("blind build request identity, evidence, rubric, source, or private mapping is invalid")
	}
	packet := BlindPacket{
		PlanDigest: request.PlanDigest, TaskAlias: request.TaskAlias, Evidence: append([]PacketEvidence(nil), request.Evidence...),
		RubricQuestions: append([]string(nil), request.RubricQuestions...), PrivacyClass: request.PrivacyClass, PublicReleasable: request.PublicReleasable,
	}
	slots := append([]SlotMapping(nil), request.SlotMappings...)
	sort.Slice(packet.Evidence, func(left, right int) bool { return packet.Evidence[left].Slot < packet.Evidence[right].Slot })
	sort.Strings(packet.RubricQuestions)
	sort.Slice(slots, func(left, right int) bool { return slots[left].Slot < slots[right].Slot })
	if len(slots) != len(packet.Evidence) {
		return errors.New("blind build request private slots do not cover every public evidence slot")
	}
	for index, item := range packet.Evidence {
		if missing(item.Slot, item.Kind, item.License, item.Limitation) || !validDigest(item.ContentDigest) || item.Content == "" && request.PublicReleasable {
			return fmt.Errorf("blind build request evidence %d is incomplete", index)
		}
		if index > 0 && packet.Evidence[index-1].Slot >= item.Slot || slots[index].Slot != item.Slot || !validDigest(slots[index].SourceDigest) {
			return errors.New("blind build request public and private slots must be valid, unique, and identical")
		}
		if item.Content != "" && digestText(item.Content) != item.ContentDigest {
			return fmt.Errorf("blind build request evidence %d content digest is invalid", index)
		}
	}
	if err := uniqueSorted("blind build request rubric questions", packet.RubricQuestions); err != nil {
		return err
	}
	return uniqueSorted("blind build request forbidden values", request.ForbiddenValues)
}

func BuildBlindedPacket(packet BlindPacket, sourceCaseDigest, condition, expectedRelation, blindingKeyID string, key []byte, slots []SlotMapping) (BlindPacket, PrivateMapping, error) {
	if !validDigest(sourceCaseDigest) || missing(condition, expectedRelation, blindingKeyID) || len(key) < 32 {
		return BlindPacket{}, PrivateMapping{}, errors.New("blind packet requires source identity, private condition/relation, key ID, and at least 32 key bytes")
	}
	packet.Evidence = append([]PacketEvidence(nil), packet.Evidence...)
	packet.RubricQuestions = append([]string(nil), packet.RubricQuestions...)
	slots = append([]SlotMapping(nil), slots...)
	sourceTaskAlias := packet.TaskAlias
	sort.Slice(packet.Evidence, func(left, right int) bool { return packet.Evidence[left].Slot < packet.Evidence[right].Slot })
	sort.Strings(packet.RubricQuestions)
	sort.Slice(slots, func(left, right int) bool { return slots[left].Slot < slots[right].Slot })
	if len(slots) != len(packet.Evidence) {
		return BlindPacket{}, PrivateMapping{}, errors.New("blind packet private slots do not cover every public evidence slot")
	}
	for index := range slots {
		if slots[index].Slot != packet.Evidence[index].Slot {
			return BlindPacket{}, PrivateMapping{}, errors.New("blind packet public and private evidence slots differ")
		}
	}
	for index := range packet.Evidence {
		slotDigest := keyedDigest(key, "slot", packet.PlanDigest, sourceCaseDigest, blindingKeyID, slots[index].Slot, slots[index].SourceDigest)
		packet.Evidence[index].Slot = "slot-" + slotDigest
		slots[index].Slot = packet.Evidence[index].Slot
	}
	packet.TaskAlias = "taskref-" + keyedDigest(key, "task", packet.PlanDigest, sourceCaseDigest, blindingKeyID, sourceTaskAlias)
	sort.Slice(packet.Evidence, func(left, right int) bool { return packet.Evidence[left].Slot < packet.Evidence[right].Slot })
	sort.Slice(slots, func(left, right int) bool { return slots[left].Slot < slots[right].Slot })
	opaquePacketID := "packet-" + keyedDigest(key, "packet", packet.PlanDigest, sourceCaseDigest, blindingKeyID)
	sealedPacket, err := SealBlindPacket(packet, opaquePacketID)
	if err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	mapping, err := SealPrivateMapping(PrivateMapping{
		PacketID: opaquePacketID, SourceTaskAlias: sourceTaskAlias, SourceCaseDigest: sourceCaseDigest, Condition: condition, ExpectedRelation: expectedRelation,
		SlotMappings: slots, BlindingKeyID: blindingKeyID,
	})
	if err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	return sealedPacket, mapping, nil
}

func keyedDigest(key []byte, values ...string) string {
	mac := hmac.New(sha256.New, key)
	for index, value := range values {
		if index > 0 {
			_, _ = mac.Write([]byte{0})
		}
		_, _ = mac.Write([]byte(value))
	}
	return hex.EncodeToString(mac.Sum(nil))
}
