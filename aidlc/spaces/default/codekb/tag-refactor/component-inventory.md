# 部品の一覧

## 一覧の見方

各部品の見出しの名前は、`reverse-engineering-timestamp.md` の Scope of Analysis の `components` と同じです。行数はテストを除いた Go のソースの数です(2026-11-07、コミット `9bf1fb6`)。

| 部品 | 種類 | 行数(約) | 主な依存 |
|---|---|---|---|
| `cmd/tag-refactor` | 入口 | 80 | `internal/cli` |
| `internal/cli` | コマンド | 1,800 | ほぼすべての `internal` |
| `internal/iac` | 業務(IaC) | 4,700 | `planner`、`restype`、`fileout`、`errkind`、yaml/v3、hcl/v2 |
| `internal/planner` | 業務(変更案) | 1,800 | `inventory`、`rules`、`errkind` |
| `internal/impact` | 業務(影響分析) | 1,500 | `planner`、`awsaccess`、`errkind` |
| `internal/inventory` | 業務(収集) | 1,300 | `awsaccess`、`errkind` |
| `internal/rules` | 業務(ルール) | 2,200 | `errkind`、goccy/go-yaml |
| `internal/apply` | 業務(書き換え) | 2,400 | `awsaccess`、`journal`、`planner`、`errkind` |
| `internal/journal` | 業務(記録と印) | 1,400 | `awsaccess`、`fileout`、`errkind` |
| `internal/awsaccess` | 出入り口(AWS) | 2,400 | AWS SDK for Go v2、`errkind` |
| `internal/display` | 出入り口(出力) | 4,000 | `iac`、`apply`、`planner`、`inventory`、`journal`、`awsaccess`、x/term、x/sys |
| `internal/fileout` | 出入り口(ファイル) | 400 | `errkind` |
| `internal/errkind` | 共通 | 200 | なし |
| `internal/restype` | 共通 | 60 | なし |
| `internal/testutil` | テスト | 2,800 | 標準ライブラリだけ |
| `scripts` | 検証 | 1,600(bash を含む) | AWS CLI、`go` |
| `test/sandbox` | 検証のデータ | — | — |
| `docs/iam` | 文書 | — | — |
| `.github` | CI | — | GitHub Actions |

## 部品ごとの責任

### cmd/tag-refactor

- 責任: `main` で `cli.Run` を呼ぶ。Ctrl+C(Unix では SIGTERM も)で取り消す context を作り、最初のシグナルのあとは Go の既定に戻して2回目ですぐ終わる。panic は値もスタックも出さず、固定の1行と終了コード 1 にする。
- ファイル: `main.go`、`signals_unix.go`、`signals_windows.go`。
- 依存: `internal/cli`。

### internal/cli

- 責任: cobra のコマンドとオプションの定義(`scan`、`plan`、`rules suggest`、`apply`、`rollback`、`iac`、共通の `--lang`、`--debug`)、オプションの確かめの順序(AWS を呼ぶ前に)、コマンドごとに使える AWS の読み取りだけを持つ組み立て(`tagReaders`、`readers`、`applyReaders`、`rollbackReaders`)、影響分析の途中で認証情報が切れたら全体を止める `stoppingPolicyReader`、確認の入力(`yes`)、2段の停止、終了コードと2行のエラーのメッセージ(`describe`)。
- iac の部分(`iac.go`): オプション4つ、`--plan` と `--dir` の必須の確かめ、`deps.iac` か `iac.New().Run` の呼び出し、`display.IacResult`、`Result.ExitCode()` からの終了コードと意味、`WriteErr` の2行の表示。
- 依存: `apply`、`awsaccess`、`display`、`errkind`、`fileout`、`iac`、`impact`、`inventory`、`journal`、`planner`、`rules`、cobra、x/term。

### internal/iac

