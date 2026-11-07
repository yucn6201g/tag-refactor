# 業務の規則: U7 取り消し

`rollback` の規則です。ID はこの作業単位の中での番号です。ほかの作業単位の規則は「U6 の規則 4.7」のように書きます(その作業単位の `rules.md` の ID から「BR」を省いた番号です)。

出典: 要件 FR6.1〜FR6.5、NFR4、ストーリー US5.1(AC5.1.1〜AC5.1.6)、US6.1 の AC6.1.1(`rollback` のヘルプ)、契約 C4・C5・C10・C12 と全体の決まり(`aws:` で始まるタグ)、`functional-design-questions.md` Q1〜Q9、U6 の規則 3.1・3.2・3.6・3.8・4.1〜4.7・5.1〜5.5。

```yaml
rules:
  # 1. 入力と記録
  - id: BR1.1
    statement: rollback は --run で実行IDを必ず受け取る
    category: validation
    applies_to: RollbackRequest
    trigger: rollback の開始
    logic: IF --run がない、または U6 の runId の形(UTC の秒までと乱数の接尾辞。U6 の規則 5.4)でない THEN 使い方の誤り(InvalidInput)。ほかのオプションは --force、--record-from、--yes、--force-unlock、--profile、--lang(Q1)。--record-from の形は U6 の --record-to と同じに確かめる(U6 の規則 5.2)
    violation: 何もせずに終了コード 1。正しい使い方を示す
    source: [FR6.1, 契約 C5, Q1]
  - id: BR1.2
    statement: 記録は --record-from の場所から読み、なければ手元の既定の場所から読む
    category: constraint
    applies_to: RollbackRequest.recordFrom
    trigger: 印を取ったあと(BR2.1。印を取る前には記録を読まない)
    logic: |
      --record-from がなければ <ホーム>/.tag-refactor/runs/<accountId>/<runId>.json(U6 の規則 5.1)。手元のフォルダーなら <フォルダー>/<accountId>/<runId>.json、s3://<バケット>/<前置き> なら <前置き>/<accountId>/<runId>.json(U6 の規則 5.2)。accountId は今のアカウント(AwsAccess の Resolve)。
      IF 記録がない THEN NotFound で止め、apply で --record-to を指定した場合は同じ値を --record-from に渡すよう案内する(Q2)。IF 記録の version がこのツールの読めるものでない THEN UnsupportedVersion で止める(契約の決まり)。IF 記録の形が壊れている THEN InvalidInput で止める
    violation: 何も戻さずに終了コード 1
    source: [FR6.1, 契約 C4, C10, Q2]
  - id: BR1.3
    statement: 記録のアカウントと実行IDが今の指定と同じで、記録の中身が正しいときだけ進む
    category: validation
    applies_to: RunRecord.accountId、RunRecord.runId、JournalEntry.touchedKeys
    trigger: 記録を読んだあと
    logic: |
      IF 記録の accountId と今のアカウントが違う THEN AccountMismatch で止める(既定の場所は今のアカウントのフォルダーなので、違いが起きるのは --record-from で別の場所を指定したときと、記録を手で動かしたとき)。
      IF 記録の runId が --run と違う THEN InvalidInput で止める(記録を手で動かしたか、名前を変えたとき)。
      IF どれかの項目の touchedKeys に aws: で始まるキー(大文字と小文字を区別しない)がある THEN InvalidInput で止める(apply は aws: のキーを touchedKeys に入れないので、記録が手で変えられている)。
      項目の ARN にアカウントがあれば記録の accountId との一致(S3 のバケットなどアカウントの欄が空の ARN は確かめない)、ARN のリージョン(あれば)と項目の region の一致、touchedKeys が beforeTags と plannedAfterTags のキーに含まれること、キーと値の長さと個数が AWS の上限内であることも確かめ、外れれば InvalidInput で止める(記録の置き場所に書ける別の人が、書き換えの権限を持つ利用者に任意のタグを付けさせないため。詳しくは U7 の NFR の要件 NFR3.15)
    violation: 何も戻さず、違いを示して終了コード 1
    source: [FR6.1, 契約 C4, C12]
  - id: BR1.4
    statement: 判定する項目は、touchedKeys が空でなく、status が skipped でない項目だけにする
    category: constraint
    applies_to: JournalEntry
    trigger: 記録を読んだあと
    logic: 記録の entries のうち、touchedKeys が空でなく、status が skipped でないものを RollbackTarget にする(契約 C4 の注)。status は pending、applied、failed のどれでもよい。記録の completed が false でも同じに扱う(FR6.5、Q3)。IF 判定する項目が1つもない THEN 「取り消すものはない」と示し、記録を変えずに印を消して終える(apply が何も書いていない記録なので、rollback の結果を残す意味がない。判定する項目があって戻すものがない場合は BR4.5)
    violation: 判定する項目がなければ終了コード 0
    source: [FR6.5, 契約 C4, Q3]

  # 2. 印
  - id: BR2.1
    statement: 記録を読む前に、apply と同じ印を取る
    category: constraint
    applies_to: LockInfo
    trigger: 入力とアカウントを確かめたあと、記録を読む前
    logic: rollbackId(BR6.3)を作り、Lock(accountId, 記録の場所, rollback, rollbackId) で手元の既定の場所の印を取り、記録の場所が S3 なら S3 の印も取る(U6 の規則 3.1)。Lock に要るのは今のアカウント、--record-from(なければ既定)の場所、rollbackId だけで、記録の中身は要らない。印を持っている間は、同じアカウントの apply と別の rollback が記録もタグも変えないので、印のあとに読んだ記録をそのまま判定と保存のもとにできる。IF 印がある THEN 印の中身(作った時刻、ホスト名、プロセスID、runId、目的)を示して止め、動いていないことを確かめてから --force-unlock で外すよう案内する(Q3)。途中で止まった apply が残した印も同じに扱い、自動では外さない
    violation: 何も戻さずに終了コード 1(Conflict)
    source: [契約 C10, NFR7, Q3]
  - id: BR2.2
    statement: 残った印は --force-unlock を付けたときだけ外す
    category: authorization
    applies_to: RollbackRequest.forceUnlock
    trigger: --force-unlock のとき、印を取る前
    logic: U6 の規則 3.2 と同じ。外した印の中身を標準エラーに示してから、印を取り直す
    violation: なし
    source: [NFR7, Q1, Q3]
  - id: BR2.3
    statement: 印を取ったあとは、どの終わり方でも印を消してから終える
    category: constraint
    applies_to: LockHandle
    trigger: 印を取ったあとのすべての終わり方(確認で yes 以外、戻すものがない、止められた、誤り、パニック)
    logic: U6 の規則 3.8 と同じ。消す前に印の runId が自分の rollbackId であることを確かめ、他人の印は消さない
    violation: 印を消せなければ、場所と --force-unlock の案内を示して終了コード 1
    source: [契約 C10, NFR7]

  # 3. 判定
  - id: BR3.1
    statement: 判定の前に、判定する項目の今のタグを ARN を指定して読む
    category: validation
    applies_to: RollbackTarget.currentTags
    trigger: 印を取ったあと
    logic: 項目の region ごとに、ARN を指定して今のタグを読む(U6 の ARN を指定した読み取り)。IF 答えに出ない THEN verdict を gone にする。IF 答えに出ない かつ 記録の plannedAfterTags か beforeTags の aws: を除くキーが空(apply が最後のタグまで消した項目か、タグが1つもなかったリソースに apply で付けた項目) THEN gone とせず、今のタグを空として判定する(タグの API が、タグが1つもないリソースを返さない場合に、戻すべき項目を飛ばしたり、すでに戻った項目を「なくなった」にしたりしないため。beforeTags が空なら alreadyRestored になりうる。返すかどうかは検証用アカウントで確かめる)。IF そのリージョンが読めない THEN そのリージョンの項目の verdict を unknown にし、failed(readFailed)にする
    violation: 読めなかった項目は戻さず、終了コードは 1
    source: [FR6.3, 契約 C4]
  - id: BR3.2
    statement: 項目ごとに、touchedKeys のキーだけを今のタグと比べて判定する
    category: calculation
    applies_to: RollbackTarget.verdict
    trigger: 今のタグを読んだあと(読み直しのたびにも。BR5.6)
    logic: |
      touchedKeys のキー k ごとに、今の値 cur(k)(なければ「ない」)を、beforeTags の値 b(k) と plannedAfterTags の値 p(k)(どちらも、なければ「ない」)と比べる。
      IF すべての k で cur(k) = b(k) THEN alreadyRestored(何もしない)。
      ELSE IF すべての k で cur(k) = b(k) か cur(k) = p(k) THEN restore(この apply が書いた。中間の状態を含む)。
      ELSE changedSinceApply(どれかのキーが b とも p とも違う。後から変えられた)。
      比べるのは touchedKeys だけで、ほかのキーと aws: のキーは比べない(契約 C4)
    violation: なし
    source: [FR6.1, FR6.3, 契約 C4]
  - id: BR3.3
    statement: 後から変えられた項目は、既定では戻さずに飛ばし、--force のときだけ戻す
    category: policy
    applies_to: RollbackTarget
    trigger: verdict が changedSinceApply
    logic: IF --force THEN restore と同じに戻す(touchedKeys を beforeTags に戻す。ほかのキーは触らない)。ELSE status を skipped(changedSinceApply)にし、一覧に示す
    violation: 飛ばしがあれば終了コード 2(BR7.2)
    source: [FR6.3, AC5.1.3, AC5.1.4, Q4]
  - id: BR3.4
    statement: 戻すときに付けるタグと消すキーを、touchedKeys と beforeTags から求める
    category: calculation
    applies_to: RollbackTarget.tagsToSet、RollbackTarget.keysToRemove
    trigger: restore、または --force の changedSinceApply
    logic: touchedKeys のキー k ごとに、IF b(k) がある かつ cur(k) ≠ b(k) THEN tagsToSet[k] = b(k)。IF b(k) がない かつ cur(k) がある THEN keysToRemove に k を足す。aws: のキーは touchedKeys に含まれないので、書きも消しもしない(全体の決まり)。戻したあとのタグは、今のタグの touchedKeys だけを beforeTags に合わせたもの
    violation: なし
    source: [FR6.1, NFR4, 契約 C4]
  - id: BR3.5
    statement: 戻すとタグが50を超える項目は戻さず、途中だけ50を超える項目は消すのを先にする
    category: validation
    applies_to: RollbackTarget.removeFirst
    trigger: tagsToSet と keysToRemove を求めたあと
    logic: aws: を除いて数える。IF 戻したあとのタグが50を超える(ほかの人が足したキーがある場合など) THEN skipped(tagLimitExceeded)。ELSE IF 付けたあと消す前の数が50を超える THEN removeFirst を true にする(U6 の規則 3.6 と同じ。Q5)
    violation: 50を超える項目は戻さず、終了コード 2
    source: [Q5, U6 の規則 3.6]
  - id: BR3.6
    statement: なくなったリソースは飛ばす
    category: policy
    applies_to: RollbackTarget
    trigger: verdict が gone
    logic: status を skipped(resourceNotFound)にし、一覧に示す。--force でも戻さない
    violation: 飛ばしがあれば終了コード 2
    source: [FR6.3, FR6.4]

  # 4. 確認
  - id: BR4.1
    statement: 確認画面は、戻す前に、戻す件数と後から変えられた件数などを示す
    category: policy
    applies_to: RollbackSummary
    trigger: 書き戻しの前(--yes のときも要約は標準エラーに出す)
    logic: アカウント、実行ID、記録の場所、apply が途中で止まった記録か(completed が false なら「途中で止まった apply の記録。書き換えた分だけを戻す」)、戻す件数、後から変えられた件数(--force なら「上書きして戻す」、なければ「飛ばす」)、すでに戻っている件数、なくなった件数、50を超えて戻せない件数、読めなかった件数を示す。IF 記録の rollbacks に completed が true のものがある THEN 「この記録はすでに取り消し済み(その日時)。そのあとに同じ変更を apply していると、それも戻す」と警告する(判定は今のタグだけで行うため、同じ値は区別できない)。言語と色と3桁区切りは U4 の表示の部品を使う(U4 の規則 3.7〜3.9)
    violation: なし
    source: [FR6.2, AC5.1.2, AC6.1.2, AC6.1.3]
  - id: BR4.2
    statement: yes と入力したときだけ戻す
    category: authorization
    applies_to: RollbackRequest
    trigger: --yes がないとき
    logic: U6 の規則 2.2 と同じ。前後の空白を除いて yes(小文字)のときだけ進む。入力の終わりと、入力待ちの Ctrl+C は yes 以外として扱う
    violation: yes 以外なら、何も戻さず、記録も変えずに印を消して終了コード 0
    source: [FR6.2, AC5.1.2]
  - id: BR4.3
    statement: 標準入力が端末でなく --yes もなければ、何も戻さずに止める
    category: authorization
    applies_to: RollbackRequest.interactive
    trigger: 判定のあと、書き戻すものがあり確認が要ると分かったとき(BR4.5 で確認が要らない場合は確かめない)
    logic: U6 の規則 2.3 と同じ。--yes を案内し、記録を変えずに印を消す
    violation: 終了コード 1
    source: [FR6.2, Q4]
  - id: BR4.4
    statement: --yes のときは確認なしで戻す。後から変えられた項目は --force もあるときだけ上書きする
    category: authorization
    applies_to: RollbackRequest.yes、RollbackRequest.force
    trigger: --yes のとき
    logic: IF --yes だけ THEN 要約を示し、changedSinceApply の項目は飛ばして残りを戻す。IF --yes と --force THEN 要約を示し、changedSinceApply の項目も上書きして戻す(Q4)
    violation: --yes だけで飛ばしがあれば終了コード 2
    source: [FR6.3, Q4]
  - id: BR4.5
    statement: 書き戻すものが1つもなければ、確認を求めずに結果を記録して終える
    category: policy
    applies_to: RollbackSummary
    trigger: 判定のあと
    logic: IF restore の項目がなく、--force の changedSinceApply の項目もない THEN 確認を求めず(端末でなくても止めない)、項目ごとの結果(alreadyRestored、skipped、failed)を BR6.1 のとおり記録に足して(BeginRollback、RecordRollbackResults、CompleteRollback を続けて行い、completed は true)、結果を示して終える。BR1.4(判定する項目がない)と違って記録に残すのは、判定した結果(2回目の rollback で戻っていたこと、後から変えられていたこと)に意味があるため
    violation: 終了コードは BR7.2 のとおり(すべて alreadyRestored なら 0)
    source: [FR6.4, 契約 C4(二重の rollback)]

  # 5. 書き戻し
  - id: BR5.1
    statement: 1つの項目では、付けるタグを先に、消すキーを後に書く
    category: constraint
    applies_to: RollbackTarget
    trigger: 書き戻し
    logic: IF removeFirst THEN 消すキーを先に書く。ELSE 付けるタグを先に書く(U6 の規則 4.1 と同じ。Q5)。tagsToSet か keysToRemove が空なら、その呼び出しはしない
    violation: なし
    source: [Q5, 契約 C4]
  - id: BR5.2
    statement: 一部の項目が失敗しても、ほかの項目を続けて戻す
    category: policy
    applies_to: RollbackTarget.status
    trigger: 書き込みが失敗したとき
    logic: U6 の規則 4.2 と同じ分け方で、何も書いていない失敗は failed(writeFailed)、片方だけ書いた失敗は failed(partialWrite)、結果が分からない呼び出し(時間切れ、接続の切断、2回目の Ctrl+C で取り消した呼び出し)は failed(resultUnknown)にする。書き込みが NotFound(リソースがなくなった)なら skipped(resourceNotFound)にする。どれも同じ --run でもう一度 rollback すれば、BR3.2 の判定で正しく扱える(中間の状態は restore、戻っていれば alreadyRestored)。認証情報の誤り(期限切れを含む)と、一時的な失敗で終わった書き込みの呼び出しが10回続いたとき(U6 の規則 4.2 と同じ回数)は、新しい書き込みを止める(BR5.5)
    violation: 失敗があれば終了コード 1
    source: [FR6.4, AC5.1.5, U6 の規則 4.2]
  - id: BR5.3
    statement: 書いたあとに今のタグを読み戻し、touchedKeys が beforeTags と一致したかを確かめる
    category: policy
    applies_to: RollbackTarget.confirmed
    trigger: 1つのまとまりを書いたあと
    logic: 読み戻しの答えに出ない項目は、BR3.1 と同じ条件(beforeTags か plannedAfterTags の aws: を除くキーが空)なら今のタグを空として比べ、それ以外なら読み戻せなかったものとする。IF 読み戻せて、すべての touchedKeys で今の値が beforeTags と同じ THEN restored(confirmed true)。IF 読み戻せて違う THEN 上限つきで読み直し(回数と間隔は NFR の設計で決める。タグの API の反映の遅れのため)、最後まで違えば failed(notRestored)にし、同じ --run でもう一度 rollback するよう案内する(反映の遅れだけなら、もう一度の rollback で alreadyRestored になる)。IF 読み戻せない THEN restored(confirmed false。記録の reason は notConfirmed)とし、確かめられなかった件数を示す(U6 の規則 4.3 と同じ扱い。Q6)
    violation: notRestored があれば終了コード 1
    source: [NFR4, Q6]
  - id: BR5.4
    statement: 結果は途中でも記録に保存し、保存できなくなったら新しい書き込みを止める
    category: constraint
    applies_to: RollbackRun
    trigger: 書き戻しの間
    logic: 確認のあと、最初の書き込みの前に、BeginRollback で rollbacks に completed false の RollbackRun を足して保存する(BR6.1。保存できなければ何も書かずに止める)。書いている間は、終わった項目の結果を RecordRollbackResults で U6 と同じ頻度で保存する(U6 の規則 4.4。頻度は NFR の設計で決める)。IF 保存に失敗した THEN 新しい書き込みを止め、BR5.5 と同じに終える
    violation: 終了コード 1
    source: [NFR6, 契約 C4, C10]
  - id: BR5.5
    statement: Ctrl+C などで止められたときは、apply と同じ2段の止め方で終え、結果を残す
    category: policy
    applies_to: RollbackOutcome.stopped
    trigger: 止める合図(Ctrl+C)、認証情報の誤り、一時的な失敗が続いたとき、記録の保存の失敗
    logic: U6 の規則 4.5 と同じ。1回目は新しい書き込みを始めず、動いているまとまりを終えてから止める。2回目は動いている呼び出しを取り消し、その項目は failed(resultUnknown)にする。まだ戻していない項目は結果に入れない。rollbacks の RollbackRun は completed false のまま保存し、印を消す(Q8)。同じ --run でもう一度 rollback すれば、戻した項目は alreadyRestored になり、残りを戻せる
    violation: 終了コード 1。戻した件数と、もう一度 rollback するコマンドを示す
    source: [FR6.5, Q8, U6 の規則 4.5]
  - id: BR5.6
    statement: 書き込みのまとまりごとに今のタグを読み直し、判定し直す
    category: validation
    applies_to: RollbackTarget.verdict
    trigger: 1つのまとまりを書く直前
    logic: まとまりの項目の今のタグを読み直し、BR3.2 で判定し直す。IF alreadyRestored になった THEN 書かずに alreadyRestored。IF changedSinceApply になり --force でない THEN 書かずに skipped(changedSinceApply)。IF gone になった THEN skipped(resourceNotFound)。ELSE tagsToSet と keysToRemove を求め直して(BR3.4、BR3.5)書く(U6 の規則 4.7 と同じ。Q6)
    violation: なし
    source: [FR6.3, Q6, U6 の規則 4.7]

  # 6. 記録
  - id: BR6.1
    statement: rollback の結果は、取り消した apply の記録の rollbacks に足す
    category: constraint
    applies_to: RollbackRun
    trigger: BR5.4、BR4.5、終わるとき
    logic: |
      契約 C10 の3つの呼び出しで、同じ記録(手元のファイルか S3)の rollbacks を変える。BeginRollback は rollbackId、startedAt、completed false、空の results の RollbackRun を足す。RecordRollbackResults は、その rollbackId の RollbackRun の results に、判定が済んだ項目の結果(restored、skipped、failed、alreadyRestored)を ARN を鍵に足すか置き換える。CompleteRollback は completed を true にする。pending は書かない。
      どの呼び出しも、印を取ったあとに読んだ記録(BR1.2)を、このツールが知らない項目も含めてそのまま持ち、自分の rollbackId の RollbackRun だけを変えて書き戻す。印を持っている間は、同じアカウントの apply と別の rollback は記録を書かないので、保存のたびに読み直さない(--force-unlock で他人の印を外して同時に動かした場合は、どちらかの結果が失われうる。U6 と同じ限界として README に書く)。apply の entries と completed、ほかの RollbackRun、このツールが知らない項目はそのまま残す(新しい版のツールが足した項目を消さないため)。保存は U6 と同じく途中で壊れない方法で行う(U6 の規則 5.3)
    violation: 保存に失敗したら終了コード 1(BR5.4)
    source: [契約 C4, C10, NFR6]
  - id: BR6.2
    statement: 最後まで終えたときだけ、RollbackRun を completed にする
    category: constraint
    applies_to: RollbackRun.completed
    trigger: すべての項目を判定し終えたとき
    logic: IF 止められずに最後まで終えた THEN CompleteRollback で completed を true にして保存し、印を消す。ELSE false のまま(BR5.5)
    violation: なし
    source: [契約 C4, FR6.5]
  - id: BR6.3
    statement: rollbackId は runId と同じ形で作る
    category: calculation
    applies_to: RollbackRun.rollbackId
    trigger: 印を取る前
    logic: U6 の規則 5.4 と同じ形(UTC の秒までと乱数の接尾辞)で作る。印の中身の runId にも rollbackId を書く
    violation: なし
    source: [契約 C4, C10]
  - id: BR6.4
    statement: 飛ばしと失敗の詳細は、手元のログファイルにも残す
    category: policy
    applies_to: RollbackOutcome.logFile
    trigger: 飛ばしか失敗が1つでもあるとき
    logic: U6 の規則 5.5 と同じ場所と形で、<rollbackId>.log に項目ごとに1行(ARN、状態、理由、誤りの種類、操作)を書く。タグの値と AWS のエラー文は書かない
    violation: なし
    source: [FR6.4]

  # 7. 結果
  - id: BR7.1
    statement: 結果として、戻した・飛ばした・失敗した件数と、一覧を示す
    category: policy
    applies_to: RollbackOutcome
    trigger: 終わるとき
    logic: 戻した件数(読み戻しで確かめられなかった件数を添える)、飛ばした件数、失敗した件数、すでに戻っていた件数を示す。飛ばした項目と失敗した項目の一覧を理由つきで示し(後から変えられた、なくなった、50を超える、読めなかった、書けなかった、一部だけ書いた、結果が分からない、戻らなかった)、ログファイルの場所を示す。IF 後から変えられて飛ばした項目がある THEN 上書きして戻すコマンド(--force を付けたもの)を示す。IF 止められた、または notRestored がある THEN もう一度 rollback するコマンドを示す。どちらのコマンドにも、--record-from と --profile を指定した場合はそれも含める(U6 の規則 6.3 の取り消しのコマンドと同じ形。Q9)
    violation: なし
    source: [FR6.4, AC5.1.5, Q9]
  - id: BR7.2
    statement: rollback の終了コードを結果から決める
    category: calculation
    applies_to: RollbackOutcome
    trigger: 終わるとき
    logic: |
      IF 失敗がある、止められた、記録の保存の失敗、アカウントの不一致、印があった、記録がない・読めない、印を消せなかった THEN 1。
      ELSE IF 飛ばした項目がある(後から変えられた、なくなった、50を超える) THEN 2。
      ELSE 0(すべて restored か alreadyRestored。判定する項目がない、確認で yes 以外のときも 0)。1 と 2 の両方にあたるときは 1(Q7)
    violation: なし
    source: [契約 C5, Q7]
  - id: BR7.3
    statement: rollback のヘルプに、オプションと終了コードと確認の動きを示す
    category: policy
    applies_to: Cli
    trigger: rollback --help
    logic: BR1.1 のオプション、記録の既定の場所と --record-from、後から変えられたリソースと --force の関係、--yes の動き、残った印と --force-unlock、終了コード 0・1・2 の意味を示す。U1〜U6 のヘルプの形に合わせる
    violation: なし
    source: [契約 C5, U4 からの引き継ぎ(AC6.1.1 の rollback の分)]
  - id: BR7.4
    statement: apply が示す取り消しのコマンドは、そのまま rollback で使える
    category: policy
    applies_to: Cli(apply)
    trigger: apply の結果を示すとき
    logic: U6 の規則 6.3 のとおり、apply は tag-refactor rollback --run <runId> を示し、--record-to を指定した場合は --record-from <同じ値>、--profile を指定した場合は --profile <同じ値> を含める(U6 で実装済み。U7 では変えない)。U7 は、このコマンドのオプション(--run、--record-from、--profile)をすべて受け付け、示されたコマンドで記録を見つけられることを、受け入れ条件のテストで確かめる(Q2)
    violation: なし
    source: [FR5.11, Q2, U6 の規則 6.3]
```

