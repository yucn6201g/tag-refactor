# 単体テストの手順: U8 IaC への反映

## 準備

- U1〜U7 と同じ(Go の最新の安定版、`GOTOOLCHAIN=local`、Go の標準の `testing` だけ)。追加の設定ファイルは要らない。
- `iac` は AWS を呼ばないので、AWS の認証情報は要らない。AWS を呼ばないことの確かめにだけ、HTTP の層の偽の AWS(`internal/testutil`)を立てて、要求が0件であることを見る。
- ファイルのテストは、テストごとの一時フォルダー(`t.TempDir`)にテンプレートと `.tf` を作って動かす。git のテストは、テストの中で一時のリポジトリを作る(`GIT_CONFIG_GLOBAL` と `GIT_CONFIG_NOSYSTEM` で利用者の設定に頼らない。`git` のないところでは、git を使うテストを飛ばし、git がないことの確かめは差し替えで行う)。
- 言語は、受け入れ条件と表示のテストで `--lang en` と `t.Setenv`(`LC_ALL`、`LANG`)で固定する(U4 と同じ)。

## この作業単位のテストの実行

U8 で変える・足すパッケージだけを指定して実行します。

最初の受け入れ条件のテストより前に動くことを確かめるコマンド(Step 1):

```bash
go test -mod=readonly ./internal/fileout/... ./internal/journal/... ./internal/cli/...
```

U8 の受け入れ条件のテスト(Step 2 で書き、Step 11 で通す):

```bash
go test -mod=readonly -run 'TestAcceptanceU8' ./internal/cli/...
```

書き出しの部品を直したあとの U4・U6・U7 の回帰(Step 3):

```bash
go test -mod=readonly ./internal/fileout/... ./internal/journal/... ./internal/apply/... ./internal/cli/...
```

U8 のすべての単体テスト(Step 12、Step 13、Step 15):

```bash
go test -mod=readonly ./internal/iac/... ./internal/restype/... ./internal/fileout/... ./internal/journal/... ./internal/display/... ./internal/cli/... ./internal/rules/...
```

解析の速さとメモリの計測(Step 1):

```bash
go test -mod=readonly -run '^$' -bench 'BenchmarkParse' -benchmem ./internal/iac/...
```

大きなテスト(Step 13):

```bash
go test -mod=readonly -run 'TestLargeIacU8' ./internal/iac/...
```

手元の、実際の git を使う大きなテスト(Step 13。CI では動かさない):

```bash
TAG_REFACTOR_LARGE_REAL_GIT=1 go test -mod=readonly -run 'TestLargeIacU8' ./internal/iac/...
```

レース検出つき(Linux など cgo が使える環境だけ。CI の `race` ジョブと同じ):

```bash
go test -mod=readonly -race ./internal/iac/... ./internal/fileout/... ./internal/journal/...
```

カバレッジ(80%以上。測る範囲は U1 と同じ `cmd` と `internal` のすべて。`internal/iac` 単体の値も記録する):

```bash
go test -mod=readonly -coverpkg=./cmd/tag-refactor/...,./internal/... -coverprofile=coverage.out ./cmd/tag-refactor/... ./internal/...
go tool cover -func=coverage.out
```

確かめの補助の構文と型(Step 14):

```bash
go vet scripts/u6check.go
bash -n scripts/verify.sh
```

検証用アカウントの確認(手元だけ。U2 で承認した検証コマンドの中で動く。U6 の確認用のスタックとプロファイル `tag-refactor-readonly` があること。AWS に書かない):

```bash
"C:\Program Files\Git\bin\bash.exe" -c "TAG_REFACTOR_SANDBOX_ACCOUNT_ID=<検証用アカウントの ID> bash scripts/verify.sh"
```

## カバレッジの目標

- Go の文単位で80%以上(`cmd` と `internal` のすべて。配線の部分も外さない)。U8 で足したパッケージ(`internal/iac`、`internal/restype`)だけでも80%以上(NFR10.12)。下限を下げて通すことはしない。

