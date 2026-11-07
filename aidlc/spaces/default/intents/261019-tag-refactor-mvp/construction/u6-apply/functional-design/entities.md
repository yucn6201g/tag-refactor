# エンティティ: U6 書き換えと記録

`apply` が扱うデータの形です。保存した変更案(契約 C2)と、書き換えの記録(契約 C4)の形は契約のとおりで、ここでは apply の中で使う形と、記録・印の形を定めます。保存した変更案の読み込みと、変更後のタグの求め方は U4 の部品(U4 の規則 4.4、1.9)を使います。

出典: 契約 C2・C4・C5・C10・C12、`rules.md`、`functional-design-questions.md` Q1〜Q9、要件 FR5.1〜FR5.11・NFR6・NFR7、ストーリー US4.1・US4.2。

```yaml
entities:
  - name: ApplyRequest
    description: apply の1回の実行の指定(コマンドの入口で作る)
    owner: Cli
    attributes:
      - { name: planFile, type: string, required: false, constraints: "--plan。ruleFile とどちらか1つだけ(BR1.1)" }
      - { name: ruleFile, type: string, required: false, constraints: "--rules。planFile とどちらか1つだけ(BR1.1)" }
      - { name: profile, type: string, required: false, constraints: "--profile。U1 のまま" }
      - { name: yes, type: boolean, required: true, default: false, constraints: "--yes" }
      - { name: allowImpact, type: boolean, required: true, default: false, constraints: "--allow-impact" }
      - { name: recordTo, type: RecordLocation, required: true, constraints: "--record-to。省略時は手元の既定の場所(BR5.1)" }
      - { name: forceUnlock, type: boolean, required: true, default: false, constraints: "--force-unlock(BR3.2)" }
      - { name: lang, type: enum, required: false, allowed: [en, ja], constraints: "--lang。U4 のまま" }
      - { name: interactive, type: boolean, required: true, constraints: "標準入力が端末か(BR2.3)" }

  - name: RecordLocation
    description: 記録と印を置く場所
    owner: Journal
    attributes:
      - { name: kind, type: enum, required: true, allowed: [local, s3] }
      - { name: dir, type: string, required: false, constraints: "local のとき。既定は <ホーム>/.tag-refactor/runs/<accountId>/(BR5.1)" }
      - { name: bucket, type: string, required: false, constraints: "s3 のとき。s3://<bucket>/<prefix> の bucket" }
      - { name: prefix, type: string, required: false, constraints: "s3 のとき。空でもよい。記録は <prefix>/<accountId>/<runId>.json(BR5.2)" }
    constraints:
      - "どちらの kind でも、手元の印とログファイルは既定の場所(<ホーム>/.tag-refactor/runs/<accountId>/)に置く(BR3.1、BR5.2、BR5.5)。dir は記録の置き場所だけを変える"

  - name: ApplyTarget
    description: 書き換える1つのリソースの、apply の中での作業の状態
    owner: Applier
    attributes:
      - { name: arn, type: string, required: true, unique: true }
      - { name: region, type: string, required: true, constraints: "タグの API を呼ぶリージョン(C2 の region。グローバルなリソースも同じ)" }
      - { name: resourceType, type: string, required: true }
      - { name: planBeforeTags, type: "map<string,string>", required: true, constraints: "変更案を作った時点のタグ(C2 の beforeTags。aws: を含む)" }
      - { name: currentTags, type: "map<string,string>", required: false, constraints: "書き換える直前に読んだタグ(aws: を含む)。読めなければ空で、status は failed" }
      - { name: changes, type: "list<Change>", required: true, constraints: "C2 の changes。変更案の順" }
      - { name: plannedAfterTags, type: "map<string,string>", required: true, constraints: "currentTags に changes を順に当てはめた結果(U4 の規則 1.9)。aws: のキーは currentTags のまま" }
      - { name: tagsToSet, type: "map<string,string>", required: true, constraints: "plannedAfterTags のうち、currentTags にないキーか値が違うキー。aws: は含まない(BR3.4、BR3.5)" }
      - { name: keysToRemove, type: "list<string>", required: true, constraints: "currentTags にあって plannedAfterTags にないキー。aws: は含まない(BR3.4、BR3.5)" }
      - { name: touchedKeys, type: "list<string>", required: true, constraints: "tagsToSet のキーと keysToRemove の和。並びはキーの文字列の順" }
      - { name: impacted, type: boolean, required: true, constraints: "C2 の changes のどれかが impacted(affected か undecided)" }
      - { name: iacManagedBy, type: string, required: false, constraints: "C2 の iacManagedBy" }
      - { name: status, type: enum, required: true, allowed: [pending, applied, skipped, failed], default: pending }
      - { name: reason, type: enum, required: false, allowed: [changedSincePlan, resourceNotFound, tagLimitExceeded, nothingToWrite, readFailed, writeFailed, partialWrite, resultUnknown, interrupted], constraints: "skipped と failed の理由と、pending のまま残る理由(resultUnknown、interrupted)。記録には reason と、誤りの種類(C12)を書く(BR4.2)" }
      - { name: removeFirst, type: boolean, required: true, default: false, constraints: "途中の数が50を超えるため、消すキーを先に書くリソース(BR3.6、BR4.1)" }
      - { name: errorKind, type: string, required: false, constraints: "failed のとき。C12 の種類(AccessDenied、Throttled など)。エラー文は入れない" }
      - { name: afterTags, type: "map<string,string>", required: false, constraints: "書き換えた後に読み戻したタグ(読めた場合。BR4.3)" }
      - { name: afterConfirmed, type: boolean, required: false, constraints: "afterTags の aws: を除いた部分が plannedAfterTags と同じなら true(BR4.3)" }
    constraints:
      - "status の移り変わりは functional-spec.md の状態の図のとおり"
      - "touchedKeys が空のリソースは書き換えない(reason nothingToWrite で skipped。終了コードには数えない。BR3.4)"

  - name: ApplySummary
    description: 確認画面に出す要約(BR2.1)
    owner: Applier
    attributes:
      - { name: accountId, type: string, required: true }
      - { name: planId, type: string, required: true }
      - { name: planCreatedAt, type: datetime, required: false, constraints: "--plan のとき" }
      - { name: regionCount, type: integer, required: true }
      - { name: resourceCount, type: integer, required: true, constraints: "変更のあるリソースの数" }
      - { name: changeCountsByKind, type: "map<kind,integer>", required: true, constraints: "renameKey、changeValue、addRequired、removeKey" }
      - { name: impactedChanges, type: "list<ImpactedChange>", required: true, constraints: "影響のある変更。記号(! か ?)と文字で示す" }
      - { name: inventoryIncomplete, type: boolean, required: true }
      - { name: impactIncomplete, type: boolean, required: true }
      - { name: iacManagedCount, type: integer, required: true }
      - { name: recordLocation, type: RecordLocation, required: true }

  - name: RunRecord
    description: 書き換えの記録(契約 C4 の形。U7 が読む)
    owner: Journal
    attributes:
      - { name: version, type: integer, required: true, allowed: [1] }
      - { name: toolVersion, type: string, required: true }
      - { name: runId, type: string, required: true, unique: true, constraints: "BR5.4 の形" }
      - { name: accountId, type: string, required: true }
      - { name: planId, type: string, required: true, constraints: "--rules のときは apply の中で作った変更案の ID" }
      - { name: startedAt, type: datetime, required: true, constraints: "UTC" }
      - { name: completed, type: boolean, required: true, constraints: "Complete まで進んだときだけ true" }
      - { name: entries, type: "list<JournalEntry>", required: true }
      - { name: rollbacks, type: "list<RollbackResult>", required: false, constraints: "U7 が書く" }

  - name: JournalEntry
    description: 記録の1つの項目(契約 C4 の entries)
    owner: Journal
    attributes:
      - { name: arn, type: string, required: true }
      - { name: region, type: string, required: true }
      - { name: touchedKeys, type: "list<string>", required: true, constraints: "aws: を含まない" }
      - { name: beforeTags, type: "map<string,string>", required: true, constraints: "ApplyTarget.currentTags(書き換える直前に読んだタグ)" }
      - { name: plannedAfterTags, type: "map<string,string>", required: true }
      - { name: afterTags, type: "map<string,string>", required: false }
      - { name: status, type: enum, required: true, allowed: [pending, applied, skipped, failed] }
      - { name: reason, type: string, required: false, constraints: "ApplyTarget.reason と errorKind。エラー文は入れない" }
    constraints:
      - "touchedKeys・beforeTags・plannedAfterTags は、そのリソースを書き換えるより先に保存する(BR3.7)"
      - "項目の種類ごとの値は functional-spec.md の「記録の項目の値」の表のとおり。書いていない項目(飛ばした、読めなかった)の touchedKeys は空"
      - "U7 は status に関係なく、touchedKeys が空でない項目を契約 C4 の表で判定する(BR4.2)"

  - name: LockInfo
    description: 同時実行を防ぐ印の中身(BR3.1、BR3.2)
    owner: Journal
    attributes:
      - { name: accountId, type: string, required: true }
      - { name: purpose, type: enum, required: true, allowed: [apply, rollback] }
      - { name: runId, type: string, required: true }
      - { name: createdAt, type: datetime, required: true, constraints: "UTC" }
      - { name: host, type: string, required: true, constraints: "ホスト名" }
      - { name: pid, type: integer, required: true }
      - { name: toolVersion, type: string, required: true }
    constraints:
      - "手元の印は --record-to に関係なく常に <ホーム>/.tag-refactor/runs/<accountId>/.lock。S3 に記録するときは <prefix>/<accountId>/.lock も。中身は同じ JSON(BR3.1)"
      - "runId は印を取る前に作り、Begin に同じ値を渡す(BR5.4)。rollback(U7)は自分の rollbackId をここに書く"

  - name: ApplyResult
    description: apply の終わりに示す結果(BR6.1)
    owner: Applier
    attributes:
      - { name: runId, type: string, required: false, constraints: "記録を始めた場合だけ" }
      - { name: appliedCount, type: integer, required: true }
      - { name: skipped, type: "list<ApplyTarget>", required: true, constraints: "reason nothingToWrite を除く" }
      - { name: failed, type: "list<ApplyTarget>", required: true, constraints: "failed と、結果が分からず pending のまま残ったもの(resultUnknown)" }
      - { name: conflictCount, type: integer, required: true, constraints: "変更案の衝突の件数(書き換えないもの)" }
      - { name: unsetRequiredCount, type: integer, required: true, constraints: "値が未指定の必須タグの件数(書き換えないもの)" }
      - { name: unconfirmedCount, type: integer, required: true, constraints: "afterConfirmed が false か、読み戻せなかった applied の数" }
      - { name: interrupted, type: boolean, required: true }
      - { name: rollbackCommand, type: string, required: false, constraints: "記録を始めた場合だけ(BR6.3)" }
      - { name: logFile, type: string, required: false, constraints: "失敗か飛ばしがあったとき(BR5.5)" }
      - { name: exitCode, type: integer, required: true, allowed: [0, 1, 2], constraints: "BR6.2" }

relationships:
  - { from: ApplyRequest, to: RecordLocation, cardinality: "1..1" }
  - { from: ApplySummary, to: ApplyTarget, cardinality: "1..*", note: "要約は ApplyTarget から数える" }
  - { from: RunRecord, to: JournalEntry, cardinality: "0..*" }
  - { from: ApplyTarget, to: JournalEntry, cardinality: "0..1", note: "記録を始めたあとは、1つの ApplyTarget が1つの JournalEntry になる" }
  - { from: ApplyResult, to: ApplyTarget, cardinality: "1..*" }
```

## まとめ

- **ApplyRequest と RecordLocation** は、コマンドの指定と記録の場所です。`--plan` と `--rules` はどちらか1つです。記録は既定で手元(ホームの下、アカウントごと)、指定すれば S3 に置き、印とログファイルはどちらの場合も手元に置きます。
- **ApplyTarget** は、1つのリソースを書き換えるまでの作業の状態です。書き換える直前に読んだ今のタグ(`currentTags`)から、書き換え後に期待するタグ(`plannedAfterTags`)、付けるタグ(`tagsToSet`)、消すキー(`keysToRemove`)を求めます。どれも `aws:` のキーを含みません。
- **RunRecord と JournalEntry** は契約 C4 の記録です。`beforeTags` には、変更案の時点ではなく、書き換える直前に読んだタグを書きます(変化の確かめ(BR3.3)で、`aws:` を除けば変更案の時点と同じであることを確かめたもの)。
- **LockInfo** は、同時実行を防ぐ印の中身です。残ってしまったときに、人が「実行中でない」と判断できる情報を持ちます。
- **ApplySummary と ApplyResult** は、確認画面と結果の表示に使う形です。
