package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

const (
	pilotInspectionReadinessPath = "pilot-readiness.json"
	pilotInspectionBundlePath    = "review-bundle.json"
	pilotInspectionMappingsPath  = "private-mappings.json"
	pilotInspectionWorkbookPath  = "owner-inspection.md"
	pilotInspectionAtlasPath     = "owner-change-atlas.md"
	pilotInspectionScarcityPath  = "owner-scarcity-inspection.md"
	pilotInspectionPlanPath      = "relation-audit-plan.json"
	pilotInspectionPrimaryPath   = "relation-primary-sample.json"
	pilotInspectionSentinelPath  = "relation-scarcity-sentinel.json"
	pilotInspectionPilotPath     = "relation-pilot-sample.json"
)

type guidedPilotPackage struct {
	root              string
	inventory         relation.PilotPackageInventory
	binding           relation.PilotInspectionPackageBinding
	readiness         relation.RelationPilotReadiness
	bundle            relation.ReviewBundle
	mappings          []relation.PrivateMapping
	plan              relation.RelationPlanV3
	primary           relation.PrimarySampleV3
	sentinel          relation.ScarcitySentinelV3
	pilot             relation.PilotSampleV3
	scarcityMaterials []relation.CaseMaterial
}

type pilotInspectionGuideLocation struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Purpose   string `json:"purpose"`
}

func runRelationPilotInspectionSessionStart(args []string) int {
	flags := flag.NewFlagSet("relation pilot-inspection-session-start", flag.ContinueOnError)
	packageRootPath := flags.String("package-root", "", "exact owner-only package-format-v5 root")
	privateRootPath := flags.String("private-root", "", "owner-only journal vault outside the package root")
	inspectorAlias := flags.String("inspector-alias", "", "pseudonymous owner inspector alias")
	createdAt := flags.String("created-at", "", "RFC3339 session creation time after readiness")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	loaded, err := loadGuidedPilotPackage(*packageRootPath)
	if err != nil {
		return relationReviewInputError("pilot-inspection-session-start", err)
	}
	if err := rejectJournalInsidePackage(loaded.root, *privateRootPath); err != nil {
		return relationReviewInputError("pilot-inspection-session-start", err)
	}
	session, err := relation.BuildPilotInspectionSession(
		loaded.readiness, loaded.bundle, loaded.mappings, loaded.plan, loaded.primary, loaded.sentinel,
		loaded.pilot, loaded.scarcityMaterials, loaded.binding, *inspectorAlias, *createdAt,
	)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-start", err)
	}
	encoded, err := relation.EncodeIndented(session)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-start", err)
	}
	root, err := createRelationPrivateRoot(*privateRootPath)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-start", err)
	}
	if err := rejectJournalInsidePackage(loaded.root, root.Path()); err != nil {
		return relationReviewOperationError("pilot-inspection-session-start", err)
	}
	relative := pilotInspectionSessionPath(session.Digest)
	if err := root.PublishSensitiveExclusive(relative, encoded); err != nil {
		return relationReviewOperationError("pilot-inspection-session-start", fmt.Errorf("publish immutable session: %w", err))
	}
	return printRelationJSON(struct {
		Created              bool                           `json:"created"`
		SessionDigest        string                         `json:"session_digest"`
		RelativePath         string                         `json:"relative_path"`
		RequiredAssessments  int                            `json:"required_assessments"`
		FirstTarget          relation.PilotInspectionTarget `json:"first_target"`
		InspectionSources    []string                       `json:"inspection_sources"`
		HumanStudyStatus     string                         `json:"human_study_status"`
		ExternalActionStatus relation.ExternalActionStatus  `json:"external_action_status"`
	}{
		Created: true, SessionDigest: session.Digest, RelativePath: relative, RequiredAssessments: relation.PilotInspectionRequiredAssessments,
		FirstTarget: relation.PilotInspectionTarget{
			SubjectKind: relation.PilotInspectionSubjectCorePacket, SubjectID: session.Packets[0].PacketID,
			Dimension: session.CoreDimensions[0],
		},
		InspectionSources: pilotInspectionSourceNames(),
		HumanStudyStatus:  session.HumanStudyStatus, ExternalActionStatus: session.ExternalActionStatus,
	})
}

