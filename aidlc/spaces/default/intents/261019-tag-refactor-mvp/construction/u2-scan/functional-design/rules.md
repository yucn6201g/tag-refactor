# 業務の規則: U2 収集

U2 で完成させる `scan` と、`plan` の収集の規則です。U1 の規則(`construction/u1-walking-skeleton/functional-design/rules.md`)のうち、ここで書き換えないもの(認証情報、出力先、ルールファイルの安全な読み方など)は、そのまま使います。ID はこの作業単位の中での番号です。

出典: 要件 FR1.1〜FR1.8、FR7.4〜FR7.8、NFR1、NFR3、NFR5、NFR8、AS1、AS3、OQ5、契約 C1・C3・C5・C6・C7・C11・C12、`functional-design-questions.md` Q1〜Q3、AWS のドキュメント(タグの取得の API、Resource Explorer)。レビュー(`reviews/review-01.md`)の R-01〜R-15 を受けて改訂しました。

```yaml
rules:
  # 1. 読むリージョン
  - id: BR1.1
    statement: scan と plan は、既定ではアカウントで有効なすべてのリージョンを読む。--region を1回以上指定したときは、指定したリージョンだけを読む
    category: policy
    applies_to: InventorySnapshot.scannedRegions
    trigger: scan と plan の開始時
    logic: IF --region がない THEN 有効なリージョン(オプトインで有効にしたものを含む)をすべて読む ELSE 指定したリージョンだけ。同じ値の繰り返しは1つにまとめる。値は1回の --region に1つ(コンマ区切りは使わない)。1つでも有効でない値があれば、AWS のリソースを読む前に U1 の BR1.1 のとおり誤りにする
    violation: InvalidInput、終了コード 1
    source: [FR1.1, AC1.1.1, U1 の機能仕様の仮定, review-01 R-05, R-12]
  - id: BR1.2
    statement: 調べたリージョンの数と、有効なリージョンの数を示す。調べた数は「済み」になったリージョンの数で、読めなかったリージョンは含めない
    category: policy
    applies_to: scan と plan の結果
    trigger: 結果の表示
    logic: Summary の行に「<済みの数> of <有効な数> regions」を出す。--region で絞ったときも分母は有効な数(例 2 of 17)
    violation: なし
    source: [FR1.1, AC1.1.1, review-01 R-12]

  # 2. タグの付いたリソースの収集(scan と plan で共通)
  - id: BR2.1
    statement: 各リージョンで、タグの取得の API が返すリソース(タグが付いている、またはかつて付いていたもの)を最後まで読む
    category: constraint
    applies_to: Resource
    trigger: リージョンごとの収集
    logic: 続きの印が空になるまで読む(U1 と同じ)。返ったリソースは everTagged = true
    violation: 読めなければ BR4.1
    source: [FR1.1, AWS のドキュメント(GetResources は一度もタグが付いていないリソースを返さない)]
  - id: BR2.2
    statement: グローバルなリソース(ARN にリージョンがないもの)は、ARN で重複を除いて1回だけ数える
    category: calculation
    applies_to: Resource
    trigger: すべてのリージョンの収集のあと
    logic: 同じ ARN が複数のリージョンから返ったら1件にし、global = true、region は返したリージョンのうち名前の文字列の順で最初のもの(毎回同じ結果になるように)
    violation: なし
    source: [契約 C7, review-01 R-09]
  - id: BR2.3
    statement: リソースに aws:cloudformation:stack-name のタグがあれば、CloudFormation で管理されている印を付ける
    category: calculation
    applies_to: Resource.iacManagedBy
    trigger: 収集のあと
    logic: iacManagedBy = "cloudformation:<スタック名>"。このタグは表記ゆれや必須タグの判定には使わない(aws: のキー)
    violation: なし
    source: [FR3.7 の前提, 契約 C7]
  - id: BR2.4
    statement: リージョンは同時に読む。同時に読む数は、性能の目標(NFR1)と呼び出し回数の上限を両立する値を NFR の設計で決める
    category: constraint
    applies_to: Inventory
    trigger: 収集
    logic: 決まった数までのリージョンを同時に読み、終わったら次のリージョンを読む
    violation: なし
    source: [NFR1, AC1.1.9, review-01 R-14]
  - id: BR2.5
    statement: リソースの種類は、ARN の形(サービス:種類、例 ec2:instance)で持つ。AWS Config が返す CloudFormation の形(AWS::EC2::Instance)は、ツールが持つ対応表で ARN の形に変える。対応表にない種類は、AWS Config の形のまま持つ
    category: calculation
    applies_to: Resource.resourceType
    trigger: 収集と検出
    logic: タグの取得の API と Resource Explorer の結果は ARN の形。AWS Config の結果だけ変換する
    violation: なし
    source: [U1 の BR2.1, review-01 R-08]

  # 3. 一度もタグが付いていないリソース(scan だけ)
  - id: BR3.1
    statement: 一度もタグが付いていないリソースは、リージョンごとに、下の表の順で探す。件数の上限は設けず、最後まで読む
    category: policy
    applies_to: UntaggedDiscoveryRegion
    trigger: タグの付いたリソースの収集のあと
    logic: |
      Resource Explorer に集約の索引(全リージョンを1か所で検索できるもの)があれば、そのリージョンで1回だけ探し、結果をリージョンに振り分ける。
      それ以外は、リージョンごとに次の順で試す。
        1. AWS Config がすべての種類を記録している → Config の問い合わせで、利用者のタグがないリソースを探す(method = awsConfig)
        2. Config が記録していない、一部の種類だけを記録している、拒否された、再試行を使い切って失敗した → Resource Explorer を試す
        3. Resource Explorer の索引と既定のビューがある → 件数に上限のない一覧の API(ListResources)で、条件 tag:none のリソースを、続きの印がなくなるまで読む(method = resourceExplorer)。1ページの件数は 1,000 未満の固定の値にする(1,000 を指定すると続きの印が返らず、1,000 件で打ち切られるため)。1,000 件までに限られる検索の API(Search)は呼ばない。既定のビューがない、または使えないリージョンで一覧が 401 を返したときは、Resource Explorer は使えないとする
        4. どちらも使えない → status = unavailable(理由は両方の分を記録する)
      Config が一部の種類だけを記録していて Resource Explorer も使えるときは、両方の結果を合わせる。
      見つけたリソースのうち、タグの取得の API で既に返したもの(ARN が同じ)は除く。ARN がないものは、タグを変更できない種類として数える(BR3.3)。見つけたリソースは everTagged = false、タグは空
    violation: なし(使えないリージョンは BR3.2)
    source: [FR1.2, AC1.1.2, Q1, 要件 OQ5, AWS のドキュメント(Resource Explorer の tag:none、Search は 1,000 件まで、ListResources は上限なし・MaxResults 1000 では NextToken なし・既定のビューがなければ 401), review-01 R-02, R-03, R-09, R-15]
  - id: BR3.2
    statement: 一度もタグが付いていないリソースを探せなかったリージョンがあれば、そのリージョンと理由を示し、残りの結果を出す。これだけでは終了コードを変えない。必須タグの欠落の件数は、探せた範囲での件数であり、探せなかったリージョンの分だけ実際より少ないことがある(下限)と示す。なお、タグを変更できないと分かっていない種類は欠落として数えるため(BR3.3)、その分は実際より多いこともある
    category: policy
    applies_to: InventorySnapshot.untaggedDiscovery
    trigger: BR3.1 で status が unavailable か failed のリージョンがあるとき
    logic: すべてのリージョンで探せた → 全体の状態 complete。一部のリージョンだけ探せなかった → partial。すべて探せなかった → unavailable。partial と unavailable のときは、リージョンと理由(足りない API の名前を含む)を出す
    violation: 終了コードには影響しない(BR5.3)
    source: [FR1.3, AC1.1.3, 契約 C5 の例外, review-01 R-01]
  - id: BR3.3
    statement: 見つけたリソースのうち、タグの取得の API が対応していない種類(または ARN のないもの)は、「タグを変更できない種類」として種類ごとの件数を示す
    category: calculation
    applies_to: InventorySnapshot.unsupportedResourceTypes
    trigger: BR3.1 のあと
    logic: ツールが持つ「タグを変更できないと分かっている種類」の一覧(ARN の形)にある種類と、ARN のないものを数える。一覧にない種類はタグを変更できるものとして扱う(一覧が古くなっても、必須タグの欠落を少なく数える向きにずれないため。U2 の NFR 要件の tech-stack-decisions.md)。その代わり、一覧が古いと、実際にはタグを変更できない種類が欠落として多めに数えられうる。一覧はツールの版ごとに見直す
    violation: なし
    source: [AS3, 契約 C3, review-01 R-08]

  # 4. 読めなかったリージョンと、AWS の失敗
  - id: BR4.1
    statement: タグの取得の API でリージョンを読めなかったとき(拒否、再試行を使い切った一時的な失敗、時間切れ)は、そのリージョンと理由を記録し、他のリージョンの収集を続ける
    category: policy
    applies_to: InventorySnapshot.failedRegions
    trigger: リージョンごとの収集の失敗
    logic: failedRegions に(リージョン、理由、API の名前)を加える。一度もタグが付いていないリソースの検出の失敗は、ここではなく BR3.2 で扱う
    violation: 結果を不完全にする(BR5.3)
    source: [FR1.5, AC1.1.5, FR7.7, AC1.2.2, review-01 R-01]
  - id: BR4.2
    statement: すべての AWS の呼び出し(タグの取得、AWS Config、Resource Explorer)は、呼び出し回数の上限の応答のとき、待ち時間を延ばしながら再試行する。時間の上限は1回の呼び出しごとにかける
    category: policy
    applies_to: AwsAccess
    trigger: 呼び出し回数の上限の応答
    logic: 待ち時間を回数とともに延ばし(揺らぎを入れる)、決まった回数まで再試行する。時間の上限は U1 と同じく1回の試行と1回の呼び出し(再試行を含む)にかける。回数と時間の値は NFR の設計で決める
    violation: 使い切ったら、タグの取得なら BR4.1、検出なら BR3.2
    source: [NFR5, AC1.1.10, 契約 C6, review-01 R-10]
  - id: BR4.3
    statement: 認証情報の誤り(なし、期限切れ)は、すべてのリージョンに共通なので、収集の途中でも全体を止める。それまでに読んだ結果は出さない
    category: policy
    applies_to: Cli
    trigger: CredentialsMissing が返ったとき
    logic: 収集を止め、U1 の W3 のとおり2行の案内を出して終了コード 1。部分的な結果は標準出力に出さない
    violation: 終了コード 1
    source: [FR7.8, AC1.2.3, review-01 R-10]

  # 5. 表記ゆれ、必須タグ、結果
  - id: BR5.1
    statement: キーの表記ゆれは、大文字・小文字と区切り文字の違いに加えて、略語の辞書でまとめる。辞書はキー全体が略語と一致するときだけ使う。辞書を使って初めてまとまったものには印を付ける
    category: calculation
    applies_to: KeyVariantGroup
    trigger: scan の結果の作成
    logic: 正規化(小文字、-・_・空白を除く)したキーが辞書の略語と一致すれば、正式な形に置き換えてからまとめる。辞書: env→environment、prod→production、dev→development、stg→staging。キーの一部だけ(例 envname)は置き換えない。aws: のキーは対象外。辞書を使わなければ書き方が1つだけだったまとまりは「(abbreviation match)」の印を付け、終了コード 2 の理由にはしない
    violation: なし
    source: [FR1.6, AC1.1.6, Q2, review-01 R-11]
  - id: BR5.2
    statement: --rules を指定したときは、ルールファイルの required のキーごとに、そのキーが付いていないリソースの件数を数える。required は「すべてのリソースに必要なキー」で、default と when は付ける値の決め方なので、件数の対象は絞らない。キーは大文字・小文字を区別して比べる
    category: calculation
    applies_to: MissingRequiredTag
    trigger: scan の結果の作成
    logic: 対象は、タグの付いたリソースと、一度もタグが付いていないリソースの両方。ただし、タグを変更できない種類(BR3.3)のリソースは数えず、別に「(N more on resource types that cannot be tagged)」と示す。aws: で始まる required のキーは無視して警告する。scan では required を使うので、U1 の「required は使わない」の警告は出さない(plan は U3・U4 まで出す)。--rules がなければ「Required tags missing: not checked (no rule file)」と示す
    violation: ルールファイルの誤りは U1 の BR3.1 のとおり
    source: [FR1.7, FR2.3, FR3.3, AC1.1.7, Q3, AS1, 契約 C1, review-01 R-06, R-08]
  - id: BR5.3
    statement: scan の終了コードは、エラーか不完全(タグの取得で読めなかったリージョンがある)なら 1、そうでなく表記ゆれ(辞書だけのまとまりを除く)か必須タグの欠落があれば 2、どちらもなければ 0。一度もタグが付いていないリソースを探せなかったこと、タグを変更できない種類があること、一度もタグが付いていないリソースがあることだけでは変えない
    category: calculation
    applies_to: CommandOutcome.exitCode
    trigger: scan の終わり
    logic: IF error OR failedRegions が空でない THEN 1 ELSE IF 表記ゆれ OR 欠落 THEN 2 ELSE 0
    violation: なし
    source: [FR7.4, FR1.3, FR1.8, AC1.1.3, AC1.1.8, 契約 C5]
  - id: BR5.4
    statement: 問題がなければ「問題なし」と、調べたリソースの件数を示す。一度もタグが付いていないリソースを探せなかったリージョンがあれば、そのことを添える
    category: policy
    applies_to: scan の結果
    trigger: 終了コードが 0 のとき
    logic: 「No issues found」と件数を出す。BR3.2 が partial か unavailable なら「No issues found (untagged-resource discovery: <状態>)」とする
    violation: なし
    source: [FR1.8, AC1.1.8, review-01 R-07]
  - id: BR5.5
    statement: 一度もタグが付いていないリソースの件数を、結果に出す
    category: policy
    applies_to: scan の結果
    trigger: scan の結果の作成
    logic: 「Resources with no user tags: N」の行を出す。この件数だけでは終了コードを変えない(必須タグの欠落として数えたものは BR5.2)
    violation: なし
    source: [AC1.1.2, review-01 R-07]
  - id: BR5.6
    statement: plan も U2 の収集(全リージョン)を使う。plan では、一度もタグが付いていないリソースは探さない。タグの取得で読めなかったリージョンがあれば、結果の先頭に「Results are incomplete」とリージョンを出し、終了コード 1 にする
    category: policy
    applies_to: plan
    trigger: plan の収集
    logic: 名前の変更は、タグの付いたリソースだけが対象なので、一度もタグが付いていないリソースは要らない(必須タグを付ける変更は U4 で扱う)。終了コードは U1 の終了コードの規則のとおり 1 が最優先
    violation: 終了コード 1
    source: [U1 の終了コードの規則, AC1.1.9, review-01 R-05]
  - id: BR5.7
    statement: 調べた範囲の「Checked:」の行を、scan と plan の両方で、調べたリージョンと、使った方法を含めて出す
    category: policy
    applies_to: 標準エラーの出力
    trigger: 結果のあと
    logic: scan は「Checked: tagged resources in <済みの数> of <有効な数> regions (tagging API); never-tagged resources via <方法の内訳>」。plan は「Checked: tagged resources in <済みの数> of <有効な数> regions (tagging API), and customer managed IAM policies attached to a user, role or group」(変更がなければ U1 と同じく「no IAM policies were read: nothing to change」)
    violation: なし
    source: [U1 のレビュー R-04, review-01 R-05]

  # 6. 進み具合
  - id: BR6.1
    statement: リージョンごとの状態(済み、実行中、待ち、失敗)と経過時間を標準エラーに出す。端末なら同じ場所を書き換え、端末でなければ状態が変わるたびに1行ずつ追記する。件数は重複を除く前の暫定の値として「so far」を付ける
    category: policy
    applies_to: 進み具合の通知口(契約 C11)
    trigger: 収集の間
    logic: 端末かどうかは標準エラーで判定する。一度もタグが付いていないリソースを探す段階も「discovering never-tagged resources」として出す。U2 は色を使わない(色と NO_COLOR は U4)
    violation: なし
    source: [FR1.4, AC1.1.4, モックアップ 画面1, review-01 R-13]

  # 7. 権限
  - id: BR7.1
    statement: U2 で新しく呼ぶ AWS の読み取りの操作を、読み取り用の IAM ポリシーの例に足し、例だけで scan と plan が動くようにする
    category: authorization
    applies_to: docs/iam/readonly-policy.json
    trigger: U2 のコードの生成
    logic: 足す IAM の操作は U2 の NFR 要件(security-requirements.md の表)のとおり。config:DescribeConfigurationRecorders、config:DescribeConfigurationRecorderStatus、config:SelectResourceConfig、resource-explorer-2:ListIndexes、resource-explorer-2:GetDefaultView、resource-explorer-2:GetView、resource-explorer-2:Search。最後の1つは、一覧の API(ListResources)を呼ぶのに AWS が求める IAM の操作で、検索の API(Search)を呼ぶためではない(BR3.1 で呼ばないのは API の Search)。すべて読み取りの操作で、U1 のテスト(読み取りだけ、* を含まない、コードの操作の表と一致)を通す。Get、List、Describe で始まらない操作は config:SelectResourceConfig と resource-explorer-2:Search の2つだけを許す
    violation: 例にない操作を呼ぶと、例だけのロールで AccessDenied になる
    source: [NFR3, AC1.2.1, U1 の NFR3.2, review-01 R-04, AWS のドキュメント(ListResources の最小の権限は resource-explorer-2:Search)]
```

