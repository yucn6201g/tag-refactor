# 開発者によるコードの調査(Reverse Engineering)

対象: リポジトリ `github.com/yucn6201g/tag-refactor`(コミット `9bf1fb6b83d5745e197116eb7a6392043493e1dd`、調査日 2026-11-07)。初めての全体の調査で、既存のコード知識ベースはありません。

## Developer Code Scan Results

### Scan Coverage
- **Analyzed deeply**:
  - `internal/iac/`(テスト以外のすべてのソース: `doc.go`、`iac.go`、`run.go`、`target.go`、`tfstate.go`、`walk.go`、`source.go`、`read_yaml.go`、`read_hcl.go`、`changes.go`、`edit.go`、`quote.go`、`diff.go`、`git.go`。`read_json.go` と `depth.go` は関数の単位で確認。テストは関数名で全件を確認)
  - `internal/cli/`(`cli.go`、`commands.go`、`iac.go` は全文。`apply.go`、`rollback.go`、`suggest.go`、`messages.go` はオプションの定義と関数の単位で確認)
  - `cmd/tag-refactor/`(`main.go` とシグナルの扱い)
  - `internal/planner/`(`planner.go` の型と、保存した変更案の形式を決める `saved.go` の書き出しと読み込み)
  - `internal/restype/`、`internal/errkind/`、`internal/fileout/replace.go`
  - `internal/display/iac.go`(iac の出力とメッセージの一覧)
  - `README.md`、`go.mod`、`.gitignore`、`.gitattributes`、`.github/workflows/ci.yml`、`.github/workflows/scheduled.yml`、`.github/dependabot.yml`
- **Skimmed only**:
  - `internal/apply/`、`internal/journal/`、`internal/awsaccess/`、`internal/inventory/`、`internal/impact/`、`internal/rules/`(パッケージの説明、公開の型と関数、主な関数の説明のコメント)
  - `internal/display/`(`iac.go` 以外。パッケージの説明と公開の API)
  - `internal/testutil/`(偽の AWS と、iac の大きなテスト用のファイル作成)
  - `scripts/`(`verify.sh`、`verify-u1.sh`、`u6check.go` の先頭と説明)
  - `test/sandbox/`、`docs/iam/`(ファイルの一覧と役割)
  - 各パッケージのテスト(テスト関数の名前とビルドタグ)
- **Left out**:
  - `.gitignore` が無視するビルドとテストの出力(ルートの実行ファイル、`dist/`、`*.test`、カバレッジの出力)。開いていません。
  - AI-DLC の導入物と作業領域(調査の対象外)。

### Packages Found
- `cmd/tag-refactor` — 実行ファイルの入口 — Go — `cli.Run` を呼び、Ctrl+C(Unix では SIGTERM も)で取り消す context を作り、panic を固定の1行と終了コード 1 に変える
- `internal/cli` — コマンドライン — Go — cobra でコマンドとオプションを組み立て、部品をつなぎ、エラーの種類を終了コードと2行のメッセージに変える。終了コードを決めるのはこのパッケージだけ
- `internal/iac` — IaC への反映 — Go — 保存した変更案を CloudFormation(YAML と JSON)と Terraform(ルートモジュールの `.tf`)に反映する。AWS を呼ばない
- `internal/planner` — 変更案 — Go — ルールファイルと収集の結果から変更案(4種類の変更)を作り、JSON で保存し読み戻す(契約 C2)
- `internal/impact` — 影響分析 — Go — IAM とリソースポリシーのうち、変わるタグを参照するものを探す(契約 C9)
- `internal/inventory` — 収集 — Go — 全リージョンのタグ付きリソースを読み、キーの表記ゆれをまとめ、一度もタグのないリソースを探す(契約 C7)
- `internal/rules` — ルールファイル — Go — ルールファイルを読み検証し、`rules suggest` の下書きを作る(契約 C1、C8)
- `internal/apply` — 書き換えと取り消し — Go — `apply` と `rollback` の実行(まとめ書き、読み戻し、2段の停止)
- `internal/journal` — 記録と印 — Go — 実行の記録(契約 C4、C10)と、同じアカウントの同時実行を防ぐ印
- `internal/awsaccess` — AWS への入口 — Go — AWS SDK を使う唯一のパッケージ(契約 C6)。読み取りの `Session` と書き込みの `Writer` を分ける
- `internal/display` — 表示 — Go — 標準出力と標準エラーへの出力のすべて。4つの形式、英語と日本語、色、サニタイズ
- `internal/fileout` — ファイルの保存 — Go — 一時ファイルと名前の変更による保存と置き換え(`Create`、`Replace`)
- `internal/errkind` — エラーの種類 — Go — 共通のエラーの種類と理由(契約 C12)
- `internal/restype` — 種類の表 — Go — iac が扱う resourceType と CloudFormation と Terraform の種類の対応表
- `internal/testutil` — テストの補助 — Go — HTTP の層の偽の AWS、AWS の環境の切り離し、偽の時計、iac の大きなテスト用のファイル
- `scripts` — 検証のスクリプト — bash と Go(`//go:build ignore`)— 検証用アカウントでのチェックポイントの確認(手元だけ)

