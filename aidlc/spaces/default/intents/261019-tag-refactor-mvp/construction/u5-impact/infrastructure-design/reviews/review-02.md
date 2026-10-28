## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-25T02:58:08Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md > 検証用アカウントのリソース > RU5、結合テスト手順 4 | 結合テストは「RU5 の `team` の名前の変更が原因の `Resource` の指摘」を必須としているが、RU5(IAM のロール)のタグが `plan --region ap-northeast-1` の変更に出ることは、どこにも確かめられていない。IAM のロールは全リージョン共通のリソースで、scan の入口(タグ用 API、AWS Config、Resource Explorer)はリージョンごとに引く。これまでの確認用リソース(U1 の R1・R2、U2、U4 のテスト)は、タグ付きの IAM ロールを scan が拾うことを一度も使っていない。`team` の変更が無ければ、BR3.3 の「IAM のユーザーとロールの変更」が空になり、`PrincipalTag/team` と `Resource` の指摘は出ず、結合テストは必ず失敗する(検証コマンドが通らない)。 | コード生成の前に、既存のアカウントで読み取りだけの確認をして、RU5 相当のタグ付きロールが `--region ap-northeast-1` だけで変更に出るかを確かめ、結果をこの文書に書く。出ないなら、テストと検証スクリプトの plan の `--region` に IAM を引くリージョン(例: us-east-1)を足すか、`PrincipalTag`/`Resource` の期待を IAM のロールが確実に出る形に変える。どちらかをこの文書で決める。 | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md > 検証用アカウントのリソース(U5 の確認用のスタックの表) | テンプレートの書き方が、そのままでは作れない、または期待する指摘が出ない形になりうる。(1) バケットポリシー、キューポリシー、トピックポリシー、シークレットのリソースポリシーは CloudFormation では別のリソースの種類(`AWS::S3::BucketPolicy`、`AWS::SQS::QueuePolicy`、`AWS::SNS::TopicPolicy`、`AWS::SecretsManager::ResourcePolicy`)だが、表には6つのリソースしかなく、どの種類をどう足すか決まっていない。(2) RU5 の `Resource` の `${aws:PrincipalTag/team}` は、バケットの ARN を入れるために `Fn::Sub` を使うと CloudFormation の置き換えとして読まれ、`${!aws:PrincipalTag/team}` と書かないとスタックの作成が失敗するか、変数が消える。文書はこの書き方を決めていない。(3) S3、SQS、SNS、Secrets Manager のリソースポリシーの条件キーに `aws:ResourceTag/env` を使うことが、各サービスで受け付けられるか(S3 はバケットの ABAC の有効化に左右される)を確かめた根拠がない。受け付けられないと作成が失敗し、AWS への変更の手順の途中で止まる。 | 表に、ポリシーを付ける4つのリソースの種類と依存(ポリシー → 対象)を足す。RU5 の `Resource` は `Fn::Sub` の `${!...}` による書き方(または `Fn::Join`)を明記する。5つのポリシーが条件キーつきで作成できることを、`cfn-lint` と、人の許可を得た作成の前の確認(各サービスの文書か、変更セットの検証)で確かめ、受け付けられないサービスがあるときの代わりの形(例: 条件を `StringEquals` から別の形へ、またはそのサービスだけ偽の AWS のテストに任せる)を決めておく。 | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md > AWS への変更の手順(人の許可を得てから) | 人に示す手順が、そのまま実行できず、許可の意味も弱い。(1) IAM のロールを作る・更新するスタックには `--capabilities CAPABILITY_IAM` が要るが、書かれていない。(2) U1 のスタックの更新は、`SsoPermissionSetName` に既定値が無いため `aws cloudformation deploy` に `--parameter-overrides` が要る(`deploy` は前の値の再利用ができない)が、U5 のスタックの分しか触れていない。(3)「変更セットで先に確かめる」と「`aws cloudformation deploy` で行う」が両立しない。`deploy` は既定で変更セットを作ってすぐ実行するため、R2 のポリシーだけが変わることを確かめる機会が無い(`--no-execute-changeset` が要る)。人が許可する内容が、実行されるコマンドと変更セットの中身になっていない。 | 人に示す内容を、実際のコマンド(`--capabilities`、`--parameter-overrides`、`--tags`、`--no-execute-changeset` で変更セットを作って `describe-change-set` で R2 の `Policies` だけの変更とロールが置き換えにならないことを示す手順、許可のあとに `execute-change-set`)に書き換える。U5 のスタックの作成も同じ形にする。 | Resolved (reviewer) |
| R-04 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 結合テスト手順 3 | `impactAnalysisComplete` が true であることを必須にしているが、アカウント全体(ap-northeast-1 のほかのキー、キュー、トピック、シークレット、バケットのポリシー、残った U2 のキューなど)がすべて R2 の権限で読めることが前提になる。前提の確認も、スタックの外の読めなかったものを許す設けも無い。KMS の状態によっては `KMSInvalidStateException` が Internal になり不完全になる(`security-design.md` §1)。 | 結合テストが、読めなかったものを出して失敗にする(今の案)のに加え、スタック外の読めなかったものだけが原因のときは、その旨を切り分けて表示する、または前提(アカウントに R2 で読めないポリシーが無いこと)を文書に書く。 | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 動かし方、結合テスト手順 2 | 手順 2 の plan のコマンドに `--profile` が無い。U4 のテストは `--profile` に `TAG_REFACTOR_IT_PROFILE` を渡している。渡さないと管理用の認証情報で動いて R2 の権限の不足(NFR3.9 の受け入れ条件)が見えなくなる。環境変数も `TAG_REFACTOR_IT_U5_ROLE など` と省略されていて、6つの名前と、スタックの出力のキー名が決まっていない。前の確かめは、スタックの状態だけで、R2 が更新済みかを見ない(未更新だとテストが読めなかったもので落ちるが、案内が出ない)。 | `--profile` を手順に明記する。6つの環境変数名と出力のキー名を表にする。前の確かめに、R2 のポリシーに足した操作(例: `iam:GetRolePolicy` の対象)が入っていることの確認か、失敗時の案内(「U1 のスタックの R2 を更新する」)を足す。 | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 測る値の記録、チェックポイントでの人の確認 | L95 の測り方が決まりきっていない。(1) 測る処理が Go のテストの中か `verify.sh` の中か、見積もりの式(`logical-components.md`)を誰が計算するかが書かれていない。(2) 検証用アカウントは1リージョンで数個のリソースしかなく、KMS、SNS、Secrets Manager の操作は標本が数件で、L95 が最大値と同じになる。(3) 式の L95(IAM)、L95(リソースポリシー)に、操作ごとの L95 をどう1つにまとめるか(最大値か)が無い。(4) 上限に当たった待ち時間の回数を数えることが書かれていない。 | 標本の最小数(足りないときは繰り返して集める)、まとめ方(例: 種類ごとの最大の L95)、出力する行の形、計算を人が行うか `verify.sh` が出すかを書く。 | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md > `ci.yml` > 足すテスト、traceability.json NFR8.20 | `lint` ジョブの `cfn-lint test/sandbox/*-fixtures.yaml` が新しい `u5-fixtures.yaml` を自動で検査するのに、「`ci.yml` の変更なし」の説明がこれに触れていない(ポリシー変数や `Fn::Sub` の警告が CI を落としうる)。また NFR8.20 の受け入れ条件(1 MiB + 1 バイトで読めなかったものと終了コード1、ちょうど 1 MiB は読まれる)のテストが、足すテストの表に明示されていない。 | 表に cfn-lint の対象が増えることを書く。NFR8.20 の大きさの境界のテストを、表の行として足す。 | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md > 検証用アカウントのリソース > 条件キーを受け付けるかの確かめ | `ValidatePolicy` の条件を「ポリシーの種類はリソースポリシー」としているが、6つのうち RU5 の `u5-inline` は IAM ロールのインラインポリシー(アイデンティティポリシー)で、種類が違う。そのまま実行すると RU5 の確認だけ誤った種類で当てることになる。 | RU5 はアイデンティティポリシーの種類で、ほかの5つはリソースポリシー(リソースの種類つき)で当てる、と書き分ける。 | New |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u5-impact/infrastructure-design/cicd-pipeline.md > 測る値の記録 | 「再試行の回数が1以上の行の数」を「呼び出し回数の上限に当たった回数」と呼んでいる。再試行はスロットリング以外の一時的なエラーでも起きるため、上限に当たった回数とは限らない。 | 名前を「再試行が起きた行の数」にするか、上限に当たったと判定できる条件(スロットリングの応答など)を書く。チェックポイントの見積もりの判断には影響しないので、コード生成の中で直してよい。 | New |

