# 非機能の設計の質問: U3 CDK の synth と対応づけ(残り)

U3 の非機能の要件は `construction/u3-cdk-synth/nfr-requirements/` にあります。U1 と U2 の非機能の設計(`construction/u1-walking-skeleton/nfr-design/`、`construction/u2-pull-request/nfr-design/`)は、そのまま U3 にも当てます。ここでは、U3 で新しく決める設計のうち、人に決めてもらいたい2点を聞きます。ほかの点(Python の版の確かめを synth の手前に置くこと、`.venv` の環境の足し方、`--debug` のパスの `~` への置き換え、進み具合の時計を `display.Printer` に置くことなど)は、要件と機能の設計から決まるので、設計の文書にそのまま書きます。

## Q1. 手元で Python のテストを動かすための `.venv`

`py-minimal` の synth のテストには、`py-minimal` の `.venv` と、そこに入れた依存が要ります(NFR8.3)。CI はテストの前に作ります。手元では、無ければ理由を出してテストを飛ばします。

A. テストは `.venv` を作らない。開発者向けの文書(CONTRIBUTING か README の開発の節)に、手元で `.venv` を作る3行の手順を書く
B. `go test` の中で、無ければ `.venv` を作って依存を入れる(ネットワークを使い、テストが遅くなる)
X. Other (please specify)

[Answer]: A

## Q2. CI の Python のテストの置き場所

Python と `py-minimal` の準備(Python を入れる、`.venv` を作る、依存を入れる)で、OS ごとの CI の時間が延びます(数分の見込み)。

A. 今の3つの OS のテストの仕事に、準備の手順を足す(`ci-gate` が待つ仕事は変わらない)
B. Python のテストだけの仕事を別に作り、並べて動かす(`ci-gate` が待つ仕事を足す)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- Q1 = A: テストは `.venv` を作りません。手元で `py-minimal` の `.venv` を作る3行の手順を、開発者向けの文書に書きます。`.venv` が無いとき、手元ではテストを理由つきで飛ばし、CI では失敗にします。
- Q2 = A: CI の Python の準備(`actions/setup-python`、`.venv` を作る、`pip install --only-binary=:all:`、`npm ci --ignore-scripts`)は、今の3つの OS のテストの仕事に足します。`ci-gate` が待つ仕事は変えません。
- そのほかの設計は、要件と機能の設計から決まるものとして、設計の文書に書きます(Python の版の確かめを synth の手前に置く、`.venv` の PATH と VIRTUAL_ENV の足し方、`--debug` のパスの `~` への置き換え、進み具合の時計を `display.Printer` に置く、など)。U1 と U2 の非機能の設計は U3 にもそのまま当てます。

この要約は正しいですか?

A. Looks correct
B. Request Changes
X. Other (please specify)

[Answer]: Looks correct
