package study

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
)

// IdenticalResponseRedistributionRightSchemaVersion is the frozen schema identity
// for the TASK 070 response redistribution-right evidence record. It stores the
// primary-source basis (not a legal opinion) for redistributing the exact
// provider response bytes the identical-response study will capture.
const IdenticalResponseRedistributionRightSchemaVersion = "evalwitness.identical-response-redistribution-right.v1"

// RedistributionRightEvidence records one candidate route's primary-source
// output-ownership clause and its recorded conditions. The verdict is a
// procedural status over the quoted text, never a legal conclusion.
type RedistributionRightEvidence struct {
	RouteID          string   `json:"route_id"`
	Provider         string   `json:"provider"`
	BaseURL          string   `json:"base_url"`
	Model            string   `json:"model"`
	DocumentTitle    string   `json:"document_title"`
	PrimarySourceURL string   `json:"primary_source_url"`
	RetrievalDate    string   `json:"retrieval_date"`
	EffectiveDate    string   `json:"effective_date"`
	AssignmentClause string   `json:"assignment_clause"`
	PermittedUses    []string `json:"permitted_uses"`
	Conditions       []string `json:"conditions"`
	SPDXExpression   string   `json:"spdx_expression"`
	Attribution      string   `json:"attribution"`
	Scope            string   `json:"scope"`
	Verdict          string   `json:"verdict"`
}

// RedistributionRightSummary records the aggregate procedural status across the
// two candidate routes.
type RedistributionRightSummary struct {
	Routes                     int      `json:"routes"`
	OutputRightsAssigned       int      `json:"output_rights_assigned"`
	Verdict                    string   `json:"verdict"`
	ReleaseDecisionRemainsWith string   `json:"release_decision_remains_with"`
	UnsupportedClaims          []string `json:"unsupported_claims"`
}

// IdenticalResponseRedistributionRight is the frozen evidence record.
type IdenticalResponseRedistributionRight struct {
	SchemaVersion string                        `json:"schema_version"`
	Evidence      []RedistributionRightEvidence `json:"evidence"`
	Summary       RedistributionRightSummary    `json:"summary"`
	Digest        string                        `json:"digest"`
}

const (
	redistributionRetrievalDate = "2026-08-18"
	redistributionScope         = "provider response bytes (Output) from the exact request, not the provider model weights or the upstream benchmark trajectories"
)

// BuildIdenticalResponseRedistributionRight reproduces the frozen redistribution
// evidence record deterministically. It makes no network call and no provider
// call; the quoted clauses and retrieval date are frozen constants.
func BuildIdenticalResponseRedistributionRight() (IdenticalResponseRedistributionRight, error) {
	record := IdenticalResponseRedistributionRight{
		SchemaVersion: IdenticalResponseRedistributionRightSchemaVersion,
		Evidence: []RedistributionRightEvidence{
			{
				RouteID:          "deepseek-direct-deepseek-v4",
				Provider:         "deepseek",
				BaseURL:          "https://api.deepseek.com",
				Model:            "deepseek-v4-flash|deepseek-v4-pro",
				DocumentTitle:    "DeepSeek Open Platform Terms of Service",
				PrimarySourceURL: "https://cdn.deepseek.com/policies/en-US/deepseek-open-platform-terms-of-service.html",
				RetrievalDate:    redistributionRetrievalDate,
				EffectiveDate:    "2026-04-29",
				AssignmentClause: "We assign any rights, title, and interests—if any—in the Outputs of the Services to you",
				PermittedUses:    []string{"academic research", "derivative product development", "personal use"},
				Conditions: []string{
					"do not use DeepSeek brand identifiers in a way that implies endorsement or a partnership",
					"disclose to end users that the content is AI-generated and may contain errors",
				},
				SPDXExpression: "NOASSERTION",
				Attribution:    "DeepSeek Open Platform Terms of Service Section 4.2; attribution to DeepSeek is not a license condition, but brand misuse is prohibited by Section 5",
				Scope:          redistributionScope,
				Verdict:        "output_rights_assigned_with_conditions",
			},
			{
				RouteID:          "opencode-go-deepseek-v4-flash-0731",
				Provider:         "opencode-go-cn",
				BaseURL:          "https://opencode.ai/zen/go/v1",
				Model:            "deepseek-v4-flash",
				DocumentTitle:    "OpenCode Terms of Use",
				PrimarySourceURL: "https://opencode.ai/legal/terms-of-service",
				RetrievalDate:    redistributionRetrievalDate,
				EffectiveDate:    "2026-03-06",
				AssignmentClause: "We hereby assign to you all our right, title, and interest, if any, in and to Output",
				PermittedUses:    []string{"own internal use"},
				Conditions: []string{
					"the Services are for your own internal use, not on behalf of or for the benefit of any third party",
					"do not use Output to develop artificial intelligence models that compete with the Services or any Third Party Models",
					"do not represent that Output was human-generated when it was not",
				},
				SPDXExpression: "NOASSERTION",
				Attribution:    "OpenCode Terms of Use, Who Owns the Services and Content section; output ownership is assigned, but the internal-use and no-competing-model conditions apply",
				Scope:          redistributionScope,
				Verdict:        "output_rights_assigned_subject_to_internal_use_review",
			},
		},
		Summary: RedistributionRightSummary{
			Routes:                     2,
			OutputRightsAssigned:       2,
			Verdict:                    "both_candidate_routes_assign_output_rights",
			ReleaseDecisionRemainsWith: "owner review of the exact publication scope against the recorded conditions; this record is evidence, not authorization",
			UnsupportedClaims: []string{
				"a legal opinion that any specific publication is lawful",
				"a license to redistribute the provider model weights",
				"a license to redistribute the upstream benchmark trajectory bodies",
				"authorization to make live provider calls",
			},
		},
	}
	digest, err := identicalResponseRedistributionRightDigest(record)
	if err != nil {
		return IdenticalResponseRedistributionRight{}, err
	}
	record.Digest = digest
	if err := record.Validate(); err != nil {
		return IdenticalResponseRedistributionRight{}, err
	}
	return record, nil
}

