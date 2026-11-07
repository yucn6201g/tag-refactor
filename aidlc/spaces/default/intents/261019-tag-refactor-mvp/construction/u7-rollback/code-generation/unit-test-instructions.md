# 単体テストの手順: U7 取り消し

## 準備

- U1〜U6 と同じ(Go の最新の安定版、`GOTOOLCHAIN=local`、Go の標準の `testing` だけ)。追加の設定ファイルは要らない。
- rollback の流れと記録のテストは、HTTP の層の偽の AWS(`internal/testutil`。U7 でタグが0個のリソースを答えに出さない切り替えと、読み戻しの遅れの切り替えを足す)か、`internal/apply` の中のメモリの偽の AWS で動く。AWS の認証情報は要らない。
- 記録と印のテストは、テストごとの一時のホーム(`t.TempDir` を `HOME`・`USERPROFILE` に設定)で動かし、利用者の本物の `~/.tag-refactor` に触れない。
- 言語は、受け入れ条件と表示のテストで `--lang en` と `t.Setenv`(`LC_ALL`、`LANG`)で固定する(U4 と同じ)。

## この作業単位のテストの実行

U7 で変える・足すパッケージだけを指定して実行します。

最初の受け入れ条件のテストより前に動くことを確かめるコマンド(Step 1):

```bash
go test -mod=readonly ./internal/testutil/... ./internal/apply/... ./internal/journal/...
```

U7 の受け入れ条件のテスト(Step 2 で書き、Step 9 で通す):

```bash
go test -mod=readonly -run 'TestAcceptanceU7' ./internal/cli/...
```

共有に直したあとの U6 の回帰(Step 3、Step 4):

```bash
go test -mod=readonly ./internal/apply/... ./internal/journal/... ./internal/cli/...
```

U7 のすべての単体テスト(Step 10、Step 11、Step 13):

```bash
go test -mod=readonly ./internal/apply/... ./internal/journal/... ./internal/awsaccess/... ./internal/display/... ./internal/cli/... ./internal/rules/... ./internal/testutil/...
```

大きな偽の AWS のテスト(Step 11):

```bash
go test -mod=readonly -run 'TestLargeRollbackU7' ./internal/apply/...
```

レース検出つき(Linux など cgo が使える環境だけ。CI の `race` ジョブと同じ):

```bash
go test -mod=readonly -race ./internal/journal/... ./internal/apply/...
```

カバレッジ(80%以上。測る範囲は U1 と同じ `cmd` と `internal` のすべて):

```bash
go test -mod=readonly -coverpkg=./cmd/tag-refactor/...,./internal/... -coverprofile=coverage.out ./cmd/tag-refactor/... ./internal/...
go tool cover -func=coverage.out
```

確かめの補助の構文と型(Step 12):

```bash
go vet scripts/u6check.go
```

検証用アカウントの確認(手元だけ。U2 で承認した検証コマンドの中で動く。U6 の確認用のスタックとプロファイル `tag-refactor-apply` があること):

```bash
"C:\Program Files\Git\bin\bash.exe" -c "TAG_REFACTOR_SANDBOX_ACCOUNT_ID=<検証用アカウントの ID> bash scripts/verify.sh"
```

## カバレッジの目標

- Go の文単位で80%以上(`cmd` と `internal` のすべて。配線の部分も外さない)。下限を下げて通すことはしない。

## 差し替えの使い方

| 層 | 使う場所 | 使い方 |
|---|---|---|
| HTTP の層の偽の AWS(`internal/testutil`) | 受け入れ条件のテスト、大きなテスト、0個のタグ、読み戻しの遅れ | タグが0個のリソースを答えに出す・出さない、決めた ARN に決めた回数だけ古いタグを返す、書き込みの失敗の一覧、回数と同時の数と遅れ |
| メモリの偽の AWS(`internal/apply` の中) | 判定、結果の表、放棄、止め方 | U6 の apply のテストと同じ作り |
| ファイルの操作、偽の ObjectStore | `internal/journal` の rollback の記録 | 途中の失敗、S3 の記録の開閉と保存 |
| 時計 | 5秒ごとの保存、読み戻しの読み直しの待ち(1・2・4秒) | パッケージの中の変数で差し替える |
| 確認の入力、端末か、合図(Ctrl+C) | `internal/cli` | U6 と同じ |
| 一時フォルダー | ホーム、`--record-from`、記録 | Go の `t.TempDir` |

## テストデータ

- apply の記録は、U6 の apply を偽の AWS で動かして作るか、テストのコードで組み立てる(途中で止まった状態、手で変えた記録)。
- 大きなテストの分布(17リージョン、偏ったリージョン1つ、2つの変更の型)は U6 と同じ組み立てを使い、乱数の種を固定する。
- 検証用アカウントのルールは `test/sandbox/u6-add-rules.yaml` と `u7-*.yaml`。

## テストの数の目安(Standard)

| パッケージ | 単体テスト |
|---|---|
| `internal/apply`(rollback) | 記録の中身の確かめ 8、判定と0個のタグ 6、単位とまとまり 5、結果の表 8、読み戻しの読み直し 5、止め方と放棄 7、差分0の場面 8、終了コード 5、大きなテスト 3 |
| `internal/apply`(共有に直した部品) | U6 のテストのまま(回帰) |
| `internal/journal` | OpenForRollback と目的 4、生の JSON の保持 5、3つの呼び出しと保存 6、S3 3、保存の役の切り出し(U6 のテストのまま) |
| `internal/awsaccess` | ARN の分け方 4、権限の例の包含 2 |
| `internal/display` | 確認画面と結果 6、文言 2 |
| `internal/cli` | 受け入れ条件 7 + オプションと入力と印 8 |
| `internal/rules` | U7 のルールファイル 1 |
