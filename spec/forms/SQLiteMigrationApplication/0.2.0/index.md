---
title: SQLite Migration Application 0.2.0
description: UIDで固定したSQLite databaseへ順序付きのmigration setを適用し、適用済みの履歴を書き換えずに保つ。
formUrl: https://edge.forms.takoform.com/forms/SQLiteMigrationApplication/0.2.0/
hostApi: forms.takoform.com/v2
---

# SQLite Migration Application 0.2.0

このFormは、指定されたSQLiteMigrationSetを指定されたSQLiteDatabaseへ順番に適用し、進行状況を記録します。database内の永続履歴を基準に、適用済みmigrationの内容や順序の書換えを拒否し、未適用分だけを実行します。Application Resourceを削除してもdatabaseのschemaや履歴は元に戻しません。

Resourceの作成・取得・更新・削除、認可、世代、管理Operation、失敗応答と再送規則は[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。このFormを公開したことは、Hostの対応や実装を意味しません。

## 1. 目的と範囲

Application Resourceは指定したMigration Setとdatabaseの関係を記録し、databaseをそのSetの履歴まで進めます。Migration Setは順序付きSQL fileと正確なbyte列を保管する別Resourceです。その入力、manifest、URL取得の安全制限、fileの検証と保管は[SQLiteMigrationSet 0.2.0](../../SQLiteMigrationSet/0.2.0/)が定めます。

databaseとMigration SetはHostが発行したResource UIDで特定します。名前で検索したり、「最新」のResourceへ付け替えたりしません。本Form URLは`https://edge.forms.takoform.com/forms/SQLiteMigrationApplication/0.2.0/`であり、完全一致で識別します。

## 2. 入力と既定値

`spec`は次の二つの必須フィールドだけを持つobjectです。既定値はなく、作成後は変更できません。

| フィールド | 型・制約 | 既定値 | 作成後 |
| --- | --- | --- | --- |
| `database` | `{ "resourceUid": string }`。Host API v2で定義する不透明で空でないUID。参照先Form URLは`https://edge.forms.takoform.com/forms/SQLiteDatabase/0.2.0/`と完全一致すること。 | なし。必須。 | 不変。 |
| `migrationSet` | `{ "resourceUid": string }`。不透明で空でないUID。参照先Form URLは`https://edge.forms.takoform.com/forms/SQLiteMigrationSet/0.2.0/`と完全一致すること。 | なし。必須。 | 不変。 |

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

1 fileはSQLiteMigrationSetが定める上限16 MiB以内です。SQLはdatabaseのschemaと利用者データを変更できますが、Host管理のmigration ledgerを読み書き・改名・削除したり、履歴との対応を崩す作用は禁止です。transaction開始/commit/rollback/savepoint制御、`ATTACH`/`DETACH`、`VACUUM`、`PRAGMA`、およびHost transaction外へ効果を逃がす作用は禁止します。上記作用をtriggerなどSQL objectの本文内で宣言する場合も、実行時の効果が禁止範囲に達してはなりません。databaseへの外部接続、拡張読み込み、Host外のfile・network・processアクセスも許しません。Hostはstatementの解析とSQLite authorizer等を通じて実行時の作用を制限します。SQLiteMigrationSetはfile bytesの検証・保管を行い、この実行制約を所有しません。

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
POST /resources { form: "https://edge.forms.takoform.com/forms/SQLiteMigrationApplication/0.2.0/", space, name, spec: Spec }: Operation<Resource>
GET /resources/{resourceUid}: Resource
PUT /resources/{resourceUid} (expectedGeneration, idempotencyKey, spec: Spec): Operation<Resource>
DELETE /resources/{resourceUid} (expectedGeneration, idempotencyKey): Operation<Resource>
```

HTTP path、header名、Operation field、statusとproblem detailの完全な構文は[Host API v2](https://takoform.com/spec/host-api/v2/http)が定めます。このFormは別のHTTP routeを追加しません。同一specのPUTも新しいgenerationとOperationを必ず作り、参照とdatabase履歴を再照合します。DELETEはApplicationだけに作用し、schemaやdataを戻しません。

## 8. 未知のフィールドと拡張

`spec`は`database`と`migrationSet`だけを持ち、各参照objectは`resourceUid`だけを持ちます。未知field、余分な`name`/`kind`/`apiVersion`/locator、private inputを拒否します。`observed`は未観測時の空object、または§4で定める全fieldを持つobjectだけです。Hostは余分な観測fieldや不完全な配列を返しません。Host API Resource/Operation共通fieldの未知拡張は[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。参照形式、履歴、削除の意味を変えるには別Form URLを使います。
