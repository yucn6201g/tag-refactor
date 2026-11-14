# コード生成の計画: U1 最小の一本

## Summary

- Builds: `iac --pr` と、TypeScript の CDK アプリ1つのキーの名前の変更の反映(synth、補助のプログラム、照合、書き換え、プルリクエストの作成)、CI の準備
- Touches: 新しい `internal/procexec`、`internal/redact`、`internal/cdk`(埋め込みの JavaScript を含む)、`internal/pullrequest`。今の `internal/iac`、`internal/cli`、`internal/display`、`.github/`、README の `iac` の節
- Tests: 受け入れ条件のテスト 9 本と結合テスト 1 本を先に、実装のあとで部品ごとに 5〜8 本の単体テスト(約 60 本)と、実際の synth を動かすテスト

## 元にしたもの

- 機能の設計: `construction/u1-walking-skeleton/functional-design/`(BR1.1〜BR5.2)
- 非機能の要件と設計: `construction/u1-walking-skeleton/nfr-requirements/`、`nfr-design/`
- インフラの設計: `construction/u1-walking-skeleton/infrastructure-design/`
- 契約: `inception/contract-design/contract-summary.md`(X1、X2、C1〜C7)
- 単位: `inception/units-generation/unit-of-work.md`(U1)
- 要件: `inception/requirements-analysis/requirements.md`

## 前の段階から持ち越して、この計画で決めること

| 持ち越し | この計画での決め方 | 手順 |
|---|---|---|
| 機能の設計のレビューの残り: キーの箇所が0か2以上のときの場所と理由を出す段階 | CDK の読み取り(手順 9)で、変更前のキーの箇所の数を数え、0 なら今の判定(keyNotInSource か反映済み)、2 以上なら cdkAmbiguous を、場所(place)を決める前に出す。場所のファイルは、変更前のキーの箇所があればそのファイル、無ければ作成の箇所のファイル | 9 |
| 機能の設計のレビューの残り: BR3.4 と契約の recheck_failed の理由 | Recheck に合わないときは、契約 C7 のとおり cdkUnsupportedShape にする(BR3.4 の unreadableFile を改める)。`--debug` に理由を出す | 9 |
| 機能の設計のレビューの残り: BR4.1 の確かめ | Check に、行き先のブランチがリモートにあること(`git ls-remote --heads`)と、merge、rebase、cherry-pick、revert の途中でないこと(Git の管理フォルダーの印のファイル)を足す | 10 |
| 非機能の設計のレビューの残り: 補助のプログラムの結果の大きさ | 補助のプログラムの標準出力(結果の JSON)だけは上限を 16 MiB にし、ほかの取り込みは 64 KiB のまま | 5 |
| インフラの設計のレビューの残り: 置くときのプロファイル | 検証用のスタックは、今の README の手順と同じ管理者のプロファイル `tag-refactor-sandbox` で置く(人の明示の許可を得てから) | 14 |
| インフラの設計のレビューの残り: ブートストラップの無いアカウント | テスト用のアプリは `DefaultStackSynthesizer({ generateBootstrapVersionRule: false })` にし、`stackName` を `tag-refactor-cdk-skeleton` に固定する。synth の出力にブートストラップの版のパラメータとルールが無いことを確かめる | 3 |
| インフラの設計のレビューの残り: 取り消された CI の残り物 | github-pr のテストの始めに、作ってから1日を超えた `tag-refactor/iac-` で始まるブランチとそのプルリクエストを片付ける | 4 |
| インフラの設計のレビューの残り: テスト専用のリポジトリの依存 | テスト専用のリポジトリのアプリは、tag-refactor の `ts-minimal` の写しとし、写す手順を README(テスト専用のリポジトリのもの)に書く | 14 |
| 契約の未決事項: 外への問い合わせの止め方 | 固定した CDK の CLI の版の文書で `CDK_DISABLE_VERSION_CHECK`、`--no-notices`、`CDK_DISABLE_CLI_TELEMETRY` を確かめ、`code-summary.md` に書く | 7 |

## Testing Contract

