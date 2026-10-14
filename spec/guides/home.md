---
title: Edge Forms
description: Edge Formsの概要と、公開済みHost API v1資料への入口。
---

# Edge Forms

Worker、ストレージ、イベント処理に使う資源の仕様です。
入力だけでなく、接続方法、更新できる範囲、削除と失敗時の挙動まで定めます。
利用するHostがこのFormに対応するかを確認し、Hostの案内に従って資源を操作します。

## 何を作りますか

- **Workerを動かす** — [アプリの識別](/forms/ModuleWorker/0.3.0/)、[コード](/forms/WorkerBundle/0.2.0/)、[実行する版](/forms/WorkerVersion/0.5.0/)、[公開先](/forms/WorkerEndpoint/0.3.0/)。
- **データを保存する** — [KV](/forms/EdgeKVNamespace/0.2.0/)、[オブジェクト](/forms/ObjectBucket/0.2.0/)、[SQLite](/forms/SQLiteDatabase/0.2.0/)。
- **イベントを処理する** — [Queue](/forms/AtLeastOnceQueue/0.2.0/)、[Cron](/forms/WorkerCronTrigger/0.3.0/)、[Actor](/forms/ActorNamespace/0.3.0/)、[Workflow](/forms/DurableWorkflow/0.3.0/)。

[全17種類と組み合わせを見る →](/v2/)

## 使い始める

[Hostで資源を作る](/v2/use-host/)では、対応確認から作成・更新・削除までを説明します。
OpenTofuやTerraformを使う場合は、[Providerの対応版と使い分け](/v2/providers/)を先に確認してください。

資源の意味はこのサイト、共通の通信方法は[Takoform](https://takoform.com/)で確認できます。
Host API v2の実行先、認証、対応状況、料金・利用条件は各Hostが定めます。このサイトの作者は`tako0614`です。
他の作者のFormやProviderと組み合わせて使えます。

既存v1の資料は[以前のForm一覧](/v1/)と[移行の案内](/v2/migration/)に分けています。
