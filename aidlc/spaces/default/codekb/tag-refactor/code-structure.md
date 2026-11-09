# コードの構成

## フォルダーとパッケージの構成

```
tag-refactor/
  cmd/tag-refactor/        入口(main.go、Unix と Windows のシグナル)
  internal/
    cli/                   コマンドとオプション、部品の組み立て、終了コード
    iac/                   IaC への反映(CloudFormation YAML/JSON、Terraform)
    planner/               変更案の作成、保存(契約 C2)、読み込み
    impact/                影響分析(IAM とリソースポリシー)
    inventory/             収集、キーのまとめ、一度もタグのないリソース
    rules/                 ルールファイルの読み込みと下書き
    apply/                 apply と rollback の実行
    journal/               実行の記録と印
    awsaccess/             AWS SDK を使う唯一の部品
    display/               出力のすべて(形式、言語、色、幅)
    fileout/               一時ファイルと名前の変更による保存
    errkind/               エラーの種類(契約 C12)
    restype/               iac の種類の対応表
    testutil/              テストの補助(偽の AWS ほか)
  scripts/                 検証用アカウントでの確認(手元だけ)
  test/sandbox/            検証用のスタックとルールファイル
  docs/iam/                権限の例(読み取り用、書き換え用)
  .github/                 CI、毎週の脆弱性の確認、Dependabot
```

`internal/` の下に置くことで、すべてのパッケージがモジュールの外から使えないようにしています。公開の API はコマンドラインだけです。パッケージは機能(収集、計画、影響、書き換え、IaC)ごとに分け、外部への出入り口(AWS、ファイル、端末)を別のパッケージにしています。

## ファイルの分類

Go のソースは、テストと `internal/testutil` を除いて約 24,100 行(うち `internal/iac` が約 4,700 行)、`internal/testutil` が約 2,800 行、テストは 115 ファイル・約 28,800 行(テスト関数 763)です。

| 分類 | 主なファイル |
|---|---|
| 入口・コマンド(Controller/Handler) | `cmd/tag-refactor/main.go`、`internal/cli/cli.go`、`commands.go`、`apply.go`、`rollback.go`、`suggest.go`、`iac.go` |
| 業務の処理(Service/UseCase) | `internal/planner/planner.go`、`internal/impact/impact.go`・`match.go`・`policy.go`、`internal/inventory/inventory.go`・`analysis.go`・`discover.go`、`internal/apply/apply.go`・`rollback.go`・`execution.go`・`worker.go`、`internal/iac/run.go`・`changes.go`・`edit.go` |
| データの形(Model/Entity) | `internal/planner/planner.go` の型、`internal/inventory/types.go`、`internal/journal/journal.go` の型、`internal/iac/iac.go` の `Request`・`Result`・`ManualItem`、`internal/iac/source.go` の `tagSite`・`tagEntry` |
| 読み書き(Repository/DAO) | `internal/planner/saved.go`、`internal/journal/store.go`・`saver.go`・`lock.go`、`internal/rules/rules.go`・`schema.go`・`encoding.go`、`internal/iac/read_yaml.go`・`read_json.go`・`read_hcl.go`・`tfstate.go`、`internal/fileout/fileout.go`・`replace.go` |
| 外部への出入り口(Adapter) | `internal/awsaccess/*.go`、`internal/iac/git.go` |
| 補助(Utility/Helper) | `internal/errkind/errkind.go`、`internal/restype/restype.go`、`internal/iac/quote.go`・`diff.go`・`depth.go`・`walk.go`、`internal/display/width.go`・`markdown.go`・`color.go` |
| 表示(View) | `internal/display/*.go`(`display.go`、`planout.go`、`scanout.go`、`results.go`、`apply.go`、`rollback.go`、`iac.go`、`messages.go`、`progress.go`、`terminal_*.go`、`lang_*.go`) |
| 設定 | `go.mod`、`.gitattributes`、`.gitignore`、`.github/workflows/*.yml`、`.github/dependabot.yml` |
| テスト | `*_test.go`、`internal/testutil/`、`internal/*/testdata/`、`test/sandbox/` |
| 生成物 | `internal/display/width_table.go`(`gen_width.go` が Unicode の `EastAsianWidth.txt` から作る。`//go:build ignore` の生成プログラム) |

