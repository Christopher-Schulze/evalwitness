package capsule

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func privateRelationRegistry(base *Registry) (*Registry, error) {
	if base == nil {
		return nil, errors.New("private relation registry requires a base registry")
	}
	document, err := SealRegistry(PrivateRelationRegistryID, base.Digest(), privateRelationTypes())
	if err != nil {
		return nil, err
	}
	validators := map[string]PayloadValidator{
		privateRelationFileValidatorID: func(payload []byte) error {
			if len(payload) == 0 {
				return errors.New("private relation package file is empty")
			}
			return nil
		},
		privateRelationReceiptValidatorID: func(payload []byte) error {
			var receipt PrivateRelationPackageReceipt
			if err := protocol.DecodeStrict(payload, &receipt); err != nil {
				return err
			}
			return receipt.Validate()
		},
		privateRelationEventChainValidatorID: func(payload []byte) error {
			var chain PrivateRelationEventChain
			if err := protocol.DecodeStrict(payload, &chain); err != nil {
				return err
			}
			if err := chain.Validate(); err != nil {
				return err
			}
			_, err := decodePrivateRelationEvents(chain)
			return err
		},
		privateRelationCommitmentValidatorID: func(payload []byte) error {
			var commitment PrivateRelationSourceCommitment
			if err := protocol.DecodeStrict(payload, &commitment); err != nil {
				return err
			}
			return commitment.Validate()
		},
		privateRelationSessionValidatorID: func(payload []byte) error {
			_, err := relation.DecodePilotInspectionSession(bytes.NewReader(payload))
			return err
		},
		privateRelationInspectionValidatorID: func(payload []byte) error {
			record, err := relation.DecodePilotInspectionRecord(bytes.NewReader(payload))
			if err != nil {
				return err
			}
			if record.SchemaVersion != relation.PilotInspectionSchemaVersionV3 {
				return errors.New("private relation inspection is not protocol-v3 evidence")
			}
			return nil
		},
		privateRelationCompletionValidatorID: func(payload []byte) error {
			_, err := relation.DecodePilotInspectionCompletion(bytes.NewReader(payload))
			return err
		},
		privateRelationProofValidatorID: func(payload []byte) error {
			var proof PrivateRelationProof
			if err := protocol.DecodeStrict(payload, &proof); err != nil {
				return err
			}
			return proof.Validate()
		},
	}
	return ExtendRegistryWithBindings(
		base,
		document,
		validators,
		map[string]BindingValidator{privateRelationBindingValidatorID: validatePrivateRelationBindings},
	)
}

func PrivateRelationRegistry(base *Registry) (*Registry, error) {
	return privateRelationRegistry(base)
}

func validatePrivateRelationBindings(context BindingContext) error {
	switch context.Component.TypeID {
	case PrivateRelationPackageReceiptSchemaVersion:
		return validatePrivateRelationReceiptBindings(context)
	case relation.PilotInspectionSessionSchemaVersion:
		return validatePrivateRelationSessionBindings(context)
	case PrivateRelationEventChainSchemaVersion:
		return validatePrivateRelationEventChainBindings(context)
	case relation.PilotInspectionSchemaVersionV3:
		return validatePrivateRelationInspectionBindings(context)
	case relation.PilotInspectionCompletionSchemaVersion:
		return validatePrivateRelationCompletionBindings(context)
	case PrivateRelationSourceCommitmentSchemaVersion:
		return validatePrivateRelationCommitmentBindings(context)
	case PrivateRelationProofSchemaVersion:
		return validatePrivateRelationProofBindings(context)
	default:
		return fmt.Errorf("unsupported private relation binding type %q", context.Component.TypeID)
	}
}

