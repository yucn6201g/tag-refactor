# 観測の設計: U1 最小の一本

要件は `construction/u1-walking-skeleton/nfr-requirements/observability-requirements.md`(NFR10.1〜NFR10.3、NFR1.5)と `security-requirements.md` の NFR1.4 です。この道具は利用者の端末で動く CLI なので、メトリクスの収集、ダッシュボード、アラート、分散トレース、相関 ID、SLI/SLO は持ちません。利用者が見るのは標準エラーの行と標準出力の URL です。

## 1. 進み具合の行(NFR10.1、NFR10.3)

`display.Printer` が、契約 C6 の `cdk.ProgressSink` と `pullrequest.ProgressSink` を実装します。行は標準エラーに1行ずつ出し、色を使うときも文字だけで意味が分かるようにします(今の表示と同じ)。

| 知らせ | 行の例(英語) | 行の例(日本語) |
|---|---|---|
| AppsWaiting | `CDK apps to synthesize: infra (waiting)` | `synth する CDK のアプリ: infra(待ち)` |
| AppStarted | `infra: synthesizing (TypeScript)` | `infra: synth しています(TypeScript)` |
| AppDone | `infra: done (42s)` | `infra: 済み(42 秒)` |
| AppFailed | `infra: failed (12s): AWS credentials needed` | `infra: 失敗(12 秒): AWS の認証情報が要る` |
| AppSkipped | `infra: skipped: not TypeScript` | `infra: 外しました: TypeScript でない` |
| Step(checked) | `Pull request: origin (github.com/o/r), base main, branch tag-refactor/iac-...` | `プルリクエスト: origin(github.com/o/r)、行き先 main、ブランチ tag-refactor/iac-...` |
| Step(started、done、failed) | `Pull request: push done` | `プルリクエスト: push 済み` |

URL は、成功したときに標準出力に1行だけで出します。

## 2. 言語(NFR10.2)

U1 で足す文は、今の文の一覧(`internal/cli/messages.go` の英語と日本語)に、同じキーで両方の言語を足します。今のキーの抜けを確かめるテストが、新しいキーも確かめます。

## 3. `--debug`(NFR1.4、NFR1.5)

`--debug` のときだけ、標準エラーに次の行を出します。どの行も security-design.md の伏せ字の関数を通します。

- 起動した外のプログラムの名前と引数(作業のフォルダーは `--dir` からの相対)
- 渡した環境の変数の名前の一覧(値は出さない)
- 段階ごとの経過時間(synth、analyze、recheck、確かめ、ブランチ、コミット、push、作成)
- 失敗したときの、synth と補助のプログラムの標準エラー(コードの抜粋の行を除いたもの)
