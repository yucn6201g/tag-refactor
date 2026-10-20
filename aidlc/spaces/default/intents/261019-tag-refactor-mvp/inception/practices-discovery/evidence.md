# 根拠と未確認の点: tag-refactor

リードの aidlc-pipeline-deploy-agent が、調べたこと、推測したこと、面談で決まったこと、まだ決まっていないことをまとめたものです。このワークフローは協力エージェント(collaborators)を使わない設定(`collaborators: off`)のため、品質・開発・セキュリティの各エージェントによる独立した確認は行っていません。統合する協力エージェントの成果物もありません。リードの下書きと、人の面談の回答(Q1〜Q18、まとめの確認済み)だけで仕上げています。

## 1. 前提

| 項目 | 内容 | 出典 |
|---|---|---|
| プロジェクトの種類 | Greenfield(新規) | `aidlc-state.md` の Project Type |
| 範囲(scope) | `mvp`(`skeleton: on`。Operation フェーズは実行しない) | `aidlc-state.md`、`.claude/scopes/aidlc-mvp.md` |
| テスト戦略 | `Standard` | `aidlc-state.md` の Test Strategy |
| 既存のチームの進め方 | なし。`memory/team.md` と `memory/project.md` は全項目が空のひな形で、今回が初めての実行 | `aidlc/spaces/default/memory/team.md`、`project.md` |
| 提案の元にした既定値 | `memory/org.md` の Way of Working、Walking Skeleton、Testing Posture、Deployment、Code Style | `aidlc/spaces/default/memory/org.md` |
| 面談の記録 | 18問すべてに回答があり、まとめは「Looks correct」で確認済み | `inception/practices-discovery/practices-discovery-questions.md` |

## 2. 調べたもの(確認できた事実)

| 調べたもの | 結果 |
|---|---|
| リポジトリのルート | `.claude/`、`.gitignore`、`.mcp.json`、`aidlc/` だけ。Go のファイル(`go.mod` など)、`README`、`LICENSE`、`Makefile` はない |
| CI の設定 | `.github/` はない。CI は未設定 |
| lint・整形の設定 | `.golangci.yml`、`.editorconfig`、`.gitattributes` はない |
| リリースの設定 | `.goreleaser.yml` などはない |
| Git の履歴 | コミットは1件だけ(`2054bad Set up AI-DLC 2.11.0 for Claude Code`) |
| Git のブランチ | `main` のほかに、手元に `chore/setup-aidlc` がある(`main` と同じコミットを指す) |
| リモート | `origin` = `https://github.com/yucn6201g/tag-refactor.git` |
| `.gitignore` | AI-DLC が入れた行だけ。Go 向けの行(ビルド成果物など)はない |
| 開発に使う端末 | Windows(作業ディレクトリは `C:\work\tag-refactor`)。Git の `core.autocrlf` は `true` |
| 道具の有無 | `go`、`golangci-lint`、`goreleaser`、`gh` は見つからなかった。Go のツールチェーンは開発者の端末にまだ入っていない |

## 3. 推測したこと(確認はしていない)

- `chore/setup-aidlc` ブランチがあることから、開発者にはトピック用のブランチを切る習慣があるかもしれません。根拠は1件だけで、弱い推測です。面談の結果(Q18 でプルリクエスト経由に決定)とは矛盾しません。
- リポジトリが GitHub にあるため、CI は GitHub Actions が最も自然な選択肢です。ただし、CI の道具は面談で聞いていないため、`team-practices.md` には書いていません。
- 利用者は Linux、macOS、Windows で使うため(依頼の説明より)、GitHub Releases で配る「OS ごとのバイナリ」(Q11)は、少なくともこの3つの OS 向けになる見込みです。CPU アーキテクチャ(amd64、arm64 など)は決めていません。

## 4. 前のステージから引き継いだ関連事項

| 事項 | 進め方への影響 | 出典 |
|---|---|---|
| 開発は依頼者1人 | プルリクエストのレビューをする人も自分だけ。CI の自動チェックが品質の主な守りになる | `ideation/feasibility/constraint-register.md` C-O1 |
| 実装言語は Go、経験は少ない | テストの順番(Q6)と lint の厳しさ(Q15)を、学習の負担とのつり合いで選んでもらった | `constraint-register.md` C-T1 |
| OSS として一般公開する | ライセンスを決める必要がある(未決定。§8 参照) | `constraint-register.md` C-R1 |
| 検証は個人の検証用アカウントだけ | 最小の一本の検証(Q5)と結合テスト(Q8)の扱いに関係した | `constraint-register.md` C-O4 |
| 3段階で出す(第1弾 scan と plan、第2弾 apply と rollback、第3弾 IaC への書き戻し) | 最初の公開の時期(Q12)に関係した | `ideation/scope-definition/scope-document.md` |

## 5. 面談で決まったことと、その反映先

