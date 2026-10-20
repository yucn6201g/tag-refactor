# インフラの仕様: U1 最小の一本

tag-refactor は利用者の端末で動く CLI で、サーバーやクラウドの上で動くものはありません。U1 のインフラは、CI(GitHub Actions)と、検証用 AWS アカウントの確認用のリソースと、手元からの接続です。

出典: `infrastructure-design-questions.md` Q1〜Q3、NFR の設計(`security-design.md`、`reliability-design.md`、`logical-components.md`)、チームの進め方(§Way of Working、§Testing Posture、§Deployment)。レビュー(`reviews/review-01.md`)の R-01〜R-06 を受けて改訂しました。

## Deployment

| Facet | Choice | Rationale |
|---|---|---|
| 実行の形 | 利用者の端末で動く1つの実行ファイル(`CGO_ENABLED=0`)。Linux、macOS、Windows | 要件 NFR9、NFR の設計 NFR9.2 |
| ネットワーク | 利用者の端末から AWS の API と、SDK の標準の認証情報の解決が使う宛先へ、HTTPS で出ていくだけ。入ってくる通信はない | NFR8.8 |
| 保存先 | なし(U1 はファイルを書かない。ルールファイルを読むだけ) | 機能設計 |
| 環境 | (1) CI(GitHub Actions。AWS を呼ばない)、(2) 手元の開発環境、(3) 個人の検証用 AWS アカウント(手元からだけ使う) | チームの進め方 §Testing Posture |
| IaC | 検証用アカウントの確認用のリソースだけを CloudFormation のテンプレートで定める(`test/sandbox/u1-fixtures.yaml`)。人がデプロイし、検証の後に削除する | Q1 |
| リージョン | 確認用のスタックは `us-east-1` に置き、U1 の検証も `--region us-east-1` で行う | 1つに固定して、手順と期待結果を一意にするため |
| 大きさと費用 | 費用のかからないリソースだけ(IAM のロールと管理ポリシー、空の S3 バケット、SQS のキュー)。AWS Config と Resource Explorer は U1 では使わない(U2 の E3) | Q1、少額の費用の範囲 |
| 確認用のバイナリ | `main` に取り込むたびに、3つの OS のバイナリを GitHub Actions の成果物として7日間だけ残す。公開しない | Q3、チームの進め方 §Deployment |

## Infrastructure Services

| Service | Role | Configuration | Notes |
|---|---|---|---|
| GitHub Actions | CI | `cicd-pipeline.md` のとおり | AWS の認証情報は置かない |
| GitHub のブランチの保護 | マージの関門 | `cicd-pipeline.md` の「関門」のとおり | 人が設定し、U1 のチェックポイントで確かめる |
| Dependabot | 依存の更新 | Go の依存(gomod、`tool` で管理する道具を含む)と GitHub Actions を週1回 | NFR8.7 |
| IAM Identity Center(SSO) | 手元から検証用アカウントへの接続 | 人の SSO のプロファイル(例 `tag-refactor-sandbox`)。一時的な認証情報だけを使う | Q2。長く使えるアクセスキーは作らない |
| CloudFormation(検証用アカウント) | 確認用のリソースのスタック | スタック名 `tag-refactor-u1-fixtures`、`us-east-1`。スタックに目印のタグ `project=tag-refactor-fixture` を付ける | 下の表 |

### 確認用のリソース(`test/sandbox/u1-fixtures.yaml`)

リソースの名前は、S3 のバケットを含めて、すべて CloudFormation の自動の名前にします(他人との衝突を避ける)。

| 記号 | リソース | 中身 | 確かめること |
|---|---|---|---|
| QA | SQS のキュー | タグ `env=dev` | 名前の変更 `env → Environment`(BR4.1) |
| QB | SQS のキュー | タグ `Env=dev` | 表記ゆれのまとまり(BR2.2)、`Env → Environment` |
| QC | SQS のキュー | タグ `env=dev` と `Environment=prod` | ぶつかり(BR4.2) |
| B1 | S3 のバケット(空、公開をすべて遮断、既定の暗号化) | タグ `env=dev`、`cost-center=a`、`cost_center=b` | 種類の違うリソースでの名前の変更と、区切り文字の表記ゆれ |
| P1 | IAM の管理ポリシー(R1 に付ける) | Allow `sqs:GetQueueAttributes`、Resource は QA と QB の ARN、条件 `StringEquals aws:ResourceTag/env = dev` | `!`(affected)の検出(BR5.3)。チェックポイントでの実際の評価(下の「確認2の確かめ方」) |
| P2 | IAM の管理ポリシー(R1 に付ける) | Allow `sqs:GetQueueAttributes`、Resource はこのアカウントの存在しないキューの ARN(`.../tag-refactor-nonexistent`)。条件に `ec2:ResourceTag/env = dev` と、キー側にポリシー変数を置いた条件を含む | サービス固有の条件キー(BR5.2)と `?`(undecided、BR5.4)。存在しないリソースへの読み取りだけなので、実害はない |
| P3 | IAM の管理ポリシー(どこにも付けない) | P1 と同じ文書 | 付いていないポリシーを読まないこと(BR5.1) |
| R1 | IAM のロール | P1・P2 を付ける。信頼の設定は下のとおり | ポリシーが「付いている」状態を作り、チェックポイントで P1 の実際の評価を確かめる |
| R2 | IAM のロール(読み取り用) | `docs/iam/readonly-policy.json` と同じ文書のインラインポリシーだけを付ける。信頼の設定は下のとおり | NFR3.2: 例のポリシーだけで scan と plan が動くこと |

