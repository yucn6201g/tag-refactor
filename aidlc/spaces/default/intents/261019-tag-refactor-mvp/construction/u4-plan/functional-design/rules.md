# 業務の規則: U4 変更案

U4 で完成させる変更案(`plan`)の作り方、集計、4つの出力形式、言語と色、保存と読み込みの規則です。U1 で作った変更案の最小版(キー名の変更、衝突、`changeId`、影響分析の呼び出し)を広げます。ID はこの作業単位の中での番号です。

出典: 要件 FR3.1〜FR3.8、FR7.2〜FR7.5、NFR11、ストーリー US3.1(AC3.1.1〜AC3.1.8)と US6.1(AC6.1.1〜AC6.1.5)、U3 から引き継いだ受け入れ条件 AC2.1.3・AC2.1.4、契約 C2・C3・C5・C8・C9・C12 と全体の決まり(`aws:` で始まるタグ)、詳細なモックアップの画面4〜6と表示の決まり、`functional-design-questions.md` Q1〜Q8、U1 の規則(BR4.1〜BR4.4)、U3 の照会の部品(BR2.3、BR3.2、BR4.2)。

```yaml
rules:
  # 1. 変更の作り方(リソースごと)
  - id: BR1.1
    statement: 正しいキー名ごとに、付いているキー(正しいキー名と別名)の値を、値の別名を当てはめたあとで比べる。すべて同じなら、別名ごとに正しいキー名への renameKey を作る
    category: calculation
    applies_to: Change(renameKey)
    trigger: plan の変更案の作成
    logic: 付いているキー = 正しいキー名と別名のうち付いているもの(別名はルールファイルの順)。IF 正しいキー名だけ THEN 何もしない。それぞれの値を U3 の正しい値の照会(BR2.3)で正しい形にし、IF すべて同じ THEN 別名ごとに renameKey(fromKey=別名, toKey=正しいキー名, fromValue=その別名の収集したままの値)。mergesIntoExisting は U1 BR4.2 のまま(正しいキー名が付いていれば全件 true、なければ最初の別名だけ false)。mergesIntoExisting が true の renameKey は、toKey に何も書かず fromKey を消すだけとする(toKey の今の値は、値の別名を当てると同じなので残す。書き方の違いは BR1.3 の changeValue で直す)。false の renameKey は、toKey に fromKey の値を付けてから fromKey を消す
    violation: なし
    source: [FR2.2, FR3.1, AC3.1.1, Q3, U1 BR4.1, U1 BR4.2]
  - id: BR1.2
    statement: 値の別名を当てはめても、付いているキーの値が1つでも違えば、そのキーは変えずに「衝突」として示す
    category: calculation
    applies_to: KeyConflict
    trigger: BR1.1 の比較
    logic: IF 正しい形にした値が2種類以上 THEN KeyConflict を1件作り、そのキーの renameKey と changeValue は作らない。示す値は収集したままの値
    violation: 要確認(終了コード 2。BR2.6)
    source: [Q3, U1 BR4.2]
  - id: BR1.3
    statement: 正しいキー名の値(名前の変更のあとの値)が値の別名なら、正しい値への changeValue を作る。キー名の変更と重なるときは、renameKey のあとに置く
    category: calculation
    applies_to: Change(changeValue)
    trigger: BR1.1 のあと(衝突のないキーだけ)
    logic: 名前の変更のあとに正しいキー名が持つ値 v を BR1.9 で求める。U3 の正しい値の照会で Changed なら changeValue(fromKey=正しいキー名, fromValue=v, toValue=正しい値)。fromKey は変更後のキー名。正しいキー名がなく別名もない値(ルールの values にない値)は変えない
    violation: なし
    source: [FR3.1, AC3.1.1, Q1, U3 BR2.3]
  - id: BR1.4
    statement: 必須タグは、正しいキー名で付いていないときだけ addRequired を作る。値が決まらなければ「値が未指定」にし、そのリソースの他の変更は残す。既に付いている値は変えない
    category: calculation
    applies_to: Change(addRequired)、UnsetRequiredTag
    trigger: BR1.1〜BR1.3 のあと
    logic: 必須タグのキーごとに(キーの文字列の順)。IF 名前の変更のあとのタグ(BR1.9)にそのキーがある、またはそのキーが衝突している(BR1.2。別名の形では付いている) THEN 何もしない(Q2)。ELSE U3 の必須タグの値の照会(BR3.2。収集したままのタグを渡す)を呼び、resolved なら addRequired(toKey=キー, toValue=値に U3 の正しい値の照会を当てた値)、unspecified なら UnsetRequiredTag にそのリソースを加える。値の別名を当てるのは、ルールの必須タグの値が値の別名の書き方で書かれていても、次の plan で changeValue が出ないようにするため(plan、apply、plan で「変更は不要」になる)
    violation: 値が未指定は要確認(終了コード 2。BR2.6)
    source: [FR2.3, FR3.1, FR3.3, AC3.1.1, AC3.1.3, AC2.1.3, Q2, U3 BR3.2]
  - id: BR1.5
    statement: 消すキーとして書かれたキーが付いていれば、removeKey を作る。消すキーかどうかは完全一致で決める
    category: calculation
    applies_to: Change(removeKey)
    trigger: BR1.4 のあと
    logic: 収集したままのタグのキーごとに(キーの文字列の順)、U3 の消すかどうかの照会(BR4.2)が「消す」なら removeKey(fromKey=キー, fromValue=値)。消すキーは正しいキー名・別名・必須タグと重ならない(U3 で誤りにしている)ため、BR1.1〜BR1.4 の変更とぶつからない
    violation: なし
    source: [FR2.4, FR3.1, AC3.1.1, AC2.1.4, U3 BR4.2]
  - id: BR1.6
    statement: aws: で始まるキーには変更を作らず、消す候補にも入れない
    category: policy
    applies_to: Change、RemoveCandidate
    trigger: BR1.1〜BR1.5、BR2.3
    logic: IF キー(fromKey、toKey)が aws: で始まる THEN 変更を作らない。aws: かどうかは大文字・小文字を区別せずに判定し(AWS:、Aws: も含む)、U2・U3 と同じ判定の部品を使う。U3 が aws: の規則を無視して警告しているため、ルールからは作られないが、変更を作る直前でも確かめる(念のための守り)
    violation: なし
    source: [契約の全体の決まり, FR5.9]
  - id: BR1.7
    statement: タグを変更できない種類のリソースには変更を作らず、値が未指定にも数えない
    category: policy
    applies_to: Plan
    trigger: リソースごとの処理の最初
    logic: IF U2 の種類の照会でタグを変更できない種類 THEN そのリソースを飛ばす。件数は unsupportedResourceTypes(U2 の集計)で示す
    violation: なし
    source: [AS3, 契約 C2 unsupportedResourceTypes]
  - id: BR1.8
    statement: 変更は決まった順に並べ、changeId を振る
    category: calculation
    applies_to: Change.changeId
    trigger: 変更案の作成
    logic: リソースは ARN の文字列の順。1つのリソースの中は、(1) 正しいキー名の文字列の順に、そのキーの renameKey(別名の順)、続いて changeValue、(2) 必須タグのキーの順に addRequired、(3) キーの順に removeKey。changeId は c-000001 から作った順に振る。apply はこの順に行う
    violation: なし
    source: [C2, U1 BR4.3, Q1]
  - id: BR1.9
    statement: 変更後のタグは、変更前のタグに変更を順に当てはめて求める。差分の形と、U6 の書き換え後に期待するタグは、この規則で同じものになる
    category: calculation
    applies_to: Plan、U6 の plannedAfterTags
    trigger: 差分の表示、BR1.3、BR1.4、U6
    logic: renameKey は、mergesIntoExisting が false なら toKey = fromKey の値として fromKey を消し、true なら toKey に何も書かず fromKey を消す(BR1.1。U6 の書き込みも同じ)。changeValue は fromKey の値を toValue にする。addRequired は toKey = toValue。removeKey は fromKey を消す。aws: のキーはそのまま残る
    violation: なし
    source: [C2, C4 plannedAfterTags, Q1]

  # 2. 集計
  - id: BR2.1
    statement: 変更されるリソースの総数は、変更が1つ以上あるリソースの数とし、変更が重なっても1件と数える。種類ごとの件数は変更の数で数える
    category: calculation
    applies_to: PlanSummary
    trigger: 変更案の作成のあと
    logic: resourcesToChange = changes の resourceArn の重複のない数。changesByKind = 種類ごとの変更の数。一覧の行は種類ごとに次の単位でまとめ、リソースの数を示す。renameKey は (fromKey, toKey)、changeValue は (fromKey, fromValue, toValue)、addRequired は (toKey)、removeKey は (fromKey)。renameKey と removeKey の fromValue(参考)ではまとめを分けない。addRequired は値が1種類ならその値、複数なら「(from rules)」と示す。行の順は種類(renameKey、changeValue、addRequired、removeKey)、その中はリソースの数の多い順、同じならキーの順
    violation: なし
    source: [FR3.2, AC3.1.1, AC3.1.2, U1 BR4.3]
  - id: BR2.2
    statement: 値が未指定の必須タグは、キーごとにリソースの数とリソースの一覧を示す。変更案には入れない
    category: calculation
    applies_to: UnsetRequiredTag
    trigger: BR1.4
    logic: キーの文字列の順。リソースは ARN の順。一覧の形では件数と理由(default も当てはまる条件もない)を示し、差分・JSON・Markdown ではリソースも示す
    violation: なし
    source: [FR3.3, AC3.1.3]
  - id: BR2.3
    statement: 消す候補は、どの規則にもないキー(正しいキー名、別名、必須タグ、消すキーのどれでもない)とし、変更案には入れない
    category: calculation
    applies_to: RemoveCandidate
    trigger: 変更案の作成
    logic: 変更できる種類のリソースのタグのキーごとに、U3 の照会でどれにも当たらなければ候補。aws: で始まるキーは、大文字・小文字を区別せずに除く(BR1.6)。キーの文字列の順に並べ、それぞれのリソースの数を示す(JSON の removeCandidates はキー名だけ)
    violation: なし(案内。終了コードは変えない。BR2.6)
    source: [FR3.4, AC3.1.4, 契約の全体の決まり]
  - id: BR2.4
    statement: 変更のあるリソースのうち、IaC で管理されている印のあるものを「IaC で管理されている」と警告し、その数を示す
    category: policy
    applies_to: PlanSummary.iacManagedResources、Resource.iacManagedBy
    trigger: 変更案の作成のあと
    logic: IF 変更が1つ以上あり、U2 の iacManagedBy がある THEN 警告の対象。一覧では件数、差分と Markdown ではリソースごとに [IaC: CloudFormation stack "<名前>"] を付ける
    violation: なし(案内。apply の確認画面で再び示す)
    source: [FR3.7, AC3.1.7, AS7]
  - id: BR2.5
    statement: 影響のある変更には「!」、判定できない参照だけが当たる変更には「?」を付ける
    category: calculation
    applies_to: Change.impacted、表示の印
    trigger: 影響分析(契約 C9)のあと
    logic: affected の finding の changeIds に入る変更は「!」。affected がなく undecided の finding だけに入る変更は「?」。どちらも impacted = true(契約 C5: undecided も影響ありとして扱う)
    violation: なし
    source: [FR3.8, 契約 C2, C5]
  - id: BR2.6
    statement: plan の終了コードは、収集か影響分析が不完全なら 1、変更・衝突・値が未指定・影響のある参照のどれかがあれば 2、それ以外は 0 とする。消す候補だけでは 0
    category: policy
    applies_to: plan の終了コード
    trigger: plan の終わり
    logic: IF 収集が不完全 OR 影響分析が不完全 THEN 1。ELSE IF changes、conflicts、unsetRequiredTags、impactFindings のどれかが空でない THEN 2。ELSE 0。JSON の exitCode も同じ値
    violation: なし
    source: [FR7.4, 契約 C5, Q4]
  - id: BR2.7
    statement: 変更が1つもなければ、影響分析を行わない。そのうえで衝突も値が未指定もなければ、「変更は不要」と調べたリソースの件数を示す
    category: policy
    applies_to: Plan
    trigger: 変更案の作成のあと
    logic: IF changes が空 THEN ポリシーを読まない(影響分析の入力が0件のため。U1 BR4.4 の「変更も衝突もない」を「変更がない」に改める。今のコードの呼び出しの条件と同じ)。IF changes、conflicts、unsetRequiredTags がすべて空 THEN 「No changes needed」と件数。消す候補があれば、その下に案内として示す
    violation: なし
    source: [U1 BR4.4, FR1.8 の考え方]

  # 3. 出力の形式、言語、色
  - id: BR3.1
    statement: 一覧、差分、JSON、Markdown の4つの形式は、同じ変更案のデータから作り、変更の件数と影響の件数を一致させる
    category: constraint
    applies_to: Display
    trigger: plan の出力
    logic: 形式ごとに集計をし直さず、変更案と BR2.1 の集計の結果をそのまま使う。例外として、消す候補ごとのリソースの数は一覧と Markdown にだけ出し、JSON の removeCandidates はキー名だけとする(契約 C2・C3 のまま。消す候補のキーの数は4つの形式で一致する)。形式は --format(table、diff、json、markdown。既定は table)で選ぶ。知らない値は使い方の誤り
    violation: 使い方の誤りは InvalidInput(終了コード 1)
    source: [FR3.5, AC3.1.5, 契約 C3, C5]
  - id: BR3.2
    statement: 一覧の形は、対象、変更、値が未指定、消す候補、IaC、影響、要約と終了コード、次の手順の順に示す
    category: policy
    applies_to: Display(table)
    trigger: plan --format table
    logic: 操作の仕様(interaction-spec の plan「表示の順番」)とモックアップの画面4のとおり。不完全なら「Results are incomplete」または「Impact analysis is incomplete」を先頭に出す。衝突は変更の次に示す
    violation: なし
    source: [FR3.1〜FR3.4, FR3.7, FR3.8, 画面4]
  - id: BR3.3
    statement: 差分の形は、変更・衝突・値が未指定のあるすべてのリソースを ARN の順に出し、変更前と変更後のタグを行ごとに示す
    category: policy
    applies_to: Display(diff)
    trigger: plan --format diff
    logic: リソースごとに ARN の行(IaC の印つき)。変わるキーだけを出す: 消えるキーは「- key = "値"」、増えるキーと値が変わるキーは「+ key = "値"」(変更前のキーがあれば先に「-」)。キー名だけが変わり値が同じなら「~ from -> to = "値"」。値の比較は BR1.9 の変更前と変更後で行う。影響の印(BR2.5)を行の先頭に付ける。衝突は「! conflict」、値が未指定は「! required tag value not set: <キー>」
    violation: なし
    source: [FR3.5, 画面5, Q1]
  - id: BR3.4
    statement: JSON の形は契約 C3 とし、plan の details には契約 C2 と同じ形の resources、keyConflicts、impactFindings、unreadPolicies、unsetRequiredTags、removeCandidates を入れる。scan の --format json も契約 C3 のとおり出す
    category: constraint
    applies_to: Display(json)
    trigger: scan、plan の --format json
    logic: 正しい JSON を1つだけ標準出力に出す。exitCode は実際の終了コードと同じ値(scan は U1 の決まり、plan は BR2.6)。failedRegions と unsupportedResourceTypes は scan と plan のどちらにも入れる。summary には契約 C3 の項目に加えて conflictResources と unsetRequiredResources を入れる(項目を足すだけなので version は 1 のまま)。数は数値のまま(BR3.6 の区切りは付けない)。文字列は訳さない。項目の順は決まった順にする(同じ入力で同じバイト列)
    violation: なし
    source: [FR3.5, FR7.5, AC6.1.5, 契約 C2, C3]
  - id: BR3.5
    statement: Markdown の形は、一覧の形と同じ順の節を見出しと表で書き、リソースごとの変更は <details> で畳んだ表で全件書く。scan の --format markdown も同じ書き方で、表記ゆれと必須タグの欠落を表にする
    category: policy
    applies_to: Display(markdown)
    trigger: scan、plan の --format markdown
    logic: 見出しは ## で節ごと。表のセルの | と改行はエスケープし、キーと値はコードの書式(`…`)で囲む。リソースごとの表は ARN、変更、IaC、影響の列。件数は BR3.6
    violation: なし
    source: [FR3.5, 契約 C3 の未決事項, Q6]
  - id: BR3.6
    statement: 一覧、差分、Markdown の件数には3桁区切りのカンマを付ける。JSON は数値のまま
    category: policy
    applies_to: Display(table、diff、markdown)、scan の一覧
    trigger: 件数を出すとき
    logic: 1000 以上の整数を 1,000 の形にする。日本語でも同じ形。ARN、アカウントID、キーや値の中の数字には付けない
    violation: なし
    source: [画面4, Q8]
  - id: BR3.7
    statement: 色は補助として使い、意味は必ず記号と文字でも表す。NO_COLOR があるか、出力先が端末でなければ色を出さない
    category: policy
    applies_to: Display
    trigger: すべての出力
    logic: 標準出力と標準エラーを別々に判定する。IF NO_COLOR が空でない値で設定されている OR 出力先が端末でない THEN 色なし。記号(+ ~ - ! ? > x)と文字は色の有無によらず同じ。色は表示の決まり(design-system-mapping)の対応表のとおり。JSON には色を付けない
    violation: なし
    source: [FR7.3, NFR11, AC6.1.3]
  - id: BR3.8
    statement: 言語は --lang、なければ OS の言語設定で決め、日本語でも英語でもなければ英語にする
    category: calculation
    applies_to: Display の言語
    trigger: コマンドの開始時
    logic: --lang(en、ja)があればそれ。なければ LC_ALL、LC_MESSAGES、LANG の順に最初の空でない値を見て、ja で始まれば日本語、それ以外は英語。Windows で環境変数がなければ、OS の表示言語が日本語なら日本語。--lang の知らない値は使い方の誤り
    violation: 使い方の誤りは InvalidInput(終了コード 1)
    source: [FR7.2, AC6.1.2, 契約 C5]
  - id: BR3.9
    statement: 日本語にするのは --help 以外のすべての人が読む文言とする。数字、ARN、ポリシー名、キー名、値、オプション名、JSON は訳さない
    category: policy
    applies_to: Display の文言、rules suggest の下書きのコメント
    trigger: 言語が日本語のとき
    logic: 標準出力の結果(一覧、差分、Markdown)、標準エラーの進み具合・案内・警告・エラー、rules suggest の下書きのコメントを日本語にする。--help の説明は英語のまま。英語と日本語で同じ情報を出す。例外として、ほかのパッケージが作る誤りの詳細の文(ルールファイルの誤りの詳細、AWS へのアクセスの詳細、影響分析の読めなかった理由、一度もタグが付いていないリソースの検出の理由)と `--debug` の行は英語のまま出し、エラーの2行の枠と案内だけを日本語にする(保存した変更案の読み込みの誤りは、文言の識別子で返して日本語にする)
    violation: なし
    source: [FR7.2, Q7, 表示の決まり「文言の書き方」]
  - id: BR3.10
    statement: 結果(一覧、差分、JSON、Markdown)だけを標準出力に出し、進み具合、案内、警告、エラー、保存したことは標準エラーに出す
    category: constraint
    applies_to: Display
    trigger: すべての出力
    logic: U1 の出力先の決まりのまま。行ごとの出力先は functional-spec.md の「出力先の表」のとおり。標準出力には、結果の節(不完全の印を含む)と Summary の行だけを出す。アカウント・リージョン・作成日時の行、Exit code の行、次の手順の行、Saved plan の行、進み具合、警告、エラーは標準エラーに出す。Markdown では対象(アカウント、リージョン、作成日時)を結果の一部として ## の節に書く。形式や言語を変えても変わらない。--out を付けても標準出力の結果は同じ
    violation: なし
    source: [FR7.5, AC3.1.8, AC6.1.5, 契約 C5]

  # 4. 保存と読み込み
  - id: BR4.1
    statement: --out のファイルには契約 C2 の形で変更案を保存する。既にあるファイルは上書きせず、一時ファイルに書いてから名前を付ける
    category: constraint
    applies_to: SavedPlan
    trigger: plan --out
    logic: AWS を呼ぶ前に、保存先に既にファイルがないことと、保存先のフォルダに一時ファイルを作れることを確かめる(一時ファイルは最初に作って保存まで持つ)。IF 既にファイルがある THEN InvalidInput(上書きしない。消すか別の名前を案内)。IF 一時ファイルを作れない THEN StorageFailed。変更案を作り、影響分析を済ませたあと、結果を出す前にその一時ファイルに書いて名前を付ける。一時ファイルは、どの終わり方(保存の成功と失敗、ルールファイルや認証情報の誤り、収集の失敗、Ctrl+C などの割り込み)でも消す。書き出す変更案が読み込みの上限(NFR の要件で決める)を超えるときは保存せずに誤りにし、`--region` で分けて作るよう案内する。保存に失敗したら StorageFailed とし、標準出力には何も出さない。成功したら、いつもどおり結果を標準出力に出し、標準エラーに「Saved plan to <パス> (N resources, account <ID>)」を出す。収集や影響分析が不完全でも保存し、不完全の印を付ける
    violation: 終了コード 1
    source: [FR3.6, AC3.1.6, Q5, 契約 C2]
  - id: BR4.2
    statement: 保存する変更案には、版、ツールの版、変更案のID、アカウント、作成日時(UTC)、リージョン、不完全の印、ルールファイルの場所、変更のあるリソースの変更前のタグ全体と変更、影響、値が未指定、消す候補、タグを変更できない種類を含める
    category: constraint
    applies_to: SavedPlan
    trigger: BR4.1
    logic: 契約 C2 の必須の項目をすべて書く。resources は変更が1つ以上あるリソースだけ(beforeTags は aws: を含む全体)。衝突は契約 C2 に足す項目 keyConflicts(項目を足すだけなので version は 1 のまま)に書く
    violation: なし
    source: [FR3.6, AC3.1.6, 契約 C2, 契約の決まり「追加の変更」]
  - id: BR4.3
    statement: 変更案のIDは、作成日時(UTC の秒まで)と乱数の接尾辞から作る
    category: calculation
    applies_to: Plan.planId
    trigger: 変更案の作成
    logic: plan-YYYYMMDDTHHMMSSZ-xxxx(xxxx は16進の4文字)。同じ秒に2つ作っても重ならないようにする
    violation: なし
    source: [契約 C2, C4 の runId の形]
  - id: BR4.4
    statement: 保存した変更案を読むときは、版、必須の項目、変更の種類ごとの必須の項目、changeId の重なりを確かめ、知らない項目は無視する
    category: validation
    applies_to: SavedPlan の読み込み(U6、U8 が使う)
    trigger: 変更案のファイルの読み込み
    logic: IF version が 1 以外 THEN UnsupportedVersion(ファイルの version と toolVersion、読める version を案内)。IF 必須の項目がない、型が違う、列挙の値(kind、verdict、sourceKind)が知らない値、kind ごとの必須の項目がない、changeId が重なる、resources の arn が重なる、impactFindings の changeIds が知らない changeId を指す、fromKey・toKey が aws: で始まる(大文字・小文字を区別しない)、keyConflicts・unsetRequiredTags の項目の形が違う THEN InvalidInput(項目の場所と直し方)。知らない項目は無視する。読み込んだあと、変更の印(! と ?)と impacted を impactFindings から BR2.5 で求め直す(保存した impacted は使わない)
    violation: 終了コード 1
    source: [契約 C2, 契約の決まり「バージョン」, C12]
```

