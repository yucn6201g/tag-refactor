# CI/CD と検証の設計: U7 取り消し

U1〜U6 の CI/CD の流れ(`ci.yml` のジョブ、`ci-gate`、`race` ジョブ、関門、秘密の値、検証コマンド `scripts/verify.sh`)は、そのまま使います。U7 では、検証用アカウントに新しいスタックを作らず、`verify.sh` の U6 の確かめのあとに、書き換え用のロール R3 で apply と rollback を続けて動かす確かめを足します。

出典: NFR の設計(`security-design.md`、`logical-components.md`)、NFR 要件(NFR3.13〜NFR3.15、NFR4.2〜NFR4.4、NFR5.18〜NFR5.24、NFR6.3、NFR6.4、NFR7.2、NFR8.26、NFR9.10、NFR10.11、NFR11.9)、機能設計(`rules.md`、`functional-spec.md`)、契約 C1(ルールファイル)・C4・C5・C10、`infrastructure-design-questions.md` Q1・Q2、U6 のインフラの設計、チームの進め方(§Way of Working、§Testing Posture)。

## `ci.yml`

ジョブは足しません。U7 のテストは、今のジョブの中で動きます。

| 確かめ | 動くジョブ | 注 |
|---|---|---|
| U7 の受け入れ条件と単体テスト(偽の AWS。差分0の各場面、0個のタグの切り替えの両方、放棄、記録の中身の確かめ) | `test`(3つの OS × 2つの版)、`coverage` | AWS の認証情報は要らない(NFR10.11) |
| 大きな偽の AWS のテスト(2つの分布の1万件の rollback。NFR5.19、NFR5.20、NFR5.22) | `test`、`race`(件数10分の1、時間の上限10倍) | 2つの分布のそれぞれが各 OS で30秒以内(NFR5.22)。U7 の大きなテスト全体で各 OS で2分以内(NFR10.11) |
| `RecordRollbackResults` と5秒ごとの保存の同時実行(NFR6.4) | `race` | |
| 印の競争、壊れない保存、Ctrl+C、確認の入力(NFR7.2、NFR9.10) | `test`(3つの OS) | U6 の部品の上で動く |
| rollback が呼ぶ操作が権限の例に含まれる(偽の AWS で見える操作。NFR3.13) | `test` | U6 の `docs_apply_test.go` の表を使う |
| U7 のルールファイルが読める(`TestSandboxU7RuleFilesAreValid`。下の6つのファイルが「キーの規則がない」以外の警告なしで読めること、`u7-rename-rules.yaml` が `renameKey` と `changeValue` の変更案になること、`u7-zero-add-rules.yaml` に `region` の条件があること) | `test` | U6 の `TestSandboxU6RuleFilesAreValid` と同じ作り。AWS に触れる前に誤りが分かる |
| 表示の期待の出力(NFR11.9) | `test` | 英語と日本語、`NO_COLOR` |
| U6 の apply のテスト(共有に直した部品の回帰) | `test`、`race` | 期待は変えない |

## 検証用アカウントのリソース(Q1)

- **新しいスタックは作らない。** 5つの対象は U6 と同じ(U5 のスタックの BU5、QU5、TU5、SU5 と、U6 のスタックの GU6)。記録を S3 に置く確かめには U6 の BU6R、権限には U6 の R3(プロファイル `tag-refactor-apply`)を使う。U5・U6 のスタックは変えない。
- **一時的なキュー:** タグが0個になる確かめのため、検証のスクリプトが管理用のプロファイルで SQS のキュー `tag-refactor-u7-zero-<乱数>` を `tag-refactor-u7-only=1` だけを付けて作る(`project` のタグは付けない)。ARN は変数 `u7_queue_arn` に入れる。作る前は変数が空で、後片付けは変数が空でなければ消す(作る前に失敗したときは何も消さない)。費用は無料の範囲。

## 検証のスクリプト(`scripts/verify.sh` に足す)

### 差し込む位置と順序

U6 の確かめ(手順 1〜7)のすぐあと、U2 のタグのないキューを消す前に置く(U6 の確かめで、R3 のプロファイルと印が残っていないことを確かめたあと)。U6 の確かめが失敗したら U7 の確かめは動かさない。

### オプション

- **apply:** `--profile tag-refactor-apply --yes --allow-impact --debug --lang en`(U6 と同じく、範囲の確かめで U1 の P2 の「決められない」だけを許したあと)。記録の場所は一時フォルダー(`--record-to $u6_dir/records`)、手順 1 の3回目だけ `--record-to s3://<BU6R>/runs`。
- **rollback:** `--profile tag-refactor-apply --yes --debug --lang en`(`--allow-impact` は rollback にない。契約 C5)。記録の場所は apply と同じ値を `--record-from` に渡す(一時フォルダーか、3回目は `s3://<BU6R>/runs`)。

