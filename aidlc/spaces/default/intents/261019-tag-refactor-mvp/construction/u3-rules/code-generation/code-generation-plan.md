# コードの生成の計画: U3 ルール

## Summary

- Builds: ルールファイル(契約 C1)のすべての項目の読み込みと矛盾の確かめ、UTF-16 の読み込み、必須タグの値・正しい値・消すかどうかの照会、`rules suggest`(`--output` つき)、ファジングと読み戻しのテスト、`verify.sh` への U3 の実機の確認
- Touches: `internal/rules/`、`internal/cli/`(rules suggest、接続の二段化、Load のオプション)、`internal/inventory/`(種類の照会の関数の公開)、`internal/display/`(下書きの出力の口)、`scripts/verify.sh`、`.gitattributes`、`README.md`
- Tests: 受け入れ条件のテスト約8件(先に書く)、パッケージごとの単体テスト5〜8件(約40件)、ファジング(種の入力は CI)、読み戻し、性能、検証用アカウントでの rules suggest の確認(手元だけ)。カバレッジ80%以上

## 前提と進め方

- 設計の出典: `construction/u3-rules/` の機能設計、非機能要件、非機能の設計、インフラの設計。契約 `inception/contract-design/contract-summary.md`(C1、C5、C8)。ストーリー US2.1(AC2.1.1〜AC2.1.5)。
- **ブランチ:** `bolt/u3-rules`(`main` から作成済み)。ここまでの AI-DLC の記録はコミット済み(`f7a7b4e`)。マージは人がプルリクエストで squash マージする。
- **既存のコードに手を入れる:** U1・U2 のファイルをその場で直す。同じ役割のファイルを別に作らない。
- **順番(Testing Contract の custom):** 受け入れ条件のテストを先に書いてコミットし、次に実装、実装の後に細かい単体テストを書き、最後にすべてのテストを実行する。
- **U4 との境目:** AC2.1.3 と AC2.1.4 は、U3 では照会の部品と単体テストまで(機能設計 Q2)。`plan` の変更案への組み込みと受け入れ条件のテストは U4。
- **確かめながら決めること(結果は `code-summary.md` に書く):**
  - goccy/go-yaml が `\UXXXXXXXX` のエスケープを読めること(NFR の設計のレビューで確かめ済み。読み戻しのテストで実際に確かめる)。
  - Go の `testing.F` の種のファイルの置き場と形(`testdata/fuzz/FuzzParse/`)。
  - `.gitattributes` に `**/testdata/fuzz/** -text` を足したあと、`git check-attr text` が `unset` になること。

## 手順

