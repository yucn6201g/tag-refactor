# CI/CD と検証の設計: U6 書き換えと記録

U1〜U5 の CI/CD の流れ(`ci.yml` のジョブ、`ci-gate`、`race` ジョブ、関門、秘密の値、検証コマンド `scripts/verify.sh`)は、そのまま使います。U6 では、検証用アカウントに U6 の確認用のスタックを1つ足し、`verify.sh` に、書き換え用のロールで実際にタグを書き換えて戻す確かめを足します。

出典: NFR の設計(`security-design.md`、`logical-components.md`)、NFR 要件(NFR3.1、NFR3.10〜NFR3.12、NFR4.1、NFR5.12〜NFR5.17、NFR6.1、NFR6.2、NFR7.1、NFR8.23〜NFR8.25、NFR9.8、NFR9.9、NFR10.10、NFR11.8)、契約 C1(ルールファイル)・C2・C4・C5、`infrastructure-design-questions.md` Q1・Q2、チームの進め方(§Way of Working、§Testing Posture)。

## `ci.yml`

ジョブは足しません。U6 のテストは、今のジョブの中で動きます。

| 確かめ | 動くジョブ | 注 |
|---|---|---|
| U6 の受け入れ条件と単体テスト(偽の AWS) | `test`(3つの OS × 2つの版)、`coverage` | AWS の認証情報は要らない(NFR10.10) |
| 大きな偽の AWS のテスト(2つの分布。NFR5.13、NFR5.14、NFR5.16) | `test`、`race`(件数10分の1、時間の上限10倍) | 各 OS で合わせて2分以内 |
| 印の競争、壊れない保存、Windows の名前の変更のやり直し(NFR6.1、NFR7.1、NFR9.8、NFR9.9) | `test`(3つの OS) | Windows の振る舞いは windows-latest で確かめる |
| フォルダーとファイルの権限(NFR8.24) | `test`(ubuntu、macos) | Windows では作れることだけ |
| 書き換え用の権限の例と、コードの操作の表・種類の表の一致(NFR3.11) | `test` | U1 の `docs_test.go` を広げる。テンプレート `test/sandbox/u6-fixtures.yaml` の R3 も照らす |
| 読み取りのコマンドが書き込みを呼ばない(NFR3.1、NFR3.10) | `test` | U1 の確かめに、書き込みの操作を足す |
| S3 の記録の要求に ACL がなく、名前が前置きの下にある(NFR8.25) | `test` | 偽の S3 に届いた要求を調べる |
| U6 の確認用のテンプレート | `lint`(`cfn-lint test/sandbox/*-fixtures.yaml` に自然に入る) | |
| 表示の期待の出力(NFR11.8) | `test` | 英語と日本語、`NO_COLOR` |

## 検証用アカウントのリソース(Q1)

### 書き換えの対象

- **U5 のスタック `tag-refactor-u5-fixtures`(変えない):** バケット BU5、キュー QU5、トピック TU5、シークレット SU5、ロール RU5。
- **U6 のスタックのセキュリティグループ GU6**(EC2 の種類)。
- 書き換えるのは確かめ専用のキー `tag-refactor-u6-check` だけで、足して消す。U1〜U5 の確かめが使うタグ(`env`、`team` など)は変えない。
- ルールは、5つの種類(`AWS::S3::Bucket`、`AWS::SQS::Queue`、`AWS::SNS::Topic`、`AWS::SecretsManager::Secret`、`AWS::EC2::SecurityGroup`)に、確認用のスタックのタグ `project: tag-refactor-fixture` が付いているものだけに当たる(下のルールファイル)。そのため U1・U2・U6 の確認用のスタックの、同じ種類のリソース(U1 のキュー、U2 のキューなど)にも当たる。どれも R3 の種類の表にある種類なので、R3 で書き換えられる。書き換える前に `plan` で範囲を確かめる(下の手順 2)。 IAM のロール(RU5)は、チェックポイントの検証で `tag:GetResources` が IAM のロールを返さず scan で集まらないと分かったため、人の判断で対象から外した。R3 の IAM のタグの操作は偽の AWS のテストだけで確かめる。

### U6 の確認用のスタック `tag-refactor-u6-fixtures`(新規)

