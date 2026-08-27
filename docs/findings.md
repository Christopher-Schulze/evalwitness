# Findings

EvalWitness reports only claims that resolve to typed evidence. The generated
claim view at the end is the canonical provider-free reference boundary; the
admitted identical-response v5 package has its own sealed E2 claim ledger.
`scripts/tests/run-claimcheck.sh` recomputes it from the provider-free reference
capsule and rejects stale values, widened scope, or missing caveats.

## Evidence roles

| Evidence role | What it can establish | What it cannot establish |
|---|---|---|
| Current provider-free capsule | deterministic artifact identity, method self-falsification, claim transport, negative controls, exact replay, and explicit unavailable states | verifier correctness, provider behavior, human validity, prevalence, transfer, drift, or independent reproduction |
| Admitted exact-response bundle | bounded extraction-semantics observations from identical response bytes, strict Top-20 evidence, complete research lineage, and reproducible offline analysis | correctness, provider or model ranking, human validity, transfer, prevalence, or population generalization |
| Legacy development artifacts | exact values and paired analyses recorded in the committed artifacts | current route capability, exact historical response reconstruction, confirmatory paper reproduction, or generalization |
| Owner inspection attestation | public schema, digest, denominators, aggregate local owner-attested statuses, and disclosure limits | blinded human support, independent review, provider performance, public source reproduction, or authorization |

The legacy artifacts declare Terminal-Bench 2 and SWE-bench Verified development
runs collected on 2026-08-06 and 2026-08-07. Their requested route/model metadata
is inspectable, but the public capsule does not bind complete historical response
bytes and current route attestations. Every legacy result therefore remains E1.

The admitted identical-response v5 bundle is a separate evidence role. It binds
complete schema-3 response records, an exact route attestation, complete research
lineage, and a deterministic analysis; it does not promote the legacy artifacts
or widen their claim ceiling.

## Current finding A: the method preserves its own falsification history

| Field | Record |
|---|---|
| Research question | Does each mutation-method generation retain the counterevidence that invalidated its predecessor, and does the current generation refuse an unavailable held-out construct? |
| Protocol | Deterministically rebuild `evalwitness.claim-autopsy.v1` from the verified capsule and ledger; verify the immutable v1 repair evidence, v2 firewall challenge, v3 natural audit, typed release, and scarcity contract |
| Dataset role | v1/v2 frozen negative controls; v3 provider-free development corpus; scarcity cases are a separate exhaustive sentinel, never part of the seven-family primary estimand |
| Task unit | typed mutation attempt for eligibility; selected controlled case for release; source task and lineage cluster remain bound parents |
| Sample | v1: three frozen false acceptances; v2: fourteen challenges containing five false acceptances, six positive controls, and three shared guards; v3: 939 attempts, 283 selected cases, 280 inferential-core cases, and three scarcity-sentinel cases |
| Missingness | 195 of 198 scarcity attempts fail the frozen eligibility firewall; missing cases are not imputed and the quota is not relaxed |
| Test | exact deterministic reproduction, schema validation, digest validation, parent verification, and frozen negative-control transition tests; no provider or human test |
| Interval | not applicable to the exact artifact census; no prevalence interval is permitted because the corpus is not a probability sample |
| Multiplicity | not applicable; this is a versioned mechanism and availability audit, not a family of significance tests |
| Result | v1 is `falsified`; v2 is `superseded`; v3 is `admitted_development`; scarcity availability is 3 of 40 with zero test-role cases, so `held_out_omitted_evidence_validity` is unsupported |
| Capsule and claim identity | Claim Autopsy method lane; `frozen_corpus_availability` supported and `held_out_omitted_evidence_validity` unsupported in [the canonical scarcity contract](../eval/results/relation-scarcity-negative-evidence.json) |
| Scope | E1 provider-free mechanism conformance for the exact frozen artifacts |
| Limitations | no construct prevalence, human agreement, verifier robustness, provider behavior, v4 feasibility, or population claim |

