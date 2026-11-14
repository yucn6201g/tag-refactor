# 実現性と制約の質問: iac への CDK の反映とプルリクエストの作成

前の intent(`261019-tag-refactor-mvp`)の実現性の段階で、次のことは決まっています。ここでは聞き直しません。

- 実装の言語は Go で、単一のバイナリで配る(前の intent の C-T1)
- 開発は依頼者1人で、範囲と優先度も1人で決める(前の intent の C-O1。今回の意図の文書の Q6 でも同じ)
- 期限は決めない(前の intent の C-O2)
- 検証に使えるのは個人の検証用アカウントだけ(前の intent の C-O4)
- 規制や社内規程の要件はなく、OSS として公開する(前の intent の C-R1)

今回の2つの機能(CDK のソースへの反映、GitHub のプルリクエストの作成)に特有の制約だけを聞きます。

## Q1. CDK のソースを読むための前提

CDK のソース(TypeScript と Python)のタグを書き換えるには、ソースを読んで、タグを付けている箇所を見つける必要があります。利用者の端末に、Go のバイナリ以外のものが入っていることを前提にしてよいですか?

A. 前提にしない。tag-refactor のバイナリだけで、CDK のソースを読んで書き換える(CDK を使う人の端末には Node.js や Python があるが、それにも頼らない)
B. CDK を使う人の端末には Node.js(TypeScript の場合)や Python(Python の場合)が必ずあるので、それを呼んでソースを読んでよい
C. A を基本にし、A で読めない書き方のときだけ B を使う
D. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: B

## Q2. CDK のリソースとソースの対応の見つけ方

CDK が作ったリソースの論理 ID には生成された文字(ハッシュ)が付くので、そのままではソースのどこで作られたかが分かりません。`cdk synth` の出力(`cdk.out` のフォルダー)には、リソースとソースの中の場所(コンストラクトのパス)の対応が入っています。どれを前提にしますか?

A. 利用者が `cdk synth` を自分で実行し、その出力(`cdk.out`)をオプションで渡す。tag-refactor は `cdk` を動かさない(前の intent で、`terraform show -json` の出力を利用者に渡してもらったのと同じ考え方)
B. tag-refactor が `cdk synth` を自分で実行する(利用者の端末に Node.js と CDK が要る)
C. `cdk.out` を使わず、ソースだけから推定する
D. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: B

## Q3. プルリクエストを作る方法と認証情報

前の intent の非機能要件 NFR8 で、認証情報を表示、記録、ログに出さないと決めています。プルリクエストを作るときの GitHub の認証は、どうしますか?

A. 利用者の GitHub CLI(`gh`)を呼び、`gh` がすでに持っている認証を使う。tag-refactor はトークンに触れない(`git push` も利用者の `git` を使う)
B. GitHub の API を tag-refactor が直接呼び、トークンは環境変数(`GITHUB_TOKEN` など)から読む。`git push` は利用者の `git` を使う
C. A を基本にし、`gh` がないときは B を使う
D. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: A

## Q4. GitHub を使うテストの範囲

チームの規則では、CI から実際の AWS アカウントは呼びません。GitHub についてはどうしますか?

A. CI では GitHub の呼び出しを差し替えてテストする。実際の GitHub にプルリクエストを作る結合テストは、テスト用のリポジトリを使って手元だけで実行する
B. CI でも、テスト用のリポジトリに実際のプルリクエストを作る
C. 実際の GitHub を使う結合テストはしない(差し替えたテストだけ)
D. まだ決めていない
X. Other (please specify)

[Answer]: B

## Q5. CDK を使うテストの範囲

書き換えたソースで、CDK が本当に変更後のタグを作るかを確かめるには、`cdk synth` を動かすテストが要ります。それには Node.js、Python、CDK のライブラリが要ります。

A. CI の3つの OS(Linux、macOS、Windows)すべてで、Node.js と Python と CDK を入れて `cdk synth` のテストも動かす
B. `cdk synth` のテストは CI の Linux だけで動かし、ほかの OS は書き換えのテストだけにする
C. `cdk synth` のテストは手元だけで実行し、CI は書き換えのテストだけにする
D. `cdk synth` を動かすテストはしない
X. Other (please specify)

[Answer]: A

## Q6. 新しい依存ライブラリ

CDK のソースの解析や GitHub の API のために、Go の依存ライブラリが増えるかもしれません。チームの規則では、依存の脆弱性を `govulncheck` で確かめ、更新は Dependabot で行います。

