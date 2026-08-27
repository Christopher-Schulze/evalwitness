# Security Policy

## Supported version

No EvalWitness version is currently published or supported. After the first
release, only the latest published version will be eligible for security fixes.
The current `0.2.0` source is a local release candidate.

## Private disclosure

Report suspected vulnerabilities privately to
`christopher.schulze.github@proton.me`. Include the affected commit or release,
reproduction steps, impact, and whether public exploitation is known. Do not
attach provider credentials, private trajectories, raw participant material, or
other people's personal data. Encrypt sensitive evidence before sending it and
request a current public key first.

The maintainer will acknowledge a complete report within seven calendar days,
triage severity and affected versions, coordinate a fix and disclosure date,
and credit the reporter unless anonymity is requested. Public disclosure before
a fix is available may put users at risk.

## Security boundary

EvalWitness processes untrusted trajectory and archive inputs through bounded,
fail-closed readers, but no blanket claim of safety for hostile production use
exists. Provider execution is opt-in and configured routes receive the submitted
content. `EVALWITNESS_OFFLINE=true` disables provider dispatch but is not an
operating-system network sandbox; combine it with OS-level network denial when
network access must be forbidden. The application has no telemetry. Public
artifact scanning applies generic credential and private-path detection to text
files; opaque files are checked only for exact registered secret-bearing
environment values. Release admission supplements that boundary with
tracked-source scanning, clean embedded build settings, exact source/module-graph
binding, and archive-only reproducible-build checks.

The root `.gitignore` is a defense-in-depth boundary for local material: it
excludes environment files, owner-only custody, build and cache output, fetched
trajectory data, agent/editor state, session or chat exports, token/OAuth stores,
local databases, authentication files, logs, and certificate bundles. It does
not replace the fail-closed artifact scan, which evaluates the exact public
candidate before release.
