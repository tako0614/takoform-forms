---
title: Static Asset Bundle
description: Worker Versionに添付できる検証済みの不変な静的ファイル集合
formUrl: https://edge.forms.takoform.com/forms/StaticAssetBundle/0.2.0/
hostApi: forms.takoform.com/v2
---

# StaticAssetBundle 0.2.0

## 1. 目的と範囲

このFormは、Worker Versionに添付できる検証済みの不変な静的ファイル集合を表す。Hostはbytesの検証と
保持を行うが、Workerを公開したり、assetの探索順・not-found時の挙動を決めたり、コードを実行したり、
assetへの到達性を保証したりしない。Serving policyはWorker Versionが所有する。識別子は上記の完全な
Form URLであり、Takoform Host API v2を使う。

Hostはcreate、read、update、deleteの全操作を実装し、この契約全体に適合するときだけ対応済みと
宣言する。`supported: true`は技術的対応を表すだけで、取得元へのアクセス許可や容量予約ではない。
Hostは個別の取得をネットワーク・資源ポリシーにより拒否できる。Offeringを公開する場合、その件数・
サイズ・取得範囲はHostの実際の上限を超えて約束してはならない。
manifestは静的asset payloadの目録であり、Form仕様の配布物ではない。HostがForm仕様を取得・実行する必要はなく、
package、署名、Coreを利用要件としない。

## 2. 入力と既定値

公開 `spec` は次のキーだけを持つ。

```json
{
  "artifact": {
    "url": "https://artifacts.example.invalid/site/manifest.json",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }
}
```

`artifact`は必須で既定値がなく、作成後は変更できない。`url`はmanifestの絶対HTTPS URLでASCII
8192文字以下とする。userinfo、query、fragmentを含めてはならない。公開者はURLのどの部分にも認証情報などの
秘密を置いてはならない。Hostはuserinfo、query、fragmentのあるURLを拒否し、呼出者のcredentialを転送せず、
URLを通常ログへ出さない。Hostが任意のopaque pathから秘密を確実に判定できるとは限らないため、必要な場合は
追加のorigin/path policyで取得を制限する。`sha256`は接頭部分のない小文字16進64桁で、manifestのHTTP応答
本文の厳密なbyte列に対するSHA-256である。異なるURLまたはdigestに変える場合は新しいResourceを作る。

manifestはUTF-8のJSON objectで、未知のfieldと重複keyを拒否し、`files`だけを持つ。

```json
{
  "files": [
    {
      "path": "assets/app.css",
      "url": "https://artifacts.example.invalid/site/app.css",
      "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
      "mediaType": "text/css"
    }
  ]
}
```

manifestの最大長は1 MiB。`files`は必須の順序付き配列で要素数は1〜512。各要素はちょうど`path`、`url`、
`sha256`、`mediaType`を持ち、すべて必須で既定値はない。各fileは最大16 MiB、全fileの合計は最大128 MiB。
`path`は空でない相対POSIX pathでUTF-8表現が1024 bytes以下とする。先頭・末尾の`/`、backslash (U+005C)、
`?`、`#`、制御文字 (U+0000..U+001F、U+007F)、空segment、`.`または`..` segmentを含めてはならない。
比較はUnicode文字列の完全一致で大文字小文字を区別し、重複pathを拒否する。

fileごとの`url`はASCII 8192文字以下の絶対HTTPS URLで、userinfo、query、fragmentを含めてはならない。
`sha256`は接頭部分のない小文字16進64桁で、HTTP応答本文の厳密なbyte列を検証する。`mediaType`はRFC 9110
のparameterなしmedia type構文に適合しなければならない。配信時には宣言値を使うが、このフィールドは配信routeや
WorkerへのBindingを定義しない。Hostの既定値はない。
取得元のHTTP `Content-Type`は判定に使わず、manifestの`mediaType`だけを値として使う。

## 3. 秘密入力

秘密入力はない。`spec`、manifest、URLにcredential、access token、署名付きURLなどの秘密値を含めては
ならない。Hostはuserinfo、query、fragmentを含むURLを拒否し、呼出者のcredentialを転送せず、URLを通常ログへ
出さない。Hostが任意のopaque pathに秘密があるかを完全判定できるとは限らず、必要なら取得元policyを加える。
`privateInputs`は省略または空objectだけを受け付ける。

## 4. 観測状態、output、利用可能状態

未検証の`observed`は `{}`。検証後は `manifestSha256`（小文字hex 64桁の文字列）、`fileCount`（1〜512の整数）、
`totalBytes`（1〜134217728の整数）、`files`を含める。`files`はmanifestと同じ順の配列で、各要素は正確に
`path`（文字列）、`sha256`（接頭部分のない小文字hex 64桁）、`mediaType`（文字列）、`byteSize`（0〜16777216の
整数）を持つ。`fileCount`は配列長、`totalBytes`は`byteSize`の合計と一致する。取得URLは含めない。全byteのdigest
照合と永続保持が済むまで検証済みと報告してはならない。`output`は常に `{}`。

