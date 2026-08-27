#!/usr/bin/env bash
# Provider-free outcome-governance, sample-lock, schema, and agreement gate.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
tmp_dir=$(mktemp -d)
tmp_bin=$(mktemp /tmp/evalwitness-outcome-XXXXXX)
trap 'rm -rf "${tmp_dir}"; rm -f "${tmp_bin}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness

"$tmp_bin" outcome plan --plan @eval/governance/outcome-adjudication-v1.json > "$tmp_dir/plan.json"
for schema in plan record record-draft evidence evidence-draft blind-build-request blind-packet private-mapping label label-draft resolution agreement preservation sample-commitment pilot-sample-v1 pilot-readiness-v1 pilot-sample pilot-readiness pilot-source-binding pilot-private-materials pilot-inspection natural-inventory-request natural-inventory executable-log qualification-set qualification-report review-bundle reviewer-record review-assignment label-batch mapping-reveal adjudication-ledger reviewer-handbook reviewer-kit blinding-protocol blinding-probe blinding-probe-batch blinding-analysis rubric-ambiguity-analysis source-audit; do
  "$tmp_bin" outcome schema --type "$schema" > "$tmp_dir/$schema.schema.json"
done
"$tmp_bin" outcome validate --type sample-commitment --document @eval/governance/outcome-mutation-sample-v1.json > "$tmp_dir/sample-validation.json"
"$tmp_bin" outcome validate --type pilot-sample-v1 --document @eval/governance/outcome-pilot-sample-v1.json > "$tmp_dir/pilot-v1-validation.json"
"$tmp_bin" outcome validate --type pilot-sample --document @eval/governance/outcome-pilot-sample-v2.json > "$tmp_dir/pilot-validation.json"
"$tmp_bin" outcome natural-request --request @eval/governance/outcome-natural-inventory-request-v1.json > "$tmp_dir/natural-request.json"
"$tmp_bin" outcome natural-inventory \
  --plan @eval/governance/outcome-adjudication-v1.json \
  --request @eval/governance/outcome-natural-inventory-request-v1.json > "$tmp_dir/natural-inventory.json"
"$tmp_bin" outcome validate --type natural-inventory --document @"$tmp_dir/natural-inventory.json" > "$tmp_dir/natural-validation.json"
"$tmp_bin" outcome pilot-sample \
  --plan @eval/governance/outcome-adjudication-v1.json \
  --inventory @"$tmp_dir/natural-inventory.json" > "$tmp_dir/pilot.json"
python3 - "$tmp_dir/pilot.json" eval/governance/outcome-pilot-sample-v2.json <<'PY'
import json
import pathlib
import sys

generated = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
governed = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
if generated != governed:
    raise SystemExit("outcome-only pilot v2 drifted from its governed natural inventory")
PY
"$tmp_bin" outcome qualification > "$tmp_dir/qualification-default.json"
"$tmp_bin" outcome qualification --set @eval/governance/outcome-qualification-v1.json > "$tmp_dir/qualification-governed.json"
"$tmp_bin" outcome validate --type qualification-set --document @eval/governance/outcome-qualification-v1.json > "$tmp_dir/qualification-validation.json"
"$tmp_bin" outcome review handbook > "$tmp_dir/handbook-default.json"
"$tmp_bin" outcome review handbook --handbook @eval/governance/outcome-reviewer-handbook-v1.json \
  --qualification-set @eval/governance/outcome-qualification-v1.json > "$tmp_dir/handbook-governed.json"
"$tmp_bin" outcome validate --type reviewer-handbook --document @eval/governance/outcome-reviewer-handbook-v1.json > "$tmp_dir/handbook-validation.json"
python3 - "$tmp_dir/qualification-governed.json" "$tmp_dir" <<'PY'
import json
import pathlib
import sys

qualification = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
root = pathlib.Path(sys.argv[2])
for reviewer in ("reviewer-left", "reviewer-right", "reviewer-tie"):
    for index, case in enumerate(qualification["cases"]):
        draft = {
            "packet_id": case["packet"]["packet_id"],
            "adjudicator_alias": reviewer,
            "reviewer_slot": 1,
            "primary_outcome": case["expected_outcome"],
            "task_satisfaction": "unclear",
            "technical_correctness": "unclear",
            "verification_quality": "unclear",
            "harmful_side_effects": "not_applicable",
            "evidence_sufficiency": "unclear",
            "reason_codes": case["required_reason_codes"],
            "submitted_at": "2026-08-09T13:00:00Z",
            "rubric_version": qualification["rubric_version"],
            "qualification_digest": qualification["digest"],
            "conflicts_of_interest": [],
        }
        (root / f"qualification-draft-{reviewer}-{index}.json").write_text(json.dumps(draft, separators=(",", ":")), encoding="utf-8")
PY
for reviewer in reviewer-left reviewer-right reviewer-tie; do
  for index in 0 1 2 3 4; do
    "$tmp_bin" outcome label --draft @"$tmp_dir/qualification-draft-$reviewer-$index.json" > "$tmp_dir/qualification-label-$reviewer-$index.json"
  done
done
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
for reviewer in ("reviewer-left", "reviewer-right", "reviewer-tie"):
    labels = [json.loads((root / f"qualification-label-{reviewer}-{index}.json").read_text(encoding="utf-8")) for index in range(5)]
    (root / f"qualification-labels-{reviewer}.json").write_text(json.dumps(labels, separators=(",", ":")), encoding="utf-8")
PY
for reviewer in reviewer-left reviewer-right reviewer-tie; do
  "$tmp_bin" outcome qualify \
    --set @eval/governance/outcome-qualification-v1.json \
    --labels @"$tmp_dir/qualification-labels-$reviewer.json" > "$tmp_dir/qualification-report-$reviewer.json"
done
python3 - "$tmp_dir/natural-inventory.json" eval/governance/outcome-natural-inventory-v1.json <<'PY'
import json
import pathlib
import sys

generated = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
governed = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
if generated != governed:
    raise SystemExit("natural outcome inventory drifted from governed development artifacts")
PY

python3 - "$tmp_dir" <<'PY'
import hashlib
import json
import os
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
for index in range(1, 4):
    content = f"Licensed redacted trajectory evidence {index}."
    request = {
        "schema_version": "evalwitness.outcome-blind-build-request.v1",
        "plan_digest": "2b7fa309bdf2a4151c640275f858342624fddfbf5d6ada316e5f996ef14dd46e",
        "task_alias": f"natural-task-opaque-{index}",
        "evidence": [{
            "slot": "source-candidate-one",
            "kind": "trajectory",
            "content": content,
            "content_digest": hashlib.sha256(content.encode()).hexdigest(),
            "license": "MIT",
            "limitation": "redacted fixture",
        }],
        "rubric_questions": ["Is the available evidence sufficient?"],
        "privacy_class": "public",
        "public_releasable": True,
        "source_case_digest": hashlib.sha256(f"source-case-{index}".encode()).hexdigest(),
        "condition": "presentation_invariance" if index == 1 else "falsified_test_evidence",
        "expected_relation": "quality_equal",
        "slot_mappings": [{
            "slot": "source-candidate-one",
            "source_digest": hashlib.sha256(f"source-artifact-{index}".encode()).hexdigest(),
        }],
        "blinding_key_id": "audit-key-v1",
        "forbidden_values": ["deepseek", "verifier_winner"],
    }
    (root / f"packet-request-{index}.json").write_text(json.dumps(request, separators=(",", ":")), encoding="utf-8")
