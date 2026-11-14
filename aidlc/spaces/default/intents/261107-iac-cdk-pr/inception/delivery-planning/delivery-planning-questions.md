# 作業の計画の質問: iac への CDK の反映とプルリクエストの作成

作業の単位(U1〜U6、`inception/units-generation/unit-of-work.md`)と、その依存(`unit-of-work-dependency.md`)は決まっています。この段階では、Construction で単位をどの順番で作るかを決めます。

ここでは、作る1回分のまとまりを Bolt と呼びます。Bolt は、1つ以上の単位を設計からコードまで通し、動くものができたところで終わる、1回の作業です。チームの進め方では、1つの Bolt が `main` の1つのコミット(squash マージ)になります。

最初の単位 U1 は、最小の一本(端から端まで細く通す最初の一本、walking skeleton)です。これはチームの進め方で決まっているので、ここでは聞きません。

## Q1. U1 のあとの順番

依存の図では、U2(プルリクエストの残り)と U3(synth の残り)はどちらも U1 だけに依存しています。U4 と U5 は U3 に、U6 は U4 と U5 に依存しています。

A. U1 → U2 → U3 → U4 → U5 → U6。範囲の定義(scope-definition)で「プルリクエストの機能を先に」と決めたとおり、プルリクエストの残りを先に仕上げ、そのあと CDK を広げる
B. U1 → U3 → U4 → U5 → U6 → U2。難しい CDK の部分を先に進め、プルリクエストの残りは最後にする
C. 任せる
X. Other (please specify)

[Answer]: A

## Q2. 順番を点数で決めるか

A. 点数の式(WSJF: 価値、急ぎ、危険の減り具合を足し、大きさで割る)は使わない。開発は1人で単位は6つなので、依存の図と理由を文で書く
B. WSJF で点数を付け、その順に並べる
C. 任せる
X. Other (please specify)

[Answer]: A

## Q3. 1つの Bolt の大きさとプルリクエストの分け方

A. 1つの単位を1つの Bolt にする。Bolt ごとにブランチを作り、プルリクエストで `main` に squash マージする(全部で6つ)。AI-DLC の記録は、それぞれのプルリクエストに入れる
B. 6つの単位を1つのブランチで作り、最後に1つのプルリクエストにまとめる
C. 任せる
X. Other (please specify)

[Answer]: A

## Q4. 同時に作るか

U2 と U3、U4 と U5 は、互いに依存しません。今の設定は、1つずつ順に作る形(unit-major、serial)です。

A. 1つずつ順に作る(今の設定のまま)
B. 依存しない組は同時に作る
C. 任せる
X. Other (please specify)

[Answer]: A

## Q5. 外のことで止まりうるもの

次の3つは、人の手が要るか、人の許可が要ります。

1. テスト専用の GitHub のリポジトリと、そこにだけ書ける fine-grained トークン(CI の secret に置く)
2. 検証用の AWS アカウント(663798383745、ap-northeast-1)に、最小の CDK アプリを置くこと(AWS の変更なので、変更を見せてから人の明示の許可を得る)
3. AWS の SSO のログイン(切れたら人が `aws sso login` をする)

A. この3つで足りる。1 と 2 は U1 の Code Generation の前までにそろえ、そろわなければ U1 の検証のところで止めて人に頼む
B. ほかにもある、または準備の時期を変える(Other に書く)
C. 任せる
X. Other (please specify)

[Answer]: A

## Q6. いちばん心配なこと(早めに確かめるもの)

A. `cdk synth` の認証情報の遮断と、3つの OS の CI での synth(U1 で早めに確かめる)
B. TypeScript と Python のソースを正しく書き換えられるか(U4、U5)
C. CI から実際の GitHub にプルリクエストを作るテストの安定さ(U1)
D. 任せる
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

- 作る順番は U1 → U2 → U3 → U4 → U5 → U6。最小の一本(U1)のあと、プルリクエストの残りを先に仕上げ、そのあと CDK を広げる(Q1: A)
- 点数の式(WSJF)は使わず、依存の図と理由を文で書く(Q2: A)
- 1つの単位を1つの Bolt にし、Bolt ごとにブランチとプルリクエストを作って `main` に squash マージする(全部で6つ)。AI-DLC の記録もそれぞれのプルリクエストに入れる(Q3: A)
- 1つずつ順に作る(unit-major、serial のまま)(Q4: A)
- 外の依存は、テスト専用のリポジトリとトークン、検証用のアカウントへの最小の CDK アプリの配置(人の明示の許可を得てから)、SSO のログインの3つ。前の2つは U1 の Code Generation の前までにそろえ、そろわなければ U1 の検証で止めて人に頼む(Q5: A)
- いちばんの心配は、`cdk synth` の認証情報の遮断と、3つの OS の CI での synth。U1 で早めに確かめる(Q6: A)

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
