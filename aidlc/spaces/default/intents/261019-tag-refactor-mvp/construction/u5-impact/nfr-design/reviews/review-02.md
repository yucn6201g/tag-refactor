## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-25T02:45:09Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-design/security-design.md > §2 の5(ワイルドカードの使い回しの上限)、logical-components.md > 結果の順と照合の作り(変更の索引は読むだけ) | 設計は「変更の索引は最初に1回作り、読み取りの仕事は読むだけ」と述べ、同時に動く仕事が結果を共有の入れ物に書かないとしている。しかし、使い回しの対象の正規表現は、現行の `internal/impact/impact.go` の `changeIndex.pattern`(186〜205行)で、索引の中の `patterns` マップに、照合のたびに書き込まれている。複数の goroutine が `match` を呼ぶと、Go の「concurrent map writes」(回復できない致命的な異常)で plan が落ちる。1,024個の上限も、どの単位で数えるか(索引全体か、仕事ごとか)が書かれておらず、同時に数える場合の扱いが決まっていない。索引を「読むだけ」とする主張と矛盾する | 正規表現の使い回しを、(a) 仕事ごと(またはリージョンごと)の入れ物にする、(b) ロックで守った1つの入れ物にして、上限の数え方もその中で行う、のどちらかに決めて書く。索引の本体(`byKey`、`keys`)は読むだけ、使い回しの入れ物は別、と分けて示す。`-race` 付きで、ワイルドカードを含むポリシーを同時に読む大きな偽の AWS のテストを通すことを、テストの設計に明記する | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-design/logical-components.md > 同時に読む作り、10分の見積もり、1ページの件数と呼び出しの回数 | 同時に読む作りと、呼び出しの式・見積もりが一致していない。(1) 「リージョンごと: 4つのサービスの一覧を順に読む → ポリシーの読み取りを SetLimit(4)」とあるが、KMS の `DescribeKey`(利用者が作ったキーの選別、キーごとに1回)、SQS の `GetQueueAttributes`、S3 の `GetBucketPolicy`、`GetBucketLocation` が「一覧」の段(順)か「ポリシーの読み取り」の段(4つまで同時)か書かれていない。`DescribeKey` が一覧の段なら、式の「最も多いリージョンの回数 ÷ 4」は過小になる。(2) リージョンごとの式に `b(r)`(そのリージョンのバケットのポリシー読み取り)が無く、見積もりの「最も呼び出しの多いリージョン」にバケットが入るかが決まらない。S3 の `GetBucketLocation` を「1つずつ順に」読むとあるが、全バケットで答えにリージョンが無い場合は B 回の直列になる。(3) 「管理ポリシーの一覧 → 文書を1つずつの仕事に」という書き方は、`errgroup.SetLimit(4)` の仕事の中から次の仕事を `Go` で足すと、4つの枠が親で埋まって待ち合わせる(デッドロック)作りと区別がつかない | 各 API 呼び出しが、どの段(直列、IAM の4つ、リージョンの中の4つ)に入るかを表にする。`DescribeKey` を含む「リソースごとの読み取り」はすべて4つの枠に入れるか、直列とするなら見積もりの式に直列の分を足す。リージョンごとの式に `b(r)` を足し、見積もりの「最も多いリージョン」の定義をその式にそろえる。仕事は、制限のついたグループの外の1つの親(ディスパッチャ)から足し、仕事の中から足さない、と明記する。デッドロックしないことを、枠を小さくした差し替え(テストの変数)で確かめるテストを足す | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-design/security-design.md > §4 失敗の扱いと取り消し(認証情報の誤りの扱い) | 認証情報の誤りで止める仕組みの持ち主が決まっていない。設計は「U1 では読めなかったものの1つにしていた」ので、`Analyze` がその誤りを返すように変えるとしている。しかし現行の `internal/cli/commands.go`(287〜325行)には、すでに `stoppingPolicyReader` があり、読み取りの口を包んで `CredentialsMissing` で実行の ctx を取り消し、誤りを返している。この包みは `PolicyReader` の2つのメソッドだけを持つため、約20個の新しい読み取りの口に広げるか、やめるかが必要になるが、設計の `internal/cli` の行は「リージョンを渡す」だけで、この包みに触れていない。2つの仕組み(`Analyze` 内と CLI の包み)が併存すると、どちらの誤りが返るか(同時に起きた `Expired` と `Rejected` の違い、先着順)が実行ごとに変わりうる。また、§4 のテストは goroutine の残りだけで、認証情報の誤りで残りが取り消され、plan が案内で終わり、終了コード1になることのテストが無い(NFR5.10 の合否の基準に取り消しはあるが、認証情報の誤りはこの設計のテストに無い)。U1 の `impact_test.go` に「認証情報の誤りは読めなかったもの」を期待するテストがあれば、それも直す必要がある | 止める判定を1か所(`Analyze` か CLI の包みのどちらか)に決め、もう一方を外すか、新しい口に広げることを `logical-components.md` の `internal/cli` と `internal/impact` の行に書く。複数の認証情報の誤りが同時に起きたときは、最初に記録した1つを返す、と決めて書く。「認証情報の誤りで残りを取り消し、案内で終わり、終了コード1」「U1 のテストの期待を直す」ことを、テストの設計に足す | Resolved (reviewer) |
| R-04 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-design/logical-components.md > 1ページの件数と呼び出しの回数(「約4,950回」) | 「SNS とシークレットのページが増えて約4,900回から約4,950回」という数が、設計の式から出せない。測る条件では SNS 500、シークレット 500 を17リージョンに分けており、100件のページに変えて増えるのは、偏ったリージョンの分の数回だけである。また、KMS の `k(r)` は AWS が管理するキーを含むため、測る条件の「KMS 300」より多くなる。NFR1.16 の合否は「式の上限以下」なので、上限の数が不確かだと検査が緩むか、通らなくなる | 測る条件のリソースの分布(リージョンごとの k、k'、q、t、s、b、b')を決めて書き、その分布で式を計算した数を示す | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-design/traceability.json > NFR1.17、NFR5.11 | NFR1.17 の対応先は「遅れなしの偽の AWS で10秒以内」とあるが、`logical-components.md` のテストの設計には10秒の基準も、それを測る方法も無く、あるのは「大きなテストを合わせて2分以内」だけである。NFR5.11 も、対応先の節に、乱数の種を変えた10回の実行、標準出力と保存した変更案(`planId`・`createdAt` を除く)の一致、CI の Linux での `-race` 付きの実行、が書かれていない(遅れを乱数で入れ替えられる、`race` のとき件数を10分の1、とだけある) | 対応先の節に、NFR1.17 の10秒の基準と測り方(CI の各 OS)、NFR5.11 の10回の実行と比べるもの、Linux の `-race` を書く | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-design/security-design.md > §1(読み取りの口の表)、docs/iam/readonly-policy.json | 新しい `Get*` の操作(`iam:GetUserPolicy`、`iam:GetRolePolicy`、`iam:GetGroupPolicy`、`s3:GetBucketPolicy`、`kms:GetKeyPolicy`、`sqs:GetQueueAttributes`、`sns:GetTopicAttributes`、`secretsmanager:GetResourcePolicy` など)の `Resource` の範囲が決まっていない。U1 の例は `iam:GetPolicyVersion` を `policy/*` に絞っているのに対し、「広すぎる権限を渡す」脅威への対策が操作名の固定だけになっている | 例のポリシーで、各 `Get*` の `Resource` をアカウントとサービスの ARN の形に絞れるものは絞り、絞れないものは理由を書く。例の検査が `Resource` の形を許すことも確かめる | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-design/security-design.md > §1、§4(新しい5サービスの誤りの扱い) | 「ポリシーなし」の答え(`NoSuchBucketPolicy` など)以外の、新しい5サービスの誤りの扱いが無い。現行の `internal/awsaccess/invoke.go` の誤りコードの表は共有の名前だけで、リソースが一覧と読み取りの間に消えた場合(S3 の `NoSuchBucket`、SQS の `NonExistentQueue`、Secrets Manager・KMS の `ResourceNotFoundException`/`NotFoundException`)は未知のコードとして Internal になり、読めなかったもの(終了コード1)になる。2,000件のリソースがある環境では、実行中に消えるリソースは現実に起きる | 一覧のあとで消えたリソースを「ポリシーなし(対象外)」とするか「読めなかった」とするかを決め、新しい5サービスの誤りコードの対応(アクセス拒否、認証情報、消えた、一時的)を表にして、`invoke.go` の表に足すことを書く | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-design/security-design.md > §1「新しい5つのサービスの誤りの扱い」、internal/awsaccess/invoke.go の `errorCodes` と `opErrorCodes` | 消えたリソースの表は 5 サービスだけで、IAM の `NoSuchEntity` が入っていない。足した IAM の読み取り(`List*Policies`、`Get*Policy`、`GetPolicyVersion`)は 2,500 回前後を 2 分半かけて呼ぶため、一覧のあとで削除されたユーザー、ロール、ポリシーに当たりうる。現行の表に `NoSuchEntity` は無く、Internal の「読めなかったもの」になり、plan が不完全で終了コード 1 になる。また、`ResourceNotFoundException` や `NotFound` は現行の `errorCodes` が全サービス共通の表なので、そこへ足すと Config などほかの操作の分類も変わりうる。表の `opErrorCodes`(操作ごとの表)を使うかどうかが書かれていない | 表に IAM の `NoSuchEntity`(足した操作だけ)を足して NotFound として扱うか、扱わない理由を書く。NotFound の追加は操作ごとの表(`opErrorCodes`)に入れ、U1〜U4 の既存の分類を変えないことを書く | New |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/nfr-design/traceability.json > NFR1.6 の target | 対応先が logical-components.md の「1ページの件数と呼び出しの回数」となっているが、その見出しは「1ページの件数」と「呼び出しの回数の式」に分かれていて、この名前の節は無い | 対応先を「呼び出しの回数の式」(必要なら「1ページの件数」も)に直す | New |

