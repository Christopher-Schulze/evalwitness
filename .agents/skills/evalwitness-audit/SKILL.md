---
name: evalwitness-audit
description: Use when an AI agent needs to install, configure, validate, or operate EvalWitness as an MCP/CLI evidence audit lab for coding-agent trajectories, especially before comparing trajectories, running benchmark evaluation, wiring MCP clients, or making paper-grade verifier claims.
---

# EvalWitness Audit

EvalWitness is a provider-portable, reproducible verifier audit lab for
coding-agent trajectories. Portability names versioned contracts, not universal
route support. Reproducibility names the exact evidence depth reported by the
verified capsule, never an implied paper result or independent replication.

## Operating Rule

Treat EvalWitness as a full verifier only when `./evalwitness doctor` reports a current `bounded_qualified` or `study_qualified` attestation for the exact request contract. `probe` and `doctor --live` establish only `probe_compatible`; they never qualify score extraction. If no current attestation exists, stop and report that the route is not paper-grade.

## First Checks

Run from the EvalWitness repository root:

```bash
./evalwitness doctor
./evalwitness presets
scripts/tests/run-replay-smoke.sh
scripts/tests/run-claimcheck.sh
```

Expected repository-default route when ambient canonical or legacy configuration
does not select another preset:

- `EVALWITNESS_PRESET=bai-deepseek-v4-flash`
- provider `bai`
- base URL `https://api.b.ai/v1`
- model `deepseek-v4-flash`
- key env `BAI_API_KEY`
- thinking mode `disabled`

This default is a configuration choice validated by a live `evalwitness probe`
(logprobs=true, top_logprobs=20 at probe time); it is not a capability
attestation. The historically measured route remains available as
`opencode-go-deepseek-v4-flash-0731` with `OPENCODE_GO_API_KEY`.

Do not print API keys. If a key is missing, tell the user which env var to set.

For a bounded live action, run the command once without `--authorize`. Inspect
the printed `evalwitness.live-authorization.v1` plan, especially route, request
fingerprint, calls, attempts, token ceilings, duration, concurrency, and cost.
Only then repeat the byte-identical command with
`--authorize <authorization_digest>`. Any changed input produces a new digest.

Qualify a route with the real score contract, not the metadata probe:

```bash
./evalwitness attest
./evalwitness attest --authorize <authorization_digest>
./evalwitness doctor
```

## CLI Workflows

Build and verify the provider-free reference evidence first:

```bash
work_root="$(mktemp -d)"
./evalwitness capsule build --repository-root . --destination "${work_root}/reference"
./evalwitness capsule verify \
  --source "${work_root}/reference" \
  --ledger "${work_root}/reference.claims.json" \
  --statement "${work_root}/reference.intoto.json" \
  --projection "${work_root}/reference.projection.json" \
  --autopsy "${work_root}/reference.autopsy.json"
./evalwitness claim challenge --all \
  --capsule "${work_root}/reference" \
  --ledger "${work_root}/reference.claims.json"
```

Treat the capsule, claim ledger, Statement, projection, and Claim Autopsy as one
publication group. Every destination must be new. `claim challenge` mutates only
an ephemeral copy and must report the exact intended guard for each receipt.

Verify the self-contained stress mechanism without repository fixture access:

```bash
repo_root="$(pwd)"
empty_root="$(mktemp -d)"
(cd "$empty_root" && "$repo_root/evalwitness" stress verify-development-challenge \
  --challenge "@$repo_root/eval/results/stress-development-challenge-v1.json")
./evalwitness stress verify-development-challenge-receipt \
  --challenge @eval/results/stress-development-challenge-v1.json \
  --receipt @eval/results/stress-development-challenge-receipt-v1.json
```

Require four verified embedded fixtures, the exact 53-attempt/30-accepted/two-
rejection reduction accounting, all seven guards passed, and zero empirical
units, provider calls, or network requirement. This verifies the released
EvalWitness mechanism package; it is not independent implementation replication,
held-out evidence, or verifier reliability.

Verify two trajectories:

```bash
./evalwitness verify --mode delta --task @task.txt --trajectory @a.txt --trajectory @b.txt --output json
# inspect the plan, then repeat with --authorize <authorization_digest>
```

Run pairwise selection over multiple trajectories. The mechanism is measured
only as provider-free E1 conformance (the same service behind `cli.verify`,
`mcp.pairwise`, and `bon`; see internal/mcp/smoke_test.go). Whether selection
improves task outcomes is an unrun locked empirical question: no shipped
quality claim attaches to this command.

```bash
./evalwitness verify --mode pairwise --task @task.txt --trajectory @a.txt --trajectory @b.txt --trajectory @c.txt --output text
```

