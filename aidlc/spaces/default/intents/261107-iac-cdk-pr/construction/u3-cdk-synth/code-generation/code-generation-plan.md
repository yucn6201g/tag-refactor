# コード生成の計画: U3 CDK の synth と対応づけ(残り)

## Summary

- Builds: `--dir` の下のすべての CDK のアプリの発見(`.venv` と `venv` に入らない)と名前の順の synth、`--cdk-app` の確かめと絞り込み、Python のアプリの版の判定と `.venv` の Python での synth(版の確かめ、`toolTooOld`、`--app` の置き換え)、`--synth-timeout`、needsAWS の文の追加、一部のアプリの失敗のあとも続ける処理と、どのアプリか分からないリソースの理由、作成の記録(`cdk synth --debug`)での補い(補助のプログラムの `from: "trace"` と Recheck)、進み具合の表と時計、`--debug` のホームのフォルダーの `~`
- Touches: 今の `internal/cdk`(新しいファイル `python.go`、`trace.go`、`home.go`)、`internal/cdk/helper/ts-helper.js`、`internal/iac`、`internal/display`(新しいファイル `iaccdk.go`)、`internal/cli`、`internal/testutil`、`internal/cdk/testdata/apps/` の8つのテスト用のアプリ、`.github/workflows/ci.yml` と `release.yml`、`.github/dependabot.yml`、`.gitignore`、README の `iac` と「開発」の節、契約の「Construction での直し」の表と U3 の設計の文書の直し
- Tests: 受け入れ条件のテスト 14 本(`TestV02U3`)を先に、実装のあとで部品ごとに 5〜8 本の単体テスト(約 70 本)と、実際の `cdk synth` を3つの OS で動かす結合テスト、ワークフローを読むテスト、-race のテスト

## 元にしたもの

- 機能の設計: `construction/u3-cdk-synth/functional-design/`(functional-spec.md の W1〜W5、rules.md の BR1.1〜BR6.1、entities.md)
- 非機能の要件: `construction/u3-cdk-synth/nfr-requirements/`(security-requirements.md の NFR1.11〜NFR1.13、NFR2.5〜NFR2.7、NFR3.11〜NFR3.17、NFR4.5、NFR5.3、NFR8.3、NFR8.4、NFR10.4、NFR10.5、tech-stack-decisions.md の NFR6.16〜NFR6.21)
- 非機能の設計: `construction/u3-cdk-synth/nfr-design/`(security-design.md の1節〜15節、logical-components.md。回答 ND3-Q1 = A、ND3-Q2 = A)
- 基盤の設計: `construction/u3-cdk-synth/infrastructure-design/cicd-pipeline.md`(回答 Q1 = A、CI の Python は `"3.13"`)
- 契約: `inception/contract-design/contract-summary.md`(X1、X2、C2、C3、C4、C5、C6、C7)
- 単位: `inception/units-generation/unit-of-work.md`(U3)
- 要件とストーリー: `inception/requirements-analysis/requirements.md`(FR2、NFR1〜NFR10)、`inception/user-stories/stories.md`(AC2.1.1〜AC2.1.7、AC4.1.3、AC4.1.4)
- U1 と U2 の設計と CI: `construction/u1-walking-skeleton/`、`construction/u2-pull-request/` の下(そのまま当てる)

## 前の段階から持ち越して、この計画で決めること

