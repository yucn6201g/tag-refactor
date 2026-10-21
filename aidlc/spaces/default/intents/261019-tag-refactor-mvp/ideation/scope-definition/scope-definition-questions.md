# 範囲の定義 — 質問

前のステージで、最初のバージョンに含める機能(scan、plan、apply、rollback、IaCへの書き戻し)と含めないものは決まっています。このステージでは、その中の優先順位と、範囲の境目を決めます。

## Q1. 最初のバージョンの出し方

実現可能性の評価では、範囲が1人で作るには大きいことが最大のリスクでした。最初のバージョンをどう出しますか?

A. 段階的に出す: 第1弾 scan と plan、第2弾 apply と rollback、第3弾 IaCへの書き戻し
B. 段階的に出す: 第1弾 scan、plan、apply、rollback、第2弾 IaCへの書き戻し
C. すべてそろってから最初のリリースにする
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q2. 必ず必要な機能(Must have)

「最初のバージョンが完成した」と言うために、必ず必要な機能はどれですか?(select all that apply)

A. scan と plan(影響分析つき)
B. apply と rollback
C. IaCへの書き戻し(Terraform)
D. IaCへの書き戻し(CloudFormation)
E. IaCへの書き戻し(CDK の TypeScript と Python)
X. Other (please specify)

[Answer]: A, B

## Q3. GitHub へのプルリクエスト作成の優先度

IaCの修正を GitHub のプルリクエストとして出す機能は、どの程度必要ですか?

A. 必ず必要(Must have)
B. 重要だが、なくても使える(Should have)。最初は修正後のファイルや差分を出力すれば足りる
C. あればよい(Could have)
D. 今回は作らない(Won't have)
X. Other (please specify)

[Answer]: C

## Q4. 作る順番の方針

機能を作る順番は、何を優先して決めますか?

A. 価値優先: 利用者がすぐ使えるものから作る
B. リスク優先: CDK の書き戻しや性能目標など、不確かなものを先に小さく試す
C. 依存関係優先: 他の機能の土台になるもの(タグの収集など)から作る
D. Not yet defined
X. Other (please specify)

[Answer]: C

## Q8. IaCへの書き戻しの優先度(Q2の補足)

Q2で、必ず必要な機能(Must have)は「scan と plan」と「apply と rollback」になり、IaCへの書き戻し(Terraform、CloudFormation、CDK)は選ばれませんでした。前のステージでは、IaCへの書き戻しも最初のバージョンに含めると決めています(第3弾、Q1)。IaCへの書き戻しの優先度はどれにしますか?

A. 重要(Should have): 第3弾で3種類すべてに対応する
B. 重要(Should have)なのは Terraform と CloudFormation で、CDK はあればよい(Could have)
C. 3種類ともあればよい(Could have)
D. Not yet defined
X. Other (please specify)

[Answer]: B

## Consolidated Summary Confirmation

回答のまとめ:

- 出し方(Q1): 3段階で出す。第1弾 scan と plan、第2弾 apply と rollback、第3弾 IaCへの書き戻し
- 必ず必要な機能(Q2): scan と plan(影響分析つき)、apply と rollback
- GitHub のプルリクエスト作成(Q3): あればよい(Could have)
- 作る順番(Q4): 依存関係優先(タグの収集など、土台になるものから作る)
- apply で行う変更(Q5): 表記ゆれをそろえる、必須タグを付ける、不要なタグを消す
- 一度もタグが付いたことのないリソース(Q6): 最初のバージョンで、別の方法を使って見つける
- 統一後の基準(Q7): ツールが既存のタグから候補を提案し、それをもとに利用者がルールファイルを作って編集する
- IaCへの書き戻しの優先度(Q8): Terraform と CloudFormation は重要(Should have)、CDK はあればよい(Could have)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct

## Q5. apply で行う変更の種類

apply でタグをどう書き換えられるようにしますか?(select all that apply)

A. キー名や値の表記ゆれをそろえる(例: `env` を `Environment` に、`Prod` を `prod` に)
B. 足りない必須タグを付ける(値は利用者が指定する)
C. 不要になったタグを消す
D. Not yet defined
X. Other (please specify)

[Answer]: A, B, C

## Q6. 一度もタグが付いたことのないリソースの扱い

実現可能性の評価で、一度もタグが付いたことのないリソースは、タグ用の一覧では見つからないことが分かりました。必須タグの欠落を調べるとき、こうしたリソースをどう扱いますか?

A. 最初のバージョンで、別の方法を使って見つける
B. 最初のバージョンでは、タグが一度でも付いたことのあるリソースだけを対象にし、この制限を明記する
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q7. 統一後の基準(正しいキー名や値)の決め方

表記ゆれをそろえるには、「どれが正しいキー名や値か」の基準が必要です。この基準をどう決めますか?

A. 利用者がルールファイルに書く(正しいキー名・値と、別の書き方の対応)
B. ツールが既存のタグから候補を提案し、利用者が確定する
C. A と B の両方(提案をもとにルールファイルを作り、利用者が編集する)
D. Not yet defined
X. Other (please specify)

[Answer]: C
