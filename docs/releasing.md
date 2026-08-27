# Releasing

This is the pre-publication runbook, not evidence that a release exists. Remote
tagging, pushing, asset upload, and publication require explicit owner
authorization plus the clean-clone and capsule proofs below.

## Public claim set

EvalWitness is a provider-portable, reproducible verifier audit lab for
coding-agent trajectories. A release publishes one claim set from the canonical
ledger and its verified capsule. README, findings, runtime documentation, this
runbook, and the agent skill consume the generated claim projection; none may
maintain a separate result timeline.

The release surfaces have two complementary flagships: the provider-free
`evalwitness.claim-autopsy.v1` engineering proof and the admitted v5
identical-response bundle. The same capsule parents drive the Claim Autopsy's
method-integrity lane, five-layer claim-transport lane, negative controls,
offline explorer, and required caveats; the v5 graph contributes the
bounded exact-response empirical result. Before publication,
`evalwitness claim surface verify` must confirm every registered public surface.
This runbook remains pre-publication evidence until an authorized release proves
the remote tag, workflow, downloaded assets, checksums, signatures, attestations,
and anti-rollback state.

## What a release contains

| asset | produced by | automated |
|---|---|---|
| complete seven-role candidate with source, exact offline Go module proxy, five binaries, public capsule, protocol kit, controlled evidence, offline explorer, documentation, and replication starter | `scripts/build/build-release-candidate.sh` | yes |
| PAX-free deterministic USTAR+gzip source archive plus canonical commit, format, digest, size, file-count, and directory-count report | `evalwitness release source-archive` | yes |
| canonical clean source-tree provenance with exact commit, status, path, Git mode, size, file digest, totals, and graph digest | `evalwitness release source-index` | yes |
| canonical `release-manifest.json` with exact path, role, size, SHA-256, commit, source-archive digest, product version, and non-claim state | `evalwitness release manifest` | yes |
| conservative SPDX 2.3 SBOM derived from embedded Go build information in all five clean binaries | `evalwitness release sbom` | yes |
| in-toto Statement v1 binding the exact release manifest and SBOM | `evalwitness release statement` | yes |
| explicit-key DSSE envelope, trust root, and one-signature policy | `evalwitness release sign` with an existing owner-approved mode-`0600` Ed25519 key | optional local authorization |
| internal `SHA256SUMS` for the complete candidate graph | candidate builder, stored inside the candidate archive | yes |
| release-root `SHA256SUMS` for every flat GitHub release asset except itself | tag workflow | yes after authorization |
| archive-only `roundtrip-verification.json` with manifest-bound portable provenance, empty-cache local-proxy tests, explicit fetched-trajectory status, byte-identical host rebuild, and exact evidence/view reproduction | `scripts/tests/run-release-roundtrip.sh` | yes |
| `terminal_trajs.tar.gz`, `swebench_verified_trajs.tar.gz` | `scripts/build/pack-eval-data.sh` | **no** |
| `SHA256SUMS-eval-data` for those archives | same | **no** |
| public reference capsule, deterministic archive, claim ledger, capsule in-toto Statement, claim projection, and Claim Autopsy | candidate builder through `evalwitness capsule build` | yes |
| empirically admitted v5 response bundle, outer capsule, five-claim ledger, 34-receipt challenge pack, omission-explicit inventory, clean-clone report, and combined Explorer | development candidate builder from the sealed and verified v5 graph; provider identity remains technical provenance | yes |

The candidate builder admits the registered v5 study graph. The raw
39-MB capture is not duplicated as a naked public file: it remains byte-exact
inside the reviewed response bundle, beside the sealed policy, rights evidence,
checksums, and exact false-positive review. The signed manifest inventories the
bundle, study parents, analysis, outer capsule, ledger, challenges,
reproduction report, and an explicit direct-asset omission record. Owner-key
signing, remote downloaded-byte verification, and publication remain external
actions. The capture and derived analysis are project artifacts covered by the
repository license when distributed; the attested B.AI route remains provenance only.

The trajectory data is 226 MB unpacked and is deliberately not in git history, so
CI has nothing to pack. It has to be uploaded from a working copy that has
fetched it.

**The two release-root checksum files are named apart on purpose.**
`SHA256SUMS` covers the flat GitHub release assets; `SHA256SUMS-eval-data`
covers the separately uploaded trajectory archives. The candidate archive keeps
its own internal `SHA256SUMS` for its closed graph. Reusing one release-root name
would silently replace a checksum set and make direct verification misleading.

