# 担当の割り当て: iac への CDK の反映とプルリクエストの作成

この文書は、Bolt を誰が作るかを決めます。Bolt は、1つ以上の作業の単位を設計からコードまで通し、動くものができたところで終わる1回の作業です。順番と中身は `bolt-plan.md` にあります。

この作業の範囲(mvp)では、チームの編成の段階(Team Formation)を行っていません。そのため、複数のチームに割り当てる表(Program Board、チームと Bolt の対応の一覧)は作らず、すべての Bolt を1つの担当で作ります。

| Bolt | 単位 | 作る担当 | 承認する人 |
|---|---|---|---|
| B1 | U1 u1-walking-skeleton | aidlc-developer-agent(AI) | 開発者(利用者本人) |
| B2 | U2 u2-pull-request | aidlc-developer-agent(AI) | 開発者(利用者本人) |
| B3 | U3 u3-cdk-synth | aidlc-developer-agent(AI) | 開発者(利用者本人) |
| B4 | U4 u4-cdk-typescript | aidlc-developer-agent(AI) | 開発者(利用者本人) |
| B5 | U5 u5-cdk-python | aidlc-developer-agent(AI) | 開発者(利用者本人) |
| B6 | U6 u6-cdk-manual | aidlc-developer-agent(AI) | 開発者(利用者本人) |

- 開発は1人なので、プルリクエストのレビューの承認は必須にしません(チームの進め方)。人が見るのは、AI-DLC の各段階の承認と、squash マージのときです。
- 人にしかできないことは、次のとおりです。
  - テスト専用のリポジトリとトークンの用意
  - AWS の変更の許可
  - SSO のログイン
  - push とプルリクエストの作成の依頼
  - squash マージ
  - `v0.2.0` のタグ付け
  詳しくは `external-dependency-map.md` にあります。
- Construction は、このセッションから1つずつ順に進めます(unit-major、serial)。
