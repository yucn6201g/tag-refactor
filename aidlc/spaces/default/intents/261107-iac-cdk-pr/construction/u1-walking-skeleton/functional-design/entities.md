# エンティティ: U1 最小の一本

U1 で扱うデータの形です。出典は、単位の定義(`inception/units-generation/unit-of-work.md` の U1)、部品(`inception/domain-design/components.md`)、契約(`inception/contract-design/contract-summary.md` の C1〜C7)、要件(`inception/requirements-analysis/requirements.md`)、このステージの質問の回答(FD-Q…、`functional-design-questions.md`)です。

U1 は最小の一本なので、各エンティティは契約の形を持ちますが、U1 で値が入るのは一部です。U1 で使わない値の扱いは、それぞれの `u1_scope` に書きます。

```yaml
entities:
  - name: CdkApp
    description: cdk.json のあるフォルダー1つ。U1 では --dir の下の1つだけを扱う(契約 C2 の App)
    attributes:
      - {name: dir, type: path, required: true, constraints: "--dir からの相対、/ 区切り。--dir の外は不可"}
      - {name: language, type: enum, required: true, allowed: [typescript, python, other]}
      - {name: cdkMajor, type: integer, required: true, allowed: [0, 1, 2]}
    u1_scope: language が typescript で cdkMajor が 2 のアプリだけを synth する。それ以外は synth せず、手で直す一覧に出す(理由の文は U6)
    relationships:
      - {to: Synthesis, cardinality: "1 to 0..1", direction: "CdkApp -> Synthesis"}
      - {to: CdkAnalysis, cardinality: "1 to 0..1", direction: "CdkApp -> CdkAnalysis"}

  - name: IsolatedEnvironment
    description: cdk synth と補助のプログラムに渡す環境(契約 C5)
    attributes:
      - {name: variables, type: map<string,string>, required: true, constraints: "AWS_ で始まる変数と GitHub のトークンの変数を含まない"}
      - {name: emptyCredentialsFile, type: path, required: true, constraints: "空の一時ファイル。AWS_SHARED_CREDENTIALS_FILE に入れる"}
      - {name: emptyConfigFile, type: path, required: true, constraints: "空の一時ファイル。AWS_CONFIG_FILE に入れる"}
    entity_constraints:
      - AWS_EC2_METADATA_DISABLED は "true"、CDK_DISABLE_VERSION_CHECK は "1"
      - 一時ファイルは、実行が成功しても失敗しても消す

  - name: Synthesis
    description: 1つのアプリの synth の結果(契約 C2)
    attributes:
      - {name: app, type: ref(CdkApp), required: true}
      - {name: stacks, type: list(SynthesizedStack), required: true}
      - {name: elapsed, type: duration, required: true}
      - {name: failure, type: SynthFailure, required: false}
    entity_constraints:
      - failure があるとき stacks は空
      - synth の出力は一時的なフォルダーに置き、読んだあとで消す。利用者の cdk.out は変えない

  - name: SynthFailure
    description: synth できなかった理由(契約 C2 の Failure)
    attributes:
      - {name: reason, type: enum, required: true, allowed: [toolMissing, needsAWS, timeout, cdkV1, unknownVersion, unsupportedLanguage, synthError]}
      - {name: tool, type: string, required: false}
      - {name: detail, type: string, required: false, constraints: "外のプログラムの出力から作る。--debug の端末表示にだけ出し、プルリクエストの本文には入れない"}

  - name: SynthesizedStack
    description: synth で得たスタック1つ
    attributes:
      - {name: name, type: string, required: true, constraints: "実際にデプロイされるスタック名(manifest の stackName。無ければ成果物の ID)"}
      - {name: account, type: string, required: true, constraints: "決まっていなければ空"}
      - {name: region, type: string, required: true, constraints: "決まっていなければ空"}
      - {name: resources, type: list(SynthesizedResource), required: true}

  - name: SynthesizedResource
    description: synth のテンプレートのリソース1つ
    attributes:
      - {name: logicalId, type: string, required: true}
      - {name: type, type: string, required: true}
      - {name: constructPath, type: string, required: true, constraints: "テンプレートの Metadata の aws:cdk:path"}
      - {name: tags, type: map<string,string>, required: true, constraints: "Properties.Tags から読んだもの。無ければ空"}

  - name: CdkAnalysis
    description: 補助のプログラムが1つのアプリのソースを読んだ結果(契約 C3、C4)
    attributes:
      - {name: tagSites, type: list(CdkTagSite), required: true}
      - {name: constructSites, type: list(ConstructSite), required: true}
      - {name: unsupported, type: list(UnsupportedSite), required: true}
    u1_scope: U1 の補助のプログラムが見つけるのは Tags.of(x).add('キー', '値') の、キーと値が文字列そのままの形だけ。それ以外の形は unsupported に入る

  - name: CdkTagSite
    description: ソースの中の、タグを付ける1か所(契約 C3 の TagSite)
    attributes:
      - {name: useFile, type: path, required: true}
      - {name: useLine, type: integer, required: true, min: 1}
      - {name: op, type: enum, required: true, allowed: [add, remove, stackTags, l1Tags, other]}
      - {name: scopeConstructPath, type: string, required: true}
      - {name: keyEdit, type: Edit, required: true}
      - {name: valueEdit, type: Edit, required: false, constraints: "op が remove のときだけ無い"}
      - {name: removeEdit, type: Edit, required: true}
    u1_scope: op は add だけ。keyEdit と valueEdit の shape が literal のものだけを書き換え、expression は cdkExpression、sameFileConst などは cdkUnsupportedShape にする

  - name: Edit
    description: ソースの中の書き換えの範囲(契約 C3)
    attributes:
      - {name: file, type: path, required: true}
      - {name: start, type: integer, required: true, min: 0, constraints: "UTF-8 のバイトの位置"}
      - {name: end, type: integer, required: true, constraints: "start 以上。引用符を含む"}
      - {name: shape, type: enum, required: true, allowed: [literal, sameFileConst, sameFileMap, crossFileArg, expression]}
      - {name: text, type: string, required: true, constraints: "引用符を除いた今の文字列"}
      - {name: sharedWith, type: list(string), required: true}

  - name: ConstructSite
    description: コンストラクトを作っているソースの箇所(契約 C3)
    attributes:
      - {name: constructPath, type: string, required: true}
      - {name: file, type: path, required: true}
      - {name: line, type: integer, required: true, min: 1}
      - {name: constructId, type: string, required: true}
      - {name: from, type: enum, required: true, allowed: [source, trace]}
    u1_scope: from は source だけ(作成の記録での補いは U3)

  - name: CdkTarget
    description: 変更案のリソースのうち、CDK のものと決まったもの。今の iac の対象(target)に CDK の欄を足したもの
    attributes:
      - {name: arn, type: string, required: true}
      - {name: changes, type: list(PlanChange), required: true}
      - {name: app, type: ref(CdkApp), required: true}
      - {name: stackName, type: string, required: true}
      - {name: logicalId, type: string, required: true}
      - {name: constructPath, type: string, required: true}
      - {name: account, type: string, required: true, constraints: "変更案の Plan.AccountID。空なら照合に使わない"}
      - {name: region, type: string, required: true, constraints: "変更案のリソースの Region。空なら照合に使わない"}
      - {name: global, type: boolean, required: true, constraints: "変更案のリソースの Global。真ならリージョンを照合に使わない"}
    relationships:
      - {to: SynthesizedResource, cardinality: "1 to 1", direction: "CdkTarget -> SynthesizedResource"}
      - {to: CdkTagSite, cardinality: "1 to 0..n", direction: "CdkTarget -> CdkTagSite(範囲がこのリソースを含むもの)"}

  - name: IacResultAdditions
    description: 今の iac の結果(internal/iac の Result、FileResult、Tool、ManualReason)と対象(target)に U1 で足すもの(契約 C7)
    attributes:
      - {name: Tool.ToolCDK, type: enum value, required: true, constraints: "3つ目の IaC。FileResult.Tool に入る"}
      - {name: FileResult.Language, type: enum, required: false, allowed: [typescript, python], constraints: "Tool が ToolCDK のときだけ"}
      - {name: Result.CDKApps, type: "list({dir, language, status: done | failed | skipped, elapsed, failureReason})", required: true}
      - {name: Result.WrittenFiles, type: list(path), required: true, constraints: "Files のうち Written が真のもの。--dir からの相対"}
      - {name: ManualReason (U1 で足す), type: enum values, required: true, allowed: [cdkAppFailed, cdkUnsupportedLanguage, cdkV1, cdkNoSource, cdkScopeTag, cdkSharedValue, cdkAmbiguous, cdkExpression, cdkUnsupportedShape]}
      - {name: target.app、constructPath、region、global, type: fields, required: false, constraints: "Tool が ToolCDK の対象のとき。account は実行の中で1つ(変更案の AccountID)"}
    entity_constraints:
      - Result.ExitCode は、今の判断に加え、CDKApps に failed か skipped があれば 2 を返す(契約 C7 exit_code_rule)
      - 理由の文(英語と日本語)は、U1 では最小の1行ずつを足し、U6 で整える

  - name: PullRequestPlan
    description: 書き換えの前の確かめで決まった、プルリクエストの作り方(契約 C1 の Plan)
    attributes:
      - {name: remote, type: string, required: true}
      - {name: remoteUrl, type: string, required: true}
      - {name: host, type: string, required: true, constraints: "GitHub のホスト"}
      - {name: base, type: string, required: true, constraints: "実行したときのブランチ"}
      - {name: branch, type: string, required: true, constraints: "tag-refactor/iac-<YYYYMMDD-HHMMSS> の既定の形"}
      - {name: title, type: string, required: true, constraints: "1行、256 文字まで"}
      - {name: userBody, type: string, required: false}
      - {name: repoRoot, type: path, required: true}
      - {name: prefix, type: string, required: true}
      - {name: gitDir, type: path, required: true}
    u1_scope: --pr-branch、--pr-title、--pr-body-file は U2。U1 は既定の値だけを使う

  - name: PublishInput
    description: 書き換えのあとに作成に渡すもの(契約 C1)
    attributes:
      - {name: files, type: list(path), required: true, min: 1}
      - {name: title, type: string, required: true}
      - {name: body, type: string, required: true}

  - name: PublishOutcome
    description: プルリクエストの作成の結果(契約 C1)
    attributes:
      - {name: completedSteps, type: list(enum), required: true, allowed: [branch, commit, push, create]}
      - {name: failedStep, type: enum, required: true, allowed: [branch, commit, push, create, ""]}
      - {name: url, type: string, required: false}
      - {name: bodyFile, type: path, required: false}
      - {name: resumeCommands, type: list(string), required: false}
    u1_scope: 途中の失敗の示し方と続きの命令の全体は U2。U1 は失敗した段階と原因を示して終了コード 1 にする
```

## まとめ

- CDK の側は、アプリ(`CdkApp`)を、認証情報を遮断した環境(`IsolatedEnvironment`)で synth した結果(`Synthesis`)と、補助のプログラムがソースを読んだ結果(`CdkAnalysis`)の2つで成り立ちます。
- 変更案のリソースは、スタック名と論理 ID で `SynthesizedResource` に結び付き、`CdkTarget` になります。`CdkTarget` は、コンストラクトのパスを通して、ソースのタグの箇所(`CdkTagSite`)にたどり着きます。
- プルリクエストの側は、書き換えの前の `PullRequestPlan`、書き換えのあとの `PublishInput`、結果の `PublishOutcome` の3つです。
- U1 で値が入るのは、TypeScript の CDK v2 のアプリ1つ、文字列そのままの `Tags.of(x).add`、既定のブランチ名とタイトルだけです。残りは後の単位で埋めます。
