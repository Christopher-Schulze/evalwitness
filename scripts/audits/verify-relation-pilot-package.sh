#!/usr/bin/env bash
# Independently verify a prepared owner-only TASK 068 relation pilot package.

set -euo pipefail
umask 077

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)

binary=""
package_root=""

usage() {
  cat >&2 <<'EOF'
usage: scripts/audits/verify-relation-pilot-package.sh \
  --package-root PATH [--binary PATH]

Verifies file inventory, owner-only permissions, SHA-256 integrity, every sealed
artifact, public-safe launch-brief scanning, version-aware package inventory, and
independent bundle/readiness/dossier/brief/change-receipt/change-atlas/workbook
reproduction. Package v5 also reconstructs all restricted scarcity-sentinel
materials and its owner-only appendix. No provider, reviewer, key, or network
action is used.
EOF
}

while (($# > 0)); do
  case "$1" in
    --binary)
      binary=${2:-}
      shift 2
      ;;
    --package-root)
      package_root=${2:-}
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$package_root" || ! -d "$package_root" || -L "$package_root" ]]; then
  echo "error: --package-root must name an existing non-symlink directory" >&2
  exit 2
fi

cd "$repo_root"
work_dir=$(mktemp -d /tmp/evalwitness-relation-pilot-verify-XXXXXXXX)
trap 'rm -rf "$work_dir"' EXIT

if [[ -z "$binary" ]]; then
  binary="$work_dir/evalwitness"
  go build -o "$binary" ./cmd/evalwitness
elif [[ ! -f "$binary" || -L "$binary" || ! -x "$binary" ]]; then
  echo "error: --binary must name an executable regular non-symlink file" >&2
  exit 2
fi

while IFS= read -r -d '' path; do
  if [[ -L "$path" ]]; then
    echo "error: relation pilot package contains a symlink: $path" >&2
    exit 1
  fi
  mode=$(stat -f '%Lp' "$path" 2>/dev/null || stat -c '%a' "$path")
  if [[ -d "$path" && "$mode" != 700 ]]; then
    echo "error: relation pilot package directory is not mode 0700: $path" >&2
    exit 1
  fi
  if [[ -f "$path" && "$mode" != 600 ]]; then
    echo "error: relation pilot package file is not mode 0600: $path" >&2
    exit 1
  fi
done < <(find "$package_root" -print0)

(cd "$package_root" && shasum -a 256 -c SHA256SUMS > /dev/null)

python3 - "$package_root" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
summary = json.loads((root / "package-summary.json").read_text(encoding="utf-8"))
package_format = summary.get("format_version")
expected_cases = 8
fixed = {
    ".evalwitness-cache-root.json",
    "OPERATING-INSTRUCTIONS.md",
    "SHA256SUMS",
    "blind-packets.json",
    "controlled-corruption-release.json",
    "owner-inspection.md",
    "package-summary.json",
    "pilot-launch-brief.md",
    "pilot-launch-dossier.json",
    "pilot-launch-dossier.schema.json",
    "pilot-inspection.schema.json",
    "pilot-readiness.json",
    "private-mappings.json",
    "qualification-set.json",
    "relation-audit-plan.json",
    "relation-pilot-sample.json",
    "relation-primary-sample.json",
    "relation-study-amendment.json",
    "review-bundle.json",
    "reviewer-handbook.json",
}
if package_format == "evalwitness.relation-pilot-package.v1":
    if summary.get("change_atlas_policy") is not None:
        fixed.add("owner-change-atlas.md")
elif package_format in {"evalwitness.relation-pilot-package.v2", "evalwitness.relation-pilot-package.v3"}:
    fixed.update({"owner-change-atlas.md", "pilot-change-receipt.json", "pilot-change-receipt.schema.json"})
elif package_format in {"evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"}:
    expected_cases = 7
    fixed.update({
        "construct-firewall-challenge.json",
        "construct-repair-evidence.json",
        "controlled-corruption-audit.json",
        "controlled-corruption-plan.json",
        "owner-change-atlas.md",
        "package-inventory.json",
        "pilot-change-receipt.json",
        "pilot-change-receipt.schema.json",
        "relation-scarcity-sentinel.json",
    })
    if package_format == "evalwitness.relation-pilot-package.v5":
        fixed.add("owner-scarcity-inspection.md")
else:
    raise SystemExit("relation pilot package format version is unsupported")
files = {str(path.relative_to(root)) for path in root.rglob("*") if path.is_file()}
directories = {str(path.relative_to(root)) for path in root.rglob("*") if path.is_dir()}
variable = files - fixed
groups = {
    "materials": sorted(path for path in variable if path.startswith("materials/")),
    "packets": sorted(path for path in variable if path.startswith("packets/")),
    "mappings": sorted(path for path in variable if path.startswith("mappings/")),
    "qualification_keys": sorted(path for path in variable if path.startswith("qualification-keys/")),
    "sentinel_materials": sorted(path for path in variable if path.startswith("sentinel-materials/")),
}
if fixed - files or set().union(*groups.values()) != variable:
    raise SystemExit("relation pilot package file inventory is incomplete or contains an unknown file")
expected_directories = {"mappings", "materials", "packets", "qualification-keys"}
if package_format == "evalwitness.relation-pilot-package.v5":
    expected_directories.add("sentinel-materials")
if directories != expected_directories:
    raise SystemExit("relation pilot package directory inventory is invalid")
if len(groups["materials"]) != expected_cases or len(groups["packets"]) != expected_cases or len(groups["mappings"]) != expected_cases or len(groups["qualification_keys"]) != 1:
    raise SystemExit("relation pilot package custody counts are invalid")
