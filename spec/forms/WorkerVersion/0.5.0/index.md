---
title: WorkerVersion 0.5.0
description: 実行コード、環境と型付きBindingの固定スナップショット
formUrl: https://edge.forms.takoform.com/forms/WorkerVersion/0.5.0/
hostApi: forms.takoform.com/v2
---

# WorkerVersion 0.5.0

WorkerVersionは一つの[ModuleWorker 0.3.0](../../ModuleWorker/0.3.0/)に属する配信・実行snapshotである。
任意のbundle、handler宣言、非秘密変数、秘密変数の名前と封じた値、型付きBinding、任意のassetを
固定する。秘密値は公開Resourceには現れないが、このVersion UIDに結び付く実行内容の一部である。
トラフィックを受けるかどうかは[WorkerDeployment 0.4.0](../../WorkerDeployment/0.4.0/)が別に決める。
本Formは`forms.takoform.com/v2`のResourceであり、公開・Host対応・稼働の証明ではない。

## 入力 {#input}

`spec`は次のキーだけを持つオブジェクトである。参照値はすべて`{"resourceUid":"..."}`
という一キーのオブジェクトであり、UIDはHost API v2の非空UID文字列である。Hostは同じHost・
同じSpaceのResourceをUIDで固定し、呼出者にその参照先の利用権限があることと、以下に記した
**正確なForm URL**に一致することを副作用前に検証する。名前、kindの類推、URL redirect、
「最新」への再解決はしない。参照先の削除・再作成は古いUIDを新しいResourceへ付け替えない。

- **`worker`（必須）** — [ModuleWorker 0.3.0](../../ModuleWorker/0.3.0/)のUID。この版の所有Worker。
- **`bundle`（条件付き）** — [WorkerBundle 0.2.0](../../WorkerBundle/0.2.0/)のUID。検証済みmanifestが示すmodule bytes。handlerを一つでも宣言する場合は必須。省略できるのは以下の静的専用構成だけである。
- **`handlers`（必須）** — 重複なしの0〜3個。`fetch`、`scheduled`、`queue`だけを受け付ける。bundleがある場合は実際のdefault exportと一致させる。空配列はhandlerを提供しない。
- **`vars`（任意）** — 既定値は `{}`、最大64キー。非秘密JSON値を`env`へ投影する。
- **`requiredSensitiveVars`（任意）** — 既定値は `[]`、重複なし最大64名。秘密値の**名前だけ**を公開し、値は`privateInputs`で与える。
- **Binding配列（各任意）** — `kvBindings`、`sqliteBindings`、`bucketBindings`、`queueProducerBindings`、`serviceBindings`、`actorBindings`、`workflowBindings`。各既定値は `[]`、各最大64件。各要素は`{"name":string,"resource":{"resourceUid":string}}`。
- **`assets`（任意）** — 省略時はasset配信なし。指定時は三キーとも必須。`bundle`は[StaticAssetBundle 0.2.0](../../StaticAssetBundle/0.2.0/)の参照、`runWorkerFirst`はboolean、`notFoundHandling`は`none`または`single_page_application`。

WorkerBundleの`bundle`を省略する場合は、検証済みの`assets.bundle`を必ず指定し、
`handlers:[]`、`assets.runWorkerFirst:false`、`vars:{}`、`requiredSensitiveVars:[]`、
全Binding配列`[]`（省略した既定値を含む）だけを許す。moduleのない版にはhandler、
named Actor/Workflow class、実行用envや秘密値を置けない。assetの検証・保持が未完了なら
このVersionをReadyにしない。bundleがある版では空の`handlers`も許すが、module graphと
default exportを検証する。bundleもassetsもない版は拒否する。

例えば、既存の同一SpaceのWorkerとBundleを参照する最小の`spec`は次の形である。
UIDは説明用で、これらのUIDの存在やHost対応を示さない。

```json
{
  "worker": { "resourceUid": "worker-uid" },
  "bundle": { "resourceUid": "bundle-uid" },
  "handlers": ["fetch"]
}
```

静的ファイルだけを公開する最小の`spec`は次の形であり、コード用の空bundleやダミーの
`fetch` exportは不要である。参照先assetは先に検証・保持されていなければならない。

```json
{
  "worker": { "resourceUid": "worker-uid" },
  "handlers": [],
  "assets": {
    "bundle": { "resourceUid": "assets-uid" },
    "runWorkerFirst": false,
    "notFoundHandling": "none"
  }
}
```

Bindingの参照先は順に[EdgeKVNamespace 0.2.0](../../EdgeKVNamespace/0.2.0/)、
[SQLiteDatabase 0.2.0](../../SQLiteDatabase/0.2.0/)、
[ObjectBucket 0.2.0](../../ObjectBucket/0.2.0/)、
[AtLeastOnceQueue 0.2.0](../../AtLeastOnceQueue/0.2.0/)、
ModuleWorker 0.3.0、[ActorNamespace 0.3.0](../../ActorNamespace/0.3.0/)、
[DurableWorkflow 0.3.0](../../DurableWorkflow/0.3.0/)である。

