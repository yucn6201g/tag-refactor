## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Iteration:** 1

この審査は助言(advisory)です。指摘は承認の場で人が判断するための材料で、修正後の再審査は前提にしていません。

確認できた点: Q1 で選んだ4つの外向き形式(C1 ルールファイル、C2 変更案、C3 出力 JSON、C4 記録)には、すべて `version` を持つ仕様ブロックがあります。持ち主は Q4 のとおりです(ルール = U3、変更案と出力 = U4、記録 = U6)。再試行を AwsAccess だけに置く点(C6、契約の決まり)は Q5 と合っています。終了コードの表(C5)は、モックアップと FR7.4 の主な場合(キャンセル 0、アカウント不一致 1、`--yes` だけで影響あり 2、失敗あり 1、飛ばしのみ 2)と一致しています。一方で、下の指摘のとおり、rollback と apply が実装できない箇所と、契約どうしの食い違いがあります。

### Prior findings

| ID | Now | Severity | Note |
|---|---|---|---|
| なし | - | - | 初回(Iteration 1)のため、前回の指摘はありません |

### New findings

| # | Severity (high/medium/low) | Location | Finding | Suggested fix |
|---|---|---|---|---|
| 1 | high | `inception/contract-design/contract-summary.md` > C4 `entries[].status: pending`、C10 `RecordBefore` | `pending` の項目には書き換え前の `beforeTags` しかなく、「書き換えるつもりだった結果」がありません。途中で止まった apply(FR6.5)で、rollback は AWS の今のタグが `beforeTags` と違っていても、「apply が書き込んだが記録が間に合わなかった」のか「他の人が変えた」のか区別できません。区別できなければ、戻し損ねるか、他人の変更を上書きします。キー名の変更は「タグを付ける」と「タグを消す」の2回の呼び出し(C6 `TagResources`/`UntagResources`)で、間で止まると、書き換え前とも後とも違う中間の状態になります。 | `RecordBefore` の項目に、書き換え後に期待するタグ(例: `plannedAfterTags`)を持たせ、C4 にも追加する。rollback の判定を「今のタグ = `beforeTags` なら何もしない / = `plannedAfterTags` または `afterTags` なら戻す / どれとも違えば後から変えられたとみなす」と明記する。中間の状態(2回の呼び出しの間)の扱いも決める。 |
| 2 | high | C2 `resources[].changes`、C2 `impactFindings[].causedBy`、C9 `TagChangeInput.changeId` と `ImpactResult.findings.changeIds` | C9 は変更を `changeId` で指すのに、C2 の `changes` には `changeId` がありません(`components.md` の Change エンティティには `changeId` があります)。C2 の `causedBy` は「キーと種類」の文字列としか書かれていません。同じキー・同じ種類の変更が複数のリソースにあると、どのリソースのどの変更が影響を受けるのかを保存した変更案から特定できません。また `impactFindings` はポリシー単位で、リソース(ARN)との対応がありません。そのため、apply の確認画面で「影響のある変更」を示す(FR5.4)ことと、`--yes` だけのときに止まる(FR5.5)判定を、U6 が保存した変更案だけからは実装できません。 | C2 の `changes` の各項目に `changeId`(変更案の中で一意)を必須で追加する。`causedBy` を `changeId` の配列と定義して C9 の `changeIds` と揃える。`impactFindings` から、影響を受ける変更・リソースへたどれることを仕様に書く。 |
| 3 | high | C2 `beforeTags`、C4 `beforeTags`/`afterTags`、契約の決まり(`aws:` の扱いの記述なし) | `beforeTags` と `afterTags` は「タグ全体」と書かれ、`aws:` で始まるタグも含みます。しかし、`aws:` で始まるタグは (a) AWS 側で変わることがあり、(b) タグを付け直す API では付けられず、(c) タグ数の上限(FR5.10)に含まれません。そのため、apply の変化の確認(FR5.6)と rollback の「後から変えられたか」の判定(FR6.3)で、全体を比べると、`aws:` のタグの変化だけで誤って飛ばします。rollback が `beforeTags` をそのまま書き戻そうとすると `aws:` のキーで失敗します。上限の判定も、`aws:` を数えると誤って「上限超え」と判断します。 | 「比較・書き戻し・上限の数え方では `aws:` で始まるキーを除く」と契約に明記する。保存するのは全体のままでも、除外の規則を C2・C4 の説明に書く。C1 の `aws:` 無視の規則は、`aliases` や `values` の中に `aws:` のキーがある場合にも及ぶかを決めて書く(別名から `aws:` のキーへの変更は rename ではなく削除になりうる)。 |
| 4 | medium | C1〜C4 の `version`、契約の決まり > バージョン(Q2) | Q2 は、読めない `version` のファイルを「どのバージョンのツールで作ったか」が分かる形で止めると決めています。しかし、どの仕様ブロックにも、作ったツールのバージョンを持つ項目(例: `toolVersion`)がありません。`version` は形式の版で、ツールの版ではないため、止めるときの案内に必要な情報がファイルにありません。 | C2・C3・C4(と、`rules suggest` が出す C1 の下書きの先頭のコメントか項目)に `toolVersion`(または `generatedBy`)を追加する。読み取り側のエラーに、ファイルの `version` と `toolVersion`、このツールが読める `version` を出すことを書く。 |
| 5 | medium | 契約の一覧 C6、C7、C9 の Owner と `inception/units-generation/unit-of-work-dependency.md` の依存 | 持ち主と作業単位の依存が食い違っています。C9 は提供側 U5、使う側 U4(Planner が呼ぶ)で、U4 が U5 のインターフェースに依存します。しかし依存関係では U5 が U4 に依存する向きで、循環になります(U1 の最小版が両方を持つので実装上は進みますが、契約の持ち主の説明としては矛盾します)。また C6 と C7 の持ち主は U2 ですが、U3・U4 は U2 に依存せず、U2 と並行して進める前提です(同じ文書の「並行して進められる作業単位」)。契約が確定する前に使う側が進むことになります。 | インターフェースの持ち主は、U1 の最小版が最初に定義する作業単位(C6、C7)と、使う側(C9 は U4 または U1)に置く、または「U1 で定義、U2/U5 で拡張」と書く。依存関係の図と矛盾しないことを確かめる。 |
| 6 | medium | C5 `exitCodes`、C3 `summary.undecidedReferences`、C2 `verdict: undecided` | `affected`(`!`)と `undecided`(`?`)は C2・C3 で分かれていますが、「影響あり」に `undecided` が含まれるかを契約が決めていません。plan の終了コード 2 の条件、apply の確認画面、`--yes` だけで止まる条件(FR5.5)、`--allow-impact` で進められる範囲のそれぞれに関わります。FR4.5 は「影響なしとは表示しない」とだけ言っています。 | 「`undecided` も `affected` と同じく終了コード 2 と `--yes` での停止の対象にする」など、決めて C5 に書く。`--allow-impact` を `--yes` なしで付けたときの扱いも明記する(モックアップは「`--yes` のときに」とだけ書いています)。 |
| 7 | medium | 契約の決まり > 追加の変更、C1 | 「項目を足すだけなら `version` を上げず、読む側は知らない項目を無視する」という規則を、すべての形式に一律に当てはめています。ルールファイル(C1)は利用者が手で書くので、`requried:` のような綴りの誤りが黙って無視され、FR2.5(誤りの場所と直し方を示す)に反します。また C2 の `changes[].kind` や C4 の `status` に値を足すことも「追加」にあたりますが、古い読み手が知らない値を無視すると、apply が変更を黙って飛ばします。 | C1 では、知らないキーを警告(または誤り)にする。項目の追加は無視してよいが、列挙型(`kind`、`status`、`verdict`)に値を足す変更は `version` を上げる対象にする、と区別して書く。 |
| 8 | medium | C3(出力の JSON)`details`、`summary`、`complete` | `details` が `description` の文だけで、項目の形が決まっていません。scan の「表記ゆれのグループ」と「必須タグの欠落」の項目名が、利用者のスクリプトに対する契約として未定義です。plan の `details` は「C2 の resources と impactFindings と同じ形」とされ、C2 の `unsetRequiredTags` と `removeCandidates` が含まれるか不明です(FR3.5: 4つの形式で件数が一致)。要件 AS3 の「対応できなかった種類と件数」(`InventorySnapshot.unsupportedResourceTypes`)を運ぶ項目が、C2 にも C3 にもありません。FR1.3 の「探せなかった理由」を入れる項目も、`untaggedDiscovery: unavailable` にはありません。 | scan の `details` を `variantGroups[]` と `missingRequired[]` として項目まで決める。plan の `details` に `unsetRequiredTags` と `removeCandidates` を含めると明記する。`unsupportedResourceTypes`(種類と件数)と `untaggedDiscoveryReason` を C3(必要なら C2)に追加する。 |
| 9 | medium | C12 エラーの種類、C6 `AccountResolver.Resolve` | エラーの種類(Q5)に抜けがあります。(a) `Resolve` の戻りが `InvalidInput, AccessDenied` だけで、FR7.8 の `CredentialsMissing` が入っていません。(b) 読めない `version` のファイル(Q2)を止めるときの種類がなく、`InvalidInput` に入るのか不明です。(c) 変更案や記録の保存先ファイルの書き込み失敗(apply の `record-failed`、`plan --out`)と、S3 以外のローカル I/O の失敗に、種類がありません。`Transient` は「AwsAccess の再試行を使い切った後」と定義されているため、ローカルの失敗には使えません。 | `Resolve` の戻りに `CredentialsMissing` と `Transient` を加える。`UnsupportedVersion`(または `InvalidInput` の細目)と、ローカル/S3 の書き込み失敗を表す種類(例: `StorageFailed`)を C12 に加え、C5 の終了コードと対応させる。 |
| 10 | medium | C10 Journal | NFR7 は apply と rollback の同時実行から記録を守ると決めており、C12 の `Conflict` も rollback を含みます。しかし C10 の `Begin` は apply のためのもので、rollback が使えるロックの取得・解放がありません(Rollbacker は `Load` だけを呼ぶ)。また rollback 自体の結果(戻した・飛ばした・失敗した)を記録する口がなく、同じ実行IDを2回 rollback した場合の挙動(2回目は afterTags と違うので「後から変えられた」と判定されて飛ばされる)も決まっていません。 | `BeginRollback(runId)` と `Release` のようなロックの口、または apply と共通のロックの取得を C10 に加える。rollback の結果を記録に残すか、残さないなら二重実行の挙動を契約に書く。 |
| 11 | low | C2 `changes[].kind` 以降の項目 | `kind` だけが必須で、種類ごとにどの項目が必須かが決まっていません(例: `changeValue` のキーは `fromKey` か `toKey` か、`addRequired` は `toKey` と `toValue` か、`removeKey` は `fromKey` か)。U4 が書く側、U6・U8 が読む側なので、解釈がずれやすいところです。 | 種類ごとの必須項目を表にして C2 に書く(JSON Schema の `oneOf` でもよい)。 |
| 12 | low | C7 `Resource.region`、C8 `RuleSet`、C11 `ProgressSink` | グローバルなリソースの `region` に何を入れるか(`components.md` の未決事項)が契約に引き継がれていません。C2 は `region` を必須としており、`TagWriter.TagResources(region, ...)` も `region` を取るので、書き込み先の `region` が決まりません。また C1 は `aws:` のキーを「警告する」としていますが、C8 の `Load` の戻り(`RuleSet`)にも C11 の `ProgressSink` にも警告を運ぶ口がありません。 | グローバルなリソースの `region` の値(例: タグ用 API を呼ぶ固定のリージョン)を決めて C7 に書く。`RuleSet` に `warnings`、または `ProgressSink` に `Warn(message)` を足す。 |
| 13 | low | 契約の一覧 > 未決事項 | 未決事項の表が不足しています。(a) C5 の「オプション名の確定」の Blocks が U1 だけですが、`apply` と `rollback` のオプションは U6・U7 が持つので、Blocks に U6、U7 が必要です。(b) 記録の既定の保存先(`.tag-refactor/runs/`(仮))と、同時実行を防ぐ印の置き場所が、`interaction-spec.md` では仮なのに、契約の未決事項にありません(Blocks は U6、U7)。(c) 要件 OQ5(一度もタグが付いていないリソースを探すときの件数の上限と扱い)が、C7 `FindUntaggedResources` の戻り(打ち切りの印がない)に反映されていません(Blocks は U2)。(d) `runId` の例 `20261019-2110` は分単位で、同じ分に2回実行すると衝突する可能性があります(Blocks は U6)。 | 上の4点を未決事項の表に加え、それぞれの Blocks を U1/U6/U7/U2 から選んで明記する。`runId` は一意になる形(秒以下、または乱数の接尾辞)に変える。 |

### Summary

rollback が途中で止まった apply を正しく扱うための情報(指摘1)、変更案の中で影響のある変更を特定する識別子(指摘2)、`aws:` のタグを比較・上限・書き戻しから除く規則(指摘3)が契約に欠けており、第2弾の apply と rollback は現状の契約のままでは実装できません。そのため NOT-READY と判断しました。形式・持ち主・版・エラー/再試行の方針は Q1〜Q5 とおおむね整合しており、指摘1〜3 を直し、指摘4〜10 を反映すれば、第1弾の契約はそのまま進められます。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
