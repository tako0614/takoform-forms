---
title: Object Bucket 0.3.0
description: 一つの名前空間を持つobject保管場所。本文はbyte streamで読み書きし、完了済みの変更を直後の読取りに反映します。
formUrl: https://edge.forms.takoform.com/forms/ObjectBucket/0.3.0/
hostApi: forms.takoform.com/v2
---

# Object Bucket 0.3.0

このFormは、一つの名前空間を持つobject保管場所を表します。本文はJSON文字列ではなく、Worker Bindingへ渡すbyte streamです。put/delete完了後のget、head、listはその変更を観測します。CORS、期限削除、書込ロックなどはこのFormに含みません。この仕様は[Host API v2](https://takoform.com/spec/host-api/v2/http)用の契約であり、公開だけでHostの対応や実装を示すものではありません。

Resourceの作成・取得・更新・削除と管理OperationのHTTP形式は[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。ここでは保管場所固有の入力とWorkerから呼ぶJavaScript機能を定めます。本文の読書きはHost管理Operationではなく、Worker内から直接行う非同期Binding呼出しです。

## 1. 目的と範囲

Object Bucketは一つのobject集合を所有します。keyは一つの名前で、delimiterは一覧表示をまとめるためだけに使います。上書きすると旧内容を置き換え、版履歴は持ちません。etagによるobject単位の条件付き読書きができますが、複数keyをまとめたtransactionや一括確定はありません。

Canonical Form URLは`https://edge.forms.takoform.com/forms/ObjectBucket/0.3.0/`です。末尾スラッシュまで含む文字列の完全一致が識別子です。

## 2. 入力と既定値

`spec`は空のJSON object `{}`だけです。設定fieldも既定値もありません。`null`、object以外、未知fieldは拒否します。CORS、期限削除、lockの設定をここへ追加してはいけません。作成後も空objectから変更できません。

各JavaScript関数の引数・options objectは定義したものだけを受け付けます。UTF-8 byteと明記する場合を除き、文字列上限はUnicode文字数です。

## 3. 秘密入力

`privateInputs`はありません。指定された場合は拒否します。本文やcontent typeも秘密入力ではありませんが、本文を通常の管理記録やログへ出力してはいけません。

## 4. 観測状態、出力、利用可能状態

作成時は`observed: {}`、`output: {}`、`observedAt: null`です。最初の確認後、`observed`は次の全fieldを持ちます。上限は整数、`consistency`は示した文字列だけです。

```json
{
  "bucketExists": true,
  "maxKeyBytes": 979,
  "maxObjectBytes": 5368709120,
  "maxSinglePutBytes": 314572800,
  "maxMultipartParts": 10000,
  "consistency": "strong-read-after-write"
}
```

`bucketExists:false`は確認済みの不在、`observed:{}`は未観測です。上限はkey 979 UTF-8 byte、本文5,368,709,120 byte、単一put 314,572,800 byte、multipart 10,000 partsです。上限値を変えたHostは本Formに適合しません。`consistency`は文字列`"strong-read-after-write"`だけです。`observedAt`は最後に全fieldを確認した時刻で、個別objectの新しさは示しません。`output`は常に空objectです。

このFormはアプリケーションの稼働や特定利用者からの到達性を定義しません。保管場所作成の成功だけでは利用資格、CORS設定、objectの存在を意味しません。

## 5. 操作と失敗

Host API Resource操作は作成・取得・更新・削除です。空の`spec`と異なる更新は副作用前に拒否します。同じ空`spec`のPUTはHost API v2に従う新generation・新Operationとなり、保管場所の実在と管理対応を再確認します。作成では空の保管場所を一つ作ります。所有を確認できない既存bucketを初期化・引き継いではいけません。

作成や削除の応答を失った場合、元要求の再送は[Host API v2の再試行と保持](https://takoform.com/spec/host-api/v2/http#retry)に従います。保持期間内の同一要求・同一`Idempotency-Key`の再送は元Operationを返します。期限後はResource/Operationを照合し、結果不明のまま盲目的に再送してはいけません。Resource GETとOperation GETは観測だけを行い、実行を始めません。

Hostは元の実行先と識別子を使い、バックグラウンドで結果の照合を試みます。結果を安全に判定できない間は`reconciling`を維持し、運用者による確定を待ちます。元Operationが非終端の間、同じResourceへの新しいPUT/DELETEを受理してはいけません。

元Operationが`failed`で終了した後は、最新generationを指定した新しいPUT/DELETEで同じUIDの状態を収束させます。既知の部分削除は保存した進捗から再開し、対象の不在を確認できれば削除を成功にできます。PUTによる一般的な保管場所の再作成・初期化は保証しません。

削除はこの保管場所と中の全objectを消します。取得は管理記録と最後の観測を返します。

次の関数はWorker Bindingで呼び出します。すべてPromiseを返し、成功時は表の値で解決します。関数固有の失敗は、`name`がerror codeである`Error`によりPromiseを拒否します。引数型やoptionsの形が不正なら`TypeError`です。これらの読書きはHost APIのHTTP経路、管理Operation ID、管理用再送keyを持ちません。

| 関数 | 入力 | 成功時の値 | Promise拒否時の`Error.name` | 再実行と一貫性 |
| --- | --- | --- | --- | --- |
| `head` | `key` | `{etag,size,contentType?,uploadedAtMillis?}`または`null` | `invalid_key`, `backend_unavailable` | 読取専用。現etagとbyte sizeを返す。 |
| `get` | `key, options?` | `{body: ReadableStream<Uint8Array>,etag,size,partial,contentType?,range?}`または`null` | `invalid_key`, `precondition_failed`, `range_not_satisfiable`, `backend_unavailable` | 読取専用。条件不一致ならbodyは返らない。 |
| `put` | `key, body, options?` | `{etag,size}` | `invalid_key`, `invalid_body`, `value_too_large`, `precondition_failed`, `backend_unavailable` | 非冪等。既存object全体を置換。解決後のget/head/listは変更を観測。 |
| `delete` | `key` | `undefined` | `invalid_key`, `backend_unavailable` | 冪等。不在でも成功。解決後のget/head/listは不在を観測。 |
| `list` | `options?` | `{objects,prefixes?,truncated,cursor?}` | `invalid_cursor`, `backend_unavailable` | 読取専用。UTF-8順。解決済みの変更を反映。 |
| `createMultipartUpload` | `key, options?` | `{uploadId}` | `invalid_key`, `backend_unavailable` | 非冪等。keyは予約せず、完了まで現objectに影響しない。 |
| `uploadPart` | `key, uploadId, partNumber, body, options?` | `{partNumber,etag}` | `invalid_key`, `invalid_body`, `upload_not_found`, `backend_unavailable` | 非冪等。同じ番号への書込みは当該partを置換。 |
| `completeMultipartUpload` | `key, uploadId, parts` | `{etag,size}` | `invalid_key`, `invalid_part`, `upload_not_found`, `value_too_large`, `backend_unavailable` | 非冪等。parts全体を一objectとして可視化。 |
| `abortMultipartUpload` | `key, uploadId` | `undefined` | `invalid_key`, `upload_not_found`, `backend_unavailable` | 非冪等。uploadとpartsを捨て、現keyは変更しない。 |

keyは1〜979 UTF-8 byte。object最大sizeは5,368,709,120 byte (5 GiB)、単一putは最大314,572,800 byteです。単一put上限を超えるobjectにはmultipartを使います。長さが事前に分からないstreamも、各操作の実byte上限内なら本版のputまたはuploadPartへ渡せます。

`put(key, body, options?)`と`uploadPart(..., body, options?)`のbodyはUTF-8文字列、ArrayBuffer、または`ReadableStream<Uint8Array>`です。文字列はUTF-8符号化後、ArrayBufferは渡したbyte列の長さを使います。streamではoptionsと`contentLength`を省略できます。`contentLength`のown fieldがないことは未申告の長さを意味し、`0`とは区別します。`0`は直ちに正常EOFへ到達する空bodyの宣言であり、正常EOFを確認する前に成功にしてはいけません。`contentLength`を持つ場合は有限で安全な0以上の整数を指定し、実byte数と完全一致させます。文字列・ArrayBufferでも指定値は固有のbyte数と一致させます。

`contentLength`の型がnumber以外なら`TypeError`です。own fieldの`undefined`と`null`も省略ではなく、この型違反です。numberでも非有限、小数、負数、安全な整数でない値、5 GiBを超える値は`invalid_body`です。有効な申告数が単一put上限を超えるputは本文消費前に`value_too_large`で拒否します。bodyやoptionの型・形、未知optionの違反も本文消費前に拒否します。実byte数の不足・超過、非byte chunkは`invalid_body`、producerの読取例外は診断を漏らさない`backend_unavailable`です。未申告streamも受信中に上限を検査し、putの実byte上限超過は`value_too_large`、uploadPartの実byte上限超過は`invalid_body`です。成功時の`size`は申告値から推定せず、実際に書いたbyte数です。

Hostは最初の非同期staging処理より前に、key、uploadId、partNumber、content type、条件、申告長を検証して固定し、文字列・ArrayBufferの本文も所有するbyte列へ固定します。呼出し後のoptionsやArrayBufferの変更で送信先・条件・本文を差し替えてはいけません。content typeは省略可能、指定時1〜256文字で、既定の型をFormは保証しません。

### 本文のEOF、取消し、公開

Hostは一つのproducerを高々一回消費し、backpressureに従って有限のchunk単位で転送・計数します。長さ発見だけを目的に本文全体をメモリへbufferしてはいけません。Host所有のprivateな一時保管場所へchunk単位でstageしてから公開することは許します。その名前、provider endpoint、認証情報、一時保管識別子をBindingやResource状態へ公開しません。get/head/listからstage中の本文は見えません。

putまたはpartの置換を可視化できるのは、producerの正常EOF、全chunkのbyte型、申告がある場合の正確なcount、実byte上限を確認した後だけです。HTTP bodyの終端がproducerの例外や切断を隠す場合、その終端だけでは正常EOFの証拠になりません。正常EOF前の例外、切断、取消し、型・count・上限違反は書込を公開せず、既存objectと同じ番号の完成済みpartを保持します。存続する呼出元へ取消し・切断による失敗を報告できる場合は`backend_unavailable`で拒否します。既存の条件不一致は`precondition_failed`です。条件は最終公開時のobjectに対して原子的に適用し、事前headと無条件putの組合せで代用しません。

取消しはproducerと転送先へ伝播し、Host所有のI/Oと一時保管を追跡します。producerの取消しPromiseが停止しても失敗の報告や有限のI/O停止処理を無期限に待たせません。正常EOF後に送った確定処理の応答を失った場合は、書込済みかどうか不明なことがあります。Promise拒否や接続切断を「保存されていない」の証拠にせず、本文の自動再送、headの一致だけによる別put、同じpart番号への無条件再送をしません。元の送信先・固定入力・本文整合性・確定証拠を保持して照合し、安全に結果を確定できるまで必要なprivate保管と所有を保持します。timeoutや保持期限だけで不明な書込の所有を解放・退役したり、後から到着する書込を許したままbucket削除を成功にしてはいけません。これは新しい公開Operationや照合APIを追加する規定ではありません。

ETagはobjectのbyte列を強く検証する不透明な1〜256 Unicode文字です。get optionsの`ifMatch`と`ifNoneMatch`は同時指定できず、各値は1〜256文字です。put optionsの`ifMatch`と`ifNoneMatch`も同時指定できず、`ifNoneMatch`は`"*"`だけです。条件不一致では何も変更せず、`precondition_failed`で拒否します。rangeは`{offset, length?}`で、offsetは0〜5,368,709,120、lengthは1〜同上です。offsetがobject size以上なら`range_not_satisfiable`です。length省略時は末尾までです。`size`は全objectのbyte数、`range`は実際に返した範囲です。

一覧optionsは`prefix`（既定空、最大979 UTF-8 byte）、`delimiter`（省略または1〜16文字）、`limit`（既定100、1〜1000）、`cursor`（不透明文字列、1〜4096文字）です。prefix後にdelimiterを含むkeyはobject一覧から外し、その共通prefixを重複なしの`prefixes`へ入れます。cursorは次頁要求へ変更せず渡します。認識されないcursorは`invalid_cursor`です。`truncated:true`の場合に限りcursorを返します。

multipartはpart番号1〜10,000を使います。各partは5 GiB以下、完成objectも5 GiB以下です。uploadはkeyを予約しません。`uploadPart`で再度同じ番号を書くと、そのupload内のpartを置換します。`completeMultipartUpload`のpartsは1〜10,000件で、JavaScript入力ではpart番号が昇順かつ重複なしでなければなりません。配列や各要素の型・順序・必須fieldが不正なら`TypeError`です。形が正しい場合も、登録済みpartでない、ETagが異なる等は`invalid_part`です。最大番号のpart以外は各々5,242,880 byte以上で、かつ同じbyte長でなければなりません。違反は`invalid_part`となり、完成objectを変更しません。完成時にだけobjectを一括可視化します。中断後にHostが破棄したuploadの操作は`upload_not_found`です。optionsは閉じたplain objectで、getは条件・range、putは長さ・content type・条件、listはprefix/cursor/delimiter/limit、createMultipartUploadはcontent type、uploadPartはcontentLengthだけを許します。余分な位置引数、未知option、値の型・形の不正は`TypeError`です。

putとmultipart関数は非冪等です。Promise拒否や呼出元切断で結果が分からない場合、Host APIの再送keyや管理Operation IDで本文操作を照合できると仮定してはいけません。アプリケーションが`uploadId`と各partのETagを保持し、必要に応じて条件付きputで再実行の影響を抑えます。同じkeyへの書込みは後勝ちで、key間transactionや旧版復元はありません。

## 6. 参照、所有、削除

Object Bucket Resourceは自分のbucketとそこに属する全objectを所有します。WorkerVersionはUID参照でこのResourceをBindingできます。生きたWorkerVersionが参照中ならdeleteを受理せず、`409 dependency_conflict`を返して何も削除しません。WorkerVersionを強制削除したり、連鎖削除したりしてはいけません。全参照がなくなった後のdeleteはこのbucketのみを削除します。本文はResource state、管理Operation JSON、通常ログへ複製せず、JavaScript stream経路で処理します。

## 7. Worker JavaScript Binding

WorkerVersionの`bucketBindings`がこのResourceをUIDで参照すると、Worker内の`env.NAME`に次のobjectを渡します。`NAME`はWorkerVersionで指定するJavaScript識別子です。関数は本Formの契約で、別版InterfaceやHost APIのdata-plane経路はありません。Bindingを作るときHostはWorkerVersionとResource双方の認可を検査します。

```text
type Body = string | ArrayBuffer | ReadableStream<Uint8Array>
type Range = { offset: number; length?: number }
type GetOptions = { ifMatch?: string; ifNoneMatch?: string; range?: Range }
type PutOptions = { contentLength?: number; contentType?: string; ifMatch?: string; ifNoneMatch?: "*" }
type ListOptions = { prefix?: string; cursor?: string; delimiter?: string; limit?: number }
type Head = { etag: string; size: number; contentType?: string; uploadedAtMillis?: number }
type ObjectBody = { body: ReadableStream<Uint8Array>; etag: string; size: number; partial: boolean; range?: Range; contentType?: string }
type Page = { objects: { key: string; etag: string; size: number; uploadedAtMillis?: number }[]; prefixes?: string[]; truncated: boolean; cursor?: string }
head(key: string): Promise<Head | null>
get(key: string, options?: GetOptions): Promise<ObjectBody | null>
put(key: string, body: Body, options?: PutOptions): Promise<{ etag: string; size: number }>
delete(key: string): Promise<void>
list(options?: ListOptions): Promise<Page>
createMultipartUpload(key: string, options?: { contentType?: string }): Promise<{ uploadId: string }>
uploadPart(key: string, uploadId: string, partNumber: number, body: Body, options?: { contentLength?: number }): Promise<{ etag: string; partNumber: number }>
completeMultipartUpload(key: string, uploadId: string, parts: { partNumber: number; etag: string }[]): Promise<{ etag: string; size: number }>
abortMultipartUpload(key: string, uploadId: string): Promise<void>
```

全関数はPromiseを返します。不在keyに対する`head`と`get`は`null`で解決し、`not_found`では拒否しません。読み出した本文は`body`のReadableStreamとして返します。書込み完了後の読み取りは強い整合性を持ちます。値の型やoption形に誤りがあれば`TypeError`、本文長や保管制約など関数固有の失敗は表の`Error.name`で拒否します。メッセージ本文は契約で固定しません。Binding宣言は認可や容量予約を付与しません。

## 8. 旧版との関係と拡張

本版は長さ未申告streamを受け付ける新しいFormです。[ObjectBucket 0.2.0](../../ObjectBucket/0.2.0/)のstreamの長さ必須規定は変えません。WorkerVersionは参照先Resourceの受理済みの正確なForm URLからBindingの挙動を選びます。呼出元が渡すflag、Binding名、同じkind、現在の既定値だけで旧Resourceを本版へ読み替えてはいけません。管理Resourceの空spec、上限、読取結果、strong read-after-write、multipartの非final part最小長・同長規則は旧版と同じです。v1の`edge.objects` Interfaceやそのdigestを本版の別名として扱いません。

`spec`、各関数のoptions、および配列要素で未知fieldを拒否します。Resource/Operation共通応答の未知fieldは[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。本文長を検査し、申告数との不一致を保存成功にしてはいけません。別の保管規則や一貫性を導入する場合は別Form URLを使います。