func validatePrivateRelationReceiptBindings(context BindingContext) error {
	var receipt PrivateRelationPackageReceipt
	if err := protocol.DecodeStrict(context.Payload, &receipt); err != nil {
		return err
	}
	inventoryParent, err := uniqueReferenceParent(context, referencePrivateRelationInventoryType)
	if err != nil {
		return err
	}
	inventory, err := relation.DecodePilotPackageInventory(bytes.NewReader(inventoryParent.Payload))
	if err != nil {
		return err
	}
	if receipt.InventoryComponentID != inventoryParent.Record.ComponentID || receipt.PackageInventoryDigest != inventory.Digest ||
		receipt.PayloadFiles != inventory.PayloadFiles || receipt.PayloadBytes != inventory.PayloadBytes {
		return errors.New("private relation receipt differs from its inventory parent")
	}
	commitment, err := uniqueReferenceParent(context, referencePrivateCommitmentType)
	if err != nil {
		return err
	}
	if commitment.Reference.Resolution != ParentExternal || receipt.PublicCapsuleID != commitment.Reference.CapsuleID ||
		receipt.PublicCommitmentComponentID != commitment.Record.ComponentID {
		return errors.New("private relation receipt differs from its public commitment parent")
	}
	files, err := privateRelationFilesFromParents(receipt, context.Parents)
	if err != nil {
		return err
	}
	return verifyPrivateRelationInventoryPayloads(inventory, files)
}

func validatePrivateRelationSessionBindings(context BindingContext) error {
	session, err := relation.DecodePilotInspectionSession(bytes.NewReader(context.Payload))
	if err != nil {
		return err
	}
	parent, err := uniqueReferenceParent(context, referencePrivateRelationInventoryType)
	if err != nil {
		return err
	}
	inventory, err := relation.DecodePilotPackageInventory(bytes.NewReader(parent.Payload))
	if err != nil {
		return err
	}
	if session.Package.PackageFormat != inventory.PackageFormat || session.Package.PackageInventoryDigest != inventory.Digest {
		return errors.New("private relation session differs from its exact package inventory")
	}
	return nil
}

func validatePrivateRelationEventChainBindings(context BindingContext) error {
	chain, events, session, status, err := privateRelationJournalParents(context)
	if err != nil {
		return err
	}
	expected, err := buildPrivateRelationEventChain(session, privateRelationEventBytes(chain))
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(chain, expected) || len(events) != status.Events {
		return errors.New("private relation event-chain summary differs from its session-bound events")
	}
	return nil
}

func validatePrivateRelationInspectionBindings(context BindingContext) error {
	record, err := relation.DecodePilotInspectionRecord(bytes.NewReader(context.Payload))
	if err != nil {
		return err
	}
	chain, events, session, status, err := privateRelationJournalParents(context)
	if err != nil {
		return err
	}
	if len(events) != chain.EventCount || record.InspectorAlias != session.InspectorAlias ||
		record.ReadinessDigest != session.ReadinessDigest || record.BundleDigest != session.BundleDigest ||
		record.MappingCommitmentDigest != session.MappingCommitment || record.Packets != len(session.Packets) ||
		len(record.Decisions) != len(status.DecisionSummaries) {
		return errors.New("private relation inspection differs from its session or journal parents")
	}
	for index, summary := range status.DecisionSummaries {
		decision := record.Decisions[index]
		if decision.PacketID != summary.PacketID || decision.Disposition != summary.Disposition || !slices.Equal(decision.ReasonCodes, summary.ReasonCodes) {
			return errors.New("private relation inspection decisions differ from the journal-derived summaries")
		}
	}
	return nil
}

func validatePrivateRelationCompletionBindings(context BindingContext) error {
	completion, err := relation.DecodePilotInspectionCompletion(bytes.NewReader(context.Payload))
	if err != nil {
		return err
	}
	recordParent, err := uniqueReferenceParent(context, relation.PilotInspectionSchemaVersionV3)
	if err != nil {
		return err
	}
	record, err := relation.DecodePilotInspectionRecord(bytes.NewReader(recordParent.Payload))
	if err != nil {
		return err
	}
	receiptParent, err := uniqueReferenceParent(context, PrivateRelationPackageReceiptSchemaVersion)
	if err != nil {
		return err
	}
	var receipt PrivateRelationPackageReceipt
	if err := protocol.DecodeStrict(receiptParent.Payload, &receipt); err != nil {
		return err
	}
	chain, _, session, status, err := privateRelationJournalParents(context)
	if err != nil {
		return err
	}
	if completion.SessionDigest != session.Digest || completion.PackageInventoryDigest != receipt.PackageInventoryDigest ||
		completion.EventCount != chain.EventCount || completion.HeadDigest != chain.HeadDigest ||
		completion.RequiredAssessments != chain.RequiredAssessments || completion.InspectionRecordDigest != record.Digest ||
		completion.CoreStatus != record.OverallStatus || !reflect.DeepEqual(completion.DecisionSummaries, status.DecisionSummaries) ||
		!reflect.DeepEqual(completion.ScarcitySummaries, status.ScarcitySummaries) || status.ScarcityBoundary == nil ||
		completion.ScarcityBoundary != *status.ScarcityBoundary {
		return errors.New("private relation completion differs from its package, journal, or inspection parents")
	}
	return nil
}

