# 機能設計の質問: U7 取り消し(u7-rollback)

U7 では `rollback` を作ります。U6 の `apply` が残した記録をもとに、タグを書き換える前の状態に戻します。

## すでに決まっていること(質問しません)

- **ストーリー:** US5.1(書き換えを元に戻す。AC5.1.1〜AC5.1.6)。要件 FR6.1〜FR6.5、NFR4(別の変更が入っていなければ差分0)。US6.1 の AC6.1.1 のうち `rollback` のヘルプ。
- **判定(契約 C4):** 項目ごとに `touchedKeys` のキーだけを、今のタグと比べます(`aws:` のキーは含めません)。すべて `beforeTags` と同じなら `alreadyRestored`、どのキーも `beforeTags` か `plannedAfterTags` と同じ(中間の状態を含む)なら戻す、どれかが両方と違えば後から変えられたとして飛ばします(`--force` なら戻す)。status に関係なく `touchedKeys` が空でない項目を判定し、status が `skipped` の項目は判定しません。
- **結果の記録(契約 C4):** 同じ記録の `rollbacks` に、`rollbackId` と項目ごとの結果(`restored`・`skipped`・`failed`・`alreadyRestored`)を残します。2回目の rollback では、戻した項目は `alreadyRestored` になります。
- **印(契約 C10):** apply と同じ印(`Lock`、目的 `rollback`)を取り、どの終わり方でも外します。同じアカウントで apply か rollback が動いていれば Conflict で止めます。
- **アカウントの確認:** 記録のアカウントと今のアカウントが違えば、何も戻さずに止めます(終了コード 1)。
- **`aws:` のタグ:** 書きも消しもしません(全体の決まり)。
- **書き込みと読み取りの口:** U6 の `TagWriter`(`TagResources`、`UntagResources`)と、ARN を指定した読み取り(`GetTags`)を使います。
- **確認(FR6.2):** 戻す件数と、後から変えられたリソースの件数を示し、`yes` の入力で実行します。

## Q1. rollback のオプション名

モックアップの案は、`--run <実行ID>`(必須)、`--force`(後から変えられたリソースも戻す)、`--record-from <場所>`(記録の置き場所)、`--yes`(確認を省く)、`--profile`、`--lang` です。名前はどうしますか?

A. 案のとおりにし、apply と同じ `--force-unlock`(残った印を外す)も足す
B. 案のとおりにするが、`--force` を `--overwrite-changed` のように、何を上書きするかが分かる名前にする
C. 案のとおりにし、`--force-unlock` は足さない(残った印は apply の `--force-unlock` で外す)
X. Other (please specify)

[Answer]: A

## Q2. 記録の探し方

apply の記録は、既定では `<ホーム>/.tag-refactor/runs/<accountId>/<runId>.json`、`--record-to` を指定したときはその場所(手元のフォルダーか `s3://<バケット>/<前置き>`)にあります。rollback は記録をどう探しますか?

A. `--record-from` がなければ既定の場所を見る。apply で `--record-to` を指定した場合は、同じ値を `--record-from` に渡す(apply の結果の取り消しのコマンドに `--record-from` を含めて示す)
B. A に加えて、既定の場所になければ、前回の apply の `--record-to` を手元に覚えておいて自動で探す
C. `--record-from` を必須にする
X. Other (please specify)

[Answer]: A

## Q3. 途中で止まった apply と残った印

強制終了した apply は、記録の `completed` が false で、`pending` の項目と、手元(と S3)の印が残ります。rollback はどう扱いますか?

A. 記録が未完了でも、残っている範囲で戻す(FR6.5。`pending` の項目も表で判定する)。印が残っていれば Conflict で止め、apply と同じく、動いていないことを確かめてから `--force-unlock` で外すよう案内する
B. A と同じだが、印の中身の runId が取り消す記録の runId と同じなら、印を自動で外して進める
C. 未完了の記録は、確認画面で警告を出し、`yes` でだけ進める(`--yes` では止める)
X. Other (please specify)

[Answer]: A

## Q4. `--yes` と `--force` の組み合わせ

`--yes` は確認を省き、`--force` は後から変えられたリソースも戻します。両方を付けたときはどうしますか?

A. どちらも付けられる。`--yes` だけなら後から変えられたリソースは飛ばし(終了コード 2)、`--yes --force` なら確認なしで上書きして戻す。端末でなく `--yes` もなければ、何も戻さずに止める(終了コード 1。apply と同じ)
B. `--force` は人の確認を必須にし、`--yes` とは一緒に使えない(使い方の誤り)
C. `--yes --force` で後から変えられたリソースがあるときは、もう1つの確かめのオプション(例: `--i-know`)を求める
X. Other (please specify)

[Answer]: A

## Q5. 戻すときの書き込みの順番

戻すには、apply で付けたキーを `beforeTags` の値に戻すか消し、apply で消したキーを付け直します。タグ数の上限は50です。順番はどうしますか?

A. apply と同じく、付ける(`beforeTags` の値に戻す・付け直す)を先、消すを後にする。付けると途中で `aws:` を除いて50を超えるときだけ、消すを先にする
B. 常に消すを先にする(途中で上限を超えないため)
C. 1つの呼び出しで済むように、付ける分と消す分のどちらかしかない項目から先に戻す
X. Other (please specify)

