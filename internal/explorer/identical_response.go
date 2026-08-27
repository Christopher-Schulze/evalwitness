package explorer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/replay"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	IdenticalResponseViewSchemaVersion = "evalwitness.identical-response-explorer.v1"
	identicalResponseReproductionPath  = "eval/governance/identical-response-reproduction-report-v5.json"
	identicalResponseReproductionCmd   = "scripts/evals/reproduce-identical-response-v5.sh"
)

type IdenticalResponseClaimView struct {
	ClaimID            string   `json:"claim_id"`
	Statement          string   `json:"statement"`
	Status             string   `json:"status"`
	EvidenceLevel      string   `json:"evidence_level"`
	EvidenceComponents []string `json:"evidence_components"`
	Caveats            []string `json:"caveats"`
	ChallengeReceipts  int      `json:"challenge_receipts"`
}

type IdenticalResponseDecisionView struct {
	State         string `json:"state"`
	SelectedIndex *int   `json:"selected_index,omitempty"`
	Margin        string `json:"margin"`
}

type IdenticalResponseFailureView struct {
	GroupID           string                        `json:"group_id"`
	TaskID            string                        `json:"task_id"`
	ResponseDigest    string                        `json:"response_digest"`
	RecordDigest      string                        `json:"record_digest"`
	DistributionAware IdenticalResponseDecisionView `json:"distribution_aware"`
	ChosenToken       IdenticalResponseDecisionView `json:"chosen_token"`
}

type IdenticalResponseComparisonView struct {
	TaskGroups            int    `json:"task_groups"`
	Complete              int    `json:"complete"`
	Agreements            int    `json:"agreements"`
	Disagreements         int    `json:"disagreements"`
	Unresolved            int    `json:"unresolved"`
	DisagreementRate      string `json:"disagreement_rate"`
	OutcomeCovered        int    `json:"outcome_covered"`
	OutcomeCoverageRate   string `json:"outcome_coverage_rate"`
	MissingnessUpperBound string `json:"missingness_upper_bound"`
}

type IdenticalResponseChallengeReceiptView struct {
	ClaimID       string `json:"claim_id"`
	ChallengeID   string `json:"challenge_id"`
	Class         string `json:"class"`
	Mutation      string `json:"mutation"`
	ObservedGuard string `json:"observed_guard"`
	BeforeState   string `json:"before_state"`
	AfterState    string `json:"after_state"`
	Digest        string `json:"digest"`
	Passed        bool   `json:"passed"`
}

type IdenticalResponseChallengeView struct {
	PackDigest  string                                  `json:"pack_digest"`
	Total       int                                     `json:"total"`
	ClassCounts []CountView                             `json:"class_counts"`
	Receipts    []IdenticalResponseChallengeReceiptView `json:"receipts"`
}

type IdenticalResponseReproductionView struct {
	Command           string      `json:"command"`
	Status            string      `json:"status"`
	SourceCommit      string      `json:"source_commit"`
	CleanCloneProof   bool        `json:"clean_clone_proof"`
	Network           string      `json:"network"`
	ProviderCalls     int         `json:"provider_calls"`
	ArtifactsVerified int         `json:"artifacts_verified"`
	Source            ArtifactRef `json:"source"`
}

type IdenticalResponseView struct {
	SchemaVersion   string                            `json:"schema_version"`
	Availability    Availability                      `json:"availability"`
	CapsuleID       string                            `json:"capsule_id"`
	ManifestDigest  string                            `json:"manifest_digest"`
	BaseCapsuleID   string                            `json:"base_capsule_id"`
	LedgerDigest    string                            `json:"ledger_digest"`
	EvidenceCeiling string                            `json:"evidence_ceiling"`
	Status          string                            `json:"status"`
	Method          string                            `json:"method"`
	RouteID         string                            `json:"route_id"`
	Model           string                            `json:"model"`
	ObservedCalls   int                               `json:"observed_calls"`
	Comparison      IdenticalResponseComparisonView   `json:"comparison"`
	Claims          []IdenticalResponseClaimView      `json:"claims"`
	Failures        []IdenticalResponseFailureView    `json:"failures"`
	Challenges      IdenticalResponseChallengeView    `json:"challenges"`
	Limitations     []string                          `json:"limitations"`
	Reproduction    IdenticalResponseReproductionView `json:"reproduction"`
	Source          ArtifactRef                       `json:"source"`
	AnalysisSource  ArtifactRef                       `json:"analysis_source"`
	Digest          string                            `json:"digest"`
}