| 持ち越し | この計画での決め方 | 手順 |
|---|---|---|
| 機能の設計の Minor (1): entities.md の UnattributedManualReason の `tool` を添える条件 | 「reason が cdkAppFailed で、synth に失敗したアプリ(道具が無い、版が古い、AWS が要る、時間切れ、synth のエラー)がちょうど1つのとき」に直す。BR3.2 のとおり、synth に成功したあと補助のプログラムの失敗で failed にしたアプリは数えない(そのスタックは照合に加わっているため)。実装もこの条件にする | 10、15 |
| 機能の設計の Minor (2): 外すアプリ(other、v1、版が不明)の AppSkipped を送るのは IacWriter か Synthesize か | **Synthesize が送る**(契約 C2 のまま)。IacWriter は、外すアプリ(`App.Synthesizable()` が false)を名前の順に先に `Synthesize` に渡し(cdk は起動されず、`Synthesis.Failure` と AppSkipped だけが返る)、そのあとで synth するアプリの一覧で AppsWaiting を送り、synth するアプリを1つずつ渡す。理由: (1) 契約 C2 と今の `Synthesize` がすでにそう書かれていて、契約の形を変えずに済む。(2) 外すかどうかの判断(`skipReason`)が `internal/cdk` にあり、判断と知らせが同じ場所にあれば、IacWriter と食い違って二重に送ったり送り漏れたりしない。(3) BR1.4 の順番(外すアプリの知らせが AppsWaiting より先)は、IacWriter の呼ぶ順で守れる。(4) 表示の `pendingSkipped`(非機能の設計の9節の案 B)は、この順番を前提にしている。U1 で IacWriter が送っていた `notInU1` の AppSkipped は、`synthesizeFirst` と一緒に消える。契約の形は変わらないが、呼ぶ順番を C2 の注記として「Construction での直し」の表に1行足す | 10、15 |
| `toolTooOld`(Python が古い)を足す先 | `cdk.ReasonToolTooOld` を足し、契約 C2 の `Failure.Reason`、契約 C7 の `failure_to_manual`(`{status: failed, manual: cdkAppFailed}`、道具 python)、entities.md の SynthFailure の `allowed`、表示の `cdkWhy` と英日の文(`iac.cdkWhyToolTooOld`、`iac.cdkToolHintPythonOld`)に入れる。`internal/iac` の `failureStatus` は今の `default` の枝で `failed` と `cdkAppFailed` になるので変えない | 6、7、11、15 |
| 非機能の設計の12節「Construction での直し」 | 12節の表の 14 行を、契約の「Construction での直し」の表(C2、C3、C4、C6、C7、X1、X2 の分)と、機能の設計と非機能の要件の文書(entities.md、rules.md の BR3.2、NFR3.12、NFR6.17 の文)に、1行ずつ足すか直す。下の手順 15 に一覧を置く | 15 |
| 最初に確かめること (a): 作成の記録の実際の形(BR4.2)と needsAWS の8つの文(BR2.6) | テスト用のアプリを置いたあと、ほかの実装より先に、`ts-trace` を `cdk synth --debug` で、`ts-lookup` と `ts-profile` を認証情報を外した環境で、実際の cdk で synth し、manifest の metadata のどこに作成の記録が入るか、標準エラーにどの文が出るかを見る。結果をこの計画の「最初の確かめの結果」の節に書き、`trace.go` の読み方と needsAWS の一覧をそれに合わせる。一覧の文が合わなければ rules.md の BR2.6 を直し、「Construction での直し」の表に1行足す。`py-minimal` の記録に利用者の `.py` の行が入るかも見て、U5 のために書き残す(U3 では使わない) | 4、6、8、15 |
| 最初に確かめること (b): Windows で `cdk.cmd` に引用符つきの引数が届くか | ほかの実装より先に `TestQuotedArgReachesCmd` を書き、手元の Windows の Go で動かす。結果を「最初の確かめの結果」に書く。Go の `stable` と `oldstable` の両方の結果は、この Bolt のプルリクエストの CI の Windows の仕事で確かめる(push は人の許可を得てから)。どちらかで届かなければ、非機能の設計の4節で先に決めたとおり `windowsQuotedApp` を false にし、README の `iac` の節と残るリスクに注記し、`TestCDKPythonAppUnderSpacedFolder` を「Windows では `--app` が無く、cdk.json の app を `.venv\Scripts\python.exe app.py` にした写しで synth できる」に変える | 4、7、15、16 |
| 基盤の設計の Minor: テスト用のアプリの `__pycache__` | **`.gitignore` に `internal/cdk/testdata/apps/**/__pycache__/` を足す**。理由: `release.yml` の build は未追跡のファイルも作業の木の変更とみなして公開を止める。テストは写した先で synth する作りだが、手元で開発者が `py-minimal` をその場で synth したり、Python が `.venv` の外に `__pycache__` を書いたりする道を、どのテストが書いたかによらず1行でふさげる。go test のあとに git status が空であることを確かめる手順は、書いたことを見つけるだけで止めず、3つの OS の CI の定義を増やすので採らない | 3 |
| 手元の `.venv` | テストは `.venv` を作らない(ND3-Q1 = A)。README の「開発」の節に、`py-minimal` で動かす3行の手順を Unix と Windows(PowerShell)に分けて書く。この Bolt の作業では、手順 3 で自分がその3行を動かして手元の `.venv` を作る | 3、15 |
| CI の Python の準備 | 新しい仕事を作らず、今の `ci.yml` の `test`、`race`、`coverage` と `release.yml` の `build` に、`setup-python`(`python-version` は `"3.13"`、lint の仕事と同じハッシュ)と `py-minimal` の準備の手順を足す。`github-pr` と `ci-gate` の `needs` は変えない | 13 |
| テスト用のアプリ | `py-minimal`(Python の CDK、`requirements.txt` は `==` で固定、CDK の CLI は `package.json` と `package-lock.json` で固定)、`ts-trace`、`ts-lookup`、`ts-profile`、`ts-v1`、`py-v1`、`unknown-version`、`other-lang` を足す。どれも整形も lint もしない | 3 |
| 固定した依存に wheel の無いもの(NFR6.19) | 手順 3 で `--only-binary=:all:` で入らないパッケージがあれば、その時点で止め、そのパッケージだけを例外にする理由を書いて人に確かめる(勝手に例外を足さない) | 3 |

## 最初の確かめの結果

手順 4 で書きます(計画の承認の時点では空です)。

