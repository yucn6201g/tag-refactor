# 業務の規則: U1 最小の一本

U1 の `scan` と `plan` が従う規則です。出典は要件(FR・NFR)、契約(C1〜C12)、`functional-design-questions.md` の回答(Q1〜Q5)です。

```yaml
rules:
  # 1. 接続と安全
  - id: BR1.1
    statement: 読むリージョンは、--region があればそれ、なければ AWS の標準の設定(プロファイル、環境変数)のリージョンとする
    category: policy
    applies_to: AccountContext.region
    trigger: scan と plan の開始時
    logic: IF --region がある THEN それを使う ELSE IF 標準の設定にリージョンがある THEN それを使う ELSE 誤りとする。IF 指定したリージョンがアカウントの有効なリージョンにない THEN 誤りとする。指定したリージョンの STS が認証情報を拒否したときは、us-east-1 で有効なリージョンを確かめ直し、一覧になければ「そのリージョンは使えない」とする(有効にしていないオプトインのリージョンを、認証情報の誤りと取り違えないため。コードのレビュー R-02)
    violation: InvalidInput。「リージョンが決まらない」または「そのリージョンは使えない」ことと、--region か AWS_REGION で指定する方法を示し、終了コード 1
    source: [Q1, FR7.6]
  - id: BR1.7
    statement: U1 が受け付けるオプションは、scan では --profile と --region、plan では --rules、--profile、--region とし、すべてのサブコマンドで --debug を受け付ける(NFR 要件 Q3)。契約 C5 にあるが U1 で扱わないオプション(--format、--lang、--out など)は受け付けない
    category: validation
    applies_to: Cli
    trigger: コマンドの解釈
    logic: IF 上の一覧にないオプション THEN 誤りとする。IF C5 にあるが U1 で扱わないオプション THEN 「この版では使えない」と案内する
    violation: InvalidInput。終了コード 1
    source: [C5, Q1, review-01 R-01]
  - id: BR1.2
    statement: --profile があれば、そのプロファイルの認証情報とリージョンを使う
    category: policy
    applies_to: AccountContext.profile
    trigger: scan と plan の開始時
    logic: IF --profile がある THEN そのプロファイルを使う ELSE 標準の設定の順に従う
    violation: プロファイルが見つからなければ InvalidInput。プロファイル名を示し、終了コード 1
    source: [FR7.6, AC1.2.4]
  - id: BR1.3
    statement: 認証情報が見つからなければ、何が足りないかと設定方法を示して終える
    category: validation
    applies_to: AccountContext
    trigger: アカウントの確認(STS で呼び出し元を確かめる)の前
    logic: IF 認証情報が見つからない、または期限が切れている(SSO や一時的な認証情報の期限切れ) THEN CredentialsMissing
    violation: 2行の案内(足りないもの、または期限切れであることと、設定や再ログインの方法)を標準エラーに出し、終了コード 1
    source: [FR7.8, AC1.2.3, C12]
  - id: BR1.4
    statement: 認証情報(アクセスキー、シークレット、セッショントークン)を、標準出力、標準エラー、ログのどこにも出さない
    category: policy
    applies_to: すべての出力とエラーの文言
    trigger: 出力のたび
    logic: IF 出力する文字列に認証情報が含まれうる THEN それを含めない(AWS の SDK のエラー文をそのまま出さず、種類と API 名だけを出す)
    violation: 起きてはならない。テストで確かめる
    source: [NFR8, AC1.2.5]
  - id: BR1.5
    statement: scan と plan には読み取りの口だけを渡し、書き込みの API を呼ばない
    category: authorization
    applies_to: scan、plan
    trigger: コマンドの組み立て時
    logic: scan と plan では TagReader と PolicyReader だけを作り、TagWriter は作らない
    violation: 起きてはならない。読み取り用の IAM ポリシーだけで動くことを検証用アカウントで確かめる
    source: [NFR3, AC1.2.1, C6]
  - id: BR1.6
    statement: 権限が足りないときは、足りない操作(API)の名前を示す
    category: validation
    applies_to: AWS の呼び出し
    trigger: AWS が拒否を返したとき
    logic: IF 拒否された THEN AccessDenied に API の名前(例 tag:GetResources)を付けて返す
    violation: タグの読み取りが拒否されたら、U1 では結果を出せないため終了コード 1。ポリシーの読み取りの拒否は BR5.5
    source: [FR7.7, AC1.2.2]
  - id: BR1.8
    statement: 一時的な失敗は、AWS へのアクセスの部品の既定の再試行だけで扱い、使い切ったら Transient として終える。待ち時間を延ばしながらの再試行と呼び出し回数の上限への対応は U2 で強める
    category: policy
    applies_to: AwsAccess
    trigger: AWS の呼び出しが一時的に失敗したとき
    logic: IF 既定の再試行の後も失敗 THEN Transient
    violation: 終了コード 1
    source: [C6, C12, review-01 R-13]

  # 2. scan
  - id: BR2.1
    statement: 読むリージョンのタグの付いたリソースを、タグの共通の API で集める。一度もタグが付いていないリソースは U1 では集めない
    category: constraint
    applies_to: InventorySnapshot
    trigger: scan と plan の収集
    logic: 読むリージョンのタグの付いたリソースをすべて読む(ページが分かれていても最後まで読む)。API が返したリソースは、タグが空でも、aws: のタグしかなくても、「調べたリソース」に数える。resourceType は ARN のサービスと種類の部分(例 ec2:instance)とする
    violation: 読み取りに失敗したら BR1.6
    source: [FR1.1 の最小版, C7, review-01 R-10]
  - id: BR2.2
    statement: キーの正規化は、小文字にし、区切り文字(-、_、空白)を取り除いたものとする。正規化したキーが同じキーを1つのまとまりにする
    category: calculation
    applies_to: KeyVariantGroup.normalizedKey
    trigger: scan の結果の作成
    logic: normalizedKey = 小文字(キー) から -、_、空白 を除いたもの。aws: で始まるキーは対象にしない
    violation: なし
    source: [Q2, FR1.6 の最小版]
  - id: BR2.3
    statement: まとまりに2つ以上の書き方があれば、表記ゆれとして示す。件数は、書き方ごとのリソース数と、まとまり全体のリソース数(1つのリソースは1回)とする
    category: calculation
    applies_to: KeyVariantGroup
    trigger: scan の結果の作成
    logic: IF spellings の数 >= 2 THEN isVariant = true。totalResources は、どれかの書き方を持つリソースの重複のない数。表示の順は、まとまりは totalResources の多い順、まとまりの中は件数の多い順で、同じ件数ならキーの文字列の順とする
    violation: なし
    source: [Q2, FR1.6, FR3.2 の数え方, review-01 R-11]
  - id: BR2.4
    statement: 表記ゆれがなければ「問題なし」と、調べたリソースの件数を示す
    category: policy
    applies_to: scan の結果
    trigger: scan の結果の表示
    logic: IF 表記ゆれが0件 THEN 「No issues found」と件数を出す
    violation: なし
    source: [FR1.8]

  # 3. ルールファイル(最小版)
  - id: BR3.1
    statement: ルールファイルは version 1 で、U1 では keys の各キーの aliases だけを使う。契約 C1 にある他の項目(values、required、remove、when)は読めるが使わず、警告する。契約にないキーは誤りとする
    category: validation
    applies_to: RuleSet
    trigger: plan の開始時(ルールファイルの読み込み)
    logic: IF ファイルがない THEN NotFound。IF 読めない THEN StorageFailed。IF 空、または YAML として壊れている THEN InvalidInput(行と直し方)。IF version が 1 でない THEN UnsupportedVersion。IF 知らないキー、または型の誤り THEN InvalidInput(行、キー、直し方)。IF values、required、remove がある THEN 警告「この版では使わない」。IF keys がない、または空 THEN 警告「no key rules」(コードのレビュー R-07)
    violation: 誤りは標準エラーにファイル名、行番号、キー、直し方を出し、終了コード 1
    source: [C1, C12, FR2.5 の最小版, 契約の決まり(C1 では知らないキーを誤りにする), review-01 R-07]
  - id: BR3.2
    statement: 1つの別名は1つの正しいキーにだけ属する。別名は、どの正しいキーとも同じ文字列であってはならない。比べるときは大文字・小文字を区別する。正しいキーどうしが、BR2.2 の正規化で同じになる場合は警告する
    category: validation
    applies_to: KeyRule.aliases
    trigger: ルールファイルの読み込み
    logic: IF 同じ別名が2つの正しいキーにある、または別名がどれかの正しいキーと同じ THEN InvalidInput。IF 正規化した正しいキーが重なる THEN 警告
    violation: 誤りは関係する行番号をすべて示し、直し方を示して終了コード 1。警告は続ける
    source: [AS1, C1, review-01 R-08]
  - id: BR3.3
    statement: aws: で始まるキーは、正しいキーにも別名にも使えない。書かれていたら、その規則を無視して警告する
    category: policy
    applies_to: KeyRule
    trigger: ルールファイルの読み込み
    logic: IF キーまたは別名が aws: で始まる THEN それを除き、warnings に行とキーを加える
    violation: 警告だけ。終了コードは変えない
    source: [FR5.9, 契約の全体の決まり(aws: のタグ)]

  # 4. plan(名前の変更)
  - id: BR4.1
    statement: リソースに別名のキーが1つだけ付いていて、正しいキーが付いていなければ、別名から正しいキーへの名前の変更を作る
    category: calculation
    applies_to: Change
    trigger: plan の変更案の作成
    logic: リソースごと、正しいキーごとに、付いているキー = {正しいキー, 別名} のうち付いているもの。IF 付いているのが別名1つだけ THEN renameKey(fromKey=別名, toKey=正しいキー, value=その値)
    violation: なし
    source: [FR2.2, FR3.1 の最小版, AC2.1.2]
  - id: BR4.2
    statement: 付いているキーが2つ以上(正しいキーと別名、または別名どうし)なら、値がすべて同じときだけ、別名を正しいキーにまとめる。値が1つでも違えば変えずに「ぶつかり」として示す
    category: calculation
    applies_to: Change、KeyConflict
    trigger: plan の変更案の作成
    logic: IF 付いているキーの値がすべて同じ THEN 別名ごとに renameKey を作る ELSE KeyConflict を1件作り、そのキーの変更は作らない。別名の順はルールファイルの aliases の順とする。mergesIntoExisting は「その変更を当てる時点で、toKey が既にある」ことを表し、正しいキーが付いていれば全件 true、付いていなければ最初の別名だけ false で2つ目以降は true とする
    violation: ぶつかりは要確認(終了コード 2)
    source: [Q3, review-01 R-09]
  - id: BR4.3
    statement: 変更には変更案の中で一意の changeId を付ける。変更されるリソースの総数は、1つのリソースを1回だけ数える
    category: calculation
    applies_to: Change.changeId、Plan
    trigger: plan の変更案の作成
    logic: changeId は作った順に c-000001 から振る。作る順は、リソースの ARN の文字列の順、その中は正しいキーの文字列の順とする(毎回同じ結果にするため)。総数 = changes の resourceArn の重複のない数
    violation: なし
    source: [C2, FR3.2]
  - id: BR4.4
    statement: 変更が1件もなく、ぶつかりもなければ、「変更は不要」と調べたリソースの件数を示し、影響分析は行わない
    category: policy
    applies_to: Plan
    trigger: plan の変更案の作成の後
    logic: IF changes と conflicts がどちらも空 THEN ポリシーを読まずに「No changes needed」と件数を出す
    violation: なし(終了コード 0)
    source: [FR1.8 の考え方, review-01 R-05]

  # 5. 影響分析(最小版)
  - id: BR5.1
    statement: 読むポリシーは、利用者が作った管理ポリシーのうち、ユーザー、ロール、グループのどれかに付いているもの。中身は既定の版を読む
    category: constraint
    applies_to: PolicySource
    trigger: plan の影響分析
    logic: 利用者が作った管理ポリシーの一覧(ページが分かれていても最後まで読む)から、どこかに付いているものだけを選び、既定の版の文書を読む
    violation: 読めなければ BR5.5
    source: [Q4, FR4.1 の最小版, review-01 R-13]
  - id: BR5.2
    statement: 照合する条件キーは、リソースのタグの条件キーだけとする。条件キーは「<前置>:ResourceTag/<タグのキー>」の形で、前置は aws か、英小文字・数字・ハイフンからなるサービス名(例 ec2、s3)とする。前置と ResourceTag の綴りは大文字・小文字を区別せずに比べる。タグのキーも、大文字・小文字を区別せずに比べる
    category: calculation
    applies_to: ImpactFinding.conditionKey
    trigger: plan の影響分析
    logic: 条件キーを最初の「:」で分け、後ろが大文字・小文字なしで「ResourceTag/」で始まるものを取り出す。その後ろをタグのキーとする。例 aws:ResourceTag/env、ec2:ResourceTag/env、s3:ResourceTag/env はどれも対象
    violation: なし
    source: [FR4.4 の最小版, AWS のドキュメント(aws:ResourceTag/tag-key: Tag keys are not case-sensitive), review-01 R-02]
  - id: BR5.3
    statement: 名前の変更は、条件のタグのキーが fromKey か toKey と(大文字・小文字を区別せずに)同じなら影響あり(affected)とする。fromKey と toKey が大文字・小文字だけ違う変更を影響なしとするのは、前置が aws の条件キーだけとする。サービス固有の条件キーでは、大文字・小文字だけの変更も影響ありとする(見逃さない側に倒す)
    category: calculation
    applies_to: ImpactFinding
    trigger: plan の影響分析
    logic: IF 小文字(条件のキー) が 小文字(fromKey) とも 小文字(toKey) とも違う THEN 影響なし ELSE IF 前置が aws AND 小文字(fromKey) = 小文字(toKey) THEN 影響なし ELSE affected
    violation: なし
    source: [FR4.1, FR4.3, AC3.2.1, AWS のドキュメント, review-01 R-03]
  - id: BR5.4
    statement: 条件のタグのキーにポリシー変数(${...})が含まれる場合は、どの変更についても判定できない(undecided)とする。ワイルドカード(*、?)だけを含む場合は、大文字・小文字を区別せずに fromKey と toKey に当てはめ、どちらかに当てはまれば affected、どちらにも当てはまらなければ影響なしとする
    category: policy
    applies_to: ImpactFinding.verdict
    trigger: plan の影響分析
    logic: IF タグのキーに "${" がある THEN すべての変更に undecided(前置が aws で大文字・小文字だけの変更を除く) ELSE IF "*" か "?" がある THEN パターンとして fromKey と toKey に照合し、当てはまれば BR5.3 と同じ扱い
    violation: なし
    source: [FR4.5, AC3.2.4, C9, review-01 R-12]
  - id: BR5.5
    statement: ポリシーの一覧や中身を読めなかった場合は、読めなかった種類と理由を示し、影響分析を不完全とする
    category: policy
    applies_to: UnreadPolicy、Plan.impactAnalysisComplete
    trigger: ポリシーの読み取りが拒否または失敗したとき
    logic: IF 読めなかった THEN unreadPolicies に加え(個別のポリシーが読めなかった場合はその ARN も)、impactAnalysisComplete = false。読めた分の照合は続ける
    violation: 結果の先頭に「Impact analysis is incomplete」を出し、終了コード 1
    source: [FR4.7, AC3.2.5]
  - id: BR5.6
    statement: 影響ごとに、ポリシーの ARN、ステートメント(Sid か番号)、条件キーと値、原因の変更を示す。影響のある変更には impacted を付ける
    category: policy
    applies_to: ImpactFinding、Change.impacted
    trigger: plan の結果の作成
    logic: (ポリシー、ステートメント、条件の演算子、条件キー)ごとに1件の ImpactFinding を作り、当てはまる変更の changeId をすべて changeIds に入れる。値が複数あれば「, 」でつないで conditionValue にする。その変更の impacted = true。要約の affected と undecided の数は ImpactFinding の件数とする。表示の順はポリシーの ARN、ステートメントの順とする
    violation: なし
    source: [FR4.6, C2, C9, review-01 R-11]

  # 6. 出力と終了コード
  - id: BR6.1
    statement: 結果は標準出力に、進み具合・案内・警告・エラーは標準エラーに出す
    category: policy
    applies_to: すべての出力
    trigger: 出力のたび
    logic: 標準出力には、結果の表と「Summary」の行だけを出す。アカウントとリージョンの行、進み具合、警告、エラー、調べた範囲の「Checked:」の行(コードのレビュー R-04)、「Exit code」の行は標準エラーに出す
    violation: 起きてはならない。標準出力をファイルに保存して確かめる
    source: [FR7.5, AC3.1.8, C5, review-01 R-04]
  - id: BR6.2
    statement: U1 の出力は、端末向けの一覧の形と英語だけとする。意味は記号と文字で表し、色は使わない
    category: constraint
    applies_to: Display
    trigger: 出力のたび
    logic: 記号は ~(名前の変更)、!(影響あり)、?(判定できない)を使う
    violation: なし
    source: [FR7.3, モックアップ 画面2・画面4, unit-of-work.md U1]
  - id: BR6.3
    statement: 終了コードは、エラーか不完全なら 1、そうでなく要確認(表記ゆれ、変更、ぶつかり、影響)があれば 2、どちらでもなければ 0 とする
    category: calculation
    applies_to: CommandOutcome.exitCode
    trigger: コマンドの終わり
    logic: IF error か incomplete THEN 1 ELSE IF needsReview THEN 2 ELSE 0
    violation: なし
    source: [FR7.4, Q2, Q5, モックアップ §1, C5]
```

