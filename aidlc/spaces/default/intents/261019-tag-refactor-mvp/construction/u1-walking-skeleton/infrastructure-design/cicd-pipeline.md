# CI/CD の流れ: U1 最小の一本

出典: NFR の設計(`security-design.md` §6、`reliability-design.md`)、`infrastructure-design-questions.md` Q3、チームの進め方(§Way of Working、§Testing Posture、§Code Style、§Deployment)。レビュー(`reviews/review-01.md`)の R-04、R-07〜R-12 を受けて改訂しました。

U1 では CI だけを作ります。公開(GitHub Releases、チェックサム)は別の作業です(`inception/delivery-planning/external-dependency-map.md` の E6)。

## Go と道具の版

- **Go の版:** マトリクスは `actions/setup-go` の `stable`(最新版)と `oldstable`(1つ前の版)にする。`GOTOOLCHAIN=local` を全ジョブに設定し、Go が自動で別の版を取りに行かないようにする。`go.mod` の `go` 行は、1つ前の版以下に保つ。
- **道具の版:** `govulncheck` は `go.mod` の `tool` の指定で版を管理し、Dependabot の gomod の対象にする。`golangci-lint` は公式の Action(SHA で固定)に版を入力として渡す。この版は Dependabot が追えないため、月に1回、人が新しい版を確かめる(チェックポイントの外の定期の作業として `monitoring-design.md` に書く)。
- **cfn-lint:** 版を固定して入れる(AWS は呼ばない)。

## ワークフローとジョブ

### 1. `ci.yml`(プルリクエストと、`main` への取り込みで動く)

| ジョブ | 動く場所 | 中身 |
|---|---|---|
| `test` | Linux、macOS、Windows × Go の `stable` と `oldstable` | `go test -mod=readonly ./...`(受け入れ条件のテスト、単体テスト、性能のテスト、出力の守りのテスト、IAM の例のポリシーの検査、確認用のテンプレートの R2 と `docs/iam/readonly-policy.json` の一致のテスト)。`integration` のビルドタグのテストは動かさない |
| `coverage` | Linux、`stable` | `-coverpkg=./...` ですべてのパッケージを測り、80%未満なら失敗させる |
| `lint` | Linux | `gofmt` の差分がないこと、`golangci-lint`(既定の linter だけ)、`go vet -tags integration ./...`(検証用アカウントのテストの型の検査。実行はしない)、`cfn-lint test/sandbox/*.yaml` |
| `vuln` | Linux | `govulncheck` |
| `modcheck` | Linux | `go mod verify`、`go mod tidy` の後に差分がないこと |
| `build` | Linux、macOS、Windows | `CGO_ENABLED=0` でビルドし、実行ファイルを空のディレクトリにコピーして `--help` を動かす |
| `ci-gate` | Linux | `needs` に上の全ジョブを並べ、すべて成功したときだけ成功する集約のジョブ。ブランチの保護では、これだけを必須にする |
| `artifacts` | Linux(3つの OS 向けにクロスビルド) | `main` への取り込みのときだけ、`ci-gate` の後に動く。3つの OS のバイナリを GitHub Actions の成果物として残す(保存は7日間)。公開しない |

- すべての Actions はコミットの SHA で固定する。ワークフローの権限は `contents: read`。リポジトリの取り出しでは認証情報を残さない設定にする。
- 各ジョブに時間の上限(15分)を設ける。同じプルリクエストの古い実行は、新しい push で取り消す(`concurrency`)。
- AWS の認証情報などの秘密の値は、CI に置かない。CI から実際の AWS は呼ばない。
- 性能のテストは、共有のランナーの揺れで不安定にならないよう、NFR の設計の10秒に対して余裕をもって判定する(手元の計測で、上限の半分以下で終わることを確かめてから入れる)。不安定になったら、時間ではなく AWS の呼び出しの回数とメモリの割り当ての数で判定する形に変える。