## Current finding B: verification evidence can be localized to its first loss

| Field | Record |
|---|---|
| Research question | Does one declared failable verification claim survive the native trace path into verifier input, and where does a non-failable same-path comparison stop? |
| Protocol | Verify the verification-lineage development release; derive the five ordered layers `runtime_witness` -> `native_export` -> `canonical_graph` -> `retained_bundle` -> `verifier_request`; compare the accepted BOM with the earliest-loss certificate |
| Dataset role | two synthetic adapter-development fixtures; no empirical task group and no research-admitted source |
| Task unit | one claim candidate with bound executable, operands, observable failure condition, record IDs, object digests, and request lineage |
| Sample | one accepted positive path and one rejected same-path counterexample |
| Missingness | none inside the sealed fixtures; empirical trace availability is entirely unmeasured |
| Test | deterministic offline proof, release-file digest verification, graph/BOM/certificate cross-binding, and fail-closed layer validation |
| Interval | not applicable to two conformance fixtures; no empirical coverage estimate is reported |
| Multiplicity | not applicable |
| Result | `claim-fixed-fixture-exits-zero` reaches an accepted BOM across all five layers; the non-failable comparison stops at `retained_bundle` before a verifier request |
| Capsule and claim identity | claim `claim-fixed-fixture-exits-zero`; BOM `offline-bom-stdout-success`; loss certificate `task_069-same-path-loss-v1`; release `task_069-development-release-v1` |
| Scope | E1 development-fixture transport and release-integrity evidence with zero provider calls |
| Limitations | no empirical retention rate, provider result, verifier correctness, transfer, human validity, or causal agent attribution |

## Current finding C: local owner inspection did not clear the scarcity boundary

| Field | Record |
|---|---|
| Research question | Is the complete private owner-inspection chain internally complete, and what aggregate construct statuses may be exposed publicly without disclosing restricted evidence? |
| Protocol | Verify the private chain during projection, publish only the closed public attestation, and validate schema, digest, denominators, aggregate status, disclosure, and claim boundary |
| Dataset role | local pre-review custody over seven core pilot constructs and three separately reported scarcity constructs |
| Task unit | one required assessment event; aggregate construct is the public reporting unit |
| Sample | all 66 required assessments completed; seven core constructs and three scarcity constructs |
| Missingness | no required assessment event is missing; private identities and restricted evidence are intentionally withheld, so public source reproduction is unavailable |
| Test | deterministic private-chain verification and public-attestation validation; no formal human study or provider evaluation |
| Interval | not applicable to owner-attested completion and status counts |
| Multiplicity | not applicable |
| Result | core status is `passed`; scarcity and overall status are `revision_required`; two of three scarcity constructs require revision |
| Capsule and claim identity | [`evalwitness.relation-owner-inspection-public-attestation.v1`](../eval/results/relation-owner-inspection-attestation.json); claims `public_document_integrity`, `private_owner_inspection`, `core_construct_status`, and `scarcity_construct_status`; the artifact declares `capsule_status="not_yet_capsule_bound"` |
| Scope | local owner-authorized, agent-assisted custody and aggregate public projection |
| Limitations | not blinded human evidence, independent review, relation admission, provider performance, public source reproduction, study launch, or publication authority |

## Current finding D: identical-response extraction semantics are reproducibly bounded

