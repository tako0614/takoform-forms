---
title: QueueConsumer
description: 一つのAtLeastOnceQueueをModuleWorkerへ接続する消費Attachment。
formUrl: https://edge.forms.takoform.com/forms/QueueConsumer/0.3.0/
hostApi: forms.takoform.com/v2
---

# QueueConsumer 0.3.0

このFormは、一つのAtLeastOnceQueueを一つのModuleWorkerへ接続する消費Attachmentを表します。キューへの消費はBindingではなく内向きの起動関係です。Host API v2のResourceとして管理し、共通の作成・取得・更新・削除と再送規則は[Takoform Host API v2 HTTP API](https://takoform.com/spec/host-api/v2/http)に従います。ModuleWorkerのruntime ABIは[ModuleWorker 0.3.0 §runtime](https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/#runtime)に従います。

## 入力

`spec` は次のフィールドだけを受け付けます。`queue` と `worker` はAttachment UIDの間不変です。変更する場合は旧Attachmentを削除し、新しい名前または解放済みの名前で作り直します。その他の設定は更新できます。

| フィールド | 要件、既定値、範囲 | 作成後の変更 |
| --- | --- | --- |
| `queue` | 必須の `{ "resourceUid": "…" }`。AtLeastOnceQueue 0.2.0のResourceを参照します。既定値なし。 | 不変。 |
| `worker` | 必須の `{ "resourceUid": "…" }`。ModuleWorker 0.3.0のResourceを参照し、`queue` handlerを提供します。既定値なし。 | 不変。 |
| `maxBatchSize` | 必須整数 `1..100`。一回に渡す最大メッセージ数。既定値なし。 | 変更できます。 |
| `maxBatchTimeoutSeconds` | 必須整数 `0..60`。バッチを満たすために待つ最大秒数。`0` は待たない設定です。既定値なし。 | 変更できます。 |
| `maxConcurrency` | 必須整数 `1..250`。同時に実行するバッチ数の上限。既定値なし。 | 変更できます。 |
| `maxRetries` | 必須整数 `0..100`。初回配信後の再配信上限。既定値なし。 | 変更できます。 |
| `retryDelaySeconds` | 必須整数 `0..43200`。失敗したメッセージを再配信可能にするまでの既定秒数。既定値なし。 | 変更できます。 |
| `deadLetterQueue` | 任意の `{ "resourceUid": "…" }`。AtLeastOnceQueue 0.2.0を参照します。省略時は再試行を使い切ったメッセージを破棄します。 | 変更・省略できます。 |

```json
{
  "queue": { "resourceUid": "queue-uid" },
  "worker": { "resourceUid": "worker-uid" },
  "maxBatchSize": 10,
  "maxBatchTimeoutSeconds": 5,
  "maxConcurrency": 4,
  "maxRetries": 3,
  "retryDelaySeconds": 30
}
```

秘密入力はありません。空でない `privateInputs` は拒否します。未知のspecフィールドは拒否します。

## 配信とInterface操作

参照は `resourceUid` に固定されます。各参照先は同じHost、同じSpaceにあり、Form URLが上記の正確なForm URLと完全一致しなければなりません。`queue` と `worker` は必須です。`deadLetterQueue` を指定した場合はそれも同じ条件で検証します。参照は認可を付与しません。

参照WorkerはModuleWorker 0.3.0が定める `queue` handlerを宣言・提供しなければなりません。HostはABI所定のバッチを渡します。batchは1〜`maxBatchSize`個の順序付きmessageを持ちます。各messageには1〜256文字の安定した不透明ID、UTC Unix epochミリ秒の受理時刻、最大127000 byteの元の不透明バイト列、初回1から始まる整数の試行回数があります。batchには1〜256文字の識別子とQueue Resource名が含まれます。呼出しはat-least-onceで、同じメッセージが複数回・異なる順序で届き得ます。handlerは副作用を重複実行しても安全にする必要があります。

ModuleWorkerの `queue` handlerが受け取るbatchとsettlement methodは[ModuleWorker 0.3.0 §runtime](https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/#runtime)で定義します。各methodはHostのResource管理HTTP APIではなく、配信中messageのsettlementです。handlerにはbatch単位のobjectが渡され、各methodはそのbatchへ束縛されるため、JavaScript呼出しにbatch IDは渡しません。

| JavaScript操作 | 必須引数と任意引数 | 成功結果 | `Error.name`による拒否 |
| --- | --- | --- | --- |
| `batch.acknowledge(messageId)` | このbatch内のmessage ID（1〜256文字） | Promiseが`undefined`で解決 | `unknown_batch`, `unknown_message`, `already_settled`, `backend_unavailable` |
| `batch.retry(messageId, delaySeconds?)` | batch内ID；任意の整数遅延 `0..43200` | Promiseが`undefined`で解決 | 同上 |
| `batch.acknowledgeAll()` | なし | Promiseが`undefined`で解決 | `unknown_batch`, `backend_unavailable` |
| `batch.retryAll(delaySeconds?)` | 任意の整数遅延 `0..43200` | Promiseが`undefined`で解決 | `unknown_batch`, `backend_unavailable` |

`batch.acknowledge` は一通を配信済みに確定し、二度目のsettlementを `already_settled` で拒否します。`batch.retry` は一通を再配信待ちへ戻し、指定遅延がなければ `retryDelaySeconds` を使います。`batch.acknowledgeAll` は未settledの全通を配信済みにし、`batch.retryAll` は未settledの全通を再配信待ちにします。バッチ外IDを受理せず、settle済みメッセージの結果を反転しません。これらのJavaScript Promise拒否はInterfaceのデータ操作失敗であり、Host API v2のHTTP `Problem Details` やResource `Operation.error` ではありません。

handlerが例外なく戻った場合、未settled分をacknowledgeします。handlerがthrowまたは返却Promiseがrejectした場合、既にsettleされた結果は維持し、残る未settled分をretryします。`maxRetries` は再配信回数であり初回を数えません。最大配信回数は `1 + maxRetries` です。上限到達後、`deadLetterQueue` があればそのキューへ新しいメッセージとして移し、なければ破棄します。移動先のメッセージIDと受理時刻は新しくなり、attemptsは1に戻ります。元メッセージとの対応を表す移植可能な情報はありません。

queue参照ごとに有効なConsumerは最大1つです。二つ目、存在しない/削除中/型違い/他SpaceのResource、Workerが必要なhandlerを持たない参照は副作用前に拒否します。dead-letterグラフに自己参照または長さを問わない循環が生じる更新・作成も拒否します。dead-letter転送でattemptsがリセットされ、メッセージが循環することを防ぎます。

## 観測、出力、利用可能状態

`observed` は初回観測前は `{}` です。確認後は `queueExists`、`workerExists`、`consumerAttached` の真偽値を含み、各値はHostが最後に確認した状態を示します。確認できない項目は省略し、`false` は確認済みの不在・未接続だけに使います。形の例:

```json
{
  "queueExists": true,
  "workerExists": true,
  "consumerAttached": true
}
```

このFormの `output` は常に `{}` です。Attachmentの作成・更新成功は設定が受理されたことを意味し、Worker handlerが成功したこと、キューが空であること、アプリケーションが健全であることを意味しません。

## CRUD、拒否、障害復旧

- **作成:** Host API v2の共通検証の後、両参照、必須handler、キューごとの単一Consumer制約、dead-letter循環、主体の参照権限を副作用前に検査します。制約を満たさない場合はAttachmentや配信設定を変更しません。
- **取得:** AttachmentのResourceと最後に確定した観測を返します。読み取りはメッセージ配信を開始しません。
- **更新:** `spec` を全体置換します。`queue` と `worker` の変更は `invalid_spec` として副作用前に拒否します。他の設定変更も、制約検証が済むまで適用しません。Consumer有効化の変更結果が不明なら別Consumerを追加せず照合します。
- **削除:** このAttachmentだけを解除します。Queue、Worker、dead-letter Queueを削除せず、未処理メッセージを別Queueへ移しません。Hostは停止境界を確定してからAttachmentを削除し、進行中のhandlerや未settledメッセージを成功扱いしません。

未settledメッセージはACKが確定するまで再配信義務として残ります。Host再起動後もsettlement記録と未settled状態を回復します。ACKまたはretry要求の結果が不明なら、同じbatch/message識別子を照合し、settlementが確認できない間は未知として扱います。処理済みと確定しない限りメッセージを消失扱いにしてはいけません。再起動やtimeoutでACKがあったと推測してはいけません。handler再実行による重複はこのFormのat-least-once契約の一部です。

v2 Operationで作成・更新・削除を再送するとき、同じIdempotency-Keyは同じOperationを返します。外部設定の反映が一部だけ成功した場合は `effect:partial`、反映結果を確定できない場合は `effect:unknown` としてOperationを `reconciling` に保ちます。応答待ち時間超過だけで失敗・無効果と断定しません。HostはOperationを再起動後に復元し、未完了中に新しいOperationで別Attachmentを作りません。既知の部分失敗は同一Resource UIDへの明示的PUTまたはDELETEで収束・解除できる必要があります。

削除時、キューに未settledメッセージがあってもAttachment解除はそれらをACKしません。進行中の配送が終了または安全に再配信可能となるまでHostは削除を完了扱いしません。Hostがその安全境界を証明できなければOperationはunknown/reconcilingのままです。

## 所有、参照、削除の範囲

このResourceが所有するのはQueueConsumer Attachmentだけです。Queue、Worker、dead-letter Queueは `{ "resourceUid": "…" }` で参照します。参照先は同じHost/Spaceに属し、それぞれのForm URLが `https://edge.forms.takoform.com/forms/AtLeastOnceQueue/0.2.0/` または `https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/` に完全一致しなければなりません。dead-letterQueueはQueueConsumer自身が排出するQueueへ戻らず、全dead-letter参照のグラフに循環を作らない必要があります。Queueの送信操作とメッセージ形式は[AtLeastOnceQueue 0.2.0](https://edge.forms.takoform.com/forms/AtLeastOnceQueue/0.2.0/)に従います。

Attachment削除は接続だけを外し、参照先Resourceやそのデータを連鎖削除しません。参照先Queueの削除はこのAttachmentが残る間拒否されます。参照UIDは名前変更・再作成後も別Resourceへ付け替えません。参照の存在から利用権限を得ることはありません。

## 未知のフィールド

`spec`、参照オブジェクト、Interface呼出しオブジェクトは閉じた形です。未知フィールドは拒否し、無視・保存・転送してはいけません。参照オブジェクトは `resourceUid` 一つだけを持ちます。
