# 業務の規則: U8 IaC への反映

`iac` の規則です。ID はこの作業単位の中での番号です。ほかの作業単位の規則は「U4 の規則 3.2」のように書きます(その作業単位の `rules.md` の ID から「BR」を省いた番号です)。

出典: 要件 FR8.1・FR8.2・AS7・OQ3、ストーリー US7.1(AC7.1.1、AC7.1.2)、契約 C2・C5・C12 と全体の決まり(`aws:` で始まるタグ)、`functional-design-questions.md` Q1〜Q9、U4 の規則(変更案の4種類の変更と `mergesIntoExisting`)。

変更案の変更の項目名は契約 C2 のとおりです: renameKey は `fromKey`・`toKey`・`fromValue`・`mergesIntoExisting`、changeValue は `fromKey`・`fromValue`・`toValue`、addRequired は `toKey`・`toValue`、removeKey は `fromKey`・`fromValue`。

```yaml
rules:
  # 1. 入力
  - id: BR1.1
    statement: iac は --plan と --dir を必ず受け取る
    category: validation
    applies_to: IacRequest
    trigger: iac の開始
    logic: IF --plan がない、または --dir がない THEN 使い方の誤り(InvalidInput)。IF --dir が存在するフォルダーでない THEN InvalidInput。ほかのオプションは --tf-state、--write と、全体のオプションの --lang、--debug(Q1、Q2、Q6)。--profile は受け付けない(AWS を呼ばないため。Q8)
    violation: 何もせずに終了コード 1。正しい使い方を示す
    source: [FR8.1, FR8.2, 契約 C5, Q2, Q6, Q8]
  - id: BR1.2
    statement: 変更案は plan --out が書いた JSON で、版が読めるものだけを使う
    category: validation
    applies_to: IacRequest.planPath
    trigger: 入力の確かめのあと
    logic: U4 の変更案の読み込み(契約 C2)を使う。IF ファイルがない THEN NotFound。IF version が読めるものでない THEN UnsupportedVersion。IF 形が壊れている THEN InvalidInput。変更案のアカウントと今のアカウントは比べない(AWS を呼ばないため。Q8)
    violation: 何もせずに終了コード 1
    source: [契約 C2, C12, Q8]
  - id: BR1.3
    statement: --tf-state は terraform show -json の出力として読む
    category: validation
    applies_to: IacRequest.tfStatePath
    trigger: --tf-state があるとき、変更案を読んだあと
    logic: |
      最上位に format_version がある JSON を受け付ける。IF planned_values、resource_changes、prior_state のどれかがある THEN plan のファイルの出力なので InvalidInput で止め、state の出力(引数なしの terraform show -json)を渡すよう案内する。values がなければ、リソースのない state として扱う(terraform show -json は空の state で values を出さないため)。values.root_module と、その child_modules(入れ子を含む)の resources のうち、mode が managed のものを読み、各リソースの address と values.arn を取る。values.arn のないリソースは対応に入れない(BR2.3)。
      IF ファイルがない THEN NotFound。IF format_version がない(state のファイルそのもの、ほかの JSON) THEN InvalidInput で止め、terraform show -json の出力を渡すよう案内する
    violation: 何もせずに終了コード 1
    source: [FR8.1, AS7, Q2]

  # 2. どの IaC のリソースかを決める
  - id: BR2.1
    statement: 変更が1つ以上あるリソースだけを反映の対象にする
    category: constraint
    applies_to: IacTarget
    trigger: 変更案を読んだあと
    logic: 変更案の resources のうち changes が空でないものを IacTarget にする。変更案の影響の指摘(impactFindings)は使わない
    violation: (なし)
    source: [FR8.1, FR8.2, 契約 C2]
  - id: BR2.2
    statement: aws:cloudformation のタグがあるリソースは CloudFormation のリソースとする
    category: calculation
    applies_to: IacTarget.tool
    trigger: 対象を決めたあと
    logic: |
      IF beforeTags に aws:cloudformation:stack-name と aws:cloudformation:logical-id の両方がある THEN tool = cloudformation、stackName と logicalId をその値にする。
      IF aws:cloudformation:stack-name だけがある THEN tool = cloudformation とし、リソース全体を ManualItem(templateNotFound)にする(論理 ID が分からないため)。
      CloudFormation のリソースは、--tf-state にあっても Terraform としては扱わない
    violation: (なし)
    source: [FR8.2, 契約 C2(iacManagedBy), Q2]
  - id: BR2.3
    statement: --tf-state のリソースと ARN が完全に同じものは Terraform のリソースとし、どちらでもないものは IaC で管理されていないとする
    category: calculation
    applies_to: IacTarget.tool
    trigger: BR2.2 のあと
    logic: |
      IF --tf-state があり、state のリソースの values.arn が IacTarget の arn と文字列として完全に同じ THEN tool = terraform、address をそのアドレスにする。id や名前の一部では比べない(別の種類の同じ名前のリソースに当たらないため)。同じ ARN のリソースが state に2つ以上あれば ManualItem(ambiguous)。
      ほかは tool = none とし、unmanagedResources に数える(手で直すものには入れない。IaC で作っていないリソースが多い場合に一覧が長くなりすぎないため)。values.arn のない種類の Terraform のリソースも none になる
    violation: (なし)
    source: [FR8.1, AS7, Q2, Q9]
  - id: BR2.4
    statement: Terraform のモジュールの中のリソースと、count や for_each で作るリソースは書き換えない
    category: policy
    applies_to: IacTarget(tool = terraform)
    trigger: BR2.3 のあと
    logic: |
      IF address が module. で始まる THEN ManualItem(inModule)。モジュールのソースを変えると、そのモジュールを使うほかのリソースも変わるため。
      IF address が [ で始まる添字を持つ(例: aws_sqs_queue.q[0]、aws_sqs_queue.q["a"]) THEN ManualItem(multipleInstances)。同じブロックから作るほかのリソースも変わるため
    violation: (なし)
    source: [FR8.1, Q4]

  # 3. 場所を見つける
  - id: BR3.1
    statement: CloudFormation は --dir の下を、Terraform は --dir の直下だけを探し、作られたファイルや依存のフォルダーは見ない
    category: constraint
    applies_to: SourceLocation
    trigger: 場所を探す前
    logic: |
      CloudFormation の候補: --dir の下を再帰的に見て、拡張子が .yaml、.yml、.json、.template のファイル。名前が .git、.terraform、node_modules、cdk.out のフォルダーは見ない(cdk.out は CDK が作ったテンプレートで、書き換えても次の合成で上書きされるため)。
      Terraform の候補: --dir の直下の .tf のファイルだけ(ルートモジュール。サブフォルダーのモジュールは見ない。.tf.json は見ない)。--dir には terraform show -json を実行したルートモジュールのフォルダーを渡す。
      シンボリックリンクのファイルとフォルダーは、どちらも見ない(書き換えでリンクを普通のファイルに置き換えないため)。
      候補のファイルが読めない、または構文の誤りで解析できないときは、そのファイルを候補から外し、外したファイルの数を標準エラーに警告として出す(名前は --debug で示す)。外したファイルがあるときの templateNotFound と blockNotFound には、外したファイルの名前を detail に入れる
    violation: (なし)
    source: [FR8.1, FR8.2, Q2]
  - id: BR3.2
    statement: CloudFormation のリソースは、論理 ID とリソースの種類の両方が合うテンプレートの中から1つに決める
    category: calculation
    applies_to: IacTarget.location(tool = cloudformation)
    trigger: BR3.1 のあと
    logic: |
      最上位に Resources のマッピングがあり、その中のどれかの値が Type を持つファイルをテンプレートとする。Resources に logicalId のキーを持ち、その Type が変更案の resourceType と対応表で合う(例: AWS::SQS::Queue と sqs:queue、AWS::S3::Bucket と s3:bucket)テンプレートを集める。対応表は apply が書き換える種類(U6 の書き換えの対象の種類の表)の CloudFormation の Type で、表にない Type は合わないとする。
      IF 0個 THEN ManualItem(templateNotFound。論理 ID は同じでも種類が違うテンプレートがあれば、その名前を detail に入れる)。IF 1個 THEN それ。IF 2個以上 THEN ファイル名(拡張子を除く)が stackName と同じものが1つだけならそれ、ほかは ManualItem(ambiguous。候補のファイル名を detail に入れる)
    violation: (なし)
    source: [FR8.2, Q2, Q3]
  - id: BR3.3
    statement: Terraform のリソースは、アドレスと同じ resource ブロックの中から1つに決める
    category: calculation
    applies_to: IacTarget.location(tool = terraform)
    trigger: BR3.1 のあと
    logic: |
      address の種類と名前(aws_sqs_queue.orders なら resource "aws_sqs_queue" "orders")のブロックを、--dir の直下の .tf から集める。
      IF 0個 THEN ManualItem(blockNotFound。--dir にルートモジュールのフォルダーを渡すよう案内する)。IF 1個 THEN それ。IF 2個以上 THEN ManualItem(ambiguous)(Terraform は同じアドレスの定義の重複を許さないので、構成の誤り)
    violation: (なし)
    source: [FR8.1, Q2, Q3]

  # 4. タグの書き場所と書き換え
  - id: BR4.1
    statement: タグの書き場所は3つの形だけを扱い、書き場所そのものが式なら書き換えない
    category: validation
    applies_to: TagSite
    trigger: 場所を決めたあと
    logic: |
      CloudFormation: リソースの Properties の Tags。Key と Value のマッピングの並びなら cfnKeyValueList、キーと値のマッピングなら cfnMap(AWS::SSM::Parameter や AWS::Serverless の種類など)。
      Terraform: resource ブロックの直下の tags 属性で、値がオブジェクトの書き方({ ... })なら tfMap。
      IF Tags や tags が組み込み関数(!If、!Ref、Fn::If など)、変数、local、merge()、for 式などの式 THEN ManualItem(tagsNotLiteral)。
      IF YAML のアンカー(&)か別名(*)が、リソース、Properties、Tags、タグの要素、キーか値のどれかに付いている、または Properties か Tags の中にマージのキー(<<)がある THEN ManualItem(tagsNotLiteral)(アンカーで共有した書き方を変えると、別名で参照するほかのリソースも変わるため)。
      IF Terraform の tag ブロック(Auto Scaling グループなど)のように、3つの形のどれでもない THEN ManualItem(unsupportedTagShape)。
      Tags や tags がない場合は BR4.5
    violation: (なし)
    source: [FR8.1, FR8.2, Q4]
  - id: BR4.2
    statement: キーと値の両方が1行の文字列で直接書いてあるタグだけを書き換える
    category: validation
    applies_to: TagEntry.literal
    trigger: タグの書き場所を読むとき
    logic: |
      CloudFormation の YAML: Key と Value(cfnMap ではキーと値)が、タグ(!Sub、!Ref など)の付いていない1行のスカラー(引用符なし、' か " で囲んだもの)。ブロックスカラー(縦棒や > で始まる書き方)と、複数行にわたるスカラーは literal でない。
      CloudFormation の JSON: Key と Value が JSON の文字列。
      Terraform: キーが名前か引用符の文字列で、値が1行の引用符の文字列で補間(${ ... }、%{ ... })を含まないもの。ヒアドキュメントは literal でない。
      値は、書いてある文字列として読む: 引用符で囲んだものは引用符を外してエスケープを戻した文字列、引用符なしの YAML は書いてあるとおりの文字(Value: 100 は "100"、Value: yes は "yes"。YAML の型には読み替えない)。
      ほかは literal = false。literal = false のタグに変更が当たれば ManualItem(notLiteral)
    violation: (なし)
    source: [FR8.1, FR8.2, Q4]
  - id: BR4.3
    statement: 変更は、ソースが変更の前の状態なら反映し、すでに変更の後の状態なら反映済みとして数え、どちらでもなければ手で直す
    category: validation
    applies_to: Change、TagEntry
    trigger: 変更を当てる前
    logic: |
      まずリソース全体を見る: 変更案の変更が触るキー(fromKey と toKey)のすべてについて、ソースのタグが変更の後の状態(beforeTags に変更をすべて当てた値。消すキーはないこと)と同じなら、そのリソースの変更をすべて alreadyReflected にする(前に --write した、利用者が手で直したなど)。
      そうでなければ、変更を変更案の順に、ソースのタグの並び(前の変更を当てたあとの並び)に当てる。キーは大文字と小文字を区別して比べる。変更ごとに、前の状態(pre)なら反映し、後の状態(post)なら alreadyReflected、どちらでもなければ ManualItem にする:
        renameKey(mergesIntoExisting = false): pre は fromKey が fromValue で toKey がない。post は fromKey がなく toKey がある。fromKey も toKey もなければ keyNotInSource、fromKey の値が違えば valueDiffers、fromKey と toKey の両方があれば keyAlreadyExists。
        renameKey(mergesIntoExisting = true。変更案を作ったとき toKey がすでにあり、fromKey を消すだけの変更): pre は fromKey が fromValue。post は fromKey がない。toKey がソースにあるかは問わない(toKey がスタックのタグや default_tags で付いている場合も、AWS の変更と同じく fromKey を消すだけでよいため)。fromKey の値が違えば valueDiffers。
        changeValue: pre は fromKey が fromValue。post は fromKey が toValue。fromKey がなければ keyNotInSource、ほかの値なら valueDiffers。
        addRequired: pre は toKey がない。post は toKey が toValue。toKey がほかの値なら keyAlreadyExists(ソースにはあるのに AWS にはない。どちらが正しいかは人が決める)。
        removeKey: pre は fromKey が fromValue。post は fromKey がない。ほかの値なら valueDiffers。
      keyNotInSource のタグは、スタックに付けたタグ、プロバイダーの default_tags、別の仕組みで付いたタグの場合がある(理由の案内に書く)。
      removeKey と、mergesIntoExisting が true の renameKey で、fromKey がソースにないとき: 「前に消した(反映済み)」と「もともとソースになく、スタックのタグや default_tags で付いている」を、ソースだけでは区別できない。反映済み(alreadyReflected)として数え、手で直す一覧には入れないが、結果に「ソースにないため反映済みとみなした削除」としてリソースとキーを別に一覧にし、スタックのタグや default_tags で付けている場合はそちらを直すよう案内する(BR6.1。終了コードは変えない)。
      例: 同じリソースで env を Environment に(renameKey、false)、env2 を Environment に(renameKey、true)、Environment の値を Prod から prod に(changeValue)の3つ。ソースが env: Prod と env2: Prod なら、1つ目で env が Environment になり、2つ目で env2 を消し、3つ目で値が prod になる。2回目の実行では、ソースが Environment: prod だけなので、リソース全体が alreadyReflected になる
    violation: その変更を ManualItem にし、同じリソースのほかの変更は続ける
    source: [FR8.1, FR8.2, 契約 C2, Q4, Q7]
  - id: BR4.4
    statement: 4種類の変更を、元のテキストの範囲の差し替えで反映し、ほかの文字は変えない
    category: calculation
    applies_to: Edit
    trigger: BR4.3 で反映すると決めた変更
    logic: |
      Edit の作り方: 1つのリソースの変更は、まずメモリの上でタグの並びに順に当てる。そのあと、元の並びと当てたあとの並びを比べ、元の要素ごとに「キーの差し替え」「値の差し替え」「要素の削除」と、足した要素の「挿入」の Edit を作る。キーと値は別の範囲で、削除する要素にはキーや値の差し替えを作らないので、範囲は重ならない(env から Environment、Prod から prod は、同じ要素のキーと値の2つの Edit になる)。
      範囲の取り方: 解析器が返す位置をもとに、元のテキストのバイト位置で範囲を求める。範囲の中の文字が、解析で読んだキーや値の書き方と同じであることを確かめ、違えばそのファイルを書き換えない(ManualItem(unreadableFile))。YAML は github.com/goccy/go-yaml のトークンの位置、Terraform は HashiCorp の hclsyntax の範囲(新しい依存)を使う。Terraform も範囲の差し替えだけで書き、= の桁はそろえ直さない(terraform fmt は利用者が行う)。
      キーと値の書き方: 引用符の種類(なし、'、")は元のまま保ち、新しい文字列を同じ引用符の規則でエスケープする(' の中は ' を2つ、" の中は \ と " と制御文字をエスケープ)。元が引用符なしの YAML で、新しい文字列が引用符なしのまま書ける形(先頭が英字か数字で、英字・数字・_・.・/・@・- だけからなり、YAML 1.1 と 1.2 のどちらでも文字列に読めるもの。true・yes・on・off・null・~・数・日付などに読めるものは除く)でなければ、" で囲む。JSON は JSON の文字列で書く。Terraform は " の文字列で書き、\ と " と制御文字をエスケープし、${ を $${ に、%{ を %%{ にして補間にならないようにする。Terraform のキーは名前として書ける形でなければ " で囲む(同じエスケープ)。
      削除(removeKey、mergesIntoExisting の renameKey): 要素がそれぞれ別の行に書いてある書き方(YAML のブロックの並びとマッピング、複数行の JSON の配列とオブジェクト、複数行の tags = { ... })だけで行う。YAML と Terraform は要素の行(cfnKeyValueList は - の行から Key と Value のマッピングの終わりまで)を消す。JSON は要素とその前後のカンマを、配列が正しい JSON のまま残るように消す。消すと書き場所が空になるときは、空の書き方(Tags: []、"Tags": []、tags = {})に置き換える。
      追加(addRequired): BR4.5 のとおり。
      1行にまとめた書き方(YAML の流れの書き方 [ ... ] や { ... }、1行の JSON の配列、1行の tags = { ... })では、キーと値の差し替えだけを行い、削除と追加は ManualItem(unsupportedTagShape)にする。
      どの変更でも、コメント、ほかの行、並び、インデント、改行の種類(LF か CRLF)、ファイルの先頭の BOM は変えない(Q5)。aws: で始まるキーは足さず、変えず、消さない
    violation: (なし)
    source: [FR8.1, FR8.2, Q5, Q7]
  - id: BR4.5
    statement: 必須タグの追加は、要素がそれぞれ別の行に書いてある書き場所に、最後の要素の書き方を写して足す
    category: policy
    applies_to: Change(kind = addRequired)
    trigger: BR4.3 で反映すると決めた addRequired
    logic: |
      IF タグの書き場所がない、または空(Tags: []、tags = {})THEN ManualItem(noTagSite)(足す形とインデントを決めるもとがないため)。IF 書き場所が式 THEN ManualItem(tagsNotLiteral)。IF 1行にまとめた書き方 THEN ManualItem(unsupportedTagShape)。
      IF cfnKeyValueList のどれかの要素が Key と Value 以外の項目(Auto Scaling グループの PropagateAtLaunch など)を持つ THEN ManualItem(unsupportedTagShape)(足す要素に必要な項目が分からないため。変更と削除は行う)。
      ほかは、写すもとの要素を決める: 元の並びの要素のうち、キーと値が literal で、要素の行にコメントを含まないものの最後(削除する要素でもよい)。IF そのような要素がない THEN ManualItem(unsupportedTagShape)。
      写すもとの要素のテキスト(YAML と Terraform はその要素の行、JSON はその要素の { から } まで)をそのまま写し、キーと値の範囲だけを toKey と toValue に置き換えて(引用符とエスケープは BR4.4)、元の並びの最後の要素のすぐあとに挿入する。インデントと改行は写すもとの要素の行の始まりに合わせる。JSON では、元の並びの最後の要素の } のすぐあとに、カンマと、要素の間の空白(写すもとの要素の前の空白と同じ)と、写した要素を挿入する(最後の要素を削除する場合も、配列が正しい JSON になるよう BR4.6 で確かめる)。足す順は変更案の順
    violation: ManualItem にする
    source: [FR8.1, FR8.2, Q7]
  - id: BR4.6
    statement: 書き換えたあとのファイルを読み直して、意図した形になったことを確かめる
    category: validation
    applies_to: FileResult
    trigger: 1つのファイルの Edit をすべて当てたあと(書き出す前)
    logic: |
      書き換えたあとの内容をもう一度解析し、対象のリソースのタグの並びが、メモリの上で当てた並びと同じで、ほかのリソースと、CloudFormation の Resources の外(Terraform ならほかのブロックと、対象のブロックの tags 以外の属性)が変わっていないことを確かめる。
      IF 解析できない、または違う THEN そのファイルの変更をすべて取りやめ、そのファイルの変更を ManualItem(unreadableFile)にする。これはこのツールの不具合の疑いなので、--debug で詳細を示し、報告を案内する(対応しない書き方は BR4.1〜BR4.5 で先に unsupportedTagShape などにする)
    violation: そのファイルを書き換えない
    source: [FR8.1, FR8.2, Q5]

  # 5. 差分と書き換え
  - id: BR5.1
    statement: --write がなければファイルを書き換えず、差分だけを示す
    category: policy
    applies_to: IacRequest.write
    trigger: すべてのファイルの Edit を決めたあと
    logic: ファイルごとに unified diff(--dir からの相対パス、前後3行)を標準出力に出す。ファイルは読むだけで書き換えない
    violation: (なし)
    source: [Q6]
  - id: BR5.2
    statement: --write のときは、--dir が git の作業ツリーで、書き換えるファイルにコミットしていない変更がないことを確かめてから書く
    category: authorization
    applies_to: IacRequest.write
    trigger: 書き出す前
    logic: |
      git のコマンドで、--dir が作業ツリーの中か、書き換えるファイルがコミット済みで変更がないか(まだ追加していないファイルも変更とみなす)を確かめる。
      IF git のコマンドがない、--dir が作業ツリーでない、または書き換えるファイルに変更がある THEN 何も書き換えずに止め、理由と、差分だけを見るには --write を外すことを示す(終了コード 1)。
      書き換えるファイルがなければ、git の確かめはしない
    violation: 何も書き換えずに終了コード 1
    source: [Q6]
  - id: BR5.3
    statement: すべてのファイルが読んだときから変わっていないことを先に確かめてから、壊れない方法で書く
    category: constraint
    applies_to: FileResult.written
    trigger: --write で書き出すとき(BR5.2 のあと)
    logic: |
      書き出す前に、書き換えるすべてのファイルの中身が読んだときと同じかを確かめる。1つでも違えば、どのファイルも書かずに止める(終了コード 1)。
      そのあと1つずつ、同じフォルダーの一時ファイルに書いてから名前を変える(U6 の記録の保存と同じ)。ファイルの権限は元のまま保つ。--dir の外には書かない。
      書き出しの途中で1つのファイルの書き込みに失敗したら、そこで止め、書き換えたファイルと書き換えていないファイルを示す(git で戻せる。終了コード 1)
    violation: 終了コード 1
    source: [Q6]

  # 6. 結果
  - id: BR6.1
    statement: 結果は、ファイルごとの反映、反映済み、手で直す一覧、IaC で管理されていないリソースの数を示す
    category: policy
    applies_to: IacResult
    trigger: 終わるとき
    logic: |
      標準出力に、ファイルごとの差分(BR5.1)と、反映した変更の数を出す。続けて、すでに反映済みだった変更の数、ソースにないため反映済みとみなした削除の一覧(リソースの ARN、キー、ファイル。スタックのタグや default_tags を確かめる案内つき。BR4.3)、手で直す一覧(リソースの ARN、キー、ファイルと行、理由と、どうすればよいかの案内)、IaC で管理されていないリソースの数を出す。
      IF --tf-state がなく、管理されていないリソースが1件以上ある THEN 「Terraform で管理しているリソースがあれば、terraform show -json の出力を --tf-state で渡してください」と標準エラーに出す。IF 反映した変更と反映済みの変更が0件で、管理されていないリソースが1件以上ある THEN 「IaC に反映したものはありません」と標準エラーに警告を出す(どちらも終了コードは変えない。BR6.2)。
      --write のときは、書き換えたファイルの一覧と、IaC を適用する前に差分をレビューすることの案内を出す。iac は terraform も CloudFormation のスタックの更新も実行しない。
      CloudFormation のテンプレートを書き換えたときは、スタックの更新で同じタグになること(apply 済みならタグは変わらない)を案内する
    violation: (なし)
    source: [FR8.1, FR8.2, Q9]
  - id: BR6.2
    statement: 終了コードは、手で直すものがなければ 0、あれば 2、誤りなら 1
    category: calculation
    applies_to: IacResult.exitCode
    trigger: 終わるとき
    logic: |
      IF 入力の誤り(BR1.1〜BR1.3)THEN 結果を作らずに 1。
      IF --write の確かめか書き出しに失敗した(BR5.2、BR5.3)THEN 差分と一覧を示したうえで 1。
      ELSE IF ManualItem が1件以上 THEN 2。
      ELSE 0(反映済みの変更と、IaC で管理されていないリソースは、終了コードに影響しない。反映するものが0件の場合も 0)
    violation: (なし)
    source: [契約 C5, Q9]
  - id: BR6.3
    statement: iac のヘルプは、オプション、探し方、書き換えない場合、終了コードを示す
    category: policy
    applies_to: Cli
    trigger: iac --help
    logic: --plan、--dir(CloudFormation は下のフォルダーも探し、Terraform はルートモジュールのフォルダーを渡すこと)、--tf-state(terraform show -json の出力を渡すこと)、--write(git の確かめ)、手で直すことになる主な場合、終了コード 0・1・2 を示す。U1〜U7 のヘルプの形に合わせる。CDK とプルリクエストの作成はまだ扱わないことも示す
    violation: (なし)
    source: [契約 C5, Q1]
```

