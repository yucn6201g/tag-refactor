# tag-refactor

tag-refactor is a command-line tool that finds AWS tag keys written in
different ways (for example `env` and `Env`) and plans how to unify them,
showing which IAM policies use the tags that would change. It reads every
enabled region, finds resources that were never tagged, and counts missing
required tags. It only reads from AWS. (The rest of this README is in Japanese.)

## できること(U1 と U2)

AWS への書き込みは一切しません。

| コマンド | できること |
|---|---|
| `scan` | アカウントで有効なすべてのリージョンを読み(`--region` で絞れます)、書き方の違うタグのキーをまとめて示します。大文字・小文字と区切り文字(`-`、`_`、空白)の違いに加えて、略語の辞書(`env`→`environment`、`prod`→`production`、`dev`→`development`、`stg`→`staging`。キー全体が一致するときだけ)でまとめます。辞書を使って初めてまとまったものには `(abbreviation match)` の印を付け、終了コードの理由にしません。`--rules` のルールファイルの `required` のキーが付いていないリソースの数、一度もタグが付いていないリソース、タグを変更できない種類も示します。`aws:` で始まるキーは対象にしません。 |
| `plan` | ルールファイルの別名(例: `env` を `Environment` にする)から、キー名の変更案を作ります。値が違ってぶつかるものは変えずに示します。さらに、ユーザー・ロール・グループに付いている、利用者が作った IAM の管理ポリシーのうち、`<前置>:ResourceTag/<キー>` の条件で変更するキーを使っているものを示します。`scan` と同じく全リージョンを読みます(一度もタグが付いていないリソースは探しません)。 |

調べる範囲は次のとおりです。どちらのコマンドも、調べた範囲を `Checked:` で始まる1行で標準エラーに出します。

- タグの付いた(または、かつて付いていた)リソースは、各リージョンのタグ付けの API(`tag:GetResources`)で読みます。リージョンは同時に8つまで読みます。読めなかったリージョン(拒否、再試行を使い切った失敗、時間切れ)があると、結果の先頭に `! Results are incomplete` とリージョンと理由を出し、終了コードは 1 です。
- 一度もタグが付いていないリソースは、タグ付けの API が返さないので、`scan` だけが次の順で探します。
  1. Resource Explorer に集約の索引があれば、そのリージョンで1回だけ探し、結果をリージョンに振り分けます。
  2. それ以外のリージョンは、AWS Config がすべての種類を記録していれば Config で、そうでなければ Resource Explorer(そのリージョンの索引と、絞り込みの条件のない既定のビュー)で探します。Config が一部の種類だけを記録しているときは両方を使います。
  3. どちらも使えないリージョンは、理由とともに `Untagged-resource discovery: partial`(一部)または `unavailable`(すべて)と示します。これだけでは終了コードは変わりません。このとき必須タグの件数は下限です(`Required tag counts are a lower bound.`)。
- IAM のグローバルな種類(ユーザー、グループ、ロール、管理ポリシー)は、一度もタグが付いていないリソースの検出の対象外です。AWS Config は既定ではこれらを記録せず、Resource Explorer は IAM のタグを索引しないためです。
- `plan` の影響分析が読むのは、ユーザー・ロール・グループに付いている、利用者が作った IAM の管理ポリシーだけです。インラインポリシー、リソースベースのポリシー、AWS の管理ポリシー、組織の SCP は読みません。「0 affected」は、この範囲での結果です。変更がないときは、ポリシーを読みません。

まだできないこと(後の作業単位): 値の変更、必須タグを付ける変更、変更の適用と取り消し、JSON などの出力形式、日本語の表示、色、インラインポリシーやリソースポリシーの影響分析。

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

# ルールファイルから変更案と影響を見る(書き換えはしない)
tag-refactor plan --rules tagrules.yaml --profile dev

# AWS の呼び出しごとに1行(操作、リージョン、時間、再試行の回数)を標準エラーに出す
tag-refactor scan --debug --region us-east-1
```

| オプション | 意味 |
|---|---|
| `--profile` | 使う AWS のプロファイル。省略すると AWS の標準の設定(環境変数など)に従います。 |
| `--region` | 読むリージョン。繰り返して複数を指定できます(同じ値は1つにまとめます)。省略するとアカウントで有効なすべてのリージョンを読みます。アカウントの確認(STS、EC2、IAM)は、最初の `--region`、またはプロファイルか `AWS_REGION` のリージョンで行います。有効でない値が1つでもあれば、リソースを読む前にエラーにします。 |
| `--rules` | ルールファイル。`plan` では必須、`scan` では省略できます(省略すると `Required tags missing: not checked (no rule file)`)。 |
| `--debug` | 呼び出しの記録を標準エラーに出します。タグの値、ポリシーの中身、問い合わせの文、認証情報は出しません。 |

認証情報を受け取るオプションはありません。認証情報は AWS の SDK が、環境変数、共有の設定ファイル、SSO などの標準の方法で読みます。実行の途中で認証情報の期限が切れた場合は、すべての読み取りを止め、途中までの結果は出さずに終了コード 1 で終わります。

AWS の呼び出しは、呼び出し回数の上限などの一時的な失敗のとき、待ち時間を延ばしながら最大10回まで試します(1回の待ちは最大20秒、1回の試行は30秒まで、再試行を含めた1回の呼び出しは5分まで)。

### ルールファイル

```yaml
version: 1
keys:
  Environment:
    aliases: [env, Env]
required:
  Owner:
    default: platform
