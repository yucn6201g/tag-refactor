## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-08T12:51:49Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/infrastructure-design/cicd-pipeline.md > 5 検証用のスタックの手順 の手順4 (and infrastructure-specification.md > 使うサービス > AWS CloudFormation) | 手順4は `aws cloudformation deploy` を `--profile tag-refactor-apply` で動かすとしている。このプロファイルが引き受ける R3 は、test/sandbox/u6-fixtures.yaml の IAM ポリシーを見ると、タグの読み取りと書き換えの権限だけを持つ。CloudFormation のスタックの作成と S3 のバケットの作成の権限は無い。README の既存の手順では、スタックの設置は管理者のプロファイル `tag-refactor-sandbox` で行っている。このままだと検証用のスタックは置けず、検証コマンドの前提が崩れる | 設置に使うプロファイルを、スタックとバケットを作れるもの(管理者の `tag-refactor-sandbox` など)に直す。`tag-refactor-apply` は検証の `plan` と `apply` 側に限ると書く。権限を足す場合は、R3 のポリシーを広げる影響を明記する | New |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/infrastructure-design/infrastructure-specification.md > 配置 > IaC (and infrastructure-design-questions.md Q1 A) | ブートストラップをせずに `cdk synth` のテンプレートをそのまま `aws cloudformation deploy` で置くとしている。しかし既定の合成器で作ったテンプレートには、アセットが無くても、パラメータ BootstrapVersion(SSM パラメータ /cdk-bootstrap/.../version を参照)と、それを検査するルール CheckBootstrapVersion が入る。ブートストラップの無いアカウントでは、デプロイがこのパラメータの取得で失敗する。設計はこの点に触れておらず、テスト用のアプリがこれを止める設定を持つとも書いていない | 次のどちらかを書く。(1) テスト用のアプリが BootstrapVersion のルールを出さない設定(合成器の generateBootstrapVersionRule を false にする、またはブートストラップ不要の合成器)を使う。(2) 検証用のアカウントにブートストラップがあることを前提にする。あわせて Code Generation で、合成したテンプレートに BootstrapVersion が無いことを確かめる手順を加える | New |
| R-03 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/infrastructure-design/infrastructure-specification.md > 使うサービス > Amazon S3 | スタック名 `tag-refactor-cdk-skeleton` が iac の照合と同じになる根拠は、CDK のスタックの ID か stackName が同じ名前であることに依る。しかし、アプリのスタックの ID や stackName をどう決めるかが書かれていない。論理 ID はハッシュ付きで合成の結果に従うので、アプリを書き換えると置いたスタックと食い違う | テスト用のアプリで stackName を `tag-refactor-cdk-skeleton` に固定すると書く。アプリを変えたら再度置くことも、手順に1行加える | New |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/infrastructure-design/cicd-pipeline.md > 1 プルリクエストごとの CI > github-pr | ci.yml の concurrency は、プルリクエストで cancel-in-progress が true になっている。github-pr が実行中に取り消されると、t.Cleanup の後片付けが終わらず、テスト専用のリポジトリに閉じていないプルリクエストとブランチが残りうる。残りを見つけるのは、同じ実行のログだけである | 取り消された実行の残りを、次の実行の最初に掃除するか、github-pr を cancel-in-progress の対象から外す。または残りが出うることと、人が掃除する手順を書く | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u1-walking-skeleton/infrastructure-design/cicd-pipeline.md > 3 依存の更新(Dependabot) | Dependabot の npm の行は tag-refactor のリポジトリのテスト用のアプリだけを対象にする。テスト専用のリポジトリにも同じアプリの写しがあり、npm ci の対象になる(security-design.md の6節)が、その依存の更新は誰も見ない。固定した版が古くなる | テスト専用のリポジトリの写しを、tag-refactor 側と同じ内容に保つ手順(または更新の確認の担当)を書く | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| none declared | not run | 検証の道具は宣言されていない。ci.yml と dependabot.yml を直接読んで整合を確かめた |

### Summary

CI の設計(secret を github-pr の1手順の env だけに限ること、secret が無い実行は仕事の中で飛ばして成功とするため ci-gate が止まらないこと、CI が AWS を呼ばないこと、AWS の変更を許可の前に行わないこと)は、今の ci.yml と dependabot.yml に対して実装でき、NFR の設計と traceability.json の ID も整合している。ただし検証用のスタックの置き方に Major が2件あり(設置に使うプロファイルが権限を持たない点と、ブートストラップ無しでは BootstrapVersion のルールで失敗しうる点)、いずれも検証コマンドの前提に関わるので、Major は2件以内で READY としつつ、Code Generation の前に直すことを強く勧める。
