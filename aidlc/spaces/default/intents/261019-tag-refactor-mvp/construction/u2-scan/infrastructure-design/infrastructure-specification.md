# インフラの仕様: U2 収集

U1 のインフラの仕様(`construction/u1-walking-skeleton/infrastructure-design/infrastructure-specification.md`)は、ここで変えない限りそのまま使います。tag-refactor は利用者の端末で動く CLI で、U2 でもサーバーやクラウドの上で動くものはありません。U2 で変わるのは、CI のジョブと、検証用 AWS アカウントの確認用のリソースです。

出典: `infrastructure-design-questions.md` Q1〜Q3、NFR の設計(`security-design.md` §1・§2・§5、`reliability-design.md` §3・§5・§6、`performance-design.md` §4、`logical-components.md`)、AWS のドキュメント(Resource Explorer の有効化と索引の作成にかかる時間、検索の構文、CloudFormation の `AWS::ResourceExplorer2::Index`、`AWS::ResourceExplorer2::View`、`AWS::ResourceExplorer2::DefaultViewAssociation`)。

## Deployment

| Facet | Choice | Rationale |
|---|---|---|
| 実行の形 | U1 のまま(利用者の端末で動く1つの実行ファイル、`CGO_ENABLED=0`) | NFR9 |
| ネットワーク | U1 のまま。通信の先に AWS Config と Resource Explorer の API が加わるだけ | `security-design.md` §6 |
| 保存先 | なし | 機能設計 |
| 環境 | U1 のまま(CI、手元、個人の検証用 AWS アカウント) | チームの進め方 §Testing Posture |
| IaC | 確認用のリソースは CloudFormation のテンプレートで定める。U1 のテンプレートを直し(R2)、U2 のテンプレート `test/sandbox/u2-fixtures.yaml` を足す。一度もタグが付いていないリソースだけは、検証のスクリプトが作って消す(下の「タグのないキュー」) | Q1、Q2。CloudFormation で作るとタグが自動で付くため |
| リージョン | 確認用のスタックはどちらも `ap-northeast-1` | U1 と同じ |
| 大きさと費用 | 費用のかからないものだけ。Resource Explorer は費用がかからない。AWS Config の記録は有効にしない | NFR3.5 |

## Infrastructure Services

| Service | Role | Configuration | Notes |
|---|---|---|---|
| GitHub Actions | CI | `cicd-pipeline.md` のとおり。U2 でレース検出のジョブを足す | AWS の認証情報は置かない |
| CloudFormation(検証用アカウント) | 確認用のリソースのスタック | `tag-refactor-u1-fixtures`(R2 を更新)と `tag-refactor-u2-fixtures`(新規)。どちらもスタックに目印のタグ `project=tag-refactor-fixture` を付ける | 下の表 |
| Resource Explorer(検証用アカウント) | 一度もタグが付いていないリソースの検出の確認 | `ap-northeast-1` の LOCAL の索引と、絞り込みの条件のない既定のビュー | 集約の索引は使わない(偽の AWS だけで確かめる) |
| IAM Identity Center(SSO) | 手元から検証用アカウントへの接続 | U1 のまま。管理者のプロファイル `tag-refactor-sandbox`(スタックの操作、タグのないキューの作成と削除)と、R2 を引き受ける `tag-refactor-readonly`(ツールの確認) | ツールの確認は `tag-refactor-readonly` だけで行う |

### 確認用のリソースの変更

| 記号 | スタック | リソース | 中身 | 確かめること |
|---|---|---|---|---|
| R2(変更) | U1 | IAM のロール(読み取り用) | インラインポリシーを、例のポリシー `docs/iam/readonly-policy.json` の新しい内容(U1 の操作に、`security-design.md` §1 の7つの IAM の操作を足したもの)と同じにする | NFR3.3、NFR3.5: 例のポリシーだけで scan と plan が動く |
| IX | U2 | `AWS::ResourceExplorer2::Index` | 種類 `LOCAL` | Resource Explorer の経路(BR3.1) |
| VW | U2 | `AWS::ResourceExplorer2::View` | 絞り込みの条件なし。IX ができてから作る | 既定のビューの確認(`GetDefaultView`、`GetView`) |
| DV | U2 | `AWS::ResourceExplorer2::DefaultViewAssociation` | VW をこのリージョンの既定のビューにする | `ListResources` が既定のビューで動くこと |

- **既にある索引とビュー:** アカウントに `ap-northeast-1` の索引か既定のビューが既にあると、スタックの作成が失敗するか、既定のビューを置き換えてしまう。そこで U2 のテンプレートに、パラメータ `CreateIndex`(既定 `true`)と `CreateDefaultView`(既定 `true`)を置き、条件でリソースを作るかどうかを決める。デプロイの前に、管理者のプロファイルで `list-indexes` と `get-default-view` を確かめ、既にあれば `false` を渡す。既にある既定のビューに絞り込みの条件があるときは、検証の期待結果が変わる(そのリージョンが `failed` になる)ので、デプロイを中止して人が判断する。
- **IX の中身がそろうまでの時間:** 索引を作ってから中身がそろうまで、最大1時間ほどかかる(AWS のドキュメント)。U2 のスタックは、チェックポイントの検証の1時間以上前にデプロイする。

### タグのないキュー(Q2)