func validatePrivateRelationCommitmentBindings(context BindingContext) error {
	var commitment PrivateRelationSourceCommitment
	if err := protocol.DecodeStrict(context.Payload, &commitment); err != nil {
		return err
	}
	parent, err := uniqueReferenceParent(context, PrivateRelationPackageFileSchemaVersion)
	if err != nil {
		return err
	}
	if commitment.SourceComponentID != parent.Record.ComponentID || commitment.SourceSHA256 != parent.Record.Payload.Digest ||
		protocol.DigestBytes(parent.Payload) != commitment.SourceSHA256 {
		return errors.New("private relation source commitment differs from its exact package-file parent")
	}
	return nil
}

func validatePrivateRelationProofBindings(context BindingContext) error {
	var proof PrivateRelationProof
	if err := protocol.DecodeStrict(context.Payload, &proof); err != nil {
		return err
	}
	receiptParent, err := uniqueReferenceParent(context, PrivateRelationPackageReceiptSchemaVersion)
	if err != nil {
		return err
	}
	var receipt PrivateRelationPackageReceipt
	if err := protocol.DecodeStrict(receiptParent.Payload, &receipt); err != nil {
		return err
	}
	inventoryParent, err := uniqueReferenceParent(context, referencePrivateRelationInventoryType)
	if err != nil {
		return err
	}
	inventory, err := relation.DecodePilotPackageInventory(bytes.NewReader(inventoryParent.Payload))
	if err != nil {
		return err
	}
	files, err := privateRelationFilesFromParents(receipt, context.Parents)
	if err != nil {
		return err
	}
	if err := verifyPrivateRelationInventoryPayloads(inventory, files); err != nil {
		return err
	}
	sessionParent, err := uniqueReferenceParent(context, relation.PilotInspectionSessionSchemaVersion)
	if err != nil {
		return err
	}
	eventParent, err := uniqueReferenceParent(context, PrivateRelationEventChainSchemaVersion)
	if err != nil {
		return err
	}
	recordParent, err := uniqueReferenceParent(context, relation.PilotInspectionSchemaVersionV3)
	if err != nil {
		return err
	}
	completionParent, err := uniqueReferenceParent(context, relation.PilotInspectionCompletionSchemaVersion)
	if err != nil {
		return err
	}
	chain, eventChain, err := decodePrivateRelationChain(
		inventory,
		files,
		sessionParent.Payload,
		eventParent.Payload,
		recordParent.Payload,
		completionParent.Payload,
	)
	if err != nil {
		return err
	}
	attestation, err := relation.BuildOwnerInspectionPublicAttestation(chain)
	if err != nil {
		return err
	}
	publicParent, err := uniqueReferenceParent(context, relation.OwnerInspectionPublicAttestationSchemaVersion)
	if err != nil {
		return err
	}
	expected, err := sealPrivateRelationProof(
		receipt,
		eventChain,
		chain,
		publicParent.Reference.CapsuleID,
		receipt.PublicCommitmentComponentID,
		publicParent.Record.ComponentID,
		attestation.Digest,
	)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(proof, expected) || proof.PublicCapsuleID != receipt.PublicCapsuleID {
		return errors.New("private relation proof differs from deterministic package, chain, or public projection reproduction")
	}
	return nil
}

