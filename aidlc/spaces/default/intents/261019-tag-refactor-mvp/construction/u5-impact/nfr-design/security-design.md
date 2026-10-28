# セキュリティの設計: U5 影響分析

U1〜U4 のセキュリティの設計(読み取りの口だけを渡す、操作の表と例のポリシーの照合、認証情報を出さない、表示の前の置き換え、JSON と Markdown のエスケープ)は、そのまま使います。ここでは U5 で足す部分を定めます。

出典: `nfr-requirements/security-requirements.md`(NFR3.9、NFR5.10、NFR8.20〜NFR8.22)、`nfr-requirements/performance-requirements.md`(NFR5.11)、`nfr-design-questions.md` Q1、機能設計 `functional-design/`(BR1.1〜BR1.7、BR2.4、BR5.3、BR5.4)、契約 C2・C9・C12、U2 の NFR5.8(認証情報の誤りで実行を止める)。

## 1. 読み取りの口だけを広げる(NFR3.9)

`internal/awsaccess` の読み取りのインターフェース(U1 の `PolicyReader`)に、次の読み取りだけを足します。書き込みの口は作りません。`internal/impact` はこのインターフェースだけを受け取り、SDK の型には触れません。

| 口 | 呼ぶ API(IAM の操作) | 返すもの |
|---|---|---|
| IAM のポリシー(利用者が作った管理ポリシーと、ユーザー・ロール・グループのインラインポリシー) | `GetAccountAuthorizationDetails`(`iam:GetAccountAuthorizationDetails`。絞り込み `User`、`Role`、`Group`、`LocalManagedPolicy`、1ページ 1,000 件、すべてのページを順に) | 管理ポリシーの ARN、付いている数、権限の上限として使われている数、既定の版の URL エンコードの文書(既定の版が答えに無ければ、そのポリシーを読めなかったものにする)。インラインポリシーのユーザー・ロール・グループの種類・名前・ARN、ポリシーの名前、URL エンコードの文書 |
| バケットの一覧と場所 | `ListBuckets`(`s3:ListAllMyBuckets`)、`GetBucketLocation`(`s3:GetBucketLocation`。一覧の答えにリージョンが無いバケットだけ) | 名前、リージョン |
| バケットポリシー | `GetBucketPolicy`(`s3:GetBucketPolicy`。バケットのリージョンで呼ぶ) | 文書、または「ポリシーなし」 |
| キー | `ListKeys`、`DescribeKey`、`GetKeyPolicy`(名前 `default`) | キーの ARN、利用者が作ったキーか、文書 |
| キュー | `ListQueues`、`GetQueueAttributes`(`Policy` と `QueueArn` だけを求める) | キューの ARN、文書(属性が無ければポリシーなし) |
| トピック | `ListTopics`、`GetTopicAttributes` | トピックの ARN、文書(`Policy` が無ければポリシーなし) |
| シークレット | `ListSecrets`、`GetResourcePolicy` | シークレットの ARN、文書(`ResourcePolicy` が無ければポリシーなし) |

- コードの操作の表(`internal/awsaccess` の API の操作と IAM の操作の対応)に、この表の操作を足します。API の名前と IAM の操作の名前が違うのは `ListBuckets`(`s3:ListAllMyBuckets`)だけです。
- 読み取り用の IAM ポリシーの例(`docs/iam/readonly-policy.json`)と、検証用アカウントの読み取り用ロール R2 に、表の IAM の操作を足します。U1・U2 の例の検査(操作に `*` を含まない、`Get`・`List`・`Describe` で始まる、操作の表と例と R2 が一致)をそのまま通します。R2 を変えるのは、インフラの設計で決める手順で、人の許可を得てから行います。
- **例のポリシーの `Resource` の範囲:** 一覧の操作(`List*`)は、AWS がリソースの範囲を指定できないため、U1 の例と同じく `"*"` の文に入れます。`iam:GetAccountAuthorizationDetails` もリソースの範囲を指定できないため `"*"` の文に入れます。1つのリソースを読む操作は、アカウントとサービスの ARN の形に絞った文に分けます: `s3:GetBucketPolicy`・`s3:GetBucketLocation` は `arn:aws:s3:::*`。`kms:DescribeKey`・`kms:GetKeyPolicy` は `arn:aws:kms:*:<account-id>:key/*`。`sqs:GetQueueAttributes` は `arn:aws:sqs:*:<account-id>:*`。`sns:GetTopicAttributes` は `arn:aws:sns:*:<account-id>:*`。`secretsmanager:GetResourcePolicy` は `arn:aws:secretsmanager:*:<account-id>:secret:*`。リージョンを `*` にするのは、影響分析がすべての有効なリージョンを読むためです。例の検査には、`Resource` が `"*"` か、その操作のサービスの ARN の形であることを足します。
- テスト: HTTP の層の偽の AWS で plan を動かし、記録された操作がすべて操作の表の読み取りであることを確かめます(U1 の NFR3.1 の差し替えに、足した口を加えます)。
- **IAM の読み方を改めた(2026-10-28):** はじめは、管理ポリシーを `ListPolicies`・`GetPolicyVersion` で、インラインポリシーを `ListUsers`・`ListRoles`・`ListGroups`・`List*Policies`・`Get*Policy` で読む口でした。U5 のチェックポイントで10分の見積もり(NFR1.19)を超えたため、人の判断で上の表の1つの口に置き換え、使わなくなった IAM の操作を操作の表、例のポリシー、R2 から外しました。IAM は1つの読み取りなので、「一覧のあとで消えた」(IAM の `NoSuchEntity` を NotFound にする扱い)は無くなります。どのページかを読めなければ、IAM の管理ポリシーとインラインポリシーのすべてを読めなかったものにします(機能設計 BR1.5)。答えに含まれる信頼ポリシーなどの照合に使わない中身は、出力にも保存にも `--debug` の行にも出しません(§3)。