func runRelationPilotInspectionSessionRecord(args []string) int {
	flags := flag.NewFlagSet("relation pilot-inspection-session-record", flag.ContinueOnError)
	packageRootPath := flags.String("package-root", "", "exact owner-only package-format-v5 root")
	privateRootPath := flags.String("private-root", "", "owner-only journal vault")
	sessionDigest := flags.String("session", "", "guided inspection session digest")
	packetID := flags.String("packet", "", "packet ID; omit with --next")
	scarcityCaseID := flags.String("scarcity-case", "", "scarcity case ID; mutually exclusive with --packet")
	scarcityBoundary := flags.Bool("scarcity-boundary", false, "record the global scarcity interpretation boundary")
	dimensionValue := flags.String("dimension", "", "inspection dimension; omit with --next")
	assessmentValue := flags.String("assessment", "", "passed, failed, or indeterminate")
	recordedAt := flags.String("recorded-at", "", "RFC3339 assessment time")
	correct := flags.Bool("correct", false, "append an explicit correction to an answered dimension")
	next := flags.Bool("next", false, "target the next unanswered dimension")
	confirmation := flags.String("confirm", "", "exact KIND:ID:DIMENSION:ASSESSMENT confirmation")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	_, root, session, events, err := loadGuidedInspectionState(*packageRootPath, *privateRootPath, *sessionDigest)
	if err != nil {
		return relationReviewInputError("pilot-inspection-session-record", err)
	}
	status, err := relation.VerifyPilotInspectionJournal(session, events)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-record", err)
	}
	if *next {
		if strings.TrimSpace(*packetID) != "" || strings.TrimSpace(*scarcityCaseID) != "" || *scarcityBoundary || strings.TrimSpace(*dimensionValue) != "" || *correct {
			return relationReviewInputError("pilot-inspection-session-record", errors.New("--next cannot be combined with an explicit target, --dimension, or --correct"))
		}
		if status.Next == nil {
			return relationReviewInputError("pilot-inspection-session-record", errors.New("session has no unanswered dimension; use an explicit --correct target or finalize"))
		}
		*dimensionValue = string(status.Next.Dimension)
		switch status.Next.SubjectKind {
		case relation.PilotInspectionSubjectCorePacket:
			*packetID = status.Next.SubjectID
		case relation.PilotInspectionSubjectScarcityCase:
			*scarcityCaseID = status.Next.SubjectID
		case relation.PilotInspectionSubjectScarcityBoundary:
			*scarcityBoundary = true
		}
	} else if strings.TrimSpace(*dimensionValue) == "" {
		return relationReviewInputError("pilot-inspection-session-record", errors.New("--dimension and exactly one target are required unless --next is used"))
	}
	subjectKind, subjectID, err := pilotInspectionCommandTarget(*packetID, *scarcityCaseID, *scarcityBoundary)
	if err != nil {
		return relationReviewInputError("pilot-inspection-session-record", err)
	}
	dimension := relation.PilotInspectionDimension(strings.TrimSpace(*dimensionValue))
	assessment := relation.PilotInspectionAssessment(strings.TrimSpace(*assessmentValue))
	expectedConfirmation := string(subjectKind) + ":" + subjectID + ":" + string(dimension) + ":" + string(assessment)
	if *confirmation != expectedConfirmation {
		return relationReviewInputError("pilot-inspection-session-record", fmt.Errorf("explicit confirmation must equal %q", expectedConfirmation))
	}
	event, err := relation.BuildPilotInspectionEvent(
		session, events, subjectKind, subjectID, dimension, assessment, strings.TrimSpace(*recordedAt), *correct,
	)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-record", err)
	}
	encoded, err := relation.EncodeIndented(event)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-record", err)
	}
	relative := pilotInspectionEventPath(session.Digest, event.Sequence)
	if err := root.PublishSensitiveExclusive(relative, encoded); err != nil {
		return relationReviewOperationError("pilot-inspection-session-record", fmt.Errorf("append assessment event: %w", err))
	}
	events, err = loadPilotInspectionEvents(root, session.Digest)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-record", err)
	}
	status, err = relation.VerifyPilotInspectionJournal(session, events)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-record", err)
	}
	return printRelationJSON(struct {
		Appended             bool                            `json:"appended"`
		Sequence             int                             `json:"sequence"`
		EventDigest          string                          `json:"event_digest"`
		Correction           bool                            `json:"correction"`
		CompletedAssessments int                             `json:"completed_assessments"`
		RequiredAssessments  int                             `json:"required_assessments"`
		ReadyToFinalize      bool                            `json:"ready_to_finalize"`
		Next                 *relation.PilotInspectionTarget `json:"next,omitempty"`
	}{
		Appended: true, Sequence: event.Sequence, EventDigest: event.Digest, Correction: event.SupersedesDigest != "",
		CompletedAssessments: status.CompletedAssessments, RequiredAssessments: status.RequiredAssessments,
		ReadyToFinalize: status.ReadyToFinalize, Next: status.Next,
	})
}

func runRelationPilotInspectionSessionStatus(args []string) int {
	flags := flag.NewFlagSet("relation pilot-inspection-session-status", flag.ContinueOnError)
	packageRootPath := flags.String("package-root", "", "exact owner-only package-format-v5 root")
	privateRootPath := flags.String("private-root", "", "owner-only journal vault")
	sessionDigest := flags.String("session", "", "guided inspection session digest")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	loaded, root, session, events, err := loadGuidedInspectionState(*packageRootPath, *privateRootPath, *sessionDigest)
	if err != nil {
		return relationReviewInputError("pilot-inspection-session-status", err)
	}
	status, err := relation.VerifyPilotInspectionJournal(session, events)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-status", err)
	}
	finalized, completionDigest, inspectionDigest, err := verifyExistingPilotInspectionCompletion(root, session, events, loaded)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-status", err)
	}
	return printRelationJSON(struct {
		relation.PilotInspectionJournalStatus
		PackageInventoryDigest string                        `json:"package_inventory_digest"`
		InspectionSources      []string                      `json:"inspection_sources"`
		Finalized              bool                          `json:"finalized"`
		CompletionDigest       string                        `json:"completion_digest,omitempty"`
		InspectionDigest       string                        `json:"inspection_digest,omitempty"`
		HumanStudyStatus       string                        `json:"human_study_status"`
		ExternalActionStatus   relation.ExternalActionStatus `json:"external_action_status"`
	}{
		PilotInspectionJournalStatus: status, PackageInventoryDigest: loaded.inventory.Digest,
		InspectionSources: pilotInspectionSourceNames(),
		Finalized:         finalized, CompletionDigest: completionDigest, InspectionDigest: inspectionDigest,
		HumanStudyStatus: session.HumanStudyStatus, ExternalActionStatus: session.ExternalActionStatus,
	})
}