for index, value in enumerate(("31", "32", "33", "34")):
    path = root / ("blinding-key" if index == 0 else f"review-seed-{index}")
    path.write_text(value * 32 + "\n", encoding="utf-8")
    os.chmod(path, 0o600)
PY
for index in 1 2 3; do
  "$tmp_bin" outcome packet \
    --request @"$tmp_dir/packet-request-$index.json" \
    --key-file "$tmp_dir/blinding-key" \
    --private-root "$tmp_dir/private-vault" > "$tmp_dir/public-packet-$index.json"
  "$tmp_bin" outcome validate --type blind-packet --document @"$tmp_dir/public-packet-$index.json" > "$tmp_dir/packet-validation-$index.json"
done

python3 - "$tmp_dir" <<'PY'
import hashlib
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
items = []
for index in range(1, 4):
    packet = json.loads((root / f"public-packet-{index}.json").read_text(encoding="utf-8"))
    items.append({"task_group_id": "group-" + hashlib.sha256(f"review-group-{index}".encode()).hexdigest(), "packet": packet})
(root / "review-items.json").write_text(json.dumps(items, separators=(",", ":")), encoding="utf-8")
mappings = [json.loads(path.read_text(encoding="utf-8")) for path in sorted((root / "private-vault" / "mappings").glob("*.json"))]
(root / "review-mappings.json").write_text(json.dumps(mappings, separators=(",", ":")), encoding="utf-8")
PY

"$tmp_bin" outcome review bundle \
  --plan @eval/governance/outcome-adjudication-v1.json \
  --qualification-set @eval/governance/outcome-qualification-v1.json \
  --handbook @eval/governance/outcome-reviewer-handbook-v1.json \
  --items @"$tmp_dir/review-items.json" \
  --data-role test --visibility public --created-at 2026-08-09T12:00:00Z > "$tmp_dir/review-bundle.json"
"$tmp_bin" outcome review blinding-protocol --bundle @"$tmp_dir/review-bundle.json" \
  --mappings @"$tmp_dir/review-mappings.json" --created-at 2026-08-09T13:30:00Z > "$tmp_dir/blinding-protocol.json"
"$tmp_bin" outcome validate --type blinding-protocol --document @"$tmp_dir/blinding-protocol.json" > "$tmp_dir/blinding-protocol-validation.json"
"$tmp_bin" outcome review reviewer --alias reviewer-left --role primary --consented-at 2026-08-09T12:30:00Z \
  --independence-attested --authorship-policy-accepted --contact-held-privately > "$tmp_dir/reviewer-left.json"
"$tmp_bin" outcome review reviewer --alias reviewer-right --role primary --consented-at 2026-08-09T12:30:00Z \
  --independence-attested --authorship-policy-accepted --contact-held-privately > "$tmp_dir/reviewer-right.json"
"$tmp_bin" outcome review reviewer --alias reviewer-tie --role tie_break --consented-at 2026-08-09T12:30:00Z \
  --independence-attested --authorship-policy-accepted --contact-held-privately > "$tmp_dir/reviewer-tie.json"
"$tmp_bin" outcome review assign-primary --bundle @"$tmp_dir/review-bundle.json" --reviewer @"$tmp_dir/reviewer-left.json" \
  --qualification @"$tmp_dir/qualification-report-reviewer-left.json" --slot 1 --seed-file "$tmp_dir/review-seed-1" \
  --assigned-at 2026-08-09T14:00:00Z > "$tmp_dir/assignment-left.json"
"$tmp_bin" outcome review assign-primary --bundle @"$tmp_dir/review-bundle.json" --reviewer @"$tmp_dir/reviewer-right.json" \
  --qualification @"$tmp_dir/qualification-report-reviewer-right.json" --slot 2 --seed-file "$tmp_dir/review-seed-2" \
  --assigned-at 2026-08-09T14:00:00Z > "$tmp_dir/assignment-right.json"
for reviewer in left right; do
  "$tmp_bin" outcome review kit --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-$reviewer.json" \
    --handbook @eval/governance/outcome-reviewer-handbook-v1.json --generated-at 2026-08-09T14:05:00Z > "$tmp_dir/reviewer-kit-$reviewer.json"
  "$tmp_bin" outcome review verify-kit --kit @"$tmp_dir/reviewer-kit-$reviewer.json" \
    --bundle @"$tmp_dir/review-bundle.json" > "$tmp_dir/reviewer-kit-$reviewer-validation.json"
  "$tmp_bin" outcome review render-kit --kit @"$tmp_dir/reviewer-kit-$reviewer.json" \
    --bundle @"$tmp_dir/review-bundle.json" > "$tmp_dir/reviewer-kit-$reviewer.md"
done

python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
bundle = json.loads((root / "review-bundle.json").read_text(encoding="utf-8"))
packets = [item["packet"]["packet_id"] for item in bundle["items"]]
states = {
    "reviewer-left": ["solved", "solved", "unsolved"],
    "reviewer-right": ["solved", "unsolved", "unsolved"],
}
for reviewer, reviewer_states in states.items():
    assignment = json.loads((root / f"assignment-{reviewer.removeprefix('reviewer-')}.json").read_text(encoding="utf-8"))
    report = json.loads((root / f"qualification-report-{reviewer}.json").read_text(encoding="utf-8"))
    state_by_packet = dict(zip(packets, reviewer_states))
    for index, packet_id in enumerate(assignment["packet_ids"]):
        technical_correctness = "sufficient"
        verification_quality = "sufficient"
        evidence_sufficiency = "sufficient"
        reason_codes = ["evidence_consistent"]
        if reviewer == "reviewer-right" and packet_id == packets[1]:
            technical_correctness = "insufficient"
            verification_quality = "unclear"
            evidence_sufficiency = "insufficient"
            reason_codes = ["evidence_insufficient", "technical_defect", "verification_incomplete"]
        draft = {
            "packet_id": packet_id, "adjudicator_alias": reviewer, "reviewer_slot": assignment["reviewer_slot"],
            "primary_outcome": state_by_packet[packet_id], "task_satisfaction": "sufficient",
            "technical_correctness": technical_correctness, "verification_quality": verification_quality,
            "harmful_side_effects": "not_applicable", "evidence_sufficiency": evidence_sufficiency,
            "reason_codes": reason_codes, "submitted_at": "2026-08-09T14:30:00Z",
            "rubric_version": assignment["rubric_version"], "qualification_digest": report["digest"], "conflicts_of_interest": [],
        }
        (root / f"review-draft-{reviewer}-{index}.json").write_text(json.dumps(draft, separators=(",", ":")), encoding="utf-8")
