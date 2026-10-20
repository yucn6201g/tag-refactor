# 開発の進め方 — 質問

このプロジェクトでの開発の進め方(取り込み方、最初に作る一本、テスト、リリース、コードの書き方)を決めます。まだコードがないため、AI-DLC の既定値(`aidlc/spaces/default/memory/org.md`)を「推奨」として示しています。ここで決めた内容は、承認後に `aidlc/spaces/default/memory/team.md` に記録され、以降の作業の基準になります。

## 進め方

## Q1. 作業のまとまりを `main` に取り込む方法

1つの作業のまとまりが終わったら、どうやって `main` に取り込みますか?

A. 手元で `main` に squash マージしてから push する(AI-DLC の既定に近い。ただし GitHub の CI はマージの後にしか走らない)
B. GitHub にブランチを push し、プルリクエストで squash マージする(マージ前に CI が走る)
C. A を基本にし、外部からの貢献だけプルリクエストで受ける
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q2. コミットメッセージの書き方

コミットメッセージの書き方を決めますか?

A. Conventional Commits(`feat:`、`fix:` などの接頭辞)に従う。リリースノートの自動作成に使える
B. 決めない
C. Not yet defined
X. Other (please specify)

[Answer]: A

## 最初に作る一本

## Q3. 端から端まで動く最小の一本を先に作るか

最初に、端から端まで動く最小の一本を作りますか? walking skeleton とは、全体を通して動く最小版のことです。本格的な機能を入れる前に、部品同士がつながることを確かめるために最初に作ります。

A. 作る(`mvp` の既定)
B. 作らない
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q4. 最小の一本に含めるもの

最小の一本(Q3)を作る場合、何を含めますか?

A. `scan` の最小版(1リージョンのタグを読んで表に出す)に、ビルド、テスト、lint、CI を加えたもの
B. A に `plan`(影響分析)の最小版も加えたもの
C. Not applicable(最小の一本は作らない)
D. Not yet defined
X. Other (please specify)

[Answer]: B

## Q5. 最小の一本が動いたことの確かめ方

最小の一本が動いたことを、どう確かめますか?

A. 個人の検証用AWSアカウントを実際に呼ぶ(認証情報と少額の費用が必要)
B. 手元で用意した偽のAWSの応答で確かめる(認証情報は不要)
C. 通常は B で確かめ、最小の一本を承認するときだけ A も実行する
D. Not yet defined
X. Other (please specify)

[Answer]: A

## テスト

## Q6. テストを書く順番

テストはいつ書きますか?

A. 実装のあとにテストを書く(test-after。`mvp` の既定)
B. 先にテストを書く(TDD。Go の学習に役立つが、慣れるまで時間がかかる)
C. 受け入れ条件(Given/When/Then)を先にテストにし、細かい単体テストは実装のあとに書く
D. Not yet defined
X. Other (please specify)

[Answer]: C

## Q7. カバレッジの下限

テストで通るコードの割合(カバレッジ)の下限をどうしますか? AI-DLC の既定は80%です。

A. 80%。Go の標準の測り方(文単位)で測る
B. 80%。ただし、コマンドの入口など配線だけの部分は測る対象から外す
C. 別の値にする
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q8. AWS を呼ぶ部分のテスト

AWS を呼ぶ部分は、どうテストしますか?

A. 単体テストでは AWS の呼び出しを差し替えて CI で実行し、実アカウントを使う結合テストは手元だけで実行する
B. CI からも検証用アカウントで結合テストを実行する(GitHub と AWS の連携設定が必要。外部からのプルリクエストでは注意が必要)
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q9. テストを実行するOS

CI でテストを実行するOSはどれにしますか?

A. Linux、macOS、Windows の3つ
B. Linux だけ(ビルドは3つのOS向けに行う)
C. Not yet defined
X. Other (please specify)

[Answer]: A

## リリース

## Q10. リリースの流れ

リリースの流れはどうしますか?

A. `main` に取り込むたびに確認用のバイナリをビルドし、人が `vX.Y.Z` のタグを付けたときだけ公開する
B. `main` に取り込むたびに自動で公開する
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q11. 配り方

どの方法でツールを配りますか?(select all that apply)

A. GitHub Releases(OSごとのバイナリ)
B. `go install`
C. Homebrew(macOS、Linux)
D. Scoop(Windows)
E. Not yet defined
X. Other (please specify)

