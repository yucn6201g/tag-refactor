## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-09T09:56:26Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 1節(続きの命令を実際のシェルで動かすテストの行と cli の受け入れテストの行) | ci.yml は defaults.run.shell を bash にしており、windows-latest でも go test は Git Bash から起動される。Git Bash の環境では MSYSTEM が設定されている(この作業環境の bash では MINGW64 だった)。security-design.md の4節では、DefaultShell は Windows で MSYSTEM が無いときだけ PowerShell を返すので、Windows の CI では DefaultShell が sh を返す。DefaultShell を通る cli の受け入れテスト(push を失敗させて、命令に本文が無く --body-file がある、commit が2行に分かれることを確かめる)は、Windows の CI で PowerShell 用ではなく sh 用の命令と表示を受け取る。この前提が1節にも3節にも書かれておらず、変える定義が無いという結論が、Windows のテストで崩れるおそれがある。 | 1節に、DefaultShell を通るテストは、使うシェルをテストの中で固定すること(Shell を引数で渡す、または t.Setenv で MSYSTEM を消すか設定する)を書く。あわせて、実際のシェルで動かすテストが DefaultShell に頼らず、Windows の CI で MSYSTEM が設定されていても powershell.exe と pwsh の両方で動くことを書く。 | New |
| R-02 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 3節 | 見込みの式が「値の数 × 2つの版」となっているが、Go の版(stable と oldstable)は別々の仕事で、1つの仕事の中で2回かかるのは PowerShell の2つの版(powershell.exe と pwsh)である。数え方がどちらか読み取れない。1回あたり 1 秒前後という数字も、測った値か推定か書かれておらず、Windows PowerShell 5.1 の起動は遅いことがある。また、実際に効く上限は仕事の 30 分ではなく、go test が1つのパッケージごとに持つ既定の 10 分で、cli のパッケージには cdk synth や npm を動かす既存のテストも入っている。 | 見込みを「1つの仕事の中でシェルを起動する回数 × 1回あたりの時間」の形で書き直す。1回あたりの時間が推定であることを書く。シェルのテストを置くパッケージ(pullrequest か cli)と、go test の既定の 10 分に対する余裕を書き、近ければ -timeout を足すかどうかを決める。 | New |
| R-03 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 1節(サブテスト options の行)と3節(github-pr) | github-pr の増える時間を「1分ほど」としているが、今の TestGitHubPullRequest は1回の中で gh repo clone、npm ci、go build、変更案の保存、iac --pr、gh pr diff を動かしている。サブテストを2つに分けたとき、clone、npm ci、ビルドを共有するのか、サブテストごとにやり直すのか、同じ clone の中で default の後に options が始まる状態(切り替わったブランチと書き換えたファイル)をどう戻すのかが書かれていない。共有しないなら増える時間は1分では済まず、共有するなら状態の持ち越しが要る。 | サブテストごとに別の clone と作業のフォルダーを使うのか共有するのかを書き、それに合わせて増える時間を数え直す。20 分の上限に対する余裕もあわせて書く。 | New |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 1節(続きの命令のテストの行)と 4節 | 実際のシェルで動かすテストが、Linux と macOS で powershell.exe や pwsh を要求するのか、Windows 以外では飛ばすのかが書かれていない。CI=true で見つからなければ失敗にする規則(NFR8.1 と同じ流儀)のまま、Linux や macOS の仕事に pwsh を要求すると、pwsh が無い環境で落ちる。release.yml も go test ./... を ubuntu で動かすので、同じ前提の影響を受けるが、release.yml について何も書かれていない。 | PowerShell のテストは runtime.GOOS が windows のときだけ動かし、Linux と macOS では飛ばす(sh のテストだけ動かす)ことを1節に書く。release.yml の Linux の go test も同じ前提で通ることを4節に一言書く。 | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/traceability.json > coverage NFR6.13、NFR1.10 | NFR6.13 の target は5節だけを指すが、5節は古い枝の掃除の読み方しか書いておらず、既定のブランチ名の日時を UTC と Z で作り、時計を差し替えて確かめる単体テストは、1節の「ブランチ名」の一語にしか対応しない。NFR1.10 の target も pullrequest の単体テストだけを指し、nfr-design 側の確かめ方にある cli の受け入れテスト(中断で本文のファイルの場所と続きの命令が出る)が入っていない。表のまま読むと、確かめる場所が抜けていると判断できない。 | 1節の表に、既定のブランチ名の UTC とクロックの差し替えのテスト、本文のファイルの中断時の表示のテストを行として足し、NFR6.13 と NFR1.10 の target をそれらに向ける。 | New |

### Summary

U2 のテストは、仕事、ジョブの範囲、-run の条件(サブテストも動く)、secret の渡し方(github-pr の1手順だけ)の点で、今の ci.yml を変えずに動くという主張はおおむね成り立ちます。ただし、Windows の CI は Git Bash から go test を起動するため、MSYSTEM によって DefaultShell が sh を返し、DefaultShell を通る受け入れテストが崩れるおそれがあり、この前提が設計に無いため NOT-READY とします。時間の見込みの数え方、サブテストの状態の持ち越し、PowerShell のテストを動かす OS、traceability の対応先は、修正のうえで確かめてください。質問を省いたことは、要件と設計の段階の回答(NQ2-Q1、ND2-Q1、ND2-Q2)で決まっているので問題ありません。