```

- `keys` の各キーの `aliases` は `plan` が使います。
- `required` のキーの名前は `scan` が使い、そのキーが付いていないリソースの数を数えます(大文字・小文字を区別します)。`default` と `when` は形だけを確かめます。`aws:` で始まる `required` のキーは無視して警告します。`plan` は `required` をまだ使わず、「この版では使わない」と警告します。
- `values`、`remove` は形を確かめたうえで「この版では使わない」と警告します。
- 知らないキー、型の誤り、YAML の誤りは、ファイル名、行番号、キー、直し方を示して止まります。
- `keys` がない、または空のときは、`no key rules` の警告を出します(`plan` は変更案を作りません)。
- アンカー(`&`)、別名(`*`)、統合キー(`<<`)、タグ(`!`)、重複したキー、64 KiB を超えるファイルは受け付けません。
- 入れ子が 32 段を超えるファイル(括弧 `[`、`{` の入れ子、またはインデントの入れ子)は、YAML として読む前に行番号を示して止めます。引用符の中、コメント、テキストの中の括弧は入れ子に数えませんが、それらの括弧が合わせて 1024 個を超えるファイルも受け付けません(タグのキーと値に括弧は使えません)。
- CRLF の改行のファイルも同じに読めます。

### 出力と終了コード

- 標準出力には結果だけを出します。アカウントとリージョン、進み具合、警告、エラー、`Checked:` の行、かかった時間、`Exit code` の行は標準エラーに出します。
- 進み具合は、標準エラーが端末なら同じ場所を書き換えて示します(リージョンごとの状態と経過時間)。端末でないとき、`TERM=dumb` のとき、Windows で仮想端末の処理を有効にできないときは、状態が変わるたびに1行ずつ追記します。標準出力をファイルに向けても、標準エラーが端末なら書き換えます。
- 記号: `~` 名前の変更、`!` 影響あり、`?` 判定できない(キーにポリシー変数を含む条件)。色は使いません。
- AWS から読んだ文字列(ARN、種類の名前など)の制御文字や双方向の制御文字は、`\u{…}` の形に置き換えて出します。

| 終了コード | 意味 |
|---|---|
| 0 | 問題なし(`scan`: 表記ゆれも必須タグの欠落もない、`plan`: 変更不要) |
| 1 | エラー、読めなかったリージョンがある、または影響分析が不完全(ポリシーを読めなかった) |
| 2 | 確認が必要(表記ゆれ、必須タグの欠落、変更、ぶつかり、影響のどれかがある) |

一度もタグが付いていないリソースを探せなかったこと、見つかったこと、タグを変更できない種類があることだけでは、終了コードは変わりません。

エラーは2行(`Error:` 何が起きたか、`Fix:` どう直すか)で出します。AWS の SDK のエラー文はそのまま出しません。

## 必要な権限

読み取りの権限だけで動きます。必要な IAM の操作は [`docs/iam/readonly-policy.json`](docs/iam/readonly-policy.json) にまとめています。この例の `<account-id>` は、使うアカウントの ID(12桁)に置き換えてください。`iam:GetPolicyVersion` は、そのアカウントの、利用者が作った管理ポリシー(`arn:aws:iam::<account-id>:policy/*`)だけに絞っています。ほかの操作はリソースを絞れないため `*` です。

| IAM の操作 | 使う場面 |
|---|---|
| `ec2:DescribeRegions` | アカウントで有効なリージョンの一覧と、指定したリージョンが使えるかの確認。指定したリージョンの STS が認証情報を拒否したときは、`us-east-1` から使えるリージョンを確かめ直し、有効にしていないオプトインのリージョンなら「そのリージョンは使えない」と示します |
| `tag:GetResources` | タグの付いたリソースの読み取り |
| `iam:ListPolicies` | 付いている、利用者が作った管理ポリシーの一覧(`plan` で変更があるときだけ) |
| `iam:GetPolicyVersion` | そのポリシーの既定の版の中身(`plan` で変更があるときだけ) |
| `config:DescribeConfigurationRecorders` | AWS Config が何を記録しているか(`scan`) |
| `config:DescribeConfigurationRecorderStatus` | AWS Config が記録しているか(`scan`) |
| `config:SelectResourceConfig` | AWS Config に記録されたリソースとタグの問い合わせ(`scan`) |
| `resource-explorer-2:ListIndexes` | Resource Explorer の索引と、集約の索引の有無(`scan`) |
| `resource-explorer-2:GetDefaultView` | リージョンの既定のビュー(`scan`) |
| `resource-explorer-2:GetView` | 既定のビューに絞り込みの条件があるか(`scan`) |
| `resource-explorer-2:Search` | API の `ListResources` に AWS が求める IAM の操作。タグのないリソースを件数の上限なく読みます(`scan`)。1,000 件までに限られる API の `Search` は呼びません |

`sts:GetCallerIdentity`(呼び出し元の確認)は IAM の権限が要りません。AWS Config や Resource Explorer の記録・索引・ビューを作ったり変えたりする操作は呼びません。使えないときは、理由(足りない IAM の操作の名前、`no default Resource Explorer view` など)を示すだけです。`iam:ListPolicies` や `iam:GetPolicyVersion` が拒否された場合、`plan` は変更案を出したうえで「影響分析は不完全」と示し、終了コード 1 で終わります。

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
```

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

### 検証

```bash
TAG_REFACTOR_SANDBOX_ACCOUNT_ID=<検証用アカウントの ID> bash scripts/verify.sh
```

`scripts/verify.sh` は次を行います。

1. `scripts/verify-u1.sh`(すべてのテストと、U1 の結合テスト)を動かします。
2. 管理者のプロファイルの呼び出し元のアカウントを `TAG_REFACTOR_SANDBOX_ACCOUNT_ID` と比べ、違えば止めます。
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

Construction の終わりに、人の指示で U2、U1 の順にスタックを消します。U2 のスタックを消すと、テンプレートで作った索引とビューも消えます(`false` を渡して作らなかったものは残ります)。
