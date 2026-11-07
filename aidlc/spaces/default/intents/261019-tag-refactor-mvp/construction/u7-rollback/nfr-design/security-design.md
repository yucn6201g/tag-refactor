# セキュリティと信頼性の設計: U7 取り消し

U7 の NFR の要件(`construction/u7-rollback/nfr-requirements/security-requirements.md`、`tech-stack-decisions.md`)を、部品の作りに落とします。U6 の設計(`construction/u6-apply/nfr-design/security-design.md`)のうち、変えずに使うものは「U6 のまま」と書きます。

出典: U7 の NFR の要件(NFR3.13〜NFR3.15、NFR4.2〜NFR4.4、NFR5.18〜NFR5.24、NFR6.3、NFR6.4、NFR7.2、NFR8.26、NFR9.10、NFR10.11、NFR11.9)、U7 の機能設計(`rules.md` BR1.1〜BR7.4)、契約 C4・C6・C10・C12、`nfr-design-questions.md` Q1・Q2。

## 1. 書き込みの口と権限(NFR3.13、NFR3.14)

- **口を渡す範囲:** `internal/cli` は、`apply` と `rollback` のときだけ書き込みの口(`awsaccess.Writer` と ObjectStore)を作り、`apply.Rollbacker` に渡す。`scan`、`rules suggest`、`plan` には渡さない(U6 のまま。U6 の型の確かめのテストに `rollback` の場合を足す)。
- **権限の例:** U6 の `docs/iam/apply-policy.json` を変えない(JSON のポリシーには説明を書く項目がないため、例の説明は README の権限の節だけに置く)。README の権限の節に「apply と rollback の両方に使う」と、「S3 の記録の前置きは apply の `--record-to` と rollback の `--record-from` で同じにする。例の `<record-prefix>` と違うと AccessDenied になる」を書く。
- **確かめの範囲:** 偽の AWS で見える操作(`tag:GetResources`、`tag:TagResources`、`tag:UntagResources`、`sts:GetCallerIdentity`、S3 の `GetObject`・`PutObject`・`DeleteObject`)について、rollback が呼んだ操作の集まりが例の許可の一覧に含まれることをテストする(U6 の `docs_apply_test.go` の表を使う)。種類ごとのタグの操作(`ec2:DeleteTags` など)は偽の AWS では見えないので、U7 のチェックポイントで、5つの種類それぞれについて「値を戻す(付ける)」と「apply が足したキーを消す」の両方を R3 で行って確かめる(インフラの設計で手順にする)。
- **書くキーの範囲:** rollback が TagWriter に渡すのは、判定の結果の `tagsToSet` と `keysToRemove` だけで、どちらも `touchedKeys` から作る(機能設計 BR3.4)。`aws:` のキーは `touchedKeys` に入らない(§2 で確かめる)うえ、TagWriter の念のための防御(U6 の NFR3.12)もそのまま効く。

## 2. 記録の中身の確かめ(NFR3.15、機能設計 BR1.3)

記録は、置き場所(S3 や共有のフォルダー)に書ける別の人が書き換えられる入力として扱います。`apply.Rollbacker` は、記録を読んだ直後、今のタグを読む前に、次をすべて確かめ、1つでも外れれば AWS を呼ばずに InvalidInput(どの項目のどの確かめかを示す)で止めます。

| 確かめ | 作り |
|---|---|
| (a) アカウントと実行ID | 記録の `accountId` が今のアカウント、`runId` が `--run` と同じ |
| (b) ARN の形とアカウント | ARN が `arn:` で始まり、`arn:<partition>:<service>:<region>:<account>:<resource>` の6つに分けられ、`<partition>`・`<service>`・`<resource>` が空でない。`<account>` が空でなければ `accountId` と同じ(S3 のバケットなど空のものは確かめない)。記録の中で同じ ARN が2回出ない |
| (c) リージョン | 項目の `region` がリージョン名の形(小文字の英字、数字、ハイフンだけで、`ap-northeast-1` のように英字で始まり数字で終わる)。ARN の `<region>` が空でなければ、項目の `region` と同じ(グローバルなリソースは、`region` がタグの API を呼ぶリージョンになるため、形だけを確かめる) |
| (d) `aws:` のキー | `touchedKeys` のどのキーも、大文字と小文字を区別せずに `aws:` で始まらない |
| (e) 触れたキーの範囲 | `touchedKeys` のどのキーも、`beforeTags` か `plannedAfterTags` のキーにあり、`beforeTags` と `plannedAfterTags` で値が違う(片方にしかないキーを含む)。apply が `touchedKeys` に入れるのは書き換えで値の変わるキーだけなので、値の同じキーがあれば記録が手で変えられている |
| (f) 上限 | キーは128文字以下、値は256文字以下(文字は Unicode の文字で数える。AWS のタグの上限と同じ)、`beforeTags` の `aws:` 以外のキーは50個以下 |

