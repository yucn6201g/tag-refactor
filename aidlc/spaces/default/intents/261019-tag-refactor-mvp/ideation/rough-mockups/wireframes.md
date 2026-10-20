# 端末出力のモックアップ: tag-refactor

tag-refactor はコマンドラインのツールなので、ここでの「画面」は端末に出る出力のことです。ラフな案であり、文言や記号、列の並びは後のステージで詰めます。

出典の書き方:
- 「M-Q…」はこのステージの `rough-mockups-questions.md` の回答
- 「S-Q…」は `ideation/scope-definition/scope-definition-questions.md` の回答
- 「IB-…」は `ideation/scope-definition/intent-backlog.md` の項目

## 共通の決まりごと

| 項目 | 内容 | 出典 |
|---|---|---|
| 呼び出し方 | 機能ごとのサブコマンド(`tag-refactor <サブコマンド>`) | M-Q2 |
| 表示言語 | 英語と日本語を設定で切り替える。下の例は英語、日本語の例を1つ併記 | M-Q5 |
| 色 | 色は補助で、意味は必ず記号(`+` `~` `-` `!` `?`)と文字でも表す | M-Q6 |
| 色を消す | `NO_COLOR` 環境変数が設定されていれば色を出さない | M-Q6 |
| 進み具合 | 全リージョンを調べている間は、リージョンごとの進み具合を表示する | M-Q6 |
| 対応OS | Linux、macOS、Windows | M-Q7 |

**記号の意味(全画面で共通)**

| 記号 | 意味 |
|---|---|
| `+` | タグを追加する |
| `~` | タグのキー名または値を変える |
| `-` | タグを消す |
| `!` | 影響を受ける参照元がある(要確認) |
| `?` | 影響があるか判定できない(要確認) |

**アクセシビリティの共通メモ:** 端末の出力なので、見出しやランドマークはありません。代わりに、(1) 色に頼らず記号と文字で意味を伝える、(2) 罫線や装飾を最小限にして、スクリーンリーダーでも行単位で読める平文にする、(3) すべての操作をキーボードのコマンド入力だけで行える、の3点をすべての画面に適用します(M-Q6)。

## サブコマンドの一覧(案)

| サブコマンド | 役割 | 段階 | 出典 |
|---|---|---|---|
| `scan` | タグを集め、問題(表記ゆれ、必須タグの欠落)を件数つきで一覧にする | 第1弾 | IB-01、IB-03 |
| `rules suggest`(仮称) | 既存のタグから、正しいキー名・値の候補を出し、ルールファイルの下書きを作る | 第1弾 | IB-02、S-Q7 |
| `plan` | ルールファイルに沿った変更案と、影響を受ける参照元を示す | 第1弾 | IB-06 |
| `apply` | plan に沿ってタグを書き換える | 第2弾 | IB-07 |
| `rollback` | apply による変更を取り消す | 第2弾 | IB-08 |
| `iac` / `pr`(仮称) | IaCへの書き戻しと、プルリクエストの作成 | 第3弾 | IB-09〜IB-12 |

---

## 画面1: scan(実行中)

全リージョンを調べている間の表示です(M-Q6)。

```
$ tag-refactor scan
Scanning account 123456789012 (17 regions)

  [done]     us-east-1        2,431 resources
  [done]     ap-northeast-1   1,870 resources
  [running]  eu-west-1          612 resources so far
  [waiting]  sa-east-1
  ...
  Progress: 9/17 regions   elapsed 01:12
```

- アクセシビリティ: 状態は `[done]` などの文字で示し、色だけに頼らない。

## 画面2: scan(結果)

```
$ tag-refactor scan
Scanned 9,812 resources in 17 regions (03:41)

Tag key variants (same meaning, different spelling)
  Environment   used on 4,102 resources
    env           2,950
    Env             811
    environment     341

Required tags missing
  Owner         1,244 resources
  CostCenter      903 resources

Never-tagged resources found: 318

Next: run "tag-refactor rules suggest" to draft a rule file.
```

- 最後の行で、次にすることを1つだけ示す。
- アクセシビリティ: 見出しは行頭の文字だけで区切り、表の罫線は使わない。

## 画面3: rules suggest(ルールファイルの下書き)

```
$ tag-refactor rules suggest > tagrules.yaml
Drafted rules for 3 keys. Review and edit tagrules.yaml, then run "tag-refactor plan".
```

ルールファイルの中身の例(利用者が編集する。S-Q7):

```yaml
keys:
  Environment:
    aliases: [env, Env, environment]
    values:
      prod: [Prod, production]
      dev:  [Dev, develop]
required: [Owner, CostCenter]
```

## 画面4: plan(端末向けの一覧・既定)

