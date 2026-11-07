# Construction から Operation への確認

**判定: 条件つき(PASS ではありません)**

作ると決めたものには、未解決のものはありません。ただし、作業単位をまたぐ対応の確かめは不合格で、人が範囲の外と決めた ID が未カバーのまま残っています。Build and Test で、人がこの未達を受け入れました(2026-11-07、`Accept failure`)。

実行計画の Operation のステージ(deployment-pipeline、environment-provisioning、deployment-execution、observability-setup、performance-validation など)は、すべて SKIP です。この確認のあとに動く Operation の作業はありません。

## 確認した項目

| 項目 | 結果 | 根拠 |
|---|---|---|
| すべての作業単位を作った | はい(U1〜U8) | 各 `construction/u*/code-generation/code-summary.md` |
| すべての作業単位をテストした | はい | `construction/build-and-test/test-results.md` |
| すべての `traceability.json` がある | はい(8件) | 各 `construction/u*/code-generation/traceability.json` |
| 対応表に未解決のものがない | はい。`Deferred` 16 行はすべて後の作業単位に OK がある。`N/A` は `AC7.2.1`、`AC7.3.1` の2行で、範囲の外 | `construction/build-and-test/cross-unit-traceability.md` |
| 対応表の対象のファイルが存在する | はい。U5 の `BR1.2` の誤ったファイル名を Build and Test で直した | 同上 |
| FR、NFR、AC をまたぐ確かめ | **不合格**(OK 133、一部 1、未カバー 8) | 同上 |
| CI の関門が Build and Test のコマンドを強制する | はい。手元だけのものを除く。`ci-gate` を必須にした(この段階で穴を埋めた) | `construction/ci-pipeline/quality-gates.md` |

## 未カバーの ID

| ID | 内容 | 決めた場所 | 扱い |
|---|---|---|---|
| `NFR12`、`AC6.2.1`、`AC6.2.2`、`AC6.2.3` | GitHub Releases、`go install`、チェックサム | `inception/units-generation/units-generation-questions.md` Q3、Q4 | 別の作業。優先度は Must(第1弾)で、**`v0.1.0` の前に必要** |
| `FR8.3`、`AC7.2.1` | CDK への反映 | U8 の機能設計 Q1 | 後の作業。優先度は Could |
| `FR8.4`、`AC7.3.1` | プルリクエストの作成 | U8 の機能設計 Q1 | 後の作業。優先度は Could |
| `FR8`(一部) | 上の2つを含む親の要件 | 同上 | `FR8.1`、`FR8.2` は作った |

## この確認の時点で終わっていないこと

| 項目 | 状態 |
|---|---|
| U8 のプルリクエスト #20 | CI は通ったが、未マージ。`main` は `82bb6f3` |
| squash マージ後の `main` の CI | 未確認 |
| AI-DLC の記録のコミット | 未コミット(U6、U7、U8 の記録と、ステートと監査の変更) |
| `-race`、3つの OS、cfn-lint | #20 の CI で確かめた |
| 検証用アカウントの確認用のスタック | 残っている(U1、U2、U5、U6) |
| 配布と `v0.1.0` | 別の作業 |

## 読むときの注意

- **Met の根拠:** Build and Test の判定の表の細かい要件は、対応表の対象があり、その確かめが通ったことで Met としています。1行ずつの読み直しはしていません。
- **タイミングの問題:** `rollback` の不具合(`52163d6`)の原因はタグの API の読み取りの遅れと見ていますが、推測です。1回の検証の通過で直ったとは言い切れません。
- **`plan` の限界:** `apply` の直後の変更案は、書いたばかりのリソースを見落とすことがあります。README にはまだ書いていません。
- **U8 の直し2つと U7 の振る舞いの変更:** `52163d6` と `da2dad9` は、コードのレビューを通っていません(下の追記で、あとからレビューしました)。

## 公開のあとの追記(2026-11-07)

上の判定は、この確認をした時点のものです。そのあとに次のことが終わりました。判定の表は書き換えていません。

### 終わったこと

| 項目 | 結果 | 根拠 |
|---|---|---|
| U8 のプルリクエスト #20 | `main` に squash マージ(`e8c83f9`) | GitHub |
| 配布の仕組み | プルリクエスト #21 で `main` に入った(`e2fffdc`)。別のセッションで作り、AI-DLC の段階は通していない | `.github/workflows/release.yml`、README の「インストール」 |
| マージのあとの `main` の CI | `e2fffdc` の `ci` が成功 | GitHub Actions |
| `v0.1.0` の公開 | 人がタグ `v0.1.0` を push し、`release` の実行(build、3つの smoke、publish)がすべて成功 | GitHub Releases の `v0.1.0` |

### 未カバーだった ID の確かめ

| ID | 結果 | 確かめたこと |
|---|---|---|
| `AC6.2.1` | 満たした | 公開物は linux、darwin、windows の amd64 と arm64 の6つ。`release` の smoke が `windows_amd64`、`linux_amd64`、`darwin_arm64` で動いた。手元(Windows)で `windows_amd64.zip` を展開し、`tag-refactor.exe --version` が `tag-refactor v0.1.0` を出した。ほかの3つは、手元でも smoke でも動かしていない |
| `AC6.2.2` | 満たした | 空の `GOPATH` と `GOBIN` で `go install github.com/yucn6201g/tag-refactor/cmd/tag-refactor@v0.1.0` が通り、`--version` が `v0.1.0` を出した |
| `AC6.2.3` | 満たした | 公開物に `tag-refactor_v0.1.0_checksums.txt` がある。手元に取った `windows_amd64.zip` の SHA-256 が、その行と一致した。ほかの5つは手元では比べていない |
| `NFR12` | 満たした | 上の3つ |
| `FR8.3`、`AC7.2.1`、`FR8.4`、`AC7.3.1` | 未カバーのまま | 第2弾の新しい intent で扱う |

### あとからのコードのレビュー

`52163d6` と `da2dad9` を読み直しました。

- **`52163d6`(`rollback`):** 1件の穴がありました。書き込みの結果が分からなかった記録は、理由に誤りの種類が付きます(`resultUnknown:Throttling` など)。直しは理由が `resultUnknown` と完全に同じときだけ読み取りを疑ったため、これらと、一部だけ書けた記録(`partialWrite:...`)が対象から漏れていました。理由の `:` より前を見て、`resultUnknown` と `partialWrite` も対象にしました。テストを足し、README の説明も直しました。
- **`da2dad9`(検証スクリプト):** 問題は見つかりませんでした。確かめの条件は緩めていません。

### Terraform の例

`internal/iac/testdata/terraform-show-example.json` を実物と照らしました。Terraform 1.17.1 と AWS プロバイダー 6.72.0 で、確認用のスタックの既存のリソースを手元の state に取り込み(AWS は読むだけ)、`terraform show -json` の出力を `iac` の読み方で読みました。

- 8つの管理リソース(`count` と `for_each`、子モジュールのものを含む)が `values.arn` で読め、データソースは外れました。
- 実物にあって例になかったもの(`identity`、`identity_schema_version`、`index`、子モジュールの項目の順)を例に足しました。読み方は変えていません。
- 表の18種類のうち、残りの11種類の `values.arn` は確かめていません(確認用のスタックにない種類)。詳しくは `internal/iac/testdata/README.md` にあります。
