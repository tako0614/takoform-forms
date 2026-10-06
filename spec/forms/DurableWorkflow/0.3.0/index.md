---
title: DurableWorkflow 0.3.0
description: 履歴replayとdurable stepを持つWorkflow classの同一性
formUrl: https://edge.forms.takoform.com/forms/DurableWorkflow/0.3.0/
hostApi: forms.takoform.com/v2
---

# DurableWorkflow 0.3.0

このFormは[ModuleWorker 0.3.0](../../ModuleWorker/0.3.0/)がexportする一つの
Workflow classと、そのinstance ID空間・履歴を表す。instanceはこのResourceの実行dataで
あり、個別のTakoform Resourceではない。コードは有効な
[WorkerDeployment 0.4.0](../../WorkerDeployment/0.4.0/)のweighted
[WorkerVersion 0.5.0](../../WorkerVersion/0.5.0/)から選ぶ。
`forms.takoform.com/v2`のこのFormをHostが支持すると言うには、この章のinstance、
durable step、replay、停止と削除の全条件を満たさなければならない。

## 入力、観測、CRUD

`spec`は`worker:{"resourceUid":string}`と`className:string`だけを必須とする。
`className`は`^[A-Za-z_$][A-Za-z0-9_$]{0,63}$`。既定値も秘密入力もない。未知キー、
`null`、入れ子参照の余分なキーを拒否する。Hostは参照先が同じHost・Spaceの
ModuleWorker 0.3.0であり、呼出者に利用権限があると副作用前に確認する。
参照先Worker Resourceは作成時に存在しなければならない。Deploymentだけは未作成でも
Workflowを作成できるが、全weighted Versionに正しいclassが揃うまでReadyでない。
UID参照は名前やlatestへ再解決しない。
Workflow作成要求の受理前に、有効なDeploymentの現在の配分と、先に受理された
未完了のDeployment作成・配分更新の適用予定をそれぞれ調べる。対象となる全weighted
Versionは、この`className`のWorkflow class ABIを満たさなければならない。classの
欠落・ABI不適合が確定した場合は`409 dependency_conflict`で副作用前に拒否し、既存の
Deploymentとその配分を変更しない。対象Versionの検証が未完了で判定できない場合は
`409 resource_busy`で受理せず、未検証を不適合と断定しない。有効なDeploymentもその
未完了の作成・配分更新もなければ、先行作成を許し、正しいVersionが選ばれるまでは
Readyにしない。受理済みOperationの結果は後からHTTPの事前拒否へ置き換えない。

`spec`の例（UIDは説明用）:

```json
{
  "worker": { "resourceUid": "worker-uid" },
  "className": "ReportWorkflow"
}
```

未観測の`observed`は `{}`。確認済みなら`ready:boolean`と、`queued`、`running`、
`sleeping`、`waiting`、`complete`、`errored`、`terminated`の各instance件数を持つ
`instanceCounts`を返す。時点は`observedAt`であり、GET時の即時照会を保証しない。
`ready:true`は新規instanceを受理できることだけを表し、アプリの成功ではない。`output`は `{}`。

作成はUIDと空のinstance ID空間を確保する。取得は管理記録と最後の観測を返す。
PUTは全体置換で、同じ`worker`と`className`だけ許す。変更要求は副作用前に拒否し、
同一specの新しいキーは新しいOperation/generationを作り再照合する。コード昇格は
Deploymentで行い、このResourceのPUTで既存履歴を改変・移行しない。

削除は、**削除されていないWorkerVersionの`workflowBindings`からの参照**がある間だけ
`409 dependency_conflict`で副作用前に拒否する。Versionがweightedか否かは問わない。
参照がなくなれば、このResourceの明示的なDELETEはそのUIDに属するqueued/running/
sleeping/waitingのinstanceも含めて廃止できる。Hostは新規create/event/wakeを閉じ、
実行ownerをfenceし、future continuationを取消し、childの物理的停止を確認してから、
このUIDの全instance ID・step履歴・未一致eventをpurgeする。terminal状態への事前移行や、
利用者による個別`terminate()`は必要ない。停止・fenceを証明できない間は成功にせず、
同じOperationで照合を続ける。古いowner tokenはその後commitできない。
他Resource、Worker、Version、外部dataを連鎖削除しない。削除後の同名再作成は新UID・
空履歴である。結果不明時は元のIdempotency-Keyで同じOperationを照合し、別UIDを
作り直さない。短時間の並行変更は`resource_busy`、存続するBindingは
`dependency_conflict`と区別する。

