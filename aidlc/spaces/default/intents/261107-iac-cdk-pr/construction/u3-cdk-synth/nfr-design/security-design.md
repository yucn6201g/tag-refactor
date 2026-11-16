# セキュリティの設計: U3 CDK の synth と対応づけ(残り)

U1 のセキュリティの設計(`construction/u1-walking-skeleton/nfr-design/security-design.md`)と信頼性の設計(同じフォルダーの `reliability-design.md`)、U2 のセキュリティの設計(`construction/u2-pull-request/nfr-design/security-design.md`)は、U3 にもそのまま当てます。ここでは、U3 の非機能の要件(`construction/u3-cdk-synth/nfr-requirements/security-requirements.md` の NFR1.11〜NFR1.13、NFR2.5〜NFR2.7、ほかの区分の NFR3.11〜NFR3.17、NFR4.5、NFR5.3、NFR8.3、NFR8.4、NFR10.4、NFR10.5 と、`tech-stack-decisions.md` の NFR6.16〜NFR6.21)を、どのパッケージのどの関数と型でどう満たすかを書きます。U3 の種類で作る文書は、この文書と `logical-components.md` と `traceability.json` だけなので、性能、信頼性、観測、テストの設計も、この文書に節を分けて置きます。

機能の流れは `construction/u3-cdk-synth/functional-design/functional-spec.md`(W1〜W5)、決まりは同じフォルダーの `rules.md`(BR1.1〜BR6.1)、データの形は `entities.md`、契約は `inception/contract-design/contract-summary.md` の X1、C2、C3、C5、C6、C7 です。出典の ND3-Q1、ND3-Q2 は、このステージの `nfr-design-questions.md` の Q1 と Q2 の回答です。型と呼ぶ場所の変更の一覧は `logical-components.md` にあります。

この文書では、今のコードにある型と関数はその名前で書き、U3 で新しく作るものには「(足す)」と付けます。

## 1. 守るものと境目

U3 で新しく境目を越えるものは4つです。

| もの | どこからどこへ | 守り方 |
|---|---|---|
| Python のアプリの synth に渡す環境 | 利用者の端末の環境 → `cdk synth` とそれが動かす `python` | U1 の遮断した環境に、`.venv` の PATH の先頭と VIRTUAL_ENV だけを足す(2節) |
| 使う Python の実行ファイル | アプリのフォルダーの `.venv`、または PATH → tag-refactor が起動する `python --version` と cdk が起動する `python` | アプリの直下の `.venv` だけを見る。版を確かめてから synth する(2節、3節)。`--app` の置き換えはシェルが読み替えない形だけにする(4節) |
| 作成の記録と標準エラーに入る利用者の端末のパス | `cdk synth --debug` のクラウドアセンブリと標準エラー → TraceSite、`--debug` の行 | TraceSite はアプリからの相対のパスだけにし、絶対パスは捨てる。`--debug` の行のホームのフォルダーは `~` にする(5節) |
| 利用者が `--cdk-app` で渡すパス | 利用者のコマンドの引数 → synth するアプリの選び方 | 発見したアプリの一覧との一致だけで選び、ファイルシステムをたどらない(7節) |

## 2. Python の選び方と `.venv` の環境(NFR2.5、NFR2.6)

### 使う Python を選ぶ(BR2.2、NFR2.6)

`internal/cdk` に `python.go`(足す)を置き、次の型と関数を足します。

```go
// interpreter は python のアプリの synth で使う Python(BR2.2)。(足す)
type interpreter struct {
    path   string // 版の確かめに使う実行ファイル。.venv の Python の絶対パスか、LookPath の結果。確かめないときは空
    venv   string // アプリの直下の .venv の絶対パス。無ければ空
    appArg string // cdk synth の --app に渡す命令(4節)。置き換えないときは空
}

// venvPython はアプリの直下の .venv の Python を返す。(足す)
func venvPython(appAbs string, windows bool) (python, venv string, ok bool)

// firstWord は app の命令の、先頭の空白を除いた最初の語と、その後ろの残り(区切りの空白を含む)を返す。(足す)
func firstWord(command string) (word, rest string)

// isPythonWord は語が python、python3、py のどれかかを返す。末尾の .exe を除き、大文字と小文字を区別しない。(足す)
func isPythonWord(word string) bool

// pythonFor は python のアプリの interpreter を決める。見つからなければ Failure{toolMissing, <道具の名前>}。(足す)
func (s *Synthesizer) pythonFor(app App) (interpreter, *Failure)

// toolName は app の命令の最初の語から、表示と手で直す一覧に出す道具の名前を作る。(足す)
func toolName(word string) string
```

- `venvPython` は、Windows では `filepath.Join(appAbs, ".venv", "Scripts", "python.exe")`、ほかでは `filepath.Join(appAbs, ".venv", "bin", "python")` を `os.Stat`(リンクはたどる)で見て、`Mode().IsRegular()` のときだけ `ok` を true にします。`venv` は `filepath.Join(appAbs, ".venv")` です。`appAbs` は `--dir` を `EvalSymlinks` で直した根の下の `App.Abs` なので、絶対パスです。
- 親のフォルダー、`--dir`、PATH の `.venv` は見ません(NFR2.6)。`.venv` を作る、パッケージを入れる、`.venv` の中に書く処理は持ちません。
- `pythonFor` の順番:
  1. `venvPython` が `ok` なら、`path` と `venv` にその値を入れる。
  2. そうでなければ `firstWord(app.Command)` の語を `s.cfg.Runner.LookPath` で探す。見つからないときの `Tool` は、語が `isPythonWord` なら `"python"`、そうでなければ `toolName(語)` です(例: `pipenv run python app.py` で `pipenv` が無ければ `Tool: "pipenv"`)。どちらも `Reason: ReasonToolMissing` です。見つかれば、語が `isPythonWord` のときだけ `path` にその結果を入れる(それ以外、例えば `pipenv run python app.py` では、使う Python が分からないので `path` を空にし、版を確かめない。NFR6.16)。
  - `toolName`: `filepath.Base` で最後の要素だけにし(cdk.json に `/home/alice/bin/pipenv` のような絶対パスが書かれていても、ホームのフォルダーの名前を表示と本文に出さない)、`Sanitize` で表示できない文字を置き換え、64 文字(コードポイント)で切ります。結果が空、`.`、`/`、`\` なら `"python"` にします。
  - 表示: `Tool` が `node`、`cdk`、`python` のときだけ、その道具の入れ方の文(11節の `iac.cdkToolHint*`)を添えます。それ以外の道具の名前では入れ方の文を出さず、`iac.cdkToolNotFound`(英 `%s was not found`、日 `%s が見つかりません`)の1行だけを出します(「Python 3 を入れてください」は出さない)。手で直す一覧の行は、今の `DetailTool` の出し方のまま、その名前を添えます(BR3.2 で synth に失敗したアプリがちょうど1つのとき)。
  3. `venv` があるときは 4節の `appCommand` で `appArg` を決める。
- 道具の確かめの順番は、今の `Synthesizer.run` の node、cdk のあとに python を足します(BR2.1)。どれも `AppStarted` を送ったあとです(今の `Synthesize` が `AppStarted` のあとに `run` を呼ぶ形のまま)。

### 環境を作る(NFR2.5)

今の `isolate(parent []string)` は、親の環境から AWS_ で始まる変数とトークンの変数を外し、空の設定ファイルとメタデータを止める変数を足します。U3 では、その結果に `.venv` の分だけを足す関数を `python.go` に足します。`isolate` 自体の外し方と足し方は変えません。

```go
// withVenv は isolate が作った環境に、.venv の実行ファイルのフォルダーを PATH の先頭に足し、
// VIRTUAL_ENV を .venv の絶対パスにした新しい環境を返す。vars は変えない。(足す)
func withVenv(vars []string, venv string, windows bool) []string
```

1. 実行ファイルのフォルダー `bin` は、Windows では `filepath.Join(venv, "Scripts")`、ほかでは `filepath.Join(venv, "bin")`。
2. 名前の比べ方: Windows では `strings.EqualFold`、ほかでは完全一致。
3. PATH: `vars` の中で名前が `PATH` に一致する変数をすべて探す。最初に見つかったものの名前の綴り(Windows の `Path` など)と値を覚え、一致したものをすべて外す。新しい値は、覚えた値が空なら `bin`、そうでなければ、覚えた値の最初の要素(`os.PathListSeparator` で区切った先頭)が `bin` と同じ(Windows は `EqualFold`、`filepath.Clean` したもので比べる)ならそのまま、違えば `bin + string(os.PathListSeparator) + 覚えた値`。これを覚えた綴り(無ければ `PATH`)で1つだけ足す。渡す環境の PATH の名前の変数は、いつも1つです。
4. VIRTUAL_ENV: 名前が `VIRTUAL_ENV` に一致する変数をすべて外し、`VIRTUAL_ENV=<venv>` を1つ足す。
5. ほかの変数(`PYTHONHOME`、`PYTHONPATH` を含む)は触りません(NFR2.5 は足すものを PATH と VIRTUAL_ENV に限る)。

呼ぶ場所: `Synthesizer.run` の中で、`isolate` の直後に、python のアプリで `interpreter.venv` があるときだけ `env.vars = withVenv(env.vars, in.venv, runtime.GOOS == "windows")` とします。この環境を、版の確かめ(3節)、normal の synth、trace の synth(`Options.WithTrace`)のすべてに使います。`.venv` の無いアプリでは `withVenv` を呼ばず、親の VIRTUAL_ENV は親の値のまま渡ります(NFR2.5)。

`--debug` に出す環境の行(今の `env.names()`)は名前だけで、値は出しません(U1 のまま)。

単体テスト(`python_test.go`、足す):

- `withVenv` を `windows` を true と false の両方で呼ぶ表のテスト(OS によらずどれも動く)。Windows の場合に親が `Path=C:\x` だけ、`Path` と `PATH` の両方、PATH が無い、先頭がすでに `.venv\Scripts` の4つ。どれも結果の PATH の名前の変数が1つで、先頭が `bin`、重ならない。
- 親に `VIRTUAL_ENV=/other` があるとき、`.venv` のあるアプリの環境では `.venv` の絶対パス、無いアプリの環境では `/other` のまま。
- 差し替えた実行の口(`fakeRunner`)で、python のアプリの synth、trace の synth、版の確かめの3つの呼び出しの `Command.Env` に、AWS_ で始まる変数とトークンの変数が無く、`AWS_EC2_METADATA_DISABLED`、`CDK_DISABLE_VERSION_CHECK`、`CDK_DISABLE_CLI_TELEMETRY`、空のファイルを指す2つの変数と、PATH の先頭と VIRTUAL_ENV があること。
- `pythonFor` の表のテスト(差し替えた `LookPath`): `python3 app.py` で見つからない → `toolMissing`、`Tool: "python"`。`pipenv run python app.py` で見つからない → `Tool: "pipenv"`。`/home/alice/bin/pipenv run python app.py` で見つからない → `Tool: "pipenv"`(`alice` が無い)。`pipenv` が見つかる → 失敗なしで `path` が空。表示の単体テスト(`internal/display`)で、`Tool: "pipenv"` の失敗の行に `pipenv` と `iac.cdkToolNotFound` の文があり、`iac.cdkToolHintPython` の文が無いこと。
- NFR2.6: 親のフォルダーにだけ `.venv/bin/python` があるとき、`LookPath` が app の命令の最初の語で呼ばれ、`.venv` が使われない。`py-minimal` の実際の synth のあとに、`.venv` の下のファイルの一覧(`__pycache__` の下を除く)と `pyvenv.cfg` の中身が変わらない(Python が `__pycache__` に書くことは止めないので、比べる対象から外す)。

## 3. Python の版の確かめ(NFR6.16、NFR3.14)

### 置く場所と順番

`Synthesizer.run` の中で、次の順に行います。

1. node の `LookPath`、`findCDK`(今のまま)。
2. python のアプリなら `pythonFor`(2節)。失敗なら `toolMissing`(道具は `python`、または app の命令の最初の語の `toolName`)。
3. `isolate`(今のまま。一時ファイルを `defer env.close()` で消す)と `withVenv`(2節)。
4. python のアプリで `interpreter.path` が空でなければ `checkPython`(下)。失敗なら synth せずにその失敗を返す。
5. `os.MkdirTemp` で出力先を作り(今のまま)、synth する。

どれも `AppStarted` のあとなので、失敗したアプリの進み具合は running から failed になります(契約 C2、BR2.1)。

### 読み方と分け方

```go
// MinPythonVersion は Python の最小の版 "3.9"(NFR6.16)。表示の文が使う。(足す)
func MinPythonVersion() string