type identicalResponseReproductionReport struct {
	SchemaVersion   string `json:"schema_version"`
	SourceCommit    string `json:"source_commit"`
	Status          string `json:"status"`
	CleanCloneProof bool   `json:"clean_clone_proof"`
	Environment     struct {
		Network       string `json:"network"`
		ProviderCalls int    `json:"provider_calls"`
	} `json:"environment"`
	RegisteredArtifacts struct {
		Count               int  `json:"count"`
		ByteHashesIdentical bool `json:"byte_hashes_identical"`
	} `json:"registered_artifacts"`
	Capsule struct {
		CapsuleID       string `json:"capsule_id"`
		ManifestDigest  string `json:"manifest_digest"`
		EvidenceCeiling string `json:"evidence_ceiling"`
		Valid           bool   `json:"valid"`
	} `json:"capsule"`
	Claims struct {
		LedgerDigest string `json:"ledger_digest"`
		Valid        bool   `json:"valid"`
	} `json:"claims"`
	Challenges struct {
		PackDigest string `json:"pack_digest"`
		Receipts   int    `json:"receipts"`
		AllPassed  bool   `json:"all_passed"`
	} `json:"challenges"`
	Limitations []string `json:"limitations"`
}

func AddIdenticalResponse(
	ctx context.Context,
	report Report,
	base capsule.Package,
	child capsule.Package,
	ledger claimledger.Ledger,
	pack claimledger.ChallengePack,
	reproductionRaw []byte,
) (Report, error) {
	if ctx == nil || len(reproductionRaw) == 0 {
		return Report{}, errors.New("identical-response explorer requires context and reproduction evidence")
	}
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	if report.IdenticalResponse != nil {
		return Report{}, errors.New("evidence explorer already contains identical-response evidence")
	}
	view, err := buildIdenticalResponseView(ctx, base, child, ledger, pack, reproductionRaw)
	if err != nil {
		return Report{}, err
	}
	report.IdenticalResponse, report.Digest = &view, ""
	return sealReport(report)
}

func buildIdenticalResponseView(
	ctx context.Context,
	base capsule.Package,
	child capsule.Package,
	ledger claimledger.Ledger,
	pack claimledger.ChallengePack,
	reproductionRaw []byte,
) (IdenticalResponseView, error) {
	if _, err := replay.VerifyIdenticalResponseCapsuleFamily(ctx, child, base); err != nil {
		return IdenticalResponseView{}, fmt.Errorf("verify identical-response capsule family: %w", err)
	}
	if _, err := claimledger.VerifyLedger(ctx, child.Registry, child.Manifest, child.Payloads, ledger); err != nil {
		return IdenticalResponseView{}, fmt.Errorf("verify identical-response claim ledger: %w", err)
	}
	if err := validateChallengePackBinding(ctx, pack, child, ledger); err != nil {
		return IdenticalResponseView{}, err
	}
	root, rootRecord, err := decodeIdenticalResponseComponent[replay.IdenticalResponseEvidenceRoot](child, replay.IdenticalResponseOuterBindSchemaVersion)
	if err != nil {
		return IdenticalResponseView{}, err
	}
	analysis, analysisRecord, err := decodeIdenticalResponseComponent[replay.IdenticalResponseAnalysis](child, replay.IdenticalResponseAnalysisComponentSchemaVersion)
	if err != nil {
		return IdenticalResponseView{}, err
	}
	return assembleIdenticalResponseView(root, rootRecord, analysis, analysisRecord, child, ledger, pack, reproductionRaw)
}

