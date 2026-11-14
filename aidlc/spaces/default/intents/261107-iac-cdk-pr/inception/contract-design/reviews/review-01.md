## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-08T07:46:11Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C3 TagSite (Key, Value, Origin) | `TagSite` の `Origin` が1つしかない。FR3.3 の(2)同じファイルの定数と(3)辞書を回す書き方では、キーと値の両方が別々の元の文字列からたどられる(例: `Object.entries(TAGS)` の各エントリはキーも値も辞書の中にある)。キー名の変更と値の変更は別の範囲を書き換えるので、1つの `Origin` ではどちらの元かが決まらず、片方しか直せない。また、`Key` と `Value` の位置がどのファイルのものか(使う場所か元か)、書き換えの対象が `Key`/`Value` と `Origin` のどちらかを決める規則も書かれていない。 | `KeyOrigin` と `ValueOrigin` に分けるか、`Key` と `Value` のそれぞれに「書き換える範囲(ファイル、Start、End)」を持たせる。リテラルの場合は使う場所そのもの、たどった場合は元、と書き換え先の決め方を1行で定め、辞書を回す場合の1つの `TagSite` と複数のエントリの関係(エントリごとに1件か)も決める。 | New |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C2 SynthesizedResource / C7 target_rule | 変更案のリソースを CDK のものと決める鍵が「スタック名と論理 ID」だけで、アプリとデプロイ先(アカウント、リージョン)を含まない。複数のアプリで同じスタック名と論理 ID があるとき、または環境を決めないスタックを複数のリージョンに置いたとき(同じソースから別々のリソースができる)、変更案の1つのリソースがどのアプリのどの行に対応するか決まらず、誤ったソースへ反映するか、2件目が「値が違う」になる。`Stacks[].Name` が、テンプレートの名前か、デプロイされる実際のスタック名(タグ `aws:cloudformation:stack-name` と比べる値)かも書かれていない。 | 照合の鍵とその衝突時の規則を契約に書く。(1) `Stacks[].Name` は、実際にデプロイされるスタック名とする。(2) 複数のアプリで鍵が衝突したときは手で直す一覧に出す(黙って先のアプリを採らない)。(3) 同じソースへ複数のリージョンのリソースが対応する場合の扱い(同じ変更なら1件の書き換え、違えば手で直す一覧)を決める。必要なら `Synthesis.Stacks` に環境(アカウント、リージョン、不明なら空)を足す。 | New |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > X2 never_contains / C2 Failure.Detail / C7 cdkAppFailed | 公開のリポジトリに作られるプルリクエストの本文に、「手で直す一覧」の理由が入る。X2 は ARN、アカウント ID、タグの値、トークンを含めないと書くが、`cdkAppFailed` の `Detail` は `cdk synth` や利用者のアプリが出した文(標準エラーなど)から作られ、ARN、アカウント ID、パス、タグの値、アプリが読んだ秘密の値を含みうる。含めない仕組み(許可する語だけを使う、取り除く、など)と、それを確かめるテストの契約が無い。本文は公開の場に出るので、プロジェクトの禁止事項(秘密の値を置かない)にも関わる。 | `Failure.Detail` と `ManualItem.Detail` のうち、本文と表示に出してよいものを決める(例: `Reason` の種類ごとの固定の文と、道具の名前だけを出し、利用者のアプリの出力は `--debug` の端末表示に限り本文には入れない)。ARN とアカウント ID の形を本文の組み立て側で取り除く規則と、それを確かめる受け入れテストを契約に足す。 | New |
| R-04 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C6 ProgressSink, 契約の決まり(足すだけの変更) | 「今の `ProgressSink` に足す」とあるが、コードには `inventory.ProgressSink` と `apply.ProgressSink` の2つの別のインタフェースがあり(どちらも受け手の `display.Printer` が実装、`internal/inventory/inventory.go`、`internal/apply/apply.go`)、`iac` 用のものは無い。どちらかに足すと、そのパッケージの全部の実装(テストの差し替えを含む)がビルドで壊れ、`internal/cdk` と `internal/pullrequest` がそのパッケージに依存することにもなる。「足すだけの変更は使う側を壊さない」という決まりは、Go のインタフェースには当てはまらない。また Q5 は「プルリクエストの段階(済み、失敗)」とするが、`PullRequestStep` に失敗を表す形が無く、FR2.7 の「待ち」の状態を伝える知らせも無い。 | 既存のインタフェースの流儀(使う側のパッケージが自分の通知口を定義し、`display.Printer` が実装する)に合わせ、`internal/cdk` と `internal/pullrequest` がそれぞれ小さな通知口を持つ形に契約を直す。`PullRequestStep` の段階に成否を表す欄を足し、アプリの「待ち」と「外した(skipped)」の知らせを足すか、表示側で持つ状態として省くかを決める。「足すだけの変更」の決まりは、構造体の欄と列挙値に限ると書き直す。 | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C2 Options.WithTrace / Synthesis | `WithTrace` で synth し直した結果(コンストラクトのパスからファイルと行)を受け取る欄が `Synthesis` にも `Analysis` にも無い。未決事項は設定の名前と出力の形だけを挙げており、結果を Go のどの型で渡すかが抜けている。U3 と U4 の両方がここに依存する。 | `Synthesis` に作成の箇所の対応(コンストラクトのパス、ファイル、行)の欄を足すか、`AnalyzeRequest` で受け取る形にし、`ConstructSite` との優先順位(ソースをたどった結果と記録のどちらを採るか)を1行で決める。 | New |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C1 (Publisher rules), X1 | NFR3 は `git` と `gh` の呼び出しにも時間の上限を求める(合否の基準は応答しない差し替えのプログラムで止まること)が、C1 に上限の値も、利用者が変えられるかも書かれていない。コミットと push は利用者のフックと署名(対話式のパスフレーズなど)を動かすので、短い固定の上限は正しい操作を途中で止め、半端な状態(コミット済みで push 前)を作りうる。 | C1 に段階ごとの上限の値(または「フックを含む段階は上限を長くする、設定できる」)を書き、上限で止まった段階を `FailedStep` と `ResumeCommand` で扱うことを明記する。 | New |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C1 PublishOutcome.ResumeCommand | 続きの命令に、タイトルと本文(`--pr-body-file` は最大 1 MiB、本文には利用者のタグのキーなどの文字が入る)を含めるとあるが、引用符の付け方が `sh`、PowerShell、`cmd` で異なり、長さの上限もある。そのまま作ると、Windows で動かない、またはキーの中の記号で命令が壊れる。 | 本文は命令に直接入れず、`--body-file` に渡す形にするか、短い場合だけ入れる規則と OS ごとの引用符の付け方を契約に書く。 | New |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C2 App.Language: other, C7 ManualReason/CDKApps.Status | `Language` が `other`(Java、Go、.NET の CDK)のアプリを、どの `Status` と理由で扱うかが決まっていない。`Failure.Reason` の `cdkV1` と、`ManualReason` の `cdkV1`/`cdkAppFailed` の対応も書かれていない(FR2.10 は v1 だけを挙げている)。 | `other` のアプリは `skipped` とし、理由とメッセージを決める。`Failure.Reason` から `ManualReason` への対応を表にする。 | New |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/contract-design/contract-summary.md > C1 rules (commit) | 「files だけをステージする(ほかのステージ済みのファイルは入れない)」とあるが、手段が書かれていない。`git add <files>` のあとの `git commit` は、利用者がすでにステージしていた別のファイルも入れる。FR1.8 の合否の基準(未コミットのほかのファイルが入らない)を満たすには、ステージ済みのファイルの検出か、パスを限った `git commit -- <files>` が要る。`--dir` がリポジトリの根でないときの `files` のパスの基準(`--dir` からの相対)から `git` へ渡す形も書かれていない。 | コミットの手段(パスを限ったコミット、または Check でステージ済みのほかのファイルがあれば止める)と、`files` をリポジトリの根からの相対に直す責任の置き場を C1 に書く。 | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| (none declared for this stage) | not run | 検証ツールは宣言されていないので、契約の相互参照(C1〜C7、X1、X2 と、各単位、components.md、requirements.md の対応)は目で確かめた。IDs と所有者の対応は一貫している(U1〜U6 の依存の向きに循環は無い)。 |

### Summary

契約は単位の境目(U1 から U6)と外の境目(コマンド、プルリクエストの本文)をよく網羅し、所有者も依存の向きと矛盾しない。ただし、反映の中心になる `TagSite` の書き換え先の表し方、リソースの照合の鍵、公開のプルリクエストに出る失敗の理由の扱い、通知口の実際の型との食い違いの4点は、実装者が推測するしかなく、手直しが要る。これらは契約の文章の修正で解ける(助言の位置づけのレビューなので、承認の場で判断してください)。
