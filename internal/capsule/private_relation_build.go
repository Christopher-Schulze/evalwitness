package capsule

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func BuildPrivateRelationPackage(base ReferencePackage, sources PrivateRelationSources) (PrivateRelationPackage, error) {
	if base.Registry == nil {
		return PrivateRelationPackage{}, errors.New("private relation capsule requires a public reference package")
	}
	basePackage := Package(base)
	if _, err := VerifyPackage(
		context.Background(), base.Registry, base.Manifest, base.Payloads,
		VerificationOptions{MaximumVisibility: VisibilityPublic},
	); err != nil {
		return PrivateRelationPackage{}, fmt.Errorf("verify public reference capsule: %w", err)
	}
	registry, err := privateRelationRegistry(base.Registry)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	inventory, err := relation.DecodePilotPackageInventory(bytes.NewReader(sources.InventoryPayload))
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	if inventory.PayloadFiles != privateRelationPackageFiles || len(sources.PackageFiles) != privateRelationPackageFiles {
		return PrivateRelationPackage{}, fmt.Errorf("private relation package has %d inventory files and %d supplied files, want %d", inventory.PayloadFiles, len(sources.PackageFiles), privateRelationPackageFiles)
	}
	if err := validatePrivateRelationSources(inventory, sources); err != nil {
		return PrivateRelationPackage{}, err
	}
	publicCommitment, err := componentByType(base.Manifest.Components, referencePrivateCommitmentType)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	publicAttestationRecord, err := componentByType(base.Manifest.Components, relation.OwnerInspectionPublicAttestationSchemaVersion)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	var commitment privateOmissionCommitment
	if err := decodeReferenceJSON(base.Payloads[publicCommitment.Payload.Digest], &commitment); err != nil {
		return PrivateRelationPackage{}, err
	}
	if commitment.SubjectDigest != inventory.Digest {
		return PrivateRelationPackage{}, errors.New("private package inventory differs from the public omission commitment")
	}
	publicAttestation, err := relation.DecodeOwnerInspectionPublicAttestation(bytes.NewReader(base.Payloads[publicAttestationRecord.Payload.Digest]))
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	if err := relation.VerifyOwnerInspectionPublicAttestation(publicAttestation, sources.Chain); err != nil {
		return PrivateRelationPackage{}, fmt.Errorf("verify public projection from private chain: %w", err)
	}

	records := make([]ComponentRecord, 0, privateRelationPackageFiles+11)
	payloads := make(map[string][]byte, privateRelationPackageFiles+11)
	fileRecords := make(map[string]ComponentRecord, privateRelationPackageFiles)
	for index, file := range inventory.Files {
		record, normalized, err := BuildComponent(registry, ComponentInput{
			Name:   fmt.Sprintf("relation.private-package-file.%03d", index+1),
			TypeID: PrivateRelationPackageFileSchemaVersion, Visibility: VisibilityPrivate,
			Payload: sources.PackageFiles[file.Path], Parents: nil,
		})
		if err != nil {
			return PrivateRelationPackage{}, fmt.Errorf("build private package file %q: %w", file.Path, err)
		}
		if record.Payload.Digest != file.SHA256 || record.Payload.Bytes != file.Bytes {
			return PrivateRelationPackage{}, fmt.Errorf("private package file %q differs from its inventory", file.Path)
		}
		records = append(records, record)
		payloads[record.Payload.Digest] = normalized
		fileRecords[file.Path] = record
	}
	inventoryRecord, err := addPrivateRelationComponent(registry, ComponentInput{
		Name: "relation.private-package-inventory", TypeID: referencePrivateRelationInventoryType,
		Visibility: VisibilityPrivate, Payload: sources.InventoryPayload,
	}, &records, payloads)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	receipt, receiptRecord, err := buildPrivateRelationReceipt(
		registry, base, inventory, inventoryRecord, fileRecords, publicCommitment, &records, payloads,
	)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	commitments, err := buildPrivateRelationSourceCommitments(registry, fileRecords, &records, payloads)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	sessionRecord, err := addPrivateRelationComponent(registry, ComponentInput{
		Name: "relation.owner-inspection-session", TypeID: relation.PilotInspectionSessionSchemaVersion,
		Visibility: VisibilityPrivate, Payload: sources.SessionPayload,
		Parents: []ParentRef{internalParentRef(EdgeObservedFrom, inventoryRecord)},
	}, &records, payloads)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	eventChain, err := buildPrivateRelationEventChain(sources.Chain.Session, sources.EventPayloads)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	eventRaw, err := protocol.CanonicalMarshal(eventChain)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	eventRecord, err := addPrivateRelationComponent(registry, ComponentInput{
		Name: "relation.owner-inspection-events", TypeID: PrivateRelationEventChainSchemaVersion,
		Visibility: VisibilityPrivate, Payload: eventRaw,
		Parents: []ParentRef{internalParentRef(EdgeObservedFrom, sessionRecord)},
	}, &records, payloads)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	inspectionRecord, err := addPrivateRelationComponent(registry, ComponentInput{
		Name: "relation.owner-inspection-record", TypeID: relation.PilotInspectionSchemaVersionV3,
		Visibility: VisibilityPrivate, Payload: sources.InspectionPayload,
		Parents: []ParentRef{
			internalParentRef(EdgeDerivedFrom, eventRecord),
			internalParentRef(EdgeDerivedFrom, sessionRecord),
		},
	}, &records, payloads)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	completionRecord, err := addPrivateRelationComponent(registry, ComponentInput{
		Name: "relation.owner-inspection-completion", TypeID: relation.PilotInspectionCompletionSchemaVersion,
		Visibility: VisibilityPrivate, Payload: sources.CompletionPayload,
		Parents: []ParentRef{
			internalParentRef(EdgeAttests, eventRecord),
			internalParentRef(EdgeAttests, receiptRecord),
			internalParentRef(EdgeAttests, inspectionRecord),
			internalParentRef(EdgeAttests, sessionRecord),
		},
	}, &records, payloads)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	proof, err := sealPrivateRelationProof(
		receipt, eventChain, sources.Chain, base.Manifest.CapsuleID, publicCommitment.ComponentID,
		publicAttestationRecord.ComponentID, publicAttestation.Digest,
	)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	proofRaw, err := protocol.CanonicalMarshal(proof)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	proofParents := []ParentRef{
		internalParentRef(EdgeDerivedFrom, eventRecord),
		internalParentRef(EdgeDerivedFrom, inventoryRecord),
		internalParentRef(EdgeDerivedFrom, receiptRecord),
		internalParentRef(EdgeDerivedFrom, completionRecord),
		internalParentRef(EdgeDerivedFrom, inspectionRecord),
		internalParentRef(EdgeDerivedFrom, sessionRecord),
		externalParentRef(EdgeDerivedFrom, publicAttestationRecord, base.Manifest.CapsuleID),
	}
	for _, file := range inventory.Files {
		proofParents = append(proofParents, internalParentRef(EdgeDerivedFrom, fileRecords[file.Path]))
	}
	for _, record := range commitments {
		proofParents = append(proofParents, internalParentRef(EdgeDerivedFrom, record))
	}
	proofRecord, err := addPrivateRelationComponent(registry, ComponentInput{
		Name: "relation.owner-private-proof", TypeID: PrivateRelationProofSchemaVersion,
		Visibility: VisibilityPrivate, Payload: proofRaw, Parents: proofParents,
	}, &records, payloads)
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	manifest, err := BuildManifest(registry, ManifestInput{
		StudyID: "task-068-owner-inspection", CellID: "private-owner-inspection-v1",
		ParentCapsules:  []CapsuleRef{{Relation: "extends", CapsuleID: base.Manifest.CapsuleID}},
		ScientificRoots: []string{proofRecord.ComponentID}, PresentationRoots: []string{}, Components: records,
	})
	if err != nil {
		return PrivateRelationPackage{}, err
	}
	privatePackage := Package{Registry: registry, Manifest: manifest, Payloads: payloads}
	if _, err := VerifyPackageFamily(
		context.Background(), privatePackage, []Package{basePackage},
		VerificationOptions{MaximumVisibility: VisibilityPrivate},
	); err != nil {
		return PrivateRelationPackage{}, err
	}
	return PrivateRelationPackage{
		Registry: registry, Manifest: manifest, Payloads: payloads,
		Proof: proof, Attestation: publicAttestation,
	}, nil
}