func runRelationPilotInspectionSessionGuide(args []string) int {
	flags := flag.NewFlagSet("relation pilot-inspection-session-guide", flag.ContinueOnError)
	packageRootPath := flags.String("package-root", "", "exact owner-only package-format-v5 root")
	privateRootPath := flags.String("private-root", "", "owner-only journal vault")
	sessionDigest := flags.String("session", "", "guided inspection session digest")
	packetID := flags.String("packet", "", "explicit core packet ID")
	scarcityCaseID := flags.String("scarcity-case", "", "explicit scarcity case ID")
	scarcityBoundary := flags.Bool("scarcity-boundary", false, "guide the global scarcity interpretation boundary")
	dimensionValue := flags.String("dimension", "", "inspection dimension for an explicit target")
	next := flags.Bool("next", false, "guide the next unanswered target")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	loaded, _, session, events, err := loadGuidedInspectionState(*packageRootPath, *privateRootPath, *sessionDigest)
	if err != nil {
		return relationReviewInputError("pilot-inspection-session-guide", err)
	}
	status, err := relation.VerifyPilotInspectionJournal(session, events)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-guide", err)
	}
	var target relation.PilotInspectionTarget
	if *next {
		if strings.TrimSpace(*packetID) != "" || strings.TrimSpace(*scarcityCaseID) != "" || *scarcityBoundary || strings.TrimSpace(*dimensionValue) != "" {
			return relationReviewInputError("pilot-inspection-session-guide", errors.New("--next cannot be combined with an explicit target or --dimension"))
		}
		if status.Next == nil {
			return relationReviewInputError("pilot-inspection-session-guide", errors.New("session has no unanswered target; guide an explicit target to review a completed assessment"))
		}
		target = *status.Next
	} else {
		subjectKind, subjectID, targetErr := pilotInspectionCommandTarget(*packetID, *scarcityCaseID, *scarcityBoundary)
		if targetErr != nil {
			return relationReviewInputError("pilot-inspection-session-guide", targetErr)
		}
		target = relation.PilotInspectionTarget{
			SubjectKind: subjectKind, SubjectID: subjectID,
			Dimension: relation.PilotInspectionDimension(strings.TrimSpace(*dimensionValue)),
		}
	}
	if err := relation.ValidatePilotInspectionTarget(session, target); err != nil {
		return relationReviewInputError("pilot-inspection-session-guide", err)
	}
	locations, err := pilotInspectionGuideLocations(loaded, target)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-guide", err)
	}
	return printRelationJSON(struct {
		SessionDigest        string                         `json:"session_digest"`
		PackageInventory     string                         `json:"package_inventory_digest"`
		Target               relation.PilotInspectionTarget `json:"target"`
		Prompt               string                         `json:"prompt"`
		AllowedAssessments   []string                       `json:"allowed_assessments"`
		ConfirmationTemplate string                         `json:"confirmation_template"`
		Locations            []pilotInspectionGuideLocation `json:"locations"`
		HumanStudyStatus     string                         `json:"human_study_status"`
		ExternalActionStatus relation.ExternalActionStatus  `json:"external_action_status"`
	}{
		SessionDigest: session.Digest, PackageInventory: loaded.inventory.Digest, Target: target,
		Prompt:               pilotInspectionDimensionPrompt(target.Dimension),
		AllowedAssessments:   []string{"passed", "failed", "indeterminate"},
		ConfirmationTemplate: string(target.SubjectKind) + ":" + target.SubjectID + ":" + string(target.Dimension) + ":<passed|failed|indeterminate>",
		Locations:            locations, HumanStudyStatus: session.HumanStudyStatus, ExternalActionStatus: session.ExternalActionStatus,
	})
}