### 改行コード(`.gitattributes`)

- リポジトリの全体を `* text=auto eol=lf` にする。Windows のランナーでも、Go のファイル、テストデータ(期待する出力、ルールファイルの YAML、JSON)が LF で取り出される。
- 「CRLF のルールファイルも同じに読める」テスト(NFR9.1)の入力は、ファイルとして置かず、テストの中で CRLF の文字列を組み立てる。

### 2. `scheduled.yml`(週1回の定期実行)

| ジョブ | 中身 |
|---|---|
| `vuln-weekly` | `main` に対して `govulncheck` を動かす。失敗の通知は、定期実行の設定を最後に変えた人に届く(今はリポジトリの持ち主1人) |

- 公開リポジトリでは、長い間リポジトリに動きがないと定期実行が止まることがある(未確認)。第1弾の間は問題になりにくいため、注意書きにとどめる。

### 3. `dependabot.yml`

- Go の依存(gomod)と GitHub Actions を、週1回確かめる。コミットメッセージの接頭辞を、gomod は `chore(deps)`、Actions は `ci` にする(Conventional Commits に合わせる)。Dependabot のプルリクエストも、`ci-gate` を通ったものだけをマージする。

## 関門(ブランチの保護)

`main` に、次の設定のブランチの保護(またはルールセット)を置きます。人が設定し、U1 のチェックポイントで確かめます。

| 設定 | 値 | 理由 |
|---|---|---|
| 必須のステータス | `ci-gate` だけ | マトリクスのジョブの名前は版が変わると変わるため、集約のジョブで受ける |
| ブランチを最新にしてからマージ | 有効 | `main` の最新に対して CI が通ったものだけをマージする |
| 管理者にも適用(保護を回避できる人を置かない) | 有効 | 持ち主1人の開発で、関門を確実に効かせる |
| 必須の承認者の数 | 0 | 1人の開発で、作者は自分のプルリクエストを承認できないため。人の承認は、AI-DLC のチェックポイントでの承認と、人が自分でマージすることで行う |
| マージの方法 | squash マージだけ。既定のコミットメッセージはプルリクエストのタイトル | 1つの Bolt が `main` の1つのコミットになる。タイトルを Conventional Commits の形にする |

- **リポジトリの公開の状態:** 非公開のリポジトリで無料のプランだと、ブランチの保護が使えない可能性がある(未確認)。U1 のチェックポイントで状態を確かめ、使えない場合は、OSS として公開するか、関門を使わず「`ci-gate` が通ったことを確かめてから人がマージする」運用にするかを、人が決める。
- プルリクエストのタイトルの形式の自動の検査は、U1 では入れない。

## 流れ(1つの Bolt)

1. `main` から短命なブランチを作る(例 `bolt/u1-walking-skeleton`)。
2. 受け入れ条件のテストをコミットし、次に実装、次に細かい単体テストをコミットする(NFR10.2)。
3. GitHub に push し、プルリクエストを作る(タイトルは Conventional Commits の形。例 `feat: walking skeleton for scan and plan`)。`ci.yml` が動く。
4. `ci-gate` が通り、AI-DLC のチェックポイントで人が承認したら、人が squash マージする。
5. `main` への取り込みで `ci.yml` がもう一度動き、確認用のバイナリが成果物として残る。

## 配置と戻し方

- 配置(デプロイ)はない。公開は人が `vX.Y.Z` のタグを付けたときだけで、U1 では行わない。
- 問題のある変更が `main` に入ったら、そのコミットを打ち消すプルリクエスト(`git revert`)を作り、同じ関門を通してマージする。

## 秘密の値

| 秘密の値 | 置き場所 |
|---|---|
| AWS の認証情報 | CI には置かない。手元の SSO のプロファイルだけ |
| GitHub のトークン | ワークフローに自動で渡されるもの(権限は読み取りだけ)。他は置かない |
