# CI と配布の流れ: U3 CDK の synth と対応づけ(残り)

tag-refactor は利用者の端末で動く CLI です。U3 でも、本番のインフラ、サーバー、AWS のリソースは足しません。U1 と U2 の CI と配布の流れ(`construction/u1-walking-skeleton/infrastructure-design/cicd-pipeline.md`、`construction/u2-pull-request/infrastructure-design/cicd-pipeline.md`)をそのまま使い、U3 では、Go のテストを動かす仕事に Python のテスト用のアプリの準備を足し、Dependabot に `py-minimal` を足します。ここには、その変更を今の YAML のどこに入れるかと、U3 のテストが CI のどこで動くかを書きます。元にしたものは次のとおりです。

- U3 の非機能の設計: `construction/u3-cdk-synth/nfr-design/security-design.md`(10節を中心に、2節〜5節、8節、9節)と `logical-components.md`
- 機能の流れ: `construction/u3-cdk-synth/functional-design/functional-spec.md`
- 契約: `inception/contract-design/contract-summary.md`(C2、C3、C5)
- 今の CI の定義: `.github/workflows/ci.yml`、`release.yml`、`scheduled.yml`、`.github/dependabot.yml`
- このステージの質問の回答: `infrastructure-design-questions.md` の Q1 = A(CI の Python は lint の仕事と同じ `"3.13"`)。要約は人が確認済みです。

## 1. U3 のテストが動く仕事

| U3 のテスト | 置くパッケージ | 動く仕事 | 前提 | 要件 |
|---|---|---|---|---|
| 作成の記録のパスの単体テスト(`trace_test.go`)、`--debug` の行と `shortHome` の単体テスト(`home_test.go`) | `internal/cdk` | `ci.yml` の test(3つの OS × Go の2つの版)、race、coverage、`release.yml` の build | なし。Unix の実際のリンクのテストは Windows で飛ぶ | NFR1.11、NFR1.12 |
| 本文と表示にアプリの状態の詳しい中身が出ない受け入れテスト | `internal/cli` | 同じ | 偽の cdk | NFR1.13 |
| `withVenv` と環境の単体テスト、`.venv` の選び方と `.venv` が変わらないテスト | `internal/cdk` | 同じ | `py-minimal` の `.venv`(2節の準備) | NFR2.5、NFR2.6 |
| `--cdk-app` の受け入れテスト(リンクとジャンクションを含む) | `internal/cli` | 同じ | なし | NFR2.7 |
| `--synth-timeout` の単体テストと、終わらない偽の cdk のテスト | `internal/cli`、`internal/cdk` | 同じ | `TestMain` の偽の python と cdk | NFR3.11、NFR6.21 |
| 起動の回数、補助のプログラムの `traceSites`、`compareAnalyses`、Recheck のテスト | `internal/iac`、`internal/cdk` | 同じ | `ts-trace` に `ts-minimal` の `node_modules` をつなぐ | NFR3.13、NFR3.17 |
| Python の版の確かめの差し替えのテストと `TestMinPythonMatchesTheTestApp` | `internal/cdk` | 同じ | `py-minimal` の `.venv` の `aws_cdk_lib-*.dist-info/METADATA`。無ければ CI では失敗 | NFR3.14、NFR6.16 |
| 発見で `.venv` と `venv` に入らないテスト | `internal/cdk`、`internal/iac` | 同じ | なし | NFR3.15 |
| 子まで止めるテスト(`hang` の版の確かめと synth) | `internal/cdk` | 同じ(3つの OS) | `TestMain` の偽物 | NFR3.16 |
| 後片付けと中断のテスト | `internal/cdk`、`internal/iac` | 同じ | なし | NFR4.5 |
| 前の intent と U1 の受け入れ条件のテスト(置き換えるものを除く) | 各パッケージ | 同じ | 今のまま | NFR5.3 |
| `appCommand` の表のテスト、空白と `$` を含むフォルダーの `py-minimal` の synth | `internal/cdk` | 同じ。`$` のフォルダーは Unix だけ | `py-minimal` の `.venv` と `node_modules` | NFR6.17 |
| `TestQuotedArgReachesCmd`(引用符つきの引数が `cdk.cmd` に届くか) | `internal/cdk` | test の Windows(Go の `stable` と `oldstable`)。ほかの OS では飛ぶ | なし。Code Generation の最初のプルリクエストで結果を見る(security-design.md の4節) | NFR6.17 |
| `requirements.txt` の行がすべて `==` であることと、Dependabot の設定を読むテスト | `internal/cli`(`workflow_test.go`)、`internal/cdk` | 同じ | なし | NFR6.18 |
| ワークフローを読むテスト(3節) | `internal/cli` | 同じ | なし | NFR6.19、NFR8.3、NFR8.4 |
| 時計と進み具合の表の単体テスト、-race のテスト | `internal/display` | 同じ。-race のテストは race の仕事で意味を持つ | なし | NFR6.20、NFR10.4 |
| `py-minimal` の実際の synth | `internal/cdk`、`internal/cli` | 同じ(3つの OS) | `py-minimal` の `.venv`、`node_modules` の CDK の CLI、Node.js | NFR8.3 |
| needsAWS のテスト(`ts-lookup`、`ts-profile`)と、v1、版が分からない、ほかの言語のアプリのテスト | `internal/cdk`、`internal/cli` | 同じ | `ts-lookup` と `ts-profile` に `ts-minimal` の `node_modules` をつなぐ。AWS の認証情報は無い | NFR8.4 |
| `--lang ja` と `--lang en` の受け入れテスト | `internal/cli` | 同じ | なし | NFR10.5 |
| README の見積もりの式 | なし(人がレビューで確かめる) | なし | なし | NFR3.12 |

