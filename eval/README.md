# Evaluation Materials

Reference data and reproduction harness from the original LLM-as-a-Verifier
research. The production Go binary reads these files only when an eval
subcommand is invoked. Useful when you want to:

- Run paper-style Terminal-Bench and SWE-bench Verified selection with evalwitness
- Verify no-provider dataset claims for Pass@1 and oracle metrics
- Compare evalwitness output against the reference Python implementation
- Have authentic agent trajectories on hand for fixture-based testing

## Layout

| path | contents | size |
|---|---|---|
| `paper/PAPER.md` | original repo README with formula, methodology, headline numbers | small |
| `python-reference/` | original Python reference implementation (`verifier_core.py`, `run_terminal_bench.py`, `run_swe_bench.py`) | 44 KB |
| `trajectories/terminal_trajs/forge_gpt54/` | 89 Terminal-Bench-2 tasks × 5 trajectories per task (Forge + GPT-5.4 submission) | ~100 MB |
| `trajectories/swebench_verified_trajs/` | 3 agents × 500 SWE-bench Verified instances each (Claude-Opus-4.5 + 4.6, Gemini-3-Flash) | ~120 MB |
| `scripts/` | shell wrappers that drive evalwitness against the trajectories | small |
| `governance/controlled-corruption-v3-plan.json` | frozen typed-proof natural-corpus audit plan | 1.4 KB |
| `governance/controlled-corruption-v3-natural-audit.json` | complete 939-attempt applied/rejected typed-firewall ledger and exact scarcity result | 2.54 MB |
| `governance/controlled-corruption-v3-release.json` | typed 280-case core plus three-case scarcity release with attempt/firewall custody | 3.64 MB |
| `governance/agent-only-study-v1.json` | coding-agent-only formal study: 20 calibration plus 20 test cases, two independent validators, automatic tie-break custody, and zero provider/human actions | 315 KB |
| `governance/agent-only-study-schema-v1.json` | strict JSON Schema 2020-12 contract for the coding-agent-only study artifact | small |
| `governance/verification-evidence-challenge-v1.json` | nine-case direct-invocation, result-provenance, failability, and claim-specific evidence-loss challenge | small |
| `governance/verification-lineage-plan-v1.json` | sealed provider-free RQ1-RQ6 plan with role isolation, exclusive loss states, holdouts, stopping rules, uncertainty, and claim ceilings | 15 KB |
| `governance/verification-lineage-schema-inventory-v1.json` | content-addressed ten-object schema-body inventory and acyclic typed parent DAG | 9.4 KB |
| `governance/verification-lineage-source-inventory-v1.json` | pre-acquisition source-admission inventory that keeps historical goldens out of research denominators | small |
| `governance/trace-source-specifications-v1.json` | provider-free pinned registry for six trace contracts, licenses, admission boundaries, and expected capability states | small |
| `governance/synthetic-execution-witness-fixtures-v1.json` | seven real fixed-process witness controls for exits, separated streams, wrappers, masked failure, state change, and truncation | small |
| `governance/verification-lineage-golden-vectors-v1.json` | 63 provider-free native-format vectors over 21 semantic cases, including strict ambiguous-identity rejection and explicit representability boundaries | 121 KB |
| `governance/verification-lineage-adapter-conformance-v1.json` | 504 passing normative accounting, linkage, identity, command, redaction, retention, exit, and round-trip checks over the sealed native vectors | 153 KB |
| `governance/verification-lineage-parser-lock-v1.json` | pre-calibration lock over 24 parser/mapping source files, six reproduced governance artifacts, 63 vectors, and 504 passing checks | 6.4 KB |
| `governance/verification-lineage-source-readiness-audit-v1.json` | closed pre-acquisition result: 5 candidate classes, 3 development-only materializations, 0 research admissions, and no empirical denominator | small |
| `governance/verification-lineage-holdout-readiness-audit-v1.json` | deterministic format selection plus explicit invalidation of retrospective format and syntax-family transfer claims | small |
| `governance/verification-lineage-corpus-feasibility-v1.json` | unchanged 20/20 threshold decision: current generation shortfall 40, no threshold weakening, no future-impossibility claim | small |
| `governance/verification-lineage-capability-matrix-v1.json` | three native-format vectors separating 30 pinned representability states from observed development-vector fields | 53 KB |
| `governance/verification-lineage-offline-proof-v1.json` | compact five-layer accepted chain plus the same-path non-failable counterexample, bound to all readiness evidence | 3.8 KB |
| `results/verification-lineage-same-path-loss-certificate-v1.json` | public-safe exact certificate locating the first semantic loss in the same-path counterexample | 1.6 KB |
| `results/verification-lineage-offline-graph-v1.json` | canonical two-path development graph derived from the offline proof and loss certificate | 1.9 KB |
| `results/verification-lineage-offline-graph-v1.svg` | deterministic human-readable projection of the canonical graph, with the development-only boundary visible | 4.8 KB |
| `results/verification-lineage-offline-audit-v1.json` | conserved one-unit development audit underlying the accepted path | 4.0 KB |
| `results/verification-lineage-offline-bom-example-v1.json` | complete accepted verification-evidence bill of materials for the positive fixture | 4.7 KB |
| `results/verification-lineage-development-dataset-card-v1.json` | dataset card that exposes 0 empirical task groups and the exact development-only use boundary | 4.7 KB |
| `results/verification-lineage-limitations-v1.json` | seven machine-readable open evidence boundaries and their required resolutions | 2.3 KB |
| `results/verification-lineage-development-release-v1.json` | 20-file public development manifest with byte counts, SHA-256, parents, and zero-provider reproduction | 7.4 KB |
| `governance/relation-*-v3.json` | frozen plan, 28-case primary, three-case sentinel, seven-case pilot, and amendment | small |
| `results/relation-scarcity-negative-evidence.json` | canonical closed public-evidence contract for the 198/3/195 funnel, 2/1/0 roles, case/parent commitments, and claim states | 4.4 KB |
| `results/relation-scarcity-negative-evidence.md` | deterministic human-readable projection of the canonical JSON contract | 4.5 KB |
| `results/relation-owner-inspection-attestation.json` | closed public aggregate of the verified private 66-assessment owner-inspection chain and its permanent non-claims | 6.8 KB |
| `results/stress-development-case-study-v1.json` | strict machine artifact for the provider-free candidate-order negative control, full reduction chain, and two-line one-minimal witness | 81.8 KB |
| `results/stress-development-case-study-v1.md` | deterministic human projection of the validated stress case study | 2.5 KB |
| `results/stress-development-case-study-v1.svg` | deterministic paper-ready vector projection of order reversal, complete reduction tape, final witness, and claim boundary | 8.4 KB |
| `results/stress-development-challenge-v1.json` | self-contained licensed fixture bytes plus exact commitments for repository-independent mechanism verification | 6.1 KB |
| `results/stress-development-challenge-receipt-v1.json` | deterministic receipt for the complete reduction reproduction and seven fixed adversarial guards | 3.0 KB |
| `results/stress-held-out-campaign-plan-v1.json` | canonical provider-free topology lock over all ten held-out arms, exact cell sets and repetitions, with every live binding explicitly absent | 9.7 KB |
| `results/stress-held-out-campaign-plan-v1.md` | deterministic human projection of the held-out campaign topology and non-authorization boundary | 2.0 KB |
| `results/stress-held-out-readiness-refusal-v1.json` | canonical provider-free refusal over the exact held-out lock and current owner gate, with one passed, one blocked, six missing gates, and no execution permit | 3.3 KB |
| `results/stress-held-out-readiness-refusal-v1.md` | deterministic human projection of the non-authorizing held-out gate ledger | 1.8 KB |
| `governance/legacy-response-custody-v1.json` | public-safe read-only census of 17,876 schema-1 response files, exact identity gaps, and zero exact-bundle admissibility | small |