if groups["materials"] != [f"materials/{index:02d}.json" for index in range(1, expected_cases + 1)] or groups["packets"] != [f"packets/{index:02d}.json" for index in range(1, expected_cases + 1)]:
    raise SystemExit("relation pilot package material or packet ordering is invalid")
if package_format == "evalwitness.relation-pilot-package.v5" and groups["sentinel_materials"] != [f"sentinel-materials/{index:02d}.json" for index in range(1, 4)]:
    raise SystemExit("relation pilot package scarcity material custody is invalid")
if package_format != "evalwitness.relation-pilot-package.v5" and groups["sentinel_materials"]:
    raise SystemExit("historical relation pilot package contains unsupported scarcity materials")
if package_format in {"evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"}:
    import hashlib
    inventory = json.loads((root / "package-inventory.json").read_text(encoding="utf-8"))
    inventory_body = {key: value for key, value in inventory.items() if key != "digest"}
    inventory_digest = hashlib.sha256(json.dumps(inventory_body, sort_keys=True, separators=(",", ":")).encode("utf-8")).hexdigest()
    if inventory.get("digest") != inventory_digest:
        raise SystemExit("relation pilot package inventory digest is invalid")
    if inventory_body.get("schema_version") != "evalwitness.relation-pilot-package-inventory.v1" or inventory_body.get("package_format") != package_format or inventory_body.get("hash_algorithm") != "sha256" or inventory_body.get("scope") != "all_package_payload_files_except_inventory_and_sha256sums":
        raise SystemExit("relation pilot package inventory identity is invalid")
    expected_directories = [{"path": path, "mode": "0700"} for path in sorted(directories)]
    expected_files = []
    for relative in sorted(files - {"package-inventory.json", "SHA256SUMS"}):
        content = (root / relative).read_bytes()
        expected_files.append({"path": relative, "bytes": len(content), "mode": "0600", "sha256": hashlib.sha256(content).hexdigest()})
    if inventory_body.get("directories") != expected_directories or inventory_body.get("files") != expected_files or inventory_body.get("payload_files") != len(expected_files) or inventory_body.get("payload_bytes") != sum(item["bytes"] for item in expected_files):
        raise SystemExit("relation pilot package inventory does not exactly reproduce its payload")
schema_generation = "v2" if package_format == "evalwitness.relation-pilot-package.v3" else "v3" if package_format in {"evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"} else "v1"
schema_files = {
    "pilot-inspection.schema.json": f"https://evalwitness.dev/schemas/relation-pilot-inspection.{schema_generation}.json",
    "pilot-launch-dossier.schema.json": f"https://evalwitness.dev/schemas/relation-pilot-launch-dossier.{schema_generation}.json",
}
if "pilot-change-receipt.schema.json" in files:
    schema_files["pilot-change-receipt.schema.json"] = f"https://evalwitness.dev/schemas/relation-pilot-change-receipt.{schema_generation}.json"
for relative, expected_id in schema_files.items():
    schema = json.loads((root / relative).read_text(encoding="utf-8"))
    if schema.get("$id") != expected_id:
        raise SystemExit("relation pilot package contains a schema from the wrong protocol generation")
release = json.loads((root / "controlled-corruption-release.json").read_text(encoding="utf-8"))
primary = json.loads((root / "relation-primary-sample.json").read_text(encoding="utf-8"))
pilot = json.loads((root / "relation-pilot-sample.json").read_text(encoding="utf-8"))
amendment = json.loads((root / "relation-study-amendment.json").read_text(encoding="utf-8"))
qualification = json.loads((root / "qualification-set.json").read_text(encoding="utf-8"))
handbook = json.loads((root / "reviewer-handbook.json").read_text(encoding="utf-8"))
bundle = json.loads((root / "review-bundle.json").read_text(encoding="utf-8"))
readiness = json.loads((root / "pilot-readiness.json").read_text(encoding="utf-8"))
dossier = json.loads((root / "pilot-launch-dossier.json").read_text(encoding="utf-8"))
required = {
    "review_objective": "controlled_relation",
    "packets": expected_cases,
    "technical_status": "structurally_ready_for_owner_semantic_review",
    "semantic_inspection_status": "requires_owner_manual_inspection",
    "owner_inspection_status": "not_completed",
    "launch_status": "not_launchable_pending_owner_inspection_and_authorization",
    "human_study_status": "not_run",
    "external_action_status": "not_authorized",
    "launch_brief_policy": "evalwitness.relation-pilot-launch-brief.v1",
    "launch_brief_public_safety_status": "passed",
}
if package_format == "evalwitness.relation-pilot-package.v1" and "owner-change-atlas.md" in fixed:
    required.update({
        "change_atlas_policy": "evalwitness.relation-pilot-change-atlas.v1",
        "change_atlas_status": "generated_owner_only_no_decisions",
    })
if package_format in {"evalwitness.relation-pilot-package.v2", "evalwitness.relation-pilot-package.v3", "evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"}:
    required.update({
        "change_atlas_policy": "evalwitness.relation-pilot-change-atlas.v1",
        "change_atlas_status": "generated_owner_only_receipt_bound_no_decisions",
        "change_receipt_schema_version": {"evalwitness.relation-pilot-package.v2": "evalwitness.relation-pilot-change-receipt.v1", "evalwitness.relation-pilot-package.v3": "evalwitness.relation-pilot-change-receipt.v2", "evalwitness.relation-pilot-package.v4": "evalwitness.relation-pilot-change-receipt.v3", "evalwitness.relation-pilot-package.v5": "evalwitness.relation-pilot-change-receipt.v3"}[package_format],
        "change_receipt_policy": "evalwitness.relation-pilot-change-receipt.v1",
        "change_receipt_status": "verified_owner_only_no_raw_content_no_decisions",
    })
