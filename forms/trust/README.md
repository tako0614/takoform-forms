# Signed publisher evidence

This directory stores the publisher's trust inputs and immutable signed-set
evidence for Edge Form Packages. A signed set proves publisher provenance for
its exact package subjects; it does not grant Host support, admission,
installation, activation, or deployment. For the package publication overview,
see [publishing](../../docs/publishing.md).

## Checked-in evidence

The checkout contains two set directories. Set
`e7f8a39311dd011b8467e97e7f300cabb9a6b06c` carries the sequence-zero genesis
checkpoint. Set `cdd30b711e2c6857b1b4d247b1471f5676904933` is cryptographically
verified but explicitly abandoned as `evidence-only`: three package identities
changed before publication. Its old ObjectBucket, WorkerDeployment, and
WorkerVersion roots remain available for audit/readback, but their abandoned
set tag and package tags must remain absent. These are checked-in facts; this
file does not assert the current state of remote refs.

`forms/trust/abandoned-prepublication.json` records the one-time abandoned
set. Do not append another abandoned set to that singleton format without an
architecture decision. The selected active roster is derived from the pinned
current-family index and candidate set; see the [package inventory](../README.md#v1-package-history) for source-tree
counts.

## Trust inputs and workflow

`publisher-policy.json` pins the accepted GitHub Actions OIDC issuer,
repository, workflow, and `refs/heads/main` identity. `trusted-root.json` is
the exact public Sigstore root passed to released Takoform Core v1.1.0. This
repository chooses those bytes explicitly; Core supplies no ambient publisher
root. Neither file contains a credential or private key.

For the first signing request, `bun run prepare:trust -- --output
<empty-external-directory>` derives the canonical package-index subjects and
genesis checkpoint. A real revocation advancement uses the
[revocation runbook](../../docs/revocation-advancement.md). A no-revocation
successor uses the [continuation path](../../docs/publisher-continuation.md).
Both preparation paths derive predecessors from fresh, credential-free public
evidence; callers cannot supply a checkpoint pin or serialized verification
report as authority.

After signing, verify and install the candidate from a clean checkout of its
exact source commit:

```console
bun run verify:trust -- --evidence <candidate> --expected-source-commit <commit>
bun run install:trust -- --evidence <candidate> --expected-source-commit <commit>
```

Verification checks every exact package subject and the complete signed
checkpoint chain using released Core v1.1.0. Installation creates
`forms/trust/sets/<source-commit>/` with create-only writes. An existing set
is never replaced. If an interrupted local install leaves an incomplete
untracked directory, use the guarded
[`recover:trust` procedure](../../docs/revocation-advancement.md#recover-an-interrupted-local-install);
otherwise discard the checkout and install in a fresh clone.

Publication and anonymous readback are handled by `bun run deploy`; see the
[publication overview](../../docs/publishing.md) for its exact obligations.
Published package paths and tags are immutable. Repair forward with a new
source commit and a new set.
