import { afterEach, describe, expect, test } from "bun:test";
import { createHash } from "node:crypto";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, join } from "node:path";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

import {
  deriveEdgeFormFreezeCandidates,
  FREEZE_KIND,
  FREEZE_PATH,
  verifyEdgeFormFreeze,
} from "./edge-form-freeze.mjs";

const temporaryRoots = [];

afterEach(() => {
  for (const root of temporaryRoots.splice(0))
    rmSync(root, { recursive: true, force: true });
});

function write(root, path, value) {
  const target = join(root, path);
  mkdirSync(dirname(target), { recursive: true });
  writeFileSync(target, value);
}

function git(root, ...args) {
  const result = spawnSync("git", args, { cwd: root, encoding: "utf8" });
  if (result.status !== 0) throw new Error(result.stderr || result.stdout);
  return result.stdout.trim();
}

function formSource(kind, version, body = "Normative Form definition.") {
  return `---\ntitle: ${kind} ${version}\ndescription: Authored Form for testing.\nformUrl: https://edge.forms.takoform.com/forms/${kind}/${version}/\nhostApi: forms.takoform.com/v2\n---\n\n# ${kind} ${version}\n\n${body}\n`;
}

function writeForm(root, kind, version, body) {
  const path = `spec/forms/${kind}/${version}/index.md`;
  write(root, path, formSource(kind, version, body));
  return path;
}

function createFixture() {
  const root = mkdtempSync(join(tmpdir(), "takoform-edge-form-freeze-"));
  temporaryRoots.push(root);
  git(root, "init", "--quiet");
  git(root, "config", "user.email", "freeze-test@example.invalid");
  git(root, "config", "user.name", "Freeze Test");
  write(root, ".gitignore", "\n");
  git(root, "add", ".gitignore");
  git(root, "commit", "--quiet", "-m", "fixture root");
  writeForm(root, "WorkerVersion", "0.5.0");
  writeForm(root, "ModuleWorker", "0.3.0");
  write(
    root,
    "spec/guides/index.md",
    "---\ntitle: Guides\ndescription: Reader guide.\n---\n\n# Guides\n\nMutable guide.\n",
  );
  return { root };
}

function createCliFixture() {
  const fixture = createFixture();
  mkdirSync(join(fixture.root, "scripts"), { recursive: true });
  copyFileSync(
    fileURLToPath(new URL("./edge-form-freeze.mjs", import.meta.url)),
    join(fixture.root, "scripts/edge-form-freeze.mjs"),
  );
  copyFileSync(
    fileURLToPath(new URL("./edge-v2-docs.mjs", import.meta.url)),
    join(fixture.root, "scripts/edge-v2-docs.mjs"),
  );
  return fixture;
}

function runCli(root, ...args) {
  return spawnSync(
    process.execPath,
    [join(root, "scripts/edge-form-freeze.mjs"), ...args],
    {
      cwd: root,
      encoding: "utf8",
    },
  );
}

function manifest(forms) {
  return { kind: FREEZE_KIND, forms };
}

function commitInitialFreeze(root, forms) {
  write(root, FREEZE_PATH, `${JSON.stringify(manifest(forms), null, 2)}\n`);
  git(root, "add", ".");
  git(root, "commit", "--quiet", "-m", "first Edge Form URL freeze");
  return git(root, "log", "-1", "--format=%H", "--", FREEZE_PATH);
}