### 使うルールファイル

| ファイル | 中身 |
|---|---|
| `test/sandbox/u6-add-rules.yaml`(U6) | 確かめ専用のキー `tag-refactor-u6-check=1` を5つの種類の確認用のリソースに付ける |
| `test/sandbox/u7-src-rules.yaml`(新規) | 必須タグ `tag-refactor-u7-src`。`when` を5つの種類ごとに1つずつ書き、`match.resourceType` と `match.tags: {project: tag-refactor-fixture}` で絞って値 `a` を付ける |
| `test/sandbox/u7-rename-rules.yaml`(新規) | `keys: {tag-refactor-u7-dst: {aliases: [tag-refactor-u7-src], values: {b: [a]}}}`(キー名の変更と値の変更。U4 の規則で `renameKey` と `changeValue` になる) |
| `test/sandbox/u7-only-remove-rules.yaml`(新規) | `remove: [tag-refactor-u7-only]` |
| `test/sandbox/u7-zero-add-rules.yaml`(新規) | 必須タグ `tag-refactor-u7-zero`。`when` で `match.resourceType: sqs` と `match.region: ap-northeast-1` に値 `1`(タグが0個のキューにはタグの条件を付けられないため)。`ap-northeast-1` のタグの付いた SQS のキュー(U1 の QA、QB、QC と U5 の QU5)にも合う。手順 4 で、それらも写しを取り、戻ったことを確かめる |
| `test/sandbox/u7-cleanup-remove-rules.yaml`(新規) | `remove: [tag-refactor-u6-check, tag-refactor-u7-src, tag-refactor-u7-dst, tag-refactor-u7-zero]`(後片付け用) |

### 確かめの補助(`scripts/u6check.go` を広げる)

| 動作 | 確かめること |
|---|---|
| `plan`(広げる) | 確かめるキーと変更の種類(`add`、`remove`、`rename`)を引数で受け取る。`rename` は `renameKey`(src → dst)と `changeValue`(dst の a → b)だけを許す。範囲は「確認用のリソース(`project=tag-refactor-fixture`)、または引数で渡した ARN」に限る(一時的なキューは引数で渡す。`beforeTags` が空でもよい)。引数の ARN は、変更案に入っていなければならない「必須」(手順 4.2)と、入っていてもいなくてもよい「任意」(手順 4.5)を選べる。それ以外のリソース(前の実行で残った `tag-refactor-u7-only` 付きのキューを含む)が入っていれば、書く前に失敗にする。U1 の P2 の「決められない」だけを許すのは U6 と同じ |
| `rollback`(新規) | rollback の記録(apply の記録の `rollbacks`)を読み、指定した rollbackId(なければ最後のもの)が completed: true で、results に failed と skipped がないこと。`--restored-min 1` で restored が1件以上(手順 1。読み取りが古くて空振りしていないこと)、`--all-already` ですべて alreadyRestored(手順 3、手順 4.5 の2回目)を確かめる |
| `entries`(新規) | rollback の前後の記録で `entries` と `completed` が同じこと(手順 1 の3回目) |
| `same`(新規) | 2つのタグの写し(`aws:` を除く)が同じこと(差分0) |

`scripts/u6check.go` は `//go:build ignore` で通常のビルドとテストに入らないため、U7 で `verify.sh` に `go vet scripts/u6check.go` を新しく足し、ツールのビルドのすぐあと、最初の AWS の呼び出しの前に動かして、構文と型の誤りを AWS に触れる前に見つける(U6 にはこの確かめはない)。

### 手順

「写しを取る」は、管理用のプロファイルで `tag:GetResources` を対象の ARN で呼び、`aws:` を除いたタグを一時フォルダーの JSON に残すこと。「一致を確かめる」は、写しと今のタグを `u6check same` で比べ、違えば10秒おいて最大6回読み直し(結果整合)、それでも違えば対象と違うキーを示して失敗にすること。各 apply の合否は U6 の手順 3 と同じく記録で決め(`u6check record`)、各 rollback の合否は `u6check rollback` で決める。

1. **足して戻す(5回。NFR3.13、NFR4.3、NFR5.21):** 旗 `u7_added` を立ててから、次をくり返す。
   1. 5つの対象の写しを取る。
   2. U6 の add のルールで範囲を確かめて `apply` する(`tag-refactor-u6-check` を足す)。
   3. その実行IDで `rollback` する。終了コード 0 で、`u6check rollback --restored-min 1` が通ることを確かめる。
   4. 5つの対象の一致を確かめる(差分0)。
   5. 3回目(記録は S3)は、rollback の前に管理用のプロファイルで S3 の記録を取っておき、rollback のあとの記録と `u6check entries` で比べ、`rollbacks` に completed: true の RollbackRun が1つ増えたことを確かめる。この3回目の実行IDと記録の場所を変数 `u7_s3_run` に残す(人の確認で使う)。
   6. rollback の `--debug` の行(`^debug: (tag|s3):`)を集める。