[Answer]: A

## Q6. 戻す前の読み直しと、戻したあとの確かめ

判定のために今のタグを読んでから実際に書くまでに、タグが変えられることがあります。どう扱いますか?

A. apply と同じく、書き込みのまとまりごとに今のタグを読み直して判定し直し、書いたあとに読み戻して、`touchedKeys` が `beforeTags` と一致したかで `restored` を決める
B. 最初に全件を読んで判定するだけにし、読み戻しはしない(書き込みが成功すれば `restored`)
C. A と同じだが、読み戻しで一致しなかったら1回だけ書き直す
X. Other (please specify)

[Answer]: A

## Q7. 終了コード

C5 の決まり(0 問題なし、1 エラー・一部の失敗、2 飛ばしあり)を rollback にどう当てますか?

A. すべて `restored` か `alreadyRestored` なら 0。後から変えられて飛ばした項目があれば 2。失敗、止めた、記録の保存の失敗、アカウントの不一致、印があったなら 1(1 と 2 が両方なら 1)。確認で `yes` 以外なら 0
B. A と同じだが、`alreadyRestored` だけ(何もすることがなかった)なら 2 にして気づけるようにする
C. 飛ばした項目があっても 0 にし、件数の表示だけで知らせる
X. Other (please specify)

[Answer]: A

## Q8. rollback の途中で止められたとき

Ctrl+C や認証情報の期限切れで rollback が途中で止まったときは、どうしますか?

A. apply と同じ2段の止め方にする(1回目は動いているまとまりを終えて止め、2回目は呼び出しを取り消す)。結果は `rollbacks` に `completed: false` で残し、同じ `--run` でもう一度 rollback すれば、戻した項目は `alreadyRestored` になり、残りを戻せる
B. A と同じだが、止めたときの結果は記録に残さない
C. 止められたら、戻した分を apply の状態にやり直す
X. Other (please specify)

[Answer]: A

## Q9. 結果の表示

FR6.4 の3つの件数に加えて、何を示しますか?

A. 戻した・飛ばした・失敗した件数、`alreadyRestored` の件数、飛ばしたリソースと失敗したリソースの一覧(理由つき)、ログファイルの場所、上書きして戻すコマンド(`--force` を付けたもの、飛ばしがあったとき)
B. A から、上書きして戻すコマンドを除く(誤って上書きしないため)
C. 3つの件数だけを示し、一覧はログファイルにだけ残す
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答をまとめると、U7 の機能設計は次の内容になります。

1. **オプション(Q1 A):** `rollback --run <実行ID>`(必須)、`--force`(後から変えられたリソースも戻す)、`--record-from <場所>`、`--yes`、`--force-unlock`(残った印を外す。apply と同じ)、`--profile`、`--lang`。
2. **記録の探し方(Q2 A):** `--record-from` がなければ既定の場所(`<ホーム>/.tag-refactor/runs/<accountId>/<runId>.json`)を見ます。apply で `--record-to` を指定した場合は同じ値を `--record-from` に渡します。apply の結果に示す取り消しのコマンドにも `--record-from` を含めます。
3. **途中で止まった apply(Q3 A):** 記録が未完了でも、残っている範囲で戻します(`pending` の項目も判定の表で決める)。印が残っていれば Conflict で止め、動いていないことを確かめてから `--force-unlock` で外すよう案内します。
4. **`--yes` と `--force`(Q4 A):** 両方を付けられます。`--yes` だけなら後から変えられたリソースは飛ばし(終了コード 2)、`--yes --force` なら確認なしで上書きして戻します。端末でなく `--yes` もなければ、何も戻さずに止めます(終了コード 1)。
5. **書き込みの順番(Q5 A):** 付ける(値を戻す・付け直す)を先、消すを後にします。付けると途中で `aws:` を除いて50を超えるときだけ、消すを先にします。
6. **読み直しと確かめ(Q6 A):** 書き込みのまとまりごとに今のタグを読み直して判定し直し、書いたあとに読み戻して、`touchedKeys` が `beforeTags` と一致したかで `restored` を決めます。
7. **終了コード(Q7 A):** すべて `restored` か `alreadyRestored` なら 0、後から変えられて飛ばした項目があれば 2、失敗・止めた・記録の保存の失敗・アカウントの不一致・印があったなら 1(両方なら 1)。確認で `yes` 以外なら 0。
8. **途中で止められたとき(Q8 A):** apply と同じ2段の止め方にし、結果は `rollbacks` に `completed: false` で残します。同じ `--run` でもう一度 rollback すれば、戻した項目は `alreadyRestored` になり、残りを戻せます。
9. **結果の表示(Q9 A):** 戻した・飛ばした・失敗した件数、`alreadyRestored` の件数、飛ばしたリソースと失敗したリソースの一覧(理由つき)、ログファイルの場所、飛ばしがあったときの上書きのコマンド(`--force` 付き)を示します。

Does this all look correct before I generate the artifact?

[Answer]: Looks correct
