## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T16:51:04Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR4.4 (必須タグの追加)、BR4.5 (TagKeys)、BR4.7 | 必須タグの追加(addRequired)は、`Null` と「付ける値が条件の値に当てはまる」場合だけを影響ありとしている。しかし AWS の評価では、`IfExists` 付きの演算子(例 `StringEqualsIfExists: aws:ResourceTag/env: dev`)は、キーが無いと true、キーが付くと実際の値で評価される。付ける値が `dev` 以外(例 `prod`)なら、タグを付けるだけで結果が true から false に変わるのに、BR4.4 は値が当てはまらないため「影響なし」を返す。`aws:TagKeys` でも、許可リスト型の条件(`ForAnyValue:StringNotEquals aws:TagKeys: [env, owner]` で Deny)では、リストにない新しいキーを必須タグとして付けることが拒否に変わるのに、BR4.5 は toKey が値に当てはまらないため見逃す。Q5 の動機(「タグが無かったリソースに付くと条件の結果が変わる」)と食い違い、NFR2 の「見逃さない」趣旨を満たさない。 | addRequired の判定に、演算子の修飾子(`IfExists`)と TagKeys の否定・`ForAllValues` を考慮する規則を足す。例: `IfExists` 付きの条件は、キーが当てはまれば値に関係なく affected(または undecided)。TagKeys の否定の演算子は、toKey が値に当てはまらないことも影響ありにする。決定表(functional-spec.md の判定の表)も合わせて直す。 | New |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR2.2、BR4.7 と inception/user-stories/stories.md > AC3.2.4、requirements.md > FR4.5 | FR4.5 の受け入れ基準は「ワイルドカードを含む条件が `?` として表示され、影響なしとは表示されない」、AC3.2.4 も同じ趣旨である。設計は、タグのキーの `*` `?` も条件の値の `StringLike` のパターンも「パターンとして当てはめる」ので、当てはまれば `!`、当てはまらなければ影響なしになり、ワイルドカードを含む条件が `?` になる場面が無い(`?` になるのは `${` とその他の演算子だけ)。Q&A で U1 の規則 5.4 を引き継ぐと決めているが、上流の受け入れ基準との食い違いが、仕様のどこにも明記されていない。このままでは AC3.2.4 のテストを書けない。 | 次のどちらかを行う。(a) ワイルドカードを含む条件は、判定の結果に関係なく `?` を返す(または当てはまらない場合だけ `?` にする)規則にする。(b) 上流の FR4.5・AC3.2.4 の文言との差分を functional-spec.md に明記し、承認を受けられる形にする(AC をどう満たすか、どのテストで確かめるかを書く)。 | New |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR4.3、BR4.2 と functional-spec.md > 判定の表 | 規則と決定表が、読み方によって食い違う。(1) BR4.3 の logic は「当てはまる値があれば affected、…、演算子が Null なら影響なし」の順に書かれており、文字どおりに読むと `Null` の値(`"true"`/`"false"`)がタグの値 `true` などに当たって affected になる。決定表は「Null は影響なし」で、Null の確認を先に置く BR4.4 とも順序が逆である。(2) BR4.2 の logic は大文字・小文字だけの名前の変更の例外を `location = conditionKey` に限っているのに、末尾の「条件の値や Resource の中の変数の参照(BR2.4)も同じ」は、変数の参照も同じ例外を受けるとも読める。決定表は変数の行を「affected」だけにしている。 | BR4.3 の logic で Null の確認を比較より前に置く。BR4.2 の末尾の文を、変数の参照は例外を受けない(常に affected)と言い切る形に直す。規則と決定表の全セルを突き合わせて一致を確かめる。 | New |
| R-04 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR3.2 と BR2.4 | 条件キーのタグのキーに `${aws:PrincipalTag/k}` が入る場合(`aws:ResourceTag/${aws:PrincipalTag/k}`)、リソースポリシーでは BR3.2 によりそのリソース自身の変更にしか結び付かない。しかしキーの決まり方は IAM のロール・ユーザーのタグ `k` に依存するため、ロールの `k` の変更は結び付かず見逃す(BR2.4 は条件の値と Resource の中の変数しか探さない)。 | 条件キーの中の変数も BR2.4 の対象にして、PrincipalTag なら BR3.3 の結び付けを加える。または、見逃しを許す理由を規則に書く。 | New |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR5.3 と functional-spec.md > 流れ W2(S3) | S3 のバケットのリージョンが一覧の答えに入っていない場合に「別に読む」としているが、その API(`s3:GetBucketLocation` など)が BR5.3 の足す操作の一覧に無い。一覧は「読み取り用の例が実際の呼び出しをすべて含むことを確かめるテスト」の正本になるため、後で一覧が漏れる原因になる。 | 一覧の答えにリージョンが入る前提を設計の確定事項にするか、フォールバックの API を BR5.3 に足す。 | New |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR1.1、BR1.4 | 他のポリシーの権限の上限(permissions boundary)として使われる管理ポリシーは、条件にタグを使い得るが、BR1.1 の「どこかに付いている」に含まれるのか、BR1.4 の読まないものなのかが書かれていない(`ListPolicies` の `OnlyAttached` は権限ポリシーとしての付与が対象で、境界専用のポリシーが返るかは確認が必要)。 | 境界として使われる管理ポリシーを読むか読まないかを決めて BR1.1 か BR1.4 と README の記載に書く。読む場合は取得の方法を規則にする。 | New |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR5.1、entities.md > ImpactFinding と contract-summary.md > C2 impactFindings | ImpactFinding は `conditionOperator` を持ち、指摘の単位にも使うが、C2 の `impactFindings` のスキーマと C9 の `ImpactResult` の項目に `conditionOperator` が無い。同じステートメントの同じ条件キーで演算子だけが違う2つの指摘(`StringEquals` と `StringLike`)は、保存した変更案と JSON の出力では区別できない。また BR5.1 は Resource が配列で複数の値に変数がある場合の `conditionValue` を決めていない。 | C2 に `conditionOperator` を(省略できる項目として)足す。Resource の複数の値の `conditionValue` のつなぎ方を BR5.1 に決める。 | New |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/rules.md > BR5.2 | 順の規則の「条件の順(演算子、条件キーの文字列の順)、Resource、NotResource の順」は、Resource の指摘(演算子が空)を条件より前に置くのか後に置くのかが読み取れない。実装者によって順が変わり、並び順を毎回同じにする目的(決定性、ゴールデンテスト)が実装ごとに変わる。 | 並べる鍵を、(ステートメントの順、種類の番号(条件、Resource、NotResource)、演算子、条件キー、verdict)のように明示する。 | New |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/functional-design/functional-spec.md > 検出率100%のテスト用の参照 | NFR2 のテスト用の参照の次元は、ポリシーの種類・参照・変更の3つで、演算子の種類(否定、`IfExists`、`ForAnyValue`/`ForAllValues`、`IgnoreCase`、`StringLike`、`Null`、その他)が入っていない。BR2.3 と BR4.7 の分岐の大半は、この次元でしか通らず、上の Major の見逃しも検出できない。 | テスト用の参照に演算子の次元を足し、期待する判定の表に入れる。 | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `aidlc engine sensor-traceability`(traceability.json、functional-design) | PASS(gaps 0、orphans 0、invalid 0) | 要件・ストーリー・規則の対応づけの形は問題ない。判定の意味の正しさは確かめていない。 |

