# 証拠: Practices Discovery(第2弾、再実行)

この文書は、リード(`aidlc-pipeline-deploy-agent`)が下書きのために調べたもの、そこから考えたこと、面談(`practices-discovery-questions.md` の Q1〜Q9)で決まったこと、まだ決まっていないことをまとめたものです。今回は協力するエージェントのいない実行(リードだけ)なので、ほかの参加者の所見はありません。面談の結果は「面談の結果」の節にあり、`team-practices.md` と `discovered-rules.md` はその結果に合わせて仕上げました。

## 前提

- 再実行です。`aidlc/spaces/default/memory/team.md` の5つの節(Way of Working、Walking Skeleton、Testing Posture、Deployment、Code Style)は、前の意図 `261019-tag-refactor-mvp` で人が承認した内容です。これを今の基準とし、証拠やこの意図で決まった制約が変化を示すところだけを書き換えました。
- `aidlc/spaces/default/memory/project.md` の `## Mandated` は `None. (affirmed 2026-10-19)` です。`## Forbidden` は空です。
- この意図は `aidlc/spaces/default/intents/261107-iac-cdk-pr` です。`iac` に CDK(TypeScript と Python)のタグの反映と、利用者の `gh` と `git` によるプルリクエストの作成を足し、2つそろったら `v0.2.0` として公開します。スコープは `mvp`(`skeleton: on`)、深さは Standard、テスト戦略は Standard です。

## 調べたもの