// minPython は最小の版。py-minimal に固定した aws-cdk-lib の Requires-Python の下限と同じにする。(足す)
var minPython = [2]int{3, 9}

// pythonVersionLimit は版の確かめの1回の上限(NFR3.14)。(足す)
const pythonVersionLimit = 30 * time.Second

// pythonVersion は "Python <主>.<副>" を、行の先頭で探す。(足す)
var pythonVersion = regexp.MustCompile(`(?m)^Python (\d+)\.(\d+)`)

// checkPython は in.path に --version を渡して版を確かめる。失敗のときは Failure、中断のときは文脈の誤りを返す。(足す)
func (s *Synthesizer) checkPython(ctx context.Context, app App, in interpreter, env []string, debug *[]string) (*Failure, error)
```

- 起動: `s.cfg.Runner.Run(ctx, procexec.Command{Name: in.path, Args: []string{"--version"}, Dir: app.Abs, Env: env, Timeout: s.versionTimeout(), StdoutLimit: 4 << 10, StderrLimit: 4 << 10})`。`s.versionTimeout()` は `Synthesizer.VersionTimeout`(足す。0 なら `pythonVersionLimit`)で、テストだけが短くします。
- 分け方(上から順に見て、最初に当たったもの):

| 終わり方 | 結果 |
|---|---|
| `errors.Is(err, procexec.ErrTimeout)` | `Failure{Reason: timeout, Tool: python}`(NFR3.14) |
| `ctx.Err() != nil` | `nil, ctx.Err()`(中断。W5) |
| `*procexec.StartError` | `Failure{toolMissing, python}` |
| ほかの `err`、または `res.ExitCode != 0` | `Failure{toolMissing, python}` |
| 標準出力に `pythonVersion` が無く、標準エラーにも無い | `Failure{toolMissing, python}`(Microsoft Store の案内の偽物の `python3` など) |
| 読んだ `(主, 副)` が `minPython` より小さい(主で比べ、同じなら副で比べる。数で比べ、`3.14` を `3.9` より大きいとする) | `Failure{Reason: toolTooOld, Tool: python}` |
| それ以外 | `nil, nil`(通る) |

- 標準出力を先に見て、無ければ標準エラーを見ます(Python 2 は標準エラーに出すため)。
- `--debug` の行(`Synthesis.Debug` に足す): 通ったときは `cdk: <dir>: python: <パス> (Python 3.12)`、古いときは `cdk: <dir>: python: <パス> is older than 3.9`、読めないときは `cdk: <dir>: python: <パス> answered <答えの最初の行>`。`<パス>` は 5節の `shortHome` を通し、答えの行は前後の空白を除いて 200 文字(コードポイント)で切り、`shortHome` と `redact.String` を通します。
- `--version` は利用者のコードを動かしません。`py` のランチャーも同じ形で答えます。

### toolTooOld の扱い

- `internal/cdk/synth.go` の `FailureReason` に `ReasonToolTooOld FailureReason = "toolTooOld"`(足す)。
- `internal/iac` の `failureStatus` は、今の `default` の枝で `CDKAppFailed` と `ReasonCDKAppFailed` を返すので、変えずに `toolTooOld` が `failed` と `cdkAppFailed` になります。BR3.2 では synth に失敗したアプリとして数え、道具の名前は `python` です。
- 表示の対応表 `cdkWhy` に `toolTooOld` を足し、文は 11節の一覧のとおりです。

### 最小の版を確かめるテスト

`internal/cdk/python_test.go` に `TestMinPythonMatchesTheTestApp`(足す)を置きます。

1. `testutil.CDKTestApp("py-minimal")` の下で、`.venv/lib/python*/site-packages/aws_cdk_lib-*.dist-info/METADATA`(Unix の形)と `.venv/Lib/site-packages/aws_cdk_lib-*.dist-info/METADATA`(Windows の形)を `filepath.Glob` で探す。
2. 見つからなければ `testutil.RequirePythonTools`(10節)と同じく、環境の変数 `CI` が空でなければ失敗、空なら理由を出して飛ばす。CI では `.venv` を作ってからテストを動かすので、必ず動きます。
3. `Requires-Python:` で始まる行を読み、`(?:>=|~=)\s*(\d+)\.(\d+)` の最初の一致を下限とする。行が無いか一致が無ければ、CI でも手元でも失敗にする(METADATA の形が変わったことを知らせるため)。
4. 下限が `minPython` と同じことを確かめる。Dependabot が `aws-cdk-lib` を上げて下限が変わると、そのプルリクエストの CI がここで落ちます。

差し替えの単体テスト: `fakeRunner` で `python --version` の答えを `Python 3.8.18`、`Python 3.9.0`、`Python 3.14.5`、標準エラーへの `Python 3.9.0`、`Microsoft Store`(読めない)、終了コード 1、`ErrTimeout` にし、`toolTooOld`、通る、通る、通る、`toolMissing`、`toolMissing`、`timeout`(道具 python)になること、失敗のときに cdk の `Run` が呼ばれないことを確かめます。

## 4. `--app` の置き換え(NFR6.17)

### 命令の作り方

```go
// appCommand は app の命令の最初の語を python(.venv の Python の絶対パス)に置き換えた命令を返す。
// 置き換えないときは "", false。(足す)
func appCommand(command, python string, windows bool) (string, bool)

// windowsQuotedApp は、Windows で空白を含むパスを二重引用符で囲んで --app に渡すかどうか。
// Code Generation の最初の確かめ(下の TestQuotedArgReachesCmd)で、Go の stable か oldstable の
// どちらかで引用符つきの引数が cdk.cmd に届かないと分かったら false にする。(足す)
const windowsQuotedApp = true
```

1. `word, rest := firstWord(command)`。`isPythonWord(word)` でなければ置き換えない(`.venv/bin/python app.py` のように自分でパスを書いたものは、語の全体が `python` などでないので置き換えない。BR2.2)。
2. `python` に、二重引用符の中でもシェルが読み替える文字があれば置き換えない。Unix は `"`、`$`、`` ` ``、`\`。Windows は `"`、`%`。この場合は PATH の先頭の `.venv`(2節)だけに頼ります。
3. `python` に空白(`unicode.IsSpace`)があれば、二重引用符で囲む。Windows で `windowsQuotedApp` が false なら、置き換えない。
4. 空白が無くても、引用符の外でシェルが読み替える文字があれば、二重引用符で囲む。Unix は `'`、`&`、`;`、`|`、`<`、`>`、`(`、`)`、`*`、`?`、`[`、`#`、`~`、`!`、`{`、`}`。Windows は `&`、`|`、`<`、`>`、`^`、`(`、`)`。Windows で `windowsQuotedApp` が false なら置き換えない(この4は NFR6.17 の書き方を広げたもの。NFR6.17 は「パスに空白があれば二重引用符で囲む」とだけ書き、空白の無いパスの `&` や `(` などを扱っていないので、要件の文の直しを 12節に入れる)。
5. 置き換えた命令は、囲まないときは `python + rest`、囲むときは `"\"" + python + "\"" + rest` です(`rest` は区切りの空白ごとそのまま後ろにつなぐ)。

渡し方: `Synthesizer.run` は、`interpreter.appArg` が空でなければ、`args` に `"--app", in.appArg` を足して `os/exec`(`procexec`)で渡します。tag-refactor はシェルを通しません。cdk.json は変えません。

### cdk.cmd を経る受け渡しのテストと切り替え

- `internal/cdk` の synth のテスト `TestCDKPythonAppUnderSpacedFolder`(足す)で、`py-minimal` を `t.TempDir()` の下の `a b` という名前のフォルダーに写し(10節の `testutil.CopyCDKApp`)、実際の cdk で synth して、スタックが読めることを確かめます。Windows ではアプリの `node_modules\.bin\cdk.cmd` を経ます。
- Unix では、`a$b` という名前のフォルダーの下でも synth でき、そのとき `Command.Args` に `--app` が無いことを確かめます。
- 既定を先に確かめる手順(Code Generation の最初の手順として計画に書く): ほかの実装より先に、`internal/cdk` に Windows だけで動くテスト `TestQuotedArgReachesCmd`(足す)を作り、最初のプルリクエストの CI の Windows の仕事(Go の `stable` と `oldstable` の両方。今の test の行列のまま)で動かします。テストは、`t.TempDir()` の下の `a b` というフォルダーに、受け取った `%*` をファイルに書くだけの `cdk.cmd` を置き、`procexec.System` で `"C:\…\a b\.venv\Scripts\python.exe" app.py` の形の引数を `--app` のあとに渡して、ファイルに書かれた引数が、引用符を含めて渡したものと同じであることを確かめます。`os/exec` が起動の前に誤りを返す場合も「届かない」とみなします。
- 決め方(先に決めておく): 2つの版のどちらでも届けば、`windowsQuotedApp` は true のままにします。どちらか1つでも届かなければ、`windowsQuotedApp` を false にし、`README.md` の `iac` の節と 13節の残るリスクに「Windows で空白などを含むフォルダーの下では、tag-refactor は `--app` を渡さず PATH の先頭の `.venv` だけに頼るので、cdk.json の app を `.venv\Scripts\python.exe app.py` のように書く」を足し、`TestCDKPythonAppUnderSpacedFolder` を「Windows で空白を含むフォルダーでは `--app` が無く、cdk.json の app を `.venv\Scripts\python.exe app.py` にした写しで synth できる」に変えます。結果と、どちらにしたかを Code Generation の計画に書きます。
- `TestQuotedArgReachesCmd` はそのまま残し、`windowsQuotedApp` が true の間は「届く」ことを確かめます(Go の版を上げて届かなくなれば CI が落ちて気づける)。false にしたあとは、理由を出して飛ばします。
- 上の手順のあと、`TestCDKPythonAppUnderSpacedFolder` で、アプリの `node_modules\.bin\cdk.cmd`(npm が作る実際のもの)を経ても届くことを確かめます。
- `appCommand` の単体テストは、`windows` を true と false の両方で、空白、`$`、`%`、`"`、`&`、`(`、`.exe` つきの語、`PYTHON3`、`.venv/bin/python app.py`、`pipenv run python app.py`、語のあとのタブを入れた表で行います。

