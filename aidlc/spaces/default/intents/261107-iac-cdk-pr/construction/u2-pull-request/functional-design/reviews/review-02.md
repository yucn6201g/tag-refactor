## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-09T07:54:32Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/rules.md > BR4.3 and functional-spec.md > W1-10 | 続きの命令は push などの失敗でも gh pr create --body-file を含み、本文のファイルの場所も示す設計だが、既存の Publish は本文のファイルを create の段階の中で初めて書く(internal/pullrequest/pullrequest.go の create と writeBody)。branch、commit、push で失敗すると、続きの命令が指すファイルが存在しない。契約 C1 は create の前に書くと定めている | BR4.3 に、本文のファイルをブランチ作成の前(失敗時に命令を作る前)に書くことを明記する。書けなかったときの扱い(書き換えの前か後か、終了コード)も決める。U1 の実装を変える作業として functional-spec の「U1 から変えるところ」の表に加える | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/rules.md > BR3.2 and entities.md > PullRequestBody | --pr-body-file の中身は伏せ字を通さないと決めているが、既存の Publish は本文の全体を redact.String に通してから渡す(pullrequest.go の Publish)。このままだと利用者の本文が書き換わり、決まりと実装が食い違う。逆に通さないと決めた以上、利用者の本文に入ったトークンはそのまま公開のプルリクエストに載るので、その責任の所在も書かれていない | Publish 側の伏せ字を外し、組み立てる側(IacDisplay)だけで行うと決めて、変更箇所と受け入れテスト(利用者の本文が変わらないこと、組み立てた部分が伏せ字になること)を書く。利用者の本文にトークンの形があったときの扱い(そのまま載せる、警告する、止める)を明示する | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/rules.md > BR3.3 and BR1.1 | --pr-body-file は 1 MiB まで許すが、本文は 65,536 文字までで、超えると書き出しのあとで止まる(終了コード 1、ファイルは書き換え済みのまま)。利用者の本文だけで上限を超えることは書き換えの前に分かるのに、書き換えの前の確かめに入っていない。「どのファイルも変えずに止まる」という入力の誤りの方針とも合わない | BR1.1 に、利用者の本文だけで 65,536 文字(単位は文字。バイトでないことも明記)を超えるときの入力の誤りを足す。さらに、変更したファイルの一覧のための余白を決めるか、書き出しのあとで止まる場合の案内(書き換え済みであること)を BR3.3 に書く | Resolved (reviewer) |
| R-04 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/traceability.json > coverage AC1.1.5 | AC1.1.5(フックと署名が利用者の設定どおりに動き、コミットに iac が書き換えたファイルだけが入る)の対応先が BR2.3 と BR5.1 になっているが、どちらもコミットの中身やフックには触れない。U2 の決まりにはこの受け入れ条件を検証する規則が無く、U1 から変えないと書いただけになっている | U1 の該当の決まり(commit --only、フックと署名、ステージ済みのほかのファイルが残ること)を U2 の BR として参照し、受け入れテストの条件を書く。対応表の対象を実際の規則に直す | Resolved (reviewer) |
| R-05 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/rules.md > BR4.1 | 書き出すものが無く手で直す項目も無いときは終了コード 0 とするが、契約 C7 の exit_code_rule は、synth できなかった(failed か skipped の)CDK アプリがあれば 2 を返すと定める。BR4.2 だけがこれに触れ、BR4.1 は触れていない。「反映するものはありません」と 0 を返すと、失敗したアプリが利用者に伝わらない | BR4.1 の条件に、失敗か外した CDK アプリがある場合の終了コード 2 と表示を足す。本文の手で直す一覧は、アプリ単位の失敗が一覧の行になるか(ManualLine の resourceName のアプリのフォルダー)も合わせて決める | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/entities.md > ManualLine.file and functional-spec.md > 3 | 手で直す一覧の file は --dir からの相対だが、変更したファイルの一覧は既存の実装ではリポジトリの根からの相対(Prefix を付ける)で、同じ本文の中で基準が違う。また file が空のとき、または line が無いときの行の形が決まっていない | 2つの一覧の基準をそろえる(リポジトリの根からの相対を勧める)。file 空と line 無しの行の形を決める | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/rules.md > BR4.3 | 続きの命令の中身が決まりきっていない。commit の命令に付けるファイル一覧とリポジトリの根(git -C か、動かすフォルダー)、--literal-pathspecs の有無、gh が成功したのに URL を読めなかったときの扱いが書かれていない。開発者が推測することになる | 各段階の命令の例をリポジトリの根からの相対と引用符つきで書く。URL が読めなかった場合は、成功扱いにして本文のファイルを残さないか、失敗扱いにするかを決める | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/rules.md > BR1.1 | --pr-title は伏せ字を通すため、利用者が書いた 256 文字のタイトルが実際には変わりうるが、その旨と、伏せ字でタイトルが空になる場合の扱いが書かれていない | 伏せ字のあとの検査(空でない、256 文字まで)をするか、しないと決めて書く | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/entities.md > ManualLine の YAML | line_format の複数行ブロックが、続く resourceName、key、reasonText の3つの属性の行を飲み込んでおり、属性として読めない(attributes の並びも途切れている)。 | line_format を attributes の外か最後に移し、3つの属性を attributes の並びに戻す | New |
| R-10 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/rules.md > BR4.3 | 本文のファイルを Publish の始めに書くとしたが、書き込みに失敗したときの扱い(止めるか、終了コード、続きの命令に本文のファイルを含めない扱い)が決まっていない。書き換えはすでに済んでいる。 | 本文のファイルが書けないときは、作成に進まず終了コード 1 で、書き出したファイルは残ること、手で作る方法を示すことを BR4.3 に書く | New |
| R-11 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/rules.md > BR1.1 と BR3.3 | 60,000 文字と 65,536 文字の数え方(コードポイントか、バイトか)が書かれていない。GitHub の上限は文字単位なので、実装者が推測することになる。 | 単位を、Unicode のコードポイントなどと明記する | New |
| R-12 | Minor | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/functional-design/functional-spec.md > 2 の W1-3 と W1-4 | W1-3 が BR2.3(gitCheck)を書き換えの前の確かめに含めるが、W1-4 は書き換えを作ったあとの gitCheck としている。どのファイルを調べるかは書き換え案が決まらないと分からないので、順序の説明が食い違う。 | W1-3 から BR2.3 を外し、W1-4 の gitCheck が書き出しの前の最後の確かめであることを明記する | New |