| Field | Record |
|---|---|
| Research question | Under one immutable completion, how often does the distribution-aware decision differ from the chosen-token decision? |
| Protocol | Exact replay of the admitted schema-3 response bundle; both arms consume the same response bytes and differ only in score extraction semantics |
| Dataset role | bounded development study with a locked task-group manifest; not a probability sample of providers, models, or tasks |
| Task unit | source-task group |
| Sample | 60 complete groups, 300 candidate scores, and 12 outcome-swing groups |
| Missingness | 0 unresolved groups; independent outcome evidence covers 2 of 60 groups only |
| Test | exact agreement/disagreement census and registered outcome-sensitivity check; no quality-superiority test is claimed |
| Interval | disagreement rate 7/60 = 0.1167; exact worst-case missingness upper confidence bound 0.2080 |
| Multiplicity | one registered counterfactual comparison |
| Result | 53 agreements, 7 disagreements, and 0 unresolved groups |
| Capsule and claim identity | Identical-response v5 outer capsule and claims CLM-070 through CLM-074; [`identical-response-offline-analysis-v5.json`](../eval/governance/identical-response-offline-analysis-v5.json), [`identical-response-capture-bai-flash-v5.jsonl`](../eval/governance/identical-response-capture-bai-flash-v5.jsonl), [`identical-response-route-attestation-v5.json`](../eval/governance/identical-response-route-attestation-v5.json), and [`identical-response-research-lineage-admission-v5.json`](../eval/governance/identical-response-research-lineage-admission-v5.json) |
| Scope | admitted E2 bounded empirical evidence; the serving route is retained as provenance, not as a provider-quality claim |
| Limitations | no correctness, calibration, human validity, provider ranking, cross-route transfer, checkpoint attribution, prevalence, or population-generalization claim |

## Legacy finding 1: recorded Terminal-Bench ratio

| Field | Record |
|---|---|
| Research question | What verifier-to-random score ratio is recorded in the committed Terminal-Bench development artifact? |
| Protocol | Evaluate the exact ratio expression over the sealed legacy-derived result component |
| Dataset role | development only |
| Task unit | benchmark aggregate; task-level selection is not used for this descriptive ratio |
| Sample | 89 recorded tasks, 17 decidable for selection comparisons |
| Missingness | complete derived task rows are present; raw historical response and route-attestation bytes are unavailable |
| Test | none; descriptive exact ratio |
| Interval | not applicable; no confirmatory random-selection interval was admitted |
| Multiplicity | not applicable |
| Result | CLM-011 records exactly 15/14 and remains `exploratory` |
| Capsule and claim identity | Reference-capsule legacy component `legacy.result.terminal-bench-verifier`; CLM-011 |
| Scope | one Terminal-Bench development artifact |
| Limitations | no pooled, method-general, current-route, or paper-reproduction claim |

## Legacy finding 2: bounded call ratio without outcome equivalence

| Field | Record |
|---|---|
| Research question | What call-count ratio is recorded for the bounded and paper-parity paths, and do the paired task outcomes establish equivalence? |
| Protocol | Exact call-count ratio across the two committed artifacts; exact two-sided McNemar comparison with Newcombe paired score interval per benchmark |
| Dataset role | development only |
| Task unit | task; only tasks with outcome variation are decision-informative |
| Sample | Terminal-Bench: 17 decidable tasks and two discordances; SWE-bench: 86 decidable tasks and 27 discordances |
| Missingness | invariant-outcome tasks are excluded from paired inference by design; historical response lineage is incomplete |
| Test | Terminal-Bench 1-1 discordant, p=1.0000; SWE-bench 13-14 discordant, p=1.0000 |
| Interval | bounded minus parity: Terminal-Bench 0.0000, 95% CI [-0.2038, 0.2038]; SWE-bench -0.0116, 95% CI [-0.1274, 0.1047] |
| Multiplicity | each displayed query uses Bonferroni family size 1; no equivalence margin was prespecified |
| Result | CLM-022 supports the exact 101/1712 call-count ratio; neither directional advantage nor outcome equivalence is established |
| Capsule and claim identity | Reference-capsule legacy call facts; CLM-022 |
| Scope | exact committed development artifacts and their declared analysis |
| Limitations | lower call count is not equal quality, universal efficiency, or paper-result reproduction |

## Legacy finding 3: distribution-over-letter superiority is unsupported