ARN を6つに分ける小さな関数を `internal/awsaccess` に1つ足す(`internal/inventory` の `arnRegion` と同じ、`:` で6つに分ける読み方。`inventory` は変えない)。確かめは判定する項目(機能設計 BR1.4)だけでなく、記録のすべての項目に行う(外れた項目があることそのものが、記録が手で変えられたしるしのため)。

**確かめで防げる範囲と残るリスク:** この確かめは、別のアカウントのリソース、`aws:` のキー、AWS の上限を超える値、形の壊れた記録を防ぐ。同じアカウントの中で、形の正しい項目(例: `plannedAfterTags` を今のタグに合わせ、`beforeTags` に好きな値を書いた項目)を足す改ざんは防げない(判定は restore になる)。これに対する対策は、記録の置き場所を自分だけが書ける場所にすることで、README の rollback の節に次を書く: 「rollback は記録に書かれたタグに戻します。記録の置き場所(`--record-to`、`--record-from` のフォルダーや S3 の前置き)は、自分だけが書ける場所にしてください。ほかの人が書ける場所の記録を使うと、その人が書いたタグを付けることになります」。

## 3. タグが0個のリソースの扱い(NFR4.2 (c)、機能設計 BR3.1・BR5.3)

- **判定と読み戻しの共通の関数:** 「答えに出ない項目の今のタグ」を決める関数を1つ作り、最初の判定、単位ごとの読み直し、読み戻しのすべてで使う。答えに出ない項目は、`beforeTags` か `plannedAfterTags` の `aws:` を除くキーが空なら空のタグ、そうでなければ「なくなった」(判定)または「読み戻せなかった」(読み戻し)とする。
- **偽の AWS:** `internal/testutil` のタグのアカウントに「タグが0個のリソースを答えに出さない」切り替えを足す(既定は、今のとおり一度でもタグがあれば出す)。NFR4.2 (c) のテストは、切り替えの両方で同じ期待を確かめる表のテストにする。
- **限界:** 答えに出さない API では、`beforeTags` が空の項目は、本当に削除されていても `alreadyRestored` になる(戻すタグがないので害はない)。README に書く。

## 4. 記録の読み込みと結果の書き戻し(NFR6.3、NFR6.4、Q2)