func validatePrivateRelationSources(inventory relation.PilotPackageInventory, sources PrivateRelationSources) error {
	for _, file := range inventory.Files {
		raw, found := sources.PackageFiles[file.Path]
		if !found || int64(len(raw)) != file.Bytes || protocol.DigestBytes(raw) != file.SHA256 {
			return fmt.Errorf("private relation package file %q is missing or changed", file.Path)
		}
	}
	for name := range sources.PackageFiles {
		if !slices.ContainsFunc(inventory.Files, func(file relation.PilotPackageInventoryFile) bool { return file.Path == name }) {
			return fmt.Errorf("private relation package contains undeclared file %q", name)
		}
	}
	session, err := relation.DecodePilotInspectionSession(bytes.NewReader(sources.SessionPayload))
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(session, sources.Chain.Session) || session.Package.PackageInventoryDigest != inventory.Digest ||
		sources.Chain.PackageBinding.PackageInventoryDigest != inventory.Digest {
		return errors.New("private relation session bytes differ from the verified package-bound chain")
	}
	if len(sources.EventPayloads) != len(sources.Chain.Events) {
		return errors.New("private relation event payload count differs from the verified chain")
	}
	for index, raw := range sources.EventPayloads {
		event, err := relation.DecodePilotInspectionEvent(bytes.NewReader(raw))
		if err != nil {
			return fmt.Errorf("decode private relation event %d: %w", index+1, err)
		}
		if !reflect.DeepEqual(event, sources.Chain.Events[index]) {
			return fmt.Errorf("private relation event %d bytes differ from the verified chain", index+1)
		}
	}
	record, err := relation.DecodePilotInspectionRecord(bytes.NewReader(sources.InspectionPayload))
	if err != nil {
		return err
	}
	completion, err := relation.DecodePilotInspectionCompletion(bytes.NewReader(sources.CompletionPayload))
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(record, sources.Chain.Record) || !reflect.DeepEqual(completion, sources.Chain.Completion) {
		return errors.New("private relation inspection or completion bytes differ from the verified chain")
	}
	_, err = relation.BuildOwnerInspectionPublicAttestation(sources.Chain)
	return err
}

