# CI/CD の流れ: U5 影響分析

U1〜U4 の CI/CD の流れ(`ci.yml` のジョブ、`ci-gate`、`race` ジョブ、関門、秘密の値、検証コマンド `scripts/verify.sh`)は、そのまま使います。U5 では、検証用アカウントにテスト用のスタックを1つ足し、U1 の確認用のスタックの読み取り用ロール R2 に権限を足します。公開(GitHub Releases)は行いません。

出典: NFR の設計(`security-design.md`、`logical-components.md`)、NFR 要件(NFR1.16〜NFR1.19、NFR2.1、NFR2.2、NFR3.9、NFR5.10、NFR5.11、NFR8.20〜NFR8.22、NFR10.9)、`infrastructure-design-questions.md` Q1、チームの進め方(§Way of Working、§Testing Posture)。

## `ci.yml`

ジョブの設定の変更はありません。U5 で足すテストは、今のジョブの中で動きます。CI からは実際の AWS を呼びません。

| 足すテスト | 動くジョブ | 補足 |
|---|---|---|
| 検出率100%の期待する判定の表(252行と32行、行の数の照合。NFR2.1) | `test`(3つの OS × 2つの版)、`race`、`coverage` | 偽の読み取りの口で動く |
| 文書の大きさの境目(1 MiB + 1 バイトは読めなかったもので終了コード1、ほかのポリシーの指摘は出る。ちょうど 1 MiB は読まれる。NFR8.20) | 同上 | IAM は URL エンコードを戻す前、リソースポリシーは返ったままのバイト数で数える |
| 文書の読み取りとファジングの種の入力(`FuzzParsePolicy`。NFR8.21) | 同上 | `go test` は種の入力を普通のテストとして動かす。CI ではファジングそのもの(`-fuzz`)は動かさない。種は `internal/impact/testdata/fuzz/FuzzParsePolicy/` に置き、今の `**/testdata/fuzz/** -text` で守られる |
| 大きな偽の AWS(呼び出しの回数、ツール自身の処理の時間10秒、同時の数の上限、枠を1にしても終わる。NFR1.16〜NFR1.18) | `test`、`race` | `race` のときは U2〜U4 と同じ `race` のビルドタグで件数を10分の1にし、上限を10倍にする |
| 同じ結果(乱数の種を変えた10回の実行。NFR5.11) | `test`、`race` | `race` のジョブ(Linux、cgo を有効にしたもの)で `-race` 付きでも動く。ワイルドカードの使い回しの入れ物の競合を見つけるため |
| 失敗を集める、認証情報の誤りで止める、取り消したあと goroutine が残らない(NFR5.10) | `test`、`race` | 偽の AWS の応答を止める差し替えを使う |
| 足した操作が操作の表と例のポリシーと R2 で一致する、`Resource` の形の検査(NFR3.9) | `test` | U1 の `docs_test.go` を広げる。テンプレート `test/sandbox/u1-fixtures.yaml` の R2 も照らす |
| `--debug` と読めなかった理由に文書の中身が出ない、出力の守り(NFR8.22) | `test` | U4 の出力の守りのテストに、影響の指摘の入力を足す |
| U5 の確認用のテンプレートの検査 | `lint` | 今の `cfn-lint test/sandbox/*-fixtures.yaml` が、新しい `u5-fixtures.yaml` も自動で検査する。ポリシー変数は `Fn::Sub` の中で `${!aws:PrincipalTag/team}` と書き、cfn-lint の警告で CI を落とさない |

- U5 の大きなテストは、CI の各 OS で合わせて2分以内に収める。カバレッジの下限80%は変えない(NFR10.9)。
- 足す AWS の SDK のモジュール(`service/s3`、`service/kms`、`service/sqs`、`service/sns`、`service/secretsmanager`)は、今の `govulncheck` のジョブと Dependabot(`gomod`)の対象に入る。

## 検証用アカウントのリソース(Q1)

### U5 の確認用のスタック `tag-refactor-u5-fixtures`

テンプレートは `test/sandbox/u5-fixtures.yaml` です。U1・U2 と同じく、名前は CloudFormation に作らせ、`ap-northeast-1` に置き、スタックに `project=tag-refactor-fixture` のタグを付けます。組み込みの関数は長い形(`Fn::GetAtt`、`Fn::Sub`、`Ref`)で書きます。パラメーターは U1 と同じ `SsoPermissionSetName` だけです。

