## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-09T09:52:29Z
**Iteration:** 2

### Prior findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 3節 部品ごとの伏せ字 | 今のコードは本文の全体に最後に1回だけ伏せ字を通しており、設計の「今のまま」は事実と違っていた | 部品ごとに伏せ字を通す形と、利用者の本文の受け取り方を書く | Resolved (reviewer) 3節の表が部品ごとの通し方、PullRequestBody の userBody 引数、全体と Publish で通さないことを書き、単体テスト (a) から (c) も足した |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 5節 本文のファイル(名前) | 切る長さ 80 バイトだと名前が最大 102 文字になり、要件の 100 文字以下を満たせない | 切る長さを減らすか基準を直し、具体的な数字で書く | Resolved (reviewer) 60 バイトに切り、最大 82 文字と計算が合う。要件の NFR1.10 も 60 に揃っている |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 7節 実際の GitHub の結合テスト | CI の run の条件が完全一致なので、別名の新しい関数は CI で動かず、新しい枝名は掃除の対象外だった | サブテストにするか run の条件を広げるかを決め、枝名を掃除の対象に合わせる | Resolved (reviewer) TestGitHubPullRequest のサブテスト 2 つにすれば ci.yml の run の条件のまま両方動く。枝名は掃除の接頭辞で始まる |
| R-04 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 5節 失敗と中断 | 中断のとき PullRequestStopped を呼ぶ前に返しており、本文のファイルを作れない場合の表示の道も無かった | 中断と作れなかったときの表示を、関数の名前と順番で書く | Resolved (reviewer) 失敗と中断の呼ぶ順番と、作れないときの sink と out の値が書かれ、8節に中断の受け入れテストもある |
| R-05 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 続きの命令(ResumeLine) | commit の段階は add と commit の 2 つの命令なのに、つなぎ方が決まっていなかった | つなぎ方をシェルごとに決めるか 2 行に分ける | Resolved (reviewer) 1 行に 1 つの命令とし、commit を add と commit の 2 行に分けた。連結の記号を使わない |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 と 2節(型) | 型の置き場所が今のコードとつながっていなかった | 型の変更を一覧にする | Resolved (reviewer) logical-components.md に型と呼ぶ場所の変更の表が加わり、ResumeCommands を消すこと、CommittedFiles を使うこと、BodyFile と UserBody を使うことが書かれた |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 続きの命令(commit の段階) | 続きの add に literal-pathspecs が付いていなかった | 付けると書き、記号を含む値をテストに加える | Resolved (reviewer) 表の add の行に付き、値の一覧に * と [a] と :x が入った |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 7節 後片付け | 枝名の末尾 Z で staleBranch が日時を読めなくなる | staleBranch を UTC と Z に合わせる | Resolved (reviewer) 接頭辞のすぐあとの 15 文字を UTC で読み、あとの Z と suffix を無視すると 7節と 8節に書かれた |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 2節 呼ぶ場所 | 本文の読み方が決まっておらず、特殊なファイルで止まりうる | 通常のファイルだけ、上限つきで読むと決める | Resolved (reviewer) IsRegular の確かめのあと LimitReader で 1 MiB と 1 バイトまで読むと決まった |
| R-10 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 Manual の行 | 囲めない値を端末にそのまま出すと表示を書き換えられる | 制御文字を見える形で示す | Resolved (reviewer) Manual の値は strconv.QuoteToGraphic を通すと書かれた |
| R-11 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 2節 の表 | 数えなかった一致のあとの探しなおしと、か所の数え方が決まっていなかった | 探しなおす位置と N の単位を決める | Resolved (reviewer) 一致の先頭の次のバイトから探しなおし、か所は行の数、改行は LF だけと決まった |
| R-12 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 シェルの選び方 | runtime.GOOS だけだと Git Bash で値が壊れる | 残るリスクに書き、sh 用を出す条件を決める | Resolved (reviewer) MSYSTEM で Git Bash と MSYS2 を見分けて sh 用を出し、それ以外の残るリスクと表示の注記を書いた |

### New findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-13 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 3節 部品ごとの伏せ字 | 本文が 65,536 文字を超えるときに手で直す一覧の後ろを省き、それでも超えたら作成に進まず止める決まり(BR3.3)がある。しかし新しい PullRequestBody は string だけを返し、省いても超えたことを呼び出し側に伝える道が無い。処理の順番の図にも、この停止の分岐が無い。8節にも 65,536 文字の境界のテストが無い | PullRequestBody が超過を返す形(例えば本文と超過の印、または誤り)を型の表に書く。図に、書き換えのあと Publish の前に止める分岐と終了コード 1 を足す。8節に 65,536 文字の境界のテストを足す | New |
| R-14 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 続きの命令(quoteSh と quotePowerShell) | 続きの命令の値は、囲めない条件が C0 の制御文字と DEL だけで、C1 の制御文字や双方向の制御文字(Cf)は命令にそのまま入って端末に出る。今の表示の Sanitize は Cc、Cf、Zl、Zp を escape するので、この道だけ同じ守りが外れる。また命令を Sanitize に通すとバックスラッシュが二重になり貼れなくなるが、通すかどうかも書かれていない | 命令は Sanitize を通さず生で出すと書く。そのうえで、Sanitize が escape する分類(Cc、Cf、Zl、Zp)の文字を含む値も囲めない値として Manual にする | New |

