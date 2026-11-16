# 基盤の設計の質問: U3 CDK の synth と対応づけ(残り)

tag-refactor は利用者の端末で動く CLI で、サーバーやクラウドの資源を置きません。U3 の基盤の設計は、CI(GitHub Actions)と Dependabot の変更だけです。変更の中身(Python の準備を go test を動かす仕事に足す、wheel だけの pip、`--ignore-scripts` の npm ci、Dependabot に py-minimal の pip と npm を足す)は、NFR 要件と NFR 設計(`construction/u3-cdk-synth/nfr-design/security-design.md` の10節)で決まっています。ここでは1点だけ聞きます。

## Q1. CI で入れる Python の版の書き方

今の lint の仕事は `python-version: "3.13"`(副の版まで。パッチの版は CI の実行のたびに最新になる)で Python を入れています。

A. 同じく `"3.13"` と書き、lint と同じ版にそろえる。パッチの版の違いで結果が変わることは、CDK の synth ではまず無いとする
B. パッチの版まで固定する(`"3.13.x"` の具体の値)。Dependabot は Python の版を更新しないので、手で上げる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- Q1 = A: CI で入れる Python は、lint の仕事と同じ `python-version: "3.13"` と書きます(パッチの版は実行のたびに最新)。
- そのほかの CI と Dependabot の変更は、NFR 設計の10節のとおりです。go test を動かす仕事(3つの OS のテスト、race、coverage、release.yml の build)に、`actions/setup-python`(ハッシュで固定)、py-minimal の `.venv` の作成、`pip install --only-binary=:all:`、`pip check`、`npm ci --ignore-scripts` を足し、`ci-gate` が待つ仕事は変えません。Dependabot に py-minimal の pip と npm を足します。AWS の認証情報と秘密の値は CI に足しません。

この要約は正しいですか?

A. Looks correct
B. Request Changes
X. Other (please specify)

[Answer]: Looks correct
