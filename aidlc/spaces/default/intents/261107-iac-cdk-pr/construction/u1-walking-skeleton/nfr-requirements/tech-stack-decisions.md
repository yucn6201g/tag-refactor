# 技術の選択: U1 最小の一本

今の技術の構成は `aidlc/spaces/default/codekb/tag-refactor/technology-stack.md` にあります。U1 では、それに次のものを足します。元になる要件は `inception/requirements-analysis/requirements.md`、契約は `inception/contract-design/contract-summary.md` です。出典の FD-Q… は機能の設計の回答(`construction/u1-walking-skeleton/functional-design/functional-design-questions.md`)、NQ-Q… はこのステージの回答です。

| ID | 選んだもの | 理由 | 比べたもの | 出典 |
|---|---|---|---|---|
| NFR6.2 | Go の新しいパッケージ `internal/cdk`(synth と補助のプログラムの呼び出し)と `internal/pullrequest`(git と gh の呼び出し)。外のプログラムは `os/exec` で、シェルを通さずに呼ぶ | 今の `internal/iac` の `git` の呼び出しと同じ形。cgo が要らない | シェルを通す呼び出し(引用符の扱いで壊れやすい、注入の恐れ) | NFR6、components.md |
| NFR6.3 | TypeScript 用の補助のプログラムは、ビルドの要らない JavaScript(CommonJS の1ファイル)。`go:embed` でバイナリに入れる | Go のビルドに Node.js を要らなくする。配るバイナリは1つのまま | TypeScript で書いて変換する(ビルドに Node.js が要る) | FD-Q4、ADR-003 |
| NFR6.4 | 補助のプログラムが使う TypeScript は、利用者のアプリの `node_modules/typescript`(4.0 以上 5.x まで)。使う API は、ソースの読み取り(`createSourceFile`)と木のたどり方(`forEachChild`)、位置の取り方だけ。範囲の外の版なら、アプリを失敗にして理由を示す | 利用者のアプリと同じ TypeScript で読めば、書き方の解釈が食い違わない。配るバイナリに TypeScript を入れずに済む | TypeScript を同梱する(配るものが大きくなり、ライセンスの表示が要る) | NQ-Q4、ADR-003 |
| NFR6.5 | Node.js は 18 以上。起動の前に `node --version` で確かめ、満たさなければアプリを失敗にして理由を示す | CDK v2 が求める版に合わせる | 版を確かめない(分かりにくい失敗になる) | NQ-Q4 |
| NFR6.6 | `cdk` は、アプリの `node_modules/.bin/cdk` を先に、無ければ PATH の `cdk`(Windows では `cdk.cmd`)を使う。引数は `synth --quiet --no-notices --output <一時的なフォルダー>` | アプリが固定した CLI の版を優先する。`npx` は無いときに取りに行く恐れがある | `npx cdk`(ネットワークから取りに行くことがある) | BR1.4 |
| NFR6.7 | テスト用の CDK のアプリは `internal/cdk/testdata/apps/ts-minimal`。`aws-cdk-lib`、`constructs`、`aws-cdk`(CLI)、`typescript`、`ts-node` を `package-lock.json` で固定する。整形も lint もしない | 書き方をそのまま試すため(チームの進め方) | `internal/iac/testdata`(ほかのセッションが触っている) | FD-Q3 |
| NFR6.8 | 補助のプログラムの整形の確かめは Prettier(CI で `--check`)。版は CI の定義で固定する | チームの進め方(TypeScript と JavaScript は Prettier) | 確かめない | チームの進め方 |
| NFR6.9 | Go の依存は足さない。JSON は `encoding/json`、ハッシュは `crypto/sha256`、プロセスの木を止めるのは OS ごとの標準の方法(Unix はプロセスのグループ、Windows はジョブオブジェクトか `taskkill /T`。`golang.org/x/sys` は今の依存) | 依存を増やさない。ライセンスと `govulncheck` の確かめが要らない | 新しいライブラリ | NFR6 |
| NFR6.10 | CI: GitHub Actions の今の `ci.yml` に、3つの OS で Node.js(`actions/setup-node`、版は CI の定義で固定、npm のキャッシュ)を入れ、テスト用のアプリで `npm ci` を実行する手順を足す。アクションはコミットのハッシュで固定する。新しい仕事は `ci-gate` が待つ仕事に入れる | チームの進め方 | 別のワークフロー(`ci-gate` に入れにくい) | NFR8、チームの進め方 |
