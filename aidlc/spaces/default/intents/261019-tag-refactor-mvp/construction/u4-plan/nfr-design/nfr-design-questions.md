# 非機能の設計 — 質問(U4 変更案)

U4 の非機能要件(`construction/u4-plan/nfr-requirements/`)を、どう作るかを決めます。U1〜U3 の設計は、ここで変えない限りそのまま使います。

決まっていること(質問しません):

- 新しい依存は足さない(`tech-stack-decisions.md`)。
- 保存する変更案(契約 C2)と、JSON の出力の plan の中身(契約 C3)は同じ形なので、JSON の型は `planner` に1組だけ作り、`display` はそれを使って C3 を書く。保存と読み込みは `planner` が受け持つ。
- 4つの形式、文言の一覧(英語と日本語)、言語の決め方、色の判定、Markdown のエスケープは `display` に置く。`cli` は `--format`、`--lang`、`--out` を受け付けて、`display` と `planner` に渡すだけにする。
- JSON の後処理(DEL、C1 の制御文字、書式の文字を `\uXXXX` にする)は、`encoding/json` の出力のバイト列のうち、その文字だけを置き換える。JSON の構文の文字はすべて ASCII なので、その文字が現れるのは文字列の中だけになる。
- Windows の仮想端末の処理は、U2 の関数を「指定したストリームについて有効にし、元に戻す関数を返す」形に広げ、標準出力と標準エラーのそれぞれに使う。
- 保存した変更案の読み込みのファジングは `planner` に置き、種の入力は `internal/planner/testdata/fuzz/` に置く。

## Q1. Ctrl+C で止めたとき

今のツールは Ctrl+C を受け取る仕組みがなく、すぐに終わります。そのため、`--out` や `rules suggest --output` の一時ファイルが残りえます(NFR8.17)。

A. 入口で Ctrl+C(Unix では SIGTERM も)を受け取り、処理中の AWS の呼び出しを取り消して、後片付け(一時ファイルの削除、Windows の端末の設定を戻す)をしてから終わる。終了コードは 1(契約 C5 の決まりの中で「エラー」とする)、標準エラーに「Interrupted. Nothing was saved.」を出す。2回目の Ctrl+C では、後片付けを待たずにすぐ終わる
B. A と同じだが、終了コードはシェルの慣習の 130 にする(契約 C5 に 130 を足す)
X. Other (please specify)

[Answer]: A

## Q2. `--out` の書き込みの仕方

保存する変更案が上限(256 MiB)を超えるときは保存しません(NFR8.18)。

A. 一時ファイルに少しずつ書きながら大きさを数え、上限を超えたところで書くのをやめて一時ファイルを消す(メモリは小さいまま)
B. 変更案をいったんメモリの中で JSON にし、大きさを確かめてから一時ファイルに書く(作りは簡単だが、最大で 256 MiB ほどのメモリを使う)
X. Other (please specify)

[Answer]: A

## Q3. 日本語を含む表の桁の揃え方

一覧と差分の形は、列を空白で揃えています(例 `~ rename key   env -> Environment   2,950 resources`)。今の揃え方は文字の数で数えるため、日本語の文言(`キー名の変更`)や日本語のキーと値では、端末で列がずれます(日本語の文字は端末で2文字分の幅になるため)。

A. 端末での幅で揃える。幅は、Unicode の東アジアの文字幅(W と F は2、それ以外は1、結合文字は0)の表を、ツールの中に小さく持って求める(新しい依存なし)。曖昧な幅(A)の文字は1とする
B. `golang.org/x/text/width` を使って幅を求める(新しい依存が増える)
C. 揃えない。日本語の表示では列の揃えをやめ、項目の区切りを `  ` にする
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- Ctrl+C(Q1): 入口で Ctrl+C(Unix では SIGTERM も)を受け取り、処理中の AWS の呼び出しを取り消し、一時ファイルを消して Windows の端末の設定を戻してから、終了コード1で終わります。標準エラーには「Interrupted. Nothing was saved.」を出します。2回目の Ctrl+C では、すぐに終わります。U3 の `rules suggest --output` も同じ仕組みで守られます。
- `--out` の書き込み(Q2): 一時ファイルに少しずつ書きながら大きさを数え、256 MiB を超えたら書くのをやめて一時ファイルを消します。
- 表の桁の揃え方(Q3): 端末での幅で揃えます。幅は、Unicode の東アジアの文字幅の小さな表をツールの中に持って求めます(W と F は2、結合文字は0、曖昧な幅は1)。新しい依存は足しません。

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
