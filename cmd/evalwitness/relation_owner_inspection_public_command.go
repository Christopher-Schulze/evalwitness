package main

import (
	"bytes"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
)

func runRelationRenderOwnerInspectionPublicAttestation(args []string) int {
	flags := flag.NewFlagSet("relation render-owner-inspection-public-attestation", flag.ContinueOnError)
	packageRootPath := flags.String("package-root", "", "exact owner-only package-format-v5 root")
	privateRootPath := flags.String("private-root", "", "owner-only journal vault")
	sessionDigest := flags.String("session", "", "finalized guided inspection session digest")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	loaded, root, session, events, err := loadGuidedInspectionState(*packageRootPath, *privateRootPath, *sessionDigest)
	if err != nil {
		return relationReviewInputError("render-owner-inspection-public-attestation", err)
	}
	completionRaw, err := root.ReadSensitive(pilotInspectionCompletionPath(session.Digest), relation.MaximumDocumentSize)
	if err != nil {
		return relationReviewInputError("render-owner-inspection-public-attestation", fmt.Errorf("read finalized completion: %w", err))
	}
	completion, err := relation.DecodePilotInspectionCompletion(bytes.NewReader(completionRaw))
	if err != nil {
		return relationReviewInputError("render-owner-inspection-public-attestation", err)
	}
	recordRaw, err := root.ReadSensitive(filepath.Join("pilot-inspections", completion.InspectionRecordDigest+".json"), relation.MaximumDocumentSize)
	if err != nil {
		return relationReviewInputError("render-owner-inspection-public-attestation", fmt.Errorf("read finalized inspection: %w", err))
	}
	record, err := relation.DecodePilotInspectionRecord(bytes.NewReader(recordRaw))
	if err != nil {
		return relationReviewInputError("render-owner-inspection-public-attestation", err)
	}
	chain := relation.OwnerInspectionPrivateChain{
		Completion: completion, Record: record, Session: session, Events: events,
		Readiness: loaded.readiness, Bundle: loaded.bundle, Mappings: loaded.mappings,
		Plan: loaded.plan, Primary: loaded.primary, Sentinel: loaded.sentinel, Pilot: loaded.pilot,
		ScarcityMaterials: loaded.scarcityMaterials, PackageBinding: loaded.binding,
	}
	attestation, err := relation.BuildOwnerInspectionPublicAttestation(chain)
	if err != nil {
		return relationReviewOperationError("render-owner-inspection-public-attestation", err)
	}
	if err := relation.VerifyOwnerInspectionPublicAttestation(attestation, chain); err != nil {
		return relationReviewOperationError("render-owner-inspection-public-attestation", err)
	}
	return printRelationJSON(attestation)
}
