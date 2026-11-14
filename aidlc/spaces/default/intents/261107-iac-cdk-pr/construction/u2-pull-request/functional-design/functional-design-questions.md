# 機能の設計の質問: U2 プルリクエストの作成(残り)

U2 は、U1 で通した `iac --pr` を、要件のとおりに仕上げる単位です(`inception/units-generation/unit-of-work.md`)。受け入れ条件は AC1.1.2〜AC1.1.10 です(`unit-of-work-story-map.md`)。オプション、確かめ、本文、続きの命令の形は、契約(`inception/contract-design/contract-summary.md` の X1、X2、C1)と要件(`requirements.md` の FR1.3〜FR1.13)でほぼ決まっています。ここでは、まだ決まっていないものだけを聞きます。

## Q1. 既定のブランチ名の日時

既定のブランチ名は `tag-refactor/iac-<YYYYMMDD-HHMMSS>` です(契約 X1)。日時をどの時刻で入れるかが決まっていません(契約の未決事項)。

A. UTC にし、末尾に `Z` を付ける(例 `tag-refactor/iac-20261109-035311Z`)。だれがどこで動かしても名前の付け方がそろい、夏時間でずれない
B. 端末の時刻にする(例 `tag-refactor/iac-20261109-125311`)。今の U1 と同じ
C. 任せる
X. Other (please specify)

[Answer]: A

## Q2. 本文が GitHub の上限を超えるとき

GitHub のプルリクエストの本文は 65,536 文字までです。手で直す項目がとても多いと、超えることがあります。

A. 手で直す一覧の後ろを省き、「ほか N 件(全部はこのコマンドの表示を見てください)」の1行で終える。変更したファイルの一覧と `--pr-body-file` の中身は省かない。それでも超えるときは、作る前に止めて理由を示す(終了コード 1)
B. 超えたら作る前に止める(終了コード 1)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q3. `--pr-body-file` のパスの基準

A. ほかのファイルのオプション(`--plan`、`--tf-state`)と同じく、コマンドを動かしたフォルダーからの相対(または絶対)にする
B. `--dir` からの相対にする
C. 任せる
X. Other (please specify)

[Answer]: A

## Q4. 続きの命令をどこに示すか

途中で失敗したときに、続きを手で行う命令(PowerShell 用か sh 用)を示すことは決まっています(契約 C1)。

A. 標準エラーに、済んだ段階と失敗した段階と原因のあとで、続きの命令を1行ずつ示す。本文を書いたファイルの場所も示す。`--debug` が無くても示す
B. `--debug` のときだけ、続きの命令を示す
C. 任せる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- 既定のブランチ名の日時は UTC にし、末尾に `Z` を付ける(例 `tag-refactor/iac-20261109-035311Z`)(Q1: A)
- 本文が GitHub の上限(65,536 文字)を超えるときは、手で直す一覧の後ろを省いて「ほか N 件」の1行で終える。変更したファイルの一覧と `--pr-body-file` の中身は省かない。それでも超えるときは、作る前に止める(終了コード 1)(Q2: A)
- `--pr-body-file` のパスは、コマンドを動かしたフォルダーからの相対(または絶対)にする(Q3: A)
- 途中で失敗したときは、標準エラーに、済んだ段階、失敗した段階と原因、続きの命令(1行ずつ)、本文のファイルの場所を示す。`--debug` が無くても示す(Q4: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
