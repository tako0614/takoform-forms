# Takoform Form／Binding 1.0 標準化変更案

作成日: 2026-10-14 UTC。レビュー反映: 2026-10-15 JSTのTakoserverレビューと、その後の決定。状態: **最新のShouta方針を最優先で反映。ローンチ必須はTier 1に加えContainerとActor（WebSocket upgradeを含む）。各Formは完成まで0.xで反復し、仕様が妥当と判断できたものをCTO確認後に1.0へ昇格する。takoserverは1.0のみサポートする。** 文書は変更案であり、実装完了・昇格承認・公開・Host対応の証拠ではない。Web標準との衝突・独自拡張は§9.3のCTO報告用一覧に分離する。

## 1. 結論と今回の境界

既存Edgeの17 Formと復活するContainerを安定版へ揃えるShouta方針を設計目標とし、既存7 Bindingの内容も整える。ContainerをWorkerから利用するBindingを加えると、設計対象は **18 Form／8 Binding** になる。**ローンチは安定版で行い、Tier 1の12 Form／4 Bindingに、Container・ActorNamespaceとcontainer／actor Bindingを加えた14 Form／6 Binding、およびActorのWebSocket upgradeを必須とする。** hosted CustomDomainは従来どおり必須外。Queue／Workflow等の全対象完成を追加の条件にはしない。集合を束ねる新しいHost API版やpackageの版は作らない。

**今回通知されたShouta方針が、直前のCTO決定の該当部分とキュー済みの指示に優先する。** 現在有効な方針を以下へ統合し、置換した旧決定を§9.2で明記する。Web標準との衝突候補は§9.3に分ける。

1. **Host APIは `forms.takoform.com/v2` のまま。** HTTP経路、Resource／Operation envelope、UID、generation、再送、privateInputs、Support／Offeringの意味を変えない。
2. 実行時はWeb標準の型を使う。公開する名前・設定にCloudflare専用の語彙や型を要求せず、Cloudflare、Docker、Linuxの利用者から同じ入力・出力・失敗・永続化の意味が見えるようにする。
3. **ContainerとActor（WebSocket upgradeを含む）をローンチ必須へ戻す。** 第1段階のWorker系、SQLite系、ObjectBucket、EdgeKV、StaticAssetBundle、Cron、およびsqlite／edge-kv／object-bucket／service Bindingは引き続き先行し、10/21を設計レビューの目標とする。日付だけを理由に未完成のFormを1.0へ昇格しない。
4. 11/04は第2段階の中間判断日とする。**Container・Actor／upgradeが未完ならローンチの未充足条件として扱い、未対応のまま必須を満たしたことにしない。** hosted CustomDomainは必須外のまま、未完なら未対応として公開する。Queue／Workflowは後続設計を続ける。
5. D04の型付き結果、D05のnon-final part等長・streamのContent-Length必須、D07のasset Range任意とfallbackのnull／index.html限定は承認済み。ただしWeb標準型の意味と衝突する実装・API拡張まで一括承認されたとは扱わず、§9.3で個別に報告する。
6. **1.0は完成した0.xからの昇格とする。各Formは完成まで0.xで反復し、0.xの追加公開を可能にする。1.0昇格前にCTOへ確認する。** 公開済み0.xのbytesを上書きせず、修正は新しい0.x版へ進める。Container Draft PR takoform-forms#17はそのまま進めてよい。WorkerVersionの初回安定版へ必要なBindingを含める構成は§7.2の提案として詰め、旧1.1.0送りを完成条件より優先しない。
7. **takoserverは1.0のみサポートする。** 0.xを読取・復旧・削除用として新takoserverへ残す旧案も置き換える。既存hosted 0.xがあれば、旧環境での効果確定・移送・退役方法を先に決め、新takoserverへ未処理0.xを持ち込まない。0.x／1.0の90日並行サポートは設けない。
8. Formごとの複数版登録機構とexact URLの参照検査は維持し、**登録機構の能力とtakoserverが対応する版の許可方針を分ける。** 新takoserverの提供対象はCTO確認を経た1.0のみで、0.xをcomplete／retainedのどちらにもサポート登録しない。
9. **Provider 5.0.0の公開前にもCTOへ確認する。** Formの1.0昇格確認とProvider公開確認は別々に記録する。今回の文書commit・pushやDraft PR継続の許可を公開承認とみなさない。本番deploy・公開releaseは今回行わない。

本案の推奨は、既に中立なForm kind名と管理モデルを極力保ち、**実行値の型、asset設定、ストレージの結果形、HTTPの扱い、Actorのupgrade契約**を直すこと。`Worker`、`Binding`、`Namespace`、`cron`、`queue`、`bucket`自体はCloudflare専用語ではない。ただし現在のmodule default exportやscheduled／queue handlerの呼出し規約までWHATWGの標準であるとは説明しない。

初版のwriterはこの `PROPOSAL.md` のみ。今回のレビュー反映では本文とGit共有用の提案文書をcommit・pushするが、productのコード、規範仕様、task ledger、freeze、package、state、設定は変更しない。本番deploy・公開releaseは行わない。以下の実装順・移行・検証は将来の作業案であり、今回実行した作業ではない。

## 2. 調査基準と現状の読み分け

### 2.1 参照点

checkoutが古い、またはdirtyなrepoがあるため、現在の作業ディレクトリの表示と採用するv2仕様を混同しない。主にローカルGit objectを `git show` で読んだ。以下のSHAはこの調査で確認した参照点で、ネットワーク上の最新mainや公開状態を再確認したという意味ではない。

| 記号 | 読んだ正本・参照点 | 使用目的 |
| --- | --- | --- |
| T | `takoform` checkout HEAD `7b2343fc8e349e428179407fef064de5b7bf44f7`、v2はlocal origin/main `d9daf8e7ed22bd335484d7553de9abbb802df790` の `spec/host-api/v2/{README,http,forms,migration}.md` | 共通API、exact Form URL、不変UID、Support、秘密、互換性境界。 |
| F | `takoform-forms` checkout HEAD `c8800721866046e86492edd4012380e8e7caabca` の17 Form JSON／7 Binding JSON／関連Interface、v2は `8b976cb617f84698bc3c79cbcf705c1a330a2316` の `spec/forms/` 全17本文とfreeze／guides | 旧packageと現行v2本文を別々に比較。以下の「現行」は原則v2本文。 |
| FC | Container案に対応するForms commit `55811b390dbae8bd0c67a65e471d68d3ffb72006` の `Container/0.1.0`、`WorkerVersion/0.6.0`、`WorkerDeployment/0.5.0` | 既存17 Formとは別の未公開案。最新方針では0.xで反復し、完成・CTO確認後に安定版へ昇格する材料。 |
| P | `terraform-provider-takoform` checkout HEAD `81ef75cfc50e0ab9b598f536507d29e596f6603c`、実装比較はTASK-0098の統合commit `97ba18102b550d48319a71575568e36ebeeca7bb` | `internal/edgev2`、`internal/provider/v2_*`、intent journal／private input文書、保存された旧spec・Form custody。 |
| S | `takoserver` checkout HEAD `6b261a9edbe6e2cfd98d5e22e82c6e55c655ef7a`、v2実装比較はlocal origin/main `82917e2058c5eabe1cde94ee170875d3eb9213e5` | `src/takoform-v2/`、self-host投影、runtime／data service、Container runtime、関連docs。 |
| Q | `takoserver-private` local origin/main `7e03ae5e50711593f7883c09a23b089b38010cd5` の関連sourceのみ | REPORTが指すWfPのnormal map、KV key codec、ObjectBucket namespace、Worker wrapper。credentialやproduction stateは読まない。 |
| C | `takos-control` checkoutの TASK-0090 (`takos-control:tasks/TASK-0090-takoform-v2-specification.md`)、TASK-0098 (`takos-control:tasks/TASK-0098-provider-host-v2-client.md`)。checkout HEAD `366cb107fba8bf37ad7b9678cc2db40968a654fb` | v2採用とForm公開の境界、Providerのexact URL／state／intentの判断。task本文にはHEAD後の作業記録も含まれる。 |
| R | Edge coverage REPORT (`operator-runs:takoform-edge-coverage-20261014/REPORT.md`) | 17＋7の実装・試験・構成の不足。報告のS相当基準は `a4273e63` で、本案のSより古い。 |
| K | Container SPEC (`operator-runs:takoform-container-20261014/SPEC.md`) | OCI、private HTTP、health、切替、stream、秘密未実装、egress、Linuxの範囲。 |
| V | [Takoserverレビュー](REVIEW-takoserver.md)、作成2026-10-15 JST。S側基準 `376afb5c`、Qは同じ `7e03ae5e` | レビューは実測・CTO決定ではなく、工数とnative制約の仮説を含む。レビュー原文は保持し、後続のCTO決定・反映点・不同意点を§9.2、標準差異を§9.3へ記録する。 |