## internal/iac の中の構成

| ファイル | 役割 |
|---|---|
| `doc.go` | パッケージの説明(AWS、terraform、スタックの更新を呼ばない) |
| `iac.go` | 公開の型(`Request`、`Result`、`ManualItem`、`FileResult`、`SkippedFile`)、理由と種類の定数、`Writer`/`New`、`resolveRoot`、`loadPlan`(200 MB) |
| `run.go` | `Writer.Run` と流れ全体(並列の読み取り `forEach`、メモリの予算、`placeTargets`、`processFile`、`editFile`、`finish`、`write`、`checkBeforeWriting`) |
| `target.go` | 変更案のリソースから対象を作り、CloudFormation、Terraform、管理外に分ける |
| `tfstate.go` | `terraform show -json` を1リソースずつ読み、ARN からアドレスへの索引を作る |
| `walk.go` | 候補のファイルの一覧(除外のフォルダー、深さ、数と大きさの上限)と内容の下調べ(`prepare`) |
| `depth.go` | 解析の前の入れ子の深さの検査(YAML、JSON、HCL) |
| `source.go` | 3つの形式に共通の型(`span`、`tagEntry`、`tagSite`、`parsedFile`)、行の表、解析のメモリの予算、場所の索引 `placeIndex` |
| `read_yaml.go`、`read_json.go`、`read_hcl.go` | 形式ごとの読み取り。ノードの位置からバイトの範囲を求め、範囲の文字列を読み戻して確かめる |
| `changes.go` | 変更をメモリの上で当てる(`simulate`)。反映、反映済み、手で直す、反映済みとみなした削除に分ける |
| `edit.go` | 範囲の書き換えの作成(置き換え、削除、追加、場所全体の置き換え)、並べ替え、適用、書き換えたあとの確かめ |
| `quote.go` | 新しいキーと値の書き方(YAML の引用符、JSON、HCL のエスケープ) |
| `diff.go` | 書き換えの範囲から unified diff を作る(前後3行) |
| `git.go` | `--write` の前の git の確かめ |

## コードのパターン

- **差し替えのための変数**: テストが外部の処理を置き換えられるよう、パッケージの変数に関数を置きます(`iac/run.go` の `openFile`・`replaceFile`、`iac/git.go` の `lookGit`・`runGit`・`gitCheck`、`cli/cli.go` の `closeDisplay`・`getenv`、`fileout/replace.go` の `replaceRename` など)。上限の値も変数にしてテストで小さくします(`maxCandidates`、`maxFileBytes`、`parseBudgetBytes`)。
- **依存の注入**: `cli.Deps` が HTTP の層(`Transport`)、時計、時間の上限、標準入力、iac の実行(`iac iacRunner`)を受け取り、ゼロ値が本番の設定です。
- **エラーの種類**: すべての部品は `*errkind.Error`(`Kind`、`Reason`、`DetailID`/`HintID` と引数)を返し、`cli.describe` が言語に合わせて2行にします。AWS の SDK の文言は出しません。
- **結果のオブジェクト**: iac の `Run` は、入力の誤りなどは `error` で返し、結果が出たあとの `--write` の失敗は `Result.WriteErr` に入れます。終了コードは `Result.ExitCode()` が決め、`cli` が `fixedExit` として使います。
- **2回の読み取り**: iac は1回目で索引だけを作り(解析の木は捨てる)、反映するファイルだけを2回目に読み、SHA-256 で同じであることを確かめます。書く直前にもう一度読みます。
- **要件の番号を引くコメント**: 関数や定数のコメントが BR(業務の決まり)、NFR、契約(C1〜C12)、U(Unit)の番号を引き、設計の文書と結び付けています。
- **ビルドタグ**: OS ごとのファイル(`*_windows.go`、`*_other.go`、`signals_unix.go`)、race の有無で規模を変えるテスト(`race_test.go`、`norace_test.go`)、実際のアカウントの結合テスト(`integration`)、生成やスクリプト(`ignore`)。