## 5. `--debug` と作成の記録のパス(NFR1.11、NFR1.12)

### ホームのフォルダーを `~` にする(NFR1.12)

```go
// homeForms はホームのフォルダーの書き方を、長いものから順に返す。Clean した形と、
// eval(既定は filepath.EvalSymlinks)で実際のパスにした形が違えば両方。(足す)
func homeForms(home string, eval func(string) (string, error)) []string

// shortHome は s の中の、homes のどれかに当たる部分を ~ に置き換える。(足す)
func shortHome(s string, homes []string, windows bool) string
```

- `home` は `Config.Home`(足す。`func() (string, error)`、既定は `os.UserHomeDir`)で、`Config.withDefaults` で決めて `Synthesizer` と `Analyzer` が1回だけ読み、`homeForms` に通した結果を持ちます。読めないか空なら空の一覧にし、置き換えません。`eval` が誤りを返したら、Clean した形だけにします。
- `homeForms` が実際のパスの形も返すのは、アプリのパス(`App.Abs`、`.venv` のパス、置き換えた `--app` の命令)が `EvalSymlinks` した根から作られるためです。ホームがシンボリックリンクの先にある環境(macOS の `/var` → `/private/var` のような形を含む)でも、どちらの書き方も `~` になります。
- `shortHome` は `s` を左から見て、各位置で `homes` を順に(長いものから)比べ、最初に一致したものを使います。一致を `~` にするのは、一致の直後が文字列の終わりか、英数字(`unicode.IsLetter`、`unicode.IsDigit`)、`_`、`-`、`.` のどれでもない文字のときだけです。つまり `/`、`\`、引用符、コロン、括弧、空白、コンマなどが続けば置き換え、`/home/alice2`、`/home/alice_x`、`/home/alice-x`、`/home/alice.bak` は置き換えません。置き換えたら一致の後ろから、置き換えなければ1文字進めて探し続けます。Windows では大文字と小文字を区別しません(`strings.EqualFold` で長さ分を比べる)。
- 通す行: `internal/cdk` が `Synthesis.Debug` と `Analysis.Debug` に足すすべての行。今の `keptLines(stderr, debug, prefix)` は `keptLines(stderr, debug, prefix, homes)`(`homes` は `[]string`)に変え、`redact.Lines` で残した行(スタックの `at ` の行には絶対パスが入る)に `shortHome` を通します。順番は `shortHome` のあとに `redact.String`(`redact.Lines` が中で通す)です。
- synth の命令の行: 今の `cdk: <dir>: cdk synth --quiet --no-notices --output <temporary folder>` に、`--debug`(trace のとき)と `--app <置き換えた命令>`(置き換えたとき)を足します。置き換えた命令は `shortHome` を通します。例: `cdk: infra/py: cdk synth --quiet --no-notices --output <temporary folder> --app "~/work/my app/infra/py/.venv/bin/python" app.py`。
- `.venv` を PATH だけで使ったとき(置き換えないとき)も、`cdk: <dir>: .venv: <shortHome を通した .venv のパス> (first on PATH, VIRTUAL_ENV)` の1行を出し、`.venv` を使ったことが分かるようにします。
- 版の確かめの答えの行も `shortHome` を通します(3節)。

単体テスト(`internal/cdk` の `home_test.go`、足す):

- 境目の表(`windows` は true と false の両方): `homes` を `["/home/alice"]` にして、`/home/alice`(終わり)、`/home/alice/app`、`"/home/alice"`、`'/home/alice'`、`/home/alice:12`、`(/home/alice)`、`/home/alice app`、`/home/alice,x` は `~` になり、`/home/alice2`、`/home/alice_x`、`/home/alice-x`、`/home/alice.bak` は変わらない。1行に2回出るものは両方 `~` になる。Windows の形は `C:\Users\Alice` に対して `c:\users\alice\app` と `"C:\Users\Alice"` が `~` になる。
- `homeForms`: 差し替えた `eval` が別のパスを返すと2つ(長いものが先)、同じパスか誤りを返すと1つ。`homes` が `["/private/var/home/alice", "/var/home/alice"]` のとき、両方の書き方が `~` になり、`/private/var/home/alice` が `/private~` にならない(長いものを先に比べるため)。
- 実際のリンク(Unix だけ。Windows のシンボリックリンクは権限が要るので飛ばす): `t.TempDir()` の下の実際のフォルダーへのシンボリックリンクを `Config.Home` にし、`EvalSymlinks` した根の下のアプリで、`--app` の行と `.venv` の行に実際のパスが残らず `~` になること。

### 作成の記録のパス(NFR1.11)

`internal/cdk` に `trace.go`(足す)を置きます。trace の synth(`Options.WithTrace` が true)が成功したときだけ、`Synthesizer.run` が出力先を消す前に呼びます。

```go
// Options に足す欄
WithTrace bool // cdk synth --debug で作成の記録を残す(BR4.1、契約 C2)(足す)

// Synthesis に足す欄(契約 C2 の TraceSites)
TraceSites []TraceSite // WithTrace のときだけ(足す)

// readTraceSites はクラウドアセンブリの作成の記録から、コンストラクトのパスごとに利用者のソースの
// 最初の箇所を読む。返す File はアプリのフォルダーからの相対、/ 区切り。(足す)
func readTraceSites(outDir, appAbs string, windows bool) (sites []TraceSite, dropped int, err error)

// traceFrame は記録の1行から、ファイルの絶対パスと行番号を取り出す。(足す)
func traceFrame(line string, windows bool) (file string, lineNo int, ok bool)

// userFile は file がアプリのフォルダーの下で、node_modules、.venv、venv、site-packages の下でないときに、
// アプリからの相対のパス(/ 区切り)を返す。(足す)
func userFile(file, appAbs string, windows bool) (string, bool)
```

- 記録のどの欄にどの書き方で入るか(manifest の `metadata` の項目の `trace` の配列か、など)は、BR4.2 のとおり Code Generation でテスト用のアプリの実際の出力と照らして決め、テストで固定します。この節は、取り出したあとのパスの扱いを決めます。
- `traceFrame`: V8 のスタックの行の形 `at <名前> (<ファイル>:<行>:<桁>)` と `at <ファイル>:<行>:<桁>` を、行の末尾から読みます。`file://` で始まるものは `url.Parse` のパスにし、Windows で `/C:/` の形なら先頭の `/` を除きます。絶対パスでないものは捨てます。
- `userFile`: `filepath.Rel(appAbs, filepath.Clean(file))` が誤りか、`..` で始まるか、絶対パスなら捨てます(アプリの外)。相対のパスを `/` で区切った要素のどれかが `node_modules`、`.venv`、`venv`、`site-packages`(Windows では大文字と小文字を区別しない)なら捨てます。`App.Abs` は `EvalSymlinks` した根の下にあり、Node.js も実際のパスで記録するので、比べる両方が実際のパスです。
- 1つのコンストラクトのパスについて、記録を上から見て最初に `userFile` が通った行だけを使います(BR4.2)。行番号が 1 より小さいものは捨てます。返す `sites` は `ConstructPath` の順に並べます(補助のプログラムに渡す順と、Recheck に渡す順を決まったものにするため)。
- 絶対パスは、`readTraceSites` の中の変数にだけあり、`TraceSite`、`Synthesis.Debug`、誤りの文のどれにも入りません。`--debug` には `cdk: <dir>: trace: <使えたパスの数> construct paths, <捨てた行の数> frames left out` だけを出します。誤り(manifest が読めないなど)は、パスを含まない固定の文で `Synthesis.Debug` に出し、TraceSite を空にします(BR4.3 と同じく、集めたリソースは cdkNoSource)。
- 出力先は、今の `run` の `defer os.RemoveAll(out)` で消えます。記録の中身はメモリにだけあり、`run` から戻ったら残りません。
- `TraceSite` は `AnalyzeRequest.TraceSites` で補助のプログラムに渡り(相対のパスのまま)、補助のプログラムが返す `ConstructSite.File` も相対です。手で直す一覧と本文のファイルは、今のとおり `--dir` からの相対です。

単体テスト(`trace_test.go`、足す): 記録の行に `/home/alice/app/lib/stack.ts:12:5`、`C:\Users\alice\app\lib\stack.ts:12:5`、`file:///home/alice/app/bin/app.ts:3:1`、アプリの外、`node_modules`、`.venv`、`venv`、`site-packages` の下のフレームを入れた表で、テストを動かしている OS の形のパスは相対の TraceSite になり、ほかの OS の形とアプリの外と4つのフォルダーの下は捨てられることを確かめます(3つの OS の CI で、Unix と Windows の両方の形を確かめることになる)。受け入れテスト(`internal/cli`)で、差し替えた cdk が書く記録と標準エラーに両方の形のホームのパスを入れ、プルリクエストの本文、コミットのメッセージ、`--debug` なしの標準エラーに `alice` が出ず、`--debug` の行では `~` になることを確かめます。

### 作成の記録の箇所を補助のプログラムで使う(BR4.4、契約 C3 construct_rules)

今の補助のプログラム(`internal/cdk/helper/ts-helper.js`)は、要求の `traceSites` を読まず、作成の箇所を `from: "source"` だけで出します。また今の `Analyzer.Recheck` は `TraceSites` を空の配列で送り、`internal/iac` の `compareAnalyses` は `ConstructSites` 全体の一致を見ます。このままでは、2回目の Analyze で trace 由来の作成の箇所が入ったアプリは、Recheck の結果にそれが無いので、編集がすべて cdkUnsupportedShape になります。そこで U3 で次の3つを変えます。Analyzer の起動の回数、上限(2 分)、`node --version` の確かめは変えません。

**1. 補助のプログラム(`ts-helper.js`)が `traceSites` から作成の箇所を作る**

- `analyze(ts, request)` の、ソースをたどる処理(`stackInstances` と `readClass`)のあとに、`traceSites(ts, files, request, out)`(足す)を呼びます。
- ソースで見つかったパスの集合を `out.constructSites` の `constructPath` から作り、`request.traceSites` を渡された順に見て、次のどれかに当たるものは飛ばします: パスがすでに集合にある(ソースの結果を先に使う。construct_rules)、`file` が読んだソースのファイル(`files` の Map。parseError のファイルと、`sourceFiles` に入らないファイルを除く)に無い、`line` が 1 より小さい。
- 残ったものごとに、`traceNew(ts, source, line, id, mode)`(足す)で `new` の式を探します。`id` はコンストラクトのパスの最後の要素(最後の `/` の後ろ)です。
  1. 開始の行(`lineOf(sf, node.getStart(sf))`)が `line` と同じ `NewExpression` のうち、ソースの順で最初のもの(いちばん外側)。
  2. 1 が無く、`mode` が `recheck` のときだけ: 2つ目の引数が文字列(`stringLiteral`)で、その中身が `id` と同じ `NewExpression` のうち、開始の行が `line` に最も近いもの(同じ近さなら前の行)。書き換えで行がずれても同じ作成の箇所を見つけるためです。`analyze` のときは作成の記録の行がそのまま正しいので、2 を使いません。
  3. どちらも無ければ、そのパスの作成の箇所は出しません(analyze ではそのリソースは cdkNoSource のまま。recheck では 3つ目の比べ方で食い違いになり、編集を捨てる方向に失敗する)。
