# セキュリティの設計: U2 プルリクエストの作成(残り)

U1 のセキュリティの設計(`construction/u1-walking-skeleton/nfr-design/security-design.md`)は、U2 にもそのまま当てます。ここでは、U2 の非機能の要件(`construction/u2-pull-request/nfr-requirements/security-requirements.md` の NFR1.6〜NFR1.10、NFR9.2 と、`tech-stack-decisions.md` の NFR6.11〜NFR6.15)を、どの部品でどう満たすかを書きます。機能の流れは `construction/u2-pull-request/functional-design/functional-spec.md`、契約は `inception/contract-design/contract-summary.md` の X1、X2、C1 です。出典の ND2-Q… は、このステージの `nfr-design-questions.md` の回答です。型と呼ぶ場所の変更の一覧は `logical-components.md` にあります。

## 1. 守るものと境目

U2 で新しく境目を越えるものは3つです。

| もの | どこからどこへ | 守り方 |
|---|---|---|
| 利用者の本文(`--pr-body-file`) | 利用者のファイル → 公開のプルリクエスト | 書き換えの前に形を確かめ、止めるか知らせる(2節)。伏せ字は通さない(3節) |
| 続きの命令 | tag-refactor → 利用者の端末の標準エラー → 利用者が貼り付けるシェル | 本文を入れず、値をシェルごとに囲み、囲めない値は手順の文にする(4節) |
| 本文のファイル | tag-refactor → `<Git の管理フォルダー>/tag-refactor/` | 重ならない名前で自分だけが読めるように作り、成功したら消す(5節) |

## 2. 利用者の本文の読み取りと形の確かめ(NFR1.6、NFR1.7、NFR6.11、NFR6.14)

### 読み取り(BR1.1)

本文は `pullrequest.Check` の `checkLocal` の最初(タイトルの確かめの直後、git を呼ぶ前)で読みます。契約 C1 の `CheckRequest.BodyFile` と `Plan.UserBody` をそのまま使います。`internal/cli` は、`--pr-body-file` を動かしたフォルダーからの絶対のパスに直して(FD2-Q3)、`CheckRequest.BodyFile` に入れます。

1. `os.Stat` で、ふつうのファイル(`Mode().IsRegular()`)でなければ入力の誤り(名前付きパイプ、デバイス、フォルダーを断る)。
2. 開いて、`io.ReadAll(io.LimitReader(f, 1<<20+1))` で読み、1 MiB を超えたら入力の誤り。
3. `utf8.Valid` でなければ入力の誤り。
4. `utf8.RuneCountInString` が 60,000 を超えたら入力の誤り(NFR6.14)。
5. `redact.Scan` で形を確かめる(下)。

どれも書き換えの前で、git も gh もまだ呼んでいません。誤りは `errkind.InvalidInput` で、終了コード 1 です。

### 形の確かめ

`internal/redact` に、次の関数を足します。値は返さず、形の種類と行の番号だけを返します。

```go
// Finding は本文の中に見つけた形の1種類。
type Finding struct {
    Kind  string // githubToken | accessKeyID | urlPassword | arn | accountID
    Stop  bool   // true なら入力の誤りとして止める
    Lines []int  // その形がある行の番号(1 から)。昇順で、同じ行は1回だけ
}

// Scan は利用者の本文を確かめる。返す順は上の Kind の順で固定。見つからない種類は返さない。
func Scan(text string) []Finding
```

| Kind | Stop | 形(Go の `regexp`) | 境目の確かめ |
|---|---|---|---|
| `githubToken` | true | 6つの接頭辞のどれか(`ghp_`、`gho_`、`ghu_`、`ghs_`、`ghr_`、`github_pat_`)のあとに `[A-Za-z0-9_]{20,}` | 一致の直前の1バイトが英数字か `_` なら数えない |
| `accessKeyID` | true | `AKIA` か `ASIA` のあとに `[A-Z0-9]{16}` | 一致の直前と直後の1バイトが英大文字か数字なら数えない |
| `urlPassword` | true | `://[^/@\s:]+:[^/@\s]+@` | なし |
| `arn` | false | 伏せ字と同じ `arn` の形 | なし |
| `accountID` | false | 伏せ字と同じ、前後が数字でないちょうど12桁の数字 | なし |

