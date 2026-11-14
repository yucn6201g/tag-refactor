<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->
- 2026-11-07T14:55:31Z — 説明が指す前の intent の記録(requirements、stories、U8 の機能設計)と README の iac の節は、質問の形を決めるためだけに読んだ。そこに書かれた内容は、Q1〜Q8 の回答で確かめたものだけを文書に入れた。

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-07T14:55:31Z — Q8 は「特になし」で、式で作る CDK のタグ、TypeScript と Python 以外の言語、GitHub 以外、作成後の操作のどれも、除外としては確かめていない。どこまで自動で反映するかと、プルリクエストの作り方(認証、ブランチ名など)は、範囲の定義と要件の分析で決める。
