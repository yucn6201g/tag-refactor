<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-31T03:23:15Z [unit u8-iac] — 要件 OQ3 の「詳しい振る舞い」を Q1〜Q9 で決めた。CloudFormation は aws:cloudformation の論理 ID のタグ、Terraform は terraform show -json の ARN でつなげ、変更案に項目を足さずに済むと読んだ(契約 C2 の未決事項を閉じた)。
- 2026-10-30T06:43:37Z [unit u7-rollback] — 判定は記録と今のタグだけで行い、完了した rollback の有無は判定に使わず確認画面の警告にした; 同じ値は区別できないため。
- 2026-10-28T15:05:00Z [unit u6-apply] — 書き換え後に期待するタグは、変更案の時点ではなく書き換える直前に読んだタグに変更を当てはめて求め、同じキーへの途中の値は書かずに最終の値だけを書く(U4 から引き継いだ未決事項を解いた)。
- 2026-10-24T16:57:00Z [unit u5-impact] — FR4.5 と AC3.2.4 の「ワイルドカードは ?」を、機械的に判定できない条件だけを ? にする意味に読んだ; ワイルドカードは IAM と同じパターンとして当てはめられ、当てはまれば ! の方が強い印になるため。承認の場で確かめる項目として functional-spec.md の Assumptions に書いた。
- 2026-10-24T06:15:31Z [unit u4-plan] — 必須タグは付いていないときだけ付ける(既にある値は変えない)と読んだ; 値の書き方の統一は値の別名で行う役割分担にした(Q2)。
- 2026-10-24T00:04:11Z [unit u3-rules] — match.tags は別名と値の別名を置き換え、消すキーを除いたあとのタグと比べる; 利用者が正しい名前で条件を書けるようにし、U4 が置き換えの順を決めなくて済むようにするため。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-31T03:23:15Z [unit u8-iac] — 回答 Q5 A の hclwrite は、オブジェクトの中の1つのキーを差し替えられず桁もそろえ直すため、hclsyntax の範囲の差し替えに変えた(Q5 の目的の、変える行だけを変える、に合わせた)。質問のファイルの YAML の依存の前提(go.yaml.in/yaml/v3)は誤りで、goccy/go-yaml を使う。
- 2026-10-30T06:43:37Z [unit u7-rollback] — 契約 C10 の RecordRollback を BeginRollback・RecordRollbackResults・CompleteRollback に分けた。BR7.4 は U6 で実装済みの取り消しのコマンド(--record-from、--profile)を確かめるだけにした。
- 2026-10-28T15:05:00Z [unit u6-apply] — 次の作業単位に U8 が示されたが、Bolt の計画(段階の順)と人の選択に従い、U8 を脇に置いて U6 を先に進めた。
- 2026-10-24T16:57:00Z [unit u5-impact] — 質問になかった権限の上限(permissions boundary)の管理ポリシーも読むことにし、契約 C2 に conditionOperator、unreadPolicies に policyRef と region を足した; 見逃しを減らし、演算子だけが違う指摘を保存した変更案でも区別するため。項目を足すだけなので version は 1 のまま。
- 2026-10-24T06:15:31Z [unit u4-plan] — 契約 C2 の renameKey の意味を mergesIntoExisting で分け、C2・C3 に項目を足した; 値の別名を入れると既にある正しいキー名の値を上書きしてしまうため。contract-summary.md と story map にも書いた。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-31T03:23:15Z [unit u8-iac] — 誤って別のリソースやキーを書き換えないことを優先し、論理 ID と種類、ARN の完全一致、前の状態の値の確かめ、アンカー・1行の書き方・式を手で直す一覧に回す側に倒した。レビュー1回目は NOT-READY(Major 8)、2回目は READY で、残りの Major 1件と Minor 3件も直した。
- 2026-10-30T06:43:37Z [unit u7-rollback] — 記録を読む前に印を取る順番にした; 記録がないなどの誤りも印を取ってから示すが、同時実行で記録を壊さないことを優先した。
- 2026-10-28T15:05:00Z [unit u6-apply] — 50個の上限: 最終が50以下でも途中だけ超えるリソースは消すのを先に書く(飛ばすより書き換えを優先。途中の状態は契約 C4 の表で戻せる)。タグの API に条件付きの書き込みがないため、まとまりごとの読み直しでも、読み直しから書き込みまでのわずかな間の変化は防げないことを受け入れた。
- 2026-10-24T16:57:00Z [unit u5-impact] — 必須タグの追加では IfExists と ForAnyValue・ForAllValues の付いた条件を値に関係なく影響ありにした; タグが無いときの結果が値によらず決まり、正確に判定すると規則が複雑になるため、見逃さない側に倒した。
- 2026-10-24T06:15:31Z [unit u4-plan] — 消す候補のリソース数は一覧と Markdown だけに出し、JSON はキー名だけのままにした; 契約 C2・C3 を変えずに済むため。
- 2026-10-24T00:04:11Z [unit u3-rules] — match.resourceType は scan と同じ形(service と service:type)にした; 収集の種類の決め方が s3 や sqs のようにコロンのない値を返すため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-31T03:23:15Z [unit u8-iac] — hcl/v2 の新しい依存の可否、CloudFormation の Type と resourceType の対応表、terraform show -json の values.arn がある種類、ファイルの大きさと数の上限は NFR の要件と設計で決める。CDK とプルリクエストの作成は後の作業。
- 2026-10-30T06:43:37Z [unit u7-rollback] — タグがすべて消えたリソースをタグの API が返すか、読み戻しの読み直しの回数と間隔は、NFR の設計と検証用アカウントで確かめる。
- 2026-10-28T15:05:00Z [unit u6-apply] — 2回目のレビューの Minor 2件(状態の図に pending から skipped(BR4.7)がない、期待するタグの例の表の4行目が空行で切れている)は、NFR の設計かコードの生成のときに functional-spec.md を直す。BR4.7 で飛ばした項目を U7 が判定しないことは契約 C4 の注に書いた。
- 2026-10-24T16:57:00Z [unit u5-impact] — 契約 C9 の「ワイルドカードは undecided」の文言と ImpactResult の項目を BR2.2 と C2 に合わせる。TagKeys と Null の組み合わせ、ArnNotLike などその他の否定の演算子の扱いをコード生成の前に決める(レビューの Minor の指摘)。
- 2026-10-24T06:15:31Z [unit u4-plan] — U6 が同じキーへの書き込みをまとめて最終の値だけを書くかは U6 の設計で決める; 途中の値が rollback の判定に当たりうるため。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