- 行は `\n` だけで区切ります(`\r\n` の `\r` は行の末尾の文字として扱い、`\r` だけの区切りは行と数えない)。1つの形が行をまたぐことはありません。
- 境目の確かめで数えなかった一致があったときは、その一致の先頭の次のバイトから、同じ行を探しなおします(`FindAllStringIndex` をそのまま使わず、`FindStringIndex` を位置をずらしながら呼ぶ)。数えなかった一致と重なる本物の形を見落とさないためです。
- 「か所」は行の数です。同じ行に同じ形が何度あっても1か所と数えます。表示は種類ごとに1行で、行の番号は先頭の10個までと「ほか N か所」(N は残りの行の数)です。

### 止めるときと知らせるとき

- `Stop` の Finding が1つでもあれば、`checkLocal` は `errkind.InvalidInput` の誤り(表示の文の ID は `iac.prBodySecret`)を返します。誤りには Finding の種類と行の番号だけを持たせます。
- `Stop` が無ければ、知らせの Finding を `Plan.BodyNotices` に入れて進みます。`internal/cli` は `Check` が通ったあと、書き換えの前に、知らせの文(ID は `iac.prBodyNotice`)を種類ごとに1行、標準エラーに出します。
- 表示の文と形の名前(`GitHub token`、`AWS access key ID`、`URL with a password`、`ARN`、`12-digit number` と日本語の名前)は、`internal/display` の文の一覧(カタログ)に、英語と日本語で足します。
- `--debug` にも、種類と行の番号だけを出します。本文の行そのものは出しません。

## 3. 部品ごとの伏せ字(NFR1.8)

今のコードは、`display.PullRequestBody` が組み立てた本文の全体に最後に1回 `redact.String` を通し、`Publish` もタイトルと本文に `redact.String` を通しています。U2 では、伏せ字を部品ごとに通す形に変えます。

| 部品 | どこで伏せ字を通すか | 変えるところ |
|---|---|---|
| `--pr-title` か既定のタイトル | `pullrequest.checkLocal` の最初に、1回だけ `redact.String` を通し、そのあとで `checkTitle`(空でない、改行なし、256 文字まで)を確かめる。変わったら `Plan.TitleRedacted` を true にし、cli が標準エラーに1行(`iac.prTitleRedacted`)を出す | `Plan.Title` には通したあとの文を入れる |
| 本文の導入(1〜3行) | `display.PullRequestBody` の中で、その部品の文字列に `redact.String` を通す | 新しく部品ごとに通す |
| 変更したファイルの一覧(見出しと行) | 同じく、その部品に通す | 同上 |
| 手で直す一覧(見出しと行、省いた件数の行) | 同じく、その部品に通す。上限での省略(BR3.3)は通したあとの長さで数える | 新しく組み立てる(U1 には無い) |
| 利用者の本文 | 通さない | `PullRequestBody` の引数に `userBody string` を足し、空でなければ、導入の代わりに先頭にそのまま置く(末尾に改行が無ければ1つ足し、空行を1つ挟む) |
| 本文の全体 | 通さない | `PullRequestBody` の最後の `redact.String(b.String())` をやめる |
| コミットのメッセージ | `PublishInput.Title`(= `Plan.Title`)をそのまま使う | 今のまま |
| Publish | 何も通さない | `title, body := redact.String(in.Title), redact.String(in.Body)` をやめ、`in.Title` と `in.Body` をそのまま使う |

`PullRequestBody` の新しい形は次のとおりです。

```go
// PullRequestBody は本文を組み立てる。手で直す一覧の後ろを省いても 65,536 文字(コードポイント)を
// 超えるときは、ok を false にして返す(BR3.3)。そのとき cli は Publish を呼ばず、書き換えたファイルは
// そのままに、理由と手で作る手順(iac.prBodyTooLong)を示して終了コード 1 にする。
func PullRequestBody(lang Lang, res *iac.Result, prefix, userBody string) (body string, ok bool)
```