## Authorized release procedure

```bash
# 1. Verify locally before creating any tag or remote state.
scripts/tests/run-tests.sh

# 2. Build the complete unsigned development candidate outside the repository.
candidate_parent="$(mktemp -d)"
candidate="${candidate_parent}/evalwitness-$(go run ./cmd/evalwitness version)"
scripts/build/build-release-candidate.sh --destination "${candidate}"

# 3. Before any tag publication, build a separate signed candidate with the
#    owner-approved key. The key is read-only, must be mode 0600, and is never
#    copied. Unsigned candidates are development-only and the tag workflow
#    refuses to publish one.
signed_candidate="${candidate_parent}/evalwitness-signed-$(go run ./cmd/evalwitness version)"
scripts/build/build-release-candidate.sh \
  --destination "${signed_candidate}" \
  --key-file /absolute/path/to/approved-ed25519-private-key.hex

# The tag workflow alone records publication authorization. It first requires
# exact tag v<product-version> to resolve to HEAD, then builds with:
# --external-publication authorized_by_tag

# 4. Re-run the one complete verifier without trusting README prose.
"${signed_candidate}/assets/binary/evalwitness-$(go env GOOS)-$(go env GOARCH)" \
  release verify \
  --assets "${signed_candidate}/assets" \
  --manifest "${signed_candidate}/release-manifest.json" \
  --sbom "${signed_candidate}/evalwitness.spdx.json" \
  --statement "${signed_candidate}/release.intoto.json" \
  --signature "${signed_candidate}/signature"

# 5. Repeat steps 1 through 4 from a fresh clone at the exact candidate commit.
#    Present both verification reports and remaining limitations to the owner.
#    Only explicit publication authorization permits a tag, push, upload, or announcement.
```

The candidate builder refuses a dirty source tree, an in-repository destination,
an existing destination, missing browser dependencies, noncanonical five-binary
inventory, path-bearing or target-inconsistent embedded build metadata, missing
evidence role, source/module-graph drift, a non-USTAR or PAX-bearing source
archive, a false source-archive report, missing or dirty portable source
provenance, secret-bearing public material, and incomplete signature input. A
tag-authorized candidate additionally requires the exact version tag at HEAD.
The builder creates no remote, tag, registry, release, or announcement.

The cross-platform release build uses stripped symbols, an empty Go build ID,
eight-byte function alignment, and a project-only no-inlining profile
(`github.com/Christopher-Schulze/evalwitness/...=-l`). This keeps the 20 MiB
artifact contract deterministic while leaving development and test builds, the
Go standard library, and external dependencies on their normal optimizer
settings. The archive round-trip uses the identical profile before comparing the
host binary byte-for-byte.
Before building, it runs the curated tracked-source safety gate. Its final public
scan applies generic credential, environment-dump, and private-path rules to
text files; opaque files are checked only for exact registered secret-bearing
environment values because the binaries embed the scanner's own detector
patterns. The report exposes both file classes. Clean-source provenance,
VCS-independent embedded build settings, and byte-identical repository and
archive-only builds cover the remaining release boundary; this is not a claim
of exhaustive executable secret analysis.

## Exact-response bundle admission

The v5 empirical exact-response bundle now exists locally at
`eval/governance/identical-response-capsule-v5.tar.gz`; the shipped golden
bundle remains synthetic conformance evidence. The v5 bundle was captured as
canonical newline-terminated schema 3 under the authorized study and its
redistribution evidence is embedded. Its sealed policy declares
`lineage_policy="complete_research"`; `exact_fixture` is synthetic-only and is not
admissible as empirical evidence. Every record must pass complete research-lineage
validation and bind the exact study-cell identity named by the policy. The producer
source tree must be a clean commit, and the binary's embedded revision and modified
state must match it.

The v5 bundle is a local candidate-builder asset together with the outer
identical-response capsule, sealed ledger, 34-receipt challenge pack, clean-clone report,
and derived Explorer. It is not a signed or published release until the approved
owner key, exact tag, hosted workflow, downloaded-byte verification, and release
publication all succeed.