| 論理 ID | CloudFormation の種類 | 内容 | 依存 |
|---|---|---|---|
| RU5 | `AWS::IAM::Role` | タグ `env=dev`、`team=data`。信頼ポリシーは U1 の R1 と同じ(検証用アカウントの SSO のロールだけ)。インラインポリシー `u5-inline`(下) | BU5、QU5 |
| BU5 | `AWS::S3::Bucket` | タグ `env=dev`。公開を止め、AES256 で暗号化する(U1 の B1 と同じ) | — |
| BU5Policy | `AWS::S3::BucketPolicy` | BU5 のバケットポリシー(下) | BU5 |
| KU5 | `AWS::KMS::Key` | タグ `env=dev`。キーポリシー(下)を `KeyPolicy` に直接書く。`PendingWindowInDays: 7` | — |
| QU5 | `AWS::SQS::Queue` | タグ `env=dev`。`SqsManagedSseEnabled: true` | — |
| QU5Policy | `AWS::SQS::QueuePolicy` | QU5 のキューポリシー(下) | QU5 |
| TU5 | `AWS::SNS::Topic` | タグ `env=dev` | — |
| TU5Policy | `AWS::SNS::TopicPolicy` | TU5 のトピックポリシー(下) | TU5 |
| SU5 | `AWS::SecretsManager::Secret` | タグ `env=dev`。値は `GenerateSecretString` で作らせ、誰も読まない | — |
| SU5Policy | `AWS::SecretsManager::ResourcePolicy` | SU5 のリソースポリシー(下)。`BlockPublicPolicy: true` | SU5 |

| ポリシー | ステートメント |
|---|---|
| RU5 の `u5-inline` | (1) QU5 の `sqs:GetQueueAttributes` を、`StringEquals aws:ResourceTag/env: dev` のときだけ許す。(2) `Fn::Sub` で `arn:${AWS::Partition}:s3:::${BU5}/${!aws:PrincipalTag/team}/*` を作り、その `s3:GetObject` を許す(`${!...}` は CloudFormation に置き換えさせず、`${aws:PrincipalTag/team}` のまま IAM に渡す書き方) |
| BU5Policy | Principal `*` の `s3:*` を、`Bool aws:SecureTransport: false` と `StringEquals aws:ResourceTag/env: dev` の両方に当てはまるときだけ拒否する(拒否は暗号化しない通信だけなので、利用者の操作を止めない) |
| KU5 のキーポリシー | (1) アカウントの root に `kms:*` を許す標準のステートメント(これがないとキーを管理できなくなる)。(2) アカウントの root に `kms:DescribeKey` を `StringEquals aws:ResourceTag/env: dev` のときだけ許す |
| QU5Policy | アカウントの root に `sqs:GetQueueAttributes` を `StringEquals aws:ResourceTag/env: dev` のときだけ許す |
| TU5Policy | アカウントの root に `sns:GetTopicAttributes` を `StringEquals aws:ResourceTag/env: dev` のときだけ許す |
| SU5Policy | アカウントの root に `secretsmanager:DescribeSecret` を `StringEquals aws:ResourceTag/env: dev` のときだけ許す |

- **条件キーを受け付けるかの確かめ:** `aws:ResourceTag/<キー>` は AWS のグローバルな条件キーで、ポリシーの文法としてはどのポリシーにも書けます。ただ、サービスがポリシーを保存するときに知らない条件キーを拒む場合があるため、作る前に2つ確かめます。(1) CI の `cfn-lint`。(2) 作成の前に、管理用のプロファイルで IAM Access Analyzer の `ValidatePolicy`(読み取りの操作。ポリシーの種類はリソースポリシー、サービスは各サービス)を6つのポリシーに当て、誤り(ERROR)がないこと。変更セットの作成と実行のどこかでサービスが拒んだときは、そのリソースとポリシーだけをテンプレートから外し、その種類は偽の AWS のテスト(NFR2.1)だけで確かめることにして、チェックポイントの記録に残します。
- 出力(Outputs)のキー名は `RoleArn`、`BucketArn`(`arn:aws:s3:::<名前>`)、`KeyArn`、`QueueArn`、`TopicArn`、`SecretArn` です。
- **費用:** KMS のキーは月 1 USD、シークレットは月 0.40 USD で、どちらも時間で割った分がかかります。数日なら数十円です。ほかは無料の範囲です。
- **後片付け:** Construction の終わりに、U1・U2 の確認用のスタックと一緒に消します。バケットは空のままにして、スタックの削除で消えるようにします。KMS のキーは7日の待ち期間のあとに消えます。シークレットはスタックの削除で消える(削除の待ち期間は CloudFormation の既定)ことを、削除のときに確かめます。

