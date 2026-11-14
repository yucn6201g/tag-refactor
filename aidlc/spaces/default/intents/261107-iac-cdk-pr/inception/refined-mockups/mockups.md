# 詳しいモックアップ: iac への CDK の反映とプルリクエストの作成

ラフなモックアップ(`ideation/rough-mockups/wireframes.md`、`user-flow.md`)を、要件(`inception/requirements-analysis/requirements.md`)とユーザーストーリー(`inception/user-stories/stories.md`)に合わせて詳しくしたものです。表示は、今の `iac` の実際の出力の形(手で直す一覧の「リソース、キー、場所、理由」、最後の「完了」と「終了コード」の行)に合わせています。

出典の書き方: RF-Q… はこのステージの `refined-mockups-questions.md` の回答、FR… / NFR… は要件、US… / AC… はストーリーと受け入れ条件、画面… はラフなモックアップの画面です。

## 0. 共通の決まり(前の作業から引き継ぐもの)

| 項目 | 決まり | 出典 |
|---|---|---|
| 出力先 | 結果(差分、手で直す一覧、プルリクエストの URL)は標準出力、進み具合と案内とエラーは標準エラー | 前の作業の表示の決まり |
| 端末でないとき | 進み具合は、書き換えずに1行ずつ追記する | 前の作業の表示の決まり |
| 色 | 補助にとどめ、意味は記号と文字で表す。`NO_COLOR` か端末でないときは色を出さない | 前の作業の表示の決まり、`team-practices.md` |
| 言語 | `--lang` で英語と日本語を選ぶ。以下の例は英語で、日本語の例を最後に置く | NFR10 |

## 1. 新しいオプション

| オプション | 意味 | 出典 |
|---|---|---|
| `--pr` | 書き換え(`--write` を含む)のあと、ブランチ、コミット、push、プルリクエストの作成まで行う | FR1.1、M-Q1 |
| `--cdk-app <フォルダー>` | 対象の CDK のアプリを絞り込む。繰り返して複数指定できる。`--dir` からの相対 | FR2.1、RF-Q1 |
| `--synth-timeout <時間>` | `cdk synth` のアプリごとの時間の上限(既定 `10m`) | FR2.5、RF-Q1 |
| `--pr-branch <名前>` | 作るブランチの名前(既定 `tag-refactor/iac-<YYYYMMDD-HHMMSS>`) | FR1.4、RF-Q1 |
| `--pr-title <文字>` | プルリクエストのタイトル | FR1.4、RF-Q1 |
| `--pr-body-file <ファイル>` | プルリクエストの本文をファイルから読む(手で直す一覧は、その後ろに足す) | FR1.4、FR1.10、RF-Q1 |

`--pr-branch`、`--pr-title`、`--pr-body-file` は `--pr` なしでは誤り(終了コード 1)です。`--cdk-app` と `--synth-timeout` は `--pr` がなくても使えます。

## 2. 終了コード(今の `iac` に足すもの)

| コード | 意味 | 当てはまる場合 | 出典 |
|---|---|---|---|
| 0 | 手で直すものがない | 書き換えた(`--pr` ならプルリクエストを作った)、または反映するものがない | FR1.11、FR1.13 |
| 1 | エラー | 入力の誤り、書き換えの前の確かめの失敗、書き出しの失敗、ブランチ、コミット、push、作成の途中の失敗 | FR1.5、FR1.12、FR1.13 |
| 2 | 手で直すものがある | プルリクエストを作ったが手で直すものが残った、手で直すものだけが残った、一部の CDK のアプリが失敗した | FR1.13、FR2.9、US-Q5 |

## 画面1: `cdk synth` の進み具合(標準エラー)

端末のとき(同じ行を書き換える):

```
$ tag-refactor iac --plan plan.json --dir infra
Synthesizing 2 CDK apps (no AWS credentials are passed to cdk)
  [done]     app-ts   TypeScript   00:12
  [running]  app-py   Python       00:08
```

端末でないとき(1行ずつ追記する):

```
synth: app-ts started (TypeScript)
synth: app-ts done in 00:12
synth: app-py started (Python)
synth: app-py failed in 00:03: python was not found
```

- 状態は `[waiting]`、`[running]`、`[done]`、`[failed]` の文字で示す(FR2.7、AC2.1.2)。
- アプリの名前は `--dir` からの相対のフォルダー。言語は `cdk.json` の `app` の命令から決める。

## 画面2: 差分だけを見る(`--write` も `--pr` もなし)