`github-pr` の仕事は、`-run '^TestGitHubPullRequest$'` で U1 と U2 の結合テストだけを動かします。U3 のテストはここでは動かないので、Python の準備の対象にしません(2節)。

## 2. CI の定義の変更

### 2.1 `ci.yml` の最上位の `env`

今の `CDK_TEST_APP` の行のすぐ後ろに、次の2行を足します。

| キー | 値 | 理由 |
|---|---|---|
| `PY_TEST_APP` | `internal/cdk/testdata/apps/py-minimal` | Python のテスト用のアプリの場所。手順とキャッシュの鍵で使う |
| `PYTHON_VERSION` | `"3.13"` | lint の仕事の `python-version: "3.13"` と同じ(Q1 = A)。パッチの版は実行のたびに最新になる。NFR6.16 の 3.9 以上を満たす |

lint の仕事の `python-version: "3.13"` は変えません(書き方をそろえるための書き換えもしない)。2つの値が同じであることは、3節のテストで確かめます。

### 2.2 `ci.yml` の test、race、coverage の仕事

3つの仕事は、今どれも「checkout → setup-go → setup-node → Install the test CDK app → go test」の順です。次のように変えます。

1. `actions/setup-node` の `cache-dependency-path` を、1行から2行(`${{ env.CDK_TEST_APP }}/package-lock.json` と `${{ env.PY_TEST_APP }}/package-lock.json`)にします。npm のキャッシュの鍵に `py-minimal` の lock も入ります。
2. 今の `Install the test CDK app` の手順のすぐ後ろ、go test の手順の前に、次の2つの手順を足します。

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

- アクションのハッシュは、今の lint の仕事の `setup-python` と同じもの(`6d4fd22b20465ccc0f7354476588e8df7c9ca0e0`、`v7.1.0`)です。Dependabot の github-actions の更新で、lint の仕事と一緒に上がります。
- `defaults.run.shell: bash` なので、windows-latest でも同じ `run` が Git Bash で動きます。`.venv` の Python の場所だけを `RUNNER_OS` で分けます。
- `.venv` の Python を直接呼ぶので、仕事の PATH と VIRTUAL_ENV は変えません(テストの側で、遮断した環境に `.venv` の分を足すことを確かめるため。security-design.md の2節)。
- pip のキャッシュは `setup-python` の `cache: pip` に任せ、鍵は `requirements.txt` です。`.venv` そのものはキャッシュしません(壊れた `.venv` を引き継がないため)。
- 順番の理由: Python の準備を Node.js の準備のあとに置くのは、`py-minimal` の `npm ci` が `setup-node` の Node.js を使うためです。go test の前に置くのは、道具が無いと CI ではテストが失敗する作り(`testutil.RequirePythonTools`)だからです。
- test の仕事は `CGO_ENABLED: "0"`、race は `"1"` のままです。Python の準備は cgo と関係しません。
- 仕事の `timeout-minutes: 30` は変えません(5節の見込みで足りる)。

### 2.3 `ci.yml` の変えない仕事

