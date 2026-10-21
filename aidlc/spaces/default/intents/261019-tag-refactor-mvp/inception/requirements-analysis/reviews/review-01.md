## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-product-lead-agent
**Date:** 2026-10-19T12:12:11Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/requirements-analysis/requirements.md > FR5.8、FR6.1、FR6.3、FR3.6 | FR5.8 は「書き換える前」の状態しか記録しない。しかし FR6.3 は「apply の後に別の変更が入ったか」を判定する必要があり、そのためには apply が書き込んだ後の状態(または書き込んだ内容)も記録が要る。実行IDの記録の探し方(手元ファイルか S3 か)も FR6.1 に書かれていない。apply 時に、保存した変更案の対象アカウントと実行中のアカウントが違う場合の扱いもない。FR3.6 は変更案にアカウントを含めるだけで、確認の要件がない。 | FR5.8 に、書き換え後の状態(または適用した変更)も記録すると明記する。rollback が、手元と S3 のどちらの記録でも実行IDから見つけられることと、記録がないときの動作を決める。アカウント不一致のときは apply と rollback が止まることを、合否の基準つきで追加する。 | New |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/requirements-analysis/requirements.md > FR1.6、FR2.1、AS1 | 表記ゆれを「同じ意味」でまとめる基準が定義されていない。合否の基準は例(`env` / `Env` / `environment`)1つだけで、`env` と `environment` のような略語を何で同一視するのか(大文字小文字の無視、略語辞書、類似度など)が分からない。AS1 は「まとめるかは利用者がルールファイルで決める」としており、FR1.6 の自動グルーピングと食い違っている。開発者は実装を推測するしかなく、QA も例以外のテストを書けない。 | グルーピングの方法を具体的に決める(例: 大文字小文字の違いと区切り文字の違いだけをまとめる、略語は対象外、など)。FR1.6 と FR2.1 で同じ基準を使うと明記し、AS1 との関係を整理する。合否の基準に、まとめない例も加える。 | New |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/requirements-analysis/requirements.md > FR7.4、FR1.3、FR1.5、FR4.7、FR5.7、FR6.4 | 終了コードの定義(0 = 問題なし、1 = エラー、2 = 問題や影響が見つかった)が、実際に起こる状況をカバーしていない。plan で変更案が出たとき、scan や影響分析が不完全(FR1.5、FR4.7)なとき、apply や rollback で一部が失敗・スキップされたとき(FR5.6、FR5.7、FR6.3)の終了コードが決まっていない。FR1.3 は「1にならない」としか言っていない。スクリプトから使う前提(R-Q12)なので、ここが曖昧だと使えない。 | コマンドごと・状況ごとの終了コードの表を作る(特に、不完全な結果、一部失敗、スキップのあった成功)。各行に合否の基準を付ける。 | New |
| R-04 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/requirements-analysis/requirements.md > FR3.7、AS7、FR8の位置付け | FR3.7(plan、第1弾)は「IaC で管理されていると分かるリソース」を警告するとしているが、AS7 は「第2弾では CloudFormation だけ」としている。plan は第1弾なので、段階の記述が食い違っている。また、R-Q10 は CloudFormation と Terraform の両方を挙げて警告を選んだのに、Terraform の警告を落とすことは回答を確認する場でも確認されていない。CDK で作ったリソースは CloudFormation 経由なので、どこまで判定できるかも不明。 | FR3.7 の対象を、段階ごとに明記する(第1弾の plan で何を判定するか)。Terraform を警告の対象から外すなら、その決定を利用者に確認してから、範囲外または未決事項に入れる。CDK 由来のリソースの扱いも書く。 | New |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/requirements-analysis/requirements.md > FR5.9、FR5.10、NFR7 | これらの要件は、質問と回答にも前のステージの成果物にも根拠がない。出典欄は「R-Q5〜R-Q10」「コーナーの洗い出し」のような広い書き方で、どの根拠から来たのかを辿れない。段階のルールは、新しい要件の出所を記録することを求めている。 | 各要件に、出所(新規にこのステージで追加したのか、どの前提や制約から来たのか)を具体的に書く。人の確認が要る追加なら、レビューの場でそれと分かるようにする。 | New |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/requirements-analysis/requirements.md > FR2.3、FR2.4、FR3.4 | 条件ごとの値が複数当てはまるとき(リソースの種類とリージョンの両方の条件に合う場合など)の優先順位が決まっていない。また、消す候補(FR3.4)は、ルールファイルにないキーすべてが対象なので、`aws:` で始まるキーや正当なキーが大量に並ぶ可能性がある。 | 複数の条件が当てはまったときの優先順位と、競合したときの動作を決める。消す候補から `aws:` のキーを除くか、一覧の出し方(件数だけにするなど)を決める。 | New |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/requirements-analysis/requirements.md > FR4.1、FR4.2、CON3 | 影響分析の対象外が、SCP、RCP、Backup などの列挙だけで、列挙にないポリシーの種類(アクセス許可の境界、ロールの信頼ポリシー、Lambda や ECR など上記5種類以外のリソースポリシー)が対象かどうか分からない。信頼ポリシーはタグ条件を使うことが多い。 | 対象とする種類と、対象外とする種類(境界、信頼ポリシー、上記以外のリソースポリシー)を明記する。対象外のものがあることを plan の出力で利用者に伝えるかも決める。 | New |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/requirements-analysis/requirements.md > FR1.1、FR1.2、FR5.8、NFR6 | グローバルサービスのリソース(IAM、S3、CloudFront など)が複数リージョンで重複して数えられたり漏れたりする場合の扱いが書かれていない。また、書き換え前の記録を保存できなかった(ディスクがいっぱい、S3 に書けないなど)とき、apply を中止することが明記されていない(NFR6 は壊れ方だけを扱う)。 | グローバルなリソースをどのリージョンで一度だけ数えるかを決める。記録の保存に失敗したら、何も書き換えずに止まることを、合否の基準つきで追加する。 | New |

### Summary

質問への回答と要件の対応はおおむね取れており、機能ごとに合否の基準もありますが、rollback の前提となる記録の内容、表記ゆれの判定基準、終了コードの網羅、IaC 判定の段階が曖昧または食い違っており、そのまま実装すると手戻りが出ます。この4点(Major)を直してから承認することをお勧めします。
