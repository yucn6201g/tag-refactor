# 契約の一覧: iac への CDK の反映とプルリクエストの作成

作業の単位は `inception/units-generation/unit-of-work.md` と `unit-of-work-dependency.md`、部品は `inception/domain-design/components.md`、要件は `inception/requirements-analysis/requirements.md` です。出典の CD-Q… はこのステージの `contract-design-questions.md` の回答、R-… は1回目のレビュー(`reviews/review-01.md`)の指摘です。

この作業は1つのバイナリの中で完結します。境目は次の4つです。

- Go の関数と型(同じプロセスの中)
- Go と埋め込んだ補助のプログラムの間の JSON
- 外のプログラムの呼び出し
- 利用者が触るコマンドとプルリクエストの本文

前の作業の契約(変更案の JSON、`inventory.ProgressSink` と `apply.ProgressSink`、エラーの種類 `errkind`)は変えずに使い、足すところだけを書きます。

## 契約の一覧

| # | 提供する単位 | 使う側 | 形 | 持ち主 |
|---|---|---|---|---|
| X1 | U1、U2、U3(`IacCommand`) | External: 利用者(端末) | コマンドのオプションと終了コード | U2 |
| X2 | U2(`IacDisplay`、`PullRequestPublisher`) | External: GitHub とレビューする人 | プルリクエストのタイトルと本文 | U2 |
| C1 | U1、U2(`PullRequestPublisher`) | `IacCommand` | Go の関数と型 | U2 |
| C2 | U1、U3(`CdkSynthesizer`) | `IacWriter` | Go の関数と型 | U3 |
| C3 | U1、U4、U5(`CdkSourceAnalyzer`) | `IacWriter` | Go の関数と型 | U4 |
| C4 | 埋め込んだ補助のプログラム(TypeScript 用、Python 用) | `CdkSourceAnalyzer` | 標準入出力の JSON(版つき) | U4(TypeScript)、U5(Python)。形は U4 が持つ |
| C5 | すべての外のプログラムの呼び出し | `CdkSynthesizer`、`CdkSourceAnalyzer`、`PullRequestPublisher` | 渡す環境と時間の上限の決まり | U1 |
| C6 | `IacDisplay`(`display.Printer`)が実装する | `CdkSynthesizer`、`PullRequestPublisher` | それぞれのパッケージが定義する進み具合の通知口 | U1 |
| C7 | `IacWriter` | `IacDisplay`、`IacCommand` | `iac.Result` に足す欄と理由、リソースとソースの照合の決まり | U6(理由)、U1(欄と照合) |

U1 は最小の一本なので、C1〜C7 のそれぞれの最小の形を U1 で作り、持ち主の単位で仕上げます(作業の単位の依存の表のとおり)。

## X1. コマンド `iac`(足すオプションと終了コード)

```yaml
# cli contract
command: tag-refactor iac
existing_flags: [--plan, --dir, --tf-state, --write, --lang, --debug]   # 変えない
new_flags:
  --pr:            {type: bool, implies: --write}
  --cdk-app:       {type: string, repeatable: true, relative_to: --dir}
  --synth-timeout: {type: duration, default: 10m, min: 1s}
  --pr-branch:     {type: string, default: "tag-refactor/iac-<YYYYMMDD-HHMMSS>", requires: --pr}
  --pr-title:      {type: string, requires: --pr, non_empty: true, single_line: true, max_chars: 256}
  --pr-body-file:  {type: path, requires: --pr, max_size: 1MiB}
input_errors:      # 書き換えの前に確かめ、終了コード 1
  - --pr-* without --pr
  - --cdk-app not a folder under --dir, or no cdk.json in it
  - --synth-timeout not a duration, or < 1s
  - --pr-body-file missing, unreadable, larger than 1 MiB, not UTF-8
  - --pr-branch not a valid git branch name
  - --pr-title empty, has a line break, or longer than 256 characters
exit_codes:
  0: nothing to fix by hand (written, PR opened, or nothing to reflect)
  1: input error, check before writing failed, write failed, branch/commit/push/PR step failed
  2: something to fix by hand (PR opened with manual items, only manual items, a CDK app failed or was skipped)
precedence: 1 > 2 > 0
nothing_written: |
  --pr を付けても書き換えたファイルが無いときは、ブランチもコミットもプルリクエストも作らない(Publish を呼ばない、FR1.11)。
  手で直す項目が無ければ「反映するものはありません」と示して 0、あれば一覧を示して 2。
unchanged: without CDK apps and without --pr, output and exit codes stay as before (NFR5)
```

## X2. プルリクエストのタイトルと本文

