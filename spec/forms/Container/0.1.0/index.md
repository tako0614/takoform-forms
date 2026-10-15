---
title: Container 0.1.0
description: digest固定OCI imageのprivate HTTPサービスとWorker Container Binding
formUrl: https://edge.forms.takoform.com/forms/Container/0.1.0/
hostApi: forms.takoform.com/v2
---

# Container 0.1.0

Containerは一つのlogical private HTTPサービスを管理する。このResourceのUIDは実行process、
Docker container ID、Cloudflare Durable Object IDではない。旧未公開ContainerServiceの
image・health・更新・復旧をHost API v2へ引き継いだ新しい契約であり、旧FormRefやstateと
自動変換しない。以下のInterface **container.http** とBinding **module-worker.container**
を同じ版固定本文で定義する。これらの名前はこの未公開案での選択である。

## 入力 {#input}

`spec`は次のキーだけを持つオブジェクト。未知キー、`null`、型変換、JSON重複キーを拒否する。
長さはUnicodeスカラー値で数え、不正なUnicode文字列も拒否する。

| フィールド | 必須・既定 | 意味と上限 |
| --- | --- | --- |
| `image` | 必須 | 明示registry/repositoryに`@sha256:`と64桁の小文字hexを付けたOCI image参照。最大512文字。scheme、userinfo、query、fragment、tagのみの参照を拒否。任意tagはdigestより前だけに置ける。 |
| `httpPort` | 必須 | image内のHTTP listener。整数1〜65535。public portの割当ではない。 |
| `healthPath` | 必須 | GET readiness probeのabsolute path、1〜2048文字。`/`または空でないsegment列。query/fragment/backslash/control/空segmentを拒否。percent escapeは正しいUTF-8で、一度decodeしたsegmentに`/`、backslash、control、`.`、`..`を含めない。 |
| `workloadRevision` | 必須 | 利用者が宣言する実行構成のrevision。`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`。Host UID/generationの代わりにはならない。 |
| `environment` | 任意、`{}` | 非秘密string map、最大64キー。名前は`^[A-Za-z_][A-Za-z0-9_]{0,63}$`。各値は4096文字以下、NULなし。`CLOUDFLARE_`、`TAKOFORM_`で始まる名前は予約し、全backendで拒否する。 |
| `requiredSensitiveVars` | 任意、`[]` | 名前だけの宣言欄。名前文法は`^[A-Z][A-Z0-9_]{0,63}$`、重複・environmentとの衝突を許さない。この初期版は秘密注入を提供せず、**非空配列を拒否する**。値をこの欄へ置けない。 |
| `outboundInternet` | 任意、`false` | `true`を明示した場合だけ実行processの外向きInternet通信を許可する。課金・容量・全宛先への到達性は保証しない。 |

imageのregistryは小文字のDNS名（各labelは英数字で始まり終わり内部hyphen可）または
`localhost`、任意のportは1〜65535。repositoryは小文字英数字で始まる空でないsegment列で、
各segmentは`[a-z0-9]+(?:[._-][a-z0-9]+)*`。tagは`[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}`。
healthPathのraw segmentは`[A-Za-z0-9._~!$&'()*+,;=:@-]`または`%HH`の列である。
ここでcontrolはU+0000〜001FとU+007F。非ASCII文字はUTF-8をpercent-encodeする。
image digestは単一のlinux/amd64 OCI manifestを識別する。multi-platform indexのdigest、
違うplatform、取得bytesがdigestと異なるimageを拒否する。registry認可・manifest/layer検証・
retentionはHostの責任で、URLやdigestを知るだけでは取得権限を得ない。

```json
{
  "image": "ghcr.io/example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "httpPort": 8080,
  "healthPath": "/health",
  "workloadRevision": "revision-1",
  "environment": { "APP_MODE": "production" },
  "outboundInternet": false,
  "requiredSensitiveVars": []
}
```

例は説明用であり、imageの存在やHostのsupportを示さない。`privateInputs`は省略または
空objectだけを許す。environmentは公開specとして保存・readbackされるので秘密を置かない。
値から秘密を自動検出する契約はない。秘密注入・rotationは後続のForm URLで定義する。

