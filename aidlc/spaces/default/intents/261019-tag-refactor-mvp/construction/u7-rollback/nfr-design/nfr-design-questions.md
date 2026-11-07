# NFR の設計 — 質問(U7 取り消し)

NFR の要件(`construction/u7-rollback/nfr-requirements/`)を、部品の分け方と、記録の扱い方に落とします。

決まっていること(質問しません):

- 権限の例は U6 の `docs/iam/apply-policy.json` をそのまま使う(NFR3.13)。新しい操作は足さない。
- 書き戻しの進め方: 読み直しの単位(ARN の順に100個)を先に切り、読み直しのあとに単位の中で同じ組み合わせを20個までまとめる。リージョンは同時に4つまで、リージョンの中は順(NFR5.19)。apply のまとまりの作り方は変えない。
- 読み戻しの読み直しは1・2・4秒で最大3回(NFR5.24)。記録の保存は5秒ごと、重ねない、3回続けて失敗で止める(NFR6.4)。止める条件は U6 と同じ(NFR5.23)。
- 記録は印を取ったあとに1回読み、知らない項目を生の JSON のまま持って書き戻す(NFR6.3)。記録の中身を書き込みの前に確かめる(NFR3.15)。
- 偽の AWS に、タグが0個のリソースを答えに出す・出さないの切り替えを足す(NFR4.2 (c))。

## Q1. 部品の分け方(パッケージ)

rollback の流れ(判定、書き戻し、読み戻し、結果の記録)を、どのパッケージに置きますか?U6 の `internal/apply` には、リージョンの役(同時4つまで、パニックを止めに変える)、止める役(2段の止め方、一時的な失敗の数え方)、1つのまとまりの書き方(付けるのが先、途中だけ50を超えるなら消すのが先、失敗の分け方)があります。

A. `internal/apply` の中に rollback のファイルを足す(パッケージの説明を「apply と rollback のタグの書き込み」に改める)。リージョンの役、止める役、1つのまとまりの書き方は、apply と rollback が同じパッケージの中で使う。apply のまとまりと単位の作り方(`planUnits`)と判定(`decide`)は変えず、rollback は自分の単位の切り方と判定を持つ。公開する口は `Applier` の隣に `Rollbacker` を足すだけ
B. 新しいパッケージ `internal/rollback` を作り、U6 の部品のうち使うもの(リージョンの役、止める役、まとまりの書き方)を `internal/apply` から公開して使う
C. 共通の部品を新しいパッケージ(例: `internal/tagwrite`)に移し、`internal/apply` と新しい `internal/rollback` の両方がそれを使う(U6 のコードを動かす)
X. Other (please specify)

[Answer]: A

## Q2. 記録の読み込みと、rollback の結果の書き戻しの作り

rollback は、apply の記録を読み、知らない項目を残したまま `rollbacks` に結果を足し、5秒ごとに保存します。`internal/journal` をどう広げますか?

A. `Journal.OpenForRollback(LockHandle, runId, location)` で記録を読み、`RollbackRun`(契約 C10 の `BeginRollback`・`RecordRollbackResults`・`CompleteRollback` と、U6 の5秒ごとの保存と同じ保存の役)を返す。記録の型は U6 のまま使い、知らない項目は、最上位と各項目と各 RollbackRun の生の JSON の写しを持って、書き戻すときに混ぜ直す。壊れない保存(一時ファイルと名前の変更、S3 の上書き)は U6 の部品を使う
B. A と同じだが、知らない項目は最上位だけ残し、項目の中の知らない項目は残さない(作りが簡単になる)
C. 記録を型を使わずに汎用の JSON(map)として読み書きし、`rollbacks` だけを型で扱う
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答をまとめると、U7 の NFR の設計は次の内容になります。

1. **部品の分け方(Q1 A):** `internal/apply` の中に rollback のファイルを足し、パッケージの説明を「apply と rollback のタグの書き込み」に改めます。リージョンの役、止める役、1つのまとまりの書き方は、apply と rollback が同じパッケージの中で使います。apply の単位とまとまりの作り方と判定は変えず、rollback は自分の単位の切り方と判定を持ちます。公開する口は `Applier` の隣に `Rollbacker` を足すだけです。
2. **記録の扱い(Q2 A):** `internal/journal` に `OpenForRollback(LockHandle, runId, location)` を足し、契約 C10 の3つの呼び出しと5秒ごとの保存の役を持つ `RollbackRun` を返します。記録の型は U6 のまま使い、知らない項目は最上位・各項目・各 RollbackRun の生の JSON の写しを持って、書き戻すときに混ぜ直します。壊れない保存は U6 の部品を使います。

Does this all look correct before I generate the artifact?

[Answer]: Looks correct