`trajectories/` is NOT tracked in git and unpacks to about 226 MB. This
repository does not claim that a current downloadable release exists. Use the
fetcher only after independently verifying an authorized release or mirror with
the expected checksum-bound archives:

```bash
eval/fetch-eval-data.sh            # verified release via gh CLI
eval/fetch-eval-data.sh <tag>      # verified specific tag
EVALWITNESS_DATA_BASE_URL=https://mirror.example/evalwitness eval/fetch-eval-data.sh
```

Maintainers pack candidate assets with `scripts/build/pack-eval-data.sh`.
Uploading them remains an explicitly authorized release action.

## Exact offline evidence reproduction

```bash
scripts/evals/reproduce-public-evidence.sh --profile full
```

The command creates an isolated empty EvalWitness home, configuration, and
response cache, proves the same loopback connection succeeds outside and fails
inside hard operating-system network denial, builds the synthetic schema-3
exact-response capsule twice, requires byte-identical archives,
verifies embedded MIT redistribution evidence, exact-replays the response,
rejects schema 1 and corruption, and runs the complete claimcheck. Its canonical
success report states `network="denied"`, `provider_calls=0`, and
`clean_clone_proof=false`. The guarded build deliberately reuses the host's
already provisioned Go module and build caches, so this is a provisioned local
mechanism proof rather than dependency-empty clean-clone evidence.

