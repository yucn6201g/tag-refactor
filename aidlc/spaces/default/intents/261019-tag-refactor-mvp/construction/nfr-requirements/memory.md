<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-25T02:30:56Z [unit u5-impact] — 作業単位の種類は library で、性能の要件は必須の成果物ではないが、NFR1(10分)と NFR2(検出率100%)が U5 の中心のため performance-requirements.md も作った。
- 2026-10-24T06:30:19Z [unit u4-plan] — Q3 の「HTML として働かせない」は、データを必ずコードスパンに入れることで満たすと読んだ; コードスパンの中で文字の参照に変えると、コピーした値が元と変わるため。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-25T02:30:56Z [unit u5-impact] — レビューの Major 2件(API ごとのページの件数を無視した呼び出しの回数の式、1つのリージョンの中を順に読むことを反映しない10分の見積もり)は、READY の判定のあと要件の文書を直さず、NFR の設計で API ごとのページの件数の表と、最も多いリージョンを下限にした見積もりの式として決め直す。
- 2026-10-24T06:30:19Z [unit u4-plan] — Windows の標準出力でも仮想端末の処理を U4 で有効にし、終了時に戻すことにした; U2 は標準エラーの進み具合のときだけ有効にしていたため。
- 2026-10-24T00:10:00Z [unit u3-rules] — PowerShell 5.1 のリダイレクトが書く UTF-16 のルールファイルを読めるようにする要件(NFR9.5)を足した; 下書きを Windows でそのまま保存して使う主要な流れを壊さないため。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-25T02:30:56Z [unit u5-impact] — IAM は同時に4つ、リソースポリシーはリージョンを同時に8つ(1つのリージョンの中は順に)とした; IAM はアカウント全体で呼び出し回数の上限を持ち、リソースポリシーはリージョンごとに上限を持つため、U2 の収集と同じ作りに揃えた。
- 2026-10-24T06:30:19Z [unit u4-plan] — 読み込みの上限を 256 MiB とし、超える変更案は保存しない; 最悪の大きさ(500 MiB 以上)まで受けるとメモリが数 GB になるため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-25T02:30:56Z [unit u5-impact] — 1 MiB の上限は SDK が本体を読んだあとに比べるため、メモリより解析の時間を守るもの。大きすぎるときの理由の文言、NFR2.1 の組み合わせの数、取り消し後の goroutine の確認、病的なワイルドカードの種を NFR の設計で決める(レビューの Minor)。
- 2026-10-24T06:30:19Z [unit u4-plan] — コード生成の計画では、色の条件を機能設計 BR3.7 ではなく NFR11.7(TERM=dumb と Windows の仮想端末の有効化を含む)から取る。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