| 仕事 | 変えない理由 |
|---|---|
| `github-pr` | go test を `-run '^TestGitHubPullRequest$'` で絞り、U3 のテストを動かさない。Python の手順を足さず、秘密の値を渡す手順の数も増やさない |
| `lint` | Python は cfn-lint のためにだけ入れている。U3 で変えない |
| `vuln`、`modcheck`、`build`、`prettier`、`artifacts` | go test を動かさない。`prettier` の対象(`internal/cdk/helper/` と `test/verify/*.js`)は、`ts-helper.js` の変更も今のまま確かめる |
| `ci-gate` | `needs: [test, race, coverage, lint, vuln, modcheck, build, prettier, github-pr]` を変えない。Python の手順は、待っている仕事の中に足すので、新しい仕事も新しい必須のチェックも増えない(ND3-Q2 = A) |

`scheduled.yml`(毎週の govulncheck)も go test を動かさないので変えません。

### 2.4 `release.yml` の build の仕事

- 最上位の `env` の `CDK_TEST_APP` の行のすぐ後ろに、`ci.yml` と同じ `PY_TEST_APP` と `PYTHON_VERSION: "3.13"` を足します。`release.yml` は `ci.yml` の `env` を引き継がないためです。
- `setup-node` の `cache-dependency-path` を2行にします(2.2 の1と同じ)。
- 今の `Install the test CDK app`(`npm ci --ignore-scripts --prefix "$CDK_TEST_APP"`)のすぐ後ろ、`Require Go ... or later` の前に、2.2 と同じ `setup-python` と `Install the Python test CDK app` の2つの手順を足します。`Test (no AWS calls)` の手順(`go test -mod=readonly ./...`)より前になります。
- この仕事は、ビルドのあとに「作業の木が変わっていない(`vcs.modified=false`、`+dirty` でない)」ことを確かめます。`py-minimal` の `.venv/` と `node_modules/` が git に無視されていないと、ここで公開が止まります。そのため、`.gitignore` に `internal/cdk/testdata/apps/*/.venv/` を足す変更(security-design.md の10節。`node_modules/` は今の行で無視される)を、この手順と同じプルリクエストに入れます。
- `smoke` と `publish` の仕事は変えません(テストを動かさない)。
- `release.yml` は `pull_request` の `paths` に自分自身を含むので、U3 でこのファイルを変えるプルリクエストでは、公開を除いたすべての手順が下書きとして動き、Python の手順も確かめられます。

### 2.5 Dependabot(`.github/dependabot.yml`)

今の ts-minimal の npm の項目の後ろに、次の2つを足します。

| package-ecosystem | directory | schedule.interval | commit-message.prefix | そのほか |
|---|---|---|---|---|
| `pip` | `/internal/cdk/testdata/apps/py-minimal` | `weekly` | `chore(deps)` | `groups` で、この項目の更新をすべて1つのプルリクエストにまとめる(名前 `py-minimal-pip`、`patterns: ["*"]`) |
| `npm` | `/internal/cdk/testdata/apps/py-minimal` | `weekly` | `chore(deps)` | なし(中身は CDK の CLI の `aws-cdk` だけ) |

- `groups` を付けるのは、`requirements.txt` が `pip freeze` の結果(推移的な依存もすべて `==`)だからです。`aws-cdk-lib` だけを上げるプルリクエストは、それが求める `jsii` などの新しい版と、固定した古い版がぶつかり、`pip install` か `pip check` で落ちることがあります。まとめて上げれば、そろった組み合わせになります。
- ts-minimal の npm の項目の `typescript` の除外は、`py-minimal` の npm には要りません(TypeScript を持たない)。
- 依存を入れないテスト用のアプリ(`ts-lookup`、`ts-profile`、`ts-trace`、`ts-v1`、`py-v1`、`unknown-version`、`other-lang`)は足しません。`py-v1` の `requirements.txt` は v1 の判定のためだけのもので、入れないので更新もしません。
- Dependabot のプルリクエストには秘密の値が渡りませんが、Python の準備と U3 のテストは秘密の値を使わないので、そのまま動きます。

## 3. ワークフローを読むテストに足す確かめ

`internal/cli/workflow_test.go` に足します。YAML は今のテストと同じ読み方(`gopkg.in/yaml` の構造体)で読みます。

