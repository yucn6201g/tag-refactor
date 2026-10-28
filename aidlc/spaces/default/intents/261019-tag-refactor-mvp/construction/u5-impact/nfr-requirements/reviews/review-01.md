## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-25T02:29:58Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-requirements/performance-requirements.md > 「呼び出しの回数の式」と NFR1.16 | 式は1ページの件数を単一の `Lp`(「各 API が許す最大。IAM は1,000」)で表し、他の API は「NFR の設計で確かめる」と先送りしている。実際の上限は API ごとに違う。SNS の `ListTopics` はページの件数を指定できず1ページ約100件、Secrets Manager の `ListSecrets` の `MaxResults` は最大100、KMS の `ListKeys` と SQS の `ListQueues` は最大1,000、S3 の `ListBuckets` は最大10,000である。`Lp`=1,000 で上限を作ると、1つのリージョンにトピックやシークレットが500件あるとき、式の `max(1, ⌈t(r)÷Lp⌉)` は1回だが実際は5回になり、NFR1.16 の「式の上限以下」が現実の API の動きでは満たせない。件数の多い一覧ほど式が過小になる。U2 の NFR1.10(ページの件数の罠)と同じ種類の穴である | API ごとのページの件数の定数(IAM 1,000、KMS、SQS、SNS、Secrets Manager、S3 の各値)を AWS のドキュメントで確かめて表に固定し、式を `Lp` 単一ではなく API ごとの値にする。偽の AWS は各 API の実際のページの件数で分割して返すようにし、NFR1.16 の合否の基準に書く。約4,900回の見積もりも更新する | New |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-requirements/performance-requirements.md > NFR1.19 と NFR1.18 | 時間の見積もりが「リージョンの回数の合計 ÷ 8 × L95 × 1.5」で、1つのリージョンの中は順に読む(NFR1.18)という制約のクリティカルパスを反映していない。測る条件はリソースを「1つのリージョンに偏ったものを含める」としているが、見積もりの式は偏りを固定の1.5倍でしか見ない。2,000リソースが1リージョンに集まれば、そのリージョンだけで約2,100回の逐次の呼び出しとなり、L95=0.2秒で約7分かかる。式の結果は約80秒で、10分以内の判定が実態より大幅に甘くなる。また S3 の呼び出し(`b'`+`b`)が「リージョンの回数」に入るのか曖昧で、IAM の分には安全率がなく、U2 の NFR1.8 にある呼び出し回数の上限に当たった待ち時間の加算もない | リソースポリシーの分を `max(合計 ÷ 8 × L95、最大のリージョンの回数 × L95)` のように最大のリージョンの逐次の分を下限にする。S3 の分の扱い、IAM への安全率、呼び出し回数の上限の待ち時間を式に明記する。偏ったケースの見積もりも記録に残させる。この偏りで10分を超えるなら、1リージョン内の同時数(Q1 の C 案)を見直す条件も書く | New |
| R-03 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-requirements/security-requirements.md > NFR8.20、tech-stack-decisions.md > 文書の読み取り | 1 MiB の上限は SDK が応答の本体を読み終えたあとに比べるため、メモリを守るという脅威の欄の説明(「メモリを使い切る」)には効かない。また「大きすぎる」という読めなかった理由は、機能設計 BR1.5 が定める理由(エラーの種類と操作の名前、JSON として読めない場合の固定の文)に無く、リソースポリシーの大きさをどのバイト数で比べるかも書かれていない | 脅威の欄の文言を「パースの時間と CPU」に合わせて直すか、本体の読み込みも制限する。理由の文言を固定し(BR1.5 の追記として)、リソースポリシーは返ったままのバイト数で比べると明記する | New |
| R-04 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-requirements/performance-requirements.md > NFR2.1 | 「ポリシーの種類7」だが機能仕様ではインラインをユーザー、ロール、グループのそれぞれで試すとあり、組み合わせの数(例: 7×7×4)が何件か式で固定されていない。「表の行の数と組み合わせの数が一致する」の組み合わせの数が曖昧で、合否が機械的に決まらない | 期待する組み合わせの数を数式と値で固定し(インラインの3種類の扱いを含む)、テストがその値と照合する形にする | New |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-requirements/security-requirements.md > NFR5.10、NFR8.21 | U2 の NFR5.8 にある「取り消したあとに読み取りの goroutine が残らない」確かめが U5 に引き継がれていない。ファジングの種に `a*a*a*b` のような後戻りの多いワイルドカードと長い入力の組み合わせが無く、正規表現への変換と使い回し(パターンごとのキャッシュ)の上限も無いため、tech-stack の主張(入力の長さに比例)が確かめられない | NFR5.10 の基準に終了時の goroutine の残りが無いことを足す。ファジングの種にワイルドカードの病的な入力を足し、キャッシュの大きさに上限を設ける | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `aidlc engine sensor-traceability` (traceability.json) | pass: true、gaps/orphans/invalid すべて空 | NFR1〜NFR12 の対応は構造として揃っている。N/A の理由(NFR4、NFR6、NFR7 は書き換えと記録、NFR9 は新しい OS 依存なし、NFR11 は色の要件、NFR12 は配布)は要件の記述と矛盾しない |

### 確かめた点

- IAM の操作名は妥当: `s3:ListAllMyBuckets`、`s3:GetBucketPolicy`、`s3:GetBucketLocation`、`kms:ListKeys`、`kms:DescribeKey`、`kms:GetKeyPolicy`、`sqs:ListQueues`、`sqs:GetQueueAttributes`、`sns:ListTopics`、`sns:GetTopicAttributes`、`secretsmanager:ListSecrets`、`secretsmanager:GetResourcePolicy`、および IAM の各 List/Get。すべて Get/List/Describe で始まり、読み取りの例の検査と両立する。
- 呼び出し回数の式の構造は W2 と BR1.1〜BR1.3 の流れに一致し、IAM の `MaxItems` 1,000 は `internal/awsaccess/read.go` の `policyPageSize` とも整合する(上記 Major は他 API の値に関するもの)。約4,900回の合計は式から再計算して一致した。
- 同時数 IAM 4 とリージョン 8 は、結果を集めてから BR5.2 の鍵で並べ直す設計のため決定性と矛盾しない。

### Summary

Critical は無く、Major は2件(API ごとのページの件数を無視した呼び出し回数の式、1リージョン内が逐次であることを反映しない10分の見積もり)で、READY の基準(Major 2件以下)の範囲内である。ただし2件ともコード生成後に手戻りを生むため、承認の前に直すことを推奨する。