PY
for reviewer in reviewer-left reviewer-right; do
  for index in 0 1 2; do
    "$tmp_bin" outcome label --draft @"$tmp_dir/review-draft-$reviewer-$index.json" > "$tmp_dir/review-label-$reviewer-$index.json"
  done
done
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
for reviewer in ("reviewer-left", "reviewer-right"):
    labels = [json.loads((root / f"review-label-{reviewer}-{index}.json").read_text(encoding="utf-8")) for index in range(3)]
    (root / f"review-labels-{reviewer}.json").write_text(json.dumps(labels, separators=(",", ":")), encoding="utf-8")
PY
"$tmp_bin" outcome review label-batch --assignment @"$tmp_dir/assignment-left.json" --labels @"$tmp_dir/review-labels-reviewer-left.json" \
  --committed-at 2026-08-09T15:00:00Z > "$tmp_dir/batch-left.json"
"$tmp_bin" outcome review label-batch --assignment @"$tmp_dir/assignment-right.json" --labels @"$tmp_dir/review-labels-reviewer-right.json" \
  --committed-at 2026-08-09T15:00:00Z > "$tmp_dir/batch-right.json"
"$tmp_bin" outcome review analyze-rubric --bundle @"$tmp_dir/review-bundle.json" \
  --left-assignment @"$tmp_dir/assignment-left.json" --left-batch @"$tmp_dir/batch-left.json" \
  --right-assignment @"$tmp_dir/assignment-right.json" --right-batch @"$tmp_dir/batch-right.json" \
  --analyzed-at 2026-08-09T15:02:00Z > "$tmp_dir/rubric-ambiguity-analysis.json"
"$tmp_bin" outcome validate --type rubric-ambiguity-analysis --document @"$tmp_dir/rubric-ambiguity-analysis.json" > "$tmp_dir/rubric-ambiguity-validation.json"
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
mappings = json.loads((root / "review-mappings.json").read_text(encoding="utf-8"))
truth = {item["packet_id"]: item["condition"] for item in mappings}
for reviewer in ("left", "right"):
    assignment = json.loads((root / f"assignment-{reviewer}.json").read_text(encoding="utf-8"))
    wrong_used = False
    drafts = []
    for packet_id in assignment["packet_ids"]:
        condition = truth[packet_id]
        draft = {
            "packet_id": packet_id,
            "condition_guess": condition,
            "confidence": 0.8 if reviewer == "left" else 0.9,
            "recognized_task": False,
            "recognition_basis": "none",
            "submitted_at": "2026-08-09T15:01:00Z",
        }
        if reviewer == "right" and condition == "presentation_invariance":
            draft["condition_guess"] = "unknown"
            draft["confidence"] = 0
        elif reviewer == "right" and not wrong_used:
            draft["condition_guess"] = "presentation_invariance"
            draft["confidence"] = 0.7
            draft["recognized_task"] = True
            draft["recognition_basis"] = "task_text"
            wrong_used = True
        drafts.append(draft)
    (root / f"blinding-probe-drafts-{reviewer}.json").write_text(json.dumps(drafts, separators=(",", ":")), encoding="utf-8")
PY
for reviewer in left right; do
  "$tmp_bin" outcome review blinding-probe-batch --protocol @"$tmp_dir/blinding-protocol.json" \
    --assignment @"$tmp_dir/assignment-$reviewer.json" --label-batch @"$tmp_dir/batch-$reviewer.json" \
    --drafts @"$tmp_dir/blinding-probe-drafts-$reviewer.json" --committed-at 2026-08-09T15:05:00Z > "$tmp_dir/blinding-probes-$reviewer.json"
  "$tmp_bin" outcome validate --type blinding-probe-batch --document @"$tmp_dir/blinding-probes-$reviewer.json" > "$tmp_dir/blinding-probes-$reviewer-validation.json"
done
"$tmp_bin" outcome review assign-tie --bundle @"$tmp_dir/review-bundle.json" --reviewer @"$tmp_dir/reviewer-tie.json" \
  --qualification @"$tmp_dir/qualification-report-reviewer-tie.json" \
  --left-assignment @"$tmp_dir/assignment-left.json" --left-batch @"$tmp_dir/batch-left.json" \
  --right-assignment @"$tmp_dir/assignment-right.json" --right-batch @"$tmp_dir/batch-right.json" \
  --seed-file "$tmp_dir/review-seed-3" --assigned-at 2026-08-09T15:10:00Z > "$tmp_dir/assignment-tie.json"
"$tmp_bin" outcome review kit --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-tie.json" \
  --handbook @eval/governance/outcome-reviewer-handbook-v1.json --generated-at 2026-08-09T15:15:00Z > "$tmp_dir/reviewer-kit-tie.json"
"$tmp_bin" outcome review verify-kit --kit @"$tmp_dir/reviewer-kit-tie.json" \
  --bundle @"$tmp_dir/review-bundle.json" > "$tmp_dir/reviewer-kit-tie-validation.json"
"$tmp_bin" outcome review render-kit --kit @"$tmp_dir/reviewer-kit-tie.json" \
  --bundle @"$tmp_dir/review-bundle.json" > "$tmp_dir/reviewer-kit-tie.md"
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
assignment = json.loads((root / "assignment-tie.json").read_text(encoding="utf-8"))
report = json.loads((root / "qualification-report-reviewer-tie.json").read_text(encoding="utf-8"))
draft = {
    "packet_id": assignment["packet_ids"][0], "adjudicator_alias": "reviewer-tie", "reviewer_slot": 3,
    "primary_outcome": "solved", "task_satisfaction": "sufficient", "technical_correctness": "sufficient",
    "verification_quality": "sufficient", "harmful_side_effects": "not_applicable", "evidence_sufficiency": "sufficient",
    "reason_codes": ["evidence_consistent"], "submitted_at": "2026-08-09T15:20:00Z",
    "rubric_version": assignment["rubric_version"], "qualification_digest": report["digest"], "conflicts_of_interest": [],
}
(root / "review-draft-reviewer-tie.json").write_text(json.dumps(draft, separators=(",", ":")), encoding="utf-8")
PY
"$tmp_bin" outcome label --draft @"$tmp_dir/review-draft-reviewer-tie.json" > "$tmp_dir/review-label-reviewer-tie.json"
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
label = json.loads((root / "review-label-reviewer-tie.json").read_text(encoding="utf-8"))
(root / "review-labels-reviewer-tie.json").write_text(json.dumps([label], separators=(",", ":")), encoding="utf-8")
PY
"$tmp_bin" outcome review label-batch --assignment @"$tmp_dir/assignment-tie.json" --labels @"$tmp_dir/review-labels-reviewer-tie.json" \
  --committed-at 2026-08-09T15:30:00Z > "$tmp_dir/batch-tie.json"