| 質問 | 回答 | 反映したこと |
|---|---|---|
| Q1 取り込み方 | A(手元で `main` に squash マージしてから push)。のちに Q18 で C に変更 | 最終的な扱いは Q18 を参照 |
| Q2 コミットメッセージ | A(Conventional Commits) | Way of Working に「Conventional Commits に従う」と記載 |
| Q3 最小の一本を先に作るか | A(作る) | Walking Skeleton に「最初に最小の一本を作る」と記載 |
| Q4 最小の一本の中身 | B(`scan` の最小版と `plan` の最小版、ビルド、テスト、lint、CI) | Walking Skeleton に中身として記載 |
| Q5 最小の一本の確かめ方 | A(個人の検証用 AWS アカウントを実際に呼ぶ) | Walking Skeleton に「検証コマンドは検証用アカウントを実際に呼ぶ」と記載。偽の応答による確認(B、C)は採らない |
| Q6 テストの順番 | C(受け入れ条件を先にテストにし、単体テストは実装のあと) | Testing Posture の `**Methodology**: custom` と `**Ordering**` |
| Q7 カバレッジの下限 | A(80%、Go の標準の文単位) | Testing Posture に記載。配線部分を計測の対象から外す案(B)は採らない |
| Q8 AWS を呼ぶ部分のテスト | A(単体テストは差し替えて CI で、実アカウントの結合テストは手元だけ) | Testing Posture に記載 |
| Q9 テストを実行する OS | A(Linux、macOS、Windows) | Testing Posture に記載 |
| Q10 リリースの流れ | A(`main` への取り込みごとに確認用のバイナリをビルドし、人が `vX.Y.Z` のタグを付けたときだけ公開) | Deployment に記載 |
| Q11 配り方 | A、B(GitHub Releases と `go install`) | Deployment に記載。Homebrew と Scoop は採らない |
| Q12 最初の公開の時期 | A(第1弾の終了時に `v0.1.0`) | Deployment に記載 |
| Q13 配布物の安全性 | A(チェックサムのみ) | Deployment に記載。署名と SLSA provenance は採らない |
| Q14 整形の道具 | A(`gofmt` のみ) | Code Style に記載 |
| Q15 lint の厳しさ | A(`golangci-lint` の既定の検査のみ) | Code Style に記載。`gosec`、`errorlint`、`revive` の追加(B)は採らない |
| Q16 改行コード | A(`.gitattributes` で Go のファイルを LF に固定) | Code Style に記載 |
| Q17 依存ライブラリの確認 | A、B(`govulncheck` を CI で実行、Dependabot を使う) | Code Style に記載 |
| Q18 マージ前の確認(Q1 の補足) | C(Q1 を見直し、プルリクエストでマージする) | Way of Working を「GitHub に push し、プルリクエストで `main` に squash マージ、マージ前に CI を実行」に変更。Testing Posture の「マージ前に CI でテスト」もこれで満たす |

## 6. org.md の既定値をどう取り入れたか

| 項目 | org.md の既定値 | 扱い |
|---|---|---|
| Way of Working | トランクベース開発、短命なブランチ、`main` への squash マージ、worktree の起点と取り込み先は `main` | そのまま取り入れた。取り込みの手段だけを Q18 によりプルリクエスト経由にした。org.md は squash の手段を決めていないため、矛盾しない。`branching-strategies.md` の分類では、ブランチの寿命が短いことからトランクベース開発に最も近く、worktree の指定は `--base main --target main --strategy squash` のままになる |
| Way of Working | squash したコミットに Bolt の slug で名前を付ける | `team-practices.md` では書き直していない(org.md の規則として引き続き有効)。Conventional Commits(Q2)と、どう組み合わせるか(例: `feat: <slug>`)は聞いていない(§8 参照) |
| Walking Skeleton | `skeleton: on` のとき、最初の Unit を最小の一本として先に作り、人が承認した検証コマンドで確かめ、チェックポイントで承認を得る | そのまま取り入れ、中身(Q4)と確かめ方(Q5)を加えた。検証コマンドの具体的な中身は、org.md のとおり Construction で人が承認して決める |
| Testing Posture | 確定した進め方がないときは `test-after`。`mvp` は行カバレッジ80%以上と、マージ前の CI 実行 | 進め方は Q6 により `custom` に変えた(org.md は team.md での確定を求めており、既定値の置き換えにあたる)。80%の「行カバレッジ」は、Q7 により Go の標準の文(statement)カバレッジとして測る。Go の標準の道具は行単位の割合を出さないため、これを80%の下限の測り方とする。マージ前の CI 実行は、Q18 のプルリクエスト経由のマージで満たす |
| Deployment | マージのたびに staging 環境へ配置し、本番への配置には別の人の承認(テックリードとプロダクトオーナー)が必要 | このツールは利用者の端末で動く CLI で、配置する環境がない。Q10 により、「マージのたびに staging へ配置」を「`main` への取り込みのたびに確認用のバイナリをビルド(公開しない)」に、「本番の配置の人による承認」を「人が `vX.Y.Z` のタグを付けて push したときだけ公開」に読み替えた。開発者は1人なので、承認する役割はその1人が兼ねる。本番への継続的デプロイ(取り込みのたびの自動公開)は採らない(Q10 の B を採らない) |
| Code Style | ルートの設定ファイルに従う。Go の整形は `gofmt`。lint は CI でマージ前に実行し、失敗したらマージしない。名前の付け方は言語の慣習に従う | そのまま取り入れ、Q14〜Q17 の選択(`gofmt` のみ、`golangci-lint` の既定のみ、LF 固定、`govulncheck` と Dependabot)を加えた |