Benchmark-scale live evaluation is fail-closed without an authorized,
checksum-valid `StudyRecord` whose manifest binds the dataset, split, route,
analysis, budget, and publication contract. A digest string by itself cannot
authorize a study. Dataset dry runs and exact replay remain network-free.

Current locked-study command shape:

```bash
./evalwitness eval-terminal --limit 3 --n-reps 1 --max-workers 1 --no-sprt --study-record @authorized-study-record.json --output text
./evalwitness eval-swebench --limit 3 --n-reps 1 --max-workers 1 --no-sprt --study-record @authorized-study-record.json --output text
# Inspect the emitted live-authorization plan, then repeat the unchanged command
# with --authorize <authorization_digest>.
```

If the provider returns `429 Too Many Requests`, report provider rate limiting. Do not call it an EvalWitness failure. Retry later, reduce workers/reps, or use cached/replay fixtures.

Verify a local release candidate without trusting narrative text:

```bash
candidate_parent="$(mktemp -d)"
candidate="${candidate_parent}/evalwitness-$(./evalwitness version)"
scripts/build/build-release-candidate.sh --destination "${candidate}"
"${candidate}/assets/binary/evalwitness-$(go env GOOS)-$(go env GOARCH)" \
  release verify \
  --assets "${candidate}/assets" \
  --manifest "${candidate}/release-manifest.json" \
  --sbom "${candidate}/evalwitness.spdx.json" \
  --statement "${candidate}/release.intoto.json" \
  --allow-unsigned-development
scripts/tests/run-release-roundtrip.sh --candidate "${candidate}"
```

Use `--key-file` only when the owner supplied and explicitly approved an
existing mode-`0600` Ed25519 private key. A signed candidate replaces the final
allowance with `--signature "${candidate}/signature"`. Local verification never
authorizes a tag, push, upload, registry entry, participant contact, or
announcement. The round trip uses only the manifest-verified, PAX-free canonical
USTAR source archive, manifest-bound portable source-tree provenance, and manifest-bound local Go
module proxy, starts with empty Go caches, reproduces the host binary
byte-for-byte, and reruns the capsule, claim, challenge, surface, Claim Autopsy,
and explorer evidence chain. It remains local mechanism evidence, not an
independent replication claim.

Run the dataset-only tally. If `eval/trajectories/` is missing, fetch only from
an independently verified authorized release or mirror; otherwise leave the
dataset gate explicitly skipped:

```bash
./evalwitness eval-terminal --dry-run --output text
```

Verify a real agent session transcript directly (Claude Code JSONL, Codex rollout JSONL, and OpenCode export JSON parse natively):

```bash
./evalwitness verify --mode absolute --task @task.txt --trajectory @session.jsonl
```

Best-of-N over an agent command (requires a git repo; attempts run in isolated worktrees):

```bash
./evalwitness bon -n 3 --task @task.txt -- <agent command>
```

Paper-parity and judge-mode comparison runs:

```bash
./evalwitness eval-terminal --paper-parity --output json      # frozen reference-prompt configuration
./evalwitness eval-terminal --paper-parity --judge-mode ...   # raw-text baseline for comparison
```

## MCP Rollout

Use the absolute binary path and pass the key through the MCP client environment:

```bash
/absolute/path/to/evalwitness mcp-serve
```

Tools exposed:

- `evalwitness_delta`
- `evalwitness_pairwise`
- `evalwitness_absolute`

The first live MCP call omits `authorization_digest` and returns JSON-RPC
`-32011` with the complete authorization plan. Inspect it and repeat the same
tool call with that digest. Replay and offline MCP calls need no live approval.

For Codex:

```bash
codex mcp add evalwitness -- /absolute/path/to/evalwitness mcp-serve
```

Copy-paste config assets live in `config/mcp/`:

- `codex.toml`
- `opencode.json`
- `claude-code.sh`
- `kilocode.jsonc`
- `generic-mcp.json`

For OpenCode/KiloCode/Claude Code, configure a local stdio MCP server with command `/absolute/path/to/evalwitness`, args `["mcp-serve"]`, and env containing either `EVALWITNESS_PRESET=bai-deepseek-v4-flash` plus `BAI_API_KEY`, or an explicitly selected route and its key.

## Claims Boundary

Allowed claim after a live probe passes: the exact route was `probe_compatible` at that time. Allowed claim after a current bounded attestation passes: the exact route and request contract produced strict Top-20 score evidence within the recorded freshness window. Neither result proves a provider-issued checkpoint identity.

