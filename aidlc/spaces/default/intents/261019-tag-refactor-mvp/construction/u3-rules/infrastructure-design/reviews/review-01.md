## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | high | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/infrastructure-design/cicd-pipeline.md > `ci.yml` の節の2つ目の箇条書き(`testdata/fuzz/** -text`) | 足すと書かれている `.gitattributes` の行 `testdata/fuzz/** -text` は、スラッシュを含むパターンなので、リポジトリのルート直下の `testdata/fuzz/` にしか当たらない。ファジングの種の置き場は `internal/rules/testdata/fuzz/FuzzParse/` なので、この行を足しても種は `* text=auto eol=lf` の対象のままになる。作業用のリポジトリで `git check-attr` を使って確かめたところ、`internal/rules/testdata/fuzz/FuzzParse/a` は `text: auto`、ルート直下の `testdata/fuzz/FuzzParse/a` だけが `text: unset` になった。設計が守ると言っている「UTF-16 や不正な UTF-8 の種を改行の変換から守る」は、この書き方では実現しない。 | パターンを `**/testdata/fuzz/** -text` のように、どの階層の `testdata/fuzz/` にも当たる形に直す。あわせて、実装後に `git check-attr text -- internal/rules/testdata/fuzz/FuzzParse/<種>` が `unset` になることを確かめる手順を、設計かチェックポイントの確認に加える(Go のファジングの種は引用符つきの文字列で書かれる文字ファイルなので、この行が本当に要るかどうかも、あわせて見直す)。 | New |
| R-02 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/infrastructure-design/cicd-pipeline.md > 検証のスクリプト(Q1) > 手順4(一時ファイルは終了の処理で消す)と「`scripts/verify.sh` の最後に」 | 今の `scripts/verify.sh` は、手順4で `trap cleanup EXIT` を使ってキューを消している。同じ `EXIT` に別の `trap` を足すと前のものが置き換わり、テスト用の SQS キューが消されずに残る。設計は「終了の処理で消す」としか書かず、既存の `cleanup` に組み込むのか別に持つのかを決めていない。 | 一時ファイルの削除を既存の `cleanup` 関数の中で行うと明記する(一時ファイルの名前は、手順の前に `mktemp` で決めて変数に持つ)。`trap` を新しく定義しない、とも書く。 | New |
| R-03 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/infrastructure-design/cicd-pipeline.md > 検証のスクリプト(Q1) > 「`scripts/verify.sh` の最後に、U3 の確認を足します」 | `scripts/verify.sh` の最後は、U2 のキューを作り、Resource Explorer に載るまで最大30分待つ手順(手順5)のあとにある。手順5が失敗する(ヘッダーにも、索引が埋まるのに数時間かかることがあると書いてある)と、`fail` で終了して U3 の確認まで届かない。U3 の確認(`rules suggest` から `scan --rules` まで)は、キューも Resource Explorer の索引も使わないのに、U2 の待ち時間に巻き込まれる。また、U3 の確認が `env` と `Env` を両方持つタグ付きリソース(U1 の確認用スタック)に頼ることは、検証コマンドの中でどこでも前提確認されていない。 | U3 の確認を差し込む位置を決める(U1 の確認のすぐあと、U2 のキューを作る前が候補)。または、U3 を別のスクリプトにして `verify.sh` から呼ぶ。あわせて、確認の前に、U1 の確認用スタックが `CREATE_COMPLETE` または `UPDATE_COMPLETE` であることを確かめ、無いときは何を直すかを示して止まる手順を設計に書く。 | New |
| R-04 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/infrastructure-design/cicd-pipeline.md > チェックポイントでの人の確認(`-fuzz` の5分) | 手元の Windows でのファジングは、新しく作られたテスト用の実行ファイルを動かす。既存の `verify.sh` と `verify-u1.sh` は、Application Control がそれをふさぐ場合に備えて再試行を入れている。ファジングの手順には、その場合の扱い(再試行するのか、確認を別の手順に替えるのか)が書かれていない。 | ふさがれたときは、同じコマンドをもう一度動かす、と一行加える。それでも動かないときの替え手段(例: WSL や別の端末)を、NFR8.13 の確認の記録に残す形にする。 | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| sensor-traceability | pass: true(gaps、orphans、invalid_entries はすべて空) | 上流の ID の対応づけは通っている。上の指摘は、記載された手順が実際のリポジトリで動くか、という内容に関するもの。 |

### Summary

CI のジョブを変えない方針と、AWS の権限・確認用スタックを変えない方針は、現在の `ci.yml` や `docs/iam/readonly-policy.json` と矛盾せず妥当である。ただし、`.gitattributes` の行が目的のディレクトリに当たらないこと(実証済み)、`verify.sh` の既存の `trap` を壊しかねないこと、U3 の確認が U2 の長い待ち時間の後ろに置かれることを直す必要がある。
