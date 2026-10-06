---
title: Hostで資源を作る
description: 対応確認、作成、状態確認、更新、削除をHost API v2で進める手順。
---

# Hostで資源を作る

以下は、EdgeKVNamespaceを一つ作る通信例です。説明用のHostを使っており、実環境での成功記録ではありません。
HostがこのFormを実装し、利用者にSpaceへの権限があることを前提にします。認証方式、APIの接続先、料金はHostの案内で確認します。

## 1. 接続先と対応するFormを確認する

`GET https://host.example/.well-known/takoform/v2`で、APIの`baseUrl`、認証案内、上限、任意機能を確認します。
以後はその`baseUrl`を使います。Formの仕様はこのサイトで読み、Hostから仕様を取り寄せる必要はありません。

使うForm URLは次の文字列です。末尾の`/`まで含めて識別子で、転送先や別の版とは同一視しません。

```text
https://edge.forms.takoform.com/forms/EdgeKVNamespace/0.2.0/
```

`GET {baseUrl}/support?form={URLエンコードしたForm URL}`で`form`の一致と`supported:true`を確認します。
Formに必須の操作が実装されていなければ、このHostでは利用できません。
APIの版だけが同じでも、すべてのFormを扱えるとは限りません。

Discoveryの`capabilities.offerings`が`true`なら、`GET {baseUrl}/offerings?form=...&space=...`で
提供条件を選びます。選んだ`id`と`revision`を作成時に渡し、料金や制約の確認を省略しません。
以下の例ではOfferingのないHostを使います。Offeringがないことは無料という意味ではありません。

## 2. 作成する

```http
POST /apis/forms.takoform.com/v2/resources HTTP/1.1
Host: host.example
Content-Type: application/json
Idempotency-Key: 0494f052-d72f-4a95-916a-5c5c3e791fd3

{
  "form": "https://edge.forms.takoform.com/forms/EdgeKVNamespace/0.2.0/",
  "space": "personal",
  "name": "page-cache",
  "spec": {}
}
```

認証ヘッダーは説明上省略しています。`spec:{}`は、このFormが調整可能な設定を持たないためです。
結果整合性などの性質は、空の設定であっても個別仕様が定めます。

HostはOperationを返します。`202`ならまだ処理中です。応答の`Location`でOperationを取得し、
`status:succeeded`を確認した後、その`resourceUid`を使ってResourceを取得します。
`failed`ならエラーと残った効果を確認します。`reconciling`は結果不明の調査中であり、別の作成を始める理由にはなりません。

## 3. 状態を確認する

`GET {baseUrl}/resources/{resourceUid}`で`generation`、`observedGeneration`、`observedAt`と
Form固有の`observed`を確認します。取得時刻と外部の最終観測時刻は別です。
このFormの管理操作が成功しても、Workerが接続済みとは限りません。利用するには別途WorkerVersionのBindingが必要です。

## 4. 更新する

```http
PUT /apis/forms.takoform.com/v2/resources/r_kv_1 HTTP/1.1
Host: host.example
Content-Type: application/json
Idempotency-Key: 55bdc29f-b7c5-4444-82fe-154ebdc1750b
Takoform-Expected-Generation: 1

{ "spec": {} }
```

このFormでは入力を変えられないため、同じ設定を照合する更新です。新しいキーの更新は新しいOperationと世代を持ちます。
一般のFormではPUTが`spec`全体の置換になります。変更不能なbundleやWorkerVersionの内容を変える場合は、
新しいResourceを作り、参照元を明示的に切り替えます。
WorkerVersionの秘密値も実行する版の一部です。秘密を変更する場合は新しいWorkerVersionを用意し、
Deploymentで切り替えます。同じUIDのまま、稼働中の版の意味を変えません。

## 5. 削除する

最新の世代を取得して、別の`Idempotency-Key`と`Takoform-Expected-Generation`を付けた
`DELETE {baseUrl}/resources/{resourceUid}`を送ります。参照しているWorkerVersionが残っていれば、
先に利用を終了し、その参照を片付ける必要があります。Hostは暗黙にアプリ全体を削除しません。
Operationの成功が確認できるまで削除完了として扱いません。KVの内容も削除対象なので、必要なデータは事前に退避します。

## 応答が消えた場合

同じ操作の応答を受け取れなかった場合は、同じ認証主体、経路、本文、キーで再送します。
タイムアウトだけでキーやResource名を変えると別の操作になります。Hostが知らせた再送保証期間とOperationの保持期限を確認し、
保証期間を過ぎた場合は現状を照合してから判断します。再起動後も重複した外部資源を作らないことはHost実装側の要件です。

