# セキュリティと信頼性の設計: U6 書き換えと記録

NFR の要件(`construction/u6-apply/nfr-requirements/`)を、書き込みの口、権限の例、記録と印の作り、失敗の分け方に落とします。部品の置き場所と流れは `logical-components.md` にあります。

出典: NFR の要件 NFR3.10〜NFR11.8、機能設計(`construction/u6-apply/functional-design/`)、契約 C4・C6・C10・C12、`nfr-design-questions.md` Q1・Q2、AWS の Service Authorization Reference(Resource Groups Tagging API)と AWS 管理ポリシー `ResourceGroupsTaggingAPITagUntagSupportedResources`(v4)。

## 1. 書き込みの口と、渡す範囲(NFR3.10、NFR3.12)

- **口の実装:** `internal/awsaccess` に TagWriter(`TagResources`、`UntagResources`)と ObjectStore(`Put`、`Get`、`PutIfAbsent`、`Delete`。契約 C6)を実装する。どちらも U1 の `invoke`(時間の上限、再試行、`--debug` の行)を通す。
- **渡す範囲:** `internal/cli` は、`apply` と `rollback`(U7)のコマンドだけで TagWriter と ObjectStore を作って渡す。`scan`、`rules suggest`、`plan` に渡す口の型は読み取りの口だけで、TagWriter と ObjectStore を満たさない(型の確かめのテストで調べる)。
- **`aws:` の防御:** TagWriter は、付けるタグか消すキーに、`aws:` で始まるキー(`strings.HasPrefix(strings.ToLower(key), "aws:")`)が1つでもあれば、AWS を呼ばずに InvalidInput を返す。Applier は機能設計 BR3.5 で先に除くので、これは二段目の守り。

## 2. 書き換え用の権限の例(NFR3.11、Q1)

`docs/iam/apply-policy.json` は、次の4つの文からなる。読み取り用の例(`docs/iam/readonly-policy.json`)の文はそのまま含める。

| 文 | 操作 | `Resource` |
|---|---|---|
| 読み取り(U1〜U5 の例のすべて) | 読み取り用の例のとおり | 読み取り用の例のとおり |
| タグの API | `tag:TagResources`、`tag:UntagResources`(`tag:GetResources` は読み取りの文にある) | `"*"`(Service Authorization Reference でリソースの種類の欄が空。範囲を指定できない) |
| 種類ごとのタグの操作 | 下の「書き換えの対象の種類の表」の操作 | 表の `Resource`(`<account-id>` は例ではその文字のまま。U1 の読み取り用の例と同じ書き方) |
| 記録を S3 に置く場合 | `s3:GetObject`、`s3:PutObject`、`s3:DeleteObject`、`s3:ListBucket`(`s3:prefix` の条件で `<record-prefix>/*` に絞る。ないと、ない記録の `GetObject` が NotFound ではなく AccessDenied になるため) | オブジェクトの操作は `arn:aws:s3:::<record-bucket>/<record-prefix>/*`、`s3:ListBucket` は `arn:aws:s3:::<record-bucket>`(例のバケットと前置き) |

バケットの既定の暗号化が SSE-KMS の場合は、そのキーへの `kms:GenerateDataKey` と `kms:Decrypt` も要る。例には入れず、README で案内する(NFR8.25 のとおり、暗号化はバケットの既定に任せる)。

### 書き換えの対象の種類の表(正本)

操作の名前は AWS 管理ポリシー `ResourceGroupsTaggingAPITagUntagSupportedResources`(v4)と照らした。`Resource` の形は、各サービスの ARN の形(タグの API が返す ARN)に合わせた。アカウントの欄が空の ARN(EC2 のスナップショットと AMI)は別に書く。コードの生成の手順の最初に、各行を Service Authorization Reference と照らして確かめ、その操作がリソースの種類を持たなければ `"*"` に直して、その理由を README に書く。