### 新しい5つのサービスの誤りの扱い

`internal/awsaccess` の誤りのコードの表(`invoke.go`)に、次を足します。どの誤りも、表示や記録には種類と操作の名前だけを出します(§3)。

| 起きること | 誤りのコード | 種類 | 影響分析での扱い |
|---|---|---|---|
| アクセスが拒否された | 各サービスの `AccessDenied`、`AccessDeniedException`、`AuthorizationError`(SNS)、KMS の `AccessDeniedException` | AccessDenied | 読めなかったもの |
| 認証情報が無い・切れた | 共有の名前(U1・U2 のまま) | CredentialsMissing | §4 のとおり実行を止める |
| 一時的な失敗、回数の上限 | 共有の名前と、各サービスの `Throttling`、`ThrottlingException`、`RequestLimitExceeded`、`InternalError`、`ServiceUnavailable` | Transient(AwsAccess が再試行し、使い切ったら) | 読めなかったもの |
| 一覧のあとでリソースが消えた | S3 の `NoSuchBucket`、SQS の `AWS.SimpleQueueService.NonExistentQueue`・`QueueDoesNotExist`、KMS の `NotFoundException`、SNS の `NotFound`、Secrets Manager の `ResourceNotFoundException` | NotFound | 「ポリシーなし」と同じに扱う(もう無いリソースのポリシーは、変更の影響を受けない)。読めなかったものにしない |
| ポリシーが付いていない | S3 の `NoSuchBucketPolicy` | (誤りにしない) | ポリシーなし |
| KMS のキーの状態で読めない | KMS の `KMSInvalidStateException` | 新しい種類は足さず、Internal | 読めなかったもの |

## 2. 文書の大きさと読み取り(NFR8.20、NFR8.21)

1. 文書を受け取ったら、まず大きさを比べます。IAM の文書は URL エンコードを戻す前のバイト数、リソースポリシーは返ったままのバイト数で、1,048,576 バイトを超えたら JSON として読まずに「読めなかった」にします。理由の文言は固定の「the policy document is larger than 1 MiB」です。
2. この上限は、SDK が応答を読み終えたあとに比べるため、メモリではなく解析の時間と CPU を守るものです。AWS 自身がポリシーの大きさを数十 KB までに制限しているため、応答の読み込みそのものは制限しません。
3. 文書の読み取りは1つの関数(`impact` の中の、文書の文字列と「URL エンコードか」を受け取り、ステートメントの一覧か誤りを返す関数)にまとめます。U1 の読み取り(`parsePolicy`)を、条件のすべての種類、`Resource`・`NotResource`、ポリシー変数の取り出しに広げたものです。
4. ファジングのテスト `FuzzParsePolicy` を、この関数に当てます。種の入力: IAM の URL エンコードの文書、リソースポリシーの文書、Statement が1つのオブジェクト、条件の値が配列・数・真偽値・オブジェクト、閉じていない `${`、入れ子の `${`、既定値つきの変数、深い入れ子、不正な UTF-8、`%` を含む値、`a*a*a*a*b` のようなワイルドカードの条件キーと長い値。1回の読み取りが1秒を超えたら失敗にします(U4 の `FuzzLoadPlan` と同じ作り)。CI では種の入力だけで動き、手元で5分走らせた結果を U5 のチェックポイントの記録に残します。
5. **ワイルドカードの照合と使い回し:** U1 と同じく、`*` と `?` だけを特別な文字にした正規表現(入力の長さに比例する時間で動く)に変えます。作った正規表現は、1回の影響分析で1つの使い回しの入れ物に入れます。この入れ物は、変更の索引とは別の部品で、ロックで守り、同時に動く読み取りの仕事から使えます。入れ物に入れるのは、ロックの中で数えて1,024個までです。それを超えたパターンは、入れ物に入れずにその場で作って使います(メモリを際限なく使わないため)。変更の索引(キー、ARN、IAM のユーザーとロールの変更)は、影響分析の始めに1回作ったら、どの仕事からも読むだけで、書き込みません。U1 の `changeIndex` の中にあった使い回しの地図は、この入れ物に移します。

