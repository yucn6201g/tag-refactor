# 単体テストの手順: U5 影響分析

## 準備

- U1〜U4 と同じ(Go の最新の安定版、`GOTOOLCHAIN=local`、Go の標準の `testing` だけ)。追加の設定ファイルは要らない。
- 影響分析の判定と文書の読み取りのテストは AWS を呼ばない(偽の読み取りの口)。読み取りの口と `plan` のコマンドとしてのテストは、HTTP の層の偽の AWS(`internal/testutil`。U5 で IAM のインラインポリシーと5つのサービスを足す)で動く。AWS の認証情報は要らない。
- ファジングは Go の標準(`testing.F`)。CI では種の入力だけが普通のテストとして動く。
- 言語は、受け入れ条件と表示のテストで `--lang en` と `t.Setenv`(`LC_ALL`、`LANG`)で固定する(U4 と同じ)。

## この作業単位のテストの実行

U5 で変える・足すパッケージだけを指定して実行します。

最初の受け入れ条件のテストより前に動くことを確かめるコマンド(Step 1):

```bash
go test -mod=readonly ./internal/impact/... ./internal/awsaccess/... ./internal/testutil/...
```

U5 の受け入れ条件のテスト(Step 2 で書き、Step 10 で通す):

```bash
go test -mod=readonly -run 'TestAcceptanceU5' ./internal/cli/...
```

U5 のすべての単体テスト(Step 11、Step 12、Step 14):

```bash
go test -mod=readonly ./internal/impact/... ./internal/awsaccess/... ./internal/planner/... ./internal/display/... ./internal/cli/... ./internal/testutil/...
```

検出率100%の表、ファジングの種の入力、大きな偽の AWS のテスト(Step 11、Step 12):

```bash
go test -mod=readonly -run 'TestDetectionTable|FuzzParsePolicy|TestLargeAnalysisU5' ./internal/impact/...
```

同じ結果のテストをレース検出つきで(Linux など cgo が使える環境だけ。CI の `race` ジョブと同じ):

```bash
go test -mod=readonly -race -run 'TestLargeAnalysisU5Deterministic' ./internal/impact/...
```

ファジング(手元だけ。5分。Step 14):

```bash
go test -mod=readonly -run '^$' -fuzz FuzzParsePolicy -fuzztime 5m ./internal/impact/
```

カバレッジ(80%以上。測る範囲は U1 と同じ `cmd` と `internal` のすべて):

```bash
go test -mod=readonly -coverpkg=./cmd/tag-refactor/...,./internal/... -coverprofile=coverage.out ./cmd/tag-refactor/... ./internal/...
go tool cover -func=coverage.out
```

検証用アカウントの確認(手元だけ。U2 で承認した検証コマンドの中で動く。U5 の確認用のスタックと R2 の更新が済んでから):

```bash
"C:\Program Files\Git\bin\bash.exe" -c "TAG_REFACTOR_SANDBOX_ACCOUNT_ID=663798383745 bash scripts/verify.sh"
```

## カバレッジの目標

- Go の文単位で80%以上(`cmd` と `internal` のすべて。配線の部分も外さない)。下限を下げて通すことはしない。

## 差し替えの使い方

| 層 | 使う場所 | 使い方 |
|---|---|---|
| 読み取りの口 | `impact` の判定、検出率100%の表、文書の読み取り | `awsaccess` の読み取りのインターフェースの偽物(決まった文書と誤りを返す)をテストの中で組み立てる |
| HTTP の層の偽の AWS(`internal/testutil`) | `awsaccess` の口、受け入れ条件のテスト、大きなテスト、認証情報の誤り、取り消し | U2 の偽の AWS に IAM と5つのサービスを足す。一覧は API ごとの件数で分ける。回数、同時の数、遅れ、誤りの応答を決められる |
| 同時に読む数 | 枠を1にしても終わるテスト、同時の数のテスト | パッケージの中の変数で差し替える(U2 と同じ) |
| 時計と乱数 | 変更案の ID、遅れの入れ替え | 決まった日時と乱数の種を渡す |
| 一時フォルダ | `--out`、ファジング | テストごとに作り、終わったら消す(Go の `t.TempDir`) |

## テストデータ

- 検出率100%の期待する判定の表は、`internal/impact` のテストのコードの中の表(Go のスライス)に置く。表の行の数(252と32)をテストで確かめる。
- ファジングの種は `internal/impact/testdata/fuzz/FuzzParsePolicy/` に置き、`**/testdata/fuzz/** -text` で守られる。
- 大きなテストの分布(17リージョン、偏ったリージョン1つ)はテストのコードで組み立て、乱数の種を固定する。
- 検証用アカウントのテスト用のポリシーは `test/sandbox/u5-fixtures.yaml`、ルールは `test/sandbox/u5-rules.yaml`。

## テストの数の目安(Standard)

| パッケージ | 単体テスト |
|---|---|
| `internal/impact` | 検出率100%の表(252行と32行)、判定 8、結び付け 5、文書の読み取りと参照の取り出し 8、指摘のまとめと並べ方 5、使い回しの入れ物 3、大きなテスト 6(回数、10秒、同時の数、枠1、同じ結果、goroutine) |
| `internal/awsaccess` | 口ごとの読み取り 8、誤りのコード 5、例のポリシーの検査(`Resource` の形) 3 |
| `internal/planner` | 足した項目の保存と読み込み 4 |
| `internal/display` | 読めなかったものの表示 3、文言 2 |
| `internal/cli` | 受け入れ条件 6 + 認証情報の誤り 4 |
