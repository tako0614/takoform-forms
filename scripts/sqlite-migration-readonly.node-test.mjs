import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import {
  checkMigration,
  fixtureDatabase,
} from "../conformance/host-v2/sqlite-migration/v0.4.0/reference-check.mjs";

const root = new URL("../", import.meta.url);
const base = new URL("conformance/host-v2/sqlite-migration/v0.4.0/", root);
const corpus = JSON.parse(readFileSync(new URL("cases.json", base), "utf8"));
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");

test("0.4 source identities resolve to exact frozen bytes", () => {
  const freeze = JSON.parse(
    readFileSync(new URL("spec/forms.freeze.json", root)),
  );
  for (const url of [corpus.form, corpus.migrationSetForm]) {
    const entry = freeze.forms.find((entry) => entry.url === url);
    assert.ok(entry);
    assert.equal(
      entry.sha256,
      `sha256:${digest(readFileSync(new URL(entry.path, root)))}`,
    );
  }
  assert.equal(
    new Set(corpus.cases.map((entry) => entry.id)).size,
    corpus.cases.length,
  );
});

for (const entry of corpus.cases) {
  test(`${entry.allowed ? "accepts" : "rejects"} ${entry.id}`, () => {
    const database = fixtureDatabase(
      corpus.setupSql + (entry.setupSql ? ";" + entry.setupSql : ""),
    );
    try {
      if (entry.allowed) {
        const { results } = checkMigration(database, entry.sql);
        if (entry.expectedRows)
          assert.deepEqual(
            JSON.parse(JSON.stringify(results[0])),
            entry.expectedRows,
          );
      } else assert.throws(() => checkMigration(database, entry.sql));
      assert.equal(
        database.prepare("PRAGMA foreign_keys").get().foreign_keys,
        1,
      );
      assert.equal(
        database.prepare("PRAGMA defer_foreign_keys").get().defer_foreign_keys,
        0,
      );
      assert.equal(
        database.prepare("SELECT count(*) AS n FROM migration_history").get().n,
        0,
      );
    } finally {
      database.close();
    }
  });
}

test("forbidden neighbor rolls back prior file effects", () => {
  const database = fixtureDatabase(corpus.setupSql);
  try {
    assert.throws(() =>
      checkMigration(
        database,
        "INSERT INTO evidence VALUES ('rollback'); PRAGMA foreign_keys=OFF;",
      ),
    );
    assert.equal(
      database.prepare("SELECT count(*) AS n FROM evidence").get().n,
      0,
    );
  } finally {
    database.close();
  }
});

test("schema write protection survives privileged writable_schema setup", () => {
  for (const entry of corpus.cases.filter((entry) =>
    /^schema-(?:insert|update|delete)$/u.test(entry.id),
  )) {
    const database = fixtureDatabase(corpus.setupSql);
    try {
      database.exec("PRAGMA writable_schema=ON");
      assert.throws(() => checkMigration(database, entry.sql));
      assert.equal(
        database
          .prepare("SELECT count(*) AS n FROM sqlite_schema WHERE name='child'")
          .get().n,
        1,
      );
    } finally {
      database.close();
    }
  }
});

test("implicit introspection cannot escape through a temp shadow", () => {
  const database = fixtureDatabase(corpus.setupSql);
  try {
    database.exec("CREATE TEMP TABLE child(secret TEXT)");
    assert.throws(() => checkMigration(database, "PRAGMA table_info(child)"));
    assert.throws(() =>
      checkMigration(database, "SELECT * FROM pragma_table_info('child')"),
    );
    checkMigration(database, "PRAGMA main.table_info(child)");
  } finally {
    database.close();
  }
});

const manifest = JSON.parse(
  readFileSync(new URL("fixtures/manifest.json", base), "utf8"),
);
const fixtureRoot = new URL("fixtures/", base);
function fixture(name) {
  assert.match(name, /^[a-zA-Z0-9_-]+\.sql$/u);
  const bytes = readFileSync(new URL(name, fixtureRoot));
  const entry = manifest.files.find((file) => file.path === name);
  assert.ok(entry, `fixture declared: ${name}`);
  assert.equal(entry.sha256, `sha256:${digest(bytes)}`);
  return bytes.toString("utf8");
}

test("shared fixtures resolve every recorded byte digest", () => {
  for (const entry of manifest.files) fixture(entry.path);
});

for (const migration of manifest.migrations) {
  test(`validates unchanged Yurucommu ${migration.id} with SQLite parser and authorizer`, () => {
    assert.equal(migration.expected, "accept");
    const database = fixtureDatabase();
    for (const name of migration.setup) {
      database.exec("BEGIN");
      database.exec(fixture(name));
      database.exec("COMMIT");
    }
    try {
      const { effects, results } = checkMigration(
        database,
        fixture(migration.file),
      );
      assert.ok(results.length > 1);
      if (migration.id.includes("0036")) {
        assert.ok(effects.some((effect) => effect.first === "table_info"));
        assert.ok(
          effects.some((effect) => effect.first === "foreign_key_list"),
        );
        assert.ok(effects.some((effect) => effect.first === "sqlite_master"));
        assert.equal(
          database
            .prepare(
              "SELECT count(*) AS n FROM pragma_foreign_key_list('notification_pushers')",
            )
            .get().n,
          0,
        );
      } else {
        assert.ok(
          effects.some((effect) => effect.first === "defer_foreign_keys"),
        );
      }
      assert.equal(
        database.prepare("PRAGMA foreign_keys").get().foreign_keys,
        1,
      );
      assert.equal(
        database.prepare("PRAGMA defer_foreign_keys").get().defer_foreign_keys,
        0,
      );
      assert.equal(
        database.prepare("SELECT count(*) AS n FROM migration_history").get().n,
        0,
      );
    } finally {
      database.close();
    }
  });
}

test("each introspection API returns its native metadata without changing state", () => {
  const database = fixtureDatabase(corpus.setupSql);
  try {
    database.exec(
      "INSERT INTO parent VALUES(1); INSERT INTO child VALUES(1,1)",
    );
    const schema = database
      .prepare("SELECT * FROM sqlite_schema ORDER BY name")
      .all();
    for (const [name, object] of [
      ["table_info", "child"],
      ["foreign_key_list", "child"],
      ["index_list", "child"],
      ["index_info", "child_parent_idx"],
    ]) {
      const expected = database.prepare(`PRAGMA ${name}('${object}')`).all();
      assert.ok(expected.length > 0);
      const { results } = checkMigration(
        database,
        `PRAGMA ${name}('${object}'); SELECT * FROM pragma_${name}('${object}')`,
      );
      assert.deepEqual(results, [expected, expected]);
    }
    assert.deepEqual(
      database.prepare("SELECT * FROM sqlite_schema ORDER BY name").all(),
      schema,
    );
    assert.deepEqual(
      { ...database.prepare("SELECT * FROM child").get() },
      { id: 1, parent_id: 1 },
    );
  } finally {
    database.close();
  }
});
