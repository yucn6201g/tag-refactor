<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-31T13:48:02Z [unit u8-iac] — iac は AWS を呼ばないので、NFR 設計の中心は利用者のファイルとリポジトリの扱い(位置、Edit、git、書き出し、上限)と読んだ。planner を通した SDK の間接の依存は、実行ファイルが1つで SDK をもともと含むため受け入れ、AWS を呼ばないことは直接の取り込み・口の型・実行のテストで確かめると決めた。
- 2026-10-30T07:11:19Z [unit u7-rollback] — rollback は internal/apply の中に置き、リージョンの役・止める役・まとまりの書き方・2回目の Ctrl+C の放棄の仕組みを apply と共有する小さな型に移す。
- 2026-10-29T02:10:00Z [unit u6-apply] — タグの API の書き込みの失敗の一覧には対象サービスのコードがそのまま入るため、分類の主な鍵を HTTP の状態にし、Transient を使い切ったものは読み戻しで書けたかを決めることにした。
- 2026-10-25T02:45:58Z [unit u5-impact] — 要件のレビューの Major 2件(API ごとのページの件数、偏ったリージョンの見積もり)を、この設計で決め直すものと読み、段ごとの表と式で直した。
- 2026-10-24T06:40:31Z [unit u4-plan] — Ctrl+C かどうかは誤りの種類ではなく外側の context で決めると読んだ; awsaccess が取り消しを Internal の誤りに変えるため。
- 2026-10-22T12:30:55Z [unit u2-scan] — AWS Config の全種類記録の判定に includeGlobalResourceTypes を使わない; 既定の設定でも false で、2022年2月以降のリージョンでは true にできないため。グローバルな IAM の種類は Config の経路の検出の対象外とし、Checked の行で示す。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-31T13:48:02Z [unit u8-iac] — 人の判断で、YAML の解析器を goccy/go-yaml から go.yaml.in/yaml/v3 に変え(goccy の位置がコメント等でずれ、10 MB で約5秒・約1 GB。yaml/v3 は実測で正しく 0.5 秒・約190 MB)、種類の表を SDK に依存しない internal/restype に resourceType ごとの18行で置いた(Q3 A を改めた)。git の確かめは git status をやめ、ls-files と ls-tree のハッシュを自分で比べる。
- 2026-10-30T07:11:19Z [unit u7-rollback] — 契約 C10 に OpenForRollback を足し、3つの呼び出しは RollbackRun のメソッドにした。U6 の LockHandle に目的を持たせる。
- 2026-10-29T02:10:00Z [unit u6-apply] — 書き換え用の権限の例の表は、操作の名前を AWS 管理ポリシーと照らしたが、Resource の形の Service Authorization Reference での確かめはコードの生成の最初の手順に残した。
- 2026-10-25T02:45:58Z [unit u5-impact] — 人の判断(Q1 A)で、要件の NFR1.18 の「1つのリージョンの中は順に」を「同時に4つまで」に改めた; 多くのアカウントはリソースが1つのリージョンに集まり、順に読むと10分に近づくため。
- 2026-10-24T06:40:31Z [unit u4-plan] — Q2 の「少しずつ書く」は json.Encoder ではなく、配列の項目を1つずつ Marshal する WritePlan で実現する; Encoder は全体を組み立ててから1回で書くため。
- 2026-10-24T00:29:16Z [unit u3-rules] — rules suggest に --output を足した; PowerShell のリダイレクトはコンソールのコードページで子の出力を読むため、日本語の値が保存で文字化けするから。
- 2026-10-22T12:30:55Z [unit u2-scan] — Resource Explorer の GetView を操作に足した; 既定のビューの絞り込みの条件を確かめるため。承認済みの NFR 要件と機能設計の操作の表を同時に直した。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-31T13:48:02Z [unit u8-iac] — レビューは1回目 NOT-READY(Critical 1・Major 4)、2回目 NOT-READY(Major 3)で上限に達し、人の判断で直して復旧用の3回目で READY。要素の間のコメントを守るため、組み立て直しをやめて要素ごとの Edit にし、すべて消す書き場所にコメントがあるときは手で直す側に倒した。
- 2026-10-30T07:11:19Z [unit u7-rollback] — レビューは READY だったが、Major 1件(放棄の仕組みの共有)と Minor 8件を設計に入れてから進めた。同じアカウントの中の記録の改ざんは残るリスクとし、置き場所の権限で守る。
- 2026-10-29T02:10:00Z [unit u6-apply] — 一時的な失敗の数え役は全リージョン共有(NFR5.17)のため、1つのリージョンだけの障害では止まらない。全体を止めるより、ほかのリージョンの書き換えを続けることを選んだ。
- 2026-10-25T02:45:58Z [unit u5-impact] — 認証情報の誤りで止めるのは cli の包みだけにし、Analyze は特別に扱わない; 既にある U2 の仕組みと二重にならず、どの誤りが返るかが1か所で決まるため。
- 2026-10-24T06:40:31Z [unit u4-plan] — 一時ファイルは書く直前に作り、確かめは作ってすぐ消す試しで行う; 2回目の Ctrl+C で残る時間を書いている数秒だけにするため。
- 2026-10-24T00:29:16Z [unit u3-rules] — 文字コードの見分けと大きさの判定を Parse の先頭にまとめた; ファジングの対象と一致させ、すべての経路に届かせるため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-31T13:48:02Z [unit u8-iac] — レビュー3回目の Minor(JSON で末尾の要素を続けて消すと Edit が重なり unreadableFile になる)は、復旧用のレビューのあとは設計を変えないため、Code Generation の計画で「続けて消す要素を1つの範囲にまとめる」規則とテストとして扱う。yaml/v3 と hcl/v2 の解析の速さとメモリは Code Generation の最初に測る。
- 2026-10-30T07:11:19Z [unit u7-rollback] — U6 のテストの補助関数が動いた項目を参照していれば、補助関数だけを直す(期待は変えない)。コード生成で確かめる。
- 2026-10-29T02:10:00Z [unit u6-apply] — 一度もタグのないリソースが GetResources の答えに出るかを、U6 のチェックポイントで確かめる。インフラの設計で、確認用のスタックに EC2 のセキュリティグループとタグのないリソースを入れる。
- 2026-10-25T02:45:58Z [unit u5-impact] — コード生成で、IAM の NoSuchEntity も「一覧のあとで消えた」に入れ、NotFound の追加は操作ごとの表(opErrorCodes)に入れて U1〜U4 の分類を変えない。traceability の NFR1.6 の対応先の見出しは「呼び出しの回数の式」(レビューの Minor)。
- 2026-10-24T06:40:31Z [unit u4-plan] — 東アジアの幅の表は Go の unicode と同じ版(17.0.0)から作る。Go を上げたら作り直す手順をコード生成で決める。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->

