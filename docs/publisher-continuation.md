# Publisher-set continuation source path

This is an unpublished source capability of the official Edge Form publisher,
not a second Takoform trust profile or authorization to sign or release. The
current default roster remains the same 17 published Forms. The two Container
source candidates remain outside that roster and outside released paths.

An eventual no-revocation successor starts from the exact latest public set,
read anonymously from canonical `main` and its immutable tags. The publisher
CLI accepts the **complete active roster** as explicit release roots, but they
must exactly match the repository's current-family index and its digest-pinned
Edge candidate set (paths, FormRefs, and package digests). The caller's list
does not choose publication authority; the selected candidate projection does.
The CLI also rejects the immutable abandoned evidence-only identities. The
current projection selects 17 packages, so an unpromoted 19-path request fails.
It prepares a create-only external directory containing the inherited
checkpoint raw bytes and Sigstore bundle, complete inherited statement and
checkpoint history, every active canonical
package-index subject, and the publisher-owned `publisher-set/lineage.json`.
This signed subject binds the immediate `previousSetId`, previous and current
checkpoint pins (equal for a continuation), and every exact active package
index path and digest. All active indexes and the lineage subject are signed
from the same new source commit. The old checkpoint is never re-signed. This
mode creates no statement, new checkpoint, second genesis, or
`forms/revocations/v*` tag.

The preparation command is:

```console
go run ./cmd/publisher-trust prepare-continuation \
  --repository . --previous-set <latest-public-set-id> \
  --active-release-path forms/releases/<release-id>/<artifact-id> \
  --output <empty-external-directory>
```

Repeat `--active-release-path` for **every** root in the complete selected
active roster; the single path above illustrates the flag, not a release plan.
If source selection later changes, the owning candidate set and pinned
current-family index must be updated separately before this command can
prepare that roster. Historical signed-set verification remains independent
of whichever roster is current at verification time.

The CLI refuses a dirty checkout, a source commit different from public main,
or a predecessor that cannot be verified from fresh anonymous public evidence.
The preparation route Core-verifies each package closure and checks it against
the inherited checkpoint before signing. Verification and installation use
released Core v1.1.0 for every actual Sigstore bundle and the cumulative
checkpoint capability. Synthetic test signatures cannot pass their public
entrypoints. A later publication decision must separately promote an exact
active roster (19 for the first Container proposal), preserve all old released
identities, connect the signing workflow, and update the publication plan's
preflight and anonymous readback. Until then, `bun run deploy --
form-packages-edge` must remain fail-closed for a 19-package continuation.

Another no-revocation successor may follow a continuation or a real
advancement and select a new active Form version. The old package can leave
the active roster, but its release root, package tag, and historical signed
set remain byte-exact and independently verifiable. The latest signed
checkpoint pin, raw bytes, bundle, and full cumulative history remain exact.
Published-set verification bounds predecessor replay depth to 1024 sets so a
fabricated acyclic chain cannot recurse without limit.
It checks each set's lineage structure and signature before following that
set's predecessor claim; cycle and depth checks still apply on every call.

Later *real revocations* can each extend the immediately preceding set's
checkpoint through Core's normal one-statement cumulative extension. Each
signed lineage subject names the immediate set as `previousSetId`, even when
the checkpoint being extended was signed by an older set. The package roster
comes from that exact verified predecessor, not from the current default 17.
The new checkpoint, all active package indexes, and lineage are signed from
the new source commit; each genuine advancement alone can create its new
revocation tag.
