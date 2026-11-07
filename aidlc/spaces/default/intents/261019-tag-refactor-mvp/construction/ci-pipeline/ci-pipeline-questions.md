# CI Pipeline の質問

CI はすでに動いています(`.github/workflows/ci.yml`、`scheduled.yml`、`.github/dependabot.yml`)。U8 のプルリクエスト #20 でも、すべてのジョブが通りました。ここでは、足りないものだけを決めます。

## すでに決まっていること(質問しません)

- **CI の道具:** GitHub Actions(U1 のインフラの設計)。AWS の CodePipeline などは使わない。
- **ブランチの進め方:** トランクベース。短命なブランチからプルリクエストで `main` に squash マージする(チームの進め方)。
- **マージの前の確かめ:** `test`(3つの OS × Go 2版)、`race`、`coverage`(80%)、`lint`、`vuln`、`modcheck`、`build` を `ci-gate` でまとめる(U1 のインフラの設計)。
- **成果物の置き場所:** `main` への取り込みのときだけ、GitHub Actions の成果物に7日間残す。公開しない。ECR や CodeArtifact、S3 は使わない。
- **配布:** GitHub Releases と `go install` の仕組みは別の作業(units-generation の Q3、Q4)。Build and Test でも、未達のまま進むと人が決めた(2026-11-07)。この段階では作らない。

## 見つかったこと

`main` のルールセットは有効で、プルリクエストの必須、削除と強制 push の禁止は効いています。ただし、**必須のステータスチェックの一覧が空です。** U1 のインフラの設計は「`ci-gate` だけを必須にする」としていて、`ci.yml` のコメントもそう書いていますが、設定されていません。チームの決まりは「CI が通ったものだけをマージする」です。今は、CI が赤でもマージできます。Dependabot の #11 と #12 は、CI が赤のまま `main` に入り、そのあと `main` の CI が U7 まで赤のままでした。

## Q1. `ci-gate` の必須の設定

`main` のルールセットの必須のステータスチェックに、`ci-gate` を足しますか?

A. 足す。人の許可を得て、GitHub の API でルールセットを1か所だけ更新する。CI が通らないプルリクエストはマージできなくなる。管理者も例外にしない(今のルールセットと同じ)
B. 足さない。今までどおり、人が `ci-gate` を確かめてからマージする。その旨を文書に書き、`ci.yml` のコメントを直す
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

Does this all look correct before I generate the artifact?

- `ci.yml` と `scheduled.yml`、`dependabot.yml` は変えない(#20 のものをそのまま使う)
- `main` のルールセットの必須のステータスチェックに `ci-gate` を足す(ほかの規則は変えない)
- 成果物の置き場所は GitHub Actions の7日間のまま。公開の仕組みは作らない(別の作業)
- `ci-config.md` と `quality-gates.md` に今の CI と関門を書き、Construction の終わりの確認を `verification/phase-check-construction.md` に書く。範囲の外の未カバーの ID は隠さず書く

- Looks correct
- Request changes

[Answer]: Looks correct