This is exact conformance of the reproduction mechanism and committed derived
evidence. It is not exact reconstruction of the eleven provider runs. The legacy
custody object records 17,876 schema-1 response files totaling 71,094,038 bytes,
but schema 1 lacks the full identities required for exact replay, so all 17,876
remain legacy-only and none is placed in a public response bundle. Any empirical
exact-response bundle must come from a new, redistribution-authorized schema-3
capture under a separately authorized study.

## Identical-response capture

`eval/governance/identical-response-capture-bai-flash-v5.jsonl` is the current
schema-3 capture: 60 records from one attested OpenAI-compatible route
(`bai/deepseek-v4-flash`), Top-20 evidence, one worker, and a locked 10-second
dispatch interval. Its capture-run attestation is `complete`, and research
admission is `admitted` with 60/60 complete research-lineage records. The route
and model are retained in the machine artifacts as provenance only. The
registered offline v5 analysis answers
`distribution_aware_vs_chosen_token` with 53 agreements, 7 disagreements, and
zero unresolved groups. Outcome sensitivity covers only 2/60 groups and does
not support a quality claim. The v5 outer capsule, sealed claim ledger, and
clean-clone proof are committed artifacts and do not authorize a signed release.
The release builder packages the verified response bundle, registered study
parents, offline analysis, outer capsule, five-claim ledger, 34-receipt challenge
pack, clean-clone report, and omission-explicit inventory.

`scripts/evals/reproduce-identical-response-v5.sh` reproduces the v5 capsule,
ledger, challenge pack, capture admission, and registered analysis from a fresh
clone with empty Go caches and hard network denial. Its canonical result is
`eval/governance/identical-response-reproduction-report-v5.json`.
`claim report` and `claim render` accept the verified base/outer capsule, ledger,
challenge pack, and reproduction report only as one all-or-none evidence family;
the resulting offline Explorer shows the 60/53/7/34 comparison, every failure
row and receipt, the evidence ceiling, and the exact reproduction command.

## Reproducing paper numbers with evalwitness

