# エンティティ: U4 変更案

U1 の変更案の形(`Plan`、`Change`、`KeyConflict`、影響分析の型)に、U4 で足す項目と、出力の設定を定めます。ここに書かない項目は U1 のままです。影響分析の入出力の型(`TagChangeInput`、`ImpactResult`、`ImpactFinding`、`UnreadPolicy`。契約 C9)は U1 のまま使います。

出典: 契約 C2・C3・C5・C9、`rules.md`、`functional-design-questions.md` Q1〜Q8、U1 の変更案の形、U3 の照会の答え(`RequiredValueAnswer`、正しい値と消すかどうかの答え)。

```yaml
entities:
  - name: Plan
    description: 1回の plan で作る変更案(U1 に項目を足す)。保存するときは契約 C2 の形になる
    owner: Planner
    attributes:
      - { name: planId, type: string, required: true, unique: true, constraints: "plan-YYYYMMDDTHHMMSSZ-xxxx(BR4.3)" }
      - { name: accountId, type: string, required: true }
      - { name: createdAt, type: datetime, required: true, constraints: "UTC" }
      - { name: regions, type: "list<string>", required: true }
      - { name: ruleFile, type: string, required: true, constraints: "使ったルールファイルの場所(参考)" }
      - { name: scannedResources, type: integer, required: true, min: 0 }
      - { name: resources, type: "list<PlannedResource>", required: true, default: "[]", constraints: "変更が1つ以上あるリソースだけ。ARN の順" }
      - { name: conflicts, type: "list<KeyConflict>", required: true, default: "[]" }
      - { name: unsetRequiredTags, type: "list<UnsetRequiredTag>", required: true, default: "[]", constraints: "キーの順" }
      - { name: removeCandidates, type: "list<RemoveCandidate>", required: true, default: "[]", constraints: "キーの順。aws: を含まない(BR1.6)" }
      - { name: unsupportedResourceTypes, type: "list<TypeCount>", required: true, default: "[]", constraints: "U2 の集計のまま" }
      - { name: impactFindings, type: "list<ImpactFinding>", required: true, default: "[]" }
      - { name: unreadPolicies, type: "list<UnreadPolicy>", required: true, default: "[]" }
      - { name: inventoryComplete, type: boolean, required: true }
      - { name: impactAnalysisComplete, type: boolean, required: true }
    constraints:
      - "同じ収集の結果とルールファイルからは、planId と createdAt を除いて同じ変更案になる(BR1.8)"

  - name: PlannedResource
    description: 変更のある1つのリソース(契約 C2 の resources[])
    owner: Planner
    attributes:
      - { name: arn, type: string, required: true, unique: true }
      - { name: region, type: string, required: true, constraints: "タグの API を呼ぶリージョン" }
      - { name: global, type: boolean, required: true }
      - { name: resourceType, type: string, required: true, constraints: "ARN の形(例 ec2:instance)" }
      - { name: beforeTags, type: "map<string,string>", required: true, constraints: "収集したままのタグ全体(aws: を含む)" }
      - { name: iacManagedBy, type: string, required: false, constraints: "例 cloudformation:logging。なければ空" }
      - { name: changes, type: "list<Change>", required: true, constraints: "1件以上。BR1.8 の順" }
    relationships:
      - { to: Change, cardinality: "1..*", direction: "PlannedResource → Change" }

  - name: Change
    description: 1つのリソースのタグに対する1つの変更(U1 の renameKey に3つの種類を足す)
    owner: Planner
    attributes:
      - { name: changeId, type: string, required: true, unique: true, constraints: "c-000001 から(BR1.8)" }
      - { name: resourceArn, type: string, required: true, references: PlannedResource.arn }
      - { name: kind, type: enum, required: true, allowed: [renameKey, changeValue, addRequired, removeKey] }
      - { name: fromKey, type: string, required: false, constraints: "renameKey、changeValue、removeKey で必須" }
      - { name: toKey, type: string, required: false, constraints: "renameKey、addRequired で必須" }
      - { name: fromValue, type: string, required: false, constraints: "changeValue で必須。renameKey と removeKey では変更前の値を参考に書く" }
      - { name: toValue, type: string, required: false, constraints: "changeValue、addRequired で必須" }
      - { name: mergesIntoExisting, type: boolean, required: true, default: false, constraints: "renameKey だけ。toKey が既にあるとき true(U1 BR4.2)。true なら toKey に書かず fromKey を消すだけ(BR1.1、BR1.9)" }
      - { name: impacted, type: boolean, required: true, default: false, constraints: "影響の finding のどれかに入れば true(BR2.5)" }
      - { name: mark, type: enum, required: true, default: none, allowed: [none, affected, undecided], constraints: "表示の印。affected が優先(BR2.5)" }
    constraints:
      - "fromKey と toKey は aws: で始まらない(BR1.6)"
      - "changeValue の fromKey は、同じリソースの renameKey の toKey(変更後のキー名)になることがある(Q1)"

  - name: KeyConflict
    description: 正しいキー名と別名に、値の別名を当てても違う値が付いているリソース(U1 のまま。比べ方だけ変わる)。保存と JSON では契約 C2 に足す keyConflicts[] になる
    owner: Planner
    attributes:
      - { name: resourceArn, type: string, required: true, constraints: "保存と JSON では arn" }
      - { name: canonicalKey, type: string, required: true }
      - { name: presentTags, type: "map<string,string>", required: true, constraints: "収集したままの値" }
      - { name: keyOrder, type: "list<string>", required: true, constraints: "表示の順。保存と JSON には書かない(presentTags のキーを別名の順、正しいキー名の順に並べ直せるため)" }

  - name: UnsetRequiredTag
    description: 値を決められなかった必須タグと、そのリソース(契約 C2 の unsetRequiredTags[])
    owner: Planner
    attributes:
      - { name: key, type: string, required: true }
      - { name: arns, type: "list<string>", required: true, constraints: "1件以上。ARN の順" }

  - name: RemoveCandidate
    description: どの規則にもないキー(契約 C2 の removeCandidates はキー名だけ。件数は表示に使う)
    owner: Planner
    attributes:
      - { name: key, type: string, required: true, constraints: "aws: で始まらない" }
      - { name: resourceCount, type: integer, required: true, min: 1 }

  - name: PlanSummary
    description: 4つの形式で共通に使う集計(契約 C3 の summary)
    owner: Planner
    attributes:
      - { name: scannedResources, type: integer, required: true }
      - { name: resourcesToChange, type: integer, required: true, constraints: "重なっても1件(BR2.1)" }
      - { name: changesByKind, type: "map<kind,integer>", required: true }
      - { name: changeLines, type: "list<ChangeLine>", required: true, constraints: "一覧の形の行" }
      - { name: conflictResources, type: integer, required: true, constraints: "契約 C3 の summary に足す項目" }
      - { name: unsetRequiredResources, type: integer, required: true, constraints: "契約 C3 の summary に足す項目" }
      - { name: affectedReferences, type: integer, required: true }
      - { name: undecidedReferences, type: integer, required: true }
      - { name: iacManagedResources, type: integer, required: true }

  - name: ChangeLine
    description: 一覧の形の1行(同じ内容の変更をまとめたもの。まとめる単位は種類ごとに BR2.1 のとおり)
    owner: Planner
    attributes:
      - { name: kind, type: enum, required: true, allowed: [renameKey, changeValue, addRequired, removeKey] }
      - { name: fromKey, type: string, required: false, constraints: "renameKey、changeValue、removeKey" }
      - { name: toKey, type: string, required: false, constraints: "renameKey、addRequired" }
      - { name: fromValue, type: string, required: false, constraints: "changeValue だけ" }
      - { name: toValue, type: string, required: false, constraints: "changeValue、addRequired(値が1種類のときだけ。複数なら空で「(from rules)」と示す)" }
      - { name: resources, type: integer, required: true, min: 1 }

  - name: OutputOptions
    description: 出力の設定(コマンドの開始時に決まる)
    owner: Display
    attributes:
      - { name: format, type: enum, required: true, default: table, allowed: [table, diff, json, markdown], constraints: "scan は diff を受け付けない(契約 C5)" }
      - { name: language, type: enum, required: true, allowed: [en, ja], constraints: "BR3.8" }
      - { name: colorStdout, type: boolean, required: true, constraints: "BR3.7" }
      - { name: colorStderr, type: boolean, required: true, constraints: "BR3.7" }
      - { name: outPath, type: string, required: false, constraints: "plan だけ。既にあるファイルは不可(BR4.1)" }
```

## 要約

- **変更案(`Plan`):** U1 の項目に、使ったルールファイル、変更のあるリソースの一覧(`PlannedResource`)、値が未指定、消す候補、タグを変更できない種類を足します。保存すると契約 C2 の形になり、衝突は C2 に足す項目 `keyConflicts` に入ります。
- **変更(`Change`):** U1 のキー名の変更に、値の変更、必須タグの追加、削除を足します。種類ごとの必須の項目は契約 C2 の表のとおりです。表示の印(`!` か `?`)を持ちます。
- **集計(`PlanSummary`、`ChangeLine`):** 4つの形式が同じ数を出すための集計です。形式ごとに数え直しません。
- **出力の設定(`OutputOptions`):** 形式、言語、色(標準出力と標準エラーで別々)、保存先です。
