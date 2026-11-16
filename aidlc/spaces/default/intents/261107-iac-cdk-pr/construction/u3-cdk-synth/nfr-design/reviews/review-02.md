## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-15
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/nfr-design/logical-components.md > 部品の表の Analyzer の行、契約との関係(契約 C3) と security-design.md > 5節、8節の起動の回数の表 | 契約 C3 の traceSites は「今のコードの形のまま使う」、Analyzer は「ふるまいは変えない」と書いてあるが、今のコードでは補助のプログラム(internal/cdk/helper/ts-helper.js)は request の traceSites を一切読まず、作成の箇所を from が source のものしか出さない(trace も from trace も出てこない)。部品の表にも ts-helper.js の変更の行が無い。さらに Analyzer.Recheck は TraceSites を空の配列で固定して送り、iac の recheckCDK は compareAnalyses で ConstructSites の一致を見る。2回目の Analyze に trace 由来の作成の箇所が入ると、Recheck の結果にはそれが無く、「construct sites changed」で、trace を使ったアプリの編集がすべて cdkUnsupportedShape になって捨てられる。trace の synth の設計(5節、BR4.1〜BR4.4、NFR3.13、NFR3.17)の最後の一歩が成り立たず、実装者は推測しないと作れない | ts-helper.js が traceSites から from が trace の ConstructSite を作る変更を、論理の部品の表と型と呼ぶ場所の変更に足す(Analyzer のふるまいを変えないという書き方も直す)。Recheck が trace 由来の箇所をどう扱うかを決める(Recheck にも TraceSites を渡す、compareAnalyses で from が trace の箇所を比べない、など)。決めた方針を、8節の起動の回数のテストと Recheck のテストに足す | Resolved (reviewer) |
| R-02 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/nfr-design/security-design.md > 9節 動きの表(AppSkipped、AppStarted)と logical-components.md > 処理の順番 | 処理の順番では、外すアプリへの AppSkipped を AppsWaiting より先に送る(BR1.4)。AppsWaiting は新しい progress を作り apps を synth するアプリだけで決めるので、外したアプリには表の行が無い。表の 行を skipped にする と書いてあるが、行が無いときの描き方(表の中に出すのか、表の上に1行で出すのか)と、byApp に無いアプリへの AppStarted、AppDone が来たときの扱いが決まっていない。BR5.1 の状態に 外した があることとも合わない | 外したアプリの出し方(表に行を足す、または say で表の上に出す)と、未知のアプリの通知を無視するか行を足すかを9節に書く。AppsWaiting に外すアプリも含める案も比べて1つ選ぶ | Resolved (reviewer) |
| R-03 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/nfr-design/security-design.md > 5節 shortHome | 一致の直後が文字列の終わり、/、 のときだけ ~ にするので、ホームのパスの直後が引用符、コロン、括弧、空白のときは置き換わらない。また shortHome の home は os.UserHomeDir の値そのままだが、アプリのパスは EvalSymlinks した実際のパスから作るので、ホームがシンボリックリンクやリダイレクトの先にある環境では、--app の行と .venv の行にユーザー名が残る | 境目を、直後が英数字、アンダースコア、ハイフン、ドットでないことに広げる。home の EvalSymlinks した形も同じく置き換える。単体テストに両方を足す | Resolved (reviewer) |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/nfr-design/security-design.md > 12節 Construction での直し、10節 CI | 設計者が決めた契約の直し(契約 X2 の resource_name を BR3.2 に合わせる、NFR3.12 の見積もりの式、NFR6.17 の引用の対象を広げたこと)が12節の表に無く、traceability.json の NFR3.12、NFR6.17 も OK のまま。release.yml は自前の env を持つので、PY_TEST_APP と PYTHON_VERSION を足すかが書かれていない。ワークフローを読むテストの 対象の仕事 から、go test を -run で絞って動かす github-pr を除く条件も書かれていない(今の TestEveryJobThatRunsGoTestInstallsNode は go test を含むすべての仕事を見る) | 12節の表に X2 の resource_name、NFR3.12 と NFR6.17 の要件の直しを足し、traceability.json の note に差分を書く。release.yml の env への追加と、テストが github-pr を除く方法(run に -run がある仕事を除くなど)を10節に1行ずつ書く | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/nfr-design/security-design.md > 4節 cdk.cmd を経る受け渡しのテストと切り替え | Windows で cdk.cmd に二重引用符を含む引数を渡すと、Go の os/exec が批処理の引数として拒否する版がある。その場合、windowsQuotedApp を true にした既定は空白を含むフォルダーで必ず失敗する。切り替えを Code Generation のテスト結果に任せているが、既定の値と、false のときの利用者向けの案内を先に確かめずに決めている | Code Generation の最初に Go の版(stable と oldstable)で、引用符つきの引数が cdk.cmd に届くかを確かめる手順を計画に書く。届かないと分かったときは、既定を false にして README の案内を出す、と先に決めておく | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/nfr-design/security-design.md > 3節 pythonFor の順番 2 | app の命令の最初の語が python 系でない(pipenv など)とき、LookPath に失敗すると Tool が python の toolMissing になり、実際に無い道具の名前と合わない。BR2.1 は先頭の語が見つからないことを python が無いとする、としているため食い違いはないが、表示の入れ方の文(Python 3 を入れてください)が pipenv の欠落には合わない | 先頭の語が python 系でない場合は、Tool に先頭の語を入れるか、入れ方の文を出さないかを決めて3節に書く。Writer.Run が w.synth を持たない場合の selectApps の扱い(prepareCDK の今の nil の確かめと同じ)も7節に足す | Resolved (reviewer) |