### U1 の確認用のスタックの R2

- `docs/iam/readonly-policy.json` に、NFR の設計の `security-design.md` §1 の操作と `Resource` の範囲を足します。一覧の操作は今の `"*"` の文に、1つのリソースを読む操作は、サービスごとの ARN の形に絞った文に入れます。
- `test/sandbox/u1-fixtures.yaml` の R2 のインラインポリシーを、同じ内容にします(`<account-id>` は `Fn::Sub` の `${AWS::AccountId}`)。U1 のテスト(`docs_test.go`)が、例のファイルとテンプレートの R2 が同じことを確かめます。R2 の変更はポリシーの書き換えだけで、ロールは作り直されません。
- 権限の上限として使われる管理ポリシー、ユーザー、グループのリソースはスタックに足しません(偽の AWS のテストで確かめます)。
- **R2 の2回目の更新(2026-10-28):** U5 のチェックポイントで IAM の読み方を `GetAccountAuthorizationDetails` に改めたため(`security-design.md` §1)、R2 から使わなくなった IAM の操作(`iam:ListPolicies`、`iam:GetPolicyVersion`、`iam:ListUsers`、`iam:ListRoles`、`iam:ListGroups`、`iam:List*Policies`、`iam:Get*Policy`)を外し、`iam:GetAccountAuthorizationDetails` を `"*"` の文に足します。手順は下と同じ(変更セットを作って中身を示し、人の許可を得てから実行。変更セットの名前は `u5-gaad`)。R2 の `Policies` だけが変わり、`Replacement` は `False` のはずです。
- **U5 のスタックの改訂(2026-10-28):** 変更セットの実行で、KMS がキーポリシーの `aws:ResourceTag` を、SQS がキューポリシーの `aws:ResourceTag` を拒んだため、下の取り決めどおり KU5 と QU5Policy をテンプレートから外しました(QU5 は RU5 が読むので残す)。スタックのリソースは8個、出力は `RoleArn`、`BucketArn`、`TopicArn`、`SecretArn` の4つです。キーポリシーとキューポリシーは偽の AWS のテスト(NFR2.1)だけで確かめます。

### AWS への変更の手順(人の許可を得てから)

コード生成でテンプレートを書いたあと、管理用のプロファイル(`tag-refactor-sandbox`)で、まず変更セットを作るだけにして(実行しない)、その中身を人に示し、許可を得てから実行します。人が許可するのは、示した変更セットの中身と、次の実行のコマンドです。

1. **前の確かめ(読み取りだけ):** 6つのポリシーに IAM Access Analyzer の `ValidatePolicy` を当て、誤りがないことを示す。
2. **U1 のスタックの変更セットを作る:** `aws cloudformation create-change-set --stack-name tag-refactor-u1-fixtures --change-set-name u5-r2 --template-body file://test/sandbox/u1-fixtures.yaml --parameters ParameterKey=SsoPermissionSetName,UsePreviousValue=true --capabilities CAPABILITY_IAM --tags Key=project,Value=tag-refactor-fixture --region ap-northeast-1 --profile tag-refactor-sandbox`
3. **U5 のスタックの変更セットを作る:** `aws cloudformation create-change-set --stack-name tag-refactor-u5-fixtures --change-set-type CREATE --change-set-name u5-create --template-body file://test/sandbox/u5-fixtures.yaml --parameters ParameterKey=SsoPermissionSetName,ParameterValue=<U1 と同じ値> --capabilities CAPABILITY_IAM --tags Key=project,Value=tag-refactor-fixture --region ap-northeast-1 --profile tag-refactor-sandbox`
4. **中身を示す:** `aws cloudformation describe-change-set` で、U1 は R2 の `Policies` だけが変わり、`Replacement` が `False` であること、U5 は上の表の10個のリソースを作ることを示す。
5. **許可を得てから実行する:** `aws cloudformation execute-change-set` を2つに対して行い、`aws cloudformation wait stack-update-complete`(U1)と `wait stack-create-complete`(U5)で終わりを待つ。
6. 許可が得られなければ、`delete-change-set` で変更セットを消し、何も変えない。

