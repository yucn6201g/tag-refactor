# 機能設計の質問: U8 IaC への反映(u8-iac)

U8 では、変更案(`plan --out` の JSON)の中身を IaC のソースに反映する `iac` を作ります。apply で AWS のタグを変えても、IaC のソースが古いままだと、次に IaC を適用したときにタグが元に戻ります。それを防ぐためのコマンドです。詳しい振る舞いは、要件で「第3弾に取りかかる前に決める」としていました(要件 OQ3)。この質問で決めます。

## すでに決まっていること(質問しません)

- **ストーリー:** US7.1(Terraform と CloudFormation への反映。AC7.1.1、AC7.1.2。Should)、US7.2(CDK の TypeScript と Python。Could)、US7.3(プルリクエストの作成。AC7.3.1。Could)。要件 FR8.1〜FR8.4。
- **入力(契約 C2):** 変更案の JSON を読みます。リソースごとに `arn`、`resourceType`、`beforeTags`(`aws:` を含むタグ全体)、`iacManagedBy`(例: `cloudformation:stack/logging`)と、4種類の変更(`renameKey`、`changeValue`、`addRequired`、`removeKey`)があります。
- **CloudFormation のリソースの見分け方:** CloudFormation で作ったリソースには、AWS が `aws:cloudformation:stack-name` と `aws:cloudformation:logical-id` のタグを付けます。変更案の `beforeTags` にもこの2つが入っているので、テンプレートの中の論理 ID とつなげられます。
- **Terraform のリソース:** タグだけでは Terraform で管理されているかが分かりません(要件 AS7)。どのリソースがどの `resource` ブロックかを知るには、Terraform の state が要ります。
- **`aws:` のタグ:** IaC にも書きません(全体の決まり)。
- **既存の部品:** 変更案の読み込み(U4 の `planner`)、表示の言語と色(U4)、YAML の読み書き(`go.yaml.in/yaml/v3` がすでに依存にある)。
- **CLI の口(契約 C5):** U1 では `iac` を「まだ使えないコマンド」にしてあります。U8 でこれを置き換えます。

## Q1. U8 で作る範囲

FR8.1(Terraform)と FR8.2(CloudFormation)は Should、FR8.3(CDK)と FR8.4(プルリクエスト)は Could です。U8 で何を作りますか?

A. Terraform と CloudFormation(YAML と JSON)への反映だけを作る。CDK とプルリクエストの作成は作らず、README に後の作業として書く(Could を後回しにして、反映の正しさに集中する)
B. A に加えて、プルリクエストの作成(FR8.4)も作る。CDK は作らない
C. 4つすべてを作る(CDK は TypeScript と Python のソースの、タグの文字列の値だけを書き換える)
D. CloudFormation だけを作り、Terraform 以降は後にする
X. Other (please specify)

[Answer]: A

## Q2. 書き換えるファイルの見つけ方

変更案には、テンプレートや `.tf` のファイルの場所は入っていません。どう見つけますか?

A. 利用者が IaC のリポジトリのフォルダーを `--dir` で渡す。CloudFormation は、そのフォルダーの `.yaml`・`.yml`・`.json`・`.template` のうち、`Resources` に論理 ID を持つテンプレートを探す。Terraform は、利用者が `terraform show -json` の出力を `--tf-state <ファイル>` で渡し、ARN(か ID)から `resource` のアドレス(例: `aws_sqs_queue.orders`)を引き、そのアドレスのブロックを `--dir` の `.tf` から探す
B. A と同じだが、Terraform の state はツールが `terraform show -json` を自分で動かして読む(`terraform` のコマンドが要る)
C. 利用者がスタックごと・リソースごとの対応表(YAML)を書いて渡す
X. Other (please specify)

[Answer]: A

## Q3. 1つのリソースが複数のテンプレートやブロックに見つかったとき

同じ論理 ID を持つテンプレートが `--dir` の中に2つ以上ある(環境ごとのコピーなど)場合や、同じアドレスのブロックが2つ以上のモジュールにある場合はどうしますか?

A. どれにも書かず、そのリソースを「場所を決められない」として結果の一覧に出す。スタック名とファイル名が同じものが1つだけあれば、それを選ぶ
B. 見つかったすべてに書く
C. 最初に見つかったものに書く
X. Other (please specify)

[Answer]: A

## Q4. タグの値が文字列でないとき

IaC では、タグの値が文字列でない書き方がよくあります。CloudFormation の `!Ref`・`!Sub`・`!GetAtt`・パラメーター、Terraform の変数・`local`・`merge()`・`for` 式、モジュールの引数、プロバイダーの `default_tags` などです。どうしますか?

A. タグのキーと値が、そのリソースのブロックの中に文字列で直接書いてあるものだけを書き換える。それ以外(式、変数、モジュール、`default_tags`、スタックに付けたタグの伝わり)は書き換えず、「手で直す」として場所と理由を一覧に出す
B. A に加えて、Terraform の `default_tags` に文字列で書いてあるキーも書き換える(プロバイダーの中で1回だけ)
C. 式や変数も追いかけて、元の定義(パラメーターの既定値、`variable` の `default`、`locals`)を書き換える
X. Other (please specify)

[Answer]: A

## Q5. ファイルの書式の保ち方

書き換えたファイルの差分は、人がプルリクエストでレビューします。書式をどこまで保ちますか?

