#!/usr/bin/env node

// Verify the extracted data without consulting the predecessor checkout. The
// baseline is a checked-in byte map; generation and Go validators provide the
// semantic checks layered on top of it.
import { createHash } from "node:crypto";
import { existsSync, lstatSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const baselinePath = path.join(root, "integrity", "source-baseline.json");
const baseline = JSON.parse(readFileSync(baselinePath, "utf8"));
const failures = [];

if (baseline.format !== "takoform-forms.source-baseline@v1") {
  failures.push(
    `unexpected baseline format ${JSON.stringify(baseline.format)}`,
  );
}
if (baseline.source?.commit !== "220d37b284d8288e6e12d31375ecfdca6a5f15c5") {
  failures.push("baseline is not bound to the Provider tombstone commit");
}

function digest(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}

function isSafeRelative(value) {
  return (
    typeof value === "string" &&
    value.length > 0 &&
    !path.isAbsolute(value) &&
    !value.includes("\\") &&
    value
      .split("/")
      .every(
        (segment) => segment.length > 0 && segment !== "." && segment !== "..",
      )
  );
}

function pathUnderRoot(relative, label) {
  if (!isSafeRelative(relative)) {
    failures.push(`${label}: unsafe relative path ${JSON.stringify(relative)}`);
    return null;
  }
  return path.join(root, relative);
}

const expectedPaths = Object.keys(baseline.files ?? {}).sort();
for (const relative of expectedPaths) {
  const expected = baseline.files[relative];
  const absolute = pathUnderRoot(relative, "baseline path");
  if (!absolute) continue;
  if (!existsSync(absolute)) {
    failures.push(`${relative}: baseline file is missing`);
    continue;
  }
  const info = lstatSync(absolute);
  if (!info.isFile()) {
    failures.push(`${relative}: baseline path is not a regular file`);
    continue;
  }
  const bytes = readFileSync(absolute);
  if (bytes.length !== expected.bytes || digest(bytes) !== expected.sha256) {
    failures.push(`${relative}: source-baseline byte digest drift`);
  }
}

function walk(directory, relative = "") {
  const paths = [];
  for (const name of readdirSync(directory).sort()) {
    const child = path.join(directory, name);
    const childRelative = relative ? `${relative}/${name}` : name;
    const info = lstatSync(child);
    if (info.isDirectory()) paths.push(...walk(child, childRelative));
    else if (info.isFile() && name.endsWith(".json")) paths.push(childRelative);
    else if (info.isFile()) {
      failures.push(
        `${childRelative}: unexpected non-schema file in normative root`,
      );
    } else if (!info.isFile())
      failures.push(`${childRelative}: unsupported filesystem entry`);
  }
  return paths;
}

const trackedRoots = [
  "forms/candidates",
  "interfaces/candidates",
  "bindings/candidates",
  "conformance/takoform-v1/families",
];
const actualPaths = trackedRoots
  .flatMap((relative) => walk(path.join(root, relative), relative))
  .sort();
if (actualPaths.join("\n") !== expectedPaths.join("\n")) {
  const expected = new Set(expectedPaths);
  const actual = new Set(actualPaths);
  for (const relative of expectedPaths)
    if (!actual.has(relative))
      failures.push(`${relative}: baseline path is not present`);
  for (const relative of actualPaths)
    if (!expected.has(relative))
      failures.push(`${relative}: unlisted normative file`);
}

const familyIndexPath = path.join(
  root,
  "forms/candidates/current-family-index.json",
);
const familyIndex = JSON.parse(readFileSync(familyIndexPath, "utf8"));
if (familyIndex.format !== "takoform.current-family-index@v1")
  failures.push("current-family index format drift");
if (
  !Array.isArray(familyIndex.families) ||
  familyIndex.families.length !== baseline.counts.families
) {
  failures.push(
    `family count drift: got ${familyIndex.families?.length}, want ${baseline.counts.families}`,
  );
}
let formCount = 0;
const familyGroups = new Set();
for (const family of familyIndex.families ?? []) {
  if (familyGroups.has(family.group))
    failures.push(`duplicate family ${family.group}`);
  familyGroups.add(family.group);
  formCount += family.formCount ?? 0;
  if (!/^\S+\.forms\.takoform\.com$/u.test(family.group))
    failures.push(`unsafe current family ${family.group}`);
  const expectedCandidatePath = `forms/candidates/${family.group}/candidate-set.json`;
  const candidateRelative = family.candidateSet;
  if (candidateRelative !== expectedCandidatePath) {
    failures.push(
      `${family.group}: candidate-set path is not the current family path`,
    );
  }
  const candidatePath = pathUnderRoot(
    candidateRelative,
    `${family.group} candidate-set`,
  );
  if (!candidatePath || !candidateRelative.startsWith("forms/candidates/")) {
    failures.push(
      `${family.group}: candidate-set path escapes the candidate root`,
    );
    continue;
  }
  if (!existsSync(candidatePath)) {
    failures.push(`${family.group}: candidate set is missing`);
    continue;
  }
  const candidateBytes = readFileSync(candidatePath);
  if (digest(candidateBytes) !== family.sha256)
    failures.push(`${family.group}: candidate-set digest does not match index`);
  const candidate = JSON.parse(candidateBytes);
  if (
    candidate.family !== family.group ||
    !Array.isArray(candidate.forms) ||
    candidate.forms.length !== family.formCount
  ) {
    failures.push(`${family.group}: candidate-set form count or group drift`);
  }
  for (const entry of candidate.forms ?? []) {
    const packageRelative = entry.path;
    const packageRoot = pathUnderRoot(
      packageRelative,
      `${family.group}/${entry.kind} package`,
    );
    if (
      !packageRoot ||
      !packageRelative.startsWith(`forms/candidates/${family.group}/`)
    ) {
      failures.push(
        `${family.group}/${entry.kind}: package path escapes the family candidate root`,
      );
      continue;
    }
    const definitionPath = path.join(packageRoot, "definition.json");
    const indexPath = path.join(packageRoot, "package-index.json");
    if (!existsSync(definitionPath) || !existsSync(indexPath)) {
      failures.push(
        `${family.group}/${entry.kind}: package files are incomplete`,
      );
      continue;
    }
    const definition = JSON.parse(readFileSync(definitionPath, "utf8"));
    const packageIndex = JSON.parse(readFileSync(indexPath, "utf8"));
    if (
      definition.apiVersion !== family.group ||
      definition.kind !== entry.kind ||
      definition.role !== entry.role
    ) {
      failures.push(`${family.group}/${entry.kind}: candidate identity drift`);
    }
    if (
      packageIndex.formRef?.schemaDigest !== entry.formRef?.schemaDigest ||
      packageIndex.formRef?.kind !== entry.kind
    ) {
      failures.push(`${family.group}/${entry.kind}: package FormRef drift`);
    }
  }
}
if (formCount !== baseline.counts.forms)
  failures.push(
    `Form count drift: got ${formCount}, want ${baseline.counts.forms}`,
  );
if (familyGroups.has("edge.forms.takoform.com/v1beta1"))
  failures.push("retained edge/v1beta1 family is not a current candidate");

const interfaceSet = JSON.parse(
  readFileSync(
    path.join(root, "interfaces/candidates/v1alpha1/candidate-set.json"),
    "utf8",
  ),
);
const bindingSet = JSON.parse(
  readFileSync(
    path.join(root, "bindings/candidates/v1alpha2/candidate-set.json"),
    "utf8",
  ),
);
if (interfaceSet.interfaces?.length !== baseline.counts.interfaces)
  failures.push("Interface candidate count drift");
if (bindingSet.bindings?.length !== baseline.counts.bindings)
  failures.push("Binding candidate count drift");
const interfaceCandidatePath = pathUnderRoot(
  familyIndex.interfaceCandidateSet?.path,
  "Interface candidate-set",
);
const bindingCandidatePath = pathUnderRoot(
  familyIndex.bindingCandidateSet?.path,
  "Binding candidate-set",
);
if (
  familyIndex.interfaceCandidateSet?.path !==
  "interfaces/candidates/v1alpha1/candidate-set.json"
) {
  failures.push("Interface candidate-set path drift");
}
if (
  familyIndex.bindingCandidateSet?.path !==
  "bindings/candidates/v1alpha2/candidate-set.json"
) {
  failures.push("Binding candidate-set path drift");
}
if (
  !interfaceCandidatePath ||
  !interfaceCandidatePath
    .toString()
    .includes(`${path.sep}interfaces${path.sep}candidates${path.sep}`)
) {
  failures.push("Interface candidate-set path escapes the interface root");
} else if (
  familyIndex.interfaceCandidateSet?.sha256 !==
  digest(readFileSync(interfaceCandidatePath))
) {
  failures.push(
    "Interface candidate-set digest does not match current-family index",
  );
}
if (
  !bindingCandidatePath ||
  !bindingCandidatePath
    .toString()
    .includes(`${path.sep}bindings${path.sep}candidates${path.sep}`)
) {
  failures.push("Binding candidate-set path escapes the binding root");
} else if (
  familyIndex.bindingCandidateSet?.sha256 !==
  digest(readFileSync(bindingCandidatePath))
) {
  failures.push(
    "Binding candidate-set digest does not match current-family index",
  );
}

if (failures.length)
  throw new Error(`source integrity check failed:\n${failures.join("\n")}`);
console.log(
  `source baseline matches ${expectedPaths.length} canonical files; ${formCount} Forms, ${interfaceSet.interfaces.length} Interfaces, ${bindingSet.bindings.length} Bindings`,
);