## 検証のスクリプト

`scripts/verify.sh` に、U5 の確認を足します。検証コマンド(`"C:\Program Files\Git\bin\bash.exe" -c "TAG_REFACTOR_SANDBOX_ACCOUNT_ID=663798383745 bash scripts/verify.sh"`)は変えません。

- **差し込む位置:** U4 の確認のすぐあと、U2 のタグのないキューを作る前に置く。U2 の最大30分の待ちに巻き込まれないようにする。
- **前の確かめ(管理用のプロファイル、読み取りだけ):**
  1. スタック `tag-refactor-u5-fixtures` の状態が `CREATE_COMPLETE` か `UPDATE_COMPLETE` であることを確かめ、出力の6つの ARN を読む。なければ「`test/sandbox/u5-fixtures.yaml` を置く(`cicd-pipeline.md` の手順)」の案内で失敗にする。
  2. U1 のスタックの R2 のインラインポリシーに `iam:GetAccountAuthorizationDetails` が入っていることを、`aws iam get-role-policy` で確かめる(はじめは `iam:GetRolePolicy` を確かめていたが、IAM の読み方を改めたため置き換えた。2026-10-28)。なければ「U1 の確認用のスタックの R2 を更新する」の案内で失敗にする。
- **動かし方:** U2〜U4 と同じく `run_go_test` で `go test -mod=readonly -count=1 -tags integration -run '^TestIntegrationU5' -v ./internal/cli/...` を動かす。テストが飛ばされたら失敗にし、`--- PASS: TestIntegrationU5ImpactSandbox (` で始まる行がちょうど1つあることを確かめる(U4 と同じ)。テストに渡す環境変数は次のとおり。

| 環境変数 | 値 |
|---|---|
| `TAG_REFACTOR_IT_PROFILE` | 読み取り用のプロファイル(`tag-refactor-readonly`。R2)。U4 と同じ |
| `TAG_REFACTOR_SANDBOX_ACCOUNT_ID` | 検証用アカウントの ID。U4 と同じ |
| `TAG_REFACTOR_IT_U5_ROLE_ARN` | 出力 `RoleArn` |
| `TAG_REFACTOR_IT_U5_BUCKET_ARN` | 出力 `BucketArn` |
| `TAG_REFACTOR_IT_U5_KEY_ARN` | 出力 `KeyArn` |
| `TAG_REFACTOR_IT_U5_QUEUE_ARN` | 出力 `QueueArn` |
| `TAG_REFACTOR_IT_U5_TOPIC_ARN` | 出力 `TopicArn` |
| `TAG_REFACTOR_IT_U5_SECRET_ARN` | 出力 `SecretArn` |