## 規則のまとめ

| ID | 規則 | 種類 | 出典 |
|---|---|---|---|
| BR1.1 | 値の別名を当てたあとで比べ、同じなら別名を正しいキー名に変える | calculation | FR3.1、Q3 |
| BR1.2 | 値が違えば衝突として変えない | calculation | Q3、U1 BR4.2 |
| BR1.3 | 正しいキー名の値が値の別名なら値を変える(キー名の変更のあと) | calculation | FR3.1、Q1 |
| BR1.4 | 必須タグは付いていないときだけ付ける。決まらなければ値が未指定 | calculation | FR3.1、FR3.3、Q2 |
| BR1.5 | 消すキーが付いていれば消す | calculation | FR2.4、FR3.1 |
| BR1.6 | aws: のキーは変えず、候補にも入れない | policy | 全体の決まり |
| BR1.7 | タグを変更できない種類は飛ばす | policy | AS3 |
| BR1.8 | 変更の順と changeId | calculation | C2、Q1 |
| BR1.9 | 変更後のタグは変更を順に当てて求める | calculation | C2、C4 |
| BR2.1 | 総数は重なっても1件。行は種類ごとに決めた単位でまとめる | calculation | FR3.2 |
| BR2.2 | 値が未指定はキーごとに件数と一覧 | calculation | FR3.3 |
| BR2.3 | どの規則にもないキーは消す候補 | calculation | FR3.4 |
| BR2.4 | IaC の印のある変更対象を警告 | policy | FR3.7 |
| BR2.5 | 影響は `!`、判定できないだけなら `?` | calculation | FR3.8 |
| BR2.6 | 終了コード(不完全 1、問題 2、消す候補だけなら 0) | policy | FR7.4、Q4 |
| BR2.7 | 変更がなければ影響分析をしない | policy | U1 BR4.4 |
| BR3.1 | 4つの形式は同じデータから作り件数を合わせる | constraint | FR3.5 |
| BR3.2 | 一覧の形の順 | policy | 画面4 |
| BR3.3 | 差分の形は全件を変更前と変更後で | policy | 画面5、Q1 |
| BR3.4 | JSON は契約 C3(plan の中身は C2 と同じ形) | constraint | FR3.5、FR7.5 |
| BR3.5 | Markdown は節と、畳んだ全件の表 | policy | Q6 |
| BR3.6 | 件数に3桁区切り(JSON は除く) | policy | Q8 |
| BR3.7 | 色は補助。NO_COLOR か端末でなければ色なし | policy | FR7.3、NFR11 |
| BR3.8 | 言語は --lang、OS の設定、英語の順 | calculation | FR7.2 |
| BR3.9 | --help 以外を日本語にする | policy | Q7 |
| BR3.10 | 結果は標準出力、ほかは標準エラー | constraint | FR7.5 |
| BR4.1 | --out は契約 C2 で保存。上書きしない | constraint | FR3.6、Q5 |
| BR4.2 | 保存する中身 | constraint | FR3.6 |
| BR4.3 | 変更案のIDの形 | calculation | C2 |
| BR4.4 | 保存した変更案の読み込みの確かめ | validation | C2、C12 |