- 作成の記録の形(BR4.2): 手元の Windows で、`ts-trace` の写しを `ts-minimal` の `node_modules`(aws-cdk-lib 2.276.0、CDK の CLI 2.1148.1)につなぎ、認証情報を外した環境で `cdk synth --debug` を動かして確かめた。作成の記録は manifest.json の中ではなく、スタックの成果物の `additionalMetadataFile`(`<スタック>.metadata.json`)にある。このファイルはコンストラクトのパス(先頭に `/` が付く。例 `/tag-refactor-cdk-trace/Logs/Resource`)ごとの項目の配列で、作成の記録は `type` が `aws:cdk:creationStack` の項目の `data`(文字列の配列)に入る。行の形は V8 の `at ` を付けない `<名前> (<ファイル>:<行>:<桁>)`(例 `makeBucket (C:\…\ts-trace\lib\stack.ts:8:10)`)で、ファイルは実際のパスの絶対パス。`...new Bucket2 in aws-cdk-lib...`、`...node internals, ts-node, ts-node...`、`(no user code in 10 frames, …)` のような省略の行が混ざる。L2 のコンストラクト(`…/Logs`)には項目が無く、L1(`…/Logs/Resource`)の記録の最初の利用者の行が `new s3.Bucket(` の行(8 行目)になる。`trace.go` は、`additionalMetadataFile` と、古い形のための成果物の中の `metadata` の両方を読み、`aws:cdk:creationStack` の `data` と、項目の `trace` の配列の両方を見る。行は `at ` の有無を問わず、末尾の括弧の中か行の全体から `<ファイル>:<行>[:<桁>]` を取り出す。`internal/iac` は、パスの最後が `Resource` か `Default` の TraceSite を親のパスに付け替えて補助のプログラムに渡す(作成の箇所の `new` の式が作るのは親のコンストラクトで、recheck で2つ目の引数の文字列とパスの最後の要素を比べられるようにするため)。
- needsAWS の文(BR2.6): `ts-lookup`(環境を決めたスタックで `valueFromLookup`)の標準エラーは `ERROR Need to perform AWS calls for account 000000000000, but no credentials have been configured (Construct Annotations)` で、一覧の `Need to perform AWS calls` と `no credentials have been configured` に当たる。一覧の8つは直さない。`ts-profile` は、cdk.json の `"profile"` だけでは synth が終了コード 0 で通った(プロファイルが見つからなくても、CDK の CLI はアカウントを決めずに続ける。`--debug` のときだけ `Unable to determine the default AWS account (CredentialsProviderError): Could not resolve credentials using profile: [...] in configuration/credentials file(s).` が出る)。そのため `ts-profile` には、`"profile"` に加えて環境を決めたスタックの `valueFromLookup` を入れ、needsAWS になる形にした(標準エラーは `ts-lookup` と同じ文)。rules.md の BR2.3 の「cdk.json の profile の指定では synth は失敗し」は、AWS の値を引かないアプリには当たらない(報告に書く。計画の手順 15 の範囲の外なので文書は直さない)。
- `py-minimal` の記録の `.py` の行(U5 のため): 入る。`__init__ (C:\…\py_minimal\py_minimal_stack.py:15)` と `<module> (C:\…\app.py:13)` の形で、桁が無い(`<ファイル>:<行>`)。先頭に `...jsii runtime, node internals...` と `(no user code in 10 frames, …)` が付く。U3 では使わない。
- `TestQuotedArgReachesCmd`(手元の Windows の Go): Go 1.27.0(windows/amd64)で届かない。`a b` のフォルダーの `cdk.cmd` に、引用符つきの `--app` を `procexec.System` で渡すと、cmd.exe が `'C:\…\a' is not recognized as an internal or external command` で終了コード 1 になる(Go が `.cmd` を cmd.exe で動かすときに、パスの空白と引数の中の引用符が重なると、cmd.exe が先頭と末尾の引用符を外す)。空白の無いフォルダーの `cdk.cmd` でも、Go は中の引用符を `\"` にして渡すので、`%*` は渡したものと同じにはならない(実際の `cdk.cmd` から node には `"…" app.py` の形で届く)。CI の `stable` と `oldstable`: `windowsQuotedApp` を false にしたため、このテストは CI でも理由を出して飛ばされ、2つの版の結果は CI では得られない(手順 16 で、この点を報告に書く)。
- `windowsQuotedApp`: false。持ち越しの表で先に決めたとおり、Windows では空白か引用符の外の特別な文字を含む Python のパスで `--app` を置き換えず、PATH の先頭の `.venv` だけに頼る。README の `iac` の節と残るリスクに注記し、`TestCDKPythonAppUnderSpacedFolder` は Windows では `--app` が無く、cdk.json の app を `.venv\Scripts\python.exe app.py` にした写しで synth できることを確かめる形にする。

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

## 触らないもの

- README の `plan` の節、`internal/apply`、`scripts/`、`internal/iac/testdata`(ほかのセッションが触っている)
- 前の intent の記録(`aidlc/spaces/default/intents/261019-tag-refactor-mvp/` の下。未コミットの変更があっても、触らず、ステージもしない)
- `.agents/`、`.codex/`、`AGENTS.md`(自分の変更ではない)
- `internal/procexec` のコード(版の確かめと Python の synth も今の起動の口を使う)と、`cdk.ProgressSink` の5つのメソッド(契約の決まり R-04)
- 補助のプログラムの JSON の形と `HelperVersion`(`from: "trace"` は契約 C3 と C4 にすでにある値)

## 手順

チームの進め方(custom)に従い、受け入れ条件のテストを先に書き(手順 5)、実装(手順 6〜13)、実装のあとで細かい単体テスト(手順 14)、すべてのテスト(手順 16)の順に進めます。手順 3 と 4 は、受け入れ条件のテストと実装の前提になるテスト用のアプリと、設計が「最初に確かめる」と決めた2つの確かめです。

### 構成と準備

- [x] **Step 1: 作業ブランチ。** U2 のプルリクエストが `main` に squash マージされていることを確かめる(入っていなければ人に知らせて待つ)。計画の承認のあとで、このフォルダーで `git fetch origin` を実行し、`git switch -c bolt/v0.2-u3-cdk-synth origin/main` を実行する(このブランチの切り替えは、この計画の承認をもって人の許可とする)。`aidlc/` の下の未コミットの U3 の記録(状態、監査、日誌、`construction/u3-cdk-synth/` の下)はそのまま持ち越す。切り替えのあと `git status` で、記録と自分のものでない変更(`.agents/`、`.codex/`、`AGENTS.md`、`261019-tag-refactor-mvp/` の下)だけが残ることを確かめる。自分のものでない変更には触らず、ステージもしない。上の「触らないもの」を守る。(全体)
- [x] **Step 2: テストの道具の確かめ。** `GOTOOLCHAIN=local CGO_ENABLED=0 go test ./internal/cdk/ ./internal/iac/ ./internal/display/ ./internal/cli/ ./internal/testutil/` が `origin/main` のままで通ることを確かめ、テストの数と失敗の数を書き留める(変更の前の基準)。`unit-test-instructions.md` のこの Unit の命令を記録する。手元の Node.js(`node --version`)、`ts-minimal` の `node_modules/.bin/cdk.cmd` とその版、Python(`py --version` と `python3` が Microsoft Store の案内の偽物でないか)、`pip`、Go の版を確かめて書き留める。どれかが無ければ、手元で飛ばされるテストの範囲を書き留める。(全体)
- [x] **Step 3: テスト用のアプリと `.gitignore`。** `internal/cdk/testdata/apps/` に次を足す。どれも整形も lint もしない(チームの進め方)。
  - `py-minimal`: `cdk init app --language python` の形の最小のアプリ(cdk.json の app は `python3 app.py`、スタックは1つ、リソースは S3 バケット1つとスタックのタグ)。きれいな `.venv` に `aws-cdk-lib` と `constructs` を `--only-binary=:all:` で入れ、`pip freeze` の結果を `requirements.txt` に書く(すべての行が `==`)。`aws-cdk` だけを書いた `package.json` と `package-lock.json`(版は `ts-minimal` の `aws-cdk` と同じ)。wheel の無いパッケージがあればここで止め、例外にする理由を書いて人に確かめる(NFR6.19)。
  - `ts-trace`: スタックのクラスの外の関数 `makeBucket(scope)` の中で `new s3.Bucket(scope, 'Logs', …)` を作り、スタックのコンストラクターで `makeBucket(this)` と `Tags.of(this).add('Owner', 'team-a')` を呼ぶ。スタックのリソースはこのバケットだけ。`package.json` は `ts-minimal` と同じ依存の名前だけを持ち、依存は入れない。
  - `ts-lookup`(架空の 12 桁のアカウントとリージョンを決めたスタックで `ssm.StringParameter.valueFromLookup`、`cdk.context.json` は置かない)、`ts-profile`(cdk.json に `"profile"`)、`ts-v1`(`@aws-cdk/core`)、`py-v1`(`requirements.txt` に `aws-cdk.core`)、`unknown-version`(CDK の依存の無い package.json)、`other-lang`(app が `mvn -e -q compile exec:java`)。どれも依存を入れない。
  - `.gitignore` の今の `node_modules/` と `cdk.out/` の行の隣に、`internal/cdk/testdata/apps/*/.venv/` と `internal/cdk/testdata/apps/**/__pycache__/` を足す(持ち越しの表のとおり)。
  - README の「開発」の節の3行(手順 15 で書く)と同じ手順で、手元の `py-minimal` に `.venv` と `node_modules` を作る。テストのコードは `.venv` を作らない。
  - `git status` で、足したアプリのファイルだけが出て、`.venv`、`node_modules`、`__pycache__` が出ないことを確かめる。
  - (BR6.1、NFR6.18、NFR8.3、NFR8.4)