> R-01 Reviewer note: 使い回しの入れ物は変更の索引とは別の部品になり、ロックの中で数えて 1,024 個までと決まった(security-design.md §2 の5、logical-components.md の「結果の順と照合の作り」)。現行の `changeIndex.patterns`(impact.go)を移すと明記され、索引は読むだけという主張とも矛盾しない。ワイルドカードを含むポリシーを多く入れて Linux で `-race` 付きでも動かすテストも「同じ結果」に入った

> R-02 Reviewer note: 「同時に読む作り」に段ごとの表ができ、`GetBucketLocation`(S3 の場所の仕事)、`DescribeKey`/`GetKeyPolicy`/`GetQueueAttributes`/`GetBucketPolicy` などがどの段に入るかが決まった。J(r) に b(r) が入り、仕事は制限のついたグループの外の親だけが足すと明記され、枠をすべて 1 にしても終わるテストもある。件数を要件の測る条件(S3 200、KMS 300、SQS/SNS/Secrets Manager 各 500、IAM 2,504)と突き合わせて式を計算し直すと、IAM の分は約151秒、リソースポリシーの分は約89秒、1リージョンに集まる場合は約179秒で、記述と一致した

> R-03 Reviewer note: 認証情報の誤りで止めるのは `internal/cli` の `stoppingPolicyReader` だけと決まり、足した口のすべてに広げ、最初の 1 つだけを覚えて返すと書かれた(現行の commands.go の `check`/`stopped` と整合)。取り消し、案内、終了コード 1、同時の 2 つの誤り、goroutine の戻りのテストが §4 に入り、U1 の単体テストの扱いも書かれた