```json
{
  "version": 1,
  "methodology": "custom",
  "source": "team",
  "ordering": "まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。",
  "scope": "mvp",
  "test_strategy": "standard",
  "project_type": "brownfield",
  "applicable_notes": [
    {
      "layer": "org",
      "text": "We treat tests as a first-class deliverable in every Bolt. The specific\nmethodology (TDD, BDD, ATDD, or classic test-after) is affirmed at\npractices-discovery and recorded in `team.md` under this heading with explicit\n`Methodology` and `Ordering` fields; Code Generation resolves those fields\nindependently from coverage, tooling, and scope notes.\n\nWhen no posture has been affirmed, our default per scope is:\n- **Methodology**: test-after\n- **Ordering**: implement each applicable testable layer, then write and run\n  that layer's tests.\n- `mvp`, `enterprise`, `feature`, `infra`, `classic` add an 80% line-coverage\n  floor and CI execution before merge.\n- `bugfix`, `security-patch` add a targeted regression for the specific\n  bug/vulnerability and require the existing suite to remain green.\n- `express` uses the Minimal strategy: requirement-driven unit tests (one per\n  requirement, with a happy-path floor per component); existing tests remain\n  green.\n- `poc`, `refactor`, `workshop` add no extra new-test floor and require the\n  existing suite to remain green.\n\nThe active `Test Strategy` still applies in every scope and determines test\nvolume/types. Scope floors are additive; they never reduce or replace the\nselected strategy.\n\nBuild and Test verifies defined coverage floors and affirmed quality targets;\nthey may not be weakened to make a step pass.\n\nAffirm a stricter posture in `team.md` if the team commits to one."
    },
    {
      "layer": "team",
      "text": "私たちはテストを、すべての Bolt で作る成果物の一部として扱います。\n\n- **Methodology**: custom\n- **Methodology evidence**: 前の作業(`261019-tag-refactor-mvp`)の面談で、テストの進め方の質問の回答は C(受け入れ条件(Given/When/Then)を先にテストにし、細かい単体テストは実装のあとに書く)でした。テストを先に書く部分と、実装のあとに書く部分が混ざっているため、`tdd`、`bdd`、`atdd`、`test-after` のどれにも当てはまらず、`custom` とします。コミットの履歴にも、`iac` の受け入れ条件のテストを実装の前に足したコミット(`2296959`)があり、この進め方が守られています。\n- **Ordering**: まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。\n- カバレッジの下限は80%です。Go の標準の測り方(`go test -cover` の文(statement)単位)で測ります。コマンドの入口などの配線部分も、外のプログラム(`git`、`gh`、`cdk`、Node.js、Python)を呼ぶ部分も、計測の対象から外しません。外のプログラムの呼び出しは差し替えてテストします。\n- マージの前に、プルリクエストの CI でテストを実行します。\n- AWS を呼ぶ部分の単体テストでは、AWS の呼び出しを差し替え、AWS の認証情報なしで CI で実行します。実際のアカウントを使う結合テストは、手元だけで実行します。CI から実際の AWS アカウントは呼びません。`cdk synth` には AWS の認証情報を渡さず、テストには AWS を呼ばない CDK のアプリだけを使います。\n- GitHub については、CI でも実際の GitHub にプルリクエストを作る結合テストを実行します。テスト専用のリポジトリを1つ用意し、そこにだけ書ける fine-grained トークンを CI の secret に置きます。secret の無い実行(fork からのプルリクエスト、Dependabot のプルリクエストなど)では、このテストを飛ばします。テストで作ったブランチとプルリクエストは、テストのあとで閉じて消します。\n- CI のテストは、Linux、macOS、Windows の3つの OS と、Go の `stable` と `oldstable` の2つの版で、配るバイナリと同じ `CGO_ENABLED=0` で実行します。Linux では `-race` 付きでも実行します。\n- `cdk synth` を動かすテストも、CI の3つの OS すべてで実行します。そのために、CI に Node.js、Python、CDK の CLI を入れます。\n- `cdk synth` を動かすテストで Node.js、Python、CDK の CLI が見つからないとき、CI ではテストを失敗にします(飛ばして通すことはしません)。手元では、理由を出してテストを飛ばします。\n- テストのデータとして置く CDK のアプリ(TypeScript と Python)の依存(npm と pip のパッケージ、CDK の CLI)は、版を固定し、Dependabot で更新します。\n- テストの量と種類は、ワークフローのテスト戦略(Test Strategy)で決まります。上の下限は、それに加わる条件です。下限を下げて Build and Test を通すことはしません。"
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
    "runner_step": "Verify the existing test runner/configuration and record the exact unit-scoped command.",
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
      "Verify the existing test runner/configuration and record the exact unit-scoped command.",
      "Custom ordering - まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。",
      "Implementation and tests - preserve that exact ordering; do not convert it to layer-local TDD.",
      "Environment/build configuration.",
      "Documentation and traceability."
    ]
  },
  "input_sha256": "sha256:a0d6750d5eccdd3aff7d4d1895f455934eeacec8431076f4c18acc04ee0858cd",
  "contract_sha256": "sha256:36a5093ffce4ef8b71667a32cc2ced29ea3105b4fc692f6ae7906d40b17cc996"
}
```