| 論理 ID | 種類 | 中身 | 費用 |
|---|---|---|---|
| VU6 | `AWS::EC2::VPC` | `10.66.0.0/16`。サブネット、インターネットゲートウェイ、NAT は作らない | なし |
| GU6 | `AWS::EC2::SecurityGroup` | VU6 の中。受け付ける規則なし | なし |
| BU6R | `AWS::S3::Bucket` | 記録を S3 に置く確かめ用。公開のブロック、既定の暗号化(SSE-S3)、所有者の強制(ACL を使わない)、バージョニングなし | 記録のオブジェクトの分だけ |
| BU6RPolicy | `AWS::S3::BucketPolicy` | TLS でない要求を拒む(BU5 と同じ) | なし |
| R3 | `AWS::IAM::Role` | 書き換え用のロール。信頼ポリシーは R2 と同じ(検証用アカウントの SSO のロールだけが引き受けられる)。インラインポリシーは `docs/iam/apply-policy.json` と同じ中身で、`<account-id>` は `${AWS::AccountId}`、`<record-bucket>` は BU6R、`<record-prefix>` は `runs` | なし |

出力: `ApplyRoleArn`(R3)、`SecurityGroupArn`(GU6 の ARN を `Fn::Sub` で作る)、`RecordBucket`(BU6R の名前)。パラメーターは U1・U5 と同じ `SsoPermissionSetName`。名前は CloudFormation が作ります。スタックのタグは `project=tag-refactor-fixture`。

- **R3 と例の一致:** U1 のテスト(`docs_test.go`)を広げ、テンプレートの R3 のポリシーが、例のファイルの置き換えと同じことを確かめます。
- **手元のプロファイル:** 人が、R3 を引き受けるプロファイル(既定の名前 `tag-refactor-apply`)を手元の AWS の設定に足します(R2 と同じ形)。README に手順を書きます。

### AWS への変更の手順(人の許可を得てから)

U5 と同じく、変更セットを作って中身を示し、人の許可を得てから実行します。

1. **前の確かめ(読み取りだけ):** R3 のインラインポリシーに IAM Access Analyzer の `ValidatePolicy`(アイデンティティポリシー)を、BU6RPolicy に `ValidatePolicy`(リソースポリシー、`AWS::S3::Bucket`)を当て、誤り(ERROR)がないことを示します。
2. **変更セットを作る:** `aws cloudformation create-change-set --stack-name tag-refactor-u6-fixtures --change-set-type CREATE --change-set-name u6-create --template-body file://test/sandbox/u6-fixtures.yaml --parameters ParameterKey=SsoPermissionSetName,ParameterValue=<U1 と同じ値> --capabilities CAPABILITY_IAM --tags Key=project,Value=tag-refactor-fixture --region ap-northeast-1 --profile tag-refactor-sandbox`
3. **中身を示す:** `describe-change-set` で、5つのリソースを作ることを示します。
4. **許可を得てから実行する:** `execute-change-set` と `wait stack-create-complete`。許可が得られなければ `delete-change-set`。
5. **後片付け:** Construction の終わりに、U1・U2・U5 と一緒に U6 のスタックを消します(BU6R は中の記録を消してから)。

## 検証のスクリプト(`scripts/verify.sh` に足す)

検証コマンド(`"C:\Program Files\Git\bin\bash.exe" -c "TAG_REFACTOR_SANDBOX_ACCOUNT_ID=663798383745 bash scripts/verify.sh"`)は変えません。

### 差し込む位置と順序

U6 の確かめは、すべての確かめの最後に置きます。U1〜U5 の確かめ(U2 の Resource Explorer の待ち、U2 の確かめを含む)が終わり、U2 のタグのないキューを消す前です。U6 は確かめ専用のキーを足して消すだけですが、U1〜U5 の確かめの結果に影響しないよう、それらのあとに動かします。U2 のタグのないキューは、手順 5 で使ってから U2 のとおり消します。

### 使うルールファイル(新規)

