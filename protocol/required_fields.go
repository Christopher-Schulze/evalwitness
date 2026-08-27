package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func validateAuditCaseRequiredFields(raw []byte) error {
	root, err := rawObject(raw, "")
	if err != nil {
		return err
	}
	if err := requireRawFields(root, "", "schema_version", "protocol_version", "case_id", "conformance_level", "kind", "description", "required_capabilities", "invocation", "expected", "extensions"); err != nil {
		return err
	}
	invocation, err := rawObject(root["invocation"], "/invocation")
	if err != nil {
		return err
	}
	if err := requireRawFields(invocation, "/invocation", "schema_version", "invocation_id", "operation", "offline", "timeout_millis", "max_input_bytes", "trajectory_refs", "extensions"); err != nil {
		return err
	}
	expected, err := rawObject(root["expected"], "/expected")
	if err != nil {
		return err
	}
	if err := requireRawFields(expected, "/expected", "status"); err != nil {
		return err
	}
	if err := validateRawObjectArray(invocation["trajectory_refs"], "/invocation/trajectory_refs", []string{"canonical_schema", "source_format", "source_digest", "trajectory_digest", "accounting_digest", "trace_envelope_digest", "mapping_report_digest", "mapping_policy_version"}, nil); err != nil {
		return err
	}
	if raw, ok := invocation["request_vector"]; ok {
		object, objectErr := rawObject(raw, "/invocation/request_vector")
		if objectErr != nil {
			return objectErr
		}
		if err := requireRawFields(object, "/invocation/request_vector", "vector_id", "canonical_utf8_hex", "expected_sha256"); err != nil {
			return err
		}
	}
	if raw, ok := invocation["canonical_json"]; ok {
		object, objectErr := rawObject(raw, "/invocation/canonical_json")
		if objectErr != nil {
			return objectErr
		}
		if err := requireRawFields(object, "/invocation/canonical_json", "source_utf8_hex", "expected_canonical_utf8_hex", "expected_sha256"); err != nil {
			return err
		}
	}
	if raw, ok := invocation["score_evidence"]; ok {
		if err := validateRawScoreEvidence(raw); err != nil {
			return err
		}
	}
	if raw, ok := invocation["decision_evidence"]; ok {
		object, objectErr := rawObject(raw, "/invocation/decision_evidence")
		if objectErr != nil {
			return objectErr
		}
		if err := requireRawFields(object, "/invocation/decision_evidence", "schema_version", "policy_version", "state", "conditional_scores_decimal", "decision_strength_decimal", "score_evidence_digests", "budget_digest", "provenance_digest"); err != nil {
			return err
		}
	}
	if raw, ok := invocation["replay_evidence"]; ok {
		object, objectErr := rawObject(raw, "/invocation/replay_evidence")
		if objectErr != nil {
			return objectErr
		}
		if err := requireRawFields(object, "/invocation/replay_evidence", "request_fingerprint", "response_digest", "evidence_digest", "replay_status"); err != nil {
			return err
		}
	}
	if raw, ok := invocation["attestation_evidence"]; ok {
		object, objectErr := rawObject(raw, "/invocation/attestation_evidence")
		if objectErr != nil {
			return objectErr
		}
		if err := requireRawFields(object, "/invocation/attestation_evidence", "schema_version", "attestation_digest", "state", "evidence_ceiling", "observed_at", "expires_at", "request_fingerprint", "route_limitations"); err != nil {
			return err
		}
	}
	if err := validateRawExtensions(root["extensions"], "/extensions"); err != nil {
		return err
	}
	return validateRawExtensions(invocation["extensions"], "/invocation/extensions")
}

