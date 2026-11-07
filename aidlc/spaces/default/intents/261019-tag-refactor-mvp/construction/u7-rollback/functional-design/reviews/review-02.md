## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-30T06:43:05Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u7-rollback/functional-design/functional-spec.md > 流れ W1 の手順 3(記録を読む)と手順 6(印を取る)、および rules.md の BR1.2・BR2.1・BR6.1 | 記録を読むのが印を取る前(手順 3)で、印を取ったあとに読み直す規則がない。印を取るまでの間に、動いている apply や別の rollback が記録を変えうる。(1) 読んだ記録が古いまま判定すると、書き込み直前の読み直しで飛ばされた項目(U6 の規則 4.7。`pending` として保存された時点の `touchedKeys` が残る)を `pending` と見て、`--force` で、apply が書いていないリソースの他人の変更を `beforeTags` で上書きしうる。(2) 古い記録に `rollbacks` を足して保存すると、同時に終わった別の rollback の結果や、apply が更新した `entries`・`completed` を消す(BR6.1 の「apply の entries と completed は変えない」を破る)。(3) apply が `Begin` 直後で `RecordBefore` 前なら「取り消すものはない」(BR1.4、終了コード 0)と誤って示して終える。NFR7 が守るはずの「同時実行で記録が壊れない」が、この順番で破れる | 印の取得(Lock には accountId・記録の場所・rollbackId だけが要り、記録の中身は要らない)を、記録を読むより前に移す。または、印を取った直後に記録を読み直し、判定・保存のもとにはその版だけを使う。`RecordRollback` は保存のたびに最新の記録を読み直して `rollbacks` だけを足す(読み込み、足す、保存を印の中で行う)と書く。「記録がない・アカウントが違うといった誤りを印なしで示す」ための順番の説明(W1 の末尾)も、この変更に合わせて改める | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/contract-design/contract-summary.md > C10 Journal の `RecordRollback(LockHandle, runId, rollbackId, []result)` と、construction/u7-rollback/functional-design/rules.md の BR5.4・BR6.1・BR6.2、entities.md の RollbackRun | U7 が C10 を更新したと述べているが、`RecordRollback` の引数には `completed` と `startedAt` を運ぶ場所がなく、「最初の書き込みの前に completed: false の RollbackRun を足す」「途中で結果を保存する」「最後に completed: true にする」(BR5.4・BR6.1・BR6.2)を、この署名では表せない。同じ rollbackId への2回目以降の保存が、追加か置き換え(rollbackId を鍵にした upsert)かも書かれていない。BR4.5 の「すべて alreadyRestored で completed: true」の1回だけの保存も、どの呼び出しになるか決まっていない。実装者が署名を推測して決めることになる | C10 に、rollback の記録用の呼び出しを正式に定義する。例: 開始(rollbackId と startedAt、completed: false で追加)、結果の追加(rollbackId を鍵に、判定が済んだ項目の結果を置き換えまたは追加)、完了(completed: true)。保存のたびに最新の記録を読み直すこと、`entries`・`completed`・知らない項目を変えないことを、契約の規則として書く | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u7-rollback/functional-design/rules.md > BR7.4 と BR7.1、functional-spec.md > 契約への変更(C5 の apply の行) | BR7.4 は「U6 の規則 6.3 を改める」とし、取り消しのコマンドを `--record-to` がある場合は `rollback --run <runId> --record-from <値>`、ない場合は `rollback --run <runId>` と定めている。しかし実装済みの U6(`internal/cli/apply.go` の `rollbackCommand`、`apply_cli_test.go`)はすでに `--record-from` を含めており、`--profile` を指定したときは `--profile <値>` も付けている(U6 の規則 6.3)。BR7.4 を字面どおりに実装すると `--profile` が消えて既存のテストが落ち、別のアカウントのプロファイルを使うと、案内どおりに実行しても「記録がない」で止まる。BR7.1 の「上書きして戻すコマンド」と「もう一度 rollback するコマンド」にも `--profile` の記述がない。BR7.4 は U7 の作業として存在しない変更を主張している | BR7.4 を、U6 の規則 6.3 がすでに満たしていること(U7 では変更しない)として書き直すか、`--profile` を含めた形に直す。BR7.1 の2つのコマンドにも、`--record-from` と同様に `--profile` を指定した場合は含めると書く。C5 の行の「apply が示す…を含める」も、既存の動作の確認として扱い、U7 の変更に数えない。回帰のテストが U6 側にすでにあることを traceability に書く | Resolved (reviewer) |
| R-04 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u7-rollback/functional-design/rules.md > BR5.3 と functional-spec.md > Assumptions & Open Questions(読み戻しの項目) | 書いた直後の読み戻しで、値が違えばただちに `failed(notRestored)`(終了コード 1)にする。タグの読み取り API は反映が遅れることがあり(U6 は同じ状況を「確かめられなかった」として許している)、健全な書き戻しが誤って失敗になり、NFR4 を確かめる検証コマンドが不安定になるおそれがある。待ちの扱いは NFR の設計に委ねられているが、機能の規則としては再読み取りの有無が決まっていない | 読み戻しで違った場合に、上限つきで読み直してから `notRestored` にする(回数と間隔は NFR の設計で決める)と規則に書く。または、違った場合は U6 と同じ「確かめられなかった」にする代わりに、同じ `--run` でもう一度 rollback すれば `alreadyRestored` になることを案内する、と決める | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u7-rollback/functional-design/rules.md > BR1.2・BR1.3・BR3.4 と entities.md > RollbackTarget | 読んだ記録の入力の確かめが足りない。(1) 記録の `runId` と `--run` が同じかを確かめる規則がない(`--record-from` で手で動かした別の記録を読みうる)。(2) 記録の `touchedKeys` に `aws:` のキーが入っていた場合(手で編集された記録)の防御がない。TagWriter は InvalidInput を返すが、BR5.2 の分類のどこに入るかが決まっていない。(3) 記録を書き直すとき、知らない項目を保つことが決まっていない(C4 は「読む側は知らない項目を無視する」とだけ定めているため、型に載せて書き戻すと将来の項目が消える) | BR1.3 に runId の一致を足す。BR1.4 か BR3.4 に、`touchedKeys` から `aws:` のキー(大文字小文字を区別しない)を除くか、そのような記録を InvalidInput として止める規則を足す。BR6.1 に、知らない項目を保って書き戻すことを足す | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u7-rollback/functional-design/rules.md > BR5.2・BR5.5 と entities.md > RollbackTarget.verdict | 「一時的な失敗が続いたとき新しい書き込みを止める」の回数が書かれていない(U6 は10件。U6 の規則 4.2)。また、今のタグを読めず `failed(readFailed)` になった項目は判定ができないのに、`verdict` が必須で、取りうる値に「判定なし」がない | BR5.2 に回数を書くか、「U6 と同じ10件」と明記する。`verdict` を任意にするか、`unknown` を値に足す | Resolved (reviewer) |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u7-rollback/functional-design/functional-spec.md > 流れ W1 の手順 5 と rules.md > BR4.3・BR4.5・BR1.4 | 端末の確認(手順 5)が判定(手順 7)より前にあるため、パイプから実行して `--yes` なしだと、すべて `alreadyRestored` で確認が要らない場合でも終了コード 1 で止まる(BR4.5 の「確認を求めずに終える」と矛盾)。また、BR1.4 は「記録も変えない」、BR4.5 は「結果を記録に足す」とあり、判定する項目がない場合とすべて戻っている場合で記録を変えるかどうかが分かれる理由が書かれていない | 端末の確認を、確認が必要と分かった時点(手順 9 の直前)に移すか、現状の順番を意図として明記する。記録を足す場合と足さない場合の違いを1か所の表にまとめる | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u7-rollback/functional-design/traceability.json > coverage の AC5.1.4・AC5.1.1・AC5.1.6 | 対応づけが実際の規則と合わない。AC5.1.4(`--force` で上書きして戻す)の対象に BR3.5(50を超える項目)が入り、付けるタグと消すキーを求める BR3.4 が入っていない。AC5.1.1 に BR7.4 が入っているが、上の指摘のとおり BR7.4 は rollback の動作ではない。ツールの検査(sensor-traceability)は通っている | AC5.1.4 の対象を BR3.3・BR3.4・BR4.4 にし、AC5.1.1 から BR7.4 を外す(BR7.4 の扱いを直したあとで再確認する) | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u7-rollback/functional-design/functional-spec.md > Assumptions & Open Questions と rules.md > BR3.1・BR1.4 | 確かめていない前提が2つある。(1) 「答えに出ないリソースは gone」(BR3.1)は、apply が最後のタグまで消した(removeCandidates)リソースを、読み取り API が返すかに依存する。返さなければ、タグを戻すべきリソースが `resourceNotFound` で飛ばされ、NFR4 を満たせない(飛ばしとして終了コード 2 で見えるので安全側ではある)。(2) rollback A のあとに同じ変更を再び apply(B)してから rollback A をもう一度実行すると、今のタグが `plannedAfterTags` と同じなので `restore` と判定され、B の書き換えを `--force` なしで戻す。記録の `rollbacks` に完了した結果があるのに、判定に使っていない | (1) を Assumptions に書き、検証用アカウントでの結合テストの確認項目に入れる。(2) は、仕様として許す(同じ値は区別できないと明記する)か、完了した rollbackId があるときに警告を出す、のどちらかを決めて書く | Resolved (reviewer) |