- [x] **Step 4: 最初の確かめ(a)と(b)。** ほかの実装より先に行い、結果を上の「最初の確かめの結果」の節に書く。
  - (a) `ts-trace` を `testutil.CopyCDKApp` と同じ形で一時フォルダーに写し、`ts-minimal` の `node_modules` をつないで、`cdk synth --debug --output <一時フォルダー>` を動かし、`manifest.json` の `metadata` の項目(`aws:cdk:logicalId` などの `trace` の配列か)のどこに、どの書き方(V8 のスタックの行の形か)で作成の記録が入るかを見る。`ts-lookup` と `ts-profile` を、AWS_ で始まる変数を外し、`AWS_SHARED_CREDENTIALS_FILE` と `AWS_CONFIG_FILE` を空の一時ファイルに向けた環境で synth し、標準エラーの文を BR2.6 の8つと照らす。`py-minimal` を `cdk synth --debug` で synth し、記録に利用者の `.py` の行が入るかを見る(U5 のため)。どれも AWS の認証情報を渡さず、AWS は呼ばない。
  - (b) `internal/cdk` に Windows だけで動く `TestQuotedArgReachesCmd` を書く(非機能の設計の4節: `t.TempDir()` の下の `a b` に、受け取った `%*` をファイルに書く `cdk.cmd` を置き、`procexec.System` で引用符つきの `--app` を渡し、書かれた引数が引用符を含めて同じことを確かめる。`os/exec` の起動の前の誤りも「届かない」とみなす)。手元の Windows で動かす。
  - 結果に合わせて、この計画のあとの手順(手順 6 の needsAWS の一覧、手順 8 の `trace.go` の読み方、手順 7 の `windowsQuotedApp`)を進める。設計の前提が崩れる結果(記録がクラウドアセンブリに残らないなど)なら、実装に進まずに人に知らせる。
  - (BR2.6、BR4.2、NFR6.17、契約 C2 の未決事項)

### 受け入れ条件のテスト(先に書く)