```bash
# Terminal-Bench-2 — pairwise tournament across 5 trials per task
./eval/scripts/run-terminal-bench.sh

# Exact reference pipeline (K=4, unbundled, single order)
./evalwitness eval-terminal --paper-parity --output json

# Dataset-only sanity check — no provider calls
./evalwitness eval-terminal --dry-run --output text

# SWE-bench Verified — pairwise tournament across 3 agents per instance
./evalwitness eval-swebench --dry-run --output text
./eval/scripts/run-swebench.sh --limit 3 --n-reps 1 --no-sprt

# No-provider claim gate for both eval datasets plus replay/default-route checks
scripts/tests/run-claimcheck.sh

# Reproduce the complete v3 construct audit twice without a provider
scripts/audits/run-controlled-corruption-v3.sh

# Reproduce the coding-agent-only formal study without a provider
scripts/audits/run-agent-only-study.sh

# Reproduce the 15-relation stress catalog and public one-minimal case study
scripts/audits/run-stress-lab.sh
./evalwitness stress development-case-study --repository-root . --format markdown
./evalwitness stress development-case-study --repository-root . --format svg > /tmp/evalwitness-stress-certificate.svg
./evalwitness stress validate --type development-case-study \
  --repository-root . \
  --document @eval/results/stress-development-case-study-v1.json
./evalwitness stress held-out-campaign --repository-root . --format markdown
./evalwitness stress validate --type held-out-campaign-plan \
  --repository-root . \
  --document @eval/results/stress-held-out-campaign-plan-v1.json
./evalwitness stress held-out-readiness --repository-root . --format markdown
./evalwitness stress validate --type held-out-run-readiness-refusal \
  --repository-root . \
  --document @eval/results/stress-held-out-readiness-refusal-v1.json

# Reproduce the post-inspection verification-evidence gates without a provider
./evalwitness mutation verification-evidence build-challenge
./evalwitness mutation verification-evidence validate-challenge \
  --challenge @eval/governance/verification-evidence-challenge-v1.json

# Reproduce and strictly validate the sealed verification-lineage research plan
./evalwitness trace lineage plan
./evalwitness trace lineage schema-inventory
./evalwitness trace lineage source-inventory
./evalwitness trace lineage source-specifications
./evalwitness trace lineage fixture-witnesses
./evalwitness trace lineage golden-vectors
./evalwitness trace lineage adapter-conformance
./evalwitness trace lineage parser-lock --repository-root .
./evalwitness trace lineage parser-lock-verify --repository-root . \
  --document @eval/governance/verification-lineage-parser-lock-v1.json
./evalwitness trace lineage source-readiness --repository-root .
./evalwitness trace lineage source-readiness-verify --repository-root . \
  --document @eval/governance/verification-lineage-source-readiness-audit-v1.json
./evalwitness trace lineage holdout-readiness --repository-root .
./evalwitness trace lineage holdout-readiness-verify --repository-root . \
  --document @eval/governance/verification-lineage-holdout-readiness-audit-v1.json
./evalwitness trace lineage corpus-feasibility --repository-root .
./evalwitness trace lineage corpus-feasibility-verify --repository-root . \
  --document @eval/governance/verification-lineage-corpus-feasibility-v1.json
./evalwitness trace lineage capability-matrix
./evalwitness trace lineage capability-matrix-verify \
  --document @eval/governance/verification-lineage-capability-matrix-v1.json
./evalwitness trace lineage offline-proof --repository-root .
./evalwitness trace lineage offline-proof-verify --repository-root . \
  --document @eval/governance/verification-lineage-offline-proof-v1.json
./evalwitness trace lineage loss-certificate --repository-root .
./evalwitness trace lineage loss-certificate-verify --repository-root . \
  --document @eval/results/verification-lineage-same-path-loss-certificate-v1.json
./evalwitness trace lineage lineage-graph --repository-root . --format json
./evalwitness trace lineage lineage-graph --repository-root . --format svg
./evalwitness trace lineage lineage-graph-verify --repository-root . \
  --document @eval/results/verification-lineage-offline-graph-v1.json
./evalwitness trace lineage offline-audit --repository-root .
./evalwitness trace lineage offline-audit-verify --repository-root . \
  --document @eval/results/verification-lineage-offline-audit-v1.json
./evalwitness trace lineage offline-bom --repository-root .
./evalwitness trace lineage offline-bom-verify --repository-root . \
  --document @eval/results/verification-lineage-offline-bom-example-v1.json
./evalwitness trace lineage development-dataset-card --repository-root .
./evalwitness trace lineage development-dataset-card-verify --repository-root . \
  --document @eval/results/verification-lineage-development-dataset-card-v1.json
./evalwitness trace lineage limitations --repository-root .
./evalwitness trace lineage limitations-verify --repository-root . \
  --document @eval/results/verification-lineage-limitations-v1.json
./evalwitness trace lineage development-release --repository-root .
./evalwitness trace lineage development-release-verify --repository-root . \
  --document @eval/results/verification-lineage-development-release-v1.json
./evalwitness trace lineage intake \
  --source internal/preprocess/testdata/golden/codex-rollout.jsonl
./evalwitness trace lineage schema --type plan
./evalwitness trace lineage validate --type plan \
  --document @eval/governance/verification-lineage-plan-v1.json

# Emit every closed lineage schema
for type in assessment audit bom candidate capability-vector dataset-card execution-witness plan release source; do
  ./evalwitness trace lineage schema --type "${type}"
done

# Reproduce v3 governance and the complete version-closed relation protocol without a provider
scripts/audits/run-relation-governance-v3.sh

# Reproduce the canonical public-evidence contract byte-for-byte
./evalwitness relation render-scarcity-public-brief \
  --format json \
  --plan @eval/governance/relation-audit-plan-v3.json \
  --primary-sample @eval/governance/relation-primary-sample-v3.json \
  --scarcity-sentinel @eval/governance/relation-scarcity-sentinel-v3.json \
  --corpus-plan @eval/governance/controlled-corruption-v3-plan.json \
  --corpus-audit @eval/governance/controlled-corruption-v3-natural-audit.json \
  --release @eval/governance/controlled-corruption-v3-release.json

# Validate the committed contract; use --format markdown for its human view
./evalwitness relation validate --type scarcity-public-evidence \
  --document @eval/results/relation-scarcity-negative-evidence.json

# Validate the public owner-inspection aggregate without private inputs
./evalwitness relation validate --type owner-inspection-public-attestation \
  --document @eval/results/relation-owner-inspection-attestation.json

# With the withheld package and journal, reverify the private chain and reproduce
# the public aggregate without disclosing private journal or evidence identities
./evalwitness relation render-owner-inspection-public-attestation \
  --package-root /secure/relation-pilot-package \
  --private-root /secure/relation-inspection-journal --session <digest>

# Independently reproduce the current owner-only v3 pilot package
scripts/audits/verify-relation-pilot-package.sh --package-root private/relation-pilot-v6
```

