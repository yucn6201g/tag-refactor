# エンティティ: U7 取り消し

`rollback` が扱うデータの形です。書き換えの記録(契約 C4。項目 `JournalEntry` と rollback の結果 `rollbacks`)と、印(U6 の `LockInfo`)の形は U6 で決めたとおりで、ここでは rollback の中で使う形を定めます。記録の場所(`RecordLocation`)も U6 のものを使います。

出典: 契約 C4・C5・C10・C12、`rules.md`、`functional-design-questions.md` Q1〜Q9、要件 FR6.1〜FR6.5・NFR4、ストーリー US5.1、U6 の `entities.md`(RecordLocation、JournalEntry、LockInfo)。

```yaml
entities:
  - name: RollbackRequest
    description: rollback の1回の実行の指定(コマンドの入口で作る)
    owner: Cli
    attributes:
      - { name: runId, type: string, required: true, constraints: "--run。U6 の runId の形(UTC の秒までと乱数の接尾辞。U6 の規則 5.4)でなければ使い方の誤り(BR1.1)" }
      - { name: recordFrom, type: RecordLocation, required: true, constraints: "--record-from。省略時は手元の既定の場所(BR1.2)。形は U6 の --record-to と同じ" }
      - { name: force, type: boolean, required: true, default: false, constraints: "--force。後から変えられたリソースも戻す(BR3.3)" }
      - { name: yes, type: boolean, required: true, default: false, constraints: "--yes" }
      - { name: forceUnlock, type: boolean, required: true, default: false, constraints: "--force-unlock(BR2.2)" }
      - { name: profile, type: string, required: false, constraints: "--profile。U1 のまま" }
      - { name: lang, type: enum, required: false, allowed: [en, ja], constraints: "--lang。U4 のまま" }
      - { name: interactive, type: boolean, required: true, constraints: "標準入力が端末か(BR4.3)" }

  - name: RollbackTarget
    description: 判定する1つの項目(記録の JournalEntry のうち、touchedKeys が空でなく status が skipped でないもの)の、rollback の中での作業の状態
    owner: Rollbacker
    attributes:
      - { name: arn, type: string, required: true, unique: true }
      - { name: region, type: string, required: true, constraints: "記録の region" }
      - { name: touchedKeys, type: "list<string>", required: true, constraints: "記録の touchedKeys。aws: を含まない" }
      - { name: beforeTags, type: "map<string,string>", required: true, constraints: "記録の beforeTags(aws: を含む)" }
      - { name: plannedAfterTags, type: "map<string,string>", required: true, constraints: "記録の plannedAfterTags" }
      - { name: currentTags, type: "map<string,string>", required: false, constraints: "判定の直前に読んだ今のタグ(aws: を含む)。答えに出なければ「なくなった」(BR3.1)" }
      - { name: verdict, type: enum, required: true, allowed: [alreadyRestored, restore, changedSinceApply, gone, unknown], constraints: "BR3.2 の判定。読み直しのたびに決め直す(BR5.6)。今のタグを読めなかった項目は unknown(BR3.1)" }
      - { name: tagsToSet, type: "map<string,string>", required: true, constraints: "touchedKeys のうち beforeTags にあって、今の値が違うか今ないキーと、その beforeTags の値(BR3.4)" }
      - { name: keysToRemove, type: "list<string>", required: true, constraints: "touchedKeys のうち beforeTags になく、今あるキー(BR3.4)" }
      - { name: removeFirst, type: boolean, required: true, default: false, constraints: "付けると途中で aws: を除いて50を超えるとき true(BR3.5)" }
      - { name: status, type: enum, required: true, allowed: [pending, restored, skipped, failed, alreadyRestored], default: pending, constraints: "pending は rollback の中だけの値で、記録には書かない(BR6.1)" }
      - { name: reason, type: enum, required: false, allowed: [changedSinceApply, resourceNotFound, tagLimitExceeded, readFailed, writeFailed, partialWrite, resultUnknown, notRestored], constraints: "skipped と failed の理由(BR5.2)" }
      - { name: errorKind, type: string, required: false, constraints: "failed のときの誤りの種類(契約 C12)" }
      - { name: confirmed, type: boolean, required: false, constraints: "restored のとき、読み戻しで確かめられたか(BR5.3)" }

  - name: RollbackRun
    description: 記録の rollbacks に足す、1回の rollback の結果(契約 C4)
    owner: Journal
    attributes:
      - { name: rollbackId, type: string, required: true, unique: true, constraints: "runId と同じ形で新しく作る(BR6.3)" }
      - { name: startedAt, type: datetime, required: true }
      - { name: completed, type: boolean, required: true, constraints: "最後まで終えたときだけ true(BR5.5、BR6.2)" }
      - { name: results, type: "list<RollbackResult>", required: true }

  - name: RollbackResult
    description: 1つの項目の rollback の結果(契約 C4)
    owner: Journal
    attributes:
      - { name: arn, type: string, required: true }
      - { name: status, type: enum, required: true, allowed: [restored, skipped, failed, alreadyRestored] }
      - { name: reason, type: string, required: false, constraints: "skipped と failed の理由の名前。restored で読み戻しで確かめられなかったときは notConfirmed。AWS のエラー文は書かない" }

  - name: RollbackSummary
    description: 確認画面に示す要約(BR4.1)
    owner: Rollbacker
    attributes:
      - { name: accountId, type: string, required: true }
      - { name: runId, type: string, required: true }
      - { name: recordWhere, type: string, required: true, constraints: "記録のパスか S3 の URI" }
      - { name: applyCompleted, type: boolean, required: true, constraints: "記録の completed。false なら「途中で止まった apply」と示す" }
      - { name: toRestore, type: integer, required: true, constraints: "verdict が restore の数" }
      - { name: changedSinceApply, type: integer, required: true, constraints: "verdict が changedSinceApply の数。--force なら戻す数に入る" }
      - { name: alreadyRestored, type: integer, required: true }
      - { name: gone, type: integer, required: true, constraints: "verdict が gone の数" }
      - { name: overLimit, type: integer, required: true, constraints: "戻すと50を超えて飛ばす数(BR3.5)" }
      - { name: unreadable, type: integer, required: true, constraints: "今のタグを読めなかった数(BR3.1)" }
      - { name: force, type: boolean, required: true }

  - name: RollbackOutcome
    description: rollback の結果の表示と終了コードのもと(BR7.1、BR7.2)
    owner: Rollbacker
    attributes:
      - { name: rollbackId, type: string, required: false, constraints: "結果を記録に残したとき" }
      - { name: restored, type: integer, required: true }
      - { name: skipped, type: integer, required: true }
      - { name: failed, type: integer, required: true }
      - { name: alreadyRestored, type: integer, required: true }
      - { name: unconfirmed, type: integer, required: true, constraints: "restored のうち読み戻しで確かめられなかった数" }
      - { name: stopped, type: enum, required: false, allowed: [signal, credentials, transient, storage, internal], constraints: "途中で止まった理由(BR5.5)" }
      - { name: saveFailed, type: boolean, required: true, default: false }
      - { name: logFile, type: string, required: false }

relationships:
  - { from: RollbackRequest, to: RecordLocation, cardinality: "1", note: "記録の場所(U6 の RecordLocation)" }
  - { from: RollbackTarget, to: JournalEntry, cardinality: "1", note: "記録の1つの項目から作る" }
  - { from: RollbackSummary, to: RollbackTarget, cardinality: "0..*", note: "要約は RollbackTarget から数える" }
  - { from: RollbackRun, to: RollbackResult, cardinality: "0..*" }
  - { from: RollbackTarget, to: RollbackResult, cardinality: "0..1", note: "判定が済んだ RollbackTarget が1つの RollbackResult になる" }
  - { from: RollbackOutcome, to: RollbackTarget, cardinality: "0..*" }
```

## まとめ

- **RollbackRequest** は、コマンドの指定です。`--run` は必須で、記録の場所は `--record-from`(省略時は手元の既定の場所)で決めます。
- **RollbackTarget** は、1つの項目を戻すまでの作業の状態です。記録の `touchedKeys`・`beforeTags`・`plannedAfterTags` と、判定の直前に読んだ今のタグ(`currentTags`)から、判定(`verdict`)と、付けるタグ(`tagsToSet`)、消すキー(`keysToRemove`)を求めます。どれも `aws:` のキーを含みません。
- **RollbackRun と RollbackResult** は、契約 C4 の `rollbacks` に足す結果です。同じ記録に何回でも足せます。
- **RollbackSummary と RollbackOutcome** は、確認画面と結果の表示、終了コードに使う形です。
