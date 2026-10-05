---
title: SQLite Migration Set
description: 明示的なDB適用に使う不変で順序付きのSQLite migration集合
formUrl: https://edge.forms.takoform.com/forms/SQLiteMigrationSet/0.2.0/
hostApi: forms.takoform.com/v2
---

# SQLiteMigrationSet 0.2.0

## 1. 目的と範囲

このFormは、不変で順序付きのSQL migration file集合を表す。Hostはbytesを検証して保持するが、SQLの実行、
databaseの所有、rollbackは行わない。databaseへの適用は別のSQLite Migration Application Formが所有する。
識別子は上記の完全なForm URLであり、Takoform Host API v2を使う。

Hostはcreate、read、update、deleteの全操作を実装し、この契約全体に適合するときだけ対応済みと宣言する。
`supported: true`は技術的対応を表すだけで、取得元へのアクセス許可や容量予約ではない。Hostは個別の取得を
ネットワーク・資源ポリシーにより拒否できる。Offeringを公開する場合、その件数・サイズ・取得範囲はHostの
実際の上限を超えて約束してはならない。
manifestはmigration payloadの目録であり、Form仕様の配布物ではない。HostがForm仕様を取得・実行する必要はなく、
package、署名、Coreを利用要件としない。

## 2. 入力と既定値

公開 `spec` は次のキーだけを持つ。

```json
{
  "artifact": {
    "url": "https://artifacts.example.invalid/migrations/manifest.json",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }
}
```

`artifact`は必須で既定値がなく、作成後は変更できない。`url`はmanifestの絶対HTTPS URLでASCII 8192文字以下。
userinfo、query、fragmentを含めてはならない。公開者はURLのどの部分にもcredentialなどの秘密を置いてはならない。
Hostはuserinfo、query、fragmentのあるURLを拒否し、呼出者のcredentialを転送せず、URLを通常ログへ出さない。
Hostが任意のopaque pathから秘密を完全判定できるとは限らず、必要なら取得元policyを加える。`sha256`は接頭部分の
ない小文字16進64桁で、manifestのHTTP応答本文の厳密なbyte列に対するSHA-256である。URLまたはdigestを変える場合は
新しいResourceを作る。

manifestはUTF-8のJSON objectで、未知fieldと重複keyを拒否し、`files`だけを持つ。

```json
{
  "files": [
    {
      "path": "migrations/0001-create.sql",
      "url": "https://artifacts.example.invalid/migrations/0001-create.sql",
      "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
      "mediaType": "application/sql"
    }
  ]
}
```

manifestは最大1 MiB。`files`は必須の順序付き配列で要素数は1〜512。各file objectはちょうど`path`、`url`、
`sha256`、`mediaType`を持ち、すべて必須で既定値はない。各fileは最大16 MiB、全fileの合計は最大128 MiB。
各`mediaType`は正確に `application/sql` とする。migration順は配列順そのものである。各migration fileはUTF-8として
厳密にdecodeでき、UTF-8 BOMで始まってはならない。

`path`は空でない相対POSIX pathでUTF-8表現が1024 bytes以下。先頭・末尾の`/`、backslash (U+005C)、`?`、
`#`、制御文字 (U+0000..U+001F、U+007F)、空segment、`.`または`..` segmentを含めてはならない。
比較はUnicode文字列の完全一致で大文字小文字を区別し、重複pathを拒否する。fileごとの`url`もASCII 8192文字
以下の絶対HTTPS URLでuserinfo・query・fragmentなしとする。`sha256`は接頭部分のない小文字16進64桁で、HTTP応答
本文の厳密なbyte列を検証する。このFormに既定値はない。

## 3. 秘密入力

秘密入力はない。`spec`、manifest、URLにcredential、access token、署名付きURLなどの秘密値を含めてはならない。
Hostはuserinfo、query、fragmentを含むURLを拒否し、呼出者のcredentialを転送せず、URLを通常ログへ出さない。
Hostが任意のopaque pathに秘密があるかを完全判定できるとは限らず、必要なら取得元policyを加える。
`privateInputs`は省略または空objectだけを受け付ける。

## 4. 観測状態、output、利用可能状態

未検証の`observed`は `{}`。検証後は`manifestSha256`（小文字hex 64桁の文字列）、`fileCount`（1〜512の整数）、
`totalBytes`（1〜134217728の整数）、`files`を含める。`files`はmanifestと同じ順序の配列で、各要素は正確に
`path`（文字列）、`sha256`（接頭部分のない小文字hex 64桁）、`mediaType`（文字列）、`byteSize`（0〜16777216の
整数）を持つ。`fileCount`は配列長、`totalBytes`は各`byteSize`の合計と一致する。取得URLは含めない。全fileの
digest照合と永続保持が済むまで、検証済みとして報告してはならない。`output`は常に `{}`。

管理操作の成功はHostが順序付きmigration fileを検証・保持したことだけを示す。databaseがmigrationを適用したこと、
対象DBでSQLが有効なこと、アプリケーションが利用可能なことを示さない。部分取得一覧を完了済み観測にせず、
失敗・中断時もResourceとOperationをHost API v2の規則で保持する。