func runRelationPilotInspectionSessionFinalize(args []string) int {
	flags := flag.NewFlagSet("relation pilot-inspection-session-finalize", flag.ContinueOnError)
	packageRootPath := flags.String("package-root", "", "exact owner-only package-format-v5 root")
	privateRootPath := flags.String("private-root", "", "owner-only journal vault")
	sessionDigest := flags.String("session", "", "guided inspection session digest")
	inspectedAt := flags.String("inspected-at", "", "RFC3339 completion time after the last event")
	confirmation := flags.String("confirm", "", "exact completion digest shown by the confirmation plan")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	loaded, root, session, events, err := loadGuidedInspectionState(*packageRootPath, *privateRootPath, *sessionDigest)
	if err != nil {
		return relationReviewInputError("pilot-inspection-session-finalize", err)
	}
	record, completion, err := relation.BuildPilotInspectionCompletion(
		session, events, loaded.readiness, loaded.bundle, loaded.mappings, strings.TrimSpace(*inspectedAt),
	)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-finalize", err)
	}
	if strings.TrimSpace(*confirmation) == "" {
		return printRelationJSON(struct {
			Ready                  bool                                            `json:"ready"`
			ConfirmationDigest     string                                          `json:"confirmation_digest"`
			InspectionRecordDigest string                                          `json:"inspection_record_digest"`
			CoreStatus             relation.PilotInspectionOverallStatus           `json:"core_status"`
			ScarcityStatus         relation.PilotInspectionOverallStatus           `json:"scarcity_status"`
			OverallStatus          relation.PilotInspectionOverallStatus           `json:"overall_status"`
			DecisionSummaries      []relation.PilotInspectionDecisionSummary       `json:"decision_summaries"`
			ScarcitySummaries      []relation.PilotInspectionScarcitySummary       `json:"scarcity_summaries"`
			ScarcityBoundary       relation.PilotInspectionScarcityBoundarySummary `json:"scarcity_boundary"`
			HumanStudyStatus       string                                          `json:"human_study_status"`
			ExternalActionStatus   relation.ExternalActionStatus                   `json:"external_action_status"`
		}{
			Ready: true, ConfirmationDigest: completion.Digest, InspectionRecordDigest: record.Digest,
			CoreStatus: completion.CoreStatus, ScarcityStatus: completion.ScarcityStatus,
			OverallStatus: completion.OverallStatus, DecisionSummaries: completion.DecisionSummaries,
			ScarcitySummaries: completion.ScarcitySummaries, ScarcityBoundary: completion.ScarcityBoundary,
			HumanStudyStatus: completion.HumanStudyStatus, ExternalActionStatus: completion.ExternalActionStatus,
		})
	}
	if strings.TrimSpace(*confirmation) != completion.Digest {
		return relationReviewInputError("pilot-inspection-session-finalize", errors.New("completion confirmation does not match the exact rendered decision state"))
	}
	if err := relation.VerifyPilotInspectionCompletion(completion, session, events, record, loaded.readiness, loaded.bundle, loaded.mappings); err != nil {
		return relationReviewOperationError("pilot-inspection-session-finalize", err)
	}
	encodedRecord, err := relation.EncodeIndented(record)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-finalize", err)
	}
	inspectionPath := filepath.Join("pilot-inspections", record.Digest+".json")
	if err := publishSensitiveExclusiveOrVerify(root, inspectionPath, encodedRecord); err != nil {
		return relationReviewOperationError("pilot-inspection-session-finalize", fmt.Errorf("publish inspection record: %w", err))
	}
	encodedCompletion, err := relation.EncodeIndented(completion)
	if err != nil {
		return relationReviewOperationError("pilot-inspection-session-finalize", err)
	}
	completionPath := pilotInspectionCompletionPath(session.Digest)
	if err := publishSensitiveExclusiveOrVerify(root, completionPath, encodedCompletion); err != nil {
		return relationReviewOperationError("pilot-inspection-session-finalize", fmt.Errorf("publish completion receipt: %w", err))
	}
	if _, _, _, err := verifyExistingPilotInspectionCompletion(root, session, events, loaded); err != nil {
		return relationReviewOperationError("pilot-inspection-session-finalize", fmt.Errorf("independent post-publication verification: %w", err))
	}
	return printRelationJSON(struct {
		Finalized            bool                                  `json:"finalized"`
		CompletionDigest     string                                `json:"completion_digest"`
		InspectionDigest     string                                `json:"inspection_digest"`
		InspectionPath       string                                `json:"inspection_path"`
		CompletionPath       string                                `json:"completion_path"`
		CoreStatus           relation.PilotInspectionOverallStatus `json:"core_status"`
		ScarcityStatus       relation.PilotInspectionOverallStatus `json:"scarcity_status"`
		OverallStatus        relation.PilotInspectionOverallStatus `json:"overall_status"`
		HumanStudyStatus     string                                `json:"human_study_status"`
		ExternalActionStatus relation.ExternalActionStatus         `json:"external_action_status"`
	}{
		Finalized: true, CompletionDigest: completion.Digest, InspectionDigest: record.Digest,
		InspectionPath: inspectionPath, CompletionPath: completionPath,
		CoreStatus: completion.CoreStatus, ScarcityStatus: completion.ScarcityStatus, OverallStatus: completion.OverallStatus,
		HumanStudyStatus: record.HumanStudyStatus, ExternalActionStatus: record.ExternalActionStatus,
	})
}

func loadGuidedInspectionState(packageRootPath, privateRootPath, sessionDigest string) (guidedPilotPackage, *safety.CacheRoot, relation.PilotInspectionSession, []relation.PilotInspectionEvent, error) {
	var zero guidedPilotPackage
	loaded, err := loadGuidedPilotPackage(packageRootPath)
	if err != nil {
		return zero, nil, relation.PilotInspectionSession{}, nil, err
	}
	if err := rejectJournalInsidePackage(loaded.root, privateRootPath); err != nil {
		return zero, nil, relation.PilotInspectionSession{}, nil, err
	}
	if !validCommandDigest(sessionDigest) {
		return zero, nil, relation.PilotInspectionSession{}, nil, errors.New("a valid session digest is required")
	}
	root, err := openRelationPrivateRoot(privateRootPath)
	if err != nil {
		return zero, nil, relation.PilotInspectionSession{}, nil, err
	}
	session, err := readPilotInspectionSession(root, sessionDigest)
	if err != nil {
		return zero, nil, relation.PilotInspectionSession{}, nil, err
	}
	if session.Digest != sessionDigest {
		return zero, nil, relation.PilotInspectionSession{}, nil, errors.New("session path and content digest disagree")
	}
	if err := relation.VerifyPilotInspectionSession(
		session, loaded.readiness, loaded.bundle, loaded.mappings, loaded.plan, loaded.primary,
		loaded.sentinel, loaded.pilot, loaded.scarcityMaterials, loaded.binding,
	); err != nil {
		return zero, nil, relation.PilotInspectionSession{}, nil, err
	}
	events, err := loadPilotInspectionEvents(root, sessionDigest)
	if err != nil {
		return zero, nil, relation.PilotInspectionSession{}, nil, err
	}
	if _, err := relation.VerifyPilotInspectionJournal(session, events); err != nil {
		return zero, nil, relation.PilotInspectionSession{}, nil, err
	}
	return loaded, root, session, events, nil
}

