# Held-out campaign plan

> Provider-free topology evidence. This plan fixes workload shape and execution class, not provider requests, replay payloads, budget, authorization, or execution.

- Status: `topology_locked_live_bindings_absent`
- Partition: `57` cases, `1140` cells, `440` supported, `700` structural unsupported
- Provider-dependent workload: `3` arms, `342` supported cells, `684` verification inputs, `2052` registered side repetitions
- Live-provider workload: `2` arms, `228` supported cells, `456` verification inputs, `1368` registered side repetitions
- Sealed-replay workload: `1` arm, `114` supported cells, `228` verification inputs, `684` registered side repetitions
- Zero-cost workload: `7` arms, `98` supported cells, `294` deterministic repetitions
- Fixed repetitions: `3`
- Run authorized: `false`
- Execution permit issued: `false`
- Provider calls: `0`
- Empirical units: `0`
- Campaign digest: `7458788aa2635a2eb063eb568f8c37fc7548b2babef1f6ab3893726effacde97`

## Arm topology

| Arm | Execution class | Provider-dependent | Test | Supported | Unsupported | Verification inputs | Registered repetitions |
|---|---|---:|---:|---:|---:|---:|---:|
| `explicit-text-judge` | `live_provider` | `true` | 114 | 114 | 0 | 228 | 684 |
| `external-protocol-adapter` | `sealed_provider_replay` | `true` | 114 | 114 | 0 | 228 | 684 |
| `score-token-verifier` | `live_provider` | `true` | 114 | 114 | 0 | 228 | 684 |
| `zero-cost-fewest-error-words` | `deterministic_local` | `false` | 114 | 14 | 100 | 0 | 42 |
| `zero-cost-fewest-steps` | `deterministic_local` | `false` | 114 | 14 | 100 | 0 | 42 |
| `zero-cost-fewest-trace-bytes` | `deterministic_local` | `false` | 114 | 14 | 100 | 0 | 42 |
| `zero-cost-first-listed` | `deterministic_local` | `false` | 114 | 14 | 100 | 0 | 42 |
| `zero-cost-most-error-words` | `deterministic_local` | `false` | 114 | 14 | 100 | 0 | 42 |
| `zero-cost-most-steps` | `deterministic_local` | `false` | 114 | 14 | 100 | 0 | 42 |
| `zero-cost-most-trace-bytes` | `deterministic_local` | `false` | 114 | 14 | 100 | 0 | 42 |

## Live-binding boundary

Every live-binding flag is `false`: StudyRecord, execution bindings, two live-provider request plans, the sealed-replay plan, provider call counts, budgets, current route attestations, authorization digests, and the private capsule family.

## Claim boundary

Supported: the exact held-out arm topology, execution classes, and registered repetition workload are locked provider-free while every live execution binding remains absent.

Unsupported:

- provider request count
- provider budget
- live route currency
- execution authorization
- held-out execution
- empirical verifier reliability
