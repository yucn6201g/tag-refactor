# CI と配布の流れ: U2 プルリクエストの作成(残り)

tag-refactor は利用者の端末で動く CLI です。U2 では、本番のインフラも AWS のリソースも足しません。U1 の CI と配布の流れ(`construction/u1-walking-skeleton/infrastructure-design/cicd-pipeline.md`)をそのまま使い、CI の定義(`.github/workflows/ci.yml`、`release.yml`、`dependabot.yml`)も変えません。ここには、U2 のテストが今の CI のどこで動くかと、テストの側で守る前提を書きます。元にしたものは次のとおりです。

- U2 の非機能の設計: `construction/u2-pull-request/nfr-design/security-design.md`(4節、5節、7節、8節)と `logical-components.md`
- 機能の流れ: `construction/u2-pull-request/functional-design/functional-spec.md`
- 契約: `inception/contract-design/contract-summary.md`(X1、X2、C1)
- 部品の一覧: `inception/domain-design/components.md`

U2 の決めごとは、要件と設計の段階の回答(NQ2-Q1、ND2-Q1、ND2-Q2)で決まっています。そのため、このステージで新しく人に聞くことはありません。

## 1. U2 のテストが動く仕事

| U2 のテスト | 置くパッケージ | 動く仕事(今の `ci.yml`) | 前提 |
|---|---|---|---|
| `Scan` の単体テスト | `internal/redact` | test(3つの OS × Go の2つの版)、race、coverage | なし |
| 本文のファイルの単体テスト(0600、成功で消える、失敗で残る、名前の長さ、作れないとき) | `internal/pullrequest` | test、race、coverage | git(ランナーに入っている) |
| ブランチ名の単体テスト(先頭の `-`、`check-ref-format` と出力の比べ) | `internal/pullrequest` | test、race、coverage | git |
| 既定のブランチ名の単体テスト(差し替えた時計の UTC と末尾の `Z`。時間帯を UTC+9 にしても同じ) | `internal/pullrequest` | test、race、coverage | なし |
| Publish がタイトルと本文を変えない単体テスト | `internal/pullrequest` | test、race、coverage | 偽の git と gh |
| 続きの命令を sh で動かすテスト | `internal/pullrequest` | test、race、coverage の Linux と macOS | `sh`。Windows では動かさない |
| 続きの命令を PowerShell で動かすテスト | `internal/pullrequest` | test の Windows(Go の2つの版) | `powershell.exe`(5.1)と `pwsh`(7)。`runtime.GOOS` が windows のときだけ動かし、ほかの OS では飛ばす。Windows で見つからなければ、CI(`CI=true`)では失敗、手元では飛ばす |
| cli の受け入れテスト(本文の形で止める、知らせ、タイトルの伏せ字、中断のときに本文のファイルの場所と続きの命令が出る、本文の上限) | `internal/cli` | test、race、coverage | 偽の git と gh。使うシェルはテストの中で固定する(下) |
| 実際の GitHub の結合テストのサブテスト `options`(NFR9.2) | `internal/cli` | github-pr。`-run '^TestGitHubPullRequest$'` がサブテストも動かす | secret `TAG_REFACTOR_TEST_REPO_TOKEN` と変数 `TAG_REFACTOR_TEST_REPO`(U1 のまま)。無ければ飛ばす |
| 古い枝の掃除の `staleBranch` の単体テスト(`Z` と `-options` のある名前) | `internal/cli` | test、race、coverage | なし |

### シェルの固定

`ci.yml` は `defaults.run.shell: bash` なので、windows-latest でも `go test` は Git Bash から起動され、環境の変数 `MSYSTEM` が入っています。`DefaultShell` は `MSYSTEM` があると sh を選ぶ(security-design.md の4節)ので、テストは `DefaultShell` の結果に頼らず、次のようにシェルを固定します。

- 引用の関数と `ResumeCommands` の単体テストは、`Shell` を引数で渡します。sh 用と PowerShell 用を、どの OS でも両方確かめます(文字列の比べだけで、シェルは起動しない)。
- 実際のシェルで動かすテストも、`Shell` を引数で渡します。起動するシェルは上の表のとおり OS で決め、`MSYSTEM` には頼りません。
- cli の受け入れテストは、`t.Setenv("MSYSTEM", ...)` で、確かめたい側のシェルを選びます(空なら Windows で PowerShell、値があれば sh)。Windows 以外は、いつも sh です。
- `DefaultShell` そのものの単体テストは、`MSYSTEM` を空と `MINGW64` に切り替えて確かめます。

## 2. 段階と合否

U1 の表(整形と lint、テスト、race、カバレッジ 80%、脆弱性、結合テスト、ビルド)のままです。U2 で足すコードもカバレッジに入ります。止める条件も U1 と同じで、`ci-gate` が待つ仕事は変わりません。

`release.yml` の build の仕事も、ubuntu で `go test ./...` を動かします。PowerShell のテストは Windows 以外では飛ぶので、同じ前提で通ります。

## 3. 実行の時間

数字は推定で、測った値ではありません。U2 の Code Generation のあとの CI で実際の時間を確かめ、上限に近ければ、その Bolt のプルリクエストで見直します。

| どこ | 増える分の見込み | 効く上限 | 余裕 |
|---|---|---|---|
| `internal/pullrequest`(Windows の test) | シェルの起動の回数 = 値の数(およそ 15)× PowerShell の2つの版 = 約 30 回。1回あたり 1〜3 秒(Windows PowerShell 5.1 の起動は遅いことがある)とすると、30〜90 秒 | `go test` のパッケージごとの既定の 10 分。今の `internal/pullrequest` は 1 秒未満 | 十分。`-timeout` は足さない |
| `internal/pullrequest`(Linux と macOS の test) | sh の起動が約 15 回で、数秒 | 同じ | 十分 |
| github-pr | サブテストごとに別の clone と作業のフォルダーを使う(下)。増えるのは clone、`npm ci`、`iac --pr`、プルリクエストの作成と後片付けの1回分で、1〜3 分 | 仕事の上限 20 分。今の仕事は全体で 2 分ほど | 十分 |

### 結合テストのサブテストの分け方

`default` と `options` は、それぞれ別の一時フォルダーにテスト専用のリポジトリを clone し、`npm ci` もそれぞれで行います。片方のサブテストが切り替えたブランチや書き換えたファイルを、もう片方が引き継がないようにするためです。`tag-refactor` の二進のビルドは、テストの始めに1回だけ行い、両方で使います。

## 4. 配置とやり直し

U1 のままです。`main` への squash マージのたびに確認用のバイナリを作ります(公開しない)。公開は、人が `vX.Y.Z` のタグを push したときだけです。`v0.2.0` は B6 のあとに出します。やり直しは revert のプルリクエストで行います。

## 5. secret と AWS

- secret と変数は U1 のまま(`TAG_REFACTOR_TEST_REPO_TOKEN` と `TAG_REFACTOR_TEST_REPO`)で、渡すのは github-pr の結合テストの手順の `env` だけです。U2 のサブテストも同じ手順の中で動くので、渡す範囲は広がりません。
- U2 は AWS のリソースを作らず、検証用のスタック `tag-refactor-cdk-skeleton` も変えません。CI から実際の AWS は呼びません。
- サブテスト `options` の枝名は `tag-refactor/iac-<UTC の日時>Z-options` で、U1 の古い枝の掃除の対象に入ります。掃除の日時の読み方は、UTC に直します(security-design.md の7節。1節の `staleBranch` のテスト)。
