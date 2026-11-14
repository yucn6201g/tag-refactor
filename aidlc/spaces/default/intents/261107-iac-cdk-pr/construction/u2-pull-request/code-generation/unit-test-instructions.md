# 単体テストの手順: U2 プルリクエストの作成(残り)

計画は `code-generation-plan.md`、テストの決まり(Testing Contract)は同じファイルの中にあります。U1 の手順(`construction/u1-walking-skeleton/code-generation/unit-test-instructions.md`)の道具と設定は、そのまま使います。

## 道具と設定

- Go の標準の `testing` だけを使う(外のアサーションやモックのライブラリは足さない)。ローカルの Go は `GOTOOLCHAIN=local`。
- 配るバイナリと同じく `CGO_ENABLED=0` で動かす(`-race` だけは CI の Linux で cgo を有効にする)。
- 受け入れ条件のテストは本物の git を使う。gh は偽物(テストの二進)を使う。
- 続きの命令を実際のシェルで動かすテストは、Linux と macOS では `sh`、Windows では `powershell.exe` と `pwsh` を使う。

## この単位のテストの動かし方

この単位で足し、直したテストを動かす命令です。受け入れ条件のテストは `TestV02U2` で、続きの命令のテストは `TestResume` で、本文の形の確かめは `TestScan` で始めます。前の単位のテストに、この3つの接頭辞で始まるものはありません。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -run '^(TestV02U2.*|TestResume.*|TestScan.*|TestPullRequest.*|TestStaleBranches|TestGitHubPullRequest)$' ./internal/redact/ ./internal/pullrequest/ ./internal/display/ ./internal/cli/
```

パッケージ全体を動かす命令です。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test ./internal/redact/ ./internal/pullrequest/ ./internal/display/ ./internal/cli/ ./internal/testutil/
```

実際の GitHub の結合テストは、手元では secret の環境変数を入れたときだけ動き、無ければ飛ばされます。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -count=1 -run '^TestGitHubPullRequest$' ./internal/cli/
```

受け入れ条件のテスト(手順 4)は実装の前に書くので、1つ目の命令は、手順 4 の直後に失敗することを確かめるのにも使います。

## カバレッジの目標

- この単位で直したパッケージ(`redact`、`pullrequest`、`display`、`cli`)は、それぞれ文のカバレッジ 80% 以上を保つ。
- リポジトリ全体(`./cmd/tag-refactor/...` と `./internal/...`)の 80% の下限は、Build and Test と CI の coverage の仕事で確かめる。下限は下げない。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -coverprofile=u2.out ./internal/redact/ ./internal/pullrequest/ ./internal/display/ ./internal/cli/
```

## 差し替えの方法

- git と gh の呼び出しは、`internal/procexec` の実行の口を差し替えるか、テストの二進を `git` や `gh` の名前で `PATH` の先頭に置いて差し替える(`internal/testutil`)。偽物は受け取った引数と、トークンの変数の名前(値は記録しない)を書き出す。
- 時計は `deps.Now` を差し替える。時間帯は `time.Local` を差し替えて UTC+9 を試す。
- 使うシェルはテストの中で固定する(`Shell` を引数で渡す。cli の受け入れテストは `t.Setenv("MSYSTEM", ...)`)。Windows の CI は Git Bash から `go test` を動かすので、`MSYSTEM` に頼らない。
- AWS は呼ばない。変更案はテストの中で作った JSON を使う。

## テストのデータ

- 本文のファイルとタイトルに入れるトークンの形は、テストの中で文字列を組み立てて作る(本物のトークンではない、決まった文字の並び)。コミットするファイルに、そのままの形で書かない(秘密の値を探す道具の誤検出も避ける)。
- `internal/iac/testdata` には触らない(ほかのセッションが触っている)。

## 道具が無いとき

- Windows で `powershell.exe` か `pwsh` が無いとき、環境変数 `CI` が空でなければ `t.Fatal`、空なら `t.Skip` に理由を書いて飛ばす。
- 名前付きパイプのテストは Unix だけで動かし、Windows では飛ばす。
