## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-08T04:15:13Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/domain-design/components.md > component IacWriter behaviour と component CdkSynthesizer behaviour | 変更案のリソースが CDK のものかどうかは「CdkSynthesizer の結果のスタックと論理 ID に一致するもの」で決める。ところが synth に失敗したアプリは結果の一覧を返さないので、そのアプリのリソースは一致せず、今までどおりの CloudFormation か IaC の外に回る。FR2.9 は「失敗したアプリのリソースを理由つきで手で直す一覧に出す」と求めており、US4.1 の合否もここにかかるが、失敗したアプリのリソースをどう見分けるかが部品のどこにも書かれていない。2つのアプリの片方が失敗する場合、誤った理由(templateNotFound など)で一覧に出るか、「IaC の外」に数えられて一覧に出ないおそれがある。 | 失敗したアプリのリソースの帰属を決めて書く。たとえば、スタック名のタグがあるのにどの成功したアプリのスタックにも一致しないリソースは、失敗したアプリがあるときは、そのアプリの失敗の理由つきで手で直す一覧に出す、と IacWriter の振る舞いに足す。あわせて、アプリが失敗した場合の IacResult の cdkApps と ManualItem の理由の関係も書く。 | New |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/domain-design/components.md > component CdkSynthesizer behaviour(環境の記述) と decisions.md > ADR-001 Security and compliance | 認証情報の遮断が「AWS の認証情報の環境変数とプロファイルの指定は渡さない」だけである。AWS の SDK と CDK の CLI は、環境変数が無くても `~/.aws/credentials` と `~/.aws/config` の既定のプロファイルや、EC2 などのメタデータから認証情報を得る。FR2.4 の合否は「AWS への呼び出しは起きない」だが、環境変数を外すだけではそれを保てず、利用者が既定の認証情報を持つ端末では、保存されていない値の引き当て(context lookup)が実際に AWS を呼びうる。ADR-001 は遮断を「1か所で守れる」としているが、その1か所で何をするかが足りない。 | 遮断の方法を部品の振る舞いと ADR に書く。環境変数を外すことに加え、共有の認証情報と設定のファイルの場所を空の一時ファイルに向ける、メタデータの取得を止める、など、設定の読み込み経路も止める。AWS を呼ばないことをテストでどう確かめるか(差し替えた synth の環境の検査だけで足りるか)も決める。同じ環境の整理を CdkSourceAnalyzer の振る舞いにも明記する(ADR-003 は述べているが部品に無い)。 | New |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/domain-design/decisions.md > ADR-005 Decision と components.md > component CdkSourceAnalyzer | ADR-005 は「書き換えたあとの読み直しには補助のプログラムをもう一度使う」とする。今の internal/iac は、内容をメモリの上で編集し、メモリの上で読み直して確かめてから、ファイルに置き換える(`run.go` の `editFile` → `checkEdits` → `write`)。補助のプログラムはディスクのファイルを読むので、そのまま使うと、確かめる前にファイルを書くか、プロジェクト全体の一時的な写しを作るかになる。前者は FR3.7(確かめて意図した形でなければ書き換えない)と NFR4 を崩し、後者は TypeScript のコンパイラーが import をたどる前提と合わない。さらに今の仕組みは、ファイルごとに並行して動き、メモリの予算で絞る。ファイルごとに Node.js や Python を起動する場合の数と時間の上限は、どの部品にも決まっていない。ADR-005 の Consequences は「時間がかかる」としか書いていない。 | 補助のプログラムが、書き換え後の内容を標準入力などで受け取って読み直せる(ディスクに書く前に確かめられる)ことを部品の契約の前提として書く。並行して起動できる数と、呼び出しの回数の上限(ファイルごとか、アプリごとにまとめるか)を決める。決められない場合は、ADR-005 の Consequences に、確かめの順番の制約として明記し、契約の設計の段階の課題として引き継ぐ。 | New |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/domain-design/components.md > entity SynthesizedResource と entity TagSite | スコープに付けたタグは「変更案がそのスコープでタグを受けるすべてのリソースを同じように変えるときだけ」書き換える(FR3.5)。その判断には、スコープの下の各リソースの今のタグの値と、そのリソースの種類がタグを受けるかどうかが要るが、SynthesizedResource の項目は stackName、logicalId、resourceType、constructPath、app だけである。また、スタックの tags とリソース単位の指定が同じキーを持つときの優先順位も、どこでも触れていない。CdkSourceAnalyzer は depends_on が空なのに、TagSite が SynthesizedResource を参照し、効く範囲(scopeConstructPath)を求めるための入力(synth のパスの一覧を渡すのか、ソースだけから決めるのか)が書かれていない。 | SynthesizedResource に synth のテンプレートのタグの項目を足すか、足さない理由を書く。タグを受けるかどうかの判断元と、優先順位の扱いを決める。IacWriter が synth の結果を CdkSourceAnalyzer に渡すのか、IacWriter が両者を突き合わせるのかを、interaction の文に書く。 | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/requirements-analysis/requirements.md > 8. Open questions と decisions.md | 要件の Open questions のうち「書き換えのあとに cdk synth をもう一度動かして確かめるか」「差分を見る実行と `--pr` の実行で synth が2回動く負担をどうするか」は、どちらも「設計で決める」とされているが、decisions.md にも components.md にも決定も先送りも書かれていない。ADR-004 の作成の記録のための2回目の synth も加わり、1回の実行で synth が最大3回動きうる。あわせて、変更案に stack-name のタグのリソースが1つも無い(Terraform だけの変更案など)のに、リポジトリに cdk.json があるだけで synth を動かすのか、も書かれていない。動かすなら、CDK を使わない変更案の利用者が、NFR5 の「今と変えない」に反して、synth の失敗や遅さの影響を受ける。 | 2つの Open questions について、決定するか、機能の設計に引き継ぐと明記する。synth を動かす条件(変更案に CDK のリソースの候補があるときだけ、など)を IacWriter の振る舞いに足す。 | New |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/domain-design/components.md > component IacDisplay と component CdkSynthesizer, PullRequestPublisher | FR2.7 は synth の間のアプリごとの状態の更新、FR1.9 は段階ごとの行を求める。しかし IacDisplay の depends_on は結果の型(IacResult、PublishOutcome)を読む線だけで、実行中の進み具合を CdkSynthesizer や PullRequestPublisher から表示へ渡す線(コールバック、イベントなど)がカタログにも図にもない。結果の型だけでは、実行中の表示は作れない。 | 進み具合を渡す仕組み(たとえば IacCommand が進み具合の受け手を渡す)を、interaction と図に足す。どの部品が受け手の型を持つかも決める。 | New |
| R-07 | Minor | aidlc/spaces/default/codekb/tag-refactor/architecture.md > 気になる点(外部プロセスの呼び出しの共通化) と components.md > 理由 | コードの記録は、外のプログラムの呼び出し(環境変数の整理、時間の上限、差し替えの口)を共通の実行の部品にまとめる余地を指摘している。今回の設計では、cdk、Node.js、Python、git、gh の呼び出しが、CdkSynthesizer、CdkSourceAnalyzer、PullRequestPublisher、IacWriter(既存の `iac/git.go`)の4か所に分かれ、NFR1〜NFR3 とチームの「外のプログラムの呼び出しは差し替える」が、それぞれで実装される。共通化するかしないかの判断が ADR にない。 | 共通の実行の部品を置くか、置かない理由(パッケージごとに違う環境の整理が要る、など)を、ADR として1つ足す。置く場合は、部品と依存を components.md に足す。 | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| 手作業の確かめ: 名前の一意、宣言されていない参照、自己依存 | PASS | 6つの部品の名前は一意で、`component:` と `owned_by` の参照はすべて宣言済み。自己依存はない |
| 手作業の確かめ: depends_on と dependents の対称 | PASS | IacCommand、IacWriter、CdkSynthesizer、CdkSourceAnalyzer、PullRequestPublisher、IacDisplay のすべての組が対称 |
| 手作業の確かめ: 依存の循環 | PASS | IacCommand → IacWriter、PullRequestPublisher、IacDisplay、IacWriter → CdkSynthesizer、CdkSourceAnalyzer、IacDisplay → IacWriter、PullRequestPublisher で、循環はない |
| 手作業の確かめ: 持つものの一意と識別子、references の宣言 | PASS | 7つのものは、それぞれ1つの部品だけが持ち、識別子がある。references の相手(CdkApp、TagSite、SynthesizedResource)は、`owned_by` の部品の下に宣言されている |
| 手作業の確かめ: インフラが外部依存になっていること | PASS | git、gh、cdk、Node.js、Python、GitHub、ファイルシステムはすべて external_dependencies で、部品ではない |
| 手作業の確かめ: 追跡 | PASS | stories.md の US1.1、US2.1、US3.1、US3.2、US4.1 の5件が traceability.json に漏れなく載り、すべて部品に対応している |
| 手作業の確かめ: ADR の構造 | PASS | ADR-001〜ADR-005 は Context、Decision、Consequences、Alternatives Rejected を持ち、2案以上を比べ、Security and compliance も書いている |

### Summary

部品の分け方、依存の向き、持つものの割り当て、ADR の形は整っていて、構造の確かめにも通る。ただし、synth に失敗したアプリのリソースの帰属、認証情報の遮断の範囲、メモリ上の確かめを外の補助のプログラムに置き換えたときの順番の3点は、そのままでは実装で迷うため、承認の前に判断してほしい。