| 種類 | 付ける操作 | 消す操作 | `Resource` |
|---|---|---|---|
| EC2(インスタンス、ボリューム、スナップショット、AMI、セキュリティグループなど) | `ec2:CreateTags` | `ec2:DeleteTags` | `arn:aws:ec2:*:<account-id>:*`、`arn:aws:ec2:*::snapshot/*`、`arn:aws:ec2:*::image/*`(スナップショットと AMI の ARN はアカウントの欄が空) |
| S3 のバケット | `s3:GetBucketTagging`、`s3:PutBucketTagging`、`s3:TagResource` | `s3:GetBucketTagging`、`s3:PutBucketTagging`、`s3:UntagResource` | `arn:aws:s3:::*` |
| SQS のキュー | `sqs:TagQueue` | `sqs:UntagQueue` | `arn:aws:sqs:*:<account-id>:*` |
| SNS のトピック | `sns:TagResource` | `sns:UntagResource` | `arn:aws:sns:*:<account-id>:*` |
| Secrets Manager のシークレット | `secretsmanager:TagResource` | `secretsmanager:UntagResource` | `arn:aws:secretsmanager:*:<account-id>:secret:*` |
| IAM(ロール、ユーザー、管理ポリシー) | `iam:TagRole`、`iam:TagUser`、`iam:TagPolicy` | `iam:UntagRole`、`iam:UntagUser`、`iam:UntagPolicy` | `arn:aws:iam::<account-id>:role/*`、`user/*`、`policy/*` |
| KMS のキー | `kms:TagResource` | `kms:UntagResource` | `arn:aws:kms:*:<account-id>:key/*` |
| Lambda の関数 | `lambda:TagResource` | `lambda:UntagResource` | `arn:aws:lambda:*:<account-id>:function:*` |
| DynamoDB のテーブル | `dynamodb:TagResource` | `dynamodb:UntagResource` | `arn:aws:dynamodb:*:<account-id>:table/*` |
| CloudWatch Logs のロググループ | `logs:TagResource`、`logs:TagLogGroup` | `logs:UntagResource`、`logs:UntagLogGroup` | `arn:aws:logs:*:<account-id>:log-group:*` |
| RDS(DB インスタンス、クラスター、スナップショットなど) | `rds:AddTagsToResource` | `rds:RemoveTagsFromResource` | `arn:aws:rds:*:<account-id>:*` |

- 表にない種類は、例のポリシーでは書き換えられず、AccessDenied(足りない操作の名前つき)の失敗になる。README で、AWS 管理ポリシー `ResourceGroupsTaggingAPITagUntagSupportedResources` を使うか、その種類の操作を足すよう案内する。
- コードには、この表を Go の表(`internal/awsaccess` の操作の表に、種類ごとのタグの操作を足したもの)として持つ。例のポリシーの操作の集まりが「読み取り用の例 + `tag:TagResources`・`tag:UntagResources` + 表の操作 + S3 の3つ」と完全に一致することを、テストで確かめる(足りない操作も余計な操作もない)。
- 検証用アカウントでの確かめ(U6 のチェックポイント): 表のうち、確認用のスタックに置く種類(S3 のバケット、SQS、SNS、Secrets Manager、IAM のロール、EC2 のセキュリティグループ(費用がかからない))を、例から作った書き換え用のロールで1件ずつ書き換えて戻す。一度もタグが付いたことのないリソースも1件入れる(下の「一度もタグのないリソース」)。スタックの作り方はインフラの設計で決める。表のうち確かめない種類(EC2 のインスタンス・スナップショット・AMI、KMS、Lambda、DynamoDB、CloudWatch Logs、RDS)は、README に「検証用アカウントでは確かめていない」と書く。

### 一度もタグのないリソース

`GetResources` は「タグがある、または以前にタグがあったリソース」を返す API なので、一度もタグが付いたことのないリソースは、ARN を指定しても答えに出ないことがある。そのため、最初の読み取りと読み直し(機能設計 BR3.3、BR4.7)で答えに出なかったリソースは、次のように扱う。

- 変更案の `beforeTags` から `aws:` を除いたものが空なら、今のタグを空(`aws:` のタグも空)として進める。リソースが本当に消えていれば、書き込みが 404 で失敗し、そのときに `resourceNotFound` として飛ばす(§5)。
- 空でなければ、機能設計のとおり `resourceNotFound` として飛ばす(タグがあったはずのリソースが答えに出ないのは、消えたため)。

