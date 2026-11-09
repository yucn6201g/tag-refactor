# 単体テストの手順: U1 最小の一本

計画は `code-generation-plan.md`、テストの決まり(Testing Contract)は同じファイルの中にあります。

## 道具と設定

- Go の標準の `testing` だけを使う(外のアサーションやモックのライブラリは足さない)。ローカルの Go は `GOTOOLCHAIN=local`。
- 配るバイナリと同じく `CGO_ENABLED=0` で動かす(`-race` だけは CI の Linux で cgo を有効にする)。
- 実際の synth と補助のプログラムを動かすテストには、Node.js 18 以上と、テスト用のアプリの `node_modules` が要る。先にテスト用のアプリで `npm ci --ignore-scripts` を実行する。

```bash
npm ci --ignore-scripts --prefix internal/cdk/testdata/apps/ts-minimal
```

## この単位のテストの動かし方

新しいパッケージのテストを動かす命令です。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test ./internal/procexec/ ./internal/redact/ ./internal/cdk/... ./internal/pullrequest/
```

今のパッケージに足した、この単位のテストを動かす命令です。受け入れ条件のテストは `TestV02U1`、`iac` の CDK の道と表示の CDK の行は `TestCDK`、表示のプルリクエストの行は `TestPullRequest` で始まります。そのほかのテストと、この単位で直した前のテスト(`TestIacPassesItsOptionsOn`、`TestIacIsPutTogetherWithoutAWS`、`TestEveryIdentifierInTheSourcesIsInTheCatalog`)は名前で挙げます。前の単位のテストに、この3つの接頭辞で始まるものはありません。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -run '^(TestV02U1.*|TestCDK.*|TestPullRequest.*|TestManualLineOfAFailedApp|TestGuidanceOfAWrittenCDKSource|TestGitHubPullRequest|TestTheTokenReachesOneStepOnly|TestTheEnvironmentsOfTheIntegrationTest|TestStaleBranches|TestEveryJobThatRunsGoTestInstallsNode|TestIacPassesItsOptionsOn|TestIacIsPutTogetherWithoutAWS|TestEveryIdentifierInTheSourcesIsInTheCatalog)$' ./internal/iac/ ./internal/cli/ ./internal/display/
```

実際の GitHub の結合テストは、手元では secret の環境変数を入れたときだけ動き、無ければ飛ばされます。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -run '^TestGitHubPullRequest$' ./internal/cli/
```

受け入れ条件のテスト(手順 4)は実装の前に書くので、上の2つ目の命令は、手順 4 の直後に失敗することを確かめるのにも使います。

## カバレッジの目標

- この単位で足したパッケージ(`procexec`、`redact`、`cdk`、`pullrequest`)は、それぞれ文のカバレッジ 80% 以上を目安にする。
- リポジトリ全体(`./cmd/tag-refactor/...` と `./internal/...`)の 80% の下限は、Build and Test と CI の coverage の仕事で確かめる。下限は下げない。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -coverprofile=u1.out ./internal/procexec/ ./internal/redact/ ./internal/cdk/... ./internal/pullrequest/
```

## 差し替えの方法

- 外のプログラム(`cdk`、`node`、`git`、`gh`)は、`internal/procexec` の実行の口(インタフェース)を差し替えてテストする。差し替えたものは、渡された引数と環境を記録し、決めた出力と終了コードを返す。
- 終わらないプログラムや孫を作るプログラムは、テストのバイナリ自身を子として動かす(`os.Args[0]` と環境変数で振る舞いを選ぶ、今のリポジトリと同じ方法)。
- gh は、手元の受け入れ条件のテストでは、テストのバイナリ自身を `gh` という名前で `PATH` の先頭に置いて差し替える。git は本物を使い、origin は一時的なフォルダーの裸のリポジトリにする。
- AWS は呼ばない。変更案は、テストの中で作った JSON(stack-name と logical-id のタグつき)を使う。

## テストのデータ

- テスト用の CDK アプリは `internal/cdk/testdata/apps/ts-minimal`。テストでは一時的なフォルダーに写し、`node_modules` はリンク(Unix はシンボリックリンク、Windows はジャンクション)でつなぐ。後片付けでは、リンクだけを消してから写しを消す。
- 補助のプログラムの単体テスト用の小さな TypeScript のソースは、テストの中で文字列として作り、一時的なフォルダーに書く。
- `internal/iac/testdata` には触らない(ほかのセッションが触っている)。
- トークンなどの秘密の値は、テストのデータにも、コミットするファイルにも書かない。

## 道具が無いとき

Node.js、CDK の CLI、テスト用のアプリの `node_modules` のどれかが無いときの扱いは次のとおりです。

- 環境変数 `CI` が空でないとき: `t.Fatal` で失敗にする。
- 手元のとき: `t.Skip` に理由を書いて飛ばす。
