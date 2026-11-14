# 設計の決定(ADR): iac への CDK の反映とプルリクエストの作成

部品は `components.md`、要件は `requirements.md`、ストーリーは `stories.md`、今の構成はコードの記録の `architecture.md` と `component-inventory.md` です。出典の DD-Q… はこのステージの `domain-design-questions.md` の回答で、4つとも利用者に任されて選んだものです。

## ADR-001: CDK とプルリクエストを新しいパッケージに分ける

## Status: Accepted

### Context

今の `internal/iac` は約 4,700 行で、CloudFormation と Terraform の読み取り、場所の決定、書き換え、git の読み取りの確かめを持ちます(`component-inventory.md`)。今回、`cdk synth` と補助のプログラムという外のプログラムの呼び出しと、git の書き込みと gh の呼び出しが加わります(要件 FR1、FR2)。

### Decision

新しいパッケージを2つ作ります。`internal/cdk`(CdkSynthesizer と CdkSourceAnalyzer)と `internal/pullrequest`(PullRequestPublisher)です。`internal/iac` は `internal/cdk` を呼び、CDK を3つ目の IaC として扱います(DD-Q1)。

### Consequences

- よい点: 外のプログラムを呼ぶ部分を、パッケージごとに差し替えてテストできる(チームの進め方のカバレッジ 80%)。`internal/iac` の大きさが増えすぎない。プルリクエストの作成を、CDK の有無によらず使える(要件 FR1.2)。
- よくない点: パッケージの間の型(リソースの一覧、タグの箇所)を決める手間が増える。
- 中立: 型は契約の設計の段階で決める。

### Alternatives Rejected

- すべてを `internal/iac` の中に置く: git の書き込みと外のプログラムが `internal/iac` に入り、今の「git は読み取りだけ」の境界が崩れる。
- `internal/cdk` をさらに `synth` と `source` に分ける: 2つの部品の間に呼び出しがなく、どちらも `IacWriter` だけから呼ばれるので、ファイルの分け方で足りる。

### Security and compliance

外のプログラムを呼ぶ部分がパッケージに分かれるので、渡す環境(認証情報を外す)と時間の上限を、それぞれのパッケージの中の1か所で守れる(要件 NFR1〜NFR3)。

## ADR-002: プルリクエストの作成は internal/cli がつなぐ

## Status: Accepted

### Context

今の `internal/iac` は、リポジトリの設定のプログラム(フック、fsmonitor など)が動かないように、`git` の読み取りの操作だけを使います(コードの記録の `architecture.md`)。プルリクエストの作成では、利用者のフックと署名が動きます(要件 FR1.7)。また、確かめは書き換えの前に行う必要があります(要件 FR1.5)。

### Decision

`IacCommand`(`internal/cli`)が、`PullRequestPublisher` の確かめ → `IacWriter` の書き換え → `PullRequestPublisher` のブランチ、コミット、push、作成、の順につなぎます。`IacWriter` は git の読み取りだけのままです(DD-Q2)。

### Consequences

- よい点: `internal/iac` の安全の境界が変わらない。プルリクエストの作成が、どの IaC の書き換えにも同じように使える。
- よくない点: `IacCommand` に順番の管理が加わり、少し大きくなる。
- 中立: 確かめのうち、ネットワークを使わないもの(ブランチ、リモートの決定、入力)を先に、gh とリモートの確かめを次に、synth はそのあとに行う順は、機能の設計で決める。

### Alternatives Rejected

- `IacWriter` の中から `PullRequestPublisher` を呼ぶ: `internal/iac` が git の書き込みを持つことになり、今の境界が崩れる。

### Security and compliance

git の書き込みと gh の呼び出しが `internal/pullrequest` だけに閉じるので、GitHub のトークンを読まない、出力に出さない(要件 NFR1)ことを1か所で確かめられる。

## ADR-003: TypeScript と Python は埋め込んだ補助のプログラムで読む

## Status: Accepted

### Context

CDK のタグの箇所を正しく見つけるには、TypeScript と Python の構文を読む必要があります。配るバイナリは cgo なしです(チームの進め方)。利用者の端末の Node.js と Python を呼んでよいと決めています(実現性の制約 C-T2)。

### Decision

