# EvalWitness Documentation

Living SSOT for runtime documentation. Spec lives in `spec.md` and is the target-state blueprint; this file documents what is currently shipped.

## Sections

- Overview
- Program Charter and Identity
- Research Contract
- Identical-response study
- Terminology
- Evidence and Claim Governance
- Target Architecture
- Offline Evidence Explorer
- Closest Work and Contribution Boundary
- Evidence architecture and publication gates
- Install and Build
- Configuration
- Subcommands
- Agent Self-Checks
- MCP Server Integration
- Agent Skill
- Provider support appendix
- Architecture
- Score Evidence Contract
- Verifier Audit Protocol
- Modes
- Optimizations Active
- Order-Bias Mitigation
- Multi-Criterion Bundling
- SPRT Adaptive Reps
- Best-of-N Orchestration (bon)
- Judge Mode
- Cost Model
- Audit Log
- Evaluation Harness
- Study Governance
- Signal Reliability
- Zero-Cost Baselines
- Statistical Discipline
- Evidence-Reliance Audit
- Determinism and Replay
- Cache and Preprocessing
- Trace Interoperability
- Verification Lineage
- Controlled Corruption Benchmark
- Metamorphic and Differential Stress Contracts
- Controlled Relation Construct Audit
- Coding-Agent-Only Formal Study
- Outcome Validity and Blinded Adjudication
- Security and Privacy
- Release and License

## Overview

EvalWitness is a provider-portable, reproducible verifier audit lab for
coding-agent trajectories. Its primary research
objects are content-addressed experiment capsules, a canonical claim ledger,
executable claim challenges, and complete verification lineage. One fingerprinted
verification application service remains available as an audited subsystem and
exposes pairwise, absolute, and delta decisions through CLI, MCP, benchmark
evaluation, Best-of-N, and the offline application-protocol adapter without
entrypoint-specific scoring logic.

It is both a production-oriented Go 1.27 evidence system and a bounded research
instrument: the product proves how decisions are produced, while the admitted
studies state exactly which empirical question was measured and which broader
questions were intentionally not run.

Provider-portable means that requests, score evidence, traces, attestations, and
claims use versioned contracts. It does not mean that every provider or
OpenAI-compatible endpoint currently works. Reproducible means the current
provider-free capsule, Claim Autopsy, challenges, exact replay, development
release, and admitted identical-response v5 graph can be verified deterministically. The
v5 graph has an author-operated clean-clone proof; it does not mean that the
historical paper result, a human study, a signed downloadable release, or an
independent replication currently exists.

### Empirical provenance

The admitted identical-response v5 bundle is complete for its declared
bounded estimand: 60 locked task groups, strict Top-20 evidence, 53 agreements,
7 disagreements, and 0 unresolved groups. It was captured on one attested
`bai/deepseek-v4-flash` route; the exact route identity and attestation remain in
the machine artifacts as provenance, not as a product or model ranking.

The capture, response bytes, and derived analyses are project artifacts covered
by the repository license when distributed. The exact B.AI route remains technical provenance only;
it does not imply provider endorsement, model-weight redistribution, or a
provider-quality claim. External publication remains an explicit repository-
owner action and does not change the empirical admission state.

Provider labels are separate from the HTTP wire format. `EVALWITNESS_PROVIDER` is an arbitrary label used for env-key lookup, hashed route-namespace identity, and audit fields. Raw provider and model identifiers are stored in owner-only route metadata and never become path components. `EVALWITNESS_WIRE_FORMAT` is intentionally `openai` only: evalwitness requires the implemented Chat Completions response shape with output-token `logprobs` / `top_logprobs`. A base URL and model can be configured without adding a wire adapter, but that is not a compatibility claim. The exact route qualifies only after `evalwitness attest` records strict score evidence for the exact request contract.

Anthropic-style Messages APIs are not an implemented EvalWitness wire format.
The current verifier path requires score-token probability fields in the
implemented response contract; text-only routes belong to the explicitly
separate judge treatment and cannot be relabeled as verifier evidence.

## Program Charter and Identity

The runtime identity is **EvalWitness**, with canonical slug, binary, repository,
Go module, MCP server, and configuration prefix `evalwitness`. The complete
current surface uses this identity under an explicit one-release compatibility
contract. The name describes the program contract: each evaluator result
must carry a reviewable witness that identifies the input, route, response,
analysis, uncertainty, and policy that produced it. It does not imply that a
model output is ground truth.

The target program keeps four coupled outputs over one evidence system. Only
outputs present in the verified release manifest are current capabilities:

1. A verifier instrument that qualifies score-token routes, compares coding-agent
   trajectories, abstains when evidence is insufficient, and replays decisions
   offline.
2. An open audit protocol and controlled corruption benchmark for testing any
   compatible evaluator against distribution-preserving and gold-by-construction
   cases.
3. A machine-readable reliability profile, policy-as-code CI kit, and offline
   evidence explorer for external agent projects.
4. Research artifacts for bounded extraction semantics, controlled robustness,
   evidence reliance, and explicit uncertainty under locked studies. Transfer,
   drift, and broader validity are separate future study contracts.

The product contract is:

> No route, decision, confidence, benchmark result, or public claim is trusted
> without machine-readable evidence identifying the input, request, provider
> capability, response, binary, analysis, and applicable uncertainty.

The public information architecture has three non-competing reader paths over
the same capsule and claim identities:

| Reader | First question | Current evidence entrypoint | Ceiling on entry |
|---|---|---|---|
| Engineer | Can this named adapter, route, and retained trace satisfy the strict verifier contract? | protocol conformance, exact replay, route attestation, and mapping-loss report | E1 offline; E2 only for a current named live attestation |
| Researcher | Which scoped verifier claims survive controlled falsification, uncertainty, and evidence loss? | capsule-derived Claim Autopsy, governed findings, and the admitted identical-response v5 package | E1 provider-free mechanism evidence plus the admitted E2 identical-response scope; no broader E4 claim |
| Reviewer | Can one claim be traced to parents, broken in an ephemeral copy, and withdrawn at the exact guard? | five-minute terminal proof and self-contained offline explorer | E1 local mechanism conformance, never independent reproduction |

### Identity decision

The identity audit was performed on 2026-08-08. Availability is a point-in-time
engineering check, not trademark clearance or a promise that a domain remains
available.

| Surface | Evidence checked | Decision |
|---|---|---|
| Existing name | The active `Robby955/logprobe` repository and `logprobe` crate diagnose LLM logprob normalization, missing mass, and entropy bias | Collision confirmed in the same technical field; do not publish under `logprobe` |
| GitHub repository | Exact in-name GitHub API search returned zero repositories for `evalwitness` | Clear at check time |
| Package names | PyPI, npm, crates.io, and Go module proxy returned no exact `evalwitness` package | Clear at check time; recheck immediately before publication |
| Executable | `command -v evalwitness` returned no executable; exact Homebrew formula and cask searches returned none | Clear on the development machine and Homebrew indexes |
| MCP prefix | GitHub code search returned no `evalwitness_` MCP tool prefix | Clear at check time |
| Searchability | Exact web and GitHub repository searches returned no software product named EvalWitness | Distinctive enough for technical search |
| Domains | Registry RDAP returned not found for `evalwitness.com`, `.ai`, `.io`, and `.dev`; DNS was empty for all four | Potentially available at check time; no domain was purchased |
| Legal | No counsel-led trademark clearance was performed | Publication and commercial use require a fresh registry and legal review |

The canonical surfaces are `EvalWitness`, `evalwitness`,
`github.com/Christopher-Schulze/evalwitness`, `evalwitness_*` MCP tools,
`EVALWITNESS_*` environment variables, `evalwitness.toml` or
`evalwitness.json`, `~/.config/evalwitness`, and a cache root named
`evalwitness`. Any new public surface must follow this identity contract and the
automated residue gate.

### Migration contract

| Surface | Target | Compatibility rule |
|---|---|---|
| Repository and Go module | `github.com/Christopher-Schulze/evalwitness` | Rename once, update every import atomically, verify the GitHub redirect separately; old Go imports are not claimed compatible |
| CLI | `evalwitness` | No legacy executable is built: no public consumer existed before the rename; scripts accept only the canonical binary except for explicitly tested local override variables |
| MCP server and tools | server `evalwitness`; tools `evalwitness_pairwise`, `evalwitness_absolute`, `evalwitness_delta`, `evalwitness_calibration_evaluate` | Legacy tool names dispatch to the same handlers for one transition release and are marked deprecated in descriptions |
| Environment | `EVALWITNESS_*` | New name wins on conflict; old `LOGPROBE_*` is read only when the new counterpart is absent and emits one process-level redacted warning |
| Config files | `evalwitness.toml`, `evalwitness.json`, `~/.config/evalwitness/.env` | New files win; legacy files are fallback-only and never rewritten |
| Cache and replay | new cache namespace and new request identity | Legacy cache is read-only import evidence; no new write enters the old namespace and no cross-name hit occurs without an exact request fingerprint |
| Schemas | new `evalwitness.*` schema names only for genuinely revised contracts | Existing `logprobe.reliability.v1` remains immutable and readable as a legacy schema |
| Skills and MCP configs | `evalwitness-audit` skill and EvalWitness command examples | Only canonical skill/config assets ship; MCP tool aliases carry the compatibility boundary |
| Release assets | `evalwitness-{os}-{arch}` and checksums | No public asset is emitted without explicit publication authorization; legacy asset aliases, if shipped, are byte-identical |
| Historical artifacts | Preserve original names, route fields, and digests | Never rewrite history to simulate provenance that was not recorded |

The verified pre-migration inventory contained 74 non-ignored files with
`logprobe`, `LogProbe`, or `LOGPROBE`, 63 distinct `LOGPROBE_*` tokens, three
public MCP tools, `cmd/logprobe`, the old Go module, config/cache paths, the agent
skill, build/release assets, result schemas, scripts, docs, and result artifacts.
The post-migration residue gate permits the old identity only in tested
compatibility readers, immutable historical schemas/artifacts, and explicit
migration documentation. Historical completed tasks and immutable result content
remain unchanged.

## Research Contract

The frozen research question is:

> Under provider, trace-format, presentation, and controlled trajectory-quality
> interventions, which properties of truncated score-token distributions remain
> stable and decision-useful, and can a calibrated selective policy meet a
> prespecified held-out risk-coverage target while exposing reproducible failure
> witnesses and measured evidence-reliance maps for coding-agent trajectory
> evaluation?

Existing artifacts are development evidence. They may shape hypotheses and
engineering tests, but they are not the held-out test set and confirm none of the
hypotheses below.

| ID | Primary hypothesis | Null hypothesis | Primary outcome |
|---|---|---|---|
| H1 | Provider or gateway changes alter score evidence under a fixed requested alias | Score-evidence distributions are exchangeable across matched routes | Prespecified cross-route distribution and decision contrasts |
| H2 | Valid-score mass and truncation state add calibration information beyond conditional expected score | Mass-aware features add no held-out calibration or discrimination value | Held-out log loss, Brier, ECE, AUC, and risk-coverage difference |
| H3 | A leakage-free calibrated selective policy meets a prespecified error target at useful coverage | The target selective risk is exceeded or coverage is below its floor | Task-cluster upper confidence bound on selective error and achieved coverage |
| H4 | A bounded real score task predicts later extraction-contract failures better than a capability declaration | Attestation status has no association with subsequent failure or degradation | Failure and degradation rates joined to attestation state |
| H5 | CLI, MCP, eval, protocol adapter, and Best-of-N are equivalent for canonical inputs | At least one entrypoint changes request identity, score evidence, or decision | Cross-entrypoint conformance failure rate and exact fingerprint mismatch |
| H6 | Executable defects move decisions more reliably than deceptive success prose | Defect and prose interventions have equal outcome-conditioned effects | Paired defect sensitivity and deceptive-claim resistance |
| H7 | Semantics-preserving trace transformations expose instability not visible in benchmark accuracy | Expected metamorphic relations hold within their locked tolerance | Relation violation rate by family and reduced counterexamples |
| H8 | Mass-aware score evidence improves selective correctness and robustness | Mass-aware and score-only policies have equal held-out performance | Locked paired risk-coverage and relation-robustness contrasts |
| H9 | Compact canary drift predicts loss of policy applicability or relation stability | Canary drift has no prespecified predictive association | Longitudinal applicability failures and held-out relation changes |
| H10 | Verifier output relies more on executable outcome evidence than persuasive success prose when executable quality is held fixed | Outcome-evidence and prose interventions have equal distribution and decision effects | Factor-level reliance contrasts, interactions, invalid-case rate, and minimal witnesses |

Engineering evidence proves that code behaves under a named fixture, replay, or
bounded route observation. Research evidence tests a frozen estimand against an
independent outcome under a locked manifest, declared exclusions, task/source
clustering, multiplicity control, and an immutable capsule. Passing unit tests
cannot establish transfer, calibration, superiority, causality, or novelty.

The program does not train a foundation model, replace human evaluation, provide
an RL reward before calibration, reproduce the original paper's absolute Gemini
numbers, support every OpenAI-compatible endpoint, build a generic eval platform,
host an observability backend, invent a universal trace format, publish a provider
leaderboard, or issue a scalar trust certification. It does not infer causal
responsibility for agent steps or model-internal reasoning from changes in
verifier outputs.

## Terminology

| Term | Canonical meaning |
|---|---|
| route | One addressable serving path defined by provider label, gateway, base URL, requested model, wire contract, and relevant configuration |
| provider | Organization or local system operating the serving endpoint; not inferred from an arbitrary config label |
| gateway | Intermediary that authenticates, routes, transforms, or meters a request before the model-serving backend |
| requested model | Identifier sent in the request; never treated as proof of the backend checkpoint |
| served model | Identifier returned by the endpoint for the response; may still be an alias |
| checkpoint assertion | Provider or artifact statement that binds a served response to a specific model checkpoint, with issuer and verification limits |
| capability attestation | Time-bounded result of executing the required score task against a route and recording the observed contract |
| score evidence | Token-aligned visible probability support, valid-score support, mass, truncation state, conditional moments, and extraction limitations |
| decision | Policy output selecting, tying, abstaining, or failing, bound to its score evidence and policy version |
| run | One execution under one frozen runtime configuration and authorization digest |
| study | A prespecified collection of runs, cells, outcomes, and analyses answering named hypotheses |
| capsule | Content-addressed provenance graph binding observations, derivations, checksums, visibility, and claims |
| claim | Human-readable assertion with exact scope, status, evidence level, capsule expressions, uncertainty, caveats, and guarding test |
| trace | Ordered source or canonical event stream describing an agent execution; not synonymous with a raw transcript or code attribution record |
| attribution record | Agent Trace-compatible record linking code ranges or changes to human or AI contribution provenance |
| audit case | Versioned evaluator input, expected structural properties, admissibility rules, and outcome evidence |
| metamorphic relation | Expected relationship between evaluator outputs before and after a declared input transformation |
| mutation witness | Executable, formal, or independently adjudicated evidence that a controlled transformation changed the intended factor |
| outcome evidence | Evidence independent of evaluator output that establishes task or transformation outcome within stated limits |
| reliability profile | Versioned multidimensional projection of scoped evidence; never a universal scalar certificate |
| evidence factor | Typed component of visible trajectory evidence whose influence is a study target |
| evidence intervention | Validated removal, masking, or replacement of one evidence factor under preserved lineage and outcome constraints |
| reliance contrast | Prespecified change in score distribution, decision, or abstention under an admissible evidence intervention |
| reliance map | Collection of factor-level reliance contrasts with denominators, uncertainty, interactions, invalid cells, and scope |
| policy | Versioned machine-readable acceptance, abstention, or CI rule over compatible profile and capsule fields |
| registry entry | Signed, offline-verifiable projection of a capsule and protocol result accepted under explicit governance |
| reproduction | Re-execution of the same released capsule, code, data, and analysis contract to obtain the scoped result |
| replication | Independent application of the frozen protocol to a different implementation, route, actor, or governed dataset |

`model` alone is forbidden in scientific claims. Use `requested model`, `served
model`, `model family`, or `checkpoint assertion`. `Confidence` alone is also
forbidden; name the raw win probability, folded decision margin, calibrated
probability, interval, or policy output. `Paper parity` means prompt and pipeline
conformance only unless an independently reproduced empirical result is named.

## Evidence and Claim Governance

### Evidence ladder

| Level | Evidence | Permitted claim scope |
|---|---|---|
| E0 | Source or configuration inspection | Configured capability only, never executed behavior |
| E1 | Unit, property, fuzz, deterministic replay, or committed derived artifact with a guarding test | Local behavior under the exact fixture or artifact |
| E2 | Bounded live conformance run with complete score evidence | Route capability at the recorded time and request shape |
| E3 | Locked held-out run on one attested route | Route-specific empirical result with uncertainty |
| E4 | Prespecified multi-provider and multi-family study | Transfer or heterogeneity within the studied matrix |
| E5 | Independent clean-clone reproduction from a released capsule | Reproducibility of that exact release |
| E6 | Independent actor applies the frozen protocol to a separate implementation, route, or governed dataset | Scoped independent replication or interoperability |

Claim status is independent of level. `supported` means wording and scope do not
exceed available evidence. `exploratory` means real evidence exists but it is
development-only, incomplete, or used to generate a future hypothesis.
`unsupported` means the current evidence cannot carry the assertion.
`superseded` means a narrower or corrected claim replaces it while history
remains visible. The machine-readable reference ledger enforces those states:
assertable claims require non-presence-only expressions, current generations,
usable attestation state, exact evidence ceilings, and executable negative
controls. Unsupported and superseded wording can remain visible only with its
recorded status and caveat.

### Canonical claim ledger projection

<!-- evalwitness:claim-surface:documentation:begin -->
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
| CLM-009 | supported | E1 | number:589 | The two primary legacy verifier artifacts bind a total of 589 benchmark tasks | Dataset-only legacy scope; incomplete run provenance caps this claim at E1 |
| CLM-010 | exploratory | E1 | number:0 | The two primary legacy verifier artifacts record zero unextracted scores | Quote only with exact artifact, route, date, decidable count, and legacy-provenance caveat |
| CLM-011 | exploratory | E1 | number:15/14 | The recorded Terminal-Bench verifier-to-random score ratio is exactly 15/14 | Terminal-Bench development artifact only; no pooled or method-general statement |
| CLM-012 | supported | E1 | number:0 | The two primary legacy verifier artifacts record zero pair-decision ties | The 240-decision denominator is governed separately by CLM-017 |
| CLM-013 | unsupported | E1 | number:240 | Reading the distribution improves selection accuracy over reading the emitted letter | Current paired tests do not support superiority |
| CLM-014 | unsupported | E1 | string:unavailable_public_source | Verifier and judge arms are exact response-identical counterfactuals | Exact response identity requires bound response bytes and TASK-043 fingerprints, which the public reference lacks |
| CLM-015 | unsupported | E1 | boolean:false | Raw win_probability is calibrated confidence or a valid correctness threshold | Development calibration curves invalidate a calibrated-confidence interpretation |
| CLM-016 | supported | E1 | number:53/64 | The recorded Terminal-Bench development AUC is exactly 0.828125 under the committed artifact | Development-only and task-clustered; other metrics remain separate ledger expressions |
| CLM-017 | supported | E1 | number:240 | The two primary legacy verifier artifacts contain exactly 240 pair decisions | Exact committed artifacts only; no population denominator is implied |
| CLM-018 | supported | E1 | string:absolute | The committed Terminal-Bench absolute artifact declares the absolute selection strategy | Association and quality conclusions remain separate and inherit legacy provenance limits |
| CLM-019 | supported | E1 | number:7 | The most-patch-hunks baseline descriptively solves seven more decidable SWE-bench tasks than the verifier in the committed artifact | Say outscores in this artifact; do not claim causal superiority |
| CLM-020 | unsupported | E1 | number:56 | The hunk-count heuristic is generally better than the verifier | The paired comparison and external validity do not support a general claim |
| CLM-021 | unsupported | E1 | number:6 | Removing size clauses caused the six-task SWE-bench score increase | The six-task crossing is descriptive; causality is unsupported |
| CLM-022 | supported | E1 | number:101/1712 | The committed verifier-to-paper-parity call-count ratio is exactly 101/1712 | Descriptive development comparison only; no empirical equivalence claim |
| CLM-023 | unsupported | E1 | number:0 | A third pair call buys nothing, or two calls are universally sufficient | Zero recorded third calls is a bounded sweep observation, not universal sufficiency |
| CLM-024 | unsupported | E1 | number:25/26 | A 16k evidence budget is harmless or optimal | The available sample cannot exclude harm or establish an optimum |
| CLM-025 | supported | E1 | number:85/97 | The committed Terminal-Bench absolute-to-default call-count ratio is exactly 85/97 | No universal efficiency or quality claim; the score difference remains separately guarded |
| CLM-026 | superseded | E1 | string:not_run | Passing a short capability probe qualifies a route | Short probes are diagnostics only and cannot qualify a route |
| CLM-027 | unsupported | E1 | string:unavailable_public_source | Three surveyed routes passed short probes and failed production-shaped score extraction | The public capsule lacks the historical response and attestation bytes needed to assert this survey result |
| CLM-028 | unsupported | E1 | string:not_run | Dry-run, replay, or a config catalog proves live provider capability | Only a bounded live E2 attestation can establish route capability |
| CLM-029 | unsupported | E0 | boolean:false | Current filesystem, archive, transcript, logging, and release behavior is safe for untrusted public use | The reference proves bounded artifact handling, not blanket safety for untrusted public use |
| CLM-030 | unsupported | E1 | string:not_run | Warm-cache regeneration is a complete scientific reproduction | Replay and collection clocks are separate; clean-clone reproduction remains required |
| CLM-031 | unsupported | E1 | string:not_run | The repository reproduces the paper's headline empirical results | Reference conformance and dataset tallies are not headline empirical reproduction |
| CLM-032 | unsupported | E1 | number:0 | Findings transfer across providers, gateways, model families, checkpoints, or time | Transfer requires the prespecified multi-provider and longitudinal studies |
| CLM-033 | unsupported | E0 | string:not_run | Evidence-reliance effects establish agent-step causality or model-internal explanation | Agent-step causality and model-internal explanation are permanently outside claim scope |
| CLM-034 | unsupported | E0 | string:not_authorized | Downloadable releases currently exist at the README release URL | The local repository has no publication proof; release publication requires explicit authorization |

View digest: `95eca85fec41163bdf14d8cbb1e1c406495e128dab7dc4beac1818403c7cd78f`

<!-- evalwitness:claim-surface:documentation:end -->
Six surfaces are machine-tracked projections of the reference ledger: README,
`docs/findings.md`, this document, `docs/releasing.md`,
`eval/results/claim-surface-v1.json`, and the EvalWitness audit skill. The wider
surface census also covers `docs/spec.md`, result and evaluation documentation,
MCP examples, release metadata, future profiles and registry views, explorer
data, and paper tables and figures; those are not silently represented as
generated ledger projections. The vendored `eval/paper/PAPER.md` is attributed
source material and is never rewritten as a current project claim.

The separately sealed identical-response E2 ledger owns the admitted exact-response
package and is linked from the empirical-provenance and prospective-study
sections; it is not merged into the provider-free reference claim surface.

| Propagation surface | Current claim role | Verification rule |
|---|---|---|
| `README.md` | Headline identity, bounded proof path, reader routes, current contribution, unavailable dimensions, setup, and release status | Release-scoped positioning may assert only verified capsule, ledger, autopsy, challenge, and release-manifest parents |
| `docs/findings.md` | Detailed development interpretations and withdrawn conclusions | Current machine view is ledger-derived; explanatory prose must remain within the same ceilings |
| `docs/documentation.md` | Runtime behavior plus this charter and governance SSOT | Product changes update behavior here; the claim ledger validates claim expressions |
| `docs/spec.md` | Public interfaces, schemas, defaults, architecture, and test promises | Interface changes update target state and preserve identity compatibility |
| `eval/results/README.md` | Artifact inventory, route narrative, scores, calibration, canonical negative-evidence JSON, derived Markdown, and reproduction instructions | Legacy provider results are typed capsule inputs; canonical JSON remains evidence and Markdown remains a renderer-bound human view |
| `eval/README.md` | Paper-reference and evaluation workflow claims | Shared execution and artifact-evaluation gates keep instructions aligned with shipped behavior |
| `eval/paper/PAPER.md` | Vendored original-project narrative and headline numbers | Immutable attributed source; never a generated current-project view |
| `.agents/skills/evalwitness-audit/SKILL.md` and skill metadata | Operational capability, expected route, integrations, and allowed claims | Current operational wording is ledger-validated; live capability still requires current attestation |
| `config/mcp/*` | Executable, server, tool, environment, and preset examples | Identity compatibility and behavioral conformance are tested |
| `docs/releasing.md` | Pre-publication asset plan, release procedure, reproducibility, and claim freshness | A signed release requires explicit authorization after the local clean-clone and release gates |
| `.github/workflows/*` and build scripts | Test, artifact, version, and publication behavior | Release provenance and signatures remain authorization-gated |
| Reliability profiles and registry entries | Scoped reliability and signed community projections | Must derive from exact capsule and ledger parents |
| Explorer data | Human-facing evidence and counterexamples | Offline renderer consumes public capsule projections only |
| Paper tables and figures | Research claims, statistics, limitations, and artifact instructions | Generated or checked against locked studies, capsules, and claim ledger |
| Release notes, tags, packages, and repository metadata | Public identity and release-level promises | Created only after explicit publication authorization |

## Target Architecture

One application path owns each state transition. CLI, MCP, eval, protocol
adapters, drift capture, and Best-of-N may parse entrypoint-specific arguments,
but may not build requests, extract scores, or decide independently.

| Layer | Sole owner | Persistent output |
|---|---|---|
| Ingestion | Canonical transcript ingestion service | Source fingerprint, canonical event stream, and loss report |
| Interchange | Versioned mapping adapters | Trace envelope plus OpenTelemetry GenAI or Agent Trace mapping report |
| Evidence | Evidence compiler | Evidence bundle, retention report, and digest |
| Request | Canonical request builder | Request envelope and fingerprint |
| Provider | Attested provider port with budget firewall | Response record, served identity, usage, and attestation reference |
| Score evidence | Strict token-alignment and distribution extractor | Score-evidence record, never a naked float |
| Decision | Versioned selective policy | Selection, tie, abstention, or failure with reasons |
| Verification application | One planned and lifecycle-owned run service | Canonical request set, run fingerprint, decision, budget snapshot, checked audit, and cleanup state |
| Evaluation | Locked study runner | Independent outcomes, metrics, uncertainty, and exclusions |
| Audit protocol | Protocol runner over evaluator adapters | Conformance matrix and case-level results |
| Controlled corpus | Transformation and validation service | Mutation manifest, lineage, outcome, and mutation witness |
| Outcome validity | Independent execution and blinded adjudication service | Evidence ledger, public packet, private mapping, reviewer labels, resolution, agreement, and preservation |
| Stress analysis | Relation engine and reducer | Relation results and minimized counterexamples |
| Reliance analysis | Prespecified intervention analyzer | Reliance contrasts, maps, invalid cells, and witnesses |
| Capsule | Content-addressed provenance builder | Immutable observations, derived children, checksums, and visibility |
| Claims | Claim ledger and claimcheck | Scoped claim states, expressions, caveats, and guarding tests |
| Profile | Reliability projection and policy evaluator | Versioned dimensions, applicability, and policy result |
| Adoption | Offline CI, registry, integrations, explorer, and replication kit | External-use and independent-verification evidence |

Target data flow:

`session/OTel/attribution source -> canonical events + loss report -> controlled
audit case -> evidence bundle -> request envelope -> attested route or exact
replay -> score evidence -> decision or abstention -> relation/outcome analysis ->
evidence-reliance analysis -> experiment capsule -> claim ledger -> reliability
profile -> policy/registry/explorer -> paper and public narrative`

### Capsule and claim runtime

`internal/capsule` is the domain-neutral evidence kernel. The sealed registry
closes component type, schema, role, allowed visibility, media type,
canonicalization, validator, binding validator, parent edge kind, parent type,
cardinality, and resolution. Verification rejects unknown types, schema drift,
undeclared or role-inverting edges, cycles, visibility escalation, duplicate
identity, non-canonical JSON, digest mismatch, missing bytes, extra files, and
undeclared directories before evaluating a claim.

Scientific identity is the SHA-256 root over every non-presentation component
and its typed graph. Presentation roots are separately bound and may change only
through new renderer receipts; they do not rewrite scientific identity. The
reference build contains 71 components, 69 scientific and two presentation, and
its canonical archive contains 74 files. The additional scientific commitment
is the read-only legacy-cache census: it binds inventory metadata and the exact
absence of admissible response identity without exposing response or operational
content. Source provenance records the exact
Git-visible snapshot. Build provenance records source and binary digests,
commit, dirty state, embedded VCS revision and modified state, target, Go
version, flags, dependencies, and the explicit
`digest-bound-external-binary` reproducibility limit. A clean binary cannot be
paired with a later dirty source tree.

`capsule build` constructs the public reference package without provider calls,
then stages a capsule directory, deterministic tar.gz archive, canonical claim
ledger, in-toto Statement, deterministic claim projection, and Claim Autopsy.
Every target must be absent. Regular files publish by no-overwrite hard link and
directories by native no-replace rename on Darwin, Linux, and Windows. A returned
error rolls back newly published targets. Because the six outputs may have
different parent directories, a process crash between publishes can leave a
recoverable subset; this is deliberately not described as a filesystem-wide
atomic transaction.

The public owner-inspection component contains only the closed aggregate
attestation and an explicit private omission. Public verification cannot and
does not claim reproduction of private source bytes. `capsule
build-private-relation` instead seals the package inventory, 53 package files,
66 ordered events, inspection record, completion receipt, source commitments,
and replay proof into a private 64-component capsule. `capsule
verify-private-relation` verifies both capsules, the complete private chain, and
their family binding while retaining owner-only modes.

The statement uses in-toto Statement v1 with predicate type
`https://evalwitness.dev/attestation/capsule/v1` and binds the capsule,
manifest, registry, study, cell, roots, component counts, and visibility counts.
The signing library implements DSSE pre-authentication encoding, explicit
Ed25519 trust roots and thresholds, and the Sigstore bundle v0.3 media type for
the explicit-public-key form. Certificate, transparency-log, timestamp, and
keyless verification material are rejected in that mode. The CLI currently
emits and verifies the unsigned canonical capsule Statement. The separate
release subsystem binds the full release manifest and SPDX SBOM into its own
in-toto Statement, supports explicit-key DSSE, and keeps platform keyless
attestation as a distinct post-tag proof.

`internal/claim` seals 34 stable claims to the capsule manifest. Exact
expressions support pointer equality, count, sum, ratio, difference, and
all-equal operations. Status, E0-E6 level, scope, uncertainty, multiplicity,
caveat, evidence ceiling, attestation state, generation, history, and guarding
test travel together. The current ledger contains 13 supported, two exploratory,
18 unsupported, and one superseded claim. Fifteen are current and 19 remain
historical. The closed eight-class challenge registry produces 189 applicable
receipts across the ledger. Each receipt proves the declared guard fails for the
declared reason on an ephemeral copy while the sealed-source digest remains
unchanged.

Claim Autopsy joins the v1 to v3 method-self-falsification lane to the five-layer
verification-lineage claim-transport lane. `claim verify`, `claim explain`, `claim challenge`,
`claim autopsy`, and `claim surface` all load the same verified capsule and
canonical ledger. Six tracked public surfaces are deterministically rendered or
byte-verified; hand-editing a generated block fails claimcheck.

`internal/verification` is the shipped application boundary. It validates and
preprocesses canonical input once, derives the exact request set and run
fingerprint, resolves live limits and authorization, opens one runtime, dispatches
one typed mode policy, rejects inadmissible strict-verifier selections, snapshots
the shared budget, records the run audit after resource cleanup, and closes the
audit last. `verify`, canonical and deprecated MCP tools, both benchmark runners,
Best-of-N selection, and `protocol application-adapter` are syntax adapters over
that service. Cross-entrypoint replay fixtures assert identical semantic request
sets and byte-stable decisions while retaining the entrypoint in provenance.

### Trust boundaries

| Boundary | Untrusted input | Required control |
|---|---|---|
| Filesystem | Cache, fixture, archive, output, task, and trajectory paths | Canonicalization, containment, marker ownership, no broad deletion |
| Transcript | JSONL, terminal text, tool output, paths, and secrets | Bounded parsing, redaction, loss accounting, no execution |
| Provider | Bodies, headers, identity, usage, retry hints, and token arrays | Strict schemas, bounded resources, captured identity, fail-closed semantics |
| Replay | Old schemas, edited fixtures, and incomplete identity | Versioned schema, exact fingerprints, and checksums |
| MCP | Methods, notifications, IDs, arguments, and cancellation | Protocol validation, deadlines, and no paid notification side effects |
| Evaluation | Reused tasks, missing outcomes, optional exclusions, and multiple tests | Locked manifest, task/source splits, declared analyses, immutable exclusions |
| External adapter | Operator-configured executable and protocol stream | No fixture-selected commands, bounded lifecycle, offline default, strict schema |
| Trace interchange | Evolving schemas and sensitive optional content | Pinned versions, mapping-loss report, opt-in content, bounded import |
| Controlled mutation | Multi-factor or invalid transformations | Executable/formal witness, ambiguity state, lineage isolation, blind audit |
| Outcome adjudication | Reviewer leakage, handbook drift, unqualified labels, benchmark circularity, and infrastructure failures | HMAC-rekeyed packets, separate owner-only mapping, content-addressed handbook and reviewer kits, post-label semantic-blinding probes with chance-corrected uncertainty, reviewer-bound qualification, typed evidence, and explicit unresolved states |
| Evidence intervention | Changed task identity, quality, or multiple factors | Typed allowlist, lineage, outcome-preservation validator, randomized assignment |
| HTML and report | Labels, traces, links, and embedded data | Escaping, CSP, no network, public-data allowlist, deterministic renderer |
| Community intake | Malicious, stale, duplicate, or misleading capsules | Offline verification, freshness, expiry, governance, no live CI |
| Publication | Stale prose and selectively copied metrics | Claim ledger, capsule links, evidence labels, claimcheck |

## Offline Evidence Explorer

EvalWitness ships a self-contained evidence explorer for inspecting one verified
public capsule without a provider, backend, HTTP server, or runtime package
manager. It is a claim autopsy, not a general dashboard and not an LLM judge.
After the caller supplies the loaded capsule, ledger, and Claim Autopsy,
`BuildReport` establishes one atomic claim-side re-verification boundary: it
verifies the capsule and ledger once, evaluates every claim, deterministically
rederives and compares Claim Autopsy, and generates every applicable challenge
receipt. The report builder then byte-verifies the 20-file verification-lineage
development release before it emits any view model. The unchanged source is not
reverified between those claim-side derivations.

| view | current evidence |
|---|---|
| Scope | adapter-development role, two development fixtures, zero empirical task groups, zero admitted research sources, zero provider calls, and explicit non-ranking boundary |
| Claim and BOM | one accepted fixed-fixture claim, exact executable/operands, required and decisive channels, repository-state binding, and freshness state |
| Method integrity | v1 falsification, v2 supersession, v3 development admission, guarding tests, evidence references, and frozen denominators including 939 attempts, 3 of 40 admitted scarcity cases, and zero test-role scarcity cases |
| Owner inspection | exact public attestation projection with 66 of 66 completed assessments, 16 aggregate dimensions, separate core/scarcity/overall dispositions, ten closed public claims, and an enforced firewall around private journal identities and restricted evidence |
| Claim transport | accepted five-layer witness path plus rejected same-path comparison with first loss at retained-bundle failability |
| Stress witness | repository-validated candidate-order reversal, exact A/B and B/A selections, 53 replayed reduction attempts, 30 accepted removals, 23 retained-proof rejections, two-line one-minimal witness, complete digest chain, and explicit zero-provider/zero-empirical boundary |
| Evidence reliance | capsule-derived E1 mechanism map over 24 source-task fixtures, all 1,536 registered cells, 98 term-outcome dimensions, ten selector audits, five explicit arm-contrast families, and one privacy-safe one-minimal witness; 1,392 measured and 144 abstained cells remain separate |
| Identical response | optional verified v5 outer-capsule view with 60 same-response groups, 53 agreements, seven disagreements, five sealed claims, 34 challenge receipts, seven exact failure rows, explicit 2/60 outcome coverage, evidence ceiling, and clean-clone command |
| Dataset and limitations | typed development-only dataset card, exact counts, out-of-scope uses, and current limitation availability |
| Break This Claim | deterministic maximum-coverage claim `CLM-011`, all eight registered challenge classes, seven executable receipts, one typed not-applicable class, 189 pack receipts, unchanged sealed-source digests, and exact replay commands |
| Release | all 20 manifest files loaded through bounded root-confined reads and verified by byte count and SHA-256 |
| Extensions | calibration, verifier relation, outcome, and profile panels become available only when their exact registered component types exist; reliance requires its verified child capsule and ledger together; identical-response requires its verified base/outer capsules, ledger, challenge pack, and reproduction report together; missing evidence remains typed `not_measured` or `not_run` |

### Stress witness visual philosophy

**Forensic Compression** treats reduction as a visible scientific instrument,
not decoration. A long sequence of replay decisions applies pressure across a
graphite field until the trajectory collapses from 32 lines to two. The visual
tension is clinical precision against a single undeniable failure.

Space carries the argument. Candidate order occupies one bounded chamber, the
reduction tape spans the larger chamber, and the final witnesses sit below as
the irreducible residue. The monumental `32 -> 2` transition is the signature
move; every secondary element yields to it.

Color is semantic material: cobalt records accepted removals, vermilion marks
retained proof and the violated relation, cold white carries the surviving
evidence, and graphite isolates the instrument from the rest of the Explorer.
Monospace labels behave like chain-of-custody marks rather than explanatory
copy.

The tracked presentation image is the exact 1440-by-1000 deterministic Explorer
capture, displayed at half width in the GitHub README. A fresh report-bound
capture must be byte-identical before the asset is accepted. No separate art
renderer, hand-entered number, crop, recompression, browser mockup, or marketing
claim may enter this visual path.

The companion 1400-by-900 reduction certificate is a paper-ready vector view,
not a second scientific artifact. The stress CLI derives its order panels,
53-decision tape, `32 -> 2` summary, retained outputs, minimality status, digest
prefix, and visible claim boundary directly from the same validated case-study
object. The tracked SVG contains no scripts, external references, raster payloads,
or metadata and must reproduce byte-for-byte through the stress audit.

