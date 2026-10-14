# Takoform 1.0 標準化案 — takoserver側レビュー

作成: 2026-10-15 JST（Takoserver担当向け、読取り調査のみ）。対象: 同ディレクトリの `PROPOSAL.md`。
状態: **takoserverの立場からの意見。CTO決定・実装着手・合意を表すものではない。** repo・PR・公開物は変更していない。

## 0. 照合した基準

| 記号 | 参照点 | 備考 |
| --- | --- | --- |
| S | `takoserver` origin/main `376afb5c`（2026-10-15 00:14 JST, #353） | 提案の基準 `82917e20` の直後。差分は #353（hosted portable Tier 1 sample harness, `scripts/wfp-hosted-staging-e2e.ts`, `tests/wfp-hosted-staging-sample.test.ts`）のみ。通常checkout `6b261a9e` は古いので `git show` で読んだ。 |
| Q | `takoserver-private` origin/main `7e03ae5e` | 提案と同じ。 |
| R / B / K | edge-coverage REPORT、v2-blockers REPORT、Container SPEC | 工数の基準線は R §6（0.x 全型qualification: 23–44人日）。 |

コードで確認した主な事実:

- Form URLは各Formファイルに**1版だけ**ハードコード（例 `src/takoform-v2/forms/worker-specs.ts:5–9`、`object-bucket.ts:2`）。WorkerVersionのBinding先も `src/takoform-v2/forms/worker-references.ts:22–28` で0.x URLに固定。→ 1.0追加は「新parserを足す」だけでなく、**exact URL→backendの複数版レジストリ化と参照グラフの版対応**が前提作業になる。
- `worker-specs.ts:19–21,64–76,340–350` に `kvBindings`／`bucketBindings`／`requiredSensitiveVars`／`runWorkerFirst`／`notFoundHandling` がある（提案の記述どおり）。
- SQL BLOBの `{encoding:"base64",data}` はアプリ境界まで出ている（`src/providers/selfhost-worker-wrapper.ts:1484–1491,2737,2844`、`selfhost-v2-sqlite-worker-projection.ts:97`）。
- Cronは `scheduledTime: number`（epoch分境界）をcursorと配送に使用（`worker-cron-trigger-scheduler.ts:27,48,108–124`）。occurrenceIdは未実装だが (trigger UID, scheduledTime) から決定的に作れる。
- Bundle媒体型 `application/javascript+module` は `forms/worker-bundle.ts:24,156`、`worker-code-eligibility.ts:706`、`worker-code-runtime.ts:224`、`workflow-native-selection.ts:216` に散在。
- `worker-static-runtime.ts` にRange処理はない（asset Range必須化は新規実装）。
- Q `src/entry-cloudflare-v2-host.ts:101–114` は通常profileで一部Formを retainedForms に置く（R §1と一致）。Q のWfP wrapperは v2〜v6 が並存（`src/wfp/cloudflare-managed-worker-wrapper-v*.ts`）し、1.0 ABIは**さらに一つ wrapper 世代を足す**ことになる。
- #353 harness は0.x portable API（`execute/query/transaction`、KV metadata/TTL、bucket CRUD/range/conditional/multipart、service fetch）を前提。1.0 ABIでは書き直しが必要。

## 1. 総合判断

**方向性には賛成、ただし11/15 launchとの両立のため第1段階のruntime変更は絞るべき。**

- 賛成: Host API v2据置き、17 kind名維持、管理specの語彙改名（values／secretNames／edgeKvBindings／objectBucketBindings／assets.order／fallback）、SQL BLOB→Uint8Array、KV get統合とttlSeconds、cron `scheduledAt:Date`＋occurrenceId、service fetch の RequestInit 対応、`text/javascript`。これらは**機械的でtakoserverの既存設計（authority／broker／内部Base64 wire）を壊さない**。
- 反対／縮小を求める: (a) ObjectBucket の **R2非等長part再構成**と**未知長streamのstaging**を1.0必須にすること、(b) put／delete／completeまで **Response返却**にすること、(c) asset **Range必須化**、(d) 0.xと1.0の**長期並行サポート（90日）をlaunch時点から**前提にすること。
- 最大のリスクはスケジュール。R §6 の0.x qualification（23–44人日）の大半は、1.0でruntime ABIが変わるとやり直しになる。**launch対象を0.xから1.0へ切り替えるなら、0.xのhosted qualificationへの投資を今止める判断が必要**（§6 エスカレーション）。

