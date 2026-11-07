## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-31T03:21:42Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR4.3 renameKey、BR4.4 | 変更案の renameKey には mergesIntoExisting があり(契約 C2。U4 の planner が別名の統合で普通に出す)、true のときは toKey に書かず fromKey を消すだけの意味です。BR4.3 はこれを使わず、renameKey を常に「fromKey を toKey に変える」とし、toKey がすでにあれば keyAlreadyExists にします。別名(env と Environment が両方ある)の統合は手で直す一覧に落ち、同じリソースで別名が2つあると2つ目は必ず keyAlreadyExists になります。逆に toKey が default_tags やスタックのタグ由来でソースにないだけの場合は、AWS では「fromKey を消すだけ」の変更を、ソースでは fromKey を toKey に変える書き換えにしてしまい、意味が変わります | BR4.3 と BR4.4 に mergesIntoExisting の場合分けを書く。true のときは fromKey のタグの値が変更前の値と同じことを確かめて消すだけにし、toKey がソースにあるかどうかで失敗にしない(ない場合の扱いも決める)。false のときだけ名前を変える。同じリソースの複数の renameKey が続く場合の判定も例で示す | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR3.2 | CloudFormation のテンプレートの選び方が、論理 ID だけを手がかりにしています。OrdersQueue、Bucket、Queue のような論理 ID は別々のスタックで普通に重なります。--dir にテンプレートが1つだけ該当するとスタック名もリソースの種類も確かめずにそれを選ぶため、別のスタックのリソースの変更を、たまたま同じ論理 ID のテンプレートに書き込みます。BR4.3 の値の一致は env の Prod のような一般的な値では歯止めになりません。ファイル名とスタック名の比べは複数あるときの決め手にしか使われません | 場所を決める条件に、変更案の resourceType(sqs:queue など)とテンプレートの Type(AWS::SQS::Queue など)の対応を加える。1つしか見つからない場合も、Type が合わないなら templateNotFound にする。スタック名がファイル名やテンプレートのメタデータと食い違うときの扱い(注意を出すか、手で直すにするか)を決める | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR2.3 | state の ARN がないときの id の比べ方(ARN の最後の / か : のあとと同じなら同じリソース)は、リソースの種類を見ません。同じ名前の別の種類(Lambda 関数 orders と IAM ロール orders、SQS と SNS など)に当たり、別のリソースを Terraform のリソースと決めます。理由に挙げた「S3 のバケットは arn がない」「SQS の id は URL」も、実際の AWS プロバイダーでは arn 属性があり(functional-spec.md の前提の記述も同じ)、理由として成り立ちません。--tf-state が別のアカウントやリージョンのものでも、id の末尾だけの比べ方では誤って当たります | id の比べ方をやめるか、変更案の resourceType と Terraform のリソースの種類(aws_sqs_queue など)の対応表を決めて種類が合うときだけ比べるようにする。ARN の完全一致を基本とし、ARN がない種類は管理されていないに数える方針でよいかを決め、前提の記述(functional-spec.md の Assumptions)も実際の属性に合わせて直す | Resolved (reviewer) |
| R-04 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/functional-spec.md > 反映の例(Terraform)、Assumptions & Open Questions(hclwrite)と rules.md > BR4.4 | 書き換えの方法が食い違っています。entities.md の Edit と BR4.4 は「テキストの範囲の差し替えで、変える行以外は一切変えない」としています。一方 functional-spec は Terraform に hclwrite を使い、= の桁を terraform fmt と同じ形にそろえ直し、差分の行が増えることを許しています。hclwrite は属性や式のトークンの書き換えには向きますが、オブジェクトの書き方の中の1つのキーや値だけを差し替える部品はなく、桁そろえはタグの書き場所全体に及びます。BR4.6 の「ほかのブロックが変わっていない」の確かめも、この桁の変化をどう扱うか書いてありません | Terraform の書き換えの方法を1つに決める。書式を保つという Q5 の目的に合わせるなら、hclsyntax で範囲を読み、Edit(バイト範囲の差し替え)で当てる形にし、桁そろえはしないと書く。hclwrite を使うなら、変わってよい範囲(タグの書き場所の中だけ)を BR4.4 と BR4.6 に明記し、例と矛盾しないようにする | Resolved (reviewer) |
| R-05 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR4.2、BR4.4 | YAML の扱いに危険な抜けがあります。(1) アンカー(&name)のついたスカラーやタグの並びは、別名(*)でないので literal になりますが、書き換えると別名で参照している別のリソースのタグまで変わります。(2) 引用符なしにしてよいかの判定を、Go の YAML(1.2 寄り)で行うのか、CloudFormation が使う YAML 1.1(yes、no、on、off が真偽値)で行うのかが決まっていません。(3) 縦棒や > のブロックスカラー、複数行の書き方、引用符の中のエスケープを新しい値に付ける規則がありません。(4) ソースの値と変更前の値の比べ方が、書いてある文字列と解析後の値のどちらか決まっていません(Value: yes や Value: 1.0 など)。(5) yaml の Node は開始の行と桁しかなく、スカラーの終わりと、桁が文字数でバイトでない点を扱う方法が示されていません | BR4.2 に、アンカーが付いている・アンカーを参照する別名がほかにある要素は literal にしない(手で直す)と加える。引用符の要否は YAML 1.1 と 1.2 の両方で別の型に読めるものを引用符で囲む、と決める。ブロックスカラーは notLiteral にする。比較は書いてある文字列で行うと決め、エスケープの規則を書く。位置の取り方(元のテキストから範囲を求める)を機能の仕様に書く | Resolved (reviewer) |
| R-06 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR4.4、BR4.5 | 要素の削除と追加が「その行」「同じ形とインデント」で書かれており、一行にまとめた書き方に対応していません。Terraform の tags = { env = "Prod" } や、CloudFormation の JSON を一行にしたもの、Tags: [{Key: a, Value: b}] の流れの書き方では、行を消すと括弧ごと消え、JSON ではカンマの足し引きもあります(BR4.6 の読み直しで取りやめにはなるが、よくある形が不具合扱いの unreadableFile になる)。また Auto Scaling グループの Tags のように Key と Value 以外の項目(PropagateAtLaunch)を持つ要素に「同じ形」で足すと、必須の項目が欠けたテンプレートになります。BR4.1 の cfnKeyValueList は Key と Value 以外の項目を持つ要素を禁じていません | 一行の書き方と JSON のカンマの扱いを、削除と追加それぞれについて規則に書く(扱わないなら unsupportedTagShape にする)。cfnKeyValueList は Key と Value だけの要素の並びに限り、ほかの項目がある要素を持つリソースは追加を noTagSite か unsupportedTagShape にする。unreadableFile は不具合の疑いだけに使い、対応しない書き方の理由は別にする | Resolved (reviewer) |
| R-07 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR3.3 | Terraform のブロックを --dir の下の全部の .tf から再帰的に探します。ルートモジュールは1つのフォルダーの .tf で、ローカルモジュール(modules/queue/ など)の中にも同じ種類と名前(this、main など、よくある名前)のブロックがあります。ルートのアドレス aws_sqs_queue.orders の探索でモジュールの中のブロックが候補に入り、本来1つに決まるものが ambiguous になります。モジュールの中にだけ同名があり、ルートの定義が別のフォルダーにある構成では、モジュールの中のブロックに誤って書く可能性があります | ルートモジュールの範囲(--dir の直下の .tf)を基本とするか、--dir 直下に .tf が無いとき・複数あるときの扱いを決める。モジュールのフォルダーの .tf を探す対象から外す方法を書く。再帰して探す必要があるなら、その理由と曖昧になる条件を規則に書く | Resolved (reviewer) |
| R-08 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR4.3 | ソースがすでに変更後の状態(利用者が手で直した、前回の --write のあと、同じ plan で2回目に実行した)のときの扱いがありません。renameKey は keyNotInSource、addRequired は keyAlreadyExists、changeValue は valueDiffers になり、すべて手で直す一覧に出て終了コード 2 になります。--write のあとの再実行や、一部を手で直したあとの再実行で、反映済みを成功として扱えず、結果の意味(0 はすべて反映した)が崩れます | 変更後のタグがすでにソースにある場合(rename なら toKey があり fromKey がない、changeValue なら toValue と同じ、addRequired なら同じ値で存在、removeKey ならキーがない)を alreadyReflected として扱い、手で直す一覧に出さず、数だけを示して終了コードに影響させないと決める。keyNotInSource と区別できる条件を書く | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR6.1、BR6.2 と functional-spec.md > 業務の場面(--tf-state なし) | --tf-state を渡さないと、Terraform のリソースもすべて管理されていないに数えられ、終了コード 0 で終わります。場面の表には「結果に --tf-state の案内が出る」とありますが、BR6.1 にその出力の規則がありません。反映したものが0件で管理されていないものだけのとき、利用者が AC7.1.1 の反映が済んだと誤解します | BR6.1 に、--tf-state が無く管理されていないリソースがあるときの案内を明記する。管理されていないものが変更案のリソースの大半で反映が0件のときは、警告を標準エラーに出すかを決める(終了コードの扱いも書く) | Resolved (reviewer) |
| R-10 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR5.3 | 「書く直前に、ファイルの中身が読んだときと同じかを確かめ、違えばそのファイルを書かずに止める(ほかのファイルも書かない)」とありますが、確かめをファイルごとに書く直前にするのか、全ファイルを先に確かめるのか決まっていません。ファイルごとなら、先に書いたファイルが残り、「ほかのファイルも書かない」と矛盾します。シンボリックリンクのファイルを一時ファイルと名前の変更で書くと、リンクが普通のファイルに置き換わる点も触れていません | 書く前に全ファイルの確かめを終えてから、書き出しに入ると手順を決める。書き出しの途中の失敗の扱いは今の記述(書いたものと書いていないものを示す)を保つ。シンボリックリンクのファイルは書き換えない(手で直す)か、リンク先に書くかを決める | Resolved (reviewer) |
| R-11 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/functional-design-questions.md > すでに決まっていること(既存の部品)と functional-spec.md > Assumptions & Open Questions | YAML の読み書きに go.yaml.in/yaml/v3 がすでに依存にあるとしていますが、go.mod の依存は github.com/goccy/go-yaml です(go.yaml.in/yaml/v3 は go.sum にあるだけで、直接の依存ではない)。範囲を差し替える方針は、どの YAML の部品がどの形で位置(バイトの位置、終わり)を返すかに左右されます | 前提を実際の依存に直す。位置を取る部品を決める(goccy/go-yaml の ast のトークンの位置を使うか、元のテキストから範囲を求めるか)。hclwrite(または hclsyntax)が新しい依存になることを、依存の追加として NFR の検討に引き継ぐと書く | Resolved (reviewer) |
| R-12 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/functional-spec.md > 契約への変更(C3)と entities.md > IacRequest、IacResult | 契約への変更の表は C3 に iac の標準出力の形を足すとしていますが、contract-summary.md の C3 にはその記載がなく(scan と plan の JSON だけ)、更新されていません。IacRequest に BR1.1 の --debug がなく、IacResult の exitCode に 1 があるのに、終了コード 1 では結果が作られません。BR4.3 は契約の項目名(renameKey と removeKey は fromKey、changeValue は fromKey、addRequired は toKey)でなく key と書いています | C3 の変更を contract-summary.md に記録するか、表から外す。IacRequest に debug を足すか、全体のオプションとして扱うと書く。IacResult の exitCode を、結果が作られる 0 と 2 のときの値にするか、1 の場合の扱いを書く。BR4.3 の項目名を契約に合わせる | Resolved (reviewer) |
| R-13 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR3.1、BR1.3 | 構文の誤りで解析できないテンプレートを、黙って候補から外します(--debug でだけ示す)。対象のテンプレートが壊れている場合、利用者には templateNotFound と見えて原因が分かりません。また terraform show -json は、リソースがない空の state では values を出さないため、BR1.3 の「values.root_module がある」の確かめで InvalidInput になり、案内が実態と合いません | 候補から外したファイルの数と名前を、手で直す理由の案内に含める(templateNotFound や blockNotFound のとき、解析できなかったファイルがあれば detail に出す)。values がない state は、リソースがない state として扱うか、案内を別にする | Resolved (reviewer) |
| R-14 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/entities.md > Edit、rules.md > BR4.3、BR4.4 | 同じリソースの変更を順に当てる(BR4.3)一方、Edit は元のファイルのバイト位置で持ちます。同じタグ要素への renameKey のあとの changeValue(仕様の例の env から Environment、さらに Prod から prod)や、同じ書き場所への追加・削除の続きで、Edit の範囲が重なる、または位置がずれる場合の合成の規則がありません | Edit を元のテキストの位置で持つなら、同じ要素への変更を1つの Edit にまとめる規則と、重なる範囲を持つ変更が来たときの扱い(まとめるか、手で直す)を決める。仕様の例のとおりに動くことを、その規則で説明する | Resolved (reviewer) |
| R-15 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR4.3 removeKey と renameKey(mergesIntoExisting = true) の post | 反映済みを入れた結果、removeKey と mergesIntoExisting が true の rename は「fromKey がソースにない」をすべて反映済みとして数えます。fromKey がスタックのタグや default_tags で付いていてソースに書かれていないだけの場合も、反映済みになり、手で直す一覧にも出ず終了コード 0 になります。IaC を次に適用するとそのタグはまた付くので、iac が防ぎたいタグの戻りが、成功として報告されます。keyNotInSource は changeValue と rename(false)にしか出なくなり、理由の表の「変える・消すキーがソースにない」とも合いません | 消す変更(removeKey、rename の true)で、キーがソースにないときは「すでに消えている」と「そもそもソースに無い」を区別できないことを規則に書く。そのリソースにタグの書き場所がない、または fromKey が beforeTags にあるのにソースの書き場所のどこにもない場合は、反映済みの数とは別に案内(スタックのタグや default_tags かもしれない)を結果に出す。案内を終了コードに影響させるかも決める | New |
| R-16 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR4.4 キーと値の書き方 | 引用符なしにしてよい形を、文字の種類(英字・数字・_・.・/・@・-)だけで決めています。@ や . や - で始まる値、- だけの値は、YAML の引用符なしのスカラーとして書けない、または別の意味になります。Terraform の " の文字列で、新しい値に ${ や %{ が入るときのエスケープも書いてありません(AWS のタグの文字には $ がないが、ルールファイル由来の値で防ぐ根拠が無い)。書けない形は BR4.6 の読み直しで unreadableFile になり、不具合の疑いとして案内されます | 引用符なしにできる条件に、先頭の文字の制限(@、`、-、.、?、: などで始まらない)を加える。Terraform のエスケープに ${ と %{ を加える | New |
| R-17 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR4.5 | 足す要素は最後の元の要素の書き方を写しますが、写す元が、値が !Ref などの式や、ブロックスカラー、複数行の要素、行末のコメント付きの場合でもよいのかが決まっていません(仕様の例は !Ref の要素から写している)。コメントまで写すと、足した要素に同じコメントが付きます。JSON では、いまの最後の要素にカンマを足す Edit と、足す要素にはカンマを付けない規則が必要ですが、「カンマも写す」だけでは、写した最後の要素にカンマがないため、そのままでは不正な JSON になります | 写す対象を、キーと値の構造とインデントと引用符の種類に限り、値のタグ、コメント、複数行の構造は写さないと書く。JSON の挿入では、前の最後の要素にカンマを足す Edit を作る、と書く | New |
| R-18 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u8-iac/functional-design/rules.md > BR1.3 | format_version があり values がなければ空の state として受け付けるため、terraform show -json に plan のファイルを渡した出力(format_version と planned_values、resource_changes がある)も空の state として通ります。--tf-state を渡したのにすべてのリソースが管理されていない扱いになり、原因が分かりません | values がなく planned_values か resource_changes がある JSON は、state の出力でないとして InvalidInput にする。空の state として扱うのは、それらがないときだけにする | New |

