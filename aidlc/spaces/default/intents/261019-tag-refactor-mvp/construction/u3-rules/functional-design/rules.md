# 業務の規則: U3 ルール

U3 で完成させるルールファイル(契約 C1)の読み方と、`rules suggest` の規則です。U1 で決めたルールファイルの安全な読み方(通常のファイルだけ、64 KiB まで、入れ子の深さ、アンカー・別名・統合キー・タグ・重複したキーを拒む、改行の正規化)は、そのまま使います。ID はこの作業単位の中での番号です。

出典: 要件 FR2.1〜FR2.5、FR3.3、FR3.4、ストーリー US2.1(AC2.1.1〜AC2.1.5)、契約 C1・C5・C7・C8・C12 と全体の決まり(`aws:` で始まるタグ)、`functional-design-questions.md` Q1〜Q6、U2 の収集(表記ゆれのまとめ方、略語の辞書、種類の対応表)、AWS のタグの制限(キーは128文字まで、値は256文字まで)。

```yaml
rules:
  # 1. ルールファイルの読み込みと誤りの知らせ方
  - id: BR1.1
    statement: ルールファイルの誤りは、ファイル名、行番号、キー、何が誤りか、直し方の5つを示して知らせ、最初に見つけた誤りで止める
    category: validation
    applies_to: RuleSet の読み込み
    trigger: --rules を受け付けるコマンドの開始時(AWS を呼ぶ前)
    logic: 誤りを見つけたら、それ以上読まずに InvalidInput(版の誤りは UnsupportedVersion)を返す。行番号はファイルの1行目を1とし、改行の種類(LF、CRLF)によらない
    violation: 終了コード 1。標準出力には何も出さない
    source: [FR2.5, AC2.1.5, 契約 C1, C12]
  - id: BR1.2
    statement: 契約 C1 にないキーは誤りにする。綴りの近いキー(編集距離2以下)があれば、それを候補として示す
    category: validation
    applies_to: ルールファイルのすべての階層
    trigger: 読み込み
    logic: IF その階層で許されたキーにない THEN 誤り。直し方は「did you mean "<候補>"?」と、その階層で許されたキーの一覧
    violation: InvalidInput
    source: [FR2.5, AC2.1.5, 契約 C1]
  - id: BR1.3
    statement: 型の誤り(文字列のはずが数や真偽値、一覧のはずが単一の値、対応表のはずが一覧など)は、正しい書き方の例を直し方として示す
    category: validation
    applies_to: ルールファイルのすべての値
    trigger: 読み込み
    logic: タグのキーと値は文字列だけを受け付ける。数のように見える値(例 123、true)は、引用符で囲むよう案内する
    violation: InvalidInput
    source: [FR2.5, AC2.1.5, 契約 C1]
  - id: BR1.4
    statement: version は必須で、1 だけを受け付ける
    category: validation
    applies_to: RuleSet.version
    trigger: 読み込みの最初
    logic: version がない、または整数でない → InvalidInput。1 以外の整数 → UnsupportedVersion(このツールが読める版を案内する)。version の確認は、ほかのキーより先に行う
    violation: 終了コード 1
    source: [FR2.5, 契約 C1, 契約の決まり(バージョン)]
  - id: BR1.5
    statement: タグのキーは1〜128文字、タグの値は0〜256文字とし、超えたものや空のキーは誤りにする
    category: validation
    applies_to: keys、aliases、values、required、when.value、remove
    trigger: 読み込み
    logic: 文字数は Unicode の文字で数える。match.tags の値は比べるだけなので、長さだけを確かめる
    violation: InvalidInput(AWS のタグの制限を直し方に示す)
    source: [FR2.5, AWS のタグの制限]

  # 2. 正しいキー名、別名、正しい値
  - id: BR2.1
    statement: 別名は1つの正しいキー名だけに属し、それ自体が正しいキー名であってはならない。キー名は大文字・小文字を区別して比べる
    category: constraint
    applies_to: KeyRule.aliases
    trigger: 読み込みの終わり(すべてのキーを読んだあと)
    logic: 同じ別名が2つの正しいキー名の下にある → 誤り(両方の行を示す)。別名が別の正しいキー名と同じ → 誤り。同じ正しいキー名の下に同じ別名が2回 → 警告して1つにする。正規化すると同じになる正しいキー名が2つ → 警告
    violation: InvalidInput(警告は続ける)
    source: [FR2.2, AC2.1.2, 契約 C1]
  - id: BR2.2
    statement: 正しい値ごとに、別の書き方(値の別名)を書ける。値の別名は、同じキーの中で1つの正しい値だけに属し、それ自体が同じキーの正しい値であってはならない。値は大文字・小文字を区別して比べる
    category: constraint
    applies_to: KeyRule.values
    trigger: 読み込み
    logic: 同じキーの中で、同じ値の別名が2つの正しい値の下にある、または別の正しい値と同じ → 誤り。正しい値と同じ値の別名 → 警告して無視する
    violation: InvalidInput(警告は続ける)
    source: [FR2.2, 契約 C1]
  - id: BR2.3
    statement: あるキーの値について、正しい値を答える。値の別名なら正しい値を返し、正しい値そのもの、またはどこにも書かれていない値なら「変えない」を返す
    category: calculation
    applies_to: 値の照会(U4 の変更案が使う)
    trigger: U4 の plan
    logic: キーは正しいキー名で照会する(別名のキーは先に正しいキー名に置き換えてから照会する)。値は完全一致で比べる
    violation: なし
    source: [FR2.2, 契約 C8]

  # 3. 必須タグ
  - id: BR3.1
    statement: 必須タグのキーは、別のキーの別名であってはならない。消すキーと同じであってもならない
    category: constraint
    applies_to: RequiredTag.key
    trigger: 読み込みの終わり
    logic: required のキーが keys のどれかの aliases にある → 誤り(直し方として正しいキー名を示す)。remove にもある → 誤り。required のキーが keys にない → 問題なし(新しいキーを付ける規則として扱う)
    violation: InvalidInput
    source: [FR2.3, FR2.4, 契約 C1]
  - id: BR3.2
    statement: リソースの必須タグの値は、when を上から順に評価して最初に当てはまった条件の値とする。当てはまる条件がなければ default、default もなければ「値が未指定」とする
    category: calculation
    applies_to: 必須タグの値の照会(U4 の変更案が使う)
    trigger: U4 の plan
    logic: |
      1つの条件は、match に書いた項目がすべて当てはまるときに当てはまる(かつ)。
      resourceType はリソースの種類(scan と同じ形)と完全一致。
      region はリソースのリージョンと完全一致。グローバルなリソース(ARN にリージョンがないもの)は、region を書いた条件には当てはまらない(グローバルなリソースのリージョンは、読んだリージョンによって変わるため)。
      tags は、書いたすべてのキーと値の組が、比べる相手のタグにあること(キーも値も大文字・小文字を区別して完全一致)。比べる相手は「キーの別名を正しいキー名に置き換え、値の別名を正しい値に置き換え、消すキーを除いたあとのタグ」とし、必須タグを付ける前の状態とする。別名と正しいキー名の両方が付いていてぶつかるリソースでは、正しいキー名の今の値を使う。
      答えは「値と、それを決めた行(when の行、または default の行)」か「値が未指定」のどちらか
    violation: 値が未指定のときの扱いは U4(FR3.3)
    source: [FR2.3, FR3.3, AC2.1.3, Q1, 契約 C1 の未決事項]
  - id: BR3.3
    statement: match には1つ以上の項目を書く。項目のない match は誤りにする
    category: validation
    applies_to: RequiredTag.when[].match
    trigger: 読み込み
    logic: 何も書かない match はすべてのリソースに当てはまり、default と区別がつかないため、default を使うよう案内する
    violation: InvalidInput
    source: [FR2.3, FR2.5]
  - id: BR3.4
    statement: match.resourceType は、scan が示すリソースの種類と同じ ARN の形で書く。形は「サービス名」だけ(例 s3、sqs)か「サービス名:種類」(例 ec2:instance)のどちらか。CloudFormation の形(AWS::EC2::Instance)と unknown は誤りにする
    category: validation
    applies_to: RequiredTag.when[].match.resourceType
    trigger: 読み込み
    logic: |
      サービス名は小文字・数字・ハイフン、種類は空白と「:」を含まない1文字以上の文字列。
      IF 「AWS::」で始まる THEN 誤り。U2 の種類の対応表に載っていれば、その ARN の形を直し方に示す(例 write "ec2:instance")。載っていなければ「この種類は条件に書けない」と示す(scan が AWS::... の形のまま示す種類は、ARN を持たないか対応表にない種類で、条件では扱えない。この制限は README に書く)。
      IF unknown THEN 誤り(ARN を解釈できなかったリソースの印で、種類ではない)
    violation: InvalidInput
    source: [FR2.3, Q6, U2 の種類の対応表と収集の種類の決め方]
  - id: BR3.6
    statement: match.tags のキーは、別のキーの別名であってはならず、値は、そのキーの値の別名であってはならない
    category: validation
    applies_to: RequiredTag.when[].match.tags
    trigger: 読み込みの終わり
    logic: BR3.2 のとおり、match.tags は別名と値の別名を置き換えたあとのタグと比べるため、別名で書いた条件はどのリソースにも当てはまらない。そこで、キーが別名なら正しいキー名を、値が値の別名なら正しい値を、直し方に示して誤りにする。match.tags のキーが消すキーと同じ場合も、どのリソースにも当てはまらないため誤りにする
    violation: InvalidInput
    source: [FR2.3, FR2.5]
  - id: BR3.5
    statement: match.region はリージョン名の形(例 ap-northeast-1)で書く
    category: validation
    applies_to: RequiredTag.when[].match.region
    trigger: 読み込み
    logic: U1 の --region と同じ形の確かめ方を使う。アカウントで有効かどうかは確かめない(ルールファイルは複数のアカウントで使えるため)
    violation: InvalidInput
    source: [FR2.3, FR2.5]

  # 4. 消すキー
  - id: BR4.1
    statement: 消すキーは、正しいキー名や別名であってはならない。同じキーを2回書いたら警告して1つにする
    category: constraint
    applies_to: RuleSet.removeKeys
    trigger: 読み込みの終わり
    logic: remove のキーが keys の正しいキー名、またはどれかの aliases にある → 誤り(別名なら、名前の変更で消えることを直し方に示す)
    violation: InvalidInput(警告は続ける)
    source: [FR2.4, 契約 C1]
  - id: BR4.2
    statement: あるキーを消すかどうかを答える。remove に書いたキーと完全一致(大文字・小文字を区別)したときだけ「消す」とする
    category: calculation
    applies_to: 消すキーの照会(U4 の変更案が使う)
    trigger: U4 の plan
    logic: 正規化や略語の辞書は使わない。消す候補(ルールファイルにないキー、FR3.4)を出すのは U4
    violation: なし
    source: [FR2.4, FR3.4, AC2.1.4]

  # 5. aws: で始まるキー
  - id: BR5.1
    statement: keys、aliases、values のキー、required、remove のどこかに aws: で始まるキー(大文字・小文字を問わない)があれば、その規則を無視して警告する。match.tags の aws: のキーは条件として使える
    category: policy
    applies_to: RuleSet.warnings
    trigger: 読み込み
    logic: 無視した規則は RuleSet に入れない。match.tags の aws: のキー(例 aws:cloudformation:stack-name)は読むだけなので、条件として受け付ける
    violation: 警告(行、キー、理由)。終了コードは変えない
    source: [契約の全体の決まり(aws:)、契約 C8]

  # 6. rules suggest
  - id: BR6.1
    statement: 下書きのキーは、U2 と同じまとめ方(正規化と略語の辞書)でまとめ、まとまりの中で最も多くのリソースで使われている書き方を正しいキー名にし、残りを別名にする
    category: calculation
    applies_to: SuggestedKey
    trigger: rules suggest
    logic: 件数は、そのキーが付いたリソースの数(ARN で重複を除く)。件数が同じなら、略語の辞書の正式な形を先に、次に名前の順(バイトの順)。別名は件数の多い順、同じなら名前の順
    violation: なし
    source: [FR2.1, AC2.1.1, Q3]
  - id: BR6.2
    statement: 見つかったすべてのキー(aws: で始まるものを除く)を下書きに入れる。書き方が1つだけのキーは別名なしで入れる
    category: policy
    applies_to: SuggestedRuleFile.keys
    trigger: rules suggest
    logic: 下書きをそのまま plan に使ったとき、すでに使われているキーが「消す候補」にならないようにするため
    violation: なし
    source: [FR2.1, FR3.4, Q5]
  - id: BR6.3
    statement: キーのまとまりごとに、値もキーと同じまとめ方でまとめ、書き方が2つ以上ある値のまとまりだけを values に書く。まとまりの中の異なる値が20個を超えるキーは、値の候補を出さない
    category: calculation
    applies_to: SuggestedKey.values
    trigger: rules suggest
    logic: 対象は、まとまりのすべての書き方のキーに付いた値。正しい値は最も多くのリソースで使われている書き方(同じなら略語の辞書の正式な形、次に名前の順)。値を出さなかったキーには、その旨をコメントで書く
    violation: なし
    source: [FR2.1, AC2.1.1, Q4]
  - id: BR6.4
    statement: 下書きは、そのまま読み込める C1 の形で標準出力に出す。キーと値は、どんな文字を含んでも同じ文字列として読み戻せる引用の形で書く
    category: constraint
    applies_to: SuggestedRuleFile
    trigger: rules suggest
    logic: キーと値はすべて二重引用符で囲み、印字できない文字(制御文字、書式の文字、行や段落の区切り)は YAML の \uXXXX の形で書く。こうすると端末を書き換える文字がそのまま出ず、読み込むと元の文字列に戻る。並びは、キーは件数の多い順、同じなら名前の順
    violation: なし
    source: [FR2.1, AC2.1.1, NFR8]
  - id: BR6.5
    statement: 下書きの先頭に、作ったツールの版、アカウント、調べたリージョンの数、作った日時をコメントで書く。読めなかったリージョンがあれば、そのこともコメントで書く。required と remove は、書き方の例をコメントで書く。タグの付いたリソースが1つもなければ、keys を空(keys: {})にして出す
    category: policy
    applies_to: SuggestedRuleFile の先頭
    trigger: rules suggest
    logic: 先頭の行は「# generated by tag-refactor <版>」。読めなかったリージョンがあれば「# Results are incomplete: <リージョン>」の行を足す。required と remove は、利用者が決めるもので候補を作れないため、例だけを示す。keys: {} の下書きも、そのまま読み込める
    violation: なし
    source: [FR2.1, 契約 C1]
  - id: BR6.6
    statement: タグの取得で読めなかったリージョンがあれば、下書きは出すが、先頭のコメントと標準エラーに「結果は不完全」とリージョンを示し、終了コードを 1 にする
    category: policy
    applies_to: rules suggest の終了コード
    trigger: rules suggest の終わり
    logic: エラーまたは不完全 → 1。それ以外 → 0(表記ゆれがあっても 0。scan と違い、問題を見つけるコマンドではないため)。認証情報の誤りなどで止まったときは、下書きを出さない
    violation: 終了コード 1
    source: [契約 C5 の終了コード, FR2.1]
  - id: BR6.7
    statement: 下書きは、ルールファイルの読み込みの上限(64 KiB)に収める。収まらないときは、値の候補を件数の少ないキーから順に外し、それでも収まらなければ下書きを出したうえで警告し、終了コードを 1 にする
    category: constraint
    applies_to: SuggestedRuleFile の大きさ
    trigger: rules suggest の出力の前
    logic: |
      下書きを組み立てて大きさを測る。64 KiB を超えたら、値の候補をリソースの件数の少ないキーから外し(そのキーに「values omitted: the draft would exceed 64 KiB」のコメントを付ける)、収まるまで繰り返す。
      値の候補をすべて外しても超える(キーだけで上限を超える。1キーあたり平均40バイトとして約1,600キー)ときは、下書きをそのまま出し、標準エラーに「the draft is larger than 64 KiB and cannot be read as it is; remove keys you do not need」と出して、終了コード 1 にする
    violation: 終了コード 1(キーだけで上限を超えるとき)
    source: [FR2.1, AC2.1.1, 読み込みの上限(U1 のルールファイルの安全な読み方)]
```