- 責任: 保存した変更案を CloudFormation のテンプレート(YAML、JSON、`.template`)と Terraform のルートモジュールの `.tf` に反映する。AWS、terraform、スタックの更新を呼ばない。
- 主な処理: 対象の決定(スタックのタグか state の ARN)、候補のファイルの一覧(上限: 20,000 個、1つ 10 MB、合計 500 MB、深さ 32、入れ子 256)、2回の読み取りとハッシュの確かめ、場所の決定(論理 ID と種類、アドレス)、変更のメモリの上での適用、範囲の書き換え、書き換えたあとの再解析での確かめ、unified diff、`--write` の git の確かめ、書く直前の読み直し、`fileout.Replace` での置き換え。
- 並列: 読み取りと解析は最大8つのゴルーチン(`maxWorkers`)、解析中のバイト数は 32 MiB まで(`parseBudgetBytes`)、ソフトなメモリの上限 1.5 GiB(`softMemoryLimit`)。
- 外部のプロセス: `git`(読み取りのコマンドだけ、60 秒の上限、`GIT_` の環境変数を消す)。
- 依存: `planner`、`restype`、`fileout`、`errkind`、`go.yaml.in/yaml/v3`、`hashicorp/hcl/v2`、`x/sync/semaphore`。AWS の SDK は取り込まない(テスト `TestImportsNoAWSDirectly`)。

### internal/planner

- 責任: 収集の結果とルールファイルから変更案を作る(リソースごとに名前の変更 → 値の変更 → 必須タグ → 削除の順、`c-000001` からの番号)。衝突、値が未指定、消す候補、変更できない種類を分ける。影響分析のインタフェースを定義し、結果を結び付ける。変更案の保存(`WritePlan`、`SafeJSONWriter` で制御文字をエスケープ)と読み込み(`LoadPlan`、`ParsePlan`)。終了コードの規則(`Plan.ExitCode`)。
- 依存: `inventory`、`rules`、`errkind`。

### internal/impact

- 責任: IAM のポリシー(`iam:GetAccountAuthorizationDetails` で一括)と、S3、KMS、SQS、SNS、Secrets Manager のリソースポリシーを読み、タグの条件キーとポリシー変数を4種類の変更と照合して「影響あり」「判定できない」を出す。読めないポリシーは不完全として記録し、ほかは続ける。並列はリージョン 8、1リージョンの中 4。
- 依存: `planner`、`awsaccess`、`errkind`、`x/sync/errgroup`。

### internal/inventory

- 責任: 最大8リージョンを同時に `tag:GetResources` で読む。読めないリージョンは結果に残し、認証情報の切れは全体を止める。キーの表記ゆれのまとめ(大文字・小文字、区切り文字、略語の辞書)、必須タグの欠落、一度もタグのないリソースの検出(Resource Explorer の集約の索引、AWS Config、Resource Explorer)。
- 依存: `awsaccess`、`errkind`、`x/sync/errgroup`。

### internal/rules

- 責任: ルールファイルの読み込み(UTF-8/UTF-16、64 KiB、入れ子の事前の検査、構文木での検査)、検証(知らないキー、型、矛盾を行番号つきで最初の1つで止める)、`plan` からの問い合わせへの答え、`rules suggest` の下書きの作成。
- 依存: `errkind`、goccy/go-yaml。`inventory` からの情報は関数の値で受け取る。

### internal/apply

- 責任: `apply`(印、確認、今のタグの読み取り、飛ばしの判定、書くタグと消すキーの計算、50個の上限、書く前の記録、リージョン 4 の並列、20件のまとめ書き、100件の単位での読み直しと読み戻し、2段の停止)と `rollback`(記録の確かめ、触れたキーだけの判定、書き戻し、読み戻しの再試行、記録への追記)。共通の部分は `execution.go`。
- 依存: `awsaccess`、`journal`、`planner`、`errkind`。

### internal/journal

- 責任: 実行の記録(手元は一時ファイルとディスクへの書き出しと名前の変更、S3 は1回の書き込み。5秒ごとと終わりに保存)、ログのファイル、印(手元は `O_EXCL`、S3 は「なければ作る」)、`--force-unlock`、記録の場所の解釈(`ParseLocation`)、実行 ID。
- 依存: `awsaccess`、`fileout`、`errkind`。

### internal/awsaccess

- 責任: AWS の SDK を使う唯一の部品。設定の読み込み、リージョンの決定、再試行(最大10回、1回の待ち最大20秒、1回の試行 30 秒、1回の呼び出し 5 分)、エラーのコードから種類への変換(SDK の文言は出さない)、`--debug` の記録、読み取りの `Session` と書き込みの `Writer`、ARN の解析、権限の一覧(文書とテストで照合)。
- 依存: AWS SDK for Go v2(11 のサービス)、smithy-go、`errkind`。