管理操作の成功が示すのは、宣言されたバイト列をHostが検証・保持したことだけである。Worker VersionがBundleを
参照することや、requestからassetを取得できることは保証しない。取得途中の一覧は検証済みとせず、失敗・中断時も
ResourceとOperationをHost API v2の規則に従って保持する。

## 5. 操作と失敗

- **create:** specとmanifestを検査し、全fileを取得・digest照合する。manifestと厳密に一致するバイト列をHost管理の
  永続保持領域へ保存してから成功させる。digestを保持物の識別子に使い、応答不明や再起動後も同じ識別子へ別内容を
  割り当てない。
- **read:** Resourceと最後に完了した観測を返す。取得元から再取得したり、新しい検証Operationを始めたりしない。
- **update:** 更新後の`spec.artifact`はURL文字列とdigest文字列が完全一致する同一値でなければならない。
  JSON objectのkey順は比較に影響しない。変更値は副作用前に拒否する。同一spec PUTはHost API v2通常のgeneration
  とOperationを作って保持物を再照合するが、バイト列や識別子を変えない。
- **delete:** 削除完了前のWorker Version Resourceから参照中なら `409 dependency_conflict` とし、Resourceや保持物を変更しない。
  参照がなくなった後、このResourceだけが所有する保持物を解放する。取得元や他ownerの共有blobを削除せず、Worker
  Versionの配信方針やDeploymentを変更しない。

Hostは入力URLだけを根拠に取得権限を得ない。バイト列の解決方法は、(1)呼出者が利用を認可されたHost内の検証済み
永続保持物、(2)公開HTTPSからの取得、(3)Host運用者が事前設定した正確なprivate origin / network allowlistと
Host保持認証のいずれかとする。接続時は選択policyの許可先を確認し、実接続IPを許可範囲内へ固定または照合する。
HTTPS hostnameとTLS証明書検証は維持する。許可されないprivate、loopback、link-local、multicast、reserved、
metadata-service addressへの接続を拒否する。userinfo、query、fragmentは認めず、呼出者の認証情報を送らない。
Host保持認証を別originへ転送せず、redirectを追従しない。取得時は`Accept-Encoding: identity`相当を要求し、
identity以外のcontent-encodingを拒否する。SHA-256対象はHTTP transfer framing除去後、文字decode前の応答本文。
HTTP 200以外は失敗。有限のHost timeoutを設け、manifest 1 MiB、各file 16 MiB、aggregate 128 MiBの上限を超える
前に読み取りを止める。Hostは到達不能またはpolicy不適合の取得元を拒否できるが、別内容で代替しない。digest不一致、
invalid JSON、duplicate key、invalid UTF-8、未知フィールド、不正path、重複path、不正media typeも検証失敗である。
このFormはartifact転送endpointやcredential APIを定義しない。

HostはResourceまたは参照先が必要とする限りmanifestとfile、または同等のcontent-addressed永続保持を続ける。
再起動、応答喪失、取得元消失後も同じ識別子と検証状態を回復できるようにする。digest一致blobの存在だけで利用を
認可せず、再利用前に当該Resourceの所有権・認可を確認する。他ownerが参照する共有blobをGCしてはならない。
不完全な保持物はHost API v2に従って失敗または`reconciling`とし、同じ識別子へ別バイト列を割り当てない。
同じrequestと元Idempotency-Keyは元Operationを返し、新しいkeyによる同一spec PUTは通常のgeneration/Operation
手順を通る。

## 6. 参照、所有、削除

このFormはResourceと、それが作成したartifactの保持物だけを所有する。WorkerVersionはHost Resource UIDを
固定して参照する。参照は取得元URLや別Resourceの保持物を読む権限を与えない。削除完了前のWorker Version Resource
から参照があれば削除を拒否する。参照解消後も、このResource専用の保持物だけを解放し、他の所有者が参照する共有
バイト列は残す。取得元、配信方針、Worker Deployment、外部Resourceを削除・変更・rollbackしない。

## 7. InterfaceとBinding

必要・提供するInterface、Bindingはない。Hostのartifact取得能力はHost実装の責任であり、Resourceがportableな
通信権限を付与するわけではない。

## 8. 未知fieldと拡張

`spec`、`artifact`、manifest、file objectに未知のフィールドがあれば拒否する。未知の`privateInputs`も拒否する。
`files`の順序はmanifestとobservedで保つが、asset探索順は定めない。拡張用フィールドはない。構造、上限、path規則、
lifecycleの意味を変える場合は新しいForm URLを使い、既存v1 Formのバイト列や意味を変更しない。