同一spec比較では任意の既定値を展開し、objectのキー順を無視する。stringはUnicode正規化せず
同じスカラー列で比較する。`workloadRevision`も比較対象で、同じrevision文字列でも他の
設定が変われば変更である。Formの既定値展開は共通APIのreplay fingerprintとは別である。
original requestの省略・明示の違いを勝手に同一再送として扱わない。

## Interface: container.http {#interface}

一つの選択済みserving executionへprivate HTTP要求を送るInterfaceである。callerは
instanceやnative URLを選ばない。Hostは新規callのroute選択を切替と直列化し、call開始時に
一つのhealthy serving generationへ固定する。同じUIDでもapplication dataの整合性や
request間のprocess affinityは保証しない。runtime filesystem、memory、process identityは
ephemeralで、再作成やscale-to-zeroをまたぐ永続性はない。永続dataは別サービスが所有する。

実行はimageのENTRYPOINT/CMDを使う。任意exec、shell、start/stop、port選択、native DO
storageへの操作はこのInterfaceに含めない。scale-to-zero後のcallは同じ保持済み構成を起動し、
health確認後に配送する。準備できなければ`backend_unavailable`であり、別imageへfallbackしない。

## Worker Binding: module-worker.container {#worker-binding}

[WorkerVersion 0.6.0](../../WorkerVersion/0.6.0/)の
`containerBindings:[{"name":string,"resource":{"resourceUid":string}}]`に宣言する。
省略は`[]`、最大64件。名前はJavaScript identifier文法
`^[A-Za-z_$][A-Za-z0-9_$]{0,63}$`、vars・秘密名・他Bindingを含むenv全体で重複禁止。
resource参照は非空UID stringだけを持つ閉じたobject。同じHost・Space、正確なこのForm URL、
callerの参照利用権限を副作用前に確認する。名前や最新URLで再解決せず、削除・再作成後の
別UIDへ付け替えない。WorkerVersion UIDとtarget UIDへscopeした能力であり、UIDそのものは
認可を与えない。実行中の認可取消は新規callを拒否し、既存callもHostが取消して隔離する。

アプリの唯一のメソッドは`env.APP.fetch(request: Request): Promise<Response>`。

```javascript
export default {
  async fetch(request, env) {
    return env.APP.fetch(request);
  }
};
```

このBindingへ渡すRequestはHTTP(S) URL、methodはGET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS。
URLのauthorityは配送先を選ばない。URLのpathnameとqueryをそのまま対象httpPortへ渡す。
userinfo、fragment、backslash、path/query内のcontrol、8192文字を超えるpath+queryを拒否。
WebSocket/HTTP upgradeは提供せず、Upgrade要求は`invalid_request`で拒否する。
GET/HEADの要求bodyはない。入力は未消費のRequestだけで、取消済みsignalは配送前に拒否する。
callerがResponseから新しい要求を作る判断と、Hostのtransparent retryは別である。

### Streaming、headers、error

- Request/Response bodyは双方stream、backpressureを伝え、全量を先読みしない。Promiseは
  最終response headを受けた時点でresolveし、applicationの4xx/5xxも通常Responseとして返す。
  3xxもそのまま返し、Locationへ追従しない。101やupgrade、malformed headはhead前の失敗。
- 各方向のbody上限は104857600 bytes。既知の過大requestは配送前、既知の過大responseは
  head公開前に拒否。未知長のoverflowは部分配送後でもstreamをabortし、再送を許可しない。
- 各方向headerはFetch Headersで正規化したentryが最大64、各name/valueのsingle-byte長と
  `: `、CRLFの4 bytesの和が最大16384 bytes。ASCII tokenのname、HTABとU+0020〜007E/
  U+0080〜00FFのvalueだけを許す。CR/LF/NULを拒否。Set-Cookieは`getSetCookie()`で別entryを
  保持する。他のduplicate fieldのcasing/order/raw wire表現はFetchのnormalizationに従う。
- 上限はtransport header除去前に検査する。Host、Connection、Keep-Alive、Proxy-Authenticate、
  Proxy-Authorization、TE、Trailer、Transfer-Encoding、UpgradeとConnectionが指名する
  headerを除去し、native宛先用HostはHostが作る。application Authorizationは保持する。
  ConnectionがContent-Lengthを指名する要求はinvalid_requestとする。
