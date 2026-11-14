# API の文書

## コマンドライン(外部の API)

ツールの公開の API はコマンドラインだけです(すべてのパッケージは `internal/`)。コマンドは `internal/cli` が cobra で定義します。どのコマンドも位置の引数を取りません(`cobra.NoArgs`)。

### 共通のオプション(ルート)

| オプション | 意味 |
|---|---|
| `--lang` | `en` か `ja`。なければ `LC_ALL`、`LC_MESSAGES`、`LANG`、Windows の表示言語の順。知らない値は使い方の誤り(終了コード 1) |
| `--debug` | AWS の呼び出しごとに1行(操作、リージョン、時間、再試行の数)を標準エラーに出す。iac では git の標準エラーと確かめに失敗した理由を出す。タグの値、ポリシーの中身、ソースの中身は出さない |

### コマンドとオプション

| コマンド | オプション | 説明 |
|---|---|---|
| `scan` | `--rules`(任意)、`--profile`、`--region`(繰り返し)、`--format`(`table`/`json`/`markdown`) | 表記ゆれ、必須タグの欠落、一度もタグのないリソース |
| `plan` | `--rules`(必須)、`--profile`、`--region`、`--format`(`table`/`diff`/`json`/`markdown`)、`--out` | 変更案と影響。`--out` で JSON に保存(既にあれば上書きしない) |
| `rules suggest` | `--profile`、`--region`、`--output` | ルールファイルの下書き(UTF-8、BOM なし、LF、0600) |
| `apply` | `--plan` か `--rules`(どちらか1つ)、`--yes`、`--allow-impact`、`--record-to`(フォルダーか `s3://<bucket>/<prefix>`)、`--force-unlock`、`--profile` | タグの書き換え |
| `rollback` | `--run`(必須)、`--record-from`、`--force`、`--yes`、`--force-unlock`、`--profile` | `apply` の取り消し |
| `iac` | `--plan`(必須)、`--dir`(必須)、`--tf-state`、`--write` | 変更案を CloudFormation と Terraform に反映。`--profile` はない |

`--region` は1回に1つで、繰り返して複数を指定します。リージョンの名前の形は AWS を呼ぶ前に確かめ、有効でないリージョンはリソースを読む前にエラーにします。

### 終了コード

| コマンド | 0 | 1 | 2 |
|---|---|---|---|
| `scan` | 問題なし | エラー、中断、読めないリージョン | 表記ゆれか必須タグの欠落 |
| `plan` | 変更不要(消す候補だけはありうる) | エラー、中断、収集か影響分析が不完全、保存の失敗 | 変更、衝突、値が未指定、影響のどれか |
| `rules suggest` | 下書きを出した | エラー、不完全、キーだけで 64 KiB を超えた | なし |
| `apply` | すべて書き換えた、または `yes` 以外を入力 | エラー、失敗や結果不明のリソース、停止、記録や印の失敗、アカウントの違い、収集が不完全な変更案 | 飛ばしたリソース、衝突か値が未指定、`--yes` だけで影響あり |
| `rollback` | すべて戻した、取り消すものがない、`yes` 以外 | エラー、失敗、停止、記録がない・読めない・別のアカウント、印 | 飛ばしたリソース(後から変えられた、なくなった、50個を超える) |
| `iac` | 手で直すものがない(反映不要、管理外だけの場合も) | 入力の誤り、`--write` の git の確かめか書き出しの失敗、中断 | 手で直すものがある |

規則の実装は、`scan`/`rules suggest` が `cli.outcome.exitCode`、`plan` が `planner.Plan.ExitCode`、`apply` と `rollback` と `iac` は各部品の結果(`iac.Result.ExitCode`、`apply.RollbackOutcome.ExitCode` など)を `cli.fixedExit` に入れます。Ctrl+C で止めたときは、どのコマンドも `Interrupted. Nothing was saved.` などを出して 1 です。

### 標準出力と標準エラー

- 標準出力: 結果だけ(一覧、差分、JSON、Markdown、iac の差分と件数と一覧)。
- 標準エラー: アカウントとリージョン、進み具合、警告、エラーの2行(`Error:`/`Fix:`、日本語は `エラー:`/`直し方:`)、`Checked:` の行、かかった時間、`Exit code` の行、案内。
- iac の標準出力: ファイルごとの unified diff(`--- a/<相対パス>`、`+++ b/<相対パス>`、前後3行)、ファイルごとの件数(`<file>: N changes can be reflected` / `reflected` / `not written`)、`already reflected: N`、「反映済みとみなした削除」の一覧、`To fix by hand (N):` の一覧(ARN、キー、`ファイル:行` か `(no place)`、理由、詳細)、`Not managed by IaC: N`。iac には JSON の出力はありません。

## ファイルの形式

### 保存した変更案(契約 C2、`plan --out`)

UTF-8(BOM なし)、LF、0600。`planner.WritePlan` が書き、`planner.LoadPlan`/`ParsePlan` が読みます(`apply --plan` と `iac --plan`)。大きさの上限は 256 MiB(iac は 200 MB を超えると読まない)。

