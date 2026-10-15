---
title: SQLite Migration Application 0.4.0
description: UIDで固定したSQLite databaseへ順序付きのmigration setを適用し、適用済みの履歴を書き換えずに保つ。
formUrl: https://edge.forms.takoform.com/forms/SQLiteMigrationApplication/0.4.0/
hostApi: forms.takoform.com/v2
---

# SQLite Migration Application 0.4.0

このFormは、指定されたSQLiteMigrationSetを指定されたSQLiteDatabaseへ順番に適用し、進行状況を記録します。database内の永続履歴を基準に、適用済みmigrationの内容や順序の書換えを拒否し、未適用分だけを実行します。Application Resourceを削除してもdatabaseのschemaや履歴は元に戻しません。

Resourceの作成・取得・更新・削除、認可、世代、管理Operation、失敗応答と再送規則は[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。このFormを公開したことは、Hostの対応や実装を意味しません。

## 1. 目的と範囲

Application Resourceは指定したMigration Setとdatabaseの関係を記録し、databaseをそのSetの履歴まで進めます。Migration Setは順序付きSQL fileと正確なbyte列を保管する別Resourceです。その入力、manifest、URL取得の安全制限、fileの検証と保管は[SQLiteMigrationSet 0.4.0](../../SQLiteMigrationSet/0.4.0/)が定めます。

databaseとMigration SetはHostが発行したResource UIDで特定します。名前で検索したり、「最新」のResourceへ付け替えたりしません。本Form URLは`https://edge.forms.takoform.com/forms/SQLiteMigrationApplication/0.4.0/`であり、完全一致で識別します。

## 2. 入力と既定値

`spec`は次の二つの必須フィールドだけを持つobjectです。既定値はなく、作成後は変更できません。

| フィールド | 型・制約 | 既定値 | 作成後 |
| --- | --- | --- | --- |
| `database` | `{ "resourceUid": string }`。Host API v2で定義する不透明で空でないUID。参照先Form URLは`https://edge.forms.takoform.com/forms/SQLiteDatabase/0.2.0/`と完全一致すること。 | なし。必須。 | 不変。 |
| `migrationSet` | `{ "resourceUid": string }`。不透明で空でないUID。参照先Form URLは`https://edge.forms.takoform.com/forms/SQLiteMigrationSet/0.4.0/`と完全一致すること。 | なし。必須。 | 不変。 |

UIDは大文字・小文字を区別する不透明な文字列です。内容を解釈し直したり、名前・path・slug・digestで置き換えてはいけません。参照objectは`resourceUid`だけを持ちます。別のdatabaseまたはSetへの差替えを更新として受け付けません。新しい組合せには別のApplication Resourceを作ります。

例:

```json
{
  "database": { "resourceUid": "res-db-7N4wQ0Jm" },
  "migrationSet": { "resourceUid": "res-set-2kL9sD1p" }
}
```

## 3. 秘密入力

`privateInputs`はありません。Migration SQL、配布file、credentialを秘密入力として受け付けません。Hostが配布元へ接続するときは呼出者の認証情報を送らず、Hostの認可境界で取得します。SQLと操作文書は公開入力として扱います。

## 4. 観測状態、出力、利用可能状態

作成直後は`observed: {}`、`output: {}`、`observedAt: null`です。二つのResource参照と履歴をすべて確認した後は、`observed`が次の形になります。

```json
{
  "databaseUid": "res-db-7N4wQ0Jm",
  "migrationSetUid": "res-set-2kL9sD1p",
  "appliedEntries": [
    { "path": "migrations/001-create.sql", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }
  ],
  "targetEntries": [
    { "path": "migrations/001-create.sql", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" },
    { "path": "migrations/002-add-index.sql", "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" }
  ],
  "ready": false
}
```

`observed`は未観測なら空object `{}`です。観測済みの場合は上例のfieldすべてを持ち、他のfieldを含めません。`databaseUid`と`migrationSetUid`は`spec`のUIDと完全一致する文字列です。`appliedEntries`は0〜512件、`targetEntries`は1〜512件の順序付きarrayです。各entryは`path`（1〜1024 UTF-8 byteの相対POSIX path）と`sha256`（prefixなし小文字hex 64桁）だけを持ちます。`targetEntries`はSQLiteMigrationSetの全件検証済みmanifestです。`ready`はbooleanで、二つのUIDが参照先と一致し、`appliedEntries`配列が`targetEntries`配列とpath・digest・順序のすべてで完全一致したことを観測した場合だけtrueです。件数だけの一致やprefix一致では足りません。確認不能なときは部分観測を合成せず、最後の完全観測とその`observedAt`を維持します。`output`は常に空objectです。readyはmigration履歴の一致を示すだけで、applicationの稼働を示しません。

## 5. 操作と失敗

Host API Resource操作は作成・取得・更新・削除です。create前に両UIDが同じHost・同じSpaceに存在し、参照Form URLが期待値に完全一致し、呼出者に個別の利用権限があることを検査します。UID参照だけで権限は得られません。Hostは参照先ResourceをUIDに固定し、以後名前で再解決しません。削除中・削除済みの参照は受理しません。

HostはMigration Setの全manifestとfile bytesを検証し、databaseの永続履歴を読みます。target manifestは履歴全体を順序どおり含み、過去entryのpathとdigestが完全一致しなければなりません。変更・並替え・削除はSQL実行前に拒否します。適用済みentryは再実行せず、未適用分だけを実行します。各migration fileは厳密なUTF-8として解釈します。1 file内には複数のSQLite SQL statementを記せます。HostはSQLite parser/authorizerで文ごとの実際の構文・作用を判定し、単語検索で拒否してはいけません。たとえばtrigger本文中の`BEGIN`/`END`はtransaction controlではありません。migration SQLはHostが開始する一つのSQLite transaction内で、そのfile内の全statementを順に実行します。最終statementと当該entryの履歴追記は一体でcommitし、SQL失敗、制限違反、非許可作用があればそのfileと履歴追記の両方をrollbackします。前のfileのcommit済み進捗は保持し、同じApplicationの再調整は最初の未記録fileから再開します。

1 fileはSQLiteMigrationSetが定める上限16 MiB以内です。SQLはdatabaseのschemaと利用者データを変更できますが、Host管理のmigration ledgerを読み書き・改名・削除したり、履歴との対応を崩す作用は禁止です。transaction開始/commit/rollback/savepoint制御、`ATTACH`/`DETACH`、`VACUUM`、下記の例外以外の`PRAGMA`、およびHost transaction外へ効果を逃がす作用は禁止します。上記作用をtriggerなどSQL objectの本文内で宣言する場合も、実行時の効果が禁止範囲に達してはなりません。databaseへの外部接続、拡張読み込み、Host外のfile・network・processアクセスも許しません。Hostはstatementの解析とSQLite authorizer等を通じて実行時の作用を制限します。SQLiteMigrationSetはfile bytesの検証・保管を行い、この実行制約を所有しません。

### defer_foreign_keysの代入

PRAGMAによる設定変更は、次の代入形だけを許可します。`ON`、`OFF`、`TRUE`、`FALSE`、`1`、`0`は
選択肢であり、`|`をSQLに記述するわけではありません。

```sql
PRAGMA defer_foreign_keys = ON;
PRAGMA defer_foreign_keys = OFF;
PRAGMA defer_foreign_keys = TRUE;
PRAGMA defer_foreign_keys = FALSE;
PRAGMA defer_foreign_keys = 1;
PRAGMA defer_foreign_keys = 0;
```

`PRAGMA`、`defer_foreign_keys`と値のASCII英字は大文字・小文字を区別しません。`PRAGMA`と名前の間には
一つ以上のASCII空白が必要です。文の前後、`=`の前後、末尾の`;`の前後にもASCII空白を置けます。
ここでASCII空白とはU+0009〜U+000DとU+0020です。末尾の`;`は省略できます。名前や値をquoteした形、
schema修飾、括弧による引数、式、コメントをこの代入形に加えた表記は許可しません。Unicodeのcase foldingや
ASCII以外の空白でこの範囲を広げてはいけません。

`PRAGMA defer_foreign_keys;`のような値の読取り、`PRAGMA foreign_keys = ON;`、
`PRAGMA foreign_keys = OFF;`、および次節の読取り専用4種以外のPRAGMAは引き続き禁止です。`ON`、`TRUE`、`1`は現在のHost transactionの
外部キー検査をcommitまで遅延させ、`OFF`、`FALSE`、`0`は遅延設定を無効にします。`foreign_keys`による
外部キー検査自体の有効・無効は変更しません。Hostはこの例外を、fileのtransaction開始・終了やledger操作を
migration SQLへ許す根拠にしてはいけません。commitまたはrollback後、次のfileへこの設定を引き継ぎません。

Hostはfile全体をSQLite parserで分離し、文ごとの構文と作用をSQLite authorizer等で制限します。
上記の許可は一つのPRAGMA statementに対するものです。複数statementをまとめて一つの許可形と判定したり、
fileに許可PRAGMAが一つあるだけで残りのSQLを許可したりしてはいけません。

### 読取り専用のschema introspection

migration SQLでは、次の4種に限って、PRAGMA statementと対応するtable-valued PRAGMA functionによる
読取りを許可します。PRAGMA statement名には`pragma_`を付けません。

| PRAGMA statement名 | table-valued function名 | 読み取る情報 |
| --- | --- | --- |
| `table_info` | `pragma_table_info` | 指定tableまたはviewのcolumn定義。 |
| `foreign_key_list` | `pragma_foreign_key_list` | 指定tableの外部キー定義。 |
| `index_list` | `pragma_index_list` | 指定tableのindex一覧。 |
| `index_info` | `pragma_index_info` | 指定indexのcolumn定義。 |

PRAGMA statementではSQLiteの読取り構文`PRAGMA [main.]name(object)`または
`PRAGMA [main.]name = object`を使えます。ここで`[main.]`は省略可能という説明であり、角括弧を
そのままSQLへ記述する意味ではありません。これら4種の`= object`は対象objectの指定であり、設定の書込みでは
ありません。対象名のSQLiteのidentifier/string quoting、大文字・小文字とコメントの規則をそのまま使います。
末尾の`;`は省略できます。対象objectはtable/view/indexの一つの名前です。引数を省略した形や存在しないobjectが
SQLiteの通常の意味で空結果を返す場合も、読取り専用であることだけを理由に失敗させません。

functionでは`pragma_name(object)`または`pragma_name(object, 'main')`のように、objectと省略可能な
schema指定を渡します。objectはSQL式でよく、たとえば`pragma_foreign_key_list(source.name)`の相関参照を
許可します。SQLiteの通常のSELECTとしてprojection、WHERE、JOIN、CTE、subquery等へ組み込めます。
functionの省略可能なschemaは最後の引数です。SQLiteのeponymous virtual tableとしてhiddenな`arg`/`schema`
columnを制約する等価な読取りも同じ制限に従います。object省略で空結果となるnativeな読取りを、このForm独自の
必須引数規則で拒否しません。SQL構文や引数がSQLite自体で不正な場合は通常の`migration_sql_error`です。

```sql
PRAGMA table_info('child');
PRAGMA main.foreign_key_list('child');
PRAGMA index_list = 'child';
PRAGMA main.index_info('child_parent_idx');
SELECT * FROM pragma_table_info('child');
SELECT * FROM pragma_index_info('child_parent_idx', 'main');
SELECT source.name, fk."from", fk."table"
  FROM main.sqlite_master AS source
  JOIN pragma_foreign_key_list(source.name, 'main') AS fk
 WHERE source.type = 'table' AND source.name = 'child';
```

introspectionの対象databaseは参照先SQLiteDatabaseの`main`だけです。schema省略時も、明示した`main`も、
ASCII大文字・小文字を区別せず同じ`main`を指します。`temp`またはattached databaseを対象にする指定・解決は
拒否します。functionの名前の修飾とfunction末尾のschema引数を混同してはいけません。Hostはparseした構文と
実行時に解決されるdatabase/objectを検査し、動的引数、schema省略、alias、view、CTE、subqueryやtrigger経由でも
この境界を守ります。objectを動的に指定できることはHost管理migration ledgerの直接introspectionまたはその行の
読取りを許可しません。

`main`の`sqlite_master`と`sqlite_schema`は同じschema catalogを読む名前として許可し、schema省略も同じ
`main`への解決に限って許可します。catalogからのSELECTや通常の読取りqueryはtable/index/view/triggerの定義を
参照できます。schema一覧にHost ledgerの名前が現れること自体はledger行の読取りとは区別します。catalogを
読めることはledgerの直接introspection、行の読取り・書込み・改名・削除を許す根拠にはなりません。
`sqlite_temp_master`/`sqlite_temp_schema`、`temp`またはattached schemaのcatalogはこの許可に含めません。

migration SQLからschema catalogを直接INSERT/UPDATE/DELETEしたり、`writable_schema`等で直接書込みを
有効にしたり、metadataを経由してschemaや設定を書き換えたりしてはいけません。Hostはparser/authorizerで
直接のcatalog書込みを拒否します。一方、認可された通常のCREATE/ALTER/DROP等のDDLをSQLite engineが実行する際の
内部catalog更新はそのDDLの一部です。catalogへの全書込みを機械的に禁止して正当なDDLを拒否してはいけません。
Host ledgerを対象にするDDLは引き続き禁止です。

上記4種以外のPRAGMA statementと`pragma_*` table-valued functionは、読取りだけであっても禁止です。
`foreign_keys`の読取り・代入、`defer_foreign_keys`の読取り、`table_xinfo`/`table_list`、schema/configurationの
変更等をこの例外で許可しません。以前から許可する通常のSQL式・組込みscalar functionを、この4種のリストに
ないという理由だけで禁止するわけではありません。いずれのmetadata読取りも、既存のSQL副作用・ledger・
transaction制約を弱めません。この許可は本Migration Applicationの実行に限り、Worker向け
[SQLiteDatabase 0.2.0](../../SQLiteDatabase/0.2.0/) BindingのSQL制限を変更しません。

構文とschema catalogのSQLite側の説明は[PRAGMA functions](https://www.sqlite.org/pragma.html#pragfunc)と
[Schema Table](https://www.sqlite.org/schematab.html)を参照できます。許可範囲と保護境界は本Formの規則に従います。

DatabaseやMigration Setの不在、別Host/Space、Form URL違い、参照権限なし、manifest/file digest不一致、空manifest、重複path、不正UTF-8、16 MiB超過、禁止SQL作用、履歴破損・prefix不一致、database利用中・利用不能、SQL構文errorが失敗です。Operation受付前の入力・権限・参照検査失敗はHost API v2が定める`invalid_request`、`forbidden`、`not_found`、`dependency_conflict`または`invalid_spec`として拒否し、SQLを実行しません。受理後のOperationが失敗した場合、その`error.code`は`artifact_invalid`（manifest・file・digest検証失敗）、`migration_history_conflict`（履歴破損、過去entryの変更または適用順序の不一致）、`migration_sql_error`（SQL構文・禁止作用・実行失敗）、`database_busy`（databaseの排他利用中）、`backend_unavailable`（実行先利用不可）のいずれかです。OperationはHost API v2に従って失敗を返し、効果が不明なら`reconciling`、一部適用済みなら`failed`と部分効果を記録します。readyに偽装してはいけません。

| 操作 | 成功時の意味 | 失敗・復旧 |
| --- | --- | --- |
| `create` | 二つのUIDを固定し、未適用fileを順に実行する。 | 参照・認可・履歴prefixをSQL前に検査。不明結果は元Operationとdatabase履歴を照合し、未記録fileから続ける。 |
| `read` | 固定参照、検証済みtarget履歴・database履歴、readyを返す。 | 参照先が一時利用不可なら最後の完全観測を保持し、`observedAt`を更新しない。 |
| `update` | 現在の二つのUIDと同じspecだけを受理する。新generation・新Operationを必ず作り、再照合・再調整する。 | UID変更は副作用前に拒否。別組合せには新Resourceを作る。 |
| `delete` | Application Resourceだけを削除する。 | 管理状態の不確実さを照合する。database、履歴、Migration Set、fileを削除せず、down migrationや連鎖削除をしない。 |

適用は永続schema/dataを変える非冪等操作です。応答不明を新Applicationや新しい再送keyによる最初からのやり直しで解決してはいけません。Host API v2の元keyで同じ管理Operationを特定し、database履歴と完全観測を照合します。migration fileのbyte上限はSQLiteMigrationSetが定め、SQL実行とtransaction制約は本Formが所有します。Workerから呼ぶSQLiteDatabaseのSQL Bindingはmigrationの代用になりません。

## 6. 参照、所有、削除

両参照の形は`{ "resourceUid": string }`です。UIDは不透明で空でなく、参照ごとに次をすべて検査します。

- Resourceが同じHost・同じSpaceに属すること。
- Form URLが期待する版固定URLと完全一致すること。
- 呼出者にその個別Resourceを読む・利用する権限があること。
- Resourceが削除中・削除済みでないこと。Migration Setなら全fileのbyte列とdigestを検証できること。

参照文字列は権限を与えず、同じSpaceにあるだけでも足りません。ApplicationはdatabaseとMigration Setを所有しません。同じdatabaseを複数Applicationが参照する場合、Hostはmigration実行を直列化し、履歴との一致を毎回確かめます。Migration Setとfileの所有者はSQLiteMigrationSet Formです。

ApplicationのdeleteはこのResourceだけを除去します。down migrationを実行せず、履歴を消去・短縮・書換えせず、databaseやMigration Setも削除しません。依存Resourceへの連鎖削除はありません。databaseやMigration Setを削除するのは各Resourceへの別要求です。生きたApplicationが参照するMigration Setの削除は`409 dependency_conflict`で拒否します。

## 7. Host API操作

このFormにWorker向けJavaScript Bindingやdata-plane関数はありません。管理操作はHost API v2 Resource APIです。作成・更新・削除の結果は管理Operationで確認します。各操作の形を示します。

```text
type ResourceRef = { resourceUid: string }
type Spec = {
  database: ResourceRef;     // SQLiteDatabase の版固定URL
  migrationSet: ResourceRef; // SQLiteMigrationSet の版固定URL
}
POST /resources { form: "https://edge.forms.takoform.com/forms/SQLiteMigrationApplication/0.4.0/", space, name, spec: Spec }: Operation<Resource>
GET /resources/{resourceUid}: Resource
PUT /resources/{resourceUid} (expectedGeneration, idempotencyKey, spec: Spec): Operation<Resource>
DELETE /resources/{resourceUid} (expectedGeneration, idempotencyKey): Operation<Resource>
```

HTTP path、header名、Operation field、statusとproblem detailの完全な構文は[Host API v2](https://takoform.com/spec/host-api/v2/http)が定めます。このFormは別のHTTP routeを追加しません。同一specのPUTも新しいgenerationとOperationを必ず作り、参照とdatabase履歴を再照合します。DELETEはApplicationだけに作用し、schemaやdataを戻しません。

## 8. 未知のフィールドと拡張

`spec`は`database`と`migrationSet`だけを持ち、各参照objectは`resourceUid`だけを持ちます。未知field、余分な`name`/`kind`/`apiVersion`/locator、private inputを拒否します。`observed`は未観測時の空object、または§4で定める全fieldを持つobjectだけです。Hostは余分な観測fieldや不完全な配列を返しません。Host API Resource/Operation共通fieldの未知拡張は[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。参照形式、履歴、削除の意味を変えるには別Form URLを使います。
