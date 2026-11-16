# 非機能の要件の質問: U3 CDK の synth と対応づけ(残り)

U3 の非機能の要件の大部分は、U1 で決めたもの(`construction/u1-walking-skeleton/nfr-requirements/`)をそのまま使います。たとえば、synth に AWS の認証情報を渡さないこと、外のプログラムを子まで止めること、表示に秘密の値を出さないことです。機能の設計は `construction/u3-cdk-synth/functional-design/` にあります。

新しく決めることは2つです。

## Q1. Python の版を確かめるか

U1 では、synth の前に `node --version` で Node.js が 18 以上かを確かめ、古ければ理由を示してそのアプリを失敗にしています。Python のアプリで同じ確かめをするかを決めます。確かめないと、古い Python では synth が「synth のエラー」で終わり、原因は `--debug` を付けないと分かりません。

A. 確かめる。synth の前に、使う Python(`.venv` のものか PATH のもの)で版を読み、`aws-cdk-lib` が求める最小の版(3.9 を予定。Code Generation で、テスト用のアプリに固定する `aws-cdk-lib` の求める版と照らして決める)より古ければ、synth せずにそのアプリを失敗にし、版が古いことと入れ方を示す
B. 確かめない(古い Python では synth のエラーになる)
X. Other (please specify)

[Answer]: A

## Q2. 全体の時間の上限を置くか

`--synth-timeout`(既定 10 分)は1回の synth ごとの上限です。アプリが N 個あると、最も長くて N × 10 分、作成の記録のための2回目の synth を含めると N × 20 分かかります。

A. 全体の上限は置かない。1回ごとの上限と、進み具合の表示(経過時間と1分ごとの行)と、Ctrl+C での中断で足りるとする。README に、最も長い時間の見積もり方を書く
B. 全体の上限を置く(新しいオプションを足す)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- Q1 = A: Python のアプリでは、synth の前に、使う Python(`.venv` のものか PATH のもの)で版を読みます。`aws-cdk-lib` が求める最小の版(3.9 を予定。Code Generation で、テスト用のアプリに固定する `aws-cdk-lib` の求める版と照らして決めます)より古ければ、synth せずにそのアプリを失敗にし、版が古いことと入れ方を示します。
- Q2 = A: 全体の時間の上限は置きません。1回の synth ごとの上限(`--synth-timeout`)、進み具合の表示、Ctrl+C の中断で足りるとし、README に最も長い時間の見積もり方(アプリの数 × 上限 × 2)を書きます。
- それ以外の非機能の要件(synth に AWS の認証情報を渡さない、外のプログラムを子まで止める、一時ファイルを消す、表示に秘密の値を出さない、3つの OS の CI で synth のテストを動かす、表示を英語と日本語で用意する)は、U1 で決めたものを U3 にも当て、Python のアプリと複数のアプリに広げます。

この要約は正しいですか?

A. Looks correct
B. Request Changes
X. Other (please specify)

[Answer]: Looks correct
