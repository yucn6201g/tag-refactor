# インフラの設計 — 質問(U5 影響分析)

U5 は利用者の端末で動く CLI の一部で、配置するサーバーはありません。インフラの設計で決めるのは、CI の流れ、検証用アカウントに置くテスト用のリソース(NFR2.2)、読み取り用ロール R2 の権限の更新(NFR3.9)、検証コマンド `scripts/verify.sh` の U5 の確認です。

決まっていること(質問しません):

- CI(`ci.yml`)のジョブは変えない。U5 のテスト(検出率100%の表、大きな偽の AWS、ファジングの種、`-race` での同じ結果のテスト)は、今の `test`(3つの OS × 2つの版)、`race`、`coverage` のジョブの中で動く。CI からは実際の AWS を呼ばない。
- 足す AWS の SDK のモジュール(S3、KMS、SQS、SNS、Secrets Manager)は、CI の `govulncheck` と Dependabot の対象に入る。
- R2 の権限は、`docs/iam/readonly-policy.json` と同じにする(U1 のテストが2つを照らす)。R2 は U1 の確認用のスタック(`tag-refactor-u1-fixtures`)にあるため、テンプレート(`test/sandbox/u1-fixtures.yaml`)を直し、スタックを更新する。検証用アカウントの変更なので、コード生成のあと、人の許可を得てから行う。
- テスト用のリソースは CloudFormation で作り、Construction の終わりに、ほかの確認用のスタックと一緒に消す。名前は CloudFormation に作らせ、スタックに `project=tag-refactor-fixture` のタグを付ける(U1・U2 と同じ)。
- 新しい操作の時間(NFR1.19 の L95)は、チェックポイントで `--debug` の行から測り、記録に残す。`FuzzParsePolicy` は手元で5分走らせ、記録に残す。

## Q1. 検証用アカウントに置くテスト用のリソース

NFR2.2(検証用アカウントで、テスト用のポリシーへの影響を plan が見つける)のために、どのリソースを置きますか?

A. 新しいスタック `tag-refactor-u5-fixtures` に、影響分析が読む7つの種類をすべて置く。ロール1つ(インラインポリシーで `aws:ResourceTag/env` と `aws:PrincipalTag/team` を使う)、S3 のバケット、KMS のキー、SQS のキュー、SNS のトピック、Secrets Manager のシークレット(それぞれ `env` のタグを持ち、リソースポリシーで `aws:ResourceTag/env` を使う)。U1 のスタックの管理ポリシーと合わせて、すべての読み取りの操作を、R2 の実際の権限で確かめられる。費用は KMS のキー(月 1 USD を時間で割った分)と、シークレット(月 0.40 USD を時間で割った分)で、数日なら数十円
B. A から、費用のかかる KMS のキーと Secrets Manager のシークレットを除く(この2つは偽の AWS のテストだけで確かめる。R2 の権限の誤りは、実際の AWS では確かめられない)
C. 新しいリソースは置かない。U1 のスタックの管理ポリシーだけで確かめる(NFR2.2 の「少なくとも1つのリソースポリシー」を満たさない)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答と決まっていることをまとめると、U5 のインフラの設計は次のとおりです。

1. **CI:** `ci.yml` のジョブは変えない。U5 のテストは今の `test`、`race`、`coverage` のジョブで動き、実際の AWS は呼ばない。足す SDK のモジュールは `govulncheck` と Dependabot の対象に入る。
2. **テスト用のリソース(Q1 A):** 新しいスタック `tag-refactor-u5-fixtures` に、インラインポリシーを持つロール1つ、S3 のバケット、KMS のキー、SQS のキュー、SNS のトピック、Secrets Manager のシークレットを置く。それぞれ `env` のタグを持ち、`aws:ResourceTag/env` などの条件を使うポリシーを付ける。費用は数日で数十円。Construction の終わりに消す。
3. **R2 の権限:** `docs/iam/readonly-policy.json` に足した操作を、U1 のスタックの R2 にも足す(テンプレートを直してスタックを更新する)。
4. **検証コマンド:** `scripts/verify.sh` に U5 の確認(U5 のスタックがあることと、結合テストで plan が期待する指摘を出すこと)を足す。
5. **AWS への変更:** スタックの作成と更新は、コード生成のあと、人の許可を得てから行う。
6. **チェックポイント:** 新しい操作の L95 を測って10分の見積もりを記録し、`FuzzParsePolicy` を手元で5分走らせて記録する。

[Answer]: Looks correct
