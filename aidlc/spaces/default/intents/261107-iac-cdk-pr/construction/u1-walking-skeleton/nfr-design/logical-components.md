# 論理の部品: U1 最小の一本

部品の一覧は `inception/domain-design/components.md`、契約は `inception/contract-design/contract-summary.md` にあります。ここでは、U1 の非機能の設計がどの部品のどこに効くかと、失敗がどこまで広がるかを書きます。すべての部品は1つのバイナリの中にあり、配置の単位は1つです。

| 部品(パッケージ) | 役割 | 効く非機能の設計 | 失敗の広がり |
|---|---|---|---|
| `internal/procexec`(新規) | 外のプログラムの上限つきの起動と、プロセスの木ごとの停止 | 上限、停止、`WaitDelay`(reliability-design.md の1節) | 呼び出した1つの段階だけ |
| `internal/cdk` の Synthesizer(新規) | アプリの発見、認証情報の遮断、synth、出力の読み取り | 遮断(security-design.md の2節)、一時的なフォルダーの片付け | そのアプリだけ(failed)。ほかのアプリと CloudFormation、Terraform の処理は続く |
| `internal/cdk` の Analyzer(新規)と埋め込んだ補助のプログラム | ソースの読み取りと読み直し。起動の前に `node --version` が 18 以上か、補助のプログラムが読み込んだ typescript が 4.0 以上 5.x までかを確かめ、外れればアプリを failed にする | 書き出しの守り(security-design.md の5節)、起動の回数(performance-design.md) | そのアプリだけ(failed)、または読めないファイルだけ(SkippedFiles) |
| `internal/iac`(今のものに足す) | 対象の決定、CDK の読み取り、書き換え、確かめ、書き出し | 今の書き出しの仕組み、メモリの予算 | ファイルの単位(手で直す一覧)。書き出しの失敗は実行全体(終了コード 1) |
| `internal/pullrequest`(新規) | 書き換えの前の確かめと、ブランチからプルリクエストの作成 | 出力の扱いと伏せ字(security-design.md の3節と4節)、段階ごとの上限 | 失敗した段階で止まる。済んだ段階は残る |
| `internal/redact`(新規) | 伏せ字の関数 | 閉じた一覧の規則(security-design.md の4節) | なし(純粋な関数) |
| `internal/display` | 進み具合の行と結果の表示 | 進み具合の行、言語(observability-design.md) | なし |
| `internal/cli` | オプション、順番(確かめ → 書き換え → 作成)、終了コード | 書き出しの失敗のときに作成しない(reliability-design.md の3節) | 実行全体 |
| CI(`.github/workflows/ci.yml`) | 3つの OS のテスト、Node.js と npm ci、結合テスト | トークンの範囲(security-design.md の6節)、仕事の上限 30 分 | その仕事。`ci-gate` が待つ |

共有するものは、利用者の端末の一時フォルダーと、`--dir` の下のファイルだけです。一時フォルダーは呼び出しごとに新しく作り、ほかの呼び出しと共有しません。

## cdk の実行ファイルの探し方(NFR6.6)

`CdkSynthesizer` は、次の順に `cdk` を探します。

1. アプリのフォルダーの `node_modules/.bin/cdk`(Windows では `node_modules/.bin/cdk.cmd`)
2. `PATH` の `cdk`(`exec.LookPath`。Windows では `PATHEXT` により `cdk.cmd` も見つかる)

見つからなければ、そのアプリを failed(toolMissing、Tool は cdk)にします。`npx` は使いません(無いときにネットワークから取りに行くため)。引数は `synth --quiet --no-notices --output <一時的なフォルダー>` で、作業のフォルダーはアプリのフォルダーです。

`CDK_DISABLE_VERSION_CHECK` と `--no-notices` と `CDK_DISABLE_CLI_TELEMETRY` で外への問い合わせが止まることは、U1 の Code Generation で CDK の CLI の文書(テスト用のアプリが固定した版のもの)と照らして確かめ、その結果を Code Generation の記録に書きます。

## CI の設計(NFR6.8、NFR6.10、NFR8.1、NFR8.2、NFR9.1)

今の `ci.yml` の仕事(test、race、coverage、lint、vuln、modcheck、build、ci-gate)に、次のとおり足します。アクションはすべてコミットのハッシュで固定し、版をコメントに書きます(今と同じ)。

| 仕事 | 足すこと |
|---|---|
| test(3つの OS × Go の2つの版) | `actions/setup-node`(Node.js 22 の版を固定、npm のキャッシュ、`cache-dependency-path` はテスト用のアプリの `package-lock.json`)と、テスト用のアプリのフォルダーごとの `npm ci --ignore-scripts`。`timeout-minutes` を 30 にする |
| race | test と同じ setup-node と npm ci。synth を動かすテストもここで動く |
| coverage | test と同じ setup-node と npm ci。synth を動かす部分もカバレッジに入る |
| prettier(新規、Linux) | setup-node のあと、`npm exec --yes prettier@<固定した版> -- --check internal/cdk/helper/` で、補助のプログラムの整形を確かめる |
| github-pr(新規、Linux) | setup-node、npm ci、`gh` は GitHub のランナーに入っているものを使う。結合テストの手順だけに secret を渡す(security-design.md の6節)。secret が空ならテストは飛ばされ、仕事は成功する |
| ci-gate | `needs` に prettier と github-pr を足す |

道具が無いと CI では失敗にする決まり(NFR8.1)は、Go のテストを動かす test、race、coverage、github-pr のすべてに Node.js と npm ci があることで守ります。lint、vuln、modcheck、build は Go のテストを動かさないので、Node.js は入れません。

Dependabot(`.github/dependabot.yml`)に、npm の更新を足します。

| ecosystem | directory | 間隔 | コミットの接頭辞 |
|---|---|---|---|
| npm | `/internal/cdk/testdata/apps/ts-minimal` | 毎週 | `chore(deps)` |

テスト用のアプリを増やしたとき(U4、U5)は、その単位で同じ行を足します。

## 契約との違い(U1 だけの一時的なもの)

U1 では、アプリが2つ以上見つかったとき、最初の1つ以外を `AppSkipped` で外します(scalability-design.md)。その理由 `notInU1` は契約 C6 と C7 の理由の一覧にありません。U1 だけの一時的な理由で、手で直す一覧には出さず(外したアプリのリソースは、テンプレートの探索で見つからなければ cdkAppFailed にはせず、今までどおり templateNotFound)、U3 で複数のアプリを扱うときに消します。
