# 業務の規則: U5 影響分析

U5 で完成させる影響分析の規則です。U1 の最小版(利用者が作った管理ポリシーの `<前置>:ResourceTag/<キー>` を、キー名の変更について照合。U1 の規則 5.1〜5.6)を、読むポリシー、照合する場所、変更の種類のすべてに広げます。ID はこの作業単位の中での番号です。ほかの作業単位の規則は「U1 の規則 5.5」のように書きます(その作業単位の `rules.md` の ID から「BR」を省いた番号です)。U1 の規則のうち、ここで言い直したものはここが正本です。

出典: 要件 FR4.1〜FR4.7、FR3.8、NFR2、NFR3、CON3、ストーリー US3.2(AC3.2.1〜AC3.2.6)、契約 C2・C9・C12、`functional-design-questions.md` Q1〜Q6、U1 の規則 5.1〜5.6、U4 の規則 2.5〜2.7、AWS のドキュメント(`aws:ResourceTag/tag-key` のキーは大文字・小文字を区別しない)。

```yaml
rules:
  # 1. 読むポリシー
  - id: BR1.1
    statement: IAM の管理ポリシーは、利用者が作ったもののうち、ユーザー、ロール、グループのどれかに付いているもの(権限の上限(permissions boundary)として付いているものを含む)を読み、中身は既定の版を読む(U1 の規則 5.1 を広げる)
    category: constraint
    applies_to: PolicySource(iamManaged)
    trigger: 影響分析の開始
    logic: 利用者が作った管理ポリシーを、IAM の設定をまとめて返す GetAccountAuthorizationDetails(絞り込みは LocalManagedPolicy、ほかの種類と同じ呼び出し)で、ページが分かれていても最後まで読み、付いている数か権限の上限として使われている数が1以上のものだけを選び、それぞれ既定の版(版の一覧のうち既定のもの)の文書を使う。既定の版の文書が答えに無いポリシーは、そのポリシーを読めなかったものにする(BR1.5)。U5 のチェックポイントで、呼び出しの回数を減らすため ListPolicies と GetPolicyVersion から改めた(2026-10-28)
    violation: 読めなければ BR1.5
    source: [FR4.1, U1 の規則 5.1]
  - id: BR1.2
    statement: IAM のインラインポリシーは、すべてのユーザー、ロール、グループについて読む
    category: constraint
    applies_to: PolicySource(iamInline)
    trigger: 影響分析の開始
    logic: BR1.1 と同じ GetAccountAuthorizationDetails の呼び出し(絞り込みは User、Role、Group)で、ユーザー、ロール、グループと、それぞれのインラインポリシーの名前と文書を最後まで読む。サービスにリンクされたロールも同じに読む。U5 のチェックポイントで、一覧・名前・文書を別々に読む呼び出しから改めた(2026-10-28)
    violation: 読めなければ BR1.5
    source: [FR4.1, Q2]
  - id: BR1.3
    statement: リソースポリシーは、5つのサービスの一覧の API で変更案のリージョンのすべてのリソースを探し、それぞれのポリシーを読む。scan の結果には頼らない
    category: constraint
    applies_to: PolicySource(s3Bucket、kmsKey、sqsQueue、snsTopic、secretsManagerSecret)
    trigger: 影響分析の開始
    logic: >
      S3 はバケットの一覧を1回読み(全リージョン分が、バケットのリージョンとともに返る。答えにリージョンが無いバケットは、バケットの場所を読んで決める)、バケットのリージョンが変更案のリージョンに入るものについて、そのリージョンでバケットポリシーを読む。
      KMS、SQS、SNS、Secrets Manager は、変更案のリージョンごとに一覧を最後まで読み、リソースごとにポリシーを読む
      (KMS はキーの情報を読んで、利用者が作ったキーだけ。キーポリシーの名前は default)。
      ポリシーが付いていないこと(S3 のポリシーなし、SQS・SNS の Policy の属性なし、Secrets Manager のポリシーなし)は、読めなかったことにしない
    violation: 読めなければ BR1.5
    source: [FR4.2, Q1]
  - id: BR1.4
    statement: 読まないポリシーを決めておき、README に書く
    category: constraint
    applies_to: PolicySource
    trigger: 影響分析の開始
    logic: ロールの信頼ポリシー、AWS が管理するポリシー、どこにも付いていない管理ポリシー、AWS が管理する KMS のキー、SCP と RCP、そのほか CON3 の対象外のものは読まない。README の影響分析の節に、読むものと読まないものを書く
    violation: なし
    source: [CON3, Q2, U1 の規則 5.1]
  - id: BR1.5
    statement: 一覧や中身を読めなかった場合は、種類と理由を示し、影響分析を不完全にする。読めたものの照合は続ける
    category: policy
    applies_to: UnreadPolicy、ImpactResult.complete
    trigger: ポリシーの読み取りが拒否または失敗したとき(AwsAccess の再試行を使い切ったあと)
    logic: >
      一覧を読めなければ、その種類(リソースポリシーならそのリージョンの、その種類)の UnreadPolicy を1件作る(policyRef は空)。
      IAM は1つの読み取り(BR1.1、BR1.2)なので、どのページかを読めなければ iamManaged と iamInline の UnreadPolicy を1件ずつ作り(policyRef は空)、IAM のポリシーは1つも照合しない。
      1つのポリシーを読めなければ、そのポリシーの UnreadPolicy を作る(policyRef は BR1.6 の形)。
      文書を JSON として読めなければ、理由を "the policy document could not be parsed" とする。
      UnreadPolicy が1件でもあれば complete = false。理由はエラーの種類と操作の名前だけで、エラー文は入れない
    violation: 結果の先頭に「影響分析は不完全」を出し、終了コード 1(U4 の規則 2.6)
    source: [FR4.7, AC3.2.5, U1 の規則 5.5]
  - id: BR1.6
    statement: ポリシーの参照(policyRef)は、ポリシーを一意に指す文字列にする
    category: calculation
    applies_to: PolicySource.policyRef、ImpactFinding.policyRef、UnreadPolicy.policyRef
    trigger: ポリシーを読むとき
    logic: 管理ポリシーはポリシーの ARN。インラインポリシーは「ユーザー・ロール・グループの ARN」+「#」+「ポリシーの名前」(IAM の名前とパスには # を使えないため、区切りが一意になる)。リソースポリシーは、ポリシーが付いたリソースの ARN(S3 は arn:<partition>:s3:::<バケット名>、KMS はキーの ARN、SQS は QueueArn、SNS はトピックの ARN、Secrets Manager はシークレットの ARN)
    violation: なし
    source: [FR4.6, 契約 C2(policyRef)]
  - id: BR1.7
    statement: IAM の文書は URL エンコードを戻してから JSON として読む。リソースポリシーの文書は返ったまま JSON として読む
    category: validation
    applies_to: PolicySource
    trigger: 文書を読んだあと
    logic: IAM(管理ポリシーとインラインポリシー)は URL エンコードで返るため戻す。リソースポリシーは戻さない(値の中の % を壊さないため)。Statement は1つのオブジェクトでも配列でもよい
    violation: JSON として読めなければ BR1.5
    source: [U1 の規則 5.1, FR4.2]

  # 2. タグへの参照を見つける
  - id: BR2.1
    statement: 照合する条件キーは、前置のあとが ResourceTag/<キー>、RequestTag/<キー>、PrincipalTag/<キー>、TagKeys のものとする
    category: calculation
    applies_to: TagReference(location = conditionKey)
    trigger: 条件を読むとき
    logic: >
      条件キーを最初の「:」で分ける。前置は aws か、英小文字・数字・ハイフンからなるサービス名(例 ec2、s3、iam)。
      後ろが ResourceTag/、RequestTag/、PrincipalTag/ で始まればその後ろをタグのキーとし、TagKeys と同じならタグのキーなし(TagKeys)とする。
      前置と ResourceTag などの綴りは大文字・小文字を区別せずに比べる。ほかの条件キー(s3:ExistingObjectTag/ など)は参照にしない
    violation: なし
    source: [FR4.4, AC3.2.6, U1 の規則 5.2]
  - id: BR2.2
    statement: タグのキーの比べ方は、大文字・小文字を区別せず、* と ? はパターン、${ を含むものは判定できないとする
    category: calculation
    applies_to: TagReference.tagKeyPattern
    trigger: BR4 の照合
    logic: IF タグのキーに "${" がある THEN 判定できない(BR4.6)。ELSE IF "*" か "?" がある THEN 大文字・小文字を区別しないパターンとして変更のキーに当てはめる(パターンは機械的に判定できるため、当てはまれば affected、当てはまらなければ影響なし。FR4.5 の「判定できない」に当たるのはポリシー変数とその他の演算子。AC3.2.4 との関係は functional-spec.md の Assumptions)。ELSE 大文字・小文字を区別せずに変更のキーと比べる
    violation: なし
    source: [FR4.5, AC3.2.4, U1 の規則 5.2, U1 の規則 5.4]
  - id: BR2.3
    statement: 演算子は、照合では ForAnyValue:、ForAllValues: と IfExists を外して種類を決める。表示には書いたまま残す
    category: calculation
    applies_to: Condition.baseOperator
    trigger: 条件を読むとき
    logic: >
      種類 = 等しい(StringEquals、StringNotEquals。大文字・小文字を区別する)、
      大文字・小文字なし(StringEqualsIgnoreCase、StringNotEqualsIgnoreCase)、
      パターン(StringLike、StringNotLike。大文字・小文字を区別する)、
      Null、
      その他(Arn、Numeric、Date、Bool、IpAddress などの文字列以外)。演算子の名前は大文字・小文字を区別せずに比べる
    violation: なし
    source: [Q4, FR4.4]
  - id: BR2.4
    statement: 条件キーの中、条件の値、Resource、NotResource の中のポリシー変数も、タグへの参照として見つける
    category: calculation
    applies_to: TagReference(location = keyVariable、conditionValue、resource、notResource)
    trigger: ステートメントを読むとき
    logic: >
      値の中の ${…} を探し、中身(既定値の「, '…'」より前)が BR2.1 の ResourceTag/、RequestTag/、PrincipalTag/ の形なら参照にする。
      条件キーの中(例 aws:ResourceTag/${aws:PrincipalTag/k} の PrincipalTag/k。location = keyVariable)と条件の値の中の参照はその条件に属し、Resource と NotResource の中の参照はその値に属する。
      条件キーが BR2.1 に当てはまらない条件(例 s3:prefix)でも、値の中の参照は見つける。条件キーの中の変数は、条件キーそのものの参照(BR4.6 で判定できない)とは別の参照として、BR3 で結び付ける
    violation: なし
    source: [Q6, FR4.4, FR4.5]

  # 3. 参照と変更の結び付け
  - id: BR3.1
    statement: IAM のポリシーの ResourceTag、RequestTag、TagKeys の参照は、そのキーのすべての変更に結び付ける
    category: calculation
    applies_to: TagReference、TagChangeInput
    trigger: BR4 の照合の前
    logic: IF sourceKind が iamManaged か iamInline AND tagKind が ResourceTag、RequestTag、TagKeys THEN 結び付ける候補 = すべての変更
    violation: なし
    source: [Q3, FR4.1]
  - id: BR3.2
    statement: リソースポリシーの ResourceTag、RequestTag、TagKeys の参照は、そのポリシーが付いたリソース自身の変更だけに結び付ける
    category: calculation
    applies_to: TagReference、TagChangeInput
    trigger: BR4 の照合の前
    logic: IF sourceKind がリソースポリシー AND tagKind が ResourceTag、RequestTag、TagKeys THEN 結び付ける候補 = resourceArn が attachedResourceArn と(文字列として)同じ変更
    violation: なし
    source: [Q3, FR4.2]
  - id: BR3.3
    statement: PrincipalTag の参照は、どのポリシーでも、IAM のユーザーとロールの変更だけに結び付ける
    category: calculation
    applies_to: TagReference、TagChangeInput
    trigger: BR4 の照合の前
    logic: IF tagKind が PrincipalTag THEN 結び付ける候補 = resourceArn が arn:<partition>:iam::<account>:user/ か role/ で始まる変更
    violation: なし
    source: [Q3, FR4.4]

  # 4. 変更の種類ごとの判定
  - id: BR4.1
    statement: 変更のキーは、renameKey は fromKey と toKey、changeValue と removeKey は fromKey、addRequired は toKey とする。TagKeys 以外の参照は、タグのキーが変更のキーに当てはまる変更だけを判定する
    category: calculation
    applies_to: TagReference、TagChangeInput
    trigger: BR3 の候補ごと
    logic: IF tagKind が TagKeys THEN BR4.5。ELSE IF タグのキーが判定できない(${) THEN BR4.6。ELSE IF BR2.2 で変更のキーのどれにも当てはまらない THEN 影響なし。ELSE 変更の種類で BR4.2〜BR4.4
    violation: なし
    source: [FR4.3, U1 の規則 5.3]
  - id: BR4.2
    statement: キー名の変更とタグの削除は、キーが当てはまれば値に関係なく影響あり。大文字・小文字だけの名前の変更を影響なしとするのは、aws:ResourceTag の条件キーだけとする
    category: calculation
    applies_to: ImpactFinding.verdict
    trigger: BR4.1 で renameKey か removeKey
    logic: IF renameKey AND location = conditionKey AND tagKind = ResourceTag AND 前置が aws AND 小文字(fromKey) = 小文字(toKey) THEN 影響なし。ELSE affected。変数の参照(BR2.4。location が keyVariable、conditionValue、resource、notResource)はこの例外を受けず、キーが当てはまれば常に affected
    violation: なし
    source: [FR4.1, FR4.3, AC3.2.1, AC3.2.3, U1 の規則 5.3, AWS のドキュメント]
  - id: BR4.3
    statement: 値の変更は、条件の値と、変更前か変更後の値を比べる。条件の値や Resource の中の変数の参照では判定できないとする
    category: calculation
    applies_to: ImpactFinding.verdict
    trigger: BR4.1 で changeValue
    logic: >
      次の順に判定する。
      (1) IF location が keyVariable、conditionValue、resource、notResource THEN undecided(実際の値は評価のときまで分からない)。
      (2) ELSE IF 演算子が Null THEN 影響なし(タグがあるかないかは変わらない)。
      (3) ELSE 比べる値 = fromValue と toValue とし、BR4.7 で条件の値と比べる。当てはまる値があれば affected、なければ、値の中に ${ があるか演算子がその他なら undecided、そうでなければ影響なし。
      IfExists と ForAnyValue:、ForAllValues: は、値の変更の前後ともタグが有るため、判定を変えない
    violation: なし
    source: [FR4.3, AC3.2.2, Q4, Q6]
  - id: BR4.4
    statement: 必須タグの追加は、タグが無い状態から付ける値への変更として判定する
    category: calculation
    applies_to: ImpactFinding.verdict
    trigger: BR4.1 で addRequired
    logic: >
      次の順に判定する。
      (1) IF location が keyVariable、conditionValue、resource、notResource THEN affected(参照していたタグが無い状態から有る状態に変わるため)。
      (2) ELSE IF 演算子が Null THEN affected。
      (3) ELSE IF 演算子に IfExists が付く、または ForAnyValue: か ForAllValues: が付く THEN affected(タグが無いときの結果は値に関係なく決まり、付いたあとは値で決まるため、付ける値に関係なく結果が変わりうる)。
      (4) ELSE 比べる値 = toValue とし、BR4.7 で条件の値と比べる。当てはまれば affected、なければ、値の中に ${ があるか演算子がその他なら undecided、そうでなければ影響なし
      (修飾子のない演算子は、タグが無いとき、否定のないものは false、否定のものは true になり、付けた値が条件の値に当てはまらなければ結果は同じため)
    violation: なし
    source: [Q5, FR4.3]
  - id: BR4.5
    statement: TagKeys の参照は、条件の値をタグのキーとして変更のキーと比べる。値の変更は影響なし
    category: calculation
    applies_to: ImpactFinding.verdict
    trigger: BR4.1 で tagKind が TagKeys
    logic: >
      次の順に判定する。
      (1) IF changeValue THEN 影響なし。
      (2) ELSE IF renameKey か addRequired AND (演算子が否定のもの OR ForAllValues: が付く) THEN affected(許すキーの一覧として使われる形で、新しいキー(toKey)が一覧に無ければ、そのキーを付ける操作の結果が変わるため。一覧にあっても一覧の書き直しが要りうる)。
      (3) ELSE 比べる値 = 変更のキー(BR4.1)とし、BR4.7 で条件の値と比べる。当てはまれば affected、なければ、値の中に ${ があるか演算子がその他なら undecided、そうでなければ影響なし
    violation: なし
    source: [FR4.4, AC3.2.6]
  - id: BR4.6
    statement: 条件キーのタグのキーにポリシー変数があれば、結び付けた変更はすべて判定できない
    category: policy
    applies_to: ImpactFinding.verdict
    trigger: BR4.1 でタグのキーに ${ がある
    logic: BR3 で結び付けた変更をすべて undecided とする。ただし BR4.2 の大文字・小文字だけの名前の変更(aws:ResourceTag)は影響なし。変数の中のキーが分かっても、評価のときまで実際のキーは決まらない
    violation: なし
    source: [FR4.5, AC3.2.4, U1 の規則 5.4]
  - id: BR4.7
    statement: 条件の値の比べ方は、演算子の種類に合わせる
    category: calculation
    applies_to: Condition.values
    trigger: BR4.3、BR4.4、BR4.5
    logic: >
      条件の値ごとに、${ を含む値は比べない(判定できないものとして数える)。
      等しい = 文字列が同じ。大文字・小文字なし = 大文字・小文字を区別せずに同じ。パターン = * と ? を使うパターンとして当てはまる(大文字・小文字を区別する)。
      ただし TagKeys の値(タグのキー)は、等しいとパターンでも大文字・小文字を区別せずに比べる(見逃さない側に倒す)。
      否定の演算子(StringNotEquals など)も、否定のない演算子と同じに比べる。
      演算子がその他なら比べず、判定できないものとして数える
    violation: なし
    source: [Q4, FR4.5]

  # 5. 結果
  - id: BR5.1
    statement: 指摘は(ポリシー、ステートメント、演算子、条件キー、判定)ごとに1件とし、当てはまる変更の changeId をすべて持つ
    category: calculation
    applies_to: ImpactFinding
    trigger: 照合のあと
    logic: >
      条件の参照(条件キーと条件の値)は、その条件の(演算子、条件キー)にまとめる。conditionValue は条件の値を「, 」でつないだもの。
      Resource と NotResource の参照は、conditionOperator を空、conditionKey を "Resource" か "NotResource" にしてまとめ、conditionValue は Resource(または NotResource)のすべての値を文書の順に「, 」でつないだものとする。
      conditionOperator は保存した変更案と JSON の出力にも書く(契約 C2 に省略できる項目として足す)。
      同じまとまりで、ある変更が affected と undecided の両方になったら affected だけに入れる。changeIds は変更の順で重ねない。
      要約の affected と undecided の数は指摘の件数とする(U1 の規則 5.6)
    violation: なし
    source: [FR4.6, AC3.2.1, Q6, U1 の規則 5.6]
  - id: BR5.2
    statement: 結果の順は、毎回同じにする
    category: calculation
    applies_to: ImpactResult
    trigger: 照合のあと
    logic: 指摘は次の鍵の順に並べる。(1) sourceKind の順(iamManaged、iamInline、s3Bucket、kmsKey、sqsQueue、snsTopic、secretsManagerSecret)、(2) policyRef の文字列の順、(3) 文書の中のステートメントの順、(4) 場所の種類の順(条件、Resource、NotResource)、(5) conditionOperator の文字列の順、(6) conditionKey の文字列の順、(7) verdict の順(affected、undecided)。unreadPolicies は sourceKind の順、region の文字列の順(空が先)、policyRef の文字列の順(空が先)
    violation: なし
    source: [FR4.6, U4 の規則 1.8]
  - id: BR5.3
    statement: 影響分析は読み取りの API だけを呼び、読み取り用の IAM ポリシーの例に足した操作を書く
    category: constraint
    applies_to: ImpactAnalyzer、AwsAccess
    trigger: 影響分析の実行
    logic: >
      足す操作: iam:GetAccountAuthorizationDetails(IAM の管理ポリシーとインラインポリシーのすべて。U1 の iam:ListPolicies と iam:GetPolicyVersion はこれに置き換える。2026-10-28)、s3:ListAllMyBuckets、s3:GetBucketPolicy、kms:ListKeys、kms:DescribeKey、
      kms:GetKeyPolicy、s3:GetBucketLocation(一覧の答えにリージョンが無いバケットだけ)、sqs:ListQueues、sqs:GetQueueAttributes、sns:ListTopics、sns:GetTopicAttributes、secretsmanager:ListSecrets、
      secretsmanager:GetResourcePolicy。README の読み取り用の例と、その例が実際の呼び出しをすべて含むことを確かめるテスト(U2)に加える
    violation: 書き込みの API を呼べば NFR3 の違反
    source: [NFR3, AC1.2.1, FR7.7]
  - id: BR5.4
    statement: Ctrl+C などで止められたら、途中の結果を返さずに止める(U1 のまま)
    category: policy
    applies_to: ImpactAnalyzer
    trigger: 照合の途中で ctx が取り消されたとき
    logic: IF ctx が取り消された THEN エラーを返す。読めなかったポリシーとしては扱わない
    violation: plan は中断として終わる(U4 のまま)
    source: [U1 の規則 5.5, U4 の NFR の設計]
```

