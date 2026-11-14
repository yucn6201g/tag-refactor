# 意図の文書: iac への CDK の反映とプルリクエストの作成

## Problem Statement

- tag-refactor の `iac` コマンドに、CDK のソース(TypeScript と Python)へのタグの変更の反映(FR8.3、AC7.2.1)と、反映した変更から GitHub のプルリクエストを作る機能(FR8.4、AC7.3.1)を足します。[desc]
- 解決したい問題は次の2つで、どちらも同じくらい大事です。[Q1]
  - CDK で作ったリソースのタグを `apply` で変えても、CDK のソースが古いままなので、次の `cdk deploy` で元に戻ってしまう。[Q1]
  - `iac --write` のあと、プルリクエストを出すまでの手作業(ブランチ、コミット、push、作成)が手間で、チームのレビューに乗せにくい。[Q1]

## Target Customer

- 利用者は、前の intent の利用者像 P1(佐藤さん)と同じです。CDK で作ったリソースを持つチームのタグを整理し、IaC も直す人です。[Q2]
- 利用者が得るものは、CDK のソースを AWS のタグと同じ変更にそろえられることと、その修正をプルリクエストとしてチームのレビューに出せることです。[Q1] [Q2]

## Success Metrics

| 指標 | 合否の判断 | Source |
|---|---|---|
| CDK のソースへの反映 | 受け入れ条件 AC7.2.1(CDK のソースが、変更後のタグを表すように書き換えられる)を自動テストで満たす | [Q3] |
| プルリクエストの作成 | 受け入れ条件 AC7.3.1(修正を含むプルリクエストが GitHub に作られる)を自動テストで満たす | [Q3] |
| 数の目標 | 置かない | [Q3] |

## Initiative Trigger

- 第1弾(`v0.1.0`)を公開したので、U8 の機能設計で後回しにした優先度 Could の2つ(FR8.3 と FR8.4)に取りかかる順番が来ました。[Q4] [desc]

## Initial Scope Signal

| 区分 | 内容 | Source |
|---|---|---|
| 計画(workflow-selected) | `mvp` | [scope] |
| 作るもの | CDK のソース(TypeScript と Python)へのタグの変更の反映と、反映した変更からの GitHub のプルリクエストの作成 | [desc] |
| この版で作らないもの(利用者が確かめたもの) | 特になし | [Q8] |

## Assumptions & Open Questions

None.