func loadGuidedPilotPackage(packageRootPath string) (guidedPilotPackage, error) {
	var zero guidedPilotPackage
	root, err := canonicalPrivateDirectory(packageRootPath)
	if err != nil {
		return zero, err
	}
	inventory, err := readPrivateRelationDocument(filepath.Join(root, "package-inventory.json"), relation.DecodePilotPackageInventory)
	if err != nil {
		return zero, err
	}
	if err := verifyPilotPackageInventory(root, inventory); err != nil {
		return zero, err
	}
	plan, err := readPrivateRelationDocument(filepath.Join(root, pilotInspectionPlanPath), relation.DecodePlanV3)
	if err != nil {
		return zero, err
	}
	primary, err := readPrivateRelationDocument(filepath.Join(root, pilotInspectionPrimaryPath), func(reader io.Reader) (relation.PrimarySampleV3, error) {
		return relation.DecodePrimarySampleV3(reader, plan)
	})
	if err != nil {
		return zero, err
	}
	sentinel, err := readPrivateRelationDocument(filepath.Join(root, pilotInspectionSentinelPath), func(reader io.Reader) (relation.ScarcitySentinelV3, error) {
		return relation.DecodeScarcitySentinelV3(reader, plan, primary)
	})
	if err != nil {
		return zero, err
	}
	pilot, err := readPrivateRelationDocument(filepath.Join(root, pilotInspectionPilotPath), func(reader io.Reader) (relation.PilotSampleV3, error) {
		return relation.DecodePilotSampleV3(reader, plan, primary, sentinel)
	})
	if err != nil {
		return zero, err
	}
	scarcityMaterials := make([]relation.CaseMaterial, len(sentinel.Cases))
	for index := range sentinel.Cases {
		materialPath := filepath.Join(root, "sentinel-materials", fmt.Sprintf("%02d.json", index+1))
		material, readErr := readPrivateRelationDocument(materialPath, relation.DecodeCaseMaterial)
		if readErr != nil {
			return zero, readErr
		}
		scarcityMaterials[index] = material
	}
	if err := relation.VerifyScarcityInspectionMaterials(plan, primary, sentinel, scarcityMaterials); err != nil {
		return zero, err
	}
	readiness, err := readPrivateRelationDocument(filepath.Join(root, pilotInspectionReadinessPath), relation.DecodeRelationPilotReadiness)
	if err != nil {
		return zero, err
	}
	bundle, err := readPrivateRelationDocument(filepath.Join(root, pilotInspectionBundlePath), relation.DecodeReviewBundle)
	if err != nil {
		return zero, err
	}
	mappings, err := readPrivateRelationDocument(filepath.Join(root, pilotInspectionMappingsPath), relation.DecodePrivateMappings)
	if err != nil {
		return zero, err
	}
	if err := relation.VerifyRelationPilotReadiness(readiness, bundle, mappings); err != nil {
		return zero, err
	}
	files := make(map[string]relation.PilotPackageInventoryFile, len(inventory.Files))
	for _, file := range inventory.Files {
		files[file.Path] = file
	}
	required := []string{
		pilotInspectionReadinessPath, pilotInspectionBundlePath, pilotInspectionMappingsPath,
		pilotInspectionWorkbookPath, pilotInspectionAtlasPath, pilotInspectionScarcityPath,
		pilotInspectionPlanPath, pilotInspectionPrimaryPath, pilotInspectionSentinelPath, pilotInspectionPilotPath,
		"sentinel-materials/01.json", "sentinel-materials/02.json", "sentinel-materials/03.json",
	}
	for _, name := range required {
		if _, exists := files[name]; !exists {
			return zero, fmt.Errorf("package inventory does not bind required guided-inspection input %q", name)
		}
	}
	binding := relation.PilotInspectionPackageBinding{
		PackageFormat: inventory.PackageFormat, PackageInventoryDigest: inventory.Digest,
		ReadinessSHA256: files[pilotInspectionReadinessPath].SHA256, BundleSHA256: files[pilotInspectionBundlePath].SHA256,
		MappingsSHA256: files[pilotInspectionMappingsPath].SHA256, WorkbookSHA256: files[pilotInspectionWorkbookPath].SHA256,
		ChangeAtlasSHA256: files[pilotInspectionAtlasPath].SHA256, ScarcitySentinelSHA256: files[pilotInspectionSentinelPath].SHA256,
		ScarcityAppendixSHA256: files[pilotInspectionScarcityPath].SHA256,
	}
	return guidedPilotPackage{
		root: root, inventory: inventory, binding: binding, readiness: readiness, bundle: bundle, mappings: mappings,
		plan: plan, primary: primary, sentinel: sentinel, pilot: pilot, scarcityMaterials: scarcityMaterials,
	}, nil
}