## Caller Bindingとinstance {#binding}

`workflowBindings[].resource`からの`env.NAME`は次のsurfaceを提供する。

```typescript
interface WorkflowBinding {
  create(input: { id?: string; params?: DataDocument }): Promise<WorkflowInstance>;
  get(id: string): Promise<WorkflowInstance>;
}
interface WorkflowInstance {
  readonly id: string;
  status(): Promise<{status: WorkflowStatus; output?: DataDocument;
    error?: {reason: WorkflowFailure; message?: string}} >;
  sendEvent(input: {type: string; payload?: DataDocument}): Promise<void>;
  terminate(): Promise<void>;
}
```

`id`は1〜256 Unicodeスカラー値。省略時にHostが一意なIDを発行する。既存の保持中IDを
createへ渡すと`instance_exists`で拒否し、応答を失ったcreateが同名の二重実行に化けない。
`get`は未知・保持期限後に`unknown_instance`。状態は正確に`queued`、`running`、
`sleeping`、`waiting`、`complete`、`errored`、`terminated`の七つである。`output`は
complete時だけ、`error`はerrored時だけで、失敗したrunも`status()`自体は成功する。
failure reasonは`run_threw`、`step_failed`、`step_limit_exceeded`、
`lifetime_exceeded`、`step_definition_mismatch`だけ。

全document（params、step結果、event payload、output）はplainまたはnull-prototypeの
object rootで最大1024 own property、RFC 8785 canonical UTF-8で最大1048576 bytes。
内部はnull、boolean、有限数、Unicode文字列、同条件のobjectまたは穴のない通常配列に限る。
accessor、symbol、function、BigInt、非JSON値、循環、配列の余分なproperty、nested
`undefined`を拒否する。共有する非循環objectはよい。getterや`toJSON`を実行せずdescriptor
を調べて直列化する。省略できるdocument全体の`undefined`だけを「なし」とみなす。
不正paramsは`invalid_params`、容量超過は`document_too_large`。未知の入力キーは拒否する。

`sendEvent`は1〜256文字の`type`と任意のpayloadをdurably受理し、instanceごとの受理順を
付けてから成功する。消費完了までは保証しない。terminal instanceは`instance_terminal`、
未知IDは`unknown_instance`。待機中の一致waitに直接渡せる場合を除き、未消費eventは最大
1024件かつ保存した`{type,payload?}`のcanonical UTF-8総量1048576 bytesで、超過は
`event_queue_full`として受理しない。terminal遷移では未一致eventをpurgeする。
`terminate()`はownerをfenceし、future continuationを取消し、物理的に実行contextが停止して
から`terminated`を公開して成功する。停止を証明できなければ成功を返さない。すでにterminal
なら成功する。caller操作のbackend不達は`backend_unavailable`で識別する。

## Class ABI、replay、step {#runtime}

全weighted Versionのmain moduleは`className`のconstructibleなnamed exportとcallableな
prototype `run`を持つ。Hostは新しい実行contextごとに`new Export(env)`を作り、
`instance.run({instanceId,params?},step)`を一回呼ぶ。`env`はそのVersionの宣言済み環境
のみで、管理controlは入れない。戻り値はdocumentまたは`undefined`（直接またはPromise）で、
`{output:...}`というwrapperを要求しない。constructor/module評価を含めstep外のコードは
履歴に対し決定的で副作用を起こさない。replayは新しいobject/closure/module stateから始まり、
前contextの変数や未settle Promiseは残らない。各contextは開始時点の配分から一版を選び
固定する。retry/wakeは新版を選び得るため、作者は履歴のstep name・効果・出力との互換性を
保つ。Hostは履歴を書き換えないし、その互換性を一般に証明できない。

