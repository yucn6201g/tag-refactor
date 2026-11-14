# ビジネスルール: U1 最小の一本

U1 の決まりです。出典は、要件(`inception/requirements-analysis/requirements.md` の FR と NFR)、契約(`inception/contract-design/contract-summary.md`)、このステージの質問の回答(FD-Q…、`functional-design-questions.md`)です。

```yaml
rules:
  # 1. アプリの発見と synth
  - id: BR1.1
    statement: CDK のアプリは、--dir の下の cdk.json のあるフォルダーとして見つける
    category: policy
    applies_to: CdkApp
    trigger: iac の実行で、変更案に aws:cloudformation:stack-name のタグのリソースがあるとき
    logic: IF 変更案に stack-name のタグのリソースが1つも無い THEN 探さず synth もしない(今の iac と同じ動き)。ELSE --dir の下を探し、.git、node_modules、cdk.out、シンボリックリンク、ジャンクションには入らない
    violation: なし(見つからなければ今までどおり CloudFormation のテンプレートを探す)
    source: FR2.1、NFR5、契約 C7 synth_rule
  - id: BR1.2
    statement: アプリの言語と CDK の版を、cdk.json の app の命令と依存の宣言から決める
    category: calculation
    applies_to: CdkApp
    trigger: アプリを見つけたとき
    logic: IF app の命令が node、npx、ts-node、tsx で始まるか .ts や .js を名指す THEN typescript。版は、アプリのフォルダーから --dir までさかのぼり、最初に見つかった package.json の dependencies か devDependencies で決める。aws-cdk-lib があれば 2、@aws-cdk/core があれば 1、どこにも無ければ 0
    violation: typescript 以外は unsupportedLanguage、版が 1 は cdkV1、版が 0 は unknownVersion で外す(synth しない)。そのアプリのリソースの扱いは BR2.3
    source: 契約 C2 detection、FR2.10
  - id: BR1.3
    statement: cdk synth には、AWS と GitHub の認証情報を渡さない
    category: policy
    applies_to: IsolatedEnvironment
    trigger: cdk synth と補助のプログラムを動かすとき
    logic: 親の環境から AWS_ で始まる変数と GH_TOKEN、GITHUB_TOKEN、GH_ENTERPRISE_TOKEN、GITHUB_ENTERPRISE_TOKEN を外す。名前は大文字と小文字を区別せずに比べる(Windows では aws_profile も同じ変数のため)。AWS_SHARED_CREDENTIALS_FILE と AWS_CONFIG_FILE を空の一時ファイルに向け、AWS_EC2_METADATA_DISABLED=true、CDK_DISABLE_VERSION_CHECK=1、CDK_DISABLE_CLI_TELEMETRY=true を足す。PATH と HOME/USERPROFILE などは残す
    violation: 決まりに合わない環境では起動しない(作る関数のテストで、外した変数が無く足した値があることを確かめる)
    residual_risk: |
      synth は利用者のアプリのコードを動かすので、そのコードがホームの下のファイル(gh の hosts.yml、決め打ちのパスの AWS の設定など)を自分で読むことは止められない。
      これは利用者が自分で cdk synth を動かすときと同じ信頼の範囲で、tag-refactor が認証情報を足すことはない。README の iac の節に、この範囲を書く(U2 で README を直すときに含める)。
    source: FR2.4、NFR2、契約 C5
  - id: BR1.4
    statement: synth の出力は一時的なフォルダーに置き、時間の上限を置く
    category: constraint
    applies_to: Synthesis
    trigger: cdk synth を動かすとき
    logic: |
      呼び出す実行ファイルは、アプリの node_modules/.bin の cdk があればそれ、無ければ PATH の cdk(Windows では cdk.cmd も探す)。
      引数は synth --quiet --no-notices --output <一時的なフォルダー>。作業のフォルダーはアプリのフォルダー。
      標準出力は読み捨て、標準エラーは失敗の分類(BR1.5)と --debug の表示にだけ使う。
      出力先の一時的なフォルダーは、成功、失敗、中断のどれでも消す。上限は 10 分で、超えたら子のプロセスを止めて timeout の失敗にする
    violation: 失敗のときはそのアプリのリソースの扱いを BR2.3 に従う。利用者の cdk.out は変えない
    source: FR2.5、FR2.6、NFR3、契約 C2
  - id: BR1.5
    statement: synth が失敗した理由を、決まった種類に分ける
    category: calculation
    applies_to: SynthFailure
    trigger: synth が 0 以外で終わった、起動できなかった、時間切れのとき
    logic: |
      IF cdk か node が見つからない THEN toolMissing(Tool は見つからない道具の名前)。IF 時間切れ THEN timeout。
      IF 標準エラーに AWS の認証情報や問い合わせを求める既知の文があれば needsAWS。U1 の既知の文は次の4つで、U3 で増やす:
      "Unable to resolve AWS account to use"、"Need to perform AWS calls"、"Could not load credentials"、"Unable to locate credentials"。
      ELSE synthError
    violation: |
      分類だけ。外のプログラムの出力は --debug のときだけ端末に出す。
      needsAWS のときは、対処として「一度 AWS の認証情報のある端末で cdk synth を動かし、cdk.context.json に値を保存してから、もう一度実行する」を示す(U1 の最小の文。U6 で整える)
    source: FR2.4、FR2.8、AC2.1.3、契約 C2 needs_aws
  - id: BR1.6
    statement: 補助のプログラムの失敗を、アプリの状態と手で直す一覧の理由に対応させる
    category: calculation
    applies_to: CdkAnalysis
    trigger: 補助のプログラムが失敗したとき
    logic: |
      上限は呼び出しごとに 2 分。
      - node が無い、typescript がアプリから見つからない(noTypeScript)、時間切れ、内部の失敗: そのアプリを failed にし、Tool に node、typescript、helper のどれかを入れる
      - 読めないファイル(parseError): そのファイルだけを今の SkippedFiles に入れ、ほかのファイルは使う
      - 結果の版が違う: 埋め込みの不具合なので、書き換えの前に終了コード 1 で止める
    violation: failed のアプリのリソースの扱いは BR2.3
    source: 契約 C7 helper_failure_to_manual
  - id: BR1.7
    statement: synth の間、アプリごとの状態と経過時間を標準エラーに文字で示す
    category: policy
    applies_to: CdkApp
    trigger: アプリを見つけてから synth と補助のプログラムが終わるまで
    logic: 契約 C6 の cdk.ProgressSink で、待ち(AppsWaiting)、実行中(AppStarted)、済み(AppDone、経過時間つき)、失敗(AppFailed、経過時間と理由の種類つき)、外した(AppSkipped)を知らせ、表示が1行ずつ標準エラーに出す。端末でないときの表示の工夫は U3
    violation: なし
    source: FR2.7、AC2.1.2、契約 C6

  # 2. リソースとソースの対応づけ
  - id: BR2.1
    statement: 変更案のリソースを、スタック名と論理 ID とアカウントとリージョンで synth のリソースに結び付ける
    category: calculation
    applies_to: CdkTarget
    trigger: synth が成功したアプリがあるとき
    logic: 変更案のタグ aws:cloudformation:stack-name と aws:cloudformation:logical-id、変更案のアカウント、リソースのリージョンで照合する。スタックのアカウントやリージョンが空なら、その軸は照合に使わない。リソースが Global かリージョンが空なら、リージョンを照合に使わない
    violation: 候補が2つ以上なら書き換えず手で直す一覧に出す(cdkAmbiguous)。候補が0なら、今までどおり CloudFormation のテンプレートを探す
    source: FR2.11、契約 C7 match_rule
  - id: BR2.2
    statement: 補助のプログラムは、U1 で扱う形について、コンストラクトのパスと Tags.of の範囲をソースから組み立てる
    category: calculation
    applies_to: ConstructSite、CdkTagSite
    trigger: 補助のプログラムがアプリのソースを読むとき
    logic: |
      U1 で扱う形は次のとおり。これに当たらない書き方は unsupported に入れる。
      1. アプリの入口のファイル(cdk.json の app が名指すもの)で、`new <スタックのクラス>(<app の変数>, '<スタックの ID>', ...)` を見つける。スタックのパスは '<スタックの ID>'。
      2. そのクラスのコンストラクターの中で、`const <変数> = new <クラス>(this, '<ID>', ...)` か `this.<名前> = new <クラス>(this, '<ID>', ...)` を見つける。コンストラクトのパスは '<スタックの ID>/<ID>'。
      3. 同じコンストラクターの中の `Tags.of(<変数>).add('<キー>', '<値>')`(または `Tags.of(this.<名前>)`)の範囲は、その変数のコンストラクトのパス。`Tags.of(this)` の範囲はスタックのパス。
      4. 同じスタックのクラスを2回 new したときは、スタックの ID ごとにパスを作る。そのときのタグの箇所は、同じ Edit を持ち、SharedWith にもう一方の範囲が入る。
      synth のリソースのコンストラクトのパス(aws:cdk:path、例 'Stack/Bucket/Resource')に、範囲のパスが前方一致(/ の区切りで)すれば、そのタグはそのリソースに効く。
    violation: 対象のリソースのパスに一致する ConstructSite が無ければ、書き換えず手で直す一覧に出す(cdkNoSource)
    example: |
      デモのアプリ(internal/cdk/testdata/apps/ts-minimal)の期待する結果:
      - bin/app.ts の `new SkeletonStack(app, 'tag-refactor-cdk-skeleton')` → スタックのパス 'tag-refactor-cdk-skeleton'
      - lib/stack.ts の `const bucket = new s3.Bucket(this, 'Bucket')` → ConstructSite のパス 'tag-refactor-cdk-skeleton/Bucket'、その行
      - lib/stack.ts の `Tags.of(bucket).add('Env', 'dev')` → TagSite の範囲 'tag-refactor-cdk-skeleton/Bucket'、KeyEdit は 'Env' の引用符を含む範囲
      - synth のリソース 'tag-refactor-cdk-skeleton/Bucket/Resource'(AWS::S3::Bucket)に、このタグが効く
    source: FR2.11、AC2.1.6、契約 C3
  - id: BR2.3
    statement: synth できなかったアプリがあるとき、結び付かなかったリソースをその理由で手で直す一覧に出す
    category: policy
    applies_to: CdkTarget
    trigger: BR2.1 で候補が0のリソースが、CloudFormation のテンプレートの探索でも見つからなかったとき
    logic: IF failed か skipped のアプリがある THEN そのリソースを、アプリの理由で手で直す一覧に出す(toolMissing、needsAWS、timeout、synthError、補助のプログラムの失敗は cdkAppFailed、cdkV1 と unknownVersion は cdkV1、unsupportedLanguage は cdkUnsupportedLanguage)。ELSE 今までどおり templateNotFound
    violation: なし
    source: FR2.8、FR2.9、契約 C7 failure_to_manual と match_rule

  # 3. ソースへの反映
  - id: BR3.1
    statement: キーの名前の変更は、範囲がそのリソースを含む Tags.of(x).add の、文字列そのままのキーを書き換えて反映する
    category: calculation
    applies_to: CdkTagSite
    trigger: CdkTarget の変更が、キーの名前の変更のとき
    logic: |
      リソースに効くタグの箇所(範囲がそのリソースを含むもの。複数のファイルにまたがってよい)を集め、1つのタグ一式として今の iac の判定に渡す。
      IF 変更前のキーの箇所がちょうど1つで、キーと値の Shape が literal THEN その KeyEdit の範囲だけを新しいキーに書き換える。引用符の種類は元のままにする。
      書き換えは、その箇所のあるファイルにだけ作る
    violation: |
      変更前のキーの箇所が2つ以上なら、どれが効くかを決めず cdkAmbiguous。
      キーか値の Shape が expression なら cdkExpression。sameFileConst、sameFileMap、crossFileArg は U1 では扱わないので cdkUnsupportedShape(U4 で扱う)。
      変更前のキーの箇所が0のときは BR3.3
    source: FR3.1、FR3.2、AC3.1.1 の一部、契約 C3 edit_rules
  - id: BR3.2
    statement: 範囲の下でほかのリソースも同じキーを持つときは、書き換えない
    category: constraint
    applies_to: CdkTagSite
    trigger: BR3.1 で書き換えようとするとき
    logic: |
      IF そのタグの箇所の範囲の下に、同じキーのタグを持つ synth のリソースが、対象のリソースのほかにもある THEN 書き換えない。
      IF その箇所の Edit の SharedWith が空でない(同じスタックのクラスを2回使うなど) THEN SharedWith のすべての範囲に、同じ変更(種類、キー、値)を持つ変更案の対象があるときだけ、1回だけ書き換える(契約 C7 same_site_rule)
    violation: 範囲の下にほかのリソースがあれば cdkScopeTag。SharedWith の範囲のどれかに同じ変更が無ければ cdkSharedValue
    source: FR3.5、FD-Q2、契約 C7 same_site_rule
  - id: BR3.3
    statement: CDK の書き換えも、今の iac の決まり(反映済みの数え方、値の違い、aws: のキー、差分、書き出し)に従う
    category: policy
    applies_to: CdkTarget
    trigger: CDK のソースの書き換えを作るとき
    logic: ソースがすでに変更後の形なら反映済みに数える。変更前のキーが無ければ keyNotInSource、値が変更前の値と違えば valueDiffers、新しいキーがすでにあれば keyAlreadyExists とする。aws: で始まるキーは足さず、変えず、消さない。--write が無ければ差分だけを示す
    violation: 理由をつけて手で直す一覧に出す(今の iac の理由を使う)
    source: FR3.6、FR3.8、FR3.9、FD-Q1
  - id: BR3.4
    statement: 書き換えたあとの内容を、ディスクに書く前に補助のプログラムで読み直して確かめる
    category: validation
    applies_to: CdkAnalysis
    trigger: --write があり、アプリのファイルの書き換えが決まったとき
    logic: |
      --write の有無によらず、書き換えを作ったアプリごとに、書き換えたあとの内容をまとめて補助のプログラムに渡して読み直す(Recheck)。差分を示す前に行う。
      対象のタグの箇所が意図した形(新しいキー、同じ値、同じ範囲)になっていて、そのアプリのほかのタグの箇所(範囲、キー、値)と作成の箇所が変わっていないことを確かめる
    violation: 確かめに合わないファイルは差分にも書き出しにも入れず、そのファイルの変更を手で直す一覧に出す(unreadableFile。--debug に理由)
    source: FR3.7、ADR-005、契約 C3 Recheck、FD-Q1
  - id: BR3.5
    statement: 書き換えたアプリは、受け入れテストで synth し直して新しいキーになることを確かめる
    category: validation
    applies_to: Synthesis
    trigger: 受け入れテストと検証のとき
    logic: テスト用のアプリの写しに iac --write を動かしたあと、同じ写しを synth し直し、対象のリソースのタグに新しいキーと元の値があり、古いキーが無いことを確かめる
    violation: テストの失敗
    source: AC3.1.1

  # 4. プルリクエストの作成
  - id: BR4.1
    statement: --pr は --write を含み、書き換えの前に最低限の確かめをする
    category: validation
    applies_to: PullRequestPlan
    trigger: --pr を付けて実行したとき
    logic: 書き換えの前に、gh があってログイン済みであること、リモートが決まり GitHub であること、HEAD がブランチを指すことを確かめる。リモートは今のブランチの追跡先、無ければ origin
    violation: どのファイルも変えずに理由を示し、終了コード 1
    source: FR1.1、FR1.5、FR1.6、契約 C1
  - id: BR4.2
    statement: 書き換えたファイルがあるときだけ、ブランチ、コミット、push、作成を順に行う
    category: policy
    applies_to: PublishInput
    trigger: 書き換えが済んだとき
    logic: |
      IF 書き出しが途中で失敗した(WriteErr)か中断された(Interrupted) THEN 作成を呼ばず、書けたファイルと書けなかったファイルを示して終了コード 1。
      ELSE IF 書き出したファイルが1つ以上 THEN 実行したときの HEAD から既定の名前のブランチを作り、書き出したファイルだけを git add と git commit --only でコミットし、リモートに push し、gh pr create で行き先を実行したときのブランチにしたプルリクエストを作る。確認の問いは出さない。
      ELSE 何も作らない
    violation: どこかの段階が失敗したら、そこで止め、済んだ段階と失敗した段階を示し、終了コード 1。済んだことは元に戻さない
    source: FR1.1、FR1.3、FR1.8、FR1.9、FR1.11、FR1.12、契約 C1 commit
  - id: BR4.3
    statement: コミットのメッセージと既定のタイトルは同じ文で、本文は gh にファイルで渡す
    category: policy
    applies_to: PublishInput
    trigger: コミットとプルリクエストの作成のとき
    logic: タイトルは "Reflect tag changes from tag-refactor (<変更案のファイル名>)"(--lang に従う)。変更案のファイル名に ARN の形や 12 桁の数字の並びがあれば伏せ字にしてから入れる(BR4.4 と同じ規則)。コミットのメッセージの1行目もこの文にする。本文は Git の管理フォルダーの下のファイルに書き、gh pr create --body-file で渡し、成功したら消す
    violation: なし
    source: FR1.4、FR1.10、FD-Q6、契約 C1 body_file
  - id: BR4.4
    statement: プルリクエストの本文には、変更したファイルの一覧だけを載せ、ARN、アカウント ID、タグの値、トークン、外のプログラムの出力を載せない
    category: constraint
    applies_to: PublishInput
    trigger: 本文を組み立てるとき
    logic: U1 の本文は、作ったことを示す1〜3行と、変更したファイル(IaC の種類つき)の一覧だけにする。tag-refactor が組み立てる部分の ARN の形と 12 桁の数字の並びは伏せ字にする
    violation: なし(組み立ての決まり)
    source: FR1.10、NFR1、契約 X2
  - id: BR4.5
    statement: git と gh の呼び出しには時間の上限を置き、トークンを読まない、出さない
    category: constraint
    applies_to: PullRequestPlan
    trigger: git と gh を呼ぶとき
    logic: |
      読むだけの git と switch -c と add は 30 秒、commit と push は 10 分、gh は 2 分を上限にする。
      git と gh には利用者の環境をそのまま渡し、tag-refactor はトークンを読まず、出力にも出さない。
      commit と push では、フックと署名(パスフレーズの入力など)のために、標準入力と標準エラーを端末につなぐ。標準出力は読み捨てる。
      リモートの URL を表示や --debug に出すときは、URL の userinfo(ユーザー名とパスワードやトークン)を除く
    violation: 上限を超えたら子のプロセスを止め、その段階の失敗にする
    source: NFR1、NFR3、契約 C5 timeouts

  # 5. テストと CI
  - id: BR5.1
    statement: synth を動かすテストは3つの OS の CI で実行し、道具が無いと CI では失敗、手元では飛ばす
    category: policy
    applies_to: CdkApp
    trigger: テストを実行するとき
    logic: |
      テスト用のアプリは internal/cdk/testdata/apps/ts-minimal に置き、npm の依存は package-lock.json で版を固定する。CDK の CLI もアプリの devDependencies に入れて版を固定する。
      CI は、テストの前に、テスト用のアプリのフォルダーごとに npm ci を実行する。テストはアプリをいじらないよう、一時的なフォルダーに写してから使う(node_modules は写さずにつなぐ)。
      Node.js、CDK の CLI、テスト用のアプリの node_modules のどれかが無いとき、CI の環境(環境変数 CI が空でない)ならテストを失敗にし、手元なら理由を出して飛ばす。
      新しい CI の仕事(3つの OS での synth を含むテスト)は、必須のチェック ci-gate が待つ仕事に入れる
    violation: CI で道具が無ければテストの失敗
    source: NFR8、AC2.1.7、FD-Q3
  - id: BR5.2
    statement: 実際の GitHub にプルリクエストを作る結合テストは、テスト専用のリポジトリのトークンがあるときだけ実行し、あとで片付ける
    category: policy
    applies_to: PublishOutcome
    trigger: CI と手元の結合テストのとき
    logic: |
      環境変数 TAG_REFACTOR_TEST_REPO(owner/name。秘密ではないので CI のリポジトリの変数に置く)と TAG_REFACTOR_TEST_REPO_TOKEN(CI の secret)の両方が空でないときだけ実行する。
      fork のプルリクエストや Dependabot では secret が空の文字列になるので、「空でない」で判断する。
      トークンは、テストが起動する gh と git の子のプロセスの環境の GH_TOKEN にだけ入れる。clone は gh repo clone、push の認証は gh の資格情報のヘルパー(git -c credential.helper= -c 'credential.helper=!gh auth git-credential')で行う。
      トークンは、ワークフローの定義にもテストのデータにもコミットするファイルにも書かず、ログにも出さない。
      テスト専用のリポジトリを一時的なフォルダーに clone し、npm ci のあとで iac --pr を動かしてプルリクエストができることを確かめ、最後にプルリクエストを閉じてブランチを消す。
      どちらかが空なら理由を出して飛ばす
    violation: 片付けに失敗したら、テストの失敗にして残したものの名前を出す(トークンは出さない)
    source: NFR9、AC1.1.11、プロジェクトの禁止事項
```

