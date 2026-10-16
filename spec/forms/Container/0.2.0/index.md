---
title: Container 0.2.0
description: digest固定OCI imageのprivate HTTPサービス、秘密入力とWorker Container Binding
formUrl: https://edge.forms.takoform.com/forms/Container/0.2.0/
hostApi: forms.takoform.com/v2
---

# Container 0.2.0

Containerは一つのlogical private HTTPサービスを管理する。このResourceのUIDは実行process、
Docker container ID、Cloudflare Durable Object IDではない。この版は
[Container 0.1.0](../0.1.0/)の後継であり、秘密入力、OCI image indexのpin、外向き通信の
強制境界、早期responseとuploadの関係を定める。0.1.0のURL・本文・Resourceは変えず、
0.1.0のResourceやstateを自動移行しない。以下のInterface **container.http** とBinding
**module-worker.container** を、このURLの版固定本文として定義する。

## 入力 {#input}

`spec`は次のキーだけを持つオブジェクト。未知キー、`null`、型変換、JSON重複キーを拒否する。
長さはUnicodeスカラー値で数え、不正なUnicode文字列も拒否する。

| フィールド | 必須・既定 | 意味と上限 |
| --- | --- | --- |
| `image` | 必須 | 明示registry/repositoryに`@sha256:`と64桁の小文字hexを付けたOCI image参照。最大512文字。scheme、userinfo、query、fragment、tagのみの参照を拒否。任意tagはdigestより前だけに置ける。digestが指すmanifestまたはindexは下記。 |
| `httpPort` | 必須 | image内のHTTP listener。整数1〜65535。public portの割当ではない。 |
| `healthPath` | 必須 | GET readiness probeのabsolute path、1〜2048文字。`/`または空でないsegment列。query/fragment/backslash/control/空segmentを拒否。percent escapeは正しいUTF-8で、一度decodeしたsegmentに`/`、backslash、control、`.`、`..`を含めない。 |
| `workloadRevision` | 必須 | 利用者が宣言する実行構成のrevision。`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`。Host UID/generationの代わりにはならない。 |
| `environment` | 任意、`{}` | 非秘密string map、最大64キー。名前は`^[A-Za-z_][A-Za-z0-9_]{0,63}$`。各値は4096文字以下、NULなし。`CLOUDFLARE_`、`TAKOFORM_`で始まる名前は予約し、全backendで拒否する。 |
| `requiredSensitiveVars` | 任意、`[]` | 名前だけの宣言欄、最大64件。名前文法は`^[A-Z][A-Z0-9_]{0,63}$`。重複、environmentとの衝突、`CLOUDFLARE_`・`TAKOFORM_`で始まる名前を拒否する。順序は意味を持たず、集合として比較する。値はこの欄へ置かず、[秘密入力](#private-inputs)の`privateInputs`で渡す。 |
| `outboundInternet` | 任意、`false` | `true`を明示した場合だけ、実行processが公開Internetへ接続を開始することを許可する。[外向き通信](#egress)のHost強制境界に従い、課金・容量・全宛先への到達性は保証しない。 |

imageのregistryは小文字のDNS名（各labelは英数字で始まり終わり内部hyphen可）または
`localhost`、任意のportは1〜65535。repositoryは小文字英数字で始まる空でないsegment列で、
各segmentは`[a-z0-9]+(?:[._-][a-z0-9]+)*`。tagは`[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}`。
healthPathのraw segmentは`[A-Za-z0-9._~!$&'()*+,;=:@-]`または`%HH`の列である。
ここでcontrolはU+0000〜001FとU+007F。非ASCII文字はUTF-8をpercent-encodeする。
image digestは、単一のimage manifest（OCI image manifestまたはDocker image manifest
v2 schema 2）か、image index（OCI image indexまたはDocker manifest list）を識別する。
indexの場合、Hostはindexの`manifests`から`platform.os`が`linux`かつ
`platform.architecture`が`amd64`のdescriptorを選ぶ。`platform.variant`は選択に使わず、
該当がちょうど一つでなければ拒否する。複数のamd64 variantを持つimageは、使うmanifestの
digestを直接pinする。選択先がimage manifestでない場合（入れ子のindex等）も拒否する。
platformの最終確認は、pinしたmanifestまたは選択manifestのconfigの`os`と`architecture`で
行い、`linux`と`amd64`でなければ拒否する。他platformや`unknown/unknown`のattestation
descriptorは選択されない。

index内の全descriptor（選択されないものを含む）と、実行するmanifestのconfig・layersの
descriptor digestは、正規の小文字`sha256:`だけを認め、spellingを正規化しない。他の
algorithmのdescriptorを一つでも含むindexも拒否する。index文書とmanifest文書はそれぞれ
4194304 bytes以下とする。取得した各文書とblobのbytesはdigestと一致し、blobのbytes数は
descriptorのsizeと一致しなければならない。

specのdigestを**source digest**、実際に実行するmanifestのdigestを**executable digest**と
呼ぶ。単一manifestのpinでは両者は等しい。Hostはgenerationの効果を起こす前に、source
digestから選んだexecutable digestをそのgenerationの記録に保持し、replay・再起動・drain・
recoveryで選び直さない。source index、実行するmanifest、config、layersは、その
generationの物理退役を確認するまで保持する。

実行するmanifestのconfigとlayersのdescriptor size合計（**選択image bytes**）がHostの上限を
超えるimageは、layersを転送する前に拒否する。index・manifest文書、他platform、attestationは
数えない。上限はHostがOffering等で公開する。registry認可・manifest/layer検証・retentionは
Hostの責任で、URLやdigestを知るだけでは取得権限を得ない。registry credentialはHostが所有し、
specや`privateInputs`で受け取らない。これらのimage判定は取得後に分かるため、受理前の拒否では
なく、受理後のOperation失敗として[下記のcode](#lifecycle)で表す。

```json
{
  "image": "ghcr.io/example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "httpPort": 8080,
  "healthPath": "/health",
  "workloadRevision": "revision-1",
  "environment": { "APP_MODE": "production" },
  "outboundInternet": false,
  "requiredSensitiveVars": ["API_TOKEN"]
}
```

例は説明用であり、imageの存在やHostのsupportを示さない。environmentは公開specとして
保存・readbackされるので秘密を置かない。値から秘密を自動検出する契約はない。

同一spec比較では任意の既定値を展開し、objectのキー順と`requiredSensitiveVars`の順序を
無視する。秘密値はspec比較に含めない。stringはUnicode正規化せず同じスカラー列で比較する。
`workloadRevision`も比較対象で、同じrevision文字列でも他の設定が変われば変更である。
Formの既定値展開と順序の無視は共通APIのreplay比較とは別である。同じIdempotency-Keyで
`requiredSensitiveVars`の順序だけを変えた要求も別の要求（`409 idempotency_conflict`）であり、
original requestの省略・明示の違いを勝手に同一再送として扱わない。

## 秘密入力 {#private-inputs}

秘密値は共通APIの[`privateInputs`](https://takoform.com/spec/host-api/v2/http#private-inputs)
で渡す。名前は`requiredSensitiveVars`が宣言する。各値は1〜16384 Unicodeスカラー値でNULを
含まないstring、全値のUTF-8 bytes合計は65536以下。`privateInputs`の存在（`{}`を含む）には、
Host discoveryとこのURLのsupport応答の両方が`privateInputs:true`であることが必要で、なければ
共通APIの`422 capability_required`になる。objectでない`privateInputs`やstringでない値は共通
envelopeの違反（`400 invalid_request`）である。下表の名前集合・値の違反は効果前の
`422 invalid_spec`である。

**設定済み秘密map**は、最新の受理済みCREATEまたはPUTのgenerationが持つ名前と値の組である。
そのgenerationがhealth失敗等で実現しなかった場合も変わらない。DELETEのgenerationはこのmapを
置き換えず、失敗したDELETEの後のPUTも同じmapと比較する。PUTの名前集合の比較と値の引継ぎは、
serving中のgenerationではなく、常にこのmapに対して行う。

| 要求 | `requiredSensitiveVars` | `privateInputs` | 結果 |
| --- | --- | --- | --- |
| CREATE | 空 | 省略または`{}` | 秘密なし |
| CREATE | 非空 | 宣言名と完全一致する全値 | その値を設定済み秘密mapにする |
| CREATE | 非空 | 省略、不足、余分、範囲外の値 | `invalid_spec` |
| PUT | 設定済み秘密mapの名前集合と同じ | 省略 | 設定済みの値を新generationへ引き継ぐ |
| PUT | 設定済み秘密mapの名前集合の部分集合 | 省略 | 残す名前の値を引き継ぎ、外した名前の値を新generationへ渡さない |
| PUT | 名前を一つでも追加 | 省略 | `invalid_spec` |
| PUT | 任意（空を含む） | 新しい名前集合と完全一致する全値 | 全値を置換する（rotation） |
| PUT | 任意 | 不足、余分、範囲外の値 | `invalid_spec` |

部分mapをmergeとして扱わず、名前ごとの更新はない。共通APIのとおり省略は保持であり、
削除は`requiredSensitiveVars`から名前を外して表す。mapを送ったPUTは、値が以前と同じでも
置換として扱う。値が以前と同じかをHostが判定しても、Resource・Operation・応答の違いとして
表さない。

秘密値は宣言名のまま、そのgenerationのnative process起動environmentにだけ入る。
health probe、Binding要求、Resource、Operation、`observed`、`output`、diagnostics、logへ
値も値由来の比較材料も出さない。WorkerVersion側の秘密やvarsをtarget Containerへ渡さない。

秘密はgenerationに属する。値を置換・削除したPUTは、新しい値で起動した新generationが
health確認を通った後にだけrouteを切り替える。失敗時は旧servingとその値を保ち、失敗した
desired generationを実現済みにしない。各generationの値は、そのgenerationの物理退役を確認する
までcold startと再起動のために保持する。設定済み秘密mapの値は、後に受理したCREATEまたはPUTの
generationが置き換えるまで引継ぎのために保持する。どちらにも当たらなくなった値を破棄する。
成功したDELETEは、退役と所有物cleanupの確認後に、設定済み秘密mapと全generationの値を破棄する。

lost ACK後の再送とrequestの同一性は共通APIに従い、Hostは秘密値を公開せずに比較する。
このOperationの要求で送られた値の一時保持が失効し、必要なstepが未送信と証明できる場合だけ、
同じOperationを`waiting_input`にして元のmap全体を補充させる共通APIの手順を使う。mapを
持たないPUTは`waiting_input`にならない。

Hostが保持秘密を失い、起動に必要な全値を復元できない場合、値を欠いたenvironmentや空値で
起動しない。稼働中executionは退役まで配送を続け得るが、停止後に再起動できない場合は
`observed.ready:false`とし、callは`backend_unavailable`になる。Operationが必要とする値を
routeの切替前に復元できず、`waiting_input`も使えない場合、そのOperationは`secret_unavailable`で
失敗する（effectは`none`または`partial`）。切替後の喪失だけを理由にそのOperationを
`secret_unavailable`で失敗させず、完了済みならその結果を変えない。未完了なら旧generationの
drainと物理退役など残る完了条件を引き続き確認し、切替だけで退役やcleanupを成功としない。
切替後の喪失は、再起動が必要になった時点の`ready:false`と`backend_unavailable`として現れる。
効果が不明なら共通APIどおり`reconciling`を保ち、成功した復旧を報告しない。修復はclientが
全値を送るPUTで行う。

`requiredSensitiveVars`が空の利用は、このFormの正当で完全な利用である。秘密を扱えない
Hostも、他の必須動作を満たせば`privateInputs:false`でsupportを報告でき、非空の宣言を含む
要求を`capability_required`で拒否する。

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

WorkerVersionの`containerBindings:[{"name":string,"resource":{"resourceUid":string}}]`に
宣言する。省略は`[]`、最大64件。名前はJavaScript identifier文法
`^[A-Za-z_$][A-Za-z0-9_$]{0,63}$`で、WorkerVersionのvars・秘密名・他Bindingを含むenv全体で
重複禁止。宣言できるのは、[WorkerVersion 0.8.0](../../WorkerVersion/0.8.0/)のように、
このForm URLを参照先として明記するWorkerVersion版だけである。
[WorkerVersion 0.6.0](../../WorkerVersion/0.6.0/)と[0.7.0](../../WorkerVersion/0.7.0/)は
Container 0.1.0だけを参照先とするため、このURLのResourceを参照できない。
resource参照は非空UID stringだけを持つ閉じたobject。同じHost・Space、正確なこのForm URL、
callerの参照利用権限を副作用前に確認する。名前や最新URLで再解決せず、削除・再作成後の
別UIDへ付け替えない。WorkerVersion UIDとtarget UIDへscopeした能力であり、UIDそのものは
認可を与えない。実行中の認可取消は新規callを拒否し、既存callもHostが取消して隔離する。
Bindingはcallerの秘密、vars、credentialをtargetへ渡さない。

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
  streamを取消する。どちらの場合も、未完のuploadを止めてcallerの要求body streamを取消す。
  response headの受信だけではuploadを終えず、backendが要求bodyを読み続ける間は、head後も
  uploadを配送する。Hostがresponse bodyの終端を観測した時点、またはbodyのないresponseを
  配送した時点でuploadが未完なら、Hostは未送bytesを破棄し、callerの要求body streamを取消す。
  callerが読まないresponseの終端をHostがまだ観測していない間は、uploadを続けてよい。
  このupload取消はerrorではなく、配送済みのstatus、headers、bodyを置き換えない。ただし、
  それより前にHostが観測したupload失敗を成功に置き換えず、そのerrorを公開する。要求body
  streamの取消は、native側のupload処理の物理的な終了を示さない。Hostはその終了を確認する
  まで、drainや退役の確認でそのcallを完了済みとしない。
  request失敗/取消時はまだ開いているresponse bodyもerrorにする。stream bodyのRequestに
  必要な`duplex:"half"`はRequest構築時の指定にすぎず、このBindingの配送順序はこの本文が定める。
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
（そのgenerationのexecutable digest、`sha256:`+64小文字hex）を**両方**持つ。初回失敗時は
`{ready:false}`。他のキーは禁止。index pinでは`imageDigest`がspecのsource digestと
異なり得る。`servingGeneration`が現在の`generation`と等しい場合だけ、現在のspecから
source digestを読める。それ以外はclientが自分の記録で対応付け、`observed`へsource digestを
複製しない。
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

CREATEはexact image（index pinではsource indexと選択manifest）・構成・秘密を取得・検証・
保持し、owned executionを起動、httpPortのhealthPathへGETし、2xxを確認してからservingへ選ぶ。
probeはredirectを追わず、そのbodyは破棄する。
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
秘密の置換・削除も同じ切替に従い、[秘密入力](#private-inputs)のgeneration規則で旧値を扱う。

DELETEは生きたWorkerVersionのcontainerBindings（未選択Version、pending作成も含む）が
このUIDを参照する間、外部効果前に`409 dependency_conflict`で拒否する。参照消失後は
新規配送を閉じ、既存I/Oを取消して30秒以内の退役を進め、owned executions、private route、
Host保持imageのこのResource分の参照を除く。共有imageを他Resourceの参照中に消さない。
独立したdata/Worker/Binding元を連鎖削除しない。退役・所有物cleanupの確認前に成功にしない。
秘密の保持は[秘密入力](#private-inputs)のDELETE規則に従う。

Hostは副作用前にUID、Operation、generation、workloadRevision、source digestとexecutable
digest、秘密の保持参照、backend所有記録を永続化する。lost ACK/process restart/timeout後は
その記録と実行先を照合し、不明なら`effect:unknown`/`status:reconciling`の非terminal
Operationを保つ。別名/別UIDへ作り直して
不明な効果を隠さない。partial stateは同UIDの記録に残し、terminal失敗後の明示PUT/DELETEで
forward repairする。known absentを確認したowned executionの削除は成功し得るが、timeoutは
absenceの証拠ではない。管理復旧はapplication要求の再実行を許可しない。

受理前の拒否は共通APIの`invalid_request`、`invalid_spec`、`capability_required`、
`dependency_conflict`等を使う。受理後のOperationのForm `error.code`は次だけである。

| code | 条件 |
| --- | --- |
| `image_unavailable` | Hostの認可でregistryからimage文書やblobを取得できない。 |
| `image_invalid` | digest・size・文書上限の不一致、indexの選択失敗、非正規のdescriptor digest、platform不一致。 |
| `image_too_large` | 選択image bytesがHostの公開上限を超える。 |
| `secret_unavailable` | 必要な秘密値をrouteの切替前に復元できず、`waiting_input`も使えない。 |
| `startup_failed` | 保持構成でexecutionを起動できない。 |
| `health_failed` | 期限内にhealth GETの2xxを確認できない。 |
| `cleanup_failed` | 退役または所有物cleanupを確認できない。 |

未知効果をfailedにせずreconcilingへ保持する。diagnosticsはpayload-freeで、raw native responseを
公開しない。実行dataのreverse migrationはなく、失敗更新は旧serving保持または新PUTで修復する。

## 外向き通信 {#egress}

`outboundInternet`の省略と`false`では、実行processから外向き接続を開始できない。Hostの
resolverへのDNS問合せも外向き通信として許可しない。Hostが持つのはhealth probeと配送のための
内向き経路だけである。

`true`は公開Internet宛ての接続開始の**許可**であり、到達の保証ではない。Hostは、Hostや
nodeのloopback、private address、link-local、metadata宛先（これらのIPv4-mapped IPv6表記を
含む）、Hostやbackendの管理planeへのprivate経路、他tenantのexecutionとそのprivate経路への
接続を拒否する。公開Internet上の宛先は、他tenantの公開serviceや公開APIであっても通常の
宛先である。この拒否は全protocol・全portに適用し、HTTP(S)だけを選別して他の通信を素通しする
構成を`true`の実現としない。DNS解決の方法はHostが選ぶ。`true`では、Hostが指定するresolverへの
DNS問合せだけは拒否の対象外とし、そのaddressへの他の通信は許可しない。

実行process自身のnetwork namespace内で終わるloopback通信は外向き通信ではなく、`false`でも
妨げない。namespace内のaddressを経由してHostのresolverやserviceへ転送される通信（container
内蔵DNSの転送やDNAT等）は、この例外に含めない。`false`と`true`のどちらでも、拒否は接続ごとの
実効的な接続先に適用し、IP literal、IPv4-mapped IPv6等の別表記、DNS応答の変化や再解決、
redirect、proxyや経路変換を通じて拒否対象へ到達させない。

Hostは公開したpolicyで、濫用防止のための特定の宛先port（例: TCP 25）の拒否、接続数と
外向きbytesの制限・計測、濫用時の既存接続の遮断を定めてよい。これらは公開Internetへの一般的な
到達を特定のprotocolやportだけに置き換えるものではなく、HTTP(S)だけを中継して他の通信を
拒否する構成も`true`の実現としない。Hostは、policyへの適合のためにimageのentrypoint変更や、
Hostの証明書局をimageに信頼させることを要求しない。これらの値、計測、遮断操作、料金は
OfferingまたはHostの運用interfaceが持ち、specやBindingのfieldではない。policyによる拒否や
遮断はapplicationへ通常のnetwork errorとして見え、Resourceのdesired stateを変えない。

`true`をこの強制境界つきで実現できないHostは、このURLのsupport応答で`supported:true`を
返さない。`outboundInternet`の値ごとのsupportやcapabilityはなく、`false`だけを満たす実装は
supportではない。

## backend対応と適合 {#conformance}

hostedはCloudflare ContainersをHost所有DO executor/broker経由で実行する。self-host Dockerと
Linux/systemd上のDocker Engineも、同じlinux/amd64 image、spec、UID、Bindingを使う。
裸のhost processをOCI imageとして扱わない。daemon-free OCI runtimeは別backend実装。
どのbackendもtenant隔離、owned network/process、[外向き通信](#egress)の両値、同じ
health/cutover/stream/cancel/authority semanticsを満たす。

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
stream/backpressure/cancel、response完了時のupload取消、head前後error、framing、無retry、
秘密の配送・置換・保持喪失・非公開、index pinの選択と保持、選択image bytes上限、全protocolの
false/true egressを実backendで測る。ローカルfixtureやsource公開だけをCloudflare/Docker/Linuxの
support証拠にしない。このURLのHost supportは、この本文の作成や公開から宣言しない。
