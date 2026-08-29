# Append-only Form Package revocations

Each `<statementVersion>.json` is one immutable, consecutively sequenced
security revocation for an exact `(FormRef, packageDigest)`. It must satisfy
the released public Core's embedded `form-package-revocation.schema.json` and
is delivered by the matching `forms/revocations/v<statementVersion>` tag. Each
matching
`checkpoints/<statementVersion>.json` is a cumulative index of every statement
from sequence 1 through the current sequence and includes the previous
checkpoint's canonical SHA-256 digest.

Only new statement and checkpoint files may be added. Never edit, rename,
delete, supersede in place, or reuse a version or sequence. A corrected
decision is a new statement plus cumulative checkpoint. The workflow signs the
checkpoint, and hosts retain its `(sequence, checkpoint digest, cumulative
entries digest)` pin before accepting the next checkpoint. Revocation blocks new create/update and activation but retains the
referenced bytes for observe/delete and operator evacuation. Ordinary
deprecation belongs in the Form Definition status instead.

The first publisher set uses the exact Core API v1 genesis checkpoint:

```json
{"apiVersion":"trust.forms.takoform.com/v1","checkpointVersion":"0.0.0","entries":[],"kind":"FormPackageRevocationCheckpoint","previousCheckpointDigest":null,"sequence":0}
```

`bun run prepare:trust` derives these canonical bytes from Core v1.1.0. The
publisher workflow signs them as `revocations/checkpoint.sigstore.json`; the
candidate verifier requires that bundle, the repository-pinned publisher
policy and trusted root, and `CheckNotRevoked` for every package before a set
can be installed. A serialized verification report is not evidence.

No revocation statement or signed publisher set has been published yet.
