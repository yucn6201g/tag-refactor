# 実現可能性と制約 — 質問

前のステージで決めた内容(intent-statement.md)をもとに、実現の見通しと制約を確認します。

## Q1. 連携が必要な既存の仕組み

最初のバージョンで apply、rollback、IaCへの書き戻しを含めることになりました。ツールが読み書きする必要がある既存の仕組みはどれですか?(select all that apply)

A. Terraform(state を S3 などのリモートに置いている)
B. CloudFormation、または CDK で作ったスタック
C. AWS Organizations の管理アカウント(SCP、RCP の取得元)
D. GitHub(IaCの修正をプルリクエストとして出す先)
E. None
X. Other (please specify)

[Answer]: A, B, D

## Q2. 規制やコンプライアンスの要件

このツールの開発や利用に関わる規制・社内規程はありますか?(select all that apply)

A. 特になし(OSSとして一般公開する)
B. 社内のセキュリティ規程(本番環境への書き込み権限の承認など)
C. 監査対応(SOC 2、ISO 27001 など)で、変更の記録を残す必要がある
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q3. 使う言語と経験

ツールを書く言語と、その経験はどれに近いですか?

A. Go(経験がある)
B. Go(経験は少ないが、単一バイナリで配りやすいので使いたい)
C. TypeScript / Node.js
D. Python
E. Not yet defined
X. Other (please specify)

[Answer]: B

## Q4. 期間

最初のバージョンを、いつまでに使える状態にしたいですか?

A. 1か月以内
B. 3か月以内(個人の空き時間で進める)
C. 期限は決めない
D. Not yet defined
X. Other (please specify)

[Answer]: C

## Q5. 組織上の制約

開発や公開を進めるうえで、組織上の制約はありますか?(select all that apply)

A. なし
B. 業務時間外にしか作業できない
C. OSSとして公開するには社内の承認が必要
D. Not identified
X. Other (please specify)

[Answer]: A

## Q6. 検証に使えるAWS環境

apply や rollback を含めるため、実際にタグを書き換えて試せる環境が必要です。使える環境はどれですか?(select all that apply)

A. 個人の検証用アカウント(書き込みも可)
B. 社内の検証用アカウント(書き込みも可)
C. 既存のアカウントで、読み取りだけなら試せる
D. まだない
X. Other (please specify)

[Answer]: A

## Q7. 1アカウントの範囲での SCP と RCP の扱い

影響分析の対象に SCP と RCP を含めました。一方で、複数アカウント(Organizations)の横断は最初のバージョンに含めません。SCP と RCP は、Organizations の管理アカウント(または委任された管理者のアカウント)からしか読み取れません。普通のメンバーアカウントで実行したときにどう扱いますか?

A. 管理アカウント相当の読み取り権限があるときだけ SCP と RCP を調べ、ないときは「確認できなかった」と表示する
B. SCP と RCP は、利用者がファイルで渡したものだけを分析する
C. A と B の両方に対応する
D. SCP と RCP は最初のバージョンの対象から外す
E. Not yet defined
X. Other (please specify)

[Answer]: D

## Q8. 最初に対応するIaC

IaCへの書き戻しで、最初のバージョンが対応するのはどれですか?(select all that apply)

A. Terraform
B. CloudFormation
C. CDK
D. Not yet defined
X. Other (please specify)

[Answer]: A, B, C

## Q9. CDK の書き戻しで対象にする言語(Q8の補足)

CDK はアプリを TypeScript、Python、Java、Go、C# などで書けます。言語ごとにソースコードの直し方が違うため、作る量が大きく変わります。最初のバージョンでは、どこまで対応しますか?

A. TypeScript だけ
B. TypeScript と Python
C. CDK が対応するすべての言語
D. CDK のソースは直さず、直すべき箇所とコードの案を表示するだけにする
E. Not yet defined
X. Other (please specify)

[Answer]: B

## Consolidated Summary Confirmation

回答のまとめ:

- 連携する既存の仕組み(Q1): Terraform(リモートの state)、CloudFormation / CDK のスタック、GitHub(プルリクエストの作成先)
- 規制・社内規程(Q2): 特になし(OSSとして一般公開する)
- 言語と経験(Q3): Go。経験は少ないが、単一バイナリで配りやすいので使いたい
- 期間(Q4): 期限は決めない
- 組織上の制約(Q5): なし
- 検証環境(Q6): 個人の検証用アカウント(書き込みも可)
- SCP と RCP(Q7): 最初のバージョンの影響分析の対象から外す(前のステージで選んだ対象から狭める)
- IaCへの書き戻し(Q8): Terraform、CloudFormation、CDK のすべてに対応する
- CDK の言語(Q9): TypeScript と Python

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
