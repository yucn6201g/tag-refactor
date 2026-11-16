# 単体テストの手順: U3 CDK の synth と対応づけ(残り)

計画は `code-generation-plan.md`、テストの決まり(Testing Contract)は同じファイルの中にあります。U1 と U2 の手順(`construction/u1-walking-skeleton/code-generation/unit-test-instructions.md`、`construction/u2-pull-request/code-generation/unit-test-instructions.md`)の道具と設定は、そのまま使います。

## 道具と設定

- Go の標準の `testing` だけを使う(外のアサーションやモックのライブラリは足さない)。ローカルの Go は `GOTOOLCHAIN=local`。
- 配るバイナリと同じく `CGO_ENABLED=0` で動かす(`-race` だけは CI の Linux で cgo を有効にする)。
- 実際の `cdk synth` を動かすテストは、手元の Node.js、テスト用のアプリの `node_modules/.bin/cdk`(Windows は `cdk.cmd`)、Python のアプリでは `py-minimal` の `.venv` の Python を使う。テストは `.venv` も `node_modules` も作らない。手元では README の「開発」の節の手順で先に作る。
- 実際の synth は、`testutil.CopyCDKApp` で `t.TempDir()` の下に写したアプリで動かす(テスト用のアプリのフォルダーに `cdk.out` や `__pycache__` を書かない)。`node_modules` と `.venv` は写さず、リンクでつなぐ。
- 失敗の形(応答しない cdk、子を作って待つ cdk と python、`Python 3.8.18` と答える python など)は、`internal/cdk` の `TestMain` の偽物(テストのバイナリを `TAG_REFACTOR_CDK_CHILD` の役で動かす)と、`internal/cli` の受け入れテストで `PATH` の先頭に置く偽物で作る。

## この Unit のテストの動かし方

この Unit で足し、直したテストを動かす命令です。受け入れ条件のテストは `TestV02U3` で始めます。単体テストは、`internal/cdk` では `TestCDK`、`TestPython`、`TestHome`、`TestTrace`、`TestPyMinimal` と、`TestQuotedArgReachesCmd`、`TestMinPythonMatchesTheTestApp`、`internal/iac` では `TestCDK`、`TestSelectApps`、`TestSameConstructSites`、`TestSameStacks`、`TestTraceSink`、`internal/display` では `TestCDK`、`internal/cli` では `TestParseSynthTimeout` と、ワークフローを読むテスト(`TestEveryJobThatRuns`、`TestTheTokenReachesOneStepOnly`、`TestDependabot`、`TestWorkflowPython`)で始めます。`TestCDK` で始まる U1 のテストは、この Unit が直すコード(発見、synth、照合、進み具合)を通るので、あわせて動かします。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -count=1 -run '^(TestV02U3.*|TestCDK.*|TestPython.*|TestHome.*|TestTrace.*|TestPyMinimal.*|TestQuotedArgReachesCmd|TestMinPythonMatchesTheTestApp|TestSelectApps.*|TestSameConstructSites.*|TestSameStacks.*|TestTraceSink.*|TestParseSynthTimeout.*|TestEveryJobThatRuns.*|TestTheTokenReachesOneStepOnly|TestDependabot.*|TestWorkflowPython.*)$' ./internal/cdk/ ./internal/iac/ ./internal/display/ ./internal/cli/
```

受け入れ条件のテストだけを動かす命令です。受け入れ条件のテスト(計画の手順 5)は実装の前に書くので、この命令は、手順 5 の直後に失敗することを確かめるのにも使います。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -count=1 -run '^TestV02U3' ./internal/cli/
```

Windows で `cdk.cmd` に引用符つきの引数が届くかの確かめ(計画の手順 4)です。ほかの OS では飛ばされます。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -count=1 -run '^TestQuotedArgReachesCmd$' -v ./internal/cdk/
```

時計と進み具合の表の競合の確かめです(CI では race の仕事で動く。手元では cgo と C のコンパイラーがあるときだけ)。

```bash
GOTOOLCHAIN=local CGO_ENABLED=1 go test -race -count=1 -run '^TestCDK' ./internal/display/ ./internal/iac/
```

パッケージ全体を動かす命令です。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test ./internal/cdk/ ./internal/iac/ ./internal/display/ ./internal/cli/ ./internal/testutil/
```

## カバレッジの目標