> R-01 Reviewer note: BR4.3 が mergesIntoExisting の true と false を分け、true は fromKey を消すだけ(toKey がソースにあるかは問わない)になった。複数の rename の例もある

> R-02 Reviewer note: BR3.2 が論理 ID に加えて Type と resourceType の対応で絞り、種類が違うものは templateNotFound(detail に名前)になった。同じ論理 ID と Type のテンプレートが別のスタックのものだった場合の残りの危険は、仕様の前提に書かれ、値の前の状態の確かめと差分のレビューで防ぐ扱いで受け入れられる

> R-03 Reviewer note: BR2.3 が values.arn の完全一致だけになり、id の比べ方をやめた。ARN のない種類は管理されていないに数える。前提の記述も実際の属性に直った

> R-04 Reviewer note: Terraform も hclsyntax の範囲による差し替えになり、桁はそろえ直さない。仕様の例の後の形(キーの差し替えだけで桁が残る)と BR4.4 が合っている

> R-05 Reviewer note: アンカー・別名・マージのキーは tagsNotLiteral、ブロックスカラーと複数行は notLiteral、値は書いてある文字列で比べ、引用符なしは YAML 1.1 と 1.2 の両方で文字列に読めるときだけ、位置は goccy/go-yaml のトークンから求め、範囲の文字を確かめる。細部の抜けは新しい指摘に書いた

