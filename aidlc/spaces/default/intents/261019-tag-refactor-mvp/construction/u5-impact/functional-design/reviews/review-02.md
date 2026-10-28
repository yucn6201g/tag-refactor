## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T16:55:15Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR4.4 (必須タグの追加)、BR4.5 (TagKeys)、BR4.7 | 必須タグの追加(addRequired)は、`Null` と「付ける値が条件の値に当てはまる」場合だけを影響ありとしている。しかし AWS の評価では、`IfExists` 付きの演算子(例 `StringEqualsIfExists: aws:ResourceTag/env: dev`)は、キーが無いと true、キーが付くと実際の値で評価される。付ける値が `dev` 以外(例 `prod`)なら、タグを付けるだけで結果が true から false に変わるのに、BR4.4 は値が当てはまらないため「影響なし」を返す。`aws:TagKeys` でも、許可リスト型の条件(`ForAnyValue:StringNotEquals aws:TagKeys: [env, owner]` で Deny)では、リストにない新しいキーを必須タグとして付けることが拒否に変わるのに、BR4.5 は toKey が値に当てはまらないため見逃す。Q5 の動機(「タグが無かったリソースに付くと条件の結果が変わる」)と食い違い、NFR2 の「見逃さない」趣旨を満たさない。 | addRequired の判定に、演算子の修飾子(`IfExists`)と TagKeys の否定・`ForAllValues` を考慮する規則を足す。例: `IfExists` 付きの条件は、キーが当てはまれば値に関係なく affected(または undecided)。TagKeys の否定の演算子は、toKey が値に当てはまらないことも影響ありにする。決定表(functional-spec.md の判定の表)も合わせて直す。 | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR2.2、BR4.7 と inception/user-stories/stories.md > AC3.2.4、requirements.md > FR4.5 | FR4.5 の受け入れ基準は「ワイルドカードを含む条件が `?` として表示され、影響なしとは表示されない」、AC3.2.4 も同じ趣旨である。設計は、タグのキーの `*` `?` も条件の値の `StringLike` のパターンも「パターンとして当てはめる」ので、当てはまれば `!`、当てはまらなければ影響なしになり、ワイルドカードを含む条件が `?` になる場面が無い(`?` になるのは `${` とその他の演算子だけ)。Q&A で U1 の規則 5.4 を引き継ぐと決めているが、上流の受け入れ基準との食い違いが、仕様のどこにも明記されていない。このままでは AC3.2.4 のテストを書けない。 | 次のどちらかを行う。(a) ワイルドカードを含む条件は、判定の結果に関係なく `?` を返す(または当てはまらない場合だけ `?` にする)規則にする。(b) 上流の FR4.5・AC3.2.4 の文言との差分を functional-spec.md に明記し、承認を受けられる形にする(AC をどう満たすか、どのテストで確かめるかを書く)。 | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR4.3、BR4.2 と functional-spec.md > 判定の表 | 規則と決定表が、読み方によって食い違う。(1) BR4.3 の logic は「当てはまる値があれば affected、…、演算子が Null なら影響なし」の順に書かれており、文字どおりに読むと `Null` の値(`"true"`/`"false"`)がタグの値 `true` などに当たって affected になる。決定表は「Null は影響なし」で、Null の確認を先に置く BR4.4 とも順序が逆である。(2) BR4.2 の logic は大文字・小文字だけの名前の変更の例外を `location = conditionKey` に限っているのに、末尾の「条件の値や Resource の中の変数の参照(BR2.4)も同じ」は、変数の参照も同じ例外を受けるとも読める。決定表は変数の行を「affected」だけにしている。 | BR4.3 の logic で Null の確認を比較より前に置く。BR4.2 の末尾の文を、変数の参照は例外を受けない(常に affected)と言い切る形に直す。規則と決定表の全セルを突き合わせて一致を確かめる。 | Resolved (reviewer) |
| R-04 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR3.2 と BR2.4 | 条件キーのタグのキーに `${aws:PrincipalTag/k}` が入る場合(`aws:ResourceTag/${aws:PrincipalTag/k}`)、リソースポリシーでは BR3.2 によりそのリソース自身の変更にしか結び付かない。しかしキーの決まり方は IAM のロール・ユーザーのタグ `k` に依存するため、ロールの `k` の変更は結び付かず見逃す(BR2.4 は条件の値と Resource の中の変数しか探さない)。 | 条件キーの中の変数も BR2.4 の対象にして、PrincipalTag なら BR3.3 の結び付けを加える。または、見逃しを許す理由を規則に書く。 | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR5.3 と functional-spec.md > 流れ W2(S3) | S3 のバケットのリージョンが一覧の答えに入っていない場合に「別に読む」としているが、その API(`s3:GetBucketLocation` など)が BR5.3 の足す操作の一覧に無い。一覧は「読み取り用の例が実際の呼び出しをすべて含むことを確かめるテスト」の正本になるため、後で一覧が漏れる原因になる。 | 一覧の答えにリージョンが入る前提を設計の確定事項にするか、フォールバックの API を BR5.3 に足す。 | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR1.1、BR1.4 | 他のポリシーの権限の上限(permissions boundary)として使われる管理ポリシーは、条件にタグを使い得るが、BR1.1 の「どこかに付いている」に含まれるのか、BR1.4 の読まないものなのかが書かれていない(`ListPolicies` の `OnlyAttached` は権限ポリシーとしての付与が対象で、境界専用のポリシーが返るかは確認が必要)。 | 境界として使われる管理ポリシーを読むか読まないかを決めて BR1.1 か BR1.4 と README の記載に書く。読む場合は取得の方法を規則にする。 | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR5.1、entities.md > ImpactFinding と contract-summary.md > C2 impactFindings | ImpactFinding は `conditionOperator` を持ち、指摘の単位にも使うが、C2 の `impactFindings` のスキーマと C9 の `ImpactResult` の項目に `conditionOperator` が無い。同じステートメントの同じ条件キーで演算子だけが違う2つの指摘(`StringEquals` と `StringLike`)は、保存した変更案と JSON の出力では区別できない。また BR5.1 は Resource が配列で複数の値に変数がある場合の `conditionValue` を決めていない。 | C2 に `conditionOperator` を(省略できる項目として)足す。Resource の複数の値の `conditionValue` のつなぎ方を BR5.1 に決める。 | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR5.2 | 順の規則の「条件の順(演算子、条件キーの文字列の順)、Resource、NotResource の順」は、Resource の指摘(演算子が空)を条件より前に置くのか後に置くのかが読み取れない。実装者によって順が変わり、並び順を毎回同じにする目的(決定性、ゴールデンテスト)が実装ごとに変わる。 | 並べる鍵を、(ステートメントの順、種類の番号(条件、Resource、NotResource)、演算子、条件キー、verdict)のように明示する。 | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/functional-spec.md > 検出率100%のテスト用の参照 | NFR2 のテスト用の参照の次元は、ポリシーの種類・参照・変更の3つで、演算子の種類(否定、`IfExists`、`ForAnyValue`/`ForAllValues`、`IgnoreCase`、`StringLike`、`Null`、その他)が入っていない。BR2.3 と BR4.7 の分岐の大半は、この次元でしか通らず、上の Major の見逃しも検出できない。 | テスト用の参照に演算子の次元を足し、期待する判定の表に入れる。 | Resolved (reviewer) |
| R-10 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/contract-design/contract-summary.md > C9 の rules と ImpactResult.fields、および construction/u5-impact/functional-design/functional-spec.md > 契約への変更 | 契約 C9 の規則に「ワイルドカードやポリシー変数を含む条件は undecided」とあるが、U5 の設計(BR2.2)は、ワイルドカードは機械的に当てはめ、当てはまれば affected、当てはまらなければ影響なしとする。Assumptions は FR4.5 と AC3.2.4 との差だけを挙げ、C9 のこの規則には触れていない。C9 の ImpactResult.fields にも、U5 で足した conditionOperator と、UnreadPolicy の項目が無い。実装する人が C9 と BR2.2 のどちらに従うか迷う。 | 「契約への変更」の表に C9 の行を足し、C9 の規則の文言を BR2.2 に合わせて直す(ワイルドカードは機械的に判定し、`?` にするのは変数と文字列以外の演算子)。あわせて C9 の ImpactResult.fields に conditionOperator と unreadPolicies の項目を書き足す。承認の場で確かめる Assumptions の項目にも C9 を含める。 | New |
| R-11 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR4.5(3)、BR4.7、BR2.3 | TagKeys の参照で演算子が Null のとき、BR4.5(3) は BR4.7 で値を比べるとするが、BR4.7 には Null の比べ方が無い(BR2.3 は Null を「その他」とは別の種類にしている)。BR4.5(2) の「否定の演算子」も、Arn・Numeric などの否定形(ArnNotLike など)をどう扱うか書かれていない。どちらも、見逃さない側に倒す方針は読み取れるが、実装によって判定が割れうる。 | BR4.7 に、TagKeys の値と Null を組み合わせたときの扱い(例: undecided)を 1 行足す。BR4.5(2) の否定の演算子を、BR2.3 の種類ごとに列挙する(その他の否定形は undecided)。テスト用の参照の演算子の表にも入れる。 | New |

