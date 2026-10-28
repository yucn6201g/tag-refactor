# 使う技術: U5 影響分析

U1〜U4 の技術の選択(Go、AWS の Go 用 SDK v2、cobra、goccy/go-yaml、`golang.org/x/sync`、`golang.org/x/term`、`golang.org/x/sys`、`CGO_ENABLED=0`、GitHub Actions)は、そのまま使います。U5 では、AWS の Go 用 SDK v2 のサービスのモジュールを5つ足します。

出典: 機能設計 `construction/u5-impact/functional-design/`、`security-requirements.md`、`performance-requirements.md`、`nfr-requirements-questions.md` Q1〜Q3。

| 分野 | 選んだもの | 理由 | 採らなかった案 |
|---|---|---|---|
| リソースポリシーの読み取り | AWS の Go 用 SDK v2 のサービスのモジュール `service/s3`、`service/kms`、`service/sqs`、`service/sns`、`service/secretsmanager` を足す(IAM は既に使っている `service/iam`)。呼び出しは、U1・U2 と同じく AwsAccess の時間の上限、再試行、`--debug` の行を通す | 既に使っている SDK と同じ作りで、認証情報、リージョン、再試行の扱いを揃えられる。モジュールはサービスごとに分かれているため、足すのは使う5つだけ | AWS の HTTP の API を自分で呼ぶ(署名や再試行を作り直すことになる)。AWS CLI を呼ぶ(実行ファイルが1つで済まなくなる。NFR9) |
| 同時に読む | `golang.org/x/sync` の `errgroup` で、IAM の読み取りとリソースポリシーのリージョンの読み取りに、それぞれ `SetLimit` で上限を付ける(4 と 8)。1つのポリシーの失敗はエラーとして返さず結果に集め、認証情報の誤りと取り消しだけを全体に伝える | U2 の収集と同じ作りで、失敗の伝え方を揃えられる(NFR1.18、NFR5.10) | 既定の `errgroup.WithContext` の使い方(最初の失敗ですべてを取り消す) |
| 結果の順 | 読み終えてから、指摘と読めなかったものを機能設計 BR5.2 の鍵で並べ直す | 同時に読む順に結果が左右されない(NFR5.11) | 読んだ順に並べる(実行のたびに変わる) |
| 文書の読み取り | Go の標準の `encoding/json` と `net/url`(IAM の文書の URL エンコードを戻す)。読む大きさは 1 MiB までに絞る | U1 と同じで、依存が増えない(NFR8.20) | JSON のストリームの読み取りを自分で書く(誤りの元が増える) |
| ワイルドカードの照合 | U1 と同じく、`*` と `?` だけを特別な文字にした Go の正規表現(入力の長さに比例する時間で動く)に変え、パターンごとに使い回す | `a*a*a*b` のようなパターンでも遅くならない(U1 の NFR1.5) | 自分で書いたワイルドカードの照合(後戻りの多いパターンで遅くなりうる) |
| 壊れにくさの確かめ | Go の標準のファジング(`testing.F`)。種の入力はテストのコードと `testdata/fuzz/` に置く(U3・U4 と同じ) | 追加の道具が要らない(NFR8.21) | 外部のファジングの道具 |

## 依存の決まり

- `go.mod` に、AWS の Go 用 SDK v2 の `service/s3`、`service/kms`、`service/sqs`、`service/sns`、`service/secretsmanager` を足す。版は、既に使っている SDK のモジュールと同じ時期のものにし、Dependabot の対象に入る(チームの決まり)。CI の `govulncheck` で既知の脆弱性がないことを確かめる。
- AWS を呼ぶのは `internal/awsaccess` だけ(U1 の部品の依存の向きのまま)。`internal/impact` は、AwsAccess の読み取りのインターフェースを通してポリシーを読み、SDK の型に依存しない。テストでは、このインターフェースを差し替えるか、HTTP の層の偽の AWS を使う。
- `planner` は `impact` にも `awsaccess` にも依存しない(契約 C9 のまま)。変更案のリージョンは、`cli` が `impact.New` に渡す(機能設計の「契約への変更」)。