| 確かめ | 対象 | 合格の条件 |
|---|---|---|
| Python の準備(`TestEveryJobThatRunsAllGoTestsInstallsPython`、足す) | `run` に `go test` を含み、`-run` を含まない手順を持つ仕事。今の定義では `ci.yml` の `test`、`race`、`coverage` と `release.yml` の `build` の4つ(`github-pr` は `-run` があるので外れる) | その go test の手順より前に、`uses` が `actions/setup-python@` で始まる手順と、`--only-binary=:all:`、`pip check`、`npm ci` と `--ignore-scripts` を含み、`working-directory` か `run` が `PY_TEST_APP` を指す手順がある。対象の仕事が4つより少なければ失敗(ワークフローを読めていない) |
| Node.js の準備(今の `TestEveryJobThatRunsGoTestInstallsNode`) | 今のまま、`go test` を含むすべての仕事 | 今のまま。`setup-node` の `cache-dependency-path` に2つの `package-lock.json` があることを足す |
| Python の版 | `ci.yml` と `release.yml` の最上位の `env` | `PY_TEST_APP` が `internal/cdk/testdata/apps/py-minimal`、`PYTHON_VERSION` が `ci.yml` の lint の仕事の `python-version` と同じ |
| アクションの固定 | すべてのワークフローのすべての `uses` | `@` のあとが 40 桁の16進数(チームの進め方。足す `setup-python` も含む) |
| AWS と秘密の値(今の `TestTheTokenReachesOneStepOnly` を広げる) | すべてのワークフロー | `AWS_` で始まる `env` のキーと `aws-actions/configure-aws-credentials` が無い。`secrets.` の参照は今の `github-pr` の1か所だけ。最上位の `permissions` は `contents: read` のまま |
| 必須のチェック | `ci.yml` の `ci-gate` | `needs` が今の9つの仕事と同じ |
| Dependabot | `.github/dependabot.yml` | `pip` と `npm` の項目が `/internal/cdk/testdata/apps/py-minimal` にあり、`weekly` と `chore(deps)`。`pip` の項目に `groups` がある |
| 固定した依存(`internal/cdk` に置く) | `py-minimal/requirements.txt` | 空でない行(注釈を除く)がすべて `名前==版` |

## 4. 段階と合否

U1 の表(整形と lint、テスト、race、カバレッジ 80%、脆弱性、結合テスト、ビルド)のままです。U3 で足すコード(`python` を呼ぶ部分も差し替えてテストする)もカバレッジに入ります。

| 段階 | 仕事 | 止める条件 | U3 での違い |
|---|---|---|---|
| テスト | `test`(3つの OS × 2つの版) | テストが1つでも失敗 | Python の準備が失敗するか、道具が無くてテストが失敗すると止まる |
| race | `race` | 競合の報告かテストの失敗 | 時計のゴルーチンの -race のテストが加わる |
| カバレッジ | `coverage` | 文の 80% 未満 | 足したコードも数える |
| 公開の前のテスト | `release.yml` の `build` | テストの失敗、作業の木の変更 | 同じ準備を足す。`.venv` が無視されていないと止まる |
| まとめ | `ci-gate` | 待つ仕事のどれかが成功でない | 変えない |

`py-minimal` の準備は、どの仕事でも go test の前に終わっているので、`testutil.RequirePythonTools` は CI では「無いので飛ばす」にならず、無ければ失敗にします(NFR8.3、チームの進め方)。

## 5. 実行の時間

### 今の時間(測った値)

`main` の最新の成功した実行(`ci.yml`、2026-11-15、実行 43684415189)の、仕事と手順の開始と終了の時刻から数えました(`gh run view <id> --json jobs`)。パッケージごとの時間は、ログの `ok <パッケージ> <秒>` の行です。

| 仕事 | 仕事の全体 | go test の手順 | 大きいパッケージ(stable) |
|---|---|---|---|
| test ubuntu(stable / oldstable) | 146 秒 / 117 秒 | 124 秒 / 100 秒 | `internal/cli` 65 秒、`internal/iac` 34 秒、`internal/cdk` 2 秒 |
| test macOS(stable / oldstable) | 189 秒 / 75 秒 | 148 秒 / 53 秒 | `internal/cli` 80 秒、`internal/iac` 36 秒、`internal/cdk` 3 秒 |
| test Windows(stable / oldstable) | 331 秒 / 260 秒 | 255 秒 / 178 秒 | `internal/cli` 155 秒、`internal/iac` 92 秒、`internal/cdk` 5 秒 |
| race | 198 秒 | 183 秒 | `internal/cli` 107 秒 |
| coverage | 148 秒 | 129 秒 | なし |

