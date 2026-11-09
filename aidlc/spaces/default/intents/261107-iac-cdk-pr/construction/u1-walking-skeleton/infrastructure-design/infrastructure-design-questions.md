# インフラの設計の質問: U1 最小の一本

tag-refactor は利用者の端末で動く CLI なので、サーバー、データベース、負荷分散などの本番のインフラはありません。U1 のインフラは次の3つです。

- CI(GitHub Actions)
- 検証用の AWS アカウントに置く最小の CDK アプリのスタック
- テスト専用の GitHub のリポジトリ

U1 の非機能の設計は `construction/u1-walking-skeleton/nfr-design/`、機能の設計は `construction/u1-walking-skeleton/functional-design/` にあります。

## Q1. 検証用のスタックの置き方

検証用のアカウント(663798383745、ap-northeast-1)に、最小の CDK アプリをスタック `tag-refactor-cdk-skeleton` として置きます(機能の設計の FD-Q5)。CDK の `cdk deploy` を使うには、アカウントにブートストラップ(`CDKToolkit` のスタック。S3 のバケット、ECR のリポジトリ、IAM のロールを作る)が要ります。

A. ブートストラップはしない。`cdk synth` で作った CloudFormation のテンプレートを、プロファイル `tag-refactor-apply` の `aws cloudformation deploy` で、スタック名 `tag-refactor-cdk-skeleton` として置く。最小のアプリはアセット(Lambda のコードなど)を持たないので、テンプレートだけで置ける。スタック名と論理 ID は synth の結果と同じなので、`iac` の照合はそのまま通る。置く前に、テンプレートと変更の中身を見せて、人の明示の許可を得る
B. `cdk bootstrap` をしてから `cdk deploy` で置く(CDKToolkit のスタックが増える)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q2. 検証用のバケットの設定

A. S3 のバケット1つ。名前は CloudFormation に付けさせる。パブリックアクセスはすべて止め、既定の暗号化(SSE-S3)、バージョニングはしない。中身を入れない。スタックを消すときにバケットも消す(DeletionPolicy: Delete。空なので消せる)。タグは `Tags.of(bucket).add('Env', 'dev')` の1つだけ。費用はほぼ 0(空のバケット)
B. ほかの設定にする(Other に書く)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q3. テスト専用のリポジトリ

A. 人が `yucn6201g/tag-refactor-pr-sandbox` を private で作る。`main` には、テスト用のアプリ(`internal/cdk/testdata/apps/ts-minimal` と同じもの)を置く。tag-refactor のリポジトリには、変数 `TAG_REFACTOR_TEST_REPO`(値は `yucn6201g/tag-refactor-pr-sandbox`)と secret `TAG_REFACTOR_TEST_REPO_TOKEN`(このリポジトリだけに書ける fine-grained トークン)を置く。どちらも人が設定する
B. 名前や公開の範囲を変える(Other に書く)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q4. CI の仕事の細部

非機能の設計のレビューで、仕事の上限、権限、Prettier の版が決まっていないと指摘されました。

A. 次のとおりにする。Prettier の版は、Code Generation のときの最新の安定版に固定する
- 上限(timeout-minutes): test、race、coverage は 30 分、prettier は 10 分、github-pr は 20 分
- github-pr: checkout(`persist-credentials: false`)、setup-go、setup-node、npm ci を置き、権限は `contents: read` だけにする(テスト専用のリポジトリへの書き込みは secret のトークンで行う)
- prettier: 権限は `contents: read` だけ
B. ほかの値にする(Other に書く)
C. 任せる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- 検証用のスタックは、ブートストラップをせず、`cdk synth` のテンプレートをプロファイル `tag-refactor-apply` の `aws cloudformation deploy` でスタック `tag-refactor-cdk-skeleton` として置く。置く前に中身を見せて人の明示の許可を得る(Q1: A)
- 検証用のバケットは1つ。名前は自動、パブリックアクセスはすべて止め、既定の暗号化、中身なし、スタックと一緒に消える。タグは Env=dev の1つ(Q2: A)
- テスト専用のリポジトリは、人が `yucn6201g/tag-refactor-pr-sandbox` を private で作り、`main` にテスト用のアプリを置く。変数 `TAG_REFACTOR_TEST_REPO` と secret `TAG_REFACTOR_TEST_REPO_TOKEN` は人が設定する(Q3: A)
- CI の上限は test、race、coverage が 30 分、prettier が 10 分、github-pr が 20 分。github-pr と prettier の権限は `contents: read` だけ。Prettier の版は Code Generation のときの最新の安定版に固定する(Q4: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
