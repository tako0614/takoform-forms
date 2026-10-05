---
title: ActorNamespace 0.3.0
description: 単一writerのActor ID空間、private SQL、alarmとHost所有socket
formUrl: https://edge.forms.takoform.com/forms/ActorNamespace/0.3.0/
hostApi: forms.takoform.com/v2
---

# ActorNamespace 0.3.0

このFormは、一つの[ModuleWorker 0.3.0](../../ModuleWorker/0.3.0/)がexportする
Actor classのID空間を管理する。Actorの個別ID・store・alarm・socketはこのResourceの
実行dataであり、個別のTakoform Resourceではない。同じnamespace UIDとActor IDへの
eventは逐次実行する。HostがこのFormに対応すると主張するには、この章のclass ABI、永続化、
隔離、socketまで一体として満たす必要がある。`forms.takoform.com/v2`の仕様公開はHost対応、
容量、稼働を意味しない。

## 入力、参照、CRUD

`spec`には`worker:{"resourceUid":string}`と`className:string`を必須とする。
`className`は`^[A-Za-z_$][A-Za-z0-9_$]{0,63}$`で、main ES moduleのnamed export名である。
既定値も秘密入力もない。`spec`と参照objectの未知キー、`null`を拒否する。HostはWorkerが
同じHost・Spaceに属し、呼出者に利用権限があり、Form URLがModuleWorker 0.3.0に完全一致
することを副作用前に確認する。同じWorker UIDとclassNameのnamespaceは一つだけである。
コードはWorkerの現在有効な[WorkerDeployment 0.4.0](../../WorkerDeployment/0.4.0/)が
選ぶ全weighted [WorkerVersion 0.5.0](../../WorkerVersion/0.5.0/)から得る。
Namespaceを先に作ることもできるため、Deploymentがまだない間はReadyにしない。

`spec`の例（UIDは説明用）:

```json
{
  "worker": { "resourceUid": "worker-uid" },
  "className": "CounterActor"
}
```

未観測の`observed`は `{}`。確認済みなら`ready:boolean`、`activeActorCount:integer`、
`pendingAlarmCount:integer`、`openSocketCount:integer`を返す。計数は`observedAt`時点のもので、
GETでの常時同期を要求しない。`ready:true`は全weighted Versionにclass ABIがあり、
同じIDの逐次実行と永続store/alarm/socketをHostが提供できる状態を意味し、個々の
handler成功を保証しない。`output`は `{}`。

作成はUIDと空のID空間を確保する。取得は管理記録と最後の観測を返す。PUTは同じ
`worker`と`className`の全体置換だけを受け付け、異なる値は副作用前に拒否する。
同一specの新しいキーでも新しいOperation/generationを持ち再照合する。classの変更は新しい
namespace UIDが必要で、既存Actor dataを暗黙移行しない。削除はこのnamespaceの新規配送を
閉じ、epochを進め、既存実行・stream・upgrade予約をcancelし、child実行の退役と権威ある
不在確認が済むまで成功にしない。Socketはbest effortで1001 closeし、store、alarm、
attachment、ID空間を**このnamespace UIDに限って**除去する。「生きたActor Binding」とは
削除されていないWorkerVersionの`actorBindings`に残る参照であり、そのVersionがweightedか
どうかを問わない。これがある間は`409 dependency_conflict`で副作用前に拒否する。
休眠socketやalarmは古いVersionの削除を妨げず、このNamespaceのHost管理dataとして残る。
有効Deploymentがなくなれば新しいsocket callback/alarm eventを配送しない。他のWorkerや
外部Resourceは削除しない。
削除後に同名で作り直しても新UID・新epoch・空dataであり旧IDの配送を復活させない。

## Actor classと逐次実行

