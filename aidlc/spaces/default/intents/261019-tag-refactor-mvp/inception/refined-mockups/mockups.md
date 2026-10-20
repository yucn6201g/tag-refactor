# 詳細なモックアップ: tag-refactor

ラフなモックアップ(`ideation/rough-mockups/wireframes.md`)を、要件定義(`inception/requirements-analysis/requirements.md`)とユーザーストーリー(`inception/user-stories/stories.md`)に合わせて詳しくしたものです。

出典の書き方: 「RM-Q…」はこのステージの `refined-mockups-questions.md` の回答、「FR…」「NFR…」は要件、「US…」「AC…」はストーリーと受け入れ条件です。

## 0. 共通の決まり

| 項目 | 決まり | 出典 |
|---|---|---|
| 出力先 | 結果(一覧、差分、JSON、Markdown)は標準出力、進み具合・案内・警告・エラーは標準エラー | FR7.5 |
| 端末でないとき | 進み具合は、書き換えずに1行ずつ追記する簡易な表示にする | RM-Q6 |
| 件数の多い結果 | 既定で全件を表示する | RM-Q3 |
| 色 | 補助にとどめ、意味は記号と文字で表す。`NO_COLOR` があれば色を出さない | FR7.3 |
| 言語 | OS の言語設定が日本語なら日本語、それ以外は英語。設定で切り替えられる。以下の例は英語で、日本語の例を最後に置く | FR7.2 |
| 記号 | `+` 追加、`~` 変更、`-` 削除、`!` 影響あり、`?` 判定できない、`>` 飛ばした、`x` 失敗 | 共通 |

## 1. 終了コード

| コード | 意味 | 当てはまる場合 | 出典 |
|---|---|---|---|
| 0 | 問題なし | 処理がすべて終わり、問題も影響も見つからなかった | FR7.4 |
| 1 | エラー | 処理を始められなかった(認証情報なし、ルールファイルの誤り、アカウント不一致など)。結果が不完全(一部のリージョンやポリシーを読めなかった)。apply や rollback で一部が失敗した | FR7.4、RM-Q5 |
| 2 | 要確認 | 問題(表記ゆれ、必須タグの欠落)や影響(`!` `?`)が見つかった。apply や rollback で飛ばしたリソースがあった。apply で影響のある変更があり、確認を省くオプションだけでは進めなかった | FR7.4、FR5.5、RM-Q5 |

**例外:** AWS Config も Resource Explorer も使えず、一度もタグが付いたことのないリソースを探せなかったことだけでは、1 にしない(RM-Q8、FR1.3)。その場合は、他の結果によって 0 か 2 になる。

**複数が当てはまるとき:** 1 を最優先し、次に 2、最後に 0 とする(案)。

## 画面1: scan(実行中、端末のとき)

```
$ tag-refactor scan --rules tagrules.yaml
Account 123456789012 · 17 regions

  [done]     us-east-1        2,431 resources
  [done]     ap-northeast-1   1,870 resources
  [running]  eu-west-1          612 resources so far
  [waiting]  sa-east-1
  ...
  9/17 regions · elapsed 01:12
```

端末でないとき(RM-Q6):

```
scan: us-east-1 done (2,431 resources)
scan: ap-northeast-1 done (1,870 resources)
scan: eu-west-1 started
...
```

## 画面2: scan(結果)

```
$ tag-refactor scan --rules tagrules.yaml
Scanned 9,812 resources in 17 regions (03:41)
Untagged-resource discovery: AWS Config

Tag key variants (candidates; confirm them in your rule file)
  Environment          4,102 resources
    env                  2,950
    Env                    811
    environment            341
  CostCenter             903 resources
    cost-center            512
    cost_center            391

Required tags missing (from tagrules.yaml)
  Owner         1,244 resources
  CostCenter      903 resources

Resource types not supported for tag changes: 2 types, 37 resources
  AWS::Example::Thing   30
  AWS::Example::Other    7

Exit code: 2 (issues found)
Next: run "tag-refactor rules suggest" to draft or update your rule file.
```

- 表記ゆれのグループは、大文字・小文字と区切り文字の違い、よく使われる略語の辞書でまとめた**候補**です(RM-Q2)。
- `--rules` を付けないときは、必須タグの欠落の欄は「ルールファイルがないため調べていない」と表示します(ストーリーのレビュー R-02 への対応)。
- タグを変更できないリソースの種類と件数を示します(要件の前提 AS3)。

**一部のリージョンを読めなかったとき(FR1.5):**

```
! Results are incomplete: 2 regions could not be scanned
    me-south-1   AccessDenied (ec2:DescribeRegions)
    af-south-1   AccessDenied
Scanned 9,640 resources in 15 of 17 regions (03:30)
...
Exit code: 1 (incomplete results)
```

**一度もタグが付いたことのないリソースを探せなかったとき(FR1.3、RM-Q8):**

