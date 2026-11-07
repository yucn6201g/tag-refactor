# 論理的な部品: U7 取り消し

U7 の部品の分け方と、同時の動き、単位とまとまりの作り方、見積もりの測り方、障害の範囲、テストの設計を決めます。新しいパッケージは作りません(Q1 A)。

出典: U7 の NFR の要件(`construction/u7-rollback/nfr-requirements/`)、`security-design.md`、U7 の機能設計、U6 の NFR の設計(`construction/u6-apply/nfr-design/logical-components.md`)、`nfr-design-questions.md` Q1・Q2。

## パッケージごとの変更(Q1 A、Q2 A)

| パッケージ | 足すもの・変えるもの | 役割 |
|---|---|---|
| `internal/apply` | `Rollbacker`(公開の口)、rollback の判定(契約 C4 の表、機能設計 BR3.2〜BR3.6)、単位の切り方とまとまりの作り方(下の節)、単位ごとの読み直し・書き戻し・読み戻しと読み直しの待ち、記録の中身の確かめ(`security-design.md` §2)。パッケージの説明を「apply と rollback のタグの書き込み」に改める | rollback の流れのすべて |
| `internal/apply`(共有に直すもの) | 次を、apply の `conductor`・`Target`・`journal.Run` に頼らない小さな共通の型(実行の状態)に移し、apply と rollback の指揮役がそれぞれ持つ。(1) リージョンの役(同時4つまで、パニックを止めに変える。止めの誤りの操作名は `apply` か `rollback` を引数で受け取る)。(2) 止める役(2段の止め方、一時的な失敗の数え方)。(3) 1つのまとまりの書き方(書く ARN、付けるタグ、消すキー、消すのが先か、を受け取る)。(4) 2回目の Ctrl+C の放棄の仕組み: 書き込みを始めた項目の印、放棄の印と、それ以後の結果の記録を何もしないものにすること、読み戻しと最後の保存を取り消されない文脈(U6 の `closingContext`、30秒)で行うこと、放棄のあとリージョンの役を待つ時間(U6 の `abandonWait`)。結果の記録は「項目の結果を記録する」「放棄で記録を閉じる」の2つの口を持つ小さな受け手で受け、apply は `journal.Run`、rollback は `journal.RollbackRun` を渡す。(5) 印を取る流れ(`--force-unlock` で外してから取り直す、Conflict のときに印の中身を示す)を、目的(`apply`・`rollback`)を引数にした1つの関数にする。apply の単位とまとまりの作り方(`planUnits`)と判定(`decide`)は変えない | apply と rollback で同じ書き方、止め方、放棄の仕方を使う。apply の振る舞いは変わらないので、U6 の apply のテストはそのまま通る(テストの補助関数が動いた項目を参照している場合だけ、補助関数を直す) |
| `internal/journal` | `OpenForRollback`、`RollbackRun`(`BeginRollback`・`RecordRollbackResults`・`CompleteRollback`・`Close`、生の JSON での保ち方)。U6 の `Run` から5秒ごとの保存の役を小さな型に切り出し、`Run` と `RollbackRun` の両方が使う(`security-design.md` §4、§5) | 記録の読み込みと結果の書き戻し |
| `internal/awsaccess` | ARN を6つに分ける関数(`security-design.md` §2) | 記録の中身の確かめ |
| `internal/cli` | `rollback` のサブコマンド(機能設計 BR1.1、BR4.2〜BR4.4、BR7.2、BR7.3)。`apply` と同じ合図の役(2段の Ctrl+C)、確認の入力、端末の判定を使う | コマンドの入口 |
| `internal/display` | rollback の確認画面と結果(英語と日本語)、ヘルプ、文言の表(機能設計 BR4.1、BR7.1) | 表示 |
| `internal/testutil` | タグのアカウントに、タグが0個のリソースを答えに出さない切り替えと、決めた ARN の読み取りに決めた回数だけ古いタグを返す切り替え(読み戻しの遅れ)を足す | NFR4.2 (c)、NFR5.20、NFR5.24 のテスト |

## rollback の流れの中の同時の動き(NFR5.19)

| 動くもの | 数 | 役割 |
|---|---|---|
| 指揮役(`Rollbacker.Run` を呼んだ goroutine) | 1 | 印、記録の読み込みと確かめ、最初の判定、確認、結果の記録の開始と終わり、印を消す |
| リージョンの役 | 最大4 | 1つのリージョンの単位を順に処理する(読み直し、単位の中のまとまりの書き戻し、読み戻し、読み戻しの読み直し)。U6 と同じ部品 |
| 保存の役 | 1 | 5秒ごとに結果を保存する(`security-design.md` §5) |
| 合図の役 | 1 | `internal/cli` の中。1回目の Ctrl+C で止める合図、2回目で実行の `context` を取り消す(U6 のまま) |

確認の前に全件の判定が要るため、リージョンの役を2回動かします。