- **ルールファイル `test/sandbox/u5-rules.yaml`:** `Environment` の別名に `env`、`Team` の別名に `team` を置く(キー名の変更を2つ起こす)。
- **結合テスト `TestIntegrationU5ImpactSandbox`(`internal/cli/integration_u5_test.go`):**
  1. 言語を固定する(`--lang en`、`LC_ALL=C`、`LANG=C`。U4 と同じ)。
  2. `plan --profile <TAG_REFACTOR_IT_PROFILE> --region ap-northeast-1 --rules test/sandbox/u5-rules.yaml --lang en --format json` を動かす(`cli.Run` をテストの中で呼ぶ)。必ず読み取り用のプロファイルで動かし、R2 の権限の不足を見えるようにする。
  3. `accountId` が検証用アカウントで、終了コードが 2 であることを確かめる。
  4. `impactAnalysisComplete` が true であることを確かめる。false のときは、読めなかったものを1つずつ出し、policyRef がスタックの6つの ARN のどれかか、スタックの外か、一覧(policyRef が空)かを分けて表示してから失敗にする(スタックの外だけなら「アカウントの、スタックの外のポリシーを R2 で読めない。R2 の権限かアカウントの状態を確かめる」と出す)。このテストは、検証用アカウントのすべてのポリシーが R2 で読めることを前提にする(検証用アカウントには、確認用のスタックのほかに、利用者が作ったリソースポリシーを置かない)。
  5. 指摘に、次がすべてあることを確かめる: RU5 の `iamInline`(`<RoleArn>#u5-inline`)の `aws:ResourceTag/env` の affected(QU5 などの `env` の名前の変更が原因)。BU5、KU5、QU5、TU5、SU5 のそれぞれの sourceKind の、policyRef がその ARN で、`aws:ResourceTag/env` の affected。U1 のスタックの P1 の `iamManaged` の affected。
  6. 5つのリソースポリシーの指摘の `changeIds` が、それぞれのリソース自身の変更だけを指すことを確かめる(機能設計 BR3.2)。
  7. **RU5 の変更と `Resource` の指摘:** IAM のロールはリージョンを持たないため、AWS のタグの API などが `--region ap-northeast-1` の収集で RU5 を返すかは、まだ確かめていません(検証用アカウントでの読み取りの確認は、サインインの期限切れでできていない)。そこで、変更案に RU5 の `team` の変更があるときだけ、RU5 の `Resource` の指摘(`aws:PrincipalTag/team` が原因)が affected であることを確かめます。無いときは「RU5 は収集に出なかった。PrincipalTag と Resource の照合は偽の AWS のテストで確かめる」をテストのログに出し、この項目だけを確かめずに続けます。どちらだったかは、チェックポイントの記録に残します。`PrincipalTag` と `Resource` の照合そのものは、NFR2.1 の偽の AWS のテストが必ず確かめます。
- **時間の見込み:** 1つのリージョンの収集と、IAM と5つのサービスの読み取りで、1回30秒〜1分ほど。

### 測る値の記録(NFR1.19)

- `verify.sh` が、結合テストのあとに、読み取り用のプロファイルで同じ plan を `--debug` 付きで5回動かし、標準エラーの `--debug` の行(API の名前、リージョン、時間、再試行の回数。U1 の NFR11.3 の形)を集める。検証用アカウントはリソースが少ないため、1回の plan では KMS、SNS、Secrets Manager の操作が数回しか呼ばれず、5回動かして標本を増やす。
- 操作ごとに、標本の数と L95(標本を時間の順に並べた95パーセンタイル。標本が20未満のときは最大値と同じになることを、出力に「標本が少ない」と書く)を出す。続けて、IAM の操作の L95 の最大を `L95(IAM)`、5つのサービスの操作の L95 の最大を `L95(リソースポリシー)` として1行ずつ出す。再試行の回数が1以上の行の数を「呼び出し回数の上限に当たった回数」として出す。
- 見積もりの式(NFR の設計の `logical-components.md` の「10分の見積もり」)への当てはめは、チェックポイントで人(と AI)が行い、記録に残す。`verify.sh` は値を出すだけで、10分以内かの判定はしない。
- 認証情報は表示しない(U2 と同じ)。シークレットの値は読まない(`secretsmanager:GetSecretValue` は R2 にも検証のスクリプトにもない)。

## チェックポイントでの人の確認

- 上の値から、測る条件(偏った分布)と、すべてが1つのリージョンにある場合の見積もりを出し、10分以内であることを記録する(NFR1.19)。
- 手元で `go test -run '^$' -fuzz FuzzParsePolicy -fuzztime 5m ./internal/impact/` を動かし、結果をチェックポイントの記録に残す(NFR8.21)。Windows の Application Control が新しく作られたテストの実行ファイルをふさいだときは、同じコマンドをもう一度動かす(U3・U4 と同じ扱い)。失敗する入力が見つかったら、Go が書いた入力を種としてコミットし、直してから同じ5分をもう一度動かす。
- 結合テストで、RU5 の変更が収集に出たかどうかを記録する。
- 手元の PowerShell で `plan --rules test/sandbox/u5-rules.yaml --region ap-northeast-1 --lang ja` を動かし、影響の指摘(インラインポリシーの `#` のある名前)と、読めなかったものの行が読める形で出ることを確かめる。
