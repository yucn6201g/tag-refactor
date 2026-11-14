## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-product-lead-agent
**Date:** 2026-11-08T03:48:15Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/refined-mockups/mockups.md > 冒頭の説明、画面2、画面3、画面7 | 冒頭で「今の `iac` の実際の出力の形に合わせた」としているが、今の `iac` が出す行が画面から落ちている。たとえば「already reflected: N」(`internal/display/iac.go` の `iac.already`)、「No file was changed. Add --write to change the files.」、「Not managed by IaC: …」、ファイルごとの「can be reflected / reflected」の行である。AC3.1.2 と AC3.2.2 は「反映済みに数えられる」ことを求めているが、どの画面にも反映済みの件数が出ない。ラフの画面2にあった `Summary:` の行も、置き換わった理由が書かれていない。実装者とテストを書く人は、どの行が残りどの行が消えるのか分からない。 | 画面2、3、7 に、今の `iac` が出す行のうち残すもの(反映済みの件数、書き換えていない旨の案内など)を載せる。消す行は、消す理由を書く。「今の出力と同じ行は省略している」という場合は、その旨と省略した行の一覧を明記する。 | New |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/refined-mockups/mockups.md > 画面3 と 画面5、interaction-spec.md > 書き換えの前の確かめ | 書き換えの前の確かめ(`gh`、リモート、ブランチ)と `cdk synth` のどちらを先に行うかが、画面で食い違っている。画面3は synth の後に「Checked:」を出すが、画面5は synth の行がないまま `gh` が無いと止まる。確かめを後にすると、`gh` が無いだけの利用者が、アプリごとに最大10分の synth を待ったあとで失敗に気づく。FR1.5 の「書き換える前」は両方の順で満たされるため、要件では決まらない。interaction-spec の確かめの順番にも synth の位置がない。 | 確かめと synth の順を決め(ネットワークを使わない確かめは synth の前に行うのが望ましい)、画面3、画面5、interaction-spec を同じ順にそろえる。順によって「Checked:」の位置と、画面5で synth の行が出るかどうかが変わるので、両方に反映する。 | New |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/refined-mockups/mockups.md > 1. 新しいオプション、interaction-spec.md > 確かめる順番 | 新しいオプションの誤った使い方の扱いが、一部しか書かれていない。`--pr` なしの `--pr-*` だけが誤りとされている。次の場合の表示と終了コードが決まっていない。`--cdk-app` が存在しないフォルダーや `cdk.json` のないフォルダーを指す、`--synth-timeout` が読めない値や 0 以下、`--pr-body-file` が存在しない・読めない・巨大、`--pr-branch` が git のブランチ名として使えない、`--pr-title` が空、`--cdk-app` を絞ったことで対象のアプリが0になる。ファイルが読めないことに気づくのが書き換えの後になると、FR1.5 の「何も変えずに止まる」を破る。 | 各入力の誤りについて、確かめる順番に項目を足し(入力の確かめは書き換えの前)、1文目の原因の例と終了コード 1 を、画面5の表に足す。 | New |
| R-04 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/refined-mockups/mockups.md > 画面3、画面6 | 実行後に利用者の手元のブランチがどこにあるかが、どの画面にも示されていない。interaction-spec の続きの案内は `git switch -c <名前>` から始まるため、元のブランチから新しいブランチへ切り替わるように読める。成功のあとに元のブランチへ戻すのか、新しいブランチに残すのかによって、次の作業と、途中で失敗したときの「続き」の命令が変わる。 | 成功時と失敗時の、実行後に手元で出ているブランチを決め、画面3と画面6のどちらかに1行で示す(戻さない場合は、そのことを明記する)。 | New |
| R-05 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/refined-mockups/mockups.md > 画面4、画面8 | プルリクエストの本文について2点が足りない。(1) 本文には ARN を載せずに論理 ID(CDK はコンストラクトの名前)で示すとしているが、synth に失敗したアプリのリソースには論理 ID もコンストラクトの名前もない(画面8は ARN で示している)。本文でそのリソースを何と呼ぶかが決まっていない。(2) AC1.1.6 は日本語の本文を求めているが、`--lang ja` の本文の例がない(端末の表示の例しかない)。 | synth に失敗したアプリのリソースの、本文での表し方(アプリのフォルダーとリソースの種類・名前など、ARN とアカウント ID を避ける形)を決める。日本語のタイトルと本文の例を1つ足す。 | New |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/refined-mockups/mockups.md > 画面2 と 画面3 | 画面2と画面3は同じ `plan.json` と `--dir infra` の例だが、結果が合わない。画面2は手で直すものが2件で「2 files」、画面3は1件で「4 files」である。差分の画面にも Terraform と CloudFormation のファイルが出るはずだが、画面2の差分は CDK の2つだけである。このままだとテストの期待値が食い違う。 | 画面2と画面3の件数を、同じ変更案から導かれる値にそろえる。別の変更案の例にするなら、そう書く。 | New |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/refined-mockups/mockups.md > 画面8、accessibility-checklist.md > エラーの分かりやすさ | 確認表は、エラーを「`error:` で始め、原因、対処、何が変わったか」の形にするとしている(画面5、6、8に適用)。しかし画面8の synth の失敗の行は `error:` で始まらず、何が変わったか(変わっていないか)も書かれていない。画面6の「Nothing was undone.」も、同じ形になっているかどうかが確かめにくい。 | 画面8の行を確認表の形に合わせるか、確認表の適用範囲から画面8の一部の行を外すと明記する。 | New |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/refined-mockups/mockups.md > 画面3 の最後の注、Assumptions & Open Questions | `--pr` のときに差分を端末に出さず、手で直す一覧も件数だけにする点は仮定のままで、利用者には確かめていない。書き換えの前に差分を見たい利用者は、`--pr` の前に `--pr` なしで実行することになる。また、終了コード 2 で件数しか出ない場合、中身を見るにはプルリクエストを開く必要がある。画面6の失敗でプルリクエストが作られなかったときは、手で直す一覧をどこで見られるかも示されていない。 | この振る舞いを利用者に確かめるか、決めた理由を書く。プルリクエストが作られなかったときは、手で直す一覧を端末に出すことを画面6に足す。 | New |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/inception/refined-mockups/mockups.md > 画面6 | `gh pr create` の失敗の例が「HTTP 422 (a pull request already exists for this branch?)」となっているが、ブランチは直前の確かめで新規と確かめ済みなので、この推測は利用者を迷わせる。続きの命令 `gh pr create --base … --head …` にはタイトルと本文もなく、`gh` が対話の問いを出す。 | 失敗の理由は `gh` の出力をそのまま示す形にして推測の文を外す。続きの命令に、タイトルと本文(`--pr-title` と `--pr-body-file` か既定の値)を含めるか、対話になる旨を書く。 | New |

### Summary

CDK と `--pr` の表示は、要件とストーリーの大半を画面に落とせており、成功、変更なし、手で直すものだけ、途中の失敗、一部のアプリの失敗の各状態が揃っている点は良い。ただし、今の `iac` の出力との差、確かめと synth の順、新しいオプションの誤用の扱いの3点が決まらないままでは、実装とテストで質問が戻るため NOT-READY とする(助言として。人の判断で進めてよい)。