func buildPrivateRelationReceipt(
	registry *Registry,
	base ReferencePackage,
	inventory relation.PilotPackageInventory,
	inventoryRecord ComponentRecord,
	fileRecords map[string]ComponentRecord,
	publicCommitment ComponentRecord,
	records *[]ComponentRecord,
	payloads map[string][]byte,
) (PrivateRelationPackageReceipt, ComponentRecord, error) {
	bindings := make([]PrivateRelationFileBinding, len(inventory.Files))
	for index, file := range inventory.Files {
		record := fileRecords[file.Path]
		bindings[index] = PrivateRelationFileBinding{
			Path: file.Path, Mode: file.Mode, Bytes: file.Bytes, SHA256: file.SHA256,
			ComponentID: record.ComponentID, PayloadSHA256: record.Payload.Digest,
		}
	}
	receipt := PrivateRelationPackageReceipt{
		SchemaVersion: PrivateRelationPackageReceiptSchemaVersion, PackageFormat: inventory.PackageFormat,
		PackageInventoryDigest: inventory.Digest, InventoryComponentID: inventoryRecord.ComponentID,
		PayloadFiles: inventory.PayloadFiles, PayloadBytes: inventory.PayloadBytes, Files: bindings,
		PublicCapsuleID: base.Manifest.CapsuleID, PublicCommitmentComponentID: publicCommitment.ComponentID,
	}
	var err error
	receipt.Digest, err = privateRelationDigest(receipt)
	if err != nil {
		return PrivateRelationPackageReceipt{}, ComponentRecord{}, err
	}
	if err := receipt.Validate(); err != nil {
		return PrivateRelationPackageReceipt{}, ComponentRecord{}, err
	}
	raw, err := protocol.CanonicalMarshal(receipt)
	if err != nil {
		return PrivateRelationPackageReceipt{}, ComponentRecord{}, err
	}
	parents := []ParentRef{
		internalParentRef(EdgeDerivedFrom, inventoryRecord),
		externalParentRef(EdgeDerivedFrom, publicCommitment, base.Manifest.CapsuleID),
	}
	for _, file := range inventory.Files {
		parents = append(parents, internalParentRef(EdgeDerivedFrom, fileRecords[file.Path]))
	}
	record, err := addPrivateRelationComponent(registry, ComponentInput{
		Name: "relation.private-package-receipt", TypeID: PrivateRelationPackageReceiptSchemaVersion,
		Visibility: VisibilityPrivate, Payload: raw, Parents: parents,
	}, records, payloads)
	return receipt, record, err
}