A. 変える行だけを変える(テキストの位置を使って、キー・値・タグの行だけを差し替える。コメント、並び、インデント、引用符、ほかの行はそのまま)。Terraform は HashiCorp の `hclwrite`(書式を保って書き換えるライブラリ)を新しい依存として使う。CloudFormation の YAML と JSON は、読み取りで位置を知り、その範囲だけを差し替える
B. ファイル全体を読み込んで書き出し直す(書式やコメントが変わってもよい)
C. A と同じだが、新しい依存は足さず、Terraform も自前の簡単な読み取りで位置を探す
X. Other (please specify)

[Answer]: A

## Q6. ファイルを書き換えるか、差分を示すか

`iac` はどこまでしますか?

A. 既定では書き換えずに、ファイルごとの差分(unified diff)と「手で直す」一覧を示す。`--write` を付けたときだけファイルを書き換える。書き換える前に、`--dir` が git の作業ツリーで、書き換えるファイルにコミットしていない変更がないことを確かめる(あれば止める)
B. 既定で書き換える(git の作業ツリーかは確かめない)。`--dry-run` で差分だけを示す
C. A と同じだが、git の作業ツリーかは確かめない
X. Other (please specify)

[Answer]: A

## Q7. 4種類の変更の反映の仕方

変更案の4種類の変更(キー名の変更、値の変更、必須タグの追加、キーの削除)を、IaC にどう反映しますか?

A. 4種類すべてを反映する。キー名の変更と値の変更はその場で差し替え、削除は行(か要素)を消す。必須タグの追加は、リソースにタグの書き場所(CloudFormation の `Tags`、Terraform の `tags`)が文字列だけで書いてあればそこに足し、なければ足さずに「手で直す」に出す
B. A と同じだが、タグの書き場所がなければ新しく作る(`Tags:` や `tags = {}` を足す)
C. キー名の変更と値の変更だけを反映し、追加と削除は「手で直す」に出す
X. Other (please specify)

[Answer]: A

## Q8. AWS を呼ぶか

`iac` は、変更案と手元のファイルだけで動けます(CloudFormation の論理 ID は変更案のタグにあり、Terraform は state のファイルから引くため)。AWS を呼びますか?

A. 呼ばない。認証情報も要らない(変更案のアカウントと、今のアカウントの確かめもしない)
B. CloudFormation のスタックのテンプレートを `GetTemplate` で読み、手元のテンプレートと同じかを確かめる(違えば警告する)
X. Other (please specify)

[Answer]: A

## Q9. 終了コードと結果の表示

`iac` の終わり方をどうしますか?

A. すべての変更を反映できたら 0。「手で直す」や「場所を決められない」が1件でもあれば 2(apply と同じく「確かめが要る」の意味)。変更案やファイルが読めない、`--write` で git の確かめに失敗した、などは 1。結果には、ファイルごとの反映した件数、手で直す一覧(リソース、キー、ファイルと行、理由)、IaC で管理されていないリソースの件数を出す
B. 反映できなかったものがあっても 0 にし、一覧を出すだけにする
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答をまとめると、U8 の機能設計は次の内容になります。

1. **範囲(Q1 A):** Terraform と CloudFormation(YAML と JSON)への反映だけを作ります(US7.1、FR8.1、FR8.2)。CDK(FR8.3)とプルリクエストの作成(FR8.4)は作らず、README に後の作業として書きます。US7.2 と US7.3 は U8 では満たしません。
2. **ファイルの見つけ方(Q2 A):** `iac --plan <変更案> --dir <IaC のフォルダー>`。CloudFormation は、変更案の `aws:cloudformation:logical-id` のタグの論理 ID を `Resources` に持つテンプレートを、`--dir` の `.yaml`・`.yml`・`.json`・`.template` から探します。Terraform は、`terraform show -json` の出力を `--tf-state <ファイル>` で渡してもらい、ARN(か ID)から `resource` のアドレスを引き、そのブロックを `--dir` の `.tf` から探します。
3. **見つかった場所が2つ以上(Q3 A):** どれにも書かず「場所を決められない」として一覧に出します。スタック名とファイル名が同じものが1つだけあれば、それを選びます。
4. **文字列でない値(Q4 A):** タグのキーと値がリソースのブロックの中に文字列で直接書いてあるものだけを書き換えます。式・変数・モジュール・`default_tags`・スタックのタグの伝わりは書き換えず、「手で直す」として場所と理由を一覧に出します。
5. **書式(Q5 A):** 変える行だけを差し替え、コメント・並び・インデント・引用符・ほかの行はそのままにします。Terraform は `hclwrite` を新しい依存として使い、CloudFormation の YAML と JSON は読み取りで位置を知ってその範囲だけを差し替えます。
6. **書き換えと差分(Q6 A):** 既定では書き換えず、ファイルごとの差分(unified diff)と「手で直す」一覧を示します。`--write` のときだけ書き換え、その前に `--dir` が git の作業ツリーで、書き換えるファイルにコミットしていない変更がないことを確かめます(あれば止める)。
7. **4種類の変更(Q7 A):** すべて反映します。キー名と値はその場で差し替え、削除は行(か要素)を消します。必須タグの追加は、タグの書き場所が文字列だけで書いてあればそこに足し、なければ「手で直す」に出します。
8. **AWS(Q8 A):** 呼びません。認証情報も要りません。
9. **終了コードと結果(Q9 A):** すべて反映できたら 0、「手で直す」か「場所を決められない」が1件でもあれば 2、変更案やファイルが読めない・git の確かめに失敗したなどは 1。結果には、ファイルごとの反映した件数、手で直す一覧(リソース、キー、ファイルと行、理由)、IaC で管理されていないリソースの件数を出します。

Does this all look correct before I generate the artifact?

[Answer]: Looks correct
