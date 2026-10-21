# 担当の割り当て: tag-refactor

Bolt(作業の一部を設計から実装まで一通り作り、動くものができるところまで進める1回分の作業)ごとに、誰が作るかを決めたものです。

## 体制

このワークフローの範囲(`mvp`)では、チームを組むステージ(Team Formation)を行っていません。そのため、すべての Bolt を AI の開発担当(`aidlc-developer-agent`)が、このセッションで作ります(`delivery-planning-questions.md` Q8)。人(リポジトリの持ち主)は、各ステージの承認、Bolt ごとのチェックポイントの承認、プルリクエストのマージ、`vX.Y.Z` のタグによる公開を受け持ちます。

## 割り当て

| Bolt | 作業単位 | 作る担当 | 人が受け持つこと |
|---|---|---|---|
| B1 | U1 最小の一本 | aidlc-developer-agent | 検証コマンドの承認、最小の一本のチェックポイントの承認、検証用アカウントと認証情報の用意 |
| B2 | U2 収集 | aidlc-developer-agent | チェックポイントの承認、AWS Config か Resource Explorer の有効化 |
| B3 | U3 ルール | aidlc-developer-agent | チェックポイントの承認 |
| B4 | U4 変更案 | aidlc-developer-agent | チェックポイントの承認 |
| B5 | U5 影響分析 | aidlc-developer-agent | チェックポイントの承認、確認用ポリシーの作成の承認、`v0.1.0` の公開(配布の仕組みの別の作業の後) |
| B6 | U6 書き換えと記録 | aidlc-developer-agent | チェックポイントの承認 |
| B7 | U7 取り消し | aidlc-developer-agent | チェックポイントの承認 |
| B8 | U8 IaC への反映 | aidlc-developer-agent | 第3弾の前の設計(要件 OQ3)の判断、チェックポイントの承認 |

## 進め方

- Bolt は1つずつ順番に進めます。同時に進める Bolt はないため、チームの間の調整(Program Board のような、複数チームの予定をそろえる表)は使いません。
- 各 Bolt では、設計のステージ(Functional Design など)からコードの生成までを、AI-DLC のエンジンの指示どおりに進めます。設計のステージには、それぞれのステージの担当の AI が加わります。
- 最小の一本(B1)の承認の後、残りの Construction を自動で続けるか、チェックポイントごとに確認するかを選びます(チームの進め方 §Walking Skeleton)。