チェックポイントの確かめで、一度もタグのないリソースが答えに出るかを確かめ、結果を記録に残す。

## 3. 記録と印の作り(NFR4.1、NFR6.1、NFR6.2、NFR7.1、NFR8.24、NFR8.25、NFR9.9)

### 壊れない保存(`internal/journal`)

- **手元のファイル:** 記録と同じフォルダーに一時ファイルを作り(`os.CreateTemp`、名前は `.<runId>.tmp-<乱数>` で `.json` で終わらない。権限 0600)、JSON を書き、`Sync` でディスクに書き出し、閉じてから `os.Rename` で置き換える。Windows で `os.Rename` が失敗したら(置き換え先が開かれているなど)、間に100ミリ秒おいて、合計5回まで試す(NFR9.9)。5回とも失敗したら一時ファイルを消して StorageFailed。Linux と macOS では、名前を変えたあとにフォルダーも `Sync` する(Windows ではできないため、しない)。強制終了で残った一時ファイルは、記録を読む側(U7、`.json` だけを読む)からは見えない。Journal は記録を始めるとき、同じフォルダーの1時間より古い一時ファイルを消す。
- **S3:** `PutObject` 1回で記録全体を置き換える。ACL は指定しない。暗号化はバケットの既定に任せる(NFR8.25)。オブジェクトの名前は `<前置き>/<accountId>/<runId>.json` だけ。
- **中身の順:** 記録の JSON は、項目を ARN の順に並べて書く(毎回同じ並び。U7 と人が比べやすいように)。

### 5秒ごとの保存(NFR6.2)

- Journal は、書き換えの間だけ、保存の役の goroutine を1つ動かす。5秒ごとの時計で、Journal のロックの中で記録の写し(項目の深い複製。1万件で数ミリ秒)を作り、ロックの外で保存する。`RecordAfter` はロックを短く取るだけなので、書き込みは保存を待たない。
- 前回の保存から記録が変わっていなければ、その回は保存しない。前の保存が終わっていなければ、その回は飛ばす(保存を重ねない)。飛ばした回と保存しなかった回は、失敗に数えない。
- 5秒ごとの保存の失敗は、次の回でやり直す。3回続けて失敗したら、Journal は「保存できない」状態になり、以降の `RecordAfter` は StorageFailed を返し続ける(契約 C10 の `RecordAfter` の誤り)。Applier は `RecordAfter` の誤りを受けたら、新しい書き込みを止める(機能設計 BR4.4)。
- 止めたとき・終わったときは、保存の役を止めてから、最後の保存を1回行う。最後の保存のあと、Journal は記録を閉じ、以降の `RecordAfter` を拒む(待たずに抜けた goroutine が、保存のあとに記録を変えないように)。最後の保存に失敗したら StorageFailed として終える。
- 時計はテストから差し替えられる。

### 印(NFR7.1、機能設計 BR3.1、BR3.8)

