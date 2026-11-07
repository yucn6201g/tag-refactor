# 結合テストの手順

テスト戦略は Standard なので、作業単位の境目と、作業単位をまたぐ流れを確かめます。性能とセキュリティの確かめは各作業単位の単体テストと大きなテストに含まれ、別の手順書は作っていません(下の「性能とセキュリティ」)。

## 3つの層

| 層 | 動くところ | AWS の認証情報 | 確かめること |
|---|---|---|---|
| 受け入れ条件のテスト(`TestAcceptance*`) | 手元と CI | 要らない | コマンドの入口から、HTTP の層の偽の AWS(`internal/testutil`)まで。Go の `testing` だけで動く |
| 大きなテスト(`TestLarge*`、`TestPerformance*`) | 手元と CI | 要らない | 1万リソースの呼び出しの回数、時間、メモリ |
| 検証コマンド(`scripts/verify.sh`)と結合テスト(`TestIntegration*`) | 手元だけ | 要る | 実際の検証用アカウント。CI からは呼ばない |

## 認証情報の要らないテスト

```bash
go test -mod=readonly -run 'TestAcceptance' ./internal/cli/...
go test -mod=readonly ./...
```

## 作業単位をまたぐ流れ

| 流れ | 通る作業単位 | テスト |
|---|---|---|
| `scan` から `rules suggest`、`plan` | U2、U3、U4 | `internal/cli/acceptance_u3_test.go`、`internal/cli/acceptance_u4_test.go` |
| `plan` と影響分析 | U4、U5 | `internal/cli/acceptance_u5_test.go` |
| 保存した変更案から `apply`、記録 | U4、U6 | `internal/cli/acceptance_u6_test.go` |
| `apply` の記録から `rollback`、差分0 | U6、U7 | `internal/cli/acceptance_u7_test.go`、`internal/apply/rollback_scenario_test.go` |
| 保存した変更案から `iac` | U4、U8 | `internal/cli/acceptance_u8_test.go` |
| 書き出しの部品の共有 | U4、U6、U7、U8 | `internal/fileout/replace_test.go`、`internal/journal/journal_test.go` |
| ここまでをすべて、実際のアカウントで | U1〜U8 | `scripts/verify.sh` |

## 検証用アカウント(手元だけ)

前提: 確認用のスタック(U1、U2、U5、U6)とプロファイル(`tag-refactor-sandbox`、`tag-refactor-readonly`、`tag-refactor-apply`)があり、サインイン済みであること。

```bash
"C:\Program Files\Git\bin\bash.exe" -c "TAG_REFACTOR_SANDBOX_ACCOUNT_ID=<検証用アカウントの ID> bash scripts/verify.sh"
```

約15分かかります。確認用のリソースにタグを足して消し、一時キューを作って消します。`verify: OK` で終わること。`iac` の手順は AWS に書きません。結合テストは、この中で必要な環境変数(タグのないキューの ARN など)を渡して動きます。

## 性能とセキュリティ

別の手順書を作らず、次のテストで確かめます。

| 種類 | テスト |
|---|---|
| 性能 | `TestPerformanceU2`、`TestPerformanceU3`、`TestPerformanceU4`、`TestLargeAnalysisU5`、`TestLargeApplyU6`、`TestLargeRollbackU7`、`TestLargeIacU8` |
| 権限 | 例の IAM ポリシーのテスト(`internal/awsaccess/docs_test.go`、`internal/awsaccess/docs_apply_test.go`) |
| 秘密の値 | `internal/iac/write_test.go` の `TestSecretsStayOutOfTheResult` など、各作業単位の出力の確かめ |
| 壊れた入力 | `FuzzParse`、`FuzzLoadPlan`、`FuzzParsePolicy` |

## カバレッジ

Go の文単位で80%以上。`-coverpkg` で `cmd` と `internal` のすべてを測ります。

```bash
go test -mod=readonly -coverpkg=./cmd/tag-refactor/...,./internal/... -coverprofile=coverage.out ./cmd/tag-refactor/... ./internal/...
go tool cover -func=coverage.out
```