> R-01 Reviewer note: BR4.4 に IfExists と ForAnyValue:・ForAllValues: を affected とする手順(3)が入った。修飾子のない演算子は、タグが無いとき非否定が false、否定が true になり、付けた値が条件の値に当てはまらなければ結果は同じ、という根拠も正しい。BR4.5(2) は TagKeys の否定・ForAllValues を renameKey と addRequired で affected にしている。決定表の addRequired と TagKeys の行は、この 2 つの規則と一致する。

> R-02 Reviewer note: ワイルドカードを `?` にしない扱いを functional-spec.md の Assumptions に書き、承認の場で確かめる形にした。AC3.2.4 をどのテストで確かめるか(変数は `?`、当てはまるワイルドカードは `!`)も書いてある。ただし契約 C9 の規則との食い違いが残る。下の新しい指摘 1 を参照。

> R-03 Reviewer note: BR4.3 は、変数の場所なら undecided、次に Null なら影響なし、最後に値を比べる、の順になった。BR4.2 の末尾は、変数の参照は大文字・小文字の例外を受けないと言い切っている。BR4.1〜BR4.6 と決定表の全セルを突き合わせた。BR4.2 の大文字・小文字の例外は、決定表では条件キーの行にだけ付いていて、一致する。

> R-04 Reviewer note: BR2.4 に keyVariable の場所が入り、条件キーの中の変数は条件キーそのものの参照とは別の参照として BR3 で結び付ける、と書いた。PrincipalTag は BR3.3 でユーザーとロールの変更に結び付く。同じ(演算子、条件キー)で affected と undecided が重なる場合は、BR5.1 で affected にまとまる。業務の場面と entities.md の TagReference.location も一致する。

