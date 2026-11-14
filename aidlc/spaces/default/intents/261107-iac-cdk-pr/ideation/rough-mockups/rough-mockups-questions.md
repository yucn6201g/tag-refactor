# ラフなモックアップの質問: iac への CDK の反映とプルリクエストの作成

tag-refactor はコマンドラインのツールなので、ここでの「画面」は端末に出す文字の表示です。前の intent で決めた共通の決まり(`--lang` で英語と日本語を選ぶ、色だけで意味を伝えない、進み具合は標準エラーに出す、`apply` は実行前に確認する)は引き継ぎ、聞き直しません。

範囲は `scope-document.md` と `intent-backlog.md` のとおりです(プルリクエストは1回の実行で書き換えから作成まで行う、CDK は `cdk synth` を tag-refactor が実行する)。目的は `intent-statement.md` のとおりです。

## Q1. プルリクエストを作る指定のしかた

A. `iac` に `--pr` を足す。`--pr` を付けると、書き換え(`--write` と同じ)のあと、ブランチ、コミット、push、作成まで行う
B. 新しいサブコマンド(例: `tag-refactor pr`)を作り、その中で書き換えから作成まで行う
C. `iac --write` で書き換えたあと、別のサブコマンドでプルリクエストを作る(2回の実行に分ける)
D. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: A

## Q2. CDK のアプリの指定のしかた

A. `--dir` の下で `cdk.json` を探し、見つかったフォルダーを CDK のアプリとして扱う(2つ以上あれば、それぞれ `cdk synth` する)
B. CDK のアプリのフォルダーを、新しいオプション(例: `--cdk-app`)で明示する
C. A を基本にし、B のオプションで絞り込める
D. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: C

## Q3. `cdk synth` を動かしている間の表示

`cdk synth` は、アプリによっては数十秒以上かかります。

A. 標準エラーに、アプリごとの状態(待ち、実行中、済み、失敗)と経過時間を出す(`scan` のリージョンごとの表示と同じ形)
B. 何も出さず、終わってから結果だけを出す
C. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: A

## Q4. プルリクエストを作る前の確認

A. 確認しない(`--pr` を付けたことを意思表示とする)。作ったら URL を出す
B. 差分の要約を見せて「作りますか? [y/N]」と聞く。`--yes` で飛ばせる(`apply` と同じ形)
C. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: A

## Q5. 手で直すものが残っているときのプルリクエスト

自動で書き換えられないもの(「手で直す一覧」)があるとき、今の `iac` は終了コード 2 を返します。

A. 作る。プルリクエストの本文に「手で直す一覧」を載せ、レビューする人が残りを直せるようにする
B. 作らない。すべて反映できたときだけ作る
C. 既定では作らず、オプション(例: `--allow-manual`)を付けたときだけ作る
D. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: A

## Q6. プルリクエストのタイトルと本文の言語

A. `--lang` に従う(日本語なら日本語で書く)
B. 英語に固定する(チームの誰が読むか分からないため)
C. 既定は英語で、`--lang` を指定したときだけその言語にする
D. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- プルリクエストは `iac` に `--pr` を足して作る。`--pr` を付けると、書き換え(`--write` と同じ)のあと、ブランチ、コミット、push、作成まで行う(Q1: A)
- CDK のアプリは、`--dir` の下で `cdk.json` を探して見つける。新しいオプションで絞り込める(Q2: C)
- `cdk synth` の間は、標準エラーにアプリごとの状態と経過時間を出す(Q3: A)
- プルリクエストを作る前に確認はしない。作ったら URL を出す(Q4: A)
- 手で直すものが残っていてもプルリクエストを作り、本文に「手で直す一覧」を載せる(Q5: A)
- プルリクエストのタイトルと本文は `--lang` の言語で書く(Q6: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
