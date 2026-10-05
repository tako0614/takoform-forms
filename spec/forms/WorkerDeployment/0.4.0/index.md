---
title: WorkerDeployment 0.4.0
description: 一つのWorkerへの版選択とトラフィック重み
formUrl: https://edge.forms.takoform.com/forms/WorkerDeployment/0.4.0/
hostApi: forms.takoform.com/v2
---

# WorkerDeployment 0.4.0

このFormは[ModuleWorker 0.3.0](../../ModuleWorker/0.3.0/)一つの有効な
[WorkerVersion 0.5.0](../../WorkerVersion/0.5.0/)集合を選ぶ。`forms.takoform.com/v2`の
Resourceであり、公開endpointやcronそのものを作らない。複数Versionへの配分は新たな
イベントを選ぶ規則であって、進行中の実行、Actorの永続store、Workflowの履歴を移行しない。

## 入力と参照

`spec`は`worker`と`versions`だけを必須とする。`worker`はModuleWorker 0.3.0の
`{"resourceUid":string}`参照である。`versions`は1〜8件の
`{"workerVersion":{"resourceUid":string},"weight":integer}`の配列で、
`weight`は1〜10000 basis point、総和は**正確に10000**とする。同じVersion UIDを二度
書けず、0 weightは省略で表す。配列順は選択確率に影響しない。既定値はない。
`spec`と全入れ子の未知キー、`null`、小数、負数、範囲外、重複を拒否する。秘密入力はない。
同一specの比較では`versions`をVersion UID順の集合として扱い、配列順とobjectのキー順は
無視する。同じUID・weightの全組が等しければ同一specであり、weightの変更は更新である。

全トラフィックを一版へ割り当てる`spec`の例（UIDは説明用）:

```json
{
  "worker": { "resourceUid": "worker-uid" },
  "versions": [
    { "workerVersion": { "resourceUid": "version-uid" }, "weight": 10000 }
  ]
}
```

Hostは各参照が同じHost・Spaceに存在し、要求者が利用を認可され、Form URLが上記の版に
完全一致することを副作用前に検証する。各Versionの`spec.worker.resourceUid`はこの
Deploymentの`worker.resourceUid`と一致しなければならない。Version UIDを保存し、名前や
「最新」から再解決しない。一つのWorkerに有効なDeploymentは高々一つである。

## 選択と成立条件

Hostは新たなfetch、scheduled、queue、Actor event、Workflowの新しい実行contextを受理する
時点の有効な配分からVersionを一つ選び、その一つを当該invocationの終了まで固定する。
配分はbasis pointの比率であり、個別requestに決定的なhash結果や短期の正確な件数を保証しない。
Worker Versionを選択できなければ別のVersionへ黙ってfallbackしない。Clientに見える失敗は
該当するhandler/Binding契約による。新しい配分は受理後の実行にだけ適用し、既存のHTTP stream、
Actor event、Workflow contextを途中で切り替えない。Actorの次のeventとWorkflowの次の
retry/wakeは、その時点の配分を再選択できる。

`ready:true`として有効化する前にHostは全weighted Versionのbundle、handler、Binding、
必要な秘密を確認する。添付されたendpoint/cron/queue consumerが要求するhandlerを、
到達し得る全Versionが宣言・exportしなければならない。同じWorkerを参照するActorNamespaceと
DurableWorkflowの全`className`を各Versionが正しいprototype ABIで提供しなければならない。
Workflowのstep name・効果・出力について新旧コードの履歴互換性は作者の責任であり、Hostは
既知の不整合を検出したら有効化を拒否し、検証できないことを互換性の証明として扱わない。
Hostは移行・履歴書換え・Actor store初期化で不整合を隠さない。参照先の後続削除や秘密消失で
Readyが崩れたら新規受理を拒否し、残存状態を観測可能にする。

## 状態と操作

未観測の`observed`は `{}`。確認済みなら`ready:boolean`、`active:boolean`、
`selectedVersions`（`{"resourceUid":string,"weight":integer}`の配列）を返す。
`active:true`は、このDeploymentがそのWorkerの新しいinvocationの選択元であることを表す。
`ready`は有効化可能性の観測であり、各requestや外部到達性の成功保証ではない。
`observedAt`/`observedGeneration`は共通APIの時点を示す。`output`は `{}`。

- 作成では参照と条件を検査し、一つのWorkerの有効配分として原子的に設置する。別の有効
  Deploymentがある場合は置換せず拒否する。依存先が準備中なら受理したOperationは状態を
  明示して待つことができるが、Readyでない配分へトラフィックを流さない。
- 取得は最後に確認した配分と状態を返す。管理記録がある間は実行先の一時的な不在を
  404に置き換えない。
- 更新は`spec`全体を置換する。`worker`は不変で、変えるには別Deploymentを作る。
  `versions`は全件指定し、Hostは新集合を検証してから一度に切り替える。途中の部分配分を
  公開しない。同一specの新しいキーのPUTも新しいOperation/generationで再照合する。
  旧weightから外れたVersionには切替後の新規invocationを割り当てない。既存contextは
  固定Versionで完了できるが、切替から15分を超えて残るcontextはHostが取消し、streamを
  閉じ、childの物理的退役を確認する。この退役は新配分の有効化を妨げないが、古いVersionの
  削除は退役が済むまで拒否する。Host障害時は所有記録から再開し、退役未確認を完了扱いしない。
- 削除はまず新たなfetch・scheduled・queue・Actor event・Workflow contextの選択を閉じる。
  既存invocationには取消signalを送り、HTTP body/Actor upgrade予約とHost所有streamを止め、
  child実行の物理的退役と古いownerのfenceを確認してから成功にする。Promise rejectionだけを
  退役の証明としない。Hostが確認できない間はOperationを未確定として照合を続ける。
  削除後はDeploymentなしでActor alarm/socket callbackやWorkflow wakeを新規配送しない。
  Actorのstore・alarm・socket identity、Workflow instance・履歴は各Resourceに残すが、
  旧Version UIDへのlive context参照は残さない。Worker/Versionや添付先Resourceを連鎖削除しない。

結果が不明な作成・更新・削除は元のIdempotency-Keyと要求を再送し、Operationと実行先を
照合する。タイムアウトから成功・失敗を推定して別のDeploymentを作らない。
参照先の所有者は参照先FormとHostであり、Deploymentはそのコードやdataを所有しない。
未知フィールドを受け入れる拡張や意味の変更には新しいForm URLを使う。

通常の後始末は、まずendpoint・cron・queue consumer等の入口を削除し、次にこのDeploymentを
削除して実行を退役させ、不要なVersionを削除し、そのVersionのBinding参照が消えてから
ActorNamespace/Workflowを削除し、最後にWorkerを削除する。Namespace/Workflowの休眠dataを
消すのは各Resourceの明示的なDELETEであり、Deployment DELETEの連鎖副作用ではない。
