# ユーザーストーリー — 計画と質問

## ストーリーの計画(案)

- **形式:** 「〈ペルソナ〉として、〈したいこと〉をしたい。それは〈得られる価値〉のためだ。」の形で書き、INVEST(独立している、話し合える、価値がある、見積もれる、小さい、テストできる)を満たすようにします。
- **受け入れ条件:** Given/When/Then の形で書きます。チームの進め方で、受け入れ条件を先に自動テストにすると決めているためです。
- **優先度:** 要件定義の段階(第1弾・第2弾が Must、第3弾が Should / Could)に合わせて、各ストーリーに MoSCoW の優先度を付けます。どこまでを最初に作るかの最終判断は、後の「作業計画」のステージで行います。
- **ID:** ストーリーは `US{グループ}.{番号}`、受け入れ条件は `AC{グループ}.{番号}.{番号}` で番号を付けます。

以下の4点を決めてください。

## Q1. ペルソナ(利用者像)の分け方

ストーリーの主語になる利用者像を、何人に分けますか?

A. 2人: 社内の担当者(プラットフォーム、FinOps、セキュリティ)と、OSS の利用者
B. 3人: A の社内の担当者を「タグを整理する担当者」と「セキュリティ・監査の担当者」に分ける
C. 1人: 区別しない
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q2. ストーリーのまとめ方

ストーリーを、どの単位でグループに分けますか?

A. 作業の流れ(scan → rules → plan → apply → rollback)ごと
B. ペルソナごと
C. 要件定義の機能のまとまり(FR1〜FR8)ごと
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q3. ストーリーの大きさ

1つのストーリーの大きさは、どの程度にしますか?

A. 小さめ: 1つが1〜3日で作れる程度。数は30前後になる
B. 大きめ: 機能ごとに1つ程度。数は10前後になる
C. Not yet defined
X. Other (please specify)

[Answer]: B

## Q4. 第3弾(IaC への書き戻し)のストーリー

第3弾(IaC への書き戻しとプルリクエストの作成)は、詳しい振る舞いがまだ決まっていません(要件定義の未決事項 OQ3)。ストーリーをどこまで書きますか?

A. 大まかなストーリーだけ書き、受け入れ条件は第3弾に取りかかる前に詳しくする
B. 今の時点で、受け入れ条件まで詳しく書く
C. 今回は書かない
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- ペルソナ(Q1): 2人。社内の担当者(プラットフォーム、FinOps、セキュリティ)と、OSS の利用者
- まとめ方(Q2): 作業の流れ(scan → rules → plan → apply → rollback)ごとにグループに分ける
- 大きさ(Q3): 大きめ。機能ごとに1つ程度で、10前後
- 第3弾(Q4): 大まかなストーリーだけ書き、受け入れ条件は第3弾に取りかかる前に詳しくする

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
