# Append-only Form Package revocations

Each `<statementVersion>.json` is one immutable, consecutively sequenced
security revocation for an exact `(FormRef, packageDigest)`. It must satisfy
the released public Core's embedded `form-package-revocation.schema.json` and
is delivered by the matching `forms/revocations/v<statementVersion>` tag. Each
matching
`checkpoints/<statementVersion>.json` is a cumulative index of every statement
from sequence 1 through the current sequence and includes the previous
checkpoint's canonical SHA-256 digest.

The source layout is:

```text
forms/revocations/<statementVersion>.json
forms/revocations/checkpoints/<statementVersion>.json
```

An installed set retains every canonical statement at
`revocations/statements/`, every older signed checkpoint and bundle at
`revocations/history/checkpoints/`, and the new signed head at
`revocations/checkpoint.json` plus `revocations/checkpoint.sigstore.json`.
The signed checkpoint entry contains the exact statement digest, sequence,
version, package digest, and FormRef. A second statement signature format is
not invented.

Only new statement and checkpoint files may be added. Never edit, rename,
delete, supersede in place, or reuse a version or sequence. A corrected
decision is a new statement plus cumulative checkpoint. The workflow signs the
checkpoint, and hosts retain its `(sequence, checkpoint digest, cumulative
entries digest)` pin before accepting the next checkpoint. Revocation blocks new create/update and activation but retains the
referenced bytes for observe/delete and operator evacuation. Ordinary
deprecation belongs in the Form Definition status instead.

The first publisher set uses the exact Core API v1 genesis checkpoint:

```json
{
  "apiVersion": "trust.forms.takoform.com/v1",
  "checkpointVersion": "0.0.0",
  "entries": [],
  "kind": "FormPackageRevocationCheckpoint",
  "previousCheckpointDigest": null,
  "sequence": 0
}
```

`bun run prepare:trust` derives these canonical bytes from Core v1.1.0. The
publisher workflow signs them as `revocations/checkpoint.sigstore.json`; the
candidate verifier requires that bundle, the repository-pinned publisher
policy and trusted root, and `CheckNotRevoked` for every package before a set
can be installed. A serialized verification report is not evidence.

For sequence 1 and later, follow the exact
[revocation advancement runbook](../../docs/revocation-advancement.md). Each
successful advancement creates one new set tag and one
`forms/revocations/v<statementVersion>` tag in one atomic non-force push while
preserving all 16 package tags. `statementVersion` orders trust-log evidence;
it is pinned metadata, not a third version axis. It never changes the two
Takoform axes: API/Core SemVer and the referenced Form's `definitionVersion`.

No revocation statement or signed publisher set has been published yet.
