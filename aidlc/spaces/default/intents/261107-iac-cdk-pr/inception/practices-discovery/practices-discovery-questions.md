# 進め方の確認の質問: iac への CDK の反映とプルリクエストの作成

前の作業で決めたチームの進め方(`aidlc/spaces/default/memory/team.md`)を基準にし、リリースの担当が下書き(`team-practices.md`、`discovered-rules.md`)と調べた内容(`evidence.md`)を作りました。証拠だけでは決められない点を聞きます。調べたものは、コードの記録(`code-structure.md`、`technology-stack.md`、`dependencies.md`、`code-quality-assessment.md`、`architecture.md`、`business-overview.md`)、git の履歴、GitHub の設定、`origin/main` の CI です。

## Q1. 作業の進め方に書き足す4点

GitHub の設定と履歴から、次の4点を進め方に書き足す案にしました。(1) GitHub の設定で守られていること(squash マージだけ、`main` への直接の push と強制の push の禁止、CI の `ci-gate` が必須、マージしたブランチの自動の削除)、(2) プルリクエストのレビューの承認は必須にしない、(3) AI-DLC の記録はコードと同じプルリクエストで入れる、(4) 作業ブランチは GitHub の最新の `main` から作る(手元の `main` が古いことがあるため)。

A. 4点すべてを書き足す
B. (1) と (4) だけを書き足す
C. 何も書き足さない(今のままにする)
X. Other (please specify)

[Answer]: A

## Q2. 最初に、端から端まで動く最小の一本を作りますか?

walking skeleton(最小の一本)は、全体を端から端まで通る最小の版で、本格的な機能を入れる前に、部品がつながることを確かめるために最初に作るものです。前の作業では `scan` と `plan` の最小版でした。今回はどうしますか?

A. 作る。中身は、TypeScript の最小の CDK アプリで1つのキー名の変更を反映し、テスト専用のリポジトリにプルリクエストを作るところまで(2つの機能を細く通す)
B. 作る。中身は、プルリクエストの作成だけ(今の Terraform と CloudFormation の書き換えから、テスト専用のリポジトリにプルリクエストを作る)。範囲の定義で決めた「プルリクエストを先に」の順番に合わせる
C. 作らない(最初の作業の単位から本格的に作る)
D. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q3. 最小の一本が動いたことを確かめるコマンド

A. AWS を呼ばない。保存した変更案の例で `iac` を動かし、テスト専用のリポジトリにプルリクエストができることを確かめる(Node.js、CDK の CLI、`gh` が要る)
B. 検証用の AWS アカウントで、`plan` で変更案を作るところから通す(AWS の認証情報と少額の費用が要る)
C. まだ決めていない
X. Other (please specify)

[Answer]: B

## Q4. CI で実際の GitHub を使うことと、CI に Node.js、Python、CDK を入れること

この作業の実現性の段階で、CI でもテスト専用のリポジトリに実際のプルリクエストを作ること、CI の3つの OS に Node.js、Python、CDK を入れて `cdk synth` のテストを動かすことを決めました。これをチームの進め方として残しますか?

A. チームの進め方にする(このあとの作業でも守る)
B. この作業だけの決定にする(チームの進め方には書かない)
C. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q5. `cdk synth` のテストで、Node.js、Python、CDK が無いとき

A. CI では失敗にする(テストを飛ばして通さない)。手元では、理由を出して飛ばす
B. どこでも失敗にする
C. どこでも、理由を出して飛ばす
D. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q6. テストで使う CDK のアプリの依存の版

テストのデータとして、TypeScript と Python の小さな CDK のアプリを置きます。そのアプリの依存(npm と pip のパッケージ、CDK の CLI)の版をどう扱いますか?

A. 版を固定し、Dependabot で更新する
B. 版を固定し、人が手で更新する
C. 版を固定しない(いつも最新を入れる)
D. まだ決めていない
X. Other (please specify)

[Answer]: A

## Q7. TypeScript と Python のファイルの整形と lint

A. テストのデータの CDK のアプリは、整形も lint もしない。tag-refactor 自身が TypeScript や Python の補助のプログラムを持つ場合は、その言語の標準の整形(Prettier、Black)を CI で確かめる
B. どの TypeScript と Python のファイルも、整形も lint もしない
C. まだ決めていない(設計で、補助のプログラムを持つかどうかが決まってから)
X. Other (please specify)

[Answer]: A

## Q8. 公開の進め方の書き換え

「最初に公開するのは `v0.1.0`」の行を、「`v0.1.0` は公開済み。次の `v0.2.0` は CDK とプルリクエストの2つがそろったら」に書き換え、公開のワークフローが確かめていること(タグが `main` にあること、Go の版、版の一致)を書き足す案にしました。また、CI の確認用のバイナリは5種類、公開するバイナリは6種類(`windows/arm64` は公開だけ)になっています。

A. 下書きのとおりにする(5種類と6種類の違いは意図したもの)
B. 下書きのとおりにし、CI の確認用のバイナリも6種類にそろえる
C. まだ決めていない
X. Other (please specify)

[Answer]: B

## Q9. 守る規則(必ずすること、してはいけないこと)

今は、プロジェクトの規則は「なし」です。次のものを規則にしますか?

A. 「公開のリポジトリとテスト専用のリポジトリに、秘密の値を置かない」を、してはいけないことの規則にする
B. 規則にはしない(今のまま「なし」)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- 作業の進め方に4点を書き足す: GitHub の設定で守られていること、レビューの承認は必須にしない、AI-DLC の記録はコードと同じプルリクエストで入れる、作業ブランチは GitHub の最新の `main` から作る(Q1: A)
- 最初に最小の一本を作る。中身は、TypeScript の最小の CDK アプリで1つのキー名の変更を反映し、テスト専用のリポジトリにプルリクエストを作るところまで(Q2: A)
- 最小の一本の確かめは、検証用の AWS アカウントで `plan` を作るところから通す(AWS の認証情報と少額の費用が要る)(Q3: B)
- CI で実際の GitHub にプルリクエストを作ることと、CI に Node.js、Python、CDK を入れて `cdk synth` のテストを動かすことを、チームの進め方にする(Q4: A)
- `cdk synth` のテストで道具が無いとき、CI では失敗にし、手元では理由を出して飛ばす(Q5: A)
- テストで使う CDK のアプリの依存は、版を固定し Dependabot で更新する(Q6: A)
- テストのデータの CDK のアプリは整形も lint もしない。tag-refactor 自身が TypeScript や Python の補助のプログラムを持つ場合は、Prettier と Black を CI で確かめる(Q7: A)
- 公開の進め方を下書きのとおりに書き換え、CI の確認用のバイナリも6種類にそろえる(Q8: B)
- 「公開のリポジトリとテスト専用のリポジトリに、秘密の値を置かない」を、してはいけないことの規則にする(Q9: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
