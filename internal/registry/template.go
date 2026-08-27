package registry

func IntakeTemplate() []byte {
	return []byte(`{
  "entry_id": "REPLACE_ENTRY_ID",
  "submitter": "REPLACE_SUBMITTER",
  "capsule_id": "REPLACE_CAPSULE_ID",
  "capsule_digest": "0000000000000000000000000000000000000000000000000000000000000000",
  "profile_digest": "0000000000000000000000000000000000000000000000000000000000000000",
  "challenge_nonce": "0000000000000000000000000000000000000000000000000000000000000000",
  "request_contract": "REPLACE_REQUEST_CONTRACT",
  "score_policy": "REPLACE_SCORE_POLICY",
  "trace_mapping": "REPLACE_TRACE_MAPPING",
  "schema_version": 1,
  "endpoint_kind": "chat_completions",
  "thinking_mode": "disabled",
  "temperature": 1,
  "max_output_tokens": 4096,
  "top_logprobs": 20,
  "score_alphabet": "REPLACE_SCORE_ALPHABET",
  "evidence_level": "E1",
  "observed_at": "1970-01-01T00:00:00Z",
  "expires_at": "1970-01-01T00:00:01Z",
  "served_model": "REPLACE_SERVED_MODEL",
  "checkpoint_assertion": "",
  "provider_request_id": "",
  "latency_milliseconds": 0,
  "http_attempts": 1,
  "license": "REPLACE_LICENSE",
  "status": "format_verified",
  "privacy_class": "public",
  "public_release_allowed": false,
  "contamination_free": false,
  "community_validated": false
}
`)
}