> R-01 Reviewer note: 印を取る手順が記録を読む手順の前(W1 手順 3、4。BR2.1、BR1.2)になり、判定と保存は印を持っている間に読んだ記録だけを使う。C10 の規則にも「記録を読む前に印を取る」「保存のたびに最新の記録を読み直す」が入った。古い記録での判定、`rollbacks` の上書き、「取り消すものはない」の誤りのいずれも、この順番で起きなくなった

> R-02 Reviewer note: C10 が `BeginRollback`(rollbackId と startedAt、completed false で追加)、`RecordRollbackResults`(ARN を鍵に足すか置き換える)、`CompleteRollback` の3つに分かれ、BR5.4・BR6.1・BR6.2・BR4.5 がこの3つの呼び出しで書かれている。自分の rollbackId 以外、`entries`・`completed`・知らない項目を残す規則も契約と BR6.1 の両方にある

> R-03 Reviewer note: BR7.4 が「U6 の規則 6.3 で実装済み(`--record-from` と `--profile`)、U7 では変えない」に直り、実装済みの `rollbackCommand` と一致した。BR7.1 の2つのコマンドにも `--record-from` と `--profile` が入り、C5 の行も既存の動作の確認になった

> R-04 Reviewer note: BR5.3 で、読み戻しが違ったときは上限つきで読み直してから `notRestored` にし、もう一度の rollback を案内する。回数と間隔は NFR の設計に残してあり、機能の規則としては十分

