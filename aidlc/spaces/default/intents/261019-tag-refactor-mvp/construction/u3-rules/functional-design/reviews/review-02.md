## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T00:03:48Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | high | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/rules.md > BR3.4 (および functional-spec.md の「誤りの例」) | BR3.4 は `match.resourceType` の形を「小文字・数字・ハイフンのサービス名」+「:」+「1文字以上の種類」と定めている。しかし既存コード `internal/inventory/inventory.go` の `ResourceTypeOf` は、種類の部分がない ARN に `s3`(`arn:aws:s3:::bucket`)や `sqs`(`arn:aws:sqs:...:queue`)のようにコロンのない値を返し、解釈できない ARN には `unknown` を返す。つまり scan が表示し、BR3.2 が「完全一致」で比べる値そのものを、BR3.4 は誤りとして拒否する。S3 バケット、SQS キュー、SNS トピックなど、よくタグを付ける種類に対して `when: match: {resourceType: ...}` を書けなくなる。Q6 の「scan の出力と同じ形なので、そのまま写せる」とも食い違う | BR3.4 の形の定義を、`ResourceTypeOf` が実際に返す値(`service` だけの形と `service:type` の形)に合わせる。「`unknown` は書けない」などの扱いも決める。誤りの例と、`ec2:instance` だけを挙げている説明にも `s3`、`sqs` を足し、これらを読み込めて BR3.2 で一致することを受け入れ条件の場面に加える | Resolved (reviewer) |
| R-02 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/rules.md > BR3.2 logic(match.tags)と functional-spec.md「照会の約束」 | 必須タグの値の照会は「リソース(種類、リージョン、タグ)」を入力とするが、`match.tags` を比べる相手のタグが、収集した元のタグか、`keys` の別名の置き換えと値の別名の置き換えを済ませたあとのタグかが決まっていない。たとえば `match.tags: {env: prod}` に対して、リソースが `Env: Prod` を持つとき、同じルールファイルでも U4 の組み込み方で当てはまるかどうかが変わる。BR3.2 は「同じ入力には常に同じ答え」と言うが、入力の意味が決まっていないため U4 が推測で実装することになる | `match.tags` のキーと値を、どのタグの状態に対して比べるかを決めて BR3.2 に書く(例: 元のタグ、または別名と値を置き換えたあとのタグ)。`match.tags` のキーを別名で書いたときの扱いも決める。決めた内容を例つきで業務の場面に加える | Resolved (reviewer) |
| R-03 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/rules.md > BR6.2, BR6.3, BR6.4 と、functional-spec.md > 下書きの例 | 下書きは見つかったすべてのキーと値の候補を二重引用符つきで出し、制御文字などは `uXXXX` に展開する。一方、読み込み側は 64 KiB を超えるファイルを拒否する(`internal/rules/rules.go` の `MaxFileSize`、BR1 系で「そのまま使う」と明記)。キーの種類が多いアカウントや、値の候補を最大20個ずつ出すキーが多いアカウントでは、下書きが 64 KiB を超えうる。その場合、`rules suggest` の出力をそのまま `plan --rules` に渡せず、AC2.1.1(そのまま読み込める)が満たせない。上限を超えたときの規則がどこにもない | 下書きが読み込みの上限に収まることを保証する規則を BR6 系に足す(例: 上限に近づいたら値の候補を出さない、またはキー数の上限と理由のコメントを入れる)。または、上限を超える場合の振る舞い(標準エラーの警告と終了コード)を決める。見積もりの根拠と、上限を超える場合のテストを場面に加える | Resolved (reviewer) |
| R-04 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/functional-spec.md > 契約への変更 > C8 と entities.md > RuleSet | 契約 C8 の `RuleSet` の項目名は `requiredTagRules` だが、エンティティと既存コードは `requiredTags` を使っている。契約への変更の一覧は `keyRules.values` だけを挙げており、この名前の違いを直すと書いていない。U4 が契約を見て実装すると、名前が合わない | C8 の変更として `requiredTagRules` を `requiredTags` に直す(または逆に合わせる)ことを、契約への変更の一覧に明記する。あわせて `removeKeys` が `RemoveKey` の一覧になったことも書く | Resolved (reviewer) |
| R-05 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/rules.md > BR3.4 と BR3.2(region の一致) | (1) U2 は、対応表にない AWS Config の種類を `AWS::...` の形のまま持つ。BR3.4 はそれを誤りにし、直し方を「scan の出力の種類の書き方」と案内するが、その書き方も `AWS::...` なので、書ける形が存在しない。(2) グローバルなリソースのリージョンは、読んだリージョンのうち名前順で最初のものになるため、`--region` で絞ると変わる。`match.region` の結果が、ルールファイルではなく実行時の絞り込みで変わる | (1) 対応表にない種類を条件で扱えないことを制限として明記し、案内の文言を矛盾しないものにする。(2) グローバルなリソースが `match.region` に当てはまるかどうかの扱い(当てはまらない、または常に当てはまるなど)を決めて書く | Resolved (reviewer) |
| R-06 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/functional-spec.md > 流れ W2 の手順7 と下書きの例 | BR6.6 は、読めなかったリージョンがあるとき下書きの先頭のコメントにも「結果は不完全」とリージョンを書くと定めているが、W2 の手順7と下書きの例には標準エラーの出力しか書かれていない。また、タグの付いたリソースが1つもないときの下書き(`keys` が空)の形が決まっていない | W2 の手順7と例に先頭コメントの不完全の表示を足す。リソースが0件のとき、読み込める形(例 `keys: {}`)で出すことを BR6 系に書く | Resolved (reviewer) |

