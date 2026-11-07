## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-31T14:40:00Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 手順 > 4. テンプレートを確かめる | `aws cloudformation validate-template --template-body file://<書き換えたファイル>` の書き方では、Windows の Git Bash で失敗する。書き換えたファイルは `mktemp -d` の下にあり、そのパスは `/c/Users/.../Temp/tmp.XXXX/...` の形になる。Git Bash はオプション値の `/tmp/abc` は Windows のパスに直すが、`file:///tmp/abc/x.yaml` は直さずに AWS CLI に渡す(この環境で `cmd //c echo` に渡して確かめた)。AWS CLI は `/c/Users/...` を `C:cUsers...` と読むので、ファイルが見つからず、手順 4 でスクリプトが止まる。検証コマンドの動く環境(Windows の Git Bash)で、チェックポイントの検証が通らない | 渡し方を決めて書く。例: `file://$(cygpath -m "$file")`(`cygpath` がなければそのまま)にする、または `--template-body "$(cat "$file")"` で中身を渡す(u6-fixtures.yaml は約 10 KB で 51,200 バイトの上限に収まる)。失敗したときのメッセージも決める | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/infrastructure-design/cicd-pipeline.md > 後片付け と チェックポイントでの人の確認 | 人の確認用のフォルダーを `mktemp -d` で作ると、パスは `/c/Users/.../Temp/tag-refactor-u8-check-XXXXXX` の形になる。このパスをそのまま「PowerShell で動かすコマンド」に埋めて表示すると、PowerShell では存在しない場所になり、人は手順を動かせない。そのほか、(a) 手順 2 は人の確認用の写しを「手順 5」で作ると書くが、手順 5 にその作業がない、(b) 人の確認用のフォルダーを作る時点が決まっておらず、あとの手順で失敗すると、アカウント ID と ARN を含む `u8-plan.json` を入れたフォルダーが何の表示もなく残る、(c) 「表示されたフォルダーを消す」だけで、消すコマンドがない | 表示するパスを `cygpath -w` で Windows の形にして出す(`cygpath` がない環境ではそのまま)。人の確認用のフォルダーとファイルの写しは、すべての自動の確かめが通ったあとの最後の手順で作る。失敗した場合はフォルダーを作らない。消すコマンド(例: `Remove-Item -Recurse -Force <フォルダー>`)を表示に含め、フォルダーに account ID と ARN を含む変更案が入ることも書く。手順 2 の「(手順 5)」の参照を直す | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/infrastructure-design/cicd-pipeline.md > ci.yml の表(新しい依存の行)と traceability.json の NFR9.12 | NFR9.12 は、3つの OS と Go の2つの版(stable と oldstable)で `CGO_ENABLED=0` のビルドとテストが通ることを求める。しかし今の `.github/workflows/ci.yml` では、`CGO_ENABLED=0` を置いているのは `build`(stable だけ)と `artifacts` のジョブだけで、`test`(oldstable を含む)には `CGO_ENABLED` の設定がない。そのため oldstable を `CGO_ENABLED=0` でビルドするジョブも、`CGO_ENABLED=0` でテストするジョブもなく、設計の「設定の変更は要らない」と、traceability の「build と test(Go の2つの版、CGO_ENABLED=0)」は事実と合わない。`go.mod` の `go` の行が変わらないことも CI では見ておらず、Code Generation の1回の確かめだけに頼っている | `test` のジョブに `CGO_ENABLED: "0"` を足す(`race` は別のジョブなので影響しない)、または `build` の表に oldstable を足す。ci.yml の表と traceability の NFR9.12 の行を、実際に変えるジョブの名前で書き直す。`go` の行の確かめ方(例: Code Generation の記録に前後の値を残す)も書く | Resolved (reviewer) |
| R-04 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 手順 3 と チェックポイントでの人の確認(BU6R の扱い) | BU6R が手で直す一覧に出るかどうかを ARN の有無だけで確かめるため、理由が `keyNotInSource` でなく `templateNotFound` でも通ってしまう。実際に、`test/sandbox/u6-add-rules.yaml` ではバケットの resourceType を `s3` と書いており、`logical-components.md` の種類の表の `s3:bucket` と食い違う(表の側で Code Generation に合わせると書いてあるが、検証の前提になっていない)。スタックのタグの場面を確かめる、という手順 3 のねらいが、別の理由で満たされたように見える | BU6R の確かめで理由(`keyNotInSource`、英語の出力ではその文言)まで確かめる。種類の表の resourceType を scan の書き方に合わせることを、検証の前提として cicd-pipeline.md に書く | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 手順 1 と手順 5 | (a) 変更案の確かめは「終了コード 2 になりうるので、ファイルができていれば進む」とだけ書いてあり、受け付ける終了コードも、前回のファイルを先に消すことも決まっていない(U6 は 0 か 2 だけを受け付ける)。(b) VU6 の ARN の求め方がない(スタックの出力にあるのは GU6 と BU6R だけで、VU6 は物理 ID と account・region から組み立てる必要がある)。(c) `iac` は影響の指摘を使わない(BR2.1)のに、`includes` の確かめで影響を P2 だけに限るため、U5 や U1 の無関係な指摘で検証が止まりうる。(d) 「反映済みの数が 4」は、言語ごとに変わる標準出力から取る方法が書かれていない | (a) 受け付ける終了コードを 0 と 2 にし、実行前に `u8-plan.json` を消す。(b) VU6 の ARN を `describe-stack-resource` で組み立てる手順を書く。(c) `includes` では影響の確かめを外すか、外さない理由を書く。(d) `--lang en` で読む文言の形を、Code Generation で決める項目として書く | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 手順 2 と手順 3(git の一時のリポジトリ) | 「利用者のグローバルの設定に頼らない」としながら、与えているのは名前とメールだけ。この環境の system の git は `core.autocrlf=true` で、グローバルに `commit.gpgsign`、`core.hooksPath`、`init.templateDir` などがあると、`commit` が署名の入力待ちやフックの実行で止まる。また、元のファイルが変わっていないことを `git diff --quiet -- test/sandbox` で確かめるのは作業中のリポジトリが相手で、`test/sandbox/` に未コミットの変更があるだけで失敗し、未追跡の `u8-rules.yaml` は見ない | `commit` に `-c commit.gpgsign=false -c core.hooksPath=<空のフォルダー>` を付け、フックを動かさない。`GIT_CONFIG_GLOBAL` を空にするかどうかを決めて書く。元のファイルの確かめは、写す前と書いたあとの `test/sandbox/` のチェックサムの比較に変える | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/infrastructure-design/cicd-pipeline.md > ci.yml の表(大きなテストの行)と 検証のスクリプト > 手順 5 | NFR5.25 の「手元で、実際の git を使って1回測り、記録する」は、10,000 リソース・100 MB の規模の測定を指す。ところが手順 5 の時間は、サンドボックスの十数個のファイルで 2 ファイルを書く時間なので、その記録にならない。traceability.json はこれを NFR5.25 の対応先にしている。また、60 秒の上限を持つ大きなテストは、計測で遅くなる `coverage` のジョブ(`-coverpkg`)でも動くが、そこでの扱い(上限をゆるめる、または飛ばす)が書かれていない | 実際の git を使う大きなテストを手元で動かす方法(環境変数での切り替えなど)と、その結果の記録先(`code-summary.md`)を決め、traceability の NFR5.25 の対応先を直す。手順 5 の時間は「git を呼ぶ時間の目安」と書く。`coverage` のジョブで時間の上限をどう扱うかを書く | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/infrastructure-design/traceability.json > coverage の NFR3.17 と NFR3.21 | NFR3.17 の対応先に「6行だけ変わる」とあるが、cicd-pipeline.md は4行(VU6 と GU6 の2行ずつ)に訂正済みで、古い数のまま残っている。NFR3.21 は、`cfn-lint` を書き換えたテンプレートで通すことを求めるが、この環境には `cfn-lint` がなく(PATH にない)、CI の lint は元のテンプレートだけを見るため、書き換えたテンプレートへの `cfn-lint` はどこでも動かないまま `OK` になっている | NFR3.17 の「6行」を「4行」に直す。NFR3.21 は、`cfn-lint` を飛ばす場合に `validate-template` だけになることを注記して、一部の確かめとして記録するか、`pipx run` などで `cfn-lint` を動かす手順を足す | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/infrastructure-design/cicd-pipeline.md > ci.yml > 2つ目の変更(lint のジョブの書き換えたテンプレートの cfn-lint) | 変更案 `sandbox-plan.json` で `iac --write` した写しを cfn-lint にかける、と書いてあるが、(a) 手で直すものがあれば `iac` は終了コード 2 を返すので、ステップが 2 を失敗とみなさない書き方が要る、(b) 変更案がテンプレートの論理 ID と合わなくなっても、書き換えが0件のまま元のテンプレートの cfn-lint が通るだけになり、書き換えたテンプレートを確かめたことにならない | CI のステップで終了コード 0 と 2 だけを受け付けること、`u6-fixtures.yaml` の差分が4行であること(または空でないこと)を確かめてから cfn-lint を動かすことを書く | New |

