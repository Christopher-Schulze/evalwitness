# Contributing to EvalWitness

EvalWitness accepts changes that make verifier claims more inspectable,
falsifiable, reproducible, or provider-portable without overstating the evidence.

## Before submitting a change

1. Keep provider calls disabled unless the work belongs to an authorized,
   budgeted study.
2. Add tests that fail when the changed behavior is broken. Never weaken a gate
   to accept an implementation.
3. Preserve canonical JSON, exact-byte evidence, no-overwrite publication, and
   typed `not_run` or `unsupported` states.
4. Update `docs/documentation.md`, `docs/spec.md`, `docs/findings.md`, or the
   generated claim surface only when their owned contract changes.
5. Run `scripts/tests/run-tests.sh` from a clean checkout. Release work must also
   pass `scripts/build/build-release-candidate.sh --destination NEW_DIRECTORY`
   with the destination outside the repository.

## Pull requests

Keep each pull request scoped to one auditable change. Describe the claim or
contract affected, the negative control, exact commands run, and any test or
platform not exercised. New dependencies require a license, maintenance,
security, and runtime justification. Empirical artifacts require complete
request, response, route, model, dataset, source, binary, clock, authorization,
and redistribution lineage.

By contributing, you agree that your contribution is licensed under the MIT
License in this repository. Do not submit secrets, proprietary trajectories,
participant identities, or provider responses without explicit redistribution
rights.
