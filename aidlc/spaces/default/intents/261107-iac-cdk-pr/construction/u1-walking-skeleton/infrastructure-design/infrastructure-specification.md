# インフラの仕様: U1 最小の一本

この文書は、U1 で使うインフラを決めます。元にしたものは次のとおりです。出典の ID-Q… は、このステージの `infrastructure-design-questions.md` の回答です。

- 非機能の設計: `construction/u1-walking-skeleton/nfr-design/`
- 機能の設計: `construction/u1-walking-skeleton/functional-design/`
- 部品: `inception/domain-design/components.md`
- 契約: `inception/contract-design/contract-summary.md`

tag-refactor は利用者の端末で動く CLI で、本番の配置はありません。U1 のインフラは次の3つです。

- CI(GitHub Actions)
- 検証用の AWS アカウントの最小の CDK アプリのスタック
- テスト専用の GitHub のリポジトリ

## 配置

| 面 | 選んだもの | 理由 |
|---|---|---|
| 計算の形 | 利用者の端末で動く1つのバイナリ(`CGO_ENABLED=0`)。補助のプログラムはバイナリに埋め込み、利用者の Node.js で動かす | 今の配り方(GitHub Releases、`go install`)のまま。サーバーは要らない |
| ネットワーク | 外への通信は、利用者の `git` と `gh` が行う GitHub への通信だけ。`cdk synth` には AWS の認証情報を渡さず、外への問い合わせを止める設定を付ける | security-design.md の2節 |
| 保存 | 利用者の `--dir` の下のファイルと、一時フォルダーだけ。データベースは無い | NFR4.3 |
| 環境 | 開発者の手元、CI(GitHub Actions)、検証用の AWS アカウント(663798383745、ap-northeast-1)、テスト専用の GitHub のリポジトリ | チームの進め方 |
| IaC | 検証用のスタックは、テスト専用のリポジトリにある最小の CDK アプリ(TypeScript)。`cdk synth` のテンプレートを `aws cloudformation deploy` で置き、ブートストラップはしない(ID-Q1) | 最小のアプリはアセットを持たず、テンプレートだけで置ける。CDKToolkit のスタックを増やさない |
| 規模 | CI のランナーは GitHub の標準のもの(ubuntu-latest、macos-latest、windows-latest)。検証用のスタックは空の S3 のバケット1つ | 費用はほぼ 0 |

## 使うサービス

| サービス | 役割 | 設定 | 備考 |
|---|---|---|---|
| GitHub Actions | CI | `ci.yml` に仕事を足す(cicd-pipeline.md)。アクションはコミットのハッシュで固定 | 今のワークフローに足す |
| GitHub(テスト専用のリポジトリ `yucn6201g/tag-refactor-pr-sandbox`) | 実際のプルリクエストの結合テストと検証の行き先 | private。`main` にテスト用のアプリ(`internal/cdk/testdata/apps/ts-minimal` と同じもの)を置く。人が作る(ID-Q3) | 外の依存 E1 |
| GitHub の fine-grained トークン | 結合テストの書き込み | テスト専用のリポジトリだけに Contents と Pull requests の読み書き、期限 90 日。tag-refactor のリポジトリの secret `TAG_REFACTOR_TEST_REPO_TOKEN` に人が置く | 値はどのファイルにも書かない(プロジェクトの禁止事項) |
| GitHub のリポジトリの変数 | テスト専用のリポジトリの名前 | `TAG_REFACTOR_TEST_REPO` = `yucn6201g/tag-refactor-pr-sandbox`。人が置く | 秘密ではない |
| AWS CloudFormation(検証用のアカウント) | 検証用のスタック `tag-refactor-cdk-skeleton` | `cdk synth` のテンプレートを、プロファイル `tag-refactor-apply` の `aws cloudformation deploy --stack-name tag-refactor-cdk-skeleton` で置く。置く前にテンプレートと変更の中身を見せ、人の明示の許可を得る(ID-Q1) | `v0.2.0` を出すまで残し、そのあと人の許可を得て消す |
| Amazon S3(検証用のアカウント) | 検証用のスタックの中身 | バケット1つ。名前は CloudFormation が付ける。パブリックアクセスはすべて止め、既定の暗号化(SSE-S3)、バージョニングなし、中身なし、DeletionPolicy は Delete。タグは `Tags.of(bucket).add('Env', 'dev')` の1つ(ID-Q2) | 費用はほぼ 0 |
| AWS IAM Identity Center(SSO) | 検証のときの AWS の認証 | 今のプロファイル(tag-refactor-sandbox、tag-refactor-readonly、tag-refactor-apply)を使う | 切れたら人が `aws sso login` をする(E3) |

## 共有するもの

| 共有するもの | 持ち主の単位 | 使う単位 | 使い方の境目 |
|---|---|---|---|
| テスト専用のリポジトリ | U1 | U2(プルリクエストの残りの結合テスト) | テストは毎回新しいブランチを作り、終わったらプルリクエストを閉じてブランチを消す。`main` は変えない |
| 検証用のスタック `tag-refactor-cdk-skeleton` | U1 | U2〜U6 の検証 | 検証の `plan` だけが読む。タグの書き込み(`apply`)はしない |
| `ci.yml` の Node.js と npm ci の手順 | U1 | U3〜U6 | 単位がテスト用のアプリを足したら、npm ci の対象と Dependabot の行を足す |

## 確認用に残しているスタックとの関係

今残しているスタック(tag-refactor-u1、u2、u5、u6-fixtures)は変えず、消しません。検証用のスタックは、これらと別の名前 `tag-refactor-cdk-skeleton` で置きます。