> R-01 Reviewer note: BR3.4 は `service` と `service:type` の2つの形を受け付け、`AWS::` の形と `unknown` だけを誤りにする。`ResourceTypeOf`(internal/inventory/inventory.go)の返す `s3`、`sqs`、`ec2:instance`、`unknown` と一致する。業務の場面に `s3` と `sqs` の例も足されている。

> R-02 Reviewer note: BR3.2 が、比べる相手を「別名と値の別名を置き換え、消すキーを除いたあと(必須タグを付ける前)のタグ」と決めた。キーがぶつかる場合の扱いも書かれた。BR3.6 が別名で書いた条件を誤りにし、照会の約束と業務の場面にも例がある。

> R-03 Reviewer note: BR6.7 が 64 KiB に収める規則を定めた。値の候補を件数の少ないキーから外し、それでもキーだけで超えるときは警告して終了コード 1 にする。W2 の手順6、手順8、S1 の表、業務の場面にも反映されている。

> R-04 Reviewer note: C8 の変更として `requiredTags` への統一と、`removeKeys` を `RemoveKey` の一覧にすることが書かれた。entities.md の `RuleSet` と、既存コードの `RequiredTags` の項目とも合っている。

> R-05 Reviewer note: (1) 対応表にない `AWS::...` の種類は「条件に書けない」制限として BR3.4 に明記され、README に書くことになった。誤りの例の文言も矛盾しない。(2) グローバルなリソースは `match.region` の条件に当てはまらないと BR3.2 に決められ、境界の場面にもある。

> R-06 Reviewer note: W2 の手順7に「# Results are incomplete: <リージョン>」の先頭コメントが入り、リソース0件のときは `keys: {}` で出すと書かれた。BR6.5 と業務の場面とも一致する。

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| sensor-traceability | PASS(gaps、orphans、invalid_entries とも空、findings_count 0) | 要件・ストーリー・契約からの対応に抜けや孤立はない。 |

### Summary

前回の6件はすべて直っており、新しい指摘はありません。ルールの読み込み、`rules suggest`、U4 が使う照会の約束は、既存の `ResourceTypeOf` と `RequiredTags` の実装と矛盾せず、実装者が迷わずに作れる状態です。
