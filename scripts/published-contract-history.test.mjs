import { describe, expect, test } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

import {
  assertPublishedDefinitionsUnchanged,
  verifyPublishedBaseline,
} from "./published-contract-history.mjs";

function definition(kind, version, description = "published meaning") {
  const contract = ["InterfaceDefinition", "BindingDefinition"].includes(kind);
  return Buffer.from(
    JSON.stringify({
      apiVersion: contract
        ? `${kind.toLowerCase()}.example/v1`
        : "example.forms.test",
      kind,
      ...(contract
        ? { name: "example.actor", version }
        : { definitionVersion: version }),
      description,
    }),
  );
}

describe("published contract identity preservation", () => {
  test("preserves each published kind and allows distinct forward versions", () => {
    for (const kind of [
      "ActorNamespace",
      "InterfaceDefinition",
      "BindingDefinition",
    ]) {
      const previous = new Map([
        ["old/definition.json", definition(kind, "1.0.0")],
      ]);
      expect(() =>
        assertPublishedDefinitionsUnchanged(
          previous,
          new Map([["current/definition.json", definition(kind, "1.0.0")]]),
        ),
      ).not.toThrow();
      expect(() =>
        assertPublishedDefinitionsUnchanged(
          previous,
          new Map([
            [
              "current/definition.json",
              definition(kind, "1.1.0", "new meaning"),
            ],
          ]),
        ),
      ).not.toThrow();
      expect(() =>
        assertPublishedDefinitionsUnchanged(
          previous,
          new Map([
            [
              "renamed/definition.json",
              definition(kind, "1.0.0", "new meaning"),
            ],
          ]),
        ),
      ).toThrow(/published identity has different bytes/);
    }
  });

  test("a retained Form version cannot be reused with another meaning", () => {
    const published = new Map([
      ["current/definition.json", definition("WorkerVersion", "0.3.0")],
      [
        "retained/definition.json",
        definition("WorkerVersion", "0.2.0", "retained meaning"),
      ],
    ]);
    expect(() =>
      assertPublishedDefinitionsUnchanged(
        published,
        new Map([
          [
            "candidate/definition.json",
            definition("WorkerVersion", "0.2.0", "replacement meaning"),
          ],
        ]),
      ),
    ).toThrow(/published identity has different bytes/);
    expect(() =>
      assertPublishedDefinitionsUnchanged(
        published,
        new Map([
          [
            "candidate/definition.json",
            definition("WorkerVersion", "0.2.0", "retained meaning"),
          ],
        ]),
      ),
    ).not.toThrow();
  });

  test("rejects contradictory published identities and malformed current identities", () => {
    expect(() =>
      assertPublishedDefinitionsUnchanged(
        new Map([
          ["first", definition("InterfaceDefinition", "1.0.0")],
          [
            "second",
            definition("InterfaceDefinition", "1.0.0", "other meaning"),
          ],
        ]),
        new Map(),
      ),
    ).toThrow(/snapshot reuses an exact identity/);
    expect(() =>
      assertPublishedDefinitionsUnchanged(
        new Map(),
        new Map([["candidate", definition("ActorNamespace", "latest")]]),
      ),
    ).toThrow(/invalid contract identity/);
  });

  test("fails when the exact local Git history is unavailable", () => {
    const root = mkdtempSync(path.join(tmpdir(), "missing-contract-history-"));
    try {
      expect(() => verifyPublishedBaseline(root)).toThrow(
        /unavailable locally/,
      );
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });
});