1. **1回目(判定だけ):** リージョンごとに、判定する項目の今のタグを100件ずつ読み(機能設計 BR3.1)、判定する。記録には触れない(`BeginRollback` の前)。この途中で Ctrl+C、認証情報の誤り、リージョンの役のパニックが起きたら、確認を出さず、記録を変えず、印を消して終了コード 1 で終える(止めた理由を示す)。一時的な失敗のリージョンの読み取りは、そのリージョンの項目を failed(readFailed)にして続ける。
2. **確認:** 1回目の判定から要約を作って示す(機能設計 BR4.1〜BR4.5)。
3. **結果の記録の開始:** `BeginRollback` のすぐあとに、1回目で結果が決まった項目(alreadyRestored、skipped、failed(readFailed))を `RecordRollbackResults` で1回記録する。
4. **2回目(書き戻し):** 書く項目(restore と `--force` の changedSinceApply)だけを、下の「単位とまとまりの作り方」のとおりに書き戻す。単位の読み直しで変わった項目の結果は、その単位の結果として記録する。

## 単位とまとまりの作り方(NFR5.19、NFR5.20)

1. 判定する項目(機能設計 BR1.4)を、リージョン(記録の `region`)ごとに分け、ARN の文字列の順に並べ、100個ずつに切る(読み直しの単位)。
2. 単位ごとに、書く直前に単位のすべての項目の ARN で `GetResources` を1回呼び(機能設計 BR5.6)、`security-design.md` §3 の関数で今のタグを決めて判定し直し、付けるタグ・消すキー・消すのが先かを求める。書く項目は restore と、`--force` の changedSinceApply。この読み直しの呼び出しが失敗したら(再試行を使い切った、拒否など)、その単位の項目を書かずに failed(readFailed)にし、次の単位に進む(U6 の書き込みのまとまりごとの読み直しの失敗と同じ扱い。認証情報の誤りなら止める役で止める)。
3. 書く項目を、まとまりの鍵(付けるタグを「キー=値」の文字列の順に並べたもの、消すキーを文字列の順に並べたもの、消すのが先か)で分け、同じ鍵の項目を単位の中の順のまま20個ずつに切る(まとまり)。まとまりは単位をまたがない。鍵の作り方は apply と同じ関数を使い、区切りの文字がキーや値と混ざらないことを U6 のテストで確かめた作りのまま使う。
4. まとまりごとに、共有の書き方で書く(付ける呼び出しと消す呼び出し、それぞれあれば)。
5. 単位のすべてのまとまりを書いたら、書いた項目の ARN で `GetResources` を1回呼んで読み戻す。値が違った項目があれば、その項目だけを `security-design.md` §8 のとおり読み直す。

## 30分の見積もりの測り方(NFR5.21)

- U7 のチェックポイントの検証コマンドで、確かめ専用のキーの追加、値の変更、キー名の変更の apply と、その rollback を `--debug --lang en` で5回くり返し、rollback の `--debug` の行から、`tag:GetResources`、`tag:TagResources`、`tag:UntagResources` の操作ごとの L95、再試行が起きた呼び出しの数を求める(U6 と同じ求め方)。読み戻しの読み直しの単位の数(M)は、`tag:GetResources` の行の数から、式の最初の読み取りと読み直し・読み戻しの数を引き、3で割って求める。最後の保存の時間は、記録を S3 に置いた rollback の `s3:PutObject` の行の時間で測る(新しい `--debug` の行は足さない)。手順の詳しい求め方はインフラの設計に書く。
- NFR5.21 の式に、測る条件の2つの分布の偏ったリージョン(2,000件、R = 20)の回数(tech-stack-decisions.md の表)を入れて見積もり、チェックポイントの記録に残す。M は、測った読み直しの単位の割合を R に掛けたもの。最後の保存の時間は、測った時間を記録の大きさ(約12 MB)に比例させたもの。
- **見積もりの限界:** 検証用アカウントの書き戻しは1回の呼び出しで数件しか書かないため、標本は少なく、L95 は実質の最大になる(U6 と同じ)。

## 障害の範囲と隔て方

| 障害 | 範囲 | 隔て方 |
|---|---|---|
| 1つのリソースの書き込みの失敗 | その項目 | 失敗として結果に入れ、ほかの項目を続ける(機能設計 BR5.2) |
| 1つのリージョンの読み取りの失敗 | そのリージョンの項目 | 読めない項目を failed(readFailed)にし、ほかのリージョンを続ける |
| 一時的な失敗が続く、認証情報の誤り | 全体 | 新しい書き込みを止め、結果を保存して印を消す(U6 の止める役) |
| 記録の保存の失敗 | 全体 | 3回続けて失敗したら新しい書き込みを止める。開始と終わりの保存の失敗はその場で止める(`security-design.md` §4、§5) |
| リージョンの役のパニック | 全体 | 止めに変え、結果を保存して印を消す(U6 のまま) |
| プロセスの強制終了 | 全体 | completed false の結果と印が残る。印は `--force-unlock` で外し、同じ `--run` でもう一度 rollback すれば判定し直して残りを戻せる |

