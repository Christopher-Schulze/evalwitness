package protocol

import (
	"errors"
	"fmt"
)

const ReliabilityExtensionSchema = "evalwitness.protocol.reliability-extension.v1"

type EvidenceFactor struct {
	FactorID    string `json:"factor_id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

type EvidenceIntervention struct {
	InterventionID    string   `json:"intervention_id"`
	FactorID          string   `json:"factor_id"`
	ParentDigest      string   `json:"parent_digest"`
	ChildDigest       string   `json:"child_digest"`
	ChangedEventIDs   []string `json:"changed_event_ids"`
	ChangedFieldPaths []string `json:"changed_field_paths"`
	Validator         string   `json:"validator"`
}

type RelianceContrast struct {
	ContrastID               string `json:"contrast_id"`
	FactorID                 string `json:"factor_id"`
	InterventionID           string `json:"intervention_id"`
	BaselineResultDigest     string `json:"baseline_result_digest"`
	InterventionResultDigest string `json:"intervention_result_digest"`
	EffectDecimal            string `json:"effect_decimal"`
	UncertaintyDecimal       string `json:"uncertainty_decimal"`
	Interpretation           string `json:"interpretation"`
}

type ReliabilityExtensionPayload struct {
	SchemaVersion string                 `json:"schema_version"`
	Factors       []EvidenceFactor       `json:"evidence_factors"`
	Interventions []EvidenceIntervention `json:"evidence_interventions"`
	Contrasts     []RelianceContrast     `json:"reliance_contrasts"`
}

func validateReliabilityExtension(extension Extension) error {
	if extension.Schema != ReliabilityExtensionSchema {
		return errors.New("reliability extension schema is unsupported")
	}
	var payload ReliabilityExtensionPayload
	if err := strictUnmarshal(extension.Payload, &payload); err != nil {
		return fmt.Errorf("decode reliability extension: %w", err)
	}
	if payload.SchemaVersion != ReliabilityExtensionSchema {
		return errors.New("reliability extension payload schema is invalid")
	}
	if payload.Factors == nil || payload.Interventions == nil || payload.Contrasts == nil {
		return errors.New("reliability extension omits a required array")
	}
	factors := make(map[string]struct{}, len(payload.Factors))
	for _, factor := range payload.Factors {
		if !identifierPattern.MatchString(factor.FactorID) || factor.Kind == "" || factor.Description == "" {
			return errors.New("reliability extension factor is invalid")
		}
		if _, duplicate := factors[factor.FactorID]; duplicate {
			return fmt.Errorf("duplicate reliability factor %q", factor.FactorID)
		}
		factors[factor.FactorID] = struct{}{}
	}
	interventions := make(map[string]EvidenceIntervention, len(payload.Interventions))
	for _, intervention := range payload.Interventions {
		if !identifierPattern.MatchString(intervention.InterventionID) || intervention.Validator == "" ||
			!validDigest(intervention.ParentDigest) || !validDigest(intervention.ChildDigest) ||
			intervention.ParentDigest == intervention.ChildDigest {
			return errors.New("reliability extension intervention is invalid")
		}
		if _, ok := factors[intervention.FactorID]; !ok {
			return fmt.Errorf("intervention %q references unknown factor", intervention.InterventionID)
		}
		if len(intervention.ChangedEventIDs) == 0 && len(intervention.ChangedFieldPaths) == 0 {
			return fmt.Errorf("intervention %q has no exact changed set", intervention.InterventionID)
		}
		if intervention.ChangedEventIDs == nil || intervention.ChangedFieldPaths == nil {
			return fmt.Errorf("intervention %q omits a required changed-set array", intervention.InterventionID)
		}
		if err := validateSortedUniqueStrings(intervention.ChangedEventIDs, "changed_event_ids"); err != nil {
			return fmt.Errorf("intervention %q: %w", intervention.InterventionID, err)
		}
		for _, eventID := range intervention.ChangedEventIDs {
			if !identifierPattern.MatchString(eventID) {
				return fmt.Errorf("intervention %q contains an invalid changed event ID", intervention.InterventionID)
			}
		}
		if err := validateSortedUniqueStrings(intervention.ChangedFieldPaths, "changed_field_paths"); err != nil {
			return fmt.Errorf("intervention %q: %w", intervention.InterventionID, err)
		}
		for _, path := range intervention.ChangedFieldPaths {
			if _, err := jsonPointerTokens(path); err != nil {
				return fmt.Errorf("intervention %q contains an invalid changed field path", intervention.InterventionID)
			}
		}
		if _, duplicate := interventions[intervention.InterventionID]; duplicate {
			return fmt.Errorf("duplicate intervention %q", intervention.InterventionID)
		}
		interventions[intervention.InterventionID] = intervention
	}
	contrasts := make(map[string]struct{}, len(payload.Contrasts))
	for _, contrast := range payload.Contrasts {
		if !identifierPattern.MatchString(contrast.ContrastID) || contrast.Interpretation != "verifier_output_reliance" ||
			!validDigest(contrast.BaselineResultDigest) || !validDigest(contrast.InterventionResultDigest) {
			return errors.New("reliability extension contrast is invalid")
		}
		intervention, ok := interventions[contrast.InterventionID]
		if !ok || intervention.FactorID != contrast.FactorID {
			return fmt.Errorf("contrast %q does not bind a matching intervention and factor", contrast.ContrastID)
		}
		if _, err := parseCanonicalDecimal(contrast.EffectDecimal); err != nil {
			return fmt.Errorf("contrast %q effect: %w", contrast.ContrastID, err)
		}
		uncertainty, err := parseCanonicalDecimal(contrast.UncertaintyDecimal)
		if err != nil || uncertainty.Sign() < 0 {
			return fmt.Errorf("contrast %q uncertainty is invalid", contrast.ContrastID)
		}
		if _, duplicate := contrasts[contrast.ContrastID]; duplicate {
			return fmt.Errorf("duplicate contrast %q", contrast.ContrastID)
		}
		contrasts[contrast.ContrastID] = struct{}{}
	}
	return nil
}
