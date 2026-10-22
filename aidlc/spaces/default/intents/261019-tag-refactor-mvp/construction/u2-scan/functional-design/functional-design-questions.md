# 機能設計 — 質問(U2 収集)

U2 は、U1 の最小版の `scan` を完成させます(`inception/units-generation/unit-of-work.md` の U2、ストーリー US1.1・US1.2)。

決まっていること(質問しません):

- 1アカウントの有効なすべてのリージョンを読む。既定は全リージョンで、`--region` を繰り返して絞れる(U1 の機能仕様の仮定)。
- 一度もタグが付いていないリソースは、AWS Config が使えればそれで、だめなら Resource Explorer で探す。どちらも使えなければ、理由を示して残りを続ける。それだけでは終了コードを 1 にしない(要件 FR1.2・FR1.3、契約 C5)。
- 読めなかったリージョンは、理由とともに結果の先頭に示し、結果を不完全として終了コード 1 にする(FR1.5)。
- 進み具合は標準エラーに出す。端末なら行を書き換え、端末でなければ1行ずつ追記する(FR1.4、モックアップ 画面1)。
- 時間の上限、待ち時間を延ばす再試行、呼び出し回数の上限への対応は AwsAccess の中で行う(NFR5、契約 C6)。
- グローバルなリソースは ARN で重複を除いて1回だけ数える(BR の詳細は設計で決める)。
- CloudFormation で作ったリソースの印(IaC で管理されている)は、リソースに付く `aws:cloudformation:stack-name` のタグで判定する(追加の権限が要らないため)。
- タグを変更できない種類と件数を示す(AS3)。

## Q1. 一度もタグが付いていないリソースを探すときの件数の上限(要件 OQ5)

AWS Config や Resource Explorer で、一度もタグが付いていないリソースを探すとき、件数に上限を設けますか?

A. 上限を設けず、最後まで読む(性能の目標 NFR1 の1万件の範囲を想定)
B. 1万件で打ち切り、「打ち切った」ことを示して結果を不完全(終了コード 1)にする
C. 既定は上限なしで、オプションで上限を指定できる
X. Other (please specify)

[Answer]: A

## Q2. 表記ゆれのまとめ方(受け入れ条件 AC1.1.6)

AC1.1.6 は、`env`、`Env`、`environment` を1つのまとまりにすることを求めています。U1 は大文字・小文字と区切り文字の違いだけでまとめるので、`env` と `environment` はまとまりません。略語の辞書をどこで入れますか?

A. U2 で、よく使う略語の小さな辞書(例: env⇔environment、prod⇔production、dev⇔development、stg⇔staging)を入れて、scan のまとまりに使う。U3 の `rules suggest` も同じ辞書を使う
B. 辞書は U3(ルール)で入れる。U2 の scan は、U1 と同じまとめ方のままにする(AC1.1.6 は U3 で満たす)
X. Other (please specify)

[Answer]: A

## Q3. 必須タグの欠落の件数(受け入れ条件 AC1.1.7)

AC1.1.7 は、ルールファイルで必須にしたタグが付いていないリソースの件数を、scan で示すことを求めています。ルールファイルの `required` の読み込みは U3 の受け持ちです。U2 ではどうしますか?

A. U2 の scan が、`--rules` のルールファイルから `required` のキーの名前だけを読み、欠落の件数を数える(値や条件は U3)。`--rules` がなければ「ルールファイルがないため調べていない」と示す
B. 必須タグの欠落は U3 で入れる(AC1.1.7 は U3 で満たす)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- 一度もタグが付いていないリソースの件数(Q1): 上限を設けず、最後まで読みます。要件 OQ5 はこれで決まりです。
- 略語の辞書(Q2): U2 で、よく使う略語の小さな辞書(env⇔environment、prod⇔production、dev⇔development、stg⇔staging など)を入れ、scan の表記ゆれのまとまりに使います。U3 の `rules suggest` も同じ辞書を使います。
- 必須タグの欠落(Q3): U2 の scan が、`--rules` のルールファイルから `required` のキーの名前だけを読み、欠落の件数を数えます。値や条件は U3 で扱います。`--rules` がなければ「ルールファイルがないため調べていない」と示します。

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