```
Untagged-resource discovery: unavailable
  Neither AWS Config nor Resource Explorer is enabled in this account.
  Resources that have never been tagged are not included.
```

## 画面3: rules suggest とルールファイル

```
$ tag-refactor rules suggest > tagrules.yaml
Drafted rules for 2 keys and 1 value mapping. Edit tagrules.yaml, then run "tag-refactor plan".
```

(案内のメッセージは標準エラーに出るため、`tagrules.yaml` には混ざりません。)

ルールファイル(YAML。RM-Q1)の例:

```yaml
version: 1

keys:
  Environment:
    aliases: [env, Env, environment]
    values:
      prod: [Prod, production]
      dev:  [Dev, develop]
  CostCenter:
    aliases: [cost-center, cost_center]

required:
  Owner:
    default: platform-team          # 全リソース共通の値
    when:                           # 条件ごとの値(上から順に、最初に一致したものを使う)
      - match: { resourceType: "AWS::EC2::Instance" }
        value: compute-team
      - match: { region: "ap-northeast-1", tags: { Environment: dev } }
        value: tokyo-dev-team
  CostCenter: {}                    # 値を指定しない必須タグ(欠落を数えるだけ。plan では「値が未指定」になる)

remove: [tmp-owner, legacy-id]      # 消すキー
```

- 必須タグの値は、全リソース共通の値(`default`)と条件ごとの値(`when`)の両方で指定できます(FR2.3)。
- 条件が複数当てはまるときは、上から順に最初に一致したものを使い、どれにも当てはまらなければ `default` を使います(案。機能設計で確定)。
- `default` も `when` もない必須タグは、plan で「値が未指定」になります(FR3.3)。

**ルールファイルに誤りがあるとき(FR2.5):**

```
Error: tagrules.yaml line 12: "aliases" must be a list of strings
  Fix: write it as  aliases: [env, Env]
Exit code: 1
```

## 画面4: plan(一覧の形・既定)

```
$ tag-refactor plan --rules tagrules.yaml
Account 123456789012 · 17 regions · plan created 2026-10-19 21:04 (local time)

Changes: 5,341 resources

  ~ rename key   env -> Environment            2,950 resources
  ~ rename key   Env -> Environment              811 resources
  ~ rename key   environment -> Environment      341 resources
  ~ value        Environment: Prod -> prod        640 resources
  + add          Owner = (from rules)           1,189 resources
  - remove       tmp-owner                         52 resources
  (A resource with several changes is counted once in the total.)

Required tag value not set (not changed): 55 resources
  CostCenter   55 resources  (no default or matching condition)

Remove candidates (keys not in your rules; not changed): 6 keys
  Name, aws:cloudformation:stack-name ... (excluded: aws:*), team, ...

Managed by IaC (changes may be reverted by the next deploy): 214 resources

Impact (references that use the tags being changed)
  ! IAM policy  arn:aws:iam::123456789012:policy/DevAccess
      Statement "AllowDevEC2"  Condition aws:ResourceTag/env = "dev"
      affected by: rename key env -> Environment
  ! S3 bucket policy  my-logs-bucket
      Statement "DenyNonProd"  Condition aws:ResourceTag/Environment = "Prod"
      affected by: value Environment: Prod -> prod
  ? IAM role inline policy  role/ci-deployer
      Statement "Any"  Condition aws:ResourceTag/${aws:PrincipalTag/team}
      cannot decide (policy variable)

Summary: 5,341 to change · 2 affected references · 1 undecided · 214 IaC-managed
Exit code: 2 (review needed)
Save this plan: tag-refactor plan --rules tagrules.yaml --out plan.json
```

- 変更の行は全件(全種類)を表示します(RM-Q3)。リソースごとの一覧は差分の形(画面5)で見ます。
- 値の変更でも影響分析を行います(要件 FR4.3、ラフなモックアップのレビュー R-02 への対応)。
- 消す候補には、`aws:` で始まるキーを含めません(要件のレビュー R-06 への対応)。
- 1つのリソースに複数の変更が重なっても、総数では1件と数えます(FR3.2、ラフなモックアップのレビュー R-03 への対応)。

## 画面5: plan(差分の形)

```
$ tag-refactor plan --rules tagrules.yaml --format diff
arn:aws:ec2:ap-northeast-1:123456789012:instance/i-0abc
  - env         = "Prod"
  + Environment = "prod"
  + Owner       = "compute-team"

arn:aws:s3:::my-logs-bucket   [IaC: CloudFormation stack "logging"]
  ~ Env -> Environment = "dev"
...
```

すべての対象リソースを出力します。

## 画面6: plan(JSON、Markdown、保存)

```
$ tag-refactor plan --rules tagrules.yaml --format json      > plan-view.json
$ tag-refactor plan --rules tagrules.yaml --format markdown  > plan.md
$ tag-refactor plan --rules tagrules.yaml --out plan.json
Saved plan to plan.json (5,341 resources, account 123456789012).
```