### 増える分の見込み(推定)

数え方: 増える時間 = 準備の手順の時間 + テストの時間の増え方です。go test はパッケージを並べて動かすので、テストの時間の増え方は、各パッケージの増えた分の合計ではなく、最も長くなるパッケージで決まるとみなします。新しい実際の synth の回数は、security-design.md のテストから数えます(仕事ごとに、Python の synth が Unix で約4回、Windows で約3回(`$` のフォルダーを除く)。TypeScript の synth が `ts-lookup`、`ts-profile`、`ts-trace` の normal と trace で約6回)。1回の時間は、Python の synth を Linux と macOS で 10〜20 秒、Windows で 20〜40 秒、TypeScript の synth を 5〜10 秒と 10〜20 秒と仮定します。

| 仕事 | 準備の手順(setup-python、`.venv` と pip、npm ci) | テストの増え方 | 仕事の全体の見込み |
|---|---|---|---|
| test ubuntu | 30〜60 秒 | 1〜1.5 分(`internal/cdk` が約 1.5 分になり、`internal/cli` が 65 秒から約 1.5 分) | 約 4〜5 分 |
| test macOS | 30〜60 秒 | 1〜1.5 分 | 約 3〜5.5 分 |
| test Windows | 1〜2 分(Windows はファイルの作成が遅い。今の ts-minimal の `npm ci` が 15 秒) | 2〜3 分(`internal/cli` が 155 秒から約 4 分) | 約 7.5〜10.5 分 |
| race | 30〜60 秒 | 1.5〜2.5 分 | 約 5〜6.5 分 |
| coverage | 30〜60 秒 | 1〜1.5 分 | 約 4〜5 分 |
| `release.yml` の build | 30〜60 秒 | 1〜1.5 分 | 今の時間に 1.5〜2.5 分を足す |

- `ci-gate` が待つ最も長い仕事は、今と同じく test の Windows(stable)で、約 10 分の見込みです。仕事の上限 30 分にも、パッケージごとの `go test` の既定の上限 10 分(最も長い `internal/cli` の Windows で約 4 分)にも余裕があるので、`timeout-minutes` も `-timeout` も変えません。
- 見直しの目安: U3 の最初の Bolt のプルリクエストの CI で同じ数え方で測り、どれかの仕事が 20 分を超えるか、どれかのパッケージがどれかの OS で 6 分(既定の上限の 6 割)を超えたら、その Bolt のプルリクエストで、テストの分け方か `-timeout` を見直します。
- pip のキャッシュが当たらない実行(Dependabot が `requirements.txt` を変えたときの最初の実行など)では、ダウンロードの分として 10〜30 秒が加わります。

## 6. 秘密の値と権限

- 新しい secret も変数も足しません。今の `TAG_REFACTOR_TEST_REPO_TOKEN` と `TAG_REFACTOR_TEST_REPO` は、今のとおり `github-pr` の結合テストの手順の `env` だけに渡ります。Python の手順は `github-pr` に足さないので、秘密の値と同じ仕事に、PyPI から入れたパッケージが動く手順が増えることもありません。
- どのワークフローにも AWS の認証情報(`AWS_` で始まる変数、`aws-actions/configure-aws-credentials`)を足しません。CI から実際の AWS は呼びません。`ts-lookup` と `ts-profile` の synth は、U1 の遮断した環境で動くので AWS に届かず、needsAWS になります(NFR8.4)。`ts-lookup` に書く 12 桁は架空の値で、実際のアカウントの ID は書きません。
- `permissions` は、どのワークフローも最上位の `contents: read` のままで、仕事ごとの `permissions` も広げません。`release.yml` の `publish` の `contents: write` は今のままです。
- `actions/checkout` の `persist-credentials: false` は今のままです。Python の準備の手順は git の認証情報を持ちません。
- 供給網の手当て: アクションはハッシュで固定し、pip は `--only-binary=:all:`(パッケージを入れるときにビルドのコードを動かさない)、npm は `--ignore-scripts` にし、`pip check` で依存のずれを止めます(NFR6.19)。pip のパッケージのハッシュの確かめ(`--require-hashes`)は、要件に無いので使いません(残るリスク)。
- `setup-python` の pip のキャッシュは GitHub Actions のキャッシュに置かれ、鍵は OS と `requirements.txt` のハッシュです。秘密の値は入りません。

