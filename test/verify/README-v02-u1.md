# v0.2.0 U1(最小の一本)の検証

intent `261107-iac-cdk-pr` の U1 の Construction 検証コマンドの手順です。実際の AWS と実際の GitHub を使うので、手元だけで行います(CI では行いません)。

検証では、検証用のアカウントに置いた最小の CDK アプリのスタックから `plan` で変更案を作り、その変更案でテスト専用のリポジトリに `iac --pr` を動かして、キーの1か所だけを変えるプルリクエストができることを確かめます。

## 前提

| 要るもの | 説明 |
|---|---|
| 検証用の AWS アカウント | `ap-northeast-1`。読み取り用のプロファイル(既定 `tag-refactor-readonly`)と、スタックを置くための管理者のプロファイル `tag-refactor-sandbox`。切れていたら人が `aws sso login` を実行する |
| テスト専用のリポジトリ | 例 `yucn6201g/tag-refactor-pr-sandbox`(private)。人が作る。`main` にテスト用のアプリを置く(下の「アプリを写す」) |
| 手元の道具 | Go、git、`gh`(テスト専用のリポジトリに書ける権限でログイン済み)、Node.js 18 以上、npm、AWS CLI |
| 費用 | 空の S3 のバケット1つ。ほぼ 0 |

トークンなどの秘密の値は、このリポジトリにもテスト専用のリポジトリにも書きません。CI の結合テストのトークンは、tag-refactor のリポジトリの Actions の secret `TAG_REFACTOR_TEST_REPO_TOKEN` に人が置き、リポジトリの変数 `TAG_REFACTOR_TEST_REPO` にテスト専用のリポジトリの名前を置きます。

## テスト専用のリポジトリにアプリを写す

テスト専用のリポジトリのアプリは、このリポジトリの `internal/cdk/testdata/apps/ts-minimal` の写しで、リポジトリの根に置きます。

1. テスト専用のリポジトリを手元に clone する。
2. `internal/cdk/testdata/apps/ts-minimal` の中のファイルを、`node_modules` と `cdk.out` を除いて clone の根に写す(`package.json`、`package-lock.json`、`cdk.json`、`tsconfig.json`、`bin/app.ts`、`lib/stack.ts`)。
3. clone の根に `.gitignore` を置き、`node_modules` と `cdk.out` を書く。
4. コミットして `main` に push する。

`ts-minimal` の依存を Dependabot で更新したら、同じ手順で写し直します。

## 検証用のスタックを置く

スタック `tag-refactor-cdk-skeleton` は、テスト用のアプリの `cdk synth` のテンプレートを `aws cloudformation deploy` で置いたものです。ブートストラップはしません(アプリは `generateBootstrapVersionRule: false` で、テンプレートにブートストラップの版のパラメータとルールがありません)。

**置く前に、テンプレートと命令を人に見せ、明示の許可を得ます。** 変更セットの作成も AWS の変更なので、許可の前には作りません。

1. テンプレートを作る。AWS の認証情報は要らないので、渡さない。

   ```bash
   cd internal/cdk/testdata/apps/ts-minimal
   npm ci --ignore-scripts
   out="$(mktemp -d)"; empty="$out/empty"; : > "$empty"
   env -u AWS_PROFILE -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN \
     AWS_CONFIG_FILE="$empty" AWS_SHARED_CREDENTIALS_FILE="$empty" AWS_EC2_METADATA_DISABLED=true \
     CDK_DISABLE_VERSION_CHECK=1 CDK_DISABLE_CLI_TELEMETRY=true \
     node_modules/.bin/cdk synth --quiet --output "$out/cdk.out"
   ```

2. `$out/cdk.out/tag-refactor-cdk-skeleton.template.json` を人に見せる。中身は S3 のバケット `Bucket83908E77`(パブリックアクセスはすべて止め、SSE-S3、`DeletionPolicy: Delete`、タグ `Env=dev`)と、CDK の `CDKMetadata` だけです。
3. 許可を得たら置く。

   ```bash
   aws cloudformation deploy --stack-name tag-refactor-cdk-skeleton \
     --template-file "$out/cdk.out/tag-refactor-cdk-skeleton.template.json" \
     --profile tag-refactor-sandbox --region ap-northeast-1
   ```

4. 置いたあと、`describe-stacks` と、バケットのタグ(`Env=dev` と `aws:cloudformation:stack-name`、`aws:cloudformation:logical-id`)を読んで確かめる。

   ```bash
   aws cloudformation describe-stacks --stack-name tag-refactor-cdk-skeleton --profile tag-refactor-sandbox --region ap-northeast-1
   bucket="$(aws cloudformation describe-stack-resource --stack-name tag-refactor-cdk-skeleton --logical-resource-id Bucket83908E77 \
     --profile tag-refactor-sandbox --region ap-northeast-1 --query StackResourceDetail.PhysicalResourceId --output text)"
   aws s3api get-bucket-tagging --bucket "$bucket" --profile tag-refactor-sandbox --region ap-northeast-1
   ```

## 検証コマンド

```bash
TAG_REFACTOR_SANDBOX_ACCOUNT_ID=<検証用アカウントの ID> \
TAG_REFACTOR_TEST_REPO=<owner/name> \
bash test/verify/v02-u1.sh
```

`test/verify/v02-u1.sh` は次を行います。AWS には書き込みません。

1. 読み取り用のプロファイルの呼び出し元が検証用のアカウントであることと、スタック `tag-refactor-cdk-skeleton` があることを確かめる。読み取り用のプロファイルはタグを読むことしか許されていない(`cloudformation:DescribeStacks` は無い)ので、スタックは、バケットに付いた `aws:cloudformation:stack-name` のタグをタグ付けの API で探して確かめる。
2. `tag-refactor` をビルドし、`scan` と、`Env` を `Environment` に変える規則(`test/verify/v02-u1-rules.yaml`)で `plan --out` を行う。変更案に、そのスタックのバケットの名前の変更がちょうど1つあることを確かめる(`test/verify/v02-u1-plan.js`)。検証用のアカウントには、ほかの確認用のスタックにも `Env` を持つリソースがあるため、`iac` が手で直す一覧を出して終了コード 2 で終わることは失敗にしません。
3. テスト専用のリポジトリを一時的なフォルダーに clone し、`npm ci` のあとで `iac --pr` を動かす。`cdk synth` には AWS と GitHub の認証情報が渡りません。
4. プルリクエストができ、その差分が `'Env'` を `'Environment'` に変える1行だけであることを確かめる。最後にプルリクエストを閉じてブランチを消す。

| 環境変数 | 意味 |
|---|---|
| `TAG_REFACTOR_SANDBOX_ACCOUNT_ID` | 検証用アカウントの ID(必須) |
| `TAG_REFACTOR_TEST_REPO` | テスト専用のリポジトリ(`owner/name`、必須) |
| `TAG_REFACTOR_IT_PROFILE` | 読み取り用のプロファイル(既定 `tag-refactor-readonly`) |
| `TAG_REFACTOR_KEEP_PR` | `1` なら、人が見るためにプルリクエストを閉じずに残す(見たあとで人が閉じ、ブランチを消す) |

## 後片付け

`v0.2.0` を出したあとで、消すものを人に見せ、許可を得てから消します。

```bash
aws cloudformation delete-stack --stack-name tag-refactor-cdk-skeleton --profile tag-refactor-sandbox --region ap-northeast-1
```

テスト専用のリポジトリに検証や結合テストのブランチが残っていたら、プルリクエストを閉じてブランチを消します(結合テストは、作ってから1日を超えた `tag-refactor/iac-` のブランチを始めに片付けます)。
