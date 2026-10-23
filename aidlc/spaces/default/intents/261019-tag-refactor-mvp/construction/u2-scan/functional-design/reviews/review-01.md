## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-21T14:49:57Z
**Iteration:** 1

対象: `construction/u2-scan/functional-design/` の functional-spec.md、rules.md、entities.md、traceability.json、functional-design-questions.md。上流は U1 の rules.md、U1 のコード(`internal/inventory`、`internal/rules/schema.go`、`internal/awsaccess/docs_test.go`、`docs/iam/readonly-policy.json`)、要件、ストーリー、U2 の範囲を照合しました。

判断: U2 を実装不能にする、または誤らせる high はありません。ただし、一度もタグが付いていないリソースの検出が一部のリージョンで失敗した場合の扱い、IAM 例の更新、`plan` への影響、Resource Explorer の件数上限の4点は、NFR 設計とコード生成の前に直す(または明示的に決める)べきです。

### Prior findings

| ID | Now | Severity | Note |
|---|---|---|---|
| なし | - | - | 初回のレビューのため、前回の指摘はありません |

### New findings

| ID | Severity (high/medium/low) | Location | Finding | Suggested fix |
|---|---|---|---|---|
| R-01 | medium | functional-spec.md W1 手順7、rules.md BR3.1/BR3.2、entities.md InventorySnapshot.untaggedDiscovery | 検出方法は「リージョンごとに Config、Resource Explorer の順」と書いてあるのに、結果は `untaggedDiscovery` という単一の enum(awsConfig / resourceExplorer / unavailable)で持つ。リージョン A は Config、B は Resource Explorer、C は不可、という混在や、一部のリージョンだけ検出に失敗した場合(拒否、再試行を使い切った失敗、時間切れ)を表せない。BR4.1 の `failedRegions` は GetResources の失敗だけを指すので、検出の失敗は終了コードにも結果の先頭にも出ない。必須タグの欠落件数は検出できなかった分だけ少なく出るのに、利用者に伝わらない | 検出の結果をリージョンごとに持つ(例 `untaggedDiscoveryByRegion: {region, method, status, reason}`)。「一部のリージョンで検出できなかった」状態を定義し、Summary に「untagged discovery partial」とリージョンを出す。終了コードを変えないなら、その根拠(FR1.3)と、必須タグの件数が下限であることを明記する |
| R-02 | medium | rules.md BR3.1(分岐の条件)、BR3.2 | 「AWS Config が使えれば」の意味が決まっていない。次の場合に Resource Explorer へ落とすのか、unavailable にするのかが読み取れない。(a) Config の記録は有効だが、記録する種類を絞っている、(b) Config の API が拒否された、(c) 一時的な失敗で再試行を使い切った。(a) の場合、Config だけで「全部探した」と表示して見落とす。Config と Resource Explorer の両方を使って合算するかどうかも書いていない | 分岐を表にする(記録なし、種類を絞っている、拒否、一時的失敗、成功)。拒否や失敗のときは Resource Explorer に落とす、種類を絞った Config のときは両方を試して ARN で重複を除く、などの方針を決める。方針の根拠が未確認なら、Assumptions に「uncertain」として追加する |
| R-03 | medium | rules.md BR3.1、Assumptions & Open Questions(Resource Explorer の索引) | Q1「上限なし」は、AWS 側にも結果の件数の上限があるか確認が要る。Resource Explorer の Search は、1回の検索で返す結果の総数に上限(私の記憶では 1,000 件。未確認)がある。この上限があると、ページを最後まで読んでも途中で切れて、利用者に分からないまま一度もタグが付いていないリソースを落とす。仕様は「最後まで読む」としか書いておらず、切れたことを検知する規則がない(OQ5 の本旨) | 上限の有無を AWS のドキュメントで確認し、あれば検索を分割する(リソースの種類やサービスごとの条件で複数回検索する)規則を足す。分割できないときは、「打ち切られた可能性がある」ことを示し(Q1 の A の趣旨を守りつつ)、R-01 の「partial」の状態にする |
| R-04 | medium | functional-spec.md(全体)、docs/iam/readonly-policy.json、rules.md BR1.5/BR3.2 | 新しく呼ぶ API の一覧と、読み取り用 IAM の例の更新が、仕様のどこにもない。AC1.2.1 は「ドキュメントにある読み取り用の IAM ポリシーだけで scan が動く」ことを求め、U1 の `docs_test.go` が例のポリシーを検査している。現在の例は `ec2:DescribeRegions`、`tag:GetResources`、`iam:ListPolicies`、`iam:GetPolicyVersion` だけ。U2 は少なくとも Config の記録状態の確認と問い合わせ、Resource Explorer の索引・ビューの確認と検索の API を追加で呼ぶ(正確な API 名は未確認)。この例を更新しないと、AC1.2.1 が U2 で通らない | U2 が呼ぶ API の一覧を機能仕様に書く(確認するものは例として `config:DescribeConfigurationRecorderStatus`、`config:SelectResourceConfig`、`resource-explorer-2:ListIndexes`、`resource-explorer-2:GetDefaultView`、`resource-explorer-2:Search`。実際の名前は NFR 設計で確定)。`docs/iam/readonly-policy.json` の更新と、`docs_test.go` の確認を U2 の成果物に入れる。拒否時のメッセージに API 名を出すことは BR3.2 のとおり |
| R-05 | medium | rules.md BR1.1(U1 の BR1.1 を書き換えている)、functional-spec.md W1 | U1 では `scan` と `plan` が同じ収集(`Inventory`)を使い、既定のリージョンは「標準の設定のリージョン」だった。U2 は既定を「全リージョン」に変えるが、`plan` がこの変更に従うかが書いていない。従うなら、`plan` も全リージョンの収集になり、読めなかったリージョンがあるときの `plan` の結果(変更案を出してよいか、影響分析と同様に不完全にして終了コード 1 にするか)が未定義。従わないなら、同じ `Inventory` を引数で切り替える設計が要る。U1 の動作(1リージョン、`Checked:` の文言「only the resources that the tagging API returns in us-east-1」)の黙った変更になる。AC1.1.9(scan と plan を続けて 10 分以内)も `plan` の収集を前提にしている | `plan` の収集の扱いを1か所に決めて書く。例: `plan` も U2 の収集を使い、failedRegions があれば変更案の前に「Results are incomplete」を出して終了コード 1(U1 BR6.3 と整合)。`Checked:` の行の新しい文言を `scan` と `plan` の両方で定義する(調べたリージョン、検出の方法、タグを変更できない種類)。回帰を防ぐため U1 の受け入れテストの扱いも書く |
| R-06 | medium | rules.md BR5.2、functional-spec.md「規則のまとめ」、internal/rules/schema.go(`required` は `default` と `when` を持つ) | 契約の `required` には `when`(条件)と `default` があり、U1 のルールファイルの読み込みは `required` に「この版では使わない」の警告を出す。U2 は「キーの名前だけ読む」としているが、(a) `when` 付きのキーも全リソースで数えると、条件に合わないリソースまで「欠落」にして過大に数える、(b) U1 の警告「"required" is not used in this version」を scan で出し続けるのか消すのかが不明、(c) `plan` は引き続き警告するのか、が決まっていない。AC1.1.7 の「テストデータの件数と一致する」に影響する | `when` のあるキーの扱いを決める(例: 条件を評価できない間は数えず、「N 件は条件付きのため scan では数えない」と示す)。警告の文言と対象のコマンドを規則に書く。`required` のキーが `aws:` で始まるときは、契約の決まりどおり無視して警告する(BR5.2 に明記) |
| R-07 | medium | rules.md BR5.4、BR5.3、functional-spec.md S2 と「業務の場面」 | 「問題なし」(No issues found)が、検出が不可・一部不可の場合や `--rules` なしの場合でも出る(終了コード 0)。「Clean: nothing found」は、検出を行わなかった状況と区別されない。また、一度もタグが付いていないリソースを見つけても、`--rules` がなければ issue として数えず、出力の例にも件数が出ない。AC1.1.2 の「結果に含まれる」が、合計の件数に入るだけで利用者には見えない | 一度もタグが付いていないリソースの件数を結果に出す(例 「Resources with no tags: N」)。それを issue とするかどうか(終了コード 2 にするか)を決めて書く。検出が不可・一部不可のときは「No issues found」ではなく「No issues found (untagged-resource discovery unavailable)」のように条件を付ける |
| R-08 | medium | rules.md BR3.3、BR5.2 | タグを変更できない種類(BR3.3)のリソースに、必須タグの欠落(BR5.2)を数える。これらは `apply` でも付けられないので、「欠落」は直せない件数になり、終了コード 2 の原因にもなる。また BR3.3 の「対応する種類の一覧」はツールの固定の一覧で、版ごとの更新を前提にしているが、Resource Explorer や Config が返す種類名(`AWS::EC2::Instance` 形式)とタグの API の ARN の種類(`ec2:instance` 形式)の対応の仕方が決まっていない(例の出力は `AWS::Example::Thing` の形式) | 必須タグの欠落の対象から、タグを変更できない種類を除くか、別の行として分けて示す。種類名の形式(CloudFormation 形式か ARN 形式か)を1つに決め、変換の規則(または一覧の形式)を entities.md に書く |
| R-09 | medium | rules.md BR3.1(ARN で重複を除く)、BR2.2 | 検出で見つけたリソースを「タグの API で返した ARN と同じものを除く」としているが、Config の項目には ARN がない種類があり、ARN の形式がタグの API と一致しない場合もある。ARN で突き合わせられないと、同じリソースが二重に数えられるか、反対に別のリソースが落ちる。BR2.2 の「region は最初に返したリージョン」も、リージョンを並行して読むため実行のたびに変わりうる(出力が決まらない) | ARN がないときの扱い(ARN を組み立てる、または照合できないものを別に数える)を決める。グローバルなリソースの region は、決まった規則(例 リージョン名の文字列の順で最初、または us-east-1)にする |
| R-10 | medium | rules.md BR4.1、BR4.2、functional-spec.md S1 | 「時間切れ」(timeout)の単位が決まっていない。API 1回の時間の上限なのか、1リージョン全体の上限なのか。BR4.2 の再試行が検出(Config、Resource Explorer)の呼び出しにも及ぶのかも書いていない。再試行を使い切った検出の失敗が、リージョンの失敗(終了コード 1)なのか検出の不可なのかも、R-01 とあわせて曖昧。また認証情報の期限切れが収集の途中で起きた場合、「認証情報の誤りだけは全体を止める」で終了コード 1 になるが、それまでに読んだ結果の扱い(出すか、捨てるか)がない | 時間の上限の単位(NFR5 の「すべての API 呼び出し」)を書く。検出の呼び出しも BR4.2 の対象とし、使い切ったときの分類を定める。途中の認証情報の期限切れは、読み終えた結果を出さずに終了する(部分的な結果を出さない)と明記する |
| R-11 | low | rules.md BR5.1、functional-spec.md 出力の例 | 略語の辞書は、キー全体が一致するときだけ使うので過剰なまとめは抑えられているが、`dev`(developer や device の意味で使うことがある)と `prod`(product)は別の意味で使われうる。辞書でまとめたものは終了コード 2 の原因になるため、誤ったまとめが「要確認」を生む。また、辞書でまとめたグループと、別名で定義される `plan` の名前の変更は一致しない(scan は候補、plan はルールファイルのとおり) | 出力に「(abbreviation match)」のような印を付けるか、辞書のまとめだけでは終了コード 2 にしない方針を決める。辞書の追加を利用者が行えるか(U3 以降)は範囲外として明記する。現在の見出し「candidates; confirm them in your rule file」は適切なので維持する |
| R-12 | low | rules.md BR1.2、functional-spec.md Summary の例 | 「調べた数 of 有効な数 regions」の「調べた数」が、読めなかったリージョンを含むのかが決まっていない。`--region` で絞った場合の表示(例 2 of 17)も例がない。また、重複した `--region`(同じ値の繰り返し)の扱い、有効でない値を1つでも含むときの誤りの範囲が、U1 の BR1.1 のとおりとだけある | 「調べた数 = 状態が済みのリージョンの数」と定義する(失敗は含まない)。`--region` の値は重複を除く。複数の値のうち1つでも無効なら、実行前に誤りとする(コンマ区切りを許すかも書く) |
| R-13 | low | rules.md BR6.1、functional-spec.md 進み具合の例 | 進み具合の「N resources so far」は重複を除く前の件数で、最終の Summary の件数と合わない可能性がある(BR2.2、BR3.1 で重複を除く)。端末の判定は標準エラーだけだが、標準出力を端末のままパイプしない場合など、混在時の確認がない。1行ずつ追記する形式の頻度(状態が変わるたび)は、リージョンが多いときに量が増える | 進み具合の件数は「暫定」であることを示す(so far)。端末でない場合の形式をテストで固定する。検出の段階(Config、Resource Explorer)も進み具合に出すかを決める(10,000 件で待ち時間が長くなりうる) |
| R-14 | low | traceability.json | AC1.1.9(性能)は BR2.1 と BR4.2 に対応づけているが、この性能は規則ではなく NFR 設計の同時実行数に依存する。AC1.1.10 は BR4.2、AC1.2.1 は BR2.1/BR3.1 に対応づけているが、AC1.2.1(IAM の例)は R-04 の成果物の更新が必要で、対応先に含まれていない | AC1.1.9 の対応先に「NFR 設計の同時実行数」を明記する。AC1.2.1 に IAM の例の更新(R-04)を対応づける |
| R-15 | low | functional-spec.md Assumptions & Open Questions | AWS の挙動に関する主な前提が「NFR の設計とコードの生成で確かめる」とあるが、機能設計の判断に影響するもの(Resource Explorer の集約索引とリージョンごとの索引の違い、`tag:none` が `aws:` のタグだけのリソースを含むか、Config の記録対象外の種類)が、`[assumption]` のまま確認の期限がない。特に、集約索引があるリージョンでは、全リージョンの結果が1か所の検索で返るので、リージョンごとに検索すると重複が増えて遅くなる | 確認する時点(NFR 設計の前、または検証用アカウントでの確認)を Assumptions に書き、集約索引がある場合は集約のリージョンで1回だけ検索する分岐を、BR3.1 に追加する |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| (実行できる検証ツールの指定なし) | 未実施 | 参照の確認は手作業で行った。traceability.json の AC ID(AC1.1.1〜AC1.1.10、AC1.2.1〜AC1.2.5)は、stories.md と一致する。BR ID は rules.md と一致する |

### Summary

U2 の流れ、状態、規則は、U1 の出力先の分け方(結果は標準出力、進み具合と案内は標準エラー)と終了コードの決め方(1 > 2 > 0)を保っており、AC の対応も揃っています。直すべき点は、一度もタグが付いていないリソースの検出がリージョンごとに失敗・混在したときの扱い(R-01〜R-03、R-10)、IAM の例の更新(R-04)、`plan` への影響(R-05)、必須タグの数え方(R-06、R-08)です。いずれも設計で決めれば実装可能なので READY としますが、R-01〜R-05 は次のステージへ進む前に反映することを勧めます。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