if any(summary.get(key) != value for key, value in required.items()):
    raise SystemExit("relation pilot package summary weakened its scope or status boundary")
if len(summary.get("families", [])) != expected_cases or len(set(summary["families"])) != expected_cases:
    raise SystemExit("relation pilot package summary lost family coverage")
expected_digests = {
    "source_corpus_digest": release["digest"],
    "primary_sample_digest": primary["digest"],
    "pilot_sample_digest": pilot["digest"],
    "study_amendment_digest": amendment["digest"],
    "qualification_set_digest": qualification["digest"],
    "handbook_digest": handbook["digest"],
    "bundle_digest": bundle["digest"],
    "readiness_digest": readiness["digest"],
    "launch_dossier_digest": dossier["digest"],
    "mapping_commitment_digest": readiness["mapping_commitment_digest"],
}
if package_format in {"evalwitness.relation-pilot-package.v2", "evalwitness.relation-pilot-package.v3", "evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"}:
    change_receipt = json.loads((root / "pilot-change-receipt.json").read_text(encoding="utf-8"))
    expected_digests["change_receipt_digest"] = change_receipt["digest"]
    if (
        change_receipt.get("readiness_digest") != readiness["digest"]
        or change_receipt.get("bundle_digest") != bundle["digest"]
        or change_receipt.get("mapping_commitment_digest") != readiness["mapping_commitment_digest"]
        or change_receipt.get("packets") != expected_cases
        or change_receipt.get("trajectory_pairs") != expected_cases - 1
        or change_receipt.get("candidate_order_controls") != 1
        or change_receipt.get("decision_status") != "not_recorded"
        or change_receipt.get("human_study_status") != "not_run"
        or change_receipt.get("external_action_status") != "not_authorized"
    ):
        raise SystemExit("relation pilot change receipt weakened its evidence or claim boundary")
    forbidden_receipt_fields = {"content", "task_requirement", "task_requirement_text", "trajectory"}
    def contains_forbidden_field(value):
        if isinstance(value, dict):
            return any(key in forbidden_receipt_fields or contains_forbidden_field(item) for key, item in value.items())
        if isinstance(value, list):
            return any(contains_forbidden_field(item) for item in value)
        return False
    if contains_forbidden_field(change_receipt):
        raise SystemExit("relation pilot change receipt contains raw task or trajectory content")
if package_format == "evalwitness.relation-pilot-package.v3":
    plan = json.loads((root / "relation-audit-plan.json").read_text(encoding="utf-8"))
    answer_key = json.loads((root / groups["qualification_keys"][0]).read_text(encoding="utf-8"))
    versioned = {
        "relation_protocol_version": "evalwitness.controlled-relation-review.v2",
        "source_corpus_version": "evalwitness-controlled-corruption.v2",
        "source_corpus_spec_digest": release["spec_digest"],
        "source_mutation_program_digest": release["mutation_program_digest"],
        "source_construct_audit_digest": release["spec"]["development_audit"]["construct_audit_digest"],
        "construct_firewall_commitment_digest": readiness["construct_firewall_commitment_digest"],
        "plan_schema_version": "evalwitness.relation-audit-plan.v2",
        "primary_sample_schema_version": "evalwitness.relation-primary-sample.v2",
        "pilot_sample_schema_version": "evalwitness.relation-pilot-sample.v2",
        "study_amendment_schema_version": "evalwitness.relation-study-amendment.v2",
        "qualification_set_schema_version": "evalwitness.relation-qualification-set.v2",
        "handbook_schema_version": "evalwitness.relation-reviewer-handbook.v2",
        "bundle_schema_version": "evalwitness.relation-review-bundle.v2",
        "readiness_schema_version": "evalwitness.relation-pilot-readiness.v2",
        "launch_dossier_schema_version": "evalwitness.relation-pilot-launch-dossier.v2",
        "primary_cases": 32,
        "primary_task_groups": 32,
        "primary_lineage_clusters": 24,
        "pilot_task_groups": 8,
        "pilot_lineage_clusters": 8,
        "pilot_primary_overlap": 0,
    }
    if any(summary.get(key) != value for key, value in versioned.items()):
        raise SystemExit("relation pilot package v3 lost a v2 governance, source, construct, or sampling identity")
    v2_documents = [plan, primary, pilot, amendment, qualification, handbook, bundle, readiness, dossier, change_receipt, answer_key]
    if any(document.get("protocol_version") != "evalwitness.controlled-relation-review.v2" for document in v2_documents if "protocol_version" in document):
        raise SystemExit("relation pilot package v3 contains a cross-version artifact")
