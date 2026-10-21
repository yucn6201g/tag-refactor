# Team-Level Rules

> This team's affirmed practices and corrections. Loaded after `org.md` as
> strict-additive guidance; contradictions with broader policy are rejected.
> Populated by the practices-discovery affirmation gate. Edit at the gate,
> not directly.

## Way of Working
私たちはトランクベース開発で進めます。

- すべての作業は、短命なブランチ(目安は1〜2日)を経由して `main` に統合します。`develop` やリリース用ブランチのような、長く残るブランチは作りません。
- Construction の作業ブランチ(worktree)は `main` から作り、`main` に戻します。
- ブランチは GitHub に push し、プルリクエストで `main` に squash マージします。マージの前に CI を実行し、CI が通ったものだけをマージします。
- 1つの Bolt は、`main` 上の1つのコミットになります。途中のコミットは `main` には残りませんが、AI-DLC の監査ログに作業の経過が残るので、この点は受け入れます。
- コミットメッセージは Conventional Commits(`feat:`、`fix:` などの接頭辞)に従います。
- リリースの区切りは、ブランチではなくタグ(`vX.Y.Z`)で表します。

## Walking Skeleton
最初に、端から端まで動く最小の一本(walking skeleton)を作ります。

- 最初の Unit を、この最小の一本にします。後の Unit より先に、Code Generation まで作り切ります。
- 最小の一本の中身は、`scan` の最小版(1つのリージョンのタグを読み取り、表にして出す)と、`plan`(影響分析)の最小版です。ビルド、テスト、lint、CI も、この一本に最初から含めます。
- 最小の一本が動いたことは、人が承認した Construction 検証コマンド(Construction Verification Command)で確かめます。このコマンドは、個人の検証用 AWS アカウントを実際に呼びます。AWS の認証情報と少額の費用が必要です。
- 検証のあと、最小の一本のチェックポイントで人の承認を得ます。設計段階のレビューを通っただけでは、最小の一本が動いたことにはしません。
- 最小の一本の承認後に、残りの Construction を自動で続けるか(`autonomous`)、チェックポイントごとに確認するか(`gated`)を選びます。

## Testing Posture
私たちはテストを、すべての Bolt で作る成果物の一部として扱います。

- **Methodology**: custom
- **Methodology evidence**: Q6 の回答は C(受け入れ条件(Given/When/Then)を先にテストにし、細かい単体テストは実装のあとに書く)です。テストを先に書く部分と、実装のあとに書く部分が混ざっているため、`tdd`、`bdd`、`atdd`、`test-after` のどれにも当てはまらず、`custom` とします。
- **Ordering**: まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。
- カバレッジの下限は80%です。Go の標準の測り方(`go test -cover` の文(statement)単位)で測ります。コマンドの入口などの配線部分も、計測の対象から外しません。
- マージの前に、プルリクエストの CI でテストを実行します。
- AWS を呼ぶ部分の単体テストでは、AWS の呼び出しを差し替え、AWS の認証情報なしで CI で実行します。実際のアカウントを使う結合テストは、手元だけで実行します。CI から実際の AWS アカウントは呼びません。
- CI のテストは、Linux、macOS、Windows の3つの OS で実行します。
- テストの量と種類は、ワークフローのテスト戦略(Test Strategy)で決まります。上の下限は、それに加わる条件です。下限を下げて Build and Test を通すことはしません。

## Guard Policy

<!-- Affirmed by the team. Mode: strict, relaxed, or off. Strict here holds for every intent, whatever one person asks in chat; relaxed or off here applies to every intent whose policy came from its scope, unless project.md or the person sets another; changing this line changes it. A section under the retired Change Control heading, written by an earlier release, is still read. -->

## Deployment
このツールは利用者が自分の端末で動かす CLI で、サーバーには配置しません。リリースは次のように進めます。

- `main` に取り込むたびに、CI が確認用のバイナリをビルドします。このバイナリは公開しません。
- 公開するのは、人が `vX.Y.Z` のタグを付けて push したときだけです。タグを付ける操作が、公開に対する人の承認にあたります。`main` に取り込むたびに自動で公開することはしません。
- 配り方は、GitHub Releases(OS ごとのバイナリ)と `go install` の2つです。
- 配るバイナリには、チェックサムを付けます。署名とビルドの来歴の証明は付けません。
- 最初に公開するのは `v0.1.0` で、第1弾(`scan` と `plan`)が終わった時点で出します。

## Code Style
私たちは、リポジトリのルートに置く設定ファイルに従います。

- 整形には `gofmt` だけを使います。`goimports` や `gofumpt` は使いません。
- lint には `golangci-lint` を使い、既定の linter だけを有効にします。CI でマージの前に実行し、失敗したらマージしません。
- `.gitattributes` で Go のファイルの改行コードを LF に固定します。
- 依存ライブラリの既知の脆弱性を、CI で `govulncheck` を使って確認します。依存ライブラリの更新には Dependabot を使います。
- 名前の付け方は Go の慣習に従います。プロジェクト独自の名前の規則は、チームで決めない限り設けません。
- AI のエージェントがコードの書き方を提案するときは、先にプロジェクトの lint の設定を読み、lint で扱っていない点についてだけ提案します。
## Forbidden

<!-- Team-specific forbidden patterns -->

## Mandated

<!-- Team-specific mandates -->

## Corrections

<!-- Self-learning loop appends here. -->