## 2. 第1段階 Form別レビュー

工数は takoserver(public)+takoserver-private 合算の人日仮説（実装＋focused test。3経路の実backend受入は§5の横断分に計上）。

### 2.1 ModuleWorker 1.0
- 判断: **賛成（条件付き）**。空spec・3 handler名・default export維持、`InvocationContext.signal` 追加、標準エラー明文化はOK。
- 影響: `src/providers/selfhost-worker-wrapper.ts`／`selfhost-worker-prelude.ts`（self-host）、Q `cloudflare-managed-worker-wrapper-v6.ts`＋`v5-prelude` の後継世代、`worker-code-eligibility.ts`／`worker-module-semantic-inspection.ts`（export検査）。**3–5人日**。
- 3環境で同一にできない点: (1) **client切断時の `signal` 発火** — workerdでは request.signal の切断伝播が互換フラグ依存で、WfP dispatch経由の挙動は未検証。Linux/Dockerの自前frontendでは実装可能。→「切断時にabortされ得る（best-effort）、タイムアウト／退役では必ずabort」と規定してほしい。(2) **waitUntil の寿命上限** — CFはplatform上限（約30秒）、Linuxは任意。共通上限値（例: 30秒）をFormで固定し、Linux側も合わせる。
- 代替案: `TakoformRuntimeError` は isolate 境界を越えるため `instanceof` 判定を保証できない。**`error.name === "TakoformRuntimeError"` と `code` で判定**を規範にすること。

### 2.2 WorkerBundle 1.0
- 判断: **賛成**。`text/javascript` 採用、旧媒体型は0.xのみ。
- 影響: 上記4ファイルの媒体型マップ、workerdへ渡す `moduleMediaTypes` の変換（workerd内部は旧値のままでよい＝adapter内に閉じる）。**1–2人日**。
- 非同一点: なし（loader規則はTakoform定義）。
- D07回答: accepted media type の閉集合は `text/javascript`、`application/wasm`、`text/plain`（text module）、`application/octet-stream`（bytes）、`application/json` を提案。json importはWfP/workerdとBunで扱いが揃うことを確認済みのものだけに限定。

### 2.3 WorkerVersion 1.0
- 判断: **賛成**（改名・asset形とも）。旧名aliasを受けない方針も賛成。
- 影響: `forms/worker-specs.ts`（新parser、旧は不変）、`forms/worker-references.ts`（版ごとの参照先表）、`worker-code-runtime.ts:255–259`／`worker-static-runtime.ts:88–92`／`workerd-version-graph.ts`／`workerd-site-contract.ts`（order／fallback→native設定の変換）、Q `v2-worker-version-backend.ts`／`v2-worker-version-configured-inputs.ts`／`v2-worker-assets.ts`。**3–5人日**＋複数版レジストリ化（§5横断）。
- 非同一点: なし。`fallback` はWfPネイティブのSPA動作（`single-page-application`）が「index.html固定」なので、**fallbackは当面 `null|"index.html"` の2値に限定**してほしい。任意pathをfallbackにするとWfPのnative assetsでは表現できず、handler側でのエミュレーションが必要になる（+2–3人日、性能劣化）。
- 異議: **最終1.0がTier 2のcontainerBindings等に依存して11/04まで固定できない**点。Tier 1をlaunch必須にするなら、WorkerVersion 1.0.0はTier 1 Binding＋既存queue/actor/workflow配列で固定し、containerBindingsは **WorkerVersion 1.1.0（追加のみ）** に回すことを提案。1.0で将来配列を予約するだけでも可。

### 2.4 WorkerDeployment 1.0
- 判断: **賛成**。shape維持。
- 影響: `worker-deployment-backend.ts`、Q `v2-worker-deployment-publication.ts`／`-drain.ts`。旧新Form URLを跨ぐsingle-active（同じWorkerの0.x Deploymentと1.0 Deploymentを同時activeにしない）検査を追加。**2–3人日**。
- 提案: 1.0 Deploymentは**1.0 Versionのみ**を列挙可能とし、0.x Versionとの混在は拒否（提案と同じ）。Worker UIDが新規になるため実際の衝突は少ない。

