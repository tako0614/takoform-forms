---
title: Object Bucket 0.2.0
description: 一つの名前空間を持つobject保管場所。本文はbyte streamで読み書きし、完了済みの変更を直後の読取りに反映します。
formUrl: https://edge.forms.takoform.com/forms/ObjectBucket/0.2.0/
hostApi: forms.takoform.com/v2
---

# Object Bucket 0.2.0

このFormは、一つの名前空間を持つobject保管場所を表します。本文はJSON文字列ではなく、Worker Bindingへ渡すbyte streamです。put/delete完了後のget、head、listはその変更を観測します。CORS、期限削除、書込ロックなどはこのFormに含みません。この仕様は[Host API v2](https://takoform.com/spec/host-api/v2/http)用の契約であり、公開だけでHostの対応や実装を示すものではありません。

Resourceの作成・取得・更新・削除と管理OperationのHTTP形式は[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。ここでは保管場所固有の入力とWorkerから呼ぶJavaScript機能を定めます。本文の読書きはHost管理Operationではなく、Worker内から直接行う非同期Binding呼出しです。

## 1. 目的と範囲

Object Bucketは一つのobject集合を所有します。keyは一つの名前で、delimiterは一覧表示をまとめるためだけに使います。上書きすると旧内容を置き換え、版履歴は持ちません。etagによるobject単位の条件付き読書きができますが、複数keyをまとめたtransactionや一括確定はありません。

Canonical Form URLは`https://edge.forms.takoform.com/forms/ObjectBucket/0.2.0/`です。末尾スラッシュまで含む文字列の完全一致が識別子です。

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

keyは1〜979 UTF-8 byte。object最大sizeは5,368,709,120 byte (5 GiB)。単一putは最大314,572,800 byteです。それより大きい本文や長さが事前に分からない本文にはmultipartを使います。`put(key, body, options?)`と`uploadPart(..., body, options?)`のbodyはUTF-8文字列、ArrayBuffer、または`ReadableStream<Uint8Array>`です。文字列・ArrayBufferでは`contentLength`を省略でき、省略しない場合は符号化後の正確なbyte数と一致させます。streamではoptionsと`contentLength`が必須で、有限で安全な0以上の整数を指定します。Hostはstreamを長さ発見のために丸ごとbufferしてはいけません。本文の実byte数が違えば`Error.name === "invalid_body"`で拒否し、putでは何も保存しません。`contentLength`不在・範囲外・不一致も`invalid_body`です。content typeは省略可能、指定時1〜256文字で、既定の型をFormは保証しません。

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

## 8. 未知のフィールドと拡張

`spec`、各関数のoptions、および配列要素で未知fieldを拒否します。Resource/Operation共通応答の未知fieldは[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。本文長を検査し、申告数との不一致を保存成功にしてはいけません。別の保管規則や一貫性を導入する場合は別Form URLを使います。