| Field | Record |
|---|---|
| Research question | Does the recorded verifier arm outperform the text-only judge arm on decidable tasks? |
| Protocol | Exact paired McNemar test and Newcombe paired score interval within each benchmark |
| Dataset role | development only |
| Task unit | task |
| Sample | Terminal-Bench: 17 decidable, two verifier-only and zero judge-only; SWE-bench: 86 decidable, five verifier-only and seven judge-only |
| Missingness | exact response identity is unavailable, so the difference cannot be attributed to extraction alone |
| Test | Terminal-Bench p=0.5000; SWE-bench p=0.7744, both exact and two-sided |
| Interval | verifier minus judge: Terminal-Bench 0.1176, 95% CI [-0.0525, 0.3101]; SWE-bench -0.0233, 95% CI [-0.1003, 0.0545] |
| Multiplicity | Bonferroni family size 1 per displayed query; no cross-benchmark pooled superiority claim |
| Result | CLM-013 is unsupported; CLM-014 separately forbids an exact response-identical counterfactual interpretation |
| Capsule and claim identity | Reference-capsule legacy result components; CLM-013 and CLM-014 |
| Scope | two artifact-level arm comparisons |
| Limitations | no method superiority, equivalence, or extraction-only causal attribution |

## Legacy finding 4: zero recorded verifier ties

| Field | Record |
|---|---|
| Research question | How many exact pair-decision ties occur in the two primary verifier artifacts? |
| Protocol | Exact census of pair decisions in the sealed legacy facts |
| Dataset role | development only |
| Task unit | pair decision nested within task |
| Sample | 240 recorded verifier pair decisions |
| Missingness | none in the committed decision rows; population sampling is undefined |
| Test | none; exact artifact count |
| Interval | not applicable to the committed census; no population tie-rate interval |
| Multiplicity | not applicable |
| Result | CLM-012 supports zero ties; CLM-017 independently governs the 240-decision denominator |
| Capsule and claim identity | Reference-capsule `legacy.claim-facts`; CLM-012 and CLM-017 |
| Scope | exact committed decision rows |
| Limitations | no general tie rate and no inferred accuracy advantage |

## Legacy finding 5: raw win probability is not calibrated confidence

| Field | Record |
|---|---|
| Research question | Does raw pair `win_probability` behave as calibrated correctness confidence in the committed development samples? |
| Protocol | Task-clustered calibration analysis with reliability bins, AUC, ECE, accuracy, and cluster intervals |
| Dataset role | development only; no calibration/test split |
| Task unit | pair decision clustered by source task |
| Sample | verifier/judge pairs: Terminal-Bench 36/34 over 17 task clusters; SWE-bench 108/105 over 86 task clusters |
| Missingness | only pairs with differing trajectory reward define directional correctness; no held-out route or model |
| Test | no single superiority test is promoted; the calibration bins are non-monotone in all four samples |
| Interval | verifier AUC: Terminal-Bench 0.828 with 95% task-cluster interval [0.638, 0.980]; SWE-bench 0.621 with [0.510, 0.731] |
| Multiplicity | intervals are descriptive across four development arms with no multiplicity correction; cross-arm superiority is forbidden |
| Result | CLM-015 rejects calibrated-confidence use; CLM-016 preserves only the exact Terminal-Bench development AUC |
| Capsule and claim identity | Reference-capsule legacy calibration components; CLM-015 and CLM-016 |
| Scope | committed development artifacts |
| Limitations | no held-out calibration, threshold validity, model transfer, or claim that recalibration must fail |

## Legacy finding 6: universal two-call sufficiency is unsupported

