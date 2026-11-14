# 非機能の設計の質問: U1 最小の一本

U1 の非機能の要件は `construction/u1-walking-skeleton/nfr-requirements/` にあります。そのレビューで残った指摘(外のプログラムの標準エラーからの漏れ、CI のトークンの範囲、Windows での後片付けなど)を、ここで設計として決めます。機能の流れは `construction/u1-walking-skeleton/functional-design/functional-spec.md`、契約は `inception/contract-design/contract-summary.md` です。

## Q1. git と gh の標準エラーの扱い

`git commit` と `git push` は、利用者のフックと署名(パスフレーズの入力など)のために、標準入力と標準エラーを端末につなぐと決めています。ほかの git と gh の呼び出しの出力にも、リモートの URL やアカウントの情報が入ることがあります。

A. commit と push だけは標準エラーを端末に直接つなぐ(利用者の端末にだけ出て、プルリクエストや記録には入らない)。ほかの git と gh の呼び出しの標準エラーは取り込み、失敗したときだけ、伏せ字の規則(ARN の形、12 桁の数字、トークンの形、URL の userinfo)を通してから表示と `--debug` に出す。外のプログラムの出力は、タイトル、本文、コミットのメッセージには入れない
B. すべての呼び出しの標準エラーを取り込み、伏せ字にしてから出す(フックの対話は使えなくなる)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q2. CI のテスト専用のリポジトリのトークンの守り

A. 次のとおりにする
- fine-grained トークンは、テスト専用のリポジトリだけに、Contents と Pull requests の読み書きだけを許し、期限を 90 日にする
- secret は、実際の GitHub の結合テストを動かす1つの仕事の、そのテストの手順の環境にだけ渡す
- テストは、トークンを gh と git の子のプロセスの GH_TOKEN にだけ入れ、`npm ci` などのほかの子のプロセスには渡さない
- 「npm の子のプロセスの環境にトークンが無いこと」を自動テストで確かめる
- ワークフローにトークンの値が書かれていないことを、ワークフローのファイルを読むテストで確かめる
B. secret をワークフロー全体の環境に置く
C. 任せる
X. Other (please specify)

[Answer]: A

## Q3. Windows で外のプログラムを止める方法

上限を超えたとき、子のプロセスとその下のプロセス(cdk が起動する node など)をまとめて止めます。

A. Unix ではプロセスのグループで、Windows ではジョブオブジェクト(閉じると中のプロセスがすべて止まる設定)で止める。どちらも今の依存(`golang.org/x/sys`)で作る
B. Windows では `taskkill /T /F` を呼ぶ
C. 任せる
X. Other (please specify)

[Answer]: A

## Q4. 速さの目安の確かめ方

要件では、補助のプログラムの目安(200 ファイルで 30 秒)と CI の時間の目安(10 分)を、記録だけにしています。

A. ベンチマークのテストは、目安の2倍(60 秒)を超えたら失敗にする(CI の機械のゆれに耐える)。CI の時間の目安は、合否にせず、仕事の時間の上限(timeout-minutes)を 30 分にするだけにする
B. 記録だけにする(合否にしない)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q5. 信頼できないリポジトリと、プルリクエストの行き先の取り違え

A. 次のとおりにする
- README と `iac --help` に、「CDK のアプリがあるときの iac は、そのリポジトリのコードを `cdk synth` で動かす。信頼できるリポジトリでだけ使う」と書く
- `--pr` では、作成の前に、リモート、行き先のブランチ、作るブランチを標準エラーに1行で示す(確認の問いは出さない)
B. 書くだけにし、行き先は示さない
C. 任せる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- commit と push の標準エラーだけを端末に直接つなぐ。ほかの git と gh の標準エラーは取り込み、失敗したときだけ伏せ字にしてから出す。外の出力はタイトル、本文、コミットのメッセージに入れない(Q1: A)
- CI のトークンは、テスト専用のリポジトリだけに Contents と Pull requests の読み書き、期限 90 日。結合テストの手順にだけ渡し、gh と git の子のプロセスにだけ入れ、npm には渡さない。これを自動テストで確かめる(Q2: A)
- 子のプロセスは、Unix ではプロセスのグループ、Windows ではジョブオブジェクトでまとめて止める(Q3: A)
- ベンチマークのテストは目安の2倍(60 秒)で失敗にする。CI の時間は合否にせず、仕事の上限を 30 分にする(Q4: A)
- README と `iac --help` に、CDK のアプリのコードを動かすので信頼できるリポジトリでだけ使うと書く。`--pr` では作成の前に、リモート、行き先、作るブランチを1行で示す(Q5: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
