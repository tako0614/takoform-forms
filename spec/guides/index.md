---
title: Edge Formsを使う
description: Worker、データ保存、イベント処理に必要なFormを選び、対応するHostで使うための案内。
---

# Edge Formsを使う

Edge Formsは、Workerアプリケーション、データ保存、イベント処理に使う資源の仕様です。
たとえばWorkerのコードを更新する場合、コードの集合を作り、そのコードと設定を一つの版にまとめ、
トラフィックの行き先を切り替えます。それぞれの資源が何を受け取り、何を変更するかをFormで定めます。

このサイトは`tako0614`が定義するFormを掲載します。他の作者のFormやProviderと組み合わせて使えます。
共通の通信方法は[Takoform Host API v2](https://takoform.com/spec/host-api/v2/http)、
各資源の意味は以下の個別仕様、実行先と提供条件は利用するHostが定めます。

## 目的から選ぶ

- **Workerのアプリケーションを作る** — [ModuleWorker](/forms/ModuleWorker/0.3.0/)がアプリの識別と実行モデルを定めます。
- **コードを登録する** — [WorkerBundle](/forms/WorkerBundle/0.2.0/)に、検証できるコードとファイルの組を渡します。
- **静的ファイルを使う** — [StaticAssetBundle](/forms/StaticAssetBundle/0.2.0/)を[WorkerVersion](/forms/WorkerVersion/0.5.0/)へ接続します。
- **コードと設定を固定する** — [WorkerVersion](/forms/WorkerVersion/0.5.0/)がbundle、ハンドラー、環境変数、資源への接続をまとめます。
- **稼働する版を切り替える** — [WorkerDeployment](/forms/WorkerDeployment/0.4.0/)がWorkerVersionへのトラフィック配分を選びます。
- **HTTPSで公開する** — Hostが割り当てる[WorkerEndpoint](/forms/WorkerEndpoint/0.3.0/)、または所有する名前の[WorkerCustomDomain](/forms/WorkerCustomDomain/0.3.0/)を使います。
- **key/valueを保存する** — [EdgeKVNamespace](/forms/EdgeKVNamespace/0.2.0/)は結果整合のバイト列ストアです。即時に同じ値が読める保証とは異なります。
- **ファイルを保存する** — [ObjectBucket](/forms/ObjectBucket/0.2.0/)はストリーミングとmultipart操作を持つオブジェクトストアです。
- **SQLでデータを扱う** — [SQLiteDatabase](/forms/SQLiteDatabase/0.2.0/)がデータベース、[SQLiteMigrationSet](/forms/SQLiteMigrationSet/0.2.0/)が変更履歴、[SQLiteMigrationApplication](/forms/SQLiteMigrationApplication/0.2.0/)がその適用です。
- **非同期にメッセージを処理する** — [AtLeastOnceQueue](/forms/AtLeastOnceQueue/0.2.0/)へ送り、[QueueConsumer](/forms/QueueConsumer/0.3.0/)でWorkerへ届けます。重複配信を前提にします。
- **定期処理を実行する** — [WorkerCronTrigger](/forms/WorkerCronTrigger/0.3.0/)がUTCのスケジュールでWorkerを呼び出します。
- **IDごとの状態を持つ処理を動かす** — [ActorNamespace](/forms/ActorNamespace/0.3.0/)がActorのID空間と実行を定めます。
- **中断から続行する処理を動かす** — [DurableWorkflow](/forms/DurableWorkflow/0.3.0/)がステップの保存、再実行、完了結果を定めます。

### Queue、Actor、Workflowの違い

Queueは処理したいメッセージを配送します。Actorは同じIDへの呼び出しと状態を扱います。
Workflowは一つの実行の進み具合を保存し、中断後に続けます。どれか一つが他の代替になるわけではありません。
アプリの必要な性質に合わせて選び、配信の重複や外部への副作用も各仕様に沿って扱います。

## 最初に読むもの

1. [Hostで資源を作る](/v2/use-host/)で、Formの選択から作成・更新・削除までの流れを確認します。
2. 個別Formの入力、出力、削除範囲を読みます。入力例のUIDやartifact URLは説明用です。
3. [Terraform/OpenTofuで使う](/v2/providers/)で、Providerが対応する版とHCLの所有範囲を確認します。
4. 既存v1の資源がある場合だけ、[移行と既存資料](/v2/migration/)を読みます。

## 仕様と利用できる状態

ここにある版固定URL向けの本文はHost API v2に対応するForm仕様です。現在はリポジトリ内の仕様本文で、
これらのURLへの正式公開・版の固定手順は未実施です。HostやProviderの対応済み一覧ではありません。
利用するHostの`support`で同じURLへの対応を確認し、Offeringを持つHostでは容量・料金・制限も確認します。

ContainerやVectorIndexの既存提案は、この17種類に含めていません。別の検討を、URLや実装が存在するだけで
利用できるFormとして紹介しません。各Formの版は独立しており、この一覧全体のリリース版はありません。