### Summary

前回の 12 件はすべて直っていることを、設計の文書と今のコード(pullrequest.go、iac.go、iacpr.go、ci.yml、github_pr_test.go)と照らして確かめました。名前の長さ、サブテストと CI の run の条件、失敗と中断の呼ぶ順番、型の変更の表は、実装者が迷わず書ける具体さになっています。新しい指摘は、65,536 文字を超えたときの停止の経路と、続きの命令の値の制御文字の扱いの 2 件の Minor だけで、どちらも実装の前に直せますが、承認を止めません。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 3節 部品ごとの伏せ字 | 3節の表は、本文の導入、変更したファイル、手で直す一覧について「今のまま(U1 で部品ごとに通している)」と書いているが、今のコード(internal/display/iacpr.go の PullRequestBody)は組み立てた本文の全体に最後に1回だけ伏せ字を通している。部品ごとには通していない。しかも引数に利用者の本文を受け取る口が無く、手で直す一覧もまだ組み立てていない。この前提のまま実装すると、(a) 全体への伏せ字を残して利用者の本文の ARN まで伏せてしまう(契約 X2 と NFR1.7 に反する)か、(b) 全体への伏せ字をやめたのに手で直す一覧の部品に通し忘れて、Publish が伏せ字をやめた後に ARN と12桁の数字がそのまま公開の本文に出る、のどちらかになる。 | 3節の「今のまま」を直し、PullRequestBody を部品ごとに伏せ字を通してから利用者の本文とつなぐ形に変えることを明記する。利用者の本文をどこから受け取るか(引数か、Plan.UserBody か)と、最後に全体へ伏せ字を通さないことを書く。単体テストの観点に、組み立てた本文の中で tag-refactor の部品だけが伏せ字になり、利用者の部分は変わらないことを加える。 | New |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 5節 本文のファイル(名前) | 名前を pr-body- と、先頭 80 バイトに切った名前、ハイフン、os.CreateTemp の乱数(最大10桁)、.md でつくると、最大で 102 文字になる。要件 NFR1.10 の合否の基準は「200 文字のブランチ名でもファイル名が 100 文字以下」で、乱数が10桁のとき(約8割)は満たせない。基準どおりのテストを書くと、通ったり落ちたりする。 | 切る長さを減らす(例: 乱数と接頭辞を数えて 77 バイトまで)か、要件の基準を実際の上限に合わせて直すかのどちらかを決めて、設計に具体的な数字で書く。テストは乱数の桁に依らず通る形にする。 | New |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 7節 実際の GitHub の結合テスト | 「CI の仕事 github-pr は変えない」と書いているが、今の ci.yml は go test に -run の条件として TestGitHubPullRequest の完全一致(先頭と末尾を固定した形)を渡している。新しい関数を別名(例: TestGitHubPullRequestWithOptions)で足すと、CI では一度も動かず、secret のある CI でも NFR9.2 の合否の基準(そのテストが通る)を確かめられない。さらに新しい枝名 tag-refactor/test-乱数 は、テストの古い枝の掃除(prBranchPrefix は tag-refactor/iac-)の対象外で、CI が途中で取り消されると枝がテスト専用のリポジトリに残り続ける。 | 新しいテストを TestGitHubPullRequest のサブテストにするか、ci.yml の -run の条件を広げるかを決めて、設計に書く(後者なら『変えない』を直す)。枝の名前を掃除の対象の接頭辞に合わせるか、掃除の対象を広げる。 | New |
| R-04 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 5節 失敗と中断 | 「中断(Ctrl+C)でも消さずに場所を標準エラーに示す」と書くが、今の cli の publish は ctx.Err() が空でないとき、PullRequestStopped を呼ぶ前に終了コード 1 を返す。そのため中断では、本文のファイルの場所も続きの命令も出ない。実装の置き場所の変更(中断でも PullRequestStopped を呼ぶか、別の表示を呼ぶか)が書かれていない。また本文のファイルを作れない場合も、PullRequestStopped は済んだ段階があるときと BodyFile があるときしか表示しないので、要件にある『書き出したファイルは残っていること』と手順の文が出る道が設計に無い。進捗の sink に branch の失敗を通知するかも決まっていない。 | publish の中断の分岐と、本文のファイルを作れなかったときの表示(書き出したファイルが残っていること、手順の文、sink への通知の有無)を、呼ぶ関数の名前と順番で書く。中断のときの受け入れテスト(場所が出て、ファイルが残る)も8節に足す。 | New |
| R-05 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 続きの命令(ResumeLine) | ResumeLine は1段階につき Command を1つ(シェルに貼れる1行)としているが、commit の段階は git add と git commit の2つの命令になる(BR4.3)。つなぎ方が決まっていない。sh の && は Windows PowerShell 5.1 では構文の誤りになる(7 以降では使える)。; でつなぐと add が失敗しても commit が動く。どれを選ぶかで、4節の確かめ(5.1 と 7 の両方で動かして比べる)が通るかが変わる。 | commit の段階のつなぎ方をシェルごとに決めて書く(例: PowerShell は ; でつなぎ、add が失敗したら止めるかどうかも書く。または commit を ResumeLine の2つに分ける)。確かめの対象にこの段階を含める。 | New |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 と 2節(型) | 型の置き場所が今のコードとつながっていない。PublishOutcome.ResumeCommands は今 []string だが、新しい関数は []ResumeLine を返し、どちらを残すか、いつだれが呼ぶか(Publish の中か cli か)が書かれていない。引数 in.Files は --dir からの相対だが、BR4.3 はリポジトリの根からの相対を要求する(out.CommittedFiles を使う必要がある)。CheckRequest.BodyFile と Plan.UserBody が今コードにあるのに、本文は cli が読むと書かれているので、使うのか消すのかが分からない。 | 型の変更を一覧にする: ResumeCommands の型と呼ぶ場所、使うファイルの一覧(out.CommittedFiles)、CheckRequest.BodyFile と Plan.UserBody を残すか消すか。 | New |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 続きの命令(commit の段階) | BR4.3 の続きの add の命令は git -C 根 add で始まり、--literal-pathspecs が付いていない。Publish の本物の add は付けている。ファイル名に * や [ や先頭の : があると、続きの命令では別のファイルまでステージされうる。NFR1.9 の『ファイル名にどんな文字があっても』に届かない。 | 続きの add にも --literal-pathspecs を付けると書き、ファイル名に * と [ を含むテストの値を値の一覧に加える。 | New |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 7節 後片付け | 既定の枝名が末尾に Z をつける形(NFR6.13)になると、internal/cli/github_pr_test.go の staleBranch は time.ParseInLocation で日時を読めなくなり(Z が余る。しかも手元の時間帯で読んでいる)、古い枝の掃除が黙って効かなくなる。設計はこの影響に触れていない。 | staleBranch を UTC と末尾の Z に合わせて直すことを7節に書く。 | New |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 2節 呼ぶ場所 | 本文のファイルを『読み、1 MiB を確かめる』方法が書かれていない。大きさを調べてから全部読む実装だと、名前付きパイプや /dev/zero のような特殊なファイルで止まる。BR1.1 のとおり先に拒むのか、上限つきで読むのか、分からない。 | io.LimitReader で 1 MiB と1バイトだけ読み、超えたら入力の誤りにする、通常のファイルでなければ入力の誤りにする、のように読み方を決めて書く。 | New |
| R-10 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 Manual の行 | 囲めない値(制御文字を含む値)をそのまま『名前: 値』として端末に出すと書いているが、Sanitize を通すか、制御文字を目に見える形で示すかが書かれていない。ファイル名にエスケープシーケンスが入ると、端末の表示を書き換えられるうえ、利用者も本当の値を見られない。 | Manual の値は Sanitize を通し、制御文字は見える形(x1b など)で示すと書く。 | New |
| R-11 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 2節 の表 | 境目の確かめで一致を『数えない』としたあと、どこから探しなおすかが書かれていない。FindAll 系の関数をそのまま使うと、数えなかった一致と重なる本物のトークンを見落とす。また Finding は行の番号だけを持つので、一行に同じ形が12か所あっても『ほか N か所』の N が行の数なのか一致の数なのか分からず、要件の基準(12か所で10個と『ほか 2 か所』)と食い違いうる。さらに r だけで区切る本文の行の番号は数えられない。 | 数えなかった一致のあとは、一致の先頭の次の文字から探しなおすと書く。N は行の数にするか一致の数にするかを決めて書く。 | New |
| R-12 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/nfr-design/security-design.md > 4節 シェルの選び方 | 使うシェルを runtime.GOOS だけで決めると、Windows の Git Bash に貼った PowerShell 用の命令は、一重引用符を二つ重ねる部分が消え、タイトルや名前の ' と U+2019 などが変わる(別の命令にはならないが値が壊れる)。残るリスクに書かれていない。 | 残るリスクに書き、表示に『PowerShell 用』を明示することと、sh 用を出す選択肢(環境の変数でわかる範囲)のどちらにするかを決める。 | New |

> R-01 Not re-checked this round

> R-02 Not re-checked this round

> R-03 Not re-checked this round

> R-04 Not re-checked this round

> R-05 Not re-checked this round

> R-06 Not re-checked this round

> R-07 Not re-checked this round

> R-08 Not re-checked this round

> R-09 Not re-checked this round

> R-10 Not re-checked this round

> R-11 Not re-checked this round

> R-12 Not re-checked this round
