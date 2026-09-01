# Signed publisher evidence

This directory owns public publisher inputs and immutable signed evidence for
the 17 current Edge Form Packages. It does not grant Host support or
admission. Publication separately retains two exact historical package roots,
so the release tree and tag readback contain 19 roots without adding them as
signed subjects.

`publisher-policy.json` pins the exact GitHub Actions OIDC issuer, repository,
workflow, and `refs/heads/main` identity accepted for this publisher.
`trusted-root.json` is the exact public Sigstore trusted root passed to released
Takoform Core v1.1.0. Neither file contains a credential or private key.
Its bytes are identical to
`takoform@v1.1.0/trust/testdata/trusted-root.json` (SHA-256
`6494e21ea73fa7ee769f85f57d5a3e6a08725eae1e38c755fc3517c9e6bc0b66`).
This repository pins those bytes as its publisher choice; Core does not supply
an ambient or privileged root.

`bun run prepare:trust -- --output <empty-external-directory>` writes the exact
RFC 8785 package-index subjects and the Core API v1 revocation genesis for the
first set. For sequence 1 and later, the manual workflow runs
`prepare-advancement` with a public predecessor set ID and new statement
version. That command accepts no caller checkpoint pin or evidence directory:
it clones public `main` without credentials, verifies the exact public set and
tag prefixes, replays the bounded signed checkpoint history, and copies the
retained statements and signed checkpoints into the new request. The workflow
adds one new checkpoint Sigstore v0.3 message-signature bundle and 17 current
package bundles. There is no separate statement bundle: the signed canonical
checkpoint entry binds the exact canonical statement digest and identity. The
workflow uploads a short-lived candidate and does not publish, tag, or write
this repository.

After external signing, an operator runs:

```console
bun run verify:trust -- --evidence <candidate> --expected-source-commit <commit>
bun run install:trust -- --evidence <candidate> --expected-source-commit <commit>
```

Install first reruns Core verification for all 17 exact package subjects and
the complete signed checkpoint chain from genesis, checks every package is not
revoked, requires one identical current publisher/root/source provenance, and
only then creates
`sets/<source-commit>/`. An existing set is never replaced. Serialized reports
are not accepted as verification capability or stored in a set.

The installer atomically claims the set directory before writing its members
and opens every file create-only. A process failure can therefore leave an
incomplete set that fails `bun run check:trust`; it cannot silently replace or
complete an existing identity. Prefer discarding the checkout and installing
again in a fresh clean clone. The only in-place recovery is
`bun run recover:trust -- --set-id <source-commit>`: it removes the exact
directory only after proving it is entirely untracked, absent from all
reachable local commits, absent from a credential-free public set tag, absent
from a fresh public `main` and all reachable public history, and the repository
is otherwise clean. It repeats the public and local proofs immediately before
removal. A tracked, committed,
complete, tagged, or public set is never removed; repair is a new signed source
commit and set. See the
[operator runbook](../../docs/revocation-advancement.md#recover-an-interrupted-local-install).

The deploy surface requires one installed set and performs the same
credential-free verification after public readback. It verifies the 17 current
signed package subjects plus the two retained release roots (19 immutable
roots/tags total). The package tags remain
the Core-derived content identities and may retain their older immutable Git
commit when the tagged package bytes exactly match the signed source.
`forms/sets/<source-commit>` identifies the signed publisher evidence closure.
Every checkpoint records the complete ordered publisher-set history. Deploy
requires that exact public set-tag prefix, pairs each sequence 1+ checkpoint
set with its revocation tag, and rejects fork, rollback, update, deletion, or
an unexpected set. Repair creates a new source commit and set rather than
changing an existing identity.