### 2.5 WorkerEndpoint 1.0
- 判断: **賛成**。
- 影響: `worker-endpoint-backend.ts`、Q `v2-worker-endpoint-publication.ts`。版URL追加のみ。**0.5–1人日**。
- 非同一点: 割当hostnameのsuffixは環境ごとに異なる（WfP wildcard／self-hostの運用者ドメイン）。「hostnameはHost割当・不透明」で十分。

### 2.6 SQLiteDatabase 1.0 ＋ sqlite Binding
- 判断: **賛成**。BLOB→Uint8Array、safe integer範囲維持、query rollback-only維持、`{signal}`。
- 影響: `selfhost-worker-wrapper.ts`（Base64⇄Uint8Array変換をfacadeへ）、`selfhost-v2-sqlite-worker-projection.ts`、`selfhost-v2-sqlite-binding-broker.ts`、Q `v2-worker-sqlite-binding-runtime.ts`／`-rpc.ts`。内部wireはBase64のまま。**2–4人日**。
- 非同一点: `signal` による**実行中SQLの中断**は、Node SQLite（hosted）／bun:sqlite（self-host）とも文単位で中断できない。→「signalは送信前／結果待ちを取消すが、既にbrokerへ渡った文はcommitされ得る（結果unknown）」と明記すること。
- D03回答: full int64は**1.0では不採用**（範囲外は従来どおり失敗）。bigint対応は将来の1.x追加（opt-in option）とする。

### 2.7 SQLiteMigrationSet 1.0
- 判断: **賛成**。**1人日**（URL追加と参照整合）。非同一点なし。

### 2.8 SQLiteMigrationApplication 1.0
- 判断: **賛成**。
- 影響: Form自体は**1–2人日**。ただし**hosted v2 compositionへのmountが未実装**（R §3）で、これは1.0と無関係に必要な既存不足（3–5人日、R見積り内）。
- 非同一点: なし（SQLite方言はどこでも同じ）。D1 backendは使わない（hostedアプリDBはNode SQLite）ことを維持。

### 2.9 ObjectBucket 1.0 ＋ object-bucket Binding
- 判断: **部分賛成・最大の異議点**。
- 影響（提案どおり全部やる場合）: `src/providers/edge-objects-adapter.ts`（1429行、R2Object形の変換の中心）、`selfhost-v2-object-bucket-{store,binding-broker,data-service}.ts`、`selfhost-object-store.ts`、Q `v2-object-bucket-r2-data-executor.ts`（`FixedLengthStream`依存）、`v2-object-bucket-stream-frame.ts`、`v2-worker-object-bucket-binding-runtime.ts`、`v2-object-bucket-namespace.ts`。
  - Response/Headers化＋S3 header subset: **5–8人日**
  - 未知長streamの上限付きstaging（永続一時領域、孤児回収、quota）: **3–5人日**
  - R2非等長partの再構成（受理partの保持→complete時に再分割／コピー、費用・復旧・retention）: **5–8人日＋R2費用増**
- 3環境で同一にできない点: (1) **R2はnon-final partの等長が必須**、self-host（ファイル）では不要。中立化するにはCF側だけ再構成が要る。(2) **R2 putは長さ既知が必須**（`FixedLengthStream`）。未知長はCFだけstagingが要る。(3) ETag形式はbackendごとに異なる（不透明扱いで可）。
- 代替案（推奨）:
  1. **D05は「等長を明記したS3 subsetを全backend共通で採用」**（提案のフォールバック案）。S3でも等長partは有効入力なので、利用者側の制約が増えるだけで意味は3環境で一致する。
  2. **未知長streamは1.0では非対応**。stream入力には `Content-Length` header を必須とし、不一致は失敗。Blob／BufferSource／stringは長さ既知。未知長は1.x追加。
  3. **get／headはResponse**（404/304/412/416を含む）で賛成。**put／delete／completeMultipartUploadは型付き結果**（put/complete: `{etag}`、条件不成立は `null` または 412 相当の固定code、delete: `void`）を提案。Responseにする利点が小さく、変換・テスト面だけ増える。
  - この3点で ObjectBucket 工数は **5–8人日**（提案全量なら13–21人日）。

