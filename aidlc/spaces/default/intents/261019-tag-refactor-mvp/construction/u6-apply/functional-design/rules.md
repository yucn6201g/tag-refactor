# 業務の規則: U6 書き換えと記録

`apply` と書き換えの記録の規則です。ID はこの作業単位の中での番号です。ほかの作業単位の規則は「U4 の規則 1.9」のように書きます(その作業単位の `rules.md` の ID から「BR」を省いた番号です)。

出典: 要件 FR5.1〜FR5.11、NFR6、NFR7、ストーリー US4.1・US4.2(AC4.1.1〜AC4.1.7、AC4.2.1〜AC4.2.7)、契約 C2・C4・C5・C10・C12 と全体の決まり(`aws:` で始まるタグ)、`functional-design-questions.md` Q1〜Q9、U4 の規則 1.9・4.4。

```yaml
rules:
  # 1. 入力と前の確かめ
  - id: BR1.1
    statement: apply は、保存した変更案(--plan)かルールファイル(--rules)のどちらか1つを必ず受け取る
    category: validation
    applies_to: ApplyRequest
    trigger: apply の開始
    logic: IF --plan と --rules の両方がある、またはどちらもない THEN 使い方の誤り(InvalidInput)。ほかのオプションは --yes、--allow-impact、--record-to、--force-unlock、--profile、--lang(Q1)
    violation: 何もせずに終了コード 1。正しい使い方を示す
    source: [FR5.2, 契約 C5, Q1]
  - id: BR1.2
    statement: 保存した変更案は U4 の部品で読み、変更案のアカウントと今のアカウントが同じときだけ進む
    category: validation
    applies_to: ApplyRequest.planFile、SavedPlan
    trigger: --plan のとき、AWS に書き込む前
    logic: U4 の規則 4.4 で読む(版、形、大きさ)。IF 変更案の accountId と今のアカウント(AwsAccess の Resolve)が違う THEN 止める
    violation: 何も書き換えず、両方のアカウントを示して終了コード 1(契約 C2、C5)
    source: [FR5.2, 契約 C2]
  - id: BR1.3
    statement: --rules のときは、plan と同じ手順で変更案をその場で作り、保存した変更案と同じ流れで書き換える
    category: constraint
    applies_to: ApplyRequest.ruleFile
    trigger: --rules のとき
    logic: 収集(U2)、ルール(U3)、変更案と影響分析(U4、U5)を plan と同じに行い、できた変更案(planId を持つ)を BR1.4 以降に渡す。ファイルには保存しない
    violation: 変更案を作れなければ plan と同じ誤りで終わる
    source: [FR5.1, 契約 C5, Q1]
  - id: BR1.4
    statement: 変更が1つもない変更案では、印も記録も作らずに終える。衝突や値が未指定の必須タグがあれば、その件数を示す
    category: policy
    applies_to: Plan
    trigger: 変更案を読んだ、または作ったあと
    logic: IF どのリソースにも changes がない THEN 「書き換えるものはない」と示し、変更案の衝突(keyConflicts)と値が未指定の必須タグの件数があれば、それも示して終える(確認も求めない)
    violation: 衝突か未指定があれば終了コード 2(plan と同じ)、なければ 0(BR6.2)
    source: [FR5.1, 契約 C5]
  - id: BR1.5
    statement: 収集や影響分析が不完全な変更案でも書き換えは許し、影響分析が不完全なら影響のある変更があるものとして扱う
    category: policy
    applies_to: Plan.inventoryComplete、Plan.impactAnalysisComplete
    trigger: 確認の前
    logic: IF inventoryComplete が false THEN 確認画面で「収集が不完全(書き換えない種類やリージョンがある)」と示す。IF impactAnalysisComplete が false THEN 確認画面で示し、BR2.5 の判定では影響のある変更があるものとする。作成からの時間では止めない(BR3.3 で守る)
    violation: なし
    source: [FR5.4, FR5.5, Q3]

  # 2. 確認
  - id: BR2.1
    statement: 確認画面は、書き換える前に、変更の要約、影響のある変更、不完全の印、IaC で管理されているリソース、記録の場所を示す
    category: policy
    applies_to: ApplySummary
    trigger: 書き換えの前(--yes のときも要約は標準エラーに出す)
    logic: アカウント、リージョンの数、変更案の ID と作成日時(--plan のとき)、変更のあるリソースの数、種類ごとの変更の数、影響のある変更(affected は「!」と「影響あり」、undecided は「?」と「判定できない」の記号と文字)、BR1.5 の不完全の印、IaC で管理されているリソースの数と「IaC の定義とずれ、次のデプロイで戻る可能性がある」警告(Q9)、変更案の衝突と値が未指定の必須タグの件数(書き換えないもの)、記録の場所を示す。言語(--lang、なければ OS の設定)、色(端末のときだけ。NO_COLOR とリダイレクトでは付けない)、件数の3桁区切りは U4 の表示の部品をそのまま使う(U4 の規則 3.7〜3.9)。色がなくても記号と文字で意味が分かる
    violation: なし
    source: [FR5.1, FR5.4, Q9, AC4.1.4, AC6.1.2, AC6.1.3]
  - id: BR2.2
    statement: 確認では、利用者が yes と入力したときだけ書き換える
    category: authorization
    applies_to: ApplyRequest
    trigger: --yes がなく、標準入力が端末のとき
    logic: 「yes と入力すると書き換えます」と示して1行読む。IF 前後の空白を除いて yes(小文字)と同じ THEN 書き換える。ELSE 何も書き換えずに終える。言語が日本語でも入力は yes。入力を待っている間の Ctrl+C と入力の終わり(EOF)は yes 以外として扱う(待ちを取り消せるように読む)
    violation: yes 以外なら何も書き換えず、記録も作らずに終了コード 0(契約 C5)
    source: [FR5.1, AC4.1.1, AC4.1.4]
  - id: BR2.3
    statement: 標準入力が端末でなく --yes もないときは、確認を求めずに止める
    category: authorization
    applies_to: ApplyRequest.interactive
    trigger: 確認の前
    logic: IF 標準入力が端末でない AND --yes がない THEN 止めて、スクリプトでは --yes を使うよう案内する(パイプで yes を流しても確認は通らない)
    violation: 何も書き換えずに終了コード 1
    source: [FR5.3, Q8]
  - id: BR2.4
    statement: --yes のときは、影響のある変更がなければ確認の入力なしで書き換える
    category: authorization
    applies_to: ApplyRequest.yes
    trigger: --yes のとき
    logic: IF 影響のある変更がない(BR2.5 の判定) THEN 要約を標準エラーに出して、入力を待たずに書き換える
    violation: なし
    source: [FR5.3, AC4.1.3]
  - id: BR2.5
    statement: --yes だけのときに影響のある変更があれば、何も書き換えずに止める
    category: authorization
    applies_to: ApplyRequest.yes、ApplyRequest.allowImpact
    trigger: --yes のとき
    logic: 影響のある変更 = changes のどれかが impacted(affected か undecided)、または impactAnalysisComplete が false。IF 影響のある変更がある AND --allow-impact がない THEN 止めて、影響のある変更と、--allow-impact で進められることを示す
    violation: 何も書き換えず、記録も作らずに終了コード 2(契約 C5)
    source: [FR5.5, AC4.1.5, Q3]
  - id: BR2.6
    statement: --yes と --allow-impact の両方があるときだけ、影響のある変更も確認なしで書き換える
    category: authorization
    applies_to: ApplyRequest.allowImpact
    trigger: --yes のとき
    logic: IF --yes AND --allow-impact THEN 影響のある変更を要約に示したうえで、入力を待たずに書き換える
    violation: なし
    source: [FR5.5, AC4.1.6]
  - id: BR2.7
    statement: --yes なしの --allow-impact は何も変えず、案内を出す
    category: policy
    applies_to: ApplyRequest.allowImpact
    trigger: --allow-impact があり --yes がないとき
    logic: 「--allow-impact は --yes と一緒のときだけ意味がある。確認画面で判断してください」と標準エラーに出し、BR2.2 の確認に進む
    violation: なし
    source: [契約 C5]

  # 3. 書き換えの準備
  - id: BR3.1
    statement: apply は、確認の前に、同じアカウントの印を取る。印があれば止める
    category: constraint
    applies_to: LockInfo、RecordLocation
    trigger: BR1.4 のあと、確認の前
    logic: 印を取る前に runId を作る(BR5.4)。手元の印は、--record-to に関係なく、常に <ホーム>/.tag-refactor/runs/<accountId>/.lock に「なければ作る」方法で作る。S3 に記録するときは、手元の印を取ったあとで、S3 の印(<prefix>/<accountId>/.lock)も「同じ名前がなければ作る」条件付きの書き込み(契約 C6 の ObjectStore.PutIfAbsent)で作る(Q5)。S3 の印を取れなければ、手元の印を消してから止める。どちらかの印がすでにあれば、その中身(LockInfo: 作った時刻、ホスト名、プロセスID、runId、目的)を示して止める。印を消す場面は BR3.8。rollback(U7)も同じ場所の同じ印を使う
    violation: 何も書き換えずに終了コード 1(Conflict)
    source: [NFR7, 契約 C10, AC4.2.7, Q4, Q5, Q6]
  - id: BR3.2
    statement: 残ってしまった印は自動では外さず、--force-unlock で人が外す
    category: authorization
    applies_to: LockInfo、ApplyRequest.forceUnlock
    trigger: BR3.1 で印があったとき
    logic: IF --force-unlock がない THEN 印の中身を示し、「実行中でないことを確かめてから --force-unlock を付けて実行する」と案内して止める。IF --force-unlock がある THEN 印の中身を示し、その印(手元と S3 のうち、あるもの)を消してから BR3.1 をやり直す。時刻やプロセスの有無では自動で外さない(Q6)
    violation: --force-unlock がなければ終了コード 1
    source: [NFR7, Q6]
  - id: BR3.3
    statement: 書き換える直前に今のタグを読み、aws: を除くタグ全体が変更案の作成時と違うリソースは飛ばす
    category: validation
    applies_to: ApplyTarget.currentTags、ApplyTarget.planBeforeTags
    trigger: 確認のあと、記録の BR3.7 より前
    logic: 変更のあるすべてのリソースの今のタグを読む。IF リソースが見つからない THEN skipped(resourceNotFound)。IF aws: で始まるキーを除いた今のタグが、aws: を除いた planBeforeTags と1つでも違う(キーの有無か値) THEN skipped(changedSincePlan)。IF 読めない THEN failed(readFailed)。--rules のときも同じに確かめる(Q2)
    violation: 飛ばしたリソースは書き換えず、最後に一覧で示し、終了コード 2
    source: [FR5.6, AC4.2.1, Q2]
  - id: BR3.4
    statement: 書き換え後に期待するタグは今のタグに変更を順に当てはめて求め、付けるタグと消すキーは、その結果と今のタグの差だけにする
    category: calculation
    applies_to: ApplyTarget.plannedAfterTags、tagsToSet、keysToRemove、touchedKeys
    trigger: BR3.3 で飛ばさなかったリソース
    logic: plannedAfterTags = currentTags に changes を変更案の順に当てはめた結果(U4 の規則 1.9)。tagsToSet = plannedAfterTags のうち、currentTags にないか値が違うキーと、その最終の値。keysToRemove = currentTags にあり plannedAfterTags にないキー。touchedKeys = 両方のキー。同じキーへの途中の書き込み(キー名を変えてから値を変える場合の途中の値)は書かず、最終の値だけを書く。IF touchedKeys が空 THEN 書き換えない(skipped、nothingToWrite。結果の一覧と終了コードには数えない)
    violation: なし
    source: [FR5.2, 契約 C4, U4 の規則 1.9]
  - id: BR3.5
    statement: aws: で始まるタグは、付けることも消すこともしない
    category: constraint
    applies_to: ApplyTarget.tagsToSet、keysToRemove
    trigger: BR3.4 のあと
    logic: tagsToSet と keysToRemove から、aws: で始まるキー(大文字と小文字を区別しない)を必ず除く。変更案に aws: のキーへの変更があっても(U3・U4 で除いているはずのもの)、書き換えずに警告を出す
    violation: なし(そのキーは書き換えない)
    source: [FR5.9, AC4.2.4, 全体の決まり]
  - id: BR3.6
    statement: 書き換え後のタグの数が50を超えるリソースは書き換えない
    category: validation
    applies_to: ApplyTarget.plannedAfterTags
    trigger: BR3.4 のあと
    logic: IF plannedAfterTags の aws: を除いたキーの数 > 50 THEN skipped(tagLimitExceeded)とし、キーの数と上限を理由に示す。上限はすべての種類で50(Q7)。最終の数が50以下でも、BR4.1 の順(付けるのが先)では途中の数が「aws: を除いた currentTags と plannedAfterTags のキーの和」になる。IF その和 > 50 THEN そのリソースだけ消すキーを先に書く(BR4.1 の例外。途中の各キーは beforeTags か plannedAfterTags のどちらかと同じなので、契約 C4 の表の2行目として戻せる)。上限の違う種類で AWS が「タグが多すぎる」と返したら、BR4.2 の failed として理由つきで示す
    violation: 書き換えず、最後に一覧で示し、終了コード 2
    source: [FR5.10, AC4.2.5, Q7]
  - id: BR3.7
    statement: 最初のタグを書き換える前に、記録を始め、書き換えるすべてのリソースの前後の状態を保存する
    category: constraint
    applies_to: RunRecord、JournalEntry
    trigger: BR3.3〜BR3.6 のあと、最初の書き込みの前
    logic: Begin(BR3.1 で作った runId を使う)で記録を作る。RecordBefore で、書き換えるリソースのすべてを status pending として、touchedKeys、beforeTags(= currentTags)、plannedAfterTags と一緒に保存する。BR3.3・BR3.6 で飛ばしたリソースと読めなかったリソースも、status と reason つきで同じ記録に入れる(nothingToWrite は入れない)。項目の種類ごとに記録する値は functional-spec.md の「記録の項目の値」の表のとおり(書いていない項目の touchedKeys は空にする)
    violation: IF 保存に失敗した THEN 何も書き換えず、記録の場所と理由を示して終了コード 1(StorageFailed。契約 C10。印は BR3.8 で消す)
    source: [FR5.8, NFR6, AC4.2.3, 契約 C10]
  - id: BR3.8
    statement: 印を取ったあとは、どの終わり方でも、取った印を消してから終える
    category: constraint
    applies_to: LockInfo
    trigger: BR3.1 で印を取ったあとの、すべての終わり方
    logic: 正常に終わったとき、確認で yes 以外(入力待ちの Ctrl+C を含む)のとき、手順の途中の誤り(今のタグの読み取りの前の誤り、BR3.7 の保存の失敗、BR4.2 の止め、BR4.4 の保存の失敗を含む)、Ctrl+C(BR4.5)、想定外のエラーのとき、取った印を S3、手元の順に消す。消せなかった印は、その場所を示して警告する。印が残るのは、プロセスが強制的に終わらされた場合(kill、電源断)だけで、BR3.2 の --force-unlock で外す
    violation: 印を消せなければ警告を出し、終了コード 1
    source: [NFR7, 契約 C10, Q6]

  # 4. 書き換え
  - id: BR4.1
    statement: 1つのリソースでは、付けるタグを先に、消すキーを後に書く
    category: constraint
    applies_to: ApplyTarget.tagsToSet、keysToRemove
    trigger: 書き換え
    logic: IF tagsToSet が空でない THEN タグを付ける呼び出し(TagWriter の TagResources)を先に行う。それが成功し AND keysToRemove が空でない THEN キーを消す呼び出し(UntagResources)を行う。付けるのに失敗したら、消す呼び出しは行わない。例外: BR3.6 で途中の数が50を超えるリソースは、消すのを先に、付けるのを後に行い、消すのに失敗したら付ける呼び出しは行わない
    violation: なし(途中で止まっても、各キーは beforeTags か plannedAfterTags のどちらかと同じ。契約 C4 の中間の状態)
    source: [契約 C4, FR5.10]
  - id: BR4.2
    statement: 一部のリソースの書き換えに失敗しても、残りのリソースの書き換えを続ける
    category: policy
    applies_to: ApplyTarget.status
    trigger: 書き込みの呼び出しが失敗したとき(AwsAccess の再試行を使い切ったあと)
    logic: 失敗を3つに分ける。(1) 最初の呼び出しが AWS から誤りの答え(AccessDenied、タグが多すぎる、入力の誤りなど)を受けた = 何も書いていない → failed(writeFailed)。(2) 最初の呼び出しは成功し、2つ目の呼び出しが誤りの答えを受けた = 一部だけ書いた → failed(partialWrite)とし、今のタグを読み戻して afterTags に記録する。(3) 時間切れや接続の切断で、AWS が書いたかどうか分からない → status は pending のまま(resultUnknown)とし、今のタグを読み戻せたら afterTags に記録する。どれも誤りの種類(C12)を記録して、次のリソースに進む。U7 は status に関係なく、touchedKeys が空でない項目を契約 C4 の表で判定する。新しい書き込みを止めて BR4.5 と同じに終える場合: 認証情報の誤り(CredentialsMissing。期限切れのトークンも AwsAccess がこの種類にする)を受けたとき、Transient の失敗(再試行を使い切ったもの)が10件続いたとき
    violation: 最後に失敗の件数と一覧を示し、詳細をログファイルに残し、終了コード 1
    source: [FR5.7, AC4.2.2, 契約 C4, 契約 C12]
  - id: BR4.3
    statement: 書き換えたあとに今のタグを読み戻し、記録に残す
    category: policy
    applies_to: ApplyTarget.afterTags、afterConfirmed
    trigger: リソースの書き込みが成功したあと
    logic: 書き込みに成功したリソースを applied とし、今のタグを読み戻して afterTags に入れる。IF aws: を除いた afterTags が plannedAfterTags と同じ THEN afterConfirmed = true。ELSE または読めない THEN afterConfirmed = false(status は applied のまま。タグの API の読み取りが遅れて反映される場合があるため)
    violation: afterConfirmed が false の数を結果に「確かめられなかった」として示す(終了コードは変えない)
    source: [FR5.7, 契約 C4]
  - id: BR4.4
    statement: 書き換えの結果は、途中でも記録に保存する。記録に保存できなくなったら、新しい書き込みを止める
    category: constraint
    applies_to: RunRecord
    trigger: 書き換えの途中
    logic: RecordAfter で、各リソースの status、reason、afterTags を記録に入れ、途中で壊れない方法で保存する(BR5.3)。保存の頻度は NFR の設計で決める(書き換えの前に pending で保存してあるため、保存の前に止まっても U7 が取り消せる)。IF 保存に失敗した THEN 新しい書き込みを止め、BR4.5 と同じに終える
    violation: 終了コード 1。取り消しのコマンドを示す
    source: [FR5.8, NFR6, 契約 C10]
  - id: BR4.5
    statement: Ctrl+C などで止められたときは、動いている呼び出しを終えてから記録を保存し、取り消しのコマンドを示す
    category: policy
    applies_to: RunRecord.completed、ApplyResult.interrupted
    trigger: 取り消しの合図(と、BR4.2 の認証情報の誤り、BR4.4 の保存の失敗)
    logic: 新しい書き込みを始めない。動いている呼び出しの結果を待ち(時間の上限は AwsAccess のまま)、その結果を記録に保存する。まだ書いていないリソースは pending のまま残す。Complete を呼ばない(completed は false)。印を消す(BR3.8)。待っている間にもう一度 Ctrl+C を受けたら、動いている呼び出しを待たずに取り消し、そのリソースは pending(resultUnknown)として記録を保存してから終える
    violation: 終了コード 1。書き換えた件数と、取り消しのコマンドを示す
    source: [FR5.11, NFR6, 契約 C4]
  - id: BR4.7
    statement: 書き込みのまとまりごとに、書く直前に今のタグを読み直し、最初の読み取り(BR3.3)から変わったリソースは飛ばす
    category: validation
    applies_to: ApplyTarget.currentTags
    trigger: 書き込みのまとまり(大きさは NFR の設計で決める)を書く直前
    logic: まとまりのリソースの今のタグを読み直す。IF aws: を除いたタグが、記録した beforeTags(BR3.3 で読んだもの)と違う THEN そのリソースを書かずに skipped(changedSincePlan)として記録する。なくなっていれば skipped(resourceNotFound)。全件を先に読む BR3.3 と書き込みの間の時間に、ほかの人が変えたタグを上書きしないため。読み直しから書き込みまでのわずかな間の変化は防げない(Assumptions に書く)
    violation: 飛ばしたリソースは最後に一覧で示し、終了コード 2
    source: [FR5.6, AC4.2.1]
  - id: BR4.6
    statement: すべてのリソースを終えたら、記録を完了にして印を消す
    category: constraint
    applies_to: RunRecord.completed、LockInfo
    trigger: すべてのリソースの書き換えが終わったとき
    logic: Complete で completed = true にして保存し、Release で印を消す。IF Complete の保存に失敗した THEN 書き換え自体は終わっているため結果を示すが、記録が未完了であることを示す
    violation: Complete の保存に失敗したら終了コード 1
    source: [FR5.8, 契約 C10]

  # 5. 記録
  - id: BR5.1
    statement: 記録の既定の場所は、利用者のホームの下のアカウントごとのフォルダーにする
    category: constraint
    applies_to: RecordLocation
    trigger: --record-to がないとき
    logic: <ホーム>/.tag-refactor/runs/<accountId>/<runId>.json。フォルダーがなければ作り、その利用者だけが読み書きできる権限にする(Windows は利用者のプロファイルの既定の権限)。どのフォルダーから実行しても、rollback が同じ記録を見つけられる(Q4)
    violation: フォルダーを作れなければ StorageFailed(BR3.7)
    source: [FR5.8, 契約 C4, Q4]
  - id: BR5.2
    statement: --record-to に s3://<バケット>/<前置き> を指定したときは、記録を S3 に置く
    category: constraint
    applies_to: RecordLocation
    trigger: --record-to が s3:// で始まるとき
    logic: 記録は <前置き>/<accountId>/<runId>.json、S3 の印は <前置き>/<accountId>/.lock。S3 を読み書きする権限(GetObject、PutObject、DeleteObject)が要る。印の作成は条件付きの書き込み(契約 C6 の ObjectStore.PutIfAbsent)で行い、S3 がそれに対応しないと答えたら StorageFailed として案内する。手元の印とログファイルは既定の場所に置く(BR3.1、BR5.5)。s3:// で始まらない値は手元のフォルダーとして扱い、記録をそのフォルダーの <accountId>/ の下に置く(印は既定の場所のまま)
    violation: 形が正しくなければ使い方の誤り(終了コード 1)。書けなければ StorageFailed
    source: [FR5.8, 契約 C10, Q5]
  - id: BR5.3
    statement: 記録は、書き込んでから名前を変えるなど、途中で壊れない方法で保存する
    category: constraint
    applies_to: RunRecord
    trigger: 記録を保存するたび
    logic: 手元のファイルは、同じフォルダーの一時ファイルに書き、ディスクに書き終えてから名前を変えて置き換える。S3 は1回の書き込みでオブジェクト全体を置き換える(途中の内容は見えない)。保存の途中で止まっても、前の版か新しい版のどちらかが残る
    violation: 保存に失敗したら StorageFailed(BR3.7、BR4.4)
    source: [NFR6, AC4.2.6, 契約 C4]
  - id: BR5.4
    statement: runId は UTC の秒までの日時と乱数の接尾辞で作る
    category: calculation
    applies_to: RunRecord.runId
    trigger: Begin
    logic: <yyyymmdd>T<hhmmss>Z-<4文字の16進の乱数>(例: 20261019T211003Z-7f3a)。印を取る前(BR3.1)に作り、印の中身と Begin に同じ値を使う。Begin で同じ名前の記録がすでにあれば、StorageFailed として止める(秒と乱数が同じになる確率は小さく、印を取り直すより止める方が安全)
    violation: なし
    source: [契約 C4, 契約 C10]
  - id: BR5.5
    statement: 失敗と飛ばしの詳細は、手元のログファイルにも残す
    category: policy
    applies_to: ApplyResult.logFile
    trigger: 失敗か飛ばしがあったとき
    logic: <手元の既定のフォルダー>/<runId>.log に、リソースごとに ARN、リージョン、status、reason、誤りの種類、操作の名前を1行ずつ書く。エラー文、認証情報、タグの値は書かない
    violation: ログファイルを書けなくても書き換えの結果は変えず、警告を出す
    source: [FR5.7, AC4.2.2]

  # 6. 結果
  - id: BR6.1
    statement: 終わりに、書き換えた件数、飛ばしたものと失敗したものの一覧を示す
    category: policy
    applies_to: ApplyResult
    trigger: apply の終わり(BR4.5 で止めた場合も)
    logic: 書き換えた件数、確かめられなかった件数(BR4.3)、飛ばした件数と一覧(ARN と理由)、失敗の件数と一覧(ARN と誤りの種類。一部だけ書いたものと、結果が分からないものを区別する)、変更案の衝突と値が未指定の必須タグの件数(書き換えないもの)、ログファイルの場所を示す。一覧は標準出力、案内は標準エラー(契約 C5)。言語と色は BR2.1 と同じに U4 の表示の部品を使う
    violation: なし
    source: [FR5.6, FR5.7, FR5.10, AC4.2.1, AC4.2.2, AC4.2.5]
  - id: BR6.2
    statement: 終了コードは、失敗を最も重く、飛ばしを次に重く扱う
    category: calculation
    applies_to: ApplyResult.exitCode
    trigger: apply の終わり
    logic: IF 失敗(結果が分からないものを含む)、止められた(BR4.5)、記録の保存の失敗、印を消せなかった、使い方の誤り、アカウントの不一致、印がある、収集が不完全な変更案(inventoryComplete が false。書き換えが成功しても、変更案に入らなかったリソースがあるため。契約 C5 の「結果が不完全」) THEN 1。ELSE IF 飛ばしあり(nothingToWrite を除く)、BR2.5 で止まった、変更案に衝突か値が未指定の必須タグがある THEN 2。ELSE 0(確認で yes 以外の場合も 0)
    violation: なし
    source: [契約 C5]
  - id: BR6.3
    statement: 記録を始めた apply は、終わりに runId つきの取り消しのコマンドを示す
    category: policy
    applies_to: ApplyResult.rollbackCommand
    trigger: BR3.7 で記録を始めたあと(途中で止まった場合も)
    logic: tag-refactor rollback --run <runId> を示す。--record-to で S3 か別のフォルダーを指定したときは --record-from <同じ値> を、--profile を指定したときは --profile <同じ値> を付ける
    violation: なし
    source: [FR5.11, AC4.1.7]
  - id: BR6.4
    statement: apply のヘルプに、オプションと終了コードと確認の動きを示す
    category: policy
    applies_to: Cli
    trigger: apply --help
    logic: BR1.1 のオプション、--plan と --rules のどちらか1つが必要なこと、影響のある変更と --yes・--allow-impact の関係、記録の既定の場所、終了コード 0・1・2 の意味を示す。U1〜U4 のヘルプの形に合わせる
    violation: なし
    source: [契約 C5, U4 からの引き継ぎ(AC6.1.1 の apply の分)]
```