| ファイル | 中身 |
|---|---|
| `test/sandbox/u6-add-rules.yaml` | 必須タグ `tag-refactor-u6-check`。`default` は書かない。`when` を5つの種類ごとに1つずつ書き、それぞれ `match.resourceType` と `match.tags: {project: tag-refactor-fixture}` の両方で絞って値 `1` を付ける |
| `test/sandbox/u6-remove-rules.yaml` | `remove: [tag-refactor-u6-check]` |

`when` は、どのリソースにも高々1つしか当たらない(種類が違うため)。値が未指定の必須タグ(どの `when` にも当たらないリソース)は書き換えられず、`plan` と `apply` の終了コードを 2 にするだけなので、手順では終了コードではなく記録で合否を決める。

### 手順

1. **前の確かめ(管理用のプロファイル、読み取りだけ):** スタック `tag-refactor-u6-fixtures` の状態と出力、U5 のスタックの出力を読み、5つの対象の ARN(BU5、QU5、TU5、SU5、GU6)を求めます。`TAG_REFACTOR_APPLY_PROFILE`(既定 `tag-refactor-apply`)で `sts get-caller-identity` を呼び、R3 を引き受けられることを確かめます。手元の印(`<ホーム>/.tag-refactor/runs/<accountId>/.lock`)が残っていれば、中身を示して失敗にします(自動では外さない)。
2. **範囲の確かめ(読み取りだけ):** R3 のプロファイルで `plan --rules <ルールファイル> --format json --out <一時フォルダー>/plan.json --lang en`(`--region` を付けず、すべての有効なリージョン。S3 のバケットなどグローバルなリソースも集めるため)を動かします。変更案の JSON の `resources` のうち `changes` があるものについて、次を確かめ、1つでも外れれば何も書き換えずに失敗にします。
   - 変更は `tag-refactor-u6-check` の追加(add のとき)か削除(remove のとき)だけ。
   - 種類が5つのどれか。`beforeTags` に `project=tag-refactor-fixture` がある。
   - 5つの対象の ARN がすべて入っている。入っていなければ、そのリソースが収集に出なかったことを示して失敗にする。
   - 影響の指摘は、U1 の確認用のポリシー P2 の文 `PolicyVariableKey`(条件のキー `aws:ResourceTag/${aws:PrincipalTag/team}` にポリシー変数があるため、どのキーにも「決められない」(`undecided`)が付く)によるものだけで、影響分析が完全である。それ以外の指摘があれば、確認用のスタックのポリシーがこのキーを参照しているので、設計を見直す。(チェックポイントの検証で分かったため改めた。人の判断で、P2 の指摘だけを許す形にした)
3. **書き換えて戻す(5回):** 次をくり返します。すべて R3 のプロファイル、`--yes`、`--allow-impact`、`--debug`、`--lang en`。P2 の「決められない」は `--yes` だけでは止まるため、`--allow-impact` を付けます。想定外の影響は、手順 2 の確かめ(P2 以外の指摘があれば止める)で防ぎます。記録の場所は一時フォルダー(`--record-to`)で、3回目だけ `--record-to s3://<BU6R>/runs` にします。
   1. 手順 2 を add のルールで行い、`apply --plan <一時フォルダー>/plan.json`。
   2. 合否は記録で決めます: 記録の JSON があり、`completed: true`、`failed` と `pending` が0件、5つの対象の項目が `applied`、手順 2 の範囲の外の項目がない。終了コードは 0 か 2 を許しますが、記録が作られていない 2(影響で止まった、など)は失敗にし、標準エラーを示します。
   3. 手順 2 を remove のルールで行い、`apply --plan` で消します。合否は同じく記録で決めます(`skipped` も0件)。
