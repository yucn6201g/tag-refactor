# コードの生成の計画: U7 取り消し

## Summary

- Builds: `rollback`(記録の読み込みと中身の確かめ、`touchedKeys` だけの判定、確認、`--force`・`--yes`・`--record-from`・`--force-unlock`、単位ごとの読み直しと書き戻し、読み戻しと読み直し、結果の記録、2段の止め方)、apply と rollback が共有する書き込みの部品、`journal` の rollback の記録(`OpenForRollback`、`RollbackRun`)、`verify.sh` の U7 の確かめ
- Touches: `internal/apply/`、`internal/journal/`、`internal/awsaccess/`、`internal/cli/`、`internal/display/`、`internal/testutil/`、`internal/rules/rules_test.go`、`test/sandbox/`(`u7-*-rules.yaml`)、`scripts/verify.sh`、`scripts/u6check.go`、`README.md`
- Tests: 受け入れ条件のテスト(AC5.1.1〜AC5.1.6、AC6.1.1 の `rollback`。先に書く)、部品ごとの単体テスト(apply の rollback、journal、awsaccess、display、cli で各5〜8件以上)、差分0の各場面(NFR4.2)、大きな偽の AWS のテスト(2つの分布、回数、同時の数、30秒)、U6 のテストの回帰。カバレッジ80%以上

## 前提と進め方

- 設計の出典: `construction/u7-rollback/` の機能設計(BR1.1〜BR7.4、`functional-spec.md` の流れ W1、判定の例、状態 S1・S2、記録を変える場合の表)、NFR 要件(NFR3.13〜NFR11.9)、NFR の設計(`security-design.md` §1〜§9、`logical-components.md`)、インフラの設計(`cicd-pipeline.md`)。契約 `inception/contract-design/contract-summary.md`(C4、C5、C10 の `OpenForRollback` と3つの呼び出し、C11、C12)。ストーリー US5.1(AC5.1.1〜AC5.1.6)と US6.1 の AC6.1.1 の `rollback` の分。
- **ブランチ:** U6 のプルリクエスト(`bolt/u6-apply`)が `main` に squash マージされてから、`main` から `bolt/u7-rollback` を作って始める(人の判断。トランクベース開発)。マージまでは Step 1 にも進まない。AI-DLC の記録のコミットは、人が頼んだときに行う。U7 のマージも人がプルリクエストで squash マージする。
- **既存のコードに手を入れる:** U6 の `internal/apply`(リージョンの役、止める役、まとまりの書き方、2回目の Ctrl+C の放棄、印を取る流れ)を、apply の振る舞いを変えずに共有の形に直す(NFR の設計 Q1)。U6 の `internal/journal` の `Run` から5秒ごとの保存の役を切り出し、`LockHandle` に目的を足す(Q2)。同じ役割のファイルを別に作らない。新しいパッケージは作らない。
- **U6 のテストの扱い:** 共有に直したあとも、U6 のテストの期待は変えない。テストの補助関数が動いた項目を直接見ている場合だけ、補助関数を直す(直したものは `code-summary.md` に書く)。
- **順番(Testing Contract の custom):** 受け入れ条件のテストを先に書いてコミットし、次に実装、実装の後に細かい単体テストを書き、最後にすべてのテストを実行する。
- **AWS への変更は、この計画では行わない:** U7 は検証用アカウントのスタックを変えない(インフラの設計)。`verify.sh` の U7 の確かめは、チェックポイントで人が許可した検証コマンドとして動かす。
- **確かめながら決めること(結果は `code-summary.md` に書く):**
  - 共有に直した部品の境界(apply の `conductor` から何を移したか)と、U6 のテストの補助関数を直したかどうか。
  - タグが0個のリソースの切り替え(偽の AWS)で、U6 の apply の読み戻しの扱いが変わらないこと。

## 手順