## 手順

チームの進め方(custom)に従い、受け入れ条件のテストを先に書き(手順 4)、実装(手順 5〜11)、実装のあとで細かい単体テスト(手順 12)、すべてのテスト(手順 15)の順に進めます。

### 構成と準備

- [x] **Step 1: 作業ブランチ。** `git fetch origin` のあと、このフォルダーで `git switch -c bolt/v0.2-u1-walking-skeleton origin/main` を実行する(人の許可済み)。`aidlc/` の下の未コミットの記録はそのまま持ち越す。切り替えのあと `git status` で、記録だけが変更として残ることを確かめる。README の `plan` の節、`internal/apply`、`scripts/`、`internal/iac/testdata` には触らない。(全体)
- [x] **Step 2: テストの道具の確かめ。** `GOTOOLCHAIN=local` で `go test ./internal/iac/ ./internal/cli/` が今の main で通ることを確かめ、`unit-test-instructions.md` の単位の命令を記録する。Node.js と npm の版を確かめる。(全体)
- [x] **Step 3: テスト用の CDK アプリ。** `internal/cdk/testdata/apps/ts-minimal/` を作る。
  - 中身は `package.json`(`aws-cdk-lib`、`constructs`、`aws-cdk`、`typescript`、`ts-node`、`@types/node` を固定の版で)、`package-lock.json`(`npm install` で作る)、`cdk.json`、`tsconfig.json`、`bin/app.ts`、`lib/stack.ts`。
  - スタックは `new SkeletonStack(app, 'tag-refactor-cdk-skeleton', { stackName: 'tag-refactor-cdk-skeleton', synthesizer: new DefaultStackSynthesizer({ generateBootstrapVersionRule: false }) })`。
  - バケットは `new s3.Bucket(this, 'Bucket', { blockPublicAccess: BLOCK_ALL, encryption: S3_MANAGED, removalPolicy: DESTROY })`。タグは `Tags.of(bucket).add('Env', 'dev')`。
  - 手元で `npx cdk synth` を動かし、manifest の `stackName` と `environment` の場所、テンプレートの論理 ID と `aws:cdk:path`、ブートストラップの版のパラメータとルールが無いことを確かめて記録する。整形も lint もしない。
  - (US2.1、US3.1、BR2.2 の例)

### 受け入れ条件のテスト(先に書く)