// Validate enforces the frozen redistribution-right evidence invariants.
func (record IdenticalResponseRedistributionRight) Validate() error {
	if record.SchemaVersion != IdenticalResponseRedistributionRightSchemaVersion || !validDigest(record.Digest) {
		return errors.New("identical-response redistribution-right schema identity or digest is invalid")
	}
	if record.Summary.Routes != 2 || record.Summary.OutputRightsAssigned != 2 || len(record.Evidence) != 2 ||
		missing(record.Summary.Verdict, record.Summary.ReleaseDecisionRemainsWith) || len(record.Summary.UnsupportedClaims) == 0 {
		return errors.New("identical-response redistribution-right summary is incomplete")
	}
	if err := validateUniqueText("identical-response redistribution-right unsupported claims", record.Summary.UnsupportedClaims); err != nil {
		return err
	}
	previous := ""
	for _, evidence := range record.Evidence {
		if missing(evidence.RouteID, evidence.Provider, evidence.BaseURL, evidence.Model, evidence.DocumentTitle, evidence.PrimarySourceURL,
			evidence.RetrievalDate, evidence.EffectiveDate, evidence.AssignmentClause, evidence.SPDXExpression, evidence.Attribution, evidence.Scope, evidence.Verdict) ||
			evidence.RouteID <= previous {
			return errors.New("identical-response redistribution-right evidence rows are invalid or unsorted")
		}
		previous = evidence.RouteID
		if evidence.RetrievalDate != redistributionRetrievalDate || !strings.HasPrefix(evidence.PrimarySourceURL, "https://") {
			return errors.New("identical-response redistribution-right retrieval date or source URL is invalid")
		}
		if evidence.SPDXExpression != "NOASSERTION" {
			return errors.New("identical-response redistribution-right must not fabricate an SPDX expression for a ToS assignment")
		}
		if !strings.Contains(strings.ToLower(evidence.AssignmentClause), "assign") {
			return errors.New("identical-response redistribution-right assignment clause does not quote the assignment language")
		}
		if !slices.Contains([]string{"output_rights_assigned_with_conditions", "output_rights_assigned_subject_to_internal_use_review"}, evidence.Verdict) {
			return errors.New("identical-response redistribution-right verdict is unsupported")
		}
		if len(evidence.PermittedUses) == 0 || len(evidence.Conditions) == 0 {
			return errors.New("identical-response redistribution-right permitted uses and conditions must be recorded")
		}
		if err := validateUniqueText("identical-response redistribution-right permitted uses", evidence.PermittedUses); err != nil {
			return err
		}
		if err := validateUniqueText("identical-response redistribution-right conditions", evidence.Conditions); err != nil {
			return err
		}
	}
	expected, err := identicalResponseRedistributionRightDigest(record)
	if err != nil {
		return err
	}
	if record.Digest != expected {
		return errors.New("identical-response redistribution-right digest is invalid")
	}
	return nil
}

func identicalResponseRedistributionRightDigest(record IdenticalResponseRedistributionRight) (string, error) {
	record.Digest = ""
	encoded, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