if package_format in {"evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"}:
    plan = json.loads((root / "relation-audit-plan.json").read_text(encoding="utf-8"))
    corpus_plan = json.loads((root / "controlled-corruption-plan.json").read_text(encoding="utf-8"))
    corpus_audit = json.loads((root / "controlled-corruption-audit.json").read_text(encoding="utf-8"))
    sentinel = json.loads((root / "relation-scarcity-sentinel.json").read_text(encoding="utf-8"))
    challenge = json.loads((root / "construct-firewall-challenge.json").read_text(encoding="utf-8"))
    repair = json.loads((root / "construct-repair-evidence.json").read_text(encoding="utf-8"))
    answer_key = json.loads((root / groups["qualification_keys"][0]).read_text(encoding="utf-8"))
    versioned = {
        "relation_protocol_version": "evalwitness.controlled-relation-governance.v3",
        "source_corpus_version": "evalwitness-controlled-corruption.v3",
        "source_corpus_plan_digest": corpus_plan["digest"],
        "source_corpus_audit_digest": corpus_audit["digest"],
        "source_mutation_program_digest": release["mutation_program_digest"],
        "source_construct_audit_digest": corpus_audit["digest"],
        "construct_firewall_commitment_digest": readiness["construct_firewall_commitment_digest"],
        "scarcity_sentinel_digest": sentinel["digest"],
        "scarcity_sentinel_cases": 3,
        "scarcity_sentinel_primary_overlap": 0,
        "scarcity_sentinel_pilot_overlap": 0,
        "sentinel_in_primary_estimand": False,
        "held_out_sentinel_claim_available": False,
        "construct_challenge_digest": challenge["digest"],
        "construct_repair_evidence_digest": repair["digest"],
        "construct_evidence_status": "public_safe_frozen_no_human_provider_or_population_claim",
        "inventory_policy": "evalwitness.relation-pilot-package-inventory.v1",
        "empirical_state_inheritance": "none",
        "empirical_status": "not_run",
        "plan_schema_version": "evalwitness.relation-audit-plan.v3",
        "corpus_plan_schema_version": "evalwitness.corruption-corpus-development-plan.v3",
        "corpus_audit_schema_version": "evalwitness.corruption-corpus-development-audit.v3",
        "corpus_release_schema_version": "evalwitness.corruption-corpus-release.v3",
        "primary_sample_schema_version": "evalwitness.relation-primary-sample.v3",
        "scarcity_sentinel_schema_version": "evalwitness.relation-scarcity-sentinel.v3",
        "pilot_sample_schema_version": "evalwitness.relation-pilot-sample.v3",
        "study_amendment_schema_version": "evalwitness.relation-study-amendment.v3",
        "qualification_set_schema_version": "evalwitness.relation-qualification-set.v3",
        "handbook_schema_version": "evalwitness.relation-reviewer-handbook.v3",
        "bundle_schema_version": "evalwitness.relation-review-bundle.v3",
        "readiness_schema_version": "evalwitness.relation-pilot-readiness.v3",
        "launch_dossier_schema_version": "evalwitness.relation-pilot-launch-dossier.v3",
        "primary_cases": 28,
        "primary_task_groups": 28,
        "primary_lineage_clusters": 28,
        "pilot_task_groups": 7,
        "pilot_lineage_clusters": 7,
        "pilot_primary_overlap": 0,
    }
    if any(summary.get(key) != value for key, value in versioned.items()):
        raise SystemExit("relation pilot package lost a v3 governance, natural-corpus, scarcity, falsification, or no-empirical-state identity")
    expected_digests.update({
        "scarcity_sentinel_digest": sentinel["digest"],
        "construct_challenge_digest": challenge["digest"],
        "construct_repair_evidence_digest": repair["digest"],
    })
    if release.get("plan_digest") != corpus_plan["digest"] or release.get("audit_digest") != corpus_audit["digest"] or plan.get("source_corpus_digest") != release["digest"] or plan.get("source_corpus_plan_digest") != corpus_plan["digest"] or plan.get("source_construct_audit_digest") != corpus_audit["digest"]:
        raise SystemExit("relation pilot package v3 corpus and relation governance parents disagree")
    if sentinel.get("selected_cases") != 3 or sentinel.get("primary_overlap") != 0 or sentinel.get("held_out_claim_available") or pilot.get("scarcity_sentinel_overlap") != 0 or plan.get("sentinel_in_primary_estimand"):
        raise SystemExit("relation pilot package promoted or weakened the scarcity sentinel")
    v3_documents = [plan, primary, sentinel, pilot, amendment, qualification, handbook, bundle, readiness, dossier, change_receipt, answer_key]
    if any(document.get("protocol_version") != "evalwitness.controlled-relation-governance.v3" for document in v3_documents if "protocol_version" in document):
        raise SystemExit("relation pilot package contains a cross-version v3 artifact")
    if package_format == "evalwitness.relation-pilot-package.v5":
        scarcity_required = {
            "scarcity_inspection_policy": "evalwitness.relation-scarcity-owner-inspection.v1",
            "scarcity_inspection_status": "generated_owner_only_no_decisions",
            "scarcity_materials": 3,
            "scarcity_materialization_status": "complete_replay_bound_restricted",
        }
        if any(summary.get(key) != value for key, value in scarcity_required.items()):
            raise SystemExit("relation pilot package v5 weakened its owner-only scarcity inspection boundary")
if any(summary.get(key) != value for key, value in expected_digests.items()):
    raise SystemExit("relation pilot package summary digest does not bind its contained artifact")
if summary["families"] != sorted(check["family"] for check in readiness["packet_checks"]):
    raise SystemExit("relation pilot package summary family set does not reproduce readiness")
expected_workload = {
    "required_reviewer_slots": 3,
    "primary_reviewer_slots": 2,
    "tie_break_reviewer_slots": 1,
    "qualification_cases_per_reviewer": 8,
    "required_qualification_responses": 24,
    "required_primary_judgments": 16,
    "maximum_tie_break_judgments": 8,
    "required_post_label_probes": 16,
    "maximum_total_review_actions": 64,
}
if package_format in {"evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"}:
    expected_workload.update({
        "required_primary_judgments": 14,
        "maximum_tie_break_judgments": 7,
        "required_post_label_probes": 14,
        "maximum_total_review_actions": 59,
    })
if dossier.get("reviewer_workload") != expected_workload:
    raise SystemExit("relation pilot launch dossier workload is not the governed maximum")
decision_ids = [decision["id"] for decision in dossier.get("governance_decisions", [])]
if decision_ids != [
    "authorship_and_labor_credit",
    "compensation_or_volunteer_terms",
    "consent_and_withdrawal_terms",
    "contact_and_scheduling_method",
    "human_data_retention_and_deletion",
    "reviewer_population_and_independence",
] or any(decision["status"] != "owner_decision_required" for decision in dossier["governance_decisions"]):
    raise SystemExit("relation pilot launch dossier lost an owner governance decision")