2. **名前と値を変えて戻す(1回。NFR3.13、NFR4.3):**
   1. 写し A を取り、`u7-src-rules.yaml` で範囲を確かめて `apply`(1つ目。`tag-refactor-u7-src=a` を足す)。
   2. 写し B を取り、`u7-rename-rules.yaml` で範囲(`rename`)を確かめて `apply`(2つ目。キー名を `tag-refactor-u7-dst` に、値を `b` に変える)。
   3. 2つ目の実行IDで `rollback` し、写し B との一致を確かめる(名前と値が戻る。種類ごとに「値を戻す(付ける)」と「足したキーを消す」の両方が R3 で通る)。
   4. 1つ目の実行IDで `rollback` し、写し A との一致を確かめる。
3. **もう一度の rollback(NFR4.2 (e) の検証用アカウントでの確かめ):** 手順 1 の5回目の実行IDで、もう一度 `rollback` する。終了コード 0 で、`u6check rollback --all-already` が通り、その rollback の `--debug` の行に `tag:TagResources`・`tag:UntagResources` がないことを確かめる。
4. **タグが0個になるキュー(NFR4.3):** 一時的なキューを作り(上の「リソース」)、次を行う。
   1. 一時的なキューの写し Q を取る(`tag-refactor-u7-only=1`)。
   2. `u7-only-remove-rules.yaml` で範囲を確かめ(変更は一時的なキューの `remove` だけ。ARN を引数で渡す)、`apply` してキューのタグを0個にする。
   3. 管理用のプロファイルで、キューの ARN を指定して `tag:GetResources` を10秒おきに最大6回呼び、次の3通りのどれかを表示して記録に残す: 「空のタグで答えに出た」「答えに出なかった」「6回とも古いタグ(`tag-refactor-u7-only` 付き)で出た(確かめられなかった)」。
   4. その実行IDで `rollback` し、写し Q との一致を確かめる(タグが戻る。`plannedAfterTags` が空の項目の判定)。
   5. 0個に戻る向き: もう一度 `u7-only-remove-rules.yaml` で apply してキューを0個にし、写し C(一時的なキューの0個と、U1 の QA、QB、QC、U5 の QU5 のタグ)を取る。`u7-zero-add-rules.yaml` で変更案を作り、一時的なキューの ARN が変更案に入ったかを見る。範囲の確かめ(一時的なキューの ARN は任意)で、確認用のキューと一時的なキューのほかの SQS のキュー(個人用のキューなど)が変更案に入っていれば、その ARN を示して失敗にする(書く前なので何も変わらない。検証用アカウントに確認用でないキューを置かないことが前提)。
      - **入った場合:** 範囲を確かめ(確認用のキューと、引数で渡した一時的なキューだけ)、`apply`(必須タグを足す)し、その実行IDで `rollback` する。写し C との一致(一時的なキューは0個、QA、QB、QC、QU5 は元のタグ)を確かめる。続けて同じ実行IDでもう一度 `rollback` し、`u6check rollback --all-already` で終了コード 0 を確かめる。
      - **入らなかった場合:** 失敗にせず、「`plan` が使うリージョン全体の読み取りは、タグが0個のキューを返さないため、0個に戻る向きは偽の AWS のテストに任せた」と表示して記録に残す(`plan` は、ARN を指定した読み取りではなく、リージョン全体の読み取りで集めたリソースだけから変更案を作る)。