- 見つけたら `out.constructSites` に `{constructPath, file, line: 見つけた式の開始の行, constructId: 2つ目の引数が文字列ならその中身、そうでなければ id, from: "trace"}` を足し、パスを集合に足します。trace 由来の作成の箇所は、ソースの作成の箇所の後ろに、`request.traceSites` の順(`readTraceSites` が `ConstructPath` の順に並べたもの)で並びます。
- タグの箇所(`tagSites`)と足す場所(`insertSites`)の読み方は変えません。JSON の形は変えないので、`HelperVersion` も変えません(`from: "trace"` は契約 C3 と C4 にすでにある値)。
- `recheck` のときも、`files` の中身(書き換えたあとの内容)を読んだうえで同じ処理をします(`mode` だけが 2 の有無を分ける)。

**2. Recheck に、2回目の Analyze と同じ TraceSites を渡す**

- `cdk.Analyzer.Recheck(ctx, app, edited)` を `Recheck(ctx, app, edited, traces []TraceSite)` に変え、要求の `traceSites` に `nonNilTrace(traces)` を入れます。`ConstructPaths` は今のまま空です。
- `internal/iac` の `cdkRun` に `traces []cdk.TraceSite`(足す)を足します。2回目の Analyze の結果を `run.analysis` に使ったときだけ、そのとき渡した `TraceSites` を入れます(trace の synth をしなかった、失敗した、スタックが違ったアプリでは nil)。`recheckCDK` は `r.w.analyzer.Recheck(ctx, run.app, edited, run.traces)` と呼びます。
- `internal/iac` の analyzer のインタフェースのメソッドの形が変わるので、同じ変更で、テストの差し替えを含むすべての実装を直します(契約の決まり R-04)。

**3. `compareAnalyses` で、trace 由来の作成の箇所はパスとファイルだけを比べる**

- `compareAnalyses` の `slices.Equal(after.ConstructSites, before.ConstructSites)` を、`sameConstructSites(before, after []cdk.ConstructSite) bool`(足す)に置き換えます。誤りの文 `the construct sites changed` は今のままです。
- `sameConstructSites`: 両方を `From` が `trace` のものと、それ以外(`source`)に分けます。`source` のものは、今と同じく順番、パス、ファイル、行、ID、`From` のすべてが同じことを見ます。`trace` のものは、数が同じで、`before` のそれぞれについて、`ConstructPath` と `File` が同じものが `after` にあることだけを見ます。行(と ID)は、書き換えでずれることがあるので比べません。
- タグの箇所の比べ方(`UseLine` を含む)は今のままです。

**テスト**

- 補助のプログラム(`internal/cdk` の Analyzer のテスト。`testutil.CopyCDKApp` で写した `ts-trace` を使い、本物の補助のプログラムを動かす): (a) `TraceSites` を渡すと、ソースでは見つからない関数の中の作成の箇所が `from: "trace"` で、記録の行のまま出る。(b) ソースで見つかるパスの `TraceSite` を渡しても、`from: "source"` の1件だけ。(c) `new` の無い行、ソースでないファイル、parseError のファイルを指す `TraceSite` からは何も出ない。(d) Recheck に同じ `TraceSites` と、`new` の式の上に2行を足した内容を渡すと、`from: "trace"` の作成の箇所が同じパスとファイルで、行が2つ後ろで出る。(e) 2つ目の引数が文字列でない作成の箇所で、行がずれた内容の Recheck では何も出ない。
- `compareAnalyses` の表のテスト(`internal/iac`): trace 由来の作成の箇所の行だけが違う → 一致。trace 由来の作成の箇所が無くなる、ファイルが違う、パスが違う → `the construct sites changed`。`source` の作成の箇所の行が違う → 今のとおり食い違い。
- Recheck のテスト(`internal/iac`、差し替えた analyzer): trace を使ったアプリの Recheck に渡る `traces` が、2回目の Analyze に渡した `TraceSites` と同じ。trace を使わなかったアプリでは空。trace 由来の作成の箇所を持つアプリの編集は、Recheck の結果の作成の箇所の行がずれていても cdkUnsupportedShape にならず、書き換えが残る。
- 受け入れテスト(`internal/cdk` の synth のテストと `internal/cli`): `ts-trace`(10節)で、実際の `cdk synth` と trace の synth、本物の補助のプログラムを通して、関数の中で作ったバケットの `Owner` のキー名の変更(スタックの `Tags.of(this)` の行の書き換え)が反映され、cdkNoSource にも cdkUnsupportedShape にもならないこと。

## 6. 本文と表示に出すアプリの状態(NFR1.13)

- プルリクエストの本文には、アプリの状態の節を足しません(契約 X2 の節のまま)。手で直す一覧の行は、今の `display.manualLine` のとおり、カタログの固定の文と道具の名前(`DetailTool` のとき)だけです。どのアプリのものか分からないリソース(BR3.2)の行には、アプリのフォルダーも書きません。
- 結果と進み具合の表示に出すのは、アプリのフォルダー(`--dir` からの相対)、言語、状態、経過時間、理由の種類の固定の文、道具の名前と入れ方の固定の文、needsAWS の対処の固定の文だけです。理由の文は、今の `Printer.why` のとおりカタログの文に限り、知らない理由の文字列は `Sanitize` を通してそのまま出します(U3 で足す理由はすべてカタログに入れる)。
- `SynthFailure.detail`(`Failure.Detail`)は、`internal/iac` の `CDKApp` に写しません(今の `CDKApp` に `Detail` の欄は無く、足しません)。`--debug` の行(`Synthesis.Debug`)にだけ、`redact.Lines` と `shortHome` を通して出ます。

受け入れテストで、差し替えた cdk の標準エラーに ARN、12 桁の数字、GitHub のトークンの形を入れ、`--pr` の本文、`--debug` なしの標準エラー、標準出力にどれも出ないことを確かめます。

## 7. `--cdk-app` と発見(NFR2.7、NFR3.15)

### 発見で入らないフォルダー(NFR3.15)

- `internal/cdk/app.go` の `skippedFolders` に `.venv` と `venv` を足します(`.git`、`node_modules`、`cdk.out` は今のまま)。名前は完全一致で比べます(今のまま)。リンクとジャンクションに入らないこと、深さ 32(`maxDepth`)は今のままです。
- テストのために、`discover` が呼ぶ `os.ReadDir` を変数 `readDir`(足す。既定は `os.ReadDir`)にし、読んだフォルダーを数えます。フォルダーとファイルを入れた `.venv` と `venv` のあるアプリで、その下を一度も読まないことを確かめます。
- 変更案に stack-name のタグのリソースが無く `--cdk-app` も無いときは、`Discover` を呼びません(差し替えた synthesizer で呼び出しが0回)。

### `--cdk-app` の確かめ(NFR2.7、BR1.2)

`internal/iac` に次の関数を足します。

```go
// selectApps は --cdk-app の値を、発見したアプリの一覧と照らして確かめ、選んだアプリを名前の順に返す。
// values が空なら apps をそのまま返す。(足す)
func selectApps(apps []cdk.App, values []string, windows bool) ([]cdk.App, error)
```

