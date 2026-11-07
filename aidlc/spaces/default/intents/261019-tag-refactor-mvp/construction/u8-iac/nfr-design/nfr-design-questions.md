# NFR 設計の質問: U8 IaC への反映(u8-iac)

U8 の機能設計(`rules.md` BR1.1〜BR6.3)と NFR 要件(NFR3.16〜NFR3.21、NFR5.25、NFR5.26、NFR6.5、NFR6.6、NFR8.27、NFR9.11、NFR9.12、NFR10.12〜NFR10.14、NFR11.10、NFR12.1)を、どの部品でどう満たすかを決めます。

## すでに決まっていること(質問しません)

- **解析器:** CloudFormation の YAML は goccy/go-yaml の解析木とトークン(別名は展開しない)、JSON は `encoding/json` の `Decoder`、Terraform は hcl/v2 の `hclsyntax`、`--tf-state` は `encoding/json`(NFR 要件の技術の選択)。
- **書き換え:** 元のテキストのバイト範囲の差し替え(Edit)だけ。書き換えたあとに解析し直して確かめる(機能設計 BR4.4、BR4.6)。
- **git:** `os/exec` で、決まった引数と環境で呼ぶ(NFR3.20)。
- **表示:** U4 の言語・色・表の部品を使う。

## Q1. 部品の分け方(パッケージ)

U8 の部品をどう置きますか?

A. 新しいパッケージ `internal/iac` を1つ作り、その中をファイルで分ける(変更案の読み取りと対象の決定、ファイルの集め方と上限、CloudFormation の YAML、CloudFormation の JSON、Terraform、state、Edit の作り方と当て方、差分、git、書き出しの流れ)。各形式の読み取りは「タグの書き場所と範囲を返す」同じ形のインターフェースにする。表示は `internal/display` に、コマンドの入口は `internal/cli` に置く(U6・U7 と同じ分け方)
B. 形式ごとにパッケージを分ける(`internal/iac/cfn`、`internal/iac/tf` など)
C. A と同じだが、git と差分は別のパッケージにする
X. Other (please specify)

[Answer]: A

## Q2. 書き出しの部品の共有

NFR6.5 は、壊れない書き出しの部品を `internal/fileout`(U4。利用者のファイルを書く部品)に寄せ、記録(`internal/journal`)と iac の両方が使うとしています。どう分けますか?

A. `fileout` に「元のファイルを置き換える」書き方(一時ファイルを作って書き、読み直して確かめ、元の権限に合わせて名前を変える)と、Windows の名前の変更のやり直し(待ちの長さを引数で決める)を足す。`journal` の `writeAtomic` は、名前の変更のやり直しを `fileout` のものに置き換える(待ちの長さは U6 のまま)。古い一時ファイルを消す処理は `journal` に残す。U4・U6・U7 のテストの期待は変えない
B. `journal` の部品をそのまま `iac` から呼ぶ(依存の向きは iac から journal)
C. `iac` に別に作る
X. Other (please specify)

[Answer]: A

## Q3. 種類の対応表

機能設計 BR3.2 と NFR10.14 では、CloudFormation の Type と変更案の resourceType の対応表と、Terraform の種類(`aws_*`)の確かめが要ります。表をどこに置きますか?

A. `internal/awsaccess` の書き換えの対象の種類の表(U6。apply が書き換える種類)に、CloudFormation の Type と Terraform のリソースの種類の2つの列を足す。1つの resourceType に CloudFormation の Type が複数ある場合(SAM の `AWS::Serverless::Function` など)は列に複数書ける形にする。Terraform の列は、NFR10.14 (c) のテストと、`--debug` で「state の種類と変更案の種類が合わない」を示すためだけに使う(Terraform の場所は ARN の完全一致で決める。機能設計 BR2.3)
B. `internal/iac` に別の表を作る
X. Other (please specify)

[Answer]: A

## Q4. 差分の作り方

差分(unified diff)は、Edit の範囲から変わった行が分かります。どう作りますか?