```json
{
  "version": 1,
  "toolVersion": "v0.1.0",
  "planId": "plan-20261019T120400Z-7f3a",
  "accountId": "123456789012",
  "createdAt": "2026-10-19T12:04:00Z",
  "regions": ["ap-northeast-1"],
  "inventoryComplete": true,
  "impactAnalysisComplete": true,
  "ruleFile": "tagrules.yaml",
  "resources": [
    {
      "arn": "arn:aws:sqs:ap-northeast-1:123456789012:orders",
      "region": "ap-northeast-1",
      "global": false,
      "resourceType": "sqs",
      "beforeTags": {"env": "Prod", "aws:cloudformation:stack-name": "orders", "aws:cloudformation:logical-id": "Queue"},
      "iacManagedBy": "cloudformation:orders",
      "changes": [
        {"changeId": "c-000001", "kind": "renameKey", "fromKey": "env", "toKey": "Environment", "fromValue": "Prod", "mergesIntoExisting": false, "impacted": false},
        {"changeId": "c-000002", "kind": "changeValue", "fromKey": "Environment", "fromValue": "Prod", "toValue": "prod", "impacted": false}
      ]
    }
  ],
  "keyConflicts": [{"arn": "...", "canonicalKey": "Environment", "presentTags": {"env": "a", "Env": "b"}}],
  "impactFindings": [{"verdict": "affected", "policyRef": "...", "sourceKind": "...", "statementId": "...", "conditionOperator": "...", "conditionKey": "...", "conditionValue": "...", "changeIds": ["c-000001"]}],
  "unreadPolicies": [{"sourceKind": "...", "policyRef": "...", "region": "...", "reason": "..."}],
  "unsetRequiredTags": [{"key": "CostCenter", "arns": ["..."]}],
  "removeCandidates": ["Name"],
  "unsupportedResourceTypes": [{"resourceType": "...", "count": 1}]
}
```

- `kind` は `renameKey`、`changeValue`、`addRequired`、`removeKey`。欄の有無は種類で決まります(`renameKey` は `fromKey`/`toKey`/`fromValue`/`mergesIntoExisting`、`changeValue` は `fromKey`/`fromValue`/`toValue`、`addRequired` は `toKey`/`toValue`、`removeKey` は `fromKey`/`fromValue`)。
- `changeValue` の `fromKey` は、同じリソースの `renameKey` の `toKey`(名前を変えたあとのキー)のことがあります。
- `mergesIntoExisting: true` の `renameKey` は `toKey` を書かず、`fromKey` を消すだけです(iac は削除として扱う)。
- 読み込みは `version` が 1 以外なら `UnsupportedVersion`、UTF-16 の BOM は「UTF-8 で保存し直す」誤り、知らない欄は無視、知らない `kind`/`verdict`/`sourceKind` は誤り、`aws:` で始まるキーを変える変更は誤り。影響の印(`impacted`、`Mark`)は保存された値を使わず、`impactFindings` から計算し直します。
- iac は `beforeTags` の `aws:cloudformation:stack-name` と `aws:cloudformation:logical-id` で CloudFormation のリソースを見分けます。`iacManagedBy` は使いません。

### 実行の記録(契約 C4、C10)と印

`~/.tag-refactor/runs/<アカウントID>/<実行ID>.json`(または `--record-to` のフォルダーや S3 の前置きの下)。実行 ID は `20261019T211003Z-7f3a` の形です。

| 欄 | 内容 |
|---|---|
| `version`、`toolVersion`、`runId`、`accountId`、`planId`、`startedAt`、`completed` | 実行の情報 |
| `entries[]` | `arn`、`region`、`touchedKeys`、`beforeTags`、`plannedAfterTags`、`afterTags`、`status`(`pending`/`applied`/`skipped`/`failed`)、`reason` |
| `rollbacks[]` | `rollbackId`、`startedAt`、`completed`、`results[]`(`arn`、`status`、`reason`) |

印(`.lock`)は `accountId`、`purpose`(`apply`/`rollback`)、`runId`、`createdAt`、`host`、`pid`、`toolVersion` を持ちます。失敗と飛ばしの詳細は `<実行ID>.log` に1行ずつ書きます。

### ルールファイル(契約 C1)

`version: 1`、`keys.<正しいキー>.aliases`/`values`、`required.<キー>.default`/`when[].match`(`resourceType`、`region`、`tags`)/`value`、`remove`。64 KiB まで、入れ子 32 段まで、アンカー・別名・統合キー・タグ・重複キーは受け付けません。UTF-8(BOM 可)と BOM 付きの UTF-16 を読みます。

### iac の入力: `terraform show -json` の出力

`--tf-state` には引数なしの `terraform show -json` の出力を渡します。読むのは `format_version`(必須)、`values.root_module.resources[]` の `address`・`mode`・`values.arn` と `child_modules`(256 段まで)だけで、ほかの値(秘密を含む)は保持しません。`planned_values`、`resource_changes`、`prior_state` があれば plan のファイルの出力として誤りにします。500 MB まで。UTF-16 は誤り、UTF-8 の BOM は読み飛ばします。`mode` が `managed` で `arn` のあるリソースだけを ARN → アドレスの索引にします。