- [x] **Step 4: 受け入れ条件と結合テストを先に書く。** `internal/cli/acceptance_v02u1_test.go` と `internal/cli/github_pr_test.go` を書く。この時点では失敗することを確かめる。各テストの名前は `TestV02U1` で始める(結合テストは `TestGitHubPullRequest`)。
  1. AC1.1.1(US1.1): テスト用のアプリを含む一時的な git のリポジトリと、手元の裸のリポジトリを origin にし、gh を差し替えた環境で `iac --pr`。書き換え、ブランチ、`--only` のコミット(ほかのステージ済みのファイルが入らない)、push、`gh pr create` の引数(`--base`、`--head`、`--title`、`--body-file`)、URL の1行、確認の問いが無いことを確かめる。
  2. AC2.1.2(US2.1): 標準エラーに、待ち、実行中、済み(経過時間つき)の行が出る(実際の synth)。
  3. AC2.1.3(US2.1): AWS の変数(小文字を含む)と GH_TOKEN を入れた環境で、差し替えた synth に渡った環境にそれらが無く、空の設定ファイルと止める変数があること。needsAWS の文を出す差し替えの synth で、理由と対処が出て AWS の呼び出しが無いこと。
  4. AC2.1.6(US2.1): 変更案の stack-name と logical-id のタグから、ソースの `lib/stack.ts` と作成の行が分かる(`--debug` か結果の表示)。
  5. AC2.1.7(US2.1): 道具が無いとき、`CI` が空でなければ失敗、空なら飛ばす(テストの補助の関数を差し替えて確かめる)。
  6. AC3.1.1 の一部(US3.1): アプリの写しに `iac --write` でキー `Env` を `Environment` に変え、写しを synth し直してバケットのタグが `Environment=dev` になり `Env` が無いこと(BR3.5)。差分がキーの範囲だけであること。
  7. 書き出しの失敗(BR4.2、NFR4.2): 2つ目のファイルの書き出しを失敗させると、git と gh が呼ばれず終了コード 1。
  8. NFR5: 変更案に stack-name のタグのリソースが無いと、synth も補助のプログラムも起動しない。
  9. 伏せ字(NFR1.2): 変更案のファイル名に 12 桁の数字と ARN の形を入れても、タイトル、本文、コミットのメッセージに現れない。
  10. 結合テスト AC1.1.11(US1.1): `TAG_REFACTOR_TEST_REPO` と `TAG_REFACTOR_TEST_REPO_TOKEN` が空でないときだけ、テスト専用のリポジトリで実際のプルリクエストを作り、`t.Cleanup` で閉じてブランチを消す。始めに古い残り物を片付ける。トークンの渡し方は security-design.md の6節のとおり(tag-refactor をビルドして別のプロセスで動かす)。

### 実装

- [x] **Step 5: `internal/procexec`。**
  - 非対話の `Run` と、対話の `RunInteractive` を作る。
  - 上限、`WaitDelay` 5 秒、取り込みの上限(64 KiB、補助のプログラムの標準出力だけ 16 MiB)を持たせる。
  - Unix はプロセスのグループで止める。Windows は CREATE_SUSPENDED のあとジョブオブジェクトに入れてから NtResumeProcess で動かす。
  - テストで差し替えられるよう、実行の口をインタフェースにする。
  - (NFR3.1〜NFR3.3、NFR3.10)
- [x] **Step 6: `internal/redact`。** 伏せ字の閉じた一覧(ARN、12 桁の数字、アクセスキーの ID、GitHub のトークン、URL の userinfo を `<redacted>` に)と、synth と補助のプログラムの標準エラーの許す行の一覧を作る。(NFR1.2〜NFR1.4)
- [x] **Step 7: `internal/cdk` の Synthesizer。**
  - 環境を作る関数: 大文字と小文字を区別しない名前の比べ方で外す。`TAG_REFACTOR_TEST_REPO_TOKEN` も外す。空の設定ファイルを作り、止める変数を足す。
  - Discover と言語と版の判定: `cdk.json` の app、package.json をさかのぼって探す。U1 は最初の1つだけを synth し、残りは notInU1 で外す。
  - cdk の探し方と引数、一時的な出力先、manifest とテンプレートの読み取り(スタック名、アカウント、リージョン、論理 ID、種類、`aws:cdk:path`、タグ)。
  - 失敗の分類: toolMissing、timeout、needsAWS の既知の文4つ、synthError。
  - `cdk.ProgressSink` の知らせ。
  - 外への問い合わせを止める設定を、固定した CDK の CLI の文書で確かめて記録する。
  - (BR1.1〜BR1.5、BR1.7、NFR2.1、NFR2.2)
