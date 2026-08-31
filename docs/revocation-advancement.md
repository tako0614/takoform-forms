# Revocation advancement runbook

This is the operator procedure for the official Edge Form Package publisher.
It advances one append-only Core API v1 revocation checkpoint and publishes
one immutable revocation tag. It does not authorize signing or publication:
the protected GitHub environment, an independent review, and the operator are
separate authority boundaries.

The workflow always signs 17 subjects: the new cumulative checkpoint and the
current 16 canonical package indexes. It does not invent a statement
signature. Released Takoform Core v1.1.0 derives the checkpoint entry from the
exact RFC 8785 canonical statement bytes; the signed checkpoint therefore
binds its sequence, `statementVersion`, statement digest, package digest, and
FormRef.

`statementVersion` is an immutable trust-log identifier. It is not a third
Takoform version axis and does not change API/Core SemVer or any Form's
`definitionVersion`.

## 1. Author one append-only source pair

Start from a fresh clean clone of canonical public `main`. Identify the latest
verified set ID from the last successful public verification record. Never
copy a checkpoint pin from an issue, workflow input, or serialized report.

Add exactly these two canonical JSON files:

```text
forms/revocations/<new-version>.json
forms/revocations/checkpoints/<new-version>.json
```

The statement uses `apiVersion: trust.forms.takoform.com/v1`, the next exact
sequence, a never-used non-`0.0.0` SemVer `statementVersion`, and one exact
`(FormRef, packageDigest)`. Its effects must block new create/update and
activation while retaining bytes for observe/delete and evacuation.

Construct the checkpoint with the released Core v1.1.0 primitives:

1. Parse the prior signed checkpoint and retain its verified
   `(checkpointApiVersion, sequence, digest, entriesDigest)` pin.
2. Call `RevocationCheckpointEntryForStatement` on the exact canonical new
   statement bytes.
3. Retain every prior entry byte-for-byte, append that one entry, set sequence
   to prior sequence plus one, set `checkpointVersion` to the new
   `statementVersion`, and set `previousCheckpointDigest` to the prior pin's
   digest.
4. Canonicalize the checkpoint and require
   `VerifyRevocationCheckpointExtension(priorPin, checkpointBytes)` to pass.

Do not edit, delete, rename, reorder, or regenerate any older statement or
checkpoint. Commit the pair through the normal protected-main review. Source
on `main` is staging, not a published revocation; the immutable tag and signed
set are the delivery identity.

## 2. Prepare and sign from public evidence

Dispatch `.github/workflows/form-package-signing.yml` at the exact current
protected-main commit with all three inputs:

```text
expected_commit=<current public main commit>
previous_set=<latest verified public set source commit>
statement_version=<new-version>
```

Blank `previous_set` and `statement_version` are permitted only for the first
sequence-zero genesis set, while no public set or revocation tags exist. The
two advancement inputs must otherwise be supplied together.

Before requesting OIDC signing, `prepare-advancement` performs a fresh
credential-free clone of canonical public `main`, reads the exact
`forms/sets/*` and `forms/revocations/v*` inventories, fetches their immutable
commits, compares every retained set/statement/checkpoint path with public
`main`, replays every signed checkpoint from genesis, and proves the supplied
set ID is the unique head. It rechecks public `main` after preparation. A
caller-supplied evidence directory, checkpoint pin, or verification report is
not accepted.

Download the one-day artifact only into an external temporary directory. Do
not edit it.

## 3. Verify and install create-only

In a clean checkout of the exact signed source commit:

```console
bun run verify:trust -- --evidence <candidate> --expected-source-commit <commit>
bun run install:trust -- --evidence <candidate> --expected-source-commit <commit>
```

Verification must report the exact new sequence, new statement version,
previous set ID, complete checkpoint history, 16 packages, and
`forms/revocations/v<new-version>`. Commit the newly created
`forms/trust/sets/<commit>/` directory normally. Never complete or overwrite a
pre-existing directory in place.

## 4. Review and publish once

After the installed set commit is reviewed and is clean `main`, inspect the
dry-run evidence:

```console
bun run deploy -- form-packages-edge --trust-set <signed-source-commit> --dry-run
```

The dry-run must name:

- the unique previous public set;
- all 16 Core-derived package tags;
- the create-only `forms/sets/<signed-source-commit>` tag; and
- exactly one new `forms/revocations/v<new-version>` tag.

Publication is one ordinary atomic push with direct refspecs. It contains no
force, delete, retag, or local tag-creation command:

```console
bun run deploy -- form-packages-edge --trust-set <signed-source-commit>
```

After acknowledgement, or independently at any time, verify anonymously:

```console
bun run deploy -- form-packages-edge --trust-set <signed-source-commit> --verify
```

The verifier clones public bytes without credentials, requires the exact set
and revocation tag prefixes, pairs every sequence 1+ revocation tag with its
atomic checkpoint-set publication, compares every tagged source path, replays
the bounded Core checkpoint chain, and verifies all 16 packages are not
revoked at the new head.

## 5. Settle a lost push acknowledgement

If the atomic push command returns failure after mutation starts, its result is
indeterminate. Do not delete, recreate, force, or manually move any ref. Rerun
the exact same deploy command once. If public `main`, every set/revocation tag,
all package tags, all bytes, and the signed Core report match exactly, the
command returns `PUBLISHED_SETTLED` without a second push. Any mismatch remains
blocked for investigation and forward repair.

Rollback, a second genesis, a forked predecessor, a rewritten prefix, a
missing tag, an extra tag, retagging, update, or deletion is never repaired in
place. Append a new statement/checkpoint and publish a new set. Package-byte
changes also require a new Core-derived package identity.

## Recover an interrupted local install

The safe default is to discard the checkout, make a fresh clean clone, and
verify/install the candidate there. Never remove a set that is tracked,
committed, complete, tagged, or visible on public `main`.

For the narrow case where `install:trust` was interrupted after claiming the
final directory but before completing it, use:

```console
bun run recover:trust -- --set-id <signed-source-commit>
```

The command removes only
`forms/trust/sets/<signed-source-commit>/` and only after all of these proofs:

- the exact target is a regular tree with no symlink or special file;
- every member is untracked and the repository is otherwise clean;
- no reachable local commit contains the path;
- the directory is not a complete Core-verified set;
- a credential-free canonical public set-tag lookup is empty;
- a fresh full credential-free public clone has no reachable commit containing
  the path; and
- public `main`, tag absence, and local uncommitted state still match
  immediately before removal.

If any proof fails, stop. Do not use manual `rm -rf` as a workaround. A public
or committed identity is immutable even if its bytes appear incomplete; the
only repair is a new reviewed source commit, signing run, set, and forward
publication.
