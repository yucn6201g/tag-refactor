# tag-refactor

tag-refactor is a command-line tool that finds AWS tag keys written in
different ways (for example `env` and `Env`) and plans how to unify them,
showing which IAM policies and resource policies (S3, KMS, SQS, SNS and
Secrets Manager) use the tags that would change. It reads every
enabled region, finds resources that were never tagged, counts missing
required tags, and drafts a rule file from the tags in use. It only reads
from AWS. (The rest of this README is in Japanese.)

## できること(U1〜U5)

AWS への書き込みは一切しません。

| コマンド | できること |
|---|---|
| `scan` | アカウントで有効なすべてのリージョンを読み(`--region` で絞れます)、書き方の違うタグのキーをまとめて示します。大文字・小文字と区切り文字(`-`、`_`、空白)の違いに加えて、略語の辞書(`env`→`environment`、`prod`→`production`、`dev`→`development`、`stg`→`staging`。キー全体が一致するときだけ)でまとめます。辞書を使って初めてまとまったものには `(abbreviation match)` の印を付け、終了コードの理由にしません。`--rules` のルールファイルの `required` のキーが付いていないリソースの数、一度もタグが付いていないリソース、タグを変更できない種類も示します。`aws:` で始まるキーは対象にしません。 |
| `plan` | ルールファイルから変更案を作ります。変更は4種類です: 別名を正しいキー名にする(キー名の変更)、値の別の書き方を正しい値にする(値の変更)、付いていない必須タグを付ける(追加)、消すキーを消す(削除)。値の別名を当てても値が違ってぶつかるキーは変えずに「衝突」として示し、値が決まらない必須タグは「値が未指定」、どの規則にもないキーは「消す候補」として示します(どちらも変更しません)。さらに、IAM のポリシー(管理ポリシーとインラインポリシー)と、S3、KMS、SQS、SNS、Secrets Manager のリソースポリシーのうち、変更するタグを参照しているものを、影響あり(`!`)か判定できない(`?`)として示します(下の「影響分析」)。結果は一覧、差分、JSON、Markdown の4つの形式で出せ、`--out` で変更案をファイルに保存できます。`scan` と同じく全リージョンを読みます(一度もタグが付いていないリソースは探しません)。 |
| `rules suggest` | 今使われているタグから、ルールファイルの下書きを作ります。`scan` と同じまとめ方でキーをまとめ、最も多くのリソースで使われている書き方を正しいキー名に、残りを別名にします。値も同じまとめ方でまとめ、書き方が2つ以上ある値を `values` の候補にします。全リージョンを読みます(`--region` で絞れます。一度もタグが付いていないリソースは探しません)。 |

調べる範囲は次のとおりです。どのコマンドも、調べた範囲を `Checked:` で始まる1行で標準エラーに出します。

- タグの付いた(または、かつて付いていた)リソースは、各リージョンのタグ付けの API(`tag:GetResources`)で読みます。リージョンは同時に8つまで読みます。読めなかったリージョン(拒否、再試行を使い切った失敗、時間切れ)があると、結果の先頭に `! Results are incomplete` とリージョンと理由を出し、終了コードは 1 です。
- 一度もタグが付いていないリソースは、タグ付けの API が返さないので、`scan` だけが次の順で探します。
  1. Resource Explorer に集約の索引があれば、そのリージョンで1回だけ探し、結果をリージョンに振り分けます。
  2. それ以外のリージョンは、AWS Config がすべての種類を記録していれば Config で、そうでなければ Resource Explorer(そのリージョンの索引と、絞り込みの条件のない既定のビュー)で探します。Config が一部の種類だけを記録しているときは両方を使います。
  3. どちらも使えないリージョンは、理由とともに `Untagged-resource discovery: partial`(一部)または `unavailable`(すべて)と示します。これだけでは終了コードは変わりません。このとき必須タグの件数は下限です(`Required tag counts are a lower bound.`)。
- IAM のグローバルな種類(ユーザー、グループ、ロール、管理ポリシー)は、一度もタグが付いていないリソースの検出の対象外です。AWS Config は既定ではこれらを記録せず、Resource Explorer は IAM のタグを索引しないためです。
- `plan` の影響分析が読むポリシーと読まないポリシーは、下の「影響分析」にまとめています。「0 affected」は、その範囲での結果です。変更がないときは、ポリシーを読みません。

どのコマンドも、英語か日本語で表示でき(`--lang`)、端末では色を付けます(`NO_COLOR` で消せます)。

まだできないこと(後の作業単位): 変更の適用と取り消し(`apply`、`rollback`)、IaC への反映。

## インストール

Go(最新の安定版、または1つ前の版)で、手元でビルドします。

```bash
go install github.com/yucn6201g/tag-refactor/cmd/tag-refactor@latest
```

公開のバイナリ(GitHub Releases)は、第1弾(`v0.1.0`)で用意します。

## 使い方

```bash
# 全リージョンのタグの表記ゆれ、必須タグの欠落、一度もタグが付いていないリソースを調べる
tag-refactor scan --profile dev --rules tagrules.yaml

# 2つのリージョンだけを調べる(--region は1回に1つ。繰り返して指定する)
tag-refactor scan --profile dev --region us-east-1 --region eu-west-1

# 今のタグからルールファイルの下書きを作り、ファイルに保存する
tag-refactor rules suggest --profile dev --output tagrules.yaml

# ルールファイルから変更案と影響を見る(書き換えはしない)
tag-refactor plan --rules tagrules.yaml --profile dev

# 変更案をリソースごとの差分で見る / プルリクエストに貼る Markdown にする
tag-refactor plan --rules tagrules.yaml --format diff
tag-refactor plan --rules tagrules.yaml --format markdown > plan.md

# 変更案をファイルに保存する(後の apply が読む。標準出力にはいつもの一覧が出る)
tag-refactor plan --rules tagrules.yaml --out plan.json

# 日本語で表示する / スクリプトから JSON で読む
tag-refactor scan --lang ja
tag-refactor scan --format json > scan.json

# AWS の呼び出しごとに1行(操作、リージョン、時間、再試行の回数)を標準エラーに出す
tag-refactor scan --debug --region us-east-1
```

