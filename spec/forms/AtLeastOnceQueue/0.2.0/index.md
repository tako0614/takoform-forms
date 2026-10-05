---
title: AtLeastOnceQueue
description: 順序を保証しないat-least-onceキューResource。
formUrl: https://edge.forms.takoform.com/forms/AtLeastOnceQueue/0.2.0/
hostApi: forms.takoform.com/v2
---

# AtLeastOnceQueue 0.2.0

このFormは、Hostが所有する一つのメッセージキューを表します。配信は少なくとも一度で、順序は保証されません。重複と送信順以外の順序での配信があり得るため、利用者の処理は冪等でなければなりません。Host API v2のResourceとして管理します。共通の作成・取得・更新・削除と再送規則は[Takoform Host API v2 HTTP API](https://takoform.com/spec/host-api/v2/http)に従います。特定Hostへの対応、容量、可用性、料金、アプリケーションの稼働を保証しません。

## 入力

`spec` は次のフィールドだけを受け付けます。整数は小数や文字列へ暗黙変換しません。

| フィールド | 要件、既定値、範囲 | 作成後の変更 |
| --- | --- | --- |
| `deliveryDelaySeconds` | 任意の整数。省略時のForm既定値は `0` 秒（遅延なし）。`0..43200`。各送信で別の遅延を指定した場合はその送信値が優先されます。 | 変更できます。更新は以後に受理するメッセージの既定遅延だけを変更し、すでに受理したメッセージの時刻を変更しません。 |
| `messageRetentionSeconds` | 必須の整数。Form既定値なし。`60..1209600` 秒。キューがメッセージを保持する最大期間です。 | 変更できます。更新は以後の保持判定に適用します。既に期限切れとなったメッセージを復元しません。 |

```json
{
  "deliveryDelaySeconds": 0,
  "messageRetentionSeconds": 86400
}
```

秘密入力はありません。空でない `privateInputs` は拒否します。`spec` の未知のフィールドも拒否します。

## producer Bindingとメッセージ

WorkerVersionはこのQueueをJavaScript producer Bindingとして参照できます。Bindingの実行時呼出しは下記のWorkerVersion 0.5.0 producer Binding契約に従います。キューを受け取るWorkerのイベントとバッチsettlement APIは[ModuleWorker 0.3.0 runtime契約](https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/#runtime)および[QueueConsumer 0.3.0](https://edge.forms.takoform.com/forms/QueueConsumer/0.3.0/)に従います。

成功した `send` / `sendBatch` は受理・永続化を意味し、消費済み・処理済みを意味しません。配信は後続し、重複し、送信順と異なる順序になり得ます。`messageId` は1〜256文字のキュー内で一意な不透明値で、再配信中は変わりません。`timestampMillis` はHostが受理したUTC時刻のUnix epochミリ秒で、再配信中も変わりません。`attempts` は1以上の整数で、初回が1、再配信ごとに1増えます。

キューあたりのQueueConsumerは最大1つです。複数Consumerが同じストリームを別々の再試行・dead-letter方針で分割しないよう、Hostは2つ目を拒否します。接続先の許可はResource参照だけから推定せず、producer Bindingのすべての呼出しを実行主体に認可します。

## 観測、出力、利用可能状態

`observed` はHostが最後に確認したキューの存在を表し、確認できた場合は `queueExists: true`、存在しないことを確認した場合は `queueExists: false` を含みます。初回の観測前や外部状態が不明な間は `{}` とし、未確認を空キューや不在として扱ってはいけません。任意の `messageCount` は0以上の整数で、観測時点にHostが把握した件数の概数です。これを将来の配信量、厳密な在庫、処理完了の証明として扱ってはいけません。

```json
{
  "queueExists": true,
  "messageCount": 12
}
```

このFormの `output` は常に `{}` です。キューResourceの操作が成功しても、Consumerの存在、メッセージ処理、外部サービスの健全性や到達性を保証しません。このFormはアプリケーションの利用可能状態を定義しません。

## 操作、失敗、復旧

- **作成:** v2の認可、名前、入力、Offering、参照の検証後にキューを一つ作成します。同じHost/SpaceのResource名衝突、無効な値、容量や実行先の拒否は副作用前に返します。キュー作成後に応答が失われた場合、Hostは操作IDと作成対象を永続化し、再起動後も同じOperationを照合します。作成結果が不明なら `effect:unknown` のまま `reconciling` し、別キューを盲目的に作りません。
- **取得:** Resourceの管理記録と最後に確定した観測を返します。GETが配信や再送を開始することはありません。削除済みResourceのHTTP結果は共通Host APIに従います。
- **更新:** `spec` 全体置換として設定を更新します。既定遅延の変更は既に受理したメッセージを書き換えません。保持期間は受理時刻を起点にし、現在の設定を未配信メッセージに適用します。すでに期限切れとして破棄されたメッセージは復元しません。Hostは外部状態とのずれを同じResource UIDで収束させます。
- **削除:** キューとそのメッセージをこのResourceの所有範囲として削除します。Queueを `queue` または `deadLetterQueue` として参照するQueueConsumer、または `queueProducerBindings` からこのUIDを参照する未削除WorkerVersionがあれば `dependency_conflict` として削除を拒否します。未選択Versionも参照を保持するため削除を妨げます。ConsumerやWorkerVersionを暗黙に削除しません。キューとメッセージが既に無いことを確認できた場合は削除完了です。

送信の結果が不明なWorkerは、同じ論理送信を新しいBinding呼出しとして繰り返すのではなく、Promiseの結果とアプリケーション固有の重複排除を用います。Host APIのIdempotency-KeyはCRUD Operationの再送を識別し、producer Bindingの `send` を一般的なexactly-once送信へ変換しません。Host再起動後もキューの受理記録と配信義務を復元し、未確定のメッセージを破棄済み・未受理と推定しません。外部キューの結果を照合できないときは、未知の効果として停止・報告します。

最大保持期間経過後の未配信メッセージは破棄されます。配信不能、保存上限、基盤障害はHostが明示する失敗です。部分変更があれば `effect:partial`、副作用が無いと確定した場合にだけ `effect:none` を返します。応答待ち時間超過や再起動だけを理由に `none` としてはいけません。既知の部分失敗または孤児状態はResourceに残し、同じUIDへの明示的な更新または削除で収束・後始末できる必要があります。未知の結果中に別Operationで同じキューを作成しません。

## 参照、所有、削除

このFormはキューとその格納メッセージを所有します。[QueueConsumer 0.3.0](https://edge.forms.takoform.com/forms/QueueConsumer/0.3.0/)はキューを所有せず、参照は `{ "resourceUid": "…" }` です。参照先は同じHost、同じSpaceにあり、Form URLがこの文書の `formUrl` と完全一致するResourceでなければなりません。WorkerVersion 0.5.0からのproducer参照も同じHost/Spaceと正確なForm URLを満たし、参照を使う主体にQueueへの権限が必要です。名前からUIDを再解決したり、別Host/SpaceのResourceを参照したりしてはいけません。参照は権限を与えず、Hostは参照先と各JavaScript Binding呼出しを認可します。

削除はキューResourceとそのメッセージだけに及びます。Queueを `queue` または `deadLetterQueue` として参照するQueueConsumerが一つでも残る場合、またはこのQueueを `queueProducerBindings` から参照する未削除WorkerVersionが一つでも残る場合は `dependency_conflict` で拒否します。未稼働・非選択Versionも参照を保持するため削除を妨げます。Consumer、WorkerVersion、他のキュー、アプリケーションデータは連鎖削除しません。明示的にすべての参照元を削除してからQueueを削除します。

## 未知のフィールド

`spec` とproducer Bindingの各入力オブジェクトは閉じた形です。未知のフィールドは拒否し、無視・保持・転送してはいけません。将来意味を増やす場合は新しいForm URLで契約を定義します。`send` の `options` および `sendBatch` の各message objectに未知のフィールドを渡してはいけません。

### WorkerVersionのproducer Binding

[WorkerVersion 0.5.0](https://edge.forms.takoform.com/forms/WorkerVersion/0.5.0/)の `queueProducerBindings` は省略時 `[]`、最大64要素です。各要素は `{"name":string,"resource":{"resourceUid":string}}` で、`resource` は同じHost/Space上のこのForm URLのQueueを指します。`name` は1〜64文字のJavaScript識別子（`^[A-Za-z_$][A-Za-z0-9_$]{0,63}$`）で、各配列内に重複できず、同じWorkerVersionの他Binding名・`vars`・秘密変数名とも衝突できません。`name` から環境変数 `env[name]` を作ります。

このBindingは `send` と `sendBatch` だけを持ち、Queueからの受信、peek、settlement操作は提供しません。`send(body, options?)` の `body` は `ArrayBuffer`、`ArrayBufferView`、またはUTF-8文字列、`options` は任意の `{delaySeconds?: integer}` です。`send` はPromiseで1〜256文字のHost発行message IDを返します。`sendBatch(messages)` は1〜100個の `{body, delaySeconds?}` を順序付きで原子的に送信し、入力順に対応する1〜100個の1〜256文字のmessage ID配列を返します。文字列本文はUTF-8のbyte列として扱います。各送信の `delaySeconds` は整数 `0..43200` で、省略時はResourceの `deliveryDelaySeconds` を使います。各本文のbyte長は最大127000です。送信成功はHostが受理・永続化したことを示し、配信や処理完了を示しません。

```json
{
  "queueProducerBindings": [
    {
      "name": "TASKS",
      "resource": { "resourceUid": "queue-uid" }
    }
  ]
}
```

JavaScript呼出しの失敗はPromise rejectionで、`Error.name` は `invalid_body`、`message_too_large`、`batch_too_large`、`backend_unavailable` のいずれかです。これはHost API v2のResource管理HTTP status、Problem Details、Operation状態とは別のエラー境界です。曖昧な結果で送信を繰り返すと重複し得るため、利用者はmessage IDと業務上の重複排除を用います。

Queueの削除は、参照が残る間だけでなく、削除対象外の未削除WorkerVersionが `queueProducerBindings` に参照を持つ間も拒否されます。WorkerVersionがまだ `queueProducerBindings` を実行していなくても同じです。参照元Versionを明示的に削除した後にQueueを削除します。