> R-06 Reviewer note: 1行の書き方は差し替えだけ、削除と追加は unsupportedTagShape。JSON のカンマ、Key と Value 以外の項目を持つ並びへの追加の扱いも書かれた。追加時のカンマの細部は新しい指摘に書いた

> R-07 Reviewer note: Terraform は --dir の直下だけを探す。CloudFormation は再帰、シンボリックリンクは見ない

> R-08 Reviewer note: リソース全体と変更ごとの alreadyReflected が入り、手で直す一覧に出さず終了コードに影響しない。二度目の実行の例もある。キーがソースにないときの扱いの副作用は新しい指摘に書いた

> R-09 Reviewer note: BR6.1 に --tf-state の案内と反映0件の警告(標準エラー、終了コードは変えない)が入った

> R-10 Reviewer note: BR5.3 が全ファイルを先に確かめてから書く手順になり、シンボリックリンクは見ない

> R-11 Reviewer note: 前提が goccy/go-yaml に直り、hclsyntax を新しい依存として NFR に引き継ぐと書かれた

> R-12 Reviewer note: C3 は変えないと明記され、IacRequest に debug、IacResult に反映済みの数と、結果を作る終了コードの説明が入り、項目名も契約に合った

> R-13 Reviewer note: 外したファイルの数の警告と detail が入り、values のない空の state も受け付ける

> R-14 Reviewer note: メモリの上で当ててから元の並びと比べて Edit を作る方式になり、範囲が重ならない。例の説明も合っている

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| (なし。設計のレビュー) | 実行せず | このステージは検証の道具を持たない。entities.md、rules.md、functional-spec.md の間で、ManualItem.reason の13種類、outcome の5種類(upToDate を含む)、IacResult の項目、終了コードが一致していることを確かめた。Mermaid の stateDiagram-v2 と erDiagram に構文の問題はない。traceability.json が指す BR は rules.md にすべて存在する。C2 と C5 の記述は contract-summary.md と合っている |

### Summary

1回目の Major 8件と Minor 6件は、すべて規則と仕様に反映されました(mergesIntoExisting の場合分け、Type の確かめ、ARN の完全一致、範囲の差し替え、アンカー、反映済み、ルートモジュールだけ、など)。残る Major は1件(消す変更が「ソースに無い」場合に反映済みと見分けられない点)で、規則を1つ足せば直せる範囲のため、判定は READY とします。Minor 3件は Code Generation の前に直すのが望ましいです。
