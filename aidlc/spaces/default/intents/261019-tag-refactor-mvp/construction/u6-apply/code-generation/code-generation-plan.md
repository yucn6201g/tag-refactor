# コードの生成の計画: U6 書き換えと記録

## Summary

- Builds: `apply`(確認、`--yes`・`--allow-impact`、変化の確かめ、まとめた書き込み、失敗の分け方、2段の止め方)、書き換えの記録と印(`internal/journal`。手元と S3)、書き込みの口(TagWriter、ObjectStore)、書き換え用の権限の例、U6 の確認用のスタックと `verify.sh` の U6 の確かめ
- Touches: `internal/apply/`(新規)、`internal/journal/`(新規)、`internal/awsaccess/`、`internal/cli/`、`internal/display/`、`internal/testutil/`、`docs/iam/apply-policy.json`、`test/sandbox/`(`u6-fixtures.yaml`、`u6-*-rules.yaml`)、`scripts/verify.sh`、`README.md`
- Tests: 受け入れ条件のテスト15件(先に書く)、部品ごとの単体テスト(journal、apply、awsaccess、display、cli で各5〜8件以上、約60件)、大きな偽の AWS のテスト(2つの分布、回数、同時の数、30秒)、3つの OS の保存と印のテスト。カバレッジ80%以上

## 前提と進め方

- 設計の出典: `construction/u6-apply/` の機能設計(BR1.1〜BR6.4、`functional-spec.md` の流れ W1・W2、状態 S1・S2、記録の項目の値)、NFR 要件(NFR3.10〜NFR11.8)、NFR の設計(`security-design.md`、`logical-components.md`)、インフラの設計(`cicd-pipeline.md`)。契約 `inception/contract-design/contract-summary.md`(C2、C4、C5、C6、C10、C11、C12。U6 で足した項目を含む)。ストーリー US4.1(AC4.1.1〜AC4.1.7)、US4.2(AC4.2.1〜AC4.2.7)、US6.1 の AC6.1.1 の `apply` の分。
- **ブランチ:** `bolt/u6-apply`(`main` の `1bc15f9` から作成済み)。AI-DLC の記録のコミットは、人が頼んだときに行う。マージは人がプルリクエストで squash マージする。
- **既存のコードに手を入れる:** U1 の `TagWriter` の口(`internal/awsaccess`)、`internal/cli` の `apply` の「まだ使えない」コマンド、U4 の変更案の読み込みと変更後のタグを求める部品(U4 の規則 1.9、4.4)を使う。同じ役割のファイルを別に作らない。新しいパッケージは `internal/journal` と `internal/apply` の2つだけ(NFR の設計 Q2)。
- **順番(Testing Contract の custom):** 受け入れ条件のテストを先に書いてコミットし、次に実装、実装の後に細かい単体テストを書き、最後にすべてのテストを実行する。
- **レビューで残った Minor の扱い(この計画で決める):**
  - 機能設計: 状態の図に pending から skipped(BR4.7 の読み直し)を足し、期待するタグの例の表の4行目の空行を消す(Step 13 でドキュメントと一緒に直す)。BR4.7 で飛ばした項目の `touchedKeys` は残り、U7 は status が skipped の項目を判定しない(契約 C4 の注のとおり)。
  - インフラの設計: `verify.sh` の後片付けの旗は、手順 5 の add の前にも立て直し、手順 5 の remove を記録で確かめてから下ろす(Step 12)。
- **AWS への変更は、この計画では行わない:** U6 の確認用のスタックの作成は、コードを書いたあと、チェックポイントの前に、変更セットを人に示して許可を得てから行う(`cicd-pipeline.md` の手順)。R3 を引き受けるプロファイル `tag-refactor-apply` は人が手元に足す。
- **確かめながら決めること(結果は `code-summary.md` に書く):**
  - 書き換えの対象の種類の表(NFR の設計 §2)の各行の `Resource` の形を、Service Authorization Reference と照らす(Step 8 の最初)。リソースの種類を持たない操作は `"*"` にし、理由を README に書く。
  - SDK の `TagResources`・`UntagResources` の答えの `FailedResourcesMap` の `FailureInfo` に `StatusCode` があること(SDK の型で確かめる)。

## 手順