## 規則のまとめ

| ID | 規則 | 種類 | 出典 |
|---|---|---|---|
| BR1.1 | リージョンは --region、なければ標準の設定 | policy | Q1、FR7.6 |
| BR1.2 | --profile のプロファイルを使う | policy | FR7.6 |
| BR1.3 | 認証情報がなければ案内して終了コード 1 | validation | FR7.8 |
| BR1.4 | 認証情報をどこにも出さない | policy | NFR8 |
| BR1.5 | scan と plan は読み取りの口だけを使う | authorization | NFR3 |
| BR1.6 | 権限不足は足りない API の名前を示す | validation | FR7.7 |
| BR1.7 | U1 が受け付けるオプションを限る | validation | C5 |
| BR1.8 | 一時的な失敗は既定の再試行だけ。使い切ったら Transient | policy | C6、C12 |
| BR2.1 | 1リージョンのタグの付いたリソースを集める | constraint | FR1.1 の最小版 |
| BR2.2 | 小文字にし、区切り文字を除いてキーをまとめる | calculation | Q2 |
| BR2.3 | 2つ以上の書き方があれば表記ゆれ。リソースは1回だけ数える | calculation | Q2、FR1.6 |
| BR2.4 | 表記ゆれがなければ「問題なし」と件数 | policy | FR1.8 |
| BR3.1 | ルールファイルは version 1。U1 は aliases だけ使う。知らないキーは誤り | validation | C1、FR2.5 |
| BR3.2 | 別名は1つの正しいキーにだけ属する | validation | AS1 |
| BR3.3 | aws: のキーは無視して警告 | policy | FR5.9 |
| BR4.1 | 別名だけが付いていれば名前の変更を作る | calculation | FR2.2 |
| BR4.2 | キーがぶつかれば、値が同じならまとめ、違えば「ぶつかり」 | calculation | Q3 |
| BR4.3 | changeId を決まった順に振り、リソースは1回だけ数える | calculation | C2、FR3.2 |
| BR4.4 | 変更もぶつかりもなければ「変更は不要」。影響分析はしない | policy | FR1.8 |
| BR5.1 | 付いている、利用者が作った管理ポリシーを最後まで読む | constraint | Q4、FR4.1 |
| BR5.2 | `<前置>:ResourceTag/<キー>` だけを、大文字・小文字を区別せずに比べる | calculation | FR4.4、AWS のドキュメント |
| BR5.3 | fromKey か toKey を使う条件は影響あり。大文字・小文字だけの変更を影響なしとするのは aws: の条件キーだけ | calculation | FR4.1、FR4.3 |
| BR5.4 | ポリシー変数は判定できない。ワイルドカードはパターンとして照合する | policy | FR4.5 |
| BR5.5 | 読めなかったポリシーがあれば不完全(終了コード 1) | policy | FR4.7 |
| BR5.6 | 影響ごとにポリシー、ステートメント、条件、原因の変更を示す | policy | FR4.6 |
| BR6.1 | 標準出力は表と Summary の行だけ。アカウントの行と Exit code の行は標準エラー | policy | FR7.5 |
| BR6.2 | U1 は一覧の形と英語だけ。記号で意味を表す | constraint | FR7.3 |
| BR6.3 | 終了コードは 1 > 2 > 0 の順 | calculation | FR7.4、Q5 |