tag-refactor に小さな補助のプログラム(TypeScript 用と Python 用)を埋め込み、利用者の Node.js と Python で動かします。TypeScript はアプリの `node_modules` の TypeScript のコンパイラーで、Python は標準の `ast` で読みます。補助のプログラムは読むだけで、タグの箇所と位置を JSON で返し、書き換えは Go が行います(DD-Q3)。

### Consequences

- よい点: それぞれの言語の正式な読み取りを使うので、書き方の多様さ(危険 R1)に強い。cgo が要らない。
- よくない点: アプリに TypeScript のコンパイラーが無いと読めない(TypeScript のアプリでは、ふつうは入っている)。補助のプログラムの整形を CI で確かめる必要がある(チームの進め方: Prettier と Black)。補助のプログラムのライセンスの表示を、配るアーカイブに含める必要がある。
- 中立: 補助のプログラムと Go の間の JSON の形は、契約の設計の段階で決める。

### Alternatives Rejected

- Go だけで読む: TypeScript と Python の構文の正しい読み取りを Go で書くのは大きく、簡易な読み取りでは定数や引数をたどる書き方(要件 FR3.3)で誤りやすい。
- cgo の解析器(tree-sitter など)を使う: 配るバイナリは cgo なしと決めているため使えない。

### Security and compliance

補助のプログラムは利用者のソースを読むだけで、書かない。利用者の Node.js と Python で動くので、tag-refactor が新しい実行の権限を持つことはない。補助のプログラムに渡す環境からも、AWS と GitHub の認証情報を外す。

## ADR-004: リソースとソースの対応はコンストラクトのパスをたどり、決まらないときだけ作成の記録で補う

## Status: Accepted

### Context

CDK のリソースの論理 ID には生成された文字が付き、ソースには出てきません。`cdk synth` の出力には、リソースごとにコンストラクトのパス(`aws:cdk:path`)が入っていますが、ソースのファイルと行は、ふつうは入っていません(実現性の段階の F-Q2)。

### Decision

コンストラクトのパスをたどり、補助のプログラムがソースの中の作成の箇所(`new X(scope, 'Id')` など)を探して、ファイルと行を決めます。決まらないときだけ、CDK の作成の箇所の記録を出す設定で synth し直して補います(DD-Q4)。

### Consequences

- よい点: ふだんは追加の設定なしで済む。作成の記録は重く、出力も大きいので、必要なときだけにできる。
- よくない点: パスをたどる仕組みと、記録から読む仕組みの2つを持つ。2回目の synth の時間がかかることがある。
- 中立: 作成の箇所の記録の設定の名前と形は、機能の設計で CDK v2 の文書に照らして確かめる。

### Alternatives Rejected

- いつも作成の記録を出して synth する: 時間と出力の大きさが増える。
- パスをたどるだけにする: コンストラクトの ID を変数で作るアプリなどで、対応が取れないものが増える。

### Security and compliance

2回目の synth も、1回目と同じく AWS と GitHub の認証情報を渡さず、時間の上限を置く(要件 NFR2、FR2.5)。

## ADR-005: CDK の書き換えは今の反映の仕組みに載せる

## Status: Accepted

### Context

今の `internal/iac` には、変更のメモリの上での適用、範囲の書き換え、書き換えたあとの読み直し、差分、一時ファイルでの置き換えの仕組みがあります(コードの記録の `architecture.md`)。CDK でも同じ決まりを守る必要があります(要件 FR3.6〜FR3.9、NFR4)。

### Decision

補助のプログラムが返すタグの箇所(キーと値の範囲)を、`IacWriter` が今の反映の仕組みに渡し、同じ決まりで書き換えます。書き換えたあとの読み直しには、補助のプログラムをもう一度使います。

### Consequences

- よい点: 差分、反映済みの数え方、`aws:` のキーの扱い、置き換えの安全さが、3つの IaC でそろう。
- よくない点: 読み直しのたびに補助のプログラムを起動する時間がかかる。
- 中立: 範囲の表し方(バイトの位置、文字の位置)は、契約の設計で決める。

### Alternatives Rejected

- 補助のプログラムが書き換えまで行う: Go の側の安全の仕組み(読み直し、一時ファイル、git の確かめ)を使えず、2つの言語で同じ決まりを書き直すことになる。

### Security and compliance

書き込みは今までどおり Go の側の `fileout` だけで行う。補助のプログラムはファイルを書かない。