各weighted Versionのmain moduleは、`className`のconstructibleなnamed exportを持つ。
`new Export(context, env)`でインスタンス化する。prototype（継承したapplication methodを含む）
には`fetch(request, turn)`、`alarm(turn)`、`socketMessage(socket,data,turn)`、
`socketClose(socket,event,turn)`、`socketError(socket,event,turn)`がcallableで必要、
`start(turn)`だけ任意である。accessorやインスタンス毎のmethod差替えでは満たさない。
呼出し形と返却値は次のとおり。`socket`は後述のActorSocketであり、dataは文字列または
`Uint8Array`、close eventの三フィールドは必須、error eventのcodeは固定値である。

```typescript
interface ActorInstance {
  start?(turn: {signal: AbortSignal}): void | Promise<void>;
  fetch(request: Request, turn: {signal: AbortSignal}): Response | Promise<Response>;
  alarm(turn: {signal: AbortSignal}): void | Promise<void>;
  socketMessage(socket: ActorSocket, data: string | Uint8Array,
    turn: {signal: AbortSignal}): void | Promise<void>;
  socketClose(socket: ActorSocket,
    event: {code: number; reason: string; wasClean: boolean},
    turn: {signal: AbortSignal}): void | Promise<void>;
  socketError(socket: ActorSocket, event: {code: "transport_error"},
    turn: {signal: AbortSignal}): void | Promise<void>;
}
```

constructorは同期でcontext/envを保存するにとどめ、非同期の副作用を開始しない。
`start`は新しい実行contextの配送前にawaitし、eviction後には再び呼ばれるのでidempotent
でなければならない。inspectionでアプリコードの副作用を起こさない。

