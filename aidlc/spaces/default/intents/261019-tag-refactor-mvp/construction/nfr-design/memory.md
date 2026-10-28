<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-25T02:45:58Z [unit u5-impact] — 要件のレビューの Major 2件(API ごとのページの件数、偏ったリージョンの見積もり)を、この設計で決め直すものと読み、段ごとの表と式で直した。
- 2026-10-24T06:40:31Z [unit u4-plan] — Ctrl+C かどうかは誤りの種類ではなく外側の context で決めると読んだ; awsaccess が取り消しを Internal の誤りに変えるため。
- 2026-10-22T12:30:55Z [unit u2-scan] — AWS Config の全種類記録の判定に includeGlobalResourceTypes を使わない; 既定の設定でも false で、2022年2月以降のリージョンでは true にできないため。グローバルな IAM の種類は Config の経路の検出の対象外とし、Checked の行で示す。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-25T02:45:58Z [unit u5-impact] — 人の判断(Q1 A)で、要件の NFR1.18 の「1つのリージョンの中は順に」を「同時に4つまで」に改めた; 多くのアカウントはリソースが1つのリージョンに集まり、順に読むと10分に近づくため。
- 2026-10-24T06:40:31Z [unit u4-plan] — Q2 の「少しずつ書く」は json.Encoder ではなく、配列の項目を1つずつ Marshal する WritePlan で実現する; Encoder は全体を組み立ててから1回で書くため。
- 2026-10-24T00:29:16Z [unit u3-rules] — rules suggest に --output を足した; PowerShell のリダイレクトはコンソールのコードページで子の出力を読むため、日本語の値が保存で文字化けするから。
- 2026-10-22T12:30:55Z [unit u2-scan] — Resource Explorer の GetView を操作に足した; 既定のビューの絞り込みの条件を確かめるため。承認済みの NFR 要件と機能設計の操作の表を同時に直した。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-25T02:45:58Z [unit u5-impact] — 認証情報の誤りで止めるのは cli の包みだけにし、Analyze は特別に扱わない; 既にある U2 の仕組みと二重にならず、どの誤りが返るかが1か所で決まるため。
- 2026-10-24T06:40:31Z [unit u4-plan] — 一時ファイルは書く直前に作り、確かめは作ってすぐ消す試しで行う; 2回目の Ctrl+C で残る時間を書いている数秒だけにするため。
- 2026-10-24T00:29:16Z [unit u3-rules] — 文字コードの見分けと大きさの判定を Parse の先頭にまとめた; ファジングの対象と一致させ、すべての経路に届かせるため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-25T02:45:58Z [unit u5-impact] — コード生成で、IAM の NoSuchEntity も「一覧のあとで消えた」に入れ、NotFound の追加は操作ごとの表(opErrorCodes)に入れて U1〜U4 の分類を変えない。traceability の NFR1.6 の対応先の見出しは「呼び出しの回数の式」(レビューの Minor)。
- 2026-10-24T06:40:31Z [unit u4-plan] — 東アジアの幅の表は Go の unicode と同じ版(17.0.0)から作る。Go を上げたら作り直す手順をコード生成で決める。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->

