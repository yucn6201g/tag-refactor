## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-29T02:27:34Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 使うルールファイル(`u6-add-rules.yaml`、`u6-untagged-rules.yaml`)、手順 1-1 | `u6-add-rules.yaml` は `match.tags: project=tag-refactor-fixture` だけで対象を決めているが、このタグはスタックタグなので、U1・U2・U5・U6 の4つの確認用スタックのすべてのリソースに付いている(`test/sandbox/u1-fixtures.yaml`、`u2-fixtures.yaml`、`u5-fixtures.yaml` の冒頭の説明で確認)。「確認用のスタックのリソースだけが対象」は正しいが、対象は手順 1-1 が確かめる6件(BU5・QU5・TU5・SU5・RU5・GU6)より広い。U1 のキュー3つ・バケット・管理ポリシー・ロール、U2 の Resource Explorer の index と view、R3 自身、CloudFormation のスタックそのものも含みうる。R3 の種類の表にない種類(Resource Explorer、CloudFormation など)は AccessDenied の `failed` になりうるが、手順 1-1 は「`failed` が0件」を合格条件にしているため、設計どおりには通らない可能性がある。逆に、これらを `skipped` にする規則も設計にない。`u6-untagged-rules.yaml` は `resourceType: AWS::SQS::Queue` だけで、個人アカウントのリージョン内のすべてのキュー(確認用以外を含む)に値 `1` を付けて消す。契約 C1 は `when` が複数当てはまるときの規則と、`match.tags` で `aws:` のタグを使えるかを決めていないので、この設計は範囲を絞る別の手段を示す必要がある。 | (1) 書き換える前に、読み取り専用の `plan --rules <add-rules> --format json` を動かし、変更される ARN の集合が期待する確認用リソースの集合に含まれることを確かめてから `apply` に進む(超えたら何も書き換えずに止める)。(2) `when` を種類ごとに分け、`resourceType` と `match.tags` の両方を書いて、R3 の表にある6種類に絞る。または、範囲外の項目を `skipped` か `failed` のどちらで扱い、合格条件にどう入れるかを決める。(3) `u6-untagged-rules.yaml` は、未タグのキューの名前(`tag-refactor-u2-untagged-*`)や `region` で絞るか、他のキューも書き換わることを許容する理由と範囲を書く。 | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 手順 1-1・1-2・4(終了コード) | 手順 1-1 は終了コードを「0 か 2」で受け入れるとしているが、終了コード 2 は「値が未指定の必須タグが残る」場合のほかに、`--yes` だけで影響のある変更(影響分析が不完全な場合を含む)があって何も書き換えずに止まる場合にも使われる(機能設計 W1 手順 6、BR2.5)。この設計は `--allow-impact` を渡さず、記録が `completed: true` であることも確かめないため、「書き換えずに止まった」場合と「書き換えて、確認用のスタックの外に未指定の必須タグが残った」場合を区別できない。記録の JSON が作られないために、読み取りの段階で分かりにくく失敗するだけになる。手順 4 の `u6-untagged-rules.yaml` の `apply` の終了コードは何も書かれておらず、手順 1-2 の「終了コード 0」も、`skipped`(変更案の作成後に変わったなど)が出れば 2 になるので、くり返しで揺れる。 | 各 `apply` の合格条件を、終了コードだけでなく、記録の JSON(存在すること、`completed: true`、`status` ごとの件数、6件が `applied`)で決める。終了コード 2 は、記録が書き換えを示している場合だけ許し、記録が作られていない 2 は失敗にして案内を出す。手順 4 の期待する終了コードも書く。`--allow-impact` を使うかどうかを、安全のために決めて書く。 | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 手順 6(`trap` による後片付け)、scripts/verify.sh の `cleanup` | 後片付けの仕様が、既存の `scripts/verify.sh` の作りと合わない。(1) 既存の `cleanup` は最初に `rm -rf "$u3_dir"` を行うが、`tag-refactor` の実行ファイルはその中(`$tool`)にあるので、そのあとに「確かめ専用のキーを消す apply」は動かせない。順序の指定がない。(2) `trap cleanup EXIT` はスクリプトの先頭で定義されているため、`verify-u1.sh` の失敗や R3 のプロファイルの確認前の失敗でも、U6 の後片付けが無条件に動いてしまう。キーを足した可能性があるときだけ動かす旗がない。(3) `apply` が強制終了などで印(`<ホーム>/.tag-refactor/runs/<accountId>/.lock`)を残したら、後片付けの `apply` も Conflict で止まり、次回の検証も止まる。`--force-unlock` の扱いが決まっていない。(4) 「1回動かす」だけで、SSO の期限切れなどで失敗したときの手がかりが、残ったキーを探す方法(下の指摘)に依存する。 | 後片付けの順序を決める: キーを消す `apply` を、`$u3_dir` を消す前に動かす。旗(例: キーを足す最初の `apply` の前に立て、消えたと確かめたら下ろす)があるときだけ動かし、実行ファイルがまだなければ動かさない。後片付けの `apply` が Conflict のときは、残った印の中身を示して `--force-unlock` を使うか、手で外すよう案内する。成功した後の手元の印と S3 の印が残っていないことも確かめる。 | Resolved (reviewer) |
| R-04 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 手順 2(残っていないことの確かめ)、手順 6 | 「残っていない」の確かめは、管理用のプロファイルで `--region ap-northeast-1` の `tag:GetResources` を1回呼ぶだけで、(1) IAM のロール(RU5、R3、U1 のロール)は IAM のグローバルなタグとして us-east-1 の Resource Groups Tagging API で返るのが普通なので、ap-northeast-1 の呼び出しでは見えず、IAM の種類のキーの取りこぼしを見逃しうる(しかも NFR3.11 が確かめを求める6種類の1つ)。(2) `GetResources` は結果整合なので、`remove` の直後の1回の呼び出しは、消えたキーを返して誤って失敗にしたり、書いた直後の新しいキーを返さずに誤って合格にしたりしうる。設計は IAM のグローバルなリソースを apply が us-east-1 で扱うかどうかも書いていない(手順 1-1 は RU5 が `applied` になることを期待している)。 | 残っていない確かめを、us-east-1 も含めて行い、IAM のロールは `iam list-role-tags` などサービス自身の読み取りでも確かめる。一定の時間内で再読み(上限つき)する。手順 1-1 で RU5 が書き換え対象になるリージョンの扱い(`--region ap-northeast-1` のときに IAM のグローバルなリソースを含めるか)を、`plan` の結果で確かめる手順にする。 | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 末尾の注(「U1〜U5 の確かめは…U6 の確かめの前に動かします」)、差し込む位置 | 注は U1〜U5 の確かめを U6 の前に動かすとしているが、差し込む位置は U5 のあと・U2 の index 確認とキューの作成(`verify.sh` の手順 3〜7 にあたる U2 の確かめ)の前で、U2 の最大30分の待ち、U2 の sandbox テスト、全リージョンのレイテンシ測定が U6 の手順 1〜3・5・6 のあとに動く。注の文と配置が矛盾している。 | 注を実際の順序に合わせて直すか、U2 の確かめのあとに U6 の手順 1〜3・5・6 を置く。 | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/infrastructure-design/cicd-pipeline.md > 検証用アカウントのリソース > R3、BU6R、手順 3 | R3 の `s3:ListBucket` は `s3:prefix` の条件つきで、「ない記録の `GetObject` を AccessDenied ではなく NotFound にするため」とされている。しかし、`GetObject` が存在しないキーで 404 か 403 を決めるときの `s3:ListBucket` の判定には `s3:prefix` が付かないため、条件つきの許可が効かず AccessDenied のままになる可能性がある。この検証では、存在しない記録の `Get` が NotFound になることを R3 で確かめる手順がなく、この設計の根拠が試されない。また、BU6R には TLS だけを許すバケットポリシーがなく(BU5 にはある)、手順 3 は暗号化と ACL を確かめないのに、トレーサビリティは NFR8.25 を「OK」としている。 | 手順 3 に、存在しない記録の `Get` が NotFound になる確かめと、S3 に置かれたオブジェクトの暗号化・ACL の確認(例: `head-object` と `get-object-acl`)を足す。足さないなら NFR8.25 の対応を「単体テストのみ」に直す。BU6R に TLS だけを許すポリシーを足すか、足さない理由を書く。 | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 手順 5(見積もり)、測る値の記録 | 5回の確認用のスタックだけの `apply` は、1回の `TagResources` で数件しか書かないので、20件のまとまりや 100 ARN の読み取りの L95 を代表しない(サンプルが少なく「最大」になる)。NFR5.15 の見積もりはこの L95 を 2,000 件に外挿するため、チェックポイントでの判断の根拠が弱い。R3 の権限をそのまま使うなら、`apply` の範囲を広げられない点も記録に残す必要がある。 | 見積もりの前提(サンプル数が少なくて「最大」であること、まとまりの大きさが実際と違うこと)を測る値の記録に明記し、チェックポイントでその限界を踏まえて判断することを書く。 | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/infrastructure-design/cicd-pipeline.md > 検証のスクリプト > 後片付け(1つ目の項目)と手順 4・手順 5 | 「キーを足したかもしれない」旗は手順 4 で残っていないことを確かめたら下ろすが、手順 5 は旗を下ろしたあとに untagged のキューへ再びキーを足す。手順 5 の途中で失敗しても旗が立っておらず、後片付けの remove の `apply` が動かない。キューは最後に消されるので被害は小さいが、旗の仕様と手順の順序が合わない。 | 手順 5 の最初の add の `apply` の前に旗を立て直し、手順 5 の remove が記録で確かめられたら下ろす。または旗を下ろすのは手順 5 のあとの1か所にする。 | New |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/infrastructure-design/cicd-pipeline.md > 検証用アカウントのリソース > U6 の確認用のスタック(BU6R、BU6RPolicy)と、infrastructure-design-questions.md の Q1・まとめ | 人が承認した Q1 A とまとめは、U6 のスタックを「VPC、セキュリティグループ、R3 だけ」としているが、この設計は記録用の S3 バケット BU6R とそのバケットポリシーを足している(費用の欄も「記録のオブジェクトの分」と変わる)。S3 の記録の確かめ(NFR8.25)には必要だが、承認済みの範囲との差が設計に書かれていない。 | 設計に「Q1 A への追加」として BU6R・BU6RPolicy を足した理由と費用(記録のオブジェクトの分だけ)を明記し、チェックポイントで人が確認できるようにする。 | New |

