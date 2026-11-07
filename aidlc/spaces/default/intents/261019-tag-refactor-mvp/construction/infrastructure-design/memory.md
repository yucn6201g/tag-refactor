<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-31T14:35:06Z [unit u8-iac] — iac は AWS を呼ばないので、新しいスタックは作らず、検証は確認用のスタックの読み取り(plan、describe-stack-resource、validate-template)と、test/sandbox の写しの書き換えだけにすると読んだ。
- 2026-10-30T07:26:22Z [unit u7-rollback] — 0個に戻る向きは、plan のリージョン全体の読み取りが0個のキューを返すときだけ検証用アカウントで確かめ、返さなければ偽の AWS に任せる。
- 2026-10-29T03:00:00Z [unit u6-apply] — CloudFormation が作るリソースには aws:cloudformation のタグが付くため、一度もタグのないリソースはスタックで作れない。U2 の検証が作るタグのないキューを使い回して確かめる。
- 2026-10-25T02:58:55Z [unit u5-impact] — NFR2.2 の「少なくとも1つのリソースポリシー」を、5種類すべてのリソースポリシーとインラインポリシーを R2 の実際の権限で読むことと読んだ(Q1 A)。
- 2026-10-24T09:54:02Z [unit u4-plan] — 幅の表の版は「Go の版以上なら通す」と読んだ; CI の stable と oldstable で Unicode の版が違っても片方が落ちないようにするため。
- 2026-10-22T12:47:42Z [unit u2-scan] — タグのないキューは CloudFormation ではなく検証のスクリプトで作る; CloudFormation が aws:cloudformation のタグを自動で付け、タグの取得の API で見つかってしまうため。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-31T14:35:06Z [unit u8-iac] — まとめでは「U6 の3つのリソース、6行」としたが、確認用のスタックはすべてスタックのタグ project で作られ、テンプレートに project を書くのは VU6 と GU6 だけだったので、2つが反映(4行)、ほかは keyNotInSource で終了コード 2 という期待に訂正した。ci.yml の test に CGO_ENABLED=0 を足し、lint で書き換えたテンプレートの cfn-lint を足す(レビュー1回目)。
- 2026-10-30T07:26:22Z [unit u7-rollback] — 保存の時間は新しい debug の行を足さず、S3 の rollback の s3:PutObject の行で測るよう NFR の設計も直した。
- 2026-10-29T03:00:00Z [unit u6-apply] — 承認した Q1 A(U6 のスタックは VPC・セキュリティグループ・R3 だけ)に、S3 の記録の確かめ用のバケット BU6R とバケットポリシーを足した。変更セットを示すときに人に伝える。
- 2026-10-25T02:58:55Z [unit u5-impact] — IAM のロールが地域ごとの収集に出るかを読み取りだけで確かめる予定だったが、サインインの期限切れでできず、RU5 の Resource の指摘は変更案に RU5 の変更があるときだけ確かめる形にした。
- 2026-10-24T09:54:02Z [unit u4-plan] — 結合テストは JSON と --out を同じ1回の収集で動かし、ほかの形式は別に動かす形にした; 5回の収集の件数がずれる心配を減らすため。
- 2026-10-22T12:47:42Z [unit u2-scan] — R2 は U1 のスタックのものを共有する; 例のポリシーと一致させる場所を1つにするため。U1 の仕様の「後の作業単位は自分のスタックを作る」から外れる。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-31T14:35:06Z [unit u8-iac] — 手元に cfn-lint がないので、検証では validate-template を必ず使い、書き換えたテンプレートの cfn-lint は CI の lint のジョブで偽の変更案を使って確かめる形にした。人の確認用のフォルダーは、すべての確かめが通ったあとにだけ作る。
- 2026-10-30T07:26:22Z [unit u7-rollback] — u7-zero-add のルールは ap-northeast-1 の確認用のキューすべてに合うが、名前で絞れないため、写しを取って戻ったことを確かめる形で受け入れた。
- 2026-10-29T03:00:00Z [unit u6-apply] — 書き換えの範囲はルールの when(種類と project のタグ)で絞り、書き換える前の plan で範囲を確かめる。確認用のスタックの同じ種類のリソース(U1・U2 のキューやロール、R3)にも確かめ専用のキーが付いて消えることは受け入れた。
- 2026-10-25T02:58:55Z [unit u5-impact] — 費用のかかる KMS のキーとシークレットも置いた(数日で数十円); R2 の権限の誤りを実際の AWS で見つけられるため。
- 2026-10-24T09:54:02Z [unit u4-plan] — stable の Go の Unicode が上がると CI が落ちる手間を受け入れた; 幅の表が黙って古くなるのを防ぐため。
- 2026-10-24T00:37:11Z [unit u3-rules] — U3 の実機の確認は verify.sh の U2 のキューの前に置き、cleanup と trap の定義をスクリプトの先頭近くに移す; U2 の長い待ちに巻き込まれず、途中で止まっても一時ファイルとキューを残さないため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-31T14:35:06Z [unit u8-iac] — restype の resourceType の書き方(S3 は s3)と、検証が読む英語の文言(keyNotInSource の理由、反映済みの数)は Code Generation で決める。Windows の Git Bash での cygpath の有無は検証で確かめる。
- 2026-10-30T07:26:22Z [unit u7-rollback] — タグが0個のキューをタグの API が返すかは、U7 のチェックポイントで3通りのどれかを記録する。
- 2026-10-29T03:00:00Z [unit u6-apply] — 2回目のレビューの Minor: 後片付けの旗を手順 5 の前に立て直し、手順 5 のあとで下ろす(verify.sh を作るときに直す)。
- 2026-10-25T02:58:55Z [unit u5-impact] — コード生成で、ValidatePolicy は RU5 をアイデンティティポリシー、ほかの5つをリソースポリシーとして当てる。測る値の「上限に当たった回数」は「再試行が起きた行の数」と呼ぶ(レビューの Minor)。チェックポイントで RU5 が収集に出たかを記録する。
- 2026-10-24T09:54:02Z [unit u4-plan] — なし。
- 2026-10-22T12:47:42Z [unit u2-scan] — レビューの指摘(verify-u1.sh の -run の絞り込み、他のリージョンの索引の事前確認、待ちの検索を種類で絞る、race ジョブの時間、アカウントの確認、索引の時間)はコードの生成の計画で扱う; 判定は READY のため設計の文書は直していない。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