func validateRawScoreEvidence(raw []byte) error {
	object, err := rawObject(raw, "/invocation/score_evidence")
	if err != nil {
		return err
	}
	if err := requireRawFields(object, "/invocation/score_evidence", "schema_version", "policy_version", "tag", "extraction_mode", "alignment_status", "aligned_position", "requested_top_k", "returned_top_k", "visible_probability_mass_decimal", "valid_score_mass_decimal", "unobserved_probability_mass_decimal", "score_support", "visible_alternatives", "conditional_expected_score_decimal", "conditional_variance_decimal", "extracted", "degradations", "requested_model", "route_limitations", "request_fingerprint", "response_evidence_digest"); err != nil {
		return err
	}
	alignment, err := rawObject(object["aligned_position"], "/invocation/score_evidence/aligned_position")
	if err != nil {
		return err
	}
	if err := requireRawFields(alignment, "/invocation/score_evidence/aligned_position", "token_index", "token_byte", "stream_byte", "raw_byte"); err != nil {
		return err
	}
	if err := validateRawObjectArray(object["score_support"], "/invocation/score_evidence/score_support", []string{"letter", "value_decimal", "probability_decimal", "source_ranks", "source_forms"}, nil); err != nil {
		return err
	}
	if err := validateRawObjectArray(object["visible_alternatives"], "/invocation/score_evidence/visible_alternatives", []string{"rank", "token", "logprob_decimal", "probability_decimal", "chosen"}, nil); err != nil {
		return err
	}
	return validateRawObjectArray(object["degradations"], "/invocation/score_evidence/degradations", []string{"code"}, nil)
}

func validateRawExtensions(raw []byte, path string) error {
	return validateRawObjectArray(raw, path, []string{"namespace", "schema", "required", "payload"}, func(object map[string]json.RawMessage, itemPath string) error {
		var namespace string
		if err := json.Unmarshal(object["namespace"], &namespace); err != nil {
			return fmt.Errorf("%s/namespace is not a string", itemPath)
		}
		if namespace != ReliabilityExtension {
			return nil
		}
		payload, err := rawObject(object["payload"], itemPath+"/payload")
		if err != nil {
			return err
		}
		if err := requireRawFields(payload, itemPath+"/payload", "schema_version", "evidence_factors", "evidence_interventions", "reliance_contrasts"); err != nil {
			return err
		}
		if err := validateRawObjectArray(payload["evidence_factors"], itemPath+"/payload/evidence_factors", []string{"factor_id", "kind", "description"}, nil); err != nil {
			return err
		}
		if err := validateRawObjectArray(payload["evidence_interventions"], itemPath+"/payload/evidence_interventions", []string{"intervention_id", "factor_id", "parent_digest", "child_digest", "changed_event_ids", "changed_field_paths", "validator"}, nil); err != nil {
			return err
		}
		return validateRawObjectArray(payload["reliance_contrasts"], itemPath+"/payload/reliance_contrasts", []string{"contrast_id", "factor_id", "intervention_id", "baseline_result_digest", "intervention_result_digest", "effect_decimal", "uncertainty_decimal", "interpretation"}, nil)
	})
}

func rawObject(raw []byte, path string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%s must be an object", displayPath(path))
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, fmt.Errorf("%s must be an object", displayPath(path))
	}
	return object, nil
}

func requireRawFields(object map[string]json.RawMessage, path string, fields ...string) error {
	for _, field := range fields {
		if _, present := object[field]; !present {
			return fmt.Errorf("required field %s/%s is absent", path, field)
		}
	}
	return nil
}

func validateRawObjectArray(raw []byte, path string, fields []string, validate func(map[string]json.RawMessage, string) error) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%s must be an array", displayPath(path))
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || items == nil {
		return fmt.Errorf("%s must be an array", displayPath(path))
	}
	for index, item := range items {
		itemPath := fmt.Sprintf("%s/%d", path, index)
		object, err := rawObject(item, itemPath)
		if err != nil {
			return err
		}
		if err := requireRawFields(object, itemPath, fields...); err != nil {
			return err
		}
		if validate != nil {
			if err := validate(object, itemPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func displayPath(path string) string {
	if path == "" {
		return "root"
	}
	return path
}