## 3. 文書の中身を出さない(NFR8.22)

- `--debug` の行は U1 のまま、API の名前、リージョン、時間、再試行の回数だけです。足した口も、同じ呼び出しの包み(`invoke`)を通します。
- 読めなかった理由は、U1 の関数(エラーの種類と操作の名前だけ)と、上の固定の文言、U1 の「the policy document could not be parsed」だけで作ります。SDK の誤りの文言は入れません。
- 指摘の条件の値、ポリシーの名前(policyRef の `#` の後ろ)、ステートメントの ID、`Resource` の値は、U4 の表示の前の置き換え、`SafeJSONWriter`、Markdown のコードスパンを通ります。U4 の NFR8.15・NFR8.16 のテストの入力に、制御文字や `</details>` を含む指摘を足します。

## 4. 失敗の扱いと取り消し(NFR5.10)

- ポリシーの種類、リージョン、1つのポリシーの読み取りの失敗は、エラーとして返さずに UnreadPolicy として結果に集めます。同時に読むための `errgroup` は、最初の失敗ですべてを取り消す使い方をしません(U2 と同じ)。
- **認証情報の誤りで止めるのは、`internal/cli` の包み(`stoppingPolicyReader`。U2 の NFR5.8 で作ったもの)だけが受け持ちます。** この包みを、§1 の足した口のすべてに広げます。どの口が `CredentialsMissing` を返しても、包みは最初の1つの誤りだけを覚え(ロックの中で、まだ覚えていなければ覚える)、実行の `context` を取り消します。`Analyze` は認証情報の誤りを特別に扱わず、ほかの誤りと同じく読めなかったものとして記録しますが、`context` が取り消されたため、取り消しのエラーを返して終わります。`cli` は、取り消しのエラーを受けたら包みが覚えた誤りを返し、plan は認証情報の案内で終わります(終了コード1)。同時に2つの認証情報の誤り(期限切れと拒否など)が起きても、返すのは包みが最初に覚えた1つで、実行ごとに変わりうるのはどちらの案内かだけです(どちらも認証情報の案内)。U1 の `internal/impact` のテストの「認証情報の誤りは読めなかったものになる」は、`Analyze` の単体の振る舞いとしてそのまま正しいため、変えません。
- Ctrl+C などでの取り消しも、`Analyze` は途中の結果を返さずに取り消しのエラーを返します(機能設計 BR5.4)。
- **テスト:**(1) `cli` の plan のテストで、影響分析の途中(足した口のそれぞれ。IAM のインラインポリシー、リージョンのリソースポリシー)で偽の AWS が認証情報の誤りを返すと、残りの読み取りが取り消され、認証情報の案内が出て、終了コードが1になる。(2) 2つの口が同時に認証情報の誤りを返しても、案内は1つだけ出る。(3) 偽の AWS の応答を止めておいて取り消し、`Analyze` が戻ったあと決まった時間のうちに goroutine の数が呼ぶ前と同じに戻る(U2 と同じ確かめ方)。

## 脅威と対策(U5 で足すもの)

| 脅威 | 対策 |
|---|---|
| 影響分析のために、利用者が広すぎる権限を渡す | §1(操作を表で固定し、1つのリソースを読む操作は ARN の形に絞り、例のポリシーと R2 を照らす) |
| 外の人が書き込めるリソースポリシーが、極端に大きい、壊れている、病的なワイルドカードを含む | §2(1 MiB の上限、1つの読み取りの関数とファジング、長さに比例するワイルドカードの照合、使い回しの上限) |
| 同時に動く照合が、共有の入れ物を壊して plan が落ちる | §2 の5(索引は読むだけ。使い回しの入れ物はロックで守る) |
| 文書や AWS の誤りの文言が、デバッグの出力や理由から漏れる | §3 |
| 一部が読めないのに「影響なし」に見える、または一部の失敗で全体が止まる | §4(読めなかったものを集めて不完全にする。止めるのは認証情報の誤りと取り消しだけ)、§1 の誤りの表 |
| 取り消したあとも読み取りが続き、呼び出しやメモリを使い続ける | §4(goroutine が残らないことを確かめる) |