- [x] **Step 1: テストの準備** — `unit-test-instructions.md` のコマンドが動くことを確かめる。HTTP の層の偽の AWS(`internal/testutil` のタグのアカウント)に、(1) タグが0個のリソースを `GetResources` の答えに出さない切り替え(既定は今のとおり出す)、(2) 決めた ARN の読み取りに決めた回数だけ古いタグを返す切り替え(読み戻しの遅れ)を足す。U6 のテストが既定のままで通ることを確かめる。(対応: Testing Contract の runner_step、NFR4.2 (c)、NFR5.20、NFR5.24)
- [x] **Step 2: 受け入れ条件のテスト(先に書く。`internal/cli/acceptance_u7_test.go`)** — 偽の AWS と一時のホームで `apply` のあと `rollback` を動かし、AC5.1.1(差分0。`aws:` を含むすべてのタグが apply の前と同じ、結果が記録の `rollbacks` に入る)、AC5.1.2(`yes` 以外で何も戻さず記録も変えない)、AC5.1.3(後から変えた項目を飛ばし一覧と `--force` のコマンドを示す、終了コード 2)、AC5.1.4(`--force` で戻す。`touchedKeys` 以外のキーは触らない)、AC5.1.5(戻した・飛ばした・失敗した件数)、AC5.1.6(途中で止まった apply の記録で、`--force-unlock` のあと書いた分だけ戻る)、AC6.1.1(`rollback --help`)と、apply が示す取り消しのコマンド(`--record-from`・`--profile` を含む)でそのまま rollback できること(BR7.4)を確かめる。この時点では失敗することを確かめ、これだけで1つのコミットにする。(対応: US5.1、AC6.1.1、BR7.4)
- [x] **Step 3: 共有の部品(`internal/apply`)** — `logical-components.md` のとおり、リージョンの役(止めの誤りの操作名を引数に)、止める役、1つのまとまりの書き方(書く ARN、付けるタグ、消すキー、消すのが先かを受け取り、6つの書き込みの結果を返す)、2回目の Ctrl+C の放棄の仕組み(書き込み開始の印、放棄の印と以後の結果の記録を何もしないこと、取り消されない文脈での読み戻しと最後の保存、`abandonWait`、結果の受け手の2つの口)、印を取る流れ(目的を引数に、`--force-unlock`、Conflict の印の中身)を、apply の `conductor`・`Target`・`journal.Run` に頼らない小さな型に移す。apply の `planUnits` と `decide` は変えない。U6 の apply のテストがすべて通ることを確かめ、これだけで1つのコミットにする(振る舞いを変えない直し)。(対応: NFR5.18、NFR5.23、NFR7.2、logical-components.md、security-design.md §8)
- [x] **Step 4: 記録(`internal/journal`)** — `LockHandle` に目的を足す。`Run` から5秒ごとの保存の役を小さな型に切り出す(U6 の NFR6.2 のテストの期待は変えない)。`OpenForRollback(ctx, LockHandle, runId)`(目的が `rollback` でなければ Internal、場所は LockHandle から、1回取得したバイト列を型付きの `RunRecord` と生の JSON の両方に解析、版の確かめ)と `RollbackRun`(`Record()`、`BeginRollback`・`RecordRollbackResults`・`CompleteRollback`・`Close`、最上位と `rollbacks` を生の JSON で保って自分の RollbackRun だけを置き換える、`BeginRollback` の成功で5秒ごとの保存を始める、閉じたあとの記録は何もしない、壊れない保存は U6 の部品、S3 は `Put`)。ログファイルは U6 の `WriteLog` を rollbackId の名前で使う。(対応: BR1.2、BR5.4、BR6.1〜BR6.4、NFR6.3、NFR6.4、NFR7.2、NFR8.26、security-design.md §4〜§6、契約 C10)
- [x] **Step 5: AWS の口(`internal/awsaccess`)** — ARN を6つに分ける関数(`internal/inventory` の `arnRegion` と同じ読み方。`inventory` は変えない)。(対応: NFR3.15、security-design.md §2)
- [x] **Step 6: rollback の流れ(`internal/apply`)** — 機能設計 W1 の手順 3〜13: rollbackId、印、`OpenForRollback`、記録の中身の確かめ(security-design.md §2 の (a)〜(f)、ARN の形と重複、リージョン名の形、値の同じ `touchedKeys`)、判定する項目の選び方(BR1.4)、タグが0個のリソースの関数(判定と読み戻しで共通。security-design.md §3)、1回目(判定だけ。途中の停止で記録に触れず印を消す)、要約(取り消し済みの警告を含む)、書き戻すものがないとき(BR4.5)、確認の呼び出し(確認そのものは cli から渡す関数)、`BeginRollback` と1回目の結果の記録、2回目(ARN の順に100個の単位、読み直しと判定し直し、単位の中の同じ鍵の20個のまとまり、読み直しの失敗は単位の項目を readFailed、共有の書き方、読み戻しと1・2・4秒の読み直し(時計を差し替えられる、中断で resultUnknown)、security-design.md §8 の結果の表)、`CompleteRollback`・`Close`、結果(RollbackOutcome)と終了コード(BR7.2)、進み具合(契約 C11)。公開の口は `Rollbacker`。(対応: BR1.3〜BR7.2、NFR3.14、NFR3.15、NFR4.4、NFR5.19、NFR5.20、NFR5.24、logical-components.md)
- [x] **Step 7: 表示(`internal/display`)** — 確認画面(BR4.1。途中で止まった apply の記録と取り消し済みの記録の `!` と文字、戻す・後から変えられた・すでに戻っている・なくなった・50を超える・読めなかった件数、`--force` のときの「上書きして戻す」)、`--yes` のときの要約、結果(件数、確かめられなかった件数、飛ばしと失敗の一覧と理由、ログファイル、`--force` のコマンドともう一度 rollback するコマンド。`--record-from`・`--profile` を含める)、「取り消すものはない」、ヘルプ(BR7.3)。英語と日本語の文言(U4 の文言の一覧のテストを通す)。U4 の言語と色の部品を使う。(対応: BR4.1、BR7.1、BR7.3、NFR11.9、AC6.1.2、AC6.1.3)
- [x] **Step 8: コマンドの入口(`internal/cli`)** — `rollback` のサブコマンド(U1 の「まだ使えない」を置き換える): オプション(`--run` 必須と runId の形、`--force`、`--record-from`(形は `--record-to` と同じ確かめ)、`--yes`、`--force-unlock`、`--profile`、`--lang`)、アカウントの確かめ、端末の確かめ(確認が要ると分かってから。BR4.3)、確認の入力(Ctrl+C と EOF は `yes` 以外)、合図の役(apply と同じ2段)、書き込みの口を apply と rollback にだけ渡す(NFR3.13)、ヘルプ。U1 の「まだ使えない」のテストは `iac` だけにする。(対応: BR1.1、BR4.2〜BR4.4、BR5.5、BR7.3、NFR3.13)
- [x] **Step 9: 受け入れ条件のテストを通す** — Step 2 のテストと、U1〜U6 の受け入れ条件のテストがすべて通ることを確かめる。期待が変わるものは理由を `code-summary.md` に書く。
- [x] **Step 10: 細かい単体テスト(実装の後)** — `logical-components.md` のテストの設計の表のすべての行。とくに: 差分0の各場面(変更の4種類、途中で止まった apply の3つの状態、0個の向きの両方を偽の AWS の切り替えの両方で、rollback の途中停止のあとの再実行(2つの呼び出しの間、読み直しの途中、保存の失敗、2回目の Ctrl+C)、終わった rollback の再実行、`rollbacks` が2件以上で `entries` と先の RollbackRun が変わらない)、判定の例(機能設計の表の5行)、記録の中身の確かめ (a)〜(f) の表(S3 のバケットを含む正しい記録では止まらない)、結果の表(security-design.md §8 の各行)、読み直しの前の他人の変更、1回目の途中の停止、放棄、読み戻しの読み直し(1・2・4秒、3回とも違う、中断)、知らない項目(数値の桁、Unicode、入れ子)の保持、S3 の記録での開閉と保存、`RecordRollbackResults` と5秒ごとの保存の同時実行(`race`)、印の競争(apply の間の rollback、2つの rollback)、認証情報とエラー文が出ない、表示の期待の出力(英語・日本語・`NO_COLOR`)、rollback が呼ぶ操作が権限の例に含まれる。(対応: 機能設計の各規則、NFR3.13〜NFR11.9)
- [x] **Step 11: 大きな偽の AWS のテスト** — 測る条件の2つの分布(1万リソース、17リージョン、偏ったリージョン2,000件)で apply した記録を戻し、呼び出しの回数が NFR5.20 の式の上限以下(M = 0 と、読み戻しを遅らせた M > 0)、1回の書き込みが20以下・組み合わせが混ざらない・単位をまたがない・1回の読み取りが100以下・同時のリージョンが4以下・リージョンの中は重ならない(NFR5.19)、遅れなしでそれぞれ30秒以内(NFR5.22)。`race` のビルドタグのときは件数を10分の1にし、上限を10倍にする。U7 の大きなテストは CI の各 OS で合わせて2分以内。(対応: NFR5.19、NFR5.20、NFR5.22、NFR10.11)
- [x] **Step 12: 検証のスクリプトとルールファイル** — `test/sandbox/u7-src-rules.yaml`、`u7-rename-rules.yaml`、`u7-only-remove-rules.yaml`、`u7-zero-add-rules.yaml`、`u7-cleanup-remove-rules.yaml`。`internal/rules/rules_test.go` に `TestSandboxU7RuleFilesAreValid`(読めること、rename が `renameKey` と `changeValue` になること、zero-add に `region` の条件があること)。`scripts/u6check.go` を `cicd-pipeline.md` の表のとおりに広げる(`plan` のキーと種類と必須・任意の ARN、`rollback`、`entries`、`same`)。`scripts/verify.sh` に、ツールのビルドのすぐあとの `go vet scripts/u6check.go` と、U6 の確かめのあとに `cicd-pipeline.md` の手順 1〜6 を足す(apply と rollback のオプションを分ける、一時的なキュー `u7_queue_arn`、旗 `u7_added`、`cleanup` の先頭での後片付けと残りの確かめの拡張、最後に人の確認用のコマンドを表示)。先頭のコメントの手順の説明を直す。(対応: NFR3.13、NFR4.3、NFR5.21、cicd-pipeline.md)
- [x] **Step 13: 全体の確認とドキュメント** — `gofmt -l`、`go vet ./...`、`go vet -tags integration ./...`、`golangci-lint run`(`integration` のタグありとなし)、`go test ./...`(Go 1.27 と 1.26)、カバレッジ80%以上(`-coverpkg`)、`govulncheck ./...`。下限を下げて通すことはしない。`README.md` に rollback の節(使い方、`--force` と後から変えられたリソース、`--record-from` と apply の `--record-to`、`--yes`、残った印と `--force-unlock`、結果と終了コード、限界: 記録の置き場所は自分だけが書ける場所にすること(同じアカウントの中の改ざんは防げない)、S3 の前置きを apply と同じにすること、`--force-unlock` での同時実行で結果が失われうること、読み直しから書き込みまでの窓、タグが0個のリソースの扱い)と、権限の節に「apply と rollback の両方に使う」を足す。
- [x] **Step 14: プルリクエストの準備** — `bolt/u7-rollback` のコミットを見直す(push とプルリクエストは人の許可を得てから)。