| オプション | 意味 |
|---|---|
| `--profile` | 使う AWS のプロファイル。省略すると AWS の標準の設定(環境変数など)に従います。 |
| `--region` | 読むリージョン。繰り返して複数を指定できます(同じ値は1つにまとめます)。省略するとアカウントで有効なすべてのリージョンを読みます。アカウントの確認(STS、EC2、IAM)は、最初の `--region`、またはプロファイルか `AWS_REGION` のリージョンで行います。有効でない値が1つでもあれば、リソースを読む前にエラーにします。 |
| `--rules` | ルールファイル。`plan` では必須、`scan` では省略できます(省略すると `Required tags missing: not checked (no rule file)`)。`rules suggest` は受け付けません。 |
| `--output` | `rules suggest` だけ。下書きを標準出力ではなくこのファイルに、UTF-8(BOM なし、LF の改行)で書きます。同じフォルダの一時ファイルに書いてから名前を付けるので、途中で止まっても壊れたファイルは残りません。既にあるファイルは上書きせずにエラーにします(消してから実行してください)。名前はハードリンクで付けるので、確かめてから書くまでの間にできたファイルも上書きしません。ハードリンクを使えないファイルシステム(FAT32 など)では、確かめたあとに名前を変えるため、その間にできたファイルを置き換えることがあります。下書きのファイルは本人だけが読み書きできる権限(Unix の 0600)で作ります。 |
| `--format` | 標準出力の結果の形式。`scan` は `table`(既定)、`json`、`markdown`、`plan` はこれに `diff` を加えた4つです。知らない値は、AWS を呼ぶ前にエラーにします。 |
| `--out` | `plan` だけ。変更案を JSON のファイルに保存します(下の「変更案の保存」)。既にあるファイルは上書きしません。保存先のフォルダに書けるかを、AWS を呼ぶ前に確かめます。 |
| `--lang` | 表示の言語。`en`(英語)か `ja`(日本語)。どのコマンドにも付けられます(下の「言語と色」)。 |
| `--debug` | 呼び出しの記録を標準エラーに出します。タグの値、ポリシーの中身、問い合わせの文、認証情報は出しません。この行は言語によらず英語です。 |

認証情報を受け取るオプションはありません。認証情報は AWS の SDK が、環境変数、共有の設定ファイル、SSO などの標準の方法で読みます。実行の途中で認証情報の期限が切れた場合は、すべての読み取りを止め、途中までの結果は出さずに終了コード 1 で終わります。

AWS の呼び出しは、呼び出し回数の上限などの一時的な失敗のとき、待ち時間を延ばしながら最大10回まで試します(1回の待ちは最大20秒、1回の試行は30秒まで、再試行を含めた1回の呼び出しは5分まで)。

Ctrl+C(Linux と macOS では `SIGTERM` も)で止めると、AWS の呼び出しを取り消し、一時ファイルを消し、端末の設定を戻してから、`Interrupted. Nothing was saved.` と出して終了コード 1 で終わります。2回目の Ctrl+C では、後片付けを待たずにすぐ終わります。

### plan の出力の形式

`--format` で選びます。4つの形式は同じ変更案から作るので、変更の件数(種類ごとと総数)、衝突、値が未指定、消す候補のキーの数、IaC で管理されているリソース、影響の件数は、どの形式でも同じです。件数には3桁ごとにカンマを付けます(JSON は数のまま)。

一覧(`table`、既定):

```
Changes: 5,341 resources
  ~ rename key   env -> Environment                 2,950 resources
  ~ value        Environment: Prod -> prod            640 resources
  + add          Owner = (from rules)               1,189 resources
  - remove       tmp-owner                             52 resources
  (A resource with several changes is counted once in the total.)

Required tag value not set (not changed): 55 resources
  CostCenter   55 resources  (no default or matching condition)

Remove candidates (keys not in your rules; not changed): 3 keys
  Name (812), team (40), tmp (3)

Managed by IaC (changes may be reverted by the next deploy): 214 resources

Impact (references that use the tags being changed)
  ! IAM policy  arn:aws:iam::123456789012:policy/DevAccess
      Statement "AllowDevEC2"  Condition StringEquals aws:ResourceTag/env = "dev"
      affected by: rename key env -> Environment

Summary: 5,341 to change · 0 conflicts · 1 affected · 0 undecided · 55 value not set · 3 remove candidates · 214 IaC-managed
```

差分(`diff`)は、変更、衝突、値が未指定のあるすべてのリソースを ARN の順に出し、変わるタグだけを変更前(`-`)と変更後(`+`)で示します。キー名だけが変わり値が同じなら `~ 古いキー -> 新しいキー` の1行です。影響のある変更の行の先頭には `!`、判定できない参照だけが当たる行には `?` が付きます。CloudFormation のスタックのリソースには `[IaC: CloudFormation stack "名前"]` を付けます。最後に種類ごとの件数と要約の行を出します。

```
arn:aws:ec2:ap-northeast-1:123456789012:instance/i-0abc
! - env         = "Prod"
! + Environment = "prod"
  + Owner       = "compute-team"

arn:aws:s3:::my-logs-bucket   [IaC: CloudFormation stack "logging"]
  ~ Env -> Environment = "dev"
```

JSON(`json`)は、スクリプトから読むための形です。`version`、`toolVersion`、`command`、`exitCode`(実際の終了コードと同じ値)、`accountId`、`complete`、`failedRegions`、`unsupportedResourceTypes`、`summary`、`details` を持ち、`details` には保存した変更案と同じ形の `resources`、`keyConflicts`、`impactFindings`、`unreadPolicies`、`unsetRequiredTags`、`removeCandidates` が入ります。`scan --format json` も同じ枠で、表記ゆれと必須タグの欠落を出します。文字列は訳しません。制御文字、DEL、C1 の制御文字、双方向の制御文字などの書式の文字は `\u` で始まる形にエスケープするので、端末に出しても働きません(JSON として読めば元の文字列に戻ります)。