func buildPrivateRelationSourceCommitments(
	registry *Registry,
	fileRecords map[string]ComponentRecord,
	records *[]ComponentRecord,
	payloads map[string][]byte,
) ([]ComponentRecord, error) {
	definitions := []struct {
		name    string
		kind    string
		ordinal int
		path    string
	}{
		{name: "relation.sentinel-material-commitment.01", kind: privateRelationSentinelCommitmentKind, ordinal: 1, path: "sentinel-materials/01.json"},
		{name: "relation.sentinel-material-commitment.02", kind: privateRelationSentinelCommitmentKind, ordinal: 2, path: "sentinel-materials/02.json"},
		{name: "relation.sentinel-material-commitment.03", kind: privateRelationSentinelCommitmentKind, ordinal: 3, path: "sentinel-materials/03.json"},
		{name: "relation.owner-scarcity-inspection-commitment", kind: privateRelationScarcityCommitmentKind, path: "owner-scarcity-inspection.md"},
	}
	built := make([]ComponentRecord, 0, len(definitions))
	for _, definition := range definitions {
		source, found := fileRecords[definition.path]
		if !found {
			return nil, fmt.Errorf("private relation source %q is absent", definition.path)
		}
		commitment := PrivateRelationSourceCommitment{
			SchemaVersion: PrivateRelationSourceCommitmentSchemaVersion, Kind: definition.kind,
			Ordinal: definition.ordinal, SourcePath: definition.path,
			SourceComponentID: source.ComponentID, SourceSHA256: source.Payload.Digest,
		}
		var err error
		commitment.Digest, err = privateRelationDigest(commitment)
		if err != nil {
			return nil, err
		}
		raw, err := protocol.CanonicalMarshal(commitment)
		if err != nil {
			return nil, err
		}
		record, err := addPrivateRelationComponent(registry, ComponentInput{
			Name: definition.name, TypeID: PrivateRelationSourceCommitmentSchemaVersion,
			Visibility: VisibilityPrivate, Payload: raw,
			Parents: []ParentRef{internalParentRef(EdgeDerivedFrom, source)},
		}, records, payloads)
		if err != nil {
			return nil, err
		}
		built = append(built, record)
	}
	return built, nil
}