The frozen v3 construct audit reports 689 applied and 250 rejected attempts,
with 283 selected cases. Seven families satisfy the preregistered 40-case quota.
`omitted_test_evidence` yields 3/40 because only three directly linked parsed
verification invocations exist in the frozen source set. This is a corpus-
specific construct-availability result; the audit does not estimate universal
agent behavior, human construct validity, provider quality, or verifier performance.
The stress case study applies the canonical candidate-order sensitivity relation
and existing first-listed zero-cost arm to the checked-in MIT fixtures. Reversing
the candidates changes the selected trajectory, and the shared reducer removes
30 of 32 line units while preserving that violation, leaving only `5` and `-1`.
Its canonical content digest is
`b4a7adb23505a88ee0ab79c73957bce5fbd15539a0e5e687590719d3ec4ee84b`.
It declares zero empirical units, provider calls, and network requirement. It is
a deterministic mechanism demonstration, not verifier reliability, held-out
evidence, model comparison, population inference, or a global-minimum result.
The verification-lineage plan, ten-schema inventory, and parser lock are
preregistration and development-contract evidence only. They freeze the
prospective source, role, missingness, stopping, holdout, uncertainty, claim,
content identity, parent DAG, exact production source bytes, and executed
development-vector boundaries before calibration results are inspected.
Acquisition is not started, external action is not authorized, and the lineage
laboratory permits zero provider calls or agent launches.
The committed source-readiness audit is the closed result of that boundary: it
records zero research admissions and no empirical task-group denominator. It is
not a corpus audit or population estimate.
The holdout-readiness audit then proves that v1 cannot honestly report transfer:
the deterministic format choice was development-contaminated, and no syntax-
family candidate universe was sealed. The feasibility decision preserves the
resulting 40-group shortfall without weakening the frozen 20/20 threshold or
claiming that a future protocol v2 is impossible.
The loss certificate and JSON/SVG graph are development-only projections of the
offline proof. They demonstrate deterministic loss localization and exact visual
provenance; two fixtures are not an empirical survival rate or provider ranking.
The development release binds 20 public files, including the conserved positive
audit, accepted BOM, zero-empirical-unit dataset card, and seven-entry
limitations ledger. `1.0.0-development` names a reproducible method package, not
a completed research corpus or empirical release.
The committed `results/relation-scarcity-negative-evidence.json` is the closed,
content-addressed machine source for that exact boundary. The Markdown file is
derived from the same validated object. Claimcheck validates the standalone JSON,
regenerates both views, compares exact bytes, and rejects restricted-content
leakage in either file.
The descended release and relation governance freeze a 28-case core with 28
unique tasks and lineages, a disjoint seven-case development pilot, and the
three omitted-evidence cases as an exhaustive non-held-out sentinel. All remain
`not_run` and `not_authorized`. The audit also generates all 30 v3 workflow
schemas plus the separate sentinel schema, exercises governance-to-owner-
inspection and reviewer-to-terminal-ledger chains, and rejects mixed v1/v2/v3
parents. These are synthetic protocol proofs, not human results.

