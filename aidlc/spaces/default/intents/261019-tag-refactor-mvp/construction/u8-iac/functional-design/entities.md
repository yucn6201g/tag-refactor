# エンティティ: U8 IaC への反映

`iac` が扱うデータの形です。変更案(契約 C2。`ResourcePlan` と `Change`)の形は U4 で決めたとおりで、ここでは `iac` の中で使う形を定めます。`iac` は AWS を呼ばないので(Q8)、アカウントや認証情報の形は使いません。

出典: 契約 C2・C3・C5・C12、`rules.md`、`functional-design-questions.md` Q1〜Q9、要件 FR8.1・FR8.2、ストーリー US7.1(AC7.1.1、AC7.1.2)、U4 の `entities.md`(Plan、ResourcePlan、Change)。

```yaml
entities:
  - name: IacRequest
    description: iac の1回の実行の指定(コマンドの入口で作る)
    owner: Cli
    attributes:
      - { name: planPath, type: string, required: true, constraints: "--plan。plan --out が書いた変更案の JSON(契約 C2)。版が読めなければ止める(BR1.1)" }
      - { name: dir, type: string, required: true, constraints: "--dir。IaC のソースのフォルダー。存在するフォルダーでなければ使い方の誤り(BR1.1)" }
      - { name: tfStatePath, type: string, required: false, constraints: "--tf-state。terraform show -json の出力。なければ Terraform への反映はしない(BR2.3)" }
      - { name: write, type: boolean, required: true, default: false, constraints: "--write。なければファイルを書き換えず差分だけを示す(BR5.1)" }
      - { name: lang, type: enum, required: false, allowed: [en, ja], constraints: "--lang。全体のオプション(U4 のまま)" }
      - { name: debug, type: boolean, required: true, default: false, constraints: "--debug。全体のオプション(U1 のまま)。解析できず外したファイルの名前、確かめの失敗の詳細を示す(BR3.1、BR4.6)" }

  - name: IacTarget
    description: 変更案のリソースのうち、変更が1つ以上あるものの、iac の中での作業の状態
    owner: IacWriter
    attributes:
      - { name: arn, type: string, required: true, unique: true }
      - { name: resourceType, type: string, required: true, constraints: "変更案の resourceType(例: sqs:queue)" }
      - { name: beforeTags, type: "map<string,string>", required: true, constraints: "変更案の beforeTags(aws: を含む)" }
      - { name: changes, type: "list<Change>", required: true, constraints: "変更案の changes(契約 C2。4種類)" }
      - { name: tool, type: enum, required: true, allowed: [cloudformation, terraform, none], constraints: "BR2.1〜BR2.3 で決める。none は IaC で管理されていない(見つからない)" }
      - { name: stackName, type: string, required: false, constraints: "tool が cloudformation のとき。beforeTags の aws:cloudformation:stack-name" }
      - { name: logicalId, type: string, required: false, constraints: "tool が cloudformation のとき。beforeTags の aws:cloudformation:logical-id" }
      - { name: address, type: string, required: false, constraints: "tool が terraform のとき。state の resource のアドレス(例: aws_sqs_queue.orders)。values.arn が arn と完全に同じもの(BR2.3)" }
      - { name: location, type: SourceLocation, required: false, constraints: "BR3.1〜BR3.3 で決めた1つの場所。決められなければ空" }
      - { name: outcome, type: enum, required: true, allowed: [pending, reflected, upToDate, manual, unmanaged], constraints: "状態 S1(functional-spec.md)。upToDate はすべての変更が反映済み(BR4.3)" }

  - name: SourceLocation
    description: リソースを定義している、IaC のソースの中の1か所
    owner: IacWriter
    attributes:
      - { name: file, type: string, required: true, constraints: "--dir からの相対パス(/ で区切る)" }
      - { name: format, type: enum, required: true, allowed: [cfnYaml, cfnJson, terraform] }
      - { name: line, type: integer, required: true, constraints: "リソースの定義の始まりの行(1から)" }
      - { name: tagSite, type: TagSite, required: false, constraints: "タグの書き場所。リソースにタグの書き場所がなければ空(BR4.5)" }

  - name: TagSite
    description: 1つのリソースの、タグを書いている場所と、そこに書いてあるタグ
    owner: IacWriter
    attributes:
      - { name: shape, type: enum, required: true, allowed: [cfnKeyValueList, cfnMap, tfMap], constraints: "CloudFormation の Tags の Key と Value の並び、CloudFormation の Tags のキーと値の対応、Terraform の tags = { ... }。ほかの形は書き換えない(BR4.1)" }
      - { name: entries, type: "list<TagEntry>", required: true, constraints: "書いてある順" }
      - { name: literal, type: boolean, required: true, constraints: "タグの書き場所そのものが文字列の並び(式、変数、YAML のアンカー・別名・マージのキーでない)か(BR4.1)" }
      - { name: multiline, type: boolean, required: true, constraints: "要素がそれぞれ別の行に書いてあるか。false(1行にまとめた書き方)なら削除と追加をしない(BR4.4、BR4.5)" }
      - { name: extraFields, type: boolean, required: true, default: false, constraints: "cfnKeyValueList のどれかの要素が Key と Value 以外の項目を持つか。true なら追加をしない(BR4.5)" }

  - name: TagEntry
    description: タグの書き場所の1つのタグ
    owner: IacWriter
    attributes:
      - { name: key, type: string, required: true }
      - { name: value, type: string, required: false, constraints: "literal が true のときだけ。書いてある文字列(引用符を外し、エスケープを戻したもの。YAML の型には読み替えない。BR4.2)" }
      - { name: literal, type: boolean, required: true, constraints: "キーと値の両方が1行の文字列で直接書いてあるか(式、参照、変数、補間、ブロックスカラー、複数行を含まないか)" }
      - { name: quoting, type: enum, required: false, allowed: [plain, single, double], constraints: "キーと値の引用符の種類。差し替えで保つ(BR4.4)" }
      - { name: line, type: integer, required: true }

  - name: Edit
    description: 1つのファイルへの1つの書き換え(元のテキストの範囲の差し替え)。1つのリソースの変更をメモリの上で当てたあと、元の並びと比べて作るので、同じファイルの Edit の範囲は重ならない(BR4.4)
    owner: IacWriter
    attributes:
      - { name: file, type: string, required: true }
      - { name: start, type: integer, required: true, constraints: "差し替えの始まり(ファイルの先頭からのバイト位置)" }
      - { name: end, type: integer, required: true, constraints: "差し替えの終わり。start と同じなら挿入" }
      - { name: text, type: string, required: true, constraints: "差し替える文字列。空なら削除" }
      - { name: changeId, type: string, required: true, constraints: "もとになった変更案の changeId" }

  - name: ManualItem
    description: 反映できず、人が手で直すもの
    owner: IacWriter
    attributes:
      - { name: arn, type: string, required: true }
      - { name: changeId, type: string, required: false, constraints: "変更ごとの理由のとき。リソース全体の理由なら空" }
      - { name: key, type: string, required: false }
      - { name: file, type: string, required: false }
      - { name: line, type: integer, required: false }
      - { name: reason, type: enum, required: false, allowed: [templateNotFound, blockNotFound, ambiguous, inModule, multipleInstances, notLiteral, tagsNotLiteral, unsupportedTagShape, noTagSite, keyNotInSource, valueDiffers, keyAlreadyExists, unreadableFile], constraints: "BR2〜BR4 で決める。意味は rules.md の「手で直す理由」の表。手で直す一覧では必ずある。IacResult.assumedRemovals の項目では空" }
      - { name: detail, type: string, required: false, constraints: "場所の候補のファイル名など。値や認証情報は入れない" }

  - name: FileResult
    description: 1つのファイルの反映の結果
    owner: IacWriter
    attributes:
      - { name: file, type: string, required: true }
      - { name: reflected, type: integer, required: true, constraints: "反映した変更の数" }
      - { name: diff, type: string, required: true, constraints: "unified diff(元のファイルと書き換えたあと)" }
      - { name: written, type: boolean, required: true, constraints: "--write でファイルを書き換えたか" }

  - name: IacResult
    description: iac の1回の実行の結果
    owner: IacWriter
    attributes:
      - { name: files, type: "list<FileResult>", required: true, constraints: "ファイル名の順" }
      - { name: manual, type: "list<ManualItem>", required: true, constraints: "ARN、changeId の順" }
      - { name: reflectedChanges, type: integer, required: true }
      - { name: alreadyReflectedChanges, type: integer, required: true, constraints: "ソースがすでに変更の後の状態だった変更の数(BR4.3)" }
      - { name: assumedRemovals, type: "list<ManualItem>", required: true, constraints: "ソースにないため反映済みとみなした削除(removeKey と mergesIntoExisting の renameKey。reason は空)。手で直す一覧とは別に示し、終了コードに影響しない(BR4.3、BR6.1)" }
      - { name: unmanagedResources, type: integer, required: true, constraints: "tool が none のリソースの数(BR2.3)" }
      - { name: exitCode, type: enum, required: true, allowed: [0, 1, 2], constraints: "BR6.2。結果を作るのは 0 と 2 のときと、--write の確かめか書き出しに失敗した 1 のとき。入力の誤りの 1 では結果を作らない" }
```

## まとめ

| エンティティ | 役割 | 作るところ |
|---|---|---|
| IacRequest | 1回の実行の指定 | Cli |
| IacTarget | 変更のあるリソースごとの作業の状態 | IacWriter |
| SourceLocation | リソースの定義の場所 | IacWriter |
| TagSite、TagEntry | タグの書き場所と、そこのタグ | IacWriter(ファイルの読み取り) |
| Edit | テキストの範囲の差し替え | IacWriter |
| ManualItem | 手で直すもの | IacWriter |
| FileResult、IacResult | 結果 | IacWriter |

- `iac` は変更案を書き換えません。AWS も、記録(契約 C4)も、印(契約 C10)も使いません。
- ファイルの中身(タグの値を含む)は、結果の差分にだけ出ます。差分は利用者のファイルの内容なので、そのまま出します。認証情報はどこにも出しません(全体の決まり)。