actions = dossier.get("external_actions", [])
if [action["action"] for action in actions] != ["assignment", "compensation", "contact", "packet_sharing", "publication", "recruitment", "scheduling"] or any(action["status"] != "not_authorized" for action in actions):
    raise SystemExit("relation pilot launch dossier authorized an external action")
disclosures = dossier.get("packet_disclosures", [])
if len(disclosures) != expected_cases or sorted(disclosure["family"] for disclosure in disclosures) != summary["families"] or any(disclosure["public_releasable"] or disclosure["redistribution_status"] != "restricted_reference_only" for disclosure in disclosures):
    raise SystemExit("relation pilot launch dossier weakened packet disclosure custody")
if dossier["plan_digest"] != readiness["plan_digest"] or dossier["pilot_sample_digest"] != readiness["pilot_sample_digest"] or dossier["bundle_digest"] != readiness["bundle_digest"] or dossier["readiness_digest"] != readiness["digest"] or dossier["mapping_commitment_digest"] != readiness["mapping_commitment_digest"]:
    raise SystemExit("relation pilot launch dossier does not bind readiness")
for index, case in enumerate(pilot["cases"], start=1):
    material = json.loads((root / f"materials/{index:02d}.json").read_text(encoding="utf-8"))
    packet = json.loads((root / f"packets/{index:02d}.json").read_text(encoding="utf-8"))
    if material["case_id"] != case["case_id"] or material["family"] != case["family"] or material["unit"] != case["unit"]:
        raise SystemExit("relation pilot package material order does not reproduce the governed pilot")
    if package_format == "evalwitness.relation-pilot-package.v3" and (
        material.get("schema_version") != "evalwitness.relation-case-material.v2"
        or packet.get("schema_version") != "evalwitness.relation-blind-packet.v2"
        or material.get("protocol_version") != "evalwitness.controlled-relation-review.v2"
        or packet.get("protocol_version") != "evalwitness.controlled-relation-review.v2"
    ):
        raise SystemExit("relation pilot package v3 contains a legacy material or blind packet")
    if package_format in {"evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"} and (
        material.get("schema_version") != "evalwitness.relation-case-material.v3"
        or packet.get("schema_version") != "evalwitness.relation-blind-packet.v3"
        or material.get("protocol_version") != "evalwitness.controlled-relation-governance.v3"
        or packet.get("protocol_version") != "evalwitness.controlled-relation-governance.v3"
        or material.get("source_corpus_plan_digest") != summary["source_corpus_plan_digest"]
    ):
        raise SystemExit("relation pilot package contains a legacy or corpus-plan-unbound v3 material or blind packet")
if package_format == "evalwitness.relation-pilot-package.v5":
    for index, case in enumerate(sentinel["cases"], start=1):
        material = json.loads((root / f"sentinel-materials/{index:02d}.json").read_text(encoding="utf-8"))
        if (
            material.get("schema_version") != "evalwitness.relation-case-material.v3"
            or material.get("protocol_version") != "evalwitness.controlled-relation-governance.v3"
            or material.get("case_id") != case["case_id"]
            or material.get("family") != case["family"]
            or material.get("unit") != case["unit"]
            or material.get("plan_digest") != plan["digest"]
            or material.get("source_corpus_digest") != release["digest"]
            or material.get("source_corpus_plan_digest") != corpus_plan["digest"]
            or material.get("source_construct_audit_digest") != corpus_audit["digest"]
            or material.get("source_mutation_program_digest") != release["mutation_program_digest"]
            or material.get("construct_firewall_digest") != case["construct_firewall_digest"]
            or material.get("external_action_status") != "not_authorized"
        ):
            raise SystemExit("relation pilot package v5 scarcity material does not bind its exact governed sentinel case")
mapping_by_packet = {}
for relative in groups["mappings"]:
    path = root / relative
    mapping = json.loads(path.read_text(encoding="utf-8"))
    if path.stem != mapping["digest"]:
        raise SystemExit("relation pilot package mapping filename is not content-addressed")
    mapping_by_packet[mapping["packet_id"]] = mapping
    if package_format == "evalwitness.relation-pilot-package.v3" and (
        mapping.get("schema_version") != "evalwitness.relation-private-mapping.v2"
        or mapping.get("protocol_version") != "evalwitness.controlled-relation-review.v2"
        or mapping.get("construct_firewall_digest") is None
    ):
        raise SystemExit("relation pilot package v3 contains a legacy or construct-unbound mapping")
    if package_format in {"evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"} and (
        mapping.get("schema_version") != "evalwitness.relation-private-mapping.v3"
        or mapping.get("protocol_version") != "evalwitness.controlled-relation-governance.v3"
        or mapping.get("source_corpus_plan_digest") != summary["source_corpus_plan_digest"]
        or mapping.get("construct_firewall_digest") is None
    ):
        raise SystemExit("relation pilot package contains a legacy or construct-unbound v3 mapping")
for index in range(1, expected_cases + 1):
    material = json.loads((root / f"materials/{index:02d}.json").read_text(encoding="utf-8"))
    packet = json.loads((root / f"packets/{index:02d}.json").read_text(encoding="utf-8"))
    mapping = mapping_by_packet.get(packet["packet_id"])
    if mapping is None or mapping["packet_digest"] != packet["digest"] or mapping["case_material_digest"] != material["digest"]:
        raise SystemExit("relation pilot package material, packet, and mapping custody disagree")