単体テストで確かめること: (a) 変更したファイルの名前と手で直す一覧の理由に ARN と 12 桁の数字を入れると、その部品の中で伏せ字になる。(b) 同じ並びを利用者の本文に入れると、本文の中でそのまま残る。(c) Publish は受け取ったタイトルと本文を1文字も変えずに gh とコミットに渡す。受け入れテストで、`--pr-title` の ARN が偽の gh の `--title` とコミットのメッセージで伏せ字になることを確かめます。

## 4. 続きの命令(NFR1.9、NFR6.15、ND2-Q1)

`internal/pullrequest` に `resume.go` を足します。`PublishOutcome.ResumeCommands []string`(U1 で「U2 のため」とした欄)は消し、続きの命令は `internal/cli` が Publish の失敗のあとで次の関数を呼んで作ります。

```go
type Shell int // ShellSh か ShellPowerShell

// DefaultShell は、Windows で環境の変数 MSYSTEM が無ければ ShellPowerShell、それ以外は ShellSh。
func DefaultShell() Shell

// ResumeLine は続きの命令の1行。Command が空なら、Manual の値を手順の文として示す。
type ResumeLine struct {
    Step    string   // branch | commit | push | create(commit は add と commit の2行)
    Command string   // シェルにそのまま貼れる1つの命令
    Manual  []ManualValue
}
type ManualValue struct{ Name, Value string }

func ResumeCommands(plan Plan, title string, out PublishOutcome, sh Shell) []ResumeLine
func quoteSh(v string) (string, bool)
func quotePowerShell(v string) (string, bool)
```

- 失敗した段階(`out.FailedStep`)から作成までを、次の命令で作ります。ファイルは `out.CommittedFiles`(リポジトリの根からの相対)を使います。値は伏せ字を通しません。タイトルは `plan.Title`(2節で伏せ字を通したもの)です。

| 段階 | 行 | 命令 |
|---|---|---|
| branch | 1 | `git -C <根> switch -c <ブランチ>` |
| commit | 1 | `git -C <根> --literal-pathspecs add -- <ファイル…>` |
| commit | 2 | `git -C <根> --literal-pathspecs commit --only -m <タイトル> -- <ファイル…>` |
| push | 1 | `git -C <根> push <リモート> refs/heads/<ブランチ>:refs/heads/<ブランチ>` |
| create | 1 | `gh pr create --repo <owner/name> --base <行き先> --head <ブランチ> --title <タイトル> --body-file <本文のファイル>` |

