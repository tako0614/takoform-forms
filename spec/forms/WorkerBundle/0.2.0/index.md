---
title: Worker Bundle
description: Module Workerの実行に使う、検証済みの不変ファイル集合
formUrl: https://edge.forms.takoform.com/forms/WorkerBundle/0.2.0/
hostApi: forms.takoform.com/v2
---

# WorkerBundle 0.2.0

## 1. 目的と範囲

このFormは、一つのModule Worker Versionが参照する、検証済みの不変なファイル集合を表す。Hostは
manifestと各ファイルを取得・検証・保持するが、Workerの作成、公開、実行、経路設定、稼働判定は
行わない。Formの識別子は上記のURL文字列そのものであり、Takoform Host API v2を使う。

Hostはcreate、read、update、deleteの全操作を実装し、この契約全体に適合するときだけ対応済みと
宣言する。`supported: true`は技術的対応を表すだけで、取得元へのアクセス許可や容量予約ではない。
Hostは個別の取得をネットワーク・資源ポリシーにより拒否できる。Offeringを公開する場合、その件数・
サイズ・取得範囲はHostの実際の上限を超えて約束してはならない。

module specifier解決、Worker実行ABI、file media typeの意味は[ModuleWorker 0.3.0の`modules`節](../../ModuleWorker/0.3.0/#modules)が所有する。
このFormはmanifestが宣言するバイト列とmedia typeの目録だけを定め、module解決方法を重複して定義しない。
manifestはWorker payloadの目録であり、Form仕様の配布物ではない。HostがForm仕様を取得・実行する必要はなく、
package、署名、Coreを利用要件としない。

## 2. 入力と既定値

公開 `spec` は次のキーだけを持つ。

```json
{
  "artifact": {
    "url": "https://artifacts.example.invalid/build/manifest.json",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }
}
```

`artifact`は必須で既定値がなく、作成後は変更できない。`url`はmanifestの絶対HTTPS URLで、ASCII
8192文字以下とする。userinfo、query、fragmentを含めてはならない。公開者はURLのどの部分にも認証情報などの
秘密を置いてはならない。Hostはuserinfo、query、fragmentのあるURLを拒否し、呼出者のcredentialを転送せず、
URLを通常ログへ出さない。Hostが任意のopaque pathから秘密を確実に判定できるとは限らないため、必要な場合は
追加のorigin/path policyで取得を制限する。`sha256`は接頭部分のない小文字16進64桁で、manifestのHTTP応答
本文の厳密なbyte列に対するSHA-256である。異なるURLまたはdigestに変える場合は新しいResourceを作る。

manifestはUTF-8のJSON objectで、未知のfieldと重複keyを拒否し、次のfieldだけを持つ。

```json
{
  "entrypoint": "src/index.mjs",
  "files": [
    {
      "path": "src/index.mjs",
      "url": "https://artifacts.example.invalid/build/index.mjs",
      "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
      "mediaType": "application/javascript+module"
    }
  ]
}
```

manifestの最大長は1 MiB。`entrypoint`は必須で、`files`の一要素の`path`と完全一致し、その要素の
`mediaType`は `application/javascript+module` でなければならない。`files`は必須の順序付き配列で、
要素数は1〜512。各要素はちょうど`path`、`url`、`sha256`、`mediaType`を持ち、全fieldが必須で既定値は
ない。各ファイルは最大16 MiB、全ファイル合計は最大128 MiB。ファイルの`sha256`も接頭部分のない小文字16進
64桁で、応答本文の厳密なbyte列を検証する。

`path`は空でない相対POSIX pathで、UTF-8表現が1024 bytes以下とする。先頭・末尾の`/`、backslash
(U+005C)、`?`、`#`、制御文字 (U+0000..U+001F、U+007F)、空segment、`.`または`..` segmentを含めて
はならない。比較はUnicode文字列の完全一致で大文字小文字を区別し、重複pathを拒否する。fileごとの`url`も
ASCII 8192文字以下の絶対HTTPS URLで、userinfo・query・fragmentなしとする。
`mediaType`は[ModuleWorker 0.3.0の`modules`節](../../ModuleWorker/0.3.0/#modules)が定める閉じた集合から選ぶ。この契約にHost既定値はない。
取得元のHTTP `Content-Type`は判定に使わず、manifestの`mediaType`を唯一の値とする。

## 3. 秘密入力

秘密入力はない。公開者は`spec`、manifest、URLにcredential、access token、署名付きURLなどの秘密値を含めては
ならない。Hostはuserinfo、query、fragmentを含むURLを拒否し、呼出者のcredentialを転送せず、URLを通常ログへ
出さない。Hostが任意のopaque pathに秘密があるかを完全判定できるとは限らず、必要なら追加の取得元policyを適用する。
`privateInputs`は省略または空objectだけを受け付ける。

## 4. 観測状態、output、利用可能状態

未検証の`observed`は `{}`。検証後の`manifestSha256`は小文字hex 64桁の文字列、`fileCount`は1〜512の整数、
`totalBytes`は0〜134217728の整数、`entrypoint`はmanifestのpath文字列と完全一致する。`files`はmanifestと同じ
順序の配列で、各要素は正確に`path`（文字列）、`sha256`（接頭部分のない小文字hex 64桁）、`mediaType`
（文字列）、`byteSize`（0〜16777216の整数）を持つ。各pathとmediaTypeはmanifest値と完全一致し、`fileCount`は
配列長、`totalBytes`は`byteSize`の合計と一致する。URLは含めない。全byteのdigest照合と永続保持が済むまで、
検証済みとして観測してはならない。`output`は常に `{}`。

管理操作の成功が意味するのは、宣言されたmanifestとバイト列を検証しHostが保持したことだけである。Worker Versionが
このBundleを使うこと、Workerが実行中であること、通信可能であることを保証しない。取得途中の部分一覧を
検証済みとして返さず、失敗・中断時もResourceとOperationをHost API v2の規則に従って保持する。

## 5. 操作と失敗

- **create:** specとmanifestを検査し、全ファイルを取得・digest照合する。manifestと厳密に一致したバイト列を
  Host管理の不変な保持領域へ永続保存してから成功させる。digestを保持物の識別子に使い、応答不明や再起動後も
  同じ識別子へ別内容を割り当てない。
- **read:** Resourceと最後に完了した観測を返す。取得元から再取得したり、新しい検証Operationを始めたりしない。
- **update:** 更新後の`spec.artifact`はURL文字列とdigest文字列が完全一致する同一値でなければならない。
  JSON objectのkey順は比較に影響しない。値の変更は副作用前に拒否する。同一specのPUTはHost API v2通常の
  generationとOperationを作り、同じ保持物が永続的に存在することを再照合するが、内容や識別子を変えない。
- **delete:** 削除完了前のWorker Version Resourceから参照中なら `409 dependency_conflict` とし、Resourceや保持物を変更しない。
  参照がなくなった後、このResourceだけが所有する保持物を解放する。取得元や他Resourceと共有するblobを削除せず、
  Worker VersionやDeploymentを変更・rollbackしない。

Hostは入力URLだけを根拠に取得権限を得ない。バイト列の解決方法は、(1)呼出者が利用を認可されたHost内の検証済み
永続保持物、(2)公開HTTPSからの取得、(3)Host運用者が事前設定した正確なprivate origin / network allowlistと
Host保持認証のいずれかとする。接続時は選択policyの許可先を確認し、実接続IPをその許可範囲に固定または照合する。
HTTPS hostnameとTLS証明書検証は維持する。許可されないprivate、loopback、link-local、multicast、reserved、
metadata-service addressへの接続を拒否する。userinfo、query、fragmentは認めず、呼出者の認証情報を送らない。
Host保持認証を別originへ転送せず、redirectを追従しない。取得時は`Accept-Encoding: identity`相当を要求し、
identity以外のcontent-encodingを拒否する。SHA-256対象はHTTP transfer framing除去後、文字decode前の応答本文。
HTTP 200以外は失敗。取得には有限のHost timeoutを設定し、manifest 1 MiB、各file 16 MiB、aggregate 128 MiBの
上限を超える前に読み取りを止める。Hostは到達不能またはpolicy不適合の取得元を拒否できるが、別内容で代替しない。
digest不一致、invalid JSON、duplicate key、invalid UTF-8、未知field、不正path、重複path、未対応media typeも
検証失敗である。このFormはartifact転送endpointやcredential APIを定義しない。

HostはResourceまたは参照先が必要とする限り、検証済みmanifestとファイル、または同等のcontent-addressed
永続保持を維持する。再起動、応答喪失、取得元消失後も同じ識別子と検証状態を回復できなければならない。
digest一致blobの存在だけで利用を認可せず、保持物を再利用する前に当該Resourceの所有権・認可を確認する。他ownerが
参照する共有blobをGCしてはならない。不完全な保持物はHost API v2に従って失敗または`reconciling`とし、同じ識別子へ
別バイト列を割り当てない。同じrequestと元のIdempotency-Keyは元Operationを
返し、新しいkeyでの同一spec PUTは通常のgeneration/Operation手順を通る。

## 6. 参照、所有、削除

このFormが所有するのはResourceと、それが作成したartifactの保持物だけである。WorkerVersionは正確なHost
Resource UIDでこのBundleを参照できる。参照は取得元URLや他Resourceのバイト列への権限を付与しない。削除完了前の
Worker Version Resourceから参照があれば削除を拒否する。参照消失後も、このResource専用の保持物だけを解放し、
他の所有者が参照する共有バイト列は残す。取得元、database state、Worker Deploymentは削除・rollbackしない。

## 7. InterfaceとBinding

必要・提供するInterface、Bindingはない。Hostのartifact取得能力は実装側の責任であり、このResourceがportableな
通信権限を与えるわけではない。検証時にWorker codeを実行してはならない。

## 8. 未知fieldと拡張

`spec`、`artifact`、manifest、file objectに未知のフィールドがあれば拒否する。未知の`privateInputs`も拒否する。
`files`の順序は保持するが、Worker module解決の順序は定めない。拡張用フィールドはない。構造、上限、media type、
lifecycleの意味を変える場合は新しいForm URLを使い、既存v1 Formのバイト列や意味を変更しない。
