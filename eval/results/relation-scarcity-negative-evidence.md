# EvalWitness Negative-Evidence Brief: Omitted Test Evidence

> This provider-free artifact reports what the frozen corpus could not support. It contains no restricted trajectory content, owner decision, human judgment, verifier result, provider result, held-out result, or authorization.

Evidence contract: `evalwitness.relation-scarcity-public-evidence.v1`

Evidence digest: `f401b84549b013f20deb9c78366f28b609fa7ff83cad330ac7e01617cd466d4f`

Brief policy: `evalwitness.relation-scarcity-public-brief.v1`

## Result at a glance

The frozen construct firewall evaluated **198** `omitted_test_evidence` attempts. It admitted **3**, rejected **195**, and therefore left a **37-case shortfall** against the descriptive 40-case availability target. The three admitted cases are exhaustive within this release, but their roles are two development, one calibration, and zero test.

```text
198 attempted -> 3 admitted -> 3 selected -> 2 development + 1 calibration + 0 test
```

This is a corpus-specific construct-availability result. The shortfall was preserved instead of relaxing the eligibility predicate, fabricating a test split, or treating the sentinel as an eighth balanced family.

## Eligibility funnel

| Source format | Attempted | Admitted | Rejected | Eligible task groups | Selected |
|---|---:|---:|---:|---:|---:|
| `swe_bench_cache_item_json` | 80 | 0 | 80 | 0 | 0 |
| `terminal_bench_trajectory_json` | 118 | 3 | 115 | 3 | 3 |
| **Total** | **198** | **3** | **195** |  | **3** |

| Closed rejection reason | Count |
|---|---:|
| `unverified_evidence_role` | 195 |

## Study-role boundary

| Role or use | Cases | Status |
|---|---:|---|
| development | 2 | descriptive only |
| calibration | 1 | descriptive only |
| test | 0 | unavailable; no held-out claim |
| primary-estimand overlap | 0 | excluded |
| balanced inferential core | 280 | 7 separate families, 40 cases each |
| locked relation primary sample | 28 | 28 task groups and 28 lineage clusters |

## Public case commitments

The commitments below identify the three governed cases without publishing task text, source paths, trajectory excerpts, or owner notes.

| Case | Data role | Unit | Case binding | Construct firewall |
|---:|---|---|---|---|
| 1 | `calibration` | `trajectory_pair` | `ce550ee99de8285cfa36e5472e51246666660b6f6f75b050e8653e0c3d7b582d` | `6127696ebd396bc8e6f32cb60d2c6a6f33811de4eb9b4c3ae2b131af9ffd2ce4` |
| 2 | `development` | `trajectory_pair` | `2555df910507817ae422127ae58220ffca6e0457e00efe1356acc15a6185c2d9` | `d857562a2510df38453358640c5680098f3b50a1cce4bee57a03e87b89db4c97` |
| 3 | `development` | `trajectory_pair` | `32c1972dabeb8aaa715d25ff043650801928ff0c9a3934bd8431c84f45157c9b` | `ff7df17c7d8949b4ad524ce9ef759874d5539c3d38c3cacd567dae1a07904947` |

## Evidence chain

| Artifact | Digest |
|---|---|
| corpus development plan | `5a7f4f55aafabd03ef9c802a9c48cf198a778228b40ade35e8f074df8d060c85` |
| 939-attempt natural audit | `af0c0fd56fb498586096a8776e0d40794ee93acf5afda67cc000e576bfcef4d2` |
| typed 283-case release | `9b4999dafe2d37ea04c298b80a7aba0a1769755fdfd650cd01bf3a9cc31a2e42` |
| relation governance plan | `6eac462cae0a5b626561d5cbea274a5c3a72c78b6cb9d50a8952be6ccbb6fa8c` |
| balanced primary sample | `6b721bcf0fb10e47923b46d92f3c14691cbd0dd98949ab0bf1dc016d8e1c1e43` |
| exhaustive scarcity sentinel | `ec720a56394249a47eb4c0f7ef618471ce14c69ff8c2c13e1e851350c23e71fb` |

## Claim boundary

| Claim | Evidence status |
|---|---|
| Exact availability in the frozen corpus | supported by the reproduced attempt, firewall, release, and sentinel chain |
| Held-out omitted-evidence validity | unsupported: zero test-role sentinel cases |
| Human construct agreement | not run |
| Verifier robustness for this construct | not measured |
| Provider behavior | not measured; no provider was invoked |
| Population prevalence or universal scarcity | unsupported: the corpus is not a probability sample |
| Reviewer contact, packet sharing, or publication authority | not authorized |

## Reproduce offline

```bash
evalwitness relation render-scarcity-public-brief \
  --format markdown \
  --plan @eval/governance/relation-audit-plan-v3.json \
  --primary-sample @eval/governance/relation-primary-sample-v3.json \
  --scarcity-sentinel @eval/governance/relation-scarcity-sentinel-v3.json \
  --corpus-plan @eval/governance/controlled-corruption-v3-plan.json \
  --corpus-audit @eval/governance/controlled-corruption-v3-natural-audit.json \
  --release @eval/governance/controlled-corruption-v3-release.json
```