> R-01 Reviewer note: `when` を6つの種類ごとに分け、`resourceType` と `match.tags` の両方で絞った。手順 2 が書き換え前に読み取り専用の `plan` を動かし、変更の種類、`beforeTags`、6つの対象 ARN の有無、`impacted` がないことを確かめ、外れれば何も書き換えずに止める。untagged のルールの範囲も手順 2 で確かめる。

> R-02 Reviewer note: 手順 3 の合否を記録の JSON で決める(`completed: true`、`failed`・`pending` が0件、6件が `applied`、範囲外の項目なし)。記録が作られていない終了コード 2 は失敗にする。`--allow-impact` は付けないと明記した。

> R-03 Reviewer note: 後片付けを旗つきにし、`$u3_dir` を消す前に、旗が立っていて `$tool` があるときだけ remove の `apply` を動かす。Conflict のときは印の中身を示して `--force-unlock` を案内する。実際の `verify.sh` の `cleanup` の作りと矛盾しない。旗を立てる位置のずれは、下の新しい指摘に残る。

> R-04 Reviewer note: 手順 4 が `tag:GetResources` を ap-northeast-1 と us-east-1 で呼び、IAM のロールは `iam list-role-tags` でも確かめる。10秒おき最大6回の読み直しも入った。手順 2 が `--region` なしで IAM のグローバルなリソースを集め、RU5 が入っていなければ失敗にする。