answer_path = root / groups["qualification_keys"][0]
answer_key = json.loads(answer_path.read_text(encoding="utf-8"))
if answer_path.stem != answer_key["digest"] or answer_key["qualification_set_digest"] != qualification["digest"]:
    raise SystemExit("relation pilot package qualification answer custody is invalid")
if package_format == "evalwitness.relation-pilot-package.v3" and answer_key.get("schema_version") != "evalwitness.relation-qualification-answer-key.v2":
    raise SystemExit("relation pilot package v3 contains a legacy qualification answer key")
if package_format in {"evalwitness.relation-pilot-package.v4", "evalwitness.relation-pilot-package.v5"} and answer_key.get("schema_version") != "evalwitness.relation-qualification-answer-key.v3":
    raise SystemExit("relation pilot package contains a legacy v3 qualification answer key")
PY

package_format=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["format_version"])' "$package_root/package-summary.json")
schema_suffix=""
if [[ "$package_format" == "evalwitness.relation-pilot-package.v3" ]]; then
  schema_suffix="-v2"
elif [[ "$package_format" == "evalwitness.relation-pilot-package.v4" || "$package_format" == "evalwitness.relation-pilot-package.v5" ]]; then
  schema_suffix="-v3"
fi
expected_cases=8
if [[ "$package_format" == "evalwitness.relation-pilot-package.v4" || "$package_format" == "evalwitness.relation-pilot-package.v5" ]]; then
  expected_cases=7
fi
has_change_atlas=false
has_change_receipt=false
if [[ -f "$package_root/owner-change-atlas.md" ]]; then
  has_change_atlas=true
fi
if [[ -f "$package_root/pilot-change-receipt.json" ]]; then
  has_change_receipt=true
fi

if [[ "$package_format" == "evalwitness.relation-pilot-package.v4" || "$package_format" == "evalwitness.relation-pilot-package.v5" ]]; then
  "$binary" mutation corpus validate-v3-audit \
    --plan @"$package_root/controlled-corruption-plan.json" \
    --audit @"$package_root/controlled-corruption-audit.json" > /dev/null
  "$binary" mutation corpus validate-v3-release \
    --plan @"$package_root/controlled-corruption-plan.json" \
    --audit @"$package_root/controlled-corruption-audit.json" \
    --release @"$package_root/controlled-corruption-release.json" > /dev/null
  audit_date=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["audited_at"])' "$package_root/controlled-corruption-audit.json")
  "$binary" mutation corpus plan-v3 > "$work_dir/controlled-corruption-plan.json"
  cmp "$work_dir/controlled-corruption-plan.json" "$package_root/controlled-corruption-plan.json" > /dev/null
  "$binary" mutation corpus audit-v3 \
    --root . --plan @"$package_root/controlled-corruption-plan.json" \
    --audited-at "$audit_date" > "$work_dir/controlled-corruption-audit.json"
  cmp "$work_dir/controlled-corruption-audit.json" "$package_root/controlled-corruption-audit.json" > /dev/null
  "$binary" mutation corpus build-v3 \
    --root . --plan @"$package_root/controlled-corruption-plan.json" \
    --audit @"$package_root/controlled-corruption-audit.json" \
    > "$work_dir/controlled-corruption-release.json"
  cmp "$work_dir/controlled-corruption-release.json" "$package_root/controlled-corruption-release.json" > /dev/null
  "$binary" relation plan-v3 \
    --corpus-plan @"$package_root/controlled-corruption-plan.json" \
    --corpus-audit @"$package_root/controlled-corruption-audit.json" \
    --release @"$package_root/controlled-corruption-release.json" \
    > "$work_dir/relation-audit-plan.json"
  cmp "$work_dir/relation-audit-plan.json" "$package_root/relation-audit-plan.json" > /dev/null
  "$binary" relation primary-sample-v3 \
    --plan @"$package_root/relation-audit-plan.json" \
    --corpus-plan @"$package_root/controlled-corruption-plan.json" \
    --corpus-audit @"$package_root/controlled-corruption-audit.json" \
    --release @"$package_root/controlled-corruption-release.json" \
    > "$work_dir/relation-primary-sample.json"
  cmp "$work_dir/relation-primary-sample.json" "$package_root/relation-primary-sample.json" > /dev/null
  "$binary" relation scarcity-sentinel-v3 \
    --plan @"$package_root/relation-audit-plan.json" \
    --primary-sample @"$package_root/relation-primary-sample.json" \
    --corpus-plan @"$package_root/controlled-corruption-plan.json" \
    --corpus-audit @"$package_root/controlled-corruption-audit.json" \
    --release @"$package_root/controlled-corruption-release.json" \
    > "$work_dir/relation-scarcity-sentinel.json"
  cmp "$work_dir/relation-scarcity-sentinel.json" "$package_root/relation-scarcity-sentinel.json" > /dev/null
  "$binary" relation pilot-sample-v3 \
    --plan @"$package_root/relation-audit-plan.json" \
    --primary-sample @"$package_root/relation-primary-sample.json" \
    --scarcity-sentinel @"$package_root/relation-scarcity-sentinel.json" \
    --corpus-plan @"$package_root/controlled-corruption-plan.json" \
    --corpus-audit @"$package_root/controlled-corruption-audit.json" \
    --release @"$package_root/controlled-corruption-release.json" \
    > "$work_dir/relation-pilot-sample.json"
  cmp "$work_dir/relation-pilot-sample.json" "$package_root/relation-pilot-sample.json" > /dev/null
  amendment_issued_at=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["issued_at"])' "$package_root/relation-study-amendment.json")
  "$binary" relation study-amendment-v3 \
    --plan @"$package_root/relation-audit-plan.json" \
    --primary-sample @"$package_root/relation-primary-sample.json" \
    --scarcity-sentinel @"$package_root/relation-scarcity-sentinel.json" \
    --pilot-sample @"$package_root/relation-pilot-sample.json" \
    --issued-at "$amendment_issued_at" > "$work_dir/relation-study-amendment.json"
  cmp "$work_dir/relation-study-amendment.json" "$package_root/relation-study-amendment.json" > /dev/null
  "$binary" mutation construct-challenge build > "$work_dir/construct-firewall-challenge.json"
  cmp "$work_dir/construct-firewall-challenge.json" "$package_root/construct-firewall-challenge.json" > /dev/null
  "$binary" mutation construct-challenge validate \
    --evidence @"$package_root/construct-firewall-challenge.json" > /dev/null
  cmp "$package_root/construct-repair-evidence.json" eval/governance/construct-repair-evidence-v1.json > /dev/null
  "$binary" artifact scan --class public --path "$package_root/construct-firewall-challenge.json" > /dev/null
  "$binary" artifact scan --class public --path "$package_root/construct-repair-evidence.json" > /dev/null
