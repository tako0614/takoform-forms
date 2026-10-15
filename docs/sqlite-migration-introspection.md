# SQLite migration introspection — source only

[SQLiteMigrationApplication 0.4.0](../spec/forms/SQLiteMigrationApplication/0.4.0/index.md)
adds read-only schema inspection to migration execution. It refers to
[SQLiteMigrationSet 0.4.0](../spec/forms/SQLiteMigrationSet/0.4.0/index.md), which
continues to retain ordered SQL bytes without validating or executing SQL.
Both identities are unpublished source successors. The 0.2.0 and 0.3.0
specifications and their frozen anchors retain their exact bytes.

The 0.3.0 `defer_foreign_keys` assignment remains unchanged. Four metadata
operations become available as PRAGMA statements and table-valued PRAGMA
functions: `table_info`, `foreign_key_list`, `index_list` and `index_info`.
The statement spelling is `PRAGMA table_info('child')`; the table-valued
spelling is `SELECT * FROM pragma_table_info('child')`. The `pragma_` prefix
belongs to the table-valued form. SQLite's equivalent read-only selector
`PRAGMA table_info = 'child'` is also allowed; its equals sign does not make
it a configuration write.

Table-valued functions accept ordinary SQL object expressions, including
correlated inputs such as `pragma_foreign_key_list(source.name)`. They may
appear in JOINs, CTEs and subqueries. A migration that validates its expected
schema before rebuilding a table can inspect columns and foreign keys,
compare definitions from `sqlite_master`, then execute its normal DDL.
The containing statement still requires full admission checks: a permitted
metadata source does not authorize another source, function or side effect.

Metadata scope is the referenced database's `main` schema. Both implicit
main resolution and explicit `main` are allowed; temp and attached schemas
are denied. PRAGMA schema qualification and the optional last schema argument
of a table-valued PRAGMA function are different syntax. Dynamic arguments
must obey the same resolved database and protected-object limits as literals.
The normative Form specifies SQLite's native quoting, comments and argument
forms; the strict spelling of `defer_foreign_keys` is a separate rule.

`sqlite_master` and `sqlite_schema` are aliases for the main schema catalog.
They can be read, including ordinary catalog enumeration that reveals a
Host ledger's name. This does not authorize direct ledger introspection or
reading its rows. Direct INSERT, UPDATE or DELETE against the schema catalog
and configuration changes such as `writable_schema` remain forbidden.
Authorized CREATE, ALTER and DROP statements cause SQLite itself to update
the catalog and remain legitimate migration operations. A blanket authorizer
denial of every catalog write would incorrectly reject these DDL statements.

Hosts must combine SQLite parsing with execution authorization. The
[SQLite schema documentation](https://www.sqlite.org/schematab.html) notes
that authorizer callbacks report historical catalog aliases; admission must
check resolved objects rather than spelling alone. The
[table-valued PRAGMA documentation](https://www.sqlite.org/pragma.html#pragfunc)
explains the optional last schema argument and correlated query usage.

These additions apply only to Migration Application execution. They do not
change the Worker-facing [SQLiteDatabase 0.2.0](../spec/forms/SQLiteDatabase/0.2.0/index.md)
SQL Binding. Other PRAGMAs and table-valued PRAGMA functions remain denied,
including `foreign_keys`, read-only `defer_foreign_keys`, `table_xinfo` and
`table_list`. Ledger isolation, prohibited external effects and one-file
transaction/ledger atomicity continue to apply.

## Reuse the conformance inputs

The [0.4.0 cases](../conformance/host-v2/sqlite-migration/v0.4.0/cases.json)
contain shareable SQL inputs and expected admission/observation results. The
[SQL fixture manifest](../conformance/host-v2/sqlite-migration/v0.4.0/fixtures/manifest.json)
identifies the associated complete-file fixtures. Consumers can reuse these
data files without importing this publisher's test harness or depending on
a sibling checkout. These are publisher-owned inputs, not a new generic
Takoform protocol or an extra Form release identity.

A downstream Host adapter must execute the inputs through its actual
parser, authorizer, transaction and ledger boundary. It must verify rejection
before forbidden effects, successful metadata observations, legitimate DDL,
and atomic rollback when SQL or admission fails. A word scan or the 0.3.0
single-PRAGMA predicate does not validate a whole migration file.

Local SQLite source checks establish only the exercised local behavior.
They do not prove that a Host supports these Form URLs or that hosted E2E
has recovered. Form publication, Host adoption and production deployment
are separate actions; this source change performs none of them.
