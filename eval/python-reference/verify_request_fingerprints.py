#!/usr/bin/env python3
"""Independent verifier for EvalWitness request-fingerprint corpus v2."""

import argparse
import hashlib
import json
import math
import os
import struct
import sys


CORPUS_SCHEMA = "evalwitness.request-fingerprint-corpus.v2"
REQUEST_SCHEMA_VERSION = 2
PROMPT_BUILDER_VERSION = "evalwitness.verifier.prompt.v1"


def json_string(value):
    if not isinstance(value, str):
        raise TypeError(f"expected string, got {type(value).__name__}")
    output = ['"']
    escapes = {
        '"': '\\"',
        "\\": "\\\\",
        "\b": "\\b",
        "\f": "\\f",
        "\n": "\\n",
        "\r": "\\r",
        "\t": "\\t",
    }
    for character in value:
        codepoint = ord(character)
        if 0xD800 <= codepoint <= 0xDFFF:
            raise ValueError("unpaired UTF-16 surrogate is not valid UTF-8")
        if character in escapes:
            output.append(escapes[character])
        elif codepoint < 0x20:
            output.append(f"\\u{codepoint:04x}")
        else:
            output.append(character)
    output.append('"')
    return "".join(output)


def boolean(value):
    if not isinstance(value, bool):
        raise TypeError(f"expected boolean, got {type(value).__name__}")
    return "true" if value else "false"


def integer(value):
    if isinstance(value, bool) or not isinstance(value, int):
        raise TypeError(f"expected integer, got {type(value).__name__}")
    return str(value)


def string_array(values):
    if not isinstance(values, list):
        raise TypeError("expected string array")
    return "[" + ",".join(json_string(value) for value in values) + "]"


def messages(values):
    if not isinstance(values, list) or not values:
        raise ValueError("messages must be a non-empty array")
    encoded = []
    for message in values:
        encoded.append(
            "{"
            + '"role":' + json_string(message["role"])
            + ',"content":' + json_string(message["content"])
            + ',"name":' + json_string(message.get("name", ""))
            + ',"tool_call_id":' + json_string(message.get("tool_call_id", ""))
            + "}"
        )
    return "[" + ",".join(encoded) + "]"


def logit_bias(values):
    if not isinstance(values, dict):
        raise TypeError("logit_bias must be an object")
    return "{" + ",".join(
        json_string(key) + ":" + integer(values[key])
        for key in sorted(values)
    ) + "}"


def evidence_bindings(values):
    if not isinstance(values, list):
        raise TypeError("evidence_bindings must be an array")
    encoded = []
    seen = set()
    digest_fields = (
        "source_digest", "canonical_digest", "ingestion_digest",
        "trace_envelope_digest", "mapping_report_digest",
    )
    for binding in values:
        slot = binding["input_slot"]
        if slot in seen:
            raise ValueError(f"duplicate evidence binding slot {slot!r}")
        seen.add(slot)
        for field in digest_fields:
            digest = binding[field]
            if len(digest) != 64 or any(character not in "0123456789abcdef" for character in digest):
                raise ValueError(f"invalid {field}")
        encoded.append(
            "{"
            + '"input_slot":' + json_string(slot)
            + ',"source_digest":' + json_string(binding["source_digest"])
            + ',"canonical_digest":' + json_string(binding["canonical_digest"])
            + ',"ingestion_digest":' + json_string(binding["ingestion_digest"])
            + ',"trace_envelope_digest":' + json_string(binding["trace_envelope_digest"])
            + ',"mapping_report_digest":' + json_string(binding["mapping_report_digest"])
            + ',"mapping_policy_version":' + json_string(binding["mapping_policy_version"])
            + "}"
        )
    return "[" + ",".join(encoded) + "]"


def temperature_bits(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise TypeError("temperature must be numeric")
    numeric = float(value)
    if not math.isfinite(numeric) or numeric < 0:
        raise ValueError("temperature must be finite and non-negative")
    if numeric == 0:
        numeric = 0.0
    return struct.pack(">d", numeric).hex()


def canonical_request(request):
    if request["schema_version"] != REQUEST_SCHEMA_VERSION:
        raise ValueError("unsupported request schema")
    if request["prompt_builder_version"] != PROMPT_BUILDER_VERSION:
        raise ValueError("unsupported prompt-builder contract")
    seed = request["seed"]
    encoded_seed = "null" if seed is None else integer(seed)
    fields = [
        '"schema_version":' + integer(request["schema_version"]),
        '"provider_id":' + json_string(request["provider_id"]),
        '"base_url_origin":' + json_string(request["base_url_origin"]),
        '"endpoint_path":' + json_string(request["endpoint_path"]),
        '"endpoint_kind":' + json_string(request["endpoint_kind"]),
        '"requested_model":' + json_string(request["requested_model"]),
        '"thinking_mode":' + json_string(request["thinking_mode"]),
        '"messages":' + messages(request["messages"]),
        '"temperature_f64":' + json_string(temperature_bits(request["temperature"])),
        '"seed":' + encoded_seed,
        '"max_output_tokens":' + integer(request["max_output_tokens"]),
        '"logprobs":' + boolean(request["logprobs"]),
        '"top_logprobs":' + integer(request["top_logprobs"]),
        '"stop":' + string_array(request["stop"]),
        '"score_tags":' + string_array(request["score_tags"]),
        '"response_format":' + json_string(request["response_format"]),
        '"stream":' + boolean(request["stream"]),
        '"prompt_builder_version":' + json_string(request["prompt_builder_version"]),
        '"logit_bias":' + logit_bias(request["logit_bias"]),
        '"evidence_bindings":' + evidence_bindings(request["evidence_bindings"]),
    ]
    return ("{" + ",".join(fields) + "}").encode("utf-8")


def verify_corpus(path):
    with open(path, "r", encoding="utf-8") as handle:
        corpus = json.load(handle)
    if corpus.get("corpus_schema") != CORPUS_SCHEMA:
        raise ValueError(f"unsupported corpus schema {corpus.get('corpus_schema')!r}")
    if corpus.get("request_schema_version") != REQUEST_SCHEMA_VERSION:
        raise ValueError("request schema version mismatch")
    cases = corpus.get("cases")
    if not isinstance(cases, list) or not cases:
        raise ValueError("corpus has no cases")
    seen = set()
    for case in cases:
        name = case["name"]
        if name in seen:
            raise ValueError(f"duplicate case {name!r}")
        seen.add(name)
        canonical = canonical_request(case["request"])
        if canonical.hex() != case["canonical_utf8_hex"]:
            raise ValueError(f"{name}: canonical bytes differ")
        fingerprint = hashlib.sha256(canonical).hexdigest()
        if fingerprint != case["fingerprint_sha256"]:
            raise ValueError(f"{name}: fingerprint differs")
    return len(cases)


def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    default_corpus = os.path.normpath(os.path.join(
        script_dir, "..", "..", "internal", "provider", "testdata",
        "request-fingerprint-v2.json",
    ))
    parser = argparse.ArgumentParser(
        description="Verify EvalWitness request-fingerprint corpus v2 independently",
    )
    parser.add_argument("corpus", nargs="?", default=default_corpus)
    args = parser.parse_args()
    try:
        count = verify_corpus(args.corpus)
    except (OSError, KeyError, TypeError, ValueError, json.JSONDecodeError) as error:
        print(f"request fingerprint verification failed: {error}", file=sys.stderr)
        return 1
    print(f"verified {count} request-fingerprint vectors with independent Python implementation")
    return 0


if __name__ == "__main__":
    sys.exit(main())
