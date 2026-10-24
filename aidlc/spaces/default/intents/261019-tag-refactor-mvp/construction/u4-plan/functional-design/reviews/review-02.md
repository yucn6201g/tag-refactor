## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T06:14:01Z
**Iteration:** 2

### Prior findings

| Severity | Location | Finding | Status |
|---|---|---|---|
| high | `functional-spec.md` > 流れ W1 の 11、表示の例 / `rules.md` > BR3.10 | 案内の行(終了コード、次の手順)を標準出力に出す書き方が BR3.10 と U1 の決まりに反していた | 解決済み: 「出力先の表」を行ごとに追加。Exit code、次の手順、Saved plan、アカウントの行は標準エラー、不完全の印と Summary は標準出力と決めた。U1 BR6.1 と現在のコード(不完全の印は標準出力)と一致する。`> plan.md` に案内が混ざらない場面も追加済み |
| high | `rules.md` > BR1.1、BR1.9 / 契約 C2 の kind 表 | 値の別名を入れると、mergesIntoExisting が true の renameKey の書き込みが U6 と変更後のタグで食い違う | 解決済み: true なら toKey に書かず fromKey を消すだけと BR1.1、BR1.9、契約への変更の表に明記。3 パターンと衝突の例の表で、変更後のタグがすべて `Environment=prod` になることを追える。U1 の変更案は値の別名を使わないため、旧い読み方でも同じになるという説明も筋が通る |
| medium | `rules.md` > BR3.4 / `entities.md` > PlanSummary | JSON の details に keyConflicts がなく、summary の conflictResources 等も契約にない | 解決済み: BR3.4 に keyConflicts を加え、契約への変更の表で C2(`arn`、`canonicalKey`、`presentTags`)と C3(summary の 2 項目、failedRegions と unsupportedResourceTypes を plan にも入れる)を具体化した。ただし契約の本文への反映先は下の新しい指摘を参照 |
| medium | `traceability.json` > AC6.1.1 | ヘルプの確かめを U6・U7 に先送りしたが、US6.1 の割り当て先がなかった | 解決済み: `unit-of-work-story-map.md` の「複数の作業単位にまたがるもの」に U4・U6・U7 の行を追加し、traceability の Deferred の理由にも反映済み |
| medium | `rules.md` > BR1.4 | addRequired の値に値の別名を当てておらず、plan、apply、plan が収束しない | 解決済み: BR1.4 で toValue に正しい値の照会を当てると決め、理由と業務の場面(apply のあとに plan すると変更は不要)も追加済み |
| medium | `rules.md` > BR2.1、`entities.md` > ChangeLine | 一覧の行をまとめる単位が種類ごとの表示と食い違っていた | 解決済み: BR2.1 に種類ごとのまとめる単位を書き、renameKey と removeKey の fromValue でまとめを分けないこと、addRequired の「(from rules)」も明記。ChangeLine と一致 |
| medium | `rules.md` > BR1.6、BR2.3 | `aws:` の判定が大文字・小文字を区別するように読めた | 解決済み: 区別せず、U2・U3 と同じ判定の部品を使うと BR1.6、BR2.3、BR4.4 に明記し、`AWS:Foo` の場面も追加済み |
| low | `rules.md` > BR2.7、W1 の 7 | 衝突だけで変更がないときも影響分析を呼ぶ書き方で、現在のコードと食い違っていた | 解決済み: 条件を「changes が空ならポリシーを読まない」に直した。`internal/cli/commands.go` の `len(plan.Changes) > 0` と一致し、場面も追加済み |
| low | `rules.md` > BR4.4 | 列挙値、keyConflicts、重なり、`aws:` の検査と mark の求め直しがなかった | 解決済み: BR4.4 に列挙値、kind ごとの必須項目、ARN と changeId の重なり、changeIds の参照先、`aws:` のキー、keyConflicts・unsetRequiredTags の形の検査と、読み込み後に mark と impacted を求め直す規則を追加 |
| low | `functional-spec.md` > W1 の 1、W2 | `--out` の保存先のフォルダの確認が収集のあとになっていた | 解決済み: AWS を呼ぶ前に、既存ファイルの有無と一時ファイルの作成を確かめる流れに直した。一時ファイルの後始末が一部の経路で未定義なので、新しい指摘に記した |
| low | `traceability.json` > AC6.1.4 | N/A のため、`scan --format json` の `exitCode` の一致を確かめる場所がなかった | 解決済み: OK に変え、BR3.4、BR2.6 を対応先にし、仕様の「Assumptions」にも書いた |

