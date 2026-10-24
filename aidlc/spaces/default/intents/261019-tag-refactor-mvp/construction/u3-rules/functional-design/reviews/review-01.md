## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T00:00:43Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | high | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/rules.md > BR3.4 (および functional-spec.md の「誤りの例」) | BR3.4 は `match.resourceType` の形を「小文字・数字・ハイフンのサービス名」+「:」+「1文字以上の種類」と定めている。しかし既存コード `internal/inventory/inventory.go` の `ResourceTypeOf` は、種類の部分がない ARN に `s3`(`arn:aws:s3:::bucket`)や `sqs`(`arn:aws:sqs:...:queue`)のようにコロンのない値を返し、解釈できない ARN には `unknown` を返す。つまり scan が表示し、BR3.2 が「完全一致」で比べる値そのものを、BR3.4 は誤りとして拒否する。S3 バケット、SQS キュー、SNS トピックなど、よくタグを付ける種類に対して `when: match: {resourceType: ...}` を書けなくなる。Q6 の「scan の出力と同じ形なので、そのまま写せる」とも食い違う | BR3.4 の形の定義を、`ResourceTypeOf` が実際に返す値(`service` だけの形と `service:type` の形)に合わせる。「`unknown` は書けない」などの扱いも決める。誤りの例と、`ec2:instance` だけを挙げている説明にも `s3`、`sqs` を足し、これらを読み込めて BR3.2 で一致することを受け入れ条件の場面に加える | New |
| R-02 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/rules.md > BR3.2 logic(match.tags)と functional-spec.md「照会の約束」 | 必須タグの値の照会は「リソース(種類、リージョン、タグ)」を入力とするが、`match.tags` を比べる相手のタグが、収集した元のタグか、`keys` の別名の置き換えと値の別名の置き換えを済ませたあとのタグかが決まっていない。たとえば `match.tags: {env: prod}` に対して、リソースが `Env: Prod` を持つとき、同じルールファイルでも U4 の組み込み方で当てはまるかどうかが変わる。BR3.2 は「同じ入力には常に同じ答え」と言うが、入力の意味が決まっていないため U4 が推測で実装することになる | `match.tags` のキーと値を、どのタグの状態に対して比べるかを決めて BR3.2 に書く(例: 元のタグ、または別名と値を置き換えたあとのタグ)。`match.tags` のキーを別名で書いたときの扱いも決める。決めた内容を例つきで業務の場面に加える | New |
| R-03 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/rules.md > BR6.2, BR6.3, BR6.4 と、functional-spec.md > 下書きの例 | 下書きは見つかったすべてのキーと値の候補を二重引用符つきで出し、制御文字などは `uXXXX` に展開する。一方、読み込み側は 64 KiB を超えるファイルを拒否する(`internal/rules/rules.go` の `MaxFileSize`、BR1 系で「そのまま使う」と明記)。キーの種類が多いアカウントや、値の候補を最大20個ずつ出すキーが多いアカウントでは、下書きが 64 KiB を超えうる。その場合、`rules suggest` の出力をそのまま `plan --rules` に渡せず、AC2.1.1(そのまま読み込める)が満たせない。上限を超えたときの規則がどこにもない | 下書きが読み込みの上限に収まることを保証する規則を BR6 系に足す(例: 上限に近づいたら値の候補を出さない、またはキー数の上限と理由のコメントを入れる)。または、上限を超える場合の振る舞い(標準エラーの警告と終了コード)を決める。見積もりの根拠と、上限を超える場合のテストを場面に加える | New |
| R-04 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/functional-spec.md > 契約への変更 > C8 と entities.md > RuleSet | 契約 C8 の `RuleSet` の項目名は `requiredTagRules` だが、エンティティと既存コードは `requiredTags` を使っている。契約への変更の一覧は `keyRules.values` だけを挙げており、この名前の違いを直すと書いていない。U4 が契約を見て実装すると、名前が合わない | C8 の変更として `requiredTagRules` を `requiredTags` に直す(または逆に合わせる)ことを、契約への変更の一覧に明記する。あわせて `removeKeys` が `RemoveKey` の一覧になったことも書く | New |
| R-05 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/rules.md > BR3.4 と BR3.2(region の一致) | (1) U2 は、対応表にない AWS Config の種類を `AWS::...` の形のまま持つ。BR3.4 はそれを誤りにし、直し方を「scan の出力の種類の書き方」と案内するが、その書き方も `AWS::...` なので、書ける形が存在しない。(2) グローバルなリソースのリージョンは、読んだリージョンのうち名前順で最初のものになるため、`--region` で絞ると変わる。`match.region` の結果が、ルールファイルではなく実行時の絞り込みで変わる | (1) 対応表にない種類を条件で扱えないことを制限として明記し、案内の文言を矛盾しないものにする。(2) グローバルなリソースが `match.region` に当てはまるかどうかの扱い(当てはまらない、または常に当てはまるなど)を決めて書く | New |
| R-06 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u3-rules/functional-design/functional-spec.md > 流れ W2 の手順7 と下書きの例 | BR6.6 は、読めなかったリージョンがあるとき下書きの先頭のコメントにも「結果は不完全」とリージョンを書くと定めているが、W2 の手順7と下書きの例には標準エラーの出力しか書かれていない。また、タグの付いたリソースが1つもないときの下書き(`keys` が空)の形が決まっていない | W2 の手順7と例に先頭コメントの不完全の表示を足す。リソースが0件のとき、読み込める形(例 `keys: {}`)で出すことを BR6 系に書く | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| aidlc engine sensor-traceability | PASS(gaps、orphans、invalid はいずれも空) | 受け入れ条件 AC2.1.1〜AC2.1.5 と規則の対応は構造上そろっている。上の指摘は対応表の外にある設計内容に関するもの |

### Summary

構造上の追跡は通っているが、`match.resourceType` の形の定義(BR3.4)が、既存の収集が実際に返す値(`s3`、`sqs`、`unknown`)を拒否するため、よく使う種類に条件を書けない。加えて、`match.tags` の比較の基準と、下書きが読み込みの上限(64 KiB)を超える場合が未決で、U4 と利用者がそのまま実装・利用できない。この3点を直せば READY に近づく。