"$tmp_bin" outcome review reveal --bundle @"$tmp_dir/review-bundle.json" \
  --left-assignment @"$tmp_dir/assignment-left.json" --left-batch @"$tmp_dir/batch-left.json" \
  --right-assignment @"$tmp_dir/assignment-right.json" --right-batch @"$tmp_dir/batch-right.json" \
  --tie-assignment @"$tmp_dir/assignment-tie.json" --tie-batch @"$tmp_dir/batch-tie.json" \
  --left-seed-file "$tmp_dir/review-seed-1" --right-seed-file "$tmp_dir/review-seed-2" --tie-seed-file "$tmp_dir/review-seed-3" \
  --mappings @"$tmp_dir/review-mappings.json" --revealed-at 2026-08-09T16:00:00Z --revealed-by audit-owner > "$tmp_dir/mapping-reveal.json"
"$tmp_bin" outcome review analyze-blinding --bundle @"$tmp_dir/review-bundle.json" --reveal @"$tmp_dir/mapping-reveal.json" \
  --mappings @"$tmp_dir/review-mappings.json" --left-probes @"$tmp_dir/blinding-probes-left.json" \
  --right-probes @"$tmp_dir/blinding-probes-right.json" --analyzed-at 2026-08-09T16:01:00Z > "$tmp_dir/blinding-analysis.json"
"$tmp_bin" outcome validate --type blinding-analysis --document @"$tmp_dir/blinding-analysis.json" > "$tmp_dir/blinding-analysis-validation.json"
"$tmp_bin" outcome review adjudicate --bundle @"$tmp_dir/review-bundle.json" \
  --left-assignment @"$tmp_dir/assignment-left.json" --left-batch @"$tmp_dir/batch-left.json" \
  --right-assignment @"$tmp_dir/assignment-right.json" --right-batch @"$tmp_dir/batch-right.json" \
  --tie-assignment @"$tmp_dir/assignment-tie.json" --tie-batch @"$tmp_dir/batch-tie.json" \
  --reveal @"$tmp_dir/mapping-reveal.json" --rubric-analysis @"$tmp_dir/rubric-ambiguity-analysis.json" \
  --blinding-analysis @"$tmp_dir/blinding-analysis.json" --completed-at 2026-08-09T16:05:00Z \
  --rule "third independent reviewer or unresolved" --bootstrap-iterations 10000 \
  --bootstrap-seed review-audit-v1 > "$tmp_dir/adjudication-result.json"
python3 - "$tmp_dir/adjudication-result.json" "$tmp_dir/adjudication-ledger.json" "$tmp_dir/adjudication-resolutions.json" <<'PY'
import json
import pathlib
import sys

result = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
pathlib.Path(sys.argv[2]).write_text(json.dumps(result["ledger"], separators=(",", ":")), encoding="utf-8")
pathlib.Path(sys.argv[3]).write_text(json.dumps(result["resolutions"], separators=(",", ":")), encoding="utf-8")
PY
"$tmp_bin" outcome validate --type adjudication-ledger --document @"$tmp_dir/adjudication-ledger.json" > "$tmp_dir/adjudication-ledger-validation.json"

python3 - "$tmp_dir" <<'PY'
import hashlib
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
bundle = json.loads((root / "review-bundle.json").read_text(encoding="utf-8"))
cases = [
    [("benchmark", "benchmark_reward", "solved"), ("claimed", "claimed_test_output", "solved"), ("independent", "independent_test_rerun", "solved")],
    [("benchmark", "benchmark_reward", "unsolved"), ("claimed", "claimed_test_output", "unsolved"), ("formal", "formal_mutation_relation", "solved"), ("independent", "independent_test_rerun", "solved")],
    [("claimed-a", "claimed_test_output", "solved"), ("claimed-b", "claimed_test_output", "unsolved"), ("independent", "independent_test_rerun", "unsolved")],
]
for case_index, (item, observations) in enumerate(zip(bundle["items"], cases)):
    for evidence_index, (evidence_id, kind, state) in enumerate(observations):
        independent = kind in {"independent_test_rerun", "formal_mutation_relation"}
        draft = {
            "id": evidence_id,
            "kind": kind,
            "state": state,
            "artifact_digest": hashlib.sha256(f"source-audit-{case_index}-{evidence_id}-{state}".encode()).hexdigest(),
            "observed_at": "2026-08-09T11:00:00Z",
            "independent": independent,
            "public": True,
            "limitation": "synthetic provider-free source-audit fixture",
            "parent_digests": [],
        }
        if independent:
            draft["validator_id"] = "audit-validator-v1"
        (root / f"source-evidence-draft-{case_index}-{evidence_index}.json").write_text(json.dumps(draft, separators=(",", ":")), encoding="utf-8")
PY
for draft in "$tmp_dir"/source-evidence-draft-*.json; do
  sealed=${draft/source-evidence-draft-/source-evidence-}
  "$tmp_bin" outcome evidence --draft @"$draft" > "$sealed"
done
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
bundle = json.loads((root / "review-bundle.json").read_text(encoding="utf-8"))
mappings = json.loads((root / "review-mappings.json").read_text(encoding="utf-8"))
resolutions = json.loads((root / "adjudication-resolutions.json").read_text(encoding="utf-8"))
mapping_by_packet = {item["packet_id"]: item for item in mappings}
resolution_by_packet = {item["packet_id"]: item for item in resolutions}
for case_index, item in enumerate(bundle["items"]):
    evidence = [json.loads(path.read_text(encoding="utf-8")) for path in sorted(root.glob(f"source-evidence-{case_index}-*.json"))]
    packet_id = item["packet"]["packet_id"]
    draft = {
        "task_alias": mapping_by_packet[packet_id]["source_task_alias"],
        "revision": 1,
        "evidence": evidence,
        "resolution": resolution_by_packet[packet_id]["state"],
        "resolution_basis": ["independent"],
        "limitations": ["synthetic provider-free source-audit fixture"],
        "author_id": "audit-fixture",
        "revision_reason": "provider-free source-audit fixture",
    }
    (root / f"source-record-draft-{case_index}.json").write_text(json.dumps(draft, separators=(",", ":")), encoding="utf-8")
PY
for index in 0 1 2; do
  "$tmp_bin" outcome record --draft @"$tmp_dir/source-record-draft-$index.json" > "$tmp_dir/source-record-$index.json"
done
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
records = [json.loads((root / f"source-record-{index}.json").read_text(encoding="utf-8")) for index in range(3)]
(root / "source-records.json").write_text(json.dumps(records, separators=(",", ":")), encoding="utf-8")
PY
"$tmp_bin" outcome review analyze-sources --bundle @"$tmp_dir/review-bundle.json" \
  --reveal @"$tmp_dir/mapping-reveal.json" --ledger @"$tmp_dir/adjudication-ledger.json" \
  --mappings @"$tmp_dir/review-mappings.json" --records @"$tmp_dir/source-records.json" \
  --resolutions @"$tmp_dir/adjudication-resolutions.json" --analyzed-at 2026-08-09T16:06:00Z \
  --bootstrap-iterations 10000 --bootstrap-seed source-audit-v1 > "$tmp_dir/source-audit.json"