func validateChallengePackBinding(ctx context.Context, pack claimledger.ChallengePack, child capsule.Package, ledger claimledger.Ledger) error {
	if err := pack.Validate(); err != nil {
		return fmt.Errorf("verify identical-response challenge pack: %w", err)
	}
	if pack.SourceCapsuleID != child.Manifest.CapsuleID || pack.LedgerDigest != ledger.Digest {
		return errors.New("identical-response challenge pack is detached from capsule or ledger")
	}
	expected, err := claimledger.BuildChallengePack(ctx, child.Registry, child.Manifest, child.Payloads, ledger)
	if err != nil {
		return fmt.Errorf("rebuild identical-response challenge pack: %w", err)
	}
	if expected.Digest != pack.Digest {
		return errors.New("identical-response challenge pack differs from executable ledger challenges")
	}
	return nil
}

func decodeIdenticalResponseComponent[T any](pack capsule.Package, typeID string) (T, capsule.ComponentRecord, error) {
	var zero T
	record, err := uniqueComponentByType(pack.Manifest, typeID)
	if err != nil {
		return zero, capsule.ComponentRecord{}, err
	}
	raw, found := pack.Payloads[record.Payload.Digest]
	if !found {
		return zero, capsule.ComponentRecord{}, fmt.Errorf("identical-response component payload %q is unavailable", record.Payload.Digest)
	}
	var value T
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return zero, capsule.ComponentRecord{}, fmt.Errorf("decode identical-response component %q: %w", record.Name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return zero, capsule.ComponentRecord{}, fmt.Errorf("decode identical-response component %q: trailing JSON", record.Name)
	}
	return value, record, nil
}

func uniqueComponentByType(manifest capsule.Manifest, typeID string) (capsule.ComponentRecord, error) {
	var result capsule.ComponentRecord
	found := 0
	for _, record := range manifest.Components {
		if record.TypeID == typeID {
			result, found = record, found+1
		}
	}
	if found != 1 {
		return capsule.ComponentRecord{}, fmt.Errorf("identical-response capsule requires exactly one component of type %q", typeID)
	}
	return result, nil
}

func assembleIdenticalResponseView(
	root replay.IdenticalResponseEvidenceRoot,
	rootRecord capsule.ComponentRecord,
	analysis replay.IdenticalResponseAnalysis,
	analysisRecord capsule.ComponentRecord,
	child capsule.Package,
	ledger claimledger.Ledger,
	pack claimledger.ChallengePack,
	reproductionRaw []byte,
) (IdenticalResponseView, error) {
	if err := root.Validate(); err != nil {
		return IdenticalResponseView{}, err
	}
	if err := analysis.Validate(); err != nil {
		return IdenticalResponseView{}, err
	}
	if err := validateIdenticalResponseBindings(root, analysis, child, ledger); err != nil {
		return IdenticalResponseView{}, err
	}
	reproduction, reproductionLimitations, err := buildIdenticalResponseReproduction(reproductionRaw, child, ledger, pack)
	if err != nil {
		return IdenticalResponseView{}, err
	}
	routeID, model, err := identicalResponseScope(ledger.Claims)
	if err != nil {
		return IdenticalResponseView{}, err
	}
	view := IdenticalResponseView{
		SchemaVersion: IdenticalResponseViewSchemaVersion, Availability: AvailabilityAvailable,
		CapsuleID: child.Manifest.CapsuleID, ManifestDigest: child.Manifest.ManifestDigest,
		BaseCapsuleID: root.BaseCapsuleID, LedgerDigest: ledger.Digest,
		EvidenceCeiling: root.EvidenceCeiling, Status: root.Status,
		Method: LockedIdenticalResponseMethod, RouteID: routeID, Model: model, ObservedCalls: root.ObservedCalls,
		Comparison: comparisonView(analysis), Claims: claimViews(ledger, pack),
		Failures: failureViews(analysis), Challenges: challengeView(pack),
		Limitations:    mergedLimitations(root.Limitations, ledger.Claims, reproductionLimitations),
		Reproduction:   reproduction,
		Source:         componentArtifactRef(rootRecord, root.Digest),
		AnalysisSource: componentArtifactRef(analysisRecord, analysis.Digest),
	}
	view.Digest, err = identicalResponseViewDigest(view)
	if err != nil {
		return IdenticalResponseView{}, err
	}
	if err := view.Validate(); err != nil {
		return IdenticalResponseView{}, err
	}
	return view, nil
}

