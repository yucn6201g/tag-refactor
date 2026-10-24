<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-24T09:54:02Z [unit u4-plan] — 幅の表の版は「Go の版以上なら通す」と読んだ; CI の stable と oldstable で Unicode の版が違っても片方が落ちないようにするため。
- 2026-10-22T12:47:42Z [unit u2-scan] — タグのないキューは CloudFormation ではなく検証のスクリプトで作る; CloudFormation が aws:cloudformation のタグを自動で付け、タグの取得の API で見つかってしまうため。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-24T09:54:02Z [unit u4-plan] — 結合テストは JSON と --out を同じ1回の収集で動かし、ほかの形式は別に動かす形にした; 5回の収集の件数がずれる心配を減らすため。
- 2026-10-22T12:47:42Z [unit u2-scan] — R2 は U1 のスタックのものを共有する; 例のポリシーと一致させる場所を1つにするため。U1 の仕様の「後の作業単位は自分のスタックを作る」から外れる。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-24T09:54:02Z [unit u4-plan] — stable の Go の Unicode が上がると CI が落ちる手間を受け入れた; 幅の表が黙って古くなるのを防ぐため。
- 2026-10-24T00:37:11Z [unit u3-rules] — U3 の実機の確認は verify.sh の U2 のキューの前に置き、cleanup と trap の定義をスクリプトの先頭近くに移す; U2 の長い待ちに巻き込まれず、途中で止まっても一時ファイルとキューを残さないため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-24T09:54:02Z [unit u4-plan] — なし。
- 2026-10-22T12:47:42Z [unit u2-scan] — レビューの指摘(verify-u1.sh の -run の絞り込み、他のリージョンの索引の事前確認、待ちの検索を種類で絞る、race ジョブの時間、アカウントの確認、索引の時間)はコードの生成の計画で扱う; 判定は READY のため設計の文書は直していない。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
