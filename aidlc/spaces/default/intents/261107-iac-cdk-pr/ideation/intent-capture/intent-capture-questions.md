# 意図の把握の質問: iac への CDK の反映とプルリクエストの作成

## Sources

- [desc] Initial description: "tag-refactor の第2弾として、iac コマンドに2つの機能を足す。(1) FR8.3 / AC7.2.1: CDK のソース(TypeScript と Python)への、タグの変更の反映。(2) FR8.4 / AC7.3.1: 反映した変更から GitHub のプルリクエストを作る。要件は前の intent aidlc/spaces/default/intents/261019-tag-refactor-mvp/ の requirements と U8 の機能設計(construction/u8-iac/)にあり、どちらも U8 の機能設計の Q1 で人が範囲の外にしたもの(優先度 Could)。既存の実装は internal/iac、cmd/、README の iac の節。"
- [scope] Workflow-selected scope: `mvp`.

## Q1. 解決したい問題

前の intent では、`iac` は Terraform と CloudFormation のソースだけを書き換えます。CDK で作ったリソースは、論理 ID が生成された名前のためテンプレートに見つからず「手で直す一覧」に出ます(README の「iac の限界」)。また、書き換えたあとのブランチの作成、コミット、push、プルリクエストの作成は利用者が手で行います。今回いちばん解決したい問題はどれですか?

A. CDK で作ったリソースのタグを `apply` で変えても、CDK のソースが古いままなので、次の `cdk deploy` で元に戻ってしまう
B. `iac --write` のあと、プルリクエストを出すまでの手作業(ブランチ、コミット、push、作成)が手間で、チームのレビューに乗せにくい
C. A と B の両方(どちらも同じくらい大事)
D. まだ決めていない
X. Other (please specify)

[Answer]: C

## Q2. 誰のための機能か

前の intent の利用者像 P1(佐藤さん。AWS のタグを整理するプラットフォームの担当)を引き継ぐかを決めます。

A. P1 と同じ。CDK で作ったリソースを持つチームのタグを整理し、IaC も直す人
B. P1 に加えて、プルリクエストを受け取って IaC の変更をレビューするチームの人
C. CDK を使っている OSS の利用者全般(P1 にこだわらない)
D. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q3. 成功の判断の基準

できあがったかどうかを、何で判断しますか?

A. CDK のテスト用のプロジェクト(TypeScript と Python)で、`iac --write` のあとに `cdk synth` で作ったテンプレートのタグが変更案の後の値と一致し、プルリクエストが手作業なしで作られること
B. A に加えて、文字列の値で書かれた CDK のタグは、すべて自動で反映されること(手で直す一覧に出るのは式で書かれたものだけ)
C. 受け入れ条件の AC7.2.1(CDK のソースが変更後のタグを表すように書き換えられる)と AC7.3.1(修正を含むプルリクエストが GitHub に作られる)を、自動テストで満たすこと。数の目標は置かない
D. まだ決めていない
X. Other (please specify)

[Answer]: C

## Q4. 今取りかかる理由

A. 第1弾(`v0.1.0`)を公開したので、U8 で後回しにした Could の2つに取りかかる順番が来た
B. CDK を使う利用者から要望があった
C. 自分で使っていて、CDK のリソースとプルリクエストの手作業に困った
D. 特になし
X. Other (please specify)

[Answer]: A

## Q5. 関係者

前の intent の関係者は、社内の利用者(プラットフォーム、FinOps、セキュリティの担当など)と、OSS の利用者・コントリビューターでした。今回、関係者は増えますか?

A. 増えない(前の intent と同じ)
B. 前の intent の関係者に加えて、プルリクエストを受け取る IaC のリポジトリの持ち主やレビュアー
C. 前の intent の関係者に加えて、CDK を使うチーム(TypeScript や Python のソースの書き方に慣習を持つ人)
D. 特定していない
X. Other (please specify)

[Answer]: A

## Q6. 範囲や優先度を決める人

前の intent では、依頼者本人が一人で決めていました。

A. 前と同じく、依頼者本人が一人で決める
B. 依頼者本人が決め、OSS のコントリビューターの意見を参考にする
C. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q7. 進み具合の共有

A. なし(個人で進める)
B. GitHub のプルリクエストと Releases のノートで共有する
C. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q8. この版で作らないもの(select all that apply)

計画は `mvp` で進めると決まっています。そのうえで、この版で作らないものを選んでください。

A. CDK で、タグを式で作っているもの(変数、関数の呼び出し、ループ、条件)の自動の反映。手で直す一覧に出すだけにする
B. CDK の TypeScript と Python 以外の言語(Java、C#、Go)
C. GitHub 以外(GitLab、Bitbucket など)へのプルリクエストやマージリクエストの作成
D. プルリクエストを作ったあとの操作(マージ、レビュアーやラベルの指定など)
E. 特になし
X. Other (please specify)

[Answer]: E

## Consolidated Summary Confirmation

- 解決したい問題: CDK のタグが次の `cdk deploy` で元に戻ること、と、プルリクエストを出すまでの手作業の両方(Q1: C)
- 利用者: 前の intent の P1(佐藤さん)と同じ(Q2: A)
- 成功の基準: AC7.2.1 と AC7.3.1 を自動テストで満たすこと。数の目標は置かない(Q3: C)
- きっかけ: `v0.1.0` を公開し、U8 で後回しにした Could の2つに取りかかる順番が来た(Q4: A)
- 関係者: 前の intent と同じで、増えない(Q5: A)
- 決める人: 依頼者本人が一人で決める(Q6: A)
- 進み具合の共有: なし(Q7: A)
- この版で作らないもの: 特になし(Q8: E)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