## 規則のまとめ

| ID | 規則 | 種類 | 出典 |
|---|---|---|---|
| BR1.1 | 付いている(権限の上限を含む)、利用者が作った管理ポリシーを読む | constraint | FR4.1 |
| BR1.2 | ユーザー、ロール、グループのインラインポリシーを読む | constraint | FR4.1、Q2 |
| BR1.3 | 5つのサービスの一覧の API でリソースを探し、ポリシーを読む | constraint | FR4.2、Q1 |
| BR1.4 | 読まないもの(信頼ポリシー、AWS が管理するポリシーなど)を README に書く | constraint | CON3、Q2 |
| BR1.5 | 読めなければ種類と理由を示し、不完全にする | policy | FR4.7 |
| BR1.6 | policyRef の形(インラインは `ARN#名前`、リソースポリシーはリソースの ARN) | calculation | FR4.6 |
| BR1.7 | IAM の文書だけ URL エンコードを戻す | validation | FR4.2 |
| BR2.1 | `ResourceTag/`、`RequestTag/`、`PrincipalTag/`、`TagKeys` の条件キー | calculation | FR4.4 |
| BR2.2 | タグのキーは大文字・小文字なし、`*` `?` はパターン(機械的に判定する)、`${` は判定できない | calculation | FR4.5 |
| BR2.3 | 演算子の種類(`ForAnyValue:` などと `IfExists` を外す) | calculation | Q4 |
| BR2.4 | 条件キーの中、条件の値、`Resource`、`NotResource` の中のポリシー変数 | calculation | Q6 |
| BR3.1 | IAM のポリシーは、そのキーのすべての変更に結び付ける | calculation | Q3 |
| BR3.2 | リソースポリシーは、そのリソース自身の変更だけ | calculation | Q3 |
| BR3.3 | `PrincipalTag` は IAM のユーザーとロールの変更だけ | calculation | Q3 |
| BR4.1 | 変更のキーと、キーが当てはまるかの判定 | calculation | FR4.3 |
| BR4.2 | キー名の変更と削除は影響あり(`aws:ResourceTag` の大文字・小文字だけの変更を除く) | calculation | FR4.3 |
| BR4.3 | 値の変更は `Null` を影響なしとし、ほかは条件の値と比べる。変数の参照は判定できない | calculation | Q4、Q6 |
| BR4.4 | 必須タグの追加は「無い状態から付ける値へ」。`Null`、`IfExists`、`ForAnyValue:`・`ForAllValues:` は影響あり | calculation | Q5 |
| BR4.5 | `TagKeys` は値をキーとして比べる。否定と `ForAllValues:` は新しいキーで影響あり | calculation | FR4.4 |
| BR4.6 | 条件キーの中の変数は判定できない | policy | FR4.5 |
| BR4.7 | 値の比べ方は演算子に合わせる | calculation | Q4 |
| BR5.1 | 指摘のまとめ方(判定ごとに1件)と `conditionOperator` の保存 | calculation | FR4.6 |
| BR5.2 | 結果の順(7つの鍵) | calculation | FR4.6 |
| BR5.3 | 読み取りの API だけ。読み取り用の例に操作を足す | constraint | NFR3 |
| BR5.4 | 止められたら途中の結果を返さない | policy | U1 の規則 5.5 |
