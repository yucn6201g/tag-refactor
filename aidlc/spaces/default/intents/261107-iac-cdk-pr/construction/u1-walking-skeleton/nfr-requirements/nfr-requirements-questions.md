# 非機能の要件の質問: U1 最小の一本

U1 の非機能の要件は、要件の NFR1〜NFR10(`inception/requirements-analysis/requirements.md`)と、契約の時間の上限と環境の決まり(`inception/contract-design/contract-summary.md` の C5)でほぼ決まっています。U1 の機能の設計は `construction/u1-walking-skeleton/functional-design/` にあります。ここでは、まだ数字や決まりが無いものだけを聞きます。

## Q1. 補助のプログラムの速さの目安

synth の上限は 10 分、補助のプログラムの上限は呼び出しごとに 2 分と決まっています。

A. 上限とは別に目安を置く。TypeScript のファイルが 200 個までのアプリで、補助のプログラムの読み取り(analyze)と読み直し(recheck)が、それぞれ CI の機械で 30 秒以内に終わること。テスト用のアプリでの時間を、ベンチマークのテストで記録する
B. 上限だけにし、目安は置かない
C. 任せる
X. Other (please specify)

[Answer]: A

## Q2. 埋め込んだ補助のプログラムを書き出すときの守り

A. 自分だけが読み書きできる一時的なフォルダー(os.MkdirTemp)に、自分だけが読み書きできるファイルとして書く。実行の前に、書いた中身のハッシュが埋め込んだものと同じことを確かめ、実行のあとで消す
B. 一時的なフォルダーに書くだけにする
C. 任せる
X. Other (please specify)

[Answer]: A

## Q3. `--debug` で出すもの

A. 動かした外のプログラムの名前と引数(トークンを含まないもの)、渡した環境の変数の名前(値は出さない)、段階ごとの経過時間、synth と補助のプログラムの標準エラー(ソースの中身と認証情報の形の文字列は伏せる)を出す。トークンの値は出さない
B. 今の `--debug` と同じ程度(git の結果と、確かめに失敗した理由だけ)にする
C. 任せる
X. Other (please specify)

[Answer]: A

## Q4. 対応する Node.js と TypeScript の版

A. Node.js は 18 以上。TypeScript はアプリが持つ版を使い、4.0 以上 5.x までを対象にする。補助のプログラムは、その範囲で変わらない API(ソースの読み取りと木のたどり方)だけを使う。対象の外の版なら、アプリを失敗にして理由を示す
B. 版を決めず、動けばよいとする
C. 任せる
X. Other (please specify)

[Answer]: A

## Q5. CI の時間の増え方

A. CDK のテストで増える時間は、OS ごとに 10 分以内を目安にする。npm と Go のキャッシュを使う
B. 目安を置かない
C. 任せる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- 補助のプログラムの目安: TypeScript のファイルが 200 個までのアプリで、読み取りと読み直しがそれぞれ CI の機械で 30 秒以内。ベンチマークのテストで記録する(Q1: A)
- 補助のプログラムは、自分だけが読み書きできる一時的なフォルダーとファイルに書き、実行の前にハッシュを確かめ、実行のあとで消す(Q2: A)
- `--debug` には、外のプログラムと引数、環境の変数の名前(値は出さない)、段階ごとの経過時間、synth と補助のプログラムの標準エラー(ソースの中身と認証情報の形は伏せる)を出す(Q3: A)
- Node.js は 18 以上、TypeScript はアプリの版で 4.0 以上 5.x まで。範囲の外ならアプリを失敗にして理由を示す(Q4: A)
- CDK のテストで増える CI の時間は OS ごとに 10 分以内を目安にし、npm と Go のキャッシュを使う(Q5: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
