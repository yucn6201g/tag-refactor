# インフラの設計 — 質問(U1 最小の一本)

tag-refactor は利用者の端末で動く CLI で、サーバーやクラウドの上に置くものはありません。U1 のインフラは、(1) GitHub Actions の CI、(2) 検証用 AWS アカウントの確認用のリソース、(3) 検証用アカウントへの手元からの接続、の3つです。

決まっていること(質問しません):

- CI は GitHub Actions。Linux、macOS、Windows と、Go の2つの版の組み合わせで、テスト、lint、govulncheck、3つの OS でのビルドと `--help` の確認を行う。カバレッジは Linux で測る。Actions は SHA で固定、権限は読み取りだけ、govulncheck は週1回の定期実行も行う(NFR の設計 `security-design.md` §6、`reliability-design.md`)。
- CI から実際の AWS は呼ばない。秘密の値(AWS の認証情報)は CI に置かない(チームの進め方 §Testing Posture)。
- 公開(GitHub Releases)は U1 に含めない(別の作業)。
- GitHub のブランチの保護は人が設定し、U1 のチェックポイントで確かめる。

## Q1. 検証用アカウントの確認用のリソースの作り方

U1 の検証に使う確認用のリソース(タグ `env`・`Env` を付けたリソース、`aws:ResourceTag/env` などの条件を含み、ロールに付いた IAM の管理ポリシー、読み取り用のポリシーの例だけを付けたロール)を、どう作りますか?

A. CloudFormation のテンプレートをリポジトリに置き、人が検証用アカウントにデプロイし、確認が終わったら削除する(費用のかからないリソースだけを使う)
B. 手順書を置き、人がコンソールや CLI で手で作る
C. AWS CDK で作る
X. Other (please specify)

[Answer]: A

## Q2. 検証用アカウントへの手元からの接続

手元から検証用アカウントに接続するとき、どの認証の方法を使いますか?

A. IAM Identity Center(SSO)のプロファイル(一時的な認証情報。長く使えるキーを手元に置かない)
B. IAM ユーザーのアクセスキーを設定したプロファイル
X. Other (please specify)

[Answer]: A

## Q3. `main` に取り込むたびにビルドする確認用のバイナリ

チームの進め方で「`main` に取り込むたびに、CI が確認用のバイナリをビルドする(公開しない)」と決めています。U1 では、どう残しますか?

A. 3つの OS のバイナリを、GitHub Actions の成果物として7日間だけ残す
B. Linux のバイナリだけを7日間残す
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- 確認用のリソース(Q1): CloudFormation のテンプレートをリポジトリに置きます。人が検証用アカウントにデプロイし、確認が終わったら削除します。費用のかからないリソース(IAM のロールと管理ポリシー、空の S3 バケット、SQS のキューなど)だけを使います。
- 手元からの接続(Q2): IAM Identity Center(SSO)のプロファイルを使います。長く使えるアクセスキーは手元に置きません。
- 確認用のバイナリ(Q3): `main` に取り込むたびに、3つの OS のバイナリを GitHub Actions の成果物として7日間だけ残します。公開はしません。

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