const LockedIdenticalResponseMethod = "distribution_aware_vs_chosen_token on the same immutable completion"

func validateIdenticalResponseBindings(root replay.IdenticalResponseEvidenceRoot, analysis replay.IdenticalResponseAnalysis, child capsule.Package, ledger claimledger.Ledger) error {
	if root.BaseCapsuleID == "" || root.AnalysisDigest != analysis.Digest ||
		root.AnalysisSummary.TaskGroups != analysis.Summary.TaskGroups ||
		root.AnalysisSummary.Complete != analysis.Summary.Complete ||
		root.AnalysisSummary.Agreements != analysis.Summary.Agreements ||
		root.AnalysisSummary.Disagreements != analysis.Summary.Disagreements ||
		root.AnalysisSummary.Unresolved != analysis.Summary.Unresolved ||
		ledger.CapsuleID != child.Manifest.CapsuleID || ledger.ManifestDigest != child.Manifest.ManifestDigest {
		return errors.New("identical-response explorer inputs disagree on sealed identities or denominators")
	}
	return nil
}

func identicalResponseScope(claims []claimledger.Claim) (string, string, error) {
	if len(claims) == 0 || len(claims[0].Scope.Routes) != 1 || len(claims[0].Scope.Models) != 1 {
		return "", "", errors.New("identical-response claim scope lacks one route and model")
	}
	routeID, model := claims[0].Scope.Routes[0], claims[0].Scope.Models[0]
	for _, item := range claims[1:] {
		if !slices.Equal(item.Scope.Routes, []string{routeID}) || !slices.Equal(item.Scope.Models, []string{model}) {
			return "", "", errors.New("identical-response claims disagree on route or model scope")
		}
	}
	return routeID, model, nil
}

func comparisonView(analysis replay.IdenticalResponseAnalysis) IdenticalResponseComparisonView {
	return IdenticalResponseComparisonView{
		TaskGroups: analysis.Summary.TaskGroups, Complete: analysis.Summary.Complete,
		Agreements: analysis.Summary.Agreements, Disagreements: analysis.Summary.Disagreements,
		Unresolved: analysis.Summary.Unresolved, DisagreementRate: fmt.Sprintf("%.12g", analysis.Summary.DisagreementRate),
		OutcomeCovered:        analysis.OutcomeSensitivity.Covered,
		OutcomeCoverageRate:   fmt.Sprintf("%.12g", analysis.OutcomeSensitivity.CoverageRate),
		MissingnessUpperBound: fmt.Sprintf("%.12g", analysis.Summary.MissingnessWorstCaseExactUpperBound),
	}
}

func claimViews(ledger claimledger.Ledger, pack claimledger.ChallengePack) []IdenticalResponseClaimView {
	counts := make(map[string]int)
	for _, receipt := range pack.Receipts {
		counts[receipt.ClaimID]++
	}
	views := make([]IdenticalResponseClaimView, len(ledger.Claims))
	for index, item := range ledger.Claims {
		views[index] = IdenticalResponseClaimView{
			ClaimID: item.ClaimID, Statement: item.TextTemplate, Status: string(item.Status),
			EvidenceLevel: string(item.EvidenceLevel), EvidenceComponents: slices.Clone(item.EvidenceComponents),
			Caveats: slices.Clone(item.Caveats), ChallengeReceipts: counts[item.ClaimID],
		}
	}
	return views
}