func verifyPilotPackageInventory(root string, inventory relation.PilotPackageInventory) error {
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode().Perm()&0o077 != 0 {
		return errors.New("guided inspection package root must be an owner-only directory")
	}
	expectedDirectories := make(map[string]relation.PilotPackageInventoryDirectory, len(inventory.Directories))
	for _, directory := range inventory.Directories {
		expectedDirectories[directory.Path] = directory
	}
	expectedFiles := make(map[string]relation.PilotPackageInventoryFile, len(inventory.Files))
	for _, file := range inventory.Files {
		expectedFiles[file.Path] = file
	}
	seenDirectories := make(map[string]struct{}, len(expectedDirectories))
	seenFiles := make(map[string]struct{}, len(expectedFiles))
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		relative, relErr := filepath.Rel(root, current)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("package inventory path %q is a symlink", relative)
		}
		if entry.IsDir() {
			expected, exists := expectedDirectories[relative]
			if !exists || info.Mode().Perm() != 0o700 || expected.Mode != "0700" {
				return fmt.Errorf("package directory %q is undeclared or has unsafe mode", relative)
			}
			seenDirectories[relative] = struct{}{}
			return nil
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			return fmt.Errorf("package file %q is not a mode-0600 regular file", relative)
		}
		if relative == "package-inventory.json" || relative == "SHA256SUMS" {
			return nil
		}
		expected, exists := expectedFiles[relative]
		if !exists || info.Size() != expected.Bytes {
			return fmt.Errorf("package file %q is undeclared or has the wrong size", relative)
		}
		digest, digestErr := sha256File(current)
		if digestErr != nil || digest != expected.SHA256 {
			return fmt.Errorf("package file %q digest mismatch", relative)
		}
		seenFiles[relative] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}
	if len(seenDirectories) != len(expectedDirectories) || len(seenFiles) != len(expectedFiles) {
		return errors.New("package inventory is missing one or more declared paths")
	}
	return verifyPilotPackageChecksumManifest(root, inventory)
}

func verifyPilotPackageChecksumManifest(root string, inventory relation.PilotPackageInventory) error {
	manifestPath := filepath.Join(root, "SHA256SUMS")
	info, err := os.Lstat(manifestPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > relation.MaximumDocumentSize {
		return errors.New("package SHA256SUMS must be a bounded mode-0600 regular file")
	}
	type checksumEntry struct {
		path   string
		digest string
	}
	entries := make([]checksumEntry, 0, len(inventory.Files)+1)
	for _, file := range inventory.Files {
		entries = append(entries, checksumEntry{path: file.Path, digest: file.SHA256})
	}
	inventoryDigest, err := sha256File(filepath.Join(root, "package-inventory.json"))
	if err != nil {
		return err
	}
	entries = append(entries, checksumEntry{path: "package-inventory.json", digest: inventoryDigest})
	sort.Slice(entries, func(left, right int) bool { return entries[left].path < entries[right].path })
	lines := make([]string, len(entries))
	for index, entry := range entries {
		lines[index] = entry.digest + "  " + entry.path
	}
	expected := strings.Join(lines, "\n") + "\n"
	actual, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, []byte(expected)) {
		return errors.New("package SHA256SUMS does not exactly reproduce the inventory and inventory document")
	}
	return nil
}

func readPilotInspectionSession(root *safety.CacheRoot, digest string) (relation.PilotInspectionSession, error) {
	raw, err := root.ReadSensitive(pilotInspectionSessionPath(digest), relation.MaximumDocumentSize)
	if err != nil {
		return relation.PilotInspectionSession{}, err
	}
	return relation.DecodePilotInspectionSession(bytes.NewReader(raw))
}

func loadPilotInspectionEvents(root *safety.CacheRoot, sessionDigest string) ([]relation.PilotInspectionEvent, error) {
	directoryRelative := filepath.Join("pilot-inspection-sessions", sessionDigest, "events")
	directory, err := root.Resolve(directoryRelative)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []relation.PilotInspectionEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	type eventFile struct {
		sequence int
		name     string
	}
	files := make([]eventFile, len(entries))
	for index, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || len(entry.Name()) != 11 {
			return nil, fmt.Errorf("journal events directory contains unexpected entry %q", entry.Name())
		}
		sequence, parseErr := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
		if parseErr != nil || sequence < 1 || fmt.Sprintf("%06d.json", sequence) != entry.Name() {
			return nil, fmt.Errorf("journal event filename %q is not canonical", entry.Name())
		}
		files[index] = eventFile{sequence: sequence, name: entry.Name()}
	}
	sort.Slice(files, func(left, right int) bool { return files[left].sequence < files[right].sequence })
	events := make([]relation.PilotInspectionEvent, len(files))
	for index, file := range files {
		if file.sequence != index+1 {
			return nil, errors.New("journal event sequence is truncated or contains a gap")
		}
		raw, readErr := root.ReadSensitive(filepath.Join(directoryRelative, file.name), relation.MaximumDocumentSize)
		if readErr != nil {
			return nil, readErr
		}
		event, decodeErr := relation.DecodePilotInspectionEvent(bytes.NewReader(raw))
		if decodeErr != nil {
			return nil, decodeErr
		}
		events[index] = event
	}
	return events, nil
}

func verifyExistingPilotInspectionCompletion(root *safety.CacheRoot, session relation.PilotInspectionSession, events []relation.PilotInspectionEvent, loaded guidedPilotPackage) (bool, string, string, error) {
	raw, err := root.ReadSensitive(pilotInspectionCompletionPath(session.Digest), relation.MaximumDocumentSize)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", "", nil
		}
		return false, "", "", err
	}
	completion, err := relation.DecodePilotInspectionCompletion(bytes.NewReader(raw))
	if err != nil {
		return false, "", "", err
	}
	recordRaw, err := root.ReadSensitive(filepath.Join("pilot-inspections", completion.InspectionRecordDigest+".json"), relation.MaximumDocumentSize)
	if err != nil {
		return false, "", "", err
	}
	record, err := relation.DecodePilotInspectionRecord(bytes.NewReader(recordRaw))
	if err != nil {
		return false, "", "", err
	}
	if err := relation.VerifyPilotInspectionCompletion(completion, session, events, record, loaded.readiness, loaded.bundle, loaded.mappings); err != nil {
		return false, "", "", err
	}
	return true, completion.Digest, record.Digest, nil
}