else
  "$binary" mutation corpus validate \
    --release @"$package_root/controlled-corruption-release.json" > /dev/null
fi
artifacts=(
  qualification-set:qualification-set.json
  reviewer-handbook:reviewer-handbook.json
  review-bundle:review-bundle.json
  pilot-readiness:pilot-readiness.json
)
if [[ "$package_format" != "evalwitness.relation-pilot-package.v4" && "$package_format" != "evalwitness.relation-pilot-package.v5" ]]; then
  artifacts=(
    plan:relation-audit-plan.json
    primary-sample:relation-primary-sample.json
    pilot-sample:relation-pilot-sample.json
    study-amendment:relation-study-amendment.json
    "${artifacts[@]}"
  )
fi
for artifact in "${artifacts[@]}"; do
  artifact_type=${artifact%%:*}
  artifact_path=${artifact#*:}
  "$binary" relation validate --type "$artifact_type" \
    --document @"$package_root/$artifact_path" > /dev/null
done
"$binary" relation validate --type pilot-launch-dossier \
  --document @"$package_root/pilot-launch-dossier.json" > /dev/null
if [[ "$has_change_receipt" == true ]]; then
  "$binary" relation validate --type pilot-change-receipt \
    --document @"$package_root/pilot-change-receipt.json" > /dev/null
fi
for path in "$package_root"/materials/*.json; do
  "$binary" relation validate --type case-material --document @"$path" > /dev/null
done
if [[ "$package_format" == "evalwitness.relation-pilot-package.v5" ]]; then
  for path in "$package_root"/sentinel-materials/*.json; do
    "$binary" relation validate --type case-material --document @"$path" > /dev/null
  done
fi
for path in "$package_root"/packets/*.json; do
  "$binary" relation validate --type blind-packet --document @"$path" > /dev/null
done
for path in "$package_root"/mappings/*.json; do
  "$binary" relation validate --type private-mapping --document @"$path" > /dev/null
done
for path in "$package_root"/qualification-keys/*.json; do
  "$binary" relation validate --type qualification-answer-key --document @"$path" > /dev/null
done

pilot_digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["pilot_sample_digest"])' "$package_root/package-summary.json")
bundle_created_at=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["bundle_created_at"])' "$package_root/package-summary.json")
readiness_prepared_at=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["readiness_prepared_at"])' "$package_root/package-summary.json")
dossier_prepared_at=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["dossier_prepared_at"])' "$package_root/package-summary.json")
pilot_v3_args=()
if [[ "$package_format" == "evalwitness.relation-pilot-package.v4" || "$package_format" == "evalwitness.relation-pilot-package.v5" ]]; then
  pilot_v3_args=(
    --primary-sample-v3 @"$package_root/relation-primary-sample.json"
    --scarcity-sentinel-v3 @"$package_root/relation-scarcity-sentinel.json"
  )
fi
"$binary" relation bundle \
  --plan @"$package_root/relation-audit-plan.json" \
  --sample-digest "$pilot_digest" --data-role development_pilot \
  --packets @"$package_root/blind-packets.json" \
  --mappings @"$package_root/private-mappings.json" \
  --qualification-set @"$package_root/qualification-set.json" \
  --handbook @"$package_root/reviewer-handbook.json" \
  --created-at "$bundle_created_at" > "$work_dir/review-bundle.json"
cmp "$work_dir/review-bundle.json" "$package_root/review-bundle.json" > /dev/null
"$binary" relation pilot-readiness \
  --plan @"$package_root/relation-audit-plan.json" \
  --pilot-sample @"$package_root/relation-pilot-sample.json" \
  "${pilot_v3_args[@]}" \
  --bundle @"$package_root/review-bundle.json" \
  --mappings @"$package_root/private-mappings.json" \
  --qualification-set @"$package_root/qualification-set.json" \
  --handbook @"$package_root/reviewer-handbook.json" \
  --prepared-at "$readiness_prepared_at" > "$work_dir/pilot-readiness.json"
cmp "$work_dir/pilot-readiness.json" "$package_root/pilot-readiness.json" > /dev/null
if [[ "$has_change_receipt" == true ]]; then
  "$binary" relation pilot-change-receipt \
    --readiness @"$package_root/pilot-readiness.json" \
    --bundle @"$package_root/review-bundle.json" \
    --mappings @"$package_root/private-mappings.json" \
    > "$work_dir/pilot-change-receipt.json"
  cmp "$work_dir/pilot-change-receipt.json" "$package_root/pilot-change-receipt.json" > /dev/null
  if [[ "$package_format" == "evalwitness.relation-pilot-package.v4" || "$package_format" == "evalwitness.relation-pilot-package.v5" ]]; then
    "$binary" relation schema --type "pilot-change-receipt$schema_suffix" \
      > "$work_dir/pilot-change-receipt.schema.json"
    cmp "$work_dir/pilot-change-receipt.schema.json" "$package_root/pilot-change-receipt.schema.json" > /dev/null
  fi
fi
"$binary" relation pilot-launch-dossier \
  --plan @"$package_root/relation-audit-plan.json" \
  --pilot-sample @"$package_root/relation-pilot-sample.json" \
  "${pilot_v3_args[@]}" \
  --bundle @"$package_root/review-bundle.json" \
  --mappings @"$package_root/private-mappings.json" \
  --qualification-set @"$package_root/qualification-set.json" \
  --handbook @"$package_root/reviewer-handbook.json" \
  --readiness @"$package_root/pilot-readiness.json" \
  --prepared-at "$dossier_prepared_at" > "$work_dir/pilot-launch-dossier.json"
cmp "$work_dir/pilot-launch-dossier.json" "$package_root/pilot-launch-dossier.json" > /dev/null
if [[ "$package_format" == "evalwitness.relation-pilot-package.v4" || "$package_format" == "evalwitness.relation-pilot-package.v5" ]]; then
  "$binary" relation schema --type "pilot-inspection$schema_suffix" > "$work_dir/pilot-inspection.schema.json"
  cmp "$work_dir/pilot-inspection.schema.json" "$package_root/pilot-inspection.schema.json" > /dev/null
  "$binary" relation schema --type "pilot-launch-dossier$schema_suffix" > "$work_dir/pilot-launch-dossier.schema.json"
  cmp "$work_dir/pilot-launch-dossier.schema.json" "$package_root/pilot-launch-dossier.schema.json" > /dev/null
fi
"$binary" relation render-pilot-launch-brief \
  --dossier @"$package_root/pilot-launch-dossier.json" \
  > "$work_dir/pilot-launch-brief.md"
cmp "$work_dir/pilot-launch-brief.md" "$package_root/pilot-launch-brief.md" > /dev/null
"$binary" artifact scan --class public \
  --path "$package_root/pilot-launch-brief.md" > /dev/null
if [[ "$has_change_atlas" == true ]]; then
  receipt_args=()
  if [[ "$has_change_receipt" == true ]]; then
    receipt_args=(--receipt @"$package_root/pilot-change-receipt.json")
  fi
  "$binary" relation render-pilot-change-atlas \
    "${receipt_args[@]}" \
    --readiness @"$package_root/pilot-readiness.json" \
    --bundle @"$package_root/review-bundle.json" \
    --mappings @"$package_root/private-mappings.json" \
    > "$work_dir/owner-change-atlas.md"
  cmp "$work_dir/owner-change-atlas.md" "$package_root/owner-change-atlas.md" > /dev/null
fi
"$binary" relation render-pilot-inspection \
  --readiness @"$package_root/pilot-readiness.json" \
  --bundle @"$package_root/review-bundle.json" \
  --mappings @"$package_root/private-mappings.json" \
  --handbook @"$package_root/reviewer-handbook.json" \
  > "$work_dir/owner-inspection.md"
cmp "$work_dir/owner-inspection.md" "$package_root/owner-inspection.md" > /dev/null
if [[ "$package_format" == "evalwitness.relation-pilot-package.v5" ]]; then
  scarcity_material_args=()
  sentinel_index=0
  while IFS= read -r case_id; do
    sentinel_index=$((sentinel_index + 1))
    printf -v ordinal '%02d' "$sentinel_index"
    "$binary" relation materialize-v3 \
      --root . --plan @"$package_root/relation-audit-plan.json" \
      --corpus-plan @"$package_root/controlled-corruption-plan.json" \
      --corpus-audit @"$package_root/controlled-corruption-audit.json" \
      --release @"$package_root/controlled-corruption-release.json" \
      --case-id "$case_id" > "$work_dir/sentinel-material-$ordinal.json"
    cmp "$work_dir/sentinel-material-$ordinal.json" "$package_root/sentinel-materials/$ordinal.json" > /dev/null
    scarcity_material_args+=(--material @"$package_root/sentinel-materials/$ordinal.json")
  done < <(python3 -c 'import json,sys; print("\n".join(case["case_id"] for case in json.load(open(sys.argv[1], encoding="utf-8"))["cases"]))' "$package_root/relation-scarcity-sentinel.json")
  if [[ "$sentinel_index" -ne 3 ]]; then
    echo "error: relation pilot package v5 did not reconstruct all three scarcity materials" >&2
    exit 1
  fi
  "$binary" relation render-scarcity-inspection \
    --root . \
    --plan @"$package_root/relation-audit-plan.json" \
    --primary-sample @"$package_root/relation-primary-sample.json" \
    --scarcity-sentinel @"$package_root/relation-scarcity-sentinel.json" \
    --corpus-plan @"$package_root/controlled-corruption-plan.json" \
    --corpus-audit @"$package_root/controlled-corruption-audit.json" \
    --release @"$package_root/controlled-corruption-release.json" \
    "${scarcity_material_args[@]}" \
    > "$work_dir/owner-scarcity-inspection.md"
  cmp "$work_dir/owner-scarcity-inspection.md" "$package_root/owner-scarcity-inspection.md" > /dev/null
fi

inventory_digest="none"
if [[ -f "$package_root/package-inventory.json" ]]; then
  inventory_digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["digest"])' "$package_root/package-inventory.json")
fi
echo "Relation pilot package verified: format=$package_format packets=$expected_cases families=$expected_cases change_receipt=$has_change_receipt inventory_digest=$inventory_digest owner_inspection=not_completed human_study=not_run external_action=not_authorized providers=not_invoked"
