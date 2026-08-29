# Signed publisher evidence

This directory owns public publisher inputs and immutable signed evidence for
the 16 Edge Form Packages. It does not grant Host support or admission.

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
RFC 8785 package-index subjects and the Core API v1 revocation genesis. The
manual signing workflow may add only Sigstore v0.3 message-signature bundles;
it uploads a short-lived candidate and does not publish, tag, or write this
repository.

After external signing, an operator runs:

```console
bun run verify:trust -- --evidence <candidate> --expected-source-commit <commit>
bun run install:trust -- --evidence <candidate> --expected-source-commit <commit>
```

Install first reruns Core verification for all 16 exact package subjects and
the signed genesis, checks every package is not revoked, requires one identical
publisher/root/source provenance, and only then creates
`sets/<source-commit>/`. An existing set is never replaced. Serialized reports
are not accepted as verification capability or stored in a set.

The installer atomically claims the set directory before writing its members
and opens every file create-only. A process failure can therefore leave an
incomplete set that fails `bun run check:trust`; it cannot silently replace or
complete an existing identity. Repair uses a newly signed source commit and
set.

The deploy surface requires one installed set and performs the same
credential-free verification after public readback. The package tags remain
the Core-derived content identities and may retain their older immutable Git
commit when the tagged package bytes exactly match the signed source.
`forms/sets/<source-commit>` identifies the signed publisher evidence closure.
Repair creates a new source commit and set rather than changing an existing
identity.
