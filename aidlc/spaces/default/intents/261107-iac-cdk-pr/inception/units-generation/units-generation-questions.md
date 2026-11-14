# 作業の単位の分割の質問: iac への CDK の反映とプルリクエストの作成

部品(`components.md`)、設計の決定(`decisions.md`)、要件(`requirements.md`)、ストーリー(`stories.md`)をもとに、作業の単位の分け方を聞きます。配り方は1つのバイナリ(前の作業と同じ)なので、各単位は実行ファイルに組み込まれます。チームの進め方で、最初の単位は「CDK の反映とプルリクエストの作成を細く通す最小の一本」と決まっています(`team.md` の Walking Skeleton)。作る順番(価値や危険のどちらを先にするか)は、このあとの作業の計画の段階で決めます。

## Q1. 単位の分け方

A. 6つに分ける。U1 最小の一本、U2 プルリクエストの作成(残り)、U3 CDK の synth と対応づけ(残り)、U4 TypeScript への反映、U5 Python への反映、U6 反映できないものの一覧とその他の書き方
B. 4つに分ける。U1 最小の一本、U2 プルリクエストの作成(残り)、U3 CDK(synth、TypeScript、Python をまとめる)、U4 反映できないものの一覧とその他の書き方
C. 任せる
X. Other (please specify)

[Answer]: A(利用者の回答「任せる」を受けて選んだ。ストーリーと1対1に近く、1つの単位が大きくなりすぎないため)

## Q2. 最小の一本(U1)の中身

A. TypeScript の CDK アプリ1つで、`Tags.of(x).add` の文字列のキーの名前の変更を、synth の対応を使って反映し、`--pr` でテスト専用のリポジトリにプルリクエストを作るまで。synth の認証情報の遮断と時間の上限、`--pr` の書き換え前の確かめのうち最低限(gh とリモートとブランチ)を含む。CI の準備(3つの OS への Node.js、Python、CDK の CLI の導入、テスト専用のリポジトリの secret と実際のプルリクエストのテスト)も含む
B. A と同じだが、CI の実際のプルリクエストのテストと secret の準備は U2 に回す(テスト専用のリポジトリとトークンの用意を待たずに始めるため)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q3. 依存の無い単位どうし

A. 依存の無い単位(例: プルリクエストの残りと、CDK の synth の残り)は、どちらを先に作ってもよいものとして DAG に表す
B. すべてを一列の依存にする(前の単位が終わってから次)
C. 任せる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- 単位は6つ(Q1: 任された選択で A)。すべて実行ファイルに組み込まれる
  - U1 `u1-walking-skeleton`(service): 最小の一本
  - U2 `u2-pull-request`(library): プルリクエストの作成の残り
  - U3 `u3-cdk-synth`(library): CDK の synth と対応づけの残り
  - U4 `u4-cdk-typescript`(library): TypeScript への反映
  - U5 `u5-cdk-python`(library): Python への反映
  - U6 `u6-cdk-manual`(library): 反映できないものの一覧とその他の書き方
- 最小の一本(U1)は、TypeScript の CDK アプリ1つで `Tags.of(x).add` の文字列のキーの名前の変更を synth の対応を使って反映し、`--pr` でテスト専用のリポジトリにプルリクエストを作るまで。synth の認証情報の遮断と時間の上限、最低限の確かめ(gh、リモート、ブランチ)、CI の準備(3つの OS の Node.js、Python、CDK の CLI、テスト専用のリポジトリの secret と実際のプルリクエストのテスト)を含む(Q2: A)
- 依存の無い単位どうしは、どちらを先に作ってもよいものとして表す(Q3: A)。依存は U2→U1、U3→U1、U4→U3、U5→U3、U6→U4 と U5

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
