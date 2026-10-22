# インフラの設計 — 質問(U2 収集)

U2 のインフラ(CI と、検証用 AWS アカウントの確認用のリソース)を、どう用意するかを決めます。U1 のインフラの設計(`construction/u1-walking-skeleton/infrastructure-design/`)は、ここで変えない限りそのまま使います。

決まっていること(質問しません):

- CI に、Linux だけでレース検出(`-race`、`CGO_ENABLED=1`)を動かすジョブを足し、`ci-gate` の必須に入れる(NFR の設計 `reliability-design.md` §5)。
- CI の `cfn-lint` は、`test/sandbox/` のすべてのテンプレートを検査する。
- Resource Explorer の索引と既定のビューは、確認用の CloudFormation のテンプレートで作る(NFR の設計の Q4)。索引は `ap-northeast-1` の LOCAL(集約ではない)。集約の索引は偽の AWS だけで確かめる(集約の索引は、作り直しに24時間の待ちがあり、検証用アカウントで切り替えにくいため)。
- AWS Config の記録は有効にしない(費用がかかるため)。
- Resource Explorer の索引は、作ってから中身がそろうまで最大1時間ほどかかる(AWS のドキュメント)。

## Q1. 確認用のスタックの分け方

U1 のスタック `tag-refactor-u1-fixtures` は、人の指示で残してあります。その読み取り用ロール R2 は、例のポリシー `docs/iam/readonly-policy.json` と一致することをテストで確かめています。そのため U2 で例のポリシーに操作を足すと、U1 のテンプレートの R2 も直す必要があります。U2 の確認用のリソースを、どう置きますか?

A. U1 のテンプレートの R2 を例のポリシーに合わせて直し、U1 のスタックを更新する(読み取り用のプロファイル `tag-refactor-readonly` はそのまま使える)。U2 用には別のスタック `tag-refactor-u2-fixtures` を作り、Resource Explorer の索引と既定のビューだけを置く
B. U2 用のスタックに、読み取り用ロールをもう1つ(例のポリシーと同じ)持たせ、別のプロファイルで使う。U1 のテンプレートの R2 も、テストのために同じく直す
X. Other (please specify)

[Answer]: A

## Q2. 一度もタグが付いていないリソースの用意

CloudFormation で作ったリソースには、CloudFormation が `aws:cloudformation:stack-name` などのタグを自動で付けます。そのため、タグの取得の API で見つかってしまい、「一度もタグが付いていないリソース」の確認には使えません。どう用意しますか?

A. 検証のスクリプトが、タグを付けずに SQS のキューを1つ作り(名前に乱数を付ける)、確認のあとに必ず消す。Resource Explorer の結果に現れるまで、1分ごとに最大30分待つ
B. 人がマネジメントコンソールで作り、確認のあとに消す
C. 実機では確かめず、偽の AWS のテストだけにする
X. Other (please specify)

[Answer]: A

## Q3. U2 のチェックポイントの検証コマンド

U1 のチェックポイントでは `scripts/verify-u1.sh` を、人が承認した検証コマンドとして使いました。U2 ではどうしますか?

A. U1 と U2 の確認をまとめて行う `scripts/verify.sh` を作り、U2 のチェックポイントで、これを新しい検証コマンドとして人に承認してもらう(`verify-u1.sh` は `verify.sh` から呼ぶ)
B. `scripts/verify-u2.sh` を別に作り、U2 の確認だけを行う
X. Other (please specify)

[Answer]: A

## Consolidated Summary Confirmation

回答のまとめ:

- スタックの分け方(Q1): U1 のテンプレートの読み取り用ロール R2 を例のポリシーに合わせて直し、U1 のスタック `tag-refactor-u1-fixtures` を更新します。プロファイル `tag-refactor-readonly` はそのまま使います。U2 用には別のスタック `tag-refactor-u2-fixtures` を作り、Resource Explorer の索引(`ap-northeast-1`、LOCAL)と既定のビューだけを置きます。
- 一度もタグが付いていないリソース(Q2): 検証のスクリプトが、タグを付けずに SQS のキューを1つ作り(名前に乱数を付ける)、確認のあとに必ず消します。Resource Explorer の結果に現れるまで、1分ごとに最大30分待ちます。
- 検証コマンド(Q3): U1 と U2 の確認をまとめて行う `scripts/verify.sh` を作ります(`verify-u1.sh` を中から呼ぶ)。U2 のチェックポイントで、これを新しい検証コマンドとして承認してもらいます。

Does this all look correct before I generate the artifact?

- Looks correct
- Request changes

[Answer]: Looks correct
