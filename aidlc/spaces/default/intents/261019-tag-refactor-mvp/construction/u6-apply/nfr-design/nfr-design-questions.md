# NFR の設計 — 質問(U6 書き換えと記録)

NFR の要件(`construction/u6-apply/nfr-requirements/`)を、部品の分け方と、書き換え用の権限の例の中身に落とします。

決まっていること(質問しません):

- 書き込みの進め方(まとまり20、読み直しの単位100、リージョン4つまで同時、リージョンの中は順。NFR5.13)、記録の保存(5秒ごと、重ねない、3回続けて失敗で止める。NFR6.2)、印(手元は既定の場所、S3 は条件付きの書き込み。NFR7.1)、止める条件(NFR5.17)、権限(NFR8.24)。
- `tag:TagResources`・`tag:UntagResources`・`tag:GetResources` はリソースの範囲を指定できない(AWS の Service Authorization Reference で確かめた。Resource types の欄が空)。例のポリシーでは `"*"` の文に入れる。
- タグの API で書き換えるには、種類ごとのタグの操作の権限も要る(例: `ec2:CreateTags`、`sqs:TagQueue`、`s3:PutBucketTagging`)。AWS 管理ポリシー `ResourceGroupsTaggingAPITagUntagSupportedResources` に、対応するすべての種類の操作の一覧がある。

## Q1. 書き換え用の権限の例に入れる種類

`docs/iam/apply-policy.json` に、どの種類のタグの操作を入れますか?(NFR3.11 の「書き換えの対象の種類の表」の範囲)

A. よく使う11種類: EC2(インスタンス、ボリュームなど `ec2:CreateTags`・`ec2:DeleteTags`)、S3 のバケット、SQS、SNS、Secrets Manager、IAM(ロール、ユーザー、管理ポリシー)、KMS、Lambda、DynamoDB、CloudWatch Logs、RDS。操作はアカウントとサービスの ARN の形に絞る。ほかの種類は、README で AWS 管理ポリシー `ResourceGroupsTaggingAPITagUntagSupportedResources` を案内する
B. 検証用アカウントの確認用のスタックにある5種類だけ(S3 のバケット、SQS、SNS、Secrets Manager、IAM のロール)。ほかは README で AWS 管理ポリシーを案内する
C. 自前の例は作らず、README で AWS 管理ポリシーと、読み取り用の例と S3 の記録の操作を足すことを案内する(NFR3.11 を直す必要がある)
X. Other (please specify)

[Answer]: A

## Q2. 部品の分け方(パッケージ)

U6 の部品を、どのパッケージに置きますか?(U7 の rollback も記録と印を使います)

A. `internal/journal`(記録、印、壊れない保存、5秒ごとの保存。U7 も使う)と `internal/apply`(書き換えの流れ、まとまりの計算、変化の確かめ、結果)を新しく作る。TagWriter と ObjectStore の実装は `internal/awsaccess`、`apply` のコマンドは `internal/cli`、確認画面と結果の表示は `internal/display` に足す
B. `internal/apply` の1つにまとめる(記録と印も中に置く。U7 はこのパッケージを使う)
C. A と同じだが、まとまりの計算と変化の確かめを `internal/planner` に置く(変更案に近いため)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答をまとめると、U6 の NFR の設計は次の内容になります。

1. **書き換え用の権限の例(Q1 A):** `docs/iam/apply-policy.json` に、よく使う11種類のタグの操作を入れます。EC2(`ec2:CreateTags`・`ec2:DeleteTags`)、S3 のバケット、SQS、SNS、Secrets Manager、IAM(ロール、ユーザー、管理ポリシー)、KMS、Lambda、DynamoDB、CloudWatch Logs、RDS です。操作はアカウントとサービスの ARN の形に絞ります。`tag:TagResources`・`tag:UntagResources`・`tag:GetResources` は `"*"` の文に入れます。ほかの種類は、README で AWS 管理ポリシー `ResourceGroupsTaggingAPITagUntagSupportedResources` を案内します。
2. **部品の分け方(Q2 A):** `internal/journal`(記録、印、壊れない保存、5秒ごとの保存。U7 も使う)と `internal/apply`(書き換えの流れ、まとまりの計算、変化の確かめ、結果)を新しく作ります。TagWriter と ObjectStore の実装は `internal/awsaccess`、`apply` のコマンドは `internal/cli`、確認画面と結果の表示は `internal/display` に足します。

Does this all look correct before I generate the artifact?

[Answer]: Looks correct
