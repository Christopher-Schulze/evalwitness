# EvalWitness Specification

EvalWitness is a provider-portable, reproducible verifier audit lab for
coding-agent trajectories. Its offline-verifiable research objects are
content-addressed evidence capsules, canonical claims, executable falsifiers,
and complete verification lineage. Provider portability is a versioned contract,
not universal route capability. One optional execution subsystem exposes
pairwise, absolute, and delta logprob scoring through MCP, CLI, and Go. Single
static binary; the only implemented HTTP protocol is OpenAI-compatible Chat
Completions.

## Sections
- Goal
- Stack
- Repository Structure
- Provider Layer
- Provider Wire Formats
- Capability Probe Protocol
- Provider Conformance and Live Authorization
- Verifier Core
- Trajectory Preprocessing
- Trace Interoperability
- Verification Lineage
- Controlled Corruption Benchmark
- Metamorphic and Differential Stress Lab
- Controlled Relation Construct Audit
- Coding-Agent-Only Formal Study
- Outcome Validity and Blinded Adjudication
- Experiment Capsules and Claim Ledger
- Offline Evidence Explorer
- Modes
- Unified Verification Application Service
- Reliability Output
- MCP Server
- Streaming and Early-Stop
- CLI
- Statistical Design
- Study Governance
- Best-of-N Orchestration
- Configuration
- Canonical Request and Response Identity
- Cache
- Cost Model
- Optimizations
- Error Handling and Retry
- Default Setup
- Build and Distribute
- Test Strategy
- Determinism and Replay
- Verifier Audit Protocol
- Tool Integration Recipes
- Compatibility Contracts
- Security and Privacy
- Glossary
- Out of Scope

## Goal

Seal, verify, explain, challenge, and project the evidence behind coding-agent
verification claims without requiring a provider. Preserve source, build,
dataset, route, protocol, controlled-falsification, private/public custody, and
claim-transport lineage as typed immutable components. The same binary can score
and select trajectories with LLM logprobs over a 20-letter scale (A-T) in
pairwise, absolute, and delta modes. Provider label, OpenAI-compatible base URL,
and model are config-driven; live capability is never inferred from
configuration.

## Stack

| layer | choice |
|---|---|
| language | Go 1.27.0+ |
| http | stdlib net/http |
| json | stdlib encoding/json |
| mcp transport | JSON-RPC 2.0 over stdio (stdlib bufio + json) |
| concurrency | stdlib sync, golang.org/x/sync/errgroup |
| config | typed env compatibility resolver, minimal `.env` parser, TOML/JSON files, CLI flags |
| tests | go test stdlib |

Allowed external deps: `golang.org/x/sync` for bounded fan-out,
`golang.org/x/sys` for native no-replace directory publication,
`golang.org/x/text` for Unicode archive-path normalization, and
`github.com/pelletier/go-toml/v2` for configuration. Anything else requires
justification.

## Repository Structure

| path | purpose |
|---|---|
| cmd/evalwitness/main.go | CLI entry, subcommand router |
| internal/provider/provider.go | Provider interface, wire-format registry, Capabilities probe |
| internal/provider/request.go | canonical request envelope, portable bytes, fingerprint, route identity |
| internal/provider/response.go | immutable response evidence, checksums, replay status |
| internal/provider/openai.go | OpenAI-compatible Chat Completions wire format |
| internal/verifier/evidence.go | strict token alignment, versioned score evidence, and mode validation |
| internal/verifier/compare.go | censored-support comparison and missing-tail bounds |
| internal/verifier/policy.go | decision states, abstention reasons, and evidence strength |
| internal/verifier/score.go | legacy naked-score reader retained for inspection tests only |
| internal/verifier/prompt.go | prompt templates per mode and criterion |
| internal/verifier/criteria.go | preset criteria (terminal, swe, generic, custom) |
| internal/mode/pairwise.go | N trajectories tournament selection |
| internal/mode/absolute.go | single-trajectory absolute scoring |
| internal/mode/delta.go | A vs B verdict |
| internal/verification | canonical application plan, runtime lifecycle, batch execution, strict-selection admission, run audit, and lineage |
| internal/reliability/reliability.go | development-only calibration, discrimination, task-cluster uncertainty, and source-row schema |
| internal/stats | exact paired/binomial designs, clustered factorial simulation, Walsh-coded designs, cluster-robust estimation, power, and resource preflight |
| internal/reliance | frozen evidence-factor ontology, estimands, preregistration, audited Walsh reference adapter, exact-replay panels, registered-denominator accounting, clustered analysis, selector audit, arm comparison, one-minimal witnesses, public capsule binding, and deterministic publication projections |
| internal/study | closed study schemas, canonical identity, immutable lifecycle, dataset-bound split lineage, historical-data inventory, protocol reports, and execution binding |
| internal/lineage | verification-lineage plan, source and execution evidence, earliest-loss classification, capability vectors, BOMs, audit/release artifacts, strict codecs, canonical identity, and closed schemas |
| internal/capsule | closed component registries, canonical manifests, content-addressed store, deterministic archive, build/dataset/route/study provenance, public/private family verification, in-toto Statement, DSSE, and explicit-key Sigstore bundle verification |
| internal/release | seven-role release inventory, exact five-platform binary contract, embedded-build-info SPDX 2.3 SBOM, release in-toto Statement, explicit-key DSSE sealing, and complete offline verification |
| internal/claim | canonical ledger, exact expressions, evidence ceilings, lifecycle, registered challenge receipts, projections, tracked claim surfaces, Claim Autopsy, and atomic claim-side evidence re-verification |
| internal/explorer | verified evidence-explorer projection, typed extension registry, deterministic embedded assets, canonical report codec, and self-contained HTML renderer |
| cmd/evalwitness/capsule_command.go | public and private capsule build/verification CLI |
| cmd/evalwitness/capsule_reliance_command.go | frozen-base TASK 065 child-capsule build and full publication verification CLI |
| cmd/evalwitness/claim_command.go | claim verification, explanation, challenges, and autopsy CLI |
| cmd/evalwitness/claim_report_command.go | verified canonical evidence-explorer report CLI |
| cmd/evalwitness/claim_render_command.go | immutable self-contained evidence-explorer HTML CLI |
| cmd/evalwitness/claim_surface_command.go | deterministic public claim-surface rendering and byte verification |
| web/explorer | isolated Bun, React, Tailwind CSS, shadcn/ui, Zod, Vite, Vitest, and Playwright source/build lane; absent from runtime dependency graph |
| web/explorer/scripts/capture-assets.ts | no-overwrite deterministic Claim Autopsy desktop/mobile, stress-lab desktop, owner-inspection desktop, evidence-reliance desktop, identical-response desktop, architecture SVG, and digest-bound public presentation manifest capture |
| assets/stress-witness.png | GitHub-visible byte-exact stress-lab desktop capture; regenerated and compared by the complete Explorer gate, never a scientific source |
| scripts/tests/run-evidence-explorer.sh | complete source, report, render, public-safety, file-protocol, responsive, accessibility, and presentation-asset gate |
| scripts/demos | bounded provider-free success/challenge/verification/render proof plus no-overwrite asciicast v2 recorder over actual command output |
| cmd/evalwitness/eval_reliability.go | join benchmark rewards to pairwise or absolute reliability observations |
| internal/preprocess/model.go | canonical trajectory, event, link, accounting, and derivation contracts |
| internal/preprocess/adapter_*.go | bounded vendor and benchmark source adapters ending at the canonical model |
| internal/preprocess/slice.go | hard-budget evidence selection over canonical linked events |
| internal/preprocess/fidelity.go | offline source-conservation and token-estimate audit |
| internal/preprocess/trace_*.go | pinned trace import/export, envelope, privacy, hierarchy, and mapping-loss contracts |
| internal/mutation | controlled transformation ontology, mutators, preservation projections, formal/hermetic validation, reducers, blind packets, corpus discovery, and leakage validation |
| internal/stress | versioned metamorphic relations, construct admission, stage traces and comparisons, complete result accounting, violation-preserving counterexample provenance, v3 corpus replay and arm design, held-out campaign topology and batch commitments, once-only governance, deterministic development-case-study artifacts, and self-contained challenge/receipt verification |
| cmd/evalwitness/stress_command.go | provider-free stress catalog, corpus replay planning, held-out lock and campaign, non-authorizing readiness refusal, development case study, self-contained challenge/receipt verification, strict validation, and schema CLI |
| scripts/audits/run-stress-lab.sh | double-reproduction, repository-byte and empty-workspace challenge proof, strict challenge/receipt/campaign/preflight/permit schema, inert-SVG, provider-independence, and optional source-backed held-out-lock/campaign/readiness gate |
| eval/results/stress-development-case-study-v1.{json,md,svg} | canonical machine artifact plus deterministic Markdown and paper-ready SVG projections for the candidate-order negative control over tracked fixtures |
| eval/results/stress-development-challenge-v1.json, stress-development-challenge-receipt-v1.json | self-contained licensed-fixture challenge plus deterministic complete-reproduction and seven-guard receipt; mechanism evidence only |
| eval/results/stress-held-out-campaign-plan-v1.{json,md} | canonical machine topology lock plus deterministic Markdown projection for all ten held-out arms, exact cell sets and repetitions, with live bindings absent |
| eval/results/stress-held-out-readiness-refusal-v1.{json,md} | canonical machine refusal plus deterministic Markdown projection for the exact current held-out pre-provider gate ledger |
| cmd/evalwitness/mutation_command.go | provider-free mutation, schema, control, corpus build, and corpus validation CLI |
| internal/relation | objective-isolated controlled-relation plan, pilot/primary commitments, replay/materialization, domain-separated packet blinding, owner-only mappings, qualification/handbook/assignment custody, immutable seven-axis judgment revisions, complete batch commitments, prereveal ambiguity metrics, disagreement-only tie-break assignment, post-label condition probes, commit-before-reveal mapping/seed custody, deterministic post-reveal observation translation, study amendment, strict codecs, and closed schemas |
| cmd/evalwitness/relation_command.go | provider-free relation plan, sample, replay, materialization, packet/mapping, amendment, translation, validation, and schema CLI |
| internal/agentstudy | coding-agent-only formal study selection, source audit, independent validators, automated tie-break, strict codec, and release binding |
| cmd/evalwitness/agent_only_command.go | build, validate, and schema CLI for the coding-agent-only formal study |
| cmd/evalwitness/relation_review_command.go | owner-key qualification custody, reviewer/assignment/kit workflow, immutable pair judgments, complete batch commitment, prereveal ambiguity analysis, and disagreement-only tie-break CLI |
| internal/relation/judgment.go | objective-typed seven-axis judgment drafts, content-addressed revision chains, exact assignment binding, complete batch commitments, and strict temporal gates |
| internal/relation/ambiguity.go | mapping-free primary-commitment reproduction, per-axis prevalence/disagreement, reason divergence, Wilson intervals, exact tie-break scope, and independent slot-three assignment |
| internal/outcome | outcome/evidence ledger, natural inventory, bounded development pilot and authorization-explicit readiness, HMAC packet blinding, bounded reruns, content-addressed reviewer handbook and self-contained kits, reviewer consent/qualification, randomized assignments, label and post-label source-probe commitments, prereveal rubric-ambiguity and post-reveal semantic-blinding analyses, post-commit reveal, agreement, adjudication ledger, post-ledger source audit, revision, and preservation |
| internal/outcome/pilot_inspection.go | deterministic packet-to-source reviewability preflight, exact retention/coverage denominators, structural/semantic status separation, and readiness binding |
| cmd/evalwitness/outcome_command.go | provider-free outcome governance, pilot sampling, packet, label, qualification, agreement, resolution, validation, and preservation CLI |
| cmd/evalwitness/outcome_review_command.go | fail-closed pilot readiness, handbook, reviewer onboarding, assignment, self-contained kit generation/verification/rendering, label/probe commitment, rubric-ambiguity and semantic-blinding analyses, reveal, adjudication, and post-ledger source-audit CLI |
| internal/outcome/rubric_ambiguity.go | committed-label reproduction, five-axis disagreement fingerprint, rating prevalence, unclear/indeterminate rates, reason-code Jaccard divergence, prereveal timing, and terminal-ledger binding |
| internal/outcome/blinding_audit.go | frozen condition-universe protocol, post-label source probes, uncertainty/chance-corrected semantic-leakage analysis, and terminal-ledger binding |
| internal/outcome/source_audit.go | post-ledger performance-blind reconciliation of claimed, benchmark, independent, formal, and terminal-human outcome sources with coverage, conflict, transitions, Wilson intervals, and task-cluster agreement |
| eval/governance/outcome-reviewer-handbook-v1.json | exact public v1 reviewer policy, qualification/example binding, dataset statement, and canonical digest |
| cmd/evalwitness/trace.go | provider-free trace inspection and export CLI |
| cmd/evalwitness/bon.go | best-of-N command, two-phase authorization, selection, and output |
| cmd/evalwitness/bon_process.go | bounded process groups, output tails, environment allowlist, and transcript redaction |
| cmd/evalwitness/bon_workspace.go | source/destination snapshots, isolated Git objects, patch capture, apply, and cleanup |
| cmd/evalwitness/bon_manifest.go | immutable owner-only run and attempt artifact contract |
| cmd/evalwitness/protocol_application.go | offline protocol adapter over the verification application service |
| internal/cache/disk.go | hash-keyed JSON file cache |
| internal/cache/legacy_census.go | read-only structural legacy-cache census and public-safe inventory commitments |
| internal/replay/replay.go | exact replay and atomic fixture capture |
| internal/replay/bundle.go | capsule-native exact-response bundle policy, build, verification, and request-corpus indexing |
| internal/replay/migration.go | non-destructive legacy inspection migration |
| scripts/build/pack-response-bundle.sh | seal and no-overwrite publish redistribution-authorized exact-response capsules |
| scripts/evals/reproduce-public-evidence.sh | hard-network-denied profiles with isolated empty EvalWitness state, provisioned host Go caches, and explicit non-clean-clone evidence scope |
| scripts/evals/reproduce-identical-response-v5.sh | fresh-clone TASK 070 v5 graph reproduction with declared fixture/file proxy, empty Go caches, hard network denial, registered artifact byte comparison, outer capsule/ledger/challenge verification, and canonical report |
| internal/safety | protected-path policy, cache ownership, route namespaces, bounded reads, atomic publication |
| internal/mcp/server.go | JSON-RPC 2.0 stdio loop, lifecycle |
| internal/mcp/tools.go | tool registration and dispatch |
| protocol/model.go | public verifier-audit protocol object model and conformance vocabulary |
| protocol/canonical.go | strict interoperable canonical JSON and content digests |
| protocol/adapter.go | NDJSON process state machine and bounded external adapter host |
| protocol/schemas/*.json | JSON Schema 2020-12 protocol artifacts |
| protocol/vectors/*.json | normative positive, mutation, compatibility, and required-field vectors |
| internal/config/config.go | env+toml+flag merger, defaults |
| internal/log/log.go | stderr-only structured logger (slog) |
| docs/spec.md | this file |
| docs/documentation.md | runtime SSOT (created when first feature lands) |
| local task ledger (ignored from public tree) | execution overview |
| scripts/build/build.sh | cross-platform binary build |
| scripts/build/prepare-outcome-pilot.sh | fail-closed owner-only outcome-pilot package preparation through the production CLI; existing private key, explicit ordered timestamps, absent destination, complete component validation, mode-0700/0600 output, atomic directory publication, and no external action |
| scripts/build/prepare-relation-pilot.sh | fail-closed owner-only relation-pilot package preparation for governance v1, v2, or v3 through production CLI paths; distinct existing packet/qualification keys, exact governed regeneration, ordered timestamps, absent destination, SHA-256 inventory, version-matched prelaunch artifacts, not-launchable dossier, public-safe launch brief, change receipt, receipt-bound atlas, owner workbook, v3 sentinel materials and scarcity inspection, complete verification, mode-0700/0600 output, atomic publication, and no external action |
| scripts/audits/verify-relation-pilot-package.sh | version-aware package-format-v1/v2/v3/v4/v5 relation-pilot inventory, permission, checksum, protocol/schema, source/construct binding, bundle, readiness, launch-dossier, public-safe launch-brief reproduction and scan, change-receipt/atlas/workbook reproduction, and v5 sentinel-material replay plus scarcity-inspection reproduction gate |
| scripts/tests/run-tests.sh | default format, identity residue, independent fingerprint vectors, provider-free audit suites including the coding-agent-only formal study, vet, Staticcheck 2026.2.1, non-stress unit, replay, claimcheck, and build orchestration; stress and race execute only through explicit environment opt-ins |
| scripts/tests/run-race.sh | retained opt-in project-only race-detector gate; never invoked by the default test pipeline |
| scripts/audits/run-trace-interoperability.sh | offline pinned-fixture, privacy, hierarchy, provenance, and semantic round-trip gate |
| scripts/audits/run-controlled-corruption.sh | provider-free schema, formal-control, governed corpus, balance, review-sampling, and lineage gate |
| scripts/audits/run-controlled-corruption-v3.sh | provider-free exact v3 development plan, 939-attempt typed audit, 283-case release, seven-family core, three-case sentinel, nine-case verification-evidence challenge, and claim-boundary reproduction gate |
| scripts/audits/run-relation-governance-v3.sh | provider-free exact v3 plan/primary/sentinel/pilot/amendment double-reproduction plus all 30 workflow schemas and the sentinel schema, governance-to-owner-inspection and reviewer-to-ledger execution, deep-binding, mixed-generation, independence, design-diagnostic, tamper, and nonclaim gate |
| scripts/audits/run-agent-only-study.sh | provider-free 20/20 coding-agent-only formal study double-build, parent/source-lineage validation, independent-validator and tie-break tests, and zero provider/human action gate |
| scripts/audits/run-outcome-validity.sh | provider-free outcome plan/inventory, mutation sample, historical mixed-v1 diagnostic, six-case objective-typed outcome pilot, exact raw-run/reward materialization, sealed private custody, sealed structural-reviewability inspection, restricted readiness, 40 schemas, lexical blinding, governed handbook, three self-contained rendered reviewer kits, prereveal rubric-ambiguity, post-reveal semantic-blinding, post-ledger source reconciliation, three-reviewer qualification, commit-before-reveal adjudication, agreement, and full-corpus binding gate |
| .env | secrets, gitignored |
| .env.example | template |
| .gitignore | build output, secrets, local session exports, caches, and owner-only `/private/` study material |
| go.mod | module root: github.com/Christopher-Schulze/evalwitness |
| README.md | install + quickstart |

## Provider Layer

Single `Provider` interface, registry by wire format, runtime capability probing. `EVALWITNESS_PROVIDER` is a label used for env-key lookup, hashed route-namespace identity, and audit fields. Raw provider/model identifiers are metadata values, never path components. `EVALWITNESS_WIRE_FORMAT` is intentionally `openai` only because evalwitness requires output-token logprobs.

`EVALWITNESS_BASE_URL` configures a candidate OpenAI-compatible endpoint. It does
not establish verifier compatibility; live use still requires an exact current
route attestation.

`Provider` interface methods:
| method | signature | purpose |
|---|---|---|
| Name | () string | provider identifier |
| Score | (ctx context.Context, req RequestEnvelope) (ResponseRecord, error) | execute one canonical request and return immutable response evidence |
| Capabilities | () Capabilities | feature flags for optimizations |

`RequestEnvelope` schema 2:
| field | type | description |
|---|---|---|
| schema_version | int | canonicalization contract version |
| provider_id | string | provider implementation and routing label |
| base_url_origin | string | lower-case scheme/host with default port removed |
| endpoint_path | string | canonical Chat Completions path including base path |
| endpoint_kind | string | `openai.chat.completions` |
| requested_model | string | exact caller-visible model alias |
| thinking_mode | string | response-affecting reasoning control |
| messages | []Message | ordered role, content, optional name, and tool-call ID |
| temperature | float64 | finite non-negative value; negative zero normalized to zero |
| seed | *int | absent and explicit zero remain distinct |
| max_output_tokens | int | positive output ceiling |
| logprobs | bool | verifier evidence request |
| top_logprobs | int | exact requested distribution width |
| stop | []string | ordered stop sequence; null normalized to empty |
| score_tags | []string | ordered alignment anchors |
| response_format | string | parser-visible response contract |
| stream | bool | wire-mode choice |
| prompt_builder_version | string | semantic prompt contract |
| logit_bias | map<string,int> | canonical key-sorted token bias map |
| evidence_bindings | []EvidenceBinding | ordered source, canonical, ingestion, trace-envelope, mapping-report, and mapping-policy identities for each input slot |
| lineage | RequestLineage | provenance excluded from semantic fingerprint |

`EvidenceBinding` fields: `input_slot`, `source_digest`, `canonical_digest`, `ingestion_digest`, `trace_envelope_digest`, `mapping_report_digest`, `mapping_policy_version`. Slots are unique, every digest is SHA-256, and the complete ordered collection is part of canonical request identity.

`RequestLineage` fields: `criterion_id`, `sampling_slot`, `entrypoint`, `audit_case_id`, `source_trace_hash`, `trace_map_hash`, `mutation_id`, `study_cell_id`, `policy_hash`, and `legacy_source`. The application service derives criterion, sampling, entrypoint, ordered source/map identities, and decision-policy hash from the planned request. Validated callers may add audit-case, transformation, outcome-evidence, profile-policy, capsule, and study-cell references at run scope but cannot overwrite service-derived request or score evidence. `sampling_slot` is used beside the fingerprint while lineage remains excluded from semantic request identity.

`ResponseRecord` capture schema 3, parser contract `evalwitness.score-token-parser.v2`:
| field | type | description |
|---|---|---|
| request_fingerprint | string | SHA-256 parent request identity |
| provider_id, route_id, requested_model | string | configured route identity |
| served_model, checkpoint_assertion | string | observed identity and separate external assertion |
| provider_request_id | string | upstream correlation identifier when supplied |
| received_at, status, finish_reason | scalar | observation and completion state |
| usage | TokenUsage | input, output, cached, and reasoning counts |
| usage_observed | bool | true only when the upstream response contained a usage object; false is not evidence of zero usage |
| normalized_body | bytes | exact non-stream body or normalized ordered SSE data frames |
| response_body_digest | string | SHA-256 of normalized body |
| parsed_payload_digest | string | SHA-256 of parser-visible text, distributions, and token evidence |
| evidence_digest | string | SHA-256 over all immutable response evidence and lineage |
| parser_contract_version | string | score-token parser contract |
| replay_status, replay_reason | typed string | live, exact, legacy, miss, or rejected classification |
| capability_attestation_id | string | active route-attestation reference |
| audit_lineage | RequestLineage | copied provenance parent |
| distributions, raw_text | typed values | legacy lossy tag view and generated text; never the research score contract |
| has_logprobs, observed_top_logprobs, degenerate_logprobs | scalar | distribution evidence state |
| ordered_token_evidence | []TokenEvidence | aligned chosen token/logprob and ordered alternatives with lossless normalized logprobs |

`Capabilities`:
| field | type | description |
|---|---|---|
| Logprobs | bool | server returns logprobs |
| TopLogprobsMax | int | max top_logprobs accepted |
| LogitBias | bool | logit_bias parameter respected |
| PromptCache | bool | provider exposes cache_hit usage stat |
| Streaming | bool | SSE chunks include per-token logprobs |
| ContextLimit | int | model context window in tokens; drives Trajectory Preprocessing length-check |
| MaxConcurrent | int | suggested worker ceiling |

Provider registry: `map[string]func(cfg ProviderConfig) (Provider, error)`. Selection via `EVALWITNESS_WIRE_FORMAT`. Built-in wire format: `openai`. Built-in presets set labels and endpoints for `bai-deepseek-v4-flash`, `deepseek-v4-pro`, `deepseek-v4-flash`, `fireworks-deepseek-v4-flash-0731`, `opencode-go-deepseek-v4-flash-0731`, `openrouter-ambient-deepseek-v4-flash-0731`, and `openrouter-morph-deepseek-v4-flash-0731`.

TASK 070 route identity is bound by the admitted study artifacts and is not a
product default. Other presets remain registered as compatibility configuration
and historical provenance only. A route becomes research-usable only when a
fresh exact attestation binds its requested and served model, request contract,
and strict Top-20 evidence.

API key lookup uses the provider label: `<UPPER_PROVIDER>_API_KEY`, falling back
to `EVALWITNESS_API_KEY`. Endpoint selection is configuration only and never
qualifies the route.

Capability probing: detail in Capability Probe Protocol section. `evalwitness probe` explicitly tests and persists capabilities at `${EVALWITNESS_CACHE_DIR}/routes/route-<sha256>/capabilities.json`. A probe is necessary but cannot qualify a route for research use.

Missing or rejected logprobs never switch a verifier request to another mode. Strict verifier calls fail with typed evidence or capability errors. LLM-as-a-Judge requires `EVALWITNESS_JUDGE_MODE=true` or `--judge-mode`; it sends a distinct non-logprob request and has a distinct cache/replay identity.

## Provider Wire Formats

### OpenAI-Compatible Wire Format

Wire format `openai`. An endpoint exposing the OpenAI Chat Completions request
shape can be configured with `EVALWITNESS_BASE_URL` (default
`https://api.openai.com/v1`). Configuration proves only wire-shape intent; strict
score evidence and current route qualification decide live usability. The
provider label can be `deepseek` or any custom value; it scopes the cache and the
audit log.

Endpoint: `POST {EVALWITNESS_BASE_URL}/chat/completions`

Headers:
| header | value |
|---|---|
| Content-Type | application/json |
| Authorization | Bearer ${API_KEY} |
| User-Agent | evalwitness/{version} |

Request body fields:
| field | type | value | gating |
|---|---|---|---|
| model | string | from config | always |
| messages | array | `[{role:"user", content: prompt}]` | always |
| max_tokens | int | 4096 default; semantic output freezes after all score tags, followed by a bounded terminal-usage drain | always |
| temperature | float | 1.0 default | always |
| top_p | float | 0.95 default | always |
| logprobs | bool | true | Capabilities.Logprobs=true |
| thinking | object | {"type":"disabled"} | direct DeepSeek preset; omitted unless EVALWITNESS_THINKING_MODE enabled/disabled |
| top_logprobs | int | min(20, Capabilities.TopLogprobsMax) | Capabilities.Logprobs=true |
| logit_bias | object<tokenId,int> | {ids of A-T: 100} | Capabilities.LogitBias=true |
| provider | object | {only:["morph"], require_parameters:true, allow_fallbacks:false} | openrouter-morph preset only; provider label must bind the same upstream |
| stream | bool | true | streaming optimization on |
| stop | array<string> | ["</score_B>", "</score>", "Begin your analysis now."] inverse | always; set to closing tag of last expected score |
| user | string | omitted | not used |

Response shape (non-streaming):
| path | type | description |
|---|---|---|
| id | string | provider request id |
| choices[0].message.content | string | RawText |
| choices[0].finish_reason | string | "stop", "length", "logit_bias_invalid", etc. |
| choices[0].logprobs.content | array | per-token records, present only if logprobs=true honored |
| choices[0].logprobs.content[i].token | string | chosen token text |
| choices[0].logprobs.content[i].logprob | float | logprob of chosen |
| choices[0].logprobs.content[i].top_logprobs | array | alternatives at this position |
| choices[0].logprobs.content[i].top_logprobs[k].token | string | candidate token text |
| choices[0].logprobs.content[i].top_logprobs[k].logprob | float | natural-log probability |
| usage.prompt_tokens | int | input tokens |
| usage.completion_tokens | int | output tokens |
| usage.prompt_cache_hit_tokens | int | optional, provider-specific variant |
| usage.cache_read_input_tokens | int | optional, alternative cache-read naming used by some OpenAI-compatible gateways |

Streaming response: SSE chunks `data: {...}\n\n`. Each chunk has `choices[0].delta.content` and optionally `choices[0].logprobs.content` for the new token. Terminator: `data: [DONE]`.

### Wire-Format Quirks (runtime-detected, no provider-specific code)

| condition | handling |
|---|---|
| OpenAI-compat endpoint silently strips logprobs | probe or strict extraction detects `HasLogprobs=false`; verifier fails closed |
| OpenAI-compat endpoint rejects logprobs with 4xx | classifyError returns ClassCapabilityMissing; verifier fails closed |
| OpenAI-compat reasoning model emits `<think>...</think>` blocks | StripThinkBlocks sanitizes explicit judge extraction; strict logprob extraction remains token-aligned after the think block |
| OpenAI-compat code-fence wrapping (` ```...``` `) | StripCodeFences sanitizes explicit judge extraction; strict verifier mode never consumes the regex path |
| Endpoint clamps temperature to (0, 1] | request body sends user-supplied temperature; provider 4xx surfaces clearly |

## Capability Probe Protocol

Probe goal: determine at runtime whether `(provider, model)` returns logprobs, accepts logit_bias, exposes prompt-cache stats. Probes use a small request and persist the result.

Probe request (OpenAI-compatible):
| field | value |
|---|---|
| model | from config |
| messages | `[{role:"user", content:"Reply with exactly one capital letter from A through T. Output only the letter, nothing else."}]` |
| max_tokens | 512 |
| temperature | 0 |
| logprobs | true |
| top_logprobs | 20 |

Decision tree (in order):
1. HTTP 4xx with body referencing "logprobs" or "top_logprobs" -> `Logprobs=false, TopLogprobsMax=0`
2. HTTP 4xx with parameter complaint about another field -> retry without that field, mark unsupported in caps
3. HTTP 5xx or network error -> retry up to 3 times exponential backoff; on persistent failure mark `ProbeFailed=true`, surface error
4. HTTP 200 and `choices[0].logprobs.content[]` non-empty -> `Logprobs=true`, `TopLogprobsMax = max(len(c.top_logprobs)) over content[]`
5. HTTP 200 and `choices[0].logprobs` is null/missing -> `Logprobs=false` (silently ignored)
6. Detect `usage.prompt_cache_hit_tokens` or `usage.cache_read_input_tokens` field presence -> `PromptCache=true`

Logit-bias probe (separate, optional):
- Same prompt with `logit_bias: {token_id_for_T: 100, token_id_for_A: -100}` (token ids resolved via tokenizer if known, else skip)
- If output is forced "T" -> LogitBias=true; if output ignores bias -> LogitBias=false; if HTTP error -> LogitBias=false
- Without local tokenizer access: skip probe, set LogitBias=unknown, treat as false for safety

Persistence: `${EVALWITNESS_CACHE_DIR}/routes/route-<sha256>/capabilities.json` with schema; sibling `identity.json` binds the digest to exact provider/model values:
| field | type | description |
|---|---|---|
| schema_version | int | 1 |
| ts | int64 | unix seconds of probe |
| provider | string | identifier |
| model | string | identifier |
| logprobs | bool | |
| top_logprobs_max | int | |
| logit_bias | bool | |
| prompt_cache | bool | |
| max_concurrent | int | suggested |
| probe_response_excerpt | string | first 256 bytes of raw response, redacted |

Re-probe triggers: schema_version mismatch, `--force`, file age > 30 days, model id change.

## Provider Conformance and Live Authorization

Route state is explicit and monotonic by evidence ceiling:

| state | evidence | permitted use |
|---|---|---|
| unconfigured | incomplete route configuration | inspection only |
| configured | schema-valid route configuration | offline planning only |
| probe_compatible | weak diagnostic request accepted | diagnostics only |
| bounded_qualified | bounded real score request passed the exact strict score contract | small explicitly authorized live calls |
| study_qualified | bounded qualification plus a validated locked study manifest | authorized study cell only |
| expired | freshness, route, contract, or study identity changed | replay only until requalification |
| failed | capability or score-contract failure | negative evidence; no automatic campaign |

`probe` and `doctor --live` can produce only `probe_compatible`. `evalwitness attest` uses a real pairwise or absolute prompt with temperature greater than zero, closed score tags, requested and returned Top-k at least 20, non-degenerate logprobs, strict score evidence for every tag, valid-score mass at least 0.05, current request/parser/policy contracts, exact response integrity, build identity, and an explicit freshness window. There is no judge fallback. Missing usage remains a recorded limitation and never becomes zero usage. Requested model, observed served model, and operator checkpoint assertion are separate fields.

Capability attestation schema `evalwitness.capability-attestation.v1` binds:

| group | fields |
|---|---|
| identity | provider, route, route-config digest, base origin, endpoint, requested model, served model, checkpoint assertion and source |
| contract | contract digest, request/prompt/parser/policy versions, score alphabet/tags, Top-k, temperature, thinking, streaming request, request fingerprint |
| observation | complete score evidence, evidence strength, logprob state, Top-k, finish reason, streaming and usage observations, usage, request ID, latency, attempts, response evidence digest |
| build | commit, dirty state, binary SHA-256, Go version, target |
| lifecycle | observed/expires timestamps, state, expiration reason, study manifest digest, limitations |
| integrity | content digest and `att-<digest>` identity |

Attestations are stored by exact `route_id + contract_digest`; a foreign route or contract is a miss. A route, endpoint, requested model, thinking mode, request schema, prompt builder, parser, score policy, score alphabet, tags, Top-k, sampling contract, or freshness change cannot reuse the old qualification. Privacy-safe public derivatives use schema `evalwitness.public-attestation.v1`, preserve the evidence ceiling, limitations, signer, signature, challenge, and freshness, and omit prompts, credentials, account identifiers, request IDs, usage, and raw response content. A valid signature proves the signer observed the derivative, not provider endorsement or checkpoint identity.

Every network path requires `LiveIntent=true` and an exact two-step authorization using `evalwitness.live-authorization.v1`. The canonical plan binds entrypoint, route, representative request fingerprint, request-contract digest, retry and worker semantics, output ceiling, expected/worst logical calls, optional study-manifest digest, and hard limits for logical calls, outbound attempts, estimated input tokens, reserved output tokens, concurrency, duration, and optional cost. The first invocation returns the plan; only the identical digest authorizes execution. Replay and offline paths bypass the live boundary without acquiring network authority.

One concurrency-safe run budget owns the operation deadline and every outbound attempt. Logical calls are reserved separately from attempts. A batch cancellation is checked before dispatch, so undispatched cells reserve neither calls nor attempts. Before dispatch, an attempt reserves worst-case input, output, and optional cost plus a concurrency slot. Terminal responses reconcile reservation to observed usage only when the provider supplied usage; missing usage and failed attempts retain worst-case reservation. Actual usage above a hard limit fails the run after the response and records the overrun. Retry-After is bounded by the configured ceiling and caller cancellation or the operation deadline stops retries.

Live benchmark execution requires an authorized `evalwitness.study-record.v1`, exact current arm attestation, execution binding, declared-input verification, and live authorization. A digest string alone cannot authorize a study.

## Verifier Core

20-letter scale:
| token | value |
|---|---|
| A | 20 (best) |
| B | 19 |
| C | 18 |
| ... | ... |
| S | 2 |
| T | 1 (worst) |

Lowercase a-t aliased to same value as uppercase.

`ExtractScoreEvidence(requestedTopK, response, tag, mode) -> ScoreEvidence` consumes `ResponseRecord.OrderedTokenEvidence`, not the lossy `distributions` view. `ScoreEvidence` schema is `evalwitness.score-evidence.v1`; decision policy is `evalwitness.strict-score-policy.v1`.

| field | type | description |
|---|---|---|
| schema_version, policy_version | string | exact evidence and decision-policy contracts |
| tag | string | requested score anchor |
| extraction_mode | enum | verifier or judge; mixed is diagnostic-only and inadmissible |
| alignment_status | enum | exact, ambiguous, missing, truncated, or invalid |
| aligned_position | object or absent | token index plus token, stream, and raw byte offsets |
| requested_top_k, returned_top_k | int | requested and observed alternative widths |
| visible_probability_mass | float64 | sum over unique visible raw alternatives after logprob conversion |
| valid_score_mass | float64 | visible probability supporting canonical A-T letters before conditioning |
| unobserved_probability_mass | float64 | clamped `1-visible_probability_mass` with overflow rejection |
| score_support | []ScoreSupport | canonical letter/value/probability plus source ranks/forms |
| visible_alternatives | []VisibleAlternative | rank, token, lossless logprob, probability, chosen status, canonical mapping, duplicate provenance, diagnostic |
| conditional_expected_score | float64 or absent | expectation conditioned on visible valid score support |
| conditional_variance | float64 or absent | variance under the same conditional support |
| extracted | bool | derived success marker, never trusted without invariant validation |
| degradations | []Degradation | ordered typed limitations and failures |

Strict verifier validation requires schema/policy and tag identity, verifier mode, exact alignment, requested and returned Top-k at least 20, valid-score mass at least 0.05, finite bounded probabilities, mass conservation, canonical unique support, support-mass equality, and conditional moments re-derived from that support. Missing/degenerate logprobs, tag ambiguity, response truncation, duplicate raw alternatives, numeric corruption, and visible-mass overflow fail. Upper/lowercase forms map to one canonical letter and retain the maximum form probability without double counting. Invalid, fragment, and multi-letter alternatives remain explicit diagnostics and contribute no valid-score mass.

Judge validation requires an explicit judge-mode request, exact single-tag text extraction, and valid conditional value. It never consumes verifier cache/replay identity, never carries logprob evidence in usage, and never returns a 0.5 default on malformed output. `ExtractScore` remains a legacy inspection reader and is not called by runtime score paths.

`promptPairwise(task, traceA, traceB, criterion, groundTruthNote) -> string`:
- evaluator role line
- groundTruthNote (criterion-specific)
- task block
- trajectory A block (evidence-sliced to the configured budget)
- trajectory B block
- evaluation guideline (criterion description)
- rating scale legend
- output instruction: emit `<score_A>LETTER</score_A>` and `<score_B>LETTER</score_B>` exactly

`promptAbsolute(task, trace, criterion, groundTruthNote) -> string`:
- same shape, single trace, single tag `<score>LETTER</score>`

`Criterion`:
| field | type | description |
|---|---|---|
| ID | string | stable identifier for cache keys |
| Name | string | human-readable |
| Description | string | full guideline shown to LLM |

Built-in criteria (descriptions verbatim from eval/python-reference):
| id | name | applies to |
|---|---|---|
| specification | Specification Adherence | terminal/cli tasks (paper C1) |
| output_match | Output Match | terminal/cli tasks (paper C2) |
| error_signals | Error Signal Detection | terminal/cli tasks (paper C3) |
| root_cause | Root Cause Analysis | bugfixes (paper SWE) |
| code_review | Code Quality (reviewer) | patches (paper SWE) |
| verification | Empirical Verification | patches (paper SWE) |
| test_coverage | Test Behavior Match | code with tests |
| code_quality | Code Quality | refactors |
| generic | Generic Correctness | fallback |

Paper-parity mode: `--paper-parity` on `verify`, `eval-terminal`, `eval-swebench` reproduces the reference pipeline prompt-for-prompt: single-criterion prompts, critique and bundling disabled, single-order scoring, SPRT off, reps default 4, benchmark-level ground-truth note (`PaperTerminalCriteria` / `PaperSWECriteria`). Prompt byte-parity with verifier_core.py is asserted by internal/verifier/parity_test.go.

Custom criteria loadable via `--criteria @path/to/criteria.json` or stdin in CLI / inline objects in MCP tool input.

Verifier alignment algorithm: parse complete raw score tags; concatenate chosen-token text with slice-index spans; locate each tag and first non-whitespace score byte; map that byte to its token and byte offset; if the generated stream differs from raw text, perform monotonic raw-text token alignment; reject zero or multiple alignable complete tags. Provider-reported token positions are retained as provider evidence but never used as Go slice indexes.

Evidence computation: parse finite natural logprobs; preserve every returned alternative in rank order; collapse exact raw duplicates by maximum probability while marking a structural failure; map valid A-T forms at the aligned byte; collapse case aliases by maximum probability; sum visible and valid masses deterministically; compute conditional moments only when valid mass is positive; run strict invariant validation before cache write, replay, audit-backed result, or decision.

Score-token edge cases:
| case | handling |
|---|---|
| tokenizer splits `<score_A>` across N tokens | endsWith on accumulated text matches regardless of split |
| chosen token contains whitespace plus the score letter | aligned byte offset identifies the letter without trimming away provenance |
| LLM emits closing tag immediately | missing/invalid score token; strict rejection |
| LLM emits a multi-letter score token | preserve alternative with `multi_token_score`; strict rejection when it blocks extraction |
| same complete tag appears twice | ambiguous alignment with `duplicate_tag`; strict rejection |
| no complete tag appears | missing or truncated alignment; strict rejection |
| score letter outside A-T | preserve as `invalid_score_token`; it contributes visible mass but not valid-score mass |
| LLM wraps the complete tag in a code fence | exact token alignment still applies; no verifier text fallback |

`CompareScoreEvidence(left,right) -> evalwitness.score-evidence-comparison.v1` reports canonical support intersection/union, support Jaccard, observed probability overlap, common-support conditional total variation, visible/valid-mass movement, conditional-score movement, conditional-variance movement, per-side conservative missing-tail score bounds, and interval overlap. Missing Top-k alternatives remain unobserved and are never zero-filled.

`promptPairwise` template (paragraphs separated by `\n\n`, placeholders in `{}`):
| segment | template |
|---|---|
| header | `You are an expert evaluator of AI coding agents. You will see a task description and two agent trajectories. Your job is to evaluate them on ONE specific criterion: **{criterion.Name}**.` |
| ground truth | `{criterion.GroundTruthNote}` (segment omitted if empty) |
| task | `**Task:**\n{task}` |
| traj A | `**Trajectory A:**\n{traceA}` |
| traj B | `**Trajectory B:**\n{traceB}` |
| guideline | `**Evaluation Guideline — {criterion.Name}:**\n{criterion.Description}` |
| focus | `Score each trajectory ONLY on this specific criterion. Ignore other aspects of the trajectory that are not relevant to "{criterion.Name}".` |
| scale | `**Rating Scale:**\n{SCALE_DESCRIPTION}` |
| output | `Then output your final scores:\n<score_A>LETTER_A_TO_T</score_A>\n<score_B>LETTER_A_TO_T</score_B>` |
| close | `Begin your analysis now.` |

`promptAbsolute`: same shape, single trajectory block, single output `<score>LETTER_A_TO_T</score>`.

`SCALE_DESCRIPTION` constant (literal newlines between rows):
| line | content |
|---|---|
| 1 | Rate how likely the agent correctly solved the task on a 20-point scale using letters A through T: |
| 2 | A = clearly and completely succeeded with verified output (best) |
| 3 | B-D = succeeded with only minor issues |
| 4 | E-G = above average, mostly correct with some issues |
| 5 | H-J = uncertain, leans toward success |
| 6 | K-M = uncertain, leans toward failure |
| 7 | N-P = below average, significant issues remain |
| 8 | Q-S = failed with some partial progress |
| 9 | T = clearly and completely failed (worst) |

Built-in `groundTruthNote` per criterion id:
| id | note |
|---|---|
| specification | Re-read the task SPEC carefully. Reward exact compliance over similar-but-different solutions. Penalize "right idea, wrong place / wrong format / missing constraint". |
| output_match | Focus on TERMINAL OUTPUT or produced ARTIFACTS as ground truth. Do NOT trust the agent's self-assessment or claims of success. |
| error_signals | Scan late steps for unresolved errors, exception tracebacks, non-zero exit codes, compile/test failures. End-state errors weigh heavily. |
| root_cause | Patches must address the actual buggy code path; reject symptom-only patches, downstream catches, or special-cased example workarounds. |
| test_coverage | Compare actual test outputs against expected. Reward literal match. Penalize fake-greens or skipped tests. |
| code_quality | Evaluate readability, idiom adherence, no dead code, no obvious smell, no over-engineering. |
| generic | (empty by default) |

Order-bias mitigation applies to pairwise and delta modes. LLMs can score `(A=traj_i, B=traj_j)` differently from `(A=traj_j, B=traj_i)`.

Algorithm:
| step | action |
|---|---|
| 1 | assign the first order deterministically from `sha256(task, i, j)` so position is balanced across tasks |
| 2 | extract each criterion's expectation, variance, valid score mass, and provenance from the Top-20 distribution |
| 3 | aggregate mean difference and variance; add `EVALWITNESS_PAIR_CALIBRATION_SIGMA`; convert to normal-CDF win probability |
| 4 | stop after one order when the verdict exceeds `EVALWITNESS_PAIR_CONFIDENCE` and the margin exceeds epsilon |
| 5 | otherwise run the reverse order; if still uncertain, run at most one second sample per order |
| 6 | never dispatch more than `EVALWITNESS_MAX_PAIR_CALLS`; reject a non-bundled criterion set that cannot fit one order before dispatch |

Configuration:
| env | values | default |
|---|---|---|
| EVALWITNESS_BIAS_MITIGATION | adaptive / both / single / disabled | adaptive |
| EVALWITNESS_INCONSISTENCY_POLICY | adaptive / flag-only | flag-only |
| EVALWITNESS_MAX_PAIR_CALLS | 1-4 | 2 |
| EVALWITNESS_PAIR_CONFIDENCE | (0.5,1.0) | 0.6 |
| EVALWITNESS_PAIR_CALIBRATION_SIGMA | >0 | 0.05 |

Explicit `both`, `single`, and `disabled` modes retain fixed-rep compatibility. `both` averages both positions; single/disabled surface a startup warning.

`adaptive` requires `nReps=1`. A multi-rep adaptive configuration is rejected before preprocessing or provider dispatch; fixed multi-rep work selects `both` or `single` explicitly.

## Trajectory Preprocessing

Pipeline: bounded read -> content-based format detection -> strict typed adapter -> redaction -> event/link validation -> full canonical render and measurement -> exact-fit bypass or hard evidence selection -> accounting header -> prompt assembly.

Supported source formats:
| source_format | source contract |
|---|---|
| plain_text | arbitrary non-structured UTF-8 evidence |
| claude_code_jsonl | Claude Code message, tool, attachment, file-history, system, queue, and session-state records |
| codex_rollout_jsonl | Codex rollout items, context/state records, lifecycle events, commands, patches, media, collaboration, failures, and token usage |
| opencode_export_json | OpenCode session info, ordered messages, twelve part variants, and four tool states |
| terminal_bench_trajectory_json | complete trajectory metadata, ordered steps, all calls, and all observation results |
| swe_bench_cache_item_json | cache metadata, ordered messages, all function calls/results, and final patch |
| otlp_json | OTLP JSON traces `1.8.0` under exact schema URL `https://opentelemetry.io/schemas/1.41.0` |
| agent_trace_json | Agent Trace record `0.1.0` |

Input bounds: `MaxSourceBytes=268435456`; `MaxRecordBytes=16777216`; positive values required. JSONL is read record-by-record with byte offsets and malformed or over-limit records fail before adapter dispatch. Detection is content-based. Structured unknowns fail strict mode; compatibility mode classifies them as unsupported. No transcript field can select execution, network, environment, working directory, or output location.

Canonicalization profiles:
| profile | admission | command, result, and identity contract |
|---|---|---|
| `evalwitness.canonicalization.v1` | frozen research-artifact reproduction only through `FrozenCanonicalizationV1IngestOptions` | display-only command representation; Codex terminal streams flattened into the legacy result body; historical ambiguous native call identities remain ingestible |
| `evalwitness.canonicalization.v2` | default for every new ingestion through `DefaultIngestOptions` | exactly one argv or unsupported-shell-text surface plus canonical operand digest; observed Codex exit status and separated stdout/stderr retained; missing, duplicate, unknown, and out-of-order native call identities fail strict ingestion |

An empty profile normalizes to V2; every unknown profile fails before source reading. Stored V1 command payloads remain valid only when all V2 operand fields are absent. A partially populated V2 command is invalid.

Canonical schema `evalwitness.trajectory.v1`:
| field | type | description |
|---|---|---|
| schema_version | string | exact canonical contract |
| source_format | enum | detected adapter contract |
| source_digest | SHA-256 | immutable raw-source lineage; private unless explicitly projected |
| digest | SHA-256 | canonical events, links, source identity, and derivation |
| events | []Event | ordered typed evidence |
| links | []Link | acyclic causal and derivation graph |
| report | IngestionReport | complete source, loss, redaction, selection, and usage accounting |
| derivation | Derivation or absent | parent, relation, validator, changed events, changed fields |

Event common fields: `id`, `kind`, `order`, `source`, `source_event_id`, `timestamp`, `sensitivity`, `content_bytes`, `retained_bytes`, `estimated_tokens`, `content_digest`, optional `external_trace_context`; exactly one payload of message, tool_call, tool_result, command, output, file_change, attachment, error, metadata, reasoning, contribution, or evaluation. Source location fields: record, line, part, JSON pointer, byte start, byte end. Sensitivity: public, private, potential_secret, restricted_reasoning. Reasoning payload contains only present, omitted, and encrypted state; hidden content is never rendered or required. Contribution is attribution only. Evaluation remains distinct from contribution and cannot be exported as an OTLP span.

Link types: parent, call_result, file_change, derivation, reference. Link endpoints must exist; identities and typed links must be unique; causal and derivation links must be acyclic. Reference links retain external correlation without inventing causal parentage. A derived child retains the source digest and declares parent digest, relation, validator, unique changed event IDs, and unique `/events/{event-id}/...` field paths.

Ingestion report `evalwitness.ingestion-report.v1`:
| group | fields |
|---|---|
| conservation | source_records, accounted_records, canonical_events, original_bytes, retained_bytes |
| loss | unsupported_records, parse_errors, records, fields, dispositions, reasons |
| privacy | redacted_fields, redaction_hits, redacted_bytes, omitted-sensitive accounting |
| linkage | unpaired_tool_calls, unpaired_tool_results |
| selection | categories, truncation, per-event selection |
| usage | provider_usage observations with input, output, reasoning, cached, cache-creation, and total tokens |

Disposition values: represented, metadata_only, omitted_sensitive, unsupported, redacted, truncated, rejected. Every supported source record has exactly one record-accounting entry. Unknown records cannot disappear silently.

Evidence budget: zero means unbounded; if the complete canonical render fits, output is byte-identical and truncation is false. Otherwise direct call/result edges form indivisible selection units; deterministic event scores choose units and retained units return to source order. No whole unit enters if its rendered size exceeds the remaining hard budget. If no unit fits, one highest-ranked event may be UTF-8-safe field-truncated to fit. Output above the budget is an error. Truncation records budget, retained tokens, boundary event and field, reason, original/retained digests, per-category retention, and per-event disposition. Summarization is forbidden.

Fidelity report `evalwitness.transcript-fidelity.v1`: source/canonical identities, complete ingestion report, full estimated tokens, provider usage observations, estimate-versus-reported diagnostic, and category retention at each requested budget. Default audit budgets: 16384, 32768, 65536. Public fixtures have an exact source/trajectory digest and count manifest; private reasoning, credentials, and workstation paths are forbidden.

Redaction blocklist (regex, case-insensitive):
| pattern | replacement |
|---|---|
| `\bsk-[a-zA-Z0-9_-]{20,}` | `[REDACTED_KEY]` |
| `Bearer\s+[A-Za-z0-9._\-]{12,}` | `[REDACTED]` |
| `AKIA[0-9A-Z]{16}` | `[REDACTED_AWS]` |
| `ghp_[A-Za-z0-9]{36}` | `[REDACTED_GH_TOKEN]` |
| `xox[baprs]-[A-Za-z0-9-]{10,}` | `[REDACTED_SLACK]` |
| `(["']?password["']?\s*[:=]\s*["'])[^"']+(["'])` | `$1[REDACTED]$2` |
| `eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+` | `[REDACTED_JWT]` |

Custom redaction patterns loadable via `EVALWITNESS_REDACT_PATTERNS=path/to/patterns.json` (array of `{pattern, replacement}`).

## Trace Interoperability

| contract | value |
|---|---|
| trace envelope | `evalwitness.trace-envelope.v1` |
| mapping report | `evalwitness.trace-mapping-report.v1` |
| mapping policy | `evalwitness.trace-mapping.v1` |
| OTLP protocol | `1.8.0` JSON trace data |
| OpenTelemetry GenAI semantic conventions | `1.41.0` |
| required OTel schema URL | `https://opentelemetry.io/schemas/1.41.0` |
| Agent Trace | `0.1.0`, upstream commit `2754f077f3e50c1fb5088183f5c9362077cc8ca1` |

`TraceEnvelope` fields: `schema_version`, `mapping_policy_version`, `source`, `source_digest`, `capture_interval`, `agent`, `privacy_class`, `canonical_trajectory_digest`, `mapping_report_digest`, `digest`. Source identity fields: `format`, `schema_version`, optional `upstream_commit`, `media_type`. The envelope digest excludes only itself.

`MappingReport` fields: `schema_version`, `policy_version`, `source_records`, `source_fields`, `lossless`, `entries`, `totals`, `digest`. An entry has `source_path`, optional `target_path`, `disposition`, positive `count`, and optional `reason`. Dispositions: exact, normalized, synthesized, redacted, unsupported, ambiguous, dropped. `source_fields` is the number of semantic field observations emitted by the mapper and equals entry counts and disposition totals. `lossless=true` iff every observation is exact.

OTLP import requires the exact pinned Protobuf JSON field shape, valid 16-byte trace IDs, 8-byte span IDs, unique `(trace_id, span_id)`, positive signed-64 nanosecond timestamps, nondecreasing intervals, at most 10000 spans, at most 10000 events and links per span, at most 256 span attributes, at most 128 resource/scope/link attributes, and JSON nesting at most 64. Unknown structural fields fail. Parent and reference links resolve after all spans, independent of source or timestamp order. Missing endpoints are `ambiguous`; upstream dropped-field counters are `dropped`. Missing usage attributes create no observation; present usage is a non-negative integer and maps to typed provider usage. Content is digested unless authorized. No conversation ID is synthesized. OTLP span events are `unsupported`: OpenTelemetry GenAI `gen_ai.evaluation.result` is an OTel Event represented by Logs `LogRecord.event_name`, not a span event. OTLP Logs evaluation import/export is not implemented.

Agent Trace import is strict for the pinned record shape, UUID, RFC 3339 timestamp, portable repository-relative path, supported contributor type, nonempty ranges, valid line bounds, absolute HTTP(S) conversation links, and bounded files/ranges. Unknown standard fields and version drift fail. Export requires attribution authorization and at least one contribution. Verifier evidence may be emitted only as namespaced metadata containing canonical SHA-256 references; it never becomes an attribution or quality field.

Privacy classes: `metadata_only`, `content_authorized`, `attribution_authorized`, `content_and_attribution_authorized`. The default is metadata-only. Content and attribution authorization are independent. Export cannot widen the imported authorization.

Jaeger query/UI JSON is always rejected because it is not a stable documented interchange schema. The dedicated OpenTelemetry GenAI semantic-conventions repository is not supported until it publishes an immutable Schema URL; no moving `latest` contract is accepted. A new external version requires an exact version, upstream commit, license, synthetic derivation record, expected mapping, and semantic round-trip vectors.

## Verification Lineage

Protocol identity: `evalwitness.verification-lineage-protocol.v1`; canonical
policy: `evalwitness.verification-lineage-canonical-json.v1`; study-governance
policy: `evalwitness.study-governance.v1`; maximum document size: 16777216 bytes.
All objects use strict JSON decoding, reject unknown fields and trailing values,
and bind a lowercase SHA-256 content digest.

Static source registry:
`evalwitness.trace-source-specification-registry.v1`; locked plan digest:
`5f56a357721a4ec2b650660a4efb7b4d9b67d342ada388a5d28f6e607fd9189c`;
entries: `agent_trace_json`, `claude_code_jsonl`, `codex_rollout_jsonl`,
`opencode_export_json`, `otel_genai_development`, `otlp_json_genai`; laboratory
provider calls: 0; laboratory agent launch: false. The registry is protocol
metadata and is not an eleventh lineage object. Every immutable evidence source
binds upstream commit and SHA-256; mutable official documentation is explicitly
typed mutable and cannot claim a commit or digest. Capability representability
states are `required`, `optional`, `unsupported`, and `unspecified`.

Pre-acquisition source inventory:
`evalwitness.verification-lineage-source-inventory.v1`; state:
`pre_acquisition_candidate_inventory`; task-group counts inspected: false;
external action: `not_authorized`; provider calls: 0; agent launch: false. Each
candidate binds source class, materialization, paths and manifest digest where
present, formats, permitted roles, license, consent, privacy, redistribution,
export version, task lineage, near-duplicate, authoritative-capture, admission,
and reason states. Synthetic Agent Trace and OTLP controls are adapter-development
only. Historical vendor goldens are rejected from corpus denominators until a
TASK-069 source manifest and independent witness prove the missing boundaries.

Pre-acquisition source-readiness audit:
`evalwitness.verification-lineage-source-readiness-audit.v1`; state:
`not_runnable_no_admitted_paired_research_source`; candidate classes: 5;
materialized adapter-development candidates: 3; research-admitted,
capture-calibration, locked-test, and admitted task-group counts: 0; empirical
denominators available: false; task-group counts inspected: false; external
action: `not_authorized`; provider calls: 0; agent launches: 0. The audit binds
the sealed inventory and live parser-lock digests, reports per-format source
readiness as non-inferential, and cannot represent an empirical corpus audit or
population estimate.

Holdout-readiness audit:
`evalwitness.verification-lineage-holdout-readiness-audit.v1`; format candidate
universe sealed: true; deterministic selected format: `claude_code_jsonl`;
format status: `not_runnable_development_contamination`; syntax-family candidate
universe sealed: false; syntax status:
`not_runnable_unsealed_candidate_universe`; held-out source units, predictions,
labeled outcomes, false acceptances, false rejections, unresolved, and
unsupported counts: 0; transfer claim: false. A valid protocol v2 must isolate
the selected format from development and freeze the complete syntax-family
candidate universe before outcomes.

Corpus-feasibility decision:
`evalwitness.verification-lineage-corpus-feasibility.v1`; decision:
`not_feasible_current_generation`; required calibration/test/total task groups:
20/20/40; admitted calibration/test/total task groups: 0/0/0; shortfall: 40;
threshold weakened: false; post-outcome replacement: false; acquisition
performed: false; future impossibility: false.

Native capability matrix:
`evalwitness.verification-lineage-capability-matrix.v1`; formats:
`claude_code_jsonl`, `codex_rollout_jsonl`, `opencode_export_json`; fields per
format: 10; total rows: 30; specification representability counts:
required=4, optional=14, unsupported=1, unspecified=11; development-observation
counts: observed=26, not_observed=4. Each row binds the pinned source
specification, all 21 format-specific golden vectors, normative evidence,
mapping-diff digest, and a no-format-wide-inference note. Format-wide capability,
losslessness, empirical prevalence, and provider-quality claims are false.

Offline proof:
`evalwitness.verification-lineage-offline-proof.v1`; positive fixture:
`stdout_success`; positive terminal state: `direct_verification_invocation`;
positive survival: `[true,true,true,true,true]`; required fields: `call_id`,
`exit_status`, `stderr`, `stdout`; decisive channels: `exit_status`, `stderr`,
`stdout`; positive bindings: source, execution witness, exact native pairing,
candidate, assessment, conserved audit, accepted BOM. Counterexample:
`codex_rollout_jsonl/same_path_comparison_false_target`; resolution:
`non_failable`; reason: `self_comparison`; terminal state:
`non_failable_verification`; survival: `[true,true,true,false,false]`. Evidence
role is adapter development; empirical audit, research release, provider calls,
and agent launches are false/zero.

Development portfolio package: canonical audit contains one included Codex
development task group, one direct invocation, four 1/1/0 adjacent flows, exact
terminal conservation, and `inferential=false`; accepted BOM is
`evalwitness.verification-evidence-bom.v1` with complete five-layer retention;
dataset card is `evalwitness.verification-lineage-dataset-card.v1` with 63
golden vectors, 504 conformance checks, two development fixtures, one accepted
BOM, zero empirical task groups, and zero research-admitted sources; limitations
ledger is `evalwitness.verification-lineage-limitations.v1` with seven sorted
boundaries and zero provider calls/agent launches; development release is
`evalwitness.verification-lineage-release.v1`, version `1.0.0-development`, with
20 sorted public file bindings, `all_files_verified=true`,
`public_projection=true`, and `restricted_material_excluded=true`. The release
is a method package, not a research-corpus or empirical-results release.

Trace intake:
`evalwitness.verification-lineage-trace-intake.v1`; input: one strict local trace
source under `metadata_only`; outputs: exact source/canonical/mapping identities
and counts, pinned capability plan, development-conformance plan, missing-input
loss plan, claim boundary, and digest. Development-vector availability never
sets `input_conformance`. Without execution witness, authoritative capture
surface, repository-state identity, and source manifest: status is
`blocked_before_lineage_classification`; diagnosis is
`independent_execution_witness_required`; terminal classification and BOM are
false. Captured-command execution, provider calls, and agent launches are zero.

| object | schema | required invariant |
|---|---|---|
| VerificationLineagePlan | `evalwitness.verification-lineage-plan.v1` | exact protocol, trace-mapping and verification-evidence contracts; RQ1-RQ6; task-group unit; source classes; cluster isolation; four data roles; exclusive missingness precedence; exclusions; replacement and stopping; minimum support; uncertainty; holdouts; claim ceilings; acquisition boundary; digest |
| VerificationLineageSource | `evalwitness.verification-lineage-source.v1` | agent/runtime identity class, native format and version, capture mode, task/repository/session/lineage/near-duplicate identities, license, redistribution, privacy, redaction, raw/canonical/accounting digests, role, and digest |
| ExecutionWitness | `evalwitness.execution-witness.v1` | monotonic capture sequence, invocation identity, argv or typed unsupported shell, working-directory alias, interval, exit state, separate stdout/stderr identities, truncation, state fingerprint, capture policy, and digest |
| LineageCandidate | `evalwitness.verification-lineage-candidate.v1` | exact source/witness/native/canonical/retention/request parents, candidate call/result bindings, temporal window, task cluster, parser identity, and digest |
| LineageAssessment | `evalwitness.verification-lineage-assessment.v1` | exactly one terminal state under frozen precedence, proof references, required-field survival, semantic sufficiency, decisive channels, freshness, exclusions, and digest |
| TraceCapabilityVector | `evalwitness.trace-capability-vector.v1` | pinned specification and normative-vector evidence for representability, separate observed-capture evidence, per-field required/optional/unsupported/unspecified and observed/redacted/absent/malformed states, mapping diff, and digest |
| LineageAudit | `evalwitness.verification-lineage-audit.v1` | complete attempt ledger; considered = included + excluded task groups; excluded invalid captures never enter layer flows; adjacent-layer flows, terminal states, missingness, holdout outcomes, exact conservation, claim boundary, and digest |
| VerificationEvidenceBOM | `evalwitness.verification-evidence-bom.v1` | one claim, failable property, executable and operands, observable and failure condition, runtime/native/canonical/retained/request parents, request fingerprint, separate request-lineage digest, decisive channels, freshness interval, and digest |
| VerificationLineageRelease | `evalwitness.verification-lineage-release.v1` | plan, schemas, vectors, capability matrix, conformance, audit, BOMs, dataset card, checksums, visibility, limitations, reproduction command, and digest |
| VerificationLineageDatasetCard | `evalwitness.verification-lineage-dataset-card.v1` | source populations, roles, task and lineage clusters, formats, ecosystems, capture and license boundaries, inclusion/exclusion, missingness, limitations, claim ceilings, and digest |

| child | required parent relations |
|---|---|
| VerificationLineagePlan | none |
| VerificationLineageSource | exactly one `plan` |
| ExecutionWitness | exactly one same-task-group `source` |
| LineageCandidate | exactly one same-task-group `source`; exactly one same-task-group `witness` |
| LineageAssessment | exactly one same-task-group `candidate`; exactly one same-task-group `witness` |
| TraceCapabilityVector | exactly one `plan` |
| LineageAudit | exactly one `plan`; one or more `assessment`; one or more `capability` |
| VerificationEvidenceBOM | exactly one same-task-group `source`, `witness`, `candidate`, and `assessment`; exactly one `audit` |
| VerificationLineageDatasetCard | exactly one `plan`; one or more `audit`; one or more `capability` |
| VerificationLineageRelease | exactly one `plan`, `audit`, and `dataset_card`; one or more `bom` |

Every non-plan artifact header binds the locked plan digest, task and task-group
identity, data role, typed parent references, and its own digest. The schema
inventory binds every generated closed JSON Schema body digest, validates that
all parent schema versions exist, and requires an acyclic graph. Same-task and
same-task-group edges fail closed.

Failability resolution: exactly one of `failable`, `non_failable`, or
`unresolved`; inputs bind one semantic variant, property, executable and argv,
observable failure condition, proof records, invocation execution, and optional
claim/observed state digests. Variants: direct, comparison, checksum, test run,
pipeline, wrapper, or constant success. Closed non-failable reasons: invocation
not executed, self comparison, same-state comparison, constant success,
neutralized pipeline status, unchecked digest generation, zero tests, all tests
skipped, wrapper without inner execution, wrapper-neutralized status. Closed
unresolved reasons: invocation execution, state binding, stale state, pipeline
status, cache state, wrapper execution, or failure observable unresolved.

Retention analysis binds candidate digest, sorted required fields and decisive
channels, surviving channels, truncated required fields, canonical first-loss
records, completeness, and digest. Loss kinds: `required_field_loss` at the
first layer with absent structured presence or field; `decisive_channel_loss` at
the first layer with absent semantic sufficiency or channel. Transformation-
target binding: `evalwitness.transformation-target-binding.v1`; target and task-
group identity, canonical source-object digest, retained-object digest, required
fields, decisive channels, and digest. Accepted BOM construction requires a
complete retention analysis and exact source -> witness -> pairing -> candidate
-> assessment -> audit traversal.

Primary unit: one task-clustered claim/check attempt. Calls, result chunks,
retries, and wrapper children remain nested. Role isolation dimensions are
lineage ID, near-duplicate ID, repository ID, source-session ID, and task ID.
Roles are `adapter_development`, `capture_calibration`, `locked_test`, and
`adversarial_challenge`; no cluster dimension crosses roles.

Terminal precedence is exclusive:
`invalid_capture -> behavior_absent -> export_observability_absent ->
adapter_mapping_loss -> unsupported_shell -> ambiguous_lineage ->
non_failable_verification -> claim_specific_evidence_not_weakened ->
freshness_unresolved -> direct_verification_invocation`. The first proven state
wins. Missing export text, redaction, truncation, or adapter loss cannot establish
`behavior_absent`. Timestamp proximity cannot establish causality.

Minimum inferential support: three structurally different native trace formats,
two independent agent ecosystems, 20 task groups per inferential format, 20
capture-calibration groups, 20 locked-test groups, and a paired execution witness.
Primary task-cluster proportions use 95% Clopper-Pearson intervals. Paired
task-cluster differences use a 10000-replicate percentile bootstrap with seed
20260810. Format and syntax-family holdouts freeze selection, parser, and mapping
surfaces before result access; post-result recovery is excluded from the held-out
result.

Acquisition state starts `not_started`; source counts are
`not_inspected_before_plan_lock`; external action is `not_authorized`; laboratory
provider calls are zero; laboratory agent launch is false. Analysis commands
consume explicit local inputs and never execute a captured command. The request
fingerprint remains provider-visible body identity. Complete claim lineage uses
a separate canonical request-lineage digest and rejects lineage substitution even
when request bytes are unchanged.

Execution-witness trust boundary: `process_spawn_and_wait`; clock source:
`host_monotonic_plus_utc_observation`; working directories use repository-
relative aliases; environment capture retains allowlisted names and no values;
stdout and stderr are separated and digested before redaction or retention.
`external_observation` executes no captured command during analysis.
`synthetic_fixture_generation` executes only the seven closed local fixture
cases, accepts no arbitrary executable or shell input, permits zero provider
calls, and cannot set `authoritative_for_absence`.

| witness invariant | contract |
|---|---|
| capture interval | nonzero monotonic origin and sequence; capture-window ticks are ordered; invocation ticks remain inside the window; UTC observations support bounded cross-surface alignment only |
| behavior absence | requires `complete` capture, an authoritative policy, complete descendant-process coverage, no invocation fields, no exit observation, and absent stdout/stderr; missing native text never satisfies it |
| invocation syntax | exactly one of argv or unsupported shell text; command/operand digest is mandatory; timestamp proximity alone never creates causality |
| exit | complete invocation capture requires observed exit status; an unobserved status cannot carry a value |
| streams | `absent` has zero bytes and no digests; `captured` has equal observed/retained bytes and a full digest; `truncated` has fewer retained bytes plus full, prefix, and suffix digests |
| state | repository state presence and digest are equivalent; unknown state remains unresolved |
| threats | agent narration spoofing, clock skew, command-display spoofing, dropped child process, environment mutation, output truncation, shell-wrapper ambiguity, and state drift are mandatory ordered findings |
| policy identity | capture policy and complete witness are independently content-addressed; echoed success markers are never trusted |
| capture purpose | `external_observation` forbids laboratory execution; `synthetic_fixture_generation` requires the fixed local harness and forbids behavior-absence authority |

Canonical command binding: `display` is retained for review; exactly one of
`argv` or `unsupported_shell_text` records the native syntax surface;
`operands_digest` is the SHA-256 of canonical JSON argv or shell text before
content reduction. Array-backed Codex execution records retain argv. String-
backed Claude Code, OpenCode, benchmark, and generic Codex command arguments
remain unsupported shell text and are never tokenized by guesswork.

Synthetic witness fixture schema:
`evalwitness.synthetic-execution-witness-fixtures.v1`; harness:
`evalwitness-fixed-process-fixture-v1`; fixed cases: `controlled_failure`,
`mixed_stream_failure`, `pipeline_masked_failure`, `state_change_success`,
`stdout_success`, `truncated_output_success`, `wrapper_failure_propagation`;
arbitrary executable: false; shell: false; provider calls: 0; agent launch: false;
timeout: 5000 ms; retained stream bytes: 128; clock coordinates are deterministic
and make no wall-clock duration claim. Every case contains a valid source and
execution-witness pair plus exact before/after state digests and failure
condition.

Native-format golden fixture schema:
`evalwitness.verification-lineage-golden-vectors.v1`; formats:
`claude_code_jsonl`, `codex_rollout_jsonl`, `opencode_export_json`; semantic
cases: 21; vectors: 63; role: `adapter_development`; provider calls: 0; agent
launch: false; research-denominator use: false; format-capability inference:
false; raw-secret publication: false. Every vector binds a native-input digest,
public-safe excerpt, normative terminal state, representability state, strict
import expectation, exact observed canonical identities/counts, and an explicit
mapping gap when present. Canonical and retained trajectory identities and
call/result/link counts are separate. Required cases: direct
test/check/build/outcome probe;
printed/search names; wrapper; compound command; missing command; missing,
duplicate, and out-of-order call identity; truncation; redaction; unsupported
syntax; narrated result; same-path TASK 068 false target; retained structured
success; distinct-operand comparison; planning-prose TASK 068 false target;
native exit status. Observation never upgrades `unspecified` capability.

Adapter-conformance schema:
`evalwitness.verification-lineage-adapter-conformance.v1`; input: sealed golden
set digest; statuses: `conformant`, `known_mapping_gap`, `expected_rejection`,
`not_applicable`; checks for admitted vectors: raw-record accounting, field
accounting, stable repeated import, canonical JSON re-ingest, native call/result
linkage, command display, redaction accounting, retention boundary, exit-status
preservation, native identity integrity. A `known_mapping_gap` requires one or
more failed normative checks; `conformant` requires zero. Expected rejection
and not-applicable boundaries require their boundary check to pass. The report
is `adapter_development`, provider-free, agent-free, non-empirical, and cannot
establish format-wide capability or losslessness.

Execution-witness/native pairing version:
`evalwitness.execution-witness-native-pairing.v1`; inputs: validated source,
validated execution witness, exact native bytes, maximum clock skew in
`0..60000` ms. Pairing requires exact source parent, raw-record digest/count,
canonical trajectory digest, full ingestion-accounting digest, source format,
repository alias/state, one native call ID, one parent-linked command with equal
operand digest, one call-result-linked result, observed equal exit status,
separate equal stdout/stderr digests, and call/result timestamps inside the
bounded witness interval. Timestamp proximity alone never pairs evidence.
Substituted bytes, call ID, command, exit, stream, repository, parent, or time
window fail closed.

Freshness states are `current`, `closed`, and `unresolved`. Every state is
evaluated at an explicit time. `current` binds an observed state digest and
open-ended interval with no invalidation edge. `closed` binds exactly the first
typed state-changing edge (`file_change`, `dependency_change`,
`checkout_boundary`, or `artifact_replacement`); its observation time equals the
interval end and its before-state equals the observed state. `unresolved`
contains no state or interval claim.

An accepted `VerificationEvidenceBOM` requires current freshness and exact
source, execution-witness, candidate, assessment, and audit parent digests. Its
claim binds the failable property, executable, operands, observable failure,
call/result IDs, required fields, and decisive channels. Its retention chain
separately binds native record, canonical event, transformation target, retained
bundle, request-body fingerprint, and canonical request-lineage digest. Every
required field and decisive channel must survive all five candidate layers;
truncated required fields fail. Candidate binding rejects changed claim,
candidate identity, any layer digest, or canonical request lineage even when the
request fingerprint is unchanged.

## Controlled Corruption Benchmark

| contract | value |
|---|---|
| manifest | `evalwitness.mutation-manifest.v1` |
| witness | `evalwitness.mutation-witness.v1` |
| blind packet | `evalwitness.blind-review-packet.v1` |
| reduction | `evalwitness.mutation-reduction.v1` |
| corpus specification | `evalwitness.corruption-corpus-spec.v1` |
| corpus release | `evalwitness.corruption-corpus-release.v1` |
| historical mutation program | `evalwitness.trajectory-mutation.v1` |
| corrected mutation program | `evalwitness.trajectory-mutation.v2` |
| typed-proof mutation program | `evalwitness.trajectory-mutation.v3` |
| historical relation contract | `evalwitness.controlled-relation.v1` |
| corrected relation contract | `evalwitness.controlled-relation.v2` |
| typed-proof relation contract | `evalwitness.controlled-relation.v3` |
| historical evidence boundary | `evalwitness.evidence-boundary.v1` |
| corrected evidence boundary | `evalwitness.evidence-boundary.v2` |
| typed-proof evidence boundary | `evalwitness.evidence-boundary.v3` |
| construct firewall | `evalwitness.construct-firewall.v1` |
| typed construct firewall | `evalwitness.construct-firewall.v2` |
| shell invocation parser | `evalwitness.shell-invocation.v1` |
| presentation classifier | `evalwitness.presentation-content-kind.v1` |
| typed-proof corpus development plan | `evalwitness.corruption-corpus-development-plan.v3` |
| typed-proof natural-corpus audit | `evalwitness.corruption-corpus-development-audit.v3` |
| typed-proof corpus release | `evalwitness.corruption-corpus-release.v3` |
| v3 core/sentinel policy | `evalwitness.seven-family-core-plus-scarcity-sentinel.v1` |
| v3 pre-label relation governance | `evalwitness.controlled-relation-governance.v3` |
| v3 relation plan | `evalwitness.relation-audit-plan.v3` |
| v3 relation primary sample | `evalwitness.relation-primary-sample.v3` |
| v3 relation scarcity sentinel | `evalwitness.relation-scarcity-sentinel.v3` |
| v3 relation pilot sample | `evalwitness.relation-pilot-sample.v3` |
| v3 relation amendment | `evalwitness.relation-study-amendment.v3` |
| public scarcity evidence | `evalwitness.relation-scarcity-public-evidence.v1` |
| public scarcity brief policy | `evalwitness.relation-scarcity-public-brief.v1` |
| public owner-inspection attestation | `evalwitness.relation-owner-inspection-public-attestation.v1` |
| verification-evidence assessment | `evalwitness.verification-evidence-assessment.v1` |
| verification-evidence classifier | `evalwitness.verification-evidence-classifier.v1` |
| verification-evidence challenge | `evalwitness.verification-evidence-challenge.v1` |
| canonical policy | `evalwitness.mutation-canonical-json.v1` |
| maximum decoded document | 16777216 bytes |

`Manifest`:
| field | type | invariant |
|---|---|---|
| corpus_version, mutation_id | string | nonempty corpus identity; mutation ID is `mutation-` plus the manifest content digest |
| source | SourceRef | task, repository, source family/format/location/revision, source and trajectory digests, optional paired digests, independent source outcome |
| program | Program | frozen version, family, seed, operator, unique sorted parameters |
| intervention_class | enum | semantic_quality, presentation, evidence_availability, adversarial_claim, parser_only |
| expected_relation | enum | original_better, quality_equal, quality_equal_evidence_weaker, verified_outcome_dominates, no_control_effect, ambiguous |
| affected | AffectedSurface | unique sorted original event IDs, event-addressed field paths, and path aliases |
| validator | ValidatorSpec | ID, version, formal/hermetic/preservation kind, exact contract digest, positive timeout and output ceiling |
| outcome_proof | OutcomeProof or absent | mandatory for semantic gold degradation; independent pass-to-fail witness bound to the validator contract |
| preservation | PreservationRecord | before/after quality, evidence, and causal projections; changed/preserved groups; ambiguity reasons |
| witness | Witness | relation, proven/ambiguous/invalid label, deterministic checks, content digest |
| license, privacy | typed metadata | SPDX, source/revision, redistribution, attribution, redaction policy, explicit public-release permission |
| split_group_id | string | source-task lineage group |
| original_trajectory_digest, mutated_trajectory_digest | SHA-256 | exact parent and derived child identities |
| review | ReviewState | required flag, prespecified stratum, sealed blind-packet digest |
| construct_firewall_digest | SHA-256 or absent | mandatory for v2/v3 and forbidden for v1; binds the exact applied construct proof |
| digest | SHA-256 | canonical manifest digest with digest and mutation ID omitted |

Mutation families:
| family | class | relation | primary corpus |
|---|---|---|---|
| necessary_patch_hunk_removal | semantic_quality | original_better | no; requires executable/formal outcome proof |
| known_failing_change_reintroduction | semantic_quality | original_better | no; requires executable/formal outcome proof |
| omitted_test_evidence | evidence_availability | quality_equal_evidence_weaker | yes |
| falsified_test_evidence | adversarial_claim | verified_outcome_dominates | yes |
| command_failure_hidden | adversarial_claim | verified_outcome_dominates | no; requires independently verified exit status |
| incomplete_tool_output | evidence_availability | quality_equal_evidence_weaker | yes |
| irrelevant_verbosity | presentation | quality_equal | yes |
| neutral_formatting | presentation | quality_equal | yes |
| stable_path_aliasing | presentation | quality_equal | no when source adapters omit raw paths |
| candidate_order_reversal | presentation | quality_equal | yes |
| causally_independent_event_reorder | presentation | quality_equal | yes |
| untrusted_score_tag_injection | adversarial_claim | no_control_effect | yes, decoy control |
| ambiguous_semantic_edit | semantic_quality | ambiguous | no; mandatory adjudication control |

Mutation execution: validate canonical parent -> clone typed events -> select a
target deterministically from the frozen seed -> change only family-allowed
fields -> rebuild changed event identity and accounting -> remap report and link
references -> derive child -> recompute quality/evidence/causal projections ->
evaluate relation checks -> downgrade failed or unresolved proof to ambiguous ->
seal witness, blind packet, manifest, and minimal changed-region reduction. No
trajectory content is interpreted as a command.

v1 execution remains immutable through `Apply` and
`ApplyCandidateOrderReversal`. v2 execution uses `ApplyV2` and
`ApplyCandidateOrderReversalV2`; it MUST emit exactly one sealed construct
firewall outcome. Applied outcomes contain the mutation result and an applied
report. Ineligible transformations contain no result and one rejected report.
System, malformed-input, derivation, and sealing failures remain errors and MUST
NOT be reclassified as construct scarcity.

v3 execution uses `ApplyV3` and `ApplyCandidateOrderReversalV3`; it MUST bind
relation contract v3, evidence boundary v3, and construct firewall v2. Omitted-
evidence and neutral-formatting outcomes MUST include their family-specific typed
proof. Other families MUST NOT carry either typed proof.

`ConstructFirewallReport`:
| field | type | invariant |
|---|---|---|
| schema_version | string | `evalwitness.construct-firewall.v1` |
| canonical_policy | string | `evalwitness.mutation-canonical-json.v1` |
| program_version | string | `evalwitness.trajectory-mutation.v2` |
| family | Family | registered mutation family |
| status | applied or rejected | applied has a distinct mutated digest and all checks pass; rejected has no mutated digest and at least one failed check |
| source_trajectory_digest | SHA-256 | exact parent trajectory or ordered-pair digest |
| mutated_trajectory_digest | SHA-256 or absent | mandatory only for applied status |
| target_event_ids | sorted unique string array | changed original event IDs; empty only for pair-level candidate reversal |
| proof_event_ids | sorted unique string array | complete event lineage supporting eligibility; contains every target for trajectory-level applied reports |
| semantic_role | string or absent | closed operator role such as test, check, build, outcome_probe, assistant_prose, or transaction_independent_events |
| checks | Check array | unique machine checks with expected, observed, and passed values |
| rejection_reasons | sorted unique enum array | empty for applied; nonempty for rejected |
| digest | SHA-256 | canonical report digest with digest omitted |

Closed rejection reasons: `no_applicable_target`, `preservation_failure`,
`temporal_dependency`, `token_sequence_changed`,
`transaction_dependency`, `unnatural_formatting`, and
`unverified_evidence_role`.

`ConstructFirewallReportV2`:
| field | type | invariant |
|---|---|---|
| schema_version | string | `evalwitness.construct-firewall.v2` |
| canonical_policy | string | `evalwitness.mutation-canonical-json.v1` |
| program_version | string | `evalwitness.trajectory-mutation.v3` |
| family, status, source/mutated digests, target/proof event IDs, semantic_role, checks, rejection_reasons, digest | same closed types as ConstructFirewallReport | v3 identity and digest; applied/rejected invariants unchanged |
| invocation | InvocationProof or absent | mandatory only for `omitted_test_evidence` unless rejected because no target exists |
| presentation | PresentationProof or absent | mandatory only for `neutral_formatting` unless rejected because no target exists |

`InvocationProof`:
| field | type | invariant |
|---|---|---|
| parser_version | string | `evalwitness.shell-invocation.v1` |
| tool_call_event_id, command_event_id, evidence_event_id | string | exact linked lineage; all nonempty for applied status and present in firewall proof IDs |
| segment_index | nonnegative integer | exact parsed simple-command segment |
| executable | string | normalized invoked executable basename or closed rejection marker |
| arguments | string array | ordered parsed argv after executable |
| wrapper_chain | string array | ordered allowlisted wrappers removed before classification |
| semantic_role | test, check, build, outcome_probe, or absent | nonempty only for a direct verification invocation |
| direct_invocation | boolean | true only when executable/subcommand position matches the frozen vocabulary |
| parse_status | parsed or rejected | rejected for unsupported syntax or missing command lineage |
| decision | string | closed decisive parser/classifier result |

Shell invocation grammar: reject unterminated quoting, command substitution,
backticks, grouping, and redirection; split unquoted newline, `;`, `&&`, `||`,
and pipe operators into ordered simple-command segments; preserve quoted text as
one argument; ignore shell comments only from token boundaries; consume leading
environment assignments; unwrap only `env`, `command`, `time`, `nice`, and
`bash|sh|zsh -c|-lc` recursively to depth two. Verification roles are classified
from executable plus exact subcommand/flag position. Printed, quoted, searched,
commented, or otherwise non-invoked verification names MUST reject.

`PresentationProof`:
| field | type | invariant |
|---|---|---|
| classifier_version | string | `evalwitness.presentation-content-kind.v1` |
| event_id, message_role | string | exact source event and role provenance |
| content_kind | assistant_prose, terminal_command, terminal_transcript, code, structured_data, non_assistant_role, unknown | closed first-match classification |
| text_part_count, token_count, line_count, maximum_line_bytes | nonnegative integers | exact presentation denominators; rejected proofs may record zero text parts; applied formatting requires exactly one; line count at least one |
| decision | string | closed decisive classifier result |

Presentation classification order: non-assistant role -> missing/multiple text
parts -> code fence -> complete JSON value -> terminal prefix/transcript ->
single-line command grammar -> assistant prose. Neutral formatting requires
`message_role=assistant`, `content_kind=assistant_prose`, at least 12 tokens,
punctuation, source length above 72 bytes, exact token-sequence identity, changed
rendered bytes, and no single-token line.

`ConstructRepairEvidence`:
| field | type | invariant |
|---|---|---|
| schema_version | string | `evalwitness.construct-repair-evidence.v1` |
| canonical_policy | string | `evalwitness.mutation-canonical-json.v1` |
| corpus | ConstructRepairCorpusBinding | exact audited date, plan/audit/release/program digests, source/task/attempt/applied/rejected/selected/coverage denominators; attempts equal applied plus rejected |
| cases | ConstructRepairCase[3] | fixed ordered public synthetic regressions for omitted evidence, neutral formatting, and causal reorder |
| summary | ConstructRepairSummary | exactly three v1 acceptances and three v2 rejections |
| claim_boundary | ConstructRepairClaimBoundary | provider_calls=`not_run`; human_review=`not_run`; population_inference=`not_estimated`; exact supported and unsupported claims |
| digest | SHA-256 | canonical evidence digest with digest omitted |

Each `ConstructRepairCase` binds one source trajectory digest, failure mode,
repair invariant, complete valid v1 manifest, complete valid v2 rejected
construct-firewall report, and exact closed rejection reason. The v1 manifest
MUST use mutation/relation v1 and `label_state=proven`; the v2 report MUST use
mutation v2, the same family and source trajectory, `status=rejected`, and the
case-specific reason. Build first verifies the complete v2 corpus against its
development plan and audit. Verification rebuilds all three fixtures and the
complete evidence object from the supplied parents and requires the canonical
digest to equal
`b166f124cc7e3f31b26676bba08602fca2d29a9df95f0f80a91c421f34e137c9`.

`ConstructChallengeEvidence`:
| field | type | invariant |
|---|---|---|
| schema_version | string | `evalwitness.construct-firewall-challenge.v1` |
| canonical_policy | string | `evalwitness.mutation-canonical-json.v1` |
| programs | ConstructChallengePrograms | exact v2/v3 mutation-program digests, firewall schemas, invocation parser, and presentation classifier |
| cases | ConstructChallengeCase[14] | fixed ordered synthetic minimal pairs and controls; every case binds source digest, invariant, expected statuses/reasons, full v2 observation, and full v3 observation |
| summary | ConstructChallengeSummary | cases=14; v2 applied/rejected=11/3; v3 applied/rejected=6/8; v2 false acceptances=5; v3 repaired negatives=5; positive controls=6; shared guards=3 |
| claim_boundary | ConstructChallengeClaimBoundary | provider_calls, human_review, natural_corpus_audit=`not_run`; population_inference=`not_estimated`; exact supported and unsupported claims |
| digest | SHA-256 | canonical digest `fe8419e83a9f9bbb1deba048d11da267c87259723f06e1a7fd96f3d971a9dc75` |

Challenge categories: `v2_false_acceptance` requires v2 applied and v3 rejected;
`positive_control` requires both applied; `shared_guard` requires both rejected.
Applied observations MUST bind a valid full manifest to the exact firewall
digest. Rejected observations MUST omit the manifest. Verification rebuilds all
fixtures and requires exact evidence digest identity.

`VerificationEvidenceAssessment`:
| field | type | invariant |
|---|---|---|
| schema_version, canonical_policy, classifier_version | closed string | verification-evidence assessment v1, mutation canonical JSON v1, and verification-evidence classifier v1 |
| source_format, source_trajectory_digest, target_event_id | closed source binding | valid canonical source format, exact trajectory digest, and nonempty target event |
| status | eligible or rejected | eligible only when every check passes and rejection reasons are empty; rejected requires the exactly derived closed reasons |
| semantic_role, invocation | string and InvocationProof | exact frozen v3 direct-invocation proof and role; proof evidence event equals target event |
| result_channel, result_status, result_error | closed result metadata | exact canonical result surface; adapter `completed` is not a decisive success status |
| content_kind | closed enum | verification_execution_output, command_emitted_success_marker, mixed_agent_narration, unbound_result_text, or absent |
| content_digest, content_bytes | SHA-256 and nonnegative integer | exact target evidence text identity and byte denominator |
| provenance_bound | boolean | true only for recognized verification output or an exact literal marker emitted by the bound command after a successful failable probe |
| verification_failable | boolean | true only when the invoked probe can expose failure; same-path comparisons, assertions, checksum display without check mode, and malformed comparison shapes are false |
| remaining_decisive_channels | sorted unique string array | command exit code, explicit tool-result error, or explicit success/failure status surviving text omission |
| evidence_weakened | boolean | direct invocation, provenance, and failability are proven and no equivalent decisive channel remains |
| checks | Check[4] | exact direct-invocation, provenance, failability, and claim-specific evidence-loss checks derived from typed fields |
| rejection_reasons | sorted unique enum array | exact derivation of no_applicable_target, verification_invocation_unverified, result_provenance_unbound, non_failable_verification, and claim_specific_evidence_not_weakened |
| digest | SHA-256 | canonical assessment digest with digest omitted |

Assessment: validate canonical trajectory -> resolve nonempty target -> reuse the
frozen v3 invocation proof -> classify result provenance -> prove the probe can
fail -> enumerate surviving decisive channels -> require claim-specific evidence
loss -> derive exact reasons/checks/status -> seal and validate digest. `cmp` and
`diff` require exactly two distinct operands after conservative option parsing.
`sha256sum` and `shasum` require check mode plus an input. Recognized explicit
success/failure statuses and command exit codes remain decisive after text
omission. Mixed agent narration never satisfies provenance.

`VerificationEvidenceChallenge`:
| field | type | invariant |
|---|---|---|
| schema_version, canonical_policy, classifier_version | closed string | verification-evidence challenge v1, mutation canonical JSON v1, and verification-evidence classifier v1 |
| cases | VerificationEvidenceChallengeCase[9] | fixed order: two source-derived natural negatives, two positive controls, and five synthetic guards; each binds fixture trajectory, expected status/reasons, and complete assessment |
| summary | VerificationEvidenceChallengeSummary | cases=9; natural negatives=2; positive controls=2; synthetic guards=5; eligible=2; rejected=7; provenance failures=5; failability failures=2; evidence-loss failures=7 |
| claim_boundary | closed object | deterministic minimal-pair evidence; provider calls and human review not run; population inference not estimated; no prevalence, human-validity, provider-performance, universal-shell, v4-feasibility, or generalization claim |
| digest | SHA-256 | canonical digest `23c4d331afa7df0fb9a5cce6359ee887319a6d71e394983686c599456c529250` |

Natural-negative source bindings require exact mutation ID, trajectory digest,
and target event ID. Other categories forbid source bindings. Validation derives
all summaries, checks every complete assessment and expectation, and rebuilds
all nine fixtures byte-for-byte.

`CorpusDevelopmentPlan` v3 uses schema
`evalwitness.corruption-corpus-development-plan.v3`, corpus version
`evalwitness-controlled-corruption.v3`, frozen seed
`evalwitness-controlled-corruption-v3-frozen-seed`, and mutation-program digest
`f37ec74f8096a23c5fb0e6696d279f2689f98d7e904fcef038464ca51720b3f8`.
Its source design, eight attempted primary families, 40-case family quota, exact
binomial design, and design-evidence digest remain fixed before the v3 audit.

`CorpusDevelopmentAuditV3`:
| field | type | invariant |
|---|---|---|
| schema_version | string | `evalwitness.corruption-corpus-development-audit.v3` |
| canonical_policy | string | `evalwitness.mutation-canonical-json.v1` |
| audited_at, plan_digest, corpus_version, mutation_program_digest | closed identity | date `2026-08-10`; exact v3 plan, corpus, and typed-proof program |
| source_set_digest, attempt_set_digest, firewall_set_digest, selected_case_set_digest | SHA-256 | independently reproduced canonical set commitments |
| sources | CorpusSource[200] | 100 frozen task groups, two trajectories per task, lineage-preserving 60/20/20 role split |
| attempts | ConstructAttempt[939] | complete deterministic family/source attempt universe; every attempt binds exactly one firewall |
| applied_firewalls, rejected_firewalls | ConstructFirewallReportV2 arrays | 689 applied and 250 rejected typed proofs, unique and digest-sorted |
| coverage | ConstructCoverage[16] | every family times SWE-bench/Terminal-Bench source format with attempted/applied/rejected/group/selected/reason denominators |
| selected_case_ids | string[283] | all prespecified quota selections available without predicate relaxation |
| quota_shortfalls | CorpusCount[1] | exactly `omitted_test_evidence=37` |
| source_tasks, total_attempts, applied_attempts, rejected_attempts, selected_cases, quotas_satisfied | aggregate | `100,939,689,250,283,false` and exact array consistency |
| findings | sorted string array | complete universe, no predicate relaxation, typed-proof natural audit, exact shortfall |
| digest | SHA-256 | `af0c0fd56fb498586096a8776e0d40794ee93acf5afda67cc000e576bfcef4d2` |

The omitted-evidence coverage rows are fixed at SWE `80/0/80/0` and Terminal
`118/3/115/3` for attempted/applied/rejected/selected. Every rejection reason is
`unverified_evidence_role`. The three applied cases are direct outcome probes,
split as two development and one calibration, with zero held-out test case.
This audit MUST NOT be coerced into an eight-family balanced release. Downstream
sampling MUST define a seven-family balanced inferential core and report the
three omitted-evidence cases as a separate exhaustive scarcity sentinel. The
audit supports no population, human-review, provider, or verifier-performance
inference.

`CorpusReleaseV3`:
| field | type | invariant |
|---|---|---|
| schema_version, canonical_policy, split_algorithm | closed identity | `evalwitness.corruption-corpus-release.v3`; canonical mutation JSON; lineage-component 60/20/20 split |
| corpus_version, plan_digest, audit_digest, mutation_program_digest | closed parent binding | exact v3 corpus, development plan, natural audit, and typed mutation program |
| policy | CorpusReleasePolicyV3 | 7 sorted core families, 40 cases each, core=280; sentinel=`omitted_test_evidence`, cases=3, splits calibration=1/development=2/test=0; exhaustive=true; primary/held-out/balanced-eight claims=false |
| sources | CorpusSource[200] | byte-equivalent to the audited source set |
| cases | CorpusCaseV3[283] | every audit-selected mutation exactly once; full manifest, blind packet, optional reduction, regeneration key, and applied ConstructFirewallReportV2 |
| construct_rejections | ConstructFirewallReportV2[250] | byte-equivalent to all audited rejected firewalls |
| source_family_counts, mutation_family_counts, split_counts, task_count | exact denominators | reproduce sources/cases; task_count=100 |
| selected_cases, applied_attempts, rejected_attempts | aggregate | `283,689,250` |
| digest | SHA-256 | `9b4999dafe2d37ea04c298b80a7aba0a1769755fdfd650cd01bf3a9cc31a2e42` |

v3 governance selection: exclude every sentinel source, task group, and lineage
from the primary; select two calibration and two test cases per core family;
require global source, task-group, and lineage uniqueness; accept only if one
development case per core family remains globally unique and disjoint from both
primary and sentinel; order lexicographically by family, role, and case ID.

| artifact | sample contract | exact digest |
|---|---|---|
| RelationPlanV3 | 28 primary, 7 pilot, 3 sentinel; seven core contracts; one separate omitted-evidence contract; primary/held-out sentinel flags false; empirical `not_run`; external action `not_authorized` | `6eac462cae0a5b626561d5cbea274a5c3a72c78b6cb9d50a8952be6ccbb6fa8c` |
| PrimarySampleV3 | 28 cases; 4 per core family; 14 calibration plus 14 test; 32 unique sources; 28 task groups; 28 lineages; 24 trajectory-pair plus 4 candidate-order units; complete case/source/program/manifest/witness/license/privacy/lineage/packet/regeneration/firewall commitments | `6b721bcf0fb10e47923b46d92f3c14691cbd0dd98949ab0bf1dc016d8e1c1e43` |
| ScarcitySentinelV3 | all 3 omitted-evidence cases; calibration=1, development=2, test=0; primary overlap=0; exhaustive; descriptive only; held-out claim false | `ec720a56394249a47eb4c0f7ef618471ce14c69ff8c2c13e1e851350c23e71fb` |
| PilotSampleV3 | 7 development cases; one per core family; 8 unique sources; 7 task groups; 7 lineages; primary overlap=0; sentinel overlap=0; 14 primary labels; maximum 7 tie-break labels; 14 probes | `3f0a70209575316c78b87f6cb9e7641ff1f4a023eaa86c2fe2598ee14b3c94b4` |
| StudyAmendmentV3 | issued `2026-08-10T02:50:40Z`; primary labels=56; maximum tie-break=28; probes=56; fixed stopping; no replacement; unresolved retained; sentinel excluded; empirical `not_run`; external action `not_authorized` | `a9924c62cdb6cea4aa4b9310af9a31ca5ee3cb193645cb30fd2b1135aab0ed94` |

`ScarcityPublicEvidence` is an independent public projection over the six frozen
v3 parents, not a protocol-generation descendant or an additional scientific
result.

| field | type | invariant |
|---|---|---|
| schema_version, canonical_policy, brief_policy | closed string | `evalwitness.relation-scarcity-public-evidence.v1`, relation canonical JSON v1, and scarcity public brief v1 |
| evidence_kind, construct_family | closed string | deterministic public negative evidence for `omitted_test_evidence` |
| availability | object | target=40, attempted=198, admitted=3, rejected=195, selected=3, shortfall=37, exhaustive=true |
| study_roles | object | development=2, calibration=1, test=0, primary-estimand overlap=0 |
| inferential_core, primary_sample | object | 7 families x 40=280 core cases; 28 cases/task groups/lineages in the primary |
| coverage | array[2] | exact SWE 80/0/80/0 and Terminal 118/3/115/3 attempted/admitted/rejected/selected rows |
| rejection_reasons | array[1] | `unverified_evidence_role=195` |
| cases | array[3] | ordered public role/unit/case-binding/firewall commitments; no task, source, trajectory, or owner content |
| parents | array[6] | exact corpus plan, natural audit, release, relation plan, primary sample, and scarcity sentinel IDs/digests |
| claims | array[7] | exact supported, unsupported, not-run, not-measured, and not-authorized states |
| digest | SHA-256 | canonical content digest with digest omitted; standalone validation recomputes it |

JSON is the only machine-consumption surface. Markdown is a deterministic
validated projection and MUST NOT be parsed for downstream metrics or policy.

`OwnerInspectionPublicAttestation` is a public aggregate projected only after
the complete private session/event/inspection/completion chain and governed v3
parents verify. It is owner-attested evidence, not an independent human result.

| field | type | invariant |
|---|---|---|
| schema_version, canonical_policy | closed string | `evalwitness.relation-owner-inspection-public-attestation.v1`; relation canonical JSON v1 |
| evidence_kind, inspection_mode | closed string | owner-authorized agent-assisted construct inspection; private-chain-verified public aggregate |
| inspection_date | date | UTC date only; no private event or completion timestamp |
| package_inventory_digest | SHA-256 | exact public package-v5 inventory commitment |
| assessments | object | required=completed=66; events=66; corrections=0; core=50; scarcity cases=12; scarcity boundary=4; core cases=7; scarcity cases=3; scarcity test cases=0 |
| dimensions | array[16] | ordered eight core, four scarcity-case, and four scarcity-boundary dimensions with exact applicable/passed/failed/indeterminate counts |
| outcomes | object | core=7/0/0 and passed; scarcity cases=1/2/0 and revision_required; boundary accepted; combined revision_required |
| human_study_status, external_action_status | closed string | `not_run`; `not_authorized` |
| disclosure | object | private chain verified during projection; no journal identities or restricted evidence disclosed; public validation limited to schema/digest/denominators/statuses/claim boundary; source reproduction requires withheld restricted inputs; `signature_status=unsigned_content_addressed`; embedded legacy `capsule_status=not_yet_capsule_bound`; the current reference capsule binds these exact payload bytes externally |
| claims | array[10] | public integrity supported; owner inspection and construct aggregates owner-attested; human/provider not run; external action not authorized; population/corrected-corpus claims unsupported; private source chain not publicly reproducible |
| digest | SHA-256 | canonical content digest with digest omitted; standalone validation recomputes it |

StudyAmendmentV3 inference: primary unit=`source_task_group`; effective groups=28;
one-sided exact 95% zero-contradiction upper diagnostic=`0.10146573557272465`;
at-least-one-contradiction probabilities for rates `0.05,0.10,0.20`=
`0.7621731147446675,0.9476652366972639,0.9980657186886166`; family rows are
descriptive; no per-family prevalence, held-out-sentinel, human-ground-truth,
verifier-robustness, or population claim.

Relation protocol v3 is one closed generation with 30 workflow schema identities
plus the separately typed scarcity-sentinel schema. The public governance plan
is projected only into an internal non-serializable review adapter whose digest
is the exact `RelationPlanV3.digest`; the pilot uses the same rule for the exact
`PilotSampleV3.digest`. Every public descendant has
`protocol_version=evalwitness.controlled-relation-governance.v3` and its own
`.v3` schema identity. Replay, material, mapping, readiness, change receipt, and
launch dossier additionally require `source_corpus_plan_digest`,
`source_mutation_program_digest`, and `source_construct_audit_digest`; material
and mapping require relation-contract v3, evidence-boundary v3, and the exact
typed construct-firewall digest. v1 requires all new bindings absent; v2 requires
its historical corpus-spec bindings and forbids corpus-plan bindings; v3 requires
corpus-plan bindings and forbids corpus-spec bindings. Any cross-generation
envelope, nested parent, rubric, handbook, packet, or terminal edge rejects.

v2 omitted evidence: resolve the evidence event to a canonical tool-call or
command event through call ID or call-result, parent, or reference link; classify
the frozen command vocabulary as test, check, build, or outcome_probe; bind both
events in `proof_event_ids`; remove only the evidence field. Generic completion,
download, installation, and unrelated output MUST reject.

v2 neutral formatting: require exactly one message text part, at least 12 tokens,
natural punctuation, source length above 72 characters, and no command prefix,
JSON object prefix, or code fence; wrap whitespace tokens at width 72; rebalance a
single-token final line; require exact token-sequence identity, no single-token
line, and changed bytes. Ineligible material MUST reject; padding and token-per-
paragraph fallbacks are forbidden.

v2 causal reorder: form undirected dependency components over every canonical
link kind plus equal raw source record, equal nonempty source-event ID, equal
nonempty call ID, and embedded event references; reject events in one component.
Two nonempty unequal timestamps add a temporal dependency. Only an adjacent pair
in distinct components with no temporal dependency is eligible. The report binds
both event IDs and all dependency checks.

Evidence availability allows only event evidence fields and declares
`changed_groups=[evidence_availability]`; quality and causal projections must be
equal. Presentation allows only text formatting, irrelevant text, stable aliases,
candidate order, or causally independent order and requires quality and evidence
semantics to remain equal. Semantic degradation requires an `OutcomeProof` with
`original_passed=true`, `mutated_passed=false`, `independent_of_trace=true`, and
the exact validator contract digest. Ambiguous labels cannot enter primary
relation accuracy.

`BlindReviewPacket` contains packet/content identities, pseudonymous task alias,
source format, original/mutated digests, affected-event count, sorted review
questions, and content digest. It excludes family, expected relation, source task
name, source location, and validator result. Every ambiguous case requires review;
the frozen SHA-256 rule samples proven cases before evaluator output.

`ReductionWitness` contains mutation digest, relation, and unique sorted regions.
Each region binds original event ID, field path, minimal byte ranges before and
after, and fragment digests. Reduction validates the full declared relation before
computing the longest common prefix/suffix boundary.

Trusted executable validation accepts only registry-owned absolute executable,
literal arguments, sorted environment, passing exit code, exact configuration
digest, timeout, output ceiling, and two distinct real disposable roots for one
task/revision. The operator supplies and verifies network isolation; the registry
does not create an OS sandbox. Formal controls use checked int64 arithmetic and
exact equality. Trace content cannot select executable, arguments, environment,
working directory, network policy, or resource limits.
Darwin and Linux terminate the complete validator process group after every run.
Windows executable validators require a trusted external process-tree launcher
for gold use; the built-in Windows path terminates only the direct process.

`CorpusSpec` locks source/task counts, source composition, two trajectories per
task, cases per family, sorted primary families, seed, program digest, development
audit and digest, exact design and digest, and `mutators_frozen=true`. The governed
v1 values are 100 source tasks, 60 Terminal-Bench tasks, 40 SWE-bench tasks, 200
source trajectories, 40 cases for each of eight primary families, and 320 cases.
The program digest resolves exactly one supported v1 or v2 contract tuple. A v2
development audit MUST use
`provider_free_full_corpus_build_construct_firewall_audit.v2` and bind that exact
program digest; a v1 audit MUST retain its frozen historical method and omit the
binding. The governed v2 plan is
`eval/governance/controlled-corruption-v2-plan.json`; the governed v2 spec is
`eval/governance/controlled-corruption-v2.json`.

`CorpusDevelopmentPlan`:
| field | type | invariant |
|---|---|---|
| schema_version | string | `evalwitness.corruption-corpus-development-plan.v2` |
| canonical_policy | string | `evalwitness.mutation-canonical-json.v1` |
| corpus_version, seed | string | distinct frozen v2 generation identity |
| source_tasks, terminal_tasks, swe_tasks, trajectories_per_task | integer | 100, 60, 40, and 2; composition sums exactly |
| cases_per_family, primary_families | integer, sorted unique Family array | 40 cases for each of eight construct-eligible primary families |
| mutation_program_digest | SHA-256 | exact v2 mutation/relation/evidence/firewall tuple |
| design_evidence_digest, design | SHA-256, RelationDesign | exact-binomial design reproduces before source audit |
| digest | SHA-256 | canonical pre-audit plan identity |

`CorpusDevelopmentAudit` retains the exact sorted 200-source set; every sealed
attempt; every applied and rejected firewall; coverage rows for each observed
family/source-format cell; sorted selected mutation IDs; quota shortfalls;
aggregate attempted/applied/rejected/selected denominators; reproducible findings;
set digests; plan/program identities; audit date; and canonical digest. One
trajectory task group is attempted until its first eligible construct per family;
rejections do not consume that group. The audit continues across the complete
deterministic source/group universe after the 40-case selection quota. System,
decode, derivation, relation-reduction, or sealing errors abort the audit and MUST
NOT enter a rejection denominator. Freeze requires `quotas_satisfied=true`, an
empty shortfall array, and exact reproduction of every derived field.

Governed v2 identities: plan digest
`75ce0c5eb2ab464d48685b1342af5ed64c194e03a671723eaaf772514fa87873`;
construct-audit digest
`822d8034a4a75faaf337a4abd6e51743e38104c3ffca9ce7f214e751f5d026db`;
spec digest `94989d548973dad7bfc04418781ed4f25df1b81d6ddd1fbeacc581fefaef0979`;
release digest `d0485f3484743a3d4ff907b295c0c9be11db21d2231664e5018fa2f047b6bf11`.
The complete audit denominators are 873 attempts, 738 applied, 135 rejected, 320
selected, 16 family/source-format coverage cells, and zero family shortfalls.
The governed construct-repair evidence is
`eval/governance/construct-repair-evidence-v1.json` with digest
`b166f124cc7e3f31b26676bba08602fca2d29a9df95f0f80a91c421f34e137c9`.

Source selection ranks eligible task groups and trajectories by SHA-256 of the
frozen seed and identity. Every selected Terminal task contributes two
trajectories. Every selected SWE task contributes one trajectory from each of two
distinct agent families. Split algorithm
`sha256-lineage-component-balanced-60-20-20.v1` builds transitive components over
repository, repository/task, source-task group, normalized first-user-prompt
near-duplicate ID, trajectory digest, and patch digest. It orders components by
task weight and frozen hash, then performs deterministic capacity-aware
allocation to an exact 60/20/20 task split. Every source records its
near-duplicate and lineage-component IDs; pair and mutation descendants inherit
the role. No lineage identity may occur in two roles. Roles isolate mutation and
adjudication work; previously accessed upstream tasks remain ineligible for an
untouched-confirmation claim.

`CorpusRelease` binds specification and program digests, ordered source and case
records, formal positive controls, source/family/split counts, task count, control
counts, ambiguity count, split algorithm, and release digest. Validation requires
at least 100 sources, 40 tasks, three source families, eight mutation families,
one positive control, negative and decoy controls, family counts differing by at
most one, valid manifests/packets/reductions, and no split leakage.
Each v2 case MUST embed the applied firewall report bound by its manifest.
`construct_rejections` retains every unique digest-sorted rejected report from
the complete v2 deterministic attempt universe, including rejections observed
after selection quotas were filled. v1 cases and releases MUST omit both fields.
Build and replay route exclusively from the program digest; v2 scarcity MUST fail quota
completion rather than fall back to v1 or weaken an eligibility predicate. A v2
release is admissible only when its sources, sorted case IDs, selected applied
firewalls, complete rejection set, and spec audit digest exactly reproduce the
bound `CorpusDevelopmentAudit`.

Primary relation design uses `ExactBinomialUpperDesign(tasks=40, null=0.5,
alternative=0.8, alpha=0.00625)`: critical successes 29, exact null tail
0.0032132880478457347, and alternative power 0.9124947640780025. Source task is
the independent within-family unit and the cross-family cluster unit.

Public release descriptors contain references, digests, witnesses, license and
privacy metadata, not upstream trajectory bodies. Terminal-Bench source metadata
declares Apache-2.0 and the pinned leaderboard snapshot; SWE-bench metadata
declares MIT and reference-only treatment for embedded third-party patch content;
formal-control fixtures use the repository MIT license.

## Metamorphic and Differential Stress Lab

Contracts:

| object | schema |
|---|---|
| relation | `evalwitness.stress-relation.v1` |
| construct admission | `evalwitness.stress-construct-admission.v1` |
| result | `evalwitness.stress-result.v1` |
| stage trace | `evalwitness.stress-stage-trace.v1` |
| stage comparison | `evalwitness.stress-stage-comparison.v1` |
| counterexample | `evalwitness.stress-counterexample.v1` |
| reduction observation | `evalwitness.stress-reduction-observation.v1` |
| relation registry | `evalwitness.stress-relation-registry.v1` |
| replay execution | `evalwitness.stress-replay-execution.v1` |
| arm comparison plan | `evalwitness.stress-arm-comparison-plan.v1` |
| arm comparison report | `evalwitness.stress-arm-comparison-report.v1` |
| analysis design | `evalwitness.stress-analysis-design.v1` |
| analysis report | `evalwitness.stress-analysis-report.v1` |
| zero-cost execution | `evalwitness.stress-zero-cost-execution.v1` |
| protocol adapter proof | `evalwitness.stress-protocol-adapter-proof.v1` |
| arm replay evidence | `evalwitness.stress-arm-replay-evidence.v1` |
| held-out partition lock | `evalwitness.stress-held-out-partition-lock.v1` |
| held-out campaign plan | `evalwitness.stress-held-out-campaign-plan.v1` |
| held-out campaign batch binding | `evalwitness.stress-held-out-campaign-batch-binding.v1` |
| held-out admission plan | `evalwitness.stress-held-out-admission-plan.v1` |
| held-out execution batch binding | `evalwitness.stress-held-out-execution-batch-binding.v1` |
| held-out preflight evidence | `evalwitness.stress-held-out-preflight-evidence.v1` |
| held-out preflight custody | `evalwitness.stress-held-out-preflight-custody.v1` |
| held-out execution permit | `evalwitness.stress-held-out-execution-permit.v2` |
| held-out execution reservation | `evalwitness.stress-held-out-execution-reservation.v1` |
| held-out run readiness refusal | `evalwitness.stress-held-out-run-readiness-refusal.v1` |
| held-out run seal, complete-support mechanism | `evalwitness.stress-held-out-run-seal.v1` |
| held-out run seal, admission-filtered execution | `evalwitness.stress-held-out-run-seal.v2` |
| next-version discovery ledger | `evalwitness.stress-next-version-discovery-ledger.v1` |
| development case study | `evalwitness.stress-development-case-study.v1` |
| development challenge | `evalwitness.stress-development-challenge.v1` |
| development challenge receipt | `evalwitness.stress-development-challenge-receipt.v1` |
| canonical policy | `evalwitness.stress-canonical-json.v1` |

`Relation`:

| field | type | constraint |
|---|---|---|
| id, revision | string, positive integer | stable relation identity and revision |
| kind | invariance, sensitivity, differential | exact expected relation class |
| applicability | Applicability | unit, 1..10 trajectory bounds, sorted source formats, sorted typed source requirements |
| transform | Transform | mutation, trace mapping, entrypoint, provider route, or extraction mode; exact identifier, version, and declared changed stage |
| constraints | ExpectedConstraint[] | nonempty unique identity-sorted decision, rank, conditional-score/variance, support, divergence, or mass constraints; side-movement metrics use original/transformed values and effect, pair metrics use one target value, all with explicit tolerance |
| invalid_states | InvalidState[] | nonempty unique sorted closed pre-execution exclusions |
| repeat_policy | RepeatPolicy | fixed equal minimum/maximum or registered adaptive range, maximum 16 |
| statistical_family | StatisticalFamily | family ID, primary-core/scarcity-sentinel/sensitivity/diagnostic estimand, cluster unit, multiplicity method, estimand-specific denominator policy, and closed failure mapping |
| stage_expectations | StageExpectation[6] | every ordered stage exactly once with must-match, must-differ, or may-differ |
| digest | SHA-256 | canonical content identity |

Mutation transforms MUST equal the registered v3 mutation operator, family,
intervention class, formal relation, trajectory unit, and ingestion changed
layer. Required evidence is v3 manifest, v3 construct firewall, formal witness,
exact replay, and owner attestation. `omitted_test_evidence` MUST NOT use
`primary_core`; its `scarcity_sentinel` estimand requires
`multiplicity_method=none_descriptive`.

Canonical failure mapping: invalid -> invalid; missing score and abstention ->
abstained; provider, route, timeout, and retry exhaustion -> provider_failed;
budget exhaustion and incomplete cell -> inconclusive; unsupported ->
unsupported. Denominator treatment is
`retain_every_registered_case_by_admission_and_outcome_without_complete_case_deletion`.
Primary relations require
`human_supported_source_task_relations_with_all_post_admission_outcomes`;
sensitivity relations require
`eligible_source_task_relations_stratified_by_admission_with_all_post_admission_outcomes`;
the scarcity sentinel requires
`available_sentinel_relations_over_40_target_source_tasks_with_all_post_admission_outcomes`.

`RelationRegistry` binds the exact v3 development plan, design evidence, natural
audit, release, mutation program, seven-family 280-case core, and exhaustive
three-case omitted-evidence sentinel to exactly 15 relations. Every core family
has one `primary_core` and one admission-stratified `sensitivity` relation; the
sentinel has one isolated 3/40 descriptive relation. Every relation covers each
source format used by its released cases and uses the locked `source_task`
cluster. `replay_corpus_identity_digest` commits the canonical projection of
case ID, split, task group, family, manifest digest, and ordered original and
transformed trajectory digests derived from the validated release. Any
`primary_core` relation additionally requires the v3 terminal-
ledger contract. Registry validation rejects changes to the canonical metric,
repeat, denominator, multiplicity, failure, evidence, or stage contract. It does
not create a study lock. The stress analysis design owns relation-rate inference;
paired-only study designs and corpus-construction power MUST NOT substitute for
verifier-robustness inference.

Construct admission:

| status | primary eligible | sensitivity eligible | required evidence |
|---|---:|---:|---|
| formal_only | false | true | exact valid v3 case chain plus passed non-disclosing 66-assessment, 16-dimension owner attestation; no terminal ledger |
| human_supported | true | true | formal evidence plus exact primary-audit terminal-ledger entry supporting the same case, family, formal relation, and witness |
| human_contradicted | false | false | exact terminal-ledger contradiction; retained only as `invalid/human_contradicted` with zero provider calls |
| human_unresolved | false | true | exact unresolved terminal-ledger entry |

Admission order: validate sealed relation -> require mutation transform -> verify
v3 manifest, blind packet, typed firewall, formal witness, case/family/relation
identity, and exact replay chain -> verify owner-attestation schema, digest,
package digest, 66/66 completion, 16 dimensions, passed aggregate, private-chain
verification, and non-disclosure -> verify optional primary-audit terminal ledger
and exact entry binding -> derive admission status and denominator roles -> seal
content digest. Caller-supplied status or eligibility is forbidden.

Stage order: `ingestion -> request_construction -> provider_response ->
score_extraction -> decision_policy -> rendering`. `NewStageRecord(stage, unit,
canonical_bytes)` hashes length-prefixed stage, unit, and nonempty canonical
bytes. A stage trace is a unique ordered prefix beginning at ingestion. Stage
comparison binds the relation and both trace digests, records every observed
difference, and derives the earliest observed and earliest unexpected stage.
Its fixed claim is `earliest observed digest divergence only; no causal
attribution beyond the declared controlled transform`.

Support Jaccard, probability overlap, and common-support divergence are scalar
pair-comparison metrics and MUST be evaluated against `target_value`; fabricating
original/transformed values for them is invalid. Rank, conditional score,
conditional variance, and mass metrics use explicit original/transformed values.
Rank values are integers in `[1,10]` and lower is better for preference operators; conditional score and all probability-mass
values are in `[0,1]`; conditional variance is in `[0,0.25]`.

Canonical v3 relation constraints:

| formal relation and unit | required observation | operator and frozen margin |
|---|---|---|
| `quality_equal`, single trajectory | original/transformed conditional expected score | absolute movement `<=0.05` |
| `no_control_effect`, single trajectory | original/transformed conditional expected score | absolute movement `<=0.05` |
| `original_better`, single trajectory | original/transformed conditional expected score | transformed minus original `<=-0.05` |
| `verified_outcome_dominates`, single trajectory | original/transformed conditional expected score | transformed minus original `<=-0.05` |
| `quality_equal_evidence_weaker`, single trajectory | original/transformed conditional expected score | transformed minus original `<=0.05` |
| `quality_equal`, candidate pair | selected trajectory digest before/after reversal | exact equality |

The fixed arithmetic tolerance is `1e-12`. Score margins are catalog-v1
engineering thresholds, not empirical equivalence bounds. A margin change
requires a new catalog identity and locked analysis design. Slot indices MUST
NOT substitute for selected trajectory identity. Separate absolute runs MUST
NOT emit an original/transformed selection claim.

`Result` outcomes: `satisfied`, `violated`, `abstained`, `invalid`,
`unsupported`, `provider_failed`, `inconclusive`. Required constraints are
recomputed from their stored numeric values or exact decision states. A violated
outcome requires an observed required violation; abstained and inconclusive
outcomes cannot hide a violation; provider failure retains planned/completed
repetition and provider-call counts. Unsupported and pre-admission invalid
results contain no admission or execution evidence. Human contradiction retains
its exact admission but has zero completed repetitions and zero provider calls.
Primary-core results require human-supported admission; non-primary execution
requires sensitivity eligibility.

Distribution comparisons MUST use
`evalwitness.score-evidence-comparison.v1` and retain support Jaccard,
probability overlap, common-support conditional divergence, visible/valid-mass
movement, conditional-score movement, conditional-variance movement, both
missing-tail bounds, and interval overlap. Missing support remains censored.

`ReductionObservation` binds the exact relation, privacy policy, relation proof,
privacy proof, replay result, relation/privacy/violation decisions, and digest.

`Counterexample` binds relation, source result, case, original/reduced input,
privacy policy, release eligibility, algorithm, minimality class, original/final
unit sets, original observation, ordered reduction steps, accepted count, and
digest. The original observation and every step embed a self-validating
`ReductionObservation`; each step also binds before/candidate/after digests.
Rejected steps preserve the before digest. Accepted steps use the candidate
digest and require relation revalidation, privacy revalidation, and preserved
violation. Reducer execution: enumerate deterministic bounded units -> remove one
unit -> revalidate source relation and privacy -> replay the same verifier path
-> retain only the same typed violation -> record the decision -> restart after
acceptance -> perform a final rejected-removal pass over every retained unit.
`one_minimal` means no declared retained unit can be removed individually while
preserving all three invariants; it MUST NOT be reported as a global minimum.

`DevelopmentCaseStudy`:

| field | invariant |
|---|---|
| schema_version, canonical_policy | `evalwitness.stress-development-case-study.v1`; stress canonical JSON v1 |
| case_study_id, data_role, status | `first-listed-candidate-order-one-minimal`; `adapter_development`; `mechanism_demonstration` |
| task, trajectories, license | exact repository-relative regular-file paths, byte counts, SHA-256 values, and MIT license binding for the checked-in task and two trajectory fixtures |
| task_requirement, fixture_set_digest | exact task bytes and canonical complete-fixture commitment |
| relation | canonical v3 `candidate_order_reversal` sensitivity relation over plain-text trajectory pairs |
| observation | existing `first-listed` zero-cost arm; original selection A, reversed selection B, violated identity constraint, one completed repetition, zero provider calls, no network |
| counterexample | complete shared-reducer observation and accepted/rejected step chain; `one_minimal`, not global minimum |
| final_witness | exactly trajectory A line 13 `5` and trajectory B line 13 `-1` |
| reduction | 32 original line units, 2 final units, 30 accepted removals, 93.75 percent removed |
| execution boundary | `empirical_units=0`, `provider_calls=0`, `network_required=false` |
| claims | exact allowlist for checked-in-fixture reproduction, deterministic candidate-order negative control, and shared one-minimal reducer; exact denylist for global minimum, held-out confirmation, provider/model comparison, population generalization, and verifier reliability |
| digest | canonical content digest `b4a7adb23505a88ee0ab79c73957bce5fbd15539a0e5e687590719d3ec4ee84b` |

Build MUST use only `scripts/tests/sample-task.txt`,
`scripts/tests/sample-traj-a.txt`, `scripts/tests/sample-traj-b.txt`, and
`LICENSE`; validate their regular-file status, size stability, bytes, SHA-256,
and exact MIT text; reproduce the canonical relation and observation; execute
`ReduceCounterexample`; validate the complete one-minimal chain; and seal the
artifact. Repository validation MUST rebuild the complete value and require
canonical byte equality. Strict decoding rejects unknown fields, trailing JSON,
and documents above 16 MiB. Markdown rendering MUST derive only from a validated
object. Provider configuration, credentials, endpoints, and network state MUST
NOT affect either projection.

`DevelopmentChallenge`:

| field | invariant |
|---|---|
| identity | `evalwitness.stress-development-challenge.v1`; canonical policy v1; challenge ID `candidate-order-one-minimal-portable-challenge`; status `self_contained_mechanism_challenge` |
| fixtures | exactly task, trajectory A, trajectory B, and MIT license in fixed order; positive byte count, exact SHA-256, bounded lowercase hex content, canonical fixture-set digest |
| expectation | case digest `b4a7adb23505a88ee0ab79c73957bce5fbd15539a0e5e687590719d3ec4ee84b`; violated outcome; 32 original and 2 final lines; 53 attempts; 30 accepted reductions; two final rejection attempts; exact retained unit IDs and mechanism-only claim boundary |
| execution boundary | `empirical_units=0`; `provider_calls=0`; `network_required=false` |
| claims | one exact self-contained-mechanism claim; exact denylist for global minimum, held-out confirmation, independent implementation replication, verifier/provider reliability, and population generalization |
| digest | canonical content identity excluding the digest field |

Verification MUST strictly decode at most 1 MiB, reject unknown fields and
trailing JSON, verify every embedded byte and fixture identity, reconstruct the
canonical `DevelopmentCaseStudy` in memory through the same builder and reducer,
compare every expectation, and execute the fixed guard set. It MUST NOT require
repository access, a temporary fixture tree, provider configuration, a key, or
network state.

`DevelopmentChallengeReceipt` binds the exact challenge, expected and reproduced
case, fixture set, four verified fixtures, complete 32/2 and 53/30/2 reduction
accounting, reproduced violation and one-minimality, preserved claim boundary,
zero repository/empirical/provider/network requirements, seven ordered guard
receipts, and a canonical digest. The guard set contains one positive portable-
reproduction control plus fixture-byte, expected-case, fixture-inventory,
unknown-field, trailing-document, and challenge-digest substitutions. Receipt
validation MUST reject any changed definition, expected/observed guard mismatch,
failed guard, foreign challenge, wrong case, incomplete accounting, or widened
execution boundary. The receipt proves execution of the released implementation,
not an independent reimplementation.

Closed JSON Schema 2020-12 documents expose the 35 stress object schemas with unknown
object properties rejected. The shared stress runner consumes
`internal/verification` plans and results, never constructs requests, invokes a
provider, extracts scores, or selects a decision independently. CLI, MCP,
protocol adapter, replay, and benchmark entrypoints use the same relation and
stage ports. Replay execution MUST use one cache-disabled two-side batch, carry
the admitted case and relation in both lineages, reject a batch authorization,
and receive digest-only `evalwitness.score-call-observation.v1` records with
`replay_status=exact` and `extraction_status=complete` for both plan
fingerprints. The runner MUST deep-snapshot the validated request separately
from the executor-owned copy and reject any plan that binds mutated inputs.
Fixed `n_reps` or adaptive SPRT minimum/maximum repetitions MUST equal the
sealed relation repeat policy. `BudgetStatePath` MUST be empty; persistent
cross-case budget state is forbidden. Untrusted trajectory bytes MUST NOT select
commands, endpoints, authorization, cache policy, worker policy, network access,
score output, or state consumed by a later case. Live cells require the existing exact authorization, route
attestation, study, budget, and capsule boundaries; missing replay evidence MUST
NOT trigger live work.

`ReplayV3RelationCorpus(root, plan, audit, release, registry)` MUST validate the
exact registry/release chain, resolve every frozen source, reproduce every v3
case through `mutation.ReplayCorpusCaseV3`, and require coverage of 40 cases for
each primary and sensitivity core relation plus 3 cases for the scarcity
relation. TASK 047 conformance entrypoints are `cli.verify`, `mcp.delta`,
`eval-terminal`, `eval-swebench`, `best-of-n`, and `protocol.application`.
Equivalent replay MUST preserve batch/request identity, extraction, decision,
and rendering. Provider-response evidence MUST retain entrypoint provenance and
MAY therefore differ only at that localized stage. Test-only admission evidence
MUST NOT authorize a live or empirical cell.

`ArmComparisonPlan` MUST bind one validated relation-registry digest, release
digest, identity-sorted canonical arm set, and exact identity-sorted Cartesian
cell set. Before cell expansion, the replayed corpus projection MUST reproduce
the registry's release-derived replay identity commitment; equal counts cannot
admit a changed split, task group, family, manifest, or trajectory digest.
Canonical arms: `score-token-verifier` = `cli.verify` plus
`strict_verifier`; `explicit-text-judge` = `cli.verify` plus `explicit_judge`;
`external-protocol-adapter` = `protocol.application` plus `strict_verifier`,
protocol version `1.2.0`, application invocation/result v1, and required external
process conformance; zero-cost arms = first listed and both directions of steps,
trace bytes, and error words from `internal/baseline`. Exact v3 counts: 563
relation-case identities; 10 arms; 5,630 cells; 2,249 supported; 3,381
unsupported. Zero-cost arms support only the 80 candidate-order
primary/sensitivity relation cases. Unsupported cells MUST remain explicit.
`global_score=false` is invariant.

`StressAnalysisDesign` MUST bind the registry and arm plan; primary data role
`test`; cluster unit `source_task`; any-violation and any-non-satisfied cluster
aggregation; complete supported-cell missingness retention; source-task Wilson
score intervals; nominal alpha `0.05`; Bonferroni adjustment within each
estimand's supported endpoints; and `score-token-verifier` as the paired
contrast reference. Exact test families are 28 arm-relation rate endpoints and
21 paired contrasts for each of `primary_core` and `sensitivity`.
`global_score=false` and `population_generalization=false` are invariant.

`StressAnalysisReport` MUST revalidate the exact arm plan, complete arm evidence,
analysis design, registry, and replayed corpus before emitting any estimate. It
MUST produce relation-by-arm-by-split summaries with all satisfied, violated,
abstained, invalid, runtime-unsupported, provider-failed, inconclusive, and
not-run cells; admission strata; source-task clusters; capsule counts; violation
and failure rates; and inference status. Any supported not-run cell blocks that
endpoint's interval and inference. An endpoint is `not_run` only when none of its
cells executed; any partial execution is `incomplete`, independent of source-
task iteration order. Complete test endpoints use Bonferroni-
adjusted source-task Wilson intervals. Complete test contrasts use paired
failure indicators, Newcombe method-10 risk-difference intervals, and exact
two-sided McNemar p-values at the locked adjusted alpha. Non-test and scarcity
rows are descriptive. Every violated result MUST have one ledger entry that
reports whether its exact capsule and one-minimal counterexample are both bound;
unused or cross-cell counterexamples MUST be rejected. No aggregate robustness
score is permitted.

`HeldOutPartitionLock` MUST bind the registry, release, arm plan, analysis
design, source catalog, exact next catalog, `test` data role, once-only run
policy, no-retrofit discovery policy, test-only inference policy, and canonical
sorted identities for every test case, test cell, supported test cell, and
unsupported test cell. Catalog-v1 exact counts: 57 cases; 1,140 cells; 440 supported;
700 structurally unsupported. Supported and unsupported sets MUST be disjoint and
their union MUST equal the complete test-cell set. The next catalog MUST be the
source catalog's positive integer `.vN` suffix incremented by one.

`HeldOutCampaignPlan` MUST revalidate the held-out partition, registry, release,
arm plan, analysis design, and replayed corpus, then bind the exact canonical
ten-arm topology with identity-sorted per-arm test, supported, and unsupported
cell-set commitments. Catalog-v1 counts are 57 test cases, 1,140 test cells, 440
supported cells, and 700 structural unsupported cells. Every arm MUST bind one
execution class. `explicit-text-judge` and `score-token-verifier` MUST be
`live_provider`, each with 114 supported cells, 228 original/transformed inputs,
and 684 registered side repetitions. `external-protocol-adapter` MUST be
`sealed_provider_replay`, with 114 supported cells, 228 inputs, 684 repetitions,
and no independent live-provider authority. The seven zero-cost arms MUST be
`deterministic_local`, each with 14 supported and 100 unsupported cells plus 42
repetitions. Totals are 342 provider-dependent cells, 684 inputs, 2,052 side
repetitions, 98 zero-cost supported cells, and 294 zero-cost repetitions. The
two live-provider request plans, sealed-replay plan, provider call counts,
provider budgets, current route attestations, authorization digests, StudyRecord,
execution bindings, and private capsule family MUST all remain absent.
`run_authorized=false`,
`execution_permit_issued=false`, `external_action_status=not_authorized`,
`provider_calls=0`, `empirical_units=0`, and `network_required=false` are
invariant. Registered side repetitions MUST NOT be interpreted as provider call
counts because request construction, criteria bundling, and bias policy are not
bound by this topology contract.

`verification.BatchPlanBinding` MUST validate the originating in-memory batch
through its `verification.Service` without runtime creation or execution and
MUST bind offline state, entrypoint, evidence policy, StudyManifest digest,
budget-state path, cache policy, input modes, task/criteria/base-policy digests,
fixed-repetition state, raw and prepared trajectory digests, evidence-accounting
digests, complete research lineage, ordered plan fingerprints, request set,
request and capability contracts, route identity, input/output ceilings,
worst-case logical calls, workers, hard budget, and the required live
authorization digest. A supplied approval digest MUST be rejected.

`HeldOutCampaignArmBatchBinding` MUST validate one canonical provider-dependent
arm against the exact held-out replay corpus and retain one original plus one
transformed input per supported cell in case/relation/side order. All inputs MUST
disable cache, use the fixed registered repetition count, bind outcome,
profile-policy, and capsule digests, and match the arm entrypoint and evidence
policy. Live arms MUST carry distinct persistent budget-state paths; the replay arm MUST
be offline and carry none. `HeldOutCampaignBatchBinding` MUST combine exactly
two live arm bindings and one replay binding, require one StudyManifest and route,
equal cross-arm mode/task/criteria/base-policy/raw-corpus/side/case/relation/
outcome/profile identity, aggregate live budgets, retain replay budget separately,
and bind exactly two required authorization digests. The adapter request
fingerprints, request set, request and capability contracts, route, ceilings,
logical calls, and budget MUST exactly equal the strict-verifier live source.
`status=provider_batches_bound_execution_not_authorized`, every StudyRecord,
execution-binding, route-attestation, capsule-family, run-authority, and permit
flag false, `external_action_status=not_authorized`, `provider_calls=0`,
`empirical_units=0`, and `network_required=false` are invariant.

`HeldOutAdmissionPlan` MUST revalidate the exact campaign, partition, analysis,
arm, registry, replay, v3 corpus-governance, passed owner, 28-case primary-
sample, and complete terminal-ledger parents. It MUST partition all 440
structurally supported cells into unique sorted execution-eligible and pre-
execution-ineligible sets without changing structural unsupported accounting.
Only human-supported primary cases and separately allowed sensitivity cases may
enter execution; human contradiction MUST enter neither estimand. Provider,
live, replay, and deterministic-local cell/input/repetition projections MUST
reproduce from the exact filtered sets. It MUST remain non-authorizing with zero
provider calls and empirical units.

`HeldOutExecutionBatchBinding` MUST rebuild exactly the admission-eligible
provider cells as two live `verification.BatchPlanBinding` values and one
offline strict-verifier replay binding. Every arm MUST bind its exact eligible
cell set, both relation sides, outcome/profile/private-capsule lineage, locked
StudyManifest, route, request and capability contracts, persistent budget state
for live work, and hard budget. The live arms MUST expose exactly two distinct
authorization requirements; the adapter MUST reuse the strict-verifier request
and budget contract through sealed replay and MUST carry no live authorization.
The structural 684-input campaign commitment MUST NOT substitute for this
filtered execution object. Study, route, capsule, permit, action, call, and
empirical flags remain false.

`HeldOutPreflightEvidence` MUST contain one valid authorized StudyRecord, exactly
two arm-sorted `study.ExecutionBinding` values, every unique current route-plus-
capability attestation required by the two live batches, exactly two digest-
sorted `mode.AuthorizationPlan` values, and one canonical UTC verification time.
Each study arm's `attestation_digest` MUST commit the sorted set of all its
capability-attestation digests; it is not a single-attestation shortcut. Probe-
compatible evidence is sufficient only for the explicit-text judge; strict-
verifier requests require bounded qualification. Authorization-plan integrity
MUST NOT be treated as the caller's explicit execution approval.

`HeldOutPreflightCustody` MUST verify the registered passed private relation
capsule, exact admission and execution parents, authorized study lifecycle,
both execution bindings, exact arm route/capability sets at `verified_at`, both
authorization plans, profile policy, and admission-filtered workload. It MUST
bind all parent digests and the earliest route-attestation expiry. It MUST set
every prerequisite verification flag true while retaining
`run_authorized=false`, `execution_permit_issued=false`,
`external_action_status=not_authorized`, zero calls, zero empirical units, and
no network requirement.

The held-out preflight capsule registry MUST extend the verified private
relation registry with exact-byte JSON admission, execution, preflight-evidence,
and custody components. Admission MUST derive externally from the passed private
relation proof; execution MUST derive from admission; evidence MUST derive from
execution; custody MUST derive from all three internal parents plus that exact
external proof and MUST be the sole scientific root. Family verification MUST
fail without the private parent, with a foreign registry, substituted payload,
missing component, changed parent, or non-passed owner proof.

`HeldOutExecutionPermit` MUST be built only after complete preflight-family
verification, at or after custody verification and strictly before the earliest
route expiry. It MUST re-evaluate every route capability at issue and verification
time, require the exact two execution authorization digests as explicit caller
input, and bind the preflight capsule/manifest/registry, private relation capsule,
custody/evidence/admission/execution/study/profile lineage, route and execution-
binding digest sets, both live-arm cell/input/request/capability/route/budget
projections, aggregate live budget, issue time, and exact expiry. It MUST set
`run_authorized=true`, `execution_permit_issued=true`, `single_use=true`, atomic
execution reservation and run-seal requirements, and bind exactly one valid
owner-only local reservation authority identity and exclusive-link backend
version, while retaining
`execution_started=false`, `provider_calls=0`, `empirical_units=0`, and
`network_performed=false`. Network is required only for the later execution.
The permit alone MUST NOT claim reservation, replay prevention, execution,
response evidence, reliability, or a run seal.

`HeldOutExecutionReservation` MUST be created only through the authority bound
by the permit. The authority MUST completely re-verify the permit, private
preflight capsule family, admission-filtered execution binding, StudyRecord,
route capability and freshness, budgets, and both caller-supplied authorization
digests immediately before reservation. It MUST publish an owner-only canonical
receipt under the permit digest through a filesystem-confined atomic no-replace
operation. At most one contender within that authority may succeed. The receipt
MUST bind the permit, preflight capsule, authority, reservation key and time, and
permit expiry while retaining `execution_started=false`, `provider_calls=0`,
`empirical_units=0`, and `network_performed=false`. It MUST NOT claim exactly-once
execution, execution start, provider evidence, distributed consensus, empirical
reliability, replay prevention after owner deletion, rollback, or cloning of the
authority root, or a run seal. Any run seal that supports a real held-out claim
MUST bind the exact permit and stored receipt.

`ExecuteHeldOutLiveBatch` MUST accept only the exact unapproved batch preview
already bound into one live arm and permit. It MUST verify the current
reservation and permit window, apply the already approved authorization digest
in memory without replanning, execute that batch once through the shared
`verification.Service`, and collect one digest-only
`evalwitness.score-call-observation.v1` per completed logical call.
`HeldOutLiveBatchEvidence` MUST bind the exact batch binding, permit,
reservation, run fingerprint, arm, eligible cell set, both sides of every cell,
result digests, response-body/parsed/response/score-evidence digests, budget,
lifecycle, served model, completion time, and non-empty upstream provider
request identity for every `replay_status=live` observation. Provider calls MUST
derive from per-result usage and the aggregate runtime budget, never from an arm
report projection. This proves returned live-provider response records, not
packet-level transport, global provider identity, exactly-once execution, or
reliability.

The batch authorization is aggregate authority over the complete request set.
Each constituent plan MUST retain and validate its own narrower authorization
projection; constituent authorization digests are not required to equal the
aggregate digest. Execution applies the one approved aggregate digest to every
input only as the shared batch approval consumed by `verification.ExecuteBatch`.

`HeldOutLiveReplayVerification` MUST accept only independently executed
`replay_status=exact` `ArmReplayEvidence`. It MUST revalidate every replay
against the plan, relation, admission, case, and arm, require one exact replay
for each live cell, and prove equality of response-evidence sets, normalized
score observations, verifier decisions, and logical-call counts. Every exact
observation MUST carry one identical `evalwitness.exact-replay-source.v1`
descriptor. The built-in production loader MUST derive it from the same stable,
single-read bytes used to load the replay entries. The descriptor MUST bind the
capture SHA-256 and byte count; record count; capture, request, and parser
contract versions; provider, route, and requested model; and request, lineage,
response-body, response-evidence, and record-set digests. It MUST match the
planned and held-out live-arm route and MUST contain at least as many records as
reproduced logical calls. Independent capture-byte validation additionally
requires retaining and re-inspecting the exact capture parent; the descriptor
alone MUST NOT be represented as that external proof. The normalization MAY
remove only replay-source and live/exact status fields, scope fingerprints,
budget telemetry, and usage accounting; it MUST retain provider request
identity, request, response, parsed-payload, score-evidence, extraction,
decision, uncertainty, and evidence-strength semantics. Changing a live
observation's status or manufacturing exact status without capture provenance
is not replay and MUST NOT create this artifact.

`HeldOutExecutionLedger` MUST bind the exact admission, execution, permit,
reservation, two live-batch evidence artifacts, two independent live-to-exact
replay verifications, arm report, and analysis. Every one of the 1,140 locked
test cells MUST remain visible: 700 structural unsupported, 164 pre-execution
ineligible, and 276 executed evidence units in the current governed fixture.
Each live arm MUST bind its live evidence, replay verification, and exact replay
evidence-set digest; the arm report MUST contain that same replay set. Sealed-
replay and deterministic-local arms MUST carry only their registered authority.
`live_response_evidence_observed=true` means validated live response records
exist and MUST NOT be described as packet-level network proof.
`analysis_completion_status=incomplete_due_to_pre_execution_exclusions` is
mandatory while any structurally supported cell was excluded before execution;
the ledger MUST NOT support population generalization, provider/verifier
superiority without the bound analysis, exactly-once execution, or run-seal-v1
completion.

`HeldOutRunReadinessRefusal` MUST revalidate one `HeldOutPartitionLock`, arm
plan, analysis design, relation registry, replayed corpus, closed owner
attestation, and independently supplied expected owner-package inventory digest.
Its ordered required gate set is held-out partition lock, owner inspection,
blinded human admission, authorized study record, execution and budget binding,
current route attestations, live authorization, and private capsule family.
Gate status is `passed`, `blocked`, or `missing`; passed and blocked gates MUST
carry a valid evidence digest, while missing gates MUST carry none. At least one
gate MUST be blocked or missing. `status=not_ready`, `run_authorized=false`,
`execution_permit_issued=false`, `external_action_status=not_authorized`,
`provider_calls=0`, `empirical_units=0`, and `network_required=false` are
invariant. The receipt MUST NOT authorize an executor or support held-out,
verifier-reliability, provider-quality, population, or authorization claims.

`HeldOutRunSeal` MUST bind one `HeldOutPartitionLock`, registry, arm plan,
analysis design, arm report, and analysis report; set `run_count=1`,
`reopened=false`, and `completed=true`; execute every supported test cell; retain
every unsupported test cell; require every supported test summary to be
`adjusted_complete` with zero not-run cells; and conserve violated cells as
exactly witness-bound plus witness-missing. Any supplied prior seal MUST fail as
`locked_partition_already_used`. A not-run or incomplete test report MUST NOT be
sealed.

Run-seal v1 is restricted to provider-free mechanism verification over the
complete 440-cell structural support set and MUST NOT seal an admission-filtered
real execution.

`HeldOutRunSealV2` MUST revalidate and bind the exact lock, campaign, admission,
execution batch, permit, reservation, execution ledger, live evidence,
independent live-to-exact-replay verifications, registry, reports, and analysis
parents. It MUST conserve all 1,140 test cells as structural unsupported,
pre-execution ineligible, or admission-eligible executed; bind one evidence unit
per executed cell, provider calls, capture-source set, and exact
violation/witness accounting; and publish at most one seal per frozen partition
through the permit-bound owner-only authority. `run_count=1`, `reopened=false`,
`completed=true`, `analysis_completion_status=incomplete_due_to_pre_execution_exclusions`,
`confirmatory_inference_complete=false`, and `population_generalization=false`
are invariant. The seal MUST NOT claim execution evidence for excluded cells,
global exactly-once or rollback protection, independently re-inspected capture
bytes without retained parents, packet-level transport proof, or provider,
verifier, and population superiority.

`NextVersionDiscoveryLedger` MUST bind the held-out run seal, registry, arm
report, source and exact next catalog versions, and a canonical discovery set.
It MUST validate against either the complete-support v1 seal or the
admission-filtered v2 seal through the corresponding exact parent contract.
Each discovery MUST originate from one executed violated held-out cell and bind
its result digest, source relation, discovery-evidence digest, target catalog,
and a valid sealed relation candidate. Candidate IDs and digests MUST be unique
and absent from the frozen current registry. `current_catalog_frozen=true` and
`test_partition_retrofitted=false` are invariant. An empty ledger is valid when
the run yields no new relation.

`ArmComparisonReport` MUST contain exactly one identity-sorted observation for
every plan cell. A cell status is `executed`, `not_run`, or `unsupported`.
Executed cells MUST bind one validated `ArmReplayEvidence` or
`ZeroCostExecution`, its result digest, evidence digest, outcome, completed
repetitions, and provider-call count. Protocol cells additionally bind the
protocol-proof digest. Supported cells without supplied evidence MUST remain
`not_run/supported_cell_evidence_not_provided`; plan-unsupported cells MUST
retain their exact plan reason. Duplicate, foreign, wrong-arm, and
unsupported-cell evidence MUST be rejected. Planned, executed, not-run, and
unsupported counts MUST reproduce from the cells. `global_score=false` is
invariant.

`ReplayExecution` MUST bind schema/canonical policy, relation, case, entrypoint,
evidence policy, batch fingerprint, exact original/transformed side evidence,
one validated `evalwitness.exact-replay-source.v1`, stage comparison, and digest.
Every observed request and sampling-slot identity MUST fit within that capture's
record count, and every planned request MUST match its provider, route, and
requested model. The descriptor MUST bind the active capture, request, and
parser contract versions. The policy MUST equal the extracted evidence mode.
`SealReplayResult` MUST derive absolute-score movement or selected-trajectory
identity from that execution, completed repetitions from retained evidence, and
provider calls from both verification results. `ZeroCostExecution` MUST derive
steps, rendered trace bytes, and error-word count from the replayed canonical
trajectories, reproduce the existing baseline selection across all fixed
repetitions, emit zero provider calls, and bind selected trajectory digests.

`ProtocolAdapterProof` MUST bind exact direct/subprocess TASK 053 corpus, result,
capability-matrix, and finding parity; exact protocol/application extension
identities; the lineage adapter-conformance digest;
`provider_calls=0`; `empirical_reliability=false`; and its digest.
`ArmReplayEvidence` MUST bind one supported plan cell, exact execution and
derived result. The protocol arm additionally requires the exact protocol-proof
digest. Direct and real NDJSON-subprocess application-adapter execution MUST
produce the same result and observed run digest for the canonical source
fixture. This composite proof does not claim that the complete stress corpus
was transported through the subprocess application adapter. Complete stress-
corpus subprocess conformance requires separate evidence before empirical
protocol-arm reporting.

TASK 065 relation-backed interventions MUST use `ReplayFirstRunner`,
`ConstructAdmission`, `StageTrace`, `StageComparison`, `ReducibleInput`,
`ReductionOracle`, and `ReduceCounterexample`. A reliance-witness reduction
observation MUST bind the exact replay batch fingerprint. A parallel provider,
extraction, decision, stage-localization, or minimization path is forbidden.

## Controlled Relation Construct Audit

Constants: `protocol_version=evalwitness.controlled-relation-review.v1|evalwitness.controlled-relation-review.v2`;
`canonical_policy=evalwitness.relation-canonical-json.v1`;
`review_objective=controlled_relation`;
`blinding_protocol=evalwitness.relation-hmac-sha256.v1`;
`external_action_status=not_authorized`.

| artifact | schema | required contract |
|---|---|---|
| blind packet | `evalwitness.relation-blind-packet.v1` | objective and protocol typing, opaque packet/task/side/slot/candidate identities, explicit unit, coherent task requirement, two fixed visible positions, aligned redacted evidence with exact accounting and restricted license class, seven neutral axes, packet/reviewer order commitments, no external authorization, digest |
| blind packet v2 | `evalwitness.relation-blind-packet.v2` | protocol-v2 identity plus the blind-packet v1 visible-data and leakage contract |
| blind packet v3 | `evalwitness.relation-blind-packet.v3` | protocol-v3 reviewer-visible contract; exact v3 plan and typed packet generation are verified through the owner-only mapping while corpus-plan/audit/firewall identities remain hidden from the packet surface |
| plan | `evalwitness.relation-audit-plan.v1` | corpus identity, sample rules/sizes, reviewer workload, rubric, commit/reveal rule, unresolved rule, forbidden inputs, 12 reasons, seven axes, eight family contracts, external authorization boundary, digest |
| plan v2 | `evalwitness.relation-audit-plan.v2` | exact v2 release, corpus-spec, mutation-program, and construct-audit digests; 32-case balanced primary rule; eight-case jointly feasible pilot rule; reviewer workload; rubric; commit/reveal and unresolved rules; forbidden inputs; 12 reasons; seven axes; eight family contracts; external authorization boundary; digest |
| plan v3 | `evalwitness.relation-audit-plan.v3` | exact v3 corpus-plan/audit/release/program digests; seven core-family contracts; separate exhaustive scarcity-sentinel contract excluded from primary inference; 28-case primary; seven-case pilot; v3 rubric; `not_run`; `not_authorized`; digest |
| case material | `evalwitness.relation-case-material.v1` | plan/corpus/case/unit/replay bindings, coherent redacted task requirement, original/transformed restricted excerpts, exact event omission accounting, shared immutable event-lineage selection under separate hard budgets, paired lineage digest, license/revision/redistribution fields, limitations, external-action boundary, digest |
| case material v2 | `evalwitness.relation-case-material.v2` | v2 plan/release/spec/program/audit, relation-contract, evidence-boundary, exact case-firewall, case/unit/replay, redacted evidence, lineage, license, limitation, authorization, and digest bindings |
| case material v3 | `evalwitness.relation-case-material.v3` | v3 plan/release/corpus-plan/program/audit, relation-contract-v3, evidence-boundary-v3, exact typed firewall, replay, evidence, lineage, license, limitation, authorization, and digest bindings; v2 corpus-spec identity forbidden |
| pilot sample | `evalwitness.relation-pilot-sample.v1` | plan/primary/corpus digests, development role, deterministic selection rule, eight case references, nine unique sources, eight unique task groups, eight unique lineage clusters, zero primary overlap, workload, ten deep bindings, digest |
| pilot sample v2 | `evalwitness.relation-pilot-sample.v2` | v2 plan/primary/release/spec/program/audit digests, development role, one case per family, nine unique sources, eight unique task groups, eight unique lineage clusters, zero primary source/task-group/lineage overlap, workload, ten legacy deep bindings plus full construct-firewall commitment, digest |
| pilot sample v3 | `evalwitness.relation-pilot-sample.v3` | v3 plan/primary/sentinel/release/corpus-plan/program/audit digests, seven core development cases, eight sources, seven task groups, seven lineages, zero primary and sentinel overlap, 14 primary labels, maximum seven tie-break labels, 14 probes, complete typed-firewall bindings, `not_run`, `not_authorized`, digest |
| primary sample | `evalwitness.relation-primary-sample.v1` | plan/corpus digests, exact 31-case selection, 34 sources, 28 task groups, 26 trajectory-pair units, five candidate-order units, family/split/control counts, selection digest, ten deep bindings, digest |
| primary sample v2 | `evalwitness.relation-primary-sample.v2` | v2 plan/release/spec/program/audit digests, exact 32-case selection, 36 sources, 32 unique task groups, 24 lineage clusters, four cases per family, 16 calibration plus 16 test, source-format counts, selection digest, ten legacy deep bindings plus full construct-firewall commitment, digest |
| primary sample v3 | `evalwitness.relation-primary-sample.v3` | exact 28-case seven-family core; four cases/family; 14 calibration plus 14 test; 32 sources; 28 unique task groups and lineages; complete corpus-plan/audit/program/firewall commitments; sentinel sources/tasks/lineages excluded; `not_run`; `not_authorized`; digest |
| scarcity sentinel v3 | `evalwitness.relation-scarcity-sentinel.v3` | all three natural omitted-evidence cases; two development, one calibration, zero test; exhaustive descriptive use; zero primary overlap; no held-out or primary-estimand claim; exact v3 parents; `not_run`; `not_authorized`; digest |
| private mapping | `evalwitness.relation-private-mapping.v1` | packet/material/corpus/case bindings, hidden task group/family/relation/split/control/source/witness/replay identities, visible-to-logical side and evidence mapping, seven exact HMAC domains, private packet/reviewer sort keys, key ID, external-action boundary, digest |
| private mapping v2 | `evalwitness.relation-private-mapping.v2` | protocol-v2 packet/material plus release/spec/program/audit, relation-contract, evidence-boundary, exact case-firewall, hidden identity, randomization, private-key, authorization, and digest bindings |
| private mapping v3 | `evalwitness.relation-private-mapping.v3` | protocol-v3 packet/material plus release/corpus-plan/program/audit, relation-contract-v3, evidence-boundary-v3, typed case-firewall, hidden identity, seven HMAC domains, private key identity, authorization, and digest bindings; v2 spec identity forbidden |
| pair judgment | `evalwitness.relation-pair-judgment.v1`, `evalwitness.relation-pair-judgment.v2`, `evalwitness.relation-pair-judgment.v3` | protocol-matched objective/plan/bundle/assignment/packet/reviewer/qualification/rubric bindings, exact seven visible-axis observations, canonical reasons with directional/ambiguity/insufficiency consistency, submission time, immutable revision number/parent/reason, external-action boundary, digest |
| judgment batch | `evalwitness.relation-judgment-batch.v1`, `evalwitness.relation-judgment-batch.v2`, `evalwitness.relation-judgment-batch.v3` | protocol-matched objective/plan/bundle/assignment/reviewer/qualification/rubric bindings, exact complete latest-revision packet coverage, strict post-submission commitment time, complete status, external-action boundary, digest |
| prereveal ambiguity analysis | `evalwitness.relation-prereveal-ambiguity-analysis.v1`, `evalwitness.relation-prereveal-ambiguity-analysis.v2`, `evalwitness.relation-prereveal-ambiguity-analysis.v3` | protocol-matched two-primary assignment/batch commitments, embedded slot-one/slot-two judgments, packet digests, seven-axis disagreement and rating prevalence, unclear/not-applicable counts and Wilson intervals, reason exact/zero-overlap/Jaccard metrics, exact tie-break packet IDs, not-revealed status, limitations, external-action boundary, digest |
| condition probe | `evalwitness.relation-condition-probe.v1`, `evalwitness.relation-condition-probe.v2`, `evalwitness.relation-condition-probe.v3` | protocol-matched plan/bundle/assignment/judgment-batch/judgment/packet/reviewer bindings, bounded family/direction/source-condition guesses, task recognition and basis, confidence, strictly post-label submission time, external-action boundary, digest |
| condition probe batch | `evalwitness.relation-condition-probe-batch.v1`, `evalwitness.relation-condition-probe-batch.v2`, `evalwitness.relation-condition-probe-batch.v3` | protocol-matched primary assignment/judgment-batch binding, frozen protocol-matched family/direction/control candidate universes plus unknown, complete sorted packet coverage, strict post-submission commitment time, external-action boundary, digest |
| mapping reveal | `evalwitness.relation-mapping-reveal.v1`, `evalwitness.relation-mapping-reveal.v2`, `evalwitness.relation-mapping-reveal.v3` | protocol-matched plan/bundle/prereveal-ambiguity/two-primary/two-probe commitments, optional exact disagreement tie-break assignment/batch, complete mapping references, disclosed assignment seeds that reproduce every HMAC packet order, reveal actor/time after all commitments, external-action boundary, digest |
| relation resolution | `evalwitness.relation-resolution.v1`, `evalwitness.relation-resolution.v2`, `evalwitness.relation-resolution.v3` | protocol-matched plan/bundle/reveal/mapping/packet/case/task-group/family/class/relation/unit/split/control/formal-witness bindings, visible-to-logical direction, two primary judgment digests, optional tie-break judgment, construct caveats, all seven resolved and normalized axes, frozen translation contract/result, human admissibility, verifier-not-consulted boundary, time, digest |
| formal-human comparison | `evalwitness.relation-formal-human-comparison.v1`, `evalwitness.relation-formal-human-comparison.v2`, `evalwitness.relation-formal-human-comparison.v3` | protocol-matched sample/data-role/reveal/ambiguity/primary/tie/probe/resolution commitments, exact support/contradiction/unresolved packet counts, family/class/split/control/task-group denominators, reviewer probe confusion, conservative task-cluster Wilson intervals, unresolved sensitivity, three-layer boundary, time, digest |
| terminal ledger | `evalwitness.relation-terminal-ledger.v1`, `evalwitness.relation-terminal-ledger.v2`, `evalwitness.relation-terminal-ledger.v3` | protocol-matched sample/data-role/reveal/ambiguity/comparison commitments, complete packet entries binding formal witnesses and human resolutions, exact terminal counts/status, probe and optional tie commitments, verifier-not-consulted layer, completion time, external-action boundary, digest |
| pilot readiness | `evalwitness.relation-pilot-readiness.v1` | exact plan/pilot/qualification/handbook/bundle commitments, eight owner-only mapping references and their aggregate commitment, one family-unique structural packet check per pilot case, fixed reviewer/judgment/probe workload, technical-ready and semantic-pending states, required owner action, external-action boundary, limitations, time, digest |
| pilot readiness v2 | `evalwitness.relation-pilot-readiness.v2` | protocol-v2 release/spec/program/audit bindings, digest over all eight packet/firewall pairs, exact v2 pilot/policy/bundle/mappings, workload, technical-ready/semantic-pending states, owner action, authorization, limitations, time, digest |
| pilot readiness v3 | `evalwitness.relation-pilot-readiness.v3` | protocol-v3 release/corpus-plan/program/audit bindings, exact seven-packet typed-firewall commitment, v3 pilot/primary/sentinel/bundle/mappings, 14/7/14 judgment workload, technical-ready/semantic-pending states, owner action, `not_authorized`, limitations, time, digest |
| pilot change receipt | `evalwitness.relation-pilot-change-receipt.v1` | exact readiness/bundle/mapping commitments, eight sorted packet/case/family/relation/unit/task-digest checks, logical/visible content and changed-line digests, exact prefix/suffix and source/retained/omitted denominators, paired-lineage equality, exact candidate reversal, causal-reference and full-context flags, all-line coverage, no raw task/trajectory content, no decision, human-study-not-run and external-action boundaries, limitations, digest |
| pilot change receipt v2 | `evalwitness.relation-pilot-change-receipt.v2` | protocol-v2 release/spec/program/audit and eight-firewall commitment plus the complete v1 structural delta, raw-content exclusion, no-decision, human-not-run, authorization, limitation, and digest contract |
| pilot change receipt v3 | `evalwitness.relation-pilot-change-receipt.v3` | protocol-v3 release/corpus-plan/program/audit and seven-firewall commitment; six trajectory changes plus one candidate-order control; complete structural delta; raw-content exclusion; no decision; `not_run`; `not_authorized`; digest |
| pilot launch dossier | `evalwitness.relation-pilot-launch-dossier.v1` | exact plan/pilot/bundle/readiness/qualification/handbook/mapping commitments, three reviewer slots, 24 qualification responses, 16 primary judgments, at most eight tie-break judgments, 16 post-label probes, 64 maximum total review actions, eight non-public structural packet disclosures, six owner-decision-required governance terms, seven separately unauthorized external actions, not-launchable/inspection-not-completed/human-study-not-run states, limitations, time, digest |
| pilot launch dossier v2 | `evalwitness.relation-pilot-launch-dossier.v2` | protocol-v2 release/spec/program/audit and eight-firewall commitment plus exact v2 prelaunch parents, workload, packet disclosures, unresolved governance, unauthorized actions, not-launchable/human-not-run states, limitations, time, digest |
| pilot launch dossier v3 | `evalwitness.relation-pilot-launch-dossier.v3` | protocol-v3 release/corpus-plan/program/audit and seven-firewall commitment; exact v3 pilot/primary/sentinel/readiness parents; three reviewer slots; 24 qualification responses; 14 primary judgments; maximum seven tie-breaks; 14 probes; 59 maximum actions; seven disclosures; six unresolved governance terms; seven unauthorized actions; not-launchable/inspection-pending/`not_run`; digest |
| pilot inspection | `evalwitness.relation-pilot-inspection.v1` | readiness/plan/pilot/bundle/mapping/handbook commitments, pseudonymous owner inspector, exact eight packet/case/family/unit-bound decisions, task-context/evidence-alignment/transformation-isolation/information/blinding/rubric/redistribution/candidate-order assessments, canonical reasons, derived accepted/revision-required/unresolved counts and overall status, owner-prelaunch-only scope, human-study-not-run state, external-action boundary, limitations, time, digest |
| pilot inspection v2 | `evalwitness.relation-pilot-inspection.v2` | protocol-v2 readiness lineage and owner-semantic-inspection-v2 identity plus the exact eight-decision, derived-status, human-not-run, authorization, limitation, time, and digest contract |
| pilot inspection v3 | `evalwitness.relation-pilot-inspection.v3` | protocol-v3 readiness lineage and owner-semantic-inspection-v3 identity plus exact seven-decision coverage, derived status, `not_run`, `not_authorized`, limitations, time, digest |
| pilot inspection session | `evalwitness.relation-pilot-inspection-session.v1` | immutable protocol-v3 `controlled_relation` session; package-v5 inventory and exact readiness/bundle/mapping/workbook/atlas/sentinel/scarcity SHA-256 bindings; governed plan/primary/pilot/sentinel lineage; pseudonymous inspector; seven sorted packet/family/unit commitments; three sorted scarcity case/material/role/firewall commitments; exact eight core, four scarcity-case, and four scarcity-boundary dimension vocabularies; `not_run`; `not_authorized`; creation time; digest |
| pilot inspection event | `evalwitness.relation-pilot-inspection-event.v1` | session digest; contiguous sequence; prior-event or session digest; closed core-packet/scarcity-case/scarcity-boundary subject kind, exact subject ID, and applicable dimension; explicit passed/failed/indeterminate assessment; optional latest-event supersession; explicit owner-assessment confirmation; strictly increasing time; digest |
| pilot inspection completion | `evalwitness.relation-pilot-inspection-completion.v1` | session/inventory/event-head commitments; exact 66 latest applicable assessments comprising 50 core, 12 scarcity-case, and four scarcity-boundary decisions; seven sorted core reason/disposition summaries; three sorted scarcity assessment/disposition summaries; boundary summary; existing seven-packet v3 inspection-record digest; independently derived core, scarcity, and conservative combined status; explicit owner-completion confirmation; `not_run`; `not_authorized`; completion time; digest |
| public owner-inspection attestation | `evalwitness.relation-owner-inspection-public-attestation.v1` | public package inventory commitment, UTC inspection date, exact 66-assessment and 16-dimension aggregates, core/scarcity/boundary/combined status, closed disclosure and ten-claim boundary, `not_run`, `not_authorized`, digest; excludes inspector/session/event/record/completion/packet/case/mapping/path/task/evidence identities and does not claim public source reproduction |
| qualification answer key | `evalwitness.relation-qualification-answer-key.v1` | qualification-set binding, exact seven-axis observations/reasons/explanations for eight cases, owner-only custody class, external-action boundary, digest; private regular mode-0600 input only |
| qualification answer key v2 | `evalwitness.relation-qualification-answer-key.v2` | protocol-v2 set binding plus exact answer, custody, authorization, and digest contract |
| qualification answer key v3 | `evalwitness.relation-qualification-answer-key.v3` | protocol-v3 set binding plus exact eight-answer, owner-only custody, authorization, and digest contract |
| qualification report | `evalwitness.relation-qualification-report.v1` | set/key/rubric/reviewer binding, exact per-case observation/reason results, score, mandatory-case status, qualification decision/time, external-action boundary, digest |
| qualification report v2 | `evalwitness.relation-qualification-report.v2` | protocol-v2 set/key/rubric/reviewer binding plus exact scoring, time, authorization, and digest contract |
| qualification report v3 | `evalwitness.relation-qualification-report.v3` | protocol-v3 set/key/rubric/reviewer binding plus exact scoring, time, authorization, and digest contract |
| qualification set | `evalwitness.relation-qualification-set.v1` | owner-key-randomized eight-case public competency surface, passing score 0.875, mandatory ambiguity and candidate-order cases, supervised no-lookup rule, withheld-answer rule, no answer/explanation fields, external-action boundary, digest |
| qualification set v2 | `evalwitness.relation-qualification-set.v2` | protocol-v2 plan binding plus the owner-randomized eight-case competency, supervision, withheld-answer, authorization, and digest contract |
| qualification set v3 | `evalwitness.relation-qualification-set.v3` | protocol-v3 public-plan digest and rubric-v3 binding plus the owner-randomized eight-case competency, supervision, withheld-answer, authorization, and digest contract |
| replay receipt | `evalwitness.relation-replay-receipt.v1` | corpus/case/family/unit/source identities, ordered original/transformed trajectory digests, material/manifest/packet/regeneration digests, exact status, external-action boundary, digest |
| replay receipt v2 | `evalwitness.relation-replay-receipt.v2` | protocol-v2 corpus/case/family/unit/source identity plus exact replay, authorization, and digest contract |
| replay receipt v3 | `evalwitness.relation-replay-receipt.v3` | protocol-v3 corpus-plan/audit/release/case/typed-firewall/family/unit/source identities plus exact mutation-v3 replay, authorization, and digest contract |
| review assignment | `evalwitness.relation-review-assignment.v1`, `evalwitness.relation-review-assignment.v2`, `evalwitness.relation-review-assignment.v3` | protocol-matched bundle/set/rubric binding, conflict-free qualified role-matching reviewer and slot, owner-seed digest, HMAC reviewer-specific packet order, full coverage for primary slots one/two or exact preregistered disagreement subset for tie-break slot three, planned-not-shared state, external-action boundary, digest |
| review bundle | `evalwitness.relation-review-bundle.v1` | plan/sample/rubric/qualification/handbook bindings, development-pilot or primary-audit role, restricted visibility, private packet-order protocol, complete mapped packet set, creation time, external-action boundary, digest |
| review bundle v2 | `evalwitness.relation-review-bundle.v2` | protocol-v2 plan/sample/qualification/handbook/packet/mapping bindings plus restricted order, time, authorization, and digest contract |
| review bundle v3 | `evalwitness.relation-review-bundle.v3` | protocol-v3 plan/pilot/qualification/handbook/seven-packet/seven-mapping bindings plus restricted order, time, authorization, and digest contract |
| reviewer handbook | `evalwitness.relation-reviewer-handbook.v1` | plan/qualification/example bindings, frozen purpose, evidence/procedure/axis/rating/reason/applicability/conflict/blinding/submission policies, dataset/labor/privacy/generalization statement, external-action boundary, digest |
| reviewer handbook v2 | `evalwitness.relation-reviewer-handbook.v2` | protocol-v2 plan/qualification/example bindings and 32-case/32-task-group/24-lineage sampling disclosure plus the frozen review, custody, generalization, authorization, and digest contract |
| reviewer handbook v3 | `evalwitness.relation-reviewer-handbook.v3` | protocol-v3 public-plan/qualification/example bindings and 28-case/28-task-group/28-lineage core plus separate scarcity disclosure, frozen review/custody/generalization policy, authorization, digest |
| reviewer kit | `evalwitness.relation-reviewer-kit.v1`, `evalwitness.relation-reviewer-kit.v2`, `evalwitness.relation-reviewer-kit.v3` | protocol-matched bundle/assignment/reviewer/qualification/handbook bindings, exact assigned ordinal packet manifest and bodies, planned-not-shared state, generation time, external-action boundary, digest |
| reviewer record | `evalwitness.relation-reviewer-record.v1`, `evalwitness.relation-reviewer-record.v2`, `evalwitness.relation-reviewer-record.v3` | explicit protocol, objective, pseudonymous alias, role, consent time, independence, authorship, private-contact custody, sorted conflicts, external-action boundary, digest |
| study amendment | `evalwitness.relation-study-amendment.v1` | plan/pilot/primary/corpus digests, pilot and primary workloads, cluster aggregation, missingness/replacement/stopping rules, primary estimand, exact interval design, multiplicity, detection scenarios, claim boundary, empirical status, digest |
| study amendment v2 | `evalwitness.relation-study-amendment.v2` | v2 plan/pilot/primary/release/spec/program/audit digests, eight-case pilot workload, 32-case/32-task-group primary workload, no replacement, unresolved retention, fixed stopping, task-group estimand, exact interval design, multiplicity, detection scenarios, claim boundary, `not_run`, `not_authorized`, digest |
| study amendment v3 | `evalwitness.relation-study-amendment.v3` | v3 plan/pilot/primary/sentinel/release/corpus-plan/program/audit digests, seven-case pilot workload, 28-case/28-task-group primary workload, sentinel excluded, no replacement, unresolved retention, fixed stopping, exact intervals/detection diagnostics, claim boundary, `not_run`, `not_authorized`, digest |
| translation result | `evalwitness.relation-translation-result.v1`, `evalwitness.relation-translation-result.v2`, `evalwitness.relation-translation-result.v3` | protocol-matched plan/family/relation binding, exact normalized observations, matched support/contradiction axes, terminal state, typed reasons, digest |

Pilot launch brief policy: `evalwitness.relation-pilot-launch-brief.v1` is a
deterministic Markdown projection of a validated launch dossier. It contains the
exact workload, structural examples for all eight relation families,
the v3 launch brief instead covers all seven governed core families and states
the separate non-held-out scarcity-sentinel boundary,
privacy/license boundaries, explicitly non-binding governance defaults,
separately unauthorized external actions, scientific non-claims, and an owner
authorization checklist. It contains no packet identity, digest, task text,
trajectory evidence, source identity, private mapping, human result, or launch
authority. Package preparation and independent verification both apply the
public artifact scanner; verification also reproduces the bytes exactly.

Pilot change-atlas policy: `evalwitness.relation-pilot-change-atlas.v1` is a
restricted owner-only navigation artifact. It first reproduces pilot readiness
against the complete restricted bundle and private mappings. Receipt-bound
package format v2 requires `evalwitness.relation-pilot-change-receipt.v1`;
package format v3 requires `evalwitness.relation-pilot-change-receipt.v2`;
package formats v4 and v5 require
`evalwitness.relation-pilot-change-receipt.v3`. The
atlas embeds the exact receipt digest. For each trajectory
pair it binds the complete task requirement and both content digests, counts
exact common rendered prefix and suffix lines, includes every remaining original
and transformed rendered line with bounded context, exposes
retained/source/omitted event counts and logical-to-visible alignment, and flags
an event-reference wrapper moved by causal reordering. For candidate-order
reversal it proves exact content identity and exact reverse source-candidate
order. It never selects an inspection value, infers semantic validity, replaces
the complete workbook, establishes a human result, or authorizes external
action.

Pilot change-receipt policy:
`evalwitness.relation-pilot-change-receipt.v1` contains no raw task requirement
or trajectory content. It MUST bind readiness, bundle, mapping commitment,
packet/mapping/case/family/relation/unit identity, task/content/changed-line and
lineage digests, exact line/event denominators, candidate reversal, and
structural hazard flags. Checks MUST be packet-ID sorted. Historical v1/v2
pilots cover seven trajectory pairs plus one candidate-order control; v3 covers
six trajectory pairs plus one candidate-order control. Coverage MUST be
`all_non_common_rendered_lines_digest_bound`; decision MUST be `not_recorded`;
human study MUST be `not_run`; external action MUST be `not_authorized`.

Relation pilot package format: v1 is the historical workbook inventory; the
pre-receipt Atlas extension remains detectable through its summary fields; v2
requires the change receipt, receipt schema, receipt-bound atlas, and their
summary commitments; v3 additionally requires relation protocol v2, the exact
release/spec/mutation-program/construct-audit lineage, per-case construct-
firewall bindings, version-matched material/packet/mapping/qualification/
handbook/bundle/readiness/receipt/inspection/dossier identities, and the
complete firewall commitment. Verification MUST select the fixed inventory from
the declared format and extension markers, reject unknown or mixed-generation
files, and reproduce every artifact present. Format v4 additionally requires
relation protocol v3, seven pilot materials/packets/mappings, the v3 corpus
plan/audit/release, primary/pilot/amendment, separate three-case sentinel, frozen
public challenge and repair evidence, v3 readiness/receipt/dossier/inspection
schemas, `empirical_state_inheritance=none`, and a content-addressed inventory
over every payload path, byte count, mode, and SHA-256. The current v4 inventory
binds 49 payload files/10,327,928 bytes with digest
`8545c46784685e52575ad0a2dcde23e7ddddf6d562a0dd8e3c53a07dbe701d4d`.
Format v5 additionally requires exactly three ordered restricted v3 case
materials matching every scarcity-sentinel case and construct-firewall digest,
plus a deterministic owner-only scarcity inspection with policy
`evalwitness.relation-scarcity-owner-inspection.v1`. Sentinel materials MUST be
replayed and byte-compared independently; the appendix MUST be regenerated from
the exact plan, primary, sentinel, and materials. It MUST remain outside packet,
bundle, readiness workload, label, held-out, primary-estimand, empirical-result,
and authorization state. The current v5 inventory binds 53 payload files and
10,759,623 bytes with digest
`533deaaecd328d972cdf770073afb0f56e560d4aadea59be1e111d0782eafd80`.
Preparation emits v2 for historical v1 governance, v3 for v2 governance, or v5
for v3 governance and never rewrites prior packages.

Family units: `trajectory_pair` for seven families;
`candidate_pair_orderings` for `candidate_order_reversal`. Pilot selection uses
one lexicographically deterministic non-review development case per family,
solved jointly under unique pilot task group and lineage plus zero primary
source, task-group, and lineage overlap. Primary selection contains all and only
release cases whose manifest has `review.required=true`.

Observation axes: `causal_integrity_preservation`, `evidence_strength`,
`executable_outcome_support`, `information_sufficiency`,
`presentation_equivalence`, `semantic_task_quality`,
`untrusted_content_authority`. Normalized ratings are axis-constrained and
ordered exactly as each family contract declares.

Resolution: validate plan/reveal/commitments/mapping bijection -> preserve all
seven slot-one/slot-two/tie ratings -> information-insufficiency or governed
construct-caveat veto -> primary agreement -> matching tie-break majority ->
otherwise indeterminate -> normalize visible ratings through the private mapping.
Translation: resolve the frozen family contract -> require exact ordered
axis coverage -> match support and
contradiction conditions -> return `unresolved` if an observation is
indeterminate/not-applicable, information is insufficient, complete support and
contradiction both match, or neither terminal rule matches -> otherwise return
`contradicts` when any contradiction condition matches -> otherwise return
`supports` only when every support condition matches -> seal typed reasons and
digest. Outcome labels, verifier scores, relation-test results, evaluator
decisions, source conditions, and desired conclusions are forbidden inputs.

Primary v2 selection: eight governed families x two splits
(`calibration`,`test`) x two cases; global task-group uniqueness; lexicographic
first feasible solution conditional on an eight-family development pilot with
zero source/task-group/lineage overlap and unique pilot task groups/lineages.
Primary aggregation: within each source task group, any contradiction yields
contradicted; otherwise any unresolved yields unresolved; only complete support
yields supported. Fixed sample: 32 cases, 32 effective task groups, 64 required
primary judgments, maximum 32 tie-break judgments, 64 post-label probes, no
replacement, unresolved retained, no sequential efficacy stop. Primary
estimand: task-group contradiction prevalence. One-sided exact binomial 95%
upper bound at zero contradictions: `0.0893681989862648`. Detection probability
at contradiction rates 0.05/0.10/0.20:
`0.8062885155414989`/`0.9656631617970748`/`0.9992077183748573`.
Family/class/split/control results are descriptive; lineage clusters are reported
as dependency diagnostics and do not convert the corpus into a probability
sample.

`scripts/audits/run-relation-construct.sh` validates all 31 v1 schemas, mutual
outcome/relation rejection, three terminal states, outcome-shaped-rating
rejection, complete corpus regeneration, exact plan/pilot/primary/amendment
reproduction, exact trajectory-pair and candidate-pair-order replay and
restricted materialization, both blinded packet types, independent
length-prefixed HMAC reconstruction for packet/task/side/slot/candidate and both
order domains, content-addressed owner-only mode-0600 exclusive mappings,
source/family/key leakage, license/visibility, omission accounting, aligned
selection, exact pair reversal, deep commitments, unit arity, exact structural
readiness for all eight development-pilot packets, deterministic owner-only
change-receipt and receipt-bound all-difference rendering, hidden/visible
inspection rendering, exact inspection-decision reason gates, an
independent record/readiness/bundle/mapping rebinding gate that rejects
incomplete private custody, immutable eight-family package preparation with
distinct keys and SHA-256 inventory, independent package reproduction plus
non-overwrite rejection, and a synthetic pass fixture that remains
human-study-not-run and unauthorized, eight
owner-key-randomized qualification competencies,
exclusive mode-0600 answer custody, mandatory-case scoring, handbook binding,
conflict-free synthetic reviewer fixtures, exact reviewer-specific HMAC
assignment order, complete verified kits, injection-safe deterministic Markdown,
immutable judgment revision lineage, partial-batch and non-postcommit rejection,
two complete synthetic primary batch commitments, prereveal seven-axis/rating/
reason metrics with exact denominators, one independent disagreement-only
synthetic tie-break assignment and kit, commit-before-reveal reconstruction,
post-reveal packet resolution, exact stratified and cluster denominators,
unresolved sensitivity, denominator-deletion rejection, three-layer terminal
ledger custody, independent selection identity,
and zero pilot-primary source/task-group/lineage overlap. No provider invocation
is permitted.

## Coding-Agent-Only Formal Study

The coding-agent-only study is the machine-evaluable completion contract when
human semantic review is outside scope. It is provider-free and binds only the
formal controlled-relation layer of the frozen v3 release.

| field | value |
|---|---|
| schema_version | `evalwitness.agent-only-study.v1` |
| canonical_policy | `evalwitness.agent-only-canonical-json.v1` |
| selection_algorithm | `family-quota-hash-order.v1` |
| selection | 20 calibration and 20 test cases from seven non-sentinel core families; deterministic seed; zero cross-split task-group overlap |
| primary_a | `formal-release-validator-a.v1`; sealed manifest, blind-packet, firewall, relation, witness, and source bindings |
| primary_b | `formal-release-validator-b.v1`; independent canonical digest recomputation and source/outcome binding checks |
| tie_break | `formal-release-tie-break.v1`; executed only on primary disagreement; unresolved is retained |
| provider_calls | exactly `0` |
| human_reviewers | exactly `0` |
| release_artifact | `eval/governance/agent-only-study-v1.json`; schema `eval/governance/agent-only-study-schema-v1.json` |

`Study` contains the exact plan, natural-audit, and release digests; sorted case
records; source records with task, repository, format, split, lineage, source,
trajectory, and outcome-witness digests; both validator results; optional
tie-break result; and derived counts. Its digest omits only the digest field.
Strict decoding rejects unknown fields and trailing JSON. Validation reselects
the cases from the frozen release, reproduces selection metadata, verifies every
parent identity, rederives each source audit, checks primary agreement and
tie-break resolution, and rejects any provider or human-review count above zero.

The supported estimand is formal relation integrity on the selected frozen
release cases. The artifact MUST NOT be used to claim human semantic agreement,
provider quality, transfer, prevalence, or population/generalization beyond
the declared release lineage. The reproducibility gate MUST build the artifact
twice byte-identically, validate it against all three parent artifacts, execute
the independent-validator and disagreement-only tie-break tests, and report
provider calls and human reviewers as zero.

## Outcome Validity and Blinded Adjudication

Outcome states: `solved`, `unsolved`, `indeterminate`, `invalid_task`,
`environment_failed`, `not_adjudicated`.

Evidence kinds:

| kind | source contract |
|---|---|
| claimed_test | asserted trace evidence; never executable ground truth |
| benchmark_reward | immutable benchmark observation with explicit limitation |
| independent_run | trusted-registry execution, independent validator ID, complete execution-log digest |
| formal_relation | independent validator ID and formal witness digest |
| human_label | sealed blinded-label digest and adjudication provenance |

`OutcomeRecord` fields:

| field | type | invariant |
|---|---|---|
| schema_version, canonical_policy | string | exact versioned constants |
| record_id | string | `outcome-` plus canonical digest |
| task_alias | string | nonempty pseudonymous task identity |
| revision | integer | positive; revision one has no parent, later revisions require parent digest |
| parent_digest | SHA-256 or empty | immutable revision lineage |
| evidence | Evidence[] | nonempty, validated, unique, sorted by ID |
| resolution | OutcomeState | typed state |
| resolution_basis | string[] | empty only for `not_adjudicated`; every ID exists in evidence |
| limitations | string[] | unique sorted limitations |
| author_id, revision_reason | string | mandatory revision attribution |
| digest | SHA-256 | canonical content identity |

Decisive resolution requires at least one basis evidence record with the same
state. Indeterminate resolution requires indeterminate basis evidence or at least
two conflicting basis states. Evidence sources remain separate; a later source
cannot overwrite an earlier source.

Governed adjudication plan:

| field | value |
|---|---|
| protocol_version | `evalwitness.outcome-adjudication.v1` |
| plan_digest | `2b7fa309bdf2a4151c640275f858342624fddfbf5d6ada316e5f996ef14dd46e` |
| primary_adjudicators | 2 |
| tie_break_adjudicators | 1 |
| reviewer slots | primaries exactly 1 and 2; tie-break exactly 3 |
| agreement | raw agreement, Cohen kappa, left/right label prevalence |
| uncertainty | deterministic task-cluster percentile bootstrap, 10,000 iterations |
| mutation sample | 31 cases, all eight primary mutation families represented |
| natural target | six cases per eight prespecified strata; no task-group reuse |
| mixed-objective diagnostic | one unique development mutation task group per each of eight families plus natural rank one from each populated stratum; size 14; validation-only and prohibited from reviewer launch |
| diagnostic workload | two primary reviewers: 28 hypothetical labels and 28 post-label source probes; one tie-break reviewer: at most 14 labels |
| outcome pilot v2 | natural rank one from each populated stratum only; objective `single_trajectory_outcome`; size 6; no mutation, pair, pair-order, or expected-relation unit |
| outcome pilot workload | two primary reviewers: 12 labels and 12 post-label source probes; one tie-break reviewer: at most 6 labels |
| replacement | next eligible frozen hash in the same stratum only |
| sensitivity | original and adjudicated outcomes reported separately |
| recruitment | explicit consent required |

Natural inventory request fields: schema/version, development-only source class,
sorted bounded legacy-result references, baseline rule, high-confidence-error
threshold, and canonical digest. Legacy adapter validation requires exact task
alignment, unique tasks, binary reward vectors, valid selection indices, selected
reward consistency, bounded regular JSON, and source SHA-256. Unknown legacy
top-level fields are outside the adapter contract; every consumed decision field
is validated.

Natural strata: `verifier_correct`, `verifier_wrong`,
`verifier_judge_disagreement`, `baseline_disagreement`,
`high_confidence_error`, `abstention`, `provider_failure`, `random_control`.
Selection orders strata by ascending eligible count, then frozen name; each
stratum orders candidates by plan-bound SHA-256; a task group can enter once
globally. Missing strata remain exact shortfalls; no synthetic or cross-stratum
replacement is admissible. Current governed inventory status is `incomplete`,
36/48, with six-case shortfalls for `abstention` and `provider_failure`.

`evalwitness.outcome-pilot-sample.v1` binds the plan, governed mutation sample,
complete mutation release, natural inventory, development role, deterministic
selection rule, eight required mutation families, selected and unavailable
natural strata, reviewer workload, canonically sorted source-case/task-group/
selection-evidence references, selection digest, limitations, and artifact digest. The
current governed digest is
`76436e72015a55be5bcaa6c83ea1b6fdc053fb01004ca940b4d663d882a6beb0`.
It is a mixed-objective diagnostic, not a launchable human-review sample: natural
cases require single-trajectory outcome labels, while mutations require pairwise
relation judgments. Its builder remains reproducible as `pilot-sample-v1`; its
readiness builder always rejects launch.

`evalwitness.outcome-pilot-sample.v2` binds only the plan and natural inventory,
objective `single_trajectory_outcome`, development role, exact rank-one rule,
six selected and two unavailable strata, exact 2+1 workload, six canonically
sorted objective-typed case/task-group references, selection digest,
limitations, and artifact digest. The governed digest is
`994dbbfb18c40494d1b664f4a6b45ac5b4182dcbd7f7f9293ba1f3e8eea03ff7`.
TASK 068 separately owns relation review.

Outcome pilot materialization requires repository root, governed plan/request/
inventory/v2 sample, a caller-owned 32-byte HMAC key and key ID, and exactly a
16,000-token evidence budget. It revalidates legacy source artifact digests and
task counts; replays sorted SWE-bench run order or sorted Terminal-Bench trial
order; checks the exact selected task, index, binary reward, and raw candidate;
strictly ingests and secret-redacts the raw item; applies anchored whole-event
evidence selection; and emits exactly `task_requirement` plus
`trajectory_evidence`. Selector `evalwitness.outcome-evidence-selector.v2`
requires the final nonempty patch, or the last nonempty assistant narrative when
no patch exists, plus action and result coverage. A required anchor that cannot
fit the hard budget fails materialization.
Exact model/run/trial identities are redacted; raw `source` and `call_id`
references are replaced by deterministic typed SHA-256 pseudonyms before packet
sealing. Every real packet is `restricted_reference_only` and
`public_releasable=false`.

`evalwitness.outcome-pilot-source-binding.v2` is owner-only:

| field | invariant |
|---|---|
| pilot_sample_digest, natural_inventory_digest | exact governed v2 lineage |
| objective, stratum, suite, task_id | one natural single-trajectory review unit |
| selected_index, selected_reward | exact raw candidate and binary legacy outcome |
| source_location, source_revision | private reproducibility locator |
| source_digest, trajectory_digest, retained_digest | raw, canonical, and sliced identity |
| source_events, retained_events, redaction_hits, evidence_budget_tokens | complete retention accounting; budget exactly 16,000 |
| evidence_selector, decision_anchor_kind, decision_anchor_digest | exact selector v2 and retained final-patch or assistant-narrative anchor |
| retained_messages, retained_actions, retained_results | structural evidence coverage; actions and results are positive |
| license_spdx, source_url, redistribution | source license metadata; redistribution exactly `reference_only` |
| packet_id, mapping_digest, digest | exact HMAC packet/mapping binding and canonical identity |

`evalwitness.outcome-pilot-private-materials.v1` is the sealed owner-only custody root:

| field | invariant |
|---|---|
| pilot_sample_digest | exact governed v2 pilot lineage |
| mappings, source_bindings | one aligned, canonical packet-ordered private mapping and source binding per selected case |
| mappings_digest, source_bindings_digest | exact commitments to both private collections |
| digest | canonical identity over the full private custody artifact |

The CLI publishes this artifact exactly once with mode `0600` below the supplied
owner-only root. It is never emitted on the public review-item stream.

`evalwitness.outcome-pilot-inspection.v1` is restricted metadata:

| field | invariant |
|---|---|
| pilot_sample_digest, bundle_digest, private_materials_digest, objective | exact natural outcome pilot, restricted bundle, and sealed private custody root |
| binding_references, source_binding_commitment_digest | one sorted packet/source-binding digest pair per selected case |
| mapping_commitment_digest | exact owner-only mapping-reference commitment used by readiness |
| items | one sorted row per packet with task group, source-binding digest, decision anchor, source/retained/omitted events, retention ratio, message/action/result counts, hard budget, rendered bytes/tokens, trajectory-evidence digest, and structural status |
| total_source_events, total_retained_events, total_omitted_events | exact additive event denominators |
| minimum_retention_ratio | minimum retained/source event ratio across all packets |
| packets_with_messages, packets_without_messages | complete message-coverage partition |
| patch_anchors, narrative_anchors | complete decision-anchor partition |
| reviewability_status | exactly `structurally_ready` after all row and aggregate invariants pass |
| semantic_status | exactly `requires_human_pilot`; structural inspection cannot assert semantic sufficiency |
| limitations, digest | frozen claim boundary and canonical identity |

`BlindBuildRequest` contains the plan digest, task alias, evidence, sorted rubric
questions, privacy/release class, source-case digest, private condition, expected
relation, original slot mappings, blinding-key ID, and sorted forbidden values.
Packet creation requires a 32-byte key decoded from exactly 64 lowercase hex
characters. HMAC-SHA-256 with distinct domain separators derives
`packet-<sha256>`, `taskref-<sha256>`, and `slot-<sha256>` identities. Public packet validation
requires opaque identities, content digests, licenses, limitations, sorted slots,
and sorted rubric questions. Leakage validation rejects all declared forbidden
values plus original task alias, condition, expected relation, key ID,
source-case digest, and original slot labels after case-folding. Private mapping
contains the original task alias and reverse slot mapping and
is published once with mode `0600` below an owner-only root; overwrite is
forbidden. Public packet output and private mapping output never share a stream.

Blinded label fields: opaque packet ID, adjudicator alias, reviewer slot, primary
outcome, five axis ratings, sorted typed reason codes, RFC3339 submission time,
rubric version, qualification-report digest, sorted conflicts of interest, label
ID, and digest. Rating values: `sufficient`, `insufficient`, `unclear`,
`not_applicable`. Reason codes: `claimed_only`, `environment_failure`,
`evidence_conflict`, `evidence_consistent`, `evidence_insufficient`,
`formal_relation_supports`, `harmful_side_effect`, `independent_tests_fail`,
`independent_tests_pass`, `invalid_task`, `task_satisfied`, `task_unsatisfied`,
`technical_defect`, `verification_complete`, `verification_incomplete`.

Qualification set: exactly governed public synthetic packets, rubric version,
passing score 0.80, expected outcomes, required reason codes, explanations, and
canonical digest. Qualification report: one adjudicator, one unique slot-one
label per case, rubric version, case-level outcome/reason correctness, counts,
exact score, threshold, qualified flag, latest-label qualification time, and
canonical digest. Study labels bind the passing
report digest and matching adjudicator alias. Resolution requires two passing
reviewer-specific reports; tie-break label and passing report are both present or
both absent. Primary labels require distinct aliases and exact slots one and two.
Agreement emits unresolved indeterminate when primaries disagree without a
tie-break. Original labels remain immutable.

Reviewer workflow artifacts:

| artifact | required invariant |
|---|---|
| `evalwitness.outcome-pilot-sample.v1` | exact development-only mutation/natural diagnostic inputs, one case per committed family or populated stratum, no repeated task group or source case, explicit unavailable strata, hypothetical 2+1 workload, frozen limitations, digest, and mandatory non-launchable status |
| `evalwitness.outcome-pilot-readiness.v1` | historical mixed-objective validation schema only; production builder always rejects launch |
| `evalwitness.outcome-pilot-sample.v2` | six natural single-trajectory outcome units, explicit objective, populated/unavailable strata, unique task groups/cases, exact 2+1 workload, frozen limitations, and digest |
| `evalwitness.outcome-pilot-source-binding.v2` | owner-only exact raw selection, reward, source/revision/license, anchored selector and coverage counts, redaction/retention, packet/mapping, and canonical digest binding |
| `evalwitness.outcome-pilot-private-materials.v1` | owner-only pilot lineage, packet-ordered mappings/source bindings, independent collection commitments, and full canonical digest; exclusive mode-0600 publication |
| `evalwitness.outcome-pilot-inspection.v1` | pilot/bundle/objective, exact mapping and source-binding commitments, per-packet anchor/retention/message/action/result/byte/token rows, complete aggregates, structural-ready status, mandatory semantic-human-pilot status, limitations, and digest |
| `evalwitness.outcome-pilot-readiness.v3` | exact v2 sample/plan, restricted development bundle, qualification, handbook, pre-assignment protocol, sealed inspection, six source-case/stratum/task-group mappings, two exact outcome evidence kinds, mapping commitment, timestamps, workload, technical-ready state, external-action-not-authorized state, limitations, and digest |
| `evalwitness.outcome-reviewer-handbook.v1` | exact v1 purpose, evidence rules, decision procedure, outcome/axis/reason definitions, conflict/blinding policy, checklist, dataset statement, qualification-set binding, example packet, and digest |
| `evalwitness.outcome-review-bundle.v1` | plan, rubric, qualification-set, exact handbook version/digest, data-role, visibility, creation time, unique sorted blinded packets, and task-cluster IDs are content-bound; public bundles contain only public-releasable packets |
| `evalwitness.outcome-reviewer-record.v1` | pseudonymous alias, primary/tie-break role, consent time, independence, authorship-policy acceptance, private contact custody, sorted conflicts, and digest |
| `evalwitness.outcome-review-assignment.v1` | bundle, reviewer, passing reviewer-specific qualification, rubric, exact slot/purpose, HMAC ordering protocol, secret-seed digest, assignment time, ordered packet IDs, and digest |
| `evalwitness.outcome-reviewer-kit.v1` | bundle/plan/data-role/visibility, exact embedded handbook and assignment, generation time, assigned blinded packets in committed order, and digest; no mapping, seed, evaluator result, peer label, condition, or expected relation |
| `evalwitness.outcome-label-batch.v1` | assignment, reviewer, slot, qualification, complete unique label coverage, commitment time, and digest |
| `evalwitness.outcome-blinding-protocol.v1` | bundle, frozen condition candidate universe, explicit unknown token, fixed prompt/metrics, pre-assignment creation time, and digest |
| `evalwitness.outcome-blinding-probe.v1` | protocol, assignment, committed label batch and packet label, primary reviewer/slot, source-condition guess or unknown, confidence, recognition declaration/basis, post-label submission time, and digest |
| `evalwitness.outcome-blinding-probe-batch.v1` | embedded protocol, assignment, label batch, exact packet coverage, strictly post-label commitment, probe commitment time, and digest |
| `evalwitness.outcome-rubric-ambiguity-analysis.v1` | bundle and both primary commitments, exact embedded slot-one/slot-two labels, packet/task-group bindings, primary-outcome and five-axis disagreement, left/right rating prevalence, unclear/indeterminate rates, reason-code exact/zero-overlap rates and Jaccard divergence, Wilson intervals for every binary proportion, prereveal analysis time, limitations, and digest |
| `evalwitness.outcome-mapping-reveal.v1` | paired primary assignment/batch commitments, optional paired tie-break, reproducible disclosed ordering seeds, complete packet/mapping references, reveal actor/time, and digest |
| `evalwitness.outcome-blinding-analysis.v1` | bundle/reveal/probe commitments, complete item truth and guesses, denominators, Wilson intervals, coverage, selective accuracy, marginal chance agreement, Cohen kappa, confidence/Brier error, recognition, condition/reviewer breakdowns, limitations, analysis time, and digest |
| `evalwitness.outcome-adjudication-ledger.v1` | bundle, all commitments, prereveal rubric-ambiguity analysis, reveal, post-reveal semantic-blinding analysis, agreement, resolutions, unresolved packet set, complete/unresolved status, completion time, and digest |
| `evalwitness.outcome-source-audit.v1` | terminal ledger/reveal/bundle bindings, data role and visibility, exact source evidence and human resolutions, five source summaries, within-source ambiguity, all ten source-pair agreement views, Wilson intervals, task-cluster Cohen kappa, benchmark transitions, limitations, analysis time, and digest |

Review sequence: freeze the pilot sample -> prepare every blinded packet and
owner-only mapping -> freeze the development bundle and blinding protocol ->
seal a readiness artifact that remains externally unauthorized -> obtain
explicit owner authorization -> seal reviewer consent/independence -> require
passing reviewer-bound qualification and zero declared conflicts -> assign every
bundle packet independently to slots one and two under reviewer-specific secret
HMAC ordering -> emit, bundle-verify, and deterministically render one
self-contained reviewer kit per assignment -> accept exactly one post-assignment
conflict-free rubric-matching label per assigned packet -> commit both complete
primary batches -> analyze exact committed-label outcome, axis, rating, unclear,
indeterminate, and reason-code divergence strictly before reveal -> commit a
complete post-label source-condition probe under the
pre-assignment frozen candidate universe for each primary reviewer -> assign only
primary disagreements to an independent slot-three reviewer -> commit the exact
tie-break batch -> disclose ordering seeds and mapping digests strictly after the
last required commitment -> replay every assignment order -> score committed
source guesses with complete denominators, Wilson uncertainty, chance correction,
confidence error, and recognition disclosure -> compute primary agreement and
packet resolutions -> seal both analysis digests into the final ledger. When no tie-break is
provided, each primary disagreement remains `indeterminate` and the ledger status
is `unresolved`. Missing coverage, reviewer reuse, wrong slot, rubric drift,
pre-assignment labels, pre-label or partial probes, probes committed after reveal,
early tie-break, early reveal, wrong seed, incomplete mapping, inconsistent
leakage statistics, missing analysis, or contradictory ledger status rejects.

`ExecutableLog` binds task alias, trusted validator contract, disposable
environment revision, command metadata, start/end time, exit code, bounded output
digest/size, infrastructure failure class/detail digest, outcome, limitation,
and canonical digest. Allowed infrastructure failures: `timeout`,
`output_limit`, `execution_error`, `cleanup_error`. Passing exit maps to solved;
normal nonpassing exit maps to unsolved; infrastructure failure maps to
environment failed. Executables, arguments, environment, limits, passing code,
working root, and network-disabled status originate only from the trusted
registry and operator-supplied disposable environment, never trajectory content.

Agreement pairs require opaque packet IDs and task-group IDs. Raw agreement and
reviewer prevalence are always emitted. Cohen kappa is accompanied by
`cohen_kappa_defined`; expected-agreement-one cases remain undefined. Bootstrap
sampling draws task groups with replacement and retains every pair in each drawn
cluster; report includes interval bounds and defined-kappa replicate count.

`OutcomePreservation` binds source/intervened outcome digests and states,
mechanism, evaluator-blind flag, admissibility, sorted reasons, and digest.
Admissibility requires equal task lineage and equal decisive solved/unsolved
states. State change, nondecisive state, or lineage mismatch is inadmissible.

## Experiment Capsules and Claim Ledger

Core identities:
| object | schema or policy |
|---|---|
| manifest | `evalwitness.capsule-manifest.v1` |
| registry | `evalwitness.capsule-registry.v1` |
| canonical JSON | `evalwitness.capsule-canonical-json.v1` |
| digest | lowercase SHA-256 |
| verification report | `evalwitness.capsule-verification.v1` |
| archive report | `evalwitness.capsule-archive.v1` |
| claim ledger | `evalwitness.claim-ledger.v1` |
| claim verifier | `evalwitness.claim-verifier.v1` |
| claim verification | `evalwitness.claim-verification.v1` |
| challenge receipt | `evalwitness.claim-challenge-receipt.v1` |
| challenge pack | `evalwitness.claim-challenge-pack.v1` |
| projection | `evalwitness.claim-projection.v1` |
| Claim Autopsy | `evalwitness.claim-autopsy.v1` |
| claim surface | `evalwitness.claim-surface.v1` |

Component registry descriptor:
| field | type | constraint |
|---|---|---|
| type_id, schema_id | string | stable closed identities |
| role | enum | `observation`, `derivation`, `governance`, `attestation`, `commitment`, `presentation` |
| allowed_visibilities | enum[] | sorted subset of `public`, `restricted`, `private` |
| media_type | string | exact registered media type |
| payload_profile | enum | `evalwitness.payload-exact-bytes.v1`, `evalwitness.payload-canonical-json.v1` |
| validator_id | string | required trusted validator |
| binding_validator_id | string | optional trusted cross-parent validator |
| parent_rules | ParentRule[] | sorted closed edge/type/cardinality/resolution rules |

Registry fields: `schema_version`, `canonical_policy`, `registry_id`, optional
`base_registry_digest`, sorted `types`, `digest`. Extension registry requires the
exact base digest and may add types and trusted validators without changing a
base descriptor.

Parent rules and references:
| field | values or constraint |
|---|---|
| kind | `derived_from`, `observed_from`, `governed_by`, `attests`, `commits_to`, `redacts`, `renders`, `supersedes` |
| parent_type | registered type identity |
| minimum, maximum | closed cardinality; maximum is -1 for unbounded or >= minimum |
| resolutions | sorted subset of `internal`, `external`, `omitted` |
| component_id | exact component SHA-256 identity |
| role, visibility | exact parent metadata |
| capsule_id | required for resolved external family parent; absent for internal parent |
| omission_class | required only for `omitted` |

Role invariants: `derived_from` child is derivation or governance and parent is
not presentation; `observed_from` joins observations; `governed_by` parent is
governance and child is neither governance nor presentation; `attests` child is
attestation and parent is observation, derivation, or commitment; `commits_to`
child is commitment; `redacts` child is derivation or commitment and parent is
observation, derivation, or attestation; `renders` child is presentation and
parent is non-presentation; `supersedes` preserves role and type. Only
`redacts` and `commits_to` may cross from a more private parent or omit parent
bytes. Other edges cannot expose a more private parent.

Component record:
| field | constraint |
|---|---|
| component_id | SHA-256 over the complete canonical component record with `component_id` empty, including name and sorted parents |
| name | unique stable name within capsule |
| type_id, schema_id | exact registry descriptor values |
| role, visibility, media_type | allowed registry values |
| payload | algorithm=`sha256`, digest, positive byte count, exact canonicalization profile |
| parents | sorted references satisfying exactly one registered rule each |

Manifest fields: `schema_version`, `canonical_policy`, `capsule_id`,
`manifest_digest`, `registry_digest`, `study_id`, `cell_id`, sorted
`parent_capsules`, sorted non-presentation `scientific_roots`, sorted
presentation `presentation_roots`, sorted `components`. Every component is
reachable from a declared root; internal edges are acyclic; names and component
IDs are unique.

Scientific identity: SHA-256 canonical digest over manifest schema and policy,
registry digest, study, cell, parent capsules, scientific roots, and every
non-presentation component. Manifest identity: SHA-256 canonical digest over the
complete manifest with `manifest_digest` empty. Presentation components and
roots affect manifest identity but not capsule scientific identity.

Directory representation:
| path | content |
|---|---|
| `capsule.json` | canonical manifest |
| `registry.json` | canonical closed registry |
| `checksums.txt` | sorted SHA-256 and relative path for manifest, registry, and payload files |
| `components/sha256/{digest}` | exact payload bytes |

Directory verification rejects symlinks, unsupported types, extra files,
undeclared directories, missing payloads, byte-count or digest mismatch,
non-canonical JSON, registry mismatch, graph violations, binding violations, and
visibility above the caller maximum. Default limits: 100000 components; 64 MiB
manifest; 16 MiB registry; 512 MiB per payload; 8 GiB total payload bytes.

Archive representation: gzip best compression; gzip time Unix epoch and OS=255;
USTAR; sorted three-directory inventory; sorted files; Unix epoch mtimes; explicit
public or owner-only modes; root `evalwitness-capsule-{capsule_id}`. Archive
creation verifies the source directory before writing, publishes without
overwrite, syncs file and parent, re-inspects the complete archive, and reports
SHA-256, bytes, file count, and `deterministic=true`.

Source-tree provenance `evalwitness.provenance.source-tree.v1`: algorithm
`evalwitness.git-visible-source-tree.v1`; exact Git top level and revision;
porcelain-status digest; dirty state; sorted tracked plus non-ignored untracked
entries; regular-file or symlink kind; Git mode; present, untracked, or deleted
state; bytes and SHA-256; canonical digest.

Build provenance `evalwitness.provenance.build.v2`: product version; artifact
kind; external binary digest and bytes; `binary_included=false`; commit; dirty
state; source-tree digest; Go version; main package; target OS/architecture;
sorted recorded build settings; sorted dependency manifest and digest; embedded
VCS revision and modified state; source match state; reproducibility=
`digest-bound-external-binary`; canonical digest. Product version is canonical
SemVer from `internal/product/version.go`. Available embedded revision must equal
source commit. Available embedded modified state must equal source dirty state.
The immutable v1 decoder accepts only records without product version; v2
requires it.

Capsule family verification independently verifies every parent and child,
requires the exact declared parent set, resolves each external parent component
and metadata, rejects duplicate parents, and resolves an extension base registry
to exactly one parent.

Reference packages:
| package | components | scientific | presentation | archive files | maximum visibility |
|---|---:|---:|---:|---:|---|
| public reference | 71 | 69 | 2 | 74 | public |
| private relation child | 64 | 64 | 0 | 67 | private |

Private relation child binds the public parent capsule, package-format-v5
inventory, 53 package files, immutable session, 66-event chain, inspection
record, completion receipt, public source commitments, and independently
recomputed private proof. Public reference contains only the closed
`evalwitness.relation-owner-inspection-public-attestation.v1`, private-parent
verification state, and declared omission. Public source reproduction remains
unavailable.

The public reference additionally contains
`evalwitness.legacy-cache-census.v1` as a commitment. Required values:
`total_files=17898`, `total_bytes=71103852`, `response_files=17876`,
`response_bytes=71094038`, `response_schema_version=1`,
`capability_files=7`, `capability_bytes=2147`, `operational_files=15`,
`operational_bytes=7667`, `operational_content_read=false`,
`exact_admissible_entries=0`, `read_only=true`, `provider_calls=0`, and
`sensitive_content_emitted=false`. Provider rows are sorted and partition all
responses. `published_namespace` is a provider-scoped upper bound with
`exact_request_map=false`. Scientific and operational inventory digests are
separate. The scientific digest commits to response and capability bytes; the
operational digest commits only path and size metadata. Neither the census nor a
public inventory list emits any payload bytes.

In-toto representation:
| field | value or binding |
|---|---|
| `_type` | `https://in-toto.io/Statement/v1` |
| subject name | `evalwitness-scientific-capsule/{capsule_id}` |
| subject digest | `sha256={capsule_id}` |
| predicateType | `https://evalwitness.dev/attestation/capsule/v1` |
| predicate schema | `evalwitness.in-toto-capsule-predicate.v1` |
| predicate bindings | capsule, manifest, registry, study, cell, scientific roots, scientific/presentation counts, visibility counts |

DSSE payload type: `application/vnd.in-toto+json`. Signature input: DSSE
pre-authentication encoding over exact payload type and canonical Statement
bytes. Trust root: `evalwitness.dsse-trusted-keys.v1`, sorted Ed25519 public keys,
key IDs derived from public keys, canonical root digest. Policy: sorted allowed
key IDs and threshold. Verification output:
`evalwitness.dsse-verification.v1` with capsule, payload, trust-root, trust-mode,
verified-key, and validity bindings.

Sigstore explicit-key bundle: media type
`application/vnd.dev.sigstore.bundle.v0.3+json`; exactly one DSSE signature; one
`verificationMaterial.publicKey.hint` equal to signature key ID; out-of-band
Ed25519 trust root required. X.509 certificate chains, certificate fields,
transparency-log entries, timestamp material, and unknown verification fields
reject in explicit-key mode.

Source archive report `evalwitness.source-archive-report.v1`:
| field | constraint |
|---|---|
| product, version, git_commit, source_tree_digest | `evalwitness`; canonical product SemVer; exact lowercase current HEAD commit; exact clean source-tree provenance digest |
| archive_root, format | `evalwitness-{version}`; `ustar+gzip` |
| sha256, bytes, expanded_bytes | exact bounded published archive identity and expanded regular-file bytes |
| files, directories | exact positive archive inventory counts including the root directory |
| deterministic | `true` |

Source archive algorithm: require canonical Git top level -> require clean
current HEAD including untracked files -> enumerate sorted `git ls-tree` regular
blobs with modes `100644` or `100755` -> reject links, gitlinks, special modes,
unsafe paths, control characters, and non-ASCII paths -> derive sorted exact
directory inventory -> read every exact committed blob -> write root, directory
block, and file block as strict USTAR with Unix-epoch times, zero owner IDs,
empty owner names, and modes `0755`, `0644`, or `0755` -> gzip best compression
with zero timestamp and OS 255 -> sync temporary file -> hostile-input archive
inspection -> publish by no-overwrite hard link -> sync parent -> verify digest
and report. Manifest verification repeats safety inspection, strict USTAR header
validation, directory-closure validation, count binding, size binding, and digest
binding. PAX, GNU, extended headers, links, duplicate paths, case-fold collisions,
extra directories, trailing streams, source-index drift, and concurrent
source/archive mutation reject.

Portable source-tree provenance: exact canonical
`evalwitness.provenance.source-tree.v1` at
`source/source-tree-provenance.json`; dirty=false; status digest=SHA-256(empty);
every entry state=present, kind=file, mode=`100644|100755`; commit, file count,
expanded bytes, and digest equal the source-archive report. Opt-in loader requires
both `EVALWITNESS_REPOSITORY_ROOT=ABSOLUTE_ROOT` and
`EVALWITNESS_SOURCE_TREE_PROVENANCE=ABSOLUTE_PATH`. Loader algorithm: require the
requested repository to equal the declared root -> read one
stable bounded non-link canonical document -> validate provenance digest -> walk
the extracted source without following links -> ignore only `.git` metadata ->
require every regular file exactly once -> recompute each byte count and SHA-256
-> reject undeclared, missing, changed, linked, or special files -> return the
signed clean commit identity. Nonmatching nested fixture repositories use their
own Git provenance. An ephemeral synthetic `.git` repository may serve
worktree-only tests but never supplies capsule, build, or release provenance.

Release manifest `evalwitness.release-manifest.v1`:
| field | constraint |
|---|---|
| product, product_version | `evalwitness`; canonical SemVer from `internal/product/version.go` |
| git_commit | lowercase 40- or 64-character Git object ID |
| source_archive_sha256 | lowercase SHA-256; must equal the exact `source/evalwitness-{product_version}-source.tar.gz` manifest asset |
| created_at | canonical UTC RFC3339 commit time |
| assets | sorted unique regular files; path, role, positive bounded bytes, lowercase SHA-256 |
| roles | `binary`, `capsule`, `documentation`, `evidence`, `protocol`, `replication`, `source`; every role present |
| binaries | darwin amd64/arm64, linux amd64/arm64, windows amd64; no additional binary |
| offline module graph | `source/go-proxy/index.json` plus exact file-proxy assets for the complete `go mod download all` graph |
| source archive report | canonical `evidence/source-archive-report.json`; exact commit, source-tree digest, version, USTAR+gzip format, archive digest, compressed/expanded bytes, files, and directories |
| portable source tree | canonical `source/source-tree-provenance.json`; clean commit and exact file identity equal the archive report |
| truth | provider_calls=0; empirical_study=not_run; human_study=not_run; independent_replication=not_run; external_publication is `not_authorized` locally or `authorized_by_tag` only when exact tag `v{product_version}` resolves to `git_commit` |

Release SPDX: SPDX-2.3; CC0-1.0 document data license; root package MIT;
dependency and binary license conclusions `NOASSERTION`; package URLs and module
versions from embedded Go build information; every binary requires
`-trimpath=true`, `CGO_ENABLED=0`, filename-matched GOOS/GOARCH, and no embedded
VCS settings. Commit identity is bound by the manifest and Statement, while the
absence of repository-only VCS bytes permits exact archive-only rebuilding.

Release round trip `evalwitness.release-roundtrip-verification.v1`: safe
extraction of the exact source archive; byte verification through manifest-bound portable
source-tree provenance; ephemeral synthetic Git harness excluded from provenance;
empty Go module/build caches;
`GOTOOLCHAIN=local`, `GOPROXY=file://MANIFEST_ASSET`, `GOSUMDB=off`,
`-mod=readonly`; Go tests with explicit status for the separately distributed
trajectory cache, post-test source-tree cleanliness, and curated public-source scan;
byte-identical host binary rebuild; full
release, capsule, claim, challenge-pack, claim-surface, Claim Autopsy, and
explorer reproduction; provider_calls=0; valid=true. It proves the declared
host/toolchain path, not an independent actor or every supported target.

Release in-toto Statement:
| field | value or binding |
|---|---|
| `_type` | `https://in-toto.io/Statement/v1` |
| subjects | byte-exact published `release-manifest.json` and `evalwitness.spdx.json` SHA-256 |
| predicateType | `https://evalwitness.dev/attestation/release/v1` |
| predicate schema | `evalwitness.in-toto-release-predicate.v1` |
| predicate bindings | product, version, commit, source archive, creation time, manifest, SBOM, asset count, total bytes, truth state |

Release signature directory: exact files `envelope.dsse.json`, `policy.json`,
`trust-root.json`; one canonical DSSE signature over the exact release Statement;
existing mode-`0600` 64-byte Ed25519 private key encoded as 128 lowercase hex
characters with optional final newline; key seed and public bytes consistent;
private key never copied. Unsigned verification requires
`--allow-unsigned-development`; signed verification requires the complete exact
directory.

Claim expression:
| field | constraint |
|---|---|
| operation | `component_exists`, `pointer_equals`, `count`, `sum`, `ratio`, `difference`, `all_equal` |
| operands | ordered declared component name, type ID, JSON pointer tuples |
| expected | exact boolean, string, rational number, or null |
| result | exact typed value; no floating-point comparison |

Claim fields: stable `CLM-NNN`; text template; status `supported`, `exploratory`,
`unsupported`, `superseded`, or `withdrawn`; E0-E6 level; closed scope;
expression; uncertainty; multiplicity; exact capsule IDs and evidence component
names; sorted caveats; evidence ceiling; attestation state `not_required`,
`current`, `stale`, or `unavailable`; evidence/current generation; guarding test;
eight ordered challenge declarations; append-only history.

Assertable status is supported or exploratory. Assertable claims require E1 or
higher, a non-`component_exists` expression, evidence generation equal to current
generation, attestation `not_required` or `current`, exact text equality with the
strongest evidence-ceiling statement, all required caveats, and at least one
applicable executable challenge. Superseded claims require history and a
non-current evidence generation. Current projection contains assertable claims;
historical projection contains unsupported, superseded, and withdrawn claims.

Challenge classes: `caveat-removal`, `denominator-deletion`,
`digest-substitution`, `parent-removal`, `scope-widening`, `stale-attestation`,
`superseded-generation`, `visibility-leak`. Every claim declares each class as
applied or inapplicable with the verifier-derived typed reason. Applied mutation
is registry-owned and never artifact-supplied executable code.

Challenge receipt binds verifier version, claim, source capsule, manifest,
ledger, challenge, class, exact mutation, expected and observed guard,
before/after state, equal before/after sealed-source digest, pass state, and
canonical digest. Pass requires expected guard equals observed guard and sealed
source identity is unchanged.

Tracked claim surfaces: `documentation`, `findings`, `readme`, `release`,
`result`, `skill`. `documentation` projects CLM-001 through CLM-034. `readme`
projects CLM-001, CLM-002, CLM-003, CLM-031, and CLM-032 as the public
verification card. Markdown surfaces use one exact begin/end marker pair. JSON is
canonical `evalwitness.claim-surface.v1`. Verification rebuilds all six views
from the verified capsule and ledger and requires byte equality.

## Offline Evidence Explorer

| artifact | schema | contract |
|---|---|---|
| report | `evalwitness.evidence-explorer-report.v2` | canonical verified view over one public capsule, claim ledger, derived Claim Autopsy, challenge pack, repository-validated stress development witness, bound release files, and optional independently verified evidence families |
| canonical policy | `evalwitness.evidence-explorer-canonical-json.v1` | sorted canonical JSON with SHA-256 digest over the complete report while `digest` is the empty string |
| HTML template | `evalwitness.evidence-explorer-html.v1` | deterministic self-contained document with no runtime network or backend |
| render metadata | `evalwitness.evidence-explorer-render-metadata.v1` | bytes, HTML SHA-256, renderer digest, report digest, and embedded report-payload SHA-256 |
| embedded asset manifest | `evalwitness.evidence-explorer-assets.v1` | exact CSS and JavaScript path, byte count, and SHA-256 |
| public presentation manifest | `evalwitness.evidence-explorer-public-assets.v2` | source HTML, report, renderer, reference browser, ordered Claim Autopsy, stress-lab, owner-inspection, evidence-reliance, identical-response, and architecture output paths, byte counts, and SHA-256 values |
| CLI result | `evalwitness.claim-render-result.v1` | destination, render bindings, `network_required=false`, `provider_calls=0` |

Report fields:

| field | required content |
|---|---|
| capsule | capsule ID, manifest digest, ledger digest, autopsy digest, and source identities |
| scope | evaluator, route availability, evidence role, release version, development/empirical/research counts, provider calls, empirical/ranking flags, and restricted-material exclusion |
| claim | claim ID, failable property, observable failure, acceptance, boundary, limitations, out-of-scope uses, and source |
| bom | executable, operands, required/decisive/surviving/truncated channels, repository-state digest, freshness, and source |
| method | current generation, boundary, ordered v1/v2/v3 generations, frozen denominators, observations, non-claims, guarding tests, transitions, and deep links |
| owner_inspection | public-attestation schema/content/file/attestation digests, 66-assessment completion, 16 aggregate dimensions, core/scarcity/boundary/overall outcomes, human/external-action absence, ten closed public claims, private-parent disclosure boundary, and source |
| transport | claim, accepted state, selected path/layer, accepted and loss paths, five ordered layers, per-layer object/detail availability, decisive channels, and sources |
| loss | earliest-loss identity, terminal disposition, precedence, failability resolution/reason, visibility, supported claim, unsupported claims, and source |
| stress | case-study identity and role, status, relation and constraint, original/transformed candidate orders and selections, outcome, original/final unit counts and digests, reduction counts and one-minimality, complete ordered proof/digest trace, final witness lines, provider/network/empirical boundaries, closed allowed/forbidden claims, reproduction command, and source |
| reliance | optional verified child-capsule projection with exact scope, 24 source tasks, 1,536 registered and outcome-bearing cells, excluded-cell count, 98 ordered term-outcome dimensions, ten selector audits, five arm contrasts, one privacy-safe witness, global-score prohibition, zero-provider/network boundary, and source |
| identical_response | optional verified TASK 070 projection with base/outer capsule identities, route/model scope, 60-group comparison, seven exact disagreement rows, five sealed claims, 34 receipts and class counts, limitations, evidence ceiling, and clean-clone reproduction identity |
| dataset | role, exact counts, limitations, out-of-scope uses, and source |
| limitations | artifact status, typed availability, boundary, required resolution, source, and optional resolving source |
| release | release identity, version, file totals, verified totals, public/restricted flags, complete path/role/bytes/SHA-256 manifest, and source |
| challenge | deterministic selected claim, claim state, evidence level, complete receipt counts, source identities, pack digest, ordered eight-class registry, typed availability, one receipt per applicable class, and replay command |
| extensions | stable extension ID, owner, required type IDs, exact `(type_id, component_id)` identities, missing types, and typed availability |
| reproduction | exact command, network requirement, provider calls, and source |

Availability values: `available`, `not_measured`, `not_applicable`,
`unsupported`, `not_authorized`, `not_run`, `withheld_private_parent`.
Unknown values are invalid in the report codec and neutral in the browser visual
fallback.

Challenge selection: count verified receipts per claim -> select maximum count ->
break equal counts by lexical claim ID -> project every registry class in
registry order -> require one valid receipt for each applicable class -> require
an explicit reason and no receipt for each inapplicable class. Replay commands
contain only the registered claim and challenge identities.

Report construction: load and verify the public capsule with the sealed reference
registry and public visibility ceiling -> decode the bounded canonical ledger ->
derive Claim Autopsy through the verified ledger path -> inside `BuildReport`,
atomically re-verify the capsule and ledger once, evaluate all claims,
deterministically rederive and compare Claim Autopsy, and generate every applicable challenge ->
load every bound release path through regular-file,
root-containment, byte-limit, byte-count, and SHA-256 checks -> validate the
complete report -> canonicalize -> calculate digest -> canonicalize with digest.
When reliance inputs are present, both child capsule and child ledger are
mandatory: verify the frozen public parent, verify the child family, evaluate
the complete 11-claim ledger, derive the Explorer projection from the child map,
and attach it before final report validation. A partial pair rejects; absence of
both inputs leaves the optional projection absent.
When identical-response inputs are present, all five paths are mandatory: verify
the response-bundle and outer-capsule family, sealed ledger, challenge-pack
identity and receipts, canonical analysis denominators, clean-clone report, and
source schemas; derive all rates and class counts from the verified objects; then
attach the digest-bound projection. Partial input rejects and no provider call is
permitted.

Rendering contract:

- input report passes complete validation before interpolation
- CSS, JavaScript, and manifest are compile-time embedded into the Go binary and byte-verified before rendering
- each embedded asset is at most 2 MiB; complete HTML is at most 8 MiB
- labels and link targets are escaped; asset bytes containing closing script/style element sequences reject
- canonical report bytes are base64 encoded with an independent SHA-256 meta binding
- exact CSS and JavaScript SHA-256 values authorize only the embedded blocks; script attributes and every unlisted script deny; narrowly scoped runtime style attributes remain allowed; all network, frame, object, media, form, base, and worker capabilities deny by default
- render destination is absent and publishes with exclusive creation; existing files remain unchanged
- no source map, runtime fetch, provider call, service worker, HTTP server, or external resource

Browser contract: `file://` operation; report-payload SHA-256 checked through
WebCrypto before render; responsive desktop, narrow-laptop, tablet, and mobile
layouts; keyboard-operable dialogs/tabs/tables; focus containment and Escape
dismissal; reduced-motion support; semantic landmarks; tabular fallback; WCAG
2.2 AA automated audit; zero horizontal overflow; zero external requests; zero
runtime console/page errors before accessibility instrumentation; exact method,
transport-layer, challenge-receipt, owner-inspection, table, and artifact
fragments restore only registry-derived state; unknown fragments resolve to
`#autopsy` without interpreting fragment content.

Terminal-proof contract: one `CLM-011` valid/assertable verification -> one
registered `clm-011.denominator-deletion` ephemeral challenge -> equal expected
and observed guard -> rejected after-state -> unchanged sealed-source digest ->
offline capsule and ledger verification -> self-contained render -> public
artifact scan. Runtime limit is 90 seconds. The recording is no-overwrite
asciicast v2, captures actual combined stdout/stderr, records no input, exposes
only `SHELL` and `TERM`, and remains a derived release output outside source-tree
provenance.

Build contract: `web/explorer` uses exact lockfile versions and strict TypeScript;
format, ESLint, typecheck, Vitest, Vite production build, embedded-byte identity,
Go tests, public artifact scan, Playwright browser matrix, deterministic PNG
capture, SVG parsing, and public presentation manifest verification are
mandatory. Presentation capture writes only to an absent derived-output
destination after report construction; its files are not source-provenance
inputs. The Go build and rendered runtime have no Bun, React, Vite, Tailwind, or
Playwright runtime dependency.

## Modes

### pairwise

Inputs: trajectories `[]string`, task `string`, criteria `[]Criterion`, nReps `int`. `evalwitness.selection.v2` returns state, abstention reason, conditional scores, wins, uncalibrated decision strength, evidence strength, usage, and `evalwitness.pair-decision.v2` rows.

Algorithm:
1. Default: a dynamic round-by-round bracket advances the measured winner and evaluates exactly N-1 matches; `EVALWITNESS_SINGLE_ELIM=false` generates all `(i, j)` with `i < j` instead
2. Pre-skip: for each pair where `sha256(traj_i) == sha256(traj_j)`, record `wins[i] += 0.5, wins[j] += 0.5`, dispatch nothing
4. Per pair: bundle criteria, run the balanced base order, compute conditional model win probability, and escalate only while the decision is below threshold; hard ceiling `EVALWITNESS_MAX_PAIR_CALLS` provider calls
5. Concurrent fanout ceiling: `min(EVALWITNESS_MAX_WORKERS, Capabilities.MaxConcurrent)`; dynamic tournament evaluates one round at a time because winners determine the next round
6. Explicit non-adaptive multi-rep configurations use fixed reps and optional legacy SPRT
7. Aggregate all observations in original trajectory orientation; count unique repeat IDs separately from forward/reversed paired views
8. Preserve conditional-token, repeated-sample, presentation-order, and policy variance separately; their sum is total variance
9. State is `selected`, `tied`, or `abstained`; order-direction disagreement forces `abstained/unstable_presentation_order`; an exhausted unconfident ceiling forces `abstained/evidence_ceiling_reached`; non-selected decisions have `winner=-1`
10. Tallying may retain a deterministic internal ranking, but selection state propagates every pair abstention and never publishes it as a selected benchmark reward or Best-of-N apply target

Adaptive pairwise decisions stop at `EVALWITNESS_MAX_PAIR_CALLS`. Fixed `both` configurations can apply the compatibility inconsistency policy.

### absolute

Inputs: trajectory `string`, task `string`, criteria `[]Criterion`, nReps `int`. Output schema `evalwitness.absolute-score.v2`: `state`, `conditional_score`, `per_criterion_conditional_scores`, `criterion_evidence`, `evidence_strength`, and `usage`. No confidence field is emitted.

Algorithm:
1. Bundle criteria in single prompt if `EVALWITNESS_MULTI_CRITERION_BUNDLE=true`, else one prompt per criterion
2. For each rep: dispatch promptAbsolute -> strict `ScoreEvidence` per criterion tag
3. SPRT per criterion: stop additional reps when accumulated variance below threshold; floor `EVALWITNESS_SPRT_MIN_REPS`, ceiling `EVALWITNESS_SPRT_MAX_REPS`
4. Per-criterion conditional mean across reps -> `per_criterion_conditional_scores[id]`
5. `conditional_score = mean(per_criterion_conditional_scores)`
6. Evidence strength reports observation count, extraction count, minimum returned Top-k, and minimum/mean visible and valid mass; calibrated confidence remains absent until a held-out model exists

Order-bias mitigation does not apply (single trajectory; nothing to swap).

### delta

Inputs: traceA, traceB, task, criteria, nReps. Output schema `evalwitness.delta.v2`: state, abstention reason, `winner=A|B|tie|none`, conditional margin/scores, per-criterion conditional scores, pair decision, evidence strength, observations, inconsistency, and usage.

Algorithm: one pair `(A,B)` under the same strict evidence and policy path as pairwise. Explicit `both` averages paired presentation orders; fixed multi-rep and SPRT compatibility remain available. Tie if the conditional margin is within epsilon. Order disagreement or insufficient decision strength returns `winner=none` with an abstention reason.

## Unified Verification Application Service

Schema: `evalwitness.verification-run.v1`. `internal/verification.Service` is the sole production owner of canonical preprocessing, request-set construction, replay/live selection, authorization, runtime creation, shared budget, mode dispatch, strict-verifier admission, run audit, and ordered cleanup. CLI, MCP, eval, Best-of-N, and the application-protocol adapter may only translate external syntax and format results.

Input contract:
| field | type | invariant |
|---|---|---|
| entrypoint | string | nonempty provenance label; excluded from semantic request fingerprint |
| mode | delta, absolute, pairwise | trajectory arity enforced by service |
| task | string | nonempty; maximum 1048576 bytes |
| trajectories | array<string> | absolute=1; delta=2; pairwise=2..10; each maximum 8388608 bytes; total input maximum 67108864 bytes |
| criteria | array<Criterion> | 1..16 unique complete criteria in explicit caller order |
| policy | Policy | strict_verifier or explicit_judge plus every decision-affecting mode value |
| limits | BudgetLimits | non-negative hard calls, attempts, input, output, concurrency, duration, and optional cost ceilings |
| authorization_digest | string or absent | exact live plan digest; absent only for preview or offline execution |
| budget_state_path | string or absent | restart-safe aggregate budget state |
| disable_cache | bool | per-run cache switch |
| lineage | LineageReferences | bounded validated caller-owned research references |

Planning: normalize input -> canonical-preprocess each trajectory once -> retain prepared text and accounting digests -> build the exact ordered request envelopes -> derive request fingerprints, set fingerprint, contract digest, and worst resource bounds -> resolve limits -> derive run fingerprint -> derive live authorization when applicable. `Execute` recomputes and byte-compares every plan component before opening a runtime; a mutated plan fails without dispatch.

Criterion order is caller-owned and contributes to prompt and request identity. Only map-derived or set-derived registries are lexicographically sorted. Reverse presentation order swaps trajectory text, prepared evidence, evidence bindings, and source/map lineage as one operation.

Output contract:
| field | type | invariant |
|---|---|---|
| schema_version | string | `evalwitness.verification-run.v1` |
| run_fingerprint | SHA-256 | binds normalized input, prepared evidence, exact request set, policy, limits, and caller lineage |
| mode | enum | matches planned mode |
| state | decision state | selected, tied, abstained, or failed semantics from the typed mode result |
| delta, absolute, selection | exactly one typed result | canonical mode result; strict selection is rejected if any contributing call is judge, mixed, or unextracted evidence |
| budget | BudgetSnapshot | aggregate logical calls, attempts, tokens, concurrency, cost, and deadline state |
| lifecycle | Lifecycle | runtime_open, execution, cleanup, audit are each complete or failed in the returned result; error is redacted terminal context |
| calibration_policy | object | status=unsupported_no_held_out_policy until a locked TASK 049 policy is bound; never a deployable correctness probability |
| policy.confidence_escalation | enum | disabled by default; extra adaptive calls do not read confidence_threshold; legacy is an explicit opt-in only |

`PlanBatch` validates and plans every item before runtime creation. `ExecuteBatch` uses one runtime and one aggregate budget for every item, attaches item-specific request lineage at dispatch, records item and batch failures without hiding cleanup/audit failures, and closes resources before the final run audit and audit sink.

Audit row kinds:
| kind | required evidence |
|---|---|
| provider_attempt | request fingerprint, attempt ordinal/status, usage reservation/observation, trajectory evidence, redacted error |
| provider_call | request/response identities, replay/cache status, strict score evidence, complete request lineage, inconsistent-pair contribution |
| verification_run | run/request-set fingerprints, policy, state/abstention, budget, lifecycle, output status, ordered run lineage |

Run lineage accepts `audit_case_id`, `transformation_id`, `outcome_evidence_digest`, `profile_policy_digest`, `capsule_digest`, and `study_cell_id`. Identifier references are trimmed control-free UTF-8 up to 250 bytes; digest references are exact lowercase SHA-256. The service adds ordered source-trace and trace-map digests plus the decision-policy digest. Audit is written after resource cleanup and closed last, so persisted lifecycle never reports a pending cleanup as completed evidence. The run row records its own successful write; an audit-sink close failure is observable through the returned lifecycle/error and process exit because the failed sink cannot rewrite the row.

## Reliability Output

Benchmark rewards are the only correctness labels. Reliability output is eval-only and always carries `data_role=development`; production `verify` never invents a proxy label from its own score.

Schema version: `logprobe.reliability.v1`.

| path | type | description |
|---|---|---|
| calibration.schema_version | string | reliability wire-schema identity |
| calibration.data_role | string | `development`; never held-out evidence |
| calibration.extraction_mode | string | verifier, judge, or mixed |
| calibration.cluster_count | int | task clusters used by pairwise intervals |
| calibration.metrics | object | typed ECE, MCE, Brier, AUC, and accuracy evidence; absent in absolute-only JSON |
| calibration.error_decomposition | object | fixed wrong-direction score-difference strata; absent in absolute-only JSON |
| calibration.source_rows | array | pairwise observations with execution and outcome provenance; absent in absolute-only JSON |
| calibration.absolute | object | absolute-score Spearman evidence and source rows; absent in pairwise-only JSON |
| calibration_policy | object | status=unsupported_no_held_out_policy on eval-terminal and eval-swebench; reliability metrics stay development-only |

Each `calibration.metrics.<id>` and `calibration.absolute.spearman` value:

| field | type | description |
|---|---|---|
| value | float64 | point estimate |
| count | int | observations entering the estimate |
| low_sample | bool | true when count is below 30; the metric remains visible |
| interval | object or absent | deterministic task-cluster percentile interval |

Interval fields: `lower: float64`; `upper: float64`; `level=0.95`; `method=task_cluster_percentile_bootstrap`; `replicates=2000`; `effective_replicates: int`; `clusters: int`. Pair decisions from one task are one resampling cluster. Error-share replicates containing zero directional errors are excluded and reduce `effective_replicates`.

Pairwise metric definitions use observations `(p,y)`, where `p` is the raw probability that pair slot 0 wins and `y` is its binary reward-derived outcome. Ten equal-width bins cover `[0,1]`.

| metric | definition |
|---|---|
| reliability bin | count, mean predicted probability, and mean observed outcome per non-empty bin |
| ECE | sum over bins of `bin_count / N * abs(mean_observed - mean_predicted)` |
| MCE | maximum non-empty-bin `abs(mean_observed - mean_predicted)` |
| Brier | mean `(p-y)^2` |
| AUC | probability a randomly selected positive outranks a negative; prediction ties contribute 0.5; one-class samples return 0.5 |
| accuracy | direction correct at threshold 0.5; exact 0.5 contributes 0.5 |
| Spearman | mid-rank Pearson correlation between absolute trajectory score and reward |

Pairwise observation fields:

| field | type | description |
|---|---|---|
| id | string | stable task-and-pair row identity |
| task_id | string | benchmark task or instance identity |
| pair | [2]int | trajectory indexes in original orientation |
| extraction_mode | string | verifier or judge |
| predicted | float64 | probability that pair[0] wins |
| won | bool | whether reward[pair[0]] exceeds reward[pair[1]] |
| outcome_id | string | stable reward join identity |
| mean_difference | float64 | oriented score difference |
| score_mass | float64 | retained valid score-token probability mass |
| calls | int | logical calls consumed by the decision |
| pair_call_limit | int | configured hard per-pair ceiling |
| order_policy | string | adaptive, both, single, or disabled |
| first_order | string | forward or reversed |
| inconsistent | bool | whether observed orders disagreed |

Equal-reward pairs are excluded because no correct pair direction exists. Exact `predicted=0.5` rows remain in calibration and Brier/AUC calculations but are unresolved rather than directional errors. Error bands are fixed before analysis: near_zero `[0,0.05]`; moderate `(0.05,0.20]`; large `(0.20,1]`, using `abs(mean_difference)` among wrong-direction rows.

Absolute observations are emitted only for decidable tasks and retain `id`, `task_id`, `trajectory`, `extraction_mode`, `predicted`, `actual`, and `outcome_id`. Absolute-mode JSON omits pairwise metrics and reports Spearman rank correlation separately.

Claim-ledger expression paths are exact: pairwise point estimates use `calibration.metrics.<metric>.value`; their uncertainty uses `calibration.metrics.<metric>.interval`; error evidence uses `calibration.error_decomposition.strata`; absolute rank fidelity uses `calibration.absolute.spearman`. The offline manifest uses `reports[].metrics`, `reports[].error_decomposition`, and source artifact SHA-256 values from `scripts/audits/run-calibration-analysis.sh --format json`.

## MCP Server

Transport: JSON-RPC 2.0 over stdio per MCP spec. stdout is reserved for protocol
frames and stderr for logs. Each frame is one newline-delimited JSON object.

Current protocol version: `2026-07-28`. Modern requests are stateless. Every
request carries the following fields inside `params._meta`:

| field | invariant |
|---|---|
| io.modelcontextprotocol/protocolVersion | exactly `2026-07-28` |
| io.modelcontextprotocol/clientCapabilities | object; may be empty |
| io.modelcontextprotocol/clientInfo | optional object with nonempty name and version |

Modern lifecycle: `server/discover` -> optional cached capability selection ->
stateless `tools/list` or `tools/call` requests. Modern `ping` and
`logging/setLevel` requests fail with method-not-found because the revision
removed both methods; per-request log-level metadata replaces the latter.
`server/discover` returns supported versions in newest-first order, tools
capability, instructions, `ttlMs=3600000`, and `cacheScope=public`. Every modern
success response carries `resultType=complete` and
`io.modelcontextprotocol/serverInfo` with name `evalwitness` and the canonical
product version. Modern tool success returns both one JSON-stringified text item
and byte-equivalent `structuredContent`. Expected tool-application failures
return `isError=true` with structured error content; malformed requests, unknown
methods or tools, unsupported protocol metadata, and internal failures remain
JSON-RPC errors.

Legacy compatibility versions: `2025-11-25`, `2025-06-18`, `2025-03-26`, and
`2024-11-05`. Legacy lifecycle: `initialize` with one exact supported revision
-> response echoing that revision, tools/logging capabilities, and server info ->
required `notifications/initialized` -> `tools/list`, `tools/call`, `ping`, and
`logging/setLevel`. Legacy responses preserve their historical shape and tool
failures remain JSON-RPC errors. `server/discover` is modern-only. On
`notifications/cancelled { requestId, reason? }`, the matching call is cancelled
and returns no response. EOF cancels and drains in-flight work.

Product version source: `internal/product/version.go`; canonical value must be
valid SemVer without a `v` prefix. The CLI version command, provider user agent,
MCP server metadata, build provenance, and release tag gate consume the same
value.

Tools:
| name | description |
|---|---|
| evalwitness_pairwise | select best from N agent trajectories using tournament scoring |
| evalwitness_absolute | score a single trajectory in [0,1] for goodness |
| evalwitness_delta | compare trajectory A vs B and report winner with margin |
| evalwitness_calibration_evaluate | offline IsDeployable on a test-split observation array; not a TASK 049 study or production policy |

`evalwitness_pairwise` input schema:
| field | type | required | description |
|---|---|---|---|
| task | string | yes | task description or problem statement |
| trajectories | array<string> | yes | 2-10 trace texts |
| criteria | array<string\|object> | no | preset names or full custom criteria objects, default ["generic"] |
| n_reps | int | no | base reps per pair-criterion, default 1, max 16; policy combinations can impose a lower effective bound |
| authorization_digest | string | no | exact digest returned by the live authorization preview; required only for live dispatch |

`evalwitness_pairwise` output:
| field | type | description |
|---|---|---|
| schema_version | string | `evalwitness.selection.v2` |
| state | string | selected, tied, or abstained |
| abstention_reason | string or absent | typed non-selection reason |
| best_index | int | deterministic internal ranking index; authoritative only when state=selected |
| decision_strength | float | uncalibrated 0-1 tournament margin |
| conditional_scores | array<float> | per-trajectory conditional scores |
| wins | array<float> | per-trajectory tournament wins |
| evidence_strength | object | observation and minimum/mean coverage summary |
| pair_decisions | array<object> | versioned policy, state, order/repeat evidence, uncertainty components, decision strength, raw score evidence, and abstention |
| usage | object | calls, tokens, cache, explicit logprob/judge counts, extraction mode, and estimated cost |

`evalwitness_absolute` input: `{ task: string, trajectory: string, criteria?: array, n_reps?: int, authorization_digest?: string }`.
Output schema `evalwitness.absolute-score.v2`: state, conditional score, per-criterion conditional scores, criterion evidence, evidence strength, and usage. It has no confidence field.

`evalwitness_delta` input: `{ task: string, trajectory_a: string, trajectory_b: string, criteria?: array, n_reps?: int, authorization_digest?: string }`.
Output schema `evalwitness.delta.v2`: state, abstention reason, `winner=A|B|tie|none`, conditional margin and scores, per-criterion conditional scores, decision, evidence strength, observations, and usage.

Error codes (JSON-RPC `error.code`):
| code | name | when | data fields |
|---|---|---|---|
| -32700 | parse error | malformed JSON frame | - |
| -32600 | invalid request | not JSON-RPC 2.0 shape | - |
| -32601 | method not found | unknown method | method |
| -32602 | invalid params | input schema validation failed | path, message |
| -32603 | internal error | unexpected exception | trace_id |
| -32001 | provider error | upstream LLM returned error | provider, status_code, body, request_id |
| -32002 | rate limited | provider 429 after retries exhausted | retry_after_sec |
| -32003 | capability missing | strict verifier requires logprobs that the provider does not support | provider, missing |
| -32004 | timeout | request exceeded EVALWITNESS_TIMEOUT_SEC | timeout_sec |
| -32005 | trajectory too large | post-preprocess size still exceeds context | tokens_estimated, limit |
| -32006 | invalid criterion | unknown preset or malformed custom criterion | criterion_id |
| -32007 | auth failed | API key missing, invalid, or unauthorized | provider, hint |
| -32008 | cost cap exceeded | estimated input cost > EVALWITNESS_MAX_COST_USD_PER_CALL before dispatch | est_cost_usd, cap_usd |
| -32009 | run budget exceeded | calls, estimated input, cost, or duration would cross a hard run limit | metric, used, requested, limit |
| -32010 | score evidence rejected | response fails alignment, Top-k, mass, numeric, mode, or extraction invariants | complete ScoreEvidence |
| -32011 | live authorization required | first live call or changed execution plan | complete authorization plan and digest |
| -32022 | unsupported protocol version | modern request metadata names a revision other than `2026-07-28` | supported, requested |

## Streaming and Early-Stop

Streaming optimization: stop accepting semantic score content after all required score tags have been observed, then drain the bounded stream until terminal usage, `[DONE]`, or the drain ceiling so usage can be reconciled. This preserves the evidence cutoff while preferring accurate usage over an immediate transport cancel.

Activation:
| condition | result |
|---|---|
| Capabilities.Logprobs=true | enabled by default |
| stream chunk format includes per-token logprobs | required, else fall back to non-streaming |
| EVALWITNESS_STREAM=false env override | disabled |
| Provider lacks logprobs in stream chunks (e.g. some Gemini variants) | disabled, full request used |

SSE chunk handling (OpenAI-compatible):
1. Read line, strip `data: ` prefix
2. If line == `[DONE]`: terminate normally
3. Parse JSON; extract `choices[0].delta.content` and `choices[0].logprobs.content`
4. Append delta to `text_so_far`, append logprob records to running array
5. After each chunk, run logprob position-finding pass against pending tags
6. If all tags found: freeze score content and continue the bounded read for terminal usage; the operation deadline remains authoritative
7. If finishReason is set without all tags found: record the partial response; strict verifier validation rejects it, while explicit judge mode may use exact text extraction under its separate request identity

Early-stop semantics: semantic evidence ends at the last required closing tag. The client drains the remaining bounded stream to capture provider usage; if usage is absent, the spend firewall retains the complete worst-case reservation.

Streaming disabled fallback: full POST, parse complete response, find positions in single pass. Same algorithm, no chunked logic.

## CLI

Subcommands:
| command | purpose |
|---|---|
| evalwitness verify | one-shot mode dispatch |
| evalwitness mcp-serve | start MCP server on stdio |
| evalwitness probe | weak runtime capability diagnostics; never qualifies a verifier route |
| evalwitness attest | bounded real score qualification and exact attestation persistence |
| evalwitness capsule build | provider-free public reference package; no-overwrite capsule directory, deterministic archive, claim ledger, in-toto Statement, claim projection, and Claim Autopsy |
| evalwitness capsule verify | offline registry, manifest, graph, payload, inventory, visibility, and optional ledger/Statement/projection/autopsy verification |
| evalwitness capsule build-reliance | derive and no-overwrite publish the public TASK 065 child capsule, deterministic archive, and 11-claim ledger from the frozen TASK 050 base and canonical reliance map |
| evalwitness capsule verify-reliance | verify the frozen base, public child family, map identity, 11-claim ledger, and deterministic profile, paper, and Explorer projections offline |
| evalwitness capsule build-private-relation | owner-only private relation child from exact package-format-v5 and completed guided-inspection chain |
| evalwitness capsule verify-private-relation | verify public parent, private child, complete private proof, and exact capsule-family binding |
| evalwitness claim verify | evaluate the complete canonical ledger or one `CLM-NNN` against the verified capsule |
| evalwitness claim explain | emit one closed claim contract and its current verification result |
| evalwitness claim challenge | execute one registered challenge or all applicable challenges on ephemeral evidence and emit canonical receipts |
| evalwitness claim autopsy | project method-self-falsification and five-layer claim-transport lifecycles from typed capsule parents |
| evalwitness claim report | emit the canonical verified evidence-explorer report from capsule, ledger, Claim Autopsy, release files, optional jointly supplied reliance inputs, and optional jointly supplied five-path TASK 070 inputs |
| evalwitness claim render | no-overwrite render one deterministic self-contained offline HTML explorer from the same verified evidence families and emit exact render bindings |
| evalwitness claim surface render / verify | render one closed surface or byte-verify all tracked public claim surfaces |
| evalwitness profile build / verify / diff / policy / render | build a versioned multidimensional reliability profile offline, verify its digest, diff compatible profiles with metric/scope/evidence/denominator/capsule-expression changes surfaced separately, evaluate a content-addressed named policy rendering every failed and unknown requirement, and render text/markdown/full evidence report without a global score |
| evalwitness audit --policy --profile [--format json\|junit\|markdown] | offline CI audit: strict DisallowUnknownFields decode of both inputs, recomputed policy content digest with tamper-evident declared-digest check, printed execution plan, stable exit contract 0 pass / 1 policy failed / 2 invalid input / 3 internal error; JUnit keeps statistical failures aggregate, SARIF is reserved for file-local findings |
| evalwitness eval-terminal | Terminal-Bench evaluator with Pass@1, oracle, verifier-selected score, and usage |
| evalwitness eval-swebench | SWE-bench Verified evaluator with Pass@1, oracle, verifier-selected score, and usage |
| evalwitness bon | best-of-N: execute bounded isolated attempts, persist immutable evidence, resume exact application-service selection, and optionally apply without staging |
| evalwitness fidelity | offline cross-format transcript conservation, provider-usage comparison, and budget-retention report |
| evalwitness protocol run | execute the normative corpus in-process or through an operator-selected adapter |
| evalwitness protocol cases | emit the expanded normative corpus and immutable corpus digests |
| evalwitness protocol schema | emit one embedded protocol schema by exact filename |
| evalwitness protocol reference-adapter | serve the closed-form evaluator over protocol NDJSON |
| evalwitness protocol application-adapter | serve the production verification application service over bounded protocol NDJSON; offline or exact replay only |
| evalwitness trace inspect | detect and validate a source, then emit envelope, hierarchy, retention, and mapping loss without provider access |
| evalwitness trace export | emit canonical JSON, pinned OTLP/JSON, or Agent Trace attribution under an explicit privacy class |
| evalwitness trace lineage plan | emit the sealed TASK 069 research plan |
| evalwitness trace lineage schema-inventory | emit the content-addressed ten-object parent DAG |
| evalwitness trace lineage source-inventory | emit the sealed pre-acquisition source-admission inventory |
| evalwitness trace lineage source-specifications | emit the pinned six-format source, license, admission, and expected-capability registry |
| evalwitness trace lineage fixture-witnesses | execute the closed local process controls and emit deterministic source/witness fixtures |
| evalwitness trace lineage golden-vectors | emit the sealed three-format, 21-case native adapter-development vectors and observed mapping gaps |
| evalwitness trace lineage adapter-conformance | reproduce raw-record/field accounting, causal linkage, stable identity, canonical JSON round-trip, command/redaction/retention, and exit-status checks over the sealed vectors |
| evalwitness trace lineage parser-lock / parser-lock-verify | reproduce or strictly verify the pre-calibration source and governance lock |
| evalwitness trace lineage source-readiness / source-readiness-verify | reproduce or strictly verify the zero-admission pre-acquisition audit |
| evalwitness trace lineage holdout-readiness / holdout-readiness-verify | reproduce or strictly verify the holdout contamination decision |
| evalwitness trace lineage corpus-feasibility / corpus-feasibility-verify | reproduce or strictly verify the unchanged-threshold feasibility decision |
| evalwitness trace lineage capability-matrix / capability-matrix-verify | reproduce or strictly verify the separated format-representability and development-observation matrix |
| evalwitness trace lineage offline-proof / offline-proof-verify | replay or strictly verify the sealed accepted chain and non-failable counterexample |
| evalwitness trace lineage loss-certificate / loss-certificate-verify | emit or strictly verify the public-safe earliest-loss certificate derived from the offline proof |
| evalwitness trace lineage lineage-graph / lineage-graph-verify | emit canonical JSON or its deterministic SVG projection, or strictly verify the JSON source |
| evalwitness trace lineage offline-audit / offline-audit-verify | emit or strictly verify the conserved positive development audit |
| evalwitness trace lineage offline-bom / offline-bom-verify | emit or strictly verify the accepted positive verification-evidence BOM |
| evalwitness trace lineage development-dataset-card / development-dataset-card-verify | emit or strictly verify the zero-empirical-unit development dataset card |
| evalwitness trace lineage limitations / limitations-verify | emit or strictly verify the machine-readable open evidence boundaries |
| evalwitness trace lineage development-release / development-release-verify | emit or strictly verify the 20-file public development manifest against repository bytes |
| evalwitness trace lineage intake | import one explicit trace in metadata-only mode and report capability, conformance, and missing-input plans without execution or analysis |
| evalwitness trace lineage schema | emit a closed lineage schema by exact type |
| evalwitness trace lineage validate | strictly validate and digest a lineage document by exact type |
| evalwitness design simulate | execute deterministic source-task-clustered sparse-factorial design simulation from a strict JSON spec |
| evalwitness design reliance-preflight | reproduce the frozen TASK 065 Walsh alias audit, ascending clustered power search, grid MDE, and hard subscription-route resource envelope without provider access or live authorization |
| evalwitness design identical-response | reproduce the frozen TASK 070 distribution_aware_vs_chosen_token paired design (exact conditional McNemar detectable effect plus combined-loss failure sensitivity) with zero provider calls and no Monte Carlo randomness |
| evalwitness study validate | strictly decode and validate a study manifest; emit its canonical digest |
| evalwitness study lock | create a content-addressed locked study record |
| evalwitness study transition | append one checksum-chained lifecycle transition |
| evalwitness study report | render a protocol report containing no observed claims |
| evalwitness study split | generate a deterministic dataset-bound lineage-safe split |
| evalwitness study schema | emit closed manifest, record, or split JSON Schema 2020-12 |
| evalwitness study inventory | verify the committed historical development-data boundary |
| evalwitness study identical-response-inventory | derive the frozen TASK 070 eligible development inventory (100 unique source task groups with data roles, licenses, redistribution classes, source/trajectory/evidence digests, outcome availability, and contamination boundaries) from the committed controlled-corruption release with zero provider calls |
| evalwitness study identical-response-redistribution-right | reproduce the frozen TASK 070 response redistribution-right evidence record (primary-source output-ownership assignment clauses, retrieval dates, conditions, and procedural verdicts for the DeepSeek direct and OpenCode Go routes) with zero provider calls |
| evalwitness study identical-response-protocol | reproduce the frozen TASK 070 identical-response study protocol (counterfactual, primary endpoint, task-group aggregation, missingness, multiplicity, fixed stopping, minimum support, and exact uncertainty procedure) bound by digest to the design, inventory, and redistribution-right artifacts with zero provider calls |
| evalwitness study verify-execution | verify an observed execution and declared inputs against an authorized record |
| evalwitness mutation validate | validate one content-addressed mutation manifest |
| evalwitness mutation schema | emit a closed JSON Schema for a mutation artifact |
| evalwitness mutation verification-evidence build-challenge | emit the deterministic nine-case provenance, failability, and claim-specific evidence-loss challenge |
| evalwitness mutation verification-evidence validate-challenge | strictly validate and reproduce every challenge fixture, assessment, reason, denominator, claim boundary, and digest |
| evalwitness mutation control validate | reproduce a formal pass-to-fail positive control |
| evalwitness mutation corpus spec | emit the executable default or validate a governed corpus specification |
| evalwitness mutation corpus build | regenerate a reference-only release descriptor from fetched upstream trajectories |
| evalwitness mutation corpus validate | validate release identity, witnesses, counts, balance, controls, and split isolation |
| evalwitness stress catalog | build the exact 15-relation v3 catalog from the checked-in plan, audit, and release |
| evalwitness stress arm-plan | replay the fetched v3 corpus and emit the exact 10-arm, 5,630-cell comparison plan |
| evalwitness stress analysis-design | replay the fetched v3 corpus and emit the corpus- and arm-plan-bound analysis lock |
| evalwitness stress held-out-lock | replay the fetched v3 corpus and emit the once-only 57-case, 1,140-cell test partition |
| evalwitness stress held-out-campaign | replay the fetched v3 corpus and emit the exact ten-arm test topology, cell-set commitments, and repetition arithmetic with every live binding absent |
| evalwitness stress held-out-readiness | reproduce the exact current partition and owner gate provider-free as a non-authorizing JSON or Markdown refusal with zero empirical units and provider calls |
| evalwitness stress development-case-study | emit canonical JSON or deterministic Markdown/SVG for the candidate-order negative control over tracked fixtures and its one-minimal witness; projections are never machine evidence |
| evalwitness stress development-challenge | emit the self-contained licensed-fixture challenge for the canonical development case |
| evalwitness stress verify-development-challenge | reconstruct the complete case from embedded bytes, execute seven fixed guards, and emit a deterministic receipt without repository access |
| evalwitness stress verify-development-challenge-receipt | verify a persisted receipt against the exact challenge without rerunning the reducer |
| evalwitness stress validate | strictly decode and repository-reproduce one `development-case-study`, `held-out-campaign-plan`, or `held-out-run-readiness-refusal` document |
| evalwitness stress schema | emit one of 35 closed Draft 2020-12 stress schemas |
| evalwitness outcome plan | emit the executable default or validate the governed adjudication plan |
| evalwitness outcome sample | bind the governed plan to a controlled-corruption release and emit the frozen mutation sample |
| evalwitness outcome pilot-sample | reproduce the six-case natural outcome pilot v2 from the governed plan and natural inventory |
| evalwitness outcome pilot-sample-v1 | reproduce the non-launchable mixed-objective historical diagnostic from the governed plan, mutation sample/release, and natural inventory |
| evalwitness outcome pilot-materials | reproduce exact selected raw trajectories, emit restricted two-slot outcome review items, and exclusively publish one sealed owner-only private-materials custody root |
| evalwitness outcome pilot-inspect | consume the sealed private-materials root, then reproduce and seal structural reviewability, lineage, retention, anchor, message/action/result, byte, and token denominators while keeping semantic validity explicitly pending |
| evalwitness outcome natural-request | emit the default or validate a governed development-only inventory request |
| evalwitness outcome natural-inventory | build the deterministic natural-case inventory and explicit shortfalls |
| evalwitness outcome packet | emit one public HMAC-blinded packet and exclusively publish its owner-only private mapping |
| evalwitness outcome evidence | validate a strict evidence draft and emit content-addressed outcome evidence |
| evalwitness outcome record | validate a strict record draft and emit a content-addressed pre-human outcome record |
| evalwitness outcome label | validate a label draft and emit its content-addressed sealed label |
| evalwitness outcome qualification | emit the default or validate the governed public qualification set |
| evalwitness outcome qualify | score sealed qualification labels and emit a reviewer-specific report |
| evalwitness outcome review handbook | emit the exact default content-addressed handbook or verify a sealed handbook against its governed qualification set |
| evalwitness outcome review bundle | bind the governed plan, qualification set, handbook, data role, visibility, task clusters, and blinded packets |
| evalwitness outcome review pilot-readiness | consume either the sealed private-materials root or an explicit compatible mapping array, require the matching sealed structural inspection, prove exact committed pilot coverage and local technical readiness, and preserve a machine-readable semantic-pending and not-authorized external-action boundary |
| evalwitness outcome review reviewer | seal pseudonymous consent, independence, role, authorship policy, private contact custody, and conflict declarations |
| evalwitness outcome review assign-primary | require a passing reviewer-specific qualification and generate a deterministic secret-seeded full-bundle assignment for slot one or two |
| evalwitness outcome review kit | emit one self-contained content-addressed reviewer artifact containing the exact handbook, assignment, and assigned blinded packets in committed order |
| evalwitness outcome review verify-kit | reproduce a reviewer kit against the complete sealed bundle and reject bundle, handbook, packet, plan, role, visibility, or order drift |
| evalwitness outcome review render-kit | require kit-to-bundle verification, then deterministically render escaped human-readable Markdown without private mappings or study answers |
| evalwitness outcome review label-batch | require exact post-assignment label coverage and seal the complete commitment |
| evalwitness outcome review analyze-rubric | after both primary label commitments and before reveal, reproduce their exact labels and emit outcome, five-axis, prevalence, unclear/indeterminate, and reason-code disagreement metrics |
| evalwitness outcome review blinding-protocol | derive and freeze the complete hidden-condition universe from the sealed private-materials root or an explicit compatible mapping array before reviewer assignment |
| evalwitness outcome review blinding-probe-batch | after a primary label commitment, seal complete source-condition guesses, confidence, and task-recognition disclosures before reveal |
| evalwitness outcome review assign-tie | after both primary commitments, assign exactly their disagreements to an independent slot-three reviewer |
| evalwitness outcome review reveal | consume the sealed private-materials root or an explicit compatible mapping array, then after all required commitments disclose reproducible ordering seeds and bind complete private-mapping digests |
| evalwitness outcome review analyze-blinding | consume the sealed private-materials root or an explicit compatible mapping array, then after reveal score both primary probe commitments with uncertainty, chance correction, confidence error, and recognition breakdowns |
| evalwitness outcome review adjudicate | require the bound prereveal rubric-ambiguity and post-reveal semantic-blinding analyses, compute primary agreement and resolutions, then seal the complete or explicitly unresolved adjudication ledger |
| evalwitness outcome review analyze-sources | consume the sealed private-materials root or an explicit compatible mapping array, then strictly after terminal adjudication reconcile pre-human source records with ledger-bound human resolutions without accepting verifier-performance inputs |
| evalwitness outcome agreement | compute raw agreement, prevalence, Cohen kappa, and task-cluster intervals |
| evalwitness outcome resolve | resolve two reviewer-qualified primary labels with an optional reviewer-qualified tie-break |
| evalwitness outcome preservation | evaluate outcome preservation without evaluator evidence |
| evalwitness outcome validate | strictly validate one named outcome artifact and emit its digest |
| evalwitness outcome schema | emit one of 40 closed JSON Schema 2020-12 outcome artifacts |
| evalwitness relation materialize | produce one restricted license-aware pair-aligned case artifact under the fixed per-source evidence budget while retaining the same immutable event lineages on both transformed sides |
| evalwitness relation packet | emit one objective-typed HMAC-blinded relation packet and exclusively publish its content-addressed owner-only private mapping |
| evalwitness relation qualification | owner-key-randomize the eight supervised competency packets, emit only the answer-free set, and exclusively publish the content-addressed owner-only answer key |
| evalwitness relation qualify | privately consume the mode-0600 answer key and seal exact per-case scoring with mandatory ambiguity/order competency gates |
| evalwitness relation handbook | freeze the relation-only evidence, rubric, applicability, conflict, blinding, submission, data, labor, privacy, and claim policy |
| evalwitness relation reviewer | seal one pseudonymous consent, independence, authorship, private-contact, role, and conflict record without authorizing contact or distribution |
| evalwitness relation bundle | privately consume complete mappings, verify packet custody, and emit a restricted sample/qualification/handbook-bound packet bundle in HMAC packet order |
| evalwitness relation pilot-readiness | bind the exact versioned development sample, restricted bundle, owner-only mappings, qualification, handbook, workload, per-packet structural checks, semantic-inspection requirement, and external-action boundary; v3 additionally requires the exact primary and separate scarcity-sentinel parents and seals seven core cases |
| evalwitness relation pilot-change-receipt | reproduce readiness against restricted packet/mapping custody and emit the content-addressed raw-content-free structural receipt with no decision, human result, or authorization |
| evalwitness relation pilot-launch-dossier | emit the content-addressed owner decision surface with exact maximum workload, structural packet disclosures, pending governance decisions, and separately unauthorized external actions; no raw restricted evidence or launch authority |
| evalwitness relation render-pilot-launch-brief | validate the launch dossier and render the deterministic public-safe owner decision surface without identities, digests, restricted material, mappings, results, or authorization |
| evalwitness relation render-pilot-change-atlas | reproduce readiness against restricted packet/mapping custody, optionally require the exact change receipt, and render the complete owner-only line-difference atlas, exact candidate-reversal proof, structural questions, and empty owner decision matrices without deciding construct validity |
| evalwitness relation render-pilot-inspection | verify readiness against the complete restricted bundle and private mappings, then render every hidden relation identity beside its injection-safe reviewer-visible surface and owner decision matrix |
| evalwitness relation render-scarcity-inspection | verify the exact v3 plan, primary sample, exhaustive scarcity sentinel, corpus plan, natural audit, release, and one restricted case material per sentinel reference; replay every material from frozen sources before rendering an owner-only construct-availability appendix with no decision, packet, label, held-out claim, primary-estimand input, human/provider result, or authorization |
| evalwitness relation render-scarcity-public-brief | verify the public v3 corpus plan/audit/release and relation plan/primary/sentinel chain; `--format json` seals the canonical public evidence contract and `--format markdown` renders its deterministic human view; both carry the 198/3/195 funnel, 37-case shortfall, 2-development/1-calibration/0-test boundary, public case/firewall commitments, parent digests, and closed claim states with no restricted content, owner notes, decisions, labels, human/provider/verifier results, held-out promotion, primary-estimand input, or authorization |
| evalwitness relation render-owner-inspection-public-attestation | revalidate the exact private package-v5 inventory, governed v3 parents, immutable session, contiguous event chain, inspection record, and completion receipt; project only the package commitment, date, exact assessment/dimension aggregates, aggregate outcomes, disclosure boundary, and claim states; never emit private journal identities, restricted evidence, a human/provider result, authorization, held-out validity, corrected-corpus feasibility, or a claim of public source reproduction |
| evalwitness relation pilot-inspection | privately consume one complete canonical decision per readiness packet, exclusively publish the content-addressed record at mode 0600 under a caller-owned private root, and return only its non-sensitive receipt with derived acceptance, revision-required, unresolved, claim-scope, human-study-not-run, and external-action states |
| evalwitness relation verify-pilot-inspection | require a mode-0600 inspection record and independently reproduce its exact readiness, restricted-bundle, private-mapping, packet, case, family, unit, digest, and claim-boundary bindings |
| evalwitness relation pilot-inspection-session-start | verify every declared package-v5 payload path, size, mode, and SHA-256 plus the exact v3 plan/primary/pilot/sentinel/readiness/bundle/mapping/material parents; refuse a journal root inside the immutable package; exclusively publish one immutable mode-0600 session header with a 66-assessment denominator |
| evalwitness relation pilot-inspection-session-guide | revalidate package/session/full event chain; resolve the next or one explicit applicable target; return only the fixed dimension prompt, exact source-file SHA-256, packet/case/boundary line ranges, allowed ratings, and confirmation template; never print or duplicate restricted evidence |
| evalwitness relation pilot-inspection-session-record | revalidate package/session/full event chain; target an explicit core packet, scarcity case, scarcity boundary, or the next unanswered dimension; require passed/failed/indeterminate plus exact `KIND:ID:DIMENSION:ASSESSMENT` confirmation; require `--correct` for an answered key; exclusively append the next sequence file |
| evalwitness relation pilot-inspection-session-status | revalidate package parents and every contiguous event; report core/scarcity/boundary progress, next target, derived summaries when complete, claim boundaries, and independently verified finalization state; never copy restricted evidence |
| evalwitness relation pilot-inspection-session-finalize | require all 66 applicable latest assessments; deterministically derive seven core summaries, three scarcity summaries, the four-check scarcity boundary, core/scarcity/combined status, existing seven-packet v3 inspection record, and completion digest; publish nothing without exact second-step digest confirmation; idempotently verify identical existing files and independently reproduce both outputs after publication |
| evalwitness relation assign-primary | require a conflict-free passing reviewer and privately generate an owner-seeded full-bundle order for slot one or two in planned-not-shared state |
| evalwitness relation judgment | bind one complete seven-axis visible-side response to its exact assignment and packet, or create the next immutable revision from the immediately preceding digest |
| evalwitness relation judgment-batch | require exact assignment coverage by latest sealed judgments and commit the sorted batch strictly after every submission |
| evalwitness relation analyze-ambiguity | after both complete primary commitments and before reveal, reproduce seven-axis, rating-prevalence, unclear/not-applicable, reason-divergence, uncertainty, and tie-break-scope evidence without mappings |
| evalwitness relation assign-tie | require the exact prereveal ambiguity commitment, an independent qualified tie-break reviewer, private mappings, and a separate owner seed; assign only committed disagreement packets in slot three |
| evalwitness relation probe-batch | require one post-label family/direction/source-condition/task-recognition probe per committed primary judgment and seal complete coverage before reveal |
| evalwitness relation reveal | after both complete probe batches and every required tie-break commitment, verify all private mapping references and assignment seeds/orders, then seal the reveal actor and time |
| evalwitness relation compare | consume exact reveal parents and private mappings, resolve all packet axes conservatively, and emit sealed packet resolutions plus the formal-human comparison |
| evalwitness relation terminal-ledger | verify complete bundle resolution coverage and bind formal witness, human construct state, and verifier-not-consulted evidence into the terminal ledger |
| evalwitness relation kit | create one self-contained assignment-ordered JSON reviewer kit with complete handbook and packets |
| evalwitness relation verify-kit | reproduce exact kit coverage, order, packet digests, reviewer, qualification, handbook, assignment, and bundle bindings |
| evalwitness relation render-kit | verify the kit first, then render deterministic injection-safe Markdown with untrusted-content fences |
| evalwitness relation plan | emit the executable default or validate the governed controlled-relation plan |
| evalwitness relation plan-v2 | derive and seal the prospective v2 controlled-relation plan from one strictly validated corrected corpus release with exact spec, mutation-program, and construct-audit bindings |
| evalwitness relation plan-v3 | derive and seal the v3 controlled-relation plan from the exact corpus development plan, natural audit, and typed v3 release |
| evalwitness relation pilot-sample | reproduce the eight-case non-overlapping development pilot from the plan, primary commitment, and corpus release |
| evalwitness relation primary-sample | reproduce the versioned primary rule: all and only 31 review-required v1 cases, or the jointly feasible balanced 32-case v2 family/split/task-group design |
| evalwitness relation replay | resolve frozen source bytes and reapply one case while emitting only a digest-bound exact replay receipt |
| evalwitness relation replay-v3 | resolve frozen source bytes and reapply one case through mutation-program v3 while binding the exact corpus plan, natural audit, release, typed firewall, and v3 replay identity |
| evalwitness relation materialize-v3 | build the v3 restricted paired evidence material under the exact plan/audit/release chain and fixed 16000-token selector budget |
| evalwitness relation packet-v3 | verify v3 material against its exact release case and typed firewall, emit the blinded packet, and exclusively store the owner-only v3 mapping |
| evalwitness relation study-amendment | bind the exact pilot and primary commitments to the frozen analysis, workload, missingness, stopping, and claim boundary |
| evalwitness relation translate | deterministically map complete normalized family observations to supports, contradicts, or unresolved |
| evalwitness relation validate | strictly validate one named relation artifact and emit its digest |
| evalwitness relation schema | emit one of 97 closed JSON Schema 2020-12 relation documents: 92 protocol documents comprising 31 immutable v1 contracts, 30 historical v2 contracts, 30 complete v3 workflow contracts, and one v3 scarcity-sentinel contract; the independent `scarcity-public-evidence` projection; the public `owner-inspection-public-attestation` projection; and three guided owner-inspection journal schemas |
| evalwitness agent-study build | build the coding-agent-only formal study from the exact v3 plan, natural audit, and release with deterministic 20/20 calibration/test selection |
| evalwitness agent-study validate | strictly decode the study, reselect its cases, rederive source audits, and bind every case to the supplied frozen release |
| evalwitness agent-study schema | emit the closed study schema identity and canonical policy |
| evalwitness cache stats | print cache size and entry count |
| evalwitness cache clear --scope responses\|capabilities\|all | remove only the selected schema-owned descendants |
| evalwitness replay migrate --source path [--candidate path] | preserve a legacy fixture and emit a non-exact inspection candidate with ambiguity report |
| evalwitness replay census-legacy-cache --source path --published-provider id | read-only exact structural census with public-safe identity gaps and inventory digests |
| evalwitness replay bundle seal-policy --source policy --repository-root path --producer-binary path --redistribution-evidence path --capture name=path | bind exact request corpus, producing source/binary/toolchain, rights evidence, capture set, and omission policy |
| evalwitness replay bundle build --policy policy --repository-root path --producer-binary path --redistribution-evidence path --destination path --capture name=path [--archive path] [--reviewed-findings path] | construct and no-overwrite publish a public exact-response capsule and deterministic archive; `--reviewed-findings` suppresses known false-positive safety findings by rule+fileSHA+line; omitting `--archive` selects `destination.tar.gz` |
| evalwitness replay bundle verify --source path [--reviewed-findings path] | verify capsule graph, public scan, embedded rights evidence, producer provenance, index, and exact replay payloads offline; the optional exact-content review list suppresses only matching rule+fileSHA+line findings |
| evalwitness replay capture-run attest --capture path --authorized-calls N [--output path] | inspect a schema-3 JSONL capture, reconcile the attempt ledger to the authorized call budget, and seal evalwitness.capture-run-attestation.v1 |
| evalwitness replay capture-run verify --capture path --attestation path | re-inspect the capture and reject digest, census, or status drift |
| evalwitness replay capture-run stamp --capture path --destination path --stamp path [--output path] | write a new JSONL with research-lineage overlay; refuse in-place rewrite of a sealed capture |
| evalwitness replay capture-run admit --capture path --authorized-calls N [--output path] | bind payload SHA-256 to evalwitness.capture-research-admission.v1; admitted only when capture-run status is complete |
| evalwitness replay study bind --capture path --authorized-calls N --attestation path --admission path --claim-ledger path [--bundle-policy path --study-record path --offline-analysis path --output path] | digest-bind capture-run, research admission, and parent hashes into evalwitness.identical-response-050-bind.v1; complete only when lineage is complete and all named parents are present; does not mint a TASK 050 capsule |
| evalwitness replay study portfolio --bind path --claim-ledger path [--output path] | one-screen claim chain from the bind certificate; explorer_present=false; does not raise the evidence ceiling |
| evalwitness calibration evaluate --observations path --threshold n --target-risk n --min-coverage n [--seed n] [--artifact path --route scope --domain scope] [--inventory path --root path] | evaluate held-out selective risk/coverage and IsDeployable on a test split; optional artifact forces unsupported on route/domain mismatch; optional development inventory rejects TASK 034 confirmatory leakage |
| evalwitness calibration seal --artifact path --calibrator path | checksum a model-artifact draft against calibrator bytes; reject leakage feature keys |
| evalwitness calibration verify --artifact path --calibrator path | reject calibrator checksum drift |
| evalwitness calibration apply --artifact path --route scope --domain scope [--calibrator path] | refuse route/domain mismatch; optional calibrator verify |
| verification.Result.fallback | kind=none, charged=false until a locked TASK 049 fallback policy exists; judge/human costs are reserved only through ChargeFallback |
| SelectiveDecision.fallback | explicit none/judge/human_review_handoff/no_action arm; judge and human require CostCalls>=1 |
| evalwitness calibration bind-049 --split path --study path | SHA-256 bind of committed 049 split and study bytes; has_test_role=false on the frozen identical-response assignment |
| evalwitness calibration bind-034 --inventory path [--root path] | verify development-inventory.json artifact bytes; freeze 589 historical tasks as development-only; confirmation_permitted=false |
| evalwitness registry validate-intake --entry path [--catalog path] | validate one offline registry intake entry; optional catalog JSON array rejects duplicate entry_id or capsule+profile pairs |
| evalwitness registry preflight --entry path [--catalog path] | same checks plus expiry and replay against the validator clock even without a catalog |
| evalwitness registry template | print a fill-in intake skeleton that is not a valid submission |
| evalwitness registry refresh --catalog path | offline expiry/replay refresh of every catalog entry; no live call |
| evalwitness registry review-checklist | emit the local maintainer pre-submit checklist; not community admission |
| evalwitness registry render-matrix --catalog path [--history path] | group valid intake entries by compatible request/schema/endpoint/score/top-k contract; optional history is append-only and never overwrites earlier members |
| evalwitness registry render-reliance --catalog path | group optional TASK 065 parents only when ontology, panel, estimator, intervention, outcome, and profile digests match; incompatible cells stay separate; no ranking |
| evalwitness registry index-scarcity --evidence path | decode and validate committed scarcity JSON; pin digest f401b845… and six public parents; package_format_commitment=evalwitness.relation-pilot-package.v5; never a verifier score |
| registry governance | intake writable status=format_verified; later statuses are governance-only; dispute->disputed, withdrawal->withdrawn, correction emits a new format_verified entry; rewrite_history is rejected |
| registry adversarial intake | archive name/magic rejected; payload >1MiB rejected; detached Ed25519 must verify; schema migration cannot rewrite signed history |
| evalwitness registry index-owner-inspection --attestation path | index the public owner-inspection attestation; 66 assessments / 16 dimensions; independently_reproduced and human_supported stay false |
| evalwitness registry render-method-lineage --autopsy path | project v1->v2, v2->v3, v3->frozen from AutopsyView; reject ranking and pooling |
| evalwitness registry index-empirical --attestation path | outcome_status=not_run; relation validity copied from the public owner-inspection aggregate; empirical=false |
| evalwitness registry inventory | configured preset inventory; capability_state=configured; no live call |
| evalwitness registry public-derivative --entry path | mechanism_conformance projection; omits credentials, local paths, and private payloads |
| evalwitness version | print version |

`evalwitness verify` flags:
| flag | type | default | description |
|---|---|---|---|
| --mode | string | pairwise | pairwise / absolute / delta |
| --task | string | - | inline string or @path/to/task.txt |
| --trajectory | string repeatable | - | inline or @file, 1+ for absolute, 2 for delta, 2+ for pairwise |
| --criteria | string | generic | comma-separated preset names or @path |
| --provider | string | from env | override EVALWITNESS_PROVIDER |
| --wire-format | string | from env | override EVALWITNESS_WIRE_FORMAT |
| --model | string | from env | override EVALWITNESS_MODEL |
| --base-url | string | from env | override EVALWITNESS_BASE_URL |
| --n-reps | int | 1 | reps per (pair, criterion) |
| --max-workers | int | 8 | concurrent provider calls |
| --epsilon | float | 0.02 | tie threshold for pairwise/delta |
| --no-cache | bool | false | disable disk cache |
| --paper-parity | bool | false | reference-parity pipeline: single-criterion prompts, no critique/bundling, single order, reps 4 |
| --judge-mode | bool | false | no logprob requests, raw-text extraction |
| --output | string | json | json / text |
| --verbose | bool | false | debug logging to stderr |

Live entrypoints expose an authorization digest argument (`--authorize` in the CLI, `authorization_digest` in MCP) and hard limit flags appropriate to their shape. `eval-terminal` and `eval-swebench` add `--max-calls`, `--max-attempts`, `--max-input-tokens`, `--max-output-tokens`, `--max-cost-usd`, `--max-duration`, `--study-record`, optional equality assertion `--study-manifest-digest`, `--design-alpha`, `--design-power`, `--design-alternative-q`, `--minimum-effect`, `--equivalence-margin`, `--inference-question`, `--primary-family-size`, and `--disagreement-rates`. Zero selects the conservative execution limit where supported. Every eval preflight, including dry-run, prints and embeds total/decidable counts, exact paired-design sensitivity, best/expected/worst calls, estimated input tokens, reserved output tokens, duration, and resolved hard limits. Live execution validates the authorized study record and exact execution binding before emitting or accepting the live authorization digest.

## Statistical Design

Decidability: `DecidableBinary(outcomes)` = `len(outcomes)>=2` AND all outcomes in `{0,1}` AND at least one `0` AND at least one `1`; one implementation in `internal/stats` is consumed by eval, baseline, reliability, and paired-audit paths.

Exact paired design:
| field | definition |
|---|---|
| discordant | `d=b+c` |
| exact p | `min(1, 2 * sum(BinomialPMF(d,i,0.5), i=0..min(b,c)))` |
| rejection region | all `b in [0,d]` with exact p `< alpha` |
| conditional power | sum `BinomialPMF(d,b,q)` over rejection region; `q` is declared before outcomes |
| unconditional power | sum over `d ~ Binomial(decidable_tasks, disagreement_rate)` of conditional power |
| paired effect | `disagreement_rate * (2q-1)` |
| MDE | smallest paired effect reaching target unconditional power; absent when complete separation remains under target |
| multiplicity | nominal alpha and `alpha/family_size` Bonferroni requirement reported separately |

`evalStatisticalPlan`:
| field | type | description |
|---|---|---|
| total_tasks | int | complete benchmark task count |
| decidable_tasks | int | decision-informative count |
| decidable_share | float64 | decidable / total |
| question | superiority / non_inferiority / equivalence | typed primary question |
| margin | float64 | locked positive margin for non-inferiority/equivalence |
| minimum_effect | float64 | locked task-level effect requirement |
| nominal_alpha / family_adjusted_alpha | float64 | unadjusted and Bonferroni-adjusted thresholds |
| family_size | int | primary comparison family |
| target_power | float64 | prospective power target |
| discordant_win_probability | float64 | declared alternative q |
| disagreement_sensitivity | []evalPowerRow | rates, expected discordance, exact powers, MDEs, complete-separation powers |
| warnings | []string | impossible/underpowered effect or unresolved-margin diagnostics |

Completed paired output:
| field | definition |
|---|---|
| paired_effect | `(subject_only-comparator_only)/paired_tasks` |
| interval | Newcombe paired score method 10; confidence `1-adjusted_alpha` for superiority and `1-2*adjusted_alpha` for one-sided non-inferiority/equivalence bounds |
| mcnemar_p | exact two-sided conditional p |
| smallest_significant_subject_wins | upper rejection boundary at family-adjusted alpha; absent when no split rejects |
| design_resolution | textual boundary statement, never labeled power |
| inference | typed question, margin, alpha, interval, established, conclusion |

Inference: superiority established iff exact p `< adjusted_alpha` AND interval lower `>0`; non-inferiority established iff margin `>0` AND interval lower `>-margin`; equivalence established iff margin `>0` AND interval lower `>-margin` AND interval upper `<margin`. Completed results never contain `observed_power`, `achieved_power`, or `post_hoc_power`.

Exact controlled-relation design:
| field | definition |
|---|---|
| independent unit | one unique source task within one mutation family |
| null | relation success probability `p0` |
| alternative | prespecified `p1`, with `p1>p0` |
| rejection boundary | smallest successes `k` whose exact upper tail `P(Binomial(n,p0)>=k)` is strictly below alpha |
| actual null tail | exact upper-tail probability at `k`, never replaced by nominal alpha |
| power | `P(Binomial(n,p1)>=k)` |
| multiplicity | family-adjusted alpha supplied before boundary calculation |

`ExactBinomialUpperDesign(n,p0,p1,alpha)` requires `n>=1`,
`0<=p0<p1<=1`, and `0<alpha<1`; it emits the critical successes, actual
null tail, and exact alternative power. The controlled-corruption v1 design uses
`n=40`, `p0=0.5`, `p1=0.8`, and `alpha=0.05/8`.

`ClusterSimulationSpec`:
| field | type | constraint |
|---|---|---|
| source_tasks | int | >=2; cluster unit |
| mutations_per_cell / replications | int | >=1 |
| seed | int64 | emitted unchanged |
| code_digest | string | required 64-character SHA-256 implementation digest |
| endpoint | binary_decision / continuous_distribution | binary logistic-random-intercept or continuous Gaussian-random-intercept generator |
| baseline / residual_sd / intracluster_correlation | float64 | endpoint parameters; ICC in `[0,1)` |
| factors | []FactorEffect | 1..10 unique factor IDs for enumerated designs; 1..63 for coded designs; finite declared effects |
| interactions | []InteractionEffect | unique IDs, >=2 known non-repeated factors |
| coded_design | CodedFactorialDesign or absent | algorithm `evalwitness.walsh-coded-factorial.v1`; power-of-two runs in `[2,1048576]`; exactly one unique nonzero mask below runs per factor |
| sparse_cell_fraction | float64 | `(0,1]` |
| invalid_rate / missing_rate / abstention_rate / route_failure_rate | float64 | each `[0,1)`; sum `<1` |
| alpha / family_size | float64 / int | alpha `(0,0.5)`; family >=1 |
| calls_per_source_task / input_tokens_per_source_task | int | exact fixed cluster-level resource coefficients |
| calls_per_observation / input_tokens_per_observation | int | exact observation-level resource coefficients |
| hard_calls / hard_input_tokens | int | fail closed when positive ceiling is exceeded |

Cluster simulation: enumerate and deterministically freeze sparse assignments from seed when `coded_design` is absent, otherwise require `sparse_cell_fraction=1` and generate the exact Walsh rows from run-index/mask parity -> compute design rank and aliased terms -> include source-task and observation resource coefficients -> reject hard-budget overflow -> for each replication generate one shared random intercept per source task -> preserve invalid/missing/abstention/route-failure denominators -> fit declared factorial terms -> compute source-task-cluster sandwich covariance -> test each estimable term at `alpha/family_size` -> emit mean estimate, empirical power, `sqrt(power*(1-power)/valid_runs)` Monte Carlo SE, effective source-task count, seed, algorithm digest, code digest, and exact budget. Sensitivity scenarios derive seed as `base_seed + scenario_index*1000003`. Grid inverse design returns the first declared effect meeting target power.

`ReliancePreflight`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.reliance-preflight.v1`; canonical policy, algorithm `evalwitness.reliance-walsh-power-search.v1`, frozen preregistration digest, caller-supplied code digest, and self-digest |
| alias audit | 11-factor design; 16 runs rejected by the eight-element sum-free bound; all 135408 sum-free 32-run layouts enumerated with zero qualifying interaction layouts; 64 runs selected with every main effect clear of every two-factor term and each of four declared interactions on a unique pair column |
| simulation | candidate source-task counts `8,12,16,20,24,32,40`; 64 exact Walsh rows per task; 256 fixed-seed replications; ICC `0.25`; retained invalid/missing/abstention/route-failure rates `0.05/0.03/0.05/0.02`; family size `98`; target power `0.80` |
| effects | continuous residual SD `0.15` and declared mean difference `0.05`; binary baseline `0.20` and declared log-odds contrast `0.75`; one exact-symmetry sentinel per main-effect and interaction class with complete `applies_to` membership |
| selection | first ascending candidate whose four sentinel checks each have 95% Wilson lower power bound >=`0.80`; selected source-task count `24` |
| MDE | at 24 source tasks, continuous grid `0.02,0.03,0.04,0.05` selects `0.04`; binary log-odds grid `0.25,0.50,0.75` selects `0.75`; both sentinel classes must resolve at the selected grid point |
| resource model | subscription billing; one baseline call per source task; one call per Walsh cell; 32000 evidence plus 4000 prompt tokens per attempt; 4096 maximum output tokens; five retries; 120-second attempt timeout; concurrency eight; zero marginal subscription cost |
| selected hard envelope | 1560 logical calls; 9360 attempts; 336960000 input tokens; 38338560 output tokens; 140400 seconds; concurrency eight; zero marginal subscription cost |
| search hard ceiling | 2600 logical calls; 15600 attempts; 561600000 input tokens; 63897600 output tokens; 234000 seconds; concurrency eight; zero marginal subscription cost |
| claim boundary | `live_authorized=false`; `empirical_assumptions=false`; no provider/network invocation, empirical result, model/provider comparison, or execution authority |

`EvidenceIntervention`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.evidence-intervention.v1`; canonical policy; intervention ID `intervention-<digest>`; self-digest |
| frozen parents | exact ontology, estimand-catalog, factor-assignment-set, source-trajectory, and intervened-trajectory digests |
| intervention | one known factor; one allowed retain, remove, typed-mask, or controlled-replacement operator; one estimand family; unique canonical target order |
| target | parent and intervened event IDs, event kind, field path, field value kind, exact before/after state digests, and changed flag |
| derivation | relation `evidence_reliance_intervention`; validator `evalwitness.evidence-intervention-validator.v1`; exact changed parent-event IDs and `/events/<parent-event-id><field-path>` values; canonical reference-link remapping |
| outcome | absent or one valid `outcome.Preservation` from mechanism `evalwitness.evidence-intervention-outcome.v1`; source and intervened records are supplied together |
| admissibility | `admissible`, `inadmissible`, or `unresolved`; exact typed reasons; denominator eligibility iff admissible |
| execution boundary | preserved source digest; declared pre-execution assignment stage; zero provider calls; no network requirement |

Application MUST validate the frozen ontology, estimand catalog, assignment set,
factor, operator, targets, field presence, and exactly typed replacement before
changing a deep copy. Each changed event MUST be rebuilt once through
`preprocess.RebuildDerivedEvent`; reference links MUST be remapped; the complete
child MUST be sealed through `preprocess.DeriveTrajectory`. Validation MUST
reproduce target states, unchanged event projections, untouched events,
derivation, outcome status, admissibility, denominator eligibility, and digest.
A non-retain target containing an event reference MUST fail with
`dependency_closure_required` until relation-backed closure is proven. A retain
control MUST preserve every target and emit no changed event or field. Every
other operator MUST produce a real evidence change.

Evidence-only interventions are admissible only when a paired decisive outcome
record preserves task and outcome. Missing or nondecisive outcome evidence is
unresolved; changed outcome is inadmissible. Quality-changing interventions with
preserved outcome are inadmissible; changed outcome is unresolved with
`relation_admission_required` until the TASK 068 construct chain is verified.
Assignment-stage declaration MUST NOT be represented as wall-clock freeze proof.
The frozen operator-target matrix contains 89 combinations and MUST be exercised
through the canonical application and validation path.

`FactorTreatmentPlan`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.factor-treatment-plan.v1`; reliance canonical policy; ID `treatment-plan-<digest>`; self-digest |
| frozen parents | exact ontology, estimand-catalog, factor-assignment-set, and source-trajectory digests; evidence-only or quality-changing estimand family |
| coverage | exactly one sorted non-retain treatment for each of the ten ontology factors; each treatment covers every frozen assignment for that factor exactly once; targets are globally disjoint |
| presentation | policy `evalwitness.dependency-valid-presentation-order.v1` |
| execution boundary | declared pre-execution assignment stage; zero provider calls; no network requirement |

`EvidenceInterventionCell`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.evidence-intervention-cell.v1`; reliance canonical policy; caller-supplied canonical cell ID; self-digest |
| frozen parents | exact preregistration, treatment-plan, ontology, estimand-catalog, assignment-set, source-trajectory, and intervened-trajectory digests |
| levels | exactly the ten preregistered factor IDs plus `presentation_order`; unique sorted levels coded `-1/+1` |
| composition | negative levels apply the frozen factor treatment; positive levels retain; every affected event is rebuilt once after all disjoint field changes; one child is derived with relation `evidence_reliance_factorial_cell` and validator `evalwitness.evidence-intervention-cell-validator.v1` |
| evidence | exact active factors, factor/operator/target tuples, changed event IDs, changed field paths, outcome preservation, admissibility, reasons, and denominator status |
| control | an all-positive evidence-factor cell preserves the complete parent trajectory; presentation level remains a render-only term |
| execution boundary | preserved task identity; declared pre-execution assignment stage; zero provider calls; no network requirement |

`PresentationOrderPlan`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.presentation-order-plan.v1`; reliance canonical policy; policy `evalwitness.dependency-valid-presentation-order.v1`; exact cell, assignment-set, and trajectory digests; self-digest |
| available | exact narrative event IDs; deterministic narrative-first and narrative-last topological permutations; exact rendered-text digests; every trajectory link remains forward in both orders |
| unsupported | exactly one reason: `narrative_factor_absent`, `dependency_cycle`, or `no_dependency_valid_order_contrast`; no rendered order is executable |
| rendering | negative `presentation_order` selects narrative-first; positive selects narrative-last; `preprocess.RenderTrajectoryInOrder` validates the complete unique permutation and all dependencies |
| execution boundary | canonical graph unchanged; zero provider calls; no network requirement |

`ReplayBatchEvidence`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.stress-replay-batch-evidence.v1`; stress canonical policy; common entrypoint and evidence policy; exact batch-run fingerprint; self-digest |
| request | one or more ordered unique labels and exact input digests; every caller input is deep-snapshotted; cache disabled; live authorization and persistent budget state absent |
| shared controls | inputs are identical except trajectory, study variant, and study-cell identity; a study manifest requires both variant and cell identity on every item |
| evidence | one unique plan fingerprint, complete verification result, score-call observation set, and six-stage trace per label; every item revalidates against its requested input |
| provenance | every observation is exact replay and binds the same validated capture source, provider route, requested model, request identity, and sampling slot; no replay-to-live fallback |

The canonical two-item `ReplayPairEvidence` and relation-backed
`ReplayExecution` MUST be projections of `ReplayBatchEvidence`; they MUST NOT
execute a second replay engine.

`EvidenceTaskPanelExecution`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.evidence-intervention-task-panel.v1`; reliance canonical policy; exact source-task, preregistration, treatment-plan, assignment-set, source-trajectory, study-manifest, outcome-evidence-set, batch-run, and capture-source identities; self-digest |
| panel | one baseline plus exactly 64 ordered cells matching Walsh rows `0..63`; every cell is denominator-eligible, has an available validated presentation plan, and binds its exact level vector, cell digest, and presentation digest |
| input | absolute mode; exactly one criterion; exactly one fixed repetition; baseline trajectory is canonical source rendering; cell trajectory is the selected dependency-valid rendering; common offline controls and treatment-plan lineage |
| resource | exactly 65 logical calls per source task; one shared verification batch; baseline is not repeated per cell |
| score evidence | full baseline and intervention `ScoreEvidence`; one `verifier.CompareScoreEvidence` per criterion and repetition; decision flip and abstention transition; input, plan, observation-set, and stage-trace digests |
| determinism | artifact excludes elapsed wall time; exact replay reproduces the artifact byte-for-byte; validation reconstructs every cell and contrast from frozen parents plus the requested batch |
| claim boundary | exact-replay execution mechanism only; no live authorization, empirical reliance result, model comparison, or population claim |

`ReliancePanelRegistration`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.reliance-panel-registration.v1`; reliance canonical policy; exact study-manifest, preregistration, and resolved preflight digests; self-digest |
| arm | one entrypoint, criterion ID, score tag, evidence policy, provider ID, route ID, and requested model; cross-arm pooling forbidden |
| assignment | algorithm `evalwitness.reliance-walsh64-panel.v1`; exactly 24 unique sorted source-task registrations; each binds source-task ID plus exact source-trajectory, assignment-set, treatment-plan, and outcome-evidence-set digests; no exact artifact digest may be reused across task clusters; digest uniqueness does not prove semantic or provenance independence; 64 cells per task; 1536 registered cells; 1560 planned logical calls |
| chronology | freeze stage `before_verifier_output`; chronology status `declared_not_timestamp_proven`; no timestamp or ordering claim without a later external proof |
| sealing boundary | `sealing_provider_calls=0`; `sealing_network_required=false`; registration is not execution authorization |

`RelianceCellFailureReceipt`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.reliance-cell-failure-receipt.v1`; reliance canonical policy; exact registration, study-manifest, and preregistration digests; self-digest |
| cell | one registered source task; index `0..63`; exact frozen Walsh row for that index |
| status | one preregistered retained post-randomization status except `abstained`; `measured` forbidden |
| evidence | nonempty evidence schema version plus SHA-256 evidence digest; receipt binds accounting lineage and does not independently prove status semantics |
| resource | cell-attributed logical calls `0..1`; retries and other resource evidence remain in the referenced evidence artifact |
| outcome | no imputed score, distribution, decision, or transition outcome |

`RelianceAnalysisCorpus`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.reliance-analysis-corpus.v1`; reliance canonical policy; exact registration, study-manifest, preregistration, preflight, and analysis-arm identities; self-digest |
| coverage | all 1536 registered cells exactly once; a valid task panel covers all 64 cells for its source task; otherwise each uncovered cell requires one valid failure receipt; omissions and duplicates rejected |
| order | registration source-task order, then cell index `0..63`; every level vector equals the frozen Walsh row |
| outcomes | measured and abstained panel cells contain the exact seven preregistered values derived from full score-evidence comparison and decision state; all other cells contain zero outcome values |
| missingness | all statuses remain in denominator counts; `imputation=none`; only outcome-bearing cells enter a fit |
| arm isolation | panel entrypoint, criterion, score tag, evidence policy, provider, route, and requested model equal the registration arm |
| accounting | canonical count for all 11 analysis statuses; completed-panel logical calls equal `65*panel_executions`; failure-attributed calls bounded by failure receipts |

`EvidenceRelianceAnalysis`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.evidence-reliance-analysis.v1`; reliance canonical policy; exact registration, corpus, and preregistration digests; self-digest |
| family | ten main effects plus four preregistered interactions over each of seven outcomes; Bonferroni method and exact family size `98` |
| estimator | `stats.FitClusteredFactorial`; source-task cluster sandwich covariance; nominal alpha `0.05`; complete Walsh levels retained per eligible observation |
| denominator | each outcome reports 1536 registered cells, eligible observations, and excluded cells; failure and missing cells are never removed from denominator reporting or imputed into fitting |
| measured | complete clustered fit; observation count equals eligible count; 15 parameters including intercept; full rank; family size `98` |
| inconclusive | valid zero-observation, rank-deficient, singular, insufficient-observation, or fewer-than-two-cluster design; no fit; exact stable reason; unexpected estimator errors abort construction |
| validation | reconstruction from the registration, preregistration, complete task-panel set, and failure-receipt set MUST reproduce the corpus, all seven fits, and analysis digest exactly |

`EvidenceSelectorAudit`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.evidence-selector-audit.v1`; reliance canonical policy; policy `evalwitness.evidence-selector-audit-policy.v1`; exact registration, analysis, preregistration, preflight, and source-artifact-set digests; self-digest |
| production path | selector `CanonicalPipeline/ApplyEvidenceBudget`; event-score policy `evalwitness.evidence-event-score.v1`; renderer `preprocess.RenderTrajectory`; exact production `ApplyEvidenceBudget` execution for every source and fixed budget |
| source coverage | exactly the registration's 24 sorted source tasks; each source trajectory, assignment set, and treatment plan validates against the frozen ontology and estimands and equals its registered digests; exactly 240 assignment targets |
| budgets | exact ordered token budgets `16384`, `32768`, `65536`; every factor target classified once per budget as rendered-exact, rendered-changed, unrendered within a retained event, or event-dropped |
| bytes | raw assigned canonical-event bytes and assigned rendered-event bytes reported separately before and after selection; both are unique within factor/task/event and non-additive across factors sharing an event |
| effect mapping | rule `any_declared_term_outcome_adjusted_p_lte_alpha`; alpha `0.05`; every factor carries all declared main or interaction term estimates for all seven outcomes; `adjusted_effect_detected` iff any available adjusted p-value is at most alpha; `no_adjusted_effect_detected` is not zero effect or equivalence; any inconclusive parent fit makes the factor status inconclusive |
| risk flags | detected effect plus changed, unrendered, or dropped target; no detected effect plus rendered target retention; or inconclusive analysis with no selector-alignment claim; the audit MUST NOT tune selector weights or budgets from analyzed outcomes |
| category survival | deterministic original and retained event and raw-byte counts for every observed event kind and fixed budget |
| legacy boundary | `Pipeline/EvidenceSlice` and `evalwitness.legacy-evidence-line-score.v1` recorded only as six digest-and-score probes with status `legacy_pipeline_only_not_production_verifier_path` |
| execution boundary | `provider_calls=0`; `network_required=false`; deterministic mechanism and frozen-analysis audit only; no external-model, population, causal-mechanism, or empirical selector-map claim |

`RelianceArmComparison`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.reliance-arm-comparison.v1`; reliance canonical policy; contrast policy `evalwitness.reliance-arm-contrast-policy.v1`; exact preregistration and preflight digests; self-digest |
| arm | unique arm ID; exact registration, corpus, and analysis digests; analysis-arm identity; model-family ID; identity status `alias_only` or `named_family_evidence_bound`; route-attestation digest |
| contrast matrix | one or more unique prespecified specs by ID and kind/reference/comparator tuple; kinds `evidence_policy`, `entrypoint`, `route`, `provider`, or `model_family`; canonical contrast order; direction `comparator_minus_reference` |
| support | actual changed dimensions MUST equal the kind contract exactly: `evidence_policy`; `entrypoint`; `route`; `provider,route`; or `model_family,requested_model,route`; zero-change and mixed contrasts are typed `unsupported` and have no fit |
| response pairing | an evidence-policy contrast requires identical validated `provider.ExactReplaySource` values for every source task with panels in both arms; different present response-record captures make the entire contrast unsupported; one-sided or jointly missing panels remain denominator states |
| cell pairing | exact source task, cell index, Walsh levels, intervention-cell digest, and presentation digest; statuses `both_missing`, `comparator_only`, `paired`, `reference_only`; only paired outcome-bearing cells enter fitting; no imputation |
| estimator | comparator-minus-reference outcome per cell; all seven preregistered outcomes; frozen 14 terms; source-task cluster sandwich; typed inconclusive non-estimability; registered, eligible, and excluded pair counts retained |
| multiplicity | Bonferroni method; family size `98 * count(all prespecified contrast specs)`, including unsupported specs |
| model identity boundary | model-family fitting requires `named_family_evidence_bound` for both arms; `alias_only` is unsupported; the comparison validates the digest binding but TASK 036/TASK 045 MUST validate attestation contents and named-family identity before any external claim |
| validation | reconstruction from all frozen arm evidence and the exact contrast matrix MUST reproduce arm summaries, pairing counts, fits, unsupported reasons, and digest |
| execution boundary | `provider_calls=0`; `network_required=false`; deterministic comparison mechanism only; no route, provider, model-family, entrypoint, or transfer claim without separately admitted empirical arm evidence |

`RelianceWitness`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.reliance-witness.v1`; reliance canonical policy; evaluation policy `evalwitness.reliance-witness-preservation.v1`; exact admission, relation, case, source-replay, source-batch, source-capture, outcome-identity, and original semantic-result digests; self-digest |
| reducer | MUST call `stress.ReduceCounterexample` through the existing relation-backed reduction boundary; complete embedded `stress.Counterexample`; deterministic restart-greedy one-minimality over declared units only; no parallel minimizer or global-minimum claim |
| semantic result | digest over relation, case, task group, outcome, invalid state, exact constraint observations, distribution comparisons, and planned/completed repetitions; full replay/result parents retain stage, admission, execution, and accounting provenance |
| evaluation | schema `evalwitness.reliance-witness-evaluation.v1`; exact candidate input digest, candidate replay-batch fingerprint, unchanged exact-capture digest, intervention-validity proof digest/status, outcome-identity digest, semantic-result digest, complete stress reduction observation, derived status/reasons, self-digest |
| original | exact counterexample original input/observation; admitted intervention digest; source replay digest; source batch fingerprint; status `preserved` |
| candidate replay | every changed candidate MUST use a non-source batch fingerprint and non-source replay-result digest while retaining the exact source capture; source execution identity reuse is invalid |
| preservation | `preserved` iff relation and privacy revalidate, intervention is valid, outcome identity equals the frozen source outcome, and semantic result equals the original; otherwise `unresolved` with ordered closed reasons; stress `violation_preserved` MUST equal derived preservation |
| trace | one evaluation for the original plus one per counterexample step; accepted steps MUST be preserved; rejected steps MUST be unresolved; final evaluation MUST be the last accepted preserved input and bind the reduced-input digest; final units equal the counterexample final units |
| proof boundary | intervention-validity, relation, and privacy proof digests are exact oracle evidence pointers; witness validation verifies integrity, identity, and decision consistency but does not recreate unavailable private proof contents; capsule admission owns public proof availability |
| execution boundary | exact-replay source required; every evaluation and witness has `network_required=false`; no provider/model causality, global minimality, or public empirical claim |

`RelationBackedInterventionAdmission`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.relation-backed-intervention-admission.v1`; reliance canonical policy; self-digest |
| intervention | exact intervention and assignment-set digests, factor, operator, estimand family, intervention admissibility, and intervention reasons |
| relation | exact stress relation ID/digest, relation case ID, V3 manifest digest, and construct-admission digest |
| construct evidence | formal-witness, construct-firewall, owner-attestation, optional terminal-ledger, optional human-resolution digests, and exact construct status |
| graph equivalence | original and transformed `evalwitness.relation-event-graph.v1` digests over trajectory schema, source format/digest, canonical events, links, and ingestion report |
| admission | combined admissibility and typed reasons; relation-specific primary and sensitivity eligibility |
| execution boundary | frozen assignment declaration; zero provider calls; no network requirement; no verifier result input |

Binding MUST validate the sealed intervention, stress relation, construct
admission, and exactly one trajectory-level V3 replay. The intervention parent
MUST equal the replay source. The intervention child and replay transform MUST
have identical event-graph digests while retaining their distinct derivation
metadata. Relation IDs MUST be unique and sorted; case, family, relation,
manifest, and task-group identities MUST be complete and cross-bound. The replay
MUST bind mutation-program version V3, relation-contract version V3, the exact
formal-witness digest, and the exact construct-firewall digest. Any missing or
cross-version value MUST fail before admission or denominator accounting.

Human-supported status MAY admit a complete evidence-only intervention or
resolve only a quality-changing intervention whose sole reason is
`relation_admission_required`. Formal-only and human-unresolved status MUST
remain unresolved sensitivity strata. Human contradiction MUST be inadmissible.
Missing, nondecisive, or otherwise inadmissible outcome evidence MUST NOT be
promoted by construct status. Primary-core relations MUST reject formal-only
admission. Validation MUST reconstruct the complete artifact from its frozen
parents; construct status MUST NOT alter assignment or intervention bytes.

Historical construct controls MUST be decoded from the sealed construct-repair
evidence. The exact cases `generic_completion_evidence_role`,
`pathological_executable_text_presentation`, and
`shared_tool_transaction_reorder` MUST remain `invalid_cross_version` because
their closed defects are respectively `unverified_evidence_role`,
`unnatural_formatting`, and `transaction_dependency`. They MUST NOT enter a
reliance contrast, estimator, reducer, or denominator and MUST NOT establish V3
human validity.

Relation-backed execution MUST use `stress.ReplayFirstRunner`. Each side MUST
contain the exact `preprocess.RenderTrajectory` projection of the admitted V3
replay, the mode implied by trajectory cardinality, a nonempty study-manifest
digest, variant `original` or `transformed`, the intervention ID as study-cell
ID, the admitted case and relation IDs, and the replay outcome-evidence digest.
Cache MUST be disabled; live authorization and persistent budget state MUST be
absent. The sides MUST otherwise be identical. The returned replay MUST validate
against the relation and MUST seal through `stress.SealReplayResult` with the
exact construct admission and task group.

Relation-backed reduction MUST use `stress.ReduceCounterexample`. It MUST first
revalidate the complete relation execution from its frozen parents, use the full
replay digest as `SourceResultDigest`, preserve the admitted relation and case
IDs, and require the original reduction observation to bind that replay digest.
Cross-version admission MUST fail before the oracle is evaluated. A separate
execution engine or minimization algorithm is forbidden.

`OwnerInspectionCustodyGate`:
| field | invariant |
|---|---|
| identity | schema `evalwitness.relation-owner-inspection-custody-gate.v1`; relation canonical policy; self-digest |
| public parent | exact public-attestation and package-inventory digests; exact passed aggregate state and pinned ten-claim boundary |
| private parents | exact session, inspection-record, and completion digests reproduced through `VerifyOwnerInspectionPublicAttestation` over the complete private chain |
| denominator | 66 required and completed assessments; journal-event and correction counts; 16 aggregate dimensions |
| status | core, scarcity, and overall passed; every dimension passed with zero failed or indeterminate assessments |
| authority boundary | formal-human ledger absent; primary admission false; execution authorization false |
| execution boundary | zero provider calls; no network requirement; contains private evidence identities and MUST remain private |

Gate construction MUST reject revision-required or unresolved outcomes, any
failed or indeterminate dimension, changed package/private-chain parent,
resealed status promotion, claim promotion, incomplete or duplicate claim
inventory, and digest-recomputed authority promotion. A passed gate proves local
owner-custody verification only. It MUST NOT substitute for a terminal formal-
human ledger, verifier result, study authorization, caller authorization, or
public reproduction of withheld evidence.

## Study Governance

Study schemas: manifest `evalwitness.study-manifest.v1`; locked study
`evalwitness.locked-study.v1`; record `evalwitness.study-record.v1`; split
`evalwitness.study-split.v1`; canonical policy
`evalwitness.study-canonical-json.v1`; split algorithm version 1 for task-group
stratified hashing; historical inventory
`evalwitness.development-data-inventory.v1`. Strict JSON input maximum 16777216
bytes; unknown fields and trailing JSON reject. `study schema` emits closed JSON
Schema 2020-12 with required fields and enum/constant identities.

`StudyManifest`:
| section | required contract |
|---|---|
| identity | title, research question, study kind, unique authors, creation time, non-earlier lock time |
| hypotheses | primary null and alternative, unique secondary hypotheses, unique exploratory registry |
| data | task primary unit; one or more unique dataset IDs with source/version/license/acquisition, dataset/task/label/trajectory digests, task count, permitted roles, access state, exclusions; exact split |
| arms | unique arm ID; entrypoint, route, provider, requested model, prompt/request/calibration/attestation digests, score policy, selection, candidates, repetitions |
| outcomes | unique primary/secondary/exploratory endpoints; metric, direction, typed question, margin, risk, coverage, denominator |
| inference | test, interval, design method/evidence/power, task cluster, alpha, target power, minimum effect, disagreement model, decidable N, Bonferroni families, sequential rule |
| failures | explicit missing-score, provider, route, timeout, abstention, budget, retry, incomplete-cell and denominator treatments |
| controls | random selector, task-independent selector, positive control from development or calibration |
| providers | exactly one plan per arm with exact attestation observation/expiry, served-model and checkpoint observation policies/values, retry policy version, maximum retries and request timeout |
| budget | expected and hard calls, attempts, tokens, duration, concurrency and cost |
| execution | clean commit, binary, platform, analysis command/version/digest, ordered declared input paths/digests and route IDs |
| publication | visibility, allowed claim IDs, caveats, independent-reproduction gate and registered-report timestamp |
| reliability | current protocol version plus corpus/request/schema digests, current trace mapping, outcome/adjudication/profile digests; applicable relation/validator or factor/intervention digests |
| kind-specific | exactly one applicable block: real-agent corpus governance; controlled relations with corpus/relation-contract versions, source-task cluster, typed claim and validator digests; or evidence-reliance design with a complete effect/interaction multiplicity family |
| adjudication | strata, blinding, agreement, conflict resolution, label revision and sensitivity analysis |

Data roles: `development`, `calibration`, `test`, `external_replication`,
`unavailable_for_confirmation`. Previously accessed datasets cannot permit test
or external replication. Each `SplitAssignment` binds `dataset_id`, `group_id`,
role, ordered stratum and task/repository/clone/corpus/pair-observation identities
plus trajectory, mutation, evidence, adjudication and counterexample SHA-256 identities. Split
generation hashes declared seed, dataset, stratum and group, allocates integer
weights by largest remainder per dataset/stratum, sorts output by group ID and
seals the manifest. Set and hash inputs use unsigned 64-bit length-prefixed
identity frames, so embedded separators cannot alias. Validation rejects
cross-group identity reuse, invalid descendant digests, undeclared datasets or
roles, per-dataset count mismatch, task-ID or trajectory-set digest mismatch,
and decidable N above locked N.

TASK 072 governed corpus artifacts: dataset manifest
`evalwitness.identical-response-dataset-manifest.v1` binds the
controlled-corruption-v3 governed release digest, task-ID / outcome-label /
trajectory-set SHA-256 digests, task count, and permitted roles restricted to
`development` + `calibration` because previously accessed data cannot permit
confirmatory roles. Frozen split record
`evalwitness.identical-response-frozen-split.v1` records the frozen
controlled-corruption-v3 role assignment (60 development / 40 calibration)
instead of regenerating one; near-duplicate IDs are unique per group and every
lineage cluster is role-pure. Failable gates live in
`internal/study/corpus_072_test.go`.

Inference design method: primary superiority requires
`exact_mcnemar_unconditional`; validator recomputes power at the locked minimum
effect and family-adjusted alpha with tolerance 1e-12. Primary non-inferiority or
equivalence requires `paired_joint_outcome_simulation` and a SHA-256 design
evidence identity; declared power must meet target. Every confirmatory endpoint
belongs to exactly one declared family; exploratory IDs are inadmissible.
Non-sequential design is `fixed_sample`, one look, no boundary digest. Sequential
design requires a named method, SHA-256 boundary and at least two looks.

Lifecycle: canonical manifest JSON -> SHA-256 ->
`study_id=study-<manifest_digest>` -> lock event -> sealed record. Every later
event binds previous state, next state, strictly later UTC time, actor, reason
and previous record digest. Authorization also binds the exact set of one locked
attestation digest per arm. Allowed transitions: `locked -> authorized|withdrawn`;
`authorized -> running|failed|withdrawn`; `running ->
complete|failed|withdrawn`. Record validation replays and rehashes the complete
prefix chain.

`ExecutionBinding` exact equality fields: entrypoint, route, request contract,
commit, dirty=false, binary and analysis digests, analysis command/version,
ordered input paths/digests, expected/hard budget, decidable N, alpha, target
power, minimum effect, disagreement rate, discordant win probability, power at
minimum effect, primary-family size, retry policy/version/count and request
timeout. Current attestation validation also requires the exact locked
observation time, expiry, served-model string, checkpoint assertion and source.
`VerifyDeclaredInputs` recomputes
SHA-256 over repository-relative path names and bytes recursively in sorted
order with explicit file/directory markers, including empty directories;
absolute/traversing paths, symlinks and non-regular entries reject. Publication
claim IDs must name registered primary or secondary endpoints, and the
registered-report timestamp must be within the manifest creation-to-lock window.

The committed development inventory verifies every named historical artifact
digest, unique task-ID union, artifact-set digest, exact task count,
`role=development`, and `confirmation_permitted=false`. The protocol report is
derived only from a validated record and contains study identity, hypotheses,
data, arms, locked inference, failures, execution, controls, publication and
reliability contracts; it contains no observed result claim.

## Best-of-N Orchestration

Command: `evalwitness bon -n N --task T [flags] -- <literal command...>`; N=2..10. Default source mode requires a clean worktree and uses the exact current commit. `--include-working-tree` explicitly snapshots tracked and allowed untracked content through an alternate index, verifies the tree did not change during capture, and does not modify the user index. Non-persistent destination and attempt snapshots use temporary `GIT_OBJECT_DIRECTORY` storage with the repository object store as read-only alternates; only an explicit working-tree source snapshot may create persistent detached Git objects.

Attempt execution contract:
| control | invariant |
|---|---|
| command | operator-supplied literal argv; secret-pattern values and literal values of passed secret environment variables reject |
| environment | HOME, LANG, LC_ALL, PATH, SHELL, TERM, TMPDIR, USER plus explicit `--pass-env` names and `EVALWITNESS_BON_ATTEMPT` |
| cwd | one detached attempt worktree |
| lifecycle | parent context and timeout own the complete child process group; cancellation waits for termination |
| transcript | bounded tail, dropped-byte count, secret-pattern and explicit-env-value redaction, maximum 4194304 bytes on persisted read |
| diff | full binary working-tree delta through an alternate index/object database; hard rejection above 67108864 bytes |
| artifact | exact `attempt-N.diff` or `attempt-N.transcript`; regular non-symlink file; mode 0600; create-exclusive, checked write/sync/close, digest bound |
| cleanup | worktree removed after verified capture unless `--keep`; failure reports exact retained path |

Live flow is two-phase. Phase one authorizes and runs the child attempts, then creates owner-only `evalwitness.best-of-n-run.v1` `run.json`. Phase two uses `--resume-run PATH --authorize DIGEST`; it strictly decodes an owner-only regular manifest no larger than 8388608 bytes, verifies attempt indices, exact artifact locations, modes, bounds, and digests, rechecks destination state, then runs pairwise selection through the unified application service. The run directory persists as the immutable resume/evidence boundary.

Apply contract: `--apply` requires selection state `selected`, nonempty patch, explicit apply authorization, and an unchanged destination digest over HEAD, content tree, index tree, and status. Print `git apply --stat` -> require `git apply --check` -> apply without `--index` -> verify the index tree is byte-identical to its pre-apply value. Any conflict, concurrent destination change, artifact mismatch, abstention, tie, or cleanup error fails closed without overwriting user changes.

## Configuration

Resolution order (later wins):
1. compiled-in defaults
2. `EVALWITNESS_PRESET`
3. first existing canonical config: explicit path, `~/.config/evalwitness/config.{toml,json}`, then project file
4. fallback-only legacy config under `~/.config/logprobe` or project `logprobe.{toml,json}`
5. `.env` auto-discovered
6. environment variables
7. CLI flags / MCP tool arguments

Env vars:
| name | description | default |
|---|---|---|
| EVALWITNESS_PRESET | optional preset bundle | compiled compatibility preset |
| EVALWITNESS_PROVIDER | provider identifier | configured route label |
| EVALWITNESS_WIRE_FORMAT | wire format identifier | openai |
| EVALWITNESS_MODEL | model id | configured model |
| EVALWITNESS_BASE_URL | base URL override | configured OpenAI-compatible endpoint |
| EVALWITNESS_CA_FILE | PEM CA bundle override for provider TLS verification | unset |
| EVALWITNESS_API_KEY | universal key fallback | - |
| EVALWITNESS_THINKING_MODE | OpenAI-compatible thinking switch | disabled |
| EVALWITNESS_CACHE_DIR | cache root | ~/.cache/evalwitness |
| EVALWITNESS_LEGACY_CACHE_DIR | optional read-only legacy cache import root | unset |
| EVALWITNESS_MAX_WORKERS | concurrent provider calls | 8 |
| EVALWITNESS_TIMEOUT_SEC | per-request timeout | 120 |
| EVALWITNESS_LOG_LEVEL | stderr log level | info |
| EVALWITNESS_DEFAULT_REPS | default n_reps | 1 |
| EVALWITNESS_BIAS_MITIGATION | adaptive pair order/escalation policy | adaptive |
| EVALWITNESS_INCONSISTENCY_POLICY | fixed-both inconsistency policy | flag-only |
| EVALWITNESS_MAX_PAIR_CALLS | adaptive hard call ceiling per pair | 2 |
| EVALWITNESS_PAIR_CONFIDENCE | win-probability stop threshold | 0.6 |
| EVALWITNESS_PAIR_CALIBRATION_SIGMA | residual score-difference sigma | 0.05 |
| EVALWITNESS_EXPECTED_ESCALATION_RATE | expected-case preflight estimate | 0.25 |
| EVALWITNESS_RUN_BUDGET_STATE | optional restart-safe eval budget JSON; atomic mode-0600 updates before HTTP | unset |
| EVALWITNESS_MIN_DISPATCH_INTERVAL_SEC | locked delay after each successful batch cell before the next dispatch | 0 |
| EVALWITNESS_EVIDENCE_TOKENS | per-trajectory evidence cap | 32000 |
| EVALWITNESS_SINGLE_ELIM | dynamic N-1-match tournament | true |
| EVALWITNESS_SELECTION | pairwise selection override: absolute or joint_absolute | unset |
| EVALWITNESS_SPRT | legacy fixed/multi-rep Wald adaptation | false |
| EVALWITNESS_MAX_TOKENS | explicit output cap; 0 selects the 4096 reference-parity cap, with semantic output frozen after all score tags and a bounded terminal-usage drain | 0 (auto) |
| EVALWITNESS_TEMPERATURE | score-call temperature | 1.0 |
| EVALWITNESS_SEED | provider seed passthrough where supported (OpenAI, Together, Fireworks) | unset |
| EVALWITNESS_REDACT_PATTERNS | JSON file with extra redaction rules `[{pattern, replacement}]` | unset |
| EVALWITNESS_JUDGE_MODE | skip logprob requests, raw-text extraction (LLM-as-a-Judge) | false |
| EVALWITNESS_ALLOW_JUDGE_MODE | doctor treats logprob-less routes as usable judge-mode | false |
| DEEPSEEK_API_KEY | provider-specific key | - |
| GEMINI_API_KEY | provider-specific key | - |
| OPENAI_API_KEY | provider-specific key | - |
| TOGETHER_API_KEY | provider-specific key | - |
| FIREWORKS_API_KEY | provider-specific key | - |
| OPENROUTER_API_KEY | provider-specific key | - |
| GROQ_API_KEY | provider-specific key | - |
| OLLAMA_BASE_URL | local server URL | http://localhost:11434/v1 |

Built-in presets:
| EVALWITNESS_PRESET | provider | wire_format | base_url | model | key env |
|---|---|---|---|---|---|
| bai-deepseek-v4-flash | bai | openai | https://api.b.ai/v1 | deepseek-v4-flash | BAI_API_KEY |
| deepseek-v4-pro | deepseek | openai | https://api.deepseek.com | deepseek-v4-pro | DEEPSEEK_API_KEY |
| deepseek-v4-flash | deepseek | openai | https://api.deepseek.com | deepseek-v4-flash | DEEPSEEK_API_KEY |
| fireworks-deepseek-v4-flash-0731 | fireworks | openai | https://api.fireworks.ai/inference/v1 | deepseek-v4-flash-0731 | FIREWORKS_API_KEY |
| opencode-go-deepseek-v4-flash-0731 | opencode-go-cn | openai | https://opencode.ai/zen/go/v1 | deepseek-v4-flash | OPENCODE_GO_API_KEY |
| openrouter-ambient-deepseek-v4-flash-0731 | openrouter-ambient | openai | https://openrouter.ai/api/v1 | deepseek/deepseek-v4-flash-0731 | OPENROUTER_API_KEY |
| openrouter-morph-deepseek-v4-flash-0731 | openrouter-morph | openai | https://openrouter.ai/api/v1 | deepseek/deepseek-v4-flash-0731 | OPENROUTER_API_KEY |

Provider auto-resolves API key by preferring `<UPPER_NAME>_API_KEY` then `EVALWITNESS_API_KEY`.

## Canonical Request and Response Identity

Canonical request schema 2 bytes use a fixed field order, UTF-8 JSON strings, sorted map keys, explicit nulls, normalized empty slices/maps, and IEEE-754 float64 bit strings after negative-zero normalization. Credentials, local paths, timestamps, retries, callbacks, and lineage are excluded. Ordered evidence bindings are included. `request_fingerprint=sha256(canonical_request_bytes)`; `sampling_slot` remains a separate lookup dimension so independent stochastic draws share semantics without overwriting one another.

`route_id` hashes length-prefixed provider ID, canonical origin, canonical endpoint path, and requested model. Equivalent canonical and deprecated configuration sources produce the same request envelope. Different gateways, base paths, models, thinking controls, messages, numeric controls, logprob shape, stops, score tags, response modes, streaming choices, prompt contracts, or logit bias values produce different fingerprints.

OpenRouter upstream routing is preset-owned. `openrouter-ambient` and `openrouter-morph` must map to their exact named upstreams; construction fails on any mismatch. The provider label binds the routing body covered by source and producer identities, while `provider.only`, required parameter support, and disabled fallback prevent runtime substitution.

Joint-absolute selection: ordered 2-10 candidate trajectories -> one prompt -> one score tag per (criterion,candidate) -> one immutable response per fixed repetition -> per-candidate criterion means -> highest score unless best-versus-runner-up margin is at most epsilon, then tie. Adaptive stopping and SPRT are invalid for this strategy. `worst_logical_calls=fixed_repetitions` per task group.

Batch pacing: `min_dispatch_interval_seconds > 0` requires `max_workers=1`, holds that worker slot after a successful cell, and delays the next cell. The value is part of verification policy, plan identity, live authorization, and the locked provider plan. Cancellation interrupts the wait. Failed cells fail closed immediately; pacing never hides or retries a failure.

The stable vector corpus is `internal/provider/testdata/request-fingerprint-v2.json`. `eval/python-reference/verify_request_fingerprints.py` independently implements the byte contract and must reproduce every canonical byte sequence and SHA-256 digest without calling Go code.

Exact response validation recomputes the request fingerprint, route identity, body digest, parsed-payload digest, and evidence digest. Replay status and reason may change from live to exact without changing evidence identity. All other response metadata, token evidence, parsed values, raw bytes, usage, served identity, attestation reference, and lineage are evidence-digest inputs.

## Cache

Backend: filesystem JSON under `EVALWITNESS_CACHE_DIR/routes/route-<sha256>/responses/<sha256-prefix>/<sha256-full>.json`; route `identity.json` stores exact provider/model metadata and `capabilities.json` stores the probe record. Route IDs are `route-` plus SHA-256 over length-prefixed provider/model bytes. External identifiers never form path components.

Read order: canonical root, then optional `EVALWITNESS_LEGACY_CACHE_DIR`. Legacy `LOGPROBE_CACHE_DIR` config resolves only to the read-only import root. Writes, stats, and clear target only the canonical root; a fallback hit requires the exact canonical request-key hash, increments `usage.legacy_cache_hit_calls`, and emits `cache_namespace` in audit rows.

Legacy census: `CensusLegacyCache(root, publishedProvider)` opens the root through
`os.OpenRoot`, walks without mutation, parses only structurally identified
schema-1 response files, reads no operational content, and emits canonical
`evalwitness.legacy-cache-census.v1`. Classification is exhaustive over response,
capability, and operational files. Response rows commit provider, path, byte
count, and schema through `evalwitness.legacy-cache-inventory.v1`; operational
rows commit path and byte count through a separate digest. Exact admissibility is
zero unless every request, raw response, route attestation, parser/evidence,
binary, source-tree, dataset, analysis, and collection-clock identity is present.
No schema-1 entry can satisfy that requirement.

Root contract: canonical path resolved through existing-parent symlinks; filesystem roots, volume roots, user home, repository root, working directory, and their deletion-containing ancestors rejected; root mode `0700`; `.evalwitness-cache-root.json` mode `0600` with `schema_version=1`, `product=evalwitness`, and random 128-bit `root_id`. Marker presence never overrides protected-path rejection. Unmarked non-empty roots are read-only compatibility inputs and cannot receive writes or destructive operations.

Mutation contract: sensitive descendants mode `0600`, directories mode `0700`; unique same-directory candidate; checked write, file sync, close, atomic rename, directory sync, candidate cleanup. `cache clear` requires scope `responses`, `capabilities`, or `all`; selected descendants are atomically renamed to unique tombstones and removed through a confined filesystem root. Root, marker, route identity, unknown children, and legacy roots remain.

Cache lookup key: `(request_fingerprint, sampling_slot)`. Request semantics and route are owned by the canonical envelope; the sampling slot distinguishes repetitions. Cache storage and released replay fixtures share identity primitives but have separate roots and mutation policies.

Entry:
| field | type | description |
|---|---|---|
| schema_version | int | cache schema 3 |
| request_fingerprint | string | exact semantic request identity |
| sampling_slot | string | independent-draw identity |
| response | ResponseRecord | checksum-bound response evidence |
| score_evidence | map<string,ScoreEvidence> | deterministic derivation bound to the exact request and token stream |
| created_at | int64 | unix seconds |

TTL: none. Manual via explicit `evalwitness cache clear --scope responses|capabilities|all`. Exact cache reads validate request, response, route, checksums, and byte-equivalent score-evidence re-extraction. Judge and verifier requests have distinct fingerprints and never cross-hit. Legacy schema reads are explicit compatibility lookups and never silently become exact evidence.

Hit-rate stats reported in `usage.cached_calls / usage.total_calls` per response.

## Cost Model

Per-call cost computed from configured per-million-token rates; result stored in `usage.est_cost_usd` per response when rates are known.

Formula: `cost = (input_tokens - cached_tokens) * input_price + cached_tokens * cache_price + output_tokens * output_price`. Prices in USD per million tokens.

Rate inputs:
| env | description |
|---|---|
| EVALWITNESS_INPUT_USD_PER_M | input token price per 1M tokens |
| EVALWITNESS_CACHED_USD_PER_M | cached input token price per 1M tokens |
| EVALWITNESS_OUTPUT_USD_PER_M | output token price per 1M tokens |

Unknown rates -> `est_cost_usd` omitted in output.

Subscription-based providers (for example fixed-fee plans): set `EVALWITNESS_BILLING_MODEL=subscription` to force `est_cost_usd: 0.0` regardless of configured token rates.

Per-call cost cap: `EVALWITNESS_MAX_COST_USD_PER_CALL` (default unset). If the next request cannot reserve its estimated input plus full output ceiling under the cap, return error -32008 before dispatch.

Aggregate cost: each mode result includes `usage.est_cost_usd` summed across all underlying provider calls (cache hits contribute 0).

Eval preflight computes exact logical call bounds for the selected tournament or absolute candidate-scoring policy, estimated input with the preprocessing bytes/4 method, conservative output/cost reservation, attempt bounds including retries, concurrency, and operation duration. Pairwise artifacts report `pair_matches`; absolute-selection artifacts report `candidate_scores`. `--max-calls`, `--max-attempts`, `--max-input-tokens`, `--max-output-tokens`, `--max-cost-usd`, `--max-concurrent`, and `--max-duration` default to the conservative worst case derived by the unified aggregate request plan; the descriptive estimator never overrides service limits. A concurrency-safe run budget reserves logical calls separately, then reserves every outbound attempt before HTTP. Terminal observed usage releases unused token/cost reservation; missing usage and failed attempts retain the worst case; actual overruns fail explicitly. Cache/replay hits do not consume network attempts. `EVALWITNESS_RUN_BUDGET_STATE` persists schema, original start time, limits, and aggregate reservations by fsynced mode-0600 candidate plus atomic rename; a resumed process rejects changed limits. New eval JSON declares `evalwitness.evaluation.v2` and aggregates pair calls, escalation count, per-call-count histogram, inconsistency count, mean decision strength, and mean minimum valid-score mass; `--details` retains every `PairDecision` and its score evidence.

Live eval runners require strict Top-20 score evidence for every requested tag. A missing, degenerate, truncated, ambiguous, sub-0.05-mass, or corrupt response remains uncached and receives at most three bounded budgeted extraction retries; the fourth failed attempt aborts. Benchmark evaluation never continues in mixed or implicit judge mode. `--judge-mode` is a distinct text-only request and malformed judge output also fails.

## Optimizations

| name | mechanism | impact | gating |
|---|---|---|---|
| globally bounded fanout | concurrent eval tasks and pair matches share one Runner semaphore | fills small-task workloads without exceeding EVALWITNESS_MAX_WORKERS | always on |
| single-pass preprocessing | preflight retains prepared evidence for selection | removes duplicate parse/redact/slice work | eval-terminal / eval-swebench |
| disk cache | re-runs return instantly | 100% on hit | on by default |
| adaptive K=1 | balanced first order; strict ScoreEvidence and decomposed uncertainty drive escalation | one call on clear pairs, default ceiling two and configurable hard ceiling four | default pairwise policy |
| dynamic single-elim | round-by-round bracket advances the measured winner | exactly N-1 matches | default; EVALWITNESS_SINGLE_ELIM=false for all-pairs |
| evidence slicing | retain changes, tests, failures, exit codes and final state before whole-step ranking | smaller stable prompts | trajectory exceeds EVALWITNESS_EVIDENCE_TOKENS |
| legacy adaptive reps (SPRT) | Wald's Sequential Probability Ratio Test | compatibility for explicit multi-rep runs | EVALWITNESS_SPRT=true |
| critique-then-score | LLM emits brief critique before score tag | potential quality lift with output-token overhead | EVALWITNESS_CRITIQUE_THEN_SCORE=true (default true) |
| logit bias | request plumbing to constrain output tokens to A-T | inactive: Capabilities.LogitBias is never set true because no local tokenizer resolves A-T token ids; wire support exists for future activation | Capabilities.LogitBias=true (currently unreachable) |
| max_tokens budget | 4096 default (reference parity: the prompt analyzes before emitting tags); semantic output freezes after all score tags and transport performs a bounded terminal-usage drain; EVALWITNESS_MAX_TOKENS overrides | correctness first, bounded semantic output and transport drain | always on |
| logprob streaming score freeze | freeze semantic score content after all tags, drain at most the bounded terminal-usage window, then cancel | bounded semantic output with usage reconciliation | provider supports streaming logprobs |
| multi-criterion bundling | one prompt scores all criteria via parallel score tags | fewer calls when multiple criteria are enabled | EVALWITNESS_MULTI_CRITERION_BUNDLE=true (default true) |
| cache-optimal fixed ordering | phase X then phase Y with sorted pairs | input-cost reduction on cache-supporting providers | explicit EVALWITNESS_BIAS_MITIGATION=both |
| pair-skip on identical traces | sha256 match -> skip pair, score 0.5/0.5 | trivial savings on duplicate inputs | always on |
| run budget | shared logical-call, per-attempt input/output/cost, concurrency, and operation-deadline reservation including retries | bounded spend and runtime | every live entrypoint |

Adaptive K=1 distribution decision:

For every criterion and trajectory side, extract strict `ScoreEvidence`. Conditional expectation and variance feed the current decision model; visible, valid, and unobserved mass remain separate evidence and never become invented tail probabilities. Orient observations to the original pair; retain order and repeat identity; average conditional score differences; compute conditional-token, repeated-sample, presentation-order, and configured policy variance separately; sum them for `p_win = NormalCDF(conditional_mean_difference / sqrt(total_variance))`. Stop only when the margin exceeds epsilon and the uncalibrated threshold is crossed. Opposite winner directions across presentation orders force abstention. Reaching the call ceiling without a qualifying decision also forces abstention. `EVALWITNESS_MAX_PAIR_CALLS=2` is the measured default; four is the accepted maximum. `calibrated=false` is mandatory until TASK 048 supplies a locked held-out mapping.

SPRT compatibility mode:

For an explicitly requested fixed multi-rep run, Wald's Sequential Probability Ratio Test can stop after the likelihood ratio crosses the configured threshold.

Parameters:
| name | default | description |
|---|---|---|
| EVALWITNESS_SPRT_ALPHA | 0.05 | type-I error: probability of falsely declaring winner when truly tied |
| EVALWITNESS_SPRT_BETA | 0.05 | type-II error: probability of missing the true winner |
| EVALWITNESS_SPRT_MAX_REPS | 4 | compatibility-mode rep ceiling |
| EVALWITNESS_SPRT_MIN_REPS | 1 | minimum reps before SPRT may terminate |
| EVALWITNESS_SPRT_SIGMA | 0.15 | per-rep score-diff stddev estimate (calibrated per provider, override per probe result) |

Likelihood model: per-rep score difference `d_k = s_i_k - s_j_k` modeled as Normal(mu, sigma). H_i: `mu = +epsilon`; H_j: `mu = -epsilon`. Likelihood ratio after K reps: `LR = exp(2*epsilon*sum(d_k) / sigma^2)`.

Decision thresholds: `A = (1-beta)/alpha = 19`, `B = beta/(1-alpha) ≈ 0.0526`. Continue if `B < LR < A`; declare H_i if `LR >= A`; declare H_j if `LR <= B`.

Default `EVALWITNESS_SPRT=false`; pairwise uses the distribution-backed adaptive K=1 path. Explicit non-adaptive configurations use fixed `EVALWITNESS_DEFAULT_REPS` when SPRT is disabled.

Concrete call-count example: tournament with n=8 trajectories, 28 pairs, 3 criteria, 4 reps each.
- naive: 28 * 3 * 4 = 336 expensive calls

Critique-then-Score:

Modified prompt template requiring brief critique before score tags. Intended to improve hard-case discrimination by forcing the model to state evidence before committing to score tags.

Prompt addition (inserted between focus segment and scale segment):
"Before scoring, write a 1-2 sentence critique inside `<critique_A>...</critique_A>` and `<critique_B>...</critique_B>` tags identifying the strongest evidence for your judgment."

Logprob-position-finding still anchors on `<score_A>` / `<score_B>` tags placed AFTER the critique block; critique tokens consume context but do not affect score extraction. Provider `RawText` retains the response and passes through the normal cache, replay, audit, and redaction contracts.

Cost overhead: additional critique tokens per call within the 4096 output cap; the streaming score freeze and bounded terminal-usage drain limit retained generation. Benchmark marginal cost on the target provider before claiming a fixed percentage.

Toggle: `EVALWITNESS_CRITIQUE_THEN_SCORE=true|false` (default true).

Multi-Criterion Bundling:

Single prompt evaluates all configured criteria for a given (pair, rep) at once instead of N separate calls. Prompt contains all N criterion descriptions concatenated and asks for N score-tag pairs.

Output tag layout follows explicit caller criterion order: `<score_A_<id1>>X</score_A_<id1>>` `<score_B_<id1>>Y</score_B_<id1>>` `<score_A_<id2>>...` for each criterion. Position-finding runs against all 2N tags in one response. Map-derived registries and set projections are sorted, but caller order is semantic and never rewritten.

Cost arithmetic: input tokens grow by each added criterion description while call count drops by Nx. Prompt-cache behavior is provider-specific; measure net savings on the configured provider.

Quality consideration: independent criteria (specification vs output_match vs error_signals) are safer to bundle than highly correlated criteria. Auto-bundle only when calibration evidence supports it; otherwise fall back to separate calls.

Determinism: criteria are emitted in caller order; output position equals caller index; canonical request identity includes the complete ordered criterion contract.

Toggle: `EVALWITNESS_MULTI_CRITERION_BUNDLE=true|false` (default true). Disabled: N separate calls per pair-rep.

Fixed-Both Cache-Optimal Pair Ordering:

Tournament dispatch sequenced to maximize prompt-cache hits on endpoints with automatic prefix caching.

Phase 1 (X-orders): dispatch all `(A=traj_i, B=traj_j)` pairs in lexicographic `(i, j)` with `i < j`. Constant prefix `system + task + traj_i` reused across consecutive pairs sharing same `i`.

Phase 2 (Y-orders for order-bias mitigation): dispatch all `(A=traj_j, B=traj_i)` pairs sorted by `(j, i)`. Constant prefix `system + task + traj_j` reused across consecutive pairs sharing same `j`.

Effective prefix cache hit ratio depends on provider cache semantics and trace shape; total input cost should trend toward the per-call variable suffix when cache hits are reported. Verify reduction through `usage.cached_tokens` in the audit log.

This ordering applies to explicit fixed `both` runs. Adaptive K=1 instead balances the initial order by task hash and avoids most reverse-order requests.

## Error Handling and Retry

HTTP-level retry policy:

Current retry policy: `evalwitness.provider-retry.v3`; v1 and v2 remain valid for historical replay but cannot satisfy a v3 live execution binding.

| condition | action |
|---|---|
| HTTP 200 | success, parse |
| HTTP 408 request timeout | retry up to 3, exp backoff base 1s |
| HTTP 429 too many requests | parse integer-seconds or HTTP-date `Retry-After`; wait then retry up to 5, cap 60s |
| HTTP 402 payment/credit required | non-retryable; surface the provider credit or account limit |
| Upstream HTTP 400 capability error with literal `` `logprobs` is not supported `` | close idle pooled connections, then retry within the configured attempt ceiling; all other provider capability failures remain terminal |
| HTTP 500 / 502 / 503 / 504 | retry up to 3, exp backoff base 1s |
| HTTP 4xx other | non-retryable, surface error -32001 |
| network timeout | retry up to 3, exp backoff base 2s |
| TLS handshake failure | retry up to 2, backoff 1s, 4s |
| context cancelled | caller cancel/deadline: abort immediately, no retry; per-request http.Client timeout with a live caller context: retryable (server-side queuing) |
| EOF on body read mid-stream | retry up to 2 with full re-request |

Backoff formula: `delay = min(base * 2^attempt + jitter[0,1)s, ceiling)`. Default ceiling 60s.

Provider-error classification (body keyword scan, lowercase):
| signal | classification | action |
|---|---|---|
| HTTP 402 regardless of body | payment-required | no retry, surface the provider credit or account limit |
| HTTP 429 regardless of body, or "rate" / "limit" / "quota" / "tpm" / "rpm" | rate-limited | back off, retry, escalate to -32002 if exhausted |
| "context" / "prompt tokens limit" / "length" / "too long" | context-overflow | no retry, return -32005 |
| "model" / "not found" / "does not exist" | bad-config | no retry, return -32001 with provider/status data |
| "auth" / "unauthorized" / "invalid api key" / "forbidden" | auth-failed | no retry, return -32007 |
| "logprobs" / "top_logprobs" | capability-missing | mark caps Logprobs=false and fail the strict verifier request; judge mode requires an explicit separate run |
| HTTP 5xx without classifying body | transient | retry per HTTP 5xx rule |

Retry and timeout budget: `EVALWITNESS_MAX_RETRIES` (default 5) bounds additional attempts. `EVALWITNESS_TIMEOUT_SEC` (default 120) bounds each HTTP attempt; the authorization budget owns the total operation deadline across initial attempts, retries, and Retry-After waits.

Tournament-level fault tolerance: a provider error aborts the active run instead of fabricating a 0.5 score. Completed extractable calls stay cached; the bounded supervisor can retry with lower concurrency or another qualified provider without losing provider-scoped progress.

Cache write policy: verifier responses require valid strict score evidence for every requested tag; judge responses require exact explicit judge evidence. Judge and verifier identities never cross-hit. Legacy raw-text-only entries are misses in verifier mode. Truncated, malformed, ambiguous, or failed responses are never cached and retry on the next invocation.

Benchmark supervisor policy: atomically persist the active provider route and capability-gate timestamp; prefer the provider that owns the warm cache; open a route circuit after repeated failures; honor integer or HTTP-date Retry-After; adapt worker concurrency independently per provider so one route cannot throttle another; use finite attempts and a hard global duration. Provider-specific cache and result paths never merge scores across routes. Default TASK 031 workload is bounded Terminal-Bench plus SWE-bench; paper parity and judge ablation are explicit opt-ins.

## Default Setup

| field | value |
|---|---|
| Provider | `${EVALWITNESS_PROVIDER}` |
| BaseURL | `${EVALWITNESS_BASE_URL}` |
| Model | `${EVALWITNESS_MODEL}` |
| Auth | Authorization: Bearer `${EVALWITNESS_API_KEY}` or the provider-label key |
| Thinking | disabled |
| Endpoint | POST /chat/completions |
| MaxConcurrent | 8 |
| nReps default | 1 |
| Criteria default | ["generic"] |

The custom OpenAI-compatible route is the portable setup path. The compiled
compatibility preset is retained for offline gates and backwards compatibility;
presets are configuration bundles, never permanent capability badges. A current
exact attestation is required before a live verifier call.

Coding Plan rate-limit awareness: respect `429` responses with exponential backoff base 2s, max 60s, 5 retries. If sustained 429s: surface `provider rate-limited` error with retry-after.

## Build and Distribute

| target | command |
|---|---|
| dev run | go run ./cmd/evalwitness |
| local install | go install ./cmd/evalwitness |
| static binary | CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w -buildid= -funcalign=8" -o evalwitness ./cmd/evalwitness |
| cross-platform | scripts/build/build.sh -> dist/evalwitness-{darwin,linux,windows}-{amd64,arm64}; release-only project no-inlining profile keeps every stripped artifact within the 20 MiB gate |
| run tests | `scripts/tests/list-project-go-packages.sh | xargs go test` |
| run all checks | scripts/tests/run-tests.sh |
| rebuild and verify explorer | `cd web/explorer && bun install --frozen-lockfile`, then `scripts/tests/run-evidence-explorer.sh` with Playwright Chromium installed |
| capture explorer presentation assets | `cd web/explorer && bun run capture:assets -- --html PATH --destination NEW_DIRECTORY`; destination must not exist |
| record explorer terminal proof | `go run ./scripts/demos/record-terminal-demo.go --destination NEW_CAST -- scripts/demos/run-evidence-explorer-demo.sh --binary PATH --capsule PATH --ledger PATH --repository-root PATH --destination NEW_HTML` |

Binary size gate: at most 20 MiB per stripped cross-platform artifact, measured
from the complete current release build.

Canonical product version: `internal/product/version.go`; valid SemVer without a
`v` prefix. Release tags are exactly `v<product-version>`. Build scripts cannot
inject or derive a different version from an environment variable or Git tag.

## Test Strategy

| level | scope | runner | location |
|---|---|---|---|
| unit | pure functions: extractScore, position-finding, prompt assembly, redaction, criterion validation, canonical identity, cache key derivation, cost computation | go test | `*_test.go` co-located |
| provider integration | OpenAI-compatible wire behavior against mock HTTP servers and replay fixtures | go test | internal/provider, internal/replay |
| cross-language contract | independently reproduce canonical bytes and fingerprints from stable vectors | python3 | eval/python-reference/verify_request_fingerprints.py |
| live conformance | bounded real score task -> exact expiring attestation | manual two-step `evalwitness attest` | requires provider API key and explicit authorization digest |
| mode end-to-end | each mode against scripted provider outputs | go test | internal/mode/mode_test.go |
| MCP protocol conformance | stateless `2026-07-28` discovery/list/call/error behavior plus exact legacy initialize/list/call compatibility | go test and `scripts/tests/run-product-conformance.sh` | internal/mcp, cmd/evalwitness |
| concurrency stress | high-fanout pairwise with simulated provider, race detector | `go test -race` | internal/mode |
| capability probe | probe cache persistence and response-shape detection | go test | internal/provider |
| conformance | route states, strict qualification, expiry/drift, public derivative signatures and freshness, deterministic failure scenarios | go test | internal/conformance |
| live boundary | every live CLI/MCP/Best-of-N path previews authorization and performs no network or agent execution by default | go test | cmd/evalwitness, internal/mcp, internal/mode |
| bounded evaluation | preflight arithmetic, atomic run budgets, adaptive pair ceiling, dynamic tournament, evidence slicing | go test | cmd/evalwitness, internal/mode, internal/preprocess |
| transcript fidelity | format conservation, golden digests, malformed and bounded inputs, hard budgets, UTF-8, derivation lineage, fuzz smoke | go test and `scripts/audits/run-transcript-fidelity.sh --fuzz-smoke` | internal/preprocess |
| audit protocol conformance | strict schemas, canonical JSON, 188 positive/negative cases, 111 required-field removals, frozen request digests, direct/subprocess parity, claim-level matrix | `scripts/audits/run-protocol-conformance.sh` | protocol, cmd/evalwitness |
| trace interoperability | pinned OTLP/Agent Trace versions, hostile inputs, metadata privacy, causal hierarchy, attribution boundary, licensed fixture provenance, semantic round trip, provider-free CLI | go test, fuzz smoke, `scripts/audits/run-trace-interoperability.sh` | internal/preprocess, cmd/evalwitness |
| verification lineage | sealed plan identity, closed schema, strict codec, exclusive terminal precedence, role and cluster isolation, holdouts, claim ceilings, no-acquisition boundary, deterministic artifact regeneration, earliest-loss certificate, canonical JSON-to-SVG projection, corruption rejection, and public scan | go test, `scripts/tests/run-claimcheck.sh` | internal/lineage, cmd/evalwitness |
| experiment capsule and claims | closed registries and extensions, canonical identity, role/visibility/edge graph, public/private family binding, build/source dirty-state match, deterministic archive, in-toto/DSSE/explicit-key Sigstore, no-overwrite publication, 34-claim lifecycle, exact expressions, 189 challenge receipts, Claim Autopsy, six generated surfaces, corruption/schema/missing-file rejection, clean-directory hard-network-denied verification | go test, race, `scripts/tests/run-claimcheck.sh` | internal/capsule, internal/claim, cmd/evalwitness |
| release supply chain | dirty-tree refusal, exact five-platform reproducibility, seven-role inventory, strict USTAR source-archive and portable-source binding, manifest-bound six-module offline Go proxy with escaped-path and version-list validation, empty-cache archive-only byte reproduction, embedded-build-info SPDX, release in-toto, strict private-key custody, DSSE substitution rejection, no-overwrite signature directory, unsigned-development opt-in, tag-bound publication authorization, full asset/SBOM/Statement/signature recomputation, capsule/claim/challenge/surface/Autopsy/explorer round trip, public scan, checksum verification, clean CI candidate | go test, `scripts/tests/run-release-build.sh`, `scripts/tests/run-release-roundtrip.sh`, `scripts/build/build-release-candidate.sh` | internal/release, internal/capsule, cmd/evalwitness, scripts/build |
| offline evidence explorer | atomic claim-side evidence re-verification, verified report projection, release byte binding, public owner-inspection boundary, deterministic canonical JSON/HTML, embedded-asset identity, no-overwrite publication, CSP/injection rejection, public artifact safety, challenge replay surface, registry-only deep-link restoration, typed unavailable extensions, file-protocol operation, responsive layout, keyboard/dialog behavior, reduced motion, WCAG 2.2 AA, deterministic public presentation assets, bounded real-output asciicast proof, and zero external requests or runtime errors | go test, Vitest, Playwright Chromium, `scripts/tests/run-evidence-explorer.sh`, `scripts/demos/run-evidence-explorer-demo.sh` | internal/explorer, web/explorer, cmd/evalwitness, scripts/demos |
| exact-response supply chain | policy sealing, full producer provenance, embedded redistribution evidence, deterministic request-corpus/index identity, byte-identical double build, public scan, exact replay, empty state, hard network denial, schema-1 rejection, evidence drift, payload corruption, and declared TASK 070 clean-clone graph reproduction | go test, `scripts/tests/run-response-bundle.sh`, `scripts/evals/reproduce-public-evidence.sh --profile full`, `scripts/evals/reproduce-identical-response-v5.sh` | internal/replay, internal/cache, cmd/evalwitness, scripts/build, scripts/evals |
| controlled corruption | 13 deterministic mutators, relation preservation, semantic-gold rejection, formal and trusted-executable validation, minimal reduction, strict codecs/schemas, malicious input, resource bounds, split lineage, frozen design, 320-case regeneration, typed v3 natural audit, exact result-provenance/failability/evidence-loss qualification, positive controls, controls, balance, and blind sampling | go test, fuzz seeds, `scripts/audits/run-controlled-corruption.sh`, `scripts/audits/run-controlled-corruption-v3.sh` | internal/mutation, internal/preprocess, internal/stats, cmd/evalwitness |
| metamorphic stress lab | 35 closed schemas, exact v3 relation catalog and replay commitment, 10-arm/5,630-cell support accounting, admission-filtered execution bindings, private preflight capsule-family verification, authority-bound time-limited permits, atomic at-most-once reservation receipts, permit-bound live response evidence, independently executed exact-replay verification, denominator-conserving execution ledger, readiness refusal, complete-support mechanism seal, owner-only atomic admission-filtered execution seal with exact next-catalog discovery routing, adversarial service-boundary controls, deterministic provider-independent development case study, exact JSON/Markdown/SVG regeneration, self-contained licensed-fixture challenge, seven-guard deterministic receipt, empty-workspace reconstruction, inert paper-ready vector projection, strict repository validation, and 32-line-to-two-line one-minimal reduction | go test, race, `scripts/audits/run-stress-lab.sh` | internal/stress, internal/verification, internal/mode, cmd/evalwitness, eval/results |
| evidence reliance | ten-factor/four-interaction ontology and preregistration, exhaustive Walsh alias audit, planted-effect estimator, 24-task/1,536-cell registered denominator, full score-evidence outcomes, selector/render visibility, five arm-contrast families, relation and owner-custody admission boundaries, shared one-minimal reducer, frozen TASK 050 base, public child-capsule family, 11 claims, 98-dimension profile, 98-row paper view, optional Explorer projection, deterministic two-process artifact identity, corruption rejection, public scan, and zero-provider/network boundary | go test, Vitest, Playwright Chromium, `scripts/tests/run-claimcheck.sh`, `scripts/tests/run-evidence-explorer.sh` | internal/reliance, internal/explorer, web/explorer, cmd/evalwitness, eval/results |
| outcome validity | six outcome states, evidence separation, strict evidence/record drafts, revision coherence, 40 strict schemas, natural shortfalls, deterministic non-launchable mixed-v1 diagnostic, six-case objective-typed outcome pilot v2, exact raw-run/reward resolution, anchored 16k redacted evidence slicing, restricted packets, sealed private custody and owner-only source bindings, sealed reviewability inspection with semantic-pending status, no task reuse, inspection-bound authorization-explicit readiness v3, bounded rerun failure classes, HMAC blinding, frozen handbook policy, handbook-digest bundle binding, self-contained ordered reviewer kits, bundle-required injection-safe Markdown rendering, kit-to-bundle verification, typed rubric/reasons, qualification timing/binding, consent, conflict rejection, reviewer-slot independence, reproducible random assignment, exact label commitments, committed-label reproduction, prereveal rubric ambiguity, post-label semantic-blinding probes, Wilson intervals, chance correction, confidence/Brier and recognition metrics, post-commit reveal, tie-break restriction, dual-analysis-bound terminal ledger, performance-blind post-ledger source-pair agreement and benchmark transitions, and preservation | go test, real subprocess fixtures, `scripts/audits/run-outcome-validity.sh` | internal/outcome, internal/mutation, internal/safety, cmd/evalwitness |
| relation construct validity | objective isolation, 92 closed version-aware protocol schemas plus one independent public-evidence schema and three private journal schemas, immutable v1 reproduction, v2 release/spec/program/construct-audit bindings, balanced 32-case historical primary, typed v3 283-case release, fresh 28-case/32-source/28-task-group/28-lineage core primary with four cases per seven families and 14 calibration plus 14 test, exhaustive 2-development/1-calibration/0-test omitted-evidence sentinel excluded from inference, closed JSON negative-evidence contract plus byte-derived Markdown, jointly feasible seven-family development pilot with zero primary/sentinel source/task/lineage overlap, full construct-firewall commitments, exact v3 56-primary/28-maximum-tie/56-probe primary workload and 14-primary/7-maximum-tie/14-probe/59-total pilot workload, fixed stopping, unresolved retention, bounded task-group inference, `not_run` and `not_authorized` gates, deterministic double reproduction and tamper rejection, complete version-closed v3 replay/material/packet/mapping/qualification/handbook/bundle/readiness/receipt/inspection/reviewer/reveal/comparison/terminal identities rooted in the corpus-plan and typed-firewall chain, mixed v1/v2/v3 rejection, exact all-eight-firewall historical readiness commitment, immutable independently reproduced package-format-v5 owner custody with distinct keys, seven real pilot pairs, three separately replayed sentinel materials, deterministic owner-only negative-evidence inspection, package/inventory-bound append-only guided owner journal with explicit corrections and two-step completion, separate sentinel/challenge/repair bindings, 53-file content-addressed path/size/mode/SHA-256 inventory, mode-0700/0600 custody and non-overwrite publication, content-addressed not-launchable dossier, owner governance decisions and unauthorized external actions, deterministic public-safe launch brief, raw-content-free change receipt, receipt-bound owner-only atlas, package-format-v1/v2/v3/v4/v5 verification across six historical/current packages, owner-only hidden/visible inspection workbook, exact source resolution and pair/pair-of-pairs replay, license-aware aligned restricted excerpts, seven-domain HMAC packet blinding, complete v3 reviewer workflow, conservative construct-caveat/information veto resolution, exact denominators, unresolved sensitivity, mutual outcome rejection, and three-layer terminal custody with verifier evidence unconsulted | go test, `scripts/audits/run-relation-construct.sh`, `scripts/audits/run-relation-governance-v2.sh`, `scripts/audits/run-relation-governance-v3.sh`, `scripts/audits/verify-relation-pilot-package.sh` | internal/relation, internal/mutation, internal/safety, cmd/evalwitness, scripts/build |
| coding-agent-only formal study | exact 20/20 calibration/test selection from the frozen v3 release, source/task/repository/format/split/lineage/digest audit, two independent machine validators, disagreement-only automated tie-break with unresolved retention, strict parent rebinding, byte-identical double build, tamper rejection, and zero provider/human action boundary | go test, `scripts/audits/run-agent-only-study.sh` | internal/agentstudy, internal/mutation, cmd/evalwitness, eval/governance |

Coverage targets (line):
| package | min |
|---|---|
| internal/verifier | 90% |
| internal/provider/openai | 80% |
| internal/mode | 85% |
| internal/cache | 90% |
| internal/mcp | 80% |
| cmd/evalwitness | smoke only |

Fixtures: deterministic provider behavior is covered through exact schema-3 request/response/score-evidence JSONL fixtures, mechanically preserved schema-2 and schema-1 sources, candidate-only golden regeneration through the production Runner, a synthetic capsule-native exact-response conformance bundle with embedded MIT redistribution evidence, corruption tests, and mock HTTP servers.

CI pipeline: GitHub Actions on ubuntu + macos with checkout, setup-go, artifact upload, and release attestation pinned to exact release commits; the Go version is read from go.mod so it cannot drift. Default gates include tracked/unignored Go-source `gofmt`, project-package inventory, `go vet`, Staticcheck 2026.2.1, non-stress `go test`, govulncheck, product-level modern and legacy MCP conformance, exact fuzz-target inventory and bounded non-stress fuzz smoke, `go build`, artifact safety, and `scripts/evals/reproduce-public-evidence.sh --profile full`. Stress and race gates are retained but disabled by default; repository variables `EVALWITNESS_ENABLE_STRESS_TESTS=true` and `EVALWITNESS_ENABLE_RACE_TESTS=true` opt into them explicitly. The full profile builds its producer and executes exact-response conformance plus claimcheck with isolated empty EvalWitness home/configuration/response cache under operating-system network denial, reuses the provisioned runner Go module/build caches, reports zero provider calls, and explicitly does not claim clean-clone proof. The inventory rejects packages outside the module, removes duplicates, and excludes dependency trees such as frontend `node_modules`. Cross-platform build matrix {darwin, linux, windows} x {amd64, arm64}. No live-API tests in CI.

## Determinism and Replay

Exact deterministic mode for tests and reproducible runs.

| env var | effect |
|---|---|
| EVALWITNESS_REPLAY_FROM=path | all `Provider.Score` calls require exact schema-3 request fingerprint, sampling slot, route, response, checksums, and re-derived score evidence; miss/rejection never falls back live |
| EVALWITNESS_REPLAY_TO=path | live calls stage complete request/response records and publish the fixture atomically only on checked close |
| EVALWITNESS_TEMPERATURE=0 | force temperature override per call |
| EVALWITNESS_SEED=int | passed to provider as seed where supported (OpenAI, Together, Fireworks); ignored elsewhere |

Tournament determinism: with the same fixture, task, epsilon, and input order, output is identical. All-pairs iteration is lexicographic `(i,j)`; adaptive base orientation is fixed by `sha256(task,i,j)`; dynamic tournament rounds retain input order and advance the measured winner; criterion order is caller-provided; rep index is stable.

Replay record key: `sha256(request_fingerprint || 0x00 || sampling_slot)`. Record integrity binds canonical request bytes, complete typed lineage, response evidence digest, and serialized score evidence. Loading re-derives score evidence from ordered tokens and rejects any mismatch before checking the record digest. Duplicate keys, cross-mode identity, route mismatch, legacy schema, corruption, incomplete response, missing score tags, and incompatible evidence fail closed with typed `miss` or `rejected` status and reason.

Capture stages a sibling candidate, optionally copies and validates the existing exact fixture, encodes complete records, flushes, syncs, closes, reloads and verifies every checksum, rejects concurrent target mutation, atomically renames, and syncs the parent directory. Finalization failure propagates to the CLI exit status; the failed candidate remains available for inspection.

Legacy migration: `evalwitness replay migrate --source old.jsonl [--candidate old.inspection.jsonl]` never modifies the source and never upgrades schema-1 prompt-hash or schema-2 request/response records to exact evidence. The inspection candidate records source lineage, preserved request/response data where available, `legacy` status, and every missing field including score evidence; the JSON report binds source and candidate digests.

Exact-response bundle objects:

| object | schema | invariant |
|---|---|---|
| policy | `evalwitness.response-bundle-policy.v1` | study/cell, lineage policy `exact_fixture` or `complete_research`, exact producer, sealed dataset/request-corpus digest, declared dataset/response license expressions, redistribution authorization/evidence digest, sorted capture allowlist, fixed omission classes, exact-only/public-scan flags, zero provider calls, zero network, self-digest |
| redistribution evidence | `evalwitness.redistribution-evidence.v1` | exact non-empty public-scanned bytes whose payload digest equals the policy commitment |
| capture | `evalwitness.response-capture.v1` | canonical newline-terminated schema-3 JSONL bytes; complete request/response/score-evidence/record identities; one route/model per capture; bundle-wide unique request-fingerprint plus sampling-slot keys; complete-research count and sorted study-cell census |
| index | `evalwitness.response-bundle-index.v1` | sorted capture census with exact request, lineage, body, evidence, record, payload, byte, route, model, and mode multiset commitments; total entries/bytes; policy digest; self-digest |
| verification | `evalwitness.response-bundle-verification.v1` | capsule/study/cell/policy/index/capture-set/dataset/request-corpus/source/binary/redistribution identities, explicit lineage policy and evidence ceiling, complete verified capture inspections, relative exact payload paths, public-scan result, `provider_calls=0`, `network_required=false`, `exact_replay=true`, `valid=true`; `mechanism_conformance` is synthetic and `record_complete_external_parents_unresolved` still requires an outer study capsule |

Dataset identity is the canonical digest over sorted capture name, entry count,
and request-set digest partitions. Policy sealing is single-use: producer and
dataset identity fields plus policy digest must be empty in the draft. Build
re-inspects all captures and redistribution evidence, requires equality with the
sealed request corpus and producer provenance, embeds exact bytes, verifies the
closed capsule, and publishes only to absent targets. Verification rejects extra
files, non-canonical JSON, unsafe public content, missing or changed evidence,
schema-1/2 input, duplicate exact keys, mixed routes, source/binary drift, and
payload corruption.

Capture-run and 070 bind objects:

| object | schema | invariant |
|---|---|---|
| capture-run attestation | `evalwitness.capture-run-attestation.v1` | re-inspects schema-3 JSONL; authorized_calls equals observed_calls; attempt ledger reconciled; status=complete only when every record has complete research lineage and capability attestation |
| research admission | `evalwitness.capture-research-admission.v1` | binds capture payload SHA-256; admitted only if capture-run status is complete and research lineage is complete; otherwise rejected with required_action=recapture_with_preplanned_research_lineage; stamp writes a new dest and refuses in-place rewrite |
| 070 bind certificate | `evalwitness.identical-response-050-bind.v1` | legacy identity-bound sidecar; incomplete for the v5 response and leaves evidence_ceiling at mechanism_conformance |
| 070 outer bind certificate | `evalwitness.identical-response-050-bind.v2` | public outer-capsule root digest-binds attestation, admission, analysis, authorization, study parents, and response-bundle index; complete v5 parent graph raises the scoped ceiling to record_complete_external_parents_resolved |
| 070 outer capsule | `evalwitness.capsule-manifest.v1` | eight public components and one parent capsule are content-addressed by manifest/registry digests; valid=true and scientific root is the v2 outer bind |
| 070 claim ledger | `evalwitness.claim-ledger.v1` | sealed to the outer capsule and manifest; CLM-070..CLM-074 retain explicit scope, caveats, status, and exact expressions; valid=true |
| 070 challenge pack | `evalwitness.claim-challenge-pack.v1` | sealed to the outer capsule and claim-ledger digest; 34 executable challenge receipts all pass |
| 070 clean-clone report | `evalwitness.identical-response-reproduction-report.v1` | fresh clone with empty home/Go caches, declared file proxy, hard network denial, 14 registered artifact hashes identical, provider_calls=0, status=passed |
| 070 release inventory | `evalwitness.identical-response-release-inventory.v1` | exact source path, release path, role, bytes, SHA-256, and omission state for every registered v5 asset plus explicit direct-asset omissions; raw capture ships only inside the reviewed response bundle |
| intake validation | `evalwitness.registry-intake-validation.v1` | Valid=false on schema failure, markup/path injection, status other than format_verified, community_validated=true, or catalog duplicate entry_id / capsule+profile / challenge_nonce |
| seed catalog | `eval/governance/registry-seed-catalog-v1.json` | two contrasting format_verified development entries with no community validation; gate scripts/audits/run-registry-seed-catalog.sh |
| reliance index | `evalwitness.registry-reliance-index.v1` | optional TASK 065 six-digest compatibility key; omitted entries listed; no ranking or pooling |
| scarcity index | `evalwitness.registry-scarcity-index.v1` | committed 198/3/195 and 2/1/0 availability record; rankable=false; not a verifier score |
| owner-inspection index | `evalwitness.registry-owner-inspection-index.v1` | public aggregates only; 66 assessments; independently_reproduced/community_validated/human_supported stay false |
| method lineage | `evalwitness.registry-method-lineage.v1` | v1-v2, v2-v3, v3-frozen; rankable=false; pooled=false |
| calibration deployment | `evalwitness.calibration-deployment-evaluation.v1` | EvaluateDeployment + IsDeployable on a test-split Observation array; optional artifact/route/domain forces unsupported on mismatch; finite_sample is unsupported under clustered TaskID repeats; not a held-out TASK 049 study |
| calibration model | `evalwitness.calibration.v1` | SealModelArtifact checksums calibrator bytes; leakage feature keys and route/domain mismatch fail closed |

`exact_fixture` admits incomplete research lineage only for
explicit synthetic conformance. `complete_research` rejects any capture unless
every record passes `RequestLineage.ValidateResearch()` and the capture's sole
study-cell identity equals the sealed policy cell; it additionally requires a
clean committed source tree and matching unmodified embedded VCS binary identity.
Each complete-research record also requires SHA-256 source-trace, trace-map, and
policy identities, at least one validated evidence binding, an observed served
model, and an `att-<sha256>` capability-attestation identity.

Snapshot tests: the golden delta fixture is generated through the production Runner into a non-overwriting `.candidate` and byte-compared. The canonical fixture is schema 3, with `golden-delta-replay.schema2.jsonl` and `golden-delta-replay.legacy.jsonl` retained for inspection. Cross-entrypoint tests prove CLI, MCP, eval, and Best-of-N lineage variants retain entrypoint provenance while producing the same semantic request fingerprint.

CLI replay smoke: `scripts/tests/run-replay-smoke.sh` pins empty environment and config inputs, explicitly pins route semantics, sets `EVALWITNESS_REPLAY_FROM=scripts/tests/golden-delta-replay.jsonl`, uses the shipped sample task/trajectories, disables order-bias to require one fixture entry, and asserts exact replay plus winner A without provider calls or machine-local configuration.

No-key claim gate: `scripts/tests/run-claimcheck.sh` builds a fresh temp binary,
bypasses `.env`, verifies the public capsule and all deterministic sidecars,
executes every registered claim challenge, byte-verifies the six generated claim
surfaces, rejects stale identity, scope widening, missing caveats, and unsupported
novelty language across the public narrative, and then regenerates and validates
the verification-lineage, route-configuration, preset, dataset, replay, design,
paired-analysis, calibration, and legacy-artifact claims. The gate makes zero
provider calls. Current live capability requires an exact current
`evalwitness attest` result.

## Verifier Audit Protocol

Protocol identity: `EvalWitness Verifier Audit Protocol`; current version
`1.2.0`; supported previous minor `1.1.0`; canonical policy
`evalwitness.protocol-canonical-json.v1`; schema dialect JSON Schema 2020-12.
Compatibility accepts only the declared versions. Incompatible majors fail;
additive minor behavior requires a declared supported version and normative
compatibility vector.

| conformance level | permitted claim | non-implication |
|---|---|---|
| syntax | implementation exchanges and validates the named version and normative syntax cases | score correctness, route capability, reliability |
| deterministic_replay | implementation reproduces named sealed observations and evidence invariants offline | live compatibility |
| live_score_evidence | named route returned conformant evidence in a separately authorized bounded observation | accuracy, calibration, persistence |
| empirical_reliability | named locked study measured declared failure modes on its population | transfer beyond study scope |
| independent_reproduction | named independent actor reproduced named frozen artifacts or study | universal correctness, endorsement |

| object | required invariant |
|---|---|
| EvaluatorDescriptor | exact protocol identity, supported versions, implementation identity digest, execution modes, claimed levels, evidence versions, bounded limits, live-capable flag, extensions |
| AuditCase | schema/version/ID, level/kind, description, required capabilities, exactly bound invocation, expected outcome, extensions |
| TrajectoryRef | canonical schema, source format, source/trajectory/accounting/trace-envelope/mapping-report digests, mapping-policy version; bounded inline content optional |
| AuditInvocation | offline=true for reference conformance; timeout and input ceiling within descriptor; exactly one typed payload for evidence operations |
| ScoreEvidence | strict alignment; requested/returned Top-k; ordered visible alternatives; canonical support; visible+unobserved=1; support mass=valid mass; exact conditional expectation/variance; request/response provenance; route limitations and degradations |
| DecisionEvidence | typed terminal state; selected winner bounds; abstention reason iff abstained; score-evidence, budget, and provenance digests |
| AuditFinding | stable code, severity, JSON path, message, violated invariant |
| InvocationResult | status, optional preserved evidence, findings, optional observed-artifact digest, extensions, content-sealing evidence digest |
| CapabilityMatrix | exactly five ordered levels with passed, failed, skipped, unsupported, and not-run counts plus reasons |
| AuditRun | evaluator, offline flag, case results, findings, corpus digest, unchanged TASK 043 request-corpus digest, schema-artifact digest, capability matrix, content-sealing run digest |

Canonical JSON: UTF-8; depth <=64; duplicate object keys forbidden; keys sorted
lexicographically; no insignificant whitespace; JSON numeric values restricted to
canonical integers in `[-9007199254740991,9007199254740991]`; no float,
exponent, leading plus, unsafe integer, or negative zero. Non-integer numeric
evidence uses canonical decimal strings without exponent, plus sign, leading
zero, trailing fractional zero, or negative zero. Object-field absence and
explicit null are distinct. SHA-256 is lowercase hexadecimal over canonical
bytes. The frozen TASK 043 request corpus keeps its own exact canonical byte
contract and is embedded without protocol re-encoding.

Adapter transport: one canonical JSON `AdapterMessage` per newline on stdin and
stdout; stdout contains no diagnostics. Maximum message 8388608 bytes, case
4194304 bytes, run 10000 cases, case duration 60000 ms, extensions 32. Sequence:
`hello/hello_ack -> describe/descriptor -> begin_run/run_started ->
evaluate/evaluation_result* -> end_run/run_result`. Every response binds
`reply_to`, negotiated version, and run ID. `cancel/cancelled` is valid only in
an active run between sequential invocations; host context cancellation owns an
in-flight kill. Invalid messages emit a typed non-retryable error and terminate.

Adapter launch authority: CLI/operator supplies literal executable, repeated
literal arguments, optional working directory, and host deadline. The child
receives only `EVALWITNESS_PROTOCOL_OFFLINE=1`. No case, extension, trajectory,
or adapter response can select commands, arguments, environment, cwd, output
path, provider configuration, network destination, or live mode.

Extension contract: reverse-domain namespace, schema identifier, required flag,
canonical payload. Unknown required namespace rejects; unknown optional
namespace round-trips unchanged; extensions cannot redefine core fields.
`org.evalwitness.reliability.v1` / `evalwitness.protocol.reliability-extension.v1`
binds evidence factors, exact parent-child interventions, changed event/field
sets, validators, baseline/intervention result digests, effect, uncertainty, and
the fixed interpretation `verifier_output_reliance`.

`org.evalwitness.application` carries required
`evalwitness.protocol.application-invocation.v1` input and optional
`evalwitness.protocol.application-result.v1` output. Invocation fields are mode,
task, ordered criteria, complete decision policy, and validated run lineage;
non-integer policy values are canonical non-negative decimal strings. The
adapter accepts only offline extension cases with byte-canonical inline JSON
sources whose declared canonical target schema is `evalwitness.trajectory.v1`,
checks their protocol references against application preprocessing evidence,
disables cache use, and returns run/request fingerprints, byte-exact decision
hex, deterministic budget-usage digest with elapsed wall time excluded, and
trajectory digests. The closed-form reference adapter remains the normative
independent oracle; the application adapter proves the protocol boundary reaches
the same production service without introducing another scoring implementation.

Normative execution: embedded direct evaluator and external adapter run the same
sorted corpus. Each negative case mutates one invariant; required-field corpus
removes every required AuditCase input field represented by its positive base.
Capability levels absent from the executed corpus remain `not_run`, never
inferred. `scripts/audits/run-protocol-conformance.sh` uses a fresh temporary
binary, empty environment, direct and subprocess paths, exact corpus parity,
schema checks, and the independent Python TASK 043 fingerprint verifier.

## Tool Integration Recipes

Claude Code:
- command: `claude mcp add evalwitness -- /absolute/path/to/evalwitness mcp-serve`
- result: tools `evalwitness_pairwise`, `evalwitness_absolute`, `evalwitness_delta` available in agent
- repo asset: `config/mcp/claude-code.sh`

Codex CLI: append to `~/.codex/config.toml`:
| TOML | value |
|---|---|
| [mcp_servers.evalwitness] | (table header) |
| command | "/absolute/path/to/evalwitness" |
| args | ["mcp-serve"] |
| env | (optional table; vars to set) |
| env_vars | (optional list of host env vars to forward) |

Equivalent CLI: `codex mcp add evalwitness -- /absolute/path/to/evalwitness mcp-serve`

Repo asset: `config/mcp/codex.toml`

OpenCode: append to `opencode.json` (project root) or `~/.config/mcp/opencode.json` under root key `mcp` (not `mcpServers`):
| JSON path | value |
|---|---|
| $.mcp.evalwitness.type | "local" |
| $.mcp.evalwitness.command | ["/absolute/path/to/evalwitness", "mcp-serve"] |
| $.mcp.evalwitness.enabled | true |
| $.mcp.evalwitness.environment | (optional object of env vars) |

Repo assets: `config/mcp/opencode.json`, `config/mcp/generic-mcp.json`, `config/mcp/kilocode.jsonc`.
| $.mcp.evalwitness.timeout | (optional number, ms; default 5000) |

OpenCode quirks: root key is `mcp`, command is a single array (binary + args concatenated, not split into command/args), every server requires explicit `type: "local"` for stdio.

AGENTS.md / CLAUDE.md host-agent policy:
- offline exact replay may exercise the integration without a provider; live use
  requires `doctor` to report the exact route `bounded_qualified` or
  `study_qualified` plus the two-step authorization digest
- verifier outputs remain auditable advisory evidence; an uncalibrated score,
  margin, or winner never autonomously authorizes retry, apply, merge, or release
- compare multiple solutions only when every candidate shares the same task,
  evidence contract, route, budget, and declared decision policy; preserve ties,
  abstentions, and failures

## Compatibility Contracts

Public surface that must remain stable for consumers; internal-only changes are unconstrained.

| surface | contract |
|---|---|
| MCP tool names | canonical `evalwitness_pairwise`, `evalwitness_absolute`, `evalwitness_delta`, `evalwitness_calibration_evaluate`; deprecated `logprobe_*` aliases expose identical schemas and handlers for one transition release |
| MCP tool input fields | required fields never renamed or removed; new optional fields permitted |
| MCP tool output fields | incompatible legacy naked-score fields ended at the explicit v2 migration; future incompatible changes require a new schema and reader |
| MCP error codes | code-to-meaning mapping fixed; new codes use unused -32xxx slots |
| CLI flags | existing flags never renamed; new optional flags permitted |
| Env var names | `EVALWITNESS_*` canonical; same-suffix `LOGPROBE_*` fallback for one transition release; canonical wins and consumed legacy names produce one redacted warning |
| Go library public API | non-internal package symbols stable; internal/* freely changes |
| Canonical request schema | schema 2 fixed field order, evidence bindings, and portable bytes; incompatible changes require a new schema and vectors |
| Score evidence schema | `evalwitness.score-evidence.v1`; conditional moments and coverage never collapse into one scalar |
| Decision schemas | selection, pair decision, delta, absolute score, Best-of-N, and evaluation use explicit v2 contracts |
| Capture fixture schema | schema 3 exact request/response/score-evidence/checksum records; schema 1/2 inputs are inspection-only |
| Cache entry schema | schema 3 exact request/response/score-evidence records; legacy lookup is explicit and never exact |
| Response bundle schemas | `evalwitness.response-bundle-policy.v1`, `evalwitness.redistribution-evidence.v1`, `evalwitness.response-capture.v1`, `evalwitness.response-bundle-index.v1`, and `evalwitness.response-bundle-verification.v1`; exact bytes, rights evidence, producer identity, omission policy, and request-corpus commitments never weaken in place |
| Legacy cache census | `evalwitness.legacy-cache-census.v1` with `evalwitness.legacy-cache-inventory.v1`; read-only structural classification, separate scientific/operational digests, zero sensitive content, and explicit identity gaps never weaken in place |
| Evidence explorer | `evalwitness.evidence-explorer-report.v2`, `evalwitness.evidence-explorer-canonical-json.v1`, `evalwitness.evidence-explorer-html.v1`, `evalwitness.evidence-explorer-render-metadata.v1`, `evalwitness.evidence-explorer-assets.v1`, `evalwitness.evidence-explorer-public-assets.v2`, and `evalwitness.claim-render-result.v1`; incompatible report, canonicalization, embedding, rendering, presentation capture, or result changes require a new schema version |
| Capability cache schema | each file tags `schema_version`; mismatch triggers re-probe |
| Capability attestation schema | `evalwitness.capability-attestation.v1`; exact route/contract identity, evidence ceiling, integrity, expiry, and limitations never weaken in place |
| Public attestation schema | `evalwitness.public-attestation.v1`; signed derivative never includes private request, response, account, credential, request-ID, or usage material |
| Live authorization schema | `evalwitness.live-authorization.v1`; digest binds the complete route, execution semantics, and hard limits; changed plans require new approval |
| Canonical trajectory schema | `evalwitness.trajectory.v1`; format adapters terminate at typed events/links and incompatible changes require a new schema |
| Ingestion report schema | `evalwitness.ingestion-report.v1`; every source record and loss path remains explicit |
| Transcript fidelity schema | `evalwitness.transcript-fidelity.v1`; source, canonical, usage, and budget-retention evidence remain separable |
| Trace envelope and mapping | `evalwitness.trace-envelope.v1`, `evalwitness.trace-mapping-report.v1`, and `evalwitness.trace-mapping.v1`; external schema versions and loss never become implicit |
| Verification-lineage plan | `evalwitness.verification-lineage-plan.v1`; research questions, units, source classes, cluster isolation, roles, terminal-state precedence, exclusions, replacement, stopping, support, uncertainty, holdouts, claim ceilings, and acquisition boundary cannot weaken in place |
| Verification-lineage schema inventory | `evalwitness.verification-lineage-schema-inventory.v1`; the ten document types, generated schema-body digests, schema IDs, parent cardinalities, same-task boundaries, and acyclic DAG cannot weaken in place |
| Verification-lineage parser lock | `evalwitness.verification-lineage-parser-lock.v1`; exact production source SHA-256 bindings, six reproduced governance artifacts, development-vector and conformance counts, frozen formats/surfaces, pre-calibration state, and post-lock change rule cannot weaken in place |
| Verification-lineage source readiness | `evalwitness.verification-lineage-source-readiness-audit.v1`; sealed inventory and parser-lock bindings, zero-admission denominator, non-inferential per-format readiness, external-action boundary, and empirical non-claims cannot weaken in place |
| Verification-lineage holdout readiness | `evalwitness.verification-lineage-holdout-readiness-audit.v1`; deterministic selection, contamination findings, candidate-universe status, zero outcome counts, and transfer non-claims cannot weaken in place |
| Verification-lineage corpus feasibility | `evalwitness.verification-lineage-corpus-feasibility.v1`; readiness bindings, frozen 20/20 threshold, current 40-group shortfall, protocol-v2 requirements, and future-impossibility non-claim cannot weaken in place |
| Verification-lineage capability matrix | `evalwitness.verification-lineage-capability-matrix.v1`; pinned specification and observed-development axes, exact 30-field counts, per-row vector evidence, mapping-diff identities, and format-wide non-claims cannot weaken in place |
| Verification-lineage offline proof | `evalwitness.verification-lineage-offline-proof.v1`; readiness bindings, accepted five-layer positive lineage, non-failable counterexample, exact survival vectors, and development-only claim boundary cannot weaken in place |
| Verification-lineage loss certificate | `evalwitness.verification-lineage-loss-certificate.v1`; offline-proof identity, exact counterexample, semantic failability, first-loss layer, public-safety state, and unsupported claims cannot weaken in place |
| Verification-lineage graph | `evalwitness.verification-lineage-graph.v1`; offline-proof and loss-certificate identities, exact two-path development projection, evidence digests, non-empirical state, and no-provider-ranking boundary cannot weaken in place |
| Verification-lineage limitations | `evalwitness.verification-lineage-limitations.v1`; offline-proof identity, negative feasibility, development role, seven sorted evidence boundaries, required resolutions, and zero-provider/agent reproduction cannot weaken in place |
| Mutation artifacts | `evalwitness.mutation-manifest.v1`, `evalwitness.mutation-witness.v1`, `evalwitness.blind-review-packet.v1`, and `evalwitness.mutation-reduction.v1`; incompatible changes require new schemas and mutation-program identity |
| Controlled corpus | `evalwitness.corruption-corpus-spec.v1` and `evalwitness.corruption-corpus-release.v1`; program, design, audit, lineage, counts, controls, and release digest cannot weaken in place |
| Metamorphic stress lab | the 35 closed `evalwitness.stress-*` schemas, canonical v3 catalog/arm/design/held-out identities, complete cell accounting, non-authorizing structural campaign binding, admission-filtered execution binding, preflight evidence/custody, exact private capsule-family, authority-bound execution-permit v2, atomic reservation, permit-bound live evidence, independently executed exact-replay verification, denominator-conserving execution ledger, readiness-refusal, complete-support run-seal v1, admission-filtered authority-bound run-seal v2, exact next-catalog routing, one-minimal counterexample, development-case-study, self-contained challenge, and deterministic receipt invariants cannot weaken in place |
| Evidence reliance | `evalwitness.evidence-reliance-map.v1`, `evalwitness.reliance-profile-projection.v1`, `evalwitness.reliance-paper-projection.v1`, `evalwitness.reliance-explorer-projection.v1`, the public child-capsule binding, and CLM-035 through CLM-045; registered denominators, arm isolation, selector boundaries, one-minimality scope, E1 mechanism role, and transfer/causality/global-score prohibitions cannot weaken in place |
| Outcome artifacts | `evalwitness.outcome-*.v1` plus objective-correct pilot sample `.v2`, source binding `.v2`, and readiness `.v3`; record/evidence and strict drafts, blind request/packet/private mapping, label/draft/batch, qualification, reviewer handbook/kit/record, historical and outcome-only pilot contracts, sealed owner-only pilot private materials, reviewability inspection, review bundle/assignment, rubric-ambiguity analysis, blinding protocol/probe/analysis, mapping reveal, adjudication ledger, post-ledger source audit, resolution/agreement, natural inventory, execution log, preservation, and sample commitments are content-addressed and incompatible changes require new schemas |
| Verifier audit protocol | current `1.2.0`, previous minor `1.1.0`; incompatible majors reject and levels never imply one another |
| Protocol canonical JSON | `evalwitness.protocol-canonical-json.v1`; integer-only JSON numbers and canonical decimal strings are stable |
| Protocol extension | reverse-domain namespace plus schema and required flag; unknown required fails, unknown optional is preserved |
| Route namespace | SHA-256 of length-prefixed provider/model bytes; `identity.json` must self-validate against its directory ID |
| Cache ownership | canonical marker schema/product/root ID plus protected-path and owner-only permission validation |
| Legacy cache roots | read-only exact-key import; never a write, stats, or clear target |
| Archive intake | complete inspect pass plus digest-stable staged extraction; atomic publish to absent destination only |
| Artifact review | schema 1 exact-content manifest over sorted rule/path/line/file-SHA-256 findings; changed, missing, stale, or additional findings fail |
| Pricing table format | embedded JSON shape stable; entries may be added/updated |

## Security and Privacy

Key handling:
- API keys read only from env vars, `.env`, or the selected EvalWitness TOML/JSON config file
- Keys never accepted via CLI flags (would land in shell history) -> reject with hint
- Keys never logged; stderr logger applies redaction filter on every line
- Keys never written to cache, fixture, audit, or capability-probe files
- Cache roots/directories use mode `0700`; ownership markers, route identities, responses, and capability records use mode `0600`
- Archive staging/extraction, audit logs, replay captures, persistent budget state, and every Best-of-N manifest/transcript/diff use owner-only modes; release archives and checksum manifests use explicit public modes
- Best-of-N child environment is allowlisted; extra values require `--pass-env`; literal secret-bearing argv rejects; transcript and diff capture are bounded; process-group cancellation and exact retained-path cleanup reporting are mandatory
- Public artifact mode requires explicit public classification; no path, extension, or caller default promotes sensitive data to public

Trajectory data:
- Redaction blocklist applied before any provider call (Trajectory Preprocessing pipeline)
- Cache files local-only; never uploaded
- No telemetry, no usage analytics, no remote calls beyond the configured LLM provider
- `EVALWITNESS_OFFLINE=true` env var forbids any network call; only cache-served requests succeed

Filesystem:
- Protected-path validation precedes marker trust and every destructive cache operation
- Cache mutation is confined below an opened `os.Root`; escaping symlinks and parent traversal fail closed
- Clear has no default scope and never recursively deletes the configured root
- Legacy raw-provider layouts are bounded read-only compatibility inputs; new writes use hashed route namespaces only
- Archive entries allow regular files/directories only; reject absolute/traversing/non-NFC names, separator variants, links, devices, FIFOs, xattrs, duplicates, case-fold collisions, file/directory-prefix collisions, resource-limit excess, and insufficient reservation space
- `artifact scan` emits no matched bytes and reports text/opaque counts; text scanning covers credential shapes, registered environment secrets, environment dumps, and workstation paths; opaque scanning covers exact registered environment-secret bytes because binaries embed generic detector patterns; types, symlinks, and modes remain universal checks; release admission also requires the tracked-source safety gate, clean embedded provenance, and byte-identical double builds

Untrusted input profiles:
| kind | max bytes | additional invariant |
|---|---:|---|
| protocol adapter | 8 MiB | operator owns process launch, environment, cwd, network, output, and live capability |
| trace | 256 MiB | content remains metadata-first until privacy authorization |
| mutation artifact | 16 MiB | strict schema, no trace-selected execution, content-addressed identity, explicit license/privacy and split lineage |
| outcome artifact | 16 MiB | strict schema; public packet separated from HMAC key and non-overwriting owner-only private mapping; executable contract selected only by trusted registry |
| attribution | 64 MiB | contribution data cannot select evaluation behavior |
| capsule | 512 MiB | archive policy applies before JSON policy |
| policy | 4 MiB | strict JSON and no executable configuration |
| static report | 64 MiB | relative contained links, bounded markup, escaped rendering |

Every profile also requires positive depth, total-node, string, array, object, markup, and link limits. JSON preflight rejects non-regular files, symlinks, oversize before decoding, malformed or trailing values, and over-limit trees. Any untrusted command, environment, working directory, network destination, output path, or live-mode selection returns `untrusted_control`.

Network:
- HTTPS only; reject HTTP base URLs unless `EVALWITNESS_ALLOW_INSECURE=true` (Ollama localhost dev)
- Default Go TLS config; respect system CA bundle
- Honor `HTTPS_PROXY` / `HTTP_PROXY` env vars; no auto-detection

Audit log (optional):
- `EVALWITNESS_AUDIT_LOG=path/to/audit.jsonl` -> checked `provider_attempt`, `provider_call`, and `verification_run` JSON rows
- Provider-attempt fields include exact request fingerprint, attempt ordinal/status, usage, trajectory evidence, and redacted error
- Provider-call identity includes response/served/request IDs, replay/cache/parser status, strict score evidence, request lineage, and inconsistent-pair contribution
- Verification-run identity includes run/request-set fingerprints, decision policy/state/abstention, aggregate budget, output status, cleanup-before-audit lifecycle, and ordered research lineage
- `redaction_hits` counts preprocessing patterns; >5 hits per trajectory triggers a stderr warning to surface false positives
- No prompt content, response content, keys, or caller overrides of service-generated evidence

Log redaction filter: shared credential patterns plus literal values registered from runtime configuration; recursively applies to message text, record/bound attributes, groups, errors, URLs, headers, `LogValuer` output, maps, and slices before the wrapped handler receives them.

Threat model: trusted local user, untrusted upstream provider, untrusted trajectory content. Out of scope: kernel-level secret protection, hardware key storage, multi-user separation.

## Verifier Reliability Profile

Schema `evalwitness.profile.v1.schema`; type `VerifierReliabilityProfile`; canonical `evalwitness.profile.v1`.

| field | type | description |
|---|---|---|
| schema_version | string | `evalwitness.profile.v1.schema` |
| identity | string | profile identity (`reference`, `development`, `degraded`) |
| protocol_version | string | TASK 050 capsule protocol version |
| route_scope | string | evaluated route/adapter scope |
| time_window | string | profile time window |
| domains | []string | task domains |
| data_roles | []string | data roles (calibration held-out, relation, outcome) |
| evidence_levels | []string | `E1`–`E4` evidence levels present |
| capsule_parents | []string | capsule digests parents, sorted |
| digest | string | `sha256` over sorted Dimensions/Domains/DataRoles/EvidenceLevels/CapsuleParents and full profile |
| dimensions | []Dimension | machine-readable evidence dimensions, sorted by ID |

`Dimension` fields: `id`, `status` (`measured`|`failed`|`unsupported`|`not_applicable`|`not_measured`), `metric` (required when `measured`), `interval`, `sample_unit`, `denominator`, `scope`, `policy`, `evidence_level`, `capsule_expression`, `caveats`. `Build` validates `measured` requires `metric/scope/evidence_level/capsule_expression` and computes deterministic `Digest`; `Verify` requires non-empty `Digest` and equality; `Diff` refuses incompatible `protocol_version`/`route_scope` and sorts `added/removed/changed`; `Evaluate` enforces `policy.version/digest` and fails on missing dimensions with `upper bound` semantics for `selective_risk` and `coverage`.

Policy files are strict, versioned, canonical, content-addressed `sha256`; unknown dimensions default to failure unless explicitly permitted. `profile build`, `profile verify`, `profile diff`, `profile policy`, `profile render` operate offline; `ToOTel` emits `evalwitness.profile` OTel event without private evidence; `TextReport`/`MarkdownReport` render sorted dimensions without global score. Mutation deletes must fail verification/policy; every dimension preserves `capsule_expression` for `claimcheck`.

Offline CI audit contract: `evalwitness audit --policy FILE --profile FILE [--format json|junit|markdown]` decodes both inputs with `DisallowUnknownFields`, recomputes the policy content digest (`sha256` over version plus canonical requirements JSON) and accepts a declared `digest` only when it matches, prints the execution plan before evaluation, and returns the stable exit contract 0 pass / 1 policy failed / 2 invalid input / 3 internal error. JUnit keeps statistical route failures as aggregate cases without fabricated source locations; SARIF is emitted only for findings with a real repository file and location. The composite GitHub Action runs only a pinned, checksum-verified release binary offline; it never reads provider credentials and has no build-from-source fallback.

## Glossary
| term | definition |
|---|---|
| Trajectory | a complete agent run for one task, captured as text or JSON steps; synonym Trace |
| Trial | one of N independently-generated trajectories for the same task |
| Pair | unordered combination (i, j) of two trial indices in pairwise mode |
| Criterion | a named evaluation dimension with description (e.g. specification adherence, output match) |
| Rep | one full execution of a (pair, criterion) prompt; multiple reps reduce variance |
| Score Token | a single letter A-T emitted by the LLM at the score tag position |
| Score Tag | XML-style marker like `<score_A>` whose following position holds the score token |
| Distribution | map from token to probability mass at a given logprob position |
| Expectation | mass-weighted mean of token values; basis for [0,1] score |
| Logprob | natural log of token probability returned by provider |
| Position-Finding | algorithm to locate the token index following a given tag in the chosen-token stream |
| Capability | runtime-detected feature flag (Logprobs, LogitBias, PromptCache) of a (provider, model) |
| Tournament | full or pruned round-robin of pairwise comparisons producing a single winner |
| Tournament Win | tally credit awarded to the higher-scoring trajectory in a pair; ties split 0.5/0.5 |
| Decision Strength | uncalibrated folded margin used by the current bounded policy; never correctness probability |
| Evidence Strength | observation count and minimum/mean Top-k coverage summary; not confidence |
| Probe | minimal capability-detection request issued before first real use |
| Judge Mode | explicit independent text-only request and cache/replay identity; never verifier fallback |
| Capability Cache | persisted weak diagnostic probe results; never qualification evidence |
| Capability Attestation | exact, expiring, integrity-bound observation that one real score request satisfied a declared verifier contract |
| Live Authorization | two-step digest approval binding one network entrypoint to one route, request contract, execution plan, and hard limits |
| MCP | Model Context Protocol; JSON-RPC 2.0 server-tool interface used by Claude Code, Codex, OpenCode |
| Subscription Mode | pricing-bypass mode for fixed-fee plans; reports est_cost=0 |
| Replay | deterministic test mode where provider calls are served from recorded fixtures |
| Adaptive Pair Decision | K=1 base decision using Top-20 expectation and variance, with uncertainty-triggered escalation capped at four calls |
| Controlled Mutation | deterministic typed transformation with declared affected fields, expected relation, validation witness, lineage, and content identity |
| Mutation Witness | formal, trusted executable, or preservation evidence establishing the scoped relation without using evaluator output as ground truth |
| Blind Review Packet | sealed pseudonymous review input that omits mutation family and expected relation |
| Outcome Record | immutable revision of typed outcome evidence, resolution basis, limitations, and provenance |
| Blind Outcome Packet | HMAC-rekeyed reviewer input separated from its owner-only private source mapping |
| Qualification Report | content-addressed reviewer-specific score binding study labels to the governed rubric exercise |
| Reviewer Handbook | content-addressed frozen outcome definitions, decision procedure, conflict/blinding policies, checklist, and dataset statement bound to the qualification set |
| Review Bundle | content-addressed blinded packet set binding the plan, rubric, handbook, data role, visibility, and task clusters before assignment |
| Reviewer Kit | self-contained reviewer-specific artifact binding the exact handbook, assignment, and blinded packets in committed order without mappings, seeds, evaluator output, peer labels, or study answers |
| Adjudication Ledger | terminal content-addressed commitment graph binding independent assignments, complete label batches, post-commit reveal, agreement, resolutions, and unresolved cases |

## Out of Scope (initial release)

- HTTP server mode (stdio MCP only initially)
- Hosted or network-backed dashboard
- Built-in trajectory recorder (consumes traces, does not capture them)
- Multi-tenant key management
- Authentication beyond bearer token
- Persistent rate-limit accounting across sessions
- SQLite-backed cache (JSON files only initially)
- Telemetry / analytics
- Auto-update mechanism
- Process-Reward-Model mode (per-step scoring)
- Cross-provider self-consistency voting (added in v2)
