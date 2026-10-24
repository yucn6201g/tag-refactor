# 単体テストの手順: U4 変更案

## 準備

- U1〜U3 と同じ(Go の最新の安定版、`GOTOOLCHAIN=local`、Go の標準の `testing` だけ)。追加の設定ファイルは要らない。
- 変更案の作成、4つの形式、言語、色、保存と読み込みのテストは AWS を呼ばない。`plan` と `scan` のコマンドとしてのテストは U2 の偽の AWS(HTTP の層)で動く。AWS の認証情報は要らない。
- ファジングは Go の標準(`testing.F`)。CI では種の入力だけが普通のテストとして動く。
- 言語と色のテストは、環境変数(`LANG`、`LC_ALL`、`LC_MESSAGES`、`NO_COLOR`、`TERM`)を `t.Setenv` で固定し、端末の判定と Windows の表示言語の関数を差し替える。

## この作業単位のテストの実行

U4 で変える・足すパッケージだけを指定して実行します。

最初の受け入れ条件のテストより前に動くことを確かめるコマンド(Step 1):

```bash
go test -mod=readonly ./internal/planner/... ./internal/display/...
```

U4 の受け入れ条件のテスト(Step 2 で書き、Step 10 で通す):

```bash
go test -mod=readonly -run 'TestAcceptanceU4' ./internal/cli/...
```

U4 のすべての単体テスト(Step 11、Step 12、Step 14):

```bash
go test -mod=readonly ./internal/planner/... ./internal/display/... ./internal/fileout/... ./internal/cli/... ./cmd/tag-refactor/...
```

件数の一致、ファジングの種の入力、性能のテスト(Step 12):

```bash
go test -mod=readonly -run 'TestFormatCountsAgree|FuzzLoadPlan|TestPerformanceU4' ./internal/planner/... ./internal/display/...
```

期待のファイルの作り直し(期待が変わったときだけ。作り直したら差分を人が見る):

```bash
go test -mod=readonly -run 'TestGolden' ./internal/display/... -update
```

ファジング(手元だけ。5分。Step 14):

```bash
go test -mod=readonly -run '^$' -fuzz FuzzLoadPlan -fuzztime 5m ./internal/planner/
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
- 幅の表を作るスクリプト(`internal/display/gen_width.go`)は `//go:build ignore` のため計測に入らない。

## 差し替えの使い方

| 層 | 使う場所 | 使い方 |
|---|---|---|
| 収集の結果とルール | `planner` の単体テスト、件数の一致、性能 | `inventory.Snapshot` と `rules.RuleSet` をテストの中で組み立てる(ルールは文字列から `rules.Parse`) |
| 影響分析 | `planner` と `cli` のテスト | `planner.ImpactAnalyzer` の偽物(決まった finding を返す。U1 のまま) |
| HTTP の層の偽の AWS(`internal/testutil`) | 受け入れ条件のテスト、操作の集まり、Ctrl+C | U2 のまま。呼び出しの途中で `context` を取り消す口を使う |
| 端末、環境変数、Windows の表示言語 | `display` の言語と色のテスト | 判定の関数を差し替える。Windows のランナーだけ、実際の関数を呼ぶテスト(ビルドタグで分ける) |
| 名前を付ける関数(ハードリンク、名前の変更) | `fileout` のテスト | U3 の `writeDraftFileWith` と同じく差し替える |
| 時計と乱数 | 変更案の ID、作成日時、件数の一致 | 決まった日時と乱数の種を渡す |
| 一時フォルダ | `--out`、`LoadPlan`、ファジング | テストごとに作り、終わったら消す(Go の `t.TempDir`) |

## テストデータ

- 期待のファイルは `internal/display/testdata/plan/`(4形式 × 2言語、保存した変更案)と `internal/display/testdata/scan/`(JSON と Markdown × 2言語)に置く。`.gitattributes` の `* text=auto eol=lf` で LF に固定される。
- ファジングの種は `internal/planner/testdata/fuzz/FuzzLoadPlan/` に置き、`**/testdata/fuzz/** -text` で守られる。
- 件数の一致のテストは乱数の種を固定する(CI で毎回同じ入力)。

## テストの数の目安(Standard)

| パッケージ | 単体テスト |
|---|---|
| `internal/planner` | 変更の作り方 8、集計と終了コード 6、保存と読み込み 8、`SafeJSONWriter` 5(ファジング、件数の一致、性能は別) |
| `internal/display` | 4形式の期待のファイル 8、Markdown のエスケープ 6、色 6、言語 6、幅 7、文言の一覧 2 |
| `internal/fileout` | 6 |
| `internal/cli` | 受け入れ条件 約15件 + 8(Ctrl+C、操作の集まり、認証情報、使い方の誤り) |
| `cmd/tag-refactor` | 2(シグナルの受け取り) |