func privateRelationJournalParents(context BindingContext) (PrivateRelationEventChain, []relation.PilotInspectionEvent, relation.PilotInspectionSession, relation.PilotInspectionJournalStatus, error) {
	eventParent := BoundParent{Payload: context.Payload, Record: context.Component}
	if context.Component.TypeID != PrivateRelationEventChainSchemaVersion {
		var err error
		eventParent, err = uniqueReferenceParent(context, PrivateRelationEventChainSchemaVersion)
		if err != nil {
			return PrivateRelationEventChain{}, nil, relation.PilotInspectionSession{}, relation.PilotInspectionJournalStatus{}, err
		}
	}
	var chain PrivateRelationEventChain
	if err := protocol.DecodeStrict(eventParent.Payload, &chain); err != nil {
		return PrivateRelationEventChain{}, nil, relation.PilotInspectionSession{}, relation.PilotInspectionJournalStatus{}, err
	}
	sessionParent, err := uniqueReferenceParent(context, relation.PilotInspectionSessionSchemaVersion)
	if err != nil {
		return PrivateRelationEventChain{}, nil, relation.PilotInspectionSession{}, relation.PilotInspectionJournalStatus{}, err
	}
	session, err := relation.DecodePilotInspectionSession(bytes.NewReader(sessionParent.Payload))
	if err != nil {
		return PrivateRelationEventChain{}, nil, relation.PilotInspectionSession{}, relation.PilotInspectionJournalStatus{}, err
	}
	events, err := decodePrivateRelationEvents(chain)
	if err != nil {
		return PrivateRelationEventChain{}, nil, relation.PilotInspectionSession{}, relation.PilotInspectionJournalStatus{}, err
	}
	status, err := relation.VerifyPilotInspectionJournal(session, events)
	if err != nil {
		return PrivateRelationEventChain{}, nil, relation.PilotInspectionSession{}, relation.PilotInspectionJournalStatus{}, err
	}
	return chain, events, session, status, nil
}

