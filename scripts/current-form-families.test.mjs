import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";

import { validatePublisherPathMetadata } from "./current-form-families.mjs";

const generatorSource = readFileSync(
  new URL("./current-form-families.mjs", import.meta.url),
  "utf8",
);

function validPublisherSource() {
  return {
    families: [
      {
        group: "example.forms.takoform.com",
        forms: [
          {
            kind: "Example",
            slug: "example",
            role: "identity",
            fixtures: { "desired.json": {}, "negative-example.json": {} },
          },
        ],
      },
    ],
    interfaces: [{ name: "example.runtime", version: "1.0.0" }],
    bindings: [{ name: "example.binding", version: "1.0.0" }],
  };
}

describe("publisher composition ownership", () => {
  test("does not carry an independent family, interface, or binding roster", () => {
    expect(generatorSource).not.toMatch(/\bconst\s+familySpecs\b/);
    expect(generatorSource).not.toMatch(/\bconst\s+interfaceContracts\b/);
    expect(generatorSource).not.toMatch(/\bconst\s+bindingContracts\b/);
  });

  test("rejects every publisher-derived filesystem traversal component", () => {
    const valid = validPublisherSource;
    for (const mutate of [
      (source) => {
        source.families[0].group = "../outside.forms.takoform.com";
      },
      (source) => {
        source.families[0].forms[0].slug = "../../outside";
      },
      (source) => {
        source.families[0].forms[0].fixtures = { "../outside.json": {} };
      },
      (source) => {
        source.interfaces[0].name = "../outside";
      },
      (source) => {
        source.bindings[0].name = "outside/name";
      },
    ]) {
      const source = valid();
      mutate(source);
      expect(() => validatePublisherPathMetadata(source)).toThrow();
    }
    expect(() => validatePublisherPathMetadata(valid())).not.toThrow();
  });

  test("rejects duplicate names within each contract namespace", () => {
    for (const mutate of [
      (source) =>
        source.interfaces.push({ name: "example.runtime", version: "1.0.0" }),
      (source) =>
        source.interfaces.push({ name: "example.runtime", version: "1.1.0" }),
      (source) =>
        source.bindings.push({ name: "example.binding", version: "1.0.0" }),
      (source) =>
        source.bindings.push({ name: "example.binding", version: "1.1.0" }),
    ]) {
      const source = validPublisherSource();
      mutate(source);
      expect(() => validatePublisherPathMetadata(source)).toThrow(
        "duplicate contract name",
      );
    }

    const sharedName = validPublisherSource();
    sharedName.interfaces[0].name = "shared.contract";
    sharedName.bindings[0].name = "shared.contract";
    expect(() => validatePublisherPathMetadata(sharedName)).not.toThrow();
  });
});