### 確かめたこと

- 契約の互換性: C2 の `impactFindings[].sourceKind` は7つの値を既に持つため、列挙の値は増えない。`unreadPolicies[]` への `policyRef` と `region`(省略できる項目)の追加は `version` 1 のままでよい。C2 の `unreadPolicies[].sourceKind` は文字列で問題ない。
- 配線: `internal/cli/commands.go` では `Plan.Regions` に当たる `snap` が `impact.New` の前に決まるため、リージョンを `New` で受け取る設計は成り立つ。
- BR5.3 の操作の一覧は、流れ W2 の5種類のリソースポリシーの呼び出しとIAMのインラインポリシーの呼び出しに対して、S3 のリージョンのフォールバック以外は漏れが無い。
- 結び付け BR3.1〜BR3.3 は、上の Minor の条件キーの中の変数を除いて、見逃しを作る組み合わせを見つけられなかった。
- Q&A の Q5 とワイルドカードの扱いは、人が確認した内容に沿って書かれている。上の Major は、その決定の範囲を超える上流との食い違い、または決定の前提(Q5 の動機)と食い違う部分に限った。

### Summary

Major が3件(`IfExists` と TagKeys の否定を考えない addRequired の見逃し、ワイルドカードを `?` にしないことと AC3.2.4・FR4.5 の食い違い、規則と決定表の食い違い)あり、NOT-READY です。契約の版の扱いと配線は問題ありません。
