# 単体テストの手順: U1 最小の一本

## 準備

- Go(最新の安定版)を手元に入れる。`GOTOOLCHAIN=local` を設定し、Go が別の版を自動で取りに行かないようにする。
- テストの枠組みは Go の標準の `testing` だけを使う。追加の設定ファイルは要らない。
- AWS の認証情報は要らない。テストは `internal/testutil` の偽の AWS(HTTP の層)と、契約のインターフェースの差し替えで動く。テストの手助けが、AWS の設定ファイルの場所とプロファイルの環境変数を一時的なものに置き換え、EC2 のメタデータを無効にするため、手元の `~/.aws` は読まない。

## この作業単位のテストの実行

U1 の作業単位のパッケージだけを指定して実行します(リポジトリには、まだ U1 のパッケージしかありません。後の作業単位が増えても、このコマンドは U1 のパッケージだけを動かします)。

最初の受け入れ条件のテストより前に動くことを確かめるコマンド(Step 3):

```bash
go test -mod=readonly ./internal/testutil/...
```

受け入れ条件のテスト(Step 4 で書き、Step 13 で通す):

```bash
go test -mod=readonly -run 'TestAcceptance' ./internal/cli/...
```

U1 のすべてのテスト(Step 14、Step 18):

```bash
go test -mod=readonly ./cmd/tag-refactor/... ./internal/cli/... ./internal/display/... ./internal/awsaccess/... ./internal/inventory/... ./internal/rules/... ./internal/planner/... ./internal/impact/... ./internal/errkind/... ./internal/testutil/...
```

カバレッジ(80%以上):

```bash
go test -mod=readonly -coverpkg=./cmd/tag-refactor/...,./internal/... -coverprofile=coverage.out ./cmd/tag-refactor/... ./internal/...
go tool cover -func=coverage.out
```

検証用アカウントのテスト(手元だけ。CI では動かさない):

```bash
TAG_REFACTOR_IT_PROFILE=tag-refactor-readonly go test -mod=readonly -tags integration -run 'TestIntegration' ./internal/cli/...
```

## カバレッジの目標

- Go の文単位で80%以上(`cmd` と `internal` のすべて。配線の部分も外さない)。下限を下げて通すことはしない。

## 差し替えの使い方

| 層 | 使う場所 | 使い方 |
|---|---|---|
| HTTP の層の偽の AWS(`internal/testutil`) | 受け入れ条件のテスト、`awsaccess` の単体テスト、性能のテスト | 操作ごとに決まった応答(ページ分け、拒否、遅延、想定外の応答)を登録し、受け取った操作の名前を記録する。読み取り以外の操作が来たらテストを失敗させる |
| 契約のインターフェースの差し替え | `inventory`、`planner`、`impact` の単体テスト | TagReader、PolicyReader をメモリの中のデータで置き換える |
| 時計の差し込み | 時間の上限のテスト | 実時間を待たずに上限の超過を起こす |

## テストデータ

- テストデータは、テストのコードの中で組み立てる(リソース、タグ、ポリシー文書)。CRLF のルールファイルは、テストの中で文字列として組み立てる。
- 期待する出力を比べる固定のファイルは `testdata/` に置き、LF で保存する(`.gitattributes`)。
- 認証情報の漏れのテストでは、見分けやすい偽のアクセスキー(例 `AKIAFAKEFAKEFAKE0000`)を、一時的な AWS の設定ファイルに入れて使う。

## テストの数の目安(Standard)

| パッケージ | 単体テスト |
|---|---|
| `internal/awsaccess` | 6〜8 |
| `internal/inventory` | 5〜7 |
| `internal/rules` | 7〜8 |
| `internal/planner` | 6〜8 |
| `internal/impact` | 7〜8 |
| `internal/display` | 5〜7 |
| `internal/cli` | 受け入れ条件13件 + 5〜6 |
| `internal/errkind`、`cmd/tag-refactor` | 各2〜3 |