- [x] **Step 5: 受け入れ条件のテストを先に書く。** `internal/cli/acceptance_v02u3_test.go` を書き、この時点では失敗する(または、まだ無いオプションでコンパイルできない部分は、テストの本体を書いたうえで失敗する)ことを確かめる。各テストの名前は `TestV02U3` で始める。U1 の受け入れテストと同じく、実際の cdk で synth するテストは `testutil.CopyCDKApp` で写した先で動かし、Python のアプリは `testutil.RequirePythonTools`、TypeScript のアプリは `testutil.RequireCDKTools` で道具を確かめる(CI では失敗、手元では理由を出して飛ばす)。失敗の形は、`PATH` の先頭に置く偽の `cdk` と偽の `python`(テストのバイナリ)で作る。AWS は呼ばず、変更案はテストの中で作った JSON を使う。
  1. AC2.1.1(US2.1): `--dir` の下に `ts-minimal` と `py-minimal` の写しと、`node_modules` と `.venv` の中の `cdk.json` がある。2つのアプリが名前の順に synth され、`node_modules` と `.venv` の中は対象にならない。`--cdk-app` で1つを指すと、もう1つは synth されず、アプリの状態の一覧にも出ない。
  2. AC2.1.1 の入力の誤り(BR1.2、NFR2.7): `--cdk-app` に `../other`、`a/../../other`、`node_modules/x`、`.venv/x`、リンクを経由するパス(Unix はシンボリックリンク、Windows はジャンクション)、`/etc`、`C:\x`、`C:`、`infra/none`(Windows では `..\other` も)で、終了コード 1、偽の cdk の起動0回、ソースが変わらない。変更案に stack-name のタグのリソースが無くても `--cdk-app infra/none` は終了コード 1。Windows で `Infra/TS` が `infra/ts` に一致する。
  3. AC2.1.2(US2.1、BR5.2): 端末でない標準エラーに、アプリごとの `待ち`、`実行中`、`済み`(経過時間つき)、`失敗`(理由の種類つき)の行が、名前の順に出る(`--lang ja` と `--lang en` の両方)。
  4. AC2.1.3(US2.1、BR2.3、BR2.6、NFR8.4): 偽の `AWS_ACCESS_KEY_ID` と `AWS_PROFILE` を入れた環境で、`ts-lookup` と `ts-profile` が needsAWS の失敗になり、対処(一度 AWS の認証情報のある端末で synth して `cdk.context.json` に保存する)が出て、終了コード 2。偽の cdk で、渡った環境に AWS_ で始まる変数が無い。
  5. AC2.1.4(US2.1、NFR3.11): 応答しない偽の cdk に `--synth-timeout 1s` で、約1秒で止まり、そのアプリが時間切れの失敗になり、孫のプロセスが残らない。`--synth-timeout` の `0s`、`999ms`、`10`、`abc`、`-5s` は、どのファイルも変えず、git も gh も cdk も呼ばずに終了コード 1。
  6. AC2.1.5(US2.1、BR2.5、NFR4.5): アプリの `cdk.out` に目印のファイルを置き、一時フォルダーをテストのフォルダーに向けて(`TMPDIR`、`TEMP`、`TMP` を `t.Setenv`)、成功、失敗、時間切れ、synth の途中の中断のどれでも、`cdk.out` が変わらず、一時フォルダーに何も残らない。中断ではどのファイルも書き換えず終了コード 1。
  7. AC2.1.6(US2.1、BR3.1): 2つのアプリのスタックと照合し、それぞれのアプリのリソースのキー名の変更が反映される。2つのアプリに同じスタック名と論理 ID があると、書き換えずに cdkAmbiguous で手で直す一覧に出る。
  8. AC2.1.6、AC4.1.4(BR4.1〜BR4.4、NFR3.13): `ts-trace` の `Owner` のキー名の変更で、実際の synth、trace の synth、本物の補助のプログラムを通して、スタックの `Tags.of(this)` の行が書き換えられ、cdkNoSource にも cdkUnsupportedShape にもならない。
  9. AC2.1.7(US2.1、NFR8.3): `py-minimal` の `.venv` が無いとき、環境の変数 `CI` があれば失敗、無ければ理由(README の「開発」の節を指す)を出して飛ばす(U1 の `TestV02U1MissingToolsFailInCIAndSkipLocally` と同じ形)。
  10. AC4.1.3(US4.1、BR2.1、BR2.7、BR3.2): `py-minimal`(`.venv` なし、`PATH` に `python3` なし)と `ts-minimal` の写し。Python のアプリが `失敗(道具が無い: python。Python 3 を入れてください)` になり、TypeScript のアプリの変更は反映され、どのアプリにも結び付かずテンプレートにも無いリソースは cdkAppFailed(道具 python)で手で直す一覧に出て、終了コード 2。`.venv` の偽の `python` が `Python 3.8.18` と答えると、`toolTooOld` で `Python 3.9 以上を入れてください` が出る。
  11. AC4.1.4(US4.1、BR1.3、BR3.2、BR3.3): `ts-v1`、`py-v1`、`unknown-version`、`other-lang` は synth されずに `外した` になり、どのアプリにも結び付かないリソースの理由が cdkV1(synth に失敗したアプリが無いとき)、cdkUnsupportedLanguage(`other-lang` だけのとき)になる。`py-minimal` に結び付いた変更は、書き換えずに cdkUnsupportedShape で手で直す一覧に出る(アプリの状態は済み)。
  12. NFR1.11、NFR1.12、NFR1.13: 偽の cdk が書く作成の記録と標準エラーに、Unix の `/home/alice/...` と Windows の `C:\Users\alice\...` の形のパス、ARN、12 桁の数字、GitHub のトークンの形(テストの中で組み立てる)を入れる。`--pr` の本文、コミットのメッセージ、`--debug` なしの標準エラーと標準出力に `alice`、ARN、12 桁の数字、トークンが出ず、`--debug` の行ではホームのフォルダーが `~` になる。
  13. NFR10.5: `--lang ja` と `--lang en` で、Python が無いアプリ、古い Python のアプリ、`--cdk-app` と `--synth-timeout` の誤り、needsAWS のアプリの文がそれぞれの言語で出る。
  14. NFR5.3: 変更案に stack-name のタグのリソースが無く `--cdk-app` も無いと、発見も synth もされず、出力と終了コードが今の `iac` と同じ。`--cdk-app` も `--synth-timeout` も無い、アプリが1つの TypeScript のリポジトリの標準エラーの行が U1 と同じ。

### 実装

- [x] **Step 6: `internal/cdk` の発見、版の判定、型。**
  - `skippedFolders` に `.venv` と `venv` を足し、`discover` の `os.ReadDir` を変数 `readDir` にする(NFR3.15)。
  - `newApp` に Python の版の判定を足す(BR1.3 の表: `requirements.txt`、`setup.py`、`pyproject.toml` を 1 MiB まで読み、依存の名前を小文字にして `-`、`_`、`.` の並びを `-` にそろえ、`aws-cdk-lib` は 2、`aws-cdk-core` は 1)。`skipReason` と `Synthesizable` を、Python の v2 も synth するように直す。
  - `ReasonToolTooOld`、`Options.WithTrace`(`cdk synth` に `--debug` を足す)、`Synthesis.TraceSites`、`Config.TempDir`、`Config.Home`、`Synthesizer.VersionTimeout` を足す。`isolate(parent, tempDir)`、`privateDir(base)`、`os.MkdirTemp(cfg.TempDir, …)` にする。
  - needsAWS の文を、手順 4 の結果に合わせて BR2.6 の一覧にする(大文字と小文字を区別した部分一致、`Profile` と `not found` が同じ行)。
  - `Synthesize` が、失敗のときに `sink` が `ToolFailureSink` を満たせば `AppToolFailed`、そうでなければ `AppFailed` を送るようにする。外すアプリの AppSkipped は今のまま `Synthesize` が送る(持ち越しの表)。
  - (BR1.1、BR1.3、BR2.4、BR2.6、NFR3.15、契約 C2、C6)