- **読み込み:** `journal.Journal.OpenForRollback(ctx, LockHandle, runId)` は、LockHandle の記録の場所(U6 の決め方。手元の既定の場所、`--record-from` の手元のフォルダー、S3)から記録を1回だけ取得し、版を確かめ(読めない版は UnsupportedVersion)、`RollbackRun` を返す。場所は LockHandle からだけ取り、引数で重ねて受け取らない。U6 の `LockHandle` に目的(`apply`・`rollback`)を持たせ、`OpenForRollback` は目的が `rollback` でない LockHandle を実行時に Internal で拒む(印を取る前に読まないことを、型ではなく実行時の確かめで守る)。
- **判定に渡すもの:** `RollbackRun.Record()` は、取得した記録を U6 の型(`RunRecord`)にしたもの(`entries` と、すでにある `rollbacks` の rollbackId・startedAt・completed を含む)を返す。`apply.Rollbacker` は、これで記録の中身を確かめ(§2)、判定し、BR4.1 の「取り消し済み」の警告を出す。
- **知らない項目の保ち方:** 1回取得したバイト列を、型付きの `RunRecord` と、最上位の `map[string]json.RawMessage`(と、その中の `rollbacks` の `[]json.RawMessage`)の両方に解析する(2回取得しない。取得の間に記録が変わって、確かめた中身と書き戻す中身が食い違うのを防ぐ)。書き戻すときは、最上位の写しの `rollbacks` だけを置き換える。置き換える配列は、ほかの RollbackRun は読んだときの生の JSON のまま、自分の RollbackRun だけを新しく書いたもの。rollback は `entries` と `completed` を変えないので、項目の中の知らない項目は、最上位の写しの `entries` の生の JSON のまま残る(各項目を混ぜ直す必要はない)。書き戻しでバイト列が変わることは許す: 最上位のキーの並び(Go の `map` の書き出しの順)、空白と改行、`<`・`>`・`&` の書き方(`<` など)。意味は変わらない。生の JSON のまま残る部分(`entries`、ほかの RollbackRun、知らない項目)は、数値の桁、Unicode の文字、入れ子を含めて、取得したときの中身のまま保たれる。
- **3つの呼び出し:** `BeginRollback(rollbackId, startedAt)` は completed false・空の results の自分の RollbackRun を足して、その場で保存し、成功したら5秒ごとの保存(§5)を始める(失敗したら書き込みの前に止める)。`RecordRollbackResults([]result)` は、自分の results に ARN を鍵に足すか置き換え、版の番号を上げる(保存は §5 の保存の役が行う)。`CompleteRollback()` は5秒ごとの保存を止め、completed を true にして、その場で保存する。止めたときは `Close()` で5秒ごとの保存を止め、completed false のまま、その場で保存する。放棄(2回目の Ctrl+C)で閉じたあとの `RecordRollbackResults` は何もせず誤りも返さない(U6 の `Run` の閉じたあとの扱いと同じにし、閉じたあとの書き込みで保存の失敗を起こさない)。
- **契約 C10 との対応:** 契約 C10 の3つの呼び出し(`LockHandle, runId, rollbackId` を受け取る関数の形)は、`OpenForRollback(LockHandle, runId)` が返す `RollbackRun` のメソッドとして作る(U6 の `Begin` が `Run` を返し、`RecordBefore` などがそのメソッドである形と同じ)。契約の文面に `OpenForRollback` を足す。
- **壊れない保存:** U6 の部品をそのまま使う。手元は同じフォルダーの一時ファイルに書いて名前を変える(Windows は最大5回やり直す。U6 の NFR9.9)。S3 は `PutObject` で全体を置き換える(条件付きの書き込みにしない。印を持っている間はほかの書き手がいないため。`--force-unlock` で他人の印を外して同時に動かすと、どちらかの結果が失われうることを README に書く)。
- **大きさ:** 1万件の apply の記録(約10 MB)に、1万件の結果(1件約150バイト)を足して約12 MB。1回の保存は、手元で1秒未満、S3 で数秒と見込む。保存の時間は、新しい `--debug` の行を足さずに、記録を S3 に置いた rollback の `s3:PutObject` の行(U1 の `--debug` の形。AwsAccess の呼び出しなので今も出る)で測る(NFR5.21。手元の保存は `--debug` の行を出さない)。

## 5. 5秒ごとの保存(NFR6.4)

U6 の `Run` の5秒ごとの保存の役(時計の差し替え、保存を重ねない、3回続けて失敗で壊れたしるし、止めたとき・終わったときのその場の保存)を、`journal` の中の小さな型(保存の役)に切り出し、apply の `Run` と rollback の `RollbackRun` の両方が持つ。切り出しは振る舞いを変えないので、U6 の NFR6.2 のテストの期待は変えない。テストの補助関数が `Run` の中の項目(保存中のしるしなど)を直接見ている場合は、保存の役のしるしを見るように補助関数だけを直す(期待と確かめることは変えない)。`RollbackRun` が壊れたしるしを持ったら、`apply.Rollbacker` は新しい書き込みを止める(止める理由 `storage`)。

## 6. 印(NFR7.2、機能設計 BR2.1〜BR2.3)

- `apply.Rollbacker` は、入力とアカウントを確かめたあと、rollbackId(U6 の runId と同じ作り)を作り、`Journal.Lock(accountId, location, rollback, rollbackId)` で印を取ってから `OpenForRollback` を呼ぶ。印は `defer` で、どの終わり方でも(パニックを含む)消す(U6 の作りのまま)。
- 印があれば、印の中身を示して Conflict で止める。`--force-unlock` は U6 の `Journal.ForceUnlock` を使う。

## 7. 出さないもの(NFR8.26)

- ログファイルは U6 の `Journal.WriteLog` を rollbackId の名前で使い、項目ごとに ARN、状態、理由、誤りの種類、操作の名前だけを書く。タグの値と AWS のエラー文は書かない(U6 のまま)。
- 確認画面と結果には、ARN と理由とキーの名前を出すが、認証情報とエラー文は出さない。`--debug` の行は U1 の形(操作の名前、リージョン、時間、再試行の回数)に、§4 の保存の行を足すだけ。

## 8. 失敗の分け方、止め方、読み戻しの読み直し(NFR5.18、NFR5.23、NFR5.24)