## 5. 操作と失敗

- **create:** specとmanifestを検証し、全fileを取得・digest照合する。manifestと一致したバイト列をHost管理の
  永続保持領域に保存してから成功させる。取得・検証中はSQLを実行せず、SQL文の構文や適用可否も検証しない。
  digestを保持物の識別子に使い、応答不明や再起動後も同じ識別子へ別内容を割り当てない。
- **read:** Resourceと最後に完了した観測を返す。取得元から再取得したり、databaseに対するSQL検証や新しい
  検証Operationを始めたりしない。
- **update:** 更新後の`spec.artifact`はURL文字列とdigest文字列がそれぞれ完全一致しなければならない。
  JSON objectのkey順は比較に影響しない。値の変更は副作用前に拒否する。同一spec PUTはHost API v2通常の
  generationとOperationを作って同じ保持物を再照合するが、新しい識別子やバイト列を作らない。
- **delete:** 生きたSQLite Migration Applicationが参照中なら `409 dependency_conflict` で拒否し、変更しない。
  参照がなくなった後は、このResourceだけが所有する保持物を解放する。取得元や別ownerの共有blobを削除しない。
  DB、migration ledger、schemaを変更せず、down migrationや補償SQLを実行しない。

Hostは入力URLだけを根拠に取得権限を得ない。バイト列の解決方法は、(1)呼出者が利用を認可されたHost内の検証済み
永続保持物、(2)公開HTTPSからの取得、(3)Host運用者が事前設定した正確なprivate origin / network allowlistと
Host保持認証のいずれかとする。接続時は選択policyの許可先を確認し、実接続IPを許可範囲内へ固定または照合する。
HTTPS hostnameとTLS証明書検証は維持する。許可されないprivate、loopback、link-local、multicast、reserved、
metadata-service addressへの接続を拒否する。userinfo、query、fragmentは認めず、呼出者の認証情報を送らない。
Host保持認証を別originへ転送せず、redirectを追従しない。取得時は`Accept-Encoding: identity`相当を要求し、
identity以外のcontent-encodingを拒否する。SHA-256対象はHTTP transfer framing除去後、文字decode前の応答本文。
HTTP 200以外は失敗。有限のHost timeoutを設け、manifest 1 MiB、各file 16 MiB、aggregate 128 MiBの上限を超える
前に読み取りを止める。Hostは到達不能またはpolicy不適合の取得元を拒否できるが、別内容で代替しない。timeout、digest
不一致、invalid JSON、duplicate key、invalid UTF-8、未知field、不正path、重複path、`application/sql`以外のmedia type
は検証失敗である。このFormはartifact転送endpointやcredential APIを定義しない。

HostはResourceまたは生きたApplication参照が必要とする限りmanifestとmigration file、または同等のcontent-addressed
永続保持を続ける。再起動、応答喪失、取得元消失後も同じ識別子と検証状態を回復できるようにする。digest一致blobの
存在だけで利用を認可せず、再利用前に当該Resourceの所有権・認可を確認する。他ownerが参照する共有blobをGCしては
ならない。不完全な保持物はHost API v2に従って失敗または`reconciling`とし、同じ識別子に別バイト列を割り当てない。
同じrequestと元Idempotency-Keyは元Operationを返し、新しいkeyによる同一spec PUTは通常のgeneration/Operation
手順を通る。

## 6. 参照、所有、削除

このFormはResourceと、それが作成したartifact custodyだけを所有する。SQLite Migration ApplicationはHost Resource
UIDを固定してMigration Setを参照できる。ここで生きたApplicationとは、操作の成功・失敗や実行中かどうかによらず、
削除が完了していないApplication Resourceを指す。Migration SetはApplicationやdatabaseを所有しない。生きたApplicationが
参照中は削除を拒否する。参照解消後はexclusive custodyだけを解放する。remote sourceやshared blobは削除しない。
削除によってmigration ledgerを除去したり、schemaを戻したり、databaseを削除したり、補償SQLを実行したりしない。

## 7. InterfaceとBinding

必要・提供するInterface、Bindingはない。Hostのartifact取得能力は実装側の責任であり、Resourceがportableな権限を
付与するわけではない。consumerは明示的なSQLite Migration Application操作を通じてだけSQLを適用する。

## 8. 未知fieldと拡張

`spec`、`artifact`、manifest、file objectの未知fieldを拒否する。未知の`privateInputs`も拒否する。`files`配列の順序は
migration順として意味を持ち、厳密に保持する。Applicationはdatabaseの適用済みpath-and-digest列をprefixとして
保持し、その後ろへの追加だけを適用できる。適用済みentryの書換え、並べ替え、削除をSQL実行前に拒否する。
拡張fieldはない。shape、limits、path規則、lifecycle意味を変える場合は新しいForm URLを使い、既存v1 Form bytesと
意味は変えない。