## 規則の一覧

| ID | 内容 | 種類 | 出典 |
|---|---|---|---|
| BR1.1 | `--plan` か `--rules` のどちらか1つ | validation | FR5.2、Q1 |
| BR1.2 | 保存した変更案を読み、アカウントの一致を確かめる | validation | FR5.2、C2 |
| BR1.3 | `--rules` ではその場で変更案を作る | constraint | FR5.1、Q1 |
| BR1.4 | 変更がなければ何もしない(衝突と未指定は件数を示す) | policy | FR5.1、C5 |
| BR1.5 | 不完全な変更案は警告して許し、影響分析の不完全は「影響あり」 | policy | FR5.4、FR5.5、Q3 |
| BR2.1 | 確認画面の中身(影響、不完全、IaC、記録の場所) | policy | FR5.1、FR5.4、Q9 |
| BR2.2 | `yes` のときだけ書き換える | authorization | FR5.1 |
| BR2.3 | 端末でなく `--yes` もなければ止める | authorization | FR5.3、Q8 |
| BR2.4 | `--yes` で影響がなければ確認なし | authorization | FR5.3 |
| BR2.5 | `--yes` だけで影響があれば止める(終了コード 2) | authorization | FR5.5 |
| BR2.6 | `--yes` と `--allow-impact` で影響があっても進む | authorization | FR5.5 |
| BR2.7 | `--yes` なしの `--allow-impact` は案内だけ | policy | C5 |
| BR3.1 | 確認の前に印を取る(手元は常に既定の場所、S3 はその後) | constraint | NFR7、Q4、Q5 |
| BR3.2 | 残った印は `--force-unlock` で人が外す | authorization | NFR7、Q6 |
| BR3.3 | 直前に今のタグを読み、変わっていれば飛ばす | validation | FR5.6、Q2 |
| BR3.4 | 期待するタグと、付けるタグ・消すキーの求め方 | calculation | FR5.2、C4 |
| BR3.5 | `aws:` のタグは書き換えない | constraint | FR5.9 |
| BR3.6 | 50を超えるリソースは書き換えない(途中だけ超えるなら消すのが先) | validation | FR5.10、Q7 |
| BR3.7 | 書き換えの前に記録を保存する | constraint | FR5.8、NFR6 |
| BR3.8 | どの終わり方でも印を消す | constraint | NFR7、C10 |
| BR4.1 | 付けるのが先、消すのが後(BR3.6 の例外あり) | constraint | C4 |
| BR4.2 | 一部が失敗しても続ける(何も書いていない、一部だけ、結果が分からないを区別) | policy | FR5.7、C4 |
| BR4.3 | 書き換え後に読み戻して記録する | policy | FR5.7、C4 |
| BR4.4 | 途中でも記録を保存し、保存できなければ止める | constraint | FR5.8、NFR6 |
| BR4.5 | 止められたときの終え方 | policy | FR5.11、NFR6 |
| BR4.6 | 終わったら記録を完了にして印を消す | constraint | FR5.8、C10 |
| BR4.7 | 書き込みのまとまりごとに読み直し、変わっていれば飛ばす | validation | FR5.6 |
| BR5.1 | 記録の既定の場所(ホームの下) | constraint | FR5.8、Q4 |
| BR5.2 | S3 に置くときの場所 | constraint | FR5.8、Q5 |
| BR5.3 | 途中で壊れない保存 | constraint | NFR6 |
| BR5.4 | runId の形 | calculation | C4 |
| BR5.5 | 失敗と飛ばしのログファイル | policy | FR5.7 |
| BR6.1 | 結果の件数と一覧 | policy | FR5.6、FR5.7、FR5.10 |
| BR6.2 | 終了コード | calculation | C5 |
| BR6.3 | 取り消しのコマンドを示す | policy | FR5.11 |
| BR6.4 | apply のヘルプ | policy | C5、U4 からの引き継ぎ |
