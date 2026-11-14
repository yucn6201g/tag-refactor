## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-09T09:58:28Z
**Iteration:** 2

### Prior findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 1節(続きの命令を実際のシェルで動かすテストの行と cli の受け入れテストの行) | Windows の CI は Git Bash から go test を起動し MSYSTEM が入るため、DefaultShell が sh を返し、DefaultShell を通る受け入れテストの前提が崩れるおそれがあった。 | 使うシェルをテストの中で固定することと、実際のシェルのテストが DefaultShell と MSYSTEM に頼らないことを書く。 | Resolved (reviewer) |
| R-02 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 3節 | 実行時間の見込みの式の数え方が曖昧で、測った値か推定かも不明、効く上限も誤っていた。 | 起動の回数 × 1回の時間の形に直し、推定と明記し、パッケージと go test の既定 10 分に対する余裕を書く。 | Resolved (reviewer) |
| R-03 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 1節(サブテスト options の行)と3節(github-pr) | github-pr のサブテストで clone、npm ci、ビルドを共有するのか、状態の持ち越しをどうするのかが不明だった。 | 別々の clone か共有かを決め、時間を数え直して 20 分の上限に対する余裕を書く。 | Resolved (reviewer) |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 1節(続きの命令のテストの行)と 4節 | PowerShell のテストを Linux と macOS で要求するのか不明で、release.yml の ubuntu の go test への影響も書かれていなかった。 | PowerShell のテストは Windows だけで動かすと書き、release.yml の Linux の go test も通ると書く。 | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/traceability.json > coverage NFR6.13、NFR1.10 | NFR6.13 と NFR1.10 の target が、確かめるテストの置き場所と合っていなかった。 | 1節の表に該当するテストの行を足し、target をそれに向ける。 | Resolved (reviewer) |

### New findings

None.

### Summary

R-01 から R-05 はすべて直っています。1節の表にテストごとのパッケージと前提が入り、「シェルの固定」(Shell を引数で渡す、MSYSTEM を t.Setenv で選ぶ、DefaultShell の単体テスト)が ci.yml の Git Bash という前提(security-design.md の DefaultShell の定義)と合っています。PowerShell のテストは Windows だけ、sh のテストは Linux と macOS だけ(race と coverage は ubuntu で動くので表の記述も ci.yml と合う)で、release.yml の ubuntu でも通ります。3節は起動の回数 × 1回の時間、既定の 10 分と 20 分の上限、clone ごとに別とビルド1回の決めまで一貫しており、traceability.json の対応先も 1節の行に向いています。新しい指摘はなく、READY とします。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 1節(続きの命令を実際のシェルで動かすテストの行と cli の受け入れテストの行) | ci.yml は defaults.run.shell を bash にしており、windows-latest でも go test は Git Bash から起動される。Git Bash の環境では MSYSTEM が設定されている(この作業環境の bash では MINGW64 だった)。security-design.md の4節では、DefaultShell は Windows で MSYSTEM が無いときだけ PowerShell を返すので、Windows の CI では DefaultShell が sh を返す。DefaultShell を通る cli の受け入れテスト(push を失敗させて、命令に本文が無く --body-file がある、commit が2行に分かれることを確かめる)は、Windows の CI で PowerShell 用ではなく sh 用の命令と表示を受け取る。この前提が1節にも3節にも書かれておらず、変える定義が無いという結論が、Windows のテストで崩れるおそれがある。 | 1節に、DefaultShell を通るテストは、使うシェルをテストの中で固定すること(Shell を引数で渡す、または t.Setenv で MSYSTEM を消すか設定する)を書く。あわせて、実際のシェルで動かすテストが DefaultShell に頼らず、Windows の CI で MSYSTEM が設定されていても powershell.exe と pwsh の両方で動くことを書く。 | New |
| R-02 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 3節 | 見込みの式が「値の数 × 2つの版」となっているが、Go の版(stable と oldstable)は別々の仕事で、1つの仕事の中で2回かかるのは PowerShell の2つの版(powershell.exe と pwsh)である。数え方がどちらか読み取れない。1回あたり 1 秒前後という数字も、測った値か推定か書かれておらず、Windows PowerShell 5.1 の起動は遅いことがある。また、実際に効く上限は仕事の 30 分ではなく、go test が1つのパッケージごとに持つ既定の 10 分で、cli のパッケージには cdk synth や npm を動かす既存のテストも入っている。 | 見込みを「1つの仕事の中でシェルを起動する回数 × 1回あたりの時間」の形で書き直す。1回あたりの時間が推定であることを書く。シェルのテストを置くパッケージ(pullrequest か cli)と、go test の既定の 10 分に対する余裕を書き、近ければ -timeout を足すかどうかを決める。 | New |
| R-03 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 1節(サブテスト options の行)と3節(github-pr) | github-pr の増える時間を「1分ほど」としているが、今の TestGitHubPullRequest は1回の中で gh repo clone、npm ci、go build、変更案の保存、iac --pr、gh pr diff を動かしている。サブテストを2つに分けたとき、clone、npm ci、ビルドを共有するのか、サブテストごとにやり直すのか、同じ clone の中で default の後に options が始まる状態(切り替わったブランチと書き換えたファイル)をどう戻すのかが書かれていない。共有しないなら増える時間は1分では済まず、共有するなら状態の持ち越しが要る。 | サブテストごとに別の clone と作業のフォルダーを使うのか共有するのかを書き、それに合わせて増える時間を数え直す。20 分の上限に対する余裕もあわせて書く。 | New |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/cicd-pipeline.md > 1節(続きの命令のテストの行)と 4節 | 実際のシェルで動かすテストが、Linux と macOS で powershell.exe や pwsh を要求するのか、Windows 以外では飛ばすのかが書かれていない。CI=true で見つからなければ失敗にする規則(NFR8.1 と同じ流儀)のまま、Linux や macOS の仕事に pwsh を要求すると、pwsh が無い環境で落ちる。release.yml も go test ./... を ubuntu で動かすので、同じ前提の影響を受けるが、release.yml について何も書かれていない。 | PowerShell のテストは runtime.GOOS が windows のときだけ動かし、Linux と macOS では飛ばす(sh のテストだけ動かす)ことを1節に書く。release.yml の Linux の go test も同じ前提で通ることを4節に一言書く。 | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/infrastructure-design/traceability.json > coverage NFR6.13、NFR1.10 | NFR6.13 の target は5節だけを指すが、5節は古い枝の掃除の読み方しか書いておらず、既定のブランチ名の日時を UTC と Z で作り、時計を差し替えて確かめる単体テストは、1節の「ブランチ名」の一語にしか対応しない。NFR1.10 の target も pullrequest の単体テストだけを指し、nfr-design 側の確かめ方にある cli の受け入れテスト(中断で本文のファイルの場所と続きの命令が出る)が入っていない。表のまま読むと、確かめる場所が抜けていると判断できない。 | 1節の表に、既定のブランチ名の UTC とクロックの差し替えのテスト、本文のファイルの中断時の表示のテストを行として足し、NFR6.13 と NFR1.10 の target をそれらに向ける。 | New |

> R-01 Not re-checked this round

> R-02 Not re-checked this round

> R-03 Not re-checked this round

> R-04 Not re-checked this round

> R-05 Not re-checked this round