"$tmp_bin" outcome validate --type source-audit --document @"$tmp_dir/source-audit.json" > "$tmp_dir/source-audit-validation.json"

python3 - "$tmp_dir/pairs.json" <<'PY'
import json
import pathlib
import sys

pairs = [
    {"packet_id": "packet-" + ("%064x" % index), "task_group_id": f"t{index}", "left": left, "right": right}
    for index, (left, right) in enumerate([
        ("solved", "solved"), ("solved", "unsolved"),
        ("unsolved", "unsolved"), ("unsolved", "solved"),
    ], start=1)
]
with pathlib.Path(sys.argv[1]).open("w", encoding="utf-8") as handle:
    json.dump(pairs, handle, separators=(",", ":"))
PY
"$tmp_bin" outcome agreement --pairs @"$tmp_dir/pairs.json" --bootstrap-iterations 10000 --seed outcome-audit-v1 > "$tmp_dir/agreement.json"

terminal_root="eval/trajectories/terminal_trajs/forge_gpt54"
swe_root="eval/trajectories/swebench_verified_trajs"
corpus_status="core-only"
if [ -d "$terminal_root" ] && [ -d "$swe_root" ]; then
  "$tmp_bin" mutation corpus build --root . --spec @eval/governance/controlled-corruption-v1.json > "$tmp_dir/release.json"
  "$tmp_bin" outcome sample --plan @eval/governance/outcome-adjudication-v1.json --release @"$tmp_dir/release.json" > "$tmp_dir/sample.json"
  python3 - "$tmp_dir/sample.json" eval/governance/outcome-mutation-sample-v1.json <<'PY'
import json
import pathlib
import sys

generated = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
governed = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
if generated != governed:
    raise SystemExit("outcome mutation sample commitment drifted from the governed corpus")
PY
  "$tmp_bin" outcome pilot-sample-v1 \
    --plan @eval/governance/outcome-adjudication-v1.json \
    --sample @eval/governance/outcome-mutation-sample-v1.json \
    --release @"$tmp_dir/release.json" \
    --inventory @eval/governance/outcome-natural-inventory-v1.json > "$tmp_dir/pilot-v1.json"
  python3 - "$tmp_dir/pilot-v1.json" eval/governance/outcome-pilot-sample-v1.json <<'PY'
import json
import pathlib
import sys

generated = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
governed = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
if generated != governed:
    raise SystemExit("historical mixed pilot v1 drifted from its governed mutation and natural inputs")
PY
  outcome_pilot_package="$tmp_dir/outcome-pilot-package"
  scripts/build/prepare-outcome-pilot.sh \
    --binary "$tmp_bin" \
    --key-file "$tmp_dir/blinding-key" \
    --key-id outcome-pilot-audit-v1 \
    --package-root "$outcome_pilot_package" \
    --bundle-created-at 2026-08-09T18:00:00Z \
    --protocol-created-at 2026-08-09T18:01:00Z \
    --readiness-prepared-at 2026-08-09T18:02:00Z > "$tmp_dir/outcome-pilot-prepare.log"
  python3 - "$outcome_pilot_package" "$tmp_dir/outcome-pilot-package-manifest.json" <<'PY'
import hashlib
import json
import pathlib
import stat
import sys

root = pathlib.Path(sys.argv[1])
entries = []
for path in [root, *sorted(root.rglob("*"))]:
    relative = "." if path == root else str(path.relative_to(root))
    mode = stat.S_IMODE(path.stat().st_mode)
    digest = hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None
    entries.append({"path": relative, "mode": mode, "sha256": digest})
pathlib.Path(sys.argv[2]).write_text(json.dumps(entries, sort_keys=True), encoding="utf-8")
PY
  if scripts/build/prepare-outcome-pilot.sh \
    --binary "$tmp_bin" \
    --key-file "$tmp_dir/blinding-key" \
    --key-id outcome-pilot-audit-v1 \
    --package-root "$outcome_pilot_package" \
    --bundle-created-at 2026-08-09T18:00:00Z \
    --protocol-created-at 2026-08-09T18:01:00Z \
    --readiness-prepared-at 2026-08-09T18:02:00Z > /dev/null 2>&1; then
    echo "existing outcome pilot package was replaced" >&2
    exit 1
  fi
  python3 - "$outcome_pilot_package" "$tmp_dir/outcome-pilot-package-manifest.json" <<'PY'
import hashlib
import json
import pathlib
import stat
import sys

root = pathlib.Path(sys.argv[1])
expected = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
observed = []
for path in [root, *sorted(root.rglob("*"))]:
    relative = "." if path == root else str(path.relative_to(root))
    mode = stat.S_IMODE(path.stat().st_mode)
    digest = hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None
    observed.append({"path": relative, "mode": mode, "sha256": digest})
if observed != expected:
    raise SystemExit("existing outcome pilot package changed after refused replacement")
PY
  outcome_pilot_private="$outcome_pilot_package/outcome-pilots/994dbbfb18c40494d1b664f4a6b45ac5b4182dcbd7f7f9293ba1f3e8eea03ff7.json"
  "$tmp_bin" outcome validate --type pilot-private-materials --document @"$outcome_pilot_private" > "$tmp_dir/outcome-pilot-private-validation.json"
  python3 - "$outcome_pilot_private" "$tmp_dir/outcome-pilot-bindings.json" <<'PY'
import json
import pathlib
import sys

private = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
pathlib.Path(sys.argv[2]).write_text(json.dumps(private["source_bindings"], separators=(",", ":")), encoding="utf-8")
PY
  "$tmp_bin" outcome validate --type pilot-inspection --document @"$outcome_pilot_package/pilot-inspection.json" > "$tmp_dir/outcome-pilot-inspection-validation.json"
  "$tmp_bin" outcome validate --type pilot-readiness --document @"$outcome_pilot_package/pilot-readiness.json" > "$tmp_dir/outcome-pilot-readiness-validation.json"
  corpus_status="full-corpus"
fi

python3 - "$tmp_dir" "$corpus_status" <<'PY'
import json
import math
import pathlib
import stat
import sys

