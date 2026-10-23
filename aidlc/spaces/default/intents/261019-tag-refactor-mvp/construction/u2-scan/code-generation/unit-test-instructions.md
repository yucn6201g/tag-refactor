# 単体テストの手順: U2 収集

## 準備

- U1 と同じ(Go の最新の安定版、`GOTOOLCHAIN=local`、Go の標準の `testing` だけ)。追加の設定ファイルは要らない。
- AWS の認証情報は要らない。テストは `internal/testutil` の偽の AWS(HTTP の層)で動く。U2 では、偽の AWS が AWS Config(見出し `X-Amz-Target`)と Resource Explorer(URL のパス)の要求も受ける。
- レース検出(`-race`)は cgo と C のコンパイラが要る。手元の Windows で使えない場合は、CI の Linux の `race` ジョブで確かめる。

## この作業単位のテストの実行

U2 で変える・足すパッケージだけを指定して実行します。

最初の受け入れ条件のテストより前に動くことを確かめるコマンド(Step 2):

```bash
go test -mod=readonly ./internal/testutil/...
```

U2 の受け入れ条件のテスト(Step 3 で書き、Step 12 で通す):

```bash
go test -mod=readonly -run 'TestAcceptanceU2' ./internal/cli/...
```

U2 のすべての単体テスト(Step 13、Step 14、Step 19):

```bash
go test -mod=readonly ./internal/testutil/... ./internal/errkind/... ./internal/awsaccess/... ./internal/inventory/... ./internal/display/... ./internal/cli/...
```

性能・決定性・取り消しのテスト(Step 14):

```bash
go test -mod=readonly -run 'TestPerformanceU2|TestDeterministic|TestCancel' ./internal/cli/... ./internal/inventory/...
```

レース検出(Step 14、Step 19。Linux など cgo が使える環境):

```bash
CGO_ENABLED=1 go test -mod=readonly -race ./internal/inventory/... ./internal/awsaccess/... ./internal/display/... ./internal/cli/...
```

カバレッジ(80%以上。測る範囲は U1 と同じ `cmd` と `internal` のすべて):

```bash
go test -mod=readonly -coverpkg=./cmd/tag-refactor/...,./internal/... -coverprofile=coverage.out ./cmd/tag-refactor/... ./internal/...
go tool cover -func=coverage.out
```

検証用アカウントの結合テスト(手元だけ。CI では動かさない。ふつうは `scripts/verify.sh` から動かす):

```bash
TAG_REFACTOR_IT_PROFILE=tag-refactor-readonly TAG_REFACTOR_IT_UNTAGGED_ARN=<タグのないキューの ARN> go test -mod=readonly -tags integration -run '^TestIntegrationU2' ./internal/cli/...
```

## カバレッジの目標

- Go の文単位で80%以上(`cmd` と `internal` のすべて。配線の部分も外さない)。下限を下げて通すことはしない。

## 差し替えの使い方

| 層 | 使う場所 | 使い方 |
|---|---|---|
| HTTP の層の偽の AWS(`internal/testutil`) | 受け入れ条件のテスト、`awsaccess` の単体テスト、性能・決定性・取り消しのテスト | 操作ごとに決まった応答(ページ分け、拒否、呼び出し回数の上限、401、想定外の応答、遅延)を登録する。受け取った API の名前を記録し、操作の表にない操作や API の `Search` が来たらテストを失敗させる。処理中の要求の数を数える。`MaxResults` が1000の `ListResources` には `NextToken` を返さない |
| 契約のインターフェースの差し替え | `inventory` の単体テスト | TagReader、ConfigReader、ExplorerReader をメモリの中のデータで置き換える |
| 端末の判定の差し込み口 | `display` の単体テスト | 「端末である」「仮想端末の処理を有効にできる」を差し込んで、書き換えの出力を固定する |
| 時計と待ち時間の差し込み | 再試行と時間の上限のテスト、進み具合のテスト | 実時間を待たずに、待ち時間の増え方と上限の超過を起こす |
| ページの大きさ、件数の差し替え口(パッケージの中の変数) | 複数ページのテスト、`-race` のときの性能のテスト | テストの中だけで小さな値にする |

## テストデータ

- テストデータは、テストのコードの中で組み立てる(17リージョン、リソース、タグ、Config の結果の JSON、Resource Explorer の結果、ポリシー文書)。
- 制御文字のテストでは、0x9B、U+202E、改行、不正な UTF-8 を含む ARN と種類の名前を使う。
- 認証情報の漏れのテストでは、U1 と同じ見分けやすい偽のアクセスキーを使う。

## テストの数の目安(Standard)

| パッケージ | 単体テスト |
|---|---|
| `internal/awsaccess` | 7〜8(操作の表、再試行と上限、401 の2つの場合、Config の結果の検査、ページ読み) |
| `internal/inventory` | 8(分かれ道、集約の索引、ビューの条件、IAM の除外、重複と優先、辞書、欠落、表の形) |
| `internal/display` | 5〜7(書き換え、追記、判定、置き換え、新しい行) |
| `internal/cli` | 受け入れ条件 約15件 + 5〜6 |
| `internal/testutil` | 3〜4(パスからの操作の名前、処理中の数、`MaxResults` の振る舞い) |
| `internal/errkind` | 1〜2 |