### Build System
- **Type**: Go modules(`go 1.26.0`、ツールは `GOTOOLCHAIN=local`)
- **Config Files**: `go.mod`、`go.sum`、`.gitattributes`(全テキストを LF、ファジングの種はバイトのまま)、`.gitignore`
- **Build Dependencies**: `cmd/tag-refactor` → `internal/cli` → (`internal/apply`、`internal/awsaccess`、`internal/display`、`internal/errkind`、`internal/fileout`、`internal/iac`、`internal/impact`、`internal/inventory`、`internal/journal`、`internal/planner`、`internal/rules`)。`internal/iac` → `internal/planner`、`internal/restype`、`internal/fileout`、`internal/errkind`。`internal/display` → `internal/iac`、`internal/apply`、`internal/planner`、`internal/inventory`、`internal/journal`、`internal/awsaccess`。循環はありません(Go のコンパイラが禁じる)
- 配るバイナリは `CGO_ENABLED=0`、`-trimpath` で5つの OS とアーキテクチャ向けにビルドする(`ci.yml` の `artifacts`)。`govulncheck` は `go.mod` の `tool` の行で版を固定

### APIs Discovered
- CLI — `internal/cli` — 6つのコマンド(`scan`、`plan`、`rules suggest`、`apply`、`rollback`、`iac`)と、共通の `--lang`、`--debug`
- 保存した変更案の JSON(契約 C2、`version: 1`)— `internal/planner/saved.go` — `WritePlan` と `LoadPlan`/`ParsePlan`
- 実行の記録の JSON(契約 C4、C10、`version: 1`)と印のファイル — `internal/journal`
- `plan`/`scan --format json` の出力(契約 C3)— `internal/display` と `planner.WriteDetails`
- ルールファイルの YAML(契約 C1)— `internal/rules`
- iac の入力: `terraform show -json` の出力 — `internal/iac/tfstate.go`(`format_version`、`values.root_module.resources[].address/mode/values.arn`、`child_modules`)
- 内部のインタフェース — `awsaccess`(`AccountResolver`、`TagReader`、`PolicyReader`、`ConfigReader`、`ExplorerReader`、`ARNTagReader`、`TagWriter`、`ObjectStore`、`DebugSink`)、`planner.ImpactAnalyzer`、`inventory.ProgressSink`、`apply.ProgressSink`、`iac.parsedFile`(3つの形式の読み取りの共通の形)
- 外部のプロセス — `git` だけ(`internal/iac/git.go`。`rev-parse`、`ls-files -s -z`、`ls-tree -z HEAD`)。`gh` は呼んでいません

### Frameworks & Libraries
- `github.com/spf13/cobra` — v1.10.2 — コマンドラインの組み立て
- `github.com/aws/aws-sdk-go-v2`(本体 v1.47.4、`config` v1.33.11、`credentials` v1.20.11 ほか、サービスは configservice、ec2、iam、kms、resourceexplorer2、resourcegroupstaggingapi、s3、secretsmanager、sns、sqs、sts)— AWS の呼び出し(`internal/awsaccess` だけ)
- `github.com/aws/smithy-go` — v1.28.6 — SDK のミドルウェア(試行の数え上げ)
- `github.com/goccy/go-yaml` — v1.19.2 — ルールファイルの YAML(構文木を自分で検査)
- `go.yaml.in/yaml/v3` — v3.0.6 — iac の CloudFormation YAML(ノードの木だけを使う)
- `github.com/hashicorp/hcl/v2` — v2.25.0 — iac の Terraform(`hclsyntax`。MPL-2.0)
- `golang.org/x/sync` — v0.24.0 — `errgroup`、`semaphore`
- `golang.org/x/sys` — v0.49.0 — Windows のコンソール
- `golang.org/x/term` — v0.47.0 — 端末の判定
- `golang.org/x/vuln`(`govulncheck`)— v1.9.0 — `tool` の行で固定