- [x] **Step 8: 補助のプログラムと Analyzer。**
  - `internal/cdk/helper/ts-helper.js`(CommonJS の1ファイル)を書く。
    - analyze と recheck を持つ。
    - U1 の形(スタックの new、`this` を scope にした `const`/`this.x` の new、文字列そのままの `Tags.of(x).add`)から、コンストラクトのパス、ConstructSite、TagSite(KeyEdit、ValueEdit、RemoveEdit、Shape、SharedWith)を作る。
    - UTF-16 の位置を UTF-8 のバイトの位置に直す。
    - 版 1 の JSON で返す。
    - `typescript` はアプリのフォルダーから `require.resolve` で探す。
  - `go:embed` で入れる。
  - 書き出しの守りを付ける: 本人だけのフォルダー(Unix は 0700、Windows は本人だけの ACL)、ハッシュの確かめ、テストの差し込み口。
  - Analyzer を作る。
    - 起動の前に node の版(18 以上)を確かめる。
    - typescript の版(4.0 以上 5.x まで)を補助のプログラムが報告する。
    - 結果の版を確かめる。
    - 補助のプログラムの失敗を対応させる(BR1.6)。
  - (BR1.6、BR2.2、NFR2.3、NFR6.3〜NFR6.5)
- [x] **Step 9: `internal/iac` への組み込み。**
  - `ToolCDK`、`FileResult.Language`、`Result.CDKApps`、`Result.WrittenFiles`、U1 の ManualReason を足す。
  - `ExitCode` が failed と skipped のアプリを見る(notInU1 は除く)。
  - 照合: 変更案の AccountID とリソースの Region、Global を使う。
  - CDK の候補のファイルと CDK 用の読み取り(タグ一式、キーの箇所の数の判定、場所の決め方)を足す。
  - キーの範囲だけの書き換え、範囲と共有の規則(cdkScopeTag、cdkSharedValue)を入れる。
  - processAll と finish の間に、アプリごとの Recheck を置く。合わなければ cdkUnsupportedShape。
  - synth できなかったアプリの理由(BR2.3)を出す。
  - 今の CloudFormation と Terraform の動きは変えない。
  - (BR2.1、BR2.3、BR3.1〜BR3.4、C7)
- [x] **Step 10: `internal/pullrequest`。**
  - Check: ネットワークを使わない確かめを先にする(リポジトリの根、Prefix、GitDir、HEAD がブランチ、merge などの途中でない、リモートの決定と GitHub であること)。そのあと gh のログインと、行き先のブランチがリモートにあることを確かめる。
  - Publish: switch -c、add、`commit --only`(対話)、push(対話)、本文のファイルと `gh pr create --body-file`。
  - 段階ごとの上限と、段階ごとの `pullrequest.ProgressSink` の知らせを入れる。
  - git の環境は gitEnv の決まりに従う。commit と push では GIT_TERMINAL_PROMPT を足さない。
  - リモートの URL の userinfo を除く。タイトルと本文を伏せ字にする。
  - 失敗した段階と、済んだ段階を返す。
  - (BR4.1〜BR4.5、C1)
- [x] **Step 11: `internal/cli` と `internal/display`。**
  - `--pr` を足す(`--write` を含む)。
  - 順番は、確かめ → iac の実行 → 書き出しの失敗があれば止める → 本文の組み立て → Publish → URL。
  - 行き先の1行を示す。
  - `display.Printer` が `cdk.ProgressSink` と `pullrequest.ProgressSink` を実装し、英語と日本語の文を `messages` に足す。
  - `iac --help` に、CDK と `--pr` と「信頼できるリポジトリでだけ使う」を書く。
  - 終了コードは X1 のとおり。
  - (X1、X2、C6、NFR10.1〜NFR10.3)

### 単体テスト(実装のあと)