5. **残っていないことの確かめ:** U6 の手順 4 と同じ読み方で、`tag-refactor-u6-check`、`tag-refactor-u7-src`、`tag-refactor-u7-dst`、`tag-refactor-u7-zero` が、5つの対象、確認用のキュー(QA、QB、QC、QU5)、一時的なキューに残っていないこと、手元と S3 の印が残っていないことを確かめ、旗 `u7_added` を下ろす。
6. **見積もり(NFR5.21):** 手順 1 の5回の rollback の `--debug` の行から、次を求めて表示する。
   - `tag:GetResources`、`tag:TagResources`、`tag:UntagResources` の L95 と、再試行が起きた呼び出しの数(U6 の `awk` と同じ求め方)。
   - 読み戻しの読み直しの単位の数 M: 5回の `tag:GetResources` の行の数から、式の最初の読み取りと読み直し・読み戻しの数(検証用アカウントでは1回の rollback あたり、リージョンごとに `1 + 2 × 1`)を引き、3で割る。測った単位の数(1回あたりリージョンの数 × 1)に対する割合を、測る条件の R = 20 に掛けて M を見積もる。
   - 最後の保存の時間: 手順 1 の3回目の rollback の `s3:PutObject` の行のうち最も長い時間を、検証用アカウントの記録の大きさ(取っておいた S3 の記録のバイト数)と1万件の約12 MB の比で大きくする。固定の遅れが大きく、比例しない可能性があるため、比例させた値と、測った値をそのまま使った値の両方を表示し、大きい方を使う。
   - 再試行の待ちの時間: 再試行が起きた呼び出しの数を1万件の回数に比例させ、AwsAccess の最大の待ち時間を掛ける(U6 と同じ)。
   - 2つの分布の入力(tech-stack-decisions.md の測る条件の表): 偏ったリージョンの件数 2,000、R = 20、よくある変更は W_t = W_u = 120、値がばらばらな場合は W_t = W_u = 2,000。NFR5.21 の式で2つの見積もりを出す。

### 後片付け(途中で失敗したとき)

- 旗 `u7_added`(U6 の `u6_added` とは別)は、手順 1 の最初の `apply` の直前に立て、手順 5 で下ろす。手順 1 は U6 の add のルールを使うので、`tag-refactor-u6-check` も U7 の旗で消す対象に入る。
- 既存の `cleanup`(`trap ... EXIT`)の先頭、`$u3_dir` を消す前に、次を順に行う。
  1. 旗 `u7_added` が立っていて、実行ファイルがあるときだけ、`u7-cleanup-remove-rules.yaml` で `plan` と `apply`(R3、`--yes --allow-impact`)を1回動かす(rollback では戻さない。途中で失敗した rollback の記録に頼らないため)。
  2. 既存の残りの確かめ(U6 の `u6_remaining`)を、4つのキーを確かめる形に広げ、残っていれば場所を示す。
  3. `u7_queue_arn` が空でなければ、旗に関係なく一時的なキューを消す。
- 印で止まったら、印の中身を示して `--force-unlock` を案内する。自動では外さない。

### 測る値の記録(NFR5.21、NFR4.3)

| 値 | 求め方 |
|---|---|
| L95(`tag:GetResources`、`tag:TagResources`、`tag:UntagResources`)、再試行の数、読み戻しの読み直しの単位の数 M | 手順 1 の5回の rollback の `--debug` の行(手順 6) |
| 最後の記録の保存の時間 | 手順 1 の3回目の rollback の `s3:PutObject` の行(手順 6) |
| よくある変更を戻す見積もり、値がばらばらな場合の見積もり | NFR5.21 の式(手順 6) |
| タグが0個のキューを、ARN を指定した `tag:GetResources` が答えに出すか(3通り)と、`plan` の変更案に入ったか | 手順 4 |

**見積もりの限界:** U6 と同じく、検証用アカウントの書き戻しは1回の呼び出しで数件しか書かず、標本も少ないため、L95 は実質の最大になります。保存の時間は記録の大きさに比例しないことがあります(固定の遅れが大きい)。チェックポイントでは、これらの限界を踏まえて30分の見積もりを判断し、記録に書きます。

## チェックポイントでの人の確認(NFR11.9)

- 検証のスクリプトは最後に、手順 1 の3回目(S3 の記録。後片付けで消えない)の rollback のコマンドを表示する: `tag-refactor rollback --run <u7_s3_run> --record-from s3://<BU6R>/runs --profile tag-refactor-apply --lang ja`。
- 人は PowerShell でこのコマンドを `--yes` なしで動かし、次を目で確かめる: すでに取り消し済みの記録の警告(`!` と文字)、戻すものがないので確認を求めずに結果が出ること、結果の件数(すでに戻っていた件数)と日本語の表示。この実行で、S3 の記録に2つ目の RollbackRun(すべて alreadyRestored)が足される。
- 差分0の結果、タグの API の答え、30分の見積もりを、上の限界と一緒に記録に残し、承認する。

## AWS への変更と費用

- 検証用アカウントのスタックは変えない(変更セットは要らない)。検証のスクリプトが作るのは一時的なキューだけで、終わりに消す。確認用のリソースのタグは、確かめのあいだだけ変わり、手順 5 と後片付けで元に戻る。
- 認証情報は表示しない(U2 と同じ)。R3 の権限は U6 のまま。
- 費用: 一時的なキューと、BU6R の記録のオブジェクトの分だけ(どちらも無料の範囲か、それに近い)。
