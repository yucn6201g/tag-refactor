# インフラの設計 — 質問(U3 ルール)

U3 のインフラ(CI と、検証用 AWS アカウントでの確認)を決めます。U1 と U2 のインフラの設計は、そのまま使います。

決まっていること(質問しません):

- U3 は新しい AWS の操作を呼ばない。`rules suggest` は読み取り用ロール R2 の今の権限(`tag:GetResources` など)だけで動く。例の IAM ポリシーと確認用のスタックは変えない。
- CI のジョブは変えない。U3 のテスト(ファジングの種の入力、読み戻し、性能)は、今の `test`、`race`、`coverage` のジョブの中で動く。ファジング(5分)は手元だけで動かす。
- 検証コマンドは U2 で承認した `scripts/verify.sh` のまま。U3 の確認を中に足す。

## Q1. U3 のチェックポイントでの実機の確認

`scripts/verify.sh` に、U3 の確認をどう足しますか?

A. 読み取り用のプロファイルで `rules suggest --region ap-northeast-1 --output <一時ファイル>` を動かし、その下書きをそのまま `scan --rules` に渡して読み込めることと、`env` と `Env` が1つのまとまり(正しいキー名と別名)になっていることを確かめる。確認用のスタックは変えない。日本語の値の保存は、CI のテストと、チェックポイントでの人の手元の PowerShell の確認で確かめる
B. A に加えて、確認用のスタック(U1)に日本語の値のタグを足し、実機でも日本語の値の読み戻しを確かめる(スタックの更新が要る)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- 実機の確認(Q1): `scripts/verify.sh` に、読み取り用のプロファイルで `rules suggest --region ap-northeast-1 --output <一時ファイル>` を動かし、その下書きをそのまま `scan --rules` に渡して読み込めることと、`env` と `Env` が1つのまとまり(正しいキー名と別名)になっていることを確かめる手順を足します。確認用のスタックは変えません。日本語の値の保存は、CI のテストと、チェックポイントでの人の手元の PowerShell の確認で確かめます。

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
