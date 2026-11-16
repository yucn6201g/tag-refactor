<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->
- 2026-11-15T07:46:34Z [unit u3-cdk-synth] — Python の版が古いときの失敗の理由は、toolMissing に混ぜずに新しく toolTooOld(道具 python)を足すことにした(NFR6.16)。利用者に「入れる」と「上げる」のどちらが要るかを分けて示せる。代わりに契約 C2 の Failure の理由の一覧を直す必要があり、Code Generation の計画で「Construction での直し」として書く。
- 2026-11-15T07:58:00Z [unit u3-cdk-synth] — 1回目のレビュー(Major 1件、Minor 6件)は、書式が足りずに判定を記録できなかった。指摘をすべて直してから、1回目のレビューを頼み直した。主な直し: U1 の NFR3.5(analyze は1回)を NFR3.17(trace のときだけ2回)で置き換え、最長時間の式に版の確かめと補助のプログラムの3回を入れた。Python の版が読めない答えは toolMissing にし、最小の版は py-minimal の aws-cdk-lib の Requires-Python に合わせてテストで照らす。CI の pip は wheel だけ、npm は --ignore-scripts。
- 2026-11-15T07:46:34Z [unit u3-cdk-synth] — U3 の種類では security-requirements.md と tech-stack-decisions.md しか作らないので、性能、信頼性、互換性、テスト、観測の要件(NFR3.11〜NFR3.16、NFR4.5、NFR5.3、NFR8.3、NFR8.4、NFR10.4、NFR10.5)を security-requirements.md の「ほかの区分の要件」の表にまとめた(U2 の NFR9.2 と同じ置き方)。
- 2026-11-08T11:00:00Z — U1 でアプリが2つ以上見つかったときは、フォルダーの名前の順で最初の1つだけを synth し、残りは外して理由を示すことにした(NFR3.8)。単位の定義の「cdk.json を1つ」から決めたもので、複数のアプリの扱いの全体は U3 で足す。

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-08T11:20:00Z — U1 の非機能の要件のレビュー(READY)で残った Major 2件(git と gh の標準エラーや外の出力を経由した ARN とアカウント ID の漏れ、CI のトークンの権限と受け取る仕事の範囲と npm ci に渡さないこと)と Minor 5件は、U1 の NFR Design で手当てを決める。
