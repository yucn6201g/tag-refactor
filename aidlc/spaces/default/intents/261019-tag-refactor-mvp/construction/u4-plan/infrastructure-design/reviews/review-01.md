## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T06:45:45Z
**Iteration:** 1

### Prior findings

なし

### New findings

| Severity | Location | Finding | Required action |
|---|---|---|---|
| high | construction/u4-plan/infrastructure-design/cicd-pipeline.md > `ci.yml` の表の最終行(端末での幅の表の版と `unicode.Version` の一致) | この一致テストは `test` ジョブで動くが、そのジョブは `go: [stable, oldstable]` の2つの版で動く(`.github/workflows/ci.yml` の matrix)。一方、設計は「このツールを作る Go 1.27 では 17.0.0」(`nfr-design/logical-components.md`)としか言わず、`oldstable`(今は `go.mod` の `go 1.26.0`)の `unicode.Version` が 17.0.0 かを示していない。2つの版の `unicode.Version` が違う間は、どちらの表を作っても片方の脚が必ず落ちる。Go の新しい版が出て `stable` が変わるたびに、無関係なプルリクエストの CI が全部落ちる。表の版を書く場所も「先頭のコメント」で、テストが読める定数かどうかが決まっていない。 | 一致の条件を、CI の matrix の両方で成り立つ形に直す。例: 表に Go の定数として版を持たせ、テストは `unicode.Version` が表の版と同じとき、または表の版より古いときは通し、新しいときだけ落とす(W と F の範囲は版が上がると増えるだけで、古い表でも使える)。または、このテストを `stable` の脚だけで動かし、`oldstable` では飛ばす。どちらにするかと、`oldstable` の版を確かめた結果を設計に書く。 |
| medium | construction/u4-plan/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 結合テスト `TestIntegrationU4PlanFormatsAndSavedPlan` の手順1 | 手順は `--lang` を指定しない。NFR9.7 の決め方では、`LC_ALL`、`LC_MESSAGES`、`LANG` が空の Windows は OS の表示言語に落ちる。検証コマンドは Windows の Git Bash で動かす前提で、この環境は日本語の PowerShell 利用者の端末なので、出力が日本語になり、4つの形式から件数を取る正規表現(U2 は英語の `Summary:` などに合わせている)が合わなくなるおそれが大きい。 | 結合テストの `plan` の呼び出しに必ず `--lang en` を付ける(または環境変数を `t.Setenv` で固定する)と設計に書く。日本語の件数の一致は、必要なら別の呼び出しで足す。 |
| medium | construction/u4-plan/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 動かし方 | 「テストが飛ばされたら失敗にする(U2 と同じ)」とだけあり、通った数の確認がない。U2 は `--- SKIP` の検出に加え、`--- PASS` が5つであることを確かめている(`scripts/verify.sh` 235〜236行)。`-run '^TestIntegrationU4'` が1つも一致しないとき(名前を変えた、ファイルに `integration` のタグを付け忘れた)、`go test` は成功で終わるため、確認が何も動かないまま `verify: OK` になる。 | `--- PASS: TestIntegrationU4PlanFormatsAndSavedPlan` がちょうど1つあることを確かめる、と設計に書く。 |
| medium | construction/u4-plan/infrastructure-design/cicd-pipeline.md > `ci.yml` の表の最終行の次の箇条書き(端末での幅の表を作るスクリプト `internal/display/gen/`) | 作るスクリプトを `internal/` の下の Go のパッケージにする場合、CI の扱いが決まっていない。`coverage` ジョブは `-coverpkg=./internal/...` で測るため、実行しない `main` パッケージが文の数に入り、80%の下限(配線部分も外さないというチームの決まり)を押し下げる。`test`、`lint`、`go vet`、`modcheck`(`go mod tidy` で依存が増えない確認)も通る対象になる。 | スクリプトを `//go:build ignore` の単独ファイルにする、または計測と lint の対象にする前提で小さなテスト(表の生成関数の入出力)を置く、のどちらかを決めて書く。ネットワークを使う部分は CI のテストから外すことも書く。 |
| medium | construction/u4-plan/infrastructure-design/cicd-pipeline.md > チェックポイントでの人の確認 | 人の確認が日本語の表示、列の揃い、Markdown の貼り付け、ファジングの4つで、色(NFR11.7 の「色の判定」)がない。Windows の仮想端末の処理を標準出力にも広げる変更(`logical-components.md`)は、Windows の実機でしか確かめられず、CI のテストは判定関数を差し替えるだけである。色が実際に出ることと、`NO_COLOR` やリダイレクトで消えることを確かめる手順がないと、最もずれやすい Windows の端末の処理が人の目に触れない。 | 手元の PowerShell で、色が付くこと(表と差分)、`NO_COLOR=1` を付けた実行とファイルへのリダイレクトでは色の制御の文字が出ないことを確かめる手順を足す。 |
| low | construction/u4-plan/infrastructure-design/cicd-pipeline.md > チェックポイントでの人の確認(ファジング) | ファジングが失敗する入力を見つけたとき、Go は `internal/planner/testdata/fuzz/FuzzLoadPlan/` に入力を書く。その入力を残して種に加える手順がない。 | 見つかった入力は種としてコミットし、直してから再実行する、と書く。 |
| low | construction/u4-plan/infrastructure-design/cicd-pipeline.md > `ci.yml` の表の最終行の次の箇条書き(表を作るスクリプト) | スクリプトが取ってくる `EastAsianWidth.txt` の取得元(版を含む URL)と、取ってきたファイルの検証(ハッシュ)が決まっていない。作り直しのたびに同じ表ができるかを後から確かめられない。 | 版を含む固定の URL を使い、取得したファイルの SHA-256 を表の先頭のコメントに書く。 |
| low | construction/u4-plan/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 結合テストの手順1 | 結合テストは `plan` を実機に対して5回(4つの形式と `--out`)動かす。回ごとに IAM の読み取りが走るため時間がかかり、件数の一致は5回の間にアカウントが変わらないことに頼る。 | 時間の見込みを書く。件数の比較は、5回が同じ内容を読んだことを前提とし、ずれたときの扱い(やり直し)も書く。 |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `aidlc engine sensor-traceability`(infrastructure-design の traceability.json) | pass: gaps 0、orphans 0、invalid 0 | 上流の NFR 15件(NFR1.13〜1.15、3.8、8.15〜8.19、9.6、9.7、10.6〜10.8、11.7)は、NFR 要件と NFR 設計の一覧と一致している。 |
| `bash -n scripts/verify.sh` | 構文エラーなし | 現状のスクリプトの構文は正しい。差し込み位置(U3 の確認の末尾、`# 3.` の前)は現状のファイルに存在する。 |
| 現状のファイルとの突き合わせ | 次の点は一致 | `.gitattributes` の `* text=auto eol=lf` と `**/testdata/fuzz/** -text` は、期待のファイルとファジングの種を覆う。`ci.yml` の `race`(`-race`)は `race` ビルドタグを自動で付ける。`test` は3つの OS で、`coverage` は Linux で動く。`go vet -tags integration` が新しい結合テストの型検査を担う。`u2-rules.yaml`(`Environment` の別名 `env`・`Env`、`Owner` の既定値 `platform`)は、設計が期待する `renameKey` と `addRequired` の出どころになる。 |

### Summary

CI の流れ、`.gitattributes`、検証スクリプトの差し込み位置、トレーサビリティは、現状のファイルと整合しており、おおむね実装できる状態です。ただし、端末の幅の表と `unicode.Version` の一致テストは、`stable` と `oldstable` の matrix で両方成り立つことを設計が示しておらず、Go の更新のたびに CI が落ちる構造なので、直す必要があります(high)。結合テストの言語の固定、通った数の確認、幅の表の生成スクリプトの扱い、色の人の確認は、実装前に設計へ書き足してください(medium)。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