- [x] **Step 1: テストの準備** — `unit-test-instructions.md` のコマンドが動くことを確かめる。HTTP の層の偽の AWS(`internal/testutil`)に、`tag:TagResources`・`tag:UntagResources`(1回20個まで。リソースごとの失敗の一覧に `StatusCode` とコードを入れられる)、ARN を指定した `tag:GetResources`(100個まで。一度もタグのないリソースを答えに出さない設定)、S3 の `PutObject`(`IfNoneMatch` で 412・409・501 を返せる)・`GetObject`・`DeleteObject` を足す。書き込みの回数、同時に受けている要求の数(リージョンごと)、応答の遅れ、誤りの応答を決められるようにする。(対応: Testing Contract の runner_step)
- [x] **Step 2: 受け入れ条件のテスト(先に書く。`internal/cli/acceptance_u6_test.go`)** — 偽の AWS と一時のホームで `apply` を動かし、AC4.1.1(`yes` 以外で何も書かない)、AC4.1.2(保存した変更案どおりに書く)、AC4.1.3(影響なしで `--yes`)、AC4.1.4(確認画面の `!`・`?` と文字)、AC4.1.5(`--yes` だけで影響ありなら終了コード 2、何も書かない)、AC4.1.6(`--yes --allow-impact`)、AC4.1.7(取り消しのコマンドと、U7 が読む記録)、AC4.2.1(変更案の後に変わったリソースを飛ばす)、AC4.2.2(権限のないリソースがあっても残りを書き、件数・一覧・ログファイル)、AC4.2.3(最初の書き込みの前に記録が保存されている。手元と S3)、AC4.2.4(`aws:` を書かない)、AC4.2.5(51個になるリソースを飛ばす)、AC4.2.6(保存の途中で止めても記録が壊れない)、AC4.2.7(同時の apply で後の方が止まり、記録が壊れない)、AC6.1.1(`apply --help`)を確かめる。この時点では失敗することを確かめ、これだけで1つのコミットにする。(対応: US4.1、US4.2、AC6.1.1)
- [x] **Step 3: AWS の口(`internal/awsaccess`)** — TagWriter(`TagResources`、`UntagResources`。`aws:` のキーを大文字・小文字を区別せずに拒む)、ObjectStore(`Put`、`Get`、`PutIfAbsent`、`Delete`。412・409 は Conflict、501 は StorageFailed)、ARN を指定した今のタグの読み取り。リソースごとの失敗を `StatusCode` を主にして C12 の種類に分ける表(403・404・400・429・5xx、コードの名前は補助)。操作の表に書き込みの操作を足し、書き換えの対象の種類の表(11種類)を足す。すべて U1 の `invoke` を通す。(対応: NFR3.10、NFR3.12、NFR5.12、security-design.md §1・§5、契約 C6)
- [x] **Step 4: 記録と印(`internal/journal`)** — RunRecord と JournalEntry(契約 C4)、`Lock`・`Release`・`Begin`・`RecordBefore`・`RecordAfter`・`Complete`・`Load`(契約 C10。`Lock` に runId を渡す)。壊れない保存(一時ファイル `.<runId>.tmp-<乱数>`、`Sync`、`os.Rename` を合計5回まで試す、Linux と macOS はフォルダーも `Sync`、1時間より古い一時ファイルの掃除)、S3 の保存(ACL なし)、印(手元は O_EXCL、S3 は PutIfAbsent、手元 → S3 の順で取り、S3 → 手元の順で消す、消す前に runId を確かめる)、runId の形、5秒ごとの保存の役(写しはロックの中で深い複製、変更がなければ保存しない、重ねない、3回続けて失敗したら `RecordAfter` が StorageFailed を返し続ける、最後の保存のあとは閉じる)、ログファイル(タグの値とエラー文を書かない)、権限(0700・0600、ほかの利用者が読めるフォルダーの警告)。ファイルの操作と時計を差し替えられるようにする。(対応: BR3.1、BR3.2、BR3.7、BR3.8、BR4.4、BR4.6、BR5.1〜BR5.5、NFR4.1、NFR6.1、NFR6.2、NFR7.1、NFR8.23〜NFR8.25、NFR9.9)
- [x] **Step 5: 書き換えの流れ(`internal/apply`)** — 機能設計 W1 の手順 7〜13: 印、確認の呼び出し(確認そのものは cli から渡す関数)、最初の読み取りと変化の確かめ(W2。一度もタグのないリソースの扱い)、期待するタグと付けるタグ・消すキー(U4 の規則 1.9 の部品)、`aws:` の除外、50の上限と消すのを先にする印、まとまり(JSON の鍵、20個)と読み直しの単位(100個)、リージョンの役(4つまで)、読み直し(BR4.7)、書き込みの順(BR4.1)、失敗の3つの区別と読み戻しで決めること、数え役(NFR5.17)、2段の止め方、すべての goroutine の `recover`、終わりの処理を取り消されない `context` で行うこと、結果(ApplyResult)と終了コード(BR6.2)、進み具合(契約 C11)。(対応: BR1.4、BR1.5、BR3.3〜BR3.6、BR4.1〜BR4.7、BR6.1、BR6.2、NFR5.13、NFR5.14、NFR5.17、logical-components.md)
- [x] **Step 6: 表示(`internal/display`)** — 確認画面(BR2.1。影響の `!`・`?` と文字、不完全の印、IaC の警告、衝突と未指定の件数、記録の場所)、`--yes` のときの要約、結果の一覧(飛ばしと失敗の理由、確かめられなかった件数、ログファイル)、取り消しのコマンド(BR6.3)、印の中身と `--force-unlock` の案内、`--allow-impact` だけのときの案内(BR2.7)。英語と日本語の文言(U4 の文言の一覧のテストを通す)。U4 の言語と色の部品を使う。(対応: BR2.1、BR2.7、BR6.1、BR6.3、NFR11.8、AC6.1.2、AC6.1.3)
- [x] **Step 7: コマンドの入口(`internal/cli`)** — `apply` のサブコマンド(U1 の「まだ使えない」を置き換える): オプション(`--plan`・`--rules` のどちらか1つ、`--yes`、`--allow-impact`、`--record-to`、`--force-unlock`、`--profile`、`--lang`)、`--record-to` の形の確かめ、アカウントの確かめ(BR1.2)、`--rules` のときに plan と同じ部品で変更案を作る(BR1.3)、端末の確かめ(BR2.3)、`--yes` と影響(BR2.4〜BR2.6)、確認の入力(Ctrl+C と EOF は `yes` 以外)、合図の役(1回目と2回目の Ctrl+C)、TagWriter と ObjectStore を apply にだけ渡す(NFR3.10)、ヘルプ(BR6.4)。(対応: BR1.1〜BR1.3、BR2.2〜BR2.7、BR4.5、BR6.4、NFR3.10)
- [x] **Step 8: 権限の例と確認用のテンプレート** — 最初に、書き換えの対象の種類の表の各行の `Resource` の形を Service Authorization Reference と照らして確かめ、結果を `code-summary.md` に書く。`docs/iam/apply-policy.json`(4つの文: 読み取りの例のすべて、`tag:TagResources`・`tag:UntagResources` を `"*"`、種類ごとのタグの操作、記録の S3 の4つ)。`internal/awsaccess/docs_test.go` に、例の操作の集まりが許可の一覧と完全に一致することと、テンプレートの R3 と例が同じことを足す。`test/sandbox/u6-fixtures.yaml`(VU6、GU6、BU6R、BU6RPolicy、R3、出力3つ)、`test/sandbox/u6-add-rules.yaml`・`u6-remove-rules.yaml`・`u6-untagged-rules.yaml`。`cfn-lint` を手元で通す(無ければ CI で確かめる)。(対応: NFR3.11、NFR8.25、cicd-pipeline.md)
- [x] **Step 9: 受け入れ条件のテストを通す** — Step 2 のテストと、U1〜U5 の受け入れ条件のテストがすべて通ることを確かめる。U1 の `apply` の「まだ使えない」を確かめるテストは、`apply` が使えるようになったことに合わせて直す(`rollback` と `iac` は「まだ使えない」のまま)。期待が変わるものは理由を `code-summary.md` に書く。
- [x] **Step 10: 細かい単体テスト(実装の後)** — `internal/journal`: 保存の途中の失敗(一時ファイル、書き出し、名前の変更)、Windows の名前の変更の4回失敗と5回失敗、5秒ごとの保存(重ねない、変更なしで保存しない、2回失敗のあと成功、3回で StorageFailed、閉じたあとの拒否)、印(競争、S3 の 412・409・501、runId の確かめ、`--force-unlock`)、権限と警告、ログファイルにタグの値とエラー文がない。`internal/apply`: 期待するタグの例(機能設計の表の4行)、50の上限と消すのを先にする、まとまりの鍵(`=`・`,`・改行・引用符)、読み直しの単位、変化の確かめと一度もタグのないリソース、失敗の分け方(403・404・400・429・500、状態なし、読み戻しで決める3つ)、数え役(9回で止まらない、10回で止まる、成功で0)、2段の止め方(1回目で動いている呼び出しを待つ、2回目で中断し pending)、各 goroutine のパニックで印が消える、取り消しのあとも最後の保存と印を消すことが成功する、終了コード。`internal/awsaccess`: TagWriter の `aws:` の拒否、ObjectStore、ARN を指定した読み取り、読み取りのコマンドに書き込みの口が渡らない(型の確かめ)、偽の S3 の要求に ACL がない。`internal/display`: 確認画面と結果の期待の出力(英語・日本語・`NO_COLOR`)。`internal/cli`: オプションの誤り、端末でないとき、アカウントの不一致、Ctrl+C。(対応: 機能設計の各規則、NFR3.10〜NFR11.8)
- [x] **Step 11: 大きな偽の AWS のテスト** — 測る条件の2つの分布(1万リソース、17リージョン、偏ったリージョン2,000件。よくある変更と、値がばらばらなキー名の変更)で、呼び出しの回数が NFR5.14 の式の上限以下(置いた件数とまとまりから式で計算する)、1回の書き込みが20以下・1回の読み取りが100以下・同時のリージョンが4以下・リージョンの中は重ならない(NFR5.13)、遅れなしでツール自身の処理が30秒以内(NFR5.16)。`race` のビルドタグのときは件数を10分の1にし、上限を10倍にする。U6 の大きなテストは CI の各 OS で合わせて2分以内。(対応: NFR5.13、NFR5.14、NFR5.16、NFR10.10)
- [x] **Step 12: 検証のスクリプト** — `scripts/verify.sh` の最後(U2 の確かめのあと、U2 のタグのないキューを消す前)に、`cicd-pipeline.md` の手順 1〜7 を足す: スタックと R3 のプロファイルの確かめ、`plan` による範囲の確かめ、add と remove の5回(3回目は S3 の記録)を記録の JSON で合否、残っていないことの確かめ(ap-northeast-1 と us-east-1、`iam list-role-tags`、最大6回の読み直し)、一度もタグのないキュー、S3 の記録(暗号化、ACL、ないキーが NotFound)、L95 と2つの見積もりの表示。後片付けの旗(最初の add の前と手順 5 の add の前に立て、確かめたら下ろす)と、既存の `cleanup` の先頭での remove。先頭のコメントの手順の説明を直す。(対応: NFR3.11、NFR5.15、NFR7.1、NFR8.25、cicd-pipeline.md)
- [x] **Step 13: 全体の確認とドキュメント** — `gofmt -l`、`go vet ./...`、`go vet -tags integration ./...`、`golangci-lint run`(`integration` のタグありとなし)、`go test ./...`(Go 1.27 と 1.26)、カバレッジ80%以上(`-coverpkg`)、`govulncheck ./...`。下限を下げて通すことはしない。`README.md` に apply の節(使い方、確認と `--yes`・`--allow-impact`、記録の場所と S3、印と `--force-unlock`、手元の印は別の端末を防げないこと、Windows の権限の限界、書き換え用の権限の例と表にない種類の案内、検証用アカウントで確かめていない種類、SSE-KMS のバケットの権限、値がばらばらなキー名の変更は書き込みが件数の2倍になること)と、U6 の確認用のスタックの置き方と消し方を足す。機能設計 `functional-spec.md` の状態の図と表の Minor を直す。

## ストーリーとの対応

| ストーリー | 手順 |
|---|---|
| US4.1(確認してからタグを書き換える) | Step 2、Step 5〜Step 7、Step 9、Step 10 |
| US4.2(書き換えの途中の問題で被害を広げない) | Step 2〜Step 5、Step 9〜Step 11 |
| US6.1 の AC6.1.1(`apply` のヘルプ) | Step 2、Step 7 |
| (NFR の設計: 書き込みの口、記録と印、失敗の分け方、まとめた書き込み) | Step 3〜Step 5、Step 10、Step 11 |
| (インフラの設計: 権限の例、確認用のスタック、検証のスクリプト) | Step 8、Step 12 |

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