- [x] **Step 12: 部品ごとの単体テスト(各 5〜8 本)。**
  - `procexec`: 上限で止まる、孫が残らない、対話でグループを変えない、出力の上限。終わらない子はテストのバイナリ自身を使う。
  - `redact`: 5つの形、境目、許す行。
  - `cdk`: 環境、判定、cdk の探し方、manifest の読み取り、失敗の分類、一時的なフォルダーの片付け。
  - 補助のプログラム: node で動かして JSON を確かめる。UTF-8 の位置、共有の箇所、式は expression、定数は sameFileConst。
  - Analyzer: ハッシュが違うと起動しない、版の外、結果の版の違い。
  - `iac` の CDK の道: 照合の軸、箇所の数、範囲、共有、Recheck の失敗、ExitCode。
  - `pullrequest`: 確かめの各項目、段階の失敗、userinfo、伏せ字、上限。
  - `display`: 進み具合の行、言語、カタログのキーの抜け。
  - トークンの子の環境のテストと、ワークフローのファイルを読むテスト(NFR2.4)、`GH_TOKEN` を読む `os.Getenv` が無いことのテスト(NFR1.1)も足す。
  - ベンチマーク(200 ファイル、60 秒で失敗)も足す。

### 設定と CI

- [x] **Step 13: CI と依存の更新。**
  - `ci.yml` を直す。
    - test、race、coverage に setup-node(ハッシュで固定、Node.js 22、npm のキャッシュ)と `npm ci --ignore-scripts` を足し、上限を 30 分にする。
    - prettier の仕事を足す(版はこのときの最新の安定版に固定、10 分、`contents: read`)。
    - github-pr の仕事を足す(checkout、setup-go、setup-node、npm ci、手順の env にだけ secret と変数、20 分、`contents: read`)。
    - `ci-gate` の needs に prettier と github-pr を足す。
  - `.github/dependabot.yml` に npm の行を足す。
  - 補助のプログラムを Prettier で整形する。
  - (NFR6.8、NFR6.10、NFR8.1、NFR8.2、NFR9.1)

### 検証と文書

- [x] **Step 14: 検証の手順と文書。**
  - 検証コマンドの台本 `test/verify/v02-u1.sh` を作る。手元だけで動かす。
    - 検証用のスタックがあることを確かめる。
    - `scan` と、`Env` を `Environment` に変える規則で `plan` を行う。
    - テスト専用のリポジトリを一時的なフォルダーに clone し、`npm ci` のあと `iac --pr` を動かす。
    - プルリクエストの差分がキーの1か所だけであることを確かめ、プルリクエストを閉じてブランチを消す。
  - 検証用のスタックを置く手順と、テスト専用のリポジトリにアプリを写す手順を、`test/verify/README-v02-u1.md` に書く。
    - スタックはプロファイル `tag-refactor-sandbox` で置く。
    - 置く前に、テンプレートと命令を人に見せて明示の許可を得る。
  - README の `iac` の節を直す(`plan` の節には触らない)。書く中身は次のとおり。
    - `--pr`
    - CDK の扱い(TypeScript の1つのアプリ、文字列そのままのキーの名前の変更)
    - synth に認証情報を渡さないこと
    - 信頼できるリポジトリでだけ使うこと
    - 残るリスク
  - (検証、X1、X2)
- [x] **Step 15: すべてのテストと確かめ。**
  - `gofmt -l`、`go vet ./...`、`golangci-lint run`(手元にあれば)、`go test ./...`(CGO_ENABLED=0)、カバレッジ 80% 以上を確かめる。
  - Prettier の check、`govulncheck` も通す。
  - 失敗は直してから次に進む。下限は下げない。
- [x] **Step 16: 記録。** `code-summary.md`、`traceability.json`、`source-manifest.json`(この単位で作った、変えたすべてのファイル)を書く。

## ストーリーと手順の対応

| ストーリー | 受け入れ条件 | 手順 |
|---|---|---|
| US1.1 | AC1.1.1(最小の形)、AC1.1.11 | 4、5、6、10、11、13、14 |
| US2.1 | AC2.1.2、AC2.1.3、AC2.1.6(アプリ1つ)、AC2.1.7 | 3、4、5、7、8、9、11、13 |
| US3.1 | AC3.1.1 の一部(書き方 (1) のキーの名前の変更) | 3、4、8、9 |