`step`はこのinstance/contextに束縛され、アプリからinstanceIdを渡さない。
`step.do(name,effect,retryPolicy?)`、`step.sleep(name,seconds)`、
`step.waitForEvent(name,{type,timeoutSeconds})`の三操作だけを持つ。`name`とevent typeは
1〜256 Unicodeスカラー値。同時に重なるstep、未settle stepを残したrunのsettlement、
未完了名の異種再利用はcatchできない`step_definition_mismatch`で実行を止める。
完了済み名は成功・失敗を問わず記録結果でreplayし、新たな効果を起こさない。
nameを先に検証し、完了済み名の残りの引数は使わない。未完了名の不正なJS引数はTypeError、
同種の保留stepは最初に保存した設定を維持する。最多1024 step、instance寿命は作成から
31536000秒。上限・期限はHost制御で、アプリがcatchして延命できるsentinelではない。
`do`は`Promise<DataDocument | undefined>`、`sleep`は`Promise<void>`、
`waitForEvent`は`Promise<DataDocument | undefined>`を返す。最後の値は一致eventの
`payload`そのもので、payloadが無ければ`undefined`である。`{payload:...}`や
`{result:...}`というwrapperは返さない。失敗時は下記のHost ErrorでPromiseを拒否する。

- `do`: effectは零引数callableで、結果はdocumentまたは`undefined`。
  `retryPolicy`を省けば`maxAttempts:1`。指定時は`maxAttempts`が1〜100、
  `initialDelaySeconds`既定0、`backoff`既定`constant`（または`exponential`）、
  `maxDelaySeconds`既定43200。delayは0〜43200整数で、失敗した試行kの次のdelayは
  constantなら`min(maxDelaySeconds,initialDelaySeconds)`、exponentialなら
  `min(maxDelaySeconds,initialDelaySeconds*2^(k-1))`。各試行結果をcommitしてから次へ進む。
  失敗後のretryは0 delayでもfresh contextで行う。効果の実行後、結果commit前にHostが死ぬと
  効果は重複し得る。effectはidempotentに設計する。最終失敗は`step_failed`を記録する。
- `sleep`: `seconds`は0〜31536000整数。初回の期限を固定する。未来のsleepはcontextを
  停止してから`sleeping`を公開し、そのcontextのPromiseはresolveしない。期限後の新contextが
  履歴へ成功を記録してresolveする。0秒は現contextで完了する。寿命より先へ短縮しない。
- `waitForEvent`: `type`と`timeoutSeconds`（1〜31536000整数）は必須。
  instanceで最古の一致eventを原子的に消費しstep結果と一緒にcommitする。他typeは残り、
  一eventを二waitに配らない。該当なしなら停止してから`waiting`を公開する。同時刻なら
  timeout時刻に受理したeventが勝つ。期限後は`wait_timeout`を記録し、新contextで拒否する。

StepのHost Errorは正確なsnake-case `name`をown・読取専用・非列挙として持ち、由来は
そのError object/contextへ非公開で紐づく。同じobjectの再throwは由来を保つが、`name`を
copyした別Errorは保たない。未捕捉の本物の`step_failed`だけがterminal `step_failed`に
なり、未捕捉`wait_timeout`やTypeErrorなどは`run_threw`。アプリが失敗を処理すれば
completeにもなり得る。sleeping/waiting/terminalを公開する前にHostはコード停止を確認し、
古いownerのcommitをfenceする。処理後のstatusを先に見せてからコードを止めてはならない。
terminal履歴とIDはterminal遷移から**少なくとも2592000秒**保持する。その後はHostが
破棄でき、`get`が`unknown_instance`になり得る。このWorkflow Resourceの明示的な
DELETE成功だけが早期purgeの例外である。

本Form URLに新しい入力、status、step、失敗意味を暗黙追加しない。契約変更は別URLとする。
