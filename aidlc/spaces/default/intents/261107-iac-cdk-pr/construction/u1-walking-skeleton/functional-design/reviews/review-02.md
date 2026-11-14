## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-08T08:38:47Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/functional-spec.md > 2. 流れ W1 手順 3〜9(FD-Q1 の「今の iac のファイルごとの流れに入れる」) | 既存の internal/iac に載せる方法が書かれておらず、そのままでは実装できない。(1) run.go の reflect は walkRoot が返す CloudFormation と Terraform の candidate だけを対象にし、.ts のファイルは candidate にならない。(2) editFile と simulate は、リソースごとのタグ一式(tagSite と tagEntry、logicalID か address で引く)を前提にしている。Tags.of(x).add の1か所は、リソースのタグ一式ではない。(3) checkEdits は parseContent で同じ形式を読み直すが、CDK は BR3.4 のとおりアプリごとの Recheck で、しかも --write のときだけ。差分だけを見せる実行では、書き換えの確かめが走らない。(4) Result への追加(ToolCDK、FileResult の Language、CDKApps、WrittenFiles、新しい ManualReason)、CdkTarget の account と region(今の target 構造体にも buildTargets にも無い)が entities.md に無い | 取り込み方を1つに決めて書く。CDK のソースを candidate と tagSite の並びにどう写すか(または CDK 用の parsedFile を足すか)、アプリごとの Recheck を processAll と finish のどの間に置くか、--write が無いときの確かめをどうするかを、functional-spec.md に足す。entities.md には Result と target への追加(ToolCDK、WrittenFiles、CDKApps、account と region、U1 で足す ManualReason の一覧)を足す | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/rules.md > BR2.2、BR3.1、BR3.2 | 最小の一本の中心である「Tags.of(x) の効く範囲(ScopeConstructPath)と作成の箇所を、ソースからどう決めるか」の規則が無い。BR2.2 は「パスを渡して箇所を受け取る」とだけ書く。Tags.of(bucket) の bucket から new Bucket(this, 'Id') をたどり、スタックの ID(別ファイルの new MyStack(app, 'id') にある)を足してパスを作る流れが未定義。また、同じリソースに合う Tags.of の箇所が0か所、2か所以上のときの扱いが無い(BR3.3 は0か所を keyNotInSource とするだけで、2か所以上は cdkAmbiguous の対象外)。同じスタックのクラスを2回使い、複数のリソースが同じ Edit に対応する場合の契約 C7 の same_site_rule も取り込まれていない | 補助のプログラムが範囲のパスを決める規則(U1 で扱う形だけ)を BR2.2 に足す。合う箇所が0か所と2か所以上のときの結果(手で直す一覧の理由)と、同じ Edit に複数のリソースが対応するときの扱いを BR3.1 から BR3.3 に足す。必ず通る受け入れの例(デモのアプリ)を、期待するパスと行つきで書く | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/traceability.json > coverage の AC2.1.2、AC2.1.3、AC3.1.1 | 3つの受け入れ条件が OK になっているが、観察できる結果に当たる規則が無い。AC2.1.2 は標準エラーのアプリごとの状態と経過時間で、規則は BR1.1、BR1.2、BR1.4 だけ。表示は U3 に回す(functional-spec.md の1節)と書いたまま、契約 C6 の持ち主は U1。AC2.1.3 の後半(AWS を引くアプリは失敗になり、理由と対処が出る)は、BR1.5 の「既知の文」の一覧が U3 の設計待ちで、U1 ではテストで固定できず、理由の文は U6。AC3.1.1 は「書き換えたあとにもう一度 cdk synth すると新しいキーになる」ことが条件だが、BR3.4 は静的な読み直しだけで、再 synth を確かめる規則も検証の筋書き(8節)も無い | U1 が満たす範囲を決めて書く。最小の状態表示(待ち、実行中、済み、失敗と経過時間)の規則と、needsAWS の最小の判定文と対処の文(U3、U6 で置き換える前提)を足すか、受け入れ条件を一部の達成として traceability.json で区別する。AC3.1.1 には、書き換えたアプリを再 synth して新しいキーを確かめる受け入れテスト(または検証の手順)を足す | Resolved (reviewer) |
| R-04 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/rules.md > BR4.2 | 書き込みが途中で失敗した場合が決まっていない。run.go の write は、途中のファイルで失敗すると前のファイルを Written のまま残し、WriteErr を立てる(中断のときは Interrupted)。BR4.2 の条件は「書き換えたファイルが1つ以上」だけなので、一部だけ書けた状態でブランチ、コミット、push、プルリクエストの作成に進み、中途半端な内容を公開リポジトリに出せてしまう。functional-spec.md の W2 の手順 4 にもこの分岐が無い | BR4.2 と W2 に、WriteErr か Interrupted があるときは Publish を呼ばず、終了コード 1 で済んだ書き込みと残りを示す、と足す。その受け入れテスト(2つ目のファイルの書き出しを失敗させる)を足す | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/rules.md > BR1.5、BR1.2 | 補助のプログラムの失敗(typescript が node_modules に無い、2分の上限、内部の失敗、読めないファイル)を手で直す一覧へどう出すかの規則が無い。状態図には「補助のプログラムが動かない」があるが、対応する BR が無い。テスト用のアプリで npm ci をしていない場合に最初に当たる。また BR1.2 は、契約 C2 の detection にある「アプリのフォルダーから --dir までさかのぼって依存を探す」を書いていない | 契約 C7 の helper_failure_to_manual を BR として取り込み、補助のプログラムの上限(案は2分)を決める。BR1.2 に、依存の宣言をさかのぼって探す規則を足す | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/rules.md > BR1.3、BR1.4 | 認証情報の遮断に抜けがある。(1) Windows では環境変数の名前の大文字小文字を区別しないので、aws_ で始まる小文字の変数も外す規則が必要。(2) synth が動かす利用者のアプリは、HOME の下の gh の設定(hosts.yml)など、環境変数でない認証情報を読める。これは残るリスクとして書かれていない。(3) cdk と node をどの実行ファイルで呼ぶか(PATH の cdk、node_modules の cdk、npx)と、--output と標準出力の扱いが無い。(4) 契約の未決事項で U1 が止まるもの(CDK_DISABLE_VERSION_CHECK で外への問い合わせが止まるかの確認、補助のプログラムを埋め込む形のライセンスの扱い)に答えが無い | 名前の照合を大文字小文字を区別しない形にする。残るリスクを rules.md に明記する。呼び出す実行ファイルと引数を BR1.4 に足す。U1 の未決事項2件に答えを書く(確かめた結果、または確かめ方と、ライセンスの表示が不要な理由か表示の方法) | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/rules.md > BR4.3、BR4.4、BR4.5 | (1) 既定のタイトルとコミットのメッセージに変更案のファイル名が入るが、伏せ字の対象は本文だけで、ファイル名に12桁の数字(アカウント ID)や ARN の形があると、公開リポジトリのタイトルとコミットに出る。(2) コミットと push は、フックと署名のために標準入力と標準エラーを端末につなぐ(契約 C5)と決まっているが、BR4.5 に無い。(3) PullRequestPlan の remoteUrl を表示や記録に使うとき、URL に資格情報が入っている場合の扱いが無い | 伏せ字の規則をタイトルとコミットのメッセージにも適用する。標準入力と標準エラーの扱いを BR4.5 に足す。表示する remoteUrl から userinfo を除く規則を足す | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/rules.md > BR5.1、BR5.2 と functional-spec.md の 8節 | CI と検証の手順に抜けがある。(1) テスト用のアプリと検証用の clone で npm ci を実行する手順が無く、実行しないと synth も補助のプログラムも動かない。(2) BR5.2 の「トークンの環境変数がある」は、fork のプルリクエストで secret が空の文字列になる場合を区別していない。環境変数の名前も未定。(3) CI の clone と push の認証の渡し方(ワークフローの定義にもテストのデータにもトークンを書かない)が無い。(4) 新しい CI の仕事が必須のチェックの ci-gate に含まれるかが無い | BR5.1 と BR5.2 に、npm ci の手順、「空でないとき」の条件、環境変数の名前、認証の渡し方(CI の secret と gh の設定だけ)、ci-gate への組み込みを書く。8節の手順 4 に npm ci を足す | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/functional-spec.md > 7節 端の場合(キーが変数や式) | キーが変数や式のときを cdkExpression としているが、契約 C3 では、定数や辞書から引く書き方は別の Shape(sameFileConst、sameFileMap)で、U4 で扱う形。U1 では扱わない形として cdkUnsupportedShape になるはずで、rules.md の BR3.1 の violation の2文とも食い違う | cdkExpression にするのは Shape が expression のときだけ、定数や辞書は U1 では cdkUnsupportedShape、と BR3.1 と端の場合の表をそろえる | Resolved (reviewer) |
| R-10 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/functional-spec.md > 2節 場所と読み取りの行 と rules.md BR3.1 | 対象の場所(place)を「変更前のキーの箇所のあるファイルと行」としているが、変更前のキーの箇所が0か所、2か所以上、またはコンストラクトの箇所が無い場合は、ファイルが1つに決まらない。今の iac は index.find が placeTargets の中で見つからない対象を手で直す一覧に出すが、CDK で cdkNoSource、keyNotInSource、cdkAmbiguous、cdkScopeTag を誰がどの段階で出すか、ManualItem の File と Line に何を入れるかが書かれていない。また、タグ一式が複数のファイルの箇所を含むとき、simulate と buildEdits が前提にする1つのファイルの内容との関係が書かれていない | 場所が決まらない場合は CDK の場所の決定が placeTargets の中で今の index.find と同じように手で直す一覧に出す、と書き、File と Line に入れる値(コンストラクトの箇所、無ければアプリのフォルダー)を決める。他のファイルの箇所は書き換えを作らず、数え方と競合の判定にだけ使う、と足す | New |
| R-11 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/rules.md > BR3.4 と BR3.1 | 契約 C7 は recheck_failed を cdkUnsupportedShape で手で直す一覧に出すと決めているが、BR3.4 は unreadableFile にしている。BR3.4 の trigger は「--write があり」だが、logic と W1 の手順 9 は --write の有無によらず行う、と食い違う。BR3.1 はキーか値のどちらかが式なら cdkExpression とするが、契約 C3 の change_to_edit は、キーの名前の変更では使う範囲(KeyEdit)の Shape だけを見る | BR3.4 の理由を契約に合わせるか、unreadableFile にする理由と contract-summary の変更が要るかどうかを書く。trigger を logic に合わせる。値が式のときの扱いが U1 の絞り込みだと分かるように BR3.1 に書き、U4 で契約どおりにする、と残す | New |
| R-12 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/rules.md > BR5.2 と BR5.1 | push の認証は git -c で gh の資格情報のヘルパーを指す、とあるが、push は tag-refactor の Publish が自分で動かす git で、テストが付ける -c は届かない。テスト用の clone の設定として渡す方法(clone の中の git config、GIT_CONFIG の環境変数、GIT_TERMINAL_PROMPT=0)が決まっていない。Windows の CI では Git Credential Manager の質問が出るおそれがある。BR5.1 の「node_modules をつなぐ」は、Windows でシンボリックリンクかジャンクションかが決まっていない | clone の中の git config(credential.helper を空にしたあとに gh のヘルパー)を設定する手順と、質問を出さない設定を BR5.2 に書く。node_modules を3つの OS でどうつなぐか(または写すか)を BR5.1 に決める | New |
| R-13 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/functional-design/rules.md > BR4.1 と functional-spec.md の4節 W2 手順 2 | 契約 C1 の Check は、行き先のブランチ(base)がリモートに無ければ止めると決めているが、BR4.1 と W2 の手順 2 にはその確かめが無い。無いまま進むと、push のあとの作成の段階で初めて失敗し、公開リポジトリにブランチだけが残る。W2 の手順 2.1 の merge や rebase の途中の確かめも BR4.1 に無い | BR4.1 に、行き先のブランチがリモートにあることと、merge や rebase の途中でないことを足す。足さないなら、U2 に回すと書き、リモートにブランチが無い場合の結果を端の場合の表に足す | New |