- Content-Lengthはapplication headerとして転送せず、単一のcanonical非負10進integerを
  safe integer範囲で検証してtransport framingに使う。duplicate/comma/不正値/矛盾を拒否する。
  宣言bytesと実際のcountが異なればabortする。未知長を調べるためにbufferしない。
  native responseの不正framingはhead前のbackend_unavailableである。
- response statusは200〜599。HEAD/204/205/304はbodyなしで、これらのContent-Lengthは
  representation lengthを示し得るためpayload countと比較せずcallerへ投影しない。
  他のresponseには0 bytesでもstreamがあり、既知長0と未知長を混同しない。
- caller signalのabortはuploadとbackend I/Oを取消する。Response bodyのcancelもbackend
  streamを取消する。backendが早期responseを返した場合は未完uploadを取消し、配送済みstatusを
  置き換えない。request失敗/取消時はまだ開いているresponse bodyもerrorにする。
- 配送が不明なapplication要求を**いかなるmethodでも**別executionへ自動retryしない。
  response head後のreset/overflow/count mismatchは公開したbodyのerrorであり、HTTP statusの
  置換や成功した空Responseにしない。管理Operationの再送規則をapplication I/Oへ流用しない。

reject/body errorは`Error.name:"ContainerError"`、`code`は以下の閉じた語彙。messageとcodeへ
入力値、native診断、credentialを含めない。head後は既に渡したPromiseをrejectできないので
bodyのreadが同じErrorで失敗する。

| code | 条件 |
| --- | --- |
| `invalid_request` | 入力形、method/URL/header/framing/upgrade違反。head前。 |
| `backend_unavailable` | 認可取消、healthy routeなし、起動不能、接続/head不正。head前。 |
| `request_aborted` | caller取消、upload count不一致、要求stream失敗。head前はreject、head後は残存body error。 |
| `request_too_large` | request bytes上限超過。同じ前後のerror境界。 |
| `response_aborted` | head後の接続reset、response count不一致、response stream失敗、drainでの取消。 |
| `response_too_large` | response上限超過。既知長はhead前reject、未知長はhead後body error。 |

## 観測とoutput {#state}

`observed`は未観測なら`{}`。観測後は`ready:boolean`が必須、serving executionが確立した後だけ
`servingGeneration`（v2共通generationと同じ1〜9007199254740991の整数）と`imageDigest`
（`sha256:`+64小文字hex）を**両方**持つ。初回失敗時は`{ready:false}`。他のキーは禁止。
`ready:true`はその観測で選択先のhealth GETが2xxで、配送経路が確認できた意味。
未知/timeoutはfalseまたは最後の時点付き観測で示し、freshなtrueとしない。cold idleでは
保持構成があっても、起動後のhealthを確認するまでready:falseを記録する。

更新失敗時に古いexecutionがhealthyならready:true/古いservingGenerationを保持できる。
common observedGenerationは新desired generationを実現するまでadvanceしない。
observedAtは実際の確認時点で、GET時刻ではない。outputは常に`{}`。native IP/port/ID、
credential、秘密、公開URLを返さない。management phase:idleやOperation成功はSLAではない。

## CRUD、切替、復旧、所有 {#lifecycle}

