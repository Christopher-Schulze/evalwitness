package claim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
)

const identicalResponseClaimGeneration = "identical-response-v5"

const (
	identicalResponseRootComponentName        = "identical-response.outer-bind"
	identicalResponseRootTypeID               = "evalwitness.identical-response-050-bind.v2"
	identicalResponseAnalysisComponentName    = "identical-response.offline-analysis"
	identicalResponseAnalysisTypeID           = "evalwitness.identical-response-analysis.v1"
	identicalResponseAdmissionComponentName   = "identical-response.research-admission"
	identicalResponseAdmissionTypeID          = "evalwitness.identical-response-research-admission.v1"
	identicalResponseCaptureRunComponentName  = "identical-response.capture-run-attestation"
	identicalResponseCaptureRunTypeID         = "evalwitness.identical-response-capture-run-attestation.v1"
	identicalResponseRouteComponentName       = "identical-response.route-attestation"
	identicalResponseRouteTypeID              = "evalwitness.identical-response-route-attestation.v1"
	identicalResponseStudyRecordComponentName = "identical-response.study-record"
	identicalResponseStudyRecordTypeID        = "evalwitness.identical-response-study-record.v1"
)

type identicalResponseLedgerRoot struct {
	Status                  string `json:"status"`
	EvidenceCeiling         string `json:"evidence_ceiling"`
	ProviderCalls           int    `json:"provider_calls"`
	NetworkRequired         bool   `json:"network_required"`
	AuthorizedCalls         int    `json:"authorized_calls"`
	ObservedCalls           int    `json:"observed_calls"`
	CompleteResearchEntries int    `json:"complete_research_entries"`
	StudyRecordDigest       string `json:"study_record_digest"`
	AuthorizationDigest     string `json:"authorization_digest"`
	AnalysisDigest          string `json:"analysis_digest"`
}

type identicalResponseLedgerAnalysis struct {
	StudyRecordDigest   string `json:"study_record_digest"`
	StudyManifestDigest string `json:"study_manifest_digest"`
	CaptureSHA256       string `json:"capture_sha256"`
	RouteID             string `json:"route_id"`
	Digest              string `json:"digest"`
	Summary             struct {
		TaskGroups    int `json:"task_groups"`
		Complete      int `json:"complete"`
		Agreements    int `json:"agreements"`
		Disagreements int `json:"disagreements"`
		Unresolved    int `json:"unresolved"`
	} `json:"summary"`
	OutcomeSensitivity struct {
		Covered int `json:"covered"`
	} `json:"outcome_sensitivity"`
}

type identicalResponseLedgerAdmission struct {
	Admission               string `json:"admission"`
	AuthorizedCalls         int    `json:"authorized_calls"`
	ObservedCalls           int    `json:"observed_calls"`
	CompleteResearchEntries int    `json:"complete_research_entries"`
}

type identicalResponseLedgerCaptureRun struct {
	Status          string `json:"status"`
	AuthorizedCalls int    `json:"authorized_calls"`
	ObservedCalls   int    `json:"observed_calls"`
}

type identicalResponseLedgerRoute struct {
	State    string `json:"state"`
	Identity struct {
		RouteID        string `json:"route_id"`
		ServedModel    string `json:"served_model"`
		ProviderID     string `json:"provider_id"`
		RequestedModel string `json:"requested_model"`
	} `json:"identity"`
}

type identicalResponseLedgerStudyRecord struct {
	State string `json:"state"`
}