root = pathlib.Path(sys.argv[1])
plan = json.loads((root / "plan.json").read_text(encoding="utf-8"))
sample = json.loads(pathlib.Path("eval/governance/outcome-mutation-sample-v1.json").read_text(encoding="utf-8"))
agreement = json.loads((root / "agreement.json").read_text(encoding="utf-8"))
validation = json.loads((root / "sample-validation.json").read_text(encoding="utf-8"))
pilot_v1 = json.loads(pathlib.Path("eval/governance/outcome-pilot-sample-v1.json").read_text(encoding="utf-8"))
pilot = json.loads(pathlib.Path("eval/governance/outcome-pilot-sample-v2.json").read_text(encoding="utf-8"))
pilot_v1_validation = json.loads((root / "pilot-v1-validation.json").read_text(encoding="utf-8"))
pilot_validation = json.loads((root / "pilot-validation.json").read_text(encoding="utf-8"))
natural = json.loads((root / "natural-inventory.json").read_text(encoding="utf-8"))
natural_validation = json.loads((root / "natural-validation.json").read_text(encoding="utf-8"))
packets = [json.loads((root / f"public-packet-{index}.json").read_text(encoding="utf-8")) for index in range(1, 4)]
packet_validations = [json.loads((root / f"packet-validation-{index}.json").read_text(encoding="utf-8")) for index in range(1, 4)]
qualification_default = json.loads((root / "qualification-default.json").read_text(encoding="utf-8"))
qualification_governed = json.loads((root / "qualification-governed.json").read_text(encoding="utf-8"))
qualification_validation = json.loads((root / "qualification-validation.json").read_text(encoding="utf-8"))
qualification_reports = [json.loads((root / f"qualification-report-{reviewer}.json").read_text(encoding="utf-8")) for reviewer in ("reviewer-left", "reviewer-right", "reviewer-tie")]
handbook_default = json.loads((root / "handbook-default.json").read_text(encoding="utf-8"))
handbook_governed = json.loads((root / "handbook-governed.json").read_text(encoding="utf-8"))
handbook_validation = json.loads((root / "handbook-validation.json").read_text(encoding="utf-8"))
bundle = json.loads((root / "review-bundle.json").read_text(encoding="utf-8"))
reviewer_kits = [json.loads((root / f"reviewer-kit-{reviewer}.json").read_text(encoding="utf-8")) for reviewer in ("left", "right", "tie")]
reviewer_kit_validations = [json.loads((root / f"reviewer-kit-{reviewer}-validation.json").read_text(encoding="utf-8")) for reviewer in ("left", "right", "tie")]
blinding_protocol = json.loads((root / "blinding-protocol.json").read_text(encoding="utf-8"))
blinding_protocol_validation = json.loads((root / "blinding-protocol-validation.json").read_text(encoding="utf-8"))
blinding_probe_validations = [json.loads((root / f"blinding-probes-{reviewer}-validation.json").read_text(encoding="utf-8")) for reviewer in ("left", "right")]
blinding_analysis = json.loads((root / "blinding-analysis.json").read_text(encoding="utf-8"))
blinding_analysis_validation = json.loads((root / "blinding-analysis-validation.json").read_text(encoding="utf-8"))
rubric_ambiguity = json.loads((root / "rubric-ambiguity-analysis.json").read_text(encoding="utf-8"))
rubric_ambiguity_validation = json.loads((root / "rubric-ambiguity-validation.json").read_text(encoding="utf-8"))
adjudication = json.loads((root / "adjudication-result.json").read_text(encoding="utf-8"))
adjudication_validation = json.loads((root / "adjudication-ledger-validation.json").read_text(encoding="utf-8"))
source_audit = json.loads((root / "source-audit.json").read_text(encoding="utf-8"))
source_audit_validation = json.loads((root / "source-audit-validation.json").read_text(encoding="utf-8"))
if plan["digest"] != "2b7fa309bdf2a4151c640275f858342624fddfbf5d6ada316e5f996ef14dd46e":
    raise SystemExit("outcome adjudication plan digest changed")
if sample["digest"] != "883b0164746c9e6fcaccc07f6fe3d0fd4872cbc35c08983599b48e4f6e605d06" or sample["selected_cases"] != 31:
    raise SystemExit("outcome mutation sample commitment changed")
if validation.get("valid") is not True:
    raise SystemExit("outcome sample validation failed")
if pilot_v1_validation.get("valid") is not True or pilot_v1["digest"] != "76436e72015a55be5bcaa6c83ea1b6fdc053fb01004ca940b4d663d882a6beb0":
    raise SystemExit("historical mixed pilot v1 validation failed")
if pilot_v1["selected_cases"] != 14 or pilot_v1["required_primary_labels"] != 28 or pilot_v1["maximum_tie_break_labels"] != 14 or pilot_v1["required_source_probes"] != 28:
    raise SystemExit("historical mixed pilot v1 workload changed")
if pilot_validation.get("valid") is not True or pilot["digest"] != "994dbbfb18c40494d1b664f4a6b45ac5b4182dcbd7f7f9293ba1f3e8eea03ff7":
    raise SystemExit("outcome-only pilot v2 validation failed")
if pilot["objective"] != "single_trajectory_outcome" or pilot["selected_cases"] != 6 or pilot["required_primary_labels"] != 12 or pilot["maximum_tie_break_labels"] != 6 or pilot["required_source_probes"] != 12:
    raise SystemExit("outcome pilot workload changed")
if len(pilot["selected_natural_strata"]) != 6 or pilot["unavailable_natural_strata"] != ["abstention", "provider_failure"] or any(case["objective"] != "single_trajectory_outcome" for case in pilot["cases"]):
    raise SystemExit("outcome pilot coverage or explicit natural shortfalls changed")
if natural_validation.get("valid") is not True or natural["digest"] != "054a9b1e41a7507b780a8cd74052df510889897253e18f6d7093511e18ea5b8e":
    raise SystemExit("natural outcome inventory validation failed")
if natural["status"] != "incomplete" or natural["selected_cases"] != 36 or [item["stratum"] for item in natural["shortfalls"]] != ["abstention", "provider_failure"]:
    raise SystemExit("natural outcome inventory must expose the two real prespecified shortfalls")
task_groups = [item["task_group_digest"] for item in natural["selections"]]
if len(task_groups) != len(set(task_groups)):
    raise SystemExit("natural outcome inventory reused a task group across strata")
if qualification_validation.get("valid") is not True or qualification_default != qualification_governed:
    raise SystemExit("outcome qualification set drifted from executable default")
if qualification_governed["digest"] != "a2ce822020ac6a3050de35b73ba645741e823369875fe32ef707e2d3143bd2d7" or len(qualification_governed["cases"]) != 5:
    raise SystemExit("outcome qualification set identity or case count changed")
if any(report["qualified"] is not True or report["score"] != 1 for report in qualification_reports):
    raise SystemExit("outcome qualification scoring failed its exact-answer fixture")
if handbook_validation.get("valid") is not True or handbook_default != handbook_governed:
    raise SystemExit("reviewer handbook drifted from its governed executable default")
if handbook_governed["digest"] != "c4047894da6959f38a1f5766e2e7bf0b762567639089aba3cead04e4ef1f1855" or bundle["handbook_digest"] != handbook_governed["digest"]:
    raise SystemExit("review bundle does not bind the governed reviewer handbook")
if any(validation.get("valid") is not True for validation in reviewer_kit_validations) or [len(kit["packets"]) for kit in reviewer_kits] != [3, 3, 1]:
    raise SystemExit("self-contained reviewer kit generation or verification failed")
rendered_kits = "\n".join((root / f"reviewer-kit-{reviewer}.md").read_text(encoding="utf-8") for reviewer in ("left", "right", "tie"))
for required in ["# EvalWitness Blinded Review Kit", "## Decision procedure", "## Dataset statement", "## Submission checklist"]:
    if required not in rendered_kits:
        raise SystemExit(f"rendered reviewer kit omitted {required!r}")