- 1行に1つの命令だけを置き、`&&` や `;` でつなぎません(Windows PowerShell 5.1 は `&&` を読めないため)。commit の段階は add と commit の2行で、表示は「上から順に、1行ずつ実行してください」と添えます。
- 「表示できない文字」は、`display.Sanitize` が見える形に置き換える分類と同じく、Unicode の Cc(C0 と C1 の制御文字、DEL)、Cf(双方向の制御文字など)、Zl、Zp の文字です。
- `quoteSh`: 表示できない文字があれば `false`。ほかは `'` で囲み、中の `'` を `'\''` にします。
- `quotePowerShell`: 表示できない文字、`"`、または空白を含んで末尾が `\` なら `false`。ほかは `'` で囲み、中の `'`、U+2018、U+2019、U+201A、U+201B をそれぞれ2つ重ねます。
- 1行の命令のどれかの値が `false` なら、その行は `Command` を空にし、`Manual` に、その行で使う値を「名前と値」で置きます。ほかの行には影響しません。
- 本文のファイルが無いとき(5節で作れなかったとき)は、create の行を `Manual` にし、「`gh pr create` を `--body-file` なしで動かすと、エディターで本文を書ける」と示します。
- 表示は `internal/display` の `PullRequestStopped(out, lines)` で、命令の前に「PowerShell 用」か「sh 用」を示します(FD2-Q4: いつも標準エラー)。命令の行は `Sanitize` を通さずにそのまま出します(通すと `\` が二重になって貼れなくなる。表示できない文字を含む値は、すでに `Manual` に回っている)。`Manual` の値は `strconv.QuoteToGraphic` で、制御文字を `\x1b` のような見える形にしてから出します。端末の表示を書き換えられないようにし、利用者が本当の値を読めるようにするためです。

確かめ方(ND2-Q1 = A):

- `internal/testutil` の偽の gh を、名前を選べる「引数を書き出す偽物」に広げ、テストの二進を `git` と `gh` の名前で PATH の先頭に置きます。
- Unix では、作った命令を `sh -c` で動かし、偽物が受け取った引数が、元の値で組み立てた引数と同じことを比べます。
- Windows では、同じ命令を `powershell.exe -NoProfile -NonInteractive -Command` と `pwsh -NoProfile -NonInteractive -Command` の両方で動かして比べます。
- `pwsh` か `powershell.exe` が見つからないとき、CI(環境の変数 `CI` が `true`)ではテストを失敗にし、手元では理由を出して飛ばします。GitHub の windows-latest には両方が入っています。
- 値の一覧: `'`、`"`、`$x`、`$(echo x)`、`` `x ``、U+2018〜U+201B、空白、空白を含んで末尾が `\`、改行、`-x`、`*`、`[a]`、`:x`。囲めない値では `Manual` になることも確かめます。

残るリスク: Windows の Git Bash 以外の sh(Cygwin、WSL から呼んだときなど)や `cmd.exe` に PowerShell 用の命令を貼ると、`''` を重ねた部分で値が変わります(別の命令にはならない)。表示に「PowerShell 用」と書くことで知らせます。Git Bash と MSYS2 は `MSYSTEM` で見分けて sh 用を出します。

## 5. 本文のファイル(NFR1.10、ND2-Q2)

`writeBody` を、作成の段階の中から Publish の始め(ブランチの段階の前)に移し、作り方を変えます。

- フォルダー: `<GitDir>/tag-refactor/`(`os.MkdirAll`、0700)。
- 名前: `os.CreateTemp(dir, "pr-body-"+name+"-*.md")`。`name` はブランチ名の `[^A-Za-z0-9._-]` を `-` に置き換え、先頭の 60 バイトで切ったものです(置き換えたあとは ASCII だけなので、バイトで切っても文字が壊れない)。`CreateTemp` の乱数は最大10桁なので、名前は最大で 8 + 60 + 1 + 10 + 3 = 82 文字です。`CreateTemp` は重ならない名前を選び、Unix では 0600 で作ります。テストは「名前が 100 文字以下」を確かめ、乱数の桁に依りません。
- 作れなかったとき(フォルダーかファイルを作れない、書けない): Publish は `sink.Step(branch, started)` と `sink.Step(branch, failed)` を送り、`out.FailedStep` を `branch`、`out.Err` を本文のファイルの誤り(表示の文の ID は `iac.prBodyFileFailed`、OS の誤りの文を添える)、`out.BodyFile` を空にして返します。契約 C1 の段階の名前は変えません。
- 消すとき: 作成が成功したら、このファイルだけを消します。ほかの `pr-body-*.md` には触りません(ND2-Q2 = A)。消せなかったときは、成功のまま、残った場所を `--debug` に出します。
- Windows では権限を変えません(残るリスク。NFR の文書のとおり)。

### 失敗と中断のときの表示(`internal/cli` の `publish`)

`out.FailedStep` が空でないときは、中断かどうかによらず、次の順に呼びます。

1. 中断でなければ `a.p.Error(describe(out.Err))`。中断なら誤りの文は出さず、中断の表示(今の `ExitMeaningInterrupted`)に任せる。
2. `lines := pullrequest.ResumeCommands(plan, in.Title, out, pullrequest.DefaultShell())`。
3. `a.p.PullRequestStopped(out, lines)`。表示するもの:
   - 済んだ段階が無ければ「書き換えたファイルはそのまま残っています。ブランチは作っていません」(`iac.prFilesKept`)。あれば今の `iac.prStay`。
   - 続きの命令(4節)。
   - `out.BodyFile` があれば、その場所と「使い終えたら消してかまいません」(`iac.prBodyKept`)。