func publishSensitiveExclusiveOrVerify(root *safety.CacheRoot, relative string, expected []byte) error {
	if err := root.PublishSensitiveExclusive(relative, expected); err == nil {
		return nil
	}
	existing, readErr := root.ReadSensitive(relative, relation.MaximumDocumentSize)
	if readErr != nil {
		return readErr
	}
	if !bytes.Equal(existing, expected) {
		return errors.New("existing owner-only artifact differs from the deterministic finalization")
	}
	return nil
}

func canonicalPrivateDirectory(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("owner-only package root is required")
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("owner-only package root must not be a symlink")
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func rejectJournalInsidePackage(packageRoot, privateRoot string) error {
	privateRoot = strings.TrimSpace(privateRoot)
	if privateRoot == "" {
		return errors.New("owner-only journal private root is required")
	}
	abs, err := canonicalProspectivePath(privateRoot)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(packageRoot, abs)
	if err != nil {
		return err
	}
	if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("journal private root must not equal or descend from the immutable package root")
	}
	return nil
}

func canonicalProspectivePath(value string) (string, error) {
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if _, err := os.Lstat(abs); err == nil {
		return filepath.EvalSymlinks(abs)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	missing := make([]string, 0, 4)
	ancestor := abs
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", errors.New("cannot resolve an existing ancestor for the journal root")
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for index := len(missing) - 1; index >= 0; index-- {
		resolved = filepath.Join(resolved, missing[index])
	}
	return resolved, nil
}

func openRelationPrivateRoot(path string) (*safety.CacheRoot, error) {
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		return nil, err
	}
	return safety.OpenCacheRoot(policy, strings.TrimSpace(path))
}

func sha256File(path string) (digest string, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close digest source: %w", closeErr)
		}
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func pilotInspectionSessionPath(digest string) string {
	return filepath.Join("pilot-inspection-sessions", digest, "session.json")
}

func pilotInspectionEventPath(digest string, sequence int) string {
	return filepath.Join("pilot-inspection-sessions", digest, "events", fmt.Sprintf("%06d.json", sequence))
}

func pilotInspectionCompletionPath(digest string) string {
	return filepath.Join("pilot-inspection-sessions", digest, "completion.json")
}

func pilotInspectionSourceNames() []string {
	return []string{pilotInspectionAtlasPath, pilotInspectionWorkbookPath, pilotInspectionScarcityPath}
}

func pilotInspectionGuideLocations(loaded guidedPilotPackage, target relation.PilotInspectionTarget) ([]pilotInspectionGuideLocation, error) {
	switch target.SubjectKind {
	case relation.PilotInspectionSubjectCorePacket:
		marker := "- Packet ID: `" + target.SubjectID + "`"
		workbook, err := pilotInspectionMarkerLocation(loaded, pilotInspectionWorkbookPath, marker, "## Packet ", "complete reviewer-visible evidence and hidden mapping")
		if err != nil {
			return nil, err
		}
		atlas, err := pilotInspectionMarkerLocation(loaded, pilotInspectionAtlasPath, marker, "## Packet ", "all rendered changes and structural review flags")
		if err != nil {
			return nil, err
		}
		return []pilotInspectionGuideLocation{atlas, workbook}, nil
	case relation.PilotInspectionSubjectScarcityCase:
		marker := "- Case ID: `" + target.SubjectID + "`"
		location, err := pilotInspectionMarkerLocation(loaded, pilotInspectionScarcityPath, marker, "## Sentinel case ", "complete scarcity-case evidence and owner prompts")
		if err != nil {
			return nil, err
		}
		return []pilotInspectionGuideLocation{location}, nil
	case relation.PilotInspectionSubjectScarcityBoundary:
		boundary, err := pilotInspectionHeadingLocation(loaded, pilotInspectionScarcityPath, "## Frozen scarcity boundary", "frozen availability, role, estimand, and held-out-claim boundary")
		if err != nil {
			return nil, err
		}
		completion, err := pilotInspectionHeadingLocation(loaded, pilotInspectionScarcityPath, "## Owner completion gate", "explicit no-promotion and no-authorization completion checks")
		if err != nil {
			return nil, err
		}
		return []pilotInspectionGuideLocation{boundary, completion}, nil
	default:
		return nil, errors.New("guided owner inspection target kind has no navigation surface")
	}
}

func pilotInspectionMarkerLocation(loaded guidedPilotPackage, relative, marker, sectionPrefix, purpose string) (pilotInspectionGuideLocation, error) {
	lines, err := pilotInspectionDocumentLines(loaded.root, relative)
	if err != nil {
		return pilotInspectionGuideLocation{}, err
	}
	markerIndex := -1
	for index, line := range lines {
		if line == marker {
			if markerIndex >= 0 {
				return pilotInspectionGuideLocation{}, fmt.Errorf("inspection source %q repeats target marker", relative)
			}
			markerIndex = index
		}
	}
	if markerIndex < 0 {
		return pilotInspectionGuideLocation{}, fmt.Errorf("inspection source %q lacks target marker", relative)
	}
	start := -1
	for index := markerIndex; index >= 0; index-- {
		if strings.HasPrefix(lines[index], sectionPrefix) {
			start = index
			break
		}
	}
	if start < 0 {
		return pilotInspectionGuideLocation{}, fmt.Errorf("inspection source %q lacks target section heading", relative)
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if strings.HasPrefix(lines[index], sectionPrefix) || lines[index] == "## Owner completion gate" || lines[index] == "## Atlas completion boundary" {
			end = index
			break
		}
	}
	return pilotInspectionLocation(loaded, relative, start+1, end, purpose)
}

