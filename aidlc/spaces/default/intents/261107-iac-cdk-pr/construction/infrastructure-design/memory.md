<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-08T14:00:00Z — U1 のインフラの設計のレビュー(READY)で残った Major 2件と Minor 3件は、U1 の Code Generation の計画で直す。(1) 検証用のスタックを置くプロファイルは、タグの読み書きだけの tag-refactor-apply ではなく、今の README の手順と同じ管理者の tag-refactor-sandbox にする(AWS の変更は人の明示の許可を得てから)。(2) テスト用のアプリは、ブートストラップの無いアカウントに置けるよう、BootstrapVersion のルールを出さない合成器にし、stackName を tag-refactor-cdk-skeleton に固定する。synth の出力で確かめる。(3) github-pr の後片付けが取り消しで終わらないときのため、テストの始めに、前の実行が残した古いブランチとプルリクエスト(作ってから1日を超えたもの)を片付ける。(4) テスト専用のリポジトリのアプリの依存は、tag-refactor 側の ts-minimal を写す手順で合わせる(Dependabot は tag-refactor 側だけ)。