- [x] **Step 1: テストの準備** — `unit-test-instructions.md` のコマンドが動くことを確かめる。`internal/testutil` の偽の AWS に足すものはない(U2 のまま)。U3 の受け入れ条件のテストのための手助け(ルールファイルを一時フォルダに書く、`--output` の一時ファイル)を `internal/cli` のテストに用意する。(対応: Testing Contract の runner_step)
- [x] **Step 2: 受け入れ条件のテスト(先に書く)** — `internal/cli/acceptance_u3_test.go` に、コマンドの入口から流す Given/When/Then のテストを書き、このステップだけで1つのコミットにする。対象: AC2.1.1(表記ゆれのある偽の AWS で `rules suggest` → 下書きが標準出力に出て、`env`・`Env`・`environment` が1つのまとまり、そのまま `plan --rules` に渡すと読み込める)、AC2.1.1(`--output` に UTF-8 で書かれ、標準出力は空)、AC2.1.2(下書きの別名のまま `plan` → キー名の変更)、AC2.1.5(綴りの誤り、型の誤り、別名の重なり、`resourceType: AWS::EC2::Instance`、`match: {}`、消すキーが別名 → 行・キー・直し方、終了コード 1、AWS を呼ばない)、NFR5.9(呼び出し回数の上限のあと終了コード 0、1つのリージョンの拒否で下書きは出て終了コード 1 と先頭のコメント)、NFR3.6(rules suggest が3つの操作だけを呼ぶ)。この時点では失敗する。(対応: US2.1、NFR10.2)
- [x] **Step 3: ルールファイルの読み込みの入口(`internal/rules`)** — `Load(path, opts)`、`Parse(path, data, opts)`、`Options{TypeHint}`。`Load` は種類の確認と 128 KiB + 1 バイトまでの読み取りだけにし、文字コードの見分け(UTF-8 の BOM、UTF-16LE・BE の BOM、BOM のない UTF-16 の目安)、UTF-16 から UTF-8 への変換(奇数の最後のバイトと対のないサロゲートは U+FFFD)、変換したあとの 64 KiB の判定を `Parse` の先頭に移す。U1 の呼び出し元(scan、plan、テスト)を新しい形に直す。(対応: NFR9.5、NFR8.13、security-design.md §4)
- [x] **Step 4: 契約 C1 のすべての項目を読む(`internal/rules`)** — `KeyRule.Values`(`ValueRule`)、`RequiredTag` の `Default` と `When`(`RequiredCondition`)、`RemoveKeys`(`RemoveKey`)を `RuleSet` に入れる。文字数の確かめ(BR1.5)、`match` の確かめ(BR3.3〜BR3.5。`TypeHint` で CloudFormation の形の直し方、nil なら「条件に書けない」、`unknown` は誤り)、`aws:` のキーの規則の無視と警告(BR5.1。`match.tags` では使える)。U1 の「values / required / remove は使わない」の警告を読み込みの部品から外す。(対応: BR1.2〜BR1.5、BR3.3〜BR3.5、BR5.1)
- [x] **Step 5: 項目どうしの矛盾の確かめ(`internal/rules`)** — 値の別名の重なり(BR2.2)、必須タグのキーが別名・消すキー(BR3.1)、`match.tags` の別名・値の別名・消すキー(BR3.6)、消すキーが正しいキー名・別名(BR4.1)、重複した消すキーの警告。最初の誤りで止める(BR1.1)。(対応: BR1.1、BR2.1、BR2.2、BR3.1、BR3.6、BR4.1)
- [x] **Step 6: 照会(`internal/rules`)** — 読み込んだときに対応表(別名 → 正しいキー名、(キー, 値の別名) → 正しい値、消すキーの集まり)を作る。`CanonicalValue(key, value)`、`IsRemoved(key)`、`RequiredValue(key, resource)`(タグを別名・値の別名で置き換え、消すキーを除いてから `when` を上から比べ、なければ `default`、なければ未指定。グローバルなリソースは `region` の条件に当てはまらない。ぶつかるときは正しいキー名の今の値)。答えに決めた行を付ける。(対応: BR2.3、BR3.2、BR4.2、AC2.1.3・AC2.1.4 の規則)
- [x] **Step 7: 下書き(`internal/rules`)** — `SuggestInput{Resources, KeyGroup, Preferred}` から下書きを組み立てる。キーのまとめと正しいキー名の選び方(最多、同じなら辞書の正式な形、次に名前の順。BR6.1)、すべてのキー(BR6.2)、値のまとめ(20個まで、書き方が2つ以上のものだけ。BR6.3)、YAML のエスケープ(`\"`、`\\`、`\uXXXX`、`\UXXXXXXXX`、不正な UTF-8 は U+FFFD。BR6.4)、先頭のコメントと required・remove の例、0件なら `keys: {}`(BR6.5)、64 KiB への調整(コメントのバイト数を含めて差し引き、最後に実測。BR6.7)。改行は LF。(対応: BR6.1〜BR6.5、BR6.7、NFR8.12、NFR9.4)
- [x] **Step 8: inventory と display(`internal/inventory`、`internal/display`)** — `inventory` の種類の対応表を引く関数を公開する(`rules.Options.TypeHint` に渡す)。`display` に下書きのバイト列を標準出力にそのまま書く口を足す。(対応: BR3.4、security-design.md §3)
- [x] **Step 9: コマンド(`internal/cli`)** — `rules suggest`(`--profile`、`--region`、`--output`、`--debug`。`--rules` は受け付けない)。接続の関数を二段に分け、`rules suggest` はアカウントとタグの取得だけを使う。`inventory` の収集を呼び(一度もタグが付いていないリソースは探さない)、結果を `SuggestInput` に写し、`inventory.CanonicalKey` と辞書を渡す。`--output` は同じフォルダの一時ファイルに書いてから名前を変え、既にあるファイルは上書きしない(誤り)。終了コード(BR6.6、BR6.7)と `Checked:` の行。scan と plan で `rules.Load` に `TypeHint` を渡す。plan の「values / required / remove are read but not applied to the plan yet」の1行。誤りと警告の文言は `%q` と `display` の置き換えを通す。(対応: BR6.6、BR6.7、NFR3.6、NFR3.7、NFR5.9、NFR8.14、C5)
- [x] **Step 10: 受け入れ条件のテストを通す** — Step 2 のテストと、U1・U2 の受け入れ条件のテストがすべて通ることを確かめる。U1・U2 のテストで、U3 の変更(読み込みの警告の文言、`Load` の引数)により期待が変わるものは直し、理由を `code-summary.md` に書く。
- [x] **Step 11: 細かい単体テスト(実装の後)** — `internal/rules`: 文字コード(UTF-8 の BOM、UTF-16LE・BE、BOM のない UTF-16 の ASCII だけ・日本語あり、奇数バイト、128 KiB 超)、C1 の各項目の読み込み、矛盾の確かめの各規則、照会(when の順、default、未指定、グローバルなリソース、置き換えたあとのタグ、ぶつかり)、下書き(正しいキー名の選び方、値の20個、`{}`、`keys: {}`、エスケープ、64 KiB の調整の実測)。`internal/cli`: `--output`(上書きしない、一時ファイルが残らない)、終了コード。`internal/inventory`: 種類の照会。(対応: BR1.x〜BR6.x、NFR8.12、NFR9.5)
- [x] **Step 12: ファジング、読み戻し、性能のテスト** — `FuzzParse`(パニックしない、1回1秒以内(`race` のビルドタグで10倍)、行番号の範囲、読めたときの不変条件。種: 正しいファイル、機能設計の誤りの例、深い入れ子、長い行、不正な UTF-8、UTF-16LE・BE、BOM のない UTF-16)。種のファイルを `internal/rules/testdata/fuzz/FuzzParse/` に置き、`.gitattributes` に `**/testdata/fuzz/** -text` を足して `git check-attr` で確かめる。読み戻し(乱数の種を固定。制御文字、補助面、引用符、`:`、`#`、空白、日本語、128文字のキー、256文字の値)。性能: NFR1.11(1万件・キー500個・半分は値50個・半分は値20個以下で調整が働く入力で5秒以内)、NFR1.12(1万件 × 照会3種類で2秒以内)。`race` のときは件数を減らす。(対応: NFR1.11、NFR1.12、NFR8.13、NFR10.4)
- [x] **Step 13: 検証のスクリプト** — `scripts/verify.sh`: `cleanup` と `trap cleanup EXIT` の定義をスクリプトの先頭近くに移し、キューの名前と一時ファイルの名前が空のときは何もしないようにする。U1 の確認とアカウントの確認のあと、U2 のキューの前に U3 の確認を足す: U1 のスタックの状態を確かめ、`rules suggest --region ap-northeast-1 --output <mktemp のファイル>` を読み取り用のプロファイルで動かし(終了コード 0、標準出力が空)、先頭のコメントと `env`・`Env` のまとまりを確かめ、`scan --region ap-northeast-1 --rules <ファイル>` で読み込めることを確かめる。(対応: cicd-pipeline.md の検証のスクリプト)
- [x] **Step 14: 全体の確認(手元)** — `gofmt -l`、`go vet ./...`、`go vet -tags integration ./...`、`golangci-lint run`、`go test ./...`、カバレッジ80%以上(`-coverpkg`)。手元でファジングを5分動かし、結果を `code-summary.md` に書く(Application Control でふさがれたら再実行)。下限を下げて通すことはしない。
- [x] **Step 15: ドキュメント** — `README.md`(`rules suggest` の使い方と `--output`、Windows の PowerShell での保存、ルールファイルの書き方(values、required の default と when、remove)、条件に書ける種類の形と制限、`when` の評価の順、`match.tags` は置き換えたあとのタグと比べること)。

## ストーリーとの対応

| ストーリー | 手順 |
|---|---|
| US2.1(統一の基準をルールファイルにする) | Step 2〜Step 12、Step 15 |
| (U4 で plan に組み込む規則: AC2.1.3、AC2.1.4) | Step 6、Step 11 |
| (インフラの設計) | Step 12(`.gitattributes`)、Step 13 |

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