### Test Coverage
- **Test Directories**: 各パッケージの隣の `*_test.go`(115 ファイル、テスト関数 763)。`internal/display/testdata/`(出力の期待のファイル 4形式 × 2言語、iac の結果と案内)、`internal/iac/testdata/`(`sandbox-plan.json`、手で作った `terraform-show-example.json`)、`internal/*/testdata/fuzz/`(ファジングの種)、`test/sandbox/`(検証用アカウントのスタックとルールファイル)
- **Test Frameworks**: Go の標準の `testing`(外部のアサーションのライブラリなし)。HTTP の層の偽の AWS(`internal/testutil`)。ファジング 3つ(`FuzzParse`、`FuzzLoadPlan`、`FuzzParsePolicy`)、ベンチマーク 2つ(`BenchmarkParseYAML`、`BenchmarkParseHCL`)。受け入れ条件のテストは `internal/cli/acceptance_*_test.go`(U8 は `acceptance_u8_test.go`)。実際のアカウントの結合テストは `//go:build integration`(手元だけ)
- **Coverage Config**: present(`ci.yml` の `coverage` のジョブ。`-coverpkg=./cmd/tag-refactor/...,./internal/...` の文の単位で 80% 以上を求める)。この調査ではカバレッジを測っていません

### Code Quality Indicators
- **Linting**: `gofmt -l .`、`golangci-lint` v2.15.0 の既定の linter(設定ファイルなし)、`go vet -tags integration ./...`、`cfn-lint` 1.57.2(`test/sandbox/*-fixtures.yaml` と、iac が書き換えたテンプレートの写し)。すべて `ci.yml` の `lint` のジョブ
- **CI/CD**: `.github/workflows/ci.yml`(`test` を Linux・macOS・Windows × Go stable・oldstable、`race`、`coverage`、`lint`、`vuln`、`modcheck`、`build`、必須の `ci-gate`、`main` への push のときだけの `artifacts`)、`.github/workflows/scheduled.yml`(毎週の `govulncheck`)、`.github/dependabot.yml`(gomod と github-actions を毎週)。アクションはコミットのハッシュで固定。公開(GitHub Releases)のワークフローはリポジトリにありません
- **Documentation**: `README.md`(冒頭だけ英語、本文は日本語。コマンドごとの説明、終了コード、権限、開発の手順、検証の手順)。どのパッケージにも説明のコメントがあり、関数のコメントは要件の番号(BR、NFR、契約 C)を引いています。`docs/iam/` に権限の例

### Technical Debt Signals
- TODO、FIXME、HACK、`nolint` のコメントはありません(`grep` で0件)。
- 500 行を超えるファイル: `internal/planner/planner.go`(707)、`internal/planner/saved.go`(672)、`internal/display/results.go`(588)、`internal/iac/read_yaml.go`(539)、`internal/display/messages.go`(518)、`internal/iac/depth.go`(515)、`internal/rules/schema.go`(515)。どれも1つの役割の中で大きいもので、複数の役割の混在ではありません。
- `internal/iac/testdata/terraform-show-example.json` は手で作ったもので、実物の `terraform show -json` の出力ではなく、種類ごとの `values.arn` は確かめていない(同じフォルダーの `README.md` に明記)。
- `internal/restype` の表は 18 種類だけで、表にない CloudFormation の種類は場所が見つからない扱いになります。IAM の種類は表にありますが、`scan` が IAM のロールとユーザーを集めないため、まだ変更案に入りません。
- README の「ライセンス」に、配るバイナリへのライセンスの文の同梱は「配布の作業で行う」とあり、`v0.1.0` は公開済みです。公開のワークフローがリポジトリにないため、同梱されたかはこの調査では確かめられません。
- `internal/display` が `internal/iac`、`internal/apply` などの結果の型を直接取り込んでおり、表示が各機能の型に依存しています(新しい機能を足すと表示のパッケージも変わる)。
- `scripts/verify.sh` は 1,093 行の bash で、検証用アカウントの手順が1つのファイルに集まっています(手元だけで動き、CI では動かない)。