### internal/display

- 責任: 出力のすべて。結果の4つの形式、英語と日本語のメッセージの一覧、色(`NO_COLOR`、`TERM=dumb`、Windows の仮想端末)、端末での文字の幅、制御文字のサニタイズ、進み具合、iac の差分と一覧と案内。
- 依存: `iac`、`apply`、`planner`、`inventory`、`journal`、`awsaccess`、`errkind`、x/term、x/sys/windows。

### internal/fileout

- 責任: 利用者のファイルの保存。新しいファイルは同じフォルダーの一時ファイルに書いてからハードリンクで名前を付け(既にあれば上書きしない)、既にあるファイルの置き換え(`Replace`)は一時ファイルに書いて同期して読み戻し、権限を写してから名前を変える(Windows で他のプログラムが開いているときは最大 4.7 秒待つ)。
- 依存: `errkind`。

### internal/errkind

- 責任: エラーの種類(`Internal`、`InvalidInput`、`UnsupportedVersion`、`NotFound`、`AccessDenied`、`CredentialsMissing`、`Transient`、`StorageFailed`、`AccountMismatch`、`Conflict`、`Unavailable`)と理由。包んだエラーの文言は表示しない。
- 依存: なし。

### internal/restype

- 責任: iac が扱う 18 の resourceType と、CloudFormation の種類(`AWS::S3::Bucket` など。Lambda は `AWS::Serverless::Function` も)と Terraform の種類の対応表。AWS の SDK に依存しない。
- 依存: なし。

### internal/testutil

- 責任: HTTP の層の偽の AWS(書き込みは `AllowWrites` がないとテストを失敗させる)、AWS の環境変数とホームの切り離し、偽の時計、ポリシーと検出のテスト用のデータ、iac の大きなテスト用のフォルダーの作成(`WriteLargeIac`: 1万リソース、約 100 MB)。
- 依存: 標準ライブラリだけ。

### scripts

- 責任: 検証用アカウントでのチェックポイントの確認。`verify.sh`(U1〜U8 の確認の手順をまとめた bash)、`verify-u1.sh`(すべてのテストと U1 の結合テスト)、`u6check.go`(変更案、記録、タグの写しを確かめる補助。`//go:build ignore`、AWS を呼ばない)。CI では動かない。

### test/sandbox

- 責任: 検証用アカウントのスタック(`u1-fixtures.yaml`、`u2-fixtures.yaml`、`u5-fixtures.yaml`、`u6-fixtures.yaml`)とルールファイル(U1〜U8)。CI の `cfn-lint` と、iac の書き換えの確かめ(`u6-fixtures.yaml` の写し)にも使う。

### docs/iam

- 責任: 必要な権限の例(`readonly-policy.json`、`apply-policy.json`)。`internal/awsaccess` のテストが、コードの使う操作と文書の操作の一致を確かめる。

### .github

- 責任: CI(`workflows/ci.yml`)、毎週の脆弱性の確認(`workflows/scheduled.yml`)、Dependabot(`dependabot.yml`)。詳しくは `code-quality-assessment.md`。

## 今回の意図に関わる部品

| 部品 | 関わり方 |
|---|---|
| `internal/iac` | CDK の読み取りと書き換えを足す中心。`parsedFile` の実装、場所の探し方(`placeIndex`)、除外のフォルダー(`cdk.out`、`node_modules`)、`git.go` の確かめ |
| `internal/cli` | `iac` のオプションの追加(例: プルリクエストの作成)、終了コードの規則 |
| `internal/display` | iac の結果と案内、理由の文(`reason.templateNotFound` の「CDK は後の作業」) |
| `internal/restype` | CDK のコンストラクトと resourceType の対応が要るなら、表の拡張 |
| `internal/fileout` | 書き換えの置き換え(`Replace`) |
| `internal/planner` | 変更案の読み込み(変更の予定はないが、`iac` の入力) |
| `.github` | CI で GitHub や `gh` を呼ばないことの確かめ、3つの OS でのテスト |
