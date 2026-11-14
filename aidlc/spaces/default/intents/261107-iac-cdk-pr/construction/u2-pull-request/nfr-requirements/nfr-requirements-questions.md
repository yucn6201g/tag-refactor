# 非機能の要件の質問: U2 プルリクエストの作成(残り)

U2 の非機能の要件の大部分は、U1 で決めたもの(`construction/u1-walking-skeleton/nfr-requirements/`)をそのまま使います。機能の設計は `construction/u2-pull-request/functional-design/` にあります。

新しく決めることは1つです。機能の設計では、`--pr-body-file` の中身は利用者が書いたものなので、伏せ字を通さず、そのまま本文に入れると決めました(契約 X2)。

## Q1. 利用者の本文にトークンや ARN の形があるとき

A. 本文はそのまま使うが、トークンの形(`ghp_` などの GitHub のトークン、`AKIA` などの AWS のアクセスキーの ID)があれば、書き換えの前に止める(入力の誤り、終了コード 1。どの形がどの行にあったかを示し、値は出さない)。ARN と 12 桁の数字は、利用者が意図して書くことがあるので止めず、標準エラーに1行で知らせるだけにする
B. 何もしない(利用者の責任として、そのまま載せる)
C. どの形があっても止める
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- Q1 = A: `--pr-body-file` の本文に GitHub のトークン(`ghp_`、`gho_`、`ghu_`、`ghs_`、`ghr_`、`github_pat_`)か AWS のアクセスキーの ID(`AKIA`、`ASIA`)の形があれば、どのファイルも書き換える前に入力の誤りとして止めます(終了コード 1)。どの形がどの行にあったかを示し、値は出しません。ARN と 12 桁の数字は止めず、標準エラーに1行で知らせるだけにして、本文はそのまま載せます。
- それ以外の非機能の要件(秘密の値を出さない、AWS の認証情報を渡さない、性能、信頼性、観測のしやすさ、技術の選択)は、U1 で決めたものをそのまま U2 にも当てます。

この要約は正しいですか?

A. Looks correct
B. Request Changes
X. Other (please specify)

[Answer]: Looks correct