> R-01 Reviewer note: functional-spec.md の2節に、取り込みが4か所(対象の決定、候補のファイル、場所と読み取り、書き換えの確かめ)の表で決まった。CDK の候補は補助のプログラムが名指したファイルだけにし、Recheck は processAll と finish の間で --write の有無によらず行う。Result と target への追加は entities.md の IacResultAdditions と CdkTarget に入った。run.go の実際の構造(reflect、placeTargets、processFile、editFile、finish)と矛盾しない

> R-02 Reviewer note: BR2.2 に、入口のファイルのスタックの ID からコンストラクターの中の new と Tags.of の範囲までのパスの組み立てと、前方一致の規則が入った。0か所は cdkNoSource または keyNotInSource、2か所以上は cdkAmbiguous(BR3.1)、同じ Edit に複数のリソースが対応するときは SharedWith と BR3.2 で決まった。デモのアプリの期待するパスの例もある

> R-03 Reviewer note: AC2.1.2 は BR1.7、AC2.1.3 は BR1.5 の既知の文4つと対処の文、AC3.1.1 は BR3.5(synth し直して新しいキーを確かめる)に当たる規則が入り、traceability.json の対応先と1節の「U1 で満たす部分」の表がそろった

> R-04 Reviewer note: BR4.2、W2 の手順 4、状態図の checked から stopped、端の場合の表に、WriteErr か Interrupted のときは作成を呼ばず終了コード 1、という分岐が入った