`vars`のキーは`^[A-Za-z][A-Za-z0-9._-]{0,63}$`、Bindingの`name`は
`^[A-Za-z_$][A-Za-z0-9_$]{0,63}$`、秘密名は`^[A-Z][A-Z0-9_]{0,63}$`とする。
これらは**一つのenv名前空間**であり、種類をまたぐ重複を拒否する。各Binding配列でも同じ
`name`を二度宣言できない。`vars`値は有限数、boolean、null、Unicode文字列、通常の配列・
オブジェクトからなるJSON値に限る。ネストは最大8段、各配列・オブジェクトは最大64要素・キー、
各文字列は8192 Unicodeスカラー値、オブジェクトのキーも上記`vars`のキー文法に従う。
非秘密変数にcredentialを置かない。`spec`と各入れ子の未知のキーは拒否し、`null`を省略と
みなさない。Handler、Binding、assetの省略以外に暗黙のHost既定値はない。
同一specの判定では、省略可能な`vars`を `{}`、各Binding配列と秘密名配列を `[]`へ展開する。
`handlers`、`requiredSensitiveVars`、各Binding配列は集合として扱い、文字列またはBinding
`name`の辞書順に揃えて比較する。`vars`を含むobjectのキー順は無視し、値内の配列順は保つ。
文字列はUnicode正規化せず同じスカラー列で比較し、JSON数値は数学的な値で比較する。
省略と明示した既定値は同一である。`assets`の省略と存在は別である。

### 秘密入力

`requiredSensitiveVars`の各名前がFormの`privateInputs`名である。作成時にはHostとSupportの
`privateInputs`対応が必要で、全ての名前に非空の文字列値を一度に与える。余分な名前、欠落、
空文字列は副作用前に拒否する。`privateInputs`自体を通常の`spec`、`observed`、`output`、
Operation、ログへ複製しない。PUTで`privateInputs`を省略すれば既設定値を維持する。
PUTで与える場合は名前集合と正確に一致する**全量**を与える。Hostは既設定値との一致を
内部の安全な保管・照合手段で確認し、異なる値は副作用前に`422 invalid_spec`で拒否する。
秘密の照合用hashや値はreadback・ログ・Resourceへ出さない。部分mapも拒否する。
Hostが既設定値を失い一致を確認できない場合は、同じUIDに新値を設定して復旧したことに
せずReadyを外して不確実性を示す。受理済みOperationの秘密入力を失った場合は、Host API v2の
同じOperationへの**元と同じ値**の補給手順に従う。
`requiredSensitiveVars:[]`の場合、`privateInputs`は省略または空objectだけを許す。
秘密値を変更するには新しいWorkerVersionを作り、Deploymentを明示的に切り替える。
同一spec・同一秘密値での新しいIdempotency-KeyのPUTも新しいOperation/generationを持つ。

## 実行とBinding

WorkerBundleがある場合、Hostはmodule graph・media type・exportを
[ModuleWorkerの実行Interface](../../ModuleWorker/0.3.0/)どおり検証する。
`env`のown enumerable key集合は宣言した変数・秘密名・Binding名と正確に一致し、
宣言外の権限を与えない。Bindingは参照先への操作能力であり、参照やForm URLだけで利用者権限、
Host credential、相手の秘密値を取得できない。解決できない必須BindingがあればReadyにしない。

`env[name]`に投影するJavaScriptメソッドと、その契約の所有先は次のとおり。

- **`kvBindings`** — [EdgeKVNamespace](../../EdgeKVNamespace/0.2.0/)のWorker Binding。`get`、`getWithMetadata`、`put`、`delete`、`list`。
- **`sqliteBindings`** — [SQLiteDatabase](../../SQLiteDatabase/0.2.0/)のWorker Binding。`execute`、`query`、`transaction`。
- **`bucketBindings`** — [ObjectBucket](../../ObjectBucket/0.2.0/)のWorker Binding。`head`、`get`、`put`、`delete`、`list`、`createMultipartUpload`、`uploadPart`、`completeMultipartUpload`、`abortMultipartUpload`。
- **`queueProducerBindings`** — [AtLeastOnceQueue](../../AtLeastOnceQueue/0.2.0/)のproducer Binding。`send`と`sendBatch`のみ。
- **`serviceBindings`** — [ModuleWorker](../../ModuleWorker/0.3.0/)のservice Binding。`fetch(Request)`またはURLとinitを受け取り、streaming `Response`を返す。
- **`actorBindings`** — [ActorNamespace](../../ActorNamespace/0.3.0/)のcaller Binding。同期`idFromName(name)`、`newUniqueId()`、`get(id)`、stubの`fetch`。
- **`workflowBindings`** — [DurableWorkflow](../../DurableWorkflow/0.3.0/)のcaller Binding。`create({id?,params?})`、`get(id)`、instanceの`status()`、`sendEvent({type,payload?})`、`terminate()`。