`claim report` emits canonical
`evalwitness.evidence-explorer-report.v2` JSON. The required stress view is
derived only after the tracked `evalwitness.stress-development-case-study.v1`
sidecar passes strict schema, repository-fixture, canonical-byte, digest-chain,
minimality, and claim-boundary validation. `claim render` embeds the exact
canonical report plus its SHA-256, the pinned CSS/JavaScript asset manifest, and
renderer metadata into one deterministic HTML file. The destination must not
exist. The renderer escapes labels and links, limits each embedded asset to 2
MiB and the complete HTML to 8 MiB, emits no source map, and seals exact
style/script hashes plus narrowly scoped style-attribute support into a
deny-by-default CSP. The browser independently checks
the embedded report SHA-256 through WebCrypto before rendering. Unknown status
values remain neutral instead of inheriting a positive color.

```bash
evalwitness capsule build-reliance \
  --base-capsule eval/results/evidence-reliance-base-capsule-v1 \
  --map eval/results/evidence-reliance-map-v1.json \
  --destination ./reliance-capsule

evalwitness capsule verify-reliance \
  --base-capsule eval/results/evidence-reliance-base-capsule-v1 \
  --source ./reliance-capsule \
  --map eval/results/evidence-reliance-map-v1.json \
  --ledger eval/results/evidence-reliance-claims-v1.json \
  --profile eval/results/evidence-reliance-profile-v1.json \
  --paper eval/results/evidence-reliance-paper-v1.json \
  --explorer eval/results/evidence-reliance-explorer-v1.json

evalwitness claim report \
  --capsule eval/results/evidence-reliance-base-capsule-v1 \
  --ledger eval/results/evidence-reliance-base-claims-v1.json \
  --reliance-capsule ./reliance-capsule \
  --reliance-ledger eval/results/evidence-reliance-claims-v1.json \
  --repository-root . > evidence-explorer-report.json

evalwitness claim render \
  --capsule eval/results/evidence-reliance-base-capsule-v1 \
  --ledger eval/results/evidence-reliance-base-claims-v1.json \
  --reliance-capsule ./reliance-capsule \
  --reliance-ledger eval/results/evidence-reliance-claims-v1.json \
  --repository-root . \
  --destination ./evidence-explorer.html

open ./evidence-explorer.html

cd web/explorer
bun run capture:assets -- \
  --html ../../evidence-explorer.html \
  --destination ../../dist/evidence-explorer-assets

go run ./scripts/demos/record-terminal-demo.go \
  --destination ./dist/evidence-explorer-proof.cast -- \
  ./scripts/demos/run-evidence-explorer-demo.sh \
  --binary ./evalwitness \
  --capsule ./capsule \
  --ledger ./capsule.claims.json \
  --repository-root . \
  --destination ./dist/evidence-explorer-demo.html
```

The browser runtime is ordinary embedded JavaScript under `file://`; React,
Tailwind CSS, shadcn/ui primitives, Zod, Vite, and Bun exist only in the
isolated `web/explorer` source/build lane. The Go binary embeds the checked CSS,
JavaScript, and canonical asset manifest, so normal CLI users do not need Bun.
`scripts/tests/run-evidence-explorer.sh` rebuilds the source, checks formatting,
lint, strict types, unit tests, embedded-byte identity, the canonical report,
public artifact safety, immutable rendering, and the file-protocol browser flow
at desktop, narrow-laptop, tablet, and mobile widths with WCAG 2.2 AA checks,
reduced motion, zero external requests, and zero pre-audit console errors. It
also captures byte-stable Claim Autopsy desktop/mobile, stress-lab desktop,
owner-inspection desktop, evidence-reliance desktop, and identical-response
desktop PNGs plus a 1600-by-900 architecture SVG. Their
`evalwitness.evidence-explorer-public-assets.v2` manifest binds every file to
the exact HTML, report, renderer, Playwright Chromium version, byte count, and
SHA-256. The new-destination screenshot set and SVG are derived release outputs
written only after capsule/report construction; those generated directories do
not feed back into source-tree provenance. The tracked raster projection,
`assets/stress-witness.png`, participates in source-tree identity but its pixels
depend only on the stable stress view, never on the capsule or report digest.
The Explorer gate requires it to equal a fresh `stress-lab-desktop.png`
byte-for-byte; it remains presentation evidence and never becomes a scientific
input.

The terminal proof runs one valid closed claim, withdraws the same claim through
the registered denominator-deletion challenge, verifies the capsule and ledger
offline, renders the explorer, and scans the HTML as public material. It fails
if the exact guard does not fire, the sealed source changes, provider/network
requirements appear, public findings exist, or elapsed time exceeds 90 seconds.
`record-terminal-demo.go` captures the command's real combined stdout/stderr as
no-overwrite asciicast v2 without recording input or environment variables other
than `SHELL` and `TERM`. The `.cast` and HTML remain derived release outputs.
Direct method, transport-layer, stress-witness, challenge-receipt, and artifact
fragments restore their exact inspector state under `file://`; unregistered
fragments fail closed to `#autopsy`.

## Closest Work and Contribution Boundary

The primary-source boundary was refreshed on 2026-08-12. It establishes overlap
and exclusions only. It does not prove that an EvalWitness adapter, empirical
study, or integration exists.