```
$ tag-refactor iac --plan plan.json --dir infra
Synthesizing 2 CDK apps (no AWS credentials are passed to cdk)
  [done]     app-ts   TypeScript   00:12
  [done]     app-py   Python       00:09
--- app-ts/lib/orders-stack.ts
+++ app-ts/lib/orders-stack.ts
@@ -18,5 +18,5 @@
     const queue = new sqs.Queue(this, 'Orders');
-    cdk.Tags.of(queue).add('env', 'prod');
+    cdk.Tags.of(queue).add('Environment', 'prod');
     cdk.Tags.of(queue).add('Owner', OWNER);
 
--- app-py/app.py
+++ app-py/app.py
@@ -4,3 +4,3 @@
-TAGS = {"owner": "team-a"}
+TAGS = {"Owner": "team-a"}

To fix by hand (2):
  arn:aws:s3:::billing-bucket  CostCenter  app-ts/lib/billing-stack.ts:41  the value is an expression (props.env + "-billing")
  arn:aws:sqs:ap-northeast-1:111122223333:audit  Team  app-ts/lib/audit-stack.ts:12  the tag is set on the whole stack, and the plan does not change every resource in it the same way
Done in 00:24 · 5 resources · 2 files
Exit code: 2 (some changes to fix by hand)
```

- 差分の形は今の `iac` と同じ(変える行と前後3行)(FR3.9、AC3.1.5、AC3.2.5)。
- 手で直す一覧の行は「リソース、キー、場所(ファイル:行)、理由」(今の `iac` と同じ形、FR3.9)。
- 2行目は、スタック全体に付けたタグを、変更案がそのスタックの一部のリソースしか変えないときの例(RF-Q3)。

## 画面3: プルリクエストを作る(`--pr`)

```
$ tag-refactor iac --plan plan.json --dir infra --pr
Synthesizing 2 CDK apps (no AWS credentials are passed to cdk)
  [done]     app-ts   TypeScript   00:12
  [done]     app-py   Python       00:09
Checked: gh is logged in · remote origin is on GitHub · base branch main exists on origin
Wrote 4 files (2 CDK, 1 CloudFormation, 1 Terraform)
Created branch   tag-refactor/iac-20261108-103015
Committed        4 files
Pushed           origin tag-refactor/iac-20261108-103015
Opened pull request
https://github.com/example/infra/pull/42
1 change is left to fix by hand; it is listed in the pull request.
Done in 00:31 · 5 resources · 4 files
Exit code: 2 (some changes to fix by hand)
```

- 確認の問いは出さない(FR1.9)。書き換えの前の確かめを1行で示す(FR1.5)。
- URL は1行に単独で置く(画面3のラフ)。
- 差分の表示は、`--pr` のときは出さない(プルリクエストの差分で見られるため)。手で直す一覧は、件数の行だけを出し、中身は本文に載せる(FR1.10)。[assumption]

## 画面4: プルリクエストの本文(`--lang en`)

```
Title: Reflect tag changes from tag-refactor (plan.json)

This pull request was opened by `tag-refactor iac --pr`.
It changes the tags in the infrastructure code to match the plan,
so that the next deploy does not put the old tags back.

## Changed files (4)

- `app-ts/lib/orders-stack.ts` (CDK, TypeScript)
- `app-py/app.py` (CDK, Python)
- `cfn/orders.yaml` (CloudFormation)
- `main.tf` (Terraform)

## To fix by hand (1)

- `app-ts/lib/billing-stack.ts:41` BillingBucket, CostCenter: the value is an expression
```

- 本文に載せるのは、変更したファイルと手で直す一覧だけ。ARN とアカウント ID は載せず、リソースは論理 ID(CDK はコンストラクトの名前)で示す(RF-Q2)。
- `--pr-body-file` を付けたときは、そのファイルの中身のあとに「To fix by hand」の節を足す。

## 画面5: 書き換えの前の確かめで止まる(エラー)

```
$ tag-refactor iac --plan plan.json --dir infra --pr
error: gh (GitHub CLI) was not found.
Install it from https://cli.github.com/ and run "gh auth login", then run iac again.
No file was changed.
Exit code: 1 (error)
```

ほかの理由の文(1文目)の例:

