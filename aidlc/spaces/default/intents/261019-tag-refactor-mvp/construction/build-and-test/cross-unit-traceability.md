# 作業単位をまたぐ対応の確かめ

## 判定: **不合格**(未カバーの ID があります)

要件定義の FR 64 件と NFR 12 件、ストーリーの受け入れ条件 66 件を、全作業単位(U1〜U8)の `code-generation/traceability.json` と突き合わせました。

| 判定 | 件数 |
|---|---|
| OK | 133 |
| 一部 | 1 |
| 未カバー | 8 |

未カバーと一部は、どれも人が決めた範囲の外のものです(下の表)。範囲の中の ID には、対応表の上で未カバーのものはありません(テストの中身が十分かは、ここでは判定していません)。

## 突き合わせの方法

- **受け入れ条件:** 各作業単位の対応表で、状態が `OK` で、対象のファイルが存在すれば OK とします。`Deferred` は、後の作業単位に `OK` があるときだけ OK です。
- **FR:** 対応表は受け入れ条件と細かい NFR を単位にしているので、`stories.md` の表の「ストーリーと要件」の対応を通して、そのストーリーの受け入れ条件がすべて OK のときに OK とします。親の FR は、子のストーリーをまとめて見ます。
- **NFR:** `NFR3` を `NFR3.1` などの細かい要件で見ます。`NFR12` だけは、細かい要件 `NFR12.1` が配布そのものではないので、未カバーとします。

## 未カバーと一部

| ID | 判定 | 持ち主 | 対象のファイル | 注 |
|---|---|---|---|---|
| AC6.2.1 | 未カバー | — | — | 配布は作業単位の外(units-generation Q3、Q4)。どの作業単位にも割り当てていない |
| AC6.2.2 | 未カバー | — | — | 配布は作業単位の外(units-generation Q3、Q4)。どの作業単位にも割り当てていない |
| AC6.2.3 | 未カバー | — | — | 配布は作業単位の外(units-generation Q3、Q4)。どの作業単位にも割り当てていない |
| AC7.2.1 | 未カバー | — | — | 作っていない(U8 の機能設計 Q1。優先度 Could の第3弾。後の作業) |
| AC7.3.1 | 未カバー | — | — | 作っていない(U8 の機能設計 Q1。優先度 Could の第3弾。後の作業) |
| FR8 | 一部 | u8-iac | internal/cli/acceptance_u8_test.go | 受け入れ条件 4 件のうち 2 件。未カバー: AC7.2.1、AC7.3.1 |
| FR8.3 | 未カバー | — | — | US7.2 を作っていない(U8 の機能設計 Q1。優先度 Could の第3弾。後の作業) |
| FR8.4 | 未カバー | — | — | US7.3 を作っていない(U8 の機能設計 Q1。優先度 Could の第3弾。後の作業) |
| NFR12 | 未カバー | — | — | 公開の仕組みがない。NFR12.1(U8)は hcl/v2 のライセンスの記載だけで、配布そのものではない。配布は作業単位の外 |

## 確かめの途中で直したもの

- U5 の `BR1.2` の対象 `internal/awsaccess/inline.go` は存在しないファイルでした。実際の `internal/awsaccess/read.go` に直しました。

## すべての ID

### 受け入れ条件