> R-04 Reviewer note: 測る条件の分布が決まり、4,913 回という数が式から出せるようになった。IAM 2,504 + S3 の一覧 1 + 偏ったリージョン(L=8、J=1,154)+ 残り 16 リージョン(L=64、J=1,182)の合計を計算し直して一致した。要件の「約4,900回」とも合う

> R-05 Reviewer note: traceability.json の NFR1.17 と NFR5.11 の対応先に、10 秒の基準と測り方、10 回の実行と比べるもの、Linux の `-race` が書かれた。sensor-traceability は pass(gaps、orphans、invalid_targets はいずれも空)

> R-06 Reviewer note: §1 に操作ごとの `Resource` の範囲が決まった(`List*` は `"*"`、1 つのリソースを読む操作は IAM、S3、KMS、SQS、SNS、Secrets Manager の ARN の形)。ARN の形はどれも各操作の対象のリソースの種類と合う。例の検査に `Resource` の形を足すことも書かれた(現行の docs_test.go は `"*"` と GetPolicyVersion の形しか許さないため、この拡張は必要で、設計に入っている)

> R-07 Reviewer note: 新しい 5 サービスの誤りの表(AccessDenied、CredentialsMissing、Transient、NotFound、NoSuchBucketPolicy、KMSInvalidStateException)ができ、一覧のあとで消えたリソースは「ポリシーなし」と同じに扱うと決まった。未対応の点は下の新しい指摘にまとめた

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| aidlc engine sensor-traceability(nfr-design) | PASS: `pass: true`、gaps 空、orphans 空、missing_from_table 空、invalid_targets 空、findings_count 0 | 構造は問題なし。上の NFR1.6 の対応先の名前の古さは、この検査では見つからない内容の指摘 |

### Summary

前回の Major 3 件(R-01〜R-03)と Minor 4 件(R-04〜R-07)は、すべて設計に反映されて解消している。呼び出しの回数(4,913)と 10 分の見積もり(約151秒、約89秒、約179秒)は、要件の測る条件から計算し直して一致した。新しく見つけたのは Minor 2 件だけで、実装を妨げないため READY とする。
