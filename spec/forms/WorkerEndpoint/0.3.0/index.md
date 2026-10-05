---
title: WorkerEndpoint
description: 利用者DNSを使わずHost割当てのHTTPS endpointをModuleWorkerへ接続する。
formUrl: https://edge.forms.takoform.com/forms/WorkerEndpoint/0.3.0/
hostApi: forms.takoform.com/v2
---

# WorkerEndpoint 0.3.0

このFormは、Hostが割り当てる一つのHTTPSアドレスから、ModuleWorkerの現在のactive deploymentへHTTP要求を到達させるAttachmentを表します。利用者所有のドメインや利用者によるDNS設定を要求しません。Host API v2のResourceとして管理し、共通のCRUD・再送規則は[Takoform Host API v2 HTTP API](https://takoform.com/spec/host-api/v2/http)に従います。特定のHost、DNS事業者、クラウド実装を要求しません。

## 入力

`spec` は次のフィールドだけを受け付けます。

| フィールド | 要件、既定値、範囲 | 作成後の変更 |
| --- | --- | --- |
| `worker` | 必須の `{ "resourceUid": "…" }`。ModuleWorker 0.3.0 Resourceを参照し、`fetch` handlerを提供します。既定値なし。 | Attachment UIDの間不変。変更には削除と新規作成が必要です。 |

```json
{
  "worker": { "resourceUid": "worker-uid" }
}
```

秘密入力はありません。空でない `privateInputs` は拒否します。

## 観測、出力、利用可能状態

Hostは作成要求を受理するときにアドレスを割り当てます。初回受理後の `output` は、routeやTLS設定の進行中・失敗中を含め常に両方の値を含みます。

```json
{
  "hostname": "assigned.example.net",
  "url": "https://assigned.example.net/"
}
```

`hostname` はHostが割り当てた小文字の絶対DNS名で、末尾root dotなし、最大253文字です。各labelは1〜63文字で、英数字で始まり終わり、内部には英数字またはハイフンを使います。`url` は `https://` + `hostname` + `/` に完全一致する最大262文字の絶対HTTPS URLです。TLSを必須とします。ホスト名のサフィックス、事業者、内部配置、IPアドレスは利用者が依存できる値ではありません。Resource UID存続中、hostnameとurlは不変であり、deployment切替、Host内部移動、基盤移行で変わりません。Hostが同じUIDのアドレスを維持できない場合はAttachmentを削除し、利用者が新しいResource UIDで作り直します。

`observed` は初回確認前は `{}` です。確認後は、Hostが値を確認したものだけ `tlsReady` と `activeDeploymentRouteReady` を真偽値で含めます。`false` は確認済みの未準備、項目省略は未確認または結果不明です。`observedAt` は共通Resourceの確認時刻です。例:

```json
{
  "tlsReady": true,
  "activeDeploymentRouteReady": true
}
```

`output` の値が判明したことはアプリケーションの利用可能状態を意味しません。作成Operationの成功はDNS/TLS/route設定を完了したことまでです。アプリケーションがHTTP 200を返すか、依存先へ接続できるか、業務上健全かをこのFormは保証せず、アプリケーションのhealth endpointを呼び出して成功条件にしません。active deploymentのコード変更は同じ固定アドレスへ反映されます。

## 操作、拒否、復旧

- **作成:** Worker参照が同じHost・SpaceのModuleWorker 0.3.0であり、`fetch` handlerが宣言・提供されることを副作用前に検証します。一つのWorkerに有効なEndpointは最大1つです。2つ目や参照条件違反を拒否し、既存endpointを変更しません。
- **取得:** Attachment、最後に確定したroute観測、割当て済みならhostname/urlを返します。読み取りはroute設定を変更しません。
- **更新:** Worker UIDの変更は副作用前に拒否します。同じWorkerへの再調整要求は現在のアドレスを保ったままrouteを収束させます。アドレスの差替えやschemeの変更はできません。
- **削除:** endpoint routeと割当Attachmentを解除します。Worker Resourceまたはdeploymentは削除しません。Hostは既知の部分失敗と割当てを保持し、同じUIDの更新で再調整し、削除で後始末できる必要があります。

Hostは割当アドレス、TLS構成、経路対象、Operationを永続化し、プロセス再起動後も同じUIDとアドレスを復元します。参照Workerは[ModuleWorker 0.3.0 runtime ABI](https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/#runtime)の `fetch` handlerを提供します。外部経路の作成後、応答を失うなど結果が不明な場合は `effect:unknown` として `reconciling` に保持し、別アドレスを盲目的に割り当てません。Hostが一部だけ設定したと確認した場合は `effect:partial` を返します。応答待ち時間超過や再起動のみで `effect:none` としてはいけません。既知の孤児経路を利用者に隠さず、同じUIDの明示的なupdate/deleteで収束・後始末します。未確定操作中に別UIDのendpointを同じWorkerへ発行しません。

v2のIdempotency-Key再送は同じOperationを返します。経路/TLS側で安全に照合できるまで新しい作成を始めず、照合不可能なら `unknown` として止めます。作成成功後のアプリケーション応答は経路設定と別の責任です。

## 所有、参照、削除の範囲

このResourceが所有するのは一つのassigned HTTPS endpointです。Worker参照は `{ "resourceUid": "…" }` のみです。参照先は同じHost・Spaceにあり、Form URLが `https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/` と完全一致しなければなりません。参照UIDは不変で、名前から別Workerへ再解決しません。参照は認可を与えません。

削除はEndpoint Attachmentとそれが管理するrouteだけを対象とします。Workerやそのactive deploymentを削除・変更せず、Worker削除へ連鎖しません。Worker Resource削除はEndpointが残る間拒否されます。

## Interface、Binding、未知のフィールド

このForm独自のInterfaceまたはBindingはありません。外部要求を受ける入口はModuleWorker ABIの `fetch` handlerです。`spec` と参照オブジェクトは閉じた形で、未知のフィールドは拒否します。参照オブジェクトの唯一のフィールドは `resourceUid` です。