func failureViews(analysis replay.IdenticalResponseAnalysis) []IdenticalResponseFailureView {
	views := make([]IdenticalResponseFailureView, 0, analysis.Summary.Disagreements)
	for _, row := range analysis.Rows {
		if row.Status != "disagreement" {
			continue
		}
		views = append(views, IdenticalResponseFailureView{
			GroupID: row.GroupID, TaskID: row.TaskID, ResponseDigest: row.ResponseBodyDigest,
			RecordDigest: row.RecordDigest, DistributionAware: decisionView(row.DistributionAware),
			ChosenToken: decisionView(row.ChosenToken),
		})
	}
	return views
}

func decisionView(decision replay.IdenticalResponseDecision) IdenticalResponseDecisionView {
	return IdenticalResponseDecisionView{
		State: decision.State, SelectedIndex: decision.SelectedIndex,
		Margin: fmt.Sprintf("%.12g", decision.Margin),
	}
}

func challengeView(pack claimledger.ChallengePack) IdenticalResponseChallengeView {
	classes := make([]string, 0, len(pack.ClassCounts))
	for class := range pack.ClassCounts {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	counts := make([]CountView, len(classes))
	for index, class := range classes {
		counts[index] = CountView{Name: class, Value: pack.ClassCounts[class]}
	}
	receipts := make([]IdenticalResponseChallengeReceiptView, len(pack.Receipts))
	for index, receipt := range pack.Receipts {
		receipts[index] = IdenticalResponseChallengeReceiptView{
			ClaimID: receipt.ClaimID, ChallengeID: receipt.ChallengeID, Class: string(receipt.Class),
			Mutation: receipt.Mutation, ObservedGuard: string(receipt.ObservedGuard),
			BeforeState: receipt.BeforeState, AfterState: receipt.AfterState,
			Digest: receipt.Digest, Passed: receipt.Passed,
		}
	}
	return IdenticalResponseChallengeView{PackDigest: pack.Digest, Total: len(pack.Receipts), ClassCounts: counts, Receipts: receipts}
}

func mergedLimitations(root []string, claims []claimledger.Claim, reproduction []string) []string {
	values := append(slices.Clone(root), reproduction...)
	for _, item := range claims {
		values = append(values, item.Caveats...)
	}
	sort.Strings(values)
	return slices.Compact(values)
}

func buildIdenticalResponseReproduction(raw []byte, child capsule.Package, ledger claimledger.Ledger, pack claimledger.ChallengePack) (IdenticalResponseReproductionView, []string, error) {
	var report identicalResponseReproductionReport
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&report); err != nil {
		return IdenticalResponseReproductionView{}, nil, fmt.Errorf("decode identical-response reproduction report: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return IdenticalResponseReproductionView{}, nil, errors.New("identical-response reproduction report has trailing JSON")
	}
	if err := validateIdenticalResponseReproduction(report, child, ledger, pack); err != nil {
		return IdenticalResponseReproductionView{}, nil, err
	}
	payloadDigest := sha256.Sum256(raw)
	digest := hex.EncodeToString(payloadDigest[:])
	return IdenticalResponseReproductionView{
		Command: identicalResponseReproductionCmd, Status: report.Status,
		SourceCommit: report.SourceCommit, CleanCloneProof: report.CleanCloneProof,
		Network: report.Environment.Network, ProviderCalls: report.Environment.ProviderCalls,
		ArtifactsVerified: report.RegisteredArtifacts.Count,
		Source: ArtifactRef{Kind: ArtifactReleaseFile, ID: identicalResponseReproductionPath,
			SchemaVersion: report.SchemaVersion, PayloadSHA256: digest, ArtifactDigest: digest},
	}, slices.Clone(report.Limitations), nil
}

func validateIdenticalResponseReproduction(report identicalResponseReproductionReport, child capsule.Package, ledger claimledger.Ledger, pack claimledger.ChallengePack) error {
	if report.SchemaVersion != "evalwitness.identical-response-reproduction-report.v1" ||
		report.Status != "passed" || len(report.SourceCommit) != 40 || !report.CleanCloneProof ||
		report.Environment.Network != "denied" || report.Environment.ProviderCalls != 0 ||
		report.RegisteredArtifacts.Count < 1 || !report.RegisteredArtifacts.ByteHashesIdentical ||
		!report.Capsule.Valid || report.Capsule.CapsuleID != child.Manifest.CapsuleID ||
		report.Capsule.ManifestDigest != child.Manifest.ManifestDigest ||
		report.Capsule.EvidenceCeiling != replay.IdenticalResponseEvidenceCeiling ||
		!report.Claims.Valid || report.Claims.LedgerDigest != ledger.Digest ||
		!report.Challenges.AllPassed || report.Challenges.PackDigest != pack.Digest ||
		report.Challenges.Receipts != len(pack.Receipts) || len(report.Limitations) == 0 {
		return errors.New("identical-response reproduction report is incomplete or detached")
	}
	return nil
}

func (view IdenticalResponseView) Validate() error {
	if view.SchemaVersion != IdenticalResponseViewSchemaVersion || view.Availability != AvailabilityAvailable ||
		!validDigest(view.CapsuleID) || !validDigest(view.ManifestDigest) || !validDigest(view.BaseCapsuleID) ||
		!validDigest(view.LedgerDigest) || view.EvidenceCeiling != replay.IdenticalResponseEvidenceCeiling ||
		view.Status != "complete" || view.Method != LockedIdenticalResponseMethod ||
		!strings.HasPrefix(view.RouteID, "route-") || view.Model == "" || view.ObservedCalls < 1 ||
		validateArtifactRef(view.Source) != nil || view.Source.SchemaVersion != replay.IdenticalResponseOuterBindSchemaVersion ||
		validateArtifactRef(view.AnalysisSource) != nil || view.AnalysisSource.SchemaVersion != replay.IdenticalResponseAnalysisComponentSchemaVersion ||
		!validDigest(view.Digest) {
		return errors.New("identical-response explorer identity is invalid")
	}
	if err := view.validateComparison(); err != nil {
		return err
	}
	if err := view.validateClaimsAndChallenges(); err != nil {
		return err
	}
	if err := view.validateReproduction(); err != nil {
		return err
	}
	if err := validateSortedUnique("identical-response limitations", view.Limitations); err != nil {
		return err
	}
	digest, err := identicalResponseViewDigest(view)
	if err != nil || digest != view.Digest {
		return errors.New("identical-response explorer digest is invalid")
	}
	return nil
}

func (view IdenticalResponseView) validateComparison() error {
	comparison := view.Comparison
	if comparison.TaskGroups < 1 || comparison.Complete+comparison.Unresolved != comparison.TaskGroups ||
		comparison.Agreements+comparison.Disagreements != comparison.Complete ||
		comparison.Disagreements != len(view.Failures) || comparison.OutcomeCovered < 0 ||
		comparison.OutcomeCovered > comparison.TaskGroups ||
		!validRatio(comparison.DisagreementRate, comparison.Disagreements, comparison.TaskGroups) ||
		!validRatio(comparison.OutcomeCoverageRate, comparison.OutcomeCovered, comparison.TaskGroups) ||
		!validUnitInterval(comparison.MissingnessUpperBound) {
		return errors.New("identical-response comparison is invalid")
	}
	previous := ""
	for _, failure := range view.Failures {
		if failure.GroupID <= previous || failure.TaskID == "" || !validDigest(failure.ResponseDigest) ||
			!validDigest(failure.RecordDigest) || !validDecisionView(failure.DistributionAware) ||
			!validDecisionView(failure.ChosenToken) {
			return errors.New("identical-response failure gallery is invalid or unordered")
		}
		previous = failure.GroupID
	}
	return nil
}

func validUnitInterval(value string) bool {
	parsed, err := strconv.ParseFloat(value, 64)
	return err == nil && parsed >= 0 && parsed <= 1
}

func validRatio(value string, numerator int, denominator int) bool {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || denominator < 1 || numerator < 0 || numerator > denominator {
		return false
	}
	return math.Abs(parsed-float64(numerator)/float64(denominator)) <= 1e-11
}

func validDecisionView(view IdenticalResponseDecisionView) bool {
	if view.State != replay.DecisionSelected && view.State != replay.DecisionTied && view.State != replay.DecisionMissing {
		return false
	}
	margin, err := strconv.ParseFloat(view.Margin, 64)
	return err == nil && margin >= 0 && (view.SelectedIndex != nil) == (view.State == replay.DecisionSelected)
}

func (view IdenticalResponseView) validateClaimsAndChallenges() error {
	if len(view.Claims) == 0 || view.Challenges.Total != len(view.Challenges.Receipts) ||
		!validDigest(view.Challenges.PackDigest) || validateCounts(view.Challenges.ClassCounts) != nil {
		return errors.New("identical-response claims or challenge summary is invalid")
	}
	previous := ""
	claimCounts := make(map[string]int)
	classCounts := make(map[string]int)
	for _, receipt := range view.Challenges.Receipts {
		if !receipt.Passed || receipt.ClaimID == "" || receipt.ChallengeID == "" || receipt.Class == "" ||
			receipt.Mutation == "" || receipt.ObservedGuard == "" || receipt.BeforeState == "" ||
			receipt.AfterState == "" || !validDigest(receipt.Digest) {
			return errors.New("identical-response challenge receipt is invalid")
		}
		claimCounts[receipt.ClaimID]++
		classCounts[receipt.Class]++
	}
	for _, count := range view.Challenges.ClassCounts {
		if classCounts[count.Name] != count.Value {
			return errors.New("identical-response challenge class counts do not match receipts")
		}
		delete(classCounts, count.Name)
	}
	if len(classCounts) != 0 {
		return errors.New("identical-response challenge receipts contain an uncounted class")
	}
	for _, item := range view.Claims {
		if item.ClaimID <= previous || item.Statement == "" || item.Status == "" || item.EvidenceLevel == "" ||
			len(item.EvidenceComponents) == 0 || item.Caveats == nil || item.ChallengeReceipts != claimCounts[item.ClaimID] {
			return errors.New("identical-response claim chain is invalid or unordered")
		}
		previous = item.ClaimID
		delete(claimCounts, item.ClaimID)
	}
	if len(claimCounts) != 0 {
		return errors.New("identical-response challenge receipts reference an unknown claim")
	}
	return nil
}

func (view IdenticalResponseView) validateReproduction() error {
	reproduction := view.Reproduction
	if reproduction.Command != identicalResponseReproductionCmd || reproduction.Status != "passed" ||
		len(reproduction.SourceCommit) != 40 || !reproduction.CleanCloneProof ||
		reproduction.Network != "denied" || reproduction.ProviderCalls != 0 ||
		reproduction.ArtifactsVerified < 1 || validateArtifactRef(reproduction.Source) != nil ||
		reproduction.Source.ID != identicalResponseReproductionPath {
		return errors.New("identical-response reproduction view is invalid")
	}
	return nil
}

func identicalResponseViewDigest(view IdenticalResponseView) (string, error) {
	view.Digest = ""
	return protocol.Digest(view)
}

func (report Report) validateIdenticalResponse() error {
	if report.IdenticalResponse == nil {
		return nil
	}
	return report.IdenticalResponse.Validate()
}