| Field | Record |
|---|---|
| Research question | Do the committed cache sweeps justify a universal claim that escalation beyond two calls buys nothing? |
| Protocol | Provider-free replay of nested call ceilings over already recorded calls |
| Dataset role | development tuning analysis |
| Task unit | decidable task |
| Sample | 17 Terminal-Bench and 86 SWE-bench decidable tasks |
| Missingness | no new provider draws, routes, models, or held-out tasks |
| Test | the third-call ceiling creates zero Terminal-Bench disagreements and one disagreement in each direction on SWE-bench; no universal test is admitted |
| Interval | not admitted; the ledger retains the zero third-call observation only as the value guarded by an unsupported universal claim |
| Multiplicity | multiple tuning choices were explored; no confirmatory correction or held-out decision was registered |
| Result | CLM-023 is unsupported; CLM-024 separately keeps evidence-budget harmlessness or optimality unsupported |
| Capsule and claim identity | Reference-capsule legacy derived artifacts; CLM-023 and CLM-024 |
| Scope | bounded offline sweep of committed development calls |
| Limitations | no universal sufficiency, optimality, provider transfer, or free additional sampling |

## Legacy finding 7: hunk baseline direction is artifact-specific

| Field | Record |
|---|---|
| Research question | How does the most-patch-hunks heuristic compare with the verifier on decidable SWE-bench tasks in the committed artifact? |
| Protocol | Deterministic baseline selection over the same trajectory sets and paired task outcomes |
| Dataset role | development only |
| Task unit | decidable task |
| Sample | 86 decidable SWE-bench tasks; 35 verifier-versus-hunk discordances |
| Missingness | tasks with invariant trajectory reward are excluded from selection inference; external tasks and routes are absent |
| Test | paired table records 14 verifier-only and 21 hunk-only outcomes; no general superiority claim is admitted |
| Interval | not present in the claim ledger; the supported expression is the exact seven-task artifact difference only |
| Multiplicity | many heuristic baselines were explored; their search invalidates a standalone confirmatory reading |
| Result | CLM-019 supports a seven-task descriptive difference; CLM-020 rejects general heuristic superiority |
| Capsule and claim identity | Reference-capsule SWE legacy facts; CLM-019 and CLM-020 |
| Scope | one committed SWE-bench development artifact |
| Limitations | no causal effect, universal free baseline, or external validity |

## Legacy finding 8: size-clause ablation is descriptive, not causal

| Field | Record |
|---|---|
| Research question | Did removing two size-related prompt clauses cause the observed SWE-bench outcome difference? |
| Protocol | Paired comparison of shipped and size-agnostic artifacts over identical task sets |
| Dataset role | post hoc development ablation |
| Task unit | decidable task |
| Sample | 86 decidable tasks; 26 discordances, split 10 shipped-only and 16 size-agnostic-only |
| Missingness | no held-out replication, randomized prompt assignment, second model, or exact historical response counterfactual |
| Test | exact two-sided McNemar p=0.3269 |
| Interval | shipped minus size-agnostic effect -0.0698, 95% CI [-0.1814, 0.0449] |
| Multiplicity | Bonferroni family size 1 for the displayed pair, but the ablation was selected after observing development behavior |
| Result | CLM-021 is unsupported; six observed task crossings do not establish causality |
| Capsule and claim identity | Reference-capsule shipped and size-agnostic SWE components; CLM-021 |
| Scope | one post hoc development ablation |
| Limitations | no causal, general, default-change, or benchmark-independent conclusion |

## Legacy finding 9: a short probe cannot qualify a route

This finding governs the historical route survey. The separately admitted
identical-response v5 bundle has its own exact route attestation and does not
change the legacy claim ledger.

| Field | Record |
|---|---|
| Research question | What evidence is sufficient to call a live verifier route capable? |
| Protocol | Fail-closed route state machine requiring an exact, current, expiring bounded score-contract attestation |
| Dataset role | operational conformance; historical survey notes are not an admitted empirical dataset |
| Task unit | exact provider, gateway, endpoint, requested model, served identity, request contract, and attestation window |
| Sample | no current public E2 route attestation for the historical survey; its three-route response bytes are unavailable |
| Missingness | complete historical responses and attestation bytes are absent, so the historical count cannot be reconstructed |
| Test | diagnostics may reach `probe_compatible`; only the production-shaped score task can reach full-verifier eligibility |
| Interval | not applicable |
| Multiplicity | not applicable |
| Result | CLM-026 supersedes probe qualification; CLM-027 and CLM-028 keep the historical survey and current capability inference unsupported |
| Capsule and claim identity | Reference-capsule route provenance component; CLM-026, CLM-027, and CLM-028 |
| Scope | route admission policy |
| Limitations | no current live provider capability, universal endpoint support, or prevalence claim |