func pilotInspectionHeadingLocation(loaded guidedPilotPackage, relative, heading, purpose string) (pilotInspectionGuideLocation, error) {
	lines, err := pilotInspectionDocumentLines(loaded.root, relative)
	if err != nil {
		return pilotInspectionGuideLocation{}, err
	}
	start := -1
	for index, line := range lines {
		if line == heading {
			if start >= 0 {
				return pilotInspectionGuideLocation{}, fmt.Errorf("inspection source %q repeats heading %q", relative, heading)
			}
			start = index
		}
	}
	if start < 0 {
		return pilotInspectionGuideLocation{}, fmt.Errorf("inspection source %q lacks heading %q", relative, heading)
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if strings.HasPrefix(lines[index], "## ") {
			end = index
			break
		}
	}
	return pilotInspectionLocation(loaded, relative, start+1, end, purpose)
}

func pilotInspectionDocumentLines(root, relative string) ([]string, error) {
	path := filepath.Join(root, relative)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > relation.MaximumDocumentSize {
		return nil, fmt.Errorf("inspection source %q is not a bounded regular file", relative)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("inspection source %q is not valid UTF-8", relative)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n"), nil
}

func pilotInspectionLocation(loaded guidedPilotPackage, relative string, startLine, endLine int, purpose string) (pilotInspectionGuideLocation, error) {
	for _, file := range loaded.inventory.Files {
		if file.Path == relative {
			return pilotInspectionGuideLocation{
				Path: relative, SHA256: file.SHA256, StartLine: startLine, EndLine: endLine, Purpose: purpose,
			}, nil
		}
	}
	return pilotInspectionGuideLocation{}, fmt.Errorf("package inventory lacks inspection source %q", relative)
}

func pilotInspectionDimensionPrompt(dimension relation.PilotInspectionDimension) string {
	switch dimension {
	case relation.PilotInspectionDimensionTaskContext:
		return "Does the rendered task context preserve the requirement needed to judge the controlled relation?"
	case relation.PilotInspectionDimensionEvidenceAlignment:
		return "Do the visible evidence slots align with the hidden original/transformed mapping and governed case?"
	case relation.PilotInspectionDimensionTransformationIsolation:
		return "Is the intended controlled transformation isolated without an additional semantic change?"
	case relation.PilotInspectionDimensionInformationSufficiency:
		return "Does the complete workbook retain enough task, trajectory, and outcome context for a defensible judgment?"
	case relation.PilotInspectionDimensionBlindingIntegrity:
		return "Does the reviewer-visible packet avoid leaking hidden side identity, family, or expected relation?"
	case relation.PilotInspectionDimensionRubricApplicability:
		return "Can the frozen controlled-relation rubric be applied to this packet and unit without reinterpretation?"
	case relation.PilotInspectionDimensionRedistributionBoundary:
		return "Do license, visibility, and custody fields support the planned restricted review use without public redistribution?"
	case relation.PilotInspectionDimensionCandidateOrder:
		return "Does the pair-of-pairs preserve candidate content exactly while reversing only candidate presentation order?"
	case relation.PilotInspectionDimensionScarcityOriginalEvidence:
		return "Does the original sentinel surface contain task-relevant directly linked verification evidence?"
	case relation.PilotInspectionDimensionScarcityTargetOmission:
		return "Does the transformed sentinel surface omit the targeted verification evidence?"
	case relation.PilotInspectionDimensionScarcityRelationPreservation:
		return "Apart from the targeted omission, is the governed trajectory relation preserved without an additional change?"
	case relation.PilotInspectionDimensionScarcityInformationSufficiency:
		return "Does the paired sentinel rendering retain enough local and outcome context to inspect the omission?"
	case relation.PilotInspectionDimensionScarcityExhaustiveScope:
		return "Were all and only the three exhaustive naturally eligible sentinel cases inspected?"
	case relation.PilotInspectionDimensionScarcityRoleIntegrity:
		return "Was the frozen 2-development, 1-calibration, 0-test role boundary preserved without relabeling?"
	case relation.PilotInspectionDimensionScarcityEstimandSeparation:
		return "Were sentinel observations kept outside the seven-family pilot and 28-case primary estimand?"
	case relation.PilotInspectionDimensionScarcityNonAuthorization:
		return "Was no reviewer, provider, publication, distribution, or held-out claim authorized from the scarcity inspection?"
	default:
		return ""
	}
}

func validCommandDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func pilotInspectionCommandTarget(packetID, scarcityCaseID string, scarcityBoundary bool) (relation.PilotInspectionSubjectKind, string, error) {
	packetID = strings.TrimSpace(packetID)
	scarcityCaseID = strings.TrimSpace(scarcityCaseID)
	targets := 0
	if packetID != "" {
		targets++
	}
	if scarcityCaseID != "" {
		targets++
	}
	if scarcityBoundary {
		targets++
	}
	if targets != 1 {
		return "", "", errors.New("exactly one of --packet, --scarcity-case, or --scarcity-boundary is required")
	}
	if packetID != "" {
		return relation.PilotInspectionSubjectCorePacket, packetID, nil
	}
	if scarcityCaseID != "" {
		return relation.PilotInspectionSubjectScarcityCase, scarcityCaseID, nil
	}
	return relation.PilotInspectionSubjectScarcityBoundary, relation.PilotInspectionScarcityBoundaryID, nil
}
