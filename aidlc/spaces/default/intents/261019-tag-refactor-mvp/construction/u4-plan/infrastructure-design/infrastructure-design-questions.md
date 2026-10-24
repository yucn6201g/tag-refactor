# インフラの設計 — 質問(U4 変更案)

U4 のインフラ(CI と、検証用 AWS アカウントでの確認)を決めます。U1〜U3 のインフラの設計は、そのまま使います。

決まっていること(質問しません):

- U4 は新しい AWS の操作を呼ばない。`plan` の4つの形式、`--out`、`--lang` は、読み取り用ロール R2 の今の権限だけで動く。例の IAM ポリシーと確認用のスタックは変えない。
- CI のジョブは変えない。U4 のテスト(4つの形式と2つの言語の期待のファイル、4つの形式の件数の一致、文言の一覧、`LoadPlan` のファジングの種の入力、性能、Windows の表示言語の関数を実際に呼ぶテスト)は、今の `test`(3つの OS)、`race`、`coverage` のジョブの中で動く。ファジング(5分)は手元だけで動かす。
- 期待のファイルは `testdata` に置き、今の `.gitattributes`(`* text=auto eol=lf`)で LF に固定される。ファジングの種は `**/testdata/fuzz/** -text` で守られる。足す設定はない。
- 端末での幅の表は、Go の `unicode` と同じ版の Unicode のデータから作ってコミットする。表に書いた版と `unicode.Version` が違えばテストが落ちるようにし、Go を上げたときに作り直しを忘れないようにする。表を作るスクリプトは手元で動かし、CI ではネットワークを使わない。
- 検証コマンドは U2 で承認した `scripts/verify.sh` のまま。U4 の確認を中に足す。

## Q1. U4 のチェックポイントでの実機の確認

`scripts/verify.sh` に、U4 の確認をどう足しますか?

A. Go の結合テスト(`-tags integration`。U2 と同じ形)を1つ足し、`verify.sh` から動かす。読み取り用のプロファイルで、U2 の確認用のルールファイル(`test/sandbox/u2-rules.yaml`)を使い `plan --region ap-northeast-1` を4つの形式と `--out` で動かす。保存した変更案を `planner.LoadPlan` で読めること、4つの形式と保存した変更案で件数が一致すること、U1 の確認用のスタックの `env` と `Env` から作られた変更があることを確かめる。日本語の表示は、チェックポイントでの人の手元の PowerShell の確認で確かめる
B. `verify.sh` の中で実行ファイルを動かし、終了コード(0 か 2)、保存したファイルができたこと、JSON の出力が `{` で始まることだけを確かめる(中身は CI のテストに任せる)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- 実機の確認(Q1): Go の結合テスト(`-tags integration`)を1つ足し、`scripts/verify.sh` から動かします。読み取り用のプロファイルで、U2 の確認用のルールファイル(`test/sandbox/u2-rules.yaml`)を使って `plan --region ap-northeast-1` を4つの形式と `--out` で動かし、保存した変更案を `planner.LoadPlan` で読めること、4つの形式と保存した変更案で件数が一致すること、U1 の確認用のスタックの `env` と `Env` から変更が作られることを確かめます。CI のジョブ、例の IAM ポリシー、確認用のスタックは変えません。日本語の表示は、チェックポイントで人が手元の PowerShell で確かめます。

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