- **手元の印:** `<ホーム>/.tag-refactor/runs/<accountId>/.lock` を `os.OpenFile(O_CREATE|O_EXCL|O_WRONLY, 0600)` で作り、LockInfo の JSON を書いて閉じる。すでにあれば、その中身を読んで Conflict として返す。
- **S3 の印:** `<前置き>/<accountId>/.lock` を ObjectStore の `PutIfAbsent`(`PutObject` の `IfNoneMatch: "*"`)で作る。412 PreconditionFailed と 409 ConditionalRequestConflict は Conflict(中身を `GetObject` で読んで示す)、501 NotImplemented は StorageFailed(条件付きの書き込みに対応しない保存先)。
- **順:** 手元の印を取ってから S3 の印を取る。S3 の印を取れなければ手元の印を消す。消すときは S3、手元の順。
- **消し忘れの防止:** Applier(指揮役)は印を取った直後に「印を消す」処理を `defer` で登録し、正常、誤り、Ctrl+C のどの終わり方でも消す。U6 が動かすすべての goroutine(リージョンの役、保存の役、合図の役)は、入口で `recover` を置き、パニックを Internal の誤りに変えて指揮役に返す(goroutine のパニックでプロセスが落ちないように)。指揮役自身のパニックも `recover` で受けて、印を消して最後の保存をしてから、Internal の誤りとして終える。
- **消すときの確かめ:** 印を消す前に、印の中身の runId が自分の runId と同じかを読んで確かめる。違えば(`--force-unlock` で別の実行が取り直したもの)、消さずに警告する。読んでから消すまでのわずかな間は防げない(受け入れる)。
- **終わりの処理の `context`:** 最後の保存、印を消すこと、止めたあとの読み戻しは、実行の `context` ではなく、取り消されない別の `context`(`context.WithoutCancel` に30秒の時間の上限を付けたもの)で動かす。Ctrl+C のあとでも、これらは失敗しない。
- **`--force-unlock`:** 印の中身を示してから、手元と S3 のうちある印を消し、取り直す。

### フォルダーの権限(NFR8.24)

- Linux と macOS: ツールが作るフォルダーは `os.MkdirAll(..., 0700)`、ファイルは 0600。すでにあるフォルダーは権限を変えず、`Stat` の権限に、ほかの利用者が読める・書ける印(`0o077` のいずれか)があれば警告を出す。
- Windows: Go から権限を指定できないため、その場所の権限を引き継ぐ。プロファイルの外を `--record-to` に指定した場合の限界を README に書く。

### 記録の中身(NFR4.1)

- `touchedKeys`、`beforeTags`(書き換える直前に読んだ、`aws:` を含むタグ全体)、`plannedAfterTags` を、最初の書き込みの前に保存する(機能設計 BR3.7)。項目の種類ごとの値は機能設計の「記録の項目の値」の表のとおり。

## 4. 出さないもの(NFR8.23)

- **ログファイル:** 1行1項目のタブ区切りで、`ARN`、`リージョン`、`status`、`reason`、`誤りの種類`(C12 の名前)、`操作の名前` だけを書く。タグのキーと値、AWS のエラー文、認証情報は書かない。
- **記録:** タグのキーと値は書く(取り消しに要る)。認証情報、AWS のエラー文は書かない(`reason` は理由の名前と誤りの種類だけ)。
- **印:** LockInfo の項目(時刻、ホスト名、プロセスID、runId、目的、アカウント、ツールの版)だけ。
- **`--debug`:** U1 のまま(操作の名前、リージョン、時間、再試行の回数)。書き込みの中身は出さない。

## 5. 失敗の分け方と止め方(NFR5.12、NFR5.17、機能設計 BR4.2)

### 書き込みの答えのリソースごとの失敗

`TagResources`・`UntagResources` の答えの `FailedResourcesMap` の項目(`FailureInfo`)の誤りのコードには、タグの API のコード(`InternalServiceException`、`InvalidParameterException`)のほか、対象のサービスのコードがそのまま入る。そのため、主な分け方は HTTP の状態(`StatusCode`)で行い、コードの名前は補助に使う。

| `StatusCode`(とコードの例) | 扱い | 契約 C12 の種類 |
|---|---|---|
| 429、500 番台(`InternalServiceException`、`ThrottlingException`、`ServiceUnavailable` など) | そのリソースだけを、AwsAccess と同じ待ち方で書き直す(再試行の回数の上限は1回の呼び出しの上限と共有)。使い切ったら、下の「読み戻しで決める」 | Transient |
| 403(`AccessDenied`、`AccessDeniedException`、`AuthorizationError`、`UnauthorizedOperation` など) | 書き直さない。何も書いていない失敗(機能設計 BR4.2 の (1))。足りない操作は、種類の表から分かれば名前を示す | AccessDenied |
| 404(`NoSuchEntity`、`ResourceNotFoundException`、`NotFound`、`NoSuchBucket` など) | 書き直さない。リソースが消えたものとして `skipped`(`resourceNotFound`)。失敗に数えない | NotFound |
| 400(`InvalidParameterException`、タグが多すぎるもの、`ValidationException` など) | 書き直さない。何も書いていない失敗 | InvalidInput |
| `StatusCode` がない、または上のどれでもない | コードの名前で上の表に当てはめ(補助の表は `internal/awsaccess` の誤りの表に足す)、当てはまらなければ何も書いていない失敗 | Internal |

