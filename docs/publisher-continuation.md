# Publisher-set continuation source path

This is an unpublished source capability of the official Edge Form publisher,
not a second Takoform trust profile or authorization to sign or release. The
current default roster remains the same 17 published Forms. The two Container
source candidates remain outside that roster and outside released paths.

An eventual no-revocation successor starts from the exact latest public set,
read anonymously from canonical `main` and its immutable tags. The publisher
CLI accepts exactly two explicit, source-controlled, Core-verified release
roots. It prepares a create-only external directory containing the inherited
checkpoint raw bytes and Sigstore bundle, complete inherited statement and
checkpoint history, all 19 canonical package-index subjects, and the
publisher-owned `publisher-set/lineage.json`. This signed subject binds the
immediate `previousSetId`, previous and current checkpoint pins (equal for a
continuation), and all 19 exact package-index paths and digests. All 19
indexes and the lineage subject are signed from the same new source commit.
The old checkpoint is never re-signed. This mode creates no statement, new
checkpoint, second genesis, or `forms/revocations/v*` tag.

The preparation command is:

```console
go run ./cmd/publisher-trust prepare-continuation \
  --repository . --previous-set <latest-public-set-id> \
  --new-release-path forms/releases/<release-id-a>/<artifact-id-a> \
  --new-release-path forms/releases/<release-id-b>/<artifact-id-b> \
  --output <empty-external-directory>
```

The CLI refuses a dirty checkout, a source commit different from public main,
or a predecessor that cannot be verified from fresh anonymous public evidence.
The verification and install commands always use released Core v1.1.0 for the
actual Sigstore bundles and cumulative checkpoint capability. Synthetic test
signatures cannot pass their public entrypoints. A later publication decision
must separately promote an exact 19-package roster, preserve the 17 released
identities, connect the signing workflow, and update the publication plan's
preflight and anonymous readback. Until then, `bun run deploy --
form-packages-edge` must remain fail-closed for a 19-package continuation.

Later *real revocations* can each extend the immediately preceding set's
checkpoint through Core's normal one-statement cumulative extension. Each
signed lineage subject names the immediate set as `previousSetId`, even when
the checkpoint being extended was signed by an older set. The package roster
comes from that exact verified predecessor, not from the current default 17.
The new checkpoint, all current package indexes, and lineage are signed from
the new source commit; each genuine advancement alone can create its new
revocation tag.
