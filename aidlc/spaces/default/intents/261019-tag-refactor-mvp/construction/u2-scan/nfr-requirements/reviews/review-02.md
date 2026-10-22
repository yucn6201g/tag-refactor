## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Iteration:** 2

対象: `construction/u2-scan/nfr-requirements/` の全ファイル(`security-requirements.md`、`performance-requirements.md`、`tech-stack-decisions.md`、`traceability.json`)。`functional-design/rules.md`(BR3.1、BR3.3、BR4.2、BR7.1)、U1 の `nfr-requirements/` と `nfr-design/`(`reliability-design.md`、`security-design.md`)、要件 NFR1、NFR5、NFR9、NFR10、NFR12 と突き合わせました。加えて AWS の公式ドキュメント(Resource Explorer の `ListResources`)を直接読んで確かめました。

主な結論: 前回の high 2件(R-01、R-02)の趣旨は大きく改善しました。しかし `ListResources` の公式ドキュメントを読むと、改訂の前提が2点で外れています。(1) 必要な IAM の操作が `resource-explorer-2:ListResources` ではなく `resource-explorer-2:Search` です。(2) `MaxResults` を 1000 にすると `NextToken` が返らず、1,000件で打ち切られます。どちらも、このまま実装すると「例だけのロールで動かない」または「件数の上限なしと書いたのに実際は1,000件で止まる」結果になるため、high として残します。

### Prior findings

### Prior findings

| ID | Now | Severity | Note |
|---|---|---|---|
| R-01 | Resolved | - | NFR1.6 が呼び出し回数の式、NFR1.7 がツール自身の処理時間、NFR1.8 が実測の L95 からの見積もりに分かれ、scan と plan(IAM の読み取り)も式に入りました。合否が数値で決まります。残る細部(式の抜け、決める順序)は新しい指摘 R-15、R-16 に移しました |
| R-02 | Partially resolved | high | 操作名が表に書かれ、`Select` の例外、NFR3.4 の表との照合も入りました。ただし表の `resource-explorer-2:ListResources` は、AWS のドキュメントでは必要な IAM の操作が `resource-explorer-2:Search` です(R-12)。偽の AWS が操作を取り出す方法の記述にも誤りがあります(R-14)。READY にはできません |
| R-03 | Partially resolved | medium | 検証用アカウントで Resource Explorer の索引と既定のビューを作る前提と費用が書かれました。一方で NFR3.5 の「すべての操作が1回成功する」と、「Config の問い合わせは偽の AWS だけで確かめる」が矛盾しています(R-17) |
| R-04 | Resolved | - | 最大10回の試行、待ち時間の上限20秒、1回の試行30秒、1回の呼び出し5分、使い切ったときと待ち時間の延びのテストが入りました。再試行の予算の確認は NFR の設計への持ち越しとして明示されており、許容します |
| R-05 | Resolved | - | 遅れを入れ替える偽の AWS、`-race`、Config を残す優先順位が NFR5.7 に入りました。進み具合の書き手を1つにする要件は落ちましたが、標準エラーは対象外と明記されており許容します。`-race` と `CGO_ENABLED=0` の食い違いは R-18 に移しました |
| R-06 | Partially resolved | low | Windows で仮想端末を有効にできなければ追記に戻る、Git Bash では追記にする、手元の端末での確認、が入りました。端末の判定を差し込み口にして書き換えの表示も CI で確かめること、`TERM=dumb` や片方だけが端末の組み合わせの期待が、まだありません |
| R-07 | Partially resolved | low | NFR8.9 が文字の集合(Cc、C1、Cf、Zl、Zp、不正な UTF-8、バックスラッシュ)と対象の文字列(進み具合を含む)を明記しました。一方、認証情報の期限切れとして扱う応答コードの一覧(Config と Resource Explorer を含む)の確認項目と、取り消し後に進行中の呼び出しが残らないことの確認は、まだありません |
| R-08 | Partially resolved | high | `ListResources` が `tag:none` 相当のフィルタ文字列を受け付けることは確かめられた、とのことです。ただし、その結果が `tech-stack-decisions.md` に出典付きで記録されていません。さらに、私が確かめたところ `MaxResults` の扱いに前提と食い違う点があります(R-13)。Config の `SelectResourceConfig` で利用者のタグがないものを表せるか、記録の設定の取得で全種類の記録を判定できるか、は依然として未確認です |
| R-09 | Resolved | - | `errgroup` の `SetLimit`、リージョンの失敗は結果に集める、認証情報の誤りだけを共有の取り消しで伝える、と決まりました |
| R-10 | Partially resolved | low | SDK のクライアントの名前が書かれました。表にない種類をタグを変更できるものとして扱う、という向きは理由とともに決まっています。ただし、表の整合性を確かめるテストと、表の更新の担当が要件にありません。また、実際にはタグを変更できない種類が表にないと、必須タグの欠落として数えられ、利用者が直せない終了コード 2 の原因になりえます(R-20) |
| R-11 | Resolved | - | NFR9 と NFR10 が OK になり、target が書かれました。NFR12 の持ち主は、配布の仕組みの別の作業(external-dependency-map.md の E6)と確認できました |