4. **残っていないことの確かめ:** 管理用のプロファイルで、`tag:GetResources`(`TagFilters` に `tag-refactor-u6-check`)を ap-northeast-1 と us-east-1 で呼び、IAM のロール(RU5、R3、U1 のロール。apply の対象ではないが念のため)は `iam list-role-tags` でも確かめます。結果整合のため、見つかったら10秒おいて最大6回読み直し、それでも残れば場所を示して失敗にします。手元と S3 の印(`.lock`)が残っていないことも確かめます。
5. **一度もタグのないリソース(U2 のキュー):** U2 のタグのないキューを ARN で指定して `tag:GetResources` を呼び、答えに出たかを表示してチェックポイントの記録に残します(機能設計と NFR の設計の「一度もタグのないリソース」の扱い)。そのあと U2 のとおりキューを消します。apply での書き換えは行いません: `plan` はタグ付けの API で集めたリソースだけから変更案を作るため、一度もタグのないリソースは変更案に入りません(チェックポイントの検証で分かったため、人の判断で確かめだけにした。README に制限として書いた)。
6. **S3 の記録の確かめ:** 3回目の記録が `s3://<BU6R>/runs/<accountId>/<runId>.json` にあることを確かめます。`head-object` でサーバー側の暗号化が付いていること、`get-object-acl` で所有者以外の許可がないことを確かめます。R3 で、ないキー(`runs/<accountId>/no-such-run.json`)を読み、NotFound になることを確かめます(AccessDenied なら、例のポリシーの `s3:ListBucket` の条件を直す必要があることを示して失敗にする)。
7. **見積もり(NFR5.15):** 手順 3 の10回の `--debug` の行から、`tag:GetResources`、`tag:TagResources`、`tag:UntagResources` の L95 と、再試行が起きた呼び出しの数を求めます。NFR の要件の測る条件の2つの分布について、偏ったリージョン(2,000件)の回数を NFR5.14 の式で計算し(よくある変更: 最初の読み取り20、読み直しと読み戻し50、付ける105、消す105。値がばらばら: 最初の読み取り20、読み直しと読み戻し50、付ける2,000、消す2,000)、NFR5.15 の式で見積もって表示します。

### 後片付け(途中で失敗したとき)

- 最初の add の `apply` の直前に「キーを足したかもしれない」旗を立て、手順 4 で残っていないことを確かめたら下ろします。
- 既存の `cleanup`(`trap ... EXIT`)の先頭で、旗が立っていて、実行ファイル(`$tool`)がまだあるときだけ、remove のルールで `plan` と `apply --plan` を R3 で1回動かしてから、`$u3_dir` を消すなど既存の後片付けに進みます。旗が立っていなければ何もしません(U6 より前の失敗では動かない)。
- 後片付けの `apply` が印のために止まったら(Conflict)、印の中身を示し、「実行中でないことを確かめてから `--force-unlock` で外す」と案内します。自動では外しません。
- 後片付けでも消えなければ、残った場所(手順 4 の確かめ)を示して失敗にします。

### 測る値の記録(NFR5.15)

| 値 | 求め方 |
|---|---|
| L95(`tag:GetResources`、`tag:TagResources`、`tag:UntagResources`) | 手順 3 の10回の `--debug` の行 |
| 再試行が起きた呼び出しの数 | 同上 |
| よくある変更の見積もり、値がばらばらな場合の見積もり | NFR5.14・NFR5.15 の式 |
| 一度もタグのないキューが、ARN を指定した `tag:GetResources` の答えに出たか | 手順 5 |
| S3 のないキーが NotFound になったか | 手順 6 |

**見積もりの限界:** 確認用のスタックの書き換えは、1回の `TagResources` で数件しか書かず、`GetResources` も数件の ARN しか読みません。そのため、操作ごとの標本は10〜30と少なく、L95 は実質の最大になります。20件のまとまりや100件の ARN の読み取りの時間は、これより長いことがあります。チェックポイントでは、この限界を踏まえて30分の見積もりを判断し、記録に書きます(U5 の NFR1.19 と同じ扱い)。

## チェックポイントでの人の確認

- PowerShell で `apply`(確認画面、`yes` 以外での取り消し、結果の一覧)を `--lang ja` で一度動かし、表示を目で確かめます。対象は add のルールで、確認画面で `no` と入力して取り消します(何も書き換えない)。
- 30分の見積もりと、値がばらばらな場合の見積もりを、上の限界と一緒に記録に残し、承認します。

## 認証情報と費用

- 認証情報は表示しない(U2 と同じ)。R3 は検証用アカウントの SSO のロールだけが引き受けられ、書き込みの権限はタグの操作と記録の S3 だけ。
- 費用: VPC とセキュリティグループはなし。BU6R は記録のオブジェクトの分だけ。U5 のスタックのシークレット(月 0.40 USD)は U5 のまま。