1. 値ごとに、Windows では `\` を `/` に置き換え、`path.Clean` する。
2. 空の値、または整理したものが `..` そのものか `../` で始まるなら、`usageError("--cdk-app", "iac.cdkAppOutside", "iac.cdkAppFix", 値)`。
3. 整理したものと `App.Dir` が同じ(Windows では `strings.EqualFold`)アプリを探す。無ければ `usageError("--cdk-app", "iac.cdkAppNotFound", "iac.cdkAppFix", 値)`。`/etc`、`C:/x`、`C:` のような絶対パスは `/` やドライブで始まり、発見した `Dir`(相対、`.` を含む)と一致しないので、ここで誤りになります。`node_modules/x`、`.venv/x`、リンクを経由するパス、`cdk.json` の無いフォルダー、フォルダーでないものも、発見の一覧に無いのでここで誤りになります。
4. 同じアプリを2回指したら1回にする。
5. `selectApps` はファイルシステムを読みません。値でたどるのは発見の一覧だけなので、リンクや `..` で `--dir` の外を読むことがありません。

呼ぶ場所: `Writer.Run` の中で、`loadPlan` と `readTFState` のあと、`buildTargets` の結果によらず(変更案のリソースが0件でも)、`req.CDKApps` が空でなければ `w.synth.Discover(ctx, root, nil)` と `selectApps` を呼び、誤りならそのまま返します(cli が終了コード 1 にする。どのファイルも書き換えず、synth も起動しない)。選んだ一覧は `runner` の欄 `apps []cdk.App` と `discovered bool`(足す)に入れ、`prepareCDK` は `discovered` なら `Discover` をもう一度呼ばずに使います。`--pr` のときは、cli の `checkPullRequest`(git と gh を読むだけ)のあとに来ますが、書き換えの前であることは変わりません。

`Writer` が synthesizer を持たないとき(`w.synth == nil`)は、今の `prepareCDK` の確かめ(`r.w == nil || r.w.synth == nil` なら CDK の処理をしない)と同じ条件で、`Discover` と `selectApps` を呼ばず、`runner.discovered` は false のままにします。`--cdk-app` の値は使われず、誤りにもなりません。cli の `iac` はいつも `cdk.NewSynthesizer` を入れた `Writer` を作るので、利用者の実行でこの道を通ることはありません。単体テストで、synthesizer の無い `Writer` に `CDKApps` を渡しても誤りにならず、今の出力のままであることを確かめます。

受け入れテスト(`internal/cli`): `../other`、`a/../../other`、`..\other`(Windows)、`node_modules/x`、`.venv/x`、リンクを経由するパス(Unix は `os.Symlink`、Windows はジャンクション)、`/etc`、`C:\x`、`C:` が、終了コード 1、差し替えた cdk の起動0回、ソースが変わらないこと。Windows で `Infra/TS` が `infra/ts` に一致すること。変更案に stack-name のリソースが無くても `--cdk-app infra/none` が終了コード 1 になること。

## 8. 時間の上限、起動の回数、止め方、後片付け(NFR3.11〜NFR3.17、NFR6.21、NFR4.5)

この道具は利用者が手で動かす CLI なので、再試行やサーキットブレーカーは置きません(U1 の信頼性の設計のまま)。

### `--synth-timeout`(NFR3.11、NFR6.21)

- `internal/cli` の `iacArgs` に `synthTimeout string` と `cdkApps []string`(足す)を足し、`f.StringVar(&o.synthTimeout, "synth-timeout", "10m", ...)`、`f.StringArrayVar(&o.cdkApps, "cdk-app", nil, ...)` で受けます。`--cdk-app` は `StringArrayVar` にし、コンマで分けません(フォルダーの名前にコンマがあっても1つの値)。
- `parseSynthTimeout(s string) (time.Duration, error)`(足す)は `time.ParseDuration` で読み、誤りか `< time.Second` なら `errkind.InvalidInput` の誤り(`DetailID: "iac.badSynthTimeout"`、`HintID: "iac.badSynthTimeoutFix"`)を返します。`10`(単位なし)、`abc`、`0s`、`999ms`、`-5s` は誤り、`90s`、`10m`、`1h30m` は通ります。
- `runIac` は `--plan` と `--dir` の確かめの直後、`checkPullRequest` の前に呼びます(誤りは書き換えの前で、git も gh も呼ばない)。
- `iac.Request` に `SynthTimeout time.Duration` と `CDKApps []string`(足す)を足し、cli が入れます。`internal/iac` は normal と trace のどちらの synth にも `cdk.Options{Timeout: r.req.SynthTimeout}` を渡します。`procexec.Command.Timeout` は1回の `Run` ごとの上限なので、normal と trace に別々に効きます。`cdk.DefaultSynthTimeout`(10 分)は、`Options.Timeout` が 0 のときの既定として残します。

### 起動の回数(NFR3.13、NFR3.17)

`internal/iac` のアプリごとの処理(`logical-components.md` の処理の順番)で、次の回数にします。

| 起動 | typescript のアプリ | python のアプリ |
|---|---|---|
| normal の synth | 1 | 1 |
| Python の版の確かめ | 0 | 0 か 1(3節) |
| Analyze | 1。trace の synth が成功しスタックが1回目と同じときだけ 2 | 0(U5 まで) |
| trace の synth | 作成の箇所が見つからないリソースがあるときだけ 1 | 0 |
| Recheck | 書き換えがあるときだけ 1(ファイルごとでなく、アプリの書き換えたファイルをまとめて1回)。渡す `TraceSites` は、2回目の Analyze の結果を使ったときはそのとき渡したもの、ほかは空(5節) | 0 |
| `node --version`(今の `Analyzer.checkNode`) | Analyze と Recheck の呼び出しごとに 1 | 0 |

テストは、`internal/iac` の差し替えの synthesizer と analyzer で呼び出しを数えます(trace を使わないアプリで analyze 1、使うアプリで 2、trace の synth が失敗したかスタックが違うアプリで 1、python のアプリで 0)。同じテストで、書き換えのあるアプリの Recheck が1回であること、trace を使ったアプリの Recheck に渡る `TraceSites` が2回目の Analyze に渡したものと同じで、ほかのアプリでは空であること、trace 由来の作成の箇所を持つアプリ(差し替えの Recheck が行をずらした作成の箇所を返す場合を含む)の編集が cdkUnsupportedShape にならないことも確かめます。U1 の NFR3.5 の回数のテストは、これに置き換えます。

### 全体の時間の見積もり(NFR3.12)

全体の上限は置きません。`README.md` の `iac` の節に、次の見積もり方を書きます。

> 最も長い時間 = synth するアプリの数 × (Python の版の確かめ 30 秒 + synth の上限 × 2 + (Node.js の版の確かめ 30 秒 + 補助のプログラムの上限 2 分) × 3)

Node.js の版の確かめを「× 3」の中に入れるのは、今の `Analyzer` が補助のプログラムの呼び出しごとに `node --version` を動かすためです。NFR3.12 の文は Node.js の版の確かめを1回(30 秒)として数えているので、要件の式をこの形に直すことを 12節に入れます。既定の上限では、アプリ1つあたり 30 秒 + 20 分 + 7 分 30 秒 = 28 分です(NFR3.12 の文のままの式では 27 分)。

### 子まで止める(NFR3.16)

版の確かめ、normal と trace の synth は、どれも今の `procexec.System.Run`(Unix はプロセスのグループ、Windows はジョブオブジェクト、`WaitDelay` 5 秒)で起動するので、cdk が動かした `python` とその子も一緒に止まります。新しい起動の口は作りません。

テスト(`internal/cdk`、3つの OS):

- `internal/cdk` に `TestMain`(足す)を置き、環境の変数 `TAG_REFACTOR_CDK_CHILD` があるときはテストのバイナリが偽物として振る舞うようにします(U1 の `procexec` のテストと同じ仕組み)。役は `version`(`TAG_REFACTOR_CDK_VERSION` の文を出して終わる)、`hang`(孫を起動してその番号をファイルに書き、眠る)、`grandchild`(番号を書いて眠る)です。
- 偽の Python は、テストのバイナリを `.venv/bin/python`(Windows は `.venv\Scripts\python.exe`)に写したものです。偽の cdk は、Unix ではテストのバイナリを `node_modules/.bin/cdk` に写したもの、Windows では `node_modules\.bin\cdk.cmd` に `@"<テストのバイナリ>" %*` と書いたバッチファイルです。
- `Synthesizer.VersionTimeout` と `--synth-timeout` に相当する `Options.Timeout` を 1 秒にして、`hang` の版の確かめと synth が上限のあと 10 秒の中で戻り、孫が残らないこと、取り消した文脈でも同じことを確かめます。孫が生きているかは `testutil.ProcessAlive(pid int) bool`(足す。今の `internal/procexec` の `alive_*_test.go` と同じ見方を `internal/testutil` に移す)で見ます。

### 後片付けと中断(NFR4.5)

| 一時的なもの | 作る場所 | 消す場所 |
|---|---|---|
| normal と trace の synth の出力先 | `Synthesizer.run` の `os.MkdirTemp` | 同じ関数の `defer os.RemoveAll` |
| 認証情報の遮断の空のファイル | `isolate` | 呼び出し側の `defer env.close()`(synth、版の確かめ、補助のプログラム) |
| 補助のプログラムの書き出し先 | `Analyzer.call` の `privateDir` | 同じ関数の `defer os.RemoveAll` |

- 置き場所をテストで選べるよう、`cdk.Config` に `TempDir string`(足す。空なら `os.TempDir()`)を足し、`os.MkdirTemp(cfg.TempDir, ...)`、`isolate(parent []string, tempDir string)`、`privateDir(base string)` に渡します(今の `isolate(parent)` と `privateDir()` の引数を足す)。
- 中断(Ctrl+C)では文脈が取り消され、`procexec` がプロセスの木を止め、`Synthesize` と `Analyze`、`Recheck` は `ctx.Err()` を返し、上の `defer` がすべて走ります。`internal/iac` の `prepareCDK` は、`*cdk.HelperFailure` でない誤りを今の `helperError` を通してそのまま返し、書き換えには進みません。cli は今の `iac` の中断と同じ表示と終了コード 1 にします。
- アプリの `cdk.out` は、`--output` に一時的なフォルダーを渡すので変わりません(今のまま)。

テスト: `Config.TempDir` を `t.TempDir()` の下のフォルダーに向け、(a) synth の成功、失敗、時間切れ、(b) `Analyze` を動かしている `fakeRunner.Run` の中で文脈を取り消す、(c) `Recheck` の中で取り消す、の各場合のあとに、そのフォルダーが空であること、アプリの `cdk.out` に置いた目印のファイルが変わらないことを確かめます。

## 9. 進み具合の時計(NFR10.4、NFR6.20、BR5.4)

### 持つ場所

時計は `display.Printer` が持ちます。`cdk.ProgressSink` の5つのメソッドは変えません(BR5.4、契約の決まり R-04)。

- `progress`(今の構造体。scan のリージョンの表と同じ)に、アプリの表のための欄を足します: `apps []string`、`byApp map[string]*appRow`(足す)。`apps` が nil でなければアプリの表として描きます。

```go
// appRow は進み具合の表の、アプリごとの行。(足す)
type appRow struct {
    state     string        // waiting | running | done | failed | skipped
    language  string
    startedAt time.Time     // running になった時刻
    elapsed   time.Duration // done と failed のときの時間
    reason    string        // failed と skipped の理由の種類
    tool      string        // 失敗した道具(10節の ToolFailureSink から)
    lastLine  time.Time     // 端末でないとき、この行の最後の行を足した時刻
}

// Ticker は時計の差し替え口(テスト)。(足す)
type Ticker interface {
    C() <-chan time.Time
    Stop()
}

// SetTicker は時計を差し替える(テスト)。既定は time.NewTicker を包んだもの。(足す)
func (p *Printer) SetTicker(newTicker func(time.Duration) Ticker)
```

- `Printer` に足す欄: `newTicker func(time.Duration) Ticker`、`clock *appClock`、`clockWG sync.WaitGroup`、`running int`、`pendingSkipped []skippedApp`(下の「外したアプリの出し方」)、`tickDone func()`(テストだけが入れる)。`skippedApp` は `struct{ dir, reason string }`(足す)です。

```go
// appClock は動いている時計1つ。(足す)
type appClock struct {
    stop chan struct{}
}
```

### 動き

| きっかけ(どれもロックの中) | すること |
|---|---|
| `AppSkipped`(アプリの表がまだ無いとき。BR1.4 の順番で、外すアプリの通知は `AppsWaiting` より前に来る) | `p.pendingSkipped`(足す。`[]skippedApp{dir, reason}`)に足す。書き直しの表でなければ、今の `iac.cdkSkipped` の行をすぐ出す(NFR5.3)。書き直しの表なら、ここでは何も描かない |
| `AppsWaiting(apps)` | `endProgress()` のあと、`apps` を待ちにした行と、`p.pendingSkipped` を skipped(理由つき)にした行で `progress` を作り、`p.pendingSkipped` を空にする。行はフォルダーの名前の順(`sort.Strings`。iac が synth する順と同じ)に並べ、`byApp` にはすべての行を入れる。書き直しの表なら `drawTable(true)`、そうでなければ今の `iac.cdkWaiting` の行(外したアプリの行はすでに出ている) |
| `AppStarted(app, language)` | 行を running にし、`startedAt` と `lastLine` を今にする(済みの行に来たとき(trace の synth)も同じで、経過時間を新しく数える)。`running++`。`running` が 1 になったら `startClock()`。書き直しの表なら `drawTable(false)`、そうでなければ今の `iac.cdkStarted` の行 |
| `AppDone`、`AppFailed`、`AppToolFailed`(10節) | 行を done か failed にし、`elapsed`、`reason`、`tool` を入れる。行が running だったときだけ `running--`。`running` が 0 になったら `stopClock()`。描き方は今と同じ(書き直しの表は `drawTable(false)`) |
| `AppSkipped`(アプリの表があり、`byApp` にあるアプリ) | 行を skipped にする |
| アプリの表があり、`byApp` に無いアプリへの `AppStarted`、`AppDone`、`AppFailed`、`AppToolFailed`、`AppSkipped` | 無視する(行を足さず、状態も時計も変えない)。`--debug` のとき(今の `DebugLine` と同じ条件。ロックの中なのでロックを取らない形で書く)だけ、`display: progress for an app not in the table: <通知の名前> <Sanitize(dir)>` の1行を出す |
| アプリの表がまだ無いときの `AppStarted`、`AppDone`、`AppFailed`、`AppToolFailed` | 今の1行ずつの表示のまま(`AppsWaiting` を呼ばない今の使い方とテストを壊さない) |
| 時計の1秒 | 下の `tick` |
| `endProgress()`(結果の表示、誤り、終了の行の前。今の呼び出しのまま) | `stopClock()` を足す。`p.pendingSkipped` が残っていれば(`AppsWaiting` が来なかった。すべてのアプリを外したときなど)、書き直しの表のときだけ今の `iac.cdkSkipped` の行をそれぞれ出し、空にする |
| `Close()` | ロックの中で `endProgress()`、ロックを放したあとに `p.clockWG.Wait()` |

- `startClock()`: `c := &appClock{stop: make(chan struct{})}`、`p.clock = c`、`t := p.newTicker(time.Second)`、`p.clockWG.Add(1)` のあと、ゴルーチンで次を回す。
  ```go
  for {
      select {
      case <-t.C():
          p.mu.Lock()
          if p.clock == c { p.tick() }
          done := p.tickDone
          p.mu.Unlock()
          if done != nil { done() }
      case <-c.stop:
          t.Stop()
          p.clockWG.Done()
          return
      }
  }
  ```
- `stopClock()`: `p.clock` があれば `close(p.clock.stop)` して `p.clock = nil`。ロックの中で呼び、ゴルーチンを待ちません(待つとロックの取り合いで止まる)。ロックを待っていたゴルーチンは、`p.clock != c` を見て描かずに戻ります。
- `tick()`: 書き直しの表なら `drawTable(false)`(経過時間を書き直す。0.2 秒に1回までの今の決まりの中)。端末でないときは、running の行のうち `now - lastLine >= time.Minute` のものごとに `iac.cdkRunning`(フォルダーと経過時間)の1行を足し、`lastLine = now` にする。
- 今の決まり「`progress` の状態はロックの中でだけ使う」はそのままで、時計も同じ `p.mu` を取ります。`now` は今の `p.now`(`SetClock` で差し替えられる)です。

### 外したアプリの出し方(BR5.1、BR1.4)

外したアプリは、表の中に `[外した]`(英 `[skipped]`)の行として出します。2つの案を比べて、案 B にしました。

| 案 | やり方 | 良い点 | 悪い点 |
|---|---|---|---|
| A | `internal/iac` が `AppsWaiting` に外すアプリも入れて送り、そのあとに `AppSkipped` を送る | `Printer` が通知を溜めなくてよい | 契約 C6 の `AppsWaiting`(synth を待つアプリ)の意味が変わり、BR1.4 の順番(外すアプリの通知が先)も逆になる。1行ずつの表示で、外すアプリまで「待ち」と出る |
| B(選んだもの) | 通知の順番と意味は今のまま。`Printer` が表の無いうちに来た `AppSkipped` を `pendingSkipped` に溜め、`AppsWaiting` で表を作るときに `[外した]` の行にする | 契約 C6 と BR1.4 を変えない。1行ずつの表示は U1 のまま(NFR5.3) | `Printer` に溜める欄が1つ増える |

- 見出しの `%s` は表の行の数(外したアプリを含む)です。足の `%d/%d` は、(済み + 失敗 + 外した)の行の数 / 表の行の数で、外した行は初めから終わったものとして数えます。経過時間の欄は、外した行では空にします。

### 表の形

- 見出し: `iac.cdkTableHeader`(英 `CDK apps · %s`、日 `CDK のアプリ · %s`。`%s` はアプリの数)。
- 行: `"  " + padRight(状態の語, 10) + " " + padRight(Sanitize(dir), 24) + " " + 経過時間 + " " + 理由`。状態の語は `iac.cdkState.<state>`(英 `[waiting]`、`[running]`、`[done]`、`[failed]`、`[skipped]`、日 `[待ち]`、`[実行中]`、`[済み]`、`[失敗]`、`[外した]`。BR5.1)。色は今の `paint` の決まりで付けても、語だけで意味が分かる。
- 足: `iac.cdkTableFooter`(英 `  %d/%d apps · elapsed %s`、日 `  %d/%d アプリ · 経過 %s`)。
- 経過時間の書き方 `cdkElapsed(lang, d)`(足す): 1 分未満は今の `iac.seconds`(`42s`、`42 秒`)、1 分以上は英 `1m12s`、日 `1 分 12 秒`。U1 の1行ずつの表示の文(`iac.cdkWaiting`、`iac.cdkStarted`、`iac.cdkDone`、`iac.cdkFailed`、`iac.cdkSkipped`)と、1 分未満の書き方は変えません(NFR5.3)。

### trace の synth の包みの通知口

`internal/iac` に次の型を足します。

```go
// traceSink は trace の synth に渡す通知口。失敗を同じ経過時間の済みに読み替える(BR4.3、BR5.4)。(足す)
type traceSink struct{ inner cdk.ProgressSink }