The release operator supplies four reviewed inputs: an unsealed policy draft,
the exact public redistribution-evidence file, the exact bundle-producing binary,
and one or more named schema-3 captures. The packer seals source tree, binary,
toolchain, request corpus, route/model, licenses, omission policy, and evidence
bytes, then publishes only to absent targets:

```bash
: "${policy_draft:?set reviewed policy draft path}"
: "${redistribution_evidence:?set reviewed public evidence path}"
: "${producer_binary:?set exact producing binary path}"
: "${response_bundle:?set absent destination path}"
: "${capture_name:?set canonical capture name}"
: "${schema3_capture:?set exact schema-3 capture path}"
scripts/build/pack-response-bundle.sh \
  --policy-draft "${policy_draft}" \
  --redistribution-evidence "${redistribution_evidence}" \
  --producer-binary "${producer_binary}" \
  --repository-root . \
  --destination "${response_bundle}" \
  --archive "${response_bundle}.tar.gz" \
  --capture "${capture_name}=${schema3_capture}"
"${producer_binary}" replay bundle verify --source "${response_bundle}"
```

Verification must report the expected source-tree, binary, redistribution-
evidence, policy, index, capture, and archive digests together with
`lineage_policy="complete_research"`, `provider_calls=0`,
`network_required=false`, `exact_replay=true`, and `valid=true`. The release
operator must also see `evidence_ceiling="record_complete_external_parents_unresolved"`
on the bundle and resolve the named study, source-evidence, route-attestation,
capture-run, authorization, and analysis parents through the committed outer
identical-response capsule, whose scoped ceiling is
`record_complete_external_parents_resolved`; bundle validity alone is not
empirical release readiness. The release checksum manifest covers the bundle
archive; the
capsule inventory and checksums cover the embedded policy and rights-evidence
components. The authorized release supplies signatures and remote anti-rollback proof. Schema-1
cache entries, credentials, local configuration, operational logs, and provider-
account metadata are never admitted.

## Versioning

`internal/product/version.go` is the only product-version source. The CLI,
user agent, MCP identity, capsule build-provenance v2 record, release asset
builder, and tag gate consume that exact SemVer value. Release tags add only the
conventional `v` prefix. Linker flags and environment variables cannot override
the version, so an arbitrary tag cannot silently relabel a binary.

The current source version is `0.2.0`. This identifies the candidate code; it is
not evidence that `v0.2.0` has been tagged, pushed, signed, or published. The
release workflow rejects a tag unless it equals `v$(evalwitness version)`.

Artifacts under `eval/results/` record the binary version in their `route` block,
so a published measurement stays attributable to the code that produced it.

## Release integrity model

`release-manifest.json` is canonical JSON and inventories every regular file
under `assets/`. The allowed roles are `binary`, `capsule`, `documentation`,
`evidence`, `protocol`, `replication`, and `source`; all seven must be present.
The binary role is closed to darwin amd64/arm64, linux amd64/arm64, and windows
amd64. Symlinks, special files, empty files, unknown roles, additional binaries,
and concurrent file changes reject.

`release source-archive` accepts only the exact clean current HEAD at the
canonical Git top level and an absent canonical destination outside the
repository. It enumerates regular blobs and executable bits from `git ls-tree`,
rejects symlinks, submodules, special modes, noncanonical paths, and non-ASCII
paths, then reads the exact committed blobs rather than mutable worktree bytes.
The archive has one versioned root, an exact sorted directory block, a sorted
file block, USTAR headers only, Unix-epoch times, owner IDs zero, modes `0644`
and `0755`, and gzip OS 255. It publishes without overwrite, re-inspects the
archive through the hostile-input safety layer, and emits a canonical
`evalwitness.source-archive-report.v1`. Manifest construction and release
verification bind and independently recheck that report, including strict
format, inventory counts, expanded bytes, archive digest, and source-tree digest.

`release source-index` serializes the same clean
`evalwitness.provenance.source-tree.v1` used by capsule and build provenance.
Archive-only consumers opt in through the exact pair
`EVALWITNESS_REPOSITORY_ROOT=/absolute/extracted/root` and
`EVALWITNESS_SOURCE_TREE_PROVENANCE=/absolute/manifest-bound/source-tree-provenance.json`.
The loader accepts only canonical regular evidence, verifies every declared path,
byte count, file digest, total, clean status, commit, and graph digest against the
extracted root, rejects additional files and links, and ignores only an ephemeral
`.git` harness used by repository-dependent unit tests. The portable loader is
root-bound, so nested temporary Git-fixture tests retain their own provenance.
The harness receives a synthetic local commit and never supplies release
provenance. No Git history or hidden object database enters the release source
graph.

