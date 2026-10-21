## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-product-lead-agent
**Date:** 2026-10-20T03:08:01Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/refined-mockups/mockups.md > 画面4(plan)、画面7(apply の確認画面) | 「影響分析は不完全」(FR4.7、AC3.2.5)の見せ方が、plan にも apply の確認画面にも描かれていない。interaction-spec.md の plan の partial 状態に言葉があるだけである。ポリシーを読めないまま作った plan.json を apply すると、確認画面は「影響 0 件」としか見えず、読めなかった範囲があることが伝わらない。このツールの中心の価値は「壊れるものを事前に示す」ことなので、最も危険な見落としになる。 | plan の「影響分析が不完全」の画面を追加する。保存した plan.json に不完全の印を持たせ、apply の確認画面にも同じ警告を出すと決める。`--yes` や `--allow-impact` の扱い(止めるのか、通すのか)も、終了コードとあわせて書く。 | New |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/refined-mockups/mockups.md > §1 終了コード、画面2・画面4 の `Exit code:` 行、interaction-spec.md > 各サブコマンドの States | 終了コードの契約に抜けと曖昧さがある。(1) 変更があるが影響も表記ゆれもない plan が 0 か 2 か決まっていない(plan の success は「0 または 2」としか書かれていない)。(2) `apply` や `rollback` で `yes` 以外を入力して中止した場合が 0 になっており、スクリプトからは「適用した」と区別できない。(3) 画面の `Exit code: 2 (...)` と `Next:` の行を標準出力と標準エラーのどちらに出すのか書かれておらず、`--format json` や `--format markdown` の標準出力に混ざると AC6.1.5 と AC3.1.8 を破る。 | plan が変更ありで影響なしのときの終了コードを決めて §1 の表に載せる。中止時の終了コードを明記する(既存の 0/1/2 のどれかにするか、別の値にするか)。`Exit code:` と `Next:` と要約の行について、出力先と、機械可読な形式のときに出すかどうかを決める。画面の `Exit code:` が実際の出力か、説明用の注記かも書く。 | New |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/refined-mockups/mockups.md > 画面1〜9(欠けている状態) | 受け入れ条件に対応する出力の例がない状態が複数ある。認証情報なし(AC1.2.3、FR7.8)。読み取り権限が足りないときの API 名(AC1.2.2。画面2は region の AccessDenied だけ)。「問題なし」(AC1.1.8)。変更なし(plan の no-changes)。`aws:` タグを飛ばした表示(AC4.2.4)。タグ数の上限超過で書き換えない一覧(AC4.2.5)。`--force` つきの rollback の結果(AC5.1.4)。rollback が失敗を含む結果(AC5.1.5)。途中で止まった apply への rollback(AC5.1.6)。記録が見つからない rollback。apply が同時に動いたとき(NFR7)。ヘルプ(AC6.1.1)。状態表に名前はあるが、文面と終了コードが決まっていないため、実装者が推測することになる。 | 上の各状態について、出力例を1つずつ追加する(短いもので足りる)。特にタグ数の上限超過と `aws:` の扱いは、画面8で「飛ばした」と「失敗した」のどちらに数えるか、終了コードがどうなるかを決める。 | New |
| R-04 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/refined-mockups/mockups.md > 画面2、画面3、画面4、画面7 の数値 | 例の数値が、ルールファイルと合わない。ルールで `CostCenter` に `aliases: [cost-center, cost_center]` を指定しているのに、画面4の変更に cost-center のキー名変更が出ず、画面7の `rename key 4,102` も Environment の分だけである。`Owner` には `default` があるのに、画面2で `Owner` が 1,244 件欠けているところ、画面4の追加は 1,189 件で、差の 55 件が `CostCenter` の「値が未指定」に付け替わっている。`CostCenter: {}` なら欠けている全件が未指定になるはずなのに、画面2の 903 件とも合わない。画面3は「値の対応 1 つ」と書くが、YAML には 2 つある。画面4の「消す候補」には、除外すると書いた `aws:cloudformation:stack-name` が並び、画面1・4・8の「...」は Q3(既定で全件表示)と食い違う。 | 数値と一覧が、ルールファイルの例と矛盾しないように直す。scan の「必須タグが欠けている」は別名の解決後に数えるのか前に数えるのかを一言書く。「...」は省略の記号だと注記するか、全件表示と失敗の詳細だけをログに回すのかを決めて書く。 | New |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/refined-mockups/mockups.md > 画面6(`--format json`、`--format markdown`) | JSON と Markdown の出力は、コマンドの行だけで中身の例がない。主な利用者の1人(P2)はスクリプトから使い、FR3.5 と AC3.1.5 は「4つの形式で件数が一致する」ことを確かめるため、少なくとも件数と影響の項目名は決まっている必要がある。 | `scan` と `plan` の JSON と Markdown について、主要な項目(件数、変更の種類、影響、不完全の印、終了コードに対応する値)の短い例を載せる。細かい仕様は機能設計に回すと明記してよい。 | New |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/refined-mockups/interaction-spec.md > rules suggest、plan、apply、rollback の Inputs | 共通のオプションが整理されていない。`--profile`(FR7.6)と `--lang`(FR7.2)は scan にしかなく、`rules suggest` には Inputs の表自体がない。`apply`/`rollback` で標準入力が端末でないとき(確認を読めないとき)の振る舞いも決まっておらず、Q6 の進み具合と同じ端末でない環境の角がここだけ残っている。 | 全サブコマンドで使う共通オプションの表を1つ作る。端末でないときに `yes` を読めない場合は、止めて終了コードを返すのか、`--yes` を必須にするのかを決めて書く。 | New |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/refined-mockups/accessibility-checklist.md > 項目3、項目4、項目10 | 端末での進み具合の「書き換え表示」は、スクリーンリーダーでは同じ行が繰り返し読まれやすい。しかしチェックリストは項目3を満たすとしており、書き換えを止める手段(例: 進み具合を1行追記にする指定)を示していない。また、記号 `!` が「影響あり」と「結果が不完全」の両方に使われ、機械的にも読み上げでも区別しにくい。 | 端末でも追記表示に切り替えられる指定を用意するか、その旨を前提に書く。`!` が2つの意味を持つことを許すのであれば、必ず続く文字(`Results are incomplete` など)で区別すると書く。 | New |

### Summary

コマンドの流れ、記号と色の決め、終了コードの基本方針は、Q&A の回答どおりで、要件とストーリーへの対応もよく取れている。ただし、安全性に直結する「影響分析が不完全」の見せ方と終了コードの契約に抜けがあり、受け入れ条件に対応する状態の出力例が複数欠けているため、これらを決めてから進めるのがよい。