## 規則のまとめ

| ID | 内容 | 種類 | 出典 |
|---|---|---|---|
| BR1.1 | `--plan` と `--dir` は必須。`--profile` は受け付けない | validation | FR8.1、FR8.2、Q2、Q6、Q8 |
| BR1.2 | 変更案は版を確かめて読む。アカウントは比べない | validation | C2、Q8 |
| BR1.3 | `--tf-state` は `terraform show -json` の出力として読む(空の state も受け付ける) | validation | FR8.1、AS7、Q2 |
| BR2.1 | 変更のあるリソースだけが対象 | constraint | C2 |
| BR2.2 | `aws:cloudformation` のタグで CloudFormation のリソースと決める | calculation | FR8.2、Q2 |
| BR2.3 | state の ARN の完全一致で Terraform のリソースと決め、どちらでもなければ管理されていないと数える | calculation | FR8.1、AS7、Q2 |
| BR2.4 | モジュールの中と、count・for_each のリソースは手で直す | policy | FR8.1、Q4 |
| BR3.1 | CloudFormation は `--dir` の下、Terraform は直下だけ。`.git`・`.terraform`・`node_modules`・`cdk.out` とシンボリックリンクは見ない | constraint | Q2 |
| BR3.2 | CloudFormation は論理 ID と種類でテンプレートを1つに決める | calculation | FR8.2、Q2、Q3 |
| BR3.3 | Terraform はアドレスでブロックを1つに決める | calculation | FR8.1、Q2、Q3 |
| BR4.1 | タグの書き場所は3つの形だけ。式やアンカーなら手で直す | validation | Q4 |
| BR4.2 | キーと値が1行の文字列で直接書いてあるタグだけを書き換える | validation | Q4 |
| BR4.3 | 前の状態なら反映、後の状態なら反映済み、どちらでもなければ手で直す | validation | C2、Q4、Q7 |
| BR4.4 | 元のテキストの範囲の差し替えだけで書く | calculation | Q5、Q7 |
| BR4.5 | 必須タグは、最後の要素の書き方を写して足す | policy | Q7 |
| BR4.6 | 書き換えたあとを読み直して確かめる | validation | Q5 |
| BR5.1 | `--write` がなければ差分だけ | policy | Q6 |
| BR5.2 | `--write` は git の作業ツリーで、未コミットの変更がないときだけ | authorization | Q6 |
| BR5.3 | すべてのファイルを先に確かめてから、壊れない方法で書く | constraint | Q6 |
| BR6.1 | 結果の中身と、`--tf-state` の案内 | policy | Q9 |
| BR6.2 | 終了コード | calculation | C5、Q9 |
| BR6.3 | `iac` のヘルプ | policy | C5、Q1 |