共通[Resource/Operation/replay/error](https://takoform.com/spec/host-api/v2/http)
の全create/read/update/deleteが必須。Offeringを持つHostは選択とrevisionを明示的に確認し、
capacity/placement/native instance size/料金をそこで定義する。Form URL/Space/name/Offering/UIDを
普通のPUTで移行しない。

CREATEはexact image/構成を取得・検証・保持し、owned executionを起動、httpPortのhealthPathへ
GETし、2xxを確認してからservingへ選ぶ。probeはredirectを追わず、そのbodyは破棄する。
portがlistenするだけ、native start/runningだけをhealth成功としない。
起動・probeの期限とcapacityはHostがOffering等で公開し、期限切れから効果なしと推定しない。

GETは同UIDの管理記録・spec・最後の観測を返し、実行process消失を404に置換しない。
GETから新しいOperationを作らない。background recoveryは元の所有記録を使う。

PUTは全specを置換する。全設定を変更でき、workloadRevisionだけの変更も新構成。
新Idempotency-Keyの同一spec PUTも新Operation/generationを作り再照合する。
旧executionを壊す前に新構成のhealthを確認し、routeを一度に切り替える。health失敗なら
旧servingを保ち、失敗したdesired generationを実現済みにしない。new callsは切替後のexecution、
既存callは選択済みexecutionへ固定する。切替から**30000ms**以内に旧callを完了または取消し、
残存I/Oとowned processの物理退役を確認する。部分的なrouteや別imageへのfallbackを公開しない。

DELETEは生きたWorkerVersionのcontainerBindings（未選択Version、pending作成も含む）が
このUIDを参照する間、外部効果前に`409 dependency_conflict`で拒否する。参照消失後は
新規配送を閉じ、既存I/Oを取消して30秒以内の退役を進め、owned executions、private route、
Host保持imageのこのResource分の参照を除く。共有imageを他Resourceの参照中に消さない。
独立したdata/Worker/Binding元を連鎖削除しない。退役・所有物cleanupの確認前に成功にしない。

Hostは副作用前にUID、Operation、generation、workloadRevision、exact imageとbackend所有記録を
永続化する。lost ACK/process restart/timeout後はその記録と実行先を照合し、不明なら
`effect:unknown`/`status:reconciling`の非terminal Operationを保つ。別名/別UIDへ作り直して
不明な効果を隠さない。partial stateは同UIDの記録に残し、terminal失敗後の明示PUT/DELETEで
forward repairする。known absentを確認したowned executionの削除は成功し得るが、timeoutは
absenceの証拠ではない。管理復旧はapplication要求の再実行を許可しない。

拒否は共通APIのinvalid_spec/dependency_conflict等を使う。受理後のForm error.codeは
`image_unavailable`、`image_invalid`、`startup_failed`、`health_failed`、`cleanup_failed`だけ。
未知効果をfailedにせずreconcilingへ保持する。diagnosticsはpayload-freeで、raw native responseを
公開しない。実行dataのreverse migrationはなく、失敗更新は旧serving保持または新PUTで修復する。

## backend対応と適合 {#conformance}

hostedはCloudflare ContainersをHost所有DO executor/broker経由で実行する。self-host Dockerと
Linux/systemd上のDocker Engineも、同じlinux/amd64 image、spec、UID、Bindingを使う。
裸のhost processをOCI imageとして扱わない。daemon-free OCI runtimeは別backend実装。
どのbackendもtenant隔離、owned network/process、false時のInternet拒否、同じhealth/cutover/
stream/cancel/authority semanticsを満たす。true時にもnative管理planeと他tenantへ無条件の
アクセスを許さず、Hostの認可・network policyを維持する。

Cloudflareのdurable_object policyのnamed image catalogは1 applicationあたり100 entriesまで。
これはこのFormのResource数やDocker/Linuxの共通上限ではない。HostがOffering/capacity admissionで
staging、current/old serving、drain/recoveryのimage slotsを公開・予約し、満杯なら効果前に
拒否する。registryへ事前copyしても元のmanifest/layerのdigestを検証し保持する。image map更新
だけで稼働instanceが新imageへ切り替わったと判断しない。native scheduling policy、DO namespace、
account credentialをアプリへ渡さない。

上記Cloudflare制約の参考は[Scheduling policies](https://developers.cloudflare.com/containers/configuration/scheduling-policy/)、
[linux/amd64](https://developers.cloudflare.com/containers/get-started/)、
[Container API](https://developers.cloudflare.com/containers/api/durable-object-container/)。
この本文が共通仕様の正本で、native製品の追加機能はportable caller APIを拡張しない。

適合はspec形だけでなく、healthy create、failed updateでの旧serving保持、healthy cutover、
30秒drain、lost ACKと実Host process再起動、same UID repair、bound deletion拒否、tenant認可、
stream/backpressure/cancel、head前後error、framing、無retry、false/true egressを実backendで
測る。ローカルfixtureやsource公開だけをCloudflare/Docker/Linuxのsupport証拠にしない。
現Takoserver候補はsecret/true egress/Worker bindingが未対応であり、このURLのHost supportを
今回のpublisher変更から宣言しない。
