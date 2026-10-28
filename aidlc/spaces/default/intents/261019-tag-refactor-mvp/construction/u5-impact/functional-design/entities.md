# エンティティ: U5 影響分析

U1 で作った影響分析の最小版の形に、U5 で足す項目と、照合の途中で使う値を定めます。入出力の型(`TagChangeInput`、`ImpactResult`、`ImpactFinding`、`UnreadPolicy`。契約 C9)は U1 の形を使い、U5 では `UnreadPolicy` に項目を1つ足すだけです。ここに書かない項目は U1・U4 のままです。

出典: 契約 C2・C9、`rules.md`、`functional-design-questions.md` Q1〜Q6、U1 の影響分析の規則 5.1〜5.6、U4 の変更案(`Plan.regions`、`AttachImpact`)。

```yaml
entities:
  - name: TagChangeInput
    description: 影響分析に渡す1つの変更(契約 C9。U1 のまま)
    owner: Planner
    attributes:
      - { name: changeId, type: string, required: true, unique: true, constraints: "C2 の changes[].changeId と同じ" }
      - { name: resourceArn, type: string, required: true, constraints: "文字列のまま比べる(AS6)" }
      - { name: kind, type: enum, required: true, allowed: [renameKey, changeValue, addRequired, removeKey] }
      - { name: fromKey, type: string, required: false, constraints: "renameKey、changeValue、removeKey で使う" }
      - { name: toKey, type: string, required: false, constraints: "renameKey、addRequired で使う" }
      - { name: fromValue, type: string, required: false, constraints: "changeValue で使う。renameKey と removeKey にも参考で入る" }
      - { name: toValue, type: string, required: false, constraints: "changeValue と addRequired で使う。renameKey では fromValue と同じ(U4)" }
    constraints:
      - "変更のキー(BR4.1): renameKey は fromKey と toKey、changeValue と removeKey は fromKey、addRequired は toKey"
      - "IAM のユーザーかロールの変更かは resourceArn の形(arn:<partition>:iam::<account>:user/… か role/…)で決める(BR3.3)"

  - name: PolicySource
    description: 読んだ1つのポリシー。照合が終われば文書は捨てる(U1 のまま)
    owner: ImpactAnalyzer
    attributes:
      - { name: policyRef, type: string, required: true, unique: true, constraints: "BR1.6 の形" }
      - { name: sourceKind, type: enum, required: true, allowed: [iamManaged, iamInline, s3Bucket, kmsKey, sqsQueue, snsTopic, secretsManagerSecret] }
      - { name: attachedResourceArn, type: string, required: false, constraints: "リソースポリシーのときだけ。そのポリシーが付いたリソースの ARN(BR3.2)" }
      - { name: region, type: string, required: false, constraints: "リソースポリシーを読んだリージョン。IAM は空" }
      - { name: statements, type: "list<PolicyStatement>", required: true, constraints: "文書の順" }
    relationships:
      - { to: PolicyStatement, cardinality: "1..*", direction: "PolicySource → PolicyStatement" }

  - name: PolicyStatement
    description: ポリシーの1つのステートメント
    owner: ImpactAnalyzer
    attributes:
      - { name: statementId, type: string, required: true, constraints: "Sid。なければ #<0から始まる番号>(U1 のまま)" }
      - { name: resources, type: "list<string>", required: false, constraints: "Resource と NotResource の値。変数を探すだけに使う(BR2.4)" }
      - { name: conditions, type: "list<Condition>", required: false }

  - name: Condition
    description: ステートメントの1つの条件(演算子と条件キーの組)
    owner: ImpactAnalyzer
    attributes:
      - { name: operator, type: string, required: true, constraints: "書いたまま(例 ForAnyValue:StringLikeIfExists)" }
      - { name: baseOperator, type: string, required: true, constraints: "ForAnyValue:、ForAllValues: と IfExists を外したもの(BR2.3)" }
      - { name: conditionKey, type: string, required: true, constraints: "書いたまま" }
      - { name: values, type: "list<string>", required: true, constraints: "1つの値も1件のリストにする。数や真偽値は文字にする" }

  - name: TagReference
    description: 照合の途中で使う、タグへの1つの参照。条件キー、条件キーの中の変数、条件の値の中の変数、Resource・NotResource の中の変数のどれかから作る
    owner: ImpactAnalyzer
    attributes:
      - { name: location, type: enum, required: true, allowed: [conditionKey, keyVariable, conditionValue, resource, notResource] }
      - { name: tagKind, type: enum, required: true, allowed: [ResourceTag, RequestTag, PrincipalTag, TagKeys] }
      - { name: awsPrefix, type: boolean, required: true, constraints: "前置が aws なら true" }
      - { name: tagKeyPattern, type: string, required: false, constraints: "タグのキー(TagKeys では空。キーは条件の値にある)。* と ? を含めばパターン。${ を含めば判定できない" }
      - { name: condition, type: Condition, required: false, constraints: "location が conditionKey、keyVariable、conditionValue のとき" }
      - { name: resourceText, type: string, required: false, constraints: "location が resource か notResource のとき、その値" }

  - name: ImpactFinding
    description: 変更の影響を受ける、または判定できない1つの条件(契約 C2・C9。U1 の形のまま)
    owner: ImpactAnalyzer
    attributes:
      - { name: verdict, type: enum, required: true, allowed: [affected, undecided] }
      - { name: policyRef, type: string, required: true }
      - { name: sourceKind, type: enum, required: true, allowed: [iamManaged, iamInline, s3Bucket, kmsKey, sqsQueue, snsTopic, secretsManagerSecret] }
      - { name: statementId, type: string, required: true }
      - { name: conditionOperator, type: string, required: false, constraints: "条件のときは書いたままの演算子。Resource と NotResource のときは空。U5 で保存した変更案と JSON の出力にも書く(BR5.1)" }
      - { name: conditionKey, type: string, required: true, constraints: "条件のときは条件キー。Resource と NotResource のときは \"Resource\" か \"NotResource\"(BR5.1)" }
      - { name: conditionValue, type: string, required: false, constraints: "条件の値を \", \" でつないだもの。Resource・NotResource のときは、そのすべての値を文書の順に \", \" でつないだもの" }
      - { name: changeIds, type: "list<string>", required: true, constraints: "1件以上。変更の順。重ならない" }
    constraints:
      - "(policyRef、statementId、conditionOperator、conditionKey、verdict)ごとに1件(BR5.1)"
    relationships:
      - { to: PolicySource, cardinality: "*..1", direction: "ImpactFinding → PolicySource" }
      - { to: TagChangeInput, cardinality: "*..*", direction: "ImpactFinding → TagChangeInput(changeIds)" }

  - name: UnreadPolicy
    description: 読めなかった1つのポリシー、またはリージョンの中の1種類の一覧(契約 C2・C9)
    owner: ImpactAnalyzer
    attributes:
      - { name: sourceKind, type: enum, required: true, allowed: [iamManaged, iamInline, s3Bucket, kmsKey, sqsQueue, snsTopic, secretsManagerSecret] }
      - { name: policyRef, type: string, required: false, constraints: "1つのポリシーのとき BR1.6 の形。一覧を読めなかったときは空" }
      - { name: region, type: string, required: false, constraints: "U5 で足す。リソースポリシーのリージョン。IAM と S3 の一覧は空" }
      - { name: reason, type: string, required: true, constraints: "エラーの種類と操作の名前だけ。エラー文は入れない(U1 のまま)" }

  - name: ImpactResult
    description: 影響分析の結果(契約 C9。U1 のまま)
    owner: ImpactAnalyzer
    attributes:
      - { name: findings, type: "list<ImpactFinding>", required: true, default: "[]", constraints: "BR5.2 の順" }
      - { name: unreadPolicies, type: "list<UnreadPolicy>", required: true, default: "[]", constraints: "BR5.2 の順" }
      - { name: complete, type: boolean, required: true, constraints: "unreadPolicies が空なら true" }
```

## まとめ

- **入力:** `TagChangeInput`(U4 の変更案から作る。U1 のまま)と、変更案のリージョン(`Plan.regions`)。リージョンは、リソースポリシーを探す範囲に使います(BR1.3)。
- **途中の値:** `PolicySource`、`PolicyStatement`、`Condition`、`TagReference` は照合の途中だけで使い、結果にも保存した変更案にも残しません。
- **出力:** `ImpactFinding` と `UnreadPolicy` は契約 C2 の `impactFindings`、`unreadPolicies` になります。`ImpactFinding` の `conditionOperator` と、`UnreadPolicy` の `policyRef` と `region` は保存した変更案と JSON の出力にも書きます(項目を足すだけなので `version` は 1 のまま。`functional-spec.md` の「契約への変更」)。