## 規則のまとめ

| ID | 規則 | 種類 | 出典 |
|---|---|---|---|
| BR1.1 | 既定は全リージョン。--region で絞れる(重複はまとめ、無効な値は読む前に誤り) | policy | FR1.1 |
| BR1.2 | 済みのリージョンの数と有効な数を示す | policy | AC1.1.1 |
| BR2.1 | タグの取得の API で最後まで読む | constraint | FR1.1 |
| BR2.2 | グローバルなリソースは ARN で1回だけ。region は決まった規則 | calculation | C7 |
| BR2.3 | CloudFormation の印はスタック名のタグで | calculation | FR3.7 |
| BR2.4 | リージョンは同時に読む(数は NFR の設計) | constraint | NFR1 |
| BR2.5 | 種類は ARN の形。Config の形は対応表で変える | calculation | AS3 |
| BR3.1 | 一度もタグが付いていないものは、集約の索引、Config、Resource Explorer の順で、上限なく探す | policy | FR1.2、Q1 |
| BR3.2 | 探せなかったリージョンは理由を示す。終了コードは変えない。欠落は探せた範囲の件数 | policy | FR1.3 |
| BR3.3 | タグを変更できない種類と件数 | calculation | AS3 |
| BR4.1 | タグの取得で読めなかったリージョンは理由を記録して続ける | policy | FR1.5 |
| BR4.2 | すべての呼び出しで、待ち時間を延ばして再試行 | policy | NFR5 |
| BR4.3 | 認証情報の誤りは途中でも止め、部分的な結果は出さない | policy | FR7.8 |
| BR5.1 | 略語の辞書でキーをまとめる。辞書だけのまとまりは印を付け、終了コードの理由にしない | calculation | FR1.6、Q2 |
| BR5.2 | required のキーの欠落をすべてのリソースで数える(タグを変更できない種類は別に示す) | calculation | FR1.7、Q3 |
| BR5.3 | scan の終了コードは 1 > 2 > 0 | calculation | FR7.4 |
| BR5.4 | 問題がなければ「問題なし」。探せなかったリージョンがあれば添える | policy | FR1.8 |
| BR5.5 | 一度もタグが付いていないリソースの件数を出す | policy | AC1.1.2 |
| BR5.6 | plan も全リージョンを読む。読めなかったリージョンがあれば終了コード 1 | policy | U1 の終了コードの規則 |
| BR5.7 | Checked: の行に調べたリージョンと方法を出す | policy | U1 R-04 |
| BR6.1 | 進み具合は端末なら書き換え、そうでなければ追記。件数は暫定 | policy | FR1.4 |
| BR7.1 | 新しい読み取りの操作を IAM の例に足す | authorization | NFR3、AC1.2.1 |