### 2.10 EdgeKVNamespace 1.0 ＋ edge-kv Binding
- 判断: **賛成**。get統合（`KvEntry|null`、valueはBlob）、`ttlSeconds`、`items/nextCursor`、eventualを最低保証に。
- 影響: `src/providers/selfhost-v2-kv-store.ts`（listComplete等 674–684行付近）、`selfhost-v2-kv-binding-broker.ts`、`selfhost-worker-wrapper.ts`、Q `v2-worker-edge-kv-wrapper.ts`、`v2-worker-kv-cursor*.ts`（cursor束縛は既に実装済みで流用可）。**3–5人日**。
- 非同一点: (1) 整合性はCF=eventual（最大約60秒）、self-host=strong。最低保証eventualで一致させる案に賛成（D06）。(2) CF KVの**TTL最小値60秒**。→ `ttlSeconds` の最小値を60としてForm側で固定してほしい（Linuxは短いTTLを実装できるが、意味を揃えるため）。(3) 1 key/秒程度の書込みレート制限はCF固有。共通profileでは「同一keyへの高頻度書込みは保証しない」とする。
- 異議: putの入力に `ReadableStream` を含めるなら上限（25MiB）までmaterializeが必要。1.0では**stream入力を外すか、Content-Length必須**にすることを提案（ObjectBucketと同じ理由）。

### 2.11 StaticAssetBundle 1.0
- 判断: **賛成**（shape維持、配信順はVersionへ）。
- 影響: `forms/static-asset-bundle.ts` のURL追加、`worker-static-runtime.ts`／`worker-static-publication.ts`、Q `v2-worker-assets.ts`／`v2-worker-asset-probe.ts`。**1–2人日**。
- D07回答: **asset Rangeは1.0では任意**（実装してもよいが必須にしない）。現self-host静的runtimeにRange処理はなく、必須化は+2–3人日。Content-Type／Content-Length／ETag／If-None-Match（304）／HEADの必須化には賛成。

### 2.12 WorkerCronTrigger 1.0
- 判断: **賛成**。`scheduledAt:Date`、`cron`、`triggerUid`、`occurrenceId`。
- 影響: `worker-cron-trigger-scheduler.ts`（match ledgerは流用、イベント投影のみ変更）、`selfhost-worker-wrapper.ts` scheduled部、Q `v2-worker-scheduled-event.ts`／`v2-wfp-scheduled-capability.ts`。occurrenceIdは `triggerUid + scheduledTime(分)` から決定的に生成。**2–3人日**。
- 非同一点: CFネイティブcronは**秒精度・発火遅延・取りこぼし時の挙動がplatform依存**。現Qは DO alarm fallback（private #126）で自前ledgerを持つ方向なので、「分境界の時刻、遅延上限は保証しない、記録済みoccurrenceはat-least-once」で3環境一致させられる。ネイティブCF cron triggerを直接使う構成は1.0の意味を満たさない可能性があるため、**hostedもledger経由を必須**とすることを推奨。

### 2.13 service Binding
- 判断: **賛成**。`fetch(Request|URL|string, RequestInit?)`、redirect非追従、固定UID宛。
- 影響: `src/selfhost-v2-service-binding-broker.ts`、`src/takoform-v2/service-binding-authority.ts`、`worker-service-resolution.ts`、Q `v2-worker-service-binding-runtime.ts`。**2–4人日**。
- 非同一点: (1) CF service bindingはネイティブFetcherで**双方向stream・取消が自然に伝播**、self-hostはbroker経由で取消・backpressureを自前実装（既存基盤あり）。(2) `cf` property・RPCメソッドを出さない点は賛成。(3) `RequestInit.signal` による取消時、callee側でのabort通知はbest-effortとする。

## 3. 第1段階 open decisions への回答（D01–D07）