## ストーリーとの対応

| ストーリー | 手順 |
|---|---|
| US5.1(書き換えを元に戻す) | Step 2〜Step 4、Step 6〜Step 11 |
| US6.1 の AC6.1.1(`rollback` のヘルプ) | Step 2、Step 7、Step 8 |
| (NFR の設計: 共有の部品、記録の読み込みと書き戻し、記録の中身の確かめ、0個のタグ、読み戻しの読み直し) | Step 3〜Step 6、Step 10、Step 11 |
| (インフラの設計: ルールファイル、確かめの補助、検証のスクリプト) | Step 12 |

## Testing Contract

```json
{
  "version": 1,
  "methodology": "custom",
  "source": "team",
  "ordering": "まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。",
  "scope": "mvp",
  "test_strategy": "standard",
  "project_type": "greenfield",
  "applicable_notes": [
    {
      "layer": "org",
      "text": "We treat tests as a first-class deliverable in every Bolt. The specific\nmethodology (TDD, BDD, ATDD, or classic test-after) is affirmed at\npractices-discovery and recorded in `team.md` under this heading with explicit\n`Methodology` and `Ordering` fields; Code Generation resolves those fields\nindependently from coverage, tooling, and scope notes.\n\nWhen no posture has been affirmed, our default per scope is:\n- **Methodology**: test-after\n- **Ordering**: implement each applicable testable layer, then write and run\n  that layer's tests.\n- `mvp`, `enterprise`, `feature`, `infra`, `classic` add an 80% line-coverage\n  floor and CI execution before merge.\n- `bugfix`, `security-patch` add a targeted regression for the specific\n  bug/vulnerability and require the existing suite to remain green.\n- `express` uses the Minimal strategy: requirement-driven unit tests (one per\n  requirement, with a happy-path floor per component); existing tests remain\n  green.\n- `poc`, `refactor`, `workshop` add no extra new-test floor and require the\n  existing suite to remain green.\n\nThe active `Test Strategy` still applies in every scope and determines test\nvolume/types. Scope floors are additive; they never reduce or replace the\nselected strategy.\n\nBuild and Test verifies defined coverage floors and affirmed quality targets;\nthey may not be weakened to make a step pass.\n\nAffirm a stricter posture in `team.md` if the team commits to one."
    },
    {
      "layer": "team",
      "text": "私たちはテストを、すべての Bolt で作る成果物の一部として扱います。\n\n- **Methodology**: custom\n- **Methodology evidence**: Q6 の回答は C(受け入れ条件(Given/When/Then)を先にテストにし、細かい単体テストは実装のあとに書く)です。テストを先に書く部分と、実装のあとに書く部分が混ざっているため、`tdd`、`bdd`、`atdd`、`test-after` のどれにも当てはまらず、`custom` とします。\n- **Ordering**: まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。\n- カバレッジの下限は80%です。Go の標準の測り方(`go test -cover` の文(statement)単位)で測ります。コマンドの入口などの配線部分も、計測の対象から外しません。\n- マージの前に、プルリクエストの CI でテストを実行します。\n- AWS を呼ぶ部分の単体テストでは、AWS の呼び出しを差し替え、AWS の認証情報なしで CI で実行します。実際のアカウントを使う結合テストは、手元だけで実行します。CI から実際の AWS アカウントは呼びません。\n- CI のテストは、Linux、macOS、Windows の3つの OS で実行します。\n- テストの量と種類は、ワークフローのテスト戦略(Test Strategy)で決まります。上の下限は、それに加わる条件です。下限を下げて Build and Test を通すことはしません。"
    }
  ],
  "obligations": {
    "strategy": "standard",
    "strategy_volume": [
      "Five to eight tests per component.",
      "Unit tests plus integration tests for key boundaries.",
      "Add E2E, performance, or security tests when requirements demand them."
    ],
    "scope_floor": [
      "Meet an 80% line-coverage floor.",
      "Run the selected tests in CI before merge."
    ],
    "combination_rule": "Apply every selected-strategy obligation and every scope-floor obligation; neither replaces the other, and a targeted scope regression may add the narrowest necessary test type beyond the strategy default."
  },
  "plan_profile": {
    "methodology": "custom",
    "runner_step": "Bootstrap the minimal test runner/configuration and record the exact unit-scoped command.",
    "runner_ready_before_first_test": true,
    "testable_layers": [
      "Data model / database behavior",
      "Repository / data access",
      "Business logic",
      "API / endpoint",
      "Frontend behavior"
    ],
    "steps": [
      "Project structure and production configuration skeleton.",
      "Bootstrap the minimal test runner/configuration and record the exact unit-scoped command.",
      "Custom ordering - まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。",
      "Implementation and tests - preserve that exact ordering; do not convert it to layer-local TDD.",
      "Environment/build configuration.",
      "Documentation and traceability."
    ]
  },
  "input_sha256": "sha256:718a6e7f6adfab859e079f3d60a6687e52d7f72eeadae5429a96c212c2e81b35",
  "contract_sha256": "sha256:db7ed43eac0b272a1d28a3d6455a9d1ea63b45af7f36d138842ba8e9b43fa3fc"
}
```