### 手で直す理由(ManualItem.reason)

| 理由 | 意味 | 規則 |
|---|---|---|
| `templateNotFound` | 論理 ID と種類が合うテンプレートが `--dir` にない(CDK や SAM で作ったリソース、別のリポジトリのテンプレート、解析できないテンプレートなど) | BR2.2、BR3.2 |
| `blockNotFound` | アドレスのブロックが `--dir` の直下の `.tf` にない | BR3.3 |
| `ambiguous` | 場所が2つ以上あり、1つに決められない | BR2.3、BR3.2、BR3.3 |
| `inModule` | Terraform のモジュールの中のリソース | BR2.4 |
| `multipleInstances` | count や for_each で作るリソース | BR2.4 |
| `tagsNotLiteral` | タグの書き場所そのものが式・変数・関数、または YAML のアンカー・別名・マージのキー | BR4.1、BR4.5 |
| `unsupportedTagShape` | 3つの形のどれでもない書き方、1行にまとめた書き方での削除と追加、Key と Value 以外の項目を持つ並びへの追加 | BR4.1、BR4.4、BR4.5 |
| `notLiteral` | そのタグのキーか値が式・参照・補間・ブロックスカラー・複数行 | BR4.2 |
| `keyNotInSource` | 変える・消すキーがソースにない(スタックのタグや `default_tags` かもしれない) | BR4.3 |
| `valueDiffers` | ソースの値が変更の前の値とも後の値とも違う(IaC と AWS がずれている) | BR4.3 |
| `keyAlreadyExists` | 足す・名前を変えた先のキーが、ソースに別の値ですでにある | BR4.3 |
| `noTagSite` | 必須タグを足す書き場所がない、または空 | BR4.5 |
| `unreadableFile` | 書き換えたあとの確かめに失敗した(このツールの不具合の疑い) | BR4.4、BR4.6 |

反映済み(`alreadyReflected`)は手で直す理由ではありません。数だけを示し、終了コードに影響しません(BR4.3、BR6.2)。