> R-01 Reviewer note: 5節に補助のプログラム ts-helper.js が traceSites から from が trace の作成の箇所を作る処理(traceSites、traceNew)、Recheck に traces を渡す変更(cdkRun の traces を含む)、compareAnalyses の sameConstructSites(trace 由来はパスとファイルだけを比べる)が書かれた。論理の部品の表、型の変更の表、12節の契約の直し(C3、C4、ts-helper.js)、8節の起動の回数の表とテスト、traceability.json にも反映されている。今のコードの事実(helper が traceSites を読まない、Recheck が空で固定、ConstructSites 全体を比べる)と合っている。recheck のときだけ行がずれた式を2つ目の引数の文字列で探し直す決め方も妥当で、見つからなければ編集を捨てる側に失敗する

> R-02 Reviewer note: 9節に外したアプリの出し方の2案の比較があり、AppsWaiting の意味と BR1.4 の順番を変えない案 B(pendingSkipped に溜めて表を作るときに外した行にする)を選んでいる。見出しと足の数え方、経過時間の欄の扱いも書かれた

> R-03 Reviewer note: 5節で homeForms(Clean と EvalSymlinks の両方、長い順)と、直後が英数字、アンダースコア、ハイフン、ドット以外なら置き換える境目に直り、境目の表のテスト、リンクを使ったテストも足された

> R-04 Reviewer note: 12節に X2 の resource_name、NFR3.12、NFR6.17、C3、C4、BR3.2 などの直しが入り、traceability.json の NFR3.12 に要件との差も書かれた。10節に release.yml の env への追加と、Python の確かめの対象を go test を含み -run を含まない仕事とする条件(github-pr は外れる、4つ未満なら失敗)が書かれた

> R-05 Reviewer note: 4節に、Code Generation の最初に Windows で TestQuotedArgReachesCmd を stable と oldstable の両方で動かし、どちらかで届かなければ windowsQuotedApp を false にして README と残るリスクに案内を足す、という決め方が先に書かれた

> R-06 Reviewer note: 3節で、先頭の語が python 系でないときの Tool を toolName(最後の要素、Sanitize、64 文字、空なら python)にし、C2、entities.md、BR3.2 の直しを12節に入れた。7節に w.synth が nil のときは Discover と selectApps を呼ばない扱いとテストが足された

### Summary

前回の Major(trace 由来の作成の箇所が Recheck で落ちる)は、ts-helper.js、Recheck の引数、比べ方の3点の変更として設計に入り、今のコードと契約の直しの一覧、テスト、追跡表まで整合している。Minor 5 件もすべて設計で決まった。新しく決めたこと(C4 の traceSites を recheck にも広げる、recheck だけ文字列の ID で探し直す、toolName、ts-trace)は妥当で、新しい指摘は無い。Code Generation の計画で、作成の記録の実際の形(BR4.2)と Windows の引用の確かめを最初に行うこと。
