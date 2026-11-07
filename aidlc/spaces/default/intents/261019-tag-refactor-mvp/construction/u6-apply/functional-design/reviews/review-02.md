## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-28T14:56:01Z
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/rules.md > BR3.6、BR4.1 | 50個の上限の判定が、書き換え後のタグ(plannedAfterTags)の数だけで行われている。しかし BR4.1 は「付ける」を先、「消す」を後に書くため、途中で「書き換え前のキー ∪ 書き換え後のキー」の数になる。aws: を除いて50個ちょうど付いているリソースでキー名を変える(env→Environment)と、最終は50個でも、途中で51個になり TooManyTags で失敗する。キー名の変更は主要な変更なので、よく起きる。BR3.6 を通ったのに失敗(終了コード 1)になり、Q7 の「上限の違う種類」の失敗と区別できない | 上限の判定を、書き込み中の最大の数(aws: を除く currentTags と plannedAfterTags のキーの和集合)でも行うと決める。最終の数は50以下で途中だけ超えるリソースは、(a) tagLimitExceeded として飛ばすか、(b) 消すを先に書く(各キーが beforeTags か plannedAfterTags と同じなので、C4 の表の2行目として戻せる。beforeTags は先に保存済み)と BR4.1 に例外として書く。どちらにするかを決め、AC4.2.5 に対応する場面を足す | Resolved (reviewer) |
| R-02 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/rules.md > BR4.2、BR3.7、functional-spec.md > 状態 S1 と表 | 書き込みの失敗の記録が契約 C4 とずれている。(1) C4 は pending を「まだ書き換えていない、または結果が分からない」ものとするが、BR4.2 は時間切れなど結果が分からない失敗もすべて failed にする。(2) 付けるのに成功し、消すのに失敗したリソースは中間の状態で書き換わっているのに failed になり、読み戻しも記録も定めがない(afterTags を記録するか)。C4 の rollback の表は status を使わないが、U7 が failed の項目を戻すかどうかは契約にも U6 の設計にもない。戻さないなら、書き換わったまま取り消せない。(3) skipped(changedSincePlan、tagLimitExceeded)と failed(readFailed)の項目の touchedKeys、beforeTags、plannedAfterTags に何を入れるか決まっておらず、C4 は3つとも必須。読めなかったものに currentTags は空としか書かれず、空を beforeTags に記録すると、実際のタグと違う記録になる | 項目の種類ごとに、記録する値を表にする(書いていない項目の touchedKeys は空にする、読めなかった項目の beforeTags は planBeforeTags にするなど)。失敗が「何も書いていない」「付けたが消せていない」「結果が分からない」のどれかを区別し、後の2つは書き込み後に読み戻して afterTags を記録するか、pending のまま残す。U7 が failed と pending のうち書き換わった可能性のある項目を C4 の表で判定する、と契約 C4 の注に足す | Resolved (reviewer) |
| R-03 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/rules.md > BR3.1、BR5.2 と entities.md > LockInfo、RecordLocation と functional-spec.md > 状態 S2 | 印の置き場所と、すべての終わり方での片付けが決まっていない。(1) BR3.1 と LockInfo の制約は手元の印を `<dir>/.lock` とし、RecordLocation.dir は `--record-to` が手元のフォルダーのとき、そのフォルダー配下になる。一方 BR5.2 は、そのときも印を既定の場所に置くとする。どちらかで、`--record-to` の違う2つの apply や、`--record-from` を使う rollback が同じ印を見るかどうかが変わる。(2) 印を片付ける場面は BR3.1 の「yes 以外、止めたとき、終わったとき」だけで、手順 11 の保存失敗(BR3.7)、手順 8〜10 での Ctrl+C(確認の入力の待ち中は読み込みを中断できない)、S3 の印を取れて手元の印を取れなかった場合、パニックのときの扱いがない。確認の待ち中に印を持つ設計なので、残った印が多くなり、`--force-unlock` が通常の操作になって NFR7 の守りが弱まる | 手元の印は、`--record-to` に関係なく常に既定のフォルダー(<ホーム>/.tag-refactor/runs/<accountId>/.lock)に置くと決め、LockInfo と BR3.1 の `<dir>` を直す(U7 も同じ場所を使うと明記する)。印の取得から終わりまでを、どの終わり方でも Release する1つの規則にまとめる(取得の失敗の巻き戻し、手順 8〜10 のキャンセル、BR3.7 の失敗を含む)。S3 と手元の印の取る順と外す順を決める | Resolved (reviewer) |
| R-04 | Major | aidlc/spaces/default/intents/261019-tag-refactor-mvp/inception/contract-design/contract-summary.md > C6 ObjectStore と construction/u6-apply/functional-design/functional-spec.md > 契約への変更 | S3 の印(Q5)と BR5.2 が必要とする操作が、C6 の ObjectStore にない。ObjectStore は `Put(uri, bytes)` と `Get(uri)` だけで、「同じ名前がなければ作る」条件付きの書き込みも、印を消す Delete もない。契約への変更の表は C4、C5、C10 だけを挙げ、C6 を変えていない。U6 の開発者は、S3 の条件付きの書き込みの失敗(PreconditionFailed、ConditionalRequestConflict)を Conflict に、権限不足を AccessDenied に変える仕組みを決められない | C6 の ObjectStore に PutIfAbsent(印用)と Delete を足し、契約への変更の表に C6 を加える。条件付きの書き込みの失敗を Conflict にする対応と、条件付きの書き込みに対応しない S3 互換の保存先での扱い(使えないと案内する)を決める | Resolved (reviewer) |
| R-05 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/entities.md > LockInfo.runId、rules.md > BR3.1、BR5.4 | LockInfo.runId は必須だが、印は手順 7 で作り、runId は手順 11 の Begin で作る(BR5.4)。C10 の `Lock(accountId, location, purpose)` にも runId はない。印の中身に runId を書く Q6 の回答を実現する順番がない | runId を Lock のときに作って Begin に渡すか、Begin のあとに印を上書きするかを決め、C10 を合わせる | Resolved (reviewer) |
| R-06 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/rules.md > BR4.2、BR4.4、BR4.5 | 認証情報の期限切れの扱いが決まっていない。BR4.2 は CredentialsMissing だけで書き込みを止めるが、期限切れは AccessDenied や Transient として返る可能性があり、その場合は残りのリソースが1件ずつ再試行を使い切って失敗し、長く続く。BR4.4 の記録の保存(S3)も同じ。Ctrl+C を2回押したときの動きもない | 期限切れのトークンを AwsAccess が CredentialsMissing にする、と C6・C12 に書く。同じ種類の失敗が続いたときに新しい書き込みを止める規則(件数か連続回数)と、2回目のキャンセルの動きを足す | Unresolved |
| R-07 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/rules.md > BR3.3、functional-spec.md > 流れ W1 手順 9〜12 | 今のタグを読むのは全リソースを先に行い(手順 9)、書き込みはあとで順に行う(手順 12)。10,000件では読んでから書くまでが長く、その間の変更は上書きされ、beforeTags(読んだ時点)は古くなる。rollback はその間の変更を beforeTags で上書きする。全件を pending で先に保存する(BR3.7)ための設計だが、残る窓の説明がない | 窓が残ることと、書き込みのまとめ(NFR の設計)ごとに今のタグを読み直して比べるかどうかを、Assumptions に明記して決める | Resolved (reviewer) |
| R-08 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/rules.md > BR1.4、BR6.2 | `--rules` で変更が1つもなく、衝突(keyConflicts)や値が未指定の必須タグだけがあるとき、apply は「書き換えるものはない」で終了コード 0 になる。plan は同じ入力で 2 を返し、衝突や未指定が利用者に伝わらない。また inventoryComplete が false の変更案を書き換えて終えたとき、C5 の「結果が不完全は 1」を apply にどう当てるかが BR6.2 に書かれていない | 変更なしの場合に衝突と未指定の件数を示し、終了コードを plan と合わせるかを決める。収集が不完全な変更案で書き換えが成功した場合の終了コードを BR6.2 に明記する | Resolved (reviewer) |
| R-09 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/traceability.json > AC6.1.2、AC6.1.3 と rules.md > BR2.1、BR6.4 | AC6.1.2 を N/A として「apply も同じ部品で言語を決める」と書くが、apply の確認画面と結果を `--lang` と OS の設定に従って日本語で出す規則が U6 にない(ApplyRequest.lang があるだけ)。AC6.1.3 は BR2.1 に対応づけるが、BR2.1 に `NO_COLOR` と端末でないときの色なしの規則がない。reason の `recordFailed` が enum にあるが、どの規則も使わない | BR2.1 と BR6.1 に、言語の決め方と色の規則を U4 の部品(U4 の規則 3.7〜3.9)から使うと書く。AC6.1.2 の N/A の理由を、その規則への対応に直す。使わない `recordFailed` を消すか、使う場面を決める | Resolved (reviewer) |
| R-10 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/rules.md > BR4.7、BR4.4 と functional-spec.md > 状態 S1、記録の項目の値 | BR4.7 は、記録を始めて pending で保存したあとに、書く直前の読み直しで変化が分かったリソースを skipped(changedSincePlan)にする。しかし S1 の図には pending から skipped への移り変わりがない。「記録の項目の値」の表は変わっていたリソースの touchedKeys を空、beforeTags を読み直したタグとするが、BR4.4 と契約 C10 の RecordAfter は afterTags、status、reason しか更新できない。そのため、この場合の記録は touchedKeys が空でない(BR3.7 で保存した値のまま)skipped の項目になり、表と食い違う。U7 は touchedKeys が空でない項目を判定するので、書いていないのに判定の対象になる。他人の変更後のタグが plannedAfterTags と偶然同じなら、U7 は「この apply が書いた」と見て beforeTags に戻してしまう | S1 の図に pending から skipped(BR4.7)を足す。記録の項目の値の表に、BR4.7 で飛ばした項目の行を分けて書く(touchedKeys は BR3.7 の保存のままか、更新して空にするか)。更新して空にするなら、RecordAfter に touchedKeys を渡すか、別の更新の操作を C10 に足す。更新しないなら、U7 が status が skipped の項目を判定しない、と C4 の注に足す | New |
| R-11 | Minor | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u6-apply/functional-design/functional-spec.md > 書き換え後に期待するタグの例 | 4行目(50個の例)が、前の行との間の空行で表から切り離され、表として表示されない。また、表のあとの「1行目では…」の説明は1行目の例のことで、50個の例(BR3.6)の説明がない | 空行を消して4行目を表に戻し、50個の例の説明(付けるより先に消す理由は表の中の記述で足りるが、途中の状態が C4 の表の2行目になること)を足す | New |