- **書き込みの失敗:** 1つのまとまりの書き方(付けるのが先、途中だけ50を超えるなら消すのが先)と、2つの呼び出しの成否から書き込みの結果を返す部分は、U6 のものを apply と rollback で共有する(logical-components.md)。共有の部分が返す書き込みの結果は U6 と同じ5つ(すべて書けた、1つ目が拒まれた、2つ目が拒まれた、なくなった、1つ目か2つ目が答えなしで終わった=読み戻しで決める)と、2回目の Ctrl+C で取り消された、の6つ。rollback は、これと読み戻しの値から、項目の状態を次の表で決める(apply の `decide` に当たる。「戻った」は、すべての `touchedKeys` で読み戻しの値が `beforeTags` と同じこと。「書く前と同じ」は、単位の読み直しで読んだ値と同じこと)。

| 書き込みの結果 | 読み戻し | 項目の状態 |
|---|---|---|
| すべて書けた | 戻った | restored |
| すべて書けた | 違う | 読み直し(§8 の下)のあと、戻れば restored、違えば failed(notRestored) |
| すべて書けた | 読み戻せない | restored(notConfirmed) |
| 1つ目が拒まれた(何も書いていない) | 問わない | failed(writeFailed) |
| 2つ目が拒まれた(片方だけ書いた) | 問わない | failed(partialWrite) |
| なくなった(NotFound) | 問わない | skipped(resourceNotFound) |
| 答えなしで終わった(1つ目か2つ目) | 戻った | restored |
| 答えなしで終わった(1つ目) | 書く前と同じ | failed(writeFailed) |
| 答えなしで終わった | それ以外、読み戻せない | failed(resultUnknown) |
| 2回目の Ctrl+C で取り消された | 問わない | failed(resultUnknown)(取り消した要求は AWS 側で後から反映されうるため。U6 と同じ) |

  どの failed も、同じ `--run` でもう一度 rollback すれば、今のタグで判定し直して正しく扱える(機能設計 BR5.2)。
- **止め方:** U6 の止める役(1回目の合図で新しい単位を始めない、2回目で呼び出しを取り消す、認証情報の誤りで止める、Transient で終わる呼び出しが全リージョンで10回続いたら止める)をそのまま使う。
- **読み戻しの読み直し:** 単位の読み戻しで `touchedKeys` の値が `beforeTags` と違った項目だけを集め、1秒、2秒、4秒の待ちのあとに `GetResources` で読み直す(最大3回)。待ちは止める役の合図と、実行の `context` の取り消しで中断する(中断したら、まだ一致していない項目は `notConfirmed` ではなく `failed(resultUnknown)` にする。書いた結果はあるが確かめていないため、もう一度の rollback で判定し直す)。時計は差し替えられるようにする。

## 9. 他人の変更と、上書きの窓(NFR4.4)

- 判定は、印を取ったあとに読んだ記録と、単位ごとの読み直しで行う。差し込みの時点を「単位の読み直しの前」とし、それより前の他人の変更は必ず見つける。
- 読み直しから単位の最後の書き込みまでの窓は閉じられない(タグの API に比べてから書く書き込みがない)。窓の大きさは、単位の中の書き込みの数 × 1回の時間(値がばらばらな場合で約200回、約1分)。README に限界として書く。

## 脅威と対策(U7 で足すもの)

| 脅威 | 対策 |
|---|---|
| 記録の置き場所に書ける別の人が、記録を書き換えて任意のタグを付けさせる | 記録のすべての項目を、AWS を呼ぶ前に確かめ、別のアカウント、`aws:` のキー、上限を超える値、形の壊れた記録を防ぐ(§2)。同じアカウントの中で形の正しい項目を足す改ざんは防げない(残るリスク)。記録の置き場所を自分だけが書ける場所にするよう README に書く(§2) |
| 2回目の Ctrl+C のあとも動いているリージョンが、閉じた記録に書いて保存の失敗を起こす | 放棄の仕組みを apply と共有し、放棄のあとの結果の記録を何もしないものにする(logical-components.md、§4) |
| タグが0個のリソースを読めず、戻す項目を飛ばす、戻った項目を「なくなった」にする | 判定と読み戻しで同じ関数を使い、記録の前か後が0個なら空として扱う。偽の AWS の両方の切り替えで確かめる(§3) |
| 結果の保存で、知らない項目やほかの rollback の結果を消す | 最上位と `rollbacks` を生の JSON で持ち、自分の RollbackRun だけを置き換える(§4) |
| 古い記録で判定する、同時の rollback で記録が壊れる | 印を取ってから記録を読む。`OpenForRollback` は目的 `rollback` の LockHandle だけを受け付ける(§4、§6) |
| 読み戻しの反映の遅れで、戻ったものを失敗にする | 違った項目だけを1・2・4秒で読み直す(§8) |
| 他人が単位の読み直しのあとに変えたタグを上書きする | 閉じられない窓として README に書く(§9) |
