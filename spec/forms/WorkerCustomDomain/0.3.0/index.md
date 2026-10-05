---
title: WorkerCustomDomain
description: 利用者が制御するDNS名をHTTPSでModuleWorkerへ接続するAttachment。
formUrl: https://edge.forms.takoform.com/forms/WorkerCustomDomain/0.3.0/
hostApi: forms.takoform.com/v2
---

# WorkerCustomDomain 0.3.0

このFormは、利用者が管理するDNS名を一つModuleWorkerのactive deploymentへHTTPSで接続するAttachmentを表します。Host API v2のResourceとして管理し、共通のCRUD・再送規則は[Takoform Host API v2 HTTP API](https://takoform.com/spec/host-api/v2/http)に従います。ドメイン所有・DNS管理の権利が必要です。Hostは要求を受け付ける前に利用者がそのDNS名を管理できることを検証し、TLS設定を行います。特定のDNS/TLSベンダー、検証用レコード方式、ゾーン配置やクラウドAPIは規定しません。

## 入力

`spec` は次のフィールドだけを受け付けます。両方ともAttachment UID存続中不変です。変更は削除と新規作成として行います。

| フィールド | 要件、既定値、範囲 | 作成後の変更 |
| --- | --- | --- |
| `hostname` | 必須のASCII dotted DNS名。最長253文字。各labelは1〜63文字で、ASCII英数字で始まり終わり、内部にはASCII英数字またはハイフンを使います。A-label形式の国際化ドメインを受け付けます。wildcard、IPv4/IPv6 literal、空labelは受け付けません。末尾のroot dotは任意です。 | 不変。 |
| `worker` | 必須の `{ "resourceUid": "…" }`。ModuleWorker 0.3.0 Resourceを参照します。active Deploymentの全weighted Versionが宣言済み`fetch`または検証済み`assets`でHTTP要求に応答できる必要があります。既定値なし。 | 不変。 |

HostはDNS比較・保存前にhostnameのASCII英字を小文字化し、末尾のroot dotを除去します。従って大文字やroot dotの有無だけが異なる入力は同じ名前の占有です。秘密入力はありません。空でない `privateInputs` と未知のspecフィールドは拒否します。

```json
{
  "hostname": "api.example.net.",
  "worker": { "resourceUid": "worker-uid" }
}
```

## ドメイン管理の証明とTLS

Hostは正規化後のhostnameに対し、呼出主体がDNSを制御できることを確認するまで外部route/TLSの変更を始めてはいけません。既存の有効な検証記録を再利用する場合も、その記録は同じ管理主体・同じ正規化hostnameに対するものでなければなりません。管理を確認できない、別主体が保有する、または他Attachmentが占有している場合は副作用前に拒否します。検証の具体的な確認手段、DNS/TLS事業者、内部認証フローはHost実装の詳細であり、このFormは事業者固有のAPIや値を要求しません。

HostはTLS証明書を有効化し、hostnameのHTTPS経路をWorkerのactive deploymentへ接続します。外部HTTP要求は選択済みVersionの[asset/fetch配信規則](https://edge.forms.takoform.com/forms/WorkerVersion/0.5.0/)に従います。`observed` の正確な形は下記のとおりです。HTTPアプリケーション応答や業務上の利用可能状態はこのFormの成功条件ではありません。Worker deploymentの切替は同じhostnameで応答する内容を切り替えます。

## 観測、出力、利用可能状態

Resource作成後の `output` は常に正規化済み `hostname` と `url` を含みます。`hostname` は253文字以下、`url` は `https://` + 正規化済みhostname + `/` に完全一致する262文字以下の文字列です。

```json
{
  "hostname": "api.example.net",
  "url": "https://api.example.net/"
}
```

`observed` は初回確認前は `{}` です。確認後はHostが確認した項目だけ `dnsControlVerified`、`tlsReady`、`activeDeploymentRouteReady` を真偽値で含みます。`false` は確認済みの未成立、項目省略は未確認または結果不明を表します。`observedAt` は共通Resourceの確認時刻です。例:

```json
{
  "dnsControlVerified": true,
  "tlsReady": true,
  "activeDeploymentRouteReady": true
}
```

出力が存在すること、Resourceの `idle`、Operation成功はアプリケーションがHTTP成功を返す証明ではありません。DNS管理権限の確認、TLS有効化、active deploymentへの経路準備がすべて真と確認されることをこのAttachmentの利用可能状態とし、アプリケーション応答や業務上の健全性は含みません。

## 操作、衝突、復旧

- **作成:** Host API v2の認可とResource検証後、hostname文法、DNS管理の証明、Worker参照、active Deploymentの全weighted VersionのHTTP応答条件、同一テナント全Spaceを通じたhostname占有の排他性を副作用前に検査します。一つの正規化hostnameはテナント内の有効Attachment最大1つが保有できます。二つ目を拒否し、先行Attachmentを変更しません。後のDeployment切替でも同じ条件を検証します。
- **取得:** Attachment、正規化hostname、最後に確定したDNS/TLS/route観測、出力を返します。GETは検証やroute作成を新たに開始しません。
- **更新:** `hostname` とWorker UIDの変更は副作用前に拒否します。全weighted VersionのHTTP応答条件を再確認し、同一specでの明示的更新は既存の占有記録を保ったままTLS/経路状態を収束させます。別hostnameへの変更は旧Attachmentの削除後に新Resourceとして作成します。
- **削除:** このAttachmentが所有する占有記録、経路、TLS設定を解除します。Workerやそのコードを削除・変更しません。削除完了まではHostは占有記録を保持し、同じhostnameを他Attachmentへ割り当てません。

Hostはhostnameの占有記録、DNS検証結果、TLS/経路状態、Operationを永続化し、再起動後も所有者と途中状態を復元します。DNS変更やTLS/経路反映の応答を失い結果が不明な場合は `effect:unknown` の `reconciling` にし、別占有、別UID、別実行を作りません。一部設定が残っていれば `effect:partial` を返し、同じUIDへの明示的更新で収束、削除で解除します。応答待ち時間超過だけで無効果や解放済みと断定しません。Hostが制御権の確認または外部状態の照合をできない間、占有記録を勝手に解放して重複利用を許可してはいけません。Idempotency-Keyによる再送は同じOperationを返します。

失敗後もResourceは管理記録・占有状態を失いません。削除要求の結果が不明な場合、Hostは既存hostnameの占有を維持し、実際に外部経路/証明書を解放したと確認してから削除を完了します。削除後に同じhostnameを利用するには、新しいResourceの作成と必要なDNS権限検証を行います。

## 所有、参照、削除の範囲

このResourceが所有するのはhostname Attachment、占有記録、当該経路とTLS設定だけです。Worker参照は `{ "resourceUid": "…" }` で、同じHost・同じSpaceにある `https://edge.forms.takoform.com/forms/ModuleWorker/0.3.0/` と完全一致するResourceだけを指します。名前、Host事業者の識別子、DNS値から参照先を再解決しません。Resource参照はDNS所有権・Host認可の根拠になりません。

削除はAttachmentが管理するhostname占有、経路、TLS設定の解除だけに及びます。Worker、active deployment、他Attachmentや利用者DNS zone全体を削除しません。Workerの削除はこのAttachmentが残る間拒否されます。DNSレコードを利用者自身が管理する場合でも、Hostはゾーン全体の削除を要求しません。

## Interface、Binding、未知のフィールド

このForm独自のInterfaceまたはBindingはありません。要求入口はWorkerVersionのasset配信または宣言済み`fetch` handlerです。`spec` と参照オブジェクトは閉じた形であり、未知フィールドを拒否します。参照オブジェクトは `resourceUid` のみです。
