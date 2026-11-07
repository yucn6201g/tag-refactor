<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-31T03:44:46Z [unit u8-iac] — iac は AWS を呼ばないので、AWS の呼び出しの要件(NFR5 の時間の上限・再試行、NFR7 の印)は当てはまらず、要件の中心を利用者のファイルとリポジトリ(上限、git、壊れない書き出し、差分に出る中身)に置くと読んだ。
- 2026-10-30T06:56:47Z [unit u7-rollback] — rollback は U6 の部品と権限の例をそのまま使うが、読み直しの単位を先に切ってから単位の中でまとめる順にした; 書き戻す内容が今のタグから決まるため。
- 2026-10-29T01:45:00Z [unit u6-apply] — U6 は library の作業単位で性能の要件のファイルを作らないため、書き込みの速さ・移植性・テストの要件を tech-stack-decisions.md に、記録と印の信頼性をセキュリティの要件に置いた。
- 2026-10-25T02:30:56Z [unit u5-impact] — 作業単位の種類は library で、性能の要件は必須の成果物ではないが、NFR1(10分)と NFR2(検出率100%)が U5 の中心のため performance-requirements.md も作った。
- 2026-10-24T06:30:19Z [unit u4-plan] — Q3 の「HTML として働かせない」は、データを必ずコードスパンに入れることで満たすと読んだ; コードスパンの中で文字の参照に変えると、コピーした値が元と変わるため。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-31T03:44:46Z [unit u8-iac] — 回答 Q2 A の「YAML の別名の展開は解析器の上限を使う」は根拠がなく、解析木だけを使って別名を展開しない形に変えた。Q4 A の git の呼び方に、fsmonitor の無効化、GIT_ の環境変数の除去、リテラルのパス指定、追跡と ignore の確かめを足した(レビュー1回目の指摘)。unit-of-work.md の配布の項目に MPL-2.0 の義務を書き足した。
- 2026-10-30T06:56:47Z [unit u7-rollback] — レビューの差し戻しで機能設計の BR1.3・BR3.1・BR5.3・BR6.1 と契約 C10 を直した(0個のタグ、記録の中身の確かめ、保存のたびに読み直さない)。
- 2026-10-29T01:45:00Z [unit u6-apply] — 機能設計 BR4.2 の「Transient の失敗が10件」は、NFR5.17 で「書き込みの呼び出しの数(全リージョンで続いた回数、成功で0に戻す)」と定めた。機能設計の文言は直していない。
- 2026-10-25T02:30:56Z [unit u5-impact] — レビューの Major 2件(API ごとのページの件数を無視した呼び出しの回数の式、1つのリージョンの中を順に読むことを反映しない10分の見積もり)は、READY の判定のあと要件の文書を直さず、NFR の設計で API ごとのページの件数の表と、最も多いリージョンを下限にした見積もりの式として決め直す。
- 2026-10-24T06:30:19Z [unit u4-plan] — Windows の標準出力でも仮想端末の処理を U4 で有効にし、終了時に戻すことにした; U2 は標準エラーの進み具合のときだけ有効にしていたため。
- 2026-10-24T00:10:00Z [unit u3-rules] — PowerShell 5.1 のリダイレクトが書く UTF-16 のルールファイルを読めるようにする要件(NFR9.5)を足した; 下書きを Windows でそのまま保存して使う主要な流れを壊さないため。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-31T03:44:46Z [unit u8-iac] — 合計 500 MB の上限いっぱいでのメモリはテストせず、解析木を持ち続けない作りに頼る(限界として README に書く)。60 秒の目標は、Code Generation の最初の小さな計測で見込みを確かめ、30 秒を超えるなら人に相談する。
- 2026-10-30T06:56:47Z [unit u7-rollback] — 記録は印のあとに1回読んでメモリに持ち、S3 の If-Match は付けない; --force-unlock での同時実行で結果が失われうるのは U6 と同じ限界として受け入れた。
- 2026-10-29T01:45:00Z [unit u6-apply] — 値がリソースごとに違うキーの名前を変える場合は、まとまりがほぼ1件になり書き込みが件数の2倍になる。30分の目標の合否には使わず(Q3)、見積もりの記録と README の案内で受け入れた。読み直しと読み戻しは、まとまりを束ねて最大100 ARN の GetResources にして呼び出しを減らした。
- 2026-10-25T02:30:56Z [unit u5-impact] — IAM は同時に4つ、リソースポリシーはリージョンを同時に8つ(1つのリージョンの中は順に)とした; IAM はアカウント全体で呼び出し回数の上限を持ち、リソースポリシーはリージョンごとに上限を持つため、U2 の収集と同じ作りに揃えた。
- 2026-10-24T06:30:19Z [unit u4-plan] — 読み込みの上限を 256 MiB とし、超える変更案は保存しない; 最悪の大きさ(500 MiB 以上)まで受けるとメモリが数 GB になるため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-31T03:44:46Z [unit u8-iac] — goccy/go-yaml のトークンの位置の単位(バイトか文字か)、YAML と HCL の解析の速さ、実物の terraform show -json の例の入手(手元に terraform があるか)は Code Generation の最初に確かめる。
- 2026-10-30T06:56:47Z [unit u7-rollback] — タグが0個のリソースをタグの API が返すかは U7 のチェックポイントで確かめる。返さないと、beforeTags が空の項目は削除されていても alreadyRestored になる。
- 2026-10-29T01:45:00Z [unit u6-apply] — 書き換えの対象の種類の表(種類ごとのタグの操作、Resource の形)と tag:TagResources が Resource を絞れないことを、NFR の設計で Service Authorization Reference と照らして確かめる。
- 2026-10-25T02:30:56Z [unit u5-impact] — 1 MiB の上限は SDK が本体を読んだあとに比べるため、メモリより解析の時間を守るもの。大きすぎるときの理由の文言、NFR2.1 の組み合わせの数、取り消し後の goroutine の確認、病的なワイルドカードの種を NFR の設計で決める(レビューの Minor)。
- 2026-10-24T06:30:19Z [unit u4-plan] — コード生成の計画では、色の条件を機能設計 BR3.7 ではなく NFR11.7(TERM=dumb と Windows の仮想端末の有効化を含む)から取る。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