> R-01 Reviewer note: BR3.6 が「aws: を除いた currentTags と plannedAfterTags のキーの和 > 50」を途中の数として判定し、該当するリソースだけ消すを先に書く(BR4.1 の例外、ApplyTarget.removeFirst)と決めた。spec の例の表と業務の場面にも50個ちょうどでキー名を変える場合がある。消すのを先にしても、途中で止まった各キーは beforeTags か plannedAfterTags のどちらかと同じで、C4 の表の2行目として戻せる。

> R-02 Reviewer note: BR4.2 が失敗を「何も書いていない」「一部だけ書いた」「結果が分からない(pending のまま、resultUnknown)」に分け、後の2つは読み戻して afterTags を記録する。spec に項目の種類ごとの値の表があり、C4 に「status に関係なく touchedKeys が空でない項目を判定する」注が足されている(contract-summary.md C4)。C4 の pending の定義とも合う。

> R-03 Reviewer note: 手元の印は `--record-to` に関係なく既定の場所に置くと決め、LockInfo、RecordLocation、BR3.1、BR5.2 が揃った。BR3.8 が正常終了、yes 以外、手順の途中の誤り、BR3.7 と BR4.4 の保存の失敗、Ctrl+C、想定外のエラーのすべてで、S3、手元の順に印を消すと定めた。S3 の印を取れなかったときの手元の印の扱いも決まっている。

