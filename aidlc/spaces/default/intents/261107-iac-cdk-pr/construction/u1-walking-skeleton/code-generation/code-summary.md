# コードの生成の結果: U1 最小の一本

計画(`code-generation-plan.md`)の Step 1〜16 を終えました。ブランチは `bolt/v0.2-u1-walking-skeleton`(`origin/main` の `c194b45` から)。コミット、push、プルリクエストはしていません(変更はすべて未コミット)。実際の AWS は呼んでおらず、検証用のスタックもまだ置いていません。テスト専用のリポジトリもまだありません。

テストの進め方はチームの決まり(custom)のとおりです。受け入れ条件のテスト(手順 4)を実装の前に書き、パッケージが無いためにビルドの段階で失敗することを確かめてから実装し(手順 5〜11)、実装のあとで部品ごとの単体テストを書きました(手順 12)。

## 作ったもの

| 部品 | ファイル | 役割 |
|---|---|---|
| `internal/procexec` | `procexec.go`、`tree_unix.go`、`tree_windows.go` | 外のプログラムの上限つきの起動。非対話は取り込み(既定 64 KiB)、対話は端末につなぐ。Unix はプロセスのグループ、Windows は `CREATE_SUSPENDED` のあとジョブオブジェクト(閉じると中を止める)に入れてから `NtResumeProcess`。`WaitDelay` 5 秒 |
| `internal/redact` | `redact.go` | 伏せ字の閉じた一覧(ARN、12 桁の数字、アクセスキーの ID、GitHub のトークン、URL の userinfo)と、synth と補助のプログラムの標準エラーの許す行の一覧 |
| `internal/cdk` | `app.go`、`env.go`、`synth.go`、`analyzer.go`、`private_*.go`、`helper/ts-helper.js` | アプリの発見と言語・版の判定、認証情報を外した環境、`cdk synth` と cloud assembly の読み取り、失敗の分類、埋め込んだ補助のプログラムの書き出しの守り(本人だけのフォルダー、0600、SHA-256 の確かめ)と起動、結果の版と TypeScript・Node.js の版の確かめ |
| `internal/iac` | `cdk.go`(新規)、`iac.go`、`run.go`、`target.go`、`walk.go`、`source.go`、`quote.go` | `ToolCDK`、`FileResult.Language`、`Result.CDKApps`、`Result.WrittenFiles`、CDK の理由。照合(BR2.1)、キーの箇所の数による場所の決め方、U1 の書き換えの規則(BR3.1〜BR3.3)、共有の箇所の1回だけの書き換え、Recheck(BR3.4)、synth できなかったアプリの理由(BR2.3)、終了コード |
| `internal/pullrequest` | `pullrequest.go` | Check(ネットワークを使わない確かめを先に、`gh auth status` と `git ls-remote` を次に)と Publish(`switch -c`、`add`、`commit --only`(対話)、`push`(対話)、本文のファイルと `gh pr create --body-file`)。段階ごとの上限と知らせ |
| `internal/display` | `iacpr.go`(新規)、`iac.go` | `cdk.ProgressSink` と `pullrequest.ProgressSink` の実装、行き先の1行、URL の1行、タイトルと本文、英語と日本語の文 |
| `internal/cli` | `iac.go`、`cli.go` | `--pr`(`--write` を含む)、順番(確かめ → iac → 書き出しの失敗なら止める → 本文 → Publish → URL)、`Deps.Exec`、`--help` |
| テスト用のアプリ | `internal/cdk/testdata/apps/ts-minimal/` | `aws-cdk-lib` 2.275.0、`constructs` 10.8.1、`aws-cdk` 2.1148.0、`typescript` 5.9.3、`ts-node` 10.9.2、`@types/node` 22.20.5(`package-lock.json` で固定) |
| CI | `.github/workflows/ci.yml`、`.github/workflows/release.yml`(テストの前の Node.js と `npm ci`、レビュー1回目)、`.github/dependabot.yml` | test、race、coverage に setup-node(v7.1.0 をハッシュで固定、Node.js 22.23.4、npm のキャッシュ)と `npm ci --ignore-scripts`、上限 30 分。prettier(3.9.10)と github-pr の仕事を足し、`ci-gate` が待つ。npm の Dependabot(TypeScript 6 以上は除く) |
| 検証 | `test/verify/v02-u1.sh`、`v02-u1-plan.js`、`v02-u1-rules.yaml`、`README-v02-u1.md` | 検証コマンドの台本と、スタックを置く手順、テスト専用のリポジトリにアプリを写す手順 |
| 文書 | `README.md` の `iac` の節 | `--pr`、CDK の扱い、synth に認証情報を渡さないこと、信頼できるリポジトリでだけ使うこと、残るリスク(`plan` の節には触っていない) |