## 7. 配置とやり直し、失敗の見分け方

### 配置

U1 のままです。`main` への squash マージのたびに確認用のバイナリを作ります(公開しない)。公開は、人が `vX.Y.Z` のタグを push したときだけで、`v0.2.0` は `iac` の CDK(TypeScript と Python)の反映とプルリクエストの作成がそろったときに出します。タグを付ける前に、`main` で `release.yml` を `workflow_dispatch` で1回動かし(下書き。公開しない)、Python の準備と作業の木の確かめが通ることを確かめます。

### やり直しの手順

| 戻したいもの | 手順 | 注意 |
|---|---|---|
| U3 の Bolt 全体(コードと CI の変更) | その Bolt の squash のコミットを revert するプルリクエストを作り、`ci-gate` が通ってからマージする | 1つの Bolt は `main` の1つのコミットなので、Python のテストと Python の準備が一緒に戻る |
| CI の Python の手順だけ | しない。テストが残ったまま手順だけを戻すと、CI では道具が無いのでテストが失敗する(作りのとおり)。手順の不具合は、直すプルリクエストで前に進めて直す | 急いで止めたいときは、Bolt 全体を revert する |
| Dependabot の項目だけ | `.github/dependabot.yml` の2つの項目を消すプルリクエストを作る。開いたままの Dependabot のプルリクエストは手で閉じる | テストとワークフローを読むテストの Dependabot の確かめも、同じプルリクエストで直す |
| Dependabot の1回の更新 | そのプルリクエストを閉じるか、マージ済みなら revert する | 次の週にまた来るので、止めたい版は `ignore` を足す |
| 公開 | U1 のとおり。公開したリリースは消さず、直した版を次のタグで出す | タグを付ける前の下書きで止めるのが基本 |

### 失敗の見分け方

| 見えること | 考えられる原因 | すること |
|---|---|---|
| `Install the Python test CDK app` で `No matching distribution found`(`--only-binary` のため) | 固定した依存に wheel の無いものがある | security-design.md の10節のとおり、そのパッケージだけを例外にする理由を Code Generation の計画に書き、人に確かめる(NFR6.19) |
| `pip check` が依存のずれを出す | Dependabot の更新でそろわない組み合わせになった | 同じプルリクエストで、ずれた依存の版をそろえる(`groups` で1つにまとめても出たとき) |
| go test で `RequirePythonTools` の失敗(何が無いかを文に出す) | `PY_TEST_APP` の綴りの違い、手順の順番の誤り(go test のあとに準備がある)、`.venv` の場所の違い | 3節のワークフローを読むテストの結果と、文に出た欠けたものを見る |
| Dependabot のプルリクエストで `TestMinPythonMatchesTheTestApp` が失敗 | `aws-cdk-lib` の `Requires-Python` の下限が変わった | 同じプルリクエストで `minPython` を直すか、そのプルリクエストを閉じる |
| `py-minimal` の synth で、クラウドアセンブリの版が CLI より新しいという誤り | `py-minimal` の `aws-cdk`(CLI)の更新が `aws-cdk-lib` の更新より遅れた | `py-minimal` の npm の Dependabot のプルリクエストを先にマージする |
| `setup-python` が版を見つけられない、キャッシュの誤り | ランナーの画像やネットワークの一時的な問題 | 再実行する。続くなら GitHub Actions の状態を確かめる |
| `release.yml` の build で `the working tree was modified` | `py-minimal/.venv/` が無視されていない、またはテストが `py-minimal` をその場で synth して `__pycache__` などを書いた | `.gitignore` の行を確かめる。テストは `testutil.CopyCDKApp` で写した先で synth する(security-design.md の10節)ので、その場で synth しているテストを直す |
| test の Windows で `TestQuotedArgReachesCmd` が失敗 | その Go の版で、引用符つきの引数が `cdk.cmd` に届かない | security-design.md の4節で先に決めたとおり、`windowsQuotedApp` を false にして README に案内を足す |
| needsAWS のテストが遅い、または AWS に届いた形跡がある | 遮断した環境が崩れた(NFR8.4 の違反) | `Command.Env` に AWS の変数が無いことを確かめるテストの結果を見る。CI に AWS の認証情報は無いので、実際のアカウントには届かない |
