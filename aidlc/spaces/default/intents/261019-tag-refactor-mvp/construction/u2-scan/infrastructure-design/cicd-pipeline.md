# CI/CD の流れ: U2 収集

U1 の CI/CD の流れ(`construction/u1-walking-skeleton/infrastructure-design/cicd-pipeline.md`: ワークフロー、ジョブ、関門、Go と道具の版、秘密の値)は、ここで変えない限りそのまま使います。U2 でも公開(GitHub Releases)は行いません(`inception/delivery-planning/external-dependency-map.md` の E6)。

出典: NFR の設計(`reliability-design.md` §5・§6、`performance-design.md` §5、`security-design.md` §1・§6)、`infrastructure-design-questions.md` Q3、`infrastructure-specification.md`、チームの進め方(§Way of Working、§Testing Posture、§Code Style、§Deployment)。

## `ci.yml` の変更

| ジョブ | 変更 | 理由 |
|---|---|---|
| `race`(新規) | Linux、Go の `stable`。`CGO_ENABLED=1` にして `go test -race -mod=readonly ./...` を動かす。時間の上限は15分 | NFR5.7。レース検出には cgo と C のコンパイラが要り、Linux の実行環境にはそれがある。ほかの OS では動かさない |
| `ci-gate` | `needs` に `race` を足す | ブランチの保護の必須は `ci-gate` だけのまま |
| `lint` の `cfn-lint` | 検査する対象を `test/sandbox/*-fixtures.yaml` にする(U1 と U2 のテンプレート) | `test/sandbox/` にはルールファイル(`*-rules.yaml`)もあり、それは CloudFormation のテンプレートではないため |
| `test` | 変えない。U2 で足すテスト(大きな偽の AWS、乱数の遅れ、操作の表と例のポリシーと R2 の照合、例外の2つの操作の検査、端末の判定の差し込み口)は、すべて `go test ./...` に入る | NFR1.6、NFR1.7、NFR3.3、NFR3.4、NFR5.7、NFR11.5 |
| `coverage` | 変えない(80%以上、`cmd` と `internal` のすべて) | NFR10.3 |
| `build`、`artifacts` | 変えない。配るバイナリは `CGO_ENABLED=0` のまま | NFR9 |

- U2 で足す大きな偽の AWS のテストは、CI の各 OS で合わせて2分以内に終わる大きさにする(NFR10.3)。手元で計って半分以下で終わることを確かめてから入れる(U1 の性能のテストの方針と同じ)。超えたら、件数ではなく呼び出しの回数の比べ方は変えずに、偽の AWS の応答の作り方を軽くする。
- 新しい依存(`service/configservice`、`service/resourceexplorer2`、`golang.org/x/sync`、`golang.org/x/term`)は、既存の `vuln`、`modcheck`、Dependabot の対象に自動で入る。設定の変更は要らない。
- CI から実際の AWS は呼ばない(U1 のまま)。`integration` のビルドタグのテストは、`lint` の `go vet -tags integration` で型だけを確かめる。

### ジョブと関門の対応

| 関門 | 確かめること | ジョブ |
|---|---|---|
| 3つの OS でテストが通る | 受け入れ条件、単体、性能、出力の守り、IAM の例の検査 | `test` |
| 同時に読んでもデータの競合がない | `-race` | `race` |
| カバレッジ80%以上 | 文の単位 | `coverage` |
| 書式と lint とテンプレート | `gofmt`、`golangci-lint`、`go vet -tags integration`、`cfn-lint` | `lint` |
| 依存の脆弱性と整合 | `govulncheck`、`go mod verify`、`go mod tidy` | `vuln`、`modcheck` |
| 3つの OS でビルドできる | `CGO_ENABLED=0` のビルドと `--help` | `build` |
| すべて | 上のすべての成功 | `ci-gate`(ブランチの保護の必須) |

## 検証のスクリプト(Q3)

`scripts/verify.sh` を足します。U2 のチェックポイントで、人の承認を得てから Construction の検証コマンドにします(それまでは U1 の `scripts/verify-u1.sh` のまま)。

1. `scripts/verify-u1.sh` を呼ぶ(すべてのテストと、U1 の結合テスト。中身は変えない)。失敗したら止める。
2. 管理者のプロファイル `tag-refactor-sandbox` で、検証用アカウントであること、U2 のスタックがあること、`ap-northeast-1` の索引が使える状態であることを確かめる。なければ止めて、デプロイの手順を案内する。
3. タグのないキューを作る。終了の処理で必ず消す(`infrastructure-specification.md` の「タグのないキュー」)。
4. Resource Explorer の `tag:none` にキューが現れるまで、1分ごとに最大30分待つ。
5. U2 の結合テスト(ビルドタグ `integration`)を、`TAG_REFACTOR_IT_PROFILE=tag-refactor-readonly` と、キューの ARN を渡す環境変数で動かす。結合テストが飛ばされたら失敗にする(U1 と同じ)。中身は `infrastructure-specification.md` の期待結果の表。
6. `--debug` の行から操作ごとの L95 と、10分の見積もりを表示する。
7. Windows の Application Control でテストの実行ファイルが止められたときは、U1 のスクリプトと同じく再試行する。

- スクリプトは Git Bash でも動くよう、U1 のスクリプトと同じ書き方(bash、`aws` の CLI)にする。
- 認証情報を表示しない。スクリプトは AWS の CLI の出力のうち、ARN と状態の値だけを読む(U1 で、一時的な認証情報を誤って表示したことへの対策を続ける)。

## 流れ(この Bolt)

1. `bolt/u2-scan` のブランチで、受け入れ条件のテスト、実装、細かい単体テストの順にコミットする(チームの進め方 §Testing Posture)。
2. push して、`main` へのプルリクエストを作る(タイトルは Conventional Commits の形。例 `feat: scan all regions and find never-tagged resources`)。
3. `ci-gate` が通り、U2 のチェックポイントで人が承認したら、人が squash マージする。

## 配置と戻し方、秘密の値

- 配置はない。戻すときは U1 と同じく、打ち消しのプルリクエスト(`git revert`)を同じ関門に通す。
- 確認用のスタックの変更を戻すときは、前のテンプレートで `aws cloudformation deploy` をやり直す(R2 の操作を減らすだけ、または U2 のスタックを消すだけで、データは失われない)。
- 秘密の値は U1 のまま。CI に AWS の認証情報は置かない。検証のスクリプトは手元の SSO のプロファイルだけを使う。