## 内部の API(パッケージの境界)

### internal/iac

```go
type Request struct { PlanPath, Dir, TFStatePath string; Write bool }
func New() *Writer
func (w *Writer) Run(ctx context.Context, req Request) (*Result, error)
func (r *Result) ExitCode() int
```

- `Result` の欄: `Files []FileResult`(`File`、`Tool`、`Reflected`、`Diff`、`Written`)、`Manual []ManualItem`、`Reflected`、`AlreadyReflected`、`AssumedRemovals`、`Unmanaged`、`Resources`、`SkippedFiles`、`SkippedFolders`、`HasTFState`、`WriteRequested`、`WriteErr`、`Interrupted`、`Debug`。
- `ManualItem` の欄: `ARN`、`ChangeID`、`Key`、`File`(`--dir` からの相対、`/` 区切り)、`Line`、`Reason`、`Detail`、`DetailKind`(`address`、`addresses`、`candidates`、`otherType`、`skipped`)。タグの値は持ちません。
- 手で直す理由(`ManualReason`): `templateNotFound`、`blockNotFound`、`ambiguous`、`inModule`、`multipleInstances`、`tagsNotLiteral`、`unsupportedTagShape`、`notLiteral`、`keyNotInSource`、`valueDiffers`、`keyAlreadyExists`、`noTagSite`、`unreadableFile`。
- 外したファイルの理由(`SkipReason`): `tooLarge`、`notUTF8`、`tooDeep`、`parseError`、`unreadable`。
- `Tool`: `ToolNone`、`ToolCloudFormation`、`ToolTerraform`。
- パッケージの中の形式の境界は `parsedFile`(`refs() []resourceRef`、`site(key) *tagSite`、`allTags() []resourceTags`)と `parseContent`(形式で3つの読み取りに振り分ける)です。

### internal/cli

```go
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, deps Deps) int
```

`Deps` は `Transport`(偽の AWS)、`Now`、時間の上限、`RetryDelay`、`ExplorerPageSize`、`Stdin`、`StdinIsTerminal`、パッケージの中だけの `savingHook`、`interrupts`、`iac`(`iacRunner func(context.Context, iac.Request) (*iac.Result, error)`)を持ちます。ゼロ値が本番の設定です。

### internal/planner

- `Build(snap *inventory.Snapshot, rs *rules.RuleSet, opts Options) *Plan`
- `WritePlan(w io.Writer, p *Plan) error`、`LoadPlan(path string) (*Plan, error)`、`ParsePlan(path string, data []byte) (*Plan, error)`
- `ImpactAnalyzer` インタフェース(`Analyze(ctx, []TagChangeInput) (ImpactResult, error)`)。実装は `impact.Analyzer`
- `Plan.ExitCode()`、`NeedsAnalysis()`、`AttachImpact()`、`Summary()`、`AfterTags()`

### internal/awsaccess

- 読み取り: `AccountResolver`(`Resolve`)、`TagReader`(`ListTaggedResources`)、`PolicyReader`(= `IAMPolicyReader` + `ResourcePolicyReader`)、`ConfigReader`、`ExplorerReader`、`ARNTagReader`(`GetTags`、100 ARN まで)。`Session` が実装し、書き込みのメソッドは持たない。
- 書き込み: `TagWriter`(`TagResources`、`UntagResources`、20 ARN まで)、`ObjectStore`(`Put`、`Get`、`PutIfAbsent`、`Delete`)。`Writer`(`NewWriter(s)`)が実装し、`apply` と `rollback` だけが作る。
- `DebugSink`(`APICall`)。`display.Printer` が実装。

### internal/apply と internal/journal

- `apply.New(Options) *Applier`、`apply.NewRollbacker(Options) *Rollbacker`、`apply.AWS`(= `ARNTagReader` + `TagWriter`)、`apply.ProgressSink`。
- `journal.New(Options) *Journal`、`Lock`、`Release`、`ForceUnlock`、`Begin`、`ParseLocation`、`NewRunID`、`ValidRunID`、`DefaultDir`。

### internal/fileout

- `Prepare(path, prefix)`(保存先を確かめる)、`Create(path, prefix, limit) (*Pending, error)`(新しいファイル。既にあれば上書きしない)、`Replace(path, content, ReplaceOptions)`(既にあるファイルの置き換え。iac が使う)、`RetryRename`、`TransientRename`、`UserFileWaits`。

### internal/display

`Printer` が出力のすべてを持ちます(`ScanResult`、`PlanResult`、`PlanHeader`、`PlanSaved`、`IacResult`、`Error`、`ExitCode`、`Done`、進み具合など)。メッセージは `T(lang, id, args...)` の一覧から引き、英語と日本語を持ちます。iac のメッセージは `internal/display/iac.go` の `iacCatalog` にまとまっています。