秘密入力が必要なFormでは、再送や補給も[共通HTTP仕様](https://takoform.com/spec/host-api/v2/http)に従います。
秘密値をURL、公開`spec`、ソース管理や確認用ログへ書きません。

## コードとファイルを渡す

WorkerBundle、StaticAssetBundle、SQLiteMigrationSetでは、manifestのHTTPS URLとSHA-256を指定します。
Formの仕様URLとは別の、アプリの実データです。Hostはmanifestと各ファイルを検証して保持するため、
後で配布元に接続できなくなっても、保持した資源を復旧できる必要があります。

非公開コードを公開サイトへ置くことは必須ではありません。利用者に権限のあるHost管理の保管領域や、
運用者が接続を許可した非公開の配布元も使えます。利用者がURLを書くだけで内部ネットワークへの接続や
他の利用者のファイルへのアクセスが許可されるわけではありません。事前アップロード等の補助機能を提供するかはHostの選択です。

## 静的ファイルだけを公開する場合

対応するHostでは、[StaticAssetBundle 0.2.0](https://edge.forms.takoform.com/forms/StaticAssetBundle/0.2.0/)を先に作成・検証し、
[ModuleWorker 0.3.0](https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/)の空の同一性を作ります。そのWorkerとassetの
UIDを使い、[WorkerVersion 0.5.0](https://edge.forms.takoform.com/forms/WorkerVersion/0.5.0/)を例えば次の`spec`で作成します。
コード用WorkerBundleやダミーの`fetch` handlerは必要ありません。UIDは説明用です。

```json
{
  "worker": { "resourceUid": "worker-uid" },
  "handlers": [],
  "assets": {
    "bundle": { "resourceUid": "assets-uid" },
    "runWorkerFirst": false,
    "notFoundHandling": "none"
  }
}
```

その版を10000 basis pointで選ぶ[WorkerDeployment 0.4.0](https://edge.forms.takoform.com/forms/WorkerDeployment/0.4.0/)を
作り、[WorkerEndpoint 0.3.0](https://edge.forms.takoform.com/forms/WorkerEndpoint/0.3.0/)か
[WorkerCustomDomain 0.3.0](https://edge.forms.takoform.com/forms/WorkerCustomDomain/0.3.0/)をWorkerへ添付します。
GET/HEADは検証済みassetを探索し、該当しなければ404を返します。その他のmethodも
この静的専用構成では404です。SPA fallbackを選ぶ場合はasset bundle rootに`index.html`が
必要です。公開前にHostのSupport、各Operation、Version/Deployment/入口のReady観測を
確認します。この例は特定Hostの対応や公開済みサイトの存在を示しません。

## ActorとWorkflowのclassを追加・撤去する

新しいWorkerなら、空のModuleWorkerを作り、ActorNamespaceまたはDurableWorkflowを先に
作れます。Deploymentがない間、それらはまだReadyではありません。必要なclassと
Bindingを含むWorkerVersionを作り、全weighted Versionがclass ABIを満たす
WorkerDeploymentを作ると、新しいeventを受け付けられます。WorkerVersionのBindingは
先に作ったNamespace/WorkflowのUIDを参照します。

すでに稼働中のWorkerへclassを追加する場合は、先に全weighted Versionをそのclassを
提供する版へ切り替え、切替Operationの成功を確認してからNamespace/Workflowを作ります。
古い版と新しい版を併用する間も全版にclassが必要です。classのない版が一つでも
現在の配分に残れば、Namespace/Workflowの作成は`409 dependency_conflict`で拒否され、
既存の配分は維持されます。作成後にそのUIDを使うBindingが必要なら、新しい
WorkerVersionを作ってDeploymentを切り替えます。

稼働を続けながらclassとBindingを外す場合は、まずclassを残しBindingだけを外した
中間WorkerVersionへ切り替えます。旧Versionの実行退役を確認し、Namespace/Workflowを
Bindingしている未削除Versionをすべて削除してから、Namespace/Workflowを削除します。
その後にclassも外した版へ切り替えられます。Bindingを持つ版を残したまま
Namespace/Workflowを削除したり、Namespace/Workflowが残るうちにclassのない版を
選んだりはできません。削除対象のActor dataやWorkflow履歴は各FormのDELETE条件に従い、
Deployment切替では暗黙に消えません。