A. 必要なら足してよい(ライセンスと脆弱性を確かめる)
B. 標準ライブラリと、今ある依存だけで作る
C. 足してよいが、cgo が要るものは使わない(今の CI は cgo なしでもテストしている)
D. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q7. 公開の時期

期限は決めていません(前の intent の C-O2)。この2つの機能を公開する区切りは、どうしますか?

A. 2つとも終わったら、`v0.2.0` として公開する(タグを付けるのは人)
B. 1つずつ、終わったものから公開する
C. 公開の時期は後で決める
D. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q8. ほかの作業との重なり

今、同じリポジトリで、README の `plan` の注意書き、前の intent の記録の更新、Terraform の例の照合など、別の作業も進んでいます(`internal/iac/testdata` に触れる作業を含みます)。

A. 気にしない。重なったら、マージのときに直す
B. 別の作業が `main` に入ってから、この作業のコードに取りかかる
C. この作業では、別の作業が触るファイル(README の `plan` の節、前の intent の記録、`internal/apply`、`scripts/`、`internal/iac/testdata`)を変えない
D. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q9. `cdk synth` と AWS の認証情報(Q2 の回答の確認)

Q2 で、tag-refactor が `cdk synth` を自分で実行すると決めました。CDK のアプリが AWS から値を引く書き方(`Vpc.fromLookup` など)を使っていて、その値が `cdk.context.json` に保存されていないと、`cdk synth` は AWS の認証情報を使って AWS を呼びます。一方、今の `iac` は「AWS の認証情報も要らない」と README に書いています。どうしますか?

A. `cdk synth` には AWS の認証情報を渡さない。AWS を呼ばないと synth できないアプリでは、止めて理由と対処(`cdk.context.json` を用意するなど)を示す。`iac` は今までどおり認証情報が要らない
B. 利用者の AWS の認証情報をそのまま `cdk synth` に渡してよい。CDK を対象にするときは、`iac` も認証情報が要ることがあると README に書く
C. A を基本にし、利用者が自分で `cdk synth` した出力(`cdk.out`)を渡す道も用意する
D. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: A

## Q10. CI で実際の GitHub を使うテストの準備(Q4 の回答の確認)

Q4 で、CI でも実際の GitHub にプルリクエストを作ると決めました。そのためには、書き込みのできるトークンを CI に置く必要があります。また、fork からのプルリクエストの CI には、GitHub の仕組みで secret が渡りません。どうしますか?

A. テスト専用のリポジトリを1つ作り、そのリポジトリにだけ書ける fine-grained トークンを CI の secret に置く。secret のない実行(fork からのプルリクエストなど)では、その結合テストを飛ばす
B. GitHub Actions が自動で用意する `GITHUB_TOKEN` を使い、このリポジトリ自身にプルリクエストを作ってすぐ閉じる
C. A と同じだが、結合テストは `main` への push のときだけ動かす(プルリクエストの CI では動かさない)
D. まだ決めていない(設計で決める)
X. Other (please specify)

[Answer]: A(利用者の回答「任せる」を受けて選んだ。プルリクエストの CI でも動き、書き込めるのはテスト専用のリポジトリだけになるため)

## Consolidated Summary Confirmation

- CDK のソースを読むために、利用者の端末の Node.js(TypeScript)と Python(Python)を呼んでよい(Q1: B)
- CDK のリソースとソースの対応は、tag-refactor が `cdk synth` を自分で実行して得る。利用者の端末に Node.js と CDK が要る(Q2: B)
- `cdk synth` には AWS の認証情報を渡さない。AWS を呼ばないと synth できないアプリでは、止めて理由と対処を示す。`iac` は今までどおり認証情報が要らない(Q9: A)
- プルリクエストは利用者の `gh` を呼んで作り、`gh` の認証を使う。tag-refactor はトークンに触れない。push も利用者の `git` を使う(Q3: A)
- CI でも実際の GitHub にプルリクエストを作るテストを動かす(Q4: B)。テスト専用のリポジトリと、そこにだけ書ける fine-grained トークンを CI の secret に置き、secret のない実行では飛ばす(Q10: A、任された選択)
- `cdk synth` を動かすテストは、CI の3つの OS すべてで動かす(Q5: A)
- Go の依存ライブラリは、必要なら足してよい(ライセンスと脆弱性を確かめる)(Q6: A)
- 2つの機能がそろったら、`v0.2.0` として公開する。タグを付けるのは人(Q7: A)
- 同じリポジトリのほかの作業との重なりは気にせず、マージのときに直す(Q8: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