- `--format json` は表示用の JSON、`--out` は apply 用に保存する変更案のファイル(作成日時、アカウント、変更前のタグ、変更内容を含む。FR3.6)です。

## 画面7: apply(確認画面)

```
$ tag-refactor apply --plan plan.json
Apply plan.json
  Account   123456789012   (current credentials: 123456789012)
  Regions   17
  Created   2026-10-19 21:04 (local time)

Changes: 5,341 resources
  ~ rename key   4,102    ~ value  640    + add  1,189    - remove  52

! 2 changes affect references:
    IAM policy DevAccess          (rename key env -> Environment)
    S3 bucket policy my-logs-bucket (value Environment: Prod -> prod)
? 1 reference could not be decided:
    IAM role inline policy role/ci-deployer

IaC-managed resources: 214 (the next IaC deploy may revert them)

The current tags will be recorded to .tag-refactor/runs/20261019-2110.json before any change.
Type "yes" to apply: _
```

- 確認画面に、種類ごとの件数、影響と判定不能の一覧、IaC で管理されているリソースの件数、アカウントIDとリージョンの数を出します(RM-Q4)。
- `yes` 以外を入力すると、何も書き換えずに終了します(FR5.1)。

**アカウントが違うとき(RM-Q7):**

```
Error: plan.json was created for account 123456789012, but the current credentials are for 210987654321.
  Nothing was changed. Use credentials for 123456789012, or create a new plan.
Exit code: 1
```

**確認を省くオプションで、影響のある変更があるとき(FR5.5):**

```
$ tag-refactor apply --plan plan.json --yes
Stopped: 2 changes affect references and --yes alone does not allow them.
  Nothing was changed. Review them with "tag-refactor plan", or add --allow-impact.
Exit code: 2
```

## 画面8: apply(実行中と結果)

```
Recording current tags... done (.tag-refactor/runs/20261019-2110.json)
Applying changes...
  3,200/5,341 resources · elapsed 00:48

Done: 5,200 changed · 129 skipped · 12 failed
  > Skipped (changed after the plan was created): 129 resources
  x Failed: 12 resources (details: .tag-refactor/runs/20261019-2110.log)
      arn:aws:dynamodb:...:table/orders   AccessDenied (dynamodb:TagResource)
      ...
Undo with: tag-refactor rollback --run 20261019-2110
Exit code: 1 (some changes failed)
```

- 飛ばしたリソースと失敗したリソースを分けて示します(FR5.6、FR5.7)。
- 一部が失敗したので終了コードは 1、飛ばしだけなら 2 です(RM-Q5)。

**記録の保存に失敗したとき(要件のレビュー R-08 への対応):**

```
Error: could not record the current tags (.tag-refactor/runs/20261019-2110.json): no space left on device
  Nothing was changed.
Exit code: 1
```

## 画面9: rollback

```
$ tag-refactor rollback --run 20261019-2110
Rollback run 20261019-2110
  Account   123456789012   (current credentials: 123456789012)
  Regions   17
  Restore   5,200 resources to the recorded tags
  > 31 resources were changed after the run and will be skipped (use --force to restore them too)
Type "yes" to roll back: _
```

```
Done: 5,169 restored · 31 skipped · 0 failed
Exit code: 2 (some resources skipped)
```

## 画面10: 第3弾 IaC への書き戻し(大まかなもの)

```
$ tag-refactor iac --plan plan.json --dir ./infra
Terraform: 14 files to edit
CloudFormation: 3 templates to edit
Wrote changes. Review them with "git diff".
```

詳しい振る舞いは第3弾に取りかかる前に決めます(要件 OQ3)。

## 日本語表示の例(画面2の一部)

```
17 リージョンで 9,812 件のリソースを調べました(03:41)
一度もタグが付いていないリソースの検出: AWS Config

同じ意味で書き方が違うタグキー(候補です。ルールファイルで確定してください)
  Environment          4,102 件
    env                  2,950
    Env                    811
    environment            341

終了コード: 2(問題あり)
次の手順: "tag-refactor rules suggest" でルールファイルの下書きを作るか、更新します。
```

## Assumptions & Open Questions

- 終了コードが複数当てはまるときの優先順位(1 → 2 → 0)は案であり、機能設計で確定する。[assumption]
- ルールファイルで条件が複数当てはまるときに「上から順に最初に一致したもの」を使うのは案であり、機能設計で確定する。[assumption]
- 記録のファイルの既定の置き場所(`.tag-refactor/runs/`)は仮のものである。[assumption]
- オプション名(`--rules`、`--out`、`--plan`、`--yes`、`--allow-impact`、`--force`、`--format`)は仮のものである。[assumption]
- 表示している数値やリソース名は説明用の例である。[assumption]
