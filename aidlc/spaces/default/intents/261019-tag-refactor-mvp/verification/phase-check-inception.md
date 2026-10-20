# Inception から Construction への確認

**判定: PASS**

Inception の各ステージが作った `traceability.json` を読み、要件からストーリー、部品、作業単位までのつながりに抜けがないかを確かめました。`GAP`、`ORPHAN`、存在しない対象、上流の ID の抜けは、ありません。

## 確認した表

| ステージ | ファイル | 上流の ID の数 | OK | N/A | GAP / ORPHAN |
|---|---|---|---|---|---|
| User Stories | `inception/user-stories/traceability.json` | 75(FR1〜FR8 とその細目、NFR1〜NFR12) | 75 | 0 | 0 |
| Domain Design | `inception/domain-design/traceability.json` | 13(US1.1〜US7.3) | 12 | 1 | 0 |
| Units Generation | `inception/units-generation/traceability.json` | 13(US1.1〜US7.3) | 12 | 1 | 0 |

Contract Design は `traceability.json` を作りません(要件のつながりではなく、形式の契約を受け持つため)。そのため、この確認には含めていません。

## N/A の項目

| ID | ステージ | 理由 | 扱い |
|---|---|---|---|
| US6.2 | Domain Design | 配布は部品ではなく、リリースの仕組みで扱う | 抜けではありません |
| US6.2 | Units Generation | 配布の仕組みは今回の作業単位に含めず、別の作業として後で行うと利用者が決めた(`units-generation-questions.md` Q3、Q4) | 抜けではありません。`inception/delivery-planning/external-dependency-map.md` の E6 として、B5 の後に行います |

## Construction の前に残っていること

抜けではありませんが、次の未決事項は Construction の中で決めます。

| 未決事項 | 決める時期 |
|---|---|
| 要件 OQ1(ライセンス) | `v0.1.0` の公開の前(E7) |
| 要件 OQ2(Go の最小バージョン) | B1 の設計 |
| 要件 OQ3(IaC への反映の詳しい振る舞い) | B8 の前 |
| 要件 OQ4(作業のまとまりとプルリクエストのつなぎ方) | `bolt-plan.md` の「進め方の決まり」で答えた |
| 要件 OQ5(一度もタグが付いていないリソースを探す件数の上限) | B2 の設計 |
| 契約の未決事項(`contract-summary.md`) | それぞれの Bolt の設計 |