## まとめ

| ID | 種類 | 一言で | 出典 |
|---|---|---|---|
| BR1.1 | policy | 変更案に CloudFormation のリソースがあるときだけ、cdk.json を探す | FR2.1、NFR5 |
| BR1.2 | calculation | 言語と CDK の版を決める。U1 は TypeScript の v2 だけ | FR2.10 |
| BR1.3 | policy | synth に AWS と GitHub の認証情報を渡さない | FR2.4、NFR2 |
| BR1.4 | constraint | synth の出力は一時的なフォルダー、上限は 10 分 | FR2.5、FR2.6 |
| BR1.5 | calculation | synth の失敗を種類に分ける。needsAWS の最小の文と対処 | FR2.8、AC2.1.3 |
| BR1.6 | calculation | 補助のプログラムの失敗をアプリの状態と理由に対応させる | 契約 C7 |
| BR1.7 | policy | アプリごとの状態と経過時間を標準エラーに示す | FR2.7、AC2.1.2 |
| BR2.1 | calculation | スタック名、論理 ID、アカウント、リージョンで照合する | FR2.11 |
| BR2.2 | calculation | U1 で扱う形から、コンストラクトのパスと Tags.of の範囲を組み立てる | FR2.11 |
| BR2.3 | policy | synth できなかったアプリがあるときの、結び付かなかったリソースの理由 | FR2.8、FR2.9 |
| BR3.1 | calculation | 文字列そのままのキーを書き換える。2か所以上は cdkAmbiguous | FR3.1、FR3.2 |
| BR3.2 | constraint | 範囲の下にほかのリソースがあれば書き換えない。共有の箇所は同じ変更のときだけ | FR3.5 |
| BR3.3 | policy | 反映済み、値の違い、aws: のキー、差分は今の iac のとおり | FR3.6、FR3.8、FR3.9 |
| BR3.4 | validation | 差分を示す前に、アプリごとに読み直して確かめる | FR3.7 |
| BR3.5 | validation | 受け入れテストで synth し直して確かめる | AC3.1.1 |
| BR4.1 | validation | --pr の最低限の確かめを書き換えの前にする | FR1.5、FR1.6 |
| BR4.2 | policy | 書き出しが全部済んだときだけ、ブランチ、コミット、push、作成を順に。途中で止まったら戻さない | FR1.1、FR1.8、FR1.12 |
| BR4.3 | policy | コミットのメッセージはタイトルと同じ文、本文はファイルで渡す | FR1.4、FR1.10 |
| BR4.4 | constraint | 本文に ARN や値やトークンを載せない | FR1.10、NFR1 |
| BR4.5 | constraint | git と gh に上限、トークンを読まない | NFR1、NFR3 |
| BR5.1 | policy | synth のテストは3つの OS、道具が無いと CI では失敗 | NFR8 |
| BR5.2 | policy | 実際の GitHub の結合テストはトークンがあるときだけ、あとで片付ける | NFR9 |