4. 中断なら `ExitError` と `ExitMeaningInterrupted`、そうでなければ `ExitError` と `ExitMeaningIacPRFailed` を返す。

## 6. ブランチ名の確かめ(NFR6.12)

`checkLocal` で、`--pr-branch` があるときだけ、次の順に確かめます。どれも書き換えの前です。

1. 先頭が `-` なら入力の誤り(`git` を呼ばない)。
2. `git check-ref-format --branch <名前>` を、読むだけの git の上限(30 秒)で呼ぶ。
3. 終了コードが 0 でない、または標準出力の前後の空白を除いたものが `<名前>` と違えば入力の誤り(`@{-1}` などを断る)。

既定のブランチ名(BR1.2)は tag-refactor が作る正しい名前なので、確かめません。

## 7. 実際の GitHub の結合テスト(NFR9.2)

- `internal/cli/github_pr_test.go` の `TestGitHubPullRequest` を、`t.Run` のサブテスト2つ(`default` と `options`)に分けます。CI の `-run '^TestGitHubPullRequest$'` はサブテストも動かすので、`ci.yml` は変えません。
- `options` は、`--pr-branch`(`tag-refactor/iac-<UTC の日時>Z-options`)、`--pr-title`、`--pr-body-file`(一時ファイル)を渡して `iac --pr` を動かし、`gh pr view --json headRefName,title,body` で、ブランチ名、タイトル、本文の先頭が渡したものと同じことを確かめます。
- ブランチ名を `tag-refactor/iac-` で始めるので、古い枝の掃除の対象に入ります。掃除の `staleBranch` は、接頭辞のすぐあとの `YYYYMMDD-HHMMSS` を UTC として読み、そのあとの `Z` や `-options` は無視するように直します(既定の名前が UTC と `Z` になるため。NFR6.13)。
- 後片付け(プルリクエストを閉じてブランチを消す)とトークンの扱いは U1 の6節のとおりです。

## 8. 合否の基準を機械で確かめる方法

| 要件 | テスト |
|---|---|
| NFR1.6、NFR1.7 | `internal/redact` の単体テスト(形ごとの境界、数えなかった一致と重なる本物の形、同じ行の2回、12行で10個と「ほか 2 か所」)。`internal/cli` の受け入れテスト(止めたときにソースのファイルが変わらず、git も gh も呼ばれない。標準エラーと `--debug` に値が出ない。名前付きパイプと 1 MiB を超えるファイルを断る) |
| NFR1.8 | 3節の単体テスト (a)〜(c) と受け入れテスト。BR3.3 の境界(65,536 文字ちょうどは省かない、超えると手で直す一覧の後ろを省く、省いても超えると ok が false で Publish が呼ばれず終了コード 1) |
| NFR1.9、NFR6.15 | 4節の実際のシェルのテスト。push を失敗させた受け入れテストで、命令に本文が無く `--body-file` があり、commit の段階が2行に分かれている |
| NFR1.10 | `internal/pullrequest` の単体テスト(0600、成功で消える、失敗で残る、重ならない名前、200 文字のブランチ名で名前が 100 文字以下、フォルダーが書けないときに branch の失敗になりブランチができない)。`internal/cli` の受け入れテスト(中断で、本文のファイルの場所と続きの命令が出て、ファイルが残る。本文のファイルを作れないとき「書き換えたファイルはそのまま残っています」と手順の文が出る) |
| NFR6.12 | `internal/cli` の受け入れテスト(`@{-1}`、`-x`、`--help`、`a..b`) |
| NFR6.13、NFR6.14 | 単体テスト(差し替えた時計、日本語の 60,000 と 60,001 文字)。`staleBranch` の単体テスト(`Z` と `-options` のある名前) |
| NFR9.2 | 7節の結合テスト(secret のある CI だけ) |