テストのファイルは、`internal/cli/acceptance_v02u1_test.go`(受け入れ条件 9 本)、`github_pr_test.go`(結合テスト)、`iac_pr_test.go`、`workflow_test.go`、`export_test.go`、各パッケージの `*_test.go`、`internal/testutil/fakegh.go`(テストのバイナリ自身を `gh` として使う差し替え)、`cdkapp.go`(アプリの写しと `node_modules` のリンク、道具が無いときの扱い)です。一覧は `source-manifest.json` にあります。

## 手順 3 で確かめたこと(テスト用のアプリの synth の出力)

AWS の認証情報を外した環境(空の設定ファイル、`AWS_EC2_METADATA_DISABLED=true`)で `cdk synth --quiet --no-notices --output <一時フォルダー>` を動かし、成功しました(約 14 秒)。

- `manifest.json` の `artifacts` の `tag-refactor-cdk-skeleton` は `type: aws:cloudformation:stack`、`environment: aws://unknown-account/unknown-region`、`properties.templateFile: tag-refactor-cdk-skeleton.template.json`。
- **`properties.stackName` はありません**(スタック名が成果物の ID と同じときは書かれない)。読み取りは設計のとおり「無ければ成果物の ID」にしました。
- テンプレートのバケットの論理 ID は `Bucket83908E77`、`aws:cdk:path` は `tag-refactor-cdk-skeleton/Bucket/Resource`、`Properties.Tags` は `[{Key: Env, Value: dev}]`。ほかに `CDKMetadata`(`AWS::CDK::Metadata`、条件つき)があります。
- `Parameters` の `BootstrapVersion` と `Rules` の `CheckBootstrapVersion` はありません。
- この版の CDK は、メタデータを `tag-refactor-cdk-skeleton.metadata.json` に分けて書きます(`additionalMetadataFile`)。iac は使いません。
- 補助のプログラムの結果: スタックの作成は `bin/app.ts` の 6 行目、バケットの作成は `lib/stack.ts` の 9 行目、タグの箇所は 14 行目で、キーの範囲は `'Env'`(引用符を含む UTF-8 のバイトの位置)。

## 外への問い合わせを止める設定(契約 C5 の未決事項)

固定した CDK の CLI(`aws-cdk` 2.1148.0)の README と `lib/index.js` で確かめました。

| 設定 | 確かめた内容 |
|---|---|
| `CDK_DISABLE_VERSION_CHECK=1` | README の Environment の節に「If set, disable automatic check for newer versions」。コードでは、新しい版の確かめは `process.stdout.isTTY && !process.env.CDK_DISABLE_VERSION_CHECK` のときだけ行う。iac は標準出力を取り込むので端末でなく、この変数でも止まる(二重に止まる) |
| `--no-notices` | `argv.notices` が偽になり、お知らせ(`https://cli.cdk.dev-tools.aws.dev/notices.json`)を取りに行かない |
| `CDK_DISABLE_CLI_TELEMETRY=true` | コードは `["true", "1"]` のどちらかで利用の記録の送信を止める。README にも記載 |

どれも AWS の認証情報を伴わない問い合わせです。CLI は AI のエージェントの中で動くと「AI agent detected」と表示しますが、synth の結果は変わりません。

## 計画からの違い

