## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-08T08:01:34Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C3 TagSite (Key, Value, Origin) | `TagSite` の `Origin` が1つしかない。FR3.3 の(2)同じファイルの定数と(3)辞書を回す書き方では、キーと値の両方が別々の元の文字列からたどられる(例: `Object.entries(TAGS)` の各エントリはキーも値も辞書の中にある)。キー名の変更と値の変更は別の範囲を書き換えるので、1つの `Origin` ではどちらの元かが決まらず、片方しか直せない。また、`Key` と `Value` の位置がどのファイルのものか(使う場所か元か)、書き換えの対象が `Key`/`Value` と `Origin` のどちらかを決める規則も書かれていない。 | `KeyOrigin` と `ValueOrigin` に分けるか、`Key` と `Value` のそれぞれに「書き換える範囲(ファイル、Start、End)」を持たせる。リテラルの場合は使う場所そのもの、たどった場合は元、と書き換え先の決め方を1行で定め、辞書を回す場合の1つの `TagSite` と複数のエントリの関係(エントリごとに1件か)も決める。 | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C2 SynthesizedResource / C7 target_rule | 変更案のリソースを CDK のものと決める鍵が「スタック名と論理 ID」だけで、アプリとデプロイ先(アカウント、リージョン)を含まない。複数のアプリで同じスタック名と論理 ID があるとき、または環境を決めないスタックを複数のリージョンに置いたとき(同じソースから別々のリソースができる)、変更案の1つのリソースがどのアプリのどの行に対応するか決まらず、誤ったソースへ反映するか、2件目が「値が違う」になる。`Stacks[].Name` が、テンプレートの名前か、デプロイされる実際のスタック名(タグ `aws:cloudformation:stack-name` と比べる値)かも書かれていない。 | 照合の鍵とその衝突時の規則を契約に書く。(1) `Stacks[].Name` は、実際にデプロイされるスタック名とする。(2) 複数のアプリで鍵が衝突したときは手で直す一覧に出す(黙って先のアプリを採らない)。(3) 同じソースへ複数のリージョンのリソースが対応する場合の扱い(同じ変更なら1件の書き換え、違えば手で直す一覧)を決める。必要なら `Synthesis.Stacks` に環境(アカウント、リージョン、不明なら空)を足す。 | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > X2 never_contains / C2 Failure.Detail / C7 cdkAppFailed | 公開のリポジトリに作られるプルリクエストの本文に、「手で直す一覧」の理由が入る。X2 は ARN、アカウント ID、タグの値、トークンを含めないと書くが、`cdkAppFailed` の `Detail` は `cdk synth` や利用者のアプリが出した文(標準エラーなど)から作られ、ARN、アカウント ID、パス、タグの値、アプリが読んだ秘密の値を含みうる。含めない仕組み(許可する語だけを使う、取り除く、など)と、それを確かめるテストの契約が無い。本文は公開の場に出るので、プロジェクトの禁止事項(秘密の値を置かない)にも関わる。 | `Failure.Detail` と `ManualItem.Detail` のうち、本文と表示に出してよいものを決める(例: `Reason` の種類ごとの固定の文と、道具の名前だけを出し、利用者のアプリの出力は `--debug` の端末表示に限り本文には入れない)。ARN とアカウント ID の形を本文の組み立て側で取り除く規則と、それを確かめる受け入れテストを契約に足す。 | Resolved (reviewer) |
| R-04 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C6 ProgressSink, 契約の決まり(足すだけの変更) | 「今の `ProgressSink` に足す」とあるが、コードには `inventory.ProgressSink` と `apply.ProgressSink` の2つの別のインタフェースがあり(どちらも受け手の `display.Printer` が実装、`internal/inventory/inventory.go`、`internal/apply/apply.go`)、`iac` 用のものは無い。どちらかに足すと、そのパッケージの全部の実装(テストの差し替えを含む)がビルドで壊れ、`internal/cdk` と `internal/pullrequest` がそのパッケージに依存することにもなる。「足すだけの変更は使う側を壊さない」という決まりは、Go のインタフェースには当てはまらない。また Q5 は「プルリクエストの段階(済み、失敗)」とするが、`PullRequestStep` に失敗を表す形が無く、FR2.7 の「待ち」の状態を伝える知らせも無い。 | 既存のインタフェースの流儀(使う側のパッケージが自分の通知口を定義し、`display.Printer` が実装する)に合わせ、`internal/cdk` と `internal/pullrequest` がそれぞれ小さな通知口を持つ形に契約を直す。`PullRequestStep` の段階に成否を表す欄を足し、アプリの「待ち」と「外した(skipped)」の知らせを足すか、表示側で持つ状態として省くかを決める。「足すだけの変更」の決まりは、構造体の欄と列挙値に限ると書き直す。 | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C2 Options.WithTrace / Synthesis | `WithTrace` で synth し直した結果(コンストラクトのパスからファイルと行)を受け取る欄が `Synthesis` にも `Analysis` にも無い。未決事項は設定の名前と出力の形だけを挙げており、結果を Go のどの型で渡すかが抜けている。U3 と U4 の両方がここに依存する。 | `Synthesis` に作成の箇所の対応(コンストラクトのパス、ファイル、行)の欄を足すか、`AnalyzeRequest` で受け取る形にし、`ConstructSite` との優先順位(ソースをたどった結果と記録のどちらを採るか)を1行で決める。 | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C1 (Publisher rules), X1 | NFR3 は `git` と `gh` の呼び出しにも時間の上限を求める(合否の基準は応答しない差し替えのプログラムで止まること)が、C1 に上限の値も、利用者が変えられるかも書かれていない。コミットと push は利用者のフックと署名(対話式のパスフレーズなど)を動かすので、短い固定の上限は正しい操作を途中で止め、半端な状態(コミット済みで push 前)を作りうる。 | C1 に段階ごとの上限の値(または「フックを含む段階は上限を長くする、設定できる」)を書き、上限で止まった段階を `FailedStep` と `ResumeCommand` で扱うことを明記する。 | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C1 PublishOutcome.ResumeCommand | 続きの命令に、タイトルと本文(`--pr-body-file` は最大 1 MiB、本文には利用者のタグのキーなどの文字が入る)を含めるとあるが、引用符の付け方が `sh`、PowerShell、`cmd` で異なり、長さの上限もある。そのまま作ると、Windows で動かない、またはキーの中の記号で命令が壊れる。 | 本文は命令に直接入れず、`--body-file` に渡す形にするか、短い場合だけ入れる規則と OS ごとの引用符の付け方を契約に書く。 | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C2 App.Language: other, C7 ManualReason/CDKApps.Status | `Language` が `other`(Java、Go、.NET の CDK)のアプリを、どの `Status` と理由で扱うかが決まっていない。`Failure.Reason` の `cdkV1` と、`ManualReason` の `cdkV1`/`cdkAppFailed` の対応も書かれていない(FR2.10 は v1 だけを挙げている)。 | `other` のアプリは `skipped` とし、理由とメッセージを決める。`Failure.Reason` から `ManualReason` への対応を表にする。 | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C1 rules (commit) | 「files だけをステージする(ほかのステージ済みのファイルは入れない)」とあるが、手段が書かれていない。`git add <files>` のあとの `git commit` は、利用者がすでにステージしていた別のファイルも入れる。FR1.8 の合否の基準(未コミットのほかのファイルが入らない)を満たすには、ステージ済みのファイルの検出か、パスを限った `git commit -- <files>` が要る。`--dir` がリポジトリの根でないときの `files` のパスの基準(`--dir` からの相対)から `git` へ渡す形も書かれていない。 | コミットの手段(パスを限ったコミット、または Check でステージ済みのほかのファイルがあれば止める)と、`files` をリポジトリの根からの相対に直す責任の置き場を C1 に書く。 | Resolved (reviewer) |
| R-10 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C1 Publisher Check Publish と Plan.Body, X2 | 本文の変更ファイルの一覧と手で直す一覧は書き換えのあとにしか分からず、組み立ては IacDisplay の担当だが、Plan は書き換えの前の Check で作られ、Publish は files だけを受け取る。確定した本文を Publisher に渡す経路が契約に無く、失敗時に BodyFile へ書く内容の出どころも決まっていない。 | IacCommand が書き換えのあとに IacDisplay の組み立てた本文を Plan.Body に入れて Publish を呼ぶ、と C1 に1行で書く。Check が返す Plan.Body は --pr-body-file の中身だけとする、と区別する。 | New |
| R-11 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C6 pullrequest.ProgressSink と C1 Publisher | C6 は pullrequest.ProgressSink を PullRequestPublisher に渡すと書くが、C1 の Check、Publish のシグネチャにも Publisher の生成にも通知口の受け取り口がない。step の checked を誰がいつ送るかも決まっていない。 | Publisher の生成時に通知口を受け取る形か、Check と Publish の引数に通知口を足す形かを C1 に書く。checked は Check の最後に送る、と決める。 | New |
| R-12 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C2 Discover と App.Language, App.CDKMajor | Discover が Language と CDKMajor をどこから決めるか(cdk.json の app 欄、package.json、requirements.txt など)が書かれていない。CDKMajor が 0(不明)のアプリを synth するか外すか、needsAWS をどう見分けるかも決まっていない。 | Language と CDKMajor の判定元と、0 のときの扱い(例: synth を試す)、needsAWS の見分け方を C2 に書く。または未決事項に止まる単位つきで足す。 | New |
| R-13 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > X1 exit_codes と C7 WrittenFiles | --pr を付けたのに書き換えるものが無い(WrittenFiles が空、手で直すものだけ、など)ときに、プルリクエストを作るか、作らないか、書き換えの前の Check をどこまで行うかが決まっていない。空のコミットは git commit が失敗する。 | WrittenFiles が空のときは Publish を呼ばず、その旨を示して終了コードは手で直すものの有無で 0 か 2、と X1 に書く。Check を空のときも行うかどうかも決める。 | New |
| R-14 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C1 resume の引用符 | 続きの命令の引用符を Windows は PowerShell、それ以外は POSIX と決めているが、Windows で cmd.exe や Git Bash を使う利用者には、そのまま貼ると動かない。 | 命令の見出しにシェルの種類を示すか、タイトルは一時ファイルから読む形にして引用符をなくす。または、Windows は PowerShell 前提と表示に明記する。 | New |

