# 機能の設計の質問: U1 最小の一本

U1 は、CDK の反映とプルリクエストの作成を端から端まで細く通す最初の単位です(`inception/units-generation/unit-of-work.md`)。

U1 の受け入れ条件は、`unit-of-work-story-map.md` に従って次のとおりです。

- AC1.1.1(最小の形)、AC1.1.11(CI の実際のプルリクエスト)
- AC2.1.2、AC2.1.3、AC2.1.6 の最小の形(アプリ1つ)、AC2.1.7(CI)
- AC3.1.1 の一部(書き方 (1) のキーの名前の変更)

部品と契約は `components.md` と `contract-summary.md` で決まっています。ここでは、U1 の中の流れと決まりのうち、まだ決まっていないものを聞きます。

## Q1. CDK を今の iac の流れにどう入れるか

今の `iac` は、ファイルごとに「場所を探す → 書き換えを作る → 読み直して確かめる → 差分 → 書き出す」の順で進みます(`internal/iac/run.go`)。

A. CDK のソースのファイルも、同じファイルごとの流れに入れる。読み取りだけを CDK 用にし、補助のプログラムの結果(アプリごとに1回)からそのファイルのタグの箇所を作る。書き換え後の確かめは、アプリごとにまとめて Recheck を1回呼ぶ。差分、書き出し、カウントは今のものをそのまま使う
B. CDK だけ別の流れを作る
C. 任せる
X. Other (please specify)

[Answer]: A

## Q2. U1 での `Tags.of(x)` の効く範囲の扱い

`Tags.of(bucket).add(...)` は、`bucket` のコンストラクトとその下のすべてのリソースにタグを付けます。U1 では、範囲の規則の全体(FR3.5、U4)はまだ作りません。

A. タグの箇所の範囲(ScopeConstructPath)が対象のリソースのコンストラクトのパスを含み、その範囲の下で同じキーのタグを持つ synth のリソースが対象の1つだけなら書き換える。2つ以上なら書き換えず、手で直す一覧(cdkScopeTag)に出す
B. 範囲が対象のリソースの直接の親(L2 のコンストラクトの既定の子 `Resource`)のときだけ書き換え、それ以外は手で直す一覧に出す
C. 任せる
X. Other (please specify)

[Answer]: A

## Q3. テスト用の CDK のアプリを置く場所

`internal/iac/testdata` は、ほかのセッションが触っているので使いません。

A. `internal/cdk/testdata/apps/<名前>/` に置く(TypeScript の最小のアプリ `ts-minimal` から始める)。npm の依存は `package.json` と `package-lock.json` で版を固定し、Dependabot の対象にする
B. リポジトリの根の `testdata/cdk/` に置く
C. 任せる
X. Other (please specify)

[Answer]: A

## Q4. TypeScript 用の補助のプログラムの書き方

補助のプログラムは、利用者の Node.js で動き、利用者のアプリの `node_modules/typescript` でソースを読みます(ADR-003、契約 C4)。

A. ビルドの要らない JavaScript(CommonJS の1ファイル)で書く。Go の embed でバイナリに入れ、実行のたびに一時的なフォルダーに書き出して `node` で動かす。`typescript` は、アプリのフォルダーから探して読み込む。整形は Prettier で確かめる
B. TypeScript で書き、Go のビルドの前に JavaScript に変換する(ビルドに Node.js が要る)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q5. 検証用の最小の CDK アプリの置き方と片付け方

U1 の検証では、検証用の AWS アカウントから `plan` で変更案を作り、`iac --pr` でテスト専用のリポジトリにプルリクエストを作ります。

A. テスト専用のリポジトリの `main` に、TypeScript の最小の CDK アプリ(S3 のバケット1つに `Tags.of(bucket).add('Env', 'dev')`)を置く。手元からそのアプリを検証用のアカウントにスタック `tag-refactor-cdk-skeleton` として置く(変更を見せて人の明示の許可を得てから)。`plan` では、キー `Env` を `Environment` に変えるタグの規則を使う。スタックは `v0.2.0` を出すまで残し、そのあと人の許可を得て消す
B. ほかの置き方にする(Other に書く)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q6. `--pr` のコミットのメッセージ

A. プルリクエストの既定のタイトルと同じ文(`Reflect tag changes from tag-refactor (<変更案のファイル名>)`、`--lang` に従う)を1行目にする。`--pr-title` を付けたときは、その文を使う
B. Conventional Commits の形(`chore: reflect tag changes from tag-refactor`)に固定する
C. 任せる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- CDK のソースのファイルも、今の iac のファイルごとの流れ(場所、書き換え、確かめ、差分、書き出し)に入れる。読み取りだけを CDK 用にし、補助のプログラムはアプリごとに1回、書き換え後の確かめもアプリごとに Recheck を1回呼ぶ(Q1: A)
- U1 では、`Tags.of(x)` の範囲の下で同じキーのタグを持つ synth のリソースが対象の1つだけなら書き換え、2つ以上なら cdkScopeTag で手で直す一覧に出す(Q2: A)
- テスト用の CDK のアプリは `internal/cdk/testdata/apps/<名前>/` に置き、npm の依存は版を固定して Dependabot の対象にする。最初は `ts-minimal`(Q3: A)
- TypeScript 用の補助のプログラムは、ビルドの要らない JavaScript の1ファイルで書き、バイナリに埋め込み、実行のたびに一時的なフォルダーに書き出して `node` で動かす。`typescript` はアプリのフォルダーから探す。整形は Prettier(Q4: A)
- 検証用には、テスト専用のリポジトリの `main` に TypeScript の最小の CDK アプリを置き、検証用のアカウントにスタック `tag-refactor-cdk-skeleton` として置く(人の明示の許可を得てから)。`plan` では `Env` を `Environment` に変える規則を使う。スタックは `v0.2.0` まで残し、そのあと許可を得て消す(Q5: A)
- `--pr` のコミットのメッセージは、プルリクエストのタイトルと同じ文にする(Q6: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