## Handoff Summary
- **Intent-relevant finding**:
  - iac の流れは `internal/iac/run.go` の `Writer.Run`(46行目)に1本でまとまっています: `--dir` を実体の場所にする(`iac.go` `resolveRoot`)→ `planner.LoadPlan` で変更案を読む(200 MB まで)→ `--tf-state` を1リソースずつ読む(`tfstate.go`)→ 対象を決める(`target.go` `decideTool`: `aws:cloudformation:stack-name` と `aws:cloudformation:logical-id` のタグで CloudFormation、state の ARN の完全一致で Terraform)→ 候補のファイルを探す(`walk.go`: `.yaml`/`.yml`/`.json`/`.template` は下のフォルダーも、`.tf` は直下だけ。`.git`、`.terraform`、`node_modules`、`cdk.out` は入らない)→ 1回目の読み取りで索引を作る → 場所を1つに決める(`source.go` `placeIndex.find`: 論理 ID と `restype` の種類が合うもの。2つ以上ならファイル名とスタック名で1つに)→ 2回目の読み取り(SHA-256 で同じことを確かめる)→ 変更をメモリで当てる(`changes.go` `simulate`)→ 範囲の書き換え(`edit.go`)→ 書き換えたあとの確かめ(`checkEdits`: 範囲の外のバイト、対象のタグ、ほかのリソースのタグ、要素の間の行)→ 差分(`diff.go`)→ `--write` なら git の確かめ(`git.go` `checkGit`)→ 書く直前の読み直し(`checkBeforeWriting`)→ `fileout.Replace` で1つずつ置き換え。
  - 形式ごとの読み取りは `parsedFile` インタフェース(`source.go` 130行目: `refs`、`site`、`allTags`)の背後にあり、変更の適用、範囲の書き換え、差分は形式を知りません。CDK(TypeScript と Python)の読み取りを足すなら、ここに4つ目の実装を足す形になります。ただし、CDK のリソースは `aws:cloudformation:logical-id` のタグで見つかっても、論理 ID は CDK が生成したもの(例: `MyBucketF68F3FF0`)で、ソースのコードには現れません。今は `cdk.out` を探さず、CDK のリソースは `templateNotFound` として手で直す一覧に出ます(`display/iac.go` の `reason.templateNotFound` の文、README 351行目)。
  - プルリクエストの作成に関わる現状: 外部のプロセスは `internal/iac/git.go` の `git` だけで、読み取りの `rev-parse`、`ls-files`、`ls-tree` に限られます。`git status` は使わず、`GIT_` で始まる環境変数を消し、`core.fsmonitor=false`、`GIT_OPTIONAL_LOCKS=0`、`GIT_LITERAL_PATHSPECS=1`、`GIT_TERMINAL_PROMPT=0` を付け、1回 60 秒で打ち切ります(`git.go` 20〜70行目)。これは「リポジトリの設定のプログラムを動かさない」という設計(README 327行目)です。`gh` の呼び出し、ブランチ作成、コミット、push はどこにもありません。
  - オプションは `internal/cli/iac.go` の `iacCommand`(48行目)で `--plan`、`--dir`、`--tf-state`、`--write` の4つと、ルートの `--lang`、`--debug`。終了コードは `iac.Result.ExitCode()`(`iac.go` 156行目: `WriteErr` か中断で 1、手で直すものがあれば 2、それ以外 0)で、`runIac` が `fixedExit` に入れます。表示は `display.Printer.IacResult`。
  - `cli.Deps.iac`(`iacRunner`、`cli.go` 58〜60行目)はテストが iac の実行を差し替える口で、型に AWS の接続を入れる場所がない(U8 NFR3.16)ことをテスト `TestIacIsPutTogetherWithoutAWS` と `TestImportsNoAWSDirectly` が確かめています。
- **Risks / follow-up**:
  - プルリクエストの作成は、今の「git の読み取りだけ、リポジトリの設定を動かさない」という前提と衝突します。コミットや push はフック、`gpgsign`、資格情報の補助などのリポジトリや利用者の設定を動かすため、どこまで利用者の `git`/`gh` に任せるかを要件で決める必要があります。
  - 利用者の `gh` と `git` を使うと、CI の「AWS を呼ばない」に加えて「GitHub を呼ばない」テストの仕組み(`runGit` のような差し替えの口)が要ります。今の `git.go` の `lookGit`、`runGit`、`gitCheck` の変数による差し替えが前例です。
  - CDK の論理 ID はハッシュの付いた生成名で、ソースのコンストラクトの ID とは1対1になりません。`cdk.out` の `manifest.json` やテンプレートの `aws:cdk:path` のメタデータを使うかどうかで設計が大きく変わります(`cdk.out` は今は除外のフォルダー)。
  - TypeScript と Python のパーサーは `go.mod` にありません。新しい依存はライセンス(README のライセンスの節、hcl/v2 の MPL-2.0 の前例)、`govulncheck`、`CGO_ENABLED=0` の静的なバイナリ(NFR5.7、NFR9.12)の条件を満たす必要があります。
  - 今の iac の安全の柱(範囲だけの書き換え、書き換えたあとの再解析での確かめ、2回の読み取りのハッシュ、git でのコミット済みの確認、`fileout.Replace` での置き換え)は、CDK の書き換えにも同じ強さで求められると考えられます。
  - この調査の最中にも、ほかのセッションが README、`internal/apply`、`scripts/`、`internal/iac/testdata` を変えています。フィンガープリントは公開までに変わりうる点に注意してください。