The SPDX document reads embedded Go build information from every binary. It
requires `-trimpath=true`, `CGO_ENABLED=0`, the filename's exact target, and no
embedded VCS settings. The manifest binds the exact commit and source
archive instead, allowing the archive-only build to reproduce the host binary
byte-for-byte without a hidden Git object database. EvalWitness source is
declared MIT. Dependency and compiled-binary license conclusions remain
`NOASSERTION` rather than being guessed. SPDX 2.3 is used for current GitHub
artifact interoperability; this does not claim that it is the newest SPDX
model.

The release documentation includes `THIRD_PARTY_NOTICES.md`. Its inventory is
checked against the exact Go module graph and Bun production dependency closure;
the Apache-2.0 and Lucide/Feather license bodies are byte-identical to the
installed sources used by the build.

The source role includes a closed local Go file proxy for the six modules in the
complete module graph. The round-trip gate starts with empty Go module and build
caches, disables checksum-database and toolchain network fallback, resolves only
that manifest-bound `file://` proxy, safely extracts the manifest-verified canonical
source archive, verifies the manifest-bound portable source-tree provenance, runs the Go
suite except the explicitly opt-in `internal/stress` package and builds with
`-mod=readonly`, records whether the separately distributed
trajectory cache is present, rejects any post-test source mutation, runs the
public-source scan, rebuilds the host binary byte-identically, and
reproduces every packaged capsule, claim, challenge, public claim surface, Claim
Autopsy report, v5 outer-capsule verification, and combined Explorer
byte-for-byte. This is local mechanism evidence for the recorded host and exact
Go toolchain, not independent replication.

The release in-toto Statement binds the byte-exact manifest and SBOM as two
subjects and repeats only their release identity, counts, and explicit current
truth state. The local explicit-key path signs those canonical Statement bytes
with DSSE and emits a closed trust root and threshold policy. `release verify`
recomputes the asset inventory and SBOM from the binaries, rebinds the Statement,
and verifies the signature. Unsigned candidates require the conspicuous
`--allow-unsigned-development` flag. Tag workflows require the owner-key
signature and additionally use GitHub's keyless artifact attestation. That
platform attestation is separate from the owner-key signature and both must be
verified after download.

## What a release does not promise

Version numbers track the code, not the findings. A measurement in
`docs/findings.md` belongs to the route and date recorded in its artifact, and a
later release does not revalidate it. Where a finding changes, the artifact is
regenerated and `scripts/tests/run-claimcheck.sh` fails until the prose is
corrected. That gate and the verified claim ledger, not the version number, keep
the two in step. A capsule Statement and a separate release Statement are
emitted locally. Explicit-key DSSE is implemented, but key custody exists only
when the owner supplies and authorizes a key. Keyless transparency evidence and
remote anti-rollback proof exist only after the tag workflow and downloaded
assets are independently verified; neither is inferred from a local candidate.

<!-- evalwitness:claim-surface:release:begin -->
### Machine-verified claim view

This block is deterministically derived from the canonical claim ledger and verified capsule. Edit the ledger or evidence, never these rows.

| Claim | Status | Level | Exact value | Governed statement | Required caveat |
|---|---|---|---|---|---|
| CLM-003 | supported | E1 | string:digest-bound-external-binary | The capsule digest-binds an external binary to the recorded source snapshot and build metadata | Digest binding does not prove a reproducible build or safety for untrusted use |
| CLM-029 | unsupported | E0 | boolean:false | Current filesystem, archive, transcript, logging, and release behavior is safe for untrusted public use | The reference proves bounded artifact handling, not blanket safety for untrusted public use |
| CLM-030 | unsupported | E1 | string:not_run | Warm-cache regeneration is a complete scientific reproduction | Replay and collection clocks are separate; clean-clone reproduction remains required |
| CLM-034 | unsupported | E0 | string:not_authorized | Downloadable releases currently exist at the README release URL | The local repository has no publication proof; release publication requires explicit authorization |

View digest: `363246681b7111d5d0c7201cadadd9fdc36375601f8b6403f7f5c9ed221ebb0c`

<!-- evalwitness:claim-surface:release:end -->
