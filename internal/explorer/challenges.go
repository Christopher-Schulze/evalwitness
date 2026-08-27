package explorer

import (
	"errors"
	"fmt"
	"strings"

	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

type ChallengeReceiptView struct {
	SchemaVersion           string `json:"schema_version"`
	Verifier                string `json:"verifier"`
	ClaimID                 string `json:"claim_id"`
	SourceCapsuleID         string `json:"source_capsule_id"`
	SourceManifestDigest    string `json:"source_manifest_digest"`
	SourceLedgerDigest      string `json:"source_ledger_digest"`
	ChallengeID             string `json:"challenge_id"`
	Class                   string `json:"class"`
	Mutation                string `json:"mutation"`
	ExpectedGuard           string `json:"expected_guard"`
	ObservedGuard           string `json:"observed_guard"`
	BeforeState             string `json:"before_state"`
	AfterState              string `json:"after_state"`
	SealedSourceDigest      string `json:"sealed_source_digest"`
	AfterSealedSourceDigest string `json:"after_sealed_source_digest"`
	Passed                  bool   `json:"passed"`
	Digest                  string `json:"digest"`
	ReplayCommand           string `json:"replay_command"`
}

type ChallengeClassView struct {
	Class              string                `json:"class"`
	Availability       Availability          `json:"availability"`
	GlobalReceiptCount int                   `json:"global_receipt_count"`
	InapplicableReason string                `json:"inapplicable_reason"`
	Receipt            *ChallengeReceiptView `json:"receipt"`
}

type ChallengeView struct {
	ClaimID              string               `json:"claim_id"`
	ClaimText            string               `json:"claim_text"`
	ClaimStatus          string               `json:"claim_status"`
	EvidenceLevel        string               `json:"evidence_level"`
	TotalReceipts        int                  `json:"total_receipts"`
	SelectedReceipts     int                  `json:"selected_receipts"`
	SourceCapsuleID      string               `json:"source_capsule_id"`
	SourceManifestDigest string               `json:"source_manifest_digest"`
	SourceLedgerDigest   string               `json:"source_ledger_digest"`
	PackDigest           string               `json:"pack_digest"`
	Classes              []ChallengeClassView `json:"classes"`
	Source               ArtifactRef          `json:"source"`
}

func buildChallengeView(ledger claimledger.Ledger, pack claimledger.ChallengePack) (ChallengeView, error) {
	claimID, selectedCount, err := selectChallengeClaim(pack.Receipts)
	if err != nil {
		return ChallengeView{}, err
	}
	item, err := claimledger.Lookup(ledger, claimID)
	if err != nil {
		return ChallengeView{}, err
	}
	classes, manifestDigest, err := buildChallengeClasses(item, pack)
	if err != nil {
		return ChallengeView{}, err
	}
	raw, err := protocol.CanonicalMarshal(pack)
	if err != nil {
		return ChallengeView{}, err
	}
	return ChallengeView{
		ClaimID: item.ClaimID, ClaimText: item.TextTemplate, ClaimStatus: string(item.Status),
		EvidenceLevel: string(item.EvidenceLevel), TotalReceipts: len(pack.Receipts), SelectedReceipts: selectedCount,
		SourceCapsuleID: pack.SourceCapsuleID, SourceManifestDigest: manifestDigest,
		SourceLedgerDigest: pack.LedgerDigest, PackDigest: pack.Digest, Classes: classes,
		Source: sidecarArtifactRef("claim-challenge-pack", claimledger.ChallengePackSchemaVersion, raw, pack.Digest),
	}, nil
}

func selectChallengeClaim(receipts []claimledger.ChallengeReceipt) (string, int, error) {
	counts := make(map[string]int)
	for _, receipt := range receipts {
		counts[receipt.ClaimID]++
	}
	selected := ""
	selectedCount := 0
	for claimID, count := range counts {
		if count > selectedCount || count == selectedCount && (selected == "" || claimID < selected) {
			selected, selectedCount = claimID, count
		}
	}
	if selected == "" {
		return "", 0, errors.New("evidence explorer challenge pack contains no receipts")
	}
	return selected, selectedCount, nil
}

func buildChallengeClasses(item claimledger.Claim, pack claimledger.ChallengePack) ([]ChallengeClassView, string, error) {
	receipts := challengeReceiptsByClass(pack.Receipts, item.ClaimID)
	classes := make([]ChallengeClassView, len(item.Challenges))
	manifestDigest := ""
	for index, specification := range item.Challenges {
		view, err := buildChallengeClassView(specification, receipts[string(specification.Class)], pack.ClassCounts)
		if err != nil {
			return nil, "", err
		}
		classes[index] = view
		if view.Receipt != nil {
			manifestDigest = view.Receipt.SourceManifestDigest
		}
	}
	if manifestDigest == "" {
		return nil, "", errors.New("evidence explorer selected challenge claim has no receipt")
	}
	return classes, manifestDigest, nil
}

func challengeReceiptsByClass(receipts []claimledger.ChallengeReceipt, claimID string) map[string]claimledger.ChallengeReceipt {
	indexed := make(map[string]claimledger.ChallengeReceipt)
	for _, receipt := range receipts {
		if receipt.ClaimID == claimID {
			indexed[string(receipt.Class)] = receipt
		}
	}
	return indexed
}

func buildChallengeClassView(
	specification claimledger.ChallengeSpec,
	receipt claimledger.ChallengeReceipt,
	counts map[string]int,
) (ChallengeClassView, error) {
	view := ChallengeClassView{Class: string(specification.Class), GlobalReceiptCount: counts[string(specification.Class)]}
	if specification.Applicability == claimledger.ChallengeInapplicable {
		view.Availability = AvailabilityNotApplicable
		view.InapplicableReason = specification.InapplicableReason
		return view, nil
	}
	if receipt.ChallengeID == "" {
		return ChallengeClassView{}, fmt.Errorf("evidence explorer challenge %q lacks its verified receipt", specification.ChallengeID)
	}
	view.Availability = AvailabilityAvailable
	projected := projectChallengeReceipt(receipt)
	view.Receipt = &projected
	return view, nil
}

func projectChallengeReceipt(receipt claimledger.ChallengeReceipt) ChallengeReceiptView {
	return ChallengeReceiptView{
		SchemaVersion: receipt.SchemaVersion, Verifier: receipt.Verifier, ClaimID: receipt.ClaimID,
		SourceCapsuleID: receipt.SourceCapsuleID, SourceManifestDigest: receipt.SourceManifestDigest,
		SourceLedgerDigest: receipt.SourceLedgerDigest, ChallengeID: receipt.ChallengeID, Class: string(receipt.Class),
		Mutation: receipt.Mutation, ExpectedGuard: string(receipt.ExpectedGuard), ObservedGuard: string(receipt.ObservedGuard),
		BeforeState: receipt.BeforeState, AfterState: receipt.AfterState, SealedSourceDigest: receipt.SealedSourceDigest,
		AfterSealedSourceDigest: receipt.AfterSealedSourceDigest, Passed: receipt.Passed, Digest: receipt.Digest,
		ReplayCommand: challengeReplayCommand(receipt.ClaimID, receipt.ChallengeID),
	}
}

func challengeReplayCommand(claimID string, challengeID string) string {
	return fmt.Sprintf("evalwitness claim challenge --capsule \"$CAPSULE\" --ledger \"$LEDGER\" --claim %s --challenge %s", claimID, challengeID)
}

func (report Report) validateChallenge() error {
	challenge := report.Challenge
	if challenge.ClaimID == "" || challenge.ClaimText == "" || challenge.ClaimStatus == "" || challenge.EvidenceLevel == "" ||
		challenge.TotalReceipts < challenge.SelectedReceipts || challenge.SelectedReceipts < 1 ||
		challenge.SourceCapsuleID != report.Capsule.CapsuleID || challenge.SourceLedgerDigest != report.Capsule.LedgerDigest ||
		challenge.SourceManifestDigest != report.Capsule.ManifestDigest || challenge.Source.ArtifactDigest != challenge.PackDigest ||
		validateArtifactRef(challenge.Source) != nil || challenge.Source.Kind != ArtifactSidecar ||
		challenge.Source.ID != "claim-challenge-pack" || challenge.Source.SchemaVersion != claimledger.ChallengePackSchemaVersion {
		return errors.New("evidence explorer challenge view identity is invalid")
	}
	return validateChallengeClasses(challenge)
}

func validateChallengeClasses(challenge ChallengeView) error {
	classes := claimledger.ChallengeClasses()
	if len(challenge.Classes) != len(classes) {
		return errors.New("evidence explorer challenge registry is incomplete")
	}
	selectedReceipts := 0
	totalReceipts := 0
	for index, class := range classes {
		view := challenge.Classes[index]
		if view.Class != string(class) || view.GlobalReceiptCount < 0 {
			return errors.New("evidence explorer challenge registry is unordered or invalid")
		}
		totalReceipts += view.GlobalReceiptCount
		if err := validateChallengeClass(view, challenge); err != nil {
			return err
		}
		if view.Receipt != nil {
			selectedReceipts++
		}
	}
	if totalReceipts != challenge.TotalReceipts || selectedReceipts != challenge.SelectedReceipts {
		return errors.New("evidence explorer challenge receipt counts are inconsistent")
	}
	return nil
}

func validateChallengeClass(view ChallengeClassView, challenge ChallengeView) error {
	if view.Availability == AvailabilityNotApplicable {
		if view.Receipt != nil || strings.TrimSpace(view.InapplicableReason) == "" {
			return errors.New("evidence explorer inapplicable challenge is incomplete")
		}
		return nil
	}
	if view.Availability != AvailabilityAvailable || view.InapplicableReason != "" || view.Receipt == nil {
		return errors.New("evidence explorer available challenge is incomplete")
	}
	return validateChallengeReceipt(*view.Receipt, view.Class, challenge)
}

func validateChallengeReceipt(receipt ChallengeReceiptView, class string, challenge ChallengeView) error {
	domain := claimledger.ChallengeReceipt{
		SchemaVersion: receipt.SchemaVersion, Verifier: receipt.Verifier, ClaimID: receipt.ClaimID,
		SourceCapsuleID: receipt.SourceCapsuleID, SourceManifestDigest: receipt.SourceManifestDigest,
		SourceLedgerDigest: receipt.SourceLedgerDigest, ChallengeID: receipt.ChallengeID,
		Class: claimledger.ChallengeClass(receipt.Class), Mutation: receipt.Mutation,
		ExpectedGuard: claimledger.Guard(receipt.ExpectedGuard), ObservedGuard: claimledger.Guard(receipt.ObservedGuard),
		BeforeState: receipt.BeforeState, AfterState: receipt.AfterState, SealedSourceDigest: receipt.SealedSourceDigest,
		AfterSealedSourceDigest: receipt.AfterSealedSourceDigest, Passed: receipt.Passed, Digest: receipt.Digest,
	}
	if err := domain.Validate(); err != nil || receipt.Class != class || receipt.ClaimID != challenge.ClaimID ||
		receipt.SourceCapsuleID != challenge.SourceCapsuleID || receipt.SourceManifestDigest != challenge.SourceManifestDigest ||
		receipt.SourceLedgerDigest != challenge.SourceLedgerDigest ||
		receipt.ReplayCommand != challengeReplayCommand(receipt.ClaimID, receipt.ChallengeID) {
		return errors.New("evidence explorer challenge receipt is invalid or detached")
	}
	return nil
}