> R-05 Reviewer note: 注と配置が一致した。U6 の確かめはすべての確かめの最後、U2 のタグのないキューを消す前に置く。

> R-06 Reviewer note: 手順 6 に、`head-object` の暗号化、`get-object-acl`、存在しないキーが NotFound になることの確かめを足した。BU6RPolicy で TLS でない要求を拒む。

> R-07 Reviewer note: 「見積もりの限界」を足し、標本が少なく L95 が実質の最大になること、まとまりの大きさが実際と違うことを書いた。チェックポイントでその限界を踏まえて判断する。

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| sensor-traceability | PASS(`gaps`・`orphans`・`invalid_*` はすべて空、`findings_count` 0) | 21件の上流 ID がすべて対応づいている。N/A の5件(NFR3.12、NFR5.12、NFR5.17、NFR6.2、NFR8.23)は理由つきで、単体テストで確かめる内容として妥当。 |

### Summary

前回の Major 4件と Minor 3件はすべて直っている。範囲の確かめ(手順 2)、記録による合否、旗つきの後片付け、us-east-1 と IAM を含む残存確認で、確認用のスタックの外を書き換える危険と判断の曖昧さは設計上閉じた。残るのは旗を立てる位置のずれと、承認済みの Q1 に対する BU6R の追加の記載漏れの2件の Minor だけで、実装を妨げない。
