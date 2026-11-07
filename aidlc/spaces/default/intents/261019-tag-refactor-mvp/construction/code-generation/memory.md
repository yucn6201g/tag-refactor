<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-11-07T03:02:14Z [unit u8-iac] — 設計の入れ子の守りは「深さ」として書かれていたが、目的は解析器を止めないことだと読み、時間が爆発する形(部品の数の2乗)も同じ守りの中で外すことにした。
- 2026-10-31T02:11:31Z [unit u7-rollback] — 書き戻しの読み直しの100個の単位は、判定したすべてではなく書き戻す項目だけで切ると読んだ; すでに決まった項目を読み直しても意味がなく、呼び出しの数は NFR5.20 の上限以下のまま。
- 2026-10-29T10:04:22Z [unit u6-apply] — 2回目の Ctrl+C で取り消した書き込みは、読み戻しで変更前のタグが見えても resultUnknown とした; 取り消した要求が AWS 側で後から反映されうるため(BR4.5)。
- 2026-10-28T07:12:48Z [unit u5-impact] — 計画の「結果は code-summary.md に書く」は、作る人ではなく取りまとめが Step 5 で書くものと読んだ(作る人には aidlc/ の書き込みを計画のチェックだけに限ったため)。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-11-07T03:02:14Z [unit u8-iac] — 一時ファイルを同じハンドルで読み直す(Windows で開き直しが遅いため。原因は推測で未計測)。.tf の走査に %{if}・%{for} の深さと、テンプレートの部品の費用の上限を足した。S3・SQS・SNS の resourceType は scan の書き方に合わせた。
- 2026-10-31T02:11:31Z [unit u7-rollback] — main の CI が Dependabot の #11 以降の壊れた go.sum で赤だったため、ブランチの最初のコミットで go.sum と間接依存3つを直した(U7 とは別の前提の修正)。契約の RollbackRun の名前を書き込みの型に使い、JSON の型を RollbackRecord に改名した。
- 2026-10-29T10:04:22Z [unit u6-apply] — ルールファイルの種類を scan の形で書き、権限の例の S3 は s3:ListBucket を含む4操作にし、契約 C10 の記録の操作を Run のメソッドにした。記録と S3 の印は 404 に頼らない形にレビュー後に直した。
- 2026-10-28T07:12:48Z [unit u5-impact] — ワイルドカードの使い回しに 4,096 バイト・4,096 文字の守りを足し、インラインポリシーの一覧を読めないときの policyRef をユーザー・ロール・グループの ARN にした。作る人がシェルでファイルを書いた場面が3回あった。
- 2026-10-23T14:05:15Z [unit u2-scan] — scan が required のキーを受け取るため internal/rules に RequiredTags を足した; 設計では変えない部品だったが、契約 C8 の requiredTagRules に沿う追加として扱った。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-11-07T03:02:14Z [unit u8-iac] — レビューは READY で Major 1件だったが、1つの .tf で iac が実質止まるので、チェックポイントの前に直した。形を変えて測ると、レビューにない heredoc の爆発も見つかった。費用の上限で 1 MB・1万行を超える heredoc は外れるが、ふつうの heredoc は外れないことを確かめた。
- 2026-10-31T02:11:31Z [unit u7-rollback] — レビューは READY だったが、Minor 3件(古いヘルプの文、権限のテストを実際の呼び出しで確かめる、飛ばしの reason の書き方)をチェックポイントの前に直した。依存の修正は U7 の squash に混ざるので、main を先に直すかは人に委ねる。
- 2026-10-29T10:04:22Z [unit u6-apply] — レビューは READY だったが、S3 の 404 への依存と固定の待ちのテストをチェックポイントの前に直した。権限の例の s3:prefix の条件は、バケット全体の一覧を許さないよう変えず、検証用アカウントで確かめてから決める。
- 2026-10-28T07:12:48Z [unit u5-impact] — U2 の性能テストの呼び出し回数の上限に、U5 の空のアカウントの分を足した; 読む操作が増えた分で、下限を緩めたのではない。
- 2026-10-23T14:05:15Z [unit u2-scan] — レビューは READY だったが、2回の試行で見つかった中程度の不具合2件(不完全な scan の No issues found、集約の索引の失敗後のリージョンの帰属)をチェックポイントの前に直した; 誤った結果を利用者に見せないため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-11-07T03:02:14Z [unit u8-iac] — Terraform の例は手で作ったもので、各種類の values.arn は未確認(NFR10.14 (c) が弱いまま)。-race、Linux と macOS、cfn-lint は CI で、verify.sh の U8 の手順はチェックポイントで確かめる。
- 2026-10-31T02:11:31Z [unit u7-rollback] — verify.sh の U7 の手順(一時キューがタグの API に出るまでの待ち、ARN を指定した読み取り)は未実行でチェックポイントで確かめる。-race(結果の記録と5秒ごとの保存の同時実行)、3つの OS、cfn-lint は CI で確かめる。
- 2026-10-29T10:04:22Z [unit u6-apply] — 条件つきの s3:ListBucket のもとで、ないキーの読み取りが 404 になるか(チェックポイントの手順 6)。EC2・S3・IAM・RDS のタグの操作の Resource の形、-race、cfn-lint、3つの OS は未確認。U6 のスタックの作成と tag-refactor-apply のプロファイルは人の許可と作業を待つ。
- 2026-10-28T07:12:48Z [unit u5-impact] — レビューの Minor 4件(verify.sh の測定ループの終了コード1、jsonUnread の構造体変換、? の理由の表示、unit-test-instructions.md のアカウント ID)は次の手直しで扱う。検証用アカウントのスタックの更新と作成は人の許可を待つ。
- 2026-10-23T14:05:15Z [unit u2-scan] — 既定のビューがないときの GetDefaultView の実機の形、Config の問い合わせの受理、-race、go1.27.3 での govulncheck は未確認; チェックポイントと CI で確かめる。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
