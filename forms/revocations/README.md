# Append-only Form Package revocations

This directory holds source statements and cumulative checkpoints for Core API
v1 package revocation. See the [publisher overview](../../docs/publishing.md)
for package identities and the
[advancement runbook](../../docs/revocation-advancement.md) for the maintainer
procedure.

Each `<statementVersion>.json` revokes one exact `(FormRef, packageDigest)`.
It must satisfy the released Core schema and is delivered by the matching
immutable `forms/revocations/v<statementVersion>` tag. Its matching
`checkpoints/<statementVersion>.json` extends the prior checkpoint with the
complete ordered statement history and the prior checkpoint's canonical
SHA-256 digest. The statement and checkpoint are the only source files added
for an advancement:

```text
forms/revocations/<statementVersion>.json
forms/revocations/checkpoints/<statementVersion>.json
```

The signed set retains every statement, prior signed checkpoint and bundle,
and the new signed checkpoint and bundle. The checkpoint signature binds the
canonical statement digest, sequence, version, package digest, and FormRef;
there is no separate statement-signature format. Never edit, rename, delete,
reuse, or supersede an existing statement or checkpoint. A correction is a
new statement and cumulative checkpoint. Hosts pin the checkpoint sequence,
digest, and cumulative entries digest before accepting its successor.

Revocation blocks new create/update and activation while retaining referenced
bytes for observe/delete and operator evacuation. Ordinary deprecation belongs
in the Form Definition status. `statementVersion` orders trust-log evidence;
it is not a third Takoform version axis and does not change API/Core SemVer or
the Form's `definitionVersion`.

The checkout currently contains the sequence-zero genesis checkpoint in its
signed-set evidence and no revocation statement source files. That describes
the checked-in tree only; remote publication status must be established by
anonymous readback.