| ID | takoserverの回答 |
| --- | --- |
| D01 | **賛成**。17 kind維持、values／secretNames／edgeKvBindings／objectBucketBindings／assets.order／fallback。ただし fallback は `null|"index.html"` に限定。WorkerVersion 1.0はTier 1で固定し、containerBindingsは1.1.0追加を提案。 |
| D02 | **賛成**。WinterTC Minimum Common APIを参照版固定で引用、WPT subsetは fetch/streams/encoding/url/abort の主要部に限定。エラー判定は `name`＋`code`（instanceof非保証）。signalの切断伝播はbest-effort。 |
| D03 | **賛成**。BLOB=Uint8Array、safe integer据置き、int64は1.0不採用、query rollback-only維持。signal中断はunknownになり得る旨を明記。 |
| D04 | **部分賛成**。get/headはResponse（200/206/304/404/412/416、quoted ETag）。put/delete/completeは型付き結果を提案。header許可集合は提案候補どおり、`x-amz-meta-*` 合計2KiB上限を提案。 |
| D05 | **等長S3 subsetを採用**（提案のフォールバック）。未知長streamは1.0非対応、Content-Length必須。staging／再構成は1.x以降。 |
| D06 | **賛成**。eventualを最低保証、地理分散は非必須。ttlSeconds最小60を追加。 |
| D07 | `text/javascript` 採用（閉集合は§2.2）。order／fallback採用。cronはDate＋occurrenceId。**asset Rangeは任意**。 |

## 4. Tier 2 / Tier 3（簡潔）

- **AtLeastOnceQueue／QueueConsumer**: 賛成（Blob／Date／signal）。sendBatch全件原子性はCF Queuesネイティブでは保証が弱いため、Qの自前queue（SQLite node）経由を維持する前提で可。各2–4人日。stream入力は上限付きmaterialize必須なので1.0では外すことを提案。
- **WorkerCustomDomain**: 賛成（hostname／workerのみ）。ただし**hosted v2 mountが未着手**（R §3）で、1.0以前の問題。DNS/TLS待ちがあり3–6人日＋外部待ち。launch必須ならCTO判断が必要。
- **Container**: 名称 Container／`container` Binding に賛成。**secret-free初期版**、linux/amd64、`TAKOFORM_` 予約、outboundInternet両値の実装に賛成。Linux＝systemd/Bun＋Docker Engine（D11）。現 `selfhost-container-{lifecycle,runtime}.ts` は legacy authority で outboundInternet:true を拒否、v2 Resource/Operation接続も未実装。WfPはCloudflare Containers（beta）依存で可用性・料金・リージョンが未確定。**10–15人日、launch必須にしないことを推奨**。
- **ActorNamespace**: getByName／get／createId改名は賛成（3–5人日、`src/actor-addressing.ts`／`actor-namespace-facade.ts`／Q actor binding）。**upgrade（D12）は明示 `ActorUpgrade` 型案に賛成**、ただし `src/actor-upgrade-handoff.ts`（479行）とQ socket brokerの作り直しで5–8人日。launch時はWebSocket upgradeを含めず、Actor 1.1で追加する選択肢を推奨。
- **DurableWorkflow**: 賛成（JSON限定維持、WorkflowEntrypoint継承不要）。実行中instanceは旧ABIで完了（D13賛成）。2–4人日。real platform qualificationが別途5–10人日（R §6）。
- **D14（0.x保持期間）**: Provider 5.0.0は未Registry掲載で、v2 0.xの外部利用者は事実上いない見込み。**0.xはlaunch時に新規CREATE停止（retained）、read/delete/recoveryのみ維持**、90日並行サポートは不要と考える（要CTO確認）。
- **D15**: Provider側。takoserverは exact URL を advertise するだけなので意見なし。

## 5. 工数まとめ（takoserver public＋private）