func buildPrivateRelationEventChain(session relation.PilotInspectionSession, payloads [][]byte) (PrivateRelationEventChain, error) {
	events := make([]relation.PilotInspectionEvent, len(payloads))
	entries := make([]PrivateRelationEventPayload, len(payloads))
	for index, raw := range payloads {
		event, err := relation.DecodePilotInspectionEvent(bytes.NewReader(raw))
		if err != nil {
			return PrivateRelationEventChain{}, err
		}
		events[index] = event
		entries[index] = PrivateRelationEventPayload{
			Sequence: event.Sequence, Bytes: int64(len(raw)), SHA256: protocol.DigestBytes(raw),
			PayloadBase64: base64.StdEncoding.EncodeToString(raw),
		}
	}
	status, err := relation.VerifyPilotInspectionJournal(session, events)
	if err != nil {
		return PrivateRelationEventChain{}, err
	}
	chain := PrivateRelationEventChain{
		SchemaVersion: PrivateRelationEventChainSchemaVersion, SessionDigest: session.Digest,
		Events: entries, EventCount: status.Events, Corrections: status.Corrections,
		RequiredAssessments: status.RequiredAssessments, CompletedAssessments: status.CompletedAssessments,
		CompletedCoreAssessments:     status.CompletedCoreAssessments,
		CompletedScarcityAssessments: status.CompletedScarcityAssessments,
		CompletedBoundaryAssessments: status.CompletedBoundaryAssessments,
		HeadDigest:                   status.HeadDigest, ReadyToFinalize: status.ReadyToFinalize,
	}
	chain.Digest, err = privateRelationDigest(chain)
	if err != nil {
		return PrivateRelationEventChain{}, err
	}
	return chain, chain.Validate()
}

func sealPrivateRelationProof(
	receipt PrivateRelationPackageReceipt,
	events PrivateRelationEventChain,
	chain relation.OwnerInspectionPrivateChain,
	publicCapsuleID string,
	publicCommitmentComponentID string,
	publicAttestationComponentID string,
	publicAttestationDigest string,
) (PrivateRelationProof, error) {
	proof := PrivateRelationProof{
		SchemaVersion: PrivateRelationProofSchemaVersion, PackageFormat: receipt.PackageFormat,
		PackageInventoryDigest: receipt.PackageInventoryDigest, PackagePayloadFiles: receipt.PayloadFiles,
		SessionDigest: chain.Session.Digest, EventCount: events.EventCount, Corrections: events.Corrections,
		RequiredAssessments: events.RequiredAssessments, CompletedAssessments: events.CompletedAssessments,
		InspectionRecordDigest: chain.Record.Digest, CompletionDigest: chain.Completion.Digest,
		CoreStatus: chain.Completion.CoreStatus, ScarcityStatus: chain.Completion.ScarcityStatus,
		OverallStatus: chain.Completion.OverallStatus, HumanStudyStatus: chain.Completion.HumanStudyStatus,
		ExternalActionStatus: chain.Completion.ExternalActionStatus,
		PublicCapsuleID:      publicCapsuleID, PublicCommitmentComponentID: publicCommitmentComponentID,
		PublicAttestationComponentID: publicAttestationComponentID, PublicAttestationDigest: publicAttestationDigest,
		VerificationSteps: slices.Clone(privateRelationVerificationSteps), ProviderCalls: 0,
	}
	var err error
	proof.Digest, err = privateRelationDigest(proof)
	if err != nil {
		return PrivateRelationProof{}, err
	}
	return proof, proof.Validate()
}

func addPrivateRelationComponent(
	registry *Registry,
	input ComponentInput,
	records *[]ComponentRecord,
	payloads map[string][]byte,
) (ComponentRecord, error) {
	record, normalized, err := BuildComponent(registry, input)
	if err != nil {
		return ComponentRecord{}, err
	}
	*records = append(*records, record)
	payloads[record.Payload.Digest] = normalized
	return record, nil
}
