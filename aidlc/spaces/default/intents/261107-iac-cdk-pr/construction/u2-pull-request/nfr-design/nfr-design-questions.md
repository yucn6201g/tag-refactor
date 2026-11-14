# 非機能の設計の質問: U2 プルリクエストの作成(残り)

U2 の非機能の要件は `construction/u2-pull-request/nfr-requirements/` にあります。U1 の非機能の設計(`construction/u1-walking-skeleton/nfr-design/`)は、そのまま U2 にも当てます。ここでは、U2 で新しく決める設計のうち、人に決めてもらいたい2点を聞きます。ほかの点(本文の形の確かめを `internal/redact` に置くこと、確かめる時点を書き換えの前の入力の確かめにすることなど)は、要件と機能の設計から決まるので、設計の文書にそのまま書きます。

## Q1. 続きの命令を確かめる PowerShell の版

続きの命令は Windows では PowerShell 用に出します。Windows に最初から入っている Windows PowerShell 5.1(`powershell.exe`)と、別に入れる PowerShell 7(`pwsh`)では、外のプログラムに引数を渡す方法が違います(7.3 からは `"` もそのまま渡る)。安全に囲めない値(`"`、制御文字、空白を含んで末尾が `\` の値)は、どちらの版でも手順の文にする決まりです(NFR1.9)。

A. CI の Windows で、5.1 と 7 の両方で命令を実際に動かして、受け取った引数を比べる(windows-latest には両方が入っている)
B. 5.1 だけで確かめる(利用者の多くが使う版。7 は確かめない)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q2. 残った本文のファイルの後片付け

途中で失敗したとき、本文のファイルは `<Git の管理フォルダー>/tag-refactor/` に残します(NFR1.10)。続きの命令を使い終えたあとも、利用者が消さない限り残ります。

A. tag-refactor は、自分が今回作ったファイルしか消さない。残ったファイルの場所と消し方は、失敗の表示と README で示す
B. プルリクエストができたとき、同じフォルダーの、7日より前に作られた `pr-body-*.md` も消す
C. 任せる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- Q1 = A: 続きの命令の PowerShell 用の引用は、CI の Windows で Windows PowerShell 5.1(`powershell.exe`)と PowerShell 7(`pwsh`)の両方で、命令を実際に動かして、受け取った引数を比べて確かめます。
- Q2 = A: tag-refactor は、今回作った本文のファイルだけを消します(成功したとき)。残ったファイルの場所と消し方は、失敗の表示と README で示します。
- そのほかの設計は、要件と機能の設計から決まるものとして、設計の文書に書きます(本文の形の確かめは `internal/redact` に置き、書き換えの前の入力の確かめで行う、など)。U1 の非機能の設計は U2 にもそのまま当てます。

この要約は正しいですか?

A. Looks correct
B. Request Changes
X. Other (please specify)

[Answer]: Looks correct