> R-05 Reviewer note: BR5.3 に s3:GetBucketLocation(一覧の答えにリージョンが無いバケットだけ)が入った。BR1.3 と W2 の記述とも一致する。

> R-06 Reviewer note: BR1.1 が、権限の上限として使われるポリシーも読むと決めた。ListPolicies の結果にある付いている数と権限の上限の数のどちらかが 1 以上のものを選び、「付いているものだけに絞る指定は使わない」とも書いた。取得の方法が規則になっている。

> R-07 Reviewer note: 契約 C2 に conditionOperator(省略できる項目)を足した。Resource・NotResource の conditionValue は、文書の順に「, 」でつなぐと BR5.1 と C2 の両方に書いた。entities.md の ImpactFinding も同じ。

> R-08 Reviewer note: BR5.2 が、sourceKind、policyRef、ステートメント順、場所の種類(条件、Resource、NotResource)、conditionOperator、conditionKey、verdict の 7 つの鍵を明示した。unreadPolicies の鍵も書いてある。

> R-09 Reviewer note: テスト用の参照に演算子の次元(8)を足し、演算子ごとに 4 つの変更で期待する判定を表にする、と書いた。

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| aidlc engine sensor-traceability (functional-design) | PASS: gaps、orphans、invalid_entries が空、findings_count 0 | traceability.json は上流の ID と対応づいていて、構造上の問題は無い。 |

### Summary

前回の Major 3 件(R-01 から R-03)と Minor 6 件はすべて直っている。BR4.3 から BR4.5 の判定の順は、決定表の全セルと一致していて、IfExists や TagKeys の否定による見逃しも塞がっている。新しい指摘は、C9 の古い規則と文言の整合、および TagKeys と Null の比べ方の 2 件で、どちらも Minor であり、READY を妨げない。