- この Unit で直したパッケージ(`cdk`、`iac`、`display`、`cli`、`testutil`)は、それぞれ文のカバレッジ 80% 以上を保つ。`python` と `cdk` を呼ぶ部分、`--app` の置き換え、版の確かめ、時計も、差し替えてテストし、計測の対象から外さない。
- リポジトリ全体(`./cmd/tag-refactor/...` と `./internal/...`)の 80% の下限は、Build and Test と CI の coverage の仕事で確かめる。下限は下げない。

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 go test -coverprofile=u3.out ./internal/cdk/ ./internal/iac/ ./internal/display/ ./internal/cli/ ./internal/testutil/
go tool cover -func=u3.out
```

## 差し替えの方法

- `cdk`、`python`、`node` の呼び出しは、`cdk.Config.Runner` の実行の口を差し替える(`fakeRunner`)か、テストのバイナリを偽物として `PATH` の先頭、アプリの `node_modules/.bin`、`.venv/bin`(Windows は `.venv\Scripts\python.exe` と `cdk.cmd` のバッチファイル)に置いて差し替える。偽物は受け取った引数と環境の変数の名前(値は記録しない)を書き出す。
- 時計は、`display.Printer` の `SetTicker`(偽の `Ticker`、テストが時刻を送るチャネル)と `SetClock` で差し替え、`tickDone` で送った時刻の処理が終わるのを待つ。`cdk.Config.Now` も差し替える。
- 版の確かめの上限は `Synthesizer.VersionTimeout`、synth の上限は `Options.Timeout`(受け入れテストでは `--synth-timeout 1s`)で短くする。
- 一時フォルダーの置き場所は `cdk.Config.TempDir`(受け入れテストでは `TMPDIR`、`TEMP`、`TMP` を `t.Setenv`)でテストのフォルダーに向け、実行のあとに空であることを確かめる。
- ホームのフォルダーは `cdk.Config.Home` で差し替える。Windows の形と Unix の形は、`windows` の引数で両方を試す。
- 使う OS の形(`windows` の真偽)は、純粋な関数(`withVenv`、`appCommand`、`shortHome`、`traceFrame`、`userFile`、`selectApps`)では引数で渡し、どの OS でも両方の形を確かめる。

## AWS と認証情報

- AWS は呼ばない。変更案はテストの中で作った JSON を使う。
- `cdk synth` に AWS の認証情報を渡さない。`ts-lookup` と `ts-profile` の needsAWS のテストは、`Config.Environ` や `t.Setenv` に偽の `AWS_ACCESS_KEY_ID` と `AWS_PROFILE` を入れても、起動した `cdk` に渡る環境に AWS_ で始まる変数が無いことを確かめる。遮断した環境で synth するので、実際の AWS には届かない。
- CI のどの仕事にも AWS の認証情報を置かない(ワークフローを読むテストで確かめる)。

## テストのデータ

- テスト用のアプリは `internal/cdk/testdata/apps/` の `ts-minimal`(U1)、`py-minimal`、`ts-trace`、`ts-lookup`、`ts-profile`、`ts-v1`、`py-v1`、`unknown-version`、`other-lang` を使う。整形も lint もしない。依存を入れるのは `ts-minimal` と `py-minimal` だけで、ほかは写した先に `ts-minimal` の `node_modules` をつなぐ。
- `ts-lookup` に書く 12 桁は架空の値にする。実際のアカウントの ID は書かない。
- 作成の記録や標準エラーに入れるトークンの形、ARN、ホームのフォルダーの名前(`alice`)は、テストの中で文字列を組み立てて作る(本物のトークンではない、決まった文字の並び)。コミットするファイルに、そのままの形で書かない(秘密の値を探す道具の誤検出も避ける)。
- `internal/iac/testdata` には触らない(ほかのセッションが触っている)。

## 道具が無いとき

- Python のアプリのテストは `testutil.RequirePythonTools`(`.venv` の Python、`.venv` の site-packages の `aws_cdk_lib-*.dist-info`、Node.js、`py-minimal` の `node_modules/.bin/cdk`)、TypeScript のアプリのテストは今の `testutil.RequireCDKTools` で確かめる。無いとき、環境の変数 `CI` が空でなければ `t.Fatalf`、空なら `t.Skipf` に理由(何が無いかと、README の「開発」の節の手順)を書いて飛ばす。
- `TestMinPythonMatchesTheTestApp` は、METADATA が見つからなければ上と同じく CI では失敗、手元では飛ばす。METADATA があって `Requires-Python` の行が読めないときは、CI でも手元でも失敗にする。
- `TestQuotedArgReachesCmd` は Windows だけで動き、ほかの OS では理由を出して飛ばす。`windowsQuotedApp` を false にしたあとは、理由を出して飛ばす。
- Unix の実際のシンボリックリンクのテストと、`$` を含むフォルダーの synth は Windows では飛ばす。Windows のリンクを経由する `--cdk-app` のテストはジャンクションで作る。
