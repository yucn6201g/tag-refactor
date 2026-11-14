# コードの品質の評価

## テスト

| 項目 | 状態 |
|---|---|
| テストの量 | 115 ファイル、約 28,800 行、テスト関数 763(本体のコードとほぼ同じ量)。`internal/iac` 142、`internal/cli` 181 |
| 種類 | 受け入れ条件(`internal/cli/acceptance_*_test.go`、U1〜U8)、単体、出力の期待のファイル(`internal/display/testdata/`、4形式 × 2言語、iac の結果と案内)、性能と規模(`large_test.go`、`performance_*_test.go`、1万リソースの apply・rollback・iac)、ファジング 3つ、ベンチマーク 2つ、race の有無で規模を変えるテスト、実際のアカウントの結合テスト(`//go:build integration`、手元だけ) |
| AWS | テストは AWS を呼ばない。`internal/testutil` の HTTP の層の偽の AWS で SDK の全体(署名、再試行、ページ、エラーの変換)を通す。書き込みは `AllowWrites` がないとテストを失敗させる。手元の `~/.aws` と本物の `~/.tag-refactor` は読まない |
| iac のテスト | 読み取り(位置、引用符、CRLF と BOM、日本語、ブロックスカラー)、変更の適用、範囲の書き換え、書き換えたあとの確かめ、差分、git(実際の git と差し替えの両方。SHA-256 のリポジトリ、`core.autocrlf`、特殊な名前、千ファイルの分割、リポジトリの設定のプログラムを動かさないこと、環境変数を無視すること、時間の上限)、上限(数、大きさ、深さ、入れ子、別名の爆弾)、リンクを辿らないこと、2回の読み取りの間の変化、書き込みの途中の失敗と中断、権限の保持、3つの OS で同じ差分、AWS を取り込まないこと、秘密の値を結果に入れないこと |
| カバレッジ | CI の `coverage` のジョブが `-coverpkg=./cmd/tag-refactor/...,./internal/...` の文の単位で 80% 以上を求める(入口の配線も計測に含める)。この調査では値を測っていない |
| テストの方針 | チームの方針は `custom`: 受け入れ条件(Given/When/Then)を先にテストにし、細かい単体テストは実装のあとに書く。コミットの履歴にも「iac の受け入れ条件のテストを実装の前に足す」(`2296959`)がある |

評価: **良い**。重要な道筋(AWS の読み書き、記録と取り消し、iac の書き換えの安全)に、受け入れ条件と単体と規模のテストがそろっています。手で作った `terraform show -json` の例(`internal/iac/testdata/terraform-show-example.json`)は実物の出力で確かめていない点が弱いところです。

## lint と整形と静的な検査

- `gofmt -l .` が空であることを CI で確かめる。
- `golangci-lint` v2.15.0 の既定の linter だけ(設定ファイルを置かない)。`nolint` のコメントは0件。
- `go vet -mod=readonly -tags integration ./...` で結合テストの型も確かめる。
- `govulncheck`(PR ごとと毎週)、`go mod verify`、`go mod tidy` の差分なし。
- `cfn-lint` で検証用のテンプレートと、iac が書き換えたテンプレートの写しを確かめる。
- `.gitattributes` で全テキストを LF に固定し、ファジングの種はバイトのまま。

## CI/CD

| ジョブ | 内容 |
|---|---|
| `test` | Linux、macOS、Windows × Go `stable`、`oldstable`。`CGO_ENABLED=0` |
| `race` | Linux で `-race`(このジョブだけ cgo を使う) |
| `coverage` | 80% の下限 |
| `lint` | gofmt、golangci-lint、`go vet -tags integration`、cfn-lint、iac で `test/sandbox/` の写しを書き換えて `u6-fixtures.yaml` の4行だけが変わることと cfn-lint を通ることを確かめる |
| `vuln` | `go tool govulncheck ./...` |
| `modcheck` | `go mod verify`、`go mod tidy` の差分なし |
| `build` | 3つの OS で静的なバイナリを作り、空のフォルダーで `--help` を動かす |
| `ci-gate` | すべてのジョブの成功を求める。ブランチの保護はこれだけを必須にする |
| `artifacts` | `main` への push のときだけ、5つのターゲットのバイナリを作り7日だけ残す(公開しない) |