- [x] **Step 7: `internal/cdk` の `python.go`。**
  - `interpreter`、`venvPython`(アプリの直下の `.venv` だけ、`Mode().IsRegular()`)、`firstWord`、`isPythonWord`、`pythonFor`、`toolName`(最後の要素、`Sanitize`、64 文字)、`withVenv`(PATH の変数を1つにし先頭に `.venv` の実行ファイルのフォルダー、VIRTUAL_ENV を置き換える)を、非機能の設計の2節のとおりに作る。
  - `checkPython`、`MinPythonVersion`、`minPython = {3, 9}`、`pythonVersionLimit`(30 秒)、`pythonVersion` の正規表現を、3節の分け方の表のとおりに作る(読めない答えと終了コード 0 以外は `toolMissing`、古ければ `toolTooOld`、時間切れは `timeout` で道具 python)。
  - `appCommand` と `windowsQuotedApp`(手順 4 の結果の値)を4節のとおりに作る(シェルが読み替える文字があれば置き換えない、空白と引用符の外の特別な文字があれば二重引用符で囲む)。
  - `Synthesizer.run` の順番を3節のとおりにする: node、cdk、python の確かめ → `isolate` と `withVenv` → `checkPython` → 出力先 → `cdk synth`(`--app` と `--debug` を足す)。版の確かめ、normal、trace のすべてに同じ環境を使う。
  - `--debug` の行(synth の命令、`.venv` の行、版の確かめの答え)を5節の形で足す(`shortHome` を通す)。
  - (BR2.1、BR2.2、NFR2.5、NFR2.6、NFR3.14、NFR6.16、NFR6.17)
- [x] **Step 8: `internal/cdk` の `home.go` と `trace.go`。**
  - `homeForms` と `shortHome` を5節のとおりに作る(長いものから比べる、直後が英数字、`_`、`-`、`.` なら置き換えない、Windows は大文字と小文字を区別しない)。`keptLines(stderr, debug, prefix, homes)` にし、`Synthesis.Debug` と `Analysis.Debug` のすべての行に通す。
  - `readTraceSites`、`traceFrame`、`userFile` を、手順 4 で見た記録の形に合わせて作る。アプリの外と `node_modules`、`.venv`、`venv`、`site-packages` の下を捨て、パスごとに最初の行だけを使い、`ConstructPath` の順に並べる。絶対パスは関数の外に出さず、`--debug` には数だけを出す。`run` が出力先を消す前に、`WithTrace` のときだけ呼ぶ。
  - (BR4.2、NFR1.11、NFR1.12)
- [x] **Step 9: Analyzer と補助のプログラム。**
  - `Analyzer.Recheck(ctx, app, edited, traces []TraceSite)` にし、要求の `traceSites` に入れる。`internal/iac` の analyzer のインタフェースと、テストの差し替えを含むすべての実装を同じ変更で直す(R-04)。
  - `ts-helper.js` に `traceSites` と `traceNew` を足す: ソースで見つからないパスだけ、記録の行の `new` の式(開始の行が同じ、いちばん外側)を `from: "trace"` の作成の箇所にする。recheck のときだけ、2つ目の引数の文字列がパスの最後の要素と同じで行が最も近い式を探し直す。JSON の形と `HelperVersion` は変えない。
  - (BR4.4、契約 C3 の construct_rules、C4)
- [x] **Step 10: `internal/iac` の IacWriter。**
  - `Request.CDKApps` と `Request.SynthTimeout` を足す。`selectApps` を作り、`Run` の始め(`loadPlan` と `readTFState` のあと)で、`CDKApps` が空でなく synthesizer があるときだけ `Discover` と `selectApps` を呼ぶ。`runner.apps` と `runner.discovered` を足す。
  - `prepareCDK` を複数のアプリの処理に書き直す: 外すアプリを名前の順に先に `Synthesize` に渡す(AppSkipped は Synthesize が送る) → synth するアプリの一覧で `AppsWaiting` → 1つずつ synth(`cdk.Options{Timeout: req.SynthTimeout}`) → 成功したすべてのアプリと照合 → アプリごとに、typescript は Analyze、python は結び付いた変更を cdkUnsupportedShape → 作成の箇所が見つからないリソースがあれば `traceSink` で trace の synth、`sameStacks` が真なら `TraceSites` を付けて2回目の Analyze、`cdkRun.traces` → 書き換えのあるアプリごとに Recheck(`run.traces` を渡す)。
  - `compareAnalyses` の作成の箇所の比べ方を `sameConstructSites` にする(`trace` はパスとファイルだけ)。
  - どのアプリか分からないリソースの理由を BR3.2 の優先で決め、道具の名前は synth に失敗したアプリがちょうど1つのときだけ添える(持ち越しの表)。
  - `ReasonNotInU1`、`runner.synthesizeFirst`、`Result.ExitCode` の `notInU1` の条件を消す。`CDKApp.Elapsed` は normal と trace の合計。中断は書き換えずにそのまま返す。
  - (BR1.2、BR1.4、BR2.7、BR3.1〜BR3.3、BR4.1、BR4.3、BR4.4、BR5.4、NFR3.13、NFR3.17、契約 C7)
- [x] **Step 11: `internal/display` の進み具合と文。**
  - `iaccdk.go` と `iacCDKCatalog`(非機能の設計の11節の文と、9節の表の見出し、足、状態の語)を英語と日本語で足し、`init` で `catalog` に足す。`iac.cdkWhyNotTypeScript` の文を直し、`iac.cdkWhyNotInU1` と `cdkWhy` の `notInU1` を消し、`toolTooOld` を足す。
  - アプリの表(`progress.apps`、`byApp`、`appRow`)、`pendingSkipped` と `skippedApp`、表に無いアプリの通知の無視と `--debug` の1行、時計(`Ticker`、`SetTicker`、`appClock`、`startClock`、`stopClock`、`tick`、`clockWG`)、`AppToolFailed`(`var _ cdk.ToolFailureSink = (*Printer)(nil)`)、`cdkElapsed` を9節と10節のとおりに作る。`endProgress` で時計を止め、`Close` で時計のゴルーチンを待つ。
  - 道具の入れ方の文は、理由が `toolMissing` か `toolTooOld` で道具が `node`、`cdk`、`python` のときだけ添え、ほかの道具の名前では `iac.cdkToolNotFound` だけを出す。
  - 1行ずつの表示の文と、1 分未満の経過時間の書き方は U1 のまま(NFR5.3)。
  - (BR5.1〜BR5.4、NFR10.4、NFR10.5、NFR1.13)