A. 一般の差分の方法(Myers など)は使わず、Edit の範囲から「元のファイルの変わった行の範囲」と「新しいファイルの対応する行」を求め、前後3行の文脈を付けて、近い変更を1つのまとまり(hunk)にまとめる。改行の種類は行の終わりとして保ち、表示では行の中身だけを出す。ファイルの終わりに改行がない場合は、git と同じく「\ No newline at end of file」を出す
B. 一般の差分の方法(Myers)を自前で作り、元と新しいファイルの全体を比べる
X. Other (please specify)

[Answer]: A

## Q5. 解析の並行の数

NFR5.25 は、2,000 個のファイル(100 MB)を 60 秒以内に処理することを求めます。ファイルの解析をどう進めますか?

A. ファイルの集め方(フォルダーの歩き方と上限の確かめ)は1つの流れで行い、候補のファイルの読み取りと解析は、CPU の数(上限 8)の作業者で並行に行う。結果の索引と出力の順は、ファイル名の順に並べて、実行ごとに同じにする。ヒープの最大(NFR3.19 の 2 GB)を超えないよう、同時に持つ解析木は作業者の数までにし、索引を作ったら手放す
B. すべて1つの流れで順に行う(並行にしない)
C. A と同じだが、作業者の数を固定の 4 にする
X. Other (please specify)

[Answer]: A

## Q6. 入れ子の深さの事前の確かめ(NFR3.19 (f))

解析の前に、入れ子の深さを数えて 256 を超えるファイルを外します。どう数えますか?

A. 形式ごとの小さな字句の走査を `internal/iac` に作る。YAML は、引用符の中、コメント、ブロックスカラー(インデントで終わりを判定)を飛ばして `[` と `{` を数え、行のインデントの深さ(インデントの段の数)も数える。JSON は文字列の中を飛ばして `[` と `{` を数える。HCL は文字列、コメント、ヒアドキュメントを飛ばして `[`・`{`・`(` を数える。走査は1回で、ファイルの大きさに比例する時間で終わる。数え方が正しいファイルを外さないことは、括弧を多く含む正しいファイルのテストで確かめる
B. 数えずに、解析器のエラーに任せる(深い入れ子でスタックを使い切る危険を受け入れる)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答をまとめると、U8 の NFR 設計は次の内容になります。

1. **部品の分け方(Q1 A):** 新しいパッケージ `internal/iac` を1つ作り、ファイルで分けます(対象の決定、ファイルの集め方と上限、CloudFormation の YAML、JSON、Terraform、state、Edit、差分、git、書き出しの流れ)。形式ごとの読み取りは「タグの書き場所と範囲を返す」同じ形のインターフェースにします。表示は `internal/display`、入口は `internal/cli` です。
2. **書き出しの部品(Q2 A):** `internal/fileout` に、元のファイルを置き換える書き方(一時ファイル、読み直しの確かめ、権限の合わせ、名前の変更)と、待ちの長さを引数で決める Windows のやり直しを足します。`journal` もやり直しを `fileout` のものに置き換えます(待ちは U6 のまま)。古い一時ファイルを消す処理は `journal` に残し、U4・U6・U7 のテストの期待は変えません。
3. **種類の対応表(Q3 A):** `internal/awsaccess` の書き換えの対象の種類の表に、CloudFormation の Type(複数を書ける)と Terraform のリソースの種類の列を足します。Terraform の列はテストと `--debug` の案内だけに使います。
4. **差分(Q4 A):** Edit の範囲から変わった行を求め、前後3行の文脈で近い変更をまとめます。ファイルの終わりに改行がない場合は「\ No newline at end of file」を出します。
5. **並行(Q5 A):** フォルダーの歩き方は1つの流れで、候補のファイルの読み取りと解析は CPU の数(上限 8)の作業者で並行に行います。索引と出力はファイル名の順にし、同時に持つ解析木は作業者の数までにします。
6. **入れ子の深さ(Q6 A):** 形式ごとの小さな字句の走査で、引用符・コメント・ブロックスカラー・ヒアドキュメントを飛ばして括弧とインデントの深さを数えます。

Does this all look correct before I generate the artifact?

[Answer]: Looks correct
