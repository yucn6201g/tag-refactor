# 契約の設計 — 質問

tag-refactor は1つの実行ファイルなので、作業単位の間の受け渡しはネットワーク越しではなく、プログラムの中の呼び出しです。一方で、利用者や他のツールが読み書きする「外から見える形」もあります。このステージでは、その両方の約束(契約)を決めます。

外から見える形の候補:

| 形 | 作る側 | 使う側 |
|---|---|---|
| ルールファイル(YAML) | 利用者(下書きは `rules suggest`) | ツール(U3) |
| 保存した変更案(JSON) | ツール(U4) | ツール(U6 の apply、U8 の IaC への反映)、利用者のレビュー |
| 出力の JSON(`--format json`) | ツール(U4 ほか) | 利用者のスクリプト |
| 書き換えの記録(JSON) | ツール(U6) | ツール(U7 の rollback) |
| コマンドの使い方(サブコマンド、オプション、終了コード) | ツール | 利用者、スクリプト |

## Q1. この段階で形式を決める外向きの契約

どの形を、この段階で正式な形式(項目の名前と意味)として決めますか?(select all that apply)

A. ルールファイル(YAML)
B. 保存した変更案(JSON)
C. 出力の JSON(scan と plan の `--format json`)
D. 書き換えの記録(JSON)
E. Not yet defined
X. Other (please specify)

[Answer]: A, B, C, D

## Q2. 形式の変更と互換性

ファイルの形式を後で変えるとき、古い形式との互換性をどう扱いますか?

A. すべてのファイルに `version` を持たせる。`v1.0.0` までは互換性のない変更を許すが、そのときは `version` を上げ、古い形式を読んだら分かりやすく止める
B. 最初から互換性を保つ(古い形式も必ず読める)
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q3. 作業単位の間の受け渡しの形

プログラムの中で、作業単位(部品)の間をどうつなぎますか?

A. 部品ごとに入出力の型と、呼び出しの約束(Go のインターフェース)を決める。テストでは差し替える
B. 約束(インターフェース)は作らず、具体的な型を直接呼ぶ
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q4. 契約の持ち主

ファイルの形式や受け渡しの型は、どこが持ちますか?

A. 作る側の作業単位が持つ(ルールファイル = U3、変更案と出力の JSON = U4、記録 = U6)
B. 最小の一本(U1)で共通の「契約」の置き場所を作り、すべての形式と型をそこに置く
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q5. 部品の境目でのエラーと再試行

部品の間で、エラーや時間切れをどう受け渡しますか?

A. 部品はエラーを種類つき(権限不足、見つからない、一時的な失敗、入力の誤りなど)で返し、Cli が終了コードと案内に変える。再試行は AwsAccess だけが行う
B. 各部品が、それぞれ再試行や終了コードを決める
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- この段階で決める外向きの形式(Q1): ルールファイル(YAML)、保存した変更案(JSON)、出力の JSON(scan と plan)、書き換えの記録(JSON)のすべて
- 形式の変更と互換性(Q2): すべてのファイルに `version` を持たせる。`v1.0.0` までは互換性のない変更を許すが、そのときは `version` を上げ、古い形式を読んだら分かりやすく止める
- 作業単位の間のつなぎ方(Q3): 部品ごとに入出力の型と、呼び出しの約束(Go のインターフェース)を決め、テストで差し替える
- 契約の持ち主(Q4): 作る側の作業単位が持つ(ルールファイル = U3、変更案と出力の JSON = U4、記録 = U6)
- エラーと再試行(Q5): 部品はエラーを種類つきで返し、Cli が終了コードと案内に変える。再試行は AwsAccess だけが行う

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