`env`は選択されたVersionの[Worker runtime](../../ModuleWorker/0.3.0/#runtime)に定める
宣言済み環境だけで、Hostの管理credentialを含めない。`turn.signal`はAbortSignalである。
同じnamespace UID/Actor IDには同時に高々一つのlive execution contextを置き、一つの
eventを完了・中断・退役させてから次を開始する。異なるIDは並行できる。Versionはeventごとに
選び、当該eventには固定する。次のeventが別VersionでもSQL/alarm/socket identityは残る。
永続stateのschema互換性は作者が担い、Hostはrollbackやevictionで勝手に初期化しない。
HTTP response headを送ってもActorが所有するbody producerが残る間、同じIDの次eventは
待つ。caller取消やdeadlineではstream・予約をcancelし、childの退役を確認してから次を通す。
Promise rejectionやDB fenceだけを退役の証明としない。constructor/startのwall期限と
eventのactive CPU上限は各30秒。接続中HTTPには独立したwall期限を設けないが、
alarm/message/close/error callbackのwall上限は15分とする。開始後の未捕捉例外は秘密を
出さないHost生成HTTP 500、開始不能は`backend_unavailable`である。HTTP head前の期限切れは
504、head後のbody中断は`response_aborted`、caller取消は`request_aborted`で区別する。

## Caller Binding {#binding}

WorkerVersionの`actorBindings[].resource`はこのNamespace UIDを指す。`env.NAME`は
`idFromName(name)`、`newUniqueId()`、`get(id)`を提供する。前二者と`get`は同期で、
`get`はstubを返しHost往復をしない。`name`は1〜2048 Unicodeスカラー値、返すIDは
非空・最大256文字のopaque文字列。同じnameは同じIDになり、`newUniqueId`のIDはnameから
導けない。IDをparseしたり、name再作成をdata所有証明に使ってはならない。
`stub.fetch(request:Request)`（またはURL文字列とinit）は`Promise<Response>`で、
入出力bodyをstreamする。最初の配送がActorの実体化であり、`create`/`exists`はない。
Actorの未捕捉例外で生成した500はresolveしたResponseであり、開始不能のみ
`backend_unavailable`でrejectする。`idFromName`の不正名は`invalid_name`。
別workerへのservice転送でHostのupgrade権限を伝播させない。

## private SQLとalarm

`context.id`は現在のopaque Actor ID。`context.storage`はこのnamespace UID/ID専用の
SQLite storeで、同じIDの逐次実行により単一writerとなる。`execute(sql,params?)`は
一文を実行し`{rows,rowsWritten}`、`query(sql,params?)`は常にrollbackされる読取で
`rowsWritten:0`、`transaction(statements)`は1〜100文をserializableに一括実行して
`{results}`を返す。transactionは全結果をmaterializeしてからcommitし、all-or-noneである。
schema DDLもActor自身のstoreに限って許す。SQLは1〜100000文字、paramsは最大100値。
値はnull、有限の安全整数範囲の数、最大1000000文字の文字列、または
`{"encoding":"base64","data":string}`のbytes表現である。結果は同じ値域に限り、
未対応SQL・制約・数値・結果サイズ・容量・競合・backendの失敗をそれぞれ
`invalid_sql`、`constraint_violation`、`numeric_out_of_range`、`result_too_large`、
`storage_full`、`busy`、`backend_unavailable`として識別する。transactionは部分commitしない。

`context.alarm.set(atMillis)`はUnixミリ秒の非負整数で唯一の次回alarmを置換し、過去時刻は
可能になり次第配送する。`get()`は時刻または`null`、`clear()`は既に無くても成功する。
配送中alarmと次回alarmは別に記録し、失敗した配送を消さない。`set`/`clear`は次回分のみ
変更する。Alarmとstoreはevictionを越えて残る。Actor eventに`waitUntil`や切り離した
非同期作業を持ち込まず、完了境界より後に同じIDの書込みを残さない。

## Host所有WebSocket {#sockets}

`context.sockets.accept(request,{protocol?,attachment?})`は元のclient HTTP
WebSocket-upgrade要求からこのActorへ直接届いたinvocationだけに許す。`protocol`は元要求が
提示した単一tokenまたは省略で、他を`invalid_upgrade`とする。acceptは
`{response,socket}`を返すが、responseはHostがbrandした`status:101`、`body:null`の
一回限りの予約である。元invocationの外へ転送・直列化できない。
`new Response(upgrade.body,upgrade)`だけ同じ予約のaliasとなり、cloneはTypeError、
brandなし101はRangeError、予約済みhandshake headerを矛盾して上書きしたouter応答は
101送信前に502となる。outerが予約を返さない、例外、head送信失敗では予約と暫定送信を
破棄する。予約だけの独立timerはなく、元の接続とabortに従う。

Socketは`id`、`send(string|Uint8Array)`、`close(code?,reason?)`、
`getAttachment()`、`setAttachment(bytes|null)`を持つ。`get(id)`はlive/closingのsocketまたは
`null`、`list()`はそれらをID辞書順で返す。暫定socketはaccept結果以外に見せない。
同じIDのcallbackは逐次で、終端callbackはcloseかerrorの高々一つ。終端後はget/listから
消し、send/close/setAttachmentは`socket_closed`、getAttachmentはsettlementまで使える。
socket IDはlive中一意で再利用せず、eviction/Version変更を越えて保持し、再接続は新ID。
attachmentはcopyで最大16384 bytes、`null`で消去し、oversizeは旧値を保ったまま
`attachment_too_large`。接続上限はIDごと10000、frameは各方向33554432 bytes、
outbound queueは33554432 bytes。sendはbrokerのlocal受理で戻り、peer着信は保証しない。
oversizeは`message_too_large`、queue満杯は`transport_overloaded`で部分frameを送らない。
closeの既定値はcode 1000、reason空文字。application codeは1000または3000〜4999、
reasonはUTF-8で123 bytes以下とし、無効なら`invalid_close`。Hostはoversize inboundを
1009、callback失敗/期限を1011、計画再起動を1012、削除を1001で閉じる。
broker/process喪失時には終端callbackが届かない場合があり、closeをdata cleanupと
見なさない。Hostは永続ID/ackとadmissionを持ち、重複live contextを防ぐ。

本Form URLに未知の入力を追加せず、ABIや削除意味を変える拡張は新URLとする。
