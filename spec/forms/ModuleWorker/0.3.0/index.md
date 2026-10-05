---
title: ModuleWorker 0.3.0
description: JavaScript module Worker の永続的な同一性と実行 ABI
formUrl: https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/
hostApi: forms.takoform.com/v2
---

# ModuleWorker 0.3.0

このFormは、一つのJavaScript module Workerの同一性を表す。コード、変数、Bindingは
[WorkerVersion 0.5.0](../../WorkerVersion/0.5.0/)に置き、稼働中の版とHTTPトラフィックの配分は
[WorkerDeployment 0.4.0](../../WorkerDeployment/0.4.0/)が決める。Workerを作っただけでは
実行コードも公開URLも生じない。HTTP endpoint、cron、queue consumerは別のFormによる添付である。
このResourceのCRUD、認可と管理Operationの再試行は
[Takoform Host API v2](https://takoform.com/spec/host-api/v2/http)に従う。
実行時のfetch、queue、Actor、Workflowは別の通信・処理であり、管理用の
`Idempotency-Key`やOperation IDを引数に取らない。各実行操作の再試行は本Formと
参照先FormのInterface章が定める。
この版のForm URLは上記の完全な文字列で識別する。本文は仕様であって、いずれかのHostの
対応・稼働・公開を主張しない。

## 入力、状態、操作

`spec`は必須の空オブジェクト `{}` だけである。既定値も秘密入力もない。未知のキー、
`null`、配列は拒否する。GETの`spec`も `{}` である。`observed`は未確認なら `{}`、確認済みなら
`activeDeploymentUid`（有効なDeploymentがなければ`null`）と`ready`（真偽値）を返す。
`ready:true`は、選ばれたDeploymentとその版がこのFormの実行条件を満たし、Hostが新しい
イベントを受け付けられる状態を意味する。これは特定のHTTP要求の成功や外部到達性を保証しない。
`observedAt`と`observedGeneration`は共通APIの時点を示す。`output`は `{}` である。

- 作成は空のWorker同一性を確保する。作成成功はコードの評価やURLの割当を意味しない。
- 取得は管理記録と最後の観測を返す。GETごとの実行先再照会は要求しない。
- 更新は同じ空specの全体置換のみ受け付ける。新しいIdempotency-Keyによる同一specのPUTも
  新しいOperationとgenerationを持つ。実行コードは更新せず、別のWorkerVersionとDeploymentで変える。
- 削除はそのWorkerへの生きたVersion、Deployment、ActorNamespace、DurableWorkflow、
  service Binding、endpoint等の参照がある間、変更前に`409 dependency_conflict`で拒否する。
  参照がなくても未完了実行を停止・隔離できない間は完了にしない。成功はこのWorkerの同一性だけを
  解放する。Binding先、外部サービス、他のWorker、独立した永続データを連鎖削除しない。

## 実行Interface: module Worker {#runtime}

HostがこのFormを対応表に載せるには、以下のABIを一体として実装する。WorkerVersionの
`bundle`が指す[WorkerBundle 0.2.0](../../WorkerBundle/0.2.0/)のmain ES moduleを読み、
その`default` exportはplain object、`fetch`、`scheduled`、`queue`は宣言されたものだけが
callableなown propertyでなければならない。Versionが宣言していないhandlerや未exportの
handlerを推測して呼ばない。import可能なmedia typeは
`application/javascript+module`、`text/plain`（UTF-8文字列）、
`application/octet-stream`（ArrayBuffer）、`application/wasm`（コンパイル済みModule）である。
source mapは補助物でありimport対象にしない。JSON moduleはこのABIにない。

### Moduleの解決 {#modules}

WorkerBundleのmanifestはbundle rootからの正規化されたPOSIX相対pathを列挙し、
`entrypoint`はそのうちJavaScript ES module一つを正確に指す。Hostはimport先を
**manifest内の完全なpath一致**だけで探す。`./`と`../`で始まるspecifierはimport元の
directoryからPOSIX segmentとして解決し、先頭`/`はbundle rootから解決する。
`..`でroot外へ出るもの、空segment、backslash、query `?`、fragment `#`、bare
specifier（`node:`を含む）、HTTP(S)/data URL、import mapは認めない。percent escapeを
URLとしてdecodeせず、文字列をそのままpathと比較する。`.js`の補完やdirectoryの
`index`推測、大文字小文字の正規化、外部取得はしない。相対specifierを正規化した結果が
manifestの一つのimportable entryに一致しない場合は`module_not_found`、auxiliary
source mapを指せば`unsupported_media_type`である。

static import/export-fromと`import(specifier)`は同じ解決規則を使う。staticな不一致・
syntax errorはVersionをReadyにする前に拒否する。dynamic importの値は実行時に検証し、
不一致ならそのPromiseを該当エラーでrejectする。循環するES module graphはECMAScriptの
live binding/評価順に従い、循環だけを理由に拒否しない。Hostはbundle外のmoduleを
暗黙に補わない。`text/plain`は正しいUTF-8からdecodeした文字列、
`application/octet-stream`はそのbytesを持つArrayBuffer、`application/wasm`は
compile済み`WebAssembly.Module`を、それぞれ**default exportのみ**のmoduleとして投影する。
WebAssemblyのinstantiateやimport objectはアプリが選び、Hostは自動実行しない。
不正UTF-8、invalid wasm、JavaScriptの構文・評価失敗はReady検証または実行失敗として
明示し、別media typeへfallbackしない。

呼出し形は`fetch(request: Request, env, ctx): Response | Promise<Response>`、
`scheduled(event, env, ctx): void | Promise<void>`、
`queue(batch, env, ctx): void | Promise<void>`である。`event`はUTCの`scheduledTime`
（Unixミリ秒）と一致した5-field `cron`を持つ。`batch`は`batchId`（1〜256文字）、
queue名（1〜63文字）、1〜100件の順序付きmessage列を持つ。各messageには安定した
`id`（1〜256文字）、受理時の非負整数`timestampMillis`、body bytes、1から始まる整数
`attempts`を含む。JavaScriptのbodyは`Uint8Array`であり、JSONや文字列へ暗黙変換しない。
queueの明示的なack/retryと未settle時の扱いは
[AtLeastOnceQueue 0.2.0](../../AtLeastOnceQueue/0.2.0/)と
[QueueConsumer 0.3.0](../../QueueConsumer/0.3.0/)の章に従う。fetchのrequest/response bodyは
streamとして扱い、Hostはhandler前に全文を文字列化しない。不正な戻り値または未捕捉例外は
機密を含まないHost生成のHTTP 500となる。scheduled/queueの未捕捉例外は失敗として観測し、
HTTP応答へ変換しない。`ctx.waitUntil(promise)`は同じinvocationの後続作業を登録し、
settlementまではisolateを回収しない。settlement済み応答は書き換えず、拒否は診断へ記録する。
失効したctxへの登録は`context_expired`で失敗する。

queue handlerが受け取る`batch`は次のJavaScript形を持つ。`body`は元のmessage bytesを
copyした`Uint8Array`であり、metadataは上記の値域に従う。settlementはこのbatchだけで行い、
`env`のqueue producer Bindingにはこれらのmethodを出さない。

```typescript
interface QueueBatch {
  readonly batchId: string;
  readonly queue: string;
  readonly messages: readonly {
    readonly id: string;
    readonly timestampMillis: number;
    readonly body: Uint8Array;
    readonly attempts: number;
  }[];
  acknowledge(messageId: string): Promise<void>;
  retry(messageId: string, delaySeconds?: number): Promise<void>;
  acknowledgeAll(): Promise<void>;
  retryAll(delaySeconds?: number): Promise<void>;
}
```

`messageId`はこのbatch内のID、`delaySeconds`は省略または0〜43200の整数で、省略時は
Consumerの`retryDelaySeconds`を使う。成功はdurableなsettlement確定を意味する。
失敗は`Error.name`が`unknown_batch`、`unknown_message`、`already_settled`、
`backend_unavailable`のいずれかである。`acknowledgeAll`/`retryAll`はbatch内の未settled分だけを
対象とし、既settledの結果を反転しない。通常returnでは残る未settled分をacknowledgeし、
throw/rejectでは残る未settled分をretryする。重複配送があるためhandlerの外部効果は
idempotentに設計する。これは管理ResourceのOperation/Idempotency-Keyとは別である。

実行環境`env`のown enumerable keyは、Versionに宣言した非秘密`vars`、Hostが別経路で設定した
`requiredSensitiveVars`、型付きBinding名の和に正確に一致する。重複名はVersion保存前に拒否する。
Host固有の追加globalやBindingはportableな契約にならない。最低限のglobalは`Request`、
`Response`、`Headers`、`URL`、`URLSearchParams`、`ReadableStream`、`TextEncoder`、
`TextDecoder`、`AbortController`、`AbortSignal`、`crypto`、`fetch`、`console`と標準timerである。
外向き`fetch`はHostが持つ外部通信権限と制限に従い、BindingやForm URLから権限を得ない。

ActorNamespaceとDurableWorkflowのnamed classはdefault handlerとは別exportである。
そのconstructor、turn、永続状態、step/replayは各Formの
[Actor章](../../ActorNamespace/0.3.0/)と[Workflow章](../../DurableWorkflow/0.3.0/)で定める。
同じWorkerの全weighted Versionは、参照中の全classを提供しなければReadyにならない。
単にWorkerを作成しただけでActor/Workflowの能力があると表示してはならない。

### Workerへのservice Binding {#service-binding}

[WorkerVersion 0.5.0](../../WorkerVersion/0.5.0/)の`serviceBindings[].resource`が
このWorker UIDを参照すると、呼出し元の`env.NAME`は次を提供する。

```typescript
interface WorkerService {
  fetch(request: Request): Promise<Response>;
  fetch(url: string, init?: RequestInit): Promise<Response>;
}
```

このfetchはHost内の**論理的なWorker UID**に届き、相手の現在有効なDeploymentが
選ぶVersionの`fetch` handlerを呼ぶ。public endpoint、DNS名、公開HTTP routeは不要で、
URLのhost名は配送先の選択に使わない。RequestとResponseのbodyは双方streamし、配送前に
全文bufferしない。相手handlerの未捕捉例外は相手のHost生成500 ResponseとしてPromiseが
resolveする。配送を開始できない場合だけ`Error.name === "backend_unavailable"`でrejectする。
相手が応答までに完了した副作用はその応答に先行する。service Bindingは相手の`env`、
Host credential、認可された利用者identity、他のBindingを呼出し元へ渡さない。
Resource参照があること自体はHost管理操作の認可を与えない。WebSocket upgrade予約は
Hostが元client requestに結び付けるため、このservice callで転送・作成できない。

## 参照と拡張

このFormの入力に参照やBindingはない。WorkerVersionなどはこのResourceのUIDを参照する。
UIDが違う同名Workerは別の同一性であり、参照を名前で再解決しない。将来の入力・runtime ABI・
observedの意味の変更には新しいForm URLを用い、このURLの空specに未知の入力を足さない。