> R-01 Reviewer note: BR4.3 が本文のファイルを Publish の始め(ブランチの前)に書くと明記し、W1-8 も合わせた。書けなかったときの扱いは未記載のため、新しい指摘(Minor)に分けた

> R-02 Reviewer note: BR3.2 と entities の PullRequestBody が、伏せ字は組み立て側だけで行い、Publish は本文全体に伏せ字を通さない(redact.String(in.Body) をやめる)と決めた。利用者の本文の責任は README に書くと明示した

> R-03 Reviewer note: BR1.1 に 60,000 文字の上限を足して書き換えの前に確かめ、BR3.3 に書き出し後に止まる場合の案内と終了コード 1 を書いた。文字の数え方は新しい指摘(Minor)に分けた

> R-04 Reviewer note: BR2.4 がステージ、commit --only、フックと署名、ほかのステージが残ることを定め、traceability.json の AC1.1.5 の対応先が実際の規則を指している

> R-05 Reviewer note: BR4.1 が失敗か外した CDK アプリがあるときの終了コード 2 を定め、ManualLine の resourceName にアプリの行の形も書いた。W1-6 と端の場合の表も合っている

> R-06 Reviewer note: ManualLine の file をリポジトリの根からの相対にそろえ、file 空の行の形を決めた。line 無しは file だけの行で扱われている

> R-07 Reviewer note: BR4.3 に段階ごとの命令の形(git -C、相対パス、引用符、シェルごとの引用)と、URL が読めないときの扱いを書いた

> R-08 Reviewer note: BR1.1 が、タイトルを伏せ字に通したあとで空でないこと、256 文字までを確かめ、変わったときは1行で示すと決めた

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| none declared | not applicable | このステージに検証ツールは宣言されていない。手で、BR と対応表の参照と U1 の pullrequest.go との整合を確かめた |

### Summary

前回の Major 5 件と Minor 3 件はすべて直っており、Critical と Major は残っていない。残りは entities.md の YAML の崩れ、本文のファイルが書けないときの扱い、文字の数え方、W1-3 と W1-4 の順序の説明の4件の Minor で、実装者が迷う程度のものにとどまる。
