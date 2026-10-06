---
title: TerraformとOpenTofuで使う
description: Form仕様とProviderの役割、対応する版、他のProviderとの組み合わせを確認する。
---

# TerraformとOpenTofuで使う

Terraform/OpenTofuでは、ProviderがHCLの入力をHost APIへ変換し、Resource UIDと状態を管理します。
Formは資源の意味を定めますが、HCLのresource名、引数名、import形式を定めません。
同じFormに対して別のProviderを作ることもできます。

## この作者のProvider

[`tako0614/takoform`](https://registry.terraform.io/providers/tako0614/takoform/latest/docs)は、
この作者が選んだFormを扱うTerraform/OpenTofu Providerです。すべてのTakoform対応Hostや
すべての作者のFormを自動的に扱う汎用Providerではありません。特別な「公式Provider」の権限もありません。

導入コマンド、接続設定、各resourceのHCL、importとアップグレードは
[Providerのドキュメント](https://registry.terraform.io/providers/tako0614/takoform/latest/docs)を使います。
**この仕様整備では、同Providerのv2対応は実装・検証していません。**
新しいForm URLを既存の引数に渡すだけで動くとは案内しません。

## 対応版を確認する

実行する前に、以下をそれぞれ確認します。

1. 使用するProviderのreleaseが対応するHost APIの版。
2. Providerが扱う正確なFormのURLまたはv1のFormRef。
3. 利用するHostの同じFormへの対応と、Space・Offeringの利用条件。
4. 既存stateがある場合、そのProvider版が定める更新・import・移行方法。

Providerの版を更新しても、APIやFormの意味は変わりません。逆に、新しいFormを公開しても
Providerが自動的に対応するわけではありません。利用するProviderは再現できる版へ固定し、lock fileも保存します。

## 複数のProviderを組み合わせる

一つのOpenTofu構成で、Workerにはこの作者のProvider、DNSには別のProvider、外部データベースには
そのサービスのProviderを使えます。Takoform以外のProviderと併用するための専用の仲介層は要りません。
値の接続はHCLの参照で表し、資格情報と削除範囲は各Provider・各資源の仕様に従います。

たとえばカスタムドメインを用意する構成でも、DNSゾーンを管理する権限とWorkerを公開する権限は別です。
参照にドメイン名を書いただけでDNSの所有権は移りません。具体的な設定例は対応版のProvider資料で確認します。

## Providerを実装する場合

[Host API v2](https://takoform.com/spec/host-api/v2/http)と、対象とする個別Formの仕様を実装します。
FormのURLを実行時に取得して、HCL schemaやコードを自動生成・実行することは必須ではありません。
再送キー、Operation、世代、秘密入力の省略・補給、削除後の状態を、Provider自身の永続stateと整合させます。
認証方式や提供条件はHostのものを使い、API共通仕様に作者固有の特権を足しません。