## 規則の一覧

| ID | 内容 | 種類 | 出典 |
|---|---|---|---|
| BR1.1 | `--run` は必須で、runId の形を確かめる | validation | FR6.1、Q1 |
| BR1.2 | 記録は `--record-from` か既定の場所から読む | constraint | FR6.1、Q2 |
| BR1.3 | 記録のアカウントと実行IDの一致、記録の中身の確かめ(`aws:`、ARN、キー、上限) | validation | FR6.1、C4 |
| BR1.4 | 判定するのは `touchedKeys` が空でなく `skipped` でない項目(未完了の記録も同じ) | constraint | FR6.5、C4、Q3 |
| BR2.1 | 記録を読む前に apply と同じ印を取る(残った印は自動では外さない) | constraint | C10、NFR7、Q3 |
| BR2.2 | 残った印は `--force-unlock` で人が外す | authorization | NFR7、Q1 |
| BR2.3 | どの終わり方でも印を消す | constraint | C10、NFR7 |
| BR3.1 | 今のタグを ARN を指定して読む | validation | FR6.3 |
| BR3.2 | `touchedKeys` だけを比べて判定する | calculation | FR6.1、FR6.3、C4 |
| BR3.3 | 後から変えられた項目は飛ばし、`--force` なら戻す | policy | FR6.3、Q4 |
| BR3.4 | 付けるタグと消すキーの求め方(`aws:` は触らない) | calculation | FR6.1、NFR4 |
| BR3.5 | 50を超える項目は戻さない(途中だけなら消すのが先) | validation | Q5 |
| BR3.6 | なくなったリソースは飛ばす | policy | FR6.3、FR6.4 |
| BR4.1 | 確認画面の中身 | policy | FR6.2 |
| BR4.2 | `yes` のときだけ戻す | authorization | FR6.2 |
| BR4.3 | 端末でなく `--yes` もなければ止める | authorization | FR6.2、Q4 |
| BR4.4 | `--yes` と `--force` の組み合わせ | authorization | FR6.3、Q4 |
| BR4.5 | 書き戻すものがなければ確認なしで結果を記録する | policy | FR6.4、C4 |
| BR5.1 | 付けるのが先、消すのが後(例外あり) | constraint | Q5 |
| BR5.2 | 一部が失敗しても続ける | policy | FR6.4 |
| BR5.3 | 読み戻して確かめる | policy | NFR4、Q6 |
| BR5.4 | 途中でも保存し、保存できなければ止める | constraint | NFR6、C4 |
| BR5.5 | 止められたときの終え方 | policy | FR6.5、Q8 |
| BR5.6 | まとまりごとに読み直して判定し直す | validation | FR6.3、Q6 |
| BR6.1 | 結果は C10 の3つの呼び出しで apply の記録の `rollbacks` に足す(印のあとに読んだ記録を知らない項目ごと持ち、ほかは変えない) | constraint | C4、C10 |
| BR6.2 | 最後まで終えたときだけ completed | constraint | C4、FR6.5 |
| BR6.3 | rollbackId の形 | calculation | C4 |
| BR6.4 | 飛ばしと失敗のログファイル | policy | FR6.4 |
| BR7.1 | 結果の件数と一覧、上書きのコマンド | policy | FR6.4、Q9 |
| BR7.2 | 終了コード | calculation | C5、Q7 |
| BR7.3 | rollback のヘルプ | policy | C5、U4 からの引き継ぎ |
| BR7.4 | apply が示す取り消しのコマンド(U6 で実装済み)で rollback できる | policy | FR5.11、Q2、U6 の規則 6.3 |