> R-01 Reviewer note: C3 の TagSite が KeyEdit と ValueEdit を別々に持ち、edit_rules で書き方ごとの書き換え先と、辞書を回す書き方ではエントリごとに1件とする決まりを定めている。解消した。

> R-02 Reviewer note: C2 の Stack に実際のスタック名、Account、Region を持たせ、C7 の match_rule で候補が複数のときの cdkAmbiguous と、same_site_rule を定めている。解消した。

> R-03 Reviewer note: X2 で理由の文をカタログの固定の文と道具の名前だけにし、外のプログラムの出力を本文に入れない決まり、ARN と12桁の数字の伏せ字、受け入れテストを定めている。既存の ManualItem.Detail はファイル名とアドレスだけなので、整合する。解消した。

> R-04 Reviewer note: C6 が既存の流儀に合わせ、cdk と pullrequest が各自の小さな通知口を持つ形になった。待ちと外したことの知らせ、段階の成否も入った。足すだけの変更の決まりも構造体の欄と列挙値と JSON に限定された。通知口の受け渡しの残りは新しい指摘に書く。

> R-05 Reviewer note: Synthesis.TraceSites、AnalyzeRequest.TraceSites、C4 の traceSites と、ソースを先に使う優先順位が定まった。解消した。

> R-06 Reviewer note: C5 に呼び出しごとの時間の上限の表があり、commit と push は10分、止まった段階は FailedStep と ResumeCommands で扱う。解消した。

> R-07 Reviewer note: 本文は git の管理フォルダー内のファイルに書いて --body-file で渡す形になり、タイトルは256文字までで引用符は OS で決める。解消した。cmd.exe の扱いは新しい指摘に書く。

> R-08 Reviewer note: other は skipped で cdkUnsupportedLanguage、Failure.Reason から ManualReason への対応表が C7 に入った。解消した。Language の判定は新しい指摘に書く。

> R-09 Reviewer note: git commit --only と literal-pathspecs、RepoRoot と Prefix による相対パスの直しが C1 に定まった。解消した。

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| none declared | not run | このステージには検証ツールの指定がない。参照は目視で確かめた。internal/iac の ManualItem.Detail がファイル名とアドレスだけであることと、既存の2つの ProgressSink が使う側のパッケージにあることを、コードで確かめた。 |

### Summary

前回の Major 4件と Minor 5件はすべて契約の中で解消している。残りは Publisher への本文と通知口の受け渡し、Discover の判定元、書き換えが空のときの --pr の扱いなどの Minor だけで、実装を妨げる Critical と Major は見つからなかったので READY とする。