### New findings

### New findings

| ID | Severity (high/medium/low) | Location | Finding | Suggested fix |
|---|---|---|---|---|
| R-12 | high | `security-requirements.md` > 「U2 で新しく呼ぶ操作」の表、NFR3.3、NFR3.4。`functional-design/rules.md` > BR7.1 | AWS のドキュメント(`API_ListResources.html`)では、`ListResources` を呼ぶのに必要な最小の権限は `resource-explorer-2:Search`(リソースは `*`)で、`resource-explorer-2:ListResources` ではありません。表と BR7.1 と例のポリシーに `ListResources` と書くと、例だけのロールでは AccessDenied になり、scan の検出が常に「使えない」になります。さらに `Search` は `Select`、`Get`、`List`、`Describe` のどれにも当たらないため、NFR3.3 の接頭辞の検査をそのまま通りません。NFR3.4 の「記録された操作がコードの操作の表に含まれる」の照合も、API の操作名(`ListResources`)と IAM の操作名(`Search`)が別物なので、1つの表では足りません | 表に「API の操作名」と「必要な IAM の操作名」の2列を持たせ、Resource Explorer の一覧は IAM の操作として `resource-explorer-2:Search` を許すと書き直す。NFR3.3 の検査の例外を `Search` に広げる(許すのは `config:SelectResourceConfig` と `resource-explorer-2:Search` だけ)か、読み取りの判定を「コードの表で宣言した操作だけ」に改める。ほかの操作(`ListIndexes`、`GetDefaultView`、Config の2つ)の IAM の操作名も、Service Authorization Reference と照らして表に出典付きで記録する。BR7.1 も直す。`Search` の API は使わない、という BR3.1 の方針は API の話であり、IAM の権限とは別だと明記する |
| R-13 | high | `performance-requirements.md` > NFR1.6、NFR1.10。`tech-stack-decisions.md` > Resource Explorer。`functional-design/rules.md` > BR3.1 | 同じドキュメントに、「`MaxResults` を 1000 にすると `ListResources` は `NextToken` を返さない」とあります(最大値は 1000)。「1ページ最大1,000件で、続きの印で全件」という前提のまま1,000件のページで読むと、実際の AWS では1,000件で黙って打ち切られ、「件数の上限なく読む」(NFR1.10)に反します。偽の AWS で「2,500件を3ページで返す」テストは、この制約を再現しないため、通っても実機では誤りになります。NFR1.6 の式 ⌈件数÷1,000⌉ も、ページの大きさが決まっていないため成り立ちません。加えて、既定のビューがない、または使えないリージョンは 401 で失敗します(ドキュメント)。BR3.1 の「索引と既定のビューがある」の判定と、この 401 を `unavailable` に倒すことが、要件にありません | NFR1.10 に、`MaxResults` を 1000 未満の固定値(例 100〜500。値は NFR の設計で決める)にして `NextToken` が null になるまで読む、と明記する。NFR1.6 の式を、その値を使った ⌈件数÷固定のページの大きさ⌉ に直す。偽の AWS が「`MaxResults` が 1000 のときは `NextToken` を返さない」振る舞いを再現し、ツールが 1000 を指定したらテストが失敗するようにする。検証用アカウントで1,000件を超える結果が最後まで読めることの確認(または確認できない範囲の明記)を NFR1.10 の合否に入れる。401 を「使えない」に倒すことと、その理由の表示を NFR5.6 に足す。確かめた結果は `tech-stack-decisions.md` に出典付きで書く |
| R-14 | medium | `security-requirements.md` > NFR3.4 | 「AWS Config と Resource Explorer の要求は JSON の本文で操作を指す形なので、偽の AWS は要求の見出しから操作の名前を取り出す」は、Resource Explorer には当てはまりません。Resource Explorer は REST の形(`POST /ListResources`、`POST /ListIndexes`、`POST /GetDefaultView`)で、操作は見出しではなく URL のパスで決まります。見出し(`X-Amz-Target`)で操作を指すのは Config です。この書き方のまま実装すると、偽の AWS が Resource Explorer の操作名を取り出せず、NFR3.4 の照合が実質何も確かめません | NFR3.4 の記述を「Config は `X-Amz-Target` の見出し、Resource Explorer は URL のパス」に直し、両方から操作名を取り出せることを合否の基準に入れる |
| R-15 | medium | `performance-requirements.md` > NFR1.6 | 呼び出し回数の式に、Config の `SelectResourceConfig` の問い合わせのページ数が入っていません。BR3.1 では、Config が全種類を記録しているリージョンでは Config が先に使われます。式は Resource Explorer のページ数だけで、Config 経路の上限が決まりません。さらに、タグの取得のページ ⌈件数÷100⌉ は件数が0件のリージョンで0になり、実際には各リージョンで最低1回は呼びます。実機では、ページが100件に満たないまま続きの印が返ることもあります | 式に Config の問い合わせのページ数(ページの大きさは NFR の設計で決める)を足す。各リージョンの最低1回を式に入れる。偽の AWS のテストで、Config 経路と Resource Explorer 経路の両方を確かめる |
| R-16 | medium | `performance-requirements.md` > NFR1.8、NFR1.9 | 順序がつながっていません。NFR1.9 は同時に読む数を「NFR の設計で、NFR1.8 の見積もりと呼び出し回数の上限の両方から決める」としますが、NFR1.8 は検証用アカウントで実際のコードを動かして測った L95 が必要で、これは NFR の設計の後、コードの生成と U2 のチェックポイントの後にしか得られません。また、検証用アカウントは件数が少ないため、1ページ100件のタグの取得や1,000件近いページの時間が、1万件のときの L95 より短く出る恐れがあります。見積もりが10分を超えたときの動作(同時に読む数を変える、など)も書かれていません | NFR1.9 を、NFR の設計では「仮の L と根拠を明記して数を決め、チェックポイントの NFR1.8 で確かめる」と改める。見積もりが10分を超えた場合の対応(数の見直しと再測定)を書く。L95 は、実際のページの大きさに近い応答で測るか、小さい件数で測った値が過小になりうることを安全率で扱うと明記する |
| R-17 | medium | `security-requirements.md` > NFR3.5 | 合否の基準が「新しく足した操作がすべて、例だけを付けたロールで少なくとも1回成功する」ですが、本文では AWS Config を有効にせず、「Config の問い合わせは偽の AWS で確かめる」としています。これでは `config:SelectResourceConfig` が実際の IAM で許されることも、操作名の正しさも、実機で確かめられません(R-12 と同じ種類の誤りを見逃します) | 基準を「Config の問い合わせは、記録がないことで失敗してもよい。ただし AccessDenied ではなく別の種類の失敗であること(権限があることの証拠)を確かめる」に改める。または、検証できない操作を明記して、人が一度だけ Config を有効にして確かめる手順を足す |
| R-18 | low | `performance-requirements.md` > NFR5.7。`tech-stack-decisions.md`(U1 の `CGO_ENABLED=0`) | Go のレース検出(`-race`)は cgo が有効なビルドが必要です。U1 の方針は `CGO_ENABLED=0` で、NFR5.7 の「`-race` 付きでも通る」をどの OS のどのジョブで実行するかが書かれていません。Windows では C のコンパイラも要ります | `-race` を実行するジョブ(例: Linux の CI のテストだけ、cgo を有効にする。配るバイナリは `CGO_ENABLED=0` のまま)を書く |
| R-19 | low | `performance-requirements.md` > NFR5.5。`tech-stack-decisions.md` > 呼び出し回数の上限への対応 | NFR5.5 は「U1 の NFR5.1 の『1回の呼び出しに120秒』を置き換える」としますが、U1 の NFR5.1 の文は「1回の呼び出しにつき30秒」で、120秒は `nfr-design/reliability-design.md` にある設計の値です。参照が不正確です。また、再試行の設定と時間の上限の変更は U1 で作った `awsaccess` のコードの変更を伴うことが、要件に書かれていません | 参照を「U1 の `reliability-design.md` の120秒」に直し、`awsaccess` の設定を変えて U1 のテストも更新することを書く |
| R-20 | low | `tech-stack-decisions.md` > 種類の対応表とタグを変更できない種類の一覧。`functional-design/rules.md` > BR3.2、BR3.3 | 「一覧にない種類はタグを変更できるものとして扱う」は、実際にはタグを変更できない新しい種類が、必須タグの欠落として数えられる向きにずれます。BR3.2 は必須タグの欠落の件数を「下限」と説明していますが、この向きでは過大になることもあり、説明が合いません。利用者が直せない欠落で終了コードが 2 になりえます。表の形式、重複、ARN の形を確かめるテストと、表の更新の担当も、要件にありません | 向きの選択は妥当ですが、BR3.2 の「下限」の説明を直し、結果の表示に「種類の一覧が古い可能性」の注意を出すか検討する。表の整合性のテストと更新の担当を要件に足す |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| なし | - | この段階にはスクリプトの検証ツールがありません。AWS のドキュメント(`API_ListResources.html`)を直接読んで、R-12、R-13 の根拠にしました |

### Summary

前回の指摘の多くは解消または改善しました。しかし、`ListResources` に必要な IAM の操作は `resource-explorer-2:Search` であり、`MaxResults` を 1000 にすると `NextToken` が返らず1,000件で打ち切られます。この2点(R-12、R-13)が残る限り、U2 は例だけのロールで動かない、または件数の上限なしという要件を満たさないため、NOT-READY とします。R-14 から R-17 は同じ改訂で直せるはずです。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
