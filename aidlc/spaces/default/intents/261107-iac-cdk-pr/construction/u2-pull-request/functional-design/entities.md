# エンティティ: U2 プルリクエストの作成(残り)

U2 で扱うデータの形です。U1 で作った `internal/pullrequest` の型(契約 C1 の CheckRequest、Plan、PublishInput、PublishOutcome)に、U2 で値を入れる欄と、本文の組み立ての形を足します。出典は、単位の定義(`inception/units-generation/unit-of-work.md` の U2)、契約(`inception/contract-design/contract-summary.md` の X1、X2、C1)、要件(`inception/requirements-analysis/requirements.md` の FR1)、このステージの回答(FD2-Q…、`functional-design-questions.md`)です。

```yaml
entities:
  - name: PullRequestOptions
    description: 利用者が --pr に付けるオプション(契約 X1)
    attributes:
      - {name: branch, type: string, required: false, constraints: "--pr-branch。git のブランチ名として正しいこと(git check-ref-format --branch)"}
      - {name: title, type: string, required: false, constraints: "--pr-title。空でなく、改行を含まず、256 文字まで"}
      - {name: bodyFile, type: path, required: false, constraints: "--pr-body-file。コマンドを動かしたフォルダーからの相対か絶対(FD2-Q3)。読めて、UTF-8 で、1 MiB まで"}
    entity_constraints:
      - どれも --pr が無いときに付けたら入力の誤り(終了コード 1)
      - 入力の誤りは、どのファイルも書き換える前に確かめる

  - name: DefaultBranchName
    description: --pr-branch が無いときのブランチ名
    attributes:
      - {name: value, type: string, required: true, constraints: "tag-refactor/iac-<YYYYMMDD-HHMMSS>Z。日時は UTC(FD2-Q1)"}

  - name: PullRequestBody
    description: プルリクエストの本文(契約 X2)
    attributes:
      - {name: intro, type: string, required: true, constraints: "tag-refactor iac --pr が作ったことを示す 1〜3 行。--lang に従う"}
      - {name: userBody, type: string, required: false, constraints: "--pr-body-file の中身。伏せ字を通さず、そのまま入れる"}
      - {name: changedFiles, type: "list({file, tool})", required: true, constraints: "## Changed files (N) の節。file はリポジトリの根からの相対。tool は CDK, TypeScript、CDK, Python、CloudFormation、Terraform"}
      - {name: toFixByHand, type: "list({file, line, resourceName, key, reasonText})", required: false, constraints: "## To fix by hand (N) の節。0 件なら節ごと省く"}
      - {name: omitted, type: integer, required: true, min: 0, constraints: "上限のために省いた手で直す項目の数(FD2-Q2)"}
    entity_constraints:
      - --pr-body-file が無いとき、本文は intro、changedFiles、toFixByHand の順。あるとき、userBody、changedFiles、toFixByHand の順で、intro は省く
      - tag-refactor が組み立てる部分(intro、changedFiles、toFixByHand)は、組み立て(IacDisplay)の中で、つなぎ合わせる前に伏せ字を通す。userBody は通さない。Publish は本文の全体に伏せ字を通さない
      - userBody は 60,000 文字まで(書き換えの前に確かめる)
      - 本文の全体は 65,536 文字まで

  - name: ManualLine
    description: 手で直す一覧の1行
    attributes:
      - {name: file, type: path, required: true, constraints: "リポジトリの根からの相対(changedFiles とそろえる)。分からなければ空"}
      - {name: line, type: integer, required: false, min: 1}
      - {name: resourceName, type: string, required: true, constraints: "CloudFormation と CDK は論理 ID(CDK はコンストラクトのパスの最後の要素)、Terraform はアドレス、synth できなかったアプリはアプリのフォルダーとリソースの種類"}
      - {name: key, type: string, required: true}
      - {name: reasonText, type: string, required: true, constraints: "表示の文の一覧(カタログ)の固定の文。cdkAppFailed のときだけ道具の名前を足す。外のプログラムの出力は入れない"}
    line_format: |
      file と line がある: - `<file>:<line>` <resourceName>, <key>: <reasonText>
      file だけある:       - `<file>` <resourceName>, <key>: <reasonText>
      file が空:           - <resourceName>, <key>: <reasonText>

  - name: ResumeCommand
    description: 途中で失敗したときに、続きを手で行う命令の1行(契約 C1 resume)
    attributes:
      - {name: shell, type: enum, required: true, allowed: [powershell, sh], constraints: "tag-refactor が動いている OS で決める。Windows は powershell"}
      - {name: text, type: string, required: true, constraints: "1つの命令。本文は --body-file でファイルを渡し、命令には入れない"}
    relationships:
      - {to: PublishOutcome, cardinality: "n to 1", direction: "PublishOutcome -> ResumeCommand(失敗した段階から作成までの命令)"}
```

## まとめ

- 利用者のオプション(`PullRequestOptions`)は、書き換えの前に確かめます。
- 既定のブランチ名は UTC の日時で作ります(`DefaultBranchName`)。
- 本文(`PullRequestBody`)は、変更したファイルの一覧と手で直す一覧(`ManualLine`)から組み立て、上限を超えるときは手で直す一覧の後ろを省きます。
- 途中で失敗したときは、続きの命令(`ResumeCommand`)を、失敗した段階から作成までの分だけ作ります。