| Closest primary work | Established capability | EvalWitness boundary |
|---|---|---|
| [LLM-as-a-Verifier](https://github.com/llm-as-a-verifier/llm-as-a-verifier) and [G-Eval](https://aclanthology.org/2023.emnlp-main.153/) | probability-weighted score tokens, verifier tooling, coding-agent selection, and form-filling evaluation | scoring, a general verifier SDK, coding-agent use, and best-of-N are prior art |
| [Judge Reliability Harness](https://github.com/RANDCorporation/judge-reliability-harness) and [BabelJudge](https://github.com/Shreyaskc/BabelJudge) | perturbation-based judge validation, calibration, stability, multilingual tests, and agent-trajectory degradation | EvalWitness is not the first judge-reliability or controlled-degradation suite |
| [AgentRewardBench](https://github.com/McGill-NLP/agent-reward-bench) and [AgentLens Bench](https://github.com/agent-lens/agent-lens-bench) | evaluator meta-benchmarks, formal repository checks, evidence-citing coding-trajectory review, and regression analysis | EvalWitness is not the first trajectory-judge benchmark or coding-agent review system |
| [agentevals](https://github.com/agentevals-dev/agentevals) and [OpenTelemetry GenAI semantic conventions](https://github.com/open-telemetry/semantic-conventions-genai) | evaluation over recorded OpenTelemetry traces and vendor-neutral GenAI telemetry vocabulary | the shipped EvalWitness work is a pinned loss-accounted bridge, not a new telemetry standard or the first trace evaluator |
| [Agent Trace 0.1.0](https://agent-trace.dev/) | AI-code attribution with quality assessment explicitly excluded | attribution may supply provenance; it never becomes quality evidence |
| [AgentRx](https://github.com/microsoft/AgentRx) and [Which Agent Causes Task Failures and When?](https://github.com/ag2ai/Agents_Failure_Attribution) | critical-step localization, failure taxonomy, and agent/step attribution | EvalWitness makes no first claim for failure localization or attribution |
| [Causal Agent Replay](https://github.com/jaineet17/causal-agent-replay) | downstream environment re-execution after interventions | The evidence-reliance study measures verifier-output reliance only; CLM-033 forbids agent-step causality and model-internal explanation |
| [What makes the whole?](https://openreview.net/forum?id=xd9qriSZQh) | attribute-level judge compositionality and response to targeted degradation | criterion structure is prior art; only verified verifier-evidence audits are in scope |
| [OpenJudge](https://github.com/agentscope-ai/OpenJudge) and [LangChain AgentEvals](https://github.com/langchain-ai/agentevals) | general grader construction, trajectory matching, and trajectory LLM-as-judge workflows | EvalWitness remains a narrow audit and conformance lab, not another general evaluator SDK |
| [Calibrating LLM-Based Evaluator](https://aclanthology.org/2024.lrec-main.237/) and [Log Probability Tracking of LLM APIs](https://openreview.net/forum?id=hFxivbAgVP) | evaluator calibration toward human ratings and logprob-based hosted-model change detection | held-out calibration, selective abstention, provider transfer, and drift are claims only after their own locked studies pass |
| [Robby955/logprobe](https://github.com/Robby955/logprobe) | logprob normalization, missing-mass, truncation-bias, and provider-format diagnostics under the former project name | EvalWitness avoids the identity collision and claims no ownership of those diagnostics |

The current public contribution is the intersection present in the verified
capsules: strict score-evidence contracts, versioned route and trace boundaries,
controlled executable self-falsification, five-layer verification-evidence loss
localization, a bounded exact-response counterfactual, content-addressed claim
ceilings, and executable negative controls. Claim Autopsy is the strongest
provider-free engineering artifact; the identical-response v5 study is the
strongest bounded empirical artifact. Neither is proof of verifier correctness,
human validity, provider
transfer, novelty of individual methods, or independent external replication.

Broader claims about provider transfer, longitudinal drift, community registry,
external pilots, paper headline reproduction, and signed releases remain outside
this package and require their own verified child components. The available
stress catalog, evidence-reliance maps, and development case studies are
published at their explicit mechanism-evidence ceilings.

## Evidence Architecture and Publication Gates

The private task ledger controls implementation work and is excluded from the
public tree. Public documentation describes only shipped capability, admitted
evidence, explicit limitations, and the gates that prevent unsupported claims.

| Evidence family | Current state | Claim ceiling |
|---|---|---|
| Reference capsule and Claim Autopsy | Verified provider-free mechanism evidence | E1 implementation and method evidence |
| Verification-lineage package | Verified provider-free development evidence | E1 development evidence only |
| Identical-response v5 package | Admitted Top-20 capture; clean-clone reproducible at 60 groups, 53 agreements, 7 disagreements, 0 unresolved | Bounded E2 observation for the frozen route and estimand |
| Calibration, transfer, drift, registry, and external pilots | Contracts, fixtures, or `not_run` states only | No empirical claim beyond artifact status |
| Public release | Locally verified unsigned candidate; external publication is not authorized here | No tag, download, signature, or independent-replication claim |

### Proof chain

The canonical evidence flow is source or trace -> canonical request -> score
evidence -> decision -> capsule -> claim ledger -> challenge receipts ->
Explorer and release manifest. Each link is content-addressed, independently
validated, and bounded by an explicit evidence ceiling. Claim Autopsy exposes
that chain without requiring trust in the author or a live provider.

### Live execution gate

Benchmark-scale provider work requires a current route attestation, a locked
study and estimand, a hard spend and resource budget, and explicit authorization.
Execution stops when identity is ambiguous, Top-20 score evidence is unavailable,
the manifest changes, accounting exceeds its bound, provenance is incomplete,
or any score extraction or claim validation would fall back silently.

### Publication gate

Local candidates require a clean source tree, artifact-safety admission,
reproducible source and evidence graphs, SBOM and in-toto metadata, and a
byte-identical round trip. Tagging, pushing, signing, uploading, or claiming
independent external replication requires separate external proof and
explicit authorization.

## Install and Build

Requires Go 1.27.0 or newer, which is what `go.mod` declares. An older toolchain fetches the required one automatically.

| target | command |
|---|---|
| build local binary | `go build -o evalwitness ./cmd/evalwitness` |
| install to GOBIN | `go install ./cmd/evalwitness` |
| run tests | `scripts/tests/run-tests.sh` (tracked/unignored Go-source formatting, exact project-package inventory, identity residue, independent fingerprint vectors, audit suites including the coding-agent-only formal study, vet, Staticcheck 2026.2.1, non-stress Go tests, offline evidence-explorer source/report/render/browser gate, hard-network exact-response reproduction, claimcheck, build; set `EVALWITNESS_ENABLE_STRESS_TESTS=1` or `EVALWITNESS_ENABLE_RACE_TESTS=1` only for an explicit opt-in run) |
| rebuild and verify explorer | `cd web/explorer && bun install --frozen-lockfile`, then `scripts/tests/run-evidence-explorer.sh`; the Playwright Chromium runtime must be installed for the four-profile browser gate |
| capture explorer presentation assets | `cd web/explorer && bun run capture:assets -- --html PATH --destination NEW_DIRECTORY`; the destination must not exist |
| record the bounded terminal proof | `go run ./scripts/demos/record-terminal-demo.go --destination NEW_CAST -- scripts/demos/run-evidence-explorer-demo.sh --binary PATH --capsule PATH --ledger PATH --repository-root PATH --destination NEW_HTML` |
| reproduce current public evidence | `scripts/evals/reproduce-public-evidence.sh --profile full` (producer build plus execution with isolated empty EvalWitness home/configuration/response cache under operating-system network denial and zero provider calls; reuses provisioned host Go module/build caches and is not clean-clone proof) |
| reproduce the admitted identical-response v5 graph from a clean clone | `scripts/evals/reproduce-identical-response-v5.sh` (fresh clone, declared fixture and file proxy, empty Go caches, hard network denial, 14 registered-artifact byte comparisons, outer capsule/ledger/challenge verification, and canonical report) |
| reproducible static binary | `CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w -buildid= -funcalign=8" -o evalwitness ./cmd/evalwitness`; release identity is signed in the manifest rather than embedded as path-sensitive VCS metadata |
| build complete local candidate | `scripts/build/build-release-candidate.sh --destination NEW_DIRECTORY [--key-file MODE_0600_FILE]`; requires a clean worktree and a destination outside the repository; only the version-bound tag workflow may add `--external-publication authorized_by_tag` |

## Configuration

Resolution order (later wins): defaults, `EVALWITNESS_PRESET`, the first discovered config file, environment variables, CLI flags. `.env` is loaded before resolution and only fills process variables that are not already set.

Config discovery checks `$EVALWITNESS_CONFIG_FILE`, `~/.config/evalwitness/config.toml`, `~/.config/evalwitness/config.json`, `./evalwitness.toml`, and `./evalwitness.json`; the first existing file wins. Legacy `LOGPROBE_CONFIG_FILE`, `~/.config/logprobe/config.{toml,json}`, and project `logprobe.{toml,json}` are fallback-only. Auto-discovery searches for `.env` in `$EVALWITNESS_ENV_FILE`, `./.env`, `<binary-dir>/.env`, and `~/.config/evalwitness/.env`, then the legacy home path. First found wins.

Every canonical runtime setting has a same-suffix `LOGPROBE_*` transition alias. The canonical name wins when both are set. Consumed aliases produce one sorted, redacted process-level warning; values are never logged. A legacy `LOGPROBE_CACHE_DIR` is treated as read-only import evidence, never as a write destination. Set `EVALWITNESS_LEGACY_CACHE_DIR` to request the same read-only import explicitly. Canonical cache entries always win exact-key collisions; cache stats inspect only schema-known response namespaces and clear requires an explicit `responses`, `capabilities`, or `all` scope. Results count `legacy_cache_hit_calls`, and audit rows label `cache_namespace`, so imported evidence cannot masquerade as canonical state.

Required env for any OpenAI-compatible route:
| env var | example |
|---|---|
| EVALWITNESS_PROVIDER | your-provider |
| EVALWITNESS_WIRE_FORMAT | openai |
| EVALWITNESS_MODEL | your-model |
| EVALWITNESS_BASE_URL | https://your-provider.example/v1 |
| EVALWITNESS_API_KEY | sk-... |
| EVALWITNESS_THINKING_MODE | disabled |

Operational env vars (selected):
Use a preset only when its exact route identity is intentional. A preset or
configuration never qualifies a route for empirical use; `evalwitness attest`
must establish the current request contract first.
| env var | default | purpose |
|---|---|---|
| EVALWITNESS_PRESET | compiled compatibility preset | optional preset bundle; any OpenAI-compatible endpoint is reachable with EVALWITNESS_BASE_URL and EVALWITNESS_MODEL |
| EVALWITNESS_WIRE_FORMAT | openai | HTTP API shape: openai only |
| EVALWITNESS_CACHE_DIR | `~/.cache/evalwitness` | canonical read/write cache root |
| EVALWITNESS_LEGACY_CACHE_DIR | (unset) | optional read-only import root for pre-EvalWitness entries |
| EVALWITNESS_CA_FILE | (unset) | PEM CA bundle override for TLS verification when the macOS trust service is unavailable or a private CA is required |
| EVALWITNESS_DEFAULT_REPS | 1 | base reps per pair/criterion |
| EVALWITNESS_BIAS_MITIGATION | adaptive | task-hash-balanced first order; reverse order only when uncalibrated decision strength is insufficient; both / single / disabled remain explicit modes |
| EVALWITNESS_CRITIQUE_THEN_SCORE | true | brief critique before score |
| EVALWITNESS_MULTI_CRITERION_BUNDLE | true | one prompt covers all criteria |
| EVALWITNESS_MAX_TOKENS | 0 (auto = 4096) | output budget per call; reference-parity 4096, semantic output freezes after all score tags and transport performs a bounded terminal-usage drain |
| EVALWITNESS_MAX_RETRIES | 5 | retry budget for transient provider failures and rate limits |
| EVALWITNESS_MIN_DISPATCH_INTERVAL_SEC | 0 | locked delay after each successful batch cell before the next cell starts; positive values require `EVALWITNESS_MAX_WORKERS=1`; pair with persistent run-budget state on TPM-limited routes |
| EVALWITNESS_TEMPERATURE | 1.0 | score-call temperature |
| EVALWITNESS_SEED | (unset) | seed passthrough for providers that support it |
| EVALWITNESS_INCONSISTENCY_POLICY | flag-only | fixed-rep compatibility policy; adaptive K=1 handles uncertainty directly from the distribution |
| EVALWITNESS_REDACT_PATTERNS | (unset) | JSON file with additional redaction rules |
| EVALWITNESS_CONTEXT_LIMIT | 1000000 | provider context window; bounds the evidence budget when smaller |
| EVALWITNESS_EVIDENCE_TOKENS | 32000 | maximum evidence tokens retained per trajectory; 16000/32000/64000 are calibration profiles |
| EVALWITNESS_MAX_PAIR_CALLS | 2 | hard adaptive ceiling per pair; bundled criteria keep one order to one call. The accepted range is 1 to 4; the development sweep found no support for making 3 the default |
| EVALWITNESS_PAIR_CONFIDENCE | 0.6 | legacy raw win-probability stop threshold for decision state only. Extra adaptive calls no longer read this value unless `confidence_escalation=legacy`; held-out recalibration is not yet tested |
| EVALWITNESS_PAIR_CALIBRATION_SIGMA | 0.05 | conservative residual uncertainty added to Top-20 distribution variance |
| EVALWITNESS_EXPECTED_ESCALATION_RATE | 0.25 | preflight expected-case estimate calibrated from both bounded benchmark runs (56 of 170 and 45 of 258 pairs escalated); never changes the hard limit |
| EVALWITNESS_SELECTION | (leer) | empty follows EVALWITNESS_SINGLE_ELIM; `absolute` scores each trajectory separately; `joint_absolute` scores the complete ordered candidate set in one immutable response per task group |
| EVALWITNESS_SINGLE_ELIM | true | dynamic actual-winner bracket, N-1 matches; false restores exhaustive all-pairs for a complete ranking |
| EVALWITNESS_SPRT | false | legacy fixed-rep Wald adaptation; adaptive K=1 is the production default |
| EVALWITNESS_BILLING_MODEL | pay-as-you-go | subscription = est_cost = 0 |
| EVALWITNESS_THINKING_MODE | disabled | default / enabled / disabled; direct DeepSeek presets use disabled so logprobs work on final answer tokens |
| EVALWITNESS_MAX_COST_USD_PER_CALL | (unset) | refuse calls above cap |
| EVALWITNESS_OFFLINE | false | refuse network calls |
| EVALWITNESS_NO_REDACT | false | disable trajectory redaction |
| EVALWITNESS_NO_CACHE | false | disable disk cache |
| EVALWITNESS_AUDIT_LOG | (unset) | jsonl audit path |
| EVALWITNESS_REPLAY_FROM | (unset) | serve from fixture |
| EVALWITNESS_REPLAY_TO | (unset) | capture to fixture |
| EVALWITNESS_JUDGE_MODE | false | skip logprob requests, raw-text extraction |
| EVALWITNESS_ALLOW_JUDGE_MODE | false | doctor treats logprob-less routes as usable judge-mode |

## Subcommands

| command | purpose |
|---|---|
| `evalwitness version` | print version |
| `evalwitness doctor [--live] [--output json\|text]` | config/key/capability readiness report; `--live` runs the provider probe |
| `evalwitness presets [--output json\|text]` | list built-in configuration bundles, default marker, configured state, and known limitations |
| `evalwitness probe [--provider X] [--wire-format openai] [--model Y]` | weak runtime capability diagnostics; emits only `probe_compatible` |
| `evalwitness attest [--mode pairwise\|absolute\|joint_absolute] [--criteria ids] [--task T --trajectory @FILE...]` | bounded real score-task qualification; optional task plus repeated trajectories use the production preprocessing and request-planning path before writing an exact, expiring attestation |
| `evalwitness verify --mode <delta\|absolute\|pairwise>` | one-shot dispatch; live execution requires exact authorization and matching attestation |
| `evalwitness mcp-serve` | start MCP server on stdio |
| `evalwitness cache stats` | print cache size and dir |
| `evalwitness cache clear --scope responses\|capabilities\|all` | atomically remove only the named schema-owned cache namespace; retain root, marker, route identities, legacy data, and unknown files |
| `evalwitness replay migrate --source old.jsonl [--candidate inspection.jsonl]` | preserve a legacy prompt-hash fixture and emit a non-exact inspection candidate plus digest and ambiguity report |
| `evalwitness replay census-legacy-cache --source PATH --published-provider ID` | inspect one legacy cache read-only and emit only public-safe structural counts, inventory digests, exact-identity gaps, and the named provider namespace upper bound |
| `evalwitness replay bundle seal-policy --source FILE --repository-root PATH --producer-binary PATH --redistribution-evidence FILE --capture name=FILE` | seal exact capture-corpus, source-tree, producing-binary/toolchain, license, omission, and redistribution-evidence identity into a canonical policy |
| `evalwitness replay bundle build --policy FILE --repository-root PATH --producer-binary PATH --redistribution-evidence FILE --destination PATH --capture name=FILE [--archive PATH] [--reviewed-findings FILE]` | build and no-overwrite publish a public exact-response capsule from canonical schema-3 captures only; `--reviewed-findings` suppresses known false-positive safety findings by rule+fileSHA+line matching |
| `evalwitness replay bundle verify --source PATH [--reviewed-findings FILE]` | verify the closed response-bundle graph, embedded evidence, source/build provenance, public scan, exact payload paths, and zero-provider offline boundary; the optional exact-content review list suppresses only matching rule+fileSHA+line findings |
| `evalwitness replay capture-run attest --capture PATH --authorized-calls N [--output PATH]` | seal `evalwitness.capture-run-attestation.v1` from a schema-3 JSONL capture |
| `evalwitness replay capture-run verify --capture PATH --attestation PATH` | re-inspect the capture and reject attestation digest or census drift |
| `evalwitness replay capture-run stamp --capture PATH --destination PATH --stamp FILE [--output PATH]` | write a new capture with research lineage overlay; refuses in-place rewrite |
| `evalwitness replay capture-run admit --capture PATH --authorized-calls N [--output PATH]` | bind payload SHA-256 to an admission certificate; complete lineage required for `admitted` |
| `evalwitness replay study bind --capture PATH --authorized-calls N --attestation PATH --admission PATH --claim-ledger PATH [--bundle-policy PATH --study-record PATH --offline-analysis PATH --output PATH]` | digest-bind parents into `evalwitness.identical-response-050-bind.v1`; `bind_status=complete` only with complete research lineage and all named parents; not an outer study capsule |
| `evalwitness replay study capsule build --base-capsule PATH --study-manifest FILE --study-record FILE --live-authorization FILE --route-attestation FILE --capture-run-attestation FILE --admission FILE --offline-analysis FILE --destination PATH [--archive FILE --claim-ledger FILE --challenge-pack FILE]` | build and no-overwrite publish the v5 outer capsule, sealed claim ledger, deterministic archive, and executable challenge pack offline |
| `evalwitness replay study capsule verify --base-capsule PATH --source PATH --claim-ledger FILE --challenge-pack FILE` | independently reload and verify the outer capsule family, sealed ledger, challenge-pack identity, and all challenge receipts with zero provider calls |
| `evalwitness replay study portfolio --bind PATH --claim-ledger PATH [--output PATH]` | digest-bound one-screen claim chain; `explorer_present=false`; eval-terminal not locked estimand |
| `evalwitness calibration evaluate --observations FILE --threshold F --target-risk F --min-coverage F [--seed N] [--artifact FILE --route SCOPE --domain SCOPE] [--inventory FILE --root DIR]` | run held-out `IsDeployable`; optional artifact forces unsupported on route/domain mismatch; `--inventory` rejects development-inventory confirmatory leakage |
| `evalwitness calibration seal --artifact FILE --calibrator FILE` | checksum a model-artifact draft; reject leakage feature keys |
| `evalwitness calibration verify --artifact FILE --calibrator FILE` | reject calibrator checksum drift |
| `evalwitness calibration apply --artifact FILE --route SCOPE --domain SCOPE [--calibrator FILE]` | refuse route/domain mismatch |
| `evalwitness calibration bind-049 --split FILE --study FILE` | bind committed 049 split/study bytes; frozen identical-response split has no test role |
| `evalwitness calibration bind-034 --inventory FILE [--root DIR]` | verify the committed development inventory and freeze 589 inventoried tasks as development-only |
| `evalwitness registry validate-intake --entry FILE [--catalog FILE]` | offline schema/content check of one registry intake entry; optional catalog rejects duplicates |
| `evalwitness registry preflight --entry FILE [--catalog FILE]` | intake pre-submit: expiry, replay, and catalog duplicates |
| `evalwitness registry template` | print a non-submittable intake skeleton |
| `evalwitness registry refresh --catalog FILE` | offline expiry/replay refresh of a catalog; no live call |
| `evalwitness registry review-checklist` | local maintainer pre-submit checklist; not community admission |
| `evalwitness registry render-matrix --catalog FILE [--history FILE]` | group intake entries by compatible contract; history is append-only; no ranking |
| `evalwitness registry render-reliance --catalog FILE` | group optional evidence-reliance parents only on identical ontology/panel/estimator/intervention/outcome/profile digests; no ranking |
| `evalwitness registry index-scarcity --evidence FILE` | index committed scarcity JSON as a non-rankable 198/3/195 availability record |
| `evalwitness registry index-owner-inspection --attestation FILE` | index the public owner-inspection attestation as a non-rankable readiness record; owner-pass never upgrades admission |
| `evalwitness registry render-method-lineage --autopsy FILE` | render the reference-capsule two-break method ladder; no ranking, pooling, or history overwrite |
| `evalwitness registry index-empirical --attestation FILE` | Outcome validity stays `not_run`; relation validity is the public owner-inspection aggregate only |
| `evalwitness registry inventory` | configured preset inventory; no live call, no key values, no community validation |
| `evalwitness registry public-derivative --entry FILE` | minimal public intake projection with evidence_ceiling=mechanism_conformance |
| `evalwitness capsule build --repository-root . --destination PATH` | build and no-overwrite publish the provider-free public reference capsule, deterministic archive, claim ledger, in-toto Statement, claim projection, and Claim Autopsy |
| `evalwitness capsule verify --source PATH [--ledger FILE --statement FILE --projection FILE --autopsy FILE]` | verify the closed public registry, graph, inventory, visibility, payloads, and any supplied deterministic sidecars offline |
| `evalwitness capsule build-reliance --base-capsule PATH --map FILE --destination PATH [--archive FILE --ledger FILE]` | derive and no-overwrite publish the public evidence-reliance child capsule, deterministic archive, and 11-claim ledger from the frozen reference base and canonical reliance map |
| `evalwitness capsule verify-reliance --base-capsule PATH --source PATH --map FILE --ledger FILE --profile FILE --paper FILE --explorer FILE` | verify the frozen base, child family, canonical map, 11-claim ledger, and all three deterministic public projections offline |
| `evalwitness capsule build-private-relation --package-root PATH --private-root PATH --session DIGEST --destination PATH` | revalidate and seal the completed private owner-inspection chain into an owner-only child capsule and deterministic archive |
| `evalwitness capsule verify-private-relation --public-source PATH --source PATH` | verify public and private capsules plus exact family binding without exposing private material |
| `evalwitness claim verify --capsule PATH --ledger FILE [--claim CLM-NNN]` | evaluate the complete ledger or one claim against exact capsule fields and ceilings |
| `evalwitness claim explain --capsule PATH --ledger FILE --claim CLM-NNN` | emit one claim contract together with its current verification result |
| `evalwitness claim challenge --capsule PATH --ledger FILE (--all [--claim CLM-NNN] \| --claim CLM-NNN --challenge ID)` | execute only registry-declared ephemeral falsifiers and emit exact guard receipts |
| `evalwitness claim autopsy --capsule PATH --ledger FILE` | deterministically project method-integrity and claim-transport lifecycles from their typed capsule parents |
| `evalwitness claim report --capsule PATH --ledger FILE --repository-root PATH [--reliance-capsule PATH --reliance-ledger FILE] [--identical-response-base-capsule PATH --identical-response-capsule PATH --identical-response-ledger FILE --identical-response-challenge-pack FILE --identical-response-reproduction-report FILE]` | emit the canonical verified evidence-explorer report; optional reliance inputs are all-or-none, and optional identical-response inputs are all-or-none and independently verify the base/outer capsule family, ledger, challenge pack, and clean-clone report |
| `evalwitness claim render --capsule PATH --ledger FILE --repository-root PATH --destination PATH [--reliance-capsule PATH --reliance-ledger FILE] [--identical-response-base-capsule PATH --identical-response-capsule PATH --identical-response-ledger FILE --identical-response-challenge-pack FILE --identical-response-reproduction-report FILE]` | no-overwrite render one deterministic self-contained offline HTML explorer from the same verified optional evidence families and emit its report, renderer, payload, and file digests |
| `evalwitness claim surface render\|verify` | render one closed claim surface or byte-verify all six tracked public surfaces against the ledger |
| `evalwitness profile build --identity ID --route R --dim ID:STATUS:METRIC:SCOPE:LEVEL:EXPR:DENOM:UNIT [--protocol P]` | build a versioned multidimensional reliability profile offline; capsule expressions may contain colons; emits stable JSON with deterministic digest and no global score |
| `evalwitness profile verify --in FILE` | recompute and compare the profile digest offline; non-zero exit on any mutation |
| `evalwitness profile diff --a FILE --b FILE [--format json\|text]` | compatibility-aware diff refusing protocol/route changes; metric, scope, evidence, denominator, and capsule-expression changes surface separately |
| `evalwitness profile policy --policy FILE --in FILE` | evaluate a content-addressed named policy; renders every failed and unknown requirement alongside passes |
| `evalwitness profile render --in FILE --format text\|markdown\|report` | render concise text, structured markdown table, or the full evidence report (text, markdown, claimcheck expressions, OTel event) directly from profile fields |
| `evalwitness audit --policy FILE --profile FILE [--format json\|junit\|markdown]` | offline CI audit: strict policy+profile decode (DisallowUnknownFields), recomputed policy digest with tamper-evident declared-digest check, printed execution plan, stable exit contract 0 pass / 1 policy failed / 2 invalid input / 3 internal error |
| `.github/actions/evalwitness-audit` | composite action: downloads the pinned release tarball, enforces the SHA256SUMS checksum, extracts the GOOS/GOARCH-named binary, runs the offline audit once per requested format (json junit markdown) and surfaces the canonical JSON as `result`; the root and sample workflows read exact `EVALWITNESS_RELEASE_VERSION` and `EVALWITNESS_RELEASE_CHECKSUM` repository variables and skip until both exist; no Node package, container, service, credentials, or network |
| `eval/policies/*.json` | example offline policies: pure offline replay gate, verifier-adapter conformance, profile regression — provider-key agnostic with no live upgrade path |
| `evalwitness release source-archive --repository-root PATH --commit SHA --destination NEW` | build a strict PAX-free deterministic USTAR+gzip archive from the exact clean current Git tree and emit its canonical verified report |
| `evalwitness release source-index --repository-root PATH --destination NEW` | emit canonical clean source-tree provenance for byte-exact archive-only provenance reconstruction without Git history |
| `evalwitness release manifest --assets PATH --commit SHA --source-archive-sha256 SHA --created RFC3339 [--external-publication not_authorized\|authorized_by_tag] --destination NEW` | inventory all seven canonical release roles and the exact five-platform binary set into one canonical no-overwrite manifest; the candidate builder admits tag authorization only when `v<version>` resolves to the exact commit |
| `evalwitness release sbom --assets PATH --manifest FILE --destination NEW` | derive a conservative SPDX 2.3 SBOM from exact manifest bytes and embedded Go build information in every release binary |
| `evalwitness release statement --manifest FILE --sbom FILE --destination NEW` | bind the byte-exact manifest and SBOM into an in-toto Statement v1 |
| `evalwitness release sign --manifest FILE --sbom FILE --statement FILE --key-file MODE_0600_FILE --destination NEW_DIRECTORY` | verify the inputs, sign exact Statement bytes with Ed25519 DSSE, and publish the envelope, explicit trust root, and policy as one no-overwrite directory |
| `evalwitness release verify --assets PATH --manifest FILE --sbom FILE --statement FILE [--signature DIRECTORY\|--allow-unsigned-development]` | recompute assets and SBOM, rebind the Statement, require complete signature material by default, and emit one canonical verification report |
| `evalwitness eval-terminal` | Terminal-Bench trajectory evaluator with Pass@1/oracle/verifier metrics |
| `evalwitness eval-swebench` | SWE-bench Verified evaluator with Pass@1/oracle/verifier metrics |
| `evalwitness bon -n N --task T -- <cmd>` | best-of-N: run an agent command N times in isolated worktrees, verifier picks the winner |
| `evalwitness fidelity --source PATH\|- [--source PATH...]` | offline transcript-conservation, token-estimate, and 16k/32k/64k evidence-retention audit; never loads a provider |
| `evalwitness protocol run [--adapter PATH --adapter-arg ARG...]` | run the offline normative verifier-audit corpus in-process or through an operator-selected NDJSON adapter |
| `evalwitness protocol cases` | emit the expanded positive, negative, compatibility, and frozen-request corpus |
| `evalwitness protocol schema --name FILE` | emit one embedded JSON Schema 2020-12 artifact |
| `evalwitness protocol reference-adapter` | serve the independent closed-form adapter on protocol-clean stdin/stdout |
| `evalwitness protocol application-adapter` | serve the production verification application service through the bounded protocol transport; offline or exact replay only |
| `evalwitness trace inspect --source PATH\|- [--privacy-class CLASS] [--output json\|text]` | detect and validate a supported trace, then print immutable identities, hierarchy, retention, and field-level mapping loss without provider access |
| `evalwitness trace export --source PATH\|- --target canonical\|otlp\|agent-trace [--output json\|artifact]` | export canonical evidence, pinned OTLP/JSON spans, or Agent Trace attribution under an explicit privacy class |
| `evalwitness trace lineage plan` | emit the sealed provider-free verification-lineage research plan byte-for-byte |
| `evalwitness trace lineage schema-inventory` | emit the content-addressed ten-object parent DAG byte-for-byte |
| `evalwitness trace lineage source-inventory` | emit the sealed pre-acquisition admission inventory without inspecting task-group counts or materializing external sources |
| `evalwitness trace lineage source-specifications` | emit the provider-free six-format registry with pinned commits, licenses, admission boundaries, and expected capability states |
| `evalwitness trace lineage fixture-witnesses` | execute seven fixed local child-process controls and emit deterministic source/witness pairs; accepts no executable or shell input |
| `evalwitness trace lineage golden-vectors` | reproduce 63 typed synthetic Claude Code, Codex, and OpenCode adapter-development vectors over 21 semantic cases; no raw secret, provider, agent, or research-denominator use |
| `evalwitness trace lineage adapter-conformance` | execute 504 normative checks over the sealed vectors and report conformant, expected-rejection, not-applicable, and known-gap states without hiding failures |
| `evalwitness trace lineage parser-lock --repository-root .` | reproduce the pre-calibration lock over parser/mapping source bytes, governance artifact bytes, 63 vectors, 504 conformance checks, and the no-provider boundary |
| `evalwitness trace lineage parser-lock-verify --repository-root . --document @LOCK.json` | strictly decode the lock, reproduce every bound source and artifact, rerun development vectors and conformance, and reject any resealed drift |
| `evalwitness trace lineage schema --type TYPE` | emit one closed Draft 2020-12 lineage schema; supported types are `assessment`, `audit`, `bom`, `candidate`, `capability-vector`, `dataset-card`, `execution-witness`, `plan`, `release`, and `source` |
| `evalwitness trace lineage validate --type TYPE --document @artifact.json` | strictly validate one lineage artifact, reject unknown or trailing input, enforce content identity and typed parents, and emit its validated identity |
| `evalwitness study validate\|lock\|transition\|report\|split\|schema\|inventory\|verify-execution` | strict preregistration, immutable lifecycle, split, historical-data, report, and execution-binding operations |
| `evalwitness mutation validate --manifest @manifest.json` | strictly validate and content-check one mutation manifest |
| `evalwitness mutation schema --type manifest\|witness\|blind-review-packet\|construct-firewall\|construct-firewall-v2\|construct-firewall-challenge\|verification-evidence-assessment\|verification-evidence-challenge\|corpus-spec\|corpus-development-plan\|corpus-development-audit\|corpus-development-audit-v3\|corpus-release\|corpus-release-v3\|reduction-witness\|formal-control` | emit a closed JSON Schema 2020-12 artifact for a controlled-corruption contract |
| `evalwitness mutation verification-evidence build-challenge` | deterministically emit the frozen provider-free provenance, failability, and claim-specific evidence-loss challenge |
| `evalwitness mutation verification-evidence validate-challenge --challenge @challenge.json` | strictly decode, validate, and independently reproduce every challenge case, assessment, reason, summary, and digest |
| `evalwitness mutation control validate --original @original.json --mutated @mutated.json` | reproduce the provider-free formal pass-to-fail positive control |
| `evalwitness mutation corpus spec [--spec @spec.json]` | emit the executable default corpus specification or strictly validate a governed specification |
| `evalwitness mutation corpus plan-v2 [--plan @plan.json]` | emit the frozen v2 development plan or strictly validate its pre-audit identity |
| `evalwitness mutation corpus audit-v2 --root . --plan @plan.json --audited-at YYYY-MM-DD` | exhaust the deterministic v2 construct universe, retaining every applied/rejected firewall and family/source-format denominator without filling quotas by predicate relaxation |
| `evalwitness mutation corpus freeze-v2 --plan @plan.json --audit @audit.json` | emit a governed v2 corpus spec only when the complete bound audit reproduces with zero family shortfalls |
| `evalwitness mutation corpus build --root . --spec @spec.json` | regenerate the reference-only corpus descriptor from fetched upstream trajectories without a provider |
| `evalwitness mutation corpus validate --release @release.json` | verify content identity, witnesses, counts, balance, controls, and split-lineage isolation |
| `evalwitness mutation corpus verify-v2-audit --plan @plan.json --audit @audit.json --release @release.json` | prove exact source, selected-case, applied-firewall, and complete rejected-firewall identity between a v2 release and its development audit |
| `evalwitness mutation corpus plan-v3 [--plan @plan.json]` | emit or strictly validate the frozen typed-proof natural-corpus audit plan without reusing the v2 schema identity |
| `evalwitness mutation corpus audit-v3 --root . --plan @plan.json --audited-at YYYY-MM-DD` | exhaust the deterministic v3 construct universe and retain every typed applied/rejected firewall, coverage cell, selected case, and unfilled quota without weakening eligibility |
| `evalwitness mutation corpus validate-v3-audit --plan @plan.json --audit @audit.json` | strictly decode and independently reproduce every v3 audit set digest, firewall, attempt, coverage row, shortfall, aggregate, finding, and canonical digest |
| `evalwitness mutation corpus build-v3 --root . --plan @plan.json --audit @audit.json` | materialize every audit-selected v3 case, bind it to the exact typed firewall and attempt, and emit the separate 280-core-plus-three-sentinel release without weakening v1/v2 invariants |
| `evalwitness mutation corpus validate-v3-release --plan @plan.json --audit @audit.json --release @release.json` | strictly reproduce v3 release sources, selected cases, typed firewalls, rejections, counts, core/sentinel policy, claim exclusions, and digest |
| `evalwitness stress catalog` | emit the provider-free 15-relation v3 stress catalog over the checked-in governance artifacts; no fetched trajectory cache is required |
| `evalwitness stress arm-plan\|analysis-design\|held-out-lock --repository-root .` | replay the exact fetched v3 corpus and emit the 5,630-cell arm plan, locked analysis design, or 57-case/1,140-cell held-out partition; the ignored Terminal-Bench and SWE-bench source caches are required |
| `evalwitness stress held-out-campaign --repository-root . --format json\|markdown` | freeze the exact ten-arm held-out topology, per-arm test/support/unsupported cell-set commitments, provider-side and zero-cost repetition arithmetic, and explicitly absent live bindings without deriving calls or issuing a permit |
| `evalwitness stress held-out-readiness --repository-root . --format json\|markdown` | reproduce the provider-free current-state refusal over the exact held-out lock, independently expected owner-package digest, and owner projection; emit no permit, empirical unit, provider call, or network requirement |
| `evalwitness stress development-case-study --repository-root . --format json\|markdown\|svg` | reproduce the candidate-order negative control over tracked task and trajectory fixtures and its deterministic 32-line-to-two-line one-minimal witness with zero provider calls; SVG is a human projection of the validated object |
| `evalwitness stress development-challenge --repository-root .` | package the exact licensed development fixture bytes and expected candidate-order reduction commitments into one self-contained content-addressed challenge |
| `evalwitness stress verify-development-challenge --challenge @artifact.json` | strictly decode the challenge, rebuild the complete case and shared one-minimal reduction without repository access, execute seven fixed controls and adversarial guards, and emit a deterministic zero-empirical receipt |
| `evalwitness stress verify-development-challenge-receipt --challenge @artifact.json --receipt @receipt.json` | verify the persisted receipt's exact challenge, case, fixture, reduction, guard, provider, network, and claim-boundary bindings without rerunning the reducer |
| `evalwitness stress validate --type development-case-study\|held-out-campaign-plan\|held-out-run-readiness-refusal --repository-root . --document @artifact.json` | strictly decode and repository-reproduce the selected public stress artifact; campaign and readiness validation rebuild the exact partition and all bound parents |
| `evalwitness stress schema --type TYPE` | emit one of 35 closed Draft 2020-12 stress schemas, including development challenge/receipt, admission, execution, preflight, authority-bound permit, atomic reservation, live-batch evidence, independent live-to-exact replay verification, execution-ledger, and admission-filtered run-seal contracts; schema emission is provider-free and does not create empirical artifacts |
| `evalwitness outcome plan\|sample\|pilot-sample\|pilot-sample-v1\|natural-request\|natural-inventory` | emit or validate the frozen adjudication plan, mutation sample, governed six-case natural outcome pilot v2, historical non-launchable mixed-v1 diagnostic, and development-only natural-case inventory |
| `evalwitness outcome pilot-materials` | reproduce all six v2 selections from exact raw candidate order and rewards, apply strict redaction plus the frozen 16k evidence budget, emit restricted review items, and publish one exclusive sealed owner-only private-materials custody root |
| `evalwitness outcome pilot-inspect` | reproduce a sealed per-packet structural reviewability report from the restricted bundle and sealed owner-only private-materials artifact; retain exact denominators while declaring semantic validity pending |
| `scripts/build/prepare-outcome-pilot.sh` | prepare one new immutable owner-only pilot package containing restricted items, sealed private materials, bundle, inspection, blinding protocol, and v3 readiness; require an existing private key and explicit ordered timestamps, validate every component, and authorize no external action |
| `evalwitness outcome packet\|evidence\|record\|label\|qualification\|qualify` | create HMAC-blinded public packets and owner-only mappings, seal strict evidence/record/label drafts, and score the governed reviewer qualification set |
| `evalwitness outcome review handbook\|bundle\|pilot-readiness\|reviewer\|assign-primary\|kit\|verify-kit\|render-kit\|label-batch\|analyze-rubric\|blinding-protocol\|blinding-probe-batch\|assign-tie\|reveal\|analyze-blinding\|adjudicate\|analyze-sources` | freeze the handbook and hidden-condition universe, prove exact pilot packet readiness without authorizing external action, execute the fail-closed review sequence, verify reviewer kits, measure rubric ambiguity and semantic blinding, seal the terminal ledger, then reconcile pre-human evidence sources without verifier-performance inputs |
| `evalwitness outcome agreement\|resolve\|preservation\|validate\|schema` | compute agreement, resolve only reviewer-qualified labels, test outcome preservation, and validate or emit closed schemas |
| `evalwitness relation plan\|plan-v2\|pilot-sample\|primary-sample\|study-amendment` | preserve the historical v1 governance chain or derive the prospective v2 plan from an exact corrected release, then reproduce the version-matched non-overlapping development pilot, primary commitment, and inference amendment |
| `evalwitness relation plan-v3\|primary-sample-v3\|scarcity-sentinel-v3\|pilot-sample-v3\|study-amendment-v3` | derive the fresh v3 pre-label chain from the typed release: balanced 28-case core, exhaustive non-held-out three-case sentinel, disjoint seven-case development pilot, and recalculated fixed-sample amendment |
| `evalwitness relation materialize` | emit a restricted, license-bound, pair-aligned 16k-per-source excerpt with task context, exact omission accounting, and one shared immutable event-lineage selection across size-changing transformations |
| `evalwitness relation packet` | HMAC-blind one validated pair or pair-of-pairs, emit only the reviewer-visible objective-typed packet, and exclusively publish its content-addressed owner-only mapping |
| `evalwitness relation qualification\|qualify\|handbook` | owner-key-randomize eight supervised competency cases, exclusively custody the exact answer key, score all responses with mandatory ambiguity/order cases, and freeze the relation-only handbook |
| `evalwitness relation reviewer\|bundle\|assign-primary\|kit\|verify-kit\|render-kit` | seal consent/conflict records, privately order a restricted packet bundle, plan reviewer-specific assignments, and build/verify injection-safe self-contained JSON and Markdown kits without authorizing distribution |
| `evalwitness relation pilot-readiness` | bind every governed development-pilot packet and owner-only mapping to the exact version-matched sample, qualification, handbook, workload, structural checks, semantic-inspection requirement, and `not_authorized` external-action boundary; v3 requires the exact primary and separate scarcity-sentinel parents and binds seven core packets without promoting the sentinel |
| `evalwitness relation pilot-change-receipt` | reproduce readiness against all restricted packets and private mappings, then emit the closed content-addressed machine receipt for parent/content/changed-line/lineage/denominator/reversal/hazard evidence with no raw task or trajectory content, semantic decision, human result, or authorization |
| `evalwitness relation pilot-launch-dossier` | bind readiness to the exact three-slot governed workload, one structural disclosure per pilot case, six unresolved owner-governance decisions, and seven separately unauthorized external actions; v1/v2 retain 64 maximum actions across eight packets while v3 seals 59 across seven core packets; emit no raw restricted evidence or launch authority |
| `evalwitness relation render-pilot-launch-brief` | validate a launch dossier and deterministically render a public-safe Markdown owner decision surface with exact versioned workload, every governed core-family example, the separate scarcity boundary, privacy/license constraints, non-binding governance defaults, unauthorized actions, scientific non-claims, and an authorization checklist; exclude packet identities, digests, task/evidence/source/mapping content, human results, and launch authority |
| `evalwitness relation render-pilot-change-atlas` | verify readiness against the restricted bundle and private mappings, optionally require and bind the exact change receipt, then render an owner-only all-difference navigation aid for every pilot family with complete tasks, hidden logical alignment, content digests, exact common-prefix/suffix counts, every non-common rendered line, exact candidate-reversal proof, structural review flags, and no semantic decision or authorization |
| `evalwitness relation render-pilot-inspection\|pilot-inspection\|verify-pilot-inspection` | render one deterministic owner-only workbook containing reviewer-visible evidence plus hidden mapping truth, exclusively publish the complete packet-bound decision record at mode `0600`, then independently rebind it to readiness, bundle, and mappings while preserving derived status, `human_study_status=not_run`, and external action `not_authorized` |
| `evalwitness relation pilot-inspection-session-start\|pilot-inspection-session-guide\|pilot-inspection-session-record\|pilot-inspection-session-status\|pilot-inspection-session-finalize` | bind one immutable private session to the exact package-v5 inventory, governed v3 plan/primary/pilot/sentinel chain, readiness, bundle, mappings, three sentinel materials, workbook, atlas, scarcity appendix, inspector alias, seven core packets, three scarcity cases, and closed subject/dimension vocabularies; guide each target with hash-bound source line ranges and a fixed rubric prompt without printing restricted evidence; require exactly 50 latest core assessments, 12 latest scarcity-case assessments, and four latest scarcity-boundary assessments; append only explicit mode-0600 hash-chained assessments/corrections with exact `KIND:ID:DIMENSION:ASSESSMENT` confirmation; revalidate package and journal on every resume; derive core, scarcity, and conservative combined status; refuse gaps, edits, reorder, implicit overwrite, cross-package/cross-case state, core-only finalization, and unmatched completion confirmation; publish and independently reproduce the existing seven-packet v3 inspection record plus a combined completion receipt without creating a human-study or authorization claim |
| `evalwitness relation render-scarcity-inspection` | validate the exact v3 plan, primary, exhaustive scarcity sentinel, corpus plan, natural audit, release, and one restricted material per sentinel case; replay every material from the frozen sources before rendering the separate owner-only 2-development/1-calibration/0-test construct-availability appendix without creating a packet, label, result, held-out claim, primary-estimand input, or authorization |
| `evalwitness relation render-scarcity-public-brief --format json\|markdown` | validate only the public v3 corpus plan, 939-attempt audit, typed release, relation plan, balanced primary, and exhaustive scarcity sentinel; seal the closed content-addressed JSON evidence contract or deterministically render its Markdown view with the same 198/3/195 funnel, 2/1/0 roles, case/parent commitments, and explicit claim states; emit no restricted task/source/trajectory material, owner notes, labels, human/provider/verifier results, or authorization |
| `evalwitness relation render-owner-inspection-public-attestation` | revalidate the complete private package-v5, governed-v3, session, contiguous event, inspection-record, and completion chain; emit only the public package commitment, UTC date, 66-assessment/16-dimension aggregates, core/scarcity/boundary/combined outcomes, disclosure policy, and closed claim states; exclude every private journal, event, record, completion, packet, case, mapping, path, task, and evidence identity; preserve human/provider not-run, external not-authorized, held-out unsupported, corrected-corpus unsupported, and public-source-reproduction unavailable states |
| `scripts/build/prepare-relation-pilot.sh` | reproduce v1, v2, or v3 governance through production CLI paths; require distinct mode-0600 packet and qualification keys plus ordered timestamps; emit version-matched prelaunch artifacts, not-launchable dossier, public-safe launch brief, change receipt, receipt-bound atlas, and complete workbook; v3 also binds the natural-corpus plan/audit, separate sentinel, challenge/repair evidence, three replay-bound sentinel materials, the scarcity appendix, and an exact content-addressed payload inventory; publish package format v2/v3/v5 atomically only after complete verification |
| `scripts/audits/verify-relation-pilot-package.sh` | verify all six immutable owner packages across formats v1/v2/v3/v4/v5, mode-0700/0600 custody, SHA-256 bytes, versioned inventory identity, protocol/schema generation, exact packet/mapping and construct-firewall bindings, scarcity exclusion, no-empirical-state boundary, and independent corpus/governance/bundle/readiness/dossier/schema/brief/receipt/atlas/workbook reproduction; v5 additionally replays every sentinel material and reproduces the scarcity appendix; scan public-safe evidence without a provider or reviewer |
| `evalwitness relation judgment\|judgment-batch\|analyze-ambiguity\|assign-tie` | seal complete seven-axis visible-side judgments with immutable revision parents, commit exact assignment coverage, reproduce prereveal disagreement/uncertainty metrics, and assign only committed disagreements to an independent qualified slot-three reviewer |
| `evalwitness relation probe-batch\|reveal` | commit complete family/direction/source-condition/task-recognition guesses strictly after each primary judgment batch, then disclose only verified assignment seeds and mapping references after both probe batches and every required tie-break commitment |
| `evalwitness relation compare\|terminal-ledger` | normalize committed visible-side judgments only after reveal, apply conservative packet resolution, seal exact formal-human strata/probe/cluster/sensitivity evidence, then bind formal witness, human resolution, and explicitly unconsulted verifier layers in one terminal ledger |
| `evalwitness relation replay\|replay-v3` | resolve exact frozen source bytes, reapply one trajectory or candidate-order case under its version-matched mutation program, and emit a content-addressed digest-only replay receipt |
| `evalwitness relation materialize-v3\|packet-v3` | reproduce v3 case evidence from the exact corpus-plan/audit/release chain, bind the typed firewall and v3 contract/evidence boundary, then publish only the blinded packet while exclusively storing its owner-only mapping |
| `evalwitness relation translate` | map a complete family-specific normalized observation set to a content-addressed supports, contradicts, or unresolved result |
| `evalwitness relation validate\|schema` | strictly validate 97 closed relation schema documents: 92 version-closed protocol documents comprising 31 immutable v1 contracts, 30 historical protocol-v2 contracts, 30 protocol-v3 workflow contracts, and one scarcity-sentinel contract; the independent `scarcity-public-evidence` projection; the public `owner-inspection-public-attestation` projection; and three private guided owner-inspection session/event/completion contracts |
| `evalwitness agent-study build\|validate\|schema` | build, strictly validate, and describe the provider-free coding-agent-only formal study with two independent machine validators, disagreement-only tie-break custody, exact 20/20 calibration/test selection, source lineage, and zero provider/human actions |
| `scripts/audits/run-relation-governance-v2.sh` | provider-free complete-corpus reproduction of the v2 relation plan, balanced 32-case primary sample, eight-family zero-overlap pilot, and frozen amendment; reproduce every artifact twice, generate all 30 v2 schemas, execute a synthetic version-closed reviewer-to-terminal-ledger workflow, validate one real v2 materialization, reject construct and mixed-generation tampering, and preserve `not_run`/`not_authorized` boundaries |
| `scripts/audits/run-controlled-corruption-v3.sh` | provider-free double reproduction of the frozen v3 plan, complete 939-attempt natural audit, and nine-case verification-evidence challenge; enforce exact bytes/digests, 16 coverage cells, 689 applied, 250 rejected, 283 selected, the unchanged 3/40 omitted-evidence scarcity result, two eligible controls, and seven closed rejections |
| `scripts/audits/run-relation-governance-v3.sh` | provider-free double reproduction of the v3 plan, balanced 28-case/28-lineage primary, exhaustive three-case sentinel, disjoint seven-case pilot, and 28-group amendment; generate all 30 version-closed v3 workflow schemas, the sentinel schema, and three guided journal schemas; exercise corpus-to-owner-inspection and reviewer-to-ledger chains plus journal digest/inventory gates; independently reject mixed generations, overlap, held-out-sentinel claims, lineage reuse, journal tamper/truncation/reorder/cross-package state, empirical state, and authorization |

The current private owner-inspection input is `private/relation-pilot-v6`, package
format `evalwitness.relation-pilot-package.v5`. It contains seven v3 pilot
materials, packets, and mappings; the exact corpus plan, 939-attempt audit,
typed release, primary, pilot, amendment, and separate scarcity sentinel; frozen
public challenge and repair evidence; three separately materialized replay-bound
sentinel cases; three v3 package schemas; the complete pilot workbook and atlas;
the separate 3,684-line owner-only scarcity appendix; and no decision or
human/provider result. Its 53-file payload inventory covers 10,759,623 bytes and
has digest
`533deaaecd328d972cdf770073afb0f56e560d4aadea59be1e111d0782eafd80`;
the package summary SHA-256 is
`48c94c88223cd7052a418102e133340a35590c379ab9ee367d95854bcf24eca9`.
All six directories are mode `0700`, all 55 files are mode `0600`, the two
external 65-byte key files are distinct and mode `0600`, and independent full
reproduction passes with `owner_inspection_status=not_completed`,
`human_study_status=not_run`, `empirical_state_inheritance=none`, and
`external_action_status=not_authorized`. Guided inspection stores no restricted
packet content in its journal: the immutable header carries only package and
packet commitments, each event carries one explicit assessment, and corrections
supersede prior event digests without rewriting them. The current package remains
immutable because the journal vault must not equal or descend from its root.

Live `probe`, `doctor --live`, `attest`, `verify`, MCP, and Best-of-N calls use exact digest authorization. The plan binds entrypoint, route, request fingerprint and contract, retries, workers, expected/worst calls, output ceiling, and hard calls/attempts/input/output/duration/concurrency/optional-cost limits. Changed plans fail before network dispatch. `attest --task T --trajectory @FILE...` requires both input classes together and runs them through the same canonical preprocessing, evidence binding, and request planner as production verification; this permits route qualification at the actual request shape without weakening served-model identity. Best-of-N authorizes child execution first, persists immutable owner-only attempt artifacts, then requires the exact selection authorization from `run.json` through `--resume-run`; this prevents paid selection from being approved against placeholder trajectories. Live eval additionally requires `--study-record` containing an authorized, checksum-valid record; `--study-manifest-digest` is only an optional equality assertion and never authorizes by itself.

## Agent Self-Checks

Agents and CI should run:

```bash
./evalwitness doctor
./evalwitness attest
scripts/tests/run-replay-smoke.sh
scripts/tests/run-claimcheck.sh
scripts/audits/run-protocol-conformance.sh
scripts/audits/run-trace-interoperability.sh
scripts/audits/run-controlled-corruption.sh
scripts/audits/run-controlled-corruption-v2.sh
scripts/audits/run-controlled-corruption-v3.sh
scripts/audits/run-agent-only-study.sh
scripts/audits/run-stress-lab.sh
scripts/audits/run-outcome-validity.sh
scripts/audits/run-relation-construct.sh
```

Required healthy state:

- `provider=<configured route label>`
- `model=<exact requested model>`
- `wire_format=openai`
- `key_present=true`
- `attestation_state=bounded_qualified` or `study_qualified`
- `full_verifier=true` only for that exact current request contract

`./evalwitness doctor --output text` reports `thinking_mode`, `capability_status`, attestation identity/expiry, and `next`. `./evalwitness presets` lists configuration bundles only. The `*` marker denotes the default; every preset remains `configured` until an exact local attestation says otherwise. `doctor --live` and `probe` can establish only `probe_compatible`.

`scripts/tests/run-replay-smoke.sh` is the no-key local verifier smoke. It pins empty environment and config inputs plus the complete route semantics, then uses `scripts/tests/golden-delta-replay.jsonl` to prove canonical request construction, exact fingerprint lookup, response checksum validation, extraction, replay classification, and result formatting without provider calls or machine-local configuration. `scripts/tests/run-claimcheck.sh` is the broader no-key product claim gate: it builds a fresh temp binary, bypasses `.env`, verifies the compiled compatibility default and preset metadata without treating either as a live qualification, checks Terminal-Bench and SWE-bench dry-run metrics, pins lineage artifact digests, and runs replay smoke.

## MCP Server Integration

Exposed tools:
| tool | purpose |
|---|---|
| `evalwitness_pairwise` | select best from N trajectories with tournament |
| `evalwitness_absolute` | score a single trajectory in [0,1] |
| `evalwitness_delta` | compare A vs B with margin |
| `evalwitness_calibration_evaluate` | offline `IsDeployable` on a test-split observation array; not a held-out study or production policy |

For one signed transition release, deprecated aliases `logprobe_pairwise`, `logprobe_absolute`, and `logprobe_delta` expose byte-identical schemas and dispatch to the same handlers. Canonical tools are listed first. New integrations must use only `evalwitness_*`.

Current protocol revision: `2026-07-28`. Modern requests are stateless and
carry `io.modelcontextprotocol/protocolVersion` plus an object-valued
`io.modelcontextprotocol/clientCapabilities` in `params._meta`; optional client
identity must contain both name and version. `server/discover` advertises the
current revision, the four tested legacy revisions, tool capability,
instructions, and public cache metadata. Modern `tools/list` and `tools/call`
need no initialize handshake. Their responses carry `resultType=complete`,
server identity metadata, and structured tool content; expected tool failures
are visible `isError=true` tool results rather than transport failures. Modern
`ping` and `logging/setLevel` are removed methods and fail with -32601; optional
per-request log-level metadata replaces the latter.

Legacy clients remain compatible with `2025-11-25`, `2025-06-18`,
`2025-03-26`, and `2024-11-05`. Those revisions use `initialize`, echo the exact
requested supported revision, require `notifications/initialized`, and preserve
their historical response shape. Transport is JSON-RPC 2.0 over stdio with one
newline-delimited object per frame. Stderr is redacted diagnostics only; stdout
is protocol only. Calls aborted via `notifications/cancelled` return no
response. EOF cancels and drains in-flight work. Errors retain -32010 score
evidence rejected and -32011 live authorization required or changed alongside
the existing -32001 through -32009 taxonomy; unsupported modern protocol
metadata uses -32022. A first live MCP call returns -32011 with the complete
plan and executes only when repeated with `authorization_digest`; replay and
offline calls need no live approval.

### Claude Code

```bash
claude mcp add evalwitness -- /absolute/path/to/evalwitness mcp-serve
```

Script asset: `config/mcp/claude-code.sh`.

### Codex CLI

CLI:
```bash
codex mcp add evalwitness -- /absolute/path/to/evalwitness mcp-serve
```

Or `~/.codex/config.toml`:
```toml
[mcp_servers.evalwitness]
command = "/absolute/path/to/evalwitness"
args = ["mcp-serve"]

[mcp_servers.evalwitness.env]
EVALWITNESS_PROVIDER = "your-provider"
EVALWITNESS_WIRE_FORMAT = "openai"
EVALWITNESS_BASE_URL = "https://your-provider.example/v1"
EVALWITNESS_MODEL = "your-model"
EVALWITNESS_API_KEY = "${EVALWITNESS_API_KEY}"
```

Config asset: `config/mcp/codex.toml`.

### OpenCode

`~/.config/mcp/opencode.json` (or project-local):
```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "evalwitness": {
      "type": "local",
      "command": ["/absolute/path/to/evalwitness", "mcp-serve"],
      "enabled": true,
      "environment": {
        "EVALWITNESS_PROVIDER": "your-provider",
        "EVALWITNESS_WIRE_FORMAT": "openai",
        "EVALWITNESS_BASE_URL": "https://your-provider.example/v1",
        "EVALWITNESS_MODEL": "your-model",
        "EVALWITNESS_API_KEY": "${EVALWITNESS_API_KEY}"
      }
    }
  }
}
```

OpenCode quirks: root key is `mcp` (not `mcpServers`); `command` is a single array; `type: "local"` required.

Config asset: `config/mcp/opencode.json`.

### KiloCode and Other MCP Clients

Use the client's MCP settings UI or config import with:
| field | value |
|---|---|
| command | `/absolute/path/to/evalwitness` |
| args | `["mcp-serve"]` |
| env | set `EVALWITNESS_PROVIDER`, `EVALWITNESS_WIRE_FORMAT`, `EVALWITNESS_BASE_URL`, `EVALWITNESS_MODEL`, and `EVALWITNESS_API_KEY`, or select a preset |

Generic config assets: `config/mcp/generic-mcp.json` and `config/mcp/kilocode.jsonc`.

## Agent Skill

Repo-local skill: `.agents/skills/evalwitness-audit`.

Purpose: teach coding agents how to deploy and use EvalWitness without guessing provider status or overclaiming results.

The skill requires agents to:

- require a current `bounded_qualified` or `study_qualified` attestation before calling a route paper-grade
- treat `probe` and `doctor --live` as `probe_compatible` diagnostics only
- run `scripts/tests/run-replay-smoke.sh` for a no-key local smoke
- use `./evalwitness presets` to inspect built-in providers
- reuse copy-paste MCP config assets from `config/mcp/`
- wire MCP with command `/absolute/path/to/evalwitness` and args `["mcp-serve"]`
- keep API keys out of logs and user-visible output
- distinguish dataset dry-run baselines from live reproduced paper metrics

## Provider support appendix

The primary setup path is the custom OpenAI-compatible route. Presets are
optional convenience bundles and registry/test fixtures; none qualifies a route
for empirical use. A fresh exact attestation is required for the selected model,
request contract, and strict Top-20 evidence.

<details>
<summary>Registered preset examples</summary>

| EVALWITNESS_PRESET | provider label | wire format | base URL | model | key |
|---|---|---|---|---|---|
| `bai-deepseek-v4-flash` | bai | openai | `https://api.b.ai/v1` | `deepseek-v4-flash` | `BAI_API_KEY` |
| `deepseek-v4-pro` | deepseek | openai | `https://api.deepseek.com` | `deepseek-v4-pro` | `DEEPSEEK_API_KEY` |
| `deepseek-v4-flash` | deepseek | openai | `https://api.deepseek.com` | `deepseek-v4-flash` | `DEEPSEEK_API_KEY` |
| `fireworks-deepseek-v4-flash-0731` | fireworks | openai | `https://api.fireworks.ai/inference/v1` | `deepseek-v4-flash-0731` | `FIREWORKS_API_KEY` |
| `opencode-go-deepseek-v4-flash-0731` | opencode-go-cn | openai | `https://opencode.ai/zen/go/v1` | `deepseek-v4-flash` | `OPENCODE_GO_API_KEY` |
| `openrouter-ambient-deepseek-v4-flash-0731` | openrouter-ambient | openai | `https://openrouter.ai/api/v1` | `deepseek/deepseek-v4-flash-0731` | `OPENROUTER_API_KEY` |
| `openrouter-morph-deepseek-v4-flash-0731` | openrouter-morph | openai | `https://openrouter.ai/api/v1` | `deepseek/deepseek-v4-flash-0731` | `OPENROUTER_API_KEY` |

</details>

One wire format. For custom providers, user supplies provider label, OpenAI-compatible endpoint URL, model id, and API key.

| EVALWITNESS_WIRE_FORMAT | auth header | default base URL | use for |
|---|---|---|---|
| `openai` | `Authorization: Bearer` | `https://api.openai.com/v1` | DeepSeek and any OpenAI-compatible endpoint or proxy |

Set `EVALWITNESS_PROVIDER` to the label you want in cache/audit output, for example `deepseek` or `company-gateway`. API key lookup prefers `<UPPER_PROVIDER>_API_KEY`, then `EVALWITNESS_API_KEY`.
Pinned presets are intentionally not environment-retargetable: each route
identity must match its declared upstream. A mismatched provider label fails
before dispatch.

Capabilities (logprobs, prompt-cache, streaming) may be diagnosed with `evalwitness probe`, but that result is weak `probe_compatible` evidence only. Full-verifier use requires `evalwitness attest` to execute a bounded real score prompt and persist `evalwitness.capability-attestation.v1` for the exact route and request contract. The attestation must be current, strict, non-degenerate, Top-20, complete for every requested score tag, and free of judge fallback. `scripts/tests/run-claimcheck.sh` verifies that presets and offline claims never manufacture this state.

Routes are classified by capability first; price is a separate attribute recorded
per preset and never a qualification criterion.

The registry is deliberately one model family. It stores configuration bundles,
not live prices, permanent badges, or checkpoint truth. Historical observations
belong to time-bound artifacts. A local or community attestation can report only
its signed observation level and limitations; it cannot self-prove provider
identity, provider endorsement, or an alias-to-checkpoint mapping.

DeepSeek's official API docs list `https://api.deepseek.com`, `deepseek-v4-pro`, and Chat Completions `logprobs` / `top_logprobs` up to 20. Direct DeepSeek presets send `thinking: {"type":"disabled"}` because thinking mode rejects `logprobs` / `top_logprobs`; evalwitness needs final-answer score-token probabilities. This task did not run a direct DeepSeek live call.

## Architecture

| layer | path | responsibility |
|---|---|---|
| cmd | cmd/evalwitness | CLI entry, subcommand router |
| config | internal/config | env + .env auto-discovery + flag merger |
| log | internal/log | stderr structured logging with redaction |
| provider | internal/provider | canonical request and response identity; sole Provider envelope port; OpenAI-compatible Chat Completions wire format; retry/classify; capability probe |
| conformance | internal/conformance | route-state machine, exact capability attestations, privacy-safe signed derivatives, freshness, and deterministic failure server |
| verifier | internal/verifier | versioned score evidence, strict extraction policy, censored-support comparison, uncertainty vocabulary, prompts, and criteria |
| preprocess | internal/preprocess | bounded source ingestion, versioned canonical events and causal links, redaction-before-selection, hard evidence budgets, derivation lineage, and fidelity accounting |
| sprt | internal/sprt | Wald's SPRT (pairwise) and absolute-mode variance termination |
| cost | internal/cost | configured per-session rates + Calculator (subscription override) |
| audit | internal/audit | jsonl audit log writer (concurrency-safe) |
| replay | internal/replay | fail-closed exact ReplayProvider, atomically publishing CapturingProvider, and non-destructive legacy inspection migration |
| safety | internal/safety | protected-path policy, cache-root ownership, route namespace IDs, bounded owner-only reads, and atomic publication |
| capsule | internal/capsule | closed component registry, content-addressed provenance DAG, public/private family, deterministic archive, in-toto statement, DSSE, and explicit-key Sigstore verification |
| claim | internal/claim | canonical ledger, exact expressions, evidence ceilings, lifecycle, challenge receipts, generated surfaces, and Claim Autopsy |
| explorer | internal/explorer, web/explorer | verified report projection, deterministic embedded renderer, offline responsive claim autopsy, challenge replay surface, typed extension registry, and accessibility/browser gates |
| protocol | protocol | public verifier-audit objects, strict canonical JSON, normative corpus, capability matrix, closed-form evaluator, and NDJSON process adapter |
| cache | internal/cache | route-scoped hash-keyed JSON response/capability cache with explicit clear scopes |
| study | internal/study | closed study schemas, canonical identity, immutable lifecycle, dataset-bound split lineage, historical-data inventory, protocol report, and exact execution binding |
| lineage | internal/lineage | sealed verification-lineage research plan, ten closed content-addressed artifact contracts, acyclic typed parent DAG, exclusive terminal-state taxonomy, role and holdout isolation, claim ceilings, strict codecs, and schema inventory |
| mutation | internal/mutation | controlled trajectory transformations, formal and hermetic relation validation, minimal changed-region witnesses, blind-review packets, corpus governance, and leakage checks |
| stress | internal/stress | versioned metamorphic relations, construct admission, stage localization, result accounting, v3 replay/arm/held-out governance, counterexample provenance, deterministic development-case-study artifacts, and a self-contained challenge/receipt verifier |
| outcome | internal/outcome | outcome/evidence ledger, HMAC blinding, development inventory and bounded pilot, authorization-explicit readiness, bounded reruns, reviewer consent and qualification, randomized assignment, complete label and source-probe commitments, prereveal rubric-ambiguity and post-reveal semantic-blinding analyses, post-commit reveal, agreement, adjudication ledger, post-ledger performance-blind source reconciliation, revision, and preservation |
| reliance | internal/reliance | frozen evidence-factor ontology and assignments, separated reliance estimands, preregistered factorial design, exact-replay task panels, complete registered-denominator accounting, clustered analysis, selector audit, arm comparison, one-minimal witnesses, public capsule binding, and deterministic profile/paper/Explorer projections |
| mode | internal/mode | per-mode orchestration, adaptive pair decisions, dynamic tournament, live authorization, and concurrency-safe run budget |
| verification | internal/verification | one application plan, runtime, batch, decision-admissibility, audit, budget, lineage, and cleanup contract for every production entrypoint |
| mcp | internal/mcp | lifecycle-enforced JSON-RPC 2.0 stdio server with three canonical tools and exact transition aliases |

Data flow: CLI/MCP/eval/Best-of-N/protocol syntax -> bounded source or trace adapter -> `evalwitness.trajectory.v1` events, links, ingestion report, trace envelope, and mapping report -> one verification plan with prepared evidence, caller-ordered criteria, service-owned lineage, canonical request set, and run fingerprint -> exact replay/offline path or current attestation plus exact authorization digest -> one shared logical-call/attempt/token/cost/concurrency/deadline budget -> provider attempts with retry audit -> immutable response and strict token-aligned `ScoreEvidence` -> versioned decision policy -> selection, tie, abstention, or failure -> resource cleanup -> run-level audit -> checked output. Batch evaluation plans all items first and executes them through one runtime and aggregate budget; cancellation is checked before each dispatch, so undispatched cells consume no call or attempt budget. Benchmark evals add an authorized immutable study record and verify its route, current attestation, build, analysis, declared inputs, statistical design, and budget before live authorization.

Strict verifier extraction is the default for CLI, MCP, eval, and Best-of-N. Missing or degenerate logprobs, requested or returned Top-k below 20, failed alignment, visible-mass overflow, valid-score mass below 0.05, or corrupted numeric evidence receives at most three bounded extraction retries and then returns typed evidence failure. It is never converted into a mixed or judge result. `EVALWITNESS_JUDGE_MODE=true` is an independent text-only request, cache, replay, usage, and result identity.

## Score Evidence Contract

`evalwitness.score-evidence.v1` is the atomic extraction result. New research paths do not emit a naked extracted score. Each requested tag retains extraction mode, exact/ambiguous/missing/truncated/invalid alignment, aligned token and byte coordinates, requested and returned Top-k, visible probability mass, valid A-T mass, conservative unobserved mass, canonical support, conditional expectation and variance, ordered raw alternatives, and typed degradations. Alternatives retain rank, token text, lossless logprob text, converted probability, chosen-token status, canonical letter/value, and duplicate provenance.

Uppercase/lowercase aliases map to one canonical letter and keep the maximum probability across forms, matching the historical reference rule without double counting. Exact duplicate raw alternatives are a structural failure. Invalid letters, fragments, multi-letter score tokens, non-finite logprobs, duplicate tags, and response truncation remain visible diagnostics. The strict policy independently revalidates schema, mode, alignment, Top-k, probability bounds, mass conservation, support mass, and conditional moments rather than trusting `extracted=true`.

The extracted number is `conditional_expected_score`: an expectation conditioned on visible valid score tokens. It is not a full 20-class distribution, an empirical correctness probability, or calibrated confidence. `valid_score_mass` and `unobserved_probability_mass` remain separate in pair, delta, absolute, selection, audit, cache, replay, CLI, MCP, Best-of-N, and evaluation detail outputs.

`ScoreEvidenceComparison` treats Top-k as censored evidence. It reports support union/intersection and Jaccard overlap, probability overlap, common-support conditional total variation, visible/valid-mass movement, conditional-score and conditional-variance movement, and conservative score intervals for the missing tail. Absent alternatives are never fabricated as observed zero probability.

Pair decisions use `evalwitness.pair-decision.v2` and expose policy version, state, abstention reason, per-order/per-repeat evidence, repeat count, conditional-token variance, repeated-sample variance, presentation-order variance, policy variance, and their total. `model_win_probability` and `decision_strength` are explicitly `calibrated=false` until a held-out calibration model is available. Order reversal is a paired diagnostic, never an independent repeat. Absolute output `evalwitness.absolute-score.v2` omits confidence and emits conditional score plus evidence strength. Selection and delta use version 2 schemas and cannot upgrade a pair abstention into a winner.

## Verifier Audit Protocol

EvalWitness ships protocol `1.2.0` as a narrow, offline audit-evidence
interchange and executable conformance kit. It does not define a general agent
trace, evaluator SDK, telemetry format, benchmark harness, certification, or
provider trust badge. The public Go package is `protocol`; JSON Schema 2020-12
artifacts live in `protocol/schemas`, and the positive, single-invariant
negative, compatibility, and required-field vectors live in `protocol/vectors`.

Five claim levels are deliberately non-transitive:

| level | executable statement | excluded inference |
|---|---|---|
| `syntax` | Named protocol messages and normative syntax cases validate | Scoring correctness, route capability, or reliability |
| `deterministic_replay` | Named sealed observations and evidence invariants reproduce offline | Current provider behavior |
| `live_score_evidence` | One separately authorized bounded route observation returned conformant score evidence | Accuracy, calibration, or future stability |
| `empirical_reliability` | One locked study measured its declared verifier failure modes on its stated population | Transfer outside that population |
| `independent_reproduction` | A named independent actor reproduced named frozen artifacts or a study | Universal correctness or endorsement |

The core objects are `EvaluatorDescriptor`, `AuditCase`, `TrajectoryRef`,
`AuditInvocation`, `ScoreEvidence`, `DecisionEvidence`, `AuditFinding`,
`InvocationResult`, `CapabilityMatrix`, and `AuditRun`. Score evidence retains
requested and returned Top-k, every visible alternative, canonical score
support, visible/valid/unobserved mass, exact alignment, conditional moments,
requested and served identity, route limitations, degradations, request and
response digests, and optional attestation identity. Decisions bind score,
budget, and provenance digests and use explicit selected, tied, abstained,
invalid, budget-exhausted, or provider-failed states. A naked scalar or winner is
not conformant.

Protocol canonical JSON is stricter than ordinary JSON: UTF-8 only, duplicate
keys rejected, object keys sorted, maximum depth 64, integers restricted to the
interoperable IEEE-754 range, and no JSON floats, exponents, positive signs, or
negative zero. Probabilities, logprobs, moments, effects, and uncertainty use
canonical decimal strings so another language can reproduce exact rational
checks. Explicit `null` and an absent optional field retain different identities.
Every invocation result is content sealed; every audit run binds the normative
corpus, unchanged canonical request corpus, all schema artifacts, evaluator
identity, case results, and capability matrix.

External evaluators use one JSON object per line over stdin/stdout. The state
machine is `hello -> hello_ack -> describe -> descriptor -> begin_run ->
run_started -> evaluate/evaluation_result* -> end_run -> run_result`. Message IDs,
reply IDs, run IDs, version, canonical policy, case count, and corpus digest are
checked. Stdout is protocol-only; stderr is the adapter's diagnostic channel.
Cancellation between sequential cases is acknowledged explicitly; an in-flight
deadline cancels and kills the adapter process. The host alone chooses the
literal executable, arguments, working directory, environment, deadline, and
network policy. Audit cases cannot select any of them or turn an offline run into
a live run.

Extensions use reverse-domain namespaces and named schemas. Unknown required
extensions fail closed; unknown optional extensions are preserved. The reserved
`org.evalwitness.reliability.v1` extension carries evidence factors,
interventions, and reliance contrasts with exact parent, child, changed-set, and
result digests. Its only allowed interpretation is `verifier_output_reliance`;
it does not claim agent-step causality or explain model reasoning.

Use `evalwitness protocol run` for the embedded closed-form evaluator,
`evalwitness protocol run --adapter /literal/path --adapter-arg ...` for an
operator-selected process, `evalwitness protocol cases` for the expanded corpus,
`evalwitness protocol schema --name audit-case.schema.json` for an embedded
schema, and `evalwitness protocol reference-adapter` only as the process port for
the shipped closed-form evaluator. `scripts/audits/run-protocol-conformance.sh`
builds a fresh binary in temporary storage and checks 188 cases, including 111
required-field removals, through both direct and real subprocess paths with an
empty environment and no provider access. It also runs the independent Python
canonical fingerprint verifier. This proves only syntax and deterministic replay;
the other matrix levels remain `not_run` with reasons.

## Modes

### delta
Compare two trajectories using the same balanced K=1 evidence decision and per-pair call ceiling as pairwise. The verdict exposes `state`, `abstention_reason`, `conditional_margin`, conditional per-trajectory/per-criterion scores, per-observation evidence, evidence strength, and the pair decision. An abstention has `winner=none`; a tie has `winner=tie`.

### absolute
Score a single trajectory conditionally in [0,1]. Optional SPRT-style variance-based early stop. `evalwitness.absolute-score.v2` returns `state`, `conditional_score`, `per_criterion_conditional_scores`, criterion evidence, evidence strength, and usage. It never turns one observation's zero dispersion into confidence.

### pairwise
N trajectories (2-10). Each pair starts with one task-hash-balanced order. Conditional Top-20 moments drive bounded escalation while strict evidence coverage gates admissibility; the hard ceiling is `EVALWITNESS_MAX_PAIR_CALLS` provider calls per pair with bundled criteria. Each pair can select, tie, or abstain. Any pair abstention propagates to selection state rather than being published as a winner. A dynamic bracket remains the default execution strategy; `EVALWITNESS_SINGLE_ELIM=false` restores exhaustive all-pairs. Historical benchmark figures below use legacy output semantics and remain development evidence, not evidence-era reruns.

`EVALWITNESS_SELECTION=joint_absolute` is the fixed-graph research path. One
prompt contains the ordered candidate set, one score tag per candidate and
criterion, and one response record. Candidate scores are independent absolute
judgments inside that prompt; the strategy does not branch on intermediate
winners. Fixed repetitions only. This makes distribution-aware and chosen-token
analysis consume identical response bytes without requiring all-pairs capture.

## Optimizations Active

| name | mechanism | gating |
|---|---|---|
| globally bounded fanout | eval tasks and pair matches overlap, while one Runner semaphore enforces the configured provider-call ceiling across all tasks | always on |
| disk cache | hash-keyed JSON, atomic writes | EVALWITNESS_NO_CACHE=false |
| adaptive K=1 | one balanced order; Top-20 uncertainty triggers reverse/second sample only when needed | EVALWITNESS_BIAS_MITIGATION=adaptive |
| multi-criterion bundling | one prompt scores all criteria | EVALWITNESS_MULTI_CRITERION_BUNDLE=true |
| Top-20 evidence | conditional moments, coverage, censored support, raw alternatives, typed degradation, and evidence strength | every score path |
| hard pair ceiling | never exceed EVALWITNESS_MAX_PAIR_CALLS; invalid unbundled configurations fail before dispatch | adaptive pairwise |
| dynamic tournament | actual winner advances; N-1 matches | default; EVALWITNESS_SINGLE_ELIM=false for all-pairs |
| legacy SPRT | Wald's test for explicitly requested multi-rep compatibility runs | EVALWITNESS_SPRT=true |
| critique-then-score | brief critique before score tags | EVALWITNESS_CRITIQUE_THEN_SCORE=true |
| evidence slicing | retain diffs, edits, tests, failures, exit codes and final state before ranking whole steps | when trajectory exceeds evidence budget |
| trajectory redaction | regex blocklist for secrets | EVALWITNESS_NO_REDACT=false |
| think-block stripping | strip `<think>...</think>` before explicit judge extraction; strict verifier remains token-aligned | always on |
| pre-skip identical traces | sha256 match -> 0.5/0.5, no API call | always on |
| retry with backoff | `evalwitness.provider-retry.v3`: classify HTTP errors, honor integer or HTTP-date Retry-After, exponential backoff with jitter; HTTP 402 credit/payment failures are terminal; a known upstream HTTP 400 capability error closes pooled connections and retries within the declared attempt ceiling; all other capability failures remain terminal; extraction gets at most three bounded retries before typed failure | always on |
| cost cap | refuse a provider call before HTTP if estimated cost exceeds cap | EVALWITNESS_MAX_COST_USD_PER_CALL > 0 |
| run budget | shared hard logical calls, outbound attempts, estimated input, reserved output, concurrency, duration, and optional cost; terminal observed usage reconciles reservations while missing usage retains the worst case | every live entrypoint; optional persistent state with EVALWITNESS_RUN_BUDGET_STATE |
| single-pass preprocessing | eval preflight retains prepared evidence and selection reuses it instead of parsing every trajectory twice | eval-terminal / eval-swebench |
| offline mode | refuse all network calls | EVALWITNESS_OFFLINE=true |

## Order-Bias Mitigation

LLM judges can score `(A=i, B=j)` differently from `(A=j, B=i)`. The production default assigns the first order deterministically from the task and pair hash, which balances position across a dataset without doubling every pair. A reverse-order call runs only when the first Top-20 distribution does not cross `EVALWITNESS_PAIR_CONFIDENCE`. The default ceiling is two calls per pair and the accepted configurable ceiling is four. Explicit `both`, `single`, and `disabled` modes remain for compatibility and paper-parity work. The raw decision threshold is a legacy operational rule, not a calibrated probability claim.

`adaptive` requires `n_reps=1`. Explicit multi-rep work must select `both` or `single`; invalid combinations fail before provider dispatch instead of silently changing semantics.

## Multi-Criterion Bundling

When configured, all criteria for a (pair, rep) get scored in a single prompt with `<score_A_<id>>` / `<score_B_<id>>` tag pairs per criterion in explicit caller order. Caller order is part of the request contract and is never silently rewritten; only values derived from maps or unordered sets are lexicographically sorted before hashing or serialization. Bundling reduces call count by N-fold for typical three-criterion setups.

## SPRT Adaptive Reps

### Judge route

### Usage accounting on streaming routes

Streaming early-stop cuts the connection the moment the score tags are complete,
but providers emit the usage chunk last. Cutting there discarded it, so a
streaming route reported an input size of zero: the published SWE-bench artifact
claimed `input_tokens: 0` for 326 real calls, while a non-streaming run
reported a correct 35k per call.

The stream is now drained for a bounded number of further chunks after the tags
complete, purely to collect usage, so a rambling model still cannot hold the run
open. Where a provider reports nothing at all, and for cache entries written
before this fix, the same bytes/4 estimate the hard budget already reserves is
substituted and `usage.estimated_input_calls` records how many calls that
applies to. An artifact therefore never publishes an input figure of zero, and
never presents an estimate as a measurement.

## Best-of-N Orchestration (bon)

`evalwitness bon -n <N> --task <inline|@file> [flags] -- <agent command...>` turns the verifier into a selection loop for real work:

1. Validates a Git repository and records the exact destination commit, content tree, index tree, and status digest. A dirty source fails by default. `--include-working-tree` explicitly materializes the tracked and allowed untracked state without changing the user's index, rechecks the tree for concurrent mutation, and records the snapshot commit/tree.
2. Authorizes the bounded child-execution plan, creates N detached worktrees, and runs the literal agent command with an allowlisted environment. `EVALWITNESS_BON_ATTEMPT` carries the index; `--pass-env NAME,...` is the only secret-passing mechanism; literal secret-like arguments are rejected. The parent context owns timeout and the complete child process group.
3. Captures bounded, redacted transcript tails and a maximum 64 MiB binary diff. Temporary indexes and object databases prevent attempt snapshots from adding objects to the user's repository. Each `attempt-<i>.diff` and `.transcript` is created once with mode 0600, synced, closed, hashed, and verified before its worktree is removed. Cleanup errors report the retained path.
4. Persists immutable `evalwitness.best-of-n-run.v1` state in `run.json`. Live selection stops here and prints the exact `--resume-run ... --authorize ...` command because the real trajectories did not exist at the initial authorization boundary.
5. Resume strictly decodes the bounded regular manifest, requires exact artifact names, private regular files, size ceilings, matching digests and attempt indices, revalidates the destination state, then invokes pairwise selection through `internal/verification`. A selected result is required before apply.
6. `--apply` prints the patch summary, rechecks the destination digest and conflict-free `git apply --check`, applies without `--index`, and verifies the destination index tree is unchanged. Concurrent or conflicting user changes leave both destination and patch untouched.

Flags: `-n` (2-10), `--task` (required), `--criteria` (default `specification,error_signals,code_quality`), `--n-reps`, `--parallel`, `--keep`, `--apply`, `--include-working-tree`, `--pass-env`, `--resume-run`, `--timeout-sec`, `--output json|text`, plus the shared live budget and authorization flags.

Agent recipes:

| agent | command shape |
|---|---|
| Claude Code | `evalwitness bon -n 3 --task @task.md -- claude -p "$(cat task.md)" --permission-mode acceptEdits` |
| Codex CLI | `evalwitness bon -n 3 --task @task.md -- codex exec --full-auto "$(cat task.md)"` |
| OpenCode | `evalwitness bon -n 3 --task @task.md -- opencode run "$(cat task.md)"` |
| shell pipeline | `evalwitness bon -n 3 --task "fix flaky test" -- sh -c 'my-agent < task.txt'` |

Each attempt runs in its own lifecycle-owned worktree. Run artifacts remain for explicit resume and inspection; attempt worktrees are removed unless `--keep` is set. The only persistent Git objects created in the user's object store are those required by an explicitly requested working-tree source snapshot.

## Judge Mode

Full evalwitness verification needs output-token logprobs. Routes without logprobs can run only through explicit **LLM-as-a-Judge** raw-text extraction:

| switch | effect |
|---|---|
| `EVALWITNESS_JUDGE_MODE=true` / `--judge-mode` on verify, eval-terminal, eval-swebench, bon-selection | skip logprob requests entirely, extract scores from raw text |
| `EVALWITNESS_ALLOW_JUDGE_MODE=true` | doctor reports a logprob-less route as `capability_status=judge-mode` (usable, with caveat) instead of a problem |

Judge and verifier requests never share a cache identity or replay fixture. Every result and audit line carries the actual extraction path: `usage.extraction_mode` is `verifier` or `judge`, `usage.judge_text_calls` counts explicit text calls, and audit entries carry per-call `logprobs` plus full `score_evidence`. A malformed judge response fails extraction; it never becomes a 0.5 default. `mixed` remains a diagnostic state and is inadmissible under the strict policy.

Honesty note: the paper's gains are measured for verifier mode. The judge-mode quality delta is unmeasured until a comparison run is published. Measure it yourself with:

```bash
./evalwitness eval-terminal --paper-parity --output json > verifier.json
./evalwitness eval-terminal --paper-parity --judge-mode --output json > judge.json
```

## Cost Model

Cost rates are configured via env or preset: `EVALWITNESS_INPUT_USD_PER_M`, `EVALWITNESS_CACHED_USD_PER_M`, `EVALWITNESS_OUTPUT_USD_PER_M`. `est_cost_usd` is reported when rates are known. `EVALWITNESS_BILLING_MODEL=subscription` collapses monetary cost to 0 without disabling calls, attempts, token, concurrency, or duration limits. `EVALWITNESS_MAX_COST_USD_PER_CALL` enforces a per-call ceiling. Preflight calculates expected/worst logical calls, attempt headroom, estimated input, reserved output, optional cost, concurrency, and duration. One shared budget reserves each logical call and every outbound attempt before HTTP. Terminal responses reconcile input/output/cached-token cost only when upstream usage was observed; missing usage and failures retain the worst reservation, and actual overruns fail explicitly. Cache/replay hits consume no network attempt. `EVALWITNESS_RUN_BUDGET_STATE=/secure/path.json` persists the start time and aggregate reservations atomically with mode 0600; one state file per run means a restart cannot reset the budget.

## Audit Log

`EVALWITNESS_AUDIT_LOG=path/to/audit.jsonl` enables checked JSONL evidence with three row kinds. `provider_attempt` records every real outbound attempt, including retry ordinal, status, usage, request fingerprint, trajectory evidence, and redacted failure. `provider_call` binds the completed request, immutable response, strict score evidence, cache/replay classification, served identity, and request lineage. `verification_run` binds the run/request-set fingerprints, mode, decision policy and terminal state, inconsistency, aggregate budget, output status, actual runtime/execution/cleanup state, and successful run-row write state. The run row is written only after resource cleanup and before the audit sink closes; a later close failure is returned in the command result and exit status because a failed sink cannot safely rewrite its own final row. `verification_run` rows also carry `calibration_policy.status=unsupported_no_held_out_policy`.

Run lineage carries validated caller references for audit case, transformation, outcome evidence, profile policy, capsule, and study cell plus service-generated ordered source-trace, trace-map, and decision-policy digests. Caller references are bounded UTF-8 identifiers or exact lowercase SHA-256 values; callers cannot overwrite service-generated request or score evidence. Reverse-order pair calls swap prompt evidence and provenance together. Redaction applies to all errors and fields; prompt/response content and API keys are never written.

## Evaluation Harness

`evalwitness eval-terminal` and `evalwitness eval-swebench` are the benchmark trajectory surfaces. New artifacts declare `evalwitness.evaluation.v2`; historical unversioned artifacts remain immutable legacy inputs and claimcheck refuses to treat them as evidence-era output. Dry-run planning performs the same parsing, redaction, evidence preparation, request planning, hard-limit derivation, and statistical design pass without requiring an API key or opening a provider runtime. Live execution requires `--study-record` in `authorized` state, the current arm attestation, exact manifest-bound execution inputs, and the second-step live authorization digest. An arbitrary manifest digest is insufficient. The two runners translate benchmark rows into `verification.Input`, call one `PlanBatch`, then execute all selections through one runtime, prepared-evidence set, worker ceiling, operation deadline, and aggregate budget. Hard limits come only from that exact aggregate request plan; the descriptive estimator cannot override them. Pairwise plans report `pair_matches`; absolute-selection plans report `candidate_scores` and derive calls, input, cost, and duration from single-candidate prompts. Every plan embeds total and decidable task counts plus exact design sensitivity. Compact escalation evidence contains evaluated/escalated pair counts, the call histogram, inconsistency count, mean decision strength, and mean minimum valid-score mass. Production adaptive pairs stop after the first distribution-backed observation (`confidence_escalation=disabled`); they do not spend extra calls from `EVALWITNESS_PAIR_CONFIDENCE`. `--details` retains complete pair evidence. Eval-terminal and eval-swebench summaries also emit `calibration_policy.status=unsupported_no_held_out_policy`; that field is not a deployable correctness probability. A tied or abstained selection has no selected reward and prevents an aggregate verifier score from being emitted. The nested development reliability block remains immutable `logprobe.reliability.v1` until its later migration.

Trajectory data is about 226 MB unpacked (89 Terminal-Bench tasks x 5
trajectories, 500 SWE-bench Verified instances x 3 runs) and is NOT tracked in
git. No current downloadable release is claimed. Fetching is valid only after
an authorized release or mirror with the checksum-bound archives is independently
verified:

| step | command |
|---|---|
| fetch + verify + unpack | `eval/fetch-eval-data.sh [verified-tag]` (gh CLI) or `EVALWITNESS_DATA_BASE_URL=<verified-mirror> eval/fetch-eval-data.sh` |
| pack for release (maintainer) | `scripts/build/pack-eval-data.sh` -> secret/path scan against the exact-content reviewed-findings manifest -> safe archive inspection -> `dist-eval-data/*.tar.gz` + `SHA256SUMS-eval-data`; upload with `gh release upload <tag> dist-eval-data/*`. See [releasing.md](releasing.md) — this step is manual and the first release of a repository needs it |

Eval commands print an actionable fetch hint when data is missing; `run-claimcheck.sh` skips dataset claims with an explicit SKIP line when data is absent and enforces them when present.

| command | purpose |
|---|---|
| `evalwitness eval-terminal --dry-run --output text` | load and preprocess 89 total / 17 decidable tasks; report Pass@1, oracle, exact plan, and power sensitivity without provider calls |
| `evalwitness eval-swebench --dry-run --output text` | load and preprocess 500 total / 86 decidable tasks; report Pass@1, oracle, exact plan, and power sensitivity without provider calls |
| `evalwitness design simulate --spec @design.json` | deterministic source-task-clustered sparse-factorial simulation with aliasing, MCSE, invalid/missing/abstention/route-failure denominators, and hard call/token ceilings |
| `evalwitness design reliance-preflight --code-digest SHA256` | reproduce the non-authorizing evidence-reliance Walsh alias audit, ascending source-task power search, grid MDE, and hard subscription-route resource envelope with zero provider calls |
| `evalwitness design identical-response --spec @design.json` | reproduce the frozen `distribution_aware_vs_chosen_token` paired design: exact conditional-McNemar detectable-effect and failure-sensitivity rows with zero provider calls and no Monte Carlo randomness |
| `evalwitness study identical-response-inventory --root .` | derive the frozen eligible development inventory (100 unique source task groups with data roles, licenses, redistribution classes, source/trajectory/evidence digests, outcome availability, and contamination boundaries) from the committed controlled-corruption release with zero provider calls |
| `evalwitness study identical-response-redistribution-right` | reproduce the frozen response redistribution-right evidence record: primary-source output-ownership assignment clauses, retrieval dates, conditions, and procedural verdicts for the DeepSeek direct and OpenCode Go candidate routes, with zero provider calls |
| `evalwitness study identical-response-protocol --root .` | reproduce the frozen identical-response study protocol (counterfactual, primary endpoint, task-group aggregation, missingness, multiplicity, fixed stopping, minimum support, and exact uncertainty procedure) bound by digest to the design, inventory, and redistribution-right artifacts, with zero provider calls |
| Governed corpus (`eval/governance/identical-response-dataset-manifest-v1.json`, `identical-response-frozen-split-v1.json`) | dataset manifest plus frozen split record over the same 100 source task groups: development/calibration only after previously accessed former test groups were demoted; near-duplicate IDs unique per group and lineage clusters verified role-pure. Failable gates in `internal/study/corpus_072_test.go` |
| `scripts/tests/run-replay-smoke.sh` | no-key deterministic CLI verifier smoke using replay fixture |
| `scripts/tests/run-claimcheck.sh` | no-key gate for default-route, preset, dry-run eval, and replay claims, plus every statistic the README quotes and the strict standalone validation, exact JSON/Markdown regeneration, digest pinning, and public scan of the scarcity result |
| `scripts/tests/run-artifact-safety.sh` | public-artifact secret, private-path, environment-dump, symlink, type, and mode gate over README, canonical docs, review manifest, and committed result artifacts |
| `evalwitness eval-terminal --dry-run --output json` (`.baselines`) | zero-cost selector suite: every artifact feature in both directions, scored over the same decidable tasks as the verifier and paired against it. Needs no provider, so it runs in dry runs |
| `scripts/audits/run-calibration-analysis.sh` | development reliability over published `--details` artifacts: fixed reliability bins, ECE, MCE, Brier, AUC, accuracy, task-cluster bootstrap intervals, and fixed near-zero/moderate/large wrong-direction strata on the raw win probability. No provider, no network |
| `scripts/audits/run-paired-analysis.sh` | typed inference over published `--details` artifacts through the shared Go statistics domain. Two arms that share a task set get exact McNemar, paired effect, Newcombe interval, multiplicity-adjusted rejection boundary, design resolution, and an explicit superiority/non-inferiority/equivalence conclusion; anything else is pooled against the random-pick baseline with an exact Poisson-binomial tail. Also reports per-arm tie rates |
| `scripts/audits/run-transcript-fidelity.sh` | no-provider conservation and category-retention audit over Claude Code, Codex, OpenCode, Terminal-Bench, SWE-bench, and plain-text inputs; defaults to the public golden corpus and can run bounded fuzz smokes |
| `scripts/audits/run-controlled-corruption.sh` | provider-free formal-control, schema, governed-spec, full-corpus, balance, review-sampling, and lineage gate; validates all 320 cases when fetched source trajectories exist and reports an explicit core-only skip otherwise |
| `scripts/audits/run-controlled-corruption-v3.sh` | provider-free typed-proof plan, complete natural-corpus audit, and verification-evidence challenge reproduction; validates the exact 939-attempt ledger, preserves the 37-case omitted-evidence shortfall, and pins nine provenance/failability/evidence-loss cases instead of relaxing the operator |
| `scripts/audits/run-agent-only-study.sh` | provider-free coding-agent-only formal study gate; builds the 20/20 artifact twice, validates plan/audit/release and source lineage, runs independent validators plus the disagreement-only tie-break tests, and asserts zero provider calls and human reviewers |
| `scripts/audits/run-stress-lab.sh` | provider-free double reproduction of the 15-relation catalog, public development case study, self-contained challenge/receipt, held-out campaign topology, and current readiness refusal; byte-compares all tracked stress projections, executes the challenge from an empty working directory and home, rejects active or external SVG payloads, validates the closed challenge, receipt, campaign-binding, preflight-evidence, preflight-custody, and exact execution-permit schemas, poisons provider configuration, and reproduces the 57-case held-out lock, all ten arm cell sets, repetition arithmetic, absent live bindings, and non-authorizing gate ledger when the ignored source caches are present |
| `scripts/build/prepare-outcome-pilot.sh` | provider-free, non-overwriting, owner-only preparation of the complete six-packet outcome pilot through the exact production CLI path; exercised by the outcome-validity audit |
| `scripts/build/prepare-relation-pilot.sh` | provider-free, non-overwriting, owner-only reproduction of historical v1-governance package format v2 or v2-governance package format v3 with distinct keys, immutable checksums, version-matched source/construct bindings, structural readiness, launch dossier, public-safe scanned launch brief, machine change receipt, receipt-bound atlas, and hidden/visible workbook; output remains defect-discovery evidence until a fresh v3 implementation exists |
| `scripts/audits/verify-relation-pilot-package.sh` | independently validate all historical and corrected relation package inventories, reject mixed protocol generations, reproduce bundle, readiness, dossier, schemas, brief, receipt, atlas, and workbook byte-for-byte, and scan the brief as a public artifact |
| `scripts/audits/run-outcome-validity.sh` | provider-free outcome-plan, 40-schema, natural-inventory, mutation-sample, non-launchable mixed-v1 diagnostic, six-case exact raw-source materialization, sealed private custody, sealed reviewability inspection, restricted authorization-explicit v3 readiness, HMAC-blinding, owner-only mapping/source binding, governed handbook, three self-contained rendered reviewer kits, hand-checked rubric-ambiguity, semantic-blinding, and post-ledger source analyses, three-reviewer qualification, randomized commit-before-reveal workflow, agreement, adjudication-ledger, and full-corpus binding gate |
| `evalwitness eval-terminal --limit 3 --n-reps 1 --max-workers 1 --study-record @authorized.json --output text` | validate the complete locked study binding and print the exact live authorization plan |
| `evalwitness eval-swebench --limit 3 --n-reps 1 --max-workers 1 --study-record @authorized.json --output text` | validate the complete locked SWE-bench study binding and print the exact live authorization plan |
| `evalwitness eval-terminal --paper-parity` | plan the reference-parity pipeline: paper criteria + benchmark note, no critique/bundling, single order, 4 reps |
| `evalwitness eval-swebench --paper-parity` | plan the same parity pipeline with SWE criteria root_cause/code_review/verification |
| `eval/scripts/run-terminal-bench.sh` | wrapper around the native evaluator |

The dry-run baseline for `forge_gpt54` is `Pass@1: 72.8/89 (81.8%)` and `Oracle: 80.0/89 (89.9%)`, with 17/89 tasks decidable, matching the bundled Terminal-Bench metadata. The SWE-bench Verified dry-run baseline is `Pass@1: 380.3/500 (76.1%)` and `Oracle: 422.0/500 (84.4%)`, with 86/500 instances decidable. These are dataset/oracle tallies, not live reproductions of paper headline verifier metrics. Full verifier reproduction requires an attested OpenAI-compatible route that returns strict score evidence. Configuration and historical route observations do not remove the requirement for a current route attestation.

## Study Governance

`internal/study` implements the preregistration boundary used by live benchmark
execution. It is a machine-enforced research contract, not a checklist. Strict
JSON decoding rejects unknown fields and trailing values under a 16 MiB limit;
`evalwitness study schema --type manifest|record|split` emits closed JSON Schema
2020-12 documents generated from the same typed domain.

`evalwitness.study-manifest.v1` locks identity and hypotheses; dataset sources,
versions, licenses, acquisition times, exact task/label/trajectory digests,
permitted roles and exclusions; dataset-bound split assignments; study arms;
primary, secondary and exploratory endpoints; paired inference and multiplicity;
failure and denominator handling; controls; per-arm attestation freshness, exact
served identity and checkpoint observations, versioned retry policy; hard budgets; clean build, analysis,
inputs and routes; publication gates; protocol, trace, outcome, adjudication and
profile contracts; plus kind-specific real-agent, controlled-relation, or
evidence-reliance governance. Controlled-relation manifests lock independent
corpus and relation-contract versions, source-task clustering, typed claims, and
validator digests. Evidence-reliance manifests lock every declared effect and
interaction into one explicit multiplicity family.

### Identity and lifecycle

Canonical standard-library JSON bytes produce the manifest SHA-256 and stable
`study-<manifest-digest>` identity. `study lock` creates
`evalwitness.study-record.v1` in `locked` state. Each later event contains the
previous record digest, UTC time, actor and reason; authorization additionally
contains exactly the current attestation digest locked by every arm. Permitted
transitions are `locked -> authorized|withdrawn`, `authorized ->
running|failed|withdrawn`, and `running -> complete|failed|withdrawn`. A content
or event mutation invalidates the record. The capsule kernel can now bind a
record and its descendants into immutable provenance. Release-level signature,
anti-rollback, and distribution policy remain release-level concerns; the study record
itself is checksum-bound and append-only by validation.

### Data and split boundary

The version-1 task-group stratified-hash algorithm uses a declared seed, ordered
stratification variables, integer role weights, SHA-256 ordering, and
largest-remainder allocation inside each dataset and stratum. Every assignment
names its dataset. Task IDs, repository IDs, clone families, trajectory digests,
pair observations, mutation descendants, evidence descendants, adjudication packets,
counterexamples, and corpus versions are globally leakage-checked; digest-named
descendants must be lowercase SHA-256. Dataset task counts, task-ID digests, and
trajectory-set digests are independently recomputed from assignments; roles must
be explicitly permitted, and decidable N cannot exceed locked N. Pair
observations therefore cannot leave their source-task group. Previously
accessed datasets cannot permit test or external-replication use.

`eval/governance/development-inventory.json` checksum-locks the exact eleven
published artifacts and the union of all 89 Terminal-Bench and 500 SWE-bench
task identities as `development` with `confirmation_permitted=false`.
`evalwitness study inventory` recomputes artifact, set and task-ID digests.

### Statistical and execution boundary

The primary outcome belongs to one locked Bonferroni family; every secondary
outcome belongs to exactly one declared secondary family; exploratory outcomes
cannot enter either. Superiority manifests use the exact unconditional McNemar
design and the validator independently recomputes locked power.
Non-inferiority and equivalence require a prespecified margin plus a digest-bound
paired joint-outcome simulation and declared power. Fixed-sample studies declare
one look; sequential studies require a method, boundary digest and at least two
looks. Positive controls can use development or calibration only.

Before provider dispatch, live eval verifies authorized state, entrypoint,
route, request contract, current route attestation, exact observed/expiry times,
served-identity policy and checkpoint assertion, retry version/count and request timeout, clean commit, binary and
analysis digests, analysis command/version, exact ordered input paths and
recursive file/directory identity and content digests, hard budget, decidable count, alpha, power, minimum
effect, disagreement model and multiplicity family. Declared inputs must be
clean repository-relative regular files or directories without symlinks; empty
directories are identity-bearing; extra, missing or changed inputs fail. `study
report` generates a registered protocol view from the record without observed
outcomes or manually authored claims. Its claim allowlist can name only
preregistered primary or secondary endpoints, and its timestamp must fall
between manifest creation and lock.

The served-identity contract is either `exact_observed` for one response model
identifier or `exact_observed_set` for a sorted, duplicate-free set of at least
two explicitly observed response identifiers. The latter does not authorize an
alias family: each member is named exactly, the requested model remains unchanged,
every record preserves the actual response identifier, and any unlisted third
identifier fails before capture admission. The policy and full expected set are
bound into the study execution and verification-plan fingerprints.

## Signal Reliability

Three mechanisms read one number. `win_probability` decides whether a pair is
settled after a single call, whether a second is worth spending, and whether the
reverse order is evaluated. `internal/reliability` measures whether that number
tracks correctness, over pairs whose trajectories differ in reward so a correct
answer exists. Equal-reward pairs are excluded rather than labelled, because
inventing a label there manufactures calibration out of nothing.

Ground truth exists only inside the eval harness, so the `calibration` block is
deliberately eval-only. A figure computed against the verifier's own score would
be circular and worse than none.

Metrics are computed on the raw probability, never on the folded confidence
`2*|p - 0.5|` that escalation consumes. Folding first collapses "confidently
right" and "confidently wrong" into one bin, which is precisely the failure the
package exists to detect. Bin count is fixed at ten: a bin count chosen after
seeing the curve is a free parameter and ECE is sensitive to it.

`Monotone` describes the observed ten-bin development curve. A non-monotone
sample invalidates using the raw value as a calibrated threshold on that sample.
It does not prove that isotonic, parametric, or another mapping fitted on a
calibration split will fail on held-out tasks. Measured 2026-08-07, no arm on
either benchmark is monotone in sample; fitting and testing a repair is outside
this measurement task.

`AUC` separates discrimination from calibration and the two must not be
conflated. A perfectly ranked but wildly overconfident signal scores 1.0 on AUC
and badly on ECE; the remedy for one is not the remedy for the other.

Uncertainty is clustered by task rather than by pair because several pair
decisions from one benchmark task are dependent. The deterministic bootstrap
draws 2,000 task-cluster replicates and reports percentile intervals for ECE,
MCE, Brier, AUC, accuracy, and each error-stratum share. Fixed error bands use
the absolute mean score difference for wrong-direction decisions: near-zero
`[0,0.05]`, moderate `(0.05,0.20]`, and large `(0.20,1]`. Exact `p=0.5` ties are
not directional errors.

Pairwise reliability and absolute-mode rank fidelity are separate expressions.
Pairwise runs report calibration and discrimination over `win_probability`.
Absolute runs report task-clustered Spearman rank correlation over standalone
trajectory scores and rewards. They are never folded into one headline metric.
Every source observation retains a stable row ID, task ID, pair or trajectory
identity, extraction mode, outcome identity, score difference or absolute
score, call count, pair-call limit, order policy, first order, retained score
mass, and inconsistency flag. `data_role=development` prevents these figures
from being mistaken for held-out validation.

### Held-out calibration (internal/calibration)

`internal/calibration` implements held-out calibration without test leakage. `Observation` carries only strict score-evidence fields (`conditional_diff`, `valid_mass`, etc.) plus `task_id` for cluster bootstrap; `SplitRole` (`development`/`calibration`/`test`) is enforced by `ValidateSplit` and `Lifecycle` digests. `FitPlatt` (Newton logistic on `conditional_diff`) and `FitIsotonic` (PAVA) are deterministic; `UncalibratedPolicy` and `LegacyPolicy` are explicit baselines. `EvaluateSelective` reports `coverage`, `selective_risk`, `AURC` and exact oracle `excess_AURC = AURC - (1 - p + p ln p)`; `EvaluateSelectiveWithIntervals` adds task-cluster bootstrap intervals (`2000` replicates, `0.95` percentile, `rand/v2` `PCG`). `BuildReport` emits `evalwitness.calibration.v1` with `FeatureSchema`, `Lifecycle` digests, `reliability.Metrics`, `SelectiveMetrics`, counts and deterministic `digest`. `SelectWithFallback` enforces explicit `FallbackKind` (`judge`/`human_review_handoff`/`no_action`) with budget. `NewLifecycleFromFile(path)` loads digests from explicit path against `study-manifest.v1` and is tested with `testdata/lifecycle.json` fixture. All metrics are provider-free and historically passed the race gate.


### Reliability profile (internal/profile)

`internal/profile` implements `evalwitness.profile.v1` without global score. `Profile` carries `Identity`, `ProtocolVersion`, `RouteScope`, `TimeWindow`, `Domains`, `DataRoles`, `EvidenceLevels`, `CapsuleParents`, `Dimensions` with strict `Status` `measured/failed/unsupported/not_applicable/not_measured`; `Build` validates `metric/scope/evidence_level/capsule_expression` for `measured` and computes deterministic `Digest` via sorted `Dimensions`/`Domains`/`DataRoles`/`EvidenceLevels`/`CapsuleParents` `sha256` over full profile. `Verify` requires non-empty `Digest` and checks equality, `Diff` refuses incompatible `ProtocolVersion`/`RouteScope` and sorts `Added`/`Removed`/`Changed`, `Evaluate` enforces policy `version` `digest` and fails on missing dimensions. `ToDimensions` wires `calibration.ReportArtifact` + `reliability.Report` without leakage; `selective_risk_coverage` preserves `upper bound` semantics. All `profile_test.go` `6/6` `race` green.

### Drift observatory (internal/drift)

`internal/drift` implements `evalwitness.canary-pack.v1` bounded `Pack` with `BuildPack` sorted `ID` `sha256` digest, `Canary` `purpose/task/request_contract/max_calls/max_tokens/max_time/license`, and descriptive `Thresholds` `capability_change/distribution_shift/calibration_invalidation` `Locked=false` until a study is locked. No schedule or live call by default.

### Registry (internal/registry)

`internal/registry` implements capsule `Matrix` without ranking, sorted `Digest`, `Verify(Entry, capsulePath)` hashes file `sha256` and compares `Digest` `Verified=true` only on match, `IsReady(Pilot, Matrix)` hashes `Pilot.CapsulePath` and matches verified entry. `audit.OfflineAudit` is offline `profile.Verify` + `profile.Evaluate` with `Fail`s and `PolicyDigest`. All `drift/registry -race` green.
## Zero-Cost Baselines

A selector that calls a model has to beat one that does not. `internal/baseline`
scores trajectory selection using only features the dataset already records -
step counts, durations, costs, trace size, failure vocabulary, and on SWE-bench
the size, file count and hunk count of the final patch. Every feature is
registered in **both** directions, because "fewest steps" and "most steps" are
different hypotheses and keeping only the one that wins after seeing results is
how a baseline suite stops being evidence.

Baselines need no provider and appear in `--dry-run`, so the bar a run has to
clear is visible before the run is paid for. They are scored over exactly the
decidable tasks the verifier is scored on, from the same candidate ordering, and
paired against its selection with McNemar exact from `internal/stats`. Each
result carries the smallest split that would have been significant, so a p-value
is never read without the sample that could have produced it.

Two design rules are load-bearing. Ties resolve to the lowest index
deterministically, which makes a baseline over a feature the benchmark does not
record exactly "pick the first" rather than something unreproducible. And
`Strongest` is the only selector the package offers over its own results: the
baseline a claim must beat is the best one, not a convenient one.

Measured on 2026-08-07, the strongest SWE-bench baseline outscores the verifier -
56 decidable tasks against 49 - and is the only selector on that benchmark to
significantly beat random. `scripts/tests/run-claimcheck.sh` pins that
relationship, so if it ever reverses the README section has to be rewritten
rather than quietly aging.

## Statistical Discipline

Every arm of every comparison in this repository selects from the same trajectory
sets. There is no fresh sampling between arms, so comparisons are **paired**, and
the difference of two scores is the wrong instrument for them — it discards the
pairing and overstates the evidence. `scripts/audits/run-paired-analysis.sh`
applies the right ones, over the committed `--details` artifacts, without a
provider.

| question | instrument | null hypothesis |
|---|---|---|
| does arm A beat arm B? | McNemar exact, two-sided, on discordant pairs | each disagreement is a fair coin |
| does selection beat no selection? | Poisson-binomial exact, one-sided | every task solved by a uniform random pick, which is what Pass@1 is |

Only **decidable** tasks enter either test: at least one trajectory passes and at
least one fails. Where every attempt passes or every attempt fails, selection
cannot change the outcome, and counting those tasks inflates both arms toward
whatever the dataset happens to contain.

`internal/stats.DecidableBinary` is the single definition used by eval planning,
reliability, baselines, and the Go implementation behind
`run-paired-analysis.sh`. Non-binary, single-candidate, all-pass, and all-fail
rows are not decision-informative. Terminal-Bench contains 89 total / 17
decidable tasks; SWE-bench Verified contains 500 total / 86 decidable instances.

### Prospective paired design

The exact two-sided rejection region for fixed discordant count `d` enumerates
every `b` for which `McNemarExact(b, d-b) < alpha`. Conditional power sums
`Binomial(d, q)` over that region for a declared discordant A-win probability
`q`. Before a run, `d` is unknown, so unconditional design power averages the
conditional power over `d ~ Binomial(decidable_tasks, disagreement_rate)`. The
task-level paired effect is `disagreement_rate * (2q - 1)`. Minimum detectable
effect is solved against this exact unconditional power, never by transforming
an observed p-value.

Eval design flags are `--design-alpha` (0.05), `--design-power` (0.80),
`--design-alternative-q` (0.75), `--disagreement-rates`
(`0,0.02,0.14,0.31`), `--minimum-effect`, `--equivalence-margin`,
`--inference-question`, and `--primary-family-size`. Every sensitivity row
reports expected discordance, declared-alternative power, complete-separation
power, and nominal/family-adjusted MDE. Bonferroni is the current explicit
preflight correction; the locked study manifest binds the final primary family
and method. The authorization fingerprint covers the complete
statistical design, so changing a margin, alternative, alpha, power target,
family size, or disagreement range invalidates approval.

At alpha 0.05 no exact two-sided McNemar split is significant below six
discordant tasks: `5:0` gives p=0.0625 and `6:0` gives p=0.03125. The shipped
Terminal-Bench design cannot attain 80% power under any default observed
disagreement scenario even at complete separation. On 86 decidable SWE-bench
tasks the nominal 80%-power MDE is 0.1136 at 14% disagreement and 0.1734 at 31%.
Claimcheck recomputes these values from the dry-run artifact.

### Completed paired analysis

Completed comparisons report paired task count, discordant count, task-level
effect A-B, Newcombe paired score interval (method 10) at the confidence level
required by the family-adjusted typed question, exact McNemar p-value,
nominal and family-adjusted alpha, smallest significant split, and typed
inference. `superiority`, `non_inferiority`, and `equivalence` are distinct
values. Superiority requires both exact p below adjusted alpha and interval
lower bound above zero. Non-inferiority requires a prespecified positive margin
and interval lower bound above its negative. Equivalence requires the complete
interval inside the prespecified symmetric margin. Failure to reject
superiority can only emit `superiority_not_established`; it cannot emit
equivalence. Completed artifacts contain no observed, achieved, or post-hoc
power field.

### Clustered and sparse-factorial design

`evalwitness design simulate --spec @design.json` consumes a strictly decoded
`ClusterSimulationSpec`. It locks source-task clusters, mutations per cell,
endpoint, seed, code digest, factors, prespecified interactions, sparse cell
fraction, ICC, invalid/missing/abstention/route-failure rates, alpha,
multiplicity family, per-source and per-observation resource coefficients, and
hard call/token limits. An optional `evalwitness.walsh-coded-factorial.v1`
design supplies exact power-of-two runs and one unique nonzero mask per factor;
it cannot be combined with random sparse-cell sampling. Continuous endpoints
use a Gaussian random intercept; binary endpoints use a logistic random
intercept. The analysis uses the frozen design matrix and source-task-cluster
sandwich covariance. Reports expose model and effect scales rather than
treating a binary log-odds input as a probability-difference estimate.

The simulator reports design rank and aliased terms before calls, planned and
effective source-task count, all failure denominators, estimability of every
main effect and interaction, empirical power, mean estimate, Monte Carlo
standard error, algorithm digest, caller-supplied code digest, and exact hard
budget. A deterministic sensitivity API varies ICC, invalidity, missingness,
abstention, and route failure with stable derived seeds. A grid-based inverse
solver returns the first prespecified factor effect reaching target power; it
does not interpolate a favorable value after seeing the result. Controlled
corruption and stress analysis use this at source-task/mutation-family level,
while evidence-reliance analysis uses it for sparse evidence-factor main effects
and locked interactions.

`evalwitness design reliance-preflight --code-digest SHA256` reproduces the
evidence-reliance specialization without opening a provider runtime. Its exhaustive
Walsh audit rejects 16 runs because eleven main effects exceed the eight-column
sum-free bound, checks all 135,408 admissible 11-column sum-free layouts at 32
runs and finds none that isolates the required interaction graph, and selects
64 runs. In the selected design no main effect shares a column with any two-
factor term, and every declared interaction is the unique two-factor pair on
its column.

The frozen design search uses 256 deterministic Monte Carlo replications,
Bonferroni family size 98, ICC 0.25, 15% retained post-randomization loss, a
continuous mean-difference target of 0.05 at residual SD 0.15, and a binary
log-odds target of 0.75. It requires both the estimated power and its 95% Wilson
lower bound to clear 0.80 for representative main-effect and interaction
columns, whose exact Walsh symmetry binds all preregistered terms in their
class. Twenty source tasks fail; 24 are the first passing candidate. The fixed
grid MDE is 0.04 on the continuous mean-difference scale and 0.75 on the binary
log-odds scale, whose selected-run mean estimate is about 0.113 to 0.114 on the
probability-difference scale. These are prospective simulation assumptions,
not effects learned from real verifier output.

The selected 24-task panel has 1,536 factorial cells plus one conservative
baseline call per task: 1,560 logical calls. With the current subscription-route
envelope of five retries, 32,000 evidence plus 4,000 prompt tokens, 4,096 output
tokens, 120 seconds per attempt, and concurrency eight, its fail-closed maxima
are 9,360 attempts, 336,960,000 input tokens, 38,338,560 output tokens, and
140,400 seconds. Marginal monetary cost is encoded as zero only because the
frozen billing model is `subscription`; this does not make calls, tokens, or
time free. The artifact has `live_authorized=false` and
`empirical_assumptions=false`; the later exact request plan and locked study may
tighten these limits but cannot silently exceed them.

Two consequences are load-bearing for how results are reported here.

**A raw score gap is not a result.** Terminal-Bench has 17 decidable tasks and
SWE-bench 86. A two-task difference is two discordant pairs, and two discordant
pairs cannot separate anything from chance. Both the verifier-versus-judge claim
and the evidence-budget and selection-strategy claims were stated as score gaps
before they were tested, and none of them survived in that form.

**Absence of a detected difference is not equivalence.** A cheaper option can be
descriptively attractive, but non-inferiority or equivalence requires a locked
margin and an interval or exact procedure capable of resolving it. The observed
tournament sample preserved the aggregate score at fewer calls; it does not prove
equivalence. The 16k sample likewise fails to prove harm and fails to exclude it,
so the conservative default stands and the cheaper tier stays opt-in.

## Evidence-Reliance Audit

The project ships a provider-free E1 evidence-reliance mechanism publication. It
contains the frozen design, estimator validation, canonical-event intervention,
atomic factorial-cell materialization, exact-replay task-panel execution,
registered-denominator analysis, production selector-and-renderer audit, paired
arm-comparison analysis, relation-status binding, one-minimal reliance witness,
public reference child-capsule binding, an 11-claim ledger, 98-dimension profile,
98-row paper projection, and an interactive Explorer panel. It does not contain
an external-adapter run, admitted external-model measurement, empirical
reliance result, transfer result, or live authorization.

The publication is rooted in
`eval/results/evidence-reliance-base-capsule-v1`, a frozen verified public TASK
050 snapshot. The canonical map cannot rebuild that base from the current Git
tree because the current tree already contains the map and its descendants;
doing so would create a self-reference cycle. `capsule build-reliance` therefore
loads the frozen base, derives one public child whose sole component is
`evalwitness.evidence-reliance-map.v1`, and emits the deterministic child archive
and ledger without overwriting any path. `capsule verify-reliance` independently
verifies the base, family edge, map identity, ledger, profile, paper, and Explorer
projection. Claimcheck rebuilds the child in a temporary directory and requires
byte identity with all five tracked public artifacts.

The tracked map covers 24 source-task mechanism fixtures, 64 cells per task,
1,536 registered and outcome-bearing cells, 1,560 logical exact-replay calls,
14 frozen terms, and seven outcomes for 98 term-outcome dimensions. It preserves
1,392 measured and 144 abstained cells as separate statuses, exposes ten selector
audits, all five prespecified arm-contrast families, and one public privacy-safe
one-minimal witness. The claim ledger supports CLM-035 through CLM-042 at E1 and
keeps CLM-043 through CLM-045 unsupported. A global explainability or trust score
is explicitly prohibited.

The frozen ontology `evalwitness.evidence-factor-ontology.v1` contains ten
factors: command exit, error output, executable outcome, irrelevant verbosity,
metadata, patch/edit, prompt injection, success/failure prose, test result, and
tool output. Each factor has exact canonical-event field targets. Retain,
remove, typed mask, and controlled replacement are closed operators; typed
masking is limited to content-bearing fields. Assignment artifacts bind the
ontology, trajectory, event kind, field path, exact field-value digest,
classification method, rationale, and declared pre-execution freeze stage.
Unknown, absent, duplicated, multiply assigned, factor-incompatible, and
operator-incompatible targets fail closed. The assignment digest is a
commitment, not proof of when an actor created it.

`evalwitness.reliance-estimand-catalog.v1` keeps `evidence_only` and
`quality_changing` in separate denominators and result tables. The former
requires preserved executable quality and permits only bounded claims about a
frozen verifier's observable output under an admissible intervention. Neither
family permits agent-step responsibility, environment causality, model-
internal explanation, universal faithfulness, human-reasoning equivalence, or
cross-family pooling.

`evalwitness.reliance-preregistration.v1` binds the locked study schema, all
ten main effects, four sparse interactions, seven distribution or transition
outcomes, balanced within-source-task assignment, retained failure states, no
imputation, one fixed look, and no replay-to-live fallback. Presentation order
appears only in its declared interaction. Bonferroni covers the exact 98-term-
outcome family. This object is a design component for a later locked study; it
is not live authorization or an empirical timestamp.

The deterministic reference adapter uses the audited 64-row Walsh design for
each of 16 synthetic source tasks, producing 1,024 cells with zero provider
calls and no network requirement. Its eleven main columns are clear of every
two-factor term, and each of the four declared interactions is the only pair on
its column. Its score outputs contain Top-20 alternatives, missing
probability mass, planted conditional-score and valid-mass effects, two null
factors, cluster heterogeneity, interactions, decision flips, and abstention
transitions. Every output is passed through the production
`ExtractScoreEvidence` and strict evidence validator rather than constructing a
score-evidence object by hand. `AnalyzeReferenceCorpus` fits all seven frozen
outcomes with the shared source-task-cluster sandwich estimator and the complete
98-hypothesis correction. Tests recover every planted main and interaction
effect, retain the nulls, contain the planted effects in the adjusted intervals,
reproduce exact output evidence, classify Top-k truncation, reject non-finite or
out-of-domain targets, and reject corpus or analysis tampering even after a
digest is recomputed. The shared cluster covariance accumulation is ordered, so
repeated fits are bit-deterministic.

The same package now exposes `evalwitness.reliance-preflight.v1`. It binds the
alias proof, ascending 8/12/16/20/24 source-task search, complete assumptions,
Monte Carlo error, Wilson power floor, grid MDE, selected scenarios, exact hard
resource dimensions, caller-supplied code digest, and a deterministic artifact
digest. Revalidation reconstructs the complete frozen search. This artifact
selects a design under declared synthetic assumptions; it neither authorizes a
provider call nor upgrades those assumptions to empirical evidence.

`ApplyEvidenceIntervention` now executes all 89 allowed operator-target
combinations through canonical events. It validates the frozen ontology,
estimand catalog, assignment set, typed replacement value, field presence, and
operator compatibility before mutation. Changed events are rebuilt with
`preprocess.RebuildDerivedEvent`; reference links are remapped; the complete
child is sealed with `preprocess.DeriveTrajectory`; exact before/after field
states, changed event and field sets, task identity, untouched fields and
events, outcome preservation, and the artifact digest are independently
reproduced during validation. Non-retain interventions containing event
references fail with `dependency_closure_required` until a later relation-backed
closure proof exists.

The intervention artifact is `evalwitness.evidence-intervention.v1`. It binds
the ontology, estimand, assignment, factor, operator, parent and child
trajectories, exact targets, optional production `outcome.Preservation`, typed
admissibility reasons, denominator eligibility, and a zero-provider,
zero-network execution boundary. Evidence-only cases enter the denominator only
when both decisive outcome records preserve task and executable outcome.
Missing or nondecisive outcome evidence remains unresolved; a changed outcome
is inadmissible. Quality-changing cases with a preserved outcome are
inadmissible; changed-outcome cases remain unresolved with
`relation_admission_required`. The assignment field records its declared pre-
execution freeze stage; it is not wall-clock proof that the assignment preceded
verifier output.

The frozen multi-factor execution path closes the gap between that single-factor
artifact and the selected Walsh design. `evalwitness.factor-treatment-plan.v1`
requires one non-retain treatment for every frozen factor and exact coverage of
all assigned targets. `evalwitness.evidence-intervention-cell.v1` consumes all
eleven coded levels, applies the ten negative-level evidence treatments
atomically to disjoint fields, rebuilds each changed event once, derives one
combined child, and reproduces its outcome and denominator status. An
all-positive cell is an exact source control.

Presentation order remains a separate render-only intervention.
`evalwitness.presentation-order-plan.v1` derives deterministic narrative-first
and narrative-last topological orders and validates them through
`preprocess.RenderTrajectoryInOrder`. It never mutates the canonical graph.
Absent narrative evidence, dependency cycles, and lack of a second valid order
are explicit unsupported states.

The shared `stress.ReplayFirstRunner` now exposes
`evalwitness.stress-replay-batch-evidence.v1`. It executes one labeled offline
batch through the production verification service, snapshots every requested
input, rejects control-plane substitution, retains full score observations and
stage traces, and binds all items to one validated exact-capture source. The
existing pair and controlled-relation runners are two-item views over the same
engine, so the project still has one replay implementation.

`RunEvidenceTaskPanel` seals
`evalwitness.evidence-intervention-task-panel.v1` for one source task. It accepts
exactly the 64 frozen Walsh rows plus one baseline, requires one absolute
criterion and one fixed repetition, validates every cell and dependency-valid
presentation before replay, and executes all 65 inputs in one shared batch.
This preserves the preflight resource model of one baseline call per source task
and one call per Walsh cell. The deterministic panel artifact retains full
baseline and intervention `ScoreEvidence`, every production
`CompareScoreEvidence` contrast, decision and abstention transitions, frozen
levels, input and plan fingerprints, observation-set and stage-trace digests,
batch identity, and capture provenance. Elapsed wall time is not part of the
panel artifact. Exact-replay tests prove 65-call accounting, repeated artifact
identity, incomplete-panel and input-substitution rejection, and rejection of
digest-recomputed result tampering. These are local execution-mechanism proofs,
not live or empirical reliance measurements.

The production analysis boundary begins with
`evalwitness.reliance-panel-registration.v1`. It binds the resolved preflight,
preregistration, study manifest, and exact 24 sorted source-task registrations.
Each task registration binds the source trajectory, assignment set, treatment
plan, and outcome-evidence-set digest. Reusing any one exact artifact digest
under two task clusters is rejected, preventing byte-identical aliases from
manufacturing cluster count; digest uniqueness does not prove semantic or
provenance independence. The artifact freezes 64 cells per task, 1,536
registered cells, 1,560 planned logical calls, and one non-poolable analysis
arm. The arm fixes entrypoint, criterion, score tag, evidence policy, provider,
route, and requested model. Registration sealing itself is offline. Its
`before_verifier_output` freeze stage carries
`declared_not_timestamp_proven`, so the artifact is a commitment but not proof
of chronology until a later capsule or equivalent external evidence establishes
ordering.

Every registered cell must then have exactly one evidence path. One validated
task panel supplies all 64 cells for its source task. Otherwise each uncovered
cell requires `evalwitness.reliance-cell-failure-receipt.v1`, including its
exact Walsh row, one preregistered failure status, evidence schema version,
evidence digest, and cell-attributed logical-call count. The failure receipt
binds accounting to evidence; it does not independently establish the semantic
truth of that evidence.

`evalwitness.reliance-analysis-corpus.v1` preserves all 1,536 registered cells
and canonical status counts. It rejects omissions, duplicates, source-task
substitution, mixed arms, changed Walsh levels, and unbound statuses. Measured
and abstained cells carry all seven preregistered distribution or transition
outcomes derived from the complete `CompareScoreEvidence` record and decision
states. Missing-score and execution-failure cells carry no fabricated outcome.
They remain in denominator reporting and are excluded from fitting without
imputation.

`evalwitness.evidence-reliance-analysis.v1` fits all ten main effects and four
interactions separately for every primary outcome through the shared
source-task-cluster sandwich estimator. Every fit retains the full 1,536-cell
denominator, eligible and excluded counts, and Bonferroni family size 98. Valid
rank- or cluster-deficient data produces a typed `inconclusive` result with no
fit. Unexpected estimator errors fail construction. Tests cover complete
24-task analysis, a fully failed 64-cell task without denominator loss,
one-cluster and zero-measurement inconclusive states, foreign-model pooling,
missing and duplicate cells, and resealed artifact tampering. This is still
deterministic local mechanism evidence, not an empirical provider result.

`evalwitness.evidence-selector-audit.v1` reconstructs that frozen analysis and
maps all 240 assigned factor targets across the exact 24 source artifacts to
production selector behavior at 16,384, 32,768, and 65,536 tokens. It calls
`preprocess.ApplyEvidenceBudget`, inventories the canonical event-score policy,
and checks whether each retained target actually changes the
`preprocess.RenderTrajectory` evidence surface consumed by verification. Every
target is classified once as rendered-exact, rendered-changed, unrendered inside
a retained event, or event-dropped. Raw canonical-event bytes and rendered
assigned-event bytes are separate, non-additive diagnostics; event retention is
never treated as proof of field visibility.

Factor status uses the frozen multiplicity-adjusted analysis. Any declared term
containing the factor with adjusted `p <= 0.05` on any preregistered outcome is
`adjusted_effect_detected`; `no_adjusted_effect_detected` is explicitly not a
zero-effect or equivalence claim. Risk flags expose detected effects with
changed, unrendered, or dropped targets, visible retained targets without a
detected effect, and inconclusive analyses that support no alignment claim. The
audit never tunes the selector from analyzed outcomes. It separately freezes
legacy `Pipeline/EvidenceSlice` and `evidenceLineScore` as a six-probe digest-
only inventory labeled outside the production verifier path. Reference fixtures
demonstrate that a retained evaluation event can still omit its assigned
explanation from rendered verifier evidence. This is deterministic mechanism
evidence, not an empirical external-model selector map.

`evalwitness.reliance-arm-comparison.v1` reconstructs each arm from its exact
registration, panel executions, failures, preregistration, and preflight. A
prespecified contrast is fit only when its actual changed dimensions exactly
match one closed kind: evidence policy, entrypoint, route, provider plus route,
or model family plus requested model plus route. Duplicate contrast identities,
no-change comparisons, and mixed changes are rejected or emitted as typed
unsupported results with no fitted outcome.

Evidence-policy contrasts require the same `provider.ExactReplaySource` for
every source task with panels in both arms. Thus explicit judge and strict
verifier evidence cannot be presented as paired when their response-record
captures differ. Jointly missing or one-sided task panels remain explicit
denominator states. Every eligible pair also has the same source task, Walsh
row, intervention-cell digest, and presentation digest. The comparison fits
comparator-minus-reference differences for all seven outcomes using the frozen
14-term source-task-cluster model and Bonferroni family size `98*K`, where `K`
includes every prespecified contrast even when unsupported.

Model-family contrasts require both arms to declare
`named_family_evidence_bound`; `alias_only` is unsupported. The arm artifact
binds a route-attestation digest but does not validate the referenced
attestation contents or independently prove the family label. Transfer-study
and route-attestation admission remain mandatory before a public named-family claim. Deterministic
tests cover verifier-versus-explicit-judge extraction from the exact chosen
response score, route, provider, named-family, and entrypoint contrasts,
confounding refusal, retained missing pairs, duplicate specs, response-capture
mismatch, presentation mismatch, and digest-recomputed result tampering. This
is local analysis-mechanism evidence with zero provider calls and no network
requirement, not an empirical cross-arm result.

`evalwitness.reliance-witness.v1` reuses
`stress.ReduceCounterexample` through a witness oracle adapter. It first
revalidates the exact relation-backed execution and derives the original
semantic reliance-result digest from relation, case, task group, outcome,
invalid state, complete constraint observations, distribution comparisons, and
planned/completed repetitions. Full replay and stress-result digests continue
to bind stage provenance, admission, and execution accounting outside that
semantic projection.

Each reducer evaluation binds its exact input digest, a candidate-specific
replay-batch fingerprint and replay-result digest, the unchanged exact-capture
digest, one intervention-validity proof digest and status, the frozen outcome-
evidence identity, the semantic reliance-result digest, and the complete stress
reduction observation. A changed candidate cannot reuse the source batch or
replay identity. `preserved` is derived only when relation, privacy,
intervention, outcome identity, and reliance-result identity all survive;
otherwise the receipt is `unresolved` with closed reasons. The stress
`violation_preserved` value must agree with this derivation. Accepted removals
must be preserved, rejected removals unresolved, and the final retained input
must reference the last accepted preserved receipt.

The witness embeds the complete one-minimal counterexample and all evaluation
receipts. It claims one-minimality only over the declared reduction units, not a
global minimum. Proof digests emitted by the oracle are exact evidence pointers,
not independently recreated private proof content; later reference-capsule
admission owns that verification and public-release boundary. Deterministic
tests reduce three evidence units to the required executable-evidence unit and
reject digest-recomputed changes to intervention validity, outcome identity,
reliance-result identity, replay batch, candidate replay reuse, and oracle
consistency. This is local exact-replay mechanism evidence, not an empirical
explanation of an external model.

`BindRelationBackedIntervention` produces
`evalwitness.relation-backed-intervention-admission.v1` from one already
validated controlled-relation `stress.ConstructAdmission`, its exact V3 replay, and the
sealed intervention. The bridge requires one trajectory-level mutation
relation, exact parent identity, and byte-equivalent canonical events, links,
and ingestion accounting between the intervention child and replayed transform;
different derivation metadata is retained under each owning protocol. It binds
the relation, manifest, formal witness, construct firewall, owner attestation,
terminal ledger, human resolution, assignment, intervention, and both event-
graph digests. Revalidation reconstructs the complete object from those frozen
parents.

Human-supported construct status can admit an otherwise complete evidence-only
case or resolve only the specific quality-changing
`relation_admission_required` state. Formal-only and human-unresolved cases
remain typed sensitivity strata; human contradiction is inadmissible. Missing
or nondecisive outcome evidence remains unresolved even under human support.
Primary-core relations reject formal-only admission. No verifier result is an
input, and the bridge cannot change factor assignment or intervention bytes.
This layer does not yet reconstruct the owner attestation from the withheld
private chain; `relation.BuildOwnerInspectionCustodyGate` owns that separate
operation. It replays the production private-chain verifier, requires all 66
assessments and all 16 aggregate dimensions to pass, pins the package, session,
inspection, completion, public-attestation, and claim-boundary identities, and
emits a private, content-addressed, zero-provider, zero-network gate. It always
records `formal_human_ledger_present=false`,
`primary_admission_authorized=false`, and `execution_authorized=false`.

The tracked public owner attestation remains `revision_required`, so it cannot
produce a passed custody gate. The positive gate test constructs an all-passed
private chain through the real production journal, completion, projection, and
verification functions; it is mechanism conformance, not evidence that the
tracked owner inspection passed. Tests reject the tracked revision state,
resealed status promotion over private evidence, formal-human claim promotion,
private-chain drift, and a digest-recomputed attempt to grant primary admission.
Because the gate contains private session, inspection, and completion
identities, it is private execution evidence and must not be projected into a
public artifact.

This is E1 local estimator and failure-boundary evidence only. It establishes no
real verifier reliance, provider behavior, human validity, causal responsibility,
transfer, or population result.

## Determinism and Replay

Every provider attempt uses canonical request schema 2. Its portable bytes have fixed field order, UTF-8 JSON encoding, sorted map keys, explicit nulls, normalized empty collections, and lossless float64 bit encoding after negative-zero normalization. The SHA-256 fingerprint covers provider ID, canonical origin and endpoint path, requested model, thinking mode, ordered messages, temperature, seed, output cap, logprob controls, stops, score tags, response format, streaming, prompt-builder contract, logit bias, and ordered evidence bindings. It excludes credentials, timestamps, retry state, local paths, callbacks, and typed lineage. Sampling slot is a separate lookup dimension.

Response capture schema 3 and parser contract `evalwitness.score-token-parser.v2` bind that fingerprint and route to normalized response bytes, separate body and parsed-payload digests, an evidence digest, requested and served model identity, provider request ID, finish state, usage, capability attestation reference, ordered chosen-token and alternative logprobs, audit lineage, and deterministically derived score evidence. Replay status is typed as `live`, `exact`, `legacy`, `miss`, or `rejected` with a reason.

Research capture admits response `served_model` values only under the locked
`exact_observed` or `exact_observed_set` contract. Set membership is checked on
every live response before attestation stamping and digest finalization; the
actual returned value is never normalized to the requested alias. Capture
inspection therefore retains served-model strata while an unknown value aborts
the run without publishing it into the target capture.

Public response bundles use the capsule kernel rather than copying a cache
directory. `evalwitness.response-bundle-policy.v1` seals one study/cell, a
deterministic partitioned request-corpus digest, dataset and response licenses,
the exact producing source tree, binary and Go toolchain, allowed capture names,
five fixed omission classes, an explicit lineage policy, zero provider calls, and
zero network requirement. `exact_fixture` is restricted to synthetic mechanism
conformance. `complete_research` requires every capture record to pass complete
research-lineage validation and to bind the one study-cell identity sealed by the
policy. It also requires a clean committed Git source tree and a binary whose
embedded VCS revision and unmodified state match that tree. Policy sealing, build,
capsule binding validation, and verification all enforce that distinction.
Complete research additionally requires SHA-256 source-trace, trace-map, and
policy identities, at least one exact evidence binding, an observed served model,
and an `att-<sha256>` capability-attestation identity on every response record.
The declared redistribution evidence is read as exact bytes, digest-checked,
public-scanned, and embedded as `evalwitness.redistribution-evidence.v1`.
Canonical newline-terminated schema-3 capture bytes are embedded as
`evalwitness.response-capture.v1`; `evalwitness.response-bundle-index.v1`
derives per-capture request, lineage, body, score-evidence, record, payload, byte,
route, model, and mode commitments. Verification reloads the capsule and public
scan, validates the policy/index graph, and returns only verified payload paths
for exact replay. Request-fingerprint plus sampling-slot identity is unique across
the complete bundle, so splitting one observation across two capture names cannot
inflate a denominator. Policy, capture, source tree, binary, evidence, or archive
drift fails closed.

The canonical verification report exposes the verified study, cell, policy,
index, capture-set, request-corpus, producer, and redistribution identities plus
each capture's complete route/mode/model/attestation and request/lineage/body/
evidence/record inspection. A reviewer does not need to trust a collapsed
`valid=true` summary to see which exact evidence was accepted. Its evidence
ceiling is `mechanism_conformance` for `exact_fixture` and
`record_complete_external_parents_unresolved` for `complete_research`. The latter
means the response records are internally complete but their study plan, source
evidence, route attestation, capture-run attestation, authorization, and analysis
must still resolve as verified parents in the outer study capsule before any
empirical release claim is admissible.

The v5 empirical outer capsule resolves those parents in
`evalwitness.identical-response-050-bind.v2` and raises the scoped ceiling to
`record_complete_external_parents_resolved`; this remains a bounded development
population result and is not a correctness, quality, transfer, independent-
replication, or release claim. Its decimal-bearing study and analysis child
payloads use exact-byte identity with strict schema decoding because the
repository's canonical JSON profile accepts integer JSON numbers only; the
capsule manifest, registry, and claim ledger remain canonical and
content-addressed.

`evalwitness replay capture-run attest|verify|stamp|admit` and
`evalwitness replay study bind` seal capture-run attestation, research-lineage
admission, and parent bind certificates. The v5 capture is complete and
admitted; `replay study capsule build|verify` now resolves it together with
the exact live-authorization plan into the outer study capsule `2ba8ac46` with
`evidence_ceiling=record_complete_external_parents_resolved`, a sealed
CLM-070..CLM-074 ledger, and an executable 34-receipt challenge pack. The
outer family, ledger, and challenge pack verify offline with zero provider
calls. Production `verification.Result` carries `calibration_policy.status=
unsupported_no_held_out_policy` and `fallback.kind=none` with `charged=false`.
`ChargeFallback` reserves explicit judge or human-handoff calls on
`mode.RunBudget` only when a `SelectiveDecision` abstains with a validated
costed policy. There is no locked deployable policy.

`evalwitness calibration evaluate` and
`evalwitness registry validate-intake` / `render-matrix` / `refresh` are local
library/CLI surfaces only. Registry intake status stays `format_verified`.
Dispute, correction, and withdrawal are append-only governance actions and
cannot rewrite signed history. `registry index-scarcity` pins the committed
public scarcity digest and the six package-format-v5 parent digests. Registry
CLI reads reject archive bytes and payloads over 1 MiB. The committed seed catalog
`eval/governance/registry-seed-catalog-v1.json` is two contrasting
`format_verified` development entries; it has no community validation and does
not admit a reference or reliability capsule. MCP exposes
`evalwitness_calibration_evaluate` as the same offline IsDeployable check, not
a production policy.

`scripts/tests/run-response-bundle.sh` proves the contract with the synthetic
golden capture: two independently built archives must be byte-identical, an
unguarded loopback connection must succeed and the identical guarded connection
must fail, replay runs with isolated empty EvalWitness home, configuration, and
response cache while the build reuses provisioned host Go module/build caches,
schema 1 and corrupted payloads are rejected, the MIT license bytes are embedded,
and the report must state zero provider calls. The full reproduction driver is
`scripts/evals/reproduce-public-evidence.sh --profile full`; it runs that gate
and the complete claimcheck under the same hard network boundary. This is
local mechanism conformance under `lineage_policy="exact_fixture"`, not a
dependency-empty clean-clone, historical, or empirical response bundle. The live study
requires `lineage_policy="complete_research"`; the dedicated
`scripts/evals/reproduce-identical-response-v5.sh` gate proves the declared v5
asset graph from a clean clone, while explicit publication authorization governs signed release and remote
publication proof.

`EVALWITNESS_REPLAY_TO=path` stages complete request/response records beside the destination and publishes only after checked encode, flush, file sync, close, full checksum reload, concurrent-target check, atomic rename, and directory sync. Any finalization error fails the run and retains the candidate for inspection. `EVALWITNESS_REPLAY_FROM=path` requires exact schema, route, request fingerprint, sampling slot, response checksums, requested tags, and logprob evidence; it never falls back live.

The shipped schema-3 exact fixture is `scripts/tests/golden-delta-replay.jsonl`. `scripts/tests/golden-delta-replay.schema2.jsonl` preserves its immediate predecessor and `scripts/tests/golden-delta-replay.legacy.jsonl` preserves schema 1. Golden regeneration writes only a new `.candidate` and refuses to overwrite it. `evalwitness replay migrate --source old.jsonl [--candidate inspection.jsonl]` preserves the source and emits only a `legacy` inspection candidate listing fields such as missing score evidence that cannot be reconstructed, never exact evidence. `internal/provider/testdata/request-fingerprint-v2.json` is the stable portable vector corpus, independently checked by `eval/python-reference/verify_request_fingerprints.py`.

## Cache and Preprocessing

Disk cache at `${EVALWITNESS_CACHE_DIR}/routes/route-<sha256>/responses/<sha256-prefix>/<sha256-full>.json`; the capability record for the same storage namespace is `capabilities.json`, and `identity.json` stores its exact provider/model mapping. The storage namespace hashes length-prefixed provider and requested-model identifiers, while each response record separately binds the canonical origin and endpoint path in its full route ID and request fingerprint. Raw external identifiers never enter a path and field boundaries cannot collide. The cache key is `(request_fingerprint, sampling_slot)` and schema-3 entries retain those identities, the checksum-bound response record, and derived score evidence. Reps are independent sampling slots, not cache replays. Exact reads revalidate fingerprint, full route, body, parsed payload, evidence digest, and byte-equivalent score-evidence re-extraction. Verifier and judge request identities never cross-hit. Legacy schema is an explicit compatibility read and never silently becomes exact evidence. Cache storage and released replay fixtures remain separate, so cache eviction cannot delete fixtures. Every new root carries `.evalwitness-cache-root.json` with a random 128-bit root ID; sensitive roots/directories/files are owner-only. Writes use unique same-directory candidates, file sync, checked close, atomic rename, directory sync, and per-writer cleanup. Disable per-run with `--no-cache` or globally via `EVALWITNESS_NO_CACHE=true`.

The sealed read-only legacy census covers 17,898 files and 71,103,852 bytes:
17,876 schema-1 response files and 71,094,038 response bytes, seven capability
files and 2,147 bytes, and 15 operational files and 7,667 bytes. Operational
content was not read. The six provider namespace counts and separate scientific
and operational inventory digests are committed in
`eval/governance/legacy-response-custody-v1.json`; no response body, credential,
local path, or operational content is emitted. Exact-admissible entries are zero
because schema 1 lacks request, raw-response, route-attestation, parser/evidence,
binary, source-tree, dataset, analysis, and collection-clock identities. The
`opencode-go-cn` namespace is only a 6,306-entry upper bound for eleven artifacts
that report 6,599 cache-hit calls, not an inferred one-to-one source map.

Destructive cache operations resolve existing-parent symlinks and reject filesystem or volume roots, the user home, repository root, working directory, and any candidate that could contain a protected location. An ownership marker never overrides this protected-path policy. `evalwitness cache clear` has no implicit scope: `--scope responses`, `--scope capabilities`, or `--scope all` atomically renames only those known descendants before removal. The cache root, marker, `identity.json`, unknown files, and every read-only legacy root remain intact.

Archive intake is a two-pass operation. `evalwitness archive inspect --source file.tar.gz` reads every header and expanded byte without writing; `archive extract` repeats the same validation into an owner-only sibling staging directory, verifies the source digest did not change between passes, then atomically publishes a previously absent destination. Only regular files and directories are accepted. Absolute, traversing, non-normalized, colliding, linked, device, FIFO, xattr-bearing, oversized, over-deep, high-ratio, and unreservable inputs fail before publication.

Trajectory preprocessing is one canonical, loss-accounted path. Content-based detection accepts Claude Code JSONL, Codex rollout JSONL, OpenCode export JSON, Terminal-Bench trajectory JSON, SWE-bench cache-item JSON, and plain text. A bounded reader rejects sources over 256 MiB and JSONL records over 16 MiB before decoding. Strict research mode rejects an unknown structured record; compatibility mode keeps it as an explicit `unsupported` accounting entry.

Canonicalization is explicitly profile-locked. `evalwitness.canonicalization.v2` is the default for all new ingestion and supplies the strict verification-lineage contract: one native command syntax surface, a canonical operand digest, native call-identity rejection, observed Codex exit status, and separated stdout/stderr. `evalwitness.canonicalization.v1` is available only through `FrozenCanonicalizationV1IngestOptions` to reproduce already sealed mutation, relation, and outcome artifacts with their original display-only command and flattened Codex result shape. Unknown profiles fail before source reading; partially populated V2 command shapes fail validation. The V1 compatibility path is pinned by an old-implementation trajectory digest and by the complete 320-case controlled-corruption release digest `6f60fc2ceac52fa9efbf4f5b39dc132ae62e849987cb08b5753f09941a8b020a`, so new lineage fidelity cannot silently rewrite prior research evidence.

`evalwitness.trajectory.v1` contains ordered typed events for messages, tool calls and results, commands, output, file changes, errors, attachments, metadata, and reasoning-presence markers. Each event carries stable source coordinates, source ID when available, sensitivity, byte/token counts, content digest, and exactly one typed payload. Typed parent, call/result, file-change, and derivation edges form an acyclic graph. Duplicate identities, missing endpoints, duplicate links, cycles, or unaccounted records fail validation. Hidden reasoning content is always omitted; only its existence, encrypted state, byte count, and omission reason are retained.

The ingestion report classifies every record and field as represented, metadata-only, omitted-sensitive, unsupported, redacted, truncated, or rejected. It records source/canonical counts, bytes, redactions, unsupported records, unpaired tools, per-category retention, provider-reported token observations, and exact selection decisions. Sanitization runs before any evidence-selection score. Plain text keeps byte-identical rendered content when it fits; structured sources render deterministic typed event blocks plus a compact accounting header.

Evidence selection first renders and measures the complete canonical trajectory. A zero budget is unbounded; a fitting trajectory bypasses slicing byte-for-byte. Oversized trajectories are grouped by validated call/result links, ranked deterministically, restored to source order, and admitted only if the complete rendered unit fits. If no whole unit fits, one highest-value event may be UTF-8-safe field-truncated under the hard cap. Every dropped or partial event, category, link constraint, boundary, parent digest, and retained digest is recorded. No path may exceed the declared token budget.

`evalwitness fidelity --source PATH|- --budgets 16384,32768,65536` and `scripts/audits/run-transcript-fidelity.sh` expose the offline report. Provider usage embedded by Claude Code, Codex, or OpenCode is retained component-wise and compared with the local bytes-per-four estimate. This is a diagnostic difference between a rendered source and provider-reported session usage, not tokenizer parity. Public golden fixtures and their count/digest manifest live under `internal/preprocess/testdata/golden`; private source content and source digests remain private unless the owner explicitly publishes them.

Derived trajectories use `DeriveTrajectory` with immutable source digest, parent trajectory digest, relation, validator, changed event IDs, and event-addressed field paths. Trace mappings, controlled transformations, and evidence-reliance interventions must consume this event model and emit field-level loss or derivation records rather than create another downstream transcript shape.

## Trace Interoperability

EvalWitness is a trace bridge, not a trace standard. Import terminates at `evalwitness.trajectory.v1`; every bridge emits `evalwitness.trace-envelope.v1` and `evalwitness.trace-mapping-report.v1` under mapping policy `evalwitness.trace-mapping.v1`. The envelope binds source, source digest, capture interval, agent identity, privacy class, canonical trajectory digest, mapping-report digest, and its own digest. The report classifies mapper-observed semantic fields as `exact`, `normalized`, `synthesized`, `redacted`, `unsupported`, `ambiguous`, or `dropped`. `lossless=true` is allowed only when every observed field is exact.

| external contract | exact supported input/output | retained semantics | explicit boundary |
|---|---|---|---|
| OpenTelemetry OTLP/JSON | OTLP JSON trace data `1.8.0` with schema URL `https://opentelemetry.io/schemas/1.41.0` and GenAI semantic conventions `1.41.0` | trace/span identity, causal parents and links, operation, provider/model, tool calls/results, usage, errors, status, time, and authorized content | no receiver, collector, exporter daemon, conversation-ID synthesis, or acceptance of unversioned `latest`; GenAI evaluation is an OTel Event carried by a Logs `LogRecord.event_name`, so evaluation-log import/export is not implemented and is never substituted with a span event |
| Agent Trace | record version `0.1.0`, fixture provenance pinned to upstream commit `2754f077f3e50c1fb5088183f5c9362077cc8ca1` | contributor, conversation, repository-relative path, line range, content hash, and VCS revision | attribution is not quality evaluation; verifier evidence is only an optional `org.evalwitness.verifier_evidence.v1` digest reference in metadata |
| Vendor and benchmark exports | existing Claude Code, Codex, OpenCode, Terminal-Bench, SWE-bench, and plain-text adapters | source-native evidence represented by the canonical adapter | no claim of cross-vendor semantic losslessness without the emitted mapping report |
| Jaeger query/UI JSON | rejected | none | Jaeger does not specify the query/UI response as a stable interchange contract; operators must provide OTLP/JSON |

The OpenTelemetry adapter pins the last immutable core semantic-conventions schema used by this implementation. The newer dedicated GenAI semantic-conventions repository was reviewed at commit `46d43c8949afb53765a202e89f4534eeb75ca3fa`; it currently exposes no stable Schema URL, so EvalWitness does not label it supported. A version upgrade requires an immutable schema identity, licensed fixtures, a mapping diff, and the complete interoperability gate.

OTLP fields outside the exact pinned Protobuf JSON shape fail rather than disappear. Upstream dropped-field counters become `dropped` evidence. Missing usage attributes produce no usage observation and never become observed zero; present token counts must be non-negative integers and survive semantic round trip. Non-causal OTLP reference links may form cycles, while parent and derivation cycles fail.

Privacy is capability based. `metadata_only` is the default and replaces content and attribution with digests or aliases; `content_authorized` retains prompt, response, tool, and error content; `attribution_authorized` retains repository paths and conversation URLs; `content_and_attribution_authorized` enables both. Import never authorizes release. OTLP export remains metadata-first unless content is explicitly authorized, and Agent Trace export fails unless attribution is authorized.

Canonical request schema 2 fingerprints ordered evidence bindings for every input slot: source, canonical trajectory, ingestion report, trace envelope, mapping report, and mapping-policy identities. Protocol `1.2.0` `TrajectoryRef` objects carry the same trace provenance. This prevents two lossy mappings of the same displayed text from sharing verifier or audit identity.

`evalwitness trace inspect` and `evalwitness trace export` are provider-free file operations. `scripts/audits/run-trace-interoperability.sh` builds a fresh binary, imports the licensed pinned fixtures, asserts metadata-first redaction and causal hierarchy, performs a semantic OTLP round trip, validates fixture provenance, and requires no network or provider configuration. Semantic round-trip equality is deliberately distinct from byte equality.

## Verification Lineage

Verification lineage adds the research boundary for measuring whether claim-bearing
verification evidence survives from an execution witness through a native agent
export, the canonical event graph, evidence retention, and the verifier request.
The static protocol registry at
`eval/governance/trace-source-specifications-v1.json` pins six upstream source
contracts without becoming an eleventh lineage document. It distinguishes
versioned specifications, commit-pinned implementation contracts, unversioned
documentation, and development-only material. `unspecified` is a first-class
representability state: lack of a stable format contract is never rewritten as
`optional` or `unsupported`. Claude Code therefore remains entirely unspecified
at format level; synthetic adapter vectors and observed captures remain separate
axes and cannot upgrade that upstream contract.

The shipped root artifact is the sealed
`evalwitness.verification-lineage-plan.v1` plan at
`eval/governance/verification-lineage-plan-v1.json`. Its content digest is
`5f56a357721a4ec2b650660a4efb7b4d9b67d342ada388a5d28f6e607fd9189c`
and its complete-file SHA-256 is
`e0765e67a28f96f68cc427df6a373504188c59d16e5f519776fd8cc6213f81cc`.
The closed ten-object parent DAG and the digest of every generated JSON Schema
body are emitted from code and committed as
`eval/governance/verification-lineage-schema-inventory-v1.json`. Its canonical
content digest is
`4ffb32aa9efa9bb9fc8cffaa7d9416a95c558acd3fe6e2feaca22bfb92ccdfc5`
and its complete-file SHA-256 is
`08613396c861a83c8fa8b30d8533ae275b972f3d0e84898de2120bd1a4db5359`.
The source registry content digest is
`4c955b5dc3eccf672c9befc0de7506d3079ba6d9a4a60af994cc535d8c720d44`;
its complete-file SHA-256 is
`4bf95b9ae576dbb0ee2759ad272bfa23c1cebd30c94de5972c2ba944a9e6c59a`.
The pre-acquisition source inventory content digest is
`d1d60e91a0301867b72ee821d4146a1e1d05b310a28aad8f30bcbd33362e3b39`;
its complete-file SHA-256 is
`c0eac7c5a2bcf9226c32a61eb8a543be05179426598f017d9885e8b8e717098b`.

The provider-free native-format golden set contains 63 vectors: the same 21
semantic cases projected across Claude Code JSONL, Codex rollout JSONL, and
OpenCode export JSON. It covers direct test/check/build/outcome probes, printed
and searched names, wrappers, compound commands, missing/duplicate/out-of-order
identity, truncation, redaction, unsupported syntax, both sealed controlled-relation false
targets, structured success after text omission, distinct-operand comparison,
and exit status. `representable_by_format` remains separate from the observed
adapter result; Claude Code capability stays `unspecified`, OpenCode
out-of-order records and non-Codex exit-status vectors are `not_applicable`, and
no single synthetic capture establishes a format-wide claim. Strict mode now
rejects the eight representable missing-ID, duplicate-ID, and out-of-order
negative controls. Codex retains an observed exit status plus separated stdout
and stderr; an absent exit field remains unobserved instead of becoming zero.
Native-canonical and retained trajectory identities, event counts, and
call/result/link counts are separate, so a budget loss cannot be misreported as
an import loss. Its content digest is
`eb36d591993ee332cba89a95f754a75b67ff45baf608e9ce3e7bf212bdee0b92`;
its complete-file SHA-256 is
`85f450d4adb927fba91c2f822c535c260643fce3026ef2d5ffc20d4ed3cb3602`.

The adapter-conformance report executes ten normative checks for every admitted
vector and one boundary check for every expected rejection or not-applicable
vector. Across 63 vectors it records 504 checks and all 504 pass. The status
surface is 49 `conformant`, zero `known_mapping_gap`, 11
`expected_rejection`, and three `not_applicable`.
Each admitted case independently verifies raw-record conservation, field-
mapping conservation, repeated-import identity, canonical JSON round-trip,
native call/result linkage, command display, redaction, retention, exit status,
and native identity integrity. Content digest:
`7f93360c022843aeb351afa57c52b96077a0a0c7260f3f1ce62b9c58949f58eb`;
complete-file SHA-256:
`4cd6e9968466c7be22e8442ac5c360758e87b892b6269d2959b65f2304d481b3`.

The pre-calibration parser and mapping lock is committed as
`eval/governance/verification-lineage-parser-lock-v1.json`. It binds exact
SHA-256 identities for 24 production parser, mapping, pairing, classification,
failability, retention, and BOM source files plus six independently reproduced
governance artifacts. The lock reruns 63 development vectors and 504 normative
checks with zero failures before sealing eight semantic surfaces across Claude
Code, Codex, and OpenCode. Calibration results remain uninspected and provider
calls remain zero. Any bound change before calibration requires a new lock
digest; after calibration it requires protocol v2 rather than retroactive repair.
Content digest:
`f97a31986d42613005fe7e6753b53f6457460bd5caf0a130899d7e4e1bc2c686`;
complete-file SHA-256:
`fb4f0586483a9b616d2eebf8a75fcbad559716c133ebf34c060019b3593b529e`.

The inventory admits the checked-in licensed synthetic Agent Trace and OTLP
fixtures only to `adapter_development`. Historical vendor goldens remain usable
for adapter regression, but are rejected from research denominators because
their old conversion manifest does not prove per-source license, verification-lineage
consent, task lineage, near-duplicate isolation, or an authoritative execution
witness. Owner-capture and public-native source classes remain unmaterialized;
no task-group counts, live sessions, providers, or external sources are touched.

The closed pre-acquisition readiness result is
`eval/governance/verification-lineage-source-readiness-audit-v1.json`. It
recomputes five candidate classes against the sealed inventory and live parser
lock: three are materialized for adapter development, none is admitted to the
research corpus, none is calibration- or locked-test-eligible, and no empirical
task-group denominator exists. All five per-format rows are explicitly non-
inferential. Its state is
`not_runnable_no_admitted_paired_research_source`, content digest is
`8185f702136649383c0a89898800b079ce61eb9bb25843312e1f98813c8283c9`,
and complete-file SHA-256 is
`2b1089a1ca87caa3178434282f70f0734b03e7fb5c93d483a9715ca43bbe6965`.
This is acquisition-readiness evidence, not an empirical lineage audit.

The holdout-readiness result is
`eval/governance/verification-lineage-holdout-readiness-audit-v1.json`. The
frozen rank rule selects `claude_code_jsonl` with digest
`d5e4416f331aa9ea5e4804bdb75a329158722531be4ba4a470646cdb4bac071e`,
but every candidate format mapping was already used during development and no
held-out research unit exists. The syntax-family candidate universe was never
sealed before its development cases were used. Both v1 holdouts are therefore
recorded as not runnable, with zero predictions and zero labeled outcomes.
Content digest is
`d850e528ad93d24d0da231bd6d6a4a958e83be794091f2520de7338bba338c72`;
complete-file SHA-256 is
`e07c752bdec68f587f9a171306661fc617c791827060c54f69ddb59069c42efd`.

`eval/governance/verification-lineage-corpus-feasibility-v1.json` applies the
unchanged 20-calibration plus 20-locked-test threshold to those sealed readiness
artifacts. The current generation has 0/20 calibration and 0/20 locked-test
groups, an exact shortfall of 40, so its decision is
`not_feasible_current_generation`. It does not claim future impossibility.
Protocol v2 requires uncontaminated format isolation, a pre-outcome syntax-
family universe, unchanged 20/20 support, and all role/lineage firewalls.
Content digest is
`e0afb924db7e5599a286af2c2cf12c3a55b04141e3885e29dc226167c0529eea`;
complete-file SHA-256 is
`ccf3c7cb4e15642c2b7f2e9505bd7140723bcc055b6bacd031263e01ff41d1bf`.

The development capability matrix at
`eval/governance/verification-lineage-capability-matrix-v1.json` emits one
validated `TraceCapabilityVector` for each native format. Across 30 field rows,
the pinned specification axis contains 4 required, 14 optional, 1 unsupported,
and 11 unspecified states; the separately recomputed vector-observation axis
contains 26 observed and 4 not-observed states. Claude Code remains unspecified
at format level even where its synthetic adapter vectors are observed. Every
row binds normative evidence and all 21 format vectors, and the matrix forbids
losslessness, universal capability, prevalence, and provider-quality claims.
Content digest is
`ea97498518ca540a79fabc9358a34f982e0689d5824bd5cf45cc3e7723a0531c`;
complete-file SHA-256 is
`637ce1b38d226c5fbd33a48049c44b7d6be7e67475a57f15391189fd53d2a96a`.

`eval/governance/verification-lineage-offline-proof-v1.json` is the compact
verification path. The positive fixture traverses process witness, strict Codex
native export, canonical graph, retained bundle, request identity, assessment,
conserved audit, and accepted BOM. The paired counterexample is the sealed
same-path comparison and terminates as `non_failable_verification` with
`self_comparison`, preserving layer survival `[true,true,true,false,false]`.
The report binds all readiness and capability artifacts and explicitly remains
adapter-development evidence. A fresh measured run generated it in 4.71 seconds
with 18,677,760 maximum resident bytes; artifact size is 3,820 bytes. Content digest
is `8fb5d2611f0fcdd0f89f18eb884d21e0c464ea9fb9d1a8257d5da8ff80355ab9`;
complete-file SHA-256 is
`231212a3ed20ee8908adb2f8ed10a298212aa88dd449a05fd194d4d57fc2edcf`.

The public first-loss projection is
`eval/results/verification-lineage-same-path-loss-certificate-v1.json`. It is
strictly regenerated from the sealed counterexample, contains no restricted
content, identifies `retained_bundle` as `first_loss`, and marks the verifier
request `not_reached`. Its content digest is
`2719f6df00575fb598bf373099515a5d74b8f7bb99a5bc2cfd2f665c22ffd737`;
complete-file SHA-256 is
`c5527076cbe40b2c544a631a75e72d02ad818fde71e5e31015c17532aef70f73`.
`eval/results/verification-lineage-offline-graph-v1.json` is the only numeric
source for the two-path development visual. It binds the accepted BOM and loss
certificate, declares exactly two development fixtures, and fixes
`empirical=false` plus `provider_ranking=false`. Its content digest is
`5bce9529b9f3cb4eeffb2aa904f303bc2c1c5471aa59296186d6ec8b4aff0170`;
complete-file SHA-256 is
`b5a6a3c19a281c8e61d866685746af3aea8d233e9a9bbcafc3d0df82fb723871`.
The deterministic SVG at
`eval/results/verification-lineage-offline-graph-v1.svg` is generated from that
JSON and has complete-file SHA-256
`b3ea0800a2948cb730052d7673a0d580368cbe9b98155df8b77234a9602e03d2`.
It visibly labels the development-only, no-provider, non-empirical boundary.

The public development package adds four typed evidence objects and one manifest.
`verification-lineage-offline-audit-v1.json` conserves one included Codex
development unit through all four adjacent flows; content digest
`7fa00f1d874e41f62d92a8ab061807e3d1190a430ec7a1ffa6058ef8c7fa6e45`,
file SHA-256 `33a6a6069658204d9e9e1fa0d9f48fad9a144c7ee971406845a42e68d4b679c8`.
`verification-lineage-offline-bom-example-v1.json` binds the accepted claim,
ordered executable operands, five parent layers, required fields, decisive
channels, repository state, freshness, and request identities; content digest
`d3ee5d84d4217e59394a1d6bdb6ec5c1ca3f44e03d601e06e02475831a8acf8d`,
file SHA-256 `72e498095c4e100c7cfd4bd4c8c527f3c6de2c6b41347eba979715c29678847a`.
`verification-lineage-development-dataset-card-v1.json` records 63 development
vectors, 504 conformance checks, two development fixtures, one accepted BOM,
zero empirical task groups, and zero research-admitted sources; content digest
`46d0c138adc4799fce46d2012d9618151dbd07d49d8bff23309600fb46cd6213`,
file SHA-256 `2bc9303e76ad1ece43475098e9dea9aae46cccdbeb80a70a1f4ccad4a6c122ae`.
`verification-lineage-limitations-v1.json` carries seven unresolved or out-of-
scope evidence boundaries with a required resolution for each; content digest
`95b44986ca9b1e78f9f2cc7f803f2c46642fd960b2c53a5d60605c779d32d682`,
file SHA-256 `cbffb37dc3fa5199954f0b0a4c28ff561ba7dadaafc7686db03e7760b9ad6d39`.
The `1.0.0-development` release manifest binds 20 public files by path, role,
byte count, and SHA-256 and excludes restricted material. Its content digest is
`252c7a56c3a86b779019fd02e5b3bf24f65e88e0a89880179fb164f7aa4b5702`;
file SHA-256 is
`4f2dc869dffc168ddbd3683ef70115ce3176f468a1d3b001fa1f5a5d4096cdf7`.

`evalwitness trace lineage intake --source PATH|-` is the safe new-export
entrypoint. It always imports under `metadata_only`, binds the current capability
matrix and development conformance report, records exact source/canonical/
mapping identities and counts, and emits plans before lineage classification.
Development-vector conformance is never relabeled as conformance of the supplied
input. Without a separately supplied execution witness, authoritative capture
policy, repository state, and source manifest, intake returns
`independent_execution_witness_required`, `can_classify_terminal_state=false`,
and `can_build_bom=false`. It executes no captured command, provider, or agent.

The plan freezes RQ1 through RQ6, the task-group unit, lineage and near-duplicate
isolation, four data roles, ten mutually exclusive terminal states, eight
pre-denominator exclusions, no post-outcome replacement, hard stopping limits,
minimum format support, exact and clustered uncertainty, format and syntax
holdouts, and allowed versus forbidden claims. It binds the current trace-mapping
and verification-evidence contracts without manufacturing provider arms or dummy
study records.

Acquisition remains `not_started`, source counts were
`not_inspected_before_plan_lock`, external action is `not_authorized`, provider
calls allowed by the laboratory are zero, and the laboratory cannot launch
agents. The current commands emit all ten closed schemas, reproduce the schema
inventory, and strictly validate each document type. They do not claim that a
populated corpus, format holdout, capability matrix, or empirical audit exists.

`scripts/tests/run-claimcheck.sh` regenerates the plan, schema inventory, source
inventory, source registry, synthetic witness controls, and native-format
golden vectors, adapter conformance, and the parser lock from a fresh binary,
plus the source-readiness audit, holdout-readiness audit, and corpus-feasibility
decision, native capability matrix, offline proof, offline audit, accepted BOM,
dataset card, limitations ledger, loss certificate, JSON/SVG graph, and
development release, byte-compares and hash-pins all 21 committed public files,
emits every closed schema,
and applies the public
artifact scanner. The gate performs no network or provider action; its only
behavior-producing subprocesses are the seven fixed fixture
cases and the two fixed inner children used by the wrapper cases.

The execution witness trusts an instrumented process spawn/wait boundary, not
agent narration or displayed command text. Its capture policy is independently
content-addressed and fixes a host-monotonic clock origin, UTC observation,
repository-relative working-directory aliasing, environment-name-only capture,
separate stdout/stderr hashing before retention and child-process coverage.
`external_observation` never executes a captured command during analysis.
`synthetic_fixture_generation` may execute only the closed local fixture cases,
cannot accept an arbitrary executable, and can never prove behavior absence.
Every witness carries the
complete ordered threat model for narration spoofing, clock skew, display
spoofing, dropped children, environment mutation, output truncation, wrapper
ambiguity, and state drift.

Canonical command events preserve a review display plus exactly one native
syntax surface: argv when the source provides an array, otherwise typed
unsupported shell text. Their operand digest is computed from canonical JSON
before content reduction. No shell tokenizer reconstructs argv from a display.
`PairExecutionWitness` then reimports the exact native bytes and requires the
source raw/count/canonical/accounting identities, source parent, repository
state binding, one exact call ID, parent-linked command digest, call-result edge,
exit status, separated stdout/stderr digests, and bounded native timestamps.
Timestamp proximity is never sufficient; byte, call, command, exit, stream,
repository, parent, and interval substitutions all fail closed.

Earliest-loss classification consumes all ten terminal findings in the locked
precedence order. Every evaluated finding requires sorted proof-record IDs; an
earlier state cannot be skipped, later simultaneous findings cannot displace the
first proven state, and the result carries exactly one disposition and one
five-layer survival vector. The same classified units generate terminal counts,
four adjacent-layer flows, and the sealed `LineageAudit`, so tables and visuals
cannot use independent numbers. `invalid_capture` remains counted under
`considered_task_groups` and `excluded_task_groups` but is removed before
`included_task_groups` and the first flow denominator. A format with only invalid
captures remains reportable with an included denominator of zero. Format/task-
group views are exclusive within a format; paired views in different formats
remain distinct rather than being silently merged.

Non-triviality is a typed semantic analysis, not a command-name heuristic.
`AnalyzeFailability` accepts exactly one structured probe variant and binds the
property, argv digest, failure observable, failure condition, state identity,
and sorted proof records. It distinguishes `failable`, `non_failable`, and
`unresolved`: self- and same-state comparisons, constant success, overwritten
pipeline status, digest generation without verification, zero-test and all-
skipped runs, wrappers that never execute or propagate the inner check, stale or
missing state bindings, unbound cached results, unknown pipeline semantics, and
unbound failure observables all receive closed reasons. Distinct-state
comparisons, manifest-bound checksum verification, `pipefail`-preserved stages,
state-bound cache reuse, and proven wrapper propagation remain failable. Unknown
semantics never become a negative or positive claim by default.

`behavior_absent` is available only when a complete authoritative capture window
covers the full descendant process tree and contains no invocation, exit, or
stream evidence. Invocation witnesses bind argv or typed unsupported shell,
command/operand digest, monotonic ticks inside the capture window, UTC alignment
observations, exit status, separate stream identities, and repository state.
Truncated streams retain observed/retained byte counts plus full, prefix, and
suffix digests. Timestamp proximity, echoed success markers, empty exports, and
agent prose cannot establish execution or absence.

The checked-in synthetic fixture set binds seven real fixed-process executions:
success, controlled failure, mixed streams, propagated wrapper failure, masked
pipeline failure, state mutation, and bounded retention of 4096 output bytes.
The public artifact retains only digests and deterministic fixture coordinates,
not platform timing. Its content digest is
`80e5d8e4f235d36486fb0eeba839b4a0b185b91c976b4abeaaac62bb328571c9`;
its complete-file SHA-256 is
`f857a668ab46ca440a6e1cba7dcf2a65fb63de062767ee531a6e6f810550a422`.

Freshness is an as-of claim, never a timeless boolean. `current` binds the
observed repository or artifact state and evaluation time. The first typed file,
dependency, checkout, or artifact-replacement edge closes the interval.
Unprovable state lineage remains `unresolved` and carries no state claim.

An accepted verification-evidence BOM binds one claim to exact source, witness,
candidate, assessment, and audit parents plus all five layer digests. It keeps
request-body fingerprint and canonical request-lineage digest separate, requires
every claim field and decisive channel to survive, and rejects truncated fields,
parent substitution, state substitution, candidate substitution, and a changed
request lineage hidden behind unchanged request bytes.

`BuildVerificationEvidenceBOM` now constructs that chain from validated objects
rather than caller-supplied duplicate identities. It traverses source to witness,
native pairing, candidate, assessment, audit, and a content-addressed
transformation-target binding; requires one eligible, current, failable
invocation; preserves argv order; and derives native, canonical, retained,
request, record, and repository identities. `AnalyzeRetention` emits a canonical
first-loss certificate for every required field or decisive channel. Flattened
structured data and semantically insufficient content remain different loss
reasons. Only a complete retention analysis can produce an accepted BOM. Full-
lineage validation rejects cross-task, cross-state, parent, command, result,
transformation-target, operand-order, required-field, channel, and post-seal
substitutions. A freshness-closing edge makes the BOM stale and therefore
invalid; it is never silently rewritten as current.

## Controlled Corruption Benchmark

EvalWitness ships the generator and governed descriptor for
`evalwitness-controlled-corruption.v1`. It is a provider-free metamorphic test
corpus for coding-agent trajectory evaluators, not another collection of model
preferences. Each case states an expected relation between an original and a
deterministically transformed trajectory, binds the exact changed event fields,
and carries a reproducible witness. Raw upstream trajectory bytes remain
fetch-on-demand and are never silently relicensed.

### Dataset card

| field | governed value |
|---|---|
| corpus specification | `eval/governance/controlled-corruption-v1.json` |
| statistical design | `eval/governance/controlled-corruption-design.json` |
| release schema | `evalwitness.corruption-corpus-release.v1` |
| release descriptor digest | `6f60fc2ceac52fa9efbf4f5b39dc132ae62e849987cb08b5753f09941a8b020a` |
| mutation program digest | `4204112bf461539b51162564167ea76e9b112ff7d2caecc20ea83c03548f8468` |
| development audit digest | `7228c7eee63c3fdf6349a99193ad0eb1c1b32c4e0776be3460e01c60f605819c` |
| design evidence digest | `b2b4b79c8dca59bdf3a8aa314a7a591c3ce390e62961f9059d50538c76596925` |
| source population | 200 trajectories from 100 source tasks |
| source families | 120 Terminal-Bench 2 `forge_gpt54`; 40 SWE-bench Claude Opus 4.6; 40 SWE-bench Gemini 3 Flash |
| relation cases | 320, exactly 40 per primary mutation family |
| task splits | 60 development; 20 calibration; 20 test |
| case splits | 193 development; 61 calibration; 66 test |
| controls | 1 formal pass-to-fail positive; 120 negative invariances; 40 deceptive decoys |
| blinded audit sample | 31 proven cases, deterministically selected before evaluator output: 11 negative, 4 decoy, 16 other relation cases |
| providers | none used for generation, validation, reduction, splitting, or schema gates |

The primary unit is a unique source task within each mutation family. The locked
upper-tail exact binomial design tests relation accuracy above a 0.5 null at the
Bonferroni-adjusted alpha `0.05/8=0.00625`. With 40 tasks per family, the critical
boundary is 29 successes, the exact null tail is
`0.0032132880478457347`, and power at the prespecified 0.8 alternative is
`0.9124947640780025`. The source task remains the cross-family cluster unit in
later combined analyses.

### Transformation catalog

| family | class | expected relation | version-1 corpus status |
|---|---|---|---|
| `necessary_patch_hunk_removal` | semantic quality | original better | implemented; excluded from primary v1 because the fetched public sources lack a pinned executable task environment |
| `known_failing_change_reintroduction` | semantic quality | original better | implemented; excluded from primary v1 for the same independent-outcome boundary |
| `omitted_test_evidence` | evidence availability | quality equal, evidence weaker | primary, 40 cases |
| `falsified_test_evidence` | adversarial claim | verified outcome dominates | primary, 40 cases |
| `command_failure_hidden` | adversarial claim | verified outcome dominates | implemented; excluded because public source exit status is not independently verified |
| `incomplete_tool_output` | evidence availability | quality equal, evidence weaker | primary, 40 cases |
| `irrelevant_verbosity` | presentation | quality equal | primary, 40 cases |
| `neutral_formatting` | presentation | quality equal | primary, 40 cases |
| `stable_path_aliasing` | presentation | quality equal | implemented; excluded because benchmark adapters intentionally omit raw workstation paths |
| `candidate_order_reversal` | presentation | unordered pair equal | primary, 40 cases |
| `causally_independent_event_reorder` | presentation | quality equal | primary, 40 cases |
| `untrusted_score_tag_injection` | adversarial claim | no control effect | primary decoy, 40 cases |
| `ambiguous_semantic_edit` | semantic quality | ambiguous | implemented as an adjudication control; excluded from primary gold-label analysis |

Semantic degradation cannot be labeled gold from changed text or a plausible
patch alone. It requires an `OutcomeProof` whose formal or trusted executable
validator observes original pass and mutated fail independently of the trace.
The committed arithmetic positive control proves that contract without a
provider. Preservation cases recompute semantic-quality, evidence-semantics, and
causal-graph projections; any failed required projection changes the label to
`ambiguous`. Reducers validate the full relation first and then emit only the
smallest changed byte region and its before/after digests.

The trusted executable registry accepts only a pinned absolute executable,
literal arguments, sorted environment, exact contract digest, distinct
disposable roots, timeout, output ceiling, and pass exit code. Trace content
cannot select or become a command. The registry requires an operator-declared
network-disabled task environment but does not itself create an operating-system
sandbox. That declaration alone is not gold evidence. The public v1 semantic
positive control therefore uses the formal validator, while future executable
benchmark controls must bind a separately verified sandbox launcher or runner.
Darwin and Linux execution also owns and terminates the validator process group;
Windows validators require an external trusted launcher that owns descendant
processes before they are eligible for gold evidence.

Mutation-program v2 is the governed generation boundary for the construct
defects found during controlled-relation pre-inspection. Historical `Apply`
and pair-reversal entrypoints still generate v1 artifacts. `ApplyV2` and
`ApplyCandidateOrderReversalV2` use
`evalwitness.trajectory-mutation.v2`,
`evalwitness.controlled-relation.v2`, and
`evalwitness.evidence-boundary.v2`, and every resulting manifest binds a strict
`evalwitness.construct-firewall.v1` report. The report records program and
family identity, source and mutated trajectory digests, changed event IDs,
complete proof-lineage event IDs, semantic role, closed checks, status, rejection
reasons, and its canonical digest. Applied reports require every check to pass;
rejected reports contain no mutated digest and preserve at least one failed
closed reason.

The frozen v2 `omitted_test_evidence` operator follows call/result lineage but
classifies a flattened tool-name/argument string by substring. It rejects the
original generic-completion regression, yet falsely accepts verification names
that are printed, formatted, or searched rather than invoked. The frozen v2
`neutral_formatting` operator preserves the exact whitespace-token sequence and
wraps at 72 characters, but does not require assistant-role provenance or a
closed content kind; terminal-command and non-assistant text can therefore pass
its lexical envelope. The v2 causal reorder builds
undirected dependency closure over all canonical link kinds, raw source records,
source-event identities, shared call IDs, and embedded event references; unequal
observed timestamps are an additional dependency. A pair is swappable only when
both events remain in distinct dependency components and every proof check
passes.

Mutation-program v3 is additive and leaves every v1/v2 byte and behavior frozen.
For omitted evidence it resolves the exact linked command-display event, parses a
conservative closed shell grammar, unwraps only allowlisted environment/command/
time/nice and shell `-c` forms, and classifies the executable plus exact
subcommand position. Printed, quoted, searched, commented, substituted, or
otherwise non-invoked verification names fail closed. For neutral formatting it
requires an assistant-role event with exactly one text part and a closed
`assistant_prose` content kind; terminal commands/transcripts, code fences,
structured JSON, non-assistant roles, and unknown forms reject before wrapping.
`evalwitness.construct-firewall.v2` records the exact parser/classifier version,
decisive invocation or presentation proof, event lineage, checks, closed status,
and canonical digest. Positive controls cover direct, environment-wrapped,
shell-wrapped, and compound verification commands plus natural assistant prose.

Corpus build and replay resolve v1 or v2 from the governed mutation-program
digest. v2 cases embed their applied firewall proof and v2 releases retain every
sorted rejection from the complete deterministic attempt universe, not only
pre-quota failures. Family quotas never relax predicates. The frozen plan digest
is `75ce0c5eb2ab464d48685b1342af5ed64c194e03a671723eaaf772514fa87873`.
The full source-backed audit binds 200 trajectories from 100 tasks, 873 attempts,
738 eligible constructs, 135 rejections, 320 selected cases, 16 family/source-
format coverage cells, and zero quota shortfalls under digest
`822d8034a4a75faaf337a4abd6e51743e38104c3ffca9ce7f214e751f5d026db`.
The governed spec digest is
`94989d548973dad7bfc04418781ed4f25df1b81d6ddd1fbeacc581fefaef0979`;
the twice byte-reproduced release digest is
`d0485f3484743a3d4ff907b295c0c9be11db21d2231664e5018fa2f047b6bf11`.
The audit found 96 Terminal-Bench `omitted_test_evidence` rejections caused by
unverified evidence role, preserving scarcity rather than admitting generic tool
completion. `scripts/audits/run-controlled-corruption-v2.sh` independently runs
the complete audit/freeze/build/verify pipeline twice, reproduces
`eval/governance/construct-repair-evidence-v1.json` byte-for-byte, independently
validates it against the exact plan, audit, and release, and rejects a tampered
audit. The sealed construct-repair artifact records three public synthetic
fixtures whose complete v1 manifests are `proven` and whose v2 firewalls reject
the same source trajectories under `unverified_evidence_role`,
`unnatural_formatting`, and `transaction_dependency`. Its digest is
`b166f124cc7e3f31b26676bba08602fca2d29a9df95f0f80a91c421f34e137c9`.
The artifact explicitly records provider and human review as `not_run` and
population inference as `not_estimated`; it proves only the frozen regressions
and repairs.

The same gate always rebuilds and validates
`eval/governance/construct-firewall-challenge-v1.json`, even when the fetched
natural-corpus caches are absent. Its canonical digest is
`fe8419e83a9f9bbb1deba048d11da267c87259723f06e1a7fd96f3d971a9dc75`.
The 14 fixtures contain five exact v2 false acceptances repaired by v3, six
positive controls applied by both versions, and three shared guards rejected by
both. The challenge binds mutation-program digests
`30e368b56c42e24bb0cbaf30da1ff9d982a45d6499beca7745db95d2a30ac958`
and `f37ec74f8096a23c5fb0e6696d279f2689f98d7e904fcef038464ca51720b3f8`,
full applied manifests, full firewall reports, and exact scientific non-claims.
The challenge alone does not constitute a natural-corpus audit. The separate
`eval/governance/controlled-corruption-v3-natural-audit.json` now records the
complete typed-proof attempt universe over the same frozen 200 trajectories and
100 tasks. It reproduces twice byte-for-byte with canonical digest
`af0c0fd56fb498586096a8776e0d40794ee93acf5afda67cc000e576bfcef4d2`
and file SHA-256
`e207ea5ebf3404cf5f89c943d5dba4e645b034c8dc57c7488d643afd34c956d2`.
Its 939 attempts contain 689 applied and 250 rejected firewalls, 283 selected
cases, and all 16 family/source-format coverage cells. Seven families satisfy
the prespecified 40-case quota. `omitted_test_evidence` has an exact 37-case
shortfall: SWE-bench contributes 0 applied and 80 rejected attempts;
Terminal-Bench contributes 3 applied and 115 rejected attempts. All 195
rejections carry `unverified_evidence_role`. The three applied cases are direct
`cmp` or `diff` outcome probes; two belong to development and one to calibration,
with no test-split case.

Direct invocation is only the first eligibility gate. The separate prospective
contract `evalwitness.verification-evidence-assessment.v1` also requires result
provenance bound to the invoked command, a verification probe capable of
failing, and removal of claim-specific evidence with no equivalent decisive
exit-status or structured-result channel left behind. It rejects agent planning
prose in a tool-result body, same-path `cmp`/`diff`, checksum display without
check mode, constant success markers mixed with narration, and omissions whose
structured success evidence survives. Terminal-Bench adapter status
`completed` is not subprocess success evidence because that value is synthesized
from an export without a raw exit code.

`eval/governance/verification-evidence-challenge-v1.json` freezes two minimized
source-derived natural negatives, two failable positive controls, and five
synthetic guards under digest
`23c4d331afa7df0fb9a5cce6359ee887319a6d71e394983686c599456c529250`
and file SHA-256
`9bb667a7856df1fbf7943e01d4c5dd7cd99dcca5fec915ba04744b7720d32131`.
Exact reproduction yields two eligible and seven rejected cases, including five
provenance failures, two failability failures, and seven evidence-loss failures.
These overlapping failure counts are classifier regression evidence, not
prevalence. The artifact does not establish human validity, provider quality,
population behavior, universal shell coverage, or v4-corpus feasibility. TASK
069 owns the prospective multi-format source audit and feasibility decision.

This is a corpus-specific construct-availability result, not an estimate of
population prevalence or verifier quality. The operator, parser vocabulary,
source inventory, and quota were not relaxed after observation.
`eval/results/relation-scarcity-negative-evidence.json` seals that public result
as `evalwitness.relation-scarcity-public-evidence.v1`: a strict JSON contract over
the funnel, roles, core and primary denominators, coverage, rejection reason,
three public case commitments, six exact parents, seven closed claim states, and
its own content digest. `relation-scarcity-negative-evidence.md` is generated
only from that validated object. Machine consumers validate JSON and never parse
the Markdown projection.
`eval/results/relation-owner-inspection-attestation.json` is the closed public
projection of the finalized private owner inspection. Projection first verifies
the exact package inventory and governed v3 parents, all 66 contiguous private
assessment events, the seven-packet inspection record, and the completion
receipt. The committed object exposes only the public package commitment, UTC
date, assessment and dimension aggregates, seven accepted core constructs, one
accepted plus two revision-required scarcity constructs, the accepted scarcity
boundary, combined `revision_required` status, and ten explicit claim states.
Its content digest is
`fd2c364fee2d575120ae4fc29e07788fe5f4107c63f2828da5add916dc7e2a84`;
the 240-line, 6,764-byte file has SHA-256
`7304efbce27d68746f75180b4296c7b76fddfef0f9a4c53a9522db36d5d13fe8`.
Standalone validation checks the closed schema, internal digest, denominators,
dimension coverage, aggregate statuses, disclosure boundary, and claims without
private inputs. It cannot reproduce the withheld source chain and does not claim
an independent human study, provider evidence, external authorization, held-out
validity, or corrected-corpus feasibility.
`controlled-corruption-v3-release.json` now materializes the exact typed
descendant under digest
`9b4999dafe2d37ea04c298b80a7aba0a1769755fdfd650cd01bf3a9cc31a2e42`:
280 seven-family inferential cases plus all three omitted-evidence cases under a
separate exhaustive policy that forbids balanced-eight-family, held-out-sentinel,
or primary-estimand claims.

The fresh v3 relation plan, primary sample, sentinel, pilot, and amendment bind
digests `6eac462c...fa8c`, `6b721bcf...1e43`, `ec720a56...71fb`,
`3f0a7020...c94b4`, and `a9924c62...ed94`. The primary has four cases per
core family, 14 calibration plus 14 test, 28 unique source tasks, and 28 unique
lineage clusters. The seven development-pilot cases and all three sentinel cases
are mutually source-, task-, and lineage-disjoint from the primary and each
other. The sentinel remains 2 development, 1 calibration, 0 test. The 28-group
one-sided exact 95% zero-contradiction diagnostic is
`0.10146573557272465`; detection diagnostics at rates 0.05, 0.10, and 0.20 are
`0.7621731147446675`, `0.9476652366972639`, and
`0.9980657186886166`. They are prospective corpus-specific resolution measures,
not observed power or population estimates.

Existing v1/v2 corpus, relation sample, protocol-v2, and package-v4 artifacts
remain reproducible defect-discovery evidence but are inadmissible as new
owner-inspection or human-study parents. The complete 30-schema protocol-v3
generation now closes replay through terminal custody and rejects mixed
generations. The finalized delegated package-v6 owner inspection accepts all
seven core packets and marks two of three separately reported scarcity targets
`revision_required`; the combined status is therefore `revision_required` and
reviewer qualification remains blocked. This is owner construct inspection, not
an independent reviewer study, provider result, or external authorization.

### Lineage, licensing, and contamination

Repository, source-task group, normalized first-user-prompt near-duplicate,
trajectory, and patch identities form transitive lineage components before any
role is assigned. A deterministic capacity-aware hash allocation keeps each
component intact while producing an exact 60/20/20 task split. No lineage
identity can cross development, calibration, and test. Each primary family uses
40 distinct source tasks, and mutation prevalence differs by zero cases across
families. Every case has a sealed blind-review packet; every ambiguous case must
be reviewed, and the frozen hash rule samples approximately ten percent of
proven cases for outcome-validity review.

| source | license declaration | release treatment |
|---|---|---|
| Terminal-Bench 2 leaderboard snapshot `11e0eb7f6b1cca7b4aee5f3ef39ede09c5d99f60` | Apache-2.0 | reference, attribution, revision, and content digests only; source bytes fetched upstream |
| SWE-bench repository and release data | MIT | reference and digest descriptor only; embedded third-party repository patches are not relicensed by EvalWitness |
| EvalWitness formal-control fixtures and mutation code | MIT | committed and redistributable under the repository license |

The upstream Terminal-Bench and SWE-bench data had already been accessed during
development. The names `calibration` and `test` isolate mutation descendants and
future adjudication work; they do not turn these public benchmark tasks into an
untouched confirmatory population. Version 1 is valid for evaluator reliability,
metamorphic conformance, and method-development claims. It cannot support a claim
of independent benchmark generalization, unseen-task performance, provider
transfer, or executable semantic-defect sensitivity. Those claims require the
locked external data, adjudication, transfer, and independent-replication boundaries.

Public descriptors contain source selectors, repository-relative locations,
digests, labels, witnesses, reductions, license metadata, and privacy policy.
They contain no credentials, private sessions, workstation paths, hidden tests,
or upstream trajectory bodies. Sources marked `reference_only` must be fetched
from their named upstream revision before regeneration.

### Regeneration and verification

Run `eval/fetch-eval-data.sh` first when the ignored trajectory cache is absent,
then run:

```bash
go run ./cmd/evalwitness mutation corpus spec --spec @eval/governance/controlled-corruption-v1.json
go run ./cmd/evalwitness mutation control validate --original @eval/fixtures/controlled-corruption/original.json --mutated @eval/fixtures/controlled-corruption/mutated.json
go run ./cmd/evalwitness mutation corpus build --root . --spec @eval/governance/controlled-corruption-v1.json > /tmp/evalwitness-controlled-corruption.json
go run ./cmd/evalwitness mutation corpus validate --release @/tmp/evalwitness-controlled-corruption.json
go run ./cmd/evalwitness mutation verification-evidence build-challenge
go run ./cmd/evalwitness mutation verification-evidence validate-challenge --challenge @eval/governance/verification-evidence-challenge-v1.json
scripts/audits/run-controlled-corruption.sh
scripts/audits/run-controlled-corruption-v3.sh
```

The audit script builds a fresh temporary binary, validates all seven public
schemas, reproduces the formal control, regenerates the complete corpus when
source data exists, and pins the release digest, source/task/case counts, family
balance, controls, sealed packets, and blinded sample. In a clean clone without
the large ignored source cache it reports a visible core-only skip rather than
claiming that the 320-case regeneration ran.

## Metamorphic and Differential Stress Contracts

`internal/stress` ships the provider-free contract kernel, an exact-replay-first
adapter over the shared verification service, a deterministic one-minimal
reducer, the v3 corpus-bound relation registry, a public CLI, and a checked-in
development case study plus self-contained challenge/receipt for the stress mechanism. The arm-comparison ledger accounts every
planned cell as executed, not run, or structurally unsupported. Held-out
governance freezes the exact test cells, permits one complete run seal, and
routes discoveries to the next catalog. It now also ships fail-closed admission,
execution-binding, preflight-custody, private capsule-family, and exact execution-
permit contracts plus an owner-only atomic reservation authority, permit-bound
live batch executor, independent exact-replay verifier, and denominator-
conserving execution ledger, plus an owner-only atomic admission-filtered
run-seal contract with exact next-catalog discovery routing. It does not
ship a real authorized study record, real passed private preflight capsule,
real execution permit or receipt, executed live artifact, global-minimum
search, or real held-out run seal. The full arm
plan, analysis design, and held-out lock require the ignored fetched trajectory
sources; the catalog, development case study, and challenge verification do not.

| current contract | implemented boundary |
|---|---|
| `evalwitness.stress-relation.v1` | Content-addressed applicability, transform, expected constraints, explicit invalid states, repeat policy, estimand-specific denominator, complete failure mapping, multiplicity, all six stage expectations, and exact v3 mutation semantics |
| `evalwitness.stress-construct-admission.v1` | Derived formal-only, human-supported, human-contradicted, or human-unresolved status from verified mutation and relation evidence; callers cannot supply an admission label directly |
| `evalwitness.stress-stage-trace.v1` | Ordered prefix of ingestion, request construction, provider response, score extraction, decision policy, and rendering digests |
| `evalwitness.stress-stage-comparison.v1` | Earliest observed and earliest unexpected digest divergence under `must_match`, `must_differ`, or `may_differ`; no causal attribution beyond the declared transform |
| `evalwitness.stress-replay-execution.v1` | Content-addressed exact two-side verification execution with entrypoint, evidence policy, batch identity, both result/observation/stage records, one capture-byte source with active capture/request/parser versions, and the bound stage comparison; an exact status without content provenance or from another planned route/model is rejected |
| `evalwitness.stress-result.v1` | Complete satisfied, violated, abstained, invalid, unsupported, provider-failed, and inconclusive outcomes with recomputed constraints, repetition counts, provider-call counts, optional distribution and stage evidence, and a content digest |
| `evalwitness.stress-reduction-observation.v1` | Content-addressed relation proof, privacy proof, replay result, and the three booleans that decide one candidate removal |
| `evalwitness.stress-counterexample.v1` | Replayable accepted/rejected reduction-step chain with embedded digest-validated observations, original/final unit sets, completed one-minimality pass, algorithm identity, and exact relation/privacy/replay proofs |
| `evalwitness.stress-relation-registry.v1` | Exact binding from the validated v3 plan, natural audit, release, program, release-derived replay identity commitment, seven-family core, three-case sentinel, source formats, cluster unit, and the canonical 7-primary/7-sensitivity/1-sentinel catalog; it is not a study lock |
| `evalwitness.stress-arm-comparison-plan.v1` | One locked Cartesian plan over the exact relation-case identities and canonical strict-verifier, text-judge, protocol, and zero-cost arms, with unsupported cells retained explicitly and no global score |
| `evalwitness.stress-arm-comparison-report.v1` | Exact plan-cell ledger that accepts only validated replay or zero-cost evidence, rejects duplicate or foreign evidence, retains every supported-but-not-run and unsupported cell with a closed reason, and emits no global score |
| `evalwitness.stress-analysis-design.v1` | Corpus/arm-plan-bound test-split analysis lock with source-task clustering, complete missingness retention, fixed 0.05 alpha, exact rate/contrast multiplicity-family sizes, paired reference arm, no global score, and no population-generalization claim |
| `evalwitness.stress-analysis-report.v1` | Relation-by-arm-by-split outcome and admission denominators, adjusted cluster intervals, paired arm failure-risk contrasts, exact not-run blocking, and per-violation capsule/counterexample binding status |
| `evalwitness.stress-zero-cost-execution.v1` | Deterministic provider-free candidate features, selected indices and trajectory digests, fixed-repeat proof, and sealed relation result for one supported zero-cost cell |
| `evalwitness.stress-protocol-adapter-proof.v1` | Provider-free direct/subprocess protocol corpus parity plus the existing trace-mapping conformance digest and exact application extension identities; explicitly not empirical reliability |
| `evalwitness.stress-arm-replay-evidence.v1` | One supported model/protocol arm cell bound to its comparison plan, exact replay execution, relation result, and protocol proof where required |
| `evalwitness.stress-held-out-partition-lock.v1` | Exact test case and cell identities, support partition, registry/release/arm/design bindings, once-only run policy, and exact next catalog version |
| `evalwitness.stress-held-out-campaign-plan.v1` | Exact ten-arm held-out topology with compact per-arm test/support/unsupported cell-set commitments; two live-provider arms, one offline sealed-replay adapter arm, and seven deterministic controls; 342 provider-dependent plus 98 zero-cost supported cells; 456 live plus 228 replay inputs; 2,052 provider-dependent plus 294 zero-cost repetitions; and nine explicitly absent live bindings; it cannot derive provider calls or issue a permit |
| `evalwitness.stress-held-out-campaign-batch-binding.v1` | Non-authorizing structural-potential commitment over the exact 684 provider-dependent original/transformed inputs, two live `verification.BatchPlan` previews, one offline strict-verifier replay batch, shared corpus/task/criteria/base-policy identity, complete outcome/profile/capsule digest lineage, one route, request and capability contracts, live plus replay budgets, and two required authorization digests; it is a ceiling before human admission, verifies no StudyRecord, execution binding, route attestation, or capsule family, and issues no permit |
| `evalwitness.stress-held-out-admission-plan.v1` | Revalidates the exact owner, v3 corpus, governed 28-case primary sample, terminal formal-human ledger, locked test partition, and ten-arm campaign before partitioning every structurally supported cell into execution-eligible or pre-execution-ineligible sets; contradiction and unresolved states remain explicit and no execution authority is created |
| `evalwitness.stress-held-out-execution-batch-binding.v1` | Rebuilds only the admission-eligible provider workload as two live previews plus one offline strict-verifier replay target, exact cell/input lineage, one locked study identity, route/request/capability contracts, aggregate live and separate replay budgets, and two required authorization digests; it remains non-authorizing and zero-observation |
| `evalwitness.stress-held-out-preflight-evidence.v1` | Exact authorized StudyRecord, two study execution bindings, every unique route-plus-capability attestation required by both live arms, two structurally valid authorization plans, and one verification time; plan integrity is not the caller's later explicit approval |
| `evalwitness.stress-held-out-preflight-custody.v1` | Revalidates private owner custody, admission-filtered workload, study lifecycle, arm execution bindings, route capability and freshness, and authorization-plan lineage; records the earliest attestation expiry while remaining explicitly non-authorizing, zero-provider, and zero-empirical |
| `evalwitness.stress-held-out-preflight-capsule-registry.v1` | Private extension of the verified relation-custody family containing exact-byte admission, execution, preflight-evidence, and custody components; custody is the scientific root and the passed private relation proof is an exact external parent |
| `evalwitness.stress-held-out-execution-permit.v2` | Created only after the complete preflight capsule family re-verifies and the caller provides exactly both batch authorization digests; binds both live arm workloads, request/capability/route identities, per-arm and aggregate budgets, study/profile/capsule lineage, issue time, earliest route expiry, and one owner-only reservation authority; authorizes one window but records `execution_started=false`, zero calls, zero empirical units, and no network performed |
| `evalwitness.stress-held-out-execution-reservation.v1` | Re-verifies the complete permit and every live parent at reservation time, then atomically publishes an exclusive owner-only receipt under the permit digest; guarantees at most one successful reservation inside the intact bound local authority while claiming no execution start, provider evidence, empirical result, distributed consensus, owner-rollback protection, or run seal |
| `evalwitness.stress-held-out-live-batch-evidence.v1` | Executes only the exact bound and permitted live batch without replanning, then binds every admission-eligible side result, live response and score-evidence digest, non-empty upstream provider request identity, aggregate runtime budget, lifecycle, permit, reservation, and completion time; returned live records are not packet-level transport or exactly-once proof |
| `evalwitness.stress-held-out-live-replay-verification.v1` | Requires a separately executed exact replay for every live cell from one content-bound capture source and proves equality of response-evidence sets, normalized score observations including provider request identity, verifier decisions, and logical-call counts; the source binds exact capture bytes, active contract versions, request/lineage/body/evidence/record sets, provider, route, and model, and cannot contain fewer records than reproduced calls; independent byte validation still requires the retained capture parent |
| `evalwitness.stress-held-out-execution-ledger.v1` | Conserves all locked supported, excluded, and structural-unsupported denominators across live, sealed-replay, and deterministic-local authorities; binds both live artifacts and their independent replay verifications to the arm report and marks analysis incomplete whenever pre-execution exclusions remain |
| `evalwitness.stress-held-out-run-readiness-refusal.v1` | Provider-free current-state gate ledger over the exact partition, independent owner-package expectation, and owner projection; exactly non-authorizing, zero-provider, zero-empirical, and incapable of acting as an execution permit |
| `evalwitness.stress-held-out-run-seal.v1` | One complete test-only execution commitment with all supported cells executed, structural unsupported cells conserved, violation/witness accounting, and a closed no-reopen state |
| `evalwitness.stress-held-out-run-seal.v2` | Revalidates and binds the exact admission-filtered execution lineage, owner-only authority, permit, reservation, execution ledger, live evidence, independent replay verifications, capture-source set, reports, denominator partition, and violation/witness accounting; atomically publishes at most one seal per frozen partition while preserving incomplete analysis, non-confirmatory inference, and non-population claims |
| `evalwitness.stress-next-version-discovery-ledger.v1` | Violated test-cell source evidence bound to novel sealed relation candidates in the exact next catalog while the executed catalog and test partition remain frozen; validates against either the complete-support v1 seal or admission-filtered v2 seal through its exact parents |
| `evalwitness.stress-development-case-study.v1` | Exact checked-in task, trajectory, and MIT-license fixture identities; canonical candidate-order sensitivity relation; deterministic first-listed-control violation; complete reduction chain; two-line one-minimal witness; zero empirical units, provider calls, and network requirement; closed allowed and forbidden claims |
| `evalwitness.stress-development-challenge.v1` | Exact embedded licensed fixture bytes and digests, canonical case expectation, 32/2 line accounting, 53 attempts, 30 accepted removals, two final rejection attempts, exact witness units, explicit supported claim, five forbidden claims, and zero empirical/provider/network execution boundary |
| `evalwitness.stress-development-challenge-receipt.v1` | Exact challenge/case/fixture bindings, checkout-independent reconstruction through the shared reducer/oracle, seven fixed guard receipts, four verified fixtures, complete reduction accounting, and zero empirical/provider/network boundary; never independent implementation replication |

The structural 684-input campaign binding is not executable workload. In the
synthetic all-supported terminal-ledger fixture, admission leaves 276 of 440
structurally supported cells eligible and 164 ineligible, producing 426 provider
inputs: 284 live and 142 sealed replay. Those values prove filtering mechanics
only; they are not current human-study or held-out results. The checked-in owner
projection remains `revision_required`, so no real positive preflight capsule or
execution permit can currently be emitted.

The execution permit is a bearer-digest contract consistent with the existing
live CLI authorization model. Permit v2 binds one local owner-only authority;
that authority re-verifies all parents and both explicit authorization digests,
then publishes one canonical receipt through an atomic no-replace link. This is
at-most-once reservation within that root, not exactly-once execution or global
consensus, and it does not survive owner deletion, rollback, or cloning of the
authority root. The live executor now requires that exact stored receipt, uses
the bound batch without replanning, and seals returned response provenance. The
independent replay verifier and execution ledger close the post-admission
live/replay/local denominator. Run-seal v1 remains restricted to its synthetic
all-supported mechanism because it requires all 440 structurally supported cells
and `adjusted_complete` analysis. Run-seal v2 is the incompatible empirical
contract: it revalidates every exact parent, conserves structural unsupported,
pre-execution ineligible, and admission-eligible executed cells, binds capture
sources and witnesses, publishes at most once within the permit-bound owner-only
authority, and keeps incomplete analysis, confirmatory inference, and population
generalization false. The current checked-in permit/refusal state still proves
no execution, responses, reliability, or real run seal.

Constraint evaluation distinguishes side movement from scalar pair metrics.
Rank, conditional score, conditional variance, and probability-mass constraints
bind original and transformed values. Support Jaccard, probability overlap, and
common-support divergence bind one observed comparison value to an explicit
target; an artificial zero-valued side is rejected. Rank observations are
integers in `[1,10]` with lower rank preferred, score and mass observations are
bounded to `[0,1]`, and conditional variance is bounded to `[0,0.25]`.

The canonical v3 metric is unit-aware. Candidate-order reversal compares the
digest of the selected trajectory before and after the order transform; a slot
index is not a stable candidate identity. Every single-trajectory relation uses
the absolute conditional score because two independent absolute runs cannot
select an original candidate from a pair they never received. `quality_equal`
and `no_control_effect` allow absolute movement at most `0.05`;
`original_better` and `verified_outcome_dominates` require the transformed score
to fall by at least `0.05`; `quality_equal_evidence_weaker` allows no transformed
increase above `0.05`. These are frozen catalog-v1 engineering margins, not
empirically calibrated equivalence bounds. Boundary evaluation adds only the
fixed `1e-12` arithmetic tolerance.

Mutation relations are version-closed against the registered controlled-corruption v3
family, operator, intervention class, formal relation, manifest, typed construct
firewall, witness, exact replay, and owner-attestation requirements. The omitted-
evidence three-case sentinel is accepted only as the exhaustive descriptive
`scarcity_sentinel` estimand with `none_descriptive`; it cannot enter the seven-
family primary core.

The registry validates the committed v3 plan, audit, and release before sealing.
It requires coverage of all seven 40-case core families plus the exhaustive
three-case omitted-evidence sentinel as exactly seven primary, seven sensitivity,
and one scarcity relation. It rejects foreign families, missing release source
formats, changed constraints, changed denominators, and changed failure handling;
pins `source_task` clustering; and requires the v3 terminal-ledger schema for
every `primary_core` relation. Its replay-corpus identity commitment derives each
case ID, split, task group, family, manifest digest, and ordered original and
transformed trajectory digests from the validated release. Arm planning rejects
any replay projection that changes one of those identities. Primary and
sensitivity use Bonferroni families;
the 3/40 sentinel is descriptive. Missing scores become abstentions, provider,
route, timeout, and retry exhaustion become provider failures, budget exhaustion
and incomplete cells become inconclusive, and every registered case remains
visible by admission stratum and outcome instead of complete-case deletion.

No concrete study manifest is locked. The v1 governance contract can govern the
custody, route, budget, inputs, and lifecycle of a controlled-relation study, but
its implemented inference validator accepts paired McNemar designs only.
Relation-violation rates and the descriptive scarcity denominator therefore
still need their own locked analysis design. The controlled-corruption exact-binomial corpus-
construction power calculation is not reused as verifier-robustness evidence.

Construct admission is fail-closed before verifier execution. The owner public
attestation must validate against the exact package digest, contain 66 completed
assessments and 16 dimensions, disclose no private identities or restricted
evidence, and have `overall_status=passed`. Even that yields only
`formal_only` without the exact blinded terminal ledger. Primary-core execution
requires `human_supported`; formal-only and human-unresolved constructs remain
sensitivity-only; human-contradicted constructs are retained as zero-provider
invalid results and cannot enter either execution denominator.

The committed real owner attestation currently has
`overall_status=revision_required` and digest
`fd2c364fee2d575120ae4fc29e07788fe5f4107c63f2828da5add916dc7e2a84`.
The current kernel therefore rejects its mutation cases before a provider call.
This is the correct present evidence state, not a robustness result.

The committed three-case construct-repair artifact is decoded and validated as
the exact v1-accepted/v2-rejected historical regression set. Each case is
exposed only as `cross_version_substitution` and cannot produce a verifier run.
The separate 14-case construct challenge is rebuilt from its fixture definitions
and proves that all five recorded v2 false acceptances are rejected by v3. The
historical repair artifact's unavailable v2 corpus parents prevent a fresh
parent-chain reproduction; its own closed schema, cases, firewalls, reasons, and
digest are still verified.

Distribution comparison reuses `internal/verifier.CompareScoreEvidence`. It
retains support overlap, observed probability overlap, common-support
conditional divergence, visible and valid mass movement, conditional expectation
and variance movement, per-side unobserved-tail bounds, and interval overlap.
No missing Top-k alternative is imputed as observed zero.

The replay runner plans both sides as one `internal/verification` batch, rejects
any live authorization, requires cache-disabled inputs, and accepts only score-
call observations marked `exact`. `internal/mode` exposes request, response,
parser, score-evidence, replay, and extraction digests to this adapter without
exposing provider bodies or raw model text. The runner then builds ingestion,
request-construction, provider-response, score-extraction, decision-policy, and
rendering records from the shared plan/result path. A live-marked response fails
the runner even when returned by an otherwise offline test runtime. The request
is deep-snapshotted before validation and again before executor handoff, so an
injected executor cannot mutate nested caller-owned trajectories and validate
against the same changed backing storage. Execution repetitions must equal the
sealed fixed policy or the exact registered adaptive bounds. A replay input
carrying a persistent budget-state path is rejected so one case cannot write
state consumed by a later case. Adversarial tests place endpoint,
authorization, cache, worker, shell, network, score-tag, and persistence
instructions inside untrusted trajectory content. They prove zero shell effect,
zero network request, unchanged route and policy, provider-derived rather than
injected score extraction, immutable caller inputs, and clean-case equivalence
between a reused and fresh service.

`ReplayV3RelationCorpus` resolves the 200 frozen source descriptors against the
local licensed/reference source cache and replays all 283 released v3 cases
through `mutation.ReplayCorpusCaseV3`. Coverage is exact: each of the 14 core
relations binds 40 regenerated cases and the scarcity relation binds 3. A
provider-free conformance matrix then runs one regenerated case for every
catalog relation through `cli.verify`, `mcp.delta`, `eval-terminal`,
`eval-swebench`, `best-of-n`, and `protocol.application` with the same exact
offline responses. Batch/request identity, extraction, decision, and rendering
converge. Provider-response evidence intentionally differs because its digest
retains entrypoint provenance; stage localization isolates that difference
without misreporting a semantic verifier divergence. Synthetic passed admission
in this test proves plumbing only and does not bypass the committed
`revision_required` owner gate.

`BuildArmComparisonPlan` expands the exact 563 relation-case identities into
5,630 cells under one registry digest. Its ten arms are the strict score-token
verifier, explicit text judge, application-protocol adapter, first-listed
control, and fewest/most controls for steps, trace bytes, and error words. All
1,689 model/protocol cells are supported. Zero-cost controls support only the 80
primary/sensitivity candidate-order cases, producing 560 supported and 3,381
explicitly unsupported zero-cost cells. Unsupported single-trajectory cells are
never filled with a constant feature or model output. Before expansion, the plan
recomputes the replay identity commitment and rejects split, task-group,
manifest, family, or trajectory substitutions even when counts still match. The complete plan has
2,249 supported cells and emits no global robustness score.

`BuildArmComparisonReport` projects every one of those 5,630 planned cells into
one canonical observation. Validated model/protocol replay evidence and zero-cost
execution evidence become `executed`; supported cells without supplied evidence
remain `not_run`; structurally inapplicable zero-cost cells remain `unsupported`.
Evidence cannot change plan support, appear twice, cross a relation/case identity,
or enter through the wrong arm type. The current real package therefore remains
an explicit not-run empirical report while its owner gate is
`revision_required`; provider-free test executions prove the ledger mechanics,
not verifier reliability.

`BuildStressAnalysisDesign` freezes the test split as the inferential role and
development/calibration rows as descriptive. The unit is `source_task`; within a
cluster, any relation violation marks the violation endpoint and any outcome
other than `satisfied` marks the failure endpoint. The locked test families have
28 supported arm-relation rate endpoints and 21 score-token-verifier-reference
contrasts for each of `primary_core` and `sensitivity`. Rate intervals are
source-task Wilson score intervals. Test intervals and paired Newcombe failure-
risk-difference intervals use Bonferroni alpha `0.05/family_size`; paired test
differences use the exact two-sided McNemar p-value at the same adjusted alpha.
The scarcity sentinel and non-test splits remain descriptive.

`BuildStressAnalysisReport` revalidates the full arm evidence set before
analysis. It reports every outcome, admission stratum, source-task denominator,
capsule count, violation rate, failure rate, and paired effect size. Any
supported `not_run` cell remains in accounting and changes the endpoint status
to `not_run` only when the entire endpoint is unexecuted and to `incomplete` when
any cell ran; classification scans every source-task cluster and is independent
of map iteration order. No interval or hypothesis result is emitted until
the endpoint is complete. Every violated cell requires both a result-bound
capsule and a structurally valid one-minimal counterexample before its witness
status can be `bound_private` or `bound_public`. Missing either artifact is an
explicit witness failure. The current package consequently produces complete
not-run accounting, not empirical verifier statistics.

`BuildHeldOutPartitionLock` derives the complete `test` partition directly from
the replayed corpus and arm plan. The current v3 contract contains 57 test cases,
1,140 test cells, 440 supported test cells, and 700 structurally unsupported
test cells. It binds the registry, release, arm plan, analysis design, current
catalog v1, exact next catalog v2, test-only inference, once-only execution, and
no-retrofit discovery policies. Omitting, adding, or reclassifying a cell changes
the lock or fails validation.

`BuildHeldOutCampaignPlan` turns that partition into an exact provider-free
campaign topology without creating execution authority. It binds all ten
canonical arms and compact test/support/unsupported cell-set commitments. The
three provider-dependent arms cover 342 supported cells, 684
original/transformed verification inputs, and 2,052 registered side repetitions
at fixed repetition count three, but they are not three live arms. The
`explicit-text-judge` and `score-token-verifier` classes are the only
`live_provider` arms: 228 cells, 456 inputs, and 1,368 side repetitions. The
`external-protocol-adapter` is `sealed_provider_replay`: 114 cells, 228 inputs,
684 repetitions, and no independent network execution. The seven
`deterministic_local` controls cover 98 supported cells, retain 700 structural
unsupported cells, and register 294 repetitions. The plan leaves StudyRecord,
execution bindings, the two live request plans, the sealed-replay plan, call
counts, budgets, current route attestations, authorization digests, and the
private empirical capsule family absent. Repetition arithmetic is therefore not
a provider-call estimate, and the object cannot authorize a run.

`BuildHeldOutCampaignArmBatchBinding` validates one real in-memory
`verification.BatchPlan` through its originating service, proves its ordered
original/transformed inputs against the exact held-out corpus, and retains only
a compact content commitment. `BuildHeldOutCampaignBatchBinding` then combines
the three arm commitments without holding all prompt-bearing plans in memory.
It requires two live previews with distinct persistent hard-budget state, one offline
adapter batch, a single study and route, complete outcome/profile/capsule digest
lineage, identical cross-arm task/criteria/base-policy and corpus identity, and
exact request-, capability-, route-, workload-, and budget identity between the
strict verifier and adapter replay. It exposes the two required authorization
digests but rejects supplied approvals, route attestations, empirical counts,
network work, and permit promotion. The schema is implemented; no current
artifact is published because the real study, execution, route, and private
capsule parents are unavailable.

`BuildHeldOutRunReadinessRefusal` is the pre-provider counterpart to the
post-run seal. It revalidates the exact held-out partition and the closed owner
projection against an independently supplied package-inventory digest, then
records the terminal ledger, controlled-relation StudyRecord, multi-arm
execution/budget binding, current route attestations, exact live authorization,
and verified private capsule family as required gates. The checked-in current
receipt reports one passed gate, the `revision_required` owner gate as blocked,
and six unavailable gates. It fixes `run_authorized=false`,
`execution_permit_issued=false`, `provider_calls=0`, `empirical_units=0`, and
`network_required=false`. The object deliberately cannot become a positive
permit; a future executor requires a separate permit contract built only after
the missing per-arm request, budget, route, authorization, and capsule bindings
exist.

`SealHeldOutRun` accepts no prior seal and requires every one of the 440 supported
test cells to be executed, every unsupported cell to retain its structural state,
every supported test summary to be adjusted-complete, and every violation to be
accounted as witness-bound or witness-missing. A second invocation with an
existing seal fails as `locked_partition_already_used`. `RouteHeldOutDiscoveries`
accepts only executed violated test cells, binds their result and discovery
evidence, rejects a current-catalog ID or digest, and targets exactly catalog v2.
Provider-free tests execute the full 440-cell path to prove these mechanics. They
use synthetic admission and deterministic replay providers, so they are not the
real held-out research run. The committed owner attestation remains
`revision_required`, no real run seal is persisted, and no empirical held-out
claim exists.

`SealReplayResult` derives required constraint observations from the real shared
verification result: absolute score movement for one-trajectory relations and
selected trajectory digest for the candidate-order relation. It derives the
completed repeat count from retained criterion or pair-decision evidence and
the provider-call count from both sides. Evidence policy is checked against the
actual extracted evidence mode, so judge output cannot be relabeled strict and
strict output cannot be relabeled judge. `RunZeroCostArm` uses the existing
`internal/baseline` selectors and performs the same fixed repeat contract with
zero provider calls.

The protocol arm currently composes four independently checked boundaries: the
The normative protocol corpus has identical direct and real NDJSON-subprocess case
results; the existing lineage adapter-conformance report binds trace mapping;
the application extension itself produces the same result digest and run
fingerprint in direct and real NDJSON-subprocess execution over a canonical
Agent Trace source; and the relation replay executes through
`protocol.application` on the shared verification service. The application
boundary rejects non-canonical inline JSON, disables cache use, and excludes
elapsed wall time from its otherwise exact budget-usage digest. This proves
transport conformance plus the application service and relation paths, not one
end-to-end subprocess execution of the full stress corpus. The protocol proof
therefore fixes `provider_calls=0` and
`empirical_reliability=false`; empirical arm comparison remains blocked by the
committed `revision_required` owner attestation.

The evidence-reliance analysis treats the three sealed construct-repair cases as contamination
controls only. `generic_completion_evidence_role`,
`pathological_executable_text_presentation`, and
`shared_tool_transaction_reorder` retain their closed
`unverified_evidence_role`, `unnatural_formatting`, and
`transaction_dependency` defects and are rejected as cross-version evidence.
Every V3 replay now also binds its mutation-program version, relation-contract
version, formal-witness digest, and construct-firewall digest. V1 programs, V2
relation contracts, missing firewalls, or foreign witnesses fail before
reliance admission or denominator accounting. This proves exclusion of the
known defects, not human validity of the corrected relations.

The evidence-reliance path now has one production binding over those shared ports.
`RunRelationBackedIntervention` validates the sealed reliance admission and exact
rendered V3 replay sides, requires the frozen study, cell, relation, outcome,
cache, and offline lineages, calls `ReplayFirstRunner`, and seals the existing
stress result without introducing another verification path.
`ReduceRelationBackedIntervention` reproduces that execution before delegating
to `ReduceCounterexample`; the counterexample source and original oracle
observation must bind the full replay digest. The exact-replay integration test
rejects trajectory substitution before the provider seam, produces a
one-minimal evidence-factor witness through the shared reducer, and prevents a
V1 replay from reaching the reduction oracle. This is mechanism evidence over a
synthetic replay source, not an empirical reliance result.

The counterexample document is deliberately distinct from the controlled-corruption syntactic
changed-region witness. `ReduceCounterexample` sorts bounded units, removes
exactly one unit from an immutable retained input, revalidates the relation and
privacy proof, replays the violation oracle, accepts only when all three remain
true, rejects any input mutation by the remover or oracle, embeds every validated
observation, and restarts after every accepted removal. The final pass records one
rejected attempt for every retained unit, proving deterministic one-minimality
over the declared unit set. The contract does not claim a global minimum.

The committed development case study applies that exact reducer and the
canonical v3 candidate-order sensitivity relation to the checked-in MIT
fixtures `scripts/tests/sample-task.txt`, `sample-traj-a.txt`, and
`sample-traj-b.txt`. The first-listed control selects trajectory A before order
reversal and trajectory B afterward, so the relation is violated. The reducer
removes 30 of 32 line units and leaves only line 13 from each trajectory, `5`
and `-1`, a 93.75% reduction with a completed one-minimality pass. The canonical
content digest is
`b4a7adb23505a88ee0ab79c73957bce5fbd15539a0e5e687590719d3ec4ee84b`.
`eval/results/stress-development-case-study-v1.json` is the strict machine
artifact; `stress-development-case-study-v1.md` and
`stress-development-case-study-v1.svg` are deterministic human projections.
The SVG is 1400 by 900 pixels in its intrinsic vector coordinate system, 8,634
bytes, and has SHA-256
`fa33c78c2fe77fded64732d556808f12af8a613a00cca055f207d79f08a456c3`.
The JSON declares `data_role=adapter_development`,
`status=mechanism_demonstration`, `empirical_units=0`, `provider_calls=0`, and
`network_required=false`; both projections visibly retain the mechanism-only,
zero-provider, and zero-empirical boundary. They prove a deterministic negative-
control and shared-reducer mechanism, not verifier reliability, provider
comparison, held-out confirmation, population generalization, or a global
minimum.

The compact
`eval/results/stress-development-challenge-v1.json` embeds those exact four
licensed fixture files and the canonical case commitments. Running
`stress verify-development-challenge` from an empty working directory rebuilds
the same complete case and reducer trace, then executes one positive control and
six adversarial guards covering fixture-byte tampering, expected-case
substitution, fixture reordering, unknown fields, trailing JSON, and challenge-
digest substitution. The tracked deterministic receipt records all seven guards
passed, four fixtures verified, 53 attempts, 30 accepted removals, two final
rejections, `empirical_units=0`, `provider_calls=0`, and
`network_required=false`. This closes repository-fixture portability for the
named mechanism. It does not supply an independent verifier implementation,
held-out confirmation, provider evidence, population generalization, or a
global-minimum proof.

## Controlled Relation Construct Audit

`internal/relation` governs the human construct-validity question independently
from single-trajectory outcome adjudication and verifier performance. Every
artifact binds `review_objective=controlled_relation`; outcome artifacts reject
as relation inputs and relation sample commitments reject as outcome inputs.
The provider-free foundation does not create human labels or authorize external
review.

| governed artifact | frozen evidence |
|---|---|
| historical relation audit plan v1 | digest `76fe626f3e5ba8bfd12158daf72290eff53aab6bb28c3f114db8aed592f61f91`; immutable defect-discovery contract retained for reproduction only |
| relation audit plan v2 | digest `dcaba51fcf41b8c3eeb6cedd03ec4646ac6cb80f3457d589122ff4bd82fe4bf0`; seven axes, 12 reasons, eight families, exact v2 release/spec/mutation-program/construct-audit bindings, two primaries, one disagreement-only tie-breaker |
| primary sample v2 | digest `5734229be49d6e08ad4b63ed24b1e57ec91074ed2fc45bf598314b51782a3076`; 32 cases, 36 sources, 32 unique task groups, 24 reported lineage clusters, exactly four cases per family, 16 calibration plus 16 test, full construct-firewall commitment |
| development pilot v2 | digest `cdaf1a45f81e6f978a566e3d5256fa203409384aa0df07338032ad2ceee14383`; eight development cases, nine sources, eight task groups, eight lineage clusters, one case per family, zero primary source/task-group/lineage overlap |
| study amendment v2 | digest `107aa68e5699fbc0cb8aa4d1924b892e4500a84de7b9b1aeaec7bddfd8368dbc`; exact pilot and primary digests, 32-group primary estimand, explicit 24-lineage dependence ceiling, 64 primary labels, maximum 32 tie-break labels, 64 probes, unresolved retention, no replacement, fixed stopping, `not_run`, `not_authorized` |
| corrected owner package v4 | local package-summary digest `400377b7cac0e0d40256786cb1dda7377489b113699929dfd75f546b864231e1`; package format v3, relation protocol v2, fresh distinct keys, exact per-case construct-firewall commitment, independently verified, owner inspection not completed, human study not run, external action not authorized |

The v2 primary selector jointly solves 16 family/split buckets and pilot
feasibility. It takes exactly two calibration and two test cases per family with
globally unique task groups, then accepts that lexicographic solution only when
one development case per family remains source-, task-group-, and lineage-
disjoint. This removes development contamination from the primary audit and
prevents primary selection from consuming the only valid pilot units.

The corrected prelaunch chain now propagates relation protocol v2 through replay
receipt, case material, blind packet, private mapping, qualification set and
answer key, handbook, bundle, readiness, change receipt, inspection, dossier,
and package format v3. Material and mapping records bind source spec, mutation
program, construct audit, relation contract, evidence boundary, and exact case
firewall; readiness commits the complete eight-firewall set. The verifier rejects
legacy or cross-version substitution while all historical package bytes remain
independently reproducible. Reviewer records through terminal ledger remain v1
until post-inspection relation propagation is complete and cannot yet execute a
corrected human pilot.

Seven trajectory-level families use one original/transformed trajectory pair.
`candidate_order_reversal` uses two visible candidate-order pairs and cannot be
collapsed into one mutated trajectory. Review observations are normalized to
the original/transformed identity only after commitment. Required axes are a
family-specific subset of semantic task quality, evidence strength, executable
outcome support, presentation equivalence, causal-integrity preservation,
untrusted-content authority, and information sufficiency.

`relation translate` requires every governed axis exactly once and rejects
unknown, missing, duplicated, misordered, or outcome-style ratings. A complete
support rule with no contradiction yields `supports`; any contradiction rule
yields `contradicts`; indeterminate, insufficient, not-applicable, simultaneous
support/contradiction, or unmatched evidence yields `unresolved`. The result is
content-addressed and contains matched axes and typed reasons, never verifier
scores or desired conclusions.

`relation replay` resolves only the frozen case's one or two repository-relative
source locations, rejects symlink escape and source identity/revision drift,
reapplies the exact frozen mutation program, reduction, manifest, blind packet,
and regeneration key, and returns a content-addressed replay receipt. The raw
`ReplayedMaterial` remains internal for the next license-aware materialization
stage; the CLI emits only digests and unit metadata. Candidate-order replay
proves the original and transformed digest arrays are exact reversals.

`relation materialize` turns the internal replay into
`evalwitness.relation-case-material.v1` under a fixed 16,000-token budget per
source trajectory. It includes one coherent redacted task requirement, requires
the changed event plus a final patch or assistant-narrative anchor, aligns
retained event lineage across original and transformed sides, and records exact
source/retained/omitted event counts. Candidate-order material reuses the same
two excerpts and reverses only their order. Every upstream excerpt retains SPDX,
source URL, revision, and `reference_only` status and is forced to
`restricted_reference_only` with `public_releasable=false`.

`relation packet` removes the case, family, expected relation, split, control,
source identities, source locations, provider identity, mutation/validator
evidence, key identity, and private order keys from reviewer-visible material.
It exposes only the coherent task, two fixed visible positions, aligned redacted
excerpts, exact evidence accounting, SPDX/redistribution class, limitations,
the complete neutral seven-axis rubric, and order commitments. Length-prefixed
HMAC-SHA-256 domains independently derive packet ID, task alias, visible-side
identity and direction, evidence-slot identity, candidate labels, packet order,
and reviewer-assignment order. Candidate reversal preserves two exact reversed
orderings while giving every panel-local candidate an independent opaque label.
The mapping binds all hidden source, direction, family, witness, replay, and
randomization evidence; its canonical digest is its filename, publication is
exclusive, and the owner-only file mode is `0600`.

`relation pilot-change-receipt` verifies the sealed readiness record against the
complete restricted bundle and owner-only mappings, then emits
`evalwitness.relation-pilot-change-receipt.v1`. The content-addressed restricted
receipt contains no task or trajectory body. It binds readiness, bundle and
mapping commitments; packet/case/family/relation identity; original/transformed
content and changed-line digests; exact line and event denominators; paired
lineage; candidate reversal; structural hazard flags; and explicit
no-decision/not-run/not-authorized status. Package format v2 includes the receipt
and its schema. The verifier retains separate inventory contracts for historical
v1, the pre-receipt v1 atlas extension, and receipt-bound v2.

`relation render-pilot-change-atlas` verifies the same custody before rendering
a review aid. Package v2 additionally requires the exact change receipt and
binds its digest into the atlas. Every trajectory pair binds the complete task,
both content
digests, exact unchanged prefix/suffix line counts, every remaining original and
transformed rendered line with bounded context, retained/source/omitted event
counts, and hidden logical alignment. Candidate reversal is independently
checked for exact content preservation in reverse source-candidate order. A
causal-reorder window containing an event-reference wrapper is explicitly
flagged for manual review. The atlas contains empty decision matrices only; it
does not replace the complete workbook, infer semantic validity, create a human
result, or authorize external action.

The relation-only review workflow adds eight owner-key-randomized supervised
qualification cases covering pair direction, evidence-only degradation,
presentation invariance, causal reordering, deceptive evidence, untrusted
authority, ambiguity/insufficiency, and candidate-order reversal. The public set
contains no answers or explanations; its exact answer key is content-addressed,
published once under mode `0600`, and required through a private regular-file
boundary. Qualification requires at least seven of eight exact cases plus both
mandatory ambiguity and candidate-order cases. The frozen handbook defines all
seven axes, allowed ratings, 12 reasons, applicability, conflicts, blinding,
submission, data, labor, privacy, and generalization rules without exposing a
formal relation. Conflict-free qualified primary reviewers receive independently
seeded full-bundle packet orders. Assignments and kits remain
`planned_not_shared` with `external_action_status=not_authorized`; each kit is
self-contained canonical JSON plus deterministic Markdown that places every
task and evidence body behind a fence longer than any embedded backtick run.

Raw relation review now uses `evalwitness.relation-pair-judgment.v1`. Every
judgment binds the plan, bundle, assignment, packet digest, reviewer slot,
qualification, rubric, all seven visible axes, canonical reasons, submission
time, and an immutable revision number plus parent digest. Directional,
indeterminate, and insufficient ratings require compatible reasons. A
`evalwitness.relation-judgment-batch.v1` commitment accepts exactly one latest
revision for every assigned packet, sorts by packet ID, rejects partial or
cross-reviewer coverage, and must occur strictly after every submission.

The corrected post-inspection chain is protocol-version closed. Reviewer record,
assignment, kit, pair judgment, judgment batch, prereveal ambiguity, condition
probe and batch, mapping reveal, translation result, relation resolution,
formal-human comparison, and terminal ledger each have immutable v1 plus v2
schema identities. Builders derive the version from their trusted plan or bundle;
validators reject any nested parent or child from another generation before
distribution or analysis. `relation reviewer --protocol-version v2` creates the
only root artifact in that chain that has no parent from which to inherit the
version. A complete synthetic v2 two-primary/one-tie workflow proves the path
through terminal custody without representing a participant or empirical result.

After both independent primary batches are complete,
`evalwitness.relation-prereveal-ambiguity-analysis.v1` embeds and reproduces the
exact committed judgments without private mappings. It reports 14 axis
comparisons per two-packet fixture, per-axis left/right rating prevalence,
disagreement and 95% Wilson intervals, unclear and not-applicable rates,
reason-code exact match, zero overlap, Jaccard distance, and the exact tie-break
packet set. Any axis or reason-code difference requires tie-break review. The
slot-three assignment is created only after this analysis, excludes both primary
reviewers, uses a separate owner seed/order, contains only disagreement packets,
and remains `planned_not_shared` with
`external_action_status=not_authorized`. Insufficient evidence stays admissible
and cannot be majority-voted into support.

Each primary reviewer next commits one post-label probe per packet. The probe
binds the exact judgment digest and records a bounded family guess, visible-side
direction guess, source-control guess, task recognition/basis, optional private
identity guess, confidence, and submission time. The complete probe batch must
follow its judgment batch and precede reveal. The candidate universes are frozen
as eight governed families, three direction values, three control conditions,
and an explicit `unknown`; probe results cannot revise a judgment.

`evalwitness.relation-mapping-reveal.v1` is buildable only after both complete
primary probe commitments and every disagreement-only tie-break batch. It
reproduces the prereveal ambiguity digest, all assignment/batch/probe digests,
the full packet-to-mapping reference set, and every assignment order from its
disclosed 32-byte seed before sealing the reveal actor/time. Full private mapping
contents remain separate owner inputs. The provider-free gate rejects an early
reveal and verifies a complete synthetic three-assignment reveal without
authorizing external sharing.

`relation compare` consumes that reveal plus the exact private mappings,
prereveal ambiguity, two primary batches and probes, and any required tie-break
batch. It reconstructs the left/right-to-original/transformed bijection, retains
all seven reviewer axes, and applies
`information-insufficiency-veto_then_primary-agreement_then_tie-majority_else-indeterminate.v1`.
An insufficient-information, multi-factor, hidden-context, or ambiguous-task
caveat forces the information axis unresolved; a majority cannot erase it.
Every `evalwitness.relation-resolution.v1` binds the formal witness and expected
relation, reviewer judgment digests, frozen translation contract, normalized
axes, human admissibility state, and `verifier_relation_status=not_consulted`.

`evalwitness.relation-formal-human-comparison.v1` retains exact support,
contradiction, and unresolved counts across family, intervention class, split,
control, and source-task group. It also seals post-label family/direction/control
guess confusion, task recognition, conservative task-cluster Wilson intervals,
and strict/best/worst unresolved sensitivity. `relation terminal-ledger` then
binds every packet resolution into `evalwitness.relation-terminal-ledger.v1`.
The ledger keeps formal relation, human construct judgment, and verifier result
as distinct layers; the verifier layer remains explicitly `not_consulted`.

The prospective v2 primary endpoint is source-task-group prevalence of formal-
human contradiction. All 32 cases require two primary judgments; at most 32
disagreement-only tie-break judgments and 64 post-label probes are allowed.
With 32 effective task groups and zero contradictions, the one-sided exact 95%
upper bound is 0.08937. Detection probabilities for at least one contradiction
at true rates 0.05, 0.10, and 0.20 are 0.8063, 0.9657, and 0.9992. These figures
are prospective diagnostics under the declared non-probability-sample and
lineage-dependence limitations, not observed results or population guarantees.

```bash
go run ./cmd/evalwitness relation schema --type plan
go run ./cmd/evalwitness relation plan-v2 \
  --release @controlled-corruption-v2-release.json
go run ./cmd/evalwitness relation schema --type primary-sample-v2
go run ./cmd/evalwitness relation schema --type terminal-ledger-v2
go run ./cmd/evalwitness relation validate --type pilot-sample \
  --document @eval/governance/relation-pilot-sample-v2.json
go run ./cmd/evalwitness relation replay --root . \
  --release @controlled-corruption-release.json --case-id mutation-<digest>
go run ./cmd/evalwitness relation materialize --root . \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --release @controlled-corruption-release.json --case-id mutation-<digest>
go run ./cmd/evalwitness relation packet \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --release @controlled-corruption-release.json --material @case-material.json \
  --key-file /secure/relation-key.hex --key-id relation-study-key \
  --private-root /secure/relation-mappings
go run ./cmd/evalwitness relation qualification \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --key-file /secure/qualification-key.hex --key-id relation-qualification-key \
  --private-root /secure/relation-qualification
go run ./cmd/evalwitness relation translate \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --family omitted_test_evidence --observations @observations.json
scripts/audits/run-relation-construct.sh
scripts/audits/run-relation-governance-v2.sh
```

The audit regenerates the complete controlled corpus when fetched sources are
present, reproduces both samples and the amendment byte-for-byte, independently
checks primary selection identity and pilot disjointness, validates all 31
schemas, replays and materializes both trajectory-pair and candidate-pair-order
units, builds both blinded packet types, reproduces all seven HMAC domains,
checks content-addressed mode-0600 non-overwriting mappings, source/family/key
leakage, license/visibility, exact omission accounting, aligned selection, pair
reversal, eight-case supervised qualification with owner-only answer custody,
prepares one immutable eight-family owner package with distinct packet and
qualification keys, verifies its SHA-256 inventory and 0700/0600 custody,
independently reproduces bundle/readiness/not-launchable-dossier/public-safe-
launch-brief/pilot-change-receipt/owner-change-atlas/workbook bytes, scans the
brief under the public
artifact policy,
binds the exact 64-action maximum workload plus every unresolved owner decision
and unauthorized external action, rejects replacement,
immutable judgment revision lineage, partial-batch rejection, strict
post-submission commitment times, two complete synthetic primary batches,
prereveal seven-axis/reason disagreement reproduction, exact denominators, and
one independent disagreement-only synthetic tie-break assignment,
the frozen handbook, three synthetic qualified reviewer records, exact
reviewer-specific assignment order, three complete JSON kits, injection-safe
Markdown, independent ordering commitments, post-reveal normalization,
construct-caveat and information vetoes, exact stratified denominators,
denominator-deletion rejection, all-eight-packet structural pilot readiness,
owner-only all-difference and hidden/visible inspection rendering, exact inspection-decision reason
gates, synthetic pass-path custody that remains non-empirical and unauthorized,
packet resolutions, and terminal-ledger custody.
It exercises every terminal translation class, proves mutual objective
isolation, scans public artifacts, and invokes no provider. Current empirical
status is `not_run`; current external action status is `not_authorized`.

## Coding-Agent-Only Formal Study

The repository ships a complete provider-free empirical path for the
coding-agent-only scope. It evaluates formal controlled-relation validity on a
frozen subset of the v3 controlled-corruption release without human reviewers,
LLM calls, or external actions. The historical human construct-validity and
natural semantic-outcome workflows remain separate contracts and are not
silently relabeled as machine evidence.

| artifact | contract |
|---|---|
| study | `evalwitness.agent-only-study.v1` |
| canonical policy | `evalwitness.agent-only-canonical-json.v1` |
| frozen artifact | `eval/governance/agent-only-study-v1.json` |
| schema artifact | `eval/governance/agent-only-study-schema-v1.json` |
| selection | `family-quota-hash-order.v1`, 20 calibration plus 20 test cases, seven core families, zero cross-split task-group overlap |
| independent validators | `formal-release-validator-a.v1` and `formal-release-validator-b.v1` |
| conflict path | disagreement-only `formal-release-tie-break.v1`; unresolved is retained and never replaced |
| current result | 40/40 formal cases accepted, 40/40 primary agreements, zero tie-breaks, zero unresolved |
| custody boundary | provider calls `0`, human reviewers `0`, external action not required |

Each selected case records the exact release manifest, firewall, blind-packet,
source, trajectory, and outcome-witness digests. Primary A executes the sealed
mutation validators and binding checks. Primary B independently recomputes the
canonical manifest, witness, packet, and firewall digests and rechecks source
and outcome bindings. A disagreement invokes the deterministic tie-break and is
preserved in the case record; no majority or replacement rule can erase an
unresolved result.

The machine-only estimand is formal relation integrity on the selected frozen
release cases. It does not estimate human semantic validity, provider quality,
transfer, prevalence, or population/generalization performance. The exact
artifact digest is `e50a428c0af34f8b6ff3c06dd5eb66b44c1073c5999d49f0681ce369b4b9348b`.

Reproduce and validate the complete path:

```bash
scripts/audits/run-agent-only-study.sh
go run ./cmd/evalwitness agent-study validate \
  --study @eval/governance/agent-only-study-v1.json \
  --plan @eval/governance/controlled-corruption-v3-plan.json \
  --audit @eval/governance/controlled-corruption-v3-natural-audit.json \
  --release @eval/governance/controlled-corruption-v3-release.json
```

The gate builds twice, compares bytes, validates all parent bindings, checks
the 20/20 split and zero overlap, runs the independent-validator and tie-break
tests, and explicitly reports zero provider calls and zero human reviewers.

## Outcome Validity and Blinded Adjudication

The shipped outcome-validity layer separates evaluator decisions from task
truth. `internal/outcome` represents six states: `solved`, `unsolved`,
`indeterminate`, `invalid_task`, `environment_failed`, and `not_adjudicated`.
Claimed tests, benchmark reward, independent rerun, formal relation, and human
label remain distinct evidence records with immutable digests, timestamps,
validators, parents, and limitations. A decisive resolution must cite matching
basis evidence; an indeterminate resolution must cite indeterminate or
conflicting evidence. Revisions retain the parent digest and never rewrite the
prior record.

The frozen provider-free governance artifacts are:

| artifact | current evidence |
|---|---|
| adjudication plan | digest `2b7fa309bdf2a4151c640275f858342624fddfbf5d6ada316e5f996ef14dd46e`; two primary reviewers, one independent tie-breaker, 10,000 task-cluster bootstrap iterations |
| mutation audit sample | 31 sealed cases across all eight primary controlled-corruption families; digest `883b0164746c9e6fcaccc07f6fe3d0fd4872cbc35c08983599b48e4f6e605d06` |
| natural-case inventory | 36 unique task groups selected from a target of 48; digest `054a9b1e41a7507b780a8cd74052df510889897253e18f6d7093511e18ea5b8e` |
| mixed-objective design artifact | 14 unique committed cases: one development mutation per each of eight families plus rank one from each of six populated natural strata; 28 hypothetical primary labels, at most 14 tie-break labels, and 28 source probes; digest `76436e72015a55be5bcaa6c83ea1b6fdc053fb01004ca940b4d663d882a6beb0`; reproducible but prohibited from reviewer launch |
| outcome pilot v2 | six natural rank-one single-trajectory units; 12 required primary labels, at most six tie-break labels, 12 source probes; digest `994dbbfb18c40494d1b664f4a6b45ac5b4182dcbd7f7f9293ba1f3e8eea03ff7`; technically ready for explicit authorization only |
| reviewer qualification | five public synthetic cases, passing score 0.80; digest `a2ce822020ac6a3050de35b73ba645741e823369875fe32ef707e2d3143bd2d7` |
| reviewer handbook | frozen outcome definitions, decision procedure, axes, reasons, conflict/blinding policy, submission checklist, and dataset statement; digest `c4047894da6959f38a1f5766e2e7bf0b762567639089aba3cead04e4ef1f1855` |

The natural inventory reads immutable legacy Terminal-Bench and SWE-bench
evaluation results through a bounded adapter. It uses candidate index zero as
the declared baseline, a 0.80 high-confidence-error threshold, rarest-stratum-
first deterministic selection, and global task-group deduplication. Candidate
counts are 64 verifier-correct, 39 verifier-wrong, 20 verifier/judge
disagreements, 46 baseline disagreements, 16 high-confidence errors, and 103
random controls. No legacy record contains an explicit abstention or provider-
failure state, so those prespecified strata have zero eligible rows and the
inventory correctly remains `incomplete` at 36/48. Synthetic replacements or
cross-stratum substitutions are forbidden. These previously accessed tasks are
development evidence, not untouched external validation.

The frozen v1 commitment is a bounded selection and workload diagnostic, not a
valid rubric-usability run and not the full 67-case adjudication pool. It deterministically chooses the first
eligible development mutation in each family while preventing mutation task-
group reuse, then chooses natural rank one in every populated prespecified
stratum. The absent natural `abstention` and `provider_failure` strata remain
explicitly unavailable. Mutation and legacy natural inventories use different
historical identity namespaces, so cross-source task overlap cannot be fully
disproved and remains a declared limitation. More importantly, mutation units
require pairwise relation judgments while natural units require one-trajectory
outcome labels. Its readiness builder therefore always rejects launch. The
governed v2 commitment contains only the six populated natural strata under
`objective=single_trajectory_outcome`; a separate controlled-relation package owns a non-overlapping relation
pilot and formal-human construct audit.

Outcome pilot materialization resolves each selected index against the exact
sorted raw candidate universe and refuses changed reward vectors, missing runs,
or source-artifact drift. Five selected cases resolve to SWE-bench and one to
Terminal-Bench. Every raw item passes strict canonical ingestion and secret
redaction, then the fixed 16,000-token anchored whole-event selector. It must
retain the final nonempty patch, or the last nonempty assistant narrative when
no patch exists, plus at least one action and one result. Required anchors fail
closed when they cannot fit; they are never silently traded for higher-scored
fragments. Each reviewer
unit contains exactly one `task_requirement` and one `trajectory_evidence` slot.
Raw paths, task IDs, selected indices/rewards, source and retained digests,
license/revision, redistribution status, packet ID, and mapping digest are
content-addressed in an owner-only source binding. Because the fetched sources
are catalogued `reference_only`, every real pilot packet and its bundle are
`restricted_reference_only`; the materializer never upgrades them to public.
Exact model/run/trial identities are redacted, and raw source/call references
become deterministic typed SHA-256 pseudonyms before packet sealing.
The owner-only source binding records selector v2, anchor kind/digest, retained
message/action/result counts, and the existing raw/canonical/retained lineage.
This structural floor prevents a tool-only packet from being called ready, but
does not prove that the excerpt is semantically sufficient for a human label.
`pilot-materials` seals every private mapping and source binding into one
`evalwitness.outcome-pilot-private-materials.v1` custody root. The artifact binds
the governed pilot digest, both canonical packet-ordered collections, their
independent digests, and its own digest; it is published exactly once with mode
`0600` below the owner-only root and never shares the restricted review-item
stream.
`outcome pilot-inspect` turns that boundary into a separate sealed artifact. For
all six packets it binds the pilot, restricted bundle, exact mapping commitment,
and owner-only source-binding digests; reproduces source, retained, omitted,
message, action, result, anchor, byte, and token denominators; and records
`reviewability_status=structurally_ready` beside
`semantic_status=requires_human_pilot`. The current full-corpus result retains
259 of 1,108 events, has message evidence in five of six packets, and uses five
final-patch anchors plus one assistant-narrative anchor. These figures describe
the frozen development packet construction, not human evidence sufficiency.
Readiness v3 rejects public packets, relation targets, pair-level evidence,
missing cases, reused task groups, wrong conditions, and wrong objective. A
successful artifact must additionally bind the exact inspection digest and its
mapping commitment, then records `ready_for_authorization` and simultaneously
`external_action_status=not_authorized`.

Public review packets are generated from a private build request with a caller-
supplied 32-byte key. HMAC-SHA-256 assigns separately domain-separated packet,
task-alias, and evidence-slot identifiers. The command prints only the public
packet and atomically creates exactly one non-overwriting `0600` mapping below
the supplied private root. The private mapping retains the original task alias,
source condition, expected relation, key ID, and source digests. Automated leakage
checks reject declared forbidden values plus the original task alias, condition,
relation, key ID, source-case digest, and original slot labels. This is a lexical
fail-closed control, not proof that a human cannot infer a condition from
semantic cues; a development pilot must test that risk.
Public task hashes are also pseudonymous, not anonymous against an observer who
already knows the task universe. Only HMAC-rekeyed packets go to reviewers.

The content-addressed reviewer handbook freezes the outcome definitions,
evidence rules, ordered decision procedure, five rubric axes, all typed reason
codes, conflict and blinding policies, submission checklist, and dataset
statement. The bundle binds its exact digest, not merely a human-readable
version label. The rubric rates task satisfaction, technical correctness,
verification quality, harmful side effects, and evidence sufficiency, then
records one typed primary outcome and sorted reason codes. Reviewer slots one
and two are the independent primaries; slot three is reserved for a tie-break.
`outcome resolve` requires a passing, reviewer-specific qualification report for
every supplied label and refuses duplicate primary slots or reviewer aliases.
The five-case public qualification set covers solved, unsolved, environment
failure, invalid task, and indeterminate evidence. Because its answer key is
public, it is a training and rubric-ambiguity instrument. It proves independent
reviewer competence only when answers were committed before key access or the
exercise was supervised and recorded.

The `outcome review` workflow makes the human-study sequence executable rather
than advisory. A review bundle freezes plan, rubric, qualification set, handbook,
data role, visibility, task clusters, and blinded packets. Each pseudonymous
reviewer record separately seals consent, role, independence, authorship-policy
acceptance, private contact custody, and declared conflicts. Assignment requires
a passing reviewer-specific qualification report, no declared conflict, and a
32-byte secret ordering seed; the assignment binds the rubric and a deterministic
reviewer-specific packet order while publishing only the seed digest. Every label
must use that rubric, reviewer, slot, and qualification, occur after assignment,
contain no unresolved conflict, and cover the assignment exactly once before the
batch can be committed.

After assignment, `outcome review kit` creates one self-contained,
content-addressed artifact containing the exact handbook, reviewer-specific
assignment, and assigned blinded packets in committed order. It contains no
private mapping, ordering seed, evaluator output, peer label, tie-break result,
condition, or expected relation. `verify-kit` reproduces the kit against the
sealed bundle; `render-kit` requires the same bundle proof and deterministically
emits a human-readable Markdown handbook, dataset statement, evidence packet
sequence, rubric questions, and submission procedure while neutralizing
packet-controlled Markdown and HTML. This makes review operational without
inventing labels or requiring a reviewer to understand repository internals.

`outcome review pilot-readiness` proves the outcome-only v3 identity and coverage
contract after all six blinded packets and the structural inspection exist. It
requires exact one-to-one source-case and task-group coverage across the pilot
commitment, restricted development bundle, owner-only mappings, governed
qualification and handbook, pre-assignment blinding protocol, and inspection.
Every mapping-consuming review command accepts either that sealed private-
materials custody root or a separately managed compatible mapping array, never
both. The sealed root is the canonical pilot path and avoids unaudited manual
extraction.
The inspection must bind the same bundle and mapping commitment, report every
packet structurally ready, and retain `semantic_status=requires_human_pilot`.
The readiness artifact exposes mapping references and their aggregate digest,
never mapping contents. A valid result is
`technical_status=ready_for_authorization` and
`external_action_status=not_authorized`; it cannot represent recruitment,
consent, qualification, assignment, labeling, adjudication, or packet delivery.
The historical mixed v1 readiness builder always rejects launch. Readiness v3
proves identity, protocol, and structural-reviewability bindings, not that a
redaction or excerpt faithfully preserves every task-relevant semantic detail;
only the independent development pilot can establish that.

`analyze-rubric` runs after both primary label batches are committed and before
the mapping reveal. It embeds the exact sealed slot-one and slot-two labels for
each packet, verifies full bundle coverage, and reports primary-outcome and any-
axis disagreement with Wilson 95% intervals. The five axes retain separate
disagreement counts, intervals, unclear rates, and left/right rating prevalence.
The same artifact reports primary-state prevalence, indeterminate-label rate,
overall unclear-rating rate, exact and zero-overlap reason-code counts, and mean
reason-code Jaccard distance. These are diagnostic disagreement fingerprints,
with Wilson intervals for every binary proportion, not proof that one reviewer
or the rubric is wrong. The artifact remains under
study-owner control until disclosure is permitted because it contains peer
labels. Terminal adjudication recomputes its label digests against the two
committed batches and rejects analysis at or after reveal.

Lexical leakage checks are not accepted as evidence that semantic blinding
worked. Before assignment, `blinding-protocol` freezes the complete hidden
condition vocabulary from owner-only mappings without exposing packet-to-
condition links. After each primary label batch is committed, that reviewer
must complete one source-condition guess per assigned packet or explicitly use
`unknown`, record confidence, and declare whether the task was recognized and
why. `blinding-probe-batch` binds those answers to the exact assignment, label
batch, label digest, reviewer, packet coverage, protocol, and post-label time.
`analyze-blinding` runs only after reveal and rejects probes committed after the
reveal. It reports complete denominators, guess coverage, raw and selective
accuracy with Wilson 95% intervals, marginal expected-chance accuracy, Cohen
kappa, mean confidence, attempted-guess Brier score, recognized-task rate, and
condition/reviewer/item breakdowns. These metrics detect whether reviewers could
infer hidden study conditions; they do not prove that a cue changed a label or
that recognition was misconduct. The terminal adjudication ledger is invalid
without the exact analysis digest.

The two primary batches must bind distinct reviewers in slots one and two. A
slot-three assignment can be created only after both primary commitments and
contains exactly their disagreements. Mappings and ordering seeds can be revealed
only after every required batch is committed. The reveal replays each HMAC-ranked
assignment order from its disclosed seed, binds paired assignment/batch
commitments, covers every packet mapping exactly once, and occurs strictly after
the latest commitment. The final adjudication ledger binds the prereveal rubric-
ambiguity analysis, reveal, post-reveal semantic-blinding analysis, agreement,
all resolutions, unresolved packet IDs, and
completion time. Missing labels,
early tie-breaks, early reveals, wrong seeds, rubric drift, reviewer reuse,
conflicts, or partial mappings fail closed.

Only after that terminal ledger exists can `outcome review analyze-sources`
join the owner-only mapping, one sealed pre-human outcome record per source task,
and the exact ledger resolutions. The command has no verifier-score, decision,
selection, factor-assignment, or desired-result input. It reports coverage and
within-source conflict for claimed tests, benchmark rewards, independent reruns,
formal relations, and human adjudication; all ten pairwise source comparisons;
Wilson agreement intervals; task-cluster bootstrap Cohen kappa; packet-level
contradictions; and benchmark-to-human transition counts. A source with multiple
states, or only `not_adjudicated`, remains ambiguous and is excluded from the
pairwise comparable denominator. Agreement is consistency, never proof that one
source is correct. The terminal human outcome is immutable input to this derived
analysis, so verifier performance cannot choose the correction.

| Review stage | Public artifact | Private material retained by the study owner |
|---|---|---|
| Bundle freeze | plan/rubric/handbook/set digests, role, visibility, blinded packets | source evidence not licensed for release |
| Reviewer onboarding | pseudonymous consent and independence record | identity and contact details |
| Assignment | reviewer-bound qualification, slot, packet order, seed digest | ordering seed until all required label commitments exist |
| Reviewer kit | exact handbook, assignment, ordered blinded packets, deterministic Markdown rendering | source mapping, ordering seed, peer labels, and study condition |
| Label commitment | complete content-addressed batch | no source mapping or evaluator output |
| Rubric-ambiguity analysis | outcome/axis disagreement, rating prevalence, unclear/indeterminate rates, reason-code divergence, embedded sealed labels | study-owner only until peer-label disclosure is permitted; packet mappings remain concealed |
| Blinding probe | complete post-label condition guesses, confidence, recognition basis, protocol and label bindings | packet-to-condition truth remains concealed |
| Reveal | assignment/batch pairs, disclosed ordering seeds, mapping digests, actor and time | private mapping contents unless separately releasable |
| Semantic-blinding analysis | coverage, accuracy intervals, chance correction, confidence error, recognition, condition/reviewer/item results | private mappings used only to score committed guesses |
| Adjudication | agreement, rubric-ambiguity and blinding-analysis digests, resolutions, unresolved set, final ledger | restricted evidence governed by the bundle visibility |
| Source reconciliation | per-source coverage/conflict, all source-pair agreement, benchmark transitions, exact evidence and terminal-resolution bindings | owner-only mappings and any restricted pre-human records used to build the public derivative |

Independent executable evidence enters through the trusted hermetic-validator
registry, never from a trajectory-selected command. The registry pins an
absolute executable, literal arguments, sorted environment, disposable
network-disabled workspace, timeout, output ceiling, passing exit code, and
contract digest. A normal nonpassing exit maps to `unsolved`; timeout, output
limit, launch failure, or cleanup failure maps to `environment_failed` with a
hashed failure detail. Complete execution logs are content-addressed. The
library intentionally exposes no arbitrary-command CLI surface.

Agreement reports raw agreement, both reviewers' label prevalence, Cohen's
kappa, and percentile intervals from 10,000 deterministic task-cluster bootstrap
replicates. Kappa is explicitly undefined when prevalence makes expected
agreement one, and the number of defined bootstrap replicates is reported. Raw
agreement and prevalence are always co-reported, so no coefficient is treated
as sufficient by itself. The chance-corrected measure follows
[Cohen (1960)](https://doi.org/10.1177/001316446002000104); the clustered
bootstrap is the prespecified resampling implementation, informed by published
bootstrap/jackknife work on agreement intervals
([DOI 10.1371/journal.pone.0019539](https://doi.org/10.1371/journal.pone.0019539)).

Outcome preservation permits an evidence-only intervention only when
source and intervened records share the task lineage and the same decisive
solved/unsolved state. Changed, invalid, environment-failed, indeterminate, or
lineage-mismatched outcomes are inadmissible with explicit reasons. Later
calibration and reliability reports must retain original and adjudicated
outcomes as separate sensitivity views.

Reproduce the current local evidence without a provider:

```bash
go run ./cmd/evalwitness outcome plan --plan @eval/governance/outcome-adjudication-v1.json
go run ./cmd/evalwitness outcome validate --type pilot-sample-v1 --document @eval/governance/outcome-pilot-sample-v1.json
go run ./cmd/evalwitness outcome validate --type pilot-sample --document @eval/governance/outcome-pilot-sample-v2.json
go run ./cmd/evalwitness outcome natural-inventory --plan @eval/governance/outcome-adjudication-v1.json --request @eval/governance/outcome-natural-inventory-request-v1.json
go run ./cmd/evalwitness outcome qualification --set @eval/governance/outcome-qualification-v1.json
scripts/audits/run-outcome-validity.sh
```

The implementation, governed artifacts, executable reviewer workflow, and
provider-free gates establish a complete owner-only adjudication path. No real
reviewer labels, tie-breaks, agreement estimate, disagreement taxonomy,
benchmark-transition result, or adjudicated-outcome sensitivity result is part of
the current package. Recruiting, contacting, scheduling, or sharing packets with
reviewers requires explicit user authorization; public claims therefore say
“blinded-adjudication infrastructure,” not “independently adjudicated
benchmark.” This is an intentional scope boundary, not a missing prerequisite
for the shipped product or the admitted identical-response estimand.

## Security and Privacy

API keys read only from env, `.env` files, TOML, or JSON config. Never accepted via CLI flags. The structured stderr handler recursively redacts messages, bound and per-record attributes, groups, errors, URLs, headers, log-valuers, maps, and slices. Keys never enter cache, fixture, audit, capability-probe, or Best-of-N artifact files. Cache roots, archive staging, extracted datasets, audit parents, replay-capture parents, budget-state parents, and Best-of-N run directories use `0700`; their sensitive files use `0600`. Best-of-N child processes receive only the base allowlist plus explicit `--pass-env` names, reject literal secret-bearing arguments, bound transcript and diff output, and are terminated as process groups. Public mode is a separate explicit artifact classification and is never inferred from a filename or destination.

`evalwitness artifact scan --class public|sensitive --path PATH` is the shared fail-closed publication gate. It reports rule, relative path, line, complete-file SHA-256, and separate text/opaque file counts, never the matching value. Text files are checked for built-in credential shapes, exact secret-bearing environment values, environment dumps, and private workstation paths. Opaque files are checked byte-for-byte only for registered secret-bearing environment values because compiled binaries legitimately embed the scanner's own generic detection patterns. Unsupported types, symlinks, and class-specific modes still reject under file/count/byte/depth/finding limits. Release admission therefore combines the public candidate scan with the curated tracked-source safety gate, exact source-archive, portable-source, and module-proxy binding, VCS-independent embedded build settings, and byte-identical repository and archive-only builds; it does not misrepresent generic regex scanning of an executable as a complete binary secret audit. Synthetic research fixtures can be accepted only through a strict, public, exact-content reviewed-findings manifest. The manifest is sorted, schema-versioned, non-writable, and pins rule/path/line plus full-file digest; stale, missing, changed, or additional findings fail.

Protocol, trace, attribution, capsule, policy, and static-report readers share one hostile-input envelope with byte, depth, node, string, array, object, markup, and link bounds. Inputs cannot select a command, environment, working directory, network destination, output path, or live mode. Offline report links remain relative, query-free, scheme-free, host-free, and contained.

Trajectory data redacted before selection and before any provider call (opt-out only via explicit `EVALWITNESS_NO_REDACT=true`). Transcript content is never executed by ingestion or fidelity tooling. No telemetry; only the configured LLM provider is contacted. `EVALWITNESS_OFFLINE=true` refuses any network call. HTTPS-only by default; `EVALWITNESS_ALLOW_INSECURE=true` required for plain HTTP (Ollama dev).

### Owner-only material and public repository boundary

Credential-rotation notes, private transcript exports, provider account data,
custody roots, and reviewer packets remain outside the public source tree under
ignored owner-only paths. The repository ships no API keys, cookies, login
material, personal chat exports, workstation paths, or operational logs. The
public artifact scanner and release gates reject secret-bearing values, private
paths, unsafe modes, and unreviewed findings; local incident history is not part
of the product narrative or empirical claim surface.

## Release and License

Local release build:

```bash
scripts/tests/run-tests.sh
scripts/evals/reproduce-public-evidence.sh --profile full
scripts/build/build-release-candidate.sh --destination /absolute/new/path/outside/repository
```

`internal/product/version.go` is the single product-version source. The CLI,
provider user agent, MCP server identity, build provenance, and release tag gate
consume the same canonical SemVer value. The source currently declares `0.2.0`;
that is release-candidate identity, not evidence that a tag or downloadable
release exists.

`scripts/build/build.sh` emits `-buildvcs=false`, `-trimpath`, and a release-only
project no-inlining profile (`github.com/Christopher-Schulze/evalwitness/...=-l`)
while leaving the standard library and external dependencies optimized. It
produces static binaries for darwin/linux amd64/arm64 and windows amd64. The manifest owns commit
identity so the same source archive can reproduce the bytes without a hidden Git
object database. `scripts/build/build-release-candidate.sh` requires a clean
commit and assembles those exact five binaries with a PAX-free deterministic
USTAR+gzip source archive, its canonical commit/format/inventory/digest report,
manifest-bound portable clean source-tree provenance, an exact six-module `file://` Go
proxy, public capsule, controlled evidence, protocol material, offline explorer,
documentation, and replication material under seven closed roles. The source
builder accepts only the clean current HEAD, reads exact Git blobs, preserves only
regular and executable modes, publishes without overwrite, and rejects
nonportable tracked paths. Manifest construction and verification independently
recheck the archive report, strict USTAR headers, and portable source identity.
With an empty module/build cache and `GOPROXY` restricted to that release asset,
the round-trip gate safely extracts the archive, verifies every source byte from
the manifest-bound index without Git history, runs the Go tests while recording
whether the separately distributed trajectory cache is present, and runs the
public-source scan, rebuilds the host binary byte-identically, and reproduces the capsule,
claims, challenge pack, claim surfaces, Claim Autopsy, and explorer. The candidate
then emits a canonical release manifest, SPDX 2.3 SBOM, in-toto release Statement,
round-trip report, complete checksums, and verification report. An
optional existing mode-`0600` owner key adds a DSSE envelope, explicit trust root,
and threshold policy; the private key is never copied.

The repository workflow pins checkout, setup-go, setup-bun, artifact upload, and
artifact attestation to exact commits. It runs formatting, vet, Staticcheck,
default non-stress tests, vulnerability analysis, modern and legacy MCP
conformance, non-stress fuzz smoke, hard-network reproduction, browser evidence
gates, complete candidate verification, and the cross-platform build matrix.
Race and stress suites are retained as explicit repository-variable opt-ins. A
response bundle is technically releasable only from exact schema-3 observations
through `scripts/build/pack-response-bundle.sh`; the legacy census cannot supply
it. Local green gates and workflow configuration are not
evidence that a tag, owner signature, keyless attestation, response bundle, or
downloadable release exists. Publication still requires explicit authorization
and remote proof. Live empirical capture requires separate study, route,
budget, and redistribution authorization. Release
documentation also includes exact bundled-dependency license texts from
`THIRD_PARTY_NOTICES.md`.

## Identical-Response Study (2026-08-24)

The admitted v5 capture implements the locked identical-response estimand over
the same immutable completion. The locked intersection contains 60 tasks and 12
swing tasks. The accepted run used a clean `6d39a0d` binary, a fresh exact route
attestation, Top-20 evidence, one worker, and a mandatory 10-second dispatch
interval. It completed 60 registered groups in 1208.27 seconds with 66 logical
calls, 79 HTTP attempts, 60 valid logprob records, zero unextracted scores, peak
concurrency one, and zero estimated cost. The capture-run attestation is
`complete`; research admission is `admitted` with 60/60 complete research-
lineage records. The serving route and attestation are retained as machine
provenance, not as a provider or model-quality claim.

Configured OpenAI-compatible presets are product routes, not study evidence.
Only the admitted v5 capture enters the empirical result.

The fixed v3 collection design uses `joint_absolute`: all five candidates in
each of the 60 locked task groups are scored in one completion with five
independent score tags. The graph is arm-independent and requires exactly 60
logical calls, below the authorized 85-call ceiling, while covering all 300
candidates instead of only the 12 outcome-swing groups. A locked-split dry run
reports 60 scored groups, 300 candidate scores, 60/60/60 best/expected/worst
calls, and 5,307,611 estimated input tokens at a 128k per-trajectory ceiling.

The v5 sealed bundle policy
has digest `ad75517a296f7db6657afe002558fd68bab4b6f0a6c0fa6173c51d7537e9ab92`.
Its deterministic response capsule is 7,342,405 bytes, 9 files, capsule ID
`3b26fefb5174cc63d03f47f2be5543878c287e978155116b0da6dce85d9ebf19`, and its
two independent archives are byte-identical at SHA-256
`9f38c61e94f810525c2a2b13ed1e30794f08d559507af91277df577c024e7e80`. The
public scan has zero unsuppressed findings; ten exact rule/fileSHA/line matches
from benchmark task content are recorded in the v5 reviewed-findings artifact.

Registered v5 offline analysis is provider-free and byte-reproducible. It
reports 60/60 complete task groups, 53 agreements, 7 disagreements, zero
unresolved, disagreement rate 0.1167, and exact worst-case missingness upper
confidence bound 0.2080. Outcome sensitivity covers only 2/60 groups and does
not support a quality claim. Historical mechanism conformance:
`scripts/evals/reproduce-public-evidence.sh --profile response-bundle` →
network=denied, provider_calls=0, status=passed, clean_clone_proof=false.

Current live artifacts: `eval/governance/identical-response-{capture-bai-flash-v5.jsonl,
route-attestation-v5.json, study-manifest-v5.json, study-record-v5.json,
live-result-v5.json, capture-run-attestation-v5.json,
research-lineage-admission-v5.json, offline-analysis-v5.json,
run-budget-v5.json, bundle-policy-v5.json, reviewed-findings-v5.json,
capsule-v5.tar.gz}`. The committed v5 outer capsule is
`eval/governance/identical-response-capsule-v5-outer/` with its capsule manifest,
registry, and checksums; the sealed ledger is
`eval/governance/identical-response-claim-ledger-v5.json`, the challenge pack is
`eval/governance/identical-response-claim-challenge-pack-v5.json`, and the
clean-clone report is
`eval/governance/identical-response-reproduction-report-v5.json`. These outer
artifacts verify offline.

The canonical Explorer now derives its one-screen claim chain, 60/53/7/34
comparison, seven-row failure gallery, all challenge receipts, evidence ceiling,
and clean-clone command from the verified v5 family. The release builder admits
the exact registered v5 assets, an omission-explicit inventory, the combined
Explorer, and deterministic presentation capture. Remaining external actions
are owner-key signing, remote downloaded-byte verification/publication, and
independent E5 qualification. The v5 outer capsule
resolves its named parents with
`evidence_ceiling=record_complete_external_parents_resolved`. The current v5 capture-run
attestation and lineage admission are 60/60 reconciled with
status/admission complete/admitted; the sealed v5 claim ledger and 34-receipt
challenge pack are 070-capsule-bound and verify offline.

License: MIT, see `LICENSE`.