## U1 が答える契約の未決事項

| 未決事項(契約) | U1 の答え |
|---|---|
| C5: synth が外に問い合わせることを止め切れるか | CDK_DISABLE_VERSION_CHECK=1(版の問い合わせ)、--no-notices(お知らせの取得)、CDK_DISABLE_CLI_TELEMETRY=true(利用の記録の送信)で止める。どれも認証情報を伴わない問い合わせで、止め損ねても AWS の呼び出しにはならない。NFR Design で CDK の文書と照らして確かめ、手元の結合テストで synth の間の AWS の呼び出しが無いこと(AWS の認証情報が無くても成功すること)を確かめる |
| C4: 補助のプログラムを埋め込む形とライセンスの表示 | Go の embed でバイナリに入れ、実行のたびに一時的なフォルダーに書き出して node で動かす(FD-Q4)。補助のプログラムは tag-refactor 自身のコードで、同じライセンスに従う。利用者のアプリの typescript を実行のときに読むだけで、配布物には入れないので、THIRD_PARTY_NOTICES.txt に足すものは無い |
| C2: manifest の stackName と environment を読む場所 | 一時的な出力先の manifest.json の artifacts のうち type が aws:cloudformation:stack のものから、properties.stackName(無ければ成果物の ID)と environment(aws://<アカウント>/<リージョン>。unknown-account、unknown-region は空)を読む。テンプレートは properties.templateFile。Code Generation でテスト用のアプリの実際の出力と照らす |