### New findings

| Severity | Location | Finding | Required action |
|---|---|---|---|
| medium | `functional-spec.md` > 契約への変更 / `inception/contract-design/contract-summary.md` > C2 の kind 表、C3 | C2 と C3 への変更(keyConflicts、mergesIntoExisting、renameKey の意味の変更、summary の 2 項目)は U4 の仕様の中にしか書かれていない。U6・U8 の設計が読む `contract-summary.md` の kind 表は今も「renameKey は値はそのまま」で、mergesIntoExisting も keyConflicts もない。引き継ぎ行は AC6.1.1 のヘルプにはあるが、契約の変更にはない。U6 が旧い本文を読んで実装すると、前回の high と同じ食い違い(true のときに toKey を上書きする)が戻る | `unit-of-work-story-map.md` の行と同じやり方で、契約の変更が U6・U8 の入力に届く場所を決めて書く(例: `contract-summary.md` の C2、C3 に追記する、または未決事項の表に「U4 の契約変更を U6・U8 の設計で参照する」を足す) |
| low | `functional-spec.md` > 流れ W1 の 1、W2 / `rules.md` > BR4.1 | 一時ファイルは W1 の 1 で最初に作って保存まで持つが、後始末は保存の失敗(W2 の 3)でしか決めていない。ルールファイルの誤り、認証情報の誤り、収集の失敗、割り込みで終わる経路では、保存先のフォルダに一時ファイルが残りうる | どの終了経路でも一時ファイルを消す(割り込みも含む)と BR4.1 に書き、場面に加える |
| low | `functional-spec.md` > 出力先の表(JSON の行)/ 契約 C3 | 表は JSON に `accountId`、`createdAt` などの項目があると書くが、C3 の必須項目にも契約への変更の表にも `createdAt`、`regions`、`planId` はない。一覧と Markdown にはある作成日時が JSON では何で表されるか分からない | JSON に入れる項目を決める。入れるなら契約への変更の表に足し、入れないなら表の記述を `accountId` だけに直す |
| low | `functional-spec.md` > Assumptions / `rules.md` > BR2.3、BR3.4 | 消す候補のリソース数は一覧(`Name(812)`)にしかなく、JSON の `removeCandidates` はキー名だけになる。4 つの形式で数を一致させる方針(BR3.1)と、この項目だけ異なる。仮定として明記済みなので設計としては成立する | Markdown には件数を出すか、JSON でも差を許すかを BR3.1 の例外として一言書く |
| low | `functional-spec.md` > 表示の例(値の別名)/ 契約 C4 の中間の状態 | 変更を「この順に」書くと、`Environment` は `Prod` を付けたあとで `prod` に付け直すことになり、書き込みの途中の値は `plannedAfterTags` とも `beforeTags` とも異なる。rollback の判定表(後から変えられた)に当たりうる。U6 が最終の値だけを書けば避けられる | U6 は変更後のタグ(BR1.9)の最終の値を直接書いてよい、と注記する |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `aidlc engine sensor-traceability`(U4 functional-design の traceability.json) | pass: true、gaps、orphans、invalid なし | 構造は問題ない。AC6.1.1 の Deferred は story-map の追記で受け皿ができた |
| コードとの突き合わせ(`internal/planner/planner.go`、`internal/cli/commands.go`) | 差異は設計に記載済み | U1 の mergesIntoExisting(正しいキーあり: 全件 true、なし: 最初の別名だけ false)と、`len(plan.Changes) > 0` で影響分析を呼ぶ条件は、BR1.1、BR2.7 の記述と一致 |

### Summary

前回の high 2 件(標準出力と標準エラーの行き先、mergesIntoExisting の書き込みと変更後のタグ)は、出力先の表と値の別名の 4 パターンの表で解消され、medium 5 件、low 4 件も仕様に反映されています。ブロックする問題は残っていません。新しい指摘は、契約の変更が U6・U8 の入力に届く場所の記録(medium 1)と、一時ファイルの後始末、JSON の項目、消す候補の件数、途中の書き込みの注記(low 4)です。開発者が実装時に推測を要する箇所は限られているため、READY とします。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
