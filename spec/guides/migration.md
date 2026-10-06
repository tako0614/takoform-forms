---
title: 移行と既存資料
description: v1のFormRefとpackageを保持し、別のForm URLへ明示的に移行する際の境界。
---

# 移行と既存資料

新しいForm仕様は、Host API v2を使い、版が固定されたHTTPS URLを識別子にします。
既存のv1 FormRefや署名済みpackageの内容を、そのURLの別表記として扱うわけではありません。
v1の公開内容はそのまま保存します。

## 変わるものと変わらないもの

| 対象 | 扱い |
| --- | --- |
| v1のForm定義、package、tag、署名 | 既存のbytesと識別子を保持します。新しい意味で上書きしません。 |
| このサイトの既存Formページ | 元の版の資料として残します。新しいURLへ黙って置換しません。 |
| v2向けの個別Form仕様 | 新しい版のURLで、入力・出力・操作・接続の意味を記述します。 |
| Host API | 共通仕様のv2を使います。Host内部のDBや実行基盤の移行方法はHost実装が決めます。 |
| ProviderとSDK | それぞれのreleaseで対応します。FormやAPIと同時に版を進める必要はありません。 |

v2は仕様packageのインストール、署名検証、Coreライブラリの使用を参加条件にしません。
一方、アプリのコードやファイルは実行に必要なデータです。WorkerBundle等にあるartifactのdigest検証は、
仕様を配布するpackageや署名の仕組みとは別です。

新しいWorkerVersionは、意味が定まった型付きBindingを記述します。旧v1の汎用`externalServices`枠を
自動的に引き継ぎません。外部サービスを使う場合は、アプリが使うネットワークAPI、設定、秘密の受け渡しを
明示し、資源名だけで未定義の接続方式が利用できると仮定しないでください。

## 既存Resourceを引き継ぐ

Form URLはResourceの作成後に変更できません。既存UIDの`form`だけを書き換えることも、
v1のUIDを新しいAPIで使えばそのまま引き継げると仮定することもできません。
移行は対象Hostとクライアントの手順に従います。

移行前には、元のFormとstate、背後の資源、参照元、保持すべきデータを確認します。
新しいResourceの作成やimportが必要か、既存の背後資源を二重所有しないか、接続を切り替えた後に
何を削除できるかを、Hostが明示する必要があります。このサイトは特定Hostの移行成功を保証しません。

## 公開状態を読み分ける

v1側の「選択中のソース」は、署名・公開済みの集合と同じとは限りません。Actor/Workflow等には未公開の後継候補があります。
新しいv2本文も、リポジトリに存在すること、URLで正式公開されること、Hostが実装することは別です。
`support`の結果と、実際の操作・復旧が検証された範囲を混同しません。

共通の差分は[TakoformのMigration](https://takoform.com/spec/host-api/v2/migration)、
v1のpackageと履歴は[既存のForm一覧](/v1/)と
[publisher repository](https://github.com/tako0614/takoform-forms)から確認できます。
