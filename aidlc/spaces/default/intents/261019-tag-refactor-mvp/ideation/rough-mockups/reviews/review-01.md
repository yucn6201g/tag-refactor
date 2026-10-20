## Review

**Verdict:** READY
**Reviewer:** aidlc-product-lead-agent
**Date:** 2026-10-19T11:43:25Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/ideation/rough-mockups/wireframes.md > 画面3 ルールファイルの例、画面4 plan | スコープ上の apply の変更は「表記ゆれの統一」「必須タグを付ける(値は利用者が指定)」「不要なタグを消す」の3種類(IB-07、S-Q5)。しかしルールファイルの例には、必須タグの値を指定する書き方も、消すタグを指定する書き方もない(`required: [Owner, CostCenter]` は名前だけ)。画面4の plan にも、必須タグの追加(`+`)の行がなく、`- remove key tmp-owner` はルールのどこから来たのか分からない。画面2で最も件数が多い問題(Owner 欠落 1,244件)を直す流れが見えない。 | ルールファイルの例に、必須タグの値の指定(全リソース共通の値か、リソースごとの指定か)と削除対象の指定を加える。plan の例にも必須タグの追加行を入れる。決められない場合は未決事項として明記し、要件定義に引き継ぐ。 | New |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/ideation/rough-mockups/wireframes.md > 画面4 Impact 欄 | 影響の欄は「変更するキーを使う参照元」だけを示している。一方で plan には値の変更(`Environment: Prod -> prod`、640件)がある。IAM やバケットポリシーの条件は値(`aws:ResourceTag/Environment` が `Prod`)にも依存するため、値の変更でも壊れる。このツールの中心的な価値(壊れるものを事前に示す)に関わるのに、値の変更に対する影響の見せ方がない。 | Impact 欄に、値の変更で影響を受ける参照元の例を1件示す。値の変更を影響分析の対象に含めるかどうかを、仮定または未決事項として書く。 | New |
| R-03 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/ideation/rough-mockups/wireframes.md > 画面4 | 数値が一致しない。変更行の合計は 2,950+811+640+52=4,453 で、見出しの「4,102 resources」と合わない(リソースが重複する可能性はあるが説明がない)。また画面3のルールに `environment` が別名として入っているのに、plan に `environment -> Environment`(341件)の行がない。 | 数値を整合させるか、「1つのリソースに複数の変更が重なる場合がある」と注記する。`environment` の行を追加する。 | New |
| R-04 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/ideation/rough-mockups/wireframes.md > 共通の決まりごと(アクセシビリティの共通メモ)、画面ごとの注記 | 共通メモの「罫線を最小限にして平文にする」「すべての操作をキーボードのコマンド入力だけで行える」は M-Q6 の回答(色の併用、`NO_COLOR`、進み具合)に含まれないのに、出典が M-Q6 になっている。ステージ定義が求める画面ごとのアクセシビリティの1行は、画面1・2・4にしかなく、画面3、5〜10 にはない。 | 該当の2点を `[assumption]` として書くか、出典を直す。画面ごとの1行の注記を補うか、共通メモで全画面に適用すると明記する。 | New |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/ideation/rough-mockups/wireframes.md > 画面3、画面6、画面7 | M-Q1 は手元での対話的な実行のみだが、M-Q4 は CI 向けの確認を省くオプションを求めている。CI で使う場合の終了コード、`--yes` の動き、標準出力と標準エラーの使い分けが示されていない。たとえば画面3は `> tagrules.yaml` に出力を流す例なのに、「Drafted rules for 3 keys...」というメッセージが同じ標準出力に出ると、ファイルに混ざる。 | 進み具合や案内のメッセージは標準エラーに出す方針を共通の決まりごとに1行で書く。CI 利用を今回どこまで扱うか(終了コードなど)を未決事項に書く。 | New |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/ideation/rough-mockups/wireframes.md > 状態ごとの表示 | 意図の文書は OSS 利用者の期待として「書き込み権限なしで安全に試せる」を挙げているが、scan と plan を読み取り専用の権限で動かす場合の表示(権限が足りない API の一覧など)が示されていない。「一部失敗」の行で部分的にしか触れていない。 | 読み取り専用の権限で scan と plan が動くこと、足りない権限があるときの案内を、状態ごとの表示に1行加える。 | New |

### Summary

主な流れ(scan、rules suggest、plan、apply、rollback)と、空・エラー・一部失敗の状態は、スコープの Must have と対応しており、開発者が形をつかめる水準です。残る大きな点は2つで、必須タグの追加と削除の指定方法が示されていないこと、値の変更に対する影響分析の見せ方がないことです。どちらも要件定義で決められるため、承認前に未決事項として明記するか、モックアップに1例を足すことをおすすめします。
