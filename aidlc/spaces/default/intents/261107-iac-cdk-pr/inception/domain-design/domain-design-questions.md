# ドメインの設計の質問: iac への CDK の反映とプルリクエストの作成

要件(`requirements.md`)、ストーリー(`stories.md`)、チームの進め方(`team-practices.md`)、今のコードの構成(`architecture.md`、`component-inventory.md`)をもとに、部品の分け方で選ぶ余地のある4点を聞きます。今の構成は、`internal/cli` がコマンドを受け、`internal/iac` が CloudFormation と Terraform の反映を行い(`git` は読み取りだけ)、`internal/display` が表示します。

## Q1. 新しい部品の置き場所

A. 新しいパッケージを2つ作る。`internal/cdk`(アプリの発見、`cdk synth`、リソースとソースの対応、TypeScript と Python のタグの箇所の読み取り)と `internal/pullrequest`(書き換えの前の確かめ、ブランチ、コミット、push、作成)。`internal/iac` は `internal/cdk` を呼んで CDK を3つ目の IaC として扱う
B. すべて `internal/iac` の中に置く(ファイルを分けるだけ)
C. A と同じだが、`internal/cdk` をさらに `synth`(synth と対応)と `source`(ソースの読み取り)に分ける
D. 任せる
X. Other (please specify)

[Answer]: A(利用者の回答「任せる」を受けて選んだ。CDK とプルリクエストは変わる理由が別で、今の internal/iac の大きさ(約 4,700 行)にこれ以上載せないため)

## Q2. プルリクエストの作成を呼ぶ場所

今の `internal/iac` は、`git` の読み取りの操作だけを使い、書き込みはしません(コードの記録)。

A. `internal/cli` が呼ぶ。`pullrequest` の確かめ → `iac` の書き換え → `pullrequest` のブランチ、コミット、push、作成、の順に `internal/cli` がつなぐ。`internal/iac` は git の読み取りだけのまま
B. `internal/iac` の `Run` の中から `pullrequest` を呼ぶ
C. 任せる
X. Other (please specify)

[Answer]: A(利用者の回答「任せる」を受けて選んだ。internal/iac の git は読み取りだけという今の境界を守るため)

## Q3. TypeScript と Python のソースの読み方

実現性の段階で、利用者の端末の Node.js と Python を呼んでよいと決めています(C-T2)。配るバイナリは cgo なしです(チームの進め方)。

A. tag-refactor に小さな補助のプログラム(TypeScript 用と Python 用)を埋め込み、利用者の Node.js と Python で動かす。TypeScript はアプリの `node_modules` の TypeScript のコンパイラーで、Python は標準の `ast` で読み、タグの箇所と位置を JSON で返す。書き換えそのものは Go が行う
B. Go だけで読む(TypeScript と Python の簡易な読み取りを Go で書く)
C. 任せる
X. Other (please specify)

[Answer]: A(利用者の回答「任せる」を受けて選んだ。実現性の段階の C-T2 に沿い、cgo なしで両方の言語を正しく読めるため)

## Q4. リソースとソースの場所の対応の取り方

`cdk synth` の出力には、リソースごとにコンストラクトのパス(`aws:cdk:path`、例: `OrdersStack/Orders/Resource`)が入っています。ソースのファイルと行は、ふつうは入っていません。

A. コンストラクトのパスをたどり、補助のプログラムがソースの中の `new X(scope, 'Orders')` のような作成の箇所を探して、ファイルと行を決める
B. CDK の、作成の箇所の記録を出す設定(デバッグの情報)を付けて synth し、出力からファイルと行を得る
C. A を基本にし、A で決まらないときだけ B を使う
D. 任せる
X. Other (please specify)

[Answer]: C(利用者の回答「任せる」を受けて選んだ。ふだんは追加の設定なしで済み、決まらないときだけ記録の情報で補えるため)

## Consolidated Summary Confirmation

- 新しいパッケージを2つ作る: `internal/cdk`(アプリの発見、`cdk synth`、リソースとソースの対応、TypeScript と Python のタグの箇所の読み取り)と `internal/pullrequest`(書き換えの前の確かめ、ブランチ、コミット、push、作成)。`internal/iac` は `internal/cdk` を呼んで CDK を3つ目の IaC として扱う(Q1: 任された選択で A)
- プルリクエストは `internal/cli` がつなぐ(確かめ → 書き換え → ブランチ、コミット、push、作成)。`internal/iac` は git の読み取りだけのまま(Q2: 任された選択で A)
- TypeScript と Python は、埋め込んだ小さな補助のプログラムを利用者の Node.js と Python で動かして読む(TypeScript はアプリの TypeScript のコンパイラー、Python は標準の `ast`)。タグの箇所と位置を JSON で返し、書き換えは Go が行う(Q3: 任された選択で A)
- リソースとソースの対応は、コンストラクトのパスをたどって補助のプログラムが作成の箇所を探す。決まらないときだけ、CDK の作成の箇所の記録を出す設定で synth して補う(Q4: 任された選択で C)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
