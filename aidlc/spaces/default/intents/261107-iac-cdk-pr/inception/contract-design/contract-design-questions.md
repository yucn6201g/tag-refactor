# 契約の設計の質問: iac への CDK の反映とプルリクエストの作成

作業の単位(`unit-of-work.md`、`unit-of-work-dependency.md`)、部品(`components.md`)、要件(`requirements.md`)をもとに、単位どうしの境目の決まりを聞きます。この作業の境目は、1つのバイナリの中の Go の関数と型、Go と埋め込んだ補助のプログラムの間の JSON、外のプログラム(`cdk`、`git`、`gh`)の呼び出し、利用者が触るコマンドの4つです。前の作業の契約(変更案の JSON、進み具合の通知口など)は変えずに引き継ぎます。前の段階のレビューで残った指摘(補助のプログラムで書き換え前に確かめる、認証情報の遮断の経路、進み具合の渡し方)も、ここで決めます。

## Q1. Go と補助のプログラムの間の受け渡し

A. 補助のプログラムは、標準入力で JSON の要求を受け、標準出力に JSON の結果を返す。CDK のアプリごとに1回起動し、そのアプリのファイルをまとめて読む。書き換え後の確かめは、書き換え後の内容を要求に入れて渡し、ディスクに書く前に読み直す
B. ファイルごとに1回起動する(要求と結果は A と同じ形)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q2. ソースの中の範囲の表し方

A. UTF-8 のバイトの位置(始まりと終わり)。今の `iac` の範囲の書き換えと同じ。補助のプログラムが、言語ごとの位置(TypeScript の UTF-16 の位置など)からバイトの位置に直して返す
B. 行と桁
C. 任せる
X. Other (please specify)

[Answer]: A

## Q3. 補助のプログラムの JSON の版

A. 要求と結果に版の番号を入れ、違えば止める。補助のプログラムはバイナリに埋め込むので、ふつうは同じ版がそろう
B. 版を持たない
C. 任せる
X. Other (please specify)

[Answer]: A

## Q4. `cdk synth` と補助のプログラムに渡す環境(AWS と GitHub の認証情報の遮断)

ドメインの設計のレビューで、環境変数を外すだけでは、`~/.aws` の既定の設定やメタデータから認証情報が読まれうる、と指摘されました。

A. 親の環境から `AWS_` で始まる変数と `GH_TOKEN`、`GITHUB_TOKEN` などを外し、`AWS_SHARED_CREDENTIALS_FILE` と `AWS_CONFIG_FILE` を空の一時ファイルに向け、`AWS_EC2_METADATA_DISABLED=true` を足す。最小の一本(U1)から入れる
B. 環境変数を外すだけにする
C. 任せる
X. Other (please specify)

[Answer]: A

## Q5. 実行中の進み具合の渡し方

前の作業では、`internal/cli` が表示をもとに進み具合の通知口(`ProgressSink`)を作り、ほかの部品に渡しています。

A. 今の `ProgressSink` に、CDK のアプリの状態(始まり、済み、失敗)と、プルリクエストの段階(済み、失敗)の知らせを足す
B. CDK とプルリクエスト用に、別の通知口を作る
C. 任せる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- Go と補助のプログラムは、標準入力の JSON の要求と標準出力の JSON の結果で受け渡す。CDK のアプリごとに1回起動してまとめて読む。書き換え後の確かめは、書き換え後の内容を要求に入れて渡し、ディスクに書く前に読み直す(Q1: A)
- ソースの中の範囲は UTF-8 のバイトの位置で表す。補助のプログラムが言語ごとの位置から直して返す(Q2: A)
- 補助のプログラムの JSON の要求と結果に版の番号を入れ、違えば止める(Q3: A)
- `cdk synth` と補助のプログラムに渡す環境では、`AWS_` で始まる変数と `GH_TOKEN`、`GITHUB_TOKEN` などを外し、`AWS_SHARED_CREDENTIALS_FILE` と `AWS_CONFIG_FILE` を空の一時ファイルに向け、`AWS_EC2_METADATA_DISABLED=true` を足す。最小の一本(U1)から入れる(Q4: A)
- 実行中の進み具合は、今の `ProgressSink` に、CDK のアプリの状態とプルリクエストの段階の知らせを足して渡す(Q5: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
