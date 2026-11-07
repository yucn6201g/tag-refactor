# 単体テストの手順: U6 書き換えと記録

## 準備

- U1〜U5 と同じ(Go の最新の安定版、`GOTOOLCHAIN=local`、Go の標準の `testing` だけ)。追加の設定ファイルは要らない。
- 書き換えの流れと記録のテストは AWS を呼ばない。AWS の口と `apply` のコマンドとしてのテストは、HTTP の層の偽の AWS(`internal/testutil`。U6 でタグの書き込み、ARN を指定した読み取り、S3 の書き込み・読み取り・削除と条件付きの書き込みを足す)で動く。AWS の認証情報は要らない。
- 記録と印のテストは、テストごとの一時のホーム(`t.TempDir` を `HOME`・`USERPROFILE` に設定)で動かし、利用者の本物の `~/.tag-refactor` に触れない。
- 言語は、受け入れ条件と表示のテストで `--lang en` と `t.Setenv`(`LC_ALL`、`LANG`)で固定する(U4 と同じ)。

## この作業単位のテストの実行

U6 で変える・足すパッケージだけを指定して実行します。

最初の受け入れ条件のテストより前に動くことを確かめるコマンド(Step 1):

```bash
go test -mod=readonly ./internal/awsaccess/... ./internal/testutil/...
```

U6 の受け入れ条件のテスト(Step 2 で書き、Step 9 で通す):

```bash
go test -mod=readonly -run 'TestAcceptanceU6' ./internal/cli/...
```

U6 のすべての単体テスト(Step 10、Step 11、Step 13):

```bash
go test -mod=readonly ./internal/journal/... ./internal/apply/... ./internal/awsaccess/... ./internal/display/... ./internal/cli/... ./internal/testutil/...
```

大きな偽の AWS のテスト(Step 11):

```bash
go test -mod=readonly -run 'TestLargeApplyU6' ./internal/apply/...
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

検証用アカウントの確認(手元だけ。U2 で承認した検証コマンドの中で動く。U6 の確認用のスタックの作成と、R3 を引き受けるプロファイル `tag-refactor-apply` が済んでから):

```bash
"C:\Program Files\Git\bin\bash.exe" -c "TAG_REFACTOR_SANDBOX_ACCOUNT_ID=<検証用アカウントの ID> bash scripts/verify.sh"
```

## カバレッジの目標

- Go の文単位で80%以上(`cmd` と `internal` のすべて。配線の部分も外さない)。下限を下げて通すことはしない。

## 差し替えの使い方

| 層 | 使う場所 | 使い方 |
|---|---|---|
| HTTP の層の偽の AWS(`internal/testutil`) | `awsaccess` の口、受け入れ条件のテスト、大きなテスト、失敗の分け方、止め方 | タグの書き込み(リソースごとの失敗の一覧に `StatusCode` とコード)、ARN を指定した読み取り(タグのないリソースを答えに出さない設定)、S3(412・409・501)。回数、同時の数、遅れ、誤りの応答を決められる |
| ファイルの操作 | `internal/journal` の壊れない保存 | 一時ファイルの作成、書き出し、名前の変更を、パッケージの中の変数で差し替えて、途中で失敗させる |
| 時計 | 5秒ごとの保存 | パッケージの中の変数で差し替える |
| 同時の数、まとめる数 | まとまりと同時の数のテスト | パッケージの中の変数で差し替える(U2・U5 と同じ) |
| 確認の入力、端末か | `internal/cli` | 入力の読み手と端末の判定を差し替える |
| 合図(Ctrl+C) | 2段の止め方 | 合図の役に渡す通知の口を差し替える |
| 一時フォルダー | ホーム、`--record-to`、変更案 | Go の `t.TempDir` |

## テストデータ

- 保存した変更案は、U4 の部品で作るか、`internal/testutil` の組み立ての関数で作る。
- 大きなテストの分布(17リージョン、偏ったリージョン1つ、2つの変更の型)はテストのコードで組み立て、乱数の種を固定する。
- 検証用アカウントのルールは `test/sandbox/u6-add-rules.yaml`、`u6-remove-rules.yaml`、`u6-untagged-rules.yaml`。

## テストの数の目安(Standard)

| パッケージ | 単体テスト |
|---|---|
| `internal/journal` | 壊れない保存 6、5秒ごとの保存 6、印 7、権限とログ 4 |
| `internal/apply` | 期待するタグと上限 6、まとまりと読み直しの単位 5、変化の確かめ 5、失敗の分け方 8、数え役と止め方 7、終了コード 5、大きなテスト 3 |
| `internal/awsaccess` | TagWriter と ObjectStore 8、失敗の分類 5、例のポリシーの検査 3 |
| `internal/display` | 確認画面と結果 5、文言 2 |
| `internal/cli` | 受け入れ条件 15 + オプションと入力 6 |