| 項目 | 決めたこと |
|---|---|
| 作るもの | SQS のキュー1つ。名前は `tag-refactor-u2-untagged-<乱数8文字>`。タグを付けない |
| 作る人 | `scripts/verify.sh` が、管理者のプロファイルで作る |
| 消す時 | スクリプトの終わりに必ず消す(成功しても失敗しても、中断しても。終了の処理で消す)。消せなかったときは、名前を表示して人に消すよう求める |
| 待ち方 | Resource Explorer の `tag:none` の結果にキューの ARN が現れるまで、1分ごとに最大30分待つ。現れなければ、検証を失敗にする |
| 費用 | 要求を送らないので、かからない |

### 期待結果(U2 のチェックポイント)

ルールファイルは、U1 のもの(`test/sandbox/u1-rules.yaml`)に `required: [Owner]` を足した `test/sandbox/u2-rules.yaml` を使います(中身はコードの生成で決める)。検証用アカウントには、確認用のスタックの外のリソース(既定の VPC とそのサブネット、セキュリティグループなど、一度もタグが付いていないもの)もあるので、件数は「少なくとも」で判定します。

| コマンド | 期待する結果 |
|---|---|
| `scan --region ap-northeast-1 --rules test/sandbox/u2-rules.yaml` | 一度もタグが付いていないリソースの検出が `complete`(方法は Resource Explorer)。タグのないキューが「一度もタグが付いていない」として数えられる。`Owner` の欠落が、QA、QB、QC、B1、タグのないキューを含む。U1 の表記ゆれのまとまりも出る。終了コード 2 |
| `scan --rules test/sandbox/u2-rules.yaml`(全リージョン) | `ap-northeast-1` は `complete`、他のリージョンは索引も Config もないので `unavailable` で、全体は `partial`。理由にリージョンと足りないものが出る。タグの取得で読めないリージョンはない。終了コード 2 |
| `plan`(全リージョン) | U1 の期待結果と同じ変更、ぶつかり、影響。終了コード 2 |
| Resource Explorer の複数ページ | 結合テストで、ページの大きさを5にして `tag:none` を最後まで読み、ページの大きさ500で読んだ結果と同じ ARN の集まりになる(NFR1.10) |
| Config の3つの操作 | 結合テストで、R2 から直接呼ぶ。記録の設定と状態は「記録なし」で成功する。問い合わせはツールが使う文で呼び、AccessDenied にも `InvalidExpressionException` にもならない(`security-design.md` §2)。scan 自体は記録がないため問い合わせを呼ばないので、この直接の呼び出しで確かめる |
| L95 | `--debug` の行から操作ごとの L95 を求め、`performance-design.md` §4 の式で見積もって表示する。10分以内であること |
| 進み具合の表示 | 人が手元の Windows Terminal と Git Bash で全リージョンの scan を動かし、書き換えと追記を目で確かめる(NFR9.3、NFR11.5) |

### デプロイと削除の手順(U1 の手順に足す)

1. `aws sts get-caller-identity --profile tag-refactor-sandbox` で、検証用アカウントであることを確かめる。
2. `cfn-lint test/sandbox/u1-fixtures.yaml test/sandbox/u2-fixtures.yaml`。
3. U1 のスタックを更新する(U1 の手順のデプロイのコマンドのまま。R2 だけが変わる)。
4. `aws resource-explorer-2 list-indexes --regions ap-northeast-1` と `aws resource-explorer-2 get-default-view --region ap-northeast-1` を管理者のプロファイルで確かめ、`CreateIndex` と `CreateDefaultView` の値を決める。
5. U2 のスタックをデプロイする: `aws cloudformation deploy --profile tag-refactor-sandbox --region ap-northeast-1 --stack-name tag-refactor-u2-fixtures --template-file test/sandbox/u2-fixtures.yaml --parameter-overrides CreateIndex=<true|false> CreateDefaultView=<true|false> --tags project=tag-refactor-fixture`。
6. 1時間以上おいてから、`scripts/verify.sh` を動かす。
7. 削除: Construction の終わりに、人の指示で U2、U1 の順にスタックを消す(人の指示で、U1 のスタックを残していることに合わせる)。U2 のスタックを消すと、テンプレートで作った索引とビューも消える。`false` を渡して作らなかったものは残る。

## Shared Infrastructure

| Shared Resource | Owner Unit | Consumer Units | Access Boundary |
|---|---|---|---|
| GitHub Actions のワークフロー | U1 | U2〜U8 | U2 はレース検出のジョブを足し、`ci-gate` の `needs` に入れる。この変更は人がプルリクエストで承認する |
| `docs/iam/readonly-policy.json` | U1 | U2(7つの操作を足す)、U5 | テストで、読み取りの操作だけであること、操作の表と U1 のテンプレートの R2 と一致することを確かめる |
| U1 の確認用のスタックの R2 | U1 | U2(ツールの確認に使う) | U1 の仕様では「後の作業単位は自分のスタックを作る」としていたが、R2 は例のポリシーと一致させる必要があり、2つ持つと同じものを2か所で直すことになる。そこで R2 だけは U1 のスタックのものを共有し、U2 のスタックには読み取り用ロールを置かない(Q1) |
| `scripts/verify-u1.sh` | U1 | U2 の `scripts/verify.sh` が中から呼ぶ | U1 の確認の中身は変えない |
