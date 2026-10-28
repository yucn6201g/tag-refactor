<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-24T16:57:00Z [unit u5-impact] — FR4.5 と AC3.2.4 の「ワイルドカードは ?」を、機械的に判定できない条件だけを ? にする意味に読んだ; ワイルドカードは IAM と同じパターンとして当てはめられ、当てはまれば ! の方が強い印になるため。承認の場で確かめる項目として functional-spec.md の Assumptions に書いた。
- 2026-10-24T06:15:31Z [unit u4-plan] — 必須タグは付いていないときだけ付ける(既にある値は変えない)と読んだ; 値の書き方の統一は値の別名で行う役割分担にした(Q2)。
- 2026-10-24T00:04:11Z [unit u3-rules] — match.tags は別名と値の別名を置き換え、消すキーを除いたあとのタグと比べる; 利用者が正しい名前で条件を書けるようにし、U4 が置き換えの順を決めなくて済むようにするため。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-24T16:57:00Z [unit u5-impact] — 質問になかった権限の上限(permissions boundary)の管理ポリシーも読むことにし、契約 C2 に conditionOperator、unreadPolicies に policyRef と region を足した; 見逃しを減らし、演算子だけが違う指摘を保存した変更案でも区別するため。項目を足すだけなので version は 1 のまま。
- 2026-10-24T06:15:31Z [unit u4-plan] — 契約 C2 の renameKey の意味を mergesIntoExisting で分け、C2・C3 に項目を足した; 値の別名を入れると既にある正しいキー名の値を上書きしてしまうため。contract-summary.md と story map にも書いた。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-24T16:57:00Z [unit u5-impact] — 必須タグの追加では IfExists と ForAnyValue・ForAllValues の付いた条件を値に関係なく影響ありにした; タグが無いときの結果が値によらず決まり、正確に判定すると規則が複雑になるため、見逃さない側に倒した。
- 2026-10-24T06:15:31Z [unit u4-plan] — 消す候補のリソース数は一覧と Markdown だけに出し、JSON はキー名だけのままにした; 契約 C2・C3 を変えずに済むため。
- 2026-10-24T00:04:11Z [unit u3-rules] — match.resourceType は scan と同じ形(service と service:type)にした; 収集の種類の決め方が s3 や sqs のようにコロンのない値を返すため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-24T16:57:00Z [unit u5-impact] — 契約 C9 の「ワイルドカードは undecided」の文言と ImpactResult の項目を BR2.2 と C2 に合わせる。TagKeys と Null の組み合わせ、ArnNotLike などその他の否定の演算子の扱いをコード生成の前に決める(レビューの Minor の指摘)。
- 2026-10-24T06:15:31Z [unit u4-plan] — U6 が同じキーへの書き込みをまとめて最終の値だけを書くかは U6 の設計で決める; 途中の値が rollback の判定に当たりうるため。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