引数、Promiseの結果、
`Error.name`、整合性、上限はリンク先Formの版固定Worker Binding章を適用する。
これはHost API v2の管理HTTP/Operationとは別の実行Interfaceである。例えばSQLの
`execute`はPromiseを返し、失敗はJavaScriptのErrorとしてrejectする。管理PUTの
`Idempotency-Key`やOperation IDをSQL呼出しへ渡さない。独立したInterface/Bindingの版系列や
旧package digestを新しい適合条件としない。
同一WorkerがActor/Workflow namespaceを提供する場合、全weighted Versionのmain moduleは
対応する`className`のnamed exportとcallable prototypeを持たなければならない。
WorkerVersion作成時にまだnamespaceがなくても後のDeploymentで検証し、循環した作成順序を
要求しない。bundleのない静的専用Versionはnamed classを提供できない。任意のclassが欠ける
Versionを稼働へ選択してはならない。

assetがある場合、探索keyはworkerへ渡す`Request.url`のURL pathnameから作る。
Hostは同じURL構文のpathname（percent-encodedのまま、queryは除外）を`/`で分割し、
各segmentを**一度だけ**percent-decodeしたUTF-8文字列とする。invalid escape/UTF-8、
decode後に`/`・backslash・制御文字を含むsegment、`.`/`..` segment、途中の空segmentは
探索不能として扱う。Unicode正規化、大文字小文字変換、二重decodeをしない。先頭`/`だけを
除き、rootまたは末尾`/`なら`index.html`を補う。得た相対pathをasset manifestのpathと
文字列で完全一致させ、extension補完・外部取得はしない。探索不能ならasset missであり、
SPA fallbackも適用しない。query文字列はkeyに影響しない。

asset探索とSPA fallbackは`GET`/`HEAD`要求にだけ適用する。assetが見つかれば
manifestに宣言したmedia typeのHTTP 200応答を返す。asset応答の`HEAD`は同じpathの
`GET`用assetと同じheaderを持ち、bodyだけを送らない。`runWorkerFirst:false`では
asset探索（必要ならSPA fallback）を先に行い、
miss時だけ宣言済みfetchを呼ぶ。`true`ではfetchを先に呼び、その応答がHTTP 404のときだけ
asset探索（必要ならSPA fallback）を行い、assetが見つかればその応答を返し、missなら元の
fetch応答を保つ。`notFoundHandling:single_page_application`は有効な探索keyに一致する
assetが無い場合だけasset bundle rootの`index.html`を返す。indexが無いasset bundleは
Version保存前に拒否する。`GET`/`HEAD`以外はassetを探索せず、宣言済みfetchがあれば
そのhandlerへ渡し、なければHTTP 404を返す。fetchを宣言しないVersionでは
`runWorkerFirst:true`を拒否し、assetもSPA fallbackもmissならHTTP 404を返す。
未宣言handlerを暗黙に呼ばない。公開Endpoint、CustomDomain、service BindingのHTTP要求は
同じ選択済みVersionのこの規則で配送し、入口ごとにassetの優先順位を変えない。
asset添付はWorkerBundleを変更せず、Workerへの隠れたBindingを作らない。

## 状態、CRUD、所有

`observed`は未観測なら `{}`。確認済みなら`ready:boolean`と`resolvedBindings:boolean`を持ち、
WorkerBundleを指定した場合だけ`bundleVerified:boolean`を持つ。bundleを省略した静的専用
Versionでは`bundleVerified`を省略し、`false`で代用しない。`ready:true`は必要な参照、
asset bytes、秘密、指定されたmodule exportが解決され、
この版を選択可能であることだけを表す。`output`は `{}`。GETは最後の確認結果を返し、
現在トラフィックを受けている保証ではない。

作成は検証済みsnapshotを確保する。更新は**同じ正規化spec**のみ許し、別bundle、handler、
参照、vars、Binding、assetへの変更は副作用前に拒否する。同じspecのPUTはHost API v2どおり
新しいOperation/generationを作り再照合する。実行内容を変えるには新しいWorkerVersionを作る。
削除はDeploymentのweighted選択か、まだ退役していないinvocation/contextがこの版を参照する間、
`409 dependency_conflict`で拒否する。停止と参照消失を確認した後の削除はこのVersionの記録と
Host管理の実行コピーだけを除く。WorkerBundle、asset、Binding先のResourceやそのデータは
削除しない。Actorの休眠store/alarm/socket identityやWorkflowのsleeping/waiting instance・
履歴は、稼働中contextがなければ古いVersion UIDを保持し続ける理由にならない。これらは
各Namespace/Workflow UIDのdataであり、再配送にはその時点の有効Deploymentが必要である。
Binding参照が残る**生きたVersion**は、weightedか否かに関係なくBinding先Resourceの削除を
妨げる。応答喪失時は元のIdempotency-Keyで同じOperationを取得し、不明な結果を新UIDで
作り直さない。未知の入力や変更された意味を受ける場合は新しいForm URLを発行する。