| ID | 判定 | 持ち主 | 対象のファイル | 注 |
|---|---|---|---|---|
| AC1.1.1 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.1.2 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.1.3 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.1.4 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.1.5 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.1.6 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.1.7 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.1.8 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.1.9 | OK | u2-scan | internal/cli/performance_u2_test.go |  |
| AC1.1.10 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.2.1 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.2.2 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.2.3 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC1.2.4 | OK | u2-scan | internal/cli/acceptance_test.go |  |
| AC1.2.5 | OK | u2-scan | internal/cli/acceptance_u2_test.go |  |
| AC2.1.1 | OK | u3-rules | internal/cli/acceptance_u3_test.go |  |
| AC2.1.2 | OK | u3-rules | internal/cli/acceptance_u3_test.go |  |
| AC2.1.3 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC2.1.4 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC2.1.5 | OK | u3-rules | internal/cli/acceptance_u3_test.go |  |
| AC3.1.1 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC3.1.2 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC3.1.3 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC3.1.4 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC3.1.5 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC3.1.6 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC3.1.7 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC3.1.8 | OK | u4-plan | internal/cli/acceptance_u4_test.go |  |
| AC3.2.1 | OK | u5-impact | internal/cli/acceptance_u5_test.go |  |
| AC3.2.2 | OK | u5-impact | internal/cli/acceptance_u5_test.go |  |
| AC3.2.3 | OK | u5-impact | internal/cli/acceptance_u5_test.go |  |
| AC3.2.4 | OK | u5-impact | internal/cli/acceptance_u5_test.go |  |
| AC3.2.5 | OK | u5-impact | internal/cli/acceptance_u5_test.go |  |
| AC3.2.6 | OK | u5-impact | internal/impact/detection_test.go |  |
| AC4.1.1 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.1.2 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.1.3 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.1.4 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.1.5 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.1.6 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.1.7 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.2.1 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.2.2 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.2.3 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.2.4 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.2.5 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.2.6 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC4.2.7 | OK | u6-apply | internal/cli/acceptance_u6_test.go |  |
| AC5.1.1 | OK | u7-rollback | internal/cli/acceptance_u7_test.go |  |
| AC5.1.2 | OK | u7-rollback | internal/cli/acceptance_u7_test.go |  |
| AC5.1.3 | OK | u7-rollback | internal/cli/acceptance_u7_test.go |  |
| AC5.1.4 | OK | u7-rollback | internal/cli/acceptance_u7_test.go |  |
| AC5.1.5 | OK | u7-rollback | internal/cli/acceptance_u7_test.go |  |
| AC5.1.6 | OK | u7-rollback | internal/cli/acceptance_u7_test.go |  |
| AC6.1.1 | OK | u7-rollback | internal/cli/acceptance_u7_test.go |  |
| AC6.1.2 | OK | u7-rollback | internal/display/rollback.go |  |
| AC6.1.3 | OK | u7-rollback | internal/display/rollback.go |  |
| AC6.1.4 | OK | u7-rollback | internal/apply/rollback.go |  |
| AC6.1.5 | OK | u7-rollback | internal/display/rollback.go |  |
| AC6.2.1 | 未カバー | — | — | 配布は作業単位の外(units-generation Q3、Q4)。どの作業単位にも割り当てていない |
| AC6.2.2 | 未カバー | — | — | 配布は作業単位の外(units-generation Q3、Q4)。どの作業単位にも割り当てていない |
| AC6.2.3 | 未カバー | — | — | 配布は作業単位の外(units-generation Q3、Q4)。どの作業単位にも割り当てていない |
| AC7.1.1 | OK | u8-iac | internal/cli/acceptance_u8_test.go |  |
| AC7.1.2 | OK | u8-iac | internal/cli/acceptance_u8_test.go |  |
| AC7.2.1 | 未カバー | — | — | 作っていない(U8 の機能設計 Q1。優先度 Could の第3弾。後の作業) |
| AC7.3.1 | 未カバー | — | — | 作っていない(U8 の機能設計 Q1。優先度 Could の第3弾。後の作業) |

### FR