| 違い | 理由 |
|---|---|
| 受け入れ条件 7(書き出しの失敗)は、iac の結果を差し替えて確かめた(`cli.SetIacForTest`) | 2つ目のファイルの書き出しをすべての OS で失敗させる方法が無いため。実際の書き出しの失敗は U8 の `internal/iac/write_test.go` が確かめている。「git と gh が呼ばれない」は、ブランチ、コミット、push、作成が無いことで確かめた(確かめの段階の `gh auth status` と読むだけの git は、書き換えの前に呼ばれる) |
| ベンチマークは `Test` の関数にした(`TestCDKHelperTwoHundredFiles`) | `go test` の既定でベンチマークは動かず、60 秒で失敗にする決まりを CI で守れないため |
| `gh pr create` に `--repo <owner/name>` を足した | リモートが複数あるとき、gh が対話で行き先を聞いて止まらないようにするため |
| `git push` は `<remote> refs/heads/<branch>:refs/heads/<branch>` で、追跡先は設定しない | 利用者のリポジトリの設定を変えないため |
| U1 で CDK の対象の `renameKey` 以外の変更(値の変更、追加、削除、マージ)と `Tags.of(x).remove` は `cdkUnsupportedShape` で一覧に出す | U1 の範囲はキーの名前の変更だけ(値の変更などは U4) |
| Prettier の確かめに `test/verify/*.js` も入れた | 検証の台本の変更案の確かめを JavaScript の小さなファイルにしたため(チームの決まり: JavaScript は Prettier) |
| 既存の `internal/cli/iac_internal_test.go` の2本(`TestIacPassesItsOptionsOn`、`TestIacIsPutTogetherWithoutAWS`)を直した | 契約 C6 と C7 で `iac.Request` に進み具合の通知口、`iac.Writer` に CDK の部品が足されたため。「AWS の接続を持たない」の確かめは残し、足した欄が CDK の通知口とインタフェースだけであることを確かめる形にした |
| `reason.templateNotFound` の文の「CDK は後の作業」は変えていない | 理由の文は U6 で整える決まりで、表示の golden のファイルにも入っているため |
| `--debug` の行に、渡した環境の変数の名前をすべて出す | 観測の設計(値は出さない)のとおり。行が長くなることがある |

## テストの結果

すべて手元(Windows 11、Go 1.27.0、`GOTOOLCHAIN=local`、`CGO_ENABLED=0`、Node.js 24.15.0)で動かしました。

| 確かめ | 命令 | 結果 |
|---|---|---|
| 整形 | `gofmt -l cmd internal scripts` | 違反なし(`node_modules` の中の Go のテンプレートの解析の誤りだけが出る。上の「残ること」) |
| vet | `go vet ./...`(Windows、`GOOS=linux`、`GOOS=darwin`、`-tags integration`) | 問題なし |
| lint | `golangci-lint run ./...`(v2.15.0、既定の linter。Windows と `GOOS=linux`) | 0 件 |
| すべてのテスト | `go test -mod=readonly -count=1 -coverpkg=./cmd/tag-refactor/...,./internal/... ./cmd/tag-refactor/... ./internal/...` | 19 パッケージすべて成功。`TestGitHubPullRequest` だけ secret が無く飛ばした |
| 受け入れ条件 | `go test -run 'TestV02U1\|TestGitHubPullRequest' ./internal/cli/` | 9 本成功(実際の synth を含む。合わせて約 50 秒)、1 本飛ばし |
| 単位の命令(レビュー1回目のあと) | `unit-test-instructions.md` の2つ目の命令 | 41 本成功、1 本飛ばし(`TestGitHubPullRequest`) |
| 全体の文のカバレッジ | 上と同じ(CI の coverage の仕事と同じ範囲) | **95.3%**(下限 80%) |
| 新しいパッケージのカバレッジ | `go test -cover ./internal/procexec/ ./internal/redact/ ./internal/cdk/... ./internal/pullrequest/` | procexec 82.1%、redact 100.0%、cdk 90.1%、pullrequest 92.0% |
| ベンチマーク | `TestCDKHelperTwoHundredFiles` | 200 ファイルで analyze 0.47 秒、recheck 0.46 秒(60 秒で失敗) |
| Prettier | `npm exec --yes prettier@3.9.10 -- --check internal/cdk/helper/ "test/verify/*.js"` | 違反なし |
| govulncheck | `go tool govulncheck ./...` | 手元の Go 1.27.0 の標準ライブラリの 9 件(1.27.2 と 1.27.3 で直っている)だけ。依存のモジュールの脆弱性で呼んでいるものは無い。CI の vuln の仕事は最新の stable の Go を使う |