## 規則のまとめ

| ID | 規則 | 種類 | 出典 |
|---|---|---|---|
| BR1.1 | 誤りは5つの情報で知らせ、最初の誤りで止める | validation | FR2.5、AC2.1.5 |
| BR1.2 | 知らないキーは誤り。近い綴りを示す | validation | FR2.5 |
| BR1.3 | 型の誤りは正しい書き方の例を示す | validation | FR2.5 |
| BR1.4 | version は必須で 1 だけ | validation | FR2.5、C1 |
| BR1.5 | キーは1〜128文字、値は0〜256文字 | validation | AWS のタグの制限 |
| BR2.1 | 別名は1つの正しいキー名だけに属する | constraint | FR2.2、AC2.1.2 |
| BR2.2 | 値の別名は同じキーの1つの正しい値だけに属する | constraint | FR2.2 |
| BR2.3 | 値の照会: 別名なら正しい値、それ以外は変えない | calculation | FR2.2 |
| BR3.1 | 必須タグのキーは別名でも消すキーでもない | constraint | FR2.3、FR2.4 |
| BR3.2 | 必須タグの値: when を上から、次に default、なければ未指定。tags は別名を置き換えたあとのタグと比べる。グローバルなリソースは region の条件に当てはまらない | calculation | FR2.3、AC2.1.3、Q1 |
| BR3.3 | 項目のない match は誤り | validation | FR2.3 |
| BR3.4 | resourceType は scan と同じ形(s3、ec2:instance)。CloudFormation の形と unknown は誤り | validation | FR2.3、Q6 |
| BR3.5 | region はリージョン名の形 | validation | FR2.3 |
| BR3.6 | match.tags に別名、値の別名、消すキーは書けない | validation | FR2.3 |
| BR4.1 | 消すキーは正しいキー名でも別名でもない | constraint | FR2.4 |
| BR4.2 | 消すかどうかは完全一致で答える | calculation | FR2.4、AC2.1.4 |
| BR5.1 | aws: のキーの規則は無視して警告。match.tags では使える | policy | 契約の全体の決まり |
| BR6.1 | 下書きの正しいキー名は最も多い書き方 | calculation | FR2.1、Q3 |
| BR6.2 | 見つかったすべてのキーを入れる | policy | FR2.1、Q5 |
| BR6.3 | 値の候補は書き方が2つ以上のものだけ。20個を超えるキーは出さない | calculation | FR2.1、Q4 |
| BR6.4 | そのまま読み込める形で、どんな文字も読み戻せる引用で書く | constraint | FR2.1、AC2.1.1 |
| BR6.5 | 先頭に版などのコメント(不完全ならその旨も)。required と remove は例だけ。0件なら keys: {} | policy | FR2.1、C1 |
| BR6.6 | 読めなかったリージョンがあれば下書きは出すが終了コード 1 | policy | C5 |
| BR6.7 | 下書きは 64 KiB に収める。値の候補から外し、キーだけで超えたら警告して終了コード 1 | constraint | FR2.1、AC2.1.1 |
