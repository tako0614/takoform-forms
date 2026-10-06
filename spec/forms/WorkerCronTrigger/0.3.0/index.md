---
title: WorkerCronTrigger
description: UTC cron scheduleをModuleWorkerへ接続するAttachment。
formUrl: https://edge.forms.takoform.com/forms/WorkerCronTrigger/0.3.0/
hostApi: forms.takoform.com/v2
---

# WorkerCronTrigger 0.3.0

このFormは、一つのModuleWorkerの `scheduled` handlerへUTC cron scheduleを結び付けるAttachmentです。Host API v2のResourceとして管理し、共通のCRUD・再送規則は[Takoform Host API v2 HTTP API](https://takoform.com/spec/host-api/v2/http)に従います。ModuleWorker 0.3.0の[runtime ABI](https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/#runtime)が定めるhandlerだけを対象とし、独自の実行コードや特定クラウドの設定を含みません。

## 入力

`spec` は閉じたオブジェクトです。

| フィールド | 要件、既定値、範囲 | 作成後の変更 |
| --- | --- | --- |
| `worker` | 必須の `{ "resourceUid": "…" }`。ModuleWorker 0.3.0 Resourceを参照し、`scheduled` handlerを提供します。既定値なし。 | Attachment UIDの間不変。変更には削除と新規作成が必要です。 |
| `cron` | 必須文字列。ASCIIの5フィールドを一つの空白で区切り、最長64文字。既定値なし。書式は後述。 | 変更できます。新しい式の将来の一致だけに適用し、既に発火した実行を取り消しません。 |

```json
{
  "worker": { "resourceUid": "worker-uid" },
  "cron": "*/5 * * * *"
}
```

秘密入力はありません。空でない `privateInputs` は拒否します。

## cron文法と実行

cronは次の5フィールドです: `分 時 日 月 曜日`。時刻基準はUTCのみで、timezone欄はありません。分は `0..59`、時は `0..23`、日 `1..31`、月 `1..12`、曜日 `0..6`（`0` は日曜日）です。

各フィールドはカンマ区切りの一つ以上の要素です。要素は `*`、範囲内の整数、`low-high` の範囲、`*/step`、または `low-high/step` です。名前、空要素、重複カンマ、裸の整数への `/step`、逆転範囲、フィールド範囲外の値、`1` からフィールド幅までの範囲外のstepを拒否します。stepは範囲の下限、または `*` の場合はそのフィールドの最小値から刻みます。月・曜日名を数値へ暗黙変換しません。実在しない日付（例: 2月31日）は一致する暦日がないため発火しません。

日と曜日の両方が `*` 以外で制限されている場合は、日が一致する**または**曜日が一致する日に発火します。一方だけが制限される場合はその制限だけを適用します。予定時刻にHostが利用不能、または前回実行が重なっていてその一致を一度も記録できなかった場合はスキップし、遅れて補填したり未実行分を蓄積したりしません。Hostが一致を記録した後は、その一致のhandlerを少なくとも一回呼ぶ義務があります。夏時間切替による重複・欠落はなく、同じ式の同じUTC一致時刻はHost間で同じです。

一致ごとにat-least-onceでhandlerを呼びます。同じ一致に複数回呼び出される場合があるためhandlerは冪等でなければなりません。`(cron, scheduledTime)` は一致を識別する組であり、同じ組を伴う複数呼出しは再配信です。イベントはABIで定義する一致時刻 `scheduledTime`（UTC Unix epochミリ秒）と、実際に一致した `cron` 文字列を含みます。handlerのthrowまたはPromise rejectionは失敗した実行としてHost診断に記録し、HTTP応答に変換しません。その失敗だけを理由に同じ一致分の中で再試行しません。ABIの `waitUntil` による処理失敗は診断対象であり、外部効果を巻き戻しません。

## 観測、出力、利用可能状態

`observed` は初回観測前は `{}` です。確認後は `cron`（有効化を確認した式）、`timezone`（常に `UTC`）、`scheduleReady`（有効なscheduleの登録を確認した真偽値）を含みます。次回一致を計算できた場合のみ `nextMatchAt`（RFC 3339 UTC文字列）も含みます。計算できない項目や未確認項目は省略し、`false` は確認済みの未準備だけを表します。値はResourceの `observedAt` 時点の最後の確認です。次の一致はHost稼働やhandler実行の保証ではありません。形の例:

```json
{
  "cron": "*/5 * * * *",
  "timezone": "UTC",
  "scheduleReady": true,
  "nextMatchAt": "2026-10-05T12:05:00Z"
}
```

このFormの `output` は `{}` です。Attachmentが `idle` またはOperation成功であっても、handler成功、外部サービスの健全性、アプリケーションの利用可能状態を意味しません。

## CRUD、拒否、再起動

- **作成:** v2共通の認可とResource検証に加え、Worker参照、正確なForm URL、同じHost/Space、`scheduled` handlerの宣言と提供、cron文法および各数値範囲を確認します。どれかを満たさなければ副作用前に拒否します。
- **取得:** Attachmentと最後に確認した観測を返します。GETはcron発火・handler実行を起こしません。
- **更新:** `spec` 全体置換です。Worker UIDの変更は副作用前に拒否します。新cron式の検証後、Hostは以後のUTC一致をその式に収束させます。旧式と新式の両方を長期間同時に発火させません。
- **削除:** このschedule Attachmentのみを解除します。Worker Resourceや他のscheduleを削除しません。解除前後の一致境界をHostが永続化し、再起動後に同じ一致を新しい別runとして誤って二重登録したり、すでに確定したrunを忘れたりしません。

Hostは各一致の安定した識別と起動記録を永続化します。クラッシュ後も、未配信一致を少なくとも一回届ける義務を維持します。Hostはhandler実行の結果が不明な状態で無効果と断定せず、再起動後の重複を許容してat-least-once特性を維持します。失敗または応答喪失のOperationは `effect:none`、`partial`、`unknown` を実際に確定できた範囲で返します。外部起動の結果が不明なら新しいOperationで重ねず、元のOperationと発火記録を照合します。既知の一部適用・孤児scheduleは同じUIDの更新または削除で収束させます。

作成・更新・削除の結果不明を応答待ち時間超過やHost再起動だけで成功・失敗と決めません。未完了Operationは再起動後に復元し、同じIdempotency-Keyの再送には同じOperationを返します。handlerの外部効果を取り消せない場合はそれを隠さず、再実行による重複の可能性を維持します。アプリケーションのexactly-once実行は保証しません。

## 所有、参照、削除の範囲

このResourceが所有するのは一つのschedule Attachmentです。Worker参照は `{ "resourceUid": "…" }` だけで表し、同じHost・Spaceにある `https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/` のResource UIDに固定します。Worker削除は参照が残る間拒否されます。参照は認可を付与せず、参照先を名前で再解決しません。発火イベントとhandlerの戻り値の形は[ModuleWorker 0.3.0 §runtime](https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/#runtime)が定めます。

削除によってscheduleだけを外し、Worker、アプリケーション、他のAttachment、過去のhandler効果を連鎖削除・取消ししません。過去の一致で進行中のhandlerは、Hostが安全に停止を保証できない限り既に作用している可能性を保持します。

## 未知のフィールド

`spec` とWorker参照オブジェクトは閉じた形です。未知フィールドは拒否します。参照は `resourceUid` のみを受け付け、`apiVersion`、`kind`、`name` による曖昧な再解決は行いません。
