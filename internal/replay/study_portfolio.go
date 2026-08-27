package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	StudyPortfolioSchemaVersion     = "evalwitness.identical-response-portfolio.v1"
	LockedIdenticalResponseEstimand = "distribution_aware_vs_chosen_token on the same immutable completion"
	EvalTerminalNotLockedEstimand   = "live numbers are eval-terminal; they do not answer the locked identical-response estimand"
)

type StudyPortfolioClaim struct {
	ClaimID string `json:"claim_id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
}

type sidecarClaimLedger struct {
	SchemaVersion string `json:"schema_version"`
	Claims        []struct {
		ClaimID string `json:"claim_id"`
		Title   string `json:"title"`
		Status  string `json:"status"`
	} `json:"claims"`
}

type StudyPortfolio struct {
	SchemaVersion     string                `json:"schema_version"`
	BindDigest        string                `json:"bind_digest"`
	BindStatus        string                `json:"bind_status"`
	EvidenceCeiling   string                `json:"evidence_ceiling"`
	LockedEstimand    string                `json:"locked_estimand"`
	ObservedEstimand  string                `json:"observed_estimand"`
	ExplorerPresent   bool                  `json:"explorer_present"`
	ClaimLedgerSHA256 string                `json:"claim_ledger_sha256"`
	Claims            []StudyPortfolioClaim `json:"claims"`
	MissingParents    []string              `json:"missing_parents"`
	Limitations       []string              `json:"limitations"`
	Digest            string                `json:"digest"`
}

func BuildStudyPortfolio(bindPath, ledgerPath string) (StudyPortfolio, error) {
	rawBind, err := os.ReadFile(bindPath)
	if err != nil {
		return StudyPortfolio{}, err
	}
	var certificate StudyBindCertificate
	if err := protocol.DecodeStrict(bytes.TrimSpace(rawBind), &certificate); err != nil {
		return StudyPortfolio{}, fmt.Errorf("decode bind certificate: %w", err)
	}
	recomputed, err := protocol.Digest(unsignedStudyBindCertificate(certificate))
	if err != nil {
		return StudyPortfolio{}, err
	}
	if recomputed != certificate.Digest {
		return StudyPortfolio{}, errors.New("bind certificate digest mismatch")
	}
	ledgerSHA, err := fileSHA256(ledgerPath)
	if err != nil {
		return StudyPortfolio{}, err
	}
	if ledgerSHA != certificate.ClaimLedgerSHA256 {
		return StudyPortfolio{}, errors.New("claim ledger does not match the bind certificate")
	}
	rawLedger, err := os.ReadFile(ledgerPath)
	if err != nil {
		return StudyPortfolio{}, err
	}
	var ledger sidecarClaimLedger
	if err := json.Unmarshal(bytes.TrimSpace(rawLedger), &ledger); err != nil {
		return StudyPortfolio{}, fmt.Errorf("decode claim ledger: %w", err)
	}
	claims := make([]StudyPortfolioClaim, 0, len(ledger.Claims))
	for _, claim := range ledger.Claims {
		claims = append(claims, StudyPortfolioClaim{ClaimID: claim.ClaimID, Title: claim.Title, Status: claim.Status})
	}
	portfolio := StudyPortfolio{
		SchemaVersion:     StudyPortfolioSchemaVersion,
		BindDigest:        certificate.Digest,
		BindStatus:        certificate.BindStatus,
		EvidenceCeiling:   certificate.EvidenceCeiling,
		LockedEstimand:    LockedIdenticalResponseEstimand,
		ObservedEstimand:  EvalTerminalNotLockedEstimand,
		ExplorerPresent:   false,
		ClaimLedgerSHA256: ledgerSHA,
		Claims:            claims,
		MissingParents:    slices.Clone(certificate.MissingParents),
		Limitations:       slices.Clone(certificate.Limitations),
	}
	if portfolio.MissingParents == nil {
		portfolio.MissingParents = []string{}
	}
	portfolio.Limitations = append(portfolio.Limitations,
		"portfolio is a digest-bound sidecar view; explorer path is absent")
	slices.Sort(portfolio.Limitations)
	portfolio.Limitations = slices.Compact(portfolio.Limitations)
	digest, err := protocol.Digest(unsignedStudyPortfolio(portfolio))
	if err != nil {
		return StudyPortfolio{}, err
	}
	portfolio.Digest = digest
	return portfolio, nil
}

func unsignedStudyPortfolio(portfolio StudyPortfolio) StudyPortfolio {
	portfolio.Digest = ""
	return portfolio
}