describe("Edge Form immutable URL source freeze", () => {
  test("derives only authored Form URLs and exact Markdown byte hashes", () => {
    const { root } = createFixture();
    const candidates = deriveEdgeFormFreezeCandidates(root);
    expect(candidates.map(({ url }) => url)).toEqual([
      "https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/",
      "https://edge.forms.takoform.com/forms/WorkerVersion/0.5.0/",
    ]);
    expect(candidates.map(({ path }) => path)).toEqual([
      "spec/forms/ModuleWorker/0.3.0/index.md",
      "spec/forms/WorkerVersion/0.5.0/index.md",
    ]);
    expect(candidates[0].sha256).toMatch(/^sha256:[0-9a-f]{64}$/u);
  });

  test("reports unfrozen authored Forms as pending and requireFrozen rejects them", () => {
    const { root } = createFixture();
    const result = verifyEdgeFormFreeze(root);
    expect(result.status).toBe("UNFROZEN");
    expect(result.frozen).toEqual([]);
    expect(result.pending).toHaveLength(2);
    expect(verifyEdgeFormFreeze(root, { requireFrozen: true }).status).toBe(
      "INVALID",
    );
  });

  test("bootstrap validates a present candidate without creating it", () => {
    const { root } = createFixture();
    const candidates = deriveEdgeFormFreezeCandidates(root);
    write(
      root,
      FREEZE_PATH,
      `${JSON.stringify(manifest(candidates), null, 2)}\n`,
    );
    const before = readFileSync(join(root, FREEZE_PATH));
    const result = verifyEdgeFormFreeze(root, { mode: "bootstrap" });
    expect(result.status).toBe("BOOTSTRAP_CANDIDATE");
    expect(result.frozen).toEqual(candidates);
    expect(result.pending).toEqual([]);
    expect(readFileSync(join(root, FREEZE_PATH))).toEqual(before);
  });

  test("CLI verifies bootstrap candidates and rejects missing or empty candidates", () => {
    const missing = createCliFixture();
    const absent = runCli(missing.root, "--bootstrap");
    expect(absent.status).toBe(1);
    expect(absent.stderr).toContain("candidate is missing");

    const empty = createCliFixture();
    write(
      empty.root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([]), null, 2)}\n`,
    );
    const emptyResult = runCli(empty.root, "--bootstrap");
    expect(emptyResult.status).toBe(1);
    expect(emptyResult.stderr).toContain("at least one frozen Form URL");

    const candidate = createCliFixture();
    const forms = deriveEdgeFormFreezeCandidates(candidate.root);
    write(
      candidate.root,
      FREEZE_PATH,
      `${JSON.stringify(manifest(forms), null, 2)}\n`,
    );
    const before = readFileSync(join(candidate.root, FREEZE_PATH));
    const valid = runCli(candidate.root, "--bootstrap");
    expect(valid.status).toBe(0);
    expect(valid.stdout).toContain(
      "bootstrap candidate verified; no manifest was written",
    );
    expect(readFileSync(join(candidate.root, FREEZE_PATH))).toEqual(before);
  });

  test("CLI frozen check reports success and require-frozen rejects absence", () => {
    const absent = createCliFixture();
    const unfrozen = runCli(absent.root, "--check", "--require-frozen");
    expect(unfrozen.status).toBe(1);
    expect(unfrozen.stderr).toContain("requires frozen Edge Form URLs");

    const committed = createCliFixture();
    const [candidate] = deriveEdgeFormFreezeCandidates(committed.root);
    const firstAdd = commitInitialFreeze(committed.root, [candidate]);
    const frozen = runCli(committed.root, "--check", "--require-frozen");
    expect(frozen.status).toBe(0);
    expect(frozen.stdout).toContain(firstAdd);
  });

  test("a committed per-Form anchor freezes selected URLs but leaves new drafts pending", () => {
    const { root } = createFixture();
    const candidates = deriveEdgeFormFreezeCandidates(root);
    const firstAdd = commitInitialFreeze(root, [candidates[1]]);
    const result = verifyEdgeFormFreeze(root, { requireFrozen: true });
    expect(result.status).toBe("FROZEN");
    expect(result.firstAddCommit).toBe(firstAdd);
    expect(result.frozen).toEqual([candidates[1]]);
    expect(result.pending).toEqual([candidates[0]]);
    expect(result).not.toHaveProperty("published");
  });

  test("new entries append after the existing prefix even when their URL sorts earlier", () => {
    const { root } = createFixture();
    const candidates = deriveEdgeFormFreezeCandidates(root);
    commitInitialFreeze(root, [candidates[1]]);
    const newCandidate = candidates[0];
    write(
      root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([candidates[1], newCandidate]), null, 2)}\n`,
    );
    git(root, "add", FREEZE_PATH);
    git(root, "commit", "--quiet", "-m", "append another frozen Form URL");

    const result = verifyEdgeFormFreeze(root, { requireFrozen: true });
    expect(result.status).toBe("FROZEN");
    expect(result.frozen.map((entry) => entry.url)).toEqual([
      candidates[1].url,
      newCandidate.url,
    ]);
    expect(result.pending).toEqual([]);
  });

  test("accepts a first-add on the second parent of a no-ff merge and later appends", () => {
    const { root } = createFixture();
    const base = git(root, "rev-parse", "HEAD");
    const candidates = deriveEdgeFormFreezeCandidates(root);
    git(root, "switch", "-c", "freeze-feature");
    const firstAdd = commitInitialFreeze(root, [candidates[1]]);
    git(root, "switch", "-c", "mainline", base);
    write(root, "unrelated.md", "Mainline work.\n");
    git(root, "add", "unrelated.md");
    git(root, "commit", "--quiet", "-m", "unrelated mainline work");
    git(
      root,
      "merge",
      "--quiet",
      "--no-ff",
      "-m",
      "merge freeze feature",
      "freeze-feature",
    );

    const merged = verifyEdgeFormFreeze(root, { requireFrozen: true });
    expect(merged.status).toBe("FROZEN");
    expect(merged.firstAddCommit).toBe(firstAdd);

    write(
      root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([candidates[1], candidates[0]]), null, 2)}\n`,
    );
    git(root, "add", FREEZE_PATH);
    git(root, "commit", "--quiet", "-m", "append after merge");
    expect(verifyEdgeFormFreeze(root, { requireFrozen: true }).status).toBe(
      "FROZEN",
    );
  });

  test("rejects frozen source tampering on a branch even when merge restores it", () => {
    const { root } = createFixture();
    const [candidate] = deriveEdgeFormFreezeCandidates(root);
    commitInitialFreeze(root, [candidate]);
    git(root, "switch", "-c", "tamper");
    write(
      root,
      candidate.path,
      formSource("ModuleWorker", "0.3.0", "Branch tamper."),
    );
    git(root, "add", candidate.path);
    git(root, "commit", "--quiet", "-m", "tamper frozen source");
    git(root, "switch", "-");
    write(root, "unrelated.md", "Mainline work.\n");
    git(root, "add", "unrelated.md");
    git(root, "commit", "--quiet", "-m", "unrelated mainline work");
    git(root, "merge", "--quiet", "--no-ff", "--no-commit", "tamper");
    write(root, candidate.path, formSource("ModuleWorker", "0.3.0"));
    git(root, "add", candidate.path);
    git(root, "commit", "--quiet", "-m", "restore source in merge");

    const result = verifyEdgeFormFreeze(root);
    expect(result.status).toBe("INVALID");
    expect(result.problems.join("\n")).toContain("frozen Form source differs");
  });

  test("rejects frozen manifest tampering on a branch even when merge restores it", () => {
    const { root } = createFixture();
    const candidates = deriveEdgeFormFreezeCandidates(root);
    commitInitialFreeze(root, candidates);
    git(root, "switch", "-c", "tamper");
    write(
      root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([candidates[0]]), null, 2)}\n`,
    );
    git(root, "add", FREEZE_PATH);
    git(root, "commit", "--quiet", "-m", "drop frozen entry on branch");
    git(root, "switch", "-");
    write(root, "unrelated.md", "Mainline work.\n");
    git(root, "add", "unrelated.md");
    git(root, "commit", "--quiet", "-m", "unrelated mainline work");
    git(root, "merge", "--quiet", "--no-ff", "--no-commit", "tamper");
    write(
      root,
      FREEZE_PATH,
      `${JSON.stringify(manifest(candidates), null, 2)}\n`,
    );
    git(root, "add", FREEZE_PATH);
    git(root, "commit", "--quiet", "-m", "restore manifest in merge");

    const result = verifyEdgeFormFreeze(root);
    expect(result.status).toBe("INVALID");
    expect(result.problems.join("\n")).toContain("is not append-only");
  });

  test("rejects a branch deletion and re-addition hidden by the merge result", () => {
    const { root } = createFixture();
    const [candidate] = deriveEdgeFormFreezeCandidates(root);
    commitInitialFreeze(root, [candidate]);
    git(root, "switch", "-c", "delete-and-restore");
    git(root, "rm", "--quiet", FREEZE_PATH);
    git(root, "commit", "--quiet", "-m", "delete manifest on branch");
    write(
      root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([candidate]), null, 2)}\n`,
    );
    git(root, "add", FREEZE_PATH);
    git(root, "commit", "--quiet", "-m", "re-add manifest on branch");
    git(root, "switch", "-");
    write(root, "unrelated.md", "Mainline work.\n");
    git(root, "add", "unrelated.md");
    git(root, "commit", "--quiet", "-m", "unrelated mainline work");
    git(
      root,
      "merge",
      "--quiet",
      "--no-ff",
      "-m",
      "merge branch",
      "delete-and-restore",
    );

    const result = verifyEdgeFormFreeze(root);
    expect(result.status).toBe("INVALID");
    expect(result.problems.join("\n")).toContain(
      "deletion and re-addition are forbidden",
    );
  });

  test("rejects a merge that discards an append from one parent", () => {
    const { root } = createFixture();
    const candidates = deriveEdgeFormFreezeCandidates(root);
    commitInitialFreeze(root, [candidates[1]]);
    git(root, "switch", "-c", "append-branch");
    write(
      root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([candidates[1], candidates[0]]), null, 2)}\n`,
    );
    git(root, "add", FREEZE_PATH);
    git(root, "commit", "--quiet", "-m", "append URL on branch");
    git(root, "switch", "-");
    write(root, "unrelated.md", "Mainline work.\n");
    git(root, "add", "unrelated.md");
    git(root, "commit", "--quiet", "-m", "unrelated mainline work");
    git(root, "merge", "--quiet", "--no-ff", "--no-commit", "append-branch");
    write(
      root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([candidates[1]]), null, 2)}\n`,
    );
    git(root, "add", FREEZE_PATH);
    git(root, "commit", "--quiet", "-m", "discard append in merge");

    const result = verifyEdgeFormFreeze(root);
    expect(result.status).toBe("INVALID");
    expect(result.problems.join("\n")).toContain("is not append-only");
  });

  test("rejects a changed frozen Form source even when its manifest hash is updated", () => {
    const { root } = createFixture();
    const candidates = deriveEdgeFormFreezeCandidates(root);
    commitInitialFreeze(root, [candidates[1]]);
    const changed = formSource(
      "WorkerVersion",
      "0.5.0",
      "Rewritten semantics.",
    );
    write(root, candidates[1].path, changed);
    const updated = {
      ...candidates[1],
      sha256: `sha256:${digestHex(changed)}`,
    };
    write(
      root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([updated]), null, 2)}\n`,
    );
    git(root, "add", FREEZE_PATH, candidates[1].path);
    git(root, "commit", "--quiet", "-m", "attempt to rewrite frozen Form");

    const result = verifyEdgeFormFreeze(root);
    expect(result.status).toBe("INVALID");
    expect(result.problems.join("\n")).toContain(
      `${FREEZE_PATH} is not append-only`,
    );
  });

  test("rejects removal of an old entry and its manifest deletion/re-addition", () => {
    const removed = createFixture();
    const candidates = deriveEdgeFormFreezeCandidates(removed.root);
    commitInitialFreeze(removed.root, candidates);
    write(
      removed.root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([candidates[0]]), null, 2)}\n`,
    );
    git(removed.root, "add", FREEZE_PATH);
    git(removed.root, "commit", "--quiet", "-m", "remove a frozen URL");
    expect(verifyEdgeFormFreeze(removed.root).problems.join("\n")).toContain(
      "is not append-only",
    );

    const readded = createFixture();
    const readdedCandidates = deriveEdgeFormFreezeCandidates(readded.root);
    commitInitialFreeze(readded.root, readdedCandidates);
    git(readded.root, "rm", "--quiet", FREEZE_PATH);
    git(readded.root, "commit", "--quiet", "-m", "delete freeze manifest");
    write(
      readded.root,
      FREEZE_PATH,
      `${JSON.stringify(manifest(readdedCandidates), null, 2)}\n`,
    );
    git(readded.root, "add", FREEZE_PATH);
    git(readded.root, "commit", "--quiet", "-m", "re-add freeze manifest");
    expect(verifyEdgeFormFreeze(readded.root).problems.join("\n")).toContain(
      "deletion and re-addition are forbidden",
    );
  });

  test("refuses replacement objects, grafts, and shallow history", () => {
    const replacement = createFixture();
    const candidates = deriveEdgeFormFreezeCandidates(replacement.root);
    const firstAdd = commitInitialFreeze(replacement.root, [candidates[1]]);
    const modified = formSource(
      "WorkerVersion",
      "0.5.0",
      "Replacement history.",
    );
    write(replacement.root, candidates[1].path, modified);
    const changedEntry = {
      ...candidates[1],
      sha256: `sha256:${digestHex(modified)}`,
    };
    write(
      replacement.root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([changedEntry]), null, 2)}\n`,
    );
    git(replacement.root, "add", FREEZE_PATH, candidates[1].path);
    const tree = git(replacement.root, "write-tree");
    const alternate = git(
      replacement.root,
      "commit-tree",
      tree,
      "-m",
      "parentless replacement tree",
    );
    git(replacement.root, "replace", firstAdd, alternate);
    write(
      replacement.root,
      candidates[1].path,
      formSource("WorkerVersion", "0.5.0"),
    );
    write(
      replacement.root,
      FREEZE_PATH,
      `${JSON.stringify(manifest([candidates[1]]), null, 2)}\n`,
    );
    expect(verifyEdgeFormFreeze(replacement.root).status).toBe("FROZEN");

    const grafted = createFixture();
    const graftCandidates = deriveEdgeFormFreezeCandidates(grafted.root);
    commitInitialFreeze(grafted.root, graftCandidates);
    write(grafted.root, ".git/info/grafts", "# legacy history override\n");
    expect(verifyEdgeFormFreeze(grafted.root).problems.join("\n")).toContain(
      "legacy Git graft file exists",
    );

    const shallow = createFixture();
    const shallowCandidates = deriveEdgeFormFreezeCandidates(shallow.root);
    commitInitialFreeze(shallow.root, shallowCandidates);
    write(
      shallow.root,
      ".git/shallow",
      `${git(shallow.root, "rev-parse", "HEAD")}\n`,
    );
    expect(verifyEdgeFormFreeze(shallow.root).problems.join("\n")).toContain(
      "shallow Git history",
    );
  });

  test("rejects contradictory requireFrozen bootstrap options and malformed null manifests", () => {
    const { root } = createFixture();
    expect(() =>
      verifyEdgeFormFreeze(root, { mode: "bootstrap", requireFrozen: true }),
    ).toThrow("requireFrozen cannot be used with bootstrap mode");
    write(root, FREEZE_PATH, "null\n");
    expect(verifyEdgeFormFreeze(root, { mode: "bootstrap" }).status).toBe(
      "INVALID",
    );
  });

  test("requires byte-for-byte canonical frozen URLs", () => {
    const { root } = createFixture();
    const [candidate] = deriveEdgeFormFreezeCandidates(root);
    const nonCanonicalUrls = [
      `${candidate.url}?`,
      `${candidate.url}#`,
      candidate.url.replace("https://", "https://user@"),
      candidate.url.replace(".com/", ".com:443/"),
      candidate.url.replace("https://", "https://EDGE.FORMS."),
      candidate.url.replace("/forms/", "/forms/./"),
    ];
    for (const url of nonCanonicalUrls) {
      write(
        root,
        FREEZE_PATH,
        `${JSON.stringify(manifest([{ ...candidate, url }]), null, 2)}\n`,
      );
      expect(verifyEdgeFormFreeze(root, { mode: "bootstrap" }).status).toBe(
        "INVALID",
      );
    }
  });
});

function digestHex(value) {
  return createHash("sha256").update(value).digest("hex");
}