| 対象 | 何を見たか |
|---|---|
| リバースエンジニアリングの成果物(`aidlc/spaces/default/codekb/tag-refactor/`) | `code-structure.md`、`technology-stack.md`、`dependencies.md`、`code-quality-assessment.md`、`architecture.md`、`business-overview.md`、`reverse-engineering-timestamp.md`(調査は `9bf1fb6`、2026-11-07T15:37:08Z) |
| git の履歴 | `git log --oneline -30`(`bolt/u8-iac`)、`git log --oneline -15 origin/main`、ブランチとタグの一覧、`bolt/u8-iac` のコミットの日時、`e8c83f9`(#20)と `bolt/u8-iac` の差分 |
| `origin/main` の CI と設定 | `git show origin/main:<file>` で `.github/workflows/ci.yml`、`release.yml`、`scheduled.yml`、`.github/dependabot.yml`、`.gitattributes`、`go.mod`、`README.md` の「開発」と「版の公開(メンテナー向け)」の節 |
| GitHub の設定(読み取りだけ) | `gh pr list`(最近の12件)、`gh release list`、リポジトリのマージの設定、`main` のルールセット(`gh api .../rulesets/25636149`) |
| この意図の決定 | `ideation/feasibility/constraint-register.md`(C-T1〜C-T7、C-Q1〜C-Q4、C-O1〜C-O3、C-R1)、`ideation/feasibility/feasibility-questions.md`(Q1〜Q10 と要約の確認)、`ideation/feasibility/feasibility-assessment.md`、`ideation/scope-definition/scope-document.md`、`aidlc-state.md` |
| ルートの設定ファイル | `.golangci.yml`、`Makefile`、`package.json`、`pyproject.toml`、`.prettierrc`、`.editorconfig` は、どれもリポジトリにありません |

## 実際のリポジトリの状態についての注意

- 今チェックアウトしているブランチ `bolt/u8-iac`(HEAD `9bf1fb6`)のコミットそのものは `origin/main` にありません。ただし、中身は #20 として squash マージ済みです(`e8c83f9` と `bolt/u8-iac` の差分は空)。
- `origin/main`(`67a75ed`)には、その後に #21(`--version` と公開のワークフロー、`v0.1.0` のタグ `e2fffdc`)、#22(rollback の修正と `v0.1.0` の後始末)、Dependabot の #15、#16 が入っています。手元の `main`(`918d535`)は `origin/main` より2つ古い状態です。
- そのため、リバースエンジニアリングの成果物(`code-quality-assessment.md` の「公開の仕組みが見えない」、`architecture.md` の「公開のワークフローがリポジトリにない」)は、今の `origin/main` には当てはまりません。Deployment の節は `origin/main` の `release.yml` を証拠にしました。
- 時刻の記録(`practices-discovery-timestamp.md`)のコミットは、指示どおり HEAD(`9bf1fb6`)です。最後の統合の時刻は 2026-11-08T02:26:20Z(`aidlc engine now`)で、HEAD は変わっていません。CI と公開の証拠は `origin/main`(`67a75ed`)から取りました。

## 領域ごとの所見

この節の「考えたこと」は、下書きのときの考えです。「面談での確認が要ります」とした点の結果は、後ろの「面談の結果」の節にあります。

### Way of Working

証拠:

- `main` の履歴は、プルリクエストごとに1つのコミット(`(#N)` 付き)です。#13 `bolt/u6-apply`、#14 `bolt/u7-rollback`、#20 `bolt/u8-iac` が Bolt のブランチで、ほかに `fix/...`、`claude/...`、Dependabot のブランチがあります。
- リポジトリの設定は、squash マージだけを許し(`allow_squash_merge: true`、merge と rebase は false)、マージしたブランチを自動で消します(`delete_branch_on_merge: true`)。リポジトリは公開(public)です。
- `main` のルールセット(有効)は、削除の禁止、強制の push の禁止、プルリクエストの必須(承認の数は0)、必須のチェック `ci-gate` を定めています。ルールセットのマージの方法の欄は3つとも許していますが、リポジトリの設定で squash だけになっています。
- `bolt/u8-iac` は 2026-11-05 に始まり、2026-11-07 にマージされました(約2日)。短命なブランチの目安(1〜2日)に合っています。
- コミットメッセージは Conventional Commits です。Dependabot は `.github/dependabot.yml` で `chore(deps)`(gomod)と `ci`(github-actions)の接頭辞を使います。
- `bolt/u8-iac` の最後のコミット `9bf1fb6`(AI-DLC の記録)は、#20 の squash に含まれています。AI-DLC の記録をコードと同じプルリクエストで入れていることになります。

考えたこと:

- team.md の内容は証拠と一致しており、そのまま引き継ぎました。
- 証拠から、次の4つを書き足す案にしました(面談での確認が要ります): GitHub の設定による強制(squash だけ、直接の push の禁止、`ci-gate` の必須、ブランチの自動削除)、レビューの承認を必須にしないこと、AI-DLC の記録を同じプルリクエストで入れること、作業ブランチを「GitHub の最新の `main`」から作ること(手元の `main` が古いままのことがあるため)。
- worktree の対応: トランクベース、`--base main --target main --strategy squash` のままです(`branching-strategies.md` の既定と同じ)。

### Walking Skeleton

証拠:

- スコープ `mvp` の `skeleton:` は `on` です(`.claude/scopes/aidlc-mvp.md`)。`aidlc-state.md` は Construction Checkpoints が enabled、Construction Iteration が unit-major です。
- team.md の Walking Skeleton の中身(`scan` と `plan` の最小版、検証用 AWS アカウントを呼ぶ検証コマンド)は、第1弾に固有の内容です。第2弾の対象(CDK の反映とプルリクエストの作成)には当てはまりません。
- 第2弾の `iac` は AWS を呼ばず(`architecture.md`)、保存した変更案(`iac --plan`)を入力にします。CI には、保存した変更案の例(`internal/iac/testdata/sandbox-plan.json`)で `iac` を動かす前例があります。

考えたこと:

- 「最初の Unit を最小の一本にし、人の承認した検証コマンドで確かめ、チェックポイントで承認を得る」という進め方は引き継ぎました。
- 中身は、第2弾向けの案を書きました: TypeScript の最小の CDK アプリで、1つのキー名の変更を `cdk synth` の対応を使って反映し、テスト専用のリポジトリにプルリクエストを作るまで。新しい CI の準備も最初から含めます。これは証拠から決められない案で、面談での確認が要ります。
- 第2弾の検証コマンドは AWS を呼ばない案にしました(C-T4、`iac` は AWS の認証情報が要らない)。代わりに手元の Node.js、CDK の CLI、`gh`、テスト専用のリポジトリが要ります。

### Testing Posture

証拠:

- `ci.yml` の `test` は Linux、macOS、Windows × Go `stable`、`oldstable`、`CGO_ENABLED=0`。`race` は Linux だけで `-race`(cgo を使う)。`coverage` は `-coverpkg=./cmd/tag-refactor/...,./internal/...` で80%以上を求めます。`lint` は gofmt、golangci-lint、`go vet -tags integration`、cfn-lint、`iac` で書き換えた写しの確認。ほかに `vuln`、`modcheck`、`build`、すべての成功を求める `ci-gate`。
- CI は AWS の認証情報を持たず、`permissions: contents: read`、`persist-credentials: false` です。
- コミット `2296959`(「iac の受け入れ条件のテストを実装の前に足す」)は、`custom` の進め方(受け入れ条件のテストが先)が守られている証拠です(`code-quality-assessment.md` も同じことを書いています)。
- 外のプロセスの呼び出しは、今は `internal/iac/git.go` の読み取りだけで、`lookGit`、`runGit` などの差し替えの口でテストしています(`code-structure.md`、`dependencies.md`)。
- この意図の決定: C-Q1(CI から AWS を呼ばない)、C-Q2(CI でも実際の GitHub にプルリクエストを作る。テスト専用のリポジトリと fine-grained トークンを CI の secret に置き、secret の無い実行では飛ばす)、C-Q3(`cdk synth` のテストを CI の3つの OS で動かし、Node.js、Python、CDK を入れる)、C-Q4(80%、受け入れ条件を先に、外のプログラムを呼ぶ部分も差し替えてテスト)、C-T4(`cdk synth` に AWS の認証情報を渡さない)。

考えたこと:

- `Methodology` は `custom`、`Ordering` は team.md のまま引き継ぎました。証拠に変化はありません。
- C-Q2、C-Q3、C-Q4、C-T4 を Testing Posture の箇条に足す案にしました。これらは、この意図の ideation で人が決めたことです(Q4: B、Q5: A、Q9: A。Q10 は「任せる」を受けた選択で、要約の確認で「Looks correct」)。ただし、この意図だけの決定か、チームの続く作業でも守る進め方かは、証拠からは分かりません。面談での確認が要ります。
- 「CI からは外部のサービスを呼ばない」という今の CI の性質(`code-quality-assessment.md`)は、GitHub については C-Q2 で例外になります。AWS についての決まり(CI から呼ばない)は変わりません。
- `ci.yml` にある Go の2つの版と `-race` は team.md に書かれていなかったため、書き足す案にしました。
- 「CI では Node.js、Python、CDK が無いときにテストを飛ばして通さない」は、C-Q3 を確実にするための案です。手元や公開のワークフローでの扱いは決まっていません(下の面談の点 5)。
- GitHub の secret は、fork からのプルリクエストと、Dependabot が起こした実行には渡りません。どちらでも実際の GitHub のテストは飛ぶことになります。

### Deployment

証拠:

- `release.yml`(#21): 起動は `v[0-9]+.[0-9]+.[0-9]+` のタグの push、関係するファイルを変えるプルリクエスト(ドライラン)、`workflow_dispatch`(ドライラン)。タグが `main` の祖先であること、Go の版が `MIN_GO`(go1.27.3)以上であること、バイナリの版(`--version`)がタグと一致すること、作業ツリーが変わっていないことを確かめます。6つのターゲット(linux/amd64、linux/arm64、darwin/amd64、darwin/arm64、windows/amd64、windows/arm64)を `CGO_ENABLED=0`、`-trimpath` でビルドし、`LICENSE`、`README.md`、`THIRD_PARTY_NOTICES.txt` と一緒にアーカイブにし、SHA-256 のチェックサムを作ります。3つの OS でチェックサムと実行を確かめ(`smoke`)、`gh release create --verify-tag --generate-notes` で公開します。署名と来歴の証明はありません(ファイルの先頭のコメントに「チームの決定」とあります)。
- `ci.yml` の `artifacts`: `main` への push のときだけ、5つのターゲットのバイナリを作り、7日だけ残します(公開しない)。
- `gh release list`: `v0.1.0` が 2026-11-07T14:29:27Z に公開済み(Latest)です。タグ `v0.1.0` は `e2fffdc` で、#20(`iac`)のあとなので、`apply`、`rollback`、`iac` も入っています。
- README の「インストール」は GitHub Releases と `go install` の2つです。
- この意図の決定: C-O2(2つの機能がそろったら `v0.2.0`、タグは人が付ける。片方だけでは公開しない)。

考えたこと:

- 「人のタグの push が公開の承認」「GitHub Releases と `go install`」「チェックサムを付け、署名と来歴の証明は付けない」は、証拠と一致するので引き継ぎました。
- 「最初に公開するのは `v0.1.0`」の箇条は済んだことなので、`v0.1.0` を公開した事実と、次の `v0.2.0` の条件(C-O2)に書き換える案にしました。
- `release.yml` が実際に確かめていること(タグが `main` にある、版がタグと一致する、3つの OS での実行の確認)を書き足す案にしました。
- CI の確認用のバイナリが「5つのターゲット」で、公開が「6つのターゲット」(windows/arm64 が公開だけ)という違いがあります。意図した違いかは証拠から分かりません(下の面談の点 8)。

### Code Style

証拠:

- `gofmt -l .` を CI の `lint` で確かめます。`golangci-lint` は v2.15.0(`ci.yml` の環境変数で固定)の既定の linter だけで、設定ファイルはありません。`nolint` のコメントは0件です(`code-quality-assessment.md`)。
- `.gitattributes` は `* text=auto eol=lf` で、Go のファイルに限らず、すべてのテキストのファイルを LF に固定しています。ファジングの種(`**/testdata/fuzz/**`)は `-text` でバイトのままです。team.md は「Go のファイルの改行コードを LF に固定」と書いていて、実際より狭い書き方です。
- `govulncheck` は `go.mod` の `tool` で版を固定し、プルリクエストごと(`vuln`)と毎週(`scheduled.yml`)に動きます。`modcheck` が `go mod verify` と `go mod tidy` の差分なしを確かめます。
- Dependabot は gomod と github-actions を毎週更新します。GitHub Actions のアクションは、どれもコミットのハッシュで固定されています(`technology-stack.md` も同じ)。
- `scripts/third-party-notices.sh` は、バイナリに入る Go のモジュールのライセンスの文から `THIRD_PARTY_NOTICES.txt` を作り、ライセンスのファイルのないモジュールがあると失敗します(README「版の公開」)。
- この意図の決定: C-T7(Go の依存は必要なら足してよい。ライセンスと `govulncheck` を確かめ、更新は Dependabot)、C-T1(単一のバイナリで配る。Node.js や Python の解析の仕組みは外のプログラムとして呼ぶ)。
- TypeScript と Python のファイルは、今のリポジトリにありません。整形と lint の設定(Prettier、ESLint、Black、Ruff など)もありません。

考えたこと:

- `gofmt` だけ、`golangci-lint` の既定の linter、`govulncheck`、Dependabot、Go の慣習の名前、AI のエージェントの提案の決まりは、証拠と一致するので引き継ぎました。
- `.gitattributes` の箇条は、実際の設定(すべてのテキストのファイル)に合わせて直す案にしました。第2弾では TypeScript と Python のテストのデータが増えるので、この範囲は意味を持ちます。
- C-T7 と、アクションのハッシュでの固定を書き足す案にしました。
- テストのデータの CDK アプリを「整形も lint もしない」とした案は、`iac` が書き方を1バイトも変えずに書き換えることを試すため、テストのデータの書き方をそろえないほうがよいという考えからです。証拠ではなく推測なので、面談での確認が要ります。
- tag-refactor 自身が TypeScript や Python の補助のプログラム(バイナリに埋め込んで Node.js や Python で動かすもの)を持つかは、設計で決まります(C-T1 は「外のプログラムとして呼ぶ」とだけ決めています)。持つ場合の整形、lint、ライセンスの表示は、今の team.md にもこの下書きにもありません(下の面談の点 6)。

## team.md との差分

「結果」の列は、面談で決まったことです。「事実の追記」は、証拠どおりの書き足しで、面談で個別に聞かなかったものです。

| 節 | 変えたところ | 根拠 | 結果 |
|---|---|---|---|
| Way of Working | 作業ブランチを「GitHub の最新の `main`」から作る | 手元の `main` が `origin/main` より古い | Q1: A で採用 |
| Way of Working | GitHub の設定による強制(squash だけ、直接の push・強制の push・削除の禁止、`ci-gate` の必須、ブランチの自動削除)を書き足す | リポジトリの設定とルールセット | Q1: A で採用 |
| Way of Working | レビューの承認を必須にしない | ルールセットの承認の数が0、開発は1人(C-O1) | Q1: A で採用 |
| Way of Working | AI-DLC の記録を同じプルリクエストで入れる | `9bf1fb6` が #20 に含まれる | Q1: A で採用 |
| Way of Working | Dependabot の接頭辞を書き足す | `.github/dependabot.yml` | 事実の追記 |
| Walking Skeleton | 進め方を「作業ごとに中身を決める」一般の形にし、第1弾の中身を記録として残し、第2弾の中身と検証コマンドを足す | スコープ `mvp` の `skeleton: on`、C-Q2、C-Q3 | Q2: A、Q3: B(検証コマンドは下書きの「AWS を呼ばない」案から、検証用 AWS アカウントを使う形に変えた) |
| Testing Posture | 外のプログラムを呼ぶ部分も計測し、差し替えてテストする | C-Q4 | Q4: A でチームの進め方にした |
| Testing Posture | `cdk synth` に AWS の認証情報を渡さず、AWS を呼ばないアプリでテストする | C-Q1、C-T4 | Q4: A でチームの進め方にした |
| Testing Posture | CI で実際の GitHub にプルリクエストを作る結合テスト | C-Q2 | Q4: A でチームの進め方にした |
| Testing Posture | Go の2つの版と `-race` | `ci.yml` | 事実の追記 |
| Testing Posture | `cdk synth` のテストを3つの OS で動かし、CI に Node.js、Python、CDK の CLI を入れる | C-Q3 | Q4: A でチームの進め方にした |
| Testing Posture | 道具が無いとき、CI では失敗、手元では理由を出して飛ばす | C-Q3 の確実化の案 | Q5: A で採用 |
| Testing Posture | テストの CDK アプリの依存(npm、pip、CDK の CLI)を固定し、Dependabot で更新する | 面談で新たに聞いた点 | Q6: A で採用 |
| Testing Posture | `Methodology evidence` にコミット `2296959` を足す | git の履歴 | 事実の追記 |
| Deployment | 確認用のバイナリを公開と同じ6つの組み合わせにそろえる(windows/arm64 を足す)、保存は7日 | `ci.yml` の `artifacts`(今は5つ) | Q8: B で採用(`ci.yml` の変更が要る) |
| Deployment | 公開のワークフローが確かめること(タグが `main` にある、Go の版、版の一致) | `release.yml` | Q8: B で採用 |
| Deployment | 「最初は `v0.1.0`」を、公開済みの事実と次の `v0.2.0` の条件に置き換える | `gh release list`、C-O2 | Q8: B で採用 |
| Code Style | `.gitattributes` の範囲を「すべてのテキストのファイル」に直す | `.gitattributes` | 実際の設定に合わせた訂正(面談で個別に聞かなかった) |
| Code Style | Go の依存を足すときの確認(ライセンスと `govulncheck`) | C-T7 | ideation で人が決めたことの書き足し(面談で個別に聞かなかった) |
| Code Style | アクションをコミットのハッシュで固定 | `ci.yml`、`release.yml` | 事実の追記 |
| Code Style | テストのデータの CDK アプリは整形も lint もしない。補助のプログラムを持つ場合は Prettier と Black で整形を確かめる | 推測と面談 | Q7: A で採用 |

## 面談の結果

面談の質問と回答は `practices-discovery-questions.md` にあります。最後の要約の確認の回答は「Looks correct」です。

| 質問 | 回答 | 決まったこと | 反映した先 |
|---|---|---|---|
| Q1. 作業の進め方に書き足す4点 | A | GitHub の設定による強制(squash だけ、`main` への直接の push と強制の push の禁止、`ci-gate` の必須、マージしたブランチの自動の削除)、レビューの承認は必須にしない、AI-DLC の記録はコードと同じプルリクエストで入れる、作業ブランチは GitHub の最新の `main` から作る、の4点すべてを書き足す | `team-practices.md` の Way of Working |
| Q2. 最初に最小の一本を作るか | A | 作る。中身は、TypeScript の最小の CDK アプリで1つのキー名の変更を `cdk synth` の対応を使って反映し、テスト専用のリポジトリにプルリクエストを作るところまで(2つの機能を細く通す)。最小の一本の進め方は一般の形で残し、第1弾の中身は記録として残す | Walking Skeleton |
| Q3. 最小の一本を確かめるコマンド | B | 個人の検証用 AWS アカウントで、実際の AWS から `plan` で変更案を作り、その変更案で `iac --pr` を動かす。AWS の認証情報と少額の費用、手元の Node.js、CDK の CLI、`gh`、テスト専用のリポジトリが要る | Walking Skeleton |
| Q4. CI で実際の GitHub を使うことと、CI に Node.js、Python、CDK を入れること | A | チームの進め方にする。CI でも、テスト専用のリポジトリに fine-grained トークン(CI の secret)で実際のプルリクエストを作る。secret の無い実行(fork、Dependabot)では飛ばす。CI の3つの OS に Node.js、Python、CDK を入れて `cdk synth` のテストを動かす。CI から実際の AWS は呼ばず、`cdk synth` に AWS の認証情報を渡さない | Testing Posture |
| Q5. 道具が無いときの `cdk synth` のテスト | A | CI では失敗にする(飛ばして通さない)。手元では理由を出して飛ばす | Testing Posture |
| Q6. テストの CDK アプリの依存の版 | A | npm と pip のパッケージ、CDK の CLI の版を固定し、Dependabot で更新する | Testing Posture |
| Q7. TypeScript と Python のファイルの整形と lint | A | テストのデータの CDK アプリは整形も lint もしない。tag-refactor 自身が TypeScript や Python の補助のプログラムを持つ場合は、Prettier と Black での整形を CI で確かめる | Code Style |
| Q8. 公開の進め方の書き換え | B | 下書きのとおりに書き換える(`v0.1.0` は 2026-11-07 に公開済み、`v0.2.0` は2つの機能がそろったら、タグは人が push する、`release.yml` が確かめること)。あわせて、CI の確認用のバイナリも公開と同じ6種類にそろえる(windows/arm64 を足す) | Deployment |
| Q9. 守る規則 | A | 「公開のリポジトリとテスト専用のリポジトリに、秘密の値を置かない」を、してはいけないことの規則にする。必ずすることの規則はなし | `discovered-rules.md` |

## 下書きのときに面談で聞くことにした点(記録)

下書きのときに、証拠からは決められないとして挙げた点です。どれも上の面談で答えが出ました(対応は各項目の末尾)。

1. **CI の実際の GitHub のテスト(C-Q2)を、チームの進め方にするか。** この意図だけの決定として扱うか、続く作業でも守る進め方として Testing Posture に残すか。あわせて、テスト専用のリポジトリとトークンを誰がいつ用意し、トークンの期限をどう管理するか(constraint-register の前提「人が用意する」)。→ Q4: A でチームの進め方にした。用意する人と期限の管理は未決。
2. **CI に Node.js、Python、CDK の CLI を入れること(C-Q3)を、チームの進め方にするか。** それらの版をどう固定するか(今の `GOLANGCI_LINT_VERSION`、`CFN_LINT_VERSION` のように環境変数で固定し、毎月見直すか)。テストのデータの CDK アプリの依存(npm と pip)を固定するか、Dependabot で更新するか。→ Q4: A でチームの進め方にした。依存と CDK の CLI は Q6: A(固定し Dependabot で更新)。Node.js と Python そのものの版は未決。
3. **第2弾の最小の一本の中身。** 下書きの案(TypeScript の最小の CDK アプリで1つのキー名の変更を反映し、テスト専用のリポジトリにプルリクエストを作るまで)でよいか。Python を最小の一本に含めるか、プルリクエストの作成を最小の一本から外すか。→ Q2: A(下書きの案のとおり)。
4. **第2弾の Construction 検証コマンドの性質。** AWS を呼ばない案(手元の Node.js、CDK の CLI、`gh`、テスト専用のリポジトリを使う)でよいか。第1弾のように検証用 AWS アカウントで `plan` を作るところから通すか。→ Q3: B(検証用 AWS アカウントで `plan` から通す)。
5. **`cdk synth` のテストで道具が無いときの扱い。** CI では飛ばさずに失敗にする案でよいか。手元では飛ばして、飛ばしたことを示すか。公開のワークフロー(`release.yml` の `go test ./...` は Linux だけで、今は Node.js などを入れない)ではどうするか。→ Q5: A(CI では失敗、手元では理由を出して飛ばす)。公開のワークフローでの扱いは未決。
6. **TypeScript と Python のファイルの整形と lint。** テストのデータの CDK アプリは整形も lint もしない案でよいか。tag-refactor が TypeScript や Python の補助のプログラムを持つことになった場合に、Prettier、ESLint、Black、Ruff などを使うか、そのプログラムのライセンスの表示をどうするか。→ Q7: A(Prettier と Black での整形を CI で確かめる)。lint とライセンスの表示は未決。
7. **Way of Working に書き足した4つの点**(GitHub の設定による強制、レビューの承認を必須にしない、AI-DLC の記録を同じプルリクエストで入れる、最新の `main` から作る)を、チームの進め方として承認するか。→ Q1: A(4点すべて)。
8. **Deployment の書き換え。** `v0.1.0` の箇条を、公開済みの事実と `v0.2.0` の条件に置き換えてよいか。CI の確認用のバイナリ(5つ)と公開(6つ、windows/arm64 が公開だけ)の違いは意図したものか。→ Q8: B(下書きのとおりにし、確認用のバイナリも6種類にそろえる)。
9. **ハードな制約(Mandated / Forbidden)にするものがあるか。** 下の候補を参照してください。→ Q9: A(秘密の値を置かないことを、してはいけないことの規則にする)。

## Mandated / Forbidden について

- 下書きでは、人がハードな制約として述べたものが見つからなかったため、両方とも `None.` にしていました。
- 面談の Q9 で人が A を選び、「公開のリポジトリとテスト専用のリポジトリに、秘密の値を置かない」を、してはいけないことの規則にしました。`discovered-rules.md` の `## Forbidden` に `NEVER` の形で書きました。CI の secret か手元の環境にだけ置くという補足は、Q4 で決めたトークンの置き場所(CI の secret)から書いたものです。
- `## Mandated` は `None.` のままです。人が「必ずする」と述べたものはありません。
- 下書きで挙げたほかの候補は、規則にしていません。
  - CI から実際の AWS アカウントを呼ばない(C-Q1)。すでに Testing Posture にあり、Q4 でも確かめました。
  - 認証情報を表示、記録、ログに出さない(C-T6)。製品の要件(前の NFR8)で、進め方の規則ではないと考えます。

## 調べたもの(PRACTICES_DISCOVERED に載せる一覧)

- コードの記録: `code-structure.md`、`technology-stack.md`、`dependencies.md`、`code-quality-assessment.md`、`architecture.md`、`business-overview.md`、`reverse-engineering-timestamp.md`(`aidlc/spaces/default/codekb/tag-refactor/`)
- git の履歴(`bolt/u8-iac` と `origin/main`、ブランチとタグ)
- `origin/main` の `.github/workflows/ci.yml`、`release.yml`、`scheduled.yml`、`.github/dependabot.yml`、`.gitattributes`、`go.mod`、`README.md`
- GitHub の設定(読み取りだけ): プルリクエストの一覧、リリースの一覧、マージの設定、`main` のルールセット
- この意図の ideation の成果物: `constraint-register.md`、`feasibility-questions.md`、`feasibility-assessment.md`、`scope-document.md`、`aidlc-state.md`
- 今の `aidlc/spaces/default/memory/team.md` と `project.md`
- 面談: `practices-discovery-questions.md`(Q1〜Q9 と要約の確認)

## 未確認の点と、まだ決まっていないこと

証拠の点:

- カバレッジの今の値は、リバースエンジニアリングでもこの調査でも測っていません(CI の `coverage` が80%の下限を確かめます)。
- `release.yml` で実際に `v0.1.0` を公開した実行の記録(Actions のログ)は見ていません。`gh release list` で公開済みであることだけを確かめました。
- GitHub のルールセットの設定は、今の状態を読んだだけです。いつ、なぜ変えたか(更新日時 2026-11-07)は確かめていません。

面談のあとも決まっていないこと(設計や CI の準備で決める必要があります):

- テスト専用のリポジトリと fine-grained トークンは、まだありません。誰がいつ用意するか、トークンの期限が切れる前にどう替えるかは、面談でも決めていません(constraint-register の前提は「人が用意する」)。
- CI に入れる Node.js と Python そのものの版をどう固定し、どう更新するかは決めていません(Q6 で決めたのは、テストの CDK アプリの依存と CDK の CLI の版です)。
- 公開のワークフロー(`release.yml`)のテストは、今は Linux だけで、Node.js などを入れていません。Q5 の「CI では道具が無いと失敗」をそのまま当てはめると、`release.yml` にも Node.js、Python、CDK の CLI を入れるか、`release.yml` で動かすテストの範囲を決める必要があります。
- Dependabot のプルリクエストと fork からのプルリクエストでは、実際の GitHub のテストが飛びます。必須のチェック `ci-gate` が、この飛ばしを失敗として扱わないようにする必要があります。Q5 の「飛ばして通さない」は道具が無いときの決まりで、secret が無いときの飛ばしとは別です。
- Q3 の検証コマンドは、`plan` で実際の AWS の資源から変更案を作るため、その資源が TypeScript の最小の CDK アプリと対応している必要があります(たとえば、そのアプリを検証用 AWS アカウントに配置しておく)。資源をどう用意し、検証のあとにどう片付けるかは、まだ決めていません。正確なコマンドは、Construction の検証コマンドの確認で人が承認します。
- 検証コマンドを動かす手元の環境には AWS の認証情報があります。`cdk synth` に AWS の認証情報を渡さないこと(C-T4)は、`iac` の側で守る必要があり、その方法は設計で決めます。
- `iac --pr` というフラグの名前は、面談の回答のとおりに書きました。実際の名前は設計で決まります。
- tag-refactor が TypeScript や Python の補助のプログラムを持つかは、設計で決まります。持つ場合の lint(ESLint、Ruff など)と、そのプログラムのライセンスの表示は決めていません(Q7 で決めたのは Prettier と Black での整形の確認です)。
- CI の確認用のバイナリを6種類にそろえるには、`ci.yml` の `artifacts` に windows/arm64 を足す変更が要ります。今の `ci.yml` は5種類です。