- **信頼の設定(R1、R2):** Principal はこのアカウント(`arn:aws:iam::<アカウント>:root`)とし、条件 `ArnLike aws:PrincipalArn` で、人の SSO のロール(`arn:aws:iam::<アカウント>:role/aws-reserved/sso.amazonaws.com/*/AWSReservedSSO_<権限セット名>_*`)だけに絞る。権限セット名はテンプレートのパラメータ `SsoPermissionSetName` で渡す。アカウントの外からは引き受けられない。人の権限セットには `sts:AssumeRole` の許可が要る(管理者の権限セットなら含まれる)。
- **P2 の受理の確認(R-03):** キーの側にポリシー変数を置いた条件を IAM が受け付けるかは確かめられていない。デプロイの前に `cfn-lint` と、変更セットの作成で確かめ、受け付けられない場合は、その条件を P2 から外し、BR5.4 は単体テスト(差し替え)だけで確かめる。値の側に変数を置く形への置き換えはしない(BR5.4 の対象と違うため)。
- **IAM のリソースとタグの取得 API:** U1 の検証では、IAM のリソースが scan の結果に出ることを期待しない。scan の期待結果は SQS と S3 だけで決める。ポリシーの読み取りは IAM の API で行う。

### デプロイと削除の手順

1. `aws sts get-caller-identity --profile tag-refactor-sandbox` で、検証用アカウントの ID であることを確かめる(違えば中止)。
2. `cfn-lint test/sandbox/u1-fixtures.yaml` で検査する。
3. デプロイ: `aws cloudformation deploy --profile tag-refactor-sandbox --region us-east-1 --stack-name tag-refactor-u1-fixtures --template-file test/sandbox/u1-fixtures.yaml --capabilities CAPABILITY_IAM --parameter-overrides SsoPermissionSetName=<権限セット名> --tags project=tag-refactor-fixture`
4. タグが取得の API に出るまで、最大5分待つ(検証のスクリプトは、期待したリソースが見えるまで30秒ごとに最大10回確かめる)。
5. 検証(U1 のチェックポイント)が終わったら削除する: `aws cloudformation delete-stack --profile tag-refactor-sandbox --region us-east-1 --stack-name tag-refactor-u1-fixtures`。S3 のバケットは空のままなので、そのまま消える。
6. 削除の後、目印のタグ `project=tag-refactor-fixture` の付いたリソースが残っていないことを確かめる。

- **スタックの寿命:** U1 のスタックは、検証のたびに作って消す。後の作業単位(U2、U5〜U7)は、自分の確認用のスタックを別の名前で作り、U1 のスタックを使い回さない。
- **R2 を使うプロファイル:** 手元の AWS の設定に、SSO のプロファイルを元にして R2 を引き受けるプロファイルを足す(例: `[profile tag-refactor-readonly]` に `role_arn = <R2 の ARN>` と `source_profile = tag-refactor-sandbox`)。検証用アカウントのテストは、環境変数 `TAG_REFACTOR_IT_PROFILE=tag-refactor-readonly` で動かす。

### 期待結果(`--region us-east-1`、ルールファイル `test/sandbox/u1-rules.yaml`: `Environment` の別名 `env`、`Env`)

| コマンド | 期待する結果 |
|---|---|
| `scan` | 表記ゆれのまとまり `env`(書き方 `env` と `Env`)と `cost-center`(書き方 `cost-center` と `cost_center`)がある。`aws:cloudformation:*` のキーはまとまりに出ない。終了コード 2 |
| `plan` の変更 | QA: `env → Environment`。QB: `Env → Environment`。B1: `env → Environment` |
| `plan` のぶつかり | QC(`env=dev` と `Environment=prod`) |
| `plan` の影響 | P1 が `!`(`aws:ResourceTag/env`、原因は QA・QB・B1 の変更)。P2 が `!`(`ec2:ResourceTag/env`)と `?`(ポリシー変数。受理されなかった場合はなし)。P3 は出ない |
| `plan` の終了コード | 2 |
| R2 だけで実行 | scan と plan が権限のエラーなく動く |

検証用アカウントに、このスタックの外のリソースがある場合、件数は増えます。期待結果は、上の表のリソースが含まれることで判定します。

### 確認2の確かめ方(機能設計の仮定)

- `aws:ResourceTag/env` の条件が `Env` のタグにも当てはまるか: R1 を引き受け、QB(`Env=dev`)に `sqs:GetQueueAttributes` を呼ぶ。成功すれば、キーの大文字・小文字が区別されないことを確かめられる(P1 の Resource に QB を含めているため)。
- `ec2:ResourceTag/env` のようなサービス固有の条件キーは、U1 では実際に確かめない。機能設計 BR5.3 は、見逃さない側に倒した扱いのまま残す。

## Shared Infrastructure

| Shared Resource | Owner Unit | Consumer Units | Access Boundary |
|---|---|---|---|
| GitHub Actions のワークフロー | U1 | U2〜U8(各 Bolt のプルリクエスト) | ジョブを足すのは各 Bolt。集約のジョブ `ci-gate` の `needs` の変更は人が承認する |
| `docs/iam/readonly-policy.json` | U1 | U2、U5 が操作を足す | プルリクエストのテストで、読み取りの操作だけであること、確認用のテンプレートの R2 と一致することを確かめる |
