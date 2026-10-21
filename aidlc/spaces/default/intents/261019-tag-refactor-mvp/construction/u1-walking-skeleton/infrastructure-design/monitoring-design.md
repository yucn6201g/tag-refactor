# 監視の設計: U1 最小の一本

出典: NFR の設計 `observability-design.md`、`cicd-pipeline.md`。

tag-refactor は利用者の端末で動く CLI で、動き続けるサーバーがないため、実行時のメトリクスの収集、警報、ダッシュボード、SLO はありません。実行時の観測は、標準エラーへの表示と `--debug` です(`observability-design.md`)。ここでは、開発の品質を見張る仕組みを定めます。

## Metrics & KPIs

| Metric | Source | Threshold | Why it matters |
|---|---|---|---|
| テストの成否(3つの OS × 2つの版) | `ci.yml` の `test` | すべて成功 | どの OS でも同じに動くこと(NFR9.1) |
| カバレッジ | `ci.yml` の `coverage` | 文単位で80%以上 | チームの進め方の下限(NFR10.1) |
| 既知の脆弱性 | `ci.yml` の `vuln`、`scheduled.yml` | 0件 | 依存の脆弱性(NFR8.5) |
| 検証用アカウントでの時間と件数 | U1 のチェックポイントの記録 | 目安: scan と plan がそれぞれ1分以内(`nfr-requirements/performance-requirements.md` の「目安」) | U2 で NFR1 を見積もる材料(NFR1.2) |
| golangci-lint の版 | 人の月1回の確認 | 新しい版が出ていれば、プルリクエストで上げる | Dependabot が追えない道具の版を古くしないため(NFR8.7) |

## Alerts

| Alert | Condition | Severity | Routes to |
|---|---|---|---|
| 週1回の脆弱性の確認の失敗 | `scheduled.yml` の `vuln-weekly` が失敗 | 高 | GitHub の通知(定期実行の設定を最後に変えた人。今はリポジトリの持ち主) |
| `main` での CI の失敗 | `main` への取り込みの後に `ci.yml` が失敗 | 高 | GitHub の通知(リポジトリの持ち主) |
| Dependabot の更新 | 依存の更新のプルリクエストができた | 低 | GitHub の通知 |

## SLIs / SLOs

| SLI | SLO target | Measurement window |
|---|---|---|
| (なし) | CLI でサーバーがないため定めない | — |

## Logs & Tracing

- 実行時のログは標準エラーにだけ出す。ファイルや外部への送信はしない(NFR8.8)。
- 調べるときは `--debug` で、AWS の呼び出しごとの1行(操作、リージョン、時間、再試行の回数)を出す。
- CI のログは GitHub Actions が保持する(既定の保存期間)。ダッシュボードは作らない。