**読み戻しで決める:** Transient の再試行を使い切ったリソース(非対応の種類で `InternalServiceException` が返り続ける場合を含む)は、まとまりの読み戻しの `GetResources` の答えで決める。`touchedKeys` のどのキーも `beforeTags` と同じなら何も書いていない失敗(`writeFailed`、Transient)、すべてが `plannedAfterTags` と同じなら書けたもの(`applied`)、それ以外か読み戻せなければ結果が分からないもの(`pending`、`resultUnknown`)。

呼び出し全体の誤り(例外として返るもの)は、U1 の誤りの表で分ける(期限切れのトークンは CredentialsMissing)。呼び出し全体が成功し、答えの `FailedResourcesMap` に項目があるものは、「書き込みの呼び出しの成功」として扱う(下の数え役)。

### 止める条件

- **認証情報の誤り:** 呼び出し全体が CredentialsMissing を返したら、すぐに新しい書き込みを止める。
- **一時的な失敗が続く:** 全リージョンで共有する数え役(ロックで守る整数)を持つ(NFR5.17 のとおり)。書き込みの呼び出し1回が、再試行を使い切っても呼び出し全体として Transient で終わったら1を足し、どのリージョンでも書き込みの呼び出しが1回成功したら(答えにリソースごとの失敗があっても)0に戻す。10になったら新しい書き込みを止める。リソースごとの失敗の一覧の Transient は数えない。そのため、4つのリージョンのうち1つだけが障害で、ほかが成功し続けるときは止まらない。その場合、障害のリージョンのリソースは、呼び出しごとに再試行を使い切ってから、失敗か結果が分からないものになる(全体を止めるより、ほかのリージョンの書き換えを続けることを選ぶ)。
- **止め方(2段):**
  - **止める合図(1段目):** 1回目の Ctrl+C、認証情報の誤り、一時的な失敗の10回、`RecordAfter` の誤り(保存できない)のどれかで、指揮役は「止める合図」を立てる。リージョンの役は、新しいまとまりや読み直しの単位を始める前に合図を見て、立っていれば抜ける。動いている呼び出しは、実行の `context` のまま最後まで待ち(時間の上限は AwsAccess のまま)、その結果を `RecordAfter` で記録する(機能設計 BR4.5)。
  - **強制の取り消し(2段目):** 止める合図のあとの2回目の Ctrl+C で、実行の `context` を取り消す。動いている呼び出しは中断され、そのリソースは `pending`(`resultUnknown`)のまま残る。リージョンの役が抜けるのを待たずに、指揮役は終わりの処理に進む。
  - どちらの場合も、終わりの処理(最後の保存、印を消す、取り消しのコマンド)は、取り消されない別の `context` で行う(§3)。

## 脅威と対策(U6 で足すもの)

| 脅威 | 対策 |
|---|---|
| 読み取りのコマンドから書き込みが起きる | 書き込みの口を apply と rollback にだけ渡す(§1) |
| AWS が管理するタグを壊す | Applier で除き、TagWriter でも拒む(§1) |
| 強すぎる書き込みの権限 | 例は種類ごとの操作を ARN の形に絞り、余計な操作がないことをテストで確かめる(§2) |
| 記録が途中で壊れる | 一時ファイルとディスクへの書き出しと名前の変更、S3 は1回の書き込み(§3) |
| 同時の apply | 手元の印と S3 の条件付きの書き込みの印。どの終わり方でも消す(§3) |
| 記録やログから情報が漏れる | ログに値とエラー文を書かない(§4)。利用者だけの権限(§3) |
| 一時的な障害で無駄な呼び出しが続く | 書き込みの呼び出しが10回続けて失敗したら止める(§5) |