The local package-format-v5 owner input contains seven real v3 pilot
materials/packets/mappings under fresh distinct keys and binds the exact corpus
plan, natural audit, release, separate sentinel, challenge, and repair evidence.
It also contains three separately typed replay-bound sentinel materials and an
owner-only scarcity appendix without adding pilot packets or labels. Its
53-payload-file inventory digest is
`533deaaecd328d972cdf770073afb0f56e560d4aadea59be1e111d0782eafd80`.
The immutable input package still reports owner inspection as not completed.
Its separate package-bound agent-assisted completion descendant seals 66/66
assessments, passes all seven core packets, and records scarcity and overall
status as `revision_required`. Human study remains not run, external action
remains not authorized, and inherited empirical state remains none.
The committed `results/relation-owner-inspection-attestation.json` is the only
public projection of that private descendant. Its schema, content digest,
denominators, dimension aggregates, statuses, disclosure boundary, and ten claim
states are independently validatable. The private journal chain is reverified
during projection but intentionally cannot be reproduced from the public file.

The native Go evaluators read trajectories from `eval/trajectories/`, extract
the task/issue prompt from each trajectory, run pairwise selection for swing
tasks, and report Pass@1, oracle, verifier-selected score, and provider usage.
Every live run emits a best/expected/worst preflight and enforces hard call,
estimated-input, cost, and duration limits. The production default is bundled
adaptive K=1 with a four-call pair ceiling and 32k evidence slicing;
`--paper-parity` remains the explicit scientific K=4 path. Swing tasks share a
global provider-call limiter and reuse the evidence prepared for preflight.
Verifier evals retry one incomplete Top-20 response locally, then fail fast;
implicit mixed/judge results never continue to completion.
The Python reference scripts reproduce the original paper's reporting shape
and tally winners against the ground-truth `reward` field.

## Reference Python implementation

`python-reference/verifier_core.py` is the original logprob-extraction logic
(Gemini 2.5 Flash backed). The Python scripts are kept as the paper-reference
reproduction path and for cross-checking the Go implementation.
