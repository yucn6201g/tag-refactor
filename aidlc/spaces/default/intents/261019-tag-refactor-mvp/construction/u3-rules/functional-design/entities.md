# エンティティ: U3 ルール

U1 のルールの形(`RuleSet` の正しいキー名と別名、警告)に、U3 で足す項目を定めます。ここに書かない項目は U1 のままです。

出典: 契約 C1・C8、`rules.md`、`functional-design-questions.md` Q1〜Q6。

```yaml
entities:
  - name: RuleSet
    description: 読み込んだルールファイル(U1 に項目を足す)
    owner: RuleEngine
    attributes:
      - { name: sourcePath, type: string, required: true }
      - { name: version, type: integer, required: true, allowed: [1] }
      - { name: keyRules, type: "list<KeyRule>", required: true, default: "[]", constraints: "ファイルの順" }
      - { name: requiredTags, type: "list<RequiredTag>", required: true, default: "[]", constraints: "ファイルの順" }
      - { name: removeKeys, type: "list<RemoveKey>", required: true, default: "[]", constraints: "ファイルの順。重複は除く(BR4.1)" }
      - { name: warnings, type: "list<Warning>", required: true, default: "[]" }

  - name: KeyRule
    description: 正しいキー名と、その別名と正しい値(U1 に values を足す)
    owner: RuleEngine
    attributes:
      - { name: canonicalKey, type: string, required: true, constraints: "1〜128文字。aws: で始まらない(BR1.5、BR5.1)" }
      - { name: aliases, type: "list<string>", required: true, default: "[]", constraints: "ほかの KeyRule の canonicalKey・aliases と重ならない(BR2.1)" }
      - { name: values, type: "list<ValueRule>", required: true, default: "[]" }
      - { name: line, type: integer, required: true, min: 1 }

  - name: ValueRule
    description: 正しい値と、その別の書き方
    owner: RuleEngine
    attributes:
      - { name: canonicalValue, type: string, required: true, constraints: "0〜256文字" }
      - { name: spellings, type: "list<string>", required: true, constraints: "同じ KeyRule のほかの ValueRule の canonicalValue・spellings と重ならない(BR2.2)" }
      - { name: line, type: integer, required: true, min: 1 }

  - name: RequiredTag
    description: 必須タグ(U1 では名前だけだったものに、値の決め方を足す)
    owner: RuleEngine
    attributes:
      - { name: key, type: string, required: true, constraints: "別名でも消すキーでもない(BR3.1)" }
      - { name: default, type: "string | null", required: true, default: null }
      - { name: defaultLine, type: "integer | null", required: true, default: null }
      - { name: when, type: "list<RequiredCondition>", required: true, default: "[]", constraints: "ファイルの順に評価する(BR3.2)" }
      - { name: line, type: integer, required: true, min: 1 }

  - name: RequiredCondition
    description: 必須タグの条件ごとの値
    owner: RuleEngine
    attributes:
      - { name: resourceType, type: "string | null", required: true, constraints: "scan と同じ形。service か service:type(BR3.4)" }
      - { name: region, type: "string | null", required: true, constraints: "リージョン名の形(BR3.5)" }
      - { name: tags, type: "map<string,string>", required: true, default: "{}", constraints: "aws: のキーも可(BR5.1)。別名、値の別名、消すキーは不可(BR3.6)。置き換えたあとのタグと比べる(BR3.2)" }
      - { name: value, type: string, required: true, constraints: "0〜256文字" }
      - { name: line, type: integer, required: true, min: 1 }
    constraints:
      - "resourceType、region、tags のうち1つ以上がある(BR3.3)"

  - name: RemoveKey
    description: 消すキー
    owner: RuleEngine
    attributes:
      - { name: key, type: string, required: true, constraints: "正しいキー名でも別名でもない(BR4.1)" }
      - { name: line, type: integer, required: true, min: 1 }

  - name: RequiredValueAnswer
    description: あるリソースの必須タグの値の照会の答え(U4 の変更案が使う)
    owner: RuleEngine
    attributes:
      - { name: key, type: string, required: true }
      - { name: status, type: enum, required: true, allowed: [resolved, unspecified] }
      - { name: value, type: "string | null", required: true, constraints: "status が resolved のとき" }
      - { name: decidedBy, type: enum, required: true, allowed: [when, default, none] }
      - { name: line, type: "integer | null", required: true, constraints: "値を決めた when か default の行" }

  - name: SuggestedRuleFile
    description: rules suggest が出す下書き(契約 C1 の形)
    owner: RuleEngine
    attributes:
      - { name: toolVersion, type: string, required: true }
      - { name: accountId, type: string, required: true }
      - { name: scannedRegionCount, type: integer, required: true, min: 0 }
      - { name: failedRegions, type: "list<string>", required: true, default: "[]", constraints: "空でなければ不完全(BR6.6)" }
      - { name: generatedAt, type: datetime, required: true }
      - { name: keys, type: "list<SuggestedKey>", required: true, constraints: "件数の多い順、同じなら名前の順(BR6.4)" }

  - name: SuggestedKey
    description: 下書きの1つのキーのまとまり
    owner: RuleEngine
    attributes:
      - { name: canonicalKey, type: string, required: true, constraints: "最も多い書き方(BR6.1)" }
      - { name: aliases, type: "list<string>", required: true, default: "[]" }
      - { name: resourceCount, type: integer, required: true, min: 1 }
      - { name: values, type: "list<ValueRule>", required: true, default: "[]", constraints: "書き方が2つ以上のものだけ(BR6.3)" }
      - { name: valuesOmitted, type: "enum | null", required: true, default: null, allowed: [tooManyValues, sizeLimit], constraints: "値の候補を出さなかった理由。異なる値が20個を超えた(BR6.3)か、下書きが 64 KiB を超えた(BR6.7)" }
```

## まとめ

- **ルールファイル(`RuleSet`):** U1 の正しいキー名と別名に、正しい値(`ValueRule`)、必須タグの値の決め方(`default` と `when` の条件)、消すキー(`RemoveKey`)を足して、契約 C1 のすべての項目を持ちます。項目ごとに行番号を持ち、U4 が「どの行で決まったか」を示せるようにします。
- **照会の答え(`RequiredValueAnswer`):** 必須タグの値が決まったか、未指定かと、決めた行を返します。変更案への組み込みは U4 です(Q2)。
- **下書き(`SuggestedRuleFile`):** 収集の結果から作り、見つかったすべてのキーのまとまりと、値の候補を持ちます。