Markdown(`markdown`)は、プルリクエストや Wiki に貼るための形です。対象(アカウント、リージョンの数、作成日時)、変更、衝突、値が未指定、消す候補、IaC、影響、要約を見出しと表で書き、リソースごとの変更は `<details>` で畳んだ表に全件を書きます。

- AWS から読んだ文字列(キー、値、ARN、ポリシーの名前など)は、すべてコードの書式(`` ` ``)の中に書きます。そのため、タグの値に HTML、Markdown の記号、`|` があっても、貼った先で働いたり表を崩したりしません。コードの書式の中の `<`、`>`、`&` はそのまま見えます(貼った先で値をそのままコピーできるように、文字の参照には変えていません)。
- 表のセルの中の `|` は `\|` と書きます。GitHub など、表の中のコードの書式で `\|` を `|` と表示するところに貼ってください。
- 貼る前に、標準出力だけをファイルに保存してください(`> plan.md`)。案内や終了コードの行は標準エラーに出るので、ファイルには入りません。

### 変更案の保存(plan --out)

`--out plan.json` を付けると、変更案を JSON(UTF-8、BOM なし、LF の改行)で保存します。標準出力には、付けないときと同じ結果が出ます。保存したことは標準エラーに `Saved plan to plan.json (5,341 resources, account 123456789012).` と出ます。

- 中身: 形式の版(`version`)、ツールの版、変更案の ID(`plan-20261019T120400Z-7f3a` の形)、アカウント、作成日時(UTC)、リージョン、不完全の印、使ったルールファイル(`--rules` に渡したとおり)、変更のあるリソースごとの変更前のタグ全体(`aws:` で始まるものも)と変更、衝突、影響、値が未指定、消す候補、タグを変更できない種類。認証情報、プロファイルの設定、環境変数の値は書きません。
- 既にあるファイルは上書きしません。同じフォルダの一時ファイル(`.tag-refactor-plan-*.tmp`)に書いてから名前を付けるので、保存先には完全な変更案だけが現れます。一時ファイルは、成功しても失敗しても、Ctrl+C で止めても消します。ファイルは本人だけが読み書きできる権限(Unix の 0600)で作ります。
- 256 MiB を超える変更案は保存しません。`--region` で分けて作ってください。
- 収集や影響分析が不完全でも保存し、不完全の印(`inventoryComplete`、`impactAnalysisComplete`)を付けます。
- **Windows PowerShell 5.1 では、変更案を `--out` で保存してください。** `>` のリダイレクトは UTF-16 で書き直すので、`--format json > plan.json` で作ったファイルは変更案として読めません(読むと「UTF-8 で保存し直す」案内が出ます)。

### 影響分析

`plan` は、変更が1件以上あるとき、変更するタグを参照しているポリシーを探します。AWS への呼び出しは読み取りだけです。

読むもの:

- IAM のポリシーは、`iam:GetAccountAuthorizationDetails` でまとめて読みます(1回の呼び出しで、ユーザー、ロール、グループと、利用者が作った管理ポリシーを合わせて 1,000 件まで)。照合するのは次の2つです
  - 利用者が作った IAM の管理ポリシーのうち、ユーザー・ロール・グループに付いているものと、権限の上限(permissions boundary)として使われているもの(既定の版)
  - すべてのユーザー、ロール、グループのインラインポリシー(サービスにリンクされたロールも含みます)
- S3 のバケットポリシー、KMS のキーポリシー(利用者が作ったキーだけ)、SQS のキューポリシー、SNS のトピックポリシー、Secrets Manager のシークレットのリソースポリシー。変更案のリージョン(`--region` で絞ったときはそのリージョン)のものを、各サービスの一覧から探して読みます(`scan` の結果には頼りません)。S3 は全リージョンのバケットの一覧を1回読み、変更案のリージョンにあるバケットだけのポリシーを読みます

照合しないもの: ロールの信頼ポリシー、AWS が管理する IAM のポリシー、どこにも付いていない管理ポリシー(`iam:GetAccountAuthorizationDetails` の答えには入りますが、照合しません)、AWS が管理する KMS のキー、組織の SCP と RCP、そのほかのサービスのポリシー(VPC エンドポイント、API Gateway など)。シークレットの値は読みません。

照合するもの:

- 条件キー `ResourceTag/<キー>`、`RequestTag/<キー>`、`PrincipalTag/<キー>`、`TagKeys`(前置は `aws:` でもサービス名でも)
- 条件キーの中、条件の値の中、`Resource`・`NotResource` の中のポリシー変数(例 `${aws:PrincipalTag/team}`)
- 4種類の変更(キー名の変更、値の変更、必須タグの追加、削除)のすべて。演算子の種類(等しい、大文字・小文字なし、パターン、`Null`、`IfExists`、`ForAnyValue:`・`ForAllValues:`、それ以外)に合わせて判定します

結び付ける変更:

- IAM のポリシーの `ResourceTag`、`RequestTag`、`TagKeys` は、そのキーのすべてのリソースの変更に結び付けます(IAM のポリシーは、どのリソースに当たるかを `Resource` で決めますが、そこまでは照合しません)。
- リソースポリシーの `ResourceTag`、`RequestTag`、`TagKeys` は、そのポリシーが付いたリソース自身の変更だけに結び付けます。
- `PrincipalTag` は、どのポリシーでも、IAM のユーザーとロールの変更だけに結び付けます。

印の意味:

- `!` 影響あり: その変更で、条件の結果が変わりうる参照です。タグのキーの `*` と `?` は IAM と同じパターンとして機械的に当てはめ、当てはまれば `!`、当てはまらなければ影響なしとします。`aws:ResourceTag` の条件キーは大文字・小文字を区別しないため、大文字・小文字だけのキー名の変更は影響なしとします(ほかの条件キーやポリシー変数では `!`)。
- `?` 判定できない: ポリシー変数を含む参照(実際のキーや値は AWS が評価するときまで決まりません)と、文字列を比べない演算子(`ArnLike`、`NumericEquals` など)や `TagKeys` の `Null` のように、事前に結果を決められない条件です。一覧では理由を `cannot decide (policy variable)` か `cannot decide (condition operator)` で示します。
- どちらでもない参照は示しません。影響があり得る参照を「影響なし」と表示することはありません。

指摘は、ポリシー(`<ユーザー・ロール・グループの ARN>#<名前>` はインラインポリシー、リソースポリシーはリソースの ARN)、ステートメント、演算子、条件キーごとに1件で、原因の変更をすべて示します。`Resource` と `NotResource` の中の変数による指摘は、条件キーの欄に `Resource`(または `NotResource`)と、その値を出します。

一覧や1つのポリシーを読めなかったとき(拒否、再試行を使い切った失敗、1 MiB を超える文書、JSON として読めない文書、既定の版の文書が答えに無いポリシー)は、結果の先頭に `! Impact analysis is incomplete` と、種類、リージョン、ポリシー、理由を出し、終了コードは 1 です。読めたポリシーの照合は続けます。ただし IAM は、`iam:GetAccountAuthorizationDetails` がどれか1ページでも失敗すると、管理ポリシーとインラインポリシーの両方を「一覧ごと読めなかった」とし、IAM のポリシーは1つも照合しません(読めた途中までのページで照合すると、読めなかったポリシーがあるのに影響が無いように見えるためです)。一覧のあとで消えたリソースと、ポリシーが付いていないリソースは、読めなかったことにはしません。認証情報が切れたときは、途中の結果を出さずに止まります。

ポリシーは同時に読みます。IAM は1回に1つの呼び出しで、ページを順に読みます。リソースポリシーはリージョンを8つまで、1つのリージョンの中は4つまでです。この数は変えられません。どの順に読み終わっても、結果の順は同じです。

### 言語と色

言語は、`--lang` があればそれを使います。なければ `LC_ALL`、`LC_MESSAGES`、`LANG` の順に最初の空でない値を見て、`ja` で始まれば日本語、それ以外は英語です。Windows で3つとも空のときは、Windows の表示言語(最初の優先言語)が日本語なら日本語にします。読めなければ英語です。

- 日本語にするのは、結果(一覧、差分、Markdown)、進み具合、案内、警告、エラー(保存した変更案の誤りを含む)、`rules suggest` の下書きのコメントです。`--help`、`--debug` の行、JSON は英語のままです。数字、ARN、ポリシー名、キー、値、オプション名は訳しません。ルールファイルの誤りの説明と AWS から来た詳しい文言は、日本語の表示でも英語のままです。
- Windows のコンソールでは、コードページによらず日本語が正しく出ます。列の揃えは、端末での文字の幅(全角は2文字分)で計算します。

色は補助です。意味は必ず記号(`+` 追加、`~` 変更、`-` 削除、`!` 影響ありまたは不完全、`?` 判定できない)と文字でも示します。色を付けるのは、標準出力と標準エラーのそれぞれについて、端末であり、`NO_COLOR` が設定されていない(または空)、`TERM` が `dumb` でない、Windows では仮想端末の処理を有効にできた、のすべてを満たすときだけです。ファイルやパイプに向けた出力には色の制御の文字は入りません。Windows で有効にした仮想端末の処理は、終わるときに元に戻します。

### ルールファイルの下書き(rules suggest)

```bash
# 標準出力に出す(Linux、macOS、PowerShell 7 で UTF-8 にした場合)
tag-refactor rules suggest --profile dev > tagrules.yaml

# Windows の PowerShell では --output で保存する
tag-refactor rules suggest --profile dev --output tagrules.yaml
```

Windows の PowerShell のリダイレクト(`>`、`| Out-File`)は、ツールの出力をコンソールのコードページで読み直して書くため、日本語などのタグの値が化けます。Windows では `--output` を使ってください。リダイレクトを使うなら、PowerShell 7 で先に `[Console]::OutputEncoding = [Text.Encoding]::UTF8` を設定します。Windows PowerShell 5.1 の `>` が書く UTF-16(BOM 付き)のファイルも読み込めますが、値が化けていないかは確かめてください。

下書きの例:

```yaml
# generated by tag-refactor v0.1.0
# account 123456789012, 17 of 17 regions, 2026-10-24T09:00:00Z
# Review each key: the most used spelling is proposed as the canonical key
# and the other spellings as its aliases.
version: 1
keys:
  "env":
    aliases: ["Env", "environment"]
    values:
      "prod": ["Prod", "production"]
  "Name": {}  # values not suggested: more than 20 different values
# required:
#   ...
# remove: ["tmp-owner"]
```

- 下書きのコメントは、言語が日本語(`--lang ja` など。下の「言語と色」)なら日本語、それ以外は上の例のとおり英語です。`required` と `remove` の書き方の例は、どちらの言語でも同じ YAML です。64 KiB の上限は、その言語のコメントを含めた大きさで判定します。
- 見つかったすべてのキー(`aws:` で始まるものを除く)を入れます。書き方が1つのキーは `{}` です。入れておかないと、`plan` でそのキーが「消す候補」になるためです。
- 正しいキー名は、最も多くのリソースで使われている書き方です。同じ数なら略語の辞書の正式な形(`environment` など)、次に名前の順で選びます。別名は数の多い順です。どれを正しいキー名にするかは、下書きを見て決め直してください。
- 異なる値が20個を超えるキー(名前や ID など)は、値の候補を出さず、その旨をコメントで書きます。
- キーと値はすべて二重引用符で囲み、制御文字や双方向の制御文字など印字できない文字は `\uXXXX`(基本多言語面の外は `\UXXXXXXXX`)で書きます。`[` と `{` も `\u005B`、`\u007B` で書きます(読み込み側の括弧の数の上限に数えないため)。読み込むと元の文字列に戻ります。
- `required` と `remove` は利用者が決めるもので、書き方の例だけをコメントで書きます。
- 下書きは読み込みの上限(64 KiB)に収めます。超えるときは、リソースの少ないキーから値の候補を外し、`values omitted: the draft would exceed 64 KiB` と書きます。キーだけで超えるときは、下書きを出したうえで警告し、終了コード 1 で終わります。
- 読めなかったリージョンがあると、下書きの先頭に `# Results are incomplete: <リージョン>` を書き、標準エラーにも示して、終了コード 1 で終わります。それ以外は、表記ゆれがあっても終了コード 0 です。
- 呼ぶ AWS の操作は `sts:GetCallerIdentity`、`ec2:DescribeRegions`、`tag:GetResources` だけです。

### ルールファイル

```yaml
version: 1
keys:
  Environment:
    aliases: [env, Env]
    values:
      production: [prod, Prod]
required:
  Owner:
    default: platform-team
    when:
      - match: {resourceType: ec2:instance}
        value: infra-team
      - match: {region: us-east-1, tags: {Environment: production}}
        value: us-prod-team
remove: [tmp-owner]
```

| 項目 | 意味 |
|---|---|
| `keys.<正しいキー名>.aliases` | 同じ意味の別のキー名。`plan` が正しいキー名への名前の変更にします。1つの別名は1つの正しいキー名にだけ書けます。 |
| `keys.<正しいキー名>.values` | `正しい値: [別の書き方]`。同じキーの中で、1つの書き方は1つの正しい値にだけ書けます。 |
| `required.<キー>.default` | どの条件にも当てはまらないリソースの値。 |
| `required.<キー>.when` | 条件ごとの値。上から順に比べ、最初に当てはまった条件の値を使います。どれにも当てはまらなければ `default`、`default` もなければ「値が未指定」です。 |
| `remove` | 消すキー。書いたとおり(大文字・小文字を区別)に一致したキーだけを消します。 |

`when` の `match` には、次の1つ以上を書きます(書いたものすべてに当てはまるときに当てはまります)。何も書かない `match` は誤りです。すべてのリソースに使う値は `default` に書きます。

- `resourceType`: `scan` が示す種類と同じ形で書きます。サービス名だけ(`s3`、`sqs`)か、`サービス名:種類`(`ec2:instance`)です。CloudFormation の形(`AWS::EC2::Instance`)は誤りにし、ツールが対応を知っている種類は書くべき形を示します。`scan` が `AWS::...` の形のまま示す種類(ARN を持たない、または対応表にない種類)と `unknown` は、条件に書けません。
- `region`: リージョン名(`us-east-1` など)。グローバルなリソース(IAM のロールなど、ARN にリージョンがないもの)は、`region` を書いた条件には当てはまりません。
- `tags`: 書いたすべてのキーと値の組を持つリソースに当てはまります。比べる相手は、別名を正しいキー名に、値の別の書き方を正しい値に置き換え、`remove` のキーを除いたあとのタグです。そのため、`tags` に別名、値の別の書き方、消すキーは書けません(書くと、書くべき正しい形を示して止まります)。`aws:` で始まるキー(`aws:cloudformation:stack-name` など)は、条件には使えます。

そのほかの決まり:

- `plan` は、リソースごとに次の順で変更を作り、この順に番号(`c-000001` から)を振ります。
  1. 正しいキー名(名前の順)ごとに、付いている正しいキー名と別名の値を、`values` の別の書き方を正しい値にしてから比べます。すべて同じなら、別名を正しいキー名に変えます(正しいキー名が既にあれば別名を消すだけ)。違えば変えずに「衝突」にします。続けて、正しいキー名の値が別の書き方なら正しい値に変えます。
  2. 付いていない必須タグ(キーの順)を付けます。値は `when`、`default` の順に決め、`values` の別の書き方で書いてあれば正しい値にします。既に付いている値は変えません。値が決まらなければ「値が未指定」にし、そのリソースのほかの変更は残します。
  3. `remove` のキーを消します。
- どの規則にも書かれていないキーは「消す候補」として示すだけで、消しません。消す候補だけなら終了コードは 0 です。
- `scan` は `required` のキーが付いていないリソースの数を数えます(大文字・小文字を区別します)。
- `keys`、`aliases`、`required`、`remove` の `aws:` で始まるキーは、その規則を無視して警告します。
- 必須タグのキーは、別名や消すキーと同じにできません。消すキーは、正しいキー名や別名と同じにできません。
- タグのキーは1〜128文字、値は0〜256文字です(AWS の制限)。数のように見える値(`123`、`true`)は引用符で囲みます。
- 知らないキー、型の誤り、YAML の誤り、項目どうしの矛盾は、ファイル名、行番号、キー、直し方を示して、最初の1つで止まります。AWS は呼びません。
- `keys` がない、または空のときは、`no key rules` の警告を出します(`plan` は変更案を作りません)。
- アンカー(`&`)、別名(`*`)、統合キー(`<<`)、タグ(`!`)、重複したキー、64 KiB を超えるファイルは受け付けません。
- 入れ子が 32 段を超えるファイル(括弧 `[`、`{` の入れ子、またはインデントの入れ子)は、YAML として読む前に行番号を示して止めます。引用符の中、コメント、テキストの中の括弧は入れ子に数えませんが、それらの括弧が合わせて 1024 個を超えるファイルも受け付けません(タグのキーと値に括弧は使えません)。
- CRLF の改行のファイルも同じに読めます。文字コードは UTF-8(BOM はあってもなくても)と、BOM 付きの UTF-16(LE、BE)を読みます。BOM のない UTF-16 は「UTF-8 で保存し直す」案内を出して止めます。64 KiB の上限は UTF-8 にしたあとの大きさで判定します。

### 出力と終了コード

- 標準出力には結果(一覧、差分、JSON、Markdown)だけを出します。アカウントとリージョン、変更案の作成日時、進み具合、警告、エラー、`Checked:` の行、かかった時間、`Exit code` の行、次の手順(`Save this plan: ...`)、`Saved plan` の行は標準エラーに出します。形式や言語を変えても、`--out` を付けても同じです。
- 進み具合は、標準エラーが端末なら同じ場所を書き換えて示します(リージョンごとの状態と経過時間)。端末でないとき、`TERM=dumb` のとき、Windows で仮想端末の処理を有効にできないときは、状態が変わるたびに1行ずつ追記します。標準出力をファイルに向けても、標準エラーが端末なら書き換えます。
- 記号: `+` 追加、`~` 名前や値の変更、`-` 削除、`!` 影響ありまたは不完全、`?` 判定できない(ポリシー変数や、事前に結果を決められない演算子を含む条件。「影響分析」を参照)。
- AWS から読んだ文字列(ARN、種類の名前など)の制御文字や双方向の制御文字は、一覧と差分では `\u{…}` の形に置き換えて出します。

| 終了コード | 意味 |
|---|---|
| 0 | 問題なし(`scan`: 表記ゆれも必須タグの欠落もない、`plan`: 変更不要(消す候補だけはありうる)、`rules suggest`: 下書きを出した) |
| 1 | エラー、Ctrl+C での中断、読めなかったリージョンがある、影響分析が不完全(ポリシーを読めなかった)、変更案を保存できなかった、または `rules suggest` の下書きがキーだけで 64 KiB を超えた |
| 2 | 確認が必要(`scan`: 表記ゆれか必須タグの欠落、`plan`: 変更、衝突、値が未指定、影響のどれかがある) |

一度もタグが付いていないリソースを探せなかったこと、見つかったこと、タグを変更できない種類があることだけでは、終了コードは変わりません。

エラーは2行(`Error:` 何が起きたか、`Fix:` どう直すか。日本語では `エラー:` と `直し方:`)で出します。AWS の SDK のエラー文はそのまま出しません。

## 必要な権限

読み取りの権限だけで動きます。必要な IAM の操作は [`docs/iam/readonly-policy.json`](docs/iam/readonly-policy.json) にまとめています。この例の `<account-id>` は、使うアカウントの ID(12桁)に置き換えてください。1つのリソースを読む操作は、そのアカウントのそのサービスの ARN の形に絞っています(下の表)。一覧の操作と `iam:GetAccountAuthorizationDetails` は AWS がリソースを絞れないため `*` です。影響分析はすべての有効なリージョンを読むため、KMS、SQS、SNS、Secrets Manager の ARN のリージョンは `*` です。

| IAM の操作 | 使う場面 |
|---|---|
| `ec2:DescribeRegions` | アカウントで有効なリージョンの一覧と、指定したリージョンが使えるかの確認。指定したリージョンの STS が認証情報を拒否したときは、`us-east-1` から使えるリージョンを確かめ直し、有効にしていないオプトインのリージョンなら「そのリージョンは使えない」と示します |
| `tag:GetResources` | タグの付いたリソースの読み取り |
| `iam:GetAccountAuthorizationDetails` | ユーザー、ロール、グループとそのインラインポリシー、利用者が作った管理ポリシーと、その付いている数・権限の上限として使われている数・既定の版の中身をまとめて読みます(`plan` で変更があるときだけ。以下の影響分析の操作も同じ)。リソースを絞れない操作なので `*` です |
| `s3:ListAllMyBuckets` | API の `ListBuckets` に AWS が求める IAM の操作。全リージョンのバケットとそのリージョン |
| `s3:GetBucketLocation`、`s3:GetBucketPolicy` | 一覧にリージョンが無いバケットのリージョンと、バケットポリシー。`arn:aws:s3:::*` |
| `kms:ListKeys` | KMS のキーの一覧 |
| `kms:DescribeKey`、`kms:GetKeyPolicy` | 利用者が作ったキーかと、キーポリシー。`arn:aws:kms:*:<account-id>:key/*` |
| `sqs:ListQueues`、`sqs:GetQueueAttributes` | キューの一覧と、キューの属性 `Policy` と `QueueArn`。属性は `arn:aws:sqs:*:<account-id>:*` |
| `sns:ListTopics`、`sns:GetTopicAttributes` | トピックの一覧と、トピックの属性 `Policy`。属性は `arn:aws:sns:*:<account-id>:*` |
| `secretsmanager:ListSecrets`、`secretsmanager:GetResourcePolicy` | シークレットの一覧と、そのリソースポリシー(値は読みません)。ポリシーは `arn:aws:secretsmanager:*:<account-id>:secret:*` |
| `config:DescribeConfigurationRecorders` | AWS Config が何を記録しているか(`scan`) |
| `config:DescribeConfigurationRecorderStatus` | AWS Config が記録しているか(`scan`) |
| `config:SelectResourceConfig` | AWS Config に記録されたリソースとタグの問い合わせ(`scan`) |
| `resource-explorer-2:ListIndexes` | Resource Explorer の索引と、集約の索引の有無(`scan`) |
| `resource-explorer-2:GetDefaultView` | リージョンの既定のビュー(`scan`) |
| `resource-explorer-2:GetView` | 既定のビューに絞り込みの条件があるか(`scan`) |
| `resource-explorer-2:Search` | API の `ListResources` に AWS が求める IAM の操作。タグのないリソースを件数の上限なく読みます(`scan`)。1,000 件までに限られる API の `Search` は呼びません |

`sts:GetCallerIdentity`(呼び出し元の確認)は IAM の権限が要りません。AWS Config や Resource Explorer の記録・索引・ビューを作ったり変えたりする操作は呼びません。使えないときは、理由(足りない IAM の操作の名前、`no default Resource Explorer view` など)を示すだけです。影響分析の操作が拒否された場合、`plan` は変更案と読めたポリシーの指摘を出したうえで「影響分析は不完全」と、読めなかった種類、リージョン、理由を示し、終了コード 1 で終わります。

## 開発

```bash
export GOTOOLCHAIN=local
go test -mod=readonly ./...                       # 受け入れ条件、単体、性能、決定性、取り消し、出力の守り
go test -mod=readonly -coverpkg=./cmd/tag-refactor/...,./internal/... -coverprofile=coverage.out ./cmd/tag-refactor/... ./internal/...
go tool cover -func=coverage.out                  # 80% 以上
CGO_ENABLED=1 go test -race -mod=readonly ./...   # C のコンパイラが要る(CI では Linux の race ジョブ)
gofmt -l .
golangci-lint run
go vet -tags integration ./...
go tool govulncheck ./...
go test -mod=readonly -run '^$' -fuzz FuzzParse -fuzztime 5m ./internal/rules/      # ルールファイルの読み込みのファジング(手元だけ)
go test -mod=readonly -run '^$' -fuzz FuzzLoadPlan -fuzztime 5m ./internal/planner/ # 保存した変更案の読み込みのファジング(手元だけ)
go test -mod=readonly -run '^$' -fuzz FuzzParsePolicy -fuzztime 5m ./internal/impact/ # ポリシーの文書の読み取りのファジング(手元だけ)
go test -mod=readonly -run 'TestDetectionTable|TestLargeAnalysisU5' ./internal/impact/ # 検出率100%の表と、大きな偽の AWS での影響分析
go test -mod=readonly -run 'TestGolden' ./internal/display/... -update               # 出力の期待のファイルを作り直す(差分を人が見る)
```

- CI では、ファジングの種(`internal/rules/testdata/fuzz/FuzzParse/`、`internal/planner/testdata/fuzz/FuzzLoadPlan/`、`internal/impact/testdata/fuzz/FuzzParsePolicy/` とテストのコードの中の種)だけが普通のテストとして動きます。見つかった入力は直したうえで種に足します。
- 出力の期待のファイル(`internal/display/testdata/`)は、4つの形式 × 2つの言語と保存した変更案です。`.gitattributes` で LF に固定し、3つの OS で同じバイト列になることを確かめます。
- 端末での文字の幅の表(`internal/display/width_table.go`)は、Go の `unicode` パッケージと同じ版の Unicode の `EastAsianWidth.txt` から作ります。Go を上げて Unicode の版が新しくなると、テストが「表を作り直す」案内で失敗します。そのときは `internal/display` で `go generate` を動かし(`gen_width.go` が `https://www.unicode.org/Public/<版>/ucd/EastAsianWidth.txt` を取ってきます。取れないときは `go run gen_width.go -in <手元のファイル>`)、できた `width_table.go`(先頭に元の URL と SHA-256 を書きます)をコミットします。CI はネットワークを使わず、コミットした表だけを使います。

- テストは AWS を呼びません。HTTP の層の偽の AWS(`internal/testutil`)を使い、手元の `~/.aws` も読みません。
- CI(`.github/workflows/ci.yml`)は Linux、macOS、Windows と Go の2つの版でテストし、Linux で `-race` 付きのテスト、カバレッジ、lint(`cfn-lint` は `test/sandbox/*-fixtures.yaml`)、govulncheck、モジュールの確認、ビルドを行います。

## 検証用アカウントでの確認(チェックポイント)

実際の AWS アカウントを使う確認は、手元だけで行います(CI では行いません)。少額の費用がかかる場合があります。Resource Explorer には費用はかかりません。AWS Config の記録は有効にしません。

### 準備

1. 管理者のプロファイル(例 `tag-refactor-sandbox`)で、検証用アカウントであることを確かめます: `aws sts get-caller-identity --profile tag-refactor-sandbox`。
2. U1 の確認用のスタック(`test/sandbox/u1-fixtures.yaml`、`ap-northeast-1`)を作るか、U2 で読み取り用ロール R2 の権限が増えたので更新します。手順は U1 のインフラの設計 [`infrastructure-specification.md`](aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u1-walking-skeleton/infrastructure-design/infrastructure-specification.md) にあります。
3. U2 のスタックの前に、`ap-northeast-1` に索引や既定のビューが既にあるかを確かめます。

   ```bash
   aws resource-explorer-2 list-indexes --profile tag-refactor-sandbox --region ap-northeast-1
   aws resource-explorer-2 get-default-view --profile tag-refactor-sandbox --region ap-northeast-1
   ```

   既にあれば、対応するパラメータに `false` を渡します。既にある既定のビューに絞り込みの条件があるときは、期待結果が変わるので、デプロイせずに人が判断します。
4. U2 のスタックをデプロイします(Resource Explorer の索引と、条件のない既定のビュー)。

   ```bash
   aws cloudformation deploy --profile tag-refactor-sandbox --region ap-northeast-1 \
     --stack-name tag-refactor-u2-fixtures --template-file test/sandbox/u2-fixtures.yaml \
     --parameter-overrides CreateIndex=true CreateDefaultView=true --tags project=tag-refactor-fixture
   ```

   索引の中身がそろうまで最大1時間ほどかかります。1時間以上おいてから検証します。
5. 読み取り用のロール R2 を引き受けるプロファイル(例 `tag-refactor-readonly`)を手元の AWS の設定に足します。
6. U5 では、U1 のスタックの R2 に影響分析の読み取りの権限を足し、U5 の確認用のスタック(`test/sandbox/u5-fixtures.yaml`)を作ります。どちらも、まず変更セットを作るだけにして、中身を人が確かめてから実行します(手順は U5 のインフラの設計 [`cicd-pipeline.md`](aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md))。

   ```bash
   # U1 のスタック: R2 のインラインポリシーだけが変わり、ロールは作り直されない(Replacement が False)ことを確かめる
   aws cloudformation create-change-set --stack-name tag-refactor-u1-fixtures --change-set-name u5-r2 \
     --template-body file://test/sandbox/u1-fixtures.yaml --parameters ParameterKey=SsoPermissionSetName,UsePreviousValue=true \
     --capabilities CAPABILITY_IAM --tags Key=project,Value=tag-refactor-fixture --region ap-northeast-1 --profile tag-refactor-sandbox
   # U5 のスタック: ロール RU5 とそれが読むキュー QU5、バケット BU5、トピック TU5、シークレット SU5 とそれぞれのポリシー(8個)
   # KMS と SQS はキーポリシーとキューポリシーで aws:ResourceTag を使うことを拒むため、キーとキューポリシーは作らない(偽の AWS のテストだけで確かめる)
   aws cloudformation create-change-set --stack-name tag-refactor-u5-fixtures --change-set-type CREATE --change-set-name u5-create \
     --template-body file://test/sandbox/u5-fixtures.yaml --parameters ParameterKey=SsoPermissionSetName,ParameterValue=<U1 と同じ値> \
     --capabilities CAPABILITY_IAM --tags Key=project,Value=tag-refactor-fixture --region ap-northeast-1 --profile tag-refactor-sandbox
   aws cloudformation describe-change-set --stack-name <スタック名> --change-set-name <変更セット名> --region ap-northeast-1 --profile tag-refactor-sandbox
   # 人が中身を確かめて許可したら実行する。許可しないときは delete-change-set で消す
   aws cloudformation execute-change-set --stack-name <スタック名> --change-set-name <変更セット名> --region ap-northeast-1 --profile tag-refactor-sandbox
   ```

   シークレットは月 0.40 USD で、時間で割った分がかかります。テスト用のルールファイルは `test/sandbox/u5-rules.yaml`(`env` を `Environment` に、`team` を `Team` に変える)です。

### 検証

```bash
TAG_REFACTOR_SANDBOX_ACCOUNT_ID=<検証用アカウントの ID> bash scripts/verify.sh
```

`scripts/verify.sh` は次を行います。

1. `scripts/verify-u1.sh`(すべてのテストと、U1 の結合テスト)を動かします。
2. 管理者のプロファイルの呼び出し元のアカウントを `TAG_REFACTOR_SANDBOX_ACCOUNT_ID` と比べ、違えば止めます。
   続けて U3 の確認をします。U1 のスタック(`tag-refactor-u1-fixtures`)があることを確かめ、`tag-refactor` をビルドし、読み取り用のプロファイルで `rules suggest --region ap-northeast-1 --output <一時フォルダのファイル>` を動かします。終了コードが 0、標準出力が空、下書きの先頭が `# generated by tag-refactor`、`env` と `Env` が1つのまとまりであることを確かめ、`scan --rules` でその下書きを読み込めることを確かめます。一時フォルダはスクリプトの終わりに必ず消します。
   続けて U4 の確認をします。U4 の結合テスト(`internal/cli/integration_u4_test.go`)を読み取り用のプロファイルで動かし、`plan` を `--format json --out` で1回、一覧、差分、Markdown で1回ずつ動かして、4つの形式と保存した変更案の件数が一致すること、保存した変更案を読み込めること、`env` と `Env` から `Environment` への名前の変更と `Owner` の追加があることを確かめます。テストが飛ばされたとき、またはこのテストの `PASS` の行がちょうど1つでないときは失敗にします。
   続けて U5 の確認をします。U5 のスタック(`tag-refactor-u5-fixtures`)が完成していることと4つの出力の ARN、U1 のスタックの R2 に `iam:GetAccountAuthorizationDetails` があることを確かめ、U5 の結合テスト(`internal/cli/integration_u5_test.go`)を読み取り用のプロファイルで動かします。ロール RU5 のインラインポリシー、3つのリソースポリシー(KMS のキーポリシーと SQS のキューポリシーは除く)、U1 の P1 が影響ありとして出ること、リソースポリシーの指摘がそのリソース自身の変更だけを指すこと、影響分析が完全なことを確かめます(このテストの `PASS` の行がちょうど1つでないときは失敗)。最後に、同じ `plan` を `--debug` 付きで5回動かし、操作ごとの標本の数と L95、IAM とリソースポリシーの L95 の最大、再試行が起きた呼び出しの数を表示します。10分の見積もりへの当てはめはチェックポイントで行います。
3. U2 のスタックと `ap-northeast-1` の索引(種類 `LOCAL`、状態 `ACTIVE`)と条件のない既定のビューを確かめ、全リージョンの索引の種類を表示します。集約の索引があるときは、前提と違うので止めます。
4. `ap-northeast-1` にタグのない SQS のキュー(`tag-refactor-u2-untagged-<乱数>`)を作ります。スクリプトの終わりに必ず消します(消せなかったときは名前を表示します)。
5. Resource Explorer の `tag:none` にキューが現れるまで、1分ごとに最大30分待ちます。現れなければ失敗にします。索引の中身がそろうまで数時間かかることがあるので、時間をおいて再実行してください。
6. U2 の結合テスト(`internal/cli/integration_u2_test.go`)を読み取り用のプロファイルで動かします。飛ばされたテストがあれば失敗にします。
7. `--debug` の行から求めた操作ごとの95パーセンタイル(L95)と、1万件のときの10分の見積もりを表示します。

| 環境変数 | 意味 |
|---|---|
| `TAG_REFACTOR_SANDBOX_ACCOUNT_ID` | 検証用アカウントの ID(必須) |
| `TAG_REFACTOR_SANDBOX_PROFILE` | 管理者のプロファイル(既定 `tag-refactor-sandbox`) |
| `TAG_REFACTOR_IT_PROFILE` | 読み取り用のプロファイル(既定 `tag-refactor-readonly`) |

スクリプトは認証情報を表示しません。AWS の CLI の出力からは、ARN、名前、状態の値だけを読みます。

### 後片付け

Construction の終わりに、人の指示で U5、U2、U1 の順にスタックを消します。U5 のバケットは空のままなので、スタックと一緒に消えます。KMS のキーは7日の待ち期間のあとに消えます。シークレットがスタックの削除で消えることは、削除のときに確かめます。U2 のスタックを消すと、テンプレートで作った索引とビューも消えます(`false` を渡して作らなかったものは残ります)。