> R-01 Reviewer note: 結合テスト手順 7 が、RU5 の `team` の変更が変更案にあるときだけ `Resource` の指摘を必須にした。無いときはログに理由を出して続け、どちらだったかをチェックポイントの記録に残す。「必ず失敗する」状態はなくなった。手順 5 の RU5 `iamInline` の `env` の指摘は、RU5 が収集に出るかに依存せず、QU5 などの `env` の変更が原因なので成り立つ。`PrincipalTag`/`Resource` の照合は NFR2.1 の偽の AWS のテストが担う。実環境での確認は弱まるが、分担は明記されていて実装できる。

> R-02 Reviewer note: 4つのポリシーのリソースの種類(BucketPolicy、QueuePolicy、TopicPolicy、ResourcePolicy)と依存を表に足した。`${!aws:PrincipalTag/team}` の書き方を明記した。作成の前に `ValidatePolicy`、`cfn-lint` で確かめ、サービスが拒んだときの代わりの手順(その種類をテンプレートから外し、偽の AWS のテストで確かめ、記録に残す)も決めた。

> R-03 Reviewer note: 変更セットを作る `create-change-set`(`CAPABILITY_IAM`、`--tags`、`--region`、`--profile`、U1 のパラメーター指定を含む)、`describe-change-set` で示す、許可のあとに `execute-change-set` と `wait`、許可がないときは `delete-change-set`、の順になった。U1 のテンプレートのパラメーター `SsoPermissionSetName` とも合う。

