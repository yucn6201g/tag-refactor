# 単体テストの手順: U3 ルール

## 準備

- U1・U2 と同じ(Go の最新の安定版、`GOTOOLCHAIN=local`、Go の標準の `testing` だけ)。追加の設定ファイルは要らない。
- ルールファイルの読み込み、照会、下書きのテストは AWS を呼ばない。`rules suggest` のテストは U2 の偽の AWS(HTTP の層)で動く。AWS の認証情報は要らない。
- ファジングは Go の標準(`testing.F`)。CI では種の入力だけが普通のテストとして動く。

## この作業単位のテストの実行

U3 で変える・足すパッケージだけを指定して実行します。

最初の受け入れ条件のテストより前に動くことを確かめるコマンド(Step 1):

```bash
go test -mod=readonly ./internal/rules/...
```

U3 の受け入れ条件のテスト(Step 2 で書き、Step 10 で通す):

```bash
go test -mod=readonly -run 'TestAcceptanceU3' ./internal/cli/...
```

U3 のすべての単体テスト(Step 11、Step 12、Step 14):

```bash
go test -mod=readonly ./internal/rules/... ./internal/cli/... ./internal/inventory/... ./internal/display/...
```

ファジングの種の入力と読み戻し、性能のテスト(Step 12):

```bash
go test -mod=readonly -run 'FuzzParse|TestRoundTrip|TestPerformanceU3' ./internal/rules/...
```

ファジング(手元だけ。5分。Step 14):

```bash
go test -mod=readonly -run '^$' -fuzz FuzzParse -fuzztime 5m ./internal/rules/
```

カバレッジ(80%以上。測る範囲は U1 と同じ `cmd` と `internal` のすべて):

```bash
go test -mod=readonly -coverpkg=./cmd/tag-refactor/...,./internal/... -coverprofile=coverage.out ./cmd/tag-refactor/... ./internal/...
go tool cover -func=coverage.out
```

検証用アカウントの確認(手元だけ。U2 で承認した検証コマンドの中で動く):

```bash
"C:\Program Files\Git\bin\bash.exe" -c "TAG_REFACTOR_SANDBOX_ACCOUNT_ID=663798383745 bash scripts/verify.sh"
```

## カバレッジの目標

- Go の文単位で80%以上(`cmd` と `internal` のすべて。配線の部分も外さない)。下限を下げて通すことはしない。

## 差し替えの使い方

| 層 | 使う場所 | 使い方 |
|---|---|---|
| ルールファイルの文字列とバイト列 | `rules` の単体テスト、ファジング | テストの中で組み立てる(CRLF、UTF-16、不正な UTF-8 もバイト列として作る) |
| 下書きの入力(`SuggestInput`) | `rules` の下書きの単体テスト、性能のテスト | ARN とタグの一覧をメモリで作る。まとめる関数は `inventory.CanonicalKey` と、テスト用の単純な関数の両方を使う |
| HTTP の層の偽の AWS(`internal/testutil`) | 受け入れ条件のテスト(`rules suggest`、scan、plan) | U2 のまま。呼び出し回数の上限、リージョンの拒否、記録した操作の照合 |
| 時計の差し込み | 下書きの日時の行 | 決まった日時にして、期待のファイルと比べる |
| 一時フォルダ | `--output` のテスト、ルールファイル | テストごとに作り、終わったら消す(Go の `t.TempDir`) |

## テストデータ

- 期待する下書きは `internal/rules/testdata/` に LF で置く(`.gitattributes`)。
- ファジングの種は `internal/rules/testdata/fuzz/FuzzParse/` に置き、`.gitattributes` の `**/testdata/fuzz/** -text` で改行の変換から外す。
- 読み戻しのテストは乱数の種を固定する(CI で毎回同じ入力)。

## テストの数の目安(Standard)

| パッケージ | 単体テスト |
|---|---|
| `internal/rules` | 読み込み 8、矛盾 6、照会 6、下書き 8、文字コード 6(ファジングと読み戻し、性能は別) |
| `internal/cli` | 受け入れ条件 約8件 + 5〜6(`--output`、終了コード、Load のオプション) |
| `internal/inventory` | 1〜2(種類の照会) |
| `internal/display` | 1〜2(下書きの出力の口) |