| ID | 判定 | 持ち主 | 対象のファイル | 注 |
|---|---|---|---|---|
| FR1 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.1 の受け入れ条件 10 件 |
| FR1.1 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.1 の受け入れ条件 10 件 |
| FR1.2 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.1 の受け入れ条件 10 件 |
| FR1.3 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.1 の受け入れ条件 10 件 |
| FR1.4 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.1 の受け入れ条件 10 件 |
| FR1.5 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.1 の受け入れ条件 10 件 |
| FR1.6 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.1 の受け入れ条件 10 件 |
| FR1.7 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.1 の受け入れ条件 10 件 |
| FR1.8 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.1 の受け入れ条件 10 件 |
| FR2 | OK | u3-rules | internal/cli/acceptance_u3_test.go | US2.1 の受け入れ条件 5 件 |
| FR2.1 | OK | u3-rules | internal/cli/acceptance_u3_test.go | US2.1 の受け入れ条件 5 件 |
| FR2.2 | OK | u3-rules | internal/cli/acceptance_u3_test.go | US2.1 の受け入れ条件 5 件 |
| FR2.3 | OK | u3-rules | internal/cli/acceptance_u3_test.go | US2.1 の受け入れ条件 5 件 |
| FR2.4 | OK | u3-rules | internal/cli/acceptance_u3_test.go | US2.1 の受け入れ条件 5 件 |
| FR2.5 | OK | u3-rules | internal/cli/acceptance_u3_test.go | US2.1 の受け入れ条件 5 件 |
| FR3 | OK | u4-plan | internal/cli/acceptance_u4_test.go | US3.1 の受け入れ条件 8 件 |
| FR3.1 | OK | u4-plan | internal/cli/acceptance_u4_test.go | US3.1 の受け入れ条件 8 件 |
| FR3.2 | OK | u4-plan | internal/cli/acceptance_u4_test.go | US3.1 の受け入れ条件 8 件 |
| FR3.3 | OK | u4-plan | internal/cli/acceptance_u4_test.go | US3.1 の受け入れ条件 8 件 |
| FR3.4 | OK | u4-plan | internal/cli/acceptance_u4_test.go | US3.1 の受け入れ条件 8 件 |
| FR3.5 | OK | u4-plan | internal/cli/acceptance_u4_test.go | US3.1 の受け入れ条件 8 件 |
| FR3.6 | OK | u4-plan | internal/cli/acceptance_u4_test.go | US3.1 の受け入れ条件 8 件 |
| FR3.7 | OK | u4-plan | internal/cli/acceptance_u4_test.go | US3.1 の受け入れ条件 8 件 |
| FR3.8 | OK | u4-plan | internal/cli/acceptance_u4_test.go | US3.1、US3.2 の受け入れ条件 14 件 |
| FR4 | OK | u5-impact | internal/cli/acceptance_u5_test.go | US3.2 の受け入れ条件 6 件 |
| FR4.1 | OK | u5-impact | internal/cli/acceptance_u5_test.go | US3.2 の受け入れ条件 6 件 |
| FR4.2 | OK | u5-impact | internal/cli/acceptance_u5_test.go | US3.2 の受け入れ条件 6 件 |
| FR4.3 | OK | u5-impact | internal/cli/acceptance_u5_test.go | US3.2 の受け入れ条件 6 件 |
| FR4.4 | OK | u5-impact | internal/cli/acceptance_u5_test.go | US3.2 の受け入れ条件 6 件 |
| FR4.5 | OK | u5-impact | internal/cli/acceptance_u5_test.go | US3.2 の受け入れ条件 6 件 |
| FR4.6 | OK | u5-impact | internal/cli/acceptance_u5_test.go | US3.2 の受け入れ条件 6 件 |
| FR4.7 | OK | u5-impact | internal/cli/acceptance_u5_test.go | US3.2 の受け入れ条件 6 件 |
| FR5 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.1、US4.2 の受け入れ条件 14 件 |
| FR5.1 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.1 の受け入れ条件 7 件 |
| FR5.2 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.1 の受け入れ条件 7 件 |
| FR5.3 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.1 の受け入れ条件 7 件 |
| FR5.4 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.1 の受け入れ条件 7 件 |
| FR5.5 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.1 の受け入れ条件 7 件 |
| FR5.6 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.2 の受け入れ条件 7 件 |
| FR5.7 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.2 の受け入れ条件 7 件 |
| FR5.8 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.2 の受け入れ条件 7 件 |
| FR5.9 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.2 の受け入れ条件 7 件 |
| FR5.10 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.2 の受け入れ条件 7 件 |
| FR5.11 | OK | u6-apply | internal/cli/acceptance_u6_test.go | US4.1 の受け入れ条件 7 件 |
| FR6 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US5.1 の受け入れ条件 6 件 |
| FR6.1 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US5.1 の受け入れ条件 6 件 |
| FR6.2 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US5.1 の受け入れ条件 6 件 |
| FR6.3 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US5.1 の受け入れ条件 6 件 |
| FR6.4 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US5.1 の受け入れ条件 6 件 |
| FR6.5 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US5.1 の受け入れ条件 6 件 |
| FR7 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.2、US6.1 の受け入れ条件 10 件 |
| FR7.1 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US6.1 の受け入れ条件 5 件 |
| FR7.2 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US6.1 の受け入れ条件 5 件 |
| FR7.3 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US6.1 の受け入れ条件 5 件 |
| FR7.4 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US6.1 の受け入れ条件 5 件 |
| FR7.5 | OK | u7-rollback | internal/cli/acceptance_u7_test.go | US6.1 の受け入れ条件 5 件 |
| FR7.6 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.2 の受け入れ条件 5 件 |
| FR7.7 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.2 の受け入れ条件 5 件 |
| FR7.8 | OK | u2-scan | internal/cli/acceptance_u2_test.go | US1.2 の受け入れ条件 5 件 |
| FR8 | 一部 | u8-iac | internal/cli/acceptance_u8_test.go | 受け入れ条件 4 件のうち 2 件。未カバー: AC7.2.1、AC7.3.1 |
| FR8.1 | OK | u8-iac | internal/cli/acceptance_u8_test.go | US7.1 の受け入れ条件 2 件 |
| FR8.2 | OK | u8-iac | internal/cli/acceptance_u8_test.go | US7.1 の受け入れ条件 2 件 |
| FR8.3 | 未カバー | — | — | US7.2 を作っていない(U8 の機能設計 Q1。優先度 Could の第3弾。後の作業) |
| FR8.4 | 未カバー | — | — | US7.3 を作っていない(U8 の機能設計 Q1。優先度 Could の第3弾。後の作業) |

