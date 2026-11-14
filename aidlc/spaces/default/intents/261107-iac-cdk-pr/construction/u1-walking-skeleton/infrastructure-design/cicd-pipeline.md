# CI と配布の流れ: U1 最小の一本

元にしたものは次のとおりです。出典の ID-Q… は、このステージの `infrastructure-design-questions.md` の回答です。

- CI の設計: `construction/u1-walking-skeleton/nfr-design/logical-components.md`
- トークンの扱い: `construction/u1-walking-skeleton/nfr-design/security-design.md` の6節
- テストの仕組み: `construction/u1-walking-skeleton/nfr-design/reliability-design.md`

配布(タグの push での公開)は、今の `release.yml` のままで、U1 では変えません。

## 1. プルリクエストごとの CI(`ci.yml`)

今の仕事に、次のとおり足します。アクションはすべてコミットのハッシュで固定し、版をコメントに書きます。`actions/checkout` は `persist-credentials: false` にします。ワークフロー全体の権限は今のまま `contents: read` です。

| 仕事 | OS | 上限 | 足すこと | ci-gate |
|---|---|---|---|---|
| test | Linux、macOS、Windows × Go の stable と oldstable | 30 分(15 分から上げる) | setup-node(Node.js 22 の版を固定、npm のキャッシュ)、テスト用のアプリのフォルダーごとの `npm ci --ignore-scripts` | 待つ(今と同じ) |
| race | Linux | 30 分 | test と同じ setup-node と npm ci | 待つ |
| coverage | Linux | 30 分 | test と同じ setup-node と npm ci | 待つ |
| lint、vuln、modcheck、build | 今と同じ | 今と同じ | なし(Go のテストを動かさない) | 待つ |
| prettier(新規) | Linux | 10 分 | checkout、setup-node、`npm exec --yes prettier@<Code Generation のときの最新の安定版> -- --check internal/cdk/helper/`。権限は `contents: read` だけ | 待つ |
| github-pr(新規) | Linux | 20 分 | checkout、setup-go、setup-node、テスト用のアプリの npm ci、`go test -run '^TestGitHubPullRequest$' ./internal/cli/`。この手順の `env` にだけ secret と変数を渡す。権限は `contents: read` だけ(テスト専用のリポジトリへの書き込みは secret のトークンで行う) | 待つ |

github-pr は、secret が空(fork のプルリクエスト、Dependabot)ならテストを飛ばして成功します。そのため `ci-gate` が待っても、secret の無いプルリクエストは止まりません。

## 2. 段階と合否

| 段階 | 確かめるもの | 止める条件 |
|---|---|---|
| 整形と lint | gofmt、golangci-lint(lint の仕事)、Prettier(prettier の仕事) | 1つでも違反 |
| テスト | 受け入れ条件のテスト、単体テスト、3つの OS での synth を動かすテスト、ベンチマーク | 1つでも失敗。道具が無いときも失敗(CI) |
| race | Linux の `-race` | 失敗 |
| カバレッジ | 80% 以上 | 80% 未満 |
| 脆弱性 | govulncheck | 見つかった |
| 結合テスト | テスト専用のリポジトリへの実際のプルリクエスト | 失敗、または後片付けの失敗 |
| ビルド | 6つの OS と CPU の組み合わせ | 失敗 |

## 3. 依存の更新(Dependabot)

`.github/dependabot.yml` に npm の行を足します(ディレクトリ `/internal/cdk/testdata/apps/ts-minimal`、毎週、接頭辞 `chore(deps)`)。今の gomod と github-actions の行は変えません。

## 4. 配置とやり直し

- この道具に本番の配置はありません。`main` への squash マージのたびに、今の artifacts の仕事が確認用のバイナリを作ります(公開しない)。
- 公開は、人が `vX.Y.Z` のタグを push したときだけで、今の `release.yml` のままです。`v0.2.0` は B6 のあとに出します。
- やり直しは、問題のコミットを revert するプルリクエストで行います。公開した版を取り下げるときは、人が GitHub Releases の版を消し、直した版に新しいタグを付けます。

## 5. 検証用のスタックの手順

検証用のスタックの置き方は次のとおりです。手順はテスト専用のリポジトリの README に書きます。

1. テスト専用のリポジトリを手元に clone し、アプリのフォルダーで `npm ci` を実行する。
2. `npx cdk synth --output <一時的なフォルダー>` でテンプレートを作る。このとき AWS の認証情報は要らない。
3. 手元のテンプレートの中身(作るリソース、タグ、DeletionPolicy)と、これから動かす命令を人に見せ、明示の許可を得る。変更セットの作成も AWS の変更(スタックが REVIEW_IN_PROGRESS の状態で作られる)なので、許可の前には作らない。
4. 許可を得たら、`aws cloudformation deploy --stack-name tag-refactor-cdk-skeleton --template-file <テンプレート> --profile tag-refactor-apply --region ap-northeast-1` で置く。置いたあと、`describe-stacks` と、バケットのタグ(`Env=dev` と `aws:cloudformation:*`)を読んで確かめる。
5. 片付け(`v0.2.0` のあと)も、消すものを見せ、人の許可を得てから `aws cloudformation delete-stack` で行う。

## 6. secret の扱い

| secret と変数 | 置き場所 | 渡す範囲 | 作り直し |
|---|---|---|---|
| `TAG_REFACTOR_TEST_REPO_TOKEN` | tag-refactor のリポジトリの Actions の secret | github-pr の仕事の結合テストの手順の `env` だけ | 90 日ごとに人が作り直す |
| `TAG_REFACTOR_TEST_REPO` | tag-refactor のリポジトリの Actions の変数 | 同じ手順の `env` だけ | 要らない |

どちらも、ワークフローの定義、テストのデータ、コミットするファイルには値を書きません(プロジェクトの禁止事項)。AWS の認証情報は CI に置きません(CI から実際の AWS は呼ばない)。