> R-05 Reviewer note: BR1.3 に、記録の `runId` と `--run` の一致、`touchedKeys` の `aws:` キーの検出(InvalidInput)が入り、BR6.1 に知らない項目の保持が入った

> R-06 Reviewer note: BR5.2 に「10回続いたとき(U6 の規則 4.2 と同じ)」が入り、`verdict` に `unknown` が入った(BR3.1、entities.md)

> R-07 Reviewer note: 端末の確かめ(BR4.3)が判定と BR4.5 のあとの手順 8 に移り、戻すものがなければパイプからでも止まらない。記録を変える場合と変えない場合の表が spec に入った

> R-08 Reviewer note: traceability.json の AC5.1.4 が BR3.3・BR3.4・BR4.4 になり、AC5.1.1 から BR7.4 が外れた(BR7.4 は AC5.1.6 の対応先にある。U6 の動作の確認として妥当)

> R-09 Reviewer note: (1) BR3.1 で、`plannedAfterTags` が `aws:` を除いて空の項目は、答えに出なくても `gone` とせず空のタグとして判定する(書き込みが NotFound なら BR5.2 で `resourceNotFound`)。(2) BR4.1 に、取り消し済みの記録への警告が入った。どちらも Assumptions に書かれている

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| aidlc engine sensor-traceability(traceability.json、functional-design) | PASS: gaps 0、orphans 0、invalid_entries 0、invalid_targets 0 | AC5.1.1〜AC5.1.6 と AC6.1.1〜AC6.1.5 のすべてに、実在する規則の対応先がある |
| 契約との突き合わせ(contract-summary.md の C4 の注、C5 の rollback の行、C10 の3つの呼び出し) | 整合 | rules.md、entities.md、functional-spec.md の呼び出し名、オプション名、`reason` の値が契約と一致している |
| 実装済みの U6 との突き合わせ(`internal/cli/apply.go` の `rollbackCommand`) | 整合 | BR7.4 は実装済みの動作(`--record-from`、`--profile`)と一致する |

### Summary

1回目の指摘(Major 3、Minor 6)はすべて直っており、印を取ってから記録を読む順番、C10 の3つの呼び出し、U6 との整合のいずれも、実装者が推測せずに作れる粒度になった。新しくブロックする指摘はなく、読み戻しの待ちの回数、まとめる数、保存の頻度は NFR の設計で決める前提として明記されている。