> R-04 Reviewer note: C6 の ObjectStore に `PutIfAbsent` と `Delete` が足され(contract-summary.md C6)、412 と 409 は Conflict、501 は StorageFailed と対応づけられた。契約への変更の表に C6 が入っている。

> R-05 Reviewer note: runId を印を取る前に作って `Lock(accountId, location, purpose, runId)` に渡し、Begin がその runId を使う(BR5.4、C10 の改訂、LockInfo)。順番が成り立つ。

> R-06 Reviewer note: 2回目の Ctrl+C の動き(BR4.5)と、Transient が10件続いたときの停止(BR4.2)は足された。しかし BR4.2 の「期限切れのトークンも AwsAccess が CredentialsMissing にする」は、契約への変更の表に C12 も C6 もなく、contract-summary.md の C12(CredentialsMissing は「認証情報がない」)と C6 にも書かれていない。U1 の部品の挙動を U6 が一方的に仮定している。期限切れが AccessDenied として返ると、BR4.2 の止める条件に当たらず、残りのリソースが1件ずつ失敗する。

> R-07 Reviewer note: 手順 9 の全件の読み取りと書き込みの間の窓に対して、BR4.7(書き込みのまとまりごとの読み直し)を足し、残る窓を Assumptions に明記した。

> R-08 Reviewer note: BR1.4 が変更なしのときに衝突と値が未指定の必須タグの件数を示し、終了コードを plan と合わせた。BR6.2 に、収集が不完全な変更案は書き換えが成功しても 1(C5 の「結果が不完全」)と明記された。

> R-09 Reviewer note: BR2.1 と BR6.1 が言語の決め方と色の規則を U4 の規則 3.7〜3.9 から使うと書き、それらは U4 の rules.md に存在する。AC6.1.2 と AC6.1.3 は BR2.1 に対応づけられ、使われなかった `recordFailed` は reason の列挙から消えた。

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| aidlc engine sensor-traceability (traceability.json) | PASS: gaps、orphans、invalid_entries、invalid_targets すべて空、findings_count 0 | 受け入れ条件 AC4.1.1〜AC6.1.5 は規則に対応づけられ、孤立した規則もない |

### Summary

前回の Major 4件(R-01〜R-04)と Minor 4件(R-05、R-07、R-08、R-09)は、規則、エンティティ、契約 C4・C6・C10 の改訂で解消された。未解決は R-06(期限切れの認証情報の分類が C6・C12 に書かれていない)と、新しい Minor 2件(BR4.7 で飛ばした項目の記録の扱い、spec の表の崩れ)だけで、Critical も Major もない。実装を妨げるものではなく、READY とする。