| 場合 | 1文目 | 出典 |
|---|---|---|
| `gh` にログインしていない | `gh is not logged in to github.com.` | FR1.5 |
| リモートが決まらない | `no remote to push to: the current branch tracks none, and there is no "origin".` | FR1.5、US-Q4 |
| リモートが GitHub でない | `remote "origin" is not on GitHub (gitlab.example.com).` | FR1.5 |
| HEAD がブランチを指していない | `HEAD is not on a branch; check out the branch the pull request should go to.` | FR1.5 |
| 行き先のブランチがリモートに無い | `branch "feature-x" is not on remote "origin"; push it first.` | FR1.5、US-Q4 |
| 同じ名前のブランチがある | `branch "tag-refactor/iac-20261108-103015" already exists on remote "origin"; choose another with --pr-branch.` | FR1.5 |
| 書き換えるファイルに変更がある | 今の `--write` と同じ文 | FR1.6 |

いずれも「No file was changed.」で終わり、終了コードは 1(AC1.1.4)。

## 画面6: 途中で失敗する(エラー)

```
$ tag-refactor iac --plan plan.json --dir infra --pr
...
Wrote 4 files (2 CDK, 1 CloudFormation, 1 Terraform)
Created branch   tag-refactor/iac-20261108-103015
Committed        4 files
Pushed           origin tag-refactor/iac-20261108-103015
error: gh pr create failed: HTTP 422 (a pull request already exists for this branch?)
Done so far: 4 files written, branch created and committed, branch pushed to origin.
To finish by hand, run:
  gh pr create --base main --head tag-refactor/iac-20261108-103015
Nothing was undone.
Exit code: 1 (error)
```

- どこまで済んだかを1行にまとめ、続きの命令を示す。済んだことは元に戻さない(FR1.12、AC1.1.9)。
- コミットで失敗したとき(例: フックが失敗した)は「Done so far: 4 files written, branch created.」になり、続きは `git commit` から示す。

## 画面7: 変えるものがないとき、手で直すものだけが残るとき

```
$ tag-refactor iac --plan plan.json --dir infra --pr
Nothing to reflect: the sources already match the plan.
No pull request was opened.
Exit code: 0 (nothing to fix by hand)
```

```
$ tag-refactor iac --plan plan.json --dir infra --pr
Nothing to write, but some changes must be fixed by hand.
No pull request was opened.
To fix by hand (1):
  arn:aws:s3:::billing-bucket  CostCenter  app-ts/lib/billing-stack.ts:41  the value is an expression (props.env + "-billing")
Exit code: 2 (some changes to fix by hand)
```

(FR1.11、AC1.1.7、AC1.1.8、US-Q5)

## 画面8: 一部のアプリが失敗する

```
$ tag-refactor iac --plan plan.json --dir infra
Synthesizing 2 CDK apps (no AWS credentials are passed to cdk)
  [done]     app-ts   TypeScript   00:12
  [failed]   app-py   Python       00:03
synth of app-py failed: python was not found. Install Python 3 and run iac again.
--- app-ts/lib/orders-stack.ts
...
To fix by hand (1):
  arn:aws:dynamodb:ap-northeast-1:111122223333:table/Users  Owner  app-py  the CDK app could not be synthesized (python was not found)
Exit code: 2 (some changes to fix by hand)
```

- 失敗したアプリのリソースは、場所にアプリのフォルダーを出し、理由に失敗の理由を書く(FR2.8、FR2.9、AC4.1.3)。
- AWS を呼ばないと synth できないときの理由: `the app needs AWS to look up values; run "cdk synth" there once with your credentials so cdk.context.json is saved`(FR2.4、AC2.1.3)。
- 時間切れの理由: `cdk synth did not finish within 10m (--synth-timeout)`(FR2.5)。
- CDK v1 の理由: `this is a CDK v1 app; only CDK v2 is supported`(FR2.10)。

## 日本語の表示の例(画面3の一部)

```
CDK のアプリ 2つを synth しています(cdk には AWS の認証情報を渡しません)
  [済み]     app-ts   TypeScript   00:12
  [済み]     app-py   Python       00:09
確かめました: gh にログイン済み · リモート origin は GitHub · 行き先のブランチ main は origin にあります
4つのファイルを書き換えました(CDK 2、CloudFormation 1、Terraform 1)
ブランチを作りました    tag-refactor/iac-20261108-103015
コミットしました        4つのファイル
push しました           origin tag-refactor/iac-20261108-103015
プルリクエストを作りました
https://github.com/example/infra/pull/42
手で直すものが1件あります。プルリクエストに載せました。
完了 00:31 · 5 リソース · 4 ファイル
終了コード: 2(手で直すものがあります)
```

## Assumptions & Open Questions

- `--pr` のときに端末に差分を出さず、手で直す一覧も件数だけにする(画面3)。プルリクエストで見られるため。[assumption]
- 文言とブランチ名の既定の形は、実装で細かく変わることがある。[assumption]
