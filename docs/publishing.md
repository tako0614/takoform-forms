# V1 package publication

This procedure covers the retained Host API v1 package pipeline. Host API v2
Form URL publication has a separate [site procedure](site.md#publish-through-this-repository);
package generation and signing do not publish those Markdown specifications.
The [Form inventory](../forms/README.md#v1-package-history) records the selected
source, signed history and evidence-only release roots.

## Generate and inspect

Go catalogs and `cmd/current-form-source` select the authoring source. Their
output is data-only; it contains no Host or Provider implementation.

```console
bun run generate
bun run write:publication
bun run check
```

`generate` writes the selected candidate trees and additional source-only
Container packages. `write:publication` materializes missing selected release
roots create-only. It does not sign or publish them, and never rewrites
historical, explicitly retained, source-history or abandoned evidence-only roots.
Review generated changes before committing them.

For a verified `package-index.json`, Core v1.1.0's `PublicationLocatorFor`
derives `releaseId` from the FormRef group and kind, and `artifactId` from the
package digest (`sha256:` becomes `sha256-`). The resulting path and tag are:

```text
forms/releases/<releaseId>/sha256-<digest>/
forms/<releaseId>/sha256-<digest>
```

Changing package bytes creates a new digest, path and tag. Published versions
must keep their original Definitions and bytes; author a new version when
meaning changes.

## Prepare, sign and install

The selected Actor/Workflow source differs from the installed signed set.
That set cannot provide signatures for the newer candidates. The deploy
surface requires evidence for the exact selected roster.

For the first genesis set only, prepare an exact protected-main source commit
for external keyless signing:

```console
bun run prepare:trust -- --output <empty-external-directory>
```

After the predecessor set is published and anonymously verified, use the
appropriate successor procedure:

- [Continuation](publisher-continuation.md) prepares an explicit complete
  active roster without a new revocation, preserving the signed checkpoint chain.
- [Revocation advancement](revocation-advancement.md) adds a new revocation
  statement and signed cumulative checkpoint.

The manual [`form-package-signing.yml`](../.github/workflows/form-package-signing.yml)
workflow is the publisher's signing authority. It uses GitHub Actions OIDC
and the pinned publisher policy, signs exact package-index and required
checkpoint/lineage subjects, verifies them and uploads a one-day candidate.
Its explicit modes are `genesis`, `advancement` and `continuation`. Blank mode
retains the genesis/advancement input-pair behavior; continuation must be
selected explicitly. Genesis refuses an existing public publisher history.
The workflow has no repository-write, tag or publication permission.

Verify and import the candidate create-only:

```console
bun run verify:trust -- --evidence <candidate> --expected-source-commit <commit>
bun run install:trust -- --evidence <candidate> --expected-source-commit <commit>
```

Installation creates evidence under `forms/trust/sets/<source-commit>/`.
The verified signature bundles carry publisher provenance; installation does
not establish public availability. See [trust evidence](../forms/trust/README.md) for the
exact subjects and authority checks, and the
[partial-install recovery procedure](revocation-advancement.md#recover-an-interrupted-local-install)
if installation is interrupted.

## Publish and read back

The package deploy surface requires the imported set's exact signed source
commit and the current roster. Inspect the live contract before selecting an
action:

```console
bun run deploy -- --contract
bun run deploy -- form-packages-edge --trust-set <source-commit> --dry-run
bun run deploy -- form-packages-edge --trust-set <source-commit>
bun run deploy -- form-packages-edge --trust-set <source-commit> --verify
```

`--dry-run` checks preconditions without mutation. The publish action pushes
`main` and matching immutable tags atomically; `--verify` performs anonymous
public readback. An existing package tag may point to an older commit only
when its package path is byte-identical to the signed source. Publisher evidence
uses a separate immutable `forms/sets/<source-commit>` identity. Revocation
advancement adds one `forms/revocations/v<statementVersion>` tag to that push.

Readback fetches the exact publishable current and signed-historical package
tags and compares their bytes. It also checks the untagged source-history
roots from the anonymous main tree and reruns the Core verifier over fetched
public release roots, comparing them with the local byte closure. Signature
bundles, pinned policy/root, signed checkpoint and not-revoked decisions are
verified separately. Local evidence and a successful
build are not a substitute for this public observation.

## Historical evidence and repair

The installed successor set is
`e7f8a39311dd011b8467e97e7f300cabb9a6b06c`. The earlier
`cdd30b711e2c6857b1b4d247b1471f5676904933` set remains cryptographically
verifiable but is explicitly abandoned as prepublication evidence. Its set
tag and the three superseded package tags must stay absent; deployment refuses
that set. Their bytes remain in the repository for audit.

Never overwrite, retag or delete a published identity. Repair forward with a
new Definition version or package digest and, when evidence changes, a new
publisher set. Settle a lost push acknowledgement through exact anonymous
readback. The deploy command does not push again when every public ref and byte
already matches. Forks, rollbacks, missing history, inserted tags and changed
bytes are refused; inspect the diagnostic before choosing a next action.

Publication proves package identity and bytes. Host trust admission,
installation, support, activation and resource availability remain separate
Host decisions.