### NFR

| ID | 判定 | 持ち主 | 対象のファイル | 注 |
|---|---|---|---|---|
| NFR1 | OK | u1-walking-skeleton | internal/display/display.go | 細かい要件 19/19 件(NFR1.1〜NFR1.19) |
| NFR2 | OK | u5-impact | internal/impact/detection_test.go | 細かい要件 2/2 件(NFR2.1〜NFR2.2) |
| NFR3 | OK | u6-apply | internal/cli/apply_internal_test.go | 細かい要件 21/21 件(NFR3.1〜NFR3.21) |
| NFR4 | OK | u6-apply | internal/journal/journal_test.go | 細かい要件 4/4 件(NFR4.1〜NFR4.4) |
| NFR5 | OK | u2-scan | internal/awsaccess/session.go | 細かい要件 26/26 件(NFR5.1〜NFR5.26) |
| NFR6 | OK | u8-iac | internal/journal/journal_test.go | 細かい要件 6/6 件(NFR6.1〜NFR6.6) |
| NFR7 | OK | u7-rollback | internal/journal/lock.go | 細かい要件 2/2 件(NFR7.1〜NFR7.2) |
| NFR8 | OK | u2-scan | internal/awsaccess/invoke.go | 細かい要件 27/27 件(NFR8.1〜NFR8.27) |
| NFR9 | OK | u2-scan | internal/cli/acceptance_u2_test.go | 細かい要件 12/12 件(NFR9.1〜NFR9.12) |
| NFR10 | OK | u2-scan | .github/workflows/ci.yml | 細かい要件 14/14 件(NFR10.1〜NFR10.14) |
| NFR11 | OK | u1-walking-skeleton | internal/display/results.go | 細かい要件 10/10 件(NFR11.1〜NFR11.10) |
| NFR12 | 未カバー | — | — | 公開の仕組みがない。NFR12.1(U8)は hcl/v2 のライセンスの記載だけで、配布そのものではない。配布は作業単位の外 |