- [x] **Step 12: `internal/cli` の `iac`。**
  - `--cdk-app`(`StringArrayVar`、コンマで分けない)と `--synth-timeout`(`StringVar`、既定 `10m`)を足す。`parseSynthTimeout`(`time.ParseDuration`、1 秒より短い値と読めない値は `iac.badSynthTimeout` の入力の誤り)を、`--plan` と `--dir` の確かめの直後、`checkPullRequest` の前に呼ぶ。
  - `iac.Request` に2つの値を入れる。`iacLong` の CDK の段落を、すべてのアプリ、Python のアプリ、`--cdk-app`、`--synth-timeout`、信頼できるリポジトリでだけ使うことの説明に直す。
  - (契約 X1、NFR6.21)
- [x] **Step 13: 環境とビルドの設定。**
  - `ci.yml`: 最上位の `env` に `PY_TEST_APP` と `PYTHON_VERSION: "3.13"` を足す。`test`、`race`、`coverage` の `setup-node` の `cache-dependency-path` を2行にし、`Install the test CDK app` の後ろ、go test の前に、`actions/setup-python`(lint の仕事と同じコミットのハッシュ)と `Install the Python test CDK app`(`python -m venv .venv`、`--only-binary=:all:`、`pip check`、`npm ci --ignore-scripts --no-audit --no-fund`)を足す。`github-pr`、`lint`、`ci-gate` の `needs` は変えない。
  - `release.yml`: 最上位の `env` と `build` の仕事に同じものを足す(`Require Go ... or later` の前)。
  - `.github/dependabot.yml`: `py-minimal` の `pip`(`groups` で1つにまとめる、名前 `py-minimal-pip`)と `npm` を、毎週、`chore(deps)` で足す。
  - どのワークフローにも AWS の認証情報と新しい secret を足さない。
  - `testutil`: `RequirePythonTools`、`RequirePythonToolsWith`、`ProcessAlive`(今の `internal/procexec` のテストと同じ見方。procexec のコードは変えない)を足し、`CopyCDKApp` が `.venv` と `venv` を写さず、`.venv` をリンクでつなぐようにする。
  - (NFR6.18、NFR6.19、NFR8.3、NFR8.4、cicd-pipeline.md の2節)

### 単体テスト(実装のあと)

- [x] **Step 14: 部品ごとの単体テスト(各 5〜8 本)。**
  - `internal/cdk` の発見と版: `.venv` と `venv` の下を `readDir` が一度も読まない、`Discover` が呼ばれない場合、Python の3つのファイルの読み方の表(`-r`、`://`、`[extras]`、`;`、`@`、poetry の表、`aws_cdk.core`)、1 MiB を超えるファイル、`py-minimal/requirements.txt` のすべての行が `==`(`TestPyMinimalPins`)。
  - `python.go`(`TestPython…`): `withVenv` の表(Windows の形と Unix の形、`Path` と `PATH`、PATH なし、先頭がすでに `.venv`、VIRTUAL_ENV の置き換え)、`pythonFor` の表(`python3 app.py`、`pipenv run python app.py`、絶対パスの `pipenv`、親のフォルダーだけの `.venv`)、`checkPython` の答えの表(`Python 3.8.18`、`3.9.0`、`3.14.5`、標準エラーの `3.9.0`、読めない答え、終了コード 1、`ErrTimeout`、失敗のとき cdk の `Run` を呼ばない)、`appCommand` の表(空白、`$`、`%`、`"`、`&`、`(`、`.exe`、`PYTHON3`、`.venv/bin/python app.py`、タブ)、差し替えた実行の口で版の確かめ、normal、trace の環境、`TestMinPythonMatchesTheTestApp`。
  - `home.go` と `trace.go`(`TestHome…`、`TestTrace…`): `shortHome` の境目の表、`homeForms`、Unix の実際のリンク、`traceFrame` と `userFile` の表(両方の OS の形、`file://`、アプリの外、4つのフォルダー、行番号 0)、`readTraceSites` の並びと捨てた数、読めない manifest。
  - 実際の synth と止め方(`TestCDK…`): `TestMain` の偽物(`version`、`hang`、`grandchild`)で、版の確かめと synth の時間切れと中断のあとに孫が残らない(3つの OS)、後片付け(成功、失敗、時間切れ、Analyze と Recheck の途中の中断で `Config.TempDir` が空、`cdk.out` が変わらない)、`py-minimal` の実際の synth と `.venv` の中身が変わらないこと、`TestCDKPythonAppUnderSpacedFolder`(空白のフォルダー、Unix の `$` のフォルダーでは `--app` が無い)、`ts-lookup` と `ts-profile` の needsAWS(偽の AWS の変数が `Command.Env` に無い)、`ToolFailureSink` の有無で送り先が変わる。
  - 補助のプログラム(`TestCDK…`、`ts-trace` を写して本物を動かす): 非機能の設計の5節の (a)〜(e)。
  - `internal/iac`: `selectApps` の表(Windows の大文字と小文字、重なり、`..`、絶対パス、synthesizer の無い `Writer`)、起動の回数(trace を使わない、使う、trace が失敗、スタックが違う、python のアプリ)と Recheck に渡る `traces`、`sameConstructSites` と `sameStacks` の表、`traceSink` が失敗を済みに読み替え `ToolFailureSink` を満たさないこと、BR3.2 の理由と道具の名前(synth に失敗したアプリが1つ、2つ、補助のプログラムの失敗だけ)、外すアプリの AppSkipped が AppsWaiting より先に来ること。U1 の `TestCDKFailedAppsAndTheExitCode` の `the second app` と analyze の回数のテストは、これらに置き換える(NFR5.3)。
  - `internal/display`(`TestCDK…`): 非機能の設計の9節の (a)〜(h)、-race のテスト(時計、知らせ、`Error` と `DebugLine` を同時に動かす)、道具の入れ方の文(`pipenv` では入れ方の文が出ない)、`cdkElapsed`、新しい文のカタログのキーの抜け。`iacpr_test.go` の `ReasonNotInU1` の行を消す。
  - `internal/cli`: `parseSynthTimeout` の表(`90s`、`10m`、`1h30m` が通り、`0s`、`999ms`、`10`、`abc`、`-5s` が誤り)、ワークフローを読むテスト(`TestEveryJobThatRunsAllGoTestsInstallsPython`、`TestEveryJobThatRunsGoTestInstallsNode` の2つの lock、Python の版が lint の仕事と同じ、アクションのハッシュ、AWS と秘密の値、`ci-gate` の `needs`、Dependabot の `py-minimal`)。
  - 各テストは正しい場合と、少なくとも2つの誤りか境目の場合を含める。