- 権限は `contents: read`、`persist-credentials: false`、アクションはハッシュで固定、AWS の認証情報を持たない。
- 毎週の `scheduled.yml` が `main` に `govulncheck` を当てる。
- 公開のワークフロー(タグの push での GitHub Releases、チェックサム)はリポジトリにありません。チームの方針では公開は `vX.Y.Z` のタグの push のときで、`v0.1.0` のタグはありますが、公開がどのように行われたかはこの調査では確かめられません。

評価: **良い**。3つの OS、race、カバレッジ、脆弱性、モジュールの整合を PR ごとに確かめています。

## 文書

- `README.md`: 冒頭の1段落だけ英語、本文は日本語。コマンドごとの説明、オプションの表、終了コード、出力の形式、影響分析の範囲、apply と rollback の順序と限界、iac の対象と限界、必要な権限、開発の手順、検証の手順。今回の意図(CDK とプルリクエスト)を「まだできないこと」として明記。
- コードのコメント: どのパッケージにも説明があり、関数と定数のコメントが要件の番号(BR、NFR、契約 C1〜C12、U1〜U8)と設計の文書の節を引きます。英語で書かれています。
- `docs/iam/`: 権限の例。コードの使う操作との一致をテストが確かめる。
- `internal/iac/testdata/README.md`: テストのデータの出どころと、確かめていない点の明記。

評価: **良い**。設計の文書(AI-DLC の記録)との結び付きが強い一方、コードのコメントが要件の番号に頼るため、記録を読まずに意味を取るには手間がかかります。

## 簡単な品質の評価(3段階)

| 観点 | 評価 | 根拠 |
|---|---|---|
| 名前の分かりやすさ | 良い | Go の慣習に従い、関数名が働きを表す(`resolveRoot`、`checkBeforeWriting`、`markUnreadable`) |
| 関数の大きさ | 良い〜普通 | 多くは 30 行前後。`iac/run.go` の `editFile`(約 60 行)、`git.go` の `checkGit` など一部は長め |
| エラーの扱い | 良い | すべて `errkind` の種類つき。SDK の文言を出さない。iac は入力の誤りを `error`、結果のあとの書き込みの失敗を `Result.WriteErr` に分ける。panic はワーカーで `Internal` に変える |
| テストの有無 | 良い | 重要な道筋のすべてに受け入れ条件と単体のテスト |
| 重複 | 普通 | `stoppingPolicyReader` の13のメソッドは同じ形の包み。YAML と JSON の読み取りに似た構造がある(形式の違いのため) |
| 使われないコード | 良い | TODO、FIXME、コメントアウトしたブロックは見当たらない |

## 技術的な負債と注意

1. **手で作った Terraform の例**: `terraform-show-example.json` は実物の出力ではなく、種類ごとの `values.arn` を確かめていない(フォルダーの README に明記)。
2. **種類の表の範囲**: `internal/restype` は 18 種類。表にない CloudFormation の種類は「場所が見つからない」になる。IAM の種類は表にあるが、`scan` が集めないため変更案に入らない。
3. **表示と機能の結び付き**: `internal/display` が各機能の結果の型を直接読むため、機能を足すと表示も変える必要がある。
4. **大きなファイル**: 500 行を超えるファイルが7つ(`planner.go` 707、`saved.go` 672、`results.go` 588、`read_yaml.go` 539、`messages.go` 518、`depth.go` 515、`schema.go` 515)。どれも1つの役割の中で大きいもの。
5. **検証のスクリプト**: `scripts/verify.sh` が 1,093 行の bash に U1〜U8 の確認をまとめている。CI では動かず、検証用アカウントの状態に依存する。
6. **公開の仕組みが見えない**: 公開のワークフローがリポジトリになく、README の「配るバイナリへのライセンスの文の同梱は配布の作業で行う」が済んだかを確かめられない。
7. **ほかのセッションによる並行の変更**: この調査の時点で、README の plan の節、`internal/apply`、`scripts/`、`internal/iac/testdata` がほかの作業で変わりつつありました。

## 今回の意図に向けた品質の注意

- CDK とプルリクエストを足しても、今の iac の安全の柱(範囲だけの書き換え、書き換えたあとの再解析での確かめ、2回の読み取りのハッシュ、git でのコミット済みの確認、`fileout.Replace`)と、CI の「外部のサービスを呼ばない」「3つの OS」「80% の下限」を保つ必要があります。
- `git` の書き込みと `gh` の呼び出しは、今の `git.go` の「リポジトリの設定のプログラムを動かさない」設計と性質が違います。テストでは `lookGit`/`runGit` のような差し替えの口で GitHub を呼ばずに確かめる形が前例になります。
