# ワイヤーフレーム(ラフ): iac への CDK の反映とプルリクエストの作成

## 前提

- tag-refactor はコマンドラインのツールなので、ここでの「画面」は端末に出す文字の表示です。目的は `intent-statement.md`、範囲は `scope-document.md` と `intent-backlog.md` のとおりです。
- 出典の書き方: M-Q… はこのステージの `rough-mockups-questions.md` の回答、IB-… は `intent-backlog.md` の項目です。
- 前の intent で決めた共通の決まり(`--lang` で英語と日本語を選ぶ、色だけで意味を伝えない、進み具合は標準エラー、結果は標準出力)を引き継ぎます。
- 下の例は英語の表示です(`--lang ja` なら日本語)。文言、オプションの名前、数は仮のものです。

## 共通の決まりごと(今回足すもの)

| 決まり | 内容 | 出典 |
|---|---|---|
| プルリクエストの指定 | `iac --pr`。`--pr` は `--write` を含む | M-Q1 |
| CDK のアプリの見つけ方 | `--dir` の下の `cdk.json` を探す。新しいオプションで絞り込める | M-Q2 |
| synth の進み具合 | 標準エラーに、アプリごとの状態と経過時間 | M-Q3 |
| 作る前の確認 | しない。作ったら URL を出す | M-Q4 |
| 手で直すものが残るとき | プルリクエストは作り、本文に「手で直す一覧」を載せる | M-Q5 |
| タイトルと本文の言語 | `--lang` に従う | M-Q6 |

## 画面1: CDK の synth の進み具合(標準エラー)

```
$ tag-refactor iac --plan plan.json --dir infra
synthesizing 2 CDK apps (no AWS credentials are passed to cdk)
  done     infra/app-ts      (TypeScript)   12.4s
  running  infra/app-py      (Python)        8.1s
```

- 状態は文字(`waiting`、`running`、`done`、`failed`)で出し、色は補助にとどめる(M-Q3)。
- アクセシビリティ: 見出しや画面の領域はない。1行に1つのアプリを出し、読み上げで順に追える。入力の受け付けはない。

## 画面2: 差分だけを見る(`--write` も `--pr` もなし)

```
$ tag-refactor iac --plan plan.json --dir infra
--- infra/app-ts/lib/orders-stack.ts
+++ infra/app-ts/lib/orders-stack.ts
@@ -18,3 +18,3 @@
     const queue = new sqs.Queue(this, 'Orders');
-    cdk.Tags.of(queue).add('env', 'prod');
+    cdk.Tags.of(queue).add('Environment', 'prod');
 
--- infra/app-py/app.py
+++ infra/app-py/app.py
@@ -5,3 +5,3 @@
-TAGS = {"owner": "team-a"}
+TAGS = {"Owner": "team-a"}

Summary: 2 files, 2 changes reflected, 0 already reflected, 1 to fix by hand
To fix by hand:
  infra/app-ts/lib/billing-stack.ts:41  Bucket Billing  value is an expression (props.env + "-billing")
```

- 前の intent の差分の形(変える行と前後3行)を CDK のソースにも使う(IB-05、IB-06)。
- 手で直す一覧の行は、ファイル、行、リソース、理由の順(IB-07)。
- アクセシビリティ: 差分の行は `-` と `+` の記号で区別し、色だけに頼らない。

## 画面3: プルリクエストを作る(`--pr`)

```
$ tag-refactor iac --plan plan.json --dir infra --pr
synthesizing 2 CDK apps (no AWS credentials are passed to cdk)
  done     infra/app-ts      (TypeScript)   12.4s
  done     infra/app-py      (Python)        9.0s
Wrote 4 files (2 CDK, 1 CloudFormation, 1 Terraform)
Created branch  tag-refactor/plan-20261107-1530
Committed       4 files
Pushed          origin tag-refactor/plan-20261107-1530
Opened pull request  https://github.com/example/infra/pull/42
  1 change is left to fix by hand; it is listed in the pull request.
```

- 確認の問いは出さない。作ったら URL を出す(M-Q4)。
- 手で直すものが残っていても作り、その件数を最後に出す(M-Q5)。終了コードは今の `iac` と同じく 2 にするかは要件で決める。[assumption]
- ブランチ名の形は仮(IB-02)。[assumption]
- アクセシビリティ: 各行の先頭に、何をしたかを動詞で書く。URL は1行に単独で置き、選んで写しやすくする。

## 画面4: プルリクエストの本文(`--lang en` の例)

```
Title: Reflect tag changes from tag-refactor plan (plan.json)

This pull request was opened by tag-refactor iac --pr.
It changes the tags in the infrastructure code to match the plan, so that
the next deploy does not put the old tags back.

Changed files (4):
  - infra/app-ts/lib/orders-stack.ts (CDK, TypeScript)
  - infra/app-py/app.py (CDK, Python)
  - infra/cfn/orders.yaml (CloudFormation)
  - infra/main.tf (Terraform)

To fix by hand (1):
  - infra/app-ts/lib/billing-stack.ts:41 Bucket Billing: value is an expression
```

- `--lang ja` なら日本語で書く(M-Q6)。
- 本文に「手で直す一覧」を載せる(M-Q5)。
- 本文に入れる情報の詳しい範囲(変更案のファイル名、タグの変更の表など)は要件で決める。[assumption]
- アクセシビリティ: GitHub の Markdown の見出しと箇条書きだけを使い、表の幅に頼らない。

## 画面5: 足りないものがあるとき(エラー)

```
$ tag-refactor iac --plan plan.json --dir infra --pr
error: gh (GitHub CLI) was not found. Install it from https://cli.github.com/ and run "gh auth login".
No file was changed.
```

```
$ tag-refactor iac --plan plan.json --dir infra
synthesizing 1 CDK app (no AWS credentials are passed to cdk)
  failed   infra/app-ts      (TypeScript)    3.2s
error: cdk synth of infra/app-ts needs AWS to look up values (Vpc.fromLookup).
Run "cdk synth" there once with your credentials so cdk.context.json is saved, then run iac again.
```

- 何が足りないかと、次にすることを示す(IB-03、IB-04)。認証情報の値は出さない。
- `gh` が無いとき、ファイルを書き換える前に止めるか、書き換えたあとで止めるかは要件で決める。[assumption]
- アクセシビリティ: `error:` で始め、1つの文で原因、次の文で対処を書く。

## 状態ごとの表示

| 状態 | 表示 | 出典 |
|---|---|---|
| 空(変えるものがない) | 「反映するものはありません」と出し、プルリクエストは作らない [assumption] | IB-01 |
| 読み込み中 | 画面1の synth の進み具合 | M-Q3 |
| 成功 | 画面2(差分)または画面3(URL) | M-Q4 |
| エラー | 画面5 | IB-03、IB-04 |
| 一部だけ | 画面2と画面3の「手で直す一覧」。プルリクエストは作る | M-Q5、IB-07 |

## Assumptions & Open Questions

- 文言、オプションの名前、ブランチ名の形は仮のもの。[assumption]
- 手で直すものが残ってプルリクエストを作ったときの終了コード、`gh` が無いときに書き換えの前に止めるか、変えるものがないときにプルリクエストを作らないことは、要件で決める。[assumption]