// BuildIdenticalResponseLedger builds the v5 task ledger only from the sealed
// outer capsule. The function deliberately does not import replay types, so
// claim verification remains a dependency-free consumer of the capsule DAG.
func BuildIdenticalResponseLedger(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte) (Ledger, error) {
	if ctx == nil || registry == nil {
		return Ledger{}, errors.New("identical-response claim ledger requires context and capsule registry")
	}
	if _, err := capsule.VerifyPackage(ctx, registry, manifest, payloads, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}); err != nil {
		return Ledger{}, err
	}
	rootRecord, err := identicalResponseLedgerComponent(manifest, identicalResponseRootComponentName, identicalResponseRootTypeID)
	if err != nil {
		return Ledger{}, err
	}
	analysisRecord, err := identicalResponseLedgerComponent(manifest, identicalResponseAnalysisComponentName, identicalResponseAnalysisTypeID)
	if err != nil {
		return Ledger{}, err
	}
	admissionRecord, err := identicalResponseLedgerComponent(manifest, identicalResponseAdmissionComponentName, identicalResponseAdmissionTypeID)
	if err != nil {
		return Ledger{}, err
	}
	captureRunRecord, err := identicalResponseLedgerComponent(manifest, identicalResponseCaptureRunComponentName, identicalResponseCaptureRunTypeID)
	if err != nil {
		return Ledger{}, err
	}
	routeRecord, err := identicalResponseLedgerComponent(manifest, identicalResponseRouteComponentName, identicalResponseRouteTypeID)
	if err != nil {
		return Ledger{}, err
	}
	studyRecordRecord, err := identicalResponseLedgerComponent(manifest, identicalResponseStudyRecordComponentName, identicalResponseStudyRecordTypeID)
	if err != nil {
		return Ledger{}, err
	}
	var root identicalResponseLedgerRoot
	var analysis identicalResponseLedgerAnalysis
	var admission identicalResponseLedgerAdmission
	var captureRun identicalResponseLedgerCaptureRun
	var route identicalResponseLedgerRoute
	var studyRecord identicalResponseLedgerStudyRecord
	if err := decodeClaimProjection(payloads, rootRecord, &root); err != nil {
		return Ledger{}, err
	}
	if err := decodeClaimProjection(payloads, analysisRecord, &analysis); err != nil {
		return Ledger{}, err
	}
	if err := decodeClaimProjection(payloads, admissionRecord, &admission); err != nil {
		return Ledger{}, err
	}
	if err := decodeClaimProjection(payloads, captureRunRecord, &captureRun); err != nil {
		return Ledger{}, err
	}
	if err := decodeClaimProjection(payloads, routeRecord, &route); err != nil {
		return Ledger{}, err
	}
	if err := decodeClaimProjection(payloads, studyRecordRecord, &studyRecord); err != nil {
		return Ledger{}, err
	}
	if root.Status != "complete" || root.EvidenceCeiling != "record_complete_external_parents_resolved" || root.ProviderCalls != 0 || root.NetworkRequired || root.AuthorizedCalls != root.ObservedCalls || root.ObservedCalls != root.CompleteResearchEntries || root.ObservedCalls < 1 || root.AuthorizationDigest == "" || root.AnalysisDigest != analysis.Digest || root.StudyRecordDigest != analysis.StudyRecordDigest || analysis.StudyRecordDigest == "" || analysis.Digest == "" || analysis.Summary.TaskGroups < 1 || analysis.Summary.Complete+analysis.Summary.Unresolved != analysis.Summary.TaskGroups || analysis.Summary.Complete != analysis.Summary.Agreements+analysis.Summary.Disagreements || admission.Admission != "admitted" || admission.AuthorizedCalls != root.AuthorizedCalls || admission.ObservedCalls != root.ObservedCalls || admission.CompleteResearchEntries != root.CompleteResearchEntries || captureRun.Status != "complete" || captureRun.AuthorizedCalls != root.AuthorizedCalls || captureRun.ObservedCalls != root.ObservedCalls || route.State != "bounded_qualified" || route.Identity.RouteID == "" || route.Identity.ServedModel == "" || studyRecord.State != "complete" {
		return Ledger{}, errors.New("identical-response claim projections do not agree on the sealed study boundary")
	}
	scope := Scope{
		Routes: []string{route.Identity.RouteID}, Models: []string{route.Identity.ServedModel},
		Domains: []string{"identical-response"}, Tasks: []string{"TASK-070"},
		Policies: []string{"complete_research"}, TimeBounds: []string{}, Entrypoints: []string{"eval-terminal"},
	}
	routeEvidence := []struct {
		name   string
		typeID string
	}{
		{identicalResponseRouteComponentName, identicalResponseRouteTypeID},
	}
	claims := []Claim{
		defaultClaim("CLM-070", "The sealed TASK-070 capsule records an admitted complete-research capture", StatusSupported, EvidenceE2, scope, pointer(identicalResponseAdmissionComponentName, identicalResponseAdmissionTypeID, "/admission", StringValue("admitted")), "Bounded live route observation only; no provider endorsement, transfer, or quality claim", identicalResponseClaimGeneration, AttestationCurrent),
		defaultClaim("CLM-071", fmt.Sprintf("The sealed TASK-070 capture contains exactly %d authorized logical calls", root.ObservedCalls), StatusSupported, EvidenceE2, scope, pointer(identicalResponseCaptureRunComponentName, identicalResponseCaptureRunTypeID, "/observed_calls", NumberValue(strconv.Itoa(root.ObservedCalls))), "The count is exact for this sealed capture and does not establish a population call rate", identicalResponseClaimGeneration, AttestationCurrent),
		defaultClaim("CLM-072", fmt.Sprintf("The locked TASK-070 identical-response analysis records exactly %d disagreements in %d groups", analysis.Summary.Disagreements, analysis.Summary.TaskGroups), StatusSupported, EvidenceE2, scope, ratio(identicalResponseAnalysisComponentName, identicalResponseAnalysisTypeID, "/summary/disagreements", identicalResponseAnalysisComponentName, identicalResponseAnalysisTypeID, "/summary/task_groups", fmt.Sprintf("%d/%d", analysis.Summary.Disagreements, analysis.Summary.TaskGroups)), "Descriptive locked development-population result; no correctness, calibration, or superiority claim", identicalResponseClaimGeneration, AttestationCurrent),
		defaultClaim("CLM-073", fmt.Sprintf("The locked TASK-070 identical-response analysis records exactly %d agreement groups", analysis.Summary.Agreements), StatusExploratory, EvidenceE2, scope, pointer(identicalResponseAnalysisComponentName, identicalResponseAnalysisTypeID, "/summary/agreements", NumberValue(strconv.Itoa(analysis.Summary.Agreements))), "Agreement is a descriptive extraction-semantic result and does not imply correctness", identicalResponseClaimGeneration, AttestationCurrent),
		defaultClaim("CLM-074", fmt.Sprintf("Outcome sensitivity is available for exactly %d of %d groups and cannot establish correctness or model quality", analysis.OutcomeSensitivity.Covered, analysis.Summary.TaskGroups), StatusUnsupported, EvidenceE0, scope, pointer(identicalResponseAnalysisComponentName, identicalResponseAnalysisTypeID, "/outcome_sensitivity/covered", NumberValue(strconv.Itoa(analysis.OutcomeSensitivity.Covered))), "Outcome coverage is too sparse for a quality or correctness claim", identicalResponseClaimGeneration, AttestationUnavailable),
	}
	for index := range claims {
		for _, evidence := range routeEvidence {
			appendClaimEvidence(&claims[index], evidence.name, evidence.typeID)
		}
		appendClaimEvidence(&claims[index], identicalResponseRootComponentName, identicalResponseRootTypeID)
		normalizeClaim(&claims[index])
		if err := claims[index].Validate(); err != nil {
			return Ledger{}, err
		}
	}
	return SealLedger(manifest, claims)
}

func identicalResponseLedgerComponent(manifest capsule.Manifest, name, typeID string) (capsule.ComponentRecord, error) {
	var result capsule.ComponentRecord
	found := 0
	for _, record := range manifest.Components {
		if record.Name == name {
			result, found = record, found+1
		}
	}
	if found != 1 || result.TypeID != typeID {
		return capsule.ComponentRecord{}, fmt.Errorf("identical-response claim ledger requires exactly one %q component of type %q", name, typeID)
	}
	return result, nil
}

func decodeClaimProjection(payloads map[string][]byte, record capsule.ComponentRecord, target any) error {
	payload, found := payloads[record.Payload.Digest]
	if !found {
		return fmt.Errorf("claim projection payload %q is unavailable", record.Payload.Digest)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode claim projection %q: %w", record.Name, err)
	}
	return nil
}

func appendClaimEvidence(item *Claim, component, typeID string) {
	for _, existing := range item.EvidenceComponents {
		if existing == component {
			return
		}
	}
	item.EvidenceComponents = append(item.EvidenceComponents, component)
	item.EvidenceCeiling.RequiredParentTypes = append(item.EvidenceCeiling.RequiredParentTypes, typeID)
}
