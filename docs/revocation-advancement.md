# Revocation advancement runbook

Use this procedure to extend the signed Core API v1 revocation checkpoint by
one statement. It does not authorize signing or publication: protected GitHub
OIDC, independent review, and the operator remain separate boundaries. For
the general package publication path, see [publishing](publishing.md); for a
successor without a revocation, see
[publisher-set continuation](publisher-continuation.md).

## 1. Add one source statement and checkpoint

Start from a fresh, clean clone of canonical public `main`. Identify the unique
latest verified set from the last successful public verification. Do not take
a checkpoint pin from an issue, workflow input, or serialized report.

Add exactly these files:

```text
forms/revocations/<new-version>.json
forms/revocations/checkpoints/<new-version>.json
```

The canonical statement uses `apiVersion:
trust.forms.takoform.com/v1`, the next sequence, a never-used non-`0.0.0`
SemVer `statementVersion`, and one exact `(FormRef, packageDigest)`. It blocks
new create/update and activation while preserving observe/delete and
evacuation. Build the canonical cumulative checkpoint with released Core
v1.1.0: derive the entry with `RevocationCheckpointEntryForStatement`, retain
all prior entries, increment the sequence, set `previousCheckpointDigest` to
the verified prior pin, then require
`VerifyRevocationCheckpointExtension(priorPin, checkpointBytes)` to pass.

Do not edit or reorder prior files. Review and commit the pair through
protected `main`; source on `main` stages the change, while the signed set and
immutable tags deliver it.

## 2. Prepare and sign from public evidence

Dispatch `.github/workflows/form-package-signing.yml` at the exact public
protected-main commit with:

```text
expected_commit=<current public main commit>
previous_set=<latest verified public set source commit>
statement_version=<new-version>
```

The two advancement inputs must be supplied together. Blank values are only
for sequence-zero genesis when no public set or revocation tags exist. An
omitted `transition_mode` preserves the existing input pair; set
`transition_mode=advancement` explicitly if desired. Do not select
`transition_mode=continuation` for a statement/checkpoint pair.

Before requesting OIDC signing, `prepare-advancement` performs a fresh
credential-free clone of public `main`, inventories set and revocation tags,
fetches their immutable commits, compares retained source paths with public
`main`, replays the signed checkpoint history, and proves the supplied set is
the unique head. It rechecks `main` after preparation. It accepts no caller
checkpoint pin, evidence directory, or verification report. The signed
subjects are the new checkpoint, the complete active package roster from the
verified predecessor, and a lineage subject when the predecessor uses signed
lineage. The statement is bound by its checkpoint entry and is not signed as
a separate subject.

Download the one-day artifact to an external temporary directory. Do not edit
its contents.

## 3. Verify and install create-only

In a clean checkout of the exact signed source commit, run:

```console
bun run verify:trust -- --evidence <candidate> --expected-source-commit <commit>
bun run install:trust -- --evidence <candidate> --expected-source-commit <commit>
```

Verification must report the new sequence and statement version, immediate
predecessor set, complete set/checkpoint histories, active package subjects,
and `forms/revocations/v<new-version>`. After a continuation, the immediate
predecessor set can differ from the signer of the preceding checkpoint; keep
both identities exact. Commit the newly created
`forms/trust/sets/<commit>/` normally. Never finish or overwrite an existing
set directory in place.

## 4. Review, publish, and read back

After the installed-set commit is reviewed and is clean `main`, inspect the
dry run:

```console
bun run deploy -- form-packages-edge --trust-set <signed-source-commit> --dry-run
```

It must identify the unique prior public set, all current and previously
published package tags, the create-only `forms/sets/<signed-source-commit>`
tag, and exactly one new `forms/revocations/v<new-version>` tag. Then publish
once with the ordinary atomic non-force operation:

```console
bun run deploy -- form-packages-edge --trust-set <signed-source-commit>
```

Confirm delivery through anonymous readback:

```console
bun run deploy -- form-packages-edge --trust-set <signed-source-commit> --verify
```

Readback requires exact set/revocation tag prefixes, matching atomic
checkpoint-set publication, byte-equal tagged source, a valid Core checkpoint
chain, intact package roots and tags, and every active package unrevoked at the
new head.

If a push reports failure after mutation begins, treat acknowledgement as
indeterminate. Do not delete, recreate, force, or manually move refs. Rerun
the exact deploy command once: it settles as `PUBLISHED_SETTLED` only when
public `main`, all expected tags and bytes, and the signed Core report match.
Any mismatch requires investigation and forward repair. Rollback, a second
genesis, forks, rewritten prefixes, retags, updates, and deletions are never
repaired in place.

## Recover an interrupted local install

The safe default is to discard the checkout, create a fresh clean clone, and
verify/install there. Never remove a set that is tracked, committed, complete,
tagged, or visible on public `main`.

For an interrupted install that claimed the final directory but did not finish
writing it, use:

```console
bun run recover:trust -- --set-id <signed-source-commit>
```

Recovery removes only the exact set directory after proving it is a regular
tree without symlinks or special files, every member is untracked, the
repository is otherwise clean, no reachable local commit contains it, it is
not a complete Core-verified set, no public set tag or public history contains
it, and the public and local checks still hold immediately before removal. If
any proof fails, stop. Never use manual `rm -rf`. A committed or public
identity is repaired only by a new reviewed source commit, signed set, and
forward publication.