```yaml
# document contract (GitHub Markdown)
title:
  default: "Reflect tag changes from tag-refactor (<plan file name>)"   # --lang に従う
  override: --pr-title
body:
  sections:
    - intro: 1-3 lines, says it was opened by tag-refactor iac --pr
    - changed_files: "## Changed files (N)" + one bullet per file: "`<path>` (<CDK, TypeScript | CDK, Python | CloudFormation | Terraform>)"
    - to_fix_by_hand: "## To fix by hand (N)" + one bullet per item: "`<file>:<line>` <resource name>, <key>: <reason text>"   # 0 件なら節ごと省く
  override: --pr-body-file   # ファイルの中身のあとに to_fix_by_hand の節を足す。ファイルの中身は変えない
  resource_name: CloudFormation と CDK は論理 ID(CDK はコンストラクトのパスの最後の要素)、Terraform はアドレス、synth に失敗したか外したアプリはアプリのフォルダーとリソースの種類
  language: --lang
reason_text: |
  ManualReason ごとの、表示の文の一覧(カタログ)にある固定の文だけを使う。
  cdkAppFailed のときは、固定の文に道具の名前(Tool: cdk | node | npx | python | typescript | helper)だけを足す。
  Failure.Detail と ManualItem.Detail のうち、外のプログラムや利用者のアプリが出した文は本文に入れない(R-03)。
never_contains: [ARN, AWS account ID, tag values, tokens, output of cdk synth or of the user's app]
redaction: |
  本文のうち tag-refactor が組み立てる部分(intro、changed_files、to_fix_by_hand)は、出す直前に次を伏せ字(<redacted>)にする。
  - ARN の形: arn:aws[a-z-]*:[^\s`)]*
  - 12 桁の数字の並び(前後が数字でないもの)
  --pr-body-file の中身は利用者が書いたものなので、変えない。
tests: |
  受け入れテストで、Failure.Detail と ManualItem.Detail に ARN、アカウント ID、トークンの形の文字を入れた結果から本文を作り、
  本文にそれらが現れないこと、理由の文がカタログの固定の文であることを確かめる。
```

## C1. プルリクエストの作成(PullRequestPublisher)

```yaml
# go-interface contract (in-process), package internal/pullrequest
Publisher:
  Check(ctx, CheckRequest, ProgressSink) -> (Plan, error)
    # 書き換えの前に呼ぶ。ネットワークを使わない確かめ(入力、HEAD、リモートの決定、リポジトリの根、
    # merge・rebase・cherry-pick・revert の途中でないこと)を先に、gh のログインとリモートのブランチの確かめを次に行う。
    # 最初に満たされない項目で止める。すべて通ったら、Publisher が Step(checked, done) を送る。
  Publish(ctx, Plan, PublishInput, ProgressSink) -> PublishOutcome
    # 書き換えのあとに、PublishInput.Files が空でないときだけ呼ぶ(X1 の nothing_written)。
    # 段階(branch、commit、push、create)ごとに Step(started) と Step(done | failed) を送る。
CheckRequest: {Dir, BranchName, Title, BodyFile, Now}
Plan:         {Remote, RemoteURL, Host, Base, Branch, Title, UserBody, RepoRoot, Prefix, GitDir}
  # RepoRoot は git rev-parse --show-toplevel、Prefix は --dir の RepoRoot からの相対(git rev-parse --show-prefix)、
  # GitDir は git rev-parse --absolute-git-dir。UserBody は --pr-body-file の中身(無ければ空)
PublishInput:
  Files: [string]   # IacWriter が書き換えたファイル(Result.WrittenFiles。--dir からの相対、/ 区切り)
  Title: string     # Plan.Title(--pr-title)か、IacDisplay が --lang で作った既定のタイトル
  Body: string      # IacDisplay が書き換えのあとに組み立てた、確定した本文(R-10 の2回目)
body_assembly: |
  本文は、書き換えが済んだあとに IacCommand が IacDisplay を呼んで組み立てる。
  入力は iac.Result(書き換えたファイルと種類、手で直す一覧)、--lang、Plan.UserBody。組み立ての決まりは X2。
  Publisher は本文を組み立てず、受け取った Body をそのまま渡す。
PublishOutcome:
  Remote: string
  Base: string
  Branch: string
  CommittedFiles: [string]                           # RepoRoot からの相対
  CompletedSteps: [branch | commit | push | create]
  FailedStep: branch | commit | push | create | ""   # "" なら成功
  URL: string                                        # 成功したとき
  BodyFile: string                                   # 失敗したとき、本文を書いたファイルの絶対パス
  ResumeCommands: [string]                           # 失敗したとき、続きの命令(1行に1つ、残りの段階の順)
  Err: error                                         # errkind で分類した原因。トークンを含めない