> R-05 Reviewer note: BR1.6 が補助のプログラムの失敗を failed か SkippedFiles に対応させ、上限を2分にした。BR1.2 に依存の宣言をさかのぼって探す規則が入った

> R-06 Reviewer note: BR1.3 が名前の照合を大文字小文字を区別しない形にし、residual_risk を明記した。BR1.4 が呼ぶ実行ファイルと引数を決めた。U1 の未決事項3件にも答えがある(CDK_DISABLE_CLI_TELEMETRY の効きは NFR Design で確かめる、と書かれている)

> R-07 Reviewer note: BR4.3 がタイトルとコミットのメッセージに伏せ字を適用し、BR4.5 が標準入力と標準エラーの扱いと remoteUrl の userinfo の除去を決めた

> R-08 Reviewer note: BR5.1 に npm ci と ci-gate への組み込み、BR5.2 に空でないときの条件、環境変数の名前、認証の渡し方が入り、8節(9節の検証の筋書き)の手順 4 にも npm ci が入った。ただし認証の渡し方の細部は下の新しい指摘にある

> R-09 Reviewer note: 定数や辞書は cdkUnsupportedShape、式だけが cdkExpression に、BR3.1、entities.md、端の場合の表がそろった

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| (宣言なし) | 実行せず | このステージには検証ツールの宣言が無い。代わりに internal/iac/run.go と target.go の実際の構造、planner.Resource の Region と Global、SkippedFiles の有無、契約 C1 から C7 との突き合わせで確かめた |

### Summary

1回目の指摘9件(Major 4、Minor 5)は、すべて直っている。取り込み方、範囲のパスの組み立て、途中で書き込みが失敗した場合、受け入れ条件との対応が、実装できる精度で書かれ、既存のコードの構造とも矛盾しない。改訂で入った新しい指摘は Minor 4件だけで、Critical も Major も無いので READY とする。