## テストの設計(NFR3.13〜NFR3.15、NFR4.2、NFR4.4、NFR5.18〜NFR5.24、NFR6.3、NFR6.4、NFR7.2、NFR8.26、NFR9.10、NFR10.11、NFR11.9)

| 確かめること | どこで | 対応 |
|---|---|---|
| 書き込みの口を渡す範囲、例の許可の一覧に呼んだ操作が含まれる | `internal/cli`、`internal/awsaccess`(偽の AWS で見える操作に限る) | NFR3.13 |
| `touchedKeys` 以外と `aws:` のキーを書かない | `internal/apply` | NFR3.14 |
| 記録の中身の確かめ (a)〜(f)(`arn:` の接頭辞、空の欄、重複する ARN、リージョン名の形、値の同じ `touchedKeys` を含む)。S3 のバケットを含む正しい記録では止まらない | `internal/apply` の表のテスト | NFR3.15 |
| 1回目(判定だけ)の途中の Ctrl+C、認証情報の誤り、パニックで、確認を出さず、記録が変わらず、印が消えて終了コード 1 | `internal/apply`、`internal/cli` | NFR7.2、機能設計 BR2.3 |
| 2回目の Ctrl+C の放棄: 取り消した項目が failed(resultUnknown)、放棄のあとの結果の記録で保存の失敗が起きない、止めた理由が上書きされない、もう一度の rollback で差分0 | `internal/apply`(apply の放棄のテストと同じ形) | NFR4.2 (d)、機能設計 BR5.5 |
| 書き込みの結果と読み戻しの表(`security-design.md` §8)の各行、単位の読み直しの失敗で単位の項目が readFailed になり次の単位に進む | `internal/apply` の表のテスト | NFR5.18、機能設計 BR5.2、BR5.6 |
| S3 の記録の場所での `OpenForRollback` と保存(最初の保存も `Put` で、`PutIfAbsent` でない。S3 の印と記録の前置きが同じ) | `internal/journal`(偽の ObjectStore)、`internal/cli` の受け入れ条件のテストの一部 | NFR6.3、NFR7.2 |
| `RecordRollbackResults` と5秒ごとの保存が同時に動いても壊れない | `internal/journal` の `race` のテスト | NFR6.4 |
| 知らない項目(数値の桁、Unicode、入れ子)が書き戻しで意味を変えない | `internal/journal` | NFR6.3 |
| 差分0: 変更の4種類、途中で止まった apply、0個の向きの両方(偽の AWS の切り替えの両方)、rollback の途中停止のあとの再実行、終わった rollback の再実行、`rollbacks` が2件以上 | `internal/cli` の受け入れ条件のテスト(AC5.1.1〜AC5.1.6)と `internal/apply` の単体テスト | NFR4.2 |
| 単位の読み直しの前の他人の変更を飛ばす、`--force` で書く | `internal/apply` | NFR4.4 |
| 書き込みの失敗の分け方と再試行(U6 の部品を rollback から呼ぶ) | `internal/apply` | NFR5.18 |
| まとまり20以下、組み合わせの違う項目が混ざらない、単位をまたがない、`GetResources` の ARN 100以下、リージョン4つまで、リージョンの中で重ならない | `internal/apply` の大きな偽の AWS のテスト | NFR5.19 |
| 呼び出しの回数が式の上限以下(M = 0 と M > 0) | `internal/apply` の大きな偽の AWS のテスト | NFR5.20 |
| 2つの分布の1万件の rollback が各 OS で30秒以内(race は10分の1) | `internal/apply` の大きな偽の AWS のテスト | NFR5.22 |
| 認証情報の誤りと、Transient が10回続いたときに止まる | `internal/apply` | NFR5.23 |
| 読み戻しの読み直し(1・2・4秒、3回とも違えば notRestored、Ctrl+C で中断) | `internal/apply`(時計を差し替える) | NFR5.24 |
| 知らない項目とほかの RollbackRun を保つ、保存の途中で止めても壊れない、5秒ごとの保存(U6 のテストがそのまま通ることも) | `internal/journal` | NFR6.3、NFR6.4 |
| apply が動いている間の rollback、2つの rollback の同時実行、どの終わり方でも印が消える | `internal/journal`、`internal/cli` | NFR7.2 |
| 認証情報とエラー文が出ない、ログにタグの値がない | `internal/cli`(偽の認証情報と秘密に見えるエラー文) | NFR8.26 |
| 3つの OS で、受け入れ条件のテスト、印の競争、保存の途中で止めたときの読み戻し | CI の3つの OS | NFR9.10 |
| カバレッジ80%以上、大きなテストが合わせて2分以内 | CI | NFR10.11 |
| 色なしでも意味が分かる確認画面と結果(英語と日本語の期待の出力) | `internal/display` | NFR11.9 |
| 検証用アカウントでの差分0、5つの種類の値を戻す・キーを消す、0個のリソースのタグの API の答え、L95 と見積もり | U7 のチェックポイント(インフラの設計で手順にする) | NFR3.13、NFR4.3、NFR5.21 |