> R-01 Reviewer note: 手順 4 が `cygpath -m` で `file://C:/...` の形にして渡し、51,200 バイトの上限と失敗時の表示も書いた。Git Bash が `file://` の中を直さないことへの対処として足りる

> R-02 Reviewer note: 人の確認用のフォルダーは手順 6 で、手順 1〜5 が通ったあとにだけ作る。`cygpath -w` の表示、`Remove-Item` の消すコマンド、変更案にアカウント ID と ARN が入ることの表示がある。手順の参照も直っている

> R-03 Reviewer note: `test` のジョブに `CGO_ENABLED: "0"` を足す変更が表と traceability にあり、`go` の行の前後を `code-summary.md` に記録する。`race` は別のジョブのまま

> R-04 Reviewer note: 「前提」の節に、resourceType を scan の書き方(S3 は `s3`)に合わせることと、keyNotInSource の理由の文言を検証が読むことが入った。手順 3 は理由まで確かめる

> R-05 Reviewer note: 前回のファイルを消す、終了コード 0 か 2、VU6 の ARN を `describe-stack-resource` で作る、`includes` は影響を確かめない、反映済みの数の行の形を決める、の5点が入った

> R-06 Reviewer note: `GIT_CONFIG_GLOBAL`、`GIT_CONFIG_NOSYSTEM`、空のテンプレート、`core.autocrlf=false`、`commit.gpgsign=false`、空の `core.hooksPath` を指定した。元のファイルは SHA-256 の一覧で比べる

> R-07 Reviewer note: `TAG_REFACTOR_LARGE_REAL_GIT=1` の手元の大きなテストと、`coverage` のジョブでの時間の上限3倍(`testing.CoverMode()`)が入り、手順 5 の時間は目安と書いた。traceability の NFR5.25 の対応先も直っている

> R-08 Reviewer note: NFR3.17 は4行に直り、NFR3.21 は `lint` のジョブでの書き換えたテンプレートの cfn-lint が対応先に入った

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|

ステージの定義に検証の道具がなく、実行していない。AWS は呼んでいない。

### Summary

前回の Major 3件と Minor 5件はすべて直っており、残りは CI の cfn-lint ステップの確かめが空振りしうるという Minor 1件だけで、実装者が推測なしに作れる状態です。