func (s traceSink) AppsWaiting(apps []string)                          { s.inner.AppsWaiting(apps) }
func (s traceSink) AppStarted(app, language string)                    { s.inner.AppStarted(app, language) }
func (s traceSink) AppDone(app string, elapsed time.Duration)          { s.inner.AppDone(app, elapsed) }
func (s traceSink) AppFailed(app string, elapsed time.Duration, _ string) { s.inner.AppDone(app, elapsed) }
func (s traceSink) AppSkipped(app, reason string)                      { s.inner.AppSkipped(app, reason) }
```

`traceSink` は 10節の `cdk.ToolFailureSink` を実装しません。実装しないので、`Synthesizer` は trace の synth の失敗をいつも `AppFailed` で送り、`traceSink` が済みに読み替えます。単体テストで、`traceSink` が `cdk.ToolFailureSink` を満たさないことを型の確かめで固定します。

### テスト

- 単体テスト(`internal/display`): 偽の `Ticker`(テストが時刻を送るチャネル)と `SetClock` で、(a) 書き直しの表で、時計の1回ごとに経過時間が書き直される、(b) 端末でないとき、59 秒では行が増えず、60 秒で `iac.cdkRunning` の1行が増え、その後 60 秒ごとに増える、(c) 最後の running が done になると時計が止まる(`Stop` が呼ばれ、その後の時刻を送っても描かない)、(d) 済みの行に `AppStarted` が来ると running に戻り、経過時間が 0 から数え直される、(e) `Close` のあとにゴルーチンが残らない、(f) `AppSkipped` を2つ送ってから `AppsWaiting` を送ると、書き直しの表に `[外した]` の2行が名前の順で入り、足が `2/<行の数>` から始まる。1行ずつの表示では、`AppSkipped` のときに今の `iac.cdkSkipped` の行が出る、(g) 表のあとで `byApp` に無いアプリの `AppStarted` と `AppDone` を送ると、表の行も時計も変わらず、`--debug` のときだけ1行が出る、(h) `AppsWaiting` が来ないまま `endProgress` になると、書き直しの表でも外したアプリの行が出る。`tickDone` で、送った時刻の処理が終わるのを待ちます。
- -race のテスト(race の仕事で動く): 書き直しの表を有効にした `Printer` で、時計の時刻を送るゴルーチン、`AppStarted`/`AppDone` を呼ぶゴルーチン、`Error` と `DebugLine` を呼ぶゴルーチンを同時に動かし、`Close` で終える。

## 10. 道具が無いときと古いときの表示、テスト用のアプリ、CI、Dependabot、手元の手順

### 道具の名前を進み具合に渡す(BR2.1、NFR10.5)

契約 C6 の `AppFailed` は理由だけを受け取り、道具の名前を受け取りません。一方 BR2.1 と機能の仕様の例 1 は、失敗の行に「無い道具の名前と入れ方」を出すことを求めます。`cdk.ProgressSink` にメソッドを足すと、すべての実装とテストの差し替えが壊れる(R-04)ので、`internal/cdk` に任意の口を足します。

```go
// ToolFailureSink は ProgressSink を実装するものが、合わせて実装してもよい口。
// 実装していれば、Synthesize は失敗を AppFailed の代わりにこれで送る。(足す)
type ToolFailureSink interface {
    AppToolFailed(app string, elapsed time.Duration, reason, tool string)
}
```

- `Synthesizer.Synthesize` は、`failure != nil` のとき、`sink.(ToolFailureSink)` が成り立てば `AppToolFailed(app.Dir, elapsed, string(failure.Reason), failure.Tool)`、そうでなければ今の `AppFailed` を呼びます。
- `display.Printer` は `AppToolFailed` を実装し(`var _ cdk.ToolFailureSink = (*Printer)(nil)`)、`AppFailed` と同じ状態の変化に `tool` を足します。表示の文は、理由が `toolMissing` か `toolTooOld` のときだけ、道具ごとの入れ方の文を添えます(11節の一覧)。端末でないときは失敗の行の後ろに、書き直しの表では `say` で表の上に1行で出します(needsAWS の対処の行と同じ出し方)。
- これは契約 C6 への追加なので、12節の「Construction での直し」に入れます。

### テスト用のアプリ(NFR6.18、NFR8.4、BR6.1)

`internal/cdk/testdata/apps/` に次を足します。整形も lint もしません(チームの進め方)。

| フォルダー | 中身 | 依存を入れるか |
|---|---|---|
| `py-minimal` | `cdk init app --language python` の形の最小のアプリ。cdk.json の app は `python3 app.py`。`requirements.txt` はきれいな `.venv` に入れたあとの `pip freeze` の結果(すべて `==`)。`aws-cdk` だけを書いた `package.json` と `package-lock.json` | 入れる(CI と手元の手順で `.venv` と `node_modules`) |
| `ts-lookup` | 環境(架空の 12 桁のアカウントとリージョン)を決めたスタックで、`ssm.StringParameter.valueFromLookup` を使う TypeScript のアプリ。`cdk.context.json` は置かない。`package.json` は版の判定のために `aws-cdk-lib` の名前だけを持つ | 入れない。テストは写した先に `ts-minimal` の `node_modules` をつなぐ |
| `ts-profile` | cdk.json に `"profile"` を書いた TypeScript のアプリ(BR2.6 の文を照らすため) | 入れない(同上) |
| `ts-v1` | `@aws-cdk/core` を持つ package.json の TypeScript のアプリ | 入れない(synth しない) |
| `py-v1` | `aws-cdk.core` を書いた `requirements.txt` の Python のアプリ | 入れない(synth しない) |
| `unknown-version` | CDK の依存の無い package.json の TypeScript のアプリ | 入れない(synth しない) |
| `other-lang` | cdk.json の app が `mvn -e -q compile exec:java` のアプリ | 入れない(synth しない) |
| `ts-trace` | スタックのクラスの外の関数(`function makeBucket(scope: Construct) { … new s3.Bucket(scope, 'Logs', …) }`)でバケットを作り、スタックのコンストラクターで `makeBucket(this)` を呼んで `Tags.of(this).add('Owner', 'team-a')` を付ける TypeScript のアプリ(スタックのリソースはこのバケットだけにし、範囲のタグの判断(FR3.5)で外れないようにする)。今の補助のプログラムのソースのたどり方では作成の箇所が見つからないので、trace の synth と `from: "trace"` の作成の箇所を通る(5節)。作成の記録の形(BR4.2)も、このアプリの実際の出力と照らして決める | 入れない。テストは写した先に `ts-minimal` の `node_modules` をつなぐ |

- `ts-lookup` と `ts-profile` の synth は、U1 の遮断した環境で動かすので AWS には届かず、needsAWS になります(NFR8.4)。テストは `Config.Environ` に偽の `AWS_ACCESS_KEY_ID` と `AWS_PROFILE` を入れても、起動した `cdk` の `Command.Env` に無いことも確かめます。
- `.gitignore` に `internal/cdk/testdata/apps/*/.venv/` を足します(今の `node_modules/` と `cdk.out/` の行の隣)。

### testutil(足す)

- `RequirePythonTools(tb ToolTB, appDir string) bool` と `RequirePythonToolsWith(tb, appDir, look ToolLookup) bool`: `.venv` の Python(Windows は `.venv\Scripts\python.exe`、ほかは `.venv/bin/python`)、`aws_cdk_lib-*.dist-info` が `.venv` の site-packages にあること、Node.js、アプリの `node_modules/.bin/cdk`(Windows は `cdk.cmd`)を確かめます。無ければ、今の `RequireCDKToolsWith` と同じく、`CI` が空でなければ `Fatalf`、空なら `Skipf`。理由の文には README の「開発」の節の手順を指す文を入れます。
- `CopyCDKApp` は、`node_modules` と `cdk.out` に加えて `.venv` と `venv` も写さず、`.venv` があれば `node_modules` と同じく `linkFolder`(Unix はシンボリックリンク、Windows はジャンクション)でつなぎ、後片付けではリンクだけを消します(今の `removeLink`)。
- `ProcessAlive(pid int) bool`(8節)。

### CI(NFR8.3、NFR6.19、ND3-Q2)

`ci.yml` の `env` に `PY_TEST_APP: internal/cdk/testdata/apps/py-minimal` と `PYTHON_VERSION: "3.13"`(今の lint の仕事と同じ版。NFR6.16 の 3.9 以上)を足します。Go のテストのすべてを動かす仕事、つまり `test`(3つの OS × Go の2つの版)、`race`、`coverage` に、今の `setup-node` のあとに次を足します。`github-pr` は `-run '^TestGitHubPullRequest$'` だけを動かすので足しません。

```yaml
      - uses: actions/setup-python@6d4fd22b20465ccc0f7354476588e8df7c9ca0e0 # v7.1.0
        with:
          python-version: ${{ env.PYTHON_VERSION }}
          cache: pip
          cache-dependency-path: ${{ env.PY_TEST_APP }}/requirements.txt
      - name: Install the Python test CDK app
        working-directory: ${{ env.PY_TEST_APP }}
        run: |
          python -m venv .venv
          if [ "$RUNNER_OS" = "Windows" ]; then py=.venv/Scripts/python.exe; else py=.venv/bin/python; fi
          "$py" -m pip install --disable-pip-version-check --only-binary=:all: --no-input -r requirements.txt
          "$py" -m pip check
          npm ci --ignore-scripts --no-audit --no-fund
```

- `setup-node` の `cache-dependency-path` は、`ts-minimal` と `py-minimal` の `package-lock.json` の2行にします。
- `setup-python` のコミットのハッシュは、今の lint の仕事と同じものです。
- `ci-gate` の `needs` は変えません(ND3-Q2 = A)。どの仕事にも AWS の認証情報と新しい secret を足しません。
- `.github/workflows/release.yml` の `build` の仕事も `go test -mod=readonly ./...` を動かすので、同じ2つの手順を足します(足さないと、タグの公開のときに Python のテストが失敗する)。
- `release.yml` は `ci.yml` と別の `env` を持つので、その最上位の `env` にも、今の `CDK_TEST_APP` の隣に `PY_TEST_APP: internal/cdk/testdata/apps/py-minimal` と `PYTHON_VERSION: "3.13"` を足します(`ci.yml` と同じ値)。
- 固定した依存に wheel の無いものがあると Code Generation で分かったときは、そのパッケージだけを `--only-binary=:all:` の例外にする理由を Code Generation の計画に書き、人に確かめます(NFR6.19)。

ワークフローを読むテスト(`internal/cli/workflow_test.go` に足す): (a) Go のテストのすべてを動かす仕事(`ci.yml` の `test`、`race`、`coverage` と `release.yml` の `build`)のそれぞれに `actions/setup-python` と、`--only-binary=:all:` と `pip check` のある `py-minimal` の手順があり、npm は `--ignore-scripts` であること(今の `TestEveryJobThatRunsGoTestInstallsNode` を広げる)。Python の確かめの対象の仕事は、`run` に `go test` を含み、`-run` を含まない手順を持つ仕事とします(`github-pr` は `go test ... -run '^TestGitHubPullRequest$'` なので外れる)。Node.js と npm の確かめは、今のまま `go test` を含むすべての仕事を見ます。Python の確かめの対象の仕事が4つ(`ci.yml` の `test`、`race`、`coverage` と `release.yml` の `build`)より少なければ、ワークフローを読めていないとして失敗にします。(b) どのワークフローにも `AWS_` で始まる `env` の鍵と `aws-actions/configure-aws-credentials` が無く、`secrets.` の参照が今の `github-pr` の1か所だけであること。

### Dependabot(NFR6.18)

`.github/dependabot.yml` に2つを足します。ほかのテスト用のアプリ(依存を入れないもの)は足しません。

| ecosystem | directory | 間隔 | コミットの接頭辞 |
|---|---|---|---|
| pip | `/internal/cdk/testdata/apps/py-minimal` | 毎週 | `chore(deps)` |
| npm | `/internal/cdk/testdata/apps/py-minimal` | 毎週 | `chore(deps)` |

### 手元の手順(NFR8.3、ND3-Q1 = A)

テストは `.venv` を作りません。`README.md` の「## 開発」の節に、`internal/cdk/testdata/apps/py-minimal` で動かす次の3行を、Unix と Windows(PowerShell)に分けて書きます。

```sh
python3 -m venv .venv
.venv/bin/python -m pip install --only-binary=:all: -r requirements.txt
npm ci --ignore-scripts
```

```powershell
py -m venv .venv
.venv\Scripts\python.exe -m pip install --only-binary=:all: -r requirements.txt
npm ci --ignore-scripts
```

`.venv` が無いとき、手元ではテストが `RequirePythonTools` の理由を出して飛ばされ、CI では失敗します。

## 11. 互換と言語(NFR5.3、NFR10.5)

### 互換(NFR5.3)

- 変更案に stack-name のタグのリソースが無いときは、今のとおり `prepareCDK` が synth しません。`--cdk-app` があっても、7節の確かめだけを行い、誤りが無ければ出力と終了コードは今の `iac` と同じです。
- U1 だけの一時的な扱いを消します: `iac.ReasonNotInU1`、`runner.synthesizeFirst`、`Result.ExitCode` の `a.FailureReason != ReasonNotInU1` の条件、`display` の `iac.cdkWhyNotInU1` と `cdkWhy` の行。
- U1 のテストのうち、`internal/iac/cdk_test.go` の `TestCDKFailedAppsAndTheExitCode` のサブテスト `the second app`(NFR3.8)と、analyze が1回であることを数えるテスト(NFR3.5)は、U3 の複数のアプリのテストと 8節の回数のテストに置き換えます。`internal/display/iacpr_test.go` の `ReasonNotInU1` を使う行も消します。ほかの U1 と前の intent の受け入れ条件のテストは、そのまま通します。
- 1行ずつの表示の文と、1 分未満の経過時間の書き方は U1 のままなので(9節)、アプリが1つの TypeScript のリポジトリの標準エラーの行は U1 と同じです。
- `iac --help`(`internal/cli/iac.go` の `iacLong`)と `README.md` の `iac` の節の CDK の段落を、すべてのアプリと Python のアプリ、`--cdk-app`、`--synth-timeout` の説明に直します(契約の決まり: X1 を変えるときは README を同じプルリクエストで直す)。

### 言語(NFR10.5)

U3 で足す文は、`internal/display` に `iaccdk.go`(足す)を置き、`iacCDKCatalog`(足す)に英語と日本語の両方で入れ、`init` で `catalog` に足します(今の `iacPRCatalog` と同じ形)。今の文の一覧のキーの抜けを確かめるテストが、新しいキーも確かめます。

| ID | 英語 | 日本語 |
|---|---|---|
| `iac.cdkWhyToolTooOld` | `the program is too old` | `プログラムの版が古すぎます` |
| `iac.cdkToolHintNode` | `install Node.js 18 or later` | `Node.js 18 以上を入れてください` |
| `iac.cdkToolHintCDK` | `run npm install in the app, or npm install -g aws-cdk` | `アプリのフォルダーで npm install を実行するか、npm install -g aws-cdk で入れてください` |
| `iac.cdkToolHintPython` | `install Python 3` | `Python 3 を入れてください` |
| `iac.cdkToolHintPythonOld` | `install Python %s or later` | `Python %s 以上を入れてください` |
| `iac.cdkToolNotFound` | `%s was not found` | `%s が見つかりません` |
| `iac.cdkRunning` | `%s: running (%s)` | `%s: 実行中(%s)` |
| `iac.cdkTableHeader`、`iac.cdkTableFooter`、`iac.cdkState.*` | 9節 | 9節 |
| `iac.cdkWhyNotTypeScript` | `neither TypeScript nor Python`(今の文を直す) | `TypeScript でも Python でもありません`(同) |
| `iac.badSynthTimeout` | `--synth-timeout %s is not a duration of 1s or more` | `--synth-timeout %s は 1 秒以上の時間ではありません` |
| `iac.badSynthTimeoutFix` | `give it like 90s, 10m or 1h30m` | `90s、10m、1h30m のように指定してください` |
| `iac.cdkAppOutside` | `--cdk-app %s points outside --dir` | `--cdk-app %s は --dir の外を指しています` |
| `iac.cdkAppNotFound` | `--cdk-app %s is not a CDK app found below --dir` | `--cdk-app %s は --dir の下で見つかった CDK のアプリではありません` |
| `iac.cdkAppFix` | `give the folder of a cdk.json relative to --dir; iac does not look in .git, node_modules, cdk.out, .venv, venv or links` | `--dir からの相対で cdk.json のあるフォルダーを指定してください。.git、node_modules、cdk.out、.venv、venv、リンクの下は探しません` |

needsAWS の対処(`iac.cdkNeedsAWSFix`)と、どのアプリか分からないリソースの理由(`reason.cdkAppFailed`、`reason.cdkV1`、`reason.cdkUnsupportedLanguage`)は今の文を使います。`--lang ja` と `--lang en` の受け入れテストで、Python が無いアプリ、古い Python のアプリ、`--cdk-app` と `--synth-timeout` の誤り、needsAWS のアプリの文がそれぞれの言語で出ることを確かめます。

## 12. Construction での直し(契約と機能の設計に足すもの)

U3 の Code Generation の計画で、次を契約の「Construction での直し」の表(`inception/contract-design/contract-summary.md`)に1行ずつ足し、機能の設計の文書と合わせます。

| 足す先 | 足すもの |
|---|---|
| 契約 C2 の `Failure.Reason` | `toolTooOld`(3節) |
| 契約 C2 の `Options`、`Synthesis` | 今のコードに無い `WithTrace` と `TraceSites` を、契約のとおり足す(5節) |
| 契約 C7 の `failure_to_manual` | `toolTooOld: {status: failed, manual: cdkAppFailed}`(道具 python) |
| 契約 C6 | 任意の口 `cdk.ToolFailureSink`(10節)。`cdk.ProgressSink` のメソッドは変えない |
| 契約 X1 の `input_errors` | `--cdk-app` の誤りを「`--dir` の外を指す、または発見したアプリの一覧に無い」と書き直す(7節) |
| `entities.md` の SynthFailure | `reason` の `allowed` に `toolTooOld`、`tool` の説明に「版の確かめの時間切れは python」と「python のアプリで app の命令の最初の語が python 系でなく見つからないときは、その語の最後の要素(`toolName`)」 |
| 契約 C2 の `Failure.Tool` | `cdk`、`node`、`python` に加えて、上と同じ `toolName` の値がある(2節) |
| `rules.md` の BR3.2 の 1 | 添える道具の名前の例「(node、cdk、python)」に、python のアプリの app の命令の最初の語(`toolName`)を足す(2節) |
| 表示の対応表 `cdkWhy` と文の一覧 | `toolTooOld` と 11節の文(`iac.cdkToolNotFound` を含む) |
| 契約 C3 の `Recheck` | `Recheck(ctx, App, edited, traces []TraceSite)` に変える。`traces` は、そのアプリの2回目の Analyze に渡した `TraceSites`(使わなかったアプリでは空)。`construct_rules` に「Recheck でも同じ優先順位で `traceSites` を使う。recheck のときだけ、記録の行に `new` の式が無ければ、2つ目の引数の文字列がパスの最後の要素と同じ、行が最も近い `new` の式を使う」と、「書き換えのあとの比べ方では、From が trace の作成の箇所はパスとファイルだけを比べ、行は比べない」を足す(5節) |
| 契約 C4 の要求の `traceSites` | 説明の「analyze のとき」を「analyze と recheck のとき」に直す。JSON の形と `version` は変えない(5節) |
| 補助のプログラム `internal/cdk/helper/ts-helper.js` | 要求の `traceSites` を読み、ソースで見つからないパスに `from: "trace"` の作成の箇所を出す(5節。契約 C3 の `construct_rules` の実装で、今のコードに無い部分) |
| 契約 X2 の `resource_name` | 「synth に失敗したか外したアプリはアプリのフォルダーとリソースの種類」を、BR3.2(どのアプリかは項目に書かない、FD3-Q3)に合わせて「どのアプリのものか分からないリソースも、ほかと同じく論理 ID(無ければ今の `resourceName` のとおり ARN の最後の要素)で、アプリのフォルダーは書かない」に直す(6節) |
| NFR3.12 の要件の文(`nfr-requirements/security-requirements.md`) | 見積もりの式を「版の確かめ(Python 30 秒)+ synth の上限 × 2 +(Node.js の版の確かめ 30 秒 + 補助のプログラムの上限 2 分)× 3」に直す。今の `Analyzer` は補助のプログラムの呼び出しごとに `node --version` を動かすため(8節)。既定の上限では、アプリ1つあたり 27 分から 28 分になる |
| NFR6.17 の要件の文(`nfr-requirements/tech-stack-decisions.md`) | 「パスに空白があれば二重引用符で囲む」に、「空白が無くても、引用符の外でシェルが読み替える文字(4節の一覧)があれば囲む」と、「Windows では Code Generation の最初の確かめで引用符つきの引数が `cdk.cmd` に届かないと分かったら、囲む代わりに置き換えない」を足す(4節) |

## 13. 残るリスク

- `cdk synth` は利用者のアプリのコードと、`.venv` の中の利用者が入れたパッケージを動かします。利用者が自分で `cdk synth` を動かすときと同じ信頼の範囲です。README と `iac --help` の「信頼できるリポジトリでだけ使う」の文を、Python のアプリにも当てはまる書き方にします(U1 の7節のまま)。
- 親の環境の `PYTHONHOME` と `PYTHONPATH` は、NFR2.5 のとおりそのまま渡ります。これらが `.venv` と合わない値だと synth が失敗し、synthError になります。`--debug` の環境の名前の行で分かります。
- 4節で置き換えないとき(フォルダーの名前に Unix の `"`、`$`、`` ` ``、`\`、Windows の `"`、`%` を含むとき)、Windows で app の命令が `python3 app.py` だと、PATH のほかの `python3`(Microsoft Store の案内の偽物を含む)が使われることがあります。版の確かめで toolMissing になるか、synth のエラーになり、`--debug` で使った命令が分かります。README に、cdk.json の app を `.venv\Scripts\python.exe app.py` のように書けば動くことを書きます。
- `windowsQuotedApp` を false にした場合(4節)は、空白などを含むフォルダーの下の Windows のアプリでも、上と同じことが起きます。
- Python の最小の版より新しすぎる Python で `aws-cdk-lib` が動かないことは見つけられず、synthError になります。利用者のアプリが `py-minimal` と違う版の `aws-cdk-lib` を使うと、その版の下限と `minPython` がずれることがあります。
- needsAWS の文の一覧は、CDK と AWS の SDK の版で変わることがあり、合わない文は synthError になります。一覧はテストで固定し、Code Generation で `ts-lookup` と `ts-profile` の実際の出力と照らします。
- 作成の記録の形は Code Generation で決めます(BR4.2)。読めない形のときは TraceSite が作られず、集めたリソースは cdkNoSource になります(誤った箇所を書き換える方向には失敗しない)。
- trace 由来の作成の箇所は、Recheck で書き換えにより行がずれると、2つ目の引数が文字列の `new` の式だけを探し直せます(5節)。ID を式で作るコンストラクトの上で行がずれる書き換えは、Recheck で食い違いになり、そのアプリの編集は cdkUnsupportedShape で手で直す一覧に出ます(誤った箇所を書き換える方向には失敗しない)。
- 版の確かめは、`.venv` か PATH で選んだ実行ファイルを `--version` で起動します。synth で同じ実行ファイルが起動されるので、信頼の範囲は synth と同じです。
- 時計のゴルーチンは `Close` で待ちますが、`Close` を呼ばずにプロセスが終わる道(パニックなど)では待ちません。プロセスの終わりで消えるので、残るものはありません。

## 14. 脅威と手当て(STRIDE の考え方で、関係するものだけ)

| 脅威 | 例 | 手当て | 節 |
|---|---|---|---|
| なりすまし | 親のフォルダーや PATH の先の `.venv` に置かれた別の Python が使われる | アプリの直下の `.venv` だけを見る | 2節(NFR2.6) |
| なりすまし | Microsoft Store の案内の偽物の `python3` を Python とみなす | 版の文字が読めない答えを toolMissing にする | 3節(NFR6.16) |
| 改ざん | `--cdk-app ../../other` やリンクで `--dir` の外のアプリを synth させる | 発見したアプリの一覧との一致だけで選び、ファイルシステムをたどらない | 7節(NFR2.7) |
| 改ざん | `--app` の置き換えで、フォルダーの名前の `$(...)` や `&` がシェルで命令として読まれる | tag-refactor はシェルを通さない。シェルが読み替える文字があれば置き換えないか、二重引用符で囲む | 4節(NFR6.17) |
| 改ざん | tag-refactor が利用者の cdk.json や `.venv` を書き換える | `--app` で渡し、cdk.json と `.venv` に書かない | 2節、4節(NFR2.6、NFR6.17) |
| 改ざん | CI でパッケージを入れるときに、パッケージのコードが動く | pip は `--only-binary=:all:`、npm は `--ignore-scripts` | 10節(NFR6.19) |
| 情報の漏えい | Python のアプリの synth と版の確かめに AWS の認証情報が渡り、boto3 などで AWS を呼ぶ | U1 の遮断した環境をそのまま使い、足すのは PATH と VIRTUAL_ENV だけ | 2節(NFR2.5) |
| 情報の漏えい | 作成の記録の絶対パス(利用者の名前を含む)が本文や表示に入る | TraceSite はアプリからの相対だけ。絶対パスは関数の外に出さない | 5節(NFR1.11) |
| 情報の漏えい | `--debug` の行に利用者の名前を含むホームのパスが出る | `shortHome` で `~` にする | 5節(NFR1.12) |
| 情報の漏えい | synth の標準エラーのアカウント ID、ARN、トークンが本文や結果の表示に入る | `Failure.Detail` を `CDKApp` に写さず、`--debug` にだけ伏せ字を通して出す | 6節(NFR1.13) |
| 情報の漏えい | CI のテストが実際の AWS に届く | CI に AWS の認証情報を置かず、ワークフローを読むテストで確かめる | 10節(NFR8.4) |
| サービスの妨害 | 大きな `.venv` を辿って発見が終わらない | `.venv` と `venv` に入らない、深さ 32 まで | 7節(NFR3.15) |
| サービスの妨害 | 応答しない `python --version` や synth で止まる | 30 秒と `--synth-timeout` の上限、プロセスの木ごと止める | 3節、8節(NFR3.14、NFR3.11、NFR3.16) |
| 権限の昇格 | 利用者のアプリのコードが tag-refactor と同じ権限で動く | 利用者が自分で `cdk synth` を動かすのと同じ範囲として受け入れ、README で知らせる | 13節 |

否認の脅威は、利用者の端末で利用者が動かす CLI なので当てはまりません。どの Python を使ったかは `--debug` で分かります(5節)。

## 15. 合否の基準を機械で確かめる方法

| 要件 | テスト |
|---|---|
| NFR1.11 | 5節の `trace_test.go`(両方の OS の形、アプリの外、4つのフォルダー)と、`internal/cli` の受け入れテスト(本文、コミットのメッセージ、表示に `alice` が出ない) |
| NFR1.12 | 5節の `home_test.go`(`shortHome` の境目の表、`homeForms` の実際のパスの形、Unix の実際のリンク)と `internal/cdk` の単体テスト(Windows の大文字と小文字、置き換えた命令の `--debug` の行に `~` と `.venv` があり、ホームの名前が無い、PATH だけのときの `.venv` の行)。Python の差し替えの標準エラーに入れたトークンが伏せ字になる |
| NFR1.13 | 6節の受け入れテスト |
| NFR2.5 | 2節の `withVenv` の表のテストと、差し替えた実行の口の環境のテスト(synth、trace、版の確かめ) |
| NFR2.6 | 2節のテスト(親のフォルダーだけの `.venv`、synth のあとの `.venv` の一覧) |
| NFR2.7 | 7節の受け入れテスト |
| NFR3.11、NFR6.21 | `parseSynthTimeout` の単体テスト。終わらない偽の cdk に `--synth-timeout 1s` で、約1秒で止まり孫が残らない(3つの OS)。`0s`、`500ms`、`999ms`、`10`、`abc` が終了コード 1 |
| NFR3.12 | README の `iac` の節に、8節の式(Python の版の確かめ、synth の2回、Node.js の版の確かめと補助のプログラムの3回)があることを、人がレビューで確かめる(要件の式の直しは 12節) |
| NFR3.13、NFR3.17 | 8節の回数のテスト(Recheck の回数と、Recheck に渡る `TraceSites` を含む)と、5節の補助のプログラム、`compareAnalyses`、Recheck のテスト |
| NFR3.14 | 3節の差し替えのテスト(`ErrTimeout` で timeout、synth を呼ばない)と、8節の `hang` の版の確かめのテスト |
| NFR3.15 | 7節の `readDir` を数えるテストと、`Discover` が呼ばれないテスト |
| NFR3.16 | 8節の `hang` の synth と版の確かめのテスト(時間切れと中断、3つの OS) |
| NFR4.5 | 8節の後片付けのテスト(Analyze と Recheck の途中の中断を含む) |
| NFR5.3 | 前の intent と U1 の受け入れ条件のテストがそのまま通る(11節で置き換えるものを除く) |
| NFR6.16 | 3節の差し替えのテストと `TestMinPythonMatchesTheTestApp` |
| NFR6.17 | 4節の `appCommand` の表のテスト、Code Generation の最初に Go の `stable` と `oldstable` で動かす `TestQuotedArgReachesCmd`(Windows)、空白と `$` を含むフォルダーの `py-minimal` の synth(3つの OS) |
| NFR6.18 | `py-minimal/requirements.txt` のすべての行が `==` であることを読むテストと、CI の `pip check`。Dependabot の設定を読むテスト(pip と npm の `py-minimal` がある) |
| NFR6.19、NFR8.3 | 10節のワークフローを読むテストと、3つの OS の CI で `py-minimal` の synth のテストが通ること |
| NFR6.20、NFR10.4 | 9節の単体テスト(外したアプリの行と、表に無いアプリの通知を含む)と -race のテスト |
| NFR8.4 | `ts-lookup` と `ts-profile` の synth が needsAWS になるテスト(偽の AWS の変数が `Command.Env` に無い)と、10節のワークフローを読むテスト |
| NFR10.5 | 11節の `--lang ja` と `--lang en` の受け入れテストと、文の一覧のキーの抜けのテスト |