> R-04 Reviewer note: 手順 4 に、`impactAnalysisComplete=true` がアカウント全体のポリシーを R2 で読めることに依存する前提を書き、false のときの内訳の表示も決めた。

> R-05 Reviewer note: `plan` に `--profile` を明記し、6つの環境変数と出力のキー名を表にした。前の確かめに R2 の `iam:GetRolePolicy` の有無の検査と案内を足した。

> R-06 Reviewer note: 測り方(`verify.sh` が標準エラーの `--debug` の行を集める、5回の標本、操作ごとの L95、`L95(IAM)` と `L95(リソースポリシー)`、標本が少ないときの表示、判定は人が行う)を決めた。

> R-07 Reviewer note: `ci.yml` の表に、文書の大きさの境目(NFR8.20)の行と、U5 のテンプレートが `cfn-lint` の対象に自動で入る行を足した。

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| aidlc engine sensor-traceability | pass: true(gaps、orphans、missing_from_table、invalid_entries、invalid_targets はすべて空、findings_count 0) | 追跡表に欠けも孤立もない。 |

### Summary

前回の Major 3件と Minor 4件はすべて解消している。R-01 は、実環境で確かめられない部分を条件つきの確認に変えて失敗の確実性をなくし、照合そのものを偽の AWS のテストに任せる分担を明記したので、実装できる形になった。新しく見つかった2件はどちらも Minor で、コード生成の中で直せる。Critical は無く、Major は0件のため READY とする。