[Answer]: A, B

## Q12. 最初の公開の時期

最初のバージョンはいつ公開しますか?

A. 第1弾(scan と plan)が終わった時点で `v0.1.0` を出す
B. 最初のバージョンの完成(第1弾と第2弾)の時点で出す
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q13. 配布物の安全性

配るバイナリの安全性はどこまで確保しますか?

A. チェックサムだけ付ける
B. チェックサムと署名(cosign など)を付ける
C. B に、ビルドの来歴の証明(SLSA provenance)を加える
D. Not yet defined
X. Other (please specify)

[Answer]: A

## コードの書き方

## Q14. 整形の道具

Go のコードの整形には何を使いますか?

A. `gofmt` のみ
B. `goimports`(`gofmt` に import の整理を加える)
C. `gofumpt`(より厳しい整形)
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q15. lint の厳しさ

コードの問題を自動で見つける lint(`golangci-lint`)の厳しさをどうしますか?

A. 既定の検査のみ
B. 既定の検査に、セキュリティ(`gosec`)、エラー処理(`errorlint`)、書き方(`revive`)の検査を加える
C. 多くの検査を有効にして厳しくする
D. Not yet defined
X. Other (please specify)

[Answer]: A

## Q16. 改行コード

Windows で開発するため、改行コードの扱いを決めます。どうしますか?

A. `.gitattributes` で Go のファイルの改行を LF に固定する(整形の確認で差分が出るのを防ぐ)
B. 何もしない
C. Not yet defined
X. Other (please specify)

[Answer]: A

## Q17. 依存ライブラリの確認

使っているライブラリの脆弱性や更新を、自動で確認しますか?(select all that apply)

A. `govulncheck`(既知の脆弱性の確認)を CI で実行する
B. Dependabot(更新のプルリクエストを自動で作る)を使う
C. 確認しない
D. Not yet defined
X. Other (please specify)

[Answer]: A, B

## Q18. マージ前の確認(Q1の補足)

AI-DLC の既定では、`mvp` は「マージ前にCIを通す」ことになっています。一方、Q1では「手元で `main` に squash マージしてから push する」を選んだため、GitHub のCIはマージの後にしか走りません。この食い違いをどう解消しますか?

A. CIと同じ確認(ビルド、テスト、カバレッジ、lint)を、マージ前に手元で実行することで「マージ前の確認」とする。GitHub のCIは push 後の再確認とする
B. 「マージ前にCIを通す」は求めず、push 後のCIで問題が見つかったら直す
C. Q1を見直し、プルリクエストでマージする方法に変える
D. Not yet defined
X. Other (please specify)

[Answer]: C

## Consolidated Summary Confirmation

回答のまとめ:

- 取り込み方(Q1、Q18で変更): GitHub にブランチを push し、プルリクエストで `main` に squash マージする。マージ前にCIが走る
- コミットメッセージ(Q2): Conventional Commits に従う
- 最小の一本(Q3、Q4): 作る。`scan` と `plan` の最小版に、ビルド、テスト、lint、CI を加えたもの
- 最小の一本の確かめ方(Q5): 個人の検証用AWSアカウントを実際に呼ぶ
- テストの順番(Q6): 受け入れ条件(Given/When/Then)を先にテストにし、細かい単体テストは実装のあとに書く
- カバレッジ(Q7): 80%。Go の標準の測り方(文単位)で測る
- AWS を呼ぶ部分のテスト(Q8): 単体テストは AWS の呼び出しを差し替えてCIで実行し、実アカウントの結合テストは手元だけで実行する
- テストのOS(Q9): Linux、macOS、Windows の3つ
- リリース(Q10): `main` に取り込むたびに確認用のバイナリをビルドし、`vX.Y.Z` のタグを付けたときだけ公開する
- 配り方(Q11): GitHub Releases と `go install`
- 最初の公開(Q12): 第1弾(scan と plan)が終わった時点で `v0.1.0`
- 配布物の安全性(Q13): チェックサムだけ付ける
- 整形(Q14): `gofmt` のみ
- lint(Q15): `golangci-lint` の既定の検査のみ
- 改行コード(Q16): `.gitattributes` で Go のファイルを LF に固定する
- 依存ライブラリの確認(Q17): `govulncheck` をCIで実行し、Dependabot を使う

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
