# 意図の把握と整理 — 質問

## Sources

- [desc] Initial description: "AWSのタグを後から統一するCLIツール tag-refactor を作りたい。まず読み取り専用の scan と plan(影響分析つき)を MVP とする"
- [scope] Workflow-selected scope: `mvp`.

## Q1. 解決したい問題

このツールで解決したい問題はどれですか?(select all that apply)

A. タグのキーや値の表記ゆれ(`env` / `Environment` など)で、コスト配分やレポートが分断されている
B. キー名を変えると、IAMの条件やAWS Backupの選択条件などが壊れそうで、統一に踏み切れない
C. 必須のタグが付いていないリソースがあり、所有者や用途が分からない
D. 既存のツールでは、タグを直したときに何が壊れるかを事前に確かめられない
E. Not yet defined
X. Other (please specify)

[Answer]: A, B, C, D

## Q2. 主な利用者

このツールを主に使うのは誰ですか?

A. 自分自身(自分のAWSアカウントで使う)
B. 社内のプラットフォーム、FinOps、セキュリティなどの担当チーム
C. 一般公開するOSSとして、外部の利用者
D. B と C の両方
E. Not yet defined
X. Other (please specify)

[Answer]: D

## Q3. 成功の基準

MVP(読み取り専用の scan と plan)が成功したと判断する基準はどれですか?(select all that apply)

A. 表記ゆれや必須タグの欠落を、件数つきで一覧にできる
B. キーを変更したときに影響を受ける参照元(IAMポリシー、AWS Backupなど)を、plan が漏れなく示せる
C. 実際のアカウントで現実的な時間で終わる(例: 1アカウント・数千リソースで数分以内)
D. OSSとしての反響(GitHubのスター数、利用者数など)
E. Not yet defined
X. Other (please specify)

[Answer]: A, B, C

## Q4. 作るきっかけ

なぜ今、このツールを作るのですか?(select all that apply)

A. 自分やチームの環境で、実際にタグの表記ゆれが問題になっている
B. 既存のツールでは、タグ変更の影響分析ができない
C. 新規性のあるツールを作りたい(学習やポートフォリオを含む)
D. Not identified
X. Other (please specify)

[Answer]: A, B, C

## Q5. 関係者

このツールに関わる人(使う人、意見を言う人、影響を受ける人)は誰ですか?(select all that apply)

A. 自分のみ
B. 社内の利用者(プラットフォーム、FinOps、セキュリティ担当など)
C. OSSの利用者やコントリビューター
D. Not identified
X. Other (please specify)

[Answer]: B, C

## Q6. 範囲や優先順位を決める人

MVPに何を入れるか、何を先に作るかは誰が決めますか?

A. 自分が一人で決める
B. 社内の上長やチームと合意して決める
C. 自分が決め、OSSの利用者の声も参考にする
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q7. 進捗の共有

進捗を報告する相手や、決まった報告の頻度はありますか?

A. ない(個人で進める)
B. ある(相手と頻度を Other で教えてください)
C. Not applicable
X. Other (please specify)

[Answer]: A

## Q8. 最初のバージョンに含めないもの

計画は `mvp` で進めることが決まっています。そのうえで、最初のバージョン(読み取り専用の scan と plan)に**含めない**ものはどれですか?(select all that apply)

A. タグの書き込み(apply)と取り消し(rollback)
B. IaC(Terraform、CloudFormation、CDK)への修正の書き戻し
C. コスト集計を統一する Cost Category の生成と、Tag Policy の生成
D. 複数アカウント(Organizations)の横断と、欠けているタグの値の推定
E. None(すべて最初のバージョンに含める)
X. Other (please specify)

[Answer]: C, D

## Q9. 書き込みとIaCへの書き戻しの扱い(Q8の確認)

最初の説明では、MVPを「読み取り専用の scan と plan」としていました。一方、Q8では「タグの書き込み(apply)と取り消し(rollback)」と「IaCへの修正の書き戻し」を、対象外に選んでいません。この2つを最初のバージョンでどう扱いますか?

A. 両方とも最初のバージョンには含めない(読み取り専用の scan と plan だけにする)
B. IaCへの修正は「差分を出力するだけ」なら含める(AWSにもファイルにも書き込まない)。apply と rollback は含めない
C. apply と rollback も含める(読み取り専用ではなくなる)
D. 両方とも含める(apply、rollback、IaCへの書き戻しのすべて)
E. Not yet defined
X. Other (please specify)

[Answer]: D

## Consolidated Summary Confirmation

回答のまとめ:

- 解決したい問題(Q1): 表記ゆれによる集計の分断、キー変更で権限やバックアップ設定が壊れる不安、必須タグの欠落、変更の影響を事前に確かめられないこと
- 主な利用者(Q2): 社内の担当チームと、OSSの外部利用者の両方
- 成功の基準(Q3): 問題を件数つきで一覧化できる、影響を受ける参照元を漏れなく示せる、現実的な時間で終わる
- きっかけ(Q4): 実際に問題になっている、既存ツールでは影響分析ができない、新規性のあるツールを作りたい
- 関係者(Q5): 社内の利用者、OSSの利用者・コントリビューター
- 決める人(Q6): 自分が一人で決める
- 進捗の共有(Q7): なし(個人で進める)
- 最初のバージョンに含めないもの(Q8): Cost Category と Tag Policy の生成、複数アカウントの横断と値の推定
- 書き込みとIaCへの書き戻し(Q9): apply、rollback、IaCへの書き戻しをすべて最初のバージョンに含める(最初の説明の「読み取り専用」から範囲を広げる)
- 実行時間の目標(Q10): 1アカウント・全リージョン・10,000リソースで、scan と plan が合わせて10分以内
- 影響分析の対象(Q11): IAMポリシー・SCP・RCP の条件キーと、リソースポリシー(S3、KMS、SQS、SNS、Secrets Manager など)の条件キー
- 「漏れなく」の判定(Q12): テスト用に用意した参照をすべて検出できる(検出率100%)
- 社内の利用者が期待すること(Q13): 本番を壊さず安全に変更できる、コスト配分の正確さ、作業時間の短縮、監査・コンプライアンスへの対応
- OSSの利用者が期待すること(Q14): 導入の手軽さ、書き込み権限なしで安全に試せる、対応範囲の広さ

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct

## Assumption Confirmation

文書の中に、確認できていない前提が4つあります。

- 「現実的な実行時間」の具体的な数値(リソース数と許容時間)は、正式な目標値としては確認していない。要件定義で決める。
- 「参照元を漏れなく示す」をどう測るか(対象とする参照元の種類の一覧と、その検出率)は未定。要件定義で決める。
- 社内の利用者が、このツールに何を最も期待しているか(関心事)は確認していない。
- OSSの利用者・コントリビューターが、このツールに何を最も期待しているか(関心事)は確認していない。

A. Accept assumptions
B. Convert to follow-up questions

[Answer]: B. Convert to follow-up questions

## Q10. 実行時間の目標(Q3の補足)

成功の基準に「現実的な時間で終わる」を選びました(Q3)。具体的な目標はどれにしますか?

A. 1アカウント・1リージョン・5,000リソースで、scan と plan が合わせて5分以内に終わる
B. 1アカウント・全リージョン・10,000リソースで、scan と plan が合わせて10分以内に終わる
C. 数値の目標は置かず、実行時間を計測して表示するだけにする
D. Not yet defined
X. Other (please specify)

[Answer]: B

## Q11. 影響分析の対象にする参照元(Q3の補足)

成功の基準に「影響を受ける参照元を漏れなく示せる」を選びました(Q3)。最初のバージョンで、どの種類の参照元を対象にしますか?(select all that apply)

A. IAMポリシー(ユーザー・ロール・グループ)、SCP、RCP の条件キー
B. リソースポリシー(S3バケット、KMSキー、SQS、SNS、Secrets Manager など)の条件キー
C. AWS Backup の選択条件、DLM のライフサイクルポリシー、SSM のタグによる対象指定
D. Resource Groups のタグ条件、Auto Scaling グループと起動テンプレートのタグ
E. Not yet defined
X. Other (please specify)

[Answer]: A, B

## Q12. 「漏れなく」の判定方法(Q3の補足)

Q11で選んだ種類の参照元について、「漏れなく示せた」とどう判定しますか?

A. テスト用に用意した参照を、すべて検出できる(検出率100%)
B. テスト用に用意した参照の90%以上を検出できる
C. 数値の基準は置かない
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q13. 社内の利用者が最も期待すること(Q5の補足)

社内の利用者(プラットフォーム、FinOps、セキュリティ担当など)は、このツールに何を最も期待していますか?(select all that apply)

A. 本番環境を壊さずに安全にタグを変更できること
B. コスト配分の正確さ
C. タグ整理にかかる作業時間の短縮
D. 監査やコンプライアンスへの対応
E. Not identified
X. Other (please specify)

[Answer]: A, B, C, D

## Q14. OSSの利用者・コントリビューターが最も期待すること(Q5の補足)

OSSの利用者やコントリビューターは、このツールに何を最も期待していますか?(select all that apply)

A. 導入の手軽さ(単一のバイナリ、少ない設定)
B. 書き込み権限なしで安全に試せること
C. 対応するリソースや参照元の種類の多さ
D. 拡張やコントリビュートのしやすさ
E. Not identified
X. Other (please specify)

[Answer]: A, B, C
