## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T06:07:56Z
**Iteration:** 1

### Prior findings

| Severity | Location | Finding | Status |
|---|---|---|---|
| なし | なし | なし | なし |

### New findings

| Severity | Location | Finding | Required action |
|---|---|---|---|
| high | `functional-spec.md` > 流れ W1 の 11、「表示の例」(一覧の形、Markdown の形)/ `rules.md` > BR3.10、BR3.2 | 標準出力に出す内容が自己矛盾している。BR3.10 は「結果だけを標準出力、案内・進み具合・警告は標準エラー」とし、U1 BR6.1 も「アカウントの行と Exit code の行は標準エラー」と決めている。ところが W1 の 11 は、要約・終了コードの意味・次の手順(「変更案を保存する: …」)を「一覧と Markdown では標準出力の最後」に出すと書き、日本語の一覧の例も「アカウント … 変更案の作成」の行と「終了コード: 2」「変更案を保存する」の行を標準出力に置いている。このとおり作ると、モックアップ画面6の `plan --format markdown > plan.md` のファイルに終了コードと次の手順の案内が混ざり、AC3.1.8 と AC6.1.5(案内が混ざらない)に反する。 | 標準出力と標準エラーの行き先を行ごとに表にして決める。「終了コード」「次の手順」「Saved plan」は標準エラー、「アカウント・リージョン・作成日時」の行は一覧では U1 のとおりか結果の一部か(Markdown では `## 対象` の節)を明記する。W1 の 11 と各例を BR3.10 に合わせて直す。`> plan.md` で案内が混ざらないことを業務の場面に加える。 |
| high | `rules.md` > BR1.9、BR1.1(mergesIntoExisting)/ 契約 C2 の kind 表(renameKey は「値はそのまま」)/ `entities.md` > Change.mergesIntoExisting | 値の別名を入れたことで、「正しいキー名が既に付いていて、別名の収集したままの値が違う書き方」になる場合(`env=Prod` と `Environment=prod`、または別名どうしで `Prod` と `prod`)が生まれた。BR1.9 はこのとき「toKey の値をそのまま残す」と定めるが、renameKey を「toKey に fromKey の値を付けてから fromKey を消す」と読む U6 の書き込みは、`Environment` を `Prod` に上書きする。BR1.3 は上書き前の値(`prod`)で changeValue を作らないので、書き換え後は `Environment=Prod` になり、plannedAfterTags(BR1.9)と食い違う。rollback の判定(`touchedKeys` を beforeTags か plannedAfterTags と比べる)も「後から変えられた」と誤判定する。`mergesIntoExisting` が true のときに何を書くかは、U1 BR4.2 にも U4 にも書かれていない。 | `mergesIntoExisting` が true の renameKey は toKey を書かず fromKey を消すだけ、と BR1.1 と BR1.9 と契約 C2 の kind 表に明記する(または fromValue と toKey の現在の値が違う場合に別の表現を使う)。値の別名つきで 3 パターン(正しいキーあり、別名のみで複数、書き方違いの 2 つ)の期待する変更前後のタグを業務の場面に加え、U6 が使う変更後のタグの求め方と一致させる。 |
| medium | `rules.md` > BR3.4 / `functional-spec.md` > 契約への変更、`entities.md` > PlanSummary / 契約 C2・C3 | JSON の `details` に入れるものとして BR3.4 が挙げるのは resources、impactFindings、unreadPolicies、unsetRequiredTags、removeCandidates で、`keyConflicts` が入っていない。PlanSummary の conflictResources と unsetRequiredResources も、C3 の summary の項目にない。衝突だけで終了コードが 2 になる計画でも、JSON を読むスクリプトからは理由が分からず、一覧・差分・Markdown と件数が一致しない(AC3.1.5 の趣旨)。また C2 に足す `keyConflicts` の項目名、必須かどうか、keyOrder を保存するかが、どの文書にも具体的に書かれていない。 | `keyConflicts` を JSON の details に含めると BR3.4 に書く。C2 と C3 に足す項目(keyConflicts の各項目名と必須の扱い、summary の conflictResources と unsetRequiredResources)を「契約への変更」の表に具体的に書く。failedRegions を plan の JSON に含めるかも決める。 |
| medium | `traceability.json` > AC6.1.1(Deferred)/ `unit-of-work-story-map.md` > US6.1 | AC6.1.1(5 つのサブコマンドのヘルプ)を「U6・U7 で確かめる」と先送りしているが、US6.1 のマップ先は U4 だけで、U6・U7 には US6.1 が割り当てられていない。このままでは先送り先に受け入れ条件の持ち主がなく、確かめられないまま残る。 | U6・U7 の機能設計に引き継ぐ項目として、どの文書(例: unit-of-work-story-map の備考)に記録するかを決めて書く。または U4 で `apply` と `rollback` のヘルプの内容(オプション名と説明の方針)を決めて、U6・U7 は実装だけにする。 |
| medium | `rules.md` > BR1.4、`functional-spec.md` > 業務の場面 | addRequired の値(ルールの default または when の値)に、値の別名を当てはめていない。`required: Environment: default: Prod` と `values: prod: [Prod]` を両方書くと、plan が `Environment=Prod` を付け、次の plan が `Prod` → `prod` の changeValue を出す。U3 の整合性検査(consistency.go)は必須タグの値と values の食い違いを調べていない。plan → apply → plan が収束するという性質が保証されない。 | addRequired の toValue にも CanonicalValue を当てるか、U3 に「必須タグの値が値の別名の綴りになっている」ことを誤りにする検査を足すかを決め、BR1.4 に書く。再実行で「変更は不要」になる場面を加える。 |
| medium | `rules.md` > BR2.1 と `entities.md` > ChangeLine の食い違い | BR2.1 は一覧の行を (種類, fromKey, toKey, fromValue, toValue) ごとにまとめると書く。しかし renameKey と removeKey の fromValue は「参考」で値ごとに違い、addRequired は値が複数でも 1 行(「(from rules)」)にする。BR2.1 のとおりに作ると、`env=Prod` と `env=dev` で「env -> Environment」が 2 行になり、画面4のモックアップと合わない。ChangeLine は fromValue を changeValue だけとしている。 | BR2.1 のまとめる単位を種類ごとに書く(renameKey は fromKey と toKey、changeValue は fromKey と fromValue と toValue、addRequired は toKey、removeKey は fromKey)。 |
| medium | `rules.md` > BR1.6、BR2.3 | 「aws: で始まる」を大文字・小文字を区別するように読める。しかし U2・U3 の実装(`IsAWSReservedKey`)は `AWS:` なども含めて区別せずに比べ、AWS も予約している。U4 の守り(変更を作る直前の確認)と消す候補の除外が大文字・小文字を区別すると、`AWS:foo` が消す候補に出たり removeKey が作られたりしうる。 | BR1.6、BR2.3 に「大文字・小文字を区別せず、U2・U3 と同じ判定を使う」と書く。 |
| low | `rules.md` > BR2.7、`functional-spec.md` > 流れ W1 の 7 | 影響分析を省く条件が「変更も衝突もない」だが、衝突だけで変更がない場合は、入力の変更が 0 件なのに全ポリシーを読みに行く。読めないポリシーがあると不完全(終了コード 1)になりうる。現在のコードは `len(Changes) > 0` で呼んでおり、仕様と食い違う。 | 影響分析を呼ぶ条件を「変更が 1 つ以上」にして、BR2.7 と W1 の 7 を直す(U1 BR4.4 の変更になるのでその旨も書く)。 |
| low | `rules.md` > BR4.4、`functional-spec.md` > 保存した変更案の読み込み | 機能仕様は「知らない列挙の値は誤りにする」と書くが、BR4.4 の確かめには kind・verdict・sourceKind の値の検査がない。また keyConflicts、unsetRequiredTags、同じ ARN の重なり、fromKey・toKey が `aws:` で始まらないことの検査がない。手で編集した変更案が U6 まで届く。mark は C2 に保存されないため、読み込んだあと `!` と `?` を impactFindings から求め直す規則も必要。 | BR4.4 に列挙の値の検査と上の検査を足し、読み込みで mark を求め直す規則を書く。 |
| low | `functional-spec.md` > 流れ W1 の 1、W2 | `--out` の確かめは「保存先にファイルがない」だけで、親フォルダがあるか・書けるかは、収集に時間がかかったあとの保存で初めて分かる。保存に失敗すると結果が出ずに終了コード 1 になり、時間がむだになる。 | 流れ W1 の 1 で親フォルダの存在と書き込みを確かめるか(一時ファイルを先に作るなど)、保存に失敗しても結果は標準出力に出す、のどちらかを決める。 |
| low | `traceability.json` > AC6.1.4(N/A) | `scan --format json` を U4 で足し、C3 の `exitCode` が実際の終了コードと同じであることが必要になる。AC6.1.4 を N/A にすると、U4 の新しい出力でこの一致を確かめる場所がなくなる。 | 状態を OK にして、BR3.4 の出力で exitCode が終了コードと一致することを確かめる、と書く。 |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `aidlc engine sensor-traceability`(U4 functional-design の traceability.json) | pass: true、gaps/orphans/invalid なし | 構造は問題ない。ただし AC6.1.1 の Deferred と AC6.1.4 の N/A は、構造の検査では妥当性を判定できないため、上の指摘に記した。 |

### Summary

変更の作り方(名前の変更、衝突、値の変更、必須タグ、削除、変更の順と changeId、変更後のタグ)は、全体として筋が通っています。ただし 2 点が実装で食い違います。標準出力に案内の行を出す書き方が BR3.10 と U1 の決まりに反していること、そして値の別名のために、別名を既存の正しいキーに統合するときの書き込みと変更後のタグが一致しなくなることです。どちらも U6 や AC3.1.8 の確かめに直接響くため、high としました。JSON の衝突の欠落、AC6.1.1 の先送り先、必須タグの値の収束などの medium の指摘も、承認の前に仕様へ反映してください。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
