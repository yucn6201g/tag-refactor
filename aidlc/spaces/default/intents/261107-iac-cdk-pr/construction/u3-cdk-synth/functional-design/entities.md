# エンティティ: U3 CDK の synth と対応づけ(残り)

U3 で扱うデータの形です。U1 で作った `internal/cdk` の型(契約 C2 の App、Options、Synthesis、Failure、Stack と、C6 の cdk.ProgressSink)と、`internal/iac` の `CDKApp`(契約 C7)に、U3 で値を入れる欄と新しい形を足します。出典は、単位の定義(`inception/units-generation/unit-of-work.md` の U3)、契約(`inception/contract-design/contract-summary.md` の X1、C2、C5、C6、C7)、要件(`inception/requirements-analysis/requirements.md` の FR2)、設計の決定(`inception/domain-design/decisions.md` の ADR-004)、このステージの回答(FD3-Q…、`functional-design-questions.md`)です。

```yaml
entities:
  - name: CdkAppSelection
    description: 利用者が --cdk-app で選んだアプリ(契約 X1)
    attributes:
      - {name: values, type: "list(string)", required: false, constraints: "--cdk-app。何回でも付けられる。--dir からの相対。区切りは / と、Windows では \\ も使える"}
    entity_constraints:
      - 各値は、--dir の下のフォルダーを指し(.. で --dir の外に出ない)、そのフォルダーに cdk.json があり、CdkApp の発見で見つかるもの(.git、node_modules、cdk.out、.venv、venv、リンクの下でない)であること。どれか1つでも違えば入力の誤り(終了コード 1)
      - 同じアプリを2回指したときは1回として扱う
      - 値が無いときは、見つかったアプリをすべて対象にする

  - name: SynthTimeout
    description: 1回の cdk synth の時間の上限(契約 X1、C5)
    attributes:
      - {name: value, type: duration, required: true, default: 10m, min: 1s, constraints: "--synth-timeout。Go の時間の書き方(90s、10m、1h30m)。1s より短い、または読めない値は入力の誤り(終了コード 1)"}
    entity_constraints:
      - 作成の記録を取る2回目の synth(TraceSynth)にも、同じ上限を別に置く

  - name: CdkApp
    description: cdk.json のあるフォルダー(契約 C2 の App)。U1 の欄に、Python の値と使う Python を足す
    attributes:
      - {name: dir, type: string, required: true, unique: true, constraints: "--dir からの相対、/ 区切り。--dir そのものは ."}
      - {name: language, type: enum, required: true, allowed: [typescript, python, other]}
      - {name: cdkMajor, type: integer, required: true, allowed: [0, 1, 2], constraints: "0 は分からない"}
      - {name: command, type: string, required: false, constraints: "cdk.json の app の命令"}
      - {name: interpreter, type: string, required: false, constraints: "python のとき、synth で使う Python。アプリのフォルダーの .venv にあればそれ(app の命令の最初の語が python、python3、py なら、その絶対パスに置き換えた命令を cdk synth の --app で渡す)、無ければ PATH の app の命令の最初の語(BR2.2)"}
    entity_constraints:
      - フォルダーの名前の順に並べ、その順に1つずつ synth する(FD3-Q1)

  - name: SynthRun
    description: 1つのアプリに対する1回の cdk synth
    attributes:
      - {name: app, type: reference, references: CdkApp, required: true}
      - {name: kind, type: enum, required: true, allowed: [normal, trace], constraints: "trace は作成の記録を取る2回目(Options.WithTrace、cdk synth --debug)"}
      - {name: outputDir, type: path, required: true, constraints: "一時的なフォルダー。成功、失敗、時間切れ、中断のどれでも消す"}
      - {name: timeout, type: reference, references: SynthTimeout, required: true}
      - {name: elapsed, type: duration, required: true}
      - {name: failure, type: reference, references: SynthFailure, required: false}

  - name: SynthFailure
    description: synth できなかった理由(契約 C2 の Failure)
    attributes:
      - {name: reason, type: enum, required: true, allowed: [toolMissing, toolTooOld, needsAWS, timeout, cdkV1, unknownVersion, unsupportedLanguage, synthError]}
      - {name: tool, type: string, required: false, constraints: "node、cdk、python のどれか。toolMissing のときは見つからない道具、toolTooOld のときは python、版の確かめの時間切れは python、ほかの理由で synth を動かしたときは cdk。python のアプリで app の命令の最初の語が python 系(python、python3、py)でなく見つからないときは、その語の最後の要素(toolName。例 pipenv)"}
      - {name: detail, type: string, required: false, constraints: "外のプログラムの標準エラーから残した行(伏せ字を通したもの)。--debug のときだけ端末に出す。プルリクエストの本文には入れない"}

  - name: NeedsAWSPattern
    description: synth の標準エラーに出たら needsAWS とみなす、決まった文(契約 C2 needs_aws)
    attributes:
      - {name: text, type: string, required: true, unique: true, constraints: "大文字と小文字を区別して、部分一致で比べる。一覧は rules.md の BR2.6 にあり、テストで固定する"}

  - name: TraceSite
    description: 作成の記録から読んだ、コンストラクトを作ったソースの箇所(契約 C2 の TraceSites、ADR-004)
    attributes:
      - {name: constructPath, type: string, required: true, constraints: "synth のリソースの aws:cdk:path"}
      - {name: file, type: string, required: true, constraints: "アプリのフォルダーからの相対、/ 区切り。アプリのフォルダーの外、node_modules、.venv、venv、site-packages の下のファイルは使わない"}
      - {name: line, type: integer, required: true, min: 1}
    entity_constraints:
      - 作成の箇所がソースで見つからなかったコンストラクトのパスについてだけ作る

  - name: AppProgress
    description: 進み具合の表示の、アプリごとの状態(契約 C6)
    attributes:
      - {name: app, type: string, required: true}
      - {name: state, type: enum, required: true, allowed: [waiting, running, done, failed, skipped]}
      - {name: startedAt, type: timestamp, required: false, constraints: "running になった時刻。経過時間の元"}
      - {name: elapsed, type: duration, required: false, constraints: "done と failed のときの時間"}
      - {name: reason, type: string, required: false, constraints: "failed と skipped のときの理由の種類(SynthFailure.reason、または helper)。detail は持たない"}
      - {name: lastHeartbeat, type: timestamp, required: false, constraints: "端末でないとき、最後に「実行中」の行を足した時刻(FD3-Q2)。時計は display.Printer が持つ(BR5.4)"}

  - name: CdkAppEntry
    description: 結果に残すアプリごとの状態(契約 C7 の Result.CDKApps)
    attributes:
      - {name: dir, type: string, required: true}
      - {name: language, type: enum, required: true, allowed: [typescript, python, other]}
      - {name: status, type: enum, required: true, allowed: [done, failed, skipped]}
      - {name: elapsed, type: duration, required: false, constraints: "1回目と2回目の synth の合計"}
      - {name: failureReason, type: string, required: false, constraints: "SynthFailure.reason、または helper"}
      - {name: tool, type: string, required: false, constraints: "失敗した道具(node、cdk、python、typescript、helper)"}
    entity_constraints:
      - --cdk-app で選ばなかったアプリは入れない
      - failed か skipped が1つでもあれば、終了コードは 2 以上(契約 C7 exit_code_rule)

  - name: UnattributedManualReason
    description: どのアプリのものか分からないリソースの、手で直す一覧の理由(FD3-Q3)
    attributes:
      - {name: reason, type: enum, required: true, allowed: [cdkAppFailed, cdkV1, cdkUnsupportedLanguage]}
      - {name: tool, type: string, required: false, constraints: "reason が cdkAppFailed で、synth に失敗したアプリ(補助のプログラムの失敗は数えない)がちょうど1つのときだけ、その道具の名前(SynthFailure.tool)"}
    entity_constraints:
      - 理由の優先は cdkAppFailed、cdkV1、cdkUnsupportedLanguage の順
      - どのアプリかは項目に書かない(アプリの状態の一覧で分かる)

relationships:
  - {from: CdkAppSelection, to: CdkApp, cardinality: "0..* : 0..*", direction: "選んだアプリだけを対象にする"}
  - {from: CdkApp, to: SynthRun, cardinality: "1 : 0..2", direction: "normal を1回、必要なら trace を1回"}
  - {from: SynthRun, to: SynthTimeout, cardinality: "* : 1"}
  - {from: SynthRun, to: SynthFailure, cardinality: "1 : 0..1"}
  - {from: SynthRun, to: TraceSite, cardinality: "1 : 0..*", direction: "trace のときだけ"}
  - {from: CdkApp, to: AppProgress, cardinality: "1 : 1"}
  - {from: CdkApp, to: CdkAppEntry, cardinality: "1 : 1"}
  - {from: CdkAppEntry, to: UnattributedManualReason, cardinality: "* : 0..1", direction: "failed と skipped のアプリの全体から1つ決める"}
```

## まとめ

| エンティティ | 一言で | U1 との違い |
|---|---|---|
| CdkAppSelection | `--cdk-app` で選んだアプリ | 新しい |
| SynthTimeout | 1回の synth の上限 | U1 は 10 分の決め打ち。`--synth-timeout` で変えられるようにする |
| CdkApp | `cdk.json` のあるフォルダー | Python の版の判定と、使う Python(`.venv`)を足す |
| SynthRun | 1回の synth | 作成の記録を取る2回目(trace)を足す |
| SynthFailure | synth できなかった理由 | 道具に `python` を足す |
| NeedsAWSPattern | AWS が要ることを示す決まった文 | U1 の4つから増やす |
| TraceSite | 作成の記録から読んだソースの箇所 | 新しい(ADR-004) |
| AppProgress | 進み具合の表示の状態 | 書き直しの表と、端末でないときの1分ごとの行(FD3-Q2) |
| CdkAppEntry | 結果に残すアプリの状態 | 複数のアプリ。U1 の「U1 では扱わない」の外し方をやめる |
| UnattributedManualReason | どのアプリか分からないリソースの理由 | 新しい(FD3-Q3) |
