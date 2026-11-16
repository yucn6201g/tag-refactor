## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-15T07:32:29Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/functional-design/rules.md > BR2.2、BR2.1 | BR2.2 は cdk.json の app の命令の最初の語(python3 など)を使う Python とし、.venv があれば .venv の実行ファイルのフォルダーを PATH の先頭に置くとしている。しかし .venv の存在の判定は Windows で .venv の Scripts の python.exe だけを見ており、命令の語が python3 のとき(cdk init の Python の雛形の app は python3 app.py)、Windows の .venv には python3.exe が無いので、PATH の先頭に置いても python3 は .venv の Python に解決されない。BR2.1 の道具の確かめ(PATH から最初の語を探す)も、.venv があるのに python3 が無いというだけで toolMissing(python)にしてしまう。.venv を使う形を3つの OS の CI で確かめる BR6.1 の py-minimal が、Windows で成り立つか決まっていない | .venv がある場合は、命令の最初の語にかかわらず .venv の Python で app の命令を動かす決まりにする(語の読み替え、または命令の先頭の語を .venv の Python の絶対パスに置き換えるなど、どちらかを選んで書く)。BR2.1 の確かめも、.venv の Python があればそれを見つけたものとして扱う。語が python3 の Windows の例を受け入れ条件のテストに足す | New |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/functional-design/rules.md > BR1.3 | Python の版の判定が、requirements.txt、setup.py、pyproject.toml の同じフォルダーでは最初に見つかった1つのファイルだけで決まる書き方になっている。契約 C2 の detection は、3つのファイルのどれかに aws-cdk-lib があれば 2 としている。古い cdk init の雛形のように、requirements.txt が -e . だけで aws-cdk-lib は setup.py の install_requires にある形では、requirements.txt だけを見て版が 0(unknownVersion)になり、v2 のアプリが外される(AC2.1.1、AC2.1.2 に反する)。さらに、依存の名前をファイルの種類ごとにどう取り出すか(requirements.txt の版指定、extras、コメント、-r の取り込み、setup.py の install_requires、pyproject.toml の dependencies と poetry の表)が決まっておらず、実装者が推測することになる | 同じフォルダーの3つのファイルすべてを調べ、どれかに aws-cdk-lib があれば 2、aws-cdk-core があれば 1 とする(契約 C2 と合わせる)。ファイルの種類ごとに、依存の名前の取り出し方と、読まないもの(-r、-e、URL など)の扱いを表で決める。-e . の形の requirements.txt と setup.py の組をテストに足す | New |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/functional-design/functional-spec.md > W3 の 1〜3、rules.md > BR3.3、BR4.1、BR4.2 の note | Python のアプリは U5 までソースを読まない(BR3.3、W3 の 1)のに、W3 の 2 は条件なしで作成の箇所が見つからないリソースを集めるため、Python のアプリではすべてのリソースが見つからない扱いになり、W3 の 3 で trace の synth(最大で synth の上限の時間)を無駄に動かし、W3 の 7 で存在しない Python の補助のプログラムを呼ぶことになる。BR4.1 の trigger は「1回目の Analyze が終わったとき」で、Python のアプリには Analyze が無い。一方で BR4.2 の note は Python のアプリで trace を読む前提で書かれており、BR3.3、BR4.1、BR4.2 の間で Python のアプリが trace の synth をするのかしないのかが食い違っている | Python のアプリは W3 の 1 の直後で終えること(BR3.3 の cdkUnsupportedShape の扱いだけを行い、trace の synth と2回目の Analyze はしない)を W3 と BR4.1 に明記する。BR4.2 の note と Python の TraceSite の扱いは、U5 で扱うものとして U3 の範囲から外すか、U3 でやるなら Analyze の無い Python での流れを決める | New |
| R-04 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/functional-design/functional-spec.md > W4、rules.md > BR5.1、BR5.2 | 表を毎秒更新する、1分ごとに1行を足す、CDK の処理の終わりに表を閉じて1分ごとの行を止める、という動作が要るが、契約 C6 の cdk.ProgressSink(AppsWaiting、AppStarted、AppDone、AppFailed、AppSkipped)には、時間で動く更新も終わりの知らせも無い。現在の display.Printer(internal/display/progress.go)は知らせが来たときだけ描き直す作りで、時間で動く仕組みを持たない。タイマーを誰が持ち(Printer か synth の側か)、終わりをどう知るか(新しいメソッドか Printer の別の口か)、ProgressSink にメソッドを足すと実装(テストの差し替えを含む)がすべて壊れるという契約の決まり(R-04)をどう扱うかが書かれておらず、実装者が推測することになる。Printer が別のゴルーチンから描くなら、他の出力との競合(-race のテスト)の扱いも要る。trace の synth で AppStarted を再び送ることと AppDone を2回送ることが C6 に合うかも書かれていない | W4 に、時間で動く更新(毎秒、1分ごと)と終わりの知らせの持ち主と口を決めて書く。契約 C6 を変える(メソッドや終わりの口を足す)なら、その旨と実装の直し方を contract-summary の変更として挙げる。trace の synth のときの知らせの送り方(AppStarted と AppDone を再び送るか)と、表示を閉じる前に Analyze を待つあいだの状態の見え方も決める | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/functional-design/rules.md > BR1.1 と BR1.2 | BR1.1 は stack-name のタグのリソースが無いとき「探さず」とするが、BR1.2 は同じ場合にも --cdk-app の値を発見の結果と照らして確かめるとしており、発見をしないと確かめられない。W1 でも確かめ(1)が発見(3)より前に書かれている。NFR5 の「今と変えない」は、発見だけなら読み取りだけなので守れるが、決まりが矛盾している | BR1.1 を「synth はしない。ただし --cdk-app があれば検証のために発見だけは行う」のように直し、W1 の順番も合わせる | New |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/functional-design/rules.md > BR3.2、BR3.3 | BR3.2 の「failed のアプリ」には補助のプログラムの失敗で failed にしたアプリが含まれる。しかしそのアプリは synth に成功してスタックの照合(BR3.1)に加わったあとなので、候補が0のリソースがそのアプリのものでないことは分かっている。それなのに、別のアプリが CDK v1 で外されていても、cdkAppFailed が優先され、失敗が1つならその道具の名前に helper などが添えられて、PR の本文に誤った理由が出る | BR3.2 の優先の判定では、synth に失敗したアプリだけを数える(補助のプログラムの失敗で failed にしたアプリは除く)ことを決めて書く。その場合の表示のテストも足す | New |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/functional-design/functional-spec.md > W2 の 1 と 3、状態の移り変わりの表、例 1 | 道具が無いときの状態の移り変わりが3か所で食い違う。W2 は道具の確かめ(1)を「実行中」の知らせ(3)より前に置き、例 1 も待ちから直接失敗になっている。状態の表は running から failed としており、契約 C2 は AppStarted を送ってから AppFailed を送るとしている。実装者は AppStarted を送るか決められない | 道具が無いときに AppStarted を送るかを決め、W2、状態の表、例 1、BR5.1 と BR5.2 の表示を同じにそろえる | New |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/functional-design/rules.md > BR1.1、BR4.2 | 発見は .git、node_modules、cdk.out しか避けず、Python の .venv、venv、site-packages の下は辿る。BR4.2 は TraceSite でこれらを除いており、不揃いである。Python のアプリの下に .venv があると、数万のファイルを深さ32まで辿り、パッケージの中の cdk.json を別のアプリと誤って見つける可能性がある | 発見で .venv と venv も入らないフォルダーに足すか、足さない理由を書く(契約 C2 の発見は変えずに U3 の決まりとして足せる) | New |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u3-cdk-synth/functional-design/rules.md > BR2.5、BR5.1、BR5.2 | U1 の BR1.4(一時的なフォルダーと時間の上限)と BR1.7(進み具合の表示)を U3 が仕上げ直しているのに、BR2.5 と BR5.1、BR5.2 には replaces が無い。BR2.4 だけが U1 の BR1.4 の上限を置き換えるとしており、U1 のどの決まりが U3 のどれに引き継がれるかが追えない。W5 の中断も、動いている synth だけを止めると書き、Analyze や Recheck の実行中の中断(ctx の取り消し)の扱いを書いていない | BR2.5、BR5.1、BR5.2 に replaces を足す。W5 に補助のプログラムの実行中の中断(子まで止める、一時ファイルを消す)の扱いを足すか、U1 の扱いのままとすると書く | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| entities.md の YAML の読み込み | PASS(entities 10件、relationships) | 構文は正しい |
| rules.md の YAML の読み込み | PASS(rules 22件、必須の欄あり) | 構文は正しい。BR のうち traceability.json に出ないものは無い |
| traceability.json | PASS(JSON として正しい。OK の target はすべて rules.md に存在する BR) | AC2.1.1 から AC2.1.6、AC4.1.3、AC4.1.4 の8件すべてを割り当てており、単位の対応表(US2.1 と US4.1 の U3 分)と合う |
| required-sections、upstream-coverage、traceability、linter、type-check | 設計の文書には機械の確かめの対象が無く、上の読み込みと照合で代えた | 構造の欠けは無い。問題は内容の整合にある |

### Summary

構造と受け入れ条件の対応は整っているが、Python のアプリの扱い(.venv の Python の選び方、版の判定、Analyze の無いときの trace の synth)と、進み具合の表示の時間で動く部分の持ち主が決まっておらず、実装者の推測に頼る箇所が4つの Major としてある。これらを直すまでは READY にできない。