### 文書

- [x] **Step 15: README と、契約と設計の文書の直し。**
  - README の `iac` の節: すべてのアプリを名前の順に synth すること、Python のアプリ(`.venv` の Python を先に使う、最小の版 3.9)、`--cdk-app`、`--synth-timeout`、進み具合の表、作成の記録での補い、外すアプリと失敗の扱い、全体の時間の見積もりの式(非機能の設計の8節)、残るリスクの回避(cdk.json の app を `.venv\Scripts\python.exe app.py` のように書く)、信頼できるリポジトリでだけ使うこと(Python のパッケージも動く)。`windowsQuotedApp` を false にした場合はその注記。`plan` の節には触らない。
  - README の「開発」の節: `py-minimal` の `.venv` を作る3行(Unix と PowerShell)と、無いときは手元でテストが飛ばされ CI では失敗すること。
  - 契約の「Construction での直し」の表に、次を1行ずつ足す(非機能の設計の12節と持ち越しの表のとおり): C2 の `Failure.Reason` の `toolTooOld`、C2 の `Options.WithTrace` と `Synthesis.TraceSites` の実装、C2 の `Failure.Tool` の `toolName`、C2 の注記(外すアプリは IacWriter が AppsWaiting の前に Synthesize に渡し、AppSkipped は Synthesize が送る)、C2 の needsAWS の一覧(手順 4 で直したとき)、C3 の `Recheck` の `traces` と `construct_rules`、C4 の `traceSites` を recheck でも送ること、C6 の任意の口 `cdk.ToolFailureSink`、C7 の `failure_to_manual` の `toolTooOld`、X1 の `--cdk-app` の誤りの書き方、X2 の `resource_name`。
  - 設計の文書の直し: entities.md の SynthFailure(`toolTooOld`、`tool` の説明)と UnattributedManualReason の `tool` の条件、rules.md の BR3.2 の 1 の道具の名前の例(`toolName`)、BR2.6 の一覧(手順 4 で直したとき)、security-requirements.md の NFR3.12 の式、tech-stack-decisions.md の NFR6.17 の文(引用符の外の特別な文字、`windowsQuotedApp` の切り替え)。表示の対応表と文の一覧は手順 11 のコードが元。
  - (契約の決まり: X1 と X2 を変えるときは README を同じプルリクエストで直す)

### 確かめと記録

- [ ] **Step 16: すべてのテストと確かめ。**
  - `gofmt -l .` が空、`go vet ./...`、`golangci-lint run`(手元にあれば)、`GOTOOLCHAIN=local CGO_ENABLED=0 go test ./...` を通す。
  - カバレッジ 80% 以上を確かめる(この Unit で直したパッケージごとと、`./cmd/tag-refactor/...` と `./internal/...` の全体)。`govulncheck ./...` を通す。
  - 構成の検証コマンド(`test/verify/v02-u1.sh`、人が承認したもの)で、端から端までの流れ(実際の AWS から `plan`、`iac --pr` でテスト専用のリポジトリにプルリクエスト)が壊れていないことを確かめる。AWS の認証情報と少額の費用、手元の Node.js、CDK の CLI、`gh` が要る。
  - push とプルリクエストの作成は、人の指示を得てから行う。プルリクエストの CI で、3つの OS の `py-minimal` の synth、Windows の `stable` と `oldstable` の `TestQuotedArgReachesCmd`、`release.yml` の下書きの build(作業の木の確かめ)が通ることを確かめ、`TestQuotedArgReachesCmd` の結果を「最初の確かめの結果」に書く。届かなければ持ち越しの表のとおり切り替える。CI の時間を測り、基盤の設計の5節の見直しの目安(仕事 20 分、パッケージ 6 分)を超えていないかを見る。
  - 失敗は直してから次に進む。下限は下げない。
- [x] **Step 17: 記録。** `code-summary.md`、`traceability.json`、`source-manifest.json`(この Unit で作った、変えたすべてのファイル。テスト用のアプリ、CI の定義、README、契約と設計の文書を含む)を書く。

## ストーリーと手順の対応

| ストーリー | 受け入れ条件 | 手順 |
|---|---|---|
| US2.1 | AC2.1.1 | 5(1、2)、6、10、12 |
| US2.1 | AC2.1.2 | 5(3)、11 |
| US2.1 | AC2.1.3 | 4、5(4)、6、7 |
| US2.1 | AC2.1.4 | 5(5)、7、10、12 |
| US2.1 | AC2.1.5 | 5(6)、6、10 |
| US2.1 | AC2.1.6 | 4、5(7、8)、8、9、10 |
| US2.1 | AC2.1.7 | 3、5(9)、13 |
| US4.1 | AC4.1.3 | 5(10)、6、7、10、11 |
| US4.1 | AC4.1.4 | 5(8、11)、6、8、9、10 |
| 非機能 | NFR1.11〜NFR1.13、NFR2.5〜NFR2.7、NFR3.11〜NFR3.17、NFR4.5、NFR5.3、NFR6.16〜NFR6.21、NFR8.3、NFR8.4、NFR10.4、NFR10.5 | 4、5(2、4〜6、12〜14)、6〜13、14 |
