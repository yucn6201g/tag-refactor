# インフラ設計の質問: U8 IaC への反映(u8-iac)

U8 の `iac` は AWS を呼ばないので、新しいスタックや AWS の資源は要りません。ここでは、CI の変更と、チェックポイントの検証(NFR3.21: 検証用アカウントのスタックから作った変更案を、`test/sandbox/` の写しのテンプレートに反映して確かめる)の作り方を決めます。

## すでに決まっていること(質問しません)

- **スタックは更新しない:** 検証では AWS に書かない(NFR3.21、NFR 要件 Q7)。
- **Terraform:** 検証用アカウントでは確かめず、CI のテストの偽の state と `.tf` で確かめる(NFR3.21)。
- **CI:** 既存のジョブ(3つの OS、Go の2つの版、lint、coverage、govulncheck、race、`test/sandbox/` の cfn-lint)はそのまま使う。新しい依存(hcl/v2、yaml/v3)は既存の govulncheck と Dependabot の対象になる。
- **検証の対象のスタック:** U6 の確認用のスタック `tag-refactor-u6-fixtures`。VPC(VU6)、セキュリティグループ(GU6)、記録用のバケット(BU6R)は、テンプレートの Tags に `project: tag-refactor-fixture` を文字列で直接書いている。

## Q1. 検証で使う変更

検証で作る変更案に、どの変更を入れますか?

A. 確かめ専用のルールファイル `test/sandbox/u8-rules.yaml` を作り、`project` を `Project` に名前を変え、値 `tag-refactor-fixture` を `tag-refactor-fixture-u8` に変える(キー名の変更と値の変更の2つ)。範囲は U6 の確認用のスタックの3つのリソース(`project` のタグを持つもの)だけになるよう、`plan` の結果を確かめ専用の補助(`scripts/u6check.go`)で確かめる。書き換えたテンプレートで、その3つのリソースの Tags の2行ずつ(合わせて6行)だけが変わることを確かめる
B. A に加えて、必須タグの追加と削除も入れる(U6 のテンプレートに確かめ専用のタグを足す変更が要る)
C. ほかのスタック(U1、U5)も範囲に入れる
X. Other (please specify)

[Answer]: A

## Q2. 書き換えたテンプレートの確かめ方

NFR3.21 は、書き換えたテンプレートで `cfn-lint` が通ることを求めます。手元には `cfn-lint` がありません。どう確かめますか?

A. 検証のスクリプトで、`cfn-lint` が `PATH` にあれば使い、なければ使えないことを表示して飛ばす。どちらの場合も、書き換えたテンプレートを `aws cloudformation validate-template`(読み取りだけの API。検証用アカウントの管理用のプロファイルで呼ぶ)で確かめる。CI の lint のジョブは、`test/sandbox/` の元のテンプレートを今までどおり `cfn-lint` で確かめる
B. 手元に `cfn-lint` を入れる(Python の `pip install cfn-lint`)ことを、検証の前提にする
C. `validate-template` だけで確かめ、`cfn-lint` は使わない
X. Other (please specify)

[Answer]: A

## Q3. 人の確認

U6・U7 のチェックポイントと同じく、人が手で確かめる手順を設けますか?

A. 検証のスクリプトの最後に、書き換える前の写しのフォルダーを残し、PowerShell で `--write` なし・`--lang ja` の `iac` を動かすコマンドを表示する。人が日本語の差分と結果を見て確かめ、チェックポイントで承認する。残したフォルダーは、人が確かめたあとに消すよう表示する
B. 人の確認はしない(スクリプトの自動の確かめだけ)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答をまとめると、U8 のインフラ設計は次の内容になります。新しいスタックや AWS の資源は作らず、スタックも更新しません。

1. **検証の変更(Q1 A):** 確かめ専用のルールファイル `test/sandbox/u8-rules.yaml` で、`project` を `Project` に名前を変え、値 `tag-refactor-fixture` を `tag-refactor-fixture-u8` に変える変更案を作ります。範囲が U6 の確認用のスタックの3つのリソースだけであることを `scripts/u6check.go` で確かめ、`test/sandbox/` の写しのテンプレートで、その3つの Tags の2行ずつ(合わせて6行)だけが変わることを確かめます。
2. **テンプレートの確かめ(Q2 A):** `cfn-lint` が `PATH` にあれば使い、なければ表示して飛ばします。どちらの場合も、書き換えたテンプレートを `aws cloudformation validate-template`(読み取りだけ)で確かめます。CI は元のテンプレートを今までどおり `cfn-lint` で確かめます。
3. **人の確認(Q3 A):** 最後に、書き換える前の写しのフォルダーを残し、PowerShell で `--write` なし・`--lang ja` の `iac` を動かすコマンドを表示します。人が日本語の差分と結果を確かめてからチェックポイントを承認し、残したフォルダーを消します。

Does this all look correct before I generate the artifact?

[Answer]: Looks correct
