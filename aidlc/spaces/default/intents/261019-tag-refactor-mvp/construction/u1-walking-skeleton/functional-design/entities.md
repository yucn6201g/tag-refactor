# エンティティ: U1 最小の一本

U1 で扱うデータの形です。部品の一覧(`inception/domain-design/components.md`)と契約(`inception/contract-design/contract-summary.md`)のエンティティのうち、U1 の最小版で使う部分だけを定めます。後の作業単位は、ここに項目を足します。

出典: `functional-design-questions.md` Q1〜Q5、契約 C1・C5・C6・C7・C9・C11・C12、要件 FR1.6、FR2.2、FR4.1、FR4.5、FR4.6、FR4.7、FR7.4〜FR7.8。

```yaml
entities:
  - name: AccountContext
    description: 実行に使うアカウントと、読むリージョン
    owner: AwsAccess
    attributes:
      - { name: accountId, type: string, required: true, constraints: "12桁の数字" }
      - { name: callerArn, type: string, required: true, constraints: "表示には使うが、認証情報は含まない" }
      - { name: profile, type: string, required: false, constraints: "--profile の値。なければ標準の設定" }
      - { name: region, type: string, required: true, constraints: "--region、なければ標準の設定のリージョン(BR1.1)" }

  - name: Resource
    description: タグの付いた1つのリソース
    owner: Inventory
    attributes:
      - { name: arn, type: string, required: true, unique: true, constraints: "文字列としてそのまま扱う(AS6)" }
      - { name: region, type: string, required: true }
      - { name: resourceType, type: string, required: true, constraints: "U1 では ARN のサービスと種類の部分(例 ec2:instance)。AWS::EC2::Instance の形への変換は U2" }
      - { name: tags, type: "map<string,string>", required: true, constraints: "aws: で始まるキーも含めて保持する" }
    relationships:
      - { to: InventorySnapshot, cardinality: "many-to-one", direction: "Resource → InventorySnapshot" }

  - name: InventorySnapshot
    description: 1回の収集の結果
    owner: Inventory
    attributes:
      - { name: snapshotId, type: string, required: true, unique: true }
      - { name: accountId, type: string, required: true, references: AccountContext.accountId }
      - { name: scannedAt, type: datetime, required: true, constraints: "UTC" }
      - { name: scannedRegions, type: "list<string>", required: true, constraints: "U1 では1つ" }
      - { name: failedRegions, type: "list<{region, reason}>", required: true, default: "[]" }
      - { name: resources, type: "list<Resource>", required: true }
    relationships:
      - { to: AccountContext, cardinality: "many-to-one", direction: "InventorySnapshot → AccountContext" }

  - name: KeyVariantGroup
    description: 大文字・小文字と区切り文字の違いだけで同じになるキーのまとまり(scan の結果)
    owner: Inventory
    attributes:
      - { name: normalizedKey, type: string, required: true, unique: true, constraints: "BR2.2 の正規化をしたキー" }
      - { name: spellings, type: "list<{key, resourceCount}>", required: true, min: 1, constraints: "件数の多い順。同じ件数ならキーの文字列の順" }
      - { name: totalResources, type: integer, required: true, min: 1, constraints: "どれかの書き方を持つリソースの数。1つのリソースは1回だけ数える" }
      - { name: isVariant, type: boolean, required: true, constraints: "spellings が2つ以上なら true" }

  - name: RuleSet
    description: ルールファイル(契約 C1)を読んだ結果。U1 ではキーの別名だけを使う
    owner: RuleEngine
    attributes:
      - { name: sourcePath, type: string, required: true }
      - { name: version, type: integer, required: true, allowed: [1] }
      - { name: keyRules, type: "list<KeyRule>", required: true, default: "[]" }
      - { name: warnings, type: "list<{line, key, message}>", required: true, default: "[]" }
    constraints:
      - "1つの別名は、1つの正しいキーにだけ属する(BR3.2)"

  - name: KeyRule
    description: 正しいキー名と、その別名
    owner: RuleEngine
    attributes:
      - { name: canonicalKey, type: string, required: true, unique: true, constraints: "aws: で始まらない" }
      - { name: aliases, type: "list<string>", required: true, default: "[]", constraints: "canonicalKey と同じ文字列を含まない。aws: で始まるものは除く" }
    relationships:
      - { to: RuleSet, cardinality: "many-to-one", direction: "KeyRule → RuleSet" }

  - name: Change
    description: 1つのリソースの1つのキーに対する変更。U1 では名前の変更だけ
    owner: Planner
    attributes:
      - { name: changeId, type: string, required: true, unique: true, constraints: "変更案の中で一意。例: c-000001" }
      - { name: resourceArn, type: string, required: true, references: Resource.arn }
      - { name: kind, type: enum, required: true, allowed: [renameKey], constraints: "U3・U4 で changeValue、addRequired、removeKey を足す" }
      - { name: fromKey, type: string, required: true }
      - { name: toKey, type: string, required: true }
      - { name: value, type: string, required: true, constraints: "名前の変更では値は変わらない" }
      - { name: mergesIntoExisting, type: boolean, required: true, default: false, constraints: "その変更を当てる時点で toKey が既にある場合 true(BR4.2)" }
      - { name: impacted, type: boolean, required: true, default: false }

  - name: KeyConflict
    description: 名前を変えるとキーがぶつかり、値が違うため変えなかったもの
    owner: Planner
    attributes:
      - { name: resourceArn, type: string, required: true, references: Resource.arn }
      - { name: canonicalKey, type: string, required: true }
      - { name: presentTags, type: "map<string,string>", required: true, min: 2, constraints: "正しいキーと別名のうち、リソースに付いているものと値" }

  - name: Plan
    description: 1回の plan の結果
    owner: Planner
    attributes:
      - { name: planId, type: string, required: true, unique: true }
      - { name: accountId, type: string, required: true, references: AccountContext.accountId }
      - { name: createdAt, type: datetime, required: true }
      - { name: regions, type: "list<string>", required: true }
      - { name: changes, type: "list<Change>", required: true, default: "[]" }
      - { name: conflicts, type: "list<KeyConflict>", required: true, default: "[]" }
      - { name: impactFindings, type: "list<ImpactFinding>", required: true, default: "[]" }
      - { name: unreadPolicies, type: "list<UnreadPolicy>", required: true, default: "[]" }
      - { name: inventoryComplete, type: boolean, required: true, constraints: "U1 では常に true(読めなければ誤りで終わるため)。U2 で読めなかったリージョンがあれば false になる" }
      - { name: impactAnalysisComplete, type: boolean, required: true }
    relationships:
      - { to: InventorySnapshot, cardinality: "one-to-one", direction: "Plan → InventorySnapshot" }
      - { to: RuleSet, cardinality: "many-to-one", direction: "Plan → RuleSet" }

  - name: PolicySource
    description: 読んだ IAM の管理ポリシー(U1 では利用者が作り、ユーザー・ロール・グループに付いているもの)
    owner: ImpactAnalyzer
    attributes:
      - { name: policyRef, type: string, required: true, unique: true, constraints: "ポリシーの ARN" }
      - { name: sourceKind, type: enum, required: true, allowed: [iamManaged], constraints: "U5 で他の種類を足す" }
      - { name: statements, type: "list<{sid, index, conditions}>", required: true }

  - name: ImpactFinding
    description: 変更の影響を受ける、または判定できない条件
    owner: ImpactAnalyzer
    attributes:
      - { name: verdict, type: enum, required: true, allowed: [affected, undecided] }
      - { name: policyRef, type: string, required: true, references: PolicySource.policyRef }
      - { name: sourceKind, type: enum, required: true, allowed: [iamManaged] }
      - { name: statementId, type: string, required: true, constraints: "Sid。なければ「#<0から始まる番号>」" }
      - { name: conditionOperator, type: string, required: true, constraints: "例 StringEquals" }
      - { name: conditionKey, type: string, required: true, constraints: "ポリシーに書かれたとおり" }
      - { name: conditionValue, type: string, required: false, constraints: "値が複数なら「, 」でつなぐ" }
      - { name: changeIds, type: "list<string>", required: true, min: 1, references: Change.changeId }

  - name: UnreadPolicy
    description: 読めなかったポリシーの種類と理由
    owner: ImpactAnalyzer
    attributes:
      - { name: sourceKind, type: enum, required: true, allowed: [iamManaged] }
      - { name: policyRef, type: string, required: false, constraints: "個別のポリシーだけが読めなかった場合、その ARN" }
      - { name: reason, type: string, required: true, constraints: "足りない API の名前を含める" }

  - name: TagChangeInput
    description: Planner から影響分析に渡す変更(契約 C9。U4 が定義し U5 が実装する型の最小版)
    owner: Planner
    attributes:
      - { name: changeId, type: string, required: true, references: Change.changeId }
      - { name: resourceArn, type: string, required: true }
      - { name: kind, type: enum, required: true, allowed: [renameKey] }
      - { name: fromKey, type: string, required: true }
      - { name: toKey, type: string, required: true }
      - { name: fromValue, type: string, required: true, constraints: "名前の変更では Change.value と同じ" }
      - { name: toValue, type: string, required: true, constraints: "名前の変更では Change.value と同じ" }

  - name: ImpactResult
    description: 影響分析の結果(契約 C9)
    owner: ImpactAnalyzer
    attributes:
      - { name: findings, type: "list<ImpactFinding>", required: true, default: "[]" }
      - { name: unreadPolicies, type: "list<UnreadPolicy>", required: true, default: "[]" }
      - { name: complete, type: boolean, required: true, constraints: "Plan.impactAnalysisComplete にそのまま写す" }

  - name: CommandOutcome
    description: 1回のコマンドの結論。終了コードを決める元
    owner: Cli
    attributes:
      - { name: command, type: enum, required: true, allowed: [scan, plan] }
      - { name: error, type: "ErrorKind | null", required: true, constraints: "契約 C12" }
      - { name: incomplete, type: boolean, required: true }
      - { name: needsReview, type: boolean, required: true, constraints: "表記ゆれ、変更、ぶつかり、影響のどれかがある" }
      - { name: exitCode, type: enum, required: true, allowed: [0, 1, 2], constraints: "BR6.3" }
```

## まとめ

- **集める側:** `AccountContext`(どのアカウントのどのリージョンか)と、`InventorySnapshot` の中の `Resource`(タグの付いたリソース)です。
- **scan の結果:** `KeyVariantGroup` です。大文字・小文字と区切り文字の違いだけで同じになるキーをまとめます。
- **plan の結果:** `Plan` が `Change`(名前の変更)、`KeyConflict`(値が違うため変えなかったぶつかり)、`ImpactFinding`(影響)、`UnreadPolicy`(読めなかったポリシー)を持ちます。
- **結論:** `CommandOutcome` が、エラー、不完全、要確認の3つの印から終了コードを決めます。
- U1 で使わない項目(値の別名、必須タグ、IaC の印、保存した変更案など)は、契約にはありますが、ここでは定めません。後の作業単位で足します。