func decodePrivateRelationEvents(chain PrivateRelationEventChain) ([]relation.PilotInspectionEvent, error) {
	events := make([]relation.PilotInspectionEvent, len(chain.Events))
	for index, entry := range chain.Events {
		raw, err := base64.StdEncoding.DecodeString(entry.PayloadBase64)
		if err != nil || base64.StdEncoding.EncodeToString(raw) != entry.PayloadBase64 || int64(len(raw)) != entry.Bytes || protocol.DigestBytes(raw) != entry.SHA256 {
			return nil, fmt.Errorf("private relation event payload %d has invalid exact-byte encoding", index+1)
		}
		event, err := relation.DecodePilotInspectionEvent(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		if event.Sequence != entry.Sequence {
			return nil, errors.New("private relation event payload sequence differs from its wrapper")
		}
		events[index] = event
	}
	return events, nil
}

func privateRelationEventBytes(chain PrivateRelationEventChain) [][]byte {
	payloads := make([][]byte, len(chain.Events))
	for index, entry := range chain.Events {
		payloads[index], _ = base64.StdEncoding.DecodeString(entry.PayloadBase64)
	}
	return payloads
}

func privateRelationFilesFromParents(receipt PrivateRelationPackageReceipt, parents []BoundParent) (map[string][]byte, error) {
	byID := make(map[string]BoundParent, privateRelationPackageFiles)
	for _, parent := range parents {
		if parent.Record.TypeID == PrivateRelationPackageFileSchemaVersion {
			byID[parent.Record.ComponentID] = parent
		}
	}
	if len(byID) != len(receipt.Files) {
		return nil, errors.New("private relation package-file parent count differs from its receipt")
	}
	files := make(map[string][]byte, len(receipt.Files))
	for _, binding := range receipt.Files {
		parent, found := byID[binding.ComponentID]
		if !found || parent.Record.Payload.Digest != binding.PayloadSHA256 || parent.Record.Payload.Bytes != binding.Bytes ||
			protocol.DigestBytes(parent.Payload) != binding.SHA256 || int64(len(parent.Payload)) != binding.Bytes {
			return nil, fmt.Errorf("private relation package-file parent %q differs from its receipt", binding.Path)
		}
		files[binding.Path] = slices.Clone(parent.Payload)
	}
	return files, nil
}

func verifyPrivateRelationInventoryPayloads(inventory relation.PilotPackageInventory, files map[string][]byte) error {
	if inventory.PayloadFiles != privateRelationPackageFiles || len(files) != inventory.PayloadFiles {
		return errors.New("private relation inventory and package-file set have different counts")
	}
	for _, file := range inventory.Files {
		raw, found := files[file.Path]
		if !found || file.Mode != "0600" || int64(len(raw)) != file.Bytes || protocol.DigestBytes(raw) != file.SHA256 {
			return fmt.Errorf("private relation inventory file %q is missing or changed", file.Path)
		}
	}
	return nil
}

func decodePrivateRelationChain(
	inventory relation.PilotPackageInventory,
	files map[string][]byte,
	sessionRaw []byte,
	eventChainRaw []byte,
	recordRaw []byte,
	completionRaw []byte,
) (relation.OwnerInspectionPrivateChain, PrivateRelationEventChain, error) {
	plan, err := relation.DecodePlanV3(bytes.NewReader(files["relation-audit-plan.json"]))
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	primary, err := relation.DecodePrimarySampleV3(bytes.NewReader(files["relation-primary-sample.json"]), plan)
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	sentinel, err := relation.DecodeScarcitySentinelV3(bytes.NewReader(files["relation-scarcity-sentinel.json"]), plan, primary)
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	pilot, err := relation.DecodePilotSampleV3(bytes.NewReader(files["relation-pilot-sample.json"]), plan, primary, sentinel)
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	materials := make([]relation.CaseMaterial, 3)
	for index := range materials {
		materials[index], err = relation.DecodeCaseMaterial(bytes.NewReader(files[fmt.Sprintf("sentinel-materials/%02d.json", index+1)]))
		if err != nil {
			return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
		}
	}
	if err := relation.VerifyScarcityInspectionMaterials(plan, primary, sentinel, materials); err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	readiness, err := relation.DecodeRelationPilotReadiness(bytes.NewReader(files["pilot-readiness.json"]))
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	bundle, err := relation.DecodeReviewBundle(bytes.NewReader(files["review-bundle.json"]))
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	mappings, err := relation.DecodePrivateMappings(bytes.NewReader(files["private-mappings.json"]))
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	if err := relation.VerifyRelationPilotReadiness(readiness, bundle, mappings); err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	fileByPath := make(map[string]relation.PilotPackageInventoryFile, len(inventory.Files))
	for _, file := range inventory.Files {
		fileByPath[file.Path] = file
	}
	required := []string{
		"pilot-readiness.json", "review-bundle.json", "private-mappings.json", "owner-inspection.md",
		"owner-change-atlas.md", "relation-scarcity-sentinel.json", "owner-scarcity-inspection.md",
	}
	for _, name := range required {
		if _, found := fileByPath[name]; !found {
			return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, fmt.Errorf("private relation inventory omits required package parent %q", name)
		}
	}
	binding := relation.PilotInspectionPackageBinding{
		PackageFormat: inventory.PackageFormat, PackageInventoryDigest: inventory.Digest,
		ReadinessSHA256:        fileByPath["pilot-readiness.json"].SHA256,
		BundleSHA256:           fileByPath["review-bundle.json"].SHA256,
		MappingsSHA256:         fileByPath["private-mappings.json"].SHA256,
		WorkbookSHA256:         fileByPath["owner-inspection.md"].SHA256,
		ChangeAtlasSHA256:      fileByPath["owner-change-atlas.md"].SHA256,
		ScarcitySentinelSHA256: fileByPath["relation-scarcity-sentinel.json"].SHA256,
		ScarcityAppendixSHA256: fileByPath["owner-scarcity-inspection.md"].SHA256,
	}
	session, err := relation.DecodePilotInspectionSession(bytes.NewReader(sessionRaw))
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	var eventChain PrivateRelationEventChain
	if err := protocol.DecodeStrict(eventChainRaw, &eventChain); err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	events, err := decodePrivateRelationEvents(eventChain)
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	record, err := relation.DecodePilotInspectionRecord(bytes.NewReader(recordRaw))
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	completion, err := relation.DecodePilotInspectionCompletion(bytes.NewReader(completionRaw))
	if err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	chain := relation.OwnerInspectionPrivateChain{
		Completion: completion, Record: record, Session: session, Events: events,
		Readiness: readiness, Bundle: bundle, Mappings: mappings, Plan: plan, Primary: primary,
		Sentinel: sentinel, Pilot: pilot, ScarcityMaterials: materials, PackageBinding: binding,
	}
	if _, err := relation.BuildOwnerInspectionPublicAttestation(chain); err != nil {
		return relation.OwnerInspectionPrivateChain{}, PrivateRelationEventChain{}, err
	}
	return chain, eventChain, nil
}
