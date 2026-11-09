<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->
- 2026-11-07T15:20:00Z — ステージの手順にある AWS の基盤と法令の担当への引き継ぎはしなかった。この作業では担当を主のエージェントだけにしており(collaborators off)、AWS の資源も規制の要件も増えないため。
- 2026-11-07T15:20:00Z — ステージの例の質問のうち、使っている AWS のサービスとアカウント、規制、期限、チームの技術は前の intent で決まっていたので聞き直さず、CDK とプルリクエストに特有の制約だけを聞いた。

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->
- 2026-11-07T15:20:00Z — Q10 は「任せる」と言われたので A(テスト専用のリポジトリと、そこだけに書けるトークン)を選んだ。B(GITHUB_TOKEN でこのリポジトリ自身に作る)は準備が要らないが、公開のリポジトリに試しのプルリクエストが残り、書き込みの範囲も広い。C は PR の CI で確かめられない。

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-07T15:20:00Z — Q2(ツールが cdk synth を実行する)は、Q9 の答えで AWS の認証情報を渡さないことにした。synth は利用者のアプリのコードを動かすので、時間の上限や書き込み先などの扱いを、要件と設計で決める必要がある。
