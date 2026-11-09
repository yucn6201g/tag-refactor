# 発見したルール: tag-refactor(第2弾)

## Mandated

None.

## Forbidden

- NEVER 公開のリポジトリ(tag-refactor)とテスト専用のリポジトリに、トークンなどの秘密の値を置かない(コミットするファイル、ワークフローの定義、テストのデータのどれにも書かず、CI の secret か手元の環境にだけ置く)。