足したテストは 82 本です(受け入れ条件 9、結合 1、cli 8、procexec 10、redact 6、cdk 16、iac 13、pullrequest 11、display 8。レビュー1回目で3本足した)。

## レビューへの対応(1回目)

| 指摘 | 直したところ |
|---|---|
| 1(重大): `release.yml` が Node.js と `npm ci` なしで `go test ./...` を動かし、CI では synth のテストが失敗するため、`v0.2.0` のタグの公開と試しの実行が必ず失敗する | `release.yml` の build の仕事に、`ci.yml` と同じ固定の `actions/setup-node`(Node.js 22.23.4、npm のキャッシュ、`cache-dependency-path` はテスト用のアプリの `package-lock.json`)と `npm ci --ignore-scripts --prefix internal/cdk/testdata/apps/ts-minimal` を、テストの前に足した。上限を 20 分から 30 分に上げた(`node_modules` は git の対象外なので、版に `+dirty` は付かない)。`internal/cli/workflow_test.go` に `TestEveryJobThatRunsGoTestInstallsNode` を足し、どのワークフローでも `go test` を動かす仕事は、その前に Node.js を入れて `npm ci` を動かすことを確かめる |
| 2: 対話の `git commit` と `git push` で `GIT_` の変数をすべて外すため、`GIT_SSH_COMMAND` などで認証する利用者が失敗する | `internal/pullrequest` の `gitEnv` で、コミットと push だけ、認証の変数の決まった一覧(`GIT_SSH`、`GIT_SSH_COMMAND`、`GIT_SSH_VARIANT`、`GIT_ASKPASS`)を残す。ほかの `GIT_` の変数(`GIT_DIR`、`GIT_WORK_TREE`、`GIT_INDEX_FILE`、`GIT_OBJECT_DIRECTORY`、`GIT_ALTERNATE_OBJECT_DIRECTORIES`、`GIT_CEILING_DIRECTORIES`、`GIT_NAMESPACE` など)は今までどおり外す。読むだけの git は `GIT_` の変数を1つも残さない。`SSH_ASKPASS` は `GIT_` で始まらないので、もとからそのまま渡る。単体テスト `TestGitEnvironments` を足し、README の `iac` の節の文を直した |
| 3: 既定のタイトルが 256 文字を超えると(変更案のファイル名が約 210 文字より長いと)`--pr` が止まり、U1 にはタイトルを変える方法が無い | `display.PullRequestTitle` で、伏せ字のあとのタイトルが 256 文字を超えるときは、ファイル名の始めを残して「…」を付け、全体を 256 文字に収める(切ったあとにもう一度伏せ字を通す)。利用者が書くタイトル(`--pr-title`、U2)の 256 文字の確かめは `pullrequest.Check` に残る。単体テスト `TestPullRequestTitleOfALongPlanFile` を足した |
| 4: `unit-test-instructions.md` の `-run` の式が、表示と cli に足したテストの一部(`TestPullRequest*`、`TestStaleBranches` など)を拾わない | 単位の命令を、この単位で足したテスト(と直した前のテスト3本)の接頭辞と名前を並べた式に直した。前の単位のテストに、使った接頭辞で始まるものが無いことを確かめた。この命令で 41 本が成功し、1 本(`TestGitHubPullRequest`)が飛ばされる |

## 残ること

- 検証用のスタック `tag-refactor-cdk-skeleton` を置くこと(人の明示の許可を得てから、`test/verify/README-v02-u1.md` の手順で)、テスト専用のリポジトリを作ってアプリを写すこと、CI の secret と変数を置くことは、人の作業です。
- `TestGitHubPullRequest` は secret が無いため、この作業の中では飛ばされました。
- `gofmt -l .` を手元で `npm ci` のあとに動かすと、テスト用のアプリの `node_modules` の中の Go のテンプレートの解析の誤りが標準エラーに出ます(git の対象外。CI の lint の仕事では `npm ci` をしないので出ない)。