```
$ tag-refactor plan --rules tagrules.yaml
Plan: 4,102 resources to change

  ~ rename key  env -> Environment     2,950 resources
  ~ rename key  Env -> Environment       811 resources
  ~ value       Environment: Prod -> prod  640 resources
  - remove key  tmp-owner                 52 resources

Impact (references that use the keys being changed)
  ! IAM policy   arn:aws:iam::123456789012:policy/DevAccess
                 condition aws:ResourceTag/env
  ! S3 bucket policy   my-logs-bucket
                 condition aws:ResourceTag/Env
  ? IAM role inline policy   role/ci-deployer
                 condition uses a wildcard (aws:ResourceTag/*); cannot decide

Summary: 4,102 to change, 2 affected references, 1 undecided
Save this plan with: tag-refactor plan --rules tagrules.yaml --out plan.json
```

- 影響のある参照元(`!`)と、判定できない参照元(`?`)を分けて示す。
- アクセシビリティ: 1行に1つの事実を書き、記号で種類を示す。

## 画面5: plan(差分の形)

Terraform の plan に似た形です(M-Q3)。

```
$ tag-refactor plan --rules tagrules.yaml --format diff
arn:aws:ec2:ap-northeast-1:123456789012:instance/i-0abc
  - env         = "Prod"
  + Environment = "prod"

arn:aws:s3:::my-logs-bucket
  ~ Env -> Environment = "dev"
...
```

## 画面6: plan(JSON と Markdown)

```
$ tag-refactor plan --rules tagrules.yaml --format json   > plan.json
$ tag-refactor plan --rules tagrules.yaml --format markdown > plan.md
```

- JSON は機械が読むため、Markdown はレビューやプルリクエストのコメントに貼るため(M-Q3)。中身は画面4と同じ情報を持つ。

## 画面7: apply(実行前の確認)

実行時に要約を出して `yes` を入力させる方法と、保存した plan を指定する方法の両方に対応します(M-Q4)。

```
$ tag-refactor apply --rules tagrules.yaml
This will change tags on 4,102 resources in account 123456789012.
  ~ 3 kinds of renames, - 1 kind of removal
  ! 2 references use keys being changed (see "tag-refactor plan")

The current tags will be recorded so you can undo with "tag-refactor rollback".
Type "yes" to continue: _
```

```
$ tag-refactor apply --plan plan.json
Applying saved plan plan.json (created 2026-10-19 21:04, 4,102 resources)
Type "yes" to continue: _
```

- 確認を省くオプション(例: `--yes`)も用意する(M-Q4)。
- 保存した plan より後にタグが変わっていた場合は、実行前に知らせる(案)。

## 画面8: apply(実行中と結果)

```
Applying changes...
  Progress: 3,200/4,102 resources   elapsed 00:48

Done: 4,090 changed, 12 failed
  Failed (see apply-20261019-2105.log):
    arn:aws:dynamodb:...:table/orders   AccessDenied
    ...
Undo with: tag-refactor rollback --run 20261019-2105
```

- 一部が失敗しても、成功した件数と失敗した件数を分けて示し、取り消しの方法を最後に示す。

## 画面9: rollback

```
$ tag-refactor rollback --run 20261019-2105
This will restore tags on 4,090 resources changed by run 20261019-2105.
  ! 7 resources were changed by someone else after the run; they will be skipped.
Type "yes" to continue: _
```

## 画面10: 第3弾 IaCへの書き戻し(ラフ)

```
$ tag-refactor iac --plan plan.json --dir ./infra
Terraform: 14 files to edit
CloudFormation: 3 templates to edit
Wrote changes. Review them with "git diff".
```

- プルリクエストの作成(Could have)は別のサブコマンドにする案(IB-12)。

## 状態ごとの表示

| 状態 | 表示の例 | 対象の画面 |
|---|---|---|
| 空(問題なし) | `No tag issues found in 9,812 resources.` と、次にできること(必須タグのルールを足すなど)を1行 | scan、plan |
| 読み込み中 | 画面1の進み具合 | scan、plan、apply |
| 成功 | 画面2、画面4 | すべて |
| エラー | `Could not read AWS credentials. Set a profile with --profile or AWS_PROFILE.` のように、何が起きたかと直し方を1行ずつ | すべて |
| 一部失敗 | `2 regions could not be scanned (AccessDenied): me-south-1, af-south-1. Results below are partial.` のように、結果が不完全であることを先頭に示す | scan、plan、apply |

## 日本語表示の例(画面2の一部)

```
17 リージョンで 9,812 件のリソースを調べました(03:41)

同じ意味で書き方が違うタグキー
  Environment   4,102 件
    env           2,950
    Env             811
    environment     341

次の手順: "tag-refactor rules suggest" でルールファイルの下書きを作ります。
```

## Assumptions & Open Questions

- 設定がないときの表示言語(英語にするか、OSの言語設定に合わせるか)は決めていない。[assumption]
- サブコマンドの名前(`rules suggest`、`iac`、`pr` など)と、オプションの名前は仮のものである。[assumption]
- 表示している数値やリソース名は説明用の例で、実データではない。[assumption]
