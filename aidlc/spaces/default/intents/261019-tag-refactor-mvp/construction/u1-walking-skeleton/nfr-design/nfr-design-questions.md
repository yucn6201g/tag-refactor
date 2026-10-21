# 非機能の設計 — 質問(U1 最小の一本)

U1 の非機能要件(`construction/u1-walking-skeleton/nfr-requirements/`)を、どう作るかを決めます。tag-refactor は利用者の端末で動く CLI なので、サーバー向けの仕組み(負荷分散、キャッシュの層、サーキットブレーカーなど)は使いません。

決まっていること(質問しません):

- 1回の AWS の呼び出しの時間の上限は30秒(NFR5.1)。
- AWS から読んだ文字列は、表示の前に制御文字を見える形に置き換える(NFR8.6)。
- 認証情報は AWS の SDK が標準の設定から読み、ツールは持たない(NFR8.3)。SDK のログは有効にしない(NFR8.8)。
- 使う技術: Go、AWS の Go 用 SDK(v2)、cobra、goccy/go-yaml、`CGO_ENABLED=0`(`tech-stack-decisions.md`)。

## Q1. 部品の分け方(Go のパッケージ)

ドメイン設計の部品を、Go のパッケージにどう分けますか?

A. 部品ごとに1つのパッケージにする(入口は `cmd/tag-refactor`、部品は `internal/` の下に `cli`、`display`、`awsaccess`、`inventory`、`rules`、`planner`、`impact`)。部品の境目(契約のインターフェース)がパッケージの境目と一致する
B. 少ないパッケージにまとめる(例: `internal/app` と `internal/aws` の2つ)
X. Other (please specify)

[Answer]: A

## Q2. U1 の再試行

U1 の AWS の呼び出しの再試行は、どうしますか?(待ち時間を延ばす本格的な再試行と、呼び出し回数の上限への対応は U2)

A. AWS の SDK の標準の再試行(標準のモード、既定の回数)をそのまま使う
B. U1 では再試行しない(1回失敗したら Transient)
X. Other (please specify)

[Answer]: A

## Q3. `--debug` の表示の形

`--debug` の表示を、どの形にしますか?

A. 人が読む1行の形(例: `debug: tag:GetResources us-east-1 132ms retries=0`)
B. 1行に1つの JSON(機械で集計しやすい)
X. Other (please specify)

[Answer]: A

## Q4. 検証用アカウントを使うテストの動かし方

実際の検証用 AWS アカウントを呼ぶテスト(手元だけで動かす)を、どう動かしますか?

A. Go のテストとして書き、ビルドタグ(`integration`)と、プロファイル名を入れる環境変数(例: `TAG_REFACTOR_IT_PROFILE`)があるときだけ動かす。CI では動かない
B. テストとは別のスクリプトにする(作った実行ファイルを動かして、出力を確かめる)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- 部品の分け方(Q1): 部品ごとに1つの Go のパッケージにします。入口は `cmd/tag-refactor`、部品は `internal/` の下の `cli`、`display`、`awsaccess`、`inventory`、`rules`、`planner`、`impact` です。契約のインターフェースとパッケージの境目をそろえます。
- 再試行(Q2): AWS の SDK の標準の再試行(標準のモード、既定の回数)をそのまま使います。使い切ったら Transient です。
- `--debug` の形(Q3): 人が読む1行の形にします(例: `debug: tag:GetResources us-east-1 132ms retries=0`)。
- 検証用アカウントのテスト(Q4): Go のテストとして書きます。ビルドタグ `integration` と、プロファイル名の環境変数があるときだけ動き、CI では動きません。

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