rules:
  - remote: 今のブランチの追跡先、無ければ origin。どちらも無ければ Check で止める
  - base: 実行したときのブランチ。リモートに無ければ Check で止める
  - paths: Publisher が files を RepoRoot からの相対に直す(Plan.Prefix を前に付ける)。git には `git -C <RepoRoot> --literal-pathspecs` で、`--` のあとに渡す
  - commit: |
      `git add -- <files>` のあと、`git commit --only -m <message> -- <files>` でコミットする。
      --only なので、利用者がすでにステージしていたほかのファイルはコミットに入らず、ステージされたまま残る(FR1.8、R-09)。
      フックと署名は利用者の設定のまま。
  - after: 成功しても失敗しても、手元は新しいブランチに切り替わったまま(元に戻さない)。表示でそのことを示す
  - body_file: |
      本文はいつも、コマンドの引数でなくファイルで gh に渡す(R-07)。
      Publish は create の段階の前に、本文を `<GitDir>/tag-refactor/pr-body-<名前>.md` に書き、`gh pr create --body-file <そのパス>` で渡す。
      <名前> はブランチ名の英数字、`.`、`_`、`-` 以外の文字を `-` にしたもの(Windows で使えない文字を含めない)。
      このフォルダーはコミットの対象にならない。成功したらファイルを消し、失敗したら残して PublishOutcome.BodyFile で示す。
  - resume: |
      続きの命令に本文を直接入れない。タイトルは1行で 256 文字までなので命令に入れる。
      引用符の付け方は、tag-refactor が動いている OS で決め、命令の前にどのシェル用かを示す。
      Windows は「PowerShell 用」と示し、PowerShell の単一引用符(中の ' は '' にする)を使う。
      それ以外は「sh 用」と示し、POSIX の単一引用符(中の ' は '\'' にする)を使う。
      cmd.exe や Git Bash 用の命令は作らない(表示の文で、PowerShell で実行するよう示す)。
  - timeouts: C5 の表のとおり。上限で止まった段階は FailedStep にし、ResumeCommands をその段階から作る
  - never: GitHub のトークンを読む、出力に出す
owner: U2(最小の形は U1)
```

## C2. CDK の synth(CdkSynthesizer)

```yaml
# go-interface contract (in-process), package internal/cdk
Synthesizer:
  Discover(ctx, root string, only []string) -> ([]App, error)
    # cdk.json を探す。.git、node_modules、cdk.out、シンボリックリンク、ジャンクションは見ない。
  Synthesize(ctx, App, Options, ProgressSink) -> (Synthesis, error)
    # 1つのアプリを synth する。失敗はエラーでなく Synthesis.Failure で返す(ほかのアプリを続けるため)。
    # Language が other のアプリと、CDKMajor が 1 か 0 のアプリは、synth を動かさずに Failure を返し、AppSkipped を送る。
    # それ以外は AppStarted を送り、終わったら AppDone か AppFailed を送る。
    # AppsWaiting は、Discover のあとに IacWriter が、synth するアプリの一覧で1回送る。
App:       {Dir, Language: typescript | python | other, CDKMajor: 1 | 2 | 0}
detection: |
  Language は cdk.json の "app" の命令から決める(R-12 の2回目)。
  - 命令が node、npx、ts-node、tsx のどれかで始まるか、.ts や .js のファイルを名指すなら typescript(JavaScript のアプリも TypeScript 用の補助のプログラムで読む)
  - 命令が python、python3、py のどれかで始まるか、.py のファイルを名指すなら python
  - それ以外(mvn、dotnet、go など)は other
  CDKMajor は、アプリのフォルダーから --dir までさかのぼって、最初に見つかった依存の宣言から決める。
  - typescript: package.json の dependencies か devDependencies に aws-cdk-lib があれば 2、@aws-cdk/core があれば 1
  - python: requirements.txt、setup.py、pyproject.toml に aws-cdk-lib があれば 2、aws-cdk.core があれば 1
  - どれも見つからなければ 0(分からない)。0 のアプリは synth せず、unknownVersion で外す
needs_aws: |
  synth が 0 以外で終わり、その出力が、AWS の認証情報やアカウントの問い合わせを求める既知の文(一覧は U3 の設計で決め、テストで固定する)に当たれば needsAWS。
  当たらなければ synthError。
Options:   {Timeout, WithTrace bool}   # WithTrace は作成の記録を出す設定(ADR-004)
Synthesis:
  App: App
  Stacks: [Stack]
  TraceSites: [{ConstructPath, File, Line}]   # WithTrace のときだけ。作成の記録から読んだ、ソースの箇所(R-05)。File はアプリのフォルダーからの相対、/ 区切り
  Elapsed: duration
  Failure: {Reason: toolMissing | needsAWS | timeout | cdkV1 | unknownVersion | unsupportedLanguage | synthError, Tool: string, Detail: string} | null
    # Detail は外のプログラムの出力から作る。端末には --debug のときだけ出し、プルリクエストの本文には入れない(X2)
Stack:
  Name: string      # 実際にデプロイされるスタック名(クラウドアセンブリの manifest の stackName。無ければ成果物の ID)。
                    # 変更案のタグ aws:cloudformation:stack-name と比べる値(R-02)
  Account: string   # manifest の environment から。決まっていない(unknown-account)なら ""
  Region: string    # 同じく。unknown-region なら ""
  Resources: [SynthesizedResource]
SynthesizedResource: {LogicalID, Type, ConstructPath, Tags: map[string]string}
  # Tags は synth のテンプレートの Properties.Tags から読んだ値。スコープのタグの判断(FR3.5)に使う。
rules:
  - output: 一時的なフォルダー(os.MkdirTemp)。成功、失敗、中断のどれでも消す。利用者の cdk.out は変えない
  - environment: C5
  - timeout: Options.Timeout(既定 10m)。超えたら子のプロセスを止めて Failure{timeout}
owner: U3(最小の形は U1)
```

## C3. CDK のソースの読み取り(CdkSourceAnalyzer)

```yaml
# go-interface contract (in-process), package internal/cdk
Analyzer:
  Analyze(ctx, App, AnalyzeRequest) -> (Analysis, error)
    # アプリごとに1回、補助のプログラムを起動する(C4)。
  Recheck(ctx, App, edited map[string][]byte) -> (Analysis, error)
    # 書き換え後の内容を渡して読み直す。ディスクには書かない(ADR-005、CD-Q1)。
AnalyzeRequest:
  ConstructPaths: [string]                      # synth から得た、対象のリソースのコンストラクトのパス
  TraceSites: [{ConstructPath, File, Line}]     # Synthesis.TraceSites をそのまま渡す(無ければ空)
Analysis:
  TagSites: [TagSite]
  ConstructSites: [ConstructSite]
  InsertSites: [InsertSite]
  Unsupported: [{File, Line, Reason}]   # Reason: unsupportedShape | parseError など。parseError はそのファイルだけを外す
TagSite:
  UseFile: string          # タグを付けている呼び出しのあるファイル。アプリのフォルダーからの相対、/ 区切り
  UseLine: int
  Op: add | remove | stackTags | l1Tags | other
  ScopeConstructPath: string          # このタグが効く範囲
  KeyEdit:    Edit                    # キー名を変えるときに書き換える範囲
  ValueEdit:  Edit | null             # 値を変えるときに書き換える範囲。null は Op が remove のときだけ
  RemoveEdit: Edit                    # このタグを消すときに消す範囲(文の全体、または辞書や配列のエントリと区切りのコンマ)
Edit:
  File: string             # 書き換えるファイル(アプリのフォルダーからの相対、/ 区切り)
  Start: int               # UTF-8 のバイトの位置(CD-Q2)
  End: int
  Shape: literal | sameFileConst | sameFileMap | crossFileArg | expression   # この範囲の書き方(R-15 の1回目の試み)
  Text: string             # 今の文字列の中身(引用符を除く)。Shape が expression なら空
  SharedWith: [string]     # この範囲を使う、ほかのタグの範囲(ScopeConstructPath)。FR3.5 の判断に使う
InsertSite:                # 必須タグを足す場所(AC3.1.1、AC3.2.1 の追加)
  ConstructPath: string    # 足したタグが効く範囲の根。この下のリソースすべてにタグが付く
  Kind: mapEntry | arrayEntry | tagsOfStatement
  File: string
  At: int                  # 挿入するバイトの位置
  Indent: string           # 挿入する行の字下げ(周りの行に合わせる)
  Quote: "'" | '"'         # 周りの文字列の引用符
  TrailingComma: bool      # mapEntry と arrayEntry で、最後のエントリのあとにコンマを置く流儀か
  Target: string           # tagsOfStatement のとき、コンストラクトを指す変数や式(例: bucket、self.bucket)。それ以外は空
ConstructSite: {ConstructPath, File, Line, ConstructID, From: source | trace}
edit_rules: |
  KeyEdit、ValueEdit、RemoveEdit は、それぞれ独立に書き換え先を持つ(R-01)。
  - literal: 使う場所の文字列そのもの。
  - sameFileConst、crossFileArg: たどった先の、元の文字列。
  - sameFileMap(辞書を回してタグを付ける書き方): 辞書のエントリごとに TagSite を1件作り、KeyEdit、ValueEdit、RemoveEdit はそのエントリのキー、値、エントリ全体を指す。
change_to_edit: |
  変更の種類ごとに使う範囲は次のとおり。使う範囲の Shape が expression のときだけ、cdkExpression で手で直す一覧に出す。
  - renameKey: KeyEdit(値が式でも、キーが文字列なら反映できる)
  - changeValue: ValueEdit
  - removeKey: RemoveEdit
  - addRequired: InsertSite
insert_rules: |
  必須タグの追加は、対象のリソースのコンストラクトのパスを含む InsertSite のうち、いちばん深いものを使う。
  - mapEntry と arrayEntry: そのリソースにすでにある、文字列の辞書(stack の tags など)や配列(L1 の tags)の末尾に、エントリを1つ足す。
  - tagsOfStatement: コンストラクトを作る文が変数に入れているとき、その文の次に `Tags.of(<Target>).add(<key>, <value>)` の文を足す。
  - 足す文字列は、Go が言語ごとの文字列の書き方(引用符と、\ による逃がし)で作る。補助のプログラムは位置と流儀だけを返す。
  - InsertSite.ConstructPath の下に、変更案にない、または同じ追加を持たないリソースがあれば、書き換えずに cdkScopeTag で手で直す一覧に出す(FR3.5 と同じ考え方)。
  - 使える InsertSite が無いとき(コンストラクトを変数に入れていないなど)は、cdkNoInsertSite で手で直す一覧に出す。
  - 書き換え後の Recheck で、足したタグの TagSite がそのリソースの範囲に現れることを確かめる。
construct_rules: |
  ConstructSite は、ソースをたどって見つけたもの(From: source)を先に使う。
  ソースで見つからないコンストラクトのパスだけ、TraceSites の箇所(From: trace)を使う(ADR-004、R-05)。
owner: U4(TypeScript と共通の形)、U5(Python)
```

## C4. 補助のプログラムの JSON(Go と TypeScript 用、Python 用)

```yaml
# shared-schema contract (stdin/stdout JSON, one request per process)
request:
  version: 1                         # 結果の version が違えば Go は止める(CD-Q3)
  mode: analyze | recheck
  appDir: string                     # 絶対パス
  constructPaths: [string]           # analyze のとき
  traceSites: [{constructPath, file, line}]   # analyze のとき(無ければ空)
  files: [{path: string, content: string}]    # recheck のとき。content は UTF-8 の書き換え後の内容
response:
  version: 1
  tagSites: [TagSite]                # C3 と同じ形(キーは lowerCamelCase)
  constructSites: [ConstructSite]
  insertSites: [InsertSite]
  unsupported: [{file, line, reason}]   # 読めないファイルは reason: parseError でここに入れ、ほかのファイルは読み続ける
  error: {kind: noTypeScript | internal} | null   # アプリ全体を読めないときだけ
rules:
  - the helper reads only; it never writes files
  - positions are UTF-8 byte offsets (TypeScript の UTF-16 の位置は補助のプログラムが直す)
  - stdout is the JSON only; diagnostics go to stderr and are not shown except with --debug (ソースの中身は出さない)
  - the helper never echoes source text except Edit.text of tag sites
  - typescript: uses the app's node_modules/typescript; if missing, error.kind = noTypeScript
  - python: uses the standard ast module only
  - timeout: C5 の表のとおり
owner: U4(形)、U5(Python の実装)
```

## C5. 外のプログラムに渡す環境と時間の上限

```yaml
# rule contract (applies to cdk, node, python run by CdkSynthesizer and CdkSourceAnalyzer)
remove_from_parent_env:
  - AWS_*                          # AWS_PROFILE、AWS_ACCESS_KEY_ID、AWS_SESSION_TOKEN などすべて
  - GH_TOKEN, GITHUB_TOKEN, GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN
set:
  AWS_SHARED_CREDENTIALS_FILE: <empty temp file>
  AWS_CONFIG_FILE: <empty temp file>
  AWS_EC2_METADATA_DISABLED: "true"
  CDK_DISABLE_VERSION_CHECK: "1"   # 外への版の問い合わせを止める(設計で確かめる)
keep: PATH, HOME/USERPROFILE, その他の環境(Node.js と Python を動かすのに要るもの)
git_and_gh: |
  PullRequestPublisher が呼ぶ git と gh には利用者の環境をそのまま渡す(認証は gh と git のもの)。
  tag-refactor はその値を読まない、出さない。
  コミットと push では、フックと署名(パスフレーズの入力など)が利用者とやり取りできるよう、標準入力と標準エラーを端末につなぐ。
timeouts:          # NFR3。上限を超えたら子のプロセスを止め、errkind の timeout で返す(R-06)
  cdk synth:                     --synth-timeout(既定 10m)
  helper (analyze, recheck):     2m(案。設計で大きなアプリの時間を確かめる)
  git (rev-parse, ls-remote など読むだけのもの、switch -c、add): 30s
  git commit:                    10m(利用者のフックと署名を含むため長くする)
  git push:                      10m(pre-push のフックを含むため長くする)
  gh auth status, gh pr create:  2m
  configurable: v0.2.0 では synth だけを変えられる。ほかは固定
test: |
  差し替えた実行の口で、渡した環境に remove_from_parent_env の変数が無く、set の値があることを確かめる。
  応答しない差し替えのプログラムで、それぞれの上限のあとに止まることを確かめる(上限は差し替えで短くしてテストする)。
owner: U1
```

## C6. 進み具合の通知口

前の作業では、使う側のパッケージが自分の通知口を定義し、`display.Printer` がそれを実装しています(`inventory.ProgressSink`、`apply.ProgressSink`)。この流儀に合わせ、既存の2つの通知口は変えず、新しいパッケージがそれぞれ小さな通知口を持ちます(R-04)。

CD-Q5 の回答は A(今の `ProgressSink` に知らせを足す)でした。しかし `iac` 用の `ProgressSink` は無く、既存の2つに足すとその実装がすべて壊れることがレビュー(R-04)で分かりました。そのため、知らせの中身(アプリの状態とプルリクエストの段階)は CD-Q5 のとおりにし、置き場所だけを新しい通知口に改めました。人の依頼(指摘をすべて直す)にもとづく変更です。

```yaml
# go-interface contract (in-process)
cdk.ProgressSink:                      # package internal/cdk
  AppsWaiting(apps []string)           # synth を待つアプリの一覧(FR2.7 の「待ち」)
  AppStarted(app string, language string)
  AppDone(app string, elapsed duration)
  AppFailed(app string, elapsed duration, reason string)   # reason は Failure.Reason の値。Detail は渡さない
  AppSkipped(app string, reason string)                    # unsupportedLanguage、cdkV1、unknownVersion
pullrequest.ProgressSink:              # package internal/pullrequest
  Step(step string, status string, detail string)
    # step: checked | branch | commit | push | create
    # status: started | done | failed
    # detail: ブランチ名、URL などの表示に要るもの。トークンと本文は渡さない
implementer: display.Printer(IacDisplay)。IacCommand が作り、メソッドの引数で渡す(C1 の Check と Publish、C2 の Synthesize。AppsWaiting は IacWriter が送る)
written_step: 書き換えが済んだことは IacWriter の結果から IacCommand が表示する(通知口は使わない)
owner: U1
```

## C7. `iac.Result` に足すものと、リソースとソースの照合

```yaml
# go-type contract (in-process), package internal/iac
Tool: add ToolCDK                 # 3つ目の IaC
FileResult.Tool: ToolCDK and FileResult.Language: typescript | python   # CDK のとき
Result:
  CDKApps: [{Dir, Language, Status: done | failed | skipped, Elapsed, FailureReason}]
  WrittenFiles: [string]          # --write で書いたファイル(PullRequestPublisher.Publish に渡す)
ManualReason (add):
  cdkExpression           # 値が式
  cdkScopeTag             # スコープ全体のタグで、変更案がスコープのすべてのリソースを同じように変えない
  cdkSharedValue          # 同じ元の文字列をほかのリソースも使う。または、同じ箇所に対応する複数のリソースの変更が違う
  cdkAmbiguous            # 1つのリソースに、複数のアプリまたは複数の箇所が対応する
  cdkAppFailed            # アプリの synth の失敗(本文には道具の名前だけ)
  cdkUnsupportedLanguage  # TypeScript と Python 以外の CDK のアプリ
  cdkNoSource             # 作成の箇所が分からない
  cdkNoInsertSite         # 必須タグを足す場所が無い(コンストラクトを変数に入れていないなど)
  cdkV1                   # CDK v1 のアプリ、または CDK の版が分からないアプリ
  cdkUnsupportedShape     # 対応していない書き方
failure_to_manual:        # Synthesis.Failure.Reason と CDKApps.Status から ManualReason への対応(R-08)
  toolMissing:         {status: failed,  manual: cdkAppFailed}
  needsAWS:            {status: failed,  manual: cdkAppFailed}
  timeout:             {status: failed,  manual: cdkAppFailed}
  synthError:          {status: failed,  manual: cdkAppFailed}
  cdkV1:               {status: skipped, manual: cdkV1}
  unknownVersion:      {status: skipped, manual: cdkV1}
  unsupportedLanguage: {status: skipped, manual: cdkUnsupportedLanguage}
helper_failure_to_manual: # 補助のプログラム(Analyze と Recheck)の失敗の対応(1回目の試みの R-13)
  noTypeScript:          {status: failed, manual: cdkAppFailed, tool: typescript}   # そのアプリのリソースはすべて手で直す一覧へ
  node_or_python_missing: {status: failed, manual: cdkAppFailed, tool: node | python}
  helper_timeout:        {status: failed, manual: cdkAppFailed, tool: helper}
  internal:              {status: failed, manual: cdkAppFailed, tool: helper}
  parseError:            {status: done,   manual: cdkNoSource}   # 読めないファイルだけを Result.SkippedFiles に入れ、そのファイルに作成の箇所があるリソースを手で直す一覧へ
  version_mismatch:      終了コード 1 で止める(埋め込みの不具合なので、書き換えの前に止める)
  recheck_failed:        そのアプリのファイルは書かず、変更を cdkUnsupportedShape で手で直す一覧へ(ADR-005)
match_rule: |
  変更案のリソースは、タグ aws:cloudformation:stack-name(S)と aws:cloudformation:logical-id(L)、変更案の Plan.AccountID(A)、
  リソースの Region(R)で照合する(R-02)。ARN からはアカウントもリージョンも取らない(S3 や IAM の ARN には入っていないため)。
  リソースが Global のときと Region が空のときは、リージョンを照合に使わない。A が空のときは、アカウントを照合に使わない。
  候補は、成功したアプリの Stack のうち、Name = S、LogicalID = L で、Account が "" か A、Region が "" か R(使わない軸はどの値でも合う)のもの。
  - 候補が1つ: そのリソースのコンストラクトのパスで、ソースの箇所を探す。
  - 候補が2つ以上で、違うアプリか違うコンストラクトのパス: 黙ってどれかを採らず、cdkAmbiguous で手で直す一覧に出す。
  - 候補が0: まず今までどおり CloudFormation のテンプレートを探す。見つからず、失敗したか外したアプリがあれば、
    failure_to_manual の理由で手で直す一覧に出す(どのアプリのスタックか分からない場合も)。どちらも無ければ、今までどおりの扱い。
match_tests: |
  受け入れテストで、環境(アカウントとリージョン)を決めたスタックの S3 バケット(ARN にアカウントもリージョンも無い)と
  IAM ロール(ARN にリージョンが無い)が、正しく照合されることを確かめる。
same_site_rule: |
  環境を決めないスタックを複数のリージョンやアカウントに置いたときなど、変更案の複数のリソースが同じ Edit に対応する場合、
  それらの変更(種類、キー、値)がすべて同じなら1回だけ書き換え、1つでも違えば cdkSharedValue で手で直す一覧に出す。
synth_rule: 変更案に aws:cloudformation:stack-name のタグのリソースが1つも無いときは、cdk.json があっても synth しない(NFR5)
exit_code_rule: |
  Result.ExitCode() は、今の判断(手で直す項目があれば 2)に加え、CDKApps に failed か skipped のアプリがあれば 2 を返す(X1)。
  変更案のリソースがそのアプリに対応しなくても、synth できなかったアプリがあることを利用者に知らせるため。
owner: U1(欄と照合)、U6(理由の文)
```

## 契約の決まり

- 持ち主: 各契約の持ち主の単位が形を決めます。変えるときは、使う側の単位のテストも直します。1つのバイナリの中なので、型が合わなければビルドで分かります。
- 補助のプログラムの JSON(C4)は版を持ち、版が違えば止めます。補助のプログラムはバイナリに埋め込むので、ふつうは同じ版がそろいます。
- 足すだけの変更で使う側を壊さないのは、構造体の欄、列挙の値、JSON の欄を足す場合に限ります。JSON の知らない欄は、Go の側では無視します。Go のインタフェース(C1、C2、C3、C6)にメソッドを足すと、すべての実装(テストの差し替えを含む)が壊れます。そのため、メソッドを足すときは、同じ変更で実装をすべて直します(R-04)。
- 利用者が触る契約(X1、X2)を変えるときは、README の `iac` の節を同じプルリクエストで直します。
- 前の作業の契約(変更案の JSON、`inventory.ProgressSink`、`apply.ProgressSink`、`errkind`)は変えません。

## レビューへの対応(1回目)

| 指摘 | 直したところ |
|---|---|
| R-01 | C3 の `Origin` をやめ、`KeyEdit` と `ValueEdit` がそれぞれ書き換え先(ファイル、範囲、今の文字列、共有する範囲)を持つ形にした。書き方ごとの書き換え先と、辞書のエントリごとに1件とする決まりを `edit_rules` に書いた |
| R-02 | C2 の `Stack` に、実際のスタック名(`Name`)とアカウント、リージョンを持たせた。C7 の `match_rule` で照合の鍵と、候補が複数のときの `cdkAmbiguous` を決めた。同じ箇所に複数のリソースが対応するときの扱いは `same_site_rule` に書いた |
| R-03 | X2 で、理由の文をカタログの固定の文と道具の名前だけにし、外のプログラムの出力を本文に入れないことにした。ARN と 12 桁の数字を伏せ字にする決まりと、それを確かめる受け入れテストを足した |
| R-04 | C6 を、既存の流儀に合わせて `cdk.ProgressSink` と `pullrequest.ProgressSink` の2つの新しい通知口に直した。待ちと外したことの知らせ、段階の成否を足した。「足すだけの変更」の決まりは、構造体の欄、列挙の値、JSON の欄に限ると書き直した |
| R-05 | C2 の `Synthesis` に `TraceSites` を足し、C3 の `AnalyzeRequest` と C4 の要求で受け取る形にした。ソースをたどった結果を先に使う優先順位を `construct_rules` に書いた |
| R-06 | C5 に、外のプログラムごとの時間の上限の表を足した。コミットと push は利用者のフックと署名を含むので長くし、上限で止まった段階は C1 の `FailedStep` と `ResumeCommands` で扱う |
| R-07 | C1 で、本文を Git の管理フォルダーの中のファイルに書き、`--body-file` で渡すことにした。タイトルは1行で 256 文字までとし(X1)、OS ごとの引用符の付け方を決めた |
| R-08 | C7 に、`Failure.Reason` から `CDKApps.Status` と `ManualReason` への対応の表を足した。`other` のアプリは synth せずに外し、`cdkUnsupportedLanguage` とする |
| R-09 | C1 で、`git add` のあと `git commit --only -- <files>` でコミットすることにした。ほかのステージ済みのファイルは入らない。`files` をリポジトリの根からの相対に直すのは Publisher の責任とした |

## レビューへの対応(2回目)

2回目のレビューは書式の誤りで一度やり直しました。人の依頼に従い、最初の試みで出た指摘と、やり直したレビューの指摘をすべて直しました。

| 指摘 | 直したところ |
|---|---|
| 最初の試み: 必須タグの追加の契約が無い | C3 に `InsertSite` と `insert_rules` を足し、C4 の結果に `insertSites` を足した。足す場所が無いときの `cdkNoInsertSite` を C7 に足した |
| 最初の試み: アカウントとリージョンを ARN から取っている | C7 の `match_rule` を、変更案の `Plan.AccountID` とリソースの `Region`、`Global` を使う形に直した。S3 と IAM の受け入れテストを `match_tests` に足した |
| 最初の試み: 補助のプログラムの失敗の対応 | C7 に `helper_failure_to_manual` を足した。読めないファイルは、そのファイルだけを外す(C3、C4) |
| 最初の試み: アプリの失敗と終了コード、候補0件の順番 | C7 に `exit_code_rule` を足し、候補が0件のときは CloudFormation のテンプレートを先に探す順番に直した |
| 最初の試み: 式のタグの扱い | `Shape` を `Edit` ごとに持たせ、変更の種類ごとに使う範囲を `change_to_edit` で決めた。`ValueEdit` の null は remove のときだけにし、消す範囲 `RemoveEdit` を足した |
| 最初の試み: Q5 の記録との食い違い | C6 の前置きに、CD-Q5 の回答と置き場所を改めた理由を書いた |
| R-10 | C1 に `PublishInput` と `body_assembly` を足し、本文は書き換えのあとに IacDisplay が組み立てて Publish に渡すことにした |
| R-11 | C1 の `Check` と `Publish` が通知口を引数で受け取り、`checked` は Check が通ったときに Publisher が送ることにした。C2 と C6 にも、誰がどの知らせを送るかを書いた |
| R-12 | C2 に、`Language` と `CDKMajor` の判定元(`detection`)と、`needsAWS` の見分け方を足した。版が分からないアプリは synth せずに外す |
| R-13 | X1 に `nothing_written` を足した。書き換えたファイルが無ければ Publish を呼ばない |
| R-14 | C1 の `resume` で、どのシェル用の命令かを示し、PowerShell で実行するよう案内することにした。本文のファイル名から、Windows で使えない文字を除く決まりを足した |

## 未決事項

| 契約 | 問い | 止まる単位 |
|---|---|---|
| C2 | 作成の記録を出す設定(`WithTrace`)の名前と出力の形を、CDK v2 の文書で確かめる | U3 |
| C2 | manifest の `stackName` と `environment` を読む場所を、CDK v2 のクラウドアセンブリの形で確かめる | U1 |
| C2 | `needsAWS` と見分ける、AWS の認証情報や問い合わせを求める文の一覧 | U3 |
| C3 | `InsertSite` の流儀(字下げ、引用符、コンマ)の読み取り方と、Python の `Tags.of(...).add(...)` の文の形 | U4、U5 |
| C5 | 補助のプログラムの呼び出しの時間の上限の既定値(案は 2 分)と、大きなアプリでの時間 | U1、U4 |
| C5 | `CDK_DISABLE_VERSION_CHECK` などで、synth が外に問い合わせることを止め切れるかを確かめる | U1 |
| C4 | 補助のプログラムを埋め込む形(Go の embed で一時ファイルに書いて実行する、など)と、そのライセンスの表示 | U1 |
| X1 | 既定のブランチ名の日時は端末の時刻か UTC か | U2 |
