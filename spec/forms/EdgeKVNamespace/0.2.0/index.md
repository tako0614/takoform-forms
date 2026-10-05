---
title: Edge KV Namespace 0.2.0
description: 世界各地へ複製される、不透明なbyte列を格納する名前空間。反映には遅れがあります。
formUrl: https://edge.forms.takoform.com/forms/EdgeKVNamespace/0.2.0/
hostApi: forms.takoform.com/v2
---

# Edge KV Namespace 0.2.0

このFormは世界各地へ複製される、一つのキーと値の名前空間を表します。値は文字列ではなく、不透明なbyte列です。書込みの複製には遅れがあり、直後の読取りが古い値を返すことがあります。この意味はFormに固定され、Host設定で強い一貫性へ変更できません。SQLiteのschema変更は[SQLiteMigrationApplication](../../SQLiteMigrationApplication/0.2.0/)が所有します。この仕様は[Host API v2](https://takoform.com/spec/host-api/v2/http)用のForm契約であり、公開だけでHostの対応や実装を示すものではありません。

Resourceの作成・取得・更新・削除と管理OperationのHTTP形式は[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。ここでは名前空間固有の入力とWorkerから呼ぶJavaScript機能を定めます。データの読書きはHost管理Operationではなく、Worker内から直接行う非同期Binding呼出しです。

## 1. 目的と範囲

名前空間はキーから不透明なbyte列への対応を保持します。キーごとに読み書きでき、複数キーをまたぐtransactionや強い整合性はありません。TTLを設定した値は指定時間より後に削除されることがあります。WorkerにBindingを作る場合のJavaScript機能も本Formが定めます。

Formのcanonical URLは `https://edge.forms.takoform.com/forms/EdgeKVNamespace/0.2.0/` です。末尾スラッシュを含む完全なURL文字列が識別子です。URLの正規化、リダイレクト、別名で置換してはいけません。

## 2. 入力と既定値

`spec`は空のJSON object `{}`だけを受け付けます。フィールドはありません。Form由来の既定値、作成後に変更できるフィールド、Host個別設定はありません。`null`、配列、未知のフィールドを含むobjectは拒否します。HostのOfferingがある場合も、それはHost API共通の選択であり、`spec`の一部ではありません。

各JavaScript関数の引数・options objectは定義したものだけを受け付けます。キー・prefix・メタデータkeyはUTF-8 byte数、それ以外の文字列上限はUnicode scalar value数で数えます。数値は整数です。

## 3. 秘密入力

`privateInputs`はありません。`privateInputs`にキーを一つでも指定する要求は拒否します。値とメタデータは通常入力であり、秘密情報を格納する用途ではありません。

## 4. 観測状態、出力、利用可能状態

作成直後は`observed: {}`、`output: {}`、`observedAt: null`です。観測後の`observed`は次の五fieldをすべて持つobjectです。`namespaceExists`と`consistency`はbooleanと`"eventual"`、残りは整数です。

```json
{
  "namespaceExists": true,
  "maxKeyBytes": 467,
  "maxValueBytes": 26214400,
  "maxMetadataBytes": 1024,
  "consistency": "eventual"
}
```

`namespaceExists:false`は確認済みの不在、`observed:{}`は未観測です。上限はキー467 UTF-8 byte、値26,214,400 byte、メタデータ1,024 UTF-8 byteです。観測値が上限と違うHostは本Formに適合しません。`observedAt`は最後に全fieldを確認した時刻で、個々のキーの状態は示しません。`output`は常に空objectです。

このFormは利用可能状態を定義しません。名前空間の作成が完了しても、直前の書込みが別の複製先から読めるとは限りません。Hostは複製目標をサポート情報に示せますが、自分の書込みを直後に読める保証にはなりません。

## 5. 操作と失敗

HostはResourceの`create`、`read`、`update`、`delete`を実装します。空の`spec`と異なる更新は副作用前に拒否します。同じ空`spec`のPUTはHost API v2に従う新generation・新Operationとして受理し、名前空間の実在と管理対応を再確認します。HTTP受付と管理Operationは[Host API v2](https://takoform.com/spec/host-api/v2/http)に従います。次表の関数はWorkerのJavaScriptから直接呼び出し、個別のHTTP経路や管理Operation IDを持ちません。

作成では空名前空間を一つ作ります。既に存在する名前空間を別Resourceとして初期化または引き継いではいけません。新しいUIDで盲目的に作り直してはいけません。

作成や削除の応答を失った場合、元要求の再送は[Host API v2の再試行と保持](https://takoform.com/spec/host-api/v2/http#retry)に従います。保持期間内の同一要求・同一`Idempotency-Key`の再送は元Operationを返します。期限後はResource/Operationを照合し、結果不明のまま盲目的に再送してはいけません。Resource GETとOperation GETは観測だけを行い、実行を始めません。

Hostは元の実行先と識別子を使い、バックグラウンドで結果の照合を試みます。結果を安全に判定できない間は`reconciling`を維持し、運用者による確定を待ちます。元Operationが非終端の間、同じResourceへの新しいPUT/DELETEを受理してはいけません。

元Operationが`failed`で終了した後は、最新generationを指定した新しいPUT/DELETEで同じUIDの状態を収束させます。既知の部分削除は保存した進捗から再開し、対象の不在を確認できれば削除を成功にできます。PUTによる一般的な名前空間の再作成・初期化は保証しません。

readは管理記録と最後に確認したnamespace状態を返します。外部状態を確認できない場合は不在と断定しません。deleteは対象namespaceとその配下データを削除します。WorkerVersionから参照されている間は削除を受理せず`409 dependency_conflict`を返し、参照元のWorkerVersionを連鎖削除しません。

key/valueのPUT応答が失われた場合、その呼出しにHost管理Operation IDや管理用再送keyはありません。アプリケーションが再試行の効果を決めます。二重書込みを避ける必要がある場合、keyとvalueに応じた条件付けやアプリケーション側の重複防止を用います。

以下の名前はWorker Bindingで呼ぶJavaScript関数名です。すべてPromiseを返し、成功時は表の値で解決します。関数固有の失敗は、`name`がerror codeである`Error`によりPromiseを拒否します。引数型やoptionsの形が不正なら`TypeError`です。Host APIのHTTP応答や管理Operation IDは使いません。

| 関数 | 入力（必須を太字） | Promiseが解決する値 | 拒否時の`Error.name` | 再実行と一貫性 |
| --- | --- | --- | --- | --- |
| `get` | **`key`**: 1〜467 UTF-8 byte | `ArrayBuffer`または`null` | `invalid_key`, `backend_unavailable` | 読取専用。どの複製先から読むかによって古い値や不在が返る。 |
| `getWithMetadata` | **`key`**: 同上 | `{value:ArrayBuffer, metadata?: Metadata}`または`null` | `invalid_key`, `backend_unavailable` | valueとmetadataは同じ複製先の値。metadata未設定時は省略。 |
| `put` | **`key`**, **`value`**; `options`? | `undefined` | `invalid_key`, `value_too_large`, `metadata_too_large`, `backend_unavailable` | 非冪等。値を置換。受付側に保存した後、他の複製先への反映は遅れる。 |
| `delete` | **`key`** | `undefined` | `invalid_key`, `backend_unavailable` | 冪等。不在でも成功。削除後も他の複製先には古い値が残り得る。 |
| `list` | `options`? | `{keys:[{name:string}], listComplete:boolean, cursor?:string}` | `invalid_cursor`, `backend_unavailable` | 読取専用。UTF-8 byte辞書順。結果への複製反映は遅れる。 |

Worker Bindingの`put`では値に文字列（UTF-8へ変換）またはArrayBuffer/typed arrayを渡します。文字列はUTF-8へ符号化し、ArrayBuffer/typed arrayは指定byte範囲を保存します。値は最大26,214,400 byteです。範囲外は`value_too_large`です。キーは1〜467 UTF-8 byteで、違反は`invalid_key`です。メタデータは省略または最大64項目のstring mapです。keyは最大256 UTF-8 byte、valueは最大8,192 Unicode文字。RFC 8785 canonical JSONで表したmap全体は1,024 UTF-8 byte以下です。上限超過は`metadata_too_large`。メタデータは秘密ではありません。

`PutOptions`の`expirationTtlSeconds`は省略可能で、省略時は期限なしです。指定時は60〜315,360,000秒。指定時間より後に削除が進みますが、正確な時刻は保証しません。同じキーへのputは値・メタデータ・TTLを置換します。再実行も書込みになるため、応答不明時はむやみに再試行せず、アプリケーション側で効果を判断します。

`ListOptions`の`limit`は省略時100、指定時1〜1,000件です。`prefix`は省略時空文字列で最大467 UTF-8 byteです。`cursor`はHostが発行する不透明な文字列（1〜4,096文字）で、次頁要求へ変更せず渡します。無効なcursorは`invalid_cursor`です。`listComplete:false`ならcursorを含めます。ページ間のsnapshot一貫性はありません。

`put`のPromiseが解決しても、別の複製先からの`get`が新しい値を直ちに返す保証はありません。同じWorkerや同じ場所での自分の書込み直後の読取りにも保証はありません。`get`と`getWithMetadata`は不在時に`null`で解決します。`list`も複製が収束するまで作成・削除を反映しないことがあります。読取りは冪等、書込みは非冪等です。応答不明時の再実行が安全かはアプリケーションが判断します。未知optionや型・形が不正な引数は`TypeError`として拒否します。正しい型で仕様範囲を外れる要求は表の`Error.name`を持つ`Error`で拒否します。

## 6. 参照、所有、削除

Resourceが所有するのはnamespaceとそのkey/valueです。WorkerVersionはnamespaceをUIDで参照できます。生きたWorkerVersionが参照中なら削除は`409 dependency_conflict`で拒否し、何も削除しません。参照を持つWorkerVersionをHostが強制削除することもありません。参照がなくなった後のdeleteは対象namespaceと配下データを削除し、別Resourceや別namespaceには触れません。Hostは管理記録と実行先識別子を照合してから操作します。

## 7. Worker JavaScript Binding

WorkerVersionの`kvBindings`からこのResourceを参照すると、Worker内の`env.NAME`に次のオブジェクトを渡します。`NAME`はWorkerVersionで宣言されたJavaScript識別子です。参照だけで呼出者やWorkerに新しいHost認可を与えることはなく、HostはWorkerVersion作成・実行時に権限を解決します。以下の関数は本Formの規範APIで、独立したInterface版は持ちません。

```text
type Metadata = Record<string, string>
type PutOptions = { metadata?: Metadata; expirationTtlSeconds?: number }
type ListOptions = { prefix?: string; limit?: number; cursor?: string }
get(key: string): Promise<ArrayBuffer | null>
getWithMetadata(key: string): Promise<{ value: ArrayBuffer; metadata?: Metadata } | null>
put(key: string, value: ArrayBuffer | ArrayBufferView | string, options?: PutOptions): Promise<void>
delete(key: string): Promise<void>
list(options?: ListOptions): Promise<{ keys: { name: string }[]; listComplete: boolean; cursor?: string }>
```

文字列valueはUTF-8へ変換し、ArrayBuffer/typed arrayはそのbyte範囲を保存します。`get`と`getWithMetadata`が返すArrayBufferは複製された値bytesです。`PutOptions`は§5の`metadata`、`expirationTtlSeconds`だけを持ちます。`ListOptions`は§5の`prefix`、`limit`、`cursor`だけを持ちます。metadata未設定時は戻り値から`metadata`を省略します。各field上限は§5に定め、外部の名前付きAPIを追加要件にしません。

## 8. 未知のフィールドと拡張

`spec`、JavaScript関数のoptions、および各引数で定義した値以外を拒否します。metadataは文字列keyから文字列valueへのmapだけです。Host API Resource/Operation応答の未知propertyは[Host API v2](https://takoform.com/spec/host-api/v2/http)の共通規則に従います。Formの入力や動作を変える場合は別Form URLを公開します。