for forbidden in ["natural-task-opaque", "presentation_invariance", "falsified_test_evidence", "quality_equal", "audit-key-v1", "source-candidate-one", "deepseek", "verifier_winner"]:
    if forbidden in rendered_kits:
        raise SystemExit(f"rendered reviewer kit leaked {forbidden!r}")
if any(validation.get("valid") is not True for validation in packet_validations) or any(not packet["packet_id"].startswith("packet-") for packet in packets):
    raise SystemExit("blinded packet validation failed")
public_text = json.dumps(packets, sort_keys=True)
for forbidden in ["natural-task-opaque", "presentation_invariance", "falsified_test_evidence", "quality_equal", "audit-key-v1", "source-candidate-one", "deepseek", "verifier_winner"]:
    if forbidden in public_text:
        raise SystemExit(f"blinded packet leaked {forbidden!r}")
mapping_files = list((root / "private-vault" / "mappings").glob("*.json"))
if len(mapping_files) != 3 or any(path.stat().st_mode & 0o077 for path in mapping_files):
    raise SystemExit("private outcome mappings were not stored exactly once with owner-only permissions")
mapping_task_aliases = {json.loads(path.read_text(encoding="utf-8"))["source_task_alias"] for path in mapping_files}
if mapping_task_aliases != {f"natural-task-opaque-{index}" for index in range(1, 4)}:
    raise SystemExit("private outcome mappings did not retain the original task aliases")
if agreement["raw_agreement"] != 0.5 or agreement["cohen_kappa"] != 0 or agreement["cohen_kappa_defined"] is not True:
    raise SystemExit("outcome agreement hand-check changed")
if blinding_protocol_validation.get("valid") is not True or blinding_protocol["condition_candidates"] != ["falsified_test_evidence", "presentation_invariance"]:
    raise SystemExit("semantic blinding protocol did not freeze the complete condition universe")
if any(validation.get("valid") is not True for validation in blinding_probe_validations) or blinding_analysis_validation.get("valid") is not True:
    raise SystemExit("semantic blinding probe commitments or analysis failed validation")
if blinding_analysis["observations"] != 6 or blinding_analysis["attempts"] != 5 or blinding_analysis["correct"] != 4 or blinding_analysis["recognized_tasks"] != 1:
    raise SystemExit("semantic blinding analysis denominators changed")
if not math.isclose(blinding_analysis["condition_guess_accuracy"], 2 / 3) or not math.isclose(blinding_analysis["selective_accuracy"], 0.8) or not math.isclose(blinding_analysis["cohen_kappa"], 0.4):
    raise SystemExit("semantic blinding analysis hand-check changed")
if rubric_ambiguity_validation.get("valid") is not True:
    raise SystemExit("prereveal rubric ambiguity analysis failed validation")
if rubric_ambiguity["packets"] != 3 or rubric_ambiguity["label_observations"] != 6 or rubric_ambiguity["axis_comparisons"] != 15:
    raise SystemExit("rubric ambiguity denominators changed")
if rubric_ambiguity["primary_outcome_disagreements"] != 1 or rubric_ambiguity["any_axis_disagreements"] != 1 or rubric_ambiguity["unclear_ratings"] != 1:
    raise SystemExit("rubric ambiguity counts changed")
if rubric_ambiguity["exact_reason_matches"] != 2 or rubric_ambiguity["zero_reason_overlaps"] != 1 or not math.isclose(rubric_ambiguity["mean_reason_jaccard_distance"], 1 / 3):
    raise SystemExit("rubric reason-code divergence hand-check changed")
if not math.isclose(rubric_ambiguity["exact_reason_match_rate"], 2 / 3) or not math.isclose(rubric_ambiguity["zero_reason_overlap_rate"], 1 / 3):
    raise SystemExit("rubric reason-code divergence rates changed")
axis_metrics = {item["axis"]: item for item in rubric_ambiguity["axis_metrics"]}
for axis in ("technical_correctness", "verification_quality", "evidence_sufficiency"):
    if axis_metrics[axis]["disagreements"] != 1:
        raise SystemExit(f"rubric axis disagreement changed for {axis}")
if adjudication_validation.get("valid") is not True or adjudication["ledger"]["status"] != "complete" or len(adjudication["resolutions"]) != 3:
    raise SystemExit("review commit-reveal adjudication workflow failed")
if adjudication["ledger"]["blinding_analysis_digest"] != blinding_analysis["digest"]:
    raise SystemExit("terminal adjudication ledger did not bind semantic blinding evidence")
if adjudication["ledger"]["rubric_ambiguity_digest"] != rubric_ambiguity["digest"]:
    raise SystemExit("terminal adjudication ledger did not bind prereveal rubric ambiguity evidence")
if source_audit_validation.get("valid") is not True or source_audit["adjudication_ledger_digest"] != adjudication["ledger"]["digest"]:
    raise SystemExit("post-ledger outcome source audit failed validation or terminal-ledger binding")
if source_audit["packets"] != 3 or source_audit["source_evidence_observations"] != 10 or source_audit["packets_with_internal_conflict"] != 1:
    raise SystemExit("outcome source audit denominators changed")
if source_audit["packets_with_any_disagreement"] != 1 or source_audit["benchmark_comparable_packets"] != 2 or source_audit["benchmark_changed_packets"] != 1:
    raise SystemExit("outcome source audit disagreement or benchmark transition counts changed")
pair_metrics = {(item["left"], item["right"]): item for item in source_audit["pair_metrics"]}
benchmark_human = pair_metrics[("benchmark_reward", "human_adjudication")]
independent_human = pair_metrics[("independent_test_rerun", "human_adjudication")]
if benchmark_human["comparable_cases"] != 2 or benchmark_human["agreements"] != 1 or benchmark_human["disagreements"] != 1:
    raise SystemExit("benchmark-to-human source reconciliation changed")
if independent_human["comparable_cases"] != 3 or independent_human["agreements"] != 3 or independent_human["cohen_kappa"] != 1:
    raise SystemExit("independent-rerun-to-human source reconciliation changed")