| 項目 | 推奨案（本レビューの縮小案） | 提案全量 |
| --- | ---: | ---: |
| 横断: exact URL複数版レジストリ化、0.x retained化、reference graph版対応 | 3–5 | 3–5 |
| ModuleWorker／Bundle／Version／Deployment／Endpoint | 10–16 | 12–19（任意fallback含む） |
| SQLite DB／MigrationSet／Application＋sqlite | 4–7 | 4–7 |
| ObjectBucket＋object-bucket | 5–8 | 13–21 |
| EdgeKV＋edge-kv | 3–5 | 3–6 |
| StaticAssetBundle | 1–2 | 3–5（Range必須） |
| Cron | 2–3 | 2–3 |
| service | 2–4 | 2–4 |
| #353 harness・fixture・focused testの1.0化 | 2–3 | 3–4 |
| **Tier 1 小計** | **32–53人日** | **45–74人日** |
| Tier 1の3経路（WfP／Docker／Linux）実backend受入 | R §6の該当分（約12–20人日）を**0.xではなく1.0で実施**すれば追加ではなく置換 | 同左 |
| Tier 2/3（Queue／Domain／Container／Actor／Workflow）1.0化 | 25–40人日（Container・Actor upgradeを外せば約15–25） | — |

**結論**: Tier 1だけでも1.0化は約1.5–2.5人月。R §6の既存残（23–44人日）と合わせると、4担当並行でも11/15に全Tier qualifiedは困難。Tier 1を1.0でlaunch、Tier 2/3は0.xのままlaunchまたは後追い、のどちらかに絞る必要がある。

## 6. CTOへのエスカレーション事項

1. **launch対象の版**: 11/15にlaunchするのは 0.x か 1.0 か。1.0なら、現在進行中の0.x hosted qualification（#351/#353、private #125/#126等）のうちruntime ABIに依存する受入作業を止めて1.0へ振り替える判断が必要。両方は工数的に不可能。
2. **Tier 1 runtime範囲の縮小**: ObjectBucket の等長part・未知長stream非対応（D05）、put/delete/complete の型付き結果（D04）、asset Range任意（D07）、fallback 2値。これを10/21で決めないとTier 1工数が+13人日程度膨らむ。
3. **Tier 2/3のlaunch扱い**: Container（CF Containers依存）とActor WebSocket upgradeはlaunch必須から外すことを推奨。CustomDomain hosted mountはlaunch必須か。
4. **WorkerVersion 1.0の固定時期**: Tier 2依存で11/04まで固定できない構造は、10/21にTier 1を確定する意味を弱める。containerBindingsを1.1.0へ回すか。
5. **0.x並行サポート期間**（D14）: 外部利用者がいない前提で「0.xは即retained・新規停止」にできるか。

## 7. 本レビューの限界

読取りのみ。build・test・実backend試験・CF API操作は行っていない。ネイティブ制約（R2等長part、KV TTL最小値、workerd request.signal、waitUntil上限、CF cron精度）は既知の仕様に基づく記述で、今回のアカウント／WfP構成での実測ではない。工数は仮説で、担当のコミットメントではない。

## 追記 2026-10-15 01:2x JST: D05 error 仕様（CTO 01:20 改訂の指示）
ObjectBucket `put(key, body)` / `uploadPart(...)` で body が長さ不明の `ReadableStream` の場合（Content-Length 未指定）:
- **reject する error**: `TypeError`（fetch の不正 body と同じ class。Web 標準の型は変えない）。
  - `error.name = "TypeError"`
  - `error.code = "TAKOFORM_LENGTH_REQUIRED"`
  - `error.status = 411`（HTTP 411 Length Required 相当。HTTP 経由の control API では 411 を返す）
  - message 例: `ObjectBucket.put: a ReadableStream body requires a Content-Length (411 Length Required); unknown-length streams are not supported in 1.0`
- **副作用なし**: stream を読み始める前に判定する（lock しない・1 byte も消費しない）。object／part は作らない。retry 可能。
- **長さ不一致**（宣言 Content-Length と実 byte 数が違う）: `RangeError`、`code = "TAKOFORM_CONTENT_LENGTH_MISMATCH"`、`status = 400`。object／part は commit しない（中途データは破棄）。
- **multipart の part 長不一致**（最終 part 以外が等長でない）: `RangeError`、`code = "TAKOFORM_PART_SIZE_MISMATCH"`、`status = 400`。uploadPart 時に検出できれば即、遅くとも completeMultipartUpload で reject。
- Blob／BufferSource／string は長さ既知なので対象外。3 backend（WfP/R2・Docker・Linux）で同じ class/code/status を返す。未知長 stream は 1.x で追加（code は将来も予約）。
