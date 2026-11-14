# Inception から Construction への確認

**判定: PASS**

Inception の各ステージが作った `traceability.json` を読み、要件からストーリー、部品、作業の単位までのつながりに抜けがないかを確かめました。`GAP`、`ORPHAN`、存在しない対象、上流の ID の抜けは、ありません。

## 確認した表

| ステージ | ファイル | 上流の ID の数 | OK | Deferred | GAP / ORPHAN |
|---|---|---|---|---|---|
| User Stories | `inception/user-stories/traceability.json` | 46(FR1〜FR3 とその細目、NFR1〜NFR10) | 42 | 4 | 0 |
| Domain Design | `inception/domain-design/traceability.json` | 5(US1.1、US2.1、US3.1、US3.2、US4.1) | 5 | 0 | 0 |
| Units Generation | `inception/units-generation/traceability.json` | 5(US1.1、US2.1、US3.1、US3.2、US4.1) | 5 | 0 | 0 |

Contract Design は `traceability.json` を作りません(要件のつながりではなく、形式の契約を受け持つため)。そのため、この確認には含めていません。

## Deferred の項目

ストーリーではなく、後のステージで扱う非機能の要件です。抜けではありません。

| ID | 扱うステージ |
|---|---|
| NFR4 | nfr-requirements |
| NFR5 | build-and-test |
| NFR6 | nfr-requirements |
| NFR7 | build-and-test |

## Construction の前に残っていること

抜けではありませんが、次のことは Construction の中で決めます。

| 未決事項 | 決める時期 |
|---|---|
| 契約の設計のレビューの指摘 R-15〜R-17(CDK のソースの書き換えの細部) | U4 と U5 の設計 |
| 契約の未決事項(trace の設定、manifest の読み方、`needsAWS` の文の一覧、補助のプログラムの埋め込み方など) | U1、U3、U4 の設計 |
| Construction の検証コマンド | U1 の検証の前 |
| 外の依存 E1(テスト専用のリポジトリとトークン)、E2(検証用のアカウントの最小の CDK アプリ) | U1 の Code Generation の前 |
