# tag-refactor

tag-refactor is a command-line tool that finds AWS tag keys written in
different ways (for example `env` and `Env`) and plans how to unify them,
showing which IAM policies use the tags that would change. It only reads from
AWS. (The rest of this README is in Japanese.)

## U1(最小の一本)でできること

このバージョンは、最初の作業単位 U1 の最小版です。AWS への書き込みは一切しません。

| コマンド | できること |
|---|---|
| `scan` | 1つのリージョンの、タグの付いたリソースを読み、大文字・小文字と区切り文字(`-`、`_`、空白)だけが違うタグのキーをまとめて示します。`aws:` で始まるキーは対象にしません。 |
| `plan` | ルールファイルの別名(例: `env` を `Environment` にする)から、キー名の変更案を作ります。値が違ってぶつかるものは変えずに示します。さらに、ユーザー・ロール・グループに付いている、利用者が作った IAM の管理ポリシーのうち、`<前置>:ResourceTag/<キー>` の条件で変更するキーを使っているものを示します。 |

まだできないこと(後の作業単位): 全リージョンの読み取り、値の変更、必須タグ、変更の適用と取り消し、JSON などの出力形式、日本語の表示、インラインポリシーやリソースポリシーの影響分析。

## インストール

Go(最新の安定版、または1つ前の版)で、手元でビルドします。

```bash
go install github.com/yucn6201g/tag-refactor/cmd/tag-refactor@latest
```

公開のバイナリ(GitHub Releases)は、第1弾(`v0.1.0`)で用意します。

## 使い方

```bash
# 1つのリージョンのタグのキーの表記ゆれを調べる
tag-refactor scan --profile dev --region us-east-1

# ルールファイルから変更案と影響を見る(書き換えはしない)
tag-refactor plan --rules tagrules.yaml --profile dev --region us-east-1

# AWS の呼び出しごとに1行(操作、リージョン、時間、再試行の回数)を標準エラーに出す
tag-refactor scan --debug --region us-east-1
```

| オプション | 意味 |
|---|---|
| `--profile` | 使う AWS のプロファイル。省略すると AWS の標準の設定(環境変数など)に従います。 |
| `--region` | 読むリージョン。省略するとプロファイルか `AWS_REGION` のリージョンを使います。 |
| `--rules` | `plan` のルールファイル(必須)。 |
| `--debug` | 呼び出しの記録を標準エラーに出します。タグの値、ポリシーの中身、認証情報は出しません。 |

認証情報を受け取るオプションはありません。認証情報は AWS の SDK が、環境変数、共有の設定ファイル、SSO などの標準の方法で読みます。

### ルールファイル

```yaml
version: 1
keys:
  Environment:
    aliases: [env, Env]
```

- U1 で使うのは `keys` の各キーの `aliases` だけです。`values`、`required`、`remove` は形を確かめたうえで「この版では使わない」と警告します。
- 知らないキー、型の誤り、YAML の誤りは、ファイル名、行番号、キー、直し方を示して止まります。
- アンカー(`&`)、別名(`*`)、統合キー(`<<`)、タグ(`!`)、重複したキー、64 KiB を超えるファイルは受け付けません。
- 入れ子が 32 段を超えるファイル(括弧 `[`、`{` の入れ子、またはインデントの入れ子)は、YAML として読む前に行番号を示して止めます。引用符の中、コメント、テキストの中の括弧は入れ子に数えませんが、それらの括弧が合わせて 1024 個を超えるファイルも受け付けません(タグのキーと値に括弧は使えません)。
- CRLF の改行のファイルも同じに読めます。

### 出力と終了コード

- 標準出力には結果の表と `Summary` の行だけを出します。アカウントとリージョン、進み具合、警告、エラー、かかった時間、`Exit code` の行は標準エラーに出します。
- 記号: `~` 名前の変更、`!` 影響あり、`?` 判定できない(キーにポリシー変数を含む条件)。色は使いません。

| 終了コード | 意味 |
|---|---|
| 0 | 問題なし(`scan`: 表記ゆれなし、`plan`: 変更不要) |
| 1 | エラー、または影響分析が不完全(ポリシーを読めなかった) |
| 2 | 確認が必要(表記ゆれ、変更、ぶつかり、影響のどれかがある) |

エラーは2行(`Error:` 何が起きたか、`Fix:` どう直すか)で出します。AWS の SDK のエラー文はそのまま出しません。

## 必要な権限

読み取りの権限だけで動きます。必要な操作は [`docs/iam/readonly-policy.json`](docs/iam/readonly-policy.json) にまとめています。

| 操作 | 使う場面 |
|---|---|
| `ec2:DescribeRegions` | 指定したリージョンがアカウントで使えるかの確認。指定したリージョンの STS が認証情報を拒否したときは、`us-east-1` から使えるリージョンを確かめ直し、有効にしていないオプトインのリージョンなら「そのリージョンは使えない」と示します |
| `tag:GetResources` | タグの付いたリソースの読み取り |
| `iam:ListPolicies` | 付いている、利用者が作った管理ポリシーの一覧(`plan` で変更があるときだけ) |
| `iam:GetPolicyVersion` | そのポリシーの既定の版の中身(`plan` で変更があるときだけ) |

`sts:GetCallerIdentity`(呼び出し元の確認)は IAM の権限が要りません。`iam:ListPolicies` や `iam:GetPolicyVersion` が拒否された場合、`plan` は変更案を出したうえで「影響分析は不完全」と示し、終了コード 1 で終わります。

## 開発

```bash
export GOTOOLCHAIN=local
go test -mod=readonly ./...                       # 受け入れ条件、単体、性能、出力の守り
go test -mod=readonly -coverpkg=./cmd/tag-refactor/...,./internal/... -coverprofile=coverage.out ./cmd/tag-refactor/... ./internal/...
go tool cover -func=coverage.out                  # 80% 以上
gofmt -l .
golangci-lint run
go vet -tags integration ./...
go tool govulncheck ./...
```

- テストは AWS を呼びません。HTTP の層の偽の AWS(`internal/testutil`)を使い、手元の `~/.aws` も読みません。
- CI(`.github/workflows/ci.yml`)は Linux、macOS、Windows と Go の2つの版でテストし、カバレッジ、lint、govulncheck、モジュールの確認、ビルドを行います。

## 検証用アカウントでの確認(U1 のチェックポイント)

実際の AWS アカウントを使う確認は、手元だけで行います(CI では行いません)。少額の費用がかかる場合があります。

1. 確認用のリソースを `test/sandbox/u1-fixtures.yaml`(CloudFormation、`us-east-1`)で作ります。手順、期待する結果、削除の方法は、インフラの設計 [`infrastructure-specification.md`](aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u1-walking-skeleton/infrastructure-design/infrastructure-specification.md) にあります。
2. 読み取り用のロール R2 を引き受けるプロファイル(例 `tag-refactor-readonly`)を手元の AWS の設定に足します。
3. 検証用アカウントのテストを動かします。ルールファイルは `test/sandbox/u1-rules.yaml` です。

   ```bash
   TAG_REFACTOR_IT_PROFILE=tag-refactor-readonly go test -mod=readonly -tags integration -run 'TestIntegration' ./internal/cli/...
   ```

4. 確認が終わったらスタックを削除します。
