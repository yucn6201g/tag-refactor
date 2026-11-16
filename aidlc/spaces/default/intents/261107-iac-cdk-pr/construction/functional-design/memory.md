<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

- 2026-11-15T07:28:25Z [unit u3-cdk-synth] — Python のアプリは、アプリのフォルダーの .venv の Python を先に使うことにした(BR2.2)。要件は「利用者の端末の Python を使う」だけで、cdk init の既定の形(.venv)を有効にし忘れた利用者でも synth できるようにするため。パッケージを入れたり .venv を作ったりはしない。

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

- 2026-11-15T07:37:45Z [unit u3-cdk-synth] — 1回目のレビュー(NOT-READY)を受けて、Windows の .venv に python3.exe が無い問題は、app の命令の最初の語を .venv の Python の絶対パスに置き換えて cdk synth --app で渡す形にした(BR2.2)。cdk.json は変えずに済み、PATH だけに頼るより確実。進み具合の時計は display.Printer に持たせ、ProgressSink にはメソッドを足さない(BR5.4、R-04)。Python のアプリは U5 まで Analyze も作成の記録での補いもしない(BR3.3)。
- 2026-11-15T07:28:25Z [unit u3-cdk-synth] — 作成の記録での補い(ADR-004)は、作成の箇所が見つからないリソースがあるアプリだけ cdk synth --debug で1回 synth し直す形にし、2回目の出力が1回目と違えば記録を使わないことにした(BR4.1、BR4.4)。いつも --debug にするより時間がかからず、出力が毎回変わるアプリで違う行を指す危険を避けられる。

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-09T08:30:00Z — U2 の機能の設計の2回目のレビュー(READY)で残った Minor 3件は、U2 の Code Generation の計画で決める。(1) 本文のファイルを Publish の始めに書けないときは、ブランチを作らずに止め、終了コード 1、続きの命令は本文を --body を使わずに手で作る案内にする。(2) 60,000 文字と 65,536 文字は Unicode のコードポイントで数える。(3) BR2.3 の gitCheck は、書き換え案ができたあと、ファイルを書き出す前に行う(今の iac と同じ)。W1-3 の説明をそれに合わせる。entities.md の line_format の YAML の崩れは直した。
- 2026-11-08T10:40:00Z — U1 の機能の設計の2回目のレビューで残った Minor 4件(キーの箇所が0か2以上のときの場所のファイルと理由を出す段階、BR3.4 と契約の recheck_failed の理由の食い違い、テストの push の認証の渡し方と Windows での node_modules のつなぎ方、BR4.1 の行き先のブランチと merge の途中の確かめ)は、U1 の Code Generation の計画で決める。
- 2026-11-15T07:45:00Z [unit u3-cdk-synth] — 2回目のレビュー(READY)で残った Minor 2件は、U3 の Code Generation の計画で決める。(1) entities.md の UnattributedManualReason の tool を添える条件を「synth に失敗したアプリがちょうど1つ」にそろえる(BR3.2 のとおり、補助のプログラムの失敗は数えない)。(2) 外すアプリ(other、v1、版が不明)の AppSkipped を IacWriter と Synthesize のどちらが送るかを決め、契約 C2 と違うならその旨を書く。
- 2026-11-15T07:28:25Z [unit u3-cdk-synth] — Code Generation で、cdk synth --debug の作成の記録が metadata のどの欄にどの形で入るか(BR4.2)と、needsAWS の8つの文(BR2.6)を、テスト用のアプリの実際の出力と照らして決める。Python のアプリの記録に利用者の .py の行が入るかも確かめる。