reviewability_summary = "not-materialized"
if sys.argv[2] == "full-corpus":
    package = root / "outcome-pilot-package"
    expected_package_files = {
        ".evalwitness-cache-root.json",
        "blinding-protocol.json",
        "outcome-pilots/994dbbfb18c40494d1b664f4a6b45ac5b4182dcbd7f7f9293ba1f3e8eea03ff7.json",
        "pilot-inspection.json",
        "pilot-readiness.json",
        "restricted-review-items.json",
        "review-bundle.json",
    }
    observed_package_files = {str(path.relative_to(package)) for path in package.rglob("*") if path.is_file()}
    if observed_package_files != expected_package_files:
        raise SystemExit("prepared outcome pilot package file set changed")
    if any(path.stat().st_mode & (stat.S_IRWXG | stat.S_IRWXO) for path in [package, *package.rglob("*")]):
        raise SystemExit("prepared outcome pilot package grants group or other access")
    prepare_log = (root / "outcome-pilot-prepare.log").read_text(encoding="utf-8")
    if "external_action_status=not_authorized" not in prepare_log:
        raise SystemExit("outcome pilot preparation weakened its external-action boundary")
    outcome_items = json.loads((package / "restricted-review-items.json").read_text(encoding="utf-8"))
    outcome_bindings = json.loads((root / "outcome-pilot-bindings.json").read_text(encoding="utf-8"))
    outcome_private_validation = json.loads((root / "outcome-pilot-private-validation.json").read_text(encoding="utf-8"))
    outcome_inspection = json.loads((package / "pilot-inspection.json").read_text(encoding="utf-8"))
    outcome_inspection_validation = json.loads((root / "outcome-pilot-inspection-validation.json").read_text(encoding="utf-8"))
    outcome_readiness = json.loads((package / "pilot-readiness.json").read_text(encoding="utf-8"))
    outcome_readiness_validation = json.loads((root / "outcome-pilot-readiness-validation.json").read_text(encoding="utf-8"))
    if len(outcome_items) != 6 or len(outcome_bindings) != 6 or len({item["task_group_id"] for item in outcome_items}) != 6:
        raise SystemExit("outcome pilot did not materialize six unique natural review units")
    if outcome_private_validation.get("valid") is not True:
        raise SystemExit("outcome pilot owner-only custody root is not a sealed valid artifact")
    if any(item["packet"]["public_releasable"] or item["packet"]["privacy_class"] != "restricted_reference_only" for item in outcome_items):
        raise SystemExit("reference-only outcome pilot material escaped the restricted bundle boundary")
    if any(sorted(evidence["kind"] for evidence in item["packet"]["evidence"]) != ["task_requirement", "trajectory_evidence"] for item in outcome_items):
        raise SystemExit("outcome pilot packet contains a non-outcome or pair-level evidence unit")
    expected_bindings = {
        ("baseline_disagreement", "swebench", "django__django-11400", 2, 1),
        ("high_confidence_error", "swebench", "django__django-14725", 0, 0),
        ("random_control", "terminal-bench", "large-scale-text-editing", 4, 1),
        ("verifier_correct", "swebench", "django__django-13512", 2, 1),
        ("verifier_judge_disagreement", "swebench", "pylint-dev__pylint-6528", 1, 1),
        ("verifier_wrong", "swebench", "sphinx-doc__sphinx-7985", 1, 0),
    }
    observed_bindings = {(item["stratum"], item["suite"], item["task_id"], item["selected_index"], item["selected_reward"]) for item in outcome_bindings}
    if observed_bindings != expected_bindings or any(item["evidence_budget_tokens"] != 16000 or item["redistribution"] != "reference_only" for item in outcome_bindings):
        raise SystemExit("outcome pilot raw-source resolution, reward binding, evidence budget, or redistribution boundary changed")
    if any(item["schema_version"] != "evalwitness.outcome-pilot-source-binding.v2" or item["evidence_selector"] != "evalwitness.outcome-evidence-selector.v2" for item in outcome_bindings):
        raise SystemExit("outcome pilot source binding or anchored evidence-selector policy changed")
    if any(item["decision_anchor_kind"] not in {"file_change", "message"} or len(item["decision_anchor_digest"]) != 64 for item in outcome_bindings):
        raise SystemExit("outcome pilot evidence lost its final-patch or assistant-narrative decision anchor")
    if any(item["retained_messages"] < 0 or item["retained_actions"] < 1 or item["retained_results"] < 1 for item in outcome_bindings):
        raise SystemExit("outcome pilot evidence lost action or result coverage")
    if outcome_inspection_validation.get("valid") is not True or outcome_inspection["packets"] != 6:
        raise SystemExit("outcome pilot reviewability inspection failed validation or packet coverage")
    if outcome_inspection["reviewability_status"] != "structurally_ready" or outcome_inspection["semantic_status"] != "requires_human_pilot":
        raise SystemExit("outcome pilot inspection collapsed structural readiness into semantic validity")
    if outcome_inspection["packets_with_messages"] + outcome_inspection["packets_without_messages"] != 6 or outcome_inspection["patch_anchors"] + outcome_inspection["narrative_anchors"] != 6:
        raise SystemExit("outcome pilot inspection lost message or decision-anchor denominators")
    if any(item["reviewability_status"] != "structurally_ready" or item["retained_actions"] < 1 or item["retained_results"] < 1 for item in outcome_inspection["items"]):
        raise SystemExit("outcome pilot inspection accepted a structurally incomplete packet")
    outcome_packet_text = json.dumps(outcome_items, sort_keys=True).lower()
    route_forbidden = ["__thought__", "forge agent", "claude opus", "terminal_bench.trajectory", "atif-v1.5", "selected_index", "selected_reward", "benchmark_reward"]
    route_forbidden.extend(item["task_id"].lower() for item in outcome_bindings)
    if any(value in outcome_packet_text for value in route_forbidden):
        raise SystemExit("outcome pilot packet leaked a task, provider, route, reward, or selection identity")
    if outcome_readiness_validation.get("valid") is not True or outcome_readiness["schema_version"] != "evalwitness.outcome-pilot-readiness.v3" or outcome_readiness["objective"] != "single_trajectory_outcome" or outcome_readiness["packets"] != 6:
        raise SystemExit("outcome pilot v3 readiness failed objective or packet coverage validation")
    if outcome_readiness["inspection_digest"] != outcome_inspection["digest"]:
        raise SystemExit("outcome pilot readiness did not bind the reproduced reviewability inspection")
    if outcome_readiness["technical_status"] != "ready_for_authorization" or outcome_readiness["external_action_status"] != "not_authorized" or outcome_readiness["visibility"] != "restricted":
        raise SystemExit("outcome pilot readiness weakened the authorization or redistribution boundary")
    reviewability_summary = (
        f"events={outcome_inspection['total_retained_events']}/{outcome_inspection['total_source_events']} "
        f"messages={outcome_inspection['packets_with_messages']}/{outcome_inspection['packets']} "
        f"anchors={outcome_inspection['patch_anchors']}patch+{outcome_inspection['narrative_anchors']}narrative"
    )
print(
    "Outcome validity passed: "
    f"plan={plan['digest']} mutation_sample={sample['selected_cases']} natural_sample={natural['selected_cases']}/{natural['target_cases']} "
    f"natural_shortfalls=abstention,provider_failure mixed_v1=non-launchable outcome_pilot={pilot['selected_cases']} pilot_primary_labels={pilot['required_primary_labels']} private_custody=sealed reviewability=structurally_ready/semantic_pending {reviewability_summary} families={len(sample['family_counts'])} schemas=40 "
    f"blinded_packet=sealed qualification_cases={len(qualification_governed['cases'])} reviewer_kit=self-contained rubric_ambiguity=measured semantic_blinding=measured source_audit=post-ledger-performance-blind review_workflow=commit-reveal agreement_bootstrap=10000 "
    f"corpus_check={sys.argv[2]} providers=not_invoked"
)
PY