固定sourceを辿る入口: [T v2](https://github.com/tako0614/takoform/tree/d9daf8e7ed22bd335484d7553de9abbb802df790/spec/host-api/v2)、[F Forms](https://github.com/tako0614/takoform-forms/tree/8b976cb617f84698bc3c79cbcf705c1a330a2316/spec/forms)、P codecs (`terraform-provider-takoform@97ba18102b550d48319a71575568e36ebeeca7bb:internal/edgev2/spec.go`)、[S v2 implementation](https://github.com/tako0614/takoserver/tree/82917e2058c5eabe1cde94ee170875d3eb9213e5/src/takoform-v2)。付録のfile:lineもこの参照点に対する位置である。

### 2.2 既存の三つの契約を区別する

| 集合 | 実体 | 1.0作業での扱い |
| --- | --- | --- |
| 旧v1 package | R／c880 checkoutで調べた旧集合は17 Form JSONが概ね0.1.0、WorkerVersion 0.3.0、Deployment 0.2.0。Bindingには既に1.0.0、object-bucketには1.1.0がある。 | 既存identity、bytes、tag、署名、参照を保持。新たな意味で再採番・上書きしない。 |
| 現行v2 Form | URLごとの人間向け規範。§4／§5の現行欄に個別の0.x版を示す。BindingはForm本文内の契約。 | 新しい0.x版で仕様を反復し、問題ないと判断できた版をCTO確認後に `/forms/<Kind>/1.0.0/` へ昇格。既存0.xのURLや本文は保持。 |
| 未公開Container案 | Container 0.1.0とWorkerVersion 0.6.0／Deployment 0.5.0の候補。 | Container Draft PR #17を継続し、0.xで完成まで反復する。既存候補のmerge・公開・1.0昇格は別の行為。 |

v2のBindingを「全て1.0」にするとは、**1.0 Formの版固定Binding章を完成する**という意味にする。旧 `bindings.takoform.com/v1alpha2` の `module-worker.*@1.0.0` を書き換える意味にはしない。Binding用CRUD、v2 package、共通Binding版系列は追加しない。識別は対象Formのexact URL＋その本文のBinding章と、消費側WorkerVersionのexact URLを組にする。

さらにFの8b976cbでは、selected v1 sourceに9件のForm後継、Actor Binding2.0.0、Workflow Binding3.0.0等の未公表後継がある。これらも旧公表集合やv2の版番号とは別である。以下のBinding表で「旧1.0.0」と書く箇所はR／旧集合の比較基準を指し、この後継sourceを削除・巻き戻す提案ではない。

TASK-0090のv2公開・固定の記録があり、Rが列挙する旧v1定義だけを「現行」として1.0化すると比較対象を取り違える。Pの保存specにも旧文書版やv1の記述が残る。共通APIの正本はTのv2とし、旧文書は移行・履歴の証拠として扱う。

Rは全17／7の実環境適合を証明していない。Qの通常mapではModuleWorker／WorkerVersionがretained側に置かれるsourceも確認した。Sの新しいsourceが存在することから、Rで未確認だったlive対応を完了扱いにはしない。

## 3. 全Formに適用する1.0の設計規則

### 3.1 標準を採用する範囲

| 参照 | 採用案 | 標準に含まれない部分 |
| --- | --- | --- |
| [WHATWG Fetch](https://fetch.spec.whatwg.org/#fetch-api)／[URL](https://url.spec.whatwg.org/) | 実物のRequest、Response、Headers、URL、RequestInit、BodyInitを使用。本文消費・clone・header正規化は標準に従う。 | Resource UIDへのBinding配送、認可、weighted選択、handler ABIはTakoformの定義。Form URLの完全一致規則はHost API v2のまま。 |
| [Web Streams](https://streams.spec.whatwg.org/)／[DOM AbortSignal](https://dom.spec.whatwg.org/#interface-abortsignal) | ReadableStream／WritableStream／TransformStream、backpressure、cancel、abortを入出力へ接続する。 | 非冪等処理のabortがrollbackを保証するかは各操作で定義する。通信取消だけで効果なしとは扱わない。 |
| [W3C File API](https://www.w3.org/TR/FileAPI/)／[Encoding](https://encoding.spec.whatwg.org/) | 値のsnapshotはBlob、SQL BLOBはUint8Array、文字列との相互変換は標準APIで明示する。 | Blobの保管先、quota、複製方式はHost実装。 |
| [Web Crypto](https://www.w3.org/TR/WebCryptoAPI/) | crypto.getRandomValues、randomUUID、subtle、CryptoKeyをruntimeの共通面にする。artifactのSHA-256照合を保つ。 | CryptoKeyをspec／state／JSONへ入れない。鍵の配送・秘密保管・Host認可は別契約。標準APIの存在だけで全アルゴリズム対応は主張しない。 |
| [WinterTC](https://wintertc.org/)／[Minimum Common Web Platform API](https://min-common-api.proposal.wintertc.org/) | サーバーruntimeでのWeb API共通面を参照し、採用時点の版・必要項目・差異をForm仕様に固定する。 | Web標準の値を使っただけでWinterTC全適合とは表示しない。module handler、SQL、KV、Queue、Actor、Workflowはこの標準が定めるAPIではない。 |
| [Amazon S3 API](https://docs.aws.amazon.com/AmazonS3/latest/API/Welcome.html) | object、key、ETag、条件・range、multipartの語彙と意味を参照する。 | S3はWHATWG標準ではない。Bindingを実装するだけでS3全APIや公開S3 endpointの提供を約束しない。 |
| [SQLiteの型](https://www.sqlite.org/datatype3.html)／[transaction](https://www.sqlite.org/lang_transaction.html) | SQLite SQL、placeholder、原子的commit／rollbackを明記し、runtime値と内部JSON transportを分ける。 | Web標準にSQL Bindingはない。SQLite dialectとTakoformの制約を「標準SQL全対応」と呼ばない。 |
| [POSIX crontab](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/crontab.html) | 5欄と日／曜日の一致を参照し、UTCと採用文法を明記する。 | `/step`などを含む本契約の全体をPOSIXそのものとは呼ばない。shell command、環境依存timezone、Cloudflare拡張は取り込まない。 |
| [OCI image manifest](https://github.com/opencontainers/image-spec/blob/v1.1.1/manifest.md)／[descriptor](https://github.com/opencontainers/image-spec/blob/v1.1.1/descriptor.md) | registry/repository@sha256のdigest固定imageと、manifest／layerの検証を採用する。 | imageのdigestはregistryへの取得権限・署名・安全性の証明ではない。実行・隔離・networkはHost所有。 |

上表の「採用案」は本提案の設計判断であり、外部標準がTakoformにこの形を要求するという主張ではない。Living Standard等は1.0確定時に参照版を記録する。

### 3.2 共通の値・失敗・上限

- runtimeと管理wireを分ける。spec／observed／outputは引き続きJSON。Request、Blob、CryptoKey、Stream、AbortSignalを管理JSONの疑似objectとして保存しない。内部brokerがBase64等を使うことは許すが、その表現をアプリへ漏らさない。
- 最低globalへBlob、File、FormData、WritableStream、TransformStream、DOMException等の採用項目を明記し、既存のfetch／Request／Response／Headers／URL／URLSearchParams／TextEncoder／TextDecoder／AbortController／crypto等を標準の挙動で揃える。native `request.cf`、`cf` fetch option、`Response.webSocket`、Buffer、Node stream、vendor storage classをportable型にしない。
- 型・closed objectの誤りはTypeError、標準APIが生成するDOMExceptionとabort reasonはその意味を維持する。独自の業務失敗は `name === "TakoformRuntimeError"` と固定 `code` で判定し、isolateを跨ぐ `instanceof` は保証しない。既存のsnake-case `Error.name` は旧ABIに残し、新ABIへ無言で混ぜない。get／head等のHTTP読取はResponse、ObjectBucketのput／delete／completeは§4.3の型付き結果・固定codeとする。配送不能等はPromise拒否、response head後の失敗はbody stream errorで伝える。管理Problem／Operation.errorは変えない。name／codeのcopyはHost由来のbrandや認可証明にはならない。
- `signal` の取消をHostが所有する配送・body・待機へ接続する。clientの切断検知とcalleeへのabort通知はbest-effortであることを明記し、呼出元の明示abort・Host deadline・退役時の自身の取消をbest-effortへ弱めない。既にbrokerへ渡ったSQLは、呼出元の結果待ちを取り消してもcommitされ得る。既commitのSQL、object、queue送信や外部HTTP効果は取消で巻き戻さず、不明な結果はunknownとして扱う。unknown後の自動mutation retryは行わない。
- UTF-8 byte数、Unicode scalar数、整数、秒、ミリ秒をそれぞれ明記する。TTL／retry設定は `...Seconds`、runtimeの時刻はDateまたは名前に単位を持つ値、管理観測時刻は既存RFC 3339とする。
- `supported:true`はexact Form全体の必須動作を満たす場合だけ。Offeringは容量・料金・配置を示せるが、未実装のABIや有効な設定値を実装済みに見せるための逃げ道にはしない。privateInputsだけは共通v2の明示された任意能力に従う。

## 4. 第1段階 — 10/21の先行レビューで固める差分

この節を第2段階より先にレビューし、0.xで仕様を固める。1.0昇格は完成判断とCTO確認後とする。kind名は既存17種を原則維持する。`ModuleWorker`は「TakoformのES module実行サービス」と定義し、ブラウザーのWHATWG `new Worker()`やCloudflare製品名と同一視しない。

### 4.1 Form別差分表（12種）

参照はFの `spec/forms/<Kind>/<現行版>/index.md`。表は完成時の `<Kind>/1.0.0/` の目標差分であり、まず0.xで反復する。各Formの仕様が問題ないと判断できた時点でCTOへ確認し、初めて昇格へ進む。表で維持する項目も新URLに明記する。

| Form／現行v2 | 現行 | 1.0案：語彙・型・APIの差分 | takoserverへの影響 | 移行時の注意 |
| --- | --- | --- | --- | --- |
| **ModuleWorker 0.3.0** | spec `{}`。default exportにfetch／scheduled／queue。`fetch(Request,env,ctx)`、ctx.waitUntil。最低globalは一部Web API。 | 空specと3 handler名を維持。`InvocationContext.signal:AbortSignal`を追加し、waitUntilはTakoform固有の寿命管理と明記。HTTPはRequest→Response、本文は標準stream。Blob等の共通globalと標準エラーを明文化。native module-worker ABIの直出しを止める。 | export検査、prelude、generated wrapper、全handlerへのcontext作成、退役・abort。module解決規則と循環ES moduleは現行を保つ。 | アプリがctxの追加propertyに依存せず動く場合は少修正。独自Error.nameのcatchは書き換え。Worker UIDは新規。 |
| **WorkerBundle 0.2.0** | 不変 `artifact:{url,sha256}`、manifestのentrypoint／files、媒体型・閉じたmodule graph。 | 入力とimmutable custodyを維持。通常JS moduleのmedia typeは `text/javascript` を採用する案。`application/javascript+module` は旧manifestの表現として残す。text／bytes／wasmはTakoformの明示的loader規則で、Web module全般の標準とは呼ばない。 | artifact validation、module media type map、inspector、runtime adapter。bytesのdigestは変換後ではなく取得原文で確認。新しいmanifestを生成・再hash。 | コードbytesが同じでもmedia typeを変えたmanifestは別digest。旧bundleを新URLで勝手に同一視しない。 |
| **WorkerVersion 0.5.0** | worker／bundle／handlers、vars、requiredSensitiveVars、7 Binding配列。assetsにrunWorkerFirst／notFoundHandling。 | `vars→values`、`requiredSensitiveVars→secretNames`、`kvBindings→edgeKvBindings`、`bucketBindings→objectBucketBindings`。**0.xでTier 1の4配列にactorBindings／containerBindingsを統合してから初回1.0.0へ昇格する案**を推奨し、昇格前にCTO確認。旧containerBindingsの1.1.0送りとの版構成は§7.2・§9.2で再整理する。assetsはorder／fallback、fallbackはnull／index.html。snapshot不変。 | exact URLごとのparser／正規化／ABI／reference policyと、1.0のみを許可する方針を分離。configured private inputs、実行投影、static-only判定、Provider codecを版ごとに選択。 | 正規化specだけでなく元request intentを保持。秘密値変更は新Version。未定義配列の予約・旧名aliasを作らない。昇格後の本文へfieldを後付けしない。 |
| **WorkerDeployment 0.4.0** | worker＋versions、1〜8版、weight合計10000、single-active、invocation固定、旧contextは15分で退役。 | 語彙とshapeを維持。全参照先を1.0へ固定。HTTP・scheduled・queue・Actor・Workflowのイベント選択を共通に説明し、AbortSignal／stream取消と物理退役の関係を明文化。 | 全required class／Bindingのgraph validation、旧新Form URLを跨ぐsingle-active、drain／owner fence。 | 旧WorkerのDeploymentへ1.0 Versionを混ぜない。旧新Worker間の入口切替が必要。weight変更はコード互換・データrollbackの保証ではない。 |
| **WorkerEndpoint 0.3.0** | worker参照だけ。受理時に固定hostname／url、TLSとactive routeを観測。 | shape／hostname安定性／TLSを維持し、Worker1.0へ固定。外部HTTPもservice Bindingも同じasset規則。vendor subdomain・zone・route IDは入力にしない。 | endpoint backend、frontend publication、TLS／route readbackと退役。 | 新UIDなら新割当URLになり得る。既存アドレスを移せるとは約束せず、アプリ設定・DNSを明示変更する。 |
| **SQLiteDatabase 0.2.0** | 空spec。execute／rollback-only query／transaction。BLOBがruntimeでも `{encoding:"base64",data}`。 | 空spec／SQLの3操作／rollback-only／migration分離を維持。型名を `SqlValue` にしBLOBをUint8Arrayへ。安全な有限number範囲は今回維持し、64-bit INTEGER全域を黙ってnumberへ丸めない。AbortSignalを各呼出しのoptionsで受ける。 | selfhost-v2-sqlite-worker-projection、data service codec、Node authorizer transport、結果byte budgetの検査。内部Base64をruntime境界で変換する。 | DB内容の変更とJS型の変更を分ける。Base64変換コードとcatchを修正。新DBへschema・data・migration履歴を検証付きで移す。 |
| **SQLiteMigrationSet 0.2.0** | 不変artifactと順序付きapplication/sql file。 | shape、SHA-256、UTF-8、ファイル順を維持。SQL dialectと禁止作用をSQLiteとして明記。D1 migration名・wrangler設定を要求しない。 | artifact parserは原則流用。exact Form URL追加と参照整合検査。 | 既適用SQLを改名／再hashしない。履歴の移行証明と新規初期化を区別する。 |
| **SQLiteMigrationApplication 0.2.0** | database／migrationSetは不変参照。既適用prefix一致、file＋履歴を原子的commit。DELETEはdownしない。 | shapeと永続履歴規則を維持。1.0 DB／Setだけを明示参照。runtimeのBLOB表現変更が履歴SQLの書換えを意味しないことを明記。 | migration authority／排他／ledger／native port。hostedではv2通常compositionへの接続が別途必要。 | 既存DBの履歴を同一性確認なしで「適用済み」と記入しない。Application削除や作り直しでdownしない。 |
| **ObjectBucket 0.2.0** | 空spec、独自metadata結果、5GiB／single put300MiB、streamはcontentLength必須、non-final partは等長。 | 空spec／strong read-after-writeを維持。get／headはResponse、put／completeは型付き結果、deleteはvoid。Blob／BufferSource／既知長streamに対応。**streamにはContent-Length必須、non-final partは等長**とするS3 subset。 | R2 facade、namespace、multipart owner、申告長と実byte長の検査、読取Response／書込型付き結果の変換。未知長staging・非等長part再構成は1.0の必須実装から除く。 | 読取はstatus／headersへ、put／completeの条件不成立は固定codeで判定。値本体は変換不要。未完multipartはcomplete／abortし、uploadIdを跨がせない。 |
| **EdgeKVNamespace 0.2.0** | 空spec、eventual、get／getWithMetadata、ArrayBuffer、expirationTtlSeconds、listComplete。世界各地への複製を説明。 | 空specを維持。getをBlob＋metadataのentry結果に統合、ttlSeconds、items／nextCursorへ。**TTLは60〜315360000秒、1.0のputは既知長のstring／Blob／BufferSourceだけ**。eventualは最低保証とし、強いreadを返す実装も許す。地理分散自体は非必須。 | KV facade、cursor、TTL、key codec、File-store／hostedの意味整合。native値をそのままenvへ出さない。 | getWithMetadataをgetへ変更し、value.arrayBuffer()等で読む。TTLの残りが60秒未満のdataを移す際は延命せず、失効待ちまたは移行toolで元の期限を守る。 |
| **StaticAssetBundle 0.2.0** | 不変artifact／files／mediaType。配信順はVersion所有。 | shapeとbytes custodyを維持。Blob／Responseへ載せるasset metadataとHTTP表現を明示。配信順・fallbackは§4.3の中立なVersion設定へ移す。新しいassets runtime Bindingをこの段階で増やさない。 | custodyと配信の試験を分離。Worker static/runtimeとWfP asset mountを同じ規則へ。 | 同一bytesを再利用してもResource認可は必要。旧bundleのURLを新FormのURLへ置換するだけの移行は不可。 |
| **WorkerCronTrigger 0.3.0** | worker／cron。UTC5欄、数値、range／step、日と曜日はOR。runtime scheduledTimeはepoch ms。 | specのworker／cronとUTC文法を維持。eventは `scheduledAt:Date`＋cron＋triggerUid＋安定occurrenceIdとし、CFのScheduledController形を出さない。signalはcontextに統一。未記録時刻のskip／記録済みのat-least-onceを維持。 | parser／match ledgerを活かし、event projectionとwrapperを変更。occurrenceIdは同一trigger・一致時刻で再起動後も同じ。自動clock接続を検証。 | scheduledTime→scheduledAt.getTime()。切替時に旧新cronを重複有効化しない。重複抑止はtrigger込みIDで行い、業務効果のexactly-onceを約束しない。 |

`text/javascript` は [HTMLのJavaScript MIME type取扱い](https://html.spec.whatwg.org/multipage/scripting.html#javascript-mime-type)を参照する提案で、現行loaderの独自媒体型をWeb標準だと再定義するものではない。変更後のaccepted media typeの閉じた集合は10/21の決定事項とする。

### 4.2 Binding別差分表（4種）

| Binding | 現行v2のsurface／旧Binding版 | 1.0案 | takoserverへの影響と移行 |
| --- | --- | --- | --- |
| **sqlite** | SQLiteDatabase0.2.0本文。旧module-worker.sqlite1.0.0。execute／query／transaction、Base64 BLOB、snake-case Error.name。 | 同じ3操作でUint8Array BLOB、`{signal}`、固定code。queryは引き続きrollback-onlyで、SELECT判定への置換はしない。 | SQL parser／authorizer・all-or-noneは維持し、facadeとbrokerの符号化層を変更。bool／bigint／Node Bufferを黙って変換しない。旧catchとBLOB入力を修正。 |
| **edge-kv** | EdgeKVNamespace0.2.0本文。旧module-worker.edge-kv1.0.0。getとgetWithMetadata、ArrayBuffer、expirationTtlSeconds、listComplete。 | get→KvEntryまたはnull、entry.valueはBlob。putはstring／Blob／BufferSource、**stream入力なし**。ttlSecondsは最小60、list→items／nextCursor、signal対応。 | 新ABIでnative cacheTtlを露出しない。public cursorをnamespace／principal／queryへ束縛。旧URLの仕様は履歴として保持し、新takoserverでは提供しない。 |
| **object-bucket** | ObjectBucket0.2.0本文。旧module-worker.object-bucket1.1.0。9操作、head/getは独自objectまたはnull。 | 9操作を維持。**head／getはResponse、put／completeはObjectWriteResult、deleteはvoid**。range／条件／metadataはHeaders。stream入力はContent-Length必須、non-final partは等長。 | 読取status、ETag quoting、range、書込precondition_failed、byte数とpart条件を検査。R2Object／httpMetadata／onlyIfは公開しない。 |
| **service** | ModuleWorker0.3.0本文。旧module-worker.service1.0.0。fetch(Request)／fetch(string,init)。 | `fetch(input:Request\|URL\|string,init?:RequestInit):Promise<Response>`。RequestInitで構築、signal・stream・headerを標準型で扱う。固定UID宛の配送・redirect非追従はglobal fetchとの仕様差として§9.3 WS-02へ分離し、RequestInit各項目の扱いを10/21までに決める。 | brokerのrequest取消、双方向backpressure、header重複、callee target更新、同一UID routingを検証。native Fetcher／RPCメソッド／request.cfを露出しない。標準型を使うことだけでglobal fetch完全互換とは主張しない。 |

### 4.3 第1段階で固める具体的API例

以下は差分をレビューするための署名案。最終仕様では引数ごとの必須・既定値・範囲・エラーを本文に展開する。標準型を使うことと、以下のstorage API自体がWeb標準であることを区別する。

```ts
interface InvocationContext {
  readonly signal: AbortSignal;
  waitUntil(task: PromiseLike<unknown>): void;
}

// fetch／scheduled／queueの寿命・取消はこのcontextへ揃える。
interface ServiceBinding {
  fetch(input: Request | URL | string, init?: RequestInit): Promise<Response>;
}

type SqlValue = null | number | string | Uint8Array;
type SqlRow = Record<string, SqlValue>;
type SqlResult = { rows: SqlRow[]; rowsWritten: number };
type SqlStatement = { sql: string; params?: readonly SqlValue[] };
interface SqliteBinding {
  execute(sql: string, params?: readonly SqlValue[],
          options?: { signal?: AbortSignal }): Promise<SqlResult>;
  query(sql: string, params?: readonly SqlValue[],
        options?: { signal?: AbortSignal }): Promise<SqlResult & { rowsWritten: 0 }>;
  transaction(statements: readonly SqlStatement[],
              options?: { signal?: AbortSignal }): Promise<{ results: SqlResult[] }>;
}

type BufferedBytes = string | Blob | ArrayBuffer | ArrayBufferView;
type KvMetadata = Readonly<Record<string, string>>;
type KvEntry = { value: Blob; metadata?: KvMetadata };
type Page<T> = { items: T[]; nextCursor?: string };
interface EdgeKvBinding {
  get(key: string, options?: { signal?: AbortSignal }): Promise<KvEntry | null>;
  put(key: string, value: BufferedBytes,
      options?: { metadata?: KvMetadata; ttlSeconds?: number;
                  signal?: AbortSignal }): Promise<void>;
  delete(key: string, options?: { signal?: AbortSignal }): Promise<void>;
  list(options?: { prefix?: string; limit?: number; cursor?: string;
                   signal?: AbortSignal }): Promise<Page<{ key: string }>>;
}
```

**SQLのfull 64-bit整数・bigintは1.0に入れない。** 値域は現行の有限binary64、絶対値 `Number.MAX_SAFE_INTEGER` 以下を保ち、範囲外は従来同様に失敗させて丸めない。BLOBだけを実行時Uint8Arrayに変え、内部wireのBase64は残せる。将来int64を足す場合も新しいexact Form URLで契約を定め、1.0のSqlValueを後から拡張しない。signalは送信前と結果待ちの取消を表し、broker受理後のSQLを必ず中断・rollbackできるとは約束しない。SQLはW3Cの型ではないため、Web風という理由だけでSQL文までfetchに置換しない。

**KVのttlSecondsは省略で期限なし、指定時は60〜315360000の整数秒**。putは値・metadata・TTLを置換する。1.0のKV putとQueue producerにReadableStreamは含めない。Blobは標準のstream()からアプリ内で読めるが、putに未知長streamを受ける保証とは別である。最大25MiBの値、metadata上限、UTF-8 key、eventualの最低保証は維持する。高頻度書込のrate／capacity失敗はHostが明示し、Cloudflare固有の毎秒値をWeb標準の保証として持ち込まない。

ObjectBucket案は次とする。

```ts
type ObjectOptions = { headers?: HeadersInit; signal?: AbortSignal };
type ObjectWriteResult = { etag: string };
type ObjectSummary = { key: string; size: number; etag: string;
                       lastModified?: Date };
type ObjectPage = { items: ObjectSummary[]; prefixes: string[];
                    nextCursor?: string };
type UploadedPart = { partNumber: number; etag: string };
interface ObjectBucketBinding {
  head(key: string, options?: ObjectOptions): Promise<Response>;
  get(key: string, options?: ObjectOptions): Promise<Response>;
  put(key: string, body: BufferedBytes,
      options?: ObjectOptions): Promise<ObjectWriteResult>;
  put(key: string, body: ReadableStream<Uint8Array>,
      options: ObjectOptions & { headers: HeadersInit }): Promise<ObjectWriteResult>;
  delete(key: string, options?: { signal?: AbortSignal }): Promise<void>;
  list(options?: { prefix?: string; delimiter?: string; limit?: number;
                   cursor?: string; signal?: AbortSignal }): Promise<ObjectPage>;
  createMultipartUpload(key: string, options?: ObjectOptions):
    Promise<{ uploadId: string }>;
  uploadPart(key: string, uploadId: string, partNumber: number,
             body: BufferedBytes, options?: ObjectOptions): Promise<UploadedPart>;
  uploadPart(key: string, uploadId: string, partNumber: number,
             body: ReadableStream<Uint8Array>,
             options: ObjectOptions & { headers: HeadersInit }): Promise<UploadedPart>;
  completeMultipartUpload(key: string, uploadId: string,
                          parts: readonly UploadedPart[],
                          options?: { signal?: AbortSignal }): Promise<ObjectWriteResult>;
  abortMultipartUpload(key: string, uploadId: string,
                       options?: { signal?: AbortSignal }): Promise<void>;
}
```

この案で必ず決める動作は以下。

- getは200／206、未存在404、条件不成立412、範囲不成立416。If-None-Match一致の読取は304。headも同じ確認を行うがbodyなし。アプリは読取Response.status／headersを見る。**put／completeの成功はObjectWriteResultのetag、deleteは不在を含めvoid**。putの条件不成立はnullと区別し、`name:"TakoformRuntimeError", code:"precondition_failed"` でrejectする。条件不成立でobjectを変更しない。completeの不正part／消失upload等も固定codeでrejectする。二種類の条件不成立表現を併設しない。
- `headers`は閉じた許可集合とする。get／headはRange、If-Match、If-None-Match、putはContent-Type、Cache-Control、Content-Disposition、Content-Encoding、Content-Language、Content-Length、If-Match／If-None-Match、利用者metadataは `x-amz-meta-*`。multipart開始はobject metadata、partはContent-Lengthだけ。metadataはheader名を小文字化し、各完全header名と値のUTF-8 byte長の和で合計2048 bytes以下とする案を採用する。秘密・認証・host選択headerは受け付けず、未知headerを黙って保管・転送しない。
- Range、ETag、Content-Range、Content-LengthはHTTPの構文で扱う。1.0初期範囲は単一byte range。ETagはquoteを含むHTTP validatorとして返し、digestやMD5と決めつけない。[S3 GetObject](https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html)、[S3 integrity](https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity-upload.html)を参照する。S3全実装を名乗らず、採用subsetを列挙する。
- Blob／buffer／stringは既知長。**put／uploadPartでReadableStreamを渡すときはContent-Length headerを必須**とし、安全な非負整数byte数を操作上限内で申告する。未知長streamは1.0で拒否する。既知長の値でContent-Lengthを明示する場合も実byte数と一致させる。不一致・超過を成功にせず、put途中bytesを既存keyへ公開せず旧objectを保つ。runtime bodyの標準stream、backpressure、途中取消は維持する。未知長の永続stagingは1.0では要求しない。
- multipartはpart番号とETag、5MiB以上のnon-final part、昇順・重複なし・最大10000 partに加え、**non-final partのbyte長を全て同一にする**。最後のpartだけ短くてよい。この制約をCloudflare、Docker、Linuxの全経路で同じS3 subsetとして適用する。不等長の再構成を1.0で要求しない。[R2の制約](https://developers.cloudflare.com/r2/objects/upload-objects/)、[S3 UploadPart](https://docs.aws.amazon.com/AmazonS3/latest/API/API_UploadPart.html)を参照する。未知長対応や非等長part対応を将来足す場合は新しいexact Form URLで行い、旧1.0の制約を上書きしない。
- cursorは不透明でquery・owner・bucketに束縛する。native cursor、physical prefix、credentialはpublicに出さない。ページはsnapshotを保証しない。strong read-after-writeと、複数pageに跨るsnapshot保証は別である。

**D05のエラー仕様（Vの2026-10-15 01:2x JST追記を反映）**:

| 条件 | runtimeの例外 | 副作用と検出時点 |
| --- | --- | --- |
| put／uploadPartにReadableStreamを渡しContent-Lengthが無い | **TypeError**、name:TypeError、code:TAKOFORM_LENGTH_REQUIRED、status:411 | streamをlockせず、1 byteも読まずにreject。object／partを作らない。callerは長さを明示して呼び直せるが、Hostは勝手に再送しない |
| 宣言Content-Lengthと実byte数が違う | **RangeError**、name:RangeError、code:TAKOFORM_CONTENT_LENGTH_MISMATCH、status:400 | object／partをcommitせず、中途dataを破棄。既存の確定object／partを成功扱いで置き換えない |
| complete対象でnon-final partが等長でない | **RangeError**、name:RangeError、code:TAKOFORM_PART_SIZE_MISMATCH、status:400 | 遅くともcompleteで組立前に拒否。uploadPart時の早期検出は非最終partだと確定できる場合だけとし、有効な短いfinal partを誤拒否しない |

3経路で例外のclass／name／code／statusを同じにする。isolateを跨ぐinstanceofは引き続き非保証。Blob／BufferSource／stringは長さ既知なので「長さ未指定」の対象外だが、明示headerとの不一致検査は必要である。上記codeはこの意味に固定し、将来の別exact URLで未知長入力を追加しても別の意味に再利用しない。

ここでのstatusは**runtime例外のpropertyで、Host API v2のHTTP応答の追加ではない**。独立したHost固有data-plane HTTPを設ける場合に411相当を使うことと、共通管理APIのstatusを変更することを区別する。本案は新たなcontrol／upload経路を追加せず、管理要求の失敗は既存v2の規則に従う。

**Asset設定の具体形**:

```json
{
  "assets": {
    "bundle": { "resourceUid": "asset-bundle-uid" },
    "order": "assets-first",
    "fallback": "index.html"
  }
}
```

orderとfallbackはassets指定時に必須とする。**fallbackはnullまたは文字列index.htmlの2値だけ**で、任意pathは受け付けない。index.html指定時はmanifest内のそのroot fileを検証する。assets-firstはGET／HEADでasset→fallback→handler、handler-firstはhandlerが404のときasset→fallback、両方missなら元の404を保つ。静的専用Versionはassets-firstのみ。非GET／HEADはhandler、なければ404。危険・不正なpathではfallbackしない。index.htmlは従来のSPA動作と対応するが、HTML navigationだけに変える挙動は今回足さない。URL pathnameの一度decode、dot／slash／control拒否と大小区別を維持する。

この改名は、[Cloudflareのrun_worker_first／not_found_handling](https://developers.cloudflare.com/workers/static-assets/binding/)から設定名とenumを切り離すためである。assetのContent-Type、Content-Length、ETag／If-None-Match（304）、HEADは必須、**asset Rangeは任意**とする。未対応ならRangeを無視して通常の200／全bodyを返せる。対応する実装は単一rangeの206／416等を正しく扱い、Accept-Ranges:bytesを偽って広告しない。ObjectBucketのRange必須性は変えない。asset originの取得制約・digest・保管は緩めない。

### 4.4 上限の扱い

467-byte KV key、979-byte object key、25MiB KV value、300MiB single put、5GiB object、1MiB manifest／16MiB file／128MiB bundle、SQL／queueの制限などはWeb標準が定めた数字ではない。現行値にはbackend設計由来のものが含まれる。Qの `v2-object-bucket-namespace.ts` では979-byte論理keyとphysical prefixの対応を実際に確認できる。

**推奨は1.0で既存の有効入力を狭めず、上記の値をTakoformの共通profile上限として当面維持すること。** 名前を変えるために数値まで一斉に増減しない。より大きい値を1.0必須にするなら、native prefix・staging・保持・全backend適合を先に設計する。Offeringに異なる上限を出す場合も、Formが保証する入力範囲を実装できないことを単なるcapacity不足と取り違えない。

第1段階で据え置く上限、KV TTL最小60秒、object metadata2048 bytes、streamのContent-Length必須、non-final part等長を一覧にして10/21に固定する。未知長staging・非等長再構成のquota／retentionは1.0の作業に含めない。SQL結果はUint8Array化によりcanonical JSON sizeをそのまま測れなくなるため、**内部canonical wire（BLOBを既存Base64へ符号化）のbyte budgetを引き続き判定する**。runtime値をBase64へ戻す必要はない。

## 5. 第2段階 — 11/04の中間判断へ向けた差分

第1段階は10/21の設計レビューを先行する。各Formは完成まで0.xで反復し、WorkerVersionもローンチに必要なactor／container Bindingを含む構成を昇格前に詰める。ModuleWorkerの基礎handler ABIに加え、Actor class／upgradeの接続をローンチまでに完成させる。Workflow固有class ABIは後続の設計とし、確定済みの1.0 URLを後から変更しない。複数版の登録能力と、takoserverの1.0のみ対応は§7.2で分ける。

**最新のShouta方針によりContainerとActor（WebSocket upgradeを含む）はローンチ必須へ戻す。** 11/04の中間判断で未完なら、完成・検証・1.0昇格の残作業をローンチ条件として追う。ActorのHTTPだけで完了とせず、upgradeまで受入証拠を揃える。hosted CustomDomainは必須外のまま、未完なら未対応として公開する。Queue／Workflow等は後続対象であり、全18 Form／8 Bindingの完成を追加条件にはしない。以下は完成時の目標差分で、0.xでの反復・CTO確認を経て昇格する。

### 5.1 Form別差分表（既存5種＋Container）

| Form／現行 | 現行 | 1.0案：語彙・型・APIの差分 | takoserverへの影響 | 移行時の注意 |
| --- | --- | --- | --- | --- |
| **AtLeastOnceQueue 0.2.0** | deliveryDelaySeconds／messageRetentionSeconds、send／sendBatch、opaque bytes最大127000、unordered／at-least-once。 | Form名と設定を維持。本文入力はBufferedBytes、受信はBlob。**1.0ではstream入力なし**。messageのid／attemptsを保持し、timestampMillis→enqueuedAt:Date。 | producer facade、永続受理、wrapper、byte長検査、Host receipt。最大127000 bytesとsendBatch全件原子性を守る。producerの消費側版は後続の0.x設計で決め、昇格前にCTOへ確認する。 | 既存の未消費messageとinflightをdrainするか、ID・時刻・残存TTLを含む移送を別に計画。native queueの直接copyを一般手順にしない。 |
| **QueueConsumer 0.3.0** | queue／worker／maxBatchSize／maxBatchTimeoutSeconds／maxConcurrency／maxRetries／retryDelaySeconds／deadLetterQueue。batch methodsでack／retry。 | 同じ設定とbatch settlementを維持。Cloudflare MessageBatchではなくTakoformのQueueDeliveryと説明。messageはBlob／Date。context.signalを追加。通常returnで残りack、throwで残りretry、DLQ循環拒否を保つ。 | 配送lease／settlement／worker graph／停止境界。queueのsendBatch原子性とDLQ効果のunknownを全backendで確認。 | Consumer二重接続不可。旧Consumer停止・未settled再配信可能性の確認後に新側へ切替。新旧IDの対応が必要なら明示移送toolの責任。 |
| **WorkerCustomDomain 0.3.0** | hostname／worker。所有確認後にTLS／route。正規化名を占有。 | shapeを維持。1.0 Worker宛。hostnameはURLのhostname型に沿う文字列表現・IDNA／DNSの制約を明記。zone ID／account ID／SaaS custom-hostname ID／workers.devを入力にしない。 | 新v2backendの通常mount、ドメインcontrol proof、Host全体の外部hostname競合、TLS、route退役。Qのgateway aliasと実custom domainを混同しない。 | 同じhostnameを旧新Attachmentへ同時に占有させない。所有確認とroute切替をHostが明示できなければ停止時間ありのdetach→attach。DNS zone全体を削除しない。 |
| **Container（未公開0.1.0案）** | image／httpPort／healthPath／workloadRevision、environment、requiredSensitiveVars空のみ、outboundInternet。private fetch、OCI linux/amd64、健康確認→切替→30秒drain。 | kindはContainer、Bindingはcontainerを推奨。digest固定OCI、port／health／revision／非秘密environmentとprivate HTTPを維持。`CLOUDFLARE_`を公開予約名から除き、共通 `TAKOFORM_` のみ予約。初期版が秘密非対応なら常に空しか取れないrequiredSensitiveVars欄を削除し、private input名なしと明記する。outboundInternetのfalse／true両方を意味どおり実装する。 | **ローンチ必須**。v2 lifecycle adapter、Host-owned image取得／保持、WfP broker、Docker／Linux isolation、egress enforcement、readiness、cutover／drain／unknown回復。既存legacy runtimeの接続だけで完了しない。 | Draft PR #17を継続し、0.xで完成まで反復・追加公開できる。1.0昇格前にCTO確認。既存候補のUIDを1.0へ再解釈せず、mutable tag解決・native process APIも持ち込まない。 |
| **ActorNamespace 0.3.0** | worker／className、idFromName／newUniqueId／get／stub.fetch、private SQL／alarm／socket。Host製Response.status101と特殊constructor alias。 | shapeとsingle-writerを維持。callerは`getByName(name)`／`get(id)`／`createId()`へ整え、実体化は初回配送。fetchは標準型、storage BLOBはSqlValueへ。alarmはset(Date)／get():Dateまたはnull。**101を標準Responseに見せる拡張はやめる**。upgrade結果を明示的なTakoform型として別に定義する案（§5.3）。 | deterministic ID互換、Actor SQL codec、alarm projection、全class検査、ActorResponse／upgrade handoff／socket broker／native wrapperの変更。最も大きいruntime変更。 | namespace／actor IDはdata identity。新UIDへの移行でIDだけcopyして旧dataへ届くと仮定しない。store／alarm／socket attachmentを計画し、socketは再接続。 |
| **DurableWorkflow 0.3.0** | worker／className、create／get／status／sendEvent／terminate、step.do／sleep／waitForEvent、memoized replay。 | Form名・shape・主要methodを維持。Cloudflare WorkflowEntrypoint継承不要を明示。runtime payloadはJSON documentのまま、保存する値へStream／Request／Blob／CryptoKeyを許可しない。context.signalと標準errorの境界を整理し、期限・retryは明示単位のまま。 | class inspection、workflow data encoder、Host error brand、durable history／replay、sleep／wake、physical terminate。JSON限定は保持し、structuredCloneが使えるという理由で永続型を広げない。 | 実行中instanceは旧コード・旧ABIで完了させるのが既定。step name／output形の互換なしに新runtimeでreplayしない。新規instanceを新Workflowへ向ける。 |

### 5.2 Binding別差分表（既存3種＋Container）

| Binding | 現行 | 1.0案 | takoserverへの影響と移行 |
| --- | --- | --- | --- |
| **queue-producer** | 旧1.0.0。send／sendBatch、string／ArrayBuffer／view、結果はmessage ID。 | method／結果を維持。Blobとsignalを追加し、stream入力は含めない。queueProducerBindingsの消費側版は後続設計とし、完成前に番号だけを固定しない。 | unknown sendを新しい送信として自動再実行しない。sendBatch全件原子性を保持。文字列→UTF-8、byte view範囲のcopyを検査。 |
| **actor** | 旧1.0.0。idFromName／newUniqueId／get、stub.fetch。 | `getByName(name)`→stub、`createId()`→opaque ID、`get(id)`→stub。stubはreadonly idと標準型fetch。idはparseしない。 | 名→IDの安定変換をnamespace内で固定。renameだけでもpublic ABI変更なので新URLに限定。callerへstorageやalarm権限を渡さない。 |
| **workflow** | 旧1.0.0。create／getとinstance.status／sendEvent／terminate。 | method名・7 statusを維持し、操作optionsのsignalと固定codeを揃える。取消した待機とWorkflow instanceのterminateを分ける。 | 中断したcallerだけを理由にdurable instanceを停止しない。旧historyは旧ownerに残し、new instanceのtargetだけを切替。 |
| **container（追加）** | K／FCの案はenv.APP.fetch(Request)のみ。container-httpという別名案あり。 | module-worker.containerを作業名とし、Container1.0本文のworker-binding章で定義。**WorkerVersionの0.x候補へcontainerBindingsを統合し、初回1.0.0へ昇格する構成案**をCTO確認へ出す。serviceと同じ標準型で、process管理APIを出さない。 | targetUID／callerVersionへのscope、health済みgeneration、header／stream／取消、権限取消、30秒drain。ローンチ時に実利用できる消費側まで必須。未定義fieldの予約や公開済み版への後付けはしない。 |

### 5.3 ActorのWebSocketは独立して決める

現行FのActorは `status:101` のbrand付きResponseと `new Response(upgrade.body, upgrade)` の特例を要求する。Sの `src/actor-upgrade-handoff.ts` は200のnative backingを持つActorResponse subclassでlogical101を表す。この実装を標準Responseの意味に混ぜたまま1.0にしない。

[FetchのResponse constructor](https://fetch.spec.whatwg.org/#dom-response)と[WebSockets API](https://websockets.spec.whatwg.org/)は、このサーバー側upgradeを表す共通APIを与えていない。したがって「Web標準のWebSocket型なら全部解決する」とはしない。

推奨案は、通常のfetchは標準Response、upgradeを返す場合だけ `ActorUpgrade` というTakoformの明示型を使い、handlerの戻り値を `Response | ActorUpgrade` と書くこと。`ActorUpgrade` はHost発行の一回限り能力で、別invocationへの転送・直列化・service Binding経由の生成を認めない。socketのmessage／close eventの値はWebSocket語彙に揃え、永続attachment／alarm／Host所有socketの操作は独自APIと明記する。Response subclassや `Response.webSocket` をportable globalへ足さない。

**Actor WebSocket upgradeはローンチ必須。** `ActorUpgrade`の型・能力・転送制限・socket寿命・再接続を0.xで詰め、Web標準Responseと両立する完成契約を1.0昇格前にCTOへ確認する（§9.3 WS-01）。11/04は進捗の中間判断であり、未完のupgradeを後回しにしてローンチ条件を満たしたことにはしない。通常のHTTP／streamは標準Responseのまま設計を先行する。

### 5.4 Container案から引き継ぐもの・修正するもの

- OCI imageはregistry/repositoryとlowercase sha256 digestで固定し、tagだけ、latest解決、image URLを入力にしない。初期portable workloadはlinux/amd64、単一manifestとする。multi-platform indexやARMを1.0で受けるなら、全backendの選択・digest検証を先に閉じる。
- imageのENTRYPOINT／CMD、port、health path、author-declared workloadRevision、非秘密environmentを維持する。runtime filesystem／memory／instance IDはephemeral。volume、Docker socket、native IP、DO class／namespace、instance sizeはportable設定にしない。
- Container初期版をsecret-freeにするか、秘密注入を含めるかは0.xで詰め、昇格前にCTOへ確認する。推奨する最小案は前者で、秘密名の空欄を機能のように残さない。後者ならWorker同様のgeneration-bound custody、補給、lost ACK、rotation、新規実行への適用を完成させる。Container自体はローンチ必須であり、11/04で未完でも必須から外さない。
- outboundInternet:falseは実際にdeny、trueは実際にallowの権限を与える。同意なくtrueへ変えない。Sの現legacy adapterはtrueも拒否するため、現在の実装だけではこの入力全体に適合したとは言えない。
- healthの2xx確認前に新generationへ切り替えず、不健康更新は旧servingを保つ。callごとのgeneration固定、30秒drain、取消と物理退役、unknown時の同UID回復を維持する。
- Kのbody100MiB、header64件／16KiB、path＋query8192文字、HTTP method集合／upgradeなしを共通profileとして維持する。Content-Length不一致や途中stream errorをresponse head後の別statusへ差し替えない。複数Set-Cookie、hop-by-hop header、Authorizationのapplication上の扱いをservice Bindingと整合させる。
- CloudflareではHost所有executor／broker、DockerではHost所有daemon／network／revisionを使う。KのLinuxはsystemd／Bun＋Docker Engineであり、DockerなしのOCI実装を完了した意味ではない。DockerとLinuxは独立したprovider分類ではないが、利用者が起動する各経路で同じ受入試験を行う。
- native image catalog、staging、retention、capacityはHost内部／Offeringの責任。native数値やDO scheduling policyをContainerの公開fieldへ写さない。

## 6. Cloudflareの語彙・形が残る場所と扱い

### 6.1 判定方法

「Cloudflareという文字がある箇所」と「名前は一般語だがCloudflare APIを写した形」を分けて調べた。前者だけのgrepでは、runWorkerFirst、KVのlistComplete、multipartの等長条件、module handler ABI、Actorの101を見落とす。一方、get／put／bucket／cron／SQL／ETag等の一般語までCloudflare専用語とは判定しない。

現行v2本文17件には、直接のCloudflare製品名を必須fieldとして要求する箇所は今回の検索では見つからなかった。主な問題は、既に中立化された管理specよりも、**runtime shape、固有の設定名、backend制約の持込み**にある。付録Aは位置を再確認するための全検索ヒット一覧であり、全ヒットを削除すべきという意味ではない。

### 6.2 漏れ・由来・具体的変更の一覧

| 群 | 現れる語彙・形 | 主な所在 | 1.0での処置 |
| --- | --- | --- | --- |
| 製品／tool名 | Cloudflare／Workers for Platforms／WfP／workerd／Wrangler／Miniflare／workers.dev | T/Pの過去ADR・portability docs、FormsのActor proposal、S/Qのnative runtime・operator文書 | 公開Formの要件や利用者設定にしない。実装説明の正しい固有名、履歴、旧immutable evidenceは保持。 |
| runtimeのnative型 | D1Database／D1PreparedStatement／R2Bucket／R2Object／KVNamespace／Fetcher／DurableObjectState／DurableObjectNamespace／WorkflowEntrypoint | S/Qのprovider-private adapter、過去proposal、SDK型 | 公開型はSqlValue／Blob／Request／Response等へ。内部adapterの正しいnative型名は残す。D1.prepare／batchをsqliteの公開APIへ戻さない。 |
| Workerのdispatch | module default export、fetch／scheduled／queue、env、ctx.waitUntil、scheduledTime | F ModuleWorker／WorkerVersion／Cron、旧worker.runtime、S wrapper／scheduler | default export・handler名・envはTakoform固有ABIとして明記して維持。contextへsignal、時刻はscheduledAtへ。WHATWG Worker仕様そのものとは言わない。waitUntilはService Workerにもある語で、名称自体の一律禁止は不要。 |
| 互換性設定 | compatibilityDate／compatibilityFlags／compatibility_date／compatibility_flags | T/P ADR0019等の説明・拒否例、旧host-api、S native workerd準備 | 現行Formにも基本的に存在しない。今後も入力にしない。adapterのnative pinとFormのruntime契約を別管理する。過去文書の語を消して意味を変えない。 |
| assets | runWorkerFirst／run_worker_first、notFoundHandling／not_found_handling、single_page_application／single-page-application | F WorkerVersion:30,59–60,143–151、use-host、P edgev2／v2_schema、S worker-specs／static／code runtime | order／fallbackへ。CFのnative設定への変換はadapter内へ閉じる。改名だけで探索順を変えない。 |
| KV | getWithMetadata、expirationTtlSeconds、listComplete（native list_completeに似たprojection）、cacheTtl／read type選択 | F EdgeKVNamespace:69–101、edge.kv、module-worker.edge-kv、S/Q KV facade | getのentry結果／ttlSeconds／items＋nextCursorへ。native cache options・型判別文字列・provider cursorを公開APIへ追加しない。 |
| object | R2 object型、httpMetadata／httpEtag／uploaded／onlyIf／etagMatches／etagDoesNotMatch／delimitedPrefixes、独自uploadedAtMillis／partial／range結果 | S edge-objects-adapter／selfhost-object-store、Q R2 executor、F ObjectBucket | HTTP Headers／ResponseとS3語彙へ。nativeのhead/getでbody有無が変わる戻り値をそのまま出さない。uploadedAtMillisはLast-ModifiedまたはlastModified:Dateへ。 |
| multipart制約 | non-final partの等長、stream contentLength必須、5GiB／300MiB等のprofile | F ObjectBucket／旧edge.objects、S/Q store | **等長part＋stream Content-Length必須**を全backend共通のS3 subsetとして採用。未知長staging／非等長再構成は1.0必須から除く。数値をWeb標準由来と説明しない。 |
| Queue | queue batch／sendBatch／maxBatchSize／maxBatchTimeoutSeconds／maxConcurrency／maxRetries／deadLetterQueue、127000-byte message | F Queue／Consumer／ModuleWorker、旧edge.queue、S/Q wrapper／queue scheduler | 一般的な用語と設定は維持。Cloudflare MessageBatch／Queue型を公開せず、Blob・Date・signalへ。sendBatch全件原子性など独自に強い保証はnativeへの直結で代替しない。 |
| Actor | idFromName／newUniqueId／stub、storage／alarm／Host-owned sockets、特殊101 Response | F Actor:108以降、旧worker.actor／module-worker.actor、S actor-upgrade-handoff／class execution、Q actor owner | caller名をgetByName／createIdへ。ID・single-writer・永続化の意味は明示維持。server upgradeは独自型へ分離。Durable Object classやWebSocketPairを要求しない。 |
| Workflow | WorkflowEntrypoint、step.do／sleep／waitForEvent、create／get／sendEvent／terminate | F DurableWorkflow、旧worker.workflow、S workflow runtime | native class継承やprovider instance IDは不要。一般methodはTakoform契約として維持。JSON durable data・replay・停止境界を正本にする。 |
| Domain／route | zoneId／accountId／scriptName／dispatch namespace／custom hostname／SaaS／workers.dev | S/Qのprovider／gateway／runtime deployment、過去T/P文書 | 利用者はhostnameとUIDのみ。native account／zone選択はHost所有、tenant BYOCを導入しない。public hostnameの可変suffixへ依存させない。 |
| Container | native DO wrapper／Containers application／scheduling policy、CLOUDFLARE_予約prefix、container-httpという候補名 | K、FC Container:environment、Q/S実装・提案 | 予約prefixはvendor中立にし、OCI＋HTTPのみ公開。native image catalog／process APIはHostへ。container／container-httpの二重公開を避ける。 |
| encoded bytes | `{encoding:"base64",data}`をアプリにも要求するSQL、異なるBLOB／byte表現 | F SQLite／Actor、旧edge.sql、S wrapper／sqlite projection | Cloudflare専用ではないがWeb標準の型利用を妨げる独自wire。Uint8Array／Blobへ分離し、管理wireや内部transportにだけ残す。 |

§6の固有名やnative型は、Cloudflareをbackendとして実装するコードから機械的に消す対象ではない。公開契約・アプリが触る型・設定から切り離す対象である。immutableの旧仕様やrelease closureは履歴として残す。

## 7. 1.0確定・実装・公開の段取り

### 7.1 日付と出口

10/21はTier 1を先に固める設計レビューの目標、11/04は第2段階の中間判断とする。**日程到来だけで1.0へ昇格しない。各Formを完成まで0.xで反復し、仕様が問題ないという判断とCTO確認を昇格の条件にする。** Container／Actorの必須化に伴う見積りは担当者が再提示し、この文書で新しいローンチ日を捏造しない。

| 期間／時点 | 成果物と順序 | 完了条件 | 主担当案 |
| --- | --- | --- | --- |
| 10/14–10/16 | 第1段階12 Form／4 Bindingの差分、標準型、エラー、上限とテスト例を先にレビュー | 現行0.x・次の0.x候補・昇格後の1.0目標を対照化。Container／Actor必須への接続も列挙 | CTO＋Forms＋Takoserver、ProviderはHCL/state影響を確認 |
| 10/17–**10/21 先行レビュー目標** | Tier 1の設計、WorkerVersionの必須Binding構成、registry方針、§9.3の標準差異を詰める | D04／D05／D07、KV TTL、SQL値域、cron／serviceの受入例が一意。未完成なら0.xの次版で反復し、日付だけで昇格しない | Forms＋Host。標準差異と版構成をCTOへ報告 |
| 10/22–10/28 | Tier 1の検証と並行し、**Container・Actor／WebSocket upgradeを必須作業として進める** | Container Draft PR #17継続。OCI／egress／health／drain、Actorの型・socket／再接続・消費側Bindingを0.xで完成へ近づける | Forms＋Takoserver／private、Provider |
| **11/04 中間判断** | 必須機能の完成度と見積り、hosted CustomDomain等の到達点を確認 | Container・Actor／upgradeの未完はローンチ未充足として追跡。hosted CustomDomainは必須外で未完なら未対応として公開 | CTO＋各owner |
| **各Formの1.0昇格前** | 完成した0.xの仕様・差分・適合証拠と昇格先exact URLをCTOへ提示 | **CTO確認結果を記録してから1.0昇格・公開へ進む**。不足があれば0.xで反復。公開済みidentityは上書きしない | Forms owner＋CTO |
| **Provider 5.0.0公開前** | 1.0 mapping、state／import、必須Form・Binding coverageと移行証拠を提示 | **Form昇格とは別にCTO確認**。版番号・mappingと公開対象commitを記録 | Provider owner＋CTO |
| ローンチ前 | **Tier 1＋Container＋Actor／WebSocket upgrade**の実装・適合・昇格・実利用資格確認 | 必須graphの全exact契約に証拠がある。takoserverは1.0のみ対応。既存hosted 0.xは§8で効果確定・移送・退役方法を決定し、新Hostへ残さない | 各repo owner／operator |

#### 0.xから1.0へ昇格する条件

1. 各Formの0.x候補に語彙・型・失敗・上限・永続化・全参照先を定義し、完成まで版を進める。必要な0.x追加公開は可能であり、旧版の本文・package・tagを上書きしない。
2. source・spec・正負fixture・実backend受入が同じ候補を検証していること、§9.3の標準差異に処置があることをownerが確認する。Container／ActorではBindingを通じた実利用とWebSocket upgradeを含める。
3. **Formごとに1.0昇格前のCTO確認**を受ける。確認対象の0.x版・commit／digest、差分、未解決事項、1.0のexact URLと依存先、確認結果を記録する。今回の方針通知は個別候補への昇格承認ではない。
4. 確認済み候補から1.0の新identityを作り、版とexact参照の置換に伴う差分を再検証する。昇格は契約の成熟度の判断であり、既存Resourceのform／UIDやProvider stateの上書きではない。
5. Form公開、Provider 5.0.0公開、takoserverのsupport・activationはそれぞれ別の行為。Provider 5.0.0公開前のCTO確認も独立して記録する。この文書更新ではいずれも実行しない。

### 7.2 repoを跨ぐ実装順

この調査ではtask ledgerを作らない。着手時にはcross-repo／persisted state／authority変更を含むtask ledgerをownerが作り、次の対象repo・順序・既存資源の扱いを宣言する。

1. **takoform-forms**: 0.xの本文・Binding章・fixtureで反復し、完成・CTO確認後に各Formを1.0へ昇格する。**[Container Draft PR #17](https://github.com/tako0614/takoform-forms/pull/17)はそのまま継続してよい**。これは当該PRのmerge・公開・昇格や、その未読の最新差分を承認したという意味ではない。既存freeze・published bytesは更新しない。
2. **takoserver**: 複数版registryの能力と対応版の許可方針を分け、**提供するFormは1.0のみ**とする。共通v2 engineのwireを保ってvalidator／backend／adapter・broker・normal entryを接続する。0.xを互換read／recovery／delete用entryとして新Hostへ残す案は取り消す。既存0.xの整理は旧環境の移行作業として先に解決する。
3. **takoserver-private**: WfP wrapper／image owner／mixed topology／native capacityを必須graphに合わせる。public repoはportable contractとpublic port、private repoは具体的Cloudflare backendとoperator構成を所有する。0.x候補の検証は隔離した試験環境で行い、提供Hostのsupportへ混ぜない。
4. **terraform-provider-takoform**: exact1.0 codec、必須14 Form／6 Bindingのmapping・coverage、state拒否／import、元intentの連続性を実装する。**Provider 5.0.0公開前にCTO確認**。旧0.xをlatest解決やstate文字列変更だけで1.0へ読み替えない。
5. **takoform**: Host API v2の変更は不要。必要ならinformativeな昇格・移行案内だけをownerが検討する。Form catalog・Host固有adapter・署名packageを共通v2の参加条件に持ち込まない。
6. 各ownerが最終candidateで `bun run check`。focused fixture、portable gate、real backend、CTO確認、正式公開readback、実利用qualificationを別々に記録する。公開は新identityへcreate-onlyで行い、bytes／source commit／digestをreadbackする。Hostは対応対象の全required behaviorの確認後にsupportを広告する。

#### WorkerVersionの初回安定版と必須Binding

**本案の推奨は、未昇格のWorkerVersion 0.xへedgeKvBindings／sqliteBindings／objectBucketBindings／serviceBindingsに加えactorBindings／containerBindingsを統合し、完成した構成を初回1.0.0へ昇格すること。** 対応WorkerDeploymentも同じgraphで0.x検証を行い、昇格先のexact参照表を固定する。ModuleWorker側のActor class／upgrade ABIもローンチまでに閉じる。未定義配列の予約は行わない。

直前の「containerBindingsを1.1.0へ送る」版構成は、Container必須・takoserverは1.0のみという最新方針との整合確認が必要であり、**現在の確定構成としては扱わない**。上記の初回1.0統合は文書上の提案で、採番の最終判断は昇格前のCTO確認へ出す。すでに1.0が固定済みと判明した場合はそのidentityを改変せず、別の版構成とローンチ依存をCTOと決め直す。1.1の対応を今回勝手に許可しない。Queue／Workflowの配列・消費側の版番号は後続設計とし、未完成fieldをローンチ用1.0へ予約しない。

#### exact URLごとの複数版登録と参照の仕組み

Formごとの複数版を扱える登録機構は維持するが、**登録できる構造と、提供Hostがサポートする版は別である**。Hostがコードで所有するregistryは完全なForm URLをkeyとし、専用parser／正規化、backend factory、runtime projection、参照表、対応許可を結び付ける。新takoserverの初回提供対象は昇格・CTO確認を経たexact1.0.0のみとする。0.xや将来版へSemVerから互換性を推測しない。新しい公開APIやForm取得・実行機構は追加しない。

| 対象 | 登録・対応方針 | 参照・回復の扱い |
| --- | --- | --- |
| 開発・公開履歴の0.x | Formsの開発履歴と隔離試験で扱い、新takoserverのcomplete／retained supportへ登録しない | 新Hostは0.x要求を受け付けず、1.0へredirect・自動変換しない。既存資源の効果確定・回復・移送は旧環境で先に扱う |
| 初回1.0.0の各Form | そのFormの完成候補とCTO確認に対応するparser／backend／runtimeをexact登録 | WorkerVersionの6必須BindingとWorkerDeploymentの参照表を完成仕様へ固定。新Hostでは受理した1.0 Operationを元identityで回復 |
| 将来の別安定版 | registry構造は同じkindの別exact URLを区別できるが、追加のHost対応は別判断 | 互換性・許可参照・保存identityを版ごとに検証。登録機構の試験だけで将来版をsupported:trueにしない |

1. 新規受付はexact URLと対応許可を両方検査する。0.xはread／recovery／deleteを含む互換サポートを新takoserverで提供せず、未対応として既存v2の規則で拒否する。0.xの元intentを1.0 requestに付け替えない。
2. 消費側Form URL、Binding種別、target Form URL、runtime ABI、owner／Space／UID／権限の許可組を定義する。kind一致やversionの大小比較で参照を通さない。
3. 新takoserverが受理した1.0 Resource／Operationは元のForm URL、backend identity／target／execution keyで継続・再起動回復する。registry更新後も新entryやwrapperへ付け替えず、欠落時に別backendへ再createしない。
4. 新ABI用wrapperを新世代として作り、旧環境の0.x wrapper／data／authorityとは分離する。既存0.xがある場合は移送・退役の完了を確認して切り替える。0.xデータを新Hostの起動時に消去する実装にはしない。
5. single-activeやhostname／参照／data所有の排他は、登録版によらず論理UIDで検査する。異なるForm URLへの移行は新UIDとし、PUTでformを変更しない。

受入では、1.0の正しいparserと参照選択、0.xの全互換操作拒否、wrong parser拒否、1.0元intentのOS再起動回復、wrapper世代保持、版を跨ぐ排他を確認する。複数版構造は非提供fixtureで別URLを登録して検証できるが、それを0.xや1.1の実対応の証拠にはしない。既存0.xの回復試験は旧環境・移行手順で行い、新Hostのsupportへ混ぜない。

レビューVのTier 1縮小案32–53人日等は、Container／Actorを必須に戻す前の仮説であり、現行スコープの見積りとして流用しない。Takoserver担当が必須graphと実backend受入を再見積りする。新Hostのqualificationは昇格後の1.0が対象で、0.x候補の適合試験と昇格時のexact参照検証は別に保持する。今回の文書更新でlaneの停止・変更は行わない。

### 7.3 必要な受入matrix

**ローンチ必須の受入はTier 1＋Container＋Actor／WebSocket upgradeと、昇格・registry・既存0.x移行の前提を含む。** Queue／Workflow／hosted CustomDomainは後続対応時の検証対象である。Container・Actorが未完なら未充足として追跡し、hosted CustomDomainだけは必須外・未完時未対応とする。

| 横断検査 | 最小ケース |
| --- | --- |
| API v2据置き | discovery／support exact URL、直接CRUD、expected generation、元request＋keyのlost ACK、同persistent storeでOS process再起動、unknown効果、元秘密補給。旧URLの意味が不変。 |
| 昇格／複数版登録／0.x移行 | 完成0.x候補とCTO昇格確認の一致、1.0のみのsupport／受付、0.xの互換read／recovery／deleteも提供しないこと、wrong parser・不許可参照拒否、1.0元backend／wrapperの再起動回復。既存0.xは旧環境で効果確定・移送・退役方法を確認。Provider 5.0.0公開前確認は別に記録 |
| Web型 | Request／Response実物、Blob／BufferSourceのbyte offset、二重消費、clone、Headers重複とSet-Cookie、locked／disturbed stream、cancel／backpressure、abort前後、head後stream error。 |
| Worker＋asset＋service | static-only／code＋asset、二つの探索順、fallback／危険path、GET／HEAD／POST、targetだけ更新、weighted切替、実stream途中のdrain／退役。 |
| SQL＋migration | BLOB往復、数値範囲、RETURNING付きwriteのrollback-only query、全statement結果検査前commit禁止、authorizer、既適用prefix拒否、file＋ledger原子性、途中失敗／再起動／suffix repair。 |
| KV／bucket | KV TTL59秒拒否／60秒受理、metadata・cursor／UTF-8境界、既知長body、Content-Length欠落／不一致／超過拒否、等長non-final partと短いfinal part、非等長拒否、conditional race、読取Responseと書込型付き結果、途中取消、tenant prefixとcleanup。 |
| Cron／Queue | 手動HTTPなしの自動発火、同じoccurrence再配送、旧新schedule境界、OS再起動、message重複／ack lost／retry／DLQ／batch原子性。 |
| **必須Actor／Container** | 同ID逐次／stream終了、eviction・alarm、**WebSocket upgrade・message／close・再接続・退役**、OCI digest／health前切替拒否／egress両値／drain／再起動／delete while bound。実Bindingから到達できること |
| 後続Workflow | history replay／step互換／wake／physical terminate。旧historyの移送と旧環境の終了条件を別途確認 |
| 実利用経路 | Cloudflareの実backend、Dockerの利用入口、Linuxの利用入口で同じcases。backendごとの不足・部分profileを明示。mock3種を3環境適合と数えない。 |

Rの主な不足である通常profileのmixed composition、hosted MigrationApplication／CustomDomain、full asset serving、全Binding、実clock、real backendのcleanupをこのmatrixへ取り込む。sourceや過去local PASSだけでcurrent全型PASSに更新しない。

## 8. 互換性と0.x利用者の移行

### 8.1 昇格する仕様と移行するリソースを分ける

- **1.0は0.xからの昇格である。** 0.xの各版で仕様を反復し、問題ないと判断できた完成候補をCTO確認後に1.0へ進める。0.xの追加公開を可能とする。公開済み0.x URL、旧v1 package／tag／schema／releaseのbytesは上書きせず、修正は新しい版へ進める。
- 同じHost API v2を使い続ける。Formの昇格のために `/v3`、新wire、Binding CRUD、package v2、SDK強制は追加しない。
- **仕様の昇格は既存Resourceのin-place upgradeではない。** FormはUIDの間不変なのでPUTで0.xから1.0へ変えない。新1.0 Resourceは別UIDとし、data・参照・入口は明示移送する。0.xと1.0の本文が同じ意味に成熟しても同じidentityとはみなさない。
- **新takoserverは1.0のみサポートし、0.xの新規操作だけでなく互換read／recovery／deleteも提供しない。** 直前の0.x retained案はこの方針へ置換する。複数版registryは能力として維持するが、提供対象の許可方針で0.xを外す。
- 0.x候補の開発・追加公開と、takoserverによる提供は別である。候補は隔離した検証環境で確認し、新Hostが0.xも提供するとは広告しない。**0.x／1.0の90日並行サポートは行わない**。

**既存hosted 0.xがあれば、新takoserverへの切替前に移行方法を決定し、未処理の旧intent・data・参照をどう解決するか証拠を揃える。** Provider未掲載から利用者ゼロを推測しない。元intentの回復・data取出し・削除が必要なら、旧環境を隔離して行うかHost owner所有の移行手順で扱い、新takoserverの0.x互換機能として実装しない。単にregistryから0.xを外して既存dataを見失う切替は禁止する。

| 切替前に決める項目 | 必要な内容・証拠 |
| --- | --- |
| 対象・元効果 | exact URL／UID／backend identity、active graph、pending／unknown Operationと元intent、private inputの供給元。旧環境で効果を確定・回復する手順と、その環境を終了できる条件 |
| 昇格と移送 | 移行先Formの完成0.x候補・1.0昇格確認、data／TTL／history移送、書込停止境界、Endpoint／Domain／Cron／依存参照の切替、ownerと実行順。新1.0は別UID |
| 旧版固有の作業 | repair・detach・cutover・参照安全なDELETEを旧環境で済ませる順序。新Hostの0.x read／recovery／deleteに依存しない。元requestを1.0へ改ざんして再送しない |
| 未対応の後続機能 | Queue／Workflow／hosted CustomDomain等の移行先がなければ、停止・外部移送・旧環境の移行用保管などの扱いを先に決める。Container／Actorは必須なので未対応のままローンチしない |
| 検証・失敗時 | new／old UID対応、data整合、秘密再補給、停止時間、保持期間と責任者、戻し方またはforward repair。旧環境へ戻す場合も新Hostの0.x対応を再開する手順と混同しない |

未解決の既存0.xがあればownerが切替前にCTOへ報告し、移行・退役の扱いを確定する。0.xだけを1.0という文字列へ置換することや、今回の方針からdata削除の承認を導くことはしない。

### 8.2 通常のv2 0.xからの手順

1. **ローンチ前の棚卸し・移行方法決定**: old exact URL／UID／generation／Operation、backend identity、private inputの供給元、全参照、Endpoint／Domain／Cron、SQL migration履歴、未処理Queue、Actor／Workflow dataを記録する。§8.1に従い旧環境で効果を確定し、新takoserver切替前に移送・退役条件を満たす。pending／unknownの効果を確定する前に同名のnew createを重ねない。
2. **事前確認**: 移行先Hostのexact1.0 Support／Offeringとクライアント対応を確認。未対応のTier 2への移行を強行せず、個別の保持・停止・移送方法を使う。新Resourceは同Spaceで別名を使うか別Spaceへ置く。同SpaceのResource名はFormを跨いで一意なので、同名共存を前提にしない。
3. **アプリ変換**: 旧値→新値の明示変換を行い、新しいbundle／asset manifestとdigestを作る。互換helperはアプリ側の任意ライブラリとして提供できるが、Hostの1.0 runtimeへ旧メソッドaliasを注入しない。
4. **保存data**: 移送先の1.0 storage Resourceを先に作成する。SQLは整合snapshotとschema／ledgerを検証。KVはvalue／metadata／残TTL、bucketはkey／body／metadata／validatorに関する差を検証してcopyする。移行元をread-onlyまたはwrite pauseにする境界、必要なら追いかけcopyを定義する。durable dataに完全なrollbackがない場合、旧側への再copy／forward repairを明記する。Hostが検証付きrestoreを提供しない場合は、新DBへのmigration適用＋利用者dataの移送を選び、ledgerを直接捏造しない。
5. **新graph作成**: 作成・検証した1.0 storageと新bundle／WorkerのUIDを用いてVersion／Deployment／Endpoint／schedule等を作る。秘密はprivate入力経路から再供給し、old stateやlogから抽出しない。source bytesの再利用にもowner認可とdigest照合を行う。
6. **動作確認と入口切替**: 新EndpointのHTTPS、SQL／KV／bucket、service、asset、cronに加え、必須Container・Actor／WebSocket upgradeを確認。CustomDomainは旧占有のまま勝手に新UIDへ移せない。Hostが所有する明示切替手順がなければ、停止を伴うdetach／attachを計画する。Endpoint割当URLは変わる可能性をアプリ側へ通知する。
7. **旧側退役**: 旧環境でCron／Consumer／入口の新規配送を止め、inflightと不明効果を処理する。新takoserverの0.x互換操作は使わない。参照順にDeployment→Version→storage等を削除する。Actor／Workflow dataは明示DELETEまで残し、migration ApplicationのDELETEでdownしない。
8. **完了証拠**: new exact URL／UIDとold UIDの対応、データ件数・digest／query結果、旧route／owner不在、残存保持と戻し方をoperator側に記録する。秘密やproduction記録をpublic repoへcommitしない。

### 8.3 型・設定の移行対応

| 旧0.xの利用コード／設定 | 新1.0への明示変換 | 自動変換しない条件 |
| --- | --- | --- |
| vars／requiredSensitiveVars | values／secretNames | 秘密値は含めない。公開varsに混ざった秘密を値のまま引き継がない。 |
| kvBindings／bucketBindings | edgeKvBindings／objectBucketBindings、新Resource UIDへ | old targetUIDを新Formのtargetとして使わない。 |
| runWorkerFirst:false／true | order:assets-first／handler-first | field aliasをHostが推測しない。 |
| notFoundHandling:none／single_page_application | fallback:null／index.html | index.htmlが保持済みmanifestに無い場合は拒否。 |
| SQL Base64 object | `Uint8Array`をencode/decode helperで生成 | 不正Base64や範囲外numberを丸めて救済しない。 |
| KV get／getWithMetadata、expirationTtlSeconds | getのentry.valueからarrayBuffer／text／stream、ttlSeconds | getが返したentry全体をbytesとして扱わない。 |
| bucket.getのnull／独自object、ifMatch／range option | Response.status／body／headers、If-Match／Range | opaque ETagをchecksumとして再生成しない。multipart uploadIdは持ち越さない。 |
| bucket.put／completeの旧結果、delete | put／completeはObjectWriteResult、deleteはvoid、条件不成立はprecondition_failed | put／completeをResponseとしてstatusで読む新案は採らない。streamはContent-Length必須、partはnon-final等長。 |
| WorkerVersionの0.x→必須Bindingを含む初回安定版 | 0.xでactor／containerを含む構成を完成させ、CTO確認後に昇格先exact URL・新UIDと対応Deploymentを作成 | 旧1.1.0送りを確定扱いせず、版構成を§7.2で確認。公開済み1.0へのfield後付け・PUTでのform変更をしない。 |
| scheduledTime | scheduledAt.getTime() | occurrence重複抑止のnamespaceを別triggerへ流用しない。 |
| Actor idFromName／newUniqueId | getByName／createId | old namespaceのIDとdataの所有権を新namespaceへ推論しない。 |
| catch(error.name===snake_case) | 標準例外と独自codeを分けてcatch | nameだけをcopyしてHost由来の権威あるerrorに見せない。 |

### 8.4 Provider／Terraform／OpenTofu

TASK-0098は既存v1 client／stateからの自動fallbackを採らず、fresh v2、exact URL、UID import、durable intent journal、apply-time秘密供給を選んでいる。これを1.0移行のために壊さない。

- Pの13型のv2 mappingは現行0.x URLをcompile時に列挙する。1.0採用はProvider自身のreleaseで行い、単なるendpoint変更やlatest URL解決にしない。13型以外のStaticAssetBundle／CustomDomain／Actor／Workflowと追加Containerをどうtyped公開するかはProvider ownerの対象拡張になる。
- 推奨は既存stateを無言で再解釈せず、1.0用の明示mappingとnew resource addressを用意し、旧Provider／stateを保持しながら移行できる形にすること。同じtype名を使うならForm identityの変更を検出してHost要求前に拒否し、別majorまたは明示schemaで切る。Providerの公開対象は5.0.0とし、type名・mapping・state対応を§9で詰め、公開前にCTOへ確認する。
- stateのUIDやform文字列を `state mv`／`replace-provider` だけで1.0化しない。importは既に存在するnew1.0 UIDを検証して関連付ける操作で、backend所有権やdataを移す操作ではない。
- intent journalは初回send前にexact Host／Form／request／key／時刻／replay windowを保存する。変換済みの1.0 requestは新しい意図であり、旧0.xの不明結果に対するretryへすり替えない。元key・元requestを後から正規化して書き換えない。
- privateInputsの値／単純hashをplan／state／journalへ保存しない。ACK後state保存前のprocess終了はGET照合、ACK不明は停止して元intentを保つ。1.0の型変換を再送の根拠にしない。

### 8.5 旧Host API v1の利用者

「v2内の0.x→1.0」と「旧Host API v1→v2」は別の移行である。旧v1 packageのBindingが1.0.0でも新v2 Form1.0と同じidentityではない。v1資源は旧Host／旧Providerで保持・照合し、必要なdataを移してfresh v2 graphを作る。v1→v2 HTTP translatorやstateの自動変換は導入しない。

Sのself-host文書は使用済みv1.0.0 data rootのin-place upgradeを拒否し、新しいrootへのfresh installと再作成を案内する。今回の1.0化を理由に既存rootを初期化・DB削除しない。旧環境を残す期間、未完Operation、旧writer停止、wallet等の別product stateはHost ownerの責任で確認する。

## 9. takoserverへの具体的影響と合意が必要な論点

### 9.1 実装箇所と変更の種類

以下はS基準のsourceから確認した接点。fileが存在することは、今回提案する1.0実装やlive適合の証明ではない。

| 接点 | 変更案・残す境界 |
| --- | --- |
| `src/takoform-v2/forms/worker-specs.ts`、`worker-references.ts`、各Form parser、`application.ts`／config | exact URLの複数版registryと提供版の許可方針を分離。**新takoserverは1.0のみ対応し、0.xをcomplete／retainedへサポート登録しない**。WorkerVersionのactor／container構成とDeployment参照表は0.xで完成後に昇格。受理した1.0の元intent回復とengine／routes／wireのv2契約は保つ。旧0.xの整理は別環境の移行作業。 |
| `worker-code-eligibility.ts`、`worker-code-runtime.ts`、`worker-bundle-runtime.ts`、`worker-material-validation.ts` | new manifest media type、全参照ABI、Blob等のglobal、class/export検査。コード検査の副作用禁止とartifact custodyを維持する。 |
| `worker-static-runtime.ts`、`worker-static-publication.ts`、`workerd-runtime.ts` | order／fallbackをnative設定へ変換。percent decode／path安全性、HEAD／HTTP header、static-only判定、公開入口とserviceの一致を確認する。native設定名はadapter内に残せる。 |
| `providers/selfhost-worker-wrapper.ts`、`selfhost-worker-prelude.ts`、`selfhost-v2-sqlite-worker-projection.ts` | JSのBLOB／Blob／context／error facadeを変更し、旧projectionとnew projectionをexact契約で選択。generated sourceは元sourceから作る。全native Bindingをそのままenvへ漏らさない。 |
| `forms/sqlite-worker-binding-authority.ts`、`providers/selfhost-v2-sqlite-binding-broker.ts`、`selfhost-v2-sqlite-store.ts`、migration native ports | token／callerVersion／targetUID／generationのgrantを維持して型変換。SQL authorizer／transaction commitの前後を変更しない。migration履歴移行は別persisted-schema/data作業。 |
| `providers/edge-objects-adapter.ts`、`selfhost-v2-object-bucket-*`、`selfhost-object-store.ts` | get／headのResponse、put／completeの型付き結果、Headers、標準body、Content-Lengthとnon-final等長partの検査、物理prefix／part owner回収。未知長staging／非等長再構成を1.0のscopeへ追加しない。 |
| `providers/selfhost-v2-kv-store.ts`／broker、Q `v2-worker-edge-kv-wrapper.ts`／key codec／cursor runtime | get entry／Blob、TTL改名、cursor shapeとscope。Qのraw／prefixed backend identityを上書きして既存dataを見失わない。 |
| `worker-service-resolution.ts`、`service-binding-authority.ts`、`selfhost-v2-service-binding-broker.ts` | URL object入力、RequestInit、abort／stream、private header除去、target切替を新ABIで検証。UIDが変われば新grant。固定UID配送・redirect非追従と標準fetchの差、RequestInit各項目の扱いは§9.3 WS-02で別途解決する。 |
| `worker-cron-trigger-scheduler.ts`／backend、Qのclock composition | 永続match／leaseを維持し、scheduledAt／occurrenceIdへ投影。native cronでもDO alarmでも同じskip／at-least-once／再起動の意味にする。 |
| `worker-queue-*`、queue binding authority／runtime wrapper | Blob／Date／signalを導入。sendBatch・ack／retry・DLQ・consumer停止時の未settled義務は保持。native機能の部分profileとcomplete supportを分ける。 |
| `actor-class-execution.ts`、`actor-upgrade-handoff.ts`、`actor-forward-runtime.ts`、namespace graph | public ActorResponse subclassを廃し独自upgrade resultへ。single-writer／child退役／旧owner fenceを保持。ID変換とstore／alarmはschema／data移行の影響を別検討。 |
| `workflow-*`、`selfhost-v2-workflow-composition.ts` | contextとエラー境界を整える。memoized履歴やowner brandを新旧runtimeで混ぜない。sleep／waiting公開前の物理停止、terminate／deleteの順序を維持する。 |
| `providers/selfhost-container-{lifecycle,runtime}.ts`／docker-http-revision、QのContainer broker | 既存Provider-private runtimeをv2 Resource/Operation authorityへ明示接続。legacy incarnation／rootと新UIDの所有対応を証明し、image／egress／health／stream／drainを完成する。 |
| Endpoint／CustomDomain、normal entry composition、Q `entry-cloudflare-v2-host.ts` | 正しい1.0 backendが揃ったmapだけをadvertise。新Hostへ0.x retained対応を混ぜず、必須Container／Actor graphも接続する。hosted CustomDomainの未完は必須外として明示し、mixed profileの未接続をSupportで隠さない。 |

Qで変更が必要な代表箇所は `v2-worker-edge-kv-wrapper.ts`、`v2-object-bucket-r2-data-executor.ts`、`v2-object-bucket-namespace.ts`、`v2-sqlite-node-authorizer-service.ts`、`v2-wfp-canonical-*`、`v2-worker-*-binding-*` とその生成wrapper／receiptである。public契約をprivate accountの事情で書き換えず、実装不能な必須項目を先にCTOへ返す。

### 9.2 決定事項 — 最新Shouta方針、CTO決定とTakoserverレビューへの回答

入力はV（REVIEW-takoserver.md）、直前のCTO決定、およびそれを一部上書きする最新のユーザー通知のShouta方針である。**最新通知を優先し、Container・Actor必須化、0.x反復からの昇格、takoserverは1.0のみ、2種類のCTO確認を現行方針とする。** 技術案・不同意点・残る設計判断は分ける。この決定は個別Formの昇格確認やProvider公開確認を済ませた証拠ではない。

#### 9.2.0 最新通知で確定した方針と置換範囲

| 決定 | 現在有効な内容 | 置換した旧決定／実装・確認への反映 |
| --- | --- | --- |
| **D00／D10／D12 ローンチ必須** | **Tier 1に加えContainerとActor（WebSocket upgradeを含む）を必須とする** | 直前およびキュー済みCTO決定3番のContainer・Actor必須外を置換。14 Form／6 Bindingとupgradeのgraphを完成させる。11/04で未完でも必須を外さない |
| **各Formの1.0昇格** | **完成まで0.xで反復し、仕様として問題ないと判断できたら1.0へ昇格。昇格前にCTOへ確認** | 「0.x追加公開なし」「1.0を直接公開」の方針を取り消す。0.xの次版は追加公開可能。10/21は先行レビュー目標で、自動昇格日ではない |
| **takoserverの対応版** | **1.0のみサポートする** | 「ローンチ後0.xは新規停止・読取／復旧／削除のみ保持」を置換。新Hostには0.xの互換対応を残さず、既存0.xは切替前に旧環境での移行・効果確定・退役方法を決める |
| **Provider 5.0.0** | **公開前にCTOへ確認する** | Formの昇格確認とは独立した確認。mapping／state／import／必須coverageと公開対象を提示し、結果を記録 |
| **D09 hosted CustomDomain** | **ローンチ必須外を維持。11/04中間判断で未完なら未対応として公開** | 最新通知で変更されていない部分は維持。Container／Actorの扱いと混同しない |
| **D04／D05／D07** | 型付き結果、等長non-final part、streamのContent-Length必須、asset Range任意、fallback null／index.html限定の承認を維持 | Web標準との衝突・独自拡張は引き続き§9.3の別一覧でCTOへ報告 |
| **複数版registry** | 構造として複数版を区別し、takoserverの提供版は1.0に限定 | §7.2。旧0.xを提供登録しない。WorkerVersionの初回安定版へactor／containerを統合する案と旧1.1.0送りとの整合は、昇格前のCTO確認で決める |
| **Container Draft PR #17** | **そのまま進めてよい** | 0.xでの完成を進める許可として記録。この文書更新でPRのコード・状態を変更せず、merge・昇格・公開承認へ読み替えない |
| **今回の実行範囲** | PROPOSALと索引の更新、check、commit・pushのみ | 本番deploy・公開release、個別Formの昇格、Provider 5.0.0公開はいずれも行わない |

#### 9.2.1 反映した点と理由

| ID／期限 | PROPOSALで採用した内容 | 理由と反映先 |
| --- | --- | --- |
| D01・WV **10/21先行レビュー** | 語彙・設定を0.xで反復し、初回安定版の必須6 Binding構成を詰める。完成後、CTO確認を経て1.0へ昇格する案 | Container／Actor必須化と1.0のみ対応に整合させる。旧containerBindingsの1.1.0送りをそのまま完成条件にはしない。§4.1／§7.2 |
| D02 **10/21** | Web標準型、標準DOMException／abort reason。独自errorはname＋codeで判定し、isolate越しのinstanceofは非保証。client切断検知／callee abort通知だけbest-effort | runtime境界での実装可能性と、アプリが観測できる取消・unknown効果を分ける。Host deadline／退役・自身のbody取消は維持。§3.2 |
| D03 **10/21** | BLOB=Uint8Array、safe numberの範囲据置き、**full 64-bit整数／bigintは1.0不採用**。query rollback-onlyを維持。broker受理後SQLはabortしてもcommitされ得る | 内部Base64 wireと既存authorizerを保ち、precision lossや取消＝rollbackという誤解を防ぐ。§4.2–4.3 |
| D04 **10/21、CTO承認済み** | **get／headはResponse、put／completeはObjectWriteResult、deleteはvoid**。条件不成立は固定code precondition_failedで統一。metadata2048 bytes案を採用 | ストレージ独自APIとして採用し、global fetchやservice fetchのPromise<Response>を変更しない。書込失敗をnull／例外の二系統にしない。§4.1–4.3、§8.3、§9.3 |
| D05 **10/21、CTO承認済み** | **non-final partは等長、最後は短くてよい。streamはContent-Length必須**。未知長staging・非等長part再構成を1.0必須から削除 | ストレージBindingの共通S3 subsetとして限定し、標準fetch／Request／Streams一般の制約へ広げない。永続staging等の新しい必須lifecycleを増やさない。§4.3–4.4、§9.1、§9.3 |
| D05・レビュー追記 | 長さ未指定はTypeError／TAKOFORM_LENGTH_REQUIRED／status411、実長不一致はRangeError／TAKOFORM_CONTENT_LENGTH_MISMATCH／400、非最終part等長違反はRangeError／TAKOFORM_PART_SIZE_MISMATCH／400。読取り前拒否・未commitを明記 | Web標準の例外classを保ち、callerが無効果の入力拒否と不明な実行結果を区別できる。statusはruntime情報に限定し、管理wireへ持ち込まない。§4.3 |
| D06 **10/21** | KV get統合／Blob、ttlSecondsの**最小60秒**、items／nextCursor、eventualを最低保証。1.0 putにstreamを含めない | 既知長の値とTTLの共通profileに限定できる。Linux側が短いTTLを実装できても同じ意味を保つ。§4.1–4.3 |
| D07 **10/21、CTO承認済み** | text/javascript採用、**fallbackはnull／index.htmlだけ、asset Rangeは任意**。Content-Type／Length／ETag／If-None-Match／HEADは必須 | 任意fallback pathのnative emulationとRange新規必須化を避ける。標準ResponseとHTTPの意味は保つ。§4.3、§9.3 |
| D07・Cron **10/21** | scheduledAt:Date＋triggerUid＋occurrenceId。記録済み一致の配送義務をhostedも永続ledgerで守る。occurrenceIdはtrigger UIDとUTC分境界を区別できる形式で安定生成 | native cron直接発火だけに依存せず、元matchの再起動回復と再配送を実装する。具体的なDB・DO alarmはHost選択。§4.1、§7.3 |
| 複数版registry **ローンチ前** | exact URLごとにparser／backend／runtime／参照表を分け、提供版の許可方針で1.0のみに限定 | 新Hostの0.x retained共存案は取り消す。受理した1.0は元identityで回復し、旧0.xの整理は移行前の別環境で扱う。§7.2、§8 |
| D08 **後続・11/04中間確認** | QueueはBlob／Date／signal、sendBatch全件原子的を維持。stream producer入力を外す | bodyの無制限materializeを避け、native queueの弱い保証へ置換しない。ローンチ必須にはしない。§5.1–5.2 |
| D10・D11 **11/04中間判断／ローンチ必須** | Container／container、secret-free初期案、linux/amd64、TAKOFORM_予約、egress両値、Linux=systemd／Bun＋Docker Engine | 0.xで完成まで反復し、CTO確認後に1.0へ昇格。Containerを未対応のままローンチ条件から外さない。秘密注入を含む初期scopeは昇格前に決める。§5.4 |
| D13・D14 **最新方針へ置換** | takoserverは1.0のみ。既存0.xのhistory／元intent／dataは旧環境で効果確定・回復・移送・退役を扱う | 90日並行は設けず、新Hostへ0.x互換read／recovery／deleteを追加しない。仕様の昇格と新UIDへの移行を分ける。§8 |

#### 9.2.2 同意できない点・採用しない案と理由

| レビューの提案／解釈 | 本案の判断 | 理由・代案 |
| --- | --- | --- |
| 未完成の配列・ABIを初回1.0へ予約し、後で意味を決める案（V §2.3に関連） | **未定義fieldの予約には同意しない**。ただし旧案の「全Tier 2配列を1.1へ送る」も現在の確定構成として扱わない | actor／containerはローンチ必須なので0.xで完成させ、初回1.0へ統合する案をCTOへ確認する。Queue／Workflowは後続設計。公開済み版への後付けはしない |
| 利用者ゼロを推測して移行調査を省略する前提（V §4／D14） | **移行調査省略には同意しない**。新takoserverが1.0のみ対応する方針自体は採用する | 直接HTTP・内部利用・pending／unknown Operation・dataを棚卸しし、あれば旧環境での移行・退役方法を決める。0.x互換対応を新Hostへ残す理由にはしない |
| WorkerBundle1.0の閉集合へapplication/jsonも追加する（V §2.2） | **今回の1.0には足さない**。text/javascript／application/wasm／text/plain／application/octet-streamの4種 | 本体でJSON moduleを同じloader・export・import規則で検証した証拠を今回確認していない。旧仕様からの必須変更でもなく、Tier 1確定の対象を増やす。必要なら別exact URLで追加 |
| waitUntil共通上限を例として30秒にする（V §2.1） | **約30秒という見積りをそのまま規範値にしない**。有限の共通上限は10/21までに実装者と決める | handler種別、接続継続中／応答後、WfP dispatch・alarmの寿命を区別した実証が必要。client切断検知のbest-effortと、Hostが所有する期限・退役の強制取消は別 |
| D05追記の「HTTP経由のcontrol APIでは411を返す」 | **Host API v2への適用には同意しない**。runtime TypeErrorのstatus411は採用し、独立data-plane HTTPへの投影だけを別扱いにする | Host API v2を据え置く方針と、実行時Bindingに管理HTTP経路を足さない境界に反する。JSON管理要求にはそもそもReadableStream bodyを載せない。共通error envelope・statusの変更は本taskの対象外 |

#### 9.2.3 決定後も残る設計・実施事項

| ID／期限 | 残る内容 | 必要な材料／担当 |
| --- | --- | --- |
| 各Form **1.0昇格前** | 完成した0.x候補、仕様妥当性、標準差異、参照graph、昇格先exact URLをCTOへ確認 | Forms owner＋CTO。今回の方針を個別候補への昇格承認とせず、確認結果・candidate identityを記録する |
| WorkerVersion **昇格前** | Container／Actor必須・takoserverは1.0のみという方針と、旧containerBindings 1.1.0案の採番を整合させる | 初回1.0.0へ必須6配列を統合する案を推奨するが、最終採番はCTO確認事項。既存1.0が固定済みなら改変せず再判断 |
| D16 **ローンチ前** | Tier 1＋Container＋Actor／upgradeの実backend受入。0.x候補の完成試験と、昇格後のexact1.0 qualificationを分ける | Takoserver／operator。新Hostでは0.xサポートを広告しない。旧資源の回復・移行は旧環境で検証 |
| D14 **切替前** | 既存hosted 0.xの有無、元intentの効果確定、移送・repair・参照切替・旧環境の退役条件 | Host owner。新takoserverへ0.x read／recovery／deleteを残さずに移行できる証拠。保持期間・責任者・失敗時の修復を明示 |
| D09 **11/04中間判断** | hosted CustomDomainの実装到達点、v2 mount／DNS・TLS切替とroute退役の証拠 | Host。ローンチ必須外は決定済み。未完なら未対応として公開し、native SaaSとmanaged aliasを混同しない |
| D10・D12 **11/04中間判断／昇格前** | 必須Container／Actor WebSocket upgradeの到達点、完成までの残作業・見積り、ActorUpgradeと標準Responseの分離 | Forms＋Host＋CTO。未完はローンチ未充足。0.xで完成させ、個別Formの昇格前にCTO確認 |
| D02 **10/21レビュー／昇格前** | waitUntilの共通有限上限、WinterTC参照版、主要WPT subset | Forms＋Host。fetch／streams／encoding／URL／abortの比較とWfP経路での証拠 |
| D02／D04／D05等 **Tier 1は10/21** | §9.3のWeb標準衝突・独自拡張に対する処置。承認済み制約から標準globalへの変更を導かない | CTO報告用の別一覧。Forms＋Hostが仕様差と実装差の証拠を出し、衝突項目を未処置のまま標準適合と表示しない |
| D15 **Provider 5.0.0公開前** | 必須14 Form／6 Bindingのmapping・coverage、type名、state／import／intent、公開candidateをCTOへ確認 | Provider owner＋CTO。Form昇格確認と別に公開前確認を記録。旧0.x stateを新Hostで無言再解釈しない |

これらは文書上の判断整理であり、今回CTOやTakoserver担当へメッセージ送信・作業停止指示を出したものではない。Host実装、production deploy、公開releaseのauthorityをこの欄から作らない。

### 9.3 Web標準型との衝突・独自拡張 — CTO報告用の別一覧

**D04／D05／D07と2値fallbackの承認は、標準globalや標準型の挙動を変更する承認ではない。** 下表をCTO報告用に分ける。実際の衝突、標準APIとの差異、Takoform独自APIの制約を区別し、Cloudflareの実装都合を標準の意味として公表しない。Tier 1の未解決項目は10/21の先行レビューで処置案と検証方法を揃え、各Formの昇格前にForms／TakoserverからCTOへ報告する。Actor upgradeはローンチ必須で、11/04の中間判断で完成までの残作業を確認する。昇格前に標準Responseとの境界をCTOへ確認する。

#### 9.3.1 衝突・差異の報告事項

| ID／対象 | 標準との差・衝突条件 | 本案の処置／CTOへ残す論点 | takoserverへの影響・必要な証拠 |
| --- | --- | --- | --- |
| **WS-01 Actor／Response、D12** | 現行のlogical101を返す特殊constructor／Response subclassは、標準Response constructorのstatus制約と衝突する。標準のサーバーupgrade型もない | **通常HTTPは標準Responseを維持し、101拡張を採用しない。** Actor WebSocket upgradeは必須。ActorUpgrade独自結果と能力境界を0.xで完成させ、1.0昇格前にCTO確認。標準Responseだと称さない | ActorResponse shim、constructor／clone置換、socket brokerを新ABIから分離。通常Response constructorが101を拒否し、Response.webSocket等を生やさないことを確認。根拠: [Response初期化](https://fetch.spec.whatwg.org/#initialize-a-response)、[WebSockets](https://websockets.spec.whatwg.org/) |
| **WS-02 service.fetch／fetch、Tier 1** | UID固定配送・redirect非追従はTakoformの配送規則であり、URL authorityへ送るglobal fetchや既定redirect followと同一ではない。RequestInitを受けるだけで全fetch semantics一致とは言えない | **標準Request／Response型は維持し、配送規則の差を独立した仕様差としてCTOへ報告。** RequestInit.redirect／credentials／mode等の扱い、非対応値の拒否か別API名が必要かを10/21までに決める。未決のまま標準fetch完全互換と書かない | brokerのUID認可・URL保持・redirectの3xx／Location返却、signal／body／headerを確認。global fetchの実装をBindingの都合で差し替えない。根拠: [Fetch redirect](https://fetch.spec.whatwg.org/#concept-request-redirect-mode) |
| **WS-03 ObjectBucket stream／Request／Streams、D05** | streamの既知長・Content-Length必須はストレージBindingの入力制約。これをglobal fetch／RequestやReadableStream自体へ広げると標準型の意味を変えてしまう | **D05はBinding内に限定。** headersは標準HeadersInitのまま操作ごとの許可集合を検査し、streamへ独自length propertyを要求しない。標準のRequest／Headersのguardやconstructorを緩和して対応しない。この境界をCTO報告に明記 | ObjectBucket facadeで消費前に長さを検査。未知長拒否時にlock・readゼロを確認し、通常のstream／fetchへ制約が漏れないこと、bodyUsed／locking／backpressure／cancelが標準どおりであることを検証。根拠: [BodyInit](https://fetch.spec.whatwg.org/#bodyinit-unions)、[ReadableStream](https://streams.spec.whatwg.org/#rs-class) |
| **WS-04 D05の例外class／code／status** | TypeError／RangeErrorは標準classだがTAKOFORM_* codeとstatusは独自追加情報。標準Fetchが同じ例外情報やHTTP411を必ず返すという説明は誤り | **標準class・prototypeを保つ独自例外情報として分離。** global fetch／DOMExceptionの意味を変更しない。Host API v2 controlへのHTTP411追加には不同意を維持（§9.2.2）。標準例外への追加propertyの仕様化・transportをCTO報告対象にする | facade／brokerでclass・name・code・statusの往復を定義し、cross-isolateのinstanceofを保証しない。runtime error.statusと管理HTTP responseを混同しない。管理wireの回帰試験 |
| **WS-05 AbortSignal／D02、SQL・storage・service** | client切断やcallee取消のnative検出不足を、abortしない擬似signal・reasonの別名化・呼出元の待機を止めない動作で埋めるなら、採用した標準型／取消契約と衝突する。一方、既受理mutationがcommitされること自体はrollbackを規定しないAbortSignalとの衝突ではない | **実物のAbortSignalとreasonを保持する。** client切断検知／remote通知だけbest-effort、明示abort・Host期限・退役時の自身の取消は弱めない。WfP経路で守れない具体箇所があれば、CTOへ個別報告し未対応を示す。自動retryで効果を重ねない | signal.aborted／reason／throwIfAborted、待機・body取消、結果不明のmutation、OS再起動後の元intentを検証。通信越しのreason表現を決め、標準型を独自errorへ一律置換しない。根拠: [DOM AbortSignal](https://dom.spec.whatwg.org/#interface-abortsignal) |

上記は今回の読取りで特定した設計差・実装リスクであり、現行全runtimeに対する実測一覧ではない。WS-01は衝突のある方式を不採用とした。WS-02は既存案の明示的な配送差であり、標準fetchとの同一視を避けるための契約処置が残る。WS-03〜05は境界を守れば標準型を維持できるが、D04／D05／D07の一括承認だけでは境界越えを正当化できない。

#### 9.3.2 標準型を変えない承認済み制約・独自API

| 項目 | 扱いと境界 |
| --- | --- |
| **D04 型付き書込結果** | ObjectBucketのput／complete→ObjectWriteResult、delete→voidは独自ストレージAPIの戻り値として採用済み。get／head、global fetch、service.fetchは実物Responseを維持する。ObjectWriteResultをResponseの代用品・subclassとして渡さない |
| **D05 分割サイズ** | non-final part等長・finalのみ短くてよいこと、part数・byte上限は共通ストレージprofile。ReadableStreamのchunkサイズまで揃える要求へ拡張しない。streamのContent-Length必須の境界はWS-03へ分離 |
| **D07 Range任意・fallback限定** | assetのRange未対応時は全体200を返せる。対応を広告するなら206／416等のHTTP意味を守り、未対応でAccept-Ranges:bytesを出さない。fallback null／index.htmlはasset探索設定の制約で、Request／Response型の改変ではない。object-bucketのRange必須は維持 |
| **KV TTL最小60秒／SQL safe number** | 永続化APIの値域・単位であり、Date／Number／Blob／Uint8Arrayの標準挙動を変更しない。SQLの64-bit INTEGER全域・bigintは1.0で非対応、黙って丸めない。移行時の残TTL60秒未満を勝手に延長しない |
| **InvocationContext.waitUntil等** | handler ABIと寿命管理はTakoform固有。有限上限は未確定で、Fetch／Streamsの標準機能とは称さない。標準globalにvendorのconstructor・fetch option・Response.webSocketを追加しない |

CTO報告では§9.2.0の承認済み方針と本節の標準差異を別の欄に置き、変更する標準型・独自の動作・対応するexact Form URL・3実行経路の証拠・残る判断を1項目ずつ示す。本節の作成は報告資料の準備であり、今回は外部へのメッセージ送信を行わない。

## 10. 今回の検証範囲

初版で実施したのは指定文書・定義・Git object・関連v2実装の読み取り、標準の一次資料参照、用語検索、件数・参照・提案内整合の確認である。初版ではコード修正・build・runtime test・product gateを実行していない。レビューVと直前のCTO決定を反映した版に、今回の最優先Shouta方針を適用し、ローンチ条件・差分表・決定事項・版別registry・移行手順・標準差異の別一覧を更新した。Git共有用文書には保存先repoのcomplete gateを用い、実行結果とcommit・push先は最終引継ぎに記す。これは新契約の実backend適合を証明する試験ではない。

本文の提案を現行動作と区別し、全17 Form＋7 BindingとContainer＋追加Bindingを表に含めた。付録Aの検索scope・語彙・行番号を併記し、検索で拾えない意味上の持込みは§6.2で補った。参照時点以降の別branchやlive状態の変化は未確認である。

初版の文書検査では、freezeの17種との照合、Containerを含む18 Form／8 Bindingの行、JSON例、ローカル参照先4件、表の列数、code fenceを確認した。初版作成前後で5 repoの既存statusとtracked diffのhashが一致した。最新Shouta方針の反映版では表・API署名・確定事項／不同意／未解決事項の整合を検査する。Container／Actor必須外・0.x追加公開禁止・新Hostの0.x互換保持という旧決定が現行方針として残らないこと、指定先とGit共有用コピーの一致を確認する。参照はportableなrepo:pathのまま保持する。今回も既存dirty work、product code、規範spec、freeze、packageは変更しない。本番deploy・公開releaseは実施しない。

## 付録A. 用語の全検索ヒット位置

以下は固定したGit sourceを走査した位置一覧。`repo@SHA:path:line` として解釈する。行番号は古いcheckoutの実ファイルと一致しない場合がある。

検索scopeはTのspec、Fのspec／Form・Interface・Binding候補定義／関連proposal、Pのspec・docs・edgev2・provider source・root候補定義・provider内publisher definitionコピー、Sの全非生成src。tests、website生成物、immutable release／vendorコピー全体は重複出力しない。旧コピーの存在は§2の読取りと保持方針で扱った。主要scopeは892 filesで、268 files／2552行に検索ヒットがある。Containerの未公開FC、K／R／task、Qの関連sourceはA.5に補足する。runtime全一般名詞の自動判定ではない。

検索語彙は製品・native型・設定名に加え、固有shapeを見つけるための語を含む。Cloudflare／cloudflare、workers.dev、Workers for Platforms、WfP／WFP、workerd、Wrangler／wrangler、Miniflare、D1系、R2系、KVNamespace系、DurableObject系、Durable Objects、WorkflowEntrypoint、WebSocketPair、compatibility date／flags各表記、runWorkerFirst／run_worker_first、notFoundHandling／not_found_handling、single_page_application／single-page-application、idFromName、newUniqueId、getWithMetadata、expirationTtlSeconds、listComplete／list_complete、uploadedAtMillis、contentLength、scheduledTime、waitUntil、cf、CfProperties系、HTMLRewriter、FixedLengthStream、IdentityTransformStream、httpMetadata、httpEtag、onlyIf、etagMatches、etagDoesNotMatch、delimitedPrefixes、accountId、zoneId、scriptName、CLOUDFLARE_。fetchやHeadersのような標準語は単独の検索語にしていない。

同じ行の複数ヒットを統合し、fileごとに全該当行を記載する。`contentLength`、`scheduledTime`、`waitUntil`等のヒットは由来を検討する手掛かりで、Cloudflare専用との断定ではない。

### A.1 takoform

基準: `d9daf8e7ed22bd335484d7553de9abbb802df790`。scope内 140 files、11 files／74行にヒット。

| File | 全該当行 | 検出語彙（重複除去） |
| --- | --- | --- |
| `spec/decisions/0007-current-forms-exclude-substrate-operation.md` | 15, 34, 60, 66 | `Cloudflare` |
| `spec/decisions/0019-the-module-worker-abi-is-an-exact-contract.md` | 12, 22–23, 32–33, 67, 76, 148, 177, 182–183, 234, 249, 291 | `cloudflare`, `compatibility date`, `compatibilityDate`, `compatibilityFlags`, `compatibility_date`, `compatibility_flags`, `scheduledTime`, `waitUntil`, `wrangler` |
| `spec/decisions/0020-the-edge-interfaces-state-their-data-and-delivery-model.md` | 65, 126, 137, 145, 153, 165, 169–171, 177, 216, 230, 340, 374, 381, 383, 395 | `compatibilityDate`, `contentLength` |
| `spec/decisions/0023-the-runtime-abi-is-measured-separately-from-the-control-plane.md` | 18, 132, 138, 193, 245, 282, 294 | `compatibility date`, `compatibilityDate`, `waitUntil` |
| `spec/decisions/0033-edge-app-assets-and-sqlite-migrations-are-content-addressed.md` | 44–45, 52, 54, 57 | `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| `spec/decisions/0043-forms-target-popular-vendor-locked-primitives.md` | 61, 83 | `Durable Objects`, `R2` |
| `spec/host-api/v1beta1.md` | 1241–1242, 1247–1248, 1250, 1252, 1256, 1268, 1367–1368, 1397–1398, 1423, 1506 | `compatibilityDate`, `compatibilityFlags`, `notFoundHandling`, `runWorkerFirst`, `single_page_application`, `waitUntil` |
| `spec/host-api/v1beta4.md` | 1389, 1409–1410, 1439–1440, 1465 | `compatibility date`, `compatibility flag`, `compatibilityDate`, `compatibilityFlags`, `notFoundHandling`, `waitUntil` |
| `spec/portability-boundary.md` | 32 | `compatibility date` |
| `spec/schemas/host-support-profile-v1.schema.json` | 64, 93 | `notFoundHandling` |
| `spec/schemas/host-support-profile-v1alpha2.schema.json` | 64, 93 | `notFoundHandling` |

### A.2 takoform-forms

基準: `8b976cb617f84698bc3c79cbcf705c1a330a2316`。scope内 62 files、19 files／128行にヒット。

| File | 全該当行 | 検出語彙（重複除去） |
| --- | --- | --- |
| `bindings/candidates/v1alpha2/module-worker.actor/definition.json` | 7, 24–25 | `idFromName`, `newUniqueId` |
| `bindings/candidates/v1alpha2/module-worker.edge-kv/definition.json` | 7, 25 | `getWithMetadata` |
| `bindings/candidates/v1alpha2/module-worker.object-bucket/definition.json` | 7 | `contentLength`, `uploadedAtMillis` |
| `docs/proposals/actor-execution-contract.md` | 248, 250, 604, 609, 611 | `WfP`, `cloudflare`, `waitUntil` |
| `docs/proposals/actor-runtime.md` | 37, 49, 69, 75, 100, 149, 158, 226, 233, 235, 239, 272, 281, 285, 290–291, 293, 296, 299, 303 | `Cloudflare`, `DurableObjectState`, `WfP`, `cloudflare`, `idFromName`, `newUniqueId`, `workerd` |
| `docs/proposals/actor-workflow-source-candidate.md` | 61 | `Workers for Platforms` |
| `forms/candidates/edge.forms.takoform.com/worker-version/definition.json` | 6, 191, 195, 249, 285–286, 289, 293, 300–301 | `compatibility date`, `compatibility flag`, `idFromName`, `newUniqueId`, `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| `interfaces/candidates/v1alpha1/edge.kv/definition.json` | 65, 138, 233, 282, 288, 356 | `expirationTtlSeconds`, `getWithMetadata`, `listComplete` |
| `interfaces/candidates/v1alpha1/edge.objects/definition.json` | 7, 11, 46, 175, 183, 212, 333, 419, 427, 450, 620, 643, 675, 698, 706, 721, 747, 755 | `contentLength`, `uploadedAtMillis` |
| `interfaces/candidates/v1alpha1/worker.actor/definition.json` | 7, 10, 48, 85, 130, 144, 172, 706, 715, 729, 741, 760 | `contentLength`, `idFromName`, `newUniqueId`, `waitUntil` |
| `interfaces/candidates/v1alpha1/worker.runtime/definition.json` | 7, 324, 334, 341, 445–446, 742 | `scheduledTime`, `waitUntil` |
| `interfaces/candidates/v1alpha1/worker.service/definition.json` | 7, 11, 19, 59, 72, 100, 132, 150, 168 | `contentLength` |
| `spec/forms/ActorNamespace/0.3.0/index.md` | 111, 113, 118, 138 | `idFromName`, `newUniqueId`, `waitUntil` |
| `spec/forms/EdgeKVNamespace/0.2.0/index.md` | 69, 72, 76, 78, 80, 92, 95, 98, 101 | `expirationTtlSeconds`, `getWithMetadata`, `listComplete` |
| `spec/forms/ModuleWorker/0.3.0/index.md` | 79, 90 | `scheduledTime`, `waitUntil` |
| `spec/forms/ObjectBucket/0.2.0/index.md` | 65, 75, 81, 97, 99, 101, 108 | `contentLength`, `uploadedAtMillis` |
| `spec/forms/WorkerCronTrigger/0.3.0/index.md` | 38 | `scheduledTime`, `waitUntil` |
| `spec/forms/WorkerVersion/0.5.0/index.md` | 30, 33, 59–60, 112, 117, 143, 147, 151 | `getWithMetadata`, `idFromName`, `newUniqueId`, `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| `spec/guides/use-host.md` | 116–117 | `notFoundHandling`, `runWorkerFirst` |

### A.3 terraform-provider-takoform

基準: `97ba18102b550d48319a71575568e36ebeeca7bb`。scope内 234 files、40 files／239行にヒット。

| File | 全該当行 | 検出語彙（重複除去） |
| --- | --- | --- |
| `bindings/candidates/v1alpha2/module-worker.actor/definition.json` | 7, 24–25 | `idFromName`, `newUniqueId` |
| `bindings/candidates/v1alpha2/module-worker.edge-kv/definition.json` | 7, 25 | `getWithMetadata` |
| `docs/getting-started.md` | 10 | `Cloudflare` |
| `docs/index.md` | 157 | `Cloudflare` |
| `docs/ja/getting-started.md` | 10 | `Cloudflare` |
| `docs/resources/module_worker.md` | 10, 110 | `compatibility date`, `waitUntil` |
| `docs/resources/worker_version.md` | 10, 39, 48 | `compatibility date`, `compatibility flag`, `idFromName`, `newUniqueId`, `not_found_handling`, `runWorkerFirst`, `run_worker_first` |
| `forms/candidates/edge.forms.takoform.com/module-worker/definition.json` | 6, 12 | `compatibility date`, `waitUntil` |
| `forms/candidates/edge.forms.takoform.com/worker-version/definition.json` | 6, 191, 195, 249, 285–286, 289, 293, 300–301 | `compatibility date`, `compatibility flag`, `idFromName`, `newUniqueId`, `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| `forms/candidates/edge/v1beta1/module-worker/definition.json` | 6, 11 | `compatibility date`, `waitUntil` |
| `forms/candidates/edge/v1beta1/worker-version/definition.json` | 6, 190, 194, 230–231, 234, 238, 245–246 | `compatibility date`, `compatibility flag`, `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| `interfaces/candidates/v1alpha1/edge.kv/definition.json` | 65, 138, 233, 282, 288, 356 | `expirationTtlSeconds`, `getWithMetadata`, `listComplete` |
| `interfaces/candidates/v1alpha1/worker.actor/definition.json` | 7, 10, 48, 85, 130, 144, 172, 706, 715, 729, 741, 760 | `contentLength`, `idFromName`, `newUniqueId` |
| `interfaces/candidates/v1alpha1/worker.runtime/definition.json` | 7, 324, 334, 341, 445–446, 742 | `scheduledTime`, `waitUntil` |
| `interfaces/candidates/v1alpha1/worker.service/definition.json` | 7, 11, 19, 59, 72, 100, 132, 150, 168 | `contentLength` |
| `internal/edgev2/spec.go` | 114–115, 478, 485–486, 488–490, 513–514 | `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| `internal/provider/artifacts/publisher/bindings/module-worker.actor/definition.json` | 7, 24–25 | `idFromName`, `newUniqueId` |
| `internal/provider/artifacts/publisher/bindings/module-worker.edge-kv/definition.json` | 7, 25 | `getWithMetadata` |
| `internal/provider/artifacts/publisher/bindings/module-worker.object-bucket/definition.json` | 7 | `contentLength`, `uploadedAtMillis` |
| `internal/provider/artifacts/publisher/interfaces/edge.kv/definition.json` | 65, 138, 233, 282, 288, 356 | `expirationTtlSeconds`, `getWithMetadata`, `listComplete` |
| `internal/provider/artifacts/publisher/interfaces/edge.objects/definition.json` | 7, 11, 46, 175, 183, 212, 333, 419, 427, 450, 620, 643, 675, 698, 706, 721, 747, 755 | `contentLength`, `uploadedAtMillis` |
| `internal/provider/artifacts/publisher/interfaces/worker.actor/definition.json` | 7, 10, 48, 85, 130, 144, 172, 706, 715, 729, 741, 760 | `contentLength`, `idFromName`, `newUniqueId` |
| `internal/provider/artifacts/publisher/interfaces/worker.runtime/definition.json` | 7, 324, 334, 341, 445–446, 742 | `scheduledTime`, `waitUntil` |
| `internal/provider/artifacts/publisher/interfaces/worker.service/definition.json` | 7, 11, 19, 59, 72, 100, 132, 150, 168 | `contentLength` |
| `internal/provider/artifacts/publisher/packages/edge.forms.takoform.com/module-worker/definition.json` | 6, 12 | `compatibility date`, `waitUntil` |
| `internal/provider/artifacts/publisher/packages/edge.forms.takoform.com/worker-version/definition.json` | 6, 191, 195, 249, 285–286, 289, 293, 300–301 | `compatibility date`, `compatibility flag`, `idFromName`, `newUniqueId`, `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| `internal/provider/provider.go` | 5, 311 | `Cloudflare` |
| `internal/provider/v2_schema.go` | 83 | `not_found_handling`, `run_worker_first` |
| `spec/README.md` | 181 | `Cloudflare` |
| `spec/decisions/0007-current-forms-exclude-substrate-operation.md` | 15, 34, 60, 66 | `Cloudflare` |
| `spec/decisions/0019-the-module-worker-abi-is-an-exact-contract.md` | 12, 22–23, 32–33, 67, 76, 148, 177, 182–183, 234, 249, 291 | `cloudflare`, `compatibility date`, `compatibilityDate`, `compatibilityFlags`, `compatibility_date`, `compatibility_flags`, `scheduledTime`, `waitUntil`, `wrangler` |
| `spec/decisions/0020-the-edge-interfaces-state-their-data-and-delivery-model.md` | 65, 126, 137, 145, 153, 165, 169–171, 177, 216, 230, 340, 374, 381, 383, 395 | `compatibilityDate`, `contentLength` |
| `spec/decisions/0023-the-runtime-abi-is-measured-separately-from-the-control-plane.md` | 18, 132, 138, 193, 245, 282, 294 | `compatibility date`, `compatibilityDate`, `waitUntil` |
| `spec/decisions/0033-edge-app-assets-and-sqlite-migrations-are-content-addressed.md` | 44–45, 52, 54, 57 | `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| `spec/decisions/0043-forms-target-popular-vendor-locked-primitives.md` | 61, 83 | `Durable Objects`, `R2` |
| `spec/form-families.md` | 127, 139–140, 229–230, 235–236, 238, 240, 244, 256 | `compatibility date`, `compatibilityDate`, `compatibilityFlags`, `notFoundHandling`, `runWorkerFirst`, `single_page_application`, `waitUntil` |
| `spec/host-api/v1beta1.md` | 1237–1238, 1243–1244, 1246, 1248, 1252, 1264, 1363–1364, 1393–1394, 1419, 1502 | `compatibilityDate`, `compatibilityFlags`, `notFoundHandling`, `runWorkerFirst`, `single_page_application`, `waitUntil` |
| `spec/host-api/v1beta4.md` | 1385, 1405–1406, 1435–1436, 1461 | `compatibility date`, `compatibility flag`, `compatibilityDate`, `compatibilityFlags`, `notFoundHandling`, `waitUntil` |
| `spec/interface-contract/README.md` | 92, 135, 171 | `contentLength`, `waitUntil` |
| `spec/portability-boundary.md` | 32–33 | `Cloudflare`, `compatibility date`, `workerd` |

### A.4 takoserver

基準: `82917e2058c5eabe1cde94ee170875d3eb9213e5`。scope内 456 files、198 files／2111行にヒット。

| File | 全該当行 | 検出語彙（重複除去） |
| --- | --- | --- |
| `src/actor-addressing.ts` | 35–36, 85, 104, 120 | `idFromName`, `newUniqueId` |
| `src/actor-execution-contract.ts` | 6 | `workerd` |
| `src/actor-namespace-facade.ts` | 9–10, 15–16, 43–44, 49–50, 55–56, 60–61 | `idFromName`, `newUniqueId` |
| `src/actor-native-owner-worker.ts` | 10, 12, 14, 31, 587, 596, 1173, 1639, 1805, 2152, 2253, 2255, 2326, 2339, 2380 | `WebSocketPair`, `WfP`, `idFromName`, `waitUntil` |
| `src/actor-upgrade-handoff.ts` | 122 | `workerd` |
| `src/app.ts` | 110, 166, 203 | `Cloudflare`, `D1`, `R2` |
| `src/artifact-consumer-repair.ts` | 262, 587 | `D1` |
| `src/artifact-recovery-owner-gc.ts` | 73 | `D1`, `R2` |
| `src/artifact-recovery.ts` | 32, 85, 812–813, 815, 819, 823, 869, 2622, 2624 | `R2`, `accountId`, `cloudflare` |
| `src/auth.ts` | 274, 711 | `D1` |
| `src/cloudflare-provider-surface.ts` | 7, 22, 32, 45, 50–51, 53, 55, 58, 73, 77, 80, 82, 84, 100, 152, 157, 175, 179–180, 193, 276, 291, 296 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/cloudflare-runtime-binding-materializer.ts` | 4–5, 7, 9, 11, 17, 19, 23, 26, 41, 44, 46, 51, 57, 73, 77, 81, 87, 93, 95, 104 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/cloudflare-workers-rpc.d.ts` | 1, 7–8, 11 | `Cloudflare`, `DurableObject`, `cloudflare` |
| `src/compat.ts` | 25 | `D1` |
| `src/control.ts` | 1804 | `workerd` |
| `src/database-schema.ts` | 7 | `D1` |
| `src/database.ts` | 8, 10, 12 | `D1` |
| `src/db-schema.ts` | 36, 40, 68, 88, 92, 116, 124, 132, 143–144, 148, 152, 160, 168, 172, 180, 187–188, 192, 200, 235–236, 239–240, 259–260, 263–264, 271–272, 279–280, 283–284, 340, 352, 356, 360, 364 | `Cloudflare`, `D1`, `R2`, `WfP`, `accountId`, `cloudflare`, `scriptName`, `waitUntil` |
| `src/deployment-composition.ts` | 16 | `cloudflare` |
| `src/entry-bun.ts` | 154, 160–163, 179, 206, 208, 291, 296, 310, 320, 349, 356–357, 364, 368, 376–378, 381, 385, 437, 446, 457, 493, 502, 526, 530–534, 545, 554, 556, 563, 578, 598–599, 648, 677, 681, 701, 707, 728–729, 732, 742–743, 773, 779, 794, 796–797, 801, 808–809, 816, 825, 905, 962, 1068, 1070, 1072, 1076, 1098, 1100, 1108, 1130, 1139, 1141–1143, 1297, 1304–1305, 1325, 1345, 1497, 1518, 1551, 1625, 1746 | `CLOUDFLARE_`, `Cloudflare`, `D1`, `R2`, `accountId`, `cloudflare`, `workerd` |
| `src/entry-cloudflare-worker.ts` | 1 | `cloudflare` |
| `src/entry-exact-artifact-recovery-worker.ts` | 1 | `cloudflare` |
| `src/entry-existing-space-operator-worker.ts` | 1 | `cloudflare` |
| `src/entry-form-authority-worker.ts` | 1–2 | `cloudflare` |
| `src/entry-integration-form-authority-operator-worker.ts` | 1 | `cloudflare` |
| `src/entry-integration-form-authority-worker.ts` | 1 | `cloudflare` |
| `src/entry-money-shadow-native-probe.ts` | 8, 10, 12, 126 | `D1`, `D1DatabaseLike` |
| `src/entry-public-form-runtime-payload.ts` | 6 | `Wrangler` |
| `src/entry-sponsorship-authority-worker.ts` | 1, 14, 37 | `D1`, `D1Database`, `cloudflare` |
| `src/entry-stable-local-cloudflare-host.ts` | 6, 18, 63, 67, 69, 84–85, 88, 93, 113–114, 118–119, 125–126, 133, 184, 192, 195, 200, 203, 209, 305, 382, 578–579, 583–584, 594, 653, 735, 760, 1208, 1229, 1237, 1360–1361, 1368–1372, 1380–1381, 1434 | `Cloudflare`, `Miniflare`, `accountId`, `cloudflare`, `compatibilityDate`, `notFoundHandling`, `not_found_handling`, `runWorkerFirst`, `run_worker_first`, `scriptName`, `single-page-application`, `workerd` |
| `src/entry-worker.ts` | 31–32, 60, 64, 70, 135, 140, 516, 716, 743, 749, 885, 934, 1101, 1129, 1141 | `CLOUDFLARE_`, `Cloudflare`, `D1`, `cloudflare`, `waitUntil` |
| `src/exact-artifact-recovery-worker.ts` | 20, 22, 39–40, 255, 259 | `D1DatabaseLike`, `R2BucketLike` |
| `src/existing-space-operator-proof.ts` | 31 | `workers.dev` |
| `src/form-authority-worker-composition.ts` | 14, 16, 36–37, 42 | `D1`, `D1DatabaseLike`, `R2`, `R2BucketLike` |
| `src/hosted-edge-supplies.ts` | 10, 33, 58, 60, 86 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/hosted-object-bucket-supplies.ts` | 10, 76, 108, 110, 276 | `cloudflare` |
| `src/hosted-pricing.ts` | 294, 305 | `cloudflare` |
| `src/index.ts` | 144–148 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/integration-e2e-credential-authority.ts` | 521 | `D1` |
| `src/integration-form-authority-gateway.ts` | 401 | `workers.dev` |
| `src/ledger.ts` | 645 | `D1` |
| `src/money-cutover-sql.ts` | 3, 161 | `D1` |
| `src/object-bucket-deployment.ts` | 13 | `cloudflare` |
| `src/objects-fs.ts` | 17 | `R2` |
| `src/objects-mem.ts` | 18 | `R2` |
| `src/objects-r2-http.ts` | 9, 11, 14, 19, 27–28, 35–37, 62, 77, 88, 103, 105, 117, 126, 128, 136, 150, 173, 176, 179, 183, 186, 188, 194, 197, 207, 210, 224, 269, 275, 297, 301 | `R2`, `R2HttpOptions`, `R2_LIST_MAX_BYTES`, `accountId`, `cloudflare`, `contentLength` |
| `src/objects-r2.ts` | 10, 14–15, 17, 21, 25, 29–30, 35, 40, 42, 44–46, 52–53, 58–59, 68, 78, 82, 91, 101, 117, 131, 143–144, 147 | `Cloudflare`, `R2`, `R2BucketLike`, `R2ListedLike`, `R2ObjectBodyLike`, `R2ObjectLike`, `cloudflare`, `etagDoesNotMatch`, `httpMetadata`, `onlyIf` |
| `src/org-billing.ts` | 238 | `D1` |
| `src/org-usage-summary.ts` | 49 | `D1` |
| `src/ports.ts` | 6, 51 | `Cloudflare`, `D1` |
| `src/provider-driver.ts` | 329 | `Cloudflare` |
| `src/provider-extension.ts` | 5–6, 31, 108–112, 114–117, 122, 126, 128–153, 155–162, 164–167, 169–174, 176–177, 179, 182–192 | `CLOUDFLARE_`, `Cloudflare`, `WfP`, `cloudflare` |
| `src/provider-port.ts` | 155 | `workers.dev` |
| `src/provider-runtime-bindings.ts` | 34 | `Cloudflare` |
| `src/provider-worker-endpoint-origin.ts` | 24, 179 | `Cloudflare`, `workerd` |
| `src/providers/cloudflare-actor-meter-source.ts` | 4, 22, 25, 39–40, 46, 51, 60, 66, 269, 332 | `Cloudflare`, `accountId`, `cloudflare` |
| `src/providers/cloudflare-edge-meter-contract.ts` | 3, 7, 18–20, 22–24, 26, 29, 36, 38–39, 45–46, 53–54, 61–62, 68, 75, 78–79, 81, 84, 86, 88, 90, 92 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/providers/cloudflare-edge-meter.ts` | 14–16, 19–21, 23, 27, 29, 33–34, 38, 49, 68, 71, 75, 82, 87, 102, 104, 111, 115, 131–132, 134, 141, 156–157, 165, 211, 216, 218, 229 | `CLOUDFLARE_`, `Cloudflare`, `D1_QUERY`, `accountId`, `cloudflare`, `scriptName` |
| `src/providers/cloudflare-provider-executor-codec.ts` | 22 | `Cloudflare` |
| `src/providers/cloudflare-provider-executor-port.ts` | 24, 28–29, 32–33, 38–39, 42–43, 51, 56, 59–60, 63–64, 72, 77, 80–81, 84–85, 92, 97, 100–101, 104–105, 113–114, 121–122, 125–126, 134, 139, 143, 147, 150–151, 154–155, 163–164, 172, 177, 181, 184–186, 189, 192–193, 196–197, 200–201, 208, 216, 226, 234, 237, 239, 242–243, 245, 248–249, 252, 255–261, 263–264, 266, 269, 272, 275, 278–279 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/providers/cloudflare-provider-proxy.ts` | 42–44, 47, 49, 51–59, 61–63, 66, 77, 85, 87, 98, 101–102, 150, 185, 240, 393, 395, 412, 468, 471, 473, 478, 483, 502, 526, 530, 638, 732, 784, 857, 899, 957, 1002, 1055 | `CLOUDFLARE_`, `Cloudflare`, `D1`, `R2`, `WfP`, `cloudflare`, `workers.dev` |
| `src/providers/cloudflare-r2-meter.ts` | 2, 15, 17–18, 22, 26, 39, 80, 98 | `CLOUDFLARE_`, `Cloudflare`, `R2`, `accountId`, `cloudflare` |
| `src/providers/cloudflare-readback-descriptor.ts` | 12, 14, 25, 36, 39, 43, 58, 61, 68, 73, 82–84, 86, 88, 103, 105, 108, 113, 130–131, 146, 148, 151, 157, 171, 185, 188, 196, 223, 260, 267, 269, 298, 311, 313, 315, 317, 322, 330, 343, 345, 347, 349, 354–355 | `Cloudflare`, `WfP`, `cloudflare`, `scriptName`, `workers.dev` |
| `src/providers/cloudflare-runtime-bindings.ts` | 10–11, 13–14, 18, 20, 30, 33 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/providers/cloudflare-worker-backend.ts` | 25, 27, 29, 31, 36, 45, 72, 89–91, 115, 128, 141, 154, 159, 161–162, 167, 170–171, 182–183, 208, 210–213, 220–221, 223, 245, 248–249, 253, 284, 286, 294, 298, 301–302 | `Cloudflare`, `Workers for Platforms`, `accountId`, `cloudflare`, `workers.dev` |
| `src/providers/cloudflare-workers-ai.ts` | 5, 14, 18, 32–33, 41, 44, 46, 50 | `Cloudflare`, `cf` |
| `src/providers/cloudflare.ts` | 47–49, 51, 53, 56–67, 72–83, 85, 88, 92, 96, 100, 102, 105, 119, 121, 135, 171, 174, 178, 192, 194, 220, 222–223, 231–232, 255, 266, 269–270, 273, 278–280, 318, 335, 346, 385, 397–398, 412, 467, 478, 486, 527, 593, 595, 604, 644, 658, 696, 701, 704, 711, 722, 730, 742, 752, 765, 783, 806, 852, 965–966, 974, 980, 984, 990, 996, 1006, 1015, 1022, 1031, 1039, 1048, 1055, 1065, 1086, 1088, 1090, 1092–1093, 1130, 1140–1141, 1156, 1195, 1231, 1235, 1239, 1259, 1261, 1263, 1265, 1267, 1269, 1271, 1273–1274, 1305, 1316, 1335, 1417–1418, 1483, 1494, 1505, 1516, 1531, 1539, 1551–1552, 1580, 1608, 1612, 1719, 1805, 1889, 1902, 1909–1910, 1922, 1962, 1974, 1995, 2016, 2042, 2103, 2145, 2147, 2160–2162, 2212, 2237, 2273–2275, 2277, 2310, 2326, 2328, 2341–2343, 2348, 2358, 2546, 2555, 2570, 2594, 2605, 2612, 2618, 2646, 2657, 2676, 2681, 2717, 2760, 2771, 2798–2799, 2823–2824, 2850, 2883, 2903, 2914, 2919, 2938, 2951, 2989, 3041–3043, 3057, 3060, 3073, 3080, 3102, 3151, 3245–3246, 3262, 3310, 3320, 3339, 3394, 3404, 3410, 3413, 3528, 3555–3556, 3583, 3587, 3650, 3658, 3691, 3698, 3744, 3756, 3760–3762, 3782, 3807, 3820, 3843, 3848, 3856, 3964, 3972, 4099, 4176, 4184, 4201, 4213, 4226, 4240, 4334, 4340, 4342, 4364, 4369, 4596, 4678, 4680, 4688–4690, 4692 | `CLOUDFLARE_`, `Cloudflare`, `D1`, `DurableObjectDeclaration`, `R2`, `WfP`, `Workers for Platforms`, `accountId`, `cloudflare`, `compatibilityDate`, `compatibilityFlags`, `compatibility_date`, `compatibility_flags`, `notFoundHandling`, `not_found_handling`, `runWorkerFirst`, `run_worker_first`, `scriptName`, `single-page-application`, `single_page_application`, `workers.dev`, `zoneId` |
| `src/providers/edge-objects-adapter.ts` | 37, 56, 63, 74, 85, 87, 113, 120–121, 132, 138, 149, 206, 209, 245, 261, 285, 300, 423, 476, 669, 671–672, 674, 689, 707, 717, 722, 724, 738, 747, 846–847, 854, 867, 869–870, 939–941, 951–953, 1020 | `R2`, `contentLength`, `delimitedPrefixes`, `etagDoesNotMatch`, `etagMatches`, `httpEtag`, `httpMetadata`, `onlyIf`, `uploadedAtMillis` |
| `src/providers/http-revision-serving.ts` | 450, 973 | `waitUntil` |
| `src/providers/remote.ts` | 13 | `Cloudflare` |
| `src/providers/selfhost-data-service.ts` | 20, 23, 58, 125 | `cloudflare`, `workerd` |
| `src/providers/selfhost-events.ts` | 11, 17, 22, 26, 34, 236, 310, 318–319, 329 | `Cloudflare`, `scheduledTime`, `workerd` |
| `src/providers/selfhost-extension.ts` | 5 | `workerd` |
| `src/providers/selfhost-runtime-bindings.ts` | 2 | `cloudflare` |
| `src/providers/selfhost-v2-kv-binding-broker.ts` | 300–301, 325–326, 342–343 | `expirationTtlSeconds`, `getWithMetadata` |
| `src/providers/selfhost-v2-kv-store.ts` | 40, 55, 82, 91, 251, 388, 400, 402, 424, 426, 448, 458, 674–675, 677–678, 680, 684 | `expirationTtlSeconds`, `getWithMetadata`, `listComplete` |
| `src/providers/selfhost-v2-object-bucket-binding-broker.ts` | 30, 34, 92, 402, 439 | `contentLength`, `workerd` |
| `src/providers/selfhost-v2-object-bucket-data-service.ts` | 15 | `workerd` |
| `src/providers/selfhost-v2-object-bucket-store.ts` | 65, 489 | `contentLength` |
| `src/providers/selfhost-v2-queue-transport.ts` | 16 | `workerd` |
| `src/providers/selfhost-v2-sqlite-binding-broker.ts` | 46, 54, 94, 99 | `D1`, `WfP`, `workerd` |
| `src/providers/selfhost-v2-sqlite-queue-binding-broker.ts` | 149, 373, 375 | `D1` |
| `src/providers/selfhost-v2-sqlite-worker-projection.ts` | 7 | `workerd` |
| `src/providers/selfhost-version-bindings.ts` | 303, 1808 | `workerd` |
| `src/providers/selfhost-version-materialization.ts` | 77–78, 97, 99, 440–442, 471, 488–489, 1031, 1036, 1041, 1046–1048, 1057–1058 | `notFoundHandling`, `runWorkerFirst`, `single-page-application` |
| `src/providers/selfhost-worker-wrapper.ts` | 39, 44, 48, 54, 60, 67, 71–72, 80, 94, 112, 115, 117, 165, 313, 332, 353, 691, 1028, 1037, 1073–1074, 1147, 1149, 1284, 1289, 1369–1371, 1375, 1451, 1582, 1589, 1634, 1636, 1678–1680, 1725, 1729, 1937–1939, 1980, 1987–1988, 2000, 2002–2003, 2488, 2490, 2509–2510, 2538, 2551, 2556, 2859, 2865–2867, 2870–2872, 2876 | `Cloudflare`, `R2`, `cloudflare`, `contentLength`, `expirationTtlSeconds`, `getWithMetadata`, `listComplete`, `scheduledTime`, `uploadedAtMillis`, `waitUntil`, `workerd` |
| `src/providers/selfhost.ts` | 94–95, 187, 205, 215, 220, 314, 472, 482, 973, 1191, 1282, 1497, 1624–1625, 1686, 1757, 2049, 2540, 2571, 2619, 2683, 2855–2857, 2873–2875, 2877, 2908–2909, 3280, 3298, 3553, 3570, 3710, 3756, 3805, 3845, 3974, 4115, 4149, 4289, 4583, 4596, 5106, 5237–5238, 5285–5286, 5336, 5366, 5373, 5410, 5440, 5473, 5514, 5553, 6030, 6130, 6753, 6759, 6766, 6771, 6778, 6787, 6802, 6812, 6981–6982 | `Cloudflare`, `R2`, `notFoundHandling`, `runWorkerFirst`, `scriptName`, `single-page-application`, `single_page_application`, `workerd` |
| `src/providers/sqlite-migration-policy.ts` | 60, 64, 67, 70, 288 | `Cloudflare`, `cloudflare`, `workerd` |
| `src/providers/worker-module-semantic-inspection.ts` | 161, 172, 333 | `workerd` |
| `src/providers/workerd-v2-private-binding-names.ts` | 52, 66, 74, 88 | `workerd` |
| `src/public-form-implementation-build.ts` | 9, 11 | `Cloudflare`, `Wrangler` |
| `src/public-form-runtime.ts` | 6, 16–17, 25 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/public-worker-implementation.ts` | 11–12, 111, 114, 141 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/queue-custody.ts` | 125, 133, 293, 295, 347, 534, 644, 822, 2694 | `D1`, `WfP`, `waitUntil`, `workerd` |
| `src/queue-v2-sqlite-contract.ts` | 50, 52 | `D1` |
| `src/request-lifetime.ts` | 3 | `waitUntil` |
| `src/resource-deployments.ts` | 47 | `D1` |
| `src/router.ts` | 18 | `workerd` |
| `src/selfhost-actor-execution-host.ts` | 34, 134, 897 | `workerd` |
| `src/selfhost-actor-forward-brokers.ts` | 7, 20 | `workerd` |
| `src/selfhost-actor-forward-worker-wrapper.ts` | 3 | `workerd` |
| `src/selfhost-actor-graph-authority.ts` | 13, 92 | `scriptName`, `workerd` |
| `src/selfhost-actor-lease.ts` | 10 | `workerd` |
| `src/selfhost-actor-native-process.ts` | 15, 22, 27, 76, 93, 742 | `workerd` |
| `src/selfhost-actor-public-runtime.ts` | 29–31, 180–181 | `scriptName`, `workerd` |
| `src/selfhost-composition.ts` | 53, 61, 78, 200, 202, 749 | `Cloudflare`, `workerd` |
| `src/selfhost-container-bootstrap.ts` | 49 | `workerd` |
| `src/selfhost-container-endpoint-https.ts` | 100, 110 | `workerd` |
| `src/selfhost-data-planes.ts` | 43–44, 64, 517, 568, 692, 861, 886, 897, 948, 952, 954, 968, 1200, 1660, 1664, 1714, 1760 | `Cloudflare`, `D1`, `contentLength`, `expirationTtlSeconds`, `getWithMetadata`, `listComplete`, `workerd` |
| `src/selfhost-entry-shutdown.ts` | 71, 78, 97, 99 | `workerd` |
| `src/selfhost-health.ts` | 6, 205 | `workerd` |
| `src/selfhost-object-store.ts` | 11, 38, 72, 136, 155, 173, 212, 342, 345, 348, 371, 378, 409, 422, 426, 434, 547, 594, 665, 1019, 1084 | `Cloudflare`, `R2`, `contentLength`, `uploadedAtMillis` |
| `src/selfhost-queue-pump.ts` | 26, 31–32, 61 | `Cloudflare`, `workerd` |
| `src/selfhost-runtime-binding-materializer.ts` | 60 | `Cloudflare` |
| `src/selfhost-scheduler.ts` | 18, 23, 124, 131, 200, 214 | `scheduledTime`, `workerd` |
| `src/selfhost-socket-layout.ts` | 163, 171 | `workerd` |
| `src/selfhost-v2-queue-composition.ts` | 23 | `workerd` |
| `src/selfhost-v2-queue-worker-capability.ts` | 8 | `workerd` |
| `src/selfhost-v2-runtime-boot.ts` | 18, 138, 149, 204, 213 | `waitUntil`, `workerd` |
| `src/selfhost-v2-scheduled-composition.ts` | 7 | `workerd` |
| `src/selfhost-v2-worker-composition.ts` | 82–84, 89, 139, 175, 528, 541, 571, 580, 990, 1345 | `workerd` |
| `src/selfhost-v2-worker-endpoint-frontend.ts` | 18–19 | `workerd` |
| `src/selfhost-v2-worker-endpoint-https.ts` | 13 | `workerd` |
| `src/selfhost-v2-worker-endpoint-upstream-websocket.ts` | 6 | `workerd` |
| `src/selfhost-v2-workflow-boot.ts` | 21, 23, 35, 37, 54, 67, 69 | `waitUntil`, `workerd` |
| `src/selfhost-workflow-binding-worker-wrapper.ts` | 2 | `workerd` |
| `src/selfhost-workflow-execution-host.ts` | 9, 91, 95, 117, 120, 133 | `workerd` |
| `src/selfhost-workflow-http-transport.ts` | 3 | `workerd` |
| `src/selfhost-workflow-preparation.ts` | 7–8 | `workerd` |
| `src/selfhost-workflow-private-owner.ts` | 13, 67, 79, 109, 112, 211, 306, 315 | `scriptName`, `waitUntil`, `workerd` |
| `src/selfhost-workflow-serving.ts` | 27, 261, 291 | `scriptName`, `workerd` |
| `src/shared-hosted-pricing.ts` | 14, 175, 177–179, 183, 185–186, 221 | `cloudflare` |
| `src/sql-d1-http.ts` | 4, 7, 11, 20–21, 29–31, 49, 60, 91 | `Cloudflare`, `D1`, `D1HttpOptions`, `accountId`, `cloudflare` |
| `src/sql-d1.ts` | 11, 15–17, 20, 25–27, 31, 35, 37, 40, 47, 65, 72, 79 | `D1`, `D1DatabaseLike`, `D1ResultLike`, `D1StatementLike` |
| `src/sql-sqlite.ts` | 15–16 | `D1` |
| `src/standalone-provider-composition.ts` | 9, 27, 29, 33, 37, 39, 43, 48, 55–56, 68, 70, 78, 80, 83, 85, 88, 90, 95, 101, 104, 107, 247, 251, 253, 267, 270, 272, 321–322, 326, 330, 333, 342, 350–351, 355 | `CLOUDFLARE_`, `Cloudflare`, `D1`, `R2`, `cloudflare`, `workerd` |
| `src/stripe-billing-client.ts` | 136, 138–140 | `contentLength` |
| `src/takoform-v2/actor-forward-runtime.ts` | 14, 36, 41, 89, 158, 187 | `scriptName`, `workerd` |
| `src/takoform-v2/actor-namespace-admission.ts` | 74 | `D1` |
| `src/takoform-v2/actor-namespace-graph-authority.ts` | 10 | `workerd` |
| `src/takoform-v2/forms/artifact-custody.ts` | 10, 14, 191, 1024 | `D1` |
| `src/takoform-v2/forms/worker-specs.ts` | 64–65, 192, 198, 340, 342–343, 349–350 | `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| `src/takoform-v2/selfhost-v2-workflow-composition.ts` | 4, 26, 33, 66, 74 | `waitUntil`, `workerd` |
| `src/takoform-v2/service-binding-authority.ts` | 14, 16 | `D1`, `WfP` |
| `src/takoform-v2/store.ts` | 457 | `D1` |
| `src/takoform-v2/worker-bundle-runtime.ts` | 2, 52, 68, 73 | `workerd` |
| `src/takoform-v2/worker-code-eligibility.ts` | 421 | `notFoundHandling`, `single_page_application` |
| `src/takoform-v2/worker-code-runtime.ts` | 11, 255–257, 259 | `notFoundHandling`, `runWorkerFirst`, `single-page-application`, `single_page_application`, `workerd` |
| `src/takoform-v2/worker-cron-trigger-scheduler.ts` | 27, 48, 108, 114, 119, 124, 163–164, 169, 206, 219, 237, 244, 246, 295, 367, 378–379, 390, 399, 404 | `scheduledTime` |
| `src/takoform-v2/worker-deployment-backend.ts` | 2–3 | `workerd` |
| `src/takoform-v2/worker-endpoint-backend.ts` | 2 | `workerd` |
| `src/takoform-v2/worker-invocation-custody.ts` | 33, 90, 120, 216, 240, 655, 668, 681, 721, 762, 1068 | `D1`, `scriptName`, `waitUntil` |
| `src/takoform-v2/worker-lifecycle-backend.ts` | 5 | `workerd` |
| `src/takoform-v2/worker-native-effects.ts` | 46, 48 | `D1` |
| `src/takoform-v2/worker-publication-sql-guard.ts` | 222 | `D1` |
| `src/takoform-v2/worker-publication-state.ts` | 560, 604 | `D1` |
| `src/takoform-v2/worker-queue-delivery.ts` | 948 | `D1` |
| `src/takoform-v2/worker-queue-scheduler.ts` | 54, 57 | `waitUntil` |
| `src/takoform-v2/worker-runtime-readers.ts` | 1, 124 | `workerd` |
| `src/takoform-v2/worker-static-publication.ts` | 18–19, 22–23, 85, 117, 319, 701 | `scriptName`, `waitUntil`, `workerd` |
| `src/takoform-v2/worker-static-runtime.ts` | 1, 63, 74, 88–90, 92 | `notFoundHandling`, `runWorkerFirst`, `single-page-application`, `single_page_application`, `workerd` |
| `src/takoform-v2/workflow-binding-boot.ts` | 13, 49, 82, 153, 175 | `scriptName`, `workerd` |
| `src/takoform-v2/workflow-class-admission.ts` | 76 | `D1` |
| `src/takoform-v2/workflow-forward-runtime.ts` | 3 | `workerd` |
| `src/takoform-v2/workflow-native-selection.ts` | 8–9 | `workerd` |
| `src/takoform-v2/workflow-selected-materials.ts` | 192 | `WfP` |
| `src/takoform/artifact-blob-io.ts` | 37–38, 152, 217 | `D1`, `R2` |
| `src/takoform/engine.ts` | 123 | `R2` |
| `src/takoform/exact-artifact-recovery-coordinator.ts` | 190 | `R2` |
| `src/takoform/form-authority-verification.ts` | 129, 220 | `idFromName` |
| `src/takoform/host-admission-coordinator.ts` | 262, 364, 633, 692 | `D1`, `R2` |
| `src/takoform/host-admission-endpoint.ts` | 313 | `D1`, `R2` |
| `src/takoform/host-authority.ts` | 88, 225 | `D1`, `R2` |
| `src/takoform/integration-operator-endpoint.ts` | 16 | `D1`, `R2` |
| `src/takoform/operations.ts` | 686 | `waitUntil` |
| `src/takoform/provider-delete-subeffect.ts` | 11, 55, 150 | `D1` |
| `src/takoform/sqlite-migrations.ts` | 62 | `R2` |
| `src/takoform/store.ts` | 89, 5524, 6789, 6793 | `D1` |
| `src/takoform/wire.ts` | 375 | `Cloudflare` |
| `src/worker-data-services.ts` | 3–5, 9, 30 | `Cloudflare`, `cloudflare` |
| `src/worker-module-inspection-contract.ts` | 57 | `workerd` |
| `src/worker-production-composition.ts` | 1–2, 21, 23–26, 38, 53, 80–81, 88–89, 91, 94, 104, 107, 110, 116, 121, 135, 150, 154, 158, 160, 162, 172, 174, 177, 188, 194, 197, 211, 213, 217, 249, 286 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| `src/worker-stable-local-composition.ts` | 75, 392 | `waitUntil` |
| `src/workerd-artifact.ts` | 16, 69, 149, 153, 205, 210–212, 225, 232, 269, 278, 281 | `cloudflare`, `compatibilityDate`, `workerd` |
| `src/workerd-execution-guard.ts` | 16, 25, 61, 65, 70, 78 | `workerd` |
| `src/workerd-linux-process.ts` | 147, 149, 158, 167, 176, 197, 233 | `workerd` |
| `src/workerd-runtime.ts` | 49–52, 78–79, 100, 103, 123, 127, 193–194, 221, 465, 480, 536, 646, 734, 740, 772, 949, 1151, 1502, 1601, 1625, 2293, 2301, 2425, 2476, 2629, 3091, 3519, 3548–3549, 3567–3568, 3574–3575, 3579–3580, 3586–3587, 3600–3601, 3608–3609, 3614–3615, 3667, 3672–3673, 3681, 3840, 4343, 4367, 4562, 4618, 4626, 4634, 4641–4642, 4669, 4789, 5040, 5597–5598, 5642–5643, 5901–5902, 6033, 6046, 6050, 6174, 6178, 6347, 6383, 6431, 6471, 6497, 6503, 6532, 6550, 6554, 6563, 6566, 6584, 6628, 6642, 6664, 6690, 6719, 6744, 6770, 6800, 6838, 6867, 6924, 6943, 6955, 7334, 7338–7339, 7462, 7466 | `Cloudflare`, `cloudflare`, `compatibilityDate`, `compatibilityFlags`, `notFoundHandling`, `runWorkerFirst`, `single-page-application`, `workerd` |
| `src/workerd-site-contract.ts` | 23, 31–32 | `notFoundHandling`, `runWorkerFirst`, `single-page-application`, `workerd` |
| `src/workerd-supervisor.ts` | 3, 6, 9, 15, 65, 79, 87, 120, 123–124, 196, 219, 238, 294, 304, 319, 341, 344, 361, 364, 367, 384, 397, 402, 410, 422–423, 426, 478, 495, 498, 502, 505, 587, 589, 595, 606, 627, 641, 647, 706 | `cloudflare`, `workerd` |
| `src/workerd-version-graph.ts` | 50–53, 72, 110–111, 206, 280, 743, 746, 789, 813, 904, 919, 935, 961, 963, 1010–1011, 1047–1048 | `notFoundHandling`, `runWorkerFirst`, `single-page-application`, `workerd` |
| `src/workerd-worker-execution-group.ts` | 16–17, 20, 112, 128, 140, 144, 159, 161, 170, 207, 337, 757, 760, 765, 784, 814, 823, 837, 844, 902, 905, 1058, 1079, 1103, 1124, 1126, 1161, 1165, 1234–1235, 1239, 1325, 1327, 1403, 1423, 1501 | `scriptName`, `workerd` |
| `src/workerd-worker-module-inspector.ts` | 26, 29, 52, 569, 575, 579, 585, 770, 777, 787–788, 798, 812, 852, 1144, 1150 | `compatibilityDate`, `compatibilityFlags`, `workerd` |
| `src/workerd-worker-runtime-owner.ts` | 85, 102, 112–116, 126, 198, 486, 621, 698, 718, 1546, 2412, 2562, 2758, 2822, 2828–2829, 2872, 2913, 2928, 2942, 3117, 3138, 3234, 3243, 3264, 3313, 3327, 3360, 3385, 3397, 3511, 3517, 3584–3585, 3605, 3625, 3640, 3663–3664, 3693, 3716, 3740, 3759, 3791, 3800, 3829, 3971, 3993, 3997, 4109, 4119, 4349, 4442, 4451, 4587, 4596, 4608, 4637, 4655, 4915, 4924, 5321, 5331, 5375, 5386, 5405, 5602, 5800, 5809, 5821, 5830, 5945, 5963, 5988, 5997, 6022, 6028, 6040, 6209, 6218, 6243, 6277, 6334, 6472, 6482, 6494, 6497, 6547 | `scheduledTime`, `scriptName`, `waitUntil`, `workerd` |
| `src/workerd-workflow-preparation.ts` | 10, 15, 176 | `workerd` |
| `src/workflow-execution.ts` | 60, 115, 156, 270, 1222 | `WfP`, `waitUntil` |
| `src/workflow-http-bootstrap-entry.ts` | 10, 72 | `workerd` |
| `src/workflow-http-worker.ts` | 52, 174 | `workerd` |
| `src/workflow-runtime-workerd.ts` | 1, 8, 13, 18, 35 | `Cloudflare`, `workerd` |


### A.5 Container候補・関連Host source・指定調査資料

FC／Qの参照SHAは§2.1、C／R／Kは読取り時の指定ファイル。検索語彙は上と同じ。該当なしのファイルも明記する。

| 基準 | File | 全該当行 | 検出語彙 |
| --- | --- | --- | --- |
| FC | `spec/forms/Container/0.1.0/index.md` | 11, 27, 199, 206, 213–215, 221 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
| FC | `spec/forms/WorkerVersion/0.6.0/index.md` | 34, 37, 76–77, 130, 135, 163, 167, 171 | `getWithMetadata`, `idFromName`, `newUniqueId`, `notFoundHandling`, `runWorkerFirst`, `single_page_application` |
| FC | `spec/forms/WorkerDeployment/0.5.0/index.md` | 該当なし | — |
| Q | `src/entry-cloudflare-v2-host.ts` | 40 | `WfP` |
| Q | `src/wfp/v2-edge-kv-key-codec.ts` | 該当なし | — |
| Q | `src/wfp/v2-object-bucket-namespace.ts` | 2, 78 | `Cloudflare`, `cloudflare` |
| Q | `src/wfp/v2-worker-edge-kv-wrapper.ts` | 135, 138, 195, 202, 279, 281, 290, 292, 353–354, 376, 386–387 | `expirationTtlSeconds`, `getWithMetadata`, `listComplete`, `list_complete`, `scriptName`, `waitUntil` |
| Q | `src/wfp/v2-object-bucket-r2-data-executor.ts` | 1, 34, 41–42, 53, 59, 69, 146, 154, 159, 283–284, 334, 340, 343, 346, 348–349, 413, 488, 522, 845, 848–849, 855, 868, 872, 874–875, 878, 935, 938–939, 945, 954, 1005, 1012, 1024 | `FixedLengthStream`, `R2`, `R2Bucket`, `cloudflare`, `contentLength`, `delimitedPrefixes`, `etagDoesNotMatch`, `etagMatches`, `httpMetadata`, `onlyIf`, `uploadedAtMillis` |
| Q | `src/wfp/v2-sqlite-node-authorizer-service.ts` | 3, 31, 89, 91 | `Cloudflare`, `D1` |
| C | `takos-control:tasks/TASK-0090-takoform-v2-specification.md` | 306, 377, 487, 612 | `Cloudflare`, `Wrangler`, `runWorkerFirst` |
| C | `takos-control:tasks/TASK-0098-provider-host-v2-client.md` | 23, 274, 309, 548 | `WfP` |
| R | `operator-runs:takoform-edge-coverage-20261014/REPORT.md` | 3, 9, 24, 60–61, 68, 76, 78, 83–84, 88, 94, 98, 100, 111, 115, 118, 120, 124, 129, 136, 146, 148, 157, 162, 169, 202, 212, 214, 217, 226, 233, 237, 245 | `Cloudflare`, `D1`, `Miniflare`, `R2`, `WfP`, `Wrangler`, `cloudflare`, `getWithMetadata`, `idFromName`, `newUniqueId`, `workerd` |
| K | `operator-runs:takoform-container-20261014/SPEC.md` | 50, 52, 56, 80, 82, 86 | `CLOUDFLARE_`, `Cloudflare`, `cloudflare` |
