import { describe, expect, test } from "bun:test";
import { Database } from "bun:sqlite";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

import { isAllowedMigrationPragma } from "../conformance/host-v2/sqlite-migration/allowed-pragma.mjs";

const root = new URL("../", import.meta.url);
const corpus = JSON.parse(
  readFileSync(
    new URL("conformance/host-v2/sqlite-migration/cases.json", root),
    "utf8",
  ),
);

function deferredForeignKeys(database) {
  return database.query("PRAGMA defer_foreign_keys").get().defer_foreign_keys;
}

function fixtureDatabase() {
  const database = new Database(":memory:");
  database.exec("PRAGMA foreign_keys = ON");
  database.exec(`
    CREATE TABLE parent (id INTEGER PRIMARY KEY);
    CREATE TABLE child (
      id INTEGER PRIMARY KEY,
      parent_id INTEGER REFERENCES parent(id) NOT DEFERRABLE INITIALLY IMMEDIATE
    );
    CREATE TABLE migration_history (path TEXT PRIMARY KEY);
  `);
  return database;
}

describe("SQLiteMigrationApplication PRAGMA source conformance", () => {
  test("corpus Form identities resolve to exact frozen source bytes", () => {
    const freeze = JSON.parse(
      readFileSync(new URL("spec/forms.freeze.json", root), "utf8"),
    );
    for (const url of [corpus.form, corpus.migrationSetForm]) {
      const anchor = freeze.forms.find((entry) => entry.url === url);
      expect(anchor).toBeDefined();
      const source = readFileSync(new URL(anchor.path, root));
      expect(anchor.sha256).toBe(
        `sha256:${createHash("sha256").update(source).digest("hex")}`,
      );
      expect(source.toString("utf8")).toContain(`formUrl: ${url}`);
    }
    expect(new Set(corpus.cases.map((entry) => entry.id)).size).toBe(
      corpus.cases.length,
    );
  });

  for (const entry of corpus.cases) {
    test(`${entry.allowed ? "accepts" : "rejects"} ${entry.id}`, () => {
      expect(isAllowedMigrationPragma(entry.sql)).toBe(entry.allowed);
      if (!entry.allowed) return;

      const database = fixtureDatabase();
      try {
        database.exec("BEGIN");
        database.exec(
          `PRAGMA defer_foreign_keys = ${entry.value === 1 ? "OFF" : "ON"}`,
        );
        database.exec(entry.sql);
        expect(deferredForeignKeys(database)).toBe(entry.value);
        expect(database.query("PRAGMA foreign_keys").get().foreign_keys).toBe(
          1,
        );
        database.exec("ROLLBACK");
        expect(deferredForeignKeys(database)).toBe(0);
      } finally {
        database.close();
      }
    });
  }

  test("does not coerce non-string inputs into SQL", () => {
    for (const input of [
      null,
      undefined,
      1,
      {},
      ["PRAGMA defer_foreign_keys = ON"],
    ])
      expect(isAllowedMigrationPragma(input)).toBe(false);
  });

  test("defers an immediate foreign key until commit and resets for the next file", () => {
    const database = fixtureDatabase();
    try {
      database.exec("BEGIN");
      database.exec("PRAGMA defer_foreign_keys = TRUE;");
      database.exec("INSERT INTO child VALUES (1, 10)");
      database.exec("INSERT INTO parent VALUES (10)");
      database.exec("INSERT INTO migration_history VALUES ('001-create.sql')");
      database.exec("COMMIT");
      expect(deferredForeignKeys(database)).toBe(0);
      expect(database.query("SELECT * FROM child").all()).toEqual([
        { id: 1, parent_id: 10 },
      ]);
      database.exec("BEGIN");
      expect(() => database.exec("INSERT INTO child VALUES (2, 20)")).toThrow(
        /FOREIGN KEY constraint failed/,
      );
      database.exec("ROLLBACK");
      expect(
        database.query("SELECT path FROM migration_history").all(),
      ).toEqual([{ path: "001-create.sql" }]);
    } finally {
      database.close();
    }
  });

  test("rolls back SQL and history together when a deferred foreign key fails at commit", () => {
    const database = fixtureDatabase();
    try {
      database.exec("BEGIN");
      database.exec("PRAGMA defer_foreign_keys = 1");
      database.exec("CREATE TABLE migration_effect (id INTEGER PRIMARY KEY)");
      database.exec("INSERT INTO child VALUES (1, 99)");
      database.exec("INSERT INTO migration_history VALUES ('002-invalid.sql')");
      expect(() => database.exec("COMMIT")).toThrow(
        /FOREIGN KEY constraint failed/,
      );
      database.exec("ROLLBACK");
      expect(deferredForeignKeys(database)).toBe(0);
      expect(database.query("SELECT * FROM child").all()).toEqual([]);
      expect(database.query("SELECT * FROM migration_history").all()).toEqual(
        [],
      );
      expect(
        database
          .query(
            "SELECT name FROM sqlite_schema WHERE name = 'migration_effect'",
          )
          .all(),
      ).toEqual([]);
    } finally {
      database.close();
    }
  });
});
