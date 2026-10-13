# Publisher-set continuation

Continuation creates a signed successor set without advancing revocation. It
does not create another trust profile or authorize signing or publication.
See [publishing](publishing.md) for the package publication overview and
[revocation advancement](revocation-advancement.md) when a statement is being
added.

The active release roster comes from the digest-pinned current-family index
and candidate set. The checked-in selection currently contains 17 Edge Forms;
the CLI requires the complete roster and verifies each package closure. A
caller-provided list cannot change the selection. Container source candidates
remain outside that selection until separately promoted in the owning source
manifests.

The separate `forms/source-history.json` inventory records nine unsigned
predecessor package roots from source commit
`85b2f755a0cf104caf1ae8cb3738e475d55fe988`. These roots are retained for
source-tree validation and readback only; they are not active packages,
signing subjects, signed history, or trust authority. Likewise, abandoned
evidence-only identities cannot enter a continuation. The [package inventory](../forms/README.md#v1-package-history) records the
complete checked-in release-root counts.

## Prepare a successor

Start from a clean checkout of the exact public `main` commit. Supply every
active release root to the preparation command:

```console
go run ./cmd/publisher-trust prepare-continuation \
  --repository . --previous-set <latest-public-set-id> \
  --active-release-path forms/releases/<release-id>/<artifact-id> \
  --output <empty-external-directory>
```

Repeat `--active-release-path` once for every selected package. The single
example path above is illustrative, not a roster. The command refuses a dirty
checkout, a source commit different from public `main`, an unverified public
predecessor, an incomplete or conflicting roster, and a package revoked by
the inherited checkpoint.

It copies the inherited checkpoint bytes and bundle, complete statement and
checkpoint history, and publisher policy/root into a create-only external
request. It also creates `publisher-set/lineage.json`, binding the immediate
predecessor set, unchanged checkpoint pin, and exact active package-index
paths and digests. The checkpoint is inherited byte-for-byte; it is not
re-signed. The signing workflow obtains the complete roster from
`bun scripts/form-publication.mjs --signing-roster` and signs each active
package index plus the lineage subject from the same source commit.

Dispatch `.github/workflows/form-package-signing.yml` with
`transition_mode=continuation`, `previous_set=<latest verified public set>`,
and a blank `statement_version`. Continuation is explicit; `previous_set`
alone does not select it. The workflow produces a short-lived candidate and
does not publish or tag the repository.

## Verify, install, and publish

From a clean checkout of the exact signed source commit:

```console
bun run verify:trust -- --evidence <candidate> --expected-source-commit <commit>
bun run install:trust -- --evidence <candidate> --expected-source-commit <commit>
```

Verification uses released Core v1.1.0 for every Sigstore bundle and the
inherited checkpoint. Synthetic test signatures cannot pass the public
entrypoints. Installation creates a new `forms/trust/sets/<commit>/` and
refuses to replace an existing identity. Review and commit that directory,
then use the normal deploy dry run, publish, and anonymous readback described
in [publishing](publishing.md).

A continuation adds a publisher-set identity but no statement, checkpoint,
genesis, or `forms/revocations/v*` tag. A later revocation must extend the
verified predecessor's checkpoint and use the
[advancement runbook](revocation-advancement.md). A later continuation may
select new package versions; prior release roots, tags, and signed histories
remain immutable and independently verifiable.