## 7. ルールの候補として検討したもの

`discovered-rules.md` には、人がはっきり述べた「必ず守る」「絶対にしない」という制約だけを書きます。面談の18問は、いずれも選択肢から進め方を選ぶ質問で、回答の中に人自身が述べた制約の文はありませんでした。そのため、`## Mandated` と `## Forbidden` はどちらも `None.` のままにしています。

| 候補 | 検討の結果 | 出典 |
|---|---|---|
| 公開リポジトリに秘密情報を含めない | 人の回答は「特になし(OSS として一般公開する)」で、「秘密情報を含めない」はエージェントが導いた影響の説明にすぎない。認証情報を直接コードに書かないことは、`memory/phases/construction.md` の既存のルールですでに定められている | `constraint-register.md` C-R1 |
| 書き込み権限なしで試せること | OSS の利用者が期待すること(製品の要件)で、開発の進め方のルールではない。要件定義に引き継ぐ | `intent-capture-questions.md` Q14 |
| `main` に直接マージしない、自動で公開しない、など | 面談で選んだ進め方(Q1/Q18、Q10)であり、`team-practices.md` に書いた。人が「絶対に」と述べた制約ではないため、ルールにはしていない | Q10、Q18 |

## 8. まだ決まっていない点

面談で聞いていない点は、`team-practices.md` に書かずに、ここに残しています。

### 5つの項目の範囲外

- **ライセンス**: OSS のライセンスはまだ決まっていません。`v0.1.0` を公開する前に決める必要があります。要件定義などで扱うことを提案します。
- **サポートする最小の Go のバージョン**: 決まっていません。`go.mod` を作る前(Construction の最初)に決める必要があります。

### 道具の準備

- **Go のツールチェーン**: 開発者の端末にまだ入っていません。最小の一本(ビルド、テスト、lint)を手元で動かす前に入れる必要があります。`golangci-lint` と `govulncheck` も同様です。
- **GitHub CLI(`gh`)**: 入っていません。Q18 で決めたプルリクエスト経由のマージでは、ターミナルからプルリクエストを作ることができず、GitHub の Web 画面で作る必要があります。ターミナルから進めたい場合は `gh` を入れて認証する必要があります。

### 進め方の細部

- **AI-DLC の Bolt の取り込みとプルリクエストの関係**: AI-DLC の Construction は、Bolt の worktree を `aidlc engine worktree merge --target main --strategy squash` で手元の `main` に取り込みます。Q18 で決めた「GitHub のプルリクエストで squash マージする」流れと、この手元での取り込みをどうつなぐか(Bolt のブランチを push してプルリクエストを作り、GitHub 上でマージしたあとに手元の `main` を合わせるのか、など)は決めていません。Construction に入る前に確認が必要です。
- **`main` のブランチ保護**: 「マージの前に CI を通す」を GitHub の設定で強制するか(プルリクエストを必須にする、CI の成功を必須にする)は聞いていません。
- **squash したコミットの名前**: org.md の「Bolt の slug で名前を付ける」と、Q2 の Conventional Commits をどう組み合わせるかは聞いていません。
- **AWS を呼ぶテストの分け方**: 単体テストと、実アカウントを使う結合テストを、どの仕組み(ビルドタグなど)で分けるかは聞いていません。
- **リリース作業の道具**: GoReleaser を使うか、自前の GitHub Actions のワークフローにするかは聞いていません。
- **CPU アーキテクチャ**: 配るバイナリのアーキテクチャ(amd64、arm64 など)は聞いていません。
- **`govulncheck` の扱い**: 脆弱性が見つかったときにマージを止めるかどうかは聞いていません。Dependabot の対象(Go のモジュール、GitHub Actions など)も聞いていません。
- **問題のあるリリースの取り消し方**: `memory/phases/operation.md` は配置の手順に取り消しの手順を求めていますが、`mvp` では Operation フェーズを実行しません。リリースの仕組みを Construction の CI で作るときに、取り消しの手順(前のバージョンを案内する、`go.mod` の `retract` を使う、など)を決める必要があります。
- **最小の一本の検証にかかる費用と認証情報**: 検証コマンドは個人の検証用 AWS アカウントを実際に呼ぶため(Q5)、手元に認証情報が必要で、少額の費用がかかります。認証情報はリポジトリに入れません(`memory/phases/construction.md` の既存のルール)。CI からは実アカウントを呼ばないため(Q8)、CI に AWS の認証情報を置く必要はありません。
- **影響分析の検出率100%のテスト**: 下書きでは「テスト用に用意した参照の検出率100%」(intent-statement.md の成功の基準)を固定の試験データによる自動テストにする案を書きましたが、面談では聞いていません。要件定義で受け入れ条件として扱うことを提案します。