Do not claim reproduced paper headline numbers unless a live `eval-terminal` or `eval-swebench` run actually completed and reported those metrics. The dry runs only confirm dataset Pass@1 and oracle tallies. Only `--paper-parity` runs are prompt-comparable with the reference implementation; the default pipeline (bundling, critique, SPRT) is a different configuration. Results labeled `extraction_mode: judge` or `mixed` are not full-verifier results.

No-key claim: `scripts/tests/run-replay-smoke.sh` proves the local CLI verifier path works deterministically against `scripts/tests/golden-delta-replay.jsonl`; it does not prove provider capability.

Local claim gate: `scripts/tests/run-claimcheck.sh` builds and verifies the public
reference capsule and all deterministic sidecars, runs the complete claim
challenge pack, byte-verifies six public claim surfaces, executes corruption and
schema-mismatch controls, verifies from a clean temporary directory under a hard
network deny, rejects stale identity, widened scope, missing caveats, and
unsupported novelty language across the public narrative, and then checks route
configuration, dataset tallies, replay, lineage, paired analysis, calibration,
and legacy artifact claims. It makes zero provider calls and does not establish
current live capability.

<!-- evalwitness:claim-surface:skill:begin -->
### Machine-verified claim view

This block is deterministically derived from the canonical claim ledger and verified capsule. Edit the ledger or evidence, never these rows.

| Claim | Status | Level | Exact value | Governed statement | Required caveat |
|---|---|---|---|---|---|
| CLM-001 | supported | E1 | string:evalwitness.git-visible-source-tree.v1 | The public reference capsule binds a Git-visible source-tree snapshot with the declared source-tree algorithm | Source binding proves artifact identity, not production safety or interface correctness |
| CLM-002 | supported | E1 | boolean:true | The sealed protocol audit records an offline reference-evaluator run over the bound conformance corpus | Protocol conformance does not establish empirical evaluator reliability |
| CLM-003 | supported | E1 | string:digest-bound-external-binary | The capsule digest-binds an external binary to the recorded source snapshot and build metadata | Digest binding does not prove a reproducible build or safety for untrusted use |
| CLM-004 | unsupported | E0 | string:not_run | The project is provider-agnostic or works with any OpenAI-compatible endpoint | No universal provider claim is permitted; only explicitly attested routes may be named |
| CLM-005 | unsupported | E0 | string:not_admitted | Nothing else ships this combination of reliability instrumentation | Novelty remains unconfirmed until the closest-work and release audit is rerun |
| CLM-006 | supported | E1 | number:4 | The four frozen request fixtures bind the configured route and preset metadata used by the reference package | This is configuration and request-fixture evidence, not live capability evidence |
| CLM-007 | unsupported | E2 | string:unavailable_public_source | The default route is currently full-verifier eligible | The 2026-08-06/07 route evidence is stale for a current-capability claim and must be re-attested |
| CLM-008 | supported | E1 | number:188 | The sealed offline protocol audit contains exactly 188 bound conformance-case results | Conformance-case coverage is not an empirical reproduction of the paper |
| CLM-026 | superseded | E1 | string:not_run | Passing a short capability probe qualifies a route | Short probes are diagnostics only and cannot qualify a route |
| CLM-027 | unsupported | E1 | string:unavailable_public_source | Three surveyed routes passed short probes and failed production-shaped score extraction | The public capsule lacks the historical response and attestation bytes needed to assert this survey result |
| CLM-028 | unsupported | E1 | string:not_run | Dry-run, replay, or a config catalog proves live provider capability | Only a bounded live E2 attestation can establish route capability |
| CLM-029 | unsupported | E0 | boolean:false | Current filesystem, archive, transcript, logging, and release behavior is safe for untrusted public use | The reference proves bounded artifact handling, not blanket safety for untrusted public use |
| CLM-030 | unsupported | E1 | string:not_run | Warm-cache regeneration is a complete scientific reproduction | Replay and collection clocks are separate; clean-clone reproduction remains required |
| CLM-031 | unsupported | E1 | string:not_run | The repository reproduces the paper's headline empirical results | Reference conformance and dataset tallies are not headline empirical reproduction |
| CLM-032 | unsupported | E1 | number:0 | Findings transfer across providers, gateways, model families, checkpoints, or time | Transfer requires the prespecified multi-provider and longitudinal studies |
| CLM-033 | unsupported | E0 | string:not_run | Evidence-reliance effects establish agent-step causality or model-internal explanation | Agent-step causality and model-internal explanation are permanently outside claim scope |
| CLM-034 | unsupported | E0 | string:not_authorized | Downloadable releases currently exist at the README release URL | The local repository has no publication proof; release publication requires explicit authorization |

View digest: `91da923d0cf5c71c62b2832d83cdf1c79725b8e355df475252f4d729a73449b4`

<!-- evalwitness:claim-surface:skill:end -->