## What would change the conclusions

A locked second-model and multi-route study could address CLM-032. A prespecified
held-out calibration split could test selective thresholds. A larger set of
decision-informative tasks could narrow paired intervals. Real blinded reviewers
could address outcome and relation construct validity. Those are broader future
questions, not prerequisites for the admitted identical-response bounded
package. The existing v5 capsule is author-operated and reproducible, but it is
not an independent external replication.

<!-- evalwitness:claim-surface:findings:begin -->
### Machine-verified claim view

This block is deterministically derived from the canonical claim ledger and verified capsule. Edit the ledger or evidence, never these rows.

| Claim | Status | Level | Exact value | Governed statement | Required caveat |
|---|---|---|---|---|---|
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
| CLM-027 | unsupported | E1 | string:unavailable_public_source | Three surveyed routes passed short probes and failed production-shaped score extraction | The public capsule lacks the historical response and attestation bytes needed to assert this survey result |
| CLM-031 | unsupported | E1 | string:not_run | The repository reproduces the paper's headline empirical results | Reference conformance and dataset tallies are not headline empirical reproduction |
| CLM-032 | unsupported | E1 | number:0 | Findings transfer across providers, gateways, model families, checkpoints, or time | Transfer requires the prespecified multi-provider and longitudinal studies |

View digest: `46d86de2f69a2c00cbd3749540815fdad9db44e5a1c0ab463628223de01a4c2d`

<!-- evalwitness:claim-surface:findings:end -->

## Offline baseline measurements (development evidence)

These measurements were computed entirely offline from committed per-task
details with zero provider calls. They establish reference curves for
selection quality scaling and evidence-slice retention.

### Selection quality across N

| Strategy | N=2 | N=3 | N=4 | N=5 |
|---|---|---|---|---|
| Oracle | 86.5% [77.9, 92.1] | 87.6% [79.2, 93.0] | 89.9% [81.9, 94.6] | 89.9% [81.9, 94.6] |
| Random expected | 81.5% [72.1, 88.2] | 81.6% [72.4, 88.3] | 82.3% [73.1, 88.8] | 81.8% [72.5, 88.4] |
| First-only | 82.0% [72.8, 88.6] | — | — | — |
| Committed verifier | 87.6% [79.2, 93.0] | — | — | — |

Monotonicity oracle ≥ verifier ≥ random verified at every N.
Wilson 95% score intervals shown in brackets.
Computed from eval/results/terminal-bench-verifier.json details (89 tasks × 5 trials).
N=8 requires fresh candidate generation via live calls (gated).

### Evidence retention on real Terminal-Bench traces

Per-kind survival at 16k/32k/64k budgets over 10 real trajectory files:

| Kind | Original | 16k retained | 32k retained | 64k retained |
|---|---|---|---|---|
| message | 723 | 241 (33.3%) | 143 (19.8%) | 241 (33.3%) |
| tool_call | 366 | 113 (30.9%) | 121 (33.1%) | 122 (33.3%) |
| command | 366 | 122 (33.3%) | 114 (31.1%) | 122 (33.3%) |
| tool_result | 366 | 113 (30.9%) | 121 (33.1%) | 122 (33.3%) |
| metadata | 30 | 10 (33.3%) | 5 (16.7%) | 10 (33.3%) |

Measured by internal/audit/retention_live_test.go using
audit.MeasureRetention on terminal_bench_trajectory_json format files
from eval/trajectories/terminal_trajs/forge_gpt54/.