## 差し替えの使い方

| 層 | 使う場所 | 使い方 |
|---|---|---|
| ファイルを開く関数 | `internal/iac` の大きなテスト | パッケージの中の変数で差し替え、開いた回数を数える(NFR5.26) |
| git を呼ぶ関数 | `internal/iac` の git のテスト | 時間切れ、終了コード、出力、git がないことを作る。実際の git のテストは一時のリポジトリで |
| 名前の変更と待ち | `internal/fileout`、`internal/journal` | `RetryRename` の引数(関数)と、journal のパッケージの変数(U6 のまま) |
| `retryable` | `internal/fileout` | Windows の番号(32・33・5)の誤りと、ほかの誤りを作る |
| HTTP の層の偽の AWS(`internal/testutil`) | `internal/cli` の受け入れ条件 | `iac` で要求が0件であることだけを見る |
| 確認の入力、端末か | `internal/cli` | `iac` は確認を求めないので使わない(U6 と同じ部品のまま) |
| 一時フォルダー | テンプレート、`.tf`、変更案、`--tf-state`、一時のリポジトリ | Go の `t.TempDir` |

## テストデータ

- テンプレートと `.tf` は、テストのコードで組み立てる(形ごと、書き方ごと: ブロックの並び、マッピング、流れの書き方、1行、JSON の複数行と1行、コメント、ブロックスカラー、アンカー、日本語、CRLF、BOM)。
- 変更案は、U4 の変更案の形(契約 C2)をテストのコードで組み立てる。`internal/iac/testdata/sandbox-plan.json` は確認用のスタックの論理 ID とスタック名のタグを持つ(CI の lint のジョブと手順の確かめで使う)。
- `terraform show -json` の例は、Terraform の公式の文書の例と、手元で terraform を使える場合は実物の出力から採り、採った元と版をファイルの先頭か隣の説明に書く(NFR10.14)。
- 大きなテストの分布(1万リソース、2,000ファイル、手で直すもの約10%、候補でない `.json` 1,000個)は、乱数の種を固定して組み立てる。
- 検証用アカウントのルールは `test/sandbox/u8-rules.yaml`。

## テストの数の目安(Standard)

| パッケージ | 単体テスト |
|---|---|
| `internal/iac`(入力と対象) | 変更案と版 3、`--tf-state`(流し読み、plan の出力、空、大きさ)5、対象の決め方(CloudFormation、ARN の完全一致、モジュール、添字、管理されていない)6 |
| `internal/iac`(集め方と上限) | リンクとジャンクション 3、除くフォルダーと深さ 3、数と大きさと合計 4、UTF-8 と BOM 3、入れ子の走査(各形式、正しいファイルが外れない)8 |
| `internal/iac`(読み取り) | YAML の位置と確かめ(コメント、ブロックスカラー、エスケープ、日本語)8、YAML の literal の判定 6、JSON 6、HCL 7、場所の決め方 6 |
| `internal/iac`(当て方と Edit) | pre・post・反映済み 8、Edit(差し替え、削除、挿入、結合、続けて消す、すべて消す、コメントを残す)10、引用符とエスケープの表 8、書き換えたあとの確かめ 4 |
| `internal/iac`(差分、git、書き出し、流れ) | 差分 6、git 12、書き出し 8、並行と順 3、出さないもの 4、結果と終了コード 5、大きなテスト 2 |
| `internal/restype` | 表の確かめ 3、引く関数 3 |
| `internal/fileout` | `RetryRename` 5、`Replace` 6 |
| `internal/journal` | U6・U7 のテストのまま(回帰) |
| `internal/display` | 結果の表示 6、文言 2 |
| `internal/cli` | 受け入れ条件 8 + オプションとヘルプと組み立て 6 |
| `internal/rules` | U8 のルールファイル 1 |
