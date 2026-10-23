# エンティティ: U2 収集

U1 のエンティティ(`construction/u1-walking-skeleton/functional-design/entities.md`)に、U2 で足す項目を定めます。ここに書かない項目は U1 のままです。

出典: 契約 C3・C7・C11、`rules.md`、`functional-design-questions.md` Q1〜Q3。

```yaml
entities:
  - name: Resource
    description: 1つのリソース(U1 に項目を足す)
    owner: Inventory
    attributes:
      - { name: global, type: boolean, required: true, default: false, constraints: "ARN にリージョンがないものは true(BR2.2)" }
      - { name: everTagged, type: boolean, required: true, constraints: "タグの取得の API が返したものは true。Config か Resource Explorer で見つけたものは false(BR3.1)" }
      - { name: iacManagedBy, type: "string | null", required: true, default: null, constraints: "cloudformation:<スタック名>(BR2.3)" }

  - name: InventorySnapshot
    description: 1回の収集の結果(U1 に項目を足す)
    owner: Inventory
    attributes:
      - { name: enabledRegions, type: "list<string>", required: true, constraints: "アカウントで有効なリージョン(BR1.2)" }
      - { name: scannedRegions, type: "list<string>", required: true, constraints: "読んだリージョン。--region がなければ enabledRegions と同じ" }
      - { name: failedRegions, type: "list<FailedRegion>", required: true, default: "[]" }
      - { name: untaggedDiscovery, type: enum, required: true, allowed: [complete, partial, unavailable], constraints: "全体の状態(BR3.2)。契約 C3 の complete.untaggedDiscovery はリージョンごとの内訳に広げる" }
      - { name: untaggedDiscoveryByRegion, type: "list<UntaggedDiscoveryRegion>", required: true, constraints: "リージョンごとの方法と状態(BR3.1)" }
      - { name: unsupportedResourceTypes, type: "list<{resourceType, count}>", required: true, default: "[]", constraints: "BR3.3" }
      - { name: complete, type: boolean, required: true, constraints: "failedRegions が空なら true。untaggedDiscovery が partial や unavailable でも true のまま(BR5.3)" }

  - name: UntaggedDiscoveryRegion
    description: 1つのリージョンで、一度もタグが付いていないリソースを探した結果
    owner: Inventory
    attributes:
      - { name: region, type: string, required: true }
      - { name: methods, type: "list<enum>", required: true, allowed: [awsConfig, resourceExplorer, resourceExplorerAggregator], constraints: "使った方法。Config が一部の種類だけを記録しているときは2つ(BR3.1)" }
      - { name: status, type: enum, required: true, allowed: [complete, unavailable, failed] }
      - { name: reason, type: "string | null", required: true, constraints: "unavailable と failed のときの理由。足りない API の名前を含める" }
      - { name: found, type: integer, required: true, min: 0 }

  - name: FailedRegion
    description: 読めなかったリージョン
    owner: Inventory
    attributes:
      - { name: region, type: string, required: true }
      - { name: reason, type: enum, required: true, allowed: [accessDenied, transient, timeout] }
      - { name: operation, type: string, required: false, constraints: "拒否された、または失敗した API の名前(例 tag:GetResources)" }

  - name: KeyVariantGroup
    description: 表記ゆれのまとまり(U1 のまとめ方に略語の辞書を足す)
    owner: Inventory
    attributes:
      - { name: normalizedKey, type: string, required: true, unique: true, constraints: "正規化し、辞書で正式な形に置き換えたキー(BR5.1)" }
      - { name: abbreviationOnly, type: boolean, required: true, default: false, constraints: "辞書を使わなければ書き方が1つだけだったまとまり。印を付け、終了コードの理由にしない(BR5.1)" }

  - name: AbbreviationDictionary
    description: 略語の辞書。scan のまとまりと、U3 の rules suggest が使う
    owner: Inventory
    attributes:
      - { name: entries, type: "map<string,string>", required: true, constraints: "正規化した略語 → 正式な形。env→environment、prod→production、dev→development、stg→staging" }

  - name: MissingRequiredTag
    description: 必須タグの欠落(--rules のとき)
    owner: Inventory
    attributes:
      - { name: key, type: string, required: true, constraints: "ルールファイルの required のキー" }
      - { name: resourceCount, type: integer, required: true, min: 0, constraints: "タグを変更できない種類を除いた件数(BR5.2)" }
      - { name: notTaggableCount, type: integer, required: true, min: 0, constraints: "タグを変更できない種類で、このキーが付いていないものの件数(別に示す)" }
      - { name: lowerBound, type: boolean, required: true, constraints: "一度もタグが付いていないリソースを探せなかったリージョンがあれば true(BR3.2)" }
      - { name: arns, type: "list<string>", required: true, constraints: "JSON の出力(U4)で使う。一覧の形では件数だけを出す" }

  - name: ScanResult
    description: scan の結果のまとめ
    owner: Cli
    attributes:
      - { name: snapshot, type: InventorySnapshot, required: true }
      - { name: variantGroups, type: "list<KeyVariantGroup>", required: true }
      - { name: missingRequired, type: "list<MissingRequiredTag> | null", required: true, constraints: "--rules がなければ null(「調べていない」)" }
      - { name: neverTaggedCount, type: integer, required: true, min: 0, constraints: "一度もタグが付いていないリソースの件数(BR5.5)" }
      - { name: issuesFound, type: boolean, required: true, constraints: "表記ゆれ(辞書だけのまとまりを除く)か欠落が1つでもあれば true" }

  - name: ResourceTypeMapping
    description: AWS Config の種類の形(AWS::EC2::Instance)から ARN の形(ec2:instance)への対応表と、タグの取得の API が対応する種類の一覧(BR2.5、BR3.3)
    owner: Inventory
    attributes:
      - { name: configToArnForm, type: "map<string,string>", required: true, constraints: "ツールの版ごとに更新する" }
      - { name: notTaggableTypes, type: "set<string>", required: true, constraints: "タグを変更できないと分かっている種類(ARN の形)。一覧にない種類はタグを変更できるものとして扱う。ツールの版ごとに見直す" }
```

## まとめ

- **リソース:** グローバルかどうか(`global`)、かつてタグが付いたことがあるか(`everTagged`)、CloudFormation の印(`iacManagedBy`)を足します。
- **収集の結果:** 有効なリージョン、読めなかったリージョン、一度もタグが付いていないリソースをリージョンごとに探した方法と状態(`untaggedDiscoveryByRegion`)と全体の状態、タグを変更できない種類、結果が完全かどうか(`complete`)を持ちます。
- **種類の対応表:** AWS Config の種類の形を ARN の形に変える表と、タグの取得の API が対応する種類の一覧を、ツールが持ちます。
- **表記ゆれ:** まとめるときに略語の辞書(`AbbreviationDictionary`)を使います。この辞書は U3 でも使います。
- **必須タグの欠落:** `--rules` のときだけ作り、`required` のキーごとの件数を持ちます。
