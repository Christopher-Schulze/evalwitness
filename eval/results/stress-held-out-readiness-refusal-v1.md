# Held-out run readiness refusal

> Provider-free preflight evidence. This receipt does not authorize execution and is not a held-out result.

- Status: `not_ready`
- Run authorized: `false`
- Execution permit issued: `false`
- Provider calls: `0`
- Empirical units: `0`
- Partition: `57` cases, `1140` cells, `440` supported, `700` structural unsupported
- Receipt digest: `46c61709efa76ab4577a89b598a96da046a5ed57d22747446bf253d5d6ab1588`

## Gate ledger

| Gate | Status | Evidence | Reason |
|---|---|---|---|
| `held_out_partition_lock` | `passed` | `330108c06ad51d47b73e93275ab54d11262b679538af0da861a50394ffa3c897` | `exact_locked_test_partition_validated` |
| `owner_inspection` | `blocked` | `fd2c364fee2d575120ae4fc29e07788fe5f4107c63f2828da5add916dc7e2a84` | `owner_inspection_overall_status_revision_required` |
| `blinded_human_admission` | `missing` | unavailable | `primary_audit_terminal_ledger_unavailable` |
| `authorized_study_record` | `missing` | unavailable | `controlled_relation_study_record_unavailable` |
| `execution_and_budget_binding` | `missing` | unavailable | `exact_multi_arm_execution_binding_unavailable` |
| `current_route_attestations` | `missing` | unavailable | `current_attestation_per_provider_arm_unavailable` |
| `live_authorization` | `missing` | unavailable | `exact_live_authorization_digest_unavailable` |
| `private_capsule_family` | `missing` | unavailable | `verified_private_owner_capsule_family_unavailable` |

## Claim boundary

Supported: the exact held-out partition and current owner projection were inspected provider-free, and the real run is not authorized.

Unsupported:

- held-out execution
- verifier reliability
- provider quality
- population generalization
- execution authorization
