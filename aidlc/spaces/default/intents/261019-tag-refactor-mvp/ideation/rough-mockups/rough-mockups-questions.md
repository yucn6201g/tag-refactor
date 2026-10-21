# ラフなモックアップ — 質問

tag-refactor は画面を持たないコマンドラインのツールです。このステージでは、利用者が端末で見る出力の形と、操作の流れを決めます。

## Q1. 主な使い方

利用者は、このツールをどんな場面で実行しますか?

A. 手元の端末で、対話的に実行する
B. CI(GitHub Actions など)で自動実行し、終了コードやファイルで結果を受け取る
C. A と B の両方
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q2. コマンドの構成

機能(scan、plan、apply、rollback)をどう呼び出せるようにしますか?

A. 機能ごとのサブコマンドにする(例: `tag-refactor scan`、`tag-refactor plan`)
B. 1つのコマンドにして、オプションで切り替える
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q3. plan の表示形式

plan(変更案と影響)を、どの形で出せるようにしますか?(select all that apply)

A. 端末で読みやすい一覧(色つき)
B. Terraform の plan に似た差分の形(`+` 追加、`~` 変更、`-` 削除)
C. 機械が読める形式(JSON)
D. レビューの共有やプルリクエストのコメントに貼れる Markdown
E. Not yet defined
X. Other (please specify)

[Answer]: A, B, C, D

## Q4. apply の前の確認

apply は本番環境のタグを書き換えます。実行前の確認をどうしますか?

A. 実行時に変更の要約を表示し、`yes` と入力させる(CI 用に確認を省くオプションも用意する)
B. 事前に保存した plan のファイルを指定したときだけ実行できる(Terraform の `plan -out` と同じ考え方)
C. A と B の両方
D. Not yet defined
X. Other (please specify)

[Answer]: C

## Q5. 表示する言語

ツールのメッセージや出力は、どの言語で表示しますか?

A. 英語のみ
B. 英語と日本語(設定で切り替える)
C. 最初は英語のみで、後から多言語に対応できるようにしておく
D. Not yet defined
X. Other (please specify)

[Answer]: B

## Q6. 端末での見やすさと使いやすさ

端末での表示について、どれを満たす必要がありますか?(select all that apply)

A. 色が見えなくても意味が分かる(記号や文字を色と併用する)
B. `NO_COLOR` 環境変数が設定されていれば色を消す
C. 全リージョンを調べている間、進み具合を表示する
D. Not yet defined
X. Other (please specify)

[Answer]: A, B, C

## Q7. 対応するOS

ツールはどのOSで動けばよいですか?

A. Linux、macOS、Windows のすべて
B. Linux と macOS
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- 使い方(Q1): 手元の端末で、対話的に実行する
- コマンドの構成(Q2): 機能ごとのサブコマンドにする(例: `tag-refactor scan`、`tag-refactor plan`)
- plan の表示形式(Q3): 端末で読みやすい一覧(色つき)、Terraform の plan に似た差分の形、JSON、Markdown のすべて
- apply の前の確認(Q4): 実行時に要約を表示して `yes` を入力させる方法と、保存した plan のファイルを指定する方法の両方。確認を省くオプションも用意する
- 表示する言語(Q5): 英語と日本語(設定で切り替える)
- 見やすさ(Q6): 色が見えなくても意味が分かる、`NO_COLOR` に従う、全リージョンを調べている間の進み具合を表示する
- 対応するOS(Q7): Linux、macOS、Windows のすべて

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
