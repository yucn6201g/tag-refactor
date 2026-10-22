# 非機能の設計 — 質問(U2 収集)

U2 の非機能要件(`construction/u2-scan/nfr-requirements/`)を、どう作るかを決めます。U1 の設計(`construction/u1-walking-skeleton/nfr-design/`)は、ここで変えない限りそのまま使います。

決まっていること(質問しません):

- 再試行は最大10回、待ち時間の上限20秒。1回の試行は30秒、再試行を含めた1回の呼び出しは5分(NFR5.4、NFR5.5)。
- タグの取得の1ページは100件。Config の問い合わせの1ページの上限は100件(AWS のドキュメント `API_SelectResourceConfig.html`)。
- Resource Explorer の一覧 `ListResources` の `MaxResults` は 1,000 未満にする(1,000 では `NextToken` が返らない)。必要な IAM の操作は `resource-explorer-2:Search`。
- AWS Config の問い合わせは、タグの中身を条件にできず、NULL の条件も書けない(AWS のドキュメント `querying-AWS-resources.html` の制限)。そのため Config の経路では、記録されたリソースを `tags` の項目付きですべて読み、ツールの側で利用者のタグ(`aws:` で始まらないキー)がないものを選ぶ。

## Q1. リージョンを同時に読む数

リージョンを同時に読む数(固定、利用者は変えられない)を、いくつにしますか?呼び出し回数の上限はリージョンごとにかかるため、同時に読む数を増やしても1つのリージョンの上限には当たりにくくなりません。1つのリージョンの中のページは順に読むので、全体の時間は件数の多いリージョンで決まります。仮の1回の呼び出しの時間を0.5秒とすると、1万件の見積もり(NFR1.8 の式)はどの案でも10分を大きく下回ります(A で約1分)。

A. 8(17リージョンを2〜3回に分けて読む。全体の時間と、端末と回線の負担の釣り合い)
B. 4(負担を小さく。見積もりは約2分)
C. 有効なすべてのリージョンを一度に読む
X. Other (please specify)

[Answer]: A

## Q2. Resource Explorer の一覧の1ページの件数

`ListResources` の `MaxResults` を、いくつにしますか?

A. 500(1,000 から十分に離し、ページの数も少なく保つ)
B. 100(タグの取得、Config とそろえる)
X. Other (please specify)

[Answer]: A

## Q3. AWS Config の経路の扱い

Config の問い合わせでは「利用者のタグがない」を条件に書けないため、Config の経路では記録されたリソースをすべて読むことになります(1万件で、1リージョンの全件ぶんのページが増える。見積もりは10分以内に収まる)。機能設計 BR3.1 の順(Config がすべての種類を記録していれば Config を先に使う)を、どうしますか?

A. BR3.1 の順のままにする。Config ではすべてを読み、ツールの側で選ぶ。Resource Explorer の索引がないアカウントでも探せる
B. Resource Explorer を先にする(`tag:none` で AWS の側で絞れるため、呼び出しが少ない)。Config は Resource Explorer が使えないときだけ使う。機能設計 BR3.1 を直す
X. Other (please specify)

[Answer]: A

## Q4. 検証用アカウントの Resource Explorer の準備

U2 のチェックポイントの前に、検証用アカウントに Resource Explorer の索引と既定のビューが要ります(NFR3.5。費用はかからない)。どう用意しますか?

A. 確認用の CloudFormation のテンプレート(`test/sandbox/`)に、索引と既定のビューを足す。中身は Infrastructure Design で決める
B. 人がマネジメントコンソールで作る(手順を書いて渡す)
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- 同時に読む数(Q1): リージョンを同時に8つまで読みます。固定の値で、利用者は変えられません。仮の1回の呼び出しの時間(0.5秒)での1万件の見積もりは約1分です。U2 のチェックポイントで実測して確かめます。
- Resource Explorer のページ(Q2): `ListResources` の `MaxResults` は500にします。
- Config の経路(Q3): 機能設計 BR3.1 の順のままにします。Config がすべての種類を記録していれば Config を先に使い、記録されたリソースを `tags` の項目付きですべて読んで、ツールの側で利用者のタグがないものを選びます。
- 検証用アカウントの準備(Q4): 確認用の CloudFormation のテンプレートに、Resource Explorer の索引と既定のビューを足します。中身は Infrastructure Design で決めます。

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
