// Test-only source conformance: SQLite parses statement boundaries and reports
// effects. This is not a Host adapter, artifact loader, or production sandbox.
import { DatabaseSync, constants as sqlite } from "node:sqlite";

const metadata = new Set([
  "table_info",
  "foreign_key_list",
  "index_list",
  "index_info",
]);
const catalogs = new Set(["sqlite_master", "sqlite_schema"]);
const deferPrefix =
  /^PRAGMA[\t\n\v\f\r ]+defer_foreign_keys[\t\n\v\f\r ]*=[\t\n\v\f\r ]*(?:ON|OFF|TRUE|FALSE|1|0)[\t\n\v\f\r ]*(?:;|$)/i;
const control = new Set([
  sqlite.SQLITE_TRANSACTION,
  sqlite.SQLITE_SAVEPOINT,
  sqlite.SQLITE_ATTACH,
  sqlite.SQLITE_DETACH,
]);
const writes = new Set([
  sqlite.SQLITE_INSERT,
  sqlite.SQLITE_UPDATE,
  sqlite.SQLITE_DELETE,
]);

// Skip only inter-statement trivia. SQLite, not this helper, finds the end of
// each real statement, including trigger bodies and semicolons inside strings.
function nextSql(sql) {
  for (;;) {
    sql = sql.replace(/^[\t\n\v\f\r ;]+/u, "");
    if (sql.startsWith("--")) {
      const end = sql.indexOf("\n");
      if (end < 0) return "";
      sql = sql.slice(end + 1);
    } else if (sql.startsWith("/*")) {
      const end = sql.indexOf("*/", 2);
      if (end < 0) throw new Error("unterminated SQL comment");
      sql = sql.slice(end + 2);
    } else return sql;
  }
}

export function fixtureDatabase(setup = "") {
  const database = new DatabaseSync(":memory:");
  database.exec("PRAGMA foreign_keys = ON");
  database.exec(setup);
  database.exec("CREATE TABLE migration_history (path TEXT PRIMARY KEY)");
  return database;
}

export function checkMigration(database, sql) {
  if (typeof sql !== "string" || sql.includes("\0"))
    throw new Error("migration must be NUL-free SQL text");
  const effects = [];
  const results = [];
  let deniedEffect;
  let remaining = nextSql(sql);
  database.exec("BEGIN");
  try {
    while (remaining) {
      const current = remaining;
      // These trusted queries run outside the authorizer callback. SQLite's
      // implicit metadata lookup searches temp/attached schemas as well as main.
      const shadowNames = new Set();
      for (const { name } of database.prepare("PRAGMA database_list").all()) {
        if (name === "main") continue;
        const quoted = '"' + name.replaceAll('"', '""') + '"';
        for (const row of database
          .prepare(`SELECT name FROM ${quoted}.sqlite_schema`)
          .all())
          shadowNames.add(row.name.toLowerCase());
      }
      const protectedObjects = new Set(
        database
          .prepare(
            "SELECT name FROM main.sqlite_schema WHERE tbl_name='migration_history' COLLATE NOCASE",
          )
          .all()
          .map((row) => row.name.toLowerCase()),
      );
      protectedObjects.add("migration_history");
      const userObjects = new Set(
        database
          .prepare(
            "SELECT name FROM main.sqlite_schema WHERE type IN ('table','view')",
          )
          .all()
          .map((row) => row.name.toLowerCase()),
      );
      const ddl = /^(?:CREATE|ALTER|DROP|REINDEX)\b/iu.test(current);
      database.setAuthorizer((action, first, second, dbName, origin) => {
        const a = first?.toLowerCase();
        const b = second?.toLowerCase();
        effects.push({ action, first, second, dbName, origin });
        const deny = () => {
          deniedEffect = { action, first, second, dbName, origin };
          return sqlite.SQLITE_DENY;
        };
        if (control.has(action)) return deny();
        // SQLITE_ALTER_TABLE reports its target schema in arg1, not dbName.
        if (action === sqlite.SQLITE_ALTER_TABLE && a !== "main") return deny();
        // ALTER's built-in schema rewrite checks/rewrites temp declarations as
        // well. This engine metadata read does not authorize user temp reads.
        const alterTempCatalog =
          [sqlite.SQLITE_READ, sqlite.SQLITE_UPDATE].includes(action) &&
          dbName === "temp" &&
          a === "sqlite_temp_master" &&
          /^ALTER\b/iu.test(current);
        if (dbName && dbName !== "main" && !alterTempCatalog) return deny();
        if (
          protectedObjects.has(a) ||
          (action === sqlite.SQLITE_ALTER_TABLE && b === "migration_history")
        )
          return deny();
        if (
          action === sqlite.SQLITE_FUNCTION &&
          [
            "load_extension",
            "readfile",
            "writefile",
            "eval",
            "fts3_tokenizer",
          ].includes(b)
        )
          return deny();
        if (
          action === sqlite.SQLITE_READ &&
          a?.startsWith("pragma_") &&
          !userObjects.has(a) &&
          !metadata.has(a.slice(7))
        )
          return deny();
        if (writes.has(action) && catalogs.has(a) && !ddl) return deny();
        if (action === sqlite.SQLITE_PRAGMA) {
          if (metadata.has(a)) {
            if (protectedObjects.has(b) || (!dbName && shadowNames.has(b)))
              return deny();
            return sqlite.SQLITE_OK;
          }
          // Match raw source because SQLite normalizes quotes/schema/value
          // spellings in the callback. The legacy exception stays narrow.
          if (
            a === "defer_foreign_keys" &&
            second !== null &&
            deferPrefix.test(current)
          )
            return sqlite.SQLITE_OK;
          return deny();
        }
        return sqlite.SQLITE_OK;
      });
      const statement = database.prepare(current);
      const consumed = statement.sourceSQL;
      if (!consumed || !current.startsWith(consumed))
        throw new Error("SQLite did not expose a statement boundary");
      results.push(statement.all());
      database.setAuthorizer(null);
      remaining = nextSql(current.slice(consumed.length));
    }
    database.exec("COMMIT");
    return { results, effects };
  } catch (error) {
    database.setAuthorizer(null);
    if (database.isTransaction) database.exec("ROLLBACK");
    if (deniedEffect)
      error.message += `: denied ${JSON.stringify(deniedEffect)}`;
    throw error;
  }
}
