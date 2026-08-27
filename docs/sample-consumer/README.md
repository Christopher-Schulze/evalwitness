# EvalWitness offline audit — isolated sample consumer

Minimal external repository layout demonstrating the TASK 059 policy-as-code
audit. Copy `.github/`, `config/`, and the profile JSON into your repository,
set the `EVALWITNESS_RELEASE_VERSION` and
`EVALWITNESS_RELEASE_CHECKSUM` repository variables to an exact signed release,
and every pull request runs the offline audit with zero provider calls and zero
credentials. Until both variables exist, the job is skipped rather than failing
against invented release coordinates.

Contents (byte-aligned with this repository's shipped assets):

- `.github/actions/evalwitness-audit/action.yml` — pinned-binary, checksum- and
  signature-gated offline audit composite action.
- `.github/workflows/offline-audit.yml` — pull-request-safe trigger only;
  live evaluation is never wired into automated CI.
- `config/minimal-public.json` — minimal public policy requiring three measured
  evidence-reliance dimensions from the reference profile.
- `reference-profile-for-policies.json` — deterministic TASK 058 profile
  (`evalwitness profile build --identity reference --route r1
  --dim 'a:measured:m:s:E1:c:1:task'`).

Local reproduction without GitHub:

```bash
evalwitness audit \
  --policy config/minimal-public.json \
  --profile reference-profile-for-policies.json \
  --format json
```

Exit codes: 0 pass, 1 policy failed, 2 invalid input, 3 internal error.
