---
title: SQLite Database 0.2.0
description: Embedded SQLite database with serializable transactions and a bounded portable SQL value model.
formUrl: https://edge.forms.takoform.com/forms/SQLiteDatabase/0.2.0/
hostApi: forms.takoform.com/v2
---

# SQLite Database 0.2.0

このFormは、SQLiteに従う一つの組込みdatabaseを表します。SQL値の表現、transaction、Workerから実行できるSQLの範囲はどのHostでも同じです。製品固有の実行方式を切り替える設定ではありません。schema変更の履歴と適用は[SQLiteMigrationApplication](../../SQLiteMigrationApplication/0.2.0/)が管理します。このFormは[Host API v2](https://takoform.com/spec/host-api/v2/http)用の契約であり、公開だけでHostの対応や実装を示すものではありません。

Resourceの作成・取得・更新・削除と管理OperationのHTTP形式は[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。SQL実行はWorker Bindingから行う直接の非同期呼出しであり、Host APIの管理OperationやHTTP endpointではありません。SQLと結果の制約は本書がすべて定めます。

## 1. 目的と範囲

Databaseは一つの永続SQLite databaseです。Workerからは`execute`、副作用を残さない`query`、直列化可能なtransactionを呼べます。これらの関数でschemaを変更したり、migration ledgerを改変したり、別databaseへ接続したりはできません。schema変更は`SQLiteMigrationApplication`だけが管理します。

Canonical Form URLは`https://edge.forms.takoform.com/forms/SQLiteDatabase/0.2.0/`です。完全な文字列一致を使います。

## 2. 入力と既定値

`spec`は空object `{}`だけです。設定fieldも既定値もありません。作成後に変更できるfieldはなく、未知field、null、配列を拒否します。engine、schema、transactionの意味を切り替える入力もありません。

Worker関数は後述する位置引数だけを受け付けます。省略した`params`は空配列です。SQL placeholderは順番に値を割り当て、`?1`は`params[0]`を指します。

## 3. 秘密入力

`privateInputs`はありません。認証情報、暗号鍵等をdatabase Formの秘密入力として受け取りません。SQL parameterは操作inputであり、機密情報の安全な保管経路ではありません。

## 4. 観測状態、出力、利用可能状態

作成直後は`observed: {}`、`output: {}`、`observedAt: null`です。観測後の`observed`は`{"databaseExists": boolean}`だけを持つobjectです。例:

```json
{"databaseExists": true}
```

`databaseExists:false`は確認済みの不在です。確認不能な場合はfalseにせず、最後に確実に観測した値を保持します。未知のobserved fieldを追加してはいけません。`output`は常に空objectです。利用可能状態は定義しません。作成の完了はquery成功、schema version、applicationの稼働を保証しません。

## 5. 操作と失敗

Host API Resource操作は作成・取得・更新・削除です。createは空databaseと空migration ledgerを作ります。既存databaseを初期化・上書きしてはいけません。結果不明ならResourceと実行先対応を保持し、同じResourceのread、同一`spec`のPUT、またはdeleteで照合します。空`spec`と異なる更新は副作用前に拒否します。同じ空`spec`のPUTは新generation・新Operationを作り、管理対応を再確認します。deleteはdatabase全体を消すため、WorkerVersionまたはSQLiteMigrationApplicationが参照中なら受理せず`409 dependency_conflict`を返し、何も削除しません。参照先を強制削除する連鎖処理はありません。参照がなく削除を始めた後に失敗・中断した場合、進捗を保持し、同じResourceのdeleteで再開または照合します。

SQL関数はPromiseを返します。成功した値は下表の通りです。関数固有の失敗ではPromiseを、表のcodeを`name`に持つ`Error`で拒否します。引数型・配列長・値形状の不正は`TypeError`です。Worker呼出しにはHost API HTTP ProblemやResource管理Operation IDはありません。SQLは一つだけ実行し、後続は空白・commentだけに限ります。複数statement、transaction制御文（`BEGIN`、`COMMIT`、`END`、`ROLLBACK`、`SAVEPOINT`、`RELEASE`）、schemaを変更する文（`CREATE`、`ALTER`、`DROP`。一時schemaやindex、view、triggerを含む）、`ATTACH`、`DETACH`、`VACUUM`、`PRAGMA`、`load_extension`、`sqlite_schema`またはmigration ledgerへのアクセス・変更は`sql_error`です。HostはSQLite parserとauthorizer等で構文および実際の作用を検査して禁止します。SQL本文を単語検索して判定してはいけません。文字列literal、comment、識別子、または許可された文から呼び出す既存triggerの本文に禁止語が現れることだけを理由に、実際には禁止作用のないSQLを拒否してはいけません。

| JavaScript関数 | 入力 | Promiseが解決する値 | Promise拒否の`Error.name` | 再実行・transaction |
| --- | --- | --- | --- | --- |
| `execute(sql, params?)` | `sql`: 文字列、`params`: 省略可の値配列 | `{rows: Row[]; rowsWritten: number}` | `sql_error`, `numeric_out_of_range`, `busy`, `backend_unavailable` | 非冪等。1 statementの効果をcommitする。全結果検査後にcommit。 |
| `query(sql, params?)` | 同上 | `{rows: Row[]; rowsWritten: 0}` | `sql_error`, `numeric_out_of_range`, `busy`, `backend_unavailable` | rollback-only。書込み文でも必ずrollbackする。 |
| `transaction(statements)` | 1〜100件の`{sql, params?}`配列 | `{results: {rows: Row[]; rowsWritten: number}[]}` | `sql_error`, `numeric_out_of_range`, `busy`, `backend_unavailable` | 非冪等。全statementを一つのserializable transactionで確定。失敗時すべてrollback。 |

`EdgeSqlValue`は`null`、有限binary64 number、UTF-8 string、または`{"encoding":"base64","data":"..."}`のいずれかです。numberは絶対値9,007,199,254,740,991以下とし、SQLite INTEGER/REALの保存形式の差を返しません。Hostはnumberを丸めたり文字列化、tag化、暗黙変換してはいけません。範囲外・有限でないDB出力は`numeric_out_of_range`です。JavaScript BindingでもBLOB値はこのBase64 objectで渡し、ArrayBufferへ変換しません。

```js
const result = await env.DB.query("SELECT id, payload FROM item WHERE id = ?1", [7]);
```

Base64はRFC 4648 section 4のpadding付き・改行なし形式で、復号後最大1,000,000 byteです。文字列値は最大1,000,000 UTF-8 byte。SQLは1〜100,000 UTF-8 byte。`params`は最大100値。transactionは1〜100 statements。1 statementの結果は最大10,000 rows、rowは最大100 columns、column名は最大128 UTF-8 byte。1 rowのRFC 8785 canonical JSONは最大2,000,000 UTF-8 byte、1呼出しの全出力は最大8,388,608 byteです。これらの制限には返却形式全体を含めます。入力/outputの`EdgeSqlValue`以外（boolean、bigint、tag付きobject）は許可しません。

`query`はrollback-only transaction内で実行し、成功しても副作用を残しません。`execute`と`transaction`は全行と制限を確定前に検査します。失敗時はtransaction全体をrollbackし、部分結果を返しません。三関数すべてに上記の禁止文・禁止作用が適用されます。`busy`は呼出し全体を再試行できる競合結果です。`execute`・`transaction`のPromise拒否やWorker切断は、SQL効果があったかを確定しないことがあります。これらはHost管理Operationではないため、管理Operation履歴から再実行安全性を得られません。アプリケーションが非冪等SQLの再実行を判断します。

`sql_error`はSQL構文、禁止文、値形式の失敗、`numeric_out_of_range`はportable number範囲外、`busy`は書込み競合、`backend_unavailable`は実行先利用不可です。これらはWorker側では`Error.name`として通知します。Error.messageの文面は固定しません。

## 6. 参照、所有、削除

Database Resourceは自分のdatabase、schema、ledger、全行と全blobを所有します。WorkerVersionとSQLiteMigrationApplicationは本ResourceをUID参照できます。どちらかが生きて参照中ならdeleteは`409 dependency_conflict`で拒否し、何も削除しません。これらの参照元を強制削除する連鎖処理はありません。参照がなくなった後にdeleteすると、database全体が消えます。MigrationApplicationを削除してもschemaやledgerを戻しません。Hostは名前で対象を推測せずUIDで照合します。

## 7. Worker JavaScript Binding

WorkerVersionの`sqliteBindings`がこのResourceをUIDで参照すると、Worker内の`env.NAME`に次のobjectを渡します。`NAME`はWorkerVersionで宣言するJavaScript識別子です。関数は本Formが直接定義し、独立したInterface版やHostのデータ操作HTTP endpointはありません。参照は権限を与えず、HostはWorkerVersionの作成と実行時に権限を検査します。

```text
type EdgeSqlValue = null | number | string | { encoding: "base64"; data: string }
type Row = Record<string, EdgeSqlValue>
type Statement = { sql: string; params?: EdgeSqlValue[] }
execute(sql: string, params?: EdgeSqlValue[]): Promise<{ rows: Row[]; rowsWritten: number }>
query(sql: string, params?: EdgeSqlValue[]): Promise<{ rows: Row[]; rowsWritten: 0 }>
transaction(statements: Statement[]): Promise<{ results: { rows: Row[]; rowsWritten: number }[] }>
```

§5のSQL・値・行・出力上限がこの署名に適用されます。引数形の誤りは`TypeError`、SQL固有の失敗は`Error.name`が`sql_error`、`numeric_out_of_range`、`busy`または`backend_unavailable`となるPromise拒否です。各関数の返却値とtransaction意味は§5で完全に定義します。BLOBはBase64 objectのまま渡し、Streamや別データ経路はありません。

## 8. 未知のフィールドと拡張

`spec`に未知fieldを拒否します。関数は記載した位置引数だけを受け、`Statement`と`EdgeSqlValue` objectにも未知fieldを拒否します。Rowのcolumn名はSQL結果ごとに異なりますが、上限内のUTF-8文字列と許可した値型だけを含めます。Host Resource/Operationの拡張fieldは[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。SQL値やtransactionの意味を変える場合は新しいForm URLを使います。
